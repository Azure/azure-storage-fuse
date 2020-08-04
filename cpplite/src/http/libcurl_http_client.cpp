#include <sstream>

#include "http/libcurl_http_client.h"

#include "constants.h"

namespace azure { namespace storage_lite {

        CurlEasyRequest::CurlEasyRequest(std::shared_ptr<CurlEasyClient> client, CURL *h)
            : m_client(client), m_curl(h), m_slist(NULL)
        {
            check_code(curl_easy_setopt(m_curl, CURLOPT_NOSIGNAL, 1L));
            check_code(curl_easy_setopt(m_curl, CURLOPT_HEADERFUNCTION, header_callback));
            check_code(curl_easy_setopt(m_curl, CURLOPT_HEADERDATA, this));
        }

        CurlEasyRequest::~CurlEasyRequest()
        {
            curl_easy_reset(m_curl);
            m_client->release_handle(m_curl);
            if (m_slist) {
                curl_slist_free_all(m_slist);
            }
        }

        CURLcode CurlEasyRequest::perform()
        {
            if (m_output_stream.valid())
            {
                check_code(curl_easy_setopt(m_curl, CURLOPT_WRITEFUNCTION, write));
                check_code(curl_easy_setopt(m_curl, CURLOPT_WRITEDATA, this));
            }
            check_code(curl_easy_setopt(m_curl, CURLOPT_CUSTOMREQUEST, NULL));
            switch (m_method)
            {
            case http_method::get:
                check_code(curl_easy_setopt(m_curl, CURLOPT_HTTPGET, 1L));
                break;
            case http_method::put:
                check_code(curl_easy_setopt(m_curl, CURLOPT_UPLOAD, 1L));
                break;
            case http_method::del:
                check_code(curl_easy_setopt(m_curl, CURLOPT_CUSTOMREQUEST, constants::http_delete));
                break;
            case http_method::head:
                check_code(curl_easy_setopt(m_curl, CURLOPT_HTTPGET, 1L));
                check_code(curl_easy_setopt(m_curl, CURLOPT_NOBODY, 1L));
                break;
            case http_method::post:
                check_code(curl_easy_setopt(m_curl, CURLOPT_CUSTOMREQUEST, constants::http_post));
                break;
            case http_method::patch:
                check_code(curl_easy_setopt(m_curl, CURLOPT_UPLOAD, 1L));
                check_code(curl_easy_setopt(m_curl, CURLOPT_CUSTOMREQUEST, constants::http_patch));
                break;
            }

            check_code(curl_easy_setopt(m_curl, CURLOPT_URL, m_url.data()));

            m_slist = curl_slist_append(m_slist, "Transfer-Encoding:");
            m_slist = curl_slist_append(m_slist, "Expect:");
            check_code(curl_easy_setopt(m_curl, CURLOPT_HTTPHEADER, m_slist));

            if (!m_client->get_capath().empty())
            {
                check_code(curl_easy_setopt(m_curl, CURLOPT_CAINFO, m_client->get_capath().data()));
            }

            if (!m_client->get_proxy().empty())
            {
                check_code(curl_easy_setopt(m_curl, CURLOPT_PROXY, m_client->get_proxy().data()));
            }

            const auto result = curl_easy_perform(m_curl);
            check_code(result); // has nothing to do with checks, just resets errno for succeeded ops.
            return result;
        }

        size_t CurlEasyRequest::header_callback(char *buffer, size_t size, size_t nitems, void *userdata)
        {
            CurlEasyRequest::REQUEST_TYPE *p = static_cast<CurlEasyRequest::REQUEST_TYPE *>(userdata);
            std::string header(buffer, size * nitems);
            if (!header.empty() && header.back() == '\n')
            {
                header.pop_back();
            }
            if (!header.empty() && header.back() == '\r')
            {
                header.pop_back();
            }
            auto colon = header.find(':');
            if (colon == std::string::npos) {
                auto space = header.find(' ');
                if (space != std::string::npos) {
                    std::istringstream iss(header.substr(space));
                    iss >> p->m_code;
                    if (p->m_switch_error_callback && (p->m_switch_error_callback)(p->m_code)) {
                        curl_easy_setopt(p->m_curl, CURLOPT_WRITEFUNCTION, error);
                        curl_easy_setopt(p->m_curl, CURLOPT_WRITEDATA, p);
                    }
                }
            }
            else {
                p->m_response_headers[header.substr(0, colon)] = header.substr(colon + 2);
            }
            return size * nitems;
        }

}} // azure::storage_lite
