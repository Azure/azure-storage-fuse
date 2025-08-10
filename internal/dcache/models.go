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

package dcache

type MirroredVolume struct {
	State StateEnum            `json:"state"`
	RVs   map[string]StateEnum `json:"rvs"`
}

type RawVolume struct {
	NodeId         string    `json:"node_id"`
	IPAddress      string    `json:"ipaddr"`
	RvId           string    `json:"rvid"`
	FDId           int       `json:"fdid"`
	UDId           int       `json:"udid"`
	State          StateEnum `json:"state"`
	TotalSpace     uint64    `json:"total_space"`
	AvailableSpace uint64    `json:"available_space"`
	LocalCachePath string    `json:"local_cache_path"`
}

type StateEnum string

const (
	// "invalid" can be used to indicate an illegal state.
	StateInvalid   StateEnum = "invalid"
	StateOnline    StateEnum = "online"
	StateOffline   StateEnum = "offline"
	StateDegraded  StateEnum = "degraded"
	StateDown      StateEnum = "down"
	StateOutOfSync StateEnum = "outofsync"
	StateReady     StateEnum = "ready"
	StateSyncing   StateEnum = "syncing"
	StateReadOnly  StateEnum = "readOnly"
	StateChecking  StateEnum = "checking"
	//
	// Inband offline is a state for RVs that are not reachable from a given node,
	// but are not marked offline yet by the heartbeat mechanism.
	//
	StateInbandOffline StateEnum = "inband-offline"
)

// Please change the ClusterMapExport struct if you change this struct.
type ClusterMap struct {
	Readonly      bool                      `json:"readonly"`
	State         StateEnum                 `json:"state"`
	Epoch         int64                     `json:"epoch"`
	CreatedAt     int64                     `json:"created-at"`
	LastUpdatedAt int64                     `json:"last_updated_at"`
	LastUpdatedBy string                    `json:"last_updated_by"`
	Config        DCacheConfig              `json:"config"`
	RVMap         map[string]RawVolume      `json:"rv-list"`
	MVMap         map[string]MirroredVolume `json:"mv-list"`
}

// This struct is used for better interpreting the ClusterMap struct while reading the data as json.
// RVList and MVList are sorted by their names
// Refer to ClusterMap before making any changes to this struct.
type ClusterMapExport struct {
	Readonly      bool                        `json:"readonly"`
	State         StateEnum                   `json:"state"`
	Epoch         int64                       `json:"epoch"`
	CreatedAt     int64                       `json:"created-at"`
	LastUpdatedAt int64                       `json:"last_updated_at"`
	LastUpdatedBy string                      `json:"last_updated_by"`
	Config        DCacheConfig                `json:"config"`
	RVList        []map[string]RawVolume      `json:"rv-list"` // Used single element map for more readable clustermap output.
	MVList        []map[string]MirroredVolume `json:"mv-list"` // Used single element map for more readable clustermap output.
}

type HeartbeatData struct {
	InitialHB     bool        `json:"initial_hb"`
	Hostname      string      `json:"hostname"`
	IPAddr        string      `json:"ipaddr"`
	NodeID        string      `json:"nodeid"`
	LastHeartbeat uint64      `json:"last_heartbeat"`
	RVList        []RawVolume `json:"rv-list"`
}

type DCacheConfig struct {
	CacheId                string `json:"cache-id"`
	MinNodes               uint32 `json:"min-nodes"`
	ChunkSize              uint64 `json:"chunk-size"`
	StripeSize             uint64 `json:"stripe-size"`
	NumReplicas            uint32 `json:"num-replicas"`
	MaxRVs                 uint32 `json:"max-rvs"`
	MVsPerRV               uint64 `json:"mvs-per-rv"`
	RvFullThreshold        uint64 `json:"rv-full-threshold"`
	RvNearfullThreshold    uint64 `json:"rv-nearfull-threshold"`
	HeartbeatSeconds       uint16 `json:"heartbeat-seconds"`
	HeartbeatsTillNodeDown uint8  `json:"heartbeats-till-node-down"`
	ClustermapEpoch        uint64 `json:"clustermap-epoch"`
	RebalancePercentage    uint8  `json:"rebalance-percentage"`
	SafeDeletes            bool   `json:"safe-deletes"`
	CacheAccess            string `json:"cache-access"`
	IgnoreFD               bool   `json:"ignore-fd"`
	IgnoreUD               bool   `json:"ignore-ud"`
}

type FileState string

const (
	Ready   FileState = "ready"
	Writing FileState = "writing"
)

const (
	DcacheDeletingFileNameSuffix = ".dcache.deleting"
)

type FileMetadata struct {
	Filename        string     `json:"filename"`
	State           FileState  `json:"-"`
	FileID          string     `json:"file_id"`
	Size            int64      `json:"-"`
	OpenCount       int        `json:"-"`
	ClusterMapEpoch int64      `json:"cluster_map_epoch"`
	FileLayout      FileLayout `json:"file_layout"`
	Sha1hash        []byte     `json:"sha256"`
}

type FileLayout struct {
	ChunkSize  int64    `json:"chunk_size"`
	StripeSize int64    `json:"stripe_size"`
	MVList     []string `json:"mv_list"`
}

type ComponentRVUpdateMessage struct {
	MvName     string
	RvName     string
	RvNewState StateEnum
	Err        chan error
}
