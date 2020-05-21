#pragma once

#include "get_block_list_request_base.h"

namespace azure {  namespace storage_lite {

    class get_block_list_request final : public get_block_list_request_base
    {
    public:
        get_block_list_request(const std::string &container, const std::string &blob)
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
