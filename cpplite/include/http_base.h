#pragma once

#include <chrono>
#include <functional>
#include <string>
#include <map>
#include <curl/curl.h>

#include "storage_stream.h"
#include "compare.h"

namespace azure {  namespace storage_lite {

    class http_base
    {
    public:
        enum class http_method
        {
            del,
            get,
            head,
            post,
            put,
            patch
        };

        virtual ~http_base() {}

        using http_code = int;

        virtual void set_method(http_method method) = 0;

        virtual http_method get_method() const = 0;

        virtual void set_url(const std::string &url) = 0;

        virtual std::string get_url() const = 0;

        virtual void add_header(const std::string &name, const std::string &value) = 0;

        virtual const std::map<std::string, std::string, case_insensitive_compare>& get_request_headers() const = 0;

        virtual std::string get_response_header(const std::string &name) const = 0;
        virtual const std::map<std::string, std::string, case_insensitive_compare>& get_response_headers() const = 0;

        virtual CURLcode perform() = 0;

        virtual void submit(std::function<void(http_code, storage_istream, CURLcode)> cb, std::chrono::seconds interval) = 0;

        virtual void reset() = 0;

        virtual http_code status_code() const = 0;

        virtual void set_input_stream(storage_istream s) = 0;

        virtual void reset_input_stream() = 0;

        virtual void reset_output_stream() = 0;

        virtual void set_output_stream(storage_ostream s) = 0;

        virtual void set_error_stream(std::function<bool(http_code)> f, storage_iostream s) = 0;

        virtual storage_istream get_input_stream() const = 0;

        virtual storage_ostream get_output_stream() const = 0;

        virtual storage_iostream get_error_stream() const = 0;

        virtual void set_absolute_timeout(long long timeout) = 0;

        virtual void set_data_rate_timeout() = 0;
    };

}}  // azure::storage_lite
