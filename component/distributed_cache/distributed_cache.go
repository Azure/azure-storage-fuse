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

package distributed_cache

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"syscall"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/bloberror"
	"github.com/Azure/azure-storage-fuse/v2/common/config"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal"
	dcachelib "github.com/Azure/azure-storage-fuse/v2/internal/dcache_lib"
	. "github.com/Azure/azure-storage-fuse/v2/internal/dcache_lib/api"
)

/* NOTES:
   - Component shall have a structure which inherits "internal.BaseComponent" to participate in pipeline
   - Component shall register a name and its constructor to participate in pipeline  (add by default by generator)
   - Order of calls : Constructor -> Configure -> Start ..... -> Stop
   - To read any new setting from config file follow the Configure method default comments
*/

// Common structure for Component
type DistributedCache struct {
	internal.BaseComponent
	cacheID             string
	cachePath           string
	maxCacheSize        uint64
	replicas            uint8
	maxMissedHbs        uint8
	hbDuration          uint16
	chunkSize           uint64
	minNodes            int
	stripeSize          uint64
	mvsPerRv            uint64
	rvFullThreshold     uint64
	rvNearfullThreshold uint64
	clustermapEpoch     uint64
	rebalancePercentage uint64
	safeDeletes         bool
	cacheAccess         string

	azstroage        internal.Component
	clusterManager   ClusterManager
	strorageCallback StorageCallbacks
}

// Structure defining your config parameters
type DistributedCacheOptions struct {
	CacheID             string `config:"cache-id" yaml:"cache-id,omitempty"`
	CachePath           string `config:"cache-path" yaml:"cache-path,omitempty"`
	ChunkSize           uint64 `config:"chunk-size" yaml:"chunk-size,omitempty"`
	StripeSize          uint64 `config:"stripe-size" yaml:"stripe-size,omitempty"`
	MaxCacheSize        uint64 `config:"max-cache-size" yaml:"cache-size,omitempty"`
	Replicas            uint8  `config:"replicas" yaml:"replicas,omitempty"`
	HeartbeatDuration   uint16 `config:"heartbeat-duration" yaml:"heartbeat-duration,omitempty"`
	MaxMissedHeartbeats uint8  `config:"max-missed-heartbeats" yaml:"max-missed-heartbeats,omitempty"`
	MinNodes            int    `config:"min-nodes" yaml:"min-nodes,omitempty"`
	MVsPerRv            uint64 `config:"mvs-per-rv" yaml:"mvs-per-rv,omitempty"`
	RVFullThreshold     uint64 `config:"rv-full-threshold" yaml:"rv-full-threshold,omitempty"`
	RVNearfullThreshold uint64 `config:"rv-nearfull-threshold" yaml:"rv-nearfull-threshold,omitempty"`
	ClustermapEpoch     uint64 `config:"clustermap-epoch" yaml:"clustermap-epoch,omitempty"`
	RebalancePercentage uint64 `config:"rebalance-percentage" yaml:"rebalance-percentage,omitempty"`
	SafeDeletes         bool   `config:"safe-deletes" yaml:"safe-deletes,omitempty"`
	CacheAccess         string `config:"cache-access" yaml:"cache-access,omitempty"`
}

const (
	compName                         = "distributed_cache"
	defaultHeartBeatDurationInSecond = 30
	defaultReplicas                  = 1
	defaultMaxMissedHBs              = 3
	defaultChunkSize                 = 16 * 1024 * 1024 // 16 MB
	defaultMinNodes                  = 1
	defaultStripeSize                = 4
	defaultMvsPerRv                  = 1
	defaultRvFullThreshold           = 95
	defaultRvNearfullThreshold       = 80
	defaultClustermapEpoch           = 300
	defaultRebalancePercentage       = 80
	defaultSafeDeletes               = false
	defaultCacheAccess               = "automatic"
)

// Verification to check satisfaction criteria with Component Interface
var _ internal.Component = &DistributedCache{}

func (distributedCache *DistributedCache) Name() string {
	return compName
}

func (distributedCache *DistributedCache) SetName(name string) {
	distributedCache.BaseComponent.SetName(name)
}

func (distributedCache *DistributedCache) SetNextComponent(nextComponent internal.Component) {
	distributedCache.BaseComponent.SetNextComponent(nextComponent)
}

