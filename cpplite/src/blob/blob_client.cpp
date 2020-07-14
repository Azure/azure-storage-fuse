#include <algorithm>
#include <future>
#include <sstream>

#ifdef _WIN32
#include <BaseTsd.h>
typedef SSIZE_T ssize_t;
#endif

#include "blob/blob_client.h"

#include "blob/download_blob_request.h"
#include "blob/create_block_blob_request.h"
#include "blob/delete_blob_request.h"
#include "blob/copy_blob_request.h"
#include "blob/create_container_request.h"
#include "blob/delete_container_request.h"
#include "blob/list_containers_request.h"
#include "blob/list_blobs_request.h"
#include "blob/get_block_list_request.h"
#include "blob/get_blob_property_request.h"
#include "blob/get_container_property_request.h"
#include "blob/put_block_request.h"
#include "blob/put_block_list_request.h"
#include "blob/append_block_request.h"
#include "blob/put_page_request.h"
#include "blob/get_page_ranges_request.h"
#include "blob/set_container_metadata_request.h"
#include "blob/set_blob_metadata_request.h"

#include "constants.h"
#include "storage_errno.h"
#include "executor.h"
#include "utility.h"
#include "base64.h"
#include "tinyxml2_parser.h"
#include "mstream.h"

#include <curl/curl.h>

