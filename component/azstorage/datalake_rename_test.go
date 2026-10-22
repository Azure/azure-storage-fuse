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
	"net/http"
	"net/http/httptest"
	"net/url"
	"path"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azdatalake/filesystem"
	"github.com/Azure/azure-storage-fuse/v2/internal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDatalakeRenameRequestPaths(t *testing.T) {
	tests := []struct {
		name   string
		prefix string
		source string
		target string
	}{
		{name: "root", source: "old", target: "new"},
		{name: "nested", source: "parent/old", target: "parent/new"},
		{name: "move", source: "parent/old", target: "other/new"},
		{name: "spaces", source: "parent dir/old name", target: "other dir/new name"},
		{name: "unicode", source: "Alcald\u00eda/\u092d\u093e\u0930\u0924.txt", target: "\u65b0/renamed.txt"},
		{name: "percent", source: "parent/100% done", target: "other/50% done"},
		{name: "literal escape", source: "parent/literal%2Fname", target: "other/literal%20name"},
		{name: "reserved characters", source: "parent/a?#&+.txt", target: "other/b#&+.txt"},
		{name: "prefix", prefix: "mounted/subdir", source: "parent/old", target: "other/new"},
		{name: "special prefix", prefix: "mounted dir/100%/\u65b0", source: "parent/old", target: "other/new"},
		{name: "deep path", source: strings.Repeat("dir/", 60) + "old", target: strings.Repeat("dir/", 60) + "new"},
	}

	for _, kind := range []string{"file", "directory"} {
		t.Run(kind, func(t *testing.T) {
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					source := &url.URL{Path: path.Join("/container", tt.prefix, tt.source)}
					target := &url.URL{Path: path.Join("/container", tt.prefix, tt.target)}
					requests := 0
					modified := time.Date(2026, time.October, 18, 0, 0, 0, 0, time.UTC)
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						requests++
						assert.Equal(t, http.MethodPut, r.Method)
						assert.Equal(t, source.EscapedPath(), r.Header.Get("x-ms-rename-source"))
						assert.Equal(t, target.EscapedPath(), r.URL.EscapedPath())
						assert.Equal(t, target.Path, r.URL.Path)
						w.Header().Set("Last-Modified", modified.Format(http.TimeFormat))
						w.Header().Set("ETag", `"renamed"`)
						w.WriteHeader(http.StatusCreated)
					}))
					defer server.Close()

					client, err := filesystem.NewClientWithNoCredential(strings.Replace(server.URL, "127.0.0.1", "localhost", 1)+"/container", nil)
					require.NoError(t, err)
					dl := &Datalake{
						AzStorageConnection: AzStorageConnection{Config: AzStorageConfig{prefixPath: tt.prefix}},
						Filesystem:          client,
					}
					if kind == "file" {
						attr := &internal.ObjAttr{}
						require.NoError(t, dl.RenameFile(tt.source, tt.target, attr))
						assert.True(t, modified.Equal(attr.Mtime))
						assert.True(t, modified.Equal(attr.Atime))
						assert.True(t, modified.Equal(attr.Ctime))
						assert.Equal(t, "renamed", attr.ETag)
					} else {
						require.NoError(t, dl.RenameDirectory(tt.source, tt.target))
					}
					assert.Equal(t, 1, requests)
				})
			}
		})
	}
}

func TestDatalakeRenameStorageErrors(t *testing.T) {
	for _, tt := range []struct {
		name      string
		directory bool
		code      string
		status    int
		expected  error
	}{
		{name: "missing file", code: "SourcePathNotFound", status: http.StatusNotFound, expected: syscall.ENOENT},
		{name: "missing directory", directory: true, code: "SourcePathNotFound", status: http.StatusNotFound, expected: syscall.ENOENT},
		{name: "directory too deep", directory: true, code: "PathIsTooDeep", status: http.StatusConflict, expected: syscall.ENAMETOOLONG},
	} {
		t.Run(tt.name, func(t *testing.T) {
			requests := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				assert.Equal(t, "/container/parent/source", r.Header.Get("x-ms-rename-source"))
				writeCreateFileStorageError(w, tt.status, tt.code)
			}))
			defer server.Close()
			client, err := filesystem.NewClientWithNoCredential(strings.Replace(server.URL, "127.0.0.1", "localhost", 1)+"/container", nil)
			require.NoError(t, err)
			dl := &Datalake{Filesystem: client}
			if tt.directory {
				err = dl.RenameDirectory("parent/source", "other/target")
			} else {
				err = dl.RenameFile("parent/source", "other/target", nil)
			}
			require.ErrorIs(t, err, tt.expected)
			assert.Equal(t, 1, requests)
		})
	}
}
