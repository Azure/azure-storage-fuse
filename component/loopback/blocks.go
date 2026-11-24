// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package loopback

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"slices"
	"syscall"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal"
)

const (
	// maxBlockIDLength is the longest block ID, before base64 encoding, that Azure Storage accepts.
	maxBlockIDLength = 64
	// maxCommittedBlocks is the most blocks a block blob can have.
	maxCommittedBlocks = 50000
	// maxBlockSize is the largest block Azure Storage accepts.
	maxBlockSize = 4000 * common.MbToBytes
	// maxSingleUploadSize is the largest blob azstorage uploads in a single request, without blocks.
	maxSingleUploadSize = 256 * common.MbToBytes
	// defaultUploadBlockSize is the block size azstorage uses when no block size is set.
	defaultUploadBlockSize = 16 * common.MbToBytes
)

// committedBlock is a block of the committed block list of a blob.
type committedBlock struct {
	ID   string `json:"id"`
	Size int64  `json:"size"`
}

// blockListRecord is the committed block list of a blob written in blocks through loopbackfs. It is stored for the
// inode of the file that holds the blob, so it follows the file when it is renamed. It also holds the size and the
// modification time of that file, so that it is ignored once the file is changed or replaced another way.
type blockListRecord struct {
	Size   int64            `json:"size"`
	Mtime  int64            `json:"mtime"`
	Blocks []committedBlock `json:"blocks"`
}

func blockListName(info fs.FileInfo) (string, bool) {
	id, ok := idOf(info)
	if !ok {
		return "", false
	}
	return fmt.Sprintf("%s/%x-%x.json", blockListDir, id.dev, id.ino), true
}

// readBlockList returns the recorded committed block list of the file described by info, if it still describes the
// file.
func readBlockList(root *os.Root, info fs.FileInfo) ([]committedBlock, bool) {
	name, ok := blockListName(info)
	if !ok || !info.Mode().IsRegular() {
		return nil, false
	}

	data, err := root.ReadFile(name)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			log.Warn("LoopbackFS::readBlockList : failed to read %s [%v]", name, err)
		}
		return nil, false
	}

	var record blockListRecord
	if err := json.Unmarshal(data, &record); err != nil {
		log.Warn("LoopbackFS::readBlockList : ignoring invalid block list %s [%v]", name, err)
		return nil, false
	}

	var size int64
	for _, b := range record.Blocks {
		size += b.Size
	}

	// The file was changed or replaced without the block operations, or it was removed and its inode reused.
	if record.Size != info.Size() || record.Mtime != info.ModTime().UnixNano() || size != record.Size {
		return nil, false
	}

	return record.Blocks, true
}

// writeBlockList records blocks as the committed block list of the file described by info.
func writeBlockList(root *os.Root, info fs.FileInfo, blocks []committedBlock) error {
	name, ok := blockListName(info)
	if !ok {
		return nil
	}

	if len(blocks) == 0 {
		removeBlockList(root, info)
		return nil
	}

	data, err := json.Marshal(blockListRecord{Size: info.Size(), Mtime: info.ModTime().UnixNano(), Blocks: blocks})
	if err != nil {
		return err
	}

	if err := root.MkdirAll(blockListDir, 0o700); err != nil {
		return storageError(err)
	}

	// Write the list under a unique name and rename it into place, so that it is never read half written.
	tmp, err := newUploadName(root)
	if err != nil {
		return err
	}

	if err := root.WriteFile(tmp, data, 0o600); err != nil {
		_ = root.Remove(tmp)
		return storageError(err)
	}

	if err := root.Rename(tmp, name); err != nil {
		_ = root.Remove(tmp)
		return storageError(err)
	}

	return nil
}

func removeBlockList(root *os.Root, info fs.FileInfo) {
	name, ok := blockListName(info)
	if !ok {
		return
	}

	if err := root.Remove(name); err != nil && !errors.Is(err, fs.ErrNotExist) {
		log.Warn("LoopbackFS::removeBlockList : failed to remove %s [%v]", name, err)
	}
}

