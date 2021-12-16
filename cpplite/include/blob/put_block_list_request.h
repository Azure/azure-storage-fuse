#pragma once

#include "put_block_list_request_base.h"

namespace azure {  namespace storage_lite {

    class put_block_list_request final : public put_block_list_request_base
    {
    public:
        put_block_list_request(const std::string &container, const std::string &blob)
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

        std::vector<block_item> block_list() const override
        {
            return m_block_list;
        }

        put_block_list_request &set_block_list(const std::vector<block_item> &block_list)
        {
            m_block_list = block_list;
            return *this;
        }

        std::vector<std::pair<std::string, std::string>> metadata() const override
        {
            return m_metadata;
        }

        put_block_list_request &set_metadata(const std::vector<std::pair<std::string, std::string>> &metadata)
        {
            m_metadata = metadata;
            return *this;
        }

        std::string content_type() {
            return m_content_type;
        }

        put_block_list_request &set_content_type(const std::string type)
        {
            m_content_type = type;
            return *this;
        }

        std::string ms_blob_content_type() const 
        { 
            return m_content_type; 
        }

    private:
        std::string m_container;
        std::string m_blob;
        std::vector<block_item> m_block_list;
        std::vector<std::pair<std::string, std::string>> m_metadata;
        std::string m_content_type;
    };

}}  // azure::storage_lite
