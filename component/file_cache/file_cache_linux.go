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

	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal"
	"golang.org/x/sys/unix"
)

func getChangeTime(finfo fs.FileInfo) time.Time {
	if sys := finfo.Sys(); sys != nil {
		if stat, ok := sys.(*syscall.Stat_t); ok {
			return time.Unix(stat.Ctim.Sec, stat.Ctim.Nsec)
		}
	}
	// Fallback - use modification time
	return finfo.ModTime()
}

// Creates a new object attribute
func newObjAttr(path string, info fs.FileInfo) *internal.ObjAttr {
	stat := info.Sys().(*syscall.Stat_t)
	attrs := &internal.ObjAttr{
		Path:  path,
		Name:  info.Name(),
		Size:  info.Size(),
		Mode:  info.Mode(),
		Mtime: time.Unix(stat.Mtim.Sec, stat.Mtim.Nsec),
		Atime: time.Unix(stat.Atim.Sec, stat.Atim.Nsec),
		Ctime: time.Unix(stat.Ctim.Sec, stat.Ctim.Nsec),
	}

	if info.Mode()&os.ModeSymlink != 0 {
		attrs.Flags.Set(internal.PropFlagSymlink)
	} else if info.IsDir() {
		attrs.Flags.Set(internal.PropFlagIsDir)
	}

	return attrs
}

func (fc *FileCache) getAvailableSize() (uint64, error) {
	statfs := &unix.Statfs_t{}
	err := unix.Statfs(fc.tmpPath, statfs)
	if err != nil {
		log.Debug("FileCache::getAvailableSize : statfs err [%s].", err.Error())
		return 0, err
	}

	available := statfs.Bavail * uint64(statfs.Bsize)
	return available, nil
}

func (fc *FileCache) syncFile(f *os.File, path string) error {
	// Flush all data to disk that has been buffered by the kernel.
	// We cannot close the incoming handle since the user called flush, note close and flush can be called on the same handle multiple times.
	// To ensure the data is flushed to disk before writing to storage, we duplicate the handle and close that handle.
	// f.fsync() is another option but dup+close does it quickly compared to sync
	dupFd, err := unix.Dup(int(f.Fd()))
	if err != nil {
		log.Err("FileCache::FlushFile : error [couldn't duplicate the fd] %s", path)
		return syscall.EIO
	}

	err = unix.Close(dupFd)
	if err != nil {
		log.Err("FileCache::FlushFile : error [unable to close duplicate fd] %s", path)
		return syscall.EIO
	}
	return nil
}

// pread wraps syscall.Pread for Linux
func pread(fd int, data []byte, offset int64) (int, error) {
	return syscall.Pread(fd, data, offset)
}

// pwrite wraps syscall.Pwrite for Linux
func pwrite(fd int, data []byte, offset int64) (int, error) {
	return syscall.Pwrite(fd, data, offset)
}
