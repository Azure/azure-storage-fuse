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

	"github.com/Azure/azure-storage-fuse/v2/common/config"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
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
	cacheID      string
	cachePath    string
	maxCacheSize uint64
	replicas     uint8
	maxMissedHbs uint8
	hbDuration   uint16
}

// Structure defining your config parameters
type DistributedCacheOptions struct {
	CacheID             string `config:"cache-id" yaml:"cache-id,omitempty"`
	CachePath           string `config:"path" yaml:"path,omitempty"`
	ChunkSize           uint64 `config:"chunk-size" yaml:"chunk-size,omitempty"`
	MaxCacheSize        uint64 `config:"max-cache-size" yaml:"cache-size,omitempty"`
	Replicas            uint8  `config:"replicas" yaml:"replicas,omitempty"`
	HeartbeatDuration   uint16 `config:"heartbeat-duration" yaml:"heartbeat-duration,omitempty"`
	MaxMissedHeartbeats uint8  `config:"max-missed-heartbeats" yaml:"max-missed-heartbeats,omitempty"`
}

const (
	compName                         = "distributed_cache"
	defaultHeartBeatDurationInSecond = 30
	defaultReplicas                  = 3
	defaultMaxMissedHBs              = 3
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

	cacheID := config.AddStringFlag("cache-id", "", "Cache ID for the distributed cache")
	config.BindPFlag(compName+".cache-id", cacheID)

	cachePath := config.AddStringFlag("cache-dir", "", "Local Path of the cache")
	config.BindPFlag(compName+".path", cachePath)

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
