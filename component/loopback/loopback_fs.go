/*
    _____           _____   _____   ____          ______  _____  ------
   |     |  |      |     | |     | |     |     | |       |            |
   |     |  |      |     | |     | |     |     | |       |            |
   | --- |  |      |     | |-----| |---- |     | |-----| |-----  ------
   |     |  |      |     | |     | |     |     |       | |       |
   | ____|  |_____ | ____| | ____| |     |_____|  _____| |_____  |_____


   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2020-2026 Microsoft Corporation. All rights reserved.
   Author : <blobfusedev@microsoft.com>

   Permission is hereby granted, free of charge, to any person obtaining a copy
   of this software and associated documentation files (the "Software"), to deal
   in the Software without restriction, including without limitation the rights
   to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
   copies of the Software, and to permit persons to whom the Software is
   furnished to do so, subject to the following conditions:

   The above copyright notice and this permission notice shall be included in all
   copies or substantial portions of the Software.

   THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
   IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
   FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
   AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
   LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
   OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
   SOFTWARE
*/

package loopback

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/config"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal"
	"github.com/Azure/azure-storage-fuse/v2/internal/handlemap"
)

const compName = "loopbackfs"

// defaultBlockSizeMB is the block size, in MB, of the blobs that were not written in blocks through loopbackfs, when
// the block-size-mb option is not set.
const defaultBlockSizeMB = 1

// LoopbackFS is a storage component that emulates Azure Storage on a local directory. See the package documentation.
type LoopbackFS struct {
	internal.BaseComponent

	path         string
	listPageSize atomic.Int32
	blockSize    atomic.Int64
	contentMD5   atomic.Bool

	locks    objectLocks
	versions versions
	faults   faultInjector
}

var _ internal.Component = &LoopbackFS{}

// LoopbackFSOptions are the options of the component, in the loopbackfs section of the configuration.
type LoopbackFSOptions struct {
	// Path is the backing directory that holds the objects of the emulated container. It is created if it is missing.
	Path string `config:"path" yaml:"path,omitempty"`
	// ListPageSize is the most entries a StreamDir call returns. The default is 5000, the most entries a List Blobs
	// call returns.
	ListPageSize uint32 `config:"list-page-size" yaml:"list-page-size,omitempty"`
	// BlockSizeMB is the size, in MB, of the blocks of the blobs that were not written in blocks through loopbackfs.
	// 0 presents them as blobs without blocks. The default is 1.
	BlockSizeMB float64 `config:"block-size-mb" yaml:"block-size-mb,omitempty"`
	// ContentMD5 reports the MD5 of the content of files in GetAttr and in listings.
	ContentMD5 bool `config:"content-md5" yaml:"content-md5,omitempty"`
	// Faults are the faults to inject into the operations of the component.
	Faults []FaultOptions `config:"faults" yaml:"faults,omitempty"`
}

// Name returns the name of the component.
func (lfs *LoopbackFS) Name() string {
	return compName
}

// Priority returns the priority of the component, which is at the end of the pipeline like azstorage.
func (lfs *LoopbackFS) Priority() internal.ComponentPriority {
	return internal.EComponentPriority.Consumer()
}

// Configure reads the options of the component and creates the backing directory.
func (lfs *LoopbackFS) Configure(_ bool) error {
	log.Trace("LoopbackFS::Configure : %s", lfs.Name())

	conf := LoopbackFSOptions{}
	if err := config.UnmarshalKey(lfs.Name(), &conf); err != nil {
		log.Err("LoopbackFS::Configure : config error [invalid config attributes]")
		return fmt.Errorf("config error in %s [%s]", lfs.Name(), err.Error())
	}

	backingPath := common.ExpandPath(conf.Path)
	if backingPath == "" {
		log.Err("LoopbackFS::Configure : config error [path not set]")
		return fmt.Errorf("config error in %s [path not set]", lfs.Name())
	}

	var mountPath string
	if err := config.UnmarshalKey("mount-path", &mountPath); err == nil && mountPath != "" && isWithin(backingPath, mountPath) {
		log.Err("LoopbackFS::Configure : config error [path %s is within the mount path %s]", backingPath, mountPath)
		return fmt.Errorf("config error in %s [path %s is within the mount path %s]", lfs.Name(), backingPath, mountPath)
	}

	if err := os.MkdirAll(backingPath, 0o777); err != nil {
		log.Err("LoopbackFS::Configure : config error [failed to create %s: %v]", backingPath, err)
		return fmt.Errorf("config error in %s [%s]", lfs.Name(), err.Error())
	}

	if err := lfs.applyOptions(conf); err != nil {
		log.Err("LoopbackFS::Configure : config error [%v]", err)
		return fmt.Errorf("config error in %s [%s]", lfs.Name(), err.Error())
	}

	lfs.path = backingPath
	lfs.logOptions("LoopbackFS::Configure")

	return nil
}

// OnConfigChange applies the options that can change while the component runs, which are all but path. Faults can
// thus be added to, or removed from, a mount by editing its configuration file.
func (lfs *LoopbackFS) OnConfigChange() {
	log.Trace("LoopbackFS::OnConfigChange : %s", lfs.Name())

	conf := LoopbackFSOptions{}
	if err := config.UnmarshalKey(lfs.Name(), &conf); err != nil {
		log.Err("LoopbackFS::OnConfigChange : config error [invalid config attributes]")
		return
	}

	if err := lfs.applyOptions(conf); err != nil {
		log.Err("LoopbackFS::OnConfigChange : config error [%v], keeping the previous options", err)
		return
	}

	lfs.logOptions("LoopbackFS::OnConfigChange")
}

