#pragma once

#include "get_blob_request_base.h"

namespace azure { namespace storage_lite {

    class download_blob_request final : public get_blob_request_base
    {
    public:
        download_blob_request(const std::string &container, const std::string &blob)
            : m_container(container),
            m_blob(blob),
            m_start_byte(0),
            m_end_byte(0) {}

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

        download_blob_request &set_start_byte(unsigned long long start_byte)
        {
            m_start_byte = start_byte;
            return *this;
        }

        download_blob_request &set_end_byte(unsigned long long end_byte)
        {
            m_end_byte = end_byte;
            return *this;
        }

    private:
        std::string m_container;
        std::string m_blob;
        unsigned long long m_start_byte;
        unsigned long long m_end_byte;
    };
}}  // azure::storage_lite
