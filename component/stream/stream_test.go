// /*
//     _____           _____   _____   ____          ______  _____  ------
//    |     |  |      |     | |     | |     |     | |       |            |
//    |     |  |      |     | |     | |     |     | |       |            |
//    | --- |  |      |     | |-----| |---- |     | |-----| |-----  ------
//    |     |  |      |     | |     | |     |     |       | |       |
//    | ____|  |_____ | ____| | ____| |     |_____|  _____| |_____  |_____

//    Licensed under the MIT License <http://opensource.org/licenses/MIT>.

//    Copyright © 2020-2022 Microsoft Corporation. All rights reserved.
//    Author : <blobfusedev@microsoft.com>

//    Permission is hereby granted, free of charge, to any person obtaining a copy
//    of this software and associated documentation files (the "Software"), to deal
//    in the Software without restriction, including without limitation the rights
//    to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
//    copies of the Software, and to permit persons to whom the Software is
//    furnished to do so, subject to the following conditions:

//    The above copyright notice and this permission notice shall be included in all
//    copies or substantial portions of the Software.

//    THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
//    IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
//    FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
//    AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
//    LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
//    OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
//    SOFTWARE
// */

// package stream

// import (
// 	"blobfuse2/common"
// 	"blobfuse2/common/config"
// 	"blobfuse2/common/log"
// 	"blobfuse2/internal"
// 	"blobfuse2/internal/handlemap"
// 	"context"
// 	"crypto/rand"
// 	"os"
// 	"strings"
// 	"sync"
// 	"syscall"
// 	"testing"
// 	"time"

// 	"github.com/bluele/gcache"
// 	"github.com/golang/mock/gomock"
// 	"github.com/stretchr/testify/assert"
// 	"github.com/stretchr/testify/suite"
// )

// type streamTestSuite struct {
// 	suite.Suite
// 	assert   *assert.Assertions
// 	stream   *Stream
// 	mockCtrl *gomock.Controller
// 	mock     *internal.MockComponent
// }

// var wg = sync.WaitGroup{}
// var emptyConfig = ""

// // The four file keys to be tested against
// var fileNames [4]string = [4]string{"file1", "file2"}

// const MB = 1024 * 1024

// // Helper methods for setup and getting options/data ========================================
// func newTestStream(next internal.Component, configuration string) *Stream {
// 	config.ReadConfigFromReader(strings.NewReader(configuration))
// 	// we must be in read-only mode for read stream
// 	config.SetBool("read-only", true)
// 	stream := NewStreamComponent()
// 	stream.SetNextComponent(next)
// 	stream.Configure()

// 	return stream.(*Stream)
// }

// func (suite *streamTestSuite) setupTestHelper(config string) {
// 	suite.assert = assert.New(suite.T())
// 	suite.mockCtrl = gomock.NewController(suite.T())
// 	suite.mock = internal.NewMockComponent(suite.mockCtrl)
// 	suite.stream = newTestStream(suite.mock, config)
// 	suite.stream.Start(context.Background())
// }

// func (suite *streamTestSuite) SetupTest() {
// 	err := log.SetDefaultLogger("silent", common.LogConfig{})
// 	if err != nil {
// 		panic("Unable to set silent logger as default.")
// 	}
// 	suite.setupTestHelper(emptyConfig)
// }

// func (suite *streamTestSuite) cleanupTest() {
// 	suite.stream.Stop()
// 	suite.mockCtrl.Finish()
// }

// func (suite *streamTestSuite) getRequestOptions(fileIndex int, handle *handlemap.Handle, overwriteEndIndex bool, fileSize, offset, endIndex int64) (internal.OpenFileOptions, internal.ReadInBufferOptions, *[]byte) {
// 	var data []byte
// 	openFileOptions := internal.OpenFileOptions{Name: fileNames[fileIndex], Flags: os.O_RDONLY, Mode: os.FileMode(0777)}
// 	if !overwriteEndIndex {
// 		data = make([]byte, suite.stream.cache.blockSize)
// 	} else {
// 		data = make([]byte, endIndex-offset)
// 	}
// 	readInBufferOptions := internal.ReadInBufferOptions{Handle: handle, Offset: offset, Data: data}

// 	return openFileOptions, readInBufferOptions, &data
// }

// // return data buffer populated with data of the given size
// func getBlockData(suite *streamTestSuite, size int) *[]byte {
// 	dataBuffer := make([]byte, size)
// 	rand.Read(dataBuffer)
// 	return &dataBuffer
// }

