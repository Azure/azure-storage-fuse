// Copyright (c) 2026 Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package telemetry

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

type processIdentity struct {
	PID       int    `json:"pid"`
	Start     string `json:"start"`
	Boot      string `json:"boot"`
	Namespace string `json:"namespace"`
	Host      string `json:"host,omitempty"`
}

type sessionRecord struct {
	Session  string          `json:"session"`
	Owner    processIdentity `json:"owner"`
	Updated  time.Time       `json:"updated"`
	Uptime   float64         `json:"uptime_seconds"`
	Ready    bool            `json:"ready"`
	Terminal *Event          `json:"terminal,omitempty"`
}

type stateStore struct {
	root *os.Root
	mu   sync.Mutex
	diag func(string)
}

func identity(pid int) (processIdentity, error) {
	p := processIdentity{PID: pid}
	boot, err := os.ReadFile("/proc/sys/kernel/random/boot_id")
	if err != nil {
		return p, err
	}
	p.Boot = strings.TrimSpace(string(boot))
	machine, hostErr := os.ReadFile("/etc/machine-id")
	if hostErr == nil {
		value := strings.TrimSpace(string(machine))
		if validID(value) {
			hash := sha256.Sum256([]byte(value))
			p.Host = hex.EncodeToString(hash[:])
		}
	} else if !errors.Is(hostErr, os.ErrNotExist) {
		return p, hostErr
	}
	p.Namespace, err = os.Readlink("/proc/self/ns/pid")
	if err != nil {
		return p, err
	}
	b, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return p, err
	}
	end := strings.LastIndexByte(string(b), ')')
	if end < 0 {
		return p, errors.New("invalid process identity")
	}
	fields := strings.Fields(string(b[end+1:]))
	if len(fields) < 20 {
		return p, errors.New("incomplete process identity")
	}
	p.Start = fields[19]
	return p, nil
}

// definitelyDead avoids using PIDs alone or interpreting another PID namespace.
func definitelyDead(old, current processIdentity) bool {
	if old.Boot == "" || old.Namespace == "" || old.Start == "" || old.PID <= 0 {
		return false
	}
	if old.Boot != current.Boot {
		return old.Host != "" && old.Host == current.Host
	}
	if old.Namespace != current.Namespace {
		return false
	}
	now, err := identity(old.PID)
	if errors.Is(err, os.ErrNotExist) {
		return true
	}
	return err == nil && now.Start != old.Start
}

