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

package distributed_cache

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/internal"
)

// Check if the path is of placeHolder dir/virtual sub component for root of mountpoint.
// This virtual directory should only valid if it's present at the root of the mountpoint.
func isPlaceholderDirForRoot(path string) (bool, *internal.ObjAttr) {
	if path == "fs=azure" || path == "fs=dcache" {
		attr := &internal.ObjAttr{
			Path:  path,
			Name:  filepath.Base(path),
			Size:  4096,
			Mode:  os.ModeDir,
			Mtime: time.Now(),
			Flags: internal.NewDirBitMap(),
		}
		attr.Atime = attr.Mtime
		attr.Crtime = attr.Mtime
		attr.Ctime = attr.Mtime
		attr.Flags.Set(internal.PropFlagModeDefault)
		return true, attr
	}
	return false, nil
}

// Check if path contains fs=azure as it's subdirectory,
// and returns the newpath
func isAzurePath(path string) (found bool, resPath string) {
	return isPathContainsSubDir(path, "fs=azure")
}

func isDcachePath(path string) (found bool, resPath string) {
	return isPathContainsSubDir(path, "fs=dcache")
}

// function to know path consists of given subdir at it's root
// returns path without the subdir
func isPathContainsSubDir(path string, subdir string) (found bool, resPath string) {
	if len(path) == 0 {
		return false, path
	}

	after, found := strings.CutPrefix(path, subdir)
	if !found {
		return false, path
	}

	resPath = after
	if len(resPath) > 1 && resPath[0] != '/' {
		return false, path
	}
	resPath = strings.TrimPrefix(resPath, "/")
	return
}

// hides the cache folder __CACHE__ + cacheid folder from listing the mountpoint root
func hideCacheFolder(dirList []*internal.ObjAttr, cachePath string) []*internal.ObjAttr {
	for i, attr := range dirList {
		if attr.Path == cachePath {
			// The following can be replaced with swapping the element with last one. but that would
			// mess up the blob ordering.
			return append(dirList[:i], dirList[i+1:]...)
		}
	}
	return dirList
}

func isMountPointRoot(path string) bool {
	if len(path) == 0 || (len(path) == 1 && path[0] == '/') {
		return true
	}
	return false
}
