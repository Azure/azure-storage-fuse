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
	"slices"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"

	"github.com/Azure/azure-storage-fuse/v2/internal/dcache"
	cm "github.com/Azure/azure-storage-fuse/v2/internal/dcache/clustermap"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache/debug/stats"
	mm "github.com/Azure/azure-storage-fuse/v2/internal/dcache/metadata_manager"
	rm "github.com/Azure/azure-storage-fuse/v2/internal/dcache/replication_manager"
	rpc_client "github.com/Azure/azure-storage-fuse/v2/internal/dcache/rpc/client"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache/rpc/gen-go/dcache/models"
	rpc_server "github.com/Azure/azure-storage-fuse/v2/internal/dcache/rpc/server"
)

//go:generate $ASSERT_REMOVER $GOFILE

// Cluster manager's job is twofold:
//  1. Keep the global clustermap up-to-date. One of the nodes takes up the role of the leader who periodically
//     gather information about all nodes/RVs from the heartbeats and updates the clustermap according to that.
//     It then publishes this clustermap which others download (see point #2 below).
//  2. Maintain a local clustermap copy which is used by clustermap package to respond to various queries
//     by clustermap package users. Every node downloads the global clustermap and save it as this local copy.
//
// This is the singleton cluster manager struct that holds the state of the cluster manager.
type ClusterManager struct {
	myNodeId    string
	myHostName  string
	myIPAddress string
	config      *dcache.DCacheConfig

	hbTicker     *time.Ticker
	hbTickerDone chan bool

	clusterMapTicker     *time.Ticker
	clusterMapTickerDone chan bool

	componentRVStateBatchUpdateTicker     *time.Ticker
	componentRVStateBatchUpdateTickerDone chan bool

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

	// Wait group to wait for the goroutines spawned, before stopping the cluster manager.
	wg sync.WaitGroup
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

	stats.Stats.NodeId = cmi.myNodeId
	stats.Stats.IPAddr = cmi.myIPAddress
	stats.Stats.HostName = cmi.myHostName

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
	log.Info("ClusterManager::start: ==> Ensuring initial cluster map with my RVs %+v", rvs)

	startTime := time.Now()
	err = cmi.ensureInitialClusterMap(dCacheConfig, rvs)
	if err != nil {
		return err
	}
	stats.Stats.CM.Startup.EnsureInitialClustermapDuration = stats.Duration(time.Since(startTime))

	//
	// It's unlikely, but due to some misconfiguration all of my RVs may not have been added to clustermap,
	// we should heartbeat only those that got added and also start the RPC server only for those RVs.
	//
	rvsMap := getMyRVsInClustermap(rvs)
	hbRVs := make([]dcache.RawVolume, 0, len(rvsMap))

	// map[string]dcache.RawVolume to []dcache.RawVolume, shedding the RV names.
	for _, rv := range rvsMap {
		hbRVs = append(hbRVs, rv)
	}

	//
	// clustermap MUST now have the in-core clustermap copy.
	// We call the cm.GetCacheConfig() below to validate that.
	//
	if len(hbRVs) == len(rvs) {
		// Common case.
		log.Info("ClusterManager::start: ==> Cluster map now ready with my %d RV(s) %+v, config: %+v",
			len(rvs), hbRVs, *cm.GetCacheConfig())
	} else if len(hbRVs) > 0 {
		log.Warn("ClusterManager::start: ==> Cluster map now ready, but only using %d [%+v] of %d [%+v] RV(s), config: %+v",
			len(hbRVs), hbRVs, len(rvs), rvs, *cm.GetCacheConfig())
	} else {
		//
		// Even though this node is not contributing any RVs to the cluster, we still add it to allow
		// it to access the cluster storage. If many nodes are not contributing any RVs, it becomes an
		// issue, since the cluster will not have enough RVs to store data.
		//
		// TODO: Shall we block such nodes from joining the cluster?
		//
		log.Warn("ClusterManager::start: ==> Cluster map now ready, but cannot use any of my RVs %+v, config: %+v",
			rvs, *cm.GetCacheConfig())
	}

	//
	// Rest of the code works on rvs, so update that.
	//
	rvs = hbRVs

	//
	// Now we should have a valid local clustermap with all/some/none of our RVs present in the RV list.
	//
	// Now we can start the RPC server, but only if we have some RVs to export.
	//
	// Note: Since ensureInitialClusterMap() would send the heartbeat and make the cluster aware of this
	//       node, it's possible that some other cluster node runs the new-mv workflow and sends a JoinMV
	//       RPC request to this node, before we can start the RPC server. We add resiliency for this
	//       by retrying JoinMV RPC after a small wait if RPC connection creation fails.
	//       Ref functions.go:JoinMV() for more details.
	//
	if len(rvsMap) > 0 {
		log.Info("ClusterManager::start: ==> Starting RPC server")

		common.Assert(cmi.rpcServer == nil)
		cmi.rpcServer, err = rpc_server.NewNodeServer(rvsMap)
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

		log.Info("ClusterManager::start: ==> Started RPC server on node %s IP %s", cmi.myNodeId, cmi.myIPAddress)
	} else {
		// No RVs, no RPC server.
		log.Warn("ClusterManager::start: ==> No RVs exported, not starting RPC server")
	}

	// We don't intend to have different configs in different nodes, so assert.
	common.Assert(dCacheConfig.HeartbeatSeconds == cmi.config.HeartbeatSeconds,
		"Local config HeartbeatSeconds different from global config",
		dCacheConfig.HeartbeatSeconds, cmi.config.HeartbeatSeconds)

	const maxConsecutiveFailures = 3

	//
	// No RVs, no periodic heartbeat.
	//
	if len(rvs) > 0 {
		cmi.hbTicker = time.NewTicker(time.Duration(cmi.config.HeartbeatSeconds) * time.Second)
		cmi.hbTickerDone = make(chan bool)

		cmi.wg.Add(1)

		go func() {
			defer cmi.wg.Done()

			var err error
			var consecutiveFailures int

			for {
				select {
				case <-cmi.hbTickerDone:
					log.Info("ClusterManager::start: Scheduled task \"Punch Heartbeat\" stopped")
					return
				case <-cmi.hbTicker.C:
					log.Debug("ClusterManager::start: Scheduled task \"Punch Heartbeat\" triggered")

					err = cmi.punchHeartBeat(rvs, false /* initialHB */)
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
							log.GetLoggerObj().Panicf("[PANIC] Failed to update heartbeat %d times in a row",
								consecutiveFailures)
						}
					}
				}
			}
		}()
	}

	// We don't intend to have different configs in different nodes, so assert.
	common.Assert(dCacheConfig.ClustermapEpoch == cmi.config.ClustermapEpoch,
		"Local config ClustermapEpoch different from global config",
		dCacheConfig.ClustermapEpoch, cmi.config.ClustermapEpoch)

	cmi.clusterMapTicker = time.NewTicker(time.Duration(cmi.config.ClustermapEpoch) * time.Second)
	cmi.clusterMapTickerDone = make(chan bool)

	cmi.wg.Add(1)

	go func() {
		defer cmi.wg.Done()

		var err error
		var consecutiveFailures int

		//
		// TODO: Test it and make sure it doesn't call updateStorageClusterMapIfRequired() back2back
		//       in case the prev call to updateStorageClusterMapIfRequired() took long time, causing
		//       ticks to accumulate. There's no point in calling updateStorageClusterMapIfRequired()
		//       b2b. The doc says that ticker will adjust and drop ticks for slow receivers, but we
		//       need to verify and if required, drop ticks which are long back in the past.
		//
		for {
			select {
			case <-cmi.clusterMapTickerDone:
				log.Info("ClusterManager::start: Scheduled task \"Update ClusterMap\" stopped")
				return
			case <-cmi.clusterMapTicker.C:
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
		}
	}()

	//
	// Start the batch component RV update ticker.
	// It batches component RV updates every 2 secs.
	//
	cmi.componentRVStateBatchUpdateTicker = time.NewTicker(2 * time.Second)
	cmi.componentRVStateBatchUpdateTickerDone = make(chan bool)

	cmi.wg.Add(1)

	go func() {
		defer cmi.wg.Done()

		for {
			select {
			case <-cmi.componentRVStateBatchUpdateTickerDone:
				log.Info("ClusterManager::start: Scheduled task \"ComponentRV Batch Update\" stopped")
				return
			case <-cmi.componentRVStateBatchUpdateTicker.C:
				log.Info("ClusterManager::start: Scheduled task \"ComponentRV Batch Update\" triggered")

				//
				// Get the next batch of component RV state updates.
				// Two updates to the same mv/rv will not be in the same batch.
				// Keep processing the batches until there are no more updates to process, we do not want to
				// unnecessarily delay the updates.
				//
				for {
					msgBatch := cmi.getNextComponentRVUpdateBatch()
					if len(msgBatch) > 0 {
						err := cmi.batchUpdateComponentRVState(msgBatch)
						if err != nil {
							log.Err("ClusterManager::start: batchUpdateComponentRVState failed: %v", err)
						}

						//
						// Status of the combined update is the status of each individual update.
						// Note that it's only for the updates which were actually included in the global update.
						// Some of the individual updates which were not included in the global update, would
						// be already completed individually, skip those.
						//
						for _, msg := range msgBatch {
							if msg.Err != nil {
								msg.Err <- err
								close(msg.Err)
							}
						}
					} else {
						log.Debug("ClusterManager::start: batchUpdateComponentRVState: No updates to process")
						break
					}
				}
			}
		}
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
	atomic.AddInt64(&stats.Stats.CM.LocalClustermap.TimesUpdated, 1)
	//
	// 1. Fetch the latest clustermap from metadata store.
	//
	start := time.Now()
	storageBytes, etag, err := getClusterMap()
	if err != nil {
		err1 := fmt.Errorf("failed to fetch clustermap on node %s: %v", cmi.myNodeId, err)
		log.Err("ClusterManager::fetchAndUpdateLocalClusterMap: %v", err1)

		common.Assert(len(storageBytes) == 0)
		common.Assert(etag == nil)
		//
		// Only when called from safeCleanupMyRVs(), we may not have the global clustermap yet.
		// Post that, once cmi.config is set, it should never fail.
		//
		common.Assert(cmi.config == nil, err1)
		// ENOENT is the only viable error, for everything else we retry.
		common.Assert(err == syscall.ENOENT, err)

		atomic.AddInt64(&stats.Stats.CM.LocalClustermap.UpdateFailures, 1)
		stats.Stats.CM.LocalClustermap.LastError = err1.Error()
		return nil, nil, err
	}

	updateDuration := stats.Duration(time.Since(start))

	if len(storageBytes) == 0 {
		err = fmt.Errorf("received empty clustermap on node %s", cmi.myNodeId)
		log.Err("ClusterManager::fetchAndUpdateLocalClusterMap: %v", err)
		common.Assert(false, err)
		atomic.AddInt64(&stats.Stats.CM.LocalClustermap.UpdateFailures, 1)
		stats.Stats.CM.LocalClustermap.LastError = err.Error()
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
		err = fmt.Errorf("failed to unmarshal clustermap json on node %s: %v", cmi.myNodeId, err)
		log.Err("ClusterManager::fetchAndUpdateLocalClusterMap: %v", err)
		common.Assert(false, err)
		atomic.AddInt64(&stats.Stats.CM.LocalClustermap.UpdateFailures, 1)
		stats.Stats.CM.LocalClustermap.LastError = err.Error()
		return nil, nil, err
	}

	common.Assert(cm.IsValidClusterMap(&storageClusterMap))

	atomic.StoreInt64(&stats.Stats.CM.LocalClustermap.SizeInBytes, int64(len(storageBytes)))
	atomic.StoreInt64(&stats.Stats.CM.LocalClustermap.Epoch, int64(storageClusterMap.Epoch))
	stats.Stats.CM.StorageClustermap.Leader = storageClusterMap.LastUpdatedBy

	atomic.AddInt64((*int64)(&stats.Stats.CM.LocalClustermap.TotalTime), int64(updateDuration))
	if stats.Stats.CM.LocalClustermap.MinTime == nil ||
		updateDuration < *stats.Stats.CM.LocalClustermap.MinTime {
		stats.Stats.CM.LocalClustermap.MinTime = &updateDuration
	}
	stats.Stats.CM.LocalClustermap.MaxTime =
		max(stats.Stats.CM.LocalClustermap.MaxTime, updateDuration)

	cmi.localMapLock.Lock()
	defer cmi.localMapLock.Unlock()

	//
	// 3. If we've already loaded this exact version, skip the local update.
	//
	if cmi.localMapETag != nil && *etag == *cmi.localMapETag {
		log.Debug("ClusterManager::fetchAndUpdateLocalClusterMap: ETag (%s) unchanged, not updating local clustermap",
			*etag)
		// Cache config must have been saved when we saved the clustermap.
		common.Assert(cmi.config != nil)
		common.Assert(cm.IsValidDcacheConfig(cmi.config))
		atomic.AddInt64(&stats.Stats.CM.LocalClustermap.Unchanged, 1)
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
		atomic.AddInt64(&stats.Stats.CM.LocalClustermap.UpdateFailures, 1)
		stats.Stats.CM.LocalClustermap.LastError = err.Error()
		return nil, nil, err
	} else if err := os.Rename(tmp, cmi.localClusterMapPath); err != nil {
		err = fmt.Errorf("Rename(%s -> %s) failed: %v %+v",
			tmp, cmi.localClusterMapPath, err, storageClusterMap)
		log.Err("ClusterManager::fetchAndUpdateLocalClusterMap: %v", err)
		common.Assert(false, err)
		atomic.AddInt64(&stats.Stats.CM.LocalClustermap.UpdateFailures, 1)
		stats.Stats.CM.LocalClustermap.LastError = err.Error()
		return nil, nil, err
	}

	//
	// 5. Update in-memory tag.
	//
	cmi.localMapETag = etag

	stats.Stats.CM.LocalClustermap.LastUpdated = time.Now()

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
		cmi.hbTickerDone <- true
	}

	// TODO{Akku}: Delete the heartbeat file
	// mm.DeleteHeartbeat(cmi.myNodeId)
	if cmi.clusterMapTicker != nil {
		cmi.clusterMapTicker.Stop()
		cmi.clusterMapTickerDone <- true
	}

	if cmi.componentRVStateBatchUpdateTicker != nil {
		cmi.componentRVStateBatchUpdateTicker.Stop()
		cmi.componentRVStateBatchUpdateTickerDone <- true
	}

	cm.Stop()
	cmi.wg.Wait()

	return nil
}

// This function checks the local clustermap to see how many of myRVs are present in the RV list
// and returns a list of those RVs with their RV names as seen in the clustermap.
// It compares all the fields of the RVs, not just the RV Id. This is important to ensure that
// we do not treat a stale RV Id as a match. This also means that this function can only be used
// during startup when the RV AvailableSpace has not changed in clustermap from what it was in the
// initial myRVs list created from data in the config.
func getMyRVsInClustermap(myRVs []dcache.RawVolume) map[string]dcache.RawVolume {
	// Must be passed with a valid non-empty RV list.
	common.Assert(len(myRVs) > 0)

	// My RVs which actually are in the clustermap (along with their names in the clustermap).
	cmRVs := make(map[string]dcache.RawVolume)

	//
	// Fetch all RVs owned by this node from the clustermap.
	// Some of them could be outside myRVs.
	//
	myRVsFromClustermap := cm.GetMyRVs()
	if len(myRVsFromClustermap) == 0 {
		return cmRVs
	}

	for _, myRv := range myRVs {
		for rvName, rv := range myRVsFromClustermap {
			if myRv == rv {
				cmRVs[rvName] = rv
				break
			}
		}
	}

	common.Assert(len(cmRVs) <= len(myRVs), cmRVs, myRVs, myRVsFromClustermap)

	if len(cmRVs) == len(myRVs) {
		return cmRVs
	} else if len(cmRVs) > 0 {
		//
		// This can happen if a node was previously part of the cluster and then it left and joined
		// again, but this time with a different set of RVs but with at least one common RV.
		//
		log.Warn("ClusterManager::getMyRVsInClustermap: Found %d of my %d RV(s) in clustermap, not all RVs present, found: %+v, expected: %+v, clustermap RVs: %+v",
			len(cmRVs), len(myRVs), cmRVs, myRVs, myRVsFromClustermap)
	}

	return cmRVs
}

// Cleanup the given RV's directory. If doNotDeleteMVs is nil, all MVs are deleted else those MVs
// are skipped and everything else is deleted.
// It returns failure if it fails to delete even a single matching MV. This is to ensure that
// we prevent such a node from joining the cluster.
//
// TODO: Once we have sufficient run-in we can let it join the cluster even on partial cleanup.
func cleanupRV(rv dcache.RawVolume, doNotDeleteMVs map[string]struct{}) error {
	var wg sync.WaitGroup
	var deleteSuccess atomic.Int64
	var deleteFailures atomic.Int64

	// More than a few parallel deletes may be counter productive.
	const maxParallelDeletes = 8
	var tokens = make(chan struct{}, maxParallelDeletes)

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

		mvName := entry.Name()
		if !cm.IsValidMVName(mvName) {
			common.Assert(false, rv.LocalCachePath, mvName, entry)
			return fmt.Errorf("ClusterManager::cleanupRV %s/%s is not a valid MV directory %+v",
				rv.LocalCachePath, mvName, entry)
		}

		//
		// If user wants some/active MVs to be skipped, honor that.
		//
		if doNotDeleteMVs != nil {
			if _, ok := doNotDeleteMVs[mvName]; ok {
				log.Debug("ClusterManager::cleanupRV: Not deleting active MV directory %s/%s",
					rv.LocalCachePath, mvName)
				continue
			}
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
				log.Info("ClusterManager::cleanupRV: Deleted MV dir %s", dir)
				deleteSuccess.Add(1)
			}
		}(filepath.Join(rv.LocalCachePath, mvName))
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

	atomic.AddInt64(&stats.Stats.CM.Startup.MVsDeleted, deleteSuccess.Load())
	atomic.AddInt64(&stats.Stats.CM.Startup.MVsDeleteFailed, deleteFailures.Load())

	log.Info("ClusterManager::cleanupRV: Successfully cleaned up RV dir %s, deleted %d MV(s)",
		rv.LocalCachePath, deleteSuccess.Load())

	return nil
}

