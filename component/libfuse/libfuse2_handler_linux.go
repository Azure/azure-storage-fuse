// +build fuse2

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

// CFLAGS: compile time flags -D object file creation. D= Define
// LFLAGS: loader flags link library -l binary file. l=link -ldl is for the extension to dynamically link

// #cgo CFLAGS: -DFUSE_USE_VERSION=29 -D_FILE_OFFSET_BITS=64 -D__FUSE2__
// #cgo LDFLAGS: -lfuse -ldl
// #include "libfuse_wrapper.h"
import "C"
import (
	"blobfuse2/common"
	"blobfuse2/common/log"
	"blobfuse2/internal"
	"blobfuse2/internal/handlemap"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"unsafe"
)

const (
	C_ENOENT = int(-C.ENOENT)
	C_EIO    = int(-C.EIO)
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

var fuse_opts C.fuse_options_t

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
	fuse_opts.trace_enable = C.bool(lf.traceEnable)
	return fuse_opts
}

// initFuse initializes the fuse library by registering callbacks, parsing arguments and mounting the directory
func (lf *Libfuse) initFuse() error {
	log.Trace("Libfuse::initFuse : Initializing FUSE")

	log.Trace("Libfuse::initFuse : Registering fuse callbacks")
	operations := C.fuse_operations_t{}
	C.populate_callbacks(&operations)

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

	log.Info("Libfuse::initFuse : Mounting with fuse2 library")
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

	if opts.readonly {
		options += ",ro"
	}
	// Why we pass -f
	// CGo is not very good with handling forks - so if the user wants to run blobfuse in the
	// background we fork on mount in GO (mount.go) and we just always force libfuse to mount in foreground
	arguments = append(arguments, "blobfuse2",
		C.GoString(opts.mount_path),
		"-o", options,
		"-f", "-ofsname=blobfuse2", "-okernel_cache")
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

//export libfuse2_init
func libfuse2_init(conn *C.fuse_conn_info_t) (res unsafe.Pointer) {
	log.Trace("Libfuse::libfuse2_init : init")
	C.populate_uid_gid()
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
	(*stbuf).st_atim.tv_nsec = C.long(attr.Atime.UnixNano())

	(*stbuf).st_ctim.tv_sec = C.long(attr.Ctime.Unix())
	(*stbuf).st_ctim.tv_nsec = C.long(attr.Ctime.UnixNano())

	(*stbuf).st_mtim.tv_sec = C.long(attr.Mtime.Unix())
	(*stbuf).st_mtim.tv_nsec = C.long(attr.Mtime.UnixNano())
}

// File System Operations
// Similar to well known UNIX file system operations
// Instead of returning an error in 'errno', return the negated error value (-errno) directly.
// Kernel will perform permission checking if `default_permissions` mount option was passed to `fuse_main()`
// otherwise, perform necessary permission checking

// libfuse2_getattr gets file attributes
//export libfuse2_getattr
func libfuse2_getattr(path *C.char, stbuf *C.stat_t) C.int {
	name := trimFusePath(path)
	name = common.NormalizeObjectName(name)
	//log.Trace("Libfuse::libfuse2_getattr : %s", name)

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
		log.Err("Libfuse::libfuse2_getattr : Failed to get attributes of %s (%s)", name, err.Error())
		return -C.ENOENT
	}

	// Populate stat
	fuseFS.fillStat(attr, stbuf)
	return 0
}

// Directory Operations

// libfuse_mkdir creates a directory
//export libfuse_mkdir
func libfuse_mkdir(path *C.char, mode C.mode_t) C.int {
	name := trimFusePath(path)
	name = common.NormalizeObjectName(name)
	log.Trace("Libfuse::libfuse_mkdir : %s", name)

	err := fuseFS.NextComponent().CreateDir(internal.CreateDirOptions{Name: name, Mode: fs.FileMode(uint32(mode) & 0xffffffff)})
	if err != nil {
		log.Err("Libfuse::libfuse_mkdir : Failed to create %s (%s)", name, err.Error())
		return -C.EIO
	}
	return 0
}

