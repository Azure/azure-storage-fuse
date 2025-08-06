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
	"errors"
	"fmt"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/config"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache"
	clustermanager "github.com/Azure/azure-storage-fuse/v2/internal/dcache/cluster_manager"
	cm "github.com/Azure/azure-storage-fuse/v2/internal/dcache/clustermap"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache/debug"
	fm "github.com/Azure/azure-storage-fuse/v2/internal/dcache/file_manager"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache/gc"
	mm "github.com/Azure/azure-storage-fuse/v2/internal/dcache/metadata_manager"
	rm "github.com/Azure/azure-storage-fuse/v2/internal/dcache/replication_manager"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache/rpc"
	rpc_client "github.com/Azure/azure-storage-fuse/v2/internal/dcache/rpc/client"
	"github.com/Azure/azure-storage-fuse/v2/internal/handlemap"
	gouuid "github.com/google/uuid"
)

//go:generate $ASSERT_REMOVER $GOFILE

/* NOTES:
   - Component shall have a structure which inherits "internal.BaseComponent" to participate in pipeline
   - Component shall register a name and its constructor to participate in pipeline  (add by default by generator)
   - Order of calls : Constructor -> Configure -> Start ..... -> Stop
   - To read any new setting from config file follow the Configure method default comments
*/

// Components Flow Diagram For DistributedCache.

// libfuse
//   |
//   +--> DistributedCache
//   |       |
//   |       +--> dcacheFS --> azStorage
//   |       |
//   |       +--> azureFS --> block_cache --> azStorage

// Common structure for Component
type DistributedCache struct {
	internal.BaseComponent
	cfg DistributedCacheOptions // ← holds cache‐id, cache‐dirs, replicas, chunk‐size, etc.

	azstorage       internal.Component
	storageCallback dcache.StorageCallbacks
	pw              *parallelWriter
}

// Structure defining your config parameters
type DistributedCacheOptions struct {
	CacheID   string   `config:"cache-id" yaml:"cache-id,omitempty"`
	CacheDirs []string `config:"cache-dirs" yaml:"cache-dirs,omitempty"`

	ChunkSize  uint64 `config:"chunk-size" yaml:"chunk-size,omitempty"`
	StripeSize uint64 `config:"stripe-size" yaml:"stripe-size,omitempty"`
	Replicas   uint32 `config:"replicas" yaml:"replicas,omitempty"`

	HeartbeatDuration   uint16 `config:"heartbeat-duration" yaml:"heartbeat-duration,omitempty"`
	MaxMissedHeartbeats uint8  `config:"max-missed-heartbeats" yaml:"max-missed-heartbeats,omitempty"`
	RVFullThreshold     uint64 `config:"rv-full-threshold" yaml:"rv-full-threshold,omitempty"`
	RVNearfullThreshold uint64 `config:"rv-nearfull-threshold" yaml:"rv-nearfull-threshold,omitempty"`
	MaxCacheSize        uint64 `config:"max-cache-size" yaml:"max-cache-size,omitempty"`

	MinNodes            uint32 `config:"min-nodes" yaml:"min-nodes,omitempty"`
	MaxRVs              uint32 `config:"max-rvs" yaml:"max-rvs,omitempty"`
	MVsPerRV            uint64 `config:"mvs-per-rv" yaml:"mvs-per-rv,omitempty"`
	RebalancePercentage uint8  `config:"rebalance-percentage" yaml:"rebalance-percentage,omitempty"`
	SafeDeletes         bool   `config:"safe-deletes" yaml:"safe-deletes,omitempty"`
	CacheAccess         string `config:"cache-access" yaml:"cache-access,omitempty"`
	ClustermapEpoch     uint64 `config:"clustermap-epoch" yaml:"clustermap-epoch,omitempty"`
	ReadIOMode          string `config:"read-io-mode" yaml:"read-io-mode,omitempty"`
	WriteIOMode         string `config:"write-io-mode" yaml:"write-io-mode,omitempty"`
}

const (
	compName                         = "distributed_cache"
	defaultHeartBeatDurationInSecond = 30
	defaultReplicas                  = 3
	defaultMaxMissedHBs              = 3
	defaultChunkSize                 = 4 * 1024 * 1024 // 4 MB
	defaultMinNodes                  = 1
	defaultMaxRVs                    = 100
	defaultStripeSize                = 16 * 1024 * 1024 // 16 MB
	defaultMVsPerRV                  = 10
	defaultRvFullThreshold           = 95
	defaultRvNearfullThreshold       = 80
	defaultClustermapEpoch           = 300
	defaultRebalancePercentage       = 80
	defaultSafeDeletes               = false
	defaultCacheAccess               = "automatic"
	dcacheDirContToken               = "__DCDIRENT__"
)

// Verification to check satisfaction criteria with Component Interface
var _ internal.Component = &DistributedCache{}

func (dc *DistributedCache) Name() string {
	return compName
}

func (dc *DistributedCache) SetName(name string) {
	dc.BaseComponent.SetName(name)
}

func (dc *DistributedCache) SetNextComponent(nextComponent internal.Component) {
	dc.BaseComponent.SetNextComponent(nextComponent)
}

func (dc *DistributedCache) Priority() internal.ComponentPriority {
	return internal.EComponentPriority.LevelOne()
}

// Start : Pipeline calls this method to start the component functionality
//
//	this shall not block the call otherwise pipeline will not start
func (dc *DistributedCache) Start(ctx context.Context) error {

	log.Trace("DistributedCache::Start : Starting component %s", dc.Name())

	dc.azstorage = dc.NextComponent()
	for dc.azstorage != nil && dc.azstorage.Name() != "azstorage" {
		dc.azstorage = dc.azstorage.NextComponent()
	}
	if dc.azstorage == nil {
		return log.LogAndReturnError("DistributedCache::Start error [azstorage component not found]")
	}

	// Create UUID before initializing any of the dcache components, so that UUID is correctly available for all components.
	ensureUUID()

	// rpc client must be initialized before any of its users.
	rpc_client.Start()

	dc.storageCallback = initStorageCallback(
		dc.NextComponent(),
		dc.azstorage)

	err := mm.Init(dc.storageCallback, dc.cfg.CacheID)
	if err != nil {
		return log.LogAndReturnError(fmt.Sprintf("DistributedCache::Start error [Failed to start metadata manager : %v]", err))
	}

	errString := dc.startClusterManager()
	if errString != "" {
		return log.LogAndReturnError(errString)
	}

	err = dcache.InitBufferPool(dc.cfg.ChunkSize)
	if err != nil {
		return log.LogAndReturnError(fmt.Sprintf("DistributedCache::Start error [Failed to create BufferPool : %v]", err))
	}

	err = rm.Start()
	if err != nil {
		return log.LogAndReturnError(fmt.Sprintf("DistributedCache::Start error [Failed to start replication manager : %v]", err))
	}

	dc.pw = newParallelWriter()

	err = fm.NewFileIOManager()
	if err != nil {
		return log.LogAndReturnError(fmt.Sprintf("DistributedCache::Start error [Failed to start fileio manager : %v]", err))
	}

	gc.Start()

	log.Info("DistributedCache::Start : component started successfully")

	return nil
}

