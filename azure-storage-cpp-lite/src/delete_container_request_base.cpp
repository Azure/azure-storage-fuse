#include "delete_container_request_base.h"

#include "constants.h"
#include "utility.h"

namespace microsoft_azure {
    namespace storage {

        void delete_container_request_base::build_request(const storage_account &a, http_base &h) const {
            const auto &r = *this;

            h.set_absolute_timeout(5L);

            h.set_method(http_base::http_method::del);

            storage_url url = a.get_url(storage_account::service::blob);
            url.append_path(r.container());

            url.add_query(constants::query_restype, constants::query_restype_container);
            add_optional_query(url, constants::query_timeout, r.timeout());
            h.set_url(url.to_string());

            storage_headers headers;
            add_ms_header(h, headers, constants::header_ms_client_request_id, r.ms_client_request_id(), true);
            add_ms_header(h, headers, constants::header_ms_lease_id, r.ms_lease_id(), true);

            h.add_header(constants::header_user_agent, constants::header_value_user_agent);
            add_ms_header(h, headers, constants::header_ms_date, get_ms_date(date_format::rfc_1123));
            add_ms_header(h, headers, constants::header_ms_version, constants::header_value_storage_version);

            a.credential()->sign_request(r, h, url, headers);
        }

    }
}
