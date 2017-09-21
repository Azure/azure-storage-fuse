#pragma once

#include <string>

#include "storage_EXPORTS.h"

#include "http_base.h"
#include "storage_account.h"
#include "storage_request_base.h"

namespace microsoft_azure {
    namespace storage {

        class delete_blob_request_base : public blob_request_base {
        public:
            enum class delete_snapshots {
                unspecified,
                include,
                only
            };

            virtual std::string container() const = 0;
            virtual std::string blob() const = 0;

            virtual std::string snapshot() const { return std::string(); }
            virtual delete_snapshots ms_delete_snapshots() const { return delete_snapshots::unspecified; }

            AZURE_STORAGE_API void build_request(const storage_account &a, http_base &h) const override;
        };

        //AZURE_STORAGE_API void build_request(const storage_account &a, const delete_blob_request_base &r, http_base &h);

    }
}