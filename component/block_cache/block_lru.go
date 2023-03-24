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
	"sync"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common/log"
)

// State of a block
const (
	BlockFree uint16 = iota
	BlockCached
	BlockPersisted
)

// lruNode : Node in the LRU list
type lruNode struct {
	// LRU pointers for linked list
	next *lruNode
	prev *lruNode

	// State of the current block
	state uint16

	// Length of data stored in this block
	length uint32

	// Slice holding data
	data []byte
}

type BlockLRU struct {
	mmtx sync.RWMutex
	lmtx sync.RWMutex

	nodeMap map[string]*lruNode

	count   uint32
	max     uint32
	timeout uint32

	head       *lruNode
	currMarker *lruNode
	lastMarker *lruNode

	// Timeout on which blocks will be marked unused
	cacheTimeout <-chan time.Time

	// Channel to close the manager thread
	close chan interface{}
	done chan interface{}

	// Channel to hold blocks to be refreshed
	refresh <-chan *lruNode
}

func NewBlockLRU(maxNodes uint32, timeout uint32) BlockLRU {
	return BlockLRU{
		max:        maxNodes,
		count:      0,
		timeout:    timeout,
		head:       nil,
		currMarker: &lruNode{},
		lastMarker: &lruNode{},
		nodeMap: make(map[string]*lruNode)
	}
}

func (lru *BlockLRU) Start() error {
	lru.currMarker.prev = nil
	lru.currMarker.next = lru.lastMarker
	lru.lastMarker.prev = lru.currMarker
	lru.lastMarker.next = nil
	lru.head = lru.currMarker

	if lru.timeout != 0 {
		lru.cacheTimeout = time.Tick(time.Duration(time.Duration(lru.timeout) * time.Second))
	}

	lru.close = make(chan interface{})
	lru.done = make(chan interface{})

	go lru.manager()
	return nil
}

func (lru *BlockLRU) Stop() error {
	lru.close <- 1

	<- lru.done
	return nil
}

func (lru *BlockLRU) manager() {
	log.Trace("BlockLRU::manager")

	for {
		select {
		case <-lru.cacheTimeout:
			// Timeout so lets eliminate some nodes
			lru.updateMarker()

		case node := <- lru.refresh:
			// This node is accessed so move this node to the fron to the LRU
			lru.LruLock.Lock()
			pullToFront(node)
			lru.LruLock.Lock()
			
		case <-lru.close:
			return
		}
	}
}

func (lru *BlockLRU) updateMarker() {
	log.Trace("BlockLRU::updateMarker")

	lru.lmtx..Lock()
	defer lru.lmtx.Unlock()

	// Make current marker to move to head
	// Make last marker to move to previous current marker position
	node := lru.lastMarker
	if node.next != nil {
		node.next.prev = node.prev
	}

	if node.prev != nil {
		node.prev.next = node.next
	}

	node.prev = nil
	node.next = lru.head
	lru.head.prev = node

	lru.head = node

	lru.lastMarker = lru.currMarker
	lru.currMarker = node
}

func (lru *BlockLRU) pullToFront(node *lruNode) {
	if lru.head != node {
		if node.next != nil {
			node.next.prev = node.prev
		}
	
		if node.prev != nil {
			node.prev.next = node.next
		}
	
		node.prev = nil
		node.next = lru.head
		lru.head.prev = node
		lru.head = node
	}	
}

func (lru *BlockLRU) AddOrRefresh(name string) {
	lru.mmtx.RLock()
	node, ok := lru.nodeMap[name]
	lru.mmtx.Runlock()

	if !ok {
		// This node is not found so create new node and insert in LRU
		lru.lmtx.Lock()
		node := &lruNode{
			state: BlockCached,

		}

		pullToFront(node)
		lru.lmtx.Unlock()

		// Add this new node to map
		lru.mmtx.Lock()
		nodeMap[name] = node
		lru.mmtx.Unlock()

	} else {
		// Node exists to just pull up this node in LRU
		p.refresh <- node
	}
}
