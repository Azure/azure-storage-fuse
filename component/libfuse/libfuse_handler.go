//go:build !fuse2
// +build !fuse2

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

// CFLAGS: compile time flags -D object file creation. D= Define
// LFLAGS: loader flags link library -l binary file. l=link -ldl is for the extension to dynamically link

// #cgo CFLAGS: -DFUSE_USE_VERSION=39 -D_FILE_OFFSET_BITS=64
// #cgo LDFLAGS: -lfuse3 -ldl
// #include "libfuse_wrapper.h"
// #include "extension_handler.h"
import "C" //nolint

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"syscall"
	"time"
	"unsafe"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal"
	"github.com/Azure/azure-storage-fuse/v2/internal/handlemap"
	"github.com/Azure/azure-storage-fuse/v2/internal/stats_manager"
)

/* --- IMPORTANT NOTE ---
In below code lot of places we are doing this sort of conversions:
		- fi.fh = C.ulong(uintptr(unsafe.Pointer(handle)))
		- handle := (*handlemap.Handle)(unsafe.Pointer(uintptr(fi.fh)))

To open/create calls we need to return back a handle to libfuse which shall be an integer value
As in blobfuse we maintain handle as an object, instead of returning back a running integer value as handle
we are converting back the pointer to our handle object to an integer value and sending it to libfuse.
When read/write/flush/close call comes libfuse will supply this handle value back to blobfuse.
In those calls we will convert integer value back to a pointer and get our valid handle object back for that file.
*/

var logy *os.File

const (
	C_ENOENT = int(-C.ENOENT)
	C_EIO    = int(-C.EIO)
	C_EACCES = int(-C.EACCES)
)

// Note: libfuse prepends "/" to the path.
// trimFusePath trims the first character from the path provided by libfuse
func trimFusePath(path *C.char) string {
	if path == nil {
		return ""
	}
	str := C.GoString(path)
	if str != "" {
		return str[1:]
	}
	return str
}

var fuse_opts C.fuse_options_t // nolint

// convertConfig converts the config options from Go to C
func (lf *Libfuse) convertConfig() *C.fuse_options_t {
	fuse_opts := &C.fuse_options_t{}

	// Note: C strings are allocated in the heap using malloc. Call C.free when string is no longer needed.
	fuse_opts.mount_path = C.CString(lf.mountPath)
	fuse_opts.uid = C.uid_t(lf.ownerUID)
	fuse_opts.gid = C.gid_t(lf.ownerGID)
	fuse_opts.permissions = C.uint(lf.filePermission)
	fuse_opts.entry_expiry = C.int(lf.entryExpiration)
	fuse_opts.attr_expiry = C.int(lf.attributeExpiration)
	fuse_opts.negative_expiry = C.int(lf.negativeTimeout)
	fuse_opts.readonly = C.bool(lf.readOnly)
	fuse_opts.allow_other = C.bool(lf.allowOther)
	fuse_opts.allow_root = C.bool(lf.allowRoot)
	fuse_opts.trace_enable = C.bool(lf.traceEnable)
	fuse_opts.umask = C.int(lf.umask)

	return fuse_opts
}

// initFuse initializes the fuse library by registering callbacks, parsing arguments and mounting the directory
func (lf *Libfuse) initFuse() error {
	log.Trace("Libfuse::initFuse : Initializing FUSE3")

	operations := C.fuse_operations_t{}

	if lf.extensionPath != "" {
		log.Trace("Libfuse::InitFuse : Going for extension mouting [%s]", lf.extensionPath)

		// User has given an extension so we need to register it to fuse
		//  and then register ourself to it
		extensionPath := C.CString(lf.extensionPath)
		defer C.free(unsafe.Pointer(extensionPath))

		// Load the library
		errc := C.load_library(extensionPath)
		if errc != 0 {
			log.Err("Libfuse::InitFuse : Failed to load extension err code %d", errc)
			return errors.New("failed to load extension")
		}
		log.Trace("Libfuse::InitFuse : Extension loaded")

		// Get extension callback table
		errc = C.get_extension_callbacks(&operations)
		if errc != 0 {
			C.unload_library()
			log.Err("Libfuse::InitFuse : Failed to get callback table from extension. error code %d", errc)
			return errors.New("failed to get callback table from extension")
		}
		log.Trace("Libfuse::InitFuse : Extension callback retrieved")

		// Get our callback table
		my_operations := C.fuse_operations_t{}
		C.populate_callbacks(&my_operations)

		// Send our callback table to the extension
		errc = C.register_callback_to_extension(&my_operations)
		if errc != 0 {
			C.unload_library()
			log.Err("Libfuse::InitFuse : Failed to register callback table to extension. error code %d", errc)
			return errors.New("failed to register callback table to extension")
		}
		log.Trace("Libfuse::InitFuse : Callbacks registered to extension")
	} else {
		// Populate our methods to be registered to libfuse
		log.Trace("Libfuse::initFuse : Registering fuse callbacks")
		C.populate_callbacks(&operations)
	}

	log.Trace("Libfuse::initFuse : Populating fuse arguments")
	fuse_opts := lf.convertConfig()
	var args C.fuse_args_t

	fuse_opts, ret := populateFuseArgs(fuse_opts, &args)
	if ret != 0 {
		log.Err("Libfuse::initFuse : Failed to parse fuse arguments")
		return errors.New("failed to parse fuse arguments")
	}
	// Note: C strings are allocated in the heap using malloc. Calling C.free to release the mount path since it is no longer needed.
	C.free(unsafe.Pointer(fuse_opts.mount_path))

	log.Info("Libfuse::initFuse : Mounting with fuse3 library")
	ret = C.start_fuse(&args, &operations)
	if ret != 0 {
		log.Err("Libfuse::initFuse : failed to mount fuse")
		return errors.New("failed to mount fuse")
	}

	return nil
}

