#include <sys/stat.h>
#include <BlockBlobBfsClient.h>

///<summary>
/// Authenticates the storage account and container
///</summary>
///<returns>bool: if we authenticate to the storage account and container successfully</returns>
bool BlockBlobBfsClient::AuthenticateStorage()
{
    // Authenticate the storage account
    switch (configurations.authType)
    {
    case KEY_AUTH:
        m_blob_client = authenticate_blob_accountkey();
        break;
    case SAS_AUTH:
        m_blob_client = authenticate_blob_sas();
        break;
    case MSI_AUTH:
        m_blob_client = authenticate_blob_msi();
        break;
    case SPN_AUTH:
        m_blob_client = authenticate_blob_spn();
        break;
    default:
        return false;
        break;
    }

    if (m_blob_client->is_valid())
    {
        m_blob_client->set_retry_policy(std::make_shared<azure::storage_lite::expo_retry_policy>());

        //Authenticate the storage container by using a list call
        m_blob_client->list_blobs_segmented(
            configurations.containerName,
            "/",
            std::string(),
            std::string(),
            1);
        if (errno != 0)
        {
            syslog(LOG_ERR,
                   "Unable to start blobfuse.  Failed to connect to the storage container. There might be something wrong about the storage config, please double check the storage account name, account key/sas token/OAuth access token and container name. errno = %d\n",
                   errno);
            return false;
        }
        return true;
    }
    return false;
}

