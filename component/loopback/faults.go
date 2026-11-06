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

package loopback

import (
	"errors"
	"fmt"
	"path"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common/log"

	"golang.org/x/sys/unix"
)

// Op is the name of a storage operation of loopbackfs, which a Fault can target.
type Op string

// Operations of loopbackfs that a Fault can target. They are named after the methods of the component.
const (
	OpAny                   Op = "*"
	OpCreateDir             Op = "CreateDir"
	OpDeleteDir             Op = "DeleteDir"
	OpIsDirEmpty            Op = "IsDirEmpty"
	OpReadDir               Op = "ReadDir"
	OpStreamDir             Op = "StreamDir"
	OpRenameDir             Op = "RenameDir"
	OpCreateFile            Op = "CreateFile"
	OpDeleteFile            Op = "DeleteFile"
	OpOpenFile              Op = "OpenFile"
	OpReleaseFile           Op = "ReleaseFile"
	OpFlushFile             Op = "FlushFile"
	OpRenameFile            Op = "RenameFile"
	OpReadFile              Op = "ReadFile"
	OpReadInBuffer          Op = "ReadInBuffer"
	OpWriteFile             Op = "WriteFile"
	OpTruncateFile          Op = "TruncateFile"
	OpCopyToFile            Op = "CopyToFile"
	OpCopyFromFile          Op = "CopyFromFile"
	OpCreateLink            Op = "CreateLink"
	OpReadLink              Op = "ReadLink"
	OpGetAttr               Op = "GetAttr"
	OpChmod                 Op = "Chmod"
	OpChown                 Op = "Chown"
	OpStageData             Op = "StageData"
	OpCommitData            Op = "CommitData"
	OpGetCommittedBlockList Op = "GetCommittedBlockList"
	OpGetFileBlockOffsets   Op = "GetFileBlockOffsets"
)

var knownOps = []Op{
	OpCreateDir, OpDeleteDir, OpIsDirEmpty, OpReadDir, OpStreamDir, OpRenameDir,
	OpCreateFile, OpDeleteFile, OpOpenFile, OpReleaseFile, OpFlushFile, OpRenameFile,
	OpReadFile, OpReadInBuffer, OpWriteFile, OpTruncateFile, OpCopyToFile, OpCopyFromFile,
	OpCreateLink, OpReadLink, OpGetAttr, OpChmod, OpChown,
	OpStageData, OpCommitData, OpGetCommittedBlockList, OpGetFileBlockOffsets,
}

// Fault is an error or a delay that loopbackfs injects into its operations, to emulate failures and slowness of
// Azure Storage. A call that a fault applies to waits for the delay and then fails with the error without doing
// anything, or runs normally if the fault has no error. IsDirEmpty, which cannot fail, reports false instead.
type Fault struct {
	// Op is the operation the fault applies to. An empty Op, or OpAny, matches every operation.
	Op Op
	// Path is a pattern, in the syntax of path.Match, for the objects the fault applies to, for example
	// "dir/*.bin". Like the names of objects, it has no leading slash. An empty Path matches every object.
	// Renames match their source.
	Path string
	// Err is the error returned by the calls the fault applies to. A nil Err only delays them.
	Err error
	// Delay is how long the calls the fault applies to wait before they run or fail.
	Delay time.Duration
	// Skip is the number of matching calls that the fault lets through before it starts to apply.
	Skip int
	// Count is the number of calls the fault applies to after the skipped ones. 0 applies it to all of them.
	Count int
}

func (f Fault) validate() error {
	if f.Op != "" && f.Op != OpAny && !slices.Contains(knownOps, f.Op) {
		return fmt.Errorf("unknown operation %q", f.Op)
	}

	if _, err := path.Match(f.Path, ""); err != nil {
		return fmt.Errorf("invalid path pattern %q: %w", f.Path, err)
	}

	if f.Delay < 0 || f.Skip < 0 || f.Count < 0 {
		return errors.New("delay, skip and count cannot be negative")
	}

	if f.Err == nil && f.Delay == 0 {
		return errors.New("a fault needs an error or a delay")
	}

	return nil
}

// faultRule is a fault and the number of calls it has seen.
type faultRule struct {
	fault   Fault
	pattern string
	matched int
	applied int
}

func newFaultRule(f Fault) *faultRule {
	return &faultRule{fault: f, pattern: strings.Trim(f.Path, "/")}
}

func (r *faultRule) matches(op Op, name string) bool {
	if r.fault.Op != "" && r.fault.Op != OpAny && r.fault.Op != op {
		return false
	}

	if r.pattern == "" {
		return true
	}

	matched, _ := path.Match(r.pattern, name)
	return matched
}

// take counts a call that the rule matches and reports whether the rule applies to it.
func (r *faultRule) take() bool {
	r.matched++
	if r.matched <= r.fault.Skip {
		return false
	}

	if r.fault.Count > 0 && r.applied >= r.fault.Count {
		return false
	}

	r.applied++
	return true
}

