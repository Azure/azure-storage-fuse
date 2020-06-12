#pragma once

#include <blob/blob_client.h>

namespace azure { namespace storage_lite {

    /// <summary>
    /// Constructs a blob client wrapper from storage account credential.
    /// </summary>
    /// <param name="account_name">The storage account name.</param>
    /// <param name="account_key">The storage account key.</param>
    /// <param name="concurrency">The maximum number requests could be executed in the same time.</param>
    /// <param name="use_https">True if https should be used (instead of HTTP).  Note that this may cause a sizable perf loss, due to issues in libcurl.</param>
    /// <param name="blob_endpoint">Blob endpoint URI to allow non-public clouds as well as custom domains.</param>
    /// <returns>Return a <see cref="microsoft_azure::storage::blob_client_wrapper"> object.</returns>
    std::shared_ptr<blob_client_wrapper> blob_client_wrapper_init_accountkey(
        const std::string &account_name,
        const std::string &account_key,
        const unsigned int concurrency,
        bool use_https = true,
        const std::string &blob_endpoint = "");

    /// <summary>
    /// Constructs a blob client wrapper from storage account credential.
    /// </summary>
    /// <param name="account_name">The storage account name.</param>
    /// <param name="sas_token">A sas token for the container.</param>
    /// <param name="concurrency">The maximum number requests could be executed in the same time.</param>
    /// <param name="use_https">True if https should be used (instead of HTTP).  Note that this may cause a sizable perf loss, due to issues in libcurl.</param>
    /// <param name="blob_endpoint">Blob endpoint URI to allow non-public clouds as well as custom domains.</param>
    /// <returns>Return a <see cref="microsoft_azure::storage::blob_client_wrapper"> object.</returns>
    std::shared_ptr<blob_client_wrapper> blob_client_wrapper_init_sastoken(
    const std::string &account_name,
    const std::string &sas_token,
    const unsigned int concurrency,
    bool use_https = true,
    const std::string &blob_endpoint = "");

    /// <summary>
    /// Constructs a blob client wrapper from storage account credential.
    /// </summary>
    /// <param name="account_name">The storage account name.</param>
    /// <param name="account_key">The storage account key.</param>
    /// <param name="sas_token">A sas token for the container.</param>
    /// <param name="concurrency">The maximum number requests could be executed in the same time.</param>
    /// <param name="blob_endpoint">Blob endpoint URI to allow non-public clouds as well as custom domains.</param>
    /// <returns>Return a <see cref="microsoft_azure::storage::blob_client_wrapper"> object.</returns>
    std::shared_ptr<blob_client_wrapper> blob_client_wrapper_init_oauth(
    const std::string &account_name,
    const unsigned int concurrency,
    const std::string &blob_endpoint = "");

    // A wrapper around the "blob_client_wrapper" that provides in-memory caching for "get_blob_properties" calls.
    class blob_client_attr_cache_wrapper : public blob_client_wrapper
    {
    public:
        
        /// <summary>
        /// Constructs a blob client wrapper from a blob client instance.
        /// </summary>
        /// <param name="blobClient">A <see cref="microsoft_azure::storage::blob_client"> object stored in shared_ptr.</param>
        explicit blob_client_attr_cache_wrapper(std::shared_ptr<blob_client_wrapper> blob_wrapper)
            : m_blob_client_wrapper(blob_wrapper), attr_cache()
        {
        }

        /// <summary>
        /// Constructs a blob client wrapper from another blob client wrapper instance.
        /// </summary>
        /// <param name="other">A <see cref="microsoft_azure::storage::blob_client_attr_cache_wrapper"> object.</param>
        blob_client_attr_cache_wrapper(blob_client_attr_cache_wrapper &&other)
        {
            m_blob_client_wrapper = other.m_blob_client_wrapper;
        }

        virtual blob_client_attr_cache_wrapper& operator=(blob_client_attr_cache_wrapper&& other)
        {
            m_blob_client_wrapper = other.m_blob_client_wrapper;
            return *this;
        }
        
        bool is_valid() const
        {
            return m_blob_client_wrapper != NULL;
        }

        // Represents a blob on the service
        class blob_cache_item
        {
        public:
            blob_cache_item(std::string name, blob_property props) : m_confirmed(false), m_mutex(), m_name(name), m_props(props)
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
            blob_property m_props;
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
        class attribute_cache
        {
        public:
            attribute_cache() : blob_cache(), blobs_mutex(), dir_cache(), dirs_mutex()
            {
            }

