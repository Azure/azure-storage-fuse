//go:build !unittest

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

package loopback_e2e

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

var (
	blobfuseBinary = flag.String("blobfuse2", "../../blobfuse2", "Path to the blobfuse2 binary to mount")
	runDuration    = flag.Duration("duration", 30*time.Second, "How long to run the file workload")
	stallTimeout   = flag.Duration("stall-timeout", 20*time.Second, "Fail if no file operation completes for this long")
)

const (
	burstWorkers  = 2
	burstHandles  = 256
	openWorkers   = 32
	renameWorkers = 16
	stableData    = "stable"
)

const configTemplate = `logging:
  type: base
  level: log_warning
  file-path: %s
components:
  - libfuse
  - file_cache
  - loopbackfs
libfuse:
  direct-io: true
  attribute-expiration-sec: 0
  entry-expiration-sec: 0
  negative-entry-expiration-sec: 0
file_cache:
  path: %s
  timeout-sec: 0
loopbackfs:
  path: %s
`

// TestFileCacheCloseRenameDoesNotHang mounts blobfuse2 over loopbackfs with direct_io and a zero
// file-cache timeout, matching the AKS workload that deadlocked file_cache: bursts of closes on a
// shared file, concurrent opens, and small files that are written, fsynced, closed and renamed over
// reused names. Every close and rename queues local cache cleanup. The test fails if file operations
// stop making progress, and reports the hung daemon's file-cache goroutines.
func TestFileCacheCloseRenameDoesNotHang(t *testing.T) {
	m := mountLoopback(t)

	w := &workload{root: m.dir, stable: filepath.Join(m.dir, "stable"), stop: make(chan struct{})}
	if err := os.WriteFile(w.stable, []byte(stableData), 0644); err != nil {
		t.Fatalf("failed to create %s: %v", w.stable, err)
	}

	var wg sync.WaitGroup
	start := func(count int, run func(id int)) {
		for id := 0; id < count; id++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				run(id)
			}()
		}
	}
	start(burstWorkers, func(int) { w.releaseBursts() })
	start(openWorkers, func(int) { w.openAndRead() })
	start(renameWorkers, w.writeAndRename)

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	if !w.monitor(done) {
		summary := m.dumpAndStop()
		select {
		case <-done:
		case <-time.After(time.Minute):
			t.Log("workload goroutines are still blocked after blobfuse2 exited")
		}
		t.Fatalf("no file operation completed for %v after %d operations; blobfuse2 appears hung.\n%s",
			*stallTimeout, w.progress.Load(), summary)
	}

	for _, err := range w.errors() {
		t.Error(err)
	}
	t.Logf("completed %d file operations in %v", w.progress.Load(), *runDuration)
}

type workload struct {
	root     string
	stable   string
	stop     chan struct{}
	stopOnce sync.Once
	aborted  atomic.Bool
	progress atomic.Int64

	mu   sync.Mutex
	errs []error
}

func (w *workload) stopped() bool {
	select {
	case <-w.stop:
		return true
	default:
		return false
	}
}

func (w *workload) halt() {
	w.stopOnce.Do(func() { close(w.stop) })
}

func (w *workload) fail(format string, args ...any) {
	// Errors are expected while a hung mount is torn down.
	if w.aborted.Load() {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.errs) < 20 {
		w.errs = append(w.errs, fmt.Errorf(format, args...))
	}
}

func (w *workload) errors() []error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]error(nil), w.errs...)
}

// monitor stops the workload after the configured duration. It returns false if no file operation
// completes within the stall timeout.
func (w *workload) monitor(done <-chan struct{}) bool {
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	end := time.After(*runDuration)
	last, lastChange := w.progress.Load(), time.Now()

	for {
		select {
		case <-done:
			return true
		case <-end:
			w.halt()
			end = nil
		case now := <-ticker.C:
			if p := w.progress.Load(); p != last {
				last, lastChange = p, now
			} else if now.Sub(lastChange) > *stallTimeout {
				w.aborted.Store(true)
				w.halt()
				return false
			}
		}
	}
}

