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

package debug

import (
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/internal"
	"github.com/Azure/azure-storage-fuse/v2/internal/handlemap"
)

// This package gives the debug facility to the dcache. Users can use top level sub-directory as "fs=debug" to know the
// state of the cluster/cache(maybe regarding clusterinfo, rpc calls, etc...). The files this package serves would be
// created on the fly and not stored anywhere in the filesystem.

var procFiles map[string]*procFile

// Mutex, openCnt is used for correctness of the filesystem if more that one handles for these procFiles are opened.
// without which also one can implement given that always there would one handle open for the file.
type procFile struct {
	mu            sync.Mutex                         // lock for updating the openCnt and refreshing the buffer.
	buf           []byte                             // Contents of the file.
	openCnt       int32                              // Open handles for this file.
	refreshBuffer func(*procFile) error              // Refresh the contents of the file.
	getAttr       func(*procFile, *internal.ObjAttr) // Modify any fields of attributes if needed.
}

// Directory entries in "fs=debug" directory. This list don't change as the files we support were already known.
var procDirList []*internal.ObjAttr

func init() {
	// Register the callbacks for the procFiles.
	procFiles = map[string]*procFile{
		"clustermap": &procFile{
			buf:           make([]byte, 0, 4096),
			openCnt:       0,
			refreshBuffer: readClusterMapCallback,
			getAttr:       getAttrClusterMapCallback,
		}, // Show clusterInfo about dcache.
	}

	procDirList = make([]*internal.ObjAttr, 0, len(procFiles))
	for path, _ := range procFiles {
		attr := &internal.ObjAttr{
			Name: path,
			Path: path,
			Size: 0,
		}
		procDirList = append(procDirList, attr)
	}
}

// Return the size of the file as zero, as we don't know the size at this point.
func GetAttr(options internal.GetAttrOptions) (*internal.ObjAttr, error) {
	if pFile, ok := procFiles[options.Name]; ok {
		attr := &internal.ObjAttr{
			Name:  options.Name,
			Path:  options.Name,
			Mode:  0444,
			Mtime: time.Now(),
			Atime: time.Now(),
			Ctime: time.Now(),
			Size:  0,
		}

		pFile.getAttr(pFile, attr)

		return attr, nil
	}
	return nil, syscall.ENOENT
}

func StreamDir(options internal.StreamDirOptions) ([]*internal.ObjAttr, string, error) {
	return procDirList, "", nil
}

// Read the file at the time of openFile into the corresponding buffer.
func OpenFile(options internal.OpenFileOptions) (*handlemap.Handle, error) {
	if options.Flags&syscall.O_RDWR != 0 || options.Flags&syscall.O_WRONLY != 0 {
		return nil, syscall.EACCES
	}

	handle := handlemap.NewHandle(options.Name)
	handle.SetFsDebug()

	pFile, err := openProcFile(options.Name)
	if err != nil {
		return nil, syscall.ENOENT
	}
	handle.IFObj = pFile
	return handle, nil
}

// Read the buffer inside the procFile.
// No need to acquire the lock before reading from the buffer. As the buffer for proc file  would only refreshed only
// once at the start of the openFile even there are multiple handles.
func ReadFile(options internal.ReadInBufferOptions) (int, error) {
	common.Assert(options.Handle.IFObj != nil)
	pFile := options.Handle.IFObj.(*procFile)
	common.Assert(atomic.LoadInt32(&pFile.openCnt) > 0)
	if options.Offset >= int64(len(pFile.buf)) {
		return 0, io.EOF
	}
	bytesRead := copy(options.Data, pFile.buf[options.Offset:])
	return bytesRead, nil
}

func CloseFile(options internal.CloseFileOptions) error {
	common.Assert(options.Handle.IFObj != nil)
	pFile := options.Handle.IFObj.(*procFile)
	closeProcFile(pFile)
	return nil
}

// Refresh the contents of the proc File if needed
func openProcFile(path string) (*procFile, error) {
	if pFile, ok := procFiles[path]; ok {
		pFile.mu.Lock()
		defer pFile.mu.Unlock()
		common.Assert(pFile.openCnt >= 0, fmt.Sprintf("Open Cnt for procFile: %s, openCnt: %d", path, pFile.openCnt))
		if pFile.openCnt == 0 {
			// This is the first handle to the proc File refresh the contents of the procFile.
			// Reset the buffer to length 0
			pFile.buf = pFile.buf[:0]
			err := pFile.refreshBuffer(pFile)
			if err != nil {
				return nil, err
			}
		}
		pFile.openCnt++
		return pFile, nil
	}
	return nil, syscall.ENOENT
}

func closeProcFile(pFile *procFile) {
	pFile.mu.Lock()
	defer pFile.mu.Unlock()
	common.Assert(pFile.openCnt > 0)
	pFile.openCnt--
}
