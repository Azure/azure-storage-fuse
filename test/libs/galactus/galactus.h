
#ifndef __GALACTUS_H__
#define __GALACTUS_H__
// This is the extension contract to be implemented to create an extension

// Use this command to build a extension .so : "gcc -shared -o libgalactus.so -D_FILE_OFFSET_BITS=64 -DCMAKE_BUILD_TYPE=Debug -fPIC *.c"
// Use this command to build a static .lib : gcc -Wall -fPIC -D_FILE_OFFSET_BITS=64 -DCMAKE_BUILD_TYPE=Debug -c *.c && ar -cvq libgalactus.a *.o
#include <stddef.h>
#include <stdio.h>


// Declare that we're using version 2.9 of FUSE
// 3.0 is not built-in to many distros yet.
// This line must come before #include <fuse.h>.
// Fuse3 is not supported yet
#define FUSE_USE_VERSION 29
#include <fuse.h>

#ifdef __cplusplus
extern "C" {
#endif

/*  
    ---------------------------------------------------------
        Blobfuse / Blobfuse2 supports only below operations
    ---------------------------------------------------------

    // System level operations
    fuse_operations.init        
    fuse_operations.destroy     

    // FS level operations
    fuse_operations.getattr     
    fuse_operations.statfs      
    
    // Dir level operations
    fuse_operations.opendir
    fuse_operations.releasedir
    fuse_operations.readdir     
    fuse_operations.mkdir       
    fuse_operations.rmdir       

    // File level operations
    fuse_operations.open        
    fuse_operations.create      
    fuse_operations.read        
    fuse_operations.write       
    fuse_operations.flush       
    fuse_operations.truncate    
    fuse_operations.release     
    
    fuse_operations.unlink      
    fuse_operations.rename 

    // Symlink level operations
    fuse_operations.readlink    
    fuse_operations.symlink     
    
    // Sync operations
    fuse_operations.fsync       
    fuse_operations.fsyncdir    

    // Permission operations
    fuse_operations.chmod       
    
*/

// Global variable to hold the storage callback table
struct fuse_operations storage_callbacks;

// Return a well defined string to ensure its a legit blobfuse amigo
const char* validate_signature(const char* sign);

// Call this method to pass on a config file to extension, in case it wants to read anything
int init_extension(const char* conf_file);

// Call this method to populate callbacks to be registered to fuse
int register_fuse_callbacks(struct fuse_operations *opts);

// Call this method to populate callbacks to communicate with blobfuse / blobfuse2
int register_storage_callbacks(struct fuse_operations *opts);

#ifdef __cplusplus
}
#endif

#endif //__GALACTUS_H__