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

package clustermanager

import "github.com/Azure/azure-storage-fuse/v2/internal/dcache"

// ClusterManager defines the interface for managing distributed cache, cluster configuration and heartbeat related APIs.
type ClusterManager interface {

	// Start cluster manager which expects cluster config and list of raw volumes.
	//1. Create cluster map if not present
	//2. Schedule heartbeat punching
	//3. Schedule clusterMap update for storage
	//4. Schedule clusterMap update for local cache
	start(*dcache.DCacheConfig, []dcache.RawVolume) error

	// Stop shuts down the cluster manager and releases any resources.
	//1. Cancel schedule of cluster update over storage and local cache
	//2. Cancel schedule of heartbeat punching
	stop() error

	//Update RV state to down and update MVs
	reportRVDown(rvName string) error

	//Update RV state to offline and update MVs
	reportRVFull(rvName string) error
}
