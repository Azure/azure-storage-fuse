// Copyright (c) 2026 Microsoft Corporation. All rights reserved.
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
)

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
	} {
		t.Run(tc.name, func(t *testing.T) {
			events := make(chan map[string]string, 1)
			receiver := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
			defer receiver.Close()
			session, err := telemetry.Start(telemetry.Options{
				ConnectionString: "InstrumentationKey=test;IngestionEndpoint=" + receiver.URL,
				HTTPClient:       receiver.Client(),
			})
			if err != nil || session == nil {
				t.Fatalf("telemetry start: %v", err)
			}
			defer session.Disable()
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
			if !errors.As(validationErr, &response) || response.ErrorCode != tc.selected {
				t.Fatalf("validation lost selected cause: %v", validationErr)
			}
			probe := "BlockBlob"
			if tc.account == "adls" && tc.dfsCode != "" {
				probe = "Datalake"
			}
			wantMessage := fmt.Sprintf("failed to authenticate azstorage credentials with error [%s::TestPipeline : [%s]]", probe, tc.selected)
			if tc.name == "block-special-message" {
				wantMessage = "failed to authenticate azstorage credentials with error [BlockBlob::TestPipeline : Detected FNS account being mounted as HNS]"
			}
			if validationErr.Error() != wantMessage {
				t.Fatalf("user-facing error changed: got=%q want=%q", validationErr, wantMessage)
			}
			session.Close()
			select {
			case got := <-events:
				if got["failure_category"] != tc.category || got["failure_stage"] != "storage_initialization" || got["outcome"] != "failure" {
					t.Fatalf("wrong selected failure metadata: %+v", got)
				}
				for _, value := range got {
					if strings.Contains(value, "private error fixture") || strings.Contains(value, server.URL) {
						t.Fatal("raw storage error/endpoint leaked into telemetry")
					}
				}
			default:
				t.Fatal("missing startup failure event")
			}
			if blobCalls.Load() != 1 || tc.account == "adls" && dfsCalls.Load() != 1 {
				t.Fatalf("validation made unexpected requests: dfs=%d blob=%d", dfsCalls.Load(), blobCalls.Load())
			}
		})
	}
}

func TestStorageValidationErrorPreservesNetworkCause(t *testing.T) {
	cause := &net.DNSError{Err: "private DNS fixture"}
	err := &storageValidationError{
		message: "existing validation message",
		cause:   &storageValidationError{message: "inner validation message", cause: cause},
	}
	var dns *net.DNSError
	if err.Error() != "existing validation message" || !errors.As(err, &dns) || dns != cause || !errors.Is(err, cause) {
		t.Fatal("validation wrapper changed message or hid its network cause")
	}
}
