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

package attr_cache

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common/config"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal"
	"github.com/Azure/azure-storage-fuse/v2/internal/handlemap"
)

// By default attr cache is valid for 120 seconds
const defaultAttrCacheTimeout uint32 = (120)

// Common structure for AttrCache Component
type AttrCache struct {
	internal.BaseComponent
	cacheTimeout uint32
	noSymlinks   bool
	maxFiles     int
	cacheMap     map[string]*attrCacheItem
	cacheLock    sync.RWMutex
	cleanupDone  chan bool
	cleanupCtx   context.Context
	cleanupStop  context.CancelFunc
}

// Structure defining your config parameters
type AttrCacheOptions struct {
	Timeout       uint32 `config:"timeout-sec" yaml:"timeout-sec,omitempty"`
	NoCacheOnList bool   `config:"no-cache-on-list" yaml:"no-cache-on-list,omitempty"`
	NoSymlinks    bool   `config:"no-symlinks" yaml:"no-symlinks,omitempty"`

	//maximum file attributes overall to be cached
	MaxFiles int `config:"max-files" yaml:"max-files,omitempty"`

	// support v1
	CacheOnList bool `config:"cache-on-list"`
}

const compName = "attr_cache"

// caching only first 5 mil files by default
// caching more means increased memory usage of the process
const defaultMaxFiles = 5000000 // 5 million max files overall to be cached

// Verification to check satisfaction criteria with Component Interface
var _ internal.Component = &AttrCache{}

func (ac *AttrCache) Name() string {
	return compName
}

func (ac *AttrCache) SetName(name string) {
	ac.BaseComponent.SetName(name)
}

func (ac *AttrCache) SetNextComponent(nc internal.Component) {
	ac.BaseComponent.SetNextComponent(nc)
}

func (ac *AttrCache) Priority() internal.ComponentPriority {
	return internal.EComponentPriority.LevelTwo()
}

// Start : Pipeline calls this method to start the component functionality
//
//	this shall not block the call otherwise pipeline will not start
func (ac *AttrCache) Start(ctx context.Context) error {
	log.Trace("AttrCache::Start : Starting component %s", ac.Name())

	// AttrCache : start code goes here
	ac.cacheMap = make(map[string]*attrCacheItem)

	// Start background cleanup goroutine
	ac.cleanupCtx, ac.cleanupStop = context.WithCancel(ctx)
	ac.cleanupDone = make(chan bool)
	go ac.backgroundCleanup()

	return nil
}

// Stop : Stop the component functionality and kill all threads started
func (ac *AttrCache) Stop() error {
	log.Trace("AttrCache::Stop : Stopping component %s", ac.Name())

	// Stop the background cleanup goroutine
	if ac.cleanupStop != nil {
		ac.cleanupStop()
		<-ac.cleanupDone // Wait for cleanup goroutine to finish
	}

	return nil
}

// GenConfig : Generate the default config for the component
func (ac *AttrCache) GenConfig() string {
	log.Info("AttrCache::Configure : config generation started")

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("\n%s:", ac.Name()))
	sb.WriteString(fmt.Sprintf("\n  timeout-sec: %v", defaultAttrCacheTimeout))

	return sb.String()
}

// Configure : Pipeline will call this method after constructor so that you can read config and initialize yourself
//
//	Return failure if any config is not valid to exit the process
func (ac *AttrCache) Configure(_ bool) error {
	log.Trace("AttrCache::Configure : %s", ac.Name())

	// >> If you do not need any config parameters remove below code and return nil
	conf := AttrCacheOptions{}
	err := config.UnmarshalKey(ac.Name(), &conf)
	if err != nil {
		log.Err("AttrCache::Configure : config error [invalid config attributes]")
		return fmt.Errorf("config error in %s [%s]", ac.Name(), err.Error())
	}

	if config.IsSet(compName + ".timeout-sec") {
		ac.cacheTimeout = conf.Timeout
	} else {
		ac.cacheTimeout = defaultAttrCacheTimeout
	}

	if config.IsSet(compName + ".max-files") {
		ac.maxFiles = conf.MaxFiles
	} else {
		ac.maxFiles = defaultMaxFiles
	}

	if config.IsSet(compName + ".no-symlinks") {
		ac.noSymlinks = conf.NoSymlinks
	}

	log.Crit("AttrCache::Configure : cache-timeout %d, symlink %t, max-files %d",
		ac.cacheTimeout, ac.noSymlinks, ac.maxFiles)

	return nil
}

