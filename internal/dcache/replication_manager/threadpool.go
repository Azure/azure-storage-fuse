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

package replication_manager

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	cm "github.com/Azure/azure-storage-fuse/v2/internal/dcache/clustermap"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache/rpc"
	rpc_client "github.com/Azure/azure-storage-fuse/v2/internal/dcache/rpc/client"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache/rpc/gen-go/dcache/models"
)

type threadpool struct {
	// Number of workers in the thread pool.
	worker uint32

	// Wait group to wait for all workers to finish.
	wg sync.WaitGroup

	// Channel to hold pending RPC requests.
	items chan *workitem
}

type workitem struct {
	// Node ID of the target node to which the request is sent.
	targetNodeID string

	// RV name of the target node.
	rvName string

	// Put Chunk RPC request.
	putChunkReq *models.PutChunkRequest

	// TODO: Add other RPC request types as needed.
	// For now, we only handle client PutChunk requests, but it can be extended to handle
	// other requests like StartSync, EndSync, sync PutChunk, etc.

	// Channel to send the RPC response back to the caller.
	respChannel chan *responseItem
}

type responseItem struct {
	// Node ID of the target node that processed the request.
	// Used for logging purpose.
	targetNodeID string

	// RV name of the target node that processed the request.
	// Used for logging purpose.
	rvName string

	// Put Chunk RPC response.
	putChunkResp *models.PutChunkResponse

	// TODO: Add other RPC response types as needed.

	// Error returned from the RPC call.
	err error
}

// newThreadPool creates a new thread pool with the specified number of workers.
func newThreadPool(count uint32) *threadpool {
	log.Info("ReplicationManager::newThreadPool: Creating thread pool with %d workers", count)

	common.Assert(count > 0, count)

	return &threadpool{
		worker: count,
		items:  make(chan *workitem, count*2),
	}
}

func (tp *threadpool) start() {
	log.Info("threadpool[RM]::start: Starting thread pool with %d workers", tp.worker)

	for i := uint32(0); i < tp.worker; i++ {
		tp.wg.Add(1)
		go tp.do()
	}
}

func (tp *threadpool) stop() {
	log.Info("threadpool[RM]::stop: Stopping thread pool with %d workers", tp.worker)

	close(tp.items)
	tp.wg.Wait()
}

func (tp *threadpool) schedule(item *workitem) {
	common.Assert(item.isValid(), item.toString())

	// Send the work item to the channel for processing.
	tp.items <- item
}

func (tp *threadpool) do() {
	defer tp.wg.Done()

	for item := range tp.items {
		common.Assert(item.isValid(), item.toString())

		if item.putChunkReq != nil {
			resp, err := processPutChunk(item.targetNodeID, item.putChunkReq)

			item.respChannel <- &responseItem{
				targetNodeID: item.targetNodeID,
				rvName:       item.rvName,
				putChunkResp: resp,
				err:          err,
			}
		} else {
			// TODO: Handle other RPC request types as needed.

			// Unsupported request type, should not happen.
			common.Assert(false)
		}
	}
}

func (item *workitem) toString() string {
	if item == nil {
		return "<nil>"
	}

	return fmt.Sprintf("{targetNodeID: %s, rvName: %s, putChunkReq: %s, respChannel size: %d}",
		item.targetNodeID, item.rvName,
		rpc.PutChunkRequestToString(item.putChunkReq), cap(item.respChannel))
}

func (item *workitem) isValid() bool {
	if item == nil ||
		!common.IsValidUUID(item.targetNodeID) ||
		!cm.IsValidRVName(item.rvName) ||
		cap(item.respChannel) == 0 {
		return false
	}

	//TODO: when other RPC requests are added,
	// extend this check to check that only one RPC request is set.
	if item.putChunkReq == nil {
		return false
	}

	return true
}

func processPutChunk(targetNodeID string, req *models.PutChunkRequest) (*models.PutChunkResponse, error) {
	common.Assert(req != nil)
	common.Assert(common.IsValidUUID(targetNodeID), targetNodeID)

	log.Debug("ReplicationManager::processPutChunk: Sending PutChunk request to node %s: %s",
		targetNodeID, rpc.PutChunkRequestToString(req))

	ctx, cancel := context.WithTimeout(context.Background(), RPCClientTimeout*time.Second)
	defer cancel()

	return rpc_client.PutChunk(ctx, targetNodeID, req)
}
