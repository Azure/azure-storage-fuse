//go:build !unittest
// +build !unittest

/*
    _____           _____   _____   ____          ______  _____  ------
   |     |  |      |     | |     | |     |     | |       |            |
   |     |  |      |     | |     | |     |     | |       |            |
   | --- |  |      |     | |-----| |---- |     | |-----| |-----  ------
   |     |  |      |     | |     | |     |     |       | |       |
   | ____|  |_____ | ____| | ____| |     |_____|  _____| |_____  |_____


   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2020-2026 Microsoft Corporation. All rights reserved.
   Author : <blobfusedev@microsoft.com>

   Permission is hereby granted, free of charge, to any person obtaining a copy
   of this software and associated documentation files (the "Software"), to deal
   in the Software without restriction, including without limitation the rights
   to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
   copies of the Software, and to permit persons to whom the Software is
   furnished to do so, subject to the following conditions:

   The above copyright notice and this permission notice shall be included in all
   copies or substantial portions of the Software.

   THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
   IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
   FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
   AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
   LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
   OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
   SOFTWARE
*/

package dcache_e2e

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// mountReadyTimeout bounds how long we wait for `blobfuse2 mount` to make
// the mount visible in /proc/mounts. blobfuse2 backgrounds itself after
// libfuse mounts, so we cannot rely purely on the process exit.
const mountReadyTimeout = 30 * time.Second

// mountBlobfuse mounts blobfuse2 at testCfg.mntPath using testCfg.configFile.
// It fails the test on error (fatal). Also wipes testCfg.tmpPath first so
// that block_cache / file_cache always start cold: this is central to the
// L2-miss test contract -- without a cold local cache the first read may be
// served from block_cache and never reach dist_cache.
func mountBlobfuse(t *testing.T) {
	t.Helper()

	// Ensure the mount point exists and is empty (not currently mounted).
	if err := ensureCleanMountPoint(testCfg.mntPath); err != nil {
		t.Fatalf("mount point %q not usable: %v", testCfg.mntPath, err)
	}

	// Wipe the local cache so block_cache/file_cache start empty. We rmdir
	// the directory itself rather than just its contents so that any
	// stray sub-directory (e.g. leftover per-file state) is dropped.
	if err := os.RemoveAll(testCfg.tmpPath); err != nil {
		t.Fatalf("failed to wipe local cache dir %q: %v", testCfg.tmpPath, err)
	}
	if err := os.MkdirAll(testCfg.tmpPath, 0o755); err != nil {
		t.Fatalf("failed to recreate local cache dir %q: %v", testCfg.tmpPath, err)
	}

	args := []string{
		"mount", testCfg.mntPath,
		"--config-file=" + testCfg.configFile,
		// The read-only dist_cache design forbids write-through the FUSE
		// mount. Enforcing --read-only here means an accidental os.Create
		// via the mount inside a test will fail loudly with EROFS instead
		// of silently exercising a code path that dist_cache does not
		// support in this scope.
		"--read-only=true",
	}
	t.Logf("mount: %s %s", testCfg.blobfuseBin, strings.Join(args, " "))

	cmd := exec.Command(testCfg.blobfuseBin, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("blobfuse2 mount failed: %v\nstdout: %s\nstderr: %s",
			err, stdout.String(), stderr.String())
	}

	if err := waitForMount(testCfg.mntPath, mountReadyTimeout); err != nil {
		t.Fatalf("mount did not become ready: %v\nstdout: %s\nstderr: %s",
			err, stdout.String(), stderr.String())
	}
}

// unmountBlobfuse unmounts blobfuse2 at testCfg.mntPath. It is fatal on
// error because a leaked mount will poison subsequent test cases.
func unmountBlobfuse(t *testing.T) {
	t.Helper()
	if err := runUnmount(testCfg.mntPath); err != nil {
		t.Fatalf("blobfuse2 unmount failed: %v", err)
	}
	if err := waitForUnmount(testCfg.mntPath, mountReadyTimeout); err != nil {
		t.Fatalf("mount point still busy after unmount: %v", err)
	}
}

