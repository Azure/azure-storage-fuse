//go:build !authtest
// +build !authtest

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

package azstorage

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"syscall"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/container"
	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal"
	"github.com/Azure/azure-storage-fuse/v2/internal/handlemap"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeBlob serves ranged GETs for a single blob whose content can be swapped at runtime,
// mimicking the service behaviour of returning a shorter 206 when the range exceeds the blob size.
type fakeBlob struct {
	mu      sync.Mutex
	content []byte
}

func (f *fakeBlob) set(content []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.content = content
}

func (f *fakeBlob) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	content := f.content
	f.mu.Unlock()

	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	size := int64(len(content))
	start, end := int64(0), size-1
	rng := r.Header.Get("x-ms-range")
	if rng == "" {
		rng = r.Header.Get("Range")
	}
	if rng != "" {
		spec := strings.TrimPrefix(rng, "bytes=")
		parts := strings.SplitN(spec, "-", 2)
		_, _ = fmt.Sscanf(parts[0], "%d", &start)
		if len(parts) == 2 && parts[1] != "" {
			_, _ = fmt.Sscanf(parts[1], "%d", &end)
		}
	}

	if start >= size {
		w.Header().Set("x-ms-error-code", "InvalidRange")
		w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
		return
	}
	end = min(end, size-1)

	w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, size))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", end-start+1))
	w.Header().Set("ETag", `"0x8DEADBEEF"`)
	w.WriteHeader(http.StatusPartialContent)
	_, _ = w.Write(content[start : end+1])
}

func newFakeBlobStorage(t *testing.T, content []byte) (*AzStorage, *BlockBlob, *fakeBlob) {
	t.Helper()
	require.NoError(t, log.SetDefaultLogger("silent", common.LogConfig{Level: common.ELogLevel.LOG_DEBUG()}))

	blob := &fakeBlob{content: content}
	srv := httptest.NewServer(blob)
	t.Cleanup(srv.Close)

	cli, err := container.NewClientWithNoCredential(srv.URL+"/account/container", &container.ClientOptions{
		ClientOptions: azcore.ClientOptions{Retry: policy.RetryOptions{MaxRetries: -1}},
	})
	require.NoError(t, err)

	bb := &BlockBlob{Container: cli}
	return &AzStorage{storage: bb}, bb, blob
}

func TestReadInBufferStaleSizeDoesNotLeakBuffer(t *testing.T) {
	az, _, blob := newFakeBlobStorage(t, bytes.Repeat([]byte("B"), 1024*1024))

	h := handlemap.NewHandle("f3target")
	h.Size = 1024 * 1024

	// blob is shrunk after the handle captured its size
	blob.set(bytes.Repeat([]byte("B"), 100))

	// simulate residue from a previously serviced request in the caller-owned buffer
	data := bytes.Repeat([]byte("S"), 128*1024)
	n, err := az.ReadInBuffer(&internal.ReadInBufferOptions{Handle: h, Offset: 0, Data: data})
	require.NoError(t, err)
	assert.Equal(t, 100, n)
	assert.Equal(t, bytes.Repeat([]byte("B"), 100), data[:n])

	// offset beyond the new end of blob
	n, err = az.ReadInBuffer(&internal.ReadInBufferOptions{Handle: h, Offset: 128 * 1024, Data: data})
	assert.Equal(t, syscall.ERANGE, err)
	assert.Equal(t, 0, n)
}

func TestReadInBufferStaleSizeWithoutHandle(t *testing.T) {
	az, _, _ := newFakeBlobStorage(t, bytes.Repeat([]byte("B"), 300))

	data := bytes.Repeat([]byte("S"), 256)
	n, err := az.ReadInBuffer(&internal.ReadInBufferOptions{Path: "f", Size: 1024, Offset: 200, Data: data})
	require.NoError(t, err)
	assert.Equal(t, 100, n)
	assert.Equal(t, bytes.Repeat([]byte("B"), 100), data[:n])
}

func TestBlockBlobReadInBufferReturnsBytesRead(t *testing.T) {
	_, bb, _ := newFakeBlobStorage(t, bytes.Repeat([]byte("B"), 1000))

	// full read of the requested range, buffer larger than range must be left untouched past length
	data := bytes.Repeat([]byte("S"), 500)
	var etag string
	n, err := bb.ReadInBuffer("f", 100, 200, data, &etag)
	require.NoError(t, err)
	assert.Equal(t, 200, n)
	assert.Equal(t, bytes.Repeat([]byte("B"), 200), data[:200])
	assert.Equal(t, bytes.Repeat([]byte("S"), 300), data[200:])
	assert.NotEmpty(t, etag)

	// short read when the range exceeds the blob
	data = bytes.Repeat([]byte("S"), 500)
	n, err = bb.ReadInBuffer("f", 900, 500, data, nil)
	require.NoError(t, err)
	assert.Equal(t, 100, n)

	// zero length reads till the end of blob, bounded by the buffer
	data = bytes.Repeat([]byte("S"), 500)
	n, err = bb.ReadInBuffer("f", 800, 0, data, nil)
	require.NoError(t, err)
	assert.Equal(t, 200, n)

	// requested length larger than the buffer is rejected
	n, err = bb.ReadInBuffer("f", 0, 600, data, nil)
	assert.Equal(t, syscall.EINVAL, err)
	assert.Equal(t, 0, n)

	n, err = bb.ReadInBuffer("f", 0, -1, data, nil)
	assert.Equal(t, syscall.EINVAL, err)
	assert.Equal(t, 0, n)
}

func TestBlockBlobReadRangeInBufferShortRead(t *testing.T) {
	_, bb, _ := newFakeBlobStorage(t, bytes.Repeat([]byte("B"), 1000))

	data := make([]byte, 500)
	require.NoError(t, bb.readRangeInBuffer("f", 500, 500, data))

	err := bb.readRangeInBuffer("f", 900, 500, data)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "short read")
}
