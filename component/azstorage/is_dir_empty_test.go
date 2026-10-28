/*
   Licensed under the MIT License <http://opensource.org/licenses/MIT>.
   Copyright © 2026 Microsoft Corporation. All rights reserved.
   Author : <blobfusedev@microsoft.com>
*/

package azstorage

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/container"
	"github.com/Azure/azure-storage-fuse/v2/internal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsDirEmptyPagination(t *testing.T) {
	type page struct {
		nextMarker string
		child      bool
		fail       bool
	}
	tests := []struct {
		name  string
		pages []page
		empty bool
	}{
		{name: "empty directory", pages: []page{{}}, empty: true},
		{name: "child on first page", pages: []page{{child: true}}},
		{name: "stop at first child", pages: []page{{child: true, nextMarker: "more"}}},
		{name: "child after empty page", pages: []page{{nextMarker: "second"}, {child: true}}},
		{name: "child after multiple empty pages", pages: []page{{nextMarker: "second"}, {nextMarker: "third"}, {child: true}}},
		{name: "all pages empty", pages: []page{{nextMarker: "second"}, {}}, empty: true},
		{name: "first page error", pages: []page{{fail: true}}},
		{name: "error after empty page", pages: []page{{nextMarker: "second"}, {fail: true}}},
	}

	for _, backend := range []string{"blob", "adls"} {
		t.Run(backend, func(t *testing.T) {
			for _, name := range []string{"parent", "parent/", "", "/"} {
				t.Run("path="+name, func(t *testing.T) {
					for _, tt := range tests {
						t.Run(tt.name, func(t *testing.T) {
							requests := 0
							server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
								assert.Equal(t, http.MethodGet, r.Method)
								assert.Equal(t, "/container", r.URL.Path)
								assert.Equal(t, "list", r.URL.Query().Get("comp"))
								assert.Equal(t, "1", r.URL.Query().Get("maxresults"))
								assert.Equal(t, "mounted/"+formatListDirName(name), r.URL.Query().Get("prefix"))
								if requests >= len(tt.pages) {
									t.Error("unexpected extra list request")
									w.WriteHeader(http.StatusBadRequest)
									return
								}
								marker := ""
								if requests > 0 {
									marker = tt.pages[requests-1].nextMarker
								}
								assert.Equal(t, marker, r.URL.Query().Get("marker"))
								p := tt.pages[requests]
								requests++
								if p.fail {
									w.Header().Set("x-ms-error-code", "AuthorizationPermissionMismatch")
									w.WriteHeader(http.StatusForbidden)
									return
								}
								child := ""
								if p.child {
									child = `<Blob><Name>mounted/parent/child</Name><Properties><Last-Modified>Wed, 21 Oct 2026 00:00:00 GMT</Last-Modified><Etag>"child"</Etag><Content-Length>0</Content-Length><BlobType>BlockBlob</BlobType></Properties><Metadata><hdi_isfolder>true</hdi_isfolder></Metadata></Blob>`
								}
								w.Header().Set("Content-Type", "application/xml")
								_, _ = fmt.Fprintf(w, `<EnumerationResults><Blobs>%s</Blobs><NextMarker>%s</NextMarker></EnumerationResults>`, child, p.nextMarker)
							}))
							defer server.Close()

							client, err := container.NewClientWithNoCredential(server.URL+"/container", &container.ClientOptions{
								ClientOptions: azcore.ClientOptions{Retry: policy.RetryOptions{MaxRetries: -1}},
							})
							require.NoError(t, err)
							bb := BlockBlob{
								AzStorageConnection: AzStorageConnection{Config: AzStorageConfig{prefixPath: "mounted"}},
								Container:           client,
								listDetails:         container.ListBlobsInclude{Metadata: true},
							}
							az := &AzStorage{storage: &bb}
							if backend == "adls" {
								az.storage = &Datalake{BlockBlob: bb}
							}

							assert.Equal(t, tt.empty, az.IsDirEmpty(internal.IsDirEmptyOptions{Name: name}))
							assert.Equal(t, len(tt.pages), requests)
						})
					}
				})
			}
		})
	}
}
