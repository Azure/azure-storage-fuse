#include <blobfuse.h>
#include <dlfcn.h>

#ifdef __DYNAMIC_LOAD_EXT__
void *extHandle = NULL;
typedef void* (*callback_exchanger)(struct fuse_operations *opts);
#else
#include <galactus.h>
#endif

extern struct configParams config_options;

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
    struct fuse_operations fuse_callbacks;
    struct fuse_operations storage_callbacks;

    memset(&fuse_callbacks, 0, sizeof(struct fuse_operations));
    memset(&storage_callbacks, 0, sizeof(struct fuse_operations));

    #ifdef __DYNAMIC_LOAD_EXT__    
    // Load the configured library here
    extHandle = dlopen (config_options.extensionLib.c_str(), RTLD_LAZY);
    if (extHandle == NULL) {
        syslog(LOG_ERR, "Failed to open extension library (%d)", errno);
        fprintf(stderr, "Failed to open extension library %s, errno (%d)\n", config_options.extensionLib.c_str(), errno);
        exit(1);
    }

    syslog(LOG_INFO, "Address of init %p, destroy %p", azs_init, azs_destroy);
    
    // Get the function pointers from the lib and store them in given structure
    // Once we register these methods to libfuse, calls will directly land into extension
    callback_exchanger fuse_regsiter_func = NULL;
    callback_exchanger storage_regsiter_func = NULL;

    fuse_regsiter_func = (callback_exchanger)dlsym(extHandle, "populateFuseCallbacks");
    storage_regsiter_func = (callback_exchanger)dlsym(extHandle, "populateStorageCallbacks");
    
    // Validate lib has legit functions exposed with this name
    if (fuse_regsiter_func == NULL || storage_regsiter_func == NULL) {
        syslog(LOG_ERR, "Loaded lib does not honour callback contracts (%d)", errno);
        fprintf(stderr, "Loaded lib does not honour callback contracts, lib (%s) errno (%d)\n", config_options.extensionLib.c_str(), errno);
        exit(1);
    }

    // Use the fuse registeration function to get callback table from extension
    // This table will later be registered to libfuse so that kernel callbacks land directly in extension    
    if (0 != fuse_regsiter_func(&fuse_callbacks)) {
        syslog(LOG_ERR, "Failed to retreive fuse callbacks from extension (%d)", errno);
        fprintf(stderr, "Failed to retreive fuse callbacks from extension (%d)", errno);
        exit(1);
    }

    // Populate blobfuse callbacks and register them to extension
    // This helps extension to make a call back to blobfuse to connect to storage
    set_up_blobfuse_callbacks(storage_callbacks);
    if (0 != storage_regsiter_func(&storage_callbacks)) {
        syslog(LOG_ERR, "Failed to register fuse callbacks to extension (%d)", errno);
        fprintf(stderr, "Failed to register fuse callbacks to extension (%d)", errno);
        exit(1);
    }
    #else
    // Get extension callbacks to be registered to fuse
    populateFuseCallbacks(&fuse_callbacks);

    // Supply our callbacks to extension so that it can interact us back
    set_up_blobfuse_callbacks(storage_callbacks);
    populateStorageCallbacks(&storage_callbacks);

    // VB : Test Code Below to call init of galactus and get a callback to azs_init
    //struct fuse_conn_info test_conn;
    //fuse_callbacks.init(&test_conn);
    #endif

    // We have registered our methods to lib and have received fuse handlers from the lib
    // Now register the fuse handlers of extension lib to fuse.
    azs_blob_operations = fuse_callbacks;
}


void set_up_callbacks(struct fuse_operations &azs_blob_operations)
{
    if (config_options.extensionLib == "") {
        set_up_blobfuse_callbacks(azs_blob_operations);
    } else {
        set_up_extension_callbacks(azs_blob_operations);
    }
}