// applyOptions validates and applies the options that can change while the component runs.
func (lfs *LoopbackFS) applyOptions(conf LoopbackFSOptions) error {
	pageSize := int32(common.MaxDirListCount)
	if conf.ListPageSize != 0 {
		if conf.ListPageSize > common.MaxDirListCount {
			return fmt.Errorf("list-page-size %d exceeds %d, the most entries Azure Storage lists at once",
				conf.ListPageSize, common.MaxDirListCount)
		}
		pageSize = int32(conf.ListPageSize)
	}

	blockSizeMB := float64(defaultBlockSizeMB)
	if config.IsSet(lfs.Name() + ".block-size-mb") {
		blockSizeMB = conf.BlockSizeMB
	}

	blockSize := int64(blockSizeMB * common.MbToBytes)
	if blockSizeMB < 0 || blockSize > maxBlockSize || (blockSizeMB > 0 && blockSize == 0) {
		return fmt.Errorf("block-size-mb %v must be 0, or between 1 byte and %d MB", blockSizeMB,
			maxBlockSize/common.MbToBytes)
	}

	rules := make([]*faultRule, 0, len(conf.Faults))
	for i, opts := range conf.Faults {
		f, err := opts.fault()
		if err != nil {
			return fmt.Errorf("invalid fault %d: %w", i+1, err)
		}
		rules = append(rules, newFaultRule(f))
	}

	lfs.listPageSize.Store(pageSize)
	lfs.blockSize.Store(blockSize)
	lfs.contentMD5.Store(conf.ContentMD5)
	lfs.faults.setConfigured(rules)

	return nil
}

func (lfs *LoopbackFS) logOptions(caller string) {
	lfs.faults.mu.Lock()
	faults := len(lfs.faults.configured)
	lfs.faults.mu.Unlock()

	log.Crit("%s : path %s, list-page-size %d, block-size %d, content-md5 %t, faults %d", caller, lfs.path,
		lfs.listPageSize.Load(), lfs.blockSize.Load(), lfs.contentMD5.Load(), faults)
}

// isWithin reports whether the path p is the directory dir or is inside it.
func isWithin(p, dir string) bool {
	rel, err := filepath.Rel(filepath.Clean(dir), filepath.Clean(p))
	return err == nil && filepath.IsLocal(rel)
}

// SetConsistency sets whether GetAttr and listings report the MD5 of the content of files, like the content-md5
// option.
func (lfs *LoopbackFS) SetConsistency(consistency bool) {
	lfs.contentMD5.Store(consistency)
}

// Start starts the component. A component that was not configured starts, so that pipelines that never reach it can
// be tested, but all of its operations fail.
func (lfs *LoopbackFS) Start(_ context.Context) error {
	log.Trace("LoopbackFS::Start : Starting component %s", lfs.Name())

	if lfs.path == "" {
		log.Warn("LoopbackFS::Start : %v, all operations will fail", errNotConfigured)
		return nil
	}

	log.Info("LoopbackFS::Start : emulating Azure Storage in %s", lfs.path)
	return nil
}

// Stop stops the component. It holds no resources between operations, so there is nothing to release.
func (lfs *LoopbackFS) Stop() error {
	log.Trace("LoopbackFS::Stop : Stopping component %s", lfs.Name())
	return nil
}

// openRoot opens the backing directory. Every access to the objects goes through it, so that no name and no
// symbolic link can lead outside of the backing directory.
func (lfs *LoopbackFS) openRoot() (*os.Root, error) {
	if lfs.path == "" {
		return nil, errNotConfigured
	}

	root, err := os.OpenRoot(lfs.path)
	if err != nil {
		log.Err("LoopbackFS::openRoot : failed to open %s [%v]", lfs.path, err)
		return nil, storageError(err)
	}

	return root, nil
}

// begin starts the operation op on the object rawName: it returns the name of the file of the object and applies the
// faults of the operation.
func (lfs *LoopbackFS) begin(op Op, rawName string) (string, error) {
	name, err := objectName(rawName)
	if err != nil {
		return "", err
	}

	if err := lfs.faults.check(op, objectPath(name)); err != nil {
		return "", err
	}

	return name, nil
}

// changed records a change of the object name.
func (lfs *LoopbackFS) changed(root *os.Root, name string) {
	if info, err := root.Lstat(name); err == nil {
		lfs.versions.changed(info)
	}
}

// attr returns the attributes of the object name, whose file has the file info info, the way azstorage returns the
// properties of a blob: all times are the last modification time, and the mode is marked as default because block
// blobs have no permissions.
func (lfs *LoopbackFS) attr(root *os.Root, name string, info fs.FileInfo) *internal.ObjAttr {
	attr := &internal.ObjAttr{
		Path:  objectPath(name),
		Size:  info.Size(),
		Mode:  info.Mode(),
		Mtime: info.ModTime(),
		ETag:  lfs.versions.etag(info),
	}

	if attr.Path != "" {
		attr.Name = path.Base(attr.Path)
	}

	attr.Atime = attr.Mtime
	attr.Ctime = attr.Mtime
	attr.Crtime = attr.Mtime

	attr.Flags.Set(internal.PropFlagModeDefault)
	switch {
	case isSymlink(info):
		attr.Flags.Set(internal.PropFlagSymlink)
	case info.IsDir():
		attr.Flags.Set(internal.PropFlagIsDir)
	case lfs.contentMD5.Load():
		attr.MD5 = md5Of(root, name)
	}

	return attr
}

// ------------------------- Directory operations -------------------------

