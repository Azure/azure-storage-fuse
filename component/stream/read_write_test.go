package stream

import (
	"blobfuse2/common"
	"blobfuse2/internal"
	"blobfuse2/internal/handlemap"
	"os"
	"testing"

	"github.com/stretchr/testify/suite"
)

func (suite *streamTestSuite) TestWriteConfig() {
	defer suite.cleanupTest()
	suite.cleanupTest()
	config := "stream:\n  block-size-mb: 4\n  handle-buffer-size-mb: 16\n  handle-limit: 4\n"
	suite.setupTestHelper(config, false)

	suite.assert.Equal("stream", suite.stream.Name())
	suite.assert.Equal(16*MB, int(suite.stream.BufferSizePerHandle))
	suite.assert.Equal(4, int(suite.stream.HandleLimit))
	suite.assert.EqualValues(false, suite.stream.StreamOnly)
	suite.assert.EqualValues(4*MB, suite.stream.BlockSize)

	// assert streaming is on if any of the values is 0
	suite.cleanupTest()
	config = "stream:\n  block-size-mb: 0\n  handle-buffer-size-mb: 16\n  handle-limit: 4\n"
	suite.setupTestHelper(config, false)
	suite.assert.EqualValues(true, suite.stream.StreamOnly)
}

// test small file caching
func (suite *streamTestSuite) TestCacheSmallFileOnOpen() {
	defer suite.cleanupTest()
	suite.cleanupTest()
	config := "stream:\n  block-size-mb: 16\n  handle-buffer-size-mb: 64\n  handle-limit: 4\n"
	suite.setupTestHelper(config, false)

	// make small file very large to confirm it would be stream only
	handle := &handlemap.Handle{Size: int64(100000000 * MB), Path: fileNames[0]}
	getFileBlockOffsetsOptions := internal.GetFileBlockOffsetsOptions{Name: fileNames[0]}
	openFileOptions := internal.OpenFileOptions{Name: fileNames[0], Flags: os.O_RDONLY, Mode: os.FileMode(0777)}
	bol := &common.BlockOffsetList{
		BlockList: []*common.Block{},
	}
	bol.Flags.Set(common.SmallFile)

	suite.mock.EXPECT().OpenFile(openFileOptions).Return(handle, nil)
	suite.mock.EXPECT().GetFileBlockOffsets(getFileBlockOffsetsOptions).Return(bol, nil)
	suite.stream.OpenFile(openFileOptions)

	assertBlockNotCached(suite, 0, handle)
	assertNumberOfCachedFileBlocks(suite, 0, handle)
	assertHandleStreamOnly(suite, handle)

	// small file that should get cached on open
	handle = &handlemap.Handle{Size: int64(1), Path: fileNames[1]}
	openFileOptions = internal.OpenFileOptions{Name: fileNames[1], Flags: os.O_RDONLY, Mode: os.FileMode(0777)}
	getFileBlockOffsetsOptions = internal.GetFileBlockOffsetsOptions{Name: fileNames[1]}
	readInBufferOptions := internal.ReadInBufferOptions{
		Handle: handle,
		Offset: 0,
		Data:   make([]byte, 1),
	}

	suite.mock.EXPECT().OpenFile(openFileOptions).Return(handle, nil)
	suite.mock.EXPECT().GetFileBlockOffsets(getFileBlockOffsetsOptions).Return(bol, nil)
	suite.mock.EXPECT().ReadInBuffer(readInBufferOptions).Return(len(readInBufferOptions.Data), nil)
	suite.stream.OpenFile(openFileOptions)

	assertBlockCached(suite, 0, handle)
	assertNumberOfCachedFileBlocks(suite, 1, handle)
	assertHandleNotStreamOnly(suite, handle)
}

