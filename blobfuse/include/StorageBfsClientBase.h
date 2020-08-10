#ifndef STORAGEBFSCLIENTBASE_H
#define STORAGEBFSCLIENTBASE_H

#include <blobfuse.h>
#include <BlobfuseGlobals.h>
#include <get_blob_property_request_base.h>
#include <list_blobs_request_base.h>
#include <list_paths_request.h>
#include <blob/blob_client.h>
#include <adls_client.h>

#include <FileLockMap.h>
#include "storage_errno.h"

using namespace azure::storage_lite;
using namespace azure::storage_adls;

static const int maxFailCount = 20;
#define UNUSED(x) (void)(x)

class BfsFileProperty 
{
    public:
        BfsFileProperty() : m_valid(false), m_not_exists(false), m_empty_dir(false) {}
        BfsFileProperty(bool not_exists) : m_valid(true), m_not_exists(not_exists), m_empty_dir(false) 
        {
            size = 0;
            last_modified = 0;
            m_file_mode = 0;
            is_directory = false;
            last_access = last_change = last_modified;
        }

        BfsFileProperty(
                std::string copyStatus,
                std::vector<std::pair<std::string, std::string>> metaData,
                time_t lastModified,
                std::string modestring,
                unsigned long long Size) :
                m_valid(true)
        {
            copy_status = copyStatus;
            metadata = metaData;
            last_modified = lastModified;
            size = Size;
            last_access = last_modified;
            last_change = last_modified;
            m_not_exists = false;
            m_empty_dir = false;

            SetFileMode(modestring);

            is_directory = false;
            for (auto iter = metadata.begin(); iter != metadata.end(); ++iter)
            {
                if ((iter->first.compare("hdi_isfolder") == 0) && (iter->second.compare("true") == 0))
                {
                    is_directory = true;
                    continue;
                }

                /*
                if (iter->first.compare("last_access") == 0)
                {
                    last_access = std::stoi(iter->second.c_str());
                    continue;
                }

                if (iter->first.compare("last_change") == 0)
                {
                    last_change = std::stoi(iter->second.c_str());
                    continue;
                }
                */
            }
        }

        BfsFileProperty(
                    std::string resourceType,
                    std::string Owner,
                    std::string Group,
                    std::string Permissions,
                    std::vector<std::pair<std::string, std::string>> metaData,
                    time_t lastModified,
                    std::string modestring,
                    unsigned long long Size) :
            m_valid(true)
        {
            copy_status = "";
            m_owner = Owner;
            m_group = Group;
            m_permissions = Permissions;
            metadata = metaData;
            last_modified = lastModified;
            size = Size;

            is_directory = false;
            last_access = last_modified;
            last_change = last_modified;
            m_not_exists = false;
            m_empty_dir = false;

            SetFileMode(modestring);
           
            if(resourceType == "directory")
            {
                is_directory = true;
            }

            for (auto iter = metadata.begin(); iter != metadata.end(); ++iter)
            {
                if ((iter->first.compare("hdi_isfolder") == 0) && (iter->second.compare("true") == 0))
                {
                    is_directory = true;
                    continue;
                }
                /*
                if (iter->first.compare("last_access") == 0)
                {
                    last_access = std::stoi(iter->second.c_str());
                    continue;
                }

                if (iter->first.compare("last_change") == 0)
                {
                    last_change = std::stoi(iter->second.c_str());
                    continue;
                }
                */
            }
        }

        bool isValid()
        {
            return m_valid;
        }

        bool IsDirectoryEmpty()
        {
            return m_empty_dir;
        }

        void DirectoryIsEmpty()
        {
            m_empty_dir = true;
        }

        void SetFileMode(mode_t mode) 
        {
            m_file_mode = mode;
        }

