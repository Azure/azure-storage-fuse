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
	rpc_server "github.com/Azure/azure-storage-fuse/v2/internal/dcache/rpc/server"
)

//go:generate $ASSERT_REMOVER $GOFILE

type requestType int

const (
	invalidRequest requestType = iota // Make the zero value invalid to catch errors.
	putChunkRequest
	removeChunkRequest
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
	// Node ID of the target node to which the request should be sent.
	targetNodeID string

	// RV name of the target node.
	rvName string

	// Rpc Request.
	rpcReq any

	// Type to decode rpcReq.
	reqType requestType

	// Channel to send the RPC response back to the caller.
	respChannel chan *responseItem
}

type responseItem struct {
	// RV name of the target node that processed the request.
	// Used for logging purpose.
	rvName string

	// RPC response.
	rpcResp any

	// Error returned from the RPC call, nil if success.
	err error
}

// newThreadPool creates a new thread pool with the specified number of workers.
func newThreadPool(count uint32) *threadpool {
	log.Info("ReplicationManager::newThreadPool: Creating thread pool with %d workers", count)

	common.Assert(count > 0, count)

	//
	// Create the workitem channel to hold twice as many workitems as the number of workers.
	// Big enough to let enough workitems to be queued so that workers do not need to wait
	// for workitems.
	//
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

func (tp *threadpool) schedule(item *workitem, runInline bool) {
	common.Assert(item.isValid(), item.toString())

	//
	// If caller wants us to run the item in its context do that, else
	// add the work item to the channel for processing where it will be
	// dequeued and processed by one of the free workes.
	//
	if runInline {
		tp.runItem(item)
	} else {
		tp.items <- item
	}
}

// Run one threadpool item.
func (tp *threadpool) runItem(item *workitem) {
	common.Assert(item.isValid(), item.toString())

	respItem := &responseItem{
		rvName: item.rvName,
	}

	switch item.reqType {
	case putChunkRequest:
		putChunkReq := item.rpcReq.(*models.PutChunkRequest)
		respItem.rpcResp, respItem.err = processPutChunk(item.targetNodeID, putChunkReq)

	case removeChunkRequest:
		removeChunkRequest := item.rpcReq.(*models.RemoveChunkRequest)
		respItem.rpcResp, respItem.err = processRemoveChunk(item.targetNodeID, removeChunkRequest)

	default:
		common.Assert(false, *item)
	}

	item.respChannel <- respItem
}

func (tp *threadpool) do() {
	defer tp.wg.Done()

	//
	// As long as the workitem channel is not closed, keep dequeueing workitems and process them.
	//
	for item := range tp.items {
		tp.runItem(item)
	}
}

func (item *workitem) toString() string {
	if item == nil {
		return "<nil>"
	}

	var reqType, reqString string

	switch item.reqType {
	case putChunkRequest:
		reqType = "putChunkReq"
		reqString = rpc.PutChunkRequestToString(item.rpcReq.(*models.PutChunkRequest))
	case removeChunkRequest:
		reqType = "removeChunkReq"
		reqString = rpc.RemoveChunkRequestToString(item.rpcReq.(*models.RemoveChunkRequest))
	default:
		reqType = "invalid"
		reqString = "invalid"
	}

	return fmt.Sprintf("{targetNodeID: %s, rvName: %s, %s: %s, respChannel size: %d}",
		item.targetNodeID, item.rvName, reqType, reqString, cap(item.respChannel))
}

func (item *workitem) isValid() bool {
	if item == nil ||
		!common.IsValidUUID(item.targetNodeID) ||
		!cm.IsValidRVName(item.rvName) ||
		cap(item.respChannel) == 0 {
		return false
	}

	if item.rpcReq == nil {
		return false
	}

	if item.reqType == invalidRequest {
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

	//
	// If the node to which the PutChunk() RPC call must be made is local,
	// then we directly call the PutChunk() method using the local server's handler.
	// Else we call the PutChunk() RPC via the Thrift RPC client.
	//
	if targetNodeID == rpc.GetMyNodeUUID() {
		return rpc_server.PutChunkLocal(ctx, req)
	} else {
		return rpc_client.PutChunk(ctx, targetNodeID, req)
	}
}

func processRemoveChunk(targetNodeID string, req *models.RemoveChunkRequest) (*models.RemoveChunkResponse, error) {
	common.Assert(req != nil)
	common.Assert(common.IsValidUUID(targetNodeID), targetNodeID)

	log.Debug("ReplicationManager::processRemoveChunk: Sending RemoveChunk request to node %s: %s",
		targetNodeID, rpc.RemoveChunkRequestToString(req))

	// Removing all chunks may take time, so we wait longer than usual RPCs.
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	return rpc_client.RemoveChunk(ctx, targetNodeID, req)
}