std::shared_ptr<blob_client_wrapper> BlockBlobBfsClient::authenticate_blob_accountkey()
{
    syslog(LOG_DEBUG, "Authenticating using account key");
    try
    {
        std::shared_ptr<storage_credential> cred;
        if (configurations.accountKey.length() > 0)
        {
            cred = std::make_shared<shared_key_credential>(configurations.accountName, configurations.accountKey);
        }
        else
        {
            syslog(LOG_ERR, "Empty account key. Failed to create blob client.");
            return std::make_shared<blob_client_wrapper>(false);
        }
        std::shared_ptr<storage_account> account = std::make_shared<storage_account>(
            configurations.accountName,
            cred,
            configurations.useHttps,
            configurations.blobEndpoint);
        std::shared_ptr<blob_client> blobClient = std::make_shared<blob_client>(
            account,
            configurations.concurrency);
        errno = 0;
        return std::make_shared<blob_client_wrapper>(blobClient);
    }
    catch (const std::exception &ex)
    {
        syslog(LOG_ERR, "Failed to create blob client.  ex.what() = %s.", ex.what());
        errno = blobfuse_constants::unknown_error;
        return std::make_shared<blob_client_wrapper>(false);
    }
}
std::shared_ptr<blob_client_wrapper> BlockBlobBfsClient::authenticate_blob_sas()
{
    syslog(LOG_DEBUG, "Authenticating using SAS");
    try
    {
        std::shared_ptr<storage_credential> cred;
        if (configurations.sasToken.length() > 0)
        {
            cred = std::make_shared<shared_access_signature_credential>(configurations.sasToken);
        }
        else
        {
            syslog(LOG_ERR, "Empty account key. Failed to create blob client.");
            return std::make_shared<blob_client_wrapper>(false);
        }
        std::shared_ptr<storage_account> account = std::make_shared<storage_account>(
            configurations.accountName, cred,
            configurations.useHttps,
            configurations.blobEndpoint);
        std::shared_ptr<blob_client> blobClient = std::make_shared<blob_client>(
            account,
            configurations.concurrency);
        errno = 0;
        return std::make_shared<blob_client_wrapper>(blobClient);
    }
    catch (const std::exception &ex)
    {
        syslog(LOG_ERR, "Failed to create blob client.  ex.what() = %s.", ex.what());
        errno = blobfuse_constants::unknown_error;
        return std::make_shared<blob_client_wrapper>(false);
    }
}
std::shared_ptr<blob_client_wrapper> BlockBlobBfsClient::authenticate_blob_msi()
{
    syslog(LOG_DEBUG, "Authenticating using MSI");
    try
    {
        //1. get oauth token
        std::function<OAuthToken(std::shared_ptr<CurlEasyClient>)> MSICallback = SetUpMSICallback(
            configurations.identityClientId,
            configurations.objectId,
            configurations.resourceId,
            configurations.msiEndpoint,
            configurations.msiSecret);

        std::shared_ptr<OAuthTokenCredentialManager> tokenManager = GetTokenManagerInstance(MSICallback);

        if (!tokenManager->is_valid_connection())
        {
            // todo: isolate definitions of errno's for this function so we can output something meaningful.
            errno = 1;
            return std::make_shared<blob_client_wrapper>(false);
        }

        //2. try to make blob client wrapper using oauth token
        // We should pass the token obtained earlier to this token_credentials
        std::shared_ptr<storage_credential> cred = std::make_shared<token_credential>("");
        cred->set_token_callback(&GetTokenCallback);

        std::shared_ptr<storage_account> account = std::make_shared<storage_account>(
            configurations.accountName,
            cred,
            true, //use_https must be true to use oauth
            configurations.blobEndpoint);
        std::shared_ptr<blob_client> blobClient =
            std::make_shared<blob_client>(account, max_concurrency_oauth);
        errno = 0;
        return std::make_shared<blob_client_wrapper>(blobClient);
    }
    catch (const std::exception &ex)
    {
        syslog(LOG_ERR, "Failed to create blob client.  ex.what() = %s. Please check your account name and ", ex.what());
        errno = blobfuse_constants::unknown_error;
        return std::make_shared<blob_client_wrapper>(false);
    }
}
std::shared_ptr<blob_client_wrapper> BlockBlobBfsClient::authenticate_blob_spn()
{
    syslog(LOG_DEBUG, "Authenticating using SPN");
    try
    {
        //1. get oauth token
        std::function<OAuthToken(std::shared_ptr<CurlEasyClient>)> SPNCallback = SetUpSPNCallback(
            configurations.spnTenantId,
            configurations.spnClientId,
            configurations.spnClientSecret,
            configurations.aadEndpoint);

        std::shared_ptr<OAuthTokenCredentialManager> tokenManager = GetTokenManagerInstance(SPNCallback);

        if (!tokenManager->is_valid_connection())
        {
            // todo: isolate definitions of errno's for this function so we can output something meaningful.
            errno = 1;
            syslog(LOG_ERR, "Failed to get token using SPN credentials.");
            return std::make_shared<blob_client_wrapper>(false);
        }

        //2. try to make blob client wrapper using oauth token
        // We should pass the token obtained earlier to this token_credentials
        std::shared_ptr<storage_credential> cred = std::make_shared<token_credential>("");
        cred->set_token_callback(&GetTokenCallback);

        std::shared_ptr<storage_account> account = std::make_shared<storage_account>(
            configurations.accountName,
            cred,
            true, //use_https must be true to use oauth
            configurations.blobEndpoint);
        std::shared_ptr<blob_client> blobClient =
            std::make_shared<blob_client>(account, max_concurrency_oauth);
        errno = 0;
        return std::make_shared<blob_client_wrapper>(blobClient);
    }
    catch (const std::exception &ex)
    {
        syslog(LOG_ERR, "Failed to create blob client.  ex.what() = %s. Please check your account name and ", ex.what());
        errno = blobfuse_constants::unknown_error;
        return std::make_shared<blob_client_wrapper>(false);
    }
}

///<summary>
/// Uploads contents of a file to a block blob to the Storage service
///</summary>
///TODO: params
///<returns>none</returns>
void BlockBlobBfsClient::UploadFromFile(const std::string sourcePath, METADATA &metadata)
{
    std::string blobName = sourcePath.substr(configurations.tmpPath.size() + 6 /* there are six characters in "/root/" */);
    m_blob_client->upload_file_to_blob(sourcePath, configurations.containerName, blobName, metadata);
    // upload_file_to_blob does not return a status or success if the blob succeeded
    // it does syslog if there was an exception and changes the errno.
}
///<summary>
/// Uploads contents of a stream to a block blob to the Storage service
///</summary>
///<returns>none</returns>
void BlockBlobBfsClient::UploadFromStream(std::istream &sourceStream, const std::string blobName)
{
    m_blob_client->upload_block_blob_from_stream(configurations.containerName, blobName, sourceStream);
}

