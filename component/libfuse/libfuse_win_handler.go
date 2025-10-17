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
	"io"
	"io/fs"
	"os"
	"runtime"
	"syscall"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/config"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal"
	"github.com/Azure/azure-storage-fuse/v2/internal/handlemap"
	"github.com/Azure/azure-storage-fuse/v2/internal/stats_manager"

	"github.com/winfsp/cgofuse/fuse"
)

// CgofuseFS defines the file system with functions that interface with FUSE.
type CgofuseFS struct {
	// Implement the interface from cgofuse
	fuse.FileSystemBase

	// user identifier on linux
	uid uint32

	// group identifier on linux
	gid uint32
}

const windowsDefaultSDDL = "D:P(A;;FA;;;WD)" // Enables everyone on system to have access to mount

const blockSize = 4096
const defaultDisplayCapacity = 1024 * common.TbToBytes
const O_SYNC = 0x101000
const __O_DIRECT = 0x4000

func trimFusePathWin(path string) string {
	if path == "" {
		return ""
	}

	if path[0] == '/' {
		return path[1:]
	}
	return path
}

// initFuse passes the launch options for fuse and starts the mount.
// Here are the options for FUSE.
// LINK: https://man7.org/linux/man-pages/man8/mount.fuse3.8.html
func (lf *Libfuse) initFuse() error {
	log.Trace("Libfuse::initFuse : Initializing FUSE")

	cf := NewcgofuseFS()
	cf.uid = lf.ownerUID
	cf.gid = lf.ownerGID

	lf.windowsHost = fuse.NewFileSystemHost(cf)
	// prevent Windows from calling GetAttr redundantly
	lf.windowsHost.SetCapReaddirPlus(true)

	options := fmt.Sprintf("uid=%d,gid=%d,entry_timeout=%d,attr_timeout=%d,negative_timeout=%d",
		lf.ownerUID,
		lf.ownerGID,
		lf.entryExpiration,
		lf.attributeExpiration,
		lf.negativeTimeout)

	// With WinFSP this will present all files as owned by the Authenticated Users group
	if runtime.GOOS == "windows" {
		// if uid & gid were not specified, pass -1 for both (which will cause WinFSP to look up the current user)
		uid := int64(-1)
		gid := int64(-1)
		if lf.ownerUID != 0 {
			uid = int64(lf.ownerUID)
		}
		if lf.ownerGID != 0 {
			gid = int64(lf.ownerGID)
		}
		options = fmt.Sprintf("uid=%d,gid=%d,entry_timeout=%d,attr_timeout=%d,negative_timeout=%d",
			uid,
			gid,
			lf.entryExpiration,
			lf.attributeExpiration,
			lf.negativeTimeout)

		// Using SSDL file security option: https://github.com/rclone/rclone/issues/4717
		options += ",FileSecurity=" + windowsDefaultSDDL
	}

	fuse_options := createFuseOptions(
		lf.windowsHost,
		lf.allowOther,
		lf.allowRoot,
		lf.readOnly,
		lf.nonEmptyMount,
		lf.maxFuseThreads,
		lf.umask,
	)
	options += fuse_options

	// Setup options as a slice
	opts := []string{"-o", options}

	// Runs as network file share on Windows only when mounting to drive letter.
	if runtime.GOOS == "windows" && lf.windowsNetworkShare && common.IsDriveLetter(lf.mountPath) {
		var nameStorage string

		serverName, err := os.Hostname()
		if err != nil {
			log.Err(
				"Libfuse::initFuse : failed to mount fuse. unable to determine server host name.",
			)
			return errors.New("failed to mount fuse. unable to determine server host name")
		}
		// Borrow bucket-name string from attribute cache
		if config.IsSet("s3storage.bucket-name") {
			err := config.UnmarshalKey("s3storage.bucket-name", &nameStorage)
			if err != nil {
				nameStorage = "s3"
				log.Err("initFuse : Failed to unmarshal s3storage.bucket-name")
			}
		} else if config.IsSet("azstorage.container") {
			err := config.UnmarshalKey("azstorage.container", &nameStorage)
			if err != nil {
				nameStorage = "azure"
				log.Err("initFuse : Failed to unmarshal s3storage.bucket-name")
			}
		}

		volumePrefix := fmt.Sprintf("--VolumePrefix=\\%s\\%s", serverName, nameStorage)
		opts = append(opts, volumePrefix)
	}

	// Enabling trace is done by using -d rather than setting an option in fuse
	if lf.traceEnable {
		opts = append(opts, "-d")
	}

	ret := lf.windowsHost.Mount(lf.mountPath, opts)
	if !ret {
		log.Err("Libfuse::initFuse : failed to mount fuse")
		return errors.New("failed to mount fuse")
	}

	return nil
}

