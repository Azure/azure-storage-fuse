#pragma once

#include <memory>
#include <string>

#include "storage_EXPORTS.h"

#include "storage_credential.h"
#include "storage_url.h"

namespace azure {  namespace storage_lite {

    class storage_account final
    {
    public:
        enum class service
        {
            blob,
            table,
            queue,
            file,
            adls
        };

        static std::shared_ptr<storage_account> development_storage_account();

        AZURE_STORAGE_API storage_account(const std::string &account_name, std::shared_ptr<storage_credential> credential, bool use_https = true, const std::string &blob_endpoint = std::string());

        std::shared_ptr<storage_credential> credential() const
        {
            return m_credential;
        }

        storage_url get_url(service service) const
        {
            switch (service)
            {
            case storage_account::service::blob:
                return m_blob_url;
            case storage_account::service::table:
                return m_table_url;
            case storage_account::service::queue:
                return m_queue_url;
            case storage_account::service::file:
                return m_file_url;
            case storage_account::service::adls:
                return m_adls_url;
            default:
                return storage_url();
            }
        }

    private:
        std::shared_ptr<storage_credential> m_credential;
        storage_url m_blob_url;
        storage_url m_table_url;
        storage_url m_queue_url;
        storage_url m_file_url;
        storage_url m_adls_url;
    };

}}  // azure::storage_lite
