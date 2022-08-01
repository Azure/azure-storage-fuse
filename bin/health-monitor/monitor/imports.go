package monitor

import (
	_ "github.com/Azure/azure-storage-fuse/v2/bin/health-monitor/monitor/blobfuse_stats"
	_ "github.com/Azure/azure-storage-fuse/v2/bin/health-monitor/monitor/cpu_profiler"
	_ "github.com/Azure/azure-storage-fuse/v2/bin/health-monitor/monitor/file_cache"
	_ "github.com/Azure/azure-storage-fuse/v2/bin/health-monitor/monitor/memory_profiler"
	_ "github.com/Azure/azure-storage-fuse/v2/bin/health-monitor/monitor/network_profiler"
)
