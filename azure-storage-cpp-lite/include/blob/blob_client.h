#pragma once

#include <iostream>
#include <memory>
#include <string>
#include <mutex>
#include <boost/thread/shared_mutex.hpp>
#include <syslog.h>

#include "storage_EXPORTS.h"

#include "storage_account.h"
#include "http/libcurl_http_client.h"
#include "tinyxml2_parser.h"
#include "executor.h"
#include "put_block_list_request_base.h"
#include "get_blob_property_request_base.h"
#include "get_blob_request_base.h"
#include "get_container_property_request_base.h"
#include "list_blobs_request_base.h"

namespace microsoft_azure { namespace storage {

    /// <summary>
    /// Provides a client-side logical representation of blob storage service on Windows Azure. This client is used to configure and execute requests against the service.
    /// </summary>
    /// <remarks>The service client encapsulates the base URI for the service. If the service client will be used for authenticated access, it also encapsulates the credentials for accessing the storage account.</remarks>
    class blob_client {
    public:
        /// <summary>
        /// Initializes a new instance of the <see cref="microsoft_azure::storage::blob_client" /> class.
        /// </summary>
        /// <param name="account">An existing <see cref="microsoft_azure::storage::storage_account" /> object.</param>
        /// <param name="size">An int value indicates the maximum concurrency expected during execute requests against the service.</param>
        blob_client(std::shared_ptr<storage_account> account, int size)
            : m_account(account) {
            m_context = std::make_shared<executor_context>(std::make_shared<tinyxml2_parser>(), std::make_shared<retry_policy>());
            m_client = std::make_shared<CurlEasyClient>(size);
        }

        /// <summary>
        /// Gets the curl client used to execute requests.
        /// </summary>
        /// <returns>The <see cref="microsoft_azure::storage::CurlEasyClient"> object</returns>
        std::shared_ptr<CurlEasyClient> client() const {
            return m_client;
        }

        /// <summary>
        /// Gets the storage account used to store the base uri and credentails.
        /// </summary>
        std::shared_ptr<storage_account> account() const {
            return m_account;
        }

        /// <summary>
        /// Gets the max parallelism used.
        /// </summary>
        unsigned int concurrency() const {
            return m_client->size();
        }

        /// <summary>
        /// Synchronously download the contents of a blob to a stream.
        /// </summary>
        /// <param name="container">The container name.</param>
        /// <param name="blob">The blob name.</param>
        /// <param name="offset">The offset at which to begin downloading the blob, in bytes.</param>
        /// <param name="size">The size of the data to download from the blob, in bytes.</param>
        /// <param name="os">The target stream.</param>
        /// <returns>A <see cref="std::future" /> object that represents the current operation.</returns>
        AZURE_STORAGE_API storage_outcome<chunk_property> get_chunk_to_stream_sync(const std::string &container, const std::string &blob, unsigned long long offset, unsigned long long size, std::ostream &os);

        /// <summary>
        /// Intitiates an asynchronous operation  to download the contents of a blob to a stream.
        /// </summary>
        /// <param name="container">The container name.</param>
        /// <param name="blob">The blob name.</param>
        /// <param name="offset">The offset at which to begin downloading the blob, in bytes.</param>
        /// <param name="size">The size of the data to download from the blob, in bytes.</param>
        /// <param name="os">The target stream.</param>
        /// <returns>A <see cref="std::future" /> object that represents the current operation.</returns>
        AZURE_STORAGE_API std::future<storage_outcome<void>> download_blob_to_stream(const std::string &container, const std::string &blob, unsigned long long offset, unsigned long long size, std::ostream &os);

        /// <summary>
        /// Intitiates an asynchronous operation  to upload the contents of a blob from a stream.
        /// </summary>
        /// <param name="container">The container name.</param>
        /// <param name="blob">The blob name.</param>
        /// <param name="is">The source stream.</param>
        /// <param name="metadata">A <see cref="std::vector"> that respresents metadatas.</param>
        /// <returns>A <see cref="std::future" /> object that represents the current operation.</returns>
        AZURE_STORAGE_API std::future<storage_outcome<void>> upload_block_blob_from_stream(const std::string &container, const std::string &blob, std::istream &is, const std::vector<std::pair<std::string, std::string>> &metadata);

