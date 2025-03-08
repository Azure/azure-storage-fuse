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

package dcache

import (
	"context"
	"fmt"

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
type Dcache struct {
	internal.BaseComponent
	cacheID   string
	tmpPath   string
	chunkSize uint64
	replicas  uint32
	hbTimeout uint32
	hbAbsence uint32

	storage azstorage.AzConnection
}

// Structure defining your config parameters
type DcacheOptions struct {
	CacheID   string `config:"cache-id" yaml:"cache-id,omitempty"`
	TmpPath   string `config:"path" yaml:"path,omitempty"`
	ChunkSize uint64 `config:"chunk-size" yaml:"chunk-size,omitempty"`
	CacheSize uint64 `config:"cache-size" yaml:"cache-size,omitempty"`
	Replicas  uint32 `config:"replicas" yaml:"replicas,omitempty"`
	HBTimeout uint32 `config:"heartbeat-timeout" yaml:"heartbeat-timeout,omitempty"`
	HBAbsence uint32 `config:"heartbeat-absence" yaml:"heartbeat-absence,omitempty"`
}

const compName = "dcache"

// Verification to check satisfaction criteria with Component Interface
var _ internal.Component = &Dcache{}

func (c *Dcache) Name() string {
	return compName
}

func (c *Dcache) SetName(name string) {
	c.BaseComponent.SetName(name)
}

func (c *Dcache) SetNextComponent(nc internal.Component) {
	c.BaseComponent.SetNextComponent(nc)
}

// Start : Pipeline calls this method to start the component functionality
//
//	this shall not block the call otherwise pipeline will not start
func (c *Dcache) Start(ctx context.Context) error {
	log.Trace("Dcache::Start : Starting component %s", c.Name())

	footer := internal.GetStorageComponent()
	if footer == nil {
		log.Err("Dcache::Start : error [storage component not found]")
		return fmt.Errorf("Dcache: error [storage component not found]")
	}

	azs := footer.(*azstorage.AzStorage)
	c.storage = azs.GetBlobStorage()
	if c.storage == nil {
		log.Err("Dcache::Start : error [storage component not found]")
		return fmt.Errorf("Dcache: error [storage component not found]")
	}

	return nil
}

// Stop : Stop the component functionality and kill all threads started
func (c *Dcache) Stop() error {
	log.Trace("Dcache::Stop : Stopping component %s", c.Name())

	return nil
}

// Configure : Pipeline will call this method after constructor so that you can read config and initialize yourself
//
//	Return failure if any config is not valid to exit the process
func (c *Dcache) Configure(_ bool) error {
	log.Trace("Dcache::Configure : %s", c.Name())

	// >> If you do not need any config parameters remove below code and return nil
	conf := DcacheOptions{}
	err := config.UnmarshalKey(c.Name(), &conf)
	if err != nil {
		log.Err("Dcache::Configure : config error [invalid config attributes]")
		return fmt.Errorf("Dcache: config error [invalid config attributes]")
	}

	return nil
}

// OnConfigChange : If component has registered, on config file change this method is called
func (c *Dcache) OnConfigChange() {
}

// ------------------------- Factory -------------------------------------------

// Pipeline will call this method to create your object, initialize your variables here
// << DO NOT DELETE ANY AUTO GENERATED CODE HERE >>
func NewDcacheComponent() internal.Component {
	comp := &Dcache{}
	comp.SetName(compName)
	return comp
}

// On init register this component to pipeline and supply your constructor
func init() {
	internal.AddComponent(compName, NewDcacheComponent)

	cacheID := config.AddStringFlag("cache-id", "blobfuse2", "Cache ID for the distributed cache")
	config.BindPFlag(compName+".cache-id", cacheID)

	tmpPath := config.AddStringFlag("path", "/tmp", "Path to the cache")
	config.BindPFlag(compName+".path", tmpPath)

	chunkSize := config.AddUint64Flag("chunk-size", 1024*1024, "Chunk size for the cache")
	config.BindPFlag(compName+".chunk-size", chunkSize)

	cacheSize := config.AddUint64Flag("cache-size", 1024*1024*1024, "Cache size for the cache")
	config.BindPFlag(compName+".cache-size", cacheSize)

	replicas := config.AddUint32Flag("replicas", 3, "Number of replicas for the cache")
	config.BindPFlag(compName+".replicas", replicas)

	hbTimeout := config.AddUint32Flag("heartbeat-timeout", 60, "Heartbeat timeout for the cache")
	config.BindPFlag(compName+".heartbeat-timeout", hbTimeout)

	hbAbsence := config.AddUint32Flag("heartbeat-absence", 120, "Heartbeat absence for the cache")
	config.BindPFlag(compName+".heartbeat-absence", hbAbsence)
}
