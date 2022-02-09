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

package cacheheap

import "sync"

type CacheFileAttr struct {
	Times int64
	Path  string
}

type Heap struct {
	sync.RWMutex
	fileData      []*CacheFileAttr
	filePathToIdx map[string]int
}

func (h *Heap) parent(i int) int {
	return (i - 1) / 2
}

func (h *Heap) left(i int) int {
	return (2 * i) + 1
}

func (h *Heap) right(i int) int {
	return (2 * i) + 2
}

func (h *Heap) heapify(i int) {
	if len(h.fileData) <= 1 {
		return
	}
	h.RLock()
	least := i
	leastInfo := h.fileData[least]
	l := h.left(i)
	r := h.right(i)
	var lInfo, rInfo *CacheFileAttr
	if l < len(h.fileData) {
		lInfo = h.fileData[l]
	}
	if r < len(h.fileData) {
		rInfo = h.fileData[r]
	}

	if l < len(h.fileData) && lInfo.Times < leastInfo.Times {
		least = l
		leastInfo = lInfo
	}
	if r < len(h.fileData) && rInfo.Times < leastInfo.Times {
		least = r
		leastInfo = rInfo
	}
	h.RUnlock()
	if least != i {
		h.Lock()
		ithInfo := h.fileData[i]
		//change indices of the Path to new indices
		h.filePathToIdx[ithInfo.Path] = least
		h.filePathToIdx[leastInfo.Path] = i

		//swap the CacheFileAttr directly in Heap
		h.fileData[i] = leastInfo
		h.fileData[least] = ithInfo
		h.Unlock()
		//reset the Heap structure after change
		h.heapify(least)
	}
}

//Requires Lock()
func (h *Heap) Increment(path string) {
	go func() {
		h.RLock()
		idx, ok := h.filePathToIdx[path]
		h.RUnlock()
		if ok {
			h.Lock()
			h.fileData[idx].Times += 1
			h.Unlock()
			h.heapify(idx)
		}
	}()
}

//Requires Lock()
func (h *Heap) Insert(path string) {
	info := &CacheFileAttr{
		Times: 1,
		Path:  path,
	}
	h.Lock()
	defer h.Unlock()
	h.fileData = append(h.fileData, info)
	h.filePathToIdx[path] = len(h.fileData) - 1

	i := len(h.fileData) - 1
	ithInfo := h.fileData[i]

	parent := h.parent(i)
	parentInfo := h.fileData[parent]
	for i != 0 && ithInfo.Times < parentInfo.Times {

		h.filePathToIdx[parentInfo.Path] = i
		h.filePathToIdx[ithInfo.Path] = parent

		h.fileData[i] = parentInfo
		h.fileData[parent] = ithInfo

		i = parent
		ithInfo = h.fileData[i]

		parent = h.parent(i)
		parentInfo = h.fileData[parent]
	}
}

func (h *Heap) HasValue(name string) bool {
	_, ok := h.filePathToIdx[name]
	return ok
}

func (h *Heap) InsertFromAttr(path string, info *CacheFileAttr) {
	h.Lock()
	defer h.Unlock()
	h.fileData = append(h.fileData, info)
	h.filePathToIdx[path] = len(h.fileData) - 1

	i := len(h.fileData) - 1
	ithInfo := h.fileData[i]

	parent := h.parent(i)
	parentInfo := h.fileData[parent]
	for i != 0 && ithInfo.Times < parentInfo.Times {

		h.filePathToIdx[parentInfo.Path] = i
		h.filePathToIdx[ithInfo.Path] = parent

		h.fileData[i] = parentInfo
		h.fileData[parent] = ithInfo

		i = parent
		ithInfo = h.fileData[i]

		parent = h.parent(i)
		parentInfo = h.fileData[parent]
	}
}

func (h *Heap) Delete(path string) {
	if len(h.fileData) == 0 {
		return
	}
	h.RLock()
	intf, ok := h.filePathToIdx[path]
	h.RUnlock()
	if !ok {
		return
	}
	toDeleteIdx := intf
	h.Lock()
	delete(h.filePathToIdx, path)
	h.Unlock()

	if toDeleteIdx == len(h.fileData)-1 {
		h.Lock()
		h.fileData = h.fileData[:len(h.fileData)-1]
		h.Unlock()
	} else if len(h.fileData) > 1 {
		h.Lock()
		lastInfo := h.fileData[len(h.fileData)-1]
		h.fileData[toDeleteIdx] = lastInfo
		h.filePathToIdx[lastInfo.Path] = toDeleteIdx
		h.fileData = h.fileData[:len(h.fileData)-1]
		h.Unlock()
		h.heapify(toDeleteIdx)
	} else {
		h.fileData = make([]*CacheFileAttr, 0)
		h.filePathToIdx = make(map[string]int)
	}
}

func (h *Heap) ExtractMin() string {
	if len(h.fileData) <= 0 {
		return ""
	}
	if len(h.fileData) == 1 {
		info := h.fileData[0]
		h.fileData = make([]*CacheFileAttr, 0)
		h.filePathToIdx = make(map[string]int)
		return info.Path
	}
	h.Lock()
	minInfo := h.fileData[0]
	h.fileData[0] = h.fileData[len(h.fileData)-1]
	zeroth := h.fileData[0]

	delete(h.filePathToIdx, minInfo.Path)
	h.filePathToIdx[zeroth.Path] = 0

	h.fileData = h.fileData[:len(h.fileData)-1]
	h.Unlock()

	h.heapify(0)
	return minInfo.Path
}

func (h *Heap) GetMin() *CacheFileAttr {
	h.RLock()
	defer h.RUnlock()
	if len(h.fileData) <= 0 {
		return nil
	}
	info := h.fileData[0]
	return info
}

func New() *Heap {
	return &Heap{
		fileData:      make([]*CacheFileAttr, 0),
		filePathToIdx: make(map[string]int),
	}
}
