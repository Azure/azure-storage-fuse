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

package block_cache

import (
	"testing"

	"github.com/Azure/azure-storage-fuse/v2/internal"
	"github.com/Azure/azure-storage-fuse/v2/internal/handlemap"
	"github.com/stretchr/testify/assert"
)

// Test that ReleaseFile with an invalid handle type returns an error.
func TestReleaseFile_InvalidHandleType(t *testing.T) {
	bc := NewBlockCacheComponent().(*BlockCache)
	handle := handlemap.NewHandle("/tmp/file")
	handle.IFObj = "not a blockCacheHandle"

	err := bc.ReleaseFile(internal.ReleaseFileOptions{Handle: handle})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid handle type")
}

// Test that FlushFile with an invalid handle type returns an error.
func TestFlushFile_InvalidHandleType(t *testing.T) {
	bc := NewBlockCacheComponent().(*BlockCache)
	handle := handlemap.NewHandle("/tmp/file")
	handle.IFObj = "not a blockCacheHandle"

	err := bc.FlushFile(internal.FlushFileOptions{Handle: handle})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid handle type")
}

// Test that SyncFile with an invalid handle type returns an error.
func TestSyncFile_InvalidHandleType(t *testing.T) {
	bc := NewBlockCacheComponent().(*BlockCache)
	handle := handlemap.NewHandle("/tmp/file")
	handle.IFObj = "not a blockCacheHandle"

	err := bc.SyncFile(internal.SyncFileOptions{Handle: handle})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid handle type")
}

// Test that WriteFile with an invalid handle type returns an error.
func TestWriteFile_InvalidHandleType(t *testing.T) {
	bc := NewBlockCacheComponent().(*BlockCache)
	handle := handlemap.NewHandle("/tmp/file")
	handle.IFObj = "not a blockCacheHandle"

	n, err := bc.WriteFile(&internal.WriteFileOptions{Handle: handle, Offset: 0, Data: []byte("x")})
	assert.Error(t, err)
	assert.Equal(t, 0, n)
	assert.Contains(t, err.Error(), "invalid handle type")
}

// Test that ReadInBuffer with an invalid handle type returns an error.
func TestReadInBuffer_InvalidHandleType(t *testing.T) {
	bc := NewBlockCacheComponent().(*BlockCache)
	handle := handlemap.NewHandle("/tmp/file")
	handle.IFObj = "not a blockCacheHandle"

	n, err := bc.ReadInBuffer(&internal.ReadInBufferOptions{Handle: handle, Offset: 0, Data: make([]byte, 10)})
	assert.Error(t, err)
	assert.Equal(t, 0, n)
	assert.Contains(t, err.Error(), "invalid handle type")
}

// Test that TruncateFile with an invalid handle type returns an error.
func TestTruncateFile_InvalidHandleType(t *testing.T) {
	bc := NewBlockCacheComponent().(*BlockCache)
	handle := handlemap.NewHandle("/tmp/file")
	handle.IFObj = "not a blockCacheHandle"

	err := bc.TruncateFile(internal.TruncateFileOptions{Name: "/tmp/file", NewSize: 0, Handle: handle})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid handle type")
}
