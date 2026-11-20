// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"
)

func ensureWorkDir(path string) error {
	if path == "" {
		return errors.New("work directory is empty")
	}
	info, err := checkConfigDir(path, true)
	if err != nil {
		return err
	}
	if info.Sys().(*syscall.Stat_t).Uid != uint32(os.Geteuid()) || info.Mode().Perm()&0022 != 0 {
		return fmt.Errorf("work directory %q must be owned by the current user and not writable by group or others", path)
	}
	return nil
}

func checkConfigDir(path string, create bool) (os.FileInfo, error) {
	// Check ancestors first so validated entries cannot be swapped out by another user.
	if parent := filepath.Dir(path); parent != path {
		if _, err := checkConfigDir(parent, create); err != nil {
			return nil, err
		}
	}
	info, err := os.Lstat(path)
	if create && errors.Is(err, os.ErrNotExist) {
		if err := os.Mkdir(path, 0700); err != nil && !errors.Is(err, os.ErrExist) {
			return nil, err
		}
		info, err = os.Lstat(path)
	}
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%q must be a directory, not a symlink or other file", path)
	}
	uid := info.Sys().(*syscall.Stat_t).Uid
	// A sticky directory owned by a trusted user (for example /tmp) protects the entries we own.
	if (uid != 0 && uid != uint32(os.Geteuid())) ||
		(info.Mode().Perm()&0022 != 0 && info.Mode()&os.ModeSticky == 0) {
		return nil, fmt.Errorf("directory %q must be owned by the current user or root and not writable by group or others unless sticky", path)
	}
	return info, nil
}

func writePrivateConfig(path string, write func(io.Writer) error) error {
	path, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if _, err = checkConfigDir(dir, false); err != nil {
		return err
	}
	if info, err := os.Lstat(path); err == nil {
		if !info.Mode().IsRegular() || info.Sys().(*syscall.Stat_t).Uid != uint32(os.Geteuid()) {
			return fmt.Errorf("refusing to replace %q: expected a regular file owned by the current user", path)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	// CreateTemp opens a new file with mode 0600. Rename replaces the old inode
	// without writing into a symlink, hard link, or legacy world-readable file.
	file, err := os.CreateTemp(dir, ".blobfuse-config-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())

	err = write(file)
	if err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(file.Name(), path)
}
