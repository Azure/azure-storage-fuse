#pragma once

#include <map>
#include <string>

#include "storage_EXPORTS.h"

#include "http_base.h"
#include "storage_account.h"
#include "storage_request_base.h"

namespace azure {  namespace storage_lite {

    class put_page_request_base : public blob_request_base
    {
    public:
        enum class page_write
        {
            update,
            clear
        };

        virtual std::string container() const = 0;
        virtual std::string blob() const = 0;

        virtual unsigned long long start_byte() const { return 0; }
        virtual unsigned long long end_byte() const { return 0; }
        virtual page_write ms_page_write() const = 0;
        virtual std::string ms_if_sequence_number_le() const { return std::string(); }
        virtual std::string ms_if_sequence_number_lt() const { return std::string(); }
        virtual std::string ms_if_sequence_number_eq() const { return std::string(); }

        virtual unsigned int content_length() const = 0;
        virtual std::string content_md5() const { return std::string(); }

        AZURE_STORAGE_API void build_request(const storage_account &a, http_base &h) const override;
    };

}}  // azure::storage_lite
