#pragma once

#include "list_containers_request_base.h"

namespace azure {  namespace storage_lite {

    class list_containers_request final : public list_containers_request_base
    {
    public:
        list_containers_request(const std::string &prefix, bool include_metadata = false)
            : m_prefix(prefix),
            m_include_metadata(include_metadata) {}

        std::string prefix() const override
        {
            return m_prefix;
        }

        std::string marker() const override
        {
            return m_marker;
        }

        int maxresults() const override
        {
            return m_maxresults;
        }

        bool include_metadata() const override
        {
            return m_include_metadata;
        }

        list_containers_request &set_marker(const std::string &marker)
        {
            m_marker = marker;
            return *this;
        }

        list_containers_request &set_maxresults(int maxresults)
        {
            m_maxresults = maxresults;
            return *this;
        }

    private:
        std::string m_prefix;
        std::string m_marker;
        int m_maxresults;
        bool m_include_metadata;
    };

}}  // azure::storage_lite
