// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package monitor

import (
	_ "github.com/Azure/azure-storage-fuse/v2/tools/health-monitor/monitor/blobfuse_stats"
	_ "github.com/Azure/azure-storage-fuse/v2/tools/health-monitor/monitor/cpu_mem_profiler"
	_ "github.com/Azure/azure-storage-fuse/v2/tools/health-monitor/monitor/file_cache"
	_ "github.com/Azure/azure-storage-fuse/v2/tools/health-monitor/monitor/network_profiler"
)