func (dc *DistributedCache) startClusterManager() string {

	dCacheConfig := &dcache.DCacheConfig{
		CacheId:                dc.cfg.CacheID,
		MinNodes:               dc.cfg.MinNodes,
		MaxRVs:                 dc.cfg.MaxRVs,
		ChunkSize:              dc.cfg.ChunkSize,
		StripeSize:             dc.cfg.StripeSize,
		NumReplicas:            dc.cfg.Replicas,
		MVsPerRV:               dc.cfg.MVsPerRV,
		HeartbeatSeconds:       dc.cfg.HeartbeatDuration,
		HeartbeatsTillNodeDown: dc.cfg.MaxMissedHeartbeats,
		ClustermapEpoch:        dc.cfg.ClustermapEpoch,
		RebalancePercentage:    dc.cfg.RebalancePercentage,
		SafeDeletes:            dc.cfg.SafeDeletes,
		CacheAccess:            dc.cfg.CacheAccess,
		RvFullThreshold:        dc.cfg.RVFullThreshold,
		RvNearfullThreshold:    dc.cfg.RVNearfullThreshold,
	}
	rvList, err := dc.createRVList()
	if err != nil {
		return fmt.Sprintf("DistributedCache::Start error [Failed to create RV List for cluster manager : %v]", err)
	}

	//
	// If user sets some invalid value for any config, clustermanager startup will fail.
	//
	err = clustermanager.Start(dCacheConfig, rvList)
	if err != nil {
		return fmt.Sprintf("DistributedCache::Start error [Failed to start cluster manager : %v]", err)
	}
	return ""
}

func (dc *DistributedCache) createRVList() ([]dcache.RawVolume, error) {
	ipaddr, err := getVmIp()
	if err != nil {
		return nil, log.LogAndReturnError(fmt.Sprintf("DistributedCache::Start error [Failed to get VM IP : %v]", err))
	}

	uuidVal, err := common.GetNodeUUID()
	if err != nil {
		return nil, log.LogAndReturnError(fmt.Sprintf("DistributedCache::Start error [Failed to retrieve UUID, error: %v]", err))
	}

	rvList := make([]dcache.RawVolume, len(dc.cfg.CacheDirs))
	rvIDToPath := make(map[string]string, len(dc.cfg.CacheDirs))

	for index, path := range dc.cfg.CacheDirs {
		rvId, err := getBlockDeviceUUId(path)
		if err != nil {
			return nil, log.LogAndReturnError(fmt.Sprintf("DistributedCache::Start error [failed to get raw volume UUID: %v]", err))
		}

		if common.IsDebugBuild() {
			if common.IsFakingScaleTest() {
				//
				// Generate UUID using SHA1 of the path string.
				// Additionally, append the node UUID to ensure uniqueness across nodes.
				//
				rvId = gouuid.NewSHA1(gouuid.NameSpaceDNS, []byte(uuidVal+path)).String()
			}
		}

		//
		// No two RVs exported by us must have the same RVid.
		// This will catch the following two cases:
		// - Two distinct cache-dir elements have the same filesystem GUID.
		// - User accidentally provided a duplicate cache-dir element.
		//
		if existingPath, exists := rvIDToPath[rvId]; exists {
			return nil, log.LogAndReturnError(fmt.Sprintf(
				"DistributedCache::Start error [duplicate rvId %s for path %s, conflicts with path %s]",
				rvId, path, existingPath))
		}
		rvIDToPath[rvId] = path

		totalSpace, availableSpace, err := common.GetDiskSpaceMetricsFromStatfs(path)
		if err != nil {
			return nil, log.LogAndReturnError(fmt.Sprintf("DistributedCache::Start error [failed to evaluate local cache Total space: %v]", err))
		}

		rvList[index] = dcache.RawVolume{
			NodeId:         uuidVal,
			IPAddress:      ipaddr,
			RvId:           rvId,
			FDID:           "0",
			State:          dcache.StateOnline,
			TotalSpace:     totalSpace,
			AvailableSpace: availableSpace,
			LocalCachePath: path,
		}
	}
	return rvList, nil
}

// Stop : Stop the component functionality and kill all threads started
func (dc *DistributedCache) Stop() error {
	log.Trace("DistributedCache::Stop : Stopping component %s", dc.Name())

	dc.pw.destroyParallelWriter()
	fm.EndFileIOManager()
	gc.End()
	rm.Stop()
	clustermanager.Stop()
	rpc_client.Cleanup()

	return nil
}

