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
)

//go:generate $ASSERT_REMOVER $GOFILE

// Define Duration as an alias for time.Duration to provide custom JSON marshaling that formats it as a pretty
// string of the form "10.3412ms" instead of the number of nanoseconds.
type Duration time.Duration

// MarshalJSON implements the json.Marshaler interface for Duration.
func (d Duration) MarshalJSON() ([]byte, error) {
	// Marshal as a string (e.g., "29.07ms")
	return json.Marshal(time.Duration(d).String())
}

// Metadata manager stats.
// These should help us understand the performance of the metadata manager and its interactions with Azure // storage.
//
// Note: time.Time elements are maked omitzero which is supported starting from Go 1.24, so for older go versions
//       empty time.Time values will not be omitted from the JSON output.

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
		Failures int64 `json:"failures,omitempty"`
		// Minimum, maximum, total and average duration of all getBlobSafe() calls.
		MinTime   *Duration `json:"min_time,omitempty"`
		MaxTime   Duration  `json:"max_time,omitempty"`
		TotalTime Duration  `json:"-"`
		AvgTime   Duration  `json:"avg_time,omitempty"`
		LastError string    `json:"last_error,omitempty"`
	} `json:"get_blob_safe"`

	// Storage GetBlob call stats.
	StorageGetBlob struct {
		Calls     int64     `json:"calls"`
		Failures  int64     `json:"failures,omitempty"`
		MinTime   *Duration `json:"min_time,omitempty"`
		MaxTime   Duration  `json:"max_time,omitempty"`
		TotalTime Duration  `json:"-"`
		AvgTime   Duration  `json:"avg_time,omitempty"`
		LastError string    `json:"last_error,omitempty"`
	} `json:"storage_get_blob"`

	// Storage GetProperties call stats.
	StorageGetProperties struct {
		Calls     int64     `json:"calls"`
		Failures  int64     `json:"failures,omitempty"`
		MinTime   *Duration `json:"min_time,omitempty"`
		MaxTime   Duration  `json:"max_time,omitempty"`
		TotalTime Duration  `json:"-"`
		AvgTime   Duration  `json:"avg_time,omitempty"`
		LastError string    `json:"last_error,omitempty"`
	} `json:"storage_get_properties"`

	// Storage PutBlob call stats.
	StoragePutBlob struct {
		Calls     int64     `json:"calls"`
		Failures  int64     `json:"failures,omitempty"`
		MinTime   *Duration `json:"min_time,omitempty"`
		MaxTime   Duration  `json:"max_time,omitempty"`
		TotalTime Duration  `json:"-"`
		AvgTime   Duration  `json:"avg_time,omitempty"`
		LastError string    `json:"last_error,omitempty"`
	} `json:"storage_put_blob"`

	// Storage ListDir call stats.
	StorageListDir struct {
		Calls     int64     `json:"calls"`
		Failures  int64     `json:"failures,omitempty"`
		MinTime   *Duration `json:"min_time,omitempty"`
		MaxTime   Duration  `json:"max_time,omitempty"`
		TotalTime Duration  `json:"-"`
		AvgTime   Duration  `json:"avg_time,omitempty"`
		LastError string    `json:"last_error,omitempty"`
	} `json:"storage_list_dir"`

	// Heartbeat stats.
	Heartbeat struct {
		Published       int64     `json:"published"`
		LastPublished   time.Time `json:"last_published,omitzero"`
		PublishFailures int64     `json:"publish_failures,omitempty"`
		SizeInBytes     int64     `json:"size_in_bytes"`
		MinGap          *Duration `json:"min_gap,omitempty"`
		MaxGap          Duration  `json:"max_gap"`
		TotalGap        Duration  `json:"-"`
		AvgGap          Duration  `json:"avg_gap,omitempty"`
		// How many heartbeats were fetched (only cluster_manager leader fetches heartbeats).
		Fetched       int64  `json:"fetched,omitempty"`
		FetchFailures int64  `json:"fetch_failures,omitempty"`
		LastError     string `json:"last_error,omitempty"`
	} `json:"heartbeat"`

	Clustermap struct {
		// How many times clustermap was updated (only cluster_manager leader updates clustermap).
		UpdateStartCalls int64 `json:"update_start_calls"`
		UpdateEndCalls   int64 `json:"update_end_calls"`
		// Either update start or end failures.
		UpdateFailures  int64     `json:"update_failures,omitempty"`
		LastUpdateStart time.Time `json:"-"`
		// When was the global clustermap last updated?
		// Only cluster_manager leader updates the global clustermap.
		LastUpdated     time.Time `json:"last_updated,omitzero"`
		MinUpdateTime   *Duration `json:"min_update_time,omitempty"`
		MaxUpdateTime   Duration  `json:"max_update_time,omitempty"`
		TotalUpdateTime Duration  `json:"-"`
		AvgUpdateTime   Duration  `json:"avg_update_time,omitempty"`
		// How many time clustermap was fetched.
		GetCalls    int64  `json:"get_calls,omitempty"`
		GetFailures int64  `json:"get_failures,omitempty"`
		LastError   string `json:"last_error,omitempty"`
	} `json:"clustermap"`

	// File creation involves two steps: init, finalize.
	// We maintain stats for each of these steps.
	CreateFile struct {
		// Number of calls to createFileInit().
		InitCalls     int64  `json:"init_calls"`
		InitFailures  int64  `json:"init_failures,omitempty"`
		LastErrorInit string `json:"last_error_init,omitempty"`

		// Number of calls to createFileFinalize().
		FinalizeCalls     int64  `json:"finalize_calls"`
		FinalizeFailures  int64  `json:"finalize_failures,omitempty"`
		LastErrorFinalize string `json:"last_error_finalize,omitempty"`

		MinSizeBytes   *int64 `json:"min_size_bytes,omitempty"`
		MaxSizeBytes   int64  `json:"max_size_bytes,omitempty"`
		TotalSizeBytes int64  `json:"-"`
		AvgSizeBytes   int64  `json:"avg_size_bytes,omitempty"`
	} `json:"create_file"`

	GetFile struct {
		TotalOpens       int64  `json:"total_opens"`
		SafeDeleteOpens  int64  `json:"safe_delete_opens"`
		SafeDeleteCloses int64  `json:"safe_delete_closes"`
		MaxOpenCount     int64  `json:"max_open_count"`
		Failures         int64  `json:"failures,omitempty"`
		LastError        string `json:"last_error,omitempty"`
	} `json:"get_file"`

	DeleteFile struct {
		Deleting  int64  `json:"deleting"`
		Deleted   int64  `json:"deleted"`
		Failures  int64  `json:"failures,omitempty"`
		LastError string `json:"last_error,omitempty"`
	} `json:"delete_file"`
}

