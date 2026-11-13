//go:build !unittest

// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package loopback_e2e

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/internal/telemetry/telemetrytest"
)

func TestOrdinaryLoopbackMountDisablesTelemetry(t *testing.T) {
	var requests atomic.Int64
	receiver := telemetrytest.NewReceiver(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.WriteHeader(200)
	}))
	// Shared launcher opt-out must beat an inherited enable and
	// destination, without ever contacting the compiled or configured endpoint.
	t.Setenv("BLOBFUSE_DISABLE_TELEMETRY", "false")
	for _, entry := range receiver.Env {
		key, value, _ := strings.Cut(entry, "=")
		t.Setenv(key, value)
	}
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
