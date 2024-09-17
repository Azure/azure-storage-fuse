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

#include "libfuse_defs.h"
#include "native_file_io.h"

// Method to populate the fuse structure with our callback methods
static int populate_callbacks(fuse_operations_t *opt)
{
    opt->destroy    = (void (*)(void *))libfuse_destroy;

    opt->statfs     = (int (*)(const char *path, statvfs_t *stbuf))libfuse_statfs;

    opt->mkdir      = (int (*)(const char *path, mode_t mode))libfuse_mkdir;
    opt->rmdir      = (int (*)(const char *path))libfuse_rmdir;

    opt->opendir    = (int (*)(const char *path, fuse_file_info_t *fi))libfuse_opendir;
    opt->releasedir = (int (*)(const char *path, fuse_file_info_t *fi))libfuse_releasedir;

    opt->create     = (int (*)(const char *path, mode_t mode, fuse_file_info_t *fi))libfuse_create;
    opt->open       = (int (*)(const char *path, fuse_file_info_t *fi))libfuse_open;

    // These are methods declared in C to do read/write operation directly on file for better performance
    #if 1
    opt->read       = (int (*)(const char *path, char *buf, size_t, off_t, fuse_file_info_t *))native_read_file;
    opt->write      = (int (*)(const char *path, const char *buf, size_t, off_t, fuse_file_info_t *))native_write_file;
    opt->flush      = (int (*)(const char *path, fuse_file_info_t *fi))native_flush_file;
    #else
    opt->read       = (int (*)(const char *path, char *buf, size_t, off_t, fuse_file_info_t *))libfuse_read;
    opt->write      = (int (*)(const char *path, const char *buf, size_t, off_t, fuse_file_info_t *))libfuse_write;
    opt->flush      = (int (*)(const char *path, fuse_file_info_t *fi))libfuse_flush;
    #endif
    
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

static fuse_options_t fuse_opts;
static bool context_populated = false;

// Main method to start fuse loop which will fork and send us callbacks
static int start_fuse(fuse_args_t *args, fuse_operations_t *opt)
{
    return fuse_main(args->argc, args->argv, opt, NULL);
}

// This method is not declared in Go because we are just doing "/" statfs as dummy operation
static int populate_statfs(const char *path, struct statvfs *stbuf)
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

static int fill_dir_entry(fuse_fill_dir_t filler, void *buf, char *name, stat_t *stbuf, off_t off)
{
    return filler(buf, name, stbuf, off
    #ifndef __FUSE2__
        ,(fuse_fill_dir_flags_t) fill_dir_plus
    #endif
    );
}

#endif //__LIBFUSE_H__