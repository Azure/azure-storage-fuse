#include "storage_url.h"
#include "utility.h"

namespace azure {  namespace storage_lite {

    std::string storage_url::get_encoded_path() const
    {
        return encode_url_path(m_path);
    }

    std::string storage_url::to_string() const
    {
        std::string url(m_domain);
        url.append(encode_url_path(m_path));

        bool first_query = true;
        for (const auto &q : m_query)
        {
            if (first_query)
            {
                url.append("?");
                first_query = false;
            }
            else
            {
                url.append("&");
            }
            for (const auto &value : q.second)
            {
                url.append(encode_url_query(q.first)).append("=").append(encode_url_query(value));
            }
        }
        return url;
    }

}}   // azure::storage_lite