        /// <summary>
        /// Intitiates an asynchronous operation  to delete a blob.
        /// </summary>
        /// <param name="container">The container name.</param>
        /// <param name="blob">The blob name.</param>
        /// <param name="delete_snapshots">A bool value, delete snapshots if it is true.</param>
        /// <returns>A <see cref="std::future" /> object that represents the current operation.</returns>
        AZURE_STORAGE_API std::future<storage_outcome<void>> delete_blob(const std::string &container, const std::string &blob, bool delete_snapshots = false);

        /// <summary>
        /// Intitiates an asynchronous operation  to create a container.
        /// </summary>
        /// <param name="container">The container name.</param>
        /// <returns>A <see cref="std::future" /> object that represents the current operation.</returns>
        AZURE_STORAGE_API std::future<storage_outcome<void>> create_container(const std::string &container);

        /// <summary>
        /// Intitiates an asynchronous operation  to delete a container.
        /// </summary>
        /// <param name="container">The container name.</param>
        /// <returns>A <see cref="std::future" /> object that represents the current operation.</returns>
        AZURE_STORAGE_API std::future<storage_outcome<void>> delete_container(const std::string &container);

        /// <summary>
        /// Intitiates an asynchronous operation  to get the container property.
        /// </summary>
        /// <param name="container">The container name.</param>
        /// <returns>A <see cref="std::future" /> object that represents the current operation.</returns>
        AZURE_STORAGE_API storage_outcome<container_property> get_container_property(const std::string &container);

        /// <summary>
        /// Intitiates an asynchronous operation  to list containers.
        /// </summary>
        /// <param name="prefix">The container name prefix.</param>
        /// <param name="include_metadata">A bool value, return metadatas if it is true.</param>
        /// <returns>A <see cref="std::future" /> object that represents the current operation.</returns>
        AZURE_STORAGE_API std::future<storage_outcome<list_containers_response>> list_containers(const std::string &prefix, bool include_metadata = false);

        /// <summary>
        /// Intitiates an asynchronous operation  to list all blobs.
        /// </summary>
        /// <param name="container">The container name.</param>
        /// <param name="prefix">The blob name prefix.</param>
        /// <returns>A <see cref="std::future" /> object that represents the current operation.</returns>
        AZURE_STORAGE_API std::future<storage_outcome<list_blobs_response>> list_blobs(const std::string &container, const std::string &prefix);

        /// <summary>
        /// Intitiates an asynchronous operation  to list blobs in segments.
        /// </summary>
        /// <param name="container">The container name.</param>
        /// <param name="delimiter">The delimiter used to designate the virtual directories.</param>
        /// <param name="continuation_token">A continuation token returned by a previous listing operation.</param>
        /// <param name="prefix">The blob name prefix.</param>
        /// <returns>A <see cref="std::future" /> object that represents the current operation.</returns>
        AZURE_STORAGE_API std::future<storage_outcome<list_blobs_hierarchical_response>> list_blobs_hierarchical(const std::string &container, const std::string &delimiter, const std::string &continuation_token, const std::string &prefix, int max_results = 10000);

        /// <summary>
        /// Intitiates an asynchronous operation  to get the property of a blob.
        /// </summary>
        /// <param name="container">The container name.</param>
        /// <param name="blob">The blob name.</param>
        /// <returns>A <see cref="std::future" /> object that represents the current operation.</returns>
        AZURE_STORAGE_API storage_outcome<blob_property> get_blob_property(const std::string &container, const std::string &blob);

        /// <summary>
        /// Intitiates an asynchronous operation  to download the block list of a blob.
        /// </summary>
        /// <param name="container">The container name.</param>
        /// <param name="blob">The blob name.</param>
        /// <returns>A <see cref="std::future" /> object that represents the current operation.</returns>
        AZURE_STORAGE_API std::future<storage_outcome<get_block_list_response>> get_block_list(const std::string &container, const std::string &blob);