// libfuse_opendir opens handle to given directory
//export libfuse_opendir
func libfuse_opendir(path *C.char, fi *C.fuse_file_info_t) C.int {
	name := trimFusePath(path)
	name = common.NormalizeObjectName(name)
	if name != "" {
		name = name + "/"
	}

	log.Trace("Libfuse::libfuse_opendir : %s", name)

	handle := handlemap.NewHandle(name)
	id := handlemap.Add(handle)

	// For each handle created using opendir we create
	// this structure here to hold currnet block of children to serve readdir
	handle.SetValue("cache", &dirChildCache{
		sIndex:   0,
		eIndex:   0,
		token:    "",
		length:   0,
		children: make([]*internal.ObjAttr, 0),
	})

	fi.fh = C.ulong(id)
	return 0
}

// libfuse_releasedir opens handle to given directory
//export libfuse_releasedir
func libfuse_releasedir(path *C.char, fi *C.fuse_file_info_t) C.int {
	name := trimFusePath(path)
	name = common.NormalizeObjectName(name)

	id := uint64(fi.fh)
	log.Trace("Libfuse::libfuse_releasedir : %s, handle: %d", name, id)

	handle, ok := handlemap.Load(handlemap.HandleID(id))
	if !ok {
		log.Err("Libfuse::libfuse_releasedir : file handle error [failed to find id=%d]", id)
		return 0
	}

	handle.Cleanup()
	handlemap.Delete(handle.ID)
	return 0
}

// libfuse2_readdir reads a directory
//export libfuse2_readdir
func libfuse2_readdir(_ *C.char, buf unsafe.Pointer, filler C.fuse_fill_dir_t, off C.off_t, fi *C.fuse_file_info_t) C.int {
	id := uint64(fi.fh)
	off_64 := uint64(off)

	handle, ok := handlemap.Load(handlemap.HandleID(id))
	if !ok {
		log.Err("Libfuse::libfuse2_readdir : file handle error [failed to find id=%d]", id)
	}
	log.Trace("Libfuse::libfuse2_readdir : Path %s, handle: %d, offset %d", handle.Path, id, off_64)

	val, found := handle.GetValue("cache")
	if !found {
		return C.int(C_EIO)
	}

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
			log.Err("Libfuse::libfuse2_readdir : Path %s, handle: %d, offset %d. Error in retrieval", handle.Path, id, off_64)
			if os.IsNotExist(err) {
				return C.int(C_ENOENT)
			} else {
				return C.int(C_EIO)
			}
		}

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
//export libfuse_rmdir
func libfuse_rmdir(path *C.char) C.int {
	name := trimFusePath(path)
	name = common.NormalizeObjectName(name)
	log.Trace("Libfuse::libfuse_rmdir : %s", name)

	empty := fuseFS.NextComponent().IsDirEmpty(internal.IsDirEmptyOptions{Name: name})
	if !empty {
		return -C.ENOTEMPTY
	}

	err := fuseFS.NextComponent().DeleteDir(internal.DeleteDirOptions{Name: name})
	if err != nil {
		log.Err("Libfuse::libfuse_rmdir : Failed to delete %s (%s)", name, err.Error())
		if os.IsNotExist(err) {
			return -C.ENOENT
		} else {
			return -C.EIO
		}
	}

	return 0
}

// File Operations

// libfuse_create creates a file with the specified mode and then opens it.
//export libfuse_create
func libfuse_create(path *C.char, mode C.mode_t, fi *C.fuse_file_info_t) C.int {
	name := trimFusePath(path)
	name = common.NormalizeObjectName(name)
	log.Trace("Libfuse::libfuse_create : %s", name)

	handle, err := fuseFS.NextComponent().CreateFile(internal.CreateFileOptions{Name: name, Mode: fs.FileMode(uint32(mode) & 0xffffffff)})
	if err != nil {
		log.Err("Libfuse::libfuse_create : Failed to create %s (%s)", name, err.Error())
		if os.IsExist(err) {
			return -C.EEXIST
		} else {
			return -C.EIO
		}
	}

	id := handlemap.Add(handle)
	fi.fh = C.ulong(id)

	// TODO: Do we need to open the file here?
	return 0
}