// Configure : Pipeline will call this method after constructor so that you can read config and initialize yourself
//
//	Return failure if any config is not valid to exit the process
func (distributedCache *DistributedCache) Configure(_ bool) error {
	log.Trace("DistributedCache::Configure : %s", distributedCache.Name())

	err := config.UnmarshalKey(distributedCache.Name(), &distributedCache.cfg)

	if err != nil {
		log.Err("DistributedCache::Configure : config error [invalid config attributes]")
		return fmt.Errorf("DistributedCache: config error [invalid config attributes]")
	}
	if distributedCache.cfg.CacheID == "" {
		return fmt.Errorf("config error in %s: [cache-id not set]", distributedCache.Name())
	}
	if len(distributedCache.cfg.CacheDirs) == 0 {
		return fmt.Errorf("config error in %s: [cache-dirs not set]", distributedCache.Name())
	}

	// Check if the cache directories exist.
	for _, dir := range distributedCache.cfg.CacheDirs {
		log.Info("DistributedCache::Configure : Checking if cache dir exists: %s", dir)
		info, err := os.Stat(dir)
		if os.IsNotExist(err) {
			log.Err("DistributedCache::Configure : cache-dirs %s does not exist", dir)
			return fmt.Errorf("config error in %s: [cache-dirs %s does not exist]", distributedCache.Name(), dir)
		} else if err != nil {
			log.Err("DistributedCache::Configure : error accessing cache-dirs %s: %v", dir, err)
			return fmt.Errorf("config error in %s: [error accessing cache-dirs %s: %v]", distributedCache.Name(), dir, err)
		}

		// Check if it is a directory.
		if !info.IsDir() {
			log.Err("DistributedCache::Configure : cache-dirs %s is not a directory", dir)
			return fmt.Errorf("config error in %s: [cache-dirs %s is not a directory]", distributedCache.Name(), dir)
		}

		// Test write permission by creating a temporary file.
		testFile := filepath.Join(dir, fmt.Sprintf(".perm_test_%d_%d", time.Now().UnixNano(), rand.Uint64()))
		log.Info("DistributedCache::Configure : Testing write permission in %s", dir)
		f, err := os.Create(testFile)
		if err != nil {
			log.Err("DistributedCache::Configure : cache directory %s is not writable: %v", dir, err)
			return fmt.Errorf("config error in %s: cache directory %s is not writable: %v", distributedCache.Name(), dir, err)
		}
		defer f.Close()

		// Clean up test file.
		log.Info("DistributedCache::Configure : Cleaning up temp file %s created", testFile)
		if err := os.Remove(testFile); err != nil {
			log.Err("DistributedCache::Configure : cleanup of temp file %s failed: %v", testFile, err)
			return fmt.Errorf("config error in %s: cache directory %s cleanup of temp file %s failed: %v", distributedCache.Name(), dir, testFile, err)
		}

		// Test read permission by opening the directory.
		log.Info("DistributedCache::Configure : Testing read permission in %s", dir)
		dirFile, err := os.Open(dir)
		if err != nil {
			log.Err("DistributedCache::Configure : cache directory %s is not readable: %v", dir, err)
			return fmt.Errorf("config error in %s: cache directory %s is not readable: %v", distributedCache.Name(), dir, err)
		}
		defer dirFile.Close()
	}

	if !config.IsSet(compName + ".replicas") {
		distributedCache.cfg.Replicas = defaultReplicas
	}
	if !config.IsSet(compName + ".heartbeat-duration") {
		distributedCache.cfg.HeartbeatDuration = defaultHeartBeatDurationInSecond
	}
	if !config.IsSet(compName + ".max-missed-heartbeats") {
		distributedCache.cfg.MaxMissedHeartbeats = defaultMaxMissedHBs
	}
	if !config.IsSet(compName + ".chunk-size") {
		distributedCache.cfg.ChunkSize = defaultChunkSize
	}
	if !config.IsSet(compName + ".min-nodes") {
		distributedCache.cfg.MinNodes = defaultMinNodes
	}
	if !config.IsSet(compName + ".max-rvs") {
		distributedCache.cfg.MaxRVs = defaultMaxRVs
	}
	if !config.IsSet(compName + ".stripe-size") {
		distributedCache.cfg.StripeSize = defaultStripeSize
	}
	if !config.IsSet(compName + ".mvs-per-rv") {
		//
		// If user sets mvs-per-rv in the config then we use that value and that decides the number of MVs,
		// depending on the number of RVs and replicas, but the more common case is that user does not specify
		// mvs-per-rv in the config, but instead they specify max-rvs, in that case we need to calculate the
		// MV count accordingly.
		//

		// This is the minimum number of MVs possible, what we get if we host one MV per RV.
		minMVs := int64(distributedCache.cfg.MaxRVs / distributedCache.cfg.Replicas)
		common.Assert(minMVs > 0, distributedCache.cfg)
		minMVs = max(minMVs, 1)

		// For the given cfg.MaxRVs value, this is the maximum number of MVs that we can have.
		maxMVs := minMVs * cm.MaxMVsPerRV

		// Start with the minimum number of MVs.
		numMVs := minMVs

		// Try to create at least cm.PreferredMVs number of MVs.
		if numMVs < cm.PreferredMVs {
			numMVs = cm.PreferredMVs
		}

		// but not more than the maximum number of MVs possible with cm.MaxMVsPerRV.
		if numMVs > maxMVs {
			numMVs = maxMVs
		}

		// Set MVsPerRV needed to achieve these many MVs.
		distributedCache.cfg.MVsPerRV = uint64(numMVs / minMVs)

		log.Info("DistributedCache::Configure : cfg.MVsPerRV set to %d, minMVs: %d, maxMVs: %d",
			distributedCache.cfg.MVsPerRV, minMVs, maxMVs)

		// Our calculated MVsPerRV must be within the allowed range.
		common.Assert(distributedCache.cfg.MVsPerRV >= uint64(cm.MinMVsPerRV) &&
			distributedCache.cfg.MVsPerRV <= uint64(cm.MaxMVsPerRV),
			distributedCache.cfg, numMVs, minMVs, maxMVs)
	}
	if !config.IsSet(compName + ".rv-full-threshold") {
		distributedCache.cfg.RVFullThreshold = defaultRvFullThreshold
	}
	if !config.IsSet(compName + ".rv-nearfull-threshold") {
		distributedCache.cfg.RVNearfullThreshold = defaultRvNearfullThreshold
	}
	if !config.IsSet(compName + ".clustermap-epoch") {
		distributedCache.cfg.ClustermapEpoch = defaultClustermapEpoch
	}
	if !config.IsSet(compName + ".rebalance-percentage") {
		distributedCache.cfg.RebalancePercentage = defaultRebalancePercentage
	}
	if !config.IsSet(compName + ".safe-deletes") {
		distributedCache.cfg.SafeDeletes = defaultSafeDeletes
	}
	if !config.IsSet(compName + ".cache-access") {
		distributedCache.cfg.CacheAccess = defaultCacheAccess
	}

	// Both read/write default to direct IO.
	if !config.IsSet(compName + ".read-io-mode") {
		distributedCache.cfg.ReadIOMode = rpc.DirectIO
	}
	if !config.IsSet(compName + ".write-io-mode") {
		distributedCache.cfg.WriteIOMode = rpc.DirectIO
	}

	err = rpc.SetReadIOMode(distributedCache.cfg.ReadIOMode)
	if err != nil {
		return fmt.Errorf("config error in %s: [cannot set read-io-mode (%s)]: %v",
			distributedCache.Name(), distributedCache.cfg.ReadIOMode, err)
	}

	err = rpc.SetWriteIOMode(distributedCache.cfg.WriteIOMode)
	if err != nil {
		return fmt.Errorf("config error in %s: [cannot set write-io-mode (%s)]: %v",
			distributedCache.Name(), distributedCache.cfg.WriteIOMode, err)
	}

	//
	// In direct IO read or write operations, the chunk size must be a multiple
	// of filesystem block size.
	//
	if (rpc.ReadIOMode == rpc.DirectIO || rpc.WriteIOMode == rpc.DirectIO) &&
		distributedCache.cfg.ChunkSize%common.FS_BLOCK_SIZE != 0 {
		return fmt.Errorf("config error in %s: [chunk-size (%d) must be a multiple of %d bytes]",
			distributedCache.Name(), distributedCache.cfg.ChunkSize, common.FS_BLOCK_SIZE)
	}

	return nil
}

// OnConfigChange : If component has registered, on config file change this method is called
func (dc *DistributedCache) OnConfigChange() {
}

func (dc *DistributedCache) GetAttr(options internal.GetAttrOptions) (*internal.ObjAttr, error) {
	if strings.HasPrefix(options.Name, "__CACHE__") {
		return nil, syscall.ENOENT
	}

	var attr *internal.ObjAttr
	var err error
	isAzurePath, isDcachePath, isDebugPath, rawPath := getFS(options.Name)

	if isMountPointRoot(rawPath) {
		if isAzurePath {
			return getPlaceholderDirForRoot("fs=azure"), nil
		} else if isDcachePath {
			return getPlaceholderDirForRoot("fs=dcache"), nil
		} else if isDebugPath {
			return getPlaceholderDirForRoot("fs=debug"), nil
		}
	}

	//
	// Files deleted on dcache are renamed with a special extension. These special files are for internal
	// bookkeeping and we don't want to expose those to the user.
	// Even for azure we don't allow these files.
	//
	if isDeletedDcacheFile(rawPath) {
		log.Debug("DistributedCache::GetAttr : isDeletedDcacheFile(%s), hiding", rawPath)
		return nil, syscall.ENOENT
	}

	if isDcachePath {
		// properties should be fetched from Dcache
		log.Debug("DistributedCache::GetAttr : Path is having Dcache subcomponent, path : %s", options.Name)
		rawPath = filepath.Join(mm.GetMdRoot(), "Objects", rawPath)
		options.Name = rawPath

		if attr, err = dc.NextComponent().GetAttr(options); err != nil {
			return nil, err
		}
	} else if isAzurePath {
		// properties should be fetched from Azure
		log.Debug("DistributedCache::GetAttr : Path is having Azure subcomponent, path : %s", options.Name)
		options.Name = rawPath
		if attr, err = dc.NextComponent().GetAttr(options); err != nil {
			return nil, err
		}
	} else if isDebugPath {
		// properties should be fetched from debugfs
		options.Name = rawPath
		return debug.GetAttr(options)
	} else {
		common.Assert(rawPath == options.Name, rawPath, options.Name)
		//
		// Semantics for unqualified path is, if the attr exist in dcache, serve from there else get from azure.
		//
		dcachePath := filepath.Join(mm.GetMdRoot(), "Objects", rawPath)
		options.Name = dcachePath
		log.Debug("DistributedCache::GetAttr : Unqualified Path getting attr from dcache, path : %s", options.Name)

		if attr, err = dc.NextComponent().GetAttr(options); err != nil {
			//
			// If it fails with any other error other than ENOENT, we fail the call, else if the file is not
			// present in dcache, we should try Azure.
			//
			if err != syscall.ENOENT {
				log.Err("DistributedCache::GetAttr :  Unqualified Path (%s), failed to get attr from dcache: %v",
					options.Name, err)
				return nil, err
			}

			// GetAttr from Azure.
			options.Name = rawPath
			log.Debug("DistributedCache::GetAttr :  Unqualified Path, not present in dcache, trying Azure, path : %s",
				options.Name)

			return dc.NextComponent().GetAttr(options)
		}
	}

	// Parse the metadata info for dcache specific files.
	if !isAzurePath && !isDebugPath {
		err := parseDcacheMetadata(attr)
		if err != nil {
			return nil, err
		}
	}
	return attr, nil
}

