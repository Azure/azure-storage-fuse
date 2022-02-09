// +build !fuse2

/*
    _____           _____   _____   ____          ______  _____  ------
   |     |  |      |     | |     | |     |     | |       |            |
   |     |  |      |     | |     | |     |     | |       |            |
   | --- |  |      |     | |-----| |---- |     | |-----| |-----  ------
   |     |  |      |     | |     | |     |     |       | |       |
   | ____|  |_____ | ____| | ____| |     |_____|  _____| |_____  |_____


   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2020-2022 Microsoft Corporation. All rights reserved.
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

package libfuse

// #cgo CFLAGS: -DFUSE_USE_VERSION=35 -D_FILE_OFFSET_BITS=64
// #cgo LDFLAGS: -lfuse3 -ldl
// #include "libfuse_wrapper.h"
import "C"
import (
	"blobfuse2/common"
	"blobfuse2/common/config"
	"blobfuse2/common/log"
	"blobfuse2/internal"
	"blobfuse2/internal/handlemap"
	"errors"
	"io/fs"
	"strings"
	"syscall"
	"unsafe"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type libfuseTestSuite struct {
	suite.Suite
	assert   *assert.Assertions
	libfuse  *Libfuse
	mockCtrl *gomock.Controller
	mock     *internal.MockComponent
}

var emptyConfig = ""
var defaultSize = int64(0)
var defaultMode = 0777

func newTestLibfuse(next internal.Component, configuration string) *Libfuse {
	config.ReadConfigFromReader(strings.NewReader(configuration))
	libfuse := NewLibfuseComponent()
	libfuse.SetNextComponent(next)
	libfuse.Configure()

	return libfuse.(*Libfuse)
}

func (suite *libfuseTestSuite) SetupTest() {
	err := log.SetDefaultLogger("silent", common.LogConfig{})
	if err != nil {
		panic("Unable to set silent logger as default.")
	}
	suite.setupTestHelper(emptyConfig)
}

func (suite *libfuseTestSuite) setupTestHelper(config string) {
	suite.assert = assert.New(suite.T())

	suite.mockCtrl = gomock.NewController(suite.T())
	suite.mock = internal.NewMockComponent(suite.mockCtrl)
	suite.libfuse = newTestLibfuse(suite.mock, config)
	fuseFS = suite.libfuse
	// suite.libfuse.Start(context.Background())
}

func (suite *libfuseTestSuite) cleanupTest() {
	// suite.libfuse.Stop()
	suite.mockCtrl.Finish()
}

func testMkDir(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := C.CString("/" + name)
	defer C.free(unsafe.Pointer(path))
	mode := fs.FileMode(0775)
	options := internal.CreateDirOptions{Name: name, Mode: mode}
	suite.mock.EXPECT().CreateDir(options).Return(nil)

	err := libfuse_mkdir(path, 0775)
	suite.assert.Equal(C.int(0), err)
}

func testMkDirError(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := C.CString("/" + name)
	defer C.free(unsafe.Pointer(path))
	mode := fs.FileMode(0775)
	options := internal.CreateDirOptions{Name: name, Mode: mode}
	suite.mock.EXPECT().CreateDir(options).Return(errors.New("failed to create directory"))

	err := libfuse_mkdir(path, 0775)
	suite.assert.Equal(C.int(-C.EIO), err)
}

// TODO: ReadDir test

func testRmDir(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := C.CString("/" + name)
	defer C.free(unsafe.Pointer(path))
	isDirEmptyOptions := internal.IsDirEmptyOptions{Name: name}
	suite.mock.EXPECT().IsDirEmpty(isDirEmptyOptions).Return(true)
	deleteDirOptions := internal.DeleteDirOptions{Name: name}
	suite.mock.EXPECT().DeleteDir(deleteDirOptions).Return(nil)

	err := libfuse_rmdir(path)
	suite.assert.Equal(C.int(0), err)
}

func testRmDirNotEmpty(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := C.CString("/" + name)
	defer C.free(unsafe.Pointer(path))
	isDirEmptyOptions := internal.IsDirEmptyOptions{Name: name}
	suite.mock.EXPECT().IsDirEmpty(isDirEmptyOptions).Return(false)

	err := libfuse_rmdir(path)
	suite.assert.Equal(C.int(-C.ENOTEMPTY), err)
}

func testRmDirError(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := C.CString("/" + name)
	defer C.free(unsafe.Pointer(path))
	isDirEmptyOptions := internal.IsDirEmptyOptions{Name: name}
	suite.mock.EXPECT().IsDirEmpty(isDirEmptyOptions).Return(true)
	deleteDirOptions := internal.DeleteDirOptions{Name: name}
	suite.mock.EXPECT().DeleteDir(deleteDirOptions).Return(errors.New("failed to delete directory"))

	err := libfuse_rmdir(path)
	suite.assert.Equal(C.int(-C.EIO), err)
}

func testCreate(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := C.CString("/" + name)
	defer C.free(unsafe.Pointer(path))
	mode := fs.FileMode(0775)
	info := &C.fuse_file_info_t{}
	options := internal.CreateFileOptions{Name: name, Mode: mode}
	suite.mock.EXPECT().CreateFile(options).Return(&handlemap.Handle{}, nil)

	err := libfuse_create(path, 0775, info)
	suite.assert.Equal(C.int(0), err)
	suite.assert.Equal(C.ulong(1), info.fh)
}

func testCreateError(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := C.CString("/" + name)
	defer C.free(unsafe.Pointer(path))
	mode := fs.FileMode(0775)
	info := &C.fuse_file_info_t{}
	options := internal.CreateFileOptions{Name: name, Mode: mode}
	suite.mock.EXPECT().CreateFile(options).Return(&handlemap.Handle{}, errors.New("failed to create file"))

	err := libfuse_create(path, 0775, info)
	suite.assert.Equal(C.int(-C.EIO), err)
	suite.assert.NotEqual(C.ulong(1), info.fh)
}

func testOpen(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := C.CString("/" + name)
	defer C.free(unsafe.Pointer(path))
	mode := fs.FileMode(fuseFS.filePermission)
	flags := C.O_RDWR & 0xffffffff
	info := &C.fuse_file_info_t{}
	info.flags = C.O_RDWR
	options := internal.OpenFileOptions{Name: name, Flags: flags, Mode: mode}
	suite.mock.EXPECT().OpenFile(options).Return(&handlemap.Handle{}, nil)

	err := libfuse_open(path, info)
	suite.assert.Equal(C.int(0), err)
	suite.assert.Less(C.ulong(0), info.fh)
}

func testOpenSyncFlag(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := C.CString("/" + name)
	defer C.free(unsafe.Pointer(path))
	mode := fs.FileMode(fuseFS.filePermission)
	flags := C.O_RDWR & C.O_SYNC & 0xffffffff
	info := &C.fuse_file_info_t{}
	info.flags = C.O_RDWR & C.O_SYNC
	options := internal.OpenFileOptions{Name: name, Flags: flags, Mode: mode}
	suite.mock.EXPECT().OpenFile(options).Return(&handlemap.Handle{}, nil)

	err := libfuse_open(path, info)
	suite.assert.Equal(C.int(0), err)
	suite.assert.Less(C.ulong(0), info.fh)
	suite.assert.Equal(C.int(0), info.flags&C.O_SYNC)
}

func testOpenNotExists(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := C.CString("/" + name)
	defer C.free(unsafe.Pointer(path))
	mode := fs.FileMode(fuseFS.filePermission)
	flags := C.O_RDWR & 0xffffffff
	info := &C.fuse_file_info_t{}
	info.flags = C.O_RDWR
	options := internal.OpenFileOptions{Name: name, Flags: flags, Mode: mode}
	suite.mock.EXPECT().OpenFile(options).Return(&handlemap.Handle{}, syscall.ENOENT)

	err := libfuse_open(path, info)
	suite.assert.Equal(C.int(-C.ENOENT), err)
	suite.assert.NotEqual(C.ulong(1), info.fh)
}

func testOpenError(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := C.CString("/" + name)
	defer C.free(unsafe.Pointer(path))
	mode := fs.FileMode(fuseFS.filePermission)
	flags := C.O_RDWR & 0xffffffff
	info := &C.fuse_file_info_t{}
	info.flags = C.O_RDWR
	options := internal.OpenFileOptions{Name: name, Flags: flags, Mode: mode}
	suite.mock.EXPECT().OpenFile(options).Return(&handlemap.Handle{}, errors.New("failed to open a file"))

	err := libfuse_open(path, info)
	suite.assert.Equal(C.int(-C.EIO), err)
	suite.assert.NotEqual(C.ulong(1), info.fh)
}

func testTruncate(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := C.CString("/" + name)
	defer C.free(unsafe.Pointer(path))
	size := int64(1024)
	options := internal.TruncateFileOptions{Name: name, Size: size}
	suite.mock.EXPECT().TruncateFile(options).Return(nil)

	err := libfuse_truncate(path, C.long(size), nil)
	suite.assert.Equal(C.int(0), err)
}

func testTruncateError(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := C.CString("/" + name)
	defer C.free(unsafe.Pointer(path))
	size := int64(1024)
	options := internal.TruncateFileOptions{Name: name, Size: size}
	suite.mock.EXPECT().TruncateFile(options).Return(errors.New("failed to truncate file"))

	err := libfuse_truncate(path, C.long(size), nil)
	suite.assert.Equal(C.int(-C.EIO), err)
}

func testUnlink(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := C.CString("/" + name)
	defer C.free(unsafe.Pointer(path))
	options := internal.DeleteFileOptions{Name: name}
	suite.mock.EXPECT().DeleteFile(options).Return(nil)

	err := libfuse_unlink(path)
	suite.assert.Equal(C.int(0), err)
}

func testUnlinkNotExists(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := C.CString("/" + name)
	defer C.free(unsafe.Pointer(path))
	options := internal.DeleteFileOptions{Name: name}
	suite.mock.EXPECT().DeleteFile(options).Return(syscall.ENOENT)

	err := libfuse_unlink(path)
	suite.assert.Equal(C.int(-C.ENOENT), err)
}

func testUnlinkError(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := C.CString("/" + name)
	defer C.free(unsafe.Pointer(path))
	options := internal.DeleteFileOptions{Name: name}
	suite.mock.EXPECT().DeleteFile(options).Return(errors.New("failed to delete file"))

	err := libfuse_unlink(path)
	suite.assert.Equal(C.int(-C.EIO), err)
}

// Rename

func testSymlink(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	target := "target"
	path := C.CString("/" + name)
	defer C.free(unsafe.Pointer(path))
	t := C.CString(target)
	defer C.free(unsafe.Pointer(t))
	options := internal.CreateLinkOptions{Name: name, Target: target}
	suite.mock.EXPECT().CreateLink(options).Return(nil)

	err := libfuse_symlink(t, path)
	suite.assert.Equal(C.int(0), err)
}

func testSymlinkError(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	target := "target"
	path := C.CString("/" + name)
	defer C.free(unsafe.Pointer(path))
	t := C.CString(target)
	defer C.free(unsafe.Pointer(t))
	options := internal.CreateLinkOptions{Name: name, Target: target}
	suite.mock.EXPECT().CreateLink(options).Return(errors.New("failed to create link"))

	err := libfuse_symlink(t, path)
	suite.assert.Equal(C.int(-C.EIO), err)
}

func testReadLink(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := C.CString("/" + name)
	defer C.free(unsafe.Pointer(path))
	options := internal.ReadLinkOptions{Name: name}
	suite.mock.EXPECT().ReadLink(options).Return("target", nil)

	// https://stackoverflow.com/questions/41953619/how-to-initialise-empty-c-cstring-in-cgo
	buf := C.CString("")
	err := libfuse_readlink(path, buf, 7)
	suite.assert.Equal(C.int(0), err)
	suite.assert.Equal("target", C.GoString(buf))
}

func testReadLinkNotExists(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := C.CString("/" + name)
	defer C.free(unsafe.Pointer(path))
	options := internal.ReadLinkOptions{Name: name}
	suite.mock.EXPECT().ReadLink(options).Return("", syscall.ENOENT)

	buf := C.CString("")
	err := libfuse_readlink(path, buf, 7)
	suite.assert.Equal(C.int(-C.ENOENT), err)
	suite.assert.NotEqual("target", C.GoString(buf))
}

func testReadLinkError(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := C.CString("/" + name)
	defer C.free(unsafe.Pointer(path))
	options := internal.ReadLinkOptions{Name: name}
	suite.mock.EXPECT().ReadLink(options).Return("", errors.New("failed to read link"))

	buf := C.CString("")
	err := libfuse_readlink(path, buf, 7)
	suite.assert.Equal(C.int(-C.EIO), err)
	suite.assert.NotEqual("target", C.GoString(buf))
}

func testFsync(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := C.CString("/" + name)
	defer C.free(unsafe.Pointer(path))
	mode := fs.FileMode(fuseFS.filePermission)
	flags := C.O_RDWR & 0xffffffff
	info := &C.fuse_file_info_t{}
	info.flags = C.O_RDWR
	handle := &handlemap.Handle{}
	openOptions := internal.OpenFileOptions{Name: name, Flags: flags, Mode: mode}
	suite.mock.EXPECT().OpenFile(openOptions).Return(handle, nil)
	libfuse_open(path, info)
	handle.Path = name
	handle.ID = handlemap.HandleID(info.fh)

	options := internal.SyncFileOptions{Handle: handle}
	suite.mock.EXPECT().SyncFile(options).Return(nil)

	err := libfuse_fsync(path, C.int(0), info)
	suite.assert.Equal(C.int(0), err)
}

func testFsyncHandleError(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := C.CString("/" + name)
	defer C.free(unsafe.Pointer(path))
	info := &C.fuse_file_info_t{}
	info.flags = C.O_RDWR

	err := libfuse_fsync(path, C.int(0), info)
	suite.assert.Equal(C.int(-C.EIO), err)
}

func testFsyncError(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := C.CString("/" + name)
	defer C.free(unsafe.Pointer(path))
	mode := fs.FileMode(fuseFS.filePermission)
	flags := C.O_RDWR & 0xffffffff
	info := &C.fuse_file_info_t{}
	info.flags = C.O_RDWR
	handle := &handlemap.Handle{}
	openOptions := internal.OpenFileOptions{Name: name, Flags: flags, Mode: mode}
	suite.mock.EXPECT().OpenFile(openOptions).Return(handle, nil)
	libfuse_open(path, info)
	handle.Path = name
	handle.ID = handlemap.HandleID(info.fh)

	options := internal.SyncFileOptions{Handle: handle}
	suite.mock.EXPECT().SyncFile(options).Return(errors.New("failed to sync file"))

	err := libfuse_fsync(path, C.int(0), info)
	suite.assert.Equal(C.int(-C.EIO), err)
}

func testFsyncDir(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := C.CString("/" + name)
	defer C.free(unsafe.Pointer(path))
	options := internal.SyncDirOptions{Name: name}
	suite.mock.EXPECT().SyncDir(options).Return(nil)

	err := libfuse_fsyncdir(path, C.int(0), nil)
	suite.assert.Equal(C.int(0), err)
}

func testFsyncDirError(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := C.CString("/" + name)
	defer C.free(unsafe.Pointer(path))
	options := internal.SyncDirOptions{Name: name}
	suite.mock.EXPECT().SyncDir(options).Return(errors.New("failed to sync dir"))

	err := libfuse_fsyncdir(path, C.int(0), nil)
	suite.assert.Equal(C.int(-C.EIO), err)
}

func testChmod(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := C.CString("/" + name)
	defer C.free(unsafe.Pointer(path))
	mode := fs.FileMode(0775)
	options := internal.ChmodOptions{Name: name, Mode: mode}
	suite.mock.EXPECT().Chmod(options).Return(nil)

	err := libfuse_chmod(path, 0775, nil)
	suite.assert.Equal(C.int(0), err)
}

func testChmodNotExists(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := C.CString("/" + name)
	defer C.free(unsafe.Pointer(path))
	mode := fs.FileMode(0775)
	options := internal.ChmodOptions{Name: name, Mode: mode}
	suite.mock.EXPECT().Chmod(options).Return(syscall.ENOENT)

	err := libfuse_chmod(path, 0775, nil)
	suite.assert.Equal(C.int(-C.ENOENT), err)
}

func testChmodError(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := C.CString("/" + name)
	defer C.free(unsafe.Pointer(path))
	mode := fs.FileMode(0775)
	options := internal.ChmodOptions{Name: name, Mode: mode}
	suite.mock.EXPECT().Chmod(options).Return(errors.New("failed to chmod"))

	err := libfuse_chmod(path, 0775, nil)
	suite.assert.Equal(C.int(-C.EIO), err)
}

func testChown(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := C.CString("/" + name)
	defer C.free(unsafe.Pointer(path))
	group := C.uint(5)
	owner := C.uint(4)

	err := libfuse_chown(path, owner, group, nil)
	suite.assert.Equal(C.int(0), err)
}

func testUtimens(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := C.CString("/" + name)
	defer C.free(unsafe.Pointer(path))

	err := libfuse_utimens(path, nil, nil)
	suite.assert.Equal(C.int(0), err)
}
