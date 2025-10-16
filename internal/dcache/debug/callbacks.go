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
	"strings"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	cm "github.com/Azure/azure-storage-fuse/v2/internal/dcache/clustermap"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache/debug/stats"
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
// Tarballs are stored on disk. This file only returns metadata mapping of node IDs to log tarball paths.
//
// By default, we use the default work dir for storing the collected logs and
// we also collect only the most recent log file from each node by default.
//
// If users want to specify a different directory or number of logs, they can use the
// fs=debug/logs file with a write call to specify those parameters.
//
// For example, to collect atmost 4 recent logs into /tmp/logs:
//
// echo '{"output_dir": "/tmp/logs", "number_of_logs": 4}' > /<mnt_path>/fs=debug/logs
//
// This will store the logs in /tmp/logs and will fetch 4 most recent logs from each node.
func readLogsCallback(pFile *procFile) error {
	timestamp := strings.Replace(time.Now().UTC().Format(time.RFC3339), ":", "-", -1)
	outDir := filepath.Join(common.DefaultWorkDir, fmt.Sprintf("cluster-logs-%s", timestamp))
	return collectLogs(pFile, outDir, 1 /* numLogs */)
}

// Silence unused import errors for release builds.
func init() {
	common.IsValidUUID("00000000-0000-0000-0000-000000000000")
}