func openStore(path string, diag func(string)) (*stateStore, error) {
	path = filepath.Clean(path)
	// Refuse a broad user-selected directory: this must be a dedicated store.
	if filepath.Base(path) != "microsoft-telemetry" {
		return nil, errors.New("telemetry state directory must end in microsoft-telemetry")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	if err := os.Mkdir(path, 0700); err != nil && !errors.Is(err, os.ErrExist) {
		return nil, err
	}
	st, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	owner, ok := st.Sys().(*syscall.Stat_t)
	if !ok || !st.IsDir() || st.Mode().Perm()&0077 != 0 || owner.Uid != uint32(os.Geteuid()) {
		return nil, errors.New("telemetry state directory must be private and owned by this user")
	}
	root, err := os.OpenRoot(path)
	if err != nil {
		return nil, err
	}
	return &stateStore{root: root, diag: diag}, nil
}

func (s *stateStore) locked(fn func() error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, err := s.root.OpenFile(".lock", os.O_CREATE|os.O_RDWR|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	deadline := time.Now().Add(250 * time.Millisecond)
	for {
		err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			break
		}
		if !errors.Is(err, syscall.EWOULDBLOCK) || !time.Now().Before(deadline) {
			return err
		}
		time.Sleep(5 * time.Millisecond)
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN) //nolint:errcheck
	return fn()
}

func (s *stateStore) read(name string, target any) error {
	f, err := s.root.OpenFile(name, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil || !st.Mode().IsRegular() || st.Size() > MaxStateRecordBytes {
		return errors.New("invalid telemetry state file")
	}
	decoder := json.NewDecoder(io.LimitReader(f, MaxStateRecordBytes+1))
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return errors.New("invalid trailing telemetry state")
	}
	return nil
}

func (s *stateStore) entries() ([]os.DirEntry, error) {
	f, err := s.root.Open(".")
	if err != nil {
		return nil, err
	}
	defer f.Close()
	entries, err := f.ReadDir(2049)
	if errors.Is(err, io.EOF) {
		err = nil
	}
	if len(entries) > 2048 {
		return nil, errors.New("telemetry state has too many directory entries")
	}
	return entries, err
}

func stateName(name string) bool {
	for _, prefix := range []string{"session-", "event-"} {
		if strings.HasPrefix(name, prefix) && strings.HasSuffix(name, ".json") {
			return validID(strings.TrimSuffix(strings.TrimPrefix(name, prefix), ".json"))
		}
	}
	return false
}

func temporaryState(name string) bool {
	return strings.HasPrefix(name, "tmp-") && validID(strings.TrimPrefix(name, "tmp-"))
}

func (s *stateStore) write(name string, value any) error {
	return s.writeWithinBudget(name, value, MaxStateBytes-RecoveryReserveBytes)
}

func (s *stateStore) writeWithinBudget(name string, value any, limit int64) error {
	body, err := json.Marshal(value)
	if err != nil || len(body) > MaxStateRecordBytes {
		return errors.New("invalid telemetry state payload")
	}
	entries, err := s.entries()
	if err != nil {
		return err
	}
	total, count := int64(len(body)), 0
	for _, entry := range entries {
		if temporaryState(entry.Name()) {
			if err := s.root.Remove(entry.Name()); err != nil {
				return err
			}
			continue
		}
		if !stateName(entry.Name()) || entry.Name() == name {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return err
		}
		if time.Since(info.ModTime()) > Retention {
			if err := s.root.Remove(entry.Name()); err != nil {
				return err
			}
			continue
		}
		total += info.Size()
		count++
	}
	if total > limit || count >= MaxStateRecords {
		return errors.New("telemetry state capacity exceeded")
	}
	tmp := "tmp-" + newID()
	f, err := s.root.OpenFile(tmp, os.O_CREATE|os.O_EXCL|os.O_WRONLY|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		return err
	}
	defer func() {
		if err := s.root.Remove(tmp); err != nil && !errors.Is(err, os.ErrNotExist) {
			s.diag("telemetry: cannot remove temporary state")
		}
	}()
	_, err = f.Write(body)
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if err := s.root.Rename(tmp, name); err != nil {
		return err
	}
	return s.syncDirectory()
}

func (s *stateStore) checkpoint(record sessionRecord) error {
	return s.locked(func() error {
		return s.write("session-"+record.Session+".json", record)
	})
}

// Consume evidence durably before dispatch, including when admission drops it.
// Requiring retained evidence prevents concurrent startups sending it twice.
func (s *stateStore) consumeEvent(event Event, requireRetained bool) error {
	expected, err := encode(event, "validation")
	if err != nil {
		return err
	}
	return s.locked(func() error {
		var retained Event
		name := "event-" + event.Properties["event_id"] + ".json"
		err := s.read(name, &retained)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if requireRetained && errors.Is(err, os.ErrNotExist) {
			target, err := terminalSession(event)
			if err != nil {
				return err
			}
			if target == "" {
				return errors.New("retained telemetry event already consumed")
			}
			var record sessionRecord
			if err := s.read("session-"+target+".json", &record); err != nil {
				return err
			}
			if record.Terminal == nil {
				return errors.New("retained telemetry event lacks terminal evidence")
			}
		}
		if err == nil {
			actual, err := encode(retained, "validation")
			if err != nil || !bytes.Equal(actual, expected) {
				return errors.New("conflicting retained telemetry event")
			}
		}
		// Recovery can be consumed directly from a terminal checkpoint,
		// without allocating a separate event file.
		if err := s.retireTerminalSession(event); err != nil {
			// Keep the event: it may be the only durable evidence of teardown
			// if the final session checkpoint could not be updated.
			return err
		}
		return s.removeDurable(name)
	})
}

func terminalSession(event Event) (string, error) {
	session := event.Properties["mount_session_id"]
	switch event.Name {
	case "blobfuse.mount.stopped":
	case "blobfuse.mount.result":
		if event.Properties["outcome"] != "failure" {
			return "", nil
		}
	case "blobfuse.mount.recovery":
		session = event.Properties["previous_session_id"]
		if session == event.Properties["mount_session_id"] {
			return "", errors.New("recovery event cannot terminate its reporting session")
		}
	default:
		return "", nil
	}
	if !validID(session) {
		return "", errors.New("invalid terminal telemetry session")
	}
	return session, nil
}

func (s *stateStore) retireTerminalSession(event Event) error {
	session, err := terminalSession(event)
	if err != nil || session == "" {
		return err
	}
	name := "session-" + session + ".json"
	var record sessionRecord
	if err := s.read(name, &record); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// A prior cleanup may have removed the session but failed before
			// synchronizing it. Persist its absence before dropping evidence.
			return s.syncDirectory()
		}
		return err
	}
	if record.Session != session {
		return errors.New("terminal event does not match retained session")
	}
	if record.Terminal != nil {
		retained, err := encode(*record.Terminal, "validation")
		expected, expectedErr := encode(event, "validation")
		if err != nil || expectedErr != nil || !bytes.Equal(retained, expected) {
			return errors.New("conflicting terminal telemetry evidence")
		}
	}
	if event.Name == "blobfuse.mount.recovery" {
		// Recovery is recorded durably in the old session before dispatch.
		// It must never retire the new session performing the observation.
		if record.Terminal == nil {
			return errors.New("recovery event lacks matching retained terminal evidence")
		}
	} else if event.Name == "blobfuse.mount.result" && record.Ready {
		return errors.New("startup failure conflicts with mounted session")
	}
	return s.removeDurable(name)
}

