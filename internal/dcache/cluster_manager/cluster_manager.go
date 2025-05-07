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

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"

	"github.com/Azure/azure-storage-fuse/v2/internal/dcache"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache/clustermap"
	mm "github.com/Azure/azure-storage-fuse/v2/internal/dcache/metadata_manager"
	rpc_client "github.com/Azure/azure-storage-fuse/v2/internal/dcache/rpc/client"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache/rpc/gen-go/dcache/models"
	rpc_server "github.com/Azure/azure-storage-fuse/v2/internal/dcache/rpc/server"
)

// Cluster manager's job is twofold:
//  1. Keep the global clustermap uptodate. One of the nodes takes up the role of the leader who periodically
//     gather information about all nodes/RVs from the heartbeats and updates the clustermap according to that.
//     It then publishes this clustermap which others download (see point #2 below).
//  2. Maintain a local clustermap copy which is used by clustermap package to respond to various queries
//     by clustermap package users. Every node downloads the global clustermap and save it as this local copy.
//
// This is the singleton cluster manager struct that holds the state of the cluster manager.
type ClusterManager struct {
	myNodeId         string
	myHostName       string
	myIPAddress      string
	config           *dcache.DCacheConfig
	hbTicker         *time.Ticker
	clusterMapticker *time.Ticker
	// Clustermap is refreshed periodically from metadata store and saved in this local path.
	// clustermap package reads this and provides accessor functions for querying specific parts of clustermap.
	localClusterMapPath string
	// ETag of the most recent clustermap saved in localClusterMapPath.
	localMapETag *string
	// RPC server running on this node.
	// It'll respond to RPC queries made from other nodes.
	rpcServer *rpc_server.NodeServer
}

// Error return from here would cause clustermanager startup to fail which will prevent this node from
// joining the cluster.
func (cmi *ClusterManager) start(dCacheConfig *dcache.DCacheConfig, rvs []dcache.RawVolume) error {

	valid, err := IsValidDcacheConfig(dCacheConfig)
	if !valid {
		common.Assert(false, err)
		return fmt.Errorf("ClusterManager::start Not valid cache config: %v", err)
	}

	valid, err = IsValidRVList(rvs, true /* myRVs */)
	if !valid {
		common.Assert(false, err)
		return fmt.Errorf("ClusterManager::start Not valid RV list: %v", err)
	}

	// All RVs exported by a node have the same NodeId and IPAddress, use from the first RV.
	// These will be used as the current node's id and IP address.
	cmi.myNodeId = rvs[0].NodeId
	cmi.myIPAddress = rvs[0].IPAddress

	// Cannot join the cluster w/o a valid nodeid.
	if !common.IsValidUUID(cmi.myNodeId) {
		common.Assert(false, cmi.myNodeId)
		return fmt.Errorf("ClusterManager::start Invalid nodeid: %s", cmi.myNodeId)
	}

	// Cannot join the cluster w/o a valid IP address.
	if !common.IsValidIP(cmi.myIPAddress) {
		common.Assert(false, cmi.myIPAddress)
		return fmt.Errorf("ClusterManager::start Invalid IP address: %s", cmi.myIPAddress)
	}

	// Node name is mainly used for logging, but failing to get hostname indicates some bigger problem,
	// refuse to start.
	cmi.myHostName, err = os.Hostname()
	if err != nil {
		common.Assert(false, err)
		return fmt.Errorf("ClusterManager::start Could not get hostname: %v", err)
	}

	cmi.localClusterMapPath = filepath.Join(common.DefaultWorkDir, "clustermap.json")

	// Note that all nodes in the cluster use consistent config. The node that uploads the initial
	// clustermap gets to choose that config and all other nodes follow that.
	log.Info("ClusterManager::start: myNodeId: %s, myIPAddress: %s, myHostName: %s, HeartbeatSeconds: %d, ClustermapEpoch: %d, LocalClusterMapPath: %s",
		cmi.myNodeId, cmi.myIPAddress, cmi.myHostName, dCacheConfig.HeartbeatSeconds,
		dCacheConfig.ClustermapEpoch, cmi.localClusterMapPath)

	//
	// Ensure initial clustermap is present (either it's already present, if not we set it), and perform
	// safe startup checks.
	//
	log.Info("ClusterManager::start: Ensuring initial cluster map")

	err = cmi.ensureInitialClusterMap(dCacheConfig, rvs)
	if err != nil {
		return err
	}

	log.Info("ClusterManager::start: Initial cluster map now ready")

	//
	// Now we should have a valid local clustermap with all our RVs present in the RV list.
	// TODO: Assert this expected state.
	//
	// Now we can start the RPC server.
	//
	// TODO: Since ensureInitialClusterMap() would send the heartbeat and make the cluster aware of this
	//       node, it's possible that some other cluster node runs the new-mv workflow and sends a JoinMV
	//       RPC request to this node, before we can start the RPC server. We should add resiliency for this.
	//

	log.Info("ClusterManager::start: Starting RPC server")

	common.Assert(cmi.rpcServer == nil)
	cmi.rpcServer, err = rpc_server.NewNodeServer()
	if err != nil {
		log.Err("ClusterManager::start: Failed to create RPC server")
		common.Assert(false, err)
		return err
	}

	err = cmi.rpcServer.Start()
	if err != nil {
		log.Err("ClusterManager::start: Failed to start RPC server")
		common.Assert(false, err)
		return err
	}

	log.Info("ClusterManager::start: Started RPC server on node %s IP %s", cmi.myNodeId, cmi.myIPAddress)

	// We don't intend to have different configs in different nodes, so assert.
	common.Assert(dCacheConfig.HeartbeatSeconds == cmi.config.HeartbeatSeconds,
		"Local config HeartbeatSeconds different from global config",
		dCacheConfig.HeartbeatSeconds, cmi.config.HeartbeatSeconds)

	cmi.hbTicker = time.NewTicker(time.Duration(cmi.config.HeartbeatSeconds) * time.Second)

	const maxConsecutiveFailures = 3

	go func() {
		var err error
		var consecutiveFailures int

		for range cmi.hbTicker.C {
			log.Debug("ClusterManager::start: Scheduled task \"Punch Heartbeat\" triggered")

			err = cmi.punchHeartBeat(rvs)
			if err == nil {
				consecutiveFailures = 0
			} else {
				log.Err("ClusterManager::start: Failed to punch heartbeat: %v", err)
				consecutiveFailures++
				//
				// Failing to update multiple heartbeats signifies some serious issue, take
				// ourselves down to reduce any confusion we may create in the cluster.
				//
				if consecutiveFailures > maxConsecutiveFailures {
					log.GetLoggerObj().Panicf("[PANIC] Failed to update hearbeat %d times in a row",
						consecutiveFailures)
				}
			}
		}
		log.Info("ClusterManager::start: Scheduled task \"Heartbeat Punch\" stopped")
	}()

	// We don't intend to have different configs in different nodes, so assert.
	common.Assert(dCacheConfig.ClustermapEpoch == cmi.config.ClustermapEpoch,
		"Local config ClustermapEpoch different from global config",
		dCacheConfig.ClustermapEpoch, cmi.config.ClustermapEpoch)

	cmi.clusterMapticker = time.NewTicker(time.Duration(cmi.config.ClustermapEpoch) * time.Second)

	go func() {
		var err error
		var consecutiveFailures int

		for range cmi.clusterMapticker.C {
			log.Debug("ClusterManager::start: Scheduled task \"Update ClusterMap\" triggered")
			err = cmi.updateStorageClusterMapIfRequired()
			if err != nil {
				//
				// We don't treat updateStorageClusterMapIfRequired() failure as fatal, since
				// we have mechanism to handle that. Some other node will detect this and step up.
				//
				log.Err("ClusterManager::start: updateStorageClusterMapIfRequired failed: %v", err)
			}

			err = cmi.updateClusterMapLocalCopyIfRequired()
			if err == nil {
				consecutiveFailures = 0
			} else {
				log.Err("ClusterManager::start: updateClusterMapLocalCopyIfRequired failed: %v", err)
				consecutiveFailures++
				//
				// Otoh, failing to update the local cluster copy is a serious issue, since this
				// node cannot use valid clustermap, take ourselves down to reduce any confusion
				// we may create in the cluster.
				//
				if consecutiveFailures > maxConsecutiveFailures {
					log.GetLoggerObj().Panicf("[PANIC] Failed to refresh local clustermap %d times in a row",

						consecutiveFailures)
				}
			}
		}
		log.Info("ClusterManager::start: Scheduled task \"Update ClusterMap\" stopped")
	}()

	return nil
}