// releaseBursts opens many handles to one file and closes them together, so many releases contend
// for the same file lock while queuing cache cleanup.
func (w *workload) releaseBursts() {
	for !w.stopped() {
		files := make([]*os.File, 0, burstHandles)
		var openErr error
		for len(files) < burstHandles && !w.stopped() {
			f, err := os.Open(w.stable)
			if err != nil {
				openErr = err
				break
			}
			files = append(files, f)
		}

		var closeErrs atomic.Int32
		var wg sync.WaitGroup
		for _, f := range files {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if err := f.Close(); err != nil {
					closeErrs.Add(1)
				}
			}()
		}
		wg.Wait()

		if openErr != nil || closeErrs.Load() != 0 {
			w.fail("burst on %s: open error %v, %d close errors", w.stable, openErr, closeErrs.Load())
			return
		}
		w.progress.Add(1)
	}
}

// openAndRead repeatedly opens and reads the shared file.
func (w *workload) openAndRead() {
	buf := make([]byte, len(stableData))
	for !w.stopped() {
		f, err := os.Open(w.stable)
		if err != nil {
			w.fail("open %s: %v", w.stable, err)
			return
		}
		n, readErr := io.ReadFull(f, buf)
		closeErr := f.Close()
		if readErr != nil || closeErr != nil || string(buf[:n]) != stableData {
			w.fail("read %s: got %q, read error %v, close error %v", w.stable, buf[:n], readErr, closeErr)
			return
		}
		w.progress.Add(1)
	}
}

// writeAndRename follows the incident's synchronization pattern: write a small temporary file,
// fsync and close it, rename it over the final name, then read the result back.
func (w *workload) writeAndRename(id int) {
	tmp := filepath.Join(w.root, fmt.Sprintf("file_%d_temp", id))
	final := filepath.Join(w.root, fmt.Sprintf("file_%d", id))
	for round := 0; !w.stopped(); round++ {
		data := []byte(fmt.Sprintf("worker %d round %d", id, round))
		if err := writeSynced(tmp, data); err != nil {
			w.fail("write %s: %v", tmp, err)
			return
		}
		if err := os.Rename(tmp, final); err != nil {
			w.fail("rename %s: %v", tmp, err)
			return
		}
		got, err := os.ReadFile(final)
		if err != nil || !bytes.Equal(got, data) {
			w.fail("read %s: got %q, want %q: %v", final, got, data, err)
			return
		}
		w.progress.Add(1)
	}
}

func writeSynced(path string, data []byte) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	_, err = f.Write(data)
	if err == nil {
		err = f.Sync()
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	return err
}

type loopbackMount struct {
	dir        string
	output     string
	log        string
	fusermount string
	cmd        *exec.Cmd
	exited     chan struct{}
}

// mountLoopback runs blobfuse2 in the foreground over a temporary loopbackfs directory.
func mountLoopback(t *testing.T) *loopbackMount {
	t.Helper()
	if _, err := os.Stat("/dev/fuse"); err != nil {
		t.Skipf("FUSE is not available: %v", err)
	}
	fusermount, err := exec.LookPath("fusermount3")
	if err != nil {
		if fusermount, err = exec.LookPath("fusermount"); err != nil {
			t.Skip("fusermount is not installed")
		}
	}
	bin, err := filepath.Abs(*blobfuseBinary)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(bin); err != nil {
		t.Fatalf("blobfuse2 binary not found at %s; run ./build.sh or pass -args -blobfuse2=<path>", bin)
	}

	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	m := &loopbackMount{
		dir:        filepath.Join(base, "mnt"),
		output:     filepath.Join(base, "blobfuse2.out"),
		log:        filepath.Join(base, "blobfuse2.log"),
		fusermount: fusermount,
		exited:     make(chan struct{}),
	}
	cache, storage := filepath.Join(base, "cache"), filepath.Join(base, "storage")
	for _, dir := range []string{m.dir, cache, storage} {
		if err := os.Mkdir(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}
	configPath := filepath.Join(base, "config.yaml")
	config := fmt.Sprintf(configTemplate, m.log, cache, storage)
	if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
		t.Fatal(err)
	}

	out, err := os.Create(m.output)
	if err != nil {
		t.Fatal(err)
	}
	m.cmd = exec.Command(bin, "mount", m.dir, "--config-file="+configPath, "--foreground=true", "--disable-version-check=true")
	m.cmd.Stdout, m.cmd.Stderr = out, out
	m.cmd.Env = append(os.Environ(), "GOTRACEBACK=all")
	if err := m.cmd.Start(); err != nil {
		_ = out.Close()
		t.Fatalf("failed to start blobfuse2: %v", err)
	}
	go func() {
		_ = m.cmd.Wait()
		_ = out.Close()
		close(m.exited)
	}()
	t.Cleanup(func() {
		m.unmount()
		if t.Failed() {
			t.Logf("blobfuse2 errors:\n%s", errorLines(m.log, 20))
		}
	})

	deadline := time.Now().Add(30 * time.Second)
	for !m.mounted() {
		select {
		case <-m.exited:
			t.Fatalf("blobfuse2 exited before mounting:\n%s", readOutput(m.output))
		case <-time.After(100 * time.Millisecond):
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for blobfuse2 to mount %s:\n%s", m.dir, readOutput(m.output))
		}
	}
	return m
}

