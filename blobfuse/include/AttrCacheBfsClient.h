#ifndef ATTRCACHEBFSCLIENTBASE_H
#define ATTRCACHEBFSCLIENTBASE_H

#include <StorageBfsClientBase.h>
#include <BlockBlobBfsClient.h>
#include <DataLakeBfsClient.h>


using namespace azure::storage_lite;


class AttrCacheItem
{
public:
    AttrCacheItem(std::string name, BfsFileProperty props) : m_confirmed(false), m_mutex(), m_name(name), m_props(props)
    {

    }

    // True if this item should accurately represent a blob on the service.
    // False if not (or unknown).  Marking an item as not confirmed is invalidating the cache.
    bool m_confirmed;

    // A mutex that can be locked in shared or unique mode (reader/writer lock)
    // TODO: Consider switching this to be a regular mutex
    boost::shared_mutex m_mutex;

    // Name of the blob
    std::string m_name;

    // The (cached) properties of the blob
    BfsFileProperty m_props;
};

// A thread-safe cache of the properties of the blobs in a container on the service.
// In order to access or update a single cache item, you must lock on the mutex in the relevant blob_cache_item, and also on the mutex representing the parent directory.
// This is due to the single cache item being linked to the directory
// The directory mutex must always be locked before the blob mutex, and no thread should ever have more than one blob mutex (or directory) held at once - this will prevent deadlocks.
// For example, to access the properties of a blob "dir1/dir2/blobname", you need to access and lock the mutex returned by get_dir_item("dir1/dir2"), and then the mutex in the blob_cache_item
// returned by get_blob_item("dir1/dir2/blobname").
// 
// To read the properties of the blob from the cache, lock both mutexes in shared mode.
// To update the properties of a single blob (or to invalidate a cache item), grab the directory mutex in shared mode, and the blob mutex in unique mode.  The mutexes must be held during both the
// relevant service call and the following cache update.
// For a 'list blobs' request, first grab the mutex for the directory in unique mode.  Then, make the request and parse the response.  For each blob in the response, grab the blob mutex for that item in unique mode 
// before updating it.  Don't release the directory mutex until all blobs have been updated.
// 
// TODO: Currently, the maps holding the cached information grow without bound; this should be fixed.
// TODO: Implement a cache timeout
// TODO: When we no longer use an internal copy of cpplite, the attrib cache code should stay with blobfuse - it's not really applicable in the general cpplite use case.
class AttrCache
{
public:
    AttrCache() : blob_cache(), blobs_mutex(), dir_cache(), dirs_mutex()
    {
    }

    std::shared_ptr<boost::shared_mutex> get_dir_item(const std::string& path);
    std::shared_ptr<AttrCacheItem> get_blob_item(const std::string& path);
    void invalidate_dir_recursively(const std::string& path);

private:
    std::map<std::string, std::shared_ptr<AttrCacheItem>> blob_cache;
    std::mutex blobs_mutex; // Used to protect the blob_cache map itself, not items in the map.
    std::map<std::string, std::shared_ptr<boost::shared_mutex>> dir_cache;
    std::mutex dirs_mutex;// Used to protect the dir_cache map itself, not items in the map.
};



class AttrCacheBfsClient : public StorageBfsClientBase
{
public:
    AttrCacheBfsClient(configParams opt) :
    StorageBfsClientBase(opt)
    {
        if (opt.useADLS)
        {
            isAdlsMode = true;
            syslog(LOG_INFO, "Initializing blobfuse using DataLake");
            blob_client = std::make_shared<DataLakeBfsClient>(opt);
        }
        else
        {
            isAdlsMode  = false;
            syslog(LOG_INFO, "Initializing blobfuse using BlockBlob");
            blob_client = std::make_shared<BlockBlobBfsClient>(opt);
        }
    }

    bool isADLS() { return isAdlsMode; }