// Fetch the global clustermap from metadata store and save a local copy.
// This local copy will be used by the clustermap package to answer various queries on clustermap.
//
// TODO: Add stats for measuring time taken to download the clustermap, how many times it's downloaded, etc.
func (cmi *ClusterManager) updateClusterMapLocalCopyIfRequired() error {
	// 1. Fetch the latest clustermap from metadata store.
	storageBytes, etag, err := getClusterMap()
	if err != nil {
		err = fmt.Errorf("Failed to fetch clustermap on node %s: %v", cmi.myNodeId, err)
		log.Err("ClusterManager::updateClusterMapLocalCopyIfRequired: %v", err)

		common.Assert(len(storageBytes) == 0)
		common.Assert(etag == nil)
		common.Assert(false, err)
		return err
	}

	if len(storageBytes) == 0 {
		err = fmt.Errorf("Received empty clustermap on node %s", cmi.myNodeId)
		log.Err("ClusterManager::updateClusterMapLocalCopyIfRequired: %v", err)
		common.Assert(false, err)
		return err
	}

	// Successful getClusterMap() must return a valid etag.
	common.Assert(etag != nil, fmt.Sprintf("expected non-nil ETag on node %s", cmi.myNodeId))

	log.Debug("ClusterManager::updateClusterMapLocalCopyIfRequired: Fetched global clustermap (bytes: %d, etag: %v)",
		len(storageBytes), *etag)

	// 2. If we've already loaded this exact version, skip the update.
	if etag != nil && cmi.localMapETag != nil && *etag == *cmi.localMapETag {
		log.Debug("ClusterManager::updateClusterMapLocalCopyIfRequired: ETag (%s) unchanged, not updating local clustermap copy",
			*etag)
		return nil
	}

	// 3. Unmarshal the received clustermap.
	var storageClusterMap dcache.ClusterMap
	if err := json.Unmarshal(storageBytes, &storageClusterMap); err != nil {
		err = fmt.Errorf("Failed to unmarshal clustermap json on node %s: %v", cmi.myNodeId, err)
		log.Err("ClusterManager::updateClusterMapLocalCopyIfRequired: %v", err)
		common.Assert(false, err)
		return err
	}

	common.Assert(IsValidClusterMap(&storageClusterMap))

	// 4. Atomically update the local clustermap copy.
	common.Assert(len(cmi.localClusterMapPath) > 0)
	tmp := cmi.localClusterMapPath + ".tmp"
	if err := os.WriteFile(tmp, storageBytes, 0644); err != nil {
		err = fmt.Errorf("WriteFile(%s) failed: %v %+v", tmp, err, storageClusterMap)
		log.Err("ClusterManager::updateClusterMapLocalCopyIfRequired: %v", err)
		common.Assert(false, err)
		return err
	} else if err := os.Rename(tmp, cmi.localClusterMapPath); err != nil {
		err = fmt.Errorf("Rename(%s -> %s) failed: %v %+v",
			tmp, cmi.localClusterMapPath, err, storageClusterMap)
		log.Err("ClusterManager::updateClusterMapLocalCopyIfRequired: %v", err)
		common.Assert(false, err)
		return err
	}

	// 5. Update in-memory tag.
	cmi.localMapETag = etag

	// Once saved, config should not change.
	common.Assert((cmi.config == nil) || (*cmi.config == storageClusterMap.Config),
		fmt.Sprintf("Saved config does not match the one received in clustermap: %+v -> %+v",
			*cmi.config, storageClusterMap.Config))

	cmi.config = &storageClusterMap.Config

	log.Info("ClusterManager::updateClusterMapLocalCopyIfRequired: Local clustermap updated (bytes: %d, etag: %s)",
		len(storageBytes), *etag)

	//TODO{Akku}: Notify only if there is a change in the MVs/RVs
	// 6. Notify clustermap package. It'll refresh its in-memory copy for serving its users.
	clustermap.Update()

	return nil
}

// Stop ClusterManager.
func (cmi *ClusterManager) stop() error {
	log.Info("ClusterManager::stop: stopping tickers and closing channel")
	if cmi.hbTicker != nil {
		cmi.hbTicker.Stop()
	}
	// TODO{Akku}: Delete the heartbeat file
	// mm.DeleteHeartbeat(cmi.myNodeId)
	if cmi.clusterMapticker != nil {
		cmi.clusterMapticker.Stop()
	}
	clustermap.Stop()
	return nil
}