        void SetFileMode(std::string modestring) 
        {
            m_file_mode = 0000; // Supply no file mode to begin with unless the mode string is empty
            //This is mainly used in the ADLS client
            
            if (!modestring.empty())
            {
               
                for (char & c : modestring) {
                    // Start by pushing back the mode_t.
                    m_file_mode = m_file_mode << 1; // NOLINT(hicpp-signed-bitwise) (mode_t is signed, apparently. Suppress the inspection.)
                    // Then flip the new bit based on whether the mode is enabled or not.
                    // This works because we can expect a consistent 9 character modestring.
                    m_file_mode |= (c != '-');
                }
            }
        }

        bool exists()
        {
            return !m_not_exists;
        }

        unsigned long long get_size()
        {
            return size;
        }

        time_t get_last_modified()
        {
            return last_modified != 0 ? last_modified : time(NULL);
        }

        time_t get_last_access()
        {
            return last_access != 0 ? last_access : get_last_modified();
        }

        time_t get_last_change()
        {
            return last_change != 0 ? last_change : get_last_modified();
        }

        
        std::string copy_status;
        std::vector<std::pair<std::string, std::string>> metadata;
        time_t last_modified;
        unsigned long long size;

        std::string m_owner;
        std::string m_group;
        std::string m_permissions;
        mode_t m_file_mode;
        bool is_directory;
        bool m_valid;
        bool m_not_exists;
        bool m_empty_dir;

        time_t last_access;
        time_t last_change;

};


struct list_segmented_item {
    list_segmented_item();
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
    //std::string copy_status;
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
    StorageBfsClientBase(configParams opt) : 
        configurations(opt),
        mUseCache(false),
        mAttrCacheMutex(),
        mAttrCacheMap() 
        {
            if (opt.useAttrCache)
                mUseCache = true;
        }
    virtual bool isADLS() = 0;
    
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
    virtual void UploadFromFile(const std::string localPath, METADATA &metadata) = 0;
    ///<summary>
    /// Uploads contents of a stream to a storage object(e.g. blob, file) to the Storage service
    ///</summary>
    ///<returns>none</returns>
    virtual void UploadFromStream(std::istream & sourceStream, const std::string blobName) = 0;
    virtual void UploadFromStream(std::istream & sourceStream, const std::string blobName, 
                        std::vector<std::pair<std::string, std::string>> & metadata) = 0;
    ///<summary>
    /// Downloads contents of a storage object(e.g. blob, file) to a local file
    ///</summary>
    ///<returns>none</returns>
    virtual long int DownloadToFile(const std::string blobName, const std::string filePath, time_t& last_modified) = 0;
    virtual long int DownloadToStream(const std::string blobName, std::ostream & destStream,
                        unsigned long long offset, unsigned long long size) = 0;
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
    virtual BfsFileProperty GetProperties(const std::string pathName, bool type_known = false) = 0;
    virtual access_control GetAccessControl(const std::string pathName) = 0;
    
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
    virtual std::vector<std::string> Rename(const std::string sourcePath,const  std::string destinationPath, bool isDir) = 0;
    ///<summary>
    /// Lists
    ///</summary>
    ///<returns>none</returns>
    virtual list_segmented_response List(std::string continuation, const std::string prefix, const std::string delimiter, int max_results = MAX_GET_LIST_RESULT_LIMIT) = 0;
    ///<summary>
    /// LIsts all directories within a list container
    /// Greedily list all blobs using the input params.
    ///</summary>
    virtual std::vector<std::pair<std::vector<list_segmented_item>, bool>> ListAllItemsSegmented(const std::string& prefix, const std::string& delimiter, int max_results = MAX_GET_LIST_RESULT_LIMIT) = 0;
    ///<summary>
    /// Updates the UNIX-style file mode on a path.
    ///</summary>
    virtual int ChangeMode(const char* path, mode_t mode) = 0;

    // Update metadata for a blob
    virtual int UpdateBlobProperty(std::string pathStr, std::string key, std::string value, METADATA *metadata = NULL) = 0;

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

    bool mUseCache;
    std::mutex mAttrCacheMutex;
    std::map<std::string, BfsFileProperty> mAttrCacheMap;
};

#endif