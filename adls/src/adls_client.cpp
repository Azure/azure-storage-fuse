#include "adls_client.h"
#include "logging.h"
#include "storage_errno.h"
#include "nlohmann_json_parser.h"
#include "create_directory_request.h"
#include "delete_directory_request.h"
#include "get_access_control_request.h"
#include "create_file_request.h"
#include "rename_file_request.h"
#include "append_data_request.h"
#include "flush_data_request.h"

#include <cerrno>
#include <functional>

namespace azure { namespace storage_adls {

    using azure::storage_lite::storage_outcome;
    namespace constants = azure::storage_lite::constants;

    adls_client::adls_client(std::shared_ptr<storage_account> account, int max_concurrency, bool exception_enabled) : m_account(account), m_blob_client(std::make_shared<azure::storage_lite::blob_client>(account, max_concurrency)), m_context(m_blob_client->context()), m_exception_enabled(exception_enabled)
    {
        m_context->set_json_parser(std::make_shared<nlohmann_json_parser>());
    }

    template<class RET, class FUNC>
    RET adls_client::blob_client_adaptor(FUNC func)
    {
        try
        {
            storage_outcome<RET> result = func().get();

            if (result.success() && !m_exception_enabled)
            {
                errno = 0;
            }
            else if (!result.success())
            {
                int error_code = std::stoi(result.error().code);
                if (m_exception_enabled)
                {
                    throw storage_exception(error_code, result.error().code_name, result.error().message);
                }
                else
                {
                    azure::storage_lite::logger::error(result.error().code_name + ": " + result.error().message);
                    errno = error_code;
                }
            }

            return result.response();
        }
        catch (std::exception& e)
        {
            if (m_exception_enabled)
            {
                throw;
            }
            else
            {
                azure::storage_lite::logger::error("Unknown failure: %s", e.what());
                errno = unknown_error;
            }
        }
        return RET();
    }

    void adls_client::create_filesystem(const std::string& filesystem)
    {
        return blob_client_adaptor<void>(std::bind(&azure::storage_lite::blob_client::create_container, m_blob_client, filesystem));
    }

    void adls_client::delete_filesystem(const std::string& filesystem)
    {
        return blob_client_adaptor<void>(std::bind(&azure::storage_lite::blob_client::delete_container, m_blob_client, filesystem));
    }

    bool adls_client::filesystem_exists(const std::string& filesystem)
    {
        bool container_exists = false;
        try
        {
            auto container_properties = blob_client_adaptor<azure::storage_lite::container_property>(std::bind(&azure::storage_lite::blob_client::get_container_properties, m_blob_client, filesystem));
            container_exists = container_properties.valid();
        }
        catch (const storage_exception& e)
        {
            if (e.code != 404)
            {
                throw;
            }
        }
        if (!success() && errno == 404)
        {
            errno = 0;
        }
        return container_exists;
    }

    void adls_client::set_filesystem_properties(const std::string& filesystem, const std::vector<std::pair<std::string, std::string>>& properties)
    {
        return blob_client_adaptor<void>(std::bind(&azure::storage_lite::blob_client::set_container_metadata, m_blob_client, filesystem, properties));
    }

    std::vector<std::pair<std::string, std::string>> adls_client::get_filesystem_properties(const std::string& filesystem)
    {
        auto container_properties = blob_client_adaptor<azure::storage_lite::container_property>(std::bind(&azure::storage_lite::blob_client::get_container_properties, m_blob_client, filesystem));
        std::vector<std::pair<std::string, std::string>> filesystem_properties(std::move(container_properties.metadata));
        return filesystem_properties;
    }

    list_filesystems_result adls_client::list_filesystems_segmented(const std::string& prefix, const std::string& continuation_token, const int max_results)
    {
        auto containers_segment = blob_client_adaptor<azure::storage_lite::list_constainers_segmented_response>(std::bind(&azure::storage_lite::blob_client::list_containers_segmented, m_blob_client, prefix, continuation_token, max_results, false));

        list_filesystems_result result;
        for (const auto& container_item : containers_segment.containers)
        {
            list_filesystems_item fs_item;
            fs_item.name = container_item.name;
            result.filesystems.emplace_back(std::move(fs_item));
        }
        result.continuation_token = containers_segment.next_marker;
        return result;
    }

