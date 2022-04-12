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

#ifndef __LIBFUSE_H__
#define __LIBFUSE_H__

#include <stdio.h>
#include <stdbool.h>
#include <stdint.h>
#include <stdlib.h>
#include <string.h>
#include <linux/fs.h>
#include <sys/types.h>
#include <errno.h>
#include <dlfcn.h>
#include <fcntl.h>
#include <unistd.h>

// Decide whether to add fuse2 or fuse3
#ifdef __FUSE2__
#include <fuse.h>
#else
#include <fuse3/fuse.h>
#endif

// There are structs defined in fuse3 only so giving a placeholder for fuse2
#ifdef __FUSE2__
enum  	fuse_readdir_flags { FUSE_READDIR_PLUS = (1 << 0) };
enum  	fuse_fill_dir_flags { FUSE_FILL_DIR_PLUS = (1 << 1) };
#endif

/*
    NOTES:
        1. Every method or variable defined in this file has to be static otherwise compilation will
           fail with multiple definition error
        2. Every method defined as static shall be defined in go code with //export <func-name> before it
        3. No blank line between C code and import "C" statement anywhere
        4. For void* import unsafe in Go and use unsafe.Pointer
        5. For C types like int use C.int in Go
        6. import "C" and code that uses it has to be in same Go file
*/

// -------------------------------------------------------------------------------------------------------------

typedef struct  fuse_operations         fuse_operations_t;
typedef struct  fuse_conn_info          fuse_conn_info_t;
typedef struct  fuse_config             fuse_config_t;
typedef struct  fuse_args               fuse_args_t;
typedef struct  fuse_file_info          fuse_file_info_t;
typedef struct  statvfs                 statvfs_t;
typedef struct  stat                    stat_t;
typedef struct  timespec                timespec_t;
typedef enum    fuse_readdir_flags      fuse_readdir_flags_t;
typedef enum    fuse_fill_dir_flags     fuse_fill_dir_flags_t;

// LibFuse callback declaration here
static int libfuse_statfs(const char *path, statvfs_t *stbuf);

extern void libfuse_destroy(void *private_data);

extern int libfuse_mkdir(char *path, mode_t mode);
extern int libfuse_rmdir(char *path);

extern int libfuse_opendir(char *path, fuse_file_info_t *fi);
extern int libfuse_releasedir(char *path, fuse_file_info_t *fi);

extern int libfuse_create(char *path, mode_t mode, fuse_file_info_t *fi);
extern int libfuse_open(char *path, fuse_file_info_t *fi);
extern int libfuse_read(char *path, char *buf, size_t size, off_t, fuse_file_info_t *fi);
extern int libfuse_write(char *path, char *buf, size_t size, off_t, fuse_file_info_t *fi);
extern int libfuse_flush(char *path, fuse_file_info_t *fi);
extern int libfuse_release(char *path, fuse_file_info_t *fi);
// truncate and rename is lib version specific so defined later
extern int libfuse_unlink(char *path);

extern int libfuse_symlink(char *from, char *to);
extern int libfuse_readlink(char *path, char *buf, size_t size);

extern int libfuse_fsync(char *path, int, fuse_file_info_t *fi);
extern int libfuse_fsyncdir(char *path, int, fuse_file_info_t *);

// chmod, chown and utimens are lib version specific so defined later

#ifdef __FUSE2__
extern void *libfuse2_init(fuse_conn_info_t *conn);
extern int libfuse2_getattr(char *path, stat_t *stbuf);
extern int libfuse2_readdir(char *path, void *buf, fuse_fill_dir_t filler, off_t, fuse_file_info_t *);
extern int libfuse2_truncate(char *path, off_t off);
extern int libfuse2_rename(char *src, char *dst);
extern int libfuse2_chmod(char *path, mode_t mode);
extern int libfuse2_chown(char *path, uid_t uid, gid_t gid);
extern int libfuse2_utimens(char *path, timespec_t tv[2]);
#else
extern void *libfuse_init(fuse_conn_info_t *conn, fuse_config_t *cfg);
extern int libfuse_getattr(char *path, stat_t *stbuf, fuse_file_info_t *fi);
extern int libfuse_readdir(char *path, void *buf, fuse_fill_dir_t filler, off_t, fuse_file_info_t *, fuse_readdir_flags_t);
extern int libfuse_truncate(char *path, off_t off, fuse_file_info_t *fi);
extern int libfuse_rename(char *src, char *dst, unsigned int flags);
extern int libfuse_chmod(char *path, mode_t mode, fuse_file_info_t *fi);
extern int libfuse_chown(char *path, uid_t uid, gid_t gid, fuse_file_info_t *fi);
extern int libfuse_utimens(char *path, timespec_t tv[2], fuse_file_info_t *fi);
#endif