// // return the block
// func getCachedBlock(suite *streamTestSuite, offset int64, handle *handlemap.Handle) *cacheBlock {
// 	bk := blockKey{offset, handle}
// 	blk, _ := suite.stream.cache.blocks.Get(bk)
// 	return blk.(*cacheBlock)
// }

// // return the block from persistence cache
// func diskBlockExists(suite *streamTestSuite, offset int64, handle *handlemap.Handle) bool {
// 	bk := blockKey{offset, handle}
// 	blk := &cacheBlock{}
// 	return suite.stream.cache.getBlockFromDisk(blk, bk)
// }

// // Concurrency helpers with wait group terminations ========================================
// func asyncReadInBuffer(suite *streamTestSuite, readInBufferOptions internal.ReadInBufferOptions) {
// 	suite.stream.ReadInBuffer(readInBufferOptions)
// 	wg.Done()
// }

// func asyncOpenFile(suite *streamTestSuite, openFileOptions internal.OpenFileOptions) {
// 	suite.stream.OpenFile(openFileOptions)
// 	wg.Done()
// }

// func asyncCloseFile(suite *streamTestSuite, closeFileOptions internal.CloseFileOptions) {
// 	suite.stream.CloseFile(closeFileOptions)
// 	wg.Done()
// }

// // Assertion helpers  ========================================================================

// //assert that the block is cached
// func assertBlockCached(suite *streamTestSuite, offset int64, handle *handlemap.Handle) {
// 	bk := blockKey{offset, handle}
// 	_, err := handle.CacheObj.DataBuffer.Get(bk)
// 	suite.assert.NoError(err)
// 	_, err = suite.stream.cache.blocks.Get(bk)
// 	suite.assert.NoError(err)
// }

// //assert the block is not cached and KeyNotFoundError is thrown
// func assertBlockNotCached(suite *streamTestSuite, offset int64, handle *handlemap.Handle) {
// 	bk := blockKey{offset, handle}
// 	_, err := handle.CacheObj.DataBuffer.Get(bk)
// 	suite.assert.EqualError(err, gcache.KeyNotFoundError.Error())
// 	_, err = suite.stream.cache.blocks.Get(bk)
// 	suite.assert.EqualError(err, gcache.KeyNotFoundError.Error())
// }

// func assertNumberOfCachedBlocks(suite *streamTestSuite, numOfBlocks int) {
// 	suite.assert.Equal(numOfBlocks, suite.stream.cache.blocks.Len(false))
// }

// func assertNumberOfCachedFileBlocks(suite *streamTestSuite, numOfBlocks int, handle *handlemap.Handle) {
// 	suite.assert.Equal(numOfBlocks, handle.CacheObj.DataBuffer.Len(false))
// }

// // ====================================== End of helper methods =================================
// // ====================================== Unit Tests ============================================
// func (suite *streamTestSuite) TestDefault() {
// 	defer suite.cleanupTest()
// 	suite.assert.Equal("stream", suite.stream.Name())
// 	suite.assert.EqualValues(true, suite.stream.streamOnly)
// }

// func (suite *streamTestSuite) TestConfig() {
// 	defer suite.cleanupTest()
// 	suite.cleanupTest()
// 	config := "stream:\n  block-size-mb: 16\n  blocks-per-file: 4\n  cache-size-mb: 64\n"
// 	suite.setupTestHelper(config)

// 	suite.assert.Equal("stream", suite.stream.Name())
// 	suite.assert.Equal(4, suite.stream.cache.bufferSizePerHandle)
// 	suite.assert.Equal(4, suite.stream.cache.cacheSize)
// 	suite.assert.EqualValues(false, suite.stream.streamOnly)
// 	suite.assert.EqualValues(16*MB, suite.stream.cache.blockSize)
// 	suite.assert.IsType(&gcache.LRUCache{}, suite.stream.cache.blocks)

// 	// assert streaming is on if any of the values is 0
// 	suite.cleanupTest()
// 	config = "stream:\n  block-size-mb: 0\n  blocks-per-file: 4\n  cache-size-mb: 64\n  policy: lfu"
// 	suite.setupTestHelper(config)
// 	suite.assert.EqualValues(true, suite.stream.streamOnly)
// }