func (dc *DistributedCache) StreamDir(options internal.StreamDirOptions) ([]*internal.ObjAttr, string, error) {
	var dirList []*internal.ObjAttr
	var token string
	var err error

	isAzurePath, isDcachePath, isDebugPath, rawPath := getFS(options.Name)

startListingWithNewToken:
	if isDcachePath {
		log.Debug("DistributedCache::StreamDir : Path is having Dcache subcomponent, path : %s", options.Name)
		rawPath = filepath.Join(mm.GetMdRoot(), "Objects", rawPath)
		options.Name = rawPath
		if dirList, token, err = dc.NextComponent().StreamDir(options); err != nil {
			return dirList, token, err
		}
		dirList = parseDcacheMetadataForDirEntries(dirList)
	} else if isAzurePath {
		log.Debug("DistributedCache::StreamDir : Path is having Azure subcomponent, path : %s", options.Name)
		options.Name = rawPath
		if dirList, token, err = dc.NextComponent().StreamDir(options); err != nil {
			return dirList, token, err
		}

		// While iterating the entries of the root of the container skip the cache folder.
		if isMountPointRoot(rawPath) {
			dirList = hideCacheMetadata(dirList)
		}
	} else if isDebugPath {
		log.Debug("DistributedCache::StreamDir : Path is having Debug subcomponent, path : %s", options.Name)
		return debug.StreamDir(options)
	} else {
		// When enumerating a fresh directory, options.IsFsDcache must be true.
		common.Assert(options.Token != "" || *options.IsFsDcache == true)

		// When enumerating a fresh directory, options.DcacheEntries must be empty.
		common.Assert(options.Token != "" || len(options.DcacheEntries) == 0)
		//
		// Semantics for Readdir for unquailified path, if a dirent exists in both Dcache and Azure filesystem,
		// then dirent present in the dcache takes the precedence over Azure and the entry in Azure is masked and
		// only the entry in dcache is listed. This is to match user expectation when they actually read such a
		// file, the file from dcache is read, hence it makes sense to list the same.
		// To know which virtual fs we are currently in we use options.IsFsDcache. This is set to true in opendir()
		// and thus we start by enumerating from dcache. Once dcache enumeration hits eod (signified by empty token
		// return) we set options.IsFsDcache to false so that on receiving the next Streamdir() call from fuse we
		// start enumerating from Azure. We store all the entries enumerated from dcache in options.DcacheEntries
		// map which allows us to skip any entry already listed from dcache from Azure's listing.
		//
		if *options.IsFsDcache { // List from dcache.
			log.Debug("DistributedCache::StreamDir : Listing on Unqualified path, listing from dcache, path : %s", options.Name)
			dcachePath := filepath.Join(mm.GetMdRoot(), "Objects", rawPath)
			options.Name = dcachePath
			if dirList, token, err = dc.NextComponent().StreamDir(options); err != nil {
				return dirList, token, err
			}

			dirList = parseDcacheMetadataForDirEntries(dirList)
			for _, attr := range dirList {
				options.DcacheEntries[attr.Name] = struct{}{}
			}

			if token == "" {
				// Empty token signifies end-of-directory for dcache listing, start listing from Azure.
				// We set token to the special non-empty value to prevent fuse from treating this as
				// end-of-directory.
				*options.IsFsDcache = false
				token = dcacheDirContToken
			}
		} else { // List from Azure.
			log.Debug("DistributedCache::StreamDir : Listing on Unqualified path, listing from Azure, path : %s", options.Name)
			// Reset the token if it's starting to iterate from start.
			if options.Token == dcacheDirContToken {
				options.Token = ""
			}

			options.Name = rawPath
			if dirList, token, err = dc.NextComponent().StreamDir(options); err != nil {
				return dirList, token, err
			}

			// Ignore the dirent if it's already returned by the dcache listing.
			var modifiedDirList []*internal.ObjAttr = make([]*internal.ObjAttr, 0, len(dirList))
			for _, attr := range dirList {
				if _, ok := options.DcacheEntries[attr.Name]; !ok {
					modifiedDirList = append(modifiedDirList, attr)
				}
			}

			dirList = modifiedDirList

			// While iterating the entries of the root of the container skip the cache folder.
			if isMountPointRoot(rawPath) {
				dirList = hideCacheMetadata(dirList)
			}
		}
	}

	// Start listing again, If the dirList becomes empty after hiding cachedir.
	if (len(dirList) == 0) && token != "" {
		options.Token = token
		goto startListingWithNewToken
	}

	return dirList, token, nil
}

func (dc *DistributedCache) CreateDir(options internal.CreateDirOptions) error {
	isAzurePath, isDcachePath, isDebugPath, rawPath := getFS(options.Name)

	//
	// Don't allow creating the special deleted file and avoid confusion.
	// Same for the fuse hidden file.
	//
	if isDeletedDcacheFile(rawPath) {
		return syscall.EINVAL
	}

	if common.IsFuseHiddenFile(rawPath) {
		return syscall.EINVAL
	}

	if isDcachePath {
		// Create Directory inside Dcache
		log.Debug("DistributedCache::CreateDir: Path is having Dcache subcomponent, path: %s", options.Name)
		rawPath = filepath.Join(mm.GetMdRoot(), "Objects", rawPath)
		options.Name = rawPath
		return dc.NextComponent().CreateDir(options)
	} else if isAzurePath {
		// Create Directory inside Azure
		log.Debug("DistributedCache::CreateDir: Path is having Azure subcomponent, path: %s", options.Name)
		options.Name = rawPath
		return dc.NextComponent().CreateDir(options)
	} else if isDebugPath {
		// No Permission to  create directories inside debug path
		return syscall.EACCES
	} else {
		common.Assert(rawPath == options.Name, rawPath, options.Name)
		// Semantics for creating a directory, when path doesn't have explicit namespace.
		// Create in Azure and Dcache, fail the call if any one of them fail.

		// Create Dir in Azure
		err := dc.NextComponent().CreateDir(options)
		if err != nil {
			log.Err("DistributedCache::CreateDir: Failed to create Azure directory %s: %v", options.Name, err)
			return err
		}

		// Create Dir in Dcache
		rawPath = filepath.Join(mm.GetMdRoot(), "Objects", rawPath)
		options.Name = rawPath
		err = dc.NextComponent().CreateDir(options)
		if err != nil {
			log.Err("DistributedCache::CreateDir: Failed to create Dcache directory %s: %v", options.Name, err)
			return err
		}
		// todo : if one is success and other is failure, get to the previous state by removing the
		// created entries for the files.
	}

	return nil
}

