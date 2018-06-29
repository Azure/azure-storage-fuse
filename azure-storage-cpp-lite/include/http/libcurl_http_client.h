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

#include "storage_EXPORTS.h"

#include "http_base.h"

namespace microsoft_azure {
    namespace storage {

        class CurlEasyClient;

        class CurlEasyRequest : public http_base {

            using MY_TYPE = CurlEasyRequest;

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
            const std::map<std::string, std::string>& get_headers() const override {
                return m_headers;
            }

            AZURE_STORAGE_API CURLcode perform() override;

            void submit(std::function<void(http_code, storage_istream, CURLcode)> cb, std::chrono::seconds interval) override {
                std::this_thread::sleep_for(interval);
                const auto curlCode = perform();
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
            //IN_CB m_input_callback;
            //OUT_CB m_output_callback;
            storage_istream m_input_stream;
            storage_ostream m_output_stream;
            storage_iostream m_error_stream;
            std::function<bool(http_code)> m_switch_error_callback;

            http_code m_code;
            std::map<std::string, std::string> m_headers;

            AZURE_STORAGE_API static size_t header_callback(char *buffer, size_t size, size_t nitems, void *userdata);

            /*static size_t write_callback(char *buffer, size_t size, size_t nitems, void *userdata) {
                MY_TYPE *p = static_cast<MY_TYPE *>(userdata);
                p->m_output_callback(buffer, size * nitems);
                return size * nitems;
            }*/

            /*static size_t read_callback(char *buffer, size_t size, size_t nitems, void *userdata) {
                MY_TYPE *p = static_cast<MY_TYPE *>(userdata);
                return p->m_input_callback(buffer, size * nitems);
            }*/

            static size_t write(char *buffer, size_t size, size_t nitems, void *userdata) {
                MY_TYPE *p = static_cast<MY_TYPE *>(userdata);
                p->m_output_stream.ostream().write(buffer, size * nitems);
                return size * nitems;
            }

            static size_t error(char *buffer, size_t size, size_t nitems, void *userdata) {
                MY_TYPE *p = static_cast<MY_TYPE *>(userdata);
                p->m_error_stream.ostream().write(buffer, size * nitems);
                return size * nitems;
            }

            static size_t read(char *buffer, size_t size, size_t nitems, void *userdata) {
                MY_TYPE *p = static_cast<MY_TYPE *>(userdata);
                auto &s = p->m_input_stream.istream();

                auto cur = s.tellg();
                s.seekg(0, std::ios_base::end);
                auto end = s.tellg();
                s.seekg(cur);

                auto actual_size = std::min(static_cast<size_t>(end - cur), size * nitems);
                s.read(buffer, actual_size);
                return actual_size;
            }

            static void check_code(CURLcode code, std::string = std::string()) {
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
