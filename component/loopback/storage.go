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
	"encoding/binary"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"io/fs"
	"math/rand/v2"
	"os"
	"path"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
)

// The state directory, at the top of the backing directory, holds what Azure Storage keeps outside the namespace of
// a container: uncommitted blocks, committed block lists, and blobs that are being uploaded. Operations of the
// component never list it and reject names within it.
const (
	stateDir     = ".loopbackfs"
	stagedDir    = stateDir + "/staged"
	blockListDir = stateDir + "/blocklists"
	uploadDir    = stateDir + "/uploads"
)

// copyBufferSize is the largest buffer used to copy data that the kernel cannot copy between files by itself.
const copyBufferSize = 1024 * 1024

// copyData copies size bytes from src to dst through a buffer of up to copyBufferSize bytes.
func copyData(dst io.Writer, src io.Reader, size int64) (int64, error) {
	buf := make([]byte, max(1, min(size, copyBufferSize)))
	// Hide the ReaderFrom of dst, which would copy through a smaller buffer of its own.
	return io.CopyBuffer(struct{ io.Writer }{dst}, src, buf)
}

var errNotConfigured = errors.New("loopbackfs: path is not configured")

// objectName returns the name, relative to the backing directory, of the file that holds the object name. Names are
// accepted with or without leading and trailing slashes, and the root of the container is ".". Names that Azure
// Storage would resolve to another path, and names within the state directory, fail with EINVAL.
func objectName(name string) (string, error) {
	name = strings.Trim(name, "/")
	if name == "" {
		return ".", nil
	}

	if !common.IsValidObjectName(name) {
		return "", syscall.EINVAL
	}

	name = path.Clean(name)
	if name == stateDir || strings.HasPrefix(name, stateDir+"/") {
		return "", syscall.EINVAL
	}

	return name, nil
}

// objectPath returns the path of the object stored in the file name, as the pipeline names it: "" for the root.
func objectPath(name string) string {
	if name == "." {
		return ""
	}
	return name
}

// storageError converts an error of the backing file system to the error azstorage returns for the same condition.
// azstorage returns bare errno values, which callers compare with ==, so the path is dropped.
func storageError(err error) error {
	var errno syscall.Errno
	if errors.As(err, &errno) {
		return errno
	}
	return err
}

// lookupError is storageError for an object that must exist. An object below a file does not exist in Azure Storage,
// so ENOTDIR becomes ENOENT.
func lookupError(err error) error {
	err = storageError(err)
	if err == syscall.ENOTDIR {
		return syscall.ENOENT
	}
	return err
}

// filePerm and dirPerm return the permissions of a backing file or directory for a requested mode. Azure Storage
// serves a blob whatever permissions it has, so loopbackfs keeps the right of its owner to read and write files and
// to search directories.
func filePerm(mode fs.FileMode) fs.FileMode {
	return mode.Perm() | 0o600
}

func dirPerm(mode fs.FileMode) fs.FileMode {
	return mode.Perm() | 0o700
}

func isSymlink(info fs.FileInfo) bool {
	return info.Mode()&fs.ModeSymlink != 0
}

// makeParent creates the missing parent directories of the object name. Blobs need no parent directories in a flat
// namespace, and a hierarchical namespace account creates the parents of a new blob.
func makeParent(root *os.Root, name string) error {
	dir := path.Dir(name)
	if dir == "." {
		return nil
	}

	err := storageError(root.MkdirAll(dir, 0o777))
	if err == syscall.EEXIST {
		// A parent exists but is not a directory, which creating the object would report.
		return syscall.ENOTDIR
	}
	return err
}

// newUploadName returns a unique name for a file in the upload directory, creating the directory if needed.
func newUploadName(root *os.Root) (string, error) {
	if err := root.MkdirAll(uploadDir, 0o700); err != nil {
		return "", storageError(err)
	}
	return uploadDir + "/" + strconv.FormatUint(rand.Uint64(), 36), nil
}

// createUploadFile creates an empty file in the upload directory.
func createUploadFile(root *os.Root) (*os.File, string, error) {
	const attempts = 10

	var err error
	for range attempts {
		var name string
		name, err = newUploadName(root)
		if err != nil {
			return nil, "", err
		}

		var f *os.File
		f, err = root.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o666)
		if err == nil {
			return f, name, nil
		}
		if !errors.Is(err, fs.ErrExist) {
			break
		}
	}

	return nil, "", storageError(err)
}

// upload replaces the object name with a new file that write fills, the way Azure Storage replaces a blob: readers
// see either the old or the new content. The new file keeps the permissions of the file it replaces. It returns the
// file info of the new file. The caller must hold the lock of name.
func (lfs *LoopbackFS) upload(root *os.Root, name string, write func(*os.File) error) (fs.FileInfo, error) {
	old, err := root.Lstat(name)
	switch {
	case err == nil && old.IsDir():
		return nil, syscall.EISDIR
	case errors.Is(err, fs.ErrNotExist):
		old = nil
	case err != nil:
		return nil, storageError(err)
	}

	if err := makeParent(root, name); err != nil {
		return nil, err
	}

	f, uploadName, err := createUploadFile(root)
	if err != nil {
		return nil, err
	}

	uploaded := false
	defer func() {
		if !uploaded {
			// The file may be closed already, which makes this Close fail harmlessly.
			_ = f.Close()
			_ = root.Remove(uploadName)
		}
	}()

	if err := write(f); err != nil {
		return nil, err
	}

	if old != nil && old.Mode().IsRegular() {
		if err := f.Chmod(old.Mode().Perm()); err != nil {
			return nil, storageError(err)
		}
	}

	info, err := f.Stat()
	if err != nil {
		return nil, storageError(err)
	}

	if err := f.Close(); err != nil {
		return nil, storageError(err)
	}

	if err := root.Rename(uploadName, name); err != nil {
		return nil, storageError(err)
	}
	uploaded = true

	if old != nil {
		lfs.forget(root, old)
	}
	lfs.versions.changed(info)

	return info, nil
}