        /// <summary>
        /// Intitiates an asynchronous operation  to upload a block of a blob from a stream.
        /// </summary>
        /// <param name="container">The container name.</param>
        /// <param name="blob">The blob name.</param>
        /// <param name="blockid">A Base64-encoded block ID that identifies the block.</param>
        /// <param name="is">The source stream.</param>
        /// <returns>A <see cref="std::future" /> object that represents the current operation.</returns>
        AZURE_STORAGE_API std::future<storage_outcome<void>> upload_block_from_stream(const std::string &container, const std::string &blob, const std::string &blockid, std::istream &is);

        /// <summary>
        /// Intitiates an asynchronous operation  to create a block blob with existing blocks.
        /// </summary>
        /// <param name="container">The container name.</param>
        /// <param name="blob">The blob name.</param>
        /// <param name="block_list">A <see cref="std::vector"> that contains all blocks in order.</param>
        /// <param name="metadata">A <see cref="std::vector"> that respresents metadatas.</param>
        /// <returns>A <see cref="std::future" /> object that represents the current operation.</returns>
        AZURE_STORAGE_API std::future<storage_outcome<void>> put_block_list(const std::string &container, const std::string &blob, const std::vector<put_block_list_request_base::block_item> &block_list, const std::vector<std::pair<std::string, std::string>> &metadata);

        /// <summary>
        /// Intitiates an asynchronous operation  to create an append blob.
        /// </summary>
        /// <param name="container">The container name.</param>
        /// <param name="blob">The blob name.</param>
        /// <returns>A <see cref="std::future" /> object that represents the current operation.</returns>
        AZURE_STORAGE_API std::future<storage_outcome<void>> create_append_blob(const std::string &container, const std::string &blob);

        /// <summary>
        /// Intitiates an asynchronous operation  to append the content to an append blob from a stream.
        /// </summary>
        /// <param name="container">The container name.</param>
        /// <param name="blob">The blob name.</param>
        /// <param name="is">The source stream.</param>
        /// <returns>A <see cref="std::future" /> object that represents the current operation.</returns>
        AZURE_STORAGE_API std::future<storage_outcome<void>> append_block_from_stream(const std::string &container, const std::string &blob, std::istream &is);

        /// <summary>
        /// Intitiates an asynchronous operation  to create an page blob.
        /// </summary>
        /// <param name="container">The container name.</param>
        /// <param name="blob">The blob name.</param>
        /// <param name="size">The size of the page blob, in bytes.</param>
        /// <returns>A <see cref="std::future" /> object that represents the current operation.</returns>
        AZURE_STORAGE_API std::future<storage_outcome<void>> create_page_blob(const std::string &container, const std::string &blob, unsigned long long size);

        /// <summary>
        /// Intitiates an asynchronous operation  to upload a blob range content from a stream.
        /// </summary>
        /// <param name="container">The container name.</param>
        /// <param name="blob">The blob name.</param>
        /// <param name="offset">The offset at which to begin upload to the blob, in bytes.</param>
        /// <param name="size">The size of the data, in bytes.</param>
        /// <param name="os">The target stream.</param>
        /// <returns>A <see cref="std::future" /> object that represents the current operation.</returns>
        AZURE_STORAGE_API std::future<storage_outcome<void>> put_page_from_stream(const std::string &container, const std::string &blob, unsigned long long offset, unsigned long long size, std::istream &is);

        /// <summary>
        /// Intitiates an asynchronous operation  to clear pages of a page blob range.
        /// </summary>
        /// <param name="container">The container name.</param>
        /// <param name="blob">The blob name.</param>
        /// <param name="offset">The offset at which to begin clearing, in bytes.</param>
        /// <param name="size">The size of the data to be cleared from the blob, in bytes.</param>
        /// <returns>A <see cref="std::future" /> object that represents the current operation.</returns>
        AZURE_STORAGE_API std::future<storage_outcome<void>> clear_page(const std::string &container, const std::string &blob, unsigned long long offset, unsigned long long size);

