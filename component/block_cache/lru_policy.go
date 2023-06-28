/*
    _____           _____   _____   ____          ______  _____  ------
   |     |  |      |     | |     | |     |     | |       |            |
   |     |  |      |     | |     | |     |     | |       |            |
   | --- |  |      |     | |-----| |---- |     | |-----| |-----  ------
   |     |  |      |     | |     | |     |     |       | |       |
   | ____|  |_____ | ____| | ____| |     |_____|  _____| |_____  |_____


   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2020-2023 Microsoft Corporation. All rights reserved.
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

package block_cache

import (
	"fmt"
	"sync"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common/log"
)

const (
	MAX_POOL_USAGE int32 = 80
	MIN_POOL_USAGE int32 = 50
)

type cachePolicyConfig struct {
	cacheTimeout uint32
	maxEviction  uint32

	maxSizeMB float64
}

type lruNode struct {
	next    *lruNode
	prev    *lruNode
	deleted bool
	name    string
	node *CacheNode
}

type lruPolicy struct {
	sync.Mutex
	cachePolicyConfig

	nodeMap   sync.Map
	blockPool *BlockPool

	head       *lruNode
	currMarker *lruNode
	lastMarker *lruNode

	// Channel to close main channel select loop
	closeSignal         chan int
	closeSignalValidate chan int

	// Channel to contain blocks that needs to be deleted immediately
	deleteEvent chan *CacheNode

	// Channel to contain block that are in use so push them up in lru list
	validateChan chan *CacheNode

	// Channel to check pool usage is within the limits configured or not
	poolUsageMonitor <-chan time.Time

	// Channel to check for block eviction based on block-cache timeout
	cacheTimeoutMonitor <-chan time.Time
}

const (
	// Check for block expiry in below number of seconds
	CacheTimeoutCheckInterval = 5

	// Check for pool usage in below number of seconds
	PoolUsageCheckInterval = 30
)

func NewLRUPolicy(cfg cachePolicyConfig) *lruPolicy {
	obj := &lruPolicy{
		cachePolicyConfig: cfg,
		head:              nil,
		currMarker: &lruNode{
			name: "__",
		},
		lastMarker: &lruNode{
			name: "##",
		},
	}

	return obj
}

func (p *lruPolicy) StartPolicy(pool *BlockPool) error {
	log.Trace("lruPolicy::StartPolicy")
	p.currMarker.prev = nil
	p.currMarker.next = p.lastMarker
	p.lastMarker.prev = p.currMarker
	p.lastMarker.next = nil
	p.head = p.currMarker

	p.closeSignal = make(chan int)
	p.closeSignalValidate = make(chan int)

	p.deleteEvent = make(chan *CacheNode, 1000)
	p.validateChan = make(chan *CacheNode, 10000)

	p.poolUsageMonitor = time.Tick(time.Duration(PoolUsageCheckInterval * time.Second))

	// Only start the timeoutMonitor if evictTime is non-zero.
	// If evictTime=0, we delete on invalidate so there is no need for a timeout monitor signal to be sent.
	if p.cacheTimeout != 0 {
		p.cacheTimeoutMonitor = time.Tick(time.Duration(time.Duration(p.cacheTimeout) * time.Second))
	}
	
	p.blockPool = pool
	
	go p.clearCache()
	go p.asyncCacheValid()
	
	return nil

}

func (p *lruPolicy) ShutdownPolicy() error {
	log.Trace("lruPolicy::ShutdownPolicy")
	p.closeSignal <- 1
	p.closeSignalValidate <- 1
	return nil
}

func (p *lruPolicy) UpdateConfig(c cachePolicyConfig) error {
	log.Trace("lruPolicy::UpdateConfig")
	p.maxSizeMB = c.maxSizeMB
	p.maxEviction = c.maxEviction
	return nil
}

func (p *lruPolicy) CacheValid(node *CacheNode) {
	_, found := p.nodeMap.Load(node.name)
	if !found {
		p.cacheValidate(node)
	} else {
		p.validateChan <- node
	}
}

func (p *lruPolicy) CacheInvalidate(node *CacheNode) {
	log.Trace("lruPolicy::CacheInvalidate : %s", node.name)

	_, found := p.nodeMap.Load(node.name)
	if p.cacheTimeout == 0 || !found {
		p.CachePurge(node)
	}
}

func (p *lruPolicy) CachePurge(node *CacheNode) {
	log.Trace("lruPolicy::CachePurge : %s", node.name)

	p.removeNode(node.name)
	p.deleteEvent <- node
}

func (p *lruPolicy) IsCached(name string) bool {
	log.Trace("lruPolicy::IsCached : %s", name)

	val, found := p.nodeMap.Load(name)
	if found {
		node := val.(*lruNode)
		log.Debug("lruPolicy::IsCached : %s, deleted:%t", name, node.deleted)
		if !node.deleted {
			return true
		}
	}
	log.Trace("lruPolicy::IsCached : %s, found %t", name, found)
	return false
}

func (p *lruPolicy) Name() string {
	return "lru"
}

// On validate name of the block was pushed on this channel so now update the LRU list
func (p *lruPolicy) asyncCacheValid() {
	for {
		select {
		case node := <-p.validateChan:
			p.cacheValidate(node)

		case <-p.closeSignalValidate:
			return
		}
	}
}

func (p *lruPolicy) cacheValidate(cacheNode *CacheNode) {
	var node *lruNode = nil
	
	val, found := p.nodeMap.Load(cacheNode.name)
	if !found {
		node = &lruNode{
			name:    cacheNode.name,
			node:    cacheNode,
			next:    nil,
			prev:    nil,
			deleted: false,
		}
		p.nodeMap.Store(cacheNode.name, node)
	} else {
		node = val.(*lruNode)
	}

	p.Lock()
	defer p.Unlock()

	node.deleted = false

	if node == p.head {
		return
	}

	if node.next != nil {
		node.next.prev = node.prev
	}

	if node.prev != nil {
		node.prev.next = node.next
	}

	node.prev = nil
	node.next = p.head

	p.head.prev = node
	p.head = node
}

// For all other timer based activities we check the stuff here
func (p *lruPolicy) clearCache() {
	log.Trace("lruPolicy::ClearCache")

	for {
		select {
		case item := <-p.deleteEvent:
			log.Trace("lruPolicy::Clear-delete")
			// we are asked to delete block explicitly
			p.deleteItem(item)

		case <-p.cacheTimeoutMonitor:
			log.Trace("lruPolicy::Clear-timeout monitor")
			// Block cache timeout has hit so delete all unused blokcs for past N seconds
			p.updateMarker()
			p.deleteExpiredNodes()

		case <-p.poolUsageMonitor:
			// Block cache timeout has not occurred so just monitor the cache usage
			cleanupCount := 0

			pUsage := p.blockPool.Usage()
			if pUsage > MAX_POOL_USAGE {
				continueDeletion := true
				for continueDeletion {
					log.Err("[[[]]]lruPolicy::ClearCache : High threshold reached %f > %f", pUsage, MAX_POOL_USAGE)

					cleanupCount++
					p.updateMarker()
					p.deleteExpiredNodes()

					pUsage := p.blockPool.Usage()
					if pUsage < MIN_POOL_USAGE || cleanupCount >= 3 {
						log.Err("[[[]]]lruPolicy::ClearCache : Threshold stablized %f > %f", pUsage, MIN_POOL_USAGE)
						continueDeletion = false
					}
				}
			}
			
		case <-p.closeSignal:
			return
		}
	}
}

func (p *lruPolicy) removeNode(name string) {
	log.Trace("lruPolicy::removeNode : %s", name)

	val, found := p.nodeMap.Load(name)
	if !found || val == nil {
		return
	}

	p.nodeMap.Delete(name)

	node := val.(*lruNode)
	p.Lock()
	defer p.Unlock()

	node.deleted = true
	if node == p.head {
		p.head = node.next
		p.head.prev = nil
		node.next = nil
		return
	}

	if node.next != nil {
		node.next.prev = node.prev
	}

	if node.prev != nil {
		node.prev.next = node.next
	}
	node.prev = nil
	node.next = nil
}

func (p *lruPolicy) updateMarker() {
	log.Trace("lruPolicy::updateMarker")

	p.Lock()
	node := p.lastMarker
	if node.next != nil {
		node.next.prev = node.prev
	}

	if node.prev != nil {
		node.prev.next = node.next
	}
	node.prev = nil
	node.next = p.head
	p.head.prev = node
	p.head = node

	p.lastMarker = p.currMarker
	p.currMarker = node

	p.Unlock()
}

func (p *lruPolicy) deleteExpiredNodes() {
	log.Debug("lruPolicy::deleteExpiredNodes : Starts")

	if p.lastMarker.next == nil {
		return
	}

	delItems := make([]*lruNode, 0)
	count := uint32(0)

	p.Lock()
	node := p.lastMarker.next
	p.lastMarker.next = nil

	if node != nil {
		node.prev = nil
	}

	for ; node != nil && count < p.maxEviction; node = node.next {
		delItems = append(delItems, node)
		node.deleted = true
		count++
	}

	if count >= p.maxEviction {
		log.Err("@@@lruPolicy::DeleteExpiredNodes : Max deletion count hit")
	}

	p.lastMarker.next = node
	if node != nil {
		node.prev = p.lastMarker
	}
	p.Unlock()

	log.Err("@@@lruPolicy::deleteExpiredNodes : List generated %d items", count)

	for _, item := range delItems {
		if item.deleted {
			p.removeNode(item.name)
			p.deleteItem(item.node)
		}
	}

	log.Err("@@@lruPolicy::deleteExpiredNodes : Ends")
}

func (p *lruPolicy) deleteItem(node *CacheNode) {

	log.Err("@@@lruPolicy::deleteItem : Deleting %s", node.name)

	node.lock.Lock()
	defer node.lock.Unlock()
	<- node.block.state
	node.handle.RemoveValue(fmt.Sprintf("%v", node.block.id))
	p.blockPool.Release(node.block)
	
	node.block = nil
}
