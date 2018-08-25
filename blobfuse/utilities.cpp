#include "blobfuse.h"
#include <sys/file.h>

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

std::string prepend_mnt_path_string(const std::string& path)
{
    std::string result;
    result.reserve(str_options.tmpPath.length() + 5 + path.length());
    return result.append(str_options.tmpPath).append("/root").append(path);
}

void gc_cache::add_file(std::string path)
{
    file_to_delete file;
    file.path = path;
    file.closed_time = time(NULL); 
    
    // lock before updating deque
    std::lock_guard<std::mutex> lock(m_deque_lock);
    m_cleanup.push_back(file);
}

void gc_cache::run()
{
    std::thread t1(std::bind(&gc_cache::run_gc_cache,this));
    t1.detach();
}

// cleanup function to clean cached files that are too old
void gc_cache::run_gc_cache()
{

    while(true){

        // lock the deque
        file_to_delete file;
        bool is_empty;
        {
            std::lock_guard<std::mutex> lock(m_deque_lock);
            is_empty = m_cleanup.empty();
            if(!is_empty)
            {
                file = m_cleanup.front();
            }
        }

        //if deque is empty, skip
        if(is_empty)
        {
            //run it every 1 second
            usleep(1000);
            continue;
        }

        time_t now = time(NULL);
        //check if the closed time is old enough to delete
        if((now - file.closed_time) > file_cache_timeout_in_seconds)
        {
            AZS_DEBUGLOGV("File %s being considered for deletion by file cache GC.\n", file.path.c_str());

            // path in the temp location
            const char * mntPath;
            std::string mntPathString = prepend_mnt_path_string(file.path);
            mntPath = mntPathString.c_str();

            //check if the file on disk is still too old
            //mutex lock
            auto fmutex = file_lock_map::get_instance()->get_mutex(file.path.c_str());
            std::lock_guard<std::mutex> lock(*fmutex);

            struct stat buf;
            stat(mntPath, &buf);
            if (((now - buf.st_mtime) > file_cache_timeout_in_seconds) && ((now - buf.st_ctime) > file_cache_timeout_in_seconds))
            {
                //clean up the file from cache
                int fd = open(mntPath, O_WRONLY);
                if (fd > 0)
                {
                    int flockres = flock(fd, LOCK_EX|LOCK_NB);
                    if (flockres != 0)
                    {
                        if (errno == EWOULDBLOCK)
                        {
                            // Someone else holds the lock.  In this case, we will postpone updating the cache until the next time open() is called.
                            // TODO: examine the possibility that we can never acquire the lock and refresh the cache.
                            AZS_DEBUGLOGV("Did not clean up file %s from file cache because there's still an open file handle to it.", mntPath);
                        }
                        else
                        {
                            // Failed to acquire the lock for some other reason.  We close the open fd, and continue.
                            syslog(LOG_ERR, "Did not clean up file %s from file cache because we failed to acquire the flock for an unknown reason, errno = %d.\n", mntPath, errno);
                        }
                    }
                    else
                    {
                        AZS_DEBUGLOGV("GC cleanup of cached file %s.\n", mntPath);
                        unlink(mntPath);
                        flock(fd, LOCK_UN);
                    }

                    close(fd);
                }
                else
                {
                    AZS_DEBUGLOGV("Failed to open file %s from file cache in GC, skipping cleanup. errno from open = %d.", mntPath, errno);
                }
            }

            // lock to remove from front
            {
                std::lock_guard<std::mutex> lock(m_deque_lock);
                m_cleanup.pop_front();
            }

        }
        else
        {
            // no file was timed out - let's wait a second
            usleep(1000);
        }
    }

}

