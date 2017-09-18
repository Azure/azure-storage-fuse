#include "storage_url.h"

namespace microsoft_azure {
    namespace storage {

        std::string storage_url::to_string() const {
            std::string url(m_domain);
            url.append(m_path);

            bool first_query = true;
            for (const auto &q : m_query) {
                if (first_query) {
                    url.append("?");
                    first_query = false;
                }
                else {
                    url.append("&");
                }
                for (const auto &value : q.second) {
                    url.append(q.first).append("=").append(value);
                }
            }
            return url;
        }

    }
}
