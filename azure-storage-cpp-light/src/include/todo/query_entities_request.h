#pragma once

#include <string>
#include <limits>
#include <map>

#include "defaults.h"
#include "process_storage_request.h"
#include "sign_storage_request.h"
#include "storage_url.h"
#include "utility.h"
#include "constants.h"

namespace microsoft_azure {
    namespace storage {
        namespace experimental {

            struct query_entities_request_base : table_request_base {
                std::string account() = delete;
                std::vector<unsigned char> key() = delete;
                std::string table() = delete;

                std::string sas_token() { return defaults::sas_token; };
                std::string custom_endpoint() { return defaults::custom_endpoint; }
                bool https() { return defaults::https; }
                std::string endpoint_suffix() { return defaults::endpoint_suffix; }

                std::string partition_key() { return defaults::partition_key; }
                std::string row_key() { return defaults::row_key; }

                azure::storage::experimental::payload_format payload_format() { return azure::storage::experimental::payload_format::json_nometadata; }
                std::string ms_client_request_id() { return defaults::ms_client_request_id; }
            };

            template<typename Request, typename Http>
            struct storage_request_processor<Request, Http, typename std::enable_if<std::is_base_of<query_entities_request_base, typename std::remove_reference<Request>::type>::value>::type> : storage_request_processor_base {
                static inline void process(Request &&r, Http &&h) {
                    auto method = http_method::get;
                    h.set_method(method);

                    storage_url url{ get_domain(r, constants::table_prefix) };
                    std::string table = r.table();
                    if (r.partition_key() != defaults::partition_key && r.row_key() != defaults::row_key) {
                        table.append("(PartitionKey='").append(r.partition_key()).append("',RowKey='").append(r.row_key()).append("')");
                    }
                    else {
                        table.append("()");
                    }
                    url.append_path(table);

                    std::string transform_url = url.to_string();
                    if (r.sas_token() != defaults::sas_token) {
                        transform_url.append("?").append(constants::query_api_version).append("=").append(constants::header_value_storage_version);
                        transform_url.append("&").append(r.sas_token());
                    }
                    h.set_url(transform_url);

                    h.add_header(constants::header_user_agent, defaults::user_agent);

                    std::map<std::string, std::string> ms_headers;
                    if (r.ms_client_request_id() != defaults::ms_client_request_id) {
                        h.add_header(constants::header_ms_client_request_id, r.ms_client_request_id());
                        ms_headers[constants::header_ms_client_request_id] = r.ms_client_request_id();
                    }

                    auto ms_date = get_ms_date(date_format::rfc_1123);
                    h.add_header(constants::header_ms_date, ms_date);
                    ms_headers[constants::header_ms_date] = std::move(ms_date);

                    h.add_header(constants::header_ms_version, constants::header_value_storage_version);
                    ms_headers[constants::header_ms_version] = constants::header_value_storage_version;

                    if (r.payload_format() == payload_format::json_nometadata) {
                        h.add_header(constants::header_accept, constants::header_payload_format_nometadata);
                    }
                    else {
                        h.add_header(constants::header_accept, constants::header_payload_format_fullmetadata);
                    }

                    if (r.sas_token() == defaults::sas_token) {
                        ;//h.add_header(constants::header_authorization, sign_storage_request(r, get_method(method), std::move(url), std::move(headers_to_sign), std::move(ms_headers), std::move(sh)));
                    }
                }
            };
        }
    }
}