            std::shared_ptr<boost::shared_mutex> get_dir_item(const std::string& path);
            std::shared_ptr<blob_cache_item> get_blob_item(const std::string& path);

        private:
            std::map<std::string, std::shared_ptr<blob_cache_item>> blob_cache;
            std::mutex blobs_mutex; // Used to protect the blob_cache map itself, not items in the map.
            std::map<std::string, std::shared_ptr<boost::shared_mutex>> dir_cache;
            std::mutex dirs_mutex;// Used to protect the dir_cache map itself, not items in the map.
        };

        /// <summary>
        /// Constructs a blob client wrapper from storage account credential.
        /// </summary>
        /// <param name="account_name">The storage account name.</param>
        /// <param name="account_key">The storage account key.</param>
        /// <param name="sas_token">A sas token for the container.</param>
        /// <param name="concurrency">The maximum number requests could be executed in the same time.</param>
        /// <returns>Return a <see cref="microsoft_azure::storage::blob_client_wrapper"> object.</returns>
        static blob_client_attr_cache_wrapper blob_client_attr_cache_wrapper_init_accountkey(
            const std::string &account_name,
            const std::string &account_key,
            const unsigned int concurrency,
            bool use_https = true,
            const std::string &blob_endpoint = "");

        /// <summary>
        /// Constructs a blob client wrapper from storage account credential.
        /// </summary>
        /// <param name="account_name">The storage account name.</param>
        /// <param name="account_key">The storage account key.</param>
        /// <param name="sas_token">A sas token for the container.</param>
        /// <param name="concurrency">The maximum number requests could be executed in the same time.</param>
        /// <param name="use_https">True if https should be used (instead of HTTP).  Note that this may cause a sizable perf loss, due to issues in libcurl.</param>
        /// <param name="blob_endpoint">Blob endpoint URI to allow non-public clouds as well as custom domains.</param>
        /// <returns>Return a <see cref="microsoft_azure::storage::blob_client_wrapper"> object.</returns>
        static blob_client_attr_cache_wrapper blob_client_attr_cache_wrapper_init_sastoken(
            const std::string &account_name,
            const std::string &sas_token,
            const unsigned int concurrency,
            bool use_https = true,
            const std::string &blob_endpoint = "");

        /// <summary>
        /// Constructs a blob client wrapper from storage account credential.
        /// </summary>
        /// <param name="account_name">The storage account name.</param>
        /// <param name="account_key">The storage account key.</param>
        /// <param name="sas_token">A sas token for the container.</param>
        /// <param name="concurrency">The maximum number requests could be executed in the same time.</param>
        /// <returns>Return a <see cref="microsoft_azure::storage::blob_client_wrapper"> object.</returns>
        static blob_client_attr_cache_wrapper blob_client_attr_cache_wrapper_oauth(
        const std::string &account_name,
        const unsigned int concurrency,
        const std::string &blob_endpoint = "");

        /// <summary>
        /// List blobs in segments.
        /// </summary>
        /// <param name="container">The container name.</param>
        /// <param name="delimiter">The delimiter used to designate the virtual directories.</param>
        /// <param name="continuation_token">A continuation token returned by a previous listing operation.</param>
        /// <param name="prefix">The blob name prefix.</param>
        /// <param name="maxresults">Maximum amount of results to receive</param>
        /// <returns>A response from list_blobs_segmented that contains a list of blobs and their details</returns>
        virtual list_blobs_segmented_response list_blobs_segmented(const std::string &container, const std::string &delimiter, const std::string &continuation_token, const std::string &prefix, int maxresults = 10000);

        /// <summary>
        /// Uploads the contents of a blob from a local file, file size need to be equal or smaller than 64MB.
        /// </summary>
        /// <param name="sourcePath">The source file path.</param>
        /// <param name="container">The container name.</param>
        /// <param name="blob">The blob name.</param>
        /// <param name="metadata">A <see cref="std::vector"> that respresents metadatas.</param>
        virtual void put_blob(const std::string &sourcePath, const std::string &container, const std::string blob, const std::vector<std::pair<std::string, std::string>> &metadata = std::vector<std::pair<std::string, std::string>>());

