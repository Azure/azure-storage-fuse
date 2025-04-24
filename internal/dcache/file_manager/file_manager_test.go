package filemanager

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type fileIOManagerTestSuite struct {
	suite.Suite
	assert *assert.Assertions
	file   *File
}

const (
	chunkSize          = 4 * 1024 * 1024
	numWorkers         = 10
	numReadAheadChunks = 4
	maxBuffersForPool  = 10
)

func (suite *fileIOManagerTestSuite) SetupSuite() {
	suite.assert = assert.New(suite.T())
}

func (suite *fileIOManagerTestSuite) SetupTest() {
	NewFileIOManager(numWorkers, numReadAheadChunks, chunkSize, maxBuffersForPool)
}

func (suite *fileIOManagerTestSuite) TearDownTest() {
	EndFileIOManager()
}

func createExistingFile() *File {
	file := NewFile("foo")
	file.FileMetadata.Size = 30 * 1024 * 1024
	return file
}

func createNewFile() *File {
	return NewFile("foo")
}

// This is majorly used when reading the file in random manner.
func (suite *fileIOManagerTestSuite) TestReadChunkSync() {
	file := createExistingFile()
	chnk, err := ReadChunkSync(0, file)
	suite.assert.Nil(err)
	suite.assert.NotNil(chnk)
	suite.assert.Equal(int64(0), chnk.Idx)
	suite.assert.Equal(chunkSize, len(chnk.Buf))
	suite.assert.LessOrEqual(fileIOMgr.bp.numRequestedBuffers, int64(2))

	chnk, err = ReadChunkSync(1024, file)
	suite.assert.Nil(err)
	suite.assert.NotNil(chnk)
	suite.assert.Equal(int64(0), chnk.Idx)
	suite.assert.Equal(chunkSize, len(chnk.Buf))
	suite.assert.LessOrEqual(fileIOMgr.bp.numRequestedBuffers, int64(2))

	chnk, err = ReadChunkSync(30*1024*1024*1024, file)
	suite.assert.NotNil(err)
	suite.assert.Nil(chnk)
	suite.assert.LessOrEqual(fileIOMgr.bp.numRequestedBuffers, int64(2))

}

// Testing using sequentially reading the file and checking the buffers for chunks are getting
// allocate properly.
func (suite *fileIOManagerTestSuite) TestReadChunkAsync() {
	file := createExistingFile()
	for i := int64(0); i < 30*1024*1024; i += 4 * 1024 {
		chnk, err := ReadChunkAsync(i, file)
		suite.assert.Nil(err)
		suite.assert.NotNil(chnk)
		suite.assert.Equal(i/chunkSize, chnk.Idx)
		suite.assert.Equal(chunkSize, len(chnk.Buf))
		suite.assert.LessOrEqual(fileIOMgr.bp.numRequestedBuffers, int64(2))
	}
}

// This test's the writeback policy for the upload of chunks
func (suite *fileIOManagerTestSuite) TestWriteChunk() {
	file := createNewFile()
	for i := int64(0); i < 30*1024*1024; i += 4 * 1024 {
		chnk, err := WriteChunk(i, nil, file)
		suite.assert.Nil(err)
		suite.assert.NotNil(chnk)
		suite.assert.Equal(i/chunkSize, chnk.Idx)
		suite.assert.Equal(chunkSize, len(chnk.Buf))
		suite.assert.LessOrEqual(fileIOMgr.bp.numRequestedBuffers, int64(3))
	}

}

func TestFileManager(t *testing.T) {
	suite.Run(t, new(fileIOManagerTestSuite))
}
