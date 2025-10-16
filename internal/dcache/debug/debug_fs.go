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
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal"
	rpc_client "github.com/Azure/azure-storage-fuse/v2/internal/dcache/rpc/client"
	"github.com/Azure/azure-storage-fuse/v2/internal/handlemap"
)

//go:generate $ASSERT_REMOVER $GOFILE

// This package gives the debug facility to the dcache. Users can use top level sub-directory as "fs=debug" to know the
// state of the cluster/cache(maybe regarding clusterinfo, rpc calls, etc...). The files this package serves would be
// created on the fly and not stored anywhere in the filesystem.

var procFiles map[string]*procFile

// Mutex, openCnt is used for correctness of the filesystem if more that one handles for these procFiles are opened.
// without which also one can implement given that always there would one handle open for the file.
type procFile struct {
	mu            sync.Mutex            // lock for updating the openCnt and refreshing the buffer.
	buf           []byte                // Contents of the file.
	openCnt       int32                 // Open handles for this file.
	refreshBuffer func(*procFile) error // Refresh the contents of the file.
	attr          *internal.ObjAttr     // attr of the file.
	getAttr       func(*procFile)       // Modify any fields of attributes if needed.
}

// Directory entries in "fs=debug" directory. This list don't change as the files we support were already known.
var procDirList []*internal.ObjAttr

// logsWriteRequest defines JSON schema getting logs via fs=debug/logs.
// Example:
//
//	{
//	  "output_dir": "/tmp/logs",
//	  "number_of_logs": 4
//	}
//
// output_dir: directory where collected logs would be stored.
// number_of_logs: collect atmost this number of most recent log files per node.
type logsWriteRequest struct {
	OutputDir string `json:"output_dir"`
	NumLogs   int64  `json:"number_of_logs"`
}

// Logs response struct.
type logsResp struct {
	OutputDir   string            `json:"output_dir"`
	Files       map[string]string `json:"files"`
	Collected   int               `json:"collected"`
	DurationSec float64           `json:"duration_sec"`
	Error       string            `json:"error,omitempty"`
}

func init() {
	// Register the callbacks for the procFiles.
	procFiles = map[string]*procFile{
		"clustermap": &procFile{
			refreshBuffer: readClusterMapCallback,
			getAttr:       getAttrClusterMapCallback,
		}, // Show clusterInfo about dcache.

		"stats": &procFile{
			refreshBuffer: readStatsCallback,
		}, // Show dcache stats.

		"logs": &procFile{
			refreshBuffer: readLogsCallback,
		}, // Collect logs from all nodes (on-demand).
	}

	procDirList = make([]*internal.ObjAttr, 0, len(procFiles))
	for path, pFile := range procFiles {
		pFile.attr = &internal.ObjAttr{
			Name:  path,
			Path:  path,
			Mode:  0444,
			Mtime: time.Now(),
			Atime: time.Now(),
			Ctime: time.Now(),
			Size:  0,
		}
		procDirList = append(procDirList, pFile.attr)
	}
}

// Return the size of the file as zero, as we don't know the size at this point.
func GetAttr(options internal.GetAttrOptions) (*internal.ObjAttr, error) {
	if pFile, ok := procFiles[options.Name]; ok {
		if pFile.getAttr != nil {
			pFile.getAttr(pFile)
		}

		return pFile.attr, nil
	}
	return nil, syscall.ENOENT
}

func StreamDir(options internal.StreamDirOptions) ([]*internal.ObjAttr, string, error) {
	for _, pFile := range procFiles {
		if pFile.getAttr != nil {
			pFile.getAttr(pFile)
		} else {
			pFile.attr.Mtime = time.Now()
			pFile.attr.Atime = time.Now()
			pFile.attr.Ctime = time.Now()
		}
	}
	return procDirList, "", nil
}

