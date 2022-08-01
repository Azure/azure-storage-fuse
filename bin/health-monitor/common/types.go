package common

import "path/filepath"

const (
	Blobfuse_stats   = "blobfuse_stats"
	File_cache       = "file_cache"
	Cpu_profiler     = "cpu_profiler"
	Memory_profiler  = "memory_profiler"
	Network_profiler = "network_profiler"

	HealthMon = "healthmon"
)

var (
	Pid               string
	BfsPollInterval   int
	StatsPollinterval int
	NoBfsMon          bool
	NoCpuProf         bool
	NoMemProf         bool
	NoNetProf         bool
	NoFileCacheMon    bool
	TempCachePath     string
)

var DefaultWorkDir = "$HOME/.blobfuse2"
var DefaultLogFile = filepath.Join(DefaultWorkDir, "healthmon.log")

var TransferPipe = filepath.Join(DefaultWorkDir, "pipes/transferPipe")
var PollingPipe = filepath.Join(DefaultWorkDir, "pipes/pollPipe")
