#pragma once

#include <iostream>
#include <memory>
#include <string>

#include "storage_EXPORTS.h"

#include "storage_account.h"
#include "http/libcurl_http_client.h"
#include "tinyxml2_parser.h"
#include "executor.h"
#include "put_block_list_request_base.h"
#include "get_blob_property_request_base.h"
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
        AZURE_STORAGE_API std::future<storage_outcome<list_blobs_hierarchical_response>> list_blobs_hierarchical(const std::string &container, const std::string &delimiter, const std::string &continuation_token, const std::string &prefix);

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
    /// Provides a wrapper for client-side logical representation of blob storage service on Windows Azure. This wrappered client is used to configure and execute requests against the service.
    /// </summary>
    /// <remarks>This wrappered client could limit a concurrency per client objects. And it will not throw exceptions, instead, it will set errno to return error codes.</remarks>
    class blob_client_wrapper
    {
    public:
        /// <summary>
        /// Constructs a blob client wrapper from a blob client instance.
        /// </summary>
        /// <param name="blobClient">A <see cref="microsoft_azure::storage::blob_client"> object stored in shared_ptr.</param>
        blob_client_wrapper(std::shared_ptr<blob_client> blobClient)
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
        blob_client_wrapper(bool valid)
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

        /// <summary>
        /// Constructs a blob client wrapper from storage account credential.
        /// </summary>
        /// <param name="account_name">The storage account name.</param>
        /// <param name="account_key">The storage account key.</param>
        /// <param name="concurrency">The maximum number requests could be executed in the same time.</param>
        /// <returns>Return a <see cref="microsoft_azure::storage::blob_client_wrapper"> object.</returns>
        static blob_client_wrapper blob_client_wrapper_init(const std::string &account_name, const std::string &account_key, const unsigned int concurrency);

        /// <summary>
        /// Constructs a blob client wrapper from storage account credential.
        /// </summary>
        /// <param name="account_name">The storage account name.</param>
        /// <param name="account_key">The storage account key.</param>
        /// <param name="concurrency">The maximum number requests could be executed in the same time.</param>
        /// <param name="use_https">True if https should be used (instead of HTTP).  Note that this may cause a sizable perf loss, due to issues in libcurl.
        /// <returns>Return a <see cref="microsoft_azure::storage::blob_client_wrapper"> object.</returns>
        static blob_client_wrapper blob_client_wrapper_init(const std::string &account_name, const std::string &account_key, const unsigned int concurrency, bool use_https);
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
        list_blobs_hierarchical_response list_blobs_hierarchical(const std::string &container, const std::string &delimiter, const std::string &continuation_token, const std::string &prefix);

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
        void download_blob_to_file(const std::string &container, const std::string &blob, const std::string &destPath, size_t parallel = 9);

        /// <summary>
        /// Gets the property of a blob.
        /// </summary>
        /// <param name="container">The container name.</param>
        /// <param name="blob">The blob name.</param>
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

} } // microsoft_azure::storage
