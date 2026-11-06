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
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/internal"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

const mb = int64(common.MbToBytes)

type blocksTestSuite struct {
	suite.Suite
	lfs  *LoopbackFS
	path string
}

func TestBlocks(t *testing.T) {
	suite.Run(t, new(blocksTestSuite))
}

func (s *blocksTestSuite) SetupTest() {
	s.path = s.T().TempDir()
	s.lfs = newTestLoopback(s.T(), s.path)
}

func (s *blocksTestSuite) stage(name, id string, data []byte) {
	s.Require().NoError(s.lfs.StageData(internal.StageDataOptions{Name: name, Id: id, Data: data}))
}

func (s *blocksTestSuite) commit(name string, ids ...string) string {
	etag := ""
	s.Require().NoError(s.lfs.CommitData(internal.CommitDataOptions{Name: name, List: ids, NewETag: &etag}))
	return etag
}

func (s *blocksTestSuite) content(name string) []byte {
	data, err := os.ReadFile(filepath.Join(s.path, name))
	s.Require().NoError(err)
	return data
}

func (s *blocksTestSuite) blockList(name string) []internal.CommittedBlock {
	list, err := s.lfs.GetCommittedBlockList(name)
	s.Require().NoError(err)
	if list == nil {
		return nil
	}
	return *list
}

func (s *blocksTestSuite) hasStagedBlocks(name string) bool {
	_, err := os.Stat(filepath.Join(s.path, stagedBlocksDir(name)))
	return err == nil
}

func (s *blocksTestSuite) TestStageAndCommit() {
	a, b, c := blockID(1), blockID(2), blockID(3)
	s.stage("dir/blob", a, []byte("aaaa"))
	s.stage("dir/blob", b, []byte("bb"))
	s.stage("dir/blob", c, []byte("cccccc"))

	// Staged blocks are invisible until they are committed.
	_, err := s.lfs.GetAttr(internal.GetAttrOptions{Name: "dir/blob"})
	s.Equal(syscall.ENOENT, err)
	attrs, err := s.lfs.ReadDir(internal.ReadDirOptions{Name: ""})
	s.Require().NoError(err)
	s.Empty(attrs)

	// The blob is the concatenation of blocks of any size, in the order of the list, in a directory that is created.
	etag := s.commit("dir/blob", c, a, b)
	s.Equal([]byte("ccccccaaaabb"), s.content("dir/blob"))

	attr, err := s.lfs.GetAttr(internal.GetAttrOptions{Name: "dir/blob"})
	s.Require().NoError(err)
	s.Equal(attr.ETag, etag)

	s.Equal([]internal.CommittedBlock{
		{Id: c, Offset: 0, Size: 6},
		{Id: a, Offset: 6, Size: 4},
		{Id: b, Offset: 10, Size: 2},
	}, s.blockList("dir/blob"))

	s.False(s.hasStagedBlocks("dir/blob"), "a commit discards the staged blocks")
}

func (s *blocksTestSuite) TestStageReplacesBlock() {
	a := blockID(1)
	s.stage("blob", a, []byte("old"))
	s.stage("blob", a, []byte("new data"))
	s.commit("blob", a)
	s.Equal([]byte("new data"), s.content("blob"))
}

