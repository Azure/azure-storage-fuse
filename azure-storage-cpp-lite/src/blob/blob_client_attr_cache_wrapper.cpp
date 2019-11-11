#include "blob/blob_client.h"

namespace microsoft_azure {
    namespace storage {

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

        // Performs a thread-safe map lookup of the input key in the directory map.
        // Will create new entries if necessary before returning.
        std::shared_ptr<boost::shared_mutex> blob_client_attr_cache_wrapper::attribute_cache::get_dir_item(const std::string& path)
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
        std::shared_ptr<blob_client_attr_cache_wrapper::blob_cache_item> blob_client_attr_cache_wrapper::attribute_cache::get_blob_item(const std::string& path)
        {
            std::lock_guard<std::mutex> lock(blobs_mutex);
            auto iter = blob_cache.find(path);
            if(iter == blob_cache.end())
            {
                auto blob_item = std::make_shared<blob_client_attr_cache_wrapper::blob_cache_item>("", blob_property(false));
                blob_cache[path] = blob_item;
                return blob_item;
            }
            else
            {
                return iter->second;
            }
        }

        /// <summary>
        /// List blobs in segments.
        /// </summary>
        /// <param name="container">The container name.</param>
        /// <param name="delimiter">The delimiter used to designate the virtual directories.</param>
        /// <param name="continuation_token">A continuation token returned by a previous listing operation.</param>
        /// <param name="prefix">The blob name prefix.</param>
        /// <param name="maxresults">Maximum amount of results to receive</param>
        /// <returns>A response from list_blobs_hierarchical that contains a list of blobs and their details</returns>
        list_blobs_hierarchical_response blob_client_attr_cache_wrapper::list_blobs_hierarchical(const std::string &container, const std::string &delimiter, const std::string &continuation_token, const std::string &prefix, int maxresults)
        {
            std::shared_ptr<boost::shared_mutex> dir_mutex = attr_cache.get_dir_item(prefix);
            std::unique_lock<boost::shared_mutex> uniquelock(*dir_mutex);

            errno = 0;
            list_blobs_hierarchical_response response = m_blob_client_wrapper->list_blobs_hierarchical(container, delimiter, continuation_token, prefix, maxresults);
            if (errno == 0)
            {
                for (size_t i = 0; i < response.blobs.size(); i++)
                {
                    if (!response.blobs[i].is_directory)
                    {
                        // TODO - modify list_blobs to return blob_property items; simplifying this logic.
                        blob_property properties(true);

                        properties.cache_control = response.blobs[i].cache_control;
//                        properties.content_disposition = response.blobs[i].content_disposition;  // TODO - once this is available in cpplite.
                        properties.content_encoding = response.blobs[i].content_encoding;
                        properties.content_language = response.blobs[i].content_language;
                        properties.size = response.blobs[i].content_length;
                        properties.content_md5 = response.blobs[i].content_md5;
                        properties.content_type = response.blobs[i].content_type;
                        properties.etag = response.blobs[i].etag;
                        properties.metadata = response.blobs[i].metadata;
                        properties.copy_status = response.blobs[i].copy_status;
                        properties.last_modified = curl_getdate(response.blobs[i].last_modified.c_str(), NULL);

                        // Note that this internally locks the mutex protecting the attr_cache blob list.  Normally this is fine, but here it's a bit concerning, because we've already 
                        // taken a lock on the directory string.
                        // It should be fine, there should be no chance of deadlock, as the internal mutex is released before get_blob_item returns, but we should take care when modifying.
                        std::shared_ptr<blob_client_attr_cache_wrapper::blob_cache_item> cache_item = attr_cache.get_blob_item(response.blobs[i].name);
                        std::unique_lock<boost::shared_mutex> uniquelock(cache_item->m_mutex);
                        cache_item->m_props = properties;
                        cache_item->m_confirmed = true;
                    }
                }
            }
            return response;
        }


        blob_client_attr_cache_wrapper blob_client_attr_cache_wrapper::blob_client_attr_cache_wrapper_init(const std::string &account_name, const std::string &account_key, const std::string &sas_token, const unsigned int concurrency)
        {
            return blob_client_attr_cache_wrapper_init(account_name, account_key, sas_token, concurrency, false, NULL);
        }


        blob_client_attr_cache_wrapper blob_client_attr_cache_wrapper::blob_client_attr_cache_wrapper_init(const std::string &account_name, const std::string &account_key, const std::string &sas_token, const unsigned int concurrency, bool use_https, const std::string &blob_endpoint)
        {
            std::shared_ptr<blob_client_wrapper> wrapper = blob_client_wrapper_init(account_name, account_key, sas_token, concurrency, use_https, blob_endpoint);
            return blob_client_attr_cache_wrapper(wrapper);
        }

