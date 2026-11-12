// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package common

import (
	"sync"
	"sync/atomic"
	"time"
)

// Lock item for each file
type LockMapItem struct {
	handleCount  uint32
	exLocked     atomic.Bool
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
	lockIntf, _ := l.locks.LoadOrStore(name, &LockMapItem{})
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
		return item.exLocked.Load()
	}

	return false
}

// Lock Item level operation
// Lock this file exclusively
func (l *LockMapItem) Lock() {
	l.mtx.Lock()
	l.exLocked.Store(true)
}

// TryLock locks the file only if no other operation currently holds its lock.
func (l *LockMapItem) TryLock() bool {
	if !l.mtx.TryLock() {
		return false
	}
	l.exLocked.Store(true)
	return true
}

// UnLock this file exclusively
func (l *LockMapItem) Unlock() {
	l.exLocked.Store(false)
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
