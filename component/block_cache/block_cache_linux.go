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
	"bytes"
	"os"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"golang.org/x/sys/unix"
)

// setBlockChecksum sets the checksum as an xattr on Linux.
func setBlockChecksum(localPath string, data []byte, n int) error {
	hash := common.GetCRC64(data, n)
	return unix.Setxattr(localPath, "user.md5sum", hash, 0)
}

func checkBlockConsistency(blockCache *BlockCache, item *workItem, numberOfBytes int, localPath, fileName string) bool {
	if !blockCache.consistency {
		return true
	}
	// Calculate MD5 checksum of the read data
	actualHash := common.GetCRC64(item.block.data, numberOfBytes)

	// Retrieve MD5 checksum from xattr
	xattrHash := make([]byte, 8)
	_, err := unix.Getxattr(localPath, "user.md5sum", xattrHash)
	if err != nil {
		log.Err("BlockCache::download : Failed to get md5sum for file %s [%v]", fileName, err.Error())
	} else {
		// Compare checksums
		if !bytes.Equal(actualHash, xattrHash) {
			log.Err("BlockCache::download : MD5 checksum mismatch for file %s, expected %v, got %v", fileName, xattrHash, actualHash)
			_ = os.Remove(localPath)
			return false
		}
	}

	return true
}