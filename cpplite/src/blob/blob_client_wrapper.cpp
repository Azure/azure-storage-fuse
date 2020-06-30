/* C++ interface wrapper for blob client
 * No exceptions will throw.
 */

#include <iostream>
#include <fstream>

#ifndef _WIN32
#include <sys/stat.h>
#endif

#include "blob/blob_client.h"
#include "logging.h"
#include "storage_errno.h"
#include "base64.h"

namespace azure {  namespace storage_lite {

        const unsigned long long DOWNLOAD_CHUNK_SIZE = 16 * 1024 * 1024;
        const long long MIN_UPLOAD_CHUNK_SIZE = 16 * 1024 * 1024;
        const long long MAX_BLOB_SIZE = 5242880000000; // 4.77TB 

        class mempool
        {
        public:
            ~mempool()
            {
                while(!m_buffers.empty())
                {
                    auto buffer = m_buffers.front();
                    delete[] buffer;
                    m_buffers.pop();
                }
            }

            char* get_buffer()
            {
                std::lock_guard<std::mutex> lg(m_buffers_mutex);
                if(m_buffers.empty())
                {
                    char* buffer = new char[s_block_size];
                    return buffer;
                }
                else
                {
                    char* buffer = m_buffers.front();
                    m_buffers.pop();
                    return buffer;
                }
            }
            void release_buffer(char *buffer)
            {
                std::lock_guard<std::mutex> lg(m_buffers_mutex);
                m_buffers.push(buffer);
            }
        private:
            std::queue<char*> m_buffers;
            std::mutex m_buffers_mutex;
            static const size_t s_block_size = 4*1024*1024;
        };
        static mempool mpool;
        off_t get_file_size(const char* path);

        blob_client_wrapper blob_client_wrapper::blob_client_wrapper_init(const std::string &account_name, const std::string &account_key, const std::string &sas_token, const unsigned int concurrency)
        {
            return blob_client_wrapper_init(account_name, account_key, sas_token, concurrency, false, NULL);
        }


        blob_client_wrapper blob_client_wrapper::blob_client_wrapper_init(const std::string &account_name, const std::string &account_key, const std::string &sas_token,  const unsigned int concurrency, const bool use_https, 
                                                                          const std::string &blob_endpoint)
        {
            if(account_name.empty() || ((account_key.empty() && sas_token.empty()) || (!account_key.empty() && !sas_token.empty())))
            {
                errno = invalid_parameters;
                return blob_client_wrapper(false);
            }

            /* set a default concurrency value. */
            unsigned int concurrency_limit = 40;
            if(concurrency != 0)
            {
                concurrency_limit = concurrency;
            }
            std::string accountName(account_name);
            std::string accountKey(account_key);

            try
            {
                std::shared_ptr<storage_credential>  cred;
                if (account_key.length() > 0) 
                {
                    cred = std::make_shared<shared_key_credential>(accountName, accountKey);
                }
                else
                {
                    // We have already verified that exactly one form of credentials is present, so if shared key is not present, it must be sas.
                    cred = std::make_shared<shared_access_signature_credential>(sas_token);
                }
                std::shared_ptr<storage_account> account = std::make_shared<storage_account>(accountName, cred, use_https, blob_endpoint);
                std::shared_ptr<blob_client> blobClient= std::make_shared<azure::storage_lite::blob_client>(account, concurrency_limit);
                errno = 0;
                return blob_client_wrapper(blobClient);
            }
            catch(const std::exception &ex)
            {
                logger::log(log_level::error, "Failed to create blob client.  ex.what() = %s.", ex.what());
                errno = unknown_error;
                return blob_client_wrapper(false);
            }
        }

        void blob_client_wrapper::create_container(const std::string &container)
        {
            if(!is_valid())
            {
                errno = client_not_init;
                return;
            }
            if(container.empty())
            {
                errno = invalid_parameters;
                return;
            }

            try
            {
                auto task = m_blobClient->create_container(container);
                auto result = task.get();

                if(!result.success())
                {
                    /* container already exists.
                     *               * Bug, need to compare message as well.
                     *                             * */
                    errno = std::stoi(result.error().code);
                }
                else
                {
                    errno = 0;
                }
            }
            catch(std::exception& ex)
            {
                logger::log(log_level::error, "Unknown failure in create_container.  ex.what() = %s, container = %s.", ex.what(), container.c_str());
                errno = unknown_error;
                return;
            }
        }

