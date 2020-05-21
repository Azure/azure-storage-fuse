#pragma once

#include <chrono>
#include <algorithm>
#include <math.h>

#include "storage_EXPORTS.h"

#include "http_base.h"
#include "utility.h"

namespace azure {  namespace storage_lite {

    class retry_info final
    {
    public:
        retry_info(bool should_retry, std::chrono::seconds interval)
            : m_should_retry(should_retry),
            m_interval(interval) {}

        bool should_retry() const
        {
            return m_should_retry;
        }

        std::chrono::seconds interval() const
        {
            return m_interval;
        }

    private:
        bool m_should_retry;
        std::chrono::seconds m_interval;
    };

    class retry_context final
    {
    public:
        retry_context()
            : m_numbers(0),
            m_result(0) {}

        retry_context(int numbers, http_base::http_code result)
            : m_numbers(numbers),
            m_result(result) {}

        int numbers() const
        {
            return m_numbers;
        }

        http_base::http_code result() const
        {
            return m_result;
        }

        void add_result(http_base::http_code result)
        {
            m_result = result;
            m_numbers++;
        }

    private:
        int m_numbers;
        http_base::http_code m_result;
    };

    class retry_policy_base
    {
    public:
        virtual ~retry_policy_base() {}
        virtual retry_info evaluate(const retry_context &context) const = 0;
    };

    // Default retry policy
    class retry_policy final : public retry_policy_base
    {
    public:
        retry_info evaluate(const retry_context& context) const override
        {
            const int max_retry_count = 3;
            if (context.numbers() <= max_retry_count && can_retry(context.result()))
            {
                return retry_info(true, std::chrono::seconds(0));
            }
            return retry_info(false, std::chrono::seconds(0));
        }

    private:
        bool can_retry(http_base::http_code code) const
        {
            return retryable(code);
        }
    };

    // No-retry policy
    class no_retry_policy final : public retry_policy_base
    {
    public:
        retry_info evaluate(const retry_context& context) const override
        {
            unused(context);
            return retry_info(false, std::chrono::seconds(0));
        }
    };

}}  // azure::storage_lite