// block_cache commits the IDs of the blocks it did not change together with the blocks it staged.
func (s *blocksTestSuite) TestCommitReusesCommittedBlocks() {
	a, b, c := blockID(1), blockID(2), blockID(3)
	s.stage("blob", a, []byte("1111"))
	s.stage("blob", b, []byte("2222"))
	s.stage("blob", c, []byte("33"))
	s.commit("blob", a, b, c)

	changed := blockID(4)
	s.stage("blob", changed, []byte("XXXX"))
	s.commit("blob", a, changed, c)
	s.Equal([]byte("1111XXXX33"), s.content("blob"))
	s.Equal([]internal.CommittedBlock{
		{Id: a, Offset: 0, Size: 4},
		{Id: changed, Offset: 4, Size: 4},
		{Id: c, Offset: 8, Size: 2},
	}, s.blockList("blob"))

	// A block can be listed more than once, like the zero block of block_cache.
	s.commit("blob", c, c, a)
	s.Equal([]byte("33331111"), s.content("blob"))

	// A block that is neither staged nor committed fails the commit, which leaves the blob as it is.
	err := s.lfs.CommitData(internal.CommitDataOptions{Name: "blob", List: []string{a, blockID(9)}})
	s.ErrorIs(err, syscall.EINVAL)
	s.Equal([]byte("33331111"), s.content("blob"))

	// b was left out of the last committed list, so it is gone.
	err = s.lfs.CommitData(internal.CommitDataOptions{Name: "blob", List: []string{b}})
	s.ErrorIs(err, syscall.EINVAL)
}

// block_cache stages a partial last block and, once the file grows, a filler block after it, so blocks of a list can
// have any size. The old loopbackfs placed every block at its index times the block size, which corrupted such blobs.
func (s *blocksTestSuite) TestCommitBlocksOfDifferentSizes() {
	partial, filler, next := blockID(1), blockID(2), blockID(3)
	s.stage("blob", partial, bytes.Repeat([]byte("p"), 3))
	s.stage("blob", filler, make([]byte, 5))
	s.stage("blob", next, bytes.Repeat([]byte("n"), 8))
	s.commit("blob", partial, filler, next)

	s.Equal(append(append([]byte("ppp"), make([]byte, 5)...), bytes.Repeat([]byte("n"), 8)...), s.content("blob"))
}

func (s *blocksTestSuite) TestCommitEmptyList() {
	s.Require().NoError(os.WriteFile(filepath.Join(s.path, "blob"), []byte("hello"), 0o644))

	s.commit("blob")
	s.Empty(s.content("blob"))
	s.Nil(s.blockList("blob"), "a blob without blocks has no block list")

	s.commit("new/blob")
	s.Empty(s.content("new/blob"))
}

func (s *blocksTestSuite) TestBlockIDValidation() {
	for _, id := range []string{"", "123", "not base64!", encodeID(strings.Repeat("x", maxBlockIDLength+1))} {
		err := s.lfs.StageData(internal.StageDataOptions{Name: "blob", Id: id, Data: []byte("x")})
		s.ErrorIs(err, syscall.EINVAL, "id %q", id)
	}

	short := encodeID("short")
	s.stage("blob", short, []byte("x"))
	s.stage("blob", blockID(1), []byte("y"))

	err := s.lfs.CommitData(internal.CommitDataOptions{Name: "blob", List: []string{short, blockID(1)}})
	s.ErrorIs(err, syscall.EINVAL, "the IDs of the blocks of a blob must have the same length")

	tooMany := make([]string, maxCommittedBlocks+1)
	for i := range tooMany {
		tooMany[i] = blockID(1)
	}
	err = s.lfs.CommitData(internal.CommitDataOptions{Name: "blob", List: tooMany})
	s.ErrorIs(err, syscall.EINVAL)

	s.Equal(syscall.EINVAL, s.lfs.StageData(internal.StageDataOptions{Name: "", Id: blockID(1), Data: []byte("x")}))
	s.Equal(syscall.EINVAL, s.lfs.CommitData(internal.CommitDataOptions{Name: "/", List: []string{}}))
	_, err = s.lfs.GetCommittedBlockList("")
	s.Equal(syscall.EINVAL, err)
}

