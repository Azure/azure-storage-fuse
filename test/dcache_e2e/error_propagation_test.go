//go:build !unittest
// +build !unittest

// Copyright (c) 2026 Microsoft Corporation.
// Licensed under the MIT License.

package dcache_e2e

import (
	"bytes"
	"path"
	"strings"
	"testing"
	"time"
)

const errorPayloadSize = 1024 * 1024

// TestErrorPropagation_L2FailuresBypassToAzure verifies that malformed or
// unavailable L2 responses are not exposed to the caller. The default
// bypass-on-error policy must return the complete Azure payload and must not
// populate L2 without owning the miss lock.
func TestErrorPropagation_L2FailuresBypassToAzure(t *testing.T) {
	tests := []struct {
		name       string
		mode       string
		minElapsed time.Duration
	}{
		{name: "checksum mismatch", mode: "checksum-mismatch"},
		{name: "request timeout", mode: "request-timeout", minElapsed: 25 * time.Second},
		{name: "malformed protobuf", mode: "malformed-protobuf"},
		{name: "truncated payload", mode: "truncated-payload"},
		{name: "zero-byte success", mode: "zero-byte"},
		{name: "peer lock timeout", mode: "locked-timeout", minElapsed: 3 * time.Second},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			blobPath, payload := seedErrorPropagationBlob(t, test.mode)
			m := newFaultPodMounter(t, test.mode, false)

			start := time.Now()
			got := m.ReadFile(t, blobPath)
			elapsed := time.Since(start)

			if !bytes.Equal(got, payload) {
				t.Fatalf("%s: fallback data mismatch: got len=%d md5=%s, want len=%d md5=%s",
					test.mode, len(got), md5Sum(got), len(payload), md5Sum(payload))
			}
			if elapsed < test.minElapsed {
				t.Fatalf("%s: read returned in %s, want at least %s to prove the timeout path",
					test.mode, elapsed, test.minElapsed)
			}

			pod := m.resolvePod(t)
			azureGETs, err := grepAzureGETsInPod(pod, blobPath)
			if err != nil {
				t.Fatalf("%s: verify Azure fallback: %v", test.mode, err)
			}
			if azureGETs == 0 {
				t.Fatalf("%s: no Azure GET observed; read did not prove L2 bypass", test.mode)
			}
			assertFaultServerActivity(t, m, false)
		})
	}
}

// TestErrorPropagation_AzureFailureAfterOwnedL2Miss verifies the exact
// ErrNotFoundGotLock -> azstorage failure path. The Azure error must reach the
// FUSE caller and failed/partial storage data must never be uploaded to L2.
func TestErrorPropagation_AzureFailureAfterOwnedL2Miss(t *testing.T) {
	blobPath, _ := seedErrorPropagationBlob(t, "azure-failure")
	m := newFaultPodMounter(t, "miss-got-lock", true)
	pod := m.resolvePod(t)

	data, err := m.readFileFromPodWithPartialE(pod, blobPath)
	if err == nil {
		t.Fatalf("read unexpectedly succeeded with %d bytes while Azure data GETs were rejected", len(data))
	}
	if len(data) != 0 {
		t.Fatalf("failed Azure read returned %d bytes; want no success-shaped partial data", len(data))
	}
	if !strings.Contains(err.Error(), "Input/output error") &&
		!strings.Contains(err.Error(), "Remote I/O error") {
		t.Fatalf("Azure failure surfaced as an unexpected error: %v", err)
	}
	assertFaultServerActivity(t, m, true)
}

func seedErrorPropagationBlob(t *testing.T, mode string) (string, []byte) {
	t.Helper()
	blobPath := path.Join(
		"dcache_e2e_error_"+strings.ReplaceAll(mode, "-", "_")+"_"+randomTestDirName(t, 8),
		"payload.bin",
	)
	payload := generateRandomBytes(t, errorPayloadSize)
	uploadBlob(t, blobPath, payload)
	t.Cleanup(func() { deleteBlobBestEffort(t, blobPath) })
	return blobPath, payload
}