        void blob_client_wrapper::delete_container(const std::string &container)
        {
            if(!is_valid())
            {
                errno = client_not_init;
                return;
            }
            if(container.empty())
            {
                errno = invalid_parameters;
                return;
            }

            try
            {
                auto task = m_blobClient->delete_container(container);
                auto result = task.get();

                if(!result.success())
                {
                    errno = std::stoi(result.error().code);
                }
                else
                {
                    errno = 0;
                }
            }
            catch(std::exception& ex)
            {
                logger::log(log_level::error, "Unknown failure in delete_container.  ex.what() = %s, container = %s.", ex.what(), container.c_str());
                errno = unknown_error;
                return;
            }
        }

        bool blob_client_wrapper::container_exists(const std::string &container)
        {
            if(!is_valid())
            {
                errno = client_not_init;
                return false;
            }
            if(container.empty())
            {
                errno = invalid_parameters;
                return false;
            }

            try
            {
                auto containerProperty = m_blobClient->get_container_properties(container).get().response();

                if(containerProperty.valid())
                {
                    errno = 0;
                    return true;
                }
                else
                {
                    logger::log(log_level::error, "Unknown failure in container_exists.  No exception, but the container property object is invalid.  errno = %d.", errno);
                    errno = unknown_error;
                    return false;
                }
            }
            catch(std::exception& ex)
            {
                logger::log(log_level::error, "Unknown failure in container_exists.  ex.what() = %s, container = %s.", ex.what(), container.c_str());
                errno = unknown_error;
                return false;
            }
        }

        std::vector<list_containers_item> blob_client_wrapper::list_containers_segmented(const std::string &prefix, const std::string& continuation_token, const int max_result, bool include_metadata)
        {
            if(!is_valid())
            {
                errno = client_not_init;
                return std::vector<list_containers_item>();
            }

            try
            {
                auto task = m_blobClient->list_containers_segmented(prefix, continuation_token, max_result, include_metadata);
                auto result = task.get();

                if(!result.success())
                {
                    errno = std::stoi(result.error().code);
                    return std::vector<list_containers_item>();
                }
                return result.response().containers;
            }
            catch(std::exception& ex)
            {
                logger::log(log_level::error, "Unknown failure in list_containers.  ex.what() = %s, prefix = %s.", ex.what(), prefix.c_str());
                errno = unknown_error;
                return std::vector<list_containers_item>();
            }
        }

        list_blobs_segmented_response blob_client_wrapper::list_blobs_segmented(const std::string &container, const std::string &delimiter, const std::string &continuation_token, const std::string &prefix, int max_results)
        {
            if(!is_valid())
            {
                errno = client_not_init;
                return list_blobs_segmented_response();
            }
            if(container.empty())
            {
                errno = invalid_parameters;
                return list_blobs_segmented_response();
            }

            try
            {
                auto task = m_blobClient->list_blobs_segmented(container, delimiter, continuation_token, prefix, max_results);
                auto result = task.get();

                if(!result.success())
                {
                    errno = std::stoi(result.error().code);
                    return list_blobs_segmented_response();
                }
                else
                {
                    errno = 0;
                    return result.response();
                }
            }
            catch(std::exception& ex)
            {
                logger::log(log_level::error, "Unknown failure in list_blobs_hierarchial.  ex.what() = %s, container = %s, prefix = %s.", ex.what(), container.c_str(), prefix.c_str());
                errno = unknown_error;
                return list_blobs_segmented_response();
            }
        }