func createFuseOptions(
	host *fuse.FileSystemHost,
	allowOther bool,
	allowRoot bool,
	readOnly bool,
	nonEmptyMount bool,
	maxFuseThreads uint32,
	umask uint32,
) string {
	var options string
	// While reading a file let kernel do readahead for better perf
	options += fmt.Sprintf(",max_readahead=%d", 4*1024*1024)

	// Max background thread on the fuse layer for high parallelism
	options += fmt.Sprintf(",max_background=%d", maxFuseThreads)

	if allowOther {
		options += ",allow_other"
	}
	if allowRoot {
		options += ",allow_root"
	}
	if readOnly {
		options += ",ro"
	}
	if nonEmptyMount {
		options += ",nonempty"
	}

	if umask != 0 {
		options += fmt.Sprintf(",umask=%04d", umask)
	}

	// direct_io option is used to bypass the kernel cache. It disables the use of
	// page cache (file content cache) in the kernel for the filesystem.
	if fuseFS.directIO {
		options += ",direct_io"
	} else {
		options += ",kernel_cache"
	}
	return options
}

func (lf *Libfuse) destroyFuse() error {
	log.Trace("Libfuse::destroyFuse : Destroying FUSE")
	lf.windowsHost.Unmount()
	return nil
}

func (lf *Libfuse) fillStat(attr *internal.ObjAttr, stbuf *fuse.Stat_t) {
	stbuf.Uid = lf.ownerUID
	stbuf.Gid = lf.ownerGID
	stbuf.Nlink = 1
	stbuf.Size = attr.Size

	// Populate mode
	// Backing storage implementation has support for mode.
	if !attr.IsModeDefault() {
		stbuf.Mode = uint32(attr.Mode) & 0xffffffff
	} else {
		if attr.IsDir() {
			stbuf.Mode = uint32(lf.dirPermission) & 0xffffffff
		} else {
			stbuf.Mode = uint32(lf.filePermission) & 0xffffffff
		}
	}

	if attr.IsDir() {
		stbuf.Nlink = 2
		stbuf.Size = 4096
		stbuf.Mode |= fuse.S_IFDIR
	} else if attr.IsSymlink() {
		stbuf.Mode |= fuse.S_IFLNK
	} else {
		stbuf.Mode |= fuse.S_IFREG
	}

	stbuf.Atim = fuse.NewTimespec(attr.Atime)
	stbuf.Atim.Nsec = 0
	stbuf.Ctim = fuse.NewTimespec(attr.Ctime)
	stbuf.Ctim.Nsec = 0
	stbuf.Mtim = fuse.NewTimespec(attr.Mtime)
	stbuf.Mtim.Nsec = 0
	stbuf.Birthtim = fuse.NewTimespec(attr.Mtime)
	stbuf.Birthtim.Nsec = 0
}

// NewcgofuseFS creates a new empty fuse filesystem.
func NewcgofuseFS() *CgofuseFS {
	cf := &CgofuseFS{}
	return cf
}

// Init notifies the parent process once the mount is successful.
func (cf *CgofuseFS) Init() {
	log.Trace("Libfuse::Init : Initializing FUSE")

	log.Info("Libfuse::Init : Notifying parent for successful mount")
	if err := common.NotifyMountToParent(); err != nil {
		log.Err("Libfuse::initFuse : Failed to notify parent, error: [%v]", err)
	}
}

// Destroy currently does nothing.
func (cf *CgofuseFS) Destroy() {
	log.Trace("Libfuse::Destroy : Destroy")
}

// Getattr retrieves the file attributes at the path and fills them in stat.
func (cf *CgofuseFS) Getattr(path string, stat *fuse.Stat_t, fh uint64) int {
	// TODO: Currently not using filehandle
	name := trimFusePathWin(path)
	name = common.NormalizeObjectName(name)

	// Don't log these by default, as it noticeably affects performance
	// log.Trace("Libfuse::Getattr : %s", name)

	// Return the default configuration for the root
	if name == "" {
		stat.Mode = fuse.S_IFDIR | 0777
		stat.Uid = cf.uid
		stat.Gid = cf.gid
		stat.Nlink = 2
		stat.Size = 4096
		stat.Mtim = fuse.NewTimespec(time.Now())
		stat.Atim = stat.Mtim
		stat.Ctim = stat.Mtim
		return 0
	}

	// TODO: How does this work if we trim the path?
	// Check if the file is meant to be ignored
	if ignore, found := ignoreFiles[name]; found && ignore {
		return -fuse.ENOENT
	}

	// Get attributes
	attr, err := fuseFS.NextComponent().GetAttr(internal.GetAttrOptions{Name: name})
	if err != nil {
		log.Err("Libfuse::Getattr : Failed to get attributes of %s [%s]", name, err.Error())
		switch err {
		case syscall.ENOENT:
			return -fuse.ENOENT
		case syscall.EACCES:
			return -fuse.EACCES
		default:
			return -fuse.EIO
		}
	}

	// Populate stat
	fuseFS.fillStat(attr, stat)
	return 0
}

