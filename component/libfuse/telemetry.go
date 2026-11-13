// Copyright (c) 2026 Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package libfuse

import (
	"fmt"
	"strconv"

	"github.com/Azure/azure-storage-fuse/v2/internal/telemetry"
)

func fuseTelemetryReady(major int, runtimeVersion string, protoMajor, protoMinor int) {
	props := map[string]string{
		"fuse_api_major":          strconv.Itoa(major),
		"libfuse_runtime_version": runtimeVersion,
		"kernel_fuse_protocol":    fmt.Sprintf("%d.%d", protoMajor, protoMinor),
	}
	telemetry.Ready(props)
}