// OnConfigChange : If component has registered, on config file change this method is called
func (ac *AttrCache) OnConfigChange() {
	log.Trace("AttrCache::OnConfigChange : %s", ac.Name())
	_ = ac.Configure(true)
}

// Helper Methods
// deleteDirectory: recursively marks a directory and its children from cache
// these entries are then marked as deleted to serve ENOENT responses.
func (ac *AttrCache) deleteDirectory(path string, time time.Time) {
	// Recursively mark the children of the path as deleted, then delete the path
	// For example, filesystem: a/, a/b, a/c, aa/, ab.
	// When we delete directory a, we only want to delete a/, a/b, and a/c.
	// If we do not conditionally extend a, we would accidentally delete aa/ and ab

	// Add a trailing / so that we only delete child paths under the directory and not paths that have the same prefix
	prefix := internal.ExtendDirName(path)

	for key, value := range ac.cacheMap {
		if strings.HasPrefix(key, prefix) {
			value.markDeleted(time)
		}
	}

	// We need to delete the path itself since we only handle children above.
	ac.deletePath(path, time)
}

// deletePath: removes a path from cache
func (ac *AttrCache) deletePath(path string, time time.Time) {
	// Keys in the cache map do not contain trailing /, truncate the path before referencing a key in the map.
	value, found := ac.cacheMap[internal.TruncateDirName(path)]
	if found {
		value.markDeleted(time)
	}
}

// invalidateDirectory: recursively marks a directory invalid
func (ac *AttrCache) invalidateDirectory(path string) {
	// Recursively invalidate the children of the path, then invalidate the path
	// For example, filesystem: a/, a/b, a/c, aa/, ab.
	// When we invalidate directory a, we only want to invalidate a/, a/b, and a/c.
	// If we do not conditionally extend a, we would accidentally invalidate aa/ and ab

	// Add a trailing / so that we only invalidate child paths under the directory and not paths that have the same prefix
	prefix := internal.ExtendDirName(path)

	for key, value := range ac.cacheMap {
		if strings.HasPrefix(key, prefix) {
			value.invalidate()
		}
	}

	// We need to invalidate the path itself since we only handle children above.
	ac.invalidatePath(path)
}

// Copies the attr to the given path.
func (ac *AttrCache) updateCacheEntry(path string, attr *internal.ObjAttr) {
	cacheEntry, found := ac.cacheMap[path]
	if found {
		// Copy the attr
		cacheEntry.attr = attr
		// Update the path inside the attr
		cacheEntry.attr.Path = path
		// Update the Existence of the entry
		cacheEntry.attrFlag.Set(AttrFlagExists)
		// Refresh the cache entry
		cacheEntry.cachedAt = time.Now()
	}
}

// invalidatePath: invalidates a path
func (ac *AttrCache) invalidatePath(path string) {
	// Keys in the cache map do not contain trailing /, truncate the path before referencing a key in the map.
	value, found := ac.cacheMap[internal.TruncateDirName(path)]
	if found {
		value.invalidate()
	}
}