// This is a key function that correctly synchronizes node startup.
// If this fails, clustermanager startup fails and current node won't join the cluster.
// It ensures the following:
// - Initial global clustermap is published if not already present.
// - Initial heartbeat is punched once the node is ready to announce its presence.
// - RVs of this node are added to global clustermap. This is needed for starting the RPC server.
// - Local clustermap copy is saved and updated in the clustermap package.
//
// Note: MVs may or may not be updated in clustermap yet.
func (cmi *ClusterManager) ensureInitialClusterMap(dCacheConfig *dcache.DCacheConfig, rvs []dcache.RawVolume) error {
	//
	// Get the current clustermap before sending out the first heartbeat.
	// If this is the very first node in the cluster, this creates the initial clustermap (with empty
	// RV and MV lists) which will control the global cluster config.
	// If it gets a valid clustermap, it implies that this node is joining an already active cluster.
	// The cluster though can be in any state. Depending on the cluster state as per clustermap, it
	// does the following:
	// 1. If none of the node's RVs are present in the RV list, it implies that this is the first time
	//    the node is joining this cluster. It cannot be contributing any storage to any of the MVs in
	//    the cluster.
	//    The node can safely emit its first heartbeat and continue startup.
	// 2. If one or more of the node's RVs are present in the RV list, it implies that this node was
	//    part of this cluster and is coming back up after restarting.
	//    Since this node was not part of the cluster for some time, it may be missing some data that might
	//    have been written while the node was down.
	//    It's safest to let these RVs be marked offline and removed from any MVs. For that the node may
	//    have to wait for one or more ClustermapEpoch before the current leader clustermanager detects
	//    the node as offline and updates the global clustermap. Post that we can purge our RVs and continue
	//    startup after emitting the first heartbeat. (TODO)
	//
	var currentTime int64
	var clusterMap dcache.ClusterMap
	var clusterMapBytes []byte

	isClusterMapExists, err := cmi.checkIfClusterMapExists()
	if err != nil {
		log.Err("ClusterManager::ensureInitialClusterMap: Failed to check clustermap: %v", err)
		common.Assert(false)
		return err
	}

	if isClusterMapExists {
		//
		// TODO: Need to check if we must purge our RVs, before punching the initial heartbeat.
		//       See comments in ClusterManager::start().
		//
		log.Info("ClusterManager::ensureInitialClusterMap : clustermap already exists")
		goto UpdateLocalClusterMapAndPunchInitialHeartbeat
	}

	//
	// Ok, fresh cluster and we are probably the first node coming up, create the initial clustermap.
	// Note that we can race with some other node(s) and the node that is successful in creating the
	// initial clustermap gets to dicate the cache config. Hopefully the cache config is consistent in
	// all the nodes' config yaml, and this doesn't cause surprises.
	//
	currentTime = time.Now().Unix()
	clusterMap = dcache.ClusterMap{
		Readonly:      true,
		State:         dcache.StateReady,
		Epoch:         1,
		CreatedAt:     currentTime,
		LastUpdatedAt: currentTime,
		LastUpdatedBy: cmi.myNodeId,
		Config:        *dCacheConfig,
		RVMap:         map[string]dcache.RawVolume{},
		MVMap:         map[string]dcache.MirroredVolume{},
	}

	clusterMapBytes, err = json.Marshal(clusterMap)
	if err != nil {
		log.Err("ClusterManager::ensureInitialClusterMap : Failed to marshal initial clustermap: %v %+v",
			err, clusterMap)
		common.Assert(false, err)
		return err
	}

	//
	// CreateInitialClusterMap() will succeed in the following two cases:
	// 1. It was able to upload the above clustermap to the metadata store.
	// 2. The (conditional) upload failed as clustermap was already present.
	//
	// TODO: CreateInitialClusterMap() must convey which of these happened.
	//
	err = mm.CreateInitialClusterMap(clusterMapBytes)
	if err != nil {
		log.Err("ClusterManager::ensureInitialClusterMap : CreateInitialClusterMap failed: %v %+v",
			err, clusterMap)
		common.Assert(false, err)
		return err
	}

	log.Info("ClusterManager::ensureInitialClusterMap: Initial clusterMap created successfully (or there already was a clustermap): %+v",
		clusterMap)

	//
	// Now we have the initial clustermap. Our next task is to update the clustermap with our local RVs and
	// finally save a local copy of the updated clustermap.
	//
UpdateLocalClusterMapAndPunchInitialHeartbeat:

	// Punch the first heartbeat. updateStorageClusterMapWithMyRVs() fetches this heartbeat to get info on local RVs.
	err = cmi.punchHeartBeat(rvs)
	if err != nil {
		log.Err("ClusterManager::ensureInitialClusterMap: Initial punchHeartBeat failed: %v", err)
		common.Assert(false, err)
		return err
	}
	log.Info("ClusterManager::ensureInitialClusterMap: Initial Heartbeat punched")

	//
	// Ensure our RVs are added to the clustermap. If already present in the clustermap, this will be a no-op,
	// else it updates the clustermap with our local RVs added to the RV list.
	///
	err = cmi.updateStorageClusterMapWithMyRVs()
	if err != nil {
		log.Err("ClusterManager::ensureInitialClusterMap: updateStorageClusterMapWithMyRVs failed: %v %+v",
			err, clusterMap)
		common.Assert(false, err)
		return err
	}

	// Save local copy of the clustermap.
	cmi.updateClusterMapLocalCopyIfRequired()

	// TODO: Assert that clustermap has our local RVs.
	common.Assert(cmi.config != nil)
	common.Assert(*cmi.config == *dCacheConfig)

	return nil
}

// Add my RVs to clustermap, if not already added.
// This helps get a unique RV name for each of our RVs, which is needed by the RPC server.
func (cmi *ClusterManager) updateStorageClusterMapWithMyRVs() error {
	startTime := time.Now()
	maxWait := 120 * time.Second

	for {
		// Time check.
		elapsed := time.Since(startTime)
		if elapsed > maxWait {
			common.Assert(false)
			return fmt.Errorf("ClusterManager::updateStorageClusterMapWithMyRVs: Exceeded maxWait")
		}

		clusterMapBytes, etag, err := getClusterMap()
		if err != nil {
			log.Err("ClusterManager::updateStorageClusterMapWithMyRVs: getClusterMap() failed: %v", err)
			common.Assert(false, err)
			return err
		}

		common.Assert(len(clusterMapBytes) > 0)
		common.Assert(etag != nil && len(*etag) > 0)

		log.Debug("ClusterManager::updateStorageClusterMapWithMyRVs: Fetched clusterMap (bytes: %d, etag: %v)",
			len(clusterMapBytes), *etag)

		var clusterMap dcache.ClusterMap
		if err := json.Unmarshal(clusterMapBytes, &clusterMap); err != nil {
			log.Err("ClusterManager::updateStorageClusterMapWithMyRVs: Failed to unmarshal clusterMapBytes: %d, error: %v",
				len(clusterMapBytes), err)
			common.Assert(false, err)
			return err
		}

		// Must be a valid clustermap.
		common.Assert(IsValidClusterMap(&clusterMap))

		// This is the first time we should be saving the global config.
		common.Assert(cmi.config == nil)
		cmi.config = &clusterMap.Config
		common.Assert(IsValidDcacheConfig(cmi.config))

		//
		// Now we want to add our RVs to the clustermap RV list.
		// If some other node is currently updating the clustermap, we need to wait and retry.
		//
		// TODO: Add support for checking if the node that set the state to StateChecking dies
		//       and hence it doesn't come out of that state.
		//
		if clusterMap.State == dcache.StateChecking {
			log.Info("ClusterManager::updateStorageClusterMapWithMyRVs: clustermap being updated by node %s, waiting a bit before retry",
				clusterMap.LastUpdatedBy)
			// We cannot be updating.
			common.Assert(clusterMap.LastUpdatedBy != cmi.myNodeId)
			// TODO: Add some backoff and randomness?
			time.Sleep(10 * time.Millisecond)
			continue
		}
		common.Assert(clusterMap.State == dcache.StateReady)

		clusterMap.LastUpdatedBy = cmi.myNodeId
		clusterMap.State = dcache.StateChecking
		updatedClusterMapBytes, err := json.Marshal(clusterMap)
		if err != nil {
			log.Err("ClusterManager::updateStorageClusterMapWithMyRVs: Marshal failed for clustermap: %v %+v",
				err, clusterMap)
			common.Assert(false, err)
			return err
		}

		//
		// Claim ownership of clustermap and add our RVs.
		// If some other node gets there before us, we retry. Note that we don't add a wait before
		// the retry as that other node is not updating the clustermap, it's done updating.
		//
		// TODO: Check err to see if the failure is due to etag mismatch, if not retrying may not help.
		//
		if err = mm.UpdateClusterMapStart(updatedClusterMapBytes, etag); err != nil {
			log.Warn("ClusterManager::updateStorageClusterMapWithMyRVs: Start Clustermap update failed for nodeId %s: %v, retrying",
				cmi.myNodeId, err)
			continue
		}

		log.Info("ClusterManager::updateStorageClusterMapWithMyRVs: Updating RV list")

		_, err = cmi.updateRVList(clusterMap.RVMap, true /* onlyMyRVs */)
		if err != nil {
			log.Err("ClusterManager::updateStorageClusterMapWithMyRVs: updateRVList() failed: %v", err)
			common.Assert(false, err)
			return err
		}

		clusterMap.LastUpdatedAt = time.Now().Unix()
		// TODO: Set StateReady iff MinNodes nodes are online.
		clusterMap.State = dcache.StateReady
		updatedClusterMapBytes, err = json.Marshal(clusterMap)
		if err != nil {
			log.Err("ClusterManager::updateStorageClusterMapWithMyRVs: Marshal failed for clustermap: %v %+v",
				err, clusterMap)
			common.Assert(false, err)
			return err
		}

		//TODO{Akku}: Make sure end update is happening with the same node as of start update
		if err = mm.UpdateClusterMapEnd(updatedClusterMapBytes); err != nil {
			log.Err("ClusterManager::updateStorageClusterMapWithMyRVs: UpdateClusterMapEnd() failed: %v %+v",
				err, clusterMap)
			common.Assert(false, err)
			return err
		}

		// The clustermap must now have our RVs added to RV list.
		log.Info("ClusterManager::updateStorageClusterMapWithMyRVs: cluster map updated by %s at %d %+v",
			cmi.myNodeId, clusterMap.LastUpdatedAt, clusterMap)

		break
	}

	return nil
}

