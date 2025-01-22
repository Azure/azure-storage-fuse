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

package file_cache

import (
	"os"
	"strings"
	"sync"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
)

type lruNode struct {
	next    *lruNode
	prev    *lruNode
	usage   int
	deleted bool
	name    string
}

type lruPolicy struct {
	sync.Mutex
	cachePolicyConfig

	nodeMap sync.Map

	head       *lruNode
	currMarker *lruNode
	lastMarker *lruNode

	// Channel to close main channel select loop
	closeSignal         chan int
	closeSignalValidate chan int

	// Channel to contain files that needs to be deleted immediately
	deleteEvent chan string

	// Channel to contain files that are in use so push them up in lru list
	validateChan chan string

	// Channel to check disk usage is within the limits configured or not
	diskUsageMonitor <-chan time.Time

	// Channel to check for file eviction based on file-cache timeout
	cacheTimeoutMonitor <-chan time.Time

	// DU utility was found on the path or not
	duPresent bool
}

const (
	// Check for file expiry in below number of seconds
	CacheTimeoutCheckInterval = 5

	// Check for disk usage in below number of minutes
	DiskUsageCheckInterval = 1
)

var _ cachePolicy = &lruPolicy{}

func NewLRUPolicy(cfg cachePolicyConfig) cachePolicy {
	obj := &lruPolicy{
		cachePolicyConfig: cfg,
		head:              nil,
		currMarker: &lruNode{
			name:  "__",
			usage: -1,
		},
		lastMarker: &lruNode{
			name:  "##",
			usage: -1,
		},
		duPresent: false,
	}

	return obj
}

func (p *lruPolicy) StartPolicy() error {
	log.Trace("lruPolicy::StartPolicy")
	p.currMarker.prev = nil
	p.currMarker.next = p.lastMarker
	p.lastMarker.prev = p.currMarker
	p.lastMarker.next = nil
	p.head = p.currMarker

	p.closeSignal = make(chan int)
	p.closeSignalValidate = make(chan int)

	p.deleteEvent = make(chan string, 1000)
	p.validateChan = make(chan string, 10000)

	_, err := common.GetUsage(p.tmpPath)
	if err == nil {
		p.duPresent = true
	} else {
		log.Err("lruPolicy::StartPolicy : 'du' command not found, disabling disk usage checks")
	}

	if p.duPresent {
		p.diskUsageMonitor = time.Tick(time.Duration(DiskUsageCheckInterval * time.Minute))
	}

	// Only start the timeoutMonitor if evictTime is non-zero.
	// If evictTime=0, we delete on invalidate so there is no need for a timeout monitor signal to be sent.
	log.Info("lruPolicy::StartPolicy : Policy set with %v timeout", p.cacheTimeout)

	if p.cacheTimeout != 0 {
		p.cacheTimeoutMonitor = time.Tick(time.Duration(time.Duration(p.cacheTimeout) * time.Second))
	}

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
	p.highThreshold = c.highThreshold
	p.lowThreshold = c.lowThreshold
	p.maxEviction = c.maxEviction
	p.policyTrace = c.policyTrace
	return nil
}

func (p *lruPolicy) CacheValid(name string) {
	_, found := p.nodeMap.Load(name)
	if !found {
		p.cacheValidate(name)
	} else {
		p.validateChan <- name
	}
}

func (p *lruPolicy) CacheInvalidate(name string) {
	log.Trace("lruPolicy::CacheInvalidate : %s", name)

	// We check if the file is not in the nodeMap to deal with the case
	// where timeout is 0 and there are multiple handles open to the file.
	// When the first close comes, we will remove the entry from the map
	// and attempt to delete the file. This deletion will fail (and be skipped)
	// since there are other open handles. When the last close comes in, the map
	// will be clean so we we need to try deleting the file.
	_, found := p.nodeMap.Load(name)
	if p.cacheTimeout == 0 || !found {
		p.CachePurge(name)
	}
}

