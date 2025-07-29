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

package stats

import (
	"encoding/json"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common"
)

//go:generate $ASSERT_REMOVER $GOFILE

// Metadata manager stats.
type MMStats struct {
	// Number of metadata folders created by this node.
	MetadataFoldersCreatedByThisNode int64 `json:"metadata_folders_created_by_this_node,omitempty"`

	// getBlobSafe() stats.
	GetBlobSafe struct {
		// Number of calls to getBlobSafe().
		Calls int64 `json:"calls"`
		// How many times getBlobSafe() had to retry as the blob LMT changed due to simultaneous update.
		Retries int64 `json:"retries,omitempty"`
		// How many times getBlobSafe() failed after exhausting all retries.
		Failures  int64  `json:"failures,omitempty"`
		LastError string `json:"last_error,omitempty"`
		MinUsec   *int64 `json:"min_usec,omitempty"`
		MaxUsec   int64  `json:"max_usec,omitempty"`
		TotalUsec int64  `json:"-"`
		AvgUsec   int64  `json:"avg_usec,omitempty"`
	} `json:"get_blob_safe"`

	StorageGetBlob struct {
		Calls     int64  `json:"calls"`
		Failures  int64  `json:"failures,omitempty"`
		MinUsec   *int64 `json:"min_usec,omitempty"`
		MaxUsec   int64  `json:"max_usec,omitempty"`
		TotalUsec int64  `json:"-"`
		AvgUsec   int64  `json:"avg_usec,omitempty"`
	} `json:"storage_get_blob"`

	StorageGetProperties struct {
		Calls     int64  `json:"calls"`
		Failures  int64  `json:"failures,omitempty"`
		MinUsec   *int64 `json:"min_usec,omitempty"`
		MaxUsec   int64  `json:"max_usec,omitempty"`
		TotalUsec int64  `json:"-"`
		AvgUsec   int64  `json:"avg_usec,omitempty"`
	} `json:"storage_get_properties"`

	StoragePutBlob struct {
		Calls     int64  `json:"calls"`
		Failures  int64  `json:"failures,omitempty"`
		MinUsec   *int64 `json:"min_usec,omitempty"`
		MaxUsec   int64  `json:"max_usec,omitempty"`
		TotalUsec int64  `json:"-"`
		AvgUsec   int64  `json:"avg_usec,omitempty"`
	} `json:"storage_put_blob"`

	StorageListDir struct {
		Calls     int64  `json:"calls"`
		Failures  int64  `json:"failures,omitempty"`
		MinUsec   *int64 `json:"min_usec,omitempty"`
		MaxUsec   int64  `json:"max_usec,omitempty"`
		TotalUsec int64  `json:"-"`
		AvgUsec   int64  `json:"avg_usec,omitempty"`
	} `json:"storage_list_dir"`

	Heartbeat struct {
		Published     int64     `json:"published"`
		LastPublished time.Time `json:"last_published"`
		SizeInBytes   int64     `json:"size_in_bytes"`
		MinGapUsec    *int64    `json:"min_gap_usec,omitempty"`
		MaxGapUsec    int64     `json:"max_gap_usec"`
		Fetched       int64     `json:"fetched,omitempty"`
	} `json:"heartbeat"`

	Clustermap struct {
		UpdateStartCalls int64     `json:"update_start_calls"`
		UpdateEndCalls   int64     `json:"update_end_calls"`
		LastUpdateStart  time.Time `json:"-"`
		LastUpdated      time.Time `json:"last_updated"`
		MinUpdateUsec    *int64    `json:"min_update_usec,omitempty"`
		MaxUpdateUsec    int64     `json:"max_update_usec,omitempty"`
		TotalUpdateUsec  int64     `json:"-"`
		AvgUpdateUsec    int64     `json:"avg_update_usec,omitempty"`
		GetCalls         int64     `json:"get_calls,omitempty"`
		LastError        string    `json:"last_error,omitempty"`
	} `json:"clustermap"`

	CreateFile struct {
		// Number of calls to createFileInit().
		InitCalls     int64  `json:"init_calls"`
		InitFailures  int64  `json:"init_failures,omitempty"`
		LastErrorInit string `json:"last_error_init,omitempty"`
		// Number of calls to createFileFinalize().
		FinalizeCalls     int64  `json:"finalize_calls"`
		FinalizeFailures  int64  `json:"finalize_failures,omitempty"`
		LastErrorFinalize string `json:"last_error_finalize,omitempty"`
		MinSizeBytes      *int64 `json:"min_size_bytes,omitempty"`
		MaxSizeBytes      int64  `json:"max_size_bytes,omitempty"`
		TotalSizeBytes    int64  `json:"-"`
		AvgSizeBytes      int64  `json:"avg_size_bytes,omitempty"`
	} `json:"create_file"`

	GetFile struct {
		TotalOpens       int64  `json:"total_opens"`
		SafeDeleteOpens  int64  `json:"safe_delete_opens"`
		SafeDeleteCloses int64  `json:"safe_delete_closes"`
		MaxOpenCount     int64  `json:"max_open_count"`
		LastError        string `json:"last_error,omitempty"`
	} `json:"get_file"`

	DeleteFile struct {
		Deleting  int64  `json:"deleting"`
		Deleted   int64  `json:"deleted"`
		LastError string `json:"last_error,omitempty"`
	} `json:"delete_file"`
}

