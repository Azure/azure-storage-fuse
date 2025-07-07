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

package handlemap

import (
	"container/list"
	"os"
	"sync"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/cache_policy"

	"go.uber.org/atomic"
)

//go:generate $ASSERT_REMOVER $GOFILE

type HandleID uint64

const InvalidHandleID HandleID = 0

// Flags represented in BitMap for various flags in the handle
const (
	HandleFlagUnknown uint16 = iota
	HandleFlagDirty          // File has been modified with write operation or is a new file
	HandleFlagFSynced        // User has called fsync on the file explicitly
	HandleFlagCached         // File is cached in the local system by blobfuse2
	HandleFlagFSDebug
	// Handle with HandleFSDebug is analogous to opening the proc files in linux, represents the file that was opened is
	// for some dubugging/metrics for the user and this handle can only be read. only allow if the open flags are O_RDONLY.
	// The flag is only valid in the context of open callback. If the component implements any proc files for the fs then
	// those files must return file size to be zero in getattr/readdir, as the files are generated on fly the size maynot
	// be known at the stat/open, but "read" must return EOF error when the size of the file has reached.

	// Following are the Dcache Flags
	HandleFSAzure  // Handle refers to a file in Azure
	HandleFSDcache // Handle refers to a file in Distributed Cache.
	// Both HandleFSAzure and HandleFSDcache will be set for handles corresponding
	// to file paths without an explicit fs=azure/fs=dcache namespace specified.
	HandleFlagDcacheAllowWrites // Write to Distributed Cache through this handle is only allowed if this flag is set
	HandleFlagDcacheAllowReads  // Read from Distributed Cache through this handle is only allowed if this flag is set
)

// Structure to hold in memory cache for streaming layer
type Cache struct {
	sync.RWMutex
	*cache_policy.LRUCache
	*common.BlockOffsetList
	StreamOnly  bool
	HandleCount int64
}

type Buffers struct {
	Cooked  *list.List
	Cooking *list.List
}

type Handle struct {
	sync.RWMutex
	FObj     *os.File // File object being represented by this handle
	CacheObj *Cache   // Streaming layer cache for this handle
	Buffers  *Buffers
	ID       HandleID // Blobfuse assigned unique ID to this handle
	Size     int64    // Size of the file being handled here
	Mtime    time.Time
	UnixFD   uint64                 // Unix FD created by create/open syscall
	OptCnt   uint64                 // Number of operations done on this file
	Flags    common.BitMap16        // Various states of the file
	Path     string                 // Always holds path relative to mount dir
	values   map[string]interface{} // Map to hold other info if application wants to store
	IFObj    any                    // Generic file Object that can be used by other components.
}

