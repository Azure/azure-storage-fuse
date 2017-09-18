#pragma once

#include <string>

#include "storage_EXPORTS.h"

#include "common.h"
#include "constants.h"
#include "http_base.h"
#include "storage_request_base.h"
#include "storage_url.h"

namespace microsoft_azure {
namespace storage {

enum class date_format {
    rfc_1123,
    iso_8601
};

AZURE_STORAGE_API std::string get_ms_date(date_format format);

AZURE_STORAGE_API std::string get_ms_range(unsigned long long start_byte, unsigned long long end_byte);

AZURE_STORAGE_API std::string get_http_verb(http_base::http_method method);

inline void add_optional_query(storage_url &url, const std::string &name, unsigned int value) {
    if (value > 0) {
        url.add_query(name, std::to_string(value));
    }
}

inline void add_optional_query(storage_url &url, const std::string &name, const std::string &value) {
    if (!value.empty()) {
        url.add_query(name, value);
    }
}

AZURE_STORAGE_API void add_access_condition_headers(http_base &h, storage_headers &headers, const blob_request_base &r);

inline void add_optional_header(http_base &h, const std::string &name, const std::string &value) {
    if (!value.empty()) {
        h.add_header(name, value);
    }
}

inline void add_optional_content_encoding(http_base &h, storage_headers &headers, const std::string &value) {
    if (!value.empty()) {
        h.add_header(constants::header_content_encoding, value);
        headers.content_encoding = value;
    }
}

inline void add_optional_content_language(http_base &h, storage_headers &headers, const std::string &value) {
    if (!value.empty()) {
        h.add_header(constants::header_content_language, value);
        headers.content_language = value;
    }
}

inline void add_content_length(http_base &h, storage_headers &headers, unsigned int length) {
    std::string value = std::to_string(length);
    h.add_header(constants::header_content_length, value);
    if (length > 0) {
        headers.content_length = value;
    }
}

inline void add_optional_content_md5(http_base &h, storage_headers &headers, const std::string &value) {
    if (!value.empty()) {
        h.add_header(constants::header_content_md5, value);
        headers.content_md5 = value;
    }
}

inline void add_optional_content_type(http_base &h, storage_headers &headers, const std::string &value) {
    if (!value.empty()) {
        h.add_header(constants::header_content_type, value);
        headers.content_type = value;
    }
}

inline void add_ms_header(http_base &h, storage_headers &headers, const std::string &name, const std::string &value, bool optional = false) {
    if (!optional || !value.empty()) {
        h.add_header(name, value);
        headers.ms_headers[name] = value;
    }
}

inline void add_ms_header(http_base &h, storage_headers &headers, const std::string &name, unsigned long long value, bool optional = false) {
    if (!optional || !value) {
        h.add_header(name, std::to_string(value));
        headers.ms_headers[name] = std::to_string(value);
    }
}

inline void add_metadata_header(http_base &h, storage_headers &headers, const std::string &name, const std::string &value, bool optional = false) {
    add_ms_header(h, headers, constants::header_ms_meta_prefix + name, value, optional);
}

AZURE_STORAGE_API bool retryable(http_base::http_code status_code);

inline bool unsuccessful(http_base::http_code status_code) {
    return !(status_code >= 200 && status_code < 300);
}

inline lease_status parse_lease_status(const std::string &value) {
    if (value == "locked") {
        return lease_status::locked;
    }
    else if (value == "unlocked") {
        return lease_status::unlocked;
    }
    return lease_status::unlocked;
}

inline lease_state parse_lease_state(const std::string &value) {
    if (value == "available") {
        return lease_state::available;
    }
    else if (value == "leased") {
        return lease_state::leased;
    }
    else if (value == "expired") {
        return lease_state::expired;
    }
    else if (value == "breaking") {
        return lease_state::breaking;
    }
    else if (value == "broken") {
        return lease_state::broken;
    }
    return lease_state::available;
}

inline lease_duration parse_lease_duration(const std::string &value) {
    if (value == "infinite") {
        return lease_duration::infinite;
    }
    else if (value == "fixed") {
        return lease_duration::fixed;
    }
    return lease_duration::none;
}

}
}
