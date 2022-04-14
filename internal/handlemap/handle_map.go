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

package handlemap

import (
	"blobfuse2/common"
	"os"
	"sync"

	"github.com/bluele/gcache"
	"go.uber.org/atomic"
)

type HandleID uint64

const InvalidHandleID HandleID = 0

// Flags represented in BitMap for various flags in the handle
const (
	HandleFlagUnknown uint16 = iota
	HandleFlagDirty
	HandleFlagFSynced
	HandleFlagCached
)

type Cache struct {
	DataBuffer gcache.Cache
}

type Handle struct {
	sync.RWMutex
	FObj     *os.File
	ID       HandleID
	Size     int64 // Size of the file being handled here
	Flags    common.BitMap16
	Path     string // always holds path relative to mount dir
	values   map[string]interface{}
	CacheObj *Cache
}

func NewHandle(path string) *Handle {
	return &Handle{
		ID:     InvalidHandleID,
		Path:   path,
		Size:   0,
		Flags:  0,
		values: make(map[string]interface{}),
	}
}

// Dirty : Handle is dirty or not
func (handle *Handle) Dirty() bool {
	return handle.Flags.IsSet(HandleFlagDirty)
}

// Fsynced : Handle is Fsynced or not
func (handle *Handle) Fsynced() bool {
	return handle.Flags.IsSet(HandleFlagFSynced)
}

// Cached : File is cached on local disk or not
func (handle *Handle) Cached() bool {
	return handle.Flags.IsSet(HandleFlagCached)
}

// GetFileObject : Get the OS.File handle stored within
func (handle *Handle) GetFileObject() *os.File {
	return handle.FObj
}

// SetFileObject : Store the OS.File handle
func (handle *Handle) SetFileObject(f *os.File) {
	handle.FObj = f
}

// SetValue : Store user defined parameter inside handle
func (handle *Handle) SetValue(key string, value interface{}) {
	handle.Lock()
	handle.values[key] = value
	handle.Unlock()
}

// GetValue : Retrieve user defined parameter from handle
func (handle *Handle) GetValue(key string) (interface{}, bool) {
	handle.RLock()
	val, ok := handle.values[key]
	handle.RUnlock()
	return val, ok
}

// RemoveValue : Delete user defined parameter from handle
func (handle *Handle) RemoveValue(key string) {
	handle.Lock()
	delete(handle.values, key)
	handle.Unlock()
}

// Cleanup : Delete all user defined parameter from handle
func (handle *Handle) Cleanup() {
	handle.Lock()
	for key := range handle.values {
		delete(handle.values, key)
	}
	handle.Unlock()
}

//defaultHandleMap holds a synchronized map[ HandleID ]*Handle
var defaultHandleMap sync.Map
var nextHandleID = *atomic.NewUint64(uint64(0))

// Add : Add the newly created handle to map and allocate a handle id
func Add(handle *Handle) HandleID {
	var ok = true
	var key HandleID
	for ok {
		key = HandleID(nextHandleID.Inc())
		_, ok = defaultHandleMap.LoadOrStore(key, handle)
	}
	handle.ID = key
	return key
}

// Delete : Remove handle object from map
func Delete(key HandleID) {
	defaultHandleMap.Delete(key)
}

// Load : Search the handle object based on its id
func Load(key HandleID) (*Handle, bool) {
	handleIF, ok := defaultHandleMap.Load(key)
	if !ok {
		return nil, false
	}
	handle := handleIF.(*Handle)
	return handle, true
}

//Store function must not be used in production application.
//This is a utility function present only for test scenarios.
func Store(key HandleID, path string, fd uintptr) *Handle {
	handle := &Handle{
		ID:   key,
		Path: path,
	}
	defaultHandleMap.Store(key, handle)
	return handle
}