// Statfs sets file system statistics. It returns 0 if successful.
func (cf *CgofuseFS) Statfs(path string, stat *fuse.Statfs_t) int {
	name := trimFusePathWin(path)
	name = common.NormalizeObjectName(name)
	log.Trace("Libfuse::libfuse_statfs : %s", name)

	attr, populated, err := fuseFS.NextComponent().StatFs()
	if err != nil {
		log.Err("Libfuse::Statfs: Failed to get stats %s [%s]", name, err.Error())
		return -fuse.EIO
	}

	// if populated then we need to overwrite root attributes
	if populated {
		stat.Bsize = uint64(attr.Bsize)
		stat.Frsize = uint64(attr.Frsize)
		// calculate blocks used from attr
		stat.Blocks = attr.Blocks
		stat.Bavail = stat.Blocks
		stat.Bfree = stat.Blocks
		stat.Files = attr.Files
		stat.Ffree = attr.Ffree
	} else {
		stat.Bsize = blockSize
		stat.Frsize = blockSize
		displayCapacityBlocks := uint64(defaultDisplayCapacity / blockSize)
		stat.Blocks = displayCapacityBlocks
		stat.Bavail = displayCapacityBlocks
		stat.Bfree = displayCapacityBlocks
		stat.Files = 1e9
		stat.Ffree = 1e9
	}

	return 0
}

// Mkdir creates a new directory at the path with the given mode.
func (cf *CgofuseFS) Mkdir(path string, mode uint32) int {
	name := trimFusePathWin(path)
	name = common.NormalizeObjectName(name)
	log.Trace("Libfuse::Mkdir : %s", name)

	// Check if the directory already exists. On Windows we need to make this call explicitly
	_, err := fuseFS.NextComponent().GetAttr(internal.GetAttrOptions{Name: name})
	// If the the error is nil then a file or directory with this name exists
	if err == nil || errors.Is(err, fs.ErrExist) {
		return -fuse.EEXIST
	}

	err = fuseFS.NextComponent().
		CreateDir(internal.CreateDirOptions{Name: name, Mode: fs.FileMode(mode)})
	if err != nil {
		log.Err("Libfuse::Mkdir : Failed to create %s [%s]", name, err.Error())
		if os.IsPermission(err) {
			return -fuse.EACCES
		} else if os.IsExist(err) {
			return -fuse.EEXIST
		} else {
			return -fuse.EIO
		}
	}

	libfuseStatsCollector.PushEvents(createDir, name, map[string]interface{}{md: fs.FileMode(mode)})
	libfuseStatsCollector.UpdateStats(stats_manager.Increment, createDir, (int64)(1))

	return 0
}

// Opendir opens the directory at the path.
func (cf *CgofuseFS) Opendir(path string) (int, uint64) {
	name := trimFusePathWin(path)
	name = common.NormalizeObjectName(name)
	if name != "" {
		name = name + "/"
	}

	log.Trace("Libfuse::Opendir : %s", name)

	handle := handlemap.NewHandle(name)

	// For each handle created using opendir we create
	// this structure here to hold current block of children to serve readdir
	handle.SetValue("cache", &dirChildCache{
		sIndex:   0,
		eIndex:   0,
		token:    "",
		length:   0,
		children: make([]*internal.ObjAttr, 0),
	})

	fh := handlemap.Add(handle)
	log.Debug("Libfuse::Opendir : %s fh=%d", name, fh)

	return 0, uint64(fh)
}

// Releasedir opens the handle for the directory at the path.
func (cf *CgofuseFS) Releasedir(path string, fh uint64) int {
	// Get the filehandle
	handle, exists := handlemap.LoadAndDelete(handlemap.HandleID(fh))
	if !exists {
		log.Trace("Libfuse::Releasedir : Failed to release %s, handle: %d", path, fh)
		return -fuse.EBADF
	}

	log.Trace("Libfuse::Releasedir : %s, handle: %d", handle.Path, handle.ID)

	handle.Cleanup()
	return 0
}