// test large files don't cache block on open
func (suite *streamTestSuite) TestOpenLargeFile() {
	defer suite.cleanupTest()
	suite.cleanupTest()
	config := "stream:\n  block-size-mb: 16\n  handle-buffer-size-mb: 64\n  handle-limit: 4\n"
	suite.setupTestHelper(config, false)

	handle := &handlemap.Handle{Size: int64(4 * MB), Path: fileNames[0]}
	getFileBlockOffsetsOptions := internal.GetFileBlockOffsetsOptions{Name: fileNames[0]}
	openFileOptions := internal.OpenFileOptions{Name: fileNames[0], Flags: os.O_RDONLY, Mode: os.FileMode(0777)}
	// file consists of two blocks
	bol := &common.BlockOffsetList{
		BlockList: []*common.Block{{StartIndex: 0, EndIndex: 2 * MB}, {StartIndex: 2, EndIndex: 4 * MB}},
	}

	suite.mock.EXPECT().OpenFile(openFileOptions).Return(handle, nil)
	suite.mock.EXPECT().GetFileBlockOffsets(getFileBlockOffsetsOptions).Return(bol, nil)
	suite.stream.OpenFile(openFileOptions)

	assertBlockNotCached(suite, 0, handle)
	assertNumberOfCachedFileBlocks(suite, 0, handle)
	assertHandleNotStreamOnly(suite, handle)
}

// test if handle limit met to stream only next handles
func (suite *streamTestSuite) TestStreamOnly() {
	defer suite.cleanupTest()
	suite.cleanupTest()
	// set handle limit to 1
	config := "stream:\n  block-size-mb: 16\n  handle-buffer-size-mb: 64\n  handle-limit: 1\n"
	suite.setupTestHelper(config, false)

	handle := &handlemap.Handle{Size: int64(4 * MB), Path: fileNames[0]}
	getFileBlockOffsetsOptions := internal.GetFileBlockOffsetsOptions{Name: fileNames[0]}
	openFileOptions := internal.OpenFileOptions{Name: fileNames[0], Flags: os.O_RDONLY, Mode: os.FileMode(0777)}
	bol := &common.BlockOffsetList{
		BlockList: []*common.Block{{StartIndex: 0, EndIndex: 2 * MB}, {StartIndex: 2, EndIndex: 4 * MB}},
	}

	suite.mock.EXPECT().OpenFile(openFileOptions).Return(handle, nil)
	suite.mock.EXPECT().GetFileBlockOffsets(getFileBlockOffsetsOptions).Return(bol, nil)
	suite.stream.OpenFile(openFileOptions)
	assertHandleNotStreamOnly(suite, handle)

	// create new handle
	handle = &handlemap.Handle{Size: int64(4 * MB), Path: fileNames[0]}
	getFileBlockOffsetsOptions = internal.GetFileBlockOffsetsOptions{Name: fileNames[0]}
	openFileOptions = internal.OpenFileOptions{Name: fileNames[0], Flags: os.O_RDONLY, Mode: os.FileMode(0777)}
	bol = &common.BlockOffsetList{
		BlockList: []*common.Block{{StartIndex: 0, EndIndex: 2 * MB}, {StartIndex: 2, EndIndex: 4 * MB}},
	}

	suite.mock.EXPECT().OpenFile(openFileOptions).Return(handle, nil)
	suite.stream.OpenFile(openFileOptions)

	assertBlockNotCached(suite, 0, handle)
	assertNumberOfCachedFileBlocks(suite, 0, handle)
	// confirm new handle is stream only since limit is exceeded
	assertHandleStreamOnly(suite, handle)
}

func (suite *streamTestSuite) TestReadLargeFileBlocks() {
	defer suite.cleanupTest()
	suite.cleanupTest()
	// set handle limit to 1
	config := "stream:\n  block-size-mb: 4\n  handle-buffer-size-mb: 64\n  handle-limit: 1\n"
	suite.setupTestHelper(config, false)

	handle1 := &handlemap.Handle{Size: int64(2 * MB), Path: fileNames[0]}
	getFileBlockOffsetsOptions := internal.GetFileBlockOffsetsOptions{Name: fileNames[0]}
	openFileOptions := internal.OpenFileOptions{Name: fileNames[0], Flags: os.O_RDONLY, Mode: os.FileMode(0777)}
	bol := &common.BlockOffsetList{
		BlockList: []*common.Block{{StartIndex: 0, EndIndex: 1 * MB}, {StartIndex: 1 * MB, EndIndex: 2 * MB}},
	}

	suite.mock.EXPECT().OpenFile(openFileOptions).Return(handle1, nil)
	suite.mock.EXPECT().GetFileBlockOffsets(getFileBlockOffsetsOptions).Return(bol, nil)
	suite.stream.OpenFile(openFileOptions)

	assertBlockNotCached(suite, 0, handle1)
	assertNumberOfCachedFileBlocks(suite, 0, handle1)
	assertHandleNotStreamOnly(suite, handle1)

	// data spans two blocks
	readInBufferOptions := internal.ReadInBufferOptions{
		Handle: handle1,
		Offset: 1*MB - 2,
		Data:   make([]byte, 7),
	}

	suite.mock.EXPECT().ReadInBuffer(internal.ReadInBufferOptions{
		Handle: handle1,
		Offset: 0,
		Data:   make([]byte, 1*MB)}).Return(len(readInBufferOptions.Data), nil)

	suite.mock.EXPECT().ReadInBuffer(internal.ReadInBufferOptions{
		Handle: handle1,
		Offset: 1 * MB,
		Data:   make([]byte, 1*MB)}).Return(len(readInBufferOptions.Data), nil)

	suite.stream.ReadInBuffer(readInBufferOptions)

	assertBlockCached(suite, 0, handle1)
	assertBlockCached(suite, 1*MB, handle1)
	assertNumberOfCachedFileBlocks(suite, 2, handle1)
}

