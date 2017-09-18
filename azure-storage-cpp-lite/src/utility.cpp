#include <ctime>

#include "utility.h"

#include "constants.h"

namespace microsoft_azure {
    namespace storage {

        std::string get_ms_date(date_format format) {
            char buf[30];
            std::time_t t = std::time(nullptr);
            std::tm *pm;
#ifdef WIN32
            std::tm m;
            pm = &m;
            gmtime_s(pm, &t);
#else
            pm = std::gmtime(&t);
#endif
            size_t s = std::strftime(buf, 30, (format == date_format::iso_8601 ? constants::date_format_iso_8601 : constants::date_format_rfc_1123), pm);
            return std::string(buf, s);
        }

        std::string get_ms_range(unsigned long long start_byte, unsigned long long end_byte) {
            std::string result("bytes=");
            result.append(std::to_string(start_byte)).append("-");
            if (end_byte != 0) {
                result.append(std::to_string(end_byte));
            }
            return result;
        }

        std::string get_http_verb(http_base::http_method method) {
            switch (method) {
            case http_base::http_method::del:
                return constants::http_delete;
            case http_base::http_method::get:
                return constants::http_get;
            case http_base::http_method::head:
                return constants::http_head;
            case http_base::http_method::post:
                return constants::http_post;
            case http_base::http_method::put:
                return constants::http_put;
            }
            return std::string();
        }

        void add_access_condition_headers(http_base &h, storage_headers &headers, const blob_request_base &r) {
            if (!r.if_modified_since().empty()) {
                h.add_header(constants::header_if_modified_since, r.if_modified_since());
                headers.if_modified_since = r.if_modified_since();
            }
            if (!r.if_match().empty()) {
                h.add_header(constants::header_if_match, r.if_match());
                headers.if_match = r.if_match();
            }
            if (!r.if_none_match().empty()) {
                h.add_header(constants::header_if_none_match, r.if_none_match());
                headers.if_none_match = r.if_none_match();
            }
            if (!r.if_unmodified_since().empty()) {
                h.add_header(constants::header_if_unmodified_since, r.if_unmodified_since());
                headers.if_unmodified_since = r.if_unmodified_since();
            }
        }

        bool retryable(http_base::http_code status_code) {
            if (status_code == 408 /*Request Timeout*/) {
                return true;
            }
            if (status_code >= 300 && status_code < 500) {
                return false;
            }
            if (status_code == 501 /*Not Implemented*/ || status_code == 505 /*HTTP Version Not Supported*/) {
                return false;
            }
            return true;
        }

    }
}
