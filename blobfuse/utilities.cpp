#include "blobfuse.h"
#include <FileLockMap.h>
#include <sys/file.h>
#include <BlobfuseGlobals.h>

#include <include/StorageBfsClientBase.h>
extern std::shared_ptr<StorageBfsClientBase> storage_client;

int azs_rename_directory(const char *src, const char *dst);

int map_errno(int error)
{
    auto mapping = error_mapping.find(error);
    if (mapping == error_mapping.end())
    {
        syslog(LOG_INFO, "Failed to map storage error code %d to a proper errno.  Returning EIO = %d instead.\n", error, EIO);
        return EIO;
    }
    else
    {
        return mapping->second;
    }
}

std::string prepend_mnt_path_string(const std::string &path)
{
    std::string result;
    result.reserve(config_options.tmpPath.length() + 5 + path.length());
    return result.append(config_options.tmpPath).append("/root").append(path);
}

// Acquire shared lock utility function
int shared_lock_file(int flags, int fd)
{
    if ((flags & O_NONBLOCK) == O_NONBLOCK)
    {
        if (0 != flock(fd, LOCK_SH | LOCK_NB))
        {
            int flockerrno = errno;
            if (flockerrno == EWOULDBLOCK)
            {
                AZS_DEBUGLOGV("Failure to acquire flock due to EWOULDBLOCK.  fd = %d.", fd);
            }
            else
            {
                syslog(LOG_ERR, "Failure to acquire flock for fd = %d.  errno = %d", fd, flockerrno);
            }
            close(fd);
            return 0 - flockerrno;
        }
    }
    else
    {
        if (0 != flock(fd, LOCK_SH))
        {
            int flockerrno = errno;
            syslog(LOG_ERR, "Failure to acquire flock for fd = %d.  errno = %d", fd, flockerrno);
            close(fd);
            return 0 - flockerrno;
        }
    }

    return 0;
}

bool is_directory_blob(unsigned long long size, std::vector<std::pair<std::string, std::string>> metadata)
{
    if (size == 0)
    {
        for (auto iter = metadata.begin(); iter != metadata.end(); ++iter)
        {
            if ((iter->first.compare("hdi_isfolder") == 0) && (iter->second.compare("true") == 0))
            {
                return true;
            }
        }
    }
    return false;
}

int ensure_files_directory_exists_in_cache(const std::string &file_path)
{
    char *pp;
    char *slash;
    int status;
    char *copypath = strdup(file_path.c_str());

    status = 0;
    errno = 0;
    pp = copypath;
    while (status == 0 && (slash = strchr(pp, '/')) != 0)
    {
        if (slash != pp)
        {
            *slash = '\0';
            AZS_DEBUGLOGV("Making cache directory %s.\n", copypath);
            struct stat st;
            if (stat(copypath, &st) != 0)
            {
                status = mkdir(copypath, config_options.defaultPermission);
            }

            // Ignore if some other thread was successful creating the path
            if (errno == EEXIST)
            {
                status = 0;
                errno = 0;
            }

            *slash = '/';
        }
        pp = slash + 1;
    }
    free(copypath);
    return status;
}

