//go:build integration && !fuse2

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
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"syscall"
	"testing"

	"github.com/Azure/azure-storage-fuse/v2/internal"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// pagingBackend is a pipeline component with count files in its root directory, which it lists in blocks of
// blockSize entries, the way a container is listed page by page
type pagingBackend struct {
	internal.BaseComponent
	count     int
	blockSize int
}

func pagingBackendFile(i int) string {
	return fmt.Sprintf("file%02d", i)
}

func (b *pagingBackend) StreamDir(options internal.StreamDirOptions) ([]*internal.ObjAttr, string, error) {
	first := 0
	if options.Token != "" {
		first, _ = strconv.Atoi(options.Token)
	}
	last := min(first+b.blockSize, b.count)

	var entries []*internal.ObjAttr
	for i := first; i < last; i++ {
		entries = append(entries, &internal.ObjAttr{Path: pagingBackendFile(i), Name: pagingBackendFile(i), Mode: 0644})
	}
	next := ""
	if last < b.count {
		next = strconv.Itoa(last)
	}
	return entries, next, nil
}

func (b *pagingBackend) GetAttr(options internal.GetAttrOptions) (*internal.ObjAttr, error) {
	if !strings.HasPrefix(options.Name, "file") {
		return nil, syscall.ENOENT
	}
	return &internal.ObjAttr{Path: options.Name, Name: options.Name, Mode: 0644}, nil
}

func (b *pagingBackend) files() []string {
	var names []string
	for i := range b.count {
		names = append(names, pagingBackendFile(i))
	}
	return names
}

func entryNamesOf(entries []os.DirEntry) []string {
	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names
}

// TestReaddirIntegration lists a directory that the backend lists in several blocks, through a live FUSE mount. The
// kernel list cache is off, so that every readdir reaches blobfuse2.
//
// Run with:
//
//	go test -tags "integration fuse3" -v ./component/libfuse/... -run TestReaddirIntegration
func TestReaddirIntegration(t *testing.T) {
	backend := &pagingBackend{count: 10, blockSize: 3}
	h := newIntegrationHarness(t, backend, "libfuse:\n  kernel-list-cache-expiration-sec: 0\n")
	h.start(t)
	defer h.stop(t)

	t.Run("complete listing", func(t *testing.T) {
		entries, err := os.ReadDir(h.MountDir)
		require.NoError(t, err)
		assert.Equal(t, backend.files(), entryNamesOf(entries))
	})

	t.Run("rewind before the listing is complete", func(t *testing.T) {
		dir, err := os.Open(h.MountDir)
		require.NoError(t, err)
		defer dir.Close()

		_, err = dir.ReadDir(1)
		require.NoError(t, err)
		_, err = dir.Seek(0, io.SeekStart)
		require.NoError(t, err)

		entries, err := dir.ReadDir(-1)
		require.NoError(t, err)
		assert.Equal(t, backend.files(), entryNamesOf(entries))
	})
}
