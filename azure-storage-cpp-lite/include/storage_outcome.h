#pragma once

#include <string>

#include "storage_EXPORTS.h"

namespace microsoft_azure {
    namespace storage {

        class storage_error {
        public:
            std::string code;
            std::string code_name;
            std::string message;
        };

        template<typename RESPONSE_TYPE>
        class storage_outcome {
        public:
            storage_outcome()
                : m_success(false) {}

            storage_outcome(RESPONSE_TYPE response)
                : m_success(true),
                m_response(std::move(response)) {}

            storage_outcome(storage_error error)
                : m_success(false),
                m_error(std::move(error)) {}

            bool success() const {
                return m_success;
            }

            const storage_error &error() const {
                return m_error;
            }

            const RESPONSE_TYPE &response() const {
                return m_response;
            }

        private:
            bool m_success;
            storage_error m_error;
            RESPONSE_TYPE m_response;
        };

        template<>
        class storage_outcome<void> {
        public:
            storage_outcome()
                : m_success(true) {}

            storage_outcome(storage_error error)
                : m_success(false),
                m_error(std::move(error)) {}

            bool success() const {
                return m_success;
            }

            const storage_error &error() const {
                return m_error;
            }

        private:
            bool m_success;
            storage_error m_error;
        };

    }
}
