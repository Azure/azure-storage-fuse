#pragma once

#include <map>
#include <set>
#include <string>

#include "storage_EXPORTS.h"

namespace azure {  namespace storage_lite {

    class storage_url
    {
    public:
        storage_url & set_domain(const std::string &domain)
        {
            m_domain = domain;
            return *this;
        }

        const std::string &get_domain() const
        {
            return m_domain;
        }

        storage_url &append_path(const std::string &segment)
        {
            m_path.append("/").append(segment);
            return *this;
        }

        const std::string &get_path() const
        {
            return m_path;
        }

        AZURE_STORAGE_API std::string get_encoded_path() const;

        storage_url &add_query(const std::string &name, const std::string &value)
        {
            m_query[name].insert(value);
            return *this;
        }

        const std::map<std::string, std::set<std::string>> &get_query() const
        {
            return m_query;
        }

        AZURE_STORAGE_API std::string to_string() const;

    private:
        std::string m_domain;
        std::string m_path;
        std::map<std::string, std::set<std::string>> m_query;
    };

    class storage_headers
    {
    public:
        std::string content_encoding;
        std::string content_language;
        std::string content_length;
        std::string content_md5;
        std::string content_type;
        std::string if_modified_since;
        std::string if_match;
        std::string if_none_match;
        std::string if_unmodified_since;
        std::map<std::string, std::string> ms_headers;
    };

}}  // azure::storage_lite
