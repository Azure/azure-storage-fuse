// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

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