func (cmi *ClusterManager) checkIfClusterMapExists() (bool, error) {
	_, _, err := getClusterMap()
	if err != nil {
		if os.IsNotExist(err) || err == syscall.ENOENT {
			return false, nil
		} else {
			return false, err
		}
	}

	return true, nil
}

var getClusterMap = func() ([]byte, *string, error) {
	return mm.GetClusterMap()
}

var getHeartbeat = func(nodeId string) ([]byte, error) {
	return mm.GetHeartbeat(nodeId)
}

var getAllNodes = func() ([]string, error) {
	return mm.GetAllNodes()
}

func evaluateReadOnlyState() bool {
	return false
}

func (cmi *ClusterManager) punchHeartBeat(myRVs []dcache.RawVolume) error {
	// Refresh AvailableSpace for my RVs, before publishing in the heartbeat.
	refreshMyRVs(myRVs)

	hbData := dcache.HeartbeatData{
		IPAddr:        cmi.myIPAddress,
		NodeID:        cmi.myNodeId,
		Hostname:      cmi.myHostName,
		LastHeartbeat: uint64(time.Now().Unix()),
		RVList:        myRVs,
	}

	// Marshal the data into JSON
	data, err := json.Marshal(hbData)
	if err != nil {
		err = fmt.Errorf("Failed to marshal heartbeat for node %s: %v %+v", cmi.myNodeId, err, hbData)
		log.Err("ClusterManager::punchHeartBeat: %v", err)
		common.Assert(false, err)
		return err
	}

	// Create/update heartbeat file in metadata store with name <nodeId>.hb
	err = mm.UpdateHeartbeat(cmi.myNodeId, data)
	if err != nil {
		err = fmt.Errorf("UpdateHeartbeat() failed for node %s: %v %+v", cmi.myNodeId, err, hbData)
		log.Err("ClusterManager::punchHeartBeat: %v", err)
		common.Assert(false, err)
		return err
	}

	log.Debug("ClusterManager::punchHeartBeat: heartbeat updated for node %+v", hbData)
	return nil
}