// Azure Storage discards the uncommitted blocks of a blob when the blob is committed, replaced or deleted.
func (s *blocksTestSuite) TestUncommittedBlocksAreDiscarded() {
	a := blockID(1)
	upload, err := os.CreateTemp(s.T().TempDir(), "upload")
	s.Require().NoError(err)
	defer upload.Close()

	operations := map[string]func(name string) error{
		"commit without the block": func(name string) error {
			return s.lfs.CommitData(internal.CommitDataOptions{Name: name, List: []string{}})
		},
		"delete": func(name string) error {
			return s.lfs.DeleteFile(internal.DeleteFileOptions{Name: name})
		},
		"upload": func(name string) error {
			return s.lfs.CopyFromFile(internal.CopyFromFileOptions{Name: name, File: upload})
		},
		"create": func(name string) error {
			_, err := s.lfs.CreateFile(internal.CreateFileOptions{Name: name, Mode: 0o644})
			return err
		},
		"truncate": func(name string) error {
			return s.lfs.TruncateFile(internal.TruncateFileOptions{Name: name, NewSize: 100})
		},
		"rename": func(name string) error {
			return s.lfs.RenameFile(internal.RenameFileOptions{Src: name, Dst: name + ".renamed"})
		},
		"symlink": func(name string) error {
			return s.lfs.CreateLink(internal.CreateLinkOptions{Name: name, Target: "target"})
		},
		"open with O_TRUNC": func(name string) error {
			_, err := s.lfs.OpenFile(internal.OpenFileOptions{Name: name, Flags: os.O_WRONLY | os.O_TRUNC})
			return err
		},
	}

	for operation, run := range operations {
		name := strings.ReplaceAll(operation, " ", "_")
		s.commit(name)
		s.stage(name, a, []byte("pending"))
		s.Require().True(s.hasStagedBlocks(name))

		s.Require().NoError(run(name), operation)
		s.False(s.hasStagedBlocks(name), operation)
	}
}

// A blob that was not written in blocks through loopbackfs, such as a file copied into the backing directory, is
// presented as uploaded in blocks of block-size-mb.
func (s *blocksTestSuite) TestPresentedBlockList() {
	data := bytes.Repeat([]byte("0123456789abcdef"), int(5*mb/2/16))
	s.Require().NoError(os.WriteFile(filepath.Join(s.path, "file"), data, 0o644))

	list := s.blockList("file")
	s.Require().Len(list, 3, "the partial last block must be listed")
	s.Equal([]int64{0, mb, 2 * mb}, []int64{list[0].Offset, list[1].Offset, list[2].Offset})
	s.Equal([]uint64{uint64(mb), uint64(mb), uint64(mb / 2)}, []uint64{list[0].Size, list[1].Size, list[2].Size})
	s.Equal(list, s.blockList("file"), "the IDs must be stable while the file does not change")
	for _, block := range list {
		s.Equal(int64(common.BlockIDLength), common.GetIdLength(block.Id))
	}

	// Committing the presented list with one block changed keeps the data of the other blocks, as block_cache expects.
	changed := blockID(1)
	s.stage("file", changed, bytes.Repeat([]byte("x"), int(mb)))
	s.commit("file", list[0].Id, changed, list[2].Id)
	want := append(append(append([]byte{}, data[:mb]...), bytes.Repeat([]byte("x"), int(mb))...), data[2*mb:]...)
	s.Equal(want, s.content("file"))

	s.Require().NoError(os.WriteFile(filepath.Join(s.path, "empty"), nil, 0o644))
	s.Nil(s.blockList("empty"))

	_, err := s.lfs.GetCommittedBlockList("missing")
	s.Equal(syscall.ENOENT, err)

	// With block-size-mb 0, such blobs have no blocks, like blobs uploaded in a single request.
	lfs := newTestLoopback(s.T(), s.path, "block-size-mb: 0")
	s.Require().NoError(os.WriteFile(filepath.Join(s.path, "other"), data, 0o644))
	none, err := lfs.GetCommittedBlockList("other")
	s.Require().NoError(err)
	s.Nil(none)
}

