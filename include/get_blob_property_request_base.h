#pragma once

#include <string>

#include "storage_EXPORTS.h"

#include "http_base.h"
#include "storage_account.h"
#include "storage_request_base.h"
#include "get_blob_request_base.h"

namespace azure {  namespace storage_lite {

    class get_blob_property_request_base : public blob_request_base
    {
    public:
        virtual std::string container() const = 0;
        virtual std::string blob() const = 0;

        virtual std::string snapshot() const { return std::string(); }

        AZURE_STORAGE_API void build_request(const storage_account &a, http_base &h) const override;
    };

    class blob_property
    {
    public:
        blob_property() : m_valid(false) {}

        blob_property(bool valid)
            :last_modified{time(NULL)},
            m_valid(valid)
        {
        }

        void set_valid(bool valid)
        {
            m_valid = valid;
        }

        bool valid()
        {
            return m_valid;
        }

        std::string cache_control;
        std::string content_disposition;
        std::string content_encoding;
        std::string content_language;
        unsigned long long size;
        std::string content_md5;
        std::string content_type;
        std::string etag;
        std::vector<std::pair<std::string, std::string>> metadata;
        std::string copy_status;
        time_t last_modified;
        // TODO: support lease and blob_type
        // blob_type m_type;
        // azure::storage::lease_status m_lease_status;
        // azure::storage::lease_state m_lease_state;
        // azure::storage::lease_duration m_lease_duration;

    private:
        bool m_valid;
    };
}}  // azure::storage_lite
