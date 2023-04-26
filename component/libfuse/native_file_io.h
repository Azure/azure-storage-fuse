
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

// Every read-write operation is counted and after N operations send a call up to update cache policy
#define CACHE_UPDATE_COUNTER 100


// Structure that describes file-handle object returned back to libfuse
typedef struct {
    uint64_t       fd;                  // Unix FD for this file
    uint64_t       obj;                 // Handlemap.Handle object representing this handle
    uint16_t       cnt;                 // Number of read-write operations done on this handle
    uint8_t        dirty;               // A write operation was performed on this handle
} file_handle_t;


// allocate_native_file_object : Allocate a native C-struct to hold handle map object and unix FD
static file_handle_t* allocate_native_file_object(uint64_t fd, uint64_t obj, uint64_t file_size)
{
    // Called on open / create calls from libfuse component
    file_handle_t* fobj = (file_handle_t*)malloc(sizeof(file_handle_t));
    if (fobj) {
        memset(fobj, 0, sizeof(file_handle_t));
        fobj->fd = fd;
        fobj->obj = obj;
    }

    return fobj;
}

// release_native_file_object : Release the native C-struct for handle 
static void release_native_file_object(fuse_file_info_t* fi)
{
    // Called on close operation from libfuse component
    file_handle_t* handle_obj = (file_handle_t*)fi->fh;
    if (handle_obj) {
        free(handle_obj);
    }
}


// native_pread :  Do pread on file directly without involving any Go code
static int native_pread(char *path, char *buf, size_t size, off_t offset, file_handle_t* handle_obj)
{
    errno = 0;
    int res = pread(handle_obj->fd, buf, size, offset);
    if (res == -1)
        res = -errno;
    
    #if 0
    handle_obj->cnt++;
    if (!(handle_obj->cnt % CACHE_UPDATE_COUNTER)) {
        // Time to send a call up to update the cache
        blobfuse_cache_update(path);
        handle_obj->cnt = 0;
    }   
    #endif
    
    return res;
}

// native_pwrite :  Do pwrite on file directly without involving any Go code
static int native_pwrite(char *path, char *buf, size_t size, off_t offset, file_handle_t* handle_obj)
{
    errno = 0;
    int res = pwrite(handle_obj->fd, buf, size, offset);
    if (res == -1)
        res = -errno;

    // Increment the operation counter and mark a write was done on this handle
    handle_obj->dirty = 1;
    handle_obj->cnt++;
    if (!(handle_obj->cnt % CACHE_UPDATE_COUNTER)) {
        // Time to send a call up to update the cache
        blobfuse_cache_update(path);
        handle_obj->cnt = 0;
    }

    return res;
}

// native_read_file : Read callback to decide whether to natively read or punt call to Go code
static int native_read_file(char *path, char *buf, size_t size, off_t offset, fuse_file_info_t *fi)
{
    file_handle_t* handle_obj = (file_handle_t*)fi->fh;
    #if 0
    return libfuse_read(path, buf, size, offset, fi);
    #endif

    if (handle_obj->fd == 0) {
        return libfuse_read(path, buf, size, offset, fi);
    }

    return native_pread(path, buf, size, offset, handle_obj);
}

// native_write_file : Write callback to decide whether to natively write or punt call to Go code
static int native_write_file(char *path, char *buf, size_t size, off_t offset, fuse_file_info_t *fi)
{
    file_handle_t* handle_obj = (file_handle_t*)fi->fh;
    #if 0
    return libfuse_write(path, buf, size, offset, fi);
    #endif

    if (handle_obj->fd == 0) {
        return libfuse_write(path, buf, size, offset, fi);
    }
    
    return native_pwrite(path, buf, size, offset, handle_obj);
}

// native_flush_file : Flush the file natively and call flush up in the pipeline to upload this file
static int native_flush_file(char *path, fuse_file_info_t *fi)
{
    file_handle_t* handle_obj = (file_handle_t*)fi->fh;
    int ret = libfuse_flush(path, fi);
    if (ret == 0) {
        // As file is flushed and uploaded, reset the dirty bit here
        handle_obj->dirty = 0;
    }

    return ret;
}


#ifdef ENABLE_READ_AHEAD
// read_ahead_handler : Method to serve read call from read-ahead buffer if possible
static int read_ahead_handler(char *path, char *buf, size_t size, off_t offset, file_handle_t* handle_obj) 
{
    int new_read = 0;

    /* Random read determination logic :
        handle_obj->random_reads : is counter used for this
        - For every sequential read decrement this counter by 1
        - For every new read from physical file (random read or buffer refresh) increment the counter by 2
        - At any point if the counter value is > 5 then caller will disable read-ahead on this handle

        : If file is being read sequentially then counter will be negative and a buffer refresh will not skew the counter much
        : If file is read sequentially and later application moves to random read, at some point we will disable read-ahead logic
        : If file is read randomly then counter will be positive and we will disable read-ahead after 2-3 reads
        : If file is read randomly first and then sequentially then we assume it will be random read and disable the read-ahead
    */

    if ((handle_obj->buff_start == 0  && handle_obj->buff_end == 0) || 
        offset < handle_obj->buff_start ||
        offset >= handle_obj->buff_end)
    {
        // Either this is first read call or read is outside the current buffer boundary
        // So we need to read a fresh buffer from physical file
        new_read = 1;
        handle_obj->random_reads += 2;
    } else {
        handle_obj->random_reads--;
    }

    if (new_read) {
        // We need to refresh the data from file
        int read = native_pread(path, handle_obj->buff, RA_BLOCK_SIZE, offset, handle_obj);
        FILE *fp = fopen("blobfuse2_nat.log", "a");
        if (fp) {
            fprintf(fp, "File %s, Offset %ld, size %ld, new read %d\n",
                path, offset, size, read);
            fclose(fp);
        }

        if (read <= 0) {
            // Error or EOF reached to just return 0 now
            return read;
        }

        handle_obj->buff_start = offset;
        handle_obj->buff_end = offset + read;
    }

    // Buffer is populated so calculate how much to copy from here now.
    int start = offset - handle_obj->buff_start;
    int left = (handle_obj->buff_end - offset);
    int copy = (size > left) ? left : size;
    
    FILE *fp = fopen("blobfuse2_nat.log", "a");
    if (fp) {
        fprintf(fp, "File %s, Offset %ld, size %ld, buff start %ld, buff end %ld, start %d, left %d, copy %d\n",
           path, offset, size, handle_obj->buff_start, handle_obj->buff_end, start, left, copy);
        fclose(fp);
    }

    memcpy(buf, (handle_obj->buff + start), copy);
    
    if (copy < size) {
        // Less then request data was copied so read from next offset again
        // We need to handle this here because if we return less then size fuse is not asking from
        // correct offset in next read, it just goes to offset + size only.
        copy += read_ahead_handler(path, (buf + copy), (size - copy), (offset + copy), handle_obj);
    }

    return copy;
}
#endif


#endif //__NATIVE_FILE_IO_H__