namespace azure {  namespace storage_lite {

namespace {

// Return content size from content-range header or -1 if cannot be obtained.
ssize_t get_length_from_content_range(const std::string &header)
{
   const auto pos = header.rfind('/');
   if (std::string::npos == pos) {
      return -1;
   }
   const auto lengthStr = header.substr(pos + 1);
   ssize_t result;
   if (!(std::istringstream(lengthStr) >> result)) {
      return -1;
   }
   return result;
}

} // noname namespace

storage_outcome<chunk_property> blob_client::get_chunk_to_stream_sync(const std::string &container, const std::string &blob, unsigned long long offset, unsigned long long size, std::ostream &os)
{
    auto http = m_client->get_handle();
    auto request = std::make_shared<download_blob_request>(container, blob);
    if (size > 0) {
        request->set_start_byte(offset);
        request->set_end_byte(offset + size - 1);
    }
    else {
        request->set_start_byte(offset);
    }

    http->set_output_stream(storage_ostream(os));

    // TODO: async submit transfered to sync operation. This can be utilized.
    const auto response = async_executor<void>::submit(m_account, request, http, m_context).get();
    if (response.success())
    {
        chunk_property property{};
        property.etag = http->get_response_header(constants::header_etag);
        property.totalSize = get_length_from_content_range(http->get_response_header(constants::header_content_range));
        std::istringstream(http->get_response_header(constants::header_content_length)) >> property.size;
        property.last_modified = curl_getdate(http->get_response_header(constants::header_last_modified).c_str(), NULL);
        return storage_outcome<chunk_property>(property);
    }
    return storage_outcome<chunk_property>(storage_error(response.error()));
}

std::future<storage_outcome<void>> blob_client::download_blob_to_stream(const std::string &container, const std::string &blob, unsigned long long offset, unsigned long long size, std::ostream &os)
{
    auto http = m_client->get_handle();

    auto request = std::make_shared<download_blob_request>(container, blob);

    if (size > 0) {
        request->set_start_byte(offset);
        request->set_end_byte(offset + size - 1);
    }
    else {
        request->set_start_byte(offset);
    }

    http->set_output_stream(storage_ostream(os));

    return async_executor<void>::submit(m_account, request, http, m_context);
}

std::future<storage_outcome<void>> blob_client::download_blob_to_buffer(const std::string &container, const std::string &blob, unsigned long long offset, unsigned long long size, char* buffer, int parallelism)
{
    parallelism = std::min(parallelism, int(concurrency()));

    const uint64_t grain_size = 64 * 1024;
    uint64_t block_size = size / parallelism;
    block_size = (block_size + grain_size - 1) / grain_size * grain_size;
    block_size = std::min(block_size, constants::default_block_size);

    int num_blocks = int((size + block_size - 1) / block_size);

    struct concurrent_task_info
    {
        std::string container;
        std::string blob;
        char* buffer;
        uint64_t download_offset;
        uint64_t download_size;
        uint64_t block_size;
        int num_blocks;
    };
    struct concurrent_task_context
    {
        std::atomic<int> num_workers{ 0 };
        std::atomic<int> block_index{ 0 };
        std::atomic<bool> failed{ false };
        storage_error failed_reason;

        std::promise<storage_outcome<void>> task_promise;
        std::vector<std::future<void>> task_futures;
    };

    auto info = std::make_shared<concurrent_task_info>(concurrent_task_info{ container, blob, buffer, offset, size, block_size, num_blocks });
    auto context = std::make_shared<concurrent_task_context>();
    context->num_workers = parallelism;

    auto thread_download_func = [this, info, context]()
    {
        while (true)
        {
            int i = context->block_index.fetch_add(1);
            if (i >= info->num_blocks || context->failed)
            {
                break;
            }
            char* block_buffer = info->buffer + info->block_size * i;
            uint64_t block_size = std::min(info->block_size, info->download_size - info->block_size * i);

            auto http = m_client->get_handle();
            auto request = std::make_shared<download_blob_request>(info->container, info->blob);
            request->set_start_byte(info->download_offset + info->block_size * i);
            request->set_end_byte(request->start_byte() + block_size - 1);

            auto os = std::make_shared<omstream>(block_buffer, block_size);
            http->set_output_stream(storage_ostream(os));

            auto result = async_executor<void>::submit(m_account, request, http, m_context).get();

            if (!result.success() && !context->failed.exchange(true))
            {
                context->failed_reason = result.error();
            }
        }
        if (context->num_workers.fetch_sub(1) == 1)
        {
            // I'm the last worker thread
            context->task_promise.set_value(context->failed ? storage_outcome<void>(context->failed_reason) : storage_outcome<void>());
        }
    };

    for (int i = 0; i < parallelism; ++i)
    {
        context->task_futures.emplace_back(std::async(std::launch::async, thread_download_func));
    }

    return context->task_promise.get_future();
}

std::future<storage_outcome<void>> blob_client::upload_block_blob_from_stream(const std::string &container, const std::string &blob, std::istream &is, const std::vector<std::pair<std::string, std::string>> &metadata)
{
    auto http = m_client->get_handle();

    auto request = std::make_shared<create_block_blob_request>(container, blob);

    auto cur = is.tellg();
    is.seekg(0, std::ios_base::end);
    auto end = is.tellg();
    is.seekg(cur);
    request->set_content_length(static_cast<unsigned int>(end - cur));
    if (metadata.size() > 0)
    {
        request->set_metadata(metadata);
    }

    http->set_input_stream(storage_istream(is));

    return async_executor<void>::submit(m_account, request, http, m_context);
}

std::future<storage_outcome<void>> blob_client::upload_block_blob_from_stream(const std::string &container, const std::string &blob, std::istream &is, const std::vector<std::pair<std::string, std::string>> &metadata, uint64_t streamlen)
{
    auto http = m_client->get_handle();

    auto request = std::make_shared<create_block_blob_request>(container, blob);

    request->set_content_length(static_cast<unsigned int>(streamlen));
    if (metadata.size() > 0)
    {
        request->set_metadata(metadata);
    }

    http->set_input_stream(storage_istream(is));
    http->set_is_input_length_known();
    http->set_input_content_length(streamlen);

    return async_executor<void>::submit(m_account, request, http, m_context);
}

std::future<storage_outcome<void>> blob_client::upload_block_blob_from_buffer(const std::string &container, const std::string &blob, const char* buffer, const std::vector<std::pair<std::string, std::string>> &metadata, uint64_t bufferlen, int parallelism)
{
    if (bufferlen > constants::max_num_blocks * constants::max_block_size)
    {
        storage_error error;
        error.code = std::to_string(blob_too_big);
        std::promise<storage_outcome<void>> promise;
        promise.set_value(storage_outcome<void>(error));
        return promise.get_future();
    }

    parallelism = std::min(parallelism, int(concurrency()));

    const uint64_t grain_size = 4 * 1024 * 1024;
    uint64_t block_size = bufferlen / constants::max_num_blocks;
    block_size = (block_size + grain_size - 1) / grain_size * grain_size;
    block_size = std::min(block_size, constants::max_block_size);
    block_size = std::max(block_size, constants::default_block_size);

    int num_blocks = int((bufferlen + block_size - 1) / block_size);

    std::vector<put_block_list_request_base::block_item> block_list;
    block_list.reserve(num_blocks);
    std::string uuid = get_uuid();
    for (int i = 0; i < num_blocks; ++i)
    {
        std::string block_id = std::to_string(i);
        block_id = uuid + std::string(48 - uuid.length() - block_id.length(), '-') + block_id;
        block_id = to_base64(reinterpret_cast<const unsigned char*>(block_id.data()), block_id.length());
        block_list.emplace_back(put_block_list_request_base::block_item{ std::move(block_id), put_block_list_request_base::block_type::uncommitted });
    }

    struct concurrent_task_info
    {
        std::string container;
        std::string blob;
        const char* buffer;
        uint64_t blob_size;
        uint64_t block_size;
        int num_blocks;
        std::vector<put_block_list_request_base::block_item> block_list;
        std::vector<std::pair<std::string, std::string>> metadata;
    };
    struct concurrent_task_context
    {
        std::atomic<int> num_workers{ 0 };
        std::atomic<int> block_index{ 0 };
        std::atomic<bool> failed{ false };
        storage_error failed_reason;

        std::promise<storage_outcome<void>> task_promise;
        std::vector<std::future<void>> task_futures;
    };
    auto info = std::make_shared<concurrent_task_info>(concurrent_task_info{ container, blob, buffer, bufferlen, block_size, num_blocks, std::move(block_list), metadata });
    auto context = std::make_shared<concurrent_task_context>();
    context->num_workers = parallelism;

    auto thread_upload_func = [this, info, context]()
    {
        while (true)
        {
            int i = context->block_index.fetch_add(1);
            if (i >= info->num_blocks || context->failed)
            {
                break;
            }
            const char* block_buffer = info->buffer + info->block_size * i;
            uint64_t block_size = std::min(info->block_size, info->blob_size - info->block_size * i);
            auto result = upload_block_from_buffer(info->container, info->blob, info->block_list[i].id, block_buffer, block_size).get();

            if (!result.success() && !context->failed.exchange(true))
            {
                context->failed_reason = result.error();
            }
        }
        if (context->num_workers.fetch_sub(1) == 1)
        {
            // I'm the last worker thread
            if (!context->failed)
            {
                auto result = put_block_list(info->container, info->blob, info->block_list, info->metadata).get();
                if (!result.success())
                {
                    context->failed.store(true);
                    context->failed_reason = result.error();
                }
            }
            context->task_promise.set_value(context->failed ? storage_outcome<void>(context->failed_reason) : storage_outcome<void>());
        }
    };

    for (int i = 0; i < parallelism; ++i)
    {
        context->task_futures.emplace_back(std::async(std::launch::async, thread_upload_func));
    }

    return context->task_promise.get_future();
}

std::future<storage_outcome<void>> blob_client::upload_block_from_buffer(const std::string &container, const std::string &blob, const std::string &blockid, const char* buff, uint64_t bufferlen)
{
    auto http = m_client->get_handle();

    auto request = std::make_shared<put_block_request>(container, blob, blockid);
    request->set_content_length(static_cast<unsigned int>(bufferlen));

    auto is = std::make_shared<imstream>(buff, bufferlen);
    http->set_input_stream(storage_istream(is));
    http->set_is_input_length_known();
    http->set_input_content_length(bufferlen);

    return async_executor<void>::submit(m_account, request, http, m_context);
}

std::future<storage_outcome<void>> blob_client::delete_blob(const std::string &container, const std::string &blob, bool delete_snapshots)
{
    auto http = m_client->get_handle();

    auto request = std::make_shared<delete_blob_request>(container, blob, delete_snapshots);

    return async_executor<void>::submit(m_account, request, http, m_context);
}

std::future<storage_outcome<void>> blob_client::create_container(const std::string &container)
{
    auto http = m_client->get_handle();

    auto request = std::make_shared<create_container_request>(container);

    return async_executor<void>::submit(m_account, request, http, m_context);
}

std::future<storage_outcome<void>> blob_client::delete_container(const std::string &container)
{
    auto http = m_client->get_handle();

    auto request = std::make_shared<delete_container_request>(container);

    return async_executor<void>::submit(m_account, request, http, m_context);
}

std::future<storage_outcome<container_property>> blob_client::get_container_properties(const std::string &container)
{
    auto http = m_client->get_handle();

    auto request = std::make_shared<get_container_property_request>(container);

    std::shared_future<storage_outcome<void>> response = async_executor<void>::submit(m_account, request, http, m_context);

    std::future<storage_outcome<container_property>> container_properties = std::async(std::launch::deferred, [http, response]()
    {
        if (response.get().success())
        {
            container_property properties(true);
            properties.etag = http->get_response_header(constants::header_etag);

            auto& headers = http->get_response_headers();
            for (auto iter = headers.begin(); iter != headers.end(); ++iter)
            {
                if (iter->first.find(constants::header_ms_meta_prefix) == 0)
                {
                    properties.metadata.push_back(std::make_pair(iter->first.substr(constants::header_ms_meta_prefix_size), iter->second));
                }
            }
            return storage_outcome<container_property>(properties);
        }
        else
        {
            return storage_outcome<container_property>(response.get().error());
        }
    });
    return container_properties;
}

std::future<storage_outcome<void>> blob_client::set_container_metadata(const std::string &container, const std::vector<std::pair<std::string, std::string>>& metadata)
{
    auto http = m_client->get_handle();

    auto request = std::make_shared<set_container_metadata_request>(container, metadata);

    return async_executor<void>::submit(m_account, request, http, m_context);
}

std::future<storage_outcome<list_constainers_segmented_response>> blob_client::list_containers_segmented(const std::string &prefix, const std::string& continuation_token, const int max_result, bool include_metadata)
{
    auto http = m_client->get_handle();

    auto request = std::make_shared<list_containers_request>(prefix, include_metadata);
    request->set_maxresults(max_result);
    request->set_marker(continuation_token);

    return async_executor<list_constainers_segmented_response>::submit(m_account, request, http, m_context);
}

std::future<storage_outcome<list_blobs_segmented_response>> blob_client::list_blobs_segmented(const std::string &container, const std::string &delimiter, const std::string &continuation_token, const std::string &prefix, int max_results)
{
    auto http = m_client->get_handle();

    auto request = std::make_shared<list_blobs_segmented_request>(container, delimiter, continuation_token, prefix);
    request->set_maxresults(max_results);
    request->set_includes(list_blobs_request_base::include::metadata);

    return async_executor<list_blobs_segmented_response>::submit(m_account, request, http, m_context);
}

std::future<storage_outcome<get_block_list_response>> blob_client::get_block_list(const std::string &container, const std::string &blob)
{
    auto http = m_client->get_handle();

    auto request = std::make_shared<get_block_list_request>(container, blob);

    return async_executor<get_block_list_response>::submit(m_account, request, http, m_context);
}

std::future<storage_outcome<blob_property>> blob_client::get_blob_properties(const std::string &container, const std::string &blob)
{
    auto http = m_client->get_handle();

    auto request = std::make_shared<get_blob_property_request>(container, blob);

    std::shared_future<storage_outcome<void>> response = async_executor<void>::submit(m_account, request, http, m_context);

    std::future<storage_outcome<blob_property>> blob_properties = std::async(std::launch::deferred, [http, response]()
    {
        if (response.get().success())
        {
            blob_property properties(true);
            properties.cache_control = http->get_response_header(constants::header_cache_control);
            properties.content_disposition = http->get_response_header(constants::header_content_disposition);
            properties.content_encoding = http->get_response_header(constants::header_content_encoding);
            properties.content_language = http->get_response_header(constants::header_content_language);
            properties.content_md5 = http->get_response_header(constants::header_content_md5);
            properties.content_type = http->get_response_header(constants::header_content_type);
            properties.etag = http->get_response_header(constants::header_etag);
            properties.copy_status = http->get_response_header(constants::header_ms_copy_status);
            properties.last_modified = curl_getdate(http->get_response_header(constants::header_last_modified).c_str(), NULL);
            std::string::size_type sz = 0;
            std::string contentLength = http->get_response_header(constants::header_content_length);
            if (contentLength.length() > 0)
            {
                properties.size = std::stoull(contentLength, &sz, 0);
            }

            auto& headers = http->get_response_headers();
            for (auto iter = headers.begin(); iter != headers.end(); ++iter)
            {
                if (iter->first.find(constants::header_ms_meta_prefix) == 0)
                {
                    // We need to strip ten characters from the front of the key to account for "x-ms-meta-".
                    properties.metadata.push_back(std::make_pair(iter->first.substr(constants::header_ms_meta_prefix_size), iter->second));
                }
            }
            return storage_outcome<blob_property>(properties);
        }
        else
        {
            return storage_outcome<blob_property>(response.get().error());
        }
    });
    return blob_properties;
}

std::future<storage_outcome<void>> blob_client::set_blob_metadata(const std::string &container, const std::string& blob, const std::vector<std::pair<std::string, std::string>>& metadata)
{
    auto http = m_client->get_handle();

    auto request = std::make_shared<set_blob_metadata_request>(container, blob, metadata);

    return async_executor<void>::submit(m_account, request, http, m_context);
}

std::future<storage_outcome<void>> blob_client::upload_block_from_stream(const std::string &container, const std::string &blob, const std::string &blockid, std::istream &is)
{
    auto http = m_client->get_handle();

    auto request = std::make_shared<put_block_request>(container, blob, blockid);

    auto cur = is.tellg();
    is.seekg(0, std::ios_base::end);
    auto end = is.tellg();
    is.seekg(cur);
    request->set_content_length(static_cast<unsigned int>(end - cur));

    http->set_input_stream(storage_istream(is));

    return async_executor<void>::submit(m_account, request, http, m_context);
}

std::future<storage_outcome<void>> blob_client::upload_block_from_stream(const std::string &container, const std::string &blob, const std::string &blockid, std::istream &is, uint64_t streamlen)
{
    auto http = m_client->get_handle();

    auto request = std::make_shared<put_block_request>(container, blob, blockid);
    request->set_content_length(static_cast<unsigned int>(streamlen));

    http->set_input_stream(storage_istream(is));
    http->set_is_input_length_known();
    http->set_input_content_length(streamlen);

    return async_executor<void>::submit(m_account, request, http, m_context);
}

std::future<storage_outcome<void>> blob_client::put_block_list(const std::string &container, const std::string &blob, const std::vector<put_block_list_request_base::block_item> &block_list, const std::vector<std::pair<std::string, std::string>> &metadata)
{
    auto http = m_client->get_handle();

    auto request = std::make_shared<put_block_list_request>(container, blob);
    request->set_block_list(block_list);
    if (metadata.size() > 0)
    {
        request->set_metadata(metadata);
    }

    return async_executor<void>::submit(m_account, request, http, m_context);
}

std::future<storage_outcome<void>> blob_client::create_append_blob(const std::string &container, const std::string &blob)
{
    auto http = m_client->get_handle();

    auto request = std::make_shared<create_append_blob_request>(container, blob);

    return async_executor<void>::submit(m_account, request, http, m_context);
}

std::future<storage_outcome<void>> blob_client::append_block_from_stream(const std::string &container, const std::string &blob, std::istream &is)
{
    auto http = m_client->get_handle();

    auto request = std::make_shared<append_block_request>(container, blob);

    auto cur = is.tellg();
    is.seekg(0, std::ios_base::end);
    auto end = is.tellg();
    is.seekg(cur);
    request->set_content_length(static_cast<unsigned int>(end - cur));

    http->set_input_stream(storage_istream(is));

    return async_executor<void>::submit(m_account, request, http, m_context);
}

std::future<storage_outcome<void>> blob_client::create_page_blob(const std::string &container, const std::string &blob, unsigned long long size)
{
    auto http = m_client->get_handle();

    auto request = std::make_shared<create_page_blob_request>(container, blob, size);

    return async_executor<void>::submit(m_account, request, http, m_context);
}

std::future<storage_outcome<void>> blob_client::put_page_from_stream(const std::string &container, const std::string &blob, unsigned long long offset, unsigned long long size, std::istream &is)
{
    auto http = m_client->get_handle();

    auto request = std::make_shared<put_page_request>(container, blob);
    if (size > 0)
    {
        request->set_start_byte(offset);
        request->set_end_byte(offset + size - 1);
    }
    else
    {
        request->set_start_byte(offset);
    }

    auto cur = is.tellg();
    is.seekg(0, std::ios_base::end);
    auto end = is.tellg();
    is.seekg(cur);
    auto stream_size = static_cast<unsigned int>(end - cur);
    request->set_content_length(stream_size);

    http->set_input_stream(storage_istream(is));

    return async_executor<void>::submit(m_account, request, http, m_context);
}

std::future<storage_outcome<void>> blob_client::clear_page(const std::string &container, const std::string &blob, unsigned long long offset, unsigned long long size)
{
    auto http = m_client->get_handle();

    auto request = std::make_shared<put_page_request>(container, blob, true);
    if (size > 0)
    {
        request->set_start_byte(offset);
        request->set_end_byte(offset + size - 1);
    }
    else
    {
        request->set_start_byte(offset);
    }

    return async_executor<void>::submit(m_account, request, http, m_context);
}

std::future<storage_outcome<get_page_ranges_response>> blob_client::get_page_ranges(const std::string &container, const std::string &blob, unsigned long long offset, unsigned long long size)
{
    auto http = m_client->get_handle();

    auto request = std::make_shared<get_page_ranges_request>(container, blob);
    if (size > 0)
    {
        request->set_start_byte(offset);
        request->set_end_byte(offset + size - 1);
    }
    else
    {
        request->set_start_byte(offset);
    }

    return async_executor<get_page_ranges_response>::submit(m_account, request, http, m_context);
}

std::future<storage_outcome<void>> blob_client::start_copy(const std::string &sourceContainer, const std::string &sourceBlob, const std::string &destContainer, const std::string &destBlob)
{
    auto http = m_client->get_handle();

    auto request = std::make_shared<copy_blob_request>(sourceContainer, sourceBlob, destContainer, destBlob);

    return async_executor<void>::submit(m_account, request, http, m_context);
}

}}
