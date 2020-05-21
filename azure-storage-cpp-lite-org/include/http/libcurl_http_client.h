#pragma once

#include <algorithm>
#include <condition_variable>
#include <functional>
#include <future>
#include <map>
#include <memory>
#include <mutex>
#include <queue>
#include <sstream>
#include <string>
#include <thread>
#include <vector>

#include <curl/curl.h>
#include <syslog.h>
#include <utility.h>

#include "storage_EXPORTS.h"

#include "http_base.h"

namespace microsoft_azure {
    namespace storage {

        std::string to_lower(std::string original);

        class CurlEasyClient;

        class CurlEasyRequest final : public http_base
        {

            using REQUEST_TYPE = CurlEasyRequest;

            public:
                AZURE_STORAGE_API CurlEasyRequest(std::shared_ptr<CurlEasyClient> client, CURL *h);

                AZURE_STORAGE_API ~CurlEasyRequest();

                void set_url(const std::string &url) override {
                    m_url = url;
                }

                std::string get_url() const override {
                    return m_url;
                }

                void set_method(http_method method) override {
                    m_method = method;
                }

                http_method get_method() const override {
                    return m_method;
                }

                void add_header(const std::string &name, const std::string &value) override {
                    std::string header(name);
                    header.append(": ").append(value);
                    m_slist = curl_slist_append(m_slist, header.data());
                    if (name == "Content-Length") {
                        unsigned int l;
                        std::istringstream iss(value);
                        iss >> l;
                        curl_easy_setopt(m_curl, CURLOPT_INFILESIZE, l);
                    }
                }

                std::string get_header(const std::string &name) const override {
                    auto iter = m_headers.find(name);
                    if (iter != m_headers.end())
                    {
                        return iter->second;
                    }
                    else
                    {
                        return "";
                    }
                }
                const std::map<std::string, std::string, case_insensitive_compare>& get_headers() const override {
                    return m_headers;
                }

                AZURE_STORAGE_API CURLcode perform() override;

                void submit(std::function<void(http_code, storage_istream, CURLcode)> cb, std::chrono::seconds interval) override {
                    std::this_thread::sleep_for(interval);
                    const auto curlCode = perform();

                    syslog(curlCode != CURLE_OK || unsuccessful(m_code) ? LOG_ERR : LOG_DEBUG, "%s", format_request_response().c_str());

                    cb(m_code, m_error_stream, curlCode);
                }

                void reset() override {
                    m_headers.clear();
                    curl_slist_free_all(m_slist);
                    m_slist = NULL;
                    //curl_easy_setopt(m_curl, CURLOPT_INFILESIZE, -1);
                    //curl_easy_setopt(m_curl, CURLOPT_WRITEFUNCTION, NULL);
                    //curl_easy_setopt(m_curl, CURLOPT_READFUNCTION, NULL);
                }

                http_code status_code() const override {
                    return m_code;
                }

                /*void set_output_callback(OUT_CB output_callback) override {
                    m_output_callback = output_callback;
                    check_code(curl_easy_setopt(m_curl, CURLOPT_WRITEFUNCTION, write_callback));
                    check_code(curl_easy_setopt(m_curl, CURLOPT_WRITEDATA, this));
                }*/

                /*void set_input_callback(IN_CB input_callback) override {
                    m_input_callback = input_callback;
                    check_code(curl_easy_setopt(m_curl, CURLOPT_READFUNCTION, read_callback));
                    check_code(curl_easy_setopt(m_curl, CURLOPT_READDATA, this));
                }*/

                void set_output_stream(storage_ostream s) override {
                    m_output_stream = s;
                    check_code(curl_easy_setopt(m_curl, CURLOPT_WRITEFUNCTION, write));
                    check_code(curl_easy_setopt(m_curl, CURLOPT_WRITEDATA, this));
                }

                void set_error_stream(std::function<bool(http_code)> f, storage_iostream s) override {
                    m_switch_error_callback = f;
                    m_error_stream = s;
                    //check_code(curl_easy_setopt(m_curl, CURLOPT_WRITEFUNCTION, write));
                    //check_code(curl_easy_setopt(m_curl, CURLOPT_WRITEDATA, this));
                }

                void set_input_stream(storage_istream s) override {
                    m_input_stream = s;
                    check_code(curl_easy_setopt(m_curl, CURLOPT_READFUNCTION, read));
                    check_code(curl_easy_setopt(m_curl, CURLOPT_READDATA, this));
                    check_code(curl_easy_setopt(m_curl, CURLOPT_POSTFIELDS, nullptr)); // CURL won't actually read data on POSTs unless this is explicitly set.
                }

