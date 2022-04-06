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
    #if 0
    opt->read       = (int (*)(const char *path, char *buf, size_t, off_t, fuse_file_info_t *))libfuse_read;
    opt->write      = (int (*)(const char *path, const char *buf, size_t, off_t, fuse_file_info_t *))libfuse_write;
    #else
    opt->read       = (int (*)(const char *path, char *buf, size_t, off_t, fuse_file_info_t *))native_read_file;
    opt->write      = (int (*)(const char *path, const char *buf, size_t, off_t, fuse_file_info_t *))native_write_file;
    #endif

    opt->flush      = (int (*)(const char *path, fuse_file_info_t *fi))libfuse_flush;
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


// To enable / disable read ahead logic just use this macro
//#define LIBFUSE_ENABLE_READ_AHEAD

#if defined(LIBFUSE_ENABLE_READ_AHEAD)
#define READ_AHEAD_MIN_FILE_SIZE (1 * 1024 * 1024)
#define READ_AHEAD_BUFF_SIZE (8 * 1024 * 1024)
#endif
typedef struct {
    uint64_t        fd;
    uint64_t        obj;
    #if defined(LIBFUSE_ENABLE_READ_AHEAD)
    uint64_t        buff_soffset;
    uint64_t        buff_eoffset;
    char*           buff;
    #endif
} file_handle_t;


// allocate_native_file_object : Allocate a native C-struct to hold handle map object and unix FD
static file_handle_t* allocate_native_file_object(uint64_t fd, uint64_t obj, uint64_t file_size)
{
    file_handle_t* fobj = (file_handle_t*)malloc(sizeof(file_handle_t));
    if (fobj) {
        memset(fobj, sizeof(file_handle_t), 0);

        fobj->fd = fd;
        fobj->obj = obj;
                
        #if defined(LIBFUSE_ENABLE_READ_AHEAD)
        // For very small files no point in doing read ahead
        if (file_size > READ_AHEAD_MIN_FILE_SIZE) {
            // If file size is bigger then buff_size then allocate read ahead buffer of X size
            // else allocate buffer of file size itself.
            uint64_t buff_size = (file_size > READ_AHEAD_BUFF_SIZE) ? READ_AHEAD_BUFF_SIZE : file_size;
            fobj->buff = (char*)malloc(sizeof(char) * buff_size);

            // Read first N bytes from the file and keep them ready to serve
            int read = pread(fd, fobj->buff, buff_size, 0);
            fobj->buff_soffset = 0;
            fobj->buff_eoffset = read - 1;
        }
        #endif
    }

    return fobj;
}

// release_native_file_object : Relase the native C-struct for handle 
static void release_native_file_object(file_handle_t* fobj)
{
    if (fobj) {
        #if defined(LIBFUSE_ENABLE_READ_AHEAD)
        if (fobj->buff) {
            free (fobj->buff);
        }
        #endif

        free(fobj);
    }
}

// native_pread :  Do pread on file directly wihtout involving any Go code
static int native_pread(char *path, char *buf, size_t size, off_t offset, file_handle_t* handle_obj)
{
    errno = 0;
    int res = pread(handle_obj->fd, buf, size, offset);
    if (res == -1)
        res = -errno;
        
    //blobfuse_cache_update(path);
    return res;
}

// native_read_file : Read callback to decide whether to natively read or punt call to Go code
static int native_read_file(char *path, char *buf, size_t size, off_t offset, fuse_file_info_t *fi)
{
    file_handle_t* handle_obj = (file_handle_t*)fi->fh;
    
    if (handle_obj->fd == 0) {
        return libfuse_read(path, buf, size, offset, fi);
    }

    #if !defined(LIBFUSE_ENABLE_READ_AHEAD)
    return native_pread(path, buf, size, offset, handle_obj);
    #else

    // If file is small then we do not read-ahead so directly read from file
    if (!handle_obj->buff) {
        return native_pread(path, buf, size, offset, handle_obj);
    }

    FILE *fp = fopen("blobfuse2_nat.log", "a");
    fprintf(fp, "Native Read for %s, offset %ld, len %ld\n", path, offset, size);
    fflush(fp);

    if (offset < handle_obj->buff_soffset ||
        offset > handle_obj->buff_eoffset) 
    {
        fprintf(fp, "Native Read : Reload buffer for %s, offset %ld, len %ld\n", path, offset, size);
        fflush(fp);

        int read = pread(handle_obj->fd, handle_obj->buff, READ_AHEAD_BUFF_SIZE, offset);
        handle_obj->buff_soffset = offset;
        handle_obj->buff_eoffset = offset + read - 1;
        fprintf(fp, "Native Read : Reload buffer for %s, offset %ld, len %ld, read %d\n", path, offset, size, read);
        fflush(fp);
    }

    fprintf(fp, "Native Read : Copying data for %s, offset %ld, size %ld, soffset %ld, eoffset %ld\n", 
        path, offset, size, handle_obj->buff_soffset, handle_obj->buff_eoffset);
    fflush(fp);

    int len = ((offset + size) > (handle_obj->buff_eoffset + 1)) ? 
                    (handle_obj->buff_eoffset - offset) + 1: 
                    size;

    fprintf(fp, "Native Read : Copying data for %s, offset %ld, len %d\n", path, offset, len);
    fflush(fp);

    memcpy(buf, handle_obj->buff + (offset - handle_obj->buff_soffset), len);
    if (fp) 
        fclose(fp);

    return len;
    #endif
}

// native_write_file : Write callback to decide whether to natively write or punt call to Go code
static int native_write_file(char *path, char *buf, size_t size, off_t offset, fuse_file_info_t *fi)
{
    file_handle_t* handle_obj = (file_handle_t*)fi->fh;
    if (handle_obj->fd == 0) {
        return libfuse_write(path, buf, size, offset, fi);
    }

    errno = 0;
    int res = pwrite(handle_obj->fd, buf, size, offset);
    if (res == -1)
        res = -errno;

    //blobfuse_cache_update(path);
    return res;
}

#endif //__LIBFUSE_H__