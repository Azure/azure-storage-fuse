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

namespace microsoft_azure {
namespace storage {

class blob_client {
public:
    blob_client(std::shared_ptr<storage_account> account, int size)
        : m_account(account) {
        m_context = std::make_shared<executor_context>(std::make_shared<tinyxml2_parser>(), std::make_shared<retry_policy>());
        m_client = std::make_shared<CurlEasyClient>(size);
    }

    std::shared_ptr<CurlEasyClient> client() const {
        return m_client;
    }

    std::shared_ptr<storage_account> account() const {
        return m_account;
    }

    unsigned int concurrency() const {
        return m_client->size();
    }

    AZURE_STORAGE_API std::future<storage_outcome<void>> download_blob_to_stream(const std::string &container, const std::string &blob, unsigned long long offset, unsigned long long size, std::ostream &os);

    AZURE_STORAGE_API std::future<storage_outcome<void>> upload_block_blob_from_stream(const std::string &container, const std::string &blob, std::istream &is, const std::vector<std::pair<std::string, std::string>> &metadata);

    AZURE_STORAGE_API std::future<storage_outcome<void>> delete_blob(const std::string &container, const std::string &blob, bool delete_snapshots = false);

    AZURE_STORAGE_API std::future<storage_outcome<void>> create_container(const std::string &container);

    AZURE_STORAGE_API std::future<storage_outcome<void>> delete_container(const std::string &container);

    AZURE_STORAGE_API storage_outcome<container_property> get_container_property(const std::string &container);

    //AZURE_STORAGE_API std::vector<list_containers_item> list_containers(const std::string &prefix, bool include_metadata = false);

    AZURE_STORAGE_API std::future<storage_outcome<list_containers_response>> list_containers(const std::string &prefix, bool include_metadata = false);

    AZURE_STORAGE_API std::future<storage_outcome<list_blobs_response>> list_blobs(const std::string &container, const std::string &prefix);

    AZURE_STORAGE_API std::future<storage_outcome<list_blobs_hierarchical_response>> list_blobs_hierarchical(const std::string &container, const std::string &delimiter, const std::string &continuation_token, const std::string &prefix);

    //AZURE_STORAGE_API std::future<storage_outcome<blob_property>> get_blob_property(const std::string &container, const std::string &blob);
    AZURE_STORAGE_API storage_outcome<blob_property> get_blob_property(const std::string &container, const std::string &blob);

    // upload metadata

    AZURE_STORAGE_API std::future<storage_outcome<get_block_list_response>> get_block_list(const std::string &container, const std::string &blob);

    AZURE_STORAGE_API std::future<storage_outcome<void>> upload_block_from_stream(const std::string &container, const std::string &blob, const std::string &blockid, std::istream &is);

    AZURE_STORAGE_API std::future<storage_outcome<void>> put_block_list(const std::string &container, const std::string &blob, const std::vector<put_block_list_request_base::block_item> &block_list, const std::vector<std::pair<std::string, std::string>> &metadata);

    AZURE_STORAGE_API std::future<storage_outcome<void>> create_append_blob(const std::string &container, const std::string &blob);

    AZURE_STORAGE_API std::future<storage_outcome<void>> append_block_from_stream(const std::string &container, const std::string &blob, std::istream &is);

    AZURE_STORAGE_API std::future<storage_outcome<void>> create_page_blob(const std::string &container, const std::string &blob, unsigned long long size);

    AZURE_STORAGE_API std::future<storage_outcome<void>> put_page_from_stream(const std::string &container, const std::string &blob, unsigned long long offset, unsigned long long size, std::istream &is);

    AZURE_STORAGE_API std::future<storage_outcome<void>> clear_page(const std::string &container, const std::string &blob, unsigned long long offset, unsigned long long size);

    AZURE_STORAGE_API std::future<storage_outcome<get_page_ranges_response>> get_page_ranges(const std::string &container, const std::string &blob, unsigned long long offset, unsigned long long size);
        
    AZURE_STORAGE_API std::future<storage_outcome<void>> start_copy(const std::string &sourceContainer, const std::string &sourceBlob, const std::string &destContainer, const std::string &destBlob);

private:
    std::shared_ptr<CurlEasyClient> m_client;
    std::shared_ptr<storage_account> m_account;
    std::shared_ptr<executor_context> m_context;
};

class blob_client_wrapper
{
    public:
        explicit blob_client_wrapper(std::shared_ptr<blob_client> blobClient)
            : m_blobClient(blobClient),
              m_invalid(true)
        {
            if(blobClient != NULL)
            {
                m_concurrency = blobClient->concurrency();
            }
        }
        
        explicit blob_client_wrapper(bool invalid)
            : m_invalid(invalid)
        {
        }

        blob_client_wrapper(blob_client_wrapper&& other)
        {
            m_blobClient = other.m_blobClient;
            m_concurrency = other.m_concurrency;
            m_invalid = other.m_invalid;
        }

        blob_client_wrapper& operator=(blob_client_wrapper&& other)
        {
            m_blobClient = other.m_blobClient;
            m_concurrency = other.m_concurrency;
            m_invalid = other.m_invalid;
            return *this;
        }

        bool is_valid() const
        {
            return m_invalid && (m_blobClient != NULL);
        }

        static blob_client_wrapper blob_client_wrapper_init(const std::string &account_name, const std::string &account_key, const unsigned int concurrency);
        /* C++ wrappers without exception but error codes instead */
        /* container level*/
        void create_container(const std::string &container);
        void delete_container(const std::string &container);
        bool container_exists(const std::string &container);
        std::vector<list_containers_item> list_containers(const std::string &prefix, bool include_metadata = false);

        /* blob level */
        list_blobs_hierarchical_response list_blobs_hierarchical(const std::string &container, const std::string &delimiter, const std::string &continuation_token, const std::string &prefix); 
        void put_blob(const std::string &sourcePath, const std::string &container, const std::string blob, const std::vector<std::pair<std::string, std::string>> &metadata = std::vector<std::pair<std::string, std::string>>());
        void upload_block_blob_from_stream(const std::string &container, const std::string blob, std::istream &is, const std::vector<std::pair<std::string, std::string>> &metadata = std::vector<std::pair<std::string, std::string>>());
        //void create_block_blob(const std::string &container, const std::string blob, const std::vector<std::pair<std::string, std::string>> &metadata = std::vector<std::pair<std::string, std::string>>());
        void upload_file_to_blob(const std::string &sourcePath, const std::string &container, const std::string blob, const std::vector<std::pair<std::string, std::string>> &metadata = std::vector<std::pair<std::string, std::string>>(), size_t parallel = 8);

        // void download_blob_range_to_stream(const std::string &sourcePath, const std::string &container, const std::string blob);
        void download_blob_to_stream(const std::string &container, const std::string &blob, unsigned long long offset, unsigned long long size, std::ostream &os);
        void download_blob_to_file(const std::string &container, const std::string &blob, const std::string &destPath, size_t parallel = 9);

        // blob_property can be correctly returned now.
        blob_property get_blob_property(const std::string &container, const std::string &blob);
        bool blob_exists(const std::string &container, const std::string &blob);

        void delete_blob(const std::string &container, const std::string &blob);

        void start_copy(const std::string &sourceContainer, const std::string &sourceBlob, const std::string &destContainer, const std::string &destBlob);
    private:
        blob_client_wrapper() {}

        std::shared_ptr<blob_client> m_blobClient;
        std::mutex s_mutex;
        unsigned int m_concurrency;
        bool m_invalid;
};

}
}
