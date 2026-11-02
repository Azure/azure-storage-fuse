/*
    _____           _____   _____   ____          ______  _____  ------
   |     |  |      |     | |     | |     |     | |       |            |
   |     |  |      |     | |     | |     |     | |       |            |
   | --- |  |      |     | |-----| |---- |     | |-----| |-----  ------
   |     |  |      |     | |     | |     |     |       | |       |
   | ____|  |_____ | ____| | ____| |     |_____|  _____| |_____  |_____


   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2020-2026 Microsoft Corporation. All rights reserved.
   Author : <blobfusedev@microsoft.com>

   Permission is hereby granted, free of charge, to any person obtaining a copy
   of this software and associated documentation files (the "Software"), to deal
   in the Software without restriction, including without limitation the rights
   to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
   copies of the Software, and to permit persons to whom the Software is
   furnished to do so, subject to the following conditions:

   The above copyright notice and this permission notice shall be included in all
   copies or substantial portions of the Software.

   THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
   IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
   FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
   AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
   LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
   OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
   SOFTWARE
*/

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
