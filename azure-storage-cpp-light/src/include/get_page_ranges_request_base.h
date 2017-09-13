#pragma once

#include <string>

#include "storage_EXPORTS.h"

#include "http_base.h"
#include "storage_account.h"
#include "storage_request_base.h"

namespace microsoft_azure {
    namespace storage {

        class get_page_ranges_request_base : public blob_request_base {
        public:
            virtual std::string container() const = 0;
            virtual std::string blob() const = 0;

            virtual unsigned long long start_byte() const { return 0; }
            virtual unsigned long long end_byte() const { return 0; }

            virtual std::string snapshot() const { return std::string(); }

            AZURE_STORAGE_API void build_request(const storage_account &a, http_base &h) const override;
        };

        //AZURE_STORAGE_API void build_request(const storage_account &a, const get_blob_request_base &r, http_base &h);

        class get_page_ranges_item {
        public:
            unsigned long long start;
            unsigned long long end;
        };

        class get_page_ranges_response {
        public:
            std::vector<get_page_ranges_item> pagelist;
        };

    }
}
