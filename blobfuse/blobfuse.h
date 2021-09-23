#ifndef __AZS_FUSE_DRIVER__
#define __AZS_FUSE_DRIVER__

#include <stdio.h>
#include <stdlib.h>
#include <string>
#include <errno.h>
#include <sys/types.h>
#include <sys/stat.h>
#include <ftw.h>
#include <unistd.h>
#include <time.h>
#include <iostream>
#include <sstream>
#include <fstream>
#include <map>
#include <memory>
#include <dirent.h>
#include <deque>
#include <gnutls/gnutls.h>
#include <gcrypt.h>
#include <pthread.h>


// Declare that we're using version 2.9 of FUSE
// 3.0 is not built-in to many distros yet.
// This line must come before #include <fuse.h>.
#define FUSE_USE_VERSION 29
#include <fuse.h>

#include <stddef.h>
#include <BlobfuseGlobals.h>

#define UNREFERENCED_PARAMETER(p) (p)

// instruct gcrypt to use pthread
GCRY_THREAD_OPTION_PTHREAD_IMPL;

using namespace azure::storage_lite;
using namespace blobfuse_constants;

extern std::shared_ptr<gc_cache> g_gc_cache;
extern struct configParams config_options;
extern float kernel_version;
extern float libcurl_version;

void populate_kernel_version();
void populate_libcurl_version();

// Helper function to map an HTTP error to an errno.
// Should be called on any errno returned from the Azure Storage cpp lite lib.
int map_errno(int error);

// Read Storage connection information from the config file
int read_config(std::string configFile);

// Helper function to prepend the 'tmpPath' to the input path.
// Input is the logical file name being input to the FUSE API, output is the file name of the on-disk file in the file cache.
std::string prepend_mnt_path_string(const std::string& path);

// Helper function to acquire a shared file lock while the file is open
int shared_lock_file(int flags, int fd);

// Helper function to create all directories in the path if they don't already exist.
int ensure_files_directory_exists_in_cache(const std::string& file_path);

// Greedily list all blobs using the input params.
std::vector<std::pair<std::vector<list_blobs_segmented_item>, bool>> list_all_blobs_segmented(const std::string& container, const std::string& delimiter, const std::string& prefix, const std::size_t maxresults=0);

// Returns:
// 0 if there's nothing there (the directory does not exist)
// 1 If there's either the ".directory" blob, or the hdfs-type directory blob
// 2 otherwise (the directory exists and is not empty.)
int is_directory_empty(const std::string& container, const std::string& dir_name);

// Returns true if the input has zero length and the "hdi_isfolder=true" metadata.
bool is_directory_blob(unsigned long long size, std::vector<std::pair<std::string, std::string>> metadata);

/**
 * get_attr is the general-purpose "get information about the file or directory at this path"
 * function called by FUSE.  Most important is to return whether the item is a file or a directory.
 * Similar to stat().
 *
 * Note that this is called many times, so perf here is important.
 *
 * TODO: Minimize calls to Storage
 * TODO: Returned cached information, especially from a prior List call
 *
 * @param  path  The path for which information should be evaluated.
 * @param  stbuf The 'stat' struct containing the output information.
 * @return       TODO: Error codes
 */
int azs_getattr(const char *path, struct stat *stbuf);

/**
* statfs gets the file system statistics for the tmpPath/local cache path
*
* @param  path  The path for which information should be evaluated.
* @param  stbuf The 'stat' struct containing the output information.
* @return       Returns success, or return code from the statvfs call
*/
int azs_statfs(const char *path, struct statvfs *stbuf);

/**
 * Create a directory.  In order to support empty directories, this creates a blob representing the directory.
 *
 * Current operation is, if the directory to be created is /root/a/b, the blob will be /root/a/b/.dir.
 * TODO: Change this to a blob at /root/a/b, where 'b' is empty, but has metadata representing the fact that it's a directory.
 *
 * @param  path Path of the directory to create.
 * @param  mode Permissions to the new directory - currently unimplemented.
 * @return      TODO: error codes
 */
int azs_mkdir(const char *path, mode_t mode);

/**
 * Read the contents of a directory.  For each entry to add, call the filler function with the input buffer,
 * the name of the entry, and additional data about the entry.  TODO: Keep the data (somehow) for latter getattr calls.
 *
 * @param  path   Path to the directory to read.
 * @param  buf    Buffer to pass into the filler function.  Not otherwise used in this function.
 * @param  filler Function to call to add directories and files as they are discovered.
 * @param  offset Not used
 * @param  fi     File info about the directory to be read.
 * @return        TODO: error codes.
 */
int azs_readdir(const char *path, void *buf, fuse_fill_dir_t filler, off_t offset, struct fuse_file_info *fi);


/**
 * Open an item (a file) for writing or reading.
 * At the moment, we cache the file locally in its entirety to the SSD before uploading to blob storage.
 * TODO: Implement in-memory caching.
 * TODO: Implement block-level buffering.
 * TODO: If opening for reading, cache the file locally.
 *
 * @param  path The path to the file to open
 * @param  fi   File info.  Contains the flags to use in open().  May update the fh.
 * @return      TODO: error handling.
 */
