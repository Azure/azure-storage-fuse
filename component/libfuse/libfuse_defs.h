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

#ifndef __LIBFUSE_DEFS_H__
#define __LIBFUSE_DEFS_H__

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


// There are structs defined in fuse3 only so giving a placeholder for fuse2
#ifdef __FUSE2__
enum  	fuse_readdir_flags { FUSE_READDIR_PLUS = (1 << 0) };
enum  	fuse_fill_dir_flags { FUSE_FILL_DIR_PLUS = (1 << 1) };
#endif


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

#ifdef __FUSE2__
static int fill_dir_plus = 0;
#else
static int fill_dir_plus = FUSE_FILL_DIR_PLUS;
#endif

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
    bool    allow_root;
    bool    trace_enable;
    bool    non_empty;
    int     umask;
} fuse_options_t;



// LibFuse callback declaration here
extern int libfuse_statfs(char *path, statvfs_t *stbuf);

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

// Methods that needs handling in the CGo wrapper for better performance
extern int blobfuse_cache_update(char* path);
static int native_read_file(char *path, char *buf, size_t size, off_t, fuse_file_info_t *fi);
static int native_write_file(char *path, char *buf, size_t size, off_t, fuse_file_info_t *fi);
static int native_flush_file(char *path, fuse_file_info_t *fi);


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


#endif // __LIBFUSE_DEFS_H__