// CreateDir creates the directory options.Name and its missing parents. Like a hierarchical namespace account, it fails
// with EEXIST if the directory exists.
func (lfs *LoopbackFS) CreateDir(options internal.CreateDirOptions) error {
	log.Trace("LoopbackFS::CreateDir : name=%s, mode=%v", options.Name, options.Mode)

	name, err := lfs.begin(OpCreateDir, options.Name)
	if err != nil {
		return err
	}

	if name == "." {
		return syscall.EEXIST
	}

	root, err := lfs.openRoot()
	if err != nil {
		return err
	}
	defer root.Close()

	if err := makeParent(root, name); err != nil {
		log.Err("LoopbackFS::CreateDir : failed to create the parents of %s [%v]", name, err)
		return err
	}

	if err := root.Mkdir(name, dirPerm(options.Mode)); err != nil {
		log.Err("LoopbackFS::CreateDir : failed to create %s [%v]", name, err)
		return storageError(err)
	}

	lfs.changed(root, name)
	return nil
}

// DeleteDir deletes the directory options.Name, which must be empty.
func (lfs *LoopbackFS) DeleteDir(options internal.DeleteDirOptions) error {
	log.Trace("LoopbackFS::DeleteDir : name=%s", options.Name)

	name, err := lfs.begin(OpDeleteDir, options.Name)
	if err != nil {
		return err
	}

	if name == "." {
		return syscall.EBUSY
	}

	root, err := lfs.openRoot()
	if err != nil {
		return err
	}
	defer root.Close()

	info, err := root.Lstat(name)
	if err != nil {
		return lookupError(err)
	}

	if !info.IsDir() {
		return syscall.ENOTDIR
	}

	if err := root.Remove(name); err != nil {
		log.Err("LoopbackFS::DeleteDir : failed to delete %s [%v]", name, err)
		return storageError(err)
	}

	lfs.versions.removed(info)
	return nil
}

// IsDirEmpty reports whether nothing is listed under the directory options.Name. Like a listing of Azure Storage, it
// reports true for a directory that does not exist, and false if the directory cannot be listed.
func (lfs *LoopbackFS) IsDirEmpty(options internal.IsDirEmptyOptions) bool {
	log.Trace("LoopbackFS::IsDirEmpty : name=%s", options.Name)

	name, err := lfs.begin(OpIsDirEmpty, options.Name)
	if err != nil {
		log.Err("LoopbackFS::IsDirEmpty : %s [%v]", options.Name, err)
		return false
	}

	root, err := lfs.openRoot()
	if err != nil {
		return false
	}
	defer root.Close()

	info, err := root.Lstat(name)
	if err != nil {
		return lookupError(err) == syscall.ENOENT
	}

	if !info.IsDir() {
		return true
	}

	dir, err := root.Open(name)
	if err != nil {
		log.Err("LoopbackFS::IsDirEmpty : failed to open %s [%v]", name, err)
		return false
	}
	defer dir.Close()

	for {
		names, err := dir.Readdirnames(16)
		for _, entry := range names {
			if name != "." || entry != stateDir {
				return false
			}
		}

		if err == io.EOF {
			return true
		}

		if err != nil {
			log.Err("LoopbackFS::IsDirEmpty : failed to list %s [%v]", name, err)
			return false
		}
	}
}

// listEntry is an entry of a directory listing, with the key that orders the listing. Azure Storage lists blobs and
// blob prefixes in the order of their names, and the names of prefixes end with a slash.
type listEntry struct {
	name string
	key  string
}

// list returns the entries of the directory dir, in the order Azure Storage lists them. It only reads the directory,
// so that a page of a large listing costs no more than the attributes of the entries in the page.
func list(root *os.Root, dir string) ([]listEntry, error) {
	f, err := root.Open(dir)
	if err != nil {
		return nil, lookupError(err)
	}
	defer f.Close()

	dirEntries, err := f.ReadDir(-1)
	if err != nil {
		// Nothing is listed under a file either.
		return nil, lookupError(err)
	}

	entries := make([]listEntry, 0, len(dirEntries))
	for _, entry := range dirEntries {
		if dir == "." && entry.Name() == stateDir {
			continue
		}

		key := entry.Name()
		if entry.IsDir() {
			key += "/"
		}
		entries = append(entries, listEntry{name: entry.Name(), key: key})
	}

	slices.SortFunc(entries, func(a, b listEntry) int { return strings.Compare(a.key, b.key) })
	return entries, nil
}

// listAttrs returns the attributes of the entries of the directory dir, leaving out those removed since they were
// listed.
func (lfs *LoopbackFS) listAttrs(root *os.Root, dir string, entries []listEntry) ([]*internal.ObjAttr, error) {
	dirRoot, err := root.OpenRoot(dir)
	if err != nil {
		return nil, lookupError(err)
	}
	defer dirRoot.Close()

	attrs := make([]*internal.ObjAttr, 0, len(entries))
	for _, entry := range entries {
		info, err := dirRoot.Lstat(entry.name)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, storageError(err)
		}

		name := entry.name
		if dir != "." {
			name = dir + "/" + entry.name
		}
		attrs = append(attrs, lfs.attr(root, name, info))
	}

	return attrs, nil
}

// ReadDir lists the directory options.Name. It fails with ENOENT for a directory that does not exist.
func (lfs *LoopbackFS) ReadDir(options internal.ReadDirOptions) ([]*internal.ObjAttr, error) {
	log.Trace("LoopbackFS::ReadDir : name=%s", options.Name)

	name, err := lfs.begin(OpReadDir, options.Name)
	if err != nil {
		return nil, err
	}

	root, err := lfs.openRoot()
	if err != nil {
		return nil, err
	}
	defer root.Close()

	entries, err := list(root, name)
	if err == nil {
		var attrs []*internal.ObjAttr
		if attrs, err = lfs.listAttrs(root, name, entries); err == nil {
			return attrs, nil
		}
	}

	log.Err("LoopbackFS::ReadDir : failed to list %s [%v]", name, err)
	return nil, err
}