// populateFuseArgs populates libfuse args before we call start_fuse
func populateFuseArgs(opts *C.fuse_options_t, args *C.fuse_args_t) (*C.fuse_options_t, C.int) {
	log.Trace("Libfuse::populateFuseArgs")
	if args == nil {
		return nil, 1
	}
	args.argc = 0
	args.allocated = 1

	arguments := make([]string, 0)
	options := fmt.Sprintf("entry_timeout=%d,attr_timeout=%d,negative_timeout=%d",
		opts.entry_expiry,
		opts.attr_expiry,
		opts.negative_expiry)

	if opts.allow_other {
		options += ",allow_other"
	}

	if opts.allow_root {
		options += ",allow_root"
	}

	if opts.readonly {
		options += ",ro"
	}

	if opts.umask != 0 {
		options += fmt.Sprintf(",umask=%04d", opts.umask)
	}

	options += ",max_read=1048576"

	if !fuseFS.directIO {
		options += ",kernel_cache"
	}

	// Why we pass -f
	// CGo is not very good with handling forks - so if the user wants to run blobfuse in the
	// background we fork on mount in GO (mount.go) and we just always force libfuse to mount in foreground
	arguments = append(arguments, "blobfuse2",
		C.GoString(opts.mount_path),
		"-o", options,
		"-f", "-ofsname=blobfuse2") // "-omax_read=4194304"

	if opts.trace_enable {
		arguments = append(arguments, "-d")
	}

	for _, a := range arguments {
		log.Debug("Libfuse::populateFuseArgs : opts : %s", a)
		arg := C.CString(a)
		defer C.free(unsafe.Pointer(arg))
		err := C.fuse_opt_add_arg(args, arg)
		if err != 0 {
			return nil, err
		}
	}

	return opts, 0
}

// destroyFuse is a no-op
func (lf *Libfuse) destroyFuse() error {
	log.Trace("Libfuse::destroyFuse : Destroying FUSE")
	return nil
}

//export libfuse_init
func libfuse_init(conn *C.fuse_conn_info_t, cfg *C.fuse_config_t) (res unsafe.Pointer) {
	log.Trace("Libfuse::libfuse_init : init (read : %v, write %v, read-ahead %v)", conn.max_read, conn.max_write, conn.max_readahead)

	log.Info("Libfuse::NotifyMountToParent : Notifying parent for successful mount")
	if err := common.NotifyMountToParent(); err != nil {
		log.Err("Libfuse::NotifyMountToParent : Failed to notify parent, error: [%v]", err)
	}

	C.populate_uid_gid()

	log.Info("Libfuse::libfuse_init : Kernel Caps : %d", conn.capable)

	// Populate connection information
	// conn.want |= C.FUSE_CAP_NO_OPENDIR_SUPPORT

	// Allow fuse to perform parallel operations on a directory
	if (conn.capable & C.FUSE_CAP_PARALLEL_DIROPS) != 0 {
		log.Info("Libfuse::libfuse_init : Enable Capability : FUSE_CAP_PARALLEL_DIROPS")
		conn.want |= C.FUSE_CAP_PARALLEL_DIROPS
	}

	// Kernel shall invalidate the data in page cache if file size of LMT changes
	if (conn.capable & C.FUSE_CAP_AUTO_INVAL_DATA) != 0 {
		log.Info("Libfuse::libfuse_init : Enable Capability : FUSE_CAP_AUTO_INVAL_DATA")
		conn.want |= C.FUSE_CAP_AUTO_INVAL_DATA
	}

	// Enable read-dir plus where attributes of each file are returned back
	// in the list call itself and fuse does not need to fire getAttr after list
	if (conn.capable & C.FUSE_CAP_READDIRPLUS) != 0 {
		log.Info("Libfuse::libfuse_init : Enable Capability : FUSE_CAP_READDIRPLUS")
		conn.want |= C.FUSE_CAP_READDIRPLUS
	}

	// Make the kernel readahead synchronous, that would make the implementation inside the blobfuse easier.
	// Default behaviour of kernel is to do asynchronous readahead if its capable.
	if (conn.capable & C.FUSE_CAP_ASYNC_READ) != 0 {
		log.Info("Libfuse::libfuse_init : Disable Capability : FUSE_CAP_ASYNC_READ")
		conn.want &= ^C.uint(C.FUSE_CAP_ASYNC_READ)
	}

	if (conn.capable & C.FUSE_CAP_SPLICE_WRITE) != 0 {
		// While writing to fuse device let libfuse collate the data and write big chunks
		log.Info("Libfuse::libfuse_init : Enable Capability : FUSE_CAP_SPLICE_WRITE")
		conn.want |= C.FUSE_CAP_SPLICE_WRITE
	}

	/*
		FUSE_CAP_WRITEBACK_CACHE flag is not suitable for network filesystems.  If a partial page is
		written, then the page needs to be first read from userspace.  This means, that
		even for files opened for O_WRONLY it is possible that READ requests will be
		generated by the kernel.
	*/
	if (!fuseFS.directIO) && (!fuseFS.disableWritebackCache) && ((conn.capable & C.FUSE_CAP_WRITEBACK_CACHE) != 0) {
		// Buffer write requests at libfuse and then hand it off to application
		log.Info("Libfuse::libfuse_init : Enable Capability : FUSE_CAP_WRITEBACK_CACHE")
		conn.want |= C.FUSE_CAP_WRITEBACK_CACHE
	}

	// Max background thread on the fuse layer for high parallelism
	conn.max_background = C.uint(fuseFS.maxFuseThreads)

	// While reading a file let kernel do readahed for better perf
	conn.max_readahead = (1 * 1024 * 1024)
	conn.max_read = (1 * 1024 * 1024)

	// RHEL still has 3.3 fuse version and it does not allow max_write beyond 128K
	// Setting this value to 1 MB will fail the mount.
	fuse_minor := common.GetFuseMinorVersion()
	if fuse_minor > 4 {
		log.Info("Libfuse::libfuse_init : Setting 1MB max_write for fuse minor %v", fuse_minor)
		conn.max_write = (1 * 1024 * 1024)
	} else {
		log.Info("Libfuse::libfuse_init : Ignoring max_write for fuse minor %v", fuse_minor)
		conn.max_write = (128 * 1024)
	}

	// direct_io option is used to bypass the kernel cache. It disables the use of
	// page cache (file content cache) in the kernel for the filesystem.
	if fuseFS.directIO {
		cfg.direct_io = C.int(1)
	}

	return nil
}