        void blob_client_wrapper::put_blob(const std::string &sourcePath, const std::string &container, const std::string blob, const std::vector<std::pair<std::string, std::string>> &metadata)
        {
            if(!is_valid())
            {
                errno = client_not_init;
                return;
            }
            if(sourcePath.empty() || container.empty() || blob.empty())
            {
                errno = invalid_parameters;
                return;
            }

            std::ifstream ifs;
            try
            {
                ifs.open(sourcePath, std::ifstream::in);
            }
            catch(std::exception& ex)
            {
                // TODO open failed
                logger::log(log_level::error, "Failure to open the input stream in put_blob.  ex.what() = %s, sourcePath = %s.", ex.what(), sourcePath.c_str());
                errno = unknown_error;
                return;
            }

            int error_code = 0;
            try
            {
                auto task = m_blobClient->upload_block_blob_from_stream(container, blob, ifs, metadata);
                auto result = task.get();
                if(!result.success())
                {
                    error_code = std::stoi(result.error().code);
                }
            }
            catch(std::exception& ex)
            {
                logger::log(log_level::error, "Failure to upload the blob in put_blob.  ex.what() = %s, container = %s, blob = %s, sourcePath = %s.", ex.what(), container.c_str(), blob.c_str(), sourcePath.c_str());
                error_code = unknown_error;
            }

            try
            {
                ifs.close();
            }
            catch(std::exception& ex)
            {
                logger::log(log_level::error, "Failure to close the input stream in put_blob.  ex.what() = %s, container = %s, blob = %s, sourcePath = %s.", ex.what(), container.c_str(), blob.c_str(), sourcePath.c_str());
                if (error_code == 0)
                {
                    error_code = unknown_error;
                }
            }
            errno = error_code;
        }

        void blob_client_wrapper::upload_block_blob_from_stream(const std::string &container, const std::string blob, std::istream &is, const std::vector<std::pair<std::string, std::string>> &metadata, size_t streamlen)
        {
            if(!is_valid())
            {
                errno = client_not_init;
                return;
            }
            if(container.empty() || blob.empty())
            {
                errno = invalid_parameters;
                return;
            }

            try
            {
                std::future<storage_outcome<void>> task;
                if(streamlen == blob_client_wrapper::NOT_USER_DEFINED_STREAMLEN)
                    task = m_blobClient->upload_block_blob_from_stream(container, blob, is, metadata);
                else
                    task = m_blobClient->upload_block_blob_from_stream(container, blob, is, metadata, streamlen);
                auto result = task.get();
                if(!result.success())
                {
                    errno = std::stoi(result.error().code);
                    if (errno == 0) {
                        errno = 503;
                    }
                }
                else
                {
                    errno = 0;
                }
            }
            catch(std::exception& ex)
            {
                logger::log(log_level::error, "Unknown failure in upload_block_blob_from_stream.  ex.what() = %s, container = %s, blob = %s streamlen=%zu.", ex.what(), container.c_str(), blob.c_str(), streamlen);
                errno = unknown_error;
            }
        }

