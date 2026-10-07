//go:build fuse2

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

package libfuse

// #cgo CFLAGS: -DFUSE_USE_VERSION=29 -D_FILE_OFFSET_BITS=64 -D__FUSE2__
// #cgo LDFLAGS: -lfuse -ldl
// #include "libfuse_wrapper.h"
import "C"
import (
	"errors"
	"io/fs"
	"os"
	"strings"
	"syscall"
	"unsafe"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/config"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal"
	"github.com/Azure/azure-storage-fuse/v2/internal/handlemap"
	"github.com/Azure/azure-storage-fuse/v2/internal/stats_manager"

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

type fileHandle struct {
	fd  uint64
	obj uint64
}

// Open and create call returns this kind of object
var emptyConfig = ""
var defaultSize = int64(0)
var defaultMode = 0777

func newTestLibfuse(next internal.Component, configuration string) *Libfuse {
	_ = config.ReadConfigFromReader(strings.NewReader(configuration))
	libfuse := NewLibfuseComponent()
	libfuse.SetNextComponent(next)
	_ = libfuse.Configure(true)

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

func testStatFs(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	path := C.CString("/")
	defer C.free(unsafe.Pointer(path))
	suite.mock.EXPECT().StatFs().Return(&syscall.Statfs_t{Frsize: 1,
		Blocks: 2, Bavail: 3, Bfree: 4}, true, nil)
	buf := &C.statvfs_t{}
	libfuse_statfs(path, buf)

	suite.assert.Equal(1, int(buf.f_frsize))
	suite.assert.Equal(2, int(buf.f_blocks))
	suite.assert.Equal(3, int(buf.f_bavail))
	suite.assert.Equal(4, int(buf.f_bfree))
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

func testRmDirInvalidPath(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	path := C.CString("/../escape")
	defer C.free(unsafe.Pointer(path))

	// The mock fails the test if IsDirEmpty or DeleteDir is called.
	err := libfuse_rmdir(path)
	suite.assert.Equal(C.int(-C.EINVAL), err)
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

	option := internal.GetAttrOptions{Name: name}
	suite.mock.EXPECT().GetAttr(option).Return(&internal.ObjAttr{}, nil)
	stbuf := &C.stat_t{}
	err = libfuse2_getattr(path, stbuf)
	suite.assert.Equal(C.int(0), err)
	suite.assert.Equal(stbuf.st_mtim.tv_nsec, C.long(0))
	suite.assert.NotEqual(stbuf.st_mtim.tv_sec, C.long(0))
}

// testCreateNativeIO : created files are served natively like opened files, unless file_cache offloads their IO
func testCreateNativeIO(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := C.CString("/" + name)
	defer C.free(unsafe.Pointer(path))
	options := internal.CreateFileOptions{Name: name, Mode: fs.FileMode(0775)}

	cached := handlemap.NewHandle(name)
	cached.UnixFD = 42
	cached.Flags.Set(handlemap.HandleFlagCached)
	offloaded := handlemap.NewHandle(name) // file_cache with offload-io: true
	offloaded.UnixFD = 43

	for _, tc := range []struct {
		handle *handlemap.Handle
		fd     uint64
	}{{cached, 42}, {offloaded, 0}} {
		suite.mock.EXPECT().CreateFile(options).Return(tc.handle, nil)
		info := &C.fuse_file_info_t{}
		suite.assert.Equal(C.int(0), libfuse_create(path, 0775, info))

		fobj := (*fileHandle)(unsafe.Pointer(uintptr(info.fh)))
		suite.assert.Equal(tc.fd, fobj.fd)
		C.release_native_file_object(info)
		handlemap.Delete(tc.handle.ID)
	}
}

// testNativeWriteDirty : a native write marks the handle dirty only if it changed the file
func testNativeWriteDirty(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	path := C.CString("/path")
	defer C.free(unsafe.Pointer(path))
	data := C.CString("data")
	defer C.free(unsafe.Pointer(data))

	f, err := os.CreateTemp("", "native_write")
	suite.assert.NoError(err)
	defer os.Remove(f.Name())
	defer f.Close()
	readOnly, err := os.Open(f.Name())
	suite.assert.NoError(err)
	defer readOnly.Close()

	for _, tc := range []struct {
		file  *os.File
		ret   C.int
		dirty C.uint8_t
	}{{readOnly, -C.EBADF, 0}, {f, 4, 1}} {
		handle := handlemap.NewHandle("path")
		fobj := C.allocate_native_file_object(C.uint64_t(tc.file.Fd()), C.uint64_t(uintptr(unsafe.Pointer(handle))))
		info := &C.fuse_file_info_t{}
		info.fh = C.uint64_t(uintptr(unsafe.Pointer(fobj)))

		ret := C.native_write_file(path, data, 4, 0, info)
		suite.assert.Equal(tc.ret, ret)
		suite.assert.Equal(tc.dirty, fobj.dirty)
		C.release_native_file_object(info)
	}
}

// testFlushNativeDirty : flush uploads a file changed by native writes, retries a failed upload, and does not upload
// the file again once it is uploaded
func testFlushNativeDirty(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	path := C.CString("/path")
	defer C.free(unsafe.Pointer(path))

	handle := handlemap.NewHandle("path")
	fobj := C.allocate_native_file_object(42, C.uint64_t(uintptr(unsafe.Pointer(handle))))
	info := &C.fuse_file_info_t{}
	info.fh = C.uint64_t(uintptr(unsafe.Pointer(fobj)))
	defer C.release_native_file_object(info)

	options := internal.FlushFileOptions{Handle: handle}
	upload := func(opts internal.FlushFileOptions) error {
		opts.Handle.Flags.Clear(handlemap.HandleFlagDirty)
		return nil
	}

	suite.assert.Equal(C.int(0), libfuse_flush(path, info))

	fobj.dirty = 1
	suite.mock.EXPECT().FlushFile(options).Return(syscall.EIO)
	suite.assert.Equal(C.int(-C.EIO), libfuse_flush(path, info))

	suite.mock.EXPECT().FlushFile(options).DoAndReturn(upload)
	suite.assert.Equal(C.int(0), libfuse_flush(path, info))

	suite.assert.Equal(C.int(0), libfuse_flush(path, info))
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
}

// enableLibfuseStats turns on stats collection for the test and returns the libfuse collector.
func enableLibfuseStats(suite *libfuseTestSuite) *stats_manager.StatsCollector {
	stats_manager.EnableForTest(suite.T())
	prev := libfuseStatsCollector
	libfuseStatsCollector = stats_manager.NewStatsCollector(suite.libfuse.Name())
	suite.T().Cleanup(func() { libfuseStatsCollector = prev })
	return libfuseStatsCollector
}

func testCreateUpdatesStats(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	collector := enableLibfuseStats(suite)
	name := "path"
	path := C.CString("/" + name)
	defer C.free(unsafe.Pointer(path))
	mode := fs.FileMode(0775)
	info := &C.fuse_file_info_t{}
	options := internal.CreateFileOptions{Name: name, Mode: mode}
	suite.mock.EXPECT().CreateFile(options).Return(&handlemap.Handle{}, nil)

	err := libfuse_create(path, 0775, info)
	suite.assert.Equal(C.int(0), err)

	stats := collector.StopAndGetStats()
	suite.assert.Equal(int64(1), stats[createFile])
	suite.assert.Equal(int64(1), stats[openHandles])
}

func testCreateErrorDoesNotUpdateStats(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	collector := enableLibfuseStats(suite)
	name := "path"
	path := C.CString("/" + name)
	defer C.free(unsafe.Pointer(path))
	mode := fs.FileMode(0775)
	info := &C.fuse_file_info_t{}
	options := internal.CreateFileOptions{Name: name, Mode: mode}
	suite.mock.EXPECT().CreateFile(options).Return(&handlemap.Handle{}, errors.New("failed to create file"))

	err := libfuse_create(path, 0775, info)
	suite.assert.Equal(C.int(-C.EIO), err)

	stats := collector.StopAndGetStats()
	suite.assert.NotContains(stats, createFile)
	suite.assert.NotContains(stats, openHandles)
}

func testCreateInvalidArgument(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	path := C.CString("/path")
	defer C.free(unsafe.Pointer(path))
	suite.mock.EXPECT().CreateFile(internal.CreateFileOptions{Name: "path", Mode: 0600}).Return(nil, errors.Join(syscall.EINVAL, errors.New("DFS rejected permissions")))

	err := libfuse_create(path, 0600, &C.fuse_file_info_t{})
	suite.assert.Equal(C.int(-C.EINVAL), err)
}

func testGetAttrInvalidArgument(suite *libfuseTestSuite) {
	defer suite.cleanupTest()

	name := "service-rejected-name"
	path := C.CString("/" + name)
	defer C.free(unsafe.Pointer(path))

	suite.mock.EXPECT().GetAttr(internal.GetAttrOptions{Name: name}).Return(nil, syscall.EINVAL)
	result := libfuse2_getattr(path, &C.stat_t{})
	suite.assert.Equal(C.int(-C.EINVAL), result)
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
}

func testOpenSyncDirectFlag(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := C.CString("/" + name)
	defer C.free(unsafe.Pointer(path))
	mode := fs.FileMode(fuseFS.filePermission)
	flags := C.O_RDWR & 0xffffffff
	info := &C.fuse_file_info_t{}
	info.flags = C.O_RDWR | C.O_SYNC | C.__O_DIRECT
	options := internal.OpenFileOptions{Name: name, Flags: flags, Mode: mode}
	suite.mock.EXPECT().OpenFile(options).Return(&handlemap.Handle{}, nil)

	err := libfuse_open(path, info)
	suite.assert.Equal(C.int(0), err)
	suite.assert.Equal(C.int(0), info.flags&C.O_SYNC)
	suite.assert.Equal(C.int(0), info.flags&C.__O_DIRECT)
}

// fuse2 does not have writeback caching, so append flag is passed unchanged
func testOpenAppendFlagDefault(suite *libfuseTestSuite) {
	defer suite.cleanupTest()

	name := "path"
	path := C.CString("/" + name)
	defer C.free(unsafe.Pointer(path))
	mode := fs.FileMode(fuseFS.filePermission)
	flags := C.O_RDWR | C.O_APPEND&0xffffffff
	info := &C.fuse_file_info_t{}
	info.flags = C.O_RDWR | C.O_APPEND
	options := internal.OpenFileOptions{Name: name, Flags: flags, Mode: mode}
	suite.mock.EXPECT().OpenFile(options).Return(&handlemap.Handle{}, nil)

	err := libfuse_open(path, info)
	suite.assert.Equal(C.int(0), err)

	flags = C.O_WRONLY | C.O_APPEND&0xffffffff
	info = &C.fuse_file_info_t{}
	info.flags = C.O_WRONLY | C.O_APPEND
	options = internal.OpenFileOptions{Name: name, Flags: flags, Mode: mode}
	suite.mock.EXPECT().OpenFile(options).Return(&handlemap.Handle{}, nil)

	err = libfuse_open(path, info)
	suite.assert.Equal(C.int(0), err)
}

func testOpenAppendFlagDisableWritebackCache(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	suite.cleanupTest() // clean up the default libfuse generated
	config := "libfuse:\n  disable-writeback-cache: true\n"
	suite.setupTestHelper(config) // setup a new libfuse with a custom config (clean up will occur after the test as usual)
	suite.assert.True(suite.libfuse.disableWritebackCache)

	name := "path"
	path := C.CString("/" + name)
	defer C.free(unsafe.Pointer(path))
	mode := fs.FileMode(fuseFS.filePermission)
	flags := C.O_RDWR | C.O_APPEND&0xffffffff
	info := &C.fuse_file_info_t{}
	info.flags = C.O_RDWR | C.O_APPEND
	options := internal.OpenFileOptions{Name: name, Flags: flags, Mode: mode}
	suite.mock.EXPECT().OpenFile(options).Return(&handlemap.Handle{}, nil)

	err := libfuse_open(path, info)
	suite.assert.Equal(C.int(0), err)

	flags = C.O_WRONLY | C.O_APPEND&0xffffffff
	info = &C.fuse_file_info_t{}
	info.flags = C.O_WRONLY | C.O_APPEND
	options = internal.OpenFileOptions{Name: name, Flags: flags, Mode: mode}
	suite.mock.EXPECT().OpenFile(options).Return(&handlemap.Handle{}, nil)

	err = libfuse_open(path, info)
	suite.assert.Equal(C.int(0), err)
}

func testOpenAppendFlagIgnoreAppendFlag(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	suite.cleanupTest() // clean up the default libfuse generated
	config := "libfuse:\n  ignore-open-flags: true\n"
	suite.setupTestHelper(config) // setup a new libfuse with a custom config (clean up will occur after the test as usual)
	suite.assert.True(suite.libfuse.ignoreOpenFlags)

	name := "path"
	path := C.CString("/" + name)
	defer C.free(unsafe.Pointer(path))
	mode := fs.FileMode(fuseFS.filePermission)
	flags := C.O_RDWR | C.O_APPEND&0xffffffff
	info := &C.fuse_file_info_t{}
	info.flags = C.O_RDWR | C.O_APPEND
	options := internal.OpenFileOptions{Name: name, Flags: flags, Mode: mode}
	suite.mock.EXPECT().OpenFile(options).Return(&handlemap.Handle{}, nil)

	err := libfuse_open(path, info)
	suite.assert.Equal(C.int(0), err)

	flags = C.O_WRONLY | C.O_APPEND&0xffffffff
	info = &C.fuse_file_info_t{}
	info.flags = C.O_WRONLY | C.O_APPEND
	options = internal.OpenFileOptions{Name: name, Flags: flags, Mode: mode}
	suite.mock.EXPECT().OpenFile(options).Return(&handlemap.Handle{}, nil)

	err = libfuse_open(path, info)
	suite.assert.Equal(C.int(0), err)

	flags = C.O_WRONLY & 0xffffffff
	info = &C.fuse_file_info_t{}
	info.flags = C.O_WRONLY
	options = internal.OpenFileOptions{Name: name, Flags: flags, Mode: mode}
	suite.mock.EXPECT().OpenFile(options).Return(&handlemap.Handle{}, nil)

	err = libfuse_open(path, info)
	suite.assert.Equal(C.int(0), err)
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
}

func testTruncate(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := C.CString("/" + name)
	defer C.free(unsafe.Pointer(path))
	size := int64(1024)
	options := internal.TruncateFileOptions{Name: name, OldSize: -1, NewSize: size}
	suite.mock.EXPECT().TruncateFile(options).Return(nil)

	err := libfuse2_truncate(path, C.off_t(size))
	suite.assert.Equal(C.int(0), err)
}

func testTruncateError(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := C.CString("/" + name)
	defer C.free(unsafe.Pointer(path))
	size := int64(1024)
	options := internal.TruncateFileOptions{Name: name, OldSize: -1, NewSize: size}
	suite.mock.EXPECT().TruncateFile(options).Return(errors.New("failed to truncate file"))

	err := libfuse2_truncate(path, C.off_t(size))
	suite.assert.Equal(C.int(-C.EIO), err)
}

// This test is no-op in libfuse2.
func testFTruncate(suite *libfuseTestSuite) {
}

// This test is no-op in libfuse2.
func testFTruncateError(suite *libfuseTestSuite) {
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

func testRenameDirEnametoolong(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	src := "src"
	dst := "dst"
	srcPath := C.CString("/" + src)
	dstPath := C.CString("/" + dst)
	defer C.free(unsafe.Pointer(srcPath))
	defer C.free(unsafe.Pointer(dstPath))

	srcAttr := &internal.ObjAttr{Name: src}
	srcAttr.Flags.Set(internal.PropFlagIsDir)
	suite.mock.EXPECT().GetAttr(internal.GetAttrOptions{Name: src}).Return(srcAttr, nil)
	suite.mock.EXPECT().GetAttr(internal.GetAttrOptions{Name: dst}).Return(nil, syscall.ENOENT)
	options := internal.RenameDirOptions{Src: src, Dst: dst}
	suite.mock.EXPECT().RenameDir(options).Return(syscall.ENAMETOOLONG)

	err := libfuse2_rename(srcPath, dstPath)
	suite.assert.Equal(C.int(-C.ENAMETOOLONG), err)
}

func testRenameInvalidPath(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	valid := C.CString("/file")
	invalid := C.CString("/../escape")
	defer C.free(unsafe.Pointer(valid))
	defer C.free(unsafe.Pointer(invalid))

	// The mock fails the test if any component method is called.
	suite.assert.Equal(C.int(-C.EINVAL), libfuse2_rename(invalid, valid))
	suite.assert.Equal(C.int(-C.EINVAL), libfuse2_rename(valid, invalid))
}

func testRenameBackslashName(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	src := "src"
	dst := `..\..\etc\crontab`
	srcPath := C.CString("/" + src)
	dstPath := C.CString("/" + dst)
	defer C.free(unsafe.Pointer(srcPath))
	defer C.free(unsafe.Pointer(dstPath))

	// The backslash name must reach the next component unchanged, not as "../../etc/crontab".
	srcAttr := &internal.ObjAttr{Name: src}
	suite.mock.EXPECT().GetAttr(internal.GetAttrOptions{Name: src}).Return(srcAttr, nil)
	suite.mock.EXPECT().GetAttr(internal.GetAttrOptions{Name: dst}).Return(nil, syscall.ENOENT)
	suite.mock.EXPECT().RenameFile(internal.RenameFileOptions{Src: src, Dst: dst, SrcAttr: srcAttr}).Return(nil)

	err := libfuse2_rename(srcPath, dstPath)
	suite.assert.Equal(C.int(0), err)
}

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
	target := "target"
	getAttrOpt := internal.GetAttrOptions{Name: name}
	suite.mock.EXPECT().GetAttr(getAttrOpt).Return(&internal.ObjAttr{Size: int64(len(target))}, nil)
	options := internal.ReadLinkOptions{Name: name, Size: int64(len(target))}
	suite.mock.EXPECT().ReadLink(options).Return(target, nil)

	buf := (*C.char)(C.malloc(7))
	defer C.free(unsafe.Pointer(buf))
	err := libfuse_readlink(path, buf, 7)
	suite.assert.Equal(C.int(0), err)
	got := C.GoString(buf)
	suite.assert.Equal(target, got)
}

func testReadLinkEmpty(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := C.CString("/" + name)
	defer C.free(unsafe.Pointer(path))
	getAttrOpt := internal.GetAttrOptions{Name: name}
	suite.mock.EXPECT().GetAttr(getAttrOpt).Return(&internal.ObjAttr{}, nil)
	options := internal.ReadLinkOptions{Name: name}
	suite.mock.EXPECT().ReadLink(options).Return("", nil)

	buf := C.CString("x")
	defer C.free(unsafe.Pointer(buf))
	err := libfuse_readlink(path, buf, 2)
	suite.assert.Equal(C.int(0), err)
	suite.assert.Empty(C.GoString(buf))
}

func testReadLinkMaxTarget(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := C.CString("/" + name)
	defer C.free(unsafe.Pointer(path))
	target := strings.Repeat("A", common.MaxSymlinkTargetLen)
	getAttrOpt := internal.GetAttrOptions{Name: name}
	suite.mock.EXPECT().GetAttr(getAttrOpt).Return(&internal.ObjAttr{Size: int64(len(target))}, nil)
	options := internal.ReadLinkOptions{Name: name, Size: int64(len(target))}
	suite.mock.EXPECT().ReadLink(options).Return(target, nil)

	// libfuse passes a PATH_MAX + 1 byte buffer
	const bufSize = 4097
	buf := (*C.char)(C.malloc(bufSize))
	defer C.free(unsafe.Pointer(buf))
	err := libfuse_readlink(path, buf, bufSize)
	suite.assert.Equal(C.int(0), err)
	got := C.GoString(buf)
	suite.assert.Equal(target, got)
}

func testReadLinkNotExists(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := C.CString("/" + name)
	defer C.free(unsafe.Pointer(path))
	getAttrOpt := internal.GetAttrOptions{Name: name}
	suite.mock.EXPECT().GetAttr(getAttrOpt).Return(nil, syscall.ENOENT)

	buf := C.CString("")
	defer C.free(unsafe.Pointer(buf))
	err := libfuse_readlink(path, buf, 1)
	suite.assert.Equal(C.int(-C.ENOENT), err)
}

func testReadLinkGetAttrError(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := C.CString("/" + name)
	defer C.free(unsafe.Pointer(path))
	getAttrOpt := internal.GetAttrOptions{Name: name}
	suite.mock.EXPECT().GetAttr(getAttrOpt).Return(nil, errors.New("failed to get attr"))

	buf := C.CString("")
	defer C.free(unsafe.Pointer(buf))
	err := libfuse_readlink(path, buf, 1)
	suite.assert.Equal(C.int(-C.EIO), err)
}

func testReadLinkNilAttr(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := C.CString("/" + name)
	defer C.free(unsafe.Pointer(path))
	getAttrOpt := internal.GetAttrOptions{Name: name}
	suite.mock.EXPECT().GetAttr(getAttrOpt).Return(nil, nil)

	buf := C.CString("")
	defer C.free(unsafe.Pointer(buf))
	err := libfuse_readlink(path, buf, 1)
	suite.assert.Equal(C.int(-C.EIO), err)
}

func testReadLinkError(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := C.CString("/" + name)
	defer C.free(unsafe.Pointer(path))
	getAttrOpt := internal.GetAttrOptions{Name: name}
	suite.mock.EXPECT().GetAttr(getAttrOpt).Return(&internal.ObjAttr{Size: 6}, nil)
	options := internal.ReadLinkOptions{Name: name, Size: 6}
	suite.mock.EXPECT().ReadLink(options).Return("", errors.New("failed to read link"))

	buf := (*C.char)(C.malloc(7))
	defer C.free(unsafe.Pointer(buf))
	err := libfuse_readlink(path, buf, 7)
	suite.assert.Equal(C.int(-C.EIO), err)
}

// A target longer than PATH_MAX - 1 is rejected before anything is downloaded.
func testReadLinkTargetTooLong(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := C.CString("/" + name)
	defer C.free(unsafe.Pointer(path))
	getAttrOpt := internal.GetAttrOptions{Name: name}
	suite.mock.EXPECT().GetAttr(getAttrOpt).Return(&internal.ObjAttr{Size: 16 << 20}, nil)

	const bufSize = 4097
	buf := (*C.char)(C.malloc(bufSize))
	defer C.free(unsafe.Pointer(buf))
	err := libfuse_readlink(path, buf, bufSize)
	suite.assert.Equal(C.int(-C.ENAMETOOLONG), err)
}

// A target that does not fit in a buffer smaller than PATH_MAX is rejected as well.
func testReadLinkTargetExceedsBuffer(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := C.CString("/" + name)
	defer C.free(unsafe.Pointer(path))
	getAttrOpt := internal.GetAttrOptions{Name: name}
	const bufSize = 16
	suite.mock.EXPECT().GetAttr(getAttrOpt).Return(&internal.ObjAttr{Size: bufSize}, nil)

	buf := (*C.char)(C.malloc(bufSize))
	defer C.free(unsafe.Pointer(buf))
	err := libfuse_readlink(path, buf, bufSize)
	suite.assert.Equal(C.int(-C.ENAMETOOLONG), err)
}

// If the next component returns more than it was asked for, nothing may be written past the buffer.
func testReadLinkTargetLongerThanAttr(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := C.CString("/" + name)
	defer C.free(unsafe.Pointer(path))

	const bufSize = 16
	const allocSize = 64
	target := strings.Repeat("A", 32)

	getAttrOpt := internal.GetAttrOptions{Name: name}
	suite.mock.EXPECT().GetAttr(getAttrOpt).Return(&internal.ObjAttr{Size: 6}, nil)
	options := internal.ReadLinkOptions{Name: name, Size: 6}
	suite.mock.EXPECT().ReadLink(options).Return(target, nil)

	mem := C.malloc(allocSize)
	defer C.free(mem)
	raw := unsafe.Slice((*byte)(mem), allocSize)
	for i := range raw {
		raw[i] = 0xAA
	}

	err := libfuse_readlink(path, (*C.char)(mem), bufSize)
	suite.assert.Equal(C.int(-C.ENAMETOOLONG), err)
	for i := range raw {
		suite.assert.Equal(byte(0xAA), raw[i], "byte %d of the buffer was modified", i)
	}
}

func testReadLinkZeroSize(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	path := C.CString("/path")
	defer C.free(unsafe.Pointer(path))

	buf := C.CString("")
	defer C.free(unsafe.Pointer(buf))
	err := libfuse_readlink(path, buf, 0)
	suite.assert.Equal(C.int(-C.EINVAL), err)
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
	suite.assert.NotEqual(C.ulong(0), info.fh)

	// libfuse component will return back handle in form of an integer value
	// that needs to be converted back to a pointer to a handle object
	fobj := (*fileHandle)(unsafe.Pointer(uintptr(info.fh)))
	handle = (*handlemap.Handle)(unsafe.Pointer(uintptr(fobj.obj)))

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
	suite.assert.NotEqual(C.ulong(0), info.fh)

	// libfuse component will return back handle in form of an integer value
	// that needs to be converted back to a pointer to a handle object
	fobj := (*fileHandle)(unsafe.Pointer(uintptr(info.fh)))
	handle = (*handlemap.Handle)(unsafe.Pointer(uintptr(fobj.obj)))

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

	err := libfuse2_chmod(path, 0775)
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

	err := libfuse2_chmod(path, 0775)
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

	err := libfuse2_chmod(path, 0775)
	suite.assert.Equal(C.int(-C.EIO), err)
}

func testChown(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := C.CString("/" + name)
	defer C.free(unsafe.Pointer(path))
	group := C.uint(5)
	owner := C.uint(4)

	err := libfuse2_chown(path, owner, group)
	suite.assert.Equal(C.int(0), err)
}

func testUtimens(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := C.CString("/" + name)
	defer C.free(unsafe.Pointer(path))

	err := libfuse2_utimens(path, nil)
	suite.assert.Equal(C.int(0), err)
}