void BlockBlobBfsClient::UploadFromStream(std::istream &sourceStream, const std::string blobName,
                                          std::vector<std::pair<std::string, std::string>> &metadata)
{
    m_blob_client->upload_block_blob_from_stream(configurations.containerName, blobName, sourceStream, metadata);
}

///<summary>
/// Downloads contents of a block blob to a local file
///</summary>
///<returns>none</returns>
long int BlockBlobBfsClient::DownloadToFile(const std::string blobName, const std::string filePath, time_t &last_modified)
{
    m_blob_client->download_blob_to_file(configurations.containerName, blobName, filePath, last_modified);
    struct stat stbuf;
    lstat(filePath.c_str(), &stbuf);
    if (0 == stat(filePath.c_str(), &stbuf))
        return stbuf.st_size;
    else
        return 0;
}

long int BlockBlobBfsClient::DownloadToStream(const std::string blobName, std::ostream &destStream,
                                              unsigned long long offset, unsigned long long size)
{
    m_blob_client->download_blob_to_stream(configurations.containerName, blobName, offset, size, destStream);
    return 0;
}

///<summary>
/// Creates a Directory
///</summary>
///<returns>none</returns>
bool BlockBlobBfsClient::CreateDirectory(const std::string directoryPath)
{
    // There's no such thing as a "blob directory". We need to make a blob marker to represent that a directory exists
    // The blob marker is an empty blob with the metadata containing "hdi_isfolder=true"
    std::istringstream emptyDataStream("");
    std::vector<std::pair<std::string, std::string>> metadata;
    metadata.push_back(std::make_pair("hdi_isfolder", "true"));
    errno = 0;
    m_blob_client->upload_block_blob_from_stream(
        configurations.containerName,
        directoryPath,
        emptyDataStream,
        metadata);

    if (errno != 0)
    {
        int storage_errno = errno;
        syslog(LOG_ERR,
               "Failed to upload zero-length directory marker for path %s. errno = %d.\n",
               directoryPath.c_str(),
               storage_errno);
        return 0 - map_errno(errno);
    }
    else
    {
        syslog(LOG_INFO,
               "Successfully uploaded zero-length directory marker for path %s.",
               directoryPath.c_str());
    }
    return true;
}
///<summary>
/// Deletes a Directory
///</summary>
///<returns>none</returns>
bool BlockBlobBfsClient::DeleteDirectory(const std::string directoryPath)
{
    // There's no such thing as a "blob directory". When a directory is created through blobfuse or another process
    // it makes an empty blob marker that represents a directory through a empty blob marker and metadata set to
    // "hdi_isfolder"

    errno = 0;
    D_RETURN_CODE dir_blob_exists = IsDirectoryEmpty(directoryPath);

    if ((errno != 0) && (errno != 404) && (errno != ENOENT))
    {
        syslog(LOG_ERR,
               "Failed to list blobs in a directory to determine if a directory is empty: %s. errno = %d.\n",
               directoryPath.c_str(),
               errno);
        return false; // Failure in fetching properties - errno set by blob_exists
    }
    switch (dir_blob_exists)
    {
    case D_NOTEXIST:
        //log that the directory does not exist
        syslog(LOG_ERR,
               "Directory does not exist in storage, no directory to delete: %s errno = %d\n",
               directoryPath.c_str(),
               errno);
        errno = ENOENT;
        return false;
        break;
    case D_EMPTY:
        syslog(LOG_DEBUG,
               "Directory is empty, attempting deleting directory marker: %s\n",
               directoryPath.c_str());
        DeleteFile((std::string)directoryPath);
        return true;
        break;
    case D_NOTEMPTY:
        syslog(LOG_ERR,
               "Directory is not empty, cannot delete: %s errno = %d\n",
               directoryPath.c_str(),
               errno);
        errno = ENOTEMPTY;
        return false;
        break;
    default:
        // Unforseen error,syslog and return false at the end of function
        syslog(LOG_ERR,
               "Unforseen error for deleting directory: %s errno = %d\n",
               directoryPath.c_str(),
               errno);
        break;
    }
    return false;
}
///<summary>
/// Deletes a File
///</summary>
///<returns>none</returns>
void BlockBlobBfsClient::DeleteFile(const std::string pathToDelete)
{
    m_blob_client->delete_blob(configurations.containerName, pathToDelete);
}
///<summary>
/// Gets the properties of a path
///</summary>
///<returns>BfsFileProperty object which contains the property details of the file</returns>
BfsFileProperty BlockBlobBfsClient::GetProperties(std::string pathName, bool type_known)
{
    errno = 0;
    if (type_known) {
        blob_property property = m_blob_client->get_blob_property(configurations.containerName, pathName);
        if (errno == 0) {
            BfsFileProperty ret_property(
                property.copy_status,
                property.metadata,
                property.last_modified,
                "", // Return an empty modestring because blob doesn't support file mode bits.
                property.size);

            return ret_property;
        }
    } else {
        int resultCount = 2;
        std::vector<std::pair<std::vector<list_segmented_item>, bool>> listResponse;
        ListAllItemsSegmented(pathName, "/", listResponse, resultCount);

        if (errno == 0 && listResponse.size() > 0)
        {
            list_segmented_item blobItem;
            unsigned int batchNum = 0;
            unsigned int resultStart = 0;
            // this variable will be incremented below if it is a directory, otherwise it will not be used.
            unsigned int dirSize = 0;

            for (batchNum = 0; batchNum < listResponse.size(); batchNum++)
            {
                // if skip_first start the listResults at 1
                resultStart = listResponse[batchNum].second ? 1 : 0;

                std::vector<list_segmented_item> listResults = listResponse[batchNum].first;
                for (unsigned int i = resultStart; i < listResults.size(); i++)
                {
                    syslog(LOG_ERR,"In GetProperties list_segmented_item %d file %s\n", i, listResults[i].name.c_str());

                    // if the path for exact name is found the dirSize will be 1 here so check to see if it has files or subdirectories inside
                    // match dir name or longer paths to determine dirSize
                    if (listResults[i].name.compare(pathName + '/') < 0)
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
                    if (blobItem.name.empty() && (listResults[i].name == pathName || listResults[i].name == (pathName + '/')))
                    {
                        blobItem = listResults[i];
                        syslog(LOG_ERR,"In GetProperties found blob in list hierarchical file %s\n", blobItem.name.c_str());
                        // leave 'i' at the value it is, it will be used in the remaining batches and loops to check for directory empty check.
                        if (dirSize == 0 && (is_directory_blob(0, blobItem.metadata) || blobItem.is_directory || blobItem.name == (pathName + '/')))
                        {
                            dirSize = 1; // root directory exists so 1
                        }
                    }
                }
            }

            if (!blobItem.name.empty() && (is_directory_blob(0, blobItem.metadata) || blobItem.is_directory || blobItem.name == (pathName + '/')))
            {
                blob_property property;
                if (errno == 0) {
                    time_t last_mod = time(NULL);
                    if (!blobItem.last_modified.empty()) {
                        struct tm mtime;
                        char *ptr = strptime(blobItem.last_modified.c_str(), "%a, %d %b %Y %H:%M:%S", &mtime);
                        if (ptr)
                            last_mod = timegm(&mtime);
                    }
                    BfsFileProperty ret_property(
                        "",
                        blobItem.metadata,
                        last_mod,
                        "", // Return an empty modestring because blob doesn't support file mode bits.
                        0);
                    ret_property.is_directory = true;
                    if (dirSize <= 1)
                        ret_property.DirectoryIsEmpty();

                    errno = 0;
                    return ret_property;
                }
            }
            else if (!blobItem.name.empty())
            {
                blob_property property = m_blob_client->get_blob_property(configurations.containerName, pathName);
                if (errno == 0) {
                    BfsFileProperty ret_property(
                        property.copy_status,
                        property.metadata,
                        property.last_modified,
                        "", // Return an empty modestring because blob doesn't support file mode bits.
                        property.size);

                    errno = 0;
                    return ret_property;
                }
            }
            else // none of the blobs match exactly so blob not found
            {
                syslog(LOG_ERR,"%s does not match the exact name in the top 2 return from list_hierarchial_blobs. It will be treated as a new blob", pathName.c_str());
                //errno = ENOENT;
                BfsFileProperty cache_prop = BfsFileProperty(true);
                errno = 404;
                return cache_prop;
            }
        }
    }
    return BfsFileProperty();
}
///<summary>
/// Determines whether or not a path (file or directory) exists or not
///</summary>
///<returns>none</returns>
int BlockBlobBfsClient::Exists(const std::string pathName)
{
    errno = 0;
    blob_property property = m_blob_client->get_blob_property(configurations.containerName, pathName);

    if (errno != 0)
    {
        if (errno != 404 && (errno != ENOENT))
        {
            //failed to fetch properties
            return 0 - map_errno(errno);
        }
        // does not exist
        return 1;
    }
    //return 0 for success
    return 0;
}
///<summary>
/// Determines whether or not a path (file or directory) exists or not
///</summary>
///<returns>none</returns>
bool BlockBlobBfsClient::Copy(const std::string sourcePath, const std::string destinationPath)
{
    m_blob_client->start_copy(configurations.containerName, sourcePath, configurations.containerName, destinationPath);
    return true;
}
///<summary>
/// Renames a file
///</summary>
///<returns>List of files in the cache to remove</returns>
std::vector<std::string> BlockBlobBfsClient::Rename(const std::string sourcePath, const std::string destinationPath)
{
    // Rename the directory blob, if it exists.
    errno = 0;
    BfsFileProperty property = GetProperties(sourcePath.substr(1));
    std::vector<std::string> file_paths_to_remove;
    if (property.isValid() && property.exists() && property.is_directory)
    {
        rename_directory(sourcePath.c_str(), destinationPath.c_str(), file_paths_to_remove);
    }
    else
    {
        rename_single_file(sourcePath.c_str(), destinationPath.c_str(), file_paths_to_remove);
    }
    return file_paths_to_remove;
}