// backgroundCleanup: runs in a separate goroutine to periodically clean up expired entries
func (ac *AttrCache) backgroundCleanup() {
	defer close(ac.cleanupDone)

	// Ensure minimum interval to prevent panic with NewTicker.
	// Note: `cacheTimeout` is immutable post-start and should not be modified during runtime.
	interval := time.Duration(ac.cacheTimeout) * time.Second
	if interval <= 0 {
		interval = time.Second // Use 1 second as minimum interval
	}

	// Create ticker based on cache timeout interval
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ac.cleanupCtx.Done():
			log.Trace("AttrCache::backgroundCleanup : Stopping background cleanup")
			return
		case <-ticker.C:
			ac.cleanupExpiredEntries()
		}
	}
}

// cleanupExpiredEntries: removes expired entries from the cache map
// This runs in a background goroutine to prevent memory leaks
func (ac *AttrCache) cleanupExpiredEntries() {
	// First pass: collect keys to delete under read lock to minimize write lock duration
	var keysToDelete []string
	ac.cacheLock.RLock()
	for path, item := range ac.cacheMap {
		// Check if entry has exceeded the cache timeout
		if time.Since(item.cachedAt).Seconds() >= float64(ac.cacheTimeout) {
			keysToDelete = append(keysToDelete, path)
		}
	}
	ac.cacheLock.RUnlock()

	// Second pass: delete expired entries under write lock, re-checking expiration
	if len(keysToDelete) > 0 {
		ac.cacheLock.Lock()
		for _, path := range keysToDelete {
			// Re-check if entry still exists and is still expired
			if item, exists := ac.cacheMap[path]; exists {
				if time.Since(item.cachedAt).Seconds() >= float64(ac.cacheTimeout) {
					delete(ac.cacheMap, path)
				}
			}
		}
		ac.cacheLock.Unlock()
	}
}

// ------------------------- Methods implemented by this component -------------------------------------------
// CreateDir: Mark the directory invalid
func (ac *AttrCache) CreateDir(options internal.CreateDirOptions) error {
	log.Trace("AttrCache::CreateDir : %s", options.Name)
	err := ac.NextComponent().CreateDir(options)

	if err == nil || err == syscall.EEXIST {
		ac.cacheLock.RLock()
		defer ac.cacheLock.RUnlock()
		ac.invalidatePath(options.Name)
	}
	return err
}

// DeleteDir: Mark the directory deleted and recursively mark all it's children deleted
func (ac *AttrCache) DeleteDir(options internal.DeleteDirOptions) error {
	log.Trace("AttrCache::DeleteDir : %s", options.Name)

	deletionTime := time.Now()
	err := ac.NextComponent().DeleteDir(options)

	if err == nil {
		ac.cacheLock.RLock()
		defer ac.cacheLock.RUnlock()
		ac.deleteDirectory(options.Name, deletionTime)
	}

	return err
}

// ReadDir : Optionally cache attributes of paths returned by next component
func (ac *AttrCache) ReadDir(options internal.ReadDirOptions) (pathList []*internal.ObjAttr, err error) {
	log.Trace("AttrCache::ReadDir : %s", options.Name)

	pathList, err = ac.NextComponent().ReadDir(options)
	if err == nil {
		ac.cacheAttributes(pathList)
	}

	return pathList, err
}

// StreamDir : Optionally cache attributes of paths returned by next component
func (ac *AttrCache) StreamDir(options internal.StreamDirOptions) ([]*internal.ObjAttr, string, error) {
	log.Trace("AttrCache::StreamDir : %s", options.Name)

	pathList, token, err := ac.NextComponent().StreamDir(options)
	if err == nil {
		ac.cacheAttributes(pathList)
	}

	return pathList, token, err
}

// cacheAttributes : On dir listing cache the attributes for all files
func (ac *AttrCache) cacheAttributes(pathList []*internal.ObjAttr) {
	// Check whether or not we are supposed to cache on list
	if len(pathList) > 0 {
		// Putting this inside loop is heavy as for each item we will do a kernel call to get current time
		// If there are millions of blobs then cost of this is very high.
		currTime := time.Now()

		for _, attr := range pathList {
			if len(ac.cacheMap) > ac.maxFiles {
				log.Debug("AttrCache::cacheAttributes : %s skipping adding path to attribute cache because it is full", pathList)
				break
			}

			ac.cacheLock.Lock()
			ac.cacheMap[internal.TruncateDirName(attr.Path)] = newAttrCacheItem(attr, true, currTime)
			ac.cacheLock.Unlock()
		}

	}
}