// libfuse_open opens a file
//export libfuse_open
func libfuse_open(path *C.char, fi *C.fuse_file_info_t) C.int {
	name := trimFusePath(path)
	name = common.NormalizeObjectName(name)
	log.Trace("Libfuse::libfuse_open : %s", name)
	// TODO: Should this sit behind a user option? What if we change something to support these in the future?
	// Mask out SYNC flags since write operation will fail
	if fi.flags&C.O_SYNC != 0 || fi.flags&C.__O_DIRECT != 0 {
		log.Err("Libfuse::libfuse_open : Reset flags for open %s, fi.flags %X", name, fi.flags)
		// Blobfuse2 does not support the SYNC flag. If a user application passes this flag on to blobfuse2
		// and we open the file with this flag, subsequent write operations wlil fail with "Invalid argument" error.
		// Mask them out here in the open call so that write works.
		// Oracle RMAN is one such application that sends these flags during backup
		fi.flags = fi.flags &^ C.O_SYNC
		fi.flags = fi.flags &^ C.__O_DIRECT
	}

	handle, err := fuseFS.NextComponent().OpenFile(
		internal.OpenFileOptions{
			Name:  name,
			Flags: int(int(fi.flags) & 0xffffffff),
			Mode:  fs.FileMode(fuseFS.filePermission),
		})

	if err != nil {
		log.Err("Libfuse::libfuse_open : Failed to open %s (%s)", name, err.Error())
		if os.IsNotExist(err) {
			return -C.ENOENT
		} else {
			return -C.EIO
		}
	}

	id := handlemap.Add(handle)
	fi.fh = C.ulong(id)

	return 0
}

// libfuse_read reads data from an open file
//export libfuse_read
func libfuse_read(path *C.char, buf *C.char, size C.size_t, off C.off_t, fi *C.fuse_file_info_t) C.int {
	id := uint64(fi.fh)
	offset := uint64(off)

	handle, ok := handlemap.Load(handlemap.HandleID(id))
	if !ok {
		log.Err("Libfuse::libfuse_read : file handle error [failed to find id=%d]", id)
		return -C.EIO
	}

	data := (*[1 << 30]byte)(unsafe.Pointer(buf))
	bytesRead, err := fuseFS.NextComponent().ReadInBuffer(
		internal.ReadInBufferOptions{
			Handle: handle,
			Offset: int64(offset),
			Data:   data[:size],
		})
	if err == io.EOF {
		err = nil
	}
	if err != nil {
		log.Err("Libfuse::libfuse_read : error reading file %s, handle: %d [%s]", handle.Path, id, err.Error())
		return -C.EIO
	}

	return C.int(bytesRead)
}

// libfuse_write writes data to an open file
//export libfuse_write
func libfuse_write(path *C.char, buf *C.char, size C.size_t, off C.off_t, fi *C.fuse_file_info_t) C.int {
	id := uint64(fi.fh)
	offset := uint64(off)

	handle, ok := handlemap.Load(handlemap.HandleID(id))
	if !ok {
		log.Err("Libfuse::libfuse_write : file handle error [failed to find id=%d]", id)
		return -C.EIO
	}

	data := (*[1 << 30]byte)(unsafe.Pointer(buf))
	bytesWritten, err := fuseFS.NextComponent().WriteFile(
		internal.WriteFileOptions{
			Handle: handle,
			Offset: int64(offset),
			Data:   data[:size],
		})

	if err != nil {
		log.Err("Libfuse::libfuse_write : error writing file %s, handle: %d [%s]", handle.Path, id, err.Error())
		return -C.EIO
	}

	return C.int(bytesWritten)
}