// This is no doubt the most important task done by clustermanager.
// It queries all the heartbeats present and updates clustermap's RV list and MV list accordingly.
func (cmi *ClusterManager) updateStorageClusterMapIfRequired() error {
	clusterMapBytes, etag, err := getClusterMap()
	if err != nil {
		log.Err("ClusterManager::updateStorageClusterMapIfRequired: getClusterMap() failed: %v", err)
		common.Assert(false, err)
		return err
	}

	if len(clusterMapBytes) == 0 {
		err = fmt.Errorf("Received empty clustermap on node %s", cmi.myNodeId)
		log.Err("ClusterManager::updateStorageClusterMapIfRequired: %v", err)
		common.Assert(false, err)
		return err
	}

	// Successful getClusterMap() must return a valid etag.
	common.Assert(etag != nil, fmt.Sprintf("expected non-nil ETag on node %s", cmi.myNodeId))

	log.Debug("ClusterManager::updateStorageClusterMapIfRequired: Fetched global clustermap (bytes: %d, etag: %v)",
		len(clusterMapBytes), *etag)

	var clusterMap dcache.ClusterMap
	if err := json.Unmarshal(clusterMapBytes, &clusterMap); err != nil {
		err = fmt.Errorf("Failed to unmarshal clusterMapBytes (%d): %v", len(clusterMapBytes), err)
		log.Err("ClusterManager::updateStorageClusterMapIfRequired: %v", err)
		common.Assert(false, err)
		return err
	}

	common.Assert(IsValidClusterMap(&clusterMap))

	//
	// The node that updated the clusterMap last is preferred over others, for updating the clusterMap.
	// This helps to avoid multiple nodes unnecessarily trying to update the clusterMap (only one of them will
	// succeed but we don't want to waste the effort put by all nodes). But, we have to be wary of the fact that
	// the leader node may go offline, in which case we would want some other node to step up and take the role of
	// the leader. We use the following simple strategy:
	// - Every ClustermapEpoch when the ticker fires, the leader node is automatically eligible for updating the
	//   clusterMap, it need not perform the staleness check.
	// - Every non-leader node has to perform a staleness check which defines a stale clusterMap as one that was
	//   updated more than ClustermapEpoch+thresholdEpochTime seconds in the past. thresholdEpochTime is chosen to
	//   be 60 secs to prevent minor clock skews from causing a non-leader to wrongly consider the clusterMap stale
	//   and race with the leader for updating the clusterMap. Only when the leader is down, on the next tick, one
	//   of the nodes that runs this code first will correctly find the clusterMap stale and it'd then take up the
	//   job of updating the clusterMap and becoming the new leader if it's able to successfully update the
	//   clusterMap.
	//
	// With these rules, the leader is the one that updates the clusterMap in every tick (ClustermapEpoch), while in
	// case of leader node going down, some other node will update the clusterMap in the next tick. In such case
	// the clusterMap will be updated after two consecutive ClustermapEpoch.
	//

	now := time.Now().Unix()
	if clusterMap.LastUpdatedAt > now {
		err = fmt.Errorf("LastUpdatedAt(%d) in future, now(%d), skipping update", clusterMap.LastUpdatedAt, now)
		log.Warn("ClusterManager::updateStorageClusterMapIfRequired: %v", err)

		// Be soft if it could be do to clock skew.
		if (clusterMap.LastUpdatedAt - now) < 300 {
			return nil
		}

		// Else, let the caller know.
		common.Assert(false, "cluster.LastUpdatedAt is too much in future")
		return err
	}

	clusterMapAge := now - clusterMap.LastUpdatedAt
	// Assert if clusterMap is not updated for 3 consecutive epochs.
	// That might indicate some bug.
	common.Assert(clusterMapAge < int64(clusterMap.Config.ClustermapEpoch*3),
		fmt.Sprintf("clusterMapAge (%d) >= %d", clusterMapAge, clusterMap.Config.ClustermapEpoch*3))

	const thresholdEpochTime = 60
	// Staleness check for non-leader.
	stale := clusterMapAge > int64(clusterMap.Config.ClustermapEpoch+thresholdEpochTime)
	// Are we the leader node? Leader gets to update the clustermap bypassing the staleness check.
	leader := (clusterMap.LastUpdatedBy == cmi.myNodeId)

	// stale for checking state can be different than the stale for ready state
	// TODO{Akku}: update stale calculation for checking state
	// Skip if clustermap already in checking state
	if clusterMap.State == dcache.StateChecking && !stale {
		log.Debug("ClusterManager::updateStorageClusterMapIfRequired: skipping, clustermap is being updated by (leader %s), current node (%s)", clusterMap.LastUpdatedBy, cmi.myNodeId)

		// Leader node should have updated the state to checking and it should not find the state to checking.
		common.Assert(!leader, "We don't expect leader to see the clustermap in checking state")
		return nil
	}

	// Skip if we're neither leader nor the clustermap is stale
	if !leader && !stale {
		log.Info("ClusterManager::updateStorageClusterMapIfRequired: skipping, node (%s) is not leader (leader is %s) and clusterMap is fresh (last updated at epoch %d, now %d).",
			cmi.myNodeId, clusterMap.LastUpdatedBy, clusterMap.LastUpdatedAt, now)
		return nil
	}

	// TODO: We need to update clusterMap.Epoch to contain the next higher number.

	clusterMap.LastUpdatedBy = cmi.myNodeId
	clusterMap.State = dcache.StateChecking
	updatedClusterMapBytes, err := json.Marshal(clusterMap)
	if err != nil {
		err = fmt.Errorf("Marshal failed for clustermap: %v %+v", err, clusterMap)
		log.Err("ClusterManager::updateStorageClusterMapIfRequired: %v", err)
		common.Assert(false, err)
		return err
	}

	//
	// Start the clustermap update process by first claiming ownership of the clustermap update.
	// Only one node will succeed in UpdateClusterMapStart(), and that node proceeds with the clustermap
	// update.
	//
	// Note: We still have the Assert() here as it's highly unlikely and it helps to catch any other bug.
	//
	if err = mm.UpdateClusterMapStart(updatedClusterMapBytes, etag); err != nil {
		err = fmt.Errorf("Start Clustermap update failed for nodeId %s: %v", cmi.myNodeId, err)
		log.Err("ClusterManager::updateStorageClusterMapIfRequired: %v", err)
		common.Assert(false, err)
		return err
	}

	log.Info("ClusterManager::updateStorageClusterMapIfRequired: UpdateClusterMapStart succeeded for nodeId %s",
		cmi.myNodeId)

	log.Debug("ClusterManager::updateStorageClusterMapIfRequired: updating RV list")

	changed, err := cmi.updateRVList(clusterMap.RVMap, false /* onlyMyRVs */)
	if err != nil {
		err = fmt.Errorf("Failed to reconcile RV mapping: %v", err)
		log.Err("ClusterManager::updateStorageClusterMapIfRequired: %v", err)
		common.Assert(false, err)
		return err
	}

	//
	// If one or more RVs changed state or new RV(s) were added, MV list will need to be recomputed.
	//
	// TODO: If no RV changes state, RV list and MV list won't be updated. In such case we need not update
	//       the clustermap, but note that not updating clustermap may be regarded by non-leader nodes as
	//       "leader down" and they will step up to update the clustermap. To avoid this, we should add
	//       another field LastProcessedAt apart from LastUpdatedAt. LastUpdatedAt will only be updated
	//       when the clustermap is actually updated and LastProcessedAt can be used for leader down
	//       handling.
	//
	if changed {
		cmi.updateMVList(clusterMap.RVMap, clusterMap.MVMap)
	} else {
		log.Debug("ClusterManager::updateStorageClusterMapIfRequired: No changes in RV mapping")
	}

	clusterMap.LastUpdatedAt = time.Now().Unix()
	clusterMap.State = dcache.StateReady
	updatedClusterMapBytes, err = json.Marshal(clusterMap)
	if err != nil {
		err = fmt.Errorf("Marshal failed for clustermap: %v %+v", err, clusterMap)
		log.Err("ClusterManager::updateStorageClusterMapIfRequired: %v", err)
		common.Assert(false, err)
		return err
	}

	//TODO{Akku}: Make sure end update is happening with the same node as of start update
	if err = mm.UpdateClusterMapEnd(updatedClusterMapBytes); err != nil {
		err = fmt.Errorf("UpdateClusterMapEnd() failed: %v %+v", err, clusterMap)
		log.Err("ClusterManager::updateStorageClusterMapIfRequired: %v", err)
		common.Assert(false, err)
		return err
	}

	log.Info("ClusterManager::updateStorageClusterMapIfRequired: cluster map updated by %s at %d: %+v",
		cmi.myNodeId, now, clusterMap)
	return nil
}

