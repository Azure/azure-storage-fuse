/*
    _____           _____   _____   ____          ______  _____  ------
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
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"syscall"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/container"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azdatalake/filesystem"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type createFileStorage struct {
	t                 *testing.T
	exists            bool
	etag              string
	permissionFailure bool
	permissionCode    string
	replaceOnFailure  bool
	deleteFailure     bool
	missingETag       bool
	emptyETag         bool
	requests          []string
}

func (s *createFileStorage) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.requests = append(s.requests, r.Method)
	switch r.Method {
	case http.MethodPut:
		assert.Equal(s.t, "*", r.Header.Get("If-None-Match"))
		if s.exists {
			writeCreateFileStorageError(w, http.StatusPreconditionFailed, "ConditionNotMet")
			return
		}
		s.exists = true
		s.etag = `"created"`
		if !s.missingETag {
			if s.emptyETag {
				w.Header().Set("ETag", "")
			} else {
				w.Header().Set("ETag", s.etag)
			}
		}
		w.WriteHeader(http.StatusCreated)
	case http.MethodPatch:
		assert.Equal(s.t, "setAccessControl", r.URL.Query().Get("action"))
		assert.Equal(s.t, `"created"`, r.Header.Get("If-Match"))
		assert.Equal(s.t, "rw-------", r.Header.Get("x-ms-permissions"))
		if s.permissionFailure {
			if s.replaceOnFailure {
				s.etag = `"replaced"`
			}
			code := s.permissionCode
			if code == "" {
				code = "OutOfRangeInput"
			}
			writeCreateFileStorageError(w, http.StatusBadRequest, code)
			return
		}
		w.WriteHeader(http.StatusOK)
	case http.MethodDelete:
		assert.Equal(s.t, `"created"`, r.Header.Get("If-Match"))
		assert.Empty(s.t, r.Header.Get("x-ms-delete-snapshots"))
		if s.etag != r.Header.Get("If-Match") {
			writeCreateFileStorageError(w, http.StatusPreconditionFailed, "ConditionNotMet")
			return
		}
		if s.deleteFailure {
			writeCreateFileStorageError(w, http.StatusForbidden, "AuthorizationPermissionMismatch")
			return
		}
		s.exists = false
		w.WriteHeader(http.StatusAccepted)
	default:
		s.t.Errorf("unexpected storage request: %s %s", r.Method, r.URL)
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func writeCreateFileStorageError(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/xml")
	w.Header().Set("x-ms-error-code", code)
	w.WriteHeader(status)
	_, _ = fmt.Fprintf(w, "<Error><Code>%s</Code><Message>storage error</Message></Error>", code)
}

func newDatalakeCreateTest(t *testing.T, state *createFileStorage, prefix string) *Datalake {
	t.Helper()
	state.t = t
	server := httptest.NewServer(state)
	t.Cleanup(server.Close)

	blobClient, err := container.NewClientWithNoCredential(server.URL+"/container", nil)
	require.NoError(t, err)
	dfsClient, err := filesystem.NewClientWithNoCredential(server.URL+"/container", nil)
	require.NoError(t, err)

	return &Datalake{
		AzStorageConnection: AzStorageConnection{Config: AzStorageConfig{prefixPath: prefix}},
		Filesystem:          dfsClient,
		BlockBlob: BlockBlob{
			AzStorageConnection: AzStorageConnection{Config: AzStorageConfig{prefixPath: prefix}},
			Container:           blobClient,
		},
	}
}

func createPathOfLength(length int) string {
	var path strings.Builder
	for length-path.Len() > 255 {
		path.WriteString(strings.Repeat("a", 200))
		path.WriteByte('/')
	}
	path.WriteString(strings.Repeat("z", length-path.Len()))
	return path.String()
}

func TestBlockBlobCreateFileIfAbsentReturnsETag(t *testing.T) {
	state := &createFileStorage{}
	dl := newDatalakeCreateTest(t, state, "")

	etag, err := dl.BlockBlob.createFileIfAbsent("path")

	require.NoError(t, err)
	require.NotNil(t, etag)
	assert.Equal(t, `"created"`, string(*etag))
	assert.Equal(t, []string{http.MethodPut}, state.requests)
	assert.True(t, state.exists)
}

func TestBlockBlobCreateFileIfAbsentRequiresETag(t *testing.T) {
	tests := []struct {
		name    string
		missing bool
		empty   bool
	}{
		{name: "missing ETag header", missing: true},
		{name: "empty ETag header", empty: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &createFileStorage{missingETag: tt.missing, emptyETag: tt.empty}
			dl := newDatalakeCreateTest(t, state, "")

			etag, err := dl.BlockBlob.createFileIfAbsent("path")

			require.ErrorContains(t, err, "returned no ETag")
			assert.Nil(t, etag)
			assert.Equal(t, []string{http.MethodPut}, state.requests)
			assert.True(t, state.exists, "cannot safely delete without the created blob's ETag")
		})
	}
}

func TestDatalakeCreateFileFollowsServerPathValidation(t *testing.T) {
	tests := []struct {
		name          string
		prefix        string
		path          string
		serverRejects bool
		fullBytes     int
	}{
		{name: "below limit", path: createPathOfLength(1023), fullBytes: 1023},
		{name: "at limit", path: createPathOfLength(1024), fullBytes: 1024},
		{name: "above limit accepted by server", path: createPathOfLength(1025), fullBytes: 1025},
		{name: "above limit rejected by server", path: createPathOfLength(1025), serverRejects: true, fullBytes: 1025},
		{name: "second observed overlong path", path: createPathOfLength(1049), serverRejects: true, fullBytes: 1049},
		{name: "observed overlong path", path: createPathOfLength(1144), serverRejects: true, fullBytes: 1144},
		{name: "prefix at limit", prefix: "mount", path: createPathOfLength(1018), fullBytes: 1024},
		{name: "prefix above limit", prefix: "mount", path: createPathOfLength(1019), serverRejects: true, fullBytes: 1025},
		{name: "multibyte character within character limit", path: createPathOfLength(1023) + "é", fullBytes: 1025},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &createFileStorage{permissionFailure: tt.serverRejects}
			dl := newDatalakeCreateTest(t, state, tt.prefix)
			assert.Len(t, []byte(strings.TrimPrefix(tt.prefix+"/"+tt.path, "/")), tt.fullBytes)
			for _, component := range strings.Split(tt.path, "/") {
				assert.LessOrEqual(t, len(component), 255)
			}

			err := dl.CreateFile(tt.path, 0600)
			if tt.serverRejects {
				require.ErrorIs(t, err, syscall.EINVAL)
				assert.Equal(t, []string{http.MethodPut, http.MethodPatch, http.MethodDelete}, state.requests)
				assert.False(t, state.exists)
			} else {
				require.NoError(t, err)
				assert.Equal(t, []string{http.MethodPut, http.MethodPatch}, state.requests)
				assert.True(t, state.exists)
			}
		})
	}
}

func TestDatalakeCreateFilePermissionFailureRemovesOnlyCreatedBlob(t *testing.T) {
	for _, code := range []string{"InvalidUri", "OutOfRangeInput"} {
		t.Run(code, func(t *testing.T) {
			state := &createFileStorage{permissionFailure: true, permissionCode: code}
			dl := newDatalakeCreateTest(t, state, "")

			err := dl.CreateFile("path", 0600)

			require.ErrorIs(t, err, syscall.EINVAL)
			assert.ErrorContains(t, err, code)
			assert.Equal(t, []string{http.MethodPut, http.MethodPatch, http.MethodDelete}, state.requests)
			assert.False(t, state.exists)
		})
	}
}

func TestDatalakeCreateFileOtherPermissionFailureIsNotInvalidArgument(t *testing.T) {
	state := &createFileStorage{permissionFailure: true, permissionCode: "OtherValidationError"}
	dl := newDatalakeCreateTest(t, state, "")

	err := dl.CreateFile("path", 0600)

	require.ErrorContains(t, err, "OtherValidationError")
	assert.NotErrorIs(t, err, syscall.EINVAL)
	assert.Equal(t, []string{http.MethodPut, http.MethodPatch, http.MethodDelete}, state.requests)
	assert.False(t, state.exists)
}

func TestDatalakeCreateFileDoesNotOverwriteExistingBlob(t *testing.T) {
	state := &createFileStorage{exists: true, etag: `"existing"`}
	dl := newDatalakeCreateTest(t, state, "")

	err := dl.CreateFile("path", 0600)

	require.ErrorIs(t, err, syscall.EEXIST)
	assert.Equal(t, []string{http.MethodPut}, state.requests)
	assert.True(t, state.exists)
	assert.Equal(t, `"existing"`, state.etag)
}

func TestDatalakeCreateFileDoesNotDeleteReplacement(t *testing.T) {
	state := &createFileStorage{permissionFailure: true, replaceOnFailure: true}
	dl := newDatalakeCreateTest(t, state, "")

	err := dl.CreateFile("path", 0600)

	require.ErrorIs(t, err, syscall.EINVAL)
	require.ErrorContains(t, err, "failed to remove partially created file")
	assert.Equal(t, []string{http.MethodPut, http.MethodPatch, http.MethodDelete}, state.requests)
	assert.True(t, state.exists)
	assert.Equal(t, `"replaced"`, state.etag)
}

func TestDatalakeCreateFileReportsCleanupFailure(t *testing.T) {
	state := &createFileStorage{permissionFailure: true, deleteFailure: true}
	dl := newDatalakeCreateTest(t, state, "")

	err := dl.CreateFile("path", 0600)

	require.ErrorIs(t, err, syscall.EINVAL)
	require.ErrorContains(t, err, "OutOfRangeInput")
	assert.ErrorContains(t, err, "AuthorizationPermissionMismatch")
	assert.Equal(t, []string{http.MethodPut, http.MethodPatch, http.MethodDelete}, state.requests)
	assert.True(t, state.exists)
}

func TestDatalakeCreateFileRequiresETagForPermissionUpdate(t *testing.T) {
	state := &createFileStorage{missingETag: true}
	dl := newDatalakeCreateTest(t, state, "")

	err := dl.CreateFile("path", 0600)

	require.ErrorContains(t, err, "returned no ETag")
	assert.Equal(t, []string{http.MethodPut}, state.requests)
	assert.True(t, state.exists)
}
