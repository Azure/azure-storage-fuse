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

// logsWriteRequest defines JSON schema for controlling how logs are bundled+fetched via fs=debug/logs.
// Example:
//
//	{
//	  "output_dir": "/tmp/logs",
//	  "number_of_logs": 4
//	}
//
// output_dir: local directory on the node where fs=debug/logs is read, where log bundles fetched from all nodes
//             would be stored.
// number_of_logs: collect atmost this many most recent blobfuse2.log* files per node.
//
// Note: Since logs could be large, make sure that the output_dir has enough space to store the collected logs.

type logsWriteRequest struct {
	OutputDir string `json:"output_dir"`
	NumLogs   int64  `json:"number_of_logs"`
}

// Logs response struct.
type logsResp struct {
	OutputDir   string            `json:"output_dir"`
	Files       map[string]string `json:"files"`
	NumNodes    int               `json:"number_of_nodes"`
	NumLogs     int               `json:"number_of_logs_per_node"`
	DurationSec float64           `json:"duration_sec"`
	Error       string            `json:"error,omitempty"`
}

// clusterSummaryWriteRequest defines JSON schema for controlling the generation of cluster summary
// via fs=debug/cluster-summary.
//
//	{
//	  "refresh_clustermap": true
//	}
//
// refresh_clustermap: if true, forces a refresh of the cluster map before generating the summary.

type clusterSummaryWriteRequest struct {
	RefreshClusterMap bool `json:"refresh_clustermap"`
}

var logsReq *logsWriteRequest
var clusterSummaryReq *clusterSummaryWriteRequest

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

		"logs.help": &procFile{
			refreshBuffer: readLogsHelpCallback,
		}, // Help summary about fs=debug/logs.

		"cluster-summary": &procFile{
			refreshBuffer: readClusterSummaryCallback,
		}, // Cluster summary (nodes/RVs/MVs).

		"cluster-summary.help": &procFile{
			refreshBuffer: readClusterSummaryHelpCallback,
		}, // Help summary about fs=debug/cluster-summary.
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

		if IsWriteable(path) {
			// fs=debug/logs supports write operation.
			pFile.attr.Mode = 0644
		}

		procDirList = append(procDirList, pFile.attr)
	}

	//
	// Default log collection dir is common.DefaultWorkDir and default number of logs collected is 1.
	// These values can be updated by writing an appropriate logsWriteRequest json to fs=debug/logs file.
	//
	logsReq = &logsWriteRequest{
		OutputDir: common.DefaultWorkDir,
		NumLogs:   1,
	}

	// Default cluster-summary is printed with the local cluster map (without refreshing it).
	clusterSummaryReq = &clusterSummaryWriteRequest{
		RefreshClusterMap: false,
	}
}

func IsWriteable(name string) bool {
	if name == "logs" ||
		name == "cluster-summary" {
		return true
	}
	return false
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
		if !IsWriteable(options.Name) {
			return nil, syscall.EACCES
		}

		isWrite = true
	}

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
// No need to acquire the lock before reading from the buffer. As the buffer for proc file would only be refreshed
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

// WriteFile, currently must only be called for fs=debug/logs.
// It is used to update the directory where collected logs would be stored and also
// the number of most recent logs to collect per node.
//
// By default, we use the default work dir for storing the collected logs and
// we also collect only the most recent log file from each node by default.
//
// If users want to specify a different directory or number of logs, they can use the
// fs=debug/logs file with a write call to configure these parameters.
//
// For example, to configure collecting atmost 4 recent logs per node into /tmp/logs:
//
// echo '{"output_dir": "/tmp/logs", "number_of_logs": 4}' > /<mnt_path>/fs=debug/logs
//
// This updates the parameters for logs collection and number of logs to collect per node.
// After this, next read calls to fs=debug/logs will use these updated parameters.
//
// TODO: Later when we support write for more files in fs=debug, we can refactor this function
//       and call the registered write callback for the specific procFile.

func WriteFile(options *internal.WriteFileOptions) (int, error) {
	common.Assert(options.Handle.IFObj != nil)
	common.Assert(options.Handle.IsFsDebug(), options.Handle.Path)
	// We should only get write calls for debug files which are writeable.
	common.Assert(IsWriteable(options.Handle.Path), options.Handle.Path)
	common.Assert(logsReq != nil)

	if options.Handle.Path == "logs" {
		//
		// Max path length check for output_dir + few extra bytes for rest of json.
		//
		if len(options.Data) > 4200 {
			log.Err("DebugFS::WriteFile: large logs write request of length %d",
				len(options.Data))
			return -1, syscall.EINVAL
		}

		if err := json.Unmarshal(options.Data, logsReq); err != nil {
			log.Err("DebugFS::WriteFile: failed to parse logs write request: %v [%s]",
				err, string(options.Data))
			return -1, syscall.EINVAL
		}

		if len(logsReq.OutputDir) == 0 || logsReq.NumLogs <= 0 {
			log.Err("DebugFS::WriteFile: Invalid json data: output_dir=%s, number_of_logs=%d [%s]",
				logsReq.OutputDir, logsReq.NumLogs, string(options.Data))
			return -1, syscall.EINVAL
		}

		if !filepath.IsAbs(logsReq.OutputDir) {
			log.Err("DebugFS::WriteFile: output_dir is not an absolute path: %s [%s]",
				logsReq.OutputDir, string(options.Data))
			return -1, syscall.EINVAL
		}

		log.Info("DebugFS::WriteFile: Updated logs collection config, output_dir=%s, number_of_logs=%d",
			logsReq.OutputDir, logsReq.NumLogs)
	} else if options.Handle.Path == "cluster-summary" {
		if len(options.Data) > 100 {
			log.Err("DebugFS::WriteFile: large cluster-summary write request of length %d",
				len(options.Data))
			return -1, syscall.EINVAL
		}

		if err := json.Unmarshal(options.Data, clusterSummaryReq); err != nil {
			log.Err("DebugFS::WriteFile: failed to parse cluster-summary write request: %v [%s]",
				err, string(options.Data))
			return -1, syscall.EINVAL
		}

		log.Info("DebugFS::WriteFile: Updated cluster-summary config, refresh_clustermap=%v",
			clusterSummaryReq.RefreshClusterMap)
	} else {
		common.Assert(false, options.Handle.Path)
	}

	return len(options.Data), nil
}

// Refresh the contents of the proc File on first open and if isWrite flag is false.
func openProcFile(path string, isWrite bool) (*procFile, error) {
	common.Assert(!isWrite || IsWriteable(path), isWrite, path)

	if pFile, ok := procFiles[path]; ok {
		pFile.mu.Lock()
		defer pFile.mu.Unlock()
		common.Assert(pFile.openCnt >= 0, path, pFile.openCnt)

		//
		// isWrite is currently supported only for fs=debug/logs file and we should not
		// call refreshBuffer on write opens.
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
		common.Assert(len(pFile.buf) > 0 || isWrite, len(pFile.buf))
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

// Silence unused import errors for release builds.
func init() {
	var i atomic.Int32
	i.Store(0)
	common.IsValidUUID("00000000-0000-0000-0000-000000000000")
	fmt.Printf("")
}
