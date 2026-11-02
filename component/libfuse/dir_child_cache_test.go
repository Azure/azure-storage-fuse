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
	"strconv"
	"testing"

	"github.com/Azure/azure-storage-fuse/v2/internal"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// pagedListing lists a directory of count entries in blocks of blockSize entries, the way StreamDir does
type pagedListing struct {
	count     int
	blockSize int
	calls     int // Number of blocks listed
}

func entryName(i int) string {
	return fmt.Sprintf("entry%02d", i)
}

func (p *pagedListing) list(token string, _ uint64) ([]*internal.ObjAttr, string, error) {
	p.calls++
	first := 0
	if token != "" {
		first, _ = strconv.Atoi(token)
	}
	last := min(first+p.blockSize, p.count)

	var entries []*internal.ObjAttr
	for i := first; i < last; i++ {
		entries = append(entries, &internal.ObjAttr{Name: entryName(i)})
	}
	next := ""
	if last < p.count {
		next = strconv.Itoa(last)
	}
	return entries, next, nil
}

func entryNames(entries []*internal.ObjAttr) []string {
	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name)
	}
	return names
}

func entryNameRange(first, last int) []string {
	var names []string
	for i := first; i < last; i++ {
		names = append(names, entryName(i))
	}
	return names
}

// readFrom reads the listing from offset to its end the way the kernel does: each readdir consumes at most perReaddir
// entries, as if the reply buffer had no room for more, and the next readdir continues after the last consumed entry
func readFrom(t *testing.T, cache *dirChildCache, list listBlockFunc, offset uint64, perReaddir int) []string {
	var names []string
	for {
		entries, err := cache.entriesFrom(offset, list)
		require.NoError(t, err)
		if len(entries) == 0 {
			return names
		}
		entries = entries[:min(len(entries), perReaddir)]
		names = append(names, entryNames(entries)...)
		offset += uint64(len(entries))
	}
}

func TestDirChildCacheSequential(t *testing.T) {
	for _, perReaddir := range []int{1, 2, 3, 100} {
		t.Run(fmt.Sprintf("%d per readdir", perReaddir), func(t *testing.T) {
			listing := &pagedListing{count: 10, blockSize: 3}
			names := readFrom(t, &dirChildCache{}, listing.list, 0, perReaddir)
			assert.Equal(t, entryNameRange(0, 10), names)
			assert.Equal(t, 4, listing.calls, "each block is listed once")
		})
	}
}

func TestDirChildCacheEmptyDir(t *testing.T) {
	listing := &pagedListing{count: 0, blockSize: 3}
	cache := &dirChildCache{}

	assert.Empty(t, readFrom(t, cache, listing.list, 0, 100))
	assert.Equal(t, 1, listing.calls)

	// Past the end of a listing that is complete, nothing is listed again
	entries, err := cache.entriesFrom(5, listing.list)
	require.NoError(t, err)
	assert.Empty(t, entries)
	assert.Equal(t, 1, listing.calls)
}

// TestDirChildCacheRewind verifies that rewinddir() in the middle of a listing starts it over from its first entry,
// rather than listing the next block with the token of the current one
func TestDirChildCacheRewind(t *testing.T) {
	listing := &pagedListing{count: 10, blockSize: 3}
	cache := &dirChildCache{}

	entries, err := cache.entriesFrom(0, listing.list)
	require.NoError(t, err)
	require.Equal(t, entryNameRange(0, 3), entryNames(entries))
	entries, err = cache.entriesFrom(3, listing.list)
	require.NoError(t, err)
	require.Equal(t, entryNameRange(3, 6), entryNames(entries))

	assert.Equal(t, entryNameRange(0, 10), readFrom(t, cache, listing.list, 0, 2))
}

// TestDirChildCacheSeekBack verifies that seekdir() to an offset before the current block continues the listing at
// that offset, rather than ending it
func TestDirChildCacheSeekBack(t *testing.T) {
	listing := &pagedListing{count: 10, blockSize: 3}
	cache := &dirChildCache{}

	entries, err := cache.entriesFrom(7, listing.list)
	require.NoError(t, err)
	require.Equal(t, entryNameRange(7, 9), entryNames(entries))

	assert.Equal(t, entryNameRange(4, 10), readFrom(t, cache, listing.list, 4, 2))
}

// TestDirChildCacheSeekForward verifies that seekdir() past the current block continues the listing at that offset
func TestDirChildCacheSeekForward(t *testing.T) {
	listing := &pagedListing{count: 10, blockSize: 3}
	cache := &dirChildCache{}

	entries, err := cache.entriesFrom(0, listing.list)
	require.NoError(t, err)
	require.Equal(t, entryNameRange(0, 3), entryNames(entries))

	assert.Equal(t, entryNameRange(8, 10), readFrom(t, cache, listing.list, 8, 100))
	assert.Equal(t, 4, listing.calls)
}

// TestDirChildCacheEmptyBlock verifies that a block without entries does not end the listing while there are more
// blocks, and that an empty block that does not advance the token does end it
func TestDirChildCacheEmptyBlock(t *testing.T) {
	blocks := map[string]struct {
		names []string
		next  string
	}{
		"":       {nil, "second"},
		"second": {[]string{"a", "b"}, "third"},
		"third":  {nil, "third"},
	}
	list := func(token string, _ uint64) ([]*internal.ObjAttr, string, error) {
		var entries []*internal.ObjAttr
		for _, name := range blocks[token].names {
			entries = append(entries, &internal.ObjAttr{Name: name})
		}
		return entries, blocks[token].next, nil
	}

	assert.Equal(t, []string{"a", "b"}, readFrom(t, &dirChildCache{}, list, 0, 100))
}

// TestDirChildCacheListError verifies that an error listing a block is returned, and that the block is listed again on
// the next readdir
func TestDirChildCacheListError(t *testing.T) {
	listing := &pagedListing{count: 10, blockSize: 3}
	cache := &dirChildCache{}
	failing := func(string, uint64) ([]*internal.ObjAttr, string, error) {
		return nil, "", errors.New("listing failed")
	}

	_, err := cache.entriesFrom(0, failing)
	require.Error(t, err)
	assert.Equal(t, entryNameRange(0, 10), readFrom(t, cache, listing.list, 0, 100))

	_, err = cache.entriesFrom(3, listing.list)
	require.NoError(t, err)
	_, err = cache.entriesFrom(6, failing)
	require.Error(t, err)
	assert.Equal(t, entryNameRange(6, 10), readFrom(t, cache, listing.list, 6, 100))
}
