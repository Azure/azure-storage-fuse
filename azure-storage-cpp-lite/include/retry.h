#pragma once

#include <chrono>
#include <algorithm>
#include <math.h>

#include "storage_EXPORTS.h"

#include "http_base.h"
#include "utility.h"

#define MAX_RETRIES 26 //timeout around 20 minutes

namespace microsoft_azure {
    namespace storage {

        class retry_info {
        public:
            retry_info(bool should_retry, std::chrono::seconds interval)
                : m_should_retry(should_retry),
                m_interval(interval) {}

            bool should_retry() const {
                return m_should_retry;
            }

            std::chrono::seconds interval() const {
                return m_interval;
            }

        private:
            bool m_should_retry;
            std::chrono::seconds m_interval;
        };

        class retry_context {
        public:
            retry_context()
                : m_retry_limit(MAX_RETRIES),
                m_retry_count(0),
                m_result(0) {}

            retry_context(int new_limit, http_base::http_code result)
                : m_retry_limit(new_limit),
                m_result(result) {}

            int limit() const {
                return m_retry_limit;
            }

            int current_count() const
            {
                return m_retry_count;
            }

            http_base::http_code result() const {
                return m_result;
            }

            void add_result(http_base::http_code result) {
                m_result = result;
                m_retry_count++;
            }

            void set_retry_limit(int new_limit){
                m_retry_limit = new_limit;
            }

        private:
            int m_retry_limit;
            int m_retry_count;
            http_base::http_code m_result;
        };

        class retry_policy_base {
        public:
            virtual retry_info evaluate(const retry_context &context) const = 0;
        };

        class retry_policy : public retry_policy_base {
        public:
            retry_info evaluate(const retry_context &context) const override {
                if (context.current_count() == 0) {
                    return retry_info(true, std::chrono::seconds(0));
                } else if (context.current_count() < context.limit() && can_retry(context.result())) {
                    double delay = (pow(1.2, context.current_count()-1)-1);
                    delay = std::min(delay, 60.0); // Maximum backoff delay of 1 minute
                    delay *= (((double)rand())/RAND_MAX)/2 + 0.75;
                    return retry_info(true, std::chrono::seconds((int)delay));
                }
                return retry_info(false, std::chrono::seconds(0));
            }

        private:
            bool can_retry(http_base::http_code code) const {
                return retryable(code);
            }
        };

    }
}