//export libfuse_destroy
func libfuse_destroy(data unsafe.Pointer) {
	log.Trace("Libfuse::libfuse_destroy : destroy")
}

func (lf *Libfuse) fillStat(attr *internal.ObjAttr, stbuf *C.stat_t) {
	(*stbuf).st_uid = C.uint(lf.ownerUID)
	(*stbuf).st_gid = C.uint(lf.ownerGID)
	(*stbuf).st_nlink = 1
	(*stbuf).st_size = C.long(attr.Size)

	// Populate mode
	// Backing storage implementation has support for mode.
	if !attr.IsModeDefault() {
		(*stbuf).st_mode = C.uint(attr.Mode) & 0xffffffff
	} else {
		if attr.IsDir() {
			(*stbuf).st_mode = C.uint(lf.dirPermission) & 0xffffffff
		} else {
			(*stbuf).st_mode = C.uint(lf.filePermission) & 0xffffffff
		}
	}

	if attr.IsDir() {
		(*stbuf).st_nlink = 2
		(*stbuf).st_size = 4096
		(*stbuf).st_mode |= C.S_IFDIR
	} else if attr.IsSymlink() {
		(*stbuf).st_mode |= C.S_IFLNK
	} else {
		(*stbuf).st_mode |= C.S_IFREG
	}

	(*stbuf).st_atim.tv_sec = C.long(attr.Atime.Unix())
	(*stbuf).st_atim.tv_nsec = 0

	(*stbuf).st_ctim.tv_sec = C.long(attr.Ctime.Unix())
	(*stbuf).st_ctim.tv_nsec = 0

	(*stbuf).st_mtim.tv_sec = C.long(attr.Mtime.Unix())
	(*stbuf).st_mtim.tv_nsec = 0
}

// File System Operations
// Similar to well known UNIX file system operations
// Instead of returning an error in 'errno', return the negated error value (-errno) directly.
// Kernel will perform permission checking if `default_permissions` mount option was passed to `fuse_main()`
// otherwise, perform necessary permission checking

// libfuse_getattr gets file attributes
//
//export libfuse_getattr
func libfuse_getattr(path *C.char, stbuf *C.stat_t, fi *C.fuse_file_info_t) C.int {
	name := trimFusePath(path)
	name = common.NormalizeObjectName(name)
	// log.Trace("Libfuse::libfuse_getattr : %s", name)

	// Return the default configuration for the root
	if name == "" {
		return C.get_root_properties(stbuf)
	}

	// TODO: How does this work if we trim the path?
	// Check if the file is meant to be ignored
	if ignore, found := ignoreFiles[name]; found && ignore {
		return -C.ENOENT
	}

	// Get attributes
	attr, err := fuseFS.NextComponent().GetAttr(internal.GetAttrOptions{Name: name})
	if err != nil {
		// log.Err("Libfuse::libfuse_getattr : Failed to get attributes of %s [%s]", name, err.Error())
		if err == syscall.ENOENT {
			return -C.ENOENT
		} else if err == syscall.EACCES {
			return -C.EACCES
		} else {
			return -C.EIO
		}
	}

	// Populate stat
	fuseFS.fillStat(attr, stbuf)
	return 0
}

// Directory Operations

// libfuse_mkdir creates a directory
//
//export libfuse_mkdir
func libfuse_mkdir(path *C.char, mode C.mode_t) C.int {
	name := trimFusePath(path)
	name = common.NormalizeObjectName(name)
	log.Trace("Libfuse::libfuse_mkdir : %s", name)

	err := fuseFS.NextComponent().CreateDir(internal.CreateDirOptions{Name: name, Mode: fs.FileMode(uint32(mode) & 0xffffffff)})
	if err != nil {
		log.Err("Libfuse::libfuse_mkdir : Failed to create %s [%s]", name, err.Error())
		if os.IsPermission(err) {
			return -C.EACCES
		} else if os.IsExist(err) {
			return -C.EEXIST
		} else {
			return -C.EIO
		}
	}

	libfuseStatsCollector.PushEvents(createDir, name, map[string]interface{}{md: fs.FileMode(uint32(mode) & 0xffffffff)})
	libfuseStatsCollector.UpdateStats(stats_manager.Increment, createDir, (int64)(1))

	return 0
}

// libfuse_opendir opens handle to given directory
//
//export libfuse_opendir
func libfuse_opendir(path *C.char, fi *C.fuse_file_info_t) C.int {
	name := trimFusePath(path)
	name = common.NormalizeObjectName(name)
	if name != "" {
		name = name + "/"
	}

	log.Trace("Libfuse::libfuse_opendir : %s", name)

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

	handlemap.Add(handle)
	fi.fh = C.ulong(uintptr(unsafe.Pointer(handle)))

	return 0
}

// libfuse_releasedir opens handle to given directory
//
//export libfuse_releasedir
func libfuse_releasedir(path *C.char, fi *C.fuse_file_info_t) C.int {
	handle := (*handlemap.Handle)(unsafe.Pointer(uintptr(fi.fh)))

	log.Trace("Libfuse::libfuse_releasedir : %s, handle: %d", handle.Path, handle.ID)

	handle.Cleanup()
	handlemap.Delete(handle.ID)
	return 0
}