// // Test eviction policy is set correctly depending on configuration
// func (suite *streamTestSuite) TestEvictionPolicy() {
// 	defer suite.cleanupTest()
// 	for _, policy := range []string{"lru", "lfu", "arc"} {
// 		suite.cleanupTest()
// 		config := "stream:\n  block-size-mb: 16\n  blocks-per-file: 4\n  cache-size-mb: 64\n  policy: " + policy
// 		suite.setupTestHelper(config)
// 		if policy == "lru" {
// 			suite.assert.IsType(&gcache.LRUCache{}, suite.stream.cache.blocks)
// 		} else if policy == "lfu" {
// 			suite.assert.IsType(&gcache.LFUCache{}, suite.stream.cache.blocks)
// 		} else if policy == "arc" {
// 			suite.assert.IsType(&gcache.ARC{}, suite.stream.cache.blocks)
// 		}
// 	}
// }

// func (suite *streamTestSuite) TestStreamOnlyError() {
// 	defer suite.cleanupTest()
// 	suite.cleanupTest()
// 	config := "stream:\n  block-size-mb: 0\n  blocks-per-file: 4\n  cache-size-mb: 64\n  policy: lfu"
// 	suite.setupTestHelper(config)
// 	// assert streaming is on if any of the values is 0
// 	suite.assert.EqualValues(true, suite.stream.streamOnly)
// 	handle := &handlemap.Handle{Size: int64(100 * MB), Path: fileNames[0]}
// 	_, readInBufferOptions, _ := suite.getRequestOptions(0, handle, true, int64(100*MB), 0, 5)
// 	suite.mock.EXPECT().ReadInBuffer(readInBufferOptions).Return(0, syscall.ENOENT)
// 	_, err := suite.stream.ReadInBuffer(readInBufferOptions)
// 	suite.assert.Equal(err, syscall.ENOENT)
// }

// // Test file key gets cached on open and first block is prefetched
// func (suite *streamTestSuite) TestCacheOnOpenFile() {
// 	defer suite.cleanupTest()
// 	suite.cleanupTest()
// 	config := "stream:\n  block-size-mb: 16\n  blocks-per-file: 4\n  cache-size-mb: 64\n  policy: lru"
// 	suite.setupTestHelper(config)
// 	handle := &handlemap.Handle{Size: int64(100 * MB), Path: fileNames[0]}

// 	openFileOptions, readInBufferOptions, _ := suite.getRequestOptions(0, handle, false, int64(100*MB), 0, 0)
// 	suite.mock.EXPECT().OpenFile(openFileOptions).Return(handle, nil)
// 	suite.mock.EXPECT().ReadInBuffer(readInBufferOptions).Return(int(suite.stream.cache.blockSize), nil)
// 	suite.stream.OpenFile(openFileOptions)

// 	assertNumberOfCachedBlocks(suite, 1)
// 	assertBlockCached(suite, 0, handle)
// 	assertNumberOfCachedFileBlocks(suite, 1, handle)
// }

// // If open file returns error ensure nothing is cached and error is returned
// func (suite *streamTestSuite) TestCacheOnOpenFileError() {
// 	defer suite.cleanupTest()
// 	suite.cleanupTest()
// 	config := "stream:\n  block-size-mb: 16\n  blocks-per-file: 4\n  cache-size-mb: 64\n  policy: lru"
// 	suite.setupTestHelper(config)
// 	handle := &handlemap.Handle{Size: int64(100 * MB), Path: fileNames[0]}

// 	openFileOptions, _, _ := suite.getRequestOptions(0, handle, false, int64(100*MB), 0, 0)
// 	suite.mock.EXPECT().OpenFile(openFileOptions).Return(handle, syscall.ENOENT)
// 	_, err := suite.stream.OpenFile(openFileOptions)

// 	suite.assert.Equal(err, syscall.ENOENT)
// }

// // When we evict/remove all blocks of a given file the file should be no longer referenced in the cache
// func (suite *streamTestSuite) TestFileKeyEviction() {
// 	defer suite.cleanupTest()
// 	suite.cleanupTest()
// 	// our config only fits one block - therefore with every open we purge the previous file cached
// 	config := "stream:\n  block-size-mb: 16\n  blocks-per-file: 1\n  cache-size-mb: 16\n  policy: lru"
// 	suite.setupTestHelper(config)
// 	handle_1 := &handlemap.Handle{Size: int64(100 * MB), Path: fileNames[0]}
// 	handle_2 := &handlemap.Handle{Size: int64(100 * MB), Path: fileNames[1]}

// 	for i, handle := range []*handlemap.Handle{handle_1, handle_2} {
// 		openFileOptions, readInBufferOptions, _ := suite.getRequestOptions(i, handle, false, int64(100*MB), 0, 0)
// 		suite.mock.EXPECT().OpenFile(openFileOptions).Return(handle, nil)
// 		suite.mock.EXPECT().ReadInBuffer(readInBufferOptions).Return(int(suite.stream.cache.blockSize), nil)
// 		suite.stream.OpenFile(openFileOptions)
// 		assertBlockCached(suite, 0, handle)
// 	}