        /// <summary>
        /// Intitiates an asynchronous operation  to get the page ranges fro a page blob.
        /// </summary>
        /// <param name="container">The container name.</param>
        /// <param name="blob">The blob name.</param>
        /// <param name="offset">The offset at which to get, in bytes.</param>
        /// <param name="size">The size of the data to be get from the blob, in bytes.</param>
        /// <returns>A <see cref="std::future" /> object that represents the current operation.</returns>
        AZURE_STORAGE_API std::future<storage_outcome<get_page_ranges_response>> get_page_ranges(const std::string &container, const std::string &blob, unsigned long long offset, unsigned long long size);

        /// <summary>
        /// Intitiates an asynchronous operation  to copy a blob to another.
        /// </summary>
        /// <param name="sourceContainer">The source container name.</param>
        /// <param name="sourceBlob">The source blob name.</param>
        /// <param name="destContainer">The destination container name.</param>
        /// <param name="destBlob">The destination blob name.</param>
        /// <returns>A <see cref="std::future" /> object that represents the current operation.</returns>
        AZURE_STORAGE_API std::future<storage_outcome<void>> start_copy(const std::string &sourceContainer, const std::string &sourceBlob, const std::string &destContainer, const std::string &destBlob);

    private:
        std::shared_ptr<CurlEasyClient> m_client;
        std::shared_ptr<storage_account> m_account;
        std::shared_ptr<executor_context> m_context;
    };

    /// <summary>
    /// Abstract layer of the blob_client class for the attribute cache layer,
    /// Provides a client-side logical representation of blob storage service on Windows Azure.
    //// This client is used to configure and execute requests against the service with caching the attributes in mind.
    /// </summary>
    /// <remarks>The service client encapsulates the base URI for the service. If the service client will be used for authenticated access, it also encapsulates the credentials for accessing the storage account.</remarks>
    class sync_blob_client
    {
    public:

        virtual ~sync_blob_client() = 0;
        virtual bool is_valid() const = 0;

        /// <summary>
        /// List blobs in segments.
        /// </summary>
        /// <param name="container">The container name.</param>
        /// <param name="delimiter">The delimiter used to designate the virtual directories.</param>
        /// <param name="continuation_token">A continuation token returned by a previous listing operation.</param>
        /// <param name="prefix">The blob name prefix.</param>
        /// <param name="maxresults">Maximum amount of results to receive</param>
        /// <returns>A response from list_blobs_hierarchical that contains a list of blobs and their details</returns>
        virtual list_blobs_hierarchical_response list_blobs_hierarchical(const std::string &container, const std::string &delimiter, const std::string &continuation_token, const std::string &prefix, int maxresults = 10000) = 0;

        /// <summary>
        /// Uploads the contents of a blob from a local file, file size need to be equal or smaller than 64MB.
        /// </summary>
        /// <param name="sourcePath">The source file path.</param>
        /// <param name="container">The container name.</param>
        /// <param name="blob">The blob name.</param>
        /// <param name="metadata">A <see cref="std::vector"> that respresents metadatas.</param>
        virtual void put_blob(const std::string &sourcePath, const std::string &container, const std::string blob, const std::vector<std::pair<std::string, std::string>> &metadata = std::vector<std::pair<std::string, std::string>>()) = 0;
 
        /// <summary>
        /// Uploads the contents of a blob from a stream.
        /// </summary>
        /// <param name="container">The container name.</param>
        /// <param name="blob">The blob name.</param>
        /// <param name="is">The source stream.</param>
        /// <param name="metadata">A <see cref="std::vector"> that respresents metadatas.</param>
        virtual void upload_block_blob_from_stream(const std::string &container, const std::string blob, std::istream &is, const std::vector<std::pair<std::string, std::string>> &metadata = std::vector<std::pair<std::string, std::string>>()) = 0;

