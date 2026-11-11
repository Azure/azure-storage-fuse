// Copyright (c) 2026 Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package libfuse

import (
	"fmt"
	"strconv"

	"github.com/Azure/azure-storage-fuse/v2/internal/telemetry"
)

var telemetryOperations = []string{
	"statfs", "mkdir", "rmdir", "opendir", "releasedir", "create", "open", "read",
	"write", "flush", "release", "unlink", "symlink", "readlink", "fsync", "fsyncdir",
	"getattr", "readdir", "truncate", "rename", "chmod", "chown", "utimens",
}

func fuseTelemetryReady(major int, runtimeVersion string, protoMajor, protoMinor int, counts func(int, int) uint64) {
	props := map[string]string{
		"fuse_api_major":          strconv.Itoa(major),
		"libfuse_runtime_version": runtimeVersion,
		"kernel_fuse_protocol":    fmt.Sprintf("%d.%d", protoMajor, protoMinor),
	}
	telemetry.Ready(props, func() map[string]float64 {
		result := map[string]float64{}
		for i, op := range telemetryOperations {
			total := uint64(0)
			for outcome := 0; outcome < 135; outcome++ {
				count := counts(i, outcome)
				total += count
				if count != 0 && outcome != 0 {
					name := strconv.Itoa(outcome)
					switch outcome {
					case 5:
						name = "EIO"
					case 13:
						name = "EACCES"
					case 28:
						name = "ENOSPC"
					case 116:
						name = "ESTALE"
					case 134:
						name = "other"
					}
					result["fuse_"+op+"_errno_"+name] = float64(count)
				}
			}
			if total != 0 {
				result["fuse_"+op+"_completed"] = float64(total)
			}
		}
		return result
	})
}
