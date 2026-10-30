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

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal/stats_manager"

	"golang.org/x/sys/unix"
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
// cleaned first, so ".." and repeated separators cannot reach outside the cache. The directories are opened one at a
// time from tmpPath without following symlinks, so a symlink in the cache cannot lead rmdir outside it; tmpPath itself
// may be a symlink. Callers hold dirLock for writing, see FileCache.createLocalFile.
func removeEmptyParents(tmpPath string, name string) {
	root := filepath.Clean(tmpPath)
	dir := filepath.Dir(filepath.Clean(name))
	if !isBelow(root, dir) {
		return
	}
	rel, err := filepath.Rel(root, dir)
	if err != nil {
		return
	}
	parts := strings.Split(rel, string(filepath.Separator))

	// fds[i] is the directory holding parts[i].
	rootFd, err := unix.Open(root, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		log.Debug("cachePolicy::removeEmptyParents : failed to open %s [%s]", root, err.Error())
		return
	}
	fds := []int{rootFd}
	defer func() {
		for _, fd := range fds {
			_ = unix.Close(fd)
		}
	}()

	for _, part := range parts[:len(parts)-1] {
		fd, err := unix.Openat(fds[len(fds)-1], part, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
		if err == unix.ENOENT {
			// Nothing below it exists, so start with this directory, which is already gone, and its parents.
			break
		}
		if err != nil {
			// ELOOP or ENOTDIR for a symlink or a file: what is below it is not in the cache.
			log.Debug("cachePolicy::removeEmptyParents : not removing directories in %s [%s]",
				filepath.Join(root, filepath.Join(parts[:len(fds)]...)), err.Error())
			return
		}
		fds = append(fds, fd)
	}

	for i := len(fds) - 1; i >= 0; i-- {
		// rmdir fails on a directory that is not empty, so a directory is never removed with anything in it. It does
		// not follow a symlink either.
		err := unix.Unlinkat(fds[i], parts[i], unix.AT_REMOVEDIR)
		if err == nil || err == unix.ENOENT {
			continue
		}

		if err != unix.ENOTEMPTY && err != unix.EEXIST {
			log.Debug("cachePolicy::removeEmptyParents : failed to remove %s [%s]",
				filepath.Join(root, filepath.Join(parts[:i+1]...)), err.Error())
		}
		return
	}
}

// isBelow reports whether path is inside the directory root, and is not root itself. Both must be clean.
func isBelow(root string, path string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