extern int blobfuse_cache_update(char* path);
static int native_read_file(char *path, char *buf, size_t size, off_t, fuse_file_info_t *fi);
static int native_write_file(char *path, char *buf, size_t size, off_t, fuse_file_info_t *fi);
static int native_flush_file(char *path, fuse_file_info_t *fi);
static int native_close_file(char *path, fuse_file_info_t *fi);

// -------------------------------------------------------------------------------------------------------------
// Methods not implemented by blobfuse2

// extern int libfuse_mknod(char *path, mode_t mode, dev_t dev);
// extern int libfuse_link(char *from, char *to);
// extern int libfuse_setxattr(char *path, char *name, char *value, size_t size, int flags);
// extern int libfuse_getxattr(char *path, char *name, char *value, size_t size);
// extern int libfuse_listxattr(char* path, char *list, size_t size);
// extern int libfuse_removexattr(char *path, char *name);
// extern int libfuse_access(char *path, int mask);
// extern int libfuse_lock
// extern int libfuse_bmap
// extern int libfuse_ioctl
// extern int libfuse_poll
// extern int libfuse_write_buf
// extern int libfuse_read_buf
// extern int libfuse_flock
// extern int libfuse_fallocate
// extern int libfuse_copyfilerange
// extern int libfuse_lseek
// -------------------------------------------------------------------------------------------------------------



// Method to populate the fuse structure with our callback methods
static int populate_callbacks(fuse_operations_t *opt)
{
    opt->destroy    = (void (*)(void *))libfuse_destroy;

    opt->statfs     = (int (*)(const char *, statvfs_t *))libfuse_statfs;

    opt->mkdir      = (int (*)(const char *path, mode_t mode))libfuse_mkdir;
    opt->rmdir      = (int (*)(const char *path))libfuse_rmdir;

    opt->opendir    = (int (*)(const char *path, fuse_file_info_t *fi))libfuse_opendir;
    opt->releasedir = (int (*)(const char *path, fuse_file_info_t *fi))libfuse_releasedir;

    opt->create     = (int (*)(const char *path, mode_t mode, fuse_file_info_t *fi))libfuse_create;
    opt->open       = (int (*)(const char *path, fuse_file_info_t *fi))libfuse_open;

    // These are methods declared in C to do read/write operation directly on file for better performance
    opt->read       = (int (*)(const char *path, char *buf, size_t, off_t, fuse_file_info_t *))native_read_file;
    opt->write      = (int (*)(const char *path, const char *buf, size_t, off_t, fuse_file_info_t *))native_write_file;
    opt->flush      = (int (*)(const char *path, fuse_file_info_t *fi))native_flush_file;
    opt->release    = (int (*)(const char *path, fuse_file_info_t *fi))libfuse_release;

    opt->unlink     = (int (*)(const char *path))libfuse_unlink;

    opt->symlink    = (int (*)(const char *from, const char *to))libfuse_symlink;
    opt->readlink   = (int (*)(const char *path, char *buf, size_t size))libfuse_readlink;

    opt->fsync      = (int (*)(const char *path, int, fuse_file_info_t *fi))libfuse_fsync;
    opt->fsyncdir   = (int (*)(const char *path, int, fuse_file_info_t *))libfuse_fsyncdir;


    #ifdef __FUSE2__
    opt->init       = (void *(*)(fuse_conn_info_t *))libfuse2_init;
    opt->getattr    = (int (*)(const char *, stat_t *))libfuse2_getattr;
    opt->readdir    = (int (*)(const char *path, void *buf, fuse_fill_dir_t filler, off_t, fuse_file_info_t *))libfuse2_readdir;
    opt->truncate   = (int (*)(const char *path, off_t off))libfuse2_truncate;
    opt->rename     = (int (*)(const char *src, const char *dst))libfuse2_rename;
    opt->chmod      = (int (*)(const char *path, mode_t mode))libfuse2_chmod;
    opt->chown      = (int (*)(const char *path, uid_t uid, gid_t gid))libfuse2_chown;
    opt->utimens    = (int (*)(const char *path, const timespec_t tv[2]))libfuse2_utimens;
    #else
    opt->init       = (void *(*)(fuse_conn_info_t *, fuse_config_t *))libfuse_init;
    opt->getattr    = (int (*)(const char *, stat_t *, fuse_file_info_t *))libfuse_getattr;
    opt->readdir    = (int (*)(const char *path, void *buf, fuse_fill_dir_t filler, off_t, fuse_file_info_t *, 
                               fuse_readdir_flags_t))libfuse_readdir;
    opt->truncate   = (int (*)(const char *path, off_t off, fuse_file_info_t *fi))libfuse_truncate;
    opt->rename     = (int (*)(const char *src, const char *dst, unsigned int flags))libfuse_rename;
    opt->chmod      = (int (*)(const char *path, mode_t mode, fuse_file_info_t *fi))libfuse_chmod;
    opt->chown      = (int (*)(const char *path, uid_t uid, gid_t gid, fuse_file_info_t *fi))libfuse_chown;
    opt->utimens    = (int (*)(const char *path, const timespec_t tv[2], fuse_file_info_t *fi))libfuse_utimens;
    #endif

    return 0;
}

