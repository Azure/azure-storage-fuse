#include <stdio.h>
#include <stdlib.h>
#include <syslog.h>
#include "blobfuse_ext.h"

void *ext_init(struct fuse_conn_info *conn) 
{
    syslog(LOG_INFO, "EXT : Fuse Init called back in Extension");

    syslog(LOG_INFO, "EXT : Callbacks stored in struct %p", storage_callbacks);
    syslog(LOG_INFO, "EXT : Callbacks stored for init %p", storage_callbacks->init);

    // Callback storage endpoint for init
    if (storage_callbacks && storage_callbacks->init) {
        if (0 != storage_callbacks->init(conn)) {
            syslog(LOG_ERR, "EXT : Failed to init storage end point");
        } else {
            syslog(LOG_ERR, "EXT : init method of storage failed");
        }
    } else {
        syslog(LOG_ERR, "EXT : init method not populated for storage");
    }

    // Do local init here
    return NULL;
}

void ext_destroy(void *private_data)
{
    syslog(LOG_INFO, "EXT : Fuse Destroy called back in Extension");

    // Do local cleanup before calling storage endpoint as it may unload the library

    // Callback storage endpoint to destroy
    if (storage_callbacks && storage_callbacks->destroy) {
        storage_callbacks->destroy(private_data);
    } else {
        syslog(LOG_ERR, "EXT : destroy method not populated for storage");
    }

    // Release the memory holding storage callbacks
    free(storage_callbacks);
    return;
}

int ext_statfs(const char *path, struct statvfs *stbuf)
{
    syslog(LOG_INFO, "EXT : Fuse statfs called back in Extension");

    // Pass on the call to storage endpoint
    return storage_callbacks->statfs(path, stbuf);
}

int ext_getattr(const char *path, struct stat *stbuf)
{
    syslog(LOG_INFO, "EXT : Fuse getattr called back in Extension");

    // Pass on the call to storage endpoint
    return storage_callbacks->getattr(path, stbuf);
}

int ext_readdir(const char *path, void *buf, fuse_fill_dir_t filler, off_t offset, struct fuse_file_info *finfo)
{
    syslog(LOG_INFO, "EXT : Fuse readdir called back in Extension");

    // Pass on the call to storage endpoint
    return storage_callbacks->readdir(path, buf, filler, offset, finfo);
}

int ext_mkdir(const char *path, mode_t mode)
{
    syslog(LOG_INFO, "EXT : Fuse mkdir called back in Extension");

    // Pass on the call to storage endpoint
    return storage_callbacks->mkdir(path, mode);
}

int ext_rmdir(const char *path)
{
    syslog(LOG_INFO, "EXT : Fuse rmdir called back in Extension");

    // Pass on the call to storage endpoint
    return storage_callbacks->rmdir(path);
}