//go:build amd64 || arm64

// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package common

import "syscall"

func SetFrsize(st *syscall.Statfs_t, v uint64) {
	st.Frsize = int64(v)
}
