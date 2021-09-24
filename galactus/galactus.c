#include <stdio.h>
#include <stdlib.h>
#include <syslog.h>

#include "galactus.h"
#include "callback_handlers.h"

#ifdef __cplusplus
extern "C" {
#endif

int populate_fuse_callbacks(struct fuse_operations *opts)
{
    syslog(LOG_INFO, "EXT : Populating fuse callbacks");
    opts->init           = ext_init;
    opts->destroy        = ext_destroy;
    
    opts->statfs         = ext_statfs;
    opts->getattr        = ext_getattr;

    opts->readdir        = ext_readdir;
    opts->mkdir          = ext_mkdir;
    opts->rmdir          = ext_rmdir;

    opts->open           = ext_open;
    opts->create         = ext_create;
    opts->read           = ext_read;
    opts->write          = ext_write;
    opts->flush          = ext_flush;
    opts->truncate       = ext_truncate;
    opts->release        = ext_release;

    opts->unlink         = ext_unlink;
    opts->rename         = ext_rename;

    opts->symlink        = ext_symlink;
    opts->readlink       = ext_readlink;

    opts->fsync          = ext_fsync;
    opts->fsyncdir       = ext_fsyncdir;

    opts->chmod          = ext_chmod;

    return 0;
}

// Call this method to populate callbacks to communicate with blobfuse
int populate_storage_callbacks(struct fuse_operations *opts)
{
    syslog(LOG_INFO, "EXT : Populating storage callbacks");

    storage_callbacks = *opts;
    return 0;
}


#ifdef __cplusplus
}
#endif