// committedBlocks returns the committed block list of the blob held by the file described by info. A blob that was not
// written in blocks through loopbackfs, such as a file copied into the backing directory, is presented as uploaded in
// blocks of the configured block size, or as a blob without blocks when that size is 0.
func (lfs *LoopbackFS) committedBlocks(root *os.Root, info fs.FileInfo) []committedBlock {
	if !info.Mode().IsRegular() {
		return nil
	}

	if blocks, ok := readBlockList(root, info); ok {
		return blocks
	}

	return syntheticBlocks(info.Size(), lfs.blockSize.Load())
}

// syntheticBlocks returns the block list of a blob of size bytes uploaded in blocks of blockSize bytes.
func syntheticBlocks(size, blockSize int64) []committedBlock {
	if size <= 0 || blockSize <= 0 {
		return nil
	}

	count := (size + blockSize - 1) / blockSize
	if count > maxCommittedBlocks {
		// Azure Storage cannot hold a blob of this size in blocks of this size.
		return nil
	}

	blocks := make([]committedBlock, 0, count)
	for i := range count {
		blocks = append(blocks, committedBlock{ID: syntheticBlockID(i), Size: min(blockSize, size-i*blockSize)})
	}

	return blocks
}

// syntheticBlockID returns the ID of the block at index of a blob that was not written in blocks through loopbackfs.
// It depends only on the index, so a list returned earlier stays valid while the file is unchanged. Like the IDs
// block_cache generates, it is 16 bytes long before encoding.
func syntheticBlockID(index int64) string {
	return base64.StdEncoding.EncodeToString(fmt.Appendf(nil, "loopback%08d", index))
}

// blockIDLength validates a block ID the way Azure Storage does, and returns its length before encoding.
func blockIDLength(id string) (int, error) {
	raw, err := base64.StdEncoding.DecodeString(id)
	if err != nil || len(raw) == 0 || len(raw) > maxBlockIDLength {
		return 0, fmt.Errorf("loopbackfs: invalid block ID %q, a block ID must be the base64 encoding of 1 to %d bytes: %w",
			id, maxBlockIDLength, syscall.EINVAL)
	}
	return len(raw), nil
}

// validateBlockList validates a block list to commit the way Azure Storage does.
func validateBlockList(ids []string) error {
	if len(ids) > maxCommittedBlocks {
		return fmt.Errorf("loopbackfs: block list of %d blocks exceeds the limit of %d blocks: %w",
			len(ids), maxCommittedBlocks, syscall.EINVAL)
	}

	length := 0
	for _, id := range ids {
		n, err := blockIDLength(id)
		if err != nil {
			return err
		}

		if length != 0 && n != length {
			return fmt.Errorf("loopbackfs: block IDs of a blob must have the same length, %q differs: %w", id, syscall.EINVAL)
		}
		length = n
	}

	return nil
}

// stagedBlocksDir returns the directory that holds the uncommitted blocks of the blob name. Blob names are hashed so
// that the blocks of any blob sit in a single directory.
func stagedBlocksDir(name string) string {
	sum := sha256.Sum256([]byte(name))
	return stagedDir + "/" + hex.EncodeToString(sum[:])
}

func stagedBlockName(name, id string) string {
	return stagedBlocksDir(name) + "/" + hex.EncodeToString([]byte(id))
}

// discardStagedBlocks discards the uncommitted blocks of the blob name, as Azure Storage does when the blob is
// committed, replaced or deleted.
func discardStagedBlocks(root *os.Root, name string) {
	if err := root.RemoveAll(stagedBlocksDir(name)); err != nil {
		log.Warn("LoopbackFS::discardStagedBlocks : failed to discard staged blocks of %s [%v]", name, err)
	}
}

// blobName is objectName for operations that need a blob, which the root of the container is not.
func blobName(name string) (string, error) {
	objName, err := objectName(name)
	if err == nil && objName == "." {
		return "", syscall.EINVAL
	}
	return objName, err
}

