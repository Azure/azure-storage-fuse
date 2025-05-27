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

package rpc

import (
	"fmt"
	"strings"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache/clustermap"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache/rpc/gen-go/dcache/models"
)

const (
	// defaultPort is the default port for the RPC server
	defaultPort = 9090
)

// return the node address for the given node ID
// the node address is of the form <ip>:<port>
func GetNodeAddressFromID(nodeID string) string {
	nodeAddress := fmt.Sprintf("%s:%d", clustermap.NodeIdToIP(nodeID), defaultPort)
	common.Assert(common.IsValidHostPort(nodeAddress), fmt.Sprintf("node address is not valid: %s", nodeAddress))
	return nodeAddress
}

// return the node ID of this node
func GetMyNodeUUID() string {
	nodeID, err := common.GetNodeUUID()
	common.Assert(err == nil, err)
	common.Assert(common.IsValidUUID(nodeID), nodeID)
	return nodeID
}

// convert *models.RVNameAndState to string
// used for logging
func ComponentRVsToString(rvs []*models.RVNameAndState) string {
	str := strings.Builder{}
	str.WriteString("[ ")
	for _, rv := range rvs {
		common.Assert(rv != nil, "Component RV is nil")
		str.WriteString(fmt.Sprintf("%+v ", *rv))
	}
	str.WriteString("]")
	return str.String()
}

// Given a list of component RVs where each component RV is expressed as models.RVNameAndState, convert
// it to a map with RV name as the key and RV state as the string.
func ComponentRVsListToMap(l []*models.RVNameAndState) map[string]string {
	m := make(map[string]string)

	for _, rv := range l {
		m[rv.Name] = rv.State
	}

	common.Assert(len(l) == len(m), len(l), len(m))
	return m
}

// This is the inverse of ComponentRVsListToMap().
func ComponentRVsMapToList(m map[string]string) []*models.RVNameAndState {
	l := make([]*models.RVNameAndState, 0)

	for name, state := range m {
		l = append(l, &models.RVNameAndState{
			Name:  name,
			State: state,
		})
	}

	common.Assert(len(l) == len(m), len(l), len(m))
	return l
}

// convert *models.HelloRequest to string
// used for logging
func HelloRequestToString(req *models.HelloRequest) string {
	return fmt.Sprintf("{SenderNodeID %s, ReceiverNodeID %s, Time %d, RVName %v, MV %v}",
		req.SenderNodeID, req.ReceiverNodeID, req.Time, req.RVName, req.MV)
}

func HelloResponseToString(resp *models.HelloResponse) string {
	return fmt.Sprintf("{ReceiverNodeID %s, Time %d, RVName %v, MV %v}",
		resp.ReceiverNodeID, resp.Time, resp.RVName, resp.MV)
}

// convert *models.GetChunkRequest to string
// used for logging
func GetChunkRequestToString(req *models.GetChunkRequest) string {
	return fmt.Sprintf("{SenderNodeID %v, Address %+v, OffsetInChunk %v, Length %v, ComponentRV %v}",
		req.SenderNodeID, *req.Address, req.OffsetInChunk, req.Length,
		ComponentRVsToString(req.ComponentRV))
}

func GetChunkResponseToString(resp *models.GetChunkResponse) string {
	return fmt.Sprintf("{Address %+v, DataLength: %v, ChunkWriteTime %v, TimeTaken %v, ComponentRV %v}",
		*resp.Chunk.Address, len(resp.Chunk.Data), resp.ChunkWriteTime, resp.TimeTaken,
		ComponentRVsToString(resp.ComponentRV))
}

// convert *models.PutChunkRequest to string
// exculde data and hash from the string to prevent it from being logged
func PutChunkRequestToString(req *models.PutChunkRequest) string {
	return fmt.Sprintf("{SenderNodeID %v, Address %+v, Length %v, SyncID %v, ComponentRV %v}",
		req.SenderNodeID, *req.Chunk.Address, req.Length, req.SyncID,
		ComponentRVsToString(req.ComponentRV))
}

func PutChunkResponseToString(resp *models.PutChunkResponse) string {
	return fmt.Sprintf("{TimeTaken %v, AvailableSpace %v, ComponentRV %v}",
		resp.TimeTaken, resp.AvailableSpace, ComponentRVsToString(resp.ComponentRV))
}

// convert *models.RemoveChunkRequest to string
// used for logging
func RemoveChunkRequestToString(req *models.RemoveChunkRequest) string {
	return fmt.Sprintf("{SenderNodeID %v, Address %+v, ComponentRV %v}",
		req.SenderNodeID, *req.Address, ComponentRVsToString(req.ComponentRV))
}

// convert *models.JoinMVRequest to string
// used for logging
func JoinMVRequestToString(req *models.JoinMVRequest) string {
	return fmt.Sprintf("{SenderNodeID %v, MV %v, RVName %v, ReserveSpace %v, ComponentRV %v}",
		req.SenderNodeID, req.MV, req.RVName, req.ReserveSpace, ComponentRVsToString(req.ComponentRV))
}

// convert *models.UpdateMVRequest to string
// used for logging
func UpdateMVRequestToString(req *models.UpdateMVRequest) string {
	return fmt.Sprintf("{SenderNodeID %v, MV %v, RVName %v ComponentRV %v}",
		req.SenderNodeID, req.MV, req.RVName, ComponentRVsToString(req.ComponentRV))
}

// convert *models.LeaveMVRequest to string
// used for logging
func LeaveMVRequestToString(req *models.LeaveMVRequest) string {
	return fmt.Sprintf("{SenderNodeID %v, MV %v, RVName %v, ComponentRV %v}",
		req.SenderNodeID, req.MV, req.RVName, ComponentRVsToString(req.ComponentRV))
}

// convert *models.StartSyncRequest to string
// used for logging
func StartSyncRequestToString(req *models.StartSyncRequest) string {
	return fmt.Sprintf("{SenderNodeID %v, MV %v, SourceRVName %v, TargetRVName %v, "+
		"ComponentRV %v, SyncSize %v}",
		req.SenderNodeID, req.MV, req.SourceRVName, req.TargetRVName,
		ComponentRVsToString(req.ComponentRV), req.SyncSize)
}

// convert *models.EndSyncRequest to string
// used for logging
func EndSyncRequestToString(req *models.EndSyncRequest) string {
	return fmt.Sprintf("{SenderNodeID %v, SyncID %v, MV %v, SourceRVName %v, "+
		"TargetRVName %v, ComponentRV %v, SyncSize %v}",
		req.SenderNodeID, req.SyncID, req.MV, req.SourceRVName,
		req.TargetRVName, ComponentRVsToString(req.ComponentRV), req.SyncSize)
}

// convert *models.GetMVSizeRequest to string
// used for logging
func GetMVSizeRequestToString(req *models.GetMVSizeRequest) string {
	return fmt.Sprintf("{SenderNodeID %v, MV %v, RVName %v}", req.SenderNodeID, req.MV, req.RVName)
}