func (dc *DistributedCache) CreateFile(options internal.CreateFileOptions) (*handlemap.Handle, error) {
	var dcFile *fm.DcacheFile
	var handle *handlemap.Handle
	var err error
	isAzurePath, isDcachePath, isDebugPath, rawPath := getFS(options.Name)

	//
	// Don't allow creating the special deleted file and avoid confusion.
	// Same for the fuse hidden file.
	//
	if isDeletedDcacheFile(rawPath) {
		return nil, syscall.EINVAL
	}

	if common.IsFuseHiddenFile(rawPath) {
		return nil, syscall.EINVAL
	}

	if isDcachePath {
		log.Debug("DistributedCache::CreateFile : Path is having Dcache subcomponent, path : %s", options.Name)
		options.Name = rawPath
		dcFile, err = fm.NewDcacheFile(rawPath)
		if err != nil {
			log.Err("DistributedCache::CreateFile : Dcache File Creation failed with err : %s, path : %s", err.Error(), options.Name)
			return nil, err
		}
	} else if isAzurePath {
		log.Debug("DistributedCache::CreateFile : Path is having Azure subcomponent, path : %s", options.Name)
		options.Name = rawPath
		handle, err = dc.NextComponent().CreateFile(options)
		if err != nil {
			log.Err("DistributedCache::CreateFile : Azure File Creation failed with err : %s, path : %s", err.Error(), options.Name)
			return nil, err
		}
	} else if isDebugPath {
		// Don't permit to create files inside the debug directory.
		return nil, syscall.EACCES
	} else {
		common.Assert(rawPath == options.Name, rawPath, options.Name)
		// semantics for creating a file for write with out any explicit namespace
		// Create in dcache and Azure, fail the call if any one of them fail.
		dcFile, err = fm.NewDcacheFile(rawPath)
		if err != nil {
			log.Err("DistributedCache::CreateFile : Dcache File Creation failed with err : %s, path : %s", err.Error(), options.Name)
			return nil, err
		}

		handle, err = dc.NextComponent().CreateFile(options)
		if err != nil {
			log.Err("DistributedCache::CreateFile : Azure File Creation failed with err : %s, path : %s", err.Error(), options.Name)
			return nil, err
		}
		// todo : if one is success and other is failure, get to the previous state by removing the
		// created entries for the files.
	}

	if handle == nil {
		handle = handlemap.NewHandle(options.Name)
	}

	// Set the respective filesystems that this handle can access
	if isAzurePath {
		handle.SetFsAzure()
	} else if isDcachePath {
		handle.SetFsDcache()
	} else {
		handle.SetFsDefault()
	}

	// Set Dcache file inside the handle
	handle.IFObj = dcFile

	// DCache files are immutable. They cannot be written to once created.
	// To be precise, we allow write only on an fd that's returned by creat() or open(O_CREAT|O_EXCL).
	// The file contents are sealed once the fd closes and post that the file becomes immutable.
	// Since this fd/handle corresponds to a new file being created, mark the handle to allow writes.
	// This will be checked by other handlers that write data to a file, e.g., WriteFile(), SyncFile(),
	// FlushFile().
	if handle.IFObj != nil {
		handle.SetDcacheAllowWrites()
	}
	// handle.IFObj must be set IFF DCache access is allowed through this handle.
	common.Assert(handle.IsFsDcache() == (handle.IFObj.(*fm.DcacheFile) != nil))

	return handle, nil
}

func (dc *DistributedCache) OpenFile(options internal.OpenFileOptions) (*handlemap.Handle, error) {
	var dcFile *fm.DcacheFile
	var handle *handlemap.Handle
	var err error

	isAzurePath, isDcachePath, isDebugPath, rawPath := getFS(options.Name)

	//
	// Since we hide the special delete file from fuse we should not be called to open that.
	// Same for the fuse hidden file, it's never created.
	//
	if isDeletedDcacheFile(rawPath) {
		common.Assert(false, options.Name, rawPath)
		return nil, syscall.EINVAL
	}

	if common.IsFuseHiddenFile(rawPath) {
		common.Assert(false, options.Name, rawPath)
		return nil, syscall.EINVAL
	}

	// todo: We should only support write if the file is only in Azure.
	if options.Flags&os.O_WRONLY != 0 || options.Flags&os.O_RDWR != 0 {
		log.Err("DistributedCache::OpenFile: Dcache file cannot open with flags: %X, file : %s", options.Flags, options.Name)
		return nil, syscall.EACCES
	}

	if isDcachePath {
		log.Debug("DistributedCache::OpenFile : Path is having Dcache subcomponent, path : %s", options.Name)
		options.Name = rawPath
		dcFile, err = fm.OpenDcacheFile(options.Name)
		if err != nil {
			log.Err("DistributedCache::OpenFile : Dcache File Open failed with err : %s, path : %s", err.Error(), options.Name)
			return nil, err
		}
	} else if isAzurePath {
		log.Debug("DistributedCache::OpenFile : Path is having Azure subcomponent, path : %s", options.Name)
		options.Name = rawPath
		handle, err = dc.NextComponent().OpenFile(options)
		if err != nil {
			log.Err("DistributedCache::OpenFile : Azure File Open failed with err : %s, path : %s", err.Error(), options.Name)
			return nil, err
		}
	} else if isDebugPath {
		options.Name = rawPath
		return debug.OpenFile(options)
	} else {
		// If the path don't come with no explicit namespace
		// It should first check the file in dcache, if present, read from dcache,
		// else check in azure if present, read from azure, else fail the open.
		common.Assert(rawPath == options.Name, rawPath, options.Name)
		dcFile, err = fm.OpenDcacheFile(rawPath)
		if err == nil {
			log.Debug("DistributedCache::OpenFile : Opening the file from Dcache, path : %s", options.Name)
			handle = handlemap.NewHandle(options.Name)
			handle.SetFsDcache()
		} else {
			// todo: make sure we come here when opening dcache file is returning ENOENT
			log.Err("DistributedCache::OpenFile : Dcache File Open failed with err : %s, path : %s, Trying to Open the file in Azure", err.Error(), options.Name)
			handle, err = dc.NextComponent().OpenFile(options)
			if err != nil {
				log.Err("DistributedCache::OpenFile : Azure File Open failed with err : %s, path : %s", err.Error(), options.Name)
				return nil, err
			}
			log.Debug("DistributedCache::OpenFile : Opening the file from Azure, path : %s", options.Name)
			handle.SetFsAzure()
		}
	}

	if handle == nil {
		handle = handlemap.NewHandle(options.Name)
	}

	// Set the respective filesystems that this handle can access
	if isAzurePath {
		handle.SetFsAzure()
	} else if isDcachePath {
		handle.SetFsDcache()
	}

	// Set Dcache file inside the handle
	handle.IFObj = dcFile

	if handle.IFObj != nil {
		handle.SetDcacheAllowReads()
	}

	// handle.IFObj must be set IFF DCache access is allowed through this handle.
	common.Assert(handle.IsFsDcache() == (handle.IFObj.(*fm.DcacheFile) != nil))
	return handle, nil
}

