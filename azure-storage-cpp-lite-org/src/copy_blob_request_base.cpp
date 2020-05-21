#include "copy_blob_request_base.h"

#include "constants.h"
#include "utility.h"

namespace microsoft_azure {
    namespace storage {

        void copy_blob_request_base::build_request(const storage_account &a, http_base &h) const {
            const auto &r = *this;

            h.set_absolute_timeout(5L);

            h.set_method(http_base::http_method::put);

            // source
            storage_url source_url = a.get_url(storage_account::service::blob);
            source_url.append_path(r.container()).append_path(r.blob());

            // dest
            storage_url url = a.get_url(storage_account::service::blob);
            url.append_path(r.destContainer()).append_path(r.destBlob());

            add_optional_query(url, constants::query_timeout, r.timeout());
            h.set_url(url.to_string());

            storage_headers headers;
            //add_access_condition_headers(h, headers, r);
            add_content_length(h, headers, 0);

            add_ms_header(h, headers, constants::header_ms_client_request_id, r.ms_client_request_id(), true);

            h.add_header(constants::header_user_agent, constants::header_value_user_agent);
            add_ms_header(h, headers, constants::header_ms_date, get_ms_date(date_format::rfc_1123));
            add_ms_header(h, headers, constants::header_ms_version, constants::header_value_storage_version);

            // set copy src
            add_ms_header(h, headers, constants::header_ms_copy_source, a.credential()->transform_url(source_url.get_domain() + source_url.get_path()));

            a.credential()->sign_request(r, h, url, headers);
        }

    }
}
