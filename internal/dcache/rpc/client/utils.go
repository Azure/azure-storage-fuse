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

package rpc_client

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache"
	cm "github.com/Azure/azure-storage-fuse/v2/internal/dcache/clustermap"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache/rpc"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache/rpc/gen-go/dcache/models"
)

// CollectAllNodeLogs downloads log tarballs from every node in the current cluster into outDir.
// Returns map[nodeID]pathToTar and errors aggregated if some nodes fail.
func CollectAllNodeLogs(outDir string, numLogs int64) (map[string]string, error) {
	const chunkSize = rpc.LogChunkSize
	common.Assert(numLogs > 0, numLogs)

	log.Debug("CollectAllNodeLogs: Starting %d logs per node download in %s with chunk size of %d",
		numLogs, outDir, chunkSize)

	// Create the output directory
	err := os.MkdirAll(outDir, 0777)
	if err != nil {
		err = fmt.Errorf("CollectAllNodeLogs: failed to create output dir %s: %v", outDir, err)
		log.Err("%v", err)
		return nil, err
	}

	nodeMap := cm.GetAllNodes()
	results := make(map[string]string)

	if len(nodeMap) == 0 {
		common.Assert(false)
		return results, fmt.Errorf("CollectAllNodeLogs: no nodes found in cluster")
	}

	var mu sync.Mutex
	var wg sync.WaitGroup

	//
	// We can start log collection in parallel on lot of nodes as it doesn't load any single
	// node. It'll be limited by ingress network b/w and disk IO on the requesting node.
	//
	workerCount := min(1000, len(nodeMap))
	jobs := make(chan string, workerCount)
	errCh := make(chan error, len(nodeMap))

	// Workers
	for i := 0; i < workerCount; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()
			for nodeID := range jobs {
				//
				// GetLogs() will hit RPC timeout before our context timeout of 300s, but
				// we still want to keep this higher as RPC timeout an be increased in future.
				//
				ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
				path, err := GetLogs(ctx, nodeID, outDir, numLogs, chunkSize)
				cancel()

				if err != nil {
					common.Assert(path == "", path)
					err1 := fmt.Errorf("failed to get logs for node %s [%v]", nodeID, err)
					log.Err("CollectAllNodeLogs: %v", err1)
					errCh <- err1
					continue
				}

				common.Assert(path != "")
				mu.Lock()
				results[nodeID] = path
				mu.Unlock()
			}
		}()
	}

	// Feed jobs
	for nodeID, _ := range nodeMap {
		jobs <- nodeID
	}
	close(jobs)

	// Wait for workers to finish
	wg.Wait()
	close(errCh)

	common.Assert(len(results) <= len(nodeMap), len(results), len(nodeMap))

	var allErr error
	for e := range errCh {
		if allErr == nil {
			allErr = e
		} else {
			allErr = fmt.Errorf("%v; %v", allErr, e)
		}
	}

	log.Info("CollectAllNodeLogs: downloaded logs for %d/%d nodes into %s: [%v]",
		len(results), len(nodeMap), outDir, allErr)

	return results, allErr
}

// GetNodesStats collects stats from all nodes in the cluster via RPCs and
// aggregates them into a NodesStats structure.
func GetNodesStats() (*dcache.NodesStats, error) {
	log.Debug("GetNodesStats: Starting nodes stats collection")

	nodeMap := cm.GetAllNodes()
	if len(nodeMap) == 0 {
		common.Assert(false)
		return nil, fmt.Errorf("GetNodesStats: no nodes found in cluster")
	}

	nodesStats := &dcache.NodesStats{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Count:     int64(len(nodeMap)),
		Aggregate: &dcache.NodesAggregate{},
		Errors:    make(map[string]string),
	}

	var wg sync.WaitGroup
	var mu sync.Mutex

	//
	// We can start stats collection in parallel on lot of nodes as it doesn't load any single
	// node. Also, there's not a lof of incoming data on the calling node.
	//
	workerCount := min(1000, len(nodeMap))
	jobs := make(chan string, workerCount)

	// Workers
	for i := 0; i < workerCount; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()
			for nodeID := range jobs {
				nodeInfo := &dcache.NodeInfo{
					NodeID: nodeID,
				}

				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				resp, err := GetNodeStats(ctx, nodeID, &models.GetNodeStatsRequest{})
				cancel()

				if err != nil {
					mu.Lock()
					nodesStats.Errors[nodeID] = err.Error()
					mu.Unlock()
					continue
				}

				nodeInfo.HostName = resp.HostName
				nodeInfo.IPAddress = resp.IpAddress
				nodeInfo.MemUsed = bytesToReadable(resp.MemUsedBytes)
				nodeInfo.MemTotal = bytesToReadable(resp.MemTotalBytes)
				nodeInfo.PercentMemUsed = resp.PercentMemUsed

				mu.Lock()
				nodesStats.Aggregate.MemUsedBytes += resp.MemUsedBytes
				nodesStats.Aggregate.MemTotalBytes += resp.MemTotalBytes
				nodesStats.Nodes = append(nodesStats.Nodes, nodeInfo)
				mu.Unlock()
			}
		}()
	}

	// Feed jobs
	for nodeID, _ := range nodeMap {
		jobs <- nodeID
	}
	close(jobs)

	// Wait for workers to finish
	wg.Wait()

	// Prepare aggregate stats
	nodesStats.Aggregate.MemUsed = bytesToReadable(nodesStats.Aggregate.MemUsedBytes)
	nodesStats.Aggregate.MemTotal = bytesToReadable(nodesStats.Aggregate.MemTotalBytes)
	if nodesStats.Aggregate.MemTotalBytes > 0 {
		percentUsed := (float64(nodesStats.Aggregate.MemUsedBytes) /
			float64(nodesStats.Aggregate.MemTotalBytes)) * 100.0
		nodesStats.Aggregate.PercentMemUsed = fmt.Sprintf("%.2f%%", percentUsed)
	}

	return nodesStats, nil
}

// Convert uint64 value in bytes to readable format
func bytesToReadable(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}

	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	return fmt.Sprintf("%.2f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
