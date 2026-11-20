// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

// Package cachefs provides symlink-safe access to the local disk cache of file_cache and block_cache.
//
// Blobfuse2 usually runs as root, so a symbolic or hard link planted in the cache could otherwise
// redirect a cache write to an arbitrary file. To prevent that:
//   - OpenRoot makes the cache root private to the blobfuse2 user (owner = effective uid, mode 0700)
//     and returns an *os.Root that every other function resolves names against.
//   - Every path component is opened with O_NOFOLLOW, so no symbolic link under the cache is ever
//     followed, and names that are not local to the root are rejected.
//   - Files are validated with fstat on the opened descriptor, not with a separate stat beforehand:
//     only regular files with a single link are used, and O_TRUNC is applied after validation.
//   - Cache sub-directories are created with mode 0700 and file modes are limited to permission bits.
package cachefs

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"golang.org/x/sys/unix"
)

// DirPerm is the mode of the cache root and every directory created under it.
const DirPerm os.FileMode = 0700

// ErrUnsafeEntry is returned for a name that is not local to the cache root, and for a cache entry
// that is a symbolic link, is not a regular file where one is required, or has more than one link.
var ErrUnsafeEntry = errors.New("unsafe cache entry")

// OpenRoot opens the cache directory at path and makes it private to the blobfuse2 process: it must
// be owned by the effective user and accessible only by that user, regardless of allow-other, since
// other users reach cached data only through the mount. A directory owned by another user is taken
// over when running as root, otherwise OpenRoot fails.
func OpenRoot(path string) (*os.Root, error) {
	root, err := os.OpenRoot(path)
	if err != nil {
		return nil, err
	}

	if err = securePrivateDir(root, path); err != nil {
		_ = root.Close()
		return nil, err
	}
	return root, nil
}

func securePrivateDir(root *os.Root, path string) error {
	dir, err := root.Open(".")
	if err != nil {
		return err
	}
	defer dir.Close()

	info, err := dir.Stat()
	if err != nil {
		return err
	}
	st := info.Sys().(*syscall.Stat_t)

	uid := os.Geteuid()
	if int(st.Uid) != uid {
		if err = dir.Chown(uid, os.Getegid()); err != nil {
			return fmt.Errorf("cache directory %s is owned by uid %d, expected uid %d [%w]", path, st.Uid, uid, err)
		}
		log.Warn("cachefs::OpenRoot : cache directory %s was owned by uid %d, changed owner to uid %d", path, st.Uid, uid)
	}

	if info.Mode().Perm() != DirPerm {
		if err = dir.Chmod(DirPerm); err != nil {
			return fmt.Errorf("failed to restrict permissions of cache directory %s [%w]", path, err)
		}
		log.Warn("cachefs::OpenRoot : cache directory %s had mode %v, changed to %v", path, info.Mode().Perm(), DirPerm)
	}
	return nil
}

// RelPath returns p, a path under the cache root, relative to root. A path outside root keeps a
// leading ".." and is rejected by the other functions in this package.
func RelPath(root *os.Root, p string) string {
	rel, err := filepath.Rel(root.Name(), p)
	if err != nil {
		return p
	}
	return rel
}

func pathError(op, name string, err error) error {
	return &fs.PathError{Op: op, Path: name, Err: err}
}

// rootFd returns a new descriptor for the cache root. The caller closes it.
func rootFd(root *os.Root, op, name string) (int, error) {
	if !filepath.IsLocal(name) {
		return -1, pathError(op, name, ErrUnsafeEntry)
	}

	rootDir, err := root.Open(".")
	if err != nil {
		return -1, err
	}
	defer rootDir.Close()

	fd, err := unix.FcntlInt(rootDir.Fd(), unix.F_DUPFD_CLOEXEC, 0)
	if err != nil {
		return -1, pathError(op, name, err)
	}
	return fd, nil
}