// StageData stages a block of the blob options.Name, like Put Block. A staged block is invisible until a block list
// that includes it is committed, and it is discarded when the blob is committed without it, replaced or deleted.
func (lfs *LoopbackFS) StageData(options internal.StageDataOptions) error {
	log.Trace("LoopbackFS::StageData : name=%s, id=%s, size=%d", options.Name, options.Id, len(options.Data))

	name, err := blobName(options.Name)
	if err != nil {
		return err
	}

	if err := lfs.faults.check(OpStageData, name); err != nil {
		return err
	}

	if _, err := blockIDLength(options.Id); err != nil {
		log.Err("LoopbackFS::StageData : %v", err)
		return err
	}

	if int64(len(options.Data)) > maxBlockSize {
		return fmt.Errorf("loopbackfs: block of %d bytes exceeds the limit of %d bytes: %w",
			len(options.Data), int64(maxBlockSize), syscall.EINVAL)
	}

	// Blocks of the same blob can be staged at the same time, but not while a block list is committed.
	unlock := lfs.locks.rlock(name)
	defer unlock()

	root, err := lfs.openRoot()
	if err != nil {
		return err
	}
	defer root.Close()

	if err := root.MkdirAll(stagedBlocksDir(name), 0o700); err != nil {
		log.Err("LoopbackFS::StageData : failed to create staging directory for %s [%v]", name, err)
		return storageError(err)
	}

	// Write the block under a unique name and rename it into place, so that a block staged again with the same ID
	// replaces the previous one at once.
	tmp, err := newUploadName(root)
	if err != nil {
		return err
	}

	if err := root.WriteFile(tmp, options.Data, 0o600); err != nil {
		_ = root.Remove(tmp)
		log.Err("LoopbackFS::StageData : failed to write block %s of %s [%v]", options.Id, name, err)
		return storageError(err)
	}

	if err := root.Rename(tmp, stagedBlockName(name, options.Id)); err != nil {
		_ = root.Remove(tmp)
		log.Err("LoopbackFS::StageData : failed to stage block %s of %s [%v]", options.Id, name, err)
		return storageError(err)
	}

	return nil
}

// CommitData commits a block list to the blob options.Name, like Put Block List: the blob becomes the concatenation of
// the listed blocks, each of which is either staged or already committed to the blob. The size of a block is the size
// of the data it was staged with, so options.BlockSize is not used. Uncommitted blocks that are not in the list are
// discarded.
func (lfs *LoopbackFS) CommitData(options internal.CommitDataOptions) error {
	log.Trace("LoopbackFS::CommitData : name=%s, blocks=%d", options.Name, len(options.List))

	name, err := blobName(options.Name)
	if err != nil {
		return err
	}

	if err := lfs.faults.check(OpCommitData, name); err != nil {
		return err
	}

	if err := validateBlockList(options.List); err != nil {
		log.Err("LoopbackFS::CommitData : %v", err)
		return err
	}

	unlock := lfs.locks.lock(name)
	defer unlock()

	root, err := lfs.openRoot()
	if err != nil {
		return err
	}
	defer root.Close()

	plan, err := lfs.planCommit(root, name, options.List)
	if err != nil {
		log.Err("LoopbackFS::CommitData : failed to commit blocks of %s [%v]", name, err)
		return err
	}
	defer plan.close()

	info, err := lfs.upload(root, name, plan.write)
	if err != nil {
		log.Err("LoopbackFS::CommitData : failed to write %s [%v]", name, err)
		return err
	}

	// The blob is committed even if its block list cannot be recorded, it is then presented like a blob that was not
	// written in blocks.
	if err := writeBlockList(root, info, plan.blocks); err != nil {
		log.Err("LoopbackFS::CommitData : failed to record the block list of %s [%v]", name, err)
	}

	discardStagedBlocks(root, name)

	if options.NewETag != nil {
		*options.NewETag = lfs.versions.etag(info)
	}

	return nil
}

// blockSource tells where the data of a block of a block list to commit is.
type blockSource struct {
	staged string // name of the file of a staged block
	offset int64  // offset of a committed block in the blob
	size   int64
}