func NewHandle(path string) *Handle {
	return &Handle{
		ID:       InvalidHandleID,
		Path:     path,
		Size:     0,
		Flags:    0,
		OptCnt:   0,
		values:   make(map[string]interface{}),
		CacheObj: nil,
		FObj:     nil,
		Buffers:  nil,
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

// FD : Get Unix file descriptor
func (handle *Handle) FD() int {
	return int(handle.UnixFD)
}

// SetValue : Store user defined parameter inside handle
func (handle *Handle) SetValue(key string, value interface{}) {
	handle.values[key] = value
}

// GetValue : Retrieve user defined parameter from handle
func (handle *Handle) GetValue(key string) (interface{}, bool) {
	val, ok := handle.values[key]
	return val, ok
}

// GetValue : Retrieve user defined parameter from handle
func (handle *Handle) RemoveValue(key string) (interface{}, bool) {
	val, ok := handle.values[key]
	delete(handle.values, key)
	return val, ok
}

// Cleanup : Delete all user defined parameter from handle
func (handle *Handle) Cleanup() {
	for key := range handle.values {
		delete(handle.values, key)
	}
}

// Set's the Flag that handle is pointing to a debug/proc File.
// Refer the flag to see more info.
func (handle *Handle) SetFsDebug() {
	handle.Flags.Set(HandleFlagFSDebug)
}

func (handle *Handle) IsFsDebug() bool {
	return handle.Flags.IsSet(HandleFlagFSDebug)
}

// **********************Dcache Related Methods Start******************************
func (handle *Handle) SetFsAzure() {
	// Must be set once and only once.
	common.Assert(!handle.IsFsAzure())
	common.Assert(!handle.IsFsDcache())
	handle.Flags.Set(HandleFSAzure)
}

func (handle *Handle) SetFsDcache() {
	// Must be set once and only once.
	common.Assert(!handle.IsFsAzure())
	common.Assert(!handle.IsFsDcache())
	handle.Flags.Set(HandleFSDcache)
}

func (handle *Handle) SetFsDefault() {
	// Must be set once and only once.
	common.Assert(!handle.IsFsAzure())
	common.Assert(!handle.IsFsDcache())
	handle.Flags.Set(HandleFSAzure)
	handle.Flags.Set(HandleFSDcache)
}

func (handle *Handle) IsFsAzure() bool {
	return handle.Flags.IsSet(HandleFSAzure)
}

func (handle *Handle) IsFsDcache() bool {
	return handle.Flags.IsSet(HandleFSDcache)
}

func (handle *Handle) SetDcacheAllowWrites() {
	// Must be set only once.
	common.Assert(!handle.IsDcacheAllowWrites())
	// Read and write to dcache are not allowed from the same handle.
	common.Assert(!handle.IsDcacheAllowReads())
	// Using a handle we can write to Azure, DCache or both.
	common.Assert(handle.IsFsAzure() || handle.IsFsDcache())

	handle.Flags.Set(HandleFlagDcacheAllowWrites)
}

func (handle *Handle) SetDcacheAllowReads() {
	// Must be set only once.
	common.Assert(!handle.IsDcacheAllowReads())
	// Read and write to dcache are not allowed from the same handle.
	common.Assert(!handle.IsDcacheAllowWrites())
	// Using a handle we can read from Azure or DCache but never both.
	common.Assert(handle.IsFsAzure() || handle.IsFsDcache())
	common.Assert(!(handle.IsFsAzure() && handle.IsFsDcache()))

	handle.Flags.Set(HandleFlagDcacheAllowReads)
}

func (handle *Handle) IsDcacheAllowWrites() bool {
	allowWrites := handle.Flags.IsSet(HandleFlagDcacheAllowWrites)
	allowReads := handle.Flags.IsSet(HandleFlagDcacheAllowReads)
	_ = allowReads
	// Read and write to dcache are not allowed from the same handle.
	common.Assert(!(allowWrites && allowReads))
	return allowWrites
}

func (handle *Handle) IsDcacheAllowReads() bool {
	allowReads := handle.Flags.IsSet(HandleFlagDcacheAllowReads)
	allowWrites := handle.Flags.IsSet(HandleFlagDcacheAllowWrites)
	_ = allowWrites
	// Read and write to dcache are not allowed from the same handle.
	common.Assert(!(allowWrites && allowReads))
	return allowReads
}

func (handle *Handle) SetDcacheStopWrites() {
	// Close has come to the handle and success, no more writes to this handle
	// from now.
	handle.Flags.Clear(HandleFlagDcacheAllowWrites)
	// From this point there cannot be any IO on this handle.
	common.Assert(!handle.IsDcacheAllowReads() && !handle.IsDcacheAllowWrites())
}

// **********************Dcache Related Methods End******************************

// defaultHandleMap holds a synchronized map[ HandleID ]*Handle
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

func CreateCacheObject(capacity int64, handle *Handle) {
	handle.CacheObj = &Cache{
		sync.RWMutex{},
		cache_policy.NewLRUCache(capacity),
		&common.BlockOffsetList{},
		false,
		0,
	}
}

// GetHandles : Get map of handles stored
func GetHandles() *sync.Map {
	return &defaultHandleMap
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

// Store function must not be used in production application.
// This is a utility function present only for test scenarios.
func Store(key HandleID, path string, _ uintptr) *Handle {
	handle := &Handle{
		ID:   key,
		Path: path,
	}
	defaultHandleMap.Store(key, handle)
	return handle
}