// openDirTraverse opens directory part relative to dirfd for path traversal only, without following a
// symbolic link. An O_PATH descriptor needs search permission on the directory, as path lookup does,
// not read permission.
func openDirTraverse(dirfd int, part string) (int, error) {
	return unix.Openat(dirfd, part, unix.O_PATH|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
}

// openDirNoFollow opens directory part relative to dirfd without following a symbolic link.
func openDirNoFollow(dirfd int, part string) (int, error) {
	return unix.Openat(dirfd, part, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
}

// checkTrustedDir fails unless the directory behind fd is owned by the effective uid and no other
// user can write to it. Only trusted directories may be walked, so a directory another local user
// could modify, or move children out of, is never used as a parent.
func checkTrustedDir(fd int, op, name string) error {
	var st unix.Stat_t
	if err := unix.Fstat(fd, &st); err != nil {
		return pathError(op, name, err)
	}
	if int(st.Uid) != os.Geteuid() || st.Mode&(unix.S_IWGRP|unix.S_IWOTH) != 0 {
		return pathError(op, name, unix.EACCES)
	}
	return nil
}

// openTrustedDirTraverse opens directory part for traversal and checks that it is trusted.
func openTrustedDirTraverse(dirfd int, part, op, name string) (int, error) {
	fd, err := openDirTraverse(dirfd, part)
	if err != nil {
		return -1, pathError(op, name, err)
	}
	if err = checkTrustedDir(fd, op, name); err != nil {
		_ = unix.Close(fd)
		return -1, err
	}
	return fd, nil
}

// openParent returns a descriptor for the directory containing name and the final component of
// name. Every directory on the way is opened with O_NOFOLLOW. The caller closes the descriptor.
func openParent(root *os.Root, op, name string) (int, string, error) {
	fd, err := rootFd(root, op, name)
	if err != nil {
		return -1, "", err
	}

	dir, base := filepath.Split(filepath.Clean(name))
	for part := range strings.SplitSeq(dir, string(filepath.Separator)) {
		if part == "" {
			continue
		}
		next, err := openTrustedDirTraverse(fd, part, op, name)
		_ = unix.Close(fd)
		if err != nil {
			return -1, "", err
		}
		fd = next
	}
	return fd, base, nil
}

// openNoFollow opens the final component of name with O_NOFOLLOW added to flag.
func openNoFollow(root *os.Root, op, name string, flag int, perm uint32) (int, error) {
	dirfd, base, err := openParent(root, op, name)
	if err != nil {
		return -1, err
	}
	defer unix.Close(dirfd)

	fd, err := unix.Openat(dirfd, base, flag|unix.O_NOFOLLOW|unix.O_CLOEXEC, perm)
	if err != nil {
		return -1, pathError(op, name, err)
	}
	return fd, nil
}

// checkRegular verifies that fd refers to a regular file with a single hard link.
func checkRegular(fd int, op, name string) error {
	var st unix.Stat_t
	if err := unix.Fstat(fd, &st); err != nil {
		return pathError(op, name, err)
	}
	if st.Mode&unix.S_IFMT != unix.S_IFREG || st.Nlink > 1 {
		return pathError(op, name, ErrUnsafeEntry)
	}
	return nil
}

// OpenFile opens a regular file under root like os.OpenFile. Symbolic links are never followed,
// the opened file must be a regular file with a single link, and perm is limited to permission bits.
// O_TRUNC is applied only after the file has been validated.
func OpenFile(root *os.Root, name string, flag int, perm os.FileMode) (*os.File, error) {
	// O_NONBLOCK keeps a planted FIFO from blocking the open; it is cleared once the file is validated.
	fd, err := openNoFollow(root, "open", name, (flag&^os.O_TRUNC)|unix.O_NONBLOCK, uint32(perm.Perm()))
	if err != nil {
		return nil, err
	}

	if err = checkRegular(fd, "open", name); err == nil {
		err = unix.SetNonblock(fd, false)
	}
	if err == nil && flag&os.O_TRUNC != 0 && flag&(os.O_WRONLY|os.O_RDWR) != 0 {
		err = unix.Ftruncate(fd, 0)
	}
	if err != nil {
		_ = unix.Close(fd)
		return nil, err
	}

	return os.NewFile(uintptr(fd), filepath.Join(root.Name(), name)), nil
}

// Create creates or truncates a regular file under root with mode 0600.
func Create(root *os.Root, name string) (*os.File, error) {
	return OpenFile(root, name, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
}

// Open opens a regular file under root for reading.
func Open(root *os.Root, name string) (*os.File, error) {
	return OpenFile(root, name, os.O_RDONLY, 0)
}

// openPath opens name with O_PATH, so the descriptor refers to a symbolic link itself rather than
// its target. It can be used for fstat and metadata changes without following links.
func openPath(root *os.Root, op, name string) (*os.File, error) {
	fd, err := openNoFollow(root, op, name, unix.O_PATH, 0)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(fd), filepath.Join(root.Name(), name)), nil
}

// Lstat returns the FileInfo of name without following symbolic links in any path component.
func Lstat(root *os.Root, name string) (os.FileInfo, error) {
	f, err := openPath(root, "lstat", name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return f.Stat()
}

// openMetadataTarget opens name with O_PATH and rejects symbolic links and multiply-linked files,
// so a metadata change made through the descriptor can only affect that cache entry.
func openMetadataTarget(root *os.Root, op, name string) (*os.File, error) {
	f, err := openPath(root, op, name)
	if err != nil {
		return nil, err
	}

	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 || (!info.IsDir() && info.Sys().(*syscall.Stat_t).Nlink > 1) {
		_ = f.Close()
		return nil, pathError(op, name, ErrUnsafeEntry)
	}
	return f, nil
}

// Chmod changes the permission bits of name without following symbolic links.
func Chmod(root *os.Root, name string, mode os.FileMode) error {
	f, err := openMetadataTarget(root, "chmod", name)
	if err != nil {
		return err
	}
	defer f.Close()

	perm := uint32(mode.Perm())
	err = unix.Fchmodat(int(f.Fd()), "", perm, unix.AT_EMPTY_PATH)
	if err != nil && !errors.Is(err, unix.EPERM) && !errors.Is(err, unix.EACCES) {
		// Kernels before 6.6 lack fchmodat2; the /proc magic link refers to the inode opened above.
		err = unix.Chmod("/proc/self/fd/"+strconv.Itoa(int(f.Fd())), perm)
	}
	if err != nil {
		return pathError("chmod", name, err)
	}
	return nil
}

// Chown changes the owner and group of name without following symbolic links.
func Chown(root *os.Root, name string, uid, gid int) error {
	f, err := openMetadataTarget(root, "chown", name)
	if err != nil {
		return err
	}
	defer f.Close()

	if err = unix.Fchownat(int(f.Fd()), "", uid, gid, unix.AT_EMPTY_PATH); err != nil {
		return pathError("chown", name, err)
	}
	return nil
}

// Chtimes changes the access and modification times of name without following symbolic links.
func Chtimes(root *os.Root, name string, atime, mtime time.Time) error {
	dirfd, base, err := openParent(root, "chtimes", name)
	if err != nil {
		return err
	}
	defer unix.Close(dirfd)

	ts := []unix.Timespec{unix.NsecToTimespec(atime.UnixNano()), unix.NsecToTimespec(mtime.UnixNano())}
	if err = unix.UtimesNanoAt(dirfd, base, ts, unix.AT_SYMLINK_NOFOLLOW); err != nil {
		return pathError("chtimes", name, err)
	}
	return nil
}

// Truncate changes the size of the regular file name without following symbolic links.
func Truncate(root *os.Root, name string, size int64) error {
	f, err := OpenFile(root, name, os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	defer f.Close()
	return f.Truncate(size)
}

// Remove removes the file, symbolic link or empty directory name. A symbolic link is removed itself.
func Remove(root *os.Root, name string) error {
	dirfd, base, err := openParent(root, "remove", name)
	if err != nil {
		return err
	}
	defer unix.Close(dirfd)

	err = unix.Unlinkat(dirfd, base, 0)
	if errors.Is(err, unix.EISDIR) {
		err = unix.Unlinkat(dirfd, base, unix.AT_REMOVEDIR)
	}
	if err != nil {
		return pathError("remove", name, err)
	}
	return nil
}

// RemoveAll removes name and everything under it without following symbolic links. It returns nil if
// name does not exist.
func RemoveAll(root *os.Root, name string) error {
	dirfd, base, err := openParent(root, "removeall", name)
	if errors.Is(err, unix.ENOENT) {
		return nil
	}
	if err != nil {
		return err
	}
	defer unix.Close(dirfd)

	if err = removeAllAt(dirfd, base); err != nil {
		return pathError("removeall", name, err)
	}
	return nil
}

func removeAllAt(dirfd int, name string) error {
	err := unix.Unlinkat(dirfd, name, 0)
	if err == nil || errors.Is(err, unix.ENOENT) {
		return nil
	}
	if !errors.Is(err, unix.EISDIR) {
		return err
	}

	fd, err := openDirNoFollow(dirfd, name)
	if err != nil {
		return err
	}
	if err = checkTrustedDir(fd, "removeall", name); err != nil {
		_ = unix.Close(fd)
		return err
	}
	dir := os.NewFile(uintptr(fd), name)
	names, err := dir.Readdirnames(-1)
	for _, child := range names {
		if childErr := removeAllAt(fd, child); childErr != nil && err == nil {
			err = childErr
		}
	}
	_ = dir.Close()
	if err != nil {
		return err
	}

	err = unix.Unlinkat(dirfd, name, unix.AT_REMOVEDIR)
	if errors.Is(err, unix.ENOENT) {
		return nil
	}
	return err
}

// Rename renames oldname to newname. Symbolic links are not followed in either path.
func Rename(root *os.Root, oldname, newname string) error {
	oldDir, oldBase, err := openParent(root, "rename", oldname)
	if err != nil {
		return err
	}
	defer unix.Close(oldDir)

	newDir, newBase, err := openParent(root, "rename", newname)
	if err != nil {
		return err
	}
	defer unix.Close(newDir)

	if err = unix.Renameat(oldDir, oldBase, newDir, newBase); err != nil {
		return &os.LinkError{Op: "rename", Old: oldname, New: newname, Err: err}
	}
	return nil
}

// MkdirAll creates directory name and any missing parents with mode DirPerm. It fails if any
// existing component is a symbolic link or not a directory.
func MkdirAll(root *os.Root, name string) error {
	fd, err := rootFd(root, "mkdir", name)
	if err != nil {
		return err
	}

	for part := range strings.SplitSeq(filepath.Clean(name), string(filepath.Separator)) {
		if part == "." {
			continue
		}
		err = unix.Mkdirat(fd, part, uint32(DirPerm))
		if err == nil || errors.Is(err, unix.EEXIST) {
			var next int
			next, err = openTrustedDirTraverse(fd, part, "mkdir", name)
			_ = unix.Close(fd)
			fd = next
		}
		if err != nil {
			if fd >= 0 {
				_ = unix.Close(fd)
			}
			return pathError("mkdir", name, err)
		}
	}
	return unix.Close(fd)
}

// ReadDir lists directory name, sorted by file name like os.ReadDir, without following symbolic links.
func ReadDir(root *os.Root, name string) ([]os.DirEntry, error) {
	fd, err := openNoFollow(root, "readdir", name, unix.O_RDONLY|unix.O_DIRECTORY, 0)
	if err != nil {
		return nil, err
	}
	dir := os.NewFile(uintptr(fd), filepath.Join(root.Name(), name))
	defer dir.Close()

	entries, err := dir.ReadDir(-1)
	if err != nil {
		return nil, err
	}
	slices.SortFunc(entries, func(a, b os.DirEntry) int { return strings.Compare(a.Name(), b.Name()) })
	return entries, nil
}

// ReadDirInfo lists directory name like ReadDir and returns the lstat information of each entry.
// Entries are opened with O_PATH|O_NOFOLLOW through the directory descriptor, so no symbolic link
// is followed and the path is not walked again per entry. Entries removed while listing are skipped.
func ReadDirInfo(root *os.Root, name string) ([]os.FileInfo, error) {
	fd, err := openNoFollow(root, "readdir", name, unix.O_RDONLY|unix.O_DIRECTORY, 0)
	if err != nil {
		return nil, err
	}
	dir := os.NewFile(uintptr(fd), filepath.Join(root.Name(), name))
	defer dir.Close()

	names, err := dir.Readdirnames(-1)
	if err != nil {
		return nil, err
	}
	slices.Sort(names)

	infos := make([]os.FileInfo, 0, len(names))
	for _, entry := range names {
		entryFd, err := unix.Openat(fd, entry, unix.O_PATH|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
		if err != nil {
			continue
		}
		f := os.NewFile(uintptr(entryFd), entry)
		info, err := f.Stat()
		_ = f.Close()
		if err == nil {
			infos = append(infos, info)
		}
	}
	return infos, nil
}
