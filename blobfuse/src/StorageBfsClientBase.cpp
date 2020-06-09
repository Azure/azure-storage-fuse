#include <StorageBfsClientBase.h>
#include <vector>
#include <sys/stat.h>
#include <permissions.h>
//
// Created by amanda on 1/17/20.
//

int StorageBfsClientBase::map_errno(int error)
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

std::string StorageBfsClientBase::prepend_mnt_path_string(const std::string& path)
{
    std::string result;
    result.reserve(configurations.tmpPath.length() + 5 + path.length());
    return result.append(configurations.tmpPath).append("/root").append(path);
}

int StorageBfsClientBase::ensure_directory_path_exists_cache(const std::string & file_path)
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
                status = mkdir(copypath, configurations.defaultPermission);
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


list_segmented_item::list_segmented_item(list_blobs_segmented_item item)
{
    adls = false;
    block_item = item;
}


list_segmented_item::list_segmented_item(list_paths_item item)
{
    adls = true;
    adls_item = item;
}

list_segmented_response::list_segmented_response(list_blobs_segmented_response response)
{
    adls = false;
    block_item = response;
}

list_segmented_response::list_segmented_response(list_paths_result response)
{
    adls = true;
    adls_item = response;
}
  