// mounted reads the mount table instead of touching the mount point, which could block on a hung mount.
func (m *loopbackMount) mounted() bool {
	data, err := os.ReadFile("/proc/self/mountinfo")
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(data), "\n") {
		if fields := strings.Fields(line); len(fields) > 4 && fields[4] == m.dir {
			return true
		}
	}
	return false
}

func (m *loopbackMount) unmount() {
	if m.mounted() && exec.Command(m.fusermount, "-u", m.dir).Run() != nil {
		m.detach()
	}
	m.waitForExit(15 * time.Second)
}

// detach lazily unmounts so that a busy or hung mount is still removed.
func (m *loopbackMount) detach() {
	if m.mounted() {
		_ = exec.Command(m.fusermount, "-u", "-z", m.dir).Run()
	}
}

func (m *loopbackMount) waitForExit(timeout time.Duration) {
	select {
	case <-m.exited:
	case <-time.After(timeout):
		_ = m.cmd.Process.Kill()
		<-m.exited
	}
}

// dumpAndStop makes the hung daemon print its goroutines and exit. Its exit aborts the FUSE
// connection, which releases workload goroutines blocked in file system calls.
func (m *loopbackMount) dumpAndStop() string {
	_ = m.cmd.Process.Signal(syscall.SIGQUIT)
	m.waitForExit(30 * time.Second)
	m.detach()
	return summarizeDump(m.output)
}

// summarizeDump reports file-cache goroutines from a SIGQUIT dump. The original deadlock shows
// close and rename goroutines blocked queuing cache cleanup while the cleanup worker waits for a file lock.
func summarizeDump(path string) string {
	var queued, waiting int
	var blocks []string
	for _, block := range strings.Split(readOutput(path), "\n\n") {
		if !strings.Contains(block, "component/file_cache.") {
			continue
		}
		if strings.Contains(block, "[chan send") && strings.Contains(block, "lruPolicy).CachePurge") {
			queued++
		}
		if strings.Contains(block, "[sync.Mutex.Lock") && strings.Contains(block, "lruPolicy).deleteItem") {
			waiting++
		}
		if len(blocks) < 8 {
			blocks = append(blocks, block)
		}
	}
	return fmt.Sprintf("File operations blocked queuing cache cleanup: %d\nCleanup worker waiting for a file lock: %d\n\n%s",
		queued, waiting, strings.Join(blocks, "\n\n"))
}

func readOutput(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Sprintf("failed to read %s: %v", path, err)
	}
	return string(data)
}

// errorLines returns the first file-cache errors from the blobfuse2 log.
func errorLines(path string, limit int) string {
	var lines []string
	for _, line := range strings.Split(readOutput(path), "\n") {
		if strings.Contains(line, "LOG_ERR") && (strings.Contains(line, "FileCache::") || strings.Contains(line, "lruPolicy::")) &&
			len(lines) < limit {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n")
}