// Read the file at the time of openFile into the corresponding buffer.
func OpenFile(options internal.OpenFileOptions) (*handlemap.Handle, error) {
	isWrite := false

	if options.Flags&syscall.O_RDWR != 0 || options.Flags&syscall.O_WRONLY != 0 {
		if options.Name != "logs" {
			// Only fs=debug/logs supports write operation.
			return nil, syscall.EACCES
		}

		isWrite = true
	}

	common.Assert(!isWrite || options.Name == "logs", isWrite, options.Name)

	handle := handlemap.NewHandle(options.Name)
	handle.SetFsDebug()

	pFile, err := openProcFile(options.Name, isWrite)
	if err != nil {
		return nil, syscall.ENOENT
	}

	handle.IFObj = pFile
	return handle, nil
}

// Read the buffer inside the procFile.
// No need to acquire the lock before reading from the buffer. As the buffer for proc file  would only refreshed only
// once at the start of the openFile even there are multiple handles.
func ReadFile(options *internal.ReadInBufferOptions) (int, error) {
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

func WriteFile(options *internal.WriteFileOptions) (int, error) {
	common.Assert(options.Handle.IFObj != nil)
	common.Assert(options.Handle.IsFsDebug(), options.Handle.Path)
	// Only fs=debug/logs supports write operation.
	common.Assert(options.Handle.Path == "logs", options.Handle.Path)

	var req logsWriteRequest
	if err := json.Unmarshal(options.Data, &req); err != nil {
		common.Assert(false, err)
		return 0, err
	}

	if len(req.OutputDir) == 0 || req.NumLogs <= 0 {
		common.Assert(false, req.OutputDir, req.NumLogs)
		return 0, fmt.Errorf("invalid input: output_dir=%s, number_of_logs=%d", req.OutputDir, req.NumLogs)
	}

	if !filepath.IsAbs(req.OutputDir) {
		common.Assert(false, req.OutputDir)
		return 0, fmt.Errorf("output_dir must be an absolute path: %s", req.OutputDir)
	}

	pFile := options.Handle.IFObj.(*procFile)
	common.Assert(atomic.LoadInt32(&pFile.openCnt) > 0)
	if options.Offset >= int64(len(pFile.buf)) {
		return 0, io.EOF
	}

	err := collectLogs(pFile, req.OutputDir, req.NumLogs)
	if err != nil {
		return 0, err
	}

	return len(options.Data), nil
}

// Refresh the contents of the proc File on first open and if isWrite flag is false.
func openProcFile(path string, isWrite bool) (*procFile, error) {
	common.Assert(!isWrite || path == "logs", isWrite, path)

	if pFile, ok := procFiles[path]; ok {
		pFile.mu.Lock()
		defer pFile.mu.Unlock()
		common.Assert(pFile.openCnt >= 0, path, pFile.openCnt)

		//
		// If isWrite flag is true, then WriteFile() will take care of refreshing the buffer based
		// on the write data sent to fs=debug/logs.
		//
		if pFile.openCnt == 0 && !isWrite {
			// This is the first handle to the proc File refresh the contents of the procFile.
			// Reset the buffer to length 0
			err := pFile.refreshBuffer(pFile)
			if err != nil {
				return nil, err
			}
		}

		pFile.openCnt++
		// Buffer must be allocated and must contain valid data.
		common.Assert(len(pFile.buf) > 0, len(pFile.buf))
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

func collectLogs(pFile *procFile, outDir string, numLogs int64) error {
	start := time.Now()

	logFiles, err := rpc_client.CollectAllNodeLogs(outDir, numLogs)
	if err != nil {
		log.Err("DebugFS::readLogsCallback: collection completed with errors: %v", err)
	}

	lr := &logsResp{
		OutputDir:   outDir,
		Files:       logFiles,
		Collected:   len(logFiles),
		DurationSec: time.Since(start).Seconds(),
	}

	if err != nil {
		lr.Error = err.Error()
	}

	var err1 error
	pFile.buf, err1 = json.MarshalIndent(lr, "", "  ")
	if err1 != nil {
		log.Err("DebugFS::collectLogs: err: %v", err1)
		common.Assert(false, err1)
		return err1
	}

	return nil
}

// Silence unused import errors for release builds.
func init() {
	var i atomic.Int32
	i.Store(0)
	common.IsValidUUID("00000000-0000-0000-0000-000000000000")
	fmt.Printf("")
}
