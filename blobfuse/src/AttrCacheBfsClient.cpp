#include <sys/stat.h>
#include <AttrCacheBfsClient.h>

// Helper to the the string representing the parent directory of a given item.
std::string get_parent_str(std::string object)
{
    size_t last_slash_idx = object.rfind('/');
    if (std::string::npos != last_slash_idx)
    {
        return object.substr(0, last_slash_idx);
    }
    return std::string();
}

// Directory is getting deleted, invalidate all the files and directories recursively inside
void AttrCache::invalidate_dir_recursively(const std::string& path)
{
    std::string dirPath = path + "/";
    std::shared_ptr<AttrCacheItem> cache_item;
    std::lock_guard<std::mutex> lock(blobs_mutex);
    for (auto item = blob_cache.begin(); item != blob_cache.end(); item++) 
    {
        if (item->first.rfind(dirPath.c_str(), 0) == 0 ||
            item->first == path)
        {
            cache_item = item->second;
            if (cache_item->m_confirmed) {
                //cache_item->m_confirmed = false;
                // Let the cache be still valid but mark that file no more exists on the storage
                cache_item->m_props.m_valid = true;
                cache_item->m_props.m_not_exists = true;
            }
        }
    }
}

// Performs a thread-safe map lookup of the input key in the directory map.
// Will create new entries if necessary before returning.
std::shared_ptr<boost::shared_mutex> AttrCache::get_dir_item(const std::string& path)
{
    std::lock_guard<std::mutex> lock(dirs_mutex);
    auto iter = dir_cache.find(path);
    if(iter == dir_cache.end())
    {
        auto dir_item = std::make_shared<boost::shared_mutex>();
        dir_cache[path] = dir_item;
        return dir_item;
    }
    else
    {
        return iter->second;
    }
}

// Performs a thread-safe map lookup of the input key in the blob map.
// Will create new entries if necessary before returning.
std::shared_ptr<AttrCacheItem> AttrCache::get_blob_item(const std::string& path)
{
    std::lock_guard<std::mutex> lock(blobs_mutex);
    auto iter = blob_cache.find(path);
    if(iter == blob_cache.end())
    {
        auto blob_item = std::make_shared<AttrCacheItem>("", BfsFileProperty(true));
        blob_cache[path] = blob_item;
        return blob_item;
    }
    else
    {
        return iter->second;
    }
}


bool AttrCacheBfsClient::AuthenticateStorage()
{
    return blob_client->AuthenticateStorage();
}

void AttrCacheBfsClient::UploadFromFile(const std::string sourcePath, METADATA &metadata)
{
    std::string blobName = sourcePath.substr(configurations.tmpPath.size() + 6);
    std::shared_ptr<boost::shared_mutex> dir_mutex = attr_cache.get_dir_item(get_parent_str(blobName));
    std::shared_ptr<AttrCacheItem> cache_item = attr_cache.get_blob_item(blobName);
    boost::shared_lock<boost::shared_mutex> dirlock(*dir_mutex);
    std::unique_lock<boost::shared_mutex> uniquelock(cache_item->m_mutex);
    if (cache_item->m_confirmed) {
        struct stat stbuf;
        if (0 == stat(sourcePath.c_str(), &stbuf)) {
            cache_item->m_props.size = stbuf.st_size;
            cache_item->m_props.last_modified = time(NULL);
            cache_item->m_props.m_not_exists = false;
        }
        else
            cache_item->m_confirmed = false;
    }
    return blob_client->UploadFromFile(sourcePath, metadata);
}

void AttrCacheBfsClient::UploadFromStream(std::istream &sourceStream, const std::string blobName)
{
    std::shared_ptr<boost::shared_mutex> dir_mutex = attr_cache.get_dir_item(get_parent_str(blobName));
    std::shared_ptr<AttrCacheItem> cache_item = attr_cache.get_blob_item(blobName);
    boost::shared_lock<boost::shared_mutex> dirlock(*dir_mutex);
    std::unique_lock<boost::shared_mutex> uniquelock(cache_item->m_mutex);
     if (cache_item->m_confirmed) {
        cache_item->m_props.size = 0;
        cache_item->m_props.last_modified = time(NULL);
        cache_item->m_props.m_not_exists = false;
    }
    return blob_client->UploadFromStream(sourceStream, blobName);
}

void AttrCacheBfsClient::UploadFromStream(std::istream &sourceStream, const std::string blobName,
                                          std::vector<std::pair<std::string, std::string>> &metadata)
{
    std::shared_ptr<boost::shared_mutex> dir_mutex = attr_cache.get_dir_item(get_parent_str(blobName));
    std::shared_ptr<AttrCacheItem> cache_item = attr_cache.get_blob_item(blobName);
    boost::shared_lock<boost::shared_mutex> dirlock(*dir_mutex);
    std::unique_lock<boost::shared_mutex> uniquelock(cache_item->m_mutex);
     if (cache_item->m_confirmed) {
        cache_item->m_props.size = 0;
        cache_item->m_props.last_modified = time(NULL);
        cache_item->m_props.m_not_exists = false;
    }
    return blob_client->UploadFromStream(sourceStream, blobName, metadata);
}

