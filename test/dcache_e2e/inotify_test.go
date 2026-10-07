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
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"testing"
)

// The kind nodes are plain containers on this host: they do not enter a user
// namespace, so every process inside them draws on the same uid-keyed
// fs.inotify.max_user_instances budget as the agent itself. A container that
// exhausts the budget therefore breaks unrelated processes cluster-wide.
//
// This matters because kube-proxy opens an inotify instance during startup, in
// Complete(), before it touches networking. If the budget is gone at that
// moment it exits with "failed complete: too many open files" and crash-loops,
// leaving its node without any Service ClusterIP - including kube-dns.
//
// Only a process that has to *acquire* an instance is affected, which is why a
// node restart is the trigger: kube-proxy elsewhere already holds one from
// cluster creation and never reacquires it.
const (
	inotifyMaxInstancesPath = "/proc/sys/fs/inotify/max_user_instances"
	inotifyMaxWatchesPath   = "/proc/sys/fs/inotify/max_user_watches"

	// inotifyInstanceLink is the symlink target the kernel reports for an fd
	// returned by inotify_init(2).
	inotifyInstanceLink = "anon_inode:inotify"
)

// inotifyUsage is a point-in-time sample of the host's inotify instance budget.
type inotifyUsage struct {
	maxInstances int
	maxWatches   int

	// byUID counts live inotify instances per owning uid. The kernel enforces
	// max_user_instances per uid, so the per-uid figure is the one that matters.
	byUID map[int]int

	scanned    int  // processes successfully inspected
	unreadable int  // processes this user lacked permission to inspect
	elevated   bool // counts were obtained via sudo rather than directly
}

// total returns instances held across all uids.
func (u inotifyUsage) total() int {
	n := 0
	for _, c := range u.byUID {
		n += c
	}
	return n
}

// String renders the sample, flagging exhaustion and qualifying the count when
// it could only be taken in part.
func (u inotifyUsage) String() string {
	var b strings.Builder

	fmt.Fprintf(&b, "max_user_instances=%d max_user_watches=%d in-use=%d",
		u.maxInstances, u.maxWatches, u.total())

	uids := make([]int, 0, len(u.byUID))
	for uid := range u.byUID {
		uids = append(uids, uid)
	}
	sort.Ints(uids)

	if len(uids) > 0 {
		parts := make([]string, 0, len(uids))
		for _, uid := range uids {
			parts = append(parts, fmt.Sprintf("uid%d=%d", uid, u.byUID[uid]))
		}
		fmt.Fprintf(&b, " (%s)", strings.Join(parts, " "))
	}

	// Containers run as root, so uid 0 is the budget kube-proxy competes for.
	//
	// A partial sample cannot speak to it at all: the processes we were denied
	// are precisely the root-owned ones, so uid0 would read as 0 and the budget
	// would look untouched. Report that as unknown rather than as healthy.
	if u.maxInstances > 0 {
		if u.unreadable > 0 {
			b.WriteString(" uid0-headroom=unknown")
		} else {
			root := u.byUID[0]
			headroom := u.maxInstances - root
			fmt.Fprintf(&b, " uid0-headroom=%d", headroom)
			switch {
			case headroom <= 0:
				b.WriteString(" [EXHAUSTED]")
			case headroom < 16:
				b.WriteString(" [NEAR LIMIT]")
			}
		}
	}

	fmt.Fprintf(&b, "; scanned=%d", u.scanned)
	if u.elevated {
		b.WriteString(" (counted via sudo)")
	}
	if u.unreadable > 0 {
		// Without privilege we cannot see other users' fds, and containers run
		// as root. Say so plainly instead of presenting a confident undercount.
		fmt.Fprintf(&b, " unreadable=%d [PARTIAL SAMPLE: in-use is a lower bound "+
			"and uid0 usage is not observable; run as root for an exact count]",
			u.unreadable)
	}

	return b.String()
}

