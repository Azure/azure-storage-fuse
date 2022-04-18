/*
    _____           _____   _____   ____          ______  _____  ------
   |     |  |      |     | |     | |     |     | |       |            |
   |     |  |      |     | |     | |     |     | |       |            |
   | --- |  |      |     | |-----| |---- |     | |-----| |-----  ------
   |     |  |      |     | |     | |     |     |       | |       |
   | ____|  |_____ | ____| | ____| |     |_____|  _____| |_____  |_____


   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2020-2022 Microsoft Corporation. All rights reserved.
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
	"blobfuse2/common/log"
	"os"
	"strings"
	"sync"
	"time"
)

type lfuPolicy struct {
	sync.Mutex
	cachePolicyConfig
	list        *lfuList
	removeFiles chan string
	closeChan   chan int
}

var _ cachePolicy = &lfuPolicy{}

func (l *lfuPolicy) StartPolicy() error {
	log.Trace("lfuPolicy::StartPolicy")

	go l.clearCache()
	return nil
}

func (l *lfuPolicy) ShutdownPolicy() error {
	log.Trace("lfuPolicy::ShutdownPolicy")

	l.closeChan <- 1
	return nil
}

func (l *lfuPolicy) UpdateConfig(config cachePolicyConfig) error {
	log.Trace("lfuPolicy::UpdateConfig")

	l.maxSizeMB = config.maxSizeMB
	l.highThreshold = config.highThreshold
	l.lowThreshold = config.lowThreshold
	l.maxEviction = config.maxEviction

	l.list.maxSizeMB = config.maxSizeMB
	l.list.upperThresh = config.highThreshold
	l.list.lowerThresh = config.lowThreshold
	l.list.cacheTimeout = config.cacheTimeout

	l.policyTrace = config.policyTrace
	return nil
}

func (l *lfuPolicy) CacheValid(name string) error {
	log.Trace("lfuPolicy::CacheValid : %s", name)

	l.list.Lock()
	defer l.list.Unlock()

	l.list.put(name)
	return nil
}

func (l *lfuPolicy) CacheInvalidate(name string) error {
	log.Trace("lfuPolicy::CacheInvalidate : %s", name)

	if l.cacheTimeout == 0 {
		return l.CachePurge(name)
	}

	return nil
}

func (l *lfuPolicy) CachePurge(name string) error {
	log.Trace("lfuPolicy::CachePurge : %s", name)

	l.list.Lock()
	defer l.list.Unlock()

	l.list.delete(name)
	l.removeFiles <- name

	return nil
}

func (l *lfuPolicy) IsCached(name string) bool {
	log.Trace("lfuPolicy::IsCached : %s", name)

	l.list.Lock()
	defer l.list.Unlock()

	val := l.list.get(name)
	if val != nil {
		log.Debug("lfuPolicy::IsCached : %s found", name)
		return true
	} else {
		log.Debug("lfuPolicy::IsCached : %s not found", name)
		return false
	}
}

func (l *lfuPolicy) Name() string {
	return "lfu"
}

func (l *lfuPolicy) clearItemFromCache(path string) {
	azPath := strings.TrimPrefix(path, l.tmpPath)
	if azPath[0] == '/' {
		azPath = azPath[1:]
	}

	flock := l.fileLocks.Get(azPath)
	flock.Lock()
	defer flock.Unlock()

	// Check if there are any open handles to this file or not
	if flock.Count() > 0 {
		log.Err("lfuPolicy::clearItemFromCache : File in use %s", path)
		l.CacheValid(path)
		return
	}

	// There are no open handles for this file so its safe to remove this
	deleteFile(path)
}

func (l *lfuPolicy) clearCache() {
	log.Trace("lfuPolicy::clearCache")

	for {
		select {

		case path := <-l.removeFiles:
			l.clearItemFromCache(path)

		case <-l.closeChan:
			return
		}
	}

}

func rethrowOnUnblock(f *os.File, path string, throwChan chan string) {
	log.Trace("lfuPolicy::rethrowOnUnblock : %s", path)

	log.Debug("lfuPolicy::rethrowOnUnblock : ex lock acquired [%s]", path)
	throwChan <- path
}

func NewLFUPolicy(cfg cachePolicyConfig) cachePolicy {
	pol := &lfuPolicy{
		cachePolicyConfig: cfg,
		removeFiles:       make(chan string, 10),
		closeChan:         make(chan int, 10),
	}
	pol.list = newLFUList(cfg.maxSizeMB, cfg.lowThreshold, cfg.highThreshold, pol.removeFiles, cfg.tmpPath, cfg.cacheTimeout)
	return pol
}

//Double DoublyLinkedList Implementation for O(1) lfu

type dataNode struct {
	key       string
	frequency uint64
	next      *dataNode
	prev      *dataNode
	timer     *time.Timer
}

func newDataNode(key string) *dataNode {
	return &dataNode{
		key:       key,
		frequency: 1,
	}
}

type dataNodeLinkedList struct {
	size  uint64
	first *dataNode
	last  *dataNode
}

func (dl *dataNodeLinkedList) pop() *dataNode {
	if dl.size == 0 {
		return nil
	}
	return dl.remove(dl.first)
}

func (dl *dataNodeLinkedList) remove(node *dataNode) *dataNode {
	if dl.size == 0 {
		return nil
	}
	if dl.first == dl.last {
		dl.first = nil
		dl.last = nil
	} else if dl.first == node {
		temp := dl.first
		dl.first = temp.next
		temp.next = nil
		dl.first.prev = nil
	} else if dl.last == node {
		temp := dl.last
		dl.last = temp.prev
		temp.prev = nil
		dl.last.next = nil
	} else {
		nextNode := node.next
		prevNode := node.prev
		prevNode.next = nextNode
		nextNode.prev = prevNode
		node.next = nil
		node.prev = nil
	}
	dl.size--
	return node
}

func (dl *dataNodeLinkedList) push(node *dataNode) {
	if dl.first == nil {
		dl.first = node
		dl.last = node
	} else {
		temp := dl.last
		temp.next = node
		node.prev = temp
		dl.last = node
	}
	dl.size++
}

func newDataNodeLinkedList() *dataNodeLinkedList {
	return &dataNodeLinkedList{}
}

type frequencyNode struct {
	list      *dataNodeLinkedList
	next      *frequencyNode
	prev      *frequencyNode
	frequency uint64
}

func (fn *frequencyNode) pop() *dataNode {
	return fn.list.pop()
}

func (fn *frequencyNode) remove(dn *dataNode) *dataNode {
	return fn.list.remove(dn)
}

func (fn *frequencyNode) push(dn *dataNode) {
	fn.list.push(dn)
}

func newFrequencyNode(freq uint64) *frequencyNode {
	return &frequencyNode{
		list:      newDataNodeLinkedList(),
		frequency: freq,
	}
}

type lfuList struct {
	sync.Mutex
	first        *frequencyNode
	last         *frequencyNode
	dataNodeMap  map[string]*dataNode
	freqNodeMap  map[uint64]*frequencyNode
	size         uint64
	maxSizeMB    float64
	lowerThresh  float64
	upperThresh  float64
	deleteFiles  chan string
	cachePath    string
	cacheAge     uint64
	cacheTimeout uint32
}

func (list *lfuList) deleteFrequency(freq uint64) {
	freqNode := list.freqNodeMap[freq]
	if list.first == list.last {
		list.first = nil
		list.last = nil
	} else if list.first == freqNode {
		list.first = list.first.next
		list.first.prev = nil
		freqNode.next = nil
	} else if list.last == freqNode {
		list.last = list.last.prev
		list.last.next = nil
		freqNode.prev = nil
	} else {
		nextNode := freqNode.next
		prevNode := freqNode.prev
		nextNode.prev = prevNode
		prevNode.next = nextNode
		freqNode.next = nil
		freqNode.prev = nil
	}
	list.size--
	delete(list.freqNodeMap, freq)
}

func (list *lfuList) addFrequency(freq uint64, freqNode *frequencyNode, prevFreqNode *frequencyNode) {

	if list.first == nil && list.last == nil {
		list.first = freqNode
		list.last = freqNode

		list.freqNodeMap[freq] = freqNode
		list.size++
		return
	}

	if prevFreqNode == nil {
		prevFreqNode = list.first
	}

	for prevFreqNode.next != nil && freq > prevFreqNode.next.frequency {
		prevFreqNode = prevFreqNode.next
	}

	if prevFreqNode == nil {
		freqNode.next = list.first
		list.first.prev = freqNode
		list.first = freqNode
	} else if prevFreqNode == list.last {
		prevFreqNode.next = freqNode
		freqNode.prev = prevFreqNode
		list.last = freqNode
	} else {
		nextNode := prevFreqNode.next
		freqNode.next = nextNode
		nextNode.prev = freqNode
		prevFreqNode.next = freqNode
		freqNode.prev = prevFreqNode
	}
	list.freqNodeMap[freq] = freqNode
	list.size++
}

func (list *lfuList) promote(dataNode *dataNode) {
	prevFreqNode := list.freqNodeMap[dataNode.frequency]
	prevFreqNode.remove(dataNode)
	dataNode.frequency += 1 + list.cacheAge
	if newFreqNode, ok := list.freqNodeMap[dataNode.frequency]; ok {
		newFreqNode.push(dataNode)
	} else {
		newFreqNode := newFrequencyNode(dataNode.frequency)
		list.addFrequency(dataNode.frequency, newFreqNode, prevFreqNode)
		list.freqNodeMap[dataNode.frequency] = newFreqNode
		newFreqNode.push(dataNode)
	}

	if prevFreqNode.list.size == 0 {
		list.deleteFrequency(prevFreqNode.frequency)
		list.size--
	}
}

func (list *lfuList) get(key string) *dataNode {
	if node, ok := list.dataNodeMap[key]; ok {
		if list.cacheTimeout > 0 {
			node.timer.Stop()
		}
		list.promote(node)
		list.setTimerIfValid(node)
		return node
	} else {
		return nil
	}
}

//Requires Lock()
func (list *lfuList) put(key string) {
	if node, ok := list.dataNodeMap[key]; ok {
		if list.cacheTimeout > 0 {
			node.timer.Stop()
		}
		list.promote(node)
		list.setTimerIfValid(node)
	} else {
		if usage := getUsagePercentage(list.cachePath, list.maxSizeMB); usage > list.upperThresh {
			for usage > list.lowerThresh && list.first != nil {
				toDeletePath := list.first.list.first.key
				list.first.pop()
				delete(list.dataNodeMap, toDeletePath)
				if list.first.list.size == 0 {
					list.deleteFrequency(list.first.frequency)
					list.size--
					usage = getUsagePercentage(list.cachePath, list.maxSizeMB)
				}
				list.deleteFiles <- toDeletePath
			}
		}
		newNode := newDataNode(key)
		list.dataNodeMap[key] = newNode
		if freqNode, ok := list.freqNodeMap[newNode.frequency]; ok {
			freqNode.push(newNode)
		} else {
			freqNode := newFrequencyNode(newNode.frequency)
			list.freqNodeMap[newNode.frequency] = freqNode
			freqNode.push(newNode)
			list.addFrequency(newNode.frequency, freqNode, nil)
		}
		list.setTimerIfValid(newNode)
	}
}

//Requires Lock()
func (list *lfuList) delete(key string) {
	if node, ok := list.dataNodeMap[key]; ok {
		if list.cacheTimeout > 0 {
			node.timer.Stop()
		}
		freqNode := list.freqNodeMap[node.frequency]
		freqNode.remove(node)
		delete(list.dataNodeMap, key)
		if freqNode.list.size == 0 {
			list.deleteFrequency(node.frequency)
			list.size--
		}
		list.deleteFiles <- node.key
		list.cacheAge = node.frequency
	}
}

func (list *lfuList) setTimerIfValid(node *dataNode) {
	if list.cacheTimeout > 0 {
		timer := time.AfterFunc(time.Duration(list.cacheTimeout)*time.Second, func() {
			list.Lock()
			list.delete(node.key)
			list.Unlock()
		})
		node.timer = timer
	}
}

func newLFUList(maxSizMB float64, lowerThresh float64, upperThresh float64, deleteChan chan string, cachePath string, cacheTimeout uint32) *lfuList {
	return &lfuList{
		dataNodeMap:  make(map[string]*dataNode),
		freqNodeMap:  make(map[uint64]*frequencyNode),
		size:         0,
		maxSizeMB:    maxSizMB,
		lowerThresh:  lowerThresh,
		upperThresh:  upperThresh,
		deleteFiles:  deleteChan,
		cachePath:    cachePath,
		cacheTimeout: cacheTimeout,
	}
}
