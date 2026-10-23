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
	"encoding/base64"
	"fmt"
	"io"
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
// It also serves a committed block list, set separately from the content so that it can describe
// a blob that changed after its block list was fetched, and records the uploads it receives.
type fakeBlob struct {
	mu        sync.Mutex
	content   []byte
	blocks    []int64 // sizes of the committed blocks
	cutBodies int     // number of next downloads whose body is cut off halfway
	hangUps   int     // number of downloads after those that the server hangs up on before it responds
	chunked   bool    // send downloads without a Content-Length, as after the transport decompresses them
	uploads   []fakeUpload
}

type fakeUpload struct {
	comp string // "block" for Put Block, "blocklist" for Put Block List, empty for Put Blob
	body []byte
}

func (f *fakeBlob) set(content []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.content = content
}

func (f *fakeBlob) setBlocks(sizes ...int64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.blocks = sizes
}

func (f *fakeBlob) cutNextBodies(count int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cutBodies = count
}

func (f *fakeBlob) setChunked(chunked bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.chunked = chunked
}

func (f *fakeBlob) hangUpNext(count int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.hangUps = count
}

// takeUploads returns the uploads received since the last call
func (f *fakeBlob) takeUploads() []fakeUpload {
	f.mu.Lock()
	defer f.mu.Unlock()
	uploads := f.uploads
	f.uploads = nil
	return uploads
}

func (f *fakeBlob) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == http.MethodPut:
		body, _ := io.ReadAll(r.Body)
		f.mu.Lock()
		f.uploads = append(f.uploads, fakeUpload{comp: r.URL.Query().Get("comp"), body: body})
		f.mu.Unlock()
		w.Header().Set("ETag", `"0x8DEADBEEF"`)
		w.WriteHeader(http.StatusCreated)
	case r.Method == http.MethodGet && r.URL.Query().Get("comp") == "blocklist":
		f.serveBlockList(w)
	case r.Method == http.MethodGet:
		f.serveRange(w, r)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (f *fakeBlob) serveBlockList(w http.ResponseWriter) {
	f.mu.Lock()
	blocks := f.blocks
	f.mu.Unlock()

	var list strings.Builder
	list.WriteString(`<?xml version="1.0" encoding="utf-8"?><BlockList><CommittedBlocks>`)
	for i, size := range blocks {
		name := base64.StdEncoding.EncodeToString(fmt.Appendf(nil, "block-%010d", i))
		fmt.Fprintf(&list, "<Block><Name>%s</Name><Size>%d</Size></Block>", name, size)
	}
	list.WriteString("</CommittedBlocks><UncommittedBlocks /></BlockList>")

	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, list.String())
}

func (f *fakeBlob) serveRange(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	content := f.content
	cut := f.cutBodies > 0
	hangUp := false
	if cut {
		f.cutBodies--
	} else if f.hangUps > 0 {
		f.hangUps--
		hangUp = true
	}
	chunked := f.chunked
	f.mu.Unlock()

	if hangUp {
		// The connection closes before the server sends a response
		if conn, _, err := w.(http.Hijacker).Hijack(); err == nil {
			_ = conn.Close()
		}
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

	body := content[start : end+1]
	w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, size))
	if !chunked {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(body)))
	}
	w.Header().Set("ETag", `"0x8DEADBEEF"`)
	w.WriteHeader(http.StatusPartialContent)
	if chunked {
		// Send the headers before the body, so that the server sends it in chunks instead of adding a Content-Length
		w.(http.Flusher).Flush()
	}
	if !cut {
		_, _ = w.Write(body)
		return
	}

	// The connection drops after half of the body, before the end of the range that the headers promise
	_, _ = w.Write(body[:len(body)/2])
	w.(http.Flusher).Flush()
	if conn, _, err := w.(http.Hijacker).Hijack(); err == nil {
		_ = conn.Close()
	}
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

// blockListWithPartialLastBlock : block list of a 10 byte blob made of an 8 byte block and a partial 2 byte block
func blockListWithPartialLastBlock() *common.BlockOffsetList {
	return &common.BlockOffsetList{
		BlockIdLength: 16,
		BlockList: []*common.Block{
			{StartIndex: 0, EndIndex: 8, Id: common.GetBlockID(16)},
			{StartIndex: 8, EndIndex: 10, Id: common.GetBlockID(16)},
		},
	}
}