// Takes rvMap which is a set of all known RVs (existing RV list from clustermap, and updated as per the most recent
// heartbeats), it is indexed by RV name and contains complete info about the RV, and existingMVMap which is the set
// of MVs present in the clustermap, indexed by MV name and contains complete info about the MV.
// It runs the following workflows:
//  1. degrade-mv: It goes over all the MVs in existingMVMap to see if any (but not all) of their component RVs which
//     was previously online has gone offline. It marks those MVs as degraded and the component RV as
//     offline.
//  2. offline-mv: This is similar to degrade-mv but if *all* (NumReplicas) component RVs have gone offline, the MV is
//     marked offline.
//  3. fix-mv:     For all the degraded MVs it replaces all the offline component RVs with good RVs, and sets the
//     state for those RVs as outofsync. These MVs will be later picked by Replication Manager to run
//     the resync-mv workflow.
//  4. new-mv:     This adds new MVs to the MV list, made from unused RVs. The component RVs are added in such a way
//     that more than one component RVs for an MV do not come from the same node and the same fault domain.
//
// existingMVMap is updated in-place, the caller will then publish it in the updated clustermap.
func (cmi *ClusterManager) updateMVList(rvMap map[string]dcache.RawVolume, existingMVMap map[string]dcache.MirroredVolume) {
	// We should not be called for an empty rvMap.
	common.Assert(len(rvMap) > 0)

	//
	// Approach:
	//
	// We make a list of nodes each having a list of RVs hosted by that node. This is
	// typically one RV per node, but it can be higher.
	// Each RV starts with a slot count equal to MvsPerRv. This is done so that we can
	// assign one RV to MvsPerRv MVs.
	// In Phase#1 we go over existing MVs and deduct slot count for all the RVs used
	// by the existing MVs. After that's done, we are left with RVs with updated slot
	// count signifying how many more MVs they can host.
	// In this phase we also check if any of the RVs used in existing MVs are offline
	// and mark the MVs as degraded. This is the degrade-mv workflow.
	// If all the RVs in a MV are offline, we mark the MV as offline. This is the offline-mv
	// workflow.
	// Now in Phase#2 we create as many new MVs as we can, continuing with the next
	// available MV name, each MV is assigned one RV from a different node, upto
	// NumReplicas for each MV.
	// This continues till we do not have enough RVs (from distinct nodes) for creating
	// a new MV.
	//

	log.Debug("ClusterManager::updateMVList: Updating current MV list according to the updated RV list.")

	//
	// Represents an RV.
	// An RV has a name and slots to indicate how many times the RV has been used up in various MVs.
	// One MV can use an RV at most once. slots is initialized with MvsPerRv and then decremented by
	// one everytime an RV is found as component RV to an MV.
	//
	type rv struct {
		rvName string
		slots  int
	}

	//
	// Represents a node.
	// A node has a nodeid and list of RVs.
	//
	type node struct {
		nodeId string
		rvs    []rv
	}

	NumReplicas := int(cmi.config.NumReplicas)
	MvsPerRv := int(cmi.config.MvsPerRv)

	common.Assert(NumReplicas >= int(minNumReplicas) && NumReplicas <= int(maxNumReplicas), NumReplicas)
	common.Assert(MvsPerRv >= int(minMvsPerRv) && MvsPerRv <= int(maxMvsPerRv), MvsPerRv)
	common.Assert(IsValidRVMap(rvMap))
	common.Assert(IsValidMvMap(existingMVMap, NumReplicas))

	//
	// Populate the node map (indexed by nodeid) with each node representing all its RVs.
	// This is the nodeToRvs.
	//
	nodeToRvs := make(map[string]node)
	for rvName, rvInfo := range rvMap {
		common.Assert(IsValidRV(&rvInfo))

		if rvInfo.State == dcache.StateOffline {
			// Skip RVs that are offline as they cannot contribute to any MV.
			continue
		}

		if nodeInfo, exists := nodeToRvs[rvInfo.NodeId]; exists {
			// If the node already exists, append the RV to its list.
			// This will be the case when node has more than one RV and we are encountering the second
			// or subsequent RVs.
			common.Assert(rvInfo.NodeId == nodeInfo.nodeId)
			common.Assert(len(nodeInfo.rvs) > 0)
			common.Assert(nodeInfo.rvs[0].slots == MvsPerRv)
			common.Assert(nodeInfo.rvs[0].rvName != rvName)

			nodeInfo.rvs = append(nodeInfo.rvs, rv{
				rvName: rvName,
				slots:  MvsPerRv,
			})
			nodeToRvs[rvInfo.NodeId] = nodeInfo
		} else {
			// Encountered first RV of this node. Create a new node and add the RV to it.
			nodeToRvs[rvInfo.NodeId] = node{
				nodeId: rvInfo.NodeId,
				rvs:    []rv{{rvName: rvName, slots: MvsPerRv}},
			}
		}
	}

	// Cannot have more nodes than RVs.
	common.Assert(len(nodeToRvs) <= len(rvMap), nodeToRvs, rvMap)

	//
	// Phase 1:
	//
	// Go over all MVs in existingMVMap and all component RVs for each of them and do the following:
	// - Reduce slots from the corresponding RV in nodeToRvs map.
	// - If any (not not all) component RV of an MV is offline, mark the MV as degraded and the
	//   component RV as offline. This is the degrade-mv workflow.
	// - If all component RVs of an MV are offline, mark the MV as offline.
	//   This is the offline-mv workflow.
	//
	for mvName, mv := range existingMVMap {
		offlineRv := 0
		for rvName := range mv.RVs {
			// Only valid RVs can be used as component RVs for an MV.
			_, exists := rvMap[rvName]
			common.Assert(exists)

			if rvMap[rvName].State == dcache.StateOffline {
				offlineRv++
				mv.RVs[rvName] = dcache.StateOffline
				if offlineRv == len(mv.RVs) {
					// offline-mv.
					mv.State = dcache.StateOffline
				} else {
					// degrade-mv.
					mv.State = dcache.StateDegraded
				}
				existingMVMap[mvName] = mv
				continue
			}

			//
			// This component RV is online. Reduce its slot count, so that we don't use a component RV
			// more than MvsPerRv times across different MVs.
			// Note that offline RVs are not considered as component RVs so we don't bother updating
			// their slot count.
			//
			nodeId := rvMap[rvName].NodeId
			found := false
			for i := range len(nodeToRvs[nodeId].rvs) {
				if nodeToRvs[nodeId].rvs[i].rvName == rvName {
					common.Assert(nodeToRvs[nodeId].rvs[i].slots > 0)
					nodeToRvs[nodeId].rvs[i].slots--
					found = true
					break
				}
			}
			common.Assert(found, fmt.Sprintf("Component RV %s for MV %s not found in node %s",
				rvName, mvName, nodeId))
		}
	}

	log.Debug("ClusterManager::updateMVList: existingMVMap after phase#1: %v", existingMVMap)

	//
	// Phase 2:
	//
	// Here we run the new-mv workflow, where we add as many new MVs as we can with the available RVs, picking
	// NumReplicas RVs for each new MV under the following conditions:
	// - An RV can be used as component RV by at most MvsPerRv MVs.
	// - More than one RV from the same node will not be used as component RVs for the same MV.
	// - More than one RV from the same fault domain will not be used as component RVs for the same MV.
	//
	for {
		//
		// Check if any node has exhausted all its RV's, remove such nodes from the nodeToRvs map.
		//
		for nodeId, node := range nodeToRvs {
			for j := 0; j < len(node.rvs); {
				// Remove a RV if its slots value is 0.
				if node.rvs[j].slots == 0 {
					node.rvs = append(node.rvs[:j], node.rvs[j+1:]...)
				} else {
					j++
				}
			}

			// If the node has no RVs left, remove it from the map.
			if len(node.rvs) == 0 {
				delete(nodeToRvs, nodeId)
			} else {
				nodeToRvs[nodeId] = node
			}
		}

		//
		// The nodeToRvs map is now updated with remaining nodes and their RVs.
		// For this iteration of new-mv workflow, we have only those nodes left which can contribute
		// at least one RV and only those RVs left which have at least one slot to contribute.
		//

		// New MV will need at least NumReplicas distinct nodes.
		if len(nodeToRvs) < NumReplicas {
			break
		}

		// With rvMap and MvsPerRv and NumReplicas, we cannot have more than maxMVsPossible MVs.
		maxMVsPossible := (len(rvMap) * MvsPerRv) / NumReplicas
		common.Assert(len(existingMVMap) <= maxMVsPossible, len(existingMVMap), maxMVsPossible)

		if len(existingMVMap) == maxMVsPossible {
			break
		}

		// Shuffle the nodes to encourage random selection of component RVs.
		var availableNodes []node
		for _, n := range nodeToRvs {
			availableNodes = append(availableNodes, n)

		}

		rand.Shuffle(len(availableNodes), func(i, j int) {
			availableNodes[i], availableNodes[j] = availableNodes[j], availableNodes[i]
		})

		// New MV's name, starting from index 0.
		mvName := fmt.Sprintf("mv%d", len(existingMVMap))

		//
		// Take the first NumReplicas nodes.
		// Since only those nodes are present in nodeToRvs/availableNodes which have at least one RV
		// slot available, we are guaranted to get NumReplicas component RVs from selectedNodes.
		// Simply go over the selectedNodes and pick the first available RV from each selected node.
		//
		selectedNodes := availableNodes[:NumReplicas]
		common.Assert(len(selectedNodes) == NumReplicas)

		for _, n := range selectedNodes {
			for _, r := range n.rvs {
				//
				// At the beginning of this iteration we sanitized nodeToRvs map to contain
				// only those nodes (and only those RVs) which can contribute at least one RV,
				// so there shouldn't be an RV with slot count 0.
				//
				common.Assert(r.slots > 0, fmt.Sprintf("RV %s has no slots left", r.rvName))

				if _, exists := existingMVMap[mvName]; !exists {
					// First component RV being added to mvName.
					rvwithstate := make(map[string]dcache.StateEnum)
					rvwithstate[r.rvName] = dcache.StateOnline
					// Create a new MV.
					existingMVMap[mvName] = dcache.MirroredVolume{
						RVs:   rvwithstate,
						State: dcache.StateOnline,
					}
				} else {
					// Subsequent component RVs being added to mvName.
					existingMVMap[mvName].RVs[r.rvName] = dcache.StateOnline
					common.Assert(len(existingMVMap[mvName].RVs) <= NumReplicas)
				}

				found := false
				// Decrease the slot count for the RV in nodeToRvs
				for i := range nodeToRvs[n.nodeId].rvs {
					if nodeToRvs[n.nodeId].rvs[i].rvName == r.rvName {
						common.Assert(nodeToRvs[n.nodeId].rvs[i].slots > 0)
						nodeToRvs[n.nodeId].rvs[i].slots--
						found = true
						break
					}
				}

				common.Assert(found, fmt.Sprintf("Component RV %s for MV %s not found in node %s",
					r.rvName, mvName, n.nodeId))
				break
			}
		}

		// Call join mv and check if all rv's are able to join the mv with rv list.
		mv := existingMVMap[mvName]
		deleteRv, err := cmi.joinMV(mvName, &mv)
		if err != nil {
			// TODO :: Should try reallocating the RVs to the MV a certain number of times
			// before giving up and deleting the MV.
			log.Err("ClusterManagerImpl::updateMVList: Error joining MV %s with RV %s: %v", mvName, deleteRv, err)

			for rvName := range existingMVMap[mvName].RVs {
				found := false
				for i, node := range nodeToRvs[rvMap[rvName].NodeId].rvs {
					if node.rvName == deleteRv {
						// Delete rv from the list of RVs for the node.
						rvList := nodeToRvs[rvMap[rvName].NodeId]
						rvList.rvs = append(rvList.rvs[:i], rvList.rvs[i+1:]...)
						nodeToRvs[rvMap[rvName].NodeId] = rvList
						found = true
						break
					} else if node.rvName == rvName {
						node.slots++
						found = true
						break
					}
				}
				common.Assert(found, fmt.Sprintf("Component RV %s for MV %s not found in node %s", rvName, mvName, rvMap[rvName].NodeId))
			}

			// Delete the MV from the existingMVMap.
			delete(existingMVMap, mvName)
		} else {
			log.Info("ClusterManagerImpl::updateMVList: Successfully joined all componentRV's to MV %s", mvName)
		}
	}

	log.Debug("ClusterManager::updateMVList: existing MV map after phase#2: %v", existingMVMap)
	// TODO :: arrange the map entries in lexicographical order.
}