// libfuse_readdir reads a directory
//
//export libfuse_readdir
func libfuse_readdir(_ *C.char, buf unsafe.Pointer, filler C.fuse_fill_dir_t, off C.off_t, fi *C.fuse_file_info_t, flag C.fuse_readdir_flags_t) C.int {
	handle := (*handlemap.Handle)(unsafe.Pointer(uintptr(fi.fh)))

	handle.RLock()
	val, found := handle.GetValue("cache")
	handle.RUnlock()

	if !found {
		return C.int(C_EIO)
	}

	off_64 := uint64(off)
	cacheInfo := val.(*dirChildCache)
	if off_64 == 0 ||
		(off_64 >= cacheInfo.eIndex && cacheInfo.token != "") {
		attrs, token, err := fuseFS.NextComponent().StreamDir(internal.StreamDirOptions{
			Name:   handle.Path,
			Offset: off_64,
			Token:  cacheInfo.token,
			Count:  common.MaxDirListCount,
		})

		if err != nil {
			log.Err("Libfuse::libfuse_readdir : Path %s, handle: %d, offset %d. Error in retrieval %s", handle.Path, handle.ID, off_64, err.Error())
			if os.IsNotExist(err) {
				return C.int(C_ENOENT)
			} else if os.IsPermission(err) {
				return C.int(C_EACCES)
			} else {
				return C.int(C_EIO)
			}
		}

		// TODO: Investigate why this works in fuse2 but not fuse3
		// if off_64 == 0 {
		// 	attrs = append([]*internal.ObjAttr{{Flags: fuseFS.lsFlags, Name: "."}, {Flags: fuseFS.lsFlags, Name: ".."}}, attrs...)
		// }

		cacheInfo.sIndex = off_64
		cacheInfo.eIndex = off_64 + uint64(len(attrs))
		cacheInfo.length = uint64(len(attrs))
		cacheInfo.token = token
		cacheInfo.children = cacheInfo.children[:0]
		cacheInfo.children = attrs
	}

	if off_64 >= cacheInfo.eIndex {
		// If offset is still beyond the end index limit then we are done iterating
		return 0
	}

	stbuf := C.stat_t{}
	idx := C.long(off)

	// Populate the stat by calling filler
	for segmentIdx := off_64 - cacheInfo.sIndex; segmentIdx < cacheInfo.length; segmentIdx++ {
		fuseFS.fillStat(cacheInfo.children[segmentIdx], &stbuf)

		name := C.CString(cacheInfo.children[segmentIdx].Name)
		if 0 != C.fill_dir_entry(filler, buf, name, &stbuf, idx+1) {
			C.free(unsafe.Pointer(name))
			break
		}

		C.free(unsafe.Pointer(name))
		idx++
	}

	return 0
}

// libfuse_rmdir deletes a directory, which must be empty.
//
//export libfuse_rmdir
func libfuse_rmdir(path *C.char) C.int {
	name := trimFusePath(path)
	name = common.NormalizeObjectName(name)
	log.Trace("Libfuse::libfuse_rmdir : %s", name)

	empty := fuseFS.NextComponent().IsDirEmpty(internal.IsDirEmptyOptions{Name: name})
	if !empty {
		// delete empty directories from local cache directory
		val, err := fuseFS.NextComponent().DeleteEmptyDirs(internal.DeleteDirOptions{Name: name})
		if !val {
			// either file cache has failed or not present in the pipeline
			if err != nil {
				// if error is not nil, file cache has failed
				log.Err("Libfuse::libfuse_rmdir : Failed to delete %s [%s]", name, err.Error())
			}
			return -C.ENOTEMPTY
		}
	}

	err := fuseFS.NextComponent().DeleteDir(internal.DeleteDirOptions{Name: name})
	if err != nil {
		log.Err("Libfuse::libfuse_rmdir : Failed to delete %s [%s]", name, err.Error())
		if os.IsNotExist(err) {
			return -C.ENOENT
		} else {
			return -C.EIO
		}
	}

	libfuseStatsCollector.PushEvents(deleteDir, name, nil)
	libfuseStatsCollector.UpdateStats(stats_manager.Increment, deleteDir, (int64)(1))

	return 0
}

// File Operations
//
//export libfuse_statfs
func libfuse_statfs(path *C.char, buf *C.statvfs_t) C.int {
	name := trimFusePath(path)
	name = common.NormalizeObjectName(name)
	log.Trace("Libfuse::libfuse_statfs : %s", name)

	attr, populated, err := fuseFS.NextComponent().StatFs()
	if err != nil {
		log.Err("Libfuse::libfuse_statfs : Failed to get stats %s [%s]", name, err.Error())
		return -C.EIO
	}

	// if populated then we need to overwrite root attributes
	if populated {
		(*buf).f_bsize = C.ulong(attr.Bsize)
		(*buf).f_frsize = C.ulong(attr.Frsize)
		(*buf).f_blocks = C.ulong(attr.Blocks)
		(*buf).f_bavail = C.ulong(attr.Bavail)
		(*buf).f_bfree = C.ulong(attr.Bfree)
		(*buf).f_files = C.ulong(attr.Files)
		(*buf).f_ffree = C.ulong(attr.Ffree)
		(*buf).f_flag = C.ulong(attr.Flags)
		return 0
	}

	C.populate_statfs(path, buf)

	return 0
}

