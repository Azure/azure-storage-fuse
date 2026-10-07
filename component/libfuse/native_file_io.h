
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
      - C only does the IO and marks the handle dirty when a write changed the file. Everything else is decided in
        Go: flush and release are Go callbacks that carry the dirty mark over to the handle so that file_cache
        uploads the file, and the cache policy restarts the file's cache timeout when it is closed.
*/

// file_handle_t : native object given to libfuse as the file handle (fi->fh) of an open file
typedef struct {
    uint64_t       fd;                  // Descriptor of the locally cached file, 0 if IO is not served natively
    uint64_t       obj;                 // handlemap.Handle of this open file
    uint8_t        dirty;               // A native write changed the file since the last flush
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
    file_handle_t* handle_obj = (file_handle_t*)fi->fh;
    if (handle_obj) {
        free(handle_obj);
    }
}


// native_pread : Read from the cached file
static int native_pread(char *buf, size_t size, off_t offset, file_handle_t* handle_obj)
{
    ssize_t res = pread(handle_obj->fd, buf, size, offset);
    if (res < 0)
        return -errno;

    return (int)res;
}

// native_pwrite : Write to the cached file, and mark the handle dirty if that changed the file
static int native_pwrite(char *buf, size_t size, off_t offset, file_handle_t* handle_obj)
{
    ssize_t res = pwrite(handle_obj->fd, buf, size, offset);
    if (res < 0)
        return -errno;

    if (res > 0)
        handle_obj->dirty = 1;

    return (int)res;
}

// native_read_file : libfuse read callback, served natively for cached files and by Go otherwise
static int native_read_file(char *path, char *buf, size_t size, off_t offset, fuse_file_info_t *fi)
{
    file_handle_t* handle_obj = (file_handle_t*)fi->fh;
    if (handle_obj->fd == 0)
        return libfuse_read(path, buf, size, offset, fi);

    return native_pread(buf, size, offset, handle_obj);
}

// native_write_file : libfuse write callback, served natively for cached files and by Go otherwise
static int native_write_file(char *path, char *buf, size_t size, off_t offset, fuse_file_info_t *fi)
{
    file_handle_t* handle_obj = (file_handle_t*)fi->fh;
    if (handle_obj->fd == 0)
        return libfuse_write(path, buf, size, offset, fi);

    return native_pwrite(buf, size, offset, handle_obj);
}

#endif // __NATIVE_FILE_IO_H__
