#pragma once

#include <string>

#include "storage_EXPORTS.h"

#include "http_base.h"
#include "storage_account.h"
#include "storage_request_base.h"

namespace azure {  namespace storage_lite {

    class copy_blob_request_base : public blob_request_base
    {
    public:

        virtual std::string container() const = 0;
        virtual std::string blob() const = 0;

        virtual std::string destContainer() const = 0;
        virtual std::string destBlob() const = 0;

        AZURE_STORAGE_API void build_request(const storage_account &a, http_base &h) const override;
    };

}}  // azure::storage_lite