// Duration wraps time.Duration to provide custom JSON marshaling/unmarshaling.
type Duration time.Duration

// MarshalJSON implements the json.Marshaler interface for Duration.
func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Duration(d).String()) // Marshal as a string (e.g., "5s")
}

// Cluster manager stats.
type CMStats struct {
	// Total time spent in cleaning up stale MVs from all the local RVs.
	RVCleanupDuration Duration `json:"rv_cleanup_duration,omitempty"`
	// How many MVs were deleted when this node started.
	MVsDeleted int64 `json:"mvs_deleted,omitempty"`
	// MVs that could not be deleted as deletion failed.
	MVsDeleteFailed                   int64         `json:"mvs_delete_failed,omitempty"`
	CreatedInitialClustermap          bool          `json:"created_initial_clustermap,omitempty"`
	UpdateClustermapWithMyRVsDuration Duration `json:"update_clustermap_with_my_rvs_duration,omitempty"`
	EnsureInitialClustermapDuration   Duration `json:"ensure_initial_clustermap_duration,omitempty"`
}

// Replication manager stats.
type RMStats struct {
	// TBD.
}

// File manager stats.
type FMStats struct {
	// TBD.
}

// RPC client/server stats.
type RPCStats struct {
	// TBD.
}

// DCacheStats is the aggregate of individual components' stats and some global stats.
type DCacheStats struct {
	// Global stats.
	NodeId    string    `json:"nodeid"`
	IPAddr    string    `json:"ipaddr"`
	HostName  string    `json:"hostname"`
	NodeStart time.Time `json:"starttime"`

	// Component stats.
	MM  MMStats  `json:"metadata_manager_stats"`
	CM  CMStats  `json:"cluster_manager_stats"`
	FM  FMStats  `json:"file_manager_stats"`
	RM  RMStats  `json:"replication_manager_stats"`
	RPC RPCStats `json:"rpc_stats"`
}

func (s *DCacheStats) Preprocess() {
	// Calculate averages.
	if s.MM.GetBlobSafe.Calls > 0 {
		s.MM.GetBlobSafe.AvgUsec = s.MM.GetBlobSafe.TotalUsec / s.MM.GetBlobSafe.Calls
	}

	if s.MM.StorageGetBlob.Calls > 0 {
		s.MM.StorageGetBlob.AvgUsec = s.MM.StorageGetBlob.TotalUsec / s.MM.StorageGetBlob.Calls
	}

	if s.MM.StorageGetProperties.Calls > 0 {
		s.MM.StorageGetProperties.AvgUsec = s.MM.StorageGetProperties.TotalUsec / s.MM.StorageGetProperties.Calls
	}

	if s.MM.StoragePutBlob.Calls > 0 {
		s.MM.StoragePutBlob.AvgUsec = s.MM.StoragePutBlob.TotalUsec / s.MM.StoragePutBlob.Calls
	}

	if s.MM.StorageListDir.Calls > 0 {
		s.MM.StorageListDir.AvgUsec = s.MM.StorageListDir.TotalUsec / s.MM.StorageListDir.Calls
	}

	filesCreatedFromThisNode := s.MM.CreateFile.FinalizeCalls - s.MM.CreateFile.FinalizeFailures
	if filesCreatedFromThisNode > 0 {
		s.MM.CreateFile.AvgSizeBytes = s.MM.CreateFile.TotalSizeBytes / filesCreatedFromThisNode
	}

	if s.MM.Clustermap.UpdateEndCalls > 0 {
		s.MM.Clustermap.AvgUpdateUsec = s.MM.Clustermap.TotalUpdateUsec / s.MM.Clustermap.UpdateEndCalls
	}
}

var Stats *DCacheStats

func init() {
	Stats = &DCacheStats{
		NodeStart: time.Now(),
	}

	common.IsValidUUID("00000000-0000-0000-0000-000000000000")
}
