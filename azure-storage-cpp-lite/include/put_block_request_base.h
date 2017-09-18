#pragma once

#include <map>
#include <string>

#include "storage_EXPORTS.h"

#include "http_base.h"
#include "storage_account.h"
#include "storage_request_base.h"

namespace microsoft_azure {
    namespace storage {

        class put_block_request_base : public blob_request_base {
        public:
            virtual std::string container() const = 0;
            virtual std::string blob() const = 0;
            virtual std::string blockid() const = 0;

            virtual unsigned int content_length() const = 0;
            virtual std::string content_md5() const { return std::string(); }

            AZURE_STORAGE_API void build_request(const storage_account &a, http_base &h) const override;
        };

    }
}
