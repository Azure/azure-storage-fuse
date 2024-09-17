/*
    _____           _____   _____   ____          ______  _____  ------
   |     |  |      |     | |     | |     |     | |       |            |
   |     |  |      |     | |     | |     |     | |       |            |
   | --- |  |      |     | |-----| |---- |     | |-----| |-----  ------
   |     |  |      |     | |     | |     |     |       | |       |
   | ____|  |_____ | ____| | ____| |     |_____|  _____| |_____  |_____

   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2020-2024 Microsoft Corporation. All rights reserved.
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

package stream

import (
	"context"
	"crypto/rand"
	"os"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/config"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal"
	"github.com/Azure/azure-storage-fuse/v2/internal/handlemap"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type streamTestSuite struct {
	suite.Suite
	assert   *assert.Assertions
	stream   *Stream
	mockCtrl *gomock.Controller
	mock     *internal.MockComponent
}

var wg = sync.WaitGroup{}
var emptyConfig = ""

// The four file keys to be tested against
var fileNames [4]string = [4]string{"file1", "file2"}

const MB = 1024 * 1024

// Helper methods for setup and getting options/data ========================================
func newTestStream(next internal.Component, configuration string, ro bool) (*Stream, error) {
	_ = config.ReadConfigFromReader(strings.NewReader(configuration))
	// we must be in read-only mode for read stream
	config.SetBool("read-only", ro)
	stream := NewStreamComponent()
	stream.SetNextComponent(next)
	err := stream.Configure(true)
	return stream.(*Stream), err
}

func (suite *streamTestSuite) setupTestHelper(config string, ro bool) {
	var err error
	suite.assert = assert.New(suite.T())
	suite.mockCtrl = gomock.NewController(suite.T())
	suite.mock = internal.NewMockComponent(suite.mockCtrl)
	suite.stream, err = newTestStream(suite.mock, config, ro)
	suite.assert.Equal(err, nil)
	_ = suite.stream.Start(context.Background())
}

func (suite *streamTestSuite) SetupTest() {
	err := log.SetDefaultLogger("silent", common.LogConfig{})
	if err != nil {
		panic("Unable to set silent logger as default.")
	}
	suite.setupTestHelper(emptyConfig, true)
}

func (suite *streamTestSuite) cleanupTest() {
	_ = suite.stream.Stop()
	suite.mockCtrl.Finish()
}

func (suite *streamTestSuite) getRequestOptions(fileIndex int, handle *handlemap.Handle, overwriteEndIndex bool, fileSize, offset, endIndex int64) (internal.OpenFileOptions, internal.ReadInBufferOptions, *[]byte) {
	var data []byte
	openFileOptions := internal.OpenFileOptions{Name: fileNames[fileIndex], Flags: os.O_RDONLY, Mode: os.FileMode(0777)}
	if !overwriteEndIndex {
		data = make([]byte, suite.stream.BlockSize)
	} else {
		data = make([]byte, endIndex-offset)
	}
	readInBufferOptions := internal.ReadInBufferOptions{Handle: handle, Offset: offset, Data: data}

	return openFileOptions, readInBufferOptions, &data
}

// return data buffer populated with data of the given size
func getBlockData(suite *streamTestSuite, size int) *[]byte {
	dataBuffer := make([]byte, size)
	_, _ = rand.Read(dataBuffer)
	return &dataBuffer
}

// return the block
func getCachedBlock(suite *streamTestSuite, offset int64, handle *handlemap.Handle) *common.Block {
	bk := offset
	blk, _ := handle.CacheObj.Get(bk)
	return blk
}

// Concurrency helpers with wait group terminations ========================================
func asyncReadInBuffer(suite *streamTestSuite, readInBufferOptions internal.ReadInBufferOptions) {
	_, _ = suite.stream.ReadInBuffer(readInBufferOptions)
	wg.Done()
}

func asyncOpenFile(suite *streamTestSuite, openFileOptions internal.OpenFileOptions) {
	_, _ = suite.stream.OpenFile(openFileOptions)
	wg.Done()
}

func asyncCloseFile(suite *streamTestSuite, closeFileOptions internal.CloseFileOptions) {
	_ = suite.stream.CloseFile(closeFileOptions)
	wg.Done()
}

// Assertion helpers  ========================================================================

// assert that the block is cached
func assertBlockCached(suite *streamTestSuite, offset int64, handle *handlemap.Handle) {
	_, found := handle.CacheObj.Get(offset)
	suite.assert.Equal(found, true)
}

// assert the block is not cached and KeyNotFoundError is thrown
func assertBlockNotCached(suite *streamTestSuite, offset int64, handle *handlemap.Handle) {
	_, found := handle.CacheObj.Get(offset)
	suite.assert.Equal(found, false)
}

func assertHandleNotStreamOnly(suite *streamTestSuite, handle *handlemap.Handle) {
	suite.assert.Equal(handle.CacheObj.StreamOnly, false)
}

func assertHandleStreamOnly(suite *streamTestSuite, handle *handlemap.Handle) {
	suite.assert.Equal(handle.CacheObj.StreamOnly, true)
}

func assertNumberOfCachedFileBlocks(suite *streamTestSuite, numOfBlocks int, handle *handlemap.Handle) {
	suite.assert.Equal(numOfBlocks, len(handle.CacheObj.Keys()))
}

// ====================================== End of helper methods =================================
// ====================================== Unit Tests ============================================
func (suite *streamTestSuite) TestDefault() {
	defer suite.cleanupTest()
	suite.assert.Equal("stream", suite.stream.Name())
	suite.assert.EqualValues(true, suite.stream.StreamOnly)
}

func (suite *streamTestSuite) TestConfig() {
	defer suite.cleanupTest()
	suite.cleanupTest()
	config := "stream:\n  block-size-mb: 4\n  buffer-size-mb: 16\n  max-buffers: 4\n"
	suite.setupTestHelper(config, true)

	suite.assert.Equal("stream", suite.stream.Name())
	suite.assert.Equal(16*MB, int(suite.stream.BufferSize))
	suite.assert.Equal(4, int(suite.stream.CachedObjLimit))
	suite.assert.EqualValues(false, suite.stream.StreamOnly)
	suite.assert.EqualValues(4*MB, suite.stream.BlockSize)

	// assert streaming is on if any of the values is 0
	suite.cleanupTest()
	config = "stream:\n  block-size-mb: 0\n  buffer-size-mb: 16\n  max-buffers: 4\n"
	suite.setupTestHelper(config, true)
	suite.assert.EqualValues(true, suite.stream.StreamOnly)
}

func (suite *streamTestSuite) TestReadWriteFile() {
	defer suite.cleanupTest()
	suite.cleanupTest()
	config := "stream:\n  block-size-mb: 4\n  buffer-size-mb: 16\n  max-buffers: 4\n"
	suite.setupTestHelper(config, true)

	// assert streaming is on if any of the values is 0
	suite.cleanupTest()
	config = "stream:\n  block-size-mb: 0\n  buffer-size-mb: 16\n  max-buffers: 4\n"
	suite.setupTestHelper(config, true)
	_, err := suite.stream.WriteFile(internal.WriteFileOptions{})
	suite.assert.Equal(syscall.ENOTSUP, err)
}

func (suite *streamTestSuite) TestReadTruncateFile() {
	defer suite.cleanupTest()
	suite.cleanupTest()
	config := "stream:\n  block-size-mb: 4\n  buffer-size-mb: 16\n  max-buffers: 4\n"
	suite.setupTestHelper(config, true)

	// assert streaming is on if any of the values is 0
	suite.cleanupTest()
	config = "stream:\n  block-size-mb: 0\n  buffer-size-mb: 16\n  max-buffers: 4\n"
	suite.setupTestHelper(config, true)
	err := suite.stream.TruncateFile(internal.TruncateFileOptions{})
	suite.assert.Equal(syscall.ENOTSUP, err)
}

func (suite *streamTestSuite) TestReadRenameFile() {
	defer suite.cleanupTest()
	suite.cleanupTest()
	config := "stream:\n  block-size-mb: 4\n  buffer-size-mb: 16\n  max-buffers: 4\n"
	suite.setupTestHelper(config, true)

	// assert streaming is on if any of the values is 0
	suite.cleanupTest()
	config = "stream:\n  block-size-mb: 0\n  buffer-size-mb: 16\n  max-buffers: 4\n"
	suite.setupTestHelper(config, true)
	err := suite.stream.RenameFile(internal.RenameFileOptions{})
	suite.assert.Equal(syscall.ENOTSUP, err)
}

func (suite *streamTestSuite) TestReadDeleteFile() {
	defer suite.cleanupTest()
	suite.cleanupTest()
	config := "stream:\n  block-size-mb: 4\n  buffer-size-mb: 16\n  max-buffers: 4\n"
	suite.setupTestHelper(config, true)

	// assert streaming is on if any of the values is 0
	suite.cleanupTest()
	config = "stream:\n  block-size-mb: 0\n  buffer-size-mb: 16\n  max-buffers: 4\n"
	suite.setupTestHelper(config, true)
	err := suite.stream.DeleteFile(internal.DeleteFileOptions{})
	suite.assert.Equal(syscall.ENOTSUP, err)
}

func (suite *streamTestSuite) TestFlushFile() {
	defer suite.cleanupTest()
	suite.cleanupTest()
	config := "stream:\n  block-size-mb: 4\n  buffer-size-mb: 16\n  max-buffers: 4\n"
	suite.setupTestHelper(config, true)
	handle1 := &handlemap.Handle{Size: 2, Path: fileNames[0]}
	flushFileOptions := internal.FlushFileOptions{Handle: handle1}

	err := suite.stream.FlushFile(flushFileOptions)
	suite.assert.Equal(nil, err)
}

func (suite *streamTestSuite) TestSyncFile() {
	defer suite.cleanupTest()
	suite.cleanupTest()
	config := "stream:\n  block-size-mb: 4\n  buffer-size-mb: 16\n  max-buffers: 4\n"
	suite.setupTestHelper(config, true)
	handle1 := &handlemap.Handle{Size: 2, Path: fileNames[0]}
	syncFileOptions := internal.SyncFileOptions{Handle: handle1}

	err := suite.stream.SyncFile(syncFileOptions)
	suite.assert.Equal(nil, err)
}

func (suite *streamTestSuite) TestReadDeleteDir() {
	defer suite.cleanupTest()
	suite.cleanupTest()
	config := "stream:\n  block-size-mb: 4\n  buffer-size-mb: 16\n  max-buffers: 4\n"
	suite.setupTestHelper(config, true)

	// assert streaming is on if any of the values is 0
	suite.cleanupTest()
	config = "stream:\n  block-size-mb: 0\n  buffer-size-mb: 16\n  max-buffers: 4\n"
	suite.setupTestHelper(config, true)
	err := suite.stream.DeleteDir(internal.DeleteDirOptions{})
	suite.assert.Equal(syscall.ENOTSUP, err)
}

func (suite *streamTestSuite) TestReadRenameDir() {
	defer suite.cleanupTest()
	suite.cleanupTest()
	config := "stream:\n  block-size-mb: 4\n  buffer-size-mb: 16\n  max-buffers: 4\n"
	suite.setupTestHelper(config, true)

	// assert streaming is on if any of the values is 0
	suite.cleanupTest()
	config = "stream:\n  block-size-mb: 0\n  buffer-size-mb: 16\n  max-buffers: 4\n"
	suite.setupTestHelper(config, true)
	err := suite.stream.RenameDir(internal.RenameDirOptions{})
	suite.assert.Equal(syscall.ENOTSUP, err)
}

func (suite *streamTestSuite) TestReadCreateFile() {
	defer suite.cleanupTest()
	suite.cleanupTest()
	config := "stream:\n  block-size-mb: 4\n  buffer-size-mb: 16\n  max-buffers: 4\n"
	suite.setupTestHelper(config, true)

	// assert streaming is on if any of the values is 0
	suite.cleanupTest()
	config = "stream:\n  block-size-mb: 0\n  buffer-size-mb: 16\n  max-buffers: 4\n"
	suite.setupTestHelper(config, true)
	_, err := suite.stream.CreateFile(internal.CreateFileOptions{})
	suite.assert.Equal(syscall.ENOTSUP, err)
}

func (suite *streamTestSuite) TestStreamOnlyError() {
	defer suite.cleanupTest()
	suite.cleanupTest()
	config := "stream:\n  block-size-mb: 0\n  buffer-size-mb: 16\n  max-buffers: 4\n"
	suite.setupTestHelper(config, true)
	// assert streaming is on if any of the values is 0
	suite.assert.EqualValues(true, suite.stream.StreamOnly)
	handle := &handlemap.Handle{Size: int64(100 * MB), Path: fileNames[0]}
	_, readInBufferOptions, _ := suite.getRequestOptions(0, handle, true, int64(100*MB), 0, 5)
	suite.mock.EXPECT().ReadInBuffer(readInBufferOptions).Return(0, syscall.ENOENT)
	_, err := suite.stream.ReadInBuffer(readInBufferOptions)
	suite.assert.Equal(err, syscall.ENOENT)
}

// Test file key gets cached on open and first block is prefetched
func (suite *streamTestSuite) TestCacheOnOpenFile() {
	defer suite.cleanupTest()
	suite.cleanupTest()
	config := "stream:\n  block-size-mb: 4\n  buffer-size-mb: 16\n  max-buffers: 3\n"
	suite.setupTestHelper(config, true)
	handle := &handlemap.Handle{Size: int64(100 * MB), Path: fileNames[0]}

	openFileOptions, readInBufferOptions, _ := suite.getRequestOptions(0, handle, false, int64(100*MB), 0, 0)
	suite.mock.EXPECT().OpenFile(openFileOptions).Return(handle, nil)
	suite.mock.EXPECT().ReadInBuffer(readInBufferOptions).Return(int(suite.stream.BlockSize), nil)
	_, _ = suite.stream.OpenFile(openFileOptions)

	assertBlockCached(suite, 0, handle)
	assertNumberOfCachedFileBlocks(suite, 1, handle)
}

// If open file returns error ensure nothing is cached and error is returned
func (suite *streamTestSuite) TestCacheOnOpenFileError() {
	defer suite.cleanupTest()
	suite.cleanupTest()
	config := "stream:\n  block-size-mb: 4\n  buffer-size-mb: 16\n  max-buffers: 3\n"
	suite.setupTestHelper(config, true)
	handle := &handlemap.Handle{Size: int64(100 * MB), Path: fileNames[0]}

	openFileOptions, _, _ := suite.getRequestOptions(0, handle, false, int64(100*MB), 0, 0)
	suite.mock.EXPECT().OpenFile(openFileOptions).Return(handle, syscall.ENOENT)
	_, err := suite.stream.OpenFile(openFileOptions)

	suite.assert.Equal(err, syscall.ENOENT)
}

// When we evict/remove all blocks of a given file the file should be no longer referenced in the cache
func (suite *streamTestSuite) TestFileKeyEviction() {
	defer suite.cleanupTest()
	suite.cleanupTest()
	// our config only fits one block - therefore with every open we purge the previous file cached
	config := "stream:\n  block-size-mb: 16\n  buffer-size-mb: 16\n  max-buffers: 4\n"
	suite.setupTestHelper(config, true)
	handle_1 := &handlemap.Handle{Size: int64(100 * MB), Path: fileNames[0]}
	handle_2 := &handlemap.Handle{Size: int64(100 * MB), Path: fileNames[1]}

	for i, handle := range []*handlemap.Handle{handle_1, handle_2} {
		openFileOptions, readInBufferOptions, _ := suite.getRequestOptions(i, handle, false, int64(100*MB), 0, 0)
		suite.mock.EXPECT().OpenFile(openFileOptions).Return(handle, nil)
		suite.mock.EXPECT().ReadInBuffer(readInBufferOptions).Return(int(suite.stream.BlockSize), nil)
		_, _ = suite.stream.OpenFile(openFileOptions)
		assertBlockCached(suite, 0, handle)
	}

	// since our configuration limits us to have one cached file at a time we expect to not have the first file key anymore
	assertBlockCached(suite, 0, handle_2)
	assertNumberOfCachedFileBlocks(suite, 1, handle_2)
}

func (suite *streamTestSuite) TestBlockEviction() {
	defer suite.cleanupTest()
	suite.cleanupTest()
	config := "stream:\n  block-size-mb: 16\n  buffer-size-mb: 16\n  max-buffers: 4\n"
	suite.setupTestHelper(config, true)
	handle := &handlemap.Handle{Size: int64(100 * MB), Path: fileNames[0]}

	openFileOptions, readInBufferOptions, _ := suite.getRequestOptions(0, handle, false, int64(100*MB), 0, 0)

	suite.mock.EXPECT().OpenFile(openFileOptions).Return(handle, nil)
	suite.mock.EXPECT().ReadInBuffer(readInBufferOptions).Return(int(suite.stream.BlockSize), nil)
	_, _ = suite.stream.OpenFile(openFileOptions)
	assertBlockCached(suite, 0, handle)

	_, readInBufferOptions, _ = suite.getRequestOptions(0, handle, false, int64(100*MB), 16*MB, 0)
	suite.mock.EXPECT().ReadInBuffer(readInBufferOptions).Return(int(suite.stream.BlockSize), nil)
	_, _ = suite.stream.ReadInBuffer(readInBufferOptions)

	// we expect our first block to have been evicted
	assertBlockNotCached(suite, 0, handle)
	assertBlockCached(suite, 16*MB, handle)
	assertNumberOfCachedFileBlocks(suite, 1, handle)
}

// Test handle tracking by opening/closing a file multiple times
func (suite *streamTestSuite) TestHandles() {
	defer suite.cleanupTest()
	suite.cleanupTest()
	config := "stream:\n  block-size-mb: 16\n  buffer-size-mb: 16\n  max-buffers: 4\n"
	suite.setupTestHelper(config, true)
	handle := &handlemap.Handle{Size: int64(100 * MB), Path: fileNames[0]}

	openFileOptions, readInBufferOptions, _ := suite.getRequestOptions(0, handle, false, int64(100*MB), 0, 0)
	closeFileOptions := internal.CloseFileOptions{Handle: handle}
	suite.mock.EXPECT().OpenFile(openFileOptions).Return(handle, nil)
	suite.mock.EXPECT().ReadInBuffer(readInBufferOptions).Return(int(suite.stream.BlockSize), nil)
	_, _ = suite.stream.OpenFile(openFileOptions)

	suite.mock.EXPECT().CloseFile(closeFileOptions).Return(nil)
	_ = suite.stream.CloseFile(closeFileOptions)

	// we expect to call read in buffer again since we cleaned the cache after the file was closed
	suite.mock.EXPECT().OpenFile(openFileOptions).Return(handle, nil)
	suite.mock.EXPECT().ReadInBuffer(readInBufferOptions).Return(int(suite.stream.BlockSize), nil)
	_, _ = suite.stream.OpenFile(openFileOptions)
}

func (suite *streamTestSuite) TestStreamOnlyHandleLimit() {
	defer suite.cleanupTest()
	suite.cleanupTest()
	config := "stream:\n  block-size-mb: 16\n  buffer-size-mb: 16\n  max-buffers: 1\n"
	suite.setupTestHelper(config, true)
	handle1 := &handlemap.Handle{Size: int64(100 * MB), Path: fileNames[0]}
	handle2 := &handlemap.Handle{Size: int64(100 * MB), Path: fileNames[0]}
	handle3 := &handlemap.Handle{Size: int64(100 * MB), Path: fileNames[0]}

	openFileOptions, readInBufferOptions, _ := suite.getRequestOptions(0, handle1, false, int64(100*MB), 0, 0)
	closeFileOptions := internal.CloseFileOptions{Handle: handle1}
	suite.mock.EXPECT().OpenFile(openFileOptions).Return(handle1, nil)
	suite.mock.EXPECT().ReadInBuffer(readInBufferOptions).Return(int(suite.stream.BlockSize), nil)
	_, _ = suite.stream.OpenFile(openFileOptions)
	assertHandleNotStreamOnly(suite, handle1)

	suite.mock.EXPECT().OpenFile(openFileOptions).Return(handle2, nil)
	_, _ = suite.stream.OpenFile(openFileOptions)
	assertHandleStreamOnly(suite, handle2)

	suite.mock.EXPECT().CloseFile(closeFileOptions).Return(nil)
	_ = suite.stream.CloseFile(closeFileOptions)

	// we expect to call read in buffer again since we cleaned the cache after the file was closed
	suite.mock.EXPECT().OpenFile(openFileOptions).Return(handle3, nil)
	readInBufferOptions.Handle = handle3
	suite.mock.EXPECT().ReadInBuffer(readInBufferOptions).Return(int(suite.stream.BlockSize), nil)
	_, _ = suite.stream.OpenFile(openFileOptions)
	assertHandleNotStreamOnly(suite, handle3)
}

// Get data that spans two blocks - we expect to have two blocks stored at the end
func (suite *streamTestSuite) TestBlockDataOverlap() {
	defer suite.cleanupTest()
	suite.cleanupTest()
	config := "stream:\n  block-size-mb: 16\n  buffer-size-mb: 32\n  max-buffers: 4\n"
	suite.setupTestHelper(config, true)
	handle := &handlemap.Handle{Size: int64(100 * MB), Path: fileNames[0]}

	openFileOptions, readInBufferOptions, _ := suite.getRequestOptions(0, handle, false, int64(100*MB), 0, 0)

	suite.mock.EXPECT().OpenFile(openFileOptions).Return(handle, nil)
	suite.mock.EXPECT().ReadInBuffer(readInBufferOptions).Return(int(suite.stream.BlockSize), nil)
	_, _ = suite.stream.OpenFile(openFileOptions)
	assertBlockCached(suite, 0, handle)

	// options of our request from the stream component
	_, userReadInBufferOptions, _ := suite.getRequestOptions(0, handle, true, int64(100*MB), 1*MB, 17*MB)
	// options the stream component should request for the second block
	_, streamMissingBlockReadInBufferOptions, _ := suite.getRequestOptions(0, handle, false, int64(100*MB), 16*MB, 0)
	suite.mock.EXPECT().ReadInBuffer(streamMissingBlockReadInBufferOptions).Return(int(16*MB), nil)
	_, _ = suite.stream.ReadInBuffer(userReadInBufferOptions)

	// 	we expect 0-16MB, and 16MB-32MB be cached since our second request is at offset 1MB

	assertBlockCached(suite, 0, handle)
	assertBlockCached(suite, 16*MB, handle)
	assertNumberOfCachedFileBlocks(suite, 2, handle)
}

func (suite *streamTestSuite) TestFileSmallerThanBlockSize() {
	defer suite.cleanupTest()
	suite.cleanupTest()
	config := "stream:\n  block-size-mb: 16\n  buffer-size-mb: 16\n  max-buffers: 4\n"
	suite.setupTestHelper(config, true)
	handle := &handlemap.Handle{Size: int64(15 * MB), Path: fileNames[0]}

	// case1: we know the size of the file from the get go, 15MB - smaller than our block size
	openFileOptions, readInBufferOptions, _ := suite.getRequestOptions(0, handle, true, int64(15*MB), 0, 15*MB)

	suite.mock.EXPECT().OpenFile(openFileOptions).Return(handle, nil)
	// we expect our request to be 15MB
	suite.mock.EXPECT().ReadInBuffer(readInBufferOptions).Return(int(15*MB), nil)
	_, _ = suite.stream.OpenFile(openFileOptions)

	assertBlockCached(suite, 0, handle)
	blk := getCachedBlock(suite, 0, handle)
	suite.assert.Equal(int64(15*MB), blk.EndIndex)

	// TODO: case2: file size changed in next component without stream being updated and therefore we get EOF
}

func (suite *streamTestSuite) TestEmptyFile() {
	defer suite.cleanupTest()
	suite.cleanupTest()
	config := "stream:\n  block-size-mb: 16\n  buffer-size-mb: 16\n  max-buffers: 4\n"
	suite.setupTestHelper(config, true)
	handle := &handlemap.Handle{Size: 0, Path: fileNames[0]}

	// case1: we know the size of the file from the get go, 0
	openFileOptions, readInBufferOptions, _ := suite.getRequestOptions(0, handle, true, int64(0), 0, 0)

	suite.mock.EXPECT().OpenFile(openFileOptions).Return(handle, nil)
	// we expect our request to be 0
	suite.mock.EXPECT().ReadInBuffer(readInBufferOptions).Return(int(0), nil)
	_, _ = suite.stream.OpenFile(openFileOptions)

	assertBlockCached(suite, 0, handle)
	blk := getCachedBlock(suite, 0, handle)
	suite.assert.Equal(int64(0), blk.EndIndex)
}

// When we stop the component we expect everything to be deleted
func (suite *streamTestSuite) TestCachePurge() {
	defer suite.cleanupTest()
	suite.cleanupTest()
	config := "stream:\n  block-size-mb: 4\n  buffer-size-mb: 16\n  max-buffers: 4\n"
	suite.setupTestHelper(config, true)
	handle_1 := &handlemap.Handle{Size: int64(100 * MB), Path: fileNames[0]}
	handle_2 := &handlemap.Handle{Size: int64(100 * MB), Path: fileNames[1]}

	for i, handle := range []*handlemap.Handle{handle_1, handle_2} {
		openFileOptions, readInBufferOptions, _ := suite.getRequestOptions(i, handle, false, int64(100*MB), 0, 0)

		suite.mock.EXPECT().OpenFile(openFileOptions).Return(handle, nil)
		suite.mock.EXPECT().ReadInBuffer(readInBufferOptions).Return(int(suite.stream.BlockSize), nil)
		_, _ = suite.stream.OpenFile(openFileOptions)
		assertBlockCached(suite, 0, handle)
	}

	_ = suite.stream.Stop()
	assertBlockCached(suite, 0, handle_1)
	assertBlockCached(suite, 0, handle_2)
}

// Data sanity check
func (suite *streamTestSuite) TestCachedData() {
	defer suite.cleanupTest()
	suite.cleanupTest()
	config := "stream:\n  block-size-mb: 16\n  buffer-size-mb: 32\n  max-buffers: 4\n"
	suite.setupTestHelper(config, true)
	var dataBuffer *[]byte
	var readInBufferOptions internal.ReadInBufferOptions
	handle_1 := &handlemap.Handle{Size: int64(32 * MB), Path: fileNames[0]}

	data := *getBlockData(suite, 32*MB)
	for _, off := range []int64{0, 16} {

		openFileOptions, readInBufferOptions, _ := suite.getRequestOptions(0, handle_1, false, int64(32*MB), off*MB, 0)

		if off == 0 {
			suite.mock.EXPECT().OpenFile(openFileOptions).Return(handle_1, nil)
			suite.mock.EXPECT().ReadInBuffer(readInBufferOptions).Return(int(suite.stream.BlockSize), nil)
			_, _ = suite.stream.OpenFile(openFileOptions)
		} else {
			suite.mock.EXPECT().ReadInBuffer(readInBufferOptions).Return(int(suite.stream.BlockSize), nil)
			_, _ = suite.stream.ReadInBuffer(readInBufferOptions)
		}

		assertBlockCached(suite, off*MB, handle_1)
		block := getCachedBlock(suite, off*MB, handle_1)
		block.Data = data[off*MB : off*MB+suite.stream.BlockSize]

	}
	// now let's assert that it doesn't call next component and that the data retrieved is accurate
	// case1: data within a cached block
	_, readInBufferOptions, dataBuffer = suite.getRequestOptions(0, handle_1, true, int64(32*MB), int64(2*MB), int64(3*MB))
	_, _ = suite.stream.ReadInBuffer(readInBufferOptions)
	suite.assert.Equal(data[2*MB:3*MB], *dataBuffer)

	// case2: data cached within two blocks
	_, readInBufferOptions, dataBuffer = suite.getRequestOptions(0, handle_1, true, int64(32*MB), int64(14*MB), int64(20*MB))
	_, _ = suite.stream.ReadInBuffer(readInBufferOptions)
	suite.assert.Equal(data[14*MB:20*MB], *dataBuffer)
}

// This test does a data sanity check in the case where concurrent read is happening and causes evicitons
func (suite *streamTestSuite) TestAsyncReadAndEviction() {
	defer suite.cleanupTest()
	suite.cleanupTest()
	config := "stream:\n  block-size-mb: 4\n  buffer-size-mb: 16\n  max-buffers: 4\n"
	suite.setupTestHelper(config, true)

	var blockOneDataBuffer *[]byte
	var blockTwoDataBuffer *[]byte
	var readInBufferOptions internal.ReadInBufferOptions
	handle_1 := &handlemap.Handle{Size: int64(16 * MB), Path: fileNames[0]}

	// Even though our file size is 16MB below we only check against 8MB of the data (we check against two blocks)
	data := *getBlockData(suite, 8*MB)
	for _, off := range []int64{0, 4} {
		openFileOptions, readInBufferOptions, _ := suite.getRequestOptions(0, handle_1, false, int64(16*MB), off*MB, 0)
		if off == 0 {
			suite.mock.EXPECT().OpenFile(openFileOptions).Return(handle_1, nil)
			suite.mock.EXPECT().ReadInBuffer(readInBufferOptions).Return(int(suite.stream.BlockSize), nil)
			_, _ = suite.stream.OpenFile(openFileOptions)
		} else {
			suite.mock.EXPECT().ReadInBuffer(readInBufferOptions).Return(int(suite.stream.BlockSize), nil)
			_, _ = suite.stream.ReadInBuffer(readInBufferOptions)
		}

		assertBlockCached(suite, off*MB, handle_1)
		block := getCachedBlock(suite, off*MB, handle_1)
		block.Data = data[off*MB : off*MB+suite.stream.BlockSize]
	}
	// test concurrent data access to the same file
	// call 1: data within a cached block
	_, readInBufferOptions, blockOneDataBuffer = suite.getRequestOptions(0, handle_1, true, int64(16*MB), int64(2*MB), int64(3*MB))
	_, _ = suite.stream.ReadInBuffer(readInBufferOptions)
	wg.Add(2)

	// call 2: data cached within two blocks
	_, readInBufferOptions, blockTwoDataBuffer = suite.getRequestOptions(0, handle_1, true, int64(16*MB), int64(3*MB), int64(6*MB))
	go asyncReadInBuffer(suite, readInBufferOptions)
	// wait a little so we can guarantee block offset 0 is evicted
	time.Sleep(2 * time.Second)

	// call 3: get missing block causing an eviction to block 1 with offset 0 - this ensures our data from block 1 is still copied correctly
	_, readInBufferOptions, _ = suite.getRequestOptions(0, handle_1, false, int64(16*MB), int64(12*MB), 0)
	suite.mock.EXPECT().ReadInBuffer(readInBufferOptions).Return(int(suite.stream.BlockSize), nil)
	go asyncReadInBuffer(suite, readInBufferOptions)
	wg.Wait()

	// assert data within first block is correct
	suite.assert.Equal(data[2*MB:3*MB], *blockOneDataBuffer)
	// assert data between two blocks is correct
	suite.assert.Equal(data[3*MB:6*MB], *blockTwoDataBuffer)
	// assert we did in fact evict the first block and have added the third block
	assertBlockCached(suite, 0, handle_1)
	assertBlockCached(suite, 12*MB, handle_1)
}

// This tests concurrent open and ensuring the number of handles and cached blocks is handled correctly
func (suite *streamTestSuite) TestAsyncOpen() {
	defer suite.cleanupTest()
	suite.cleanupTest()
	config := "stream:\n  block-size-mb: 4\n  buffer-size-mb: 16\n  max-buffers: 4\n"
	suite.setupTestHelper(config, true)
	handle_1 := &handlemap.Handle{Size: int64(100 * MB), Path: fileNames[0]}
	handle_2 := &handlemap.Handle{Size: int64(100 * MB), Path: fileNames[1]}

	// Open four files concurrently - each doing a readInBuffer call to store the first block
	for i, handle := range []*handlemap.Handle{handle_1, handle_2} {
		openFileOptions, readInBufferOptions, _ := suite.getRequestOptions(i, handle, false, int64(100*MB), 0, 0)
		suite.mock.EXPECT().OpenFile(openFileOptions).Return(handle, nil)
		suite.mock.EXPECT().ReadInBuffer(readInBufferOptions).Return(int(suite.stream.BlockSize), nil)
		wg.Add(1)
		go asyncOpenFile(suite, openFileOptions)
	}
	wg.Wait()

	for _, handle := range []*handlemap.Handle{handle_1, handle_2} {
		assertBlockCached(suite, 0, handle)
		assertNumberOfCachedFileBlocks(suite, 1, handle)
	}
}

func (suite *streamTestSuite) TestAsyncClose() {
	defer suite.cleanupTest()
	suite.cleanupTest()
	config := "stream:\n  block-size-mb: 4\n  buffer-size-mb: 16\n  max-buffers: 4\n"
	suite.setupTestHelper(config, true)
	handle_1 := &handlemap.Handle{Size: int64(100 * MB), Path: fileNames[0]}
	handle_2 := &handlemap.Handle{Size: int64(100 * MB), Path: fileNames[1]}

	for i, handle := range []*handlemap.Handle{handle_1, handle_2} {
		openFileOptions, readInBufferOptions, _ := suite.getRequestOptions(i, handle, false, int64(100*MB), 0, 0)
		suite.mock.EXPECT().OpenFile(openFileOptions).Return(handle, nil)
		suite.mock.EXPECT().ReadInBuffer(readInBufferOptions).Return(int(suite.stream.BlockSize), nil)
		wg.Add(1)
		go asyncOpenFile(suite, openFileOptions)
	}
	wg.Wait()

	for _, handle := range []*handlemap.Handle{handle_1, handle_2} {
		closeFileOptions := internal.CloseFileOptions{Handle: handle}
		suite.mock.EXPECT().CloseFile(closeFileOptions).Return(nil)
		wg.Add(1)
		go asyncCloseFile(suite, closeFileOptions)
	}
	wg.Wait()
}

func TestStreamTestSuite(t *testing.T) {
	suite.Run(t, new(streamTestSuite))
}
