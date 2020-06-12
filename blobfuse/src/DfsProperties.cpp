//
// Created by adreed on 2/7/2020.
//

#include <DfsProperties.h>
#include <BlobfuseConstants.h>
#include <utility.h>

#include <base64.h>
#include <blob/blob_client.h>
#include <adls_client.h>

using namespace azure::storage_adls;
using namespace azure::storage_lite;

namespace azure { namespace storage_adls {

void get_dfs_properties_request::build_request(const storage_account &account, http_base &http) const
{
    http.set_method(http_base::http_method::head);

    storage_url url = account.get_url(storage_account::service::adls);
    url.append_path(m_filesystem).append_path(m_path);

    http.set_url(url.to_string());

    storage_headers headers;
    http.add_header(blobfuse_constants::header_user_agent, blobfuse_constants::header_value_user_agent);
    add_ms_header(http, headers, blobfuse_constants::header_ms_date, get_ms_date(date_format::rfc_1123));
    add_ms_header(http, headers, blobfuse_constants::header_ms_version, blobfuse_constants::header_value_storage_version);

    account.credential()->sign_request(*this, http, url, headers);
}

template<class RET, class FUNC>
RET adls_client_ext::blob_client_adaptor_ext(FUNC func)
{
    try
    {
        storage_outcome<RET> result = func().get();

        if (result.success() && !m_exception_enabled)
        {
            errno = 0;
        }
        else if (!result.success())
        {
            int error_code = std::stoi(result.error().code);
            if (m_exception_enabled)
            {
                throw storage_exception(error_code, result.error().code_name, result.error().message);
            }
            else
            {
                errno = error_code;
            }
        }

        return result.response();
    }
    catch (std::exception& e)
    {
        if (m_exception_enabled)
        {
            throw;
        }
        else
        {
            errno = blobfuse_constants::unknown_error;
        }
    }
    return RET();
}

dfs_properties adls_client_ext::get_dfs_path_properties(const std::string &filesystem, const std::string &path) 
{
    auto http = m_blob_client->client()->get_handle();
    auto request = std::make_shared<get_dfs_properties_request>(filesystem, path);
    auto async_func = std::bind(&azure::storage_lite::async_executor<void>::submit, m_account, request, http, m_context);
    blob_client_adaptor_ext<void>(async_func);   

    dfs_properties props;
    if (success())
    {
        props.cache_control = http->get_response_header(constants::header_cache_control);
        props.content_disposition = http->get_response_header(constants::header_content_disposition);
        props.content_encoding = http->get_response_header(constants::header_content_encoding);
        props.content_language = http->get_response_header(constants::header_content_language);

        std::string cLen = http->get_response_header(constants::header_content_length);
        if (!cLen.empty())
            props.content_length = std::stoull(cLen);

        props.content_type = http->get_response_header(constants::header_content_type);
        props.content_md5 = http->get_response_header(constants::header_content_md5);
        props.etag = http->get_response_header(constants::header_etag);
        props.last_modified = curl_getdate(http->get_response_header(constants::header_last_modified).c_str(), NULL);
        props.resource_type = http->get_response_header(blobfuse_constants::header_ms_resource_type);
        props.owner = http->get_response_header(constants::header_ms_owner);
        props.group = http->get_response_header(constants::header_ms_group);
        props.permissions = http->get_response_header(constants::header_ms_permissions);
        props.acl = http->get_response_header(constants::header_ms_acl);

        // props.metadata TODO
        props.metadata = std::vector<std::pair<std::string, std::string>>{};

        std::string runningString, propName;
        for(char c : http->get_response_header(blobfuse_constants::header_ms_properties)) {
            if(propName.empty()) {
                if (c == '=') {
                    // Push state forward for the property value
                    propName = runningString;
                    runningString = "";
                } else {
                    runningString += c;
                }
            } else {
                if (c == ',') {
                    // base64 decode the value, write the metadata the vector, and move on.
                    std::string prop;
                    for (unsigned char dc : from_base64(runningString)) {
                        prop += dc;
                    }

                    props.metadata.emplace_back(propName, prop);

                    // Reset state for the next property
                    propName = "";
                    runningString = "";
                } else {
                    runningString += c;
                }
            }
        }
    }

    return props;
}

}}
