#include "blobfuse.h"

int azs_mkdir(const char *path, mode_t)
{
    if (AZS_PRINT)
    {
        fprintf(stdout, "mkdir called with path = %s\n", path);
    }

    std::string pathstr(path);
    pathstr.insert(pathstr.size(), "/" + directorySignifier);

    // We want to upload a zero-length blob in this case - it's just a marker that there's a directory.
    std::istringstream emptyDataStream("");

    std::vector<std::pair<std::string, std::string>> metadata;
    errno = 0;
    azure_blob_client_wrapper->upload_block_blob_from_stream(str_options.containerName, pathstr.substr(1), emptyDataStream, metadata);
    if (errno != 0)
    {
        return 0 - map_errno(errno);
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
    if (AZS_PRINT)
    {
        fprintf(stdout, "azs_readdir called with path = %s\n", path);
    }
    std::string pathStr(path);
    if (pathStr.size() > 1)
    {
        pathStr.push_back('/');
    }

    std::vector<std::string> local_list_results;

    std::string mntPathString = prepend_mnt_path_string(pathStr);
    DIR *dir_stream = opendir(mntPathString.c_str());
    if (dir_stream != NULL)
    {
        struct dirent* dir_ent = readdir(dir_stream);
        while (dir_ent != NULL)
        {
            if (dir_ent->d_name[0] != '.')
            {
                if (dir_ent->d_type == DT_DIR)
                {
                    struct stat stbuf;
                    stbuf.st_mode = S_IFDIR | 0770;
                    stbuf.st_nlink = 2;
                    stbuf.st_size = 4096;
                    filler(buf, dir_ent->d_name, &stbuf, 0);
                }
                else
                {
                    struct stat buffer;
                    stat((mntPathString + dir_ent->d_name).c_str(), &buffer);

                    struct stat stbuf;
                    stbuf.st_mode = S_IFREG | 0770; // Regular file (not a directory)
                    stbuf.st_nlink = 1;
                    stbuf.st_size = buffer.st_size;
                    filler(buf, dir_ent->d_name, &stbuf, 0); // TODO: Add stat information.  Consider FUSE_FILL_DIR_PLUS.
                }

                std::string dir_str(dir_ent->d_name);
                local_list_results.push_back(dir_str);

                if (AZS_PRINT)
                {
                    fprintf(stdout, "Local file/blob found.  Name = %s\n", dir_ent->d_name);
                }
            }

            dir_ent = readdir(dir_stream);
        }
    }



    errno = 0;
    std::vector<list_blobs_hierarchical_item> listResults = list_all_blobs_hierarchical(str_options.containerName, "/", pathStr.substr(1));
    if (errno != 0)
    {
        if (AZS_PRINT)
        {
            fprintf(stdout, "azs_readdir list blobs failed with error = %d\n", errno);
        }
        return 0 - map_errno(errno);
    }

    filler(buf, ".", NULL, 0);
    filler(buf, "..", NULL, 0);

    size_t i = 0;
    if (AZS_PRINT)
    {
        fprintf(stdout, "result count = %lu\n", listResults.size());
    }
    for (; i < listResults.size(); i++)
    {
        int fillerResult;
        // We need to parse out just the trailing part of the path name.
        int len = listResults[i].name.size();
        if (len > 0)
        {
            /*            char *nameCopy = (char *)malloc(len + 1);
                        memcpy(nameCopy, listResults[i].name.c_str(), len);
                        nameCopy[len] = 0;

                        char *lasts = NULL;
                        char *token = strtok_r(nameCopy, "/", &lasts);
                        char *prevtoken = NULL;

                        while (token)
                        {
                            prevtoken = token;
                            token = strtok_r(NULL, "/", &lasts);
                        }

                        std::string prev_token_str(prevtoken); */
            std::string prev_token_str;
            if (listResults[i].name.back() == '/')
            {
                prev_token_str = listResults[i].name.substr(pathStr.size() - 1, listResults[i].name.size() - pathStr.size());
            }
            else
            {
                prev_token_str = listResults[i].name.substr(pathStr.size() - 1);
            }

            // TODO: order or hash the list to improve perf
            if (std::find(local_list_results.begin(), local_list_results.end(), prev_token_str) == local_list_results.end())
            {
                if (!listResults[i].is_directory)
                {
                    if ((prev_token_str.size() > 0) && (strcmp(prev_token_str.c_str(), directorySignifier.c_str()) != 0))
                    {
                        struct stat stbuf;
                        stbuf.st_mode = S_IFREG | 0770; // Regular file (not a directory)
                        stbuf.st_nlink = 1;
                        stbuf.st_size = listResults[i].content_length;
                        fillerResult = filler(buf, prev_token_str.c_str(), &stbuf, 0); // TODO: Add stat information.  Consider FUSE_FILL_DIR_PLUS.
                        if (AZS_PRINT)
                        {
                            fprintf(stdout, "blob result = %s, fillerResult = %d\n", prev_token_str.c_str(), fillerResult);
                        }
                    }

                }
                else
                {
                    if (prev_token_str.size() > 0)
                    {
                        struct stat stbuf;
                        stbuf.st_mode = S_IFDIR | 0770;
                        stbuf.st_nlink = 2;
                        fillerResult = filler(buf, prev_token_str.c_str(), &stbuf, 0);
                        if (AZS_PRINT)
                        {
                            fprintf(stdout, "dir result = %s, fillerResult = %d\n", prev_token_str.c_str(), fillerResult);
                        }
                    }

                }
            }


//            free(nameCopy);
        }

    }
    if (AZS_PRINT)
    {
        fprintf(stdout, "Done with readdir\n");
    }
    return 0;

}

int azs_rmdir(const char *path)
{
    if (AZS_PRINT)
    {
        fprintf(stdout, "azs_rmdir called with path = %s\n", path);
    }

    std::string pathStr(path);
    if (pathStr.size() > 1)
    {
        pathStr.push_back('/');
    }

    std::string pathString(path);
    const char * mntPath;
    std::string mntPathString = prepend_mnt_path_string(pathString);
    mntPath = mntPathString.c_str();
    if (AZS_PRINT)
    {
        fprintf(stdout, "deleting local directory %s\n", mntPath);
    }
    remove(mntPath); // This will fail if the cache is not empty, which is fine, as in this case it will also fail later, after the server-side check.

    errno = 0;
    int dirStatus = is_directory_empty(str_options.containerName, "/", pathStr.substr(1));
    if (errno != 0)
    {
        return 0 - map_errno(errno);
    }
    if (dirStatus == D_NOTEXIST)
    {
        return -ENOENT;
    }
    if (dirStatus == D_NOTEMPTY)
    {
        return -ENOTEMPTY;
    }

    pathStr.append(".directory");
    azs_unlink(pathStr.c_str());


    return 0;

}

int azs_statfs(const char *path, struct statvfs *stbuf)
{

    std::string pathString(path);
    const char * mntPath;
    std::string mntPathString = prepend_mnt_path_string(pathString);
    mntPath = mntPathString.c_str();


    int res = statvfs(mntPath, stbuf);
    if (res == -1)
        return -errno;

    return 0;
}
