//go:build !unittest

// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/internal/telemetry"
	"github.com/Azure/azure-storage-fuse/v2/internal/telemetry/telemetrytest"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

const maxItem = telemetry.MaxEventBytes

type telemetryMount struct {
	binary, mountPath, outputPath string
	cmd                           *exec.Cmd
	done                          chan struct{}
	exitErr                       error
	started                       time.Time
}

func startTelemetryMount(t *testing.T, handler http.HandlerFunc) *telemetryMount {
	t.Helper()
	binary := os.Getenv("BLOBFUSE_TEST_BINARY")
	if binary == "" {
		t.Skip("set BLOBFUSE_TEST_BINARY to a freshly built BlobFuse binary to run the real mount test")
	}
	binary, err := filepath.Abs(binary)
	require.NoError(t, err)
	dir := t.TempDir()
	source, work := filepath.Join(dir, "source"), filepath.Join(dir, "work")
	configPath, logPath := filepath.Join(dir, "mount.yaml"), filepath.Join(dir, "blobfuse.log")
	m := &telemetryMount{
		binary: binary, mountPath: filepath.Join(dir, "mount"),
		outputPath: filepath.Join(dir, "worker.log"), done: make(chan struct{}),
	}
	for _, path := range []string{m.mountPath, source, work} {
		require.NoError(t, os.Mkdir(path, 0700))
	}
	server := telemetrytest.NewReceiver(t, handler)
	config := map[string]interface{}{
		"components": []string{"libfuse", "loopbackfs"},
		"loopbackfs": map[string]interface{}{"path": source},
		"foreground": false, "read-only": true, "allow-other": false,
		"default-working-dir": work,
		"logging":             map[string]interface{}{"type": "base", "level": "LOG_WARNING", "file-path": logPath},
		"microsoft-telemetry": map[string]interface{}{"disabled": false},
	}
	body, err := yaml.Marshal(config)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(configPath, body, 0600))
	output, err := os.Create(m.outputPath)
	require.NoError(t, err)
	m.cmd = exec.Command(binary, "mount", m.mountPath, "--config-file="+configPath,
		"--foreground=false", "--read-only", "--disable-version-check", "--wait-for-mount=1s")
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if key != "BLOBFUSE_DISABLE_TELEMETRY" {
			m.cmd.Env = append(m.cmd.Env, entry)
		}
	}
	m.cmd.Env = append(m.cmd.Env, server.Env...)
	m.cmd.Env = append(m.cmd.Env, "BLOBFUSE_DISABLE_TELEMETRY=false")
	m.cmd.Stdout, m.cmd.Stderr = output, output
	m.started = time.Now()
	if err := m.cmd.Start(); err != nil {
		_ = output.Close()
		t.Fatal(err)
	}
	go func() { m.exitErr = m.cmd.Wait(); _ = output.Close(); close(m.done) }()
	t.Cleanup(func() {
		if mounted, err := isMounted(m.mountPath); err != nil || mounted {
			if err := unmount(binary, m.mountPath, false); err != nil {
				t.Error("normal test cleanup unmount failed:", err)
				if err := unmount(binary, m.mountPath, true); err != nil {
					t.Error("lazy test cleanup unmount failed:", err)
				}
			}
		}
		select {
		case <-m.done:
		case <-time.After(15 * time.Second):
			_ = m.cmd.Process.Kill()
			<-m.done
			t.Error("test worker required forced cleanup")
		}
	})
	return m
}

func (m *telemetryMount) await(t *testing.T, timeout time.Duration, condition func() (bool, error)) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		ok, err := condition()
		require.NoError(t, err)
		if ok {
			return
		}
		select {
		case <-m.done:
			if m.exitErr != nil {
				t.Fatalf("worker stopped early: %v", m.exitErr)
			}
		default:
		}
		time.Sleep(20 * time.Millisecond)
	}
	output, err := os.ReadFile(m.outputPath)
	require.NoError(t, err)
	t.Fatalf("condition timed out; local worker output:\n%s", output)
}

func isMounted(path string) (bool, error) {
	b, err := os.ReadFile("/proc/self/mountinfo")
	if err != nil {
		return false, errors.New("cannot inspect Linux mount state")
	}
	for _, line := range strings.Split(string(b), "\n") {
		fields := strings.Fields(line)
		if len(fields) > 4 && fields[4] == path {
			return true, nil
		}
	}
	return false, nil
}

func unmount(binary, path string, lazy bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	args := []string{"unmount", path}
	if lazy {
		args = append(args, "--lazy")
	}
	return exec.CommandContext(ctx, binary, args...).Run()
}