func (s *blocksTestSuite) TestBlockListFollowsRenameAndRemount() {
	a, b := blockID(1), blockID(2)
	s.stage("blob", a, []byte("aaaa"))
	s.stage("blob", b, []byte("bb"))
	s.commit("blob", a, b)
	want := s.blockList("blob")

	s.Require().NoError(s.lfs.RenameFile(internal.RenameFileOptions{Src: "blob", Dst: "dir/renamed"}))
	s.Equal(want, s.blockList("dir/renamed"))

	// The list is kept with the backing directory, so it outlives the component, as it does after a remount.
	other := newTestLoopback(s.T(), s.path)
	list, err := other.GetCommittedBlockList("dir/renamed")
	s.Require().NoError(err)
	s.Equal(want, []internal.CommittedBlock(*list))

	// A blob that changes another way is presented like a blob written as a whole.
	s.Require().NoError(os.WriteFile(filepath.Join(s.path, "dir/renamed"), []byte("changed"), 0o644))
	list, err = other.GetCommittedBlockList("dir/renamed")
	s.Require().NoError(err)
	s.Equal([]internal.CommittedBlock{{Id: syntheticBlockID(0), Offset: 0, Size: 7}}, []internal.CommittedBlock(*list))
}

// After block_cache truncates a blob, it must still be able to open the blob for writing, which needs every block but
// the last to have its block size.
func (s *blocksTestSuite) TestTruncateKeepsBlocksUsable() {
	ids := []string{blockID(1), blockID(2), blockID(3)}
	for i, id := range ids {
		s.stage("blob", id, bytes.Repeat([]byte{byte('a' + i)}, int(mb)))
	}
	s.commit("blob", ids...)

	s.Require().NoError(s.lfs.TruncateFile(internal.TruncateFileOptions{Name: "blob", NewSize: 5 * mb / 2, BlockSize: mb}))
	list := s.blockList("blob")
	s.Require().Len(list, 3)
	s.Equal(ids[0], list[0].Id, "unchanged blocks keep their IDs")
	s.Equal(ids[1], list[1].Id, "unchanged blocks keep their IDs")
	s.NotEqual(ids[2], list[2].Id, "a changed block gets a new ID")
	s.Equal(uint64(mb/2), list[2].Size)

	s.Require().NoError(s.lfs.TruncateFile(internal.TruncateFileOptions{Name: "blob", NewSize: 9 * mb / 2, BlockSize: mb}))
	list = s.blockList("blob")
	s.Require().Len(list, 5)
	var size uint64
	for i, block := range list {
		if i < len(list)-1 {
			s.Equal(uint64(mb), block.Size, "block %d", i)
		}
		size += block.Size
	}
	s.Equal(uint64(9*mb/2), size)

	data := s.content("blob")
	s.Equal(bytes.Repeat([]byte("c"), int(mb/2)), data[2*mb:5*mb/2])
	s.Equal(make([]byte, 2*mb), data[5*mb/2:])

	// A truncation without a block size uploads small blobs as a whole, like azstorage.
	s.Require().NoError(s.lfs.TruncateFile(internal.TruncateFileOptions{Name: "blob", NewSize: 10}))
	s.Equal([]internal.CommittedBlock{{Id: syntheticBlockID(0), Offset: 0, Size: 10}}, s.blockList("blob"))

	s.Require().NoError(s.lfs.TruncateFile(internal.TruncateFileOptions{Name: "blob", NewSize: 0, BlockSize: mb}))
	s.Nil(s.blockList("blob"))
}