func (dc *DistributedCache) ReadInBuffer(options *internal.ReadInBufferOptions) (int, error) {
	// todo: Can this method  can handle len(options.Data)== 0?
	// Currently dcache read handles it, be sure about that.
	log.Debug("DistributedCache::ReadInBuffer : ReadInBuffer, offset : %d, buf size : %d, file : %s",
		options.Offset, len(options.Data), options.Handle.Path)

	var err error
	var bytesRead int
	if options.Handle.IsFsDcache() {
		common.Assert(options.Handle.IFObj != nil)
		common.Assert(options.Handle.IsDcacheAllowReads())
		dcFile := options.Handle.IFObj.(*fm.DcacheFile)
		bytesRead, err = dcFile.ReadFile(options.Offset, options.Data)
		if err == nil || err == io.EOF {
			return bytesRead, err
		}
		common.Assert(bytesRead == 0)
		log.Err("DistributedCache::ReadInBuffer : Failed to read the file from the Dcache, offset : %d, file : %s",
			options.Offset, options.Handle.Path)
	} else if options.Handle.IsFsAzure() {
		bytesRead, err = dc.NextComponent().ReadInBuffer(options)
		if err == nil || err == io.EOF {
			return bytesRead, err
		}
		common.Assert(bytesRead == 0)
		log.Err("DistributedCache::ReadInBuffer : Failed to read the file from the Azure, offset : %d, file : %s",
			options.Offset, options.Handle.Path)
	} else if options.Handle.IsFsDebug() {
		return debug.ReadFile(options)
	} else {
		common.Assert(false)
	}

	return 0, err
}

func (dc *DistributedCache) WriteFile(options internal.WriteFileOptions) (int, error) {
	log.Debug("DistributedCache::WriteFile : WriteFile, offset : %d, buf size : %d, file : %s",
		options.Offset, len(options.Data), options.Handle.Path)
	common.Assert(len(options.Data) != 0)
	// Debug files are readonly.
	common.Assert(!options.Handle.IsFsDebug(), options.Handle.Path)

	// When user wants to write to a default path (no explicit fs=azure/fs=dcache namespace specified)
	// we have multiple possible semantics:
	// 1. Write through
	//    In this mode every application write is written to both the dcache as well as Azure, as if
	//    user explicitly wrote to either of them. If any of these write fails, the application write
	//    is failed.
	// 2. Write back on close
	//    In this mode application writes are sent to dcache and only on close() the entire dcache
	//    file is written to Azure as well.
	// 3. Write back on eviction
	//    In this mode application writes are sent to dcache and only if/when the dcache file is evicted,
	//    we write it to Azure.
	//
	// For now we implement the "Write through" semantics.
	//
	// Set the handle is dirty to get the flush call.
	options.Handle.Flags.Set(handlemap.HandleFlagDirty)
	var dcacheErr, azureErr error

	dcacheWrite := func() error {
		log.Debug("DistributedCache::WriteFile : Dcache write, offset : %d, buf size : %d, file : %s",
			options.Offset, len(options.Data), options.Handle.Path)
		common.Assert(options.Handle.IFObj != nil)
		common.Assert(options.Handle.IsDcacheAllowWrites())

		// The following is used when writes come even after closing the file. ignore for now.
		if !options.Handle.IsDcacheAllowWrites() {
			return syscall.EIO
		}
		dcFile := options.Handle.IFObj.(*fm.DcacheFile)
		dcacheErr = dcFile.WriteFile(options.Offset, options.Data)
		if dcacheErr != nil {
			// If write on one media fails, then return err instantly
			log.Err("DistributedCache::WriteFile : Dcache File write Failed, offset : %d, file : %s",
				options.Offset, options.Handle.Path)
			return dcacheErr
		}
		return nil
	}

	azureWrite := func() error {
		log.Debug("DistributedCache::WriteFile : Azure write, offset : %d, buf size : %d, file : %s",
			options.Offset, len(options.Data), options.Handle.Path)

		_, azureErr = dc.NextComponent().WriteFile(options)
		if azureErr != nil {
			log.Err("DistributedCache::WriteFile : Azure File write Failed, offset : %d, file : %s",
				options.Offset, options.Handle.Path)
			return azureErr
		}
		return nil
	}

	if options.Handle.IsFsDcache() && options.Handle.IsFsAzure() {

		// Parallelly write to azure and dcache.
		// Enqueue the work of azure to the parallel writers and continue writing to the dcache from here.
		azureErrChan := dc.pw.EnqueuAzureWrite(azureWrite)
		dcacheErr = dcacheWrite()

		// Wait for the azure write response.
		azureErr = <-azureErrChan

	} else if options.Handle.IsFsDcache() {
		dcacheErr = dcacheWrite()
	} else if options.Handle.IsFsAzure() {
		azureErr = azureWrite()
	}

	return len(options.Data), errors.Join(dcacheErr, azureErr)
}

func (dc *DistributedCache) SyncFile(options internal.SyncFileOptions) error {
	log.Debug("DistributedCache::SyncFile : SyncFile file : %s", options.Handle.Path)

	var dcacheErr, azureErr error
	if options.Handle.IsFsDcache() {
		common.Assert(options.Handle.IFObj != nil)
		common.Assert(options.Handle.IsDcacheAllowWrites())
		dcFile := options.Handle.IFObj.(*fm.DcacheFile)
		dcacheErr = dcFile.SyncFile()
		if dcacheErr != nil {
			log.Err("DistributedCache::SyncFile : Dcache File sync failed : %s", options.Handle.Path)
		}
	}

	if options.Handle.IsFsAzure() {
		azureErr = dc.NextComponent().SyncFile(options)
		if azureErr != nil {
			log.Err("DistributedCache::SyncFile : Azure file sync failed : %s", options.Handle.Path)
		}
	}
	return errors.Join(dcacheErr, azureErr)
}

func (dc *DistributedCache) FlushFile(options internal.FlushFileOptions) error {
	log.Debug("DistributedCache::FlushFile : Close file : %s", options.Handle.Path)
	// Allow only one Flush/close call per file when writing, if user application duplicates the fd
	// then the writes after fist close would fail.

	var dcacheErr, azureErr error

	if options.Handle.IsFsDcache() {
		common.Assert(options.Handle.IFObj != nil)
		common.Assert(options.Handle.IsDcacheAllowWrites())
		if !options.Handle.IsDcacheAllowWrites() {
			return nil
		}

		dcFile := options.Handle.IFObj.(*fm.DcacheFile)
		dcacheErr = dcFile.CloseFile()
		common.Assert(dcacheErr == nil)
		if dcacheErr == nil {
			// Clear this flag to signal no more writes on this handle.
			// Fail any writes that come after this.
			options.Handle.SetDcacheStopWrites()
		}
	}

	if options.Handle.IsFsAzure() {
		azureErr = dc.NextComponent().SyncFile(internal.SyncFileOptions{
			Handle: options.Handle,
		})
		if azureErr != nil {
			log.Err("DistributedCache::FlushFile : Failed to SyncFile to Azure file : %s", options.Handle.Path)
		}
	}
	return errors.Join(dcacheErr, azureErr)
}

// Deallocate all the buffers for the file. This is an async call.
func (dc *DistributedCache) CloseFile(options internal.CloseFileOptions) error {
	log.Debug("DistributedCache::CloseFile : Release file : %s", options.Handle.Path)
	// Debug is exclusive, if debug is set dcache and azure flags cannot be set.
	common.Assert(!options.Handle.IsFsDebug() || (!options.Handle.IsFsDcache() && !options.Handle.IsFsAzure()))

	var dcacheErr, azureErr error

	if options.Handle.IsFsDcache() {
		common.Assert(options.Handle.IFObj != nil)
		//
		// A dcache file handle can be either opened for read or write.
		// Distributed cache doesn't support handles opened for readwrite.
		//
		common.Assert(!options.Handle.IsDcacheAllowReads() || !options.Handle.IsDcacheAllowWrites())

		dcFile := options.Handle.IFObj.(*fm.DcacheFile)

		//
		// While creating the file and closing the file immediately w/o any intervening writes, we don't get
		// the flush call, as libfuse component only sends it when there is some write on the handle.
		// For such files, we force DistributedCache.FlushFile() which in turn calls DcacheFile.CloseFile()
		// which finalizes the file. If there were writes on the file before close, the allow-write flag is
		// unset by DistributedCache.FlushFile() and we skip the finalize in that case.
		//
		if options.Handle.IsDcacheAllowWrites() {
			dcacheErr = dc.FlushFile(internal.FlushFileOptions{
				Handle: options.Handle,
			})
			common.Assert(dcacheErr == nil, dcacheErr)
		}

		//
		// When readonly dcache file handles are closed with safeDeletes config enabled, the file's
		// open count must be reduced, let ReleaseFile() know that.
		//
		isReadOnlyHandle := options.Handle.IsDcacheAllowReads()

		dcacheErr = dcFile.ReleaseFile(isReadOnlyHandle)
		if dcacheErr != nil {
			log.Err("DistributedCache::CloseFile : Failed to ReleaseFile for Dcache file : %s", options.Handle.Path)
		}
	}

	if options.Handle.IsFsAzure() {
		azureErr = dc.NextComponent().CloseFile(options)
		if azureErr != nil {
			log.Err("DistributedCache::SyncFile : Failed to ReleaseFile for Azure file : %s", options.Handle.Path)
		}
	}

	if options.Handle.IsFsDebug() {
		return debug.CloseFile(options)
	}

	return errors.Join(dcacheErr, azureErr)
}