        /// <summary>
        /// Uploads the contents of a blob from a local file.
        /// </summary>
        /// <param name="sourcePath">The source file path.</param>
        /// <param name="container">The container name.</param>
        /// <param name="blob">The blob name.</param>
        /// <param name="metadata">A <see cref="std::vector"> that respresents metadatas.</param>
        /// <param name="parallel">A size_t value indicates the maximum parallelism can be used in this request.</param>
        virtual void upload_file_to_blob(const std::string &sourcePath, const std::string &container, const std::string blob, const std::vector<std::pair<std::string, std::string>> &metadata = std::vector<std::pair<std::string, std::string>>(), size_t parallel = 8) = 0;

        /// <summary>
        /// Downloads the contents of a blob to a stream.
        /// </summary>
        /// <param name="container">The container name.</param>
        /// <param name="blob">The blob name.</param>
        /// <param name="offset">The offset at which to begin downloading the blob, in bytes.</param>
        /// <param name="size">The size of the data to download from the blob, in bytes.</param>
        /// <param name="os">The target stream.</param>
        virtual void download_blob_to_stream(const std::string &container, const std::string &blob, unsigned long long offset, unsigned long long size, std::ostream &os) = 0;

        /// <summary>
        /// Downloads the contents of a blob to a local file.
        /// </summary>
        /// <param name="container">The container name.</param>
        /// <param name="blob">The blob name.</param>
        /// <param name="offset">The offset at which to begin downloading the blob, in bytes.</param>
        /// <param name="size">The size of the data to download from the blob, in bytes.</param>
        /// <param name="destPath">The target file path.</param>
        /// <param name="parallel">A size_t value indicates the maximum parallelism can be used in this request.</param>
        virtual void download_blob_to_file(const std::string &container, const std::string &blob, const std::string &destPath, time_t &returned_last_modified, size_t parallel = 9) = 0;

        /// <summary>
        /// Gets the property of a blob.
        /// </summary>
        /// <param name="container">The container name.</param>
        /// <param name="blob">The blob name.</param>
        virtual blob_property get_blob_property(const std::string &container, const std::string &blob) = 0;

        /// <summary>
        /// Examines the existance of a blob.
        /// </summary>
        /// <param name="container">The container name.</param>
        /// <param name="blob">The blob name.</param>
        /// <returns>Return true if the blob does exist, otherwise, return false.</returns>
        virtual bool blob_exists(const std::string &container, const std::string &blob) = 0;

        /// <summary>
        /// Deletes a blob.
        /// </summary>
        /// <param name="container">The container name.</param>
        /// <param name="blob">The blob name.</param>
        virtual void delete_blob(const std::string &container, const std::string &blob) = 0;

        /// <summary>
        /// Copy a blob to another.
        /// </summary>
        /// <param name="sourceContainer">The source container name.</param>
        /// <param name="sourceBlob">The source blob name.</param>
        /// <param name="destContainer">The destination container name.</param>
        /// <param name="destBlob">The destination blob name.</param>
        virtual void start_copy(const std::string &sourceContainer, const std::string &sourceBlob, const std::string &destContainer, const std::string &destBlob) = 0;
    };

    /// <summary>
    /// Provides a wrapper for client-side logical representation of blob storage service on Windows Azure. This wrappered client is used to configure and execute requests against the service.
    /// </summary>
    /// <remarks>This wrappered client could limit a concurrency per client objects. And it will not throw exceptions, instead, it will set errno to return error codes.</remarks>
    class blob_client_wrapper : public sync_blob_client
    {
    public:
        /// <summary>
        /// Constructs a blob client wrapper from a blob client instance.
        /// </summary>
        /// <param name="blobClient">A <see cref="microsoft_azure::storage::blob_client"> object stored in shared_ptr.</param>
        explicit blob_client_wrapper(std::shared_ptr<blob_client> blobClient)
            : m_blobClient(blobClient),
            m_valid(true)
        {
            if (blobClient != NULL)
            {
                m_concurrency = blobClient->concurrency();
            }
        }

        /// <summary>
        /// Constructs an empty blob client wrapper.
        /// </summary>
        /// <param name="valid">A bool value indicates this client wrapper is valid or not.</param>
        explicit blob_client_wrapper(bool valid)
            : m_valid(valid)
        {
        }

