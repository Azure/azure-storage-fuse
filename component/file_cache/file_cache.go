/*
    _____           _____   _____   ____          ______  _____  ------
   |     |  |      |     | |     | |     |     | |       |            |
   |     |  |      |     | |     | |     |     | |       |            |
   | --- |  |      |     | |-----| |---- |     | |-----| |-----  ------
   |     |  |      |     | |     | |     |     |       | |       |
   | ____|  |_____ | ____| | ____| |     |_____|  _____| |_____  |_____


   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2020-2024 Microsoft Corporation. All rights reserved.
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
	"context"
	"fmt"
	"io"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/config"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal"
	"github.com/Azure/azure-storage-fuse/v2/internal/handlemap"
	"github.com/Azure/azure-storage-fuse/v2/internal/stats_manager"

	"github.com/spf13/cobra"
)

// Common structure for Component
type FileCache struct {
	internal.BaseComponent

	tmpPath   string
	fileLocks *common.LockMap
	policy    cachePolicy

	createEmptyFile bool
	allowNonEmpty   bool
	cacheTimeout    float64
	cleanupOnStart  bool
	policyTrace     bool
	missedChmodList sync.Map
	mountPath       string
	allowOther      bool
	offloadIO       bool
	syncToFlush     bool
	syncToDelete    bool
	maxCacheSize    float64

	defaultPermission os.FileMode

	refreshSec        uint32
	hardLimit         bool
	diskHighWaterMark float64

	lazyWrite    bool
	fileCloseOpt sync.WaitGroup
}

// Structure defining your config parameters
type FileCacheOptions struct {
	// e.g. var1 uint32 `config:"var1"`
	TmpPath string `config:"path" yaml:"path,omitempty"`
	Policy  string `config:"policy" yaml:"policy,omitempty"`

	Timeout     uint32 `config:"timeout-sec" yaml:"timeout-sec,omitempty"`
	MaxEviction uint32 `config:"max-eviction" yaml:"max-eviction,omitempty"`

	MaxSizeMB     float64 `config:"max-size-mb" yaml:"max-size-mb,omitempty"`
	HighThreshold uint32  `config:"high-threshold" yaml:"high-threshold,omitempty"`
	LowThreshold  uint32  `config:"low-threshold" yaml:"low-threshold,omitempty"`

	CreateEmptyFile bool `config:"create-empty-file" yaml:"create-empty-file,omitempty"`
	AllowNonEmpty   bool `config:"allow-non-empty-temp" yaml:"allow-non-empty-temp,omitempty"`
	CleanupOnStart  bool `config:"cleanup-on-start" yaml:"cleanup-on-start,omitempty"`

	EnablePolicyTrace bool `config:"policy-trace" yaml:"policy-trace,omitempty"`
	OffloadIO         bool `config:"offload-io" yaml:"offload-io,omitempty"`

	// v1 support
	V1Timeout     uint32 `config:"file-cache-timeout-in-seconds" yaml:"-"`
	EmptyDirCheck bool   `config:"empty-dir-check" yaml:"-"`
	SyncToFlush   bool   `config:"sync-to-flush" yaml:"sync-to-flush,omitempty"`
	SyncNoOp      bool   `config:"ignore-sync" yaml:"ignore-sync,omitempty"`

	RefreshSec uint32 `config:"refresh-sec" yaml:"refresh-sec,omitempty"`
	HardLimit  bool   `config:"hard-limit" yaml:"hard-limit,omitempty"`
}

const (
	compName                = "file_cache"
	defaultMaxEviction      = 5000
	defaultMaxThreshold     = 80
	defaultMinThreshold     = 60
	defaultFileCacheTimeout = 120
	defaultCacheUpdateCount = 100
	MB                      = 1024 * 1024
)

// Verification to check satisfaction criteria with Component Interface
var _ internal.Component = &FileCache{}

var fileCacheStatsCollector *stats_manager.StatsCollector

func (c *FileCache) Name() string {
	return compName
}

func (c *FileCache) SetName(name string) {
	c.BaseComponent.SetName(name)
}

func (c *FileCache) SetNextComponent(nc internal.Component) {
	c.BaseComponent.SetNextComponent(nc)
}

func (c *FileCache) Priority() internal.ComponentPriority {
	return internal.EComponentPriority.LevelMid()
}

// Start : Pipeline calls this method to start the component functionality
//
//	this shall not block the call otherwise pipeline will not start
func (c *FileCache) Start(ctx context.Context) error {
	log.Trace("Starting component : %s", c.Name())

	if c.cleanupOnStart {
		err := common.TempCacheCleanup(c.tmpPath)
		if err != nil {
			return fmt.Errorf("error in %s error [fail to cleanup temp cache]", c.Name())
		}
	}

	if c.policy == nil {
		return fmt.Errorf("config error in %s error [cache policy missing]", c.Name())
	}

	err := c.policy.StartPolicy()
	if err != nil {
		return fmt.Errorf("config error in %s error [fail to start policy]", c.Name())
	}

	// create stats collector for file cache
	fileCacheStatsCollector = stats_manager.NewStatsCollector(c.Name())

	return nil
}

// Stop : Stop the component functionality and kill all threads started
func (c *FileCache) Stop() error {
	log.Trace("Stopping component : %s", c.Name())

	// Wait for all async upload to complete if any
	if c.lazyWrite {
		log.Info("FileCache::Stop : Waiting for async close to complete")
		c.fileCloseOpt.Wait()
	}

	_ = c.policy.ShutdownPolicy()
	_ = common.TempCacheCleanup(c.tmpPath)

	fileCacheStatsCollector.Destroy()

	return nil
}

// GenConfig : Generate default config for the component
func (c *FileCache) GenConfig() string {
	log.Info("FileCache::Configure : config generation started")

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("\n%s:", c.Name()))

	tmpPath := ""
	_ = config.UnmarshalKey("tmp-path", &tmpPath)

	directIO := false
	_ = config.UnmarshalKey("direct-io", &directIO)

	timeout := defaultFileCacheTimeout
	if directIO {
		timeout = 0
	}

	sb.WriteString(fmt.Sprintf("\n  path: %v", common.ExpandPath(tmpPath)))
	sb.WriteString(fmt.Sprintf("\n  timeout-sec: %v", timeout))

	return sb.String()
}

// Configure : Pipeline will call this method after constructor so that you can read config and initialize yourself
//
//	Return failure if any config is not valid to exit the process
func (c *FileCache) Configure(_ bool) error {
	log.Trace("FileCache::Configure : %s", c.Name())

	conf := FileCacheOptions{}
	err := config.UnmarshalKey(compName, &conf)
	if err != nil {
		log.Err("FileCache: config error [invalid config attributes]")
		return fmt.Errorf("config error in %s [%s]", c.Name(), err.Error())
	}

	c.createEmptyFile = conf.CreateEmptyFile
	if config.IsSet(compName + ".file-cache-timeout-in-seconds") {
		c.cacheTimeout = float64(conf.V1Timeout)
	} else if config.IsSet(compName + ".timeout-sec") {
		c.cacheTimeout = float64(conf.Timeout)
	} else {
		c.cacheTimeout = float64(defaultFileCacheTimeout)
	}

	directIO := false
	_ = config.UnmarshalKey("direct-io", &directIO)

	if directIO {
		c.cacheTimeout = 0
		log.Crit("FileCache::Configure : Direct IO mode enabled, cache timeout is set to 0")
	}

	if config.IsSet(compName + ".empty-dir-check") {
		c.allowNonEmpty = !conf.EmptyDirCheck
	} else {
		c.allowNonEmpty = conf.AllowNonEmpty
	}
	c.cleanupOnStart = conf.CleanupOnStart
	c.policyTrace = conf.EnablePolicyTrace
	c.offloadIO = conf.OffloadIO
	c.syncToFlush = conf.SyncToFlush
	c.syncToDelete = !conf.SyncNoOp
	c.refreshSec = conf.RefreshSec
	c.hardLimit = conf.HardLimit

	err = config.UnmarshalKey("lazy-write", &c.lazyWrite)
	if err != nil {
		log.Err("FileCache: config error [unable to obtain lazy-write]")
		return fmt.Errorf("config error in %s [%s]", c.Name(), err.Error())
	}

	c.tmpPath = common.ExpandPath(conf.TmpPath)
	if c.tmpPath == "" {
		log.Err("FileCache: config error [tmp-path not set]")
		return fmt.Errorf("config error in %s error [tmp-path not set]", c.Name())
	}

	err = config.UnmarshalKey("mount-path", &c.mountPath)
	if err != nil {
		log.Err("FileCache: config error [unable to obtain Mount Path]")
		return fmt.Errorf("config error in %s [%s]", c.Name(), err.Error())
	}
	if c.mountPath == c.tmpPath {
		log.Err("FileCache: config error [tmp-path is same as mount path]")
		return fmt.Errorf("config error in %s error [tmp-path is same as mount path]", c.Name())
	}

	// Extract values from 'conf' and store them as you wish here
	_, err = os.Stat(c.tmpPath)
	if os.IsNotExist(err) {
		log.Err("FileCache: config error [tmp-path does not exist. attempting to create tmp-path.]")
		err := os.MkdirAll(c.tmpPath, os.FileMode(0755))
		if err != nil {
			log.Err("FileCache: config error creating directory after clean [%s]", err.Error())
			return fmt.Errorf("config error in %s [%s]", c.Name(), err.Error())
		}
	}

	var stat syscall.Statfs_t
	err = syscall.Statfs(c.tmpPath, &stat)
	if err != nil {
		log.Err("FileCache::Configure : config error %s [%s]. Assigning a default value of 4GB or if any value is assigned to .disk-size-mb in config.", c.Name(), err.Error())
		c.maxCacheSize = 4192 * MB
	} else {
		c.maxCacheSize = 0.8 * float64(stat.Bavail) * float64(stat.Bsize)
	}

	if config.IsSet(compName+".max-size-mb") && conf.MaxSizeMB != 0 {
		c.maxCacheSize = conf.MaxSizeMB
	}

	if !isLocalDirEmpty(c.tmpPath) && !c.allowNonEmpty {
		log.Err("FileCache: config error %s directory is not empty", c.tmpPath)
		return fmt.Errorf("config error in %s [%s]", c.Name(), "temp directory not empty")
	}

	err = config.UnmarshalKey("allow-other", &c.allowOther)
	if err != nil {
		log.Err("FileCache::Configure : config error [unable to obtain allow-other]")
		return fmt.Errorf("config error in %s [%s]", c.Name(), err.Error())
	}

	if c.allowOther {
		c.defaultPermission = common.DefaultAllowOtherPermissionBits
	} else {
		c.defaultPermission = common.DefaultFilePermissionBits
	}

	cacheConfig := c.GetPolicyConfig(conf)
	c.policy = NewLRUPolicy(cacheConfig)

	if c.policy == nil {
		log.Err("FileCache::Configure : failed to create cache eviction policy")
		return fmt.Errorf("config error in %s [%s]", c.Name(), "failed to create cache policy")
	}

	if config.IsSet(compName + ".background-download") {
		log.Warn("unsupported v1 CLI parameter: background-download is not supported in blobfuse2. Consider using the streaming component.")
	}
	if config.IsSet(compName + ".cache-poll-timeout-msec") {
		log.Warn("unsupported v1 CLI parameter: cache-poll-timeout-msec is not supported in blobfuse2. Polling occurs every timeout interval.")
	}
	if config.IsSet(compName + ".upload-modified-only") {
		log.Warn("unsupported v1 CLI parameter: upload-modified-only is always true in blobfuse2.")
	}
	if config.IsSet(compName + ".sync-to-flush") {
		log.Warn("Sync will upload current contents of file.")
	}

	c.diskHighWaterMark = 0
	if conf.HardLimit && conf.MaxSizeMB != 0 {
		c.diskHighWaterMark = (((conf.MaxSizeMB * MB) * float64(cacheConfig.highThreshold)) / 100)
	}

	log.Crit("FileCache::Configure : create-empty %t, cache-timeout %d, tmp-path %s, max-size-mb %d, high-mark %d, low-mark %d, refresh-sec %v, max-eviction %v, hard-limit %v, policy %s, allow-non-empty-temp %t, cleanup-on-start %t, policy-trace %t, offload-io %t, sync-to-flush %t, ignore-sync %t, defaultPermission %v, diskHighWaterMark %v, maxCacheSize %v, mountPath %v",
		c.createEmptyFile, int(c.cacheTimeout), c.tmpPath, int(cacheConfig.maxSizeMB), int(cacheConfig.highThreshold), int(cacheConfig.lowThreshold), c.refreshSec, cacheConfig.maxEviction, c.hardLimit, conf.Policy, c.allowNonEmpty, c.cleanupOnStart, c.policyTrace, c.offloadIO, c.syncToFlush, c.syncToDelete, c.defaultPermission, c.diskHighWaterMark, c.maxCacheSize, c.mountPath)

	return nil
}

// OnConfigChange : If component has registered, on config file change this method is called
func (c *FileCache) OnConfigChange() {
	log.Trace("FileCache::OnConfigChange : %s", c.Name())

	conf := FileCacheOptions{}
	err := config.UnmarshalKey(compName, &conf)
	if err != nil {
		log.Err("FileCache: config error [invalid config attributes]")
	}

	c.createEmptyFile = conf.CreateEmptyFile
	c.cacheTimeout = float64(conf.Timeout)
	c.policyTrace = conf.EnablePolicyTrace
	c.offloadIO = conf.OffloadIO
	c.maxCacheSize = conf.MaxSizeMB
	c.syncToFlush = conf.SyncToFlush
	c.syncToDelete = !conf.SyncNoOp
	_ = c.policy.UpdateConfig(c.GetPolicyConfig(conf))
}

func (c *FileCache) StatFs() (*syscall.Statfs_t, bool, error) {
	// cache_size = f_blocks * f_frsize/1024
	// cache_size - used = f_frsize * f_bavail/1024
	// cache_size - used = vfs.f_bfree * vfs.f_frsize / 1024
	// if cache size is set to 0 then we have the root mount usage
	maxCacheSize := c.maxCacheSize * MB
	if maxCacheSize == 0 {
		return nil, false, nil
	}

	usage, _ := common.GetUsage(c.tmpPath)
	usage = usage * MB

	available := maxCacheSize - usage
	statfs := &syscall.Statfs_t{}
	err := syscall.Statfs("/", statfs)
	if err != nil {
		log.Debug("FileCache::StatFs : statfs err [%s].", err.Error())
		return nil, false, err
	}
	statfs.Blocks = uint64(maxCacheSize) / uint64(statfs.Frsize)
	statfs.Bavail = uint64(math.Max(0, available)) / uint64(statfs.Frsize)
	statfs.Bfree = statfs.Bavail

	return statfs, true, nil
}

func (c *FileCache) GetPolicyConfig(conf FileCacheOptions) cachePolicyConfig {
	// A user provided value of 0 doesn't make sense for MaxEviction, HighThreshold or LowThreshold.
	if conf.MaxEviction == 0 {
		conf.MaxEviction = defaultMaxEviction
	}
	if conf.HighThreshold == 0 {
		conf.HighThreshold = defaultMaxThreshold
	}
	if conf.LowThreshold == 0 {
		conf.LowThreshold = defaultMinThreshold
	}

	cacheConfig := cachePolicyConfig{
		tmpPath:       c.tmpPath,
		maxEviction:   conf.MaxEviction,
		highThreshold: float64(conf.HighThreshold),
		lowThreshold:  float64(conf.LowThreshold),
		cacheTimeout:  uint32(c.cacheTimeout),
		maxSizeMB:     conf.MaxSizeMB,
		fileLocks:     c.fileLocks,
		policyTrace:   conf.EnablePolicyTrace,
	}

	return cacheConfig
}

// isLocalDirEmpty: Whether or not the local directory is empty.
func isLocalDirEmpty(path string) bool {
	f, _ := os.Open(path)
	defer f.Close()

	_, err := f.Readdirnames(1)
	return err == io.EOF
}

// invalidateDirectory: Recursively invalidates a directory in the file cache.
func (fc *FileCache) invalidateDirectory(name string) {
	log.Trace("FileCache::invalidateDirectory : %s", name)

	localPath := filepath.Join(fc.tmpPath, name)
	_, err := os.Stat(localPath)
	if os.IsNotExist(err) {
		log.Info("FileCache::invalidateDirectory : %s does not exist in local cache.", name)
		return
	} else if err != nil {
		log.Debug("FileCache::invalidateDirectory : %s stat err [%s].", name, err.Error())
		return
	}
	// TODO : wouldn't this cause a race condition? a thread might get the lock before we purge - and the file would be non-existent
	err = filepath.WalkDir(localPath, func(path string, d fs.DirEntry, err error) error {
		if err == nil && d != nil {
			log.Debug("FileCache::invalidateDirectory : %s (%d) getting removed from cache", path, d.IsDir())
			if !d.IsDir() {
				fc.policy.CachePurge(path)
			} else {
				_ = deleteFile(path)
			}
		}
		return nil
	})

	if err != nil {
		log.Debug("FileCache::invalidateDirectory : Failed to iterate directory %s [%s].", localPath, err.Error())
		return
	}

	_ = deleteFile(localPath)
}

// Note: The primary purpose of the file cache is to keep track of files that are opened by the user.
// So we do not need to support some APIs like Create Directory since the file cache will manage
// creating local directories as needed.

// DeleteDir: Recursively invalidate the directory and its children
func (fc *FileCache) DeleteDir(options internal.DeleteDirOptions) error {
	log.Trace("FileCache::DeleteDir : %s", options.Name)

	err := fc.NextComponent().DeleteDir(options)
	if err != nil {
		log.Err("FileCache::DeleteDir : %s failed", options.Name)
		// There is a chance that meta file for directory was not created in which case
		// rest api delete will fail while we still need to cleanup the local cache for the same
	}

	go fc.invalidateDirectory(options.Name)
	return err
}

// Creates a new object attribute
func newObjAttr(path string, info fs.FileInfo) *internal.ObjAttr {
	stat := info.Sys().(*syscall.Stat_t)
	attrs := &internal.ObjAttr{
		Path:  path,
		Name:  info.Name(),
		Size:  info.Size(),
		Mode:  info.Mode(),
		Mtime: time.Unix(stat.Mtim.Sec, stat.Mtim.Nsec),
		Atime: time.Unix(stat.Atim.Sec, stat.Atim.Nsec),
		Ctime: time.Unix(stat.Ctim.Sec, stat.Ctim.Nsec),
	}

	if info.Mode()&os.ModeSymlink != 0 {
		attrs.Flags.Set(internal.PropFlagSymlink)
	} else if info.IsDir() {
		attrs.Flags.Set(internal.PropFlagIsDir)
	}

	return attrs
}

// ReadDir: Consolidate entries in storage and local cache to return the children under this path.
func (fc *FileCache) ReadDir(options internal.ReadDirOptions) ([]*internal.ObjAttr, error) {
	log.Trace("FileCache::ReadDir : %s", options.Name)

	// For read directory, there are three different child path situations we have to potentially handle.
	// 1. Path in storage but not in local cache
	// 2. Path not in storage but in local cache (this could happen if we recently created the file [and are currently writing to it]) (also supports immutable containers)
	// 3. Path in storage and in local cache (this could result in dirty properties on the service if we recently wrote to the file)

	// To cover case 1, grab all entries from storage
	attrs, err := fc.NextComponent().ReadDir(options)
	if err != nil {
		log.Err("FileCache::ReadDir : error fetching storage attributes [%s]", err.Error())
		// TODO : Should we return here if the directory failed to be read from storage?
	}

	// Create a mapping from path to index in the storage attributes array, so we can handle case 3 (conflicting attributes)
	var pathToIndex = make(map[string]int)
	for i, attr := range attrs {
		pathToIndex[attr.Path] = i
	}

	// To cover cases 2 and 3, grab entries from the local cache
	localPath := filepath.Join(fc.tmpPath, options.Name)
	dirents, err := os.ReadDir(localPath)

	// If the local ReadDir fails it means the directory falls under case 1.
	// The directory will not exist locally even if it exists in the container
	// if the directory was freshly created or no files have been updated in the directory recently.
	if err == nil {
		// Enumerate over the results from the local cache and update/add to attrs to return if necessary (to support case 2 and 3)
		for _, entry := range dirents {
			entryPath := filepath.Join(options.Name, entry.Name())
			entryCachePath := filepath.Join(fc.tmpPath, entryPath)

			info, err := os.Stat(entryCachePath) // Grab local cache attributes
			// All directory operations are guaranteed to be synced with storage so they cannot be in a case 2 or 3 state.
			if err == nil && !info.IsDir() {
				idx, ok := pathToIndex[filepath.Join(options.Name, entry.Name())] // Grab the index of the corresponding storage attributes

				if ok { // Case 3 (file in storage and in local cache) so update the relevant attributes
					// Return from local cache only if file is not under download or deletion
					// If file is under download then taking size or mod time from it will be incorrect.
					if !fc.fileLocks.Locked(entryPath) {
						log.Debug("FileCache::ReadDir : updating %s from local cache", entryPath)
						attrs[idx].Size = info.Size()
						attrs[idx].Mtime = info.ModTime()
					}
				} else if !fc.createEmptyFile { // Case 2 (file only in local cache) so create a new attributes and add them to the storage attributes
					log.Debug("FileCache::ReadDir : serving %s from local cache", entryPath)
					attr := newObjAttr(entryPath, info)
					attrs = append(attrs, attr)
					pathToIndex[attr.Path] = len(attrs) - 1 // append adds to the end of an array
				}
			}
		}
	} else {
		log.Debug("FileCache::ReadDir : error fetching local attributes [%s]", err.Error())
	}

	return attrs, nil
}

// StreamDir : Add local files to the list retrieved from storage container
func (fc *FileCache) StreamDir(options internal.StreamDirOptions) ([]*internal.ObjAttr, string, error) {
	attrs, token, err := fc.NextComponent().StreamDir(options)

	if token == "" {
		// This is the last set of objects retrieved from container so we need to add local files here
		localPath := filepath.Join(fc.tmpPath, options.Name)
		dirents, err := os.ReadDir(localPath)

		if err == nil {
			// Enumerate over the results from the local cache and add to attrs
			for _, entry := range dirents {
				entryPath := filepath.Join(options.Name, entry.Name())
				entryCachePath := filepath.Join(fc.tmpPath, entryPath)

				info, err := os.Stat(entryCachePath) // Grab local cache attributes
				// If local file is not locked then only use its attributes otherwise rely on container attributes
				if err == nil && !info.IsDir() &&
					!fc.fileLocks.Locked(entryPath) {

					// This is an overhead for streamdir for now
					// As list is paginated we have no way to know whether this particular item exists both in local cache
					// and container or not. So we rely on getAttr to tell if entry was cached then it exists in storage too
					// If entry does not exists on storage then only return a local item here.
					_, err := fc.NextComponent().GetAttr(internal.GetAttrOptions{Name: entryPath})
					if err != nil && (err == syscall.ENOENT || os.IsNotExist(err)) {
						log.Debug("FileCache::StreamDir : serving %s from local cache", entryPath)
						attr := newObjAttr(entryPath, info)
						attrs = append(attrs, attr)
					}
				}
			}
		}
	}

	return attrs, token, err
}

// IsDirEmpty: Whether or not the directory is empty
func (fc *FileCache) IsDirEmpty(options internal.IsDirEmptyOptions) bool {
	log.Trace("FileCache::IsDirEmpty : %s", options.Name)

	// If the directory does not exist locally then call the next component
	localPath := filepath.Join(fc.tmpPath, options.Name)
	f, err := os.Open(localPath)
	if err == nil {
		log.Debug("FileCache::IsDirEmpty : %s found in local cache", options.Name)

		// Check local cache directory is empty or not
		path, err := f.Readdirnames(1)

		// If the local directory has a path in it, it is likely due to !createEmptyFile.
		if err == nil && !fc.createEmptyFile && len(path) > 0 {
			log.Debug("FileCache::IsDirEmpty : %s had a subpath in the local cache", options.Name)
			return false
		}

		// If there are files in local cache then dont allow deletion of directory
		if err != io.EOF {
			// Local directory is not empty fail the call
			log.Debug("FileCache::IsDirEmpty : %s was not empty in local cache", options.Name)
			return false
		}
	} else if os.IsNotExist(err) {
		// Not found in local cache so check with container
		log.Debug("FileCache::IsDirEmpty : %s not found in local cache", options.Name)
	} else {
		// Unknown error, check with container
		log.Err("FileCache::IsDirEmpty : %s failed while checking local cache [%s]", options.Name, err.Error())
	}

	log.Debug("FileCache::IsDirEmpty : %s checking with container", options.Name)
	return fc.NextComponent().IsDirEmpty(options)
}

// DeleteEmptyDirs: delete empty directories in local cache, return error if directory is not empty
func (fc *FileCache) DeleteEmptyDirs(options internal.DeleteDirOptions) error {
	localPath := options.Name
	if !strings.Contains(options.Name, fc.tmpPath) {
		localPath = filepath.Join(fc.tmpPath, options.Name)
	}

	log.Trace("FileCache::DeleteEmptyDirs : %s", localPath)

	entries, err := os.ReadDir(localPath)
	if err != nil {
		log.Debug("FileCache::DeleteEmptyDirs : Unable to read directory %s [%s]", localPath, err.Error())
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			err = fc.DeleteEmptyDirs(internal.DeleteDirOptions{
				Name: filepath.Join(localPath, entry.Name()),
			})
			if err != nil {
				log.Err("FileCache::DeleteEmptyDirs : Unable to delete directory %s [%s]", localPath, err.Error())
				return err
			}
		} else {
			log.Err("FileCache::DeleteEmptyDirs : Directory %s is not empty, contains file %s", localPath, entry.Name())
			return fmt.Errorf("unable to delete directory %s, contains file %s", localPath, entry.Name())
		}
	}

	if !strings.EqualFold(fc.tmpPath, localPath) {
		return os.Remove(localPath)
	}

	return nil
}

// RenameDir: Recursively invalidate the source directory and its children
func (fc *FileCache) RenameDir(options internal.RenameDirOptions) error {
	log.Trace("FileCache::RenameDir : src=%s, dst=%s", options.Src, options.Dst)

	err := fc.NextComponent().RenameDir(options)
	if err != nil {
		log.Err("FileCache::RenameDir : error %s [%s]", options.Src, err.Error())
		return err
	}

	go fc.invalidateDirectory(options.Src)
	// TLDR: Dst is guaranteed to be non-existent or empty.
	// Note: We do not need to invalidate Dst due to the logic in our FUSE connector, see comments there.
	return nil
}

// CreateFile: Create the file in local cache.
func (fc *FileCache) CreateFile(options internal.CreateFileOptions) (*handlemap.Handle, error) {
	//defer exectime.StatTimeCurrentBlock("FileCache::CreateFile")()
	log.Trace("FileCache::CreateFile : name=%s, mode=%d", options.Name, options.Mode)

	flock := fc.fileLocks.Get(options.Name)
	flock.Lock()
	defer flock.Unlock()

	// createEmptyFile was added to optionally support immutable containers. If customers do not care about immutability they can set this to true.
	if fc.createEmptyFile {
		// We tried moving CreateFile to a separate thread for better perf.
		// However, before it is created in storage, if GetAttr is called, the call will fail since the file
		// does not exist in storage yet, failing the whole CreateFile sequence in FUSE.
		_, err := fc.NextComponent().CreateFile(options)
		if err != nil {
			log.Err("FileCache::CreateFile : Failed to create file %s", options.Name)
			return nil, err
		}
	}

	// Create the file in local cache
	localPath := filepath.Join(fc.tmpPath, options.Name)
	fc.policy.CacheValid(localPath)

	err := os.MkdirAll(filepath.Dir(localPath), fc.defaultPermission)
	if err != nil {
		log.Err("FileCache::CreateFile : unable to create local directory %s [%s]", options.Name, err.Error())
		return nil, err
	}

	// Open the file and grab a shared lock to prevent deletion by the cache policy.
	f, err := os.OpenFile(localPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, options.Mode)
	if err != nil {
		log.Err("FileCache::CreateFile : error opening local file %s [%s]", options.Name, err.Error())
		return nil, err
	}
	// The user might change permissions WHILE creating the file therefore we need to account for that
	if options.Mode != common.DefaultFilePermissionBits {
		fc.missedChmodList.LoadOrStore(options.Name, true)
	}

	// Increment the handle count in this lock item as there is one handle open for this now
	flock.Inc()

	handle := handlemap.NewHandle(options.Name)
	handle.UnixFD = uint64(f.Fd())

	if !fc.offloadIO {
		handle.Flags.Set(handlemap.HandleFlagCached)
	}
	log.Info("FileCache::CreateFile : file=%s, fd=%d", options.Name, f.Fd())

	handle.SetFileObject(f)

	// If an empty file is created in storage then there is no need to upload if FlushFile is called immediately after CreateFile.
	if !fc.createEmptyFile {
		handle.Flags.Set(handlemap.HandleFlagDirty)
	}

	return handle, nil
}

// Validate that storage 404 errors truly correspond to Does Not Exist.
// path: the storage path
// err: the storage error
// method: the caller method name
// recoverable: whether or not case 2 is recoverable on flush/close of the file
func (fc *FileCache) validateStorageError(path string, err error, method string, recoverable bool) error {
	// For methods that take in file name, the goal is to update the path in storage and the local cache.
	// See comments in GetAttr for the different situations we can run into. This specifically handles case 2.
	if err != nil {
		if err == syscall.ENOENT || os.IsNotExist(err) {
			log.Debug("FileCache::%s : %s does not exist in storage", method, path)
			if !fc.createEmptyFile {
				// Check if the file exists in the local cache
				// (policy might not think the file exists if the file is merely marked for evication and not actually evicted yet)
				localPath := filepath.Join(fc.tmpPath, path)
				_, err := os.Stat(localPath)
				if os.IsNotExist(err) { // If the file is not in the local cache, then the file does not exist.
					log.Err("FileCache::%s : %s does not exist in local cache", method, path)
					return syscall.ENOENT
				} else {
					if !recoverable {
						log.Err("FileCache::%s : %s has not been closed/flushed yet, unable to recover this operation on close", method, path)
						return syscall.EIO
					} else {
						log.Info("FileCache::%s : %s has not been closed/flushed yet, we can recover this operation on close", method, path)
						return nil
					}
				}
			}
		} else {
			return err
		}
	}
	return nil
}

// DeleteFile: Invalidate the file in local cache.
func (fc *FileCache) DeleteFile(options internal.DeleteFileOptions) error {
	log.Trace("FileCache::DeleteFile : name=%s", options.Name)

	flock := fc.fileLocks.Get(options.Name)
	flock.Lock()
	defer flock.Unlock()

	err := fc.NextComponent().DeleteFile(options)
	err = fc.validateStorageError(options.Name, err, "DeleteFile", false)
	if err != nil {
		log.Err("FileCache::DeleteFile : error  %s [%s]", options.Name, err.Error())
		return err
	}

	localPath := filepath.Join(fc.tmpPath, options.Name)
	err = deleteFile(localPath)
	if err != nil && !os.IsNotExist(err) {
		log.Err("FileCache::DeleteFile : failed to delete local file %s [%s]", localPath, err.Error())
	}

	fc.policy.CachePurge(localPath)

	return nil
}

// isDownloadRequired: Whether or not the file needs to be downloaded to local cache.
func (fc *FileCache) isDownloadRequired(localPath string, blobPath string, flock *common.LockMapItem) (bool, bool, *internal.ObjAttr, error) {
	fileExists := false
	downloadRequired := false
	lmt := time.Time{}
	var stat *syscall.Stat_t = nil

	// The file is not cached then we need to download
	if !fc.policy.IsCached(localPath) {
		log.Debug("FileCache::isDownloadRequired : %s not present in local cache policy", localPath)
		downloadRequired = true
	}

	finfo, err := os.Stat(localPath)
	if err == nil {
		// The file exists in local cache
		// The file needs to be downloaded if the cacheTimeout elapsed (check last change time and last modified time)
		fileExists = true
		stat = finfo.Sys().(*syscall.Stat_t)

		// Deciding based on last modified time is not correct. Last modified time is based on the file was last written
		// so if file was last written back to container 2 days back then even downloading it now shall represent the same date
		// hence immediately after download it will become invalid. It shall be based on when the file was last downloaded.
		// We can rely on last change time because once file is downloaded we reset its last mod time (represent same time as
		// container on the local disk by resetting last mod time of local disk with utimens)
		// and hence last change time on local disk will then represent the download time.

		lmt = finfo.ModTime()
		if time.Since(finfo.ModTime()).Seconds() > fc.cacheTimeout &&
			time.Since(time.Unix(stat.Ctim.Sec, stat.Ctim.Nsec)).Seconds() > fc.cacheTimeout {
			log.Debug("FileCache::isDownloadRequired : %s not valid as per time checks", localPath)
			downloadRequired = true
		}
	} else if os.IsNotExist(err) {
		// The file does not exist in the local cache so it needs to be downloaded
		log.Debug("FileCache::isDownloadRequired : %s not present in local cache", localPath)
		downloadRequired = true
	} else {
		// Catch all, the file needs to be downloaded
		log.Debug("FileCache::isDownloadRequired : error calling stat %s [%s]", localPath, err.Error())
		downloadRequired = true
	}

	if fileExists && flock.Count() > 0 {
		// file exists in local cache and there is already an handle open for it
		// In this case we can not redownload the file from container
		log.Info("FileCache::isDownloadRequired : Need to re-download %s, but skipping as handle is already open", blobPath)
		downloadRequired = false
	}

	err = nil // reset err variable
	var attr *internal.ObjAttr = nil
	if downloadRequired ||
		(fc.refreshSec != 0 && time.Since(flock.DownloadTime()).Seconds() > float64(fc.refreshSec)) {
		attr, err = fc.NextComponent().GetAttr(internal.GetAttrOptions{Name: blobPath})
		if err != nil {
			log.Err("FileCache::isDownloadRequired : Failed to get attr of %s [%s]", blobPath, err.Error())
		}
	}

	if fc.refreshSec != 0 && !downloadRequired && attr != nil && stat != nil {
		// We decided that based on lmt of file file-cache-timeout has not expired
		// However, user has configured refresh time then check time has elapsed since last download time of file or not
		// If so, compare the lmt of file in local cache and once in container and redownload only if lmt of container is latest.
		// If time matches but size does not then still we need to redownlaod the file.
		if attr.Mtime.After(lmt) || stat.Size != attr.Size {
			// File has not been modified at storage yet so no point in redownloading the file
			log.Info("FileCache::isDownloadRequired : File is modified in container, so forcing redownload %s [A-%v : L-%v] [A-%v : L-%v]",
				blobPath, attr.Mtime, lmt, attr.Size, stat.Size)
			downloadRequired = true

			// As we have decided to continue using old file, we reset the timer to check again after refresh time interval
			flock.SetDownloadTime()
		} else {
			log.Info("FileCache::isDownloadRequired : File in container is not latest, skip redownload %s [A-%v : L-%v]", blobPath, attr.Mtime, lmt)
		}
	}

	return downloadRequired, fileExists, attr, err
}

// OpenFile: Makes the file available in the local cache for further file operations.
func (fc *FileCache) OpenFile(options internal.OpenFileOptions) (*handlemap.Handle, error) {
	log.Trace("FileCache::OpenFile : name=%s, flags=%d, mode=%s", options.Name, options.Flags, options.Mode)

	localPath := filepath.Join(fc.tmpPath, options.Name)
	var f *os.File
	var err error

	flock := fc.fileLocks.Get(options.Name)
	flock.Lock()
	defer flock.Unlock()

	fc.policy.CacheValid(localPath)
	downloadRequired, fileExists, attr, err := fc.isDownloadRequired(localPath, options.Name, flock)

	// return err in case of authorization permission mismatch
	if err != nil && err == syscall.EACCES {
		return nil, err
	}

	if downloadRequired {
		log.Debug("FileCache::OpenFile : Need to re-download %s", options.Name)

		fileSize := int64(0)
		if attr != nil {
			fileSize = int64(attr.Size)
		}

		if fileExists {
			log.Debug("FileCache::OpenFile : Delete cached file %s", options.Name)

			err := deleteFile(localPath)
			if err != nil && !os.IsNotExist(err) {
				log.Err("FileCache::OpenFile : Failed to delete old file %s", options.Name)
			}
		} else {
			// Create the file if if doesn't already exist.
			err := os.MkdirAll(filepath.Dir(localPath), fc.defaultPermission)
			if err != nil {
				log.Err("FileCache::OpenFile : error creating directory structure for file %s [%s]", options.Name, err.Error())
				return nil, err
			}
		}

		// Open the file in write mode.
		f, err = os.OpenFile(localPath, os.O_CREATE|os.O_RDWR, options.Mode)
		if err != nil {
			log.Err("FileCache::OpenFile : error creating new file %s [%s]", options.Name, err.Error())
			return nil, err
		}

		if options.Flags&os.O_TRUNC != 0 {
			fileSize = 0
		}

		if fileSize > 0 {
			if fc.diskHighWaterMark != 0 {
				currSize, err := common.GetUsage(fc.tmpPath)
				if err != nil {
					log.Err("FileCache::OpenFile : error getting current usage of cache [%s]", err.Error())
				} else {
					if (currSize + float64(fileSize)) > fc.diskHighWaterMark {
						log.Err("FileCache::OpenFile : cache size limit reached [%f] failed to open %s", fc.maxCacheSize, options.Name)
						return nil, syscall.ENOSPC
					}
				}

			}
			// Download/Copy the file from storage to the local file.
			err = fc.NextComponent().CopyToFile(
				internal.CopyToFileOptions{
					Name:   options.Name,
					Offset: 0,
					Count:  fileSize,
					File:   f,
				})
			if err != nil {
				// File was created locally and now download has failed so we need to delete it back from local cache
				log.Err("FileCache::OpenFile : error downloading file from storage %s [%s]", options.Name, err.Error())
				_ = f.Close()
				_ = os.Remove(localPath)
				return nil, err
			}
		}

		// Update the last download time of this file
		flock.SetDownloadTime()

		log.Debug("FileCache::OpenFile : Download of %s is complete", options.Name)
		f.Close()

		// After downloading the file, update the modified times and mode of the file.
		fileMode := fc.defaultPermission
		if attr != nil && !attr.IsModeDefault() {
			fileMode = attr.Mode
		}

		// If user has selected some non default mode in config then every local file shall be created with that mode only
		err = os.Chmod(localPath, fileMode)
		if err != nil {
			log.Err("FileCache::OpenFile : Failed to change mode of file %s [%s]", options.Name, err.Error())
		}
		// TODO: When chown is supported should we update that?

		if attr != nil {
			// chtimes shall be the last api otherwise calling chmod/chown will update the last change time
			err = os.Chtimes(localPath, attr.Atime, attr.Mtime)
			if err != nil {
				log.Err("FileCache::OpenFile : Failed to change times of file %s [%s]", options.Name, err.Error())
			}
		}

		fileCacheStatsCollector.UpdateStats(stats_manager.Increment, dlFiles, (int64)(1))
	} else {
		log.Debug("FileCache::OpenFile : %s will be served from cache", options.Name)
		fileCacheStatsCollector.UpdateStats(stats_manager.Increment, cacheServed, (int64)(1))
	}

	// Open the file and grab a shared lock to prevent deletion by the cache policy.
	f, err = os.OpenFile(localPath, options.Flags, options.Mode)
	if err != nil {
		log.Err("FileCache::OpenFile : error opening cached file %s [%s]", options.Name, err.Error())
		return nil, err
	}

	// Increment the handle count in this lock item as there is one handle open for this now
	flock.Inc()

	handle := handlemap.NewHandle(options.Name)
	inf, err := f.Stat()
	if err == nil {
		handle.Size = inf.Size()
	}

	handle.UnixFD = uint64(f.Fd())
	if !fc.offloadIO {
		handle.Flags.Set(handlemap.HandleFlagCached)
	}

	log.Info("FileCache::OpenFile : file=%s, fd=%d", options.Name, f.Fd())
	handle.SetFileObject(f)

	return handle, nil
}

// CloseFile: Flush the file and invalidate it from the cache.
func (fc *FileCache) CloseFile(options internal.CloseFileOptions) error {
	// Lock the file so that while close is in progress no one can open the file again
	flock := fc.fileLocks.Get(options.Handle.Path)
	flock.Lock()

	// Async close is called so schedule the upload and return here
	fc.fileCloseOpt.Add(1)

	if !fc.lazyWrite {
		// Sync close is called so wait till the upload completes
		return fc.closeFileInternal(options, flock)
	}

	go fc.closeFileInternal(options, flock) //nolint
	return nil
}

// closeFileInternal: Actual handling of the close file goes here
func (fc *FileCache) closeFileInternal(options internal.CloseFileOptions, flock *common.LockMapItem) error {
	log.Trace("FileCache::closeFileInternal : name=%s, handle=%d", options.Handle.Path, options.Handle.ID)

	// Lock is acquired by CloseFile, at end of this method we need to unlock
	// If its async call file shall be locked till the upload completes.
	defer flock.Unlock()
	defer fc.fileCloseOpt.Done()

	localPath := filepath.Join(fc.tmpPath, options.Handle.Path)

	err := fc.FlushFile(internal.FlushFileOptions{Handle: options.Handle, CloseInProgress: true}) //nolint
	if err != nil {
		log.Err("FileCache::closeFileInternal : failed to flush file %s", options.Handle.Path)
		return err
	}

	f := options.Handle.GetFileObject()
	if f == nil {
		log.Err("FileCache::closeFileInternal : error [missing fd in handle object] %s", options.Handle.Path)
		return syscall.EBADF
	}

	err = f.Close()
	if err != nil {
		log.Err("FileCache::closeFileInternal : error closing file %s(%d) [%s]", options.Handle.Path, int(f.Fd()), err.Error())
		return err
	}
	flock.Dec()

	// If it is an fsync op then purge the file
	if options.Handle.Fsynced() {
		log.Trace("FileCache::closeFileInternal : fsync/sync op, purging %s", options.Handle.Path)
		localPath := filepath.Join(fc.tmpPath, options.Handle.Path)

		err = deleteFile(localPath)
		if err != nil && !os.IsNotExist(err) {
			log.Err("FileCache::closeFileInternal : failed to delete local file %s [%s]", localPath, err.Error())
		}

		fc.policy.CachePurge(localPath)
		return nil
	}

	fc.policy.CacheInvalidate(localPath) // Invalidate the file from the local cache.
	return nil
}

// ReadFile: Read the local file
func (fc *FileCache) ReadFile(options internal.ReadFileOptions) ([]byte, error) {
	// The file should already be in the cache since CreateFile/OpenFile was called before and a shared lock was acquired.
	localPath := filepath.Join(fc.tmpPath, options.Handle.Path)
	fc.policy.CacheValid(localPath)

	f := options.Handle.GetFileObject()
	if f == nil {
		log.Err("FileCache::ReadFile : error [couldn't find fd in handle] %s", options.Handle.Path)
		return nil, syscall.EBADF
	}

	// Get file info so we know the size of data we expect to read.
	info, err := f.Stat()
	if err != nil {
		log.Err("FileCache::ReadFile : error stat %s [%s] ", options.Handle.Path, err.Error())
		return nil, err
	}
	data := make([]byte, info.Size())
	bytesRead, err := f.Read(data)

	if int64(bytesRead) != info.Size() {
		log.Err("FileCache::ReadFile : error [couldn't read entire file] %s", options.Handle.Path)
		return nil, syscall.EIO
	}

	return data, err
}

// ReadInBuffer: Read the local file into a buffer
func (fc *FileCache) ReadInBuffer(options internal.ReadInBufferOptions) (int, error) {
	//defer exectime.StatTimeCurrentBlock("FileCache::ReadInBuffer")()
	// The file should already be in the cache since CreateFile/OpenFile was called before and a shared lock was acquired.
	// log.Debug("FileCache::ReadInBuffer : Reading %v bytes from %s", len(options.Data), options.Handle.Path)

	f := options.Handle.GetFileObject()
	if f == nil {
		log.Err("FileCache::ReadInBuffer : error [couldn't find fd in handle] %s", options.Handle.Path)
		return 0, syscall.EBADF
	}

	// Read and write operations are very frequent so updating cache policy for every read is a costly operation
	// Update cache policy every 1K operations (includes both read and write) instead
	options.Handle.OptCnt++
	if (options.Handle.OptCnt % defaultCacheUpdateCount) == 0 {
		localPath := filepath.Join(fc.tmpPath, options.Handle.Path)
		fc.policy.CacheValid(localPath)
	}

	// Removing f.ReadAt as it involves lot of house keeping and then calls syscall.Pread
	// Instead we will call syscall directly for better perf
	return syscall.Pread(options.Handle.FD(), options.Data, options.Offset)
}

// WriteFile: Write to the local file
func (fc *FileCache) WriteFile(options internal.WriteFileOptions) (int, error) {
	//defer exectime.StatTimeCurrentBlock("FileCache::WriteFile")()
	// The file should already be in the cache since CreateFile/OpenFile was called before and a shared lock was acquired.
	//log.Debug("FileCache::WriteFile : Writing %v bytes from %s", len(options.Data), options.Handle.Path)

	f := options.Handle.GetFileObject()
	if f == nil {
		log.Err("FileCache::WriteFile : error [couldn't find fd in handle] %s", options.Handle.Path)
		return 0, syscall.EBADF
	}

	if fc.diskHighWaterMark != 0 {
		currSize, err := common.GetUsage(fc.tmpPath)
		if err != nil {
			log.Err("FileCache::WriteFile : error getting current usage of cache [%s]", err.Error())
		} else {
			if (currSize + float64(len(options.Data))) > fc.diskHighWaterMark {
				log.Err("FileCache::WriteFile : cache size limit reached [%f] failed to open %s", fc.maxCacheSize, options.Handle.Path)
				return 0, syscall.ENOSPC
			}
		}
	}

	// Read and write operations are very frequent so updating cache policy for every read is a costly operation
	// Update cache policy every 1K operations (includes both read and write) instead
	options.Handle.OptCnt++
	if (options.Handle.OptCnt % defaultCacheUpdateCount) == 0 {
		localPath := filepath.Join(fc.tmpPath, options.Handle.Path)
		fc.policy.CacheValid(localPath)
	}

	// Removing f.WriteAt as it involves lot of house keeping and then calls syscall.Pwrite
	// Instead we will call syscall directly for better perf
	bytesWritten, err := syscall.Pwrite(options.Handle.FD(), options.Data, options.Offset)

	if err == nil {
		// Mark the handle dirty so the file is written back to storage on FlushFile.
		options.Handle.Flags.Set(handlemap.HandleFlagDirty)

	} else {
		log.Err("FileCache::WriteFile : failed to write %s [%s]", options.Handle.Path, err.Error())
	}

	return bytesWritten, err
}

func (fc *FileCache) SyncFile(options internal.SyncFileOptions) error {
	log.Trace("FileCache::SyncFile : handle=%d, path=%s", options.Handle.ID, options.Handle.Path)
	if fc.syncToFlush {
		err := fc.FlushFile(internal.FlushFileOptions{Handle: options.Handle, CloseInProgress: true}) //nolint
		if err != nil {
			log.Err("FileCache::SyncFile : failed to flush file %s", options.Handle.Path)
			return err
		}
	} else if fc.syncToDelete {
		err := fc.NextComponent().SyncFile(options)
		if err != nil {
			log.Err("FileCache::SyncFile : %s failed", options.Handle.Path)
			return err
		}

		options.Handle.Flags.Set(handlemap.HandleFlagFSynced)
	}

	return nil
}

// in SyncDir we're not going to clear the file cache for now
// on regular linux its fs responsibility
// func (fc *FileCache) SyncDir(options internal.SyncDirOptions) error {
// 	log.Trace("FileCache::SyncDir : %s", options.Name)

// 	err := fc.NextComponent().SyncDir(options)
// 	if err != nil {
// 		log.Err("FileCache::SyncDir : %s failed", options.Name)
// 		return err
// 	}
// 	// TODO: we can decide here if we want to flush all the files in the directory first or not. Currently I'm just invalidating files
// 	// within the dir
// 	go fc.invalidateDirectory(options.Name)
// 	return nil
// }

// FlushFile: Flush the local file to storage
func (fc *FileCache) FlushFile(options internal.FlushFileOptions) error {
	//defer exectime.StatTimeCurrentBlock("FileCache::FlushFile")()
	log.Trace("FileCache::FlushFile : handle=%d, path=%s", options.Handle.ID, options.Handle.Path)

	// The file should already be in the cache since CreateFile/OpenFile was called before and a shared lock was acquired.
	localPath := filepath.Join(fc.tmpPath, options.Handle.Path)
	fc.policy.CacheValid(localPath)
	// if our handle is dirty then that means we wrote to the file
	if options.Handle.Dirty() {
		if fc.lazyWrite && !options.CloseInProgress {
			// As lazy-write is enable, upload will be scheduled when file is closed.
			log.Info("FileCache::FlushFile : %s will be flushed when handle %d is closed", options.Handle.Path, options.Handle.ID)
			return nil
		}

		f := options.Handle.GetFileObject()
		if f == nil {
			log.Err("FileCache::FlushFile : error [couldn't find fd in handle] %s", options.Handle.Path)
			return syscall.EBADF
		}

		// Flush all data to disk that has been buffered by the kernel.
		// We cannot close the incoming handle since the user called flush, note close and flush can be called on the same handle multiple times.
		// To ensure the data is flushed to disk before writing to storage, we duplicate the handle and close that handle.
		// f.fsync() is another option but dup+close does it quickly compared to sync
		dupFd, err := syscall.Dup(int(f.Fd()))
		if err != nil {
			log.Err("FileCache::FlushFile : error [couldn't duplicate the fd] %s", options.Handle.Path)
			return syscall.EIO
		}

		err = syscall.Close(dupFd)
		if err != nil {
			log.Err("FileCache::FlushFile : error [unable to close duplicate fd] %s", options.Handle.Path)
			return syscall.EIO
		}

		// Write to storage
		// Create a new handle for the SDK to use to upload (read local file)
		// The local handle can still be used for read and write.
		var orgMode fs.FileMode
		modeChanged := false

		uploadHandle, err := os.Open(localPath)
		if err != nil {
			if os.IsPermission(err) {
				info, _ := os.Stat(localPath)
				orgMode = info.Mode()
				newMode := orgMode | 0444
				err = os.Chmod(localPath, newMode)
				if err == nil {
					modeChanged = true
					uploadHandle, err = os.Open(localPath)
					log.Info("FileCache::FlushFile : read mode added to file %s", options.Handle.Path)
				}
			}

			if err != nil {
				log.Err("FileCache::FlushFile : error [unable to open upload handle] %s [%s]", options.Handle.Path, err.Error())
				return err
			}
		}
		err = fc.NextComponent().CopyFromFile(
			internal.CopyFromFileOptions{
				Name: options.Handle.Path,
				File: uploadHandle,
			})

		uploadHandle.Close()

		if modeChanged {
			err1 := os.Chmod(localPath, orgMode)
			if err1 != nil {
				log.Err("FileCache::FlushFile : Failed to remove read mode from file %s [%s]", options.Handle.Path, err1.Error())
			}
		}

		if err != nil {
			log.Err("FileCache::FlushFile : %s upload failed [%s]", options.Handle.Path, err.Error())
			return err
		}

		options.Handle.Flags.Clear(handlemap.HandleFlagDirty)

		// If chmod was done on the file before it was uploaded to container then setting up mode would have been missed
		// Such file names are added to this map and here post upload we try to set the mode correctly
		_, found := fc.missedChmodList.Load(options.Handle.Path)
		if found {
			// If file is found in map it means last chmod was missed on this
			// Delete the entry from map so that any further flush do not try to update the mode again
			fc.missedChmodList.Delete(options.Handle.Path)

			// When chmod on container was missed, local file was updated with correct mode
			// Here take the mode from local cache and update the container accordingly
			localPath := filepath.Join(fc.tmpPath, options.Handle.Path)
			info, err := os.Lstat(localPath)
			if err == nil {
				err = fc.Chmod(internal.ChmodOptions{Name: options.Handle.Path, Mode: info.Mode()})
				if err != nil {
					// chmod was missed earlier for this file and doing it now also
					// resulted in error so ignore this one and proceed for flush handling
					log.Err("FileCache::FlushFile : %s chmod failed [%s]", options.Handle.Path, err.Error())
				}
			}
		}
	}

	return nil
}

// GetAttr: Consolidate attributes from storage and local cache
func (fc *FileCache) GetAttr(options internal.GetAttrOptions) (*internal.ObjAttr, error) {
	log.Trace("FileCache::GetAttr : %s", options.Name)

	// For get attr, there are three different path situations we have to potentially handle.
	// 1. Path in storage but not in local cache
	// 2. Path not in storage but in local cache (this could happen if we recently created the file [and are currently writing to it]) (also supports immutable containers)
	// 3. Path in storage and in local cache (this could result in dirty properties on the service if we recently wrote to the file)

	// To cover case 1, get attributes from storage
	var exists bool
	attrs, err := fc.NextComponent().GetAttr(options)
	if err != nil {
		if err == syscall.ENOENT || os.IsNotExist(err) {
			log.Debug("FileCache::GetAttr : %s does not exist in storage", options.Name)
			exists = false
		} else {
			log.Err("FileCache::GetAttr : Failed to get attr of %s [%s]", options.Name, err.Error())
			return &internal.ObjAttr{}, err
		}
	} else {
		exists = true
	}

	// To cover cases 2 and 3, grab the attributes from the local cache
	localPath := filepath.Join(fc.tmpPath, options.Name)
	info, err := os.Lstat(localPath)
	// All directory operations are guaranteed to be synced with storage so they cannot be in a case 2 or 3 state.
	if (err == nil || os.IsExist(err)) && !info.IsDir() {
		if exists { // Case 3 (file in storage and in local cache) so update the relevant attributes
			// Return from local cache only if file is not under download or deletion
			// If file is under download then taking size or mod time from it will be incorrect.
			if !fc.fileLocks.Locked(options.Name) {
				log.Debug("FileCache::GetAttr : updating %s from local cache", options.Name)
				attrs.Size = info.Size()
				attrs.Mtime = info.ModTime()
			} else {
				log.Debug("FileCache::GetAttr : %s is locked, use storage attributes", options.Name)
			}
		} else { // Case 2 (file only in local cache) so create a new attributes and add them to the storage attributes
			if !strings.Contains(localPath, fc.tmpPath) {
				// Here if the path is going out of the temp directory then return ENOENT
				exists = false
			} else {
				log.Debug("FileCache::GetAttr : serving %s attr from local cache", options.Name)
				exists = true
				attrs = newObjAttr(options.Name, info)
			}
		}
	}

	if !exists {
		return &internal.ObjAttr{}, syscall.ENOENT
	}

	return attrs, nil
}

// RenameFile: Invalidate the file in local cache.
func (fc *FileCache) RenameFile(options internal.RenameFileOptions) error {
	log.Trace("FileCache::RenameFile : src=%s, dst=%s", options.Src, options.Dst)

	sflock := fc.fileLocks.Get(options.Src)
	sflock.Lock()
	defer sflock.Unlock()

	dflock := fc.fileLocks.Get(options.Dst)
	dflock.Lock()
	defer dflock.Unlock()

	err := fc.NextComponent().RenameFile(options)
	err = fc.validateStorageError(options.Src, err, "RenameFile", false)
	if err != nil {
		log.Err("FileCache::RenameFile : %s failed to rename file [%s]", options.Src, err.Error())
		return err
	}

	localSrcPath := filepath.Join(fc.tmpPath, options.Src)
	localDstPath := filepath.Join(fc.tmpPath, options.Dst)

	// in case of git clone multiple rename requests come for which destination files already exists in system
	// if we do not perform rename operation locally and those destination files are cached then next time they are read
	// we will be serving the wrong content (as we did not rename locally, we still be having older destination files with
	// stale content). We either need to remove dest file as well from cache or just run rename to replace the content.
	err = os.Rename(localSrcPath, localDstPath)
	if err != nil && !os.IsNotExist(err) {
		log.Err("FileCache::RenameFile : %s failed to rename local file %s [%s]", localSrcPath, err.Error())
	}

	if err != nil {
		// If there was a problem in local rename then delete the destination file
		// it might happen that dest file was already there and local rename failed
		// so deleting local dest file ensures next open of that will get the updated file from container
		err = deleteFile(localDstPath)
		if err != nil && !os.IsNotExist(err) {
			log.Err("FileCache::RenameFile : %s failed to delete local file %s [%s]", localDstPath, err.Error())
		}

		fc.policy.CachePurge(localDstPath)
	}

	err = deleteFile(localSrcPath)
	if err != nil && !os.IsNotExist(err) {
		log.Err("FileCache::RenameFile : %s failed to delete local file %s [%s]", localSrcPath, err.Error())
	}

	fc.policy.CachePurge(localSrcPath)

	if fc.cacheTimeout == 0 {
		// Destination file needs to be deleted immediately
		fc.policy.CachePurge(localDstPath)
	} else {
		// Add destination file to cache, it will be removed on timeout
		fc.policy.CacheValid(localDstPath)
	}

	return nil
}

// TruncateFile: Update the file with its new size.
func (fc *FileCache) TruncateFile(options internal.TruncateFileOptions) error {
	log.Trace("FileCache::TruncateFile : name=%s, size=%d", options.Name, options.Size)

	// If you call truncate CLI command from shell it always sends an open call first followed by truncate
	// But if you call the truncate method from a C/C++ code then open is not hit and only truncate comes

	if fc.diskHighWaterMark != 0 {
		currSize, err := common.GetUsage(fc.tmpPath)
		if err != nil {
			log.Err("FileCache::TruncateFile : error getting current usage of cache [%s]", err.Error())
		} else {
			if (currSize + float64(options.Size)) > fc.diskHighWaterMark {
				log.Err("FileCache::TruncateFile : cache size limit reached [%f] failed to open %s", fc.maxCacheSize, options.Name)
				return syscall.ENOSPC
			}
		}
	}

	var h *handlemap.Handle = nil
	var err error = nil

	if options.Size == 0 {
		// If size is 0 then no need to download any file we can just create an empty file
		h, err = fc.CreateFile(internal.CreateFileOptions{Name: options.Name, Mode: fc.defaultPermission})
		if err != nil {
			log.Err("FileCache::TruncateFile : Error creating file %s [%s]", options.Name, err.Error())
			return err
		}
	} else {
		// If size is not 0 then we need to open the file and then truncate it
		// Open will force download if file was not present in local system
		h, err = fc.OpenFile(internal.OpenFileOptions{Name: options.Name, Flags: os.O_RDWR, Mode: fc.defaultPermission})
		if err != nil {
			log.Err("FileCache::TruncateFile : Error opening file %s [%s]", options.Name, err.Error())
			return err
		}
	}

	// Update the size of the file in the local cache
	localPath := filepath.Join(fc.tmpPath, options.Name)
	fc.policy.CacheValid(localPath)

	// Truncate the file created in local system
	err = os.Truncate(localPath, options.Size)
	if err != nil {
		log.Err("FileCache::TruncateFile : error truncating cached file %s [%s]", localPath, err.Error())
		_ = fc.CloseFile(internal.CloseFileOptions{Handle: h})
		return err
	}

	// Mark the handle as dirty so that close of this file will force an upload
	h.Flags.Set(handlemap.HandleFlagDirty)

	return fc.CloseFile(internal.CloseFileOptions{Handle: h})
}

// Chmod : Update the file with its new permissions
func (fc *FileCache) Chmod(options internal.ChmodOptions) error {
	log.Trace("FileCache::Chmod : Change mode of path %s", options.Name)

	// Update the file in storage
	err := fc.NextComponent().Chmod(options)
	err = fc.validateStorageError(options.Name, err, "Chmod", false)
	if err != nil {
		if err != syscall.EIO {
			log.Err("FileCache::Chmod : %s failed to change mode [%s]", options.Name, err.Error())
			return err
		} else {
			fc.missedChmodList.LoadOrStore(options.Name, true)
		}
	}

	// Update the mode of the file in the local cache
	localPath := filepath.Join(fc.tmpPath, options.Name)
	info, err := os.Stat(localPath)
	if err == nil || os.IsExist(err) {
		fc.policy.CacheValid(localPath)

		if info.Mode() != options.Mode {
			err = os.Chmod(localPath, options.Mode)
			if err != nil {
				log.Err("FileCache::Chmod : error changing mode on the cached path %s [%s]", localPath, err.Error())
				return err
			}
		}
	}

	return nil
}

// Chown : Update the file with its new owner and group
func (fc *FileCache) Chown(options internal.ChownOptions) error {
	log.Trace("FileCache::Chown : Change owner of path %s", options.Name)

	// Update the file in storage
	err := fc.NextComponent().Chown(options)
	err = fc.validateStorageError(options.Name, err, "Chown", false)
	if err != nil {
		log.Err("FileCache::Chown : %s failed to change owner [%s]", options.Name, err.Error())
		return err
	}

	// Update the owner and group of the file in the local cache
	localPath := filepath.Join(fc.tmpPath, options.Name)
	_, err = os.Stat(localPath)
	if err == nil || os.IsExist(err) {
		fc.policy.CacheValid(localPath)

		err = os.Chown(localPath, options.Owner, options.Group)
		if err != nil {
			log.Err("FileCache::Chown : error changing owner on the cached path %s [%s]", localPath, err.Error())
			return err
		}
	}

	return nil
}

func (fc *FileCache) FileUsed(name string) error {
	// Update the owner and group of the file in the local cache
	localPath := filepath.Join(fc.tmpPath, name)
	fc.policy.CacheValid(localPath)
	return nil
}

// ------------------------- Factory -------------------------------------------

// Pipeline will call this method to create your object, initialize your variables here
// << DO NOT DELETE ANY AUTO GENERATED CODE HERE >>
func NewFileCacheComponent() internal.Component {
	comp := &FileCache{
		fileLocks: common.NewLockMap(),
	}
	comp.SetName(compName)
	config.AddConfigChangeEventListener(comp)
	return comp
}

// On init register this component to pipeline and supply your constructor
func init() {
	internal.AddComponent(compName, NewFileCacheComponent)

	tmpPathFlag := config.AddStringFlag("tmp-path", "", "configures the tmp location for the cache. Configure the fastest disk (SSD or ramdisk) for best performance.")
	config.BindPFlag(compName+".path", tmpPathFlag)

	fileCacheTimeout := config.AddUint32Flag("file-cache-timeout", defaultFileCacheTimeout, "file cache timeout")
	config.BindPFlag(compName+".timeout-sec", fileCacheTimeout)

	fileCacheTimeoutSec := config.AddUint32Flag("file-cache-timeout-in-seconds", defaultFileCacheTimeout, "file cache timeout")
	config.BindPFlag(compName+".file-cache-timeout-in-seconds", fileCacheTimeoutSec)
	fileCacheTimeoutSec.Hidden = true

	cacheSizeMB := config.AddUint32Flag("cache-size-mb", 0, "max size in MB that file-cache can occupy on local disk for caching")
	config.BindPFlag(compName+".max-size-mb", cacheSizeMB)

	highThreshold := config.AddUint32Flag("high-disk-threshold", 90, "percentage of cache utilization which kicks in early eviction")
	config.BindPFlag(compName+".high-threshold", highThreshold)

	lowThreshold := config.AddUint32Flag("low-disk-threshold", 80, "percentage of cache utilization which stops early eviction started by high-disk-threshold")
	config.BindPFlag(compName+".low-threshold", lowThreshold)

	maxEviction := config.AddUint32Flag("max-eviction", 0, "Number of files to be evicted from cache at once.")
	config.BindPFlag(compName+".max-eviction", maxEviction)
	maxEviction.Hidden = true

	emptyDirCheck := config.AddBoolFlag("empty-dir-check", false, "Disallows remounting using a non-empty tmp-path.")
	config.BindPFlag(compName+".empty-dir-check", emptyDirCheck)
	emptyDirCheck.Hidden = true

	backgroundDownload := config.AddBoolFlag("background-download", false, "File download to run in the background on open call.")
	config.BindPFlag(compName+".background-download", backgroundDownload)
	backgroundDownload.Hidden = true

	cachePollTimeout := config.AddUint64Flag("cache-poll-timeout-msec", 0, "Time in milliseconds in order to poll for possible expired files awaiting cache eviction.")
	config.BindPFlag(compName+".cache-poll-timeout-msec", cachePollTimeout)
	cachePollTimeout.Hidden = true

	uploadModifiedOnly := config.AddBoolFlag("upload-modified-only", false, "Flag to turn off unnecessary uploads to storage.")
	config.BindPFlag(compName+".upload-modified-only", uploadModifiedOnly)
	uploadModifiedOnly.Hidden = true

	cachePolicy := config.AddStringFlag("file-cache-policy", "lru", "Cache eviction policy.")
	config.BindPFlag(compName+".policy", cachePolicy)
	cachePolicy.Hidden = true

	syncToFlush := config.AddBoolFlag("sync-to-flush", false, "Sync call on file will force a upload of the file.")
	config.BindPFlag(compName+".sync-to-flush", syncToFlush)

	ignoreSync := config.AddBoolFlag("ignore-sync", false, "Just ignore sync call and do not invalidate locally cached file.")
	config.BindPFlag(compName+".ignore-sync", ignoreSync)

	hardLimit := config.AddBoolFlag("hard-limit", false, "File cache limits are hard limits or not.")
	config.BindPFlag(compName+".hard-limit", hardLimit)

	config.RegisterFlagCompletionFunc("tmp-path", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return nil, cobra.ShellCompDirectiveDefault
	})
}
