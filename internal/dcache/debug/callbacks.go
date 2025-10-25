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
	dcache "github.com/Azure/azure-storage-fuse/v2/internal/dcache"
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
// Tarballs are stored on disk. This function only returns metadata mapping of node IDs to log tarball paths.
func readLogsCallback(pFile *procFile) error {
	common.Assert(logsReq != nil)
	common.Assert(logsReq.OutputDir != "")
	common.Assert(logsReq.NumLogs > 0)

	timestamp := strings.Replace(time.Now().UTC().Format(time.RFC3339), ":", "-", -1)
	// e.g., cluster-logs-2025-10-18T08-37-38Z
	outDir := filepath.Join(logsReq.OutputDir, fmt.Sprintf("cluster-logs-%s", timestamp))

	//
	// Make the GetLogs RPC request to each node requesting numLogs logs to be tar'ed and returned
	// and save the returned tarballs to outDir.
	//
	start := time.Now()

	logFiles, err := rpc_client.CollectAllNodeLogs(outDir, logsReq.NumLogs)
	if err != nil {
		log.Err("DebugFS::readLogsCallback: collection completed with errors: %v", err)
	}

	lr := &logsResp{
		OutputDir:   outDir,
		Files:       logFiles,
		NumNodes:    len(logFiles),
		NumLogs:     int(logsReq.NumLogs),
		DurationSec: time.Since(start).Seconds(),
	}

	if err != nil {
		lr.Error = err.Error()
	} else {
		common.Assert(lr.NumNodes > 0, lr.NumNodes)
		common.Assert(lr.NumLogs <= lr.NumNodes, lr.NumLogs, lr.NumNodes)
	}

	//
	// Log index file contains mapping of node IDs to tarball paths.
	// This is returned as the proc file content.
	//
	var err1 error
	pFile.buf, err1 = json.MarshalIndent(lr, "", "  ")
	if err1 != nil {
		log.Err("DebugFS::collectLogs: failed to marshal log index json %+v: %v",
			*lr, err1)
		common.Assert(false, err1)
		return err1
	}

	return nil
}

// proc file: logs.help
func readLogsHelpCallback(pFile *procFile) error {
	help := `{
	"output_dir": "/local/dir/where/log/bundles/should/be/saved",
	"number_of_logs": 4
}
`
	pFile.buf = []byte(help)
	return nil
}

// proc file: cluster-summary
// Provides a high-level summary of cluster state: nodes (online/offline), RVs and MVs state counts.
func readClusterSummaryCallback(pFile *procFile) error {
	//
	// If user wants to view the latest cluster map, refresh it first.
	//
	if clusterSummaryReq.RefreshClusterMap {
		cm.RefreshClusterMap(0)
	}

	// Get current clustermap copy (local view).
	clusterMap := cm.GetClusterMap()

	if cm.RingBasedMVPlacement {
		clusterMap.Config.MVsPerRV = 0
		clusterMap.Config.MaxRVs = 0
	}

	summary := dcache.ClusterSummary{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Clustermap: dcache.ClustermapSummary{
			Readonly:        clusterMap.Readonly,
			Epoch:           clusterMap.Epoch,
			CreatedAt:       time.Unix(int64(clusterMap.CreatedAt), 0).UTC().Format(time.RFC3339),
			LastUpdatedAt:   time.Unix(int64(clusterMap.LastUpdatedAt), 0).UTC().Format(time.RFC3339),
			LastUpdatedBy:   clusterMap.LastUpdatedBy,
			Config:          clusterMap.Config,
			LastRefreshedAt: cm.GetClusterMapLastRefreshTime().UTC().Format(time.RFC3339),
		},
		Nodes: dcache.NodesSummary{
			Count:   int64(len(cm.GetAllNodes())),
			Offline: int64(len(cm.GetAllNodes()) - len(cm.GetOnlineNodes())),
		},
		RVs: dcache.RVsSummary{
			Count: int64(len(clusterMap.RVMap)),
		},
		MVs: dcache.MVsSummary{
			Count: int64(len(clusterMap.MVMap)),
		},
	}

	// RV summary
	for _, rv := range clusterMap.RVMap {
		switch rv.State {
		case dcache.StateOffline:
			summary.RVs.Offline++
		default:
			summary.RVs.Online++
		}
	}

	// MV summary
	for _, mv := range clusterMap.MVMap {
		switch mv.State {
		case dcache.StateOffline:
			summary.MVs.Offline++
		case dcache.StateDegraded:
			summary.MVs.Degraded++
		case dcache.StateSyncing:
			summary.MVs.Syncing++
		default:
			summary.MVs.Online++
		}
	}

	var err error
	pFile.buf, err = json.MarshalIndent(summary, "", "  ")
	if err != nil {
		log.Err("DebugFS::readClusterSummaryCallback: marshal failed: %v", err)
		common.Assert(false, err)
		return err
	}

	return nil
}

// proc file: cluster-summary.help
func readClusterSummaryHelpCallback(pFile *procFile) error {
	help := `{
	"refresh_clustermap": true|false
}
`

	pFile.buf = []byte(help)
	return nil
}

// Silence unused import errors for release builds.
func init() {
	common.IsValidUUID("00000000-0000-0000-0000-000000000000")
}
