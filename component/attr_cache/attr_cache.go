/*
    _____           _____   _____   ____          ______  _____  ------
   |     |  |      |     | |     | |     |     | |       |            |
   |     |  |      |     | |     | |     |     | |       |            |
   | --- |  |      |     | |-----| |---- |     | |-----| |-----  ------
   |     |  |      |     | |     | |     |     |       | |       |
   | ____|  |_____ | ____| | ____| |     |_____|  _____| |_____  |_____


   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2020-2026 Microsoft Corporation. All rights reserved.
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
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common/config"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal"
	"github.com/Azure/azure-storage-fuse/v2/internal/handlemap"
)

// defaultAttrCacheTimeout is the default TTL for cached attributes (seconds).
const defaultAttrCacheTimeout uint32 = 120

// defaultMaxSizeMB is the default memory limit for the attribute cache.
// Set to 0 to disable memory-based eviction (only TTL-based cleanup applies).
//
// Rough capacity at 64 MB (52-byte path, 20-byte ETag, 16-byte MD5):
//   - negative entry (tombstone): ~320 B/entry → ~210 K entries
//   - positive entry (no metadata): ~964 B/entry → ~70 K entries
//
// See TestNegativeEntryCapacityMatchesEstimate and TestPositiveEntryCapacityMatchesEstimate
// in cache_item_test.go for exact accounting and how to recalculate for different path lengths.
const defaultMaxSizeMB uint32 = 64

// AttrCache is the pipeline component that caches file/directory attributes.
// The LRU is thread-safe; no additional locking is needed around individual operations.
// Expired entries are rejected lazily at GetAttr time; the memory-bounded LRU evicts
// stale entries by LRU order as new entries arrive.
// A background sweeper goroutine reclaims memory from TTL-expired entries when the cache is idle.
type AttrCache struct {
	internal.BaseComponent
	cacheTimeout  time.Duration
	noSymlinks    bool
	maxSizeBytes  int64
	lru           *attrCacheLRU
	stopCh        chan struct{}
	sweepWg       sync.WaitGroup
	lastOp        atomic.Int64 // Unix seconds of last LRU operation; 0 = no activity yet
	noCacheOnList bool
}

// AttrCacheOptions holds the configuration for the attribute cache.
type AttrCacheOptions struct {
	Timeout       uint32 `config:"timeout-sec"      yaml:"timeout-sec,omitempty"`
	NoCacheOnList bool   `config:"no-cache-on-list" yaml:"no-cache-on-list,omitempty"`
	NoSymlinks    bool   `config:"no-symlinks"      yaml:"no-symlinks,omitempty"`
	MaxSizeMB     uint32 `config:"max-size-mb"      yaml:"max-size-mb,omitempty"`

	// support v1
	CacheOnList bool `config:"cache-on-list"`
}

const compName = "attr_cache"

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

// Start initialises the cache and launches the background TTL sweeper.
func (ac *AttrCache) Start(_ context.Context) error {
	log.Trace("AttrCache::Start : Starting component %s", ac.Name())
	ac.lru = newAttrCacheLRU(ac.maxSizeBytes, &ac.lastOp)
	if ac.cacheTimeout > 0 {
		ac.stopCh = make(chan struct{})
		ac.sweepWg.Add(1)
		go func() {
			defer ac.sweepWg.Done()
			ac.ttlSweeper()
		}()
	}
	return nil
}

// Stop cancels the background sweeper and waits for it to exit before returning.
// Memory held by expired entries is reclaimed by the GC once the component is released.
func (ac *AttrCache) Stop() error {
	log.Trace("AttrCache::Stop : Stopping component %s", ac.Name())
	if ac.stopCh != nil {
		close(ac.stopCh)
		ac.sweepWg.Wait()
		ac.stopCh = nil
	}
	return nil
}

// ttlSweeper ticks every cacheTimeout and evicts expired entries when the cache is idle.
func (ac *AttrCache) ttlSweeper() {
	ticker := time.NewTicker(ac.cacheTimeout)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			ac.sweepExpired()
		case <-ac.stopCh:
			return
		}
	}
}

// sweepExpired removes all TTL-expired entries from the LRU.
// The idle gate skips the sweep if the cache was used within the last cacheTimeout/2
// to avoid holding the write lock during active traffic.
func (ac *AttrCache) sweepExpired() {
	if ac.cacheTimeout <= 0 {
		return
	}

	// Only apply the idle gate when the cache is memory-bounded. If MaxSize()==0 (unlimited),
	// skipping sweeps under continuous traffic can let TTL-expired entries accumulate forever.
	idleThreshold := ac.cacheTimeout / 2
	if idleThreshold > 0 && ac.lru.MaxSize() > 0 {
		if last := ac.lastOp.Load(); last != 0 {
			if time.Since(time.Unix(last, 0)) < idleThreshold {
				return
			}
		}
	}
	timeout := ac.cacheTimeout
	before := ac.lru.Size()
	ac.lru.DeleteIf(func(_ string, item *attrCacheItem) bool {
		return time.Since(item.cachedAt) >= timeout
	})
	maxMB := ac.lru.MaxSize() >> 20
	if maxMB == 0 {
		log.Debug("AttrCache::sweepExpired : size %d MB (unbounded) (%d entries), reclaimed %d MB",
			ac.lru.Size()>>20, ac.lru.Len(), (before-ac.lru.Size())>>20)
		return
	}
	log.Debug("AttrCache::sweepExpired : size %d MB / %d MB (%d entries), reclaimed %d MB",
		ac.lru.Size()>>20, maxMB, ac.lru.Len(), (before-ac.lru.Size())>>20)
}

// GenConfig returns a default configuration snippet for this component.
func (ac *AttrCache) GenConfig() string {
	log.Info("AttrCache::Configure : config generation started")

	var sb strings.Builder
	fmt.Fprintf(&sb, "\n%s:", ac.Name())
	fmt.Fprintf(&sb, "\n  timeout-sec: %v", defaultAttrCacheTimeout)
	fmt.Fprintf(&sb, "\n  max-size-mb: %v", defaultMaxSizeMB)

	return sb.String()
}

// Configure reads component configuration and applies it.
func (ac *AttrCache) Configure(_ bool) error {
	log.Trace("AttrCache::Configure : %s", ac.Name())

	conf := AttrCacheOptions{}
	err := config.UnmarshalKey(ac.Name(), &conf)
	if err != nil {
		log.Err("AttrCache::Configure : config error [invalid config attributes]")
		return fmt.Errorf("config error in %s [%s]", ac.Name(), err.Error())
	}

	if config.IsSet(compName + ".timeout-sec") {
		ac.cacheTimeout = time.Duration(conf.Timeout) * time.Second
	} else {
		ac.cacheTimeout = time.Duration(defaultAttrCacheTimeout) * time.Second
	}

	if config.IsSet(compName + ".no-symlinks") {
		ac.noSymlinks = conf.NoSymlinks
	}

	// no-cache-on-list (v2) takes priority; fall back to inverted cache-on-list (v1).
	if config.IsSet(compName + ".no-cache-on-list") {
		ac.noCacheOnList = conf.NoCacheOnList
	} else if config.IsSet(compName + ".cache-on-list") {
		ac.noCacheOnList = !conf.CacheOnList
	}

	if config.IsSet(compName + ".max-size-mb") {
		ac.maxSizeBytes = int64(conf.MaxSizeMB) * 1024 * 1024
	} else {
		ac.maxSizeBytes = int64(defaultMaxSizeMB) * 1024 * 1024
	}

	effectiveMB := uint32(ac.maxSizeBytes / (1024 * 1024))
	log.Crit("AttrCache::Configure : cache-timeout %v, no-symlinks %t, no-cache-on-list %t, max-size-mb %d",
		ac.cacheTimeout, ac.noSymlinks, ac.noCacheOnList, effectiveMB)

	return nil
}

// OnConfigChange logs that attr_cache settings cannot be applied safely at runtime.
func (ac *AttrCache) OnConfigChange() {
	log.Warn("AttrCache::OnConfigChange : config change detected but not applied; restart required to apply new attr_cache settings")
}

// ------------------------- Component operations -------------------------------------------

// CreateDir marks the directory invalid in the cache.
func (ac *AttrCache) CreateDir(options internal.CreateDirOptions) error {
	log.Trace("AttrCache::CreateDir : %s", options.Name)
	err := ac.NextComponent().CreateDir(options)

	if err == nil || err == syscall.EEXIST {
		ac.lru.invalidatePath(options.Name)
	}
	return err
}

// DeleteDir marks the directory and all its children as deleted.
func (ac *AttrCache) DeleteDir(options internal.DeleteDirOptions) error {
	log.Trace("AttrCache::DeleteDir : %s", options.Name)

	deletionTime := time.Now()
	err := ac.NextComponent().DeleteDir(options)

	if err == nil {
		ac.lru.deleteDirectory(options.Name, deletionTime)
	}

	return err
}

// ReadDir caches attributes of all paths returned by the next component.
func (ac *AttrCache) ReadDir(options internal.ReadDirOptions) (pathList []*internal.ObjAttr, err error) {
	log.Trace("AttrCache::ReadDir : %s", options.Name)

	pathList, err = ac.NextComponent().ReadDir(options)
	if err == nil && !ac.noCacheOnList {
		ac.lru.cacheAttributes(pathList)
	}

	return pathList, err
}

// StreamDir caches attributes of all paths returned by the next component.
func (ac *AttrCache) StreamDir(options internal.StreamDirOptions) ([]*internal.ObjAttr, string, error) {
	log.Trace("AttrCache::StreamDir : %s", options.Name)

	pathList, token, err := ac.NextComponent().StreamDir(options)
	if err == nil && !ac.noCacheOnList {
		ac.lru.cacheAttributes(pathList)
	}

	return pathList, token, err
}

// RenameDir marks the source directory deleted and invalidates the destination.
func (ac *AttrCache) RenameDir(options internal.RenameDirOptions) error {
	log.Trace("AttrCache::RenameDir : %s -> %s", options.Src, options.Dst)

	deletionTime := time.Now()
	err := ac.NextComponent().RenameDir(options)

	if err == nil {
		ac.lru.deleteDirectory(options.Src, deletionTime)
		// TLDR: Dst is guaranteed to be non-existent or empty.
		// Note: We do not need to invalidate children of Dst due to the logic in our FUSE
		// connector, but it is always safer to double-check.
		ac.lru.invalidateDirectory(options.Dst)
	}

	return err
}

// CreateFile marks the file invalid in the cache.
func (ac *AttrCache) CreateFile(options internal.CreateFileOptions) (*handlemap.Handle, error) {
	log.Trace("AttrCache::CreateFile : %s", options.Name)
	h, err := ac.NextComponent().CreateFile(options)

	if err == nil {
		ac.lru.invalidatePath(options.Name)
	}

	return h, err
}

// DeleteFile marks the file as deleted in the cache.
func (ac *AttrCache) DeleteFile(options internal.DeleteFileOptions) error {
	log.Trace("AttrCache::DeleteFile : %s", options.Name)

	err := ac.NextComponent().DeleteFile(options)
	if err == nil {
		ac.lru.deletePath(options.Name, time.Now())
	}

	return err
}

// RenameFile copies source attributes to destination and marks the source deleted.
func (ac *AttrCache) RenameFile(options internal.RenameFileOptions) error {
	log.Trace("AttrCache::RenameFile : %s -> %s", options.Src, options.Dst)
	srcAttr := options.SrcAttr
	err := ac.NextComponent().RenameFile(options)
	if err == nil {
		ac.lru.updateCacheEntry(options.Dst, srcAttr)
		ac.lru.deletePath(options.Src, time.Now())
	}

	return err
}

// WriteFile retrieves metadata from the cache, forwards the write, then invalidates.
func (ac *AttrCache) WriteFile(options *internal.WriteFileOptions) (int, error) {
	// GetAttr on cache hit will serve from cache, on cache miss will serve from next component.
	attr, err := ac.GetAttr(internal.GetAttrOptions{Name: options.Handle.Path, RetrieveMetadata: true})
	if err != nil {
		// Ignore not-exists errors — this can happen if createEmptyFile is set to false.
		if !errors.Is(err, os.ErrNotExist) {
			return 0, err
		}
	}
	if attr != nil {
		options.Metadata = attr.Metadata
	}

	size, err := ac.NextComponent().WriteFile(options)
	if err == nil {
		ac.lru.invalidatePath(options.Handle.Path)
	}
	return size, err
}

// TruncateFile invalidates the cached entry so the next GetAttr fetches updated ETag/timestamps.
func (ac *AttrCache) TruncateFile(options internal.TruncateFileOptions) error {
	log.Trace("AttrCache::TruncateFile : %s", options.Name)

	err := ac.NextComponent().TruncateFile(options)
	if err == nil {
		ac.lru.invalidatePath(options.Name)
	}
	return err
}

// CopyFromFile retrieves metadata from the cache, forwards the copy, then invalidates.
func (ac *AttrCache) CopyFromFile(options internal.CopyFromFileOptions) error {
	log.Trace("AttrCache::CopyFromFile : %s", options.Name)

	attr, err := ac.GetAttr(internal.GetAttrOptions{Name: options.Name, RetrieveMetadata: true})
	if err != nil {
		// Ignore not exists errors - this can happen if createEmptyFile is set to false
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	if attr != nil {
		options.Metadata = attr.Metadata
	}

	err = ac.NextComponent().CopyFromFile(options)
	if err == nil {
		ac.lru.invalidatePath(options.Name)
	}
	return err
}

// SyncFile invalidates the cached entry after a sync.
func (ac *AttrCache) SyncFile(options internal.SyncFileOptions) error {
	log.Trace("AttrCache::SyncFile : %s", options.Handle.Path)

	err := ac.NextComponent().SyncFile(options)
	if err == nil {
		ac.lru.invalidatePath(options.Handle.Path)
	}
	return err
}

// SyncDir recursively invalidates cached entries for a directory.
func (ac *AttrCache) SyncDir(options internal.SyncDirOptions) error {
	log.Trace("AttrCache::SyncDir : %s", options.Name)

	err := ac.NextComponent().SyncDir(options)
	if err == nil {
		ac.lru.invalidateDirectory(options.Name)
	}
	return err
}

// GetAttr serves from cache on hit (promoting the entry to MRU), or fetches from the
// next component and caches the result on miss.
func (ac *AttrCache) GetAttr(options internal.GetAttrOptions) (*internal.ObjAttr, error) {
	log.Trace("AttrCache::GetAttr : %s", options.Name)
	truncatedPath := internal.TruncateDirName(options.Name)

	if item, ok := ac.lru.Get(truncatedPath); ok {
		if time.Since(item.cachedAt) < ac.cacheTimeout {
			log.Debug("AttrCache::GetAttr : %s served from cache", options.Name)
			if item.isNegativeEntry() {
				return &internal.ObjAttr{}, syscall.ENOENT
			}

			if item.attr != nil {
				return item.attr, nil
			}

			log.Crit("AttrCache::GetAttr : %s is marked as positive entry in cache but attr is nil", truncatedPath)
		}
	}

	// Cache miss: fetch from next component.
	pathAttr, err := ac.NextComponent().GetAttr(options)

	if err == nil {
		ac.lru.cachePositiveEntry(truncatedPath, pathAttr)
	} else if errors.Is(err, os.ErrNotExist) {
		// Cache negative entries so repeated lookups for absent paths are cheap.
		// errors.Is matches syscall.ENOENT, *fs.PathError wrapping ENOENT, and bare os.ErrNotExist sentinels.
		ac.lru.cacheNegativeEntry(truncatedPath)
	}

	return pathAttr, err
}

// CreateLink marks the link and its target invalid in the cache.
func (ac *AttrCache) CreateLink(options internal.CreateLinkOptions) error {
	log.Trace("AttrCache::CreateLink : Create symlink %s -> %s", options.Name, options.Target)

	err := ac.NextComponent().CreateLink(options)

	if err == nil {
		ac.lru.invalidatePath(options.Name)
		ac.lru.invalidatePath(options.Target)
	}

	return err
}

// FlushFile invalidates the cached entry after a flush.
func (ac *AttrCache) FlushFile(options internal.FlushFileOptions) error {
	log.Trace("AttrCache::FlushFile : %s", options.Handle.Path)
	err := ac.NextComponent().FlushFile(options)
	if err == nil {
		ac.lru.invalidatePath(options.Handle.Path)
	}
	return err
}

// Chmod updates the cached mode for a file or directory.
// It puts a new immutable item so concurrent readers see a consistent snapshot.
func (ac *AttrCache) Chmod(options internal.ChmodOptions) error {
	log.Trace("AttrCache::Chmod : Change mode of file/directory %s", options.Name)

	err := ac.NextComponent().Chmod(options)

	if err == nil {
		truncated := internal.TruncateDirName(options.Name)
		if item, ok := ac.lru.Peek(truncated); ok && item.exists && item.attr != nil {
			newAttr := *item.attr // copy the struct so the old item stays immutable
			newAttr.Mode = options.Mode
			newAttr.Ctime = time.Now()
			if !ac.lru.Put(truncated, &attrCacheItem{attr: &newAttr, exists: true, cachedAt: time.Now()}) {
				log.Err("AttrCache::Chmod : entry too large for cache, skipping path %s", truncated)
			}
		}
	}

	return err
}

// Chown updates the file owner (when datalake chown is implemented).
func (ac *AttrCache) Chown(options internal.ChownOptions) error {
	log.Trace("AttrCache::Chown : Change owner of file/directory %s", options.Name)

	err := ac.NextComponent().Chown(options)
	// TODO: Implement when datalake chown is supported.

	return err
}

// CommitData invalidates the cached entry after a data commit.
func (ac *AttrCache) CommitData(options internal.CommitDataOptions) error {
	log.Trace("AttrCache::CommitData : %s", options.Name)
	err := ac.NextComponent().CommitData(options)
	if err == nil {
		ac.lru.invalidatePath(options.Name)
	}
	return err
}

// ------------------------- Factory -------------------------------------------

// NewAttrCacheComponent creates a new AttrCache component.
func NewAttrCacheComponent() internal.Component {
	comp := &AttrCache{}
	comp.SetName(compName)

	config.AddConfigChangeEventListener(comp)
	return comp
}

func init() {
	internal.AddComponent(compName, NewAttrCacheComponent)

	attrCacheTimeout := config.AddUint32Flag("attr-cache-timeout", defaultAttrCacheTimeout, "attribute cache timeout")
	config.BindPFlag(compName+".timeout-sec", attrCacheTimeout)

	noSymlinks := config.AddBoolFlag("no-symlinks", false, "whether or not symlinks should be supported")
	config.BindPFlag(compName+".no-symlinks", noSymlinks)

	cacheOnList := config.AddBoolFlag("cache-on-list", true, "Cache attributes on listing.")
	config.BindPFlag(compName+".cache-on-list", cacheOnList)
	cacheOnList.Hidden = true

	attrCacheMaxSizeMB := config.AddUint32Flag("attr-cache-max-size-mb", defaultMaxSizeMB,
		"maximum memory in MB that attr-cache can use (0 = no limit)")
	config.BindPFlag(compName+".max-size-mb", attrCacheMaxSizeMB)
}
