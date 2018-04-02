#include "put_page_request_base.h"

#include "constants.h"
#include "utility.h"

namespace microsoft_azure {
    namespace storage {

        void put_page_request_base::build_request(const storage_account &a, http_base &h) const {
            const auto &r = *this;

            h.set_data_rate_timeout();

            h.set_method(http_base::http_method::put);

            storage_url url = a.get_url(storage_account::service::blob);
            url.append_path(r.container()).append_path(r.blob());

            url.add_query(constants::query_comp, constants::query_comp_page);
            add_optional_query(url, constants::query_timeout, r.timeout());
            h.set_url(url.to_string());

            storage_headers headers;
            add_content_length(h, headers, r.content_length());
            add_optional_content_md5(h, headers, r.content_md5());
            add_access_condition_headers(h, headers, r);

            add_ms_header(h, headers, constants::header_ms_range, get_ms_range(r.start_byte(), r.end_byte()), true);

            switch (r.ms_page_write()) {
            case put_page_request_base::page_write::update:
                add_ms_header(h, headers, constants::header_ms_page_write, constants::header_value_page_write_update);
                break;
            case put_page_request_base::page_write::clear:
                add_ms_header(h, headers, constants::header_ms_page_write, constants::header_value_page_write_clear);
                break;
            }

            add_ms_header(h, headers, constants::header_ms_if_sequence_number_lt, r.ms_if_sequence_number_lt(), true);
            add_ms_header(h, headers, constants::header_ms_if_sequence_number_le, r.ms_if_sequence_number_le(), true);
            add_ms_header(h, headers, constants::header_ms_if_sequence_number_eq, r.ms_if_sequence_number_eq(), true);

            add_ms_header(h, headers, constants::header_ms_client_request_id, r.ms_client_request_id(), true);
            add_ms_header(h, headers, constants::header_ms_lease_id, r.ms_lease_id(), true);

            h.add_header(constants::header_user_agent, constants::header_value_user_agent);
            add_ms_header(h, headers, constants::header_ms_date, get_ms_date(date_format::rfc_1123));
            add_ms_header(h, headers, constants::header_ms_version, constants::header_value_storage_version);

            a.credential()->sign_request(r, h, url, headers);
        }

    }
}