func (s *stateStore) syncDirectory() error {
	dir, err := s.root.Open(".")
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}

func (s *stateStore) removeDurable(name string) error {
	if err := s.root.Remove(name); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return s.syncDirectory()
}

func (s *stateStore) purge() error {
	return s.locked(func() error {
		entries, err := s.entries()
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if stateName(entry.Name()) || temporaryState(entry.Name()) {
				if err := s.root.Remove(entry.Name()); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func (s *stateStore) recover(current processIdentity, session string, makeEvent func(string) Event) ([]Event, error) {
	var pending []Event
	err := s.locked(func() error {
		entries, err := s.entries()
		if err != nil {
			return err
		}
		seen := map[string]bool{}
		created := 0
		terminal := map[string]Event{}
		var recoveryErr error
		var sessions []os.DirEntry
		// Collect legacy pending events first: inability to update one session must
		// not hide already durable work that can release capacity.
		for _, entry := range entries {
			if !stateName(entry.Name()) {
				continue
			}
			info, err := entry.Info()
			if err != nil {
				if !errors.Is(err, os.ErrNotExist) {
					recoveryErr = errors.Join(recoveryErr, err)
				}
				continue
			}
			if time.Since(info.ModTime()) > Retention {
				if err := s.removeDurable(entry.Name()); err != nil {
					recoveryErr = errors.Join(recoveryErr, err)
				}
				continue
			}
			if strings.HasPrefix(entry.Name(), "session-") {
				sessions = append(sessions, entry)
				continue
			}
			var e Event
			if err := s.read(entry.Name(), &e); err != nil {
				s.diag("telemetry: unreadable pending event")
				continue
			}
			if _, err := encode(e, "validation"); err != nil ||
				entry.Name() != "event-"+e.Properties["event_id"]+".json" {
				s.diag("telemetry: invalid pending event")
				continue
			}
			if !seen[e.Properties["event_id"]] {
				seen[e.Properties["event_id"]] = true
				pending = append(pending, e)
			}
			if target, err := terminalSession(e); err == nil && target != "" {
				terminal[target] = e
			}
		}
		for _, entry := range sessions {
			var record sessionRecord
			if err := s.read(entry.Name(), &record); err != nil {
				if !errors.Is(err, os.ErrNotExist) {
					s.diag("telemetry: unreadable session state")
				}
				continue
			}
			if entry.Name() != "session-"+record.Session+".json" || record.Session == session ||
				!validID(record.Session) || !definitelyDead(record.Owner, current) {
				continue
			}
			if record.Terminal == nil {
				if _, known := terminal[record.Session]; known {
					// The separate pending terminal event is sufficient
					// evidence and was already collected above.
					continue
				}
				if created >= 8 {
					s.diag("telemetry: additional unfinished sessions deferred to another startup")
					continue
				}
				created++
				e := makeEvent("recovery")
				e.Properties["previous_session_id"] = record.Session
				e.Properties["termination_class"] = "unclean_unknown"
				e.Properties["previous_ready"] = strconv.FormatBool(record.Ready)
				e.Measurements["previous_last_known_uptime_seconds"] = record.Uptime
				record.Terminal = &e
				if err := s.writeWithinBudget(entry.Name(), record, MaxStateBytes); err != nil {
					recoveryErr = errors.Join(recoveryErr, err)
					continue
				}
			}
			e := *record.Terminal
			target, err := terminalSession(e)
			if err != nil || target != record.Session {
				s.diag("telemetry: invalid terminal checkpoint")
				continue
			}
			if _, err := encode(e, "validation"); err != nil {
				s.diag("telemetry: invalid terminal checkpoint event")
				continue
			}
			// Dispatch directly from this durable checkpoint: never allocate a
			// second file merely to make recovery eligible for delivery.
			if !seen[e.Properties["event_id"]] {
				seen[e.Properties["event_id"]] = true
				pending = append(pending, e)
			}
		}
		return recoveryErr
	})
	return pending, err
}