        /// <summary>
        /// Constructs a blob client wrapper from another blob client wrapper instance.
        /// </summary>
        /// <param name="other">A <see cref="microsoft_azure::storage::blob_client_wrapper"> object.</param>
        blob_client_wrapper(blob_client_wrapper &&other)
        {
            m_blobClient = other.m_blobClient;
            m_concurrency = other.m_concurrency;
            m_valid = other.m_valid;
        }

        blob_client_wrapper& operator=(blob_client_wrapper&& other)
        {
            m_blobClient = other.m_blobClient;
            m_concurrency = other.m_concurrency;
            m_valid = other.m_valid;
            return *this;
        }

        bool is_valid() const
        {
            return m_valid && (m_blobClient != NULL);
        }

        /// <summary>
        /// Constructs a blob client wrapper from storage account credential.
        /// </summary>
        /// <param name="account_name">The storage account name.</param>
        /// <param name="account_key">The storage account key.</param>
	/// <param name="sas_token">A sas token for the container.</param>
        /// <param name="concurrency">The maximum number requests could be executed in the same time.</param>
        /// <returns>Return a <see cref="microsoft_azure::storage::blob_client_wrapper"> object.</returns>
        static blob_client_wrapper blob_client_wrapper_init(const std::string &account_name, const std::string &account_key, const std::string &sas_token, const unsigned int concurrency);

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
        static blob_client_wrapper blob_client_wrapper_init(const std::string &account_name, const std::string &account_key, const std::string &sas_token, const unsigned int concurrency, bool use_https, 
							    const std::string &blob_endpoint);
        /* C++ wrappers without exception but error codes instead */

        /* container level*/

        /// <summary>
        /// Creates a container.
        /// </summary>
        /// <param name="container">The container name.</param>
        void create_container(const std::string &container);

        /// <summary>
        /// Deletes a container.
        /// </summary>
        /// <param name="container">The container name.</param>
        void delete_container(const std::string &container);

        /// <summary>
        /// Examines the existance of a container.
        /// </summary>
        /// <param name="container">The container name.</param>
        /// <returns>Return true if the container does exist, otherwise, return false.</returns>
        bool container_exists(const std::string &container);

        /// <summary>
        /// List containers.
        /// </summary>
        /// <param name="prefix">The container name prefix.</param>
        /// <param name="include_metadata">A bool value, return metadatas if it is true.</param>
        std::vector<list_containers_item> list_containers(const std::string &prefix, bool include_metadata = false);

        /* blob level */

        /// <summary>
        /// List blobs in segments.
        /// </summary>
        /// <param name="container">The container name.</param>
        /// <param name="delimiter">The delimiter used to designate the virtual directories.</param>
        /// <param name="continuation_token">A continuation token returned by a previous listing operation.</param>
        /// <param name="prefix">The blob name prefix.</param>
        list_blobs_hierarchical_response list_blobs_hierarchical(const std::string &container, const std::string &delimiter, const std::string &continuation_token, const std::string &prefix, int maxresults = 10000);

        /// <summary>
        /// Uploads the contents of a blob from a local file, file size need to be equal or smaller than 64MB.
        /// </summary>
        /// <param name="sourcePath">The source file path.</param>
        /// <param name="container">The container name.</param>
        /// <param name="blob">The blob name.</param>
        /// <param name="metadata">A <see cref="std::vector"> that respresents metadatas.</param>
        void put_blob(const std::string &sourcePath, const std::string &container, const std::string blob, const std::vector<std::pair<std::string, std::string>> &metadata = std::vector<std::pair<std::string, std::string>>());

        /// <summary>
        /// Uploads the contents of a blob from a stream.
        /// </summary>
        /// <param name="container">The container name.</param>
        /// <param name="blob">The blob name.</param>
        /// <param name="is">The source stream.</param>
        /// <param name="metadata">A <see cref="std::vector"> that respresents metadatas.</param>
        void upload_block_blob_from_stream(const std::string &container, const std::string blob, std::istream &is, const std::vector<std::pair<std::string, std::string>> &metadata = std::vector<std::pair<std::string, std::string>>());

