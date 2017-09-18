#pragma once

#include <string>
#include <vector>

#include "storage_EXPORTS.h"

#include "http_base.h"
#include "storage_request_base.h"
#include "storage_url.h"

namespace microsoft_azure {
    namespace storage {

        class storage_credential {
        public:
            virtual void sign_request(const storage_request_base &, http_base &, const storage_url &, const storage_headers &) const {}
        };

        class shared_key_credential : public storage_credential {
        public:
            AZURE_STORAGE_API shared_key_credential(const std::string &account_name, const std::string &account_key);

            AZURE_STORAGE_API shared_key_credential(const std::string &account_name, const std::vector<unsigned char> &account_key);

            AZURE_STORAGE_API void sign_request(const storage_request_base &r, http_base &h, const storage_url &url, const storage_headers &headers) const override;

            AZURE_STORAGE_API void sign_request(const table_request_base &r, http_base &h, const storage_url &url, const storage_headers &headers) const;

            const std::string &account_name() const {
                return m_account_name;
            }

            const std::vector<unsigned char> &account_key() const {
                return m_account_key;
            }

        private:
            std::string m_account_name;
            std::vector<unsigned char> m_account_key;
        };

        class shared_access_signature_credential : public storage_credential {
        public:
            shared_access_signature_credential(const std::string &sas_token)
                : m_sas_token(sas_token) {}

            AZURE_STORAGE_API void sign_request(const storage_request_base &r, http_base &h, const storage_url &url, const storage_headers &headers) const override;

        private:
            std::string m_sas_token;
        };

        class anonymous_credential : public storage_credential {
        public:
            void sign_request(const storage_request_base &, http_base &, const storage_url &, const storage_headers &) const override {}
        };

    }
}
