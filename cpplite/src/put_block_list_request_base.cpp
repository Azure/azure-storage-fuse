#include "put_block_list_request_base.h"

#include "constants.h"
#include "utility.h"
#include "xml_writer.h"
#include "storage_stream.h"

namespace azure {  namespace storage_lite {

void put_block_list_request_base::build_request(const storage_account &a, http_base &h) const
{
    const auto &r = *this;

    h.set_absolute_timeout(30L);

    h.set_method(http_base::http_method::put);

    storage_url url = a.get_url(storage_account::service::blob);
    url.append_path(r.container()).append_path(r.blob());

    url.add_query(constants::query_comp, constants::query_comp_blocklist);
    add_optional_query(url, constants::query_timeout, r.timeout());
    h.set_url(url.to_string());

    auto xml = xml_writer::write_block_list(r.block_list());
    auto ss = std::make_shared<std::stringstream>(xml);
    h.set_input_stream(storage_istream(ss));

    storage_headers headers;
    add_content_length(h, headers, static_cast<unsigned int>(xml.size()));
    add_optional_content_md5(h, headers, r.content_md5());
    add_access_condition_headers(h, headers, r);

    add_ms_header(h, headers, constants::header_ms_client_request_id, r.ms_client_request_id(), true);
    add_ms_header(h, headers, constants::header_ms_lease_id, r.ms_lease_id(), true);

    add_ms_header(h, headers, constants::header_ms_blob_cache_control, r.ms_blob_cache_control(), true);
    add_ms_header(h, headers, constants::header_ms_blob_content_disposition, r.ms_blob_content_disposition(), true);
    add_ms_header(h, headers, constants::header_ms_blob_content_encoding, r.ms_blob_content_encoding(), true);
    add_ms_header(h, headers, constants::header_ms_blob_content_language, r.ms_blob_content_language(), true);
    add_ms_header(h, headers, constants::header_ms_blob_content_md5, r.ms_blob_content_md5(), true);
    add_ms_header(h, headers, constants::header_ms_blob_content_type, r.ms_blob_content_type(), true);

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
    add_ms_header(h, headers, constants::header_ms_version, constants::header_value_storage_blob_version);

    a.credential()->sign_request(r, h, url, headers);
}

}}  // azure::storage_lite