// Readdir reads a directory at the path.
func (cf *CgofuseFS) Readdir(path string, fill func(name string, stat *fuse.Stat_t, ofst int64) bool,
	ofst int64, fh uint64) int {
	handle, exists := handlemap.Load(handlemap.HandleID(fh))
	if !exists {
		log.Trace("Libfuse::Readdir : Failed to read %s, handle: %d", path, fh)
		return -fuse.EBADF
	}

	handle.RLock()
	val, found := handle.GetValue("cache")
	handle.RUnlock()

	if !found {
		return -fuse.EIO
	}

	ofst64 := uint64(ofst)
	cacheInfo := val.(*dirChildCache)
	if ofst64 == 0 ||
		(ofst64 >= cacheInfo.eIndex && cacheInfo.token != "") {
		attrs, token, err := fuseFS.NextComponent().StreamDir(internal.StreamDirOptions{
			Name:   handle.Path,
			Offset: ofst64,
			Token:  cacheInfo.token,
			Count:  common.MaxDirListCount,
		})

		if err != nil {
			log.Err("Libfuse::Readdir : Path %s, handle: %d, offset %d. Error in retrieval", handle.Path, handle.ID, ofst64)
			if os.IsNotExist(err) {
				return -fuse.ENOENT
			} else if os.IsPermission(err) {
				return -fuse.EACCES
			}

			return -fuse.EIO
		}

		if ofst64 == 0 {
			attrs = append([]*internal.ObjAttr{{Flags: fuseFS.lsFlags, Name: "."}, {Flags: fuseFS.lsFlags, Name: ".."}}, attrs...)
		}

		cacheInfo.sIndex = ofst64
		cacheInfo.eIndex = ofst64 + uint64(len(attrs))
		cacheInfo.length = uint64(len(attrs))
		cacheInfo.token = token
		cacheInfo.children = cacheInfo.children[:0]
		cacheInfo.children = attrs
	}

	if ofst64 >= cacheInfo.eIndex {
		// If offset is still beyond the end index limit then we are done iterating
		return 0
	}

	stbuf := fuse.Stat_t{}

	// Populate the stat by calling filler
	for segmentIdx := ofst64 - cacheInfo.sIndex; segmentIdx < cacheInfo.length; segmentIdx++ {
		fuseFS.fillStat(cacheInfo.children[segmentIdx], &stbuf)

		name := cacheInfo.children[segmentIdx].Name
		fill(name, &stbuf, ofst)
	}

	return 0
}

// Rmdir deletes a directory.
func (cf *CgofuseFS) Rmdir(path string) int {
	name := trimFusePathWin(path)
	name = common.NormalizeObjectName(name)
	log.Trace("Libfuse::Rmdir : %s", name)

	empty := fuseFS.NextComponent().IsDirEmpty(internal.IsDirEmptyOptions{Name: name})
	if !empty {
		return -fuse.ENOTEMPTY
	}

	err := fuseFS.NextComponent().DeleteDir(internal.DeleteDirOptions{Name: name})
	if err != nil {
		log.Err("Libfuse::Rmdir : Failed to delete %s [%s]", name, err.Error())
		if os.IsNotExist(err) {
			return -fuse.ENOENT
		}

		return -fuse.EIO
	}

	libfuseStatsCollector.PushEvents(deleteDir, name, nil)
	libfuseStatsCollector.UpdateStats(stats_manager.Increment, deleteDir, (int64)(1))

	return 0
}

// Create creates a new file and opens it.
func (cf *CgofuseFS) Create(path string, flags int, mode uint32) (int, uint64) {
	name := trimFusePathWin(path)
	name = common.NormalizeObjectName(name)
	log.Trace("Libfuse::Create : %s", name)

	handle, err := fuseFS.NextComponent().
		CreateFile(internal.CreateFileOptions{Name: name, Mode: fs.FileMode(mode)})
	if err != nil {
		log.Err("Libfuse::Create : Failed to create %s [%s]", name, err.Error())
		if os.IsExist(err) {
			return -fuse.EEXIST, 0
		} else if os.IsPermission(err) {
			return -fuse.EACCES, 0
		}

		return -fuse.EIO, 0
	}

	fh := handlemap.Add(handle)
	log.Trace("Libfuse::Create : %s, handle %d", name, fh)

	libfuseStatsCollector.PushEvents(
		createFile,
		name,
		map[string]interface{}{md: fs.FileMode(mode)},
	)

	// increment open file handles count
	libfuseStatsCollector.UpdateStats(stats_manager.Increment, openHandles, (int64)(1))

	return 0, uint64(fh)
}