// libfuse_create creates a file with the specified mode and then opens it.
//
//export libfuse_create
func libfuse_create(path *C.char, mode C.mode_t, fi *C.fuse_file_info_t) C.int {
	name := trimFusePath(path)
	name = common.NormalizeObjectName(name)
	log.Trace("Libfuse::libfuse_create : %s", name)

	handle, err := fuseFS.NextComponent().CreateFile(internal.CreateFileOptions{Name: name, Mode: fs.FileMode(uint32(mode) & 0xffffffff)})
	if err != nil {
		log.Err("Libfuse::libfuse_create : Failed to create %s [%s]", name, err.Error())
		if os.IsExist(err) {
			return -C.EEXIST
		} else if os.IsPermission(err) {
			return -C.EACCES
		} else {
			return -C.EIO
		}
	}

	handlemap.Add(handle)
	ret_val := C.allocate_native_file_object(0, C.ulong(uintptr(unsafe.Pointer(handle))), 0)
	if !handle.Cached() {
		ret_val.fd = 0
	}

	log.Trace("Libfuse::libfuse_create : %s, handle %d", name, handle.ID)
	fi.fh = C.ulong(uintptr(unsafe.Pointer(ret_val)))

	libfuseStatsCollector.PushEvents(createFile, name, map[string]interface{}{md: fs.FileMode(uint32(mode) & 0xffffffff)})

	// increment open file handles count
	libfuseStatsCollector.UpdateStats(stats_manager.Increment, openHandles, (int64)(1))

	return 0
}

// libfuse_open opens a file
//
//export libfuse_open
func libfuse_open(path *C.char, fi *C.fuse_file_info_t) C.int {
	name := trimFusePath(path)
	name = common.NormalizeObjectName(name)
	log.Trace("Libfuse::libfuse_open : %s", name)
	// TODO: Should this sit behind a user option? What if we change something to support these in the future?
	// Mask out SYNC and DIRECT flags since write operation will fail
	if fi.flags&C.O_SYNC != 0 || fi.flags&C.__O_DIRECT != 0 {
		log.Info("Libfuse::libfuse_open : Reset flags for open %s, fi.flags %X", name, fi.flags)
		// Blobfuse2 does not support the SYNC or DIRECT flag. If a user application passes this flag on to blobfuse2
		// and we open the file with this flag, subsequent write operations will fail with "Invalid argument" error.
		// Mask them out here in the open call so that write works.
		// Oracle RMAN is one such application that sends these flags during backup
		fi.flags = fi.flags &^ C.O_SYNC
		fi.flags = fi.flags &^ C.__O_DIRECT
	}
	if !fuseFS.disableWritebackCache {
		if fi.flags&C.O_ACCMODE == C.O_WRONLY || fi.flags&C.O_APPEND != 0 {
			if fuseFS.ignoreOpenFlags {
				log.Warn("Libfuse::libfuse_open : Flags (%X) not supported to open %s when write back cache is on. Ignoring unsupported flags.", fi.flags, name)
				// O_ACCMODE disables both RDONLY, WRONLY and RDWR flags
				fi.flags = fi.flags &^ (C.O_APPEND | C.O_ACCMODE)
				fi.flags = fi.flags | C.O_RDWR
			} else {
				log.Err("Libfuse::libfuse_open : Flag (%X) not supported to open %s when write back cache is on. Pass --disable-writeback-cache=true or --ignore-open-flags=true via CLI", fi.flags, name)
				return -C.EINVAL
			}
		}
	}

	handle, err := fuseFS.NextComponent().OpenFile(
		internal.OpenFileOptions{
			Name:  name,
			Flags: int(int(fi.flags) & 0xffffffff),
			Mode:  fs.FileMode(fuseFS.filePermission),
		})

	if err != nil {
		log.Err("Libfuse::libfuse_open : Failed to open %s [%s]", name, err.Error())
		if os.IsNotExist(err) {
			return -C.ENOENT
		} else if os.IsPermission(err) {
			return -C.EACCES
		} else {
			return -C.EIO
		}
	}

	handlemap.Add(handle)
	//fi.fh = C.ulong(uintptr(unsafe.Pointer(handle)))
	ret_val := C.allocate_native_file_object(C.ulong(handle.UnixFD), C.ulong(uintptr(unsafe.Pointer(handle))), C.ulong(handle.Size))
	if !handle.Cached() {
		ret_val.fd = 0
	}
	log.Trace("Libfuse::libfuse_open : %s, handle %d", name, handle.ID)
	fi.fh = C.ulong(uintptr(unsafe.Pointer(ret_val)))

	// increment open file handles count
	libfuseStatsCollector.UpdateStats(stats_manager.Increment, openHandles, (int64)(1))

	return 0
}

var readTime time.Duration

func timer(name string) func() {
	start := time.Now()
	return func() {
		t := time.Since(start)
		readTime += t
		if t > time.Millisecond {
			// logy.WriteString(fmt.Sprintf("%s took %v\n", name, t))
		}
		// logy.WriteString(fmt.Sprintf("Total time of read %v\n", readTime))
	}
}

// libfuse_read reads data from an open file
//
//export libfuse_read
func libfuse_read(path *C.char, buf *C.char, size C.size_t, off C.off_t, fi *C.fuse_file_info_t) C.int {
	defer timer(fmt.Sprintf("Read for path %s offet %d size %d", C.GoString(path), off, size))()
	fileHandle := (*C.file_handle_t)(unsafe.Pointer(uintptr(fi.fh)))
	handle := (*handlemap.Handle)(unsafe.Pointer(uintptr(fileHandle.obj)))

	offset := uint64(off)
	data := (*[1 << 30]byte)(unsafe.Pointer(buf))

	var err error
	var bytesRead int

	if handle.Cached() {
		bytesRead, err = syscall.Pread(handle.FD(), data[:size], int64(offset))
		//bytesRead, err = handle.FObj.ReadAt(data[:size], int64(offset))
	} else {
		bytesRead, err = fuseFS.NextComponent().ReadInBuffer(
			internal.ReadInBufferOptions{
				Handle: handle,
				Offset: int64(offset),
				Data:   data[:size],
			})
	}

	if err == io.EOF {
		err = nil
	}
	if err != nil {
		log.Err("Libfuse::libfuse_read : error reading file %s, handle: %d [%s]", handle.Path, handle.ID, err.Error())
		return -C.EIO
	}

	return C.int(bytesRead)
}