// 	// since our configuration limits us to have one cached file at a time we expect to not have the first file key anymore
// 	assertBlockCached(suite, 0, handle_2)
// 	assertNumberOfCachedBlocks(suite, 1)
// 	assertNumberOfCachedFileBlocks(suite, 1, handle_2)
// }

// func (suite *streamTestSuite) TestBlockEviction() {
// 	defer suite.cleanupTest()
// 	suite.cleanupTest()
// 	config := "stream:\n  block-size-mb: 16\n  blocks-per-file: 1\n  cache-size-mb: 16\n  policy: lru"
// 	suite.setupTestHelper(config)
// 	handle := &handlemap.Handle{Size: int64(100 * MB), Path: fileNames[0]}

// 	openFileOptions, readInBufferOptions, _ := suite.getRequestOptions(0, handle, false, int64(100*MB), 0, 0)

// 	suite.mock.EXPECT().OpenFile(openFileOptions).Return(handle, nil)
// 	suite.mock.EXPECT().ReadInBuffer(readInBufferOptions).Return(int(suite.stream.cache.blockSize), nil)
// 	suite.stream.OpenFile(openFileOptions)
// 	assertBlockCached(suite, 0, handle)

// 	_, readInBufferOptions, _ = suite.getRequestOptions(0, handle, false, int64(100*MB), 16*MB, 0)
// 	suite.mock.EXPECT().ReadInBuffer(readInBufferOptions).Return(int(suite.stream.cache.blockSize), nil)
// 	suite.stream.ReadInBuffer(readInBufferOptions)

// 	// we expect our first block to have been evicted
// 	assertNumberOfCachedBlocks(suite, 1)
// 	assertBlockNotCached(suite, 0, handle)
// 	assertBlockCached(suite, 16*MB, handle)
// 	assertNumberOfCachedFileBlocks(suite, 1, handle)
// }

// // Test handle tracking by opening/closing a file multiple times
// func (suite *streamTestSuite) TestHandles() {
// 	defer suite.cleanupTest()
// 	suite.cleanupTest()
// 	config := "stream:\n  block-size-mb: 16\n  blocks-per-file: 4\n  cache-size-mb: 64\n  policy: lfu"
// 	suite.setupTestHelper(config)
// 	handle := &handlemap.Handle{Size: int64(100 * MB), Path: fileNames[0]}

// 	openFileOptions, readInBufferOptions, _ := suite.getRequestOptions(0, handle, false, int64(100*MB), 0, 0)
// 	closeFileOptions := internal.CloseFileOptions{Handle: handle}
// 	suite.mock.EXPECT().OpenFile(openFileOptions).Return(handle, nil)
// 	suite.mock.EXPECT().ReadInBuffer(readInBufferOptions).Return(int(suite.stream.cache.blockSize), nil)
// 	suite.stream.OpenFile(openFileOptions)

// 	suite.mock.EXPECT().CloseFile(closeFileOptions).Return(nil)
// 	suite.stream.CloseFile(closeFileOptions)

// 	// we expect to call read in buffer again since we cleaned the cache after the file was closed
// 	suite.mock.EXPECT().OpenFile(openFileOptions).Return(handle, nil)
// 	suite.mock.EXPECT().ReadInBuffer(readInBufferOptions).Return(int(suite.stream.cache.blockSize), nil)
// 	suite.stream.OpenFile(openFileOptions)
// }

// func (suite *streamTestSuite) TestBlocksPerFileLargerThanCacheSize() {
// 	defer suite.cleanupTest()
// 	suite.cleanupTest()
// 	config := "stream:\n  block-size-mb: 16\n  blocks-per-file: 20\n  cache-size-mb: 16\n  policy: lru"
// 	suite.setupTestHelper(config)
// 	handle_1 := &handlemap.Handle{Size: int64(100 * MB), Path: fileNames[0]}
// 	handle_2 := &handlemap.Handle{Size: int64(100 * MB), Path: fileNames[1]}

// 	openFileOptions, readInBufferOptions, _ := suite.getRequestOptions(0, handle_1, false, int64(100*MB), 0, 0)
// 	suite.mock.EXPECT().OpenFile(openFileOptions).Return(handle_1, nil)
// 	suite.mock.EXPECT().ReadInBuffer(readInBufferOptions).Return(int(suite.stream.cache.blockSize), nil)
// 	suite.stream.OpenFile(openFileOptions)
// 	assertNumberOfCachedBlocks(suite, 1)

