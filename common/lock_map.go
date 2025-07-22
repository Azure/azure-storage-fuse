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

package common

import (
	"sync"
	"time"
)

// Lock item for each file
type LockMapItem struct {
	handleCount  uint32
	exLocked     bool
	mtx          sync.Mutex
	downloadTime time.Time
}

// Map holding locks for all the files
type LockMap struct {
	locks sync.Map
}

func NewLockMap() *LockMap {
	return &LockMap{}
}

// Map level operations

// Get the lock item based on file name, if item does not exists create it
func (l *LockMap) Get(name string) *LockMapItem {
	lockIntf, _ := l.locks.LoadOrStore(name, &LockMapItem{handleCount: 0, exLocked: false})
	item := lockIntf.(*LockMapItem)
	return item
}

// Delete item from file lock map
func (l *LockMap) Delete(name string) {
	l.locks.Delete(name)
}

// Check if this file is already exLocked or not
func (l *LockMap) Locked(name string) bool {
	lockIntf, ok := l.locks.Load(name)
	if ok {
		item := lockIntf.(*LockMapItem)
		return item.exLocked
	}

	return false
}

// Lock Item level operation
// Lock this file exclusively
func (l *LockMapItem) Lock() {
	l.mtx.Lock()
	l.exLocked = true
}

// UnLock this file exclusively
func (l *LockMapItem) Unlock() {
	l.exLocked = false
	l.mtx.Unlock()
}

// Increment the handle count
func (l *LockMapItem) Inc() {
	l.handleCount++
}

// Decrement the handle count
func (l *LockMapItem) Dec() {
	l.handleCount--
}

// Get the current handle count
func (l *LockMapItem) Count() uint32 {
	return l.handleCount
}

// Set the download time of the file
func (l *LockMapItem) SetDownloadTime() {
	l.downloadTime = time.Now()
}

// Get the download time of the file
func (l *LockMapItem) DownloadTime() time.Time {
	return l.downloadTime
}
pio install