// Start : Pipeline calls this method to start the component functionality
//
//	this shall not block the call otherwise pipeline will not start
func (dc *DistributedCache) Start(ctx context.Context) error {

	log.Trace("DistributedCache::Start : Starting component %s", dc.Name())

	cacheDir := "__CACHE__" + dc.cacheID
	dc.azstroage = dc.NextComponent()
	for dc.azstroage != nil && dc.azstroage.Name() != "azstorage" {
		dc.azstroage = dc.azstroage.NextComponent()
	}
	dc.strorageCallback = initStorageCallback(
		dc.NextComponent(),
		dc.azstroage)
	dc.clusterManager = dcachelib.NewClusterManager(dc.strorageCallback)

	// Check and create cache directory if needed
	if err := dc.setupCacheStructure(cacheDir); err != nil {
		return err
	}
	log.Info("DistributedCache::Start : Cache structure setup completed")

	return nil
}

// setupCacheStructure checks and creates necessary cache directories and metadata.
// It's doing 4 rest api calls, 3 for directory and 1 for creator file.+1 call to check the creator file
func (dc *DistributedCache) setupCacheStructure(cacheDir string) error {
	_, err := dc.azstroage.GetAttr(internal.GetAttrOptions{Name: cacheDir + "/ClusterMap.json"})
	if err != nil {
		if os.IsNotExist(err) || err == syscall.ENOENT {
			directories := []string{cacheDir, cacheDir + "/Nodes", cacheDir + "/Objects"}
			for _, dir := range directories {
				if err := dc.azstroage.CreateDir(internal.CreateDirOptions{Name: dir, IsNoneMatchEtagEnabled: true}); err != nil {

					if !bloberror.HasCode(err, bloberror.BlobAlreadyExists) {
						return logAndReturnError(fmt.Sprintf("DistributedCache::Start error [failed to create directory %s: %v]", dir, err))
					}
				}
			}

			dcacheConfig := DCacheConfig{
				// MinNodes:               dc,
				ChunkSize:              "4MB",
				StripeSize:             "256MB",
				NumReplicas:            3,
				MvsPerRv:               1,
				RvFullThreshold:        95,
				RvNearfullThreshold:    80,
				HeartbeatSeconds:       30,
				HeartbeatsTillNodeDown: 3,
				ClustermapEpoch:        300,
				RebalancePercentage:    80,
				SafeDeletes:            true,
				CacheAccess:            "automatic",
			}
			dcacheConfigJSON, err := json.Marshal(dcacheConfig)
			log.Err("DistributedCache::Start : dcacheConfigJSON: %v %s", err, dcacheConfigJSON)
			err = dc.clusterManager.CreateClusterConfig()
			if bloberror.HasCode(err, bloberror.BlobAlreadyExists) {
				return logAndReturnError(fmt.Sprintf("DistributedCache::Start error [failed to create creator file: %v]", err))
			} else {
				return nil
			}
		} else {
			return logAndReturnError(fmt.Sprintf("DistributedCache::Start error [failed to read creator file: %v]", err))
		}
	}
	return nil
}

// Stop : Stop the component functionality and kill all threads started
func (dc *DistributedCache) Stop() error {
	log.Trace("DistributedCache::Stop : Stopping component %s", dc.Name())
	return nil
}