// 	for _, off := range []int64{16, 32} {
// 		_, readInBufferOptions, _ = suite.getRequestOptions(0, handle_1, false, int64(100*MB), off*MB, 0)
// 		suite.mock.EXPECT().ReadInBuffer(readInBufferOptions).Return(int(suite.stream.cache.blockSize), nil)
// 		suite.stream.ReadInBuffer(readInBufferOptions)
// 		assertNumberOfCachedBlocks(suite, 1)
// 		assertNumberOfCachedFileBlocks(suite, 1, handle_1)
// 	}
// 	for i, handle := range []*handlemap.Handle{handle_1, handle_2} {
// 		openFileOptions, readInBufferOptions, _ := suite.getRequestOptions(i, handle, false, int64(100*MB), 0, 0)
// 		suite.mock.EXPECT().OpenFile(openFileOptions).Return(handle, nil)
// 		suite.mock.EXPECT().ReadInBuffer(readInBufferOptions).Return(int(suite.stream.cache.blockSize), nil)
// 		suite.stream.OpenFile(openFileOptions)
// 		assertNumberOfCachedBlocks(suite, 1)
// 	}
// }

// // Get data that spans two blocks - we expect to have two blocks stored at the end
// func (suite *streamTestSuite) TestBlockDataOverlap() {
// 	defer suite.cleanupTest()
// 	suite.cleanupTest()
// 	config := "stream:\n  block-size-mb: 16\n  blocks-per-file: 4\n  cache-size-mb: 64\n  policy: lru"
// 	suite.setupTestHelper(config)
// 	handle := &handlemap.Handle{Size: int64(100 * MB), Path: fileNames[0]}

// 	openFileOptions, readInBufferOptions, _ := suite.getRequestOptions(0, handle, false, int64(100*MB), 0, 0)

// 	suite.mock.EXPECT().OpenFile(openFileOptions).Return(handle, nil)
// 	suite.mock.EXPECT().ReadInBuffer(readInBufferOptions).Return(int(suite.stream.cache.blockSize), nil)
// 	suite.stream.OpenFile(openFileOptions)
// 	assertBlockCached(suite, 0, handle)

// 	// options of our request from the stream component
// 	_, userReadInBufferOptions, _ := suite.getRequestOptions(0, handle, true, int64(100*MB), 1*MB, 17*MB)
// 	// options the stream component should request for the second block
// 	_, streamMissingBlockReadInBufferOptions, _ := suite.getRequestOptions(0, handle, false, int64(100*MB), 16*MB, 0)
// 	suite.mock.EXPECT().ReadInBuffer(streamMissingBlockReadInBufferOptions).Return(int(16*MB), nil)
// 	suite.stream.ReadInBuffer(userReadInBufferOptions)

// 	// we expect 0-16MB, and 16MB-32MB be cached since our second request is at offset 1MB

// 	assertNumberOfCachedBlocks(suite, 2)
// 	assertBlockCached(suite, 0, handle)
// 	assertBlockCached(suite, 16*MB, handle)
// 	assertNumberOfCachedFileBlocks(suite, 2, handle)
// }

// func (suite *streamTestSuite) TestFileSmallerThanBlockSize() {
// 	defer suite.cleanupTest()
// 	suite.cleanupTest()
// 	config := "stream:\n  block-size-mb: 16\n  blocks-per-file: 1\n  cache-size-mb: 16\n  policy: lru"
// 	suite.setupTestHelper(config)
// 	handle := &handlemap.Handle{Size: int64(15 * MB), Path: fileNames[0]}

// 	// case1: we know the size of the file from the get go, 15MB - smaller than our block size
// 	openFileOptions, readInBufferOptions, _ := suite.getRequestOptions(0, handle, true, int64(15*MB), 0, 15*MB)

// 	suite.mock.EXPECT().OpenFile(openFileOptions).Return(handle, nil)
// 	// we expect our request to be 15MB
// 	suite.mock.EXPECT().ReadInBuffer(readInBufferOptions).Return(int(15*MB), nil)
// 	suite.stream.OpenFile(openFileOptions)

// 	assertBlockCached(suite, 0, handle)
// 	blk := getCachedBlock(suite, 0, handle)
// 	suite.assert.Equal(int64(15*MB), blk.endIndex)

// 	// TODO: case2: file size changed in next component without stream being updated and therefore we get EOF
// }