// RenameDir : Mark the source directory deleted and recursively mark all it's children deleted.
// Invalidate the destination since we may have overwritten it.
func (ac *AttrCache) RenameDir(options internal.RenameDirOptions) error {
	log.Trace("AttrCache::RenameDir : %s -> %s", options.Src, options.Dst)

	deletionTime := time.Now()
	err := ac.NextComponent().RenameDir(options)

	if err == nil {
		ac.cacheLock.RLock()
		defer ac.cacheLock.RUnlock()
		ac.deleteDirectory(options.Src, deletionTime)
		// TLDR: Dst is guaranteed to be non-existent or empty.
		// Note: We do not need to invalidate children of Dst due to the logic in our FUSE connector, see comments there,
		// but it is always safer to double check than not.
		ac.invalidateDirectory(options.Dst)
	}

	return err
}

// CreateFile: Mark the file invalid
func (ac *AttrCache) CreateFile(options internal.CreateFileOptions) (*handlemap.Handle, error) {
	log.Trace("AttrCache::CreateFile : %s", options.Name)
	h, err := ac.NextComponent().CreateFile(options)

	if err == nil {
		ac.cacheLock.RLock()
		defer ac.cacheLock.RUnlock()
		ac.invalidatePath(options.Name)
	}

	return h, err
}

// DeleteFile : Mark the file deleted
func (ac *AttrCache) DeleteFile(options internal.DeleteFileOptions) error {
	log.Trace("AttrCache::DeleteFile : %s", options.Name)

	err := ac.NextComponent().DeleteFile(options)
	if err == nil {
		ac.cacheLock.RLock()
		defer ac.cacheLock.RUnlock()
		ac.deletePath(options.Name, time.Now())
	}

	return err
}

// RenameFile : Mark the source file deleted. Invalidate the destination file.
func (ac *AttrCache) RenameFile(options internal.RenameFileOptions) error {
	log.Trace("AttrCache::RenameFile : %s -> %s", options.Src, options.Dst)
	srcAttr := options.SrcAttr
	err := ac.NextComponent().RenameFile(options)
	if err == nil {
		// Copy source attribute to destination.
		// LMT of Source will be modified by next component if the copy is success.
		ac.cacheLock.RLock()
		defer ac.cacheLock.RUnlock()
		ac.updateCacheEntry(options.Dst, srcAttr)
		ac.deletePath(options.Src, time.Now())
	}

	return err
}

// WriteFile : Mark the file invalid
func (ac *AttrCache) WriteFile(options internal.WriteFileOptions) (int, error) {

	// GetAttr on cache hit will serve from cache, on cache miss will serve from next component.
	attr, err := ac.GetAttr(internal.GetAttrOptions{Name: options.Handle.Path, RetrieveMetadata: true})
	if err != nil {
		// Ignore not exists errors - this can happen if createEmptyFile is set to false
		if !(os.IsNotExist(err) || err == syscall.ENOENT) {
			return 0, err
		}
	}
	if attr != nil {
		options.Metadata = attr.Metadata
	}

	size, err := ac.NextComponent().WriteFile(options)
	if err == nil {
		ac.cacheLock.RLock()
		defer ac.cacheLock.RUnlock()
		// TODO: Could we just update the size and mod time of the file here? Or can other attributes change here?
		ac.invalidatePath(options.Handle.Path)
	}
	return size, err
}

