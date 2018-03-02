#pragma once

#include <string>

#include "storage_EXPORTS.h"

#include "http_base.h"
#include "storage_account.h"
#include "storage_request_base.h"

namespace microsoft_azure {
    namespace storage {

        class get_blob_request_base : public blob_request_base {
        public:
            virtual std::string container() const = 0;
            virtual std::string blob() const = 0;

            virtual std::string snapshot() const { return std::string(); }
            virtual unsigned long long start_byte() const { return 0; }
            virtual unsigned long long end_byte() const { return 0; }
            virtual std::string origin() const { return std::string(); }
            virtual bool ms_range_get_content_md5() const { return false; }

            AZURE_STORAGE_API void build_request(const storage_account &a, http_base &h) const override;
        };

        //AZURE_STORAGE_API void build_request(const storage_account &a, const get_blob_request_base &r, http_base &h);

        class chunk_property
        {
        public:
            chunk_property()
               :totalSize{0},
               size{0},
               last_modified{0} //returns 1970
            {
            }
            long long totalSize;
            unsigned long long size;
            time_t last_modified;
            std::string etag;
        };
    }
}