                void set_input_buffer(char* buff) override
                {
                    m_input_buffer = buff;
                    check_code(curl_easy_setopt(m_curl, CURLOPT_READFUNCTION, read));
                    check_code(curl_easy_setopt(m_curl, CURLOPT_READDATA, this));
                    check_code(curl_easy_setopt(m_curl, CURLOPT_POSTFIELDS, nullptr)); // CURL won't actually read data on POSTs unless this is explicitly set.
                }

                void set_input_content_length(size_t content_length)
                {
                    m_input_content_length=content_length;
                }

                size_t get_input_content_length(void)
                {
                    return m_input_content_length;
                }

                void set_is_input_length_known(void)
                {
                    m_is_input_length_known=true;
                }

                bool get_is_input_length_known(void)
                {
                    return m_is_input_length_known;
                }
                void reset_input_stream() override {
                    m_input_stream.reset();
                }

                void reset_output_stream() override {
                    m_output_stream.reset();
                }

                storage_ostream get_output_stream() const override {
                    return m_output_stream;
                }

                storage_iostream get_error_stream() const override {
                    return m_error_stream;
                }

                storage_istream get_input_stream() const override {
                    return m_input_stream;
                }

                void set_absolute_timeout(long long timeout) override {
                    check_code(curl_easy_setopt(m_curl, CURLOPT_TIMEOUT, timeout)); // Absolute timeout

                    // For the moment, we are only using one type of timeout per operation, so we clear the other one, in case it was set for this handle by a prior operation:
                    check_code(curl_easy_setopt(m_curl, CURLOPT_LOW_SPEED_TIME, 0L));
                    check_code(curl_easy_setopt(m_curl, CURLOPT_LOW_SPEED_LIMIT, 0L));
                }

                void set_data_rate_timeout() override {
                    // If the download speed is less than 17KB/sec for more than a minute, timout. This time was selected because it should ensure that downloading each megabyte take no more than a minute.
                    check_code(curl_easy_setopt(m_curl, CURLOPT_LOW_SPEED_TIME, 60L));
                    check_code(curl_easy_setopt(m_curl, CURLOPT_LOW_SPEED_LIMIT, 1024L * 17L));

                    // For the moment, we are only using one type of timeout per operation, so we clear the other one, in case it was set for this handle by a prior operation:
                    check_code(curl_easy_setopt(m_curl, CURLOPT_TIMEOUT, 0L));
                }

            private:
                std::shared_ptr<CurlEasyClient> m_client;
                CURL *m_curl;
                curl_slist *m_slist;

                http_method m_method;
                std::string m_url;
                char* m_input_buffer = NULL;
                int m_input_buffer_pos = 0;
                storage_istream m_input_stream;
                storage_ostream m_output_stream;
                storage_iostream m_error_stream;
                size_t m_input_content_length;
                bool m_is_input_length_known;
                std::function<bool(http_code)> m_switch_error_callback;
                http_code m_code;
                std::map<std::string, std::string, case_insensitive_compare> m_headers;

                std::string format_request_response()
                {
                    std::string out;
                    auto currentTime = std::time(nullptr);
                    auto timestamp = std::asctime(std::localtime(&currentTime));

                    auto sigLoc = m_url.find("sig=");
                    auto tmpURL = m_url;

                    if (sigLoc != std::string::npos) {
                        // Find the string and replace the segment
                        for(auto i = sigLoc; i < tmpURL.length(); i++) {
                            if(tmpURL[i] == '&' || i == tmpURL.length()-1) {
                                auto count =
                                        (i - sigLoc) + // The real count, if we landed on &
                                        (i == tmpURL.length() - 1 ? 1 : 0); // If we're at the end, trim to the end.
                                tmpURL.replace(sigLoc, count, "sig=REDACTED");
                                break;
                            }
                        }
                    }

                    out += timestamp;
                    out.erase(out.end()-1);
                    out += " ==> REQUEST/RESPONSE\n";
                    out += "\t" + http_method_label[m_method] + " " + tmpURL + "\n";

                    // our headers
                    for(auto x = m_slist; x->next != nullptr; x = x->next) {
                        std::string header = std::string(x->data);
                        auto splitAt = header.find(':');

                        if (splitAt != header.length() - 1) {
                            std::string name = header.substr(0, splitAt);
                            std::string value = header.substr(splitAt + 2);

                            if(to_lower(name) == "authorization" || to_lower(name) == "secret") {
                                value = "REDACTED";
                            }

                            out = out.append("\t").append(name).append(": [").append(value).append("]\n");
                        } else {
                            out = out.append("\t").append(header.substr(0, splitAt)).append(": []").append("\n");
                        }
                    }

                    out += "\t--------------------------------------------------------------------------------\n";
                    out += "\tRESPONSE Status: " + std::to_string(m_code) + "\n";

                    // their headers
                    for(const auto& pair : m_headers) {
                        auto lineReturn = pair.second.find('\n');
                        // ternary statement also trims the carriage return character, which accidentally clears lines.
                        auto trimmed_str = pair.second.substr(0, pair.second[lineReturn - 1] == '\r' ? lineReturn - 1 : lineReturn );
                        out = out.append("\t").append(pair.first).append(": [").append(trimmed_str).append("]\n");
                    }

                    return out;
                }

