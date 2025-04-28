/*
    _____           _____   _____   ____          ______  _____  ------
   |     |  |      |     | |     | |     |     | |       |            |
   |     |  |      |     | |     | |     |     | |       |            |
   | --- |  |      |     | |-----| |---- |     | |-----| |-----  ------
   |     |  |      |     | |     | |     |     |       | |       |
   | ____|  |_____ | ____| | ____| |     |_____|  _____| |_____  |_____


   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2020-2025 Microsoft Corporation. All rights reserved.
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

package filemanager

import (
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type fileIOManagerTestSuite struct {
	suite.Suite
	assert *assert.Assertions
	file   *DcacheFile
}

const (
	chunkSize          = 4 * 1024 * 1024
	numWorkers         = 10
	numReadAheadChunks = 4
	numStagingBlocks   = 3
	maxBuffersForPool  = 10
)

func (suite *fileIOManagerTestSuite) SetupSuite() {
	suite.assert = assert.New(suite.T())
}

func (suite *fileIOManagerTestSuite) SetupTest() {
	NewFileIOManager(numWorkers, numReadAheadChunks, numStagingBlocks, chunkSize, maxBuffersForPool)
}

func (suite *fileIOManagerTestSuite) TearDownTest() {
	EndFileIOManager()
}

func createExistingFile() *DcacheFile {
	file := NewFile("foo")
	file.FileMetadata.Size = 30 * 1024 * 1024
	return file
}

func createNewFile() *DcacheFile {
	return NewFile("foo")
}

func (suite *fileIOManagerTestSuite) TestReadFile() {
	file := createExistingFile()
	buf := make([]byte, 4*1024)
	for i := int64(0); i < 30*1024*1024; i += 4 * 1024 {
		bytesRead, err := file.ReadFile(i, buf)
		suite.assert.Equal(4096, bytesRead)
		suite.assert.Nil(err)
		suite.assert.LessOrEqual(fileIOMgr.bp.getCurBuffersCnt(), int64(6))
	}
	// Read Last byte of the file
	bytesRead, err := file.ReadFile(file.FileMetadata.Size-1, buf)
	suite.assert.Equal(1, bytesRead)
	suite.assert.Nil(err)
	// Read EOF
	bytesRead, err = file.ReadFile(file.FileMetadata.Size, buf)
	suite.assert.Equal(0, bytesRead)
	suite.assert.Equal(io.EOF, err)

	// sync the file.
	err = file.SyncFile()
	suite.assert.Nil(err)
	// Release the file.
	err = file.ReleaseFile()
	suite.assert.Nil(err)
	suite.assert.Equal(fileIOMgr.bp.getCurBuffersCnt(), int64(0))
}

func (suite *fileIOManagerTestSuite) TestSeqWriteFile() {
	file := createNewFile()
	buf := make([]byte, 4*1024)
	for i := int64(0); i < 30*1024*1024; i += 4 * 1024 {
		err := file.WriteFile(i, buf)
		suite.assert.Nil(err)
		suite.assert.Equal(i+4096, file.lastWriteOffset)
		suite.assert.LessOrEqual(fileIOMgr.bp.getCurBuffersCnt(), int64(3))
	}
	// sync the file.
	err := file.SyncFile()
	suite.assert.Nil(err)
	// Release the file.
	err = file.ReleaseFile()
	suite.assert.Nil(err)
	suite.assert.Equal(fileIOMgr.bp.getCurBuffersCnt(), int64(0))

}

func (suite *fileIOManagerTestSuite) TestRandWriteFile() {
	file := createNewFile()
	buf := make([]byte, 4*1024)
	for i := int64(0); i < 10*1024*1024; i += 4 * 1024 {
		err := file.WriteFile(i, buf)
		suite.assert.Nil(err)
		suite.assert.Equal(i+4096, file.lastWriteOffset)
		suite.assert.LessOrEqual(fileIOMgr.bp.getCurBuffersCnt(), int64(3))
	}
	// Now write at 5MB which should fail as we only allow seq writes.
	err := file.WriteFile(5*1024*1024, buf)
	suite.assert.NotNil(err)
	// sync the file.
	err = file.SyncFile()
	suite.assert.Nil(err)
	// Release the file.
	err = file.ReleaseFile()
	suite.assert.Nil(err)
	suite.assert.Equal(fileIOMgr.bp.getCurBuffersCnt(), int64(0))

}

// This is majorly used when reading the file in random manner.
func (suite *fileIOManagerTestSuite) TestReadChunk() {
	file := createExistingFile()
	chnk, err := file.readChunk(0, true)
	suite.assert.Nil(err)
	suite.assert.NotNil(chnk)
	suite.assert.Equal(int64(0), chnk.Idx)
	suite.assert.Equal(chunkSize, len(chnk.Buf))
	suite.assert.LessOrEqual(fileIOMgr.bp.getCurBuffersCnt(), int64(2))

	chnk, err = file.readChunk(1024, true)
	suite.assert.Nil(err)
	suite.assert.NotNil(chnk)
	suite.assert.Equal(int64(0), chnk.Idx)
	suite.assert.Equal(chunkSize, len(chnk.Buf))
	suite.assert.LessOrEqual(fileIOMgr.bp.getCurBuffersCnt(), int64(2))
}

// Testing using sequentially reading the file and checking the buffers for chunks are getting
// allocate properly.
func (suite *fileIOManagerTestSuite) TestReadAheadChunk() {
	file := createExistingFile()
	for i := int64(0); i < 30*1024*1024; i += 4 * 1024 {
		chnk, err := file.readChunkWithReadAhead(i)
		suite.assert.Nil(err)
		suite.assert.NotNil(chnk)
		suite.assert.Equal(i/chunkSize, chnk.Idx)
		suite.assert.Equal(chunkSize, len(chnk.Buf))
		suite.assert.LessOrEqual(fileIOMgr.bp.getCurBuffersCnt(), int64(6))
	}
}

// This test's the writeback policy for the upload of chunks
func (suite *fileIOManagerTestSuite) TestWriteChunk() {
	file := createNewFile()
	for i := int64(0); i < 30*1024*1024; i += 4 * 1024 {
		chnk, err := file.writeChunk(i)
		suite.assert.Nil(err)
		suite.assert.NotNil(chnk)
		suite.assert.Equal(i/chunkSize, chnk.Idx)
		suite.assert.Equal(chunkSize, len(chnk.Buf))
		suite.assert.LessOrEqual(fileIOMgr.bp.getCurBuffersCnt(), int64(3))
	}

}

func TestFileManager(t *testing.T) {
	suite.Run(t, new(fileIOManagerTestSuite))
}
