#pragma once

#include "set_container_metadata_request_base.h"

namespace azure {  namespace storage_lite {

    class set_container_metadata_request final : public set_container_metadata_request_base
    {
    public:
        set_container_metadata_request(const std::string &container, const std::vector<std::pair<std::string, std::string>>& metadata)
            : m_container(container), m_metadata(metadata) {}

        std::string container() const override
        {
            return m_container;
        }

        std::vector<std::pair<std::string, std::string>> metadata() const override
        {
            return m_metadata;
        }
    private:
        std::string m_container;
        std::vector<std::pair<std::string, std::string>> m_metadata;
    };

}}  // azure::storage_lite
