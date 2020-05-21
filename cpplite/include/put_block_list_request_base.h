#pragma once

#include <string>
#include <vector>

#include "storage_EXPORTS.h"

#include "http_base.h"
#include "storage_account.h"
#include "storage_request_base.h"

namespace azure {  namespace storage_lite {

    class put_block_list_request_base : public blob_request_base
    {
    public:
        enum class block_type {
            committed,
            uncommitted,
            latest
        };

        struct block_item {
            std::string id;
            block_type type;
        };

        virtual std::string container() const = 0;
        virtual std::string blob() const = 0;

        virtual std::vector<block_item> block_list() const = 0;
        virtual std::vector<std::pair<std::string, std::string>> metadata() const = 0;

        virtual std::string content_md5() const { return std::string(); }

        virtual std::string ms_blob_cache_control() const { return std::string(); }
        virtual std::string ms_blob_content_disposition() const { return std::string(); }
        virtual std::string ms_blob_content_encoding() const { return std::string(); }
        virtual std::string ms_blob_content_language() const { return std::string(); }
        virtual std::string ms_blob_content_md5() const { return std::string(); }
        virtual std::string ms_blob_content_type() const { return std::string(); }

        AZURE_STORAGE_API void build_request(const storage_account &a, http_base &h) const override;
    };

}}  // azure::storage_lite