// StreamDir lists a page of the directory options.Name, like List Blobs: it returns at most options.Count entries,
// and no more than the list-page-size option, that follow the continuation token options.Token, with the token of the
// next page. The token is empty when the listing is complete.
func (lfs *LoopbackFS) StreamDir(options internal.StreamDirOptions) ([]*internal.ObjAttr, string, error) {
	log.Trace("LoopbackFS::StreamDir : name=%s, token=%s, count=%d", options.Name, options.Token, options.Count)

	name, err := lfs.begin(OpStreamDir, options.Name)
	if err != nil {
		return nil, "", err
	}

	var marker string
	if options.Token != "" {
		decoded, err := base64.RawURLEncoding.DecodeString(options.Token)
		if err != nil {
			log.Err("LoopbackFS::StreamDir : invalid token %s for %s", options.Token, name)
			return nil, "", syscall.EINVAL
		}
		marker = string(decoded)
	}

	count := lfs.listPageSize.Load()
	if options.Count > 0 && options.Count < count {
		count = options.Count
	}

	root, err := lfs.openRoot()
	if err != nil {
		return nil, "", err
	}
	defer root.Close()

	entries, err := list(root, name)
	if err != nil {
		log.Err("LoopbackFS::StreamDir : failed to list %s [%v]", name, err)
		return nil, "", err
	}

	start := 0
	if marker != "" {
		start = sort.Search(len(entries), func(i int) bool { return entries[i].key > marker })
	}
	end := min(start+int(count), len(entries))

	attrs, err := lfs.listAttrs(root, name, entries[start:end])
	if err != nil {
		log.Err("LoopbackFS::StreamDir : failed to list %s [%v]", name, err)
		return nil, "", err
	}

	var token string
	if end < len(entries) {
		token = base64.RawURLEncoding.EncodeToString([]byte(entries[end-1].key))
	}

	return attrs, token, nil
}

// RenameDir renames the directory options.Src to options.Dst, creating the missing parents of options.Dst.
func (lfs *LoopbackFS) RenameDir(options internal.RenameDirOptions) error {
	log.Trace("LoopbackFS::RenameDir : %s -> %s", options.Src, options.Dst)

	src, err := lfs.begin(OpRenameDir, options.Src)
	if err != nil {
		return err
	}

	dst, err := objectName(options.Dst)
	if err != nil {
		return err
	}

	if src == "." || dst == "." {
		return syscall.EBUSY
	}

	root, err := lfs.openRoot()
	if err != nil {
		return err
	}
	defer root.Close()

	if _, err := root.Lstat(src); err != nil {
		return lookupError(err)
	}

	if src == dst {
		return nil
	}

	if err := makeParent(root, dst); err != nil {
		log.Err("LoopbackFS::RenameDir : failed to create the parents of %s [%v]", dst, err)
		return err
	}

	if err := root.Rename(src, dst); err != nil {
		log.Err("LoopbackFS::RenameDir : failed to rename %s to %s [%v]", src, dst, err)
		return storageError(err)
	}

	lfs.changed(root, dst)
	return nil
}

// ------------------------- File operations -------------------------

// CreateFile creates the empty object options.Name, replacing an object of the same name like an upload of a block blob
// does, and creates its missing parents. The handle it returns holds no resources: callers can drop it, as file_cache
// and block_cache do.
func (lfs *LoopbackFS) CreateFile(options internal.CreateFileOptions) (*handlemap.Handle, error) {
	log.Trace("LoopbackFS::CreateFile : name=%s, mode=%v", options.Name, options.Mode)

	name, err := lfs.begin(OpCreateFile, options.Name)
	if err != nil {
		return nil, err
	}

	if name == "." {
		return nil, syscall.EISDIR
	}

	unlock := lfs.locks.lock(name)
	defer unlock()

	root, err := lfs.openRoot()
	if err != nil {
		return nil, err
	}
	defer root.Close()

	if err := makeParent(root, name); err != nil {
		log.Err("LoopbackFS::CreateFile : failed to create the parents of %s [%v]", name, err)
		return nil, err
	}

	// An upload replaces a symlink blob, so a symbolic link is replaced instead of followed.
	if old, err := root.Lstat(name); err == nil && isSymlink(old) {
		if err := root.Remove(name); err != nil {
			return nil, storageError(err)
		}
		lfs.forget(root, old)
	}

	f, err := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, filePerm(options.Mode))
	if err != nil {
		log.Err("LoopbackFS::CreateFile : failed to create %s [%v]", name, err)
		return nil, storageError(err)
	}

	info, err := f.Stat()
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		log.Err("LoopbackFS::CreateFile : failed to create %s [%v]", name, err)
		return nil, storageError(err)
	}

	// The file may be an existing one that was truncated.
	removeBlockList(root, info)
	discardStagedBlocks(root, name)
	lfs.versions.changed(info)

	handle := handlemap.NewHandle(options.Name)
	handle.Mtime = info.ModTime()

	return handle, nil
}