// Configure : Pipeline will call this method after constructor so that you can read config and initialize yourself
//
//	Return failure if any config is not valid to exit the process
func (distributedCache *DistributedCache) Configure(_ bool) error {
	log.Trace("DistributedCache::Configure : %s", distributedCache.Name())

	conf := DistributedCacheOptions{}
	err := config.UnmarshalKey(distributedCache.Name(), &conf)
	if err != nil {
		log.Err("DistributedCache::Configure : config error [invalid config attributes]")
		return fmt.Errorf("DistributedCache: config error [invalid config attributes]")
	}
	if conf.CacheID == "" {
		return fmt.Errorf("config error in %s: [cache-id not set]", distributedCache.Name())
	}
	if conf.CachePath == "" {
		return fmt.Errorf("config error in %s: [cache-path not set]", distributedCache.Name())
	}

	distributedCache.cacheID = conf.CacheID
	distributedCache.cachePath = conf.CachePath
	distributedCache.maxCacheSize = conf.MaxCacheSize
	distributedCache.replicas = defaultReplicas
	if config.IsSet(compName + ".replicas") {
		distributedCache.replicas = conf.Replicas
	}
	distributedCache.hbDuration = defaultHeartBeatDurationInSecond
	if config.IsSet(compName + ".heartbeat-duration") {
		distributedCache.hbDuration = conf.HeartbeatDuration
	}
	distributedCache.maxMissedHbs = defaultMaxMissedHBs
	if config.IsSet(compName + ".max-missed-heartbeats") {
		distributedCache.maxMissedHbs = uint8(conf.MaxMissedHeartbeats)
	}
	distributedCache.chunkSize = defaultChunkSize
	if config.IsSet(compName + ".chunk-size") {
		distributedCache.chunkSize = conf.ChunkSize
	}
	distributedCache.minNodes = defaultMinNodes
	if config.IsSet(compName + ".min-nodes") {
		distributedCache.minNodes = conf.MinNodes
	}
	distributedCache.stripeSize = defaultStripeSize
	if config.IsSet(compName + ".stripe-size") {
		distributedCache.stripeSize = conf.StripeSize
	}
	distributedCache.mvsPerRv = defaultMvsPerRv
	if config.IsSet(compName + ".mvs-per-rv") {
		distributedCache.mvsPerRv = conf.MVsPerRv
	}
	distributedCache.rvFullThreshold = defaultRvFullThreshold
	if config.IsSet(compName + ".rv-full-threshold") {
		distributedCache.rvFullThreshold = conf.RVFullThreshold
	}
	distributedCache.rvNearfullThreshold = defaultRvNearfullThreshold
	if config.IsSet(compName + ".rv-nearfull-threshold") {
		distributedCache.rvNearfullThreshold = conf.RVNearfullThreshold
	}
	distributedCache.clustermapEpoch = defaultClustermapEpoch
	if config.IsSet(compName + ".cluster-map-epoch") {
		distributedCache.clustermapEpoch = conf.ClustermapEpoch
	}
	distributedCache.rebalancePercentage = defaultRebalancePercentage
	if config.IsSet(compName + ".rebalance-percentage") {
		distributedCache.rebalancePercentage = conf.RebalancePercentage
	}
	distributedCache.safeDeletes = defaultSafeDeletes
	if config.IsSet(compName + ".safe-deletes") {
		distributedCache.safeDeletes = conf.SafeDeletes
	}
	distributedCache.cacheAccess = defaultCacheAccess
	if config.IsSet(compName + ".cache-access") {
		distributedCache.cacheAccess = conf.CacheAccess
	}
	return nil
}

// OnConfigChange : If component has registered, on config file change this method is called
func (dc *DistributedCache) OnConfigChange() {
}

// ------------------------- Factory -------------------------------------------

// Pipeline will call this method to create your object, initialize your variables here
// << DO NOT DELETE ANY AUTO GENERATED CODE HERE >>
func NewDistributedCacheComponent() internal.Component {
	comp := &DistributedCache{}
	comp.SetName(compName)
	return comp
}

// On init register this component to pipeline and supply your constructor
func init() {
	internal.AddComponent(compName, NewDistributedCacheComponent)

	cacheID := config.AddStringFlag("cache-id", "", "Cache ID for the distributed cache")
	config.BindPFlag(compName+".cache-id", cacheID)

	cachePath := config.AddStringFlag("dcache-cache-path", "", "Local Path of the distributed cache separated by commas")
	config.BindPFlag(compName+".cache-path", cachePath)

	chunkSize := config.AddUint64Flag("chunk-size", 16*1024*1024, "Chunk size for the cache")
	config.BindPFlag(compName+".chunk-size", chunkSize)

	maxCacheSize := config.AddUint64Flag("max-cache-size", 0, "Cache size for the cache")
	config.BindPFlag(compName+".max-cache-size", maxCacheSize)

	replicas := config.AddUint8Flag("replicas", defaultReplicas, "Number of replicas for the cache")
	config.BindPFlag(compName+".replicas", replicas)

	heartbeatDuration := config.AddUint16Flag("heartbeat-duration", defaultHeartBeatDurationInSecond, "Heartbeat duration for the cache")
	config.BindPFlag(compName+".heartbeat-duration", heartbeatDuration)

	missedHB := config.AddUint32Flag("max-missed-heartbeats", 3, "Heartbeat absence for the cache")
	config.BindPFlag(compName+".max-missed-heartbeats", missedHB)
}
