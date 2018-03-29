#include "put_block_request_base.h"

#include "constants.h"
#include "utility.h"

namespace microsoft_azure {
    namespace storage {

        void put_block_request_base::build_request(const storage_account &a, http_base &h) const {
            const auto &r = *this;

            h.set_data_rate_timeout();

            h.set_method(http_base::http_method::put);

            storage_url url = a.get_url(storage_account::service::blob);
            url.append_path(r.container()).append_path(r.blob());

            url.add_query(constants::query_comp, constants::query_comp_block);
            url.add_query(constants::query_blockid, r.blockid());
            add_optional_query(url, constants::query_timeout, r.timeout());
            h.set_url(url.to_string());

            storage_headers headers;
            add_content_length(h, headers, r.content_length());
            //add_optional_content_md5(h, headers, r.content_md5());
            // add_access_condition_headers(h, headers, r);

            add_ms_header(h, headers, constants::header_ms_client_request_id, r.ms_client_request_id(), true);
            // add_ms_header(h, headers, constants::header_ms_lease_id, r.ms_lease_id(), true);

            h.add_header(constants::header_user_agent, constants::header_value_user_agent);
            add_ms_header(h, headers, constants::header_ms_date, get_ms_date(date_format::rfc_1123));
            add_ms_header(h, headers, constants::header_ms_version, constants::header_value_storage_version);

            a.credential()->sign_request(r, h, url, headers);
        }

    }
}