// DeleteFile deletes the object options.Name, with its uncommitted blocks.
func (lfs *LoopbackFS) DeleteFile(options internal.DeleteFileOptions) error {
	log.Trace("LoopbackFS::DeleteFile : name=%s", options.Name)

	name, err := lfs.begin(OpDeleteFile, options.Name)
	if err != nil {
		return err
	}

	if name == "." {
		return syscall.EISDIR
	}

	unlock := lfs.locks.lock(name)
	defer unlock()

	root, err := lfs.openRoot()
	if err != nil {
		return err
	}
	defer root.Close()

	info, err := root.Lstat(name)
	if err != nil {
		return lookupError(err)
	}

	if info.IsDir() {
		return syscall.EISDIR
	}

	if err := root.Remove(name); err != nil {
		log.Err("LoopbackFS::DeleteFile : failed to delete %s [%v]", name, err)
		return storageError(err)
	}

	lfs.forget(root, info)
	discardStagedBlocks(root, name)

	return nil
}

// OpenFile opens the object options.Name, which must exist, like azstorage: the handle holds the size and modification
// time of the object, and no resources. Reads and writes through it address the object by name. O_TRUNC truncates the
// object, as the kernel can ask when it opens a file. Other flags are ignored: CreateFile creates objects.
func (lfs *LoopbackFS) OpenFile(options internal.OpenFileOptions) (*handlemap.Handle, error) {
	log.Trace("LoopbackFS::OpenFile : name=%s, flags=%s", options.Name, common.PrettyOpenFlags(options.Flags))

	name, err := lfs.begin(OpOpenFile, options.Name)
	if err != nil {
		return nil, err
	}

	if name == "." {
		return nil, syscall.EISDIR
	}

	truncate := options.Flags&os.O_TRUNC != 0
	if truncate {
		unlock := lfs.locks.lock(name)
		defer unlock()
	}

	root, err := lfs.openRoot()
	if err != nil {
		return nil, err
	}
	defer root.Close()

	info, err := root.Lstat(name)
	if err != nil {
		return nil, lookupError(err)
	}

	if info.IsDir() {
		return nil, syscall.EISDIR
	}

	if truncate && info.Mode().IsRegular() {
		info, err = lfs.truncateWhole(root, name)
		if err != nil {
			log.Err("LoopbackFS::OpenFile : failed to truncate %s [%v]", name, err)
			return nil, err
		}
	}

	handle := handlemap.NewHandle(options.Name)
	handle.Size = info.Size()
	handle.Mtime = info.ModTime()

	return handle, nil
}

// truncateWhole truncates the object name to 0 bytes, like an upload of an empty blob. The caller must hold the lock
// of name.
func (lfs *LoopbackFS) truncateWhole(root *os.Root, name string) (fs.FileInfo, error) {
	f, err := root.OpenFile(name, os.O_WRONLY|os.O_TRUNC, 0)
	if err != nil {
		return nil, storageError(err)
	}

	info, err := f.Stat()
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return nil, storageError(err)
	}

	removeBlockList(root, info)
	discardStagedBlocks(root, name)
	lfs.versions.changed(info)

	return info, nil
}

// faultName returns the name that faults match for the object rawName of a handle.
func faultName(rawName string) string {
	name, err := objectName(rawName)
	if err != nil {
		return strings.Trim(rawName, "/")
	}
	return objectPath(name)
}

// ReleaseFile releases a handle. Handles hold no resources, like the handles of azstorage.
func (lfs *LoopbackFS) ReleaseFile(options internal.ReleaseFileOptions) error {
	if options.Handle == nil {
		return syscall.EINVAL
	}

	log.Trace("LoopbackFS::ReleaseFile : name=%s", options.Handle.Path)
	return lfs.faults.check(OpReleaseFile, faultName(options.Handle.Path))
}

// FlushFile flushes a handle. Writes reach the object at once, so there is nothing to flush.
func (lfs *LoopbackFS) FlushFile(options internal.FlushFileOptions) error {
	if options.Handle == nil {
		return syscall.EINVAL
	}

	log.Trace("LoopbackFS::FlushFile : name=%s", options.Handle.Path)
	return lfs.faults.check(OpFlushFile, faultName(options.Handle.Path))
}

// RenameFile renames the object options.Src to options.Dst, replacing an object at options.Dst and creating its
// missing parents. Like Azure Storage, which copies the blob, it gives the object a new modification time and ETag,
// and it updates options.SrcAttr with them, as azstorage does. Uncommitted blocks of both names are discarded.
func (lfs *LoopbackFS) RenameFile(options internal.RenameFileOptions) error {
	log.Trace("LoopbackFS::RenameFile : %s -> %s", options.Src, options.Dst)

	src, err := lfs.begin(OpRenameFile, options.Src)
	if err != nil {
		return err
	}

	dst, err := objectName(options.Dst)
	if err != nil {
		return err
	}

	if src == "." || dst == "." {
		return syscall.EBUSY
	}

	unlock := lfs.locks.lock(src, dst)
	defer unlock()

	root, err := lfs.openRoot()
	if err != nil {
		return err
	}
	defer root.Close()

	srcInfo, err := root.Lstat(src)
	if err != nil {
		return lookupError(err)
	}

	if src == dst {
		return nil
	}

	if err := makeParent(root, dst); err != nil {
		log.Err("LoopbackFS::RenameFile : failed to create the parents of %s [%v]", dst, err)
		return err
	}

	dstInfo, dstErr := root.Lstat(dst)
	blocks, recorded := readBlockList(root, srcInfo)

	if err := root.Rename(src, dst); err != nil {
		log.Err("LoopbackFS::RenameFile : failed to rename %s to %s [%v]", src, dst, err)
		return storageError(err)
	}

	if dstErr == nil && !os.SameFile(srcInfo, dstInfo) {
		lfs.forget(root, dstInfo)
	}
	discardStagedBlocks(root, src)
	discardStagedBlocks(root, dst)

	if !isSymlink(srcInfo) {
		now := time.Now()
		if err := root.Chtimes(dst, now, now); err != nil {
			log.Warn("LoopbackFS::RenameFile : failed to set the modification time of %s [%v]", dst, err)
		}
	}

	info, err := root.Lstat(dst)
	if err != nil {
		return storageError(err)
	}

	if recorded {
		if err := writeBlockList(root, info, blocks); err != nil {
			log.Err("LoopbackFS::RenameFile : failed to record the block list of %s [%v]", dst, err)
		}
	}

	lfs.versions.changed(info)

	if options.SrcAttr != nil {
		options.SrcAttr.Mtime = info.ModTime()
		options.SrcAttr.Atime = options.SrcAttr.Mtime
		options.SrcAttr.Ctime = options.SrcAttr.Mtime
		options.SrcAttr.ETag = lfs.versions.etag(info)
	}

	return nil
}