func (suite *streamTestSuite) TestPurgeOnClose() {
	defer suite.cleanupTest()
	suite.cleanupTest()
	config := "stream:\n  block-size-mb: 16\n  handle-buffer-size-mb: 64\n  handle-limit: 4\n"
	suite.setupTestHelper(config, false)

	handle := &handlemap.Handle{Size: int64(1), Path: fileNames[0]}
	getFileBlockOffsetsOptions := internal.GetFileBlockOffsetsOptions{Name: fileNames[0]}
	openFileOptions := internal.OpenFileOptions{Name: fileNames[0], Flags: os.O_RDONLY, Mode: os.FileMode(0777)}
	bol := &common.BlockOffsetList{
		BlockList: []*common.Block{},
	}
	bol.Flags.Set(common.SmallFile)
	readInBufferOptions := internal.ReadInBufferOptions{
		Handle: handle,
		Offset: 0,
		Data:   make([]byte, 1),
	}

	suite.mock.EXPECT().OpenFile(openFileOptions).Return(handle, nil)
	suite.mock.EXPECT().GetFileBlockOffsets(getFileBlockOffsetsOptions).Return(bol, nil)
	suite.mock.EXPECT().ReadInBuffer(readInBufferOptions).Return(len(readInBufferOptions.Data), nil)
	suite.stream.OpenFile(openFileOptions)

	assertBlockCached(suite, 0, handle)
	assertNumberOfCachedFileBlocks(suite, 1, handle)
	assertHandleNotStreamOnly(suite, handle)

	suite.mock.EXPECT().FlushFile(internal.FlushFileOptions{Handle: handle}).Return(nil)
	suite.mock.EXPECT().CloseFile(internal.CloseFileOptions{Handle: handle}).Return(nil)
	suite.stream.CloseFile(internal.CloseFileOptions{Handle: handle})
	assertBlockNotCached(suite, 0, handle)
}

// ========================================================= Write tests =================================================================
//TODO: need to add an assertion on the blocks for their start and end indices as we append to them
//test appending to small file evicts older block if cache capacity full
func (suite *streamTestSuite) TestWriteToSmallFileEviction() {
	defer suite.cleanupTest()
	suite.cleanupTest()
	config := "stream:\n  block-size-mb: 1\n  handle-buffer-size-mb: 1\n  handle-limit: 4\n"
	suite.setupTestHelper(config, false)

	// create small file and confirm it gets cached
	handle := &handlemap.Handle{Size: int64(1 * MB), Path: fileNames[0]}
	getFileBlockOffsetsOptions := internal.GetFileBlockOffsetsOptions{Name: fileNames[0]}
	openFileOptions := internal.OpenFileOptions{Name: fileNames[0], Flags: os.O_RDONLY, Mode: os.FileMode(0777)}
	bol := &common.BlockOffsetList{
		BlockList: []*common.Block{},
	}
	bol.Flags.Set(common.SmallFile)
	readInBufferOptions := internal.ReadInBufferOptions{
		Handle: handle,
		Offset: 0,
		Data:   make([]byte, 1*MB),
	}

	suite.mock.EXPECT().OpenFile(openFileOptions).Return(handle, nil)
	suite.mock.EXPECT().GetFileBlockOffsets(getFileBlockOffsetsOptions).Return(bol, nil)
	suite.mock.EXPECT().ReadInBuffer(readInBufferOptions).Return(len(readInBufferOptions.Data), nil)
	suite.stream.OpenFile(openFileOptions)
	assertBlockCached(suite, 0, handle)
	assertNumberOfCachedFileBlocks(suite, 1, handle)

	// append new block and confirm old gets evicted
	writeFileOptions := internal.WriteFileOptions{
		Handle: handle,
		Offset: 1 * MB,
		Data:   make([]byte, 1*MB),
	}
	suite.mock.EXPECT().FlushFile(internal.FlushFileOptions{Handle: handle}).Return(nil)
	suite.stream.WriteFile(writeFileOptions)

	assertBlockNotCached(suite, 0, handle)
	assertBlockCached(suite, 1*MB, handle)
	assertNumberOfCachedFileBlocks(suite, 1, handle)
	assertHandleNotStreamOnly(suite, handle)
}

