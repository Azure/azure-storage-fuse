#include "get_block_list_request_base.h"

#include "constants.h"
#include "utility.h"

namespace azure {  namespace storage_lite {

        void get_block_list_request_base::build_request(const storage_account &a, http_base &h) const
        {
            const auto &r = *this;

            // TODO: allow setting max execution time.
            h.set_absolute_timeout(30L);

            h.set_method(http_base::http_method::get);

            storage_url url = a.get_url(storage_account::service::blob);
            url.append_path(r.container()).append_path(r.blob());

            url.add_query(constants::query_comp, constants::query_comp_blocklist);
            add_optional_query(url, constants::query_snapshot, r.snapshot());
            switch (r.blocklisttype()) {
            case blocklisttypes::all:
                url.add_query(constants::query_blocklisttype, constants::query_blocklisttype_all);
                break;
            case blocklisttypes::uncommitted:
                url.add_query(constants::query_blocklisttype, constants::query_blocklisttype_uncommitted);
                break;
            case blocklisttypes::committed:
                url.add_query(constants::query_blocklisttype, constants::query_blocklisttype_committed);
                break;
            }
            add_optional_query(url, constants::query_timeout, r.timeout());
            h.set_url(url.to_string());

            storage_headers headers;
            add_access_condition_headers(h, headers, r);

            add_ms_header(h, headers, constants::header_ms_client_request_id, r.ms_client_request_id(), true);
            add_ms_header(h, headers, constants::header_ms_lease_id, r.ms_lease_id(), true);

            h.add_header(constants::header_user_agent, constants::header_value_user_agent);
            add_ms_header(h, headers, constants::header_ms_date, get_ms_date(date_format::rfc_1123));
            add_ms_header(h, headers, constants::header_ms_version, constants::header_value_storage_blob_version);

            a.credential()->sign_request(r, h, url, headers);
        }

}}
