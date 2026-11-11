/* Copyright (c) Microsoft Corporation. Licensed under the MIT License. */
#ifndef BLOBFUSE_TELEMETRY_CALLBACKS_H
#define BLOBFUSE_TELEMETRY_CALLBACKS_H

#include "telemetry_counts.h"

static const char *telemetry_libfuse_version(void)
{
    typedef const char *(*version_func)(void);
    void *handle = dlopen(NULL, RTLD_LAZY);
    if (!handle)
        return "unknown";
    version_func version = (version_func)dlsym(handle, "fuse_pkgversion");
    const char *result = version ? version() : "unknown";
    dlclose(handle);
    return result;
}

#define TRACK_CALLBACK(name, index, arguments, expression) \
    static int tracked_##name arguments { return telemetry_result(index, (expression)); }

TRACK_CALLBACK(statfs, 0, (const char *p, statvfs_t *s), libfuse_statfs((char *)p, s))
TRACK_CALLBACK(mkdir, 1, (const char *p, mode_t m), libfuse_mkdir((char *)p, m))
TRACK_CALLBACK(rmdir, 2, (const char *p), libfuse_rmdir((char *)p))
TRACK_CALLBACK(opendir, 3, (const char *p, fuse_file_info_t *f), libfuse_opendir((char *)p, f))
TRACK_CALLBACK(releasedir, 4, (const char *p, fuse_file_info_t *f), libfuse_releasedir((char *)p, f))
TRACK_CALLBACK(create, 5, (const char *p, mode_t m, fuse_file_info_t *f), libfuse_create((char *)p, m, f))
TRACK_CALLBACK(open, 6, (const char *p, fuse_file_info_t *f), libfuse_open((char *)p, f))
TRACK_CALLBACK(read, 7, (const char *p, char *b, size_t n, off_t o, fuse_file_info_t *f),
    native_read_file((char *)p, b, n, o, f))
TRACK_CALLBACK(write, 8, (const char *p, const char *b, size_t n, off_t o, fuse_file_info_t *f),
    native_write_file((char *)p, (char *)b, n, o, f))
TRACK_CALLBACK(flush, 9, (const char *p, fuse_file_info_t *f), libfuse_flush((char *)p, f))
TRACK_CALLBACK(release, 10, (const char *p, fuse_file_info_t *f), libfuse_release((char *)p, f))
TRACK_CALLBACK(unlink, 11, (const char *p), libfuse_unlink((char *)p))
TRACK_CALLBACK(symlink, 12, (const char *a, const char *b), libfuse_symlink((char *)a, (char *)b))
TRACK_CALLBACK(readlink, 13, (const char *p, char *b, size_t n), libfuse_readlink((char *)p, b, n))
TRACK_CALLBACK(fsync, 14, (const char *p, int d, fuse_file_info_t *f), libfuse_fsync((char *)p, d, f))
TRACK_CALLBACK(fsyncdir, 15, (const char *p, int d, fuse_file_info_t *f), libfuse_fsyncdir((char *)p, d, f))

#ifdef __FUSE2__
TRACK_CALLBACK(getattr, 16, (const char *p, stat_t *s), libfuse2_getattr((char *)p, s))
TRACK_CALLBACK(readdir, 17, (const char *p, void *b, fuse_fill_dir_t fill, off_t o, fuse_file_info_t *f),
    libfuse2_readdir((char *)p, b, fill, o, f))
TRACK_CALLBACK(truncate, 18, (const char *p, off_t n), libfuse2_truncate((char *)p, n))
TRACK_CALLBACK(rename, 19, (const char *a, const char *b), libfuse2_rename((char *)a, (char *)b))
TRACK_CALLBACK(chmod, 20, (const char *p, mode_t m), libfuse2_chmod((char *)p, m))
TRACK_CALLBACK(chown, 21, (const char *p, uid_t u, gid_t g), libfuse2_chown((char *)p, u, g))
TRACK_CALLBACK(utimens, 22, (const char *p, const timespec_t t[2]), libfuse2_utimens((char *)p, (timespec_t *)t))
#else
TRACK_CALLBACK(getattr, 16, (const char *p, stat_t *s, fuse_file_info_t *f), libfuse_getattr((char *)p, s, f))
TRACK_CALLBACK(readdir, 17, (const char *p, void *b, fuse_fill_dir_t fill, off_t o, fuse_file_info_t *f, enum fuse_readdir_flags flags),
    libfuse_readdir((char *)p, b, fill, o, f, flags))
TRACK_CALLBACK(truncate, 18, (const char *p, off_t n, fuse_file_info_t *f), libfuse_truncate((char *)p, n, f))
TRACK_CALLBACK(rename, 19, (const char *a, const char *b, unsigned int flags), libfuse_rename((char *)a, (char *)b, flags))
TRACK_CALLBACK(chmod, 20, (const char *p, mode_t m, fuse_file_info_t *f), libfuse_chmod((char *)p, m, f))
TRACK_CALLBACK(chown, 21, (const char *p, uid_t u, gid_t g, fuse_file_info_t *f), libfuse_chown((char *)p, u, g, f))
TRACK_CALLBACK(utimens, 22, (const char *p, const timespec_t t[2], fuse_file_info_t *f), libfuse_utimens((char *)p, (timespec_t *)t, f))
#endif

#undef TRACK_CALLBACK

static void telemetry_wrap_callbacks(fuse_operations_t *opt)
{
    opt->statfs = tracked_statfs;
    opt->mkdir = tracked_mkdir;
    opt->rmdir = tracked_rmdir;
    opt->opendir = tracked_opendir;
    opt->releasedir = tracked_releasedir;
    opt->create = tracked_create;
    opt->open = tracked_open;
    opt->read = tracked_read;
    opt->write = tracked_write;
    opt->flush = tracked_flush;
    opt->release = tracked_release;
    opt->unlink = tracked_unlink;
    opt->symlink = tracked_symlink;
    opt->readlink = tracked_readlink;
    opt->fsync = tracked_fsync;
    opt->fsyncdir = tracked_fsyncdir;
    opt->getattr = tracked_getattr;
    opt->readdir = tracked_readdir;
    opt->truncate = tracked_truncate;
    opt->rename = tracked_rename;
    opt->chmod = tracked_chmod;
    opt->chown = tracked_chown;
    opt->utimens = tracked_utimens;
}

#endif