// Open opens a file.
func (cf *CgofuseFS) Open(path string, flags int) (int, uint64) {
	name := trimFusePathWin(path)
	name = common.NormalizeObjectName(name)
	log.Trace("Libfuse::Open : %s", name)
	// TODO: Should this sit behind a user option? What if we change something to support these in the future?
	// Mask out SYNC and DIRECT flags since write operation will fail
	if flags&O_SYNC != 0 || flags&__O_DIRECT != 0 {
		log.Info("Libfuse::Open : Reset flags for open %s, fi.flags %X", name, flags)
		// Blobfuse2 does not support the SYNC or DIRECT flag. If a user application passes this flag on to blobfuse2
		// and we open the file with this flag, subsequent write operations wlil fail with "Invalid argument" error.
		// Mask them out here in the open call so that write works.
		// Oracle RMAN is one such application that sends these flags during backup
		flags = flags &^ O_SYNC
		flags = flags &^ __O_DIRECT
	}

	handle, err := fuseFS.NextComponent().OpenFile(
		internal.OpenFileOptions{
			Name:  name,
			Flags: flags,
			Mode:  fs.FileMode(fuseFS.filePermission),
		})

	if err != nil {
		log.Err("Libfuse::Open : Failed to open %s [%s]", name, err.Error())
		if os.IsNotExist(err) {
			return -fuse.ENOENT, 0
		} else if os.IsPermission(err) {
			return -fuse.EACCES, 0
		}

		return -fuse.EIO, 0
	}

	fh := handlemap.Add(handle)
	log.Trace("Libfuse::Open : %s, handle %d", name, fh)

	// increment open file handles count
	libfuseStatsCollector.UpdateStats(stats_manager.Increment, openHandles, (int64)(1))

	return 0, uint64(fh)
}

// Read reads data from a file into the buffer with the given offset.
func (cf *CgofuseFS) Read(path string, buff []byte, ofst int64, fh uint64) int {
	//skipping the logging to avoid creating log noise and the performance costs from huge number of calls.
	//log.Debug("Libfuse::Read : reading path %s, handle: %d", path, fh)
	// Get the filehandle
	handle, exists := handlemap.Load(handlemap.HandleID(fh))
	if !exists {
		log.Trace("Libfuse::Read : error getting handle for path %s, handle: %d", path, fh)
		return -fuse.EBADF
	}

	offset := uint64(ofst)

	var err error
	var bytesRead int

	if handle.Cached() {
		bytesRead, err = handle.FObj.ReadAt(buff, int64(offset))
	} else {
		bytesRead, err = fuseFS.NextComponent().ReadInBuffer(
			&internal.ReadInBufferOptions{
				Handle: handle,
				Offset: int64(offset),
				Data:   buff,
			})
	}

	if err == io.EOF {
		err = nil
	}
	if err != nil {
		log.Err(
			"Libfuse::Read : error reading file %s, handle: %d [%s]",
			handle.Path,
			handle.ID,
			err.Error(),
		)
		return -fuse.EIO
	}

	return bytesRead
}

// Write writes data to a file from the buffer with the given offset.
func (cf *CgofuseFS) Write(path string, buff []byte, ofst int64, fh uint64) int {
	//skipping the logging to avoid creating log noise and the performance costs from huge number of calls
	//log.Debug("Libfuse::Write : Writing path %s, handle: %d", path, fh)
	// Get the filehandle
	handle, exists := handlemap.Load(handlemap.HandleID(fh))
	if !exists {
		log.Trace("Libfuse::Write : error getting handle for path %s, handle: %d", path, fh)
		return -fuse.EBADF
	}

	bytesWritten, err := fuseFS.NextComponent().WriteFile(
		&internal.WriteFileOptions{
			Handle:   handle,
			Offset:   ofst,
			Data:     buff,
			Metadata: nil,
		})

	if err != nil {
		log.Err(
			"Libfuse::Write : error writing file %s, handle: %d [%s]",
			handle.Path,
			handle.ID,
			err.Error(),
		)
		return -fuse.EIO
	}

	return bytesWritten
}

// Flush flushes any cached file data.
func (cf *CgofuseFS) Flush(path string, fh uint64) int {
	// Get the filehandle
	handle, exists := handlemap.Load(handlemap.HandleID(fh))
	if !exists {
		log.Trace("Libfuse::Flush : error getting handle for path %s, handle: %d", path, fh)
		return -fuse.EBADF
	}

	log.Trace("Libfuse::Flush : %s, handle: %d", handle.Path, handle.ID)

	// If the file handle is not dirty, there is no need to flush
	if !handle.Dirty() {
		return 0
	}

	err := fuseFS.NextComponent().FlushFile(internal.FlushFileOptions{Handle: handle})
	if err != nil {
		log.Err(
			"Libfuse::Flush : error flushing file %s, handle: %d [%s]",
			handle.Path,
			handle.ID,
			err.Error(),
		)
		switch err {
		case syscall.ENOENT:
			return -fuse.ENOENT
		case syscall.EACCES:
			return -fuse.EACCES
		default:
			return -fuse.EIO
		}
	}

	return 0
}