// libfuse_write writes data to an open file
//
//export libfuse_write
func libfuse_write(path *C.char, buf *C.char, size C.size_t, off C.off_t, fi *C.fuse_file_info_t) C.int {
	fileHandle := (*C.file_handle_t)(unsafe.Pointer(uintptr(fi.fh)))
	handle := (*handlemap.Handle)(unsafe.Pointer(uintptr(fileHandle.obj)))

	offset := uint64(off)
	data := (*[1 << 30]byte)(unsafe.Pointer(buf))
	// log.Debug("Libfuse::libfuse_write : Offset %v, Data %v", offset, size)
	bytesWritten, err := fuseFS.NextComponent().WriteFile(
		internal.WriteFileOptions{
			Handle:   handle,
			Offset:   int64(offset),
			Data:     data[:size],
			Metadata: nil,
		})

	if err != nil {
		log.Err("Libfuse::libfuse_write : error writing file %s, handle: %d [%s]", handle.Path, handle.ID, err.Error())
		return -C.EIO
	}

	return C.int(bytesWritten)
}

// libfuse_flush possibly flushes cached data
//
//export libfuse_flush
func libfuse_flush(path *C.char, fi *C.fuse_file_info_t) C.int {
	defer timer(fmt.Sprintf("Flush for path %s ", C.GoString(path)))()
	fileHandle := (*C.file_handle_t)(unsafe.Pointer(uintptr(fi.fh)))
	handle := (*handlemap.Handle)(unsafe.Pointer(uintptr(fileHandle.obj)))
	log.Trace("Libfuse::libfuse_flush : %s, handle: %d", handle.Path, handle.ID)

	// If the file handle is not dirty, there is no need to flush
	if fileHandle.dirty != 0 {
		handle.Flags.Set(handlemap.HandleFlagDirty)
	}

	// if !handle.Dirty() {
	// 	return 0
	// }

	err := fuseFS.NextComponent().FlushFile(internal.FlushFileOptions{Handle: handle})
	if err != nil {
		log.Err("Libfuse::libfuse_flush : error flushing file %s, handle: %d [%s]", handle.Path, handle.ID, err.Error())
		if err == syscall.ENOENT {
			return -C.ENOENT
		} else if err == syscall.EACCES {
			return -C.EACCES
		} else {
			return -C.EIO
		}
	}

	return 0
}

// libfuse_truncate changes the size of a file
// There are two filesystem calls which can lead to this callback:
//  1. Truncate() -> SetAttr() called on file path.
//  2. ftruncate()-> SetAttr() called on file handle.
//
// man page:    https://man7.org/linux/man-pages/man2/truncate.2.html
// Libfuse Doc: https://github.com/libfuse/libfuse/blob/fc95fd5076fd845e496bfbcec1ad9da16534b1c9/include/fuse_lowlevel.h#L328
//
//export libfuse_truncate
func libfuse_truncate(path *C.char, off C.off_t, fi *C.fuse_file_info_t) C.int {
	name := trimFusePath(path)
	name = common.NormalizeObjectName(name)
	log.Trace("Libfuse::libfuse_truncate : %s size %d", name, off)

	var handle *handlemap.Handle
	if fi == nil {
		handle = nil
	} else {
		fileHandle := (*C.file_handle_t)(unsafe.Pointer(uintptr(fi.fh)))
		handle = (*handlemap.Handle)(unsafe.Pointer(uintptr(fileHandle.obj)))
	}

	err := fuseFS.NextComponent().TruncateFile(
		internal.TruncateFileOptions{
			Handle: handle,
			Name:   name,
			Size:   int64(off),
		})
	if err != nil {
		log.Err("Libfuse::libfuse_truncate : error truncating file %s [%s]", name, err.Error())
		if os.IsNotExist(err) {
			return -C.ENOENT
		}
		return -C.EIO
	}

	libfuseStatsCollector.PushEvents(truncateFile, name, map[string]interface{}{size: int64(off)})
	libfuseStatsCollector.UpdateStats(stats_manager.Increment, truncateFile, (int64)(1))

	return 0
}

// libfuse_release releases an open file
//
//export libfuse_release
func libfuse_release(path *C.char, fi *C.fuse_file_info_t) C.int {
	fileHandle := (*C.file_handle_t)(unsafe.Pointer(uintptr(fi.fh)))
	handle := (*handlemap.Handle)(unsafe.Pointer(uintptr(fileHandle.obj)))

	log.Trace("Libfuse::libfuse_release : %s, handle: %d", handle.Path, handle.ID)

	// If the file handle is dirty then file-cache needs to flush this file
	if fileHandle.dirty != 0 {
		handle.Flags.Set(handlemap.HandleFlagDirty)
	}

	err := fuseFS.NextComponent().CloseFile(internal.CloseFileOptions{Handle: handle})
	if err != nil {
		log.Err("Libfuse::libfuse_release : error closing file %s, handle: %d [%s]", handle.Path, handle.ID, err.Error())
		if err == syscall.ENOENT {
			return -C.ENOENT
		} else if err == syscall.EACCES {
			return -C.EACCES
		} else {
			return -C.EIO
		}
	}

	handlemap.Delete(handle.ID)
	C.release_native_file_object(fi)

	// decrement open file handles count
	libfuseStatsCollector.UpdateStats(stats_manager.Decrement, openHandles, (int64)(1))

	return 0
}

