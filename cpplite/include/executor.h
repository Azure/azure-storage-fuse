#pragma once

#include <chrono>
#include <future>
#include <iterator>
#include <sstream>
#include <thread>

#include "storage_EXPORTS.h"

#include "common.h"
#include "storage_outcome.h"
#include "storage_account.h"
#include "http_base.h"
#include "xml_parser_base.h"
#include "json_parser_base.h"
#include "retry.h"
#include "utility.h"

namespace azure {  namespace storage_lite {

        class executor_context
        {
        public:
            executor_context(std::shared_ptr<xml_parser_base> xml_parser, std::shared_ptr<retry_policy_base> retry)
                : m_xml_parser(xml_parser),
                m_retry_policy(retry) {}

            std::shared_ptr<xml_parser_base> xml_parser() const
            {
                return m_xml_parser;
            }

            std::shared_ptr<json_parser_base> json_parser() const
            {
                return m_json_parser;
            }

            void set_json_parser(std::shared_ptr<json_parser_base> parser)
            {
                m_json_parser = std::move(parser);
            }

            std::shared_ptr<retry_policy_base> retry_policy() const
            {
                return m_retry_policy;
            }

            void set_retry_policy(std::shared_ptr<retry_policy_base> retry_policy)
            {
                m_retry_policy = std::move(retry_policy);
            }

        private:
            std::shared_ptr<xml_parser_base> m_xml_parser;
            std::shared_ptr<json_parser_base> m_json_parser;
            std::shared_ptr<retry_policy_base> m_retry_policy;
        };

        template<typename RESPONSE_TYPE>
        class async_executor
        {
        public:
            static void submit_helper(
                std::shared_ptr<std::promise<storage_outcome<RESPONSE_TYPE>>> promise,
                std::shared_ptr<storage_outcome<RESPONSE_TYPE>> outcome,
                std::shared_ptr<storage_account> account,
                std::shared_ptr<storage_request_base> request,
                std::shared_ptr<http_base> http,
                std::shared_ptr<executor_context> context,
                std::shared_ptr<retry_context> retry)
            {
                http->reset();
                http->set_error_stream([](http_base::http_code) { return true; }, storage_iostream::create_storage_stream());
                request->build_request(*account, *http);

                retry_info info = retry->numbers() == 0 ? retry_info(true, std::chrono::seconds(0)) : context->retry_policy()->evaluate(*retry);
                if (info.should_retry())
                {
                    http->submit([promise, outcome, account, request, http, context, retry](http_base::http_code result, storage_istream s, CURLcode code)
                    {
                        std::string str(std::istreambuf_iterator<char>(s.istream()), std::istreambuf_iterator<char>());
                        if (code != CURLE_OK || unsuccessful(result))
                        {
                            storage_error error;
                            if (code != CURLE_OK)
                            {
                                error.code = std::to_string(code);
                                error.code_name = curl_easy_strerror(code);
                            }
                            else
                            {
                                error = context->xml_parser()->parse_storage_error(str);
                                error.code = std::to_string(result);
                            }

                            *outcome = storage_outcome<RESPONSE_TYPE>(error);
                            retry->add_result(code == CURLE_OK ? result: 503);
                            http->reset_input_stream();
                            http->reset_output_stream();
                            async_executor<RESPONSE_TYPE>::submit_helper(promise, outcome, account, request, http, context, retry);
                        }
                        else if (http->get_response_header(constants::header_content_type).find(constants::header_value_content_type_json) != std::string::npos)
                        {
                            *outcome = storage_outcome<RESPONSE_TYPE>(context->json_parser()->parse_response<RESPONSE_TYPE>(str));
                            promise->set_value(*outcome);
                        }
                        else
                        {
                            *outcome = storage_outcome<RESPONSE_TYPE>(context->xml_parser()->parse_response<RESPONSE_TYPE>(str));
                            promise->set_value(*outcome);
                        }
                    }, info.interval());
                }
                else
                {
                    promise->set_value(*outcome);
                }
            }

            static std::future<storage_outcome<RESPONSE_TYPE>> submit(
                std::shared_ptr<storage_account> account,
                std::shared_ptr<storage_request_base> request,
                std::shared_ptr<http_base> http,
                std::shared_ptr<executor_context> context)
            {
                auto retry = std::make_shared<retry_context>();
                auto outcome = std::make_shared<storage_outcome<RESPONSE_TYPE>>();
                auto promise = std::make_shared<std::promise<storage_outcome<RESPONSE_TYPE>>>();
                async_executor<RESPONSE_TYPE>::submit_helper(promise, outcome, account, request, http, context, retry);
                return promise->get_future();
            }
        };

        template<>
        class async_executor<void> {
        public:
            static void submit_helper(
                std::shared_ptr<std::promise<storage_outcome<void>>> promise,
                std::shared_ptr<storage_outcome<void>> outcome,
                std::shared_ptr<storage_account> account,
                std::shared_ptr<storage_request_base> request,
                std::shared_ptr<http_base> http,
                std::shared_ptr<executor_context> context,
                std::shared_ptr<retry_context> retry)
            {
                http->reset();
                http->set_error_stream(unsuccessful, storage_iostream::create_storage_stream());
                request->build_request(*account, *http);

                retry_info info = retry->numbers() == 0 ? retry_info(true, std::chrono::seconds(0)) : context->retry_policy()->evaluate(*retry);
                if (info.should_retry())
                {
                    http->submit([promise, outcome, account, request, http, context, retry](http_base::http_code result, storage_istream s, CURLcode code)
                    {
                        if (code != CURLE_OK || unsuccessful(result))
                        {
                            storage_error error;
                            if (code != CURLE_OK)
                            {
                                error.code = std::to_string(code);
                                error.code_name = curl_easy_strerror(code);
                            }
                            else
                            {
                                std::string str(std::istreambuf_iterator<char>(s.istream()), std::istreambuf_iterator<char>());
                                error = context->xml_parser()->parse_storage_error(str);
                                error.code = std::to_string(result);
                            }

                            *outcome = storage_outcome<void>(error);
                            retry->add_result(code == CURLE_OK ? result: 503);
                            http->reset_input_stream();
                            http->reset_output_stream();
                            async_executor<void>::submit_helper(promise, outcome, account, request, http, context, retry);
                        }
                        else
                        {
                            *outcome = storage_outcome<void>();
                            promise->set_value(*outcome);
                        }
                    }, info.interval());
                }
                else
                {
                    promise->set_value(*outcome);
                }
            }

            static std::future<storage_outcome<void>> submit(
                std::shared_ptr<storage_account> account,
                std::shared_ptr<storage_request_base> request,
                std::shared_ptr<http_base> http,
                std::shared_ptr<executor_context> context)
            {
                auto retry = std::make_shared<retry_context>();
                auto outcome = std::make_shared<storage_outcome<void>>();
                auto promise = std::make_shared<std::promise<storage_outcome<void>>>();
                async_executor<void>::submit_helper(promise, outcome, account, request, http, context, retry);
                return promise->get_future();
            }
        };
    }
}
