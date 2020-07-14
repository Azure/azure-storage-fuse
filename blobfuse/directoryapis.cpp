#include <blobfuse.h>

#include <include/StorageBfsClientBase.h>
extern std::shared_ptr<StorageBfsClientBase> storage_client;

// TODO: Bug in azs_mkdir, should fail if the directory already exists.
int azs_mkdir(const char *path, mode_t)
{
    AZS_DEBUGLOGV("mkdir called with path = %s\n", path);

    std::string pathstr(path);
    
    errno = 0;
    storage_client->CreateDirectory(pathstr.substr(1).c_str());
    if (errno != 0)
    {
        int storage_errno = errno;
        syslog(LOG_ERR, "Failed to create directory for path: %s.  errno = %d.\n", path, storage_errno);
        return 0 - map_errno(errno);
    }
    else
    {
        syslog(LOG_INFO, "Successfully created directory path: %s. ", path);
    }
    globalTimes.lastModifiedTime = globalTimes.lastAccessTime = globalTimes.lastChangeTime = time(NULL);
    return 0;
}

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
int azs_readdir(const char *path, void *buf, fuse_fill_dir_t filler, off_t, struct fuse_file_info *)
{
    AZS_DEBUGLOGV("azs_readdir called with path = %s\n", path);
    std::string pathStr(path);
    if (pathStr.size() > 1)
    {
        pathStr.push_back('/');
    }

    std::vector<std::string> local_list_results;

    // Scan for any files that exist in the local cache.
    // It is possible that there are files in the cache that aren't on the service - if a file has been opened but not yet uplaoded, for example.
    std::string mntPathString = prepend_mnt_path_string(pathStr);
    DIR *dir_stream = opendir(mntPathString.c_str());
    if (dir_stream != NULL)
    {
        AZS_DEBUGLOGV("Reading contents of local cache directory %s.\n", mntPathString.c_str());
        struct dirent* dir_ent = readdir(dir_stream);
        while (dir_ent != NULL)
        {
            if (dir_ent->d_name[0] != '.')
            {
                if (dir_ent->d_type == DT_DIR)
                {
                    struct stat stbuf;
                    stbuf.st_mode = S_IFDIR | config_options.defaultPermission;
                    stbuf.st_uid = fuse_get_context()->uid;
                    stbuf.st_gid = fuse_get_context()->gid;
                    stbuf.st_nlink = 2;
                    stbuf.st_size = 4096;
                    filler(buf, dir_ent->d_name, &stbuf, 0);
                    AZS_DEBUGLOGV("Subdirectory %s found in local cache directory %s during readdir operation.\n", dir_ent->d_name, mntPathString.c_str());
                }
                else
                {
                    struct stat buffer;
                    stat((mntPathString + dir_ent->d_name).c_str(), &buffer);

                    struct stat stbuf;
                    stbuf.st_mode = S_IFREG | config_options.defaultPermission; // Regular file (not a directory)
                    stbuf.st_uid = fuse_get_context()->uid;
                    stbuf.st_gid = fuse_get_context()->gid;
                    stbuf.st_nlink = 1;
                    stbuf.st_size = buffer.st_size;
                    filler(buf, dir_ent->d_name, &stbuf, 0); // TODO: Add stat information.  Consider FUSE_FILL_DIR_PLUS.
                    AZS_DEBUGLOGV("File %s found in local cache directory %s during readdir operation.\n", dir_ent->d_name, mntPathString.c_str());
                }

                std::string dir_str(dir_ent->d_name);
                local_list_results.push_back(dir_str);
            }

            dir_ent = readdir(dir_stream);
        }
        closedir(dir_stream);
    }
    else
    {
        AZS_DEBUGLOGV("Directory %s not found in file cache during readdir operation for %s.\n", mntPathString.c_str(), path);
    }

    errno = 0;
    std::vector<std::pair<std::vector<list_segmented_item>, bool>> listResults = storage_client->ListAllItemsSegmented(pathStr.substr(1), "/");
    if (errno != 0)
    {
        int storage_errno = errno;
        syslog(LOG_ERR, "Failed to list blobs under directory %s on the service during readdir operation.  errno = %d.\n", mntPathString.c_str(), storage_errno);
        return 0 - map_errno(storage_errno);
    }
    else
    {
        AZS_DEBUGLOGV("Reading blobs of directory %s on the service.  Total blob lists found = %s.\n", pathStr.c_str()+1, to_str(listResults.size()).c_str());
    }

    // Fill the blobfuse current and parent directories
    struct stat stcurrentbuf, stparentbuf;
    stcurrentbuf.st_mode = S_IFDIR | config_options.defaultPermission;
    stparentbuf.st_mode = S_IFDIR;

    filler(buf, ".", &stcurrentbuf, 0);
    filler(buf, "..", &stparentbuf, 0);

    // Enumerating segments of list_blobs response
    for (size_t result_lists_index = 0; result_lists_index < listResults.size(); result_lists_index++)
    {
        // Check to see if the first list_blobs__hierarchical_item can be skipped to avoid duplication
        int start = listResults[result_lists_index].second ? 1 : 0;
        for (size_t i = start; i < listResults[result_lists_index].first.size(); i++)
        {
            int fillerResult;
            // We need to parse out just the trailing part of the path name.
            list_segmented_item current_item = listResults[result_lists_index].first[i];
            int len = current_item.name.size();
            if (len > 0)
            {
                std::string prev_token_str;
                if (current_item.name.back() == '/')
                {
                    prev_token_str = current_item.name.substr(pathStr.size() - 1, current_item.name.size() - pathStr.size());
                }
                else
                {
                    prev_token_str = current_item.name.substr(pathStr.size() - 1);
                }

                // Any files that exist both on the service and in the local cache will be in both lists, we need to de-dup them.
                // TODO: order or hash the list to improve perf
                if (std::find(local_list_results.begin(), local_list_results.end(), prev_token_str) == local_list_results.end())
                {
                    if (!current_item.is_directory && 
                        !is_directory_blob(current_item.content_length, current_item.metadata))
                    {
                        if ((prev_token_str.size() > 0) && (strcmp(prev_token_str.c_str(), former_directory_signifier.c_str()) != 0))
                        {
                            struct stat stbuf;
                            if (is_symlink_blob(current_item.metadata)) {
                                stbuf.st_mode = S_IFLNK  | config_options.defaultPermission; // symlink
                            } else {
                                stbuf.st_mode = S_IFREG | config_options.defaultPermission; // Regular file (not a directory)
                            }
                            stbuf.st_uid = fuse_get_context()->uid;
                            stbuf.st_gid = fuse_get_context()->gid;
                            stbuf.st_nlink = 1;
                            stbuf.st_size = current_item.content_length;
                            fillerResult = filler(buf, prev_token_str.c_str(), &stbuf, 0); // TODO: Add stat information.  Consider FUSE_FILL_DIR_PLUS.
                            AZS_DEBUGLOGV("Blob %s found in directory %s on the service during readdir operation.  Adding to readdir list; fillerResult = %d.\n", prev_token_str.c_str(), pathStr.c_str()+1, fillerResult);
                        }
                    }
                    else
                    {
                        if (prev_token_str.size() > 0)
                        {
                            // Avoid duplicate directories - this avoids duplicate entries of legacy WASB and HNS directories
                            local_list_results.push_back(prev_token_str);

                            struct stat stbuf;
                            stbuf.st_mode = S_IFDIR | config_options.defaultPermission;
                            stbuf.st_uid = fuse_get_context()->uid;
                            stbuf.st_gid = fuse_get_context()->gid;
                            stbuf.st_nlink = 2;
                            fillerResult = filler(buf, prev_token_str.c_str(), &stbuf, 0);
                            AZS_DEBUGLOGV("Blob directory %s found in directory %s on the service during readdir operation.  Adding to readdir list; fillerResult = %d. uid=%u. gid = %u\n", prev_token_str.c_str(), pathStr.c_str()+1, fillerResult, stbuf.st_uid, stbuf.st_gid);
                        }
                    }

                }
                else
                {
                    AZS_DEBUGLOGV("Skipping adding blob %s to readdir results because it was already added from the local cache.\n", prev_token_str.c_str());
                }
            }
        }
    }
    return 0;
}