        void blob_client_wrapper::upload_file_to_blob(const std::string &sourcePath, const std::string &container, const std::string blob, const std::vector<std::pair<std::string, std::string>> &metadata, size_t parallel)
        {
            if(!is_valid())
            {
                errno = client_not_init;
                return;
            }
            if(sourcePath.empty() || container.empty() || blob.empty())
            {
                errno = invalid_parameters;
                return;
            }

            off_t fileSize = get_file_size(sourcePath.c_str());
            if(fileSize < 0)
            {
                /*errno already set by get_file_size*/
                return;
            }

            if(fileSize <= 64*1024*1024)
            {
                put_blob(sourcePath, container, blob, metadata);
                // put_blob sets errno
		return;
            }

            int result = 0;

            //support blobs up to 4.77TB = if file is larger, return EFBIG error
            //need to round to the nearest multiple of 4MB for efficiency
            if(fileSize > MAX_BLOB_SIZE)
            {
                errno = EFBIG;
                return;
            }

            long long block_size = MIN_UPLOAD_CHUNK_SIZE;

            if(fileSize > (50000 * MIN_UPLOAD_CHUNK_SIZE))
            {
                long long min_block = fileSize / 50000; 
                int remainder = min_block % 4*1024*1024;
                min_block += 4*1024*1024 - remainder;
                block_size = min_block < MIN_UPLOAD_CHUNK_SIZE ? MIN_UPLOAD_CHUNK_SIZE : min_block;
            }

            std::ifstream ifs(sourcePath, std::ios::in | std::ios::binary);
            if(!ifs)
            {
                logger::log(log_level::error, "Failed to open the input stream in upload_file_to_blob.  errno = %d, sourcePath = %s.", errno, sourcePath.c_str());
                errno = unknown_error;
                return;
            }

            std::vector<put_block_list_request_base::block_item> block_list;
            std::deque<std::future<int>> task_list;
            std::mutex mutex;
            std::condition_variable cv;
            std::mutex cv_mutex;

            for(long long offset = 0, idx = 0; offset < fileSize; offset += block_size, ++idx)
            {
                // control the number of submitted jobs.
                while(task_list.size() > m_concurrency)
                {
                    auto r = task_list.front().get();
                    task_list.pop_front();
                    if (0 == result) {
                        result = r;
                    }
                }
                if (0 != result) {
                    break;
                }
                size_t length = static_cast<size_t>(std::min(block_size, fileSize - offset));

                char* buffer = (char*)malloc(static_cast<size_t>(block_size)); // This cast is save because block size should always be lower than 4GB
                if (!buffer) {
                    result = 12;
                    break;
                }
                if(!ifs.read(buffer, length))
                {
                    logger::log(log_level::error, "Failed to read from input stream in upload_file_to_blob.  sourcePath = %s, container = %s, blob = %s, offset = %lld, length = %d.", sourcePath.c_str(), container.c_str(), blob.c_str(), offset, length);
                    result = unknown_error;
                    break;
                }
                std::string raw_block_id = std::to_string(idx);
                //pad the string to length of 6.
                raw_block_id.insert(raw_block_id.begin(), 12 - raw_block_id.length(), '0');
                const std::string block_id_un_base64 = raw_block_id + get_uuid();
                const std::string block_id(to_base64(reinterpret_cast<const unsigned char*>(block_id_un_base64.c_str()), block_id_un_base64.size()));
                put_block_list_request_base::block_item block;
                block.id = block_id;
                block.type = put_block_list_request_base::block_type::uncommitted;
                block_list.push_back(block);
                auto single_put = std::async(std::launch::async, [block_id, this, buffer, length, &container, &blob, &parallel, &mutex, &cv_mutex, &cv](){
                        {
                            std::unique_lock<std::mutex> lk(cv_mutex);
                            cv.wait(lk, [&parallel, &mutex]() {
                                    std::lock_guard<std::mutex> lock(mutex);
                                    if(parallel > 0)
                                    {
                                        --parallel;
                                        return true;
                                    }
                                    return false;
                                });
                        }

                        const auto blockResult = m_blobClient->upload_block_from_buffer(container, blob, block_id, buffer, length).get();
                        free(buffer);

                        {
                            std::lock_guard<std::mutex> lock(mutex);
                            ++parallel;
                            cv.notify_one();
                        }

                        int result = 0;
                        if(!blockResult.success())
                        {
                            result = std::stoi(blockResult.error().code);
                            if (0 == result) {
                                // It seems that timeout requests has no code setup
                                result = 503;
                            }
                        }
                        return result;
                    });
                task_list.push_back(std::move(single_put));
            }

            // wait for the rest of tasks
            for(auto &task: task_list)
            {
                const auto r = task.get();
                if(0 == result)
                {
                    result = r;
                }
            }
            if(result == 0)
            {
                const auto r = m_blobClient->put_block_list(container, blob, block_list, metadata).get();
                if(!r.success())
                {
                    result = std::stoi(r.error().code);
                    logger::log(log_level::error, "put_block_list failed in upload_file_to_blob.  error code = %d, sourcePath = %s, container = %s, blob = %s.", result, sourcePath.c_str(), container.c_str(), blob.c_str());
                    if (0 == result) {
                        result = unknown_error;
                    }
                }
            }

            ifs.close();
            errno = result;
        }

        off_t get_file_size(const char* path)
        {
            struct stat st;
            if(stat(path, &st) == 0)
            {
                return st.st_size;
            }
            return -1;
        }

