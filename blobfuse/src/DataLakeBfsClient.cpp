#include <fstream>
#include <include/Permissions.h>
#include <sys/stat.h>
#include <DataLakeBfsClient.h>
#include <list_paths_request.h>
#include <DfsProperties.h>

///<summary>
/// Uploads contents of a file to a storage object(e.g. blob, file) to the Storage service
///</summary>
///TODO: params
///<returns>none</returns>
bool DataLakeBfsClient::AuthenticateStorage()
{
    // Authenticate the storage account
    switch (configurations.authType) {
        case KEY_AUTH:
            m_adls_client = authenticate_adls_accountkey();
            m_blob_client = authenticate_blob_accountkey();
            break;
        case SAS_AUTH:
            m_adls_client = authenticate_adls_sas();
            m_blob_client = authenticate_blob_sas();
            break;
        case MSI_AUTH:
            m_adls_client = authenticate_adls_msi();
            m_blob_client = authenticate_blob_msi();
            break;
        case SPN_AUTH:
            m_adls_client = authenticate_adls_spn();
            m_blob_client = authenticate_blob_spn();
            break;
        default:
            return false;
            break;
    }

    if(m_adls_client != NULL)
    {
        //Authenticate the storage container by using a list call
        m_adls_client->list_paths_segmented(
                configurations.containerName,
                "/",
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

std::shared_ptr<adls_client_ext> DataLakeBfsClient::authenticate_adls_accountkey()
{
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
            return NULL;
        }
        errno = 0;
        std::shared_ptr<storage_account> account = std::make_shared<storage_account>(
                configurations.accountName,
                cred,
                configurations.useHttps,
                configurations.blobEndpoint);
        return std::make_shared<adls_client_ext>(
                account,
                configurations.concurrency,
                false); //If this applies to blobs in the future, we can use this as a feature to exit
                                // blobfuse if we run into anything unexpected instead of logging errors
    }
    catch(const std::exception &ex)
    {
        syslog(LOG_ERR, "Failed to create blob client.  ex.what() = %s.", ex.what());
        errno = blobfuse_constants::unknown_error;
        return NULL;
    }
}
std::shared_ptr<adls_client_ext> DataLakeBfsClient::authenticate_adls_sas()
{
    try
    {
        std::shared_ptr<storage_credential> cred;
        if(configurations.sasToken.length() > 0)
        {
            cred = std::make_shared<shared_access_signature_credential>(configurations.sasToken);
        }
        else
        {
            syslog(LOG_ERR, "Empty account key. Failed to create blob client.");
            return NULL;
        }
        errno = 0;
        std::shared_ptr<storage_account> account = std::make_shared<storage_account>(
                configurations.accountName, cred,
                configurations.useHttps,
                configurations.blobEndpoint);
        return std::make_shared<adls_client_ext>(
                account,
                configurations.concurrency,
                false); //If this applies to blobs in the future, we can use this as a feature to exit
                                // blobfuse if we run into anything unexpected instead of logging errors
    }
    catch(const std::exception &ex)
    {
        syslog(LOG_ERR, "Failed to create blob client.  ex.what() = %s.", ex.what());
        errno = blobfuse_constants::unknown_error;
        return NULL;
    }
}
std::shared_ptr<adls_client_ext> DataLakeBfsClient::authenticate_adls_msi() {
    try {
        //1. get oauth token
        std::function<OAuthToken(std::shared_ptr<CurlEasyClient>)> MSICallback = SetUpMSICallback(
                configurations.identityClientId,
                configurations.objectId,
                configurations.resourceId,
                configurations.msiEndpoint);

        std::shared_ptr<OAuthTokenCredentialManager> tokenManager = GetTokenManagerInstance(MSICallback);

        if (!tokenManager->is_valid_connection()) {
            // todo: isolate definitions of errno's for this function so we can output something meaningful.
            errno = 1;
            return NULL;
        }

        //2. try to make blob client wrapper using oauth token
        errno = 0;
        std::shared_ptr<storage_credential> cred = std::make_shared<token_credential>("");
        std::shared_ptr<storage_account> account = std::make_shared<storage_account>(
                configurations.accountName,
                cred,
                true, //use_https must be true to use oauth
                configurations.blobEndpoint);
        return std::make_shared<adls_client_ext>(
                account,
                configurations.concurrency,
                false); //If this applies to blobs in the future, we can use this as a feature to exit
        // blobfuse if we run into anything unexpected instead of logging errors

    }
    catch (const std::exception &ex) {
        syslog(LOG_ERR, "Failed to create blob client.  ex.what() = %s. Please check your account name and ",
               ex.what());
        errno = blobfuse_constants::unknown_error;
        return NULL;
    }
}
std::shared_ptr<adls_client_ext> DataLakeBfsClient::authenticate_adls_spn()
{
    syslog(LOG_DEBUG, "Authenticating using MSI");
    try
    {
        //1. get oauth token
        std::function<OAuthToken(std::shared_ptr<CurlEasyClient>)> SPNCallback = SetUpSPNCallback(
                configurations.spnTenantId,
                configurations.spnClientId,
                configurations.spnClientSecret,
                configurations.aadEndpoint);

        std::shared_ptr<OAuthTokenCredentialManager> tokenManager = GetTokenManagerInstance(SPNCallback);

        if (!tokenManager->is_valid_connection()) {
            // todo: isolate definitions of errno's for this function so we can output something meaningful.
            errno = 1;
            return NULL;
        }

        //2. try to make blob client wrapper using oauth token
        std::shared_ptr<storage_credential> cred = std::make_shared<token_credential>("");
        std::shared_ptr<storage_account> account = std::make_shared<storage_account>(
                configurations.accountName,
                cred,
                true, //use_https must be true to use oauth
                configurations.blobEndpoint);
        errno = 0;
        return std::make_shared<adls_client_ext>(
                account,
                configurations.concurrency,
                false); //If this applies to blobs in the future, we can use this as a feature to exit
        // blobfuse if we run into anything unexpected instead of logging errors

    }
    catch(const std::exception &ex)
    {
        syslog(LOG_ERR, "Failed to create blob client.  ex.what() = %s. Please check your account name and ", ex.what());
        errno = blobfuse_constants::unknown_error;
        return NULL;
    }
}
///<summary>
/// Creates a Directory
///</summary>
///<returns>none</returns>
bool DataLakeBfsClient::CreateDirectory(const std::string directoryPath)
{
    // We could call the block blob CreateDirectory instead but that would require making the metadata with hdi_isfolder
    // it's easier if we let the service do that for us
    errno = 0;
    m_adls_client->create_directory(configurations.containerName, directoryPath);
    if(errno != 0)
    {
        return false;
    }
    return true;
}
///<summary>
/// Deletes a Directory
///</summary>
///<returns>none</returns>
bool DataLakeBfsClient::DeleteDirectory(const std::string directoryPath)
{
    errno = 0;
    m_adls_client->delete_directory(configurations.containerName, directoryPath);
    if(errno != 0)
    {
        return false;
    }
    return true;
}

///<summary>
/// Helper function - Checks if the "directory" blob is empty
///</summary>
D_RETURN_CODE DataLakeBfsClient::IsDirectoryEmpty(std::string path)
{
    bool success = false;
    bool old_dir_blob_found = false;
    int failcount = 0;
    std::string continuation = "";
    do{
        errno = 0;
        list_paths_result pathsResult = m_adls_client->list_paths_segmented(
                configurations.containerName,
                path,
                false,
                std::string(),
                2);
        if(errno == 0)
        {
            success = true;
            failcount = 0;
            continuation = pathsResult.continuation_token;
            if (pathsResult.paths.size() > 1) {
                return D_NOTEMPTY;
            }
            if (pathsResult.paths.size() > 0)
            {
                // A blob of the previous folder ".." could still exist, that does not count as the directory still has
                // any existing blobs
                if ((!old_dir_blob_found) &&
                    (!pathsResult.paths[0].is_directory) &&
                    (pathsResult.paths[0].name.size() > former_directory_signifier.size()) &&
                    (0 == pathsResult.paths[0].name.compare(
                            pathsResult.paths[0].name.size() - former_directory_signifier.size(),
                             former_directory_signifier.size(),
                             former_directory_signifier)))
                {
                    old_dir_blob_found = true;
                } else
                    {
                    return D_NOTEMPTY;
                }
            }
        }
        else
        {
            success = false;
            failcount++;
        }
    }
    // If we get a continuation token, and the blob size on the first or so calls is still empty, the service could
    // actually have blobs in the container, but they just didn't send them in the request, but they have a
    // continuation token so it means they could have some.
    while ((continuation.size() > 0 || !success) && failcount < 20);

    if(!success)
    {
        return D_FAILED;
    }
    return old_dir_blob_found ? D_EMPTY : D_NOTEMPTY;
}

///<summary>
/// Renames a file/directory
///</summary>
///<returns></returns>
std::vector<std::string> DataLakeBfsClient::Rename(std::string sourcePath, std::string destinationPath)
{
    m_adls_client->move_file(
            configurations.containerName,
            sourcePath.substr(1),
            configurations.containerName,
            destinationPath.substr(1));

    std::vector<std::string> file_paths_to_remove;
    std::string srcMntPathString = prepend_mnt_path_string(sourcePath);
    std::string dstMntPathString = prepend_mnt_path_string(destinationPath);

    long int rename_ret = rename_cached_file(srcMntPathString.c_str(), dstMntPathString.c_str());

    if(rename_ret != 0)
    {
        syslog(LOG_ERR, "Failure to rename source file %s in the local cache.  Errno = %d.\n", sourcePath.c_str(), errno);
    }
    file_paths_to_remove.push_back(sourcePath);
    return file_paths_to_remove;
}

list_segmented_response DataLakeBfsClient::List(std::string continuation, std::string prefix, std::string delimiter)
{
    syslog(LOG_DEBUG, "Calling List Paths, continuation:%s, prefix:%s, delimiter:%s\n",
            continuation.c_str(),
            prefix.c_str(),
            delimiter.c_str());
    list_paths_result listed_adls_response = m_adls_client->list_paths_segmented(
            configurations.containerName,
            prefix,
            false,
            continuation,
            10000);
    return list_segmented_response(listed_adls_response);

}

int DataLakeBfsClient::ChangeMode(const char *path, mode_t mode) {
    // TODO: Once ADLS works in blobfuse, verify that we don't need to get the access
    std::string pathStr(path);
    access_control accessControl;
    accessControl.acl = modeToString(mode);
    int lstaterrno = 0;

    errno = 0;
    m_adls_client->set_file_access_control(configurations.containerName, pathStr.substr(1), accessControl);
    lstaterrno = errno;

    UpdateBlobProperty(pathStr.substr(1), "last_change", std::to_string(time(NULL)));

    std::string mntPathString = prepend_mnt_path_string(pathStr);
    int acc = access(mntPathString.c_str(), F_OK);
    if (acc != -1)
    {
        // if path exists in local cache, then update permissions, if not no need to update it
        AZS_DEBUGLOGV("Accessing mntPath = %s for chmod succeeded; object is in the local cache.\n", mntPathString.c_str());

        int res = chmod(mntPathString.c_str(), mode);
        if (res == -1)
        {
            lstaterrno = errno;
            syslog(LOG_ERR, "chmod on file %s in local cache during getattr failed with errno = %d.\n", mntPathString.c_str(), lstaterrno);
            return -lstaterrno;
        }
        else
        {
            AZS_DEBUGLOGV("chmod on file %s in local cache succeeded.\n", mntPathString.c_str());
            return 0;
        }
    }

    return ((lstaterrno) ? (-lstaterrno) : 0);
}

BfsFileProperty DataLakeBfsClient::GetProperties(std::string pathName) {
    BfsFileProperty cache_prop;
    if (0 == GetCachedProperty(pathName, cache_prop)) {
        return cache_prop;
    }

    dfs_properties dfsprops =
            m_adls_client->get_dfs_path_properties(configurations.containerName, pathName);

    BfsFileProperty ret_property(
            dfsprops.cache_control,
            dfsprops.content_disposition,
            dfsprops.content_encoding,
            dfsprops.content_language,
            dfsprops.content_md5,
            dfsprops.content_type,
            dfsprops.etag,
            dfsprops.resource_type,
            dfsprops.owner,
            dfsprops.group,
            dfsprops.permissions,
            dfsprops.metadata,
            dfsprops.last_modified,
            dfsprops.permissions,
            dfsprops.content_length
            );

    SetCachedProperty(pathName, ret_property);

    return ret_property;
}

long int DataLakeBfsClient::rename_cached_file(std::string src, std::string dst)
{
    struct stat buf;

    int statret = stat(src.c_str(), &buf);
    if(statret == 0)
    {
        //source file/directory exists in cache locally
        if((buf.st_mode & S_IFMT) == S_IFREG ||
           ((buf.st_mode & S_IFMT) == S_IFLNK))
        {
            //make sure directory path exists in cache
            ensure_directory_path_exists_cache(dst.c_str());
        }
        errno = 0;
        int rename_ret = rename(src.c_str(), dst.c_str());
        if(rename_ret < 0)
        {
            syslog(LOG_ERR, "Failure to rename source %s in the local cache. errno = %d\n", src.c_str(), errno);
            return -errno;
        }
        else
        {
            AZS_DEBUGLOGV("Successfully to renamed file %s to %s in the local cache.\n", src.c_str(), dst.c_str());
        }
    }
    else
    {
        AZS_DEBUGLOGV("Failure to find source %s in the local cache. errno = %d\n", src.c_str(), errno);
        //we don't have to rename if the file or directory does not exist in the cache
        return 0;
    }
    return buf.st_size;
}


int DataLakeBfsClient::UpdateBlobProperty(std::string pathStr, std::string key, std::string value, METADATA *metadata)
{
    errno = 0;
    BfsFileProperty blob_property = GetProperties(pathStr);
    if (errno) {
        AZS_DEBUGLOGV("Failed to get property before update for %s : err %d", pathStr.c_str(), errno);
        return errno;
    }

    bool updated = false;
    if (!key.empty() && !value.empty())
    {
        for (auto iter = blob_property.metadata.begin(); iter != blob_property.metadata.end(); ++iter)
        {
            if (iter->first.compare(key.c_str()) == 0) {
                iter->second = value;
                updated = true;
            }
        }

        if (!updated) {
            blob_property.metadata.push_back(std::make_pair(key, value));
            updated = true;
        }
    }
    
    if (metadata)
        *metadata = blob_property.metadata;

    errno = 0;
    if (updated) {
        m_adls_client->set_file_properties(configurations.containerName, pathStr, blob_property.metadata);
        if (errno) {
            AZS_DEBUGLOGV("Failed to update property for %s : err %d", pathStr.c_str(), errno);
        }
    }
    InvalidateCachedProperty(pathStr);
    return errno;
}