std::vector<std::string> BlockBlobBfsClient::Rename(const std::string sourcePath,const  std::string destinationPath, bool isDir)
{
    std::vector<std::string> file_paths_to_remove;
    if (isDir) {
        rename_directory(sourcePath.c_str(), destinationPath.c_str(), file_paths_to_remove);
    } else {
        rename_single_file(sourcePath.c_str(), destinationPath.c_str(), file_paths_to_remove);
    }
    return file_paths_to_remove;
}

///<summary>
/// Lists
///</summary>
///<returns>none</returns>
int
BlockBlobBfsClient::List(std::string continuation, const std::string prefix, const std::string delimiter, list_segmented_response &resp, int max_results)
{
    //TODO: MAKE THIS BETTER
    list_blobs_segmented_response listed_blob_response = m_blob_client->list_blobs_segmented(
        configurations.containerName,
        delimiter,
        continuation,
        prefix,
        max_results);

    if (errno == 0)
        resp.populate(listed_blob_response);
    
    listed_blob_response.blobs.clear();
    listed_blob_response.blobs.shrink_to_fit();

    return errno;
}

///<summary>
/// Checks if the blob is a directory
///</summary>
///<returns>none</returns>
bool BlockBlobBfsClient::IsDirectory(const char *path)
{
    BfsFileProperty property = GetProperties(path);
    if (property.isValid() && property.exists() && property.is_directory)
        return true;
    else
        return false;
}
/*
 * Check if the directory is empty or not by checking if there is any blob with prefix exists in the specified container.
 *
 * return
 *   - D_NOTEXIST if there's nothing there (the directory does not exist)
 *   - D_EMPTY is there's exactly one blob, and it's the ".directory" blob
 *   - D_NOTEMPTY otherwise (the directory exists and is not empty.)
 */