// TruncateFile : Update the file with its truncated size
func (ac *AttrCache) TruncateFile(options internal.TruncateFileOptions) error {
	log.Trace("AttrCache::TruncateFile : %s", options.Name)

	err := ac.NextComponent().TruncateFile(options)
	if err == nil {
		ac.cacheLock.RLock()
		defer ac.cacheLock.RUnlock()

		// no need to truncate the name of the file
		value, found := ac.cacheMap[options.Name]
		if found && value.valid() && value.exists() {
			value.setSize(options.Size)
		}
		// todo: invalidating path here rather than updating with etag
		// due to some changes that are required in az storage comp which
		// were not necessarily required. Once they were done invalidation
		// of the attribute can be removed.
		ac.invalidatePath(options.Name)
	}
	return err
}

// CopyFromFile : Mark the file invalid
func (ac *AttrCache) CopyFromFile(options internal.CopyFromFileOptions) error {
	log.Trace("AttrCache::CopyFromFile : %s", options.Name)

	// GetAttr on cache hit will serve from cache, on cache miss will serve from next component.
	attr, err := ac.GetAttr(internal.GetAttrOptions{Name: options.Name, RetrieveMetadata: true})
	if err != nil {
		// Ignore not exists errors - this can happen if createEmptyFile is set to false
		if !(os.IsNotExist(err) || err == syscall.ENOENT) {
			return err
		}
	}
	if attr != nil {
		options.Metadata = attr.Metadata
	}

	err = ac.NextComponent().CopyFromFile(options)
	if err == nil {
		ac.cacheLock.RLock()
		defer ac.cacheLock.RUnlock()
		// TODO: Could we just update the size and mod time of the file here? Or can other attributes change here?
		// TODO: we're RLocking the cache but we need to also lock this attr item because another thread could be reading this attr item
		ac.invalidatePath(options.Name)
	}
	return err
}

func (ac *AttrCache) SyncFile(options internal.SyncFileOptions) error {
	log.Trace("AttrCache::SyncFile : %s", options.Handle.Path)

	err := ac.NextComponent().SyncFile(options)
	if err == nil {
		ac.cacheLock.RLock()
		defer ac.cacheLock.RUnlock()
		ac.invalidatePath(options.Handle.Path)
	}
	return err
}

func (ac *AttrCache) SyncDir(options internal.SyncDirOptions) error {
	log.Trace("AttrCache::SyncDir : %s", options.Name)

	err := ac.NextComponent().SyncDir(options)
	if err == nil {
		ac.cacheLock.RLock()
		defer ac.cacheLock.RUnlock()
		ac.invalidateDirectory(options.Name)
	}
	return err
}

// GetAttr : Try to serve the request from the attribute cache, otherwise cache attributes of the path returned by next component
func (ac *AttrCache) GetAttr(options internal.GetAttrOptions) (*internal.ObjAttr, error) {
	log.Trace("AttrCache::GetAttr : %s", options.Name)
	truncatedPath := internal.TruncateDirName(options.Name)

	ac.cacheLock.RLock()
	value, found := ac.cacheMap[truncatedPath]
	ac.cacheLock.RUnlock()

	// Try to serve the request from the attribute cache
	if found && value.valid() && time.Since(value.cachedAt).Seconds() < float64(ac.cacheTimeout) {
		if value.isDeleted() {
			log.Debug("AttrCache::GetAttr : %s served from cache", options.Name)
			// no entry if path does not exist
			return &internal.ObjAttr{}, syscall.ENOENT
		} else {
			log.Debug("AttrCache::GetAttr : %s served from cache", options.Name)
			return value.getAttr(), nil
		}
	}

	// Get the attributes from next component and cache them
	pathAttr, err := ac.NextComponent().GetAttr(options)

	ac.cacheLock.Lock()
	defer ac.cacheLock.Unlock()

	if err == nil {
		// Retrieved attributes so cache them
		if len(ac.cacheMap) < ac.maxFiles {
			ac.cacheMap[truncatedPath] = newAttrCacheItem(pathAttr, true, time.Now())
		} else {
			log.Debug("AttrCache::GetAttr : %s skipping adding to attribute cache because it is full", options.Name)
		}
	} else if err == syscall.ENOENT {
		// Path does not exist so cache a no-entry item
		ac.cacheMap[truncatedPath] = newAttrCacheItem(&internal.ObjAttr{}, false, time.Now())
	}

	return pathAttr, err
}