        void blob_client_wrapper::download_blob_to_stream(const std::string &container, const std::string &blob, unsigned long long offset, unsigned long long size, std::ostream &os)
        {
            if(!is_valid())
            {
                errno = client_not_init;
                return;
            }

            try
            {
                auto task = m_blobClient->download_blob_to_stream(container, blob, offset, size, os);
                task.wait();
                auto result = task.get();

                if(!result.success())
                {
                    errno = std::stoi(result.error().code);
                }
                else
                {
                    errno = 0;
                }
            }
            catch(std::exception& ex)
            {
                logger::log(log_level::error, "Unknown failure in download_blob_to_stream.  ex.what() = %s, container = %s, blob = %s.", ex.what(), container.c_str(), blob.c_str());
                errno = unknown_error;
                return;
            }
        }

        void blob_client_wrapper::download_blob_to_file(const std::string &container, const std::string &blob, const std::string &destPath, time_t &returned_last_modified, size_t parallel)
        {
            if(!is_valid())
            {
                errno = client_not_init;
                return;
            }

            const size_t downloaders = std::min(parallel, static_cast<size_t>(m_concurrency));
            storage_outcome<chunk_property> firstChunk;
            try
            {
                // Download the first chunk of the blob. The response will contain required blob metadata as well.
                int errcode = 0;
                std::ofstream os(destPath.c_str(), std::ofstream::binary | std::ofstream::out);
                firstChunk = m_blobClient->get_chunk_to_stream_sync(container, blob, 0, DOWNLOAD_CHUNK_SIZE, os);
                os.close();
                if (!os) {
                    logger::log(log_level::error, "get_chunk_to_stream_async failed for firstchunk in download_blob_to_file.  container = %s, blob = %s, destPath = %s.", container.c_str(), blob.c_str(), destPath.c_str());
                    errno = unknown_error;
                    return;
                }
                if (!firstChunk.success())
                {
                    if (constants::code_request_range_not_satisfiable != firstChunk.error().code) {
                        errno = std::stoi(firstChunk.error().code);
                        return;
                    }
                    // The only reason for constants::code_request_range_not_satisfiable on the first chunk is zero
                    // blob size, so proceed as there is no error.
                }
                // Smoke check if the total size is known, otherwise - fail.
                if (firstChunk.response().totalSize < 0) {
                    errno = blob_no_content_range;
                    return;
                }

                // Get required metadata - etag to verify all future chunks and the total blob size.
                const auto originalEtag = firstChunk.response().etag;
                const auto length = static_cast<unsigned long long>(firstChunk.response().totalSize);

                // Create or resize the target file if already exist.
                create_or_resize_file(destPath, length);

                // Download the rest.
                const auto left = length - firstChunk.response().size;
                const auto chunk_size = std::max(DOWNLOAD_CHUNK_SIZE, (left + downloaders - 1)/ downloaders);
                std::vector<std::future<int>> task_list;
                for(unsigned long long offset = firstChunk.response().size; offset < length; offset += chunk_size)
                {
                    const auto range = std::min(chunk_size, length - offset);
                    auto single_download = std::async(std::launch::async, [originalEtag, offset, range, this, &destPath, &container, &blob](){
                            // Note, keep std::ios_base::in to prevent truncating of the file.
                            std::ofstream output(destPath.c_str(), std::ios_base::out |  std::ios_base::in);
                            output.seekp(offset);
                            auto chunk = m_blobClient->get_chunk_to_stream_sync(container, blob, offset, range, output);
                            output.close();
                            if(!chunk.success())
                            {
                                // Looks like the blob has been replaced by smaller one - ask user to retry.
                                if (constants::code_request_range_not_satisfiable == chunk.error().code) {
                                    return EAGAIN;
                                }
                                return std::stoi(chunk.error().code);
                            }
                            // The etag has been changed - ask user to retry.
                            if (originalEtag != chunk.response().etag) {
                                return EAGAIN;
                            }
                            // Check for any writing errors.
                            if (!output) {
                                logger::log(log_level::error, "get_chunk_to_stream_async failure in download_blob_to_file.  container = %s, blob = %s, destPath = %s, offset = %llu, range = %llu.", container.c_str(), blob.c_str(), destPath.c_str(), offset, range);
                                return unknown_error;
                            }
                            return 0;
                        });
                    task_list.push_back(std::move(single_download));
                }

                // Wait for workers to complete downloading.
                for(size_t i = 0; i < task_list.size(); ++i)
                {
                    task_list[i].wait();
                    auto result = task_list[i].get();
                    // let's report the first encountered error for consistency.
                    if (0 != result && errcode == 0) {
                        errcode = result;
                    }
                }
                errno = errcode;
            }
            catch(std::exception& ex)
            {
                logger::log(log_level::error, "Unknown failure in download_blob_to_file.  ex.what() = %s, container = %s, blob = %s, destPath = %s.", ex.what(), container.c_str(), blob.c_str(), destPath.c_str());
                errno = unknown_error;
                return;
            }

            returned_last_modified = firstChunk.response().last_modified;
            return;
        }

