#include <blobfuse.h>

#include <include/StorageBfsClientBase.h>
extern std::shared_ptr<StorageBfsClientBase> storage_client;

// TODO: Bug in azs_mkdir, should fail if the directory already exists.
int azs_mkdir(const char *path, mode_t)
{
    AZS_DEBUGLOGV("mkdir called with path = %s\n", path);

    std::string pathstr(path);
    // Replace '\' with '/' as for azure storage they will be considered as path seperators
    std::replace(pathstr.begin(), pathstr.end(), '\\', '/');

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

    // Replace '\' with '/' as for azure storage they will be considered as path seperators
    std::replace(pathStr.begin(), pathStr.end(), '\\', '/');

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

    // Fill the blobfuse current and parent directories
    struct stat stcurrentbuf, stparentbuf;
    stcurrentbuf.st_mode = S_IFDIR | config_options.defaultPermission;
    stparentbuf.st_mode = S_IFDIR;

    filler(buf, ".", &stcurrentbuf, 0);
    filler(buf, "..", &stparentbuf, 0);

    std::string continuation = "";
    std::string prior = "";
    bool success = false;
    int failcount = 0;
    uint total_count = 0;
    uint iteration = 0;
    std::string prev_token_str;
    struct stat stbuf;
    list_segmented_response response;

    errno = 0;
    do
    {
        AZS_DEBUGLOGV("azs_readdir : About to call list_blobs.  Container = %s, delimiter = %s, continuation = %s, prefix = %s\n",
                      config_options.containerName.c_str(),
                      "/",
                      continuation.c_str(),
                      pathStr.substr(1).c_str());

        errno = 0;
        response.reset();
        storage_client->List(continuation, pathStr.substr(1), "/", response, 5000);
        if (errno == 0)
        {
            success = true;
            failcount = 0;

            iteration++;
            total_count += response.m_items.size();
            
            //AZS_DEBUGLOGV("Successful call to list_blobs_segmented.  results count = %d, next_marker = %s.\n", (int)response.m_items.size(), response.m_next_marker.c_str());
            
            continuation = response.m_next_marker;
            if (!response.m_items.empty())
            {
                bool skip_first = false;
                if (response.m_items[0].name == prior)
                {
                    skip_first = true;
                }
                prior = response.m_items.back().name;
                
                for (size_t i = ((skip_first) ? 1 : 0); i < response.m_items.size(); i++)
                {
                    if (response.m_items[i].name.size() > 0)
                    {
                        if (response.m_items[i].name.back() == '/')
                        {
                            prev_token_str = response.m_items[i].name.substr(pathStr.size() - 1, response.m_items[i].name.size() - pathStr.size());
                        }
                        else
                        {
                            prev_token_str = response.m_items[i].name.substr(pathStr.size() - 1);
                        }

                        if ((prev_token_str.size() > 0)
                            && std::find(local_list_results.begin(), local_list_results.end(), prev_token_str) == 
                                    local_list_results.end())
                        {
                            // Item not found in local cached list so add this one
                            stbuf.st_uid = fuse_get_context()->uid;
                            stbuf.st_gid = fuse_get_context()->gid;
                            stbuf.st_size = 0;

                            if (!response.m_items[i].is_directory && 
                                !is_directory_blob(response.m_items[i].content_length, response.m_items[i].metadata))
                            {
                                // Blob is file
                                if (is_symlink_blob(response.m_items[i].metadata)) {
                                    stbuf.st_mode = S_IFLNK  | config_options.defaultPermission;
                                } else {
                                    stbuf.st_mode = S_IFREG | config_options.defaultPermission;
                                }
                                stbuf.st_nlink = 1;
                                stbuf.st_size = response.m_items[i].content_length;
                            } else{
                                // Blob is Directory
                                stbuf.st_mode = S_IFDIR | config_options.defaultPermission;
                                stbuf.st_nlink = 2;
                                local_list_results.push_back(prev_token_str);
                            }

                            //int fillerResult = 
                            filler(buf, prev_token_str.c_str(), &stbuf, 0);
                            //AZS_DEBUGLOGV("Adding to readdir list : %s : fillerResult = %d. uid=%u. gid = %u\n", 
                            //        prev_token_str.c_str(), fillerResult, stbuf.st_uid, stbuf.st_gid);
                        }
                    }
                }
                AZS_DEBUGLOGV("#### So far %u items retreived in %u iterations.\n", total_count, iteration);
            
            }
        }
        else if (errno == 404)
        {
            success = true;
            syslog(LOG_WARNING, "list_blobs indicates blob not found");
        }
        else
        {
            failcount++;
            success = false;
            syslog(LOG_WARNING, "list_blobs failed for the %d time with errno = %d.\n", failcount, errno);
        }
    } while (((!continuation.empty()) || !success) && (failcount < 20));

    local_list_results.clear();
    local_list_results.shrink_to_fit();

    return 0;
}

int azs_rmdir(const char *path)
{
    AZS_DEBUGLOGV("azs_rmdir called with path = %s\n", path);

    std::string pathString(path);
    
    // Replace '\' with '/' as for azure storage they will be considered as path seperators
    std::replace(pathString.begin(), pathString.end(), '\\', '/');

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