int azs_rmdir(const char *path)
{
    AZS_DEBUGLOGV("azs_rmdir called with path = %s\n", path);

    std::string pathString(path);
    const char * mntPath;
    std::string mntPathString = prepend_mnt_path_string(pathString);
    mntPath = mntPathString.c_str();

    AZS_DEBUGLOGV("Attempting to delete local cache directory %s.\n", mntPath);
    remove(mntPath); // This will fail if the cache is not empty, which is fine, as in this case it will also fail later, after the server-side check.

    if(!storage_client->DeleteDirectory(pathString.substr(1)))
    {
        if(errno == HTTP_REQUEST_CONFLICT)
        {
            return -ENOTEMPTY;
        }
        return -errno;
    }
    globalTimes.lastModifiedTime = globalTimes.lastAccessTime = globalTimes.lastChangeTime = time(NULL);
    return 0;
}

int azs_statfs(const char *path, struct statvfs *stbuf)
{
    AZS_DEBUGLOGV("azs_statfs called with path = %s.\n", path);
    std::string pathString(path);

    struct stat statbuf;
    int getattrret = azs_getattr(path, &statbuf);
    if (getattrret != 0)
    {
        return getattrret;
    }

    // return tmp path stats
    errno = 0;
    int res = statvfs(config_options.tmpPath.c_str(), stbuf);
    if (res == -1)
        return -errno;

    return 0;
}
