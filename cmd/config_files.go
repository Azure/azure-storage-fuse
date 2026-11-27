// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/Azure/azure-storage-fuse/v2/common"
)

func absoluteConfigPath(path string) (string, error) {
	if path == "" {
		return "", errors.New("config path is empty")
	}
	// Cleaning before validation can hide symlinks in paths containing "..".
	for part := range strings.SplitSeq(path, string(os.PathSeparator)) {
		if part == ".." {
			return "", fmt.Errorf("config path %q must not contain a '..' component", path)
		}
	}
	return filepath.Abs(path)
}

func expandConfigWorkDir(path string) (string, error) {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		path = home + path[1:]
	}
	return absoluteConfigPath(common.ExpandPathEnv(path))
}

func ensurePrivateConfigDir(path string) error {
	path, err := expandConfigWorkDir(path)
	if err != nil {
		return err
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

func writePrivateConfig(path string, syncDirectory bool, write func(io.Writer) error) error {
	if name := path[strings.LastIndexByte(path, os.PathSeparator)+1:]; name == "" || name == "." {
		return fmt.Errorf("config path %q must name a file", path)
	}
	path, err := absoluteConfigPath(path)
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
		if info.Mode().Perm()&0200 == 0 {
			return fmt.Errorf("refusing to replace %q: owner write permission is not set: %w", path, os.ErrPermission)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	var outputDir *os.File
	if syncDirectory {
		// Check that we can open the directory before replacing the destination.
		outputDir, err = os.Open(dir)
		if err != nil {
			return fmt.Errorf("cannot open config output directory %q for sync: %w", dir, err)
		}
		defer outputDir.Close()
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
	if err := os.Rename(file.Name(), path); err != nil {
		return err
	}
	if outputDir != nil {
		if err := outputDir.Sync(); err != nil {
			return fmt.Errorf("cannot sync config output directory %q: %w", dir, err)
		}
	}
	return nil
}