// sampleInotifyUsage counts live inotify instances by walking /proc and looking
// for file descriptors whose symlink target identifies them as inotify fds.
//
// The kernel exposes the limit but no corresponding in-use counter, so walking
// /proc is the only way to obtain the current figure. Processes that exit
// mid-walk are skipped; processes owned by another user are counted as
// unreadable so the caller can tell a complete sample from a partial one.
func sampleInotifyUsage() (inotifyUsage, error) {
	u := inotifyUsage{byUID: make(map[int]int)}
	u.maxInstances = readIntFile(inotifyMaxInstancesPath)
	u.maxWatches = readIntFile(inotifyMaxWatchesPath)

	entries, err := os.ReadDir("/proc")
	if err != nil {
		return u, fmt.Errorf("read /proc: %w", err)
	}

	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue // not a process directory
		}

		fdDir := filepath.Join("/proc", e.Name(), "fd")
		fds, err := os.ReadDir(fdDir)
		if err != nil {
			if os.IsNotExist(err) {
				continue // process exited between readdir and now
			}
			u.unreadable++
			continue
		}
		u.scanned++

		n := 0
		for _, fd := range fds {
			target, err := os.Readlink(filepath.Join(fdDir, fd.Name()))
			if err != nil {
				continue // fd closed under us
			}
			if target == inotifyInstanceLink {
				n++
			}
		}
		if n == 0 {
			continue
		}

		uid := 0
		if st, err := os.Stat(filepath.Join("/proc", e.Name())); err == nil {
			if sys, ok := st.Sys().(*syscall.Stat_t); ok {
				uid = int(sys.Uid)
			}
		}
		u.byUID[uid] += n
		_ = pid
	}

	// The processes we were denied are the root-owned ones - the containers
	// that actually consume the budget - so a partial sample cannot answer the
	// question it was taken to answer. Retry under sudo, non-interactively so
	// this can never block or prompt. If sudo is unavailable the partial sample
	// stands and labels itself as such.
	if u.unreadable > 0 {
		if byUID, err := elevatedInotifyScan(); err == nil {
			u.byUID = byUID
			u.unreadable = 0
			u.elevated = true
		}
	}

	return u, nil
}

// elevatedInotifyScan counts inotify instances per uid using sudo for the fd
// listing, which is the only part that needs privilege. Process ownership is
// read directly: /proc/<pid> is world-readable even when /proc/<pid>/fd is not.
func elevatedInotifyScan() (map[int]int, error) {
	// -n: fail immediately rather than prompt. A test must never block on a
	// password, and on an agent without passwordless sudo this simply falls
	// back to the partial sample.
	out, err := exec.Command("sudo", "-n", "find", "/proc",
		"-maxdepth", "3",
		"-path", "/proc/[0-9]*/fd/*",
		"-lname", inotifyInstanceLink,
	).Output()
	if err != nil && len(out) == 0 {
		return nil, fmt.Errorf("elevated inotify scan: %w", err)
	}

	perPID := make(map[string]int)
	for _, line := range strings.Split(string(out), "\n") {
		// /proc/<pid>/fd/<n>
		parts := strings.Split(strings.TrimSpace(line), "/")
		if len(parts) < 5 || parts[1] != "proc" {
			continue
		}
		if _, err := strconv.Atoi(parts[2]); err != nil {
			continue
		}
		perPID[parts[2]]++
	}
	if len(perPID) == 0 {
		return nil, fmt.Errorf("elevated inotify scan: no instances found")
	}

	byUID := make(map[int]int)
	for pid, n := range perPID {
		uid := 0
		if st, err := os.Stat(filepath.Join("/proc", pid)); err == nil {
			if sys, ok := st.Sys().(*syscall.Stat_t); ok {
				uid = int(sys.Uid)
			}
		}
		byUID[uid] += n
	}
	return byUID, nil
}

// readIntFile returns the integer in a single-value /proc file, or -1.
func readIntFile(path string) int {
	raw, err := os.ReadFile(path)
	if err != nil {
		return -1
	}
	v, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil {
		return -1
	}
	return v
}

// inotifyUsageLine returns a one-line sample suitable for logging, or a reason
// the sample could not be taken. It never fails: this is diagnostic output.
func inotifyUsageLine() string {
	u, err := sampleInotifyUsage()
	if err != nil {
		return fmt.Sprintf("sample failed: %v", err)
	}
	return u.String()
}

// logInotifyUsage records the inotify budget at a named point in the run, so a
// failure can be correlated with the budget as it actually stood at the time
// rather than as it is reconstructed afterwards.
func logInotifyUsage(t *testing.T, phase string) {
	t.Helper()
	t.Logf("inotify: %s: %s", phase, inotifyUsageLine())
}