func (dc *DistributedCache) TruncateFile(options internal.TruncateFileOptions) error {
	return syscall.ENOTSUP
}

func (dc *DistributedCache) DeleteFile(options internal.DeleteFileOptions) error {
	log.Debug("DistributedCache::DeleteFile: Delete file: %s", options.Name)

	var dcacheErr, azureErr error

	isAzurePath, isDcachePath, isDebugPath, rawPath := getFS(options.Name)

	//
	// We fool fuse into believing that we created the special hidden file (while what we created
	// was our special ".dcache.deleting" file). Now the last open handle on the file has been closed
	// and fuse wants to delete the hidden file it created, we continue the illusion and tell fuse
	// that we deleted it :-)
	//
	if common.IsFuseHiddenFile(rawPath) {
		return nil
	}

	//
	// This is an internal file that we hide from fuse.
	//
	if isDeletedDcacheFile(rawPath) {
		return syscall.ENOENT
	}

	if isDcachePath {
		log.Debug("DistributedCache::DeleteFile: Delete for Dcache file: %s", rawPath)
		err := fm.DeleteDcacheFile(rawPath)
		if err != nil {
			log.Err("DistributedCache::DeleteFile: Delete failed for Dcache file %s: %v", options.Name, err)
			return err
		}
	} else if isAzurePath {
		log.Debug("DistributedCache::DeleteFile: Delete Azure file: %s", rawPath)
		options.Name = rawPath
		err := dc.NextComponent().DeleteFile(options)
		if err != nil {
			log.Err("DistributedCache::DeleteFile: Delete failed for Azure file %s: %v", options.Name, err)
			return err
		}
	} else if isDebugPath {
		return syscall.EROFS
	} else {
		//
		// Semantics for Unqualified Path, Delete from both Azure and Dcache. If file is present in only one qualified
		// path, then delete only from that path. If the call has come here it already means that the file is present
		// in at least one qualified path as stat would be checked before doing deletion of a file.
		//
		log.Debug("DistributedCache::DeleteFile: Delete Dcache file for Unqualified Path: %s", options.Name)

		dcacheErr = fm.DeleteDcacheFile(rawPath)
		if dcacheErr != nil {
			log.Err("DistributedCache::DeleteFile: Delete failed for Unqualified Path Dcache file %s: %v",
				rawPath, dcacheErr)

			//
			// If the file is not present in dcache, we need to delete from Azure, else on any other error, bail
			// out.
			//
			if dcacheErr != syscall.ENOENT {
				return dcacheErr
			}
		}

		options.Name = rawPath
		log.Debug("DistributedCache::DeleteFile: Delete Azure file for Unqualified Path: %s", options.Name)

		azureErr = dc.NextComponent().DeleteFile(options)
		if azureErr != nil {
			log.Err("DistributedCache::DeleteFile: Delete failed for Unqualified Path Azure file %s: %v",
				options.Name, azureErr)

			if azureErr != syscall.ENOENT {
				return azureErr
			}
		}

		//
		// If only one of the errors is ENOENT, nil the other one.
		//
		if !(dcacheErr == syscall.ENOENT && azureErr == syscall.ENOENT) {
			azureErr = nil
			dcacheErr = nil
		} else {
			// Both cannot be ENOENT, why did fuse call us in the first place?
			common.Assert(false, options.Name)
		}
	}

	return errors.Join(dcacheErr, azureErr)
}

func (dc *DistributedCache) RenameFile(options internal.RenameFileOptions) error {
	log.Debug("DistributedCache::RenameFile: %s -> %s", options.Src, options.Dst)

	//
	// The only rename that we support is the rename done by fuse to the special hidden file if an open file
	// is deleted. In that case we should create our own special file, and not the fuse hidden file.
	//
	// TODO: Need to handle the case where user deletes a dcache file causing us to create the special
	//		 deleted file, then user create a new file with the same name and before we could GC the previous
	//		 deleted file, he deletes this new file.
	//		 We will need to maintain multiple of these files using some seq number.
	//
	if common.IsFuseHiddenFile(options.Dst) {
		return dc.DeleteFile(internal.DeleteFileOptions{
			Name: options.Src,
		})
	}

	return syscall.ENOTSUP
}

// This call is made by libfuse component to check if emptiness of a directory be making the delete directory call.
func (dc *DistributedCache) IsDirEmpty(options internal.IsDirEmptyOptions) bool {
	log.Debug("DistributedCache::IsDirEmpty: Check if dir is empty: %s", options.Name)

	isAzurePath, isDcachePath, isDebugPath, rawPath := getFS(options.Name)
	if isDcachePath {
		log.Debug("DistributedCache::IsDirEmpty: IsDirEmpty for Dcache dir: %s", rawPath)
		rawPath = filepath.Join(mm.GetMdRoot(), "Objects", rawPath)
		options.Name = rawPath
		return dc.NextComponent().IsDirEmpty(options)
	} else if isAzurePath {
		log.Debug("DistributedCache::IsDirEmpty: IsDirEmpty for Azure dir: %s", rawPath)
		options.Name = rawPath
		return dc.NextComponent().IsDirEmpty(options)
	} else if isDebugPath {
		log.Debug("DistributedCache::IsDirEmpty: IsDirEmpty for Debug dir: %s", rawPath)
		// Debug directories are never empty, they always have some files.
		return false
	} else {
		log.Debug("DistributedCache::IsDirEmpty: IsDirEmpty for Unqualified Path dir: %s", options.Name)

		// We return true if the directory is empty in both dcache and azure.
		//
		// Check if directory in empty in dcache.
		dcachePath := filepath.Join(mm.GetMdRoot(), "Objects", rawPath)
		options.Name = dcachePath
		isEmpty := dc.NextComponent().IsDirEmpty(options)
		if !isEmpty {
			return false
		}

		// If the dcache dir is empty, check if the Azure dir is empty.
		options.Name = rawPath
		return dc.NextComponent().IsDirEmpty(options)
	}
}

