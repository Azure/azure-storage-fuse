#pragma once

#include "copy_blob_request_base.h"

namespace azure { namespace storage_lite {

    class copy_blob_request final : public copy_blob_request_base 
    {
    public:
        copy_blob_request(const std::string &container, const std::string &blob, const std::string &destContainer, const std::string &destBlob)
            : m_container(container),
            m_blob(blob),
            m_destContainer(destContainer),
            m_destBlob(destBlob) {}

        std::string container() const override
        {
            return m_container;
        }

        std::string blob() const override
        {
            return m_blob;
        }

        std::string destContainer() const override
        {
            return m_destContainer;
        }

        std::string destBlob() const override
        {
            return m_destBlob;
        }

    private:
        std::string m_container;
        std::string m_blob;
        std::string m_destContainer;
        std::string m_destBlob;
    };

}}  // azure::storage_lite
