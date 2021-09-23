
#ifndef __BLOBFUSE_EXT_H__
#define __BLOBFUSE_EXT_H__

// BLOBFUSE : This is the extension contract to be implemented to create and extension
// blobfuse_ext.c is a sample code for the same
// Use this command to build blobfuse_ext.c : "gcc -shared -o libgalactus.so -D_FILE_OFFSET_BITS=64 -fPIC blobfuse_ext.c callback_handlers.c "

#include <stddef.h>
#include <stdio.h>


// Declare that we're using version 2.9 of FUSE
// 3.0 is not built-in to many distros yet.
// This line must come before #include <fuse.h>.
#define FUSE_USE_VERSION 29
#include <fuse.h>

/*  
    ------------------------------------------------------
        Blobfuse supports only below operations
    ------------------------------------------------------

    // System level operations
    azs_blob_operations.init        
    azs_blob_operations.destroy     

    // FS level operations
    azs_blob_operations.getattr     
    azs_blob_operations.statfs      
    
    // Dir level operations
    azs_blob_operations.readdir     
    azs_blob_operations.mkdir       
    azs_blob_operations.rmdir       

    // File level operations
    azs_blob_operations.open        
    azs_blob_operations.create      
    azs_blob_operations.read        
    azs_blob_operations.write       
    azs_blob_operations.flush       
    azs_blob_operations.truncate    
    azs_blob_operations.release     
    
    azs_blob_operations.unlink      
    azs_blob_operations.rename 

    // Symlink level operations
    azs_blob_operations.readlink    
    azs_blob_operations.symlink     
    
    // Sync operations
    azs_blob_operations.fsync       
    azs_blob_operations.fsyncdir    

    // Permission operations
    azs_blob_operations.chmod       
    
*/


struct fuse_operations *storage_callbacks;

// Call this method to populate callbacks to be registered to fuse
int populateFuseCallbacks(struct fuse_operations *opts);

// Call this method to populate callbacks to communicate with blobfuse
int populateStorageCallbacks(struct fuse_operations *opts);


// Methods to be defined by the extension. Sample in callback_handlers.c
void *ext_init(struct fuse_conn_info *conn);
void ext_destroy(void *private_data);

int ext_statfs(const char *path, struct statvfs *stbuf);
int ext_getattr(const char *path, struct stat *stbuf);

int ext_readdir(const char *path, void *buf, fuse_fill_dir_t filler, off_t, struct fuse_file_info *);
int ext_mkdir(const char *path, mode_t mode);
int ext_rmdir(const char *path);

#endif