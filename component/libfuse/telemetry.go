// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package libfuse

import (
	"strconv"

	"github.com/Azure/azure-storage-fuse/v2/internal/telemetry"
)

func fuseTelemetryReady(major int, runtimeVersion string) {
	props := map[string]string{
		"fuse_api_major":          strconv.Itoa(major),
		"libfuse_runtime_version": runtimeVersion,
	}
	telemetry.Ready(props)
}
