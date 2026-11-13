//go:build !unittest

// Copyright (c) 2026 Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/internal/telemetry"
)

const maxItem = telemetry.MaxEventBytes

func isMounted(path string) (bool, error) {
	b, err := os.ReadFile("/proc/self/mountinfo")
	if err != nil {
		return false, errors.New("cannot inspect Linux mount state")
	}
	for _, line := range strings.Split(string(b), "\n") {
		fields := strings.Fields(line)
		if len(fields) > 4 && fields[4] == path {
			return true, nil
		}
	}
	return false, nil
}

func unmount(binary, path string, lazy bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	args := []string{"unmount", path}
	if lazy {
		args = append(args, "--lazy")
	}
	return exec.CommandContext(ctx, binary, args...).Run()
}
