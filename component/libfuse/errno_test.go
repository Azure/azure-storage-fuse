// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package libfuse

import (
	"errors"
	"fmt"
	"io/fs"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestErrnoOf(t *testing.T) {
	reported := []syscall.Errno{syscall.ENOENT, syscall.EACCES, syscall.EEXIST, syscall.EINVAL}
	for _, tc := range []struct {
		name string
		err  error
		want syscall.Errno
	}{
		{"errno", syscall.EINVAL, syscall.EINVAL},
		{"path error", &fs.PathError{Op: "open", Path: "file", Err: syscall.ENOENT}, syscall.ENOENT},
		{"io/fs error", fs.ErrNotExist, syscall.ENOENT},
		{"wrapped", fmt.Errorf("listing failed: %w", syscall.EACCES), syscall.EACCES},
		{"joined", errors.Join(syscall.EACCES, errors.New("cleanup failed")), syscall.EACCES},
		{"EPERM counts as EACCES", syscall.EPERM, syscall.EACCES},
		{"ENOTEMPTY counts as EEXIST", syscall.ENOTEMPTY, syscall.EEXIST},
		{"not reported", syscall.ENAMETOOLONG, syscall.EIO},
		{"unknown", errors.New("failed"), syscall.EIO},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, errnoOf(tc.err, reported...))
		})
	}
}

// TestErrnoOfOrder verifies that an error matching more than one reported errno is reported as the first of them
func TestErrnoOfOrder(t *testing.T) {
	assert.Equal(t, syscall.EEXIST, errnoOf(syscall.ENOTEMPTY, syscall.EEXIST, syscall.ENOTEMPTY))
	assert.Equal(t, syscall.ENOTEMPTY, errnoOf(syscall.ENOTEMPTY, syscall.ENOTEMPTY, syscall.EEXIST))
	assert.Equal(t, syscall.EIO, errnoOf(syscall.ENOENT))
}