D_RETURN_CODE BlockBlobBfsClient::IsDirectoryEmpty(std::string path)
{
    std::string delimiter = "/";
    path.append(delimiter);
    std::string continuation;
    bool success = false;
    int failcount = 0;
    bool old_dir_blob_found = false;
    do
    {
        errno = 0;
        list_blobs_segmented_response response = m_blob_client->list_blobs_segmented(configurations.containerName, delimiter, continuation, path, 2);
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
                // A blob of the previous folder ".." could still exist, that does not count as the directory still has
                // any existing blobs
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
        // If we get a continuation token, and the blob size on the first or so calls is still empty, the service could
        // actually have blobs in the container, but they just didn't send them in the request, but they have a
        // continuation token so it means they could have some.
    } while ((!continuation.empty() || !success) && failcount < 20);

    if (!success)
    {
        // errno will be set by list_blobs_hierarchial if the last call failed and we're out of retries.
        return D_FAILED;
    }

    return D_EMPTY;
}

int BlockBlobBfsClient::rename_single_file(std::string src, std::string dst, std::vector<std::string> &files_to_remove_cache)
{
    // TODO: if src == dst, return?
    // TODO: lock in alphabetical order?
    auto fsrcmutex = file_lock_map::get_instance()->get_mutex(src);
    std::lock_guard<std::mutex> locksrc(*fsrcmutex);

    auto fdstmutex = file_lock_map::get_instance()->get_mutex(dst);
    std::lock_guard<std::mutex> lockdst(*fdstmutex);

    const char *srcMntPath;
    std::string srcMntPathString = prepend_mnt_path_string(src);
    srcMntPath = srcMntPathString.c_str();

    const char *dstMntPath;
    std::string dstMntPathString = prepend_mnt_path_string(dst);
    dstMntPath = dstMntPathString.c_str();

    struct stat buf;
    if (stat(srcMntPath, &buf) == 0)
    {
        AZS_DEBUGLOGV("Source file %s in rename operation exists in the local cache.\n", src.c_str());

        // The file exists in the local cache.  Call rename() on it (note this will preserve existing handles.)
        ensure_directory_path_exists_cache(dstMntPath);
        errno = 0;
        int renameret = rename(srcMntPath, dstMntPath);
        if (renameret < 0)
        {
            syslog(LOG_ERR, "Failure to rename source file %s in the local cache.  Errno = %d.\n", src.c_str(), errno);
            return -errno;
        }
        else
        {
            AZS_DEBUGLOGV("Successfully to renamed file %s to %s in the local cache.\n", src.c_str(), dst.c_str());
        }
    }

    errno = 0;
    auto blob_property = GetProperties(src.substr(1), true);
    if ((errno == 0) && blob_property.isValid() && blob_property.exists())
    {
        AZS_DEBUGLOGV("Source file %s for rename operation exists as a blob on the service.\n", src.c_str());
        // Blob also exists on the service.  Perform a server-side copy.
        errno = 0;
        Copy(src.substr(1), dst.substr(1));
        if (errno != 0)
        {
            int storage_errno = errno;
            syslog(LOG_ERR, "Attempt to call start_copy from %s to %s failed.  errno = %d\n.", src.c_str() + 1, dst.c_str() + 1, storage_errno);
            return 0 - map_errno(errno);
        }
        else
        {
            syslog(LOG_INFO, "Successfully called start_copy from blob %s to blob %s\n", src.c_str() + 1, dst.c_str() + 1);
        }

        errno = 0;
        do
        {
            blob_property = GetProperties(dst.substr(1), true);
        } while (errno == 0 && blob_property.isValid() && blob_property.exists() && blob_property.copy_status.compare(0, 7, "pending") == 0);

        if (blob_property.copy_status.compare(0, 7, "success") == 0)
        {
            syslog(LOG_INFO, "Copy operation from %s to %s succeeded.", src.c_str() + 1, dst.c_str() + 1);
            DeleteFile(src.substr(1));
            if (errno != 0)
            {
                int storage_errno = errno;
                syslog(LOG_ERR, "Failed to delete source blob %s during rename operation.  errno = %d\n.", src.c_str() + 1, storage_errno);
                return 0 - map_errno(storage_errno);
            }
            else
            {
                syslog(LOG_INFO, "Successfully deleted source blob %s during rename operation.\n", src.c_str() + 1);
            }
        }
        else
        {
            syslog(LOG_ERR, "Copy operation from %s to %s failed on the service.  Copy status = %s.\n", src.c_str() + 1, dst.c_str() + 1, blob_property.copy_status.c_str());
            return EFAULT;
        }

        // store the file in the cleanup list
        files_to_remove_cache.push_back(dst);

        return 0;
    }
    else if (errno != 0)
    {
        int storage_errno = errno;
        syslog(LOG_ERR, "Failed to get blob properties for blob %s during rename operation.  errno = %d\n", src.c_str() + 1, storage_errno);
        return 0 - map_errno(storage_errno);
    }
    
    return 0;
}