    void adls_client::create_directory(const std::string& filesystem, const std::string& directory)
    {
        auto http = m_blob_client->client()->get_handle();
        auto request = std::make_shared<create_directory_request>(filesystem, directory);
        auto async_func = std::bind(&azure::storage_lite::async_executor<void>::submit, m_account, request, http, m_context);
        return blob_client_adaptor<void>(async_func);
    }

    void adls_client::delete_directory(const std::string& filesystem, const std::string& directory)
    {
        auto http = m_blob_client->client()->get_handle();
        std::string continuation;
        while (true)
        {
            auto request = std::make_shared<delete_directory_request>(filesystem, directory, continuation);
            auto async_func = std::bind(&azure::storage_lite::async_executor<void>::submit, m_account, request, http, m_context);
            blob_client_adaptor<void>(async_func);
            if (!success())
            {
                return;
            }
            continuation = http->get_response_header(constants::header_ms_continuation);
            if (continuation.empty())
            {
                break;
            }
        }
    }

    bool adls_client::directory_exists(const std::string& filesystem, const std::string& directory)
    {
        bool blob_exists = false;
        try
        {
            auto blob_properties = blob_client_adaptor<azure::storage_lite::blob_property>(std::bind(&azure::storage_lite::blob_client::get_blob_properties, m_blob_client, filesystem, directory));
            blob_exists = blob_properties.valid();
        }
        catch (const storage_exception& e)
        {
            if (e.code != 404)
            {
                throw;
            }
        }
        if (!success() && errno == 404)
        {
            errno = 0;
        }
        return blob_exists;
    }

    void adls_client::move_directory(const std::string& filesystem, const std::string& source_path, const std::string& destination_path)
    {
        return move_directory(filesystem, source_path, filesystem, destination_path);
    }

    void adls_client::move_directory(const std::string& source_filesystem, const std::string& source_path, const std::string& destination_filesystem, const std::string& destination_path)
    {
        auto http = m_blob_client->client()->get_handle();
        // Currently we haven't seen any difference between moving a directory and a file, so we'll just use rename_file_request.
        auto request = std::make_shared<rename_file_request>(source_filesystem, source_path, destination_filesystem, destination_path);
        auto async_func = std::bind(&azure::storage_lite::async_executor<void>::submit, m_account, request, http, m_context);
        return blob_client_adaptor<void>(async_func);
    }

    void adls_client::set_directory_properties(const std::string& filesystem, const std::string& directory, const std::vector<std::pair<std::string, std::string>>& properties)
    {
        return blob_client_adaptor<void>(std::bind(&azure::storage_lite::blob_client::set_blob_metadata, m_blob_client, filesystem, directory, properties));
    }

    std::vector<std::pair<std::string, std::string>> adls_client::get_directory_properties(const std::string& filesystem, const std::string& directory)
    {
        auto blob_properties = blob_client_adaptor<azure::storage_lite::blob_property>(std::bind(&azure::storage_lite::blob_client::get_blob_properties, m_blob_client, filesystem, directory));
        std::vector<std::pair<std::string, std::string>> directory_properties(std::move(blob_properties.metadata));
        auto ite = std::find_if(directory_properties.begin(), directory_properties.end(), [](const std::pair<std::string, std::string>& p)
        {
            return p.first == constants::header_ms_meta_hdi_isfoler + constants::header_ms_meta_prefix_size;
        });
        if (ite != directory_properties.end())
        {
            directory_properties.erase(ite);
        }
        return directory_properties;
    }

