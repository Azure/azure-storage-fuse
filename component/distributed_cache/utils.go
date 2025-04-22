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
	"strings"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/internal"
)

// Get the placeHolder dir/virtual sub component for root of mountpoint.
// This virtual directory should only valid if it's present at the root of the mountpoint.
func getPlaceholderDirForRoot(path string) *internal.ObjAttr {
	attr := &internal.ObjAttr{
		Path:  path,
		Size:  4096,
		Mode:  os.ModeDir,
		Mtime: time.Now(),
		Flags: internal.NewDirBitMap(),
	}
	attr.Atime = attr.Mtime
	attr.Crtime = attr.Mtime
	attr.Ctime = attr.Mtime
	attr.Flags.Set(internal.PropFlagModeDefault)
	return attr
}

// returns true for isAzurePath, if path has "fs=azure" as its first subdir.
// return true for isDcachPath, if path has "fs=dcache" as its first subdir.
// rawPath is the resultant path after removing virtual dirs like "fs=azure/dcache"
// returns path if it dont find any virtual dirs.
func getFS(path string) (isAzurePath bool, isDcachePath bool, rawPath string) {
	rawPath = path
	isAzurePath, tempPath := isPathContainsSubDir(path, "fs=azure")
	if isAzurePath {
		rawPath = tempPath
	} else {
		isDcachePath, tempPath = isPathContainsSubDir(path, "fs=dcache")
		if isDcachePath {
			rawPath = tempPath
		}
	}
	return isAzurePath, isDcachePath, rawPath
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
	if len(resPath) > 0 && resPath[0] != '/' {
		return false, path
	}
	resPath = strings.TrimPrefix(resPath, "/")
	return
}

// hides the cache folder that starts with prefix __CACHE__.
func hideCacheMetadata(dirList []*internal.ObjAttr) []*internal.ObjAttr {
	newDirList := make([]*internal.ObjAttr, len(dirList))
	i := 0
	for _, attr := range dirList {
		// todo: think of a better approach for doing the following.
		if !strings.HasPrefix(attr.Path, "__CACHE__") {
			newDirList[i] = attr
			i++
		}
	}
	return newDirList[:i]
}

func isMountPointRoot(path string) bool {
	if len(path) == 0 || (len(path) == 1 && path[0] == '/') {
		return true
	}
	return false
}
