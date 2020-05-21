#include "delete_directory_request.h"
#include "http_base.h"
#include "storage_url.h"
#include "utility.h"

namespace azure { namespace storage_adls {

    void delete_directory_request::build_request(const storage_account& account, http_base& http) const
    {
        using namespace azure::storage_lite;

        http.set_method(http_base::http_method::del);

        storage_url url = account.get_url(storage_account::service::adls);
        url.append_path(m_filesystem).append_path(m_directory);
        url.add_query(constants::query_recursive, "true");
        add_optional_query(url, constants::query_continuation, m_continuation);

        http.set_url(url.to_string());

        storage_headers headers;
        http.add_header(constants::header_user_agent, constants::header_value_user_agent);
        add_ms_header(http, headers, constants::header_ms_date, get_ms_date(date_format::rfc_1123));
        add_ms_header(http, headers, constants::header_ms_version, constants::header_value_storage_blob_version);

        account.credential()->sign_request(*this, http, url, headers);
    }

}}  // azure::storage_adls