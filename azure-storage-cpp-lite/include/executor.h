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
#include "retry.h"
#include "utility.h"

namespace microsoft_azure {
    namespace storage {

        class executor_context {
        public:
            executor_context(std::shared_ptr<xml_parser_base> xml_parser, std::shared_ptr<retry_policy_base> retry)
                : m_xml_parser(xml_parser),
                m_retry_policy(retry) {}

            std::shared_ptr<xml_parser_base> xml_parser() const {
                return m_xml_parser;
            }

            std::shared_ptr<retry_policy_base> retry_policy() const {
                return m_retry_policy;
            }

        private:
            std::shared_ptr<xml_parser_base> m_xml_parser;
            std::shared_ptr<retry_policy_base> m_retry_policy;
        };

        /*
        template<typename RESPONSE_TYPE>
        class executor {
        public:
            static storage_outcome<RESPONSE_TYPE> make_request_once(const storage_account &a, const storage_request_base &r, http_base &h, const xml_parser_base &x, retry_context &context) {
                std::stringstream ss;
                h.set_output_stream(storage_stream(ss));
                r.build_request(a, h);

                auto result = h.perform();
                context.add_result(result);

                if (unsuccessful(result)) {
                    return storage_outcome<RESPONSE_TYPE>(x.parse_storage_error(ss.str()));
                }
                return storage_outcome<RESPONSE_TYPE>(x.parse_response<RESPONSE_TYPE>(ss.str()));
            }

            static storage_outcome<RESPONSE_TYPE> make_requests(const storage_account &a, const storage_request_base &r, http_base &h, const xml_parser_base &x, const retry_policy &policy) {
                retry_context context(0, 0);

                auto outcome = executor<RESPONSE_TYPE>::make_request_once(a, r, h, x, context);
                while (!outcome.success()) {
                    retry_info info = policy.evaluate(context);
                    if (!info.should_retry()) {
                        break;
                    }

                    std::this_thread::sleep_for(info.interval());
                    outcome = executor<RESPONSE_TYPE>::make_request_once(a, r, h, x, context);
                }

                return outcome;
            }

        };
        */

        template<typename RESPONSE_TYPE>
        class async_executor {
        public:
            static void submit_request(std::promise<storage_outcome<RESPONSE_TYPE>> &promise, const storage_account &a, const storage_request_base &r, http_base &h, const executor_context &context, retry_context &retry) {
                h.set_error_stream([](http_base::http_code) { return true; }, storage_iostream::create_storage_stream());
                r.build_request(a, h);

                retry_info info = context.retry_policy()->evaluate(retry);
                if (info.should_retry()) {
                    h.submit([&promise, &a, &r, &h, &context, &retry](http_base::http_code result, storage_istream s) {
                        std::string str(std::istreambuf_iterator<char>(s.istream()), std::istreambuf_iterator<char>());
                        if (unsuccessful(result)) {
                            promise.set_value(storage_outcome<RESPONSE_TYPE>(context.xml_parser()->parse_storage_error(str)));
                            retry.add_result(result);
                            s.istream().seekg(0);
                            async_executor<RESPONSE_TYPE>::submit_request(promise, a, r, h, context, retry);
                        }
                        else {
                            promise.set_value(storage_outcome<RESPONSE_TYPE>(context.xml_parser()->parse_response<RESPONSE_TYPE>(str)));
                        }
                    }, info.interval());
                }
            }

            static void submit_helper(
                std::shared_ptr<std::promise<storage_outcome<RESPONSE_TYPE>>> promise,
                std::shared_ptr<storage_outcome<RESPONSE_TYPE>> outcome,
                std::shared_ptr<storage_account> account,
                std::shared_ptr<storage_request_base> request,
                std::shared_ptr<http_base> http,
                std::shared_ptr<executor_context> context,
                std::shared_ptr<retry_context> retry)
            {
                http->set_error_stream([](http_base::http_code) { return true; }, storage_iostream::create_storage_stream());
                request->build_request(*account, *http);

                retry_info info = context->retry_policy()->evaluate(*retry);
                if (info.should_retry())
                {
                    http->submit([promise, outcome, account, request, http, context, retry](http_base::http_code result, storage_istream s)
                    {
                        std::string str(std::istreambuf_iterator<char>(s.istream()), std::istreambuf_iterator<char>());
                        if (unsuccessful(result))
                        {
                            auto error = context->xml_parser()->parse_storage_error(str);
                            error.code = std::to_string(result);
                            *outcome = storage_outcome<RESPONSE_TYPE>(error);
                            //*outcome = storage_outcome<RESPONSE_TYPE>(context->xml_parser()->parse_storage_error(str));
                            retry->add_result(result);
                            s.istream().seekg(0);
                            async_executor<RESPONSE_TYPE>::submit_helper(promise, outcome, account, request, http, context, retry);
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
            static void submit_request(std::promise<storage_outcome<void>> &promise, const storage_account &a, const storage_request_base &r, http_base &h, const executor_context &context, retry_context &retry) {
                h.set_error_stream(unsuccessful, storage_iostream::create_storage_stream());
                r.build_request(a, h);

                retry_info info = context.retry_policy()->evaluate(retry);
                if (info.should_retry()) {
                    h.submit([&promise, &a, &r, &h, &context, &retry](http_base::http_code result, storage_istream s) {
                        std::string str(std::istreambuf_iterator<char>(s.istream()), std::istreambuf_iterator<char>());
                        if (unsuccessful(result)) {
                            promise.set_value(storage_outcome<void>(context.xml_parser()->parse_storage_error(str)));
                            retry.add_result(result);
                            s.istream().seekg(0);
                            async_executor<void>::submit_request(promise, a, r, h, context, retry);
                        }
                        else {
                            promise.set_value(storage_outcome<void>());
                        }
                    }, info.interval());
                }
            }

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

                retry_info info = context->retry_policy()->evaluate(*retry);
                if (info.should_retry())
                {
                    http->submit([promise, outcome, account, request, http, context, retry](http_base::http_code result, storage_istream s)
                    {
                        std::string str(std::istreambuf_iterator<char>(s.istream()), std::istreambuf_iterator<char>());
                        if (unsuccessful(result))
                        {
                            auto error = context->xml_parser()->parse_storage_error(str);
                            error.code = std::to_string(result);
                            *outcome = storage_outcome<void>(error);
                            //*outcome = storage_outcome<void>(context->xml_parser()->parse_storage_error(str));
                            retry->add_result(result);
                            s.istream().seekg(0);
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

        /*
        template<>
        class executor<void> {
        public:
            static storage_outcome<void> make_request_once(const storage_account &a, const storage_request_base &r, http_base &h, const xml_parser_base &x, retry_context &context) {
                std::stringstream ss;
                h.set_error_stream(unsuccessful, storage_stream(ss));
                r.build_request(a, h);

                auto status_code = h.perform();
                if (unsuccessful(status_code)) {
                    return storage_outcome<void>(x.parse_storage_error(ss.str()));
                }

                return storage_outcome<void>();
            }

            static storage_outcome<void> make_requests(const storage_account &a, const storage_request_base &r, http_base &h, const xml_parser_base &x, const retry_policy &policy) {
                retry_context context(0, 0);

                auto outcome = executor<void>::make_request_once(a, r, h, x, context);
                while (!outcome.success()) {
                    retry_info info = policy.evaluate(context);
                    if (!info.should_retry()) {
                        break;
                    }

                    std::this_thread::sleep_for(info.interval());
                    outcome = executor<void>::make_request_once(a, r, h, x, context);
                }

                return outcome;
            }
        };*/

    }
}
