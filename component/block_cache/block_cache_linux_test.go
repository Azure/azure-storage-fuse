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

package block_cache

import (
	"math/rand"
	"os"
	"path/filepath"
	"syscall"

	"github.com/Azure/azure-storage-fuse/v2/internal"
)

func (suite *blockCacheTestSuite) TestStrongConsistency() {
	tobj, err := setupPipeline("")
	defer tobj.cleanupPipeline()

	suite.assert.NoError(err)
	suite.assert.NotNil(tobj.blockCache)

	tobj.blockCache.consistency = true

	path := getTestFileName(suite.T().Name())
	options := internal.CreateFileOptions{Name: path, Mode: 0777}
	h, err := tobj.blockCache.CreateFile(options)
	suite.assert.NoError(err)
	suite.assert.NotNil(h)
	suite.assert.Equal(int64(0), h.Size)
	suite.assert.False(h.Dirty())

	storagePath := filepath.Join(tobj.fake_storage_path, path)
	fs, err := os.Stat(storagePath)
	suite.assert.NoError(err)
	suite.assert.Equal(int64(0), fs.Size())
	//Generate random size of file in bytes less than 2MB

	size := rand.Intn(2097152)
	data := make([]byte, size)

	n, err := tobj.blockCache.WriteFile(&internal.WriteFileOptions{Handle: h, Offset: 0, Data: data}) // Write data to file
	suite.assert.NoError(err)
	suite.assert.Equal(n, size)
	suite.assert.Equal(h.Size, int64(size))

	err = tobj.blockCache.CloseFile(internal.CloseFileOptions{Handle: h})
	suite.assert.NoError(err)
	suite.assert.Nil(h.Buffers.Cooked)
	suite.assert.Nil(h.Buffers.Cooking)

	localPath := filepath.Join(tobj.disk_cache_path, path+"::0")

	xattrMd5sumOrg := make([]byte, 32)
	_, err = syscall.Getxattr(localPath, "user.md5sum", xattrMd5sumOrg)
	suite.assert.NoError(err)

	h, err = tobj.blockCache.OpenFile(internal.OpenFileOptions{Name: path, Flags: os.O_RDWR})
	suite.assert.NoError(err)
	suite.assert.NotNil(h)
	_, _ = tobj.blockCache.ReadInBuffer(&internal.ReadInBufferOptions{Handle: h, Offset: 0, Data: data})
	err = tobj.blockCache.CloseFile(internal.CloseFileOptions{Handle: h})
	suite.assert.NoError(err)
	suite.assert.Nil(h.Buffers.Cooked)
	suite.assert.Nil(h.Buffers.Cooking)

	xattrMd5sumRead := make([]byte, 32)
	_, err = syscall.Getxattr(localPath, "user.md5sum", xattrMd5sumRead)
	suite.assert.NoError(err)
	suite.assert.Equal(xattrMd5sumOrg, xattrMd5sumRead)

	err = syscall.Setxattr(localPath, "user.md5sum", []byte("000"), 0)
	suite.assert.NoError(err)

	xattrMd5sum1 := make([]byte, 32)
	_, err = syscall.Getxattr(localPath, "user.md5sum", xattrMd5sum1)
	suite.assert.NoError(err)

	h, err = tobj.blockCache.OpenFile(internal.OpenFileOptions{Name: path, Flags: os.O_RDWR})
	suite.assert.NoError(err)
	suite.assert.NotNil(h)
	_, _ = tobj.blockCache.ReadInBuffer(&internal.ReadInBufferOptions{Handle: h, Offset: 0, Data: data})
	err = tobj.blockCache.CloseFile(internal.CloseFileOptions{Handle: h})
	suite.assert.NoError(err)
	suite.assert.Nil(h.Buffers.Cooked)
	suite.assert.Nil(h.Buffers.Cooking)

	xattrMd5sum2 := make([]byte, 32)
	_, err = syscall.Getxattr(localPath, "user.md5sum", xattrMd5sum2)
	suite.assert.NoError(err)

	suite.assert.NotEqual(xattrMd5sum1, xattrMd5sum2)
}