// libfuse_flush possibly flushes cached data
//export libfuse_flush
func libfuse_flush(path *C.char, fi *C.fuse_file_info_t) C.int {
	id := uint64(fi.fh)

	handle, ok := handlemap.Load(handlemap.HandleID(id))
	if !ok {
		log.Err("Libfuse::libfuse_flush : file handle error [failed to find id=%d]", id)
		return -C.EIO
	}

	log.Trace("Libfuse::libfuse_flush : %s, handle: %d", handle.Path, id)

	// If the file handle is not dirty, there is no need to flush
	if !handle.Dirty {
		return 0
	}

	err := fuseFS.NextComponent().FlushFile(internal.FlushFileOptions{Handle: handle})
	if err != nil {
		log.Err("Libfuse::libfuse_flush : error flushing file %s, handle: %d [%s]", handle.Path, id, err.Error())
		return -C.EIO
	}

	return 0
}

// libfuse2_truncate changes the size of a file
//export libfuse2_truncate
func libfuse2_truncate(path *C.char, off C.off_t) C.int {
	name := trimFusePath(path)
	name = common.NormalizeObjectName(name)

	log.Trace("Libfuse::libfuse2_truncate : %s size %d", name, off)

	err := fuseFS.NextComponent().TruncateFile(internal.TruncateFileOptions{Name: name, Size: int64(off)})
	if err != nil {
		log.Err("Libfuse::libfuse2_truncate : error truncating file %s [%s]", name, err.Error())
		if os.IsNotExist(err) {
			return -C.ENOENT
		}
		return -C.EIO
	}

	return 0
}

// libfuse_release releases an open file
//export libfuse_release
func libfuse_release(path *C.char, fi *C.fuse_file_info_t) C.int {
	id := uint64(fi.fh)
	handle, ok := handlemap.Load(handlemap.HandleID(id))
	if !ok {
		log.Err("Libfuse::libfuse_release : file handle error [failed to find id=%d]", id)
		return -C.EIO
	}

	log.Trace("Libfuse::libfuse_release : %s, handle: %d", handle.Path, id)

	err := fuseFS.NextComponent().CloseFile(internal.CloseFileOptions{Handle: handle})
	if err != nil {
		log.Err("Libfuse::libfuse_release : error closing file %s, handle: %d [%s]", handle.Path, id, err.Error())
		return -C.EIO
	}

	handlemap.Delete(handle.ID)
	return 0
}

// libfuse_unlink removes a file
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
		}
		return -C.EIO
	}

	return 0
}

// libfuse2_rename renames a file or directory
// https://man7.org/linux/man-pages/man2/rename.2.html
// errors handled: EISDIR, ENOENT, ENOTDIR, ENOTEMPTY, EEXIST
// TODO: handle EACCESS, EINVAL?
//export libfuse2_rename
func libfuse2_rename(src *C.char, dst *C.char) C.int {
	srcPath := trimFusePath(src)
	srcPath = common.NormalizeObjectName(srcPath)
	dstPath := trimFusePath(dst)
	dstPath = common.NormalizeObjectName(dstPath)
	log.Trace("Libfuse::libfuse2_rename : %s -> %s", srcPath, dstPath)
	// Note: When running other commands from the command line, a lot of them seemed to handle some cases like ENOENT themselves.
	// Rename did not, so we manually check here.

	// ENOENT. Not covered: a directory component in dst does not exist
	if srcPath == "" || dstPath == "" {
		log.Err("Libfuse::libfuse2_rename : src: (%s) or dst: (%s) is an empty string", srcPath, dstPath)
		return -C.ENOENT
	}

	srcAttr, srcErr := fuseFS.NextComponent().GetAttr(internal.GetAttrOptions{Name: srcPath})
	if os.IsNotExist(srcErr) {
		log.Err("Libfuse::libfuse2_rename : Failed to get attributes of %s (%s)", srcPath, srcErr.Error())
		return -C.ENOENT
	}
	dstAttr, dstErr := fuseFS.NextComponent().GetAttr(internal.GetAttrOptions{Name: dstPath})

	// EISDIR
	if (dstErr == nil || os.IsExist(dstErr)) && dstAttr.IsDir() && !srcAttr.IsDir() {
		log.Err("Libfuse::libfuse2_rename : dst (%s) is an existing directory but src (%s) is not a directory", dstPath, srcPath)
		return -C.EISDIR
	}

	// ENOTDIR
	if (dstErr == nil || os.IsExist(dstErr)) && !dstAttr.IsDir() && srcAttr.IsDir() {
		log.Err("Libfuse::libfuse2_rename : dst (%s) is an existing file but src (%s) is a directory", dstPath, srcPath)
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

		err := fuseFS.NextComponent().RenameDir(internal.RenameDirOptions{Src: srcPath, Dst: dstPath})
		if err != nil {
			log.Err("Libfuse::libfuse2_rename : error renaming directory %s -> %s [%s]", srcPath, dstPath, err.Error())
			return -C.EIO
		}
	} else {
		err := fuseFS.NextComponent().RenameFile(internal.RenameFileOptions{Src: srcPath, Dst: dstPath})
		if err != nil {
			log.Err("Libfuse::libfuse2_rename : error renaming file %s -> %s [%s]", srcPath, dstPath, err.Error())
			return -C.EIO
		}
	}

	return 0
}

