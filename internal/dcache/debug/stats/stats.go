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
	"math"
	"sync/atomic"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common"
)

//go:generate $ASSERT_REMOVER $GOFILE

type AtomicCounter struct {
	Value atomic.Int64
}

// Custom MarshalJSON implementation
func (a *AtomicCounter) MarshalJSON() ([]byte, error) {
	// Safely load the atomic value
	currentValue := a.Value.Load()
	// Marshal the value as JSON
	return json.Marshal(currentValue)
}

// Custom UnmarshalJSON implementation
func (a *AtomicCounter) UnmarshalJSON(data []byte) error {
	var value int64
	// Unmarshal the JSON data into a temporary variable
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	// Safely store the value into the atomic variable
	a.Value.Store(value)
	return nil
}

// Define Duration as an alias for time.Duration to provide custom JSON marshaling that formats it as a pretty
// string of the form "10.3412ms" instead of the number of nanoseconds.
type Duration time.Duration

// MarshalJSON implements the json.Marshaler interface for Duration.
func (d Duration) MarshalJSON() ([]byte, error) {
	// Marshal as a string (e.g., "29.07ms")
	return json.Marshal(time.Duration(atomic.LoadInt64((*int64)(&d))).String())
}

var ZeroDuration Duration = Duration(0)

func StoreMinTime(currentMin *Duration, newTime Duration) {
	loadedMin := Duration(atomic.LoadInt64((*int64)(currentMin)))
	if loadedMin == ZeroDuration || newTime < loadedMin {
		atomic.StoreInt64((*int64)(currentMin), int64(newTime))
	}
}

func StoreMaxTime(currentMax *Duration, newTime Duration) {
	loadedMax := Duration(atomic.LoadInt64((*int64)(currentMax)))
	if newTime > loadedMax {
		atomic.StoreInt64((*int64)(currentMax), int64(newTime))
	}
}

type AtomicTime struct {
	ptr atomic.Pointer[time.Time]
}

func (a *AtomicTime) Store(t time.Time) {
	copy := t
	a.ptr.Store(&copy)
}

func (a *AtomicTime) Load() time.Time {
	p := a.ptr.Load()
	if p == nil {
		return time.Time{}
	}
	return *p
}

// IsZero reports whether the stored time is the zero value or unset.
func (a *AtomicTime) IsZero() bool {
	p := a.ptr.Load()
	return p == nil || p.IsZero()
}

func (a *AtomicTime) MarshalJSON() ([]byte, error) {
	t := a.Load()
	return json.Marshal(t)
}

func (a *AtomicTime) UnmarshalJSON(b []byte) error {
	var t time.Time
	if err := json.Unmarshal(b, &t); err != nil {
		return err
	}
	a.Store(t)
	return nil
}

// Metadata manager stats.
// These should help us understand the performance of the metadata manager and its interactions with Azure // storage.
//
// Note: time.Time elements are marked omitzero which is supported starting from Go 1.24, so for older go versions
//       empty time.Time values will not be omitted from the JSON output.