// If call comes here, it means that the directory is empty.
func (dc *DistributedCache) DeleteDir(options internal.DeleteDirOptions) error {
	log.Debug("DistributedCache::DeleteDir: Delete dir: %s", options.Name)

	isAzurePath, isDcachePath, isDebugPath, rawPath := getFS(options.Name)

	if isDcachePath {
		log.Debug("DistributedCache::DeleteDir: Delete Dcache dir: %s", rawPath)
		rawPath = filepath.Join(mm.GetMdRoot(), "Objects", rawPath)
		options.Name = rawPath
		err := dc.NextComponent().DeleteDir(options)
		if err != nil {
			log.Err("DistributedCache::DeleteDir: Delete failed for Dcache dir %s: %v", options.Name, err)
			return err
		}
	} else if isAzurePath {
		log.Debug("DistributedCache::DeleteDir: Delete Azure dir: %s", rawPath)
		options.Name = rawPath
		err := dc.NextComponent().DeleteDir(options)
		if err != nil {
			log.Err("DistributedCache::DeleteDir: Delete failed for Azure dir %s: %v", options.Name, err)
			return err
		}
	} else if isDebugPath {
		return syscall.EROFS
	} else {
		log.Debug("DistributedCache::DeleteDir: Delete Unqualified Path dir: %s", options.Name)

		// Semantics for Unqualified path:
		// We get this call when both of the directories in (Azure/dcache) are empty.
		//
		// Delete Directory from dcache.
		dcachePath := filepath.Join(mm.GetMdRoot(), "Objects", rawPath)
		options.Name = dcachePath

		err := dc.NextComponent().DeleteDir(options)
		if err != nil {
			log.Err("DistributedCache::DeleteDir: Delete failed for Unqualified Path Dcache dir %s: %v",
				rawPath, err)

			if err != syscall.ENOENT {
				return err
			}
			err = nil // If the dir was not present in dcache, continue to delete from Azure.
		}

		// Delete Directory from Azure.
		options.Name = rawPath
		err = dc.NextComponent().DeleteDir(options)
		if err != nil {
			log.Err("DistributedCache::DeleteDir: Delete failed for Unqualified Path Azure dir %s: %v",
				options.Name, err)

			if err != syscall.ENOENT {
				return err
			}
			err = nil // If the dir was not present in Azure, continue to return nil.
		}
	}

	return nil
}

func (dc *DistributedCache) RenameDir(options internal.RenameDirOptions) error {
	return syscall.ENOTSUP
}

// ------------------------- Factory -------------------------------------------

// Pipeline will call this method to create your object, initialize your variables here
// << DO NOT DELETE ANY AUTO GENERATED CODE HERE >>
func NewDistributedCacheComponent() internal.Component {
	comp := &DistributedCache{}
	comp.SetName(compName)
	return comp
}

// Very first call to common.GetNodeUUID() queries the UUID from the file and caches it for later
// use. Make sure we don't proceed w/o a valid UUID.
func ensureUUID() {
	// This one should query from the uuid file or create and store in the file.
	uuid1, err := common.GetNodeUUID()
	if err != nil {
		log.GetLoggerObj().Panicf("DistributedCache::ensureUUID: GetNodeUUID(1) failed: %v", err)
	}

	// This one (and all subsequent calls) should return the cached UUID.
	uuid2, err := common.GetNodeUUID()
	if err != nil {
		log.GetLoggerObj().Panicf("DistributedCache::ensureUUID: GetNodeUUID(2) failed: %v", err)
	}

	if uuid1 != uuid2 {
		log.GetLoggerObj().Panicf("DistributedCache::ensureUUID: GetNodeUUID() returned different values, %s and %s",
			uuid1, uuid2)
	}

	if !common.IsValidUUID(uuid2) {
		log.GetLoggerObj().Panicf("DistributedCache::ensureUUID: GetNodeUUID() returned invalid UUID %s",
			uuid2)
	}
}

// On init register this component to pipeline and supply your constructor
func init() {
	// Silence unused import error for release builds.
	gouuid.New()

	internal.AddComponent(compName, NewDistributedCacheComponent)

	cacheID := config.AddStringFlag("cache-id", "", "Cache ID for the distributed cache")
	config.BindPFlag(compName+".cache-id", cacheID)

	cacheDirFlag := config.AddStringSliceFlag("cache-dirs", []string{}, "One or more local cache directories for distributed cache (comma-separated), e.g. --cache-dirs=/mnt/tmp,/mnt/abc")
	config.BindPFlag(compName+".cache-dirs", cacheDirFlag)

	chunkSize := config.AddUint64Flag("chunk-size", defaultChunkSize, "Chunk size for the cache")
	config.BindPFlag(compName+".chunk-size", chunkSize)

	maxCacheSize := config.AddUint64Flag("max-cache-size", 0, "Cache size for the cache")
	config.BindPFlag(compName+".max-cache-size", maxCacheSize)

	replicas := config.AddUint32Flag("replicas", defaultReplicas, "Number of replicas for the cache")
	config.BindPFlag(compName+".replicas", replicas)

	heartbeatDuration := config.AddUint16Flag("heartbeat-duration", defaultHeartBeatDurationInSecond, "Heartbeat duration for the cache")
	config.BindPFlag(compName+".heartbeat-duration", heartbeatDuration)

	missedHB := config.AddUint32Flag("max-missed-heartbeats", 3, "Heartbeat absence for the cache")
	config.BindPFlag(compName+".max-missed-heartbeats", missedHB)

	clustermapEpoch := config.AddUint64Flag("clustermap-epoch", defaultClustermapEpoch, "Epoch duration for the clustermap update")
	config.BindPFlag(compName+".clustermap-epoch", clustermapEpoch)

	stripeSize := config.AddUint64Flag("stripe-size", defaultStripeSize, "Stripe size for the cache")
	config.BindPFlag(compName+".stripe-size", stripeSize)

	mvsPerRv := config.AddUint64Flag("mvs-per-rv", defaultMVsPerRV, "Number of MVs per raw volume")
	config.BindPFlag(compName+".mvs-per-rv", mvsPerRv)

	rvFullThreshold := config.AddUint64Flag("rv-full-threshold", defaultRvFullThreshold, "Percent to mark RV full")
	config.BindPFlag(compName+".rv-full-threshold", rvFullThreshold)

	rvNearfullThreshold := config.AddUint64Flag("rv-nearfull-threshold", defaultRvNearfullThreshold, "Percent to mark RV near full")
	config.BindPFlag(compName+".rv-nearfull-threshold", rvNearfullThreshold)

	minNodes := config.AddUint32Flag("min-nodes", defaultMinNodes, "Minimum number of nodes required to make the cache functional")
	config.BindPFlag(compName+".min-nodes", minNodes)

	maxRVs := config.AddUint32Flag("max-rvs", defaultMaxRVs, "Estimate of maximum number of RVs (raw volumes) that the cluster will have")
	config.BindPFlag(compName+".max-rvs", maxRVs)

	rebalancePercentage := config.AddUint8Flag("rebalance-percentage", defaultRebalancePercentage, "Rebalance threshold percentage")
	config.BindPFlag(compName+".rebalance-percentage", rebalancePercentage)

	safeDeletes := config.AddBoolFlag("safe-deletes", defaultSafeDeletes, "Enable safe‑delete mode")
	config.BindPFlag(compName+".safe-deletes", safeDeletes)

	cacheAccess := config.AddStringFlag("cache-access", defaultCacheAccess, "Cache access mode (automatic/manual)")
	config.BindPFlag(compName+".cache-access", cacheAccess)

	readIOMode := config.AddStringFlag("read-io-mode", rpc.DirectIO, "IO mode for reading chunk files (direct/buffered)")
	config.BindPFlag(compName+".read-io-mode", readIOMode)

	writeIOMode := config.AddStringFlag("write-io-mode", rpc.DirectIO, "IO mode for writing chunk files (direct/buffered)")
	config.BindPFlag(compName+".write-io-mode", writeIOMode)
}
