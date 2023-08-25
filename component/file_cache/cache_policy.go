/*
    _____           _____   _____   ____          ______  _____  ------
   |     |  |      |     | |     | |     |     | |       |            |
   |     |  |      |     | |     | |     |     | |       |            |
   | --- |  |      |     | |-----| |---- |     | |-----| |-----  ------
   |     |  |      |     | |     | |     |     |       | |       |
   | ____|  |_____ | ____| | ____| |     |_____|  _____| |_____  |_____


   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2020-2023 Microsoft Corporation. All rights reserved.
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
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
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

var duPath []string = []string{"/usr/bin/du", "/usr/local/bin/du", "/usr/sbin/du", "/usr/local/sbin/du", "/sbin/du", "/bin/du"}
var selectedDuPath string = ""

// getUsage: The current cache usage in MB
func getUsage(path string) (float64, error) {
	log.Trace("cachePolicy::getCacheUsage : %s", path)

	var currSize float64
	var out bytes.Buffer

	if selectedDuPath == "" {
		selectedDuPath = "-"
		for _, dup := range duPath {
			_, err := os.Stat(dup)
			if err == nil {
				selectedDuPath = dup
				break
			}
		}
	}

	if selectedDuPath == "-" {
		log.Err("cachePolicy::getCacheUsage : error finding du in any configured path")
		return 0, fmt.Errorf("failed to find du")
	}

	// du - estimates file space usage
	// https://man7.org/linux/man-pages/man1/du.1.html
	// Note: We cannot just pass -BM as a parameter here since it will result in less accurate estimates of the size of the path
	// (i.e. du will round up to 1M if the path is smaller than 1M).
	cmd := exec.Command(selectedDuPath, "-sh", path)
	cmd.Stdout = &out

	err := cmd.Run()
	if err != nil {
		log.Err("cachePolicy::getCacheUsage : error running du [%s]", err.Error())
		return 0, err
	}

	size := strings.Split(out.String(), "\t")[0]
	if size == "0" {
		return 0, fmt.Errorf("failed to parse du output")
	}

	// some OS's use "," instead of "." that will not work for float parsing - replace it
	size = strings.Replace(size, ",", ".", 1)
	parsed, err := strconv.ParseFloat(size[:len(size)-1], 64)
	if err != nil {
		log.Err("cachePolicy::getCacheUsage : error parsing folder size [%s]", err.Error())
		return 0, fmt.Errorf("failed to parse du output")
	}

	switch size[len(size)-1] {
	case 'K':
		currSize = parsed / float64(1024)
	case 'M':
		currSize = parsed
	case 'G':
		currSize = parsed * 1024
	case 'T':
		currSize = parsed * 1024 * 1024
	}

	log.Debug("cachePolicy::getCacheUsage : current cache usage : %fMB", currSize)
	return currSize, nil
}

var currentUID int = -1

// getDiskUsageFromStatfs: Current disk usage of temp path
func getDiskUsageFromStatfs(path string) (float64, float64) {
	// We need to compute the disk usage percentage for the temp path
	var stat syscall.Statfs_t
	err := syscall.Statfs(path, &stat)
	if err != nil {
		log.Err("cachePolicy::getUsagePercentage : error getting statfs [%s]", err.Error())
		return 0, 0
	}

	if currentUID == -1 {
		currentUID = os.Getuid()
	}

	var availableSpace uint64 = 0
	if currentUID == 0 {
		// Sudo  has mounted
		availableSpace = stat.Bfree * uint64(stat.Frsize)
	} else {
		// non Sudo has mounted
		availableSpace = stat.Bavail * uint64(stat.Frsize)
	}

	totalSpace := stat.Blocks * uint64(stat.Frsize)
	usedSpace := float64(totalSpace - availableSpace)
	return usedSpace, float64(usedSpace) / float64(totalSpace) * 100
}

// getUsagePercentage:  The current cache usage as a percentage of the maxSize
func getUsagePercentage(path string, maxSize float64) float64 {
	var currSize float64 = 0
	var usagePercent float64 = 0

	if maxSize == 0 {
		currSize, usagePercent = getDiskUsageFromStatfs(path)
	} else {
		// We need to compuate % usage of temp directory against configured limit
		currSize, _ = getUsage(path)
		usagePercent = (currSize / float64(maxSize)) * 100
	}

	log.Debug("cachePolicy::getUsagePercentage : current cache usage : %f%%", usagePercent)

	fileCacheStatsCollector.UpdateStats(stats_manager.Replace, cacheUsage, fmt.Sprintf("%f MB", currSize))
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
		log.Err("lruPolicy::DeleteItem : Failed to delete local file %s", name)
		return err
	}

	return nil
}