    void adls_client::set_directory_access_control(const std::string& filesystem, const std::string& directory, const access_control& acl)
    {
        auto http = m_blob_client->client()->get_handle();
        auto request = std::make_shared<set_access_control_request>(filesystem, directory, acl);
        auto async_func = std::bind(&azure::storage_lite::async_executor<void>::submit, m_account, request, http, m_context);
        return blob_client_adaptor<void>(async_func);
    }

    access_control adls_client::get_directory_access_control(const std::string& filesystem, const std::string& directory)
    {
        auto http = m_blob_client->client()->get_handle();
        auto request = std::make_shared<get_access_control_request>(filesystem, directory);
        auto async_func = std::bind(&azure::storage_lite::async_executor<void>::submit, m_account, request, http, m_context);
        blob_client_adaptor<void>(async_func);

        access_control acl;
        if (success())
        {
            acl.owner = http->get_response_header(constants::header_ms_owner);
            acl.group = http->get_response_header(constants::header_ms_group);
            acl.permissions = http->get_response_header(constants::header_ms_permissions);
            acl.acl = http->get_response_header(constants::header_ms_acl);
        }
        return acl;
    }

    list_paths_result adls_client::list_paths_segmented(const std::string& filesystem, const std::string& directory, bool recursive, const std::string& continuation_token, const int max_results)
    {
        auto http = m_blob_client->client()->get_handle();
        auto request = std::make_shared<list_paths_request>(filesystem, directory, recursive, continuation_token, max_results);
        auto async_func = std::bind(&azure::storage_lite::async_executor<std::vector<list_paths_item>>::submit, m_account, request, http, m_context);

        list_paths_result result;
        result.paths = blob_client_adaptor<std::vector<list_paths_item>>(async_func);
        result.continuation_token = http->get_response_header(constants::header_ms_continuation);
        return result;
    }

    void adls_client::create_file(const std::string& filesystem, const std::string& file)
    {
        auto http = m_blob_client->client()->get_handle();
        auto request = std::make_shared<create_file_request>(filesystem, file);
        auto async_func = std::bind(&azure::storage_lite::async_executor<void>::submit, m_account, request, http, m_context);
        return blob_client_adaptor<void>(async_func);
    }

    void adls_client::append_data_from_stream(const std::string& filesystem, const std::string& file, uint64_t offset, std::istream& in_stream, uint64_t stream_len)
    {
        if (stream_len == 0)
        {
            uint64_t cur = in_stream.tellg();
            in_stream.seekg(0, std::ios_base::end);
            uint64_t end = in_stream.tellg();
            in_stream.seekg(cur);
            stream_len = end - cur;
        }

        auto http = m_blob_client->client()->get_handle();
        auto request = std::make_shared<append_data_request>(filesystem, file, offset, stream_len);
        http->set_input_stream(azure::storage_lite::storage_istream(in_stream));
        http->set_is_input_length_known();
        http->set_input_content_length(stream_len);

        auto async_func = std::bind(&azure::storage_lite::async_executor<void>::submit, m_account, request, http, m_context);
        return blob_client_adaptor<void>(async_func);
    }

    void adls_client::flush_data(const std::string& filesystem, const std::string& file, uint64_t offset)
    {
        auto http = m_blob_client->client()->get_handle();
        auto request = std::make_shared<flush_data_request>(filesystem, file, offset);
        auto async_func = std::bind(&azure::storage_lite::async_executor<void>::submit, m_account, request, http, m_context);
        return blob_client_adaptor<void>(async_func);
    }

    void adls_client::upload_file_from_stream(const std::string& filesystem, const std::string& file, std::istream& in_stream, const std::vector<std::pair<std::string, std::string>>& properties)
    {
        auto blob_client = m_blob_client;
        auto async_task = [blob_client, filesystem, file, &in_stream, properties]()
        {
            return blob_client->upload_block_blob_from_stream(filesystem, file, in_stream, properties);
        };
        return blob_client_adaptor<void>(async_task);
    }

    void adls_client::download_file_to_stream(const std::string& filesystem, const std::string& file, std::ostream& out_stream)
    {
        return download_file_to_stream(filesystem, file, 0, 0, out_stream);
    }