int azs_getattr(const char *path, struct stat *stbuf)
{
    AZS_DEBUGLOGV("azs_getattr called with path = %s\n", path);

    // If we're at the root, we know it's a directory
    if (strlen(path) == 1)
    {
        stbuf->st_mode = S_IFDIR | config_options.defaultPermission; // TODO: proper access control.
        stbuf->st_uid = fuse_get_context()->uid;
        stbuf->st_gid = fuse_get_context()->gid;
        stbuf->st_nlink = 2; // Directories should have a hard-link count of 2 + (# child directories).  We don't have that count, though, so we just use 2 for now.  TODO: Evaluate if we could keep this accurate or not.
        stbuf->st_size = 4096;
        stbuf->st_mtime = globalTimes.lastModifiedTime;
        stbuf->st_atime = globalTimes.lastAccessTime;
        stbuf->st_ctime = globalTimes.lastChangeTime;
        return 0;
    }

    // Check and see if the file/directory exists locally (because it's being buffered.)  If so, skip the call to Storage.
    std::string pathString(path);
    
    // Replace '\' with '/' as for azure storage they will be considered as path seperators
    std::replace(pathString.begin(), pathString.end(), '\\', '/');

    std::string mntPathString = prepend_mnt_path_string(pathString);

    // Ensure that we don't get attributes while the file is in an intermediate state.
    //std::shared_ptr<std::mutex> fmutex = file_lock_map::get_instance()->get_mutex(pathString.c_str());
    //std::lock_guard<std::mutex> lock(*fmutex);

    int res;
    int acc = access(mntPathString.c_str(), F_OK);
    if (acc != -1)
    {
        AZS_DEBUGLOGV("Accessing mntPath = %s for getattr succeeded; object is in the local cache.\n", mntPathString.c_str());
        //(void) fi;
        res = lstat(mntPathString.c_str(), stbuf);
        if (res == -1)
        {
            int lstaterrno = errno;
            syslog(LOG_ERR, "lstat on file %s in local cache during getattr failed with errno = %d.\n", mntPathString.c_str(), lstaterrno);
            return -lstaterrno;
        }
        else
        {
            AZS_DEBUGLOGV("lstat on file %s in local cache succeeded.\n", mntPathString.c_str());
            return 0;
        }
    }
    else
    {
        AZS_DEBUGLOGV("Object %s is not in the local cache during getattr.\n", mntPathString.c_str());
    }

    // It's not in the local cache.  Check to see if it's a blob on the service:
    std::string blobNameStr(pathString.substr(1).c_str());
    if (blobNameStr == ".Trash" ||
        blobNameStr == ".Trash-1000" ||
        blobNameStr == ".xdg-volume-info" ||
        blobNameStr == "autorun.inf") {
        syslog(LOG_DEBUG, "Ignoring %s in getattr", blobNameStr.c_str());
        return -(ENOENT);
    }

    errno = 0;
    //AZS_DEBUGLOGV("Storage client name is %s \n", (typeid(storage_client).name()));
    // see if it is block blob and call the block blob method
    //if the first task is to study
    if (!storage_client->isADLS())
    {
        if (config_options.useAttrCache)
        {
            // If attr-cache is enable then instead of calling list 
            // get the attributes from cache for file. for dir we will still
            // rely on list apis.
            BfsFileProperty file_property = storage_client->GetFileProperties(blobNameStr, true);
            if (file_property.isValid() && file_property.exists())
            {
                if (file_property.is_symlink || 
                    is_symlink_blob(file_property.metadata))
                {
                    stbuf->st_mode = S_IFLNK | config_options.defaultPermission;
                }
                else
                {
                    stbuf->st_mode = S_IFREG | config_options.defaultPermission; 
                }
                stbuf->st_uid = fuse_get_context()->uid;
                stbuf->st_gid = fuse_get_context()->gid;
                stbuf->st_atime = stbuf->st_ctime = stbuf->st_mtime = file_property.get_last_modified();
                stbuf->st_nlink = 1;
                stbuf->st_size = file_property.get_size();

                AZS_DEBUGLOGV("File Prop Cache : size is %llu ", file_property.get_size());
                return 0;
            }
        }

        int resultCount = 2;
        bool success = false;
        int failcount = 0;
        list_segmented_response response;
        list_segmented_item blobItem;
        do
        {
            response.reset();
            storage_client->List("", blobNameStr, "/", response, resultCount);
            
            if (errno == 404 || 
                (errno == 0  && response.m_items.size() == 0))
            {
                syslog(LOG_WARNING, "File does not currently exist on the storage or cache, errno : %d", errno);
                response.reset();
                return -(ENOENT);
            }

            if (errno != 0)
            {
                syslog(LOG_WARNING, "Failed to get info on %s, errno : %d",
                    blobNameStr.c_str(), errno);
                success = false;
                failcount++;
                continue; 
            }

            success = true;
            unsigned int dirSize = 0;
            for (unsigned int i = 0; i < response.m_items.size(); i++)
            {
                AZS_DEBUGLOGV("In azs_getattr list_segmented_item %d file %s\n", i, response.m_items[i].name.c_str());

                // if the path for exact name is found the dirSize will be 1 here so check to see if it has files or subdirectories inside
                // match dir name or longer paths to determine dirSize
                if (response.m_items[i].name.compare(blobNameStr + '/') < 0)
                {
                    dirSize++;
                    // listing is hierarchical so no need of the 2nd is blobitem.name empty condition but just in case for service errors
                    if (dirSize > 2 && !blobItem.name.empty())
                    {
                        break;
                    }
                }

                // the below will be skipped blobItem has been found already because we only need the exact match
                // find the element with the exact prefix
                // this could lead to a bug when there is a file with the same name as the directory in the parent directory. In short, that won't work.
                if (blobItem.name.empty() && (response.m_items[i].name == blobNameStr || 
                                                response.m_items[i].name == (blobNameStr + '/')))
                {
                    blobItem = response.m_items[i];
                    AZS_DEBUGLOGV("In azs_getattr found blob in list file %s\n", blobItem.name.c_str());
                    // leave 'i' at the value it is, it will be used in the remaining batches and loops to check for directory empty check.
                    if (dirSize == 0 && (is_directory_blob(0, blobItem.metadata) || blobItem.is_directory || blobItem.name == (blobNameStr + '/')))
                    {
                        dirSize = 1; // root directory exists so 1
                    }
                }
            }

            if (!blobItem.name.empty()) 
            {
                if (!blobItem.last_modified.empty()) {
                    struct tm mtime;
                    char *ptr = strptime(blobItem.last_modified.c_str(), "%a, %d %b %Y %H:%M:%S", &mtime);
                    if (ptr) {
                        stbuf->st_mtime = timegm(&mtime);
                        stbuf->st_atime = stbuf->st_ctime = stbuf->st_mtime;
                    }
                }

                stbuf->st_uid = fuse_get_context()->uid;
                stbuf->st_gid = fuse_get_context()->gid;

                if (blobItem.is_directory || is_directory_blob(0, blobItem.metadata))
                {
                    //AZS_DEBUGLOGV("%s is a directory, blob name is %s\n", mntPathString.c_str(), blobItem.name.c_str());
                    AZS_DEBUGLOGV("Blob %s, representing a directory, found during get_attr.\n", path);
                    stbuf->st_mode = S_IFDIR | config_options.defaultPermission;
                    // If st_nlink = 2, means directory is empty.
                    // Directory size will affect behaviour for mv, rmdir, cp etc.
                    // assign directory status as empty or non-empty based on the value from above
                    stbuf->st_nlink = dirSize > 1 ? 3 : 2;
                    stbuf->st_size = 4096;
                    response.reset();
                    return 0;
                }
                else
                {
                    //AZS_DEBUGLOGV("%s is a file, blob name is %s\n", mntPathString.c_str(), blobItem.name.c_str());
                    AZS_DEBUGLOGV("Blob %s, representing a file, found during get_attr.\n", path);

                    mode_t perms = config_options.defaultPermission;
                    if (is_symlink_blob(blobItem.metadata)) {
                        stbuf->st_mode = S_IFLNK | perms;
                    } else {
                        stbuf->st_mode = S_IFREG | perms; // Regular file (not a directory)
                    }
                    stbuf->st_size = blobItem.content_length;
                    stbuf->st_nlink = 1;
                    response.reset();
                    return 0;
                }
            }
        } while((!success) && (failcount < 20));

        if (errno > 0)
        {
            int storage_errno = errno;
            AZS_DEBUGLOGV("Failure when attempting to determine if %s exists on the service.  errno = %d.\n", blobNameStr.c_str(), storage_errno);
            syslog(LOG_ERR, "Failure when attempting to determine if %s exists on the service.  errno = %d.\n", blobNameStr.c_str(), storage_errno);
            response.reset();
            return 0 - map_errno(storage_errno);
        }
        else // it is a new blob
        {
            AZS_DEBUGLOGV("%s not returned in list_segmented_blobs. It is a new blob", blobNameStr.c_str());
            response.reset();
            return -(ENOENT);
        }
    } // end of processing for Blockblob
    else
    {
        BfsFileProperty blob_property = storage_client->GetProperties(blobNameStr);
        mode_t perms = blob_property.m_file_mode == 0 ? config_options.defaultPermission : blob_property.m_file_mode;

        if ((errno == 0) && blob_property.isValid() && blob_property.exists())
        {
            if (blob_property.is_directory)
            {
                AZS_DEBUGLOGV("Blob %s, representing a directory, found during get_attr.\n", path);
                stbuf->st_mode = S_IFDIR | perms;
                // If st_nlink = 2, means directory is empty.
                // Directory size will affect behaviour for mv, rmdir, cp etc.
                stbuf->st_uid = fuse_get_context()->uid;
                stbuf->st_gid = fuse_get_context()->gid;
                stbuf->st_nlink = storage_client->IsDirectoryEmpty(blobNameStr.c_str()) == D_EMPTY ? 2 : 3;
                stbuf->st_size = 4096;
                stbuf->st_mtime = blob_property.get_last_modified();
                stbuf->st_atime = blob_property.get_last_access();
                stbuf->st_ctime = blob_property.get_last_change();
                return 0;
            }

            AZS_DEBUGLOGV("Blob %s, representing a file, found during get_attr.\n", path);
            if (blob_property.is_symlink ||
                is_symlink_blob(blob_property.metadata))
            {
                stbuf->st_mode = S_IFLNK | perms;
            }
            else
            {
                stbuf->st_mode = S_IFREG | perms; // Regular file (not a directory)
            }
            stbuf->st_uid = fuse_get_context()->uid;
            stbuf->st_gid = fuse_get_context()->gid;
            stbuf->st_mtime = blob_property.get_last_modified();
            stbuf->st_atime = blob_property.get_last_access();
            stbuf->st_ctime = blob_property.get_last_change();
            stbuf->st_nlink = 1;
            stbuf->st_size = blob_property.get_size();
            return 0;
        }
        else if (errno == 0 && !blob_property.isValid() && blob_property.exists())
        {
            // Check to see if it's a directory, instead of a file
            errno = 0;
            int dirSize = is_directory_blob(blob_property.get_size(), blob_property.metadata);
            if (errno != 0)
            {
                int storage_errno = errno;
                syslog(LOG_ERR, "Failure when attempting to determine if directory %s exists on the service.  errno = %d.\n", blobNameStr.c_str(), storage_errno);
                return 0 - map_errno(storage_errno);
            }
            if (dirSize != D_NOTEXIST)
            {
                AZS_DEBUGLOGV("Directory %s found on the service.\n", blobNameStr.c_str());
                stbuf->st_mode = S_IFDIR | config_options.defaultPermission;
                // If st_nlink = 2, means direcotry is empty.
                // Directory size will affect behaviour for mv, rmdir, cp etc.
                stbuf->st_uid = fuse_get_context()->uid;
                stbuf->st_gid = fuse_get_context()->gid;
                stbuf->st_nlink = dirSize == D_EMPTY ? 2 : 3;
                stbuf->st_size = 4096;
                stbuf->st_mtime = time(NULL);
                return 0;
            }
            else
            {
                AZS_DEBUGLOGV("Entity %s does not exist.  Returning ENOENT (%d) from get_attr.\n", path, ENOENT);
                return -(ENOENT);
            }
        }
        else
        {
            if (errno == 404 || !blob_property.exists())
            {
                // The file does not currently exist on the service or in the cache
                // If the command they are calling is just checking for existence, fuse will call the next operation
                // dependent on this error number. If the command cannot continue without the existence it will print out
                // the correct error to the user.
                syslog(LOG_WARNING, "File does not currently exist on the storage or cache");
                return -(ENOENT);
            }
            // If we received a different error, then let's fail with that error
            int storage_errno = errno;
            AZS_DEBUGLOGV("Failure when attempting to determine if %s exists on the service.  errno = %d.\n", blobNameStr.c_str(), storage_errno);
            syslog(LOG_ERR, "Failure when attempting to determine if %s exists on the service.  errno = %d.\n", blobNameStr.c_str(), storage_errno);
            return 0 - map_errno(storage_errno);
        }
    }
}

