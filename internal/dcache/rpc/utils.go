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

// convert *models.RVNameAndState to string
// used for logging
func ComponentRVsToString(rvs []*models.RVNameAndState) string {
	var arr []models.RVNameAndState
	for _, rv := range rvs {
		common.Assert(rv != nil, "Component RV is nil")
		arr = append(arr, *rv)
	}
	return fmt.Sprintf("%+v", arr)
}

// convert *models.RVNameAndState to string
// exculde data and hash from the string to prevent it from being logged
func PutChunkRequestToString(req *models.PutChunkRequest) string {
	return fmt.Sprintf("Chunk address %+v, data length %v, isSync %v, Component RV %v",
		*req.Chunk.Address, req.Length, req.IsSync, ComponentRVsToString(req.ComponentRV))
}