        /// <summary>
        /// Uploads the contents of a blob from a local file.
        /// </summary>
        /// <param name="sourcePath">The source file path.</param>
        /// <param name="container">The container name.</param>
        /// <param name="blob">The blob name.</param>
        /// <param name="metadata">A <see cref="std::vector"> that respresents metadatas.</param>
        /// <param name="parallel">A size_t value indicates the maximum parallelism can be used in this request.</param>
        void upload_file_to_blob(const std::string &sourcePath, const std::string &container, const std::string blob, const std::vector<std::pair<std::string, std::string>> &metadata = std::vector<std::pair<std::string, std::string>>(), size_t parallel = 8);

        /// <summary>
        /// Downloads the contents of a blob to a stream.
        /// </summary>
        /// <param name="container">The container name.</param>
        /// <param name="blob">The blob name.</param>
        /// <param name="offset">The offset at which to begin downloading the blob, in bytes.</param>
        /// <param name="size">The size of the data to download from the blob, in bytes.</param>
        /// <param name="os">The target stream.</param>
        void download_blob_to_stream(const std::string &container, const std::string &blob, unsigned long long offset, unsigned long long size, std::ostream &os);

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
        void download_blob_to_file(const std::string &container, const std::string &blob, const std::string &destPath, time_t &returned_last_modified, size_t parallel = 9);

        /// <summary>
        /// Gets the property of a blob.
        /// </summary>
        /// <param name="container">The container name.</param>
        /// <param name="blob">The blob name.</param>
        /// <returns> A <see cref="blob_property"/> object that represents the proerty of a particular blob
        blob_property get_blob_property(const std::string &container, const std::string &blob);

        /// <summary>
        /// Examines the existance of a blob.
        /// </summary>
        /// <param name="container">The container name.</param>
        /// <param name="blob">The blob name.</param>
        /// <returns>Return true if the blob does exist, otherwise, return false.</returns>
        bool blob_exists(const std::string &container, const std::string &blob);

        /// <summary>
        /// Deletes a blob.
        /// </summary>
        /// <param name="container">The container name.</param>
        /// <param name="blob">The blob name.</param>
        void delete_blob(const std::string &container, const std::string &blob);

        /// <summary>
        /// Copy a blob to another.
        /// </summary>
        /// <param name="sourceContainer">The source container name.</param>
        /// <param name="sourceBlob">The source blob name.</param>
        /// <param name="destContainer">The destination container name.</param>
        /// <param name="destBlob">The destination blob name.</param>
        void start_copy(const std::string &sourceContainer, const std::string &sourceBlob, const std::string &destContainer, const std::string &destBlob);
    private:
        blob_client_wrapper() {}

        std::shared_ptr<blob_client> m_blobClient;
        std::mutex s_mutex;
        unsigned int m_concurrency;
        bool m_valid;
    };

    // A wrapper around the "blob_client_wrapper" that provides in-memory caching for "get_blob_properties" calls.
    class blob_client_attr_cache_wrapper : public sync_blob_client
    {
    public:
        /// <summary>
        /// Constructs a blob client wrapper from a blob client instance.
        /// </summary>
        /// <param name="blobClient">A <see cref="microsoft_azure::storage::blob_client"> object stored in shared_ptr.</param>
        explicit blob_client_attr_cache_wrapper(std::shared_ptr<sync_blob_client> blob_client_wrapper)
            : m_blob_client_wrapper(blob_client_wrapper), attr_cache()
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

        blob_client_attr_cache_wrapper& operator=(blob_client_attr_cache_wrapper&& other)
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
        static blob_client_attr_cache_wrapper blob_client_attr_cache_wrapper_init(const std::string &account_name, const std::string &account_key, const std::string &sas_token, const unsigned int concurrency);

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
        static blob_client_attr_cache_wrapper blob_client_attr_cache_wrapper_init(const std::string &account_name, const std::string &account_key, const std::string &sas_token, const unsigned int concurrency, bool use_https, 
                                const std::string &blob_endpoint);  