// commitPlan is the content of a blob that a block list commits.
type commitPlan struct {
	root    *os.Root
	current *os.File // the committed blob, which holds the data of the committed blocks
	sources []blockSource
	blocks  []committedBlock
}

// planCommit resolves the blocks of the list ids of the blob name. Like Put Block List, it uses the staged block of an
// ID if there is one, and the committed block otherwise.
func (lfs *LoopbackFS) planCommit(root *os.Root, name string, ids []string) (*commitPlan, error) {
	plan := &commitPlan{
		root:    root,
		sources: make([]blockSource, 0, len(ids)),
		blocks:  make([]committedBlock, 0, len(ids)),
	}

	committed, err := plan.openCommitted(lfs, name)
	if err != nil {
		return nil, err
	}

	for _, id := range ids {
		staged := stagedBlockName(name, id)
		info, err := root.Lstat(staged)
		if err == nil {
			plan.add(id, blockSource{staged: staged, size: info.Size()})
			continue
		}

		if !errors.Is(err, fs.ErrNotExist) {
			plan.close()
			return nil, storageError(err)
		}

		source, ok := committed[id]
		if !ok {
			plan.close()
			return nil, fmt.Errorf("loopbackfs: block %q of %s is neither staged nor committed: %w", id, name, syscall.EINVAL)
		}
		plan.add(id, source)
	}

	return plan, nil
}

// openCommitted opens the committed blob name, if it exists, and returns where its committed blocks are.
func (p *commitPlan) openCommitted(lfs *LoopbackFS, name string) (map[string]blockSource, error) {
	info, err := p.root.Lstat(name)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, storageError(err)
	}
	if info.IsDir() {
		return nil, syscall.EISDIR
	}
	if !info.Mode().IsRegular() {
		// A symlink blob has no blocks.
		return nil, nil
	}

	p.current, err = p.root.Open(name)
	if err != nil {
		return nil, lookupError(err)
	}

	info, err = p.current.Stat()
	if err != nil {
		p.close()
		return nil, storageError(err)
	}

	committed := make(map[string]blockSource)
	var offset int64
	for _, b := range lfs.committedBlocks(p.root, info) {
		if _, ok := committed[b.ID]; !ok {
			committed[b.ID] = blockSource{offset: offset, size: b.Size}
		}
		offset += b.Size
	}

	return committed, nil
}

func (p *commitPlan) add(id string, source blockSource) {
	p.sources = append(p.sources, source)
	p.blocks = append(p.blocks, committedBlock{ID: id, Size: source.size})
}

func (p *commitPlan) close() {
	if p.current != nil {
		_ = p.current.Close()
		p.current = nil
	}
}

// write writes the content of the blob to f.
func (p *commitPlan) write(f *os.File) error {
	for _, source := range p.sources {
		if err := p.copy(f, source); err != nil {
			return err
		}
	}
	return nil
}

func (p *commitPlan) copy(dst *os.File, source blockSource) error {
	var n int64
	var err error

	if source.staged != "" {
		var block *os.File
		block, err = p.root.Open(source.staged)
		if err != nil {
			return storageError(err)
		}
		n, err = io.Copy(dst, block)
		_ = block.Close()
	} else {
		// The file is private to the commit, so its offset can be moved. Copying from it at its offset lets the kernel
		// copy the data between the files.
		if _, err = p.current.Seek(source.offset, io.SeekStart); err != nil {
			return storageError(err)
		}
		n, err = io.CopyN(dst, p.current, source.size)
	}

	if err == nil && n != source.size {
		err = io.ErrUnexpectedEOF
	}
	if err != nil {
		return fmt.Errorf("loopbackfs: failed to copy a block of %d bytes: %w", source.size, storageError(err))
	}

	return nil
}

