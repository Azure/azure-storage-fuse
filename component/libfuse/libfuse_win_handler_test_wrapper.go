//go:build windows

/*
    _____           _____   _____   ____          ______  _____  ------
   |     |  |      |     | |     | |     |     | |       |            |
   |     |  |      |     | |     | |     |     | |       |            |
   | --- |  |      |     | |-----| |---- |     | |-----| |-----  ------
   |     |  |      |     | |     | |     |     |       | |       |
   | ____|  |_____ | ____| | ____| |     |_____|  _____| |_____  |_____
   Licensed under the MIT License <http://opensource.org/licenses/MIT>.
   Copyright © 2020-2025 Microsoft Corporation. All rights reserved.
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

import (
	"errors"
	"fmt"
	"io/fs"
	"runtime"
	"strings"
	"syscall"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/config"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal"
	"github.com/Azure/azure-storage-fuse/v2/internal/handlemap"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"github.com/winfsp/cgofuse/fuse"
)

type libfuseTestSuite struct {
	suite.Suite
	assert   *assert.Assertions
	libfuse  *Libfuse
	cgofuse  *CgofuseFS
	mockCtrl *gomock.Controller
	mock     *internal.MockComponent
}

// Open and create call returns this kind of object
var emptyConfig = ""

// For fuse calls
var cfuseFS *CgofuseFS

func newTestLibfuse(next internal.Component, configuration string) *Libfuse {
	err := config.ReadConfigFromReader(strings.NewReader(configuration))
	if err != nil {
		panic(fmt.Sprintf("Unable to read config from reader: %v", err))
	}
	libfuse := NewLibfuseComponent()
	libfuse.SetNextComponent(next)
	err = libfuse.Configure(true)
	if err != nil {
		panic(fmt.Sprintf("Unable to configure for testing: %v", err))
	}

	return libfuse.(*Libfuse)
}

func (suite *libfuseTestSuite) SetupTest() {
	err := log.SetDefaultLogger("silent", common.LogConfig{})
	if err != nil {
		panic(fmt.Sprintf("Unable to set silent logger as default: %v", err))
	}
	suite.setupTestHelper(emptyConfig)
}

func (suite *libfuseTestSuite) setupTestHelper(config string) {
	suite.assert = assert.New(suite.T())

	suite.mockCtrl = gomock.NewController(suite.T())
	suite.mock = internal.NewMockComponent(suite.mockCtrl)
	suite.libfuse = newTestLibfuse(suite.mock, config)
	suite.cgofuse = &CgofuseFS{}
	cfuseFS = suite.cgofuse
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
	path := "/" + name
	mode := fs.FileMode(0775)
	options := internal.CreateDirOptions{Name: name, Mode: mode}
	suite.mock.EXPECT().CreateDir(options).Return(nil)

	// On Windows we test for directory creation, so we have a call to GetAttr
	if runtime.GOOS == "windows" {
		option := internal.GetAttrOptions{Name: name}
		suite.mock.EXPECT().GetAttr(option).Return(nil, syscall.ENOENT)
	}

	err := cfuseFS.Mkdir(path, 0775)
	suite.assert.Equal(0, err)
}

func testStatFs(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	path := "/"
	suite.mock.EXPECT().StatFs().Return(&common.Statfs_t{Frsize: 1,
		Blocks: 2, Bavail: 3, Bfree: 4, Bsize: 5, Files: 6, Ffree: 7, Namemax: 8}, true, nil)
	buf := &fuse.Statfs_t{}
	ret := cfuseFS.Statfs(path, buf)

	suite.assert.Equal(0, ret)
	suite.assert.Equal(1, int(buf.Frsize))
	suite.assert.Equal(2, int(buf.Blocks))
	suite.assert.Equal(3, int(buf.Bavail))
	suite.assert.Equal(4, int(buf.Bfree))
	suite.assert.Equal(5, int(buf.Bsize))
	suite.assert.Equal(6, int(buf.Files))
	suite.assert.Equal(7, int(buf.Ffree))
	suite.assert.Equal(8, int(buf.Namemax))
}

func testStatFsNotPopulated(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	path := "/"
	suite.mock.EXPECT().StatFs().Return(nil, false, nil)
	buf := &fuse.Statfs_t{}
	ret := cfuseFS.Statfs(path, buf)

	suite.assert.Equal(0, ret)

	// By default these are all 0, so they should be populated by the system
	suite.assert.NotZero(buf.Frsize)
	suite.assert.NotZero(buf.Blocks)
	suite.assert.NotZero(buf.Bavail)
	suite.assert.NotZero(buf.Bfree)
	suite.assert.NotZero(buf.Bsize)
	suite.assert.NotZero(buf.Files)
	suite.assert.NotZero(buf.Ffree)
	suite.assert.NotZero(buf.Namemax)
}

func testStatFsError(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	path := "/"
	suite.mock.EXPECT().StatFs().Return(nil, false, errors.New("error"))
	buf := &fuse.Statfs_t{}
	ret := cfuseFS.Statfs(path, buf)
	suite.assert.Equal(ret, -fuse.EIO)
}

func testMkDirError(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := "/" + name
	mode := fs.FileMode(0775)
	options := internal.CreateDirOptions{Name: name, Mode: mode}
	suite.mock.EXPECT().CreateDir(options).Return(errors.New("failed to create directory"))

	// On Windows we test for directory creation, so we have a call to GetAttr
	if runtime.GOOS == "windows" {
		option := internal.GetAttrOptions{Name: name}
		suite.mock.EXPECT().GetAttr(option).Return(nil, syscall.ENOENT)
	}

	err := cfuseFS.Mkdir(path, 0775)
	suite.assert.Equal(-fuse.EIO, err)
}

// testMkDirErrorAttrExist only runs on Windows to test the case that the directory already exists
// and the attributes state it is a directory.
func testMkDirErrorAttrExist(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	if runtime.GOOS != "windows" {
		return
	}
	name := "path"
	path := "/" + name
	option := internal.GetAttrOptions{Name: name}
	suite.mock.EXPECT().GetAttr(option).Return(&internal.ObjAttr{Flags: internal.NewDirBitMap()}, nil)
	err := cfuseFS.Mkdir(path, 0775)
	suite.assert.Equal(-fuse.EEXIST, err)
}

// TODO: ReadDir test

func testRmDir(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := "/" + name
	isDirEmptyOptions := internal.IsDirEmptyOptions{Name: name}
	suite.mock.EXPECT().IsDirEmpty(isDirEmptyOptions).Return(true)
	deleteDirOptions := internal.DeleteDirOptions{Name: name}
	suite.mock.EXPECT().DeleteDir(deleteDirOptions).Return(nil)

	err := cfuseFS.Rmdir(path)
	suite.assert.Equal(0, err)
}

func testRmDirNotEmpty(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := "/" + name
	isDirEmptyOptions := internal.IsDirEmptyOptions{Name: name}
	suite.mock.EXPECT().IsDirEmpty(isDirEmptyOptions).Return(false)

	err := cfuseFS.Rmdir(path)
	suite.assert.Equal(-fuse.ENOTEMPTY, err)
}

func testRmDirError(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := "/" + name
	isDirEmptyOptions := internal.IsDirEmptyOptions{Name: name}
	suite.mock.EXPECT().IsDirEmpty(isDirEmptyOptions).Return(true)
	deleteDirOptions := internal.DeleteDirOptions{Name: name}
	suite.mock.EXPECT().DeleteDir(deleteDirOptions).Return(errors.New("failed to delete directory"))

	err := cfuseFS.Rmdir(path)
	suite.assert.Equal(-fuse.EIO, err)
}

func testCreate(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := "/" + name
	mode := fs.FileMode(0775)
	options := internal.CreateFileOptions{Name: name, Mode: mode}
	suite.mock.EXPECT().CreateFile(options).Return(&handlemap.Handle{}, nil)

	err, fh := cfuseFS.Create(path, 0, uint32(mode))
	suite.assert.Equal(0, err)

	option := internal.GetAttrOptions{Name: name}
	suite.mock.EXPECT().GetAttr(option).Return(&internal.ObjAttr{}, nil)
	stbuf := &fuse.Stat_t{}
	err = cfuseFS.Getattr(path, stbuf, fh)
	suite.assert.Equal(0, err)
	suite.assert.Equal(int64(0), stbuf.Mtim.Nsec)
	suite.assert.NotEqual(int64(0), stbuf.Mtim.Sec)
}

func testCreateError(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := "/" + name
	mode := fs.FileMode(0775)
	options := internal.CreateFileOptions{Name: name, Mode: mode}
	suite.mock.EXPECT().CreateFile(options).Return(&handlemap.Handle{}, errors.New("failed to create file"))

	err, _ := cfuseFS.Create(path, 0, uint32(mode))
	suite.assert.Equal(-fuse.EIO, err)
}

func testOpen(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := "/" + name
	mode := fs.FileMode(fuseFS.filePermission)
	flags := fuse.O_RDWR & 0xffffffff
	options := internal.OpenFileOptions{Name: name, Flags: flags, Mode: mode}
	suite.mock.EXPECT().OpenFile(options).Return(&handlemap.Handle{}, nil)

	err, _ := cfuseFS.Open(path, flags)
	suite.assert.Equal(0, err)
}

func testOpenSyncDirectFlag(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := "/" + name
	mode := fs.FileMode(fuseFS.filePermission)
	flags := fuse.O_RDWR & 0xffffffff
	openFlags := fuse.O_RDWR | O_SYNC | __O_DIRECT
	options := internal.OpenFileOptions{Name: name, Flags: flags, Mode: mode}
	suite.mock.EXPECT().OpenFile(options).Return(&handlemap.Handle{}, nil)

	err, _ := cfuseFS.Open(path, openFlags)
	suite.assert.Equal(0, err)
	suite.assert.Equal(0, openFlags&O_SYNC)
	suite.assert.Equal(0, openFlags&__O_DIRECT)
}

// fuse2 does not have writeback caching, so append flag is passed unchanged
func testOpenAppendFlagDefault(suite *libfuseTestSuite) {
	defer suite.cleanupTest()

	name := "path"
	path := "/" + name
	mode := fs.FileMode(fuseFS.filePermission)
	flags := fuse.O_RDWR | fuse.O_APPEND&0xffffffff
	options := internal.OpenFileOptions{Name: name, Flags: flags, Mode: mode}
	suite.mock.EXPECT().OpenFile(options).Return(&handlemap.Handle{}, nil)

	err, _ := cfuseFS.Open(path, flags)
	suite.assert.Equal(0, err)

	flags = fuse.O_WRONLY | fuse.O_APPEND&0xffffffff
	options = internal.OpenFileOptions{Name: name, Flags: flags, Mode: mode}
	suite.mock.EXPECT().OpenFile(options).Return(&handlemap.Handle{}, nil)

	err, _ = cfuseFS.Open(path, flags)
	suite.assert.Equal(0, err)
}

func testOpenAppendFlagDisableWritebackCache(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	suite.cleanupTest() // clean up the default libfuse generated
	config := "libfuse:\n  disable-writeback-cache: true\n"
	suite.setupTestHelper(config) // setup a new libfuse with a custom config (clean up will occur after the test as usual)
	suite.assert.True(suite.libfuse.disableWritebackCache)

	name := "path"
	path := "/" + name
	mode := fs.FileMode(fuseFS.filePermission)
	flags := fuse.O_RDWR | fuse.O_APPEND&0xffffffff
	options := internal.OpenFileOptions{Name: name, Flags: flags, Mode: mode}
	suite.mock.EXPECT().OpenFile(options).Return(&handlemap.Handle{}, nil)

	err, _ := cfuseFS.Open(path, flags)
	suite.assert.Equal(0, err)

	flags = fuse.O_WRONLY | fuse.O_APPEND&0xffffffff
	options = internal.OpenFileOptions{Name: name, Flags: flags, Mode: mode}
	suite.mock.EXPECT().OpenFile(options).Return(&handlemap.Handle{}, nil)

	err, _ = cfuseFS.Open(path, flags)
	suite.assert.Equal(0, err)
}

func testOpenAppendFlagIgnoreAppendFlag(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	suite.cleanupTest() // clean up the default libfuse generated
	config := "libfuse:\n  ignore-open-flags: true\n"
	suite.setupTestHelper(config) // setup a new libfuse with a custom config (clean up will occur after the test as usual)
	suite.assert.True(suite.libfuse.ignoreOpenFlags)

	name := "path"
	path := "/" + name
	mode := fs.FileMode(fuseFS.filePermission)
	flags := fuse.O_RDWR | fuse.O_APPEND&0xffffffff
	options := internal.OpenFileOptions{Name: name, Flags: flags, Mode: mode}
	suite.mock.EXPECT().OpenFile(options).Return(&handlemap.Handle{}, nil)

	err, _ := cfuseFS.Open(path, flags)
	suite.assert.Equal(0, err)

	flags = fuse.O_WRONLY | fuse.O_APPEND&0xffffffff
	options = internal.OpenFileOptions{Name: name, Flags: flags, Mode: mode}
	suite.mock.EXPECT().OpenFile(options).Return(&handlemap.Handle{}, nil)

	err, _ = cfuseFS.Open(path, flags)
	suite.assert.Equal(0, err)

	flags = fuse.O_WRONLY & 0xffffffff
	options = internal.OpenFileOptions{Name: name, Flags: flags, Mode: mode}
	suite.mock.EXPECT().OpenFile(options).Return(&handlemap.Handle{}, nil)

	err, _ = cfuseFS.Open(path, flags)
	suite.assert.Equal(0, err)
}

func testOpenNotExists(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := "/" + name
	mode := fs.FileMode(fuseFS.filePermission)
	flags := fuse.O_RDWR & 0xffffffff
	options := internal.OpenFileOptions{Name: name, Flags: flags, Mode: mode}
	suite.mock.EXPECT().OpenFile(options).Return(&handlemap.Handle{}, syscall.ENOENT)

	err, _ := cfuseFS.Open(path, flags)
	suite.assert.Equal(-fuse.ENOENT, err)
}

func testOpenError(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := "/" + name
	mode := fs.FileMode(fuseFS.filePermission)
	flags := fuse.O_RDWR & 0xffffffff
	options := internal.OpenFileOptions{Name: name, Flags: flags, Mode: mode}
	suite.mock.EXPECT().OpenFile(options).Return(&handlemap.Handle{}, errors.New("failed to open a file"))

	err, _ := cfuseFS.Open(path, flags)
	suite.assert.Equal(-fuse.EIO, err)
}

func testTruncate(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := "/" + name
	size := int64(1024)
	options := internal.TruncateFileOptions{Name: name, OldSize: -1, NewSize: size}
	suite.mock.EXPECT().TruncateFile(options).Return(nil)

	err := cfuseFS.Truncate(path, size, 0)
	suite.assert.Equal(0, err)
}

func testTruncateError(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := "/" + name
	size := int64(1024)
	options := internal.TruncateFileOptions{Name: name, OldSize: -1, NewSize: size}
	suite.mock.EXPECT().TruncateFile(options).Return(errors.New("failed to truncate file"))

	err := cfuseFS.Truncate(path, size, 0)
	suite.assert.Equal(-fuse.EIO, err)
}

func testUnlink(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := "/" + name
	options := internal.DeleteFileOptions{Name: name}
	suite.mock.EXPECT().DeleteFile(options).Return(nil)

	err := cfuseFS.Unlink(path)
	suite.assert.Equal(0, err)
}

func testUnlinkNotExists(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := "/" + name
	options := internal.DeleteFileOptions{Name: name}
	suite.mock.EXPECT().DeleteFile(options).Return(syscall.ENOENT)

	err := cfuseFS.Unlink(path)
	suite.assert.Equal(-fuse.ENOENT, err)
}

func testUnlinkError(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := "/" + name
	options := internal.DeleteFileOptions{Name: name}
	suite.mock.EXPECT().DeleteFile(options).Return(errors.New("failed to delete file"))

	err := cfuseFS.Unlink(path)
	suite.assert.Equal(-fuse.EIO, err)
}

// Rename

func testSymlink(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	target := "target"
	path := "/" + name
	t := target
	options := internal.CreateLinkOptions{Name: name, Target: target}
	suite.mock.EXPECT().CreateLink(options).Return(nil)

	err := cfuseFS.Symlink(t, path)
	suite.assert.Equal(0, err)
}

func testSymlinkError(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	target := "target"
	path := "/" + name
	t := target
	options := internal.CreateLinkOptions{Name: name, Target: target}
	suite.mock.EXPECT().CreateLink(options).Return(errors.New("failed to create link"))

	err := cfuseFS.Symlink(t, path)
	suite.assert.Equal(-fuse.EIO, err)
}

func testReadLink(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := "/" + name
	options := internal.ReadLinkOptions{Name: name}
	suite.mock.EXPECT().ReadLink(options).Return("target", nil)
	attr := &internal.ObjAttr{}
	getAttrOpt := internal.GetAttrOptions{Name: name}
	suite.mock.EXPECT().GetAttr(getAttrOpt).Return(attr, nil)

	err, target := cfuseFS.Readlink(path)
	suite.assert.Equal(0, err)
	suite.assert.Equal("target", target)
}

func testReadLinkNotExists(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := "/" + name
	options := internal.ReadLinkOptions{Name: name}
	suite.mock.EXPECT().ReadLink(options).Return("", syscall.ENOENT)
	attr := &internal.ObjAttr{}
	getAttrOpt := internal.GetAttrOptions{Name: name}
	suite.mock.EXPECT().GetAttr(getAttrOpt).Return(attr, nil)

	err, target := cfuseFS.Readlink(path)
	suite.assert.Equal(-fuse.ENOENT, err)
	suite.assert.NotEqual("target", target)
}

func testReadLinkError(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := "/" + name
	options := internal.ReadLinkOptions{Name: name}
	suite.mock.EXPECT().ReadLink(options).Return("", errors.New("failed to read link"))
	getAttrOpt := internal.GetAttrOptions{Name: name}
	suite.mock.EXPECT().GetAttr(getAttrOpt).Return(nil, nil)

	err, target := cfuseFS.Readlink(path)
	suite.assert.Equal(-fuse.EIO, err)
	suite.assert.NotEqual("target", target)
}

func testFsync(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := "/" + name
	mode := fs.FileMode(fuseFS.filePermission)
	flags := fuse.O_RDWR & 0xffffffff
	handle := &handlemap.Handle{}
	openOptions := internal.OpenFileOptions{Name: name, Flags: flags, Mode: mode}
	suite.mock.EXPECT().OpenFile(openOptions).Return(handle, nil)
	_, fh := cfuseFS.Open(path, flags)
	suite.assert.NotEqual(uint64(0), fh)

	// Need to convert the ID to the correct filehandle for mocking
	handle.ID = (handlemap.HandleID)(fh)

	options := internal.SyncFileOptions{Handle: handle}
	suite.mock.EXPECT().SyncFile(options).Return(nil)

	err := cfuseFS.Fsync(path, false, fh)
	suite.assert.Equal(0, err)
}

func testFsyncHandleError(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := "/" + name
	fh := uint64(0)

	err := cfuseFS.Fsync(path, false, fh)
	suite.assert.Equal(-fuse.EIO, err)
}

func testFsyncError(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := "/" + name
	mode := fs.FileMode(fuseFS.filePermission)
	flags := fuse.O_RDWR & 0xffffffff
	handle := &handlemap.Handle{}

	openOptions := internal.OpenFileOptions{Name: name, Flags: flags, Mode: mode}
	suite.mock.EXPECT().OpenFile(openOptions).Return(handle, nil)
	_, fh := cfuseFS.Open(path, flags)
	suite.assert.NotEqual(uint64(0), fh)

	// Need to convert the ID to the correct filehandle for mocking
	handle.ID = (handlemap.HandleID)(fh)

	options := internal.SyncFileOptions{Handle: handle}
	suite.mock.EXPECT().SyncFile(options).Return(errors.New("failed to sync file"))

	err := cfuseFS.Fsync(path, false, fh)
	suite.assert.Equal(-fuse.EIO, err)
}

func testFsyncDir(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := "/" + name
	options := internal.SyncDirOptions{Name: name}
	suite.mock.EXPECT().SyncDir(options).Return(nil)

	err := cfuseFS.Fsyncdir(path, false, 0)
	suite.assert.Equal(0, err)
}

func testFsyncDirError(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := "/" + name
	options := internal.SyncDirOptions{Name: name}
	suite.mock.EXPECT().SyncDir(options).Return(errors.New("failed to sync dir"))

	err := cfuseFS.Fsyncdir(path, false, 0)
	suite.assert.Equal(-fuse.EIO, err)
}

func testChmod(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := "/" + name
	mode := fs.FileMode(0775)
	options := internal.ChmodOptions{Name: name, Mode: mode}
	suite.mock.EXPECT().Chmod(options).Return(nil)

	err := cfuseFS.Chmod(path, 0775)
	suite.assert.Equal(0, err)
}

func testChmodNotExists(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := "/" + name
	mode := fs.FileMode(0775)
	options := internal.ChmodOptions{Name: name, Mode: mode}
	suite.mock.EXPECT().Chmod(options).Return(syscall.ENOENT)

	err := cfuseFS.Chmod(path, 0775)
	suite.assert.Equal(-fuse.ENOENT, err)
}

func testChmodError(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := "/" + name
	mode := fs.FileMode(0775)
	options := internal.ChmodOptions{Name: name, Mode: mode}
	suite.mock.EXPECT().Chmod(options).Return(errors.New("failed to chmod"))

	err := cfuseFS.Chmod(path, 0775)
	suite.assert.Equal(-fuse.EIO, err)
}

func testChown(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := "/" + name
	group := uint32(5)
	owner := uint32(4)

	err := cfuseFS.Chown(path, owner, group)
	suite.assert.Equal(0, err)
}

func testUtimens(suite *libfuseTestSuite) {
	defer suite.cleanupTest()
	name := "path"
	path := "/" + name

	err := cfuseFS.Utimens(path, nil)
	suite.assert.Equal(0, err)
}