        /// <summary>
        /// List blobs in segments.
        /// </summary>
        /// <param name="container">The container name.</param>
        /// <param name="delimiter">The delimiter used to designate the virtual directories.</param>
        /// <param name="continuation_token">A continuation token returned by a previous listing operation.</param>
        /// <param name="prefix">The blob name prefix.</param>
        /// <param name="maxresults">Maximum amount of results to receive</param>
        /// <returns>A response from list_blobs_hierarchical that contains a list of blobs and their details</returns>
        list_blobs_hierarchical_response list_blobs_hierarchical(const std::string &container, const std::string &delimiter, const std::string &continuation_token, const std::string &prefix, int maxresults = 10000);

        /// <summary>
        /// Uploads the contents of a blob from a local file, file size need to be equal or smaller than 64MB.
        /// </summary>
        /// <param name="sourcePath">The source file path.</param>
        /// <param name="container">The container name.</param>
        /// <param name="blob">The blob name.</param>
        /// <param name="metadata">A <see cref="std::vector"> that respresents metadatas.</param>
        void put_blob(const std::string &sourcePath, const std::string &container, const std::string blob, const std::vector<std::pair<std::string, std::string>> &metadata = std::vector<std::pair<std::string, std::string>>());

        /// <summary>
        /// Uploads the contents of a blob from a stream.
        /// </summary>
        /// <param name="container">The container name.</param>
        /// <param name="blob">The blob name.</param>
        /// <param name="is">The source stream.</param>
        /// <param name="metadata">A <see cref="std::vector"> that respresents metadatas.</param>
        void upload_block_blob_from_stream(const std::string &container, const std::string blob, std::istream &is, const std::vector<std::pair<std::string, std::string>> &metadata = std::vector<std::pair<std::string, std::string>>());

        /// <summary>
        /// Uploads the contents of a blob from a local file.
        /// </summary>
        /// <param name="sourcePath">The source file path.</param>
        /// <param name="container">The container name.</param>
        /// <param name="blob">The blob name.</param>
        /// <param name="metadata">A <see cref="std::vector"> that respresents metadatas.</param>
        /// <param name="parallel">A size_t value indicates the maximum parallelism can be used in this request.</param>
        void upload_file_to_blob(const std::string &sourcePath, const std::string &container, const std::string blob, const std::vector<std::pair<std::string, std::string>> &metadata = std::vector<std::pair<std::string, std::string>>(), size_t parallel = 8);

        /// <summary>
        /// Downloads the contents of a blob to a stream.
        /// </summary>
        /// <param name="container">The container name.</param>
        /// <param name="blob">The blob name.</param>
        /// <param name="offset">The offset at which to begin downloading the blob, in bytes.</param>
        /// <param name="size">The size of the data to download from the blob, in bytes.</param>
        /// <param name="os">The target stream.</param>
        void download_blob_to_stream(const std::string &container, const std::string &blob, unsigned long long offset, unsigned long long size, std::ostream &os);

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
        void download_blob_to_file(const std::string &container, const std::string &blob, const std::string &destPath, time_t &returned_last_modified, size_t parallel = 8);

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
        bool blob_exists(const std::string &container, const std::string &blob);

        /// <summary>
        /// Deletes a blob.
        /// </summary>
        /// <param name="container">The container name.</param>
        /// <param name="blob">The blob name.</param>
        void delete_blob(const std::string &container, const std::string &blob);

        /// <summary>
        /// Copy a blob to another.
        /// </summary>
        /// <param name="sourceContainer">The source container name.</param>
        /// <param name="sourceBlob">The source blob name.</param>
        /// <param name="destContainer">The destination container name.</param>
        /// <param name="destBlob">The destination blob name.</param>
        void start_copy(const std::string &sourceContainer, const std::string &sourceBlob, const std::string &destContainer, const std::string &destBlob);
        
        private:
        std::shared_ptr<sync_blob_client> m_blob_client_wrapper;
        attribute_cache attr_cache;
    };
} } // microsoft_azure::storage