func (cmi *ClusterManager) joinMV(mvName string, mv *dcache.MirroredVolume) (string, error) {
	log.Debug("ClusterManagerImpl::joinMV: Joining MV %s with rv list %+v", mvName, mv.RVs)

	var componentRVs []*models.RVNameAndState

	for rvName, rvState := range mv.RVs {
		log.Debug("ClusterManagerImpl::joinMV: Populating componentRVs list MV %s with RV %s", mvName, rvName)
		// TODO :: Add check for StateOutOfSync.
		common.Assert(rvState == dcache.StateOnline, fmt.Sprintf("ClusterManagerImpl::joinMV: Populating componentRVs list RV %s is %v, skipping list", rvName, mv.State))
		componentRVs = append(componentRVs, &models.RVNameAndState{
			Name:  rvName,
			State: string(rvState),
		})
	}

	// TODO :: Call joinMV on all RVs in parallel.
	for _, rv := range componentRVs {
		log.Debug("ClusterManagerImpl::joiningMV: Joining MV %s with RV %s", mvName, rv.Name)

		joinMvReq := &models.JoinMVRequest{
			MV:     mvName,
			RVName: rv.Name,
			// TODO :: When fix mv workflow is implemented, we need to set the
			// ReserveSpace to the amount of space that is needed to be
			// allocated to the RV.
			ReserveSpace: 0,
			ComponentRV:  componentRVs,
		}

		timeout := 2 * time.Second
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		_, err := rpc_client.JoinMV(ctx, clustermap.RVNameToNodeId(rv.Name), joinMvReq)
		common.Assert(err == nil, fmt.Sprintf("Error joining MV %s with RV %s: %v", mvName, rv.Name, err))

		if err != nil {
			log.Err("ClusterManagerImpl::joinMV: Error joining MV %s with RV %s: %v", mvName, rv.Name, err)
			return rv.Name, err
		}
	}
	return "", nil
}