// CreateLink : Mark the link and target invalid
func (ac *AttrCache) CreateLink(options internal.CreateLinkOptions) error {
	log.Trace("AttrCache::CreateLink : Create symlink %s -> %s", options.Name, options.Target)

	err := ac.NextComponent().CreateLink(options)

	if err == nil {
		ac.cacheLock.RLock()
		defer ac.cacheLock.RUnlock()
		ac.invalidatePath(options.Name)
		ac.invalidatePath(options.Target) // TODO : Why do we invalidate the target? Shouldn't the target remain unchanged?
	}

	return err
}

// FlushFile : flush file
func (ac *AttrCache) FlushFile(options internal.FlushFileOptions) error {
	log.Trace("AttrCache::FlushFile : %s", options.Handle.Path)
	err := ac.NextComponent().FlushFile(options)
	if err == nil {
		ac.cacheLock.RLock()
		defer ac.cacheLock.RUnlock()

		ac.invalidatePath(options.Handle.Path)
	}
	return err
}

// Chmod : Update the file with its new permissions
func (ac *AttrCache) Chmod(options internal.ChmodOptions) error {
	log.Trace("AttrCache::Chmod : Change mode of file/directory %s", options.Name)

	err := ac.NextComponent().Chmod(options)

	if err == nil {
		ac.cacheLock.RLock()
		defer ac.cacheLock.RUnlock()

		value, found := ac.cacheMap[internal.TruncateDirName(options.Name)]
		if found && value.valid() && value.exists() {
			value.setMode(options.Mode)
		}
	}

	return err
}

// Chown : Update the file with its new owner and group (when datalake chown is implemented)
func (ac *AttrCache) Chown(options internal.ChownOptions) error {
	err := ac.NextComponent().Chown(options)
	if err == nil {
		ac.cacheLock.RLock()
		defer ac.cacheLock.RUnlock()

		value, found := ac.cacheMap[options.Name]
		if found && value.valid() && value.exists() {
			value.setOwnerGroup(options.Owner, options.Group)
		}
	}
	return err
}

func (ac *AttrCache) CommitData(options internal.CommitDataOptions) error {
	log.Trace("AttrCache::CommitData : %s", options.Name)
	err := ac.NextComponent().CommitData(options)
	if err == nil {
		ac.cacheLock.RLock()
		defer ac.cacheLock.RUnlock()
		// TODO: Could we just update the size, etag, modtime of the file here?
		ac.invalidatePath(options.Name)
	}
	return err
}

// ------------------------- Factory -------------------------------------------

// Pipeline will call this method to create your object, initialize your variables here
// << DO NOT DELETE ANY AUTO GENERATED CODE HERE >>
func NewAttrCacheComponent() internal.Component {
	comp := &AttrCache{}
	comp.SetName(compName)

	config.AddConfigChangeEventListener(comp)
	return comp
}

// On init register this component to pipeline and supply your constructor
func init() {
	internal.AddComponent(compName, NewAttrCacheComponent)

	attrCacheTimeout := config.AddUint32Flag("attr-cache-timeout", defaultAttrCacheTimeout, "attribute cache timeout")
	config.BindPFlag(compName+".timeout-sec", attrCacheTimeout)

	noSymlinks := config.AddBoolFlag("no-symlinks", false, "whether or not symlinks should be supported")
	config.BindPFlag(compName+".no-symlinks", noSymlinks)

	cacheOnList := config.AddBoolFlag("cache-on-list", true, "Cache attributes on listing.")
	config.BindPFlag(compName+".cache-on-list", cacheOnList)
	cacheOnList.Hidden = true
}
