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

package file_cache

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal/stats_manager"
)

const DefaultEvictTime = 10

type cachePolicyConfig struct {
	tmpPath      string
	cacheTimeout uint32
	maxEviction  uint32

	maxSizeMB     float64
	highThreshold float64
	lowThreshold  float64

	fileLocks *common.LockMap

	// Held for writing while empty directories are removed from the cache. Nil means no locking.
	dirLock *sync.RWMutex

	policyTrace bool
}

type cachePolicy interface {
	StartPolicy() error
	ShutdownPolicy() error

	UpdateConfig(cachePolicyConfig) error

	CacheValid(name string)      // Mark the file as hit
	CacheInvalidate(name string) // Invalidate the file
	CachePurge(name string)      // Schedule the file for deletion

	IsCached(name string) bool // Whether or not the cache policy considers this file cached

	Name() string // The name of the policy
}

// getUsagePercentage:  The current cache usage as a percentage of the maxSize
func getUsagePercentage(path string, maxSizeMB float64) float64 {
	var curSize float64
	var usagePercent float64
	var err error

	if maxSizeMB <= 0 {
		// This should not happen as we validate config during startup. But log and return 0 to avoid division by zero
		// Even if the user doesn't set max size, it will be set to the default value
		log.Crit("cachePolicy::getUsagePercentage : invalid max size configured %f MB", maxSizeMB)
		return 0
	}

	// We need to compute % usage of temp directory against configured limit
	curSize, err = common.GetUsage(path)
	if err != nil {
		log.Err("cachePolicy::getUsagePercentage : failed to get directory usage for %s [%v]", path, err)
	}

	usagePercent = (curSize / maxSizeMB) * 100

	log.Debug("cachePolicy::getUsagePercentage : current cache usage : %f%%", usagePercent)

	fileCacheStatsCollector.UpdateStats(stats_manager.Replace, cacheUsage, fmt.Sprintf("%f MB", curSize))
	fileCacheStatsCollector.UpdateStats(stats_manager.Replace, usgPer, fmt.Sprintf("%f%%", usagePercent))

	return usagePercent
}

// Delete a given file
func deleteFile(name string) error {
	log.Debug("cachePolicy::deleteFile : attempting to delete %s", name)

	err := os.Remove(name)
	if err != nil && os.IsPermission(err) {
		// File is not having delete permissions so change the mode and retry deletion
		log.Warn("cachePolicy::deleteFile : failed to delete %s due to permission", name)

		err = os.Chmod(name, os.FileMode(0666))
		if err != nil {
			log.Err("cachePolicy::deleteFile : %s failed to reset permissions", name)
			return err
		}

		err = os.Remove(name)
	} else if err != nil && os.IsNotExist(err) {
		log.Debug("cachePolicy::deleteFile : %s does not exist in local cache", name)
		return nil
	}

	if err != nil {
		log.Err("cachePolicy::DeleteItem : Failed to delete local file %s [%v]", name, err.Error())
		return err
	}

	return nil
}

// removeEmptyParents removes the directories above name, a path in the cache directory tmpPath, that are empty. It
// stops at the first directory that is not empty and never removes tmpPath or anything outside it. Both paths are
// cleaned first, so ".." and repeated separators cannot reach outside the cache. Callers hold dirLock for writing,
// see FileCache.createLocalFile.
func removeEmptyParents(tmpPath string, name string) {
	root := filepath.Clean(tmpPath)
	for dir := filepath.Dir(filepath.Clean(name)); isBelow(root, dir); dir = filepath.Dir(dir) {
		// rmdir fails on a directory that is not empty, so a directory is never removed with anything in it.
		err := syscall.Rmdir(dir)
		if err == nil || err == syscall.ENOENT {
			continue
		}

		if err != syscall.ENOTEMPTY && err != syscall.EEXIST {
			log.Debug("cachePolicy::removeEmptyParents : failed to remove %s [%s]", dir, err.Error())
		}
		return
	}
}

// isBelow reports whether path is inside the directory root, and is not root itself. Both must be clean.
func isBelow(root string, path string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