// Given the list of existing RVs in clusterMap, add any new RVs available.
// If onlyMyRVs is true then the only RV(s) added/updated are the ones exported by the current node, else it queries
// the heartbeats from all nodes and adds all new RVs available and updates all RVs.
// existingRVMap is updated in-place.
func (cmi *ClusterManager) updateRVList(existingRVMap map[string]dcache.RawVolume, onlyMyRVs bool) (bool, error) {
	hbTillNodeDown := int64(cmi.config.HeartbeatsTillNodeDown)
	hbSeconds := int64(cmi.config.HeartbeatSeconds)
	nodeIds := []string{cmi.myNodeId}
	var err error

	//
	// If onlyMyRVs is false then we need to query heartbeats for all the nodes, else just this node.
	//
	if !onlyMyRVs {
		nodeIds, err = getAllNodes()
		if err != nil {
			common.Assert(false, err)
			return false, fmt.Errorf("ClusterManager::updateRVList: getAllNodes() failed: %v", err)
		}
		log.Debug("ClusterManager::updateRVList: Found %d nodes in cluster: %v", len(nodeIds), nodeIds)
	}

	// Both these maps are indexed by RV id.
	rVsByRvIdFromHB := make(map[string]dcache.RawVolume)
	rvLastHB := make(map[string]uint64)
	// Set to true if we add any new RV to existingRVMap or update the state of any existing RV.
	changed := false

	for _, nodeId := range nodeIds {
		log.Debug("ClusterManager::updateRVList: Fetching heartbeat for node %s", nodeId)

		bytes, err := getHeartbeat(nodeId)
		if err != nil {
			common.Assert(false, err)
			return false, fmt.Errorf("ClusterManager::updateRVList: Failed to fetch heartbeat for node %s: %v",
				nodeId, err)
		}

		var hbData dcache.HeartbeatData
		if err := json.Unmarshal(bytes, &hbData); err != nil {
			common.Assert(false, err)
			return false, fmt.Errorf("ClusterManager::updateRVList: Failed to parse heartbeat bytes (%d) for node %s: %v",
				len(bytes), nodeId, err)
		}

		// Go over all RVs exported by this node, and add them to rVsByRvIdFromHB, to be processed later.
		for _, rv := range hbData.RVList {
			if _, exists := rVsByRvIdFromHB[rv.RvId]; exists {
				msg := fmt.Sprintf("Duplicate RVId %s in heartbeat for node %s (also from node %s)",
					rv.RvId, nodeId, rVsByRvIdFromHB[rv.RvId].NodeId)
				common.Assert(false, msg)
				log.Err("ClusterManager::updateRVList: %s, skipping!", msg)
				continue
			}

			common.Assert(hbData.NodeID == rv.NodeId, "RV and HB do not agree",
				hbData.NodeID, rv.NodeId)
			common.Assert(hbData.IPAddr == rv.IPAddress, "RV and HB do not agree",
				hbData.IPAddr, rv.IPAddress)

			common.Assert(IsValidRV(&rv))

			rVsByRvIdFromHB[rv.RvId] = rv
			rvLastHB[rv.RvId] = hbData.LastHeartbeat
		}
	}

	//
	// Ok, now we have all the RVs from heartbeats in rVsByRvIdFromHB[].
	// We need to do two things:
	// 1. Check all RVs in existingRVMap and see if any of them have changed (either State or AvailableSpace).
	//    If so, update those in existingRVMap.
	// 2. All RVs in rVsByRvIdFromHB[] which are not present in existingRVMap, i.e., those RVs are newly seen,
	//    add those to existingRVMap.
	//

	// If an RV has the LastHeartbeat less than hbExpiry, it needs to be offlined.
	now := uint64(time.Now().Unix())
	hbExpiry := now - uint64(hbTillNodeDown*hbSeconds)

	// (1) Update RVs present in existingRVMap and which have changed State or AvailableSpace.
	for rvName, rvInClusterMap := range existingRVMap {

		if rvHb, found := rVsByRvIdFromHB[rvInClusterMap.RvId]; found {
			lastHB := rvLastHB[rvHb.RvId]

			if lastHB < hbExpiry {
				//
				// HB expired, mark RV offline if not already offline.
				//
				if rvInClusterMap.State != dcache.StateOffline {
					log.Warn("ClusterManager::updateRVList: Online RV %s lastHeartbeat (%d) is expired, hbExpiry (%d), marking RV offline",
						rvName, lastHB, hbExpiry)
					rvInClusterMap.State = dcache.StateOffline
					existingRVMap[rvName] = rvInClusterMap
					changed = true
				}
			} else {
				//
				// HB not expired.
				// If either the State or AvailableSpace from HB is different from what is stored
				// in existingRVMap, update it.
				//
				if (rvInClusterMap.State != rvHb.State) ||
					(rvInClusterMap.AvailableSpace != rvHb.AvailableSpace) {
					rvInClusterMap.State = rvHb.State
					rvInClusterMap.AvailableSpace = rvHb.AvailableSpace
					//TODO{Akku}: IF available space is less than 10% of total space, we might need to update the state
					existingRVMap[rvName] = rvInClusterMap
					changed = true
				}
			}

			// rVsByRvIdFromHB must only contain new RVs, delete this as it is in existingRVMap.
			delete(rVsByRvIdFromHB, rvHb.RvId)
		} else {
			//
			// RV present in existingRVMap, but missing from rVsByRvIdFromHB.
			// This is not a common occurrence, emit a warning log.
			//
			if rvInClusterMap.State != dcache.StateOffline {
				log.Warn("ClusterManager::updateRVList: Online Rv %s missing in new heartbeats", rvName)
				rvInClusterMap.State = dcache.StateOffline
				existingRVMap[rvName] = rvInClusterMap
				changed = true
			}
		}
	}

	//
	// (2) Add any new RVs.
	// Now rVsByRvIdFromHB must only have RVs which are not already in existingRVMap.
	//
	if len(rVsByRvIdFromHB) != 0 {
		log.Info("ClusterManager::updateRVList: %d new RV(s) to add to clusterMap: %v",
			len(rVsByRvIdFromHB), rVsByRvIdFromHB)

		// Find max index RV.
		maxIdx := -1
		for name := range existingRVMap {
			if i, err := strconv.Atoi(strings.TrimPrefix(name, "rv")); err == nil && i > maxIdx {
				maxIdx = i
			}
		}

		// Add new RV(s) into clusterMap, starting from the next available index.
		idx := maxIdx + 1
		for _, rv := range rVsByRvIdFromHB {
			rvName := fmt.Sprintf("rv%d", idx)
			existingRVMap[rvName] = rv
			idx++
			changed = true
			log.Info("ClusterManager::updateRVList: Adding new RV %s to cluster map: %+v", rvName, rv)
		}
	}

	return changed, nil
}

// Refresh AvailableSpace in my RVs.
func refreshMyRVs(myRVs []dcache.RawVolume) {
	for index, rv := range myRVs {
		_, availableSpace, err := common.GetDiskSpaceMetricsFromStatfs(rv.LocalCachePath)
		common.Assert(err == nil, fmt.Sprintf("Error getting disk space metrics for path %s for punching heartbeat: %v", rv.LocalCachePath, err))
		if err != nil {
			availableSpace = 0

			log.Warn("ClusterManager::refreshMyRVs: Error getting disk space metrics for path %s for punching heartbeat, forcing available space to zero: %v", rv.LocalCachePath, err)
		}
		myRVs[index].AvailableSpace = availableSpace
		myRVs[index].State = dcache.StateOnline
	}
}

var (
	// clusterManager is the singleton instance of the ClusterManager
	clusterManager *ClusterManager = nil
)

// This must be called from DistributedCache component's Start() method.
// dCacheConfig is the config as read from the config yaml.
// rvs is the list of raw volumes (local_cache_path) as specified in the config. There must be at least one raw volume.
func Start(dCacheConfig *dcache.DCacheConfig, rvs []dcache.RawVolume) error {
	common.Assert(clusterManager == nil, "ClusterManager Init must be called only once")
	common.Assert(len(rvs) > 0)

	clusterManager = &ClusterManager{}
	return clusterManager.start(dCacheConfig, rvs)
}

func Stop() error {
	common.Assert(clusterManager != nil, "ClusterManager not started")
	return clusterManager.stop()
}