// faultInjector holds the faults of the component: those from its configuration, which a configuration change
// replaces, and those injected by InjectFault.
type faultInjector struct {
	mu         sync.Mutex
	configured []*faultRule
	injected   []*faultRule
	count      atomic.Int32
}

// check applies the faults that match a call of op on the object name: it waits for their delays and returns the
// error of the first of them that has one.
func (fi *faultInjector) check(op Op, name string) error {
	if fi.count.Load() == 0 {
		return nil
	}

	var delay time.Duration
	var err error

	fi.mu.Lock()
	for _, rules := range [][]*faultRule{fi.configured, fi.injected} {
		for _, rule := range rules {
			if !rule.matches(op, name) || !rule.take() {
				continue
			}

			delay += rule.fault.Delay
			if err == nil {
				err = rule.fault.Err
			}
		}
	}
	fi.mu.Unlock()

	if delay > 0 {
		time.Sleep(delay)
	}

	if err != nil {
		log.Info("LoopbackFS::check : injecting %v into %s of %s", err, op, name)
	}

	return err
}

func (fi *faultInjector) setConfigured(rules []*faultRule) {
	fi.mu.Lock()
	defer fi.mu.Unlock()

	fi.configured = rules
	fi.count.Store(int32(len(fi.configured) + len(fi.injected)))
}

func (fi *faultInjector) inject(rule *faultRule) {
	fi.mu.Lock()
	defer fi.mu.Unlock()

	fi.injected = append(fi.injected, rule)
	fi.count.Store(int32(len(fi.configured) + len(fi.injected)))
}

func (fi *faultInjector) remove(rule *faultRule) {
	fi.mu.Lock()
	defer fi.mu.Unlock()

	fi.injected = slices.DeleteFunc(fi.injected, func(r *faultRule) bool { return r == rule })
	fi.count.Store(int32(len(fi.configured) + len(fi.injected)))
}

func (fi *faultInjector) clearInjected() {
	fi.mu.Lock()
	defer fi.mu.Unlock()

	fi.injected = nil
	fi.count.Store(int32(len(fi.configured)))
}

// InjectFault makes the operations that match f fail or wait, as f describes, until the returned function is called.
// Tests use it to emulate failures of Azure Storage; mounts configure faults in the faults option instead.
func (lfs *LoopbackFS) InjectFault(f Fault) (remove func(), err error) {
	if err := f.validate(); err != nil {
		return nil, fmt.Errorf("loopbackfs: invalid fault: %w", err)
	}

	rule := newFaultRule(f)
	lfs.faults.inject(rule)

	return func() { lfs.faults.remove(rule) }, nil
}

// ClearFaults removes the faults added by InjectFault. The faults from the configuration stay.
func (lfs *LoopbackFS) ClearFaults() {
	lfs.faults.clearInjected()
}

// FaultOptions configures a fault in the faults option of the component. See Fault for the meaning of the fields.
type FaultOptions struct {
	Op      string `config:"op" yaml:"op,omitempty"`
	Path    string `config:"path" yaml:"path,omitempty"`
	Error   string `config:"error" yaml:"error,omitempty"`
	DelayMs uint32 `config:"delay-ms" yaml:"delay-ms,omitempty"`
	Skip    uint32 `config:"skip" yaml:"skip,omitempty"`
	Count   uint32 `config:"count" yaml:"count,omitempty"`
}

func (o FaultOptions) fault() (Fault, error) {
	f := Fault{
		Op:    Op(o.Op),
		Path:  o.Path,
		Delay: time.Duration(o.DelayMs) * time.Millisecond,
		Skip:  int(o.Skip),
		Count: int(o.Count),
	}

	if o.Error != "" {
		errno, err := parseErrno(o.Error)
		if err != nil {
			return Fault{}, err
		}
		f.Err = errno
	}

	return f, f.validate()
}

// errnoByName maps the names of errno values, such as "EIO", to the values.
var errnoByName = func() map[string]syscall.Errno {
	names := map[string]syscall.Errno{
		"ENOTSUP":     syscall.ENOTSUP,
		"EOPNOTSUPP":  syscall.EOPNOTSUPP,
		"EAGAIN":      syscall.EAGAIN,
		"EWOULDBLOCK": syscall.EWOULDBLOCK,
		"EDEADLK":     syscall.EDEADLK,
		"EDEADLOCK":   syscall.EDEADLOCK,
	}
	for errno := syscall.Errno(1); errno < 256; errno++ {
		if name := unix.ErrnoName(errno); name != "" {
			names[name] = errno
		}
	}
	return names
}()

// parseErrno returns the errno value named name, such as EIO.
func parseErrno(name string) (syscall.Errno, error) {
	if errno, ok := errnoByName[strings.ToUpper(strings.TrimSpace(name))]; ok {
		return errno, nil
	}
	return 0, fmt.Errorf("unknown error %q, use the name of an errno value such as EIO or ENOENT", name)
}
