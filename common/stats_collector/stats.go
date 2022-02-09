/*
    _____           _____   _____   ____          ______  _____  ------
   |     |  |      |     | |     | |     |     | |       |            |
   |     |  |      |     | |     | |     |     | |       |            |
   | --- |  |      |     | |-----| |---- |     | |-----| |-----  ------
   |     |  |      |     | |     | |     |     |       | |       |
   | ____|  |_____ | ____| | ____| |     |_____|  _____| |_____  |_____


   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2020-2022 Microsoft Corporation. All rights reserved.
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

package stats

import (
	"sync"
	"time"
)

// FuseStats : Stats for the fuse wrapper
type FuseStats struct {
	lck sync.RWMutex

	fileOpen   uint64
	fileClose  uint64
	fileRead   uint64
	fileWrite  uint64
	fileDelete uint64
	fileRename uint64

	readDir   uint64
	deleteDir uint64
}

// AttrCacheStats : Stats for attribute cache layer
type AttrCacheStats struct {
	lck sync.RWMutex

	numFiles uint64
}

// FileCacheStats : Stats for file cache layer
type FileCacheStats struct {
	lck sync.RWMutex

	numFiles          uint64
	cacheUsage        uint64
	lastCacheEviction uint64
}

// StorageStats : Stats for stroage layer
type StorageStats struct {
	lck sync.RWMutex

	fileOpen   uint64
	fileClose  uint64
	fileRead   uint64
	fileWrite  uint64
	fileDelete uint64
	fileRename uint64

	readDir   uint64
	deleteDir uint64

	download uint64
	upload   uint64
}

// GlobalStats : Stats for gloabal monitoring
type GlobalStats struct {
	lck sync.RWMutex

	mountTime time.Time
}

type Stats struct {
	fuse      FuseStats
	attrCache AttrCacheStats
	fileCache FileCacheStats
	stroage   StorageStats
	common    GlobalStats
}
