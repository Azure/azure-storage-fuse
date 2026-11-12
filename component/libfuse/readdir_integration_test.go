//go:build integration && !fuse2

// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

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
