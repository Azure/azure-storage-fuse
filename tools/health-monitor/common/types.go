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

package common

import (
	"path/filepath"
)

const (
	BlobfuseStats     = "blobfuse_stats"
	FileCacheMon      = "file_cache_monitor"
	CpuProfiler       = "cpu_profiler"
	MemoryProfiler    = "memory_profiler"
	CpuMemoryProfiler = "cpu_mem_profiler"
	NetworkProfiler   = "network_profiler"

	BfuseMon = "bfusemon"

	OutputFileName      = "monitor"
	OutputFileExtension = "json"
	OutputFileCount     = 10
	OutputFileSizeinMB  = 10
	PolicyStats         = "policy-stats"
	PolicyPollInterval  = 30
)

var (
	Pid             string
	BfsPollInterval int
	ProcMonInterval int

	NoBfsMon       bool
	NoCpuProf      bool
	NoMemProf      bool
	NoNetProf      bool
	NoFileCacheMon bool

	TempCachePath string
	MaxCacheSize  float64
	OutputPath    string

	CheckVersion bool
)

const BfuseMonitorVersion = "1.0.0-preview.1"

var DefaultWorkDir = "$HOME/.blobfuse2"
var DefaultLogFile = filepath.Join(DefaultWorkDir, "bfuseMonitor.log")

type CacheEvent struct {
	CacheEvent      string            `json:"cacheEvent"`
	Path            string            `json:"path"`
	IsDir           bool              `json:"isDir"`
	CacheSize       int64             `json:"cacheSize"`
	CacheConsumed   string            `json:"cacheConsumed"`
	CacheFilesCnt   int64             `json:"cacheFilesCount"`
	EvictedFilesCnt int64             `json:"evictedFilesCount"`
	Value           map[string]string `json:"value"`
}

type CpuMemStat struct {
	CpuUsage string
	MemUsage string
}
