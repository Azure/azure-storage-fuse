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
	"path/filepath"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	cm "github.com/Azure/azure-storage-fuse/v2/internal/dcache/clustermap"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache/debug/stats"
	rpc_client "github.com/Azure/azure-storage-fuse/v2/internal/dcache/rpc/client"
)

//go:generate $ASSERT_REMOVER $GOFILE

// The functions that were implemented inside this file should have Callback as the suffix for their functionName.
// The function should have this decl func(*procFile) error.

// proc file: clustermap
func readClusterMapCallback(pFile *procFile) error {
	var err error
	clusterMap := cm.GetClusterMap()
	exportedClusterMap := cm.ExportClusterMap(&clusterMap)
	pFile.buf, err = json.MarshalIndent(exportedClusterMap, "", "    ")

	if err != nil {
		log.Err("DebugFS::readClusterMapCallback, err: %v", err)
		common.Assert(false, err)
	}

	return nil
}

func getAttrClusterMapCallback(pFile *procFile) {
	clusterMap := cm.GetClusterMap()
	lmt := clusterMap.LastUpdatedAt
	common.Assert(lmt > 0)
	pFile.attr.Mtime = time.Unix(lmt, 0)
	pFile.attr.Ctime = pFile.attr.Mtime
	pFile.attr.Atime = pFile.attr.Mtime
}

// proc file: stats
func readStatsCallback(pFile *procFile) error {
	var err error

	//
	// Perform any preprocessing needed before marshalling.
	// This typically computes averages, etc.
	//
	stats.Stats.Preprocess()
	pFile.buf, err = json.MarshalIndent(stats.Stats, "", "    ")
	if err != nil {
		log.Err("DebugFS::readStatsCallback, err: %v", err)
		common.Assert(false, err)
	}

	return nil
}

// proc file: logs
// On first open, it triggers collection of logs from all nodes via RPC and returns a JSON summary.
// Large tarballs are stored on disk. This file only returns metadata mapping of node IDs to log tarball paths.
func readLogsCallback(pFile *procFile) error {
	// Use default work dir for storing collected logs.
	outDir := filepath.Join(common.DefaultWorkDir, fmt.Sprintf("cluster-logs-%d", time.Now().Unix()))

	// Chunk size fixed to 16MB.
	const chunkSize = int64(16 * 1024 * 1024)

	start := time.Now()

	logFiles, err := rpc_client.CollectAllNodeLogs(outDir, chunkSize)
	if err != nil {
		log.Err("DebugFS::readLogsCallback: collection completed with errors: %v", err)
	}

	// Build response struct.
	type logsResp struct {
		OutputDir   string            `json:"output_dir"`
		Files       map[string]string `json:"files"`
		Collected   int               `json:"collected"`
		DurationSec float64           `json:"duration_sec"`
		Error       string            `json:"error,omitempty"`
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
		log.Err("DebugFS::readLogsCallback: err: %v", err1)
		common.Assert(false, err1)
	}

	return nil
}

// Silence unused import errors for release builds.
func init() {
	common.IsValidUUID("00000000-0000-0000-0000-000000000000")
}
