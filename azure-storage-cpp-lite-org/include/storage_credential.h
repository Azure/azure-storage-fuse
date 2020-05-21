#pragma once

#include <string>
#include <vector>
#include <mutex>

#include "storage_EXPORTS.h"

#include "http_base.h"
#include "storage_request_base.h"
#include "storage_url.h"
#include "OAuthTokenCredentialManager.h"

namespace microsoft_azure {
    namespace storage {

        class storage_credential {
        public:
            virtual ~storage_credential() {};
            virtual void sign_request(const storage_request_base &, http_base &, const storage_url &, const storage_headers &) const {}
            virtual std::string transform_url(std::string url) const {
                return url;
            }
        };

        class shared_key_credential final : public storage_credential {
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

        class shared_access_signature_credential final : public storage_credential {
        public:
            shared_access_signature_credential(const std::string &sas_token)
                : m_sas_token(sas_token) {
            // If there is a question mark at the beginning of the sas token, erase it for easier processing in sign_request.
            if (!m_sas_token.empty() && m_sas_token[0] == '?') {
                m_sas_token.erase(0, 1);
            }
	    }

        AZURE_STORAGE_API void sign_request(const storage_request_base &r, http_base &h, const storage_url &url, const storage_headers &headers) const override;
        AZURE_STORAGE_API std::string transform_url(std::string url) const override;

        private:
            std::string m_sas_token;
        };

        class anonymous_credential final : public storage_credential {
        public:
            void sign_request(const storage_request_base &, http_base &, const storage_url &, const storage_headers &) const override {}
        };

        class token_credential : public storage_credential {
        public:
            // NOTE: This is a blobfuse linked function.
            // It links up with the blobfuse-initialized OAuthTokenManager.
            // If it was not already initialized, this will throw.
            AZURE_STORAGE_API token_credential();

            // DEPRECATION NOTICE: This is mostly meant for compatibility with sample.cpp
            // The new token credential uses the OAuthTokenCredentialManager, and explicitly set tokens disable this functionality!
            AZURE_STORAGE_API token_credential(const std::string &token);

            void sign_request(
                    const storage_request_base &,
                    http_base &,
                    const storage_url &,
                    const storage_headers &
                    ) const override;

            // DEPRECATION NOTICE: This is mostly meant for compatibility with sample.cpp
            // The new token credential uses the OAuthTokenCredentialManager, and explicitly set tokens disable this functionality!
            void set_token(const std::string &token);

        private:
            std::shared_ptr<OAuthTokenCredentialManager> m_credmgr_ptr;
            std::string m_token;
            mutable std::mutex m_token_mutex;
        };

    }
}
