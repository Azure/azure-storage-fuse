#pragma once

#include "create_container_request_base.h"

namespace azure { namespace storage_lite {

    class create_container_request final : public create_container_request_base
    {
    public:
        create_container_request(const std::string &container, blob_public_access public_access = blob_public_access::unspecified)
            : m_container(container),
            m_blob_public_access(public_access) {}

        std::string container() const override
        {
            return m_container;
        }

        blob_public_access ms_blob_public_access() const override
        {
            return m_blob_public_access;
        }

    private:
        std::string m_container;
        blob_public_access m_blob_public_access;
    };

}}  // azure::storage_lite