// forget drops what the component keeps about a file that was removed or replaced.
func (lfs *LoopbackFS) forget(root *os.Root, info fs.FileInfo) {
	lfs.versions.removed(info)
	removeBlockList(root, info)
}

// blobReader reads the content of an object, as Azure Storage returns it: the content of a symbolic link is its
// target, like the content of a symlink blob.
type blobReader struct {
	io.ReaderAt
	info fs.FileInfo
	file *os.File
}

func openBlob(root *os.Root, name string) (*blobReader, error) {
	info, err := root.Lstat(name)
	if err != nil {
		return nil, lookupError(err)
	}

	if info.IsDir() {
		return nil, syscall.EISDIR
	}

	if isSymlink(info) {
		target, err := root.Readlink(name)
		if err != nil {
			return nil, lookupError(err)
		}
		return &blobReader{ReaderAt: strings.NewReader(target), info: info}, nil
	}

	f, err := root.Open(name)
	if err != nil {
		return nil, lookupError(err)
	}

	// Use the attributes of the opened file, so that they describe the content that is read.
	info, err = f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, storageError(err)
	}

	return &blobReader{ReaderAt: f, info: info, file: f}, nil
}

func (b *blobReader) Close() {
	if b.file != nil {
		_ = b.file.Close()
	}
}

// fileID identifies a file of the backing file system.
type fileID struct {
	dev, ino uint64
}

func idOf(info fs.FileInfo) (fileID, bool) {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return fileID{}, false
	}
	return fileID{dev: st.Dev, ino: st.Ino}, true
}

// versions numbers the changes the component makes to files, so that the ETag of a file changes with every change,
// as the ETag of a blob does, even when a change keeps the size of the file and lands within the resolution of its
// modification time.
type versions struct {
	last atomic.Uint64
	byID sync.Map // fileID -> uint64
}

// changed records a change of the file described by info.
func (v *versions) changed(info fs.FileInfo) {
	if id, ok := idOf(info); ok {
		v.byID.Store(id, v.last.Add(1))
	}
}

// removed forgets a file that was removed, as its inode can be reused by another file.
func (v *versions) removed(info fs.FileInfo) {
	if id, ok := idOf(info); ok {
		v.byID.Delete(id)
	}
}

// etag returns the ETag of the file described by info, in the format of Azure Storage ETags.
func (v *versions) etag(info fs.FileInfo) string {
	id, _ := idOf(info)

	var version uint64
	if val, ok := v.byID.Load(id); ok {
		version = val.(uint64)
	}

	var buf [40]byte
	binary.LittleEndian.PutUint64(buf[0:], id.dev)
	binary.LittleEndian.PutUint64(buf[8:], id.ino)
	binary.LittleEndian.PutUint64(buf[16:], version)
	binary.LittleEndian.PutUint64(buf[24:], uint64(info.ModTime().UnixNano()))
	binary.LittleEndian.PutUint64(buf[32:], uint64(info.Size()))

	h := fnv.New64a()
	_, _ = h.Write(buf[:])
	return fmt.Sprintf("0x%X", h.Sum64())
}

const lockStripes = 64

// objectLocks serializes the changes of an object that take more than one file system call, such as committing a
// block list. Names are hashed to a fixed set of locks, so the set does not grow with the namespace.
type objectLocks struct {
	stripes [lockStripes]sync.RWMutex
}

func stripeOf(name string) int {
	h := fnv.New32a()
	_, _ = h.Write([]byte(name))
	return int(h.Sum32() % lockStripes)
}

// lock locks the objects names for writing, in a fixed order to avoid deadlocks, and returns the function that
// unlocks them.
func (l *objectLocks) lock(names ...string) func() {
	stripes := make([]int, 0, len(names))
	for _, name := range names {
		stripes = append(stripes, stripeOf(name))
	}
	slices.Sort(stripes)
	stripes = slices.Compact(stripes)

	for _, s := range stripes {
		l.stripes[s].Lock()
	}

	return func() {
		for i := len(stripes) - 1; i >= 0; i-- {
			l.stripes[stripes[i]].Unlock()
		}
	}
}

// rlock locks the object name for reading and returns the function that unlocks it.
func (l *objectLocks) rlock(name string) func() {
	stripe := &l.stripes[stripeOf(name)]
	stripe.RLock()
	return stripe.RUnlock
}

// md5Of returns the MD5 of the content of the file name, or nil if it cannot be read.
func md5Of(root *os.Root, name string) []byte {
	f, err := root.Open(name)
	if err != nil {
		log.Err("LoopbackFS::md5Of : failed to open %s [%v]", name, err)
		return nil
	}
	defer f.Close()

	sum, err := common.GetMD5(f)
	if err != nil {
		log.Err("LoopbackFS::md5Of : failed to compute md5 of %s [%v]", name, err)
		return nil
	}
	return sum
}