// Truncate changes the size of the given file.
func (cf *CgofuseFS) Truncate(path string, size int64, fh uint64) int {
	name := trimFusePathWin(path)
	name = common.NormalizeObjectName(name)

	log.Trace("Libfuse::Truncate : %s size %d", name, size)

	err := fuseFS.NextComponent().TruncateFile(internal.TruncateFileOptions{Name: name, OldSize: -1, NewSize: size})
	if err != nil {
		log.Err("Libfuse::Truncate : error truncating file %s [%s]", name, err.Error())
		if os.IsNotExist(err) {
			return -fuse.ENOENT
		}
		return -fuse.EIO
	}

	libfuseStatsCollector.PushEvents(truncateFile, name, map[string]interface{}{"size": size})
	libfuseStatsCollector.UpdateStats(stats_manager.Increment, truncateFile, (int64)(1))

	return 0
}

// Release closes an open file.
func (cf *CgofuseFS) Release(path string, fh uint64) int {
	// Get the filehandle
	handle, exists := handlemap.Load(handlemap.HandleID(fh))
	if !exists {
		log.Trace("Libfuse::Release : error getting handle for path %s, handle: %d", path, fh)
		return -fuse.EBADF
	}
	log.Trace("Libfuse::Release : %s, handle: %d", handle.Path, handle.ID)

	err := fuseFS.NextComponent().CloseFile(internal.CloseFileOptions{Handle: handle})
	if err != nil {
		log.Err(
			"Libfuse::Release : error closing file %s, handle: %d [%s]",
			handle.Path,
			handle.ID,
			err.Error(),
		)
		switch err {
		case syscall.ENOENT:
			return -fuse.ENOENT
		case syscall.EACCES:
			return -fuse.EACCES
		default:
			return -fuse.EIO
		}
	}

	handlemap.Delete(handle.ID)

	// decrement open file handles count
	libfuseStatsCollector.UpdateStats(stats_manager.Decrement, openHandles, (int64)(1))

	return 0
}

// Unlink deletes a file.
func (cf *CgofuseFS) Unlink(path string) int {
	name := trimFusePathWin(path)
	name = common.NormalizeObjectName(name)
	log.Trace("Libfuse::Unlink : %s", name)

	err := fuseFS.NextComponent().DeleteFile(internal.DeleteFileOptions{Name: name})
	if err != nil {
		log.Err("Libfuse::Unlink : error deleting file %s [%s]", name, err.Error())
		if os.IsNotExist(err) {
			return -fuse.ENOENT
		} else if os.IsPermission(err) {
			return -fuse.EACCES
		}
		return -fuse.EIO

	}

	libfuseStatsCollector.PushEvents(deleteFile, name, nil)
	libfuseStatsCollector.UpdateStats(stats_manager.Increment, deleteFile, (int64)(1))

	return 0
}