// func (suite *streamTestSuite) TestEmptyFile() {
// 	defer suite.cleanupTest()
// 	suite.cleanupTest()
// 	config := "stream:\n  block-size-mb: 16\n  blocks-per-file: 1\n  cache-size-mb: 16\n  policy: lru"
// 	suite.setupTestHelper(config)
// 	handle := &handlemap.Handle{Size: 0, Path: fileNames[0]}

// 	// case1: we know the size of the file from the get go, 0
// 	openFileOptions, readInBufferOptions, _ := suite.getRequestOptions(0, handle, true, int64(0), 0, 0)

// 	suite.mock.EXPECT().OpenFile(openFileOptions).Return(handle, nil)
// 	// we expect our request to be 0
// 	suite.mock.EXPECT().ReadInBuffer(readInBufferOptions).Return(int(0), nil)
// 	suite.stream.OpenFile(openFileOptions)

// 	assertBlockCached(suite, 0, handle)
// 	blk := getCachedBlock(suite, 0, handle)
// 	suite.assert.Equal(int64(0), blk.endIndex)
// }

// // When we stop the component we expect everything to be deleted
// func (suite *streamTestSuite) TestCachePurge() {
// 	defer suite.cleanupTest()
// 	suite.cleanupTest()
// 	config := "stream:\n  block-size-mb: 16\n  blocks-per-file: 4\n  cache-size-mb: 64\n  policy: lru"
// 	suite.setupTestHelper(config)
// 	handle_1 := &handlemap.Handle{Size: int64(100 * MB), Path: fileNames[0]}
// 	handle_2 := &handlemap.Handle{Size: int64(100 * MB), Path: fileNames[1]}

// 	for i, handle := range []*handlemap.Handle{handle_1, handle_2} {
// 		openFileOptions, readInBufferOptions, _ := suite.getRequestOptions(i, handle, false, int64(100*MB), 0, 0)

// 		suite.mock.EXPECT().OpenFile(openFileOptions).Return(handle, nil)
// 		suite.mock.EXPECT().ReadInBuffer(readInBufferOptions).Return(int(suite.stream.cache.blockSize), nil)
// 		suite.stream.OpenFile(openFileOptions)
// 		assertBlockCached(suite, 0, handle)
// 	}

// 	suite.stream.Stop()
// 	assertNumberOfCachedBlocks(suite, 0)
// }

// // Data sanity check
// func (suite *streamTestSuite) TestCachedData() {
// 	defer suite.cleanupTest()
// 	suite.cleanupTest()
// 	config := "stream:\n  block-size-mb: 16\n  blocks-per-file: 2\n  cache-size-mb: 32\n  policy: lru"
// 	suite.setupTestHelper(config)
// 	var dataBuffer *[]byte
// 	var readInBufferOptions internal.ReadInBufferOptions
// 	handle_1 := &handlemap.Handle{Size: int64(32 * MB), Path: fileNames[0]}

// 	data := *getBlockData(suite, 32*MB)
// 	for _, off := range []int64{0, 16} {

// 		openFileOptions, readInBufferOptions, _ := suite.getRequestOptions(0, handle_1, false, int64(32*MB), off*MB, 0)

// 		if off == 0 {
// 			suite.mock.EXPECT().OpenFile(openFileOptions).Return(handle_1, nil)
// 			suite.mock.EXPECT().ReadInBuffer(readInBufferOptions).Return(int(suite.stream.cache.blockSize), nil)
// 			suite.stream.OpenFile(openFileOptions)
// 		} else {
// 			suite.mock.EXPECT().ReadInBuffer(readInBufferOptions).Return(int(suite.stream.cache.blockSize), nil)
// 			suite.stream.ReadInBuffer(readInBufferOptions)
// 		}

// 		assertBlockCached(suite, off*MB, handle_1)
// 		block := getCachedBlock(suite, off*MB, handle_1)
// 		block.data = data[off*MB : off*MB+suite.stream.cache.blockSize]

// 	}
// 	// now let's assert that it doesn't call next component and that the data retrieved is accurate
// 	// case1: data within a cached block
// 	_, readInBufferOptions, dataBuffer = suite.getRequestOptions(0, handle_1, true, int64(32*MB), int64(2*MB), int64(3*MB))
// 	suite.stream.ReadInBuffer(readInBufferOptions)
// 	suite.assert.Equal(data[2*MB:3*MB], *dataBuffer)