        blob_property blob_client_wrapper::get_blob_property(const std::string &container, const std::string &blob)
        {
            if(!is_valid())
            {
                errno = client_not_init;
                return blob_property(false);
            }

            try
            {
                auto result = m_blobClient->get_blob_properties(container, blob).get();
                if(!result.success())
                {
                    errno = std::stoi(result.error().code);
                    return blob_property(false);
                }
                else
                {
                    errno = 0;
                    return result.response();
                }
            }
            catch(std::exception& ex)
            {
                logger::log(log_level::error, "Unknown failure in get_blob_property.  ex.what() = %s, container = %s, blob = %s.", ex.what(), container.c_str(), blob.c_str());
                errno = unknown_error;
                return blob_property(false);
            }
        }

        bool blob_client_wrapper::blob_exists(const std::string &container, const std::string &blob)
        {
            if(!is_valid())
            {
                errno = client_not_init;
                return false;
            }

            try
            {
                auto blobProperty = get_blob_property(container, blob);
                if(blobProperty.valid())
                {
                    errno = 0;
                    return true;
                }
                return false;
            }
            catch(std::exception& ex)
            {
                logger::log(log_level::error, "Unknown failure in blob_exists.  ex.what() = %s, container = %s, blob = %s.", ex.what(), container.c_str(), blob.c_str());
                errno = unknown_error;
                return false;
            }
        }

        void blob_client_wrapper::delete_blob(const std::string &container, const std::string &blob)
        {
            if(!is_valid())
            {
                errno = client_not_init;
                return;
            }
            if(container.empty() || blob.empty())
            {
                errno = invalid_parameters;
                return;
            }

            try
            {
                auto task = m_blobClient->delete_blob(container, blob);
                task.wait();
                auto result = task.get();

                if(!result.success())
                {
                    errno = std::stoi(result.error().code);
                }
                else
                {
                    errno = 0;
                }
            }
            catch(std::exception& ex)
            {
                logger::log(log_level::error, "Unknown failure in delete_blob.  ex.what() = %s, container = %s, blob = %s.", ex.what(), container.c_str(), blob.c_str());
                errno = unknown_error;
                return;
            }
        }

        void blob_client_wrapper::start_copy(const std::string &sourceContainer, const std::string &sourceBlob, const std::string &destContainer, const std::string &destBlob)
        {

            if(!is_valid())
            {
                errno = client_not_init;
                return;
            }
            if(sourceContainer.empty() || sourceBlob.empty() ||
               destContainer.empty() || destBlob.empty())
            {
                errno = invalid_parameters;
                return;
            }

            try
            {
                auto task = m_blobClient->start_copy(sourceContainer, sourceBlob, destContainer, destBlob);
                task.wait();
                auto result = task.get();

                if(!result.success())
                {
                    errno = std::stoi(result.error().code);
                }
                else
                {
                    errno = 0;
                }
            }
            catch(std::exception& ex)
            {
                logger::log(log_level::error, "Unknown failure in start_copy.  ex.what() = %s, sourceContainer = %s, sourceBlob = %s, destContainer = %s, destBlob = %s.", ex.what(), sourceContainer.c_str(), sourceBlob.c_str(), destContainer.c_str(), destBlob.c_str());
                errno = unknown_error;
                return;
            }
        }

}} // azure::storage_lite
