#include "blobfuse.h"

// TODO: Bug in azs_mkdir, should fail if the directory already exists.
int azs_mkdir(const char *path, mode_t)
{
    AZS_DEBUGLOGV("mkdir called with path = %s\n", path);

    std::string pathstr(path);

    // We want to upload a zero-length blob in this case - it's just a marker that there's a directory.
    std::istringstream emptyDataStream("");

    std::vector<std::pair<std::string, std::string>> metadata;
    metadata.push_back(std::make_pair("hdi_isfolder", "true"));
    errno = 0;
    azure_blob_client_wrapper->upload_block_blob_from_stream(str_options.containerName, pathstr.substr(1), emptyDataStream, metadata);
    if (errno != 0)
    {
        int storage_errno = errno;
        syslog(LOG_ERR, "Failed to upload zero-length directory marker for path %s to blob %s.  errno = %d.\n", path, pathstr.c_str()+1, storage_errno);
        return 0 - map_errno(errno);
    }
    else
    {
        syslog(LOG_INFO, "Successfully uploaded zero-length directory marker for path %s to blob %s. ", path, pathstr.c_str()+1);
    }
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
                    stbuf.st_mode = S_IFDIR | default_permission;
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
                    stbuf.st_mode = S_IFREG | default_permission; // Regular file (not a directory)
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
    std::vector<list_blobs_hierarchical_item> listResults = list_all_blobs_hierarchical(str_options.containerName, "/", pathStr.substr(1));
    if (errno != 0)
    {
        int storage_errno = errno;
        syslog(LOG_ERR, "Failed to list blobs under directory %s on the service during readdir operation.  errno = %d.\n", mntPathString.c_str(), storage_errno);
        return 0 - map_errno(storage_errno);
    }
    else
    {
        AZS_DEBUGLOGV("Reading blobs of directory %s on the service.  Total blobs found = %s.\n", pathStr.c_str()+1, to_str(listResults.size()).c_str());
    }

    filler(buf, ".", NULL, 0);
    filler(buf, "..", NULL, 0);

    for (size_t i = 0; i < listResults.size(); i++)
    {
        int fillerResult;
        // We need to parse out just the trailing part of the path name.
        int len = listResults[i].name.size();
        if (len > 0)
        {
            std::string prev_token_str;
            if (listResults[i].name.back() == '/')
            {
                prev_token_str = listResults[i].name.substr(pathStr.size() - 1, listResults[i].name.size() - pathStr.size());
            }
            else
            {
                prev_token_str = listResults[i].name.substr(pathStr.size() - 1);
            }

            // Any files that exist both on the service and in the local cache will be in both lists, we need to de-dup them.
            // TODO: order or hash the list to improve perf
            if (std::find(local_list_results.begin(), local_list_results.end(), prev_token_str) == local_list_results.end())
            {
                if (!listResults[i].is_directory && !is_directory_blob(listResults[i].content_length, listResults[i].metadata))
                {
                    if ((prev_token_str.size() > 0) && (strcmp(prev_token_str.c_str(), former_directory_signifier.c_str()) != 0))
                    {
                        struct stat stbuf;
                        stbuf.st_mode = S_IFREG | default_permission; // Regular file (not a directory)
                        stbuf.st_uid = fuse_get_context()->uid;
                        stbuf.st_gid = fuse_get_context()->gid;
                        stbuf.st_nlink = 1;
                        stbuf.st_size = listResults[i].content_length;
                        fillerResult = filler(buf, prev_token_str.c_str(), &stbuf, 0); // TODO: Add stat information.  Consider FUSE_FILL_DIR_PLUS.
                        AZS_DEBUGLOGV("Blob %s found in directory %s on the service during readdir operation.  Adding to readdir list; fillerResult = %d.\n", prev_token_str.c_str(), pathStr.c_str()+1, fillerResult);
                    }
                }
                else
                {
                    if (prev_token_str.size() > 0)
                    {
                        struct stat stbuf;
                        stbuf.st_mode = S_IFDIR | default_permission;
                        stbuf.st_uid = fuse_get_context()->uid;
                        stbuf.st_gid = fuse_get_context()->gid;
                        stbuf.st_nlink = 2;
                        fillerResult = filler(buf, prev_token_str.c_str(), &stbuf, 0);
                        AZS_DEBUGLOGV("Blob directory %s found in directory %s on the service during readdir operation.  Adding to readdir list; fillerResult = %d.\n", prev_token_str.c_str(), pathStr.c_str()+1, fillerResult);
                    }
                }
                // Avoid duplicates
                local_list_results.push_back(prev_token_str);

            }
            else
            {
                AZS_DEBUGLOGV("Skipping adding blob %s to readdir results because it was already added from the local cache.\n", prev_token_str.c_str());
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

    errno = 0;
    int dirStatus = is_directory_empty(str_options.containerName, pathString.substr(1));
    if (errno != 0)
    {
        int storage_errno = errno;
        syslog(LOG_ERR, "Failure to query the service to determine if directory %s is empty.  errno = %d.\n", path, storage_errno);
        return 0 - map_errno(errno);
    }
    if (dirStatus == D_NOTEXIST)
    {
        syslog(LOG_ERR, "Directory %s does not exist; failing directory delete operation.\n", path);
        return -ENOENT;
    }
    if (dirStatus == D_NOTEMPTY)
    {
        syslog(LOG_ERR, "Directory %s is not empty; failing directory delete operation.\n", path);
        return -ENOTEMPTY;
    }

    // TODO: change this to just delete blobs.
    errno = 0;
    azure_blob_client_wrapper->delete_blob(str_options.containerName, pathString.substr(1));
    int dir_blob_delete_errno = errno;
    if (dir_blob_delete_errno == 0)
    {
        syslog(LOG_INFO, "Successfully deleted directory marker %s for path %s. ", pathString.c_str()+1, path);
    }
    else
    {
        AZS_DEBUGLOGV("Failed to delete directory marker %s at path %s, errno = %d.  Checking the .directory version.\n", mntPath, pathString.c_str()+1, dir_blob_delete_errno);

        pathString.append("/.directory");

        errno = 0;
        azure_blob_client_wrapper->delete_blob(str_options.containerName, pathString.substr(1));
        int old_dir_blob_delete_errno = errno;

        if (old_dir_blob_delete_errno == 0)
        {
            syslog(LOG_INFO, "Successfully deleted .directory-style directory marker for path %s to blob %s. ", path, pathString.c_str()+1);
        }
        else
        {
            // If they both fail, dir_blob_delete_errno will be the important one in 99.99% of cases
            syslog(LOG_ERR, "Failed I/O operation to delete directory %s.  errno = %d\n", path, dir_blob_delete_errno);
            return 0 - map_errno(dir_blob_delete_errno);
        }
    }

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
    int res = statvfs(str_options.tmpPath.c_str(), stbuf);
    if (res == -1)
        return -errno;

    return 0;
}
