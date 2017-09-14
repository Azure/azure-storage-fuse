#pragma once

#include <string>
#include <limits>
#include <map>

#include "storage_request.h"
#include "process_storage_request.h"
#include "sign_storage_request.h"
#include "storage_url.h"
#include "utility.h"
#include "constants.h"
#include "common.h"

namespace microsoft_azure {
    namespace storage {
        namespace experimental {

            struct get_blob_metadata_request_base : blob_request_base {
                std::string date() { return std::string(); }
                std::string range() { return std::string(); }
                std::string content_encoding() { return std::string(); }
                std::string content_language() { return std::string(); }
                unsigned long long content_length() { return 0; }
                std::string content_md5() { return std::string(); }
                std::string content_type() { return std::string(); }
                std::string if_modified_since() { return std::string(); }
                std::string if_match() { return std::string(); }
                std::string if_none_match() { return std::string(); }
                std::string if_unmodified_since() { return std::string(); }
            };

            template<typename Request, typename Http>
            struct storage_request_processor<Request, Http, std::enable_if_t<std::is_base_of<get_blob_metadata_request, std::remove_reference_t<Request>>::value>> {
                static inline void process(Request &&r, Http &&h) {
                    //static_assert(std::is_same<decltype(r.use_https()), decltype(r.get_blob_request::use_https())>::value, "get_blob_request::use_https != Request::use_https");
                    std::string method = constant::http_head;
                    h.set_method(http_method::head);

                    storage_url url;
                    url.use_https = r.use_https();
                    url.use_custom_endpoint = r.use_custom_endpoint();
                    url.domain = r.use_custom_endpoint() ? r.custom_endpoint() : r.account() + r.endpoint_suffix();
                    url.append_path(r.container()).append_path(r.blob());
                    url.add_optional_query(constant::query_comp, constant::query_comp_metadata);
                    url.add_optional_query(constant::query_snapshot, r.snapshot());
                    url.add_optional_query(constant::query_timeout, (r.timeout() != std::numeric_limits<unsigned long long>::max() ? std::to_string(r.timeout()) : std::string()));
                    h.set_url(url.to_string());

                    details::add_optional_header(h, constant::header_content_encoding, r.content_encoding());
                    details::add_optional_header(h, constant::header_content_language, r.content_language());
                    details::add_optional_header(h, constant::header_content_length, (r.content_length() != 0 ? std::to_string(r.content_length()) : std::string()));
                    details::add_optional_header(h, constant::header_content_md5, r.content_md5());
                    details::add_optional_header(h, constant::header_content_type, r.content_type());
                    details::add_optional_header(h, constant::header_date, r.date());
                    details::add_optional_header(h, constant::header_if_modified_since, r.if_modified_since());
                    details::add_optional_header(h, constant::header_if_match, r.if_match());
                    details::add_optional_header(h, constant::header_if_none_match, r.if_none_match());
                    details::add_optional_header(h, constant::header_if_unmodified_since, r.if_unmodified_since());
                    details::add_optional_header(h, constant::header_range, r.range());

                    std::map<std::string, std::string> ms_headers;
                    if (!r.lease_id().empty()) {
                        details::add_ms_header(h, ms_headers, constant::header_ms_lease_id, r.lease_id());
                    }
                    if (!r.client_request_id().empty()) {
                        details::add_ms_header(h, ms_headers, constant::header_ms_client_request_id, r.client_request_id());
                    }
                    details::add_ms_header(h, ms_headers, constant::header_ms_date, details::get_date());
                    details::add_ms_header(h, ms_headers, constant::header_ms_version, constant::header_value_storage_version);

                    h.add_header(constant::header_authorization, sign_storage_request(r, method, url, ms_headers));
                }
            };
        }
    }
}
