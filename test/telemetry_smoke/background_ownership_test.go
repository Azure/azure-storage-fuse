//go:build !unittest

// Copyright (c) 2026 Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package main

import (
	"context"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"gopkg.in/yaml.v2"
)

func TestBackgroundAttemptOwnedByDaemon(t *testing.T) {
	binary := os.Getenv("BLOBFUSE_TEST_BINARY")
	if binary == "" {
		t.Skip("set BLOBFUSE_TEST_BINARY to a freshly built BlobFuse binary to run the real mount test")
	}
	binary, err := filepath.Abs(binary)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	mountPath, source, work := filepath.Join(dir, "mount"), filepath.Join(dir, "source"), filepath.Join(dir, "work")
	for _, path := range []string{mountPath, source, work} {
		if err := os.Mkdir(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	type event struct {
		Time time.Time `json:"time"`
		Data struct {
			Base struct {
				Name         string             `json:"name"`
				Properties   map[string]string  `json:"properties"`
				Measurements map[string]float64 `json:"measurements"`
			} `json:"baseData"`
		} `json:"data"`
	}
	var mu sync.Mutex
	var events []event
	attempted := make(chan struct{})
	release := make(chan struct{})
	cancelled := make(chan struct{})
	var attemptOnce, releaseOnce, cancelOnce sync.Once
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var batch []event
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxItem)).Decode(&batch); err != nil {
			t.Error("invalid local telemetry payload:", err)
			w.WriteHeader(400)
			return
		}
		mu.Lock()
		events = append(events, batch...)
		mu.Unlock()
		for _, e := range batch {
			if e.Data.Base.Name == "blobfuse.mount.attempted" {
				attemptOnce.Do(func() { close(attempted) })
				select {
				case <-release:
				case <-r.Context().Done():
					cancelOnce.Do(func() { close(cancelled) })
					return
				}
			}
		}
		w.WriteHeader(200)
	}))
	defer server.Close()
	defer releaseOnce.Do(func() { close(release) })
	certPath := filepath.Join(dir, "ca.pem")
	if err := os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{
		Type: "CERTIFICATE", Bytes: server.Certificate().Raw,
	}), 0600); err != nil {
		t.Fatal(err)
	}
	config := map[string]interface{}{
		"components": []string{"libfuse", "loopbackfs"},
		"loopbackfs": map[string]interface{}{"path": source},
		"foreground": false, "read-only": true, "allow-other": false,
		"default-working-dir": work,
		"logging":             map[string]interface{}{"type": "base", "level": "LOG_WARNING", "file-path": filepath.Join(dir, "blobfuse.log")},
		"microsoft-telemetry": map[string]interface{}{"disabled": false},
	}
	body, err := yaml.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(dir, "mount.yaml")
	if err := os.WriteFile(configPath, body, 0600); err != nil {
		t.Fatal(err)
	}
	output, err := os.Create(filepath.Join(dir, "parent.log"))
	if err != nil {
		t.Fatal(err)
	}
	defer output.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, "mount", mountPath, "--config-file="+configPath,
		"--foreground=false", "--disable-version-check", "--wait-for-mount=1s")
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if key != "BLOBFUSE_DISABLE_TELEMETRY" && key != "BLOBFUSE_TELEMETRY_CONNECTION_STRING" && key != "SSL_CERT_FILE" {
			cmd.Env = append(cmd.Env, entry)
		}
	}
	cmd.Env = append(cmd.Env, "BLOBFUSE_DISABLE_TELEMETRY=false",
		"BLOBFUSE_TELEMETRY_CONNECTION_STRING=InstrumentationKey=local-test;IngestionEndpoint="+server.URL,
		"SSL_CERT_FILE="+certPath)
	cmd.Stdout, cmd.Stderr = output, output
	start := time.Now()
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	defer func() {
		releaseOnce.Do(func() { close(release) })
		if mounted, err := isMounted(mountPath); err != nil || mounted {
			if err := unmount(binary, mountPath, false); err != nil {
				t.Error("normal cleanup failed:", err)
				if err := unmount(binary, mountPath, true); err != nil {
					t.Error("lazy cleanup failed:", err)
				}
			}
		}
	}()
	select {
	case <-attempted:
	case <-time.After(3 * time.Second):
		t.Fatal("daemon did not start its attempted request")
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal("background parent failed:", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("background parent waited for blocked telemetry")
	}
	if elapsed := time.Since(start); elapsed >= 4*time.Second {
		t.Fatalf("background startup added a telemetry completion wait: %s", elapsed)
	}
	if mounted, err := isMounted(mountPath); err != nil || !mounted {
		t.Fatalf("daemon did not remain mounted after parent exit: mounted=%t err=%v", mounted, err)
	}
	select {
	case <-cancelled:
		t.Fatal("parent exit cancelled the attempted request")
	default:
	}
	releaseOnce.Do(func() { close(release) })
	if err := unmount(binary, mountPath, false); err != nil {
		t.Fatal("unmount failed:", err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for {
		mu.Lock()
		count := len(events)
		mu.Unlock()
		if count >= 3 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("daemon did not deliver its complete lifecycle")
		}
		time.Sleep(20 * time.Millisecond)
	}
	mu.Lock()
	defer mu.Unlock()
	counts := make(map[string]int)
	session := events[0].Data.Base.Properties["mount_session_id"]
	for _, e := range events {
		counts[e.Data.Base.Name]++
		if e.Data.Base.Properties["mount_session_id"] != session || session == "" {
			t.Fatal("background lifecycle changed session identity")
		}
		if e.Data.Base.Name == "blobfuse.mount.attempted" && (e.Time.Before(start) || e.Time.After(time.Now())) {
			t.Fatal("background attempt has an invalid observation time")
		}
		if e.Data.Base.Name == "blobfuse.mount.stopped" {
			if e.Data.Base.Properties["termination_class"] != "clean" {
				t.Fatal("successful daemon teardown was not classified as clean")
			}
			if _, present := e.Data.Base.Measurements["final_uptime_seconds"]; !present {
				t.Fatal("clean daemon shutdown is missing final uptime")
			}
			if _, present := e.Data.Base.Measurements["last_known_uptime_seconds"]; present {
				t.Fatal("clean daemon shutdown included last-known instead of final uptime")
			}
		}
	}
	if len(events) != 3 || counts["blobfuse.mount.attempted"] != 1 ||
		counts["blobfuse.mount.result"] != 1 || counts["blobfuse.mount.stopped"] != 1 {
		t.Fatalf("duplicate or missing daemon lifecycle: %+v", counts)
	}
	t.Log("Parent exited while attempted delivery was blocked; daemon delivered all three events to local HTTPS.")
}
