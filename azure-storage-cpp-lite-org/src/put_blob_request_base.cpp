#include "put_blob_request_base.h"

#include "constants.h"
#include "utility.h"

namespace microsoft_azure {
namespace storage {

void put_blob_request_base::build_request(const storage_account &a, http_base &h) const {
    const auto &r = *this;

    h.set_data_rate_timeout();

    h.set_method(http_base::http_method::put);

    storage_url url = a.get_url(storage_account::service::blob);
    url.append_path(r.container()).append_path(r.blob());

    add_optional_query(url, constants::query_timeout, r.timeout());
    h.set_url(url.to_string());

    storage_headers headers;
    add_optional_content_encoding(h, headers, r.content_encoding());
    add_optional_content_language(h, headers, r.content_language());
    add_content_length(h, headers, r.content_length());
    add_optional_content_md5(h, headers, r.content_md5());
    add_optional_content_type(h, headers, r.content_type());
    add_access_condition_headers(h, headers, r);

    add_optional_header(h, constants::header_cache_control, r.cache_control());
    add_optional_header(h, constants::header_origin, r.origin());

    add_ms_header(h, headers, constants::header_ms_client_request_id, r.ms_client_request_id(), true);
    add_ms_header(h, headers, constants::header_ms_lease_id, r.ms_lease_id(), true);

    add_ms_header(h, headers, constants::header_ms_blob_cache_control, r.ms_blob_cache_control(), true);
    add_ms_header(h, headers, constants::header_ms_blob_content_disposition, r.ms_blob_content_disposition(), true);
    add_ms_header(h, headers, constants::header_ms_blob_content_encoding, r.ms_blob_content_encoding(), true);
    add_ms_header(h, headers, constants::header_ms_blob_content_language, r.ms_blob_content_language(), true);
    if (r.ms_blob_type() == put_blob_request_base::blob_type::page_blob) {
        // check % 512
        add_ms_header(h, headers, constants::header_ms_blob_content_length, std::to_string(r.ms_blob_content_length()));
    }
    add_ms_header(h, headers, constants::header_ms_blob_content_md5, r.ms_blob_content_md5(), true);
    add_ms_header(h, headers, constants::header_ms_blob_content_type, r.ms_blob_content_type(), true);
    if (r.ms_blob_type() == put_blob_request_base::blob_type::page_blob) {
        add_ms_header(h, headers, constants::header_ms_blob_sequence_number, std::to_string(r.ms_blob_sequence_number()), true);
    }

    switch (r.ms_blob_type()) {
    case put_blob_request_base::blob_type::block_blob:
        add_ms_header(h, headers, constants::header_ms_blob_type, constants::header_value_blob_type_blockblob);
        break;
    case put_blob_request_base::blob_type::page_blob:
        add_ms_header(h, headers, constants::header_ms_blob_type, constants::header_value_blob_type_pageblob);
        break;
    case put_blob_request_base::blob_type::append_blob:
        add_ms_header(h, headers, constants::header_ms_blob_type, constants::header_value_blob_type_appendblob);
        break;
    }

    //add ms-meta
    if (r.metadata().size() > 0)
    {
        for (unsigned int i = 0; i < r.metadata().size(); i++)
        {
            add_metadata_header(h, headers, r.metadata()[i].first, r.metadata()[i].second);
        }
    }

    h.add_header(constants::header_user_agent, constants::header_value_user_agent);
    add_ms_header(h, headers, constants::header_ms_date, get_ms_date(date_format::rfc_1123));
    add_ms_header(h, headers, constants::header_ms_version, constants::header_value_storage_version);

    a.credential()->sign_request(r, h, url, headers);
}

}
}
