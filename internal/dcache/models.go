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

import (
	"time"
)

type MirroredVolume struct {
	State StateEnum            `json:"state"`
	RVs   map[string]StateEnum `json:"rvs"`
}

func (mv MirroredVolume) Equals(rhs *MirroredVolume) bool {
	if mv.State != rhs.State {
		return false
	}
	if len(mv.RVs) != len(rhs.RVs) {
		return false
	}
	for k, v := range mv.RVs {
		if rvState, ok := rhs.RVs[k]; !ok || rvState != v {
			return false
		}
	}
	return true
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
	StateSyncing   StateEnum = "syncing"
	StateReadOnly  StateEnum = "readOnly"
	//
	// Inband offline is a state for RVs that are not reachable from a given node,
	// but are not marked offline yet by the heartbeat mechanism.
	//
	StateInbandOffline StateEnum = "inband-offline"
)

// Please change the ClusterMapExport struct if you change this struct.
type ClusterMap struct {
	Readonly      bool                      `json:"readonly"`
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
	ChunkSizeMB            uint64 `json:"chunk-size-mb"`
	StripeWidth            uint64 `json:"stripe-width"`
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
	RingBasedMVPlacement   bool   `json:"ring-based-mv-placement"`
}

type FileState string

const (
	Ready   FileState = "ready"
	Writing FileState = "writing"
	Warming FileState = "warming"
)

const (
	DcacheDeletingFileNameSuffix = ".dcache.deleting"
	DummyWriteFileName           = ".dummy.write"
	//
	// Chunk index used for the metadata chunk is the following special value.
	// This value is chosen to be very large so that it does not conflict with an actual data chunk index.
	// With 4MiB chunk size, this value allows for files up to 4 ZiB in size.
	//
	MDChunkIdx = (int64(1) << 50)

	//
	// When reading the metadata chunk, we do not know the size, so we read this much data.
	// The metadata chunk is guaranteed to be smaller, so this should be sufficient.
	// This is needed for better asserting in the RPC handler.
	//
	MDChunkSize = 101
)

var (
	// This is MDChunkIdx*ChunkSizeInMiB, setup once we know the chunk size.
	MDChunkOffsetInMiB int64
)

type FileMetadata struct {
	Filename        string     `json:"filename"`
	State           FileState  `json:"-"`
	FileID          string     `json:"file_id"`
	Size            int64      `json:"-"`
	PartialSize     int64      `json:"-"`
	PartialSizeAt   time.Time  `json:"-"`
	WarmupSize      int64      `json:"warmup_size"`
	OpenCount       int        `json:"-"`
	ClusterMapEpoch int64      `json:"cluster_map_epoch"`
	FileLayout      FileLayout `json:"file_layout"`
	Sha1hash        []byte     `json:"sha256"`
}

// This is the content of the metadata chunk used to store size for partially written files.
// Note that the metadata file stores the file size as -1 until the file is closed after writing, so if a reader
// wants to read a file that's currently being written it needs to read this metadata chunk to know the partial
// size of the file. The partial size is updated in this chunk as the file is being written.
//
// Note: Update MDChunkSize if you change this struct. MDChunkSize must be > size of this struct.
type MetadataChunk struct {
	Size          int64     `json:"size"`
	LastUpdatedAt time.Time `json:"last_updated_at,omitzero"`
}

type FileLayout struct {
	ChunkSize   int64    `json:"chunk_size"`
	StripeWidth int64    `json:"stripe_width"`
	MVList      []string `json:"mv_list"`
}

type ComponentRVUpdateMessage struct {
	MvName     string
	RvName     string
	RvNewState StateEnum
	QueuedAt   time.Time
	Err        chan error
}
