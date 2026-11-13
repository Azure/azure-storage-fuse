//go:build !unittest

// Copyright (c) 2026 Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package loopback_e2e

import (
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestOrdinaryLoopbackMountDisablesTelemetry(t *testing.T) {
	var requests atomic.Int64
	receiver := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.WriteHeader(200)
	}))
	defer receiver.Close()
	cert := filepath.Join(t.TempDir(), "local-ca.pem")
	if err := os.WriteFile(cert, pem.EncodeToMemory(&pem.Block{
		Type: "CERTIFICATE", Bytes: receiver.Certificate().Raw,
	}), 0600); err != nil {
		t.Fatal(err)
	}
	// Shared launcher opt-out must beat an inherited enable and
	// destination, without ever contacting the compiled or configured endpoint.
	t.Setenv("BLOBFUSE_DISABLE_TELEMETRY", "false")
	t.Setenv("BLOBFUSE_TELEMETRY_CONNECTION_STRING", "InstrumentationKey=local-test;IngestionEndpoint="+receiver.URL)
	t.Setenv("SSL_CERT_FILE", cert)
	mount := mountLoopback(t)
	if os.Getenv("BLOBFUSE_DISABLE_TELEMETRY") != "false" {
		t.Fatal("ordinary mount launcher changed the parent environment")
	}
	if err := os.WriteFile(filepath.Join(mount.dir, "probe"), []byte("local data"), 0600); err != nil {
		t.Fatal(err)
	}
	time.Sleep(200 * time.Millisecond)
	mount.unmount()
	select {
	case <-mount.exited:
	case <-time.After(5 * time.Second):
		t.Fatal("ordinary mount worker did not exit")
	}
	if requests.Load() != 0 {
		t.Fatalf("ordinary test mount uploaded %d telemetry events", requests.Load())
	}
}
