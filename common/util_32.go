//go:build arm

// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package common

import (
	"math"
	"syscall"
)

func SetFrsize(st *syscall.Statfs_t, v uint64) {
	if v > uint64(math.MaxInt32) {
		// Clamp to MaxInt32 to avoid silent truncation.
		st.Frsize = int32(math.MaxInt32)
	} else {
		st.Frsize = int32(v)
	}
}