// When a node comes up and before it joins the cluster by posting its initial heartbeat, it's the right time
// to cleanup any stale MV data that may have been left behind by the previous incarnation of the node.
// Previous versions used to play safe and delete all hosted MVs' data on all local RVs (after waiting for the RVs
// to be marked offline), but that was too aggressive and caused unnecessary data to be deleted. If a node crashes
// and restarts immediately (typical when the blobfuse process crashes and restarts) it would result in all hosted
// MVs' data to be deleted. This may risk cluster stability if multiple nodes happen to crash and restart around
// the same time. Also, this would cause unnecessary data movement even for MVs which have not changed between
// restarts.
//
// With the following observations we can do better:
// - While this node was down if a hosted MV is written to by some node, the PutChunk RPC would fail since the
//   node is down. This will cause the client node to mark the component RV as inband-offline and from then on
//   this RV won't be used for storing or accessing chunks of that MV. The MV's data can be safely deleted from
//   the RV directory.
// - While this node was down if a hosted MV was not accessed by any other node, it would continue to be online
//   in the MV's component RVs list and since the MV data has not changed, once this node comes back up it can
//   correctly serve that MV. We do not need to delete the MV's data in this case.
// - If the node stays down for long enough and misses sufficient heartbeats, its RV(s) will be marked offline in
//   the clustermap and in all the MVs' component RVs lists. This means that no MV will try to access this RV for
//   chunk IO (read or write), and in that case we can safely delete all the MVs' hosted on the node's RV(s).
//
// This helps protect data for cases where a node goes down for a short time and then comes back up.
// This especially helps in cases where a node is restarted due to some transient issue, like a crash or some
// accidental restart.
//
// So here's the plan:
// For all RVs of this node, only delete those hosted MVs' data for which the RV is marked offline/inband-offline
// in the clustermap. If there's no clustermap or if the RV itself is offline, delete all MVs' data in the RVs.
//
// Before proceeding, ensure no duplicate RV IDs (filesystem GUIDs) exist across different cache paths,
// i.e., if an RV ID appears in both the input list and clustermap, it must refer to the *same* path.
// Any mismatch indicates a RVId collision, in that case, the startup will be aborted.
//
// In case of success the boolean return value indicates the following:
// true  -> Found a clustermap and RV(s) were either not present in the RV list or waited for RV(s) to be marked
//          offline and then cleaned up the RVs if any.
// false -> Did not find clustermap, cleaned up the RVs if any.
//
// In case of success it'll return only after all RV directories are appropriately cleaned up.
// If it finds any unexpected file/dir in the RV it complains and bails out. Note that this is the only place where
// we check if RV contains any unexpected file/dir.