                AZURE_STORAGE_API static size_t header_callback(char *buffer, size_t size, size_t nitems, void *userdata);

                static size_t write(char *buffer, size_t size, size_t nitems, void *userdata)
                {
                    REQUEST_TYPE *p = static_cast<REQUEST_TYPE *>(userdata);
                    p->m_output_stream.ostream().write(buffer, size * nitems);
                    return size * nitems;
                }

                static size_t error(char *buffer, size_t size, size_t nitems, void *userdata)
                {
                    REQUEST_TYPE *p = static_cast<REQUEST_TYPE *>(userdata);
                    p->m_error_stream.ostream().write(buffer, size * nitems);
                    return size * nitems;
                }

                static size_t read(char *buffer, size_t size, size_t nitems, void *userdata)
                {
                    REQUEST_TYPE *p = static_cast<REQUEST_TYPE *>(userdata);
                    auto &s = p->m_input_stream.istream();
                    size_t contentlen = p->get_input_content_length();
                    size_t actual_size = 0 ;
                    if( ! p->get_is_input_length_known() ) {
                        auto cur = s.tellg();
                        s.seekg(0, std::ios_base::end);
                        auto end = s.tellg();
                        s.seekg(cur);
                        actual_size = std::min(static_cast<size_t>(end-cur), size * nitems);
                    }
                    else
                    {
                        actual_size = std::min(contentlen, size * nitems);
                    }

                    if (p->m_input_buffer != NULL)
                    {
                        memcpy(buffer, p->m_input_buffer + p->m_input_buffer_pos, actual_size);
                        p->m_input_buffer_pos += actual_size;
                    }
                    else
                    {
                        s.read(buffer, actual_size);
                    }

                    if(p->get_is_input_length_known()) {
                        contentlen -= actual_size;
                        p->set_input_content_length(contentlen);
                    }

                    return actual_size;
                }

                static void check_code(CURLcode code, std::string = std::string())
                {
                    if (code != CURLE_OK) {
                        //std::cout << s << ":" << curl_easy_strerror(code) << std::endl;
                    }
                    else
                    {
                        errno = 0; // CURL sometimes sets errno internally, if everything was ok we should reset it to zero.
                    }
                }
            };

        class CurlEasyClient : public std::enable_shared_from_this<CurlEasyClient> {
        public:
            CurlEasyClient(int size) : m_size(size) {
                curl_global_init(CURL_GLOBAL_DEFAULT);
                for (int i = 0; i < m_size; i++) {
                    CURL *h = curl_easy_init();
                    m_handles.push(h);
                }
            }
            //Sets CURL CA BUNDLE location for all the curl handlers.
            CurlEasyClient(int size, const std::string& ca_path) : m_size(size)
            {
                curl_global_init(CURL_GLOBAL_DEFAULT);
                for (int i = 0; i < m_size; i++) {
                    CURL *h = curl_easy_init();
                    curl_easy_setopt(h, CURLOPT_CAPATH, ca_path.c_str());
                    m_handles.push(h);
                }
            }

            ~CurlEasyClient() {
                while (!m_handles.empty()) {
                    curl_easy_cleanup(m_handles.front());
                    m_handles.pop();
                }
                curl_global_cleanup();
            }

            int size()
            {
                return m_size;
            }

            std::shared_ptr<CurlEasyRequest> get_handle() {
                std::unique_lock<std::mutex> lk(m_handles_mutex);
                m_cv.wait(lk, [this]() { return !m_handles.empty(); });
                auto res = std::make_shared<CurlEasyRequest>(shared_from_this(), m_handles.front());
                m_handles.pop();
                return res;
            }

            void release_handle(CURL *h) {
                std::lock_guard<std::mutex> lg(m_handles_mutex);
                m_handles.push(h);
                m_cv.notify_one();
            }

        private:
            int m_size;
            std::queue<CURL *> m_handles;
            std::mutex m_handles_mutex;
            std::condition_variable m_cv;
        };
    }
}