// Structure to hold config for libfuse
typedef struct fuse_options
{
    char    *mount_path;
    uid_t   uid;
    gid_t   gid;
    mode_t  permissions;
    int     entry_expiry;
    int     attr_expiry;
    int     negative_expiry;
    bool    readonly;
    bool    allow_other;
    bool    trace_enable;
} fuse_options_t;

static fuse_options_t fuse_opts;
static bool context_populated = false;

// Main method to start fuse loop which will fork and send us callbacks
static int start_fuse(fuse_args_t *args, fuse_operations_t *opt)
{
    return fuse_main(args->argc, args->argv, opt, NULL);
}

// This method is not declared in Go because we are just doing "/" statfs as dummy operation
static int libfuse_statfs(const char *path, struct statvfs *stbuf)
{
    // return tmp path stats
    errno = 0;
    int res = statvfs("/", stbuf);
    if (res == -1)
        return -errno;

    return 0;
}

// Get uid and gid from fuse context
static void populate_uid_gid()
{
    if (!context_populated)
    {
        fuse_opts.uid = fuse_get_context()->uid;
        fuse_opts.gid = fuse_get_context()->gid;
        context_populated = true;
    }
}

// Properties for root (/) are static so just hardcoding them here
static int get_root_properties(stat_t *stbuf)
{
    populate_uid_gid();

    stbuf->st_mode = S_IFDIR | 0777;
    stbuf->st_uid = fuse_opts.uid;
    stbuf->st_gid = fuse_opts.gid;
    stbuf->st_nlink = 2;
    stbuf->st_size = 4096;
    stbuf->st_mtime = time(NULL);
    stbuf->st_atime = stbuf->st_mtime;
    stbuf->st_ctime = stbuf->st_mtime;
    return 0;
}

#ifdef __FUSE2__
static int fill_dir_plus = 0;
#else
static int fill_dir_plus = FUSE_FILL_DIR_PLUS;
#endif

static int fill_dir_entry(fuse_fill_dir_t filler, void *buf, char *name, stat_t *stbuf, off_t off)
{
    return filler(buf, name, stbuf, off
    #ifndef __FUSE2__
        ,(fuse_fill_dir_flags_t) fill_dir_plus
    #endif
    );
}

// ---------   Native READ-WRITE and READ-AHEAD logic here ---------------------
// Every read-write operation is counted and after N operations send a call up to update cache policy
#define CACHE_UPDATE_COUNTER 1000


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
    
    if (handle_obj->fd == 0) {
        return libfuse_read(path, buf, size, offset, fi);
    }

    return native_pread(path, buf, size, offset, handle_obj);
}

// native_write_file : Write callback to decide whether to natively write or punt call to Go code
static int native_write_file(char *path, char *buf, size_t size, off_t offset, fuse_file_info_t *fi)
{
    file_handle_t* handle_obj = (file_handle_t*)fi->fh;
    
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
// ---------------------------------------------------------------------------------

#endif //__LIBFUSE_H__