func (cmi *ClusterManager) safeCleanupMyRVs(myRVs []dcache.RawVolume) (bool, error) {
	log.Info("ClusterManager::safeCleanupMyRVs: ==> Cleaning up %d RV(s) %v", len(myRVs), myRVs)

	var wg sync.WaitGroup
	var failedRV atomic.Int64

	start := time.Now()
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

				err := cleanupRV(rv, nil /* doNotDeleteMVs */)
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
		log.Info("ClusterManager::safeCleanupMyRVs: ==> Successfully cleaned up %d RV(s) %v in %s",
			len(myRVs), myRVs, time.Since(start))

		return nil
	}

	//
	// Fetch clustermap and update the local copy.
	// Once this succeeds, clustermap APIs can be used for querying clustermap.
	//
	_, _, err := cmi.fetchAndUpdateLocalClusterMap()
	if err != nil {
		//
		// fetchAndUpdateLocalClusterMap() returns the raw error syscall.ENOENT when it cannot find
		// the clustermap in the metadata store.
		//
		isClusterMapExists := (err != syscall.ENOENT)

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
	// For all of our RVs that we will be adding to the cluster, ensure:
	// - No other node has an RV with the same RVid.
	// - There isn't a new RV being added which has RVid matching one of our existing RVs but a different
	//   cache path. If cache path and RVid are same for an existing and new RV, it's the same RV being
	//   added, this is the common case of a node rejoining the cluster w/o any change in RVs. updateRVList()
	//   will let this RV continue to be used as the existing RV name.
	// The bottomline is that we don't want two RVs with same RVid.
	//
	// Note that this only checks for duplicates against the RVs already present in the clustermap,
	// it cannot check for duplicates against RVs which are being added by multiple nodes that are
	// starting up at the same time. Those are checked in collectHBForGivenNodeIds().
	// Also some other node may add a duplicate RV into the clustermap after the following check,
	// hence we need to later check for duplicates after locking the clustermap.
	//
	allRVIdsFromClustermap := cm.GetAllRVsById()
	for _, myRV := range myRVs {
		common.Assert(myRV.NodeId == cmi.myNodeId, cmi.myNodeId, myRV)

		cmRV, ok := allRVIdsFromClustermap[myRV.RvId]
		if !ok {
			// This RVId is not present in the clustermap, so cannot have a duplicate.
			continue
		}

		//
		// 1st check: Some other node has an RV with the same RVid, clear duplicate.
		// 2nd check: We had an RV with the same RVid but a different cache-dir.
		//            This is not allowed, since it can cause confusion in the cluster.
		//            If the user wants to reuse an existing drive with a different cache-dir,
		//            they need to get a brand new RVId for that drive.
		//
		if myRV.NodeId != cmRV.NodeId || myRV.LocalCachePath != cmRV.LocalCachePath {
			return false, fmt.Errorf(
				"ClusterManager::safeCleanupMyRVs: Duplicate RVid %s detected, cache-dir %s being added by this node %s has the same RVid as existing cache-dir %s from node %s",
				myRV.RvId, myRV.LocalCachePath, myRV.NodeId, cmRV.LocalCachePath, cmRV.NodeId)
		}
	}

	//
	// Find which all of my RVs are present in the clustermap.
	// For those RV Ids which are not present in the clustermap delete all the MV directories, else only cleanup
	// non-active MV directories for those myRVs which are present in the clustermap.
	//
	myRvIdToName := cm.MyRvIdToNameMap()

	if len(myRvIdToName) > 0 {
		log.Info("ClusterManager::safeCleanupMyRVs: %d of my RV(s) are already present in clustermap %+v",
			len(myRvIdToName), myRvIdToName)
	} else {
		log.Info("ClusterManager::safeCleanupMyRVs: No my RV(s) in clustermap, will delete all MVs in all my RVs")
	}

	var doNotDeleteMVs map[string]struct{}
	for _, rv := range myRVs {
		log.Debug("ClusterManager::safeCleanupMyRVs: Checking my RV %+v", rv)

		//
		// Check if this my RVId is present in the clustermap.
		// If yes, we need to avoid deleting any active MVs for this RV.
		//
		rvName, ok := myRvIdToName[rv.RvId]
		if !ok {
			log.Info("ClusterManager::safeCleanupMyRVs: My RV %s doesn't exist in clustermap", rv.RvId)
		} else {
			log.Info("ClusterManager::safeCleanupMyRVs: My RV %s is present as %s in clustermap", rv.RvId, rvName)

			// Active MVs (for which this RV is online component RV), that we should not delete.
			doNotDeleteMVs = cm.GetActiveMVsForRV(rvName)
			if len(doNotDeleteMVs) > 0 {
				log.Debug("ClusterManager::safeCleanupMyRVs: %s has %d active MVs %+v, will not delete them",
					rvName, len(doNotDeleteMVs), doNotDeleteMVs)
			}
		}

		// Cleanup stale MVs from this RV.
		wg.Add(1)
		go func(rv dcache.RawVolume, doNotDeleteMVs map[string]struct{}) {
			defer wg.Done()

			err := cleanupRV(rv, doNotDeleteMVs)
			if err != nil {
				log.Err("ClusterManager::safeCleanupMyRVs: cleanupRV (%s) failed: %v",
					rv.LocalCachePath, err)
				failedRV.Add(1)
			}
		}(rv, doNotDeleteMVs)
	}

	// Wait for all RVs to complete cleanup.
	wg.Wait()

	if failedRV.Load() != 0 {
		return false, fmt.Errorf("ClusterManager::safeCleanupMyRVs: Failed to cleanup %d RV(s)",
			failedRV.Load())
	}

	log.Info("ClusterManager::safeCleanupMyRVs: ==> Successfully cleaned up %d RV(s) %v in %s",
		len(myRVs), myRVs, time.Since(start))
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
	startTime := time.Now()
	isClusterMapExists, err := cmi.safeCleanupMyRVs(rvs)
	if err != nil {
		log.Err("ClusterManager::ensureInitialClusterMap: Failed to check clustermap: %v", err)
		common.Assert(false)
		return err
	}
	stats.Stats.CM.Startup.RVCleanupDuration = stats.Duration(time.Since(startTime))

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
		Epoch:         0,
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

	log.Info("ClusterManager::ensureInitialClusterMap: ==> Initial clusterMap created successfully (or there already was a clustermap): %+v",
		clusterMap)

	//
	// Now we have the initial clustermap. Our next task is to update the clustermap with our local RVs and
	// finally save a local copy of the updated clustermap.
	//
	// safeCleanupMyRVs() would have waited for any of our RVs present in clustermap to be marked offline,
	// so it's safe to announce our presence by punching the first heartbeat.
	//
UpdateLocalClusterMapAndPunchInitialHeartbeat:

	//
	// Ensure our RVs are added to the clustermap. If already present in the clustermap, this will be a no-op,
	// else it updates the clustermap with our local RVs added to the RV list.
	// This also punches the initial heartbeat.
	//
	startTime = time.Now()
	err = cmi.updateStorageClusterMapWithMyRVs(rvs)
	if err != nil {
		log.Err("ClusterManager::ensureInitialClusterMap: updateStorageClusterMapWithMyRVs failed: %v %+v",
			err, clusterMap)
		common.Assert(false, err)
		return err
	}
	stats.Stats.CM.Startup.UpdateClustermapWithMyRVsDuration = stats.Duration(time.Since(startTime))

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
func (cmi *ClusterManager) updateStorageClusterMapWithMyRVs(myRVs []dcache.RawVolume) error {
	startTime := time.Now()
	maxWait := 120 * time.Second

	//
	// Punch the initial heartbeat before proceeding to update the clusterMap with my RVs.
	// This allows one node (that successfully claims ownership of the clusterMap) to add RVs
	// for many nodes that have punched their initial heartbeats. This helps improve cluster startup
	// dramatically, else if a node only adds its RVs to clusterMap then the clusterMap needs to be
	// updated many times, each time requiring one node to claim ownership of the clusterMap.
	//
	err := cmi.punchHeartBeat(myRVs, true /* initialHB */)
	if err != nil {
		log.Err("ClusterManager::updateStorageClusterMapWithMyRVs: Initial punchHeartBeat failed: %v", err)
		common.Assert(false, err)
		return err
	}

	log.Info("ClusterManager::updateStorageClusterMapWithMyRVs: ==> Initial Heartbeat punched, now adding %d my RV(s): %+v",
		len(myRVs), myRVs)

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
			// When updateStorageClusterMapWithMyRVs() is called, clustermap must be present.
			common.Assert(err != syscall.ENOENT, err)
			common.Assert(false, err)
			return err
		}

		//
		// Since we punched our initial heartbeat, it's possible that some other node may have added
		// our RVs to the clustermap, if so, we are done, else we need to add our RVs to the clustermap.
		// Note that we let one node add RVs for all the nodes that have punched their initial heartbeats
		// in close proximity.
		// This helps avoid multiple nodes trying to update the clustermap at the same time, which can lead
		// to really long delays in cluster startup, as each node has to get exclusive ownership of the
		// clustermap.
		//
		if len(getMyRVsInClustermap(myRVs)) == len(myRVs) {
			//
			// The clustermap has all our RVs in the RV list added by some other node, thank it and
			// proceed.
			//
			log.Info("[TIMING] ClusterManager::updateStorageClusterMapWithMyRVs: All (%d) myRVs added by node %s (took %s): %+v",
				len(myRVs), clusterMap.LastUpdatedBy, time.Since(startTime), clusterMap)
			return nil
		}

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
		// Ok, no other node has added our RVs to the clustermap, we need to add them now.
		//

		//
		// Claim ownership of clustermap and add our RVs.
		// If some other node gets there before us, we retry.
		//
		// Note: We retry even if the failure is not due to etag mismatch, hoping the error to be transient.
		//       Anyways, we have a timeout.
		//
		err = cmi.startClusterMapUpdate(clusterMap, etag)
		if err != nil {
			log.Warn("ClusterManager::updateStorageClusterMapWithMyRVs: Start Clustermap update failed for nodeId %s: %v, retrying",
				cmi.myNodeId, err)

			// Allow some time for the other node to finish updating the clustermap.
			time.Sleep(1 * time.Second)
			continue
		}

		//
		// Add our RVs and any other nodes' RVs which are simultaneously starting up.
		// Note that we pass initialHB as true so that we only look at initial heartbeats, that's
		// what the new nodes have posted.
		//
		_, err = cmi.updateRVList(clusterMap.RVMap, true /* initialHB */)
		if err != nil {
			log.Err("ClusterManager::updateStorageClusterMapWithMyRVs: updateRVList() failed: %v", err)
			//
			// Best effort end clusterMap update. If this fails other nodes will be able to claim ownership
			// albeit after some timeout period.
			//
			cmi.endClusterMapUpdate(clusterMap)
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

		//
		// The clustermap must now have our RVs added to RV list.
		// Only RVs which clash with existing RVs in the clustermap would not be added.
		//
		log.Info("[TIMING] ClusterManager::updateStorageClusterMapWithMyRVs: cluster map updated by %s at %d (took %s)",
			cmi.myNodeId, clusterMap.LastUpdatedAt, time.Since(startTime))

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

	//
	// Update the latest clustermap content and etag after successfully overriding the ownership.
	// We do this to let the caller know about the updated etag. Note that the above update to clustermap
	// would have changed the etag, if we don't update caller's etag when he tries the startClusterMapUpdate()
	// after we return, it'll fail with etag mismatch.
	//
	latestClusterMap, latestEtag, err := clusterManager.fetchAndUpdateLocalClusterMap()
	if err != nil {
		common.Assert(false, err)
		return true, fmt.Errorf("ClusterManager::clusterMapBeingUpdatedByAnotherNode: fetchAndUpdateLocalClusterMap() failed: %v", err)
	}

	log.Info("ClusterManager::clusterMapBeingUpdatedByAnotherNode: Successfully overrode stuck/stale clustermap, prev etag: %v, new etag: %v", *etag, *latestEtag)

	//
	// Update the etag and clusterMap references to the latest values.
	// etag must have changed (since we wrote the clusterMap above).
	//
	common.Assert(*etag != *latestEtag, *etag)
	*etag = *latestEtag
	*clusterMap = *latestClusterMap

	//
	// Between the endClusterMapUpdate() and fetchAndUpdateLocalClusterMap(), the clusterMap is in ready state,
	// so some other node can start updating it. Our callers will call startClusterMapUpdate() with this etag
	// that we return, hoping it corresponds to a StateReady clusterMap, we don't want them to be overwriting
	// a clusterMap being updated by some other node.
	// This is a very small window, hence emit an info log.
	//
	if clusterMap.State == dcache.StateChecking {
		age = time.Since(time.Unix(clusterMap.LastUpdatedAt, 0))
		log.Info("ClusterManager::clusterMapBeingUpdatedByAnotherNode: Node (%s), started updating clusterMap (%s ago)",
			clusterMap.LastUpdatedBy, age)
		return true, nil
	}

	//
	// This is our promise to the caller.
	//
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
	isLeaderChanged := clusterMap.LastUpdatedBy != cmi.myNodeId
	clusterMap.LastUpdatedBy = cmi.myNodeId
	//
	// Set the LastUpdatedAt. This is useful as clusterMapBeingUpdatedByAnotherNode() uses that to log
	// how long has the clusterMap been in "checking" state.
	// Later endClusterMapUpdate() will punch the final time when the clusterMap update finished.
	//
	clusterMap.LastUpdatedAt = time.Now().Unix()
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

	//
	// This node is the new leader.
	//
	if isLeaderChanged {
		stats.Stats.CM.StorageClustermap.BecameLeaderAt = time.Now()
		atomic.AddInt64((*int64)(&stats.Stats.CM.StorageClustermap.LeaderSwitches), 1)
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

	//
	// Every time clusterMap is updated, Epoch is incremented.
	// This is good for usecases which want to find out if after refresh they got a "new" clusterMap copy, not
	// necessarily "updated" clusterMap copy. Most usecases should be fine with this.
	//
	// TODO: See if it's useful to update Epoch only on clusterMap content change.
	//
	clusterMap.Epoch++

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

// Publishes the heartbeat for this node.
// initialHB indicates if this is the initial heartbeat for this node.
func (cmi *ClusterManager) punchHeartBeat(myRVs []dcache.RawVolume, initialHB bool) error {
	// Refresh AvailableSpace for my RVs, before publishing in the heartbeat.
	refreshMyRVs(myRVs)

	hbData := dcache.HeartbeatData{
		InitialHB:     initialHB,
		IPAddr:        cmi.myIPAddress,
		NodeID:        cmi.myNodeId,
		Hostname:      cmi.myHostName,
		LastHeartbeat: uint64(time.Now().Unix()),
		RVList:        myRVs,
	}

	// Marshal the data into JSON
	data, err := json.Marshal(hbData)
	if err != nil {
		err = fmt.Errorf("failed to marshal heartbeat for node %s: %v %+v", cmi.myNodeId, err, hbData)
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

	log.Debug("ClusterManager::punchHeartBeat: heartbeat (initialHB=%v) updated by node: %s (%s), RV count: %d, %+v",
		initialHB, hbData.NodeID, hbData.IPAddr, len(hbData.RVList), hbData)
	return nil
}

// This is no doubt the most important task done by clustermanager.
// It queries all the heartbeats present and updates clustermap's RV list and MV list accordingly.
func (cmi *ClusterManager) updateStorageClusterMapIfRequired() error {
	atomic.AddInt64((*int64)(&stats.Stats.CM.StorageClustermap.Calls), 1)

	var clusterMap dcache.ClusterMap
	skipUpdateRVList := false
	nodeCount := -1

	start := time.Now()

	//
	// With lot of RVs and MVs updateStorageClusterMapIfRequired() may take a long time to complete.
	// We need to put a time bound and ensure it completes before that, else it may cause stability
	// issues with other nodes trying to take ownership of the clusterMap and that can keep ping ponging
	// w/o any progress being made.
	// Note that it's ok if we cannot finish all the pending work (primarily fixing degraded MVs) in one
	// updateStorageClusterMapIfRequired() call, as long as we make forward progress, cluster will eventually
	// heal itself, after multiple iterations of updateStorageClusterMapIfRequired().
	// We don't want to do very less work as that would mean more MVs will remain degraded for longer time
	// risking data loss.
	//
	common.Assert(cmi.config.ClustermapEpoch >= 30, cmi.config.ClustermapEpoch)
	// Must complete 10 secs before the next epoch starts.
	maxTimeMargin := time.Duration(cmi.config.ClustermapEpoch-10) * time.Second
	// Not less than 35 seconds (we want to leave at least 20 secs for updateMVList()).
	minTimeMargin := 35 * time.Second
	//
	// 1 min is a good default value.
	// TODO: Make sure this is enough for the largest cluster supported.
	//
	timeMargin := 60 * time.Second
	if timeMargin > maxTimeMargin {
		timeMargin = maxTimeMargin
	}
	if timeMargin < minTimeMargin {
		timeMargin = minTimeMargin
	}

	// updateStorageClusterMapIfRequired() MUST complete before this time.
	completeBy := start.Add(timeMargin)

	for {
		//
		// Fetch and update local clustermap as some of the functions we call later down will query the local clustermap.
		//
		clusterMap, etag, err := cmi.fetchAndUpdateLocalClusterMap()
		if err != nil {
			err1 := fmt.Errorf("ClusterManager::updateStorageClusterMapIfRequired: fetchAndUpdateLocalClusterMap() failed: %v",
				err)
			log.Err(err1.Error())
			common.Assert(false, err1)
			atomic.AddInt64((*int64)(&stats.Stats.CM.StorageClustermap.Failures), 1)
			stats.Stats.CM.StorageClustermap.LastError = err1.Error()
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
			atomic.AddInt64((*int64)(&stats.Stats.CM.StorageClustermap.Failures), 1)
			stats.Stats.CM.StorageClustermap.LastError = err.Error()
			return err
		}

		clusterMapAge := now - clusterMap.LastUpdatedAt
		//
		// Assert if clusterMap is not updated for 3 consecutive epochs, it might indicate some bug.
		// For very small ClustermapEpoch values, 3 times the value will not be sufficient as the
		// thresholdClusterMapEpochTime is set to 60, so limit it to 180.
		// The max time till which the clusterMap may not be updated in the event of leader going down is
		// 2*ClustermapEpoch + thresholdClusterMapEpochTime, so for values of ClustermapEpoch above 60 seconds, 3 times
		// ClustermapEpoch is sufficient but for smaller ClustermapEpoch values we have to cap to 180, with a margin
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
			atomic.AddInt64((*int64)(&stats.Stats.CM.StorageClustermap.Failures), 1)
			stats.Stats.CM.StorageClustermap.LastError = err.Error()
			return err
		}

		if isClusterMapUpdateBlocked {
			log.Debug("ClusterManager::updateStorageClusterMapIfRequired:skipping, clustermap is being updated by (leader %s), current node (%s)",
				leaderNode, cmi.myNodeId)
			//
			// Leader node should not find the clusterMap in "checking" state as no other node should try
			// to preempt the leader while it's still alive, but...
			// Note that updateStorageClusterMapIfRequired() when run by the leader can find the clusterMap
			// in "checking" state if some other thread, mostly batchUpdateComponentRVState(), is running and
			// updating the clusterMap, just before the periodic updateStorageClusterMapIfRequired() ticker
			// fires. This is a legitimate case and we should just skip the current iteration of
			// updateStorageClusterMapIfRequired().
			//
			// We relax the assert to allow such legitimate updates from batchUpdateComponentRVState() to catch
			// if a leader finds the state as "checking" it should be transient and not remain in that state
			// for a long time.
			//
			common.Assert(!leader || !stale,
				"We don't expect leader to see the clustermap in checking state",
				leader, stale, leaderNode, clusterMapAge)
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
			log.Info("ClusterManager::updateStorageClusterMapIfRequired: skipping, node (%s) is not leader (leader is %s) and clusterMap is fresh (last updated at epoch %d, now %d, age %d secs)",
				cmi.myNodeId, leaderNode, clusterMap.LastUpdatedAt, now, clusterMapAge)
			return nil
		}

		//
		// This is an uncommon event, so log.
		//
		if !leader {
			err1 := fmt.Errorf("ClusterManager::updateStorageClusterMapIfRequired: clusterMap not updated by current leader (%s) for %d secs, ownership being claimed by new leader %s",
				leaderNode, clusterMapAge, cmi.myNodeId)
			log.Warn("%v", err1)
			atomic.AddInt64((*int64)(&stats.Stats.CM.StorageClustermap.LeaderSwitchesDueToTimeout), 1)
			// This is not an error, but interesting event, so log it.
			stats.Stats.CM.StorageClustermap.LastError = err1.Error()
		}

		//
		// Start the clustermap update process by first claiming ownership of the clustermap update.
		// Only one node will succeed in UpdateClusterMapStart(), and that node proceeds with the clustermap
		// update.
		//
		// Note: updateRVList() and updateMVList() are the only functions that can change clustermap.
		//       Enclosing them between UpdateClusterMapStart() and UpdateClusterMapEnd() ensure that only one
		//       node would be updating cluster membership details at any point. This is IMPORTANT.
		//
		// Note: The following startClusterMapUpdate() is unlikely to fail because of some other node
		//       updating the clustermap from updateStorageClusterMapIfRequired(), as only leader will
		//       proceed, but it can fail when some other asynchronous event like batchUpdateComponentRVState()
		//       updates the clustermap, from the same node or another node.
		//
		err = cmi.startClusterMapUpdate(clusterMap, etag)
		if err != nil {
			err = fmt.Errorf("Start Clustermap update failed for nodeId %s: %v", cmi.myNodeId, err)
			log.Err("ClusterManager::updateStorageClusterMapIfRequired: %v", err)
			atomic.AddInt64((*int64)(&stats.Stats.CM.StorageClustermap.Failures), 1)
			stats.Stats.CM.StorageClustermap.LastError = err.Error()
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

		if !skipUpdateRVList {
			log.Debug("ClusterManager::updateStorageClusterMapIfRequired: updating RV list")

			//
			// TODO: Shall we pass completeBy to updateRVList() also.
			//       Since it doesn't take much time, it should be ok, but do keep an key eye on it.
			//
			changed, err := cmi.updateRVList(clusterMap.RVMap, false /* initialHB */)
			if err != nil {
				err = fmt.Errorf("failed to reconcile RV mapping: %v", err)
				log.Err("ClusterManager::updateStorageClusterMapIfRequired: %v", err)
				common.Assert(false, err)
				//
				// TODO: We must reset the clusterMap state to ready.
				//
				atomic.AddInt64((*int64)(&stats.Stats.CM.StorageClustermap.Failures), 1)
				stats.Stats.CM.StorageClustermap.LastError = err.Error()
				return err
			}

			//
			// If RV list has changed we need to pre-commit the clusterMap before we run updateMVList().
			// This is required as updateMVList() will run the fix-mv workflow which would send UpdateMV RPCs
			// to other nodes, and if the clusterMap is not pre-committed, those RPC handlers will fail those requests
			// as they will treat them as "invalid change" to RV list. e.g., let's say rv0 has gone offline as per the
			// latest updateRVList(). If we run the fix-mv workflow it'll send UpdateMV RPCs to other RVs of MVs that
			// have rv0 as one of the RVs, and those RVs will reject the UpdateMV RPCs as they will find rv0 in online
			// state. Note that online->outofsync is in invalid state transition, what UpdateMV expects is
			// offline->outofsync. refreshFromClustermap() also will not help as it'll still show rv0 state as online.
			// This will result in all fix-mv attempts failing and causing a lot of failed RPC traffic.
			// We must commit the changes to RV list in the clustermap before we run the fix-mv workflow.
			// We call this a pre-commit as it commits the RV list changes but not the corresponding MV list changes,
			// thus we have the clustermap in a "half baked" state where MV list is not reflective of the latest RV list.
			// Any node that takes ownership of clustermap update MUST run updateMVList() even if they do not find
			// any changes to RV list by updateRVList().
			//
			if changed {
				log.Debug("ClusterManager::updateStorageClusterMapIfRequired: RV list changed, pre-committing clustermap")

				err = cmi.endClusterMapUpdate(clusterMap)
				if err != nil {
					err1 := fmt.Errorf("Failed to pre-commit clusterMap: %v", err)
					log.Err("ClusterManager::updateStorageClusterMapIfRequired: %v", err1)
					common.Assert(false, err1)
					atomic.AddInt64((*int64)(&stats.Stats.CM.StorageClustermap.Failures), 1)
					stats.Stats.CM.StorageClustermap.LastError = err1.Error()
					return err
				}

				//
				// Ok, clusterMap is now updated with the RV list changes.
				// Rerun the loop once more this time to update the MV list.
				//
				skipUpdateRVList = true
				continue
			}
		} else {
			log.Debug("ClusterManager::updateStorageClusterMapIfRequired: Skipping updateRVList()")
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
		// Update: We must call updateMVList() even if no RVs changed state. This is required for few reasons:
		//       - Previous call to updateMVList() may not have completed all the tasks, e.g., it may not have
		//         fixed all the degraded MVs as it may not find replacement RVs for all. In that case we want
		//         the next updateStorageClusterMapIfRequired() call to continue fixing the remaining MVs.
		//       - updateRVList() may update the RV list but updateMVList() may not be able to run or may not
		//         complete all the tasks before some other node takes over the clustermap update and it finds
		//         the changed RV list. That node won't observe any changes in the RV list, but it still must
		//         run updateMVList() to ensure that all the MVs are fixed and their state is correct.
		//
		cmi.updateMVList(clusterMap.RVMap, clusterMap.MVMap, completeBy, true /* runFixMvNewMv */)

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
			atomic.AddInt64((*int64)(&stats.Stats.CM.StorageClustermap.Failures), 1)
			stats.Stats.CM.StorageClustermap.LastError = err.Error()
			return err
		}

		err = cmi.endClusterMapUpdate(clusterMap)
		if err != nil {
			err1 := fmt.Errorf("ClusterManager::updateStorageClusterMapIfRequired: %v", err)
			log.Err("%v", err1)
			common.Assert(false, err1)
			atomic.AddInt64((*int64)(&stats.Stats.CM.StorageClustermap.Failures), 1)
			stats.Stats.CM.StorageClustermap.LastError = err1.Error()
			return err
		}

		break
	}

	// Total time taken by updateStorageClusterMapIfRequired().
	duration := stats.Duration(time.Since(start))

	atomic.AddInt64((*int64)(&stats.Stats.CM.StorageClustermap.TotalTime), int64(duration))
	if stats.Stats.CM.StorageClustermap.MinTime == nil ||
		duration < *stats.Stats.CM.StorageClustermap.MinTime {
		stats.Stats.CM.StorageClustermap.MinTime = &duration
	}
	stats.Stats.CM.StorageClustermap.MaxTime =
		max(stats.Stats.CM.StorageClustermap.MaxTime, duration)

	//
	// If the last clusterMap update was done by the same node, calculate gap between updates.
	//
	if (stats.Stats.CM.StorageClustermap.LastUpdateEpoch == clusterMap.Epoch-1) &&
		(stats.Stats.CM.StorageClustermap.LastUpdateEpoch != 0) {
		common.Assert(!stats.Stats.CM.StorageClustermap.LastUpdatedAt.IsZero())
		gap := stats.Duration(time.Since(stats.Stats.CM.StorageClustermap.LastUpdatedAt))

		atomic.AddInt64((*int64)(&stats.Stats.CM.StorageClustermap.TotalGap), int64(gap))
		if stats.Stats.CM.StorageClustermap.MinGap == nil ||
			gap < *stats.Stats.CM.StorageClustermap.MinGap {
			stats.Stats.CM.StorageClustermap.MinGap = &gap
		}
		stats.Stats.CM.StorageClustermap.MaxGap =
			max(stats.Stats.CM.StorageClustermap.MaxGap, gap)
	} else {
		// If this is the first update or the first update after a leader switch, reset the gap stats.
		stats.Stats.CM.StorageClustermap.MinGap = nil
		atomic.StoreInt64((*int64)(&stats.Stats.CM.StorageClustermap.MaxGap), 0)
		atomic.StoreInt64((*int64)(&stats.Stats.CM.StorageClustermap.TotalGap), 0)
		atomic.StoreInt64(&stats.Stats.CM.StorageClustermap.TotalUpdates, 0)
	}

	atomic.StoreInt64(&stats.Stats.CM.StorageClustermap.LastUpdateEpoch, clusterMap.Epoch)
	stats.Stats.CM.StorageClustermap.LastUpdatedAt = time.Now()
	atomic.AddInt64((*int64)(&stats.Stats.CM.StorageClustermap.TotalUpdates), 1)

	log.Info("[TIMING] ClusterManager::updateStorageClusterMapIfRequired: cluster map (%d nodes) updated by %s at %s: %+v (took %s)",
		nodeCount, cmi.myNodeId, time.Now(), clusterMap, time.Since(start))
	return nil
}

// Given an rvMap which holds most up-to-date status of all known RVs, whether they are "online" or "offline" (this
// information is mostly derived from the heartbeats, by a prior call to updateRVList(), but it can be known through
// some other mechanism, f.e., inband detection of RV offline status by RPC calls made to nodes), and existingMVMap
// which is the set of MVs present in the clustermap, indexed by MV name and contains complete info about the MV,
// updateMVList() correctly updates all MVs' component RVs state and the derived MV state.
// It can be called from two workflows:
//  1. From updateStorageClusterMapIfRequired() (periodic clustermap update thread) after it infers some change in
//     rvMap as per the latest heartbeats received.
//     In this case runFixMvNewMv parameter is passed as true.
//  2. From batchUpdateComponentRVState(), when some other workflow wants to explicitly update component RV state for
//     some MV, f.e., resync workflow may want to change an "outofsync" component RV to "syncing" or a failed PutChunk
//     call may indicate an RV as down and hence we would want to change the component RV state to "inband-offline".
//     There could be more such examples of inband RV state detection resulting in MV list update.
//     In this case runFixMvNewMv parameter is passed as false, as we do not want to overwhelm the receivers with
//     too many RPCs as a result of potentially many parallel sync jobs running.
//
// It runs the following workflows:
//  1. degrade-mv: It goes over all the MVs in existingMVMap to see if any (but not all) of their component RVs which
//                 was previously online has gone offline. It marks those MVs as degraded and the component RV as
//                 offline.
//  2. offline-mv: This is similar to degrade-mv but if *all* (NumReplicas) component RVs have gone offline, the MV is
//                 marked offline.
//  3. fix-mv:     For all the degraded MVs it replaces all the offline component RVs with good RVs, and sets the
//                 state for those RVs as outofsync. These MVs will be later picked by Replication Manager to run
//                 the resync-mv workflow.
//                 Only run if runFixMvNewMv parameter is true.
//  4. new-mv:     This adds new MVs to the MV list, made from unused RVs. The component RVs are added in such a way
//                 that more than one component RVs for an MV do not come from the same node and the same fault domain.
//                 Only run if runFixMvNewMv parameter is true.
//
// Last two workflows (fix-mv and new-mv) are called "placement workflows" as they perform placement of MVs on RVs.
// They have to make sure the placement honors the following constraints:
// - A copy of MV has to be placed exactly on NumReplicas component RVs.
// - An MV cannot have more than one component RV from the same node.
// - An MV cannot have more than one component RV from the same fault domain.
// - An MV cannot have more than one component RV from the same update domain.
// - An RV cannot host more than MVsPerRVForNewMV component RVs for new MVs and more than MVsPerRVForFixMV component
//   RVs for fix-mv.
//
// Note that when setting MV state based on component RV state, a component RV in "outofsync" or "syncing" state is
// treated as an offline component RV, basically any component RV which does not have valid data.
//
// existingMVMap is updated in-place, the caller will then publish it in the updated clustermap.
//
// Note: updateMVList() MUST be called after successfully claiming ownership of clusterMap update, by a successful
//       call to UpdateClusterMapStart(). This is IMPORTANT to ensure only one node attempts to update clusterMap
//       at any point.

func (cmi *ClusterManager) updateMVList(rvMap map[string]dcache.RawVolume,
	existingMVMap map[string]dcache.MirroredVolume, completeBy time.Time, runFixMvNewMv bool) {

	//
	// updateMVList() must have at least 20 secs to run.
	// See updateStorageClusterMapIfRequired() for details.
	//
	common.Assert(completeBy.Sub(time.Now()) > 20*time.Second, completeBy, time.Now())

	start := time.Now()

	// We should not be called for an empty rvMap.
	common.Assert(len(rvMap) > 0)

	NumReplicas := int(cmi.config.NumReplicas)
	MVsPerRVForNewMV := int(cmi.config.MVsPerRV)
	MVsPerRVForFixMV := MVsPerRVForNewMV * int(cm.MVsPerRVScaleFactor)

	common.Assert(NumReplicas >= int(cm.MinNumReplicas) && NumReplicas <= int(cm.MaxNumReplicas), NumReplicas)
	common.Assert(MVsPerRVForNewMV >= int(cm.MinMVsPerRV) && MVsPerRVForNewMV <= int(cm.MaxMVsPerRV), MVsPerRVForNewMV)
	// MVsPerRVForFixMV must be greater than MVsPerRVForNewMV, else we cannot handle node down gracefully.
	common.Assert(MVsPerRVForFixMV > MVsPerRVForNewMV, MVsPerRVForFixMV, MVsPerRVForNewMV, cm.MVsPerRVScaleFactor)
	common.Assert(cm.IsValidRVMap(rvMap))
	common.Assert(cm.IsValidMvMap(existingMVMap, NumReplicas))

	//
	//
	// Approach used by the placement workflows (fix-mv and new-mv):
	//
	// We create a map of RVs that are available for placing MVs, indexed by RV name.
	// Each RV starts with a fixed slot count value that decides how many MVs the RV
	// can hold. The MV placement logic (new-mv and fix-mv) consumes one slot for every
	// MV placed on an RV and never places more MVs than the slot count allows.
	// Since we allow more MVs on an RV during fix-mv than we allow during new-mv, to
	// support cluster availability as some nodes go down, we set the slot count to
	// MVsPerRVForFixMV which is higher than MVsPerRVForNewMV. The new-mv workflow
	// takes care that despite the higher slot count value it doesn't place more than
	// MVsPerRVForNewMV MVs on any RV.
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
	// a new MV. This is the new-mv workflow.
	//

	log.Debug("ClusterManager::updateMVList: Updating current MV list according to the latest RV list (%d RVs, %d MVs) [%s to run]",
		len(rvMap), len(existingMVMap), completeBy.Sub(time.Now()))

	//
	// Represents an RV with all info needed by the MV placement logic.
	// Each RV stores a slots counter that decides how many more MVs can be placed on it. It also contains the
	// nodeId and fault and update domain ids for the RV, to help the placement logic to not place more than one component
	// RV for an MV on the same node or the same fault or update domain. One MV can use an RV at most once. slots is
	// initialized with MVsPerRVForFixMV and then decremented by one every time an RV is used as component RV to an MV.
	// Once all slots are consumed, the RV is no longer used for placing any more MVs.
	//
	type rv struct {
		//
		// Name of the RV, like "rv0", "rv1", etc.
		//
		rvName string

		//
		// Node hosting this RV.
		// We will never pick more than one component RV for an MV from the same node.
		// We also store nodeId as a unique integer for faster comparisons, especially in node exclusion set.
		//
		nodeId    string
		nodeIdInt int

		//
		// Fault/Update domain ID for the RV. -1 signifies that the fault/update domain id is not known.
		// Such RVs can be used to host any MV o/w it means that the RV is hosted on a node that belongs to
		// the given fault/update domain. If fault/update domain id known, placer will ensure that an MV doesn't
		// have more than one component RVs from the same fault/update domain.
		//
		// Config IgnoreFD and IgnoreUD control whether the placer honors fault and update domains.
		//
		fdId int
		udId int

		//
		// The number of MVs that can be placed on this RV.
		// This is initialized to MVsPerRVForFixMV and decremented by one every time an RV is used to place
		// an MV. This can be done at the beginning when we update the slots based on the current MVs placed
		// on the RV or later when we use this RV for placing a new MV or fixing an existing MV.
		//
		slots int
	}

	//
	// All RVs that are available for placing MVs, indexed by RV name. These are only online RVs as offline RVs
	// are not used for placing MVs.
	//
	availableRVsMap := make(map[string]*rv)

	//
	// getAvailableRVsList() makes this list out of availableRVsMap.
	// This can be used only after a call to getAvailableRVsList().
	//
	var availableRVsList []*rv

	//
	// getAvailableRVsList() sets
	// - numAvailableNodes to the number of nodes that have at least one RV available for placing MVs.
	// - numAvailableFDs to the number of fault domains that have at least one RV available for placing MVs.
	// - numAvailableUDs to the number of update domains that have at least one RV available for placing MVs.
	//
	numAvailableNodes := 0
	numAvailableFDs := 0
	numAvailableUDs := 0

	//
	// From availableRVsMap, this will create equivalent availableRVsList, list of RVs that are available for
	// placing MVs. The RVs are sorted by the number of slots available, so that the RVs with more slots are at
	// the front so they are picked first for placing MVs, thus resulting in a more balanced distribution of MVs
	// across the RVs over time as nodes go down and come up.
	//
	// Note: This is costly, call it less often, judiciously.
	// TODO: Sort also based on free space available on the RVs, so that MV placer favors RVs with more free space.
	//
	getAvailableRVsList := func(newMV bool) {
		availableRVsList = make([]*rv, 0, len(availableRVsMap))

		// Nodes and fault/update domains that have at least one RV available for placing MVs.
		nodes := make(map[int]struct{})
		faultDomains := make(map[int]struct{})
		updateDomains := make(map[int]struct{})

		for _, rv := range availableRVsMap {
			// availableRVsMap must only contain online RVs.
			common.Assert(rvMap[rv.rvName].State == dcache.StateOnline, rv.rvName, rvMap[rv.rvName].State)

			// Max slots for an RV is MVsPerRVForFixMV.
			common.Assert(rv.slots <= MVsPerRVForFixMV, rv.slots, MVsPerRVForFixMV)

			// Must have a valid nodeIdInt assigned.
			common.Assert(rv.nodeIdInt > 0, *rv)

			//
			// Skip an RV if it has no free slots left or for newMV case, the used slot count has
			// reached MVsPerRVForNewMV.
			//
			usedSlots := MVsPerRVForFixMV - rv.slots
			if rv.slots > MVsPerRVForFixMV {
				usedSlots = MVsPerRVForFixMV
			}

			if rv.slots == 0 || (newMV && usedSlots >= MVsPerRVForNewMV) {
				continue
			}

			//
			// Ok, this RV can host at least one more MV.
			// Note that rv is a pointer to an rv struct, so availableRVsList also points to the same rv struct
			// which are hosted in availableRVsMap. Thus functions like consumeRVSlot() and deleteRVsFromAvailableMap()
			// which change availableRVsMap will also cause changes in availableRVsList.
			//
			availableRVsList = append(availableRVsList, rv)

			nodes[rv.nodeIdInt] = struct{}{}
			if rv.fdId != -1 {
				faultDomains[rv.fdId] = struct{}{}
			}
			if rv.udId != -1 {
				updateDomains[rv.udId] = struct{}{}
			}
		}

		// Set these global variables, some callers may need them.
		numAvailableNodes = len(nodes)
		numAvailableFDs = len(faultDomains)
		numAvailableUDs = len(updateDomains)
		_ = numAvailableFDs
		_ = numAvailableUDs

		// Sort the RVs by the number of slots available, in descending order.
		sort.Slice(availableRVsList, func(i, j int) bool {
			return availableRVsList[i].slots > availableRVsList[j].slots
		})

		log.Debug("ClusterManager::getAvailableRVsList: Available RVs: %d, nodes: %d, FD: %d, UD: %d",
			len(availableRVsList), numAvailableNodes, numAvailableFDs, numAvailableUDs)
	}

	//
	// Helper function to consume an rv slot when rvName is allotted to mvName.
	//
	// This updates availableRVsMap and availableRVsList.
	//
	consumeRVSlot := func(mvName, rvName string) {
		nodeId := rvMap[rvName].NodeId
		_ = nodeId
		// Simple assert to make sure rvName is present in rvMap.
		common.Assert(len(nodeId) > 0)
		// We don't add offline RVs to availableRVsMap, so we must not be updating their slot count.
		common.Assert(rvMap[rvName].State == dcache.StateOnline, rvName, mvName, rvMap[rvName].State)

		// RV to consume MUST be present in availableRVsMap.
		rv, ok := availableRVsMap[rvName]
		_ = ok
		common.Assert(ok, rvName, mvName, nodeId, availableRVsMap)
		common.Assert(rv.nodeId == nodeId, rvName, mvName, nodeId, availableRVsMap)
		common.Assert(rv.fdId == rvMap[rvName].FDId, rvName, mvName, nodeId, rv.fdId, rvMap[rvName].FDId)
		common.Assert(rv.udId == rvMap[rvName].UDId, rvName, mvName, nodeId, rv.udId, rvMap[rvName].UDId)

		// We initialize slot count to MVsPerRVForFixMV and then reduce it from there.
		common.Assert(rv.slots <= MVsPerRVForFixMV, rvName, mvName, nodeId, rv.slots, MVsPerRVForFixMV)
		// Caller must call consumeRVSlot() only for RVs with at least one slot available.
		common.Assert(rv.slots > 0, rvName, mvName, nodeId, availableRVsMap)

		rv.slots--
		log.Debug("ClusterManager::consumeRVSlot: Consumed slot for %s/%s, (used: %d, remaining: %d)",
			rvName, mvName, MVsPerRVForFixMV-rv.slots, rv.slots)
	}

	//
	// Helper function to remove given RV(s) from availableRVsMap.
	// This is called when some RVs are found to be "bad" and we don't want to use them for subsequent MV
	// placement.
	//
	// This updates availableRVsMap and availableRVsList.
	//
	deleteRVsFromAvailableMap := func(deleteRvNames []string) {
		for _, deleteRvName := range deleteRvNames {
			nodeId := rvMap[deleteRvName].NodeId
			_ = nodeId
			// Simple assert to make sure deleteRvName is present in rvMap.
			common.Assert(len(nodeId) > 0, deleteRvName)
			// We don't add offline RVs to availableRVsMap, so we must not be deleting them.
			common.Assert(rvMap[deleteRvName].State == dcache.StateOnline, deleteRvName, rvMap[deleteRvName].State)

			// It MUST be present in availableRVsMap.
			rv, ok := availableRVsMap[deleteRvName]
			_ = rv
			_ = ok
			common.Assert(ok, deleteRvName, nodeId, availableRVsMap)

			log.Debug("ClusterManager::deleteRVFromNode: Deleted RV %s (with %d slots), node: %s, FD: %d, UD: %d",
				deleteRvName, rv.slots, nodeId, rv.fdId, rv.udId)

			//
			// Set slots to 0 so that availableRVsList which is referring to this RV does not consider it for
			// placing any more MVs.
			//
			rv.slots = 0

			delete(availableRVsMap, deleteRvName)
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
		excludeNodes := make(map[int]struct{})
		excludeFaultDomains := make(map[int]struct{})
		excludeUpdateDomains := make(map[int]struct{})

		//
		// Pass 1: Make a list of nodes and fault domains to be excluded when picking "good" RVs in the
		//         later part. Those nodes and fault/update domains are excluded which contribute at least
		//         one good component RV.
		//
		savedRVs := make(map[string]dcache.StateEnum)
		for rvName, rvState := range mv.RVs {
			// Only valid RVs can be used as component RVs for an MV.
			rv, exists := rvMap[rvName]
			_ = exists
			common.Assert(exists)

			//
			// We make a deep copy of mv.RVs before we start fixing.
			// We fix directly in mv.RVs as it's convenient, but if we need to undo later we reset mv.RVs to
			// savedRVs.
			//
			savedRVs[rvName] = rvState

			//
			// Fix-mv workflow is run after degrade-mv/offline-mv workflows, so component RV states
			// must have been correctly updated to offline.
			//

			//
			// If state of RV in rvMap is offline, state of component RV in MV MUST be offline.
			// Note that we can have an RV as online in rvMap but still not online in MV, since once
			// an RV goes offline and comes back it cannot simply be marked online in the MV, it has
			// to go through degrade-mv/fix-mv workflows.
			//
			common.Assert(rv.State == dcache.StateOnline || rvState == dcache.StateOffline,
				rvName, mvName, rv.State, rvState)

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
			//   MV, when updateMVList()->fixMV() is called from batchUpdateComponentRVState(), we can have component
			//   RVs still in outofsync state.
			//
			// Leave this assert commented to highlight the above.
			//
			// common.Assert(rvState != dcache.StateOutOfSync, rvName, rvState)

			if rvState == dcache.StateOutOfSync {
				outofsyncRVs++
			}

			//
			// If this component RV is not offline, its containing node and fault/update domain must be excluded
			// for replacement RV(s). We don't exclude the node and fault/update domain if the component RV is
			// offline to support the case where the same node comes back up online and we may want to use the same
			// RV or another RV from the same node, as replacement RV.
			// If the component RV is inband-offline, we exclude its node because we don't want other RVs in
			// that node to be used as replacement RV (since it's likely that the entire node is down),
			// but we still count it as offline, as it needs to be replaced with a good RV. We do not exclude
			// the fault/update domain for inband-offline RVs, as we should be able to use other RVs from different
			// nodes in the same fault/update domain as replacement RVs.
			//
			if rvState != dcache.StateOffline {
				// More than one component RVs for an MV cannot come from the same node.
				excludeNodes[cm.UUIDToUniqueInt(rv.NodeId)] = struct{}{}

				if rvState == dcache.StateInbandOffline {
					// inband-offline component RVs should be treated as offline but we don't exclude
					// its fault domain, as it's ok to use other RVs from the same fault domain.
					offlineRVs++
				} else {
					// More than one component RVs for an MV cannot come from the same fault/update domain.
					if rv.FDId != -1 {
						excludeFaultDomains[rv.FDId] = struct{}{}
					}
					if rv.UDId != -1 {
						excludeUpdateDomains[rv.UDId] = struct{}{}
					}
				}
				continue
			}

			offlineRVs++
		}

		// Degraded MVs must have one or more but not all component RVs as offline, inband-offline or outofsync.
		common.Assert((offlineRVs+outofsyncRVs) != 0 && (offlineRVs+outofsyncRVs) < NumReplicas,
			mvName, offlineRVs, outofsyncRVs, NumReplicas)

		// No component RV is offline/inband-offline, nothing to fix, return.
		if offlineRVs == 0 {
			// If not offline, must have at least one outofsync, else why the MV is degraded.
			common.Assert(outofsyncRVs > 0, mvName)
			log.Debug("ClusterManager::fixMV: %s has no offline/inband-offline component RV, nothing to fix %+v",
				mvName, mv.RVs)
			return
		}

		//
		// Pass 2: For all component RVs that are offline/inband-offline, find a suitable replacement RV.
		//         A suitable RV is one, that:
		//         - Does not come from any node in excludeNodes list.
		//         - Does not come from any fault domain in excludeFaultDomains list.
		//         - Does not come from any update domain in excludeUpdateDomains list.
		//         - Has same or higher availableSpace.
		//
		// Caller creates availableRVsList which is a list of available RVs that can be used to replace the
		// offline component RVs. This is a sorted list with more suitable RVs at the front, so that we are
		// more likely to pick more suitable RVs first, thus resulting in a balanced distribution of MVs across
		// the RVs. We then iterate over the availableRVsList list and pick the 1st suitable RV.
		// As we pick RVs we update availableRVsMap which also updates availableRVsList as it is a slice of
		// those pointers that availableRVsMap refers to.
		//

		//
		// Number of component RVs we are actually able to fix for this MV.
		// If we cannot fix anything, skip the joinMV().
		//
		fixedRVs := 0
		alreadyOutOfSync := make(map[string]struct{})

		// Above loop must have made a deep copy of mv.RVs in savedRVs.
		common.Assert(len(mv.RVs) == len(savedRVs), mvName, len(mv.RVs), len(savedRVs))

		// Fix all the offline component RVs for this MV.
		for rvName, rvState := range mv.RVs {
			//
			// Usually we won't have outofsync component RVs when fixMV() is called, as they would have been
			// picked by the resync workflow and changed to syncing/online by the time fixMV() is called next
			// time, but it's possible that we are called with one or more outofsync component RVs.
			//
			// e.g., fixMV() was called for mv1 with the following component RVs.
			// mv2:{degraded map[rv0:outofsync rv3:online rv4:offline]}
			//
			// Note that rv0 was outofsync on entry and rv4 was replaced by rv1 and rv1 was newly marked outofsync,
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
			if rvState == dcache.StateOutOfSync {
				log.Debug("ClusterManager::fixMV: %s/%s already outofsync", rvName, mvName)

				// Must not already be there.
				if common.IsDebugBuild() {
					_, ok := alreadyOutOfSync[rvName]
					common.Assert(!ok, rvName, mvName, alreadyOutOfSync)
				}
				alreadyOutOfSync[rvName] = struct{}{}
				continue
			}

			// Only offline/inband-offline component RVs need to be "fixed" (aka replaced).
			if rvState != dcache.StateOffline && rvState != dcache.StateInbandOffline {
				continue
			}

			//
			// We will now be sending UpdateMV RPC asking this component RV to be moved to outofsync state.
			// Note that rvInfo on the target nodes will have the RV state as online, so it'll refresh the
			// RV state from the clusterMap. If clusterMap state is not offline it'll fail the RPC.
			// When we reach here, updateStorageClusterMapIfRequired() would have ensured that any offline
			// RV state is pre-committed before calling updateMVList(), assert for that.
			//
			if common.IsDebugBuild() {
				cmRVs := cm.GetRVs(mvName)
				common.Assert(len(cmRVs) == NumReplicas, mvName, cmRVs, NumReplicas)

				_, ok := cmRVs[rvName]
				common.Assert(ok, rvName, mvName, cmRVs)

				//
				// inband-offline state is always committed to clusterMap and not calculated by updateMVList()
				// so if rvState is inband-offline, it must have been read from clusterMap.
				//
				if rvState == dcache.StateInbandOffline {
					common.Assert(cmRVs[rvName] == dcache.StateInbandOffline, rvName, mvName, cmRVs[rvName])
				}

				//
				// rvState can be offline in two cases:
				// - It was already offline in clusterMap component RVs, when updateMVList() was called.
				// - It was not offline in clusterMap component RVs when this updateMVList() was called, but
				//   updateRVList() (called just before updateMVList()) found the heartbeat as expired and marked it
				//   as offline in rvMap passed to updateMVList() which then marked the component RV state as offline
				//   in degrade-mv workflow. In this case updateStorageClusterMapIfRequired() MUST have pre-committed
				//   the RV state change to clusterMap and hence cm.GetRVState(rvName) MUST return offline.
				//
				if rvState == dcache.StateOffline {
					common.Assert((cmRVs[rvName] == dcache.StateOffline ||
						cm.GetRVState(rvName) == dcache.StateOffline),
						rvName, mvName, cmRVs, cm.GetRVState(rvName))
				}
			}

			foundReplacement := false
			firstFreeIdx := 0
			firstFreeIdxLocked := false

			log.Debug("ClusterManager::fixMV: Fixing component RV %s/%s (state: %s)",
				rvName, mvName, rvState)

			//
			// Iterate over the availableRVsList and pick the first suitable RV.
			//
			// Note: Since the number of RVs can be very large (100K+) we need to be careful that this loop
			//       should run very very fast, as we need to fix all the degraded MVs in a short time.
			//       Avoid any string key'ed map lookups, as they are slow, and any thing else that's slow.
			//
			for _, rv := range availableRVsList {
				// Max slots for an RV is MVsPerRVForFixMV.
				common.Assert(rv.slots <= MVsPerRVForFixMV, rv.slots, MVsPerRVForFixMV)

				//
				// Skip an RV if it has no free slots left.
				// This check is the fastest, so we do it first.
				//
				if rv.slots == 0 {
					log.Debug("ClusterManager::fixMV: Skipping %s as it has no slots left", rv.rvName)

					if !firstFreeIdxLocked {
						firstFreeIdx++
					}
					continue
				}

				firstFreeIdxLocked = true

				if _, ok := excludeNodes[rv.nodeIdInt]; ok {
					//
					// Skip RVs from excluded nodes.
					// When faking scale test we add 1000s of RVs to a node which makes this log very chatty.
					// Skip it only for that as it may be useful for a real cluster.
					//
					if !common.IsFakingScaleTest() {
						log.Debug("ClusterManager::fixMV: Skipping %s from node %s in excludeNodes %+v",
							rv.rvName, rv.nodeId, excludeNodes)
					}
					continue
				}

				if rv.fdId != -1 {
					if _, ok := excludeFaultDomains[rv.fdId]; ok {
						// Skip RVs from excluded fault domains.
						log.Debug("ClusterManager::fixMV: Skipping %s from fault domain %d in excludeFaultDomains %+v",
							rv.rvName, rv.fdId, excludeFaultDomains)
						continue
					}
				}

				if rv.udId != -1 {
					if _, ok := excludeUpdateDomains[rv.udId]; ok {
						// Skip RVs from excluded update domains.
						log.Debug("ClusterManager::fixMV: Skipping %s from update domain %d in excludeUpdateDomains %+v",
							rv.rvName, rv.udId, excludeUpdateDomains)
						continue
					}
				}

				//
				// Ok, potential replacement RV, few more checks before we can use it.
				// Note that the available space check is done by joinMV() as the target node has
				// the most up-to-date information about available space, and it'll fail JoinMV
				// RPC if the available space is not enough.
				//
				newRvName := rv.rvName

				// Only online RVs are present in availableRVsList.
				common.Assert(rvMap[newRvName].State == dcache.StateOnline, newRvName, rvMap[newRvName].State)

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
						log.Debug("ClusterManager::fixMV: Not replacing %s/%s with sibling %s/%s",
							rvName, mvName, newRvName, mvName)
						continue
					}
				} else {
					//
					// The rv selected for replacement is the same as the one we are trying to replace.
					// If the state of the RV in the MV is inband-offline, we add its node to the
					// excludeNodes map. So, we should only get the state of the RV as offline, if the
					// replacement RV is same.
					//
					common.Assert(rvState == dcache.StateOffline, mvName, rvName, mv.RVs)
				}

				//
				// Use this RV to replace older RV, a newly replaced RV starts as "outofsync" to indicate
				// that the RV is good but needs to be sync'ed (from a good component RV).
				//
				// Remove the bad RV from MV. Do this before assigning the replacement RV, in case both
				// are same.
				//
				log.Debug("ClusterManager::fixMV: Replacing (%s/%s [%s] -> %s/%s [%s] [with slots: %d])",
					rvName, mvName, rvState, newRvName, mvName, dcache.StateOutOfSync, rv.slots)

				delete(mv.RVs, rvName)
				mv.RVs[newRvName] = dcache.StateOutOfSync

				//
				// Now mv is updated to correctly reflect new selected RV, with bad RV removed.
				// We don't yet update existingMVMap, we will do it once joinMV() returns
				// successfully.
				//
				foundReplacement = true
				fixedRVs++
				break
			}

			if firstFreeIdx > 0 {
				// Chop off unusable RVs from the beginning, to avoid wasted iterations for subsequent MVs.
				availableRVsList = availableRVsList[firstFreeIdx:]

				log.Debug("ClusterManager::fixMV: Initial %d RVs are full, removing from availableRVsList, %d RVs remaining",
					firstFreeIdx, len(availableRVsList))
			}

			//
			// If we could not find a replacement RV for an offline RV, it's a matter of concern as the MV
			// will be forced to run degraded for a longer period risking data loss.
			//
			if !foundReplacement {
				log.Warn("ClusterManager::fixMV: No replacement RV found for %s/%s, available RVs: %d, excludeNodes: %+v, excludeFaultDomains: %+v, excludeUpdateDomains: %+v",
					rvName, mvName, len(availableRVsList), excludeNodes, excludeFaultDomains, excludeUpdateDomains)
				atomic.AddInt64(&stats.Stats.CM.FixMV.NoReplacementRVs, 1)
				atomic.AddInt64(&stats.Stats.CM.FixMV.NoReplacementRVsCumulative, 1)
			}
		}

		// We should be fixing no more than offlineRVs RVs.
		common.Assert(fixedRVs <= offlineRVs, fixedRVs, offlineRVs)

		// Skip joinMV() if nothing changed in clustermap.
		if fixedRVs == 0 {
			log.Warn("ClusterManager::fixMV: Could not fix any RV for MV %s", mvName)
			atomic.AddInt64(&stats.Stats.CM.FixMV.MVsNotFixed, 1)
			atomic.AddInt64(&stats.Stats.CM.FixMV.MVsNotFixedCumulative, 1)
			return
		}

		//
		// Ok, we have selected a replacement RV for each offline component RV, but before we can finalize
		// the selection, we need to check with the selected RV(s).
		// Call joinMV() and check if all component RVs are able to join successfully.
		// Note that though it's called joinMV(), it sends both JoinMV and UpdateMV RPC depending on the
		// RV state. An RV which is being added to an MV for the first time (either new MV or replacing a
		// bad component RV) is sent the JoinMV RPC while an existing component RV which just needs to be
		// made aware of the component RVs is sent the UpdateMV RPC>
		//
		// Iff joinMV() is successful, consume one slot for each component RV and update existingMVMap.
		//
		failedRVs, err := cmi.joinMV(mvName, mv)
		if err == nil {
			common.Assert(len(failedRVs) == 0, failedRVs)
			log.Info("ClusterManager::fixMV: Successfully joined/updated all component RVs %+v to MV %s, original [%+v]",
				mv.RVs, mvName, savedRVs)
			for rvName, rvState := range mv.RVs {
				//
				// Consume slot for the replacement RVs, just made outofsync, but skip RVs which were already
				// outofsync on entry to fixMV().
				//
				if rvState == dcache.StateOutOfSync {
					_, exists := alreadyOutOfSync[rvName]
					if !exists {
						consumeRVSlot(mvName, rvName)
						atomic.AddInt64(&stats.Stats.CM.FixMV.RVsReplaced, 1)
						atomic.AddInt64(&stats.Stats.CM.FixMV.RVsReplacedCumulative, 1)
					}
				}
			}
			existingMVMap[mvName] = mv

			// An MV is fixed only if all offline component RVs are "fixed", i.e., replaced with good RVs.
			if fixedRVs == offlineRVs {
				atomic.AddInt64(&stats.Stats.CM.FixMV.MVsFixed, 1)
				atomic.AddInt64(&stats.Stats.CM.FixMV.MVsFixedCumulative, 1)
			} else {
				atomic.AddInt64(&stats.Stats.CM.FixMV.MVsPartiallyFixed, 1)
				atomic.AddInt64(&stats.Stats.CM.FixMV.MVsPartiallyFixedCumulative, 1)
			}
		} else {
			//
			// If we fail to fix the MV we simply return leaving the broken MV in existingMVMap.
			// TODO: We should add retries here.
			// TODO: Should we remove failedRVs from availableRVsMap? We can do it only for RPC errors that indicate
			//       a general error indicating RV's inability to be used for any MV (like RV going offline) and
			//       not an error specific to this MV.
			//
			log.Err("ClusterManager::fixMV: Error joining RV(s) %v with MV %s: %v, reverting [%+v -> %+v]",
				failedRVs, mvName, err, mv.RVs, savedRVs)

			mv.RVs = savedRVs
			existingMVMap[mvName] = mv
			atomic.AddInt64(&stats.Stats.CM.FixMV.MVsFixFailedDueToJoinMVOrUpdateMV, 1)
			atomic.AddInt64(&stats.Stats.CM.FixMV.MVsFixFailedDueToJoinMVOrUpdateMVCumulative, 1)
		}
	}

	//
	// Phase 0:
	// Populate the availableRVsMap map from the current rvMap.
	// When not running fix-mv/new-mv we don't need the availableRVsMap, so we skip this phase.
	//
	if runFixMvNewMv {
		atomic.StoreInt64((*int64)(&stats.Stats.CM.NewMV.OfflineRVs), 0)
		for rvName, rvInfo := range rvMap {
			common.Assert(cm.IsValidRV(&rvInfo))

			if rvInfo.State == dcache.StateOffline {
				atomic.AddInt64((*int64)(&stats.Stats.CM.NewMV.OfflineRVs), 1)
				// Skip RVs that are offline as they cannot contribute to any MV.
				continue
			}

			if common.IsDebugBuild() {
				_, ok := availableRVsMap[rvName]
				// Must not already be present in availableRVsMap.
				common.Assert(!ok, rvName, availableRVsMap)
			}

			availableRVsMap[rvName] = &rv{
				rvName:    rvName,
				nodeId:    rvInfo.NodeId,
				nodeIdInt: cm.UUIDToUniqueInt(rvInfo.NodeId),
				fdId:      rvInfo.FDId,
				udId:      rvInfo.UDId,
				slots:     MVsPerRVForFixMV,
			}
		}

		// Cannot have more available RVs than total RVs.
		common.Assert(len(availableRVsMap) <= len(rvMap), availableRVsMap, rvMap)
	}

	//
	// Phase 1:
	//
	// Go over all MVs in existingMVMap and correctly set MV's state based on the state of all the
	// component RVs and consume RV slots for all used component RVs. If a component RV is found to be offline as
	// per rvMap, then the component RV is force marked offline. Then it sets the MV state based on the cumulative
	// state of all of it's component RVs as follows:
	// - If all component RVs of an MV are online, the MV is marked as online, else
	// - If no component RV of an MV is online (they are either offline/inband-offline, outofsync or syncing),
	//   the MV is marked as offline, else
	// - If at least one component RV is online, the MV is marked as degraded, else
	// - All component RVs are either online or syncing, then MV is marked as syncing.
	//
	// Few examples:
	// online, online, online => online
	// online, online, offline|inband-offline => degraded
	// online, online, outofsync => degraded
	// online, outofsync, outofsync => degraded
	// online, syncing, outofsync => degraded
	// online, syncing, syncing => syncing
	// online, online, syncing => syncing
	// offline|inband-offline, syncing, syncing => offline
	// offline|inband-offline, outofsync, syncing => offline
	// offline|inband-offline, outofsync, outofsync => offline
	// offline|inband-offline, offline|inband-offline, offline|inband-offline => offline
	//
	for mvName, mv := range existingMVMap {
		offlineRVs := 0
		inbandOfflineRVs := 0
		syncingRVs := 0
		onlineRVs := 0
		outofsyncRVs := 0

		for rvName := range mv.RVs {
			// Only valid RVs can be used as component RVs for an MV.
			rv, exists := rvMap[rvName]
			_ = exists
			common.Assert(exists, rvName, mvName)

			//
			// First things first, an offline RV MUST be marked as an offline component RV.
			// fixMV() finds offline component RVs and replaces them with good RVs.
			//
			if rv.State == dcache.StateOffline {
				mv.RVs[rvName] = dcache.StateOffline
			}

			rvState := mv.RVs[rvName]

			if rvState == dcache.StateOnline {
				onlineRVs++
			} else if rvState == dcache.StateOffline {
				offlineRVs++
			} else if rvState == dcache.StateInbandOffline {
				inbandOfflineRVs++
			} else if rvState == dcache.StateOutOfSync {
				outofsyncRVs++
			} else if rvState == dcache.StateSyncing {
				syncingRVs++
			}

			//
			// This RV is not offline and is used as a component RV by this MV.
			// Reduce its slot count, so that we don't use a component RV more than MVsPerRVForFixMV times
			// across all MVs.
			// Note that offline RVs are not included in availableRVsMap so we should not be updating their slot count.
			//
			// We don't reduce slot count if the component RV itself is marked offline/inband-offline. This is because
			// an offline/inband-offline component RV for all purposes can be treated as not-used. Soon after this we
			// will run the fix-mv workflow which will replace these offline/inband-offline RVs with some online RV
			// (it could be the same RV if it has come back up online, only for StateOffline) and at that time we will
			// not increase the slot count of the outgoing component RV, so we don't reduce it now.
			//
			// Note: When not running fix-mv/new-mv workflows, we do not care about the RV slots.
			//
			if runFixMvNewMv {
				if rv.State != dcache.StateOffline {
					if rvState != dcache.StateOffline && rvState != dcache.StateInbandOffline {
						consumeRVSlot(mvName, rvName)
					}
				}
			}
		}

		common.Assert((onlineRVs+offlineRVs+inbandOfflineRVs+outofsyncRVs+syncingRVs) == len(mv.RVs),
			onlineRVs, offlineRVs, inbandOfflineRVs, outofsyncRVs, syncingRVs, len(mv.RVs))

		if (offlineRVs + inbandOfflineRVs + outofsyncRVs + syncingRVs) == len(mv.RVs) {
			// No component RV is online, offline-mv.
			mv.State = dcache.StateOffline
		} else if onlineRVs == len(mv.RVs) {
			mv.State = dcache.StateOnline
		} else if offlineRVs > 0 || inbandOfflineRVs > 0 || outofsyncRVs > 0 {
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
	// TODO: Shall we commit the clustermap changes (marking offline component RVs as offline in MV)?
	//       Note that fixMV() will call UpdateMV RPC which only allows legitimate component RVs update.
	//       For that it'll refresh the clustermap and if it gets the old clustermap (with RV as online),
	//       UpdateMV will fail.
	//
	log.Debug("ClusterManager::updateMVList: existingMVMap after phase#1, runFixMvNewMv: %v: (%d RVs, %d MVs), [%s to run] %+v",
		runFixMvNewMv, len(rvMap), len(existingMVMap), completeBy.Sub(time.Now()), existingMVMap)

	//
	// fix-mv and new-mv workflows can cause lot of RPC calls (JoinMV/UpdateMV) to be generated, so we run
	// those only when updateMVList() is called from the periodic updateStorageClusterMapIfRequired().
	//
	if !runFixMvNewMv {
		return
	}

	//
	// Phase 2:
	//
	// Fix the degraded MVs by replacing their offline RVs with good ones.
	// This is the fix-mv workflow.
	//
	// Note that we can/must only fix degraded MVs, offline MVs cannot be fixed as there's no good component
	// RV to copy chunks from. Once an MV is offline it won't be used by File Manager to put any file's data.
	// Offline MVs will just be lying around like satellite debris in space.
	//
	// TODO: See if we need delete-mv workflow to clean those up.
	//
	// Note on performance of fix-mv:
	// We call fixMV() serially for each degraded MV, so the performance of fix-mv workflow is O(Number of degraded MVs),
	// and the number of degraded MVs is a factor of how many RVs can go offline at a time and how many MVs could be
	// hosted by those RVs. Max we can have MaxMVsPerRV (100) MVs per RV and let's take max 6 RVs per node and since
	// we cannot handle more than 2 nodes going down at a time, let's take 2 nodes, so we can have at most
	// 2 * 6 * 100 = 1200 degraded MVs at a time. Each fixMV() makes JoinMV/UpdateMV RPC calls (in parallel) and
	// typically takes ~3ms, so 1200 will take ~3.6s to fix all degraded MVs. In fact for large clusters MVsPerRV will
	// be much lower, so we will have much fewer degraded MVs at a time.
	// This should be reasonable time, so we don't need to run fix-mv in parallel for each degraded MV.
	//

	// Reset per-fix-mv stats.
	atomic.StoreInt64(&stats.Stats.CM.FixMV.Calls, 0)
	atomic.StoreInt64(&stats.Stats.CM.FixMV.MVsFixed, 0)
	atomic.StoreInt64(&stats.Stats.CM.FixMV.MVsPartiallyFixed, 0)
	atomic.StoreInt64(&stats.Stats.CM.FixMV.MVsNotFixed, 0)
	atomic.StoreInt64(&stats.Stats.CM.FixMV.MVsFixFailedDueToJoinMVOrUpdateMV, 0)
	atomic.StoreInt64(&stats.Stats.CM.FixMV.RVsReplaced, 0)
	atomic.StoreInt64(&stats.Stats.CM.FixMV.NoReplacementRVs, 0)
	atomic.StoreInt64(&stats.Stats.CM.FixMV.JoinMV.Calls, 0)
	atomic.StoreInt64(&stats.Stats.CM.FixMV.JoinMV.Failures, 0)
	atomic.StoreInt64(&stats.Stats.CM.FixMV.UpdateMV.Calls, 0)
	atomic.StoreInt64(&stats.Stats.CM.FixMV.UpdateMV.Failures, 0)

	//
	// Set availableRVsList from availableRVsMap.
	// fixMV() will use this list to find replacement RVs for degraded MVs.
	// For huge clusters with lots of RVs getAvailableRVsList() can take non-trivial time, so we run it only once.
	//
	getAvailableRVsList(false /* newMV */)

	numUsableMVs := 0
	mvsProcessed := 0

	for mvName, mv := range existingMVMap {
		if mv.State != dcache.StateOffline {
			numUsableMVs++
		}

		if mv.State != dcache.StateDegraded {
			continue
		}

		fixMVStart := time.Now()

		fixMV(mvName, mv)

		mvsProcessed++

		duration := stats.Duration(time.Since(fixMVStart))
		atomic.AddInt64((*int64)(&stats.Stats.CM.FixMV.Calls), 1)
		atomic.AddInt64((*int64)(&stats.Stats.CM.FixMV.CallsCumulative), 1)
		atomic.AddInt64((*int64)(&stats.Stats.CM.FixMV.TotalTime), int64(duration))
		if stats.Stats.CM.FixMV.MinTime == nil ||
			duration < *stats.Stats.CM.FixMV.MinTime {
			stats.Stats.CM.FixMV.MinTime = &duration
		}
		stats.Stats.CM.FixMV.MaxTime =
			max(stats.Stats.CM.FixMV.MaxTime, duration)

		//
		// If we have processed more than 100 MVs, check if we are running out of time.
		// Min 100 degraded MVs we must fix for making any decent progress.
		//
		if mvsProcessed >= 100 && (mvsProcessed%100 == 0) {
			if time.Now().After(completeBy) {
				log.Warn("ClusterManager::updateMVList: Prematurely exiting fix-mv after processing %d degraded MVs [%s]",
					mvsProcessed, completeBy)
				break
			}
		}
	}

	log.Debug("ClusterManager::updateMVList: existingMVMap after phase#2 (%d RVs, %d MVs) [%s to run] %+v",
		len(rvMap), len(existingMVMap), completeBy.Sub(time.Now()), existingMVMap)

	//
	// Phase 3:
	//
	// Here we run the new-mv workflow, where we add as many new MVs as we can with the available RVs, picking
	// NumReplicas RVs for each new MV under the following conditions:
	// - An RV can be used as component RV by at most MVsPerRVForNewMV MVs.
	// - More than one RV from the same node will not be used as component RVs for the same MV.
	// - More than one RV from the same fault domain will not be used as component RVs for the same MV.
	//
	startNewMV := time.Now()

	//
	// Get the availableRVsList from availableRVsMap.
	// These RVs will be used to create new MVs.
	// Note that we don't need to recalculate availableRVsList after every new MV is created, as availableRVsList
	// is a slice of rv pointers which are pointing to the entries in availableRVsMap, and hence consumeRVSlot()
	// will update the slots which will be reflected in availableRVsList as well.
	// We do this to improve performance for setups with large number of RVs (and MVs).
	//
	getAvailableRVsList(true /* newMV */)

	//
	// Shuffle the nodes to encourage random selection of RVs (from random nodes).
	// Note that for new MV placement most/all RVs will be empty so sorting by free slots is not really useful.
	//
	rand.Shuffle(len(availableRVsList), func(i, j int) {
		availableRVsList[i], availableRVsList[j] = availableRVsList[j], availableRVsList[i]
	})

	maxMVsPossible := (len(rvMap) * MVsPerRVForNewMV) / NumReplicas
	atomic.StoreInt64(&stats.Stats.CM.NewMV.MVsPerRV, int64(MVsPerRVForNewMV))
	atomic.StoreInt64(&stats.Stats.CM.NewMV.NumReplicas, int64(NumReplicas))
	atomic.StoreInt64(&stats.Stats.CM.NewMV.MaxMVsPossible, int64(maxMVsPossible))

	for {
		//
		// With rvMap and MVsPerRVForNewMV and NumReplicas, we cannot have more than maxMVsPossible usable MVs.
		// Note that we are talking of online or degraded/syncing MVs. Offline MVs have all component RVs
		// offline and they don't consume any RV slot, so they should be omitted from usable MVs.
		//
		// Q: Why do we need to limit numUsableMVs to maxMVsPossible?
		//    IOW, why is it not ok to create as many new MVs as we can till we have available RV slots.
		// A: If we create as many new MVs as we can with the available RVs, we might end up creating more than
		//    maxMVsPossible if some of the MVs have offline RVs (fixMV() would have attempted to replace offline
		//    RVs for all degraded MVs but if joinMV() fails or any other error we can have some component RVs as
		//    offline). We don't want to create more MVs leaving some MVs with no replacement RVs available.
		//
		common.Assert(numUsableMVs <= maxMVsPossible, numUsableMVs, maxMVsPossible)
		if numUsableMVs == maxMVsPossible {
			log.Debug("ClusterManager::updateMVList: numUsableMVs [%d] == maxMVsPossible [%d]",
				numUsableMVs, maxMVsPossible)
			break
		}

		// New MV's name, starting from index 0.
		mvName := fmt.Sprintf("mv%d", len(existingMVMap))

		excludeNodes := make(map[int]struct{})
		excludeFaultDomains := make(map[int]struct{})
		excludeUpdateDomains := make(map[int]struct{})
		firstFreeIdx := 0
		firstFreeIdxLocked := false

		//
		// Iterate over the availableRVsList and pick the first suitable RV.
		//
		// Note: Since number of RVs can be very large (100K+) we need to be careful that this loop is very
		//       efficient, avoid any string key'ed map lookups, as they are slow, and any thing else that's slow.
		// Note: This is O(number of MVs created) as it creates each new MV sequentially. Each MV creation involves
		//       sending JoinMV RPC to all component RVs (in parallel), which will be hard to get below 1ms, so for
		//       20K MVs, it'll take ~20s to create all MVs, which should be fine.
		//
		for _, rv := range availableRVsList {
			usedSlots := MVsPerRVForFixMV - rv.slots
			if rv.slots > MVsPerRVForFixMV {
				common.Assert(false, rv.slots, MVsPerRVForFixMV, MVsPerRVForNewMV)
				usedSlots = MVsPerRVForFixMV
			}

			// This check is the fastest, so we do it first.
			if usedSlots >= MVsPerRVForNewMV {
				//
				// new-mv workflow cannot use an RV more than MVsPerRVForNewMV times.
				// Note that the slots may not be 0 as we initialize slots to MVsPerRVForFixMV, which is
				// greater than MVsPerRVForNewMV.
				//
				// TODO: See if removing "full" RVs from availableRVsList is good for performance with lot of RVs.
				//
				if !firstFreeIdxLocked {
					firstFreeIdx++
				}
				continue
			}

			firstFreeIdxLocked = true

			_, ok := excludeNodes[rv.nodeIdInt]
			if ok {
				// More than one component RVs for an MV cannot come from the same node.
				continue
			}

			if rv.fdId != -1 {
				_, ok = excludeFaultDomains[rv.fdId]
				if ok {
					// More than one component RVs for an MV cannot come from the same fault domain.
					continue
				}
			}

			if rv.udId != -1 {
				_, ok = excludeUpdateDomains[rv.udId]
				if ok {
					// More than one component RVs for an MV cannot come from the same update domain.
					continue
				}
			}

			if _, exists := existingMVMap[mvName]; !exists {
				// First component RV being added to mvName.
				rvwithstate := make(map[string]dcache.StateEnum)
				rvwithstate[rv.rvName] = dcache.StateOnline
				// Create a new MV.
				existingMVMap[mvName] = dcache.MirroredVolume{
					RVs:   rvwithstate,
					State: dcache.StateOnline,
				}
			} else {
				// Subsequent component RVs being added to mvName.
				existingMVMap[mvName].RVs[rv.rvName] = dcache.StateOnline
				common.Assert(len(existingMVMap[mvName].RVs) <= NumReplicas)
			}

			//
			// We decrease the slot count for the RV in availableRVsMap, only after a successful
			// joinMV() call. Note that it's ok to defer slot count adjustment as one RV will
			// be used not more than once as component RV for an MV.
			//

			// New MV ready.
			if len(existingMVMap[mvName].RVs) == NumReplicas {
				break
			}

			//
			// Subsequent component RVs for this MV cannot be from the same node or fault domain.
			//
			excludeNodes[cm.UUIDToUniqueInt(rv.nodeId)] = struct{}{}
			if rv.fdId != -1 {
				excludeFaultDomains[rv.fdId] = struct{}{}
			}
			if rv.udId != -1 {
				excludeUpdateDomains[rv.udId] = struct{}{}
			}
		}

		//
		// If we could not find enough component RVs for this MV, we won't find for any other MV, so stop
		// attempting to create more new MVs.
		//
		if len(existingMVMap[mvName].RVs) != NumReplicas {
			log.Debug("ClusterManager::updateMVList: Could not place %s, numUsableMVs: %d, maxMVsPossible: %d",
				mvName, numUsableMVs, maxMVsPossible)
			// Delete the incomplete MV from the existingMVMap.
			delete(existingMVMap, mvName)
			break
		}

		if firstFreeIdx > 0 {
			// Chop off unusable RVs from the beginning, to avoid wasted iterations for subsequent MVs.
			availableRVsList = availableRVsList[firstFreeIdx:]

			log.Debug("ClusterManager::updateMVList: %d (of %d) new MVs created, initial %d RVs are full, removing from availableRVsList, %d RVs remaining",
				numUsableMVs, maxMVsPossible, firstFreeIdx, len(availableRVsList))
		}

		common.Assert(len(existingMVMap[mvName].RVs) == NumReplicas,
			mvName, len(existingMVMap[mvName].RVs), NumReplicas)

		//
		// Call joinMV() and check if all component RVs are able to join successfully.
		// reserveBytes is 0 for a new-mv workflow.
		//
		// Iff joinMV() is successful, consume one slot for each component RV, else if joinMV() fails
		// delete the failed RV from availableRVsMap to prevent this RV from being picked again and failing.
		// Also we need to remove mv from existingMVMap.
		//
		failedRVs, err := cmi.joinMV(mvName, existingMVMap[mvName])
		if err == nil {
			common.Assert(len(failedRVs) == 0, failedRVs)
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
			atomic.AddInt64(&stats.Stats.CM.NewMV.NewMVsAdded, 1)
			// New MV always starts as online.
			atomic.AddInt64((*int64)(&stats.Stats.CM.NewMV.OnlineMVs), 1)
			stats.Stats.CM.NewMV.LastMVAddedAt = time.Now()
			// Time taken by new-mv workflow.
			newMVDuration := stats.Duration(time.Since(startNewMV))
			atomic.StoreInt64((*int64)(&stats.Stats.CM.NewMV.TimeTaken), int64(newMVDuration))
		} else {
			// TODO: Give up reallocating RVs after a few failed attempts.
			log.Err("ClusterManager::updateMVList: Error joining RV(s) %v with MV %s: %v",
				failedRVs, mvName, err)

			deleteRVsFromAvailableMap(failedRVs)
			// Delete the MV from the existingMVMap.
			delete(existingMVMap, mvName)
		}

		if time.Now().After(completeBy) {
			log.Warn("ClusterManager::updateMVList: Prematurely exiting new-mv numUsableMVs: %d, maxMVsPossible: %d [%s]",
				numUsableMVs, maxMVsPossible, completeBy)
			break
		}
	}

	// Call getAvailableRVsList() to get numAvailableNodes for the stats.
	getAvailableRVsList(true /* newMV */)

	atomic.StoreInt64(&stats.Stats.CM.NewMV.AvailableNodes, int64(numAvailableNodes))

	// Total time taken by updateMVList().
	duration := stats.Duration(time.Since(start))

	atomic.AddInt64(&stats.Stats.CM.UpdateMVList.Calls, 1)
	stats.Stats.CM.UpdateMVList.LastCallAt = time.Now()
	atomic.AddInt64((*int64)(&stats.Stats.CM.UpdateMVList.TotalTime), int64(duration))
	if stats.Stats.CM.UpdateMVList.MinTime == nil ||
		duration < *stats.Stats.CM.UpdateMVList.MinTime {
		stats.Stats.CM.UpdateMVList.MinTime = &duration
	}
	stats.Stats.CM.UpdateMVList.MaxTime =
		max(stats.Stats.CM.UpdateMVList.MaxTime, duration)

	atomic.StoreInt64(&stats.Stats.CM.NewMV.TotalRVs, int64(len(rvMap)))
	atomic.StoreInt64(&stats.Stats.CM.NewMV.TotalMVs, int64(len(existingMVMap)))

	// Count MV state for stats.
	atomic.StoreInt64(&stats.Stats.CM.NewMV.OnlineMVs, 0)
	atomic.StoreInt64(&stats.Stats.CM.NewMV.OfflineMVs, 0)
	atomic.StoreInt64(&stats.Stats.CM.NewMV.DegradedMVs, 0)
	atomic.StoreInt64(&stats.Stats.CM.NewMV.SyncingMVs, 0)
	for mvName, mv := range existingMVMap {
		switch mv.State {
		case dcache.StateOnline:
			atomic.AddInt64((*int64)(&stats.Stats.CM.NewMV.OnlineMVs), 1)
		case dcache.StateOffline:
			atomic.AddInt64((*int64)(&stats.Stats.CM.NewMV.OfflineMVs), 1)
		case dcache.StateDegraded:
			atomic.AddInt64((*int64)(&stats.Stats.CM.NewMV.DegradedMVs), 1)
		case dcache.StateSyncing:
			atomic.AddInt64((*int64)(&stats.Stats.CM.NewMV.SyncingMVs), 1)
		default:
			_ = mvName
			common.Assert(false, mvName, mv.State)
		}
	}

	log.Debug("[TIMING] ClusterManager::updateMVList: existingMVMap after phase#3 (%d RVs, %d MVs) %+v [took %s]",
		len(rvMap), len(existingMVMap), existingMVMap, time.Since(start))
}

// Given an MV, send JoinMV or UpdateMV RPC to all its component RVs. It fails if any of the RV fails the call.
// This must be called from new-mv or fix-mv workflow to let the component RVs know about the new membership details.
// It calls JoinMV for RVs joining the MV newly and UpdateMV for existing component RVs which need to be informed of
// the updated membership details. For JoinMV RPC requests it sets the ReserveSpace to reserveBytes.
// The caller must have updated 'mv' with the correct component RVs and their state before calling this.
//
// It sends JoinMV/UpdateMV based on following:
//   - It sends JoinMV RPC to all RVs of a new MV. A new MV is one which has state of online, because we will not be
//     called o/w for an online MV.
//   - For existing MVs, it sends JoinMV for those RVs which have StateOutOfSync state. These are new RVs selected by
//     fix-mv workflow.
//   - For existing MVs, it sends UpdateMV for online component RVs.
func (cmi *ClusterManager) joinMV(mvName string, mv dcache.MirroredVolume) ([]string, error) {
	log.Debug("ClusterManager::joinMV: JoinMV(%s, %+v)", mvName, mv)

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

	//
	// 'reserveBytes' is the amount of space to reserve in the RV. This will be 0 when joinMV()
	// is called from the new-mv workflow, but can be non-zero when called from the fix-mv workflow
	// for replacing an offline RV with a new good RV. The new RV must need enough space to store
	// the chunks for this MV.
	//
	var reserveBytes int64
	var err error

	if !newMV {
		//
		// Get the reserveBytes correctly, querying it from our in-core RV info maintained by RPC server.
		//
		reserveBytes, err = rm.GetMVSize(mvName)
		if err != nil {
			err = fmt.Errorf("failed to get disk usage of %s [%v]", mvName, err)
			log.Err("ClusterManager::joinMV: %v", err)
			common.Assert(false, err)
			// TODO: return error. Skipping it now because the caller of joinMV() expects failed RVs
			// along with the error. So, the error handling part of the caller must be updated to handle
			// the error returned in this case as below.
			// return "", err
		}
	}

	log.Debug("ClusterManager::joinMV: %s, state: %s, new-mv: %v, reserve bytes: %d",
		mvName, string(mv.State), newMV, reserveBytes)

	// reserveBytes must be non-zero only for degraded MV, for new-mv it'll be 0.
	common.Assert(reserveBytes == 0 || mv.State == dcache.StateDegraded, reserveBytes, mv.State)

	// For all component RVs, we need to send JoinMV/UpdateMV RPC.
	for rvName, rvState := range mv.RVs {
		log.Debug("ClusterManager::joinMV: Populating componentRVs list MV %s with RV %s (%s)",
			mvName, rvName, rvState)

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

	// Struct to hold the status from each RPC call to a component RV.
	type rpcCallComponentRVError struct {
		rvName string
		err    error
	}
	errCh := make(chan rpcCallComponentRVError, len(componentRVs))

	//
	// Note: If JoinMV RPC fails to any RV, we do not send LeaveMV() to the RVs which succeeded, for undoing the
	//       reserveBytes, instead server is supposed to correctly undo that after timeout.
	//
	startTime := time.Now()
	var wg sync.WaitGroup
	for _, rv := range componentRVs {
		rvName := rv.Name
		rvState := mv.RVs[rvName]
		//
		// Offline/inband-offline component RVs need not be sent JoinMV/UpdateMV RPC.
		// TODO: Shall we send them LeaveMV RPC?
		//
		if rvState == dcache.StateOffline || rvState == dcache.StateInbandOffline {
			continue
		}

		wg.Add(1)
		go func(rvName string, rvState dcache.StateEnum) {
			defer wg.Done()
			log.Debug("ClusterManager::joinMV: Joining MV %s with RV %s in state %s", mvName, rvName, rvState)

			joinMvReq := &models.JoinMVRequest{
				MV:           mvName,
				RVName:       rvName,
				ReserveSpace: reserveBytes,
				ComponentRV:  componentRVs,
			}

			updateMvReq := &models.UpdateMVRequest{
				MV:          mvName,
				RVName:      rvName,
				ComponentRV: componentRVs,
			}

			// TODO: Use timeout from some global variable.
			timeout := 10 * time.Second
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()

			var err error
			var action string
			var duration stats.Duration

			start := time.Now()
			if newMV || rvState == dcache.StateOutOfSync {
				//
				// All RVs of a new MV are sent JoinMV RPC.
				// else for fix-mv case outofsync component RVs are sent JoinMV RPC.
				//
				_, err = rpc_client.JoinMV(ctx, cm.RVNameToNodeId(rvName), joinMvReq)
				action = "joining"
				duration = stats.Duration(time.Since(start))

				if newMV {
					atomic.AddInt64(&stats.Stats.CM.NewMV.JoinMV.Calls, 1)
					atomic.AddInt64((*int64)(&stats.Stats.CM.NewMV.JoinMV.TotalTime),
						int64(duration))
					if stats.Stats.CM.NewMV.JoinMV.MinTime == nil ||
						duration < *stats.Stats.CM.NewMV.JoinMV.MinTime {
						stats.Stats.CM.NewMV.JoinMV.MinTime = &duration
					}
					stats.Stats.CM.NewMV.JoinMV.MaxTime =
						max(stats.Stats.CM.NewMV.JoinMV.MaxTime, duration)
				} else {
					atomic.AddInt64(&stats.Stats.CM.FixMV.JoinMV.Calls, 1)
					atomic.AddInt64(&stats.Stats.CM.FixMV.JoinMV.CallsCumulative, 1)
					atomic.AddInt64((*int64)(&stats.Stats.CM.FixMV.JoinMV.TotalTime),
						int64(duration))
					if stats.Stats.CM.FixMV.JoinMV.MinTime == nil ||
						duration < *stats.Stats.CM.FixMV.JoinMV.MinTime {
						stats.Stats.CM.FixMV.JoinMV.MinTime = &duration
					}
					stats.Stats.CM.FixMV.JoinMV.MaxTime =
						max(stats.Stats.CM.FixMV.JoinMV.MaxTime, duration)
				}
			} else {
				//
				// Else, fix-mv and online/syncing RV, send UpdateMV.
				//
				common.Assert(rvState == dcache.StateOnline ||
					rvState == dcache.StateSyncing, rvName, rvState)
				_, err = rpc_client.UpdateMV(ctx, cm.RVNameToNodeId(rvName), updateMvReq)
				action = "updating"
				duration = stats.Duration(time.Since(start))

				atomic.AddInt64(&stats.Stats.CM.FixMV.UpdateMV.Calls, 1)
				atomic.AddInt64(&stats.Stats.CM.FixMV.UpdateMV.CallsCumulative, 1)
				atomic.AddInt64((*int64)(&stats.Stats.CM.FixMV.UpdateMV.TotalTime),
					int64(duration))
				if stats.Stats.CM.FixMV.UpdateMV.MinTime == nil ||
					duration < *stats.Stats.CM.FixMV.UpdateMV.MinTime {
					stats.Stats.CM.FixMV.UpdateMV.MinTime = &duration
				}
				stats.Stats.CM.FixMV.UpdateMV.MaxTime =
					max(stats.Stats.CM.FixMV.UpdateMV.MaxTime, duration)
			}

			if err != nil {
				err = fmt.Errorf("error %s MV %s with RV %s in state %s: %v",
					action, mvName, rvName, rvState, err)
				log.Err("ClusterManager::joinMV: %v", err)
				errCh <- rpcCallComponentRVError{
					rvName: rvName,
					err:    err,
				}

				if newMV {
					atomic.AddInt64(&stats.Stats.CM.NewMV.JoinMV.Failures, 1)
					stats.Stats.CM.NewMV.JoinMV.LastError = err.Error()
				} else if rvState == dcache.StateOutOfSync {
					atomic.AddInt64(&stats.Stats.CM.FixMV.JoinMV.Failures, 1)
					atomic.AddInt64(&stats.Stats.CM.FixMV.JoinMV.FailuresCumulative, 1)
					stats.Stats.CM.FixMV.JoinMV.LastError = err.Error()
				} else {
					atomic.AddInt64(&stats.Stats.CM.FixMV.UpdateMV.Failures, 1)
					atomic.AddInt64(&stats.Stats.CM.FixMV.UpdateMV.FailuresCumulative, 1)
					stats.Stats.CM.FixMV.UpdateMV.LastError = err.Error()
				}
				return
			}
			log.Debug("ClusterManager::joinMV: Success %s MV %s with RV %s in state %s",
				action, mvName, rvName, rvState)

			//
			// A fix-mv/new-mv can only succeed when all the RVs correctly update the component RVs state in their
			// respective mvInfo and the state change is committed in the clustermap. Since our state change is
			// not transactional, each RV holds an mvInfo state change till some timeout period and if the clustermap
			// state change is not observed till the timeout, it assumes that the sender failed to commit and rolls
			// back the mvInfo state change. We need to make sure the first RV and all other RVs to which we sent
			// JoinMV have not timed out their mvInfo state change. We will have some margin for caller to update the
			// clustermap.
			//
			if time.Since(startTime) > rpc_server.GetMvInfoTimeout() {
				errStr := fmt.Sprintf("JoinMV (action: %s, new-mv: %v) for %s/%s took longer than %s, aborting joinMV",
					action, newMV, rvName, mvName, rpc_server.GetMvInfoTimeout())
				log.Err("ClusterManager::joinMV: %s", errStr)
				common.Assert(false, errStr)
				// TODO: This RV is not necessarily the real culprit.
				errCh <- rpcCallComponentRVError{rvName: rvName, err: fmt.Errorf("%s", errStr)}
			}
		}(rvName, rvState)
	}
	wg.Wait()
	close(errCh)

	var allErrs []string
	var failedRVs []string

	for errRes := range errCh {
		allErrs = append(allErrs, errRes.err.Error())
		failedRVs = append(failedRVs, errRes.rvName)
	}

	//
	// Error from any JoinMV/UpdateMV RPC call is considered a failure and we return the list of failed RVs.
	//
	if len(allErrs) > 0 {
		return failedRVs, fmt.Errorf("ClusterManager::joinMV: errors:\n%s", strings.Join(allErrs, "\n"))
	}

	return nil, nil
}

// updateRVList updates the RV list in clustermap based on the heartbeats received from nodes in the
// cluster.
//
// It has two modes of operation (controlled by initialHB):
// 1. initialHB=true:
//    This is called from updateStorageClusterMapWithMyRVs() when the node starts up and wants to add its RVs
//    to the clusterMap for the first time. As an optimization it not only adds its own RVs but also RVs
//    from other nodes which have sent InitialHB=true heartbeats in close proximity. This helps reduce cluster
//    startup time for very large clusters.
//    In this mode it can add new RVs or remove some old RVs but doesn't change the state of any RV in
//    existingRVMap.
//    This is called only once.
// 2. initialHB=false:
//    This is the more common mode and is called from the periodic updateStorageClusterMapIfRequired(), to
//    update RV states based on the heartbeats received from their corresponding nodes.
//    In this mode it doesn't add (or remove) any new RVs to the clusterMap, but it can mark some RV as
//    offline.
//
// Any update to existingRVMap is done in-place.
//
// Note: updateRVList() MUST be called after successfully claiming ownership of clusterMap update, by a successful
//       call to UpdateClusterMapStart().

func (cmi *ClusterManager) updateRVList(existingRVMap map[string]dcache.RawVolume, initialHB bool) (bool, error) {
	hbTillNodeDown := int64(cmi.config.HeartbeatsTillNodeDown)
	hbSeconds := int64(cmi.config.HeartbeatSeconds)

	start := time.Now()
	stats.Stats.CM.Heartbeats.GetNodeList.LastCallAt = start
	atomic.AddInt64(&stats.Stats.CM.Heartbeats.GetNodeList.Calls, 1)

	// Get all nodes by enumerating all the HBs from Nodes/ folder.
	nodeIds, err := getAllNodes()
	if err != nil {
		err1 := fmt.Errorf("ClusterManager::updateRVList: getAllNodes() failed: %v", err)
		common.Assert(false, err1)
		atomic.AddInt64(&stats.Stats.CM.Heartbeats.GetNodeList.Failures, 1)
		stats.Stats.CM.Heartbeats.GetNodeList.LastError = err1.Error()
		return false, err1
	}

	duration := stats.Duration(time.Since(start))

	atomic.AddInt64((*int64)(&stats.Stats.CM.Heartbeats.GetNodeList.TotalTime), int64(duration))
	if stats.Stats.CM.Heartbeats.GetNodeList.MinTime == nil ||
		duration < *stats.Stats.CM.Heartbeats.GetNodeList.MinTime {
		stats.Stats.CM.Heartbeats.GetNodeList.MinTime = &duration
	}

	// TODO: Later we can make this debug only.
	if duration > stats.Stats.CM.Heartbeats.GetNodeList.MaxTime {
		log.Warn("[TIMING] ClusterManager::updateRVList: Got %d nodes in cluster, took %s",
			len(nodeIds), time.Since(start))
	}

	stats.Stats.CM.Heartbeats.GetNodeList.MaxTime =
		max(stats.Stats.CM.Heartbeats.GetNodeList.MaxTime, duration)

	atomic.StoreInt64(&stats.Stats.CM.Heartbeats.GetNodeList.TotalNodes, int64(len(nodeIds)))

	log.Debug("ClusterManager::updateRVList: Found %d nodes in cluster (initialHB=%v), now start collecting heartbeats: %+v",
		len(nodeIds), initialHB, nodeIds)

	//
	// Fetch heartbeats for the given nodeIds but only those matching the initialHB flag.
	// rVsByRvIdFromHB and rvLastHB are maps are indexed by RV id, while nodes and failedToReadNodes
	// are list of node ids.
	// nodes is the list of nodes for which we successfully read the heartbeats matching the initialHB flag.
	// This will be nil for non-initialHB calls.
	//
	start = time.Now()
	stats.Stats.CM.Heartbeats.CollectHB.LastCallAt = start
	atomic.AddInt64(&stats.Stats.CM.Heartbeats.CollectHB.Calls, 1)

	rVsByRvIdFromHB, rvLastHB, nodes, failedToReadNodes, err := collectHBForGivenNodeIds(nodeIds, initialHB)
	_ = nodes
	if err != nil {
		atomic.AddInt64(&stats.Stats.CM.Heartbeats.CollectHB.Failures, 1)
		stats.Stats.CM.Heartbeats.CollectHB.LastError = err.Error()
		return false, err
	}

	duration = stats.Duration(time.Since(start))

	atomic.AddInt64((*int64)(&stats.Stats.CM.Heartbeats.CollectHB.TotalTime), int64(duration))
	if stats.Stats.CM.Heartbeats.CollectHB.MinTime == nil ||
		duration < *stats.Stats.CM.Heartbeats.CollectHB.MinTime {
		stats.Stats.CM.Heartbeats.CollectHB.MinTime = &duration
	}
	stats.Stats.CM.Heartbeats.CollectHB.MaxTime =
		max(stats.Stats.CM.Heartbeats.CollectHB.MaxTime, duration)

	if initialHB {
		log.Debug("ClusterManager::updateRVList: Collected %d RVs from %d nodes (initialHB), failed to read HB for %d nodes: %+v",
			len(rVsByRvIdFromHB), len(nodes), len(failedToReadNodes), failedToReadNodes)
	} else {
		log.Debug("ClusterManager::updateRVList: Collected %d RVs from %d nodes, failed to read HB for %d nodes: %+v",
			len(rVsByRvIdFromHB), len(nodeIds), len(failedToReadNodes), failedToReadNodes)
	}

	// Both the RV and the RV HB map must have the exact same RVs.
	common.Assert(len(rVsByRvIdFromHB) == len(rvLastHB), len(rVsByRvIdFromHB), len(rvLastHB))
	common.Assert(len(nodes)+len(failedToReadNodes) <= len(nodeIds), nodes, failedToReadNodes, nodeIds)

	//
	// This is later set to true if existingRVMap is changed in any way, i.e., we add any new RV to
	// existingRVMap (for initialHB=true) or update the state of any existing RV (for initialHB=false).
	//
	changed := false

	//
	// We process the heartbeats differently based on the initialHB flag:
	// 1. If initialHB is true, we are only interested in adding new RVs to existingRVMap.
	//    We first remove all RVs belonging to any node for which we see an initial HB.
	//    This node is just getting started so any RV from this node if present in existingRVMap
	//    is stale and must be removed.
	//    Post that we add RVs seen from all nodes that posted initial HB.
	// 2. If initialHB is false, we are interested in updating the state of existing RVs in existingRVMap,
	//    offlining RVs which have expired or missing heartbeats.
	//
	if !initialHB {
		// If an RV has the LastHeartbeat less than hbExpiry, it needs to be offlined.
		now := uint64(time.Now().Unix())
		hbExpiry := now - uint64(hbTillNodeDown*hbSeconds)

		atomic.StoreInt64(&stats.Stats.CM.Heartbeats.CollectHB.NumNodes, int64(len(nodeIds)))
		atomic.StoreInt64(&stats.Stats.CM.Heartbeats.CollectHB.NumRVs, int64(len(rVsByRvIdFromHB)))
		atomic.StoreInt64(&stats.Stats.CM.Heartbeats.CollectHB.Expired, 0)

		// Update RVs present in existingRVMap and which have changed State or AvailableSpace.
		for rvName, rvInClusterMap := range existingRVMap {
			if rvHb, found := rVsByRvIdFromHB[rvInClusterMap.RvId]; found {
				lastHB, found := rvLastHB[rvHb.RvId]
				_ = found

				// If an RV is present in rVsByRvIdFromHB, it MUST have a valid HB in rvLastHB.
				common.Assert(found)

				if lastHB < hbExpiry {
					atomic.AddInt64(&stats.Stats.CM.Heartbeats.CollectHB.Expired, 1)
					atomic.AddInt64(&stats.Stats.CM.Heartbeats.CollectHB.ExpiredCumulative, 1)
					if rvInClusterMap.State != dcache.StateOffline {
						log.Warn("ClusterManager::updateRVList: Online RV %s %+v lastHeartbeat (%d) has expired, hbExpiry (%d), marking RV offline",
							rvName, rvInClusterMap, lastHB, hbExpiry)
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
			} else if slices.Contains(failedToReadNodes, rvInClusterMap.NodeId) {
				//
				// If we failed to read the HB for the node this indicates some unusual problem with the storage,
				// since we only fetch hbs from nodes returned by getAllNodes() which actually enumerates all the
				// hbs in the Nodes/ folder.
				// Play safe and skip this RV for now. If its hb is indeed missing, it will be removed in the next
				// iteration of updateRVList() call.
				//
				err1 := fmt.Errorf("ClusterManager::updateRVList: Online Rv %s %+v missing in heartbeats (could not read HB for node %s), ignoring for now",
					rvName, rvInClusterMap, rvInClusterMap.NodeId)
				log.Warn("%v", err1)
				stats.Stats.CM.Heartbeats.CollectHB.LastError = err1.Error()
			} else {
				//
				// This can happen when an HB file is deleted out-of-band.
				// This can also happen when a node is restarted and an older RV is now excluded from the
				// node's RV list published in the initial heartbeat. If the RV is not being used by any MV
				// then we will remove it from the existingRVMap, but if the old RV is being used as a component
				// RV by some MV, it will not be removed by updateRVList(). Since it won't be present in later
				// heartbeats, we will reach here.
				//
				//common.Assert(false, "HB missing for RV in clustermap", rvInClusterMap)

				//
				// RV present in existingRVMap, but missing from rVsByRvIdFromHB.
				// This is not a common occurrence, emit a warning log.
				//
				if rvInClusterMap.State != dcache.StateOffline {
					err1 := fmt.Errorf("ClusterManager::updateRVList: Online Rv %s %+v missing in heartbeats, did you delete the hb file out-of-band?", rvName, rvInClusterMap)
					log.Warn("%v", err1)
					stats.Stats.CM.Heartbeats.CollectHB.LastError = err1.Error()
					rvInClusterMap.State = dcache.StateOffline
					existingRVMap[rvName] = rvInClusterMap
					changed = true
				}
			}
		}

		return changed, nil
	}

	//
	// initialHB=true
	//
	// We need to perform the following tasks:
	// 1. Remove any stale RVs belonging to nodes for which we are adding new RVs from their initial heartbeats.
	//    A stale RV is one which is present in existingRVMap but not present in the RV list published by
	//    its owning node in the (recent) initial heartbeat. These are RVs from the previous incarnation
	//    of the node, not present anymore, and must be removed.
	// 2. Add new RVs posted by nodes in their initial heartbeats, to existingRVMap.
	//    While doing this we ensure the following:
	//    a. If the new RV is not already present in existingRVMap, we assign a unique name to the RV and
	//       add it. The unique name is the first available RV name.
	//       This is the common case for a new cluster.
	//    b. If the new RV is already present in existingRVMap, but with a different NodeId, this is a
	//       case of duplicate RV Id being used by two different nodes. We skip this RV and let the
	//       existing RV be used by the existing node.
	//    c. If the new RV is already present in existingRVMap, and with the same NodeId, we simply
	//       reuse the existing RV name and update it with the new RV details.
	//       This is the common case for a node restarting and posting the same RVs again.
	//

	// We must process initial heartbeats only once.
	common.Assert(stats.Stats.CM.Heartbeats.InitialHB.NumNodes == 0, stats.Stats.CM.Heartbeats.InitialHB.NumNodes)
	atomic.StoreInt64(&stats.Stats.CM.Heartbeats.InitialHB.NumNodes, int64(len(nodes)))
	atomic.StoreInt64(&stats.Stats.CM.Heartbeats.InitialHB.NumRVs, int64(len(rVsByRvIdFromHB)))
	stats.Stats.CM.Heartbeats.InitialHB.LastCallAt = time.Now()

	//
	// Nothing to add.
	// This should not happen since we must have at least our heartbeat.
	//
	if len(rVsByRvIdFromHB) == 0 {
		common.Assert(false)
		return false, nil
	}

	// If we have some heartbeat, they must have come from some node.
	common.Assert(len(nodes) > 0)

	// Helper struct to hold the RV name and the corresponding RawVolume.
	type nameAndRV struct {
		rvName string
		rv     *dcache.RawVolume
	}
	// Map to hold the existing RVs by their RVId for faster lookup.
	var existingRVMapByRvId map[string]*nameAndRV

	//
	// Helper function to search for an RV by its RVId in existingRVMap.
	//
	getExistingRvByRvId := func(rvId string) *nameAndRV {
		// First call to this function will initialize the map.
		if existingRVMapByRvId == nil {
			existingRVMapByRvId = make(map[string]*nameAndRV)
			for rvName, rv := range existingRVMap {
				existingRVMapByRvId[rv.RvId] = &nameAndRV{rvName, &rv}
			}

			common.Assert(len(existingRVMapByRvId) == len(existingRVMap),
				len(existingRVMapByRvId), len(existingRVMap))
		}

		if namedRV, exists := existingRVMapByRvId[rvId]; exists {
			return namedRV
		}

		return nil
	}

	//
	// (1) Remove stale RVs.
	//

	// list->map for faster lookup in the following loop.
	nodeIdMap := make(map[string]struct{})
	for _, nodeId := range nodes {
		nodeIdMap[nodeId] = struct{}{}
	}
	// nodes[] must have unique node ids.
	common.Assert(len(nodeIdMap) == len(nodes), len(nodeIdMap), len(nodes))

	// Get all the component RVs in the clusterMap, to perform "is RV in use" check below.
	allComponentRVs := cm.GetAllComponentRVs()

	for rvName, rvInClusterMap := range existingRVMap {
		// Only look for stale RVs that belong to the nodes for which we are adding new RVs.
		if _, found := nodeIdMap[rvInClusterMap.NodeId]; !found {
			continue
		}

		//
		// If the RV Id is present in the latest heartbeats, it is not stale.
		// It can be duplicate but that we will check later.
		//
		if _, exists := rVsByRvIdFromHB[rvInClusterMap.RvId]; exists {
			continue
		}

		//
		// If the RV is being used as a component RV to some MV, we cannot remove it.
		// This is regardless of the state of the component RV, as we refer to RV map
		// even for finding the state of the RV. This will soon be replaced by some
		// other RV and then we can remove it from the clusterMap.
		//
		if _, exists := allComponentRVs[rvName]; exists {
			log.Warn("ClusterManager::updateRVList: Stale RV %s %+v still used by %d MV(s), not removing it",
				rvName, rvInClusterMap, allComponentRVs[rvName])
			continue
		}

		log.Warn("ClusterManager::updateRVList: Removing stale RV %s %+v belonging to node %s",
			rvName, rvInClusterMap, rvInClusterMap.NodeId)

		delete(existingRVMap, rvName)
		changed = true

		atomic.AddInt64(&stats.Stats.CM.Heartbeats.InitialHB.StaleRVsRemoved, 1)
	}

	// Now add all the new RVs from initial heartbeats seen from all nodes.
	log.Info("ClusterManager::updateRVList: %d new RV(s) to add to clusterMap: %+v",
		len(rVsByRvIdFromHB), rVsByRvIdFromHB)

	//
	// Return next free RV index after (not including) lastIdx.
	//
	getNextFreeRVIdx := func(lastIdx int64) int64 {
		idx := lastIdx + 1
		for {
			rvName := fmt.Sprintf("rv%d", idx)
			if _, exists := existingRVMap[rvName]; !exists {
				return idx
			}
			idx++
		}
	}

	//
	// Add new RV(s) into clusterMap, starting from the next available index (starting from 0).
	// Since we may remove old RVs not being exported this time by the node, we may have some
	// gaps in the RV index space. We pick the first available RV index.
	//
	nextFreeIdx := int64(-1)
	for _, rv := range rVsByRvIdFromHB {
		//
		// If the RV is already present in existingRVMap, use the same rvName.
		// This can happen if the node added its RVs but since the initialHB was lying around we
		// also decided to add RVs for that node.
		// It could also be a case of the node restarting and adding the same RV again (which is
		// a common case).
		//
		namedRV := getExistingRvByRvId(rv.RvId)
		if namedRV != nil {
			//
			// (2.b) Duplicate RV Id used by two different nodes.
			//       Drop the new RV and keep the existing one.
			//
			if namedRV.rv.NodeId != rv.NodeId {
				err1 := fmt.Errorf("ClusterManager::updateRVList: RVId %s from node %s conflicts with existing RV %s from node %s, skipping",
					rv.RvId, rv.NodeId, namedRV.rvName, namedRV.rv.NodeId)
				log.Warn("%v", err1)
				atomic.AddInt64(&stats.Stats.CM.Heartbeats.InitialHB.DuplicateRVIds, 1)
				stats.Stats.CM.Heartbeats.InitialHB.LastError = err1.Error()
				continue
			}

			//
			// (2.c) RV Id reused by the same node.
			//       Update the existing RV with the new details.
			//
			log.Debug("ClusterManager::updateRVList: Repurpose already present RV %s (%+v)",
				namedRV.rvName, *namedRV.rv)

			existingRVMap[namedRV.rvName] = rv
			changed = true
			continue
		}

		//
		// (2.a) New RV Id, not present in existingRVMap.
		//       Assign a new RV name and add it to existingRVMap.
		//
		nextFreeIdx = getNextFreeRVIdx(nextFreeIdx)
		rvName := fmt.Sprintf("rv%d", nextFreeIdx)
		existingRVMap[rvName] = rv
		changed = true
		log.Info("ClusterManager::updateRVList: Adding new RV %s to cluster map: %+v", rvName, rv)

		atomic.AddInt64(&stats.Stats.CM.Heartbeats.InitialHB.NewRVsAdded, 1)
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

// For all the nodes in nodeIds, fetch their latest heartbeats, skip those heartbeats which do not match
// the initialHB flag, and for the ones that do match, collect the RVs and their last heartbeat.
//
// It returns the following:
//   - All the RVs present in those nodes.
//   - The last heartbeat epoch for each of those RVs.
//   - List of nodeIds for which the heartbeat was found.
//     This is only valid if initialHB is true, else it is empty.
//   - List of nodeIds for which the heartbeat could not be fetched.
//
// The first two are returned as maps indexed by RVId.
//
// Note: It fetches the heartbeat for multiple nodes in parallel, currently limited to 100 parallel calls.
func collectHBForGivenNodeIds(nodeIds []string, initialHB bool) (map[string]dcache.RawVolume, map[string]uint64, []string, []string, error) {
	// Status result struct to hold the result from each goroutine.
	type rvHBResult struct {
		// NodeId for which the heartbeat (matching initialHB) was fetched.
		nodeId string
		// Raw volumes, indexed by RVId.
		rvs map[string]dcache.RawVolume
		// Last heartbeat epoch for the raw volume, indexed by RVId.
		hbs map[string]uint64
	}
	// Results channel to collect data from each goroutine.
	resultCh := make(chan rvHBResult, len(nodeIds))
	errCh := make(chan error, len(nodeIds))
	var wg sync.WaitGroup

	// Limit concurrency to 100 goroutines.
	sem := make(chan struct{}, 100)

	// Nodes from which fetching heartbeat failed.
	mu := &sync.Mutex{}
	failedToReadNodes := make([]string, 0)

	for _, nodeId := range nodeIds {
		wg.Add(1)
		go func(nodeId string) {
			defer wg.Done()

			// Acquire a semaphore slot to limit concurrency.
			sem <- struct{}{}
			defer func() { <-sem }()

			bytes, err := getHeartbeat(nodeId)
			if err != nil {
				common.Assert(false, err)
				errCh <- fmt.Errorf("failed to fetch heartbeat for node %s: %v",
					nodeId, err)

				mu.Lock()
				defer mu.Unlock()
				failedToReadNodes = append(failedToReadNodes, nodeId)
				return
			}

			var hbData dcache.HeartbeatData
			if err := json.Unmarshal(bytes, &hbData); err != nil {
				common.Assert(false, err)
				errCh <- fmt.Errorf("failed to parse heartbeat bytes (%d) for node %s: %v",
					len(bytes), nodeId, err)
				return
			}

			isValidHb, err := cm.IsValidHeartbeat(&hbData)
			if !isValidHb {
				common.Assert(false, err)
				errCh <- fmt.Errorf("invalid heartbeat for node %s: %v",
					nodeId, err)
				return
			}

			//
			// Skip heartbeats not matching the initialHB flag.
			//
			if hbData.InitialHB != initialHB {
				return
			}

			// Process RVs and their last heartbeat from the heartbeat data for this node.
			nodeRVs := make(map[string]dcache.RawVolume)
			nodeHBs := make(map[string]uint64)

			for _, rv := range hbData.RVList {
				nodeRVs[rv.RvId] = rv
				nodeHBs[rv.RvId] = hbData.LastHeartbeat
			}

			// Send result to channel.
			resultCh <- rvHBResult{nodeId: nodeId, rvs: nodeRVs, hbs: nodeHBs}
		}(nodeId)
	}
	wg.Wait()

	close(errCh)
	close(resultCh)

	//
	// Aggregated results from all nodes.
	//
	rVsByRvIdFromHB := make(map[string]dcache.RawVolume)
	rvLastHB := make(map[string]uint64)
	// Nodes which had heartbeat matching initialHB flag.
	successfulNodeIdsMap := make(map[string]struct{})

	// Go over all RVs, and add them to rVsByRvIdFromHB, to be processed later.
	for result := range resultCh {
		for rvId, rv := range result.rvs {
			if existingRv, exists := rVsByRvIdFromHB[rvId]; exists {
				msg := fmt.Sprintf("RVId %s from node %s conflicts with existing RV from node %s",
					rv.RvId, rv.NodeId, existingRv.NodeId)
				log.Err("ClusterManager::collectHBForGivenNodeIds: %s, skipping!", msg)
				common.Assert(false, msg)
				continue
			}

			// For every RVId in result.rvs, we must have the last heartbeat in result.hbs.
			lastHB, ok := result.hbs[rvId]
			_ = ok
			common.Assert(ok, rvId)

			rVsByRvIdFromHB[rvId] = rv
			rvLastHB[rvId] = lastHB

			// This node contributed at least one RV in rVsByRvIdFromHB.
			if initialHB {
				successfulNodeIdsMap[result.nodeId] = struct{}{}
			}
		}
	}

	// Convert to list for returning.
	successfulNodeIds := make([]string, 0)
	if initialHB {
		for nodeId := range successfulNodeIdsMap {
			successfulNodeIds = append(successfulNodeIds, nodeId)
		}
	}

	if len(errCh) > 0 {
		log.Err("ClusterManager::collectHBForGivenNodeIds: Errors encountered while fetching heartbeats:")
		for err := range errCh {
			log.Err("ClusterManager::collectHBForGivenNodeIds: %v", err)
		}
	}

	//
	// If we are able to collect heartbeats for any node, return the fetched heartbeats, else treat the entire
	// operation as failure.
	//
	if len(rVsByRvIdFromHB) == 0 {
		return nil, nil, nil, nil, fmt.Errorf("ClusterManager::collectHBForGivenNodeIds: Could not fetch any HB")
	}

	return rVsByRvIdFromHB, rvLastHB, successfulNodeIds, failedToReadNodes, nil
}

// Refresh AvailableSpace in my RVs.
func refreshMyRVs(myRVs []dcache.RawVolume) {
	for index, rv := range myRVs {
		availableSpace, err := rpc_server.GetAvailableSpaceForRV(rv.RvId, rv.LocalCachePath)
		common.Assert(err == nil, rv, err)
		if err != nil {
			availableSpace = 0

			log.Warn("ClusterManager::refreshMyRVs: Error getting disk space metrics for path %s for punching heartbeat, forcing available space to zero: %v", rv.LocalCachePath, err)
		}

		common.Assert(availableSpace >= 0, rv, availableSpace)
		myRVs[index].AvailableSpace = uint64(availableSpace)
		myRVs[index].State = dcache.StateOnline
	}
}

// Get the next batch of component RV state updates.
// This tries to get as many as it can from the channel with one condition - multiple updates to the same mv/rv
// must not be in the same batch.
func (cmi *ClusterManager) getNextComponentRVUpdateBatch() []*dcache.ComponentRVUpdateMessage {
	msgBatch := []*dcache.ComponentRVUpdateMessage{}

	//
	// Map to track the RV/MV combinations that we have already seen in this batch.
	// Key for this map is "mvName+rvName", value is the new RV state.
	//
	existing := make(map[string]dcache.StateEnum)

	for {
		select {
		case msg, ok := <-cm.GetComponentRVStateChannel():
			if !ok {
				log.Info("ClusterManager::getNextComponentRVUpdateBatch: breaking due to channel closed")
				goto done
			}

			common.Assert(cm.IsValidMVName(msg.MvName), msg.MvName)
			common.Assert(cm.IsValidRVName(msg.RvName), msg.RvName)
			common.Assert(cm.IsValidComponentRVState(msg.RvNewState), msg.RvNewState)
			common.Assert(msg.Err != nil)
			common.Assert(len(msg.Err) == 0, len(msg.Err))

			if rvPrevState, ok := existing[msg.MvName+msg.RvName]; ok {
				//
				// This RV/MV combination has already been added to the batch.
				// We don't support multiple updates to the same RV/MV in the same batch,
				// but if multiple updates are all updating the RV state to the same value,
				// we can safely ignore the duplicate updates.
				// Note that we still need to add the message to the batch, so that the batch
				// updater later correctly notifies the caller waiting for its update to complete
				// (by reading from the msg.Err channel).
				// Later on in the batchUpdateComponentRVState() we will check if there are
				// multiple updates for the same RV/MV combination, we will skip such duplicate
				// updates.
				//
				// Note that this is an optimization, w/o this multiple updates queued by inline
				// write failures (which could be many, since we perform multiple parallel writes
				// to different chunks for the same file) will be processed in different ticks
				// taking lot of time to update the clustermap.
				//
				if rvPrevState == msg.RvNewState {
					msgBatch = append(msgBatch, &msg)
					continue
				}

				//
				// This is not commonly expected, so make it info log.
				// The only dup update we can possibly get is an update to offline state while some other update
				// is pending.
				//
				log.Info("ClusterManager::getNextComponentRVUpdateBatch: breaking due to dup update for %s/%s, %s -> %s",
					msg.RvName, msg.MvName, rvPrevState, msg.RvNewState)

				//
				// Queue this message back to the channel.
				// I'd have loved to queue it to the head, but we cannot do that.
				// Queueing to the tail should also work as all the messages in the channel are added by
				// go routines running simultaneously, so order should not matter.
				//
				cm.GetComponentRVStateChannel() <- msg
				goto done
			}

			msgBatch = append(msgBatch, &msg)
			existing[msg.MvName+msg.RvName] = msg.RvNewState
		default:
			log.Debug("ClusterManager::getNextComponentRVUpdateBatch: breaking due to no more queued messages")
			goto done
		}
	}

done:
	log.Debug("ClusterManager::getNextComponentRVUpdateBatch: added %d update(s) in the batch (%d more queued)",
		len(msgBatch), len(cm.GetComponentRVStateChannel()))

	return msgBatch
}

// This function can be used to update component RV state for multiple MVs at a time. Caller must ensure that the
// changes are non-overlapping, i.e., no two changes refer to the same mv/rv, though changes can refer to different
// RVs of the same MV.
// The function atomically applies the requested updates to the global clustermap (retrying if some other node is
// updating the clustermap simultaneously) and returns only after it's able to successfully make the requested changes,
// or there's some error.
//
// Only following RV state transitions are valid, any other state transitions are failed with an error.
// StateOutOfSync   -> StateSyncing           [Resync start]
// StateSyncing     -> StateOnline            [Resync end]
// StateSyncing     -> StateOutOfSync         [Resync revert]
// StateOutOfSync   -> StateOutOfSync         [Resync defer]
// StateOnline      -> StateOnline            [Resync skip good RVs]
// StateOnline      -> StateInbandOffline     [Inband detection of offline RV during PutChunk(client)]
// StateSyncing     -> StateInbandOffline     [Inband detection of offline RV during PutChunk(sync)]
//
// After setting the component RV states correctly in the clustermap, it calls updateMVList() which will set the
// MV state appropriately. See updateMVList() for how MV state is set based on component RVs state.
//
// Note: If this fails the caller should typically retry after sometime with the refreshed clustermap.
func (cmi *ClusterManager) batchUpdateComponentRVState(msgBatch []*dcache.ComponentRVUpdateMessage) error {
	common.Assert(len(msgBatch) > 0)
	log.Info("ClusterManager::batchUpdateComponentRVState: Received batch of %d component RV updates", len(msgBatch))

	successCount := 0 // How many updates were successfully performed?
	failureCount := 0 // How many failed as the message had some anomaly?
	ignoredCount := 0 // How many were ignored as the requested update was already present?

	startTime := time.Now()
	maxWait := 120 * time.Second

	for {
		// Time check.
		elapsed := time.Since(startTime)
		if elapsed > maxWait {
			common.Assert(false, elapsed, maxWait, len(msgBatch))
			return fmt.Errorf("ClusterManager::batchUpdateComponentRVState: exceeded maxWait for %d updates",
				len(msgBatch))
		}

		// Get most recent clustermap copy, we will make the requested changes and publish it.
		clusterMap, etag, err := cmi.fetchAndUpdateLocalClusterMap()
		if err != nil {
			log.Err("ClusterManager::batchUpdateComponentRVState: fetchAndUpdateLocalClusterMap() failed: %v",
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
			log.Info("ClusterManager::batchUpdateComponentRVState: Clustermap being updated by node %s, waiting a bit before retry",
				clusterMap.LastUpdatedBy)

			// TODO: Add some backoff and randomness?
			time.Sleep(10 * time.Millisecond)
			continue
		}

		//
		// Claim ownership of clustermap.
		// If some other node gets there before us, we retry. Note that we don't add a wait before
		// the retry as that other node is not updating the clustermap, it's done updating.
		// Once we get the ownership, we iterate over all requested changes, validate them, fail the invalid
		// state transitions and apply the valid ones. After making all the changes to the in-core clustermap
		// we then call updateMVList() which will set the MV state correctly, and finally publish the clustermap
		// with a single call.
		//
		// TODO: Check err to see if the failure is due to etag mismatch, if not retrying may not help.
		//
		err = cmi.startClusterMapUpdate(clusterMap, etag)
		if err != nil {
			log.Warn("ClusterManager::batchUpdateComponentRVState: Start Clustermap update failed for nodeId %s: %v, retrying",
				cmi.myNodeId, err)
			continue
		}

		//
		// Map to track the RV/MV combinations that we have already seen in this batch.
		// Key for this map is "mvName+rvName", value is the new RV state.
		// See getNextComponentRVUpdateBatch() for how it can add duplicate updates for the same
		// RV/MV combination to the batch.
		//
		existing := make(map[string]dcache.StateEnum)

		//
		// Now that we have the global clusterMap lock, apply all updates in sequence, taking the clusterMap to
		// a state with all the requested changes.
		// Note that the caller has ensured that no two updates will target the same rv/mv.
		//
		for _, msg := range msgBatch {
			common.Assert(msg != nil)

			mvName := msg.MvName
			rvName := msg.RvName
			rvNewState := msg.RvNewState

			common.Assert(cm.IsValidMVName(mvName), mvName)
			common.Assert(cm.IsValidRVName(rvName), rvName)
			common.Assert(cm.IsValidComponentRVState(rvNewState), rvNewState)
			common.Assert(msg.Err != nil)
			common.Assert(len(msg.Err) == 0, len(msg.Err))

			// Individual component RV state is never moved to offline, but instead to inband-offline.
			common.Assert(rvNewState != dcache.StateOffline, rvNewState)

			// Requested MV must be valid.
			clusterMapMV, found := clusterMap.MVMap[mvName]
			if !found {
				common.Assert(false, *msg)
				msg.Err <- fmt.Errorf("MV %s not found in clusterMap, mvList %+v", mvName, clusterMap.MVMap)
				close(msg.Err)
				msg.Err = nil
				failureCount++
				continue
			}

			// and the RV passed must be a valid component RV for that MV.
			currentState, found := clusterMapMV.RVs[rvName]
			if !found {
				//
				// There's one legitimate case where this can happen:
				// A prior message in the batch updated the RV state to inband-offline, which caused the
				// fix-mv workflow to remove the RV from the MV, and this latter message got a chance to be
				// processed only after the clusterMap update.
				// The fact that the RV is not present in the MV means that the prior message was able to
				// successfully change the RV state to inband-offline, and hence this dup message must be
				// considered as successfully completed.
				//
				if rvNewState == dcache.StateInbandOffline {
					log.Debug("ClusterManager::batchUpdateComponentRVState: %s/%s (%s -> %s) RV no longer present in MV: %+v",
						rvName, mvName, currentState, rvNewState, clusterMapMV.RVs)
					msg.Err <- nil
					close(msg.Err)
					msg.Err = nil
					ignoredCount++
					continue
				}

				common.Assert(false, *msg, clusterMapMV)
				msg.Err <- fmt.Errorf("RV %s/%s not present in clustermap MV %+v", rvName, mvName, clusterMapMV)
				close(msg.Err)
				msg.Err = nil
				failureCount++
				continue
			}

			//
			// If the RV/MV combination is already present in the batch, we can ignore this update as it
			// was already processed in the previous iteration. We do assert that the previous state
			// is same as the new state requested, so that we don't end up with multiple updates for the same
			// RV/MV combination in the batch.
			// We also assert that the new state is same as the current state because the current state would
			// have been updated by the previous update in the batch. So, if we are here it means that the
			// current state is same as the new state requested, so we can ignore this update.
			//
			if rvPrevState, ok := existing[mvName+rvName]; ok {
				_ = rvPrevState

				// We cannot have multiple updates to different states for an RV/MV in the batch.
				// The state updated in the previous iteration must be same as the new state for an RV/MV.
				common.Assert(rvPrevState == rvNewState, rvPrevState, rvNewState)

				// The previous update must have updated the current state. So, we assert that the current
				// state is same as the new state requested.
				common.Assert(currentState == rvNewState, currentState, rvNewState)

				log.Debug("ClusterManager::batchUpdateComponentRVState: %s/%s, ignoring duplicate state change (%s -> %s)",
					rvName, mvName, currentState, rvNewState)
				ignoredCount++
				continue
			}

			//
			// We will reach here only for the first update for this RV/MV combination. The duplicate updates
			// to this RV/MV will be ignored above.
			// The new state requested must be valid.
			// Note that we support only few distinct state transitions.
			//
			if currentState == dcache.StateOutOfSync && rvNewState == dcache.StateSyncing ||
				currentState == dcache.StateSyncing && rvNewState == dcache.StateOnline ||
				currentState == dcache.StateSyncing && rvNewState == dcache.StateOutOfSync ||
				currentState == dcache.StateSyncing && rvNewState == dcache.StateInbandOffline ||
				currentState == dcache.StateOnline && rvNewState == dcache.StateInbandOffline {

				log.Debug("ClusterManager::batchUpdateComponentRVState: %s/%s, state change (%s -> %s)",
					rvName, mvName, currentState, rvNewState)
				successCount++
			} else {
				//
				// Following transitions are reported when an inband PutChunk failure suggests an RV as offline.
				// StateOnline  -> StateInbandOffline
				// StateSyncing -> StateInbandOffline
				//
				// We can have multiple PutChunk requests outstanding where there can be multiple updates for
				// an RV/MV to inband-offline in different batches.
				// The first request in the first batch will update the RV/MV state to inband-offline in the
				// clustermap. The requests in the subsequent batches, wanting to update the RV/MV state to
				// inband-offline, will find the RV/MV state already set to inband-offline in the clustermap.
				// So, we ignore those requests.
				//
				// We can also get this transition,
				// StateOffline -> StateInbandOffline
				// We can get this when the RV is offline and its status has been updated in the clustermap.
				// However in inband PutChunk failure, we send to update the RV state to InbandOffline.
				// Since, the RV is already marked offline in the clustermap, we ignore this request.
				//
				if currentState == rvNewState ||
					(currentState == dcache.StateOffline && rvNewState == dcache.StateInbandOffline) {
					common.Assert(currentState != rvNewState ||
						currentState == dcache.StateInbandOffline, currentState)

					log.Debug("ClusterManager::batchUpdateComponentRVState: %s/%s ignoring state change (%s -> %s)",
						rvName, mvName, currentState, rvNewState)

					msg.Err <- nil
					close(msg.Err)
					msg.Err = nil
					ignoredCount++
					continue
				}

				common.Assert(false, rvName, mvName, currentState, rvNewState)

				msg.Err <- fmt.Errorf("%s/%s invalid state change request (%s -> %s)",
					rvName, mvName, currentState, rvNewState)
				close(msg.Err)
				msg.Err = nil
				failureCount++
				continue
			}

			//
			// Update requested MV in the cluster Map.
			// MV state is not important as it'll be correctly set by updateMVList().
			// We force it to a StateOffline to catch any bug in setting the MV state correctly.
			//
			clusterMapMV.State = dcache.StateOffline
			clusterMapMV.RVs[rvName] = rvNewState
			clusterMap.MVMap[mvName] = clusterMapMV
			existing[mvName+rvName] = rvNewState
		}

		//
		// Call updateMVList() to update MV state.
		// We don't want to run the fix-mv and new-mv workflows from batchUpdateComponentRVState(), even though
		// it's batched and hence the calls are controlled, but it's ok to wait for the next clusterMap epoch.
		// We can pass runFixMvNewMv as true for the inband rv offlining case as those will be fewer, but it's
		// ok to wait for fix-mv till the next clusterMap epoch.
		// With runFixMvNewMv=false updateMVList() should not take much time, but we give it a margin of 30 seconds
		// to be safe (and also it demands min 20 secs).
		//
		// TODO: See if we want to pass runFixMvNewMv as true for the inband rv offlining case.
		//
		cmi.updateMVList(clusterMap.RVMap, clusterMap.MVMap, time.Now().Add(30*time.Second), false /* runFixMvNewMv */)

		err = cmi.endClusterMapUpdate(clusterMap)
		if err != nil {
			log.Err("ClusterManager::batchUpdateComponentRVState: endClusterMapUpdate() failed: %v %+v",
				err, clusterMap)
			common.Assert(false, err)
			return err
		}

		break
	}

	// Update local copy.
	if _, _, err := cmi.fetchAndUpdateLocalClusterMap(); err != nil {
		log.Err("ClusterManager::batchUpdateComponentRVState: fetchAndUpdateLocalClusterMap() failed: %v", err)
		common.Assert(false, err)
	}

	log.Info("ClusterManager::batchUpdateComponentRVState: total: %d, succeeded: %d, failed: %d, ignored: %d",
		len(msgBatch), successCount, failureCount, ignoredCount)

	common.Assert(len(msgBatch) == (successCount+failureCount+ignoredCount),
		len(msgBatch), successCount, failureCount, ignoredCount)
	return nil
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

	// Initialize the clustermap before any of its users.
	cm.Start()

	// Register hook for refreshing the clustermap from the metadata store, through clustermap package.
	cm.RegisterClusterMapRefresher(clusterManager.updateClusterMapLocalCopy)

	return clusterManager.start(dCacheConfig, rvs)
}

func Stop() error {
	common.Assert(clusterManager != nil, "ClusterManager not started")
	return clusterManager.stop()
}
