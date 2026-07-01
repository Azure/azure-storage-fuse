//go:build integration && !fuse2

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

package libfuse

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal"
)

// integrationHarness manages a live FUSE mount for integration tests.
//
// The caller supplies any internal.Component as the backend and the libfuse
// section of the YAML config. The harness owns mount-path creation, FUSE
// startup, and teardown — everything else is the caller's concern.
//
// Typical usage:
//
//	backend := newCountingBackend()
//	h := newIntegrationHarness(t, backend, "libfuse:\n  kernel-list-cache-expiration-sec: 30\n")
//	h.start(t)
//	defer h.stop(t)
//	// interact with h.MountDir, backend directly
type integrationHarness struct {
	MountDir string
	lf       *Libfuse
	cancel   context.CancelFunc
	done     chan struct{}
}

// newIntegrationHarness wires up the pipeline and creates a temp mount directory.
// libfuseCfg is the "libfuse:" section of the YAML config (without mount-path,
// which the harness injects automatically). Pass "" for defaults.
// The caller must call h.start(t) before interacting with MountDir and h.stop(t)
// in a defer to unmount and clean up.
func newIntegrationHarness(t *testing.T, backend internal.Component, libfuseCfg string) *integrationHarness {
	t.Helper()
	checkFuseAvailable(t)

	mountDir, err := os.MkdirTemp("", "blobfuse-integration-*")
	if err != nil {
		t.Fatalf("failed to create temp mount dir: %v", err)
	}

	cfg := fmt.Sprintf("mount-path: %s\n%s", mountDir, libfuseCfg)
	lf := newTestLibfuse(backend, cfg)

	_, cancel := context.WithCancel(context.Background())
	return &integrationHarness{
		MountDir: mountDir,
		lf:       lf,
		cancel:   cancel,
		done:     make(chan struct{}),
	}
}

// start launches the FUSE event loop in a goroutine and blocks until the
// mount point is visible in /proc/mounts (up to 5 s).
func (h *integrationHarness) start(t *testing.T) {
	t.Helper()

	// ForegroundMount=true suppresses the SIGUSR2 sent to the parent process by
	// NotifyMountToParent inside libfuse_init.  Save and restore the original
	// value so other tests in the same binary are not affected.
	origForeground := common.ForegroundMount
	common.ForegroundMount = true
	t.Cleanup(func() { common.ForegroundMount = origForeground })

	go func() {
		defer close(h.done)
		// Start blocks inside C.start_fuse (fuse_main) until the mount is
		// unmounted externally via fusermount3 -u.
		if err := h.lf.Start(context.Background()); err != nil {
			// A non-nil error from Start means fuse_main failed (e.g. mount
			// permission denied). The test will time out in waitForMount and
			// report a clear message.
			log.Err("integrationHarness::start : %v", err)
		}
	}()

	if !waitForMount(h.MountDir, 5*time.Second) {
		h.lf.Stop()
		os.RemoveAll(h.MountDir)
		t.Fatalf("FUSE mount at %s did not appear within 5s — is FUSE available and are permissions sufficient?", h.MountDir)
	}
}

// stop unmounts the FUSE filesystem, waits for the goroutine to finish, then
// cleans up the mount directory.
func (h *integrationHarness) stop(t *testing.T) {
	t.Helper()

	// Trigger unmount so fuse_main returns and the goroutine can exit.
	for _, bin := range []string{"fusermount3", "fusermount"} {
		if err := exec.Command(bin, "-u", h.MountDir).Run(); err == nil {
			break
		}
	}
	h.cancel()

	select {
	case <-h.done:
	case <-time.After(5 * time.Second):
		t.Log("warn: FUSE goroutine did not exit within 5s after unmount")
	}

	// Clean up tracker goroutine, stats collector, etc.
	_ = h.lf.Stop()
	os.RemoveAll(h.MountDir)
}

// listDir calls os.ReadDir against MountDir (or a subdirectory) and fails the
// test on error. dir is relative to MountDir; pass "" for the root.
func (h *integrationHarness) listDir(t *testing.T, dir string) {
	t.Helper()
	target := h.MountDir
	if dir != "" {
		target = h.MountDir + "/" + dir
	}
	if _, err := os.ReadDir(target); err != nil {
		t.Fatalf("ReadDir(%q) failed: %v", target, err)
	}
}

// waitForMount polls /proc/mounts until mountDir appears or timeout elapses.
func waitForMount(mountDir string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		data, _ := os.ReadFile("/proc/mounts")
		if strings.Contains(string(data), mountDir) {
			return true
		}
		time.Sleep(50 * time.Millisecond)
	}
	return false
}

// checkFuseAvailable skips the test if FUSE is not usable by the current process.
func checkFuseAvailable(t *testing.T) {
	t.Helper()
	if _, err := os.Stat("/dev/fuse"); err != nil {
		t.Skip("FUSE not available (/dev/fuse missing): ", err)
	}
	if os.Getuid() != 0 {
		// Non-root can mount FUSE only when /etc/fuse.conf contains
		// "user_allow_other".  Check for that rather than skipping blindly.
		data, _ := os.ReadFile("/etc/fuse.conf")
		if !strings.Contains(string(data), "user_allow_other") {
			t.Skip("FUSE integration tests require root or 'user_allow_other' in /etc/fuse.conf")
		}
	}
}
