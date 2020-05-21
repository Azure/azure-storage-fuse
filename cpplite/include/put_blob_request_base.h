#pragma once

#include <map>
#include <string>

#include "storage_EXPORTS.h"

#include "http_base.h"
#include "storage_account.h"
#include "storage_request_base.h"

namespace azure {  namespace storage_lite {

    class put_blob_request_base : public blob_request_base
    {
    public:
        enum class blob_type
        {
            block_blob,
            page_blob,
            append_blob
        };

        virtual std::string container() const = 0;
        virtual std::string blob() const = 0;
        virtual std::vector<std::pair<std::string, std::string>> metadata() const = 0;

        virtual std::string content_encoding() const { return std::string(); }
        virtual std::string content_language() const { return std::string(); }
        virtual unsigned int content_length() const = 0;
        virtual std::string content_md5() const { return std::string(); }
        virtual std::string content_type() const { return std::string(); }

        virtual std::string origin() const { return std::string(); }
        virtual std::string cache_control() const { return std::string(); }

        virtual std::string ms_blob_cache_control() const { return std::string(); }
        virtual std::string ms_blob_content_disposition() const { return std::string(); }
        virtual std::string ms_blob_content_encoding() const { return std::string(); }
        virtual std::string ms_blob_content_language() const { return std::string(); }
        virtual unsigned long long ms_blob_content_length() const { return 0; }
        virtual std::string ms_blob_content_md5() const { return std::string(); }
        virtual std::string ms_blob_content_type() const { return std::string(); }
        virtual unsigned long long ms_blob_sequence_number() const { return 0; }
        virtual blob_type ms_blob_type() const = 0;

        AZURE_STORAGE_API void build_request(const storage_account &a, http_base &h) const override;
    };

}}