func TestCreateNewBlocksTruncatePartialLastBlock(t *testing.T) {
	_, bb, _ := newFakeBlobStorage(t, []byte("0123456789"))

	// Extending past the last block fills it up to the block size and appends a new block for the rest
	blockList := blockListWithPartialLastBlock()
	oldID := blockList.BlockList[1].Id
	err := bb.createNewBlocksTruncate(blockList, &internal.TruncateFileOptions{Name: "f", OldSize: 10, NewSize: 20, BlockSize: 8})
	require.NoError(t, err)
	require.Len(t, blockList.BlockList, 3)

	last := blockList.BlockList[1]
	assert.EqualValues(t, 8, last.StartIndex)
	assert.EqualValues(t, 16, last.EndIndex)
	assert.Equal(t, append([]byte("89"), make([]byte, 6)...), last.Data)
	assert.True(t, last.Dirty())
	assert.NotEqual(t, oldID, last.Id)

	assert.EqualValues(t, 16, blockList.BlockList[2].StartIndex)
	assert.EqualValues(t, 20, blockList.BlockList[2].EndIndex)
	assert.True(t, blockList.IsBlockListModified())

	// Extending within the last block only grows that block
	blockList = blockListWithPartialLastBlock()
	err = bb.createNewBlocksTruncate(blockList, &internal.TruncateFileOptions{Name: "f", OldSize: 10, NewSize: 12, BlockSize: 8})
	require.NoError(t, err)
	require.Len(t, blockList.BlockList, 2)
	assert.EqualValues(t, 12, blockList.BlockList[1].EndIndex)
	assert.Equal(t, []byte("89\x00\x00"), blockList.BlockList[1].Data)
}

func TestCreateNewBlocksTruncateBlobShrunk(t *testing.T) {
	// The blob lost its last byte after the block list was fetched, so the existing data of the last block is short
	_, bb, _ := newFakeBlobStorage(t, []byte("012345678"))

	blockList := blockListWithPartialLastBlock()
	err := bb.createNewBlocksTruncate(blockList, &internal.TruncateFileOptions{Name: "f", OldSize: 10, NewSize: 20, BlockSize: 8})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "short read")
}

func TestReadInBufferCutOffBody(t *testing.T) {
	content := make([]byte, 1000)
	for i := range content {
		content[i] = byte(i)
	}
	for _, chunked := range []bool{false, true} {
		t.Run(fmt.Sprintf("chunked=%t", chunked), func(t *testing.T) {
			az, bb, blob := newFakeBlobStorage(t, content)
			blob.setChunked(chunked)
			if chunked {
				// The body comes without a Content-Length, as when the transport decompresses a gzip encoded blob
				req, err := http.NewRequest(http.MethodGet, bb.Container.URL()+"/f", nil)
				require.NoError(t, err)
				req.Header.Set("x-ms-range", "bytes=0-99")
				resp, err := http.DefaultClient.Do(req)
				require.NoError(t, err)
				_, _ = io.Copy(io.Discard, resp.Body)
				_ = resp.Body.Close()
				require.EqualValues(t, -1, resp.ContentLength)
			}

			// A body that ends cleanly before the end of the range is a shorter blob
			data := make([]byte, 500)
			n, err := bb.ReadInBuffer("f", 900, 500, data, nil)
			require.NoError(t, err)
			assert.Equal(t, 100, n)
			assert.Equal(t, content[900:], data[:n])

			// The reader downloads the rest of a body that was cut off once
			blob.cutNextBodies(1)
			n, err = bb.ReadInBuffer("f", 100, 500, data, nil)
			require.NoError(t, err)
			assert.Equal(t, 500, n)
			assert.Equal(t, content[100:600], data)

			// A body that is cut off fails the read when the request for the rest fails, here because the server hangs
			// up, with an error that wraps io.EOF
			blob.cutNextBodies(1)
			blob.hangUpNext(100)
			n, err = bb.ReadInBuffer("f", 100, 500, data, nil)
			require.Error(t, err)
			assert.Equal(t, 0, n)
			blob.hangUpNext(0)

			// A body that is cut off on every retry fails the read, also when the range goes past the end of the blob,
			// instead of returning the part that was received as if the blob were shorter
			for _, offset := range []int64{100, 900} {
				blob.cutNextBodies(100)
				n, err = bb.ReadInBuffer("f", offset, 500, make([]byte, 500), nil)
				require.ErrorIs(t, err, io.ErrUnexpectedEOF)
				assert.Equal(t, 0, n)
			}

			blob.cutNextBodies(100)
			h := handlemap.NewHandle("f")
			h.Size = int64(len(content))
			n, err = az.ReadInBuffer(&internal.ReadInBufferOptions{Handle: h, Offset: 0, Data: data})
			require.ErrorIs(t, err, io.ErrUnexpectedEOF)
			assert.Equal(t, 0, n)
			blob.cutNextBodies(0)
		})
	}
}

// shortBodyTransport answers with a body that ends cleanly before the length that its Content-Length promises.
// The HTTP transport reports such a body as io.ErrUnexpectedEOF, so this checks the Content-Length check on its own.
type shortBodyTransport struct{}

