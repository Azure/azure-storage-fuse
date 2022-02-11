#include <stdio.h>
#include <stdlib.h>
#include <syslog.h>
#include <errno.h>
#include <string.h>
#include "galactus.h"

extern struct fuse_operations storage_callbacks;

void *ext_init(struct fuse_conn_info *conn, struct fuse_config *cfg) 
{
    syslog(LOG_DEBUG, "EXT : Fuse Init called back in Extension");

    // Callback storage endpoint for init
    if (storage_callbacks.init) {
        if (NULL != storage_callbacks.init(conn, cfg)) {
            syslog(LOG_ERR, "EXT : Failed to init storage end point");
        }
    } else {
        syslog(LOG_ERR, "EXT : init method not populated for storage");
    }

    // Do local init here
    return NULL;
}

void ext_destroy(void *private_data)
{
    syslog(LOG_DEBUG, "EXT : Fuse Destroy called back in Extension");

    // Do local cleanup before calling storage endpoint as it may unload the library

    // Callback storage endpoint to destroy
    if (storage_callbacks.destroy) {
        storage_callbacks.destroy(private_data);
    } else {
        syslog(LOG_ERR, "EXT : destroy method not populated for storage");
    }

    return;
}

int ext_statfs(const char *path, struct statvfs *stbuf)
{
    syslog(LOG_DEBUG, "EXT : Fuse statfs called back in Extension");

    // Pass on the call to storage endpoint
    return storage_callbacks.statfs(path, stbuf);
}

int ext_getattr(const char *path, struct stat *stbuf, struct fuse_file_info *fi)
{
    syslog(LOG_DEBUG, "EXT : Fuse getattr called back in Extension");

    syslog(LOG_DEBUG, "EXT : Fuse getattr called for %s", path);

    if (strcmp(path, "/subtree.sh") == 0) {
        syslog(LOG_DEBUG, "EXT : Fuse getattr called for %s Matches Filter", path);
        return -ENOENT;
    }

    // Pass on the call to storage endpoint
    return storage_callbacks.getattr(path, stbuf, fi);
}

int ext_opendir(const char *path, struct fuse_file_info *fi)
{
    syslog(LOG_DEBUG, "EXT : Fuse opendir called back in Extension");

    // Pass on the call to storage endpoint
    return storage_callbacks.opendir(path, fi);
}

int ext_releasedir(const char *path, struct fuse_file_info *fi)
{
    syslog(LOG_DEBUG, "EXT : Fuse releasedir called back in Extension");

    // Pass on the call to storage endpoint
    return storage_callbacks.releasedir(path, fi);
}

int ext_readdir(const char *path, void *buf, fuse_fill_dir_t filler, off_t offset, struct fuse_file_info *finfo, enum fuse_readdir_flags flag)
{
    syslog(LOG_DEBUG, "EXT : Fuse readdir called back in Extension");

    // Pass on the call to storage endpoint
    return storage_callbacks.readdir(path, buf, filler, offset, finfo, flag);
}

int ext_mkdir(const char *path, mode_t mode)
{
    syslog(LOG_DEBUG, "EXT : Fuse mkdir called back in Extension");

    // Pass on the call to storage endpoint
    return storage_callbacks.mkdir(path, mode);
}

int ext_rmdir(const char *path)
{
    syslog(LOG_DEBUG, "EXT : Fuse rmdir called back in Extension");

    // Pass on the call to storage endpoint
    return storage_callbacks.rmdir(path);
}

int ext_open(const char *path, struct fuse_file_info *fi)
{
    syslog(LOG_DEBUG, "EXT : Fuse open called back in Extension");

    // Pass on the call to storage endpoint
    return storage_callbacks.open(path, fi);
}

int ext_create(const char *path, mode_t mode, struct fuse_file_info *fi)
{
    syslog(LOG_DEBUG, "EXT : Fuse create called back in Extension");

    // Pass on the call to storage endpoint
    return storage_callbacks.create(path, mode, fi);
}

int ext_read(const char *path, char *buf, size_t size, off_t offset, struct fuse_file_info *fi)
{
    syslog(LOG_DEBUG, "EXT : Fuse read called back in Extension");

    // Pass on the call to storage endpoint
    return storage_callbacks.read(path, buf, size, offset, fi);
}

int ext_write(const char *path, const char *buf, size_t size, off_t offset, struct fuse_file_info *fi)
{
    syslog(LOG_DEBUG, "EXT : Fuse write called back in Extension");

    // Pass on the call to storage endpoint
    return storage_callbacks.write(path, buf, size, offset, fi);
}

int ext_flush(const char *path, struct fuse_file_info *fi)
{
    syslog(LOG_DEBUG, "EXT : Fuse flush called back in Extension");

    // Pass on the call to storage endpoint
    return storage_callbacks.flush(path, fi);
}

int ext_truncate(const char * path, off_t off, struct fuse_file_info *fi)
{
    syslog(LOG_DEBUG, "EXT : Fuse truncate called back in Extension");

    // Pass on the call to storage endpoint
    return storage_callbacks.truncate(path, off, fi);
}

int ext_release(const char *path, struct fuse_file_info * fi)
{
    syslog(LOG_DEBUG, "EXT : Fuse release called back in Extension");

    // Pass on the call to storage endpoint
    return storage_callbacks.release(path, fi);  
}

int ext_unlink(const char *path)
{
    syslog(LOG_DEBUG, "EXT : Fuse unlink called back in Extension");

    // Pass on the call to storage endpoint
    return storage_callbacks.unlink(path); 
}

int ext_rename(const char *src, const char *dst, unsigned int flags)
{
    syslog(LOG_DEBUG, "EXT : Fuse rename called back in Extension");

    // Pass on the call to storage endpoint
    return storage_callbacks.rename(src, dst, flags);    
}

int ext_symlink(const char *from, const char *to)
{
    syslog(LOG_DEBUG, "EXT : Fuse symlink called back in Extension");

    // Pass on the call to storage endpoint
    return storage_callbacks.symlink(from, to);
}

int ext_readlink(const char *path, char *buf, size_t size)
{
    syslog(LOG_DEBUG, "EXT : Fuse readlink called back in Extension");

    // Pass on the call to storage endpoint
    return storage_callbacks.readlink(path, buf, size);  
}

int ext_fsync(const char * path, int flag, struct fuse_file_info *fi)
{
    syslog(LOG_DEBUG, "EXT : Fuse fsync called back in Extension");

    // Pass on the call to storage endpoint
    return storage_callbacks.fsync(path, flag, fi);      
}

int ext_fsyncdir(const char *path, int flag, struct fuse_file_info *fi)
{
    syslog(LOG_DEBUG, "EXT : Fuse fsyncdir called back in Extension");

    // Pass on the call to storage endpoint
    return storage_callbacks.fsyncdir(path, flag, fi); 
}

int ext_chmod(const char *path, mode_t mode, struct fuse_file_info* fi)
{
    syslog(LOG_DEBUG, "EXT : Fuse chmod called back in Extension");

    // Pass on the call to storage endpoint
    return storage_callbacks.chmod(path, mode, fi);    
}