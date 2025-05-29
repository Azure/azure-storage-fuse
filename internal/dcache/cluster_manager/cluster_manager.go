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
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"

	"github.com/Azure/azure-storage-fuse/v2/internal/dcache"
	cm "github.com/Azure/azure-storage-fuse/v2/internal/dcache/clustermap"
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
	// Mutex for synchronizing updates to localClusterMapPath, localMapETag and the cached copy in clustermap package.
	localMapLock sync.Mutex
	// RPC server running on this node.
	// It'll respond to RPC queries made from other nodes.
	rpcServer *rpc_server.NodeServer
}

// Error return from here would cause clustermanager startup to fail which will prevent this node from
// joining the cluster.
// The caller has checked the validity(their existence and correct rw permissions) of the localCachePath directories corresponding to the RVs.
func (cmi *ClusterManager) start(dCacheConfig *dcache.DCacheConfig, rvs []dcache.RawVolume) error {

	valid, err := cm.IsValidDcacheConfig(dCacheConfig)
	if !valid {
		common.Assert(false, err)
		return fmt.Errorf("ClusterManager::start Not valid cache config: %v", err)
	}

	valid, err = cm.IsValidRVList(rvs, true /* myRVs */)
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

	//
	// clustermap MUST now have the in-core clustermap copy.
	// We call the cm.GetCacheConfig() below to validate that.
	//
	log.Info("ClusterManager::start: Initial cluster map now ready with config %+v",
		*cm.GetCacheConfig())

	//
	// Now we should have a valid local clustermap with all our RVs present in the RV list.
	// TODO: Assert this expected state.
	//
	// Now we can start the RPC server.
	//
	// TODO: Since ensureInitialClusterMap() would send the heartbeat and make the cluster aware of this
	//       node, it's possible that some other cluster node runs the new-mv workflow and sends a JoinMV
	//       RPC request to this node, before we can start the RPC server. We should add resiliency for this
	//       by trying JoinMV RPC a few times.
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

		//
		// TODO: Test it and make sure it doesn't call updateStorageClusterMapIfRequired() back2back
		//       in case the prev call to updateStorageClusterMapIfRequired() took long time, causing
		//       ticks to accumulate. There's no point in calling updateStorageClusterMapIfRequired()
		//       b2b. The doc says that ticker will adjust and drop ticks for slow receivers, but we
		//       need to verify and if required, drop ticks which are long back in the past.
		//
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

			_, _, err = cmi.fetchAndUpdateLocalClusterMap()
			if err == nil {
				consecutiveFailures = 0
			} else {
				log.Err("ClusterManager::start: fetchAndUpdateLocalClusterMap failed: %v",
					err)
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

// Fetch the global clustermap from metadata store, save a local copy and let clustermap package know about
// the update so that it can then refresh its in-memory copy used for responding to the queries on clustermap.
//
// If it's able to successfully fetch the global clustermap, it returns a pointer to the unmarshalled ClusterMap
// and the Blob etag corresponding to that.
//
// Note: Use this instead of directly calling getClusterMap() as it ensures that once it returns we can safely
//
//	call various clustermap functions and they will return information as per the latest downloaded clustermap.
//	This is important, f.e., updateMVList() may be working on the latest clustermap copy and in the process
//	it may call some clustermap methods hoping to query the latest clustermap, if it calls getClusterMap() to
//	fetch the latest clustermap, and then calls the clustermap methods, those will be querying not the latest
//	clustermap downloaded by the last getClusterMap() call but the one that's currently updated with clustermap
//	package.
//
// TODO: Add stats for measuring time taken to download the clustermap, how many times it's downloaded, etc.
func (cmi *ClusterManager) fetchAndUpdateLocalClusterMap() (*dcache.ClusterMap, *string, error) {
	//
	// 1. Fetch the latest clustermap from metadata store.
	//
	storageBytes, etag, err := getClusterMap()
	if err != nil {
		err = fmt.Errorf("Failed to fetch clustermap on node %s: %v", cmi.myNodeId, err)
		log.Err("ClusterManager::fetchAndUpdateLocalClusterMap: %v", err)

		common.Assert(len(storageBytes) == 0)
		common.Assert(etag == nil)
		//
		// Only when called from safeCleanupMyRVs(), we may not have the global clustermap yet.
		// Post that, once cmi.config is set, it should never fail.
		//
		common.Assert(cmi.config == nil, err)
		return nil, nil, err
	}

	if len(storageBytes) == 0 {
		err = fmt.Errorf("Received empty clustermap on node %s", cmi.myNodeId)
		log.Err("ClusterManager::fetchAndUpdateLocalClusterMap: %v", err)
		common.Assert(false, err)
		return nil, nil, err
	}

	// Successful getClusterMap() must return a valid etag.
	common.Assert(etag != nil, cmi.myNodeId, len(storageBytes))

	log.Debug("ClusterManager::fetchAndUpdateLocalClusterMap: Fetched global clustermap (bytes: %d, etag: %v)",
		len(storageBytes), *etag)

	//
	// 2. Unmarshal the received clustermap.
	//
	var storageClusterMap dcache.ClusterMap
	if err := json.Unmarshal(storageBytes, &storageClusterMap); err != nil {
		err = fmt.Errorf("Failed to unmarshal clustermap json on node %s: %v", cmi.myNodeId, err)
		log.Err("ClusterManager::fetchAndUpdateLocalClusterMap: %v", err)
		common.Assert(false, err)
		return nil, nil, err
	}

	common.Assert(cm.IsValidClusterMap(&storageClusterMap))

	cmi.localMapLock.Lock()
	defer cmi.localMapLock.Unlock()

	//
	// 3. If we've already loaded this exact version, skip the local update.
	//
	if etag != nil && cmi.localMapETag != nil && *etag == *cmi.localMapETag {
		log.Debug("ClusterManager::fetchAndUpdateLocalClusterMap: ETag (%s) unchanged, not updating local clustermap",
			*etag)
		// Cache config must have been saved when we saved the clustermap.
		common.Assert(cmi.config != nil)
		common.Assert(cm.IsValidDcacheConfig(cmi.config))
		return &storageClusterMap, etag, nil
	}

	//
	// 4. Atomically update the local clustermap copy, along with corresponding localMapETag.
	//
	common.Assert(len(cmi.localClusterMapPath) > 0)
	tmp := cmi.localClusterMapPath + ".tmp"
	if err := os.WriteFile(tmp, storageBytes, 0644); err != nil {
		err = fmt.Errorf("WriteFile(%s) failed: %v %+v", tmp, err, storageClusterMap)
		log.Err("ClusterManager::fetchAndUpdateLocalClusterMap: %v", err)
		common.Assert(false, err)
		return nil, nil, err
	} else if err := os.Rename(tmp, cmi.localClusterMapPath); err != nil {
		err = fmt.Errorf("Rename(%s -> %s) failed: %v %+v",
			tmp, cmi.localClusterMapPath, err, storageClusterMap)
		log.Err("ClusterManager::fetchAndUpdateLocalClusterMap: %v", err)
		common.Assert(false, err)
		return nil, nil, err
	}

	//
	// 5. Update in-memory tag.
	//
	cmi.localMapETag = etag

	// Once saved, config should not change.
	if cmi.config != nil {
		common.Assert(*cmi.config == storageClusterMap.Config,
			fmt.Sprintf("Saved config does not match the one received in clustermap: %+v -> %+v",
				*cmi.config, storageClusterMap.Config))
	} else {
		cmi.config = &storageClusterMap.Config
		common.Assert(cm.IsValidDcacheConfig(cmi.config))
	}

	log.Info("ClusterManager::fetchAndUpdateLocalClusterMap: Local clustermap updated (bytes: %d, etag: %s)",
		len(storageBytes), *etag)

	//
	// 6. Notify clustermap package. It'll refresh its in-memory copy for serving its users.
	//
	cm.Update()

	return &storageClusterMap, etag, nil
}

func (cmi *ClusterManager) updateClusterMapLocalCopy() error {
	_, _, err := cmi.fetchAndUpdateLocalClusterMap()
	return err
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
	cm.Stop()
	return nil
}

// Cleanup one RV directory.
// It returns success only if it's able to delete all the MV directories found in the RV, else if it's not able
// to delete even one MV dir it'll return an error to prevent this node from joining the cluster.
//
// TODO: Once we have sufficient runin we can let it join the cluster even on partial cleanup.
func cleanupRV(rv dcache.RawVolume) error {
	var wg sync.WaitGroup
	var deleteSuccess atomic.Int64
	var deleteFailures atomic.Int64

	// More than a few parallel deletes may be counter productive.
	const maxParallelDeletes = 32
	var tokens = make(chan struct{}, maxParallelDeletes)

	//
	// TODO: Replace this with chunked readdir to support huge number of MVs
	//
	entries, err := os.ReadDir(rv.LocalCachePath)
	if err != nil {
		common.Assert(false, err)
		return fmt.Errorf("ClusterManager::cleanupRV os.ReadDir(%s) failed: %v", rv.LocalCachePath, err)
	}

	//
	// Cache dir must contain only MV directories (containing chunks), go over each and delete them recursively.
	//
	for _, entry := range entries {
		log.Debug("ClusterManager::cleanupRV Got %s/%s", rv.LocalCachePath, entry.Name())

		if !entry.IsDir() {
			common.Assert(false, rv.LocalCachePath, entry.Name())
			return fmt.Errorf("ClusterManager::cleanupRV %s/%s is not a directory %+v",
				rv.LocalCachePath, entry.Name(), entry)
		}

		//
		// TODO: Once we remove .sync directory support, then we can remove the .sync related code.
		//
		mvName := entry.Name()
		parts := strings.Split(mvName, ".")
		if len(parts) > 1 {
			mvName = parts[0]
			if len(parts) > 2 || parts[1] != "sync" {
				common.Assert(false, rv.LocalCachePath, entry.Name())
				return fmt.Errorf("ClusterManager::cleanupRV %s/%s is not a valid MV directory %+v",
					rv.LocalCachePath, entry.Name(), entry)
			}
		}

		if !cm.IsValidMVName(mvName) {
			common.Assert(false, rv.LocalCachePath, entry.Name())
			return fmt.Errorf("ClusterManager::cleanupRV %s/%s is not a valid MV directory %+v",
				rv.LocalCachePath, entry.Name(), entry)
		}

		//
		// Delete MV folders in parallel, but not more than maxParallelDeletes.
		//
		wg.Add(1)
		go func(dir string) {
			tokens <- struct{}{}
			defer func() {
				<-tokens
				wg.Done()
			}()
			log.Info("ClusterManager::cleanupRV: Async remove: %s", dir)

			//
			// Delete all files in MV directory, and the MV directory itself.
			// TODO: Make sure this works ok for very large number of chunks (2+ million)
			//
			err := os.RemoveAll(dir)
			if err != nil {
				log.Err("ClusterManager::cleanupRV: os.RemoveAll (%s) failed: %v", dir, err)
				deleteFailures.Add(1)
			} else {
				log.Info("ClusterManager::cleanupRV: Deleted MV dir %s/%s", rv.LocalCachePath, dir)
				deleteSuccess.Add(1)
			}
		}(filepath.Join(rv.LocalCachePath, entry.Name()))
	}

	// Wait for all running deletes to finish.
	wg.Wait()

	//
	// Fail the call if at least one MV dir delete failed.
	//
	if deleteFailures.Load() != 0 {
		common.Assert(false, rv.LocalCachePath, deleteFailures.Load(), deleteSuccess.Load())
		return fmt.Errorf("ClusterManager::cleanupRV: RV dir %s, failed to delete %d MV(s) (deleted %d)",
			rv.LocalCachePath, deleteFailures.Load(), deleteSuccess.Load())
	}

	log.Info("ClusterManager::cleanupRV: Successfully cleaned up RV dir %s, deleted %d MV(s)",
		rv.LocalCachePath, deleteSuccess.Load())

	return nil
}

// Cleanup all my local RVs, deleting any mv folders (and the stored chunks if any) created if/when the RV was part
// of the cluster in the past. Note that this cleans up the local RV directory only after making sure it's safe to
// clean, and an RV is safe to clean when it's either not present in the clusterMap RV list or it's marked as offline.
// An RV that's offline in the RV list is guaranteed to be offline in the MV list also, i.e., no MV will contact
// this RV for for chunk IO (read or write).
//
// In case of success the boolean return value indicates the following:
// true  -> Found a clustermap and RV(s) were either not present in the RV list or waited for RV(s) to be marked
//
//	offline and then cleaned up the RVs if any.
//
// false -> Did not find clustermap, cleaned up the RVs if any.
//
// In case of success it'll return only after all RV directories are fully cleaned up.
// If it finds any unexpected file/dir in the RV it complains and bails out. Note that this is the only place where
// we check if RV contains any unexpected file/dir.
func (cmi *ClusterManager) safeCleanupMyRVs(myRVs []dcache.RawVolume) (bool, error) {
	log.Info("ClusterManager::safeCleanupMyRVs: Cleaning up %d RV(s) %v", len(myRVs), myRVs)

	//
	// maxWait is the amount of time we will wait for RVs to be marked offline in clustermap.
	// Once we have the config it's set to minimum of 5 minutes and thrice the clustermap epoch to make sure
	// the clusterManager gets a chance to notice loss of heartbeat and mark the RV offline.
	//
	startTime := time.Now()
	maxWait := 300 * time.Second
	var wg sync.WaitGroup
	var failedRV atomic.Int64

	//
	// Helper function for cleaning up all my RV, once we know they are not being used by the cluster.
	//
	cleanupAllMyOfflineRVs := func() error {
		for _, rv := range myRVs {
			//
			// Each RV is most likely a separate filesystem/device, so we can delete them all in parallel.
			//
			wg.Add(1)
			go func(rv dcache.RawVolume) {
				defer wg.Done()

				err := cleanupRV(rv)
				if err != nil {
					log.Err("ClusterManager::safeCleanupMyRVs: cleanupRV (%s) failed: %v",
						rv.LocalCachePath, err)
					failedRV.Add(1)
				}
			}(rv)
		}

		wg.Wait()

		if failedRV.Load() != 0 {
			return fmt.Errorf("ClusterManager::safeCleanupMyRVs: Failed to cleanup %d RV(s)",
				failedRV.Load())
		}

		// Successfully cleaned up all RVs.
		log.Info("ClusterManager::safeCleanupMyRVs: Successfully cleaned up %d RV(s) %v", len(myRVs), myRVs)

		return nil
	}

	for {
		elapsed := time.Since(startTime)
		if elapsed > maxWait {
			//
			// We have waited enough (3 x clustermap epochs).
			// Most likely it's the case of reviving a dead cluster with a clustermap that has not been updated
			// for a long time, but play safe and bail out and let the user delete the clustermap by hand before
			// retrying.
			//
			err := fmt.Errorf("ClusterManager::safeCleanupMyRVs: Exceeded maxWait %s. If you are reviving a dead cluster, delete clustermap manually and then try again.", maxWait)
			log.Err("%v", err)
			common.Assert(false, elapsed, maxWait)
			return true, err
		}

		//
		// Fetch clustermap and update the local copy.
		// Once this succeeds, clustermap APIs can be used for querying clustermap.
		//
		_, _, err := cmi.fetchAndUpdateLocalClusterMap()
		if err != nil {
			isClusterMapExists, err1 := cmi.checkIfClusterMapExists()
			if err1 != nil {
				//
				// Fail to fetch clustermap, not safe to proceed.
				//
				common.Assert(false, err1, err)
				return false, fmt.Errorf("ClusterManager::safeCleanupMyRVs: Failed to fetch clustermap: %v, %v",
					err1, err)
			}

			//
			// This implies some other error in fetchAndUpdateLocalClusterMap(), maybe clustermap
			// unmarshal failed, or some other error. In any case we cannot query clustermap and hence not
			// safe to proceed.
			//
			if isClusterMapExists {
				common.Assert(false, err)
				return false, fmt.Errorf("ClusterManager::safeCleanupMyRVs: Failed to query clustermap: %v", err)
			}

			//
			// clustermap is not present, we can safely cleanup all our RVs
			//
			common.Assert(failedRV.Load() == 0, failedRV.Load())
			return false, cleanupAllMyOfflineRVs()
		}

		//
		// Ok, clustermap is present, we need to wait for our RVs to be marked offline in the RV list,
		// before we can safely clean them up. To be safe we wait for 3 times the clustermap epoch.
		// This is suffcient to be safe even in the event of clusterManager leader going down.
		// For very small clustermap epoch, we wait for 5 mins minimum.
		//
		maxWait = max(maxWait, time.Duration(cm.GetCacheConfig().ClustermapEpoch*3)*time.Second)

		//
		// Check status of all our RVs in the clustermap.
		//
		myRVsFromClustermap := cm.GetMyRVs()

		if len(myRVsFromClustermap) > 0 {
			log.Info("ClusterManager::safeCleanupMyRVs: Got %d of my RV(s) from clustermap %+v",
				len(myRVsFromClustermap), myRVsFromClustermap)
		} else {
			log.Info("ClusterManager::safeCleanupMyRVs: No my RV(s) in clustermap")
		}

		rvStillOnline := false
		for _, rv := range myRVs {
			log.Info("ClusterManager::safeCleanupMyRVs: Checking my RV %+v", rv)

			// Check online status for this RV.
			for rvName, rvInfo := range myRVsFromClustermap {
				log.Info("ClusterManager::safeCleanupMyRVs: My RV %s has id %s in clustermap",
					rvName, rvInfo.RvId)

				if rv.RvId != rvInfo.RvId {
					continue
				}

				if rvInfo.State != dcache.StateOffline {
					log.Info("ClusterManager::safeCleanupMyRVs: My RV %+v still online, retry in 30sec", rv)
					rvStillOnline = true
				}

				break
			}

			if rvStillOnline {
				break
			}

			// Offline RV (or RV not present in clustermap), clean up.
			wg.Add(1)
			go func(rv dcache.RawVolume) {
				defer wg.Done()

				err := cleanupRV(rv)
				if err != nil {
					log.Err("ClusterManager::safeCleanupMyRVs: cleanupRV (%s) failed: %v",
						rv.LocalCachePath, err)
					failedRV.Add(1)
				}
			}(rv)
		}

		// None of my RV online, done.
		if !rvStillOnline {
			break
		}

		time.Sleep(30 * time.Second)
	}

	// Wait for all RVs to complete cleanup.
	wg.Wait()

	if failedRV.Load() != 0 {
		return false, fmt.Errorf("ClusterManager::safeCleanupMyRVs: Failed to cleanup %d RV(s)",
			failedRV.Load())
	}

	log.Info("ClusterManager::safeCleanupMyRVs: Successfully cleaned up %d RV(s) %v", len(myRVs), myRVs)
	return true, nil
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
	// 2. If one or more of this node's RVs are present in the RV list, it implies that this node was
	//    part of this cluster and is coming back up after restarting.
	//    Since this node was not part of the cluster for some time, it may be missing some data that might
	//    have been written while the node was down.
	//    It's safest to let these RVs be marked offline and removed from any MVs. For that the node may
	//    have to wait for one or more ClustermapEpoch before the current leader clustermanager detects
	//    the node as offline and updates the global clustermap. Post that we can purge our RVs and continue
	//    startup after emitting the first heartbeat.
	//
	var currentTime int64
	var clusterMap dcache.ClusterMap
	var clusterMapBytes []byte

	//
	// safeCleanupMyRVs() cleans up all of my RVs, after performing the safe checks described above.
	// Once it returns we are guaranteed that it's safe to join the cluster.
	//
	// TODO: We need to run this same workflow when an RV goes offline not due to VM/blobfuse restarting
	//       but because of n/w connectivity. Later when it comes back up online, the RV has to go through
	//       the same join-cluster workflow.
	//       Basically any RV that is marked offline in clustermap cannot be simply marked online.
	//       It must go through the proper re-induction workflow where it must wait for it to be removed
	//       from all MVs, clean up the RV directory and then add back.
	//
	isClusterMapExists, err := cmi.safeCleanupMyRVs(rvs)
	if err != nil {
		log.Err("ClusterManager::ensureInitialClusterMap: Failed to check clustermap: %v", err)
		common.Assert(false)
		return err
	}

	if isClusterMapExists {
		//
		// TODO: Need to check if we must purge all of my RVs, before punching the initial heartbeat.
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
	// safeCleanupMyRVs() would have waited for any of our RVs present in clustermap to be marked offline,
	// so it's safe to announce our presence by punching the first heartbeat.
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

	//
	// Save local copy of the clustermap.
	//
	_, _, err = cmi.fetchAndUpdateLocalClusterMap()
	if err != nil {
		log.Err("ClusterManager::ensureInitialClusterMap: fetchAndUpdateLocalClusterMap() failed: %v",
			err)
		common.Assert(false, err)
		return err
	}

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

		clusterMap, etag, err := cmi.fetchAndUpdateLocalClusterMap()
		if err != nil {
			log.Err("ClusterManager::updateStorageClusterMapWithMyRVs: fetchAndUpdateLocalClusterMap() failed: %v",
				err)
			common.Assert(false, err)
			return err
		}

		//
		// Now we want to add our RVs to the clustermap RV list.
		//
		// If some other node/context is currently updating clustermap, we need to wait and retry.
		//
		isClusterMapUpdateBlocked, err := cmi.clusterMapBeingUpdatedByAnotherNode(clusterMap, etag)
		if err != nil {
			common.Assert(false, err)
			return err
		}

		if isClusterMapUpdateBlocked {
			log.Info("ClusterManager::updateStorageClusterMapWithMyRVs: clustermap being updated by node %s, waiting a bit before retry",
				clusterMap.LastUpdatedBy)
			// We cannot be updating.
			common.Assert(clusterMap.LastUpdatedBy != cmi.myNodeId)

			// TODO: Add some backoff and randomness?
			time.Sleep(1 * time.Second)
			continue
		}

		common.Assert(clusterMap.State == dcache.StateReady)

		//
		// Claim ownership of clustermap and add our RVs.
		// If some other node gets there before us, we retry. Note that we don't add a wait before
		// the retry as that other node is not updating the clustermap, it's done updating.
		//
		// Note: We retry even if the failure is not due to etag mismatch, hoping the error to be transient.
		//       Anyways, we have a timeout.
		//
		err = cmi.startClusterMapUpdate(clusterMap, etag)
		if err != nil {
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

		err = cmi.endClusterMapUpdate(clusterMap)
		if err != nil {
			log.Err("ClusterManager::updateStorageClusterMapWithMyRVs: endClusterMapUpdate() failed: %v %+v",
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

// These many seconds must expire on top of a ClustermapEpoch, before we consider the leader node as "dead"
// and try to claim ownership of clusterMap update.
const thresholdClusterMapEpochTime = 60

// This method checks if the cluster map is currently locked in an update (StateChecking) by another node and handles
// stale or stuck updates.
//
// How it works:
//   - If the cluster map is not in StateChecking, no update is happening → returns false.
//   - If this node (myNodeId) is the one updating it (possibly from another thread) → returns true, do nothing or
//     retry after sometime.
//   - If another node started the update but the update is still within the allowed time threshold → returns true,
//     do nothing or retry after sometime.
//   - If another node started the update but it has exceeded the allowed time threshold → the update is considered
//     stuck. In the stale/stuck case, this method:
//     1. Logs a warning about the stale ownership.
//     2. Cleanly ends the previous update using UpdateClusterMapStart and UpdateClusterMapEnd with the latest ETag.
//     3. Resets the cluster map state to StateReady, so this node can safely take over updates.
//
// Returns:
// - (false, nil): no active update or stale/stuck update and we successfully claimed ownership, safe to proceed.
// - (true, nil): an update is still ongoing (by this or another node).
// - (true, error): something failed while recovering from a stale update.
func (cmi *ClusterManager) clusterMapBeingUpdatedByAnotherNode(clusterMap *dcache.ClusterMap, etag *string) (bool, error) {
	if clusterMap.State != dcache.StateChecking {
		// If the cluster map is not in StateChecking, it means it's not being updated by another node.
		log.Debug("ClusterManager::clusterMapBeingUpdatedByAnotherNode: ClusterMap is not in StateChecking state")
		return false, nil
	}

	// Seconds elapsed since last clustermap update.
	age := time.Since(time.Unix(clusterMap.LastUpdatedAt, 0))
	// Age older than this indicates stale/stuck clustermap update, likely a node started update but died.
	staleThreshold := time.Duration(int(cmi.config.ClustermapEpoch)+thresholdClusterMapEpochTime) * time.Second

	// Check if ownership is taken by this node, likely another thread.
	if clusterMap.LastUpdatedBy == cmi.myNodeId {
		log.Debug("ClusterManager::clusterMapBeingUpdatedByAnotherNode: ClusterMap being updated by this node, possibly another thread, started %s ago", age)
		common.Assert(age < staleThreshold, age, staleThreshold)
		return true, nil
	}

	//  Check if the last updated timestamp exceeds a configured threshold, indicating stale/stuck update.
	if age < staleThreshold {
		log.Debug("ClusterManager::clusterMapBeingUpdatedByAnotherNode: ClusterMap being updated by another node (%s), started %s ago", clusterMap.LastUpdatedBy, age)
		return true, nil
	}

	log.Warn("ClusterManager::clusterMapBeingUpdatedByAnotherNode: Clustermap update stuck in StateChecking, by node %s at %d (%s ago), exceeding stale threshold %s, overriding ownership by node %s",
		clusterMap.LastUpdatedBy, clusterMap.LastUpdatedAt, age, staleThreshold, cmi.myNodeId)

	err := cmi.startClusterMapUpdate(clusterMap, etag)
	if err != nil {
		// If the failure is due to etag mismatch, it falls under the "another node is updating" category.
		if mm.IsErrConditionNotMet(err) {
			return true, nil
		}

		// Any other error is unexpected, assert to know if it happens.
		common.Assert(false, err)
		return true, fmt.Errorf("ClusterManager::clusterMapBeingUpdatedByAnotherNode: startClusterMapUpdate() failed: %v", err)
	}

	err = cmi.endClusterMapUpdate(clusterMap)
	if err != nil {
		common.Assert(false, err)
		return true, fmt.Errorf("ClusterManager::clusterMapBeingUpdatedByAnotherNode: endClusterMapUpdate() failed: %v", err)
	}

	// This is our promise to the caller.
	common.Assert(clusterMap.State == dcache.StateReady)
	common.Assert(clusterMap.LastUpdatedBy == cmi.myNodeId)

	// Sanity check to make sure we don't return an "already stale enough" clusterMap to the caller.
	age = time.Since(time.Unix(clusterMap.LastUpdatedAt, 0))
	staleThreshold = 5 * time.Second
	common.Assert(age < staleThreshold, age, staleThreshold)

	return false, nil
}

// Clustermap update is a 3 step operation
// 1. fetch current clusterMap.
// 2. claim update ownership telling other nodes to "keep away".
// 3. process and update clusterMap object.
// 4. commit the update clusterMap.
//
// startClusterMapUpdate() implements step#2 in the above and
// endClusterMapUpdate() implements step#4.
func (cmi *ClusterManager) startClusterMapUpdate(clusterMap *dcache.ClusterMap, etag *string) error {
	clusterMap.LastUpdatedBy = cmi.myNodeId
	clusterMap.State = dcache.StateChecking

	clusterMapByte, err := json.Marshal(clusterMap)
	if err != nil {
		log.Err("ClusterManager::startClusterMapUpdate: Marshal failed for clustermap: %v %+v",
			err, clusterMap)
		common.Assert(false, err)
		return err
	}

	//
	// Claim clustermap update ownership.
	//
	err = mm.UpdateClusterMapStart(clusterMapByte, etag)
	if err != nil {
		log.Warn("ClusterManager::startClusterMapUpdate: Start Clustermap update failed for nodeId %s: %v.",
			cmi.myNodeId, err)
		// Etag mismatch is the only expected error.
		common.Assert(mm.IsErrConditionNotMet(err), err)
		return err
	}

	return nil
}

func (cmi *ClusterManager) endClusterMapUpdate(clusterMap *dcache.ClusterMap) error {
	//
	// endClusterMapUpdate() must be called after startClusterMapUpdate(), hence state must be checking,
	// and we must be the owner.
	//
	common.Assert(clusterMap.State == dcache.StateChecking, clusterMap.State)
	common.Assert(clusterMap.LastUpdatedBy == cmi.myNodeId)

	clusterMap.State = dcache.StateReady
	clusterMap.LastUpdatedAt = time.Now().Unix()

	clusterMapBytes, err := json.Marshal(clusterMap)
	if err != nil {
		err = fmt.Errorf("marshal failed for clustermap: %v %+v", err, clusterMap)
		log.Err("ClusterManager::endClusterMapUpdate: %v", err)
		common.Assert(false, err)
		return err
	}

	//TODO{Akku}: Make sure end update is happening with the same node as of start update
	if err = mm.UpdateClusterMapEnd(clusterMapBytes); err != nil {
		err = fmt.Errorf("updateClusterMapEnd() failed: %v %+v", err, clusterMap)
		log.Err("ClusterManager::endClusterMapUpdate: %v", err)
		common.Assert(false, err)
		return err
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

// This should only be called from fetchAndUpdateLocalClusterMap(), all other users must call
// fetchAndUpdateLocalClusterMap().
var getClusterMap = func() ([]byte, *string, error) {
	return mm.GetClusterMap()
}

var getHeartbeat = func(nodeId string) ([]byte, error) {
	return mm.GetHeartbeat(nodeId)
}

var getAllNodes = func() ([]string, error) {
	return mm.GetAllNodes()
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
	//
	// Fetch and update local clustermap as some of the functions we call later down will query the local clustermap.
	//
	clusterMap, etag, err := cmi.fetchAndUpdateLocalClusterMap()
	if err != nil {
		log.Err("ClusterManager::updateStorageClusterMapIfRequired: fetchAndUpdateLocalClusterMap() failed: %v",
			err)
		common.Assert(false, err)
		return err
	}

	//
	// The node that updated the clusterMap last is preferred over others, for updating the clusterMap.
	// This helps to avoid multiple nodes unnecessarily trying to update the clusterMap (only one of them will
	// succeed but we don't want to waste the effort put by all nodes). But, we have to be wary of the fact that
	// the leader node may go offline, in which case we would want some other node to step up and take the role of
	// the leader. We use the following simple strategy:
	// - Every ClustermapEpoch when the ticker fires, the leader node is automatically eligible for updating the
	//   clusterMap, it need not perform the staleness check.
	// - Every non-leader node has to perform a staleness check which defines a stale clusterMap as one that was
	//   updated more than ClustermapEpoch+thresholdClusterMapEpochTime seconds in the past.
	//   thresholdClusterMapEpochTime is chosen to be 60 secs to prevent minor clock skews from causing a non-leader
	//   to wrongly consider the clusterMap stale and race with the leader for updating the clusterMap. Only when
	//   the leader is down, on the next tick, one of the nodes that runs this code first will correctly find the
	//   clusterMap stale and it'd then take up the job of updating the clusterMap and becoming the new leader if
	//   it's able to successfully update the clusterMap.
	//
	// With these rules, the leader is the one that updates the clusterMap in every tick (ClustermapEpoch), while in
	// case of leader node going down, some other node will update the clusterMap in the next tick. In such case
	// the clusterMap will be updated after two consecutive ClustermapEpoch.
	//

	startTime := time.Now()
	now := startTime.Unix()

	if clusterMap.LastUpdatedAt > now {
		err = fmt.Errorf("LastUpdatedAt (%d) in future, now (%d), skipping update", clusterMap.LastUpdatedAt, now)
		log.Warn("ClusterManager::updateStorageClusterMapIfRequired: %v", err)

		// Be soft if it could be due to clock skew.
		if (clusterMap.LastUpdatedAt - now) < 300 {
			return nil
		}

		// Else, let the caller know.
		common.Assert(false, "cluster.LastUpdatedAt is too much in future", clusterMap.LastUpdatedAt, now)
		return err
	}

	clusterMapAge := now - clusterMap.LastUpdatedAt
	//
	// Assert if clusterMap is not updated for 3 consecutive epochs, it might indicate some bug.
	// For very small ClustermapEpoch values, 3 times the value will not be sufficient as the
	// thresholdClusterMapEpochTime is set to 60, so limit it to 180.
	// The max time till which the clusterMap may not be updated in the event of leader going down is
	// 2*ClustermapEpoch + thresholdClusterMapEpochTime, so for values of ClustermapEpoch above 60 seconds, 3 times
	// ClustermapEpoch is suffcient but for smaller ClustermapEpoch values we have to cap to 180, with a margin
	// of 20 seconds.
	//
	common.Assert(clusterMapAge < int64(max(clusterMap.Config.ClustermapEpoch*3, 200)),
		fmt.Sprintf("clusterMapAge (%d) >= %d",
			clusterMapAge, int64(max(clusterMap.Config.ClustermapEpoch*3, 200))))

	// Staleness check for non-leader.
	stale := clusterMapAge > int64(clusterMap.Config.ClustermapEpoch+thresholdClusterMapEpochTime)
	// Are we the leader node? Leader gets to update the clustermap bypassing the staleness check.
	leaderNode := clusterMap.LastUpdatedBy
	leader := (leaderNode == cmi.myNodeId)

	//
	// If some other node/context is currently updating the clustermap, skip updating in this iteration, as
	// long as the staleness threshold is not met.
	// If some other thread in our node is updating then we play gentle and do not override the clustermap
	// update (despite the staleness threshold), since we are alive and that other thread hopefully will complete.
	// If it doesn't complete in time, some other node will grab ownership.
	// If some other node is updating, and it's possibly dead, then clusterMapBeingUpdatedByAnotherNode() will also
	// grab the ownership.
	//
	isClusterMapUpdateBlocked, err := cmi.clusterMapBeingUpdatedByAnotherNode(clusterMap, etag)
	if err != nil {
		return err
	}

	if isClusterMapUpdateBlocked {
		log.Debug("ClusterManager::updateStorageClusterMapIfRequired:skipping, clustermap is being updated by (leader %s), current node (%s)",
			leaderNode, cmi.myNodeId)

		//
		// Leader node should have updated the state to checking and it should not find the state to checking.
		//
		// TODO: This has been seen to fail when due to remote node being dow updateStorageClusterMapIfRequired()
		//       took long time and the next iteration was called immediately. While it was running, some other
		//       context(s) (updateComponentRVState()) were waiting to update the clusterMap, they immediately
		//       started updating as soon as last iteration of updateStorageClusterMapIfRequired() completed, and
		//       hence when the next iteration of updateStorageClusterMapIfRequired() started immediately, it
		//       finds the clusterMap state as checking.
		//       Still leaving the assert as it's useful to see if it occurrs in any other way.
		//
		common.Assert(!leader, "We don't expect leader to see the clustermap in checking state")
		return nil
	}

	//
	// Ok, clustermap can be possibly updated (can't be sure until startClusterMapUpdate() returns success).
	// If we are the leader, proceed and update the clustermap, else we need to exercise more restrain and
	// only update if it has exceeded the staleness threshold, indicating the current leader has died.
	//
	// Skip if we're neither leader nor the clustermap is stale
	//
	if !leader && !stale {
		log.Info("ClusterManager::updateStorageClusterMapIfRequired: skipping, node (%s) is not leader (leader is %s) and clusterMap is fresh (last updated at epoch %d, now %d, age %s)",
			cmi.myNodeId, leaderNode, clusterMap.LastUpdatedAt, now, clusterMapAge)
		return nil
	}

	//
	// This is an uncommon event, so log.
	//
	if !leader {
		log.Warn("ClusterManager::updateStorageClusterMapIfRequired: clusterMap not updated by current leader (%s) for %s, ownership being claimed by new leader %s",
			leaderNode, clusterMapAge, cmi.myNodeId)
	}

	// TODO: We need to update clusterMap.Epoch to contain the next higher number.

	//
	// Start the clustermap update process by first claiming ownership of the clustermap update.
	// Only one node will succeed in UpdateClusterMapStart(), and that node proceeds with the clustermap
	// update.
	//
	// Note: We still have the Assert() here as it's highly unlikely and it helps to catch any other bug.
	//
	// Note: updateRVList() and updateMVList() are the only functions that can change clustermap.
	//       Enclosing them between UpdateClusterMapStart() and UpdateClusterMapEnd() ensure that only one
	//       node would be updating cluster membership details at any point. This is IMPORTANT.
	//
	err = cmi.startClusterMapUpdate(clusterMap, etag)
	if err != nil {
		err = fmt.Errorf("Start Clustermap update failed for nodeId %s: %v", cmi.myNodeId, err)
		log.Err("ClusterManager::updateStorageClusterMapIfRequired: %v", err)
		common.Assert(false, err)
		return err
	}
	//
	// UpdateClusterMapStart() must not take long. Assert to check that.
	//
	maxTime := 5 * time.Second
	elapsed := time.Since(startTime)
	common.Assert(elapsed < maxTime, elapsed, maxTime)

	log.Info("ClusterManager::updateStorageClusterMapIfRequired: UpdateClusterMapStart succeeded for nodeId %s",
		cmi.myNodeId)

	log.Debug("ClusterManager::updateStorageClusterMapIfRequired: updating RV list")

	_, err = cmi.updateRVList(clusterMap.RVMap, false /* onlyMyRVs */)
	if err != nil {
		err = fmt.Errorf("Failed to reconcile RV mapping: %v", err)
		log.Err("ClusterManager::updateStorageClusterMapIfRequired: %v", err)
		common.Assert(false, err)
		//
		// TODO: We must reset the clusterMap state to ready.
		//
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
	//TODO: Fix this call to trigger only if the RV list has changed.
	// if changed {
	cmi.updateMVList(clusterMap.RVMap, clusterMap.MVMap)
	// } else {
	// log.Debug("ClusterManager::updateStorageClusterMapIfRequired: No changes in RV mapping")
	// }

	//
	// If we have discovered enough nodes (more than the MinNodes config value), clear the clustermap
	// Readonly status. Once clusterMap Readonly is cleared, it remains cleared.
	// Keeping cluster readonly till enough number of nodes have joined the cluster, may help to prevent
	// concentration of data on few early nodes.
	//
	nodeCount := len(getAllNodesFromRVMap(clusterMap.RVMap))
	if clusterMap.Readonly && nodeCount >= int(cmi.config.MinNodes) {
		log.Info("ClusterManager::updateStorageClusterMapIfRequired: Discovered node count %d greater than MinNodes (%d), clearing clusterMap Readonly status. New files can be created now!",
			nodeCount, cmi.config.MinNodes)

		clusterMap.Readonly = false
	}

	//
	// Check if the time elapsed since we read the global clusterMap and till we could run all the updates,
	// has exceeded ClustermapEpoch. If so, we can be at risk of having our clusterMap updates race with some
	// other node (that might have claimed ownership due to timeout). In that case we drop all the updates we
	// made to the clusterMap and do not commit them.
	// This can happen if one or more nodes are not reachable, and updateMVList() had to send some JoinMV/UpdateMV
	// RPCs, which had to timeout.
	//
	elapsed = time.Since(startTime)
	maxTime = time.Duration(clusterMap.Config.ClustermapEpoch) * time.Second
	if elapsed > maxTime {
		//
		// TODO: We must reset the clusterMap state to ready.
		//
		err = fmt.Errorf("clustermap update (%s) took longer than ClustermapEpoch (%s), bailing out",
			elapsed, maxTime)
		log.Err("ClusterManager::updateStorageClusterMapIfRequired: %v", err)
		common.Assert(false, err)
		return err
	}
	err = cmi.endClusterMapUpdate(clusterMap)
	if err != nil {
		log.Err("ClusterManager::updateStorageClusterMapIfRequired: %v", err)
		common.Assert(false, err)
		return err
	}

	log.Info("ClusterManager::updateStorageClusterMapIfRequired: cluster map (%d nodes) updated by %s at %d: %+v",
		nodeCount, cmi.myNodeId, now, clusterMap)
	return nil
}

// Given an rvMap which holds most uptodate status of all known RVs, whether they are "online" or "offline" (this
// information is mostly derived from the heartbeats, by a prior call to updateRVList(), but it can be known through
// some other mechanism, f.e., inband detection of RV offline status by RPC calls made to nodes), and existingMVMap
// which is the set of MVs present in the clustermap, indexed by MV name and contains complete info about the MV,
// updateMVList() correctly updates all MVs' component RVs state and the derived MV state.
// It can be called from two workflows:
//  1. From updateStorageClusterMapIfRequired() (periodic clustermap update thread) after it infers some change in
//     rvMap as per the latest heartbeats received.
//  2. From updateComponentRVState(), when some other workflow wants to explicitly update component RV state for some
//     MV, f.e., resync workflow may want to change an "outofsync" component RV to "syncing" or a failed PutChunk call
//     may indicate an RV as down and hence we would want to change the component RV state to "offline". There could
//     be more such examles of inband RV state detection resulting in MV list update.
//
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
// Note that when setting MV state based on component RV state, a component RV in "outofsync" or "syncing" state is
// treated as an offline component RV, basically any component RV which does not have valid data.
//
// existingMVMap is updated in-place, the caller will then publish it in the updated clustermap.
//
// Note: updateMVList() MUST be called after successfully claiming ownership of clusterMap update, by a successful
//
//	call to UpdateClusterMapStart(). This is IMPORTANT to ensure only one node attempts to update clusterMap
//	at any point.
func (cmi *ClusterManager) updateMVList(rvMap map[string]dcache.RawVolume,
	existingMVMap map[string]dcache.MirroredVolume) {
	// We should not be called for an empty rvMap.
	common.Assert(len(rvMap) > 0)

	NumReplicas := int(cmi.config.NumReplicas)
	MvsPerRv := int(cmi.config.MvsPerRv)

	common.Assert(NumReplicas >= int(cm.MinNumReplicas) && NumReplicas <= int(cm.MaxNumReplicas), NumReplicas)
	common.Assert(MvsPerRv >= int(cm.MinMvsPerRv) && MvsPerRv <= int(cm.MaxMvsPerRv), MvsPerRv)
	common.Assert(cm.IsValidRVMap(rvMap))
	common.Assert(cm.IsValidMvMap(existingMVMap, NumReplicas))

	//
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
	// TODO: Pick component RVs across fault domains and not just across nodes.
	//

	log.Debug("ClusterManager::updateMVList: Updating current MV list according to the latest RV list.")

	//
	// Represents an RV.
	// An RV has a name and slots to indicate how many times the RV has been used up in various MVs.
	// One MV can use an RV at most once. slots is initialized with MvsPerRv and then decremented by
	// one everytime an RV is found/selected as component RV to an MV.
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

	//
	// All nodes with their RVs, indexed by nodeid.
	//
	nodeToRvs := make(map[string]node)

	//
	// Helper function to consume an rv slot when rvName is allotted to mvName.
	//
	// This updates nodeToRvs.
	//
	consumeRVSlot := func(mvName, rvName string) {
		nodeId := rvMap[rvName].NodeId
		// Simple assert to make sure rvName is present in rvMap.
		common.Assert(len(nodeId) > 0)
		// We don't add offline RVs to nodeToRvs, so we must not update their slot count.
		common.Assert(rvMap[rvName].State == dcache.StateOnline, mvName, rvName, rvMap[rvName].State)
		found := false

		// Decrease the slot count for the RV in nodeToRvs
		for i := range nodeToRvs[nodeId].rvs {
			if nodeToRvs[nodeId].rvs[i].rvName == rvName {
				common.Assert(nodeToRvs[nodeId].rvs[i].slots > 0, nodeId, rvName, mvName)
				nodeToRvs[nodeId].rvs[i].slots--
				found = true
				break
			}
		}

		// Component RV for MV must be present in nodeToRvs.
		common.Assert(found, mvName, rvName, nodeId)
	}

	//
	// Helper function to remove an RV from a given node.
	// This is called when an RV is found to be "bad" and we don't want to use it for subsequent RV
	// allocations.
	//
	// This updates nodeToRvs.
	//
	deleteRVFromNode := func(deleteRvName string) {
		nodeId := rvMap[deleteRvName].NodeId
		// Simple assert to make sure deleteRvName is present in rvMap.
		common.Assert(len(nodeId) > 0)
		// We don't add offline RVs to nodeToRvs, so we must not be deleting them from nodeToRvs.
		common.Assert(rvMap[deleteRvName].State == dcache.StateOnline, deleteRvName, rvMap[deleteRvName].State)
		found := false

		for i, rv := range nodeToRvs[nodeId].rvs {
			if rv.rvName != deleteRvName {
				continue
			}

			// Delete rv from the list of RVs for the node.
			node := nodeToRvs[nodeId]
			node.rvs = append(node.rvs[:i], node.rvs[i+1:]...)
			nodeToRvs[nodeId] = node

			found = true
			log.Debug("ClusterManager::deleteRVFromNode: Deleted RV %s from node %s", deleteRvName, nodeId)
			break
		}

		// Component RV for MV must be present in nodeToRvs.
		common.Assert(found, deleteRvName, nodeId)
	}

	//
	// Helper function to perform nodeToRvs trimming. It does the following:
	// - Remove RVs whose slot count have reached 0.
	// - Remove nodes with no RVs left.
	//
	// This must be called after one or more RVs are assigned to an MV (fix-mv or new-mv).
	//
	// This updates nodeToRvs.
	//
	trimNodeToRvs := func() {
		//
		// Check if any node has exhausted all its RV's, remove such nodes from the nodeToRvs map.
		//
		for nodeId, node := range nodeToRvs {
			for j := 0; j < len(node.rvs); {
				// Remove an RV if it has no free slots left.
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
	}

	//
	// Helper function to replace offline RVs of an MV with suitable good RVs. If successful it fixes the
	// MV stored in existingMVMap[mvName], else if it fails it leaves existingMVMap[mvName] unchanged.
	// This implements the fix-mv workflow.
	// This must be called after the degrade-mv workflow has run and the state of component RVs have been
	// duly updated.
	//
	fixMV := func(mvName string, mv dcache.MirroredVolume) {
		//
		// Fix-mv must be run only for degraded MVs.
		// A degraded MV has one or more (but not all) component RVs as offline (which need to be replaced by
		// a good RV).
		//
		common.Assert(mv.State == dcache.StateDegraded, mvName, mv.State)

		// MV must have all the component RVs set.
		common.Assert(len(mv.RVs) == NumReplicas, len(mv.RVs), NumReplicas)

		offlineRVs := 0
		outofsyncRVs := 0
		excludeNodes := make(map[string]struct{})
		excludeRVNames := make(map[string]struct{})

		//
		// Pass 1: Make a list of nodes and RVs to be excluded when picking "good" RVs in the later part.
		//         Those nodes are excluded which contribute at least one good component RV.
		//         Those component RVs are excluded which are offline.
		//
		for rvName := range mv.RVs {
			// Only valid RVs can be used as component RVs for an MV.
			_, exists := rvMap[rvName]
			common.Assert(exists)

			//
			// Fix-mv workflow is run after degrade-mv/offline-mv workflows, so component RV states
			// must have been correctly updated. Also component RV state must be online, offline or
			// syncing.
			//

			//
			// If state of RV in rvMap is offline, state of component RV in MV MUST be offline.
			// Note that we can have an RV as online in rvMap but still not online in MV, since once
			// an RV goes offline and comes back it cannot simply be marked online in the MV, it has
			// to go through degrade-mv/fix-mv workflows.
			//
			common.Assert(rvMap[rvName].State == dcache.StateOnline ||
				mv.RVs[rvName] == dcache.StateOffline,
				rvName, mvName, rvMap[rvName].State, mv.RVs[rvName])

			//
			// fixMV() is called after degrade-mv/offline-mv workflow has run. That would only result in
			// one or more (but not all) offline component RVs for an MV, so we should not have component
			// RVs in "outofsync" state (fixMV() is the one who moves component RVs from offline to outofsync)
			// when fixMV() is called. Once fixMV() marks component RVs as outofsync, it should be soon followed
			// by ResyncMV() from Replication Manager which will change outofsync to syncing.
			// BUT, in the following scenario we CAN HAVE outofsync component RVs when fixMV() is called:
			// - Between the last fixMV() call that set a component RVs state as outofsync, and this call,
			//   ResyncMV didn't get a chance to run. This is unlikely but possible.
			// - ResyncMV did run, but since ResyncMV fixes one degraded MV and in one degraded MV one outofsync
			//   RV, at a time, if there are more than one degraded MVs and/or more than one outofsync RVs for an
			//   MV, when updateMVList()->fixMV() is called from updateComponentRVState(), we can have component
			//   RVs still in outofsync state.
			//
			// Leave this assert commented to highlight the above.
			//
			// common.Assert(mv.RVs[rvName] != dcache.StateOutOfSync, rvName, mv.RVs[rvName])

			if mv.RVs[rvName] == dcache.StateOutOfSync {
				outofsyncRVs++
			}

			//
			// If this component RV is not offline, its containing node must be excluded for replacement RV(s).
			// We don't exclude the node if the component RV is offline to support the case where the same node
			// comes back up online and we may want to use the same RV or another RV from the same node, as
			// replacement RV.
			//
			if mv.RVs[rvName] != dcache.StateOffline {
				excludeNodes[rvMap[rvName].NodeId] = struct{}{}
				continue
			}

			//
			// Offline RVs themselves must be excluded. Those are the ones we need to replace with good ones.
			// Note that it's possible that the same RV has now come back online, in which case it can be
			// reused and hence must not be excluded.
			//
			if rvMap[rvName].State == dcache.StateOffline {
				excludeRVNames[rvName] = struct{}{}
			}

			offlineRVs++
		}

		// Degraded MVs must have one or more but not all component RVs as offline/outofsync.
		common.Assert((offlineRVs+outofsyncRVs) != 0 && (offlineRVs+outofsyncRVs) < NumReplicas,
			mvName, offlineRVs, outofsyncRVs, NumReplicas)

		// No component RV is offline, nothing to fix, return.
		if offlineRVs == 0 {
			// If not offline, must have at least one outofsync, else why the MV is degraded.
			common.Assert(outofsyncRVs > 0, mvName)
			log.Debug("ClusterManager::fixMV: %s has no offline component RV, nothing to fix %+v",
				mvName, mv.RVs)
			return
		}

		//
		// Pass 2: For all component RVs that are offline, find a suitable RV.
		//         A suitable RV is one, that:
		//         - Does not come from any node in excludeNodes list.
		//         - Is not one of excludeRVNames.
		//         - Has the higher availableSpace/
		//
		// Shuffle the nodes to encourage random selection of replacement RV(s).
		// We then iterate over the availableNodes list and pick the 1st suitable RV.
		//
		var availableNodes []node
		for _, n := range nodeToRvs {
			availableNodes = append(availableNodes, n)

		}

		rand.Shuffle(len(availableNodes), func(i, j int) {
			availableNodes[i], availableNodes[j] = availableNodes[j], availableNodes[i]
		})

		//
		// Number of component RVs we are actually able to fix for this MV.
		// If we cannot fix anything, skip the joinMV().
		//
		fixedRVs := 0
		alreadyOutOfSync := make(map[string]struct{})

		//
		// We make a deep copy of mv.RVs before we start fixing.
		// We fix directly in mv.RVs as it's convenient, but if we need to undo later we set mv.RVs to
		// savedRVs.
		//
		savedRVs := make(map[string]dcache.StateEnum)
		for rvName, rvState := range mv.RVs {
			savedRVs[rvName] = rvState
		}

		for rvName := range mv.RVs {
			//
			// Usually we won't have outofsync component RVs when fixMV() is called, as they would have been
			// picked by the resync workflow and changed to syncing/online by the time fixMV() is called next
			// time, but it's possible that we are called with one or more outofsync component RVs.
			//
			// e.g., fixMV() was called for mv1 with the following component RVs.
			// mv2:{degraded map[rv0:outofsync rv3:online rv4:offline]}
			//
			// Note tha rv0 was outofsync on entry and rv4 was replaced by rv1 and rv1 was newly marked outofsync,
			// so the component RVs after the replacement looked like
			// mv2:{degraded map[rv0:outofsync rv3:online rv1:outofsync]}
			//
			// After joinMV() below succeeds, we won't know if a component RV was already outofsync or it was
			// just picked as a replacement for an offline RV and is hence newly marked outofsync. We will try to
			// consume slot for both rv0 (already outofsync) and rv1 (newly made outofsync). Note that rv0 would
			// have its slot already consumed as outofsync component RV do consume slots. This will cause "double
			// consume". We need to avoid this, so we store RVs which are already outofsync in a map and then check
			// before calling consumeRVSlot() after joinMV().
			//
			if mv.RVs[rvName] == dcache.StateOutOfSync {
				log.Debug("ClusterManager::fixMV: %s/%s already outofsync", rvName, mvName)
				alreadyOutOfSync[rvName] = struct{}{}
			}

			// Only offline component RVs need to be "fixed" (aka replaced).
			if mv.RVs[rvName] != dcache.StateOffline {
				continue
			}

			foundReplacement := false
			// Iterate over the shuffled nodes list and pick the first suitable RV.
			for _, node := range availableNodes {
				_, ok := excludeNodes[node.nodeId]
				if ok {
					// Skip excluded nodes.
					continue
				}

				// Potential node, pick first suitable RV.
				for idx := range node.rvs {
					newRvName := node.rvs[idx].rvName
					_, ok := excludeRVNames[newRvName]
					if ok {
						// Skip excluded RVs.
						continue
					}

					//
					// Do not pick another offline component RV as replacement, else mv.RVs[] will have fewer
					// than NumReplicas RVs.
					// e.g., let's say we enter fixMV() with the following mv0 composition,
					// mv0: {rv0: offline, rv1: online, rv2: offline}
					//
					// if we don't disallow the following, we can pick rv2 as a replacement for rv0, resulting in
					// mv0: {rv2: outofsync, rv1: online}
					//
					// But, it's ok to reuse the same RV if it's now online, so following is a valid replacement.
					// mv0: {rv0: outofsync, rv1: online, rv2: outofsync}
					//
					if newRvName != rvName {
						_, ok := mv.RVs[newRvName]
						if ok {
							continue
						}
					}

					//
					// TODO: Need to find out space requirement for the MV and exclude RVs
					//       which do not have enough availableSpace.

					//
					// Use this RV to replace older RV, a newly replaced RV starts as "outofsync" to indicate
					// that the RV is good but needs to be sync'ed (from a good component RV).
					//
					// Remove the bad RV from MV. Do this before assigning the replacement RV, in case both
					// are same.
					//
					delete(mv.RVs, rvName)
					mv.RVs[newRvName] = dcache.StateOutOfSync

					//
					// Now mv is updated to correctly reflect new selected RV, with bad RV removed.
					// We don't yet update existingMVMap, we will do it once joinMV() returns
					// successfully.
					//

					log.Debug("ClusterManager::fixMV: Replacing (%s/%s -> %s/%s)",
						rvName, mvName, newRvName, mvName)
					foundReplacement = true
					fixedRVs++
					break
				}

				if foundReplacement {
					// Once we pick an RV from a node, it cannot be used again for another RV for the MV.
					excludeNodes[node.nodeId] = struct{}{}
					break
				}
			}

			//
			// If we could not find a replacement RV for an offline RV, it's a matter of concern as the MV
			// will be forced to run degraded for a longer period risking data loss.
			//
			// TODO: For huge clusters availableNodes could be a lot of log.
			//
			if !foundReplacement {
				log.Warn("ClusterManager::fixMV: No replacement RV found for %s/%s, availableNodes: %+v, excludeNodes: %+v, excludeRVNames: %+v",
					rvName, mvName, availableNodes, excludeNodes, excludeRVNames)
			}
		}

		// We should be fixing no more than offlineRVs RVs.
		common.Assert(fixedRVs <= offlineRVs, fixedRVs, offlineRVs)

		// Skip joinMV() if nothing changed in clustermap.
		if fixedRVs == 0 {
			log.Warn("ClusterManager::fixMV: Could not fix any RV for MV %s", mvName)
			return
		}

		//
		// Ok, we have selected a replacement RV for each offline component RV, but before we can finalize
		// the selection, we need to check with the RV.
		// Call joinMV() and check if all component RVs are able to join successfully.
		// Note that though it's called joinMV(), it sends both JoinMV and UpdateMV RPC depending on the
		// RV state. An RV which is being added to an MV for the first time (either new MV or replacing a
		// bad component RV) is sent the JoinMV RPC while an existing component RV which just needs to be
		// made aware of the component RVs is sent the UpdateMV RPC>
		//
		// Iff joinMV() is successful, consume one slot for each component RV and update existingMVMap.
		//
		// TODO: Set reserveBytes correctly, querying it from our in-core RV info maintained by RPC server.
		//
		failedRV, err := cmi.joinMV(mvName, mv, 0 /* reserveBytes */)
		if err == nil {
			log.Info("ClusterManager::fixMV: Successfully joined/updated all component RVs %+v to MV %s, original [%+v]",
				mv.RVs, mvName, savedRVs)
			for rvName := range mv.RVs {
				//
				// Consume slot for the replacement RVs, just made outofsync, but skip RVs which were already
				// outofsync on entry to fixMV().
				//
				if mv.RVs[rvName] == dcache.StateOutOfSync {
					_, exists := alreadyOutOfSync[rvName]
					if !exists {
						consumeRVSlot(mvName, rvName)
					}
				}
			}
			existingMVMap[mvName] = mv

			//
			// After the consumeRVSlot() above we need to trim the nodeToRvs map as it may have fully consumed
			// some RV(s). We don't want to use those RVs in the next fixMV() iteration(s).
			//
			trimNodeToRvs()
		} else {
			//
			// If we fail to fix the MV we simply return leaving the broken MV in existingMVMap.
			// TODO: We should add retries here.
			//
			log.Err("ClusterManager::fixMV: Error joining RV %s with MV %s: %v, reverting [%+v -> %+v]",
				failedRV, mvName, err, mv.RVs, savedRVs)

			mv.RVs = savedRVs
			existingMVMap[mvName] = mv
		}
	}

	//
	// Populate the node map (indexed by nodeid) with each node representing all its RVs.
	// This is the nodeToRvs map.
	//
	for rvName, rvInfo := range rvMap {
		common.Assert(cm.IsValidRV(&rvInfo))

		if rvInfo.State == dcache.StateOffline {
			// Skip RVs that are offline as they cannot contribute to any MV.
			continue
		}

		if nodeInfo, exists := nodeToRvs[rvInfo.NodeId]; exists {
			// If the node already exists, append the RV to its list.
			// This will be the case when node has more than one RV and we are encountering the second
			// or subsequent RVs.
			common.Assert(rvInfo.NodeId == nodeInfo.nodeId, rvInfo.NodeId, nodeInfo.nodeId)
			common.Assert(len(nodeInfo.rvs) > 0)
			common.Assert(nodeInfo.rvs[0].slots == MvsPerRv, nodeInfo.rvs[0].slots, MvsPerRv)
			common.Assert(nodeInfo.rvs[0].rvName != rvName, rvName)

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
	// Go over all MVs in existingMVMap and correctly set MV's state based on the state of all the
	// component RVs and consume RV slots for all used component RVs. If a component RV is found to be offline as per rvMap, then the component RV
	// is force marked offline. Then it sets the MV state based on the cumulative state of all of it's component RVs as follows:
	// - If all component RVs of an MV are online, the MV is marked as online, else
	// - If no component RV of an MV is online (they are either offline, outofsync or syncing), the MV
	//   is marked as offline, else
	// - If at least one component RV is online, the MV is marked as degraded, else
	// - All component RVs are either online or syncing, then MV is marked as syncing.
	//
	// Few examples:
	// online, online, online => online
	// online, online, offline => degraded
	// online, online, outofsync => degraded
	// online, outofsync, outofsync => degraded
	// online, syncing, outofsync => degraded
	// online, syncing, syncing => syncing
	// online, online, syncing => syncing
	// offline, syncing, syncing => offline
	// offline, outofsync, syncing => offline
	// offline, outofsync, outofsync => offline
	// offline, offline, offline => offline
	//
	for mvName, mv := range existingMVMap {
		offlineRVs := 0
		syncingRVs := 0
		onlineRVs := 0
		outofsyncRVs := 0

		for rvName := range mv.RVs {
			// Only valid RVs can be used as component RVs for an MV.
			_, exists := rvMap[rvName]
			common.Assert(exists)

			// First things first, an offline RV MUST be marked as an offline component RV.
			if rvMap[rvName].State == dcache.StateOffline {
				mv.RVs[rvName] = dcache.StateOffline
			}

			if mv.RVs[rvName] == dcache.StateOnline {
				onlineRVs++
			} else if mv.RVs[rvName] == dcache.StateOffline {
				offlineRVs++
			} else if mv.RVs[rvName] == dcache.StateOutOfSync {
				outofsyncRVs++
			} else if mv.RVs[rvName] == dcache.StateSyncing {
				syncingRVs++
			}

			//
			// This RV is not offline and is used as a component RV by this MV.
			// Reduce its slot count, so that we don't use a component RV more than MvsPerRv times across all MVs.
			// Note that offline RVs are not included in nodeToRvs so we should not be updating their slot count.
			//
			// We don't reduce slot count if the component RV itself is marked offline. This is because an offline
			// component RV for all purposes can be treated as non-existent. Soon after this we will run the fix-mv
			// workflow which will replace these offline RVs with some online RV (it could be the same RV if it has
			// come back up online) and at that time we will not increase the slot count of the outgoing component
			// RV, so we don't reduce it now.
			//
			if rvMap[rvName].State != dcache.StateOffline {
				if mv.RVs[rvName] != dcache.StateOffline {
					consumeRVSlot(mvName, rvName)
				}
			}
		}

		common.Assert((onlineRVs+offlineRVs+outofsyncRVs+syncingRVs) == len(mv.RVs),
			onlineRVs, offlineRVs, outofsyncRVs, syncingRVs, len(mv.RVs))

		if (offlineRVs + outofsyncRVs + syncingRVs) == len(mv.RVs) {
			// No component RV is online, offline-mv.
			mv.State = dcache.StateOffline
		} else if onlineRVs == len(mv.RVs) {
			mv.State = dcache.StateOnline
		} else if offlineRVs > 0 || outofsyncRVs > 0 {
			common.Assert(onlineRVs > 0 && onlineRVs < len(mv.RVs), onlineRVs, len(mv.RVs))
			// At least one component RV is not online but at least one is online, degrade-mv.
			mv.State = dcache.StateDegraded
		} else if syncingRVs > 0 {
			common.Assert((syncingRVs+onlineRVs) == len(mv.RVs), syncingRVs, onlineRVs, len(mv.RVs))
			mv.State = dcache.StateSyncing
		} else {
			common.Assert(false)
		}

		existingMVMap[mvName] = mv
	}

	//
	// Check if any node has exhausted all its RV's, remove such nodes from the nodeToRvs map.
	// Also remove RVs which are fully consumed (no free slots left).
	//
	trimNodeToRvs()

	//
	// TODO: Shall we commit the clustermap changes (marking offline component RVs as offline in MV)?
	//       Note that fixMV() will call UpdateMV RPC which only allows legitimate component RVs update.
	//       For that it'll refresh the clustermap and if it gets the old clustermap (with RV as online),
	//       UpdateMV will ll fail.
	//
	log.Debug("ClusterManager::updateMVList: existingMVMap after phase#1: %v", existingMVMap)

	//
	// Phase 2:
	//
	// Fix the degraded MVs by replacing their offline RVs with good ones.
	// This is the fix-mv workflow.
	//
	// Note that we can/must only fix degraded MVs, offline MVs cannot be fixed as there's no good component
	// RV to copy chunks from. Once an MV is offline it won't be used by File Manager to put any file's data.
	// Offline MVs will just be lying around like satelite debris in space.
	//
	// TODO: See if we need delete-mv workflow to clean those up.
	//

	numUsableMVs := 0
	for mvName, mv := range existingMVMap {
		if mv.State != dcache.StateOffline {
			numUsableMVs++
		}

		if mv.State != dcache.StateDegraded {
			continue
		}

		fixMV(mvName, mv)
	}

	log.Debug("ClusterManager::updateMVList: existing MV map after phase#2: %v", existingMVMap)

	//
	// Phase 3:
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
		// Also remove RVs which are fully consumed (no free slots left).
		//
		trimNodeToRvs()

		//
		// The nodeToRvs map is now updated with remaining nodes and their RVs.
		// For this iteration of new-mv workflow, we have only those nodes left which can contribute
		// at least one RV and only those RVs left which have at least one slot to contribute.
		//

		// New MV will need at least NumReplicas distinct nodes.
		if len(nodeToRvs) < NumReplicas {
			log.Debug("ClusterManager::updateMVList: len(nodeToRvs) [%d] < NumReplicas [%d]",
				len(nodeToRvs), NumReplicas)
			break
		}

		//
		// With rvMap and MvsPerRv and NumReplicas, we cannot have more than maxMVsPossible usable MVs.
		// Note that we are talking of online or degraded/syncing MVs. Offline MVs have all component RVs
		// offline and they don't consume any RV slot, so they should be omitted from usable MVs.
		//
		// Q: Why do we need to limit numUsableMVs to maxMVsPossible?
		//    IOW, why is the the above check "len(nodeToRvs) < NumReplicas" not suffcient.
		// A: "len(nodeToRvs) < NumReplicas" check will try to create as many MVs as we can with the available
		//    RVs, but it might create more than maxMVsPossible if some of the MVs have offline RVs (fixMV() would
		//    have attempted to replace offline RVs for all degraded MVs but if joinMV() fails or any other error
		//    we can have some component RVs as offline). We don't want to create more MVs leaving some MVs with
		//    no replacement RVs available.
		//
		maxMVsPossible := (len(rvMap) * MvsPerRv) / NumReplicas
		common.Assert(numUsableMVs <= maxMVsPossible, numUsableMVs, maxMVsPossible)

		if numUsableMVs == maxMVsPossible {
			log.Debug("ClusterManager::updateMVList: numUsableMVs [%d] == maxMVsPossible [%d]",
				numUsableMVs, maxMVsPossible)
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
			// Only nodes with at least one RV will be present in selectedNodes.
			common.Assert(len(n.rvs) > 0)

			for _, r := range n.rvs {
				common.Assert(n.nodeId == rvMap[r.rvName].NodeId, n.nodeId, rvMap[r.rvName].NodeId)

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

				//
				// We decrease the slot count for the RV in nodeToRvs, only after a successful
				// joinMV() call. Note that it's ok to defer slot count adjustment as one RV will
				// be used not more than once as component RV for an MV.
				//

				break
			}
		}

		common.Assert(len(existingMVMap[mvName].RVs) == NumReplicas,
			mvName, len(existingMVMap[mvName].RVs), NumReplicas)

		//
		// Call joinMV() and check if all component RVs are able to join successfully.
		// reserveBytes is 0 for a new-mv workflow.
		//
		// Iff joinMV() is successful, consume one slot for each component RV, else if joinMV() fails
		// delete the failed RV fom nodeToRvs to prevent this RV from being picked again and failing.
		// Also we need to remove mv frome existingMVMap.
		//
		failedRV, err := cmi.joinMV(mvName, existingMVMap[mvName], 0 /* reserveBytes */)
		if err == nil {
			log.Info("ClusterManager::updateMVList: Successfully joined all component RVs %+v to MV %s",
				existingMVMap[mvName].RVs, mvName)

			for rvName := range existingMVMap[mvName].RVs {
				// All component RVs added by the new-mv workflow must be online.
				common.Assert(existingMVMap[mvName].RVs[rvName] == dcache.StateOnline,
					rvName, mvName, existingMVMap[mvName].RVs[rvName])
				consumeRVSlot(mvName, rvName)
			}

			// One more usable MV added to existingMVMap.
			numUsableMVs++
			common.Assert(numUsableMVs <= len(existingMVMap), numUsableMVs, len(existingMVMap))
		} else {
			// TODO: Give up reallocating RVs after a few failed attempts.
			log.Err("ClusterManager::updateMVList: Error joining RV %s with MV %s: %v",
				failedRV, mvName, err)

			deleteRVFromNode(failedRV)
			// Delete the MV from the existingMVMap.
			delete(existingMVMap, mvName)
		}
	}

	log.Debug("ClusterManager::updateMVList: existing MV map after phase#3: %v", existingMVMap)

}

// Given an MV, send JoinMV or UpdateMV RPC to all its component RVs. It fails if any of the RV fails the call.
// This must be called from new-mv or fix-mv workflow to let the component RVs know about the new membership details.
// It calls JoinMV for RVs joining the MV newly and UpdateMV for existing component RVs which need to be informed of
// the updated membership details. For JoinMV RPC requests it sets the ReserveSpace to reserveBytes.
// The caller must have updated 'mv' with the correct component RVs and their state before calling this.
// 'reserveBytes' is the amount of space to reserve in the RV. This will be 0 when joinMV() is called from the
// new-mv workflow, but can be non-zero when called from the fix-mv workflow for replacing an offline RV with
// a new good RV. The new RV must need enough space to store the chunks for this MV.
//
// It sends JoinMV/UpdateMV based on following:
//   - It sends JoinMV RPC to all RVs of a new MV. A new MV is one which has state of online, because we will not be
//     called o/w for an online MV.
//   - For existing MVs, it sends JoinMV for those RVs which have StateOutOfSync state. These are new RVs selected by
//     fix-mv workflow.
//   - For existing MVs, it sends UpdateMV for online component RVs.
//
// TODO: joinMV() should technically return more than one failed RVs.
func (cmi *ClusterManager) joinMV(mvName string, mv dcache.MirroredVolume, reserveBytes int64) (string, error) {
	log.Debug("ClusterManagerImpl::joinMV: JoinMV(%s, %+v, reserveBytes: %d)", mvName, mv, reserveBytes)

	var componentRVs []*models.RVNameAndState
	var numRVsOnline int

	// Are we called from new-mv workflow? If not, then we are called from fix-mv workflow.
	newMV := (mv.State == dcache.StateOnline)

	//
	// JoinMV/UpdateMV RPC can only be sent in the following two cases:
	// 1. From new-mv workflow - MV state must be online in this case.
	// 2. From fix-mv workflow - MV state must be degraded in this case.
	//
	common.Assert(mv.State == dcache.StateOnline || mv.State == dcache.StateDegraded, mv.State)

	// Caller must call us only with all component RVs set.
	common.Assert(len(mv.RVs) == int(cmi.config.NumReplicas), len(mv.RVs), cmi.config.NumReplicas)

	// reserveBytes must be non-zero only for degraded MV, for new-mv it'll be 0.
	common.Assert(reserveBytes == 0 || mv.State == dcache.StateDegraded, reserveBytes, mv.State)

	for rvName, rvState := range mv.RVs {
		log.Debug("ClusterManagerImpl::joinMV: Populating componentRVs list MV %s with RV %s", mvName, rvName)

		//
		// For new-mv all component RVs must be online, for fix-mv we can have the following component RV states:
		// - outofsync: These are the ones which were offline and have been fixed by the fix-mv workflow.
		// - offline: These were offline RVs, went to fix-mv but fix-mv could not find a replacement RV for these.
		// - online: These are the online component RVs. One or more component RVs must be offline/outofsync else
		//           MV won't be degraded and we won't run fix-mv.
		// - syncing: These are the component RVs currently syncing. One or more component RVs must be
		//            offline/outofsync else MV won't be degraded and we won't run fix-mv.
		//

		if rvState == dcache.StateOnline {
			numRVsOnline++
		}

		componentRVs = append(componentRVs, &models.RVNameAndState{
			Name:  rvName,
			State: string(rvState),
		})
	}

	// For newMV case, *all* component RVs must be online.
	common.Assert(newMV == (numRVsOnline == len(mv.RVs)), mvName, mv.State, numRVsOnline)

	//
	// TODO: Call JoinMV/UpdateMV on all RVs in parallel.
	// TODO: If JoinMV() fails to any RV, need to send LeaveMV() to the RVs which succeeded for undoing the
	//       reserveBytes. We can also achieve the same result in a better way by the target RV automatically
	//       undoing the reserveBytes (and the JoinMV) if it doesn't get a SyncMV within certain timeout period.
	//
	for _, rv := range componentRVs {
		//
		// Offline component RVs need not be sent JoinMV/UpdateMV RPC.
		// TODO: Shall we send them LeaveMV RPC?
		//
		if mv.RVs[rv.Name] == dcache.StateOffline {
			continue
		}

		log.Debug("ClusterManagerImpl::joinMV: Joining MV %s with RV %s", mvName, rv.Name)

		joinMvReq := &models.JoinMVRequest{
			MV:           mvName,
			RVName:       rv.Name,
			ReserveSpace: reserveBytes,
			ComponentRV:  componentRVs,
		}

		updateMvReq := &models.UpdateMVRequest{
			MV:          mvName,
			RVName:      rv.Name,
			ComponentRV: componentRVs,
		}

		// TODO: Use timeout from some global variable.
		timeout := 2 * time.Second
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		var err error
		var action string

		if newMV || mv.RVs[rv.Name] == dcache.StateOutOfSync {
			//
			// All RVs of a new MV are sent JoinMV RPC.
			// else for fix-mv case outofsync component RVs are sent JoinMV RPC.
			//
			_, err = rpc_client.JoinMV(ctx, cm.RVNameToNodeId(rv.Name), joinMvReq)
			action = "joining"
		} else {
			//
			// Else, fix-mv and online RV, send UpdateMV.
			//
			common.Assert(mv.RVs[rv.Name] == dcache.StateOnline)
			_, err = rpc_client.UpdateMV(ctx, cm.RVNameToNodeId(rv.Name), updateMvReq)
			action = "updating"
		}

		if err != nil {
			err = fmt.Errorf("Error %s MV %s with RV %s: %v", action, mvName, rv.Name, err)
			log.Err("ClusterManagerImpl::joinMV: %v", err)
			return rv.Name, err
		}
	}

	return "", nil
}

// Given the list of existing RVs in clusterMap, add any new RVs available.
// If onlyMyRVs is true then the only RV(s) added/updated are the ones exported by the current node, else it queries
// the heartbeats from all nodes and adds all new RVs available and updates all RVs.
// existingRVMap is updated in-place.
//
// Note: updateRVList() MUST be called after successfully claiming ownership of clusterMap update, by a successful
//
//	call to UpdateClusterMapStart().
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
	rVsByRvIdFromHB, rvLastHB, err := collectHBForGivenNodeIds(nodeIds)
	if err != nil {
		return false, err
	}
	// Both the RV and the RV HB map must have the exact same RVs.
	common.Assert(len(rVsByRvIdFromHB) == len(rvLastHB), len(rVsByRvIdFromHB), len(rvLastHB))

	// Set to true if we add any new RV to existingRVMap or update the state of any existing RV.
	changed := false

	//
	// Ok, now we have all the RVs from heartbeats in rVsByRvIdFromHB[].
	// We need to do two things:
	// 1. For existing RVs (in existingRVMap), update RV State and AvailableSpace in existingRVMap if it's different
	//    from the RV State and AvailableSpace as seen in the HB, or if HB has expired set RV state to offline if not
	//    already offline.
	//    If any of the existing RVs is not found in the latest HBs, for such RVs too the state in existingRVMap is
	//    set to offline. Note that for onlyMyRVs==true case we cannot do this as we don't have the exhaustive list of
	//    HBs.
	// 2. All RVs in rVsByRvIdFromHB[] which are not present in existingRVMap, i.e., those RVs are newly seen,
	//    add those to existingRVMap.
	//

	// If an RV has the LastHeartbeat less than hbExpiry, it needs to be offlined.
	now := uint64(time.Now().Unix())
	hbExpiry := now - uint64(hbTillNodeDown*hbSeconds)

	// (1.b) Update RVs present in existingRVMap and which have changed State or AvailableSpace.
	for rvName, rvInClusterMap := range existingRVMap {

		if rvHb, found := rVsByRvIdFromHB[rvInClusterMap.RvId]; found {
			lastHB, found := rvLastHB[rvHb.RvId]

			// If an RV is present in rVsByRvIdFromHB, it MUST have a valid HB in rvLastHB.
			common.Assert(found)

			// We just came here after punching the heartbeat, which indicates this is online RV.
			common.Assert(!onlyMyRVs || rvHb.State == dcache.StateOnline)

			if lastHB < hbExpiry {
				//
				// HB expired, mark RV offline if not already offline.
				//

				//
				// TODO
				// Saw this assert fire when one node died keeping clusterMap in checking state.
				// This caused updateStorageClusterMapWithMyRVs() to take a long time. It would have timed out,
				// but some other node became the leader updating the clusterMap and hence clearing thec
				// checking status, allowing updateStorageClusterMapWithMyRVs() to proceed, but by the time
				// the node's heartbeat expired.
				//
				// We just came here after punching the heartbeat, which must not expired.
				//
				common.Assert(!onlyMyRVs)

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
		} else if !onlyMyRVs {
			//
			// RV present in existingRVMap, but missing from rVsByRvIdFromHB.
			// This is not a common occurrence, emit a warning log.
			//
			// For onlyMyRV==true, case we cannot perfrom this operation as we don't have the exhaustive list of  HBs.
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

// Utility function that scans through the RV list in the given rvMap and returns the set of all nodes which
// have contributed at least one RV.
func getAllNodesFromRVMap(rvMap map[string]dcache.RawVolume) map[string]struct{} {
	nodesMap := make(map[string]struct{})

	for _, rv := range rvMap {
		nodesMap[rv.NodeId] = struct{}{}
	}

	return nodesMap
}

// For all the given NodeIds, fetch the heartbeat and return the map of RVs and map of their last heartbeat by RVId.
func collectHBForGivenNodeIds(nodeIds []string) (map[string]dcache.RawVolume, map[string]uint64, error) {
	rVsByRvIdFromHB := make(map[string]dcache.RawVolume)
	rvLastHB := make(map[string]uint64)

	for _, nodeId := range nodeIds {
		log.Debug("ClusterManager::collectHBForGivenNodeIds: Fetching heartbeat for node %s", nodeId)

		bytes, err := getHeartbeat(nodeId)
		if err != nil {
			common.Assert(false, err)
			return nil, nil, fmt.Errorf("ClusterManager::collectHBForGivenNodeIds: Failed to fetch heartbeat for node %s: %v",
				nodeId, err)
		}

		var hbData dcache.HeartbeatData
		if err := json.Unmarshal(bytes, &hbData); err != nil {
			common.Assert(false, err)
			return nil, nil, fmt.Errorf("ClusterManager::collectHBForGivenNodeIds: Failed to parse heartbeat bytes (%d) for node %s: %v",
				len(bytes), nodeId, err)
		}
		isValidHb, err := cm.IsValidHeartbeat(&hbData)
		if !isValidHb {
			common.Assert(false, err)
			return nil, nil, fmt.Errorf("ClusterManager::collectHBForGivenNodeIds: invalid heartbeart for node %s: %v",
				nodeId, err)
		}

		// Go over all RVs exported by this node, and add them to rVsByRvIdFromHB, to be processed later.
		for _, rv := range hbData.RVList {
			if _, exists := rVsByRvIdFromHB[rv.RvId]; exists {
				msg := fmt.Sprintf("Duplicate RVId %s in heartbeat for node %s (also from node %s)",
					rv.RvId, nodeId, rVsByRvIdFromHB[rv.RvId].NodeId)
				common.Assert(false, msg)
				log.Err("ClusterManager::collectHBForGivenNodeIds: %s, skipping!", msg)
				continue
			}

			rVsByRvIdFromHB[rv.RvId] = rv
			rvLastHB[rv.RvId] = hbData.LastHeartbeat
		}
	}
	return rVsByRvIdFromHB, rvLastHB, nil
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

// This function can be used to update the state of the given component RV for an MV in the global clustermap.
// The function atomically makes the requested change to the specified MV (retrying if some other node is updating
// the clustermap simultaneously) and returns only after it's able to successfully make the requested change, or
// there's some error.
//
// Only following RV state transitions are valid, for anything else it errors out.
// StateOutOfSync   -> StateSyncing     [Resync start]
// StateSyncing     -> StateOnline      [Resync end]
// StateSyncing     -> StateOutOfSync   [Resync revert]
// StateOutOfSync   -> StateOutOfSync   [Resync defer]
// StateOnline      -> StateOnline      [Resync skip good RVs]
// StateOnline      -> StateOffline     [Inband detection of offline RV during PutChunk(client)]
// StateSyncing     -> StateOffline     [Inband detection of offline RV during PutChunk(sync)]
//
// After setting the component RV state correctly in the clustermap, it calls updateMVList() which will set the
// MV state appropriately. See updateMVList() for how MV state is set based on component RVs state.
//
// Note: If this fails the caller should typically retry after sometime with the refreshed clustermap.
func (cmi *ClusterManager) updateComponentRVState(mvName string, rvName string, rvNewState dcache.StateEnum) error {
	log.Info("ClusterManager::updateComponentRVState: Set %s/%s to %s", rvName, mvName, rvNewState)

	common.Assert(cm.IsValidMVName(mvName))
	common.Assert(cm.IsValidRVName(rvName))
	common.Assert(cm.IsValidComponentRVState(rvNewState))

	startTime := time.Now()
	maxWait := 120 * time.Second

	for {
		// Time check.
		elapsed := time.Since(startTime)
		if elapsed > maxWait {
			common.Assert(false)
			return fmt.Errorf("ClusterManager::updateComponentRVState: Exceeded maxWait")
		}

		// Get most recent clustermap copy, then we will update the requested MV and publish it.
		clusterMap, etag, err := cmi.fetchAndUpdateLocalClusterMap()
		if err != nil {
			log.Err("ClusterManager::updateComponentRVState: fetchAndUpdateLocalClusterMap() failed: %v",
				err)
			common.Assert(false, err)
			return err
		}

		//
		// If some other updates are ongoing over clustermap, we need to wait and retry.
		//
		isClusterMapUpdateBlocked, err := cmi.clusterMapBeingUpdatedByAnotherNode(clusterMap, etag)
		if err != nil {
			return err
		}

		if isClusterMapUpdateBlocked {
			log.Info("ClusterManager::updateComponentRVState: Clustermap being updated by node %s, waiting a bit before retry",
				clusterMap.LastUpdatedBy)

			// TODO: Add some backoff and randomness?
			time.Sleep(10 * time.Millisecond)
			continue
		}

		// Requested MV must be valid.
		clusterMapMV, found := clusterMap.MVMap[mvName]
		if !found {
			common.Assert(false)
			return fmt.Errorf("ClusterManager::updateComponentRVState: MV %s not found in clusterMap, mvList %+v",
				mvName, clusterMap.MVMap)
		}

		// and the RV passed must be a valid component RV for that MV.
		currentState, found := clusterMapMV.RVs[rvName]
		if !found {
			return fmt.Errorf("ClusterManager::updateComponentRVState: RV %s/%s is not present in clustermap MV %v",
				rvName, mvName, clusterMapMV)
		}

		//
		// and the new state requested must be valid.
		// Note that we support only few distinct state transitions.
		//
		if currentState == dcache.StateOutOfSync && rvNewState == dcache.StateSyncing ||
			currentState == dcache.StateSyncing && rvNewState == dcache.StateOnline ||
			currentState == dcache.StateSyncing && rvNewState == dcache.StateOutOfSync ||
			currentState == dcache.StateSyncing && rvNewState == dcache.StateOffline ||
			currentState == dcache.StateOnline && rvNewState == dcache.StateOffline {

			log.Debug("ClusterManager::updateComponentRVState:  %s/%s, state change (%s -> %s)",
				rvName, mvName, currentState, rvNewState)

		} else {
			//
			// Following transitions are reported when an inband PutChunk failure suggests an RV as offline.
			// StateOnline  -> StateOffline
			// StateSyncing -> StateOffline
			//
			// Since we can have multiple PutChunk requests outstanding, all but the first one will find the
			// currentState as StateOffline, we need to ignore such updateComponentRVState() requests.
			//
			if currentState == rvNewState {
				common.Assert(currentState == dcache.StateOffline, currentState)
				log.Debug("ClusterManager::updateComponentRVState: %s/%s ignoring state change (%s -> %s)",
					rvName, mvName, currentState, rvNewState)
				return nil
			}

			common.Assert(false, rvName, mvName, currentState, rvNewState)
			return fmt.Errorf("ClusterManager::updateComponentRVState: %s/%s invalid state change request (%s -> %s)",
				rvName, mvName, currentState, rvNewState)
		}

		//
		// Ok, component RVs passed by caller match those in the clustermap and all RV state change requested
		// are valid. Update component RVs and call updateMVList() which will set the MV state correctly, and
		// run various mv workflows as needed after the current RV state change.
		//
		//
		// Claim ownership of clustermap.
		// If some other node gets there before us, we retry. Note that we don't add a wait before
		// the retry as that other node is not updating the clustermap, it's done updating.
		//
		// TODO: Check err to see if the failure is due to etag mismatch, if not retrying may not help.
		//
		err = cmi.startClusterMapUpdate(clusterMap, etag)
		if err != nil {
			log.Warn("ClusterManager::updateComponentRVState: Start Clustermap update failed for nodeId %s: %v, retrying",
				cmi.myNodeId, err)
			continue
		}

		//
		// Update requested Mv in the cluster Map.
		// MV state is not important as it'll be correctly set by updateMVList().
		// We force it to a StateOffline to catch any bug in setting the MV state correctly.
		//
		clusterMapMV.State = dcache.StateOffline
		clusterMapMV.RVs[rvName] = rvNewState
		clusterMap.MVMap[mvName] = clusterMapMV

		//
		// TODO: For now we treat component RV being flagged as offline no different from the RV being flagged
		//       offline by cm.ReportRVOffline(). Note that there could be some differences, f.e., component
		//       RV may be flagged offline on just one inband failure, while when we report an RV as offline
		//       we have to be really sure. In some error cases, like connection getting reset or read returning
		//       eof, one failure might be sufficient to correctly claim RV as offline but for error like
		//       timeout we cannot be really sure and we might want to play safe.
		//
		//       If we don't do this we will have unwanted side effects, f.e., if a component RV is marked
		//       offline but the RV is online in the RV list, then updateMVList()->fixMV() might pick the same
		//       RV as a replacement RV, which would be wrong as RV, for all purposes, is offline.
		//
		if rvNewState == dcache.StateOffline {
			rv := clusterMap.RVMap[rvName]
			if rv.State != dcache.StateOffline {
				log.Warn("ClusterManager::updateComponentRVState: Marking RV %s state (%s -> %s)",
					rvName, rv.State, dcache.StateOffline)

				rv.State = dcache.StateOffline
				clusterMap.RVMap[rvName] = rv
			}
		}

		// Call updateMVList() to update MV state and run the various mv workflows.
		cmi.updateMVList(clusterMap.RVMap, clusterMap.MVMap)

		err = cmi.endClusterMapUpdate(clusterMap)
		if err != nil {
			log.Err("ClusterManager::updateComponentRVState: endClusterMapUpdate() failed: %v %+v",
				err, clusterMap)
			common.Assert(false, err)
			return err
		}

		// The clustermap must now have update RV states in MV.
		log.Info("ClusterManager::updateComponentRVState: clustermap MV is updated by %s at %d %+v",
			cmi.myNodeId, clusterMap.LastUpdatedAt, clusterMapMV)

		break
	}

	// Update local copy.
	_, _, err := cmi.fetchAndUpdateLocalClusterMap()
	return err
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

	// Register hook for updating the component RV state for an MV, through clustermap package.
	cm.RegisterComponentRVStateUpdater(clusterManager.updateComponentRVState)

	// Register hook for refreshing the clustermap from the metadata store, through clustermap package.
	cm.RegisterClusterMapRefresher(clusterManager.updateClusterMapLocalCopy)

	return clusterManager.start(dCacheConfig, rvs)
}

func Stop() error {
	common.Assert(clusterManager != nil, "ClusterManager not started")
	return clusterManager.stop()
}