int azs_open(const char *path, struct fuse_file_info *fi);

/**
 * Read data from the file (the blob) into the input buffer
 * @param  path   Path of the file (blob) to read from
 * @param  buf    Buffer in which to copy the data
 * @param  size   Amount of data to copy
 * @param  offset Offset in the file (the blob) from which to begin reading.
 * @param  fi     File info for this file.
 * @return        TODO: Error codes
 */
int azs_read(const char *path, char *buf, size_t size, off_t offset, struct fuse_file_info *fi);

/**
 * Create a file.
 * Here we create the file locally, but don't yet make any calls to Storage
 *
 * @param  path Path of the file to create
 * @param  mode File mode, currently unused.
 * @param  fi   Fuse file info - used to set the fh.
 * @return      TODO: error codes.
 */
int azs_create(const char *path, mode_t mode, struct fuse_file_info *fi);

/**
 * Write data to the file.
 *
 * Here, we are still just writing data to the local buffer, not forwarding to Storage.
 * TODO: possible in-memory caching?
 * TODO: for very large files, start uploading to Storage before all the data has been written here.
 * @param  path   Path to the file to write.
 * @param  buf    Buffer containing the data to write.
 * @param  size   Amount of data to write
 * @param  offset Offset in the file to write the data to
 * @param  fi     Fuse file info, containing the fh pointer
 * @return        TODO: Error codes.
 */
int azs_write(const char *path, const char *buf, size_t size, off_t offset, struct fuse_file_info *fi);

/**
 * Upload the opened file to Azure Storage.
 *
 * @param  path Path to the file to flush.
 * @param  fi   File info, containing the fh pointer.  Data malloc'd in open/create and stored in fh should probably be free'd here.
 * @return      TODO: Error codes.
 */
int azs_flush(const char *path, struct fuse_file_info *fi);

/**
 * Release / close the file.
 *
 * There should be no need to upload changes to Storage, just release locks and close the file handle.
 *
 * @param  path Path to the file to release.
 * @param  fi   File info, containing the fh pointer.  Data malloc'd in open/create and stored in fh should probably be free'd here.
 * @return      TODO: Error codes.
 */
int azs_release(const char *path, struct fuse_file_info * fi);

/**
 * Unlink a file
 *
 * Delete the file from the local file cache (if present), and from Azure Storage (if present.)
 *
 * @param  path Path to the file to unlink.
 * @return      TODO: Error codes.
 */
int azs_unlink(const char *path);

/**
 * Remove a directory.
 *
 * Delete the directory locally, and the ".directory" blob in Storage.
 * Fail if the directory is not empty.
 *
 * @param  path Path to the directory to remove.
 * @return      TODO: Error codes.
 */
int azs_rmdir(const char *path);

/**
 * Change the name or location of a file or directory.
 * This method is implemented using server-side blob copy, followed by a deletion of the src.
 * This method is not atomic.
 *
 * @param  src Path to the source file or directory.
 * @param  dst Path to the destination file or directory.
 * @return      TODO: Error codes.
 */
int azs_rename(const char *src, const char *dst);

/**
 * Initialize the filesystem.
 *
 * This is called by FUSE during mount.
 * Allows the adapter to set values in fuse_conn_info, which is not available previously.
 *
 * @param  conn Configuration info of fuse driver.
 * @return      TODO: Error codes.
 */
void* azs_init(struct fuse_conn_info * conn);

/**
 * Un-mount the file system
 *
 * This should delete everything in the tmp directory.
 *
 * @param  private_data Not used
 * @return      TODO: Error codes.
 */
void azs_destroy(void *private_data);

/* Not implemented functions.
 */
int azs_access(const char *path, int mask);
int azs_fsync(const char *path, int isdatasync, struct fuse_file_info *fi);
int azs_fsyncdir(const char *path, int isdatasync, struct fuse_file_info *fi);
int azs_chown(const char *path, uid_t uid, gid_t gid);
int azs_chmod(const char *path, mode_t mode);
int azs_utimens(const char *path, const struct timespec ts[2]);
int azs_truncate(const char *path, off_t off);
int azs_setxattr(const char *path, const char *name, const char *value, size_t size, int flags);

// symlink related handlers
bool is_symlink_blob(std::vector<std::pair<std::string, std::string>> metadata);
int azs_readlink(const char *path, char *buf, size_t size);
int azs_symlink(const char *from, const char *to);

/** Not implemented. */
int azs_getxattr(const char *path, const char *name, char *value, size_t size);

/** Not implemented. */
int azs_listxattr(const char *path, char *list, size_t size);

/** Not implemented. */
int azs_removexattr(const char *path, const char *name);

/** Internal method, used to rename a single file in a (hopefully) lock-safe manner. */
int azs_rename_single_file(const char *src, const char *dst);

/**
* Convert a value into a string.
*/
template<typename T>
std::string to_str(const T& value) {
   std::ostringstream out;
   out << value;
   return out.str();
}

void set_up_callbacks(struct fuse_operations &azs_blob_operations);
#endif
