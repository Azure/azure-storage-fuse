// Copyright (c) 2026 Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package telemetry

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func mountStore(t *testing.T, base, mount, label string) *stateStore {
	t.Helper()
	store, err := openMountStore(base, mount, label, true, func(string) {})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.root.Close() })
	return store
}

func TestMountOptOutIsolation(t *testing.T) {
	base := filepath.Join(t.TempDir(), "microsoft-telemetry")
	a := mountStore(t, base, "/test/mount-A", "")
	b := mountStore(t, base, "/test/mount-B", "")
	eventA, eventB := testEvent(), testEvent()
	for store, event := range map[*stateStore]Event{a: eventA, b: eventB} {
		if err := store.persistEvent(event); err != nil {
			t.Fatal(err)
		}
	}
	keyBefore, err := os.ReadFile(filepath.Join(base, ".scope-key"))
	if err != nil {
		t.Fatal(err)
	}
	s, err := Start(Options{Disabled: true, StateDirectory: base, MountPath: "/test/mount-A"})
	if err != nil || s != nil {
		t.Fatalf("opt out: %v", err)
	}
	var result Event
	if err := a.read("event-"+eventA.Properties["event_id"]+".json", &result); !os.IsNotExist(err) {
		t.Fatalf("mount A record was not removed: %v", err)
	}
	if err := b.read("event-"+eventB.Properties["event_id"]+".json", &result); err != nil {
		t.Fatalf("mount A opt-out affected mount B: %v", err)
	}
	keyAfter, err := os.ReadFile(filepath.Join(base, ".scope-key"))
	if err != nil || !bytes.Equal(keyBefore, keyAfter) {
		t.Fatal("opt-out replaced the local scope identity")
	}
}

func TestMountScopesCanonicalAndPersistent(t *testing.T) {
	work := t.TempDir()
	target := filepath.Join(work, "target")
	alias := filepath.Join(work, "alias")
	if err := os.Mkdir(target, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, alias); err != nil {
		t.Fatal(err)
	}
	base := filepath.Join(work, "microsoft-telemetry")
	first := mountStore(t, base, filepath.Join(target, "not-created-yet"), "")
	second := mountStore(t, base, filepath.Join(alias, "not-created-yet"), "")
	if first.root.Name() != second.root.Name() {
		t.Fatal("symlink aliases generated different mount identities")
	}
	otherProfile := mountStore(t, base, filepath.Join(target, "not-created-yet"), "different-container")
	if first.root.Name() == otherProfile.root.Name() {
		t.Fatal("independent deployment scopes were merged")
	}
	explicit := mountStore(t, base, target, "stable-volume")
	reopened := mountStore(t, base, target, "stable-volume")
	if explicit.root.Name() != reopened.root.Name() {
		t.Fatal("explicit mount scope changed on reopen")
	}
	for _, path := range []string{first.root.Name(), otherProfile.root.Name(), explicit.root.Name()} {
		st, err := os.Stat(path)
		if err != nil || st.Mode().Perm() != 0700 {
			t.Fatalf("unsafe scope permissions: %v", err)
		}
		name := filepath.Base(path)
		if !strings.HasPrefix(name, "mount-") || strings.Contains(name, "target") || strings.Contains(name, "stable-volume") {
			t.Fatal("raw mount identity appeared in a scope name")
		}
	}
}

func TestConcurrentScopeCreation(t *testing.T) {
	base := filepath.Join(t.TempDir(), "microsoft-telemetry")
	var workers sync.WaitGroup
	names := make(chan string, 8)
	for range 8 {
		workers.Go(func() {
			store, err := openMountStore(base, "/test/shared-target", "", true, func(string) {})
			if err != nil {
				t.Error(err)
				return
			}
			names <- store.root.Name()
			store.root.Close()
		})
	}
	workers.Wait()
	close(names)
	expected := ""
	count := 0
	for name := range names {
		if expected != "" && name != expected {
			t.Fatal("concurrent startups created inconsistent mount scopes")
		}
		expected = name
		count++
	}
	if count != 8 {
		t.Fatalf("only %d scopes opened", count)
	}
}

func TestScopedOptOutDoesNotCreateState(t *testing.T) {
	base := filepath.Join(t.TempDir(), "microsoft-telemetry")
	if _, err := Start(Options{Disabled: true, StateDirectory: base, MountPath: "/test/new"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(base); !os.IsNotExist(err) {
		t.Fatal("opt-out created state")
	}
}

func TestScopedStoreLeavesLegacyAndOtherNamespacesUntouched(t *testing.T) {
	base := filepath.Join(t.TempDir(), "microsoft-telemetry")
	legacy, err := openStore(base, func(string) {})
	if err != nil {
		t.Fatal(err)
	}
	defer legacy.root.Close()
	e := testEvent()
	if err := legacy.persistEvent(e); err != nil {
		t.Fatal(err)
	}
	a := mountStore(t, base, "/test/same-path", "container-A")
	b := mountStore(t, base, "/test/same-path", "container-B")
	if a.root.Name() == b.root.Name() {
		t.Fatal("container scopes collided")
	}
	if _, err := Start(Options{Disabled: true, StateDirectory: base, MountPath: "/test/same-path", MountScope: "container-A"}); err != nil {
		t.Fatal(err)
	}
	var retained Event
	if err := legacy.read("event-"+e.Properties["event_id"]+".json", &retained); err != nil {
		t.Fatal("legacy records were deleted without an ownership mapping")
	}
}

func TestScopeKeyProtection(t *testing.T) {
	base := filepath.Join(t.TempDir(), "microsoft-telemetry")
	_ = mountStore(t, base, "/test/key", "")
	path := filepath.Join(base, ".scope-key")
	if err := os.Chmod(path, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := openMountStore(base, "/test/key", "", true, func(string) {}); err == nil {
		t.Fatal("accepted an unprotected scope key")
	}
}
