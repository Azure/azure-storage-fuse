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
	"io"
	"os"
	"sync"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	cm "github.com/Azure/azure-storage-fuse/v2/internal/dcache/clustermap"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache/rpc"
)

// CollectAllNodeLogs downloads log tarballs from every node in the current cluster into outDir.
// Returns map[nodeID]pathToTar and errors aggregated if some nodes fail.
func CollectAllNodeLogs(outDir string, numLogs int64) (map[string]string, error) {
	// Chunk size fixed to 16MB.
	const chunkSize = rpc.MaxLogChunkSize
	common.Assert(numLogs > 0, numLogs)

	log.Debug("CollectAllNodeLogs: Starting %d logs per node download in %s with chunk size of %d",
		numLogs, outDir, chunkSize)

	// Create the output directory
	err := os.MkdirAll(outDir, 0777)
	common.Assert(err == nil, err)

	nodeMap := cm.GetAllNodes()
	results := make(map[string]string)

	var mu sync.Mutex
	var wg sync.WaitGroup

	const workerCount = 10
	jobs := make(chan string, workerCount)
	errCh := make(chan error, len(nodeMap))

	// Workers
	for i := 0; i < workerCount; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()
			for nodeID := range jobs {
				ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
				path, err := GetLogs(ctx, nodeID, outDir, numLogs, chunkSize)
				cancel()

				if err != nil {
					common.Assert(path == nil)
					err1 := fmt.Errorf("failed to get logs for node %s [%v]", nodeID, err)
					log.Err("CollectAllNodeLogs: %v", err1)
					errCh <- err1
					continue
				}

				common.Assert(path != nil)
				mu.Lock()
				results[nodeID] = *path
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

	log.Debug("CollectAllNodeLogs: downloaded logs for %d/%d nodes into %s", len(results), len(nodeMap), outDir)
	return results, allErr
}

// Helper method to copy a file from srcPath to dstPath.
func copyFile(srcPath, dstPath string) error {
	src, err := os.Open(srcPath)
	defer src.Close()
	if err != nil {
		common.Assert(false, err)
		return err
	}

	dst, err := os.Create(dstPath)
	defer dst.Close()
	if err != nil {
		common.Assert(false, err)
		return err
	}

	if _, err = io.Copy(dst, src); err != nil {
		common.Assert(false, err)
		return err
	}

	// Flush to disk
	if err = dst.Sync(); err != nil {
		common.Assert(false, err)
		return err
	}

	// Preserve permissions
	if info, err := os.Stat(srcPath); err == nil {
		os.Chmod(dstPath, info.Mode())
	}

	return nil
}