        /// <summary>
        /// Uploads the contents of a blob from a stream.
        /// </summary>
        /// <param name="container">The container name.</param>
        /// <param name="blob">The blob name.</param>
        /// <param name="is">The source stream.</param>
        /// <param name="metadata">A <see cref="std::vector"> that respresents metadatas.</param>
        virtual void upload_block_blob_from_stream(const std::string &container, const std::string blob, std::istream &is, const std::vector<std::pair<std::string, std::string>> &metadata = std::vector<std::pair<std::string, std::string>>());

        /// <summary>
        /// Uploads the contents of a blob from a local file.
        /// </summary>
        /// <param name="sourcePath">The source file path.</param>
        /// <param name="container">The container name.</param>
        /// <param name="blob">The blob name.</param>
        /// <param name="metadata">A <see cref="std::vector"> that respresents metadatas.</param>
        /// <param name="parallel">A size_t value indicates the maximum parallelism can be used in this request.</param>
        virtual void upload_file_to_blob(const std::string &sourcePath, const std::string &container, const std::string blob, const std::vector<std::pair<std::string, std::string>> &metadata = std::vector<std::pair<std::string, std::string>>(), size_t parallel = 8);

        /// <summary>
        /// Downloads the contents of a blob to a stream.
        /// </summary>
        /// <param name="container">The container name.</param>
        /// <param name="blob">The blob name.</param>
        /// <param name="offset">The offset at which to begin downloading the blob, in bytes.</param>
        /// <param name="size">The size of the data to download from the blob, in bytes.</param>
        /// <param name="os">The target stream.</param>
        virtual void download_blob_to_stream(const std::string &container, const std::string &blob, unsigned long long offset, unsigned long long size, std::ostream &os);

        /// <summary>
        /// Downloads the contents of a blob to a local file.
        /// </summary>
        /// <param name="container">The container name.</param>
        /// <param name="blob">The blob name.</param>
        /// <param name="offset">The offset at which to begin downloading the blob, in bytes.</param>
        /// <param name="size">The size of the data to download from the blob, in bytes.</param>
        /// <param name="destPath">The target file path.</param>
        /// <param name="parallel">A size_t value indicates the maximum parallelism can be used in this request.</param>
        /// <returns>A <see cref="storage_outcome" /> object that represents the properties (etag, last modified time and size) from the first chunk retrieved.</returns>
        virtual void download_blob_to_file(const std::string &container, const std::string &blob, const std::string &destPath, time_t &returned_last_modified, size_t parallel = 8);

        /// <summary>
        /// Gets the property of a blob.
        /// </summary>
        /// <param name="container">The container name.</param>
        /// <param name="blob">The blob name.</param>
        /// <returns> A <see cref="blob_property"/> object that represents the proerty of a particular blob
        blob_property get_blob_property(const std::string &container, const std::string &blob);

        /// <summary>
        /// Gets the property of a blob.
        /// </summary>
        /// <param name="container">The container name.</param>
        /// <param name="blob">The blob name.</param>
        /// <returns> A <see cref="blob_property"/> object that represents the proerty of a particular blob
        blob_property get_blob_property(const std::string &container, const std::string &blob, bool assume_cache_invalid);

        /// <summary>
        /// Examines the existance of a blob.
        /// </summary>
        /// <param name="container">The container name.</param>
        /// <param name="blob">The blob name.</param>
        /// <returns>Return true if the blob does exist, otherwise, return false.</returns>
        virtual bool blob_exists(const std::string &container, const std::string &blob);

        /// <summary>
        /// Deletes a blob.
        /// </summary>
        /// <param name="container">The container name.</param>
        /// <param name="blob">The blob name.</param>
        virtual void delete_blob(const std::string &container, const std::string &blob);

        /// <summary>
        /// Copy a blob to another.
        /// </summary>
        /// <param name="sourceContainer">The source container name.</param>
        /// <param name="sourceBlob">The source blob name.</param>
        /// <param name="destContainer">The destination container name.</param>
        /// <param name="destBlob">The destination blob name.</param>
        virtual void start_copy(const std::string &sourceContainer, const std::string &sourceBlob, const std::string &destContainer, const std::string &destBlob);
        
        private:
        std::shared_ptr<blob_client_wrapper> m_blob_client_wrapper;
        attribute_cache attr_cache;
    };
} }