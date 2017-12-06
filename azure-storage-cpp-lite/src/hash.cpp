#include "hash.h"

#include "base64.h"

namespace microsoft_azure {
    namespace storage {
#ifdef WIN32
        hmac_sha256_hash_algorithm hmac_sha256_hash_provider::_algorithm;

        std::string hmac_sha256_hash_provider::hash_impl(const std::string &input, const std::vector<unsigned char> &key) {
            ULONG object_size = 0;
            ULONG size_length = 0;

            NTSTATUS status = BCryptGetProperty(_algorithm.handle(), BCRYPT_OBJECT_LENGTH, (PUCHAR)&object_size, sizeof(ULONG), &size_length, 0);
            if (status != 0) {
                throw std::system_error(status, std::system_category());
            }
            std::vector<UCHAR> hash_object(object_size);

            BCRYPT_HASH_HANDLE hash_handle;
            status = BCryptCreateHash(_algorithm.handle(), &hash_handle, (PUCHAR)hash_object.data(), (ULONG)hash_object.size(), (PUCHAR)key.data(), (ULONG)key.size(), 0);
            if (status != 0) {
                throw std::system_error(status, std::system_category());
            }

            status = BCryptHashData(hash_handle, (PUCHAR)input.data(), (ULONG)input.size(), 0);
            if (status != 0) {
                throw std::system_error(status, std::system_category());
            }

            status = BCryptGetProperty(hash_handle, BCRYPT_HASH_LENGTH, (PUCHAR)&object_size, sizeof(ULONG), &size_length, 0);
            if (status != 0) {
                throw std::system_error(status, std::system_category());
            }
            std::vector<UCHAR> hash(object_size);

            status = BCryptFinishHash(hash_handle, hash.data(), (ULONG)hash.size(), 0);
            if (status != 0 && status != 0xc0000008) {
                throw std::system_error(status, std::system_category());
            }

            status = BCryptDestroyHash(hash_handle);
            return to_base64(hash);
        }
#else
        std::string hash(const std::string &to_sign, const std::vector<unsigned char> &key) {
            // For proper locking, instructing gcrypt to use pthreads 
            gcry_control(GCRYCTL_SET_THREAD_CBS, &gcry_threads_pthread);
            gnutls_global_init(); 
            unsigned int l = SHA256_DIGEST_LENGTH;
            unsigned char digest[SHA256_DIGEST_LENGTH];
            gnutls_hmac_fast(GNUTLS_MAC_SHA256,key.data(), key.size(),(const unsigned char *)to_sign.data(), to_sign.size(), digest);
            gnutls_global_deinit();
            return to_base64(std::vector<unsigned char>(digest, digest + l));

        }
#endif

    }
}