    void adls_client::download_file_to_stream(const std::string& filesystem, const std::string& file, uint64_t offset, uint64_t size, std::ostream& out_stream)
    {
        // std::bind doesn't seem to work since download_blob_to_stream is an overloaded function.
        auto blob_client = m_blob_client;
        auto async_task = [blob_client, filesystem, file, offset, size, &out_stream]()
        {
            return blob_client->download_blob_to_stream(filesystem, file, offset, size, out_stream);
        };
        return blob_client_adaptor<void>(async_task);
    }

    void adls_client::delete_file(const std::string& filesystem, const std::string& file)
    {
        return blob_client_adaptor<void>(std::bind(&azure::storage_lite::blob_client::delete_blob, m_blob_client, filesystem, file, false));
    }

    bool adls_client::file_exists(const std::string& filesystem, const std::string& file)
    {
        bool blob_exists = false;
        try
        {
            auto blob_properties = blob_client_adaptor<azure::storage_lite::blob_property>(std::bind(&azure::storage_lite::blob_client::get_blob_properties, m_blob_client, filesystem, file));
            blob_exists = blob_properties.valid();
        }
        catch (const storage_exception& e)
        {
            if (e.code != 404)
            {
                throw;
            }
        }
        if (!success() && errno == 404)
        {
            errno = 0;
        }
        return blob_exists;
    }

    void adls_client::move_file(const std::string& filesystem, const std::string& source_path, const std::string& destination_path)
    {
        return move_file(filesystem, source_path, filesystem, destination_path);
    }

    void adls_client::move_file(const std::string& source_filesystem, const std::string& source_path, const std::string& destination_filesystem, const std::string& destination_path)
    {
        auto http = m_blob_client->client()->get_handle();
        auto request = std::make_shared<rename_file_request>(source_filesystem, source_path, destination_filesystem, destination_path);
        auto async_func = std::bind(&azure::storage_lite::async_executor<void>::submit, m_account, request, http, m_context);
        return blob_client_adaptor<void>(async_func);
    }

    void adls_client::set_file_properties(const std::string& filesystem, const std::string& file, const std::vector<std::pair<std::string, std::string>>& properties)
    {
        return blob_client_adaptor<void>(std::bind(&azure::storage_lite::blob_client::set_blob_metadata, m_blob_client, filesystem, file, properties));
    }

    std::vector<std::pair<std::string, std::string>> adls_client::get_file_properties(const std::string& filesystem, const std::string& file)
    {
        auto blob_properties = blob_client_adaptor<azure::storage_lite::blob_property>(std::bind(&azure::storage_lite::blob_client::get_blob_properties, m_blob_client, filesystem, file));
        std::vector<std::pair<std::string, std::string>> file_properties(std::move(blob_properties.metadata));
        return file_properties;
    }

    void adls_client::set_file_access_control(const std::string& filesystem, const std::string& file, const access_control& acl)
    {
        auto http = m_blob_client->client()->get_handle();
        auto request = std::make_shared<set_access_control_request>(filesystem, file, acl);
        auto async_func = std::bind(&azure::storage_lite::async_executor<void>::submit, m_account, request, http, m_context);
        return blob_client_adaptor<void>(async_func);
    }

    access_control adls_client::get_file_access_control(const std::string& filesystem, const std::string& file)
    {
        auto http = m_blob_client->client()->get_handle();
        auto request = std::make_shared<get_access_control_request>(filesystem, file);
        auto async_func = std::bind(&azure::storage_lite::async_executor<void>::submit, m_account, request, http, m_context);
        blob_client_adaptor<void>(async_func);

        access_control acl;
        if (success())
        {
            acl.owner = http->get_response_header(constants::header_ms_owner);
            acl.group = http->get_response_header(constants::header_ms_group);
            acl.permissions = http->get_response_header(constants::header_ms_permissions);
            acl.acl = http->get_response_header(constants::header_ms_acl);
        }
        return acl;
    }

}}  // azure::storage_adls