// Acquire shared lock utility function
int shared_lock_file(int flags, int fd)
{
    if((flags&O_NONBLOCK) == O_NONBLOCK)
    {
        if(0 != flock(fd, LOCK_SH|LOCK_NB))
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

int ensure_files_directory_exists_in_cache(const std::string& file_path)
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
                status = mkdir(copypath, default_permission);
            }

            // Ignore if some other thread was successful creating the path
	    if(errno == EEXIST)
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

std::vector<list_blobs_hierarchical_item> list_all_blobs_hierarchical(const std::string& container, const std::string& delimiter, const std::string& prefix)
{
    static const int maxFailCount = 20;
    std::vector<list_blobs_hierarchical_item> results;

    std::string continuation;

    std::string prior;
    bool success = false;
    int failcount = 0;
    do
    {
        AZS_DEBUGLOGV("About to call list_blobs_hierarchial.  Container = %s, delimiter = %s, continuation = %s, prefix = %s\n", container.c_str(), delimiter.c_str(), continuation.c_str(), prefix.c_str());

        errno = 0;
        list_blobs_hierarchical_response response = azure_blob_client_wrapper->list_blobs_hierarchical(container, delimiter, continuation, prefix);
        if (errno == 0)
        {
            success = true;
            failcount = 0;
            AZS_DEBUGLOGV("Successful call to list_blobs_hierarchical.  results count = %s, next_marker = %s.\n", to_str(response.blobs.size()).c_str(), response.next_marker.c_str());
            continuation = response.next_marker;
            if(response.blobs.size() > 0)
            {
                auto begin = response.blobs.begin();
                if(response.blobs[0].name == prior)
                {
                    std::advance(begin, 1);
                }
                results.insert(results.end(), begin, response.blobs.end());
                prior = response.blobs.back().name;
            }
        }
        else
        {
            failcount++;
            success = false;
            syslog(LOG_WARNING, "list_blobs_hierarchical failed for the %d time with errno = %d.\n", failcount, errno);

        }
    } while (((continuation.size() > 0) || !success) && (failcount < maxFailCount));

    // errno will be set by list_blobs_hierarchial if the last call failed and we're out of retries.
    return results;
}

/*
 * Check if the direcotry is empty or not by checking if there is any blob with prefix exists in the specified container.
 *
 * return
 *   - D_NOTEXIST if there's nothing there (the directory does not exist)
 *   - D_EMPTY is there's exactly one blob, and it's the ".directory" blob
 *   - D_NOTEMPTY otherwise (the directory exists and is not empty.)
 */
int is_directory_empty(const std::string& container, const std::string& dir_name)
{
    std::string delimiter = "/";
    bool dir_blob_exists = false;
    errno = 0;
    blob_property props = azure_blob_client_wrapper->get_blob_property(container, dir_name);
    if ((errno == 0) && (props.valid()))
    {
        dir_blob_exists = is_directory_blob(props.size, props.metadata);
    }
    if (errno != 0)
    {
        if ((errno != 404) && (errno != ENOENT))
        {
            return -1; // Failure in fetching properties - errno set by blob_exists
        }
    }

    std::string prefix_with_slash = dir_name;
    prefix_with_slash.append(delimiter);
    std::string continuation;
    bool success = false;
    int failcount = 0;
    bool old_dir_blob_found = false;
    do
    {
        errno = 0;
        list_blobs_hierarchical_response response = azure_blob_client_wrapper->list_blobs_hierarchical(container, delimiter, continuation, prefix_with_slash, 2);
        if (errno == 0)
        {
            success = true;
            failcount = 0;
            continuation = response.next_marker;
            if (response.blobs.size() > 1)
            {
                return D_NOTEMPTY;
            }
            if (response.blobs.size() > 0)
            {
                if ((!old_dir_blob_found) &&
                    (!response.blobs[0].is_directory) &&
                    (response.blobs[0].name.size() > former_directory_signifier.size()) &&
                    (0 == response.blobs[0].name.compare(response.blobs[0].name.size() - former_directory_signifier.size(), former_directory_signifier.size(), former_directory_signifier)))
                {
                    old_dir_blob_found = true;
                }
                else
                {
                    return D_NOTEMPTY;
                }
            }
        }
        else
        {
            success = false;
            failcount++; //TODO: use to set errno.
        }
    } while ((continuation.size() > 0 || !success) && failcount < 20);

    if (!success)
    {
    // errno will be set by list_blobs_hierarchial if the last call failed and we're out of retries.
        return -1;
    }

    return old_dir_blob_found || dir_blob_exists ? D_EMPTY : D_NOTEXIST;
}


int azs_getattr(const char *path, struct stat *stbuf)
{
    AZS_DEBUGLOGV("azs_getattr called with path = %s\n", path);
    // If we're at the root, we know it's a directory
    if (strlen(path) == 1)
    {
        stbuf->st_mode = S_IFDIR | default_permission; // TODO: proper access control.
        stbuf->st_uid = fuse_get_context()->uid;
        stbuf->st_gid = fuse_get_context()->gid;
        stbuf->st_nlink = 2; // Directories should have a hard-link count of 2 + (# child directories).  We don't have that count, though, so we just use 2 for now.  TODO: Evaluate if we could keep this accurate or not.
        stbuf->st_size = 4096;
        stbuf->st_mtime = time(NULL);
        return 0;
    }

    // Ensure that we don't get attributes while the file is in an intermediate state.
    auto fmutex = file_lock_map::get_instance()->get_mutex(path);
    std::lock_guard<std::mutex> lock(*fmutex);

    // Check and see if the file/directory exists locally (because it's being buffered.)  If so, skip the call to Storage.
    std::string pathString(path);
    std::string mntPathString = prepend_mnt_path_string(pathString);

    int res;
    int acc = access(mntPathString.c_str(), F_OK);
    if (acc != -1 )
    {
        AZS_DEBUGLOGV("Accessing mntPath = %s for get_attr succeeded; object is in the local cache.\n", mntPathString.c_str());
        //(void) fi;
        res = lstat(mntPathString.c_str(), stbuf);
        if (res == -1)
        {
            int lstaterrno = errno;
            syslog(LOG_ERR, "lstat on file %s in local cache during get_attr failed with errno = %d.\n", mntPathString.c_str(), lstaterrno);
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
        AZS_DEBUGLOGV("Object %s is not in the local cache during get_attr.\n", mntPathString.c_str());
    }

    // It's not in the local cache.  Check to see if it's a blob on the service:
    std::string blobNameStr(&(path[1]));
    errno = 0;
    auto blob_property = azure_blob_client_wrapper->get_blob_property(str_options.containerName, blobNameStr);

    if ((errno == 0) && blob_property.valid())
    {
        if (is_directory_blob(blob_property.size, blob_property.metadata))
        {
            AZS_DEBUGLOGV("Blob %s, representing a directory, found during get_attr.\n", path);
            stbuf->st_mode = S_IFDIR | default_permission;
            // If st_nlink = 2, means direcotry is empty.
            // Directory size will affect behaviour for mv, rmdir, cp etc.
            stbuf->st_uid = fuse_get_context()->uid;
            stbuf->st_gid = fuse_get_context()->gid;
            stbuf->st_nlink = is_directory_empty(str_options.containerName, blobNameStr) == D_EMPTY ? 2 : 3;
            stbuf->st_size = 4096;
            return 0;
        } else if(is_symlink_blob(blob_property.metadata))
	{
	    AZS_DEBUGLOGV("Blob %s, representing a symlink, found during get_attr.\n", path);
            stbuf->st_mode = S_IFLNK | default_permission;
            // If st_nlink = 2, means direcotry is empty.
            // Directory size will affect behaviour for mv, rmdir, cp etc.
            stbuf->st_uid = fuse_get_context()->uid;
            stbuf->st_gid = fuse_get_context()->gid;
            stbuf->st_nlink = 1;
            stbuf->st_size = blob_property.size;
            return 0;
	}

        AZS_DEBUGLOGV("Blob %s, representing a file, found during get_attr.\n", path);
        stbuf->st_mode = S_IFREG | default_permission; // Regular file (not a directory)
        stbuf->st_uid = fuse_get_context()->uid;
        stbuf->st_gid = fuse_get_context()->gid;
        stbuf->st_mtime = blob_property.last_modified;
        stbuf->st_nlink = 1;
        stbuf->st_size = blob_property.size;
        return 0;
    }
    else if (errno == 0 && !blob_property.valid())
    {
        // Check to see if it's a directory, instead of a file

        errno = 0;
        int dirSize = is_directory_empty(str_options.containerName, blobNameStr);
        if (errno != 0)
        {
            int storage_errno = errno;
            syslog(LOG_ERR, "Failure when attempting to determine if directory %s exists on the service.  errno = %d.\n", blobNameStr.c_str(), storage_errno);
            return 0 - map_errno(storage_errno);
        }
        if (dirSize != D_NOTEXIST)
        {
            AZS_DEBUGLOGV("Directory %s found on the service.\n", blobNameStr.c_str());
            stbuf->st_mode = S_IFDIR | default_permission;
            // If st_nlink = 2, means direcotry is empty.
            // Directory size will affect behaviour for mv, rmdir, cp etc.
            stbuf->st_uid = fuse_get_context()->uid;
            stbuf->st_gid = fuse_get_context()->gid;
            stbuf->st_nlink = dirSize == D_EMPTY ? 2 : 3;
            stbuf->st_size = 4096;
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
        int storage_errno = errno;
        syslog(LOG_ERR, "Failure when attempting to determine if %s exists on the service.  errno = %d.\n", blobNameStr.c_str(), storage_errno);
        return 0 - map_errno(storage_errno);
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
    std::string rootPath(str_options.tmpPath + "/root");

    errno = 0;
    // FTW_DEPTH instructs FTW to do a post-order traversal (children of a directory before the actual directory.)
    nftw(rootPath.c_str(), rm, 20, FTW_DEPTH); 
}


// Not yet implemented section:
int azs_access(const char * /*path*/, int /*mask*/)
{
    return 0;  // permit all access
}

int azs_fsync(const char * /*path*/, int /*isdatasync*/, struct fuse_file_info * /*fi*/)
{
    return 0; // Skip for now
}

int azs_chown(const char * /*path*/, uid_t /*uid*/, gid_t /*gid*/)
{
    //TODO: Implement
//    return -ENOSYS;
    return 0;
}

int azs_chmod(const char * /*path*/, mode_t /*mode*/)
{
    //TODO: Implement
//    return -ENOSYS;
    return 0;

}

//#ifdef HAVE_UTIMENSAT
int azs_utimens(const char * /*path*/, const struct timespec [2] /*ts[2]*/)
{
    //TODO: Implement
//    return -ENOSYS;
    return 0;
}
//  #endif

int azs_rename_directory(const char *src, const char *dst)
{
    AZS_DEBUGLOGV("azs_rename_directory called with src = %s, dst = %s.\n", src, dst);
    std::string srcPathStr(src);
    std::string dstPathStr(dst);

    // Rename the directory blob, if it exists.
    errno = 0;
    blob_property props = azure_blob_client_wrapper->get_blob_property(str_options.containerName, srcPathStr.substr(1));
    if ((errno == 0) && (props.valid()))
    {
        if (is_directory_blob(props.size, props.metadata))
        {
            azs_rename_single_file(src, dst);
        }
    }
    if (errno != 0)
    {
        if ((errno != 404) && (errno != ENOENT))
        {
            return 0 - map_errno(errno); // Failure in fetching properties - errno set by blob_exists
        }
    }

    if (srcPathStr.size() > 1)
    {
        srcPathStr.push_back('/');
    }
    if (dstPathStr.size() > 1)
    {
        dstPathStr.push_back('/');
    }
    std::vector<std::string> local_list_results;

    // Rename all files and directories that exist in the local cache.
    ensure_files_directory_exists_in_cache(prepend_mnt_path_string(dstPathStr + "placeholder"));
    std::string mntPathString = prepend_mnt_path_string(srcPathStr);
    DIR *dir_stream = opendir(mntPathString.c_str());
    if (dir_stream != NULL)
    {
        struct dirent* dir_ent = readdir(dir_stream);
        while (dir_ent != NULL)
        {
            if (dir_ent->d_name[0] != '.')
            {
                int nameLen = strlen(dir_ent->d_name);
                char *newSrc = (char *)malloc(sizeof(char) * (srcPathStr.size() + nameLen + 1));
                memcpy(newSrc, srcPathStr.c_str(), srcPathStr.size());
                memcpy(&(newSrc[srcPathStr.size()]), dir_ent->d_name, nameLen);
                newSrc[srcPathStr.size() + nameLen] = '\0';

                char *newDst = (char *)malloc(sizeof(char) * (dstPathStr.size() + nameLen + 1));
                memcpy(newDst, dstPathStr.c_str(), dstPathStr.size());
                memcpy(&(newDst[dstPathStr.size()]), dir_ent->d_name, nameLen);
                newDst[dstPathStr.size() + nameLen] = '\0';

                AZS_DEBUGLOGV("Local object found - about to rename %s to %s.\n", newSrc, newDst);
                if (dir_ent->d_type == DT_DIR)
                {
                    azs_rename_directory(newSrc, newDst);
                }
                else
                {
                    azs_rename_single_file(newSrc, newDst);
                }

                free(newSrc);
                free(newDst);

                std::string dir_str(dir_ent->d_name);
                local_list_results.push_back(dir_str);
            }

            dir_ent = readdir(dir_stream);
        }

        closedir(dir_stream);
    }

    // Rename all files & directories that don't exist in the local cache.
    errno = 0;
    std::vector<list_blobs_hierarchical_item> listResults = list_all_blobs_hierarchical(str_options.containerName, "/", srcPathStr.substr(1));
    if (errno != 0)
    {
        int storage_errno = errno;
        syslog(LOG_ERR, "list blobs operation failed during attempt to rename directory %s to %s.  errno = %d.\n", src, dst, storage_errno);
        return 0 - map_errno(storage_errno);
    }

    AZS_DEBUGLOGV("Total of %s results found from list_blobs call during rename operation\n.", to_str(listResults.size()).c_str());
    for (size_t i = 0; i < listResults.size(); i++)
    {
        // We need to parse out just the trailing part of the path name.
        int len = listResults[i].name.size();
        if (len > 0)
        {
            std::string prev_token_str;
            if (listResults[i].name.back() == '/')
            {
                prev_token_str = listResults[i].name.substr(srcPathStr.size() - 1, listResults[i].name.size() - srcPathStr.size());
            }
            else
            {
                prev_token_str = listResults[i].name.substr(srcPathStr.size() - 1);
            }

            // TODO: order or hash the list to improve perf
            if ((prev_token_str.size() > 0) && (std::find(local_list_results.begin(), local_list_results.end(), prev_token_str) == local_list_results.end()))
            {
                int nameLen = prev_token_str.size();
                char *newSrc = (char *)malloc(sizeof(char) * (srcPathStr.size() + nameLen + 1));
                memcpy(newSrc, srcPathStr.c_str(), srcPathStr.size());
                memcpy(&(newSrc[srcPathStr.size()]), prev_token_str.c_str(), nameLen);
                newSrc[srcPathStr.size() + nameLen] = '\0';

                char *newDst = (char *)malloc(sizeof(char) * (dstPathStr.size() + nameLen + 1));
                memcpy(newDst, dstPathStr.c_str(), dstPathStr.size());
                memcpy(&(newDst[dstPathStr.size()]), prev_token_str.c_str(), nameLen);
                newDst[dstPathStr.size() + nameLen] = '\0';

                AZS_DEBUGLOGV("Object found on the service - about to rename %s to %s.\n", newSrc, newDst);
                if (listResults[i].is_directory)
                {
                    azs_rename_directory(newSrc, newDst);
                }
                else
                {
                    azs_rename_single_file(newSrc, newDst);
                }

                free(newSrc);
                free(newDst);
            }
        }
    }
    azs_rmdir(src);
    return 0;
}



// TODO: Fix bug where the files and directories in the source in the file cache are not deleted.
// TODO: Fix bugs where the a file has been created but not yet uploaded.
// TODO: Fix the bug where this fails for multi-level dirrectories.
// TODO: If/when we upgrade to FUSE 3.0, we will need to worry about the additional possible flags (RENAME_EXCHANGE and RENAME_NOREPLACE)
int azs_rename(const char *src, const char *dst)
{
    AZS_DEBUGLOGV("azs_rename called with src = %s, dst = %s.\n", src, dst);

    struct stat statbuf;
    errno = 0;
    int getattrret = azs_getattr(src, &statbuf);
    if (getattrret != 0)
    {
        return getattrret;
    }
    if ((statbuf.st_mode & S_IFDIR) == S_IFDIR)
    {
        azs_rename_directory(src, dst);
    }
    else
    {
        azs_rename_single_file(src, dst);
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
