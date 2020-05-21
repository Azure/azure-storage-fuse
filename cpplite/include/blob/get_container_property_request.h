#pragma once

#include "get_container_property_request_base.h"

namespace azure {  namespace storage_lite {

    class get_container_property_request final : public get_container_property_request_base
    {
    public:
        get_container_property_request(const std::string &container)
            : m_container(container)
        {}

        std::string container() const override
        {
            return m_container;
        }

    private:
        std::string m_container;
    };

}}  // azure::storage_lite
