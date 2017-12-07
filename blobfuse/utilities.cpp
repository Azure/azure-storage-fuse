
#include "blobfuse.h"

int map_errno(int error)
{
    auto mapping = error_mapping.find(error);
    if (mapping == error_mapping.end())
    {
        return error;
    }
    else
    {
        return mapping->second;
    }
}

std::string prepend_mnt_path_string(const std::string path)
{
    return str_options.tmpPath + "/root" + path;
}

int ensure_files_directory_exists_in_cache(const std::string file_path)
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
            if (AZS_PRINT)
            {
                fprintf(stdout, "Making directory %s\n", copypath);
            }
            struct stat st;
            if (stat(copypath, &st) != 0)
            {
                status = mkdir(copypath, 0770);
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

std::vector<list_blobs_hierarchical_item> list_all_blobs_hierarchical(std::string container, std::string delimiter, std::string prefix)
{
    static const int maxFailCount = 20;
    std::vector<list_blobs_hierarchical_item> results;

    std::string continuation;

    std::string prior;
    bool success = false;
    int failcount = 0;
    do
    {
        if (AZS_PRINT)
        {
            fprintf(stdout, "About to call list_blobs_hierarchial.  Container = %s, delimiter = %s, continuation = %s, prefix = %s\n", container.c_str(), delimiter.c_str(), continuation.c_str(), prefix.c_str());
        }

        errno = 0;
        list_blobs_hierarchical_response response = azure_blob_client_wrapper->list_blobs_hierarchical(container, delimiter, continuation, prefix);
        if (errno == 0)
        {
            success = true;
            failcount = 0;
            if (AZS_PRINT)
            {
                fprintf(stdout, "results count = %s\n", to_str(response.blobs.size()).c_str());
                fprintf(stdout, "next_marker = %s\n", response.next_marker.c_str());
            }
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
            if (AZS_PRINT)
            {
                fprintf(stdout, "list_blobs_hierarchical failed %d time with errno = %d\n", failcount, errno);
            }

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
int is_directory_empty(std::string container, std::string delimiter, std::string prefix)
{
    std::string continuation;
    bool success = false;
    int failcount = 0;
    bool dirBlobFound = false;
    do
    {
        errno = 0;
        list_blobs_hierarchical_response response = azure_blob_client_wrapper->list_blobs_hierarchical(container, delimiter, continuation, prefix);
        if (errno == 0)
        {
            success = true;
            failcount = 0;
            continuation = response.next_marker;
            if (response.blobs.size() > 1)
            {
                return D_NOTEMPTY;
            }
            if (response.blobs.size() == 1)
            {
                if ((!dirBlobFound) &&
                    (!response.blobs[0].is_directory) &&
                    (response.blobs[0].name.size() > directorySignifier.size()) &&
                    (0 == response.blobs[0].name.compare(response.blobs[0].name.size() - directorySignifier.size(), directorySignifier.size(), directorySignifier)))
                {
                    dirBlobFound = true;
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
    } while ((continuation.size() > 0) && !success && (failcount < 20));

    if (!success)
    {
    // errno will be set by list_blobs_hierarchial if the last call failed and we're out of retries.
        return -1;
    }
    
    return dirBlobFound ? D_EMPTY : D_NOTEXIST;
}


int azs_getattr(const char *path, struct stat *stbuf)
{
    if (AZS_PRINT)
    {
        fprintf(stdout, "azs_getattr called, Path requested = %s\n", path);
    }
    // If we're at the root, we know it's a directory
    if (strlen(path) == 1)
    {
        stbuf->st_mode = S_IFDIR | 0770; // TODO: proper access control.
        stbuf->st_nlink = 2; // Directories should have a hard-link count of 2 + (# child directories).  We don't have that count, though, so we jsut use 2 for now.  TODO: Evaluate if we could keep this accurate or not.
        stbuf->st_size = 4096;
        return 0;
    }

    // Check and see if the file/directory exists locally (because it's being buffered.)  If so, skip the call to Storage.
    std::string pathString(path);
    std::string mntPathString = prepend_mnt_path_string(pathString);

    int res;
    int acc = access(mntPathString.c_str(), F_OK);
    if (AZS_PRINT)
    {
        fprintf(stdout, "accessing mntPath = %s returned %d\n", mntPathString.c_str(), acc);
    }
    if (acc != -1 )
    {
        //(void) fi;
        res = lstat(mntPathString.c_str(), stbuf);
        if (AZS_PRINT)
        {
            printf("LSTAT res = %d, errno = %d, ENOENT = %d\n", res, errno, ENOENT);
        }
        if (res == -1)
            return -errno;
        return 0;
    }

    // It's not in the local cache.  Check to see if it's a blob on the service:
    std::string blobNameStr(&(path[1]));
    errno = 0;
    auto blob_property = azure_blob_client_wrapper->get_blob_property(str_options.containerName, blobNameStr);

    if ((errno == 0) && blob_property.valid())
    {
        if (AZS_PRINT)
        {
            fprintf(stdout, "Blob found!  Name = %s\n", path);
        }
        stbuf->st_mode = S_IFREG | 0770; // Regular file (not a directory)
        stbuf->st_nlink = 1;
        stbuf->st_size = blob_property.size;
        return 0;
    }
    else if (errno == 0 && !blob_property.valid())
    {
        // Check to see if it's a directory, instead of a file
        blobNameStr.push_back('/');

        errno = 0;
        int dirSize = is_directory_empty(str_options.containerName, "/", blobNameStr);

        if (errno != 0)
        {
            if (AZS_PRINT)
            {
                fprintf(stdout, "Tried to find dir %s, but received errno = %d\n", path, errno);
            }
            return 0 - map_errno(errno);
        }
        if (dirSize != D_NOTEXIST)
        {
            if (AZS_PRINT)
            {
                fprintf(stdout, "Directory %s found with return value: %d!\n", blobNameStr.c_str(), dirSize);
            }
            stbuf->st_mode = S_IFDIR | 0770;
            // If st_nlink = 2, means direcotry is empty.
            // Directory size will affect behaviour for mv, rmdir, cp etc.
            stbuf->st_nlink = dirSize == D_EMPTY ? 2 : 3;
            stbuf->st_size = 4096;
            return 0;
        }
        else
        {
            return -(ENOENT); // -2 = Entity does not exist.
        }
    }
    else
    {
        return 0 - map_errno(errno);
    }
}

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
    std::string rootPath(str_options.tmpPath + "/root");
    char *cstr = (char *)malloc(rootPath.size() + 1);
    memcpy(cstr, rootPath.c_str(), rootPath.size());
    cstr[rootPath.size()] = 0;

    errno = 0;
    // FTW_DEPTH instructs FTW to do a post-order traversal (children of a directory before the actual directory.)
    nftw(rootPath.c_str(), rm, 20, FTW_DEPTH); 
    free(cstr);
}


// Not yet implemented section:
int azs_access(const char * /*path*/, int /*mask*/)
{
    return 0;  // permit all access
}

int azs_readlink(const char * /*path*/, char * /*buf*/, size_t /*size*/)
{
    return -EINVAL; // not a symlink
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
    if (AZS_PRINT)
    {
        fprintf(stdout, "Renaming a whole directory.  src = %s, dst = %s.\n", src, dst);
    }
    std::string srcPathStr(src);
    if (srcPathStr.size() > 1)
    {
        srcPathStr.push_back('/');
    }
    std::string dstPathStr(dst);
    if (dstPathStr.size() > 1)
    {
        dstPathStr.push_back('/');
    }
    std::vector<std::string> local_list_results;

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
    }

    errno = 0;
    std::vector<list_blobs_hierarchical_item> listResults = list_all_blobs_hierarchical(str_options.containerName, "/", srcPathStr.substr(1));
    if (errno != 0)
    {
        if (AZS_PRINT)
        {
            fprintf(stdout, "azs_readdir list blobs failed with error = %d\n", errno);
        }
        return 0 - map_errno(errno);
    }

    if (AZS_PRINT)
    {
        fprintf(stdout, "result count = %s\n", to_str(listResults.size()).c_str());
    }
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
    if (AZS_PRINT)
    {
        fprintf(stdout, "Rename file  source = %s, destination = %s\n", src, dst);
    }

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