// 	// case2: data cached within two blocks
// 	_, readInBufferOptions, dataBuffer = suite.getRequestOptions(0, handle_1, true, int64(32*MB), int64(14*MB), int64(20*MB))
// 	suite.stream.ReadInBuffer(readInBufferOptions)
// 	suite.assert.Equal(data[14*MB:20*MB], *dataBuffer)
// }

// // This test does a data sanity check in the case where concurrent read is happening and causes evicitons
// func (suite *streamTestSuite) TestAsyncReadAndEviction() {
// 	defer suite.cleanupTest()
// 	suite.cleanupTest()
// 	config := "stream:\n  block-size-mb: 4\n  blocks-per-file: 4\n  cache-size-mb: 8\n  policy: lru"
// 	suite.setupTestHelper(config)

// 	var blockOneDataBuffer *[]byte
// 	var blockTwoDataBuffer *[]byte
// 	var readInBufferOptions internal.ReadInBufferOptions
// 	handle_1 := &handlemap.Handle{Size: int64(16 * MB), Path: fileNames[0]}

// 	// Even though our file size is 16MB below we only check against 8MB of the data (we check against two blocks)
// 	data := *getBlockData(suite, 8*MB)
// 	for _, off := range []int64{0, 4} {
// 		openFileOptions, readInBufferOptions, _ := suite.getRequestOptions(0, handle_1, false, int64(16*MB), off*MB, 0)
// 		if off == 0 {
// 			suite.mock.EXPECT().OpenFile(openFileOptions).Return(handle_1, nil)
// 			suite.mock.EXPECT().ReadInBuffer(readInBufferOptions).Return(int(suite.stream.cache.blockSize), nil)
// 			suite.stream.OpenFile(openFileOptions)
// 		} else {
// 			suite.mock.EXPECT().ReadInBuffer(readInBufferOptions).Return(int(suite.stream.cache.blockSize), nil)
// 			suite.stream.ReadInBuffer(readInBufferOptions)
// 		}

// 		assertBlockCached(suite, off*MB, handle_1)
// 		block := getCachedBlock(suite, off*MB, handle_1)
// 		block.data = data[off*MB : off*MB+suite.stream.cache.blockSize]
// 	}
// 	// test concurrent data access to the same file
// 	// call 1: data within a cached block
// 	_, readInBufferOptions, blockOneDataBuffer = suite.getRequestOptions(0, handle_1, true, int64(16*MB), int64(2*MB), int64(3*MB))
// 	suite.stream.ReadInBuffer(readInBufferOptions)
// 	wg.Add(2)

// 	// call 2: data cached within two blocks
// 	_, readInBufferOptions, blockTwoDataBuffer = suite.getRequestOptions(0, handle_1, true, int64(16*MB), int64(3*MB), int64(6*MB))
// 	go asyncReadInBuffer(suite, readInBufferOptions)
// 	// wait a little so we can guarantee block offset 0 is evicted
// 	time.Sleep(2 * time.Second)

// 	// call 3: get missing block causing an eviction to block 1 with offset 0 - this ensures our data from block 1 is still copied correctly
// 	_, readInBufferOptions, _ = suite.getRequestOptions(0, handle_1, false, int64(16*MB), int64(12*MB), 0)
// 	suite.mock.EXPECT().ReadInBuffer(readInBufferOptions).Return(int(suite.stream.cache.blockSize), nil)
// 	go asyncReadInBuffer(suite, readInBufferOptions)
// 	wg.Wait()

// 	// assert data within first block is correct
// 	suite.assert.Equal(data[2*MB:3*MB], *blockOneDataBuffer)
// 	// assert data between two blocks is correct
// 	suite.assert.Equal(data[3*MB:6*MB], *blockTwoDataBuffer)
// 	// assert we did in fact evict the first block and have added the third block
// 	assertBlockNotCached(suite, 0, handle_1)
// 	assertBlockCached(suite, 12*MB, handle_1)
// }

// // This tests concurrent open and ensuring the number of handles and cached blocks is handled correctly
// func (suite *streamTestSuite) TestAsyncOpen() {
// 	defer suite.cleanupTest()
// 	suite.cleanupTest()
// 	config := "stream:\n  block-size-mb: 16\n  blocks-per-file: 4\n  cache-size-mb: 64\n  policy: lru"
// 	suite.setupTestHelper(config)
// 	handle_1 := &handlemap.Handle{Size: int64(100 * MB), Path: fileNames[0]}
// 	handle_2 := &handlemap.Handle{Size: int64(100 * MB), Path: fileNames[1]}

