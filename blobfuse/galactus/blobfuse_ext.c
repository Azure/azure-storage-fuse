#include <stdio.h>
#include <stdlib.h>
#include <syslog.h>
#include "blobfuse_ext.h"

int populateFuseCallbacks(struct fuse_operations *opts)
{
    syslog(LOG_INFO, "EXT : Populating fuse callbacks");
    opts->init           = ext_init;
    opts->destroy        = ext_destroy;
    
    opts->statfs         = ext_statfs;
    opts->getattr        = ext_getattr;

    opts->readdir        = ext_readdir;
    opts->mkdir          = ext_mkdir;
    opts->rmdir          = ext_rmdir;

    return 0;
}

int populateStorageCallbacks(struct fuse_operations *opts)
{
    syslog(LOG_INFO, "EXT : Populating storage callbacks");

    // Allocate memory to hold storage endpoint callbacks
    storage_callbacks = (struct fuse_operations*)malloc(sizeof(struct fuse_operations));

    // Copy populated storage callbacks to our local structure for later references
    *storage_callbacks = *opts;

    syslog(LOG_INFO, "EXT : Callbacks stored in struct %p", storage_callbacks);

    return 0;
}
