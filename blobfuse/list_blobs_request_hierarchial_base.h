#pragma once

#include <map>
#include <string>
#include <vector>

#include "storage_EXPORTS.h"

#include "common.h"
#include "http_base.h"
#include "storage_account.h"
#include "list_blobs_request_base.h"
#include "storage_request_base.h"

namespace azure { namespace storage_lite {

    class list_blobs_hierarchical_request_base : public blob_request_base {
    public:

        virtual std::string container() const = 0;
        virtual std::string prefix() const { return std::string(); }
        virtual std::string delimiter() const { return std::string(); }
        virtual std::string marker() const { return std::string(); }
        virtual int maxresults() const { return 0; }
        virtual list_blobs_request_base::include includes() const { return list_blobs_request_base::include::unspecifies; }

        AZURE_STORAGE_API void build_request(const storage_account &a, http_base &h) const override;
    };

    class list_blobs_hierarchical_item {
    public:
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
        lease_status status;
        lease_state state;
        lease_duration duration;
        std::string copy_status;
        std::vector<std::pair<std::string, std::string>> metadata;
        bool is_directory;
    };

    class list_blobs_hierarchical_response {
    public:
        std::string ms_request_id;
        std::vector<list_blobs_hierarchical_item> blobs;
        std::string next_marker;
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
    
} }