// libfuse_unlink removes a file
//
//export libfuse_unlink
func libfuse_unlink(path *C.char) C.int {
	name := trimFusePath(path)
	name = common.NormalizeObjectName(name)
	log.Trace("Libfuse::libfuse_unlink : %s", name)

	err := fuseFS.NextComponent().DeleteFile(internal.DeleteFileOptions{Name: name})
	if err != nil {
		log.Err("Libfuse::libfuse_unlink : error deleting file %s [%s]", name, err.Error())
		if os.IsNotExist(err) {
			return -C.ENOENT
		} else if os.IsPermission(err) {
			return -C.EACCES
		}
		return -C.EIO
	}

	libfuseStatsCollector.PushEvents(deleteFile, name, nil)
	libfuseStatsCollector.UpdateStats(stats_manager.Increment, deleteFile, (int64)(1))

	return 0
}

// libfuse_rename renames a file or directory
// https://man7.org/linux/man-pages/man2/rename.2.html
// errors handled: EISDIR, ENOENT, ENOTDIR, ENOTEMPTY, EEXIST
// TODO: handle EACCESS, EINVAL?
//
//export libfuse_rename
func libfuse_rename(src *C.char, dst *C.char, flags C.uint) C.int {
	srcPath := trimFusePath(src)
	srcPath = common.NormalizeObjectName(srcPath)
	dstPath := trimFusePath(dst)
	dstPath = common.NormalizeObjectName(dstPath)
	log.Trace("Libfuse::libfuse_rename : %s -> %s", srcPath, dstPath)
	// Note: When running other commands from the command line, a lot of them seemed to handle some cases like ENOENT themselves.
	// Rename did not, so we manually check here.

	// TODO: Support for RENAME_EXCHANGE
	if flags&C.RENAME_EXCHANGE != 0 {
		return -C.ENOTSUP
	}

	// ENOENT. Not covered: a directory component in dst does not exist
	if srcPath == "" || dstPath == "" {
		log.Err("Libfuse::libfuse_rename : src: [%s] or dst: [%s] is an empty string", srcPath, dstPath)
		return -C.ENOENT
	}

	srcAttr, srcErr := fuseFS.NextComponent().GetAttr(internal.GetAttrOptions{Name: srcPath})
	if os.IsNotExist(srcErr) {
		log.Err("Libfuse::libfuse_rename : Failed to get attributes of %s [%s]", srcPath, srcErr.Error())
		return -C.ENOENT
	}
	dstAttr, dstErr := fuseFS.NextComponent().GetAttr(internal.GetAttrOptions{Name: dstPath})

	// EEXIST
	if flags&C.RENAME_NOREPLACE != 0 && (dstErr == nil || os.IsExist(dstErr)) {
		return -C.EEXIST
	}

	// EISDIR
	if (dstErr == nil || os.IsExist(dstErr)) && dstAttr.IsDir() && !srcAttr.IsDir() {
		log.Err("Libfuse::libfuse_rename : dst [%s] is an existing directory but src [%s] is not a directory", dstPath, srcPath)
		return -C.EISDIR
	}

	// ENOTDIR
	if (dstErr == nil || os.IsExist(dstErr)) && !dstAttr.IsDir() && srcAttr.IsDir() {
		log.Err("Libfuse::libfuse_rename : dst [%s] is an existing file but src [%s] is a directory", dstPath, srcPath)
		return -C.ENOTDIR
	}

	if srcAttr.IsDir() {
		// ENOTEMPTY
		if dstErr == nil || os.IsExist(dstErr) {
			empty := fuseFS.NextComponent().IsDirEmpty(internal.IsDirEmptyOptions{Name: dstPath})
			if !empty {
				return -C.ENOTEMPTY
			}
		}

		err := fuseFS.NextComponent().RenameDir(internal.RenameDirOptions{
			Src: srcPath,
			Dst: dstPath,
		})
		if err != nil {
			log.Err("Libfuse::libfuse_rename : error renaming directory %s -> %s [%s]", srcPath, dstPath, err.Error())
			return -C.EIO
		}

		libfuseStatsCollector.PushEvents(renameDir, srcPath, map[string]interface{}{source: srcPath, dest: dstPath})
		libfuseStatsCollector.UpdateStats(stats_manager.Increment, renameDir, (int64)(1))

	} else {
		err := fuseFS.NextComponent().RenameFile(internal.RenameFileOptions{
			Src:     srcPath,
			Dst:     dstPath,
			SrcAttr: srcAttr,
			DstAttr: dstAttr,
		})
		if err != nil {
			log.Err("Libfuse::libfuse_rename : error renaming file %s -> %s [%s]", srcPath, dstPath, err.Error())
			return -C.EIO
		}

		libfuseStatsCollector.PushEvents(renameFile, srcPath, map[string]interface{}{source: srcPath, dest: dstPath})
		libfuseStatsCollector.UpdateStats(stats_manager.Increment, renameFile, (int64)(1))

	}

	return 0
}

// Symlink Operations

// libfuse_symlink creates a symbolic link
//
//export libfuse_symlink
func libfuse_symlink(target *C.char, link *C.char) C.int {
	name := trimFusePath(link)
	name = common.NormalizeObjectName(name)
	targetPath := C.GoString(target)
	targetPath = common.NormalizeObjectName(targetPath)
	log.Trace("Libfuse::libfuse_symlink : Received for %s -> %s", name, targetPath)

	err := fuseFS.NextComponent().CreateLink(internal.CreateLinkOptions{Name: name, Target: targetPath})
	if err != nil {
		log.Err("Libfuse::libfuse_symlink : error linking file %s -> %s [%s]", name, targetPath, err.Error())
		return -C.EIO
	}

	libfuseStatsCollector.PushEvents(createLink, name, map[string]interface{}{trgt: targetPath})
	libfuseStatsCollector.UpdateStats(stats_manager.Increment, createLink, (int64)(1))

	return 0
}