// get block 1, get block 2, mod block 2, mod block 1, create new block - expect block 2 to be removed
func (suite *streamTestSuite) TestLargeFileEviction() {
	defer suite.cleanupTest()
	suite.cleanupTest()
	config := "stream:\n  block-size-mb: 1\n  handle-buffer-size-mb: 2\n  handle-limit: 2\n"
	suite.setupTestHelper(config, false)

	// file consists of two blocks
	block1 := &common.Block{StartIndex: 0, EndIndex: 1 * MB}
	block2 := &common.Block{StartIndex: 1 * MB, EndIndex: 2 * MB}

	handle := &handlemap.Handle{Size: int64(2 * MB), Path: fileNames[0]}
	getFileBlockOffsetsOptions := internal.GetFileBlockOffsetsOptions{Name: fileNames[0]}
	openFileOptions := internal.OpenFileOptions{Name: fileNames[0], Flags: os.O_RDONLY, Mode: os.FileMode(0777)}
	bol := &common.BlockOffsetList{
		BlockList:     []*common.Block{block1, block2},
		BlockIdLength: 10,
	}
	readInBufferOptions := internal.ReadInBufferOptions{
		Handle: handle,
		Offset: 0,
		Data:   make([]byte, 1*MB),
	}

	suite.mock.EXPECT().OpenFile(openFileOptions).Return(handle, nil)
	suite.mock.EXPECT().GetFileBlockOffsets(getFileBlockOffsetsOptions).Return(bol, nil)
	suite.stream.OpenFile(openFileOptions)

	suite.mock.EXPECT().ReadInBuffer(readInBufferOptions).Return(len(readInBufferOptions.Data), nil)
	suite.stream.ReadInBuffer(readInBufferOptions)

	assertBlockCached(suite, 0, handle)
	assertNumberOfCachedFileBlocks(suite, 1, handle)

	// get second block
	readInBufferOptions = internal.ReadInBufferOptions{
		Handle: handle,
		Offset: 1 * MB,
		Data:   make([]byte, 1*MB),
	}

	suite.mock.EXPECT().ReadInBuffer(readInBufferOptions).Return(len(readInBufferOptions.Data), nil)
	suite.stream.ReadInBuffer(readInBufferOptions)

	assertBlockCached(suite, 1*MB, handle)
	assertNumberOfCachedFileBlocks(suite, 2, handle)

	// write to second block
	writeFileOptions := internal.WriteFileOptions{
		Handle: handle,
		Offset: 1*MB + 2,
		Data:   make([]byte, 2),
	}
	suite.stream.WriteFile(writeFileOptions)

	// write to first block
	writeFileOptions.Offset = 2
	suite.stream.WriteFile(writeFileOptions)

	// append to file
	writeFileOptions.Offset = 2*MB + 4

	// when we get the first flush - it means we're clearing out our cache
	callbackFunc := func(options internal.FlushFileOptions) {
		block1.Flags.Clear(common.DirtyBlock)
		block2.Flags.Clear(common.DirtyBlock)
	}
	suite.mock.EXPECT().FlushFile(internal.FlushFileOptions{Handle: handle}).Do(callbackFunc).Return(nil)
	suite.mock.EXPECT().FlushFile(internal.FlushFileOptions{Handle: handle}).Return(nil)

	suite.stream.WriteFile(writeFileOptions)

	assertBlockCached(suite, 0, handle)
	assertBlockCached(suite, 2*MB, handle)
	assertBlockNotCached(suite, 1*MB, handle)
	assertNumberOfCachedFileBlocks(suite, 2, handle)
	suite.assert.Equal(handle.Size, int64(2*MB+6))
}

