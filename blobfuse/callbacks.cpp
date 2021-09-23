#include <blobfuse.h>
#include <dlfcn.h>

extern struct configParams config_options;
void *extHandle = NULL;

typedef void* (*callback_exchanger)(struct fuse_operations *opts);

void set_up_blobfuse_callbacks(struct fuse_operations &azs_blob_operations)
{
    // Here, we set up all the callbacks that FUSE requires.
    azs_blob_operations.init = azs_init;
    azs_blob_operations.getattr = azs_getattr;
    azs_blob_operations.statfs = azs_statfs;
    azs_blob_operations.access = azs_access;
    azs_blob_operations.readlink = azs_readlink;
    azs_blob_operations.symlink = azs_symlink;
    azs_blob_operations.readdir = azs_readdir;
    azs_blob_operations.open = azs_open;
    azs_blob_operations.read = azs_read;
    azs_blob_operations.release = azs_release;
    azs_blob_operations.fsync = azs_fsync;
    azs_blob_operations.fsyncdir = azs_fsyncdir;
    azs_blob_operations.create = azs_create;
    azs_blob_operations.write = azs_write;
    azs_blob_operations.mkdir = azs_mkdir;
    azs_blob_operations.unlink = azs_unlink;
    azs_blob_operations.rmdir = azs_rmdir;
    azs_blob_operations.chown = azs_chown;
    azs_blob_operations.chmod = azs_chmod;
    //#ifdef HAVE_UTIMENSAT
    azs_blob_operations.utimens = azs_utimens;
    //#endif
    azs_blob_operations.destroy = azs_destroy;
    azs_blob_operations.truncate = azs_truncate;
    azs_blob_operations.rename = azs_rename;
    azs_blob_operations.setxattr = azs_setxattr;
    azs_blob_operations.getxattr = azs_getxattr;
    azs_blob_operations.listxattr = azs_listxattr;
    azs_blob_operations.removexattr = azs_removexattr;
    azs_blob_operations.flush = azs_flush;
}

void set_up_extension_callbacks(struct fuse_operations &azs_blob_operations)
{
    syslog(LOG_INFO, "Going for extension registeration");
    
    // Load the configured library here
    extHandle = dlopen (config_options.extensionLib.c_str(), RTLD_LAZY);
    if (extHandle == NULL) {
        syslog(LOG_ERR, "Failed to open extension library (%d)", errno);
        exit(1);
    }

    syslog(LOG_INFO, "Address of init %p, destroy %p", azs_init, azs_destroy);

    callback_exchanger fuse_callbacks = NULL;
    callback_exchanger storage_callbacks = NULL;

    // Get callback table exchange apis from the extesion library
    fuse_callbacks = (callback_exchanger)dlsym(extHandle, "populateFuseCallbacks");
    storage_callbacks = (callback_exchanger)dlsym(extHandle, "populateStorageCallbacks");

    if (fuse_callbacks == NULL || storage_callbacks == NULL) {
        syslog(LOG_ERR, "Loaded lib does not honour callback contracts (%d)", errno);
        exit(1);
    }

    // Create a local structure and populate it with blobfuse callbacks
    // This we need to pass on to the extension so that it can call our functions
    struct fuse_operations blobfuse_callbacks;
    set_up_blobfuse_callbacks(blobfuse_callbacks);
    if (0 != storage_callbacks(&blobfuse_callbacks)) {
        syslog(LOG_ERR, "Failed to register storage callbacks to extension (%d)", errno);
        exit(1);
    }

    // Get the function pointers from the lib and store them in given structure
    // Once we register these methods to libfuse, calls will directly land into extension
    struct fuse_operations extension_callbacks;
    if (0 != fuse_callbacks(&extension_callbacks)) {
        syslog(LOG_ERR, "Failed to retreive fuse callbacks from extension (%d)", errno);
        exit(1);
    }

    // We have registered our methods to lib and have received fuse handlers from the lib
    // Now register the fuse handlers of extension lib to fuse.
    azs_blob_operations = extension_callbacks;
}


void set_up_callbacks(struct fuse_operations &azs_blob_operations)
{
    if (config_options.extensionLib == "") {
        set_up_blobfuse_callbacks(azs_blob_operations);
    } else {
        set_up_extension_callbacks(azs_blob_operations);
    }
}