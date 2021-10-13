#include <stdio.h>
#include <stdlib.h>
#include <syslog.h>
#include <string.h>

#include "galactus.h"
#include "callback_handlers.h"

#ifdef __cplusplus
extern "C" {
#endif

int signature_verified = 0;
const char* launcher_call_sign = "ola-amigo!!";
const char* my_call_sign = "ola-amigo!!!";

const char* validate_signature(const char* sign)
{
    if (strcmp(sign, launcher_call_sign) == 0) {
        syslog(LOG_INFO, "EXT : Launcher signature verified");
        signature_verified = 1;
        return my_call_sign;
    }

    return "adios!!";
}

int init_extension(const char* conf_file)
{
    syslog(LOG_INFO, "EXT : Received config file %s", conf_file);
    return 0;
}

int register_fuse_callbacks(struct fuse_operations *opts)
{
    syslog(LOG_INFO, "EXT : Populating fuse callbacks");

    if (!signature_verified) {
        syslog(LOG_ERR, "EXT : Not a friendly neighbour.");
        return -1;
    }

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
int register_storage_callbacks(struct fuse_operations *opts)
{
    syslog(LOG_INFO, "EXT : Populating storage callbacks");
    
    if (!signature_verified) {
        syslog(LOG_ERR, "EXT : Not a friendly neighbour.");
        return -1;
    }

    storage_callbacks = *opts;
    return 0;
}


#ifdef __cplusplus
}
#endif
