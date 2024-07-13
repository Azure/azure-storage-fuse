/*
    _____           _____   _____   ____          ______  _____  ------
   |     |  |      |     | |     | |     |     | |       |            |
   |     |  |      |     | |     | |     |     | |       |            |
   | --- |  |      |     | |-----| |---- |     | |-----| |-----  ------
   |     |  |      |     | |     | |     |     |       | |       |
   | ____|  |_____ | ____| | ____| |     |_____|  _____| |_____  |_____


   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2020-2024 Microsoft Corporation. All rights reserved.
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

package entry_cache

import (
	"container/list"
	"context"
	"fmt"
	"sync"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/config"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal"
	"github.com/vibhansa-msft/tlru"
)

// Common structure for Component
type EntryCache struct {
	internal.BaseComponent
	cacheTimeout uint32
	pathLocks    *common.LockMap
	pathLRU      *tlru.TLRU
	pathMap      sync.Map
}

type pathCacheItem struct {
	children  []*internal.ObjAttr
	nextToken string
}

// By default entry cache is valid for 30 seconds
const defaultEntryCacheTimeout uint32 = (30)

// Structure defining your config parameters
type EntryCacheOptions struct {
	Timeout uint32 `config:"timeout-sec" yaml:"timeout-sec,omitempty"`
}

const compName = "entry_cache"

// Verification to check satisfaction criteria with Component Interface
var _ internal.Component = &EntryCache{}

func (c *EntryCache) Name() string {
	return compName
}

func (c *EntryCache) SetName(name string) {
	c.BaseComponent.SetName(name)
}

func (c *EntryCache) SetNextComponent(nc internal.Component) {
	c.BaseComponent.SetNextComponent(nc)
}

// Start : Pipeline calls this method to start the component functionality
//
//	this shall not block the call otherwise pipeline will not start
func (c *EntryCache) Start(ctx context.Context) error {
	log.Trace("EntryCache::Start : Starting component %s", c.Name())

	err := c.pathLRU.Start()
	if err != nil {
		log.Err("EntryCache::Start : fail to start LRU for path caching [%s]", err.Error())
		return fmt.Errorf("failed to start LRU for path caching [%s]", err.Error())
	}

	return nil
}

// Stop : Stop the component functionality and kill all threads started
func (c *EntryCache) Stop() error {
	log.Trace("EntryCache::Stop : Stopping component %s", c.Name())

	err := c.pathLRU.Stop()
	if err != nil {
		log.Err("EntryCache::Stop : fail to stop LRU for path caching [%s]", err.Error())
	}

	return nil
}

// Configure : Pipeline will call this method after constructor so that you can read config and initialize yourself
//
//	Return failure if any config is not valid to exit the process
func (c *EntryCache) Configure(_ bool) error {
	log.Trace("EntryCache::Configure : %s", c.Name())

	// >> If you do not need any config parameters remove below code and return nil
	conf := EntryCacheOptions{}
	err := config.UnmarshalKey(c.Name(), &conf)
	if err != nil {
		log.Err("EntryCache::Configure : config error [invalid config attributes]")
		return fmt.Errorf("EntryCache: config error [invalid config attributes]")
	}

	c.cacheTimeout = defaultEntryCacheTimeout
	if config.IsSet(compName + ".timeout-sec") {
		c.cacheTimeout = conf.Timeout
	}

	c.pathLRU, err = tlru.New(1000, c.cacheTimeout, c.pathEvict, 0, nil)
	if err != nil {
		log.Err("EntryCache::Start : fail to create LRU for path caching [%s]", err.Error())
		return fmt.Errorf("config error in %s [%s]", c.Name(), err.Error())
	}

	c.pathLocks = common.NewLockMap()

	return nil
}

// StreamDir : Optionally cache entries of the list
func (c *EntryCache) StreamDir(options internal.StreamDirOptions) ([]*internal.ObjAttr, string, error) {
	log.Trace("AttrCache::StreamDir : %s", options.Name)

	pathKey := fmt.Sprintf("%s##%s", options.Name, options.Token)
	flock := c.pathLocks.Get(pathKey)
	flock.Lock()
	defer flock.Unlock()

	pathEntry, found := c.pathMap.Load(pathKey)
	if !found {
		log.Debug("EntryCache::StreamDir : Cache not valid, fetch new list for path: %s, token %s", options.Name, options.Token)
		pathList, token, err := c.NextComponent().StreamDir(options)
		if err == nil && len(pathList) > 0 {
			item := pathCacheItem{
				children:  pathList,
				nextToken: token,
			}
			c.pathMap.Store(pathKey, item)
			c.pathLRU.Add(pathKey)
		}
		return pathList, token, err
	} else {
		log.Debug("EntryCache::StreamDir : Serving list from cache for path: %s, token %s", options.Name, options.Token)
		item := pathEntry.(pathCacheItem)
		return item.children, item.nextToken, nil
	}
}

// pathEvict : Callback when a node from cache expires
func (c *EntryCache) pathEvict(node *list.Element) {
	pathKey := node.Value.(string)

	flock := c.pathLocks.Get(pathKey)
	flock.Lock()
	defer flock.Unlock()

	log.Debug("EntryCache::pathEvict : Expiry for path %s", pathKey)
	c.pathMap.Delete(pathKey)
}

// ------------------------- Factory -------------------------------------------

// Pipeline will call this method to create your object, initialize your variables here
// << DO NOT DELETE ANY AUTO GENERATED CODE HERE >>
func NewEntryCacheComponent() internal.Component {
	comp := &EntryCache{}
	comp.SetName(compName)
	return comp
}

// On init register this component to pipeline and supply your constructor
func init() {
	internal.AddComponent(compName, NewEntryCacheComponent)

	entryTimeout := config.AddUint32Flag("list-cache-timeout", defaultEntryCacheTimeout, "list entry timeout")
	config.BindPFlag(compName+".timeout-sec", entryTimeout)
}