// Symlink Operations

// libfuse_symlink creates a symbolic link
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

	return 0
}

// libfuse_readlink reads the target of a symbolic link
//export libfuse_readlink
func libfuse_readlink(path *C.char, buf *C.char, size C.size_t) C.int {
	name := trimFusePath(path)
	name = common.NormalizeObjectName(name)
	//log.Trace("Libfuse::libfuse_readlink : Received for %s", name)

	targetPath, err := fuseFS.NextComponent().ReadLink(internal.ReadLinkOptions{Name: name})
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
	return 0
}

// libfuse_fsync synchronizes file contents
//export libfuse_fsync
func libfuse_fsync(path *C.char, datasync C.int, fi *C.fuse_file_info_t) C.int {
	name := trimFusePath(path)
	name = common.NormalizeObjectName(name)
	id := uint64(fi.fh)
	log.Trace("Libfuse::libfuse_fsync : %s, handle: %d", name, id)

	handle, ok := handlemap.Load(handlemap.HandleID(id))
	if !ok {
		log.Err("Libfuse::libfuse_fsync : file handle error [failed to find id=%d]", id)
		return -C.EIO
	}

	options := internal.SyncFileOptions{Handle: handle}
	// If the datasync parameter is non-zero, then only the user data should be flushed, not the metadata.
	// TODO : Should we support this?

	err := fuseFS.NextComponent().SyncFile(options)
	if err != nil {
		log.Err("Libfuse::libfuse_fsync : error syncing file %s [%s]", name, err.Error())
		return -C.EIO
	}
	return 0
}

// libfuse_fsyncdir synchronizes directory contents
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
	return 0
}

// libfuse2_chmod changes permission bits of a file
//export libfuse2_chmod
func libfuse2_chmod(path *C.char, mode C.mode_t) C.int {
	name := trimFusePath(path)
	name = common.NormalizeObjectName(name)
	log.Trace("Libfuse::libfuse2_chmod : %s", name)

	err := fuseFS.NextComponent().Chmod(
		internal.ChmodOptions{
			Name: name,
			Mode: fs.FileMode(uint32(mode) & 0xffffffff),
		})
	if err != nil {
		log.Err("Libfuse::libfuse2_chmod : error in chmod of %s [%s]", name, err.Error())
		if os.IsNotExist(err) {
			return -C.ENOENT
		}
		return -C.EIO
	}

	return 0
}

// libfuse2_chown changes the owner and group of a file
//export libfuse2_chown
func libfuse2_chown(path *C.char, uid C.uid_t, gid C.gid_t) C.int {
	name := trimFusePath(path)
	name = common.NormalizeObjectName(name)
	log.Trace("Libfuse::libfuse2_chown : %s", name)
	// TODO: Implement
	return 0
}

// libfuse2_utimens changes the access and modification times of a file
//export libfuse2_utimens
func libfuse2_utimens(path *C.char, tv *C.timespec_t) C.int {
	name := trimFusePath(path)
	name = common.NormalizeObjectName(name)
	log.Trace("Libfuse::libfuse2_utimens : %s", name)
	// TODO: is the conversion from [2]timespec to *timespec ok?
	// TODO: Implement
	// For now this returns 0 to allow touch to work correctly
	return 0
}