int BlockBlobBfsClient::rename_directory(std::string src, std::string dst, std::vector<std::string> &files_to_remove_cache)
{
    AZS_DEBUGLOGV("azs_rename_directory called with src = %s, dst = %s.\n", src.c_str(), dst.c_str());
    std::string srcPathStr(src);
    // Rename the directory blob, if it exists.
    errno = 0;
    BfsFileProperty property = GetProperties(srcPathStr.substr(1));
    if ((errno == 0) && (property.is_directory))
    {
        rename_single_file(src.c_str(), dst.c_str(), files_to_remove_cache);
    } 
    if (errno != 0)
    {
        if ((errno != 404) && (errno != ENOENT))
        {
            return 0 - map_errno(errno); // Failure in fetching properties - errno set by blob_exists
        }
    }

    errno = 0;
    if (src.size() > 1)
    {
        src.push_back('/');
    }
    if (dst.size() > 1)
    {
        dst.push_back('/');
    }
    std::vector<std::string> local_list_results;

    // Rename all files and directories that exist in the local cache.
    ensure_directory_path_exists_cache(prepend_mnt_path_string(dst + "placeholder"));
    std::string mntPathString = prepend_mnt_path_string(src);
    DIR *dir_stream = opendir(mntPathString.c_str());
    if (dir_stream != NULL)
    {
        struct dirent *dir_ent = readdir(dir_stream);
        while (dir_ent != NULL)
        {
            if (dir_ent->d_name[0] != '.')
            {
                int nameLen = strlen(dir_ent->d_name);
                char *newSrc = (char *)malloc(sizeof(char) * (src.size() + nameLen + 1));
                memcpy(newSrc, src.c_str(), src.size());
                memcpy(&(newSrc[src.size()]), dir_ent->d_name, nameLen);
                newSrc[src.size() + nameLen] = '\0';

                char *newDst = (char *)malloc(sizeof(char) * (dst.size() + nameLen + 1));
                memcpy(newDst, dst.c_str(), dst.size());
                memcpy(&(newDst[dst.size()]), dir_ent->d_name, nameLen);
                newDst[dst.size() + nameLen] = '\0';

                AZS_DEBUGLOGV("Local object found - about to rename %s to %s.\n", newSrc, newDst);
                if (dir_ent->d_type == DT_DIR) {
                    rename_directory(newSrc, newDst, files_to_remove_cache);
                } else {
                    rename_single_file(newSrc, newDst, files_to_remove_cache);
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
    std::vector<std::pair<std::vector<list_segmented_item>, bool>> listResults;
    ListAllItemsSegmented(src.substr(1), "/", listResults);

    if (errno != 0)
    {
        int storage_errno = errno;
        syslog(LOG_ERR, "list blobs operation failed during attempt to rename directory %s to %s.  errno = %d.\n", src.c_str(), dst.c_str(), storage_errno);
        return 0 - map_errno(storage_errno);
    }

    AZS_DEBUGLOGV("Total of %d result lists found from list_blobs call during rename operation\n.", (int)listResults.size());
    for (size_t result_lists_index = 0; result_lists_index < listResults.size(); result_lists_index++)
    {
        int start = listResults[result_lists_index].second ? 1 : 0;
        for (size_t i = start; i < listResults[result_lists_index].first.size(); i++)
        {
            // We need to parse out just the trailing part of the path name.
            int len = listResults[result_lists_index].first[i].name.size();
            if (len > 0)
            {
                std::string prev_token_str;
                if (listResults[result_lists_index].first[i].name.back() == '/')
                {
                    prev_token_str = listResults[result_lists_index].first[i].name.substr(src.size() - 1, listResults[result_lists_index].first[i].name.size() - src.size());
                }
                else
                {
                    prev_token_str = listResults[result_lists_index].first[i].name.substr(src.size() - 1);
                }

                // TODO: order or hash the list to improve perf
                if ((prev_token_str.size() > 0) && (std::find(local_list_results.begin(), local_list_results.end(), prev_token_str) == local_list_results.end()))
                {
                    int nameLen = prev_token_str.size();
                    char *newSrc = (char *)malloc(sizeof(char) * (src.size() + nameLen + 1));
                    memcpy(newSrc, src.c_str(), src.size());
                    memcpy(&(newSrc[src.size()]), prev_token_str.c_str(), nameLen);
                    newSrc[src.size() + nameLen] = '\0';

                    char *newDst = (char *)malloc(sizeof(char) * (dst.size() + nameLen + 1));
                    memcpy(newDst, dst.c_str(), dst.size());
                    memcpy(&(newDst[dst.size()]), prev_token_str.c_str(), nameLen);
                    newDst[dst.size() + nameLen] = '\0';

                    AZS_DEBUGLOGV("Object found on the service - about to rename %s to %s.\n", newSrc, newDst);
                    if (listResults[result_lists_index].first[i].is_directory) {
                        rename_directory(newSrc, newDst, files_to_remove_cache);
                    } else {
                        rename_single_file(newSrc, newDst, files_to_remove_cache);
                    }
                    free(newSrc);
                    free(newDst);
                }
            }
        }
    }
    //src.pop_back();
    DeleteDirectory(src.substr(1).c_str());
    return 0;
}

int BlockBlobBfsClient::ListAllItemsSegmented(
    const std::string &prefix,
    const std::string &delimiter,
    LISTALL_RES &results,
    int max_results)
{
    std::string continuation;
    std::string prior;
    bool success = false;
    int failcount = 0;
    uint total_count = 0;
    uint iteration = 0;
    list_segmented_response response;

    results.reserve(200);
    do
    {
        AZS_DEBUGLOGV("About to call list_blobs_hierarchial.  Container = %s, delimiter = %s, continuation = %s, prefix = %s\n",
                      configurations.containerName.c_str(),
                      delimiter.c_str(),
                      continuation.c_str(),
                      prefix.c_str());

        errno = 0;
        response.reset();
        List(continuation, prefix, delimiter, response, max_results);
        if (errno == 0)
        {
            success = true;
            failcount = 0;

            iteration++;
            total_count += response.m_items.size();
            
            AZS_DEBUGLOGV("Successful call to list_blobs_segmented.  results count = %d, next_marker = %s.\n", (int)response.m_items.size(), response.m_next_marker.c_str());
            AZS_DEBUGLOGV("#### So far %u items retreived in %u iterations.\n", total_count, iteration);
            

            continuation = response.m_next_marker;
            if (!response.m_items.empty())
            {
                bool skip_first = false;
                if (response.m_items[0].name == prior)
                {
                    skip_first = true;
                }
                prior = response.m_items.back().name;
                results.emplace_back(response.m_items, skip_first);
            }
        }
        else if (errno == 404)
        {
            success = true;
            syslog(LOG_WARNING, "list_blobs_segmented indicates blob not found");
        }
        else
        {
            failcount++;
            success = false;
            syslog(LOG_WARNING, "list_blobs_segmented failed for the %d time with errno = %d.\n", failcount, errno);
        }
    } while (((!continuation.empty()) || !success) && (failcount < maxFailCount));

    // errno will be set by list_blobs_hierarchial if the last call failed and we're out of retries.
    return errno;
}

///<summary>
/// Helper function - Checks metadata hdi_isfolder aka if the blob marker is a folder
///</summary>
bool BlockBlobBfsClient::is_folder(const std::vector<std::pair<std::string, std::string>> &metadata)
{
    for (auto iter = metadata.begin(); iter != metadata.end(); ++iter)
    {
        if ((iter->first.compare("hdi_isfolder") == 0) && (iter->second.compare("true") == 0))
        {
            return true;
        }
    }
    return false;
}

int BlockBlobBfsClient::ChangeMode(const char *, mode_t)
{
    return -ENOSYS;
}

int BlockBlobBfsClient::UpdateBlobProperty(std::string /*pathStr*/, std::string /*key*/, std::string /*value*/, METADATA * /*metadata*/)
{
    //  This is not supported for block blob for now
    return 0;
}

void BlockBlobBfsClient::GetExtraProperties(const std::string /*pathName*/, BfsFileProperty & /*prop*/)
{
    return;
}