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

import "github.com/Azure/azure-storage-fuse/v2/internal"

// dirCacheKey is the handle value under which libfuse_opendir stores the dirChildCache of a directory handle
const dirCacheKey = "cache"

// dirChildCache holds the block of a directory listing that readdir serves a directory handle from. The listing is
// fetched block by block, and readdir offsets count its entries: the kernel asks for the entries from an offset on,
// and continues at offset i+1 after it consumed entry i.
type dirChildCache struct {
	start    uint64              // Offset of the first entry of children
	children []*internal.ObjAttr // Current block of the listing
	token    string              // Token to list the block after children, "" if children is the last block
	listed   bool                // Whether children holds a block, as opposed to the listing not being started
}

// listBlockFunc lists the block of a directory listing that token continues with ("" for the first block), which
// starts at offset. It returns the entries of the block and the token of the next one ("" after the last block).
type listBlockFunc func(token string, offset uint64) ([]*internal.ObjAttr, string, error)

// end returns the offset just past the current block
func (c *dirChildCache) end() uint64 {
	return c.start + uint64(len(c.children))
}

// entriesFrom returns the entries of the listing from offset on, up to the end of the block that holds offset. It
// lists blocks with list as needed, and returns no entries if offset is past the end of the listing.
func (c *dirChildCache) entriesFrom(offset uint64, list listBlockFunc) ([]*internal.ObjAttr, error) {
	// A block can only be listed with the token of the block before it, so an offset before the current block starts
	// the listing over: offset 0 is the first readdir or a rewinddir(), and seekdir() can go back to any offset.
	if offset == 0 || offset < c.start {
		*c = dirChildCache{}
	}

	for !c.listed || offset >= c.end() {
		if c.listed && c.token == "" {
			return nil, nil
		}

		children, token, err := list(c.token, c.end())
		if err != nil {
			return nil, err
		}

		// An empty block that does not advance the token would make the listing go on forever
		if len(children) == 0 && token == c.token {
			token = ""
		}

		c.start, c.children, c.token, c.listed = c.end(), children, token, true
	}

	return c.children[offset-c.start:], nil
}