// GetCommittedBlockList returns the committed block list of the blob name, like Get Block List. Like azstorage, it
// returns no list for a blob without blocks.
func (lfs *LoopbackFS) GetCommittedBlockList(name string) (*internal.CommittedBlockList, error) {
	log.Trace("LoopbackFS::GetCommittedBlockList : name=%s", name)

	objName, err := blobName(name)
	if err != nil {
		return nil, err
	}

	if err := lfs.faults.check(OpGetCommittedBlockList, objName); err != nil {
		return nil, err
	}

	blocks, err := lfs.blockListOf(objName)
	if err != nil {
		return nil, err
	}

	if len(blocks) == 0 {
		return nil, nil
	}

	list := make(internal.CommittedBlockList, 0, len(blocks))
	var offset int64
	for _, b := range blocks {
		list = append(list, internal.CommittedBlock{Id: b.ID, Offset: offset, Size: uint64(b.Size)})
		offset += b.Size
	}

	return &list, nil
}

// GetFileBlockOffsets returns the committed blocks of the blob options.Name with their offsets.
func (lfs *LoopbackFS) GetFileBlockOffsets(options internal.GetFileBlockOffsetsOptions) (*common.BlockOffsetList, error) {
	log.Trace("LoopbackFS::GetFileBlockOffsets : name=%s", options.Name)

	list := &common.BlockOffsetList{BlockIdLength: common.BlockIDLength}

	name, err := blobName(options.Name)
	if err != nil {
		return list, err
	}

	if err := lfs.faults.check(OpGetFileBlockOffsets, name); err != nil {
		return list, err
	}

	blocks, err := lfs.blockListOf(name)
	if err != nil {
		return list, err
	}

	if len(blocks) > 0 {
		list.BlockIdLength = common.GetIdLength(blocks[0].ID)
	}

	var offset int64
	for _, b := range blocks {
		list.BlockList = append(list.BlockList, &common.Block{Id: b.ID, StartIndex: offset, EndIndex: offset + b.Size})
		offset += b.Size
	}

	return list, nil
}

func (lfs *LoopbackFS) blockListOf(name string) ([]committedBlock, error) {
	unlock := lfs.locks.rlock(name)
	defer unlock()

	root, err := lfs.openRoot()
	if err != nil {
		return nil, err
	}
	defer root.Close()

	info, err := root.Lstat(name)
	if err != nil {
		return nil, lookupError(err)
	}

	return lfs.committedBlocks(root, info), nil
}

// truncatedBlocks returns the committed block list that azstorage leaves to a blob of oldSize bytes with the committed
// block list blocks, when it truncates the blob to newSize bytes using blocks of blockSize bytes. Blocks that change
// get new IDs.
func truncatedBlocks(blocks []committedBlock, oldSize, newSize, blockSize int64) []committedBlock {
	if newSize == 0 || (blockSize == 0 && newSize <= maxSingleUploadSize) {
		// azstorage uploads such a blob in a single request, without blocks.
		return nil
	}

	size := blockSize
	if size == 0 {
		size = defaultUploadBlockSize
	}

	idLength := int64(common.BlockIDLength)
	if len(blocks) > 0 {
		idLength = common.GetIdLength(blocks[0].ID)
	}

	newBlock := func(n int64) committedBlock {
		return committedBlock{ID: common.GetBlockID(idLength), Size: n}
	}

	result := slices.Clone(blocks)
	if len(result) == 0 {
		// A blob without blocks is cut into blocks first.
		for offset := int64(0); offset < oldSize; offset += size {
			result = append(result, newBlock(min(size, oldSize-offset)))
		}
	}

	if newSize < oldSize {
		var end int64
		for i, b := range result {
			if end+b.Size >= newSize {
				if end+b.Size > newSize {
					result[i] = newBlock(newSize - end)
				}
				return result[:i+1]
			}
			end += b.Size
		}
		return result
	}

	end := oldSize
	if blockSize != 0 && len(result) > 0 {
		// Fill up the last block first, so that every block but the last one has the block size.
		last := &result[len(result)-1]
		if last.Size < size {
			grown := min(size, last.Size+newSize-end)
			end += grown - last.Size
			*last = newBlock(grown)
		}
	}

	for end < newSize {
		n := min(size, newSize-end)
		result = append(result, newBlock(n))
		end += n
	}

	return result
}