// Helper method for FTW to remove an entire directory & it's contents.
int rm(const char *fpath, const struct stat * /*sb*/, int tflag, struct FTW * /*ftwbuf*/)
{
    if (tflag == FTW_DP)
    {
        errno = 0;
        int ret = rmdir(fpath);
        return ret;
    }
    else
    {
        errno = 0;
        int ret = unlink(fpath);
        return ret;
    }
}

// Delete the entire contents of tmpPath.
void azs_destroy(void * /*private_data*/)
{
    AZS_DEBUGLOG("azs_destroy called.\n");
    std::string rootPath(config_options.tmpPath + "/root");

    errno = 0;
    // FTW_DEPTH instructs FTW to do a post-order traversal (children of a directory before the actual directory.)
    nftw(rootPath.c_str(), rm, 20, FTW_DEPTH);
}

// Not yet implemented section:
int azs_access(const char * /*path*/, int /*mask*/)
{
    return 0; // permit all access
}

int azs_fsync(const char * /*path*/, int /*isdatasync*/, struct fuse_file_info * /*fi*/)
{
    return 0; // Skip for now
}

int azs_chown(const char * /*path*/, uid_t /*uid*/, gid_t /*gid*/)
{
    return -ENOSYS;
}

int azs_chmod(const char *path, mode_t mode)
{
    //This is only functional when --use-adls is enabled as a mount flag
    AZS_DEBUGLOGV("azs_chmod called with path = %s, mode = %o.\n", path, mode);
    
    std::string pathString(path);
    std::replace(pathString.begin(), pathString.end(), '\\', '/');
    
    errno = 0;
    int ret = storage_client->ChangeMode(pathString.c_str(), mode);
    if (ret)
    {
        AZS_DEBUGLOGV("azs_chmod failed for path = %s, mode = %o.\n", path, mode);
        return ret;
    }

    return 0;
}

