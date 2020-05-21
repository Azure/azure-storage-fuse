#pragma once

#include "put_block_request_base.h"

namespace azure {  namespace storage_lite {

    class put_block_request final : public put_block_request_base
    {
    public:
        put_block_request(const std::string &container, const std::string &blob, const std::string &blockid)
            : m_container(container),
            m_blob(blob),
            m_blockid(blockid),
            m_content_length(0) {}

        std::string container() const override
        {
            return m_container;
        }

        std::string blob() const override
        {
            return m_blob;
        }

        std::string blockid() const override
        {
            return m_blockid;
        }

        unsigned int content_length() const override
        {
            return m_content_length;
        }

        put_block_request &set_content_length(unsigned int content_length)
        {
            m_content_length = content_length;
            return *this;
        }

    private:
        std::string m_container;
        std::string m_blob;
        std::string m_blockid;

        unsigned int m_content_length;
    };

}}  // azure::storage_lite