func (p *lruPolicy) CachePurge(name string) {
	log.Trace("lruPolicy::CachePurge : %s", name)

	p.removeNode(name)
	p.deleteEvent <- name
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

// On validate name of the file was pushed on this channel so now update the LRU list
func (p *lruPolicy) asyncCacheValid() {
	for {
		select {
		case name := <-p.validateChan:
			p.cacheValidate(name)

		case <-p.closeSignalValidate:
			return
		}
	}
}

func (p *lruPolicy) cacheValidate(name string) {
	var node *lruNode = nil

	val, found := p.nodeMap.Load(name)
	if !found {
		node = &lruNode{
			name:    name,
			next:    nil,
			prev:    nil,
			usage:   0,
			deleted: false,
		}
		p.nodeMap.Store(name, node)
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
	node.usage++

}

// For all other timer based activities we check the stuff here
func (p *lruPolicy) clearCache() {
	log.Trace("lruPolicy::ClearCache")

	for {
		select {
		case name := <-p.deleteEvent:
			log.Trace("lruPolicy::Clear-delete")
			// we are asked to delete file explicitly
			p.deleteItem(name)

		case <-p.cacheTimeoutMonitor:
			log.Trace("lruPolicy::Clear-timeout monitor")
			// File cache timeout has hit so delete all unused files for past N seconds
			p.updateMarker()
			p.printNodes()
			p.deleteExpiredNodes()

		case <-p.diskUsageMonitor:
			// File cache timeout has not occurred so just monitor the cache usage
			cleanupCount := 0
			pUsage := getUsagePercentage(p.tmpPath, p.maxSizeMB)
			if pUsage > p.highThreshold {
				continueDeletion := true
				for continueDeletion {
					log.Info("lruPolicy::ClearCache : High threshold reached %f > %f", pUsage, p.highThreshold)

					cleanupCount++
					p.updateMarker()
					p.printNodes()
					p.deleteExpiredNodes()

					pUsage := getUsagePercentage(p.tmpPath, p.maxSizeMB)
					if pUsage < p.lowThreshold || cleanupCount >= 3 {
						log.Info("lruPolicy::ClearCache : Threshold stabilized %f > %f", pUsage, p.lowThreshold)
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

	var node *lruNode = nil

	val, found := p.nodeMap.Load(name)
	if !found || val == nil {
		return
	}

	p.nodeMap.Delete(name)

	p.Lock()
	defer p.Unlock()

	node = val.(*lruNode)
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
		log.Debug("lruPolicy::DeleteExpiredNodes : Max deletion count hit")
	}

	p.lastMarker.next = node
	if node != nil {
		node.prev = p.lastMarker
	}
	p.Unlock()

	log.Debug("lruPolicy::deleteExpiredNodes : List generated %d items", count)

	for _, item := range delItems {
		if item.deleted {
			p.removeNode(item.name)
			p.deleteItem(item.name)
		}
	}

	log.Debug("lruPolicy::deleteExpiredNodes : Ends")
}

func (p *lruPolicy) deleteItem(name string) {
	log.Trace("lruPolicy::deleteItem : Deleting %s", name)

	azPath := strings.TrimPrefix(name, p.tmpPath)
	if azPath == "" {
		log.Err("lruPolicy::DeleteItem : Empty file name formed name : %s, tmpPath : %s", name, p.tmpPath)
		return
	}

	if azPath[0] == '/' {
		azPath = azPath[1:]
	}

	flock := p.fileLocks.Get(azPath)
	if p.fileLocks.Locked(azPath) {
		log.Warn("lruPolicy::DeleteItem : File in under download %s", azPath)
		p.CacheValid(name)
		return
	}

	flock.Lock()
	defer flock.Unlock()

	// Check if there are any open handles to this file or not
	if flock.Count() > 0 {
		log.Warn("lruPolicy::DeleteItem : File in use %s", name)
		p.CacheValid(name)
		return
	}

	// There are no open handles for this file so its safe to remove this
	err := deleteFile(name)
	if err != nil && !os.IsNotExist(err) {
		log.Err("lruPolicy::DeleteItem : failed to delete local file %s [%s]", name, err.Error())
	}

	// File was deleted so try clearing its parent directory
	// TODO: Delete directories up the path recursively that are "safe to delete". Ensure there is no race between this code and code that creates directories (like OpenFile)
	// This might require something like hierarchical locking.
}

func (p *lruPolicy) printNodes() {
	if !p.policyTrace {
		return
	}

	node := p.head

	var count int = 0
	log.Debug("lruPolicy::printNodes : Starts")

	for ; node != nil; node = node.next {
		log.Debug(" ==> (%d) %s", count, node.name)
		count++
	}

	log.Debug("lruPolicy::printNodes : Ends")
}