//#ifdef HAVE_UTIMENSAT
int azs_utimens(const char * /*path*/, const struct timespec[2] /*ts[2]*/)
{
    //TODO: Implement
    //    return -ENOSYS;
    return 0;
}
//  #endif

// TODO: Fix bug where the files and directories in the source in the file cache are not deleted.
// TODO: Fix bugs where the a file has been created but not yet uploaded.
// TODO: Fix the bug where this fails for multi-level dirrectories.
// TODO: If/when we upgrade to FUSE 3.0, we will need to worry about the additional possible flags (RENAME_EXCHANGE and RENAME_NOREPLACE)
int azs_rename(const char *src, const char *dst)
{
    AZS_DEBUGLOGV("azs_rename called with src = %s, dst = %s.\n", src, dst);

    std::string fromStr(src);
    std::replace(fromStr.begin(), fromStr.end(), '\\', '/');
     std::string toStr(dst);
    std::replace(toStr.begin(), toStr.end(), '\\', '/');

    errno = 0;
    struct stat stbuf;
    errno = azs_getattr(fromStr.c_str(), &stbuf);
    if (errno != 0)
        return errno;

    std::vector<std::string> to_remove;
    if (storage_client->isADLS()) {
        to_remove = storage_client->Rename(fromStr.c_str(), toStr.c_str());
    } else {
        if (stbuf.st_mode & S_IFDIR) {
            // Rename a directory
            to_remove = storage_client->Rename(fromStr.c_str(), toStr.c_str(), true);
        } else {
            // Rename a file
            to_remove = storage_client->Rename(fromStr.c_str(), toStr.c_str(), false);
        }
    }

    for (unsigned int i = 0; i < to_remove.size(); i++)
    {
        struct stat buf;
        if (0 == stat(to_remove.at(i).c_str(), &buf))
            g_gc_cache->addCacheBytes(fromStr.c_str(), buf.st_size);

        g_gc_cache->uncache_file(to_remove.at(i));
    }

    return 0;
}

int azs_setxattr(const char * /*path*/, const char * /*name*/, const char * /*value*/, size_t /*size*/, int /*flags*/)
{
    return -ENOSYS;
}
int azs_getxattr(const char * /*path*/, const char * /*name*/, char * /*value*/, size_t /*size*/)
{
    return -ENOSYS;
}
int azs_listxattr(const char * /*path*/, char * /*list*/, size_t /*size*/)
{
    return -ENOSYS;
}
int azs_removexattr(const char * /*path*/, const char * /*name*/)
{
    return -ENOSYS;
}

bool is_symlink_blob(std::vector<std::pair<std::string, std::string>> metadata)
{
    for (auto iter = metadata.begin(); iter != metadata.end(); ++iter)
    {
        if ((iter->first.compare("is_symlink") == 0) && (iter->second.compare("true") == 0))
        {
            return true;
        }
    }
    return false;
}