long int AttrCacheBfsClient::DownloadToFile(const std::string blobName, const std::string filePath, time_t &last_modified)
{
    return blob_client->DownloadToFile(blobName, filePath, last_modified);
}

long int AttrCacheBfsClient::DownloadToStream(const std::string blobName, std::ostream &destStream,
                                              unsigned long long offset, unsigned long long size)
{
    return blob_client->DownloadToStream(blobName, destStream, offset, size);
}

bool AttrCacheBfsClient::CreateDirectory(const std::string directoryPath)
{
    std::shared_ptr<boost::shared_mutex> dir_mutex = attr_cache.get_dir_item(get_parent_str(directoryPath));
    std::shared_ptr<AttrCacheItem> cache_item = attr_cache.get_blob_item(directoryPath);
    boost::shared_lock<boost::shared_mutex> dirlock(*dir_mutex);
    std::unique_lock<boost::shared_mutex> uniquelock(cache_item->m_mutex);
    cache_item->m_confirmed = false;
    return blob_client->CreateDirectory(directoryPath);
}

bool AttrCacheBfsClient::DeleteDirectory(const std::string directoryPath)
{
    attr_cache.invalidate_dir_recursively(directoryPath);
    bool ret = blob_client->DeleteDirectory(directoryPath);
    return ret;
}

void AttrCacheBfsClient::DeleteFile(const std::string pathToDelete)
{
    std::shared_ptr<boost::shared_mutex> dir_mutex = attr_cache.get_dir_item(get_parent_str(pathToDelete));
    std::shared_ptr<AttrCacheItem> cache_item = attr_cache.get_blob_item(pathToDelete);
    boost::shared_lock<boost::shared_mutex> dirlock(*dir_mutex);
    std::unique_lock<boost::shared_mutex> uniquelock(cache_item->m_mutex);
    blob_client->DeleteFile(pathToDelete);
    if (cache_item->m_confirmed) {
        cache_item->m_props.m_valid = true;
        cache_item->m_props.m_not_exists = true;
    }
}

BfsFileProperty AttrCacheBfsClient::GetProperties(std::string pathName, bool type_known)
{
    std::shared_ptr<boost::shared_mutex> dir_mutex = attr_cache.get_dir_item(get_parent_str(pathName));
    std::shared_ptr<AttrCacheItem> cache_item = attr_cache.get_blob_item(pathName);
    boost::shared_lock<boost::shared_mutex> dirlock(*dir_mutex);

    {
        boost::shared_lock<boost::shared_mutex> sharedlock(cache_item->m_mutex);
        if (cache_item->m_confirmed)
        {
            return cache_item->m_props;
        }
    }

    {
        std::unique_lock<boost::shared_mutex> uniquelock(cache_item->m_mutex);
        errno = 0;
        cache_item->m_props = blob_client->GetProperties(pathName, type_known);
        cache_item->m_confirmed = true;
        return cache_item->m_props;
    }
}

int AttrCacheBfsClient::Exists(const std::string pathName)
{
    return blob_client->Exists(pathName);
}

bool AttrCacheBfsClient::Copy(const std::string sourcePath, const std::string destinationPath)
{
    return blob_client->Copy(sourcePath, destinationPath);

}

std::vector<std::string> AttrCacheBfsClient::Rename(const std::string sourcePath, const std::string destinationPath)
{
    std::string srcPathStr(sourcePath);
    std::string dstPathStr(destinationPath);

    std::shared_ptr<boost::shared_mutex> dir_mutex = attr_cache.get_dir_item(get_parent_str(sourcePath.substr(1)));
    std::shared_ptr<AttrCacheItem> cache_item = attr_cache.get_blob_item(sourcePath.substr(1));
    boost::shared_lock<boost::shared_mutex> dirlock(*dir_mutex);
    std::unique_lock<boost::shared_mutex> uniquelock(cache_item->m_mutex);
    cache_item->m_confirmed = false;

    std::shared_ptr<boost::shared_mutex> ddir_mutex = attr_cache.get_dir_item(get_parent_str(destinationPath.substr(1)));
    std::shared_ptr<AttrCacheItem> dcache_item = attr_cache.get_blob_item(destinationPath.substr(1));
    boost::shared_lock<boost::shared_mutex> ddirlock(*ddir_mutex);
    std::unique_lock<boost::shared_mutex> duniquelock(dcache_item->m_mutex);
    dcache_item->m_confirmed = false;

    return blob_client->Rename(sourcePath, destinationPath);
}

