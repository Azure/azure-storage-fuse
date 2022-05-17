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

package file_cache

import (
	"blobfuse2/common"
	"blobfuse2/common/log"
	"bytes"
	"os"
	"os/exec"
	"strconv"
	"strings"
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

	CacheValid(name string) error      // Mark the file as hit
	CacheInvalidate(name string) error // Invalidate the file
	CachePurge(name string) error      // Schedule the file for deletion

	IsCached(name string) bool // Whether or not the cache policy considers this file cached

	Name() string // The name of the policy
}

// getUsage: The current cache usage in MB
func getUsage(path string) float64 {
	log.Trace("cachePolicy::getCacheUsage : %s", path)

	var currSize float64
	var out bytes.Buffer

	// du - estimates file space usage
	// https://man7.org/linux/man-pages/man1/du.1.html
	// Note: We cannot just pass -BM as a parameter here since it will result in less accurate estimates of the size of the path
	// (i.e. du will round up to 1M if the path is smaller than 1M).
	cmd := exec.Command("du", "-sh", path)
	cmd.Stdout = &out

	err := cmd.Run()
	if err != nil {
		log.Err("cachePolicy::getCacheUsage : error running du [%s]", err.Error())
		return 0
	}

	size := strings.Split(out.String(), "\t")[0]
	if size == "0" {
		return 0
	}

	parsed, err := strconv.ParseFloat(size[:len(size)-1], 64)
	if err != nil {
		log.Err("cachePolicy::getCacheUsage : error parsing folder size [%s]", err.Error())
		return 0
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
	return currSize
}

// getUsagePercentage:  The current cache usage as a percentage of the maxSize
func getUsagePercentage(path string, maxSize float64) float64 {
	if maxSize == 0 {
		return 0
	}

	currSize := getUsage(path)
	usagePercent := (currSize / float64(maxSize)) * 100
	log.Debug("cachePolicy::getUsagePercentage : current cache usage : %f%%", usagePercent)

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