// remountBlobfuse cycles the mount to guarantee a cold local cache between
// operations. It is the primary hook that makes L2-miss / L2-hit
// distinguishable in these tests.
func remountBlobfuse(t *testing.T) {
	t.Helper()
	unmountBlobfuse(t)
	mountBlobfuse(t)
}

// unmountBestEffort is called from TestMain to sweep any stray mount left
// behind by a prior aborted run. It never fails the process; anything
// interesting is logged to stderr.
func unmountBestEffort() {
	if testCfg.mntPath == "" {
		return
	}
	if !isMounted(testCfg.mntPath) {
		return
	}
	fmt.Fprintf(os.Stderr, "dcache_e2e: sweeping stray mount at %s\n", testCfg.mntPath)
	if err := runUnmount(testCfg.mntPath); err != nil {
		fmt.Fprintf(os.Stderr, "dcache_e2e: best-effort unmount failed: %v\n", err)
	}
}

// runUnmount tries `blobfuse2 unmount` first (matches the CLI users invoke),
// then falls back to `fusermount3 -u` and finally `sudo umount -f`. The
// fallbacks exist because blobfuse2's own unmount subcommand shells out to
// fusermount and fails if it is not on PATH; the pipeline agents have it,
// but a developer's local machine may not.
func runUnmount(mnt string) error {
	if !isMounted(mnt) {
		return nil
	}
	tryCommands := [][]string{
		{testCfg.blobfuseBin, "unmount", mnt},
		{"fusermount3", "-u", mnt},
		{"fusermount", "-u", mnt},
		{"sudo", "umount", "-f", mnt},
	}
	var lastErr error
	for _, argv := range tryCommands {
		if _, err := exec.LookPath(argv[0]); err != nil {
			continue
		}
		out, err := exec.Command(argv[0], argv[1:]...).CombinedOutput()
		if err == nil {
			return nil
		}
		lastErr = fmt.Errorf("%s: %w (output: %s)", strings.Join(argv, " "), err, strings.TrimSpace(string(out)))
	}
	if lastErr == nil {
		return fmt.Errorf("no unmount tool available on PATH")
	}
	return lastErr
}

// waitForMount polls /proc/mounts until an entry for `mnt` appears or the
// deadline elapses. blobfuse2's mount subcommand daemonizes after mounting,
// so a successful cmd.Run() does not by itself imply the FS is ready.
func waitForMount(mnt string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if isMounted(mnt) {
			// Additional smoke check: the directory should be listable.
			if _, err := os.ReadDir(mnt); err == nil {
				return nil
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("mount %q not visible after %s", mnt, timeout)
}

// waitForUnmount is the inverse of waitForMount.
func waitForUnmount(mnt string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !isMounted(mnt) {
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("mount %q still mounted after %s", mnt, timeout)
}

// isMounted checks /proc/mounts for any entry whose mount point equals `mnt`
// (after cleaning). Uses /proc/mounts rather than `mountpoint(1)` so it
// works in constrained CI containers without util-linux.
func isMounted(mnt string) bool {
	abs, err := filepath.Abs(mnt)
	if err != nil {
		return false
	}
	data, err := os.ReadFile("/proc/mounts")
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		if fields[1] == abs {
			return true
		}
	}
	return false
}

// ensureCleanMountPoint verifies `mnt` exists, is a directory, and is not
// currently mounted. It does not create the directory (the pipeline owns
// that -- creating it silently here would mask a misconfigured MOUNT_DIR).
func ensureCleanMountPoint(mnt string) error {
	info, err := os.Stat(mnt)
	if err != nil {
		return fmt.Errorf("stat: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("not a directory")
	}
	if isMounted(mnt) {
		// Attempt to unmount whatever is there; a prior test may have
		// bailed before its cleanup ran.
		if err := runUnmount(mnt); err != nil {
			return fmt.Errorf("already mounted and unmount failed: %w", err)
		}
		if err := waitForUnmount(mnt, mountReadyTimeout); err != nil {
			return fmt.Errorf("already mounted and unmount timed out: %w", err)
		}
	}
	return nil
}
