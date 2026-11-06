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

package loopback

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/internal"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInjectFault(t *testing.T) {
	dir := t.TempDir()
	lfs := newTestLoopback(t, dir)
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "dir"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "dir/a.bin"), []byte("data"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "dir/b.txt"), []byte("data"), 0o644))

	remove, err := lfs.InjectFault(Fault{Op: OpGetAttr, Path: "/dir/*.bin", Err: syscall.EIO})
	require.NoError(t, err)

	_, err = lfs.GetAttr(internal.GetAttrOptions{Name: "dir/a.bin"})
	assert.Equal(t, syscall.EIO, err)
	_, err = lfs.GetAttr(internal.GetAttrOptions{Name: "/dir/a.bin"})
	assert.Equal(t, syscall.EIO, err, "names are matched without their leading slash")
	_, err = lfs.GetAttr(internal.GetAttrOptions{Name: "dir/b.txt"})
	assert.NoError(t, err, "other objects are not affected")
	_, err = lfs.OpenFile(internal.OpenFileOptions{Name: "dir/a.bin"})
	assert.NoError(t, err, "other operations are not affected")

	remove()
	_, err = lfs.GetAttr(internal.GetAttrOptions{Name: "dir/a.bin"})
	assert.NoError(t, err)
}

func TestFaultSkipAndCount(t *testing.T) {
	dir := t.TempDir()
	lfs := newTestLoopback(t, dir)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "file"), []byte("data"), 0o644))

	_, err := lfs.InjectFault(Fault{Op: OpReadInBuffer, Err: syscall.ETIMEDOUT, Skip: 1, Count: 2})
	require.NoError(t, err)

	var got []error
	for range 5 {
		_, err := lfs.ReadInBuffer(&internal.ReadInBufferOptions{Path: "file", Size: 4, Data: make([]byte, 4)})
		got = append(got, err)
	}
	assert.Equal(t, []error{nil, syscall.ETIMEDOUT, syscall.ETIMEDOUT, nil, nil}, got)
}

// A failed call must not change anything, so that the caller can retry it.
func TestFaultHasNoSideEffects(t *testing.T) {
	dir := t.TempDir()
	lfs := newTestLoopback(t, dir)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "file"), []byte("data"), 0o644))

	_, err := lfs.InjectFault(Fault{Op: OpCreateFile, Err: syscall.ENOSPC})
	require.NoError(t, err)
	_, err = lfs.InjectFault(Fault{Op: OpDeleteFile, Err: syscall.EACCES})
	require.NoError(t, err)
	remove, err := lfs.InjectFault(Fault{Op: OpCommitData, Path: "blob", Err: syscall.EIO, Count: 1})
	require.NoError(t, err)
	defer remove()

	_, err = lfs.CreateFile(internal.CreateFileOptions{Name: "new", Mode: 0o644})
	assert.Equal(t, syscall.ENOSPC, err)
	assert.NoFileExists(t, filepath.Join(dir, "new"))

	assert.Equal(t, syscall.EACCES, lfs.DeleteFile(internal.DeleteFileOptions{Name: "file"}))
	assert.FileExists(t, filepath.Join(dir, "file"))

	id := blockID(1)
	require.NoError(t, lfs.StageData(internal.StageDataOptions{Name: "blob", Id: id, Data: []byte("data")}))
	assert.Equal(t, syscall.EIO, lfs.CommitData(internal.CommitDataOptions{Name: "blob", List: []string{id}}))
	assert.NoFileExists(t, filepath.Join(dir, "blob"))

	// The staged block is still there for the retry.
	require.NoError(t, lfs.CommitData(internal.CommitDataOptions{Name: "blob", List: []string{id}}))
	data, err := os.ReadFile(filepath.Join(dir, "blob"))
	require.NoError(t, err)
	assert.Equal(t, []byte("data"), data)
}

func TestFaultDelay(t *testing.T) {
	lfs := newTestLoopback(t, t.TempDir())

	const delay = 50 * time.Millisecond
	_, err := lfs.InjectFault(Fault{Op: OpStreamDir, Delay: delay})
	require.NoError(t, err)

	start := time.Now()
	_, _, err = lfs.StreamDir(internal.StreamDirOptions{Name: ""})
	assert.NoError(t, err, "a fault without an error only delays the call")
	assert.GreaterOrEqual(t, time.Since(start), delay)
}

