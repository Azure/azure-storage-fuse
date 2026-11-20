// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package azstorage

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-storage-fuse/v2/internal/telemetry"
	"github.com/Azure/azure-storage-fuse/v2/internal/telemetry/telemetrytest"
	"github.com/stretchr/testify/require"
)

func TestTelemetryStorageIdentity(t *testing.T) {
	for _, tc := range []struct{ endpoint, kind, name string }{
		{"https://fixtureaccount.blob.core.windows.net/path?token=private", "public", "fixtureaccount"},
		{"https://fixtureaccount.privatelink.dfs.core.windows.net", "private_link", "fixtureaccount"},
		{"https://fixtureaccount.z12.blob.storage.azure.net", "public", "fixtureaccount"},
		{"https://anotheraccount.blob.core.windows.net", "public", ""},
		{"https://fixtureaccount.blob.core.windows.net.example.invalid", "other", ""},
		{"https://fixtureaccount.blob.core.usgovcloudapi.net", "other", ""},
		{"https://127.0.0.1:10000", "other", ""},
	} {
		kind, name := storageTelemetryIdentity(tc.endpoint, "fixtureaccount")
		require.Equal(t, tc.kind, kind, tc.endpoint)
		require.Equal(t, tc.name, name, tc.endpoint)
	}
}

func TestTelemetryValidationUsesSelectedFailure(t *testing.T) {
	for _, tc := range []struct {
		name, account, dfsCode, blobCode, delayed, selected, category string
	}{
		{"dfs-auth-blob-finishes-last", "adls", "AuthenticationFailed", "ContainerNotFound", "blob", "AuthenticationFailed", "authentication"},
		{"dfs-auth-dfs-finishes-last", "adls", "AuthenticationFailed", "ContainerNotFound", "dfs", "AuthenticationFailed", "authentication"},
		{"dfs-unknown-beats-blob-auth", "adls", "InvalidQueryParameterValue", "AuthenticationFailed", "blob", "InvalidQueryParameterValue", "unknown"},
		{"blob-only-failure", "adls", "", "AuthorizationPermissionMismatch", "", "AuthorizationPermissionMismatch", "authorization"},
		{"dfs-only-failure", "adls", "FilesystemNotFound", "", "", "FilesystemNotFound", "container_not_found"},
		{"block-auth", "block", "", "AuthenticationFailed", "", "AuthenticationFailed", "authentication"},
		{"block-special-message", "block", "", "InvalidQueryParameterValue", "", "InvalidQueryParameterValue", "unknown"},
		{"wrapped-network", "", "", "", "", "", "dns_resolution"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			events := make(chan map[string]string, 1)
			receiver := telemetrytest.NewReceiver(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var batch []struct {
					Data struct {
						Base struct {
							Name       string            `json:"name"`
							Properties map[string]string `json:"properties"`
						} `json:"baseData"`
					} `json:"data"`
				}
				if err := json.NewDecoder(r.Body).Decode(&batch); err != nil {
					t.Error(err)
					w.WriteHeader(400)
					return
				}
				for _, item := range batch {
					if item.Data.Base.Name == "blobfuse.mount.result" {
						events <- item.Data.Base.Properties
					}
				}
			}))
			session, err := telemetry.Start(telemetry.Options{
				ConnectionString: receiver.ConnectionString,
				HTTPClient:       receiver.Client,
			})
			require.NoError(t, err)
			require.NotNil(t, session)
			defer session.Close()
			if tc.name == "wrapped-network" {
				telemetry.Stage("storage_initialization")
				cause := &net.DNSError{Err: "private DNS fixture"}
				err := &storageValidationError{message: "existing validation message",
					cause: &storageValidationError{message: "inner validation message", cause: cause}}
				var dns *net.DNSError
				if err.Error() != "existing validation message" || !errors.As(err, &dns) || dns != cause || !errors.Is(err, cause) {
					t.Fatal("validation wrapper changed message or hid its network cause")
				}
				observeStartupFailure(err)
			} else {
				var dfsCalls, blobCalls, started atomic.Int32
				bothStarted := make(chan struct{})
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					isDFS := r.URL.Query().Get("resource") == "filesystem"
					code, probe := tc.blobCode, "blob"
					if isDFS {
						dfsCalls.Add(1)
						code, probe = tc.dfsCode, "dfs"
					} else {
						blobCalls.Add(1)
					}
					if tc.account == "adls" {
						if started.Add(1) == 2 {
							close(bothStarted)
						}
						select {
						case <-bothStarted:
						case <-time.After(2 * time.Second):
							t.Error("ADLS validation probes did not overlap")
						}
					}
					if tc.delayed == probe {
						time.Sleep(50 * time.Millisecond)
					}
					if code != "" {
						status := http.StatusForbidden
						if strings.Contains(code, "NotFound") {
							status = http.StatusNotFound
						} else if code == "InvalidQueryParameterValue" {
							status = http.StatusBadRequest
						}
						w.Header().Set("Content-Type", "application/xml")
						w.Header().Set("x-ms-error-code", code)
						w.WriteHeader(status)
						fmt.Fprintf(w, "<Error><Code>%s</Code><Message>private error fixture</Message></Error>", code)
					} else if isDFS {
						w.Header().Set("Content-Type", "application/json")
						fmt.Fprint(w, `{"paths":[]}`)
					} else {
						w.Header().Set("Content-Type", "application/xml")
						fmt.Fprint(w, "<EnumerationResults><Blobs/><NextMarker/></EnumerationResults>")
					}
				}))
				defer server.Close()
				skip := false
				_, validationErr := configureValidationTest(t, server.URL, tc.account, &skip)
				var response *azcore.ResponseError
				require.ErrorAs(t, validationErr, &response)
				require.Equal(t, tc.selected, response.ErrorCode)
				probe := "BlockBlob"
				if tc.account == "adls" && tc.dfsCode != "" {
					probe = "Datalake"
				}
				wantMessage := fmt.Sprintf("failed to authenticate azstorage credentials with error [%s::TestPipeline : [%s]]", probe, tc.selected)
				if tc.name == "block-special-message" {
					wantMessage = "failed to authenticate azstorage credentials with error [BlockBlob::TestPipeline : Detected FNS account being mounted as HNS]"
				}
				require.EqualError(t, validationErr, wantMessage)
				require.EqualValues(t, 1, blobCalls.Load())
				if tc.account == "adls" {
					require.EqualValues(t, 1, dfsCalls.Load())
				}
			}
			session.Close()
			require.NotEmpty(t, events, "missing startup failure event")
			got := <-events
			require.Equal(t, tc.category, got["error_category"])
			require.Equal(t, "storage_initialization", got["failure_stage"])
			for _, value := range got {
				require.NotContains(t, value, "private")
				require.NotContains(t, value, "127.0.0.1")
			}
		})
	}
}
