//go:build !unittest

// Copyright (c) 2026 Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package main

import (
	"context"
	"encoding/json"
	"encoding/pem"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"gopkg.in/yaml.v2"
)

func TestStaleMountTelemetryRecovery(t *testing.T) {
	binary := os.Getenv("BLOBFUSE_TEST_BINARY")
	if binary == "" {
		t.Skip("set BLOBFUSE_TEST_BINARY to run the isolated killed-mount/remount integration test")
	}
	binary, err := filepath.Abs(binary)
	if err != nil {
		t.Fatal(err)
	}
	for _, disabled := range []bool{false, true} {
		name := "recovery"
		if disabled {
			name = "opt-out-purge"
		}
		t.Run(name, func(t *testing.T) { testStaleMount(t, binary, disabled) })
	}
}

func testStaleMount(t *testing.T, binary string, disableRemount bool) {
	dir := t.TempDir()
	mount, source, work := filepath.Join(dir, "mount"), filepath.Join(dir, "source"), filepath.Join(dir, "work")
	for _, path := range []string{mount, source, work} {
		if err := os.Mkdir(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(source, "probe.txt"), []byte("safe local data\n"), 0600); err != nil {
		t.Fatal(err)
	}
	type event struct {
		Name       string
		Properties map[string]string
	}
	var mu sync.Mutex
	var events []event
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
			t.Error("invalid local test event")
			w.WriteHeader(400)
			return
		}
		mu.Lock()
		for _, e := range batch {
			events = append(events, event{Name: e.Data.Base.Name, Properties: e.Data.Base.Properties})
		}
		mu.Unlock()
		w.WriteHeader(200)
	}))
	defer server.Close()
	cert := filepath.Join(dir, "test-ca.pem")
	if err := os.WriteFile(cert, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw}), 0600); err != nil {
		t.Fatal(err)
	}
	config := map[string]interface{}{
		"components": []string{"libfuse", "loopbackfs"},
		"loopbackfs": map[string]interface{}{"path": source},
		"foreground": true, "read-only": true, "allow-other": false,
		"default-working-dir": work,
		"logging":             map[string]interface{}{"type": "base", "level": "LOG_INFO", "file-path": filepath.Join(dir, "blobfuse.log")},
		"microsoft-telemetry": map[string]interface{}{"disabled": false, "snapshot-interval-sec": 60},
	}
	configPath := filepath.Join(dir, "mount.yaml")
	writeConfig := func() {
		t.Helper()
		b, err := yaml.Marshal(config)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(configPath, b, 0600); err != nil {
			t.Fatal(err)
		}
	}
	writeConfig()
	type worker struct {
		cmd  *exec.Cmd
		done chan struct{}
		err  error
	}
	start := func(label string) *worker {
		t.Helper()
		f, err := os.OpenFile(filepath.Join(dir, label+".log"), os.O_CREATE|os.O_WRONLY, 0600)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { f.Close() })
		w := &worker{done: make(chan struct{})}
		w.cmd = exec.Command(binary, "mount", mount, "--foreground", "--read-only", "--disable-version-check", "--config-file="+configPath)
		for _, env := range os.Environ() {
			key, _, _ := strings.Cut(env, "=")
			if key != "BLOBFUSE_DISABLE_TELEMETRY" && key != "BLOBFUSE_TELEMETRY_CONNECTION_STRING" && key != "SSL_CERT_FILE" {
				w.cmd.Env = append(w.cmd.Env, env)
			}
		}
		w.cmd.Env = append(w.cmd.Env, "BLOBFUSE_DISABLE_TELEMETRY=false",
			"BLOBFUSE_TELEMETRY_CONNECTION_STRING=InstrumentationKey=local-test;IngestionEndpoint="+server.URL,
			"SSL_CERT_FILE="+cert)
		w.cmd.Stdout, w.cmd.Stderr = f, f
		if err := w.cmd.Start(); err != nil {
			t.Fatal(err)
		}
		go func() { w.err = w.cmd.Wait(); close(w.done) }()
		t.Cleanup(func() {
			if mounted, err := isMounted(mount); err != nil || mounted {
				if err := unmount(binary, mount, false); err != nil {
					if err := unmount(binary, mount, true); err != nil {
						t.Error("test mount cleanup failed:", err)
					}
				}
			}
			select {
			case <-w.done:
			case <-time.After(10 * time.Second):
				_ = w.cmd.Process.Kill()
				<-w.done
				t.Error("worker needed forced cleanup")
			}
		})
		return w
	}
	await := func(w *worker, condition func() bool) {
		t.Helper()
		deadline := time.Now().Add(20 * time.Second)
		for time.Now().Before(deadline) {
			if condition() {
				return
			}
			select {
			case <-w.done:
				t.Fatalf("mount worker exited early: %v", w.err)
			case <-time.After(50 * time.Millisecond):
			}
		}
		t.Fatal("timed out waiting for mount lifecycle/state")
	}
	stateRoot := filepath.Join(work, "microsoft-telemetry")
	sessionFiles := func() map[string]bool {
		found := map[string]bool{}
		err := filepath.WalkDir(stateRoot, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if strings.HasPrefix(entry.Name(), "session-") {
				b, err := os.ReadFile(path)
				if err != nil {
					if errors.Is(err, os.ErrNotExist) {
						return nil
					}
					return err
				}
				var record struct {
					Session string `json:"session"`
					Ready   bool   `json:"ready"`
				}
				if json.Unmarshal(b, &record) == nil {
					found[record.Session] = record.Ready
				}
			}
			return nil
		})
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			t.Fatal(err)
		}
		return found
	}
	first := start("first")
	firstSession := ""
	await(first, func() bool {
		for id, ready := range sessionFiles() {
			if ready {
				firstSession = id
				return true
			}
		}
		return false
	})
	if err := first.cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-first.done:
	case <-time.After(5 * time.Second):
		t.Fatal("first worker did not exit after targeted kill")
	}
	if mounted, err := isMounted(mount); err != nil || !mounted {
		t.Fatalf("expected a stale mount after worker death: %t %v", mounted, err)
	}
	_, resolveErr := filepath.EvalSymlinks(mount)
	if !errors.Is(resolveErr, syscall.ENOTCONN) {
		t.Skipf("kernel did not expose the path-resolution failure under review: %v", resolveErr)
	}
	t.Log("Killed only the isolated test worker; its mount remains and path resolution returns ENOTCONN.")
	mu.Lock()
	beforeRestart := len(events)
	mu.Unlock()
	if disableRemount {
		config["microsoft-telemetry"].(map[string]interface{})["disabled"] = true
		writeConfig()
	}
	second := start("second")
	secondSession := ""
	if disableRemount {
		await(second, func() bool {
			if mounted, _ := isMounted(mount); !mounted {
				return false
			}
			return len(sessionFiles()) == 0
		})
	} else {
		await(second, func() bool {
			for id, ready := range sessionFiles() {
				if id != firstSession && ready {
					secondSession = id
				}
			}
			mu.Lock()
			defer mu.Unlock()
			for _, e := range events {
				if e.Name == "blobfuse.mount.recovery" && e.Properties["previous_session_id"] == firstSession &&
					e.Properties["termination_class"] == "unclean_unknown" && secondSession != "" {
					return true
				}
			}
			return false
		})
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	b, err := exec.CommandContext(ctx, "/bin/cat", filepath.Join(mount, "probe.txt")).Output()
	if err != nil || string(b) != "safe local data\n" {
		t.Fatalf("remounted filesystem unusable: %v", err)
	}
	if err := unmount(binary, mount, false); err != nil {
		t.Fatal("normal final unmount failed:", err)
	}
	select {
	case <-second.done:
		if second.err != nil {
			t.Fatal(second.err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("replacement worker did not exit")
	}
	mu.Lock()
	defer mu.Unlock()
	if disableRemount {
		if len(events) != beforeRestart {
			t.Fatal("disabled remount transmitted telemetry")
		}
		t.Log("Opted-out remount repaired the stale path and purged its old state without sending telemetry.")
	} else {
		attempts, results := 0, 0
		for _, e := range events {
			if e.Properties["mount_session_id"] == secondSession {
				if e.Name == "blobfuse.mount.attempted" {
					attempts++
				}
				if e.Name == "blobfuse.mount.result" {
					results++
				}
			}
		}
		if attempts != 1 || results != 1 {
			t.Fatalf("late attachment duplicated startup: attempts=%d results=%d", attempts, results)
		}
		t.Log("Remount recovered the previous session, persisted its own readiness, and reported startup once.")
	}
}
