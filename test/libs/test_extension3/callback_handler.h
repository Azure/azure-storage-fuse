
#ifndef __CALLBACK_HANDLERS_H__
#define __CALLBACK_HANDLERS_H__

// BLOBFUSE2 : This is the extension contract to be implemented to create and extension
// callback_handlers.c is a sample code for the same

// Use this command to build callback_handlers.c : "gcc -shared -o libextension3.so -D_FILE_OFFSET_BITS=64 -fPIC extension3.c callback_handlers.c "
// Use this command to build a static lib : gcc -Wall -fPIC -D_FILE_OFFSET_BITS=64 -DCMAKE_BUILD_TYPE=Debug -c *.c && ar -cvq libextension3.a *.o
#include <stddef.h>
#include <stdio.h>


// This line must come before #include <fuse.h>.
#define FUSE_USE_VERSION 35
#include <fuse3/fuse.h>


// -------------------------------------------------------------------------------------------------------------
// Methods to be defined by the extension. Sample in callback_handlers.c
extern void *ext_init(struct fuse_conn_info *conn, struct fuse_config *cfg);
extern void ext_destroy(void *private_data);

extern int ext_statfs(const char *path, struct statvfs *stbuf);
extern int ext_getattr(const char *path, struct stat *stbuf, struct fuse_file_info *fi);

extern int ext_opendir(const char *path, struct fuse_file_info *fi);
extern int ext_releasedir(const char *path, struct fuse_file_info *fi);

extern int ext_readdir(const char *path, void *buf, fuse_fill_dir_t filler, off_t, struct fuse_file_info *, enum fuse_readdir_flags);
extern int ext_mkdir(const char *path, mode_t mode);
extern int ext_rmdir(const char *path);

extern int ext_open(const char *path, struct fuse_file_info *fi);
extern int ext_create(const char *path, mode_t mode, struct fuse_file_info *fi);
extern int ext_read(const char *path, char *buf, size_t size, off_t offset, struct fuse_file_info *fi);
extern int ext_write(const char *path, const char *buf, size_t size, off_t offset, struct fuse_file_info *fi);
extern int ext_flush(const char *path, struct fuse_file_info *fi);
extern int ext_truncate(const char * path, off_t off, struct fuse_file_info *fi);
extern int ext_release(const char *path, struct fuse_file_info * fi);

extern int ext_unlink(const char *path);
extern int ext_rename(const char *src, const char *dst, unsigned int flags);

extern int ext_symlink(const char *from, const char *to);
extern int ext_readlink(const char *path, char *buf, size_t size);

extern int ext_fsync(const char * path, int , struct fuse_file_info *fi);
extern int ext_fsyncdir(const char *path, int, struct fuse_file_info *);

extern int ext_chmod(const char *path, mode_t mode, struct fuse_file_info* fi);
// -------------------------------------------------------------------------------------------------------------


#endif