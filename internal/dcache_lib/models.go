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

package dcachelib

type NodeManagerOptions struct {
	HbDuration   uint16
	HbPath       string
	MaxCacheSize uint64
	MaxMissedHbs uint8
}

type ClusterManagerOptions struct {
}

type Peer struct {
}

type VolumeState struct {
	Volume RawVolume `json:"volume,omitempty"`
	State  string    `json:"state,omitempty"`
}

type MirroredVolume struct {
	Name           string        `json:"name,omitempty"`
	RVmapWithState []VolumeState `json:"rv_map_with_state,omitempty"`
}

type RawVolume struct {
	Name             string `json:"name,omitempty"`
	HostNode         string `json:"host-node,omitempty"`
	FSID             string `json:"fsid,omitempty"`
	FDID             string `json:"fdid,omitempty"`
	State            string `json:"state,omitempty"`
	TotalSpaceGB     int    `json:"total_space_GB,omitempty"`
	AvailableSpaceGB int    `json:"available_space_GB,omitempty"`
}

type StateEnum string

const (
	StateOnline  StateEnum = "online"
	StateOffline StateEnum = "offline"
	StateDown    StateEnum = "down"
	StateReady   StateEnum = "ready"
	StateSyncing StateEnum = "syncing"
)

type ClusterConfig struct {
	Readonly      bool             `json:"readonly,omitempty"`
	State         StateEnum        `json:"state,omitempty"`
	Epoch         int64            `json:"epoch,omitempty"`
	CreatedAt     int64            `json:"created-at,omitempty"`
	LastUpdatedAt int64            `json:"last_updated_at,omitempty"`
	LastUpdatedBy string           `json:"last_updated_by,omitempty"`
	Config        DCacheConfig     `json:"config"`
	RVList        []RawVolume      `json:"rv-list"`
	MVList        []MirroredVolume `json:"mv-list"`
}

type DCacheConfig struct {
	MinNodes               int    `json:"min-nodes,omitempty"`
	ChunkSize              uint64 `json:"chunk-size,omitempty"`
	StripeSize             uint64 `json:"stripe-size,omitempty"`
	NumReplicas            uint8  `json:"num-replicas,omitempty"`
	MvsPerRv               uint64 `json:"mvs-per-rv,omitempty"`
	RvFullThreshold        uint64 `json:"rv-full-threshold,omitempty"`
	RvNearfullThreshold    uint64 `json:"rv-nearfull-threshold,omitempty"`
	HeartbeatSeconds       uint16 `json:"heartbeat-seconds,omitempty"`
	HeartbeatsTillNodeDown uint8  `json:"heartbeats-till-node-down,omitempty"`
	ClustermapEpoch        uint64 `json:"clustermap-epoch,omitempty"`
	RebalancePercentage    uint64 `json:"rebalance-percentage,omitempty"`
	SafeDeletes            bool   `json:"safe-deletes,omitempty"`
	CacheAccess            string `json:"cache-access,omitempty"`
}