type MMStats struct {
	// Number of metadata folders created by this node.
	MetadataFoldersCreatedByThisNode AtomicCounter `json:"metadata_folders_created_by_this_node,omitempty"`

	// getBlobSafe() stats.
	GetBlobSafe struct {
		// Number of calls to getBlobSafe().
		Calls AtomicCounter `json:"calls"`
		// How many times getBlobSafe() had to retry as the blob LMT changed due to simultaneous update.
		Retries AtomicCounter `json:"retries,omitempty"`
		// How many times getBlobSafe() failed after exhausting all retries.
		Failures AtomicCounter `json:"failures,omitempty"`
		// Minimum, maximum, total and average duration of all getBlobSafe() calls.
		MinTime   Duration `json:"min_time,omitempty"`
		MaxTime   Duration `json:"max_time,omitempty"`
		TotalTime Duration `json:"-"`
		AvgTime   Duration `json:"avg_time,omitempty"`
		LastError string   `json:"last_error,omitempty"`
	} `json:"get_blob_safe"`

	// Storage GetBlob call stats.
	StorageGetBlob struct {
		Calls     AtomicCounter `json:"calls"`
		Failures  AtomicCounter `json:"failures,omitempty"`
		MinTime   Duration      `json:"min_time,omitempty"`
		MaxTime   Duration      `json:"max_time,omitempty"`
		TotalTime Duration      `json:"-"`
		AvgTime   Duration      `json:"avg_time,omitempty"`
		LastError string        `json:"last_error,omitempty"`
	} `json:"storage_get_blob"`

	// Storage GetProperties call stats.
	StorageGetProperties struct {
		Calls     AtomicCounter `json:"calls"`
		Failures  AtomicCounter `json:"failures,omitempty"`
		MinTime   Duration      `json:"min_time,omitempty"`
		MaxTime   Duration      `json:"max_time,omitempty"`
		TotalTime Duration      `json:"-"`
		AvgTime   Duration      `json:"avg_time,omitempty"`
		LastError string        `json:"last_error,omitempty"`
	} `json:"storage_get_properties"`

	// Storage PutBlob call stats.
	StoragePutBlob struct {
		Calls     AtomicCounter `json:"calls"`
		Failures  AtomicCounter `json:"failures,omitempty"`
		MinTime   Duration      `json:"min_time,omitempty"`
		MaxTime   Duration      `json:"max_time,omitempty"`
		TotalTime Duration      `json:"-"`
		AvgTime   Duration      `json:"avg_time,omitempty"`
		LastError string        `json:"last_error,omitempty"`
	} `json:"storage_put_blob"`

	// Storage ListDir call stats.
	StorageListDir struct {
		Calls     AtomicCounter `json:"calls"`
		Failures  AtomicCounter `json:"failures,omitempty"`
		MinTime   Duration      `json:"min_time,omitempty"`
		MaxTime   Duration      `json:"max_time,omitempty"`
		TotalTime Duration      `json:"-"`
		AvgTime   Duration      `json:"avg_time,omitempty"`
		LastError string        `json:"last_error,omitempty"`
	} `json:"storage_list_dir"`

	// Heartbeat stats.
	Heartbeat struct {
		Published       AtomicCounter `json:"published"`
		LastPublished   AtomicTime    `json:"last_published,omitzero"`
		PublishFailures AtomicCounter `json:"publish_failures,omitempty"`
		SizeInBytes     AtomicCounter `json:"size_in_bytes"`
		MinGap          Duration      `json:"min_gap,omitempty"`
		MaxGap          Duration      `json:"max_gap,omitempty"`
		TotalGap        Duration      `json:"-"`
		AvgGap          Duration      `json:"avg_gap,omitempty"`
		// How many heartbeats were fetched (only cluster_manager leader fetches heartbeats).
		Fetched       AtomicCounter `json:"fetched,omitempty"`
		FetchFailures AtomicCounter `json:"fetch_failures,omitempty"`
		LastError     string        `json:"last_error,omitempty"`
	} `json:"heartbeat"`

	Clustermap struct {
		// How many times clustermap was updated (only cluster_manager leader updates clustermap).
		UpdateStartCalls AtomicCounter `json:"update_start_calls,omitempty"`
		UpdateEndCalls   AtomicCounter `json:"update_end_calls,omitempty"`
		// Either update start or end failures.
		UpdateFailures  AtomicCounter `json:"update_failures,omitempty"`
		LastUpdateStart AtomicTime    `json:"-"`
		// When was the global clustermap last updated?
		// Only cluster_manager leader updates the global clustermap.
		LastUpdated     AtomicTime `json:"last_updated,omitzero"`
		MinUpdateTime   Duration   `json:"min_update_time,omitempty"`
		MaxUpdateTime   Duration   `json:"max_update_time,omitempty"`
		TotalUpdateTime Duration   `json:"-"`
		AvgUpdateTime   Duration   `json:"avg_update_time,omitempty"`
		// How many time clustermap was fetched.
		GetCalls    AtomicCounter `json:"get_calls,omitempty"`
		GetFailures AtomicCounter `json:"get_failures,omitempty"`
		LastError   string        `json:"last_error,omitempty"`
	} `json:"clustermap"`

	// File creation involves two steps: init, finalize.
	// We maintain stats for each of these steps.
	CreateFile struct {
		// Number of calls to createFileInit().
		InitCalls     AtomicCounter `json:"init_calls,omitempty"`
		InitFailures  AtomicCounter `json:"init_failures,omitempty"`
		LastErrorInit string        `json:"last_error_init,omitempty"`

		// Number of calls to createFileFinalize().
		FinalizeCalls     AtomicCounter `json:"finalize_calls,omitempty"`
		FinalizeFailures  AtomicCounter `json:"finalize_failures,omitempty"`
		LastErrorFinalize string        `json:"last_error_finalize,omitempty"`

		MinSizeBytes   AtomicCounter `json:"min_size_bytes,omitempty"`
		MaxSizeBytes   AtomicCounter `json:"max_size_bytes,omitempty"`
		TotalSizeBytes AtomicCounter `json:"-"`
		AvgSizeBytes   AtomicCounter `json:"avg_size_bytes,omitempty"`
	} `json:"create_file"`

	GetFile struct {
		TotalOpens       AtomicCounter `json:"total_opens,omitempty"`
		SafeDeleteOpens  AtomicCounter `json:"safe_delete_opens,omitempty"`
		SafeDeleteCloses AtomicCounter `json:"safe_delete_closes,omitempty"`
		MaxOpenCount     AtomicCounter `json:"max_open_count,omitempty"`
		Failures         AtomicCounter `json:"failures,omitempty"`
		LastError        string        `json:"last_error,omitempty"`
	} `json:"get_file"`

	DeleteFile struct {
		Deleting  AtomicCounter `json:"deleting,omitempty"`
		Deleted   AtomicCounter `json:"deleted,omitempty"`
		Failures  AtomicCounter `json:"failures,omitempty"`
		LastError string        `json:"last_error,omitempty"`
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
		MVsDeleted AtomicCounter `json:"mvs_deleted,omitempty"`
		// MVs that could not be deleted as deletion failed for one or more chunks or the MV directory.
		MVsDeleteFailed AtomicCounter `json:"mvs_delete_failed,omitempty"`
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

	// Stats gathered from heartbeats processing by the cluster_manager leader node.
	// Heartbeat processing involve gathering node list and then fetching heartbeats for those nodes.
	// Heartbeat processing is done by updateRVList() function.
	Heartbeats struct {
		// Stats for getNodeList() call.
		// This includes the initial heartbeat and all subsequent heartbeats.
		// Only valid for the cluster_manager leader node.
		GetNodeList struct {
			Calls     AtomicCounter `json:"calls,omitempty"`
			Failures  AtomicCounter `json:"failures,omitempty"`
			MinTime   Duration      `json:"min_time,omitempty"`
			MaxTime   Duration      `json:"max_time,omitempty"`
			TotalTime Duration      `json:"-"`
			AvgTime   Duration      `json:"avg_time,omitempty"`
			LastError string        `json:"last_error,omitempty"`
			// When was the last call to getNodeList() made?
			LastCallAt AtomicTime `json:"last_call_at,omitzero"`
			// Total nodes for which a HB file was seen in the Nodes/ folder, last time we enumerated.
			TotalNodes AtomicCounter `json:"total_nodes,omitempty"`
		} `json:"get_node_list"`

		// Stats for collectHBForGivenNodeIds() call.
		// This includes the initial heartbeat and all subsequent heartbeats.
		// Only valid for the cluster_manager leader node.
		CollectHB struct {
			// How many HBs were seen as expired in the last heartbeats processing?
			Expired AtomicCounter `json:"expired,omitempty"`
			// Till now how many heartbeats were seen as expired? This number keeps growing and is indicative
			// of how things have been in the past.
			ExpiredCumulative AtomicCounter `json:"expired_cumulative,omitempty"`
			// How many nodes for which we got the heartbeats? This is for non-initial heartbeats.
			NumNodes AtomicCounter `json:"num_nodes"`
			// How many RVs (from NumNodes)? This is for non-initial heartbeats.
			NumRVs    AtomicCounter `json:"num_rvs"`
			Calls     AtomicCounter `json:"calls,omitempty"`
			Failures  AtomicCounter `json:"failures,omitempty"`
			MinTime   Duration      `json:"min_time,omitempty"`
			MaxTime   Duration      `json:"max_time,omitempty"`
			TotalTime Duration      `json:"-"`
			AvgTime   Duration      `json:"avg_time,omitempty"`
			LastError string        `json:"last_error,omitempty"`
			// When was the last call to collectHBForGivenNodeIds() made?
			LastCallAt AtomicTime `json:"last_call_at,omitzero"`
		} `json:"collect_hb"`

		// Initial heartbeat specific stats.
		// This is valid if this node was a leader and processed initial heartbeats, may not be leader anymore.
		InitialHB struct {
			// Number of nodes for which we got the initial heartbeats.
			// All nodes that start simultaneously will have their initial heartbeats processed
			// together, thus batching/saving time to add their RVs to the clustermap.
			// Higher the better.
			NumNodes AtomicCounter `json:"num_nodes"`
			// How many RVs (from NumNodes)?
			NumRVs AtomicCounter `json:"num_rvs"`
			// RVs which were in the clustermap but we did not see the initial heartbeat.
			StaleRVsRemoved AtomicCounter `json:"stale_rvs_removed,omitempty"`
			// Effective new RVs added to the clustermap.
			NewRVsAdded    AtomicCounter `json:"new_rvs_added,omitempty"`
			DuplicateRVIds AtomicCounter `json:"duplicate_rv_ids,omitempty"`
			LastError      string        `json:"last_error,omitempty"`
			// When was the last (and the only) call to process initial heartbeats made?
			LastCallAt AtomicTime `json:"last_call_at,omitzero"`
		} `json:"initial_hb"`
	} `json:"heartbeats"`

	// Local clustermap stats.
	LocalClustermap struct {
		// Size of the local clustermap in bytes.
		SizeInBytes AtomicCounter `json:"size_in_bytes"`
		// Epoch of the local clustermap.
		Epoch AtomicCounter `json:"epoch"`
		// How many times local clustermap was refreshed (from the global clustermap) on this node?
		TimesUpdated        AtomicCounter `json:"times_updated"`
		LastUpdated         AtomicTime    `json:"last_updated,omitzero"`
		TimeSinceLastUpdate Duration      `json:"time_since_last_update,omitempty"`
		// How many times local clustermap update failed (either failed to fetch or failed to update the
		// local copy)? LastError will have details.
		UpdateFailures AtomicCounter `json:"update_failures,omitempty"`
		// How many time clustermap was refreshed but it was found to be unchanged from our saved local copy?
		Unchanged AtomicCounter `json:"unchanged,omitempty"`
		// Minimum, maximum, total and average duration of all local clustermap updates.
		MinTime   Duration `json:"min_time,omitempty"`
		MaxTime   Duration `json:"max_time,omitempty"`
		TotalTime Duration `json:"-"`
		AvgTime   Duration `json:"avg_time,omitempty"`
		LastError string   `json:"last_error,omitempty"`
	} `json:"local_clustermap"`

	// Global clustermap stats.
	StorageClustermap struct {
		// Node ID of the node that is the leader (as per the latest clustermap copy that we have).
		Leader string `json:"leader,omitempty"`
		// Is this node the current cluster_manager leader?
		// Note that more than one node can claim to be the leader, but only one will be the actual leader.
		// This can happen if some node has a stale clustermap and the leader has changed since then.
		// Wait for clustermap epoch to expire and the stale node will stop claiming leadership.
		IsLeader bool `json:"is_leader"`
		// If this node is/was the cluster_manager leader, when did it become the leader?
		// This will be set even if the node is not the current leader but was leader sometime in the past.
		// This helps to see how leadership has changed over time.
		BecameLeaderAt AtomicTime `json:"became_leader_at,omitzero"`
		// And how long has it been the leader?
		// This will be set only if the node is the current leader.
		LeaderFor Duration `json:"leader_for,omitempty"`
		// How many times the leader has been switched?
		// Note that a leader is the node that updates the storage clustermap. A node can update the clustermap
		// if it sees the clustermap is not updated for a timeout period (this is LeaderSwitchesDueToTimeout),
		// or it could update the clustermap as it wanted to convey some update to the clustermap, maybe some
		// RV state change or adding a new RV.
		// The leader switches are counted *only* by the new leader and not the outgoing leader.
		LeaderSwitches AtomicCounter `json:"leader_switches,omitempty"`
		// How many times a node has to claim leadership as the current leader didn't update the clustermap?
		LeaderSwitchesDueToTimeout AtomicCounter `json:"leader_switches_due_to_timeout,omitempty"`
		LastError                  string        `json:"last_error,omitempty"`
		Calls                      AtomicCounter `json:"calls"`
		Failures                   AtomicCounter `json:"failures,omitempty"`
		// Minimum, maximum, total and average duration of all storage clustermap updates.
		MinTime   Duration `json:"min_time,omitempty"`
		MaxTime   Duration `json:"max_time,omitempty"`
		TotalTime Duration `json:"-"`
		AvgTime   Duration `json:"avg_time,omitempty"`
		// When was the last time the storage clustermap was updated by this node?
		LastUpdatedAt AtomicTime `json:"last_updated_at,omitzero"`
		// Clustermap epoch at the time of last update.
		LastUpdateEpoch AtomicCounter `json:"last_update_epoch,omitempty"`
		//
		// Following stats are reset on leader switch, so they only apply to the current leader stint.
		//
		// How many times the storage clustermap was updated after we became the leader this time?
		TotalUpdates AtomicCounter `json:"total_updates,omitempty"`
		MinGap       Duration      `json:"min_gap,omitempty"`
		MaxGap       Duration      `json:"max_gap,omitempty"`
		TotalGap     Duration      `json:"-"`
		AvgGap       Duration      `json:"avg_gap,omitempty"`
	} `json:"storage_clustermap,omitempty"`

	// updateMVList() stats.
	UpdateMVList struct {
		// How many times updateMVList() was called by the periodic cluster_manager thread to run various MV
		// workflows, such as new-mv, fix-mv, etc. Only leader node run this.
		// Non-leaders call it from batchUpdateComponentRVState(), but we don't count those calls here.
		Calls     AtomicCounter `json:"calls"`
		Failures  AtomicCounter `json:"failures,omitempty"`
		MinTime   Duration      `json:"min_time,omitempty"`
		MaxTime   Duration      `json:"max_time,omitempty"`
		TotalTime Duration      `json:"-"`
		AvgTime   Duration      `json:"avg_time,omitempty"`
		// When was the last call to updateMVList() made?
		LastCallAt AtomicTime `json:"last_call_at,omitzero"`
		LastError  string     `json:"last_error,omitempty"`
	} `json:"update_mv_list"`

	// New MV workflow stats. Also contains stats for RVs and MVs.
	// These are only valid for the cluster_manager leader node, also the leader needs to run updateMVList()
	// once to update these stats, till then these stats will be empty.
	NewMV struct {
		//
		// Following stats are valid only for current cluster_manager leader node.
		//
		MVsPerRV    AtomicCounter `json:"mvs_per_rv,omitempty"`
		NumReplicas AtomicCounter `json:"num_replicas,omitempty"`
		// Total RVs (from all nodes) at our disposal.
		TotalRVs AtomicCounter `json:"total_rvs,omitempty"`
		// How many are offline? Rest are online.
		OfflineRVs AtomicCounter `json:"offline_rvs,omitempty"`
		// Total MVs, created from all the RVs that we have.
		TotalMVs    AtomicCounter `json:"total_mvs,omitempty"`
		OnlineMVs   AtomicCounter `json:"online_mvs,omitempty"`
		DegradedMVs AtomicCounter `json:"degraded_mvs,omitempty"`
		OfflineMVs  AtomicCounter `json:"offline_mvs,omitempty"`
		SyncingMVs  AtomicCounter `json:"syncing_mvs,omitempty"`
		// Available node is one that has at least one RV that can host at least one new MV (remember MVsPerRV?).
		// In steady state this should be less than NumReplicas, else the new-mv workflow must add a new MV.
		AvailableNodes AtomicCounter `json:"available_nodes,omitempty"`

		//
		// Following stats are valid for a node if it was a cluster_manager leader and ran the new-mv workflow.
		//

		// How many MVs were added by this node?
		// Only a cluster_manager leader can add MVs, so this value is meaningful only for the leader node.
		NewMVsAdded AtomicCounter `json:"new_mvs_added,omitempty"`
		// Time when the last MV was added.
		LastMVAddedAt AtomicTime `json:"last_mv_added_at,omitzero"`
		// Total time taken to add new MVs.
		TimeTaken Duration `json:"time_taken,omitempty"`
		// Details of JoinMV calls made for adding component RVs to the new MVs.
		JoinMV struct {
			Calls     AtomicCounter `json:"calls"`
			Failures  AtomicCounter `json:"failures,omitempty"`
			MinTime   Duration      `json:"min_time,omitempty"`
			MaxTime   Duration      `json:"max_time,omitempty"`
			TotalTime Duration      `json:"-"`
			AvgTime   Duration      `json:"avg_time,omitempty"`
			LastError string        `json:"last_error,omitempty"`
		} `json:"join_mv"`
	} `json:"new_mv"`

	// Fix MV workflow stats.
	// Most of the stats have a cumulative version. The non-cumulative stats are for the latest fix-mv workflow
	// while the cumulative stats are the sum of all fix-mv workflows that have run so far.
	// Cumulative stats are useful to see how the fix-mv workflow has been performing over time, which could be
	// indicative of the cluster health.
	FixMV struct {
		// Total calls to fixMV() made from the last updateMVList() call.
		Calls AtomicCounter `json:"calls"`
		// fixMV() replaced *all* offline component RVs of these MVs with new RVs and joinMV() succeeded for all.
		MVsFixed AtomicCounter `json:"mvs_fixed,omitempty"`
		// fixMV() replaced at least one offline component RV of these MVs with new RVs and joinMV() succeeded
		// for those RVs.
		MVsPartiallyFixed AtomicCounter `json:"mvs_partially_fixed,omitempty"`
		// fixMV() could not replace even one offline component RV of these MVs with new RVs.
		MVsNotFixed AtomicCounter `json:"mvs_not_fixed,omitempty"`
		// fixMV() could find some or all replacement RVs but joinMV() failed for these MVs.
		// Note that joinMV() can fail due to JoinMV or UpdateMV failures.
		MVsFixFailedDueToJoinMVOrUpdateMV AtomicCounter `json:"mvs_fix_failed_due_to_join_mv_or_update_mv,omitempty"`
		// How many RVs were replaced.
		RVsReplaced AtomicCounter `json:"rvs_replaced,omitempty"`
		// Count of RVs for which we could not find a replacement RV, when fixMV() is called.
		// Such MVs will remain degraded till the next periodic updateMVList() call..
		NoReplacementRVs AtomicCounter `json:"no_replacement_rvs,omitempty"`

		// Cumulative version of the above stats.
		CallsCumulative                             AtomicCounter `json:"calls_cumulative,omitempty"`
		MVsFixedCumulative                          AtomicCounter `json:"mvs_fixed_cumulative,omitempty"`
		MVsPartiallyFixedCumulative                 AtomicCounter `json:"mvs_partially_fixed_cumulative,omitempty"`
		MVsNotFixedCumulative                       AtomicCounter `json:"mvs_not_fixed_cumulative,omitempty"`
		MVsFixFailedDueToJoinMVOrUpdateMVCumulative AtomicCounter `json:"mvs_fix_failed_due_to_join_mv_or_update_mv_cumulative,omitempty"`
		RVsReplacedCumulative                       AtomicCounter `json:"rvs_replaced_cumulative,omitempty"`
		NoReplacementRVsCumulative                  AtomicCounter `json:"no_replacement_rvs_cumulative,omitempty"`

		// Minimum, maximum, total and average duration of all fixMV() calls.
		MinTime   Duration `json:"min_time,omitempty"`
		MaxTime   Duration `json:"max_time,omitempty"`
		TotalTime Duration `json:"-"`
		AvgTime   Duration `json:"avg_time,omitempty"`
		JoinMV    struct {
			Calls              AtomicCounter `json:"calls"`
			CallsCumulative    AtomicCounter `json:"calls_cumulative,omitempty"`
			Failures           AtomicCounter `json:"failures,omitempty"`
			FailuresCumulative AtomicCounter `json:"failures_cumulative,omitempty"`
			MinTime            Duration      `json:"min_time,omitempty"`
			MaxTime            Duration      `json:"max_time,omitempty"`
			TotalTime          Duration      `json:"-"`
			AvgTime            Duration      `json:"avg_time,omitempty"`
			LastError          string        `json:"last_error,omitempty"`
		} `json:"join_mv"`

		UpdateMV struct {
			Calls              AtomicCounter `json:"calls"`
			CallsCumulative    AtomicCounter `json:"calls_cumulative,omitempty"`
			Failures           AtomicCounter `json:"failures,omitempty"`
			FailuresCumulative AtomicCounter `json:"failures_cumulative,omitempty"`
			MinTime            Duration      `json:"min_time,omitempty"`
			MaxTime            Duration      `json:"max_time,omitempty"`
			TotalTime          Duration      `json:"-"`
			AvgTime            Duration      `json:"avg_time,omitempty"`
			LastError          string        `json:"last_error,omitempty"`
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
	if s.MM.GetBlobSafe.Calls.Value.Load() > 0 {
		s.MM.GetBlobSafe.AvgTime =
			Duration(float64(atomic.LoadInt64((*int64)(&s.MM.GetBlobSafe.TotalTime))) /
				float64(s.MM.GetBlobSafe.Calls.Value.Load()))
	}

	if s.MM.StorageGetBlob.Calls.Value.Load() > 0 {
		s.MM.StorageGetBlob.AvgTime =
			Duration(float64(atomic.LoadInt64((*int64)(&s.MM.StorageGetBlob.TotalTime))) /
				float64(s.MM.StorageGetBlob.Calls.Value.Load()))
	}

	if s.MM.StorageGetProperties.Calls.Value.Load() > 0 {
		s.MM.StorageGetProperties.AvgTime =
			Duration(float64(atomic.LoadInt64((*int64)(&s.MM.StorageGetProperties.TotalTime))) /
				float64(s.MM.StorageGetProperties.Calls.Value.Load()))
	}

	if s.MM.StoragePutBlob.Calls.Value.Load() > 0 {
		s.MM.StoragePutBlob.AvgTime =
			Duration(float64(atomic.LoadInt64((*int64)(&s.MM.StoragePutBlob.TotalTime))) /
				float64(s.MM.StoragePutBlob.Calls.Value.Load()))
	}

	if s.MM.StorageListDir.Calls.Value.Load() > 0 {
		s.MM.StorageListDir.AvgTime =
			Duration(float64(atomic.LoadInt64((*int64)(&s.MM.StorageListDir.TotalTime))) /
				float64(s.MM.StorageListDir.Calls.Value.Load()))
	}

	filesCreatedFromThisNode := s.MM.CreateFile.FinalizeCalls.Value.Load() - s.MM.CreateFile.FinalizeFailures.Value.Load()
	if filesCreatedFromThisNode > 0 {
		s.MM.CreateFile.AvgSizeBytes.Value.Store(s.MM.CreateFile.TotalSizeBytes.Value.Load() / filesCreatedFromThisNode)
	}

	if s.MM.Heartbeat.Published.Value.Load() > 0 {
		s.MM.Heartbeat.AvgGap =
			Duration(float64(atomic.LoadInt64((*int64)(&s.MM.Heartbeat.TotalGap))) /
				float64(s.MM.Heartbeat.Published.Value.Load()))
	}

	if s.MM.Clustermap.UpdateEndCalls.Value.Load() > 0 {
		s.MM.Clustermap.AvgUpdateTime =
			Duration(float64(atomic.LoadInt64((*int64)(&s.MM.Clustermap.TotalUpdateTime))) /
				float64(s.MM.Clustermap.UpdateEndCalls.Value.Load()))
	}

	if s.CM.Heartbeats.GetNodeList.Calls.Value.Load() > 0 {
		s.CM.Heartbeats.GetNodeList.AvgTime =
			Duration(float64(atomic.LoadInt64((*int64)(&s.CM.Heartbeats.GetNodeList.TotalTime))) /
				float64(s.CM.Heartbeats.GetNodeList.Calls.Value.Load()))
	}

	if s.CM.Heartbeats.CollectHB.Calls.Value.Load() > 0 {
		s.CM.Heartbeats.CollectHB.AvgTime =
			Duration(float64(atomic.LoadInt64((*int64)(&s.CM.Heartbeats.CollectHB.TotalTime))) /
				float64(s.CM.Heartbeats.CollectHB.Calls.Value.Load()))
	}

	localClustermapUpdateCalls := s.CM.LocalClustermap.TimesUpdated.Value.Load() -
		s.CM.LocalClustermap.UpdateFailures.Value.Load()
	if localClustermapUpdateCalls > 0 {
		s.CM.LocalClustermap.AvgTime =
			Duration(float64(atomic.LoadInt64((*int64)(&s.CM.LocalClustermap.TotalTime))) /
				float64(localClustermapUpdateCalls))
	}

	if !s.CM.LocalClustermap.LastUpdated.IsZero() {
		s.CM.LocalClustermap.TimeSinceLastUpdate = Duration(time.Since(s.CM.LocalClustermap.LastUpdated.Load()))
	}

	if s.CM.NewMV.JoinMV.Calls.Value.Load() > 0 {
		s.CM.NewMV.JoinMV.AvgTime =
			Duration(float64(atomic.LoadInt64((*int64)(&s.CM.NewMV.JoinMV.TotalTime))) /
				float64(s.CM.NewMV.JoinMV.Calls.Value.Load()))
	}

	if s.NodeId == s.CM.StorageClustermap.Leader {
		// BecameLeaderAt is set when updateClusterMapStart() marks the node as the leader.
		common.Assert(!s.CM.StorageClustermap.BecameLeaderAt.IsZero(), s.NodeId, s.CM.StorageClustermap.Leader)

		s.CM.StorageClustermap.IsLeader = true
		s.CM.StorageClustermap.LeaderFor = Duration(time.Since(s.CM.StorageClustermap.BecameLeaderAt.Load()))
	} else {
		s.CM.StorageClustermap.IsLeader = false
		s.CM.StorageClustermap.LeaderFor = Duration(0)

		//
		// A non-leader may have stale stats which might be misleading, hide them.
		// Note that some stats may be useful even if the node is not the leader currently, but performed
		// some task in the past when it was the leader. We don't want to lose those stats.
		//
		s.CM.Heartbeats.GetNodeList.Calls.Value.Store(0)
		s.CM.Heartbeats.GetNodeList.Failures.Value.Store(0)
		s.CM.Heartbeats.GetNodeList.MinTime = Duration(0)
		s.CM.Heartbeats.GetNodeList.MaxTime = Duration(0)
		s.CM.Heartbeats.GetNodeList.TotalTime = Duration(0)
		s.CM.Heartbeats.GetNodeList.AvgTime = Duration(0)
		s.CM.Heartbeats.GetNodeList.LastError = ""
		s.CM.Heartbeats.GetNodeList.LastCallAt = AtomicTime{}
		s.CM.Heartbeats.GetNodeList.TotalNodes.Value.Store(0)

		s.CM.Heartbeats.CollectHB.Expired.Value.Store(0)
		s.CM.Heartbeats.CollectHB.ExpiredCumulative.Value.Store(0)
		s.CM.Heartbeats.CollectHB.NumNodes.Value.Store(0)
		s.CM.Heartbeats.CollectHB.NumRVs.Value.Store(0)
		s.CM.Heartbeats.CollectHB.Calls.Value.Store(0)
		s.CM.Heartbeats.CollectHB.Failures.Value.Store(0)
		s.CM.Heartbeats.CollectHB.MinTime = Duration(0)
		s.CM.Heartbeats.CollectHB.MaxTime = Duration(0)
		s.CM.Heartbeats.CollectHB.TotalTime = Duration(0)
		s.CM.Heartbeats.CollectHB.AvgTime = Duration(0)
		s.CM.Heartbeats.CollectHB.LastError = ""
		s.CM.Heartbeats.CollectHB.LastCallAt = AtomicTime{}

		s.CM.NewMV.MVsPerRV.Value.Store(0)
		s.CM.NewMV.NumReplicas.Value.Store(0)
		s.CM.NewMV.TotalRVs.Value.Store(0)
		s.CM.NewMV.OfflineRVs.Value.Store(0)
		s.CM.NewMV.TotalMVs.Value.Store(0)
		s.CM.NewMV.OnlineMVs.Value.Store(0)
		s.CM.NewMV.DegradedMVs.Value.Store(0)
		s.CM.NewMV.OfflineMVs.Value.Store(0)
		s.CM.NewMV.SyncingMVs.Value.Store(0)
		s.CM.NewMV.AvailableNodes.Value.Store(0)
	}

	if s.CM.StorageClustermap.TotalUpdates.Value.Load() > 0 {
		s.CM.StorageClustermap.AvgGap =
			Duration(float64(atomic.LoadInt64((*int64)(&s.CM.StorageClustermap.TotalGap))) /
				float64(s.CM.StorageClustermap.TotalUpdates.Value.Load()))
		s.CM.StorageClustermap.AvgTime =
			Duration(float64(atomic.LoadInt64((*int64)(&s.CM.StorageClustermap.TotalTime))) /
				float64(s.CM.StorageClustermap.TotalUpdates.Value.Load()))
	}

	if s.CM.UpdateMVList.Calls.Value.Load() > 0 {
		s.CM.UpdateMVList.AvgTime =
			Duration(float64(atomic.LoadInt64((*int64)(&s.CM.UpdateMVList.TotalTime))) /
				float64(s.CM.UpdateMVList.Calls.Value.Load()))
	}

	if s.CM.FixMV.CallsCumulative.Value.Load() > 0 {
		s.CM.FixMV.AvgTime =
			Duration(float64(atomic.LoadInt64((*int64)(&s.CM.FixMV.TotalTime))) /
				float64(s.CM.FixMV.CallsCumulative.Value.Load()))
	}

	if s.CM.FixMV.JoinMV.CallsCumulative.Value.Load() > 0 {
		s.CM.FixMV.JoinMV.AvgTime =
			Duration(float64(atomic.LoadInt64((*int64)(&s.CM.FixMV.JoinMV.TotalTime))) /
				float64(s.CM.FixMV.JoinMV.CallsCumulative.Value.Load()))
	}

	if s.CM.FixMV.UpdateMV.CallsCumulative.Value.Load() > 0 {
		s.CM.FixMV.UpdateMV.AvgTime =
			Duration(float64(atomic.LoadInt64((*int64)(&s.CM.FixMV.UpdateMV.TotalTime))) /
				float64(s.CM.FixMV.UpdateMV.CallsCumulative.Value.Load()))
	}
}

var Stats *DCacheStats

func init() {
	// Silence unused import errors for release builds.
	common.IsValidUUID("00000000-0000-0000-0000-000000000000")

	Stats = &DCacheStats{
		NodeStart: time.Now(),
	}

	Stats.MM.CreateFile.MinSizeBytes.Value.Store(math.MaxInt64)
}
