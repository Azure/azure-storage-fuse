
/*
 * Copyright (c) Microsoft Corporation.
 * Licensed under the MIT License
 */

#ifndef __NATIVE_FILE_IO_H__
#define __NATIVE_FILE_IO_H__

/*
    Native IO for files that the file_cache component holds locally.

    libfuse_open and libfuse_create put the descriptor of the local copy into a file_handle_t, which libfuse passes
    back with every request on the open file. Reads and writes on that descriptor are served here, without calling
    into Go: a Go callback for every request from libfuse's threads costs noticeably more CPU and throughput than
    serving the request in C.

    Contract with the Go side:
      - fd == 0 means the file is not served natively (it is not cached locally, or file_cache runs with
        offload-io: true). Those reads and writes go to libfuse_read/libfuse_write and through the pipeline. They get
        the handle ID of the file from here, so that Go does not have to call back into C on every read and write.
      - C only does the IO, and marks the handle dirty when a write changed the file. Everything else is decided in
        Go: flush, fsync and release consume the dirty mark (consume_dirty_flag) and carry it over to the handle, so
        that file_cache uploads the file.
*/

// file_handle_t : native object given to libfuse as the file handle (fi->fh) of an open file
typedef struct {
    uint64_t       fd;                  // Descriptor of the locally cached file, 0 if IO is not served natively
    uint64_t       obj;                 // handlemap.HandleID of this open file. C must not keep Go pointers.
    uint8_t        dirty;               // A native write changed the file since the mark was last consumed
} file_handle_t;


// allocate_native_file_object : Allocate the native object for an opened or created file
static file_handle_t* allocate_native_file_object(uint64_t fd, uint64_t obj)
{
    file_handle_t* fobj = (file_handle_t*)calloc(1, sizeof(file_handle_t));
    if (fobj) {
        fobj->fd = fd;
        fobj->obj = obj;
    }

    return fobj;
}

// get_native_file_object : Native object of an open file, as handed to libfuse when the file was opened or created
static file_handle_t* get_native_file_object(fuse_file_info_t* fi)
{
    return (file_handle_t*)fi->fh;
}

// release_native_file_object : Release the native object of a closed file
static void release_native_file_object(fuse_file_info_t* fi)
{
    free((file_handle_t*)fi->fh);
}


// native_read_file : libfuse read callback, served natively for cached files and by Go otherwise
static int native_read_file(char *path, char *buf, size_t size, off_t offset, fuse_file_info_t *fi)
{
    file_handle_t* handle_obj = (file_handle_t*)fi->fh;
    if (handle_obj->fd == 0)
        return libfuse_read(path, buf, size, offset, handle_obj->obj);

    ssize_t res = pread(handle_obj->fd, buf, size, offset);
    return res < 0 ? -errno : (int)res;
}

// native_write_file : libfuse write callback, served natively for cached files and by Go otherwise
static int native_write_file(char *path, char *buf, size_t size, off_t offset, fuse_file_info_t *fi)
{
    file_handle_t* handle_obj = (file_handle_t*)fi->fh;
    if (handle_obj->fd == 0)
        return libfuse_write(path, buf, size, offset, handle_obj->obj);

    ssize_t res = pwrite(handle_obj->fd, buf, size, offset);
    if (res < 0)
        return -errno;

    // Mark the handle dirty if the write changed the file. Go consumes the mark concurrently, see consume_dirty_flag.
    if (res > 0)
        __atomic_store_n(&handle_obj->dirty, 1, __ATOMIC_RELEASE);

    return (int)res;
}

// consume_dirty_flag : Atomically fetch and reset the dirty mark set by native writes on this handle.
// Flush, fsync and release consume it before they upload the file, rather than resetting it after the upload, so a
// write that lands while the file is uploaded marks the handle again. Go consumes the mark and marks the handle as one
// step under the handle's lock, see markNativeWritesDirty.
static int consume_dirty_flag(file_handle_t* handle_obj)
{
    return __atomic_exchange_n(&handle_obj->dirty, 0, __ATOMIC_ACQ_REL);
}

#endif // __NATIVE_FILE_IO_H__