func (shortBodyTransport) Do(req *http.Request) (*http.Response, error) {
	header := http.Header{}
	header.Set("Content-Length", "500")
	header.Set("Content-Range", "bytes 0-499/1000")
	header.Set("ETag", `"0x8DEADBEEF"`)
	return &http.Response{
		StatusCode:    http.StatusPartialContent,
		Header:        header,
		ContentLength: 500,
		Body:          io.NopCloser(bytes.NewReader(make([]byte, 250))),
		Request:       req,
	}, nil
}

func TestReadInBufferBodyShorterThanContentLength(t *testing.T) {
	require.NoError(t, log.SetDefaultLogger("silent", common.LogConfig{Level: common.ELogLevel.LOG_DEBUG()}))
	cli, err := container.NewClientWithNoCredential("https://account.blob.core.windows.net/container", &container.ClientOptions{
		ClientOptions: azcore.ClientOptions{Transport: shortBodyTransport{}, Retry: policy.RetryOptions{MaxRetries: -1}},
	})
	require.NoError(t, err)
	bb := &BlockBlob{Container: cli}

	n, err := bb.ReadInBuffer("f", 0, 500, make([]byte, 500), nil)
	require.ErrorIs(t, err, io.ErrUnexpectedEOF)
	assert.Equal(t, 0, n)
}

func TestReadInBufferZeroLengthRequestsBufferSize(t *testing.T) {
	var ranges []string
	_, bb, blob := newFakeBlobStorage(t, bytes.Repeat([]byte("B"), 1000))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ranges = append(ranges, r.Header.Get("x-ms-range"))
		blob.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)
	cli, err := container.NewClientWithNoCredential(srv.URL+"/account/container", &container.ClientOptions{
		ClientOptions: azcore.ClientOptions{Retry: policy.RetryOptions{MaxRetries: -1}},
	})
	require.NoError(t, err)
	bb.Container = cli

	n, err := bb.ReadInBuffer("f", 100, 0, make([]byte, 300), nil)
	require.NoError(t, err)
	assert.Equal(t, 300, n)

	// An empty buffer reads nothing, without a request for the rest of the blob
	n, err = bb.ReadInBuffer("f", 100, 0, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, 0, n)

	assert.Equal(t, []string{"bytes=100-399"}, ranges)
}

func TestWriteBlobShorterThanBlockList(t *testing.T) {
	content := []byte("0123456789abcdef")
	_, bb, blob := newFakeBlobStorage(t, content)
	blob.setBlocks(8, 8)
	options := &internal.WriteFileOptions{Handle: handlemap.NewHandle("f"), Offset: 12, Data: []byte("XY")}

	// Overwriting the middle of the second block uploads it with the rest of its existing data
	require.NoError(t, bb.Write(options))
	uploads := blob.takeUploads()
	require.Len(t, uploads, 2)
	assert.Equal(t, fakeUpload{comp: "block", body: []byte("89abXYef")}, uploads[0])
	assert.Equal(t, "blocklist", uploads[1].comp)

	// The blob lost its last 6 bytes after its block list was fetched, so the second block can not be read in full
	blob.set(content[:10])
	err := bb.Write(options)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "short read")
	assert.Empty(t, blob.takeUploads())
}

func TestTruncateBlobShorterThanCachedSize(t *testing.T) {
	content := []byte("0123456789abcdef")
	for _, tc := range []struct {
		name    string
		blocks  []int64
		options internal.TruncateFileOptions
		uploads []fakeUpload
	}{
		{
			// removeBlocksTruncate keeps the start of the block that the new size falls in
			name:    "shrink into a block",
			blocks:  []int64{8, 8},
			options: internal.TruncateFileOptions{Name: "f", OldSize: 16, NewSize: 12, BlockSize: 8},
			uploads: []fakeUpload{{comp: "block", body: []byte("89ab")}, {comp: "blocklist"}},
		},
		{
			// TruncateFileWithoutBlocks uploads the whole blob again
			name:    "shrink without blocks",
			options: internal.TruncateFileOptions{Name: "f", OldSize: 16, NewSize: 12},
			uploads: []fakeUpload{{body: []byte("0123456789ab")}},
		},
		{
			name:    "extend without blocks",
			options: internal.TruncateFileOptions{Name: "f", OldSize: 16, NewSize: 20},
			uploads: []fakeUpload{{body: append([]byte("0123456789abcdef"), 0, 0, 0, 0)}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, bb, blob := newFakeBlobStorage(t, content)
			blob.setBlocks(tc.blocks...)

			require.NoError(t, bb.TruncateFile(tc.options))
			uploads := blob.takeUploads()
			require.Len(t, uploads, len(tc.uploads))
			for i, upload := range tc.uploads {
				assert.Equal(t, upload.comp, uploads[i].comp)
				if upload.body != nil {
					assert.Equal(t, upload.body, uploads[i].body)
				}
			}

			// The blob lost its last 6 bytes after its size was cached, so the data to keep can not be read in full
			blob.set(content[:10])
			err := bb.TruncateFile(tc.options)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "short read")
			assert.Empty(t, blob.takeUploads())
		})
	}
}
