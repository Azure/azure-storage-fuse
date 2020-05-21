#pragma once

#include "get_blob_property_request_base.h"

namespace azure {  namespace storage_lite {

    class get_blob_property_request final : public get_blob_property_request_base
    {
    public:
        get_blob_property_request(const std::string &container, const std::string &blob)
            : m_container(container),
            m_blob(blob) {}

        std::string container() const override
        {
            return m_container;
        }

        std::string blob() const override
        {
            return m_blob;
        }

    private:
        std::string m_container;
        std::string m_blob;
    };

}}  // azure::storage_lite
