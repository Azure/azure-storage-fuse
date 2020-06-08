#ifndef STORAGEBFSCLIENTBASE_H
#define STORAGEBFSCLIENTBASE_H

#include <blobfuse.h>
#include <list_blobs_request_base.h>
#include <list_paths_request.h>
#include <adls_client.h>

#include "file_lock_map.h"
#include "storage_errno.h"

using namespace azure::storage_lite;
using namespace azure::storage_adls;

struct BfsFileProperty
{
    BfsFileProperty() : m_valid(false) {}
    BfsFileProperty(std::string cache_control,
                std::string content_disposition,
                std::string content_encoding,
                std::string content_language,
                std::string content_md5,
                std::string content_type,
                std::string etag,
                std::string copy_status,
                std::vector<std::pair<std::string, std::string>> metadata,
                time_t last_modified,
                std::string modestring,
                unsigned long long size) :
                m_cache_control(cache_control),
                m_content_disposition(content_disposition),
                m_content_encoding(content_encoding),
                m_content_language(content_language),
                m_content_md5(content_md5),
                m_content_type(content_type),
                m_etag(etag),
                m_copy_status(copy_status),
                m_metadata(metadata),
                m_last_modified(last_modified),
                m_size(size),
                m_valid(true)
    {
        // This is mainly used in the Blob Client
        if (!modestring.empty())
        {
            m_file_mode = 0000; // Supply no file mode to begin with unless the mode string is empty
            for (char & c : modestring) {
                // Start by pushing back the mode_t.
                m_file_mode = m_file_mode << 1; // NOLINT(hicpp-signed-bitwise) (mode_t is signed, apparently. Suppress the inspection.)
                // Then flip the new bit based on whether the mode is enabled or not.
                // This works because we can expect a consistent 9 character modestring.
                m_file_mode |= (c != '-');
            }
        }
        else
        {
            m_file_mode = 0;
        }

        is_directory = false;
        if (size == 0)
        {
            for (auto iter = metadata.begin(); iter != metadata.end(); ++iter)
            {
                if ((iter->first.compare("hdi_isfolder") == 0) && (iter->second.compare("true") == 0))
                {
                    is_directory = true;
                }
            }
        }
    }
    BfsFileProperty(std::string cache_control,
                    std::string content_disposition,
                    std::string content_encoding,
                    std::string content_language,
                    std::string content_md5,
                    std::string content_type,
                    std::string etag,
                    std::string resource_type,
                    std::string owner,
                    std::string group,
                    std::string permissions,
                    std::vector<std::pair<std::string, std::string>> metadata,
                    time_t last_modified,
                    std::string modestring,
                    unsigned long long size) :
            m_cache_control(cache_control),
            m_content_disposition(content_disposition),
            m_content_encoding(content_encoding),
            m_content_language(content_language),
            m_content_md5(content_md5),
            m_content_type(content_type),
            m_etag(etag),
            m_copy_status(""),
            m_owner(owner),
            m_group(group),
            m_permissions(permissions),
            m_metadata(metadata),
            m_last_modified(last_modified),
            m_size(size),
            m_valid(true)
    {
        //This is mainly used in the ADLS client
        if (!modestring.empty())
        {
            m_file_mode = 0000; // Supply no file mode to begin with unless the mode string is empty
            for (char & c : modestring) {
                // Start by pushing back the mode_t.
                m_file_mode = m_file_mode << 1; // NOLINT(hicpp-signed-bitwise) (mode_t is signed, apparently. Suppress the inspection.)
                // Then flip the new bit based on whether the mode is enabled or not.
                // This works because we can expect a consistent 9 character modestring.
                m_file_mode |= (c != '-');
            }
        }
        else
        {
            m_file_mode = 0;
        }
        if(resource_type == "directory")
        {
            is_directory = true;
        }
        else
        {
            is_directory = false;
            if (size == 0)
            {
                for (auto iter = metadata.begin(); iter != metadata.end(); ++iter)
                {
                    if ((iter->first.compare("hdi_isfolder") == 0) && (iter->second.compare("true") == 0))
                    {
                        is_directory = true;
                    }
                }
            }
        }
    }

    std::string m_cache_control;
    std::string m_content_disposition;
    std::string m_content_encoding;
    std::string m_content_language;
    std::string m_content_md5;
    std::string m_content_type;
    std::string m_etag;
    std::string m_copy_status;
    std::string m_owner;
    std::string m_group;
    std::string m_permissions;
    std::vector<std::pair<std::string, std::string>> m_metadata;
    time_t m_last_modified;
    mode_t m_file_mode;
    unsigned long long m_size;
    bool is_directory;
    bool m_valid;

    bool isValid()
    {
        return m_valid;
    }

    unsigned long long size()
    {
        return m_size;
    }

    time_t last_modified()
    {
        return m_last_modified;
    }


};