// Rename renames a file.
// https://man7.org/linux/man-pages/man2/rename.2.html
// errors handled: EISDIR, ENOENT, ENOTDIR, ENOTEMPTY, EEXIST
// TODO: handle EACCESS, EINVAL?
func (cf *CgofuseFS) Rename(oldpath string, newpath string) int {
	srcPath := trimFusePathWin(oldpath)
	srcPath = common.NormalizeObjectName(srcPath)
	dstPath := trimFusePathWin(newpath)
	dstPath = common.NormalizeObjectName(dstPath)
	log.Trace("Libfuse::Rename : %s -> %s", srcPath, dstPath)
	// Note: When running other commands from the command line, a lot of them seemed to handle some cases like ENOENT themselves.
	// Rename did not, so we manually check here.

	// ENOENT. Not covered: a directory component in dst does not exist
	if srcPath == "" || dstPath == "" {
		log.Err("Libfuse::Rename : src: [%s] or dst: [%s] is an empty string", srcPath, dstPath)
		return -fuse.ENOENT
	}

	srcAttr, srcErr := fuseFS.NextComponent().GetAttr(internal.GetAttrOptions{Name: srcPath})
	if os.IsNotExist(srcErr) {
		log.Err("Libfuse::Rename : Failed to get attributes of %s [%s]", srcPath, srcErr.Error())
		return -fuse.ENOENT
	}
	dstAttr, dstErr := fuseFS.NextComponent().GetAttr(internal.GetAttrOptions{Name: dstPath})

	// EISDIR
	if (dstErr == nil || os.IsExist(dstErr)) && dstAttr.IsDir() && !srcAttr.IsDir() {
		log.Err(
			"Libfuse::Rename : dst [%s] is an existing directory but src [%s] is not a directory",
			dstPath,
			srcPath,
		)
		return -fuse.EISDIR
	}

	// ENOTDIR
	if (dstErr == nil || os.IsExist(dstErr)) && !dstAttr.IsDir() && srcAttr.IsDir() {
		log.Err(
			"Libfuse::Rename : dst [%s] is an existing file but src [%s] is a directory",
			dstPath,
			srcPath,
		)
		return -fuse.ENOTDIR
	}

	if srcAttr.IsDir() {
		// ENOTEMPTY
		if dstErr == nil || os.IsExist(dstErr) {
			empty := fuseFS.NextComponent().IsDirEmpty(internal.IsDirEmptyOptions{Name: dstPath})
			if !empty {
				return -fuse.ENOTEMPTY
			}
		}

		err := fuseFS.NextComponent().
			RenameDir(internal.RenameDirOptions{Src: srcPath, Dst: dstPath})
		if err != nil {
			log.Err(
				"Libfuse::Rename : error renaming directory %s -> %s [%s]",
				srcPath,
				dstPath,
				err.Error(),
			)
			return -fuse.EIO
		}

		libfuseStatsCollector.PushEvents(
			renameDir,
			srcPath,
			map[string]interface{}{source: srcPath, dest: dstPath},
		)
		libfuseStatsCollector.UpdateStats(stats_manager.Increment, renameDir, (int64)(1))

	} else {
		err := fuseFS.NextComponent().RenameFile(internal.RenameFileOptions{Src: srcPath, Dst: dstPath})
		if err != nil {
			log.Err("Libfuse::Rename : error renaming file %s -> %s [%s]", srcPath, dstPath, err.Error())
			return -fuse.EIO
		}

		libfuseStatsCollector.PushEvents(renameFile, srcPath, map[string]interface{}{source: srcPath, dest: dstPath})
		libfuseStatsCollector.UpdateStats(stats_manager.Increment, renameFile, (int64)(1))

	}

	return 0
}

// Symlink creates a symbolic link
func (cf *CgofuseFS) Symlink(target string, newpath string) int {
	name := trimFusePathWin(newpath)
	name = common.NormalizeObjectName(name)
	targetPath := common.NormalizeObjectName(target)
	log.Trace("Libfuse::Symlink : Received for %s -> %s", name, targetPath)

	err := fuseFS.NextComponent().
		CreateLink(internal.CreateLinkOptions{Name: name, Target: targetPath})
	if err != nil {
		log.Err(
			"Libfuse::Symlink : error linking file %s -> %s [%s]",
			name,
			targetPath,
			err.Error(),
		)
		return -fuse.EIO
	}

	libfuseStatsCollector.PushEvents(createLink, name, map[string]interface{}{trgt: targetPath})
	libfuseStatsCollector.UpdateStats(stats_manager.Increment, createLink, (int64)(1))

	return 0
}

// Readlink reads the target of a symbolic link.
func (cf *CgofuseFS) Readlink(path string) (int, string) {
	name := trimFusePathWin(path)
	name = common.NormalizeObjectName(name)
	log.Trace("Libfuse::Readlink : Received for %s", name)

	linkSize := int64(0)
	attr, err := fuseFS.NextComponent().GetAttr(internal.GetAttrOptions{Name: name})
	if err == nil && attr != nil {
		linkSize = attr.Size
	}

	targetPath, err := fuseFS.NextComponent().
		ReadLink(internal.ReadLinkOptions{Name: name, Size: linkSize})
	if err != nil {
		log.Err("Libfuse::Readlink : error reading link file %s [%s]", name, err.Error())
		if os.IsNotExist(err) {
			return -fuse.ENOENT, targetPath
		}
		return -fuse.EIO, targetPath
	}

	libfuseStatsCollector.PushEvents(readLink, name, map[string]interface{}{trgt: targetPath})
	libfuseStatsCollector.UpdateStats(stats_manager.Increment, readLink, (int64)(1))

	return 0, targetPath
}

