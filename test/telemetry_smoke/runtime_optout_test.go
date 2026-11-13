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

// Explicit opt-in: this test creates a real, local read-only FUSE mount.
func TestRuntimeConfigOptOutMounted(t *testing.T) {
	binary := os.Getenv("BLOBFUSE_TEST_BINARY")
	if binary == "" {
		t.Skip("set BLOBFUSE_TEST_BINARY to a freshly built BlobFuse binary to run the real mount test")
	}
	binary, err := filepath.Abs(binary)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	mountPath := filepath.Join(dir, "mount")
	source := filepath.Join(dir, "source")
	work := filepath.Join(dir, "work")
	for _, path := range []string{mountPath, source, work} {
		if err := os.Mkdir(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(source, "probe.txt"), []byte("read-only probe\n"), 0600); err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	var names []string
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var batch []struct {
			Data struct {
				Base struct {
					Name       string            `json:"name"`
					Properties map[string]string `json:"properties"`
				} `json:"baseData"`
			} `json:"data"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxItem)).Decode(&batch); err != nil {
			t.Error("invalid local telemetry payload")
			w.WriteHeader(400)
			return
		}
		mu.Lock()
		for _, e := range batch {
			name := e.Data.Base.Name
			if name == "blobfuse.mount.result" {
				name += ":" + e.Data.Base.Properties["outcome"]
			}
			names = append(names, name)
		}
		mu.Unlock()
		w.WriteHeader(503)
	}))
	defer server.Close()
	certPath := filepath.Join(dir, "local-test-ca.pem")
	if err := os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{
		Type: "CERTIFICATE", Bytes: server.Certificate().Raw,
	}), 0600); err != nil {
		t.Fatal(err)
	}
	config := map[string]interface{}{
		"components": []string{"libfuse", "loopbackfs"},
		"loopbackfs": map[string]interface{}{"path": source},
		"foreground": true, "read-only": true, "allow-other": false,
		"default-working-dir": work,
		"logging":             map[string]interface{}{"type": "base", "level": "LOG_WARNING", "file-path": filepath.Join(dir, "blobfuse.log")},
		"microsoft-telemetry": map[string]interface{}{"disabled": false},
	}
	configPath := filepath.Join(dir, "mount.yaml")
	writeConfig := func(atomicReplace bool) {
		t.Helper()
		b, err := yaml.Marshal(config)
		if err != nil {
			t.Fatal(err)
		}
		path := configPath
		if atomicReplace {
			path += ".new"
		}
		if err := os.WriteFile(path, b, 0600); err != nil {
			t.Fatal(err)
		}
		if atomicReplace {
			if err := os.Rename(path, configPath); err != nil {
				t.Fatal(err)
			}
		}
	}
	writeConfig(false)
	logFile, err := os.OpenFile(filepath.Join(dir, "worker.log"), os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer logFile.Close()
	cmd := exec.Command(binary, "mount", mountPath, "--config-file="+configPath, "--foreground", "--read-only", "--disable-version-check")
	for _, env := range os.Environ() {
		key, _, _ := strings.Cut(env, "=")
		if key != "BLOBFUSE_DISABLE_TELEMETRY" && key != "BLOBFUSE_TELEMETRY_CONNECTION_STRING" && key != "SSL_CERT_FILE" {
			cmd.Env = append(cmd.Env, env)
		}
	}
	cmd.Env = append(cmd.Env, "BLOBFUSE_DISABLE_TELEMETRY=false",
		"BLOBFUSE_TELEMETRY_CONNECTION_STRING=InstrumentationKey=local-test;IngestionEndpoint="+server.URL,
		"SSL_CERT_FILE="+certPath)
	cmd.Stdout, cmd.Stderr = logFile, logFile
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	var exitErr error
	go func() { exitErr = cmd.Wait(); close(done) }()
	defer func() {
		mounted, err := isMounted(mountPath)
		if err != nil || mounted {
			if err := unmount(binary, mountPath, false); err != nil {
				t.Error("normal test cleanup unmount failed:", err)
				if err := unmount(binary, mountPath, true); err != nil {
					t.Error("lazy test cleanup unmount failed:", err)
				}
			}
		}
		select {
		case <-done:
		case <-time.After(15 * time.Second):
			_ = cmd.Process.Kill()
			<-done
			t.Error("test worker required forced cleanup")
		}
	}()
	await := func(timeout time.Duration, condition func() (bool, error)) {
		t.Helper()
		deadline := time.Now().Add(timeout)
		for time.Now().Before(deadline) {
			ok, err := condition()
			if err != nil {
				t.Fatal(err)
			}
			if ok {
				return
			}
			select {
			case <-done:
				t.Fatalf("worker stopped early: %v", exitErr)
			case <-time.After(100 * time.Millisecond):
			}
		}
		log, _ := os.ReadFile(filepath.Join(dir, "worker.log"))
		t.Fatalf("condition timed out; local worker output:\n%s", log)
	}
	await(20*time.Second, func() (bool, error) { return isMounted(mountPath) })
	probe := func() {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		out, err := exec.CommandContext(ctx, "/bin/cat", filepath.Join(mountPath, "probe.txt")).Output()
		if err != nil || string(out) != "read-only probe\n" {
			t.Fatalf("mounted file access failed: %v", err)
		}
	}
	probe()
	await(75*time.Second, func() (bool, error) {
		mu.Lock()
		defer mu.Unlock()
		ready := false
		for _, name := range names {
			ready = ready || name == "blobfuse.mount.result:success"
		}
		return ready, nil
	})
	t.Log("Real mount emitted readiness to the local HTTPS receiver.")
	config["microsoft-telemetry"].(map[string]interface{})["disabled"] = true
	writeConfig(true) // Exercise editor-style replacement, not a direct Disable() call.
	await(10*time.Second, func() (bool, error) {
		log, err := os.ReadFile(filepath.Join(dir, "blobfuse.log"))
		if err != nil {
			return false, err
		}
		return strings.Contains(string(log), "WatchConfig : Config change detected"), nil
	})
	mu.Lock()
	requestsAfterDisable := len(names)
	mu.Unlock()
	probe()
	t.Log("Atomic YAML edit reached the config watcher; mounted file access still works.")
	// Ensure opt-out leaves the filesystem running until normal teardown.
	select {
	case <-done:
		t.Fatalf("opt-out stopped the mount: %v", exitErr)
	case <-time.After(time.Second):
	}
	mu.Lock()
	requestsNow := len(names)
	mu.Unlock()
	if requestsNow != requestsAfterDisable {
		t.Fatalf("telemetry continued after opt-out: %d -> %d requests", requestsAfterDisable, requestsNow)
	}
	if err := unmount(binary, mountPath, false); err != nil {
		t.Fatal("unmount failed:", err)
	}
	select {
	case <-done:
		if exitErr != nil {
			t.Fatal("worker failed after unmount:", exitErr)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("worker did not exit after unmount")
	}
	mu.Lock()
	defer mu.Unlock()
	if len(names) != requestsAfterDisable {
		t.Fatal("shutdown emitted telemetry after opt-out")
	}
	if mounted, err := isMounted(mountPath); err != nil || mounted {
		t.Fatalf("test mount not cleaned up: mounted=%t err=%v", mounted, err)
	}
	if _, err := os.Stat(filepath.Join(work, "microsoft-telemetry")); !os.IsNotExist(err) {
		t.Fatalf("lifecycle-only telemetry created a recovery store: %v", err)
	}
	t.Log("No telemetry after opt-out or during shutdown; clean unmount confirmed. No Azure traffic.")
}