func TestFaultAllOperations(t *testing.T) {
	dir := t.TempDir()
	lfs := newTestLoopback(t, dir)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "file"), []byte("data"), 0o644))
	h, err := lfs.OpenFile(internal.OpenFileOptions{Name: "file"})
	require.NoError(t, err)

	_, err = lfs.InjectFault(Fault{Op: OpAny, Err: syscall.EACCES})
	require.NoError(t, err)

	_, err = lfs.GetAttr(internal.GetAttrOptions{Name: "file"})
	assert.Equal(t, syscall.EACCES, err)
	_, _, err = lfs.StreamDir(internal.StreamDirOptions{Name: ""})
	assert.Equal(t, syscall.EACCES, err)
	assert.Equal(t, syscall.EACCES, lfs.CreateDir(internal.CreateDirOptions{Name: "dir", Mode: 0o755}))
	assert.Equal(t, syscall.EACCES, lfs.ReleaseFile(internal.ReleaseFileOptions{Handle: h}))
	assert.Equal(t, syscall.EACCES, lfs.FlushFile(internal.FlushFileOptions{Handle: h}))
	assert.False(t, lfs.IsDirEmpty(internal.IsDirEmptyOptions{Name: "dir"}), "IsDirEmpty reports false when it fails")

	lfs.ClearFaults()
	_, err = lfs.GetAttr(internal.GetAttrOptions{Name: "file"})
	assert.NoError(t, err)
}

func TestFaultValidation(t *testing.T) {
	lfs := newTestLoopback(t, t.TempDir())

	for _, f := range []Fault{
		{Op: "Bogus", Err: syscall.EIO},
		{Path: "[", Err: syscall.EIO},
		{Op: OpGetAttr},
		{Err: syscall.EIO, Skip: -1},
		{Delay: -time.Second},
	} {
		remove, err := lfs.InjectFault(f)
		assert.Error(t, err, "%+v", f)
		assert.Nil(t, remove, "%+v", f)
	}
}

func TestConfiguredFaults(t *testing.T) {
	dir := t.TempDir()
	lfs := newTestLoopback(t, dir,
		"faults:",
		"  - op: GetAttr",
		"    path: configured",
		"    error: enoent",
		"  - op: StreamDir",
		"    delay-ms: 10",
		"    skip: 1",
		"    count: 1",
	)
	for _, name := range []string{"configured", "injected"} {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte("data"), 0o644))
	}

	_, err := lfs.InjectFault(Fault{Op: OpGetAttr, Path: "injected", Err: syscall.EIO})
	require.NoError(t, err)

	_, err = lfs.GetAttr(internal.GetAttrOptions{Name: "configured"})
	assert.Equal(t, syscall.ENOENT, err)
	_, err = lfs.GetAttr(internal.GetAttrOptions{Name: "injected"})
	assert.Equal(t, syscall.EIO, err)

	// ClearFaults removes only the injected faults.
	lfs.ClearFaults()
	_, err = lfs.GetAttr(internal.GetAttrOptions{Name: "injected"})
	assert.NoError(t, err)
	_, err = lfs.GetAttr(internal.GetAttrOptions{Name: "configured"})
	assert.Equal(t, syscall.ENOENT, err)

	lfs.faults.mu.Lock()
	streamDir := lfs.faults.configured[1].fault
	lfs.faults.mu.Unlock()
	assert.Equal(t, Fault{Op: OpStreamDir, Delay: 10 * time.Millisecond, Skip: 1, Count: 1}, streamDir)
}

func TestParseErrno(t *testing.T) {
	testCases := map[string]syscall.Errno{
		"EIO":         syscall.EIO,
		"eio":         syscall.EIO,
		" ENOENT ":    syscall.ENOENT,
		"ETIMEDOUT":   syscall.ETIMEDOUT,
		"ENOSPC":      syscall.ENOSPC,
		"ENOTSUP":     syscall.ENOTSUP,
		"EOPNOTSUPP":  syscall.EOPNOTSUPP,
		"EWOULDBLOCK": syscall.EWOULDBLOCK,
		"EAGAIN":      syscall.EAGAIN,
	}

	for name, want := range testCases {
		got, err := parseErrno(name)
		require.NoError(t, err, name)
		assert.Equal(t, want, got, name)
	}

	for _, name := range []string{"", "EBOGUS", "5"} {
		_, err := parseErrno(name)
		assert.Error(t, err, name)
	}
}
