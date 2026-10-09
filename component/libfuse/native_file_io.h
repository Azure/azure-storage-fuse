
/*
    _____           _____   _____   ____          ______  _____  ------
   |     |  |      |     | |     | |     |     | |       |            |
   |     |  |      |     | |     | |     |     | |       |            |
   | --- |  |      |     | |-----| |---- |     | |-----| |-----  ------
   |     |  |      |     | |     | |     |     |       | |       |
   | ____|  |_____ | ____| | ____| |     |_____|  _____| |_____  |_____


   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2020-2023 Microsoft Corporation. All rights reserved.
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
        offload-io: true). Those reads and writes go to libfuse_read/libfuse_write and through the pipeline.
      - C only does the IO, and marks the handle dirty when a write changed the file. Everything else is decided in
        Go: flush, fsync and release carry the dirty mark over to the handle (peek_dirty_flag, clear_dirty_flag),
        so that file_cache uploads the file.
*/

// file_handle_t : native object given to libfuse as the file handle (fi->fh) of an open file
typedef struct {
    uint64_t       fd;                  // Descriptor of the locally cached file, 0 if IO is not served natively
    uint64_t       obj;                 // handlemap.Handle of this open file
    uint8_t        dirty;               // A native write changed the file since the mark was last cleared
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
        return libfuse_read(path, buf, size, offset, fi);

    ssize_t res = pread(handle_obj->fd, buf, size, offset);
    return res < 0 ? -errno : (int)res;
}

// native_write_file : libfuse write callback, served natively for cached files and by Go otherwise
static int native_write_file(char *path, char *buf, size_t size, off_t offset, fuse_file_info_t *fi)
{
    file_handle_t* handle_obj = (file_handle_t*)fi->fh;
    if (handle_obj->fd == 0)
        return libfuse_write(path, buf, size, offset, fi);

    ssize_t res = pwrite(handle_obj->fd, buf, size, offset);
    if (res < 0)
        return -errno;

    // Mark the handle dirty if the write changed the file. Go clears the mark concurrently, see clear_dirty_flag.
    if (res > 0)
        __atomic_store_n(&handle_obj->dirty, 1, __ATOMIC_RELEASE);

    return (int)res;
}

// peek_dirty_flag : Whether native writes changed the file since the dirty mark was last cleared
static int peek_dirty_flag(file_handle_t* handle_obj)
{
    return __atomic_load_n(&handle_obj->dirty, __ATOMIC_ACQUIRE);
}

// clear_dirty_flag : Clear the dirty mark, once Go has marked the handle dirty. Flush, fsync and release clear it
// before they upload the file, rather than after the upload, so a write that lands during the upload marks it again.
// The exchange, rather than a plain store, makes the writes whose mark it clears visible to that upload.
static void clear_dirty_flag(file_handle_t* handle_obj)
{
    __atomic_exchange_n(&handle_obj->dirty, 0, __ATOMIC_ACQ_REL);
}

#endif // __NATIVE_FILE_IO_H__