// test stream only handle becomes cached handle
func (suite *streamTestSuite) TestStreamOnlyHandle() {
	defer suite.cleanupTest()
	suite.cleanupTest()
	// set handle limit to 1
	config := "stream:\n  block-size-mb: 4\n  handle-buffer-size-mb: 64\n  handle-limit: 1\n"
	suite.setupTestHelper(config, false)

	handle1 := &handlemap.Handle{Size: int64(2 * MB), Path: fileNames[0]}
	getFileBlockOffsetsOptions := internal.GetFileBlockOffsetsOptions{Name: fileNames[0]}
	openFileOptions := internal.OpenFileOptions{Name: fileNames[0], Flags: os.O_RDONLY, Mode: os.FileMode(0777)}
	bol := &common.BlockOffsetList{
		BlockList: []*common.Block{{StartIndex: 0, EndIndex: 1 * MB}, {StartIndex: 1, EndIndex: 2 * MB}},
	}

	suite.mock.EXPECT().OpenFile(openFileOptions).Return(handle1, nil)
	suite.mock.EXPECT().GetFileBlockOffsets(getFileBlockOffsetsOptions).Return(bol, nil)
	suite.stream.OpenFile(openFileOptions)

	assertBlockNotCached(suite, 0, handle1)
	assertNumberOfCachedFileBlocks(suite, 0, handle1)
	assertHandleNotStreamOnly(suite, handle1)

	handle2 := &handlemap.Handle{Size: int64(2 * MB), Path: fileNames[0]}
	openFileOptions = internal.OpenFileOptions{Name: fileNames[0], Flags: os.O_RDONLY, Mode: os.FileMode(0777)}
	suite.mock.EXPECT().OpenFile(openFileOptions).Return(handle2, nil)
	suite.stream.OpenFile(openFileOptions)

	assertBlockNotCached(suite, 0, handle2)
	assertNumberOfCachedFileBlocks(suite, 0, handle2)
	// confirm new handle is stream only
	assertHandleStreamOnly(suite, handle2)

	//close the first handle
	closeFileOptions := internal.CloseFileOptions{Handle: handle1}
	suite.mock.EXPECT().FlushFile(internal.FlushFileOptions{Handle: handle1}).Return(nil)
	suite.mock.EXPECT().CloseFile(closeFileOptions).Return(nil)
	suite.stream.CloseFile(closeFileOptions)

	// get block for second handle and confirm it gets cached
	readInBufferOptions := internal.ReadInBufferOptions{
		Handle: handle2,
		Offset: 0,
		Data:   make([]byte, 4),
	}

	suite.mock.EXPECT().GetFileBlockOffsets(getFileBlockOffsetsOptions).Return(bol, nil)
	suite.mock.EXPECT().ReadInBuffer(internal.ReadInBufferOptions{
		Handle: handle2,
		Offset: 0,
		Data:   make([]byte, 1*MB)}).Return(len(readInBufferOptions.Data), nil)
	suite.stream.ReadInBuffer(readInBufferOptions)

	assertBlockCached(suite, 0, handle2)
	assertNumberOfCachedFileBlocks(suite, 1, handle2)
	assertHandleNotStreamOnly(suite, handle2)
}

func (suite *streamTestSuite) TestCreateFile() {
	defer suite.cleanupTest()
	suite.cleanupTest()
	// set handle limit to 1
	config := "stream:\n  block-size-mb: 4\n  handle-buffer-size-mb: 64\n  handle-limit: 1\n"
	suite.setupTestHelper(config, false)

	handle1 := &handlemap.Handle{Size: 0, Path: fileNames[0]}
	createFileoptions := internal.CreateFileOptions{Name: handle1.Path, Mode: 0777}
	getFileBlockOffsetsOptions := internal.GetFileBlockOffsetsOptions{Name: fileNames[0]}
	bol := &common.BlockOffsetList{
		BlockList: []*common.Block{},
	}
	bol.Flags.Set(common.SmallFile)

	suite.mock.EXPECT().CreateFile(createFileoptions).Return(handle1, nil)
	suite.mock.EXPECT().GetFileBlockOffsets(getFileBlockOffsetsOptions).Return(bol, nil)
	suite.stream.CreateFile(createFileoptions)
	assertHandleNotStreamOnly(suite, handle1)
}

// func (suite *streamTestSuite) TestFlushFile() {
// }

func TestWriteStreamTestSuite(t *testing.T) {
	suite.Run(t, new(streamTestSuite))
}
