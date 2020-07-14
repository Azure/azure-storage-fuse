#pragma once

#include "set_blob_metadata_request_base.h"

namespace azure {  namespace storage_lite {

    class set_blob_metadata_request final : public set_blob_metadata_request_base
    {
    public:
        set_blob_metadata_request(const std::string &container, const std::string& blob, const std::vector<std::pair<std::string, std::string>>& metadata)
            : m_container(container), m_blob(blob), m_metadata(metadata) {}

        std::string container() const override
        {
            return m_container;
        }

        std::string blob() const override
        {
            return m_blob;
        }

        std::vector<std::pair<std::string, std::string>> metadata() const override
        {
            return m_metadata;
        }
    private:
        std::string m_container;
        std::string m_blob;
        std::vector<std::pair<std::string, std::string>> m_metadata;
    };

}}  // azure::storage_lite