        /// <summary>
        /// Uploads the contents of a blob from a local file, file size need to be equal or smaller than 64MB.
        /// </summary>
        /// <param name="sourcePath">The source file path.</param>
        /// <param name="container">The container name.</param>
        /// <param name="blob">The blob name.</param>
        /// <param name="metadata">A <see cref="std::vector"> that respresents metadatas.</param>
        void blob_client_attr_cache_wrapper::put_blob(const std::string &sourcePath, const std::string &container, const std::string blob, const std::vector<std::pair<std::string, std::string>> &metadata)
        {
            // Invalidate the cache.
            // TODO: consider updating the cache with the new values.  Will require modifying cpplite to return info from put_blob.
            std::shared_ptr<boost::shared_mutex> dir_mutex = attr_cache.get_dir_item(get_parent_str(blob));
            std::shared_ptr<blob_client_attr_cache_wrapper::blob_cache_item> cache_item = attr_cache.get_blob_item(blob);
            boost::shared_lock<boost::shared_mutex> dirlock(*dir_mutex);
            std::unique_lock<boost::shared_mutex> uniquelock(cache_item->m_mutex);
            m_blob_client_wrapper->put_blob(sourcePath, container, blob, metadata);
            cache_item->m_confirmed = false;
        }

        /// <summary>
        /// Uploads the contents of a blob from a stream.
        /// </summary>
        /// <param name="container">The container name.</param>
        /// <param name="blob">The blob name.</param>
        /// <param name="is">The source stream.</param>
        /// <param name="metadata">A <see cref="std::vector"> that respresents metadatas.</param>
        void blob_client_attr_cache_wrapper::upload_block_blob_from_stream(const std::string &container, const std::string blob, std::istream &is, const std::vector<std::pair<std::string, std::string>> &metadata)
        {
            // Invalidate the cache.
            // TODO: consider updating the cache with the new values.  Will require modifying cpplite to return info from put_blob.
            std::shared_ptr<boost::shared_mutex> dir_mutex = attr_cache.get_dir_item(get_parent_str(blob));
            std::shared_ptr<blob_client_attr_cache_wrapper::blob_cache_item> cache_item = attr_cache.get_blob_item(blob);
            boost::shared_lock<boost::shared_mutex> dirlock(*dir_mutex);
            std::unique_lock<boost::shared_mutex> uniquelock(cache_item->m_mutex);
            m_blob_client_wrapper->upload_block_blob_from_stream(container, blob, is, metadata);
            cache_item->m_confirmed = false;
        }

        /// <summary>
        /// Uploads the contents of a blob from a local file.
        /// </summary>
        /// <param name="sourcePath">The source file path.</param>
        /// <param name="container">The container name.</param>
        /// <param name="blob">The blob name.</param>
        /// <param name="metadata">A <see cref="std::vector"> that respresents metadatas.</param>
        /// <param name="parallel">A size_t value indicates the maximum parallelism can be used in this request.</param>
        void blob_client_attr_cache_wrapper::upload_file_to_blob(const std::string &sourcePath, const std::string &container, const std::string blob, const std::vector<std::pair<std::string, std::string>> &metadata, size_t parallel)
        {
            // Invalidate the cache.
            // TODO: consider updating the cache with the new values.  Will require modifying cpplite to return info from put_blob.
            std::shared_ptr<boost::shared_mutex> dir_mutex = attr_cache.get_dir_item(get_parent_str(blob));
            std::shared_ptr<blob_client_attr_cache_wrapper::blob_cache_item> cache_item = attr_cache.get_blob_item(blob);
            boost::shared_lock<boost::shared_mutex> dirlock(*dir_mutex);
            std::unique_lock<boost::shared_mutex> uniquelock(cache_item->m_mutex);
            m_blob_client_wrapper->upload_file_to_blob(sourcePath, container, blob, metadata, parallel);
            cache_item->m_confirmed = false;
        }

        /// <summary>
        /// Downloads the contents of a blob to a stream.
        /// </summary>
        /// <param name="container">The container name.</param>
        /// <param name="blob">The blob name.</param>
        /// <param name="offset">The offset at which to begin downloading the blob, in bytes.</param>
        /// <param name="size">The size of the data to download from the blob, in bytes.</param>
        /// <param name="os">The target stream.</param>
        void blob_client_attr_cache_wrapper::download_blob_to_stream(const std::string &container, const std::string &blob, unsigned long long offset, unsigned long long size, std::ostream &os)
        {
            // TODO: lock & update the attribute cache with the headers from the get call(s), once download_blob_to_* is modified to return them.
            m_blob_client_wrapper->download_blob_to_stream(container, blob, offset, size, os);
        }

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
        void blob_client_attr_cache_wrapper::download_blob_to_file(const std::string &container, const std::string &blob, const std::string &destPath, time_t &returned_last_modified, size_t parallel)
        {
            // TODO: lock & update the attribute cache with the headers from the get call(s), once download_blob_to_* is modified to return them.
            m_blob_client_wrapper->download_blob_to_file(container, blob, destPath, returned_last_modified, parallel);
        }

