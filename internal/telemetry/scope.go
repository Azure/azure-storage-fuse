// Copyright (c) 2026 Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package telemetry

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// canonicalMount resolves existing symlink parents even for a failed attempt
// whose final mount directory does not exist yet. Paths never leave this host.
func canonicalMount(path string) (string, error) {
	if path == "" {
		return "", errors.New("telemetry requires a mount path for state ownership")
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	current := filepath.Clean(absolute)
	var suffix []string
	for {
		resolved, err := filepath.EvalSymlinks(current)
		if err == nil {
			for i := len(suffix) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, suffix[i])
			}
			return resolved, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", err
		}
		suffix = append(suffix, filepath.Base(current))
		current = parent
	}
}

func scopeKey(base *stateStore, create bool) ([]byte, error) {
	f, err := base.root.OpenFile(".scope-key", os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err == nil {
		defer f.Close()
		st, err := f.Stat()
		if err != nil || !st.Mode().IsRegular() || st.Mode().Perm()&0077 != 0 || st.Size() != 32 {
			return nil, errors.New("invalid private telemetry scope key")
		}
		key, err := io.ReadAll(io.LimitReader(f, 33))
		if err != nil || len(key) != 32 {
			return nil, errors.New("cannot read telemetry scope key")
		}
		return key, nil
	}
	if !errors.Is(err, os.ErrNotExist) || !create {
		return nil, err
	}
	entries, err := base.entries()
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "mount-") {
			return nil, errors.New("telemetry scope key missing for existing mount stores")
		}
	}
	key := make([]byte, 32)
	_, _ = rand.Read(key)
	name := "tmp-" + newID()
	f, err = base.root.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := base.root.Remove(name); err != nil && !errors.Is(err, os.ErrNotExist) {
			base.diag("telemetry: cannot remove temporary scope key")
		}
	}()
	_, err = f.Write(key)
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return nil, err
	}
	if closeErr != nil {
		return nil, closeErr
	}
	if err := base.root.Rename(name, ".scope-key"); err != nil {
		return nil, err
	}
	directory, err := base.root.Open(".")
	if err != nil {
		return nil, err
	}
	defer directory.Close()
	if err := directory.Sync(); err != nil {
		return nil, err
	}
	return key, nil
}

func openMountStore(path, mountPath, stableScope string, create bool, diag func(string)) (*stateStore, error) {
	if !create {
		if _, err := os.Lstat(path); errors.Is(err, os.ErrNotExist) {
			return nil, nil
		} else if err != nil {
			return nil, err
		}
	}
	canonical, err := canonicalMount(mountPath)
	if err != nil {
		return nil, err
	}
	if len(stableScope) > 256 {
		return nil, errors.New("telemetry scope label exceeds limit")
	}
	context := "profile:" + stableScope
	if stableScope == "" {
		namespace, err := os.Readlink("/proc/self/ns/mnt")
		if err != nil {
			return nil, errors.New("telemetry mount namespace unavailable")
		}
		context = "namespace:" + namespace
	}
	base, err := openStore(path, diag)
	if err != nil {
		return nil, err
	}
	defer base.root.Close()
	var store *stateStore
	err = base.locked(func() error {
		if entries, err := base.entries(); err == nil {
			for _, entry := range entries {
				if stateName(entry.Name()) {
					diag("telemetry: legacy unscoped records retained; automatic migration is disabled")
					break
				}
			}
		} else {
			diag("telemetry: cannot inspect legacy state")
		}
		key, err := scopeKey(base, create)
		if !create && errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		hash := hmac.New(sha256.New, key)
		_, _ = hash.Write([]byte("blobfuse-mount-v1\x00" + context + "\x00" + canonical))
		name := "mount-" + hex.EncodeToString(hash.Sum(nil))
		if create {
			if err := base.root.Mkdir(name, 0700); err != nil && !errors.Is(err, os.ErrExist) {
				return err
			}
		}
		st, err := base.root.Lstat(name)
		if !create && errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		owner, ok := st.Sys().(*syscall.Stat_t)
		if !ok || !st.IsDir() || st.Mode().Perm()&0077 != 0 || owner.Uid != uint32(os.Geteuid()) {
			return errors.New("telemetry mount store must be private and owned by this user")
		}
		root, err := base.root.OpenRoot(name)
		if err != nil {
			return err
		}
		store = &stateStore{root: root, diag: diag}
		return nil
	})
	return store, err
}
