#pragma once

#include <string>
#include <vector>

#ifdef WIN32
#include <Windows.h>
#include <bcrypt.h>
#else
#include <gnutls/gnutls.h>
#include <gnutls/crypto.h>
#include <gcrypt.h>
#include <pthread.h>
#define SHA256_DIGEST_LENGTH    32
#endif

#include "storage_EXPORTS.h"

namespace microsoft_azure {
    namespace storage {

#ifdef WIN32
            class hash_algorithm_base {};

            class hmac_sha256_hash_algorithm : public hash_algorithm_base {
                BCRYPT_ALG_HANDLE _algorithm_handle;
            public:
                BCRYPT_ALG_HANDLE handle() { return _algorithm_handle; }

                hmac_sha256_hash_algorithm() {
                    NTSTATUS status = BCryptOpenAlgorithmProvider(&_algorithm_handle, BCRYPT_SHA256_ALGORITHM /* BCRYPT_MD5_ALGORITHM */, NULL, BCRYPT_ALG_HANDLE_HMAC_FLAG /* 0 */);
                    if (status != 0) {
                        throw std::system_error(status, std::system_category());
                    }
                }

                ~hmac_sha256_hash_algorithm() {
                    BCryptCloseAlgorithmProvider(_algorithm_handle, 0);
                }
            };

            template<typename Hash_Provider>
            class hash_provider_base {
                static std::string hash_impl(const std::string &input, const std::vector<unsigned char> &key) = delete;
            public:
                static std::string hash(const std::string &input, const std::vector<unsigned char> &key) {
                    return Hash_Provider::hash_impl(input, key);
                }
            };

            class hmac_sha256_hash_provider : public hash_provider_base<hmac_sha256_hash_provider> {
                static hmac_sha256_hash_algorithm _algorithm;
            public:
                AZURE_STORAGE_API static std::string hash_impl(const std::string &input, const std::vector<unsigned char> &key);
            };
#else
            GCRY_THREAD_OPTION_PTHREAD_IMPL;
            std::string hash(const std::string &to_sign, const std::vector<unsigned char> &key);
#endif

    }
}