// ReadFile returns the content of the object of options.Handle.
func (lfs *LoopbackFS) ReadFile(options internal.ReadFileOptions) ([]byte, error) {
	if options.Handle == nil {
		return nil, syscall.EINVAL
	}

	log.Trace("LoopbackFS::ReadFile : name=%s", options.Handle.Path)

	name, err := lfs.begin(OpReadFile, options.Handle.Path)
	if err != nil {
		return nil, err
	}

	root, err := lfs.openRoot()
	if err != nil {
		return nil, err
	}
	defer root.Close()

	blob, err := openBlob(root, name)
	if err != nil {
		return nil, err
	}
	defer blob.Close()

	data := make([]byte, blob.info.Size())
	n, err := io.ReadFull(io.NewSectionReader(blob, 0, blob.info.Size()), data)
	if err != nil && err != io.ErrUnexpectedEOF {
		log.Err("LoopbackFS::ReadFile : failed to read %s [%v]", name, err)
		return nil, storageError(err)
	}

	return data[:n], nil
}

// ReadInBuffer reads the object of options.Handle, or the object options.Path, from options.Offset into options.Data,
// like a ranged download.
//
// With a handle, it reads like pread(2) the current content of the object, and returns 0 bytes at its end. Without a
// handle, like azstorage, it reads no more than options.Size bytes in all, and fails with ERANGE when the offset is
// past options.Size or past the end of an object that has shrunk.
//
// When it reads data, it sets *options.Etag to the ETag of the object.
func (lfs *LoopbackFS) ReadInBuffer(options *internal.ReadInBufferOptions) (int, error) {
	rawName := options.Path
	if options.Handle != nil {
		rawName = options.Handle.Path
	} else if rawName == "" {
		log.Err("LoopbackFS::ReadInBuffer : path not given")
		return 0, syscall.EINVAL
	}

	name, err := lfs.begin(OpReadInBuffer, rawName)
	if err != nil {
		return 0, err
	}

	if options.Offset < 0 {
		return 0, syscall.EINVAL
	}

	data := options.Data
	if options.Handle == nil {
		if options.Offset > options.Size {
			return 0, syscall.ERANGE
		}
		data = data[:min(int64(len(data)), options.Size-options.Offset)]
	}

	if len(data) == 0 {
		return 0, nil
	}

	if options.Etag != nil {
		*options.Etag = ""
	}

	root, err := lfs.openRoot()
	if err != nil {
		return 0, err
	}
	defer root.Close()

	blob, err := openBlob(root, name)
	if err != nil {
		if err != syscall.ENOENT {
			log.Err("LoopbackFS::ReadInBuffer : failed to open %s [%v]", name, err)
		}
		return 0, err
	}
	defer blob.Close()

	n := 0
	if options.Offset < blob.info.Size() {
		n, err = blob.ReadAt(data, options.Offset)
		if err != nil && err != io.EOF {
			log.Err("LoopbackFS::ReadInBuffer : failed to read %s at offset %d [%v]", name, options.Offset, err)
			return 0, storageError(err)
		}
	} else if options.Handle == nil {
		return 0, syscall.ERANGE
	}

	if options.Etag != nil {
		*options.Etag = lfs.versions.etag(blob.info)
	}

	return n, nil
}

// WriteFile writes options.Data at options.Offset of the object of options.Handle, extending the object as needed.
func (lfs *LoopbackFS) WriteFile(options *internal.WriteFileOptions) (int, error) {
	if options.Handle == nil {
		return 0, syscall.EINVAL
	}

	name, err := lfs.begin(OpWriteFile, options.Handle.Path)
	if err != nil {
		return 0, err
	}

	if options.Offset < 0 {
		return 0, syscall.EINVAL
	}

	if len(options.Data) == 0 {
		return 0, nil
	}

	root, err := lfs.openRoot()
	if err != nil {
		return 0, err
	}
	defer root.Close()

	info, err := root.Lstat(name)
	if err != nil {
		return 0, lookupError(err)
	}

	if info.IsDir() {
		return 0, syscall.EISDIR
	}

	if !info.Mode().IsRegular() {
		return 0, syscall.EINVAL
	}

	f, err := root.OpenFile(name, os.O_WRONLY, 0)
	if err != nil {
		log.Err("LoopbackFS::WriteFile : failed to open %s [%v]", name, err)
		return 0, lookupError(err)
	}

	n, err := f.WriteAt(options.Data, options.Offset)
	if info, statErr := f.Stat(); statErr == nil {
		lfs.versions.changed(info)
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}

	if err != nil {
		log.Err("LoopbackFS::WriteFile : failed to write %s at offset %d [%v]", name, options.Offset, err)
		return n, storageError(err)
	}

	return n, nil
}