    ///<summary>
    /// Authenticates the storage account and container
    ///</summary>
    ///<returns>bool: if we authenticate to the storage account and container successfully</returns>
    bool AuthenticateStorage() override;
    ///<summary>
    /// Uploads contents of a file to a block blob to the Storage service
    ///</summary>
    ///TODO: params
    ///<returns>none</returns>
    void UploadFromFile(const std::string sourcePath, METADATA &metadata) override;
    ///<summary>
    /// Uploads contents of a stream to a block blob to the Storage service
    ///</summary>
    ///<returns>none</returns>
    void UploadFromStream(std::istream & sourceStream, const std::string blobName) override;
    void UploadFromStream(std::istream & sourceStream, const std::string blobName, 
                std::vector<std::pair<std::string, std::string>> & metadata) override;

    ///<summary>
    /// Downloads contents of a block blob to a local file
    ///</summary>
    ///<returns>none</returns>
    long int DownloadToFile(const std::string blobName, const std::string filePath, time_t& last_modified) override;
    long int DownloadToStream(const std::string blobName, std::ostream & destStream,
                unsigned long long offset, unsigned long long size) override;
    ///<summary>
    /// Creates a Directory
    ///</summary>
    ///<returns>none</returns>
    bool CreateDirectory(const std::string directoryPath) override;
    ///<summary>
    /// Deletes a Directory
    ///</summary>
    ///<returns>none</returns>
    bool DeleteDirectory(const std::string directoryPath) override;
    ///<summary>
    /// Checks if the blob is a directory
    ///</summary>
    ///<returns>none</returns>
    bool IsDirectory(const char * path) override;
    ///<summary>
    /// Helper function - Checks if the "directory" blob is empty
    ///</summary>
    D_RETURN_CODE IsDirectoryEmpty(std::string path) override;
    ///<summary>
    /// Deletes a File
    ///</summary>
    ///<returns>none</returns>
    void DeleteFile(std::string pathToDelete) override;
    ///<summary>
    /// Gets the properties of a path
    ///</summary>
    ///<returns>BfsFileProperty object which contains the property details of the file</returns>
    BfsFileProperty GetProperties(std::string pathName, bool type_known = false) override;
    access_control GetAccessControl(const std::string pathName) override;
    ///<summary>
    /// Determines whether or not a path (file or directory) exists or not
    ///</summary>
    ///<returns>none</returns>
    int Exists(std::string pathName) override;
    ///<summary>
    /// Determines whether or not a path (file or directory) exists or not
    ///</summary>
    ///<returns>none</returns>
    bool Copy(std::string sourcePath, std::string destinationPath) override;
    ///<summary>
    /// Renames a file
    ///</summary>
    ///<returns>none</returns>
    std::vector<std::string> Rename(std::string sourcePath, std::string destinationPath) override;
    std::vector<std::string> Rename(const std::string sourcePath,const  std::string destinationPath, bool isDir) override;
    ///<summary>
    /// Lists
    ///</summary>
    ///<returns>none</returns>
    list_segmented_response List(std::string continuation, std::string prefix, std::string delimiter, int max_results = MAX_GET_LIST_RESULT_LIMIT) override;
    ///<summary>
    /// LIsts all directories within a list container
    /// Greedily list all blobs using the input params.
    ///</summary>
    std::vector<std::pair<std::vector<list_segmented_item>, bool>> ListAllItemsSegmented(const std::string& prefix, const std::string& delimiter, int max_results = MAX_GET_LIST_RESULT_LIMIT) override;
    ///<summary>
    /// Updates the UNIX-style file mode on a path.
    ///</summary>
    int ChangeMode(const char* path, mode_t mode) override;

    int UpdateBlobProperty(std::string pathStr, std::string key, std::string value, METADATA *metadata = NULL);
    
    private:
        std::shared_ptr<StorageBfsClientBase> blob_client;
        AttrCache attr_cache;
        bool isAdlsMode;
};
#endif //ATTRCACHEBFSCLIENTBASE_H