func TestTruncatedBlocks(t *testing.T) {
	blocks := []committedBlock{{ID: blockID(1), Size: mb}, {ID: blockID(2), Size: mb}, {ID: blockID(3), Size: mb / 2}}

	sizes := func(blocks []committedBlock) []int64 {
		var sizes []int64
		for _, b := range blocks {
			sizes = append(sizes, b.Size)
		}
		return sizes
	}

	testCases := []struct {
		name      string
		blocks    []committedBlock
		oldSize   int64
		newSize   int64
		blockSize int64
		want      []int64
		kept      int
	}{
		{"to zero", blocks, 5 * mb / 2, 0, mb, nil, 0},
		{"small without block size", blocks, 5 * mb / 2, mb, 0, nil, 0},
		{"shrink to a block boundary", blocks, 5 * mb / 2, 2 * mb, mb, []int64{mb, mb}, 2},
		{"shrink within a block", blocks, 5 * mb / 2, 3 * mb / 2, mb, []int64{mb, mb / 2}, 1},
		{"grow with block size", blocks, 5 * mb / 2, 4 * mb, mb, []int64{mb, mb, mb, mb}, 2},
		{"grow within the last block", blocks, 5 * mb / 2, 11 * mb / 4, mb, []int64{mb, mb, 3 * mb / 4}, 2},
		{"grow without block size", blocks, 5 * mb / 2, 300 * mb, 0, append([]int64{mb, mb, mb / 2}, growth(300*mb-5*mb/2)...), 3},
		{"blob without blocks", nil, 3 * mb, 4 * mb, mb, []int64{mb, mb, mb, mb}, 0},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := truncatedBlocks(tc.blocks, tc.oldSize, tc.newSize, tc.blockSize)
			assert.Equal(t, tc.want, sizes(got))
			for i := range got {
				assert.Equal(t, i < tc.kept, got[i].ID == blocks[min(i, len(blocks)-1)].ID, "ID of block %d", i)
				assert.Equal(t, int64(common.BlockIDLength), common.GetIdLength(got[i].ID))
			}
		})
	}

	assert.Len(t, blocks, 3, "the input must not change")
	assert.Equal(t, mb/2, blocks[2].Size, "the input must not change")
}

// growth returns the sizes of the blocks azstorage appends to grow a blob by n bytes without a block size.
func growth(n int64) []int64 {
	var sizes []int64
	for n > 0 {
		size := min(n, int64(defaultUploadBlockSize))
		sizes = append(sizes, size)
		n -= size
	}
	return sizes
}

func (s *blocksTestSuite) TestGetFileBlockOffsets() {
	a, b := blockID(1), blockID(2)
	s.stage("blob", a, []byte("aaaa"))
	s.stage("blob", b, []byte("bb"))
	s.commit("blob", a, b)

	list, err := s.lfs.GetFileBlockOffsets(internal.GetFileBlockOffsetsOptions{Name: "blob"})
	s.Require().NoError(err)
	s.Equal(int64(common.BlockIDLength), list.BlockIdLength)
	s.Require().Len(list.BlockList, 2)
	s.Equal(a, list.BlockList[0].Id)
	s.Equal([]int64{0, 4}, []int64{list.BlockList[0].StartIndex, list.BlockList[0].EndIndex})
	s.Equal([]int64{4, 6}, []int64{list.BlockList[1].StartIndex, list.BlockList[1].EndIndex})

	s.commit("empty")
	list, err = s.lfs.GetFileBlockOffsets(internal.GetFileBlockOffsetsOptions{Name: "empty"})
	s.Require().NoError(err)
	s.True(list.HasNoBlocks())
	s.Equal(int64(common.BlockIDLength), list.BlockIdLength)

	list, err = s.lfs.GetFileBlockOffsets(internal.GetFileBlockOffsetsOptions{Name: "missing"})
	s.Equal(syscall.ENOENT, err)
	s.NotNil(list)
}

// block_cache stages the blocks of a file from many goroutines at once. Run with -race.
func (s *blocksTestSuite) TestConcurrentStaging() {
	const blocks = 32

	var wg sync.WaitGroup
	ids := make([]string, blocks)
	errs := make([]error, blocks)
	for i := range blocks {
		ids[i] = blockID(i)
		wg.Go(func() {
			errs[i] = s.lfs.StageData(internal.StageDataOptions{Name: "blob", Id: ids[i], Data: []byte(fmt.Sprintf("%02d", i))})
		})
	}
	wg.Wait()

	for _, err := range errs {
		s.Require().NoError(err)
	}

	s.commit("blob", ids...)

	var want strings.Builder
	for i := range blocks {
		fmt.Fprintf(&want, "%02d", i)
	}
	s.Equal(want.String(), string(s.content("blob")))
}