struct list_segmented_item {
    list_segmented_item(list_blobs_segmented_item);
    list_segmented_item(list_paths_item item);
    std::string name;
    std::string snapshot;
    std::string last_modified;
    std::string etag;
    unsigned long long content_length;
    std::string content_encoding;
    std::string content_type;
    std::string content_md5;
    std::string content_language;
    std::string cache_control;
    std::string copy_status;
    std::vector<std::pair<std::string, std::string>> metadata;
    access_control acl;
    mode_t mode;
    bool is_directory;
};

struct list_segmented_response {
    list_segmented_response() : m_valid(false) {}
    list_segmented_response(list_blobs_segmented_response response);
    list_segmented_response(list_paths_result response);
    std::string m_ms_request_id;
    std::vector<list_segmented_item> m_items;
    std::string m_next_marker;
    std::string continuation_token;
    bool m_valid;
};

class StorageBfsClientBase
{
public:
    StorageBfsClientBase(configParams opt) : configurations(opt)
    {}
    ///<summary>
    /// Authenticates the storage account and container
    ///</summary> 
    ///<returns>bool: if we authenticate to the storage account and container successfully</returns>
    virtual bool AuthenticateStorage() = 0;
    ///<summary>
    /// Uploads contents of a file to a storage object(e.g. blob, file) to the Storage service
    ///</summary>
    ///TODO: params
    ///<returns>none</returns>
    virtual void UploadFromFile(const std::string localPath) = 0;
    ///<summary>
    /// Uploads contents of a stream to a storage object(e.g. blob, file) to the Storage service
    ///</summary>
    ///<returns>none</returns>
    virtual void UploadFromStream(std::istream & sourceStream, const std::string blobName) = 0;
    ///<summary>
    /// Downloads contents of a storage object(e.g. blob, file) to a local file
    ///</summary>
    ///<returns>none</returns>
    virtual void DownloadToFile(const std::string blobName, const std::string filePath) = 0;
    ///<summary>
    /// Creates a Directory
    ///</summary>
    ///<returns>none</returns>
    virtual bool CreateDirectory(const std::string directoryPath) = 0;
    ///<summary>
    /// Deletes a Directory
    ///</summary>
    ///<returns>none</returns>
    virtual bool DeleteDirectory(const std::string directoryPath) = 0;
    ///<summary>
    /// Checks if the blob is a directory
    ///</summary>
    ///<returns>none</returns>
    virtual bool IsDirectory(const char * path) = 0;
    ///<summary>
    /// Helper function - Checks if the "directory" blob is empty
    ///</summary>
    virtual D_RETURN_CODE IsDirectoryEmpty(std::string path) = 0;
    ///<summary>
    /// Deletes a File
    ///</summary>
    ///<returns>none</returns>
    virtual void DeleteFile(const std::string pathToDelete) = 0;
    ///<summary>
    /// Determines whether or not a path (file or directory) exists or not
    ///</summary>
    ///<returns>none</returns>
    virtual int Exists(const std::string pathName) = 0;
    ///<summary>
    /// Gets the properties of a path
    ///</summary>
    ///<returns>BfsFileProperty object which contains the property details of the file</returns>
    virtual BfsFileProperty GetProperties(const std::string pathName) = 0;
    ///<summary>
    /// Determines whether or not a path (file or directory) exists or not
    ///</summary>
    ///<returns>none</returns>
    virtual bool Copy(const std::string sourcePath, const std::string destinationPath) = 0;
    ///<summary>
    /// Renames a file
    ///</summary>
    ///<returns>none</returns>
    virtual std::vector<std::string> Rename(const std::string sourcePath,const  std::string destinationPath) = 0;
    ///<summary>
    /// Lists
    ///</summary>
    ///<returns>none</returns>
    virtual list_segmented_response List(std::string continuation, const std::string prefix, const std::string delimiter) = 0;
    ///<summary>
    /// LIsts all directories within a list container
    /// Greedily list all blobs using the input params.
    ///</summary>
    virtual std::vector<std::pair<std::vector<list_segmented_item>, bool>> ListAllItemsHierarchical(const std::string& prefix, const std::string& delimiter) = 0;
    ///<summary>
    /// Updates the UNIX-style file mode on a path.
    ///</summary>
    virtual int ChangeMode(const char* path, mode_t mode) = 0;

protected:
    configParams configurations;
    ///<summary>
    /// Helper function - To map errno
    ///</summary>
    int map_errno(int error);
    ///<summary>
    /// Helper function - To append root foolder to ache to cache folder
    ///</summary>
    std::string prepend_mnt_path_string(const std::string& path);
    ///<summary>
    /// Helper function - Ensures directory path exists in the cache
    /// TODO: refactoring, rename variables and add comments to make sense to parsing
    ///</summary>
    int ensure_directory_path_exists_cache(const std::string & file_path);
};

#endif