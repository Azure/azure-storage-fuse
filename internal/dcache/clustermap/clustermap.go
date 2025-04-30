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

package clustermap

import "github.com/Azure/azure-storage-fuse/v2/internal/dcache"

type Clustermap interface {

	//It will be used to close the notification channel
	closeNotificationChannel()

	//It will be used to consumer the event from the channel getNotificationChannel
	consume()

	//GetNotificationChannel returns a read‐only channel of events.
	getNotificationChannel() <-chan dcache.ClustermapEvent

	//It is used by publishers to push ClusterManagerEvent events.
	publishEvent(evt dcache.ClustermapEvent)

	//It will return online MVs Map <mvName, MV> as per local cache copy of cluster map
	getActiveMVs() map[string]dcache.MirroredVolume

	//It will return the cache config as per local cache copy of cluster map
	getCacheConfig() *dcache.DCacheConfig

	//It will return degraded MVs Map <mvName, MV> as per local cache copy of cluster map
	getDegradedMVs() map[string]dcache.MirroredVolume

	//It will return all the RVs Map <rvName, RV> for this particular node as per local cache copy of cluster map
	getMyRVs() map[string]dcache.RawVolume

	//It will return all the RVs Map <rvName, rvState> for the particular mvName as per local cache copy of cluster map
	getRVs(mvName string) map[string]dcache.StateEnum

	//It will check if the given nodeId is online as per local cache copy of cluster map
	isOnline(nodeId string) bool

	//It will evaluate the lowest number of RV for given rv Names
	lowestNumberRV(rvNames []string) string

	//It will return the IP address of the given nodeId as per local cache copy of cluster map
	nodeIdToIP(nodeId string) string

	//It will return the name of RV for the given RV FSID/Blkid as per local cache copy of cluster map
	rvIdToName(rvId string) string

	//It will return the RV FSID/Blkid of the given RV name as per local cache copy of cluster map
	rvNameToId(rvName string) string

	//It will return the nodeId/node uuid of the given RV name as per local cache copy of cluster map
	rVNameToNodeId(rvName string) string

	//It will return the IP address of the given RV name as per local cache copy of cluster map
	rVNameToIp(rvName string) string
}