// TruncateFile changes the size of the object options.Name to options.NewSize; the handle, if any, is not used. When
// azstorage would truncate the blob using blocks, which it does when options.BlockSize is set or the blob is larger
// than a single upload, the committed block list changes as azstorage changes it, so block_cache can keep writing the
// blob.
func (lfs *LoopbackFS) TruncateFile(options internal.TruncateFileOptions) error {
	log.Trace("LoopbackFS::TruncateFile : name=%s, size=%d", options.Name, options.NewSize)

	name, err := lfs.begin(OpTruncateFile, options.Name)
	if err != nil {
		return err
	}

	if name == "." {
		return syscall.EISDIR
	}

	if options.NewSize < 0 {
		return syscall.EINVAL
	}

	unlock := lfs.locks.lock(name)
	defer unlock()

	root, err := lfs.openRoot()
	if err != nil {
		return err
	}
	defer root.Close()

	info, err := root.Lstat(name)
	if err != nil {
		return lookupError(err)
	}

	if info.IsDir() {
		return syscall.EISDIR
	}

	if !info.Mode().IsRegular() {
		return syscall.EINVAL
	}

	if info.Size() == options.NewSize {
		return nil
	}

	blocks := lfs.committedBlocks(root, info)

	f, err := root.OpenFile(name, os.O_WRONLY, 0)
	if err != nil {
		return lookupError(err)
	}

	err = f.Truncate(options.NewSize)
	newInfo, statErr := f.Stat()
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = statErr
	}
	if err != nil {
		log.Err("LoopbackFS::TruncateFile : failed to truncate %s [%v]", name, err)
		return storageError(err)
	}

	lfs.versions.changed(newInfo)
	discardStagedBlocks(root, name)

	newBlocks := truncatedBlocks(blocks, info.Size(), options.NewSize, options.BlockSize)
	if err := writeBlockList(root, newInfo, newBlocks); err != nil {
		log.Err("LoopbackFS::TruncateFile : failed to record the block list of %s [%v]", name, err)
	}

	return nil
}

// CopyToFile downloads the object options.Name into options.File from the start of the file, and truncates the file to
// the downloaded data, like azstorage. It downloads options.Count bytes from options.Offset, or the rest of the object
// when options.Count is 0. It fails with ERANGE for an offset past the end of the object.
func (lfs *LoopbackFS) CopyToFile(options internal.CopyToFileOptions) error {
	log.Trace("LoopbackFS::CopyToFile : name=%s, offset=%d, count=%d", options.Name, options.Offset, options.Count)

	name, err := lfs.begin(OpCopyToFile, options.Name)
	if err != nil {
		return err
	}

	if options.File == nil || options.Offset < 0 {
		return syscall.EINVAL
	}

	root, err := lfs.openRoot()
	if err != nil {
		return err
	}
	defer root.Close()

	blob, err := openBlob(root, name)
	if err != nil {
		log.Err("LoopbackFS::CopyToFile : failed to open %s [%v]", name, err)
		return err
	}
	defer blob.Close()

	size := blob.info.Size()
	if options.Offset > size {
		return syscall.ERANGE
	}

	count := size - options.Offset
	if options.Count > 0 && options.Count < count {
		count = options.Count
	}

	n, err := copyData(io.NewOffsetWriter(options.File, 0), io.NewSectionReader(blob, options.Offset, count), count)
	if err == nil {
		err = options.File.Truncate(n)
	}
	if err != nil {
		log.Err("LoopbackFS::CopyToFile : failed to download %s [%v]", name, err)
		return storageError(err)
	}

	return nil
}

// CopyFromFile uploads the content of options.File, from the start of the file, as the object options.Name, creating
// its missing parents. The object is replaced at once, as a blob is, and its uncommitted blocks are discarded.
// options.Metadata is not stored.
func (lfs *LoopbackFS) CopyFromFile(options internal.CopyFromFileOptions) error {
	log.Trace("LoopbackFS::CopyFromFile : name=%s", options.Name)

	name, err := lfs.begin(OpCopyFromFile, options.Name)
	if err != nil {
		return err
	}

	if name == "." {
		return syscall.EISDIR
	}

	if options.File == nil {
		return syscall.EINVAL
	}

	src, err := options.File.Stat()
	if err != nil {
		return storageError(err)
	}

	unlock := lfs.locks.lock(name)
	defer unlock()

	root, err := lfs.openRoot()
	if err != nil {
		return err
	}
	defer root.Close()

	_, err = lfs.upload(root, name, func(f *os.File) error {
		_, err := copyData(f, io.NewSectionReader(options.File, 0, src.Size()), src.Size())
		return storageError(err)
	})
	if err != nil {
		log.Err("LoopbackFS::CopyFromFile : failed to upload %s [%v]", name, err)
		return err
	}

	discardStagedBlocks(root, name)
	return nil
}

// ------------------------- Symlink operations -------------------------

