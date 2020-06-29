#include "set_container_metadata_request_base.h"

#include "constants.h"
#include "utility.h"

namespace azure {  namespace storage_lite {

    void set_container_metadata_request_base::build_request(const storage_account &a, http_base &h) const
    {
        const auto &r = *this;

        h.set_absolute_timeout(5L);

        h.set_method(http_base::http_method::put);

        storage_url url = a.get_url(storage_account::service::blob);
        url.append_path(r.container());

        url.add_query(constants::query_restype, constants::query_restype_container);
        url.add_query(constants::query_comp, constants::query_comp_metadata);
        add_optional_query(url, constants::query_timeout, r.timeout());
        h.set_url(url.to_string());

        storage_headers headers;

        add_content_length(h, headers, 0);
        add_ms_header(h, headers, constants::header_ms_client_request_id, r.ms_client_request_id(), true);
        // lease is not supported.
        // add_ms_header(h, headers, constants::header_ms_lease_id, r.ms_lease_id(), true);

        h.add_header(constants::header_user_agent, constants::header_value_user_agent);
        add_ms_header(h, headers, constants::header_ms_date, get_ms_date(date_format::rfc_1123));
        add_ms_header(h, headers, constants::header_ms_version, constants::header_value_storage_blob_version);

        for (const auto& m : metadata())
        {
            add_metadata_header(h, headers, m.first, m.second);
        }

        a.credential()->sign_request(r, h, url, headers);
    }
}}