// Cluster manager stats.
// We maintain stats for various tasks that cluster_manager performs, such as
// - Initial tasks when the node starts, such as cleaning up stale MVs, creating the initial clustermap, etc.
// - Local clustermap refresh.
// - Various workflows such as new-mv, fix-mv, etc.
type CMStats struct {
	// Various tasks performed by cluster_manager when the node starts.
	Startup struct {
		// Total time spent in cleaning up stale MVs from all the local RVs.
		RVCleanupDuration Duration `json:"rv_cleanup_duration,omitempty"`
		// How many MVs were deleted when this node started.
		MVsDeleted int64 `json:"mvs_deleted,omitempty"`
		// MVs that could not be deleted as deletion failed for one or more chunks or the MV directory.
		MVsDeleteFailed int64 `json:"mvs_delete_failed,omitempty"`
		// Did this node create the initial clustermap? The first node to come up that finds no clustermap
		// creates the initial clustermap.
		CreatedInitialClustermap bool `json:"created_initial_clustermap,omitempty"`
		// Time taken to update the clustermap with the local RVs.
		// Before a new node can join the cluster, it needs to add it's local RVs to the clustermap.
		// It has to do it atomically, potentially along with other nodes that are joining the cluster.
		UpdateClustermapWithMyRVsDuration Duration `json:"update_clustermap_with_my_rvs_duration"`
		// Total time taken to ensure the initial clustermap is created, with the local RVs and node joins
		// the cluster.
		EnsureInitialClustermapDuration Duration `json:"ensure_initial_clustermap_duration"`
	} `json:"startup"`

	// Local clustermap stats.
	LocalClustermap struct {
		// Size of the local clustermap in bytes.
		SizeInBytes int64 `json:"size_in_bytes"`
		// Epoch of the local clustermap.
		Epoch int64 `json:"epoch"`
		// How many times local clustermap was refreshed (from the global clustermap) on this node?
		TimesUpdated        int64     `json:"times_updated"`
		LastUpdated         time.Time `json:"last_updated,omitzero"`
		TimeSinceLastUpdate Duration  `json:"time_since_last_update,omitempty"`
		// How many times local clustermap update failed (either failed to fetch or failed to update the
		// local copy)? LastError will have details.
		UpdateFailures int64 `json:"update_failures,omitempty"`
		// How many time clustermap was refreshed but it was found to be unchanged from our saved local copy?
		Unchanged int64 `json:"unchanged,omitempty"`
		// Minimum, maximum, total and average duration of all local clustermap updates.
		MinTime   *Duration `json:"min_time,omitempty"`
		MaxTime   Duration  `json:"max_time,omitempty"`
		TotalTime Duration  `json:"-"`
		AvgTime   Duration  `json:"avg_time,omitempty"`
		LastError string    `json:"last_error,omitempty"`
	} `json:"local_clustermap"`

	// Global clustermap stats.
	StorageClustermap struct {
		// Node ID of the node that is the leader (as per the latest clustermap copy that we have).
		Leader string `json:"leader,omitempty"`
		// Is this node the cluster_manager leader?
		IsLeader       bool   `json:"is_leader"`
		LeaderSwitches int64  `json:"leader_switches,omitempty"`
		LastError      string `json:"last_error,omitempty"`
		Calls          int64  `json:"calls"`
		Failures       int64  `json:"failures,omitempty"`
	} `json:"storage_clustermap,omitempty"`

	// updateMVList() stats.
	UpdateMVList struct {
		// How many times updateMVList() was called by the periodic cluster_manager thread to run various MV
		// workflows, such as new-mv, fix-mv, etc. Only leader node run this.
		// Non-leaders call it from batchUpdateComponentRVState(), but we don't count those calls here.
		Calls     int64     `json:"calls"`
		Failures  int64     `json:"failures,omitempty"`
		MinTime   *Duration `json:"min_time,omitempty"`
		MaxTime   Duration  `json:"max_time,omitempty"`
		TotalTime Duration  `json:"-"`
		AvgTime   Duration  `json:"avg_time,omitempty"`
		LastError string    `json:"last_error,omitempty"`
	} `json:"update_mv_list"`

	// New MV workflow stats.
	// These are only valid for the cluster_manager leader node.
	NewMV struct {
		MVsPerRV    int64 `json:"mvs_per_rv,omitempty"`
		NumReplicas int64 `json:"num_replicas,omitempty"`
		// Total RVs (from all nodes) at our disposal.
		TotalRVs int64 `json:"total_rvs,omitempty"`
		// Total MVs, created from all the RVs that we have.
		TotalMVs int64 `json:"total_mvs,omitempty"`
		// With TotalRVs, NumReplicas and MVsPerRV, how many MVs can the cluster have?
		MaxMVsPossible int64 `json:"max_mvs_possible,omitempty"`
		// How many MVs were added by this node?
		// Only a cluster_manager leader can add MVs, so this value is meaningful only for the leader node.
		NewMVsAdded int64 `json:"new_mvs_added,omitempty"`
		// Available node is one that has at least one RV that can host at least one new MV (remember MVsPerRV?).
		AvailableNodes int64 `json:"available_nodes,omitempty"`
		// Time when the last MV was added.
		LastMVAddedAt time.Time `json:"last_mv_added_at,omitzero"`
		// Total time taken to add new MVs.
		TimeTaken Duration `json:"time_taken,omitempty"`

		// Details of JoinMV calls made for adding component RVs to the new MVs.
		JoinMV struct {
			Calls     int64     `json:"calls"`
			Failures  int64     `json:"failures,omitempty"`
			MinTime   *Duration `json:"min_time,omitempty"`
			MaxTime   Duration  `json:"max_time,omitempty"`
			TotalTime Duration  `json:"-"`
			AvgTime   Duration  `json:"avg_time,omitempty"`
			LastError string    `json:"last_error,omitempty"`
		} `json:"join_mv"`
	} `json:"new_mv"`

	// Fix MV workflow stats.
	FixMV struct {
		// fixMV() replaced all offline component RVs of these MVs with new RVs.
		MVsFixed int64 `json:"mvs_fixed,omitempty"`
		// fixMV() replaced at least one offline component RV of these MVs with new RVs.
		MVsPartiallyFixed int64 `json:"mvs_partially_fixed,omitempty"`
		// fixMV() could not replace even one offline component RV of these MVs with new RVs.
		MVsNotFixed int64 `json:"mvs_not_fixed,omitempty"`
		// fixMV() could find replacement RVs but joinMV() failed for these MVs.
		MVsFixFailedDueToJoinMV int64 `json:"mvs_fix_failed_due_to_join_mv,omitempty"`
		// How many RVs were cumulatively replaced.
		RVsReplaced int64 `json:"rvs_replaced,omitempty"`
		// Count of RVs for which we could not find a replacement RV, when fixMV() is called.
		// Such MVs will remain degraded till the next periodic updateMVList() call..
		NoReplacementRVs int64 `json:"no_replacement_rvs,omitempty"`
		// Total calls to fixMV().
		Calls int64 `json:"calls"`
		// Minimum, maximum, total and average duration of all fixMV() calls.
		MinTime   *Duration `json:"min_time,omitempty"`
		MaxTime   Duration  `json:"max_time,omitempty"`
		TotalTime Duration  `json:"-"`
		AvgTime   Duration  `json:"avg_time,omitempty"`
		JoinMV    struct {
			Calls     int64     `json:"calls"`
			Failures  int64     `json:"failures,omitempty"`
			MinTime   *Duration `json:"min_time,omitempty"`
			MaxTime   Duration  `json:"max_time,omitempty"`
			TotalTime Duration  `json:"-"`
			AvgTime   Duration  `json:"avg_time,omitempty"`
			LastError string    `json:"last_error,omitempty"`
		} `json:"join_mv"`

		UpdateMV struct {
			Calls     int64     `json:"calls"`
			Failures  int64     `json:"failures,omitempty"`
			MinTime   *Duration `json:"min_time,omitempty"`
			MaxTime   Duration  `json:"max_time,omitempty"`
			TotalTime Duration  `json:"-"`
			AvgTime   Duration  `json:"avg_time,omitempty"`
			LastError string    `json:"last_error,omitempty"`
		} `json:"update_mv"`
	} `json:"fix_mv"`

	// TODO: Add more stats.
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
		s.MM.GetBlobSafe.AvgTime =
			Duration(float64(s.MM.GetBlobSafe.TotalTime) / float64(s.MM.GetBlobSafe.Calls))
	}

	if s.MM.StorageGetBlob.Calls > 0 {
		s.MM.StorageGetBlob.AvgTime =
			Duration(float64(s.MM.StorageGetBlob.TotalTime) / float64(s.MM.StorageGetBlob.Calls))
	}

	if s.MM.StorageGetProperties.Calls > 0 {
		s.MM.StorageGetProperties.AvgTime =
			Duration(float64(s.MM.StorageGetProperties.TotalTime) / float64(s.MM.StorageGetProperties.Calls))
	}

	if s.MM.StoragePutBlob.Calls > 0 {
		s.MM.StoragePutBlob.AvgTime =
			Duration(float64(s.MM.StoragePutBlob.TotalTime) / float64(s.MM.StoragePutBlob.Calls))
	}

	if s.MM.StorageListDir.Calls > 0 {
		s.MM.StorageListDir.AvgTime =
			Duration(float64(s.MM.StorageListDir.TotalTime) / float64(s.MM.StorageListDir.Calls))
	}

	filesCreatedFromThisNode := s.MM.CreateFile.FinalizeCalls - s.MM.CreateFile.FinalizeFailures
	if filesCreatedFromThisNode > 0 {
		s.MM.CreateFile.AvgSizeBytes = s.MM.CreateFile.TotalSizeBytes / filesCreatedFromThisNode
	}

	if s.MM.Heartbeat.Published > 0 {
		s.MM.Heartbeat.AvgGap =
			Duration(float64(s.MM.Heartbeat.TotalGap) / float64(s.MM.Heartbeat.Published))
	}

	if s.MM.Clustermap.UpdateEndCalls > 0 {
		s.MM.Clustermap.AvgUpdateTime =
			Duration(float64(s.MM.Clustermap.TotalUpdateTime) / float64(s.MM.Clustermap.UpdateEndCalls))
	}

	localClustermapUpdateCalls := s.CM.LocalClustermap.TimesUpdated - s.CM.LocalClustermap.UpdateFailures
	if localClustermapUpdateCalls > 0 {
		s.CM.LocalClustermap.AvgTime =
			Duration(float64(s.CM.LocalClustermap.TotalTime) / float64(localClustermapUpdateCalls))
	}

	if !s.CM.LocalClustermap.LastUpdated.IsZero() {
		s.CM.LocalClustermap.TimeSinceLastUpdate = Duration(time.Since(s.CM.LocalClustermap.LastUpdated))
	}

	if s.CM.NewMV.JoinMV.Calls > 0 {
		s.CM.NewMV.JoinMV.AvgTime =
			Duration(float64(s.CM.NewMV.JoinMV.TotalTime) / float64(s.CM.NewMV.JoinMV.Calls))
	}

	if s.NodeId == s.CM.StorageClustermap.Leader {
		s.CM.StorageClustermap.IsLeader = true
	} else {
		s.CM.StorageClustermap.IsLeader = false
	}

	if s.CM.UpdateMVList.Calls > 0 {
		s.CM.UpdateMVList.AvgTime =
			Duration(float64(s.CM.UpdateMVList.TotalTime) / float64(s.CM.UpdateMVList.Calls))
	}

	if s.CM.FixMV.Calls > 0 {
		s.CM.FixMV.AvgTime =
			Duration(float64(s.CM.FixMV.TotalTime) / float64(s.CM.FixMV.Calls))
	}

	if s.CM.FixMV.JoinMV.Calls > 0 {
		s.CM.FixMV.JoinMV.AvgTime =
			Duration(float64(s.CM.FixMV.JoinMV.TotalTime) / float64(s.CM.FixMV.JoinMV.Calls))
	}

	if s.CM.FixMV.UpdateMV.Calls > 0 {
		s.CM.FixMV.UpdateMV.AvgTime =
			Duration(float64(s.CM.FixMV.UpdateMV.TotalTime) / float64(s.CM.FixMV.UpdateMV.Calls))
	}
}

var Stats *DCacheStats

func init() {
	Stats = &DCacheStats{
		NodeStart: time.Now(),
	}
}
