#include <StorageBfsClientBase.h>
#include <vector>
#include <sys/stat.h>
#include <Permissions.h>

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

list_segmented_item::list_segmented_item() 
{}

list_segmented_item::list_segmented_item(list_blobs_segmented_item &item) :
        name(item.name),
        //snapshot(item.snapshot),
        last_modified(item.last_modified),
        //etag(item.etag),
        content_length(item.content_length),
        //content_encoding(item.content_encoding),
        //content_md5(item.content_md5),
        //content_language(item.content_language),
        //cache_control(item.cache_control),
        //copy_status(item.copy_status),
        metadata(std::move(item.metadata)),
        is_directory(item.is_directory) {}

list_segmented_item::list_segmented_item(list_paths_item &item) :
        name(item.name),
        last_modified(item.last_modified),
        //etag(item.etag),
        content_length(item.content_length),
        acl(item.acl),
        mode(aclToMode(item.acl)),
        is_directory(item.is_directory) {}

list_segmented_response::list_segmented_response(list_blobs_segmented_response &response) :
        //m_ms_request_id(std::move(response.ms_request_id)),
        m_next_marker(response.next_marker),
        m_valid(true)
{
    //TODO make this better
    unsigned int item_size = response.blobs.size();
    m_items.reserve(item_size);
    for(unsigned int i = 0; i < item_size; i++)
    {
        m_items.emplace_back(list_segmented_item(response.blobs.at(i)));
    }
}

list_segmented_response::list_segmented_response(list_paths_result &response) :
    continuation_token(response.continuation_token),
    m_valid(true)
{
    //TODO make this better
    unsigned int item_size = response.paths.size();
    m_items.reserve(item_size);
    for(unsigned int i = 0; i < item_size; i++)
    {
        m_items.emplace_back(list_segmented_item(response.paths.at(i)));
    }
}

void
list_segmented_response::populate(list_blobs_segmented_response &response)
{
    m_next_marker = response.next_marker;
    m_valid = true;

    unsigned int item_size = response.blobs.size();
    m_items.reserve(item_size);

    for(unsigned int i = 0; i < item_size; i++)
    {
        m_items.emplace_back(list_segmented_item(response.blobs.at(i)));
    }
}

void
list_segmented_response::populate(list_paths_result &response)
{
    continuation_token = response.continuation_token;
    m_valid = true;

    //TODO make this better
    unsigned int item_size = response.paths.size();
    m_items.reserve(item_size);
    for(unsigned int i = 0; i < item_size; i++)
    {
        m_items.emplace_back(list_segmented_item(response.paths.at(i)));
    }
}

