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
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/bloberror"
	"github.com/Azure/azure-storage-fuse/v2/common/config"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/component/azstorage"
	"github.com/Azure/azure-storage-fuse/v2/internal"
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
	cacheID    string
	cachePath  string
	replicas   uint32
	hbTimeout  uint32
	hbDuration uint32

	storage          azstorage.AzConnection
	heartbeatManager *HeartbeatManager
}

// Structure defining your config parameters
type DistributedCacheOptions struct {
	CacheID           string `config:"cache-id" yaml:"cache-id,omitempty"`
	CachePath         string `config:"path" yaml:"path,omitempty"`
	ChunkSize         uint64 `config:"chunk-size" yaml:"chunk-size,omitempty"`
	CacheSize         uint64 `config:"cache-size" yaml:"cache-size,omitempty"`
	Replicas          uint32 `config:"replicas" yaml:"replicas,omitempty"`
	HeartbeatTimeout  uint32 `config:"heartbeat-timeout" yaml:"heartbeat-timeout,omitempty"`
	HeartbeatDuration uint32 `config:"heartbeat-duration" yaml:"heartbeat-duration,omitempty"`
	MissedHeartbeat   uint32 `config:"heartbeats-till-node-down" yaml:"heartbeats-till-node-down,omitempty"`
}

const (
	compName          = "distributed_cache"
	HeartBeatDuration = 30
	REPLICAS          = 3
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

// Start initializes the DistributedCache component without blocking the pipeline.
func (dc *DistributedCache) Start(ctx context.Context) error {
	log.Trace("DistributedCache::Start : Starting component %s", dc.Name())

	// Get and validate storage component
	storageComponent, ok := internal.GetStorageComponent().(*azstorage.AzStorage)
	if !ok || storageComponent.GetBlobStorage() == nil {
		return logAndReturnError("DistributedCache::Start : error [invalid or missing storage component]")
	}
	dc.storage = storageComponent.GetBlobStorage()
	cacheDir := "__CACHE__" + dc.cacheID

	// Check and create cache directory if needed
	if err := dc.setupCacheStructure(cacheDir); err != nil {
		return err
	}

	log.Info("DistributedCache::Start : Cache structure setup completed")
	dc.heartbeatManager = NewHeartbeatManager(dc.cachePath, dc.storage, dc.hbDuration, "__CACHE__"+dc.cacheID)
	dc.heartbeatManager.Start()
	return nil

}

// setupCacheStructure checks and creates necessary cache directories and metadata.
// It's doing 4 rest api calls, 3 for directory and 1 for creator file.+1 call to check the creator file
func (dc *DistributedCache) setupCacheStructure(cacheDir string) error {
	_, err := dc.storage.GetAttr(cacheDir + "/creator.txt")
	if err != nil {
		directories := []string{cacheDir, cacheDir + "/Nodes", cacheDir + "/Objects"}
		for _, dir := range directories {
			if err := dc.storage.CreateDirectory(dir, true); err != nil {

				// If I will check directly the creator file and if it is not created then I am kind of loop to check the file again and again Rather than that I have added the call to create remaining directory structure if the one is failed
				if bloberror.HasCode(err, bloberror.BlobAlreadyExists) {
					continue
				} else if err != nil {
					return logAndReturnError(fmt.Sprintf("DistributedCache::Start error [failed to create directory %s: %v]", dir, err))
				}
			}
		}

		// Add metadata file with VM IP
		ip, err := getVmIp()
		if err != nil {
			return logAndReturnError(fmt.Sprintf("DistributedCache::Start error [failed to get VM IP: %v]", err))
		}
		if err := dc.storage.WriteFromBuffer(internal.WriteFromBufferOptions{Name: cacheDir + "/creator.txt",
			Data: []byte(ip),
			Etag: true}); err != nil {
			if !bloberror.HasCode(err, bloberror.BlobAlreadyExists) {
				return logAndReturnError(fmt.Sprintf("DistributedCache::Start error [failed to create creator file: %v]", err))
			} else {
				return nil
			}
		}
	}
	return nil
}

// logAndReturnError logs the error and returns it.
func logAndReturnError(msg string) error {
	log.Err(msg)
	return fmt.Errorf(msg)
}

// Stop : Stop the component functionality and kill all threads started
func (dc *DistributedCache) Stop() error {
	log.Trace("DistributedCache::Stop : Stopping component %s", dc.Name())
	if dc.heartbeatManager != nil {
		dc.heartbeatManager.Stop()
	}
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
	if config.IsSet(compName + ".cache-id") {
		distributedCache.cacheID = conf.CacheID
	} else {
		log.Err("DistributedCache: config error [cache-id not set]")
		return fmt.Errorf("config error in %s error [cache-id not set]", distributedCache.Name())
	}

	if config.IsSet(compName + ".path") {
		distributedCache.cachePath = conf.CachePath
	} else {
		log.Err("DistributedCache: config error [cache-path not set]")
		return fmt.Errorf("config error in %s error [cache-path not set]", distributedCache.Name())
	}

	distributedCache.replicas = REPLICAS
	if config.IsSet(compName + ".replicas") {
		distributedCache.replicas = conf.Replicas
	}

	distributedCache.hbDuration = HeartBeatDuration
	if config.IsSet(compName + ".heartbeat-duration") {
		distributedCache.hbDuration = conf.HeartbeatDuration
	}

	return nil
}

// OnConfigChange : If component has registered, on config file change this method is called
func (distributedCache *DistributedCache) OnConfigChange() {
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

	cacheID := config.AddStringFlag("cache-id", "blobfuse", "Cache ID for the distributed cache")
	config.BindPFlag(compName+".cache-id", cacheID)

	cachePath := config.AddStringFlag("cache-dir", "/tmp", "Path to the cache")
	config.BindPFlag(compName+".path", cachePath)

	chunkSize := config.AddUint64Flag("chunk-size", 1024*1024, "Chunk size for the cache")
	config.BindPFlag(compName+".chunk-size", chunkSize)

	cacheSize := config.AddUint64Flag("cache-size", 1024*1024*1024, "Cache size for the cache")
	config.BindPFlag(compName+".cache-size", cacheSize)

	replicas := config.AddUint32Flag("replicas", 3, "Number of replicas for the cache")
	config.BindPFlag(compName+".replicas", replicas)

	heartbeatTimeout := config.AddUint32Flag("heartbeat-timeout", 30, "Heartbeat timeout for the cache")
	config.BindPFlag(compName+".heartbeat-timeout", heartbeatTimeout)

	heartbeatDuration := config.AddUint32Flag("heartbeat-duration", HeartBeatDuration, "Heartbeat duration for the cache")
	config.BindPFlag(compName+".heartbeat-duration", heartbeatDuration)

	missedHB := config.AddUint32Flag("heartbeats-till-node-down", 3, "Heartbeat absence for the cache")
	config.BindPFlag(compName+".heartbeats-till-node-down", missedHB)
}
