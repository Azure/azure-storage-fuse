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

package file_cache

import (
	"io/fs"
	"os"
	"syscall"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal"
	"golang.org/x/sys/windows"
)

func getChangeTime(finfo fs.FileInfo) time.Time {
	if sys := finfo.Sys(); sys != nil {
		if stat, ok := sys.(*syscall.Win32FileAttributeData); ok {
			// Windows - use modification time as change time
			return time.Unix(0, stat.LastWriteTime.Nanoseconds())
		}
	}
	// Fallback - use modification time
	return finfo.ModTime()
}

// Creates a new object attribute
func newObjAttr(path string, info fs.FileInfo) *internal.ObjAttr {
	stat := info.Sys().(*syscall.Win32FileAttributeData)
	attrs := &internal.ObjAttr{
		Path:  common.NormalizeObjectName(path),
		Name:  common.NormalizeObjectName(info.Name()),
		Size:  info.Size(),
		Mode:  info.Mode(),
		Mtime: time.Unix(0, stat.LastWriteTime.Nanoseconds()),
		Atime: time.Unix(0, stat.LastAccessTime.Nanoseconds()),
		Ctime: time.Unix(0, stat.CreationTime.Nanoseconds()),
	}

	if info.Mode()&os.ModeSymlink != 0 {
		attrs.Flags.Set(internal.PropFlagSymlink)
	} else if info.IsDir() {
		attrs.Flags.Set(internal.PropFlagIsDir)
	}

	return attrs
}

func (fc *FileCache) getAvailableSize() (uint64, error) {
	var free, total, avail uint64

	// Get path to the cache
	pathPtr, err := windows.UTF16PtrFromString(fc.tmpPath)
	if err != nil {
		return 0, err
	}
	err = windows.GetDiskFreeSpaceEx(pathPtr, &free, &total, &avail)
	if err != nil {
		log.Debug("FileCache::StatFs : statfs err [%s].", err.Error())
		return 0, err
	}

	return avail, nil
}

func (fc *FileCache) syncFile(f *os.File, path string) error {
	err := f.Sync()
	if err != nil {
		log.Err("FileCache::FlushFile : error [unable to sync file] %s", path)
		return syscall.EIO
	}
	return nil
}

// pread implements position-based read for Windows
func pread(fd int, data []byte, offset int64) (int, error) {
	var overlapped windows.Overlapped
	overlapped.Offset = uint32(offset)
	overlapped.OffsetHigh = uint32(offset >> 32)

	var bytesRead uint32
	err := windows.ReadFile(windows.Handle(fd), data, &bytesRead, &overlapped)
	if err != nil {
		return int(bytesRead), err
	}
	return int(bytesRead), nil
}

// pwrite implements position-based write for Windows
func pwrite(fd int, data []byte, offset int64) (int, error) {
	var overlapped windows.Overlapped
	overlapped.Offset = uint32(offset)
	overlapped.OffsetHigh = uint32(offset >> 32)

	var bytesWritten uint32
	err := windows.WriteFile(windows.Handle(fd), data, &bytesWritten, &overlapped)
	if err != nil {
		return int(bytesWritten), err
	}
	return int(bytesWritten), nil
}