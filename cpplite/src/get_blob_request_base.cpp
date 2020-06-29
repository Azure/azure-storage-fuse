#include "get_blob_request_base.h"

#include "constants.h"
#include "utility.h"

namespace azure {  namespace storage_lite {

    void get_blob_request_base::build_request(const storage_account &a, http_base &h) const
    {
        const auto &r = *this;

        h.set_data_rate_timeout();

        h.set_method(http_base::http_method::get);

        storage_url url = a.get_url(storage_account::service::blob);
        url.append_path(r.container()).append_path(r.blob());

        add_optional_query(url, constants::query_snapshot, r.snapshot());
        add_optional_query(url, constants::query_timeout, r.timeout());
        h.set_url(url.to_string());

        storage_headers headers;
        add_access_condition_headers(h, headers, r);

        add_optional_header(h, constants::header_origin, r.origin());

        add_ms_header(h, headers, constants::header_ms_client_request_id, r.ms_client_request_id(), true);
        add_ms_header(h, headers, constants::header_ms_lease_id, r.ms_lease_id(), true);
        add_ms_header(h, headers, constants::header_ms_range, get_ms_range(r.start_byte(), r.end_byte()), true);
        if (r.ms_range_get_content_md5())
        {
            // TODO check range
            add_ms_header(h, headers, constants::header_ms_range_get_content_md5, "true");
        }

        h.add_header(constants::header_user_agent, constants::header_value_user_agent);
        add_ms_header(h, headers, constants::header_ms_date, get_ms_date(date_format::rfc_1123));
        add_ms_header(h, headers, constants::header_ms_version, constants::header_value_storage_blob_version);

        a.credential()->sign_request(r, h, url, headers);
    }

}}  // azure::storage_lite