// libfuse_readlink reads the target of a symbolic link
//
//export libfuse_readlink
func libfuse_readlink(path *C.char, buf *C.char, size C.size_t) C.int {
	name := trimFusePath(path)
	name = common.NormalizeObjectName(name)
	//log.Trace("Libfuse::libfuse_readlink : Received for %s", name)

	linkSize := int64(0)
	attr, err := fuseFS.NextComponent().GetAttr(internal.GetAttrOptions{Name: name})
	if err == nil && attr != nil {
		linkSize = attr.Size
	}

	targetPath, err := fuseFS.NextComponent().ReadLink(internal.ReadLinkOptions{Name: name, Size: linkSize})
	if err != nil {
		log.Err("Libfuse::libfuse_readlink : error reading link file %s [%s]", name, err.Error())
		if os.IsNotExist(err) {
			return -C.ENOENT
		}
		return -C.EIO
	}
	data := (*[1 << 30]byte)(unsafe.Pointer(buf))
	copy(data[:size-1], targetPath)
	data[len(targetPath)] = 0

	libfuseStatsCollector.PushEvents(readLink, name, map[string]interface{}{trgt: targetPath})
	libfuseStatsCollector.UpdateStats(stats_manager.Increment, readLink, (int64)(1))

	return 0
}

// libfuse_fsync synchronizes file contents
//
//export libfuse_fsync
func libfuse_fsync(path *C.char, datasync C.int, fi *C.fuse_file_info_t) C.int {
	if fi.fh == 0 {
		return C.int(-C.EIO)
	}

	fileHandle := (*C.file_handle_t)(unsafe.Pointer(uintptr(fi.fh)))
	handle := (*handlemap.Handle)(unsafe.Pointer(uintptr(fileHandle.obj)))
	log.Trace("Libfuse::libfuse_fsync : %s, handle: %d", handle.Path, handle.ID)

	options := internal.SyncFileOptions{Handle: handle}
	// If the datasync parameter is non-zero, then only the user data should be flushed, not the metadata.
	// TODO : Should we support this?

	err := fuseFS.NextComponent().SyncFile(options)
	if err != nil {
		log.Err("Libfuse::libfuse_fsync : error syncing file %s [%s]", handle.Path, err.Error())
		return -C.EIO
	}

	libfuseStatsCollector.PushEvents(syncFile, handle.Path, nil)
	libfuseStatsCollector.UpdateStats(stats_manager.Increment, syncFile, (int64)(1))

	return 0
}

// libfuse_fsyncdir synchronizes directory contents
//
//export libfuse_fsyncdir
func libfuse_fsyncdir(path *C.char, datasync C.int, fi *C.fuse_file_info_t) C.int {
	name := trimFusePath(path)
	name = common.NormalizeObjectName(name)
	log.Trace("Libfuse::libfuse_fsyncdir : %s", name)

	options := internal.SyncDirOptions{Name: name}
	// If the datasync parameter is non-zero, then only the user data should be flushed, not the metadata.
	// TODO : Should we support this?

	err := fuseFS.NextComponent().SyncDir(options)
	if err != nil {
		log.Err("Libfuse::libfuse_fsyncdir : error syncing dir %s [%s]", name, err.Error())
		return -C.EIO
	}

	libfuseStatsCollector.PushEvents(syncDir, name, nil)
	libfuseStatsCollector.UpdateStats(stats_manager.Increment, syncDir, (int64)(1))

	return 0
}

// libfuse_chmod changes permission bits of a file
//
//export libfuse_chmod
func libfuse_chmod(path *C.char, mode C.mode_t, fi *C.fuse_file_info_t) C.int {
	name := trimFusePath(path)
	name = common.NormalizeObjectName(name)
	log.Trace("Libfuse::libfuse_chmod : %s", name)

	err := fuseFS.NextComponent().Chmod(
		internal.ChmodOptions{
			Name: name,
			Mode: fs.FileMode(uint32(mode) & 0xffffffff),
		})
	if err != nil {
		log.Err("Libfuse::libfuse_chmod : error in chmod of %s [%s]", name, err.Error())
		if os.IsNotExist(err) {
			return -C.ENOENT
		} else if os.IsPermission(err) {
			return -C.EACCES
		}
		return -C.EIO
	}

	libfuseStatsCollector.PushEvents(chmod, name, map[string]interface{}{md: fs.FileMode(uint32(mode) & 0xffffffff)})
	libfuseStatsCollector.UpdateStats(stats_manager.Increment, chmod, (int64)(1))

	return 0
}

// libfuse_chown changes the owner and group of a file
//
//export libfuse_chown
func libfuse_chown(path *C.char, uid C.uid_t, gid C.gid_t, fi *C.fuse_file_info_t) C.int {
	name := trimFusePath(path)
	name = common.NormalizeObjectName(name)
	log.Trace("Libfuse::libfuse_chown : %s", name)
	// TODO: Implement
	return 0
}

// libfuse_utimens changes the access and modification times of a file
//
//export libfuse_utimens
func libfuse_utimens(path *C.char, tv *C.timespec_t, fi *C.fuse_file_info_t) C.int {
	name := trimFusePath(path)
	name = common.NormalizeObjectName(name)
	log.Trace("Libfuse::libfuse_utimens : %s", name)
	// TODO: is the conversion from [2]timespec to *timespec ok?
	// TODO: Implement
	// For now this returns 0 to allow touch to work correctly
	return 0
}

// blobfuse_cache_update refresh the file-cache policy for this file
//
//export blobfuse_cache_update
func blobfuse_cache_update(path *C.char) C.int {
	name := trimFusePath(path)
	name = common.NormalizeObjectName(name)
	go fuseFS.NextComponent().FileUsed(name) //nolint
	return 0
}