// Fsync synchronizes the file.
func (cf *CgofuseFS) Fsync(path string, datasync bool, fh uint64) int {
	if fh == 0 {
		return -fuse.EIO
	}

	// Get the filehandle
	handle, exists := handlemap.Load(handlemap.HandleID(fh))
	if !exists {
		log.Trace("Libfuse::Fsync : error getting handle for path %s, handle: %d", path, fh)
		return -fuse.EBADF
	}
	log.Trace("Libfuse::Fsync : %s, handle: %d", handle.Path, handle.ID)

	options := internal.SyncFileOptions{Handle: handle}
	// If the datasync parameter is non-zero, then only the user data should be flushed, not the metadata.
	// TODO : Should we support this?

	err := fuseFS.NextComponent().SyncFile(options)
	if err != nil {
		log.Err("Libfuse::Fsync : error syncing file %s [%s]", handle.Path, err.Error())
		return -fuse.EIO
	}

	libfuseStatsCollector.PushEvents(syncFile, handle.Path, nil)
	libfuseStatsCollector.UpdateStats(stats_manager.Increment, syncFile, (int64)(1))

	return 0
}

// Fsyncdir synchronizes a directory.
func (cf *CgofuseFS) Fsyncdir(path string, datasync bool, fh uint64) int {
	name := trimFusePathWin(path)
	name = common.NormalizeObjectName(name)
	log.Trace("Libfuse::Fsyncdir : %s", name)

	options := internal.SyncDirOptions{Name: name}
	// If the datasync parameter is non-zero, then only the user data should be flushed, not the metadata.
	// TODO : Should we support this?

	err := fuseFS.NextComponent().SyncDir(options)
	if err != nil {
		log.Err("Libfuse::Fsyncdir : error syncing dir %s [%s]", name, err.Error())
		return -fuse.EIO
	}

	libfuseStatsCollector.PushEvents(syncDir, name, nil)
	libfuseStatsCollector.UpdateStats(stats_manager.Increment, syncDir, (int64)(1))

	return 0
}

// Chmod changes permissions of a file.
func (cf *CgofuseFS) Chmod(path string, mode uint32) int {
	name := trimFusePathWin(path)
	name = common.NormalizeObjectName(name)
	log.Trace("Libfuse::Chmod : %s", name)

	err := fuseFS.NextComponent().Chmod(
		internal.ChmodOptions{
			Name: name,
			Mode: fs.FileMode(mode),
		})
	if err != nil {
		log.Err("Libfuse::Chmod : error in chmod of %s [%s]", name, err.Error())
		if os.IsNotExist(err) {
			return -fuse.ENOENT
		} else if os.IsPermission(err) {
			return -fuse.EACCES
		}
		return -fuse.EIO
	}

	libfuseStatsCollector.PushEvents(chmod, name, map[string]interface{}{md: fs.FileMode(mode)})
	libfuseStatsCollector.UpdateStats(stats_manager.Increment, chmod, (int64)(1))

	return 0
}

// Chown changes the owner of a file.
func (cf *CgofuseFS) Chown(path string, uid uint32, gid uint32) int {
	name := trimFusePathWin(path)
	name = common.NormalizeObjectName(name)
	log.Trace("Libfuse::Chown : %s", name)
	// TODO: Implement
	return 0
}

// Utimens changes the access and modification time of a file.
func (cf *CgofuseFS) Utimens(path string, tmsp []fuse.Timespec) int {
	name := trimFusePathWin(path)
	name = common.NormalizeObjectName(name)
	log.Trace("Libfuse::Utimens : %s", name)
	// TODO: is the conversion from [2]timespec to *timespec ok?
	// TODO: Implement
	// For now this returns 0 to allow touch to work correctly
	return 0
}

// Access is not implemented.
func (cf *CgofuseFS) Access(path string, mask uint32) int {
	return -fuse.ENOSYS
}

// Getxattr  is not implemented.
func (cf *CgofuseFS) Getxattr(path string, name string) (int, []byte) {
	return -fuse.ENOSYS, nil
}

// Link is not implemented.
func (cf *CgofuseFS) Link(oldpath string, newpath string) int {
	return -fuse.ENOSYS
}

// Listxattr is not implemented.
func (cf *CgofuseFS) Listxattr(path string, fill func(name string) bool) int {
	return -fuse.ENOSYS
}

// Mknod is not implemented.
func (cf *CgofuseFS) Mknod(path string, mode uint32, dev uint64) int {
	return -fuse.ENOSYS
}

// Removexattr is not implemented.
func (cf *CgofuseFS) Removexattr(path string, name string) int {
	return -fuse.ENOSYS
}

// Setxattr  is not implemented.
func (cf *CgofuseFS) Setxattr(path string, name string, value []byte, flags int) int {
	return -fuse.ENOSYS
}

// Verify that we follow the interface
var (
	_ fuse.FileSystemInterface = (*CgofuseFS)(nil)
)