        /// <summary>
        /// Gets the property of a blob.
        /// </summary>
        /// <param name="container">The container name.</param>
        /// <param name="blob">The blob name.</param>
        blob_property blob_client_attr_cache_wrapper::get_blob_property(const std::string &container, const std::string &blob)
        {
            return get_blob_property(container, blob, false);
        }

        /// <summary>
        /// Gets the property of a blob.
        /// </summary>
        /// <param name="container">The container name.</param>
        /// <param name="blob">The blob name.</param>
        /// <param name="assume_cache_invalid">True if the blob's properties should be fetched from the service, even if the cache item seels valid.
        /// Useful if there is reason to suspect the properties may have changed behind the scenes (specifically, if there's a pending copy operation.)</param>
        blob_property blob_client_attr_cache_wrapper::get_blob_property(const std::string &container, const std::string &blob, bool assume_cache_invalid)
        {
            std::shared_ptr<boost::shared_mutex> dir_mutex = attr_cache.get_dir_item(get_parent_str(blob));
            std::shared_ptr<blob_client_attr_cache_wrapper::blob_cache_item> cache_item = attr_cache.get_blob_item(blob);
            boost::shared_lock<boost::shared_mutex> dirlock(*dir_mutex);

            if (!assume_cache_invalid)
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
                cache_item->m_props = m_blob_client_wrapper->get_blob_property(container, blob);
                if (errno != 0)
                {
                    return blob_property(false); // keep errno unchanged
                }
                cache_item->m_confirmed = true;
                return cache_item->m_props;
            }
        }

        /// <summary>
        /// Examines the existance of a blob.
        /// </summary>
        /// <param name="container">The container name.</param>
        /// <param name="blob">The blob name.</param>
        /// <returns>Return true if the blob does exist, otherwise, return false.</returns>
        bool blob_client_attr_cache_wrapper::blob_exists(const std::string &container, const std::string &blob)
        {
            blob_property props = get_blob_property(container, blob); // go through the cache
            if(props.valid())
            {
                errno = 0;
                return true;
            }
            return false;
        }

        /// <summary>
        /// Deletes a blob.
        /// </summary>
        /// <param name="container">The container name.</param>
        /// <param name="blob">The blob name.</param>
        void blob_client_attr_cache_wrapper::delete_blob(const std::string &container, const std::string &blob)
        {
            // These calls cannot be cached because we do not have a negative cache - blobs in the cache are either valid/confirmed, or unknown (which could be deleted, or not checked on the service.)
            std::shared_ptr<boost::shared_mutex> dir_mutex = attr_cache.get_dir_item(get_parent_str(blob));
            std::shared_ptr<blob_client_attr_cache_wrapper::blob_cache_item> cache_item = attr_cache.get_blob_item(blob);
            boost::shared_lock<boost::shared_mutex> dirlock(*dir_mutex);
            std::unique_lock<boost::shared_mutex> uniquelock(cache_item->m_mutex);
            m_blob_client_wrapper->delete_blob(container, blob);
            cache_item->m_confirmed = false;
        }

        /// <summary>
        /// Copy a blob to another.
        /// </summary>
        /// <param name="sourceContainer">The source container name.</param>
        /// <param name="sourceBlob">The source blob name.</param>
        /// <param name="destContainer">The destination container name.</param>
        /// <param name="destBlob">The destination blob name.</param>
        void blob_client_attr_cache_wrapper::start_copy(const std::string &sourceContainer, const std::string &sourceBlob, const std::string &destContainer, const std::string &destBlob)
        {
            // No need to lock on the source, as we're neither modifying nor querying the source blob or its cached content.
            // We do need to lock on the destination, because if the start copy operation succeeds we need to invalidate the cached data.
            std::shared_ptr<boost::shared_mutex> dir_mutex = attr_cache.get_dir_item(get_parent_str(destBlob));
            std::shared_ptr<blob_client_attr_cache_wrapper::blob_cache_item> cache_item = attr_cache.get_blob_item(destBlob);
            boost::shared_lock<boost::shared_mutex> dirlock(*dir_mutex);
            std::unique_lock<boost::shared_mutex> uniquelock(cache_item->m_mutex);
            errno = 0;
            m_blob_client_wrapper->start_copy(sourceContainer, sourceBlob, destContainer, destBlob);
            cache_item->m_confirmed = false;
        }
}}