// CreateLink creates the symbolic link options.Name to options.Target, replacing an object of the same name like an
// upload of a symlink blob does, and creates its missing parents.
func (lfs *LoopbackFS) CreateLink(options internal.CreateLinkOptions) error {
	log.Trace("LoopbackFS::CreateLink : %s -> %s", options.Name, options.Target)

	name, err := lfs.begin(OpCreateLink, options.Name)
	if err != nil {
		return err
	}

	if name == "." {
		return syscall.EEXIST
	}

	unlock := lfs.locks.lock(name)
	defer unlock()

	root, err := lfs.openRoot()
	if err != nil {
		return err
	}
	defer root.Close()

	old, err := root.Lstat(name)
	switch {
	case err == nil && old.IsDir():
		return syscall.EEXIST
	case errors.Is(err, fs.ErrNotExist):
		old = nil
	case err != nil:
		return storageError(err)
	}

	if err := makeParent(root, name); err != nil {
		log.Err("LoopbackFS::CreateLink : failed to create the parents of %s [%v]", name, err)
		return err
	}

	// Create the link under a unique name and rename it into place, to replace an object of the same name at once.
	tmp, err := newUploadName(root)
	if err != nil {
		return err
	}

	if err := root.Symlink(options.Target, tmp); err != nil {
		log.Err("LoopbackFS::CreateLink : failed to create %s [%v]", name, err)
		return storageError(err)
	}

	if err := root.Rename(tmp, name); err != nil {
		_ = root.Remove(tmp)
		log.Err("LoopbackFS::CreateLink : failed to create %s [%v]", name, err)
		return storageError(err)
	}

	if old != nil {
		lfs.forget(root, old)
	}
	discardStagedBlocks(root, name)
	lfs.changed(root, name)

	return nil
}

// ReadLink returns the target of the symbolic link options.Name. Like azstorage, it reads no more than options.Size
// bytes of the target, the size GetAttr reports, and returns an empty target when options.Size is 0.
func (lfs *LoopbackFS) ReadLink(options internal.ReadLinkOptions) (string, error) {
	log.Trace("LoopbackFS::ReadLink : name=%s, size=%d", options.Name, options.Size)

	name, err := lfs.begin(OpReadLink, options.Name)
	if err != nil {
		return "", err
	}

	if options.Size <= 0 {
		return "", nil
	}

	root, err := lfs.openRoot()
	if err != nil {
		return "", err
	}
	defer root.Close()

	target, err := root.Readlink(name)
	if err != nil {
		log.Err("LoopbackFS::ReadLink : failed to read %s [%v]", name, err)
		return "", lookupError(err)
	}

	if limit := min(options.Size, common.MaxSymlinkTargetLen); int64(len(target)) > limit {
		target = target[:limit]
	}

	return target, nil
}

// ------------------------- Attribute operations -------------------------

// GetAttr returns the attributes of the object options.Name, see attr. It fails with ENOENT for an object that does not
// exist.
func (lfs *LoopbackFS) GetAttr(options internal.GetAttrOptions) (*internal.ObjAttr, error) {
	name, err := objectName(options.Name)
	if err != nil {
		// No object has an invalid name.
		return nil, syscall.ENOENT
	}

	if err := lfs.faults.check(OpGetAttr, objectPath(name)); err != nil {
		return nil, err
	}

	root, err := lfs.openRoot()
	if err != nil {
		return nil, err
	}
	defer root.Close()

	info, err := root.Lstat(name)
	if err != nil {
		err = lookupError(err)
		if err != syscall.ENOENT {
			log.Err("LoopbackFS::GetAttr : failed to get the attributes of %s [%v]", name, err)
		}
		return nil, err
	}

	return lfs.attr(root, name, info), nil
}

// Chmod changes the mode of the object options.Name. The owner keeps the right to read and write files and to search
// directories, so that the component can still use them. Symbolic links have no mode of their own, so it does not
// change them.
func (lfs *LoopbackFS) Chmod(options internal.ChmodOptions) error {
	log.Trace("LoopbackFS::Chmod : name=%s, mode=%v", options.Name, options.Mode)

	name, err := lfs.begin(OpChmod, options.Name)
	if err != nil {
		return err
	}

	root, err := lfs.openRoot()
	if err != nil {
		return err
	}
	defer root.Close()

	info, err := root.Lstat(name)
	if err != nil {
		return lookupError(err)
	}

	if isSymlink(info) {
		return nil
	}

	mode := filePerm(options.Mode)
	if info.IsDir() {
		mode = dirPerm(options.Mode)
	}

	if err := root.Chmod(name, mode); err != nil {
		log.Err("LoopbackFS::Chmod : failed to change the mode of %s [%v]", name, err)
		return storageError(err)
	}

	lfs.changed(root, name)
	return nil
}

// Chown changes the owner of the object options.Name. Azure Storage does not fail a change of owner, so an owner that
// the process is not allowed to set is ignored.
func (lfs *LoopbackFS) Chown(options internal.ChownOptions) error {
	log.Trace("LoopbackFS::Chown : name=%s, owner=%d, group=%d", options.Name, options.Owner, options.Group)

	name, err := lfs.begin(OpChown, options.Name)
	if err != nil {
		return err
	}

	root, err := lfs.openRoot()
	if err != nil {
		return err
	}
	defer root.Close()

	if _, err := root.Lstat(name); err != nil {
		return lookupError(err)
	}

	err = root.Lchown(name, options.Owner, options.Group)
	if errors.Is(err, syscall.EPERM) {
		log.Warn("LoopbackFS::Chown : not allowed to change the owner of %s, ignoring it", name)
		return nil
	}
	if err != nil {
		log.Err("LoopbackFS::Chown : failed to change the owner of %s [%v]", name, err)
		return storageError(err)
	}

	lfs.changed(root, name)
	return nil
}

// NewLoopbackFSComponent returns a new loopbackfs component.
func NewLoopbackFSComponent() internal.Component {
	lfs := &LoopbackFS{}
	lfs.SetName(compName)
	lfs.listPageSize.Store(common.MaxDirListCount)
	lfs.blockSize.Store(defaultBlockSizeMB * common.MbToBytes)
	config.AddConfigChangeEventListener(lfs)
	return lfs
}

func init() {
	internal.AddComponent(compName, NewLoopbackFSComponent)
}