// 	// Open four files concurrently - each doing a readInBuffer call to store the first block
// 	for i, handle := range []*handlemap.Handle{handle_1, handle_2} {
// 		openFileOptions, readInBufferOptions, _ := suite.getRequestOptions(i, handle, false, int64(100*MB), 0, 0)
// 		suite.mock.EXPECT().OpenFile(openFileOptions).Return(handle, nil)
// 		suite.mock.EXPECT().ReadInBuffer(readInBufferOptions).Return(int(suite.stream.cache.blockSize), nil)
// 		wg.Add(1)
// 		go asyncOpenFile(suite, openFileOptions)
// 	}
// 	wg.Wait()

// 	assertNumberOfCachedBlocks(suite, 2)
// 	for _, handle := range []*handlemap.Handle{handle_1, handle_2} {
// 		assertBlockCached(suite, 0, handle)
// 		assertNumberOfCachedFileBlocks(suite, 1, handle)
// 	}
// }

// func (suite *streamTestSuite) TestAsyncClose() {
// 	defer suite.cleanupTest()
// 	suite.cleanupTest()
// 	config := "stream:\n  block-size-mb: 16\n  blocks-per-file: 4\n  cache-size-mb: 64\n  policy: lru"
// 	suite.setupTestHelper(config)
// 	handle_1 := &handlemap.Handle{Size: int64(100 * MB), Path: fileNames[0]}
// 	handle_2 := &handlemap.Handle{Size: int64(100 * MB), Path: fileNames[1]}

// 	for i, handle := range []*handlemap.Handle{handle_1, handle_2} {
// 		openFileOptions, readInBufferOptions, _ := suite.getRequestOptions(i, handle, false, int64(100*MB), 0, 0)
// 		suite.mock.EXPECT().OpenFile(openFileOptions).Return(handle, nil)
// 		suite.mock.EXPECT().ReadInBuffer(readInBufferOptions).Return(int(suite.stream.cache.blockSize), nil)
// 		wg.Add(1)
// 		go asyncOpenFile(suite, openFileOptions)
// 	}
// 	wg.Wait()

// 	for _, handle := range []*handlemap.Handle{handle_1, handle_2} {
// 		closeFileOptions := internal.CloseFileOptions{Handle: handle}
// 		suite.mock.EXPECT().CloseFile(closeFileOptions).Return(nil)
// 		wg.Add(1)
// 		go asyncCloseFile(suite, closeFileOptions)
// 	}
// 	wg.Wait()
// }

// // persistnace related UT
// func (suite *streamTestSuite) TestBlockPersistence() {
// 	config := "stream:\n  block-size-mb: 16\n  blocks-per-file: 1\n  cache-size-mb: 16\n  policy: lru\n  disk-persistence: true\n  disk-cache-path: ./\n  disk-size-mb: 32"
// 	suite.setupTestHelper(config)
// 	handle_1 := &handlemap.Handle{Size: int64(100 * MB), Path: fileNames[0]}

// 	openFileOptions, readInBufferOptions, _ := suite.getRequestOptions(0, handle_1, false, int64(100*MB), 0, 0)

// 	suite.mock.EXPECT().OpenFile(openFileOptions).Return(handle_1, nil)
// 	suite.mock.EXPECT().ReadInBuffer(readInBufferOptions).Return(int(suite.stream.cache.blockSize), nil)
// 	suite.stream.OpenFile(openFileOptions)
// 	assertBlockCached(suite, 0, handle_1)

// 	_, readInBufferOptions, _ = suite.getRequestOptions(0, handle_1, false, int64(100*MB), 16*MB, 0)

// 	suite.mock.EXPECT().ReadInBuffer(readInBufferOptions).Return(int(suite.stream.cache.blockSize), nil)
// 	suite.stream.ReadInBuffer(readInBufferOptions)

// 	// Validate there is only one block cached in memory
// 	assertNumberOfCachedBlocks(suite, 1)

// 	// Validate original block at offset 0 is no more cached
// 	assertBlockNotCached(suite, 0, handle_1)

// 	// Validate latest block being read is cached in memory
// 	assertBlockCached(suite, 16*MB, handle_1)

// 	// Validate there is one disk block persisted
// 	assertNumberOfCachedFileBlocks(suite, 1, handle_1)

// 	// block with offset 0 does not exists in memory
// 	// we check it shall exists in disk
// 	found := diskBlockExists(suite, 0, handle_1)
// 	suite.assert.Equal(found, true)
// }

// func TestStreamTestSuite(t *testing.T) {
// 	suite.Run(t, new(streamTestSuite))
// }
