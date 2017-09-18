#pragma once

#include <map>
#include <string>

#include "storage_EXPORTS.h"

#include "http_base.h"
#include "storage_account.h"
#include "storage_request_base.h"

namespace microsoft_azure {
    namespace storage {

        class create_container_request_base : public blob_request_base {
        public:
            enum class blob_public_access {
                unspecified,
                container,
                blob
            };

            virtual std::string container() const = 0;

            virtual blob_public_access ms_blob_public_access() const {
                return blob_public_access::unspecified;
            }

            //virtual std::map<std::string, std::string> ms_meta() const {};

            AZURE_STORAGE_API void build_request(const storage_account &a, http_base &h) const override;
        };

        //AZURE_STORAGE_API void build_request(const storage_account &a, const create_container_request_base &r, http_base &h);

    }
}

