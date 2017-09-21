#pragma once

#include <string>

#include "storage_EXPORTS.h"

#include "http_base.h"
#include "storage_account.h"
#include "storage_request_base.h"

namespace microsoft_azure {
    namespace storage {

        class get_container_property_request_base : public blob_request_base {
        public:
            virtual std::string container() const = 0;

            AZURE_STORAGE_API void build_request(const storage_account &a, http_base &h) const override;
        };

        //AZURE_STORAGE_API void build_request(const storage_account &a, const get_blob_request_base &r, http_base &h);

        class container_property
        {
        public:
            container_property(bool valid)
                :m_valid(valid)
            {
            }

            void set_valid(bool valid)
            {
                m_valid = valid;
            }

            bool valid()
            {
                return m_valid;
            }

            std::string etag;
            std::vector<std::pair<std::string, std::string>> metadata;

        private:
            container_property() {}
            bool m_valid;
        };
    }
}
