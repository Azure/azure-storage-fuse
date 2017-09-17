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
#include <fstream>
#include <map>
#include <memory>
#define FUSE_USE_VERSION 29

#include <fuse.h>
#include <stddef.h>
#include "blob/blob_client.h"

#define AZS_PRINT 1

using namespace microsoft_azure::storage;

struct fhwrapper {
    int fh;
    bool upload;
    fhwrapper(int fh, bool upload) : fh(fh), upload(upload)
    {

    }
};



struct str_options {
    std::string accountName;
    std::string accountKey;
    std::string containerName;
    std::string tmpPath;
};

extern struct str_options str_options;




extern std::shared_ptr<blob_client_wrapper> azure_blob_client_wrapper;

extern std::map<int, int> error_mapping;

extern const std::string directorySignifier;

int map_errno(int error);
std::string prepend_mnt_path_string(const std::string path);
void ensure_files_directory_exists(const std::string file_path);
std::vector<list_blobs_hierarchical_item> list_all_blobs_hierarchical(std::string container, std::string delimiter, std::string prefix);
bool list_one_blob_hierarchical(std::string container, std::string delimiter, std::string prefix);
int is_directory_empty(std::string container, std::string delimiter, std::string prefix);

int azs_access(const char *path, int mask);

/**
 * get_attr is the general-purpose "get information about the thing at this path"
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
 * @param  fi    May be NULL.  If not NULL, information about this item.
 * @return       TODO: Error codes
 */
int azs_getattr(const char *path, struct stat *stbuf);

int azs_readlink(const char *path, char *buf, size_t size);

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
 * @param  flags  Not used.  TODO: Consider prefetching on FUSE_READDIR_PLUS.
 * @return        TODO: error codes.
 */
int azs_readdir(const char *path, void *buf, fuse_fill_dir_t filler, off_t offset, struct fuse_file_info *fi);


/**
 * Open an item (a file) for writing or reading.
 * At the moment, we cache the file locally in its entirety to the SSD before uplaoding to blob storage.
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

int azs_fsync(const char *path, int isdatasync, struct fuse_file_info *fi);

int azs_flush(const char *path, struct fuse_file_info *fi);

/**
 * Release / close the file
 *
 * For reading, this is mostly a no-op.
 * For writing, this is where we actually upload the file to Azure Storage.
 *
 * @param  path Path to the file to release.
 * @param  fi   File info, containing the fh pointer.  Data malloc'd in open/create and stored in fh should probably be free'd here.
 * @return      TODO: Error codes.
 */
int azs_release(const char *path, struct fuse_file_info * fi);

int azs_unlink(const char *path);

int azs_rmdir(const char *path);

int azs_chown(const char *path, uid_t uid, gid_t gid);
int azs_chmod(const char *path, mode_t mode);
int azs_utimens(const char *path, const struct timespec ts[2]);

void azs_destroy(void *private_data);

int azs_truncate(const char *path, off_t off);
int azs_rename(const char *src, const char *dst);
int azs_setxattr(const char *path, const char *name, const char *value, size_t size, int flags);
int azs_getxattr(const char *path, const char *name, char *value, size_t size);
int azs_listxattr(const char *path, char *list, size_t size);
int azs_removexattr(const char *path, const char *name);

void *azs_init(struct fuse_conn_info * conn);


#endif