std::vector<std::string> AttrCacheBfsClient::Rename(const std::string sourcePath,const  std::string destinationPath, bool isDir)
{
    std::string srcPathStr(sourcePath);
    std::string dstPathStr(destinationPath);

    std::shared_ptr<boost::shared_mutex> dir_mutex = attr_cache.get_dir_item(get_parent_str(srcPathStr.substr(1)));
    std::shared_ptr<AttrCacheItem> cache_item = attr_cache.get_blob_item(srcPathStr.substr(1));
    boost::shared_lock<boost::shared_mutex> dirlock(*dir_mutex);
    std::unique_lock<boost::shared_mutex> uniquelock(cache_item->m_mutex);
    cache_item->m_confirmed = false;

    std::shared_ptr<boost::shared_mutex> ddir_mutex = attr_cache.get_dir_item(get_parent_str(dstPathStr.substr(1)));
    std::shared_ptr<AttrCacheItem> dcache_item = attr_cache.get_blob_item(dstPathStr.substr(1));
    boost::shared_lock<boost::shared_mutex> ddirlock(*ddir_mutex);
    std::unique_lock<boost::shared_mutex> duniquelock(dcache_item->m_mutex);
    dcache_item->m_confirmed = false;

    return blob_client->Rename(sourcePath, destinationPath, isDir);
}

list_segmented_response
AttrCacheBfsClient::List(std::string continuation, const std::string prefix, const std::string delimiter, int max_results)
{
    return blob_client->List(continuation, prefix, delimiter, max_results);
}

bool AttrCacheBfsClient::IsDirectory(const char *path)
{
    return blob_client->IsDirectory(path);
}

D_RETURN_CODE AttrCacheBfsClient::IsDirectoryEmpty(std::string path)
{
    return blob_client->IsDirectoryEmpty(path);
}

std::vector<std::pair<std::vector<list_segmented_item>, bool>> AttrCacheBfsClient::ListAllItemsSegmented(
    const std::string &prefix,
    const std::string &delimiter,
    int max_results)
{
    std::vector<std::pair<std::vector<list_segmented_item>, bool>> listResponse =
             blob_client->ListAllItemsSegmented(prefix, delimiter, max_results);

    #if 1
    if (errno == 0 && listResponse.size() > 0)
    {
        list_segmented_item blobItem;
        unsigned int batchNum = 0;
        unsigned int resultStart = 0;

        for (batchNum = 0; batchNum < listResponse.size(); batchNum++)
        {
            // if skip_first start the listResults at 1
            resultStart = listResponse[batchNum].second ? 1 : 0;

            std::vector<list_segmented_item> listResults = listResponse[batchNum].first;
            for (unsigned int i = resultStart; i < listResults.size(); i++)
            {
                blobItem = listResults[i];
                time_t last_mod = time(NULL);
                if (!blobItem.last_modified.empty()) {
                    struct tm mtime;
                    char *ptr = strptime(blobItem.last_modified.c_str(), "%a, %d %b %Y %H:%M:%S", &mtime);
                    if (ptr)
                        last_mod = timegm(&mtime);
                }   

                #if 0
                if (isAdlsMode)
                {
                    BfsFileProperty ret_property(
                        "",
                        blobItem.acl.owner,
                        blobItem.acl.group,
                        blobItem.acl.permissions,
                        blobItem.metadata,
                        last_mod,
                        blobItem.acl.permissions,
                        blobItem.content_length);

                    if (blobItem.is_directory)
                        ret_property.is_directory = true;

                    std::shared_ptr<AttrCacheItem> cache_item = attr_cache.get_blob_item(listResults[i].name);
                    std::unique_lock<boost::shared_mutex> uniquelock(cache_item->m_mutex);
                    cache_item->m_props = ret_property;
                    cache_item->m_confirmed = true;
                } else 
                #endif
                {
                    BfsFileProperty ret_property(
                            "",
                            blobItem.metadata,
                            last_mod,
                            "", 
                            blobItem.content_length);

                    /*if (isAdlsMode) {
                        access_control acl = blob_client->GetAccessControl(listResults[i].name);
                        ret_property.SetFileMode(acl.permissions);
                    }*/

                    std::shared_ptr<AttrCacheItem> cache_item = attr_cache.get_blob_item(listResults[i].name);
                    std::unique_lock<boost::shared_mutex> uniquelock(cache_item->m_mutex);
                    cache_item->m_props = ret_property;
                    cache_item->m_confirmed = true; 
                }  
            }
        }
    }
    #endif

    return listResponse;
}

int AttrCacheBfsClient::ChangeMode(const char *path, mode_t mode)
{
    std::string pathStr(path);
    if (isAdlsMode) {
        std::shared_ptr<boost::shared_mutex> dir_mutex = attr_cache.get_dir_item(get_parent_str(pathStr.substr(1)));
        std::shared_ptr<AttrCacheItem> cache_item = attr_cache.get_blob_item(pathStr.substr(1));
        boost::shared_lock<boost::shared_mutex> dirlock(*dir_mutex);
        std::unique_lock<boost::shared_mutex> uniquelock(cache_item->m_mutex);
        cache_item->m_confirmed = false;
    }
    return blob_client->ChangeMode(path, mode);
}

int AttrCacheBfsClient::UpdateBlobProperty(std::string pathStr, std::string key, std::string value, METADATA * metadata)
{
    return blob_client->UpdateBlobProperty(pathStr, key, value, metadata);
}

access_control AttrCacheBfsClient::GetAccessControl(const std::string pathName)
{
    return blob_client->GetAccessControl(pathName);
}