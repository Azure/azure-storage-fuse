#pragma once

#include "put_page_request_base.h"

namespace azure {  namespace storage_lite {

    class put_page_request final : public put_page_request_base
    {
    public:
        put_page_request(const std::string &container, const std::string &blob, bool clear = false)
            : m_container(container),
            m_blob(blob),
            m_clear(clear),
            m_start_byte(0),
            m_end_byte(0),
            m_content_length(0) {}

        std::string container() const override
        {
            return m_container;
        }

        std::string blob() const override
        {
            return m_blob;
        }

        unsigned long long start_byte() const override
        {
            return m_start_byte;
        }

        unsigned long long end_byte() const override
        {
            return m_end_byte;
        }

        put_page_request &set_start_byte(unsigned long long start_byte)
        {
            m_start_byte = start_byte;
            return *this;
        }

        put_page_request &set_end_byte(unsigned long long end_byte)
        {
            m_end_byte = end_byte;
            return *this;
        }

        page_write ms_page_write() const override
        {
            if (m_clear) {
                return page_write::clear;
            }
            return page_write::update;
        }

        unsigned int content_length() const override
        {
            return m_content_length;
        }

        put_page_request &set_content_length(unsigned int content_length)
        {
            m_content_length = content_length;
            return *this;
        }

    private:
        std::string m_container;
        std::string m_blob;
        bool m_clear;
        unsigned long long m_start_byte;
        unsigned long long m_end_byte;
        unsigned int m_content_length;
    };
}}  // azure::storage_lite
