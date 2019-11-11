#include "hash.h"

#include "base64.h"

namespace microsoft_azure {
    namespace storage {
        std::string hash_impl(const std::string &to_sign, const std::vector<unsigned char> &key) {
            unsigned int l = SHA256_DIGEST_LENGTH;
            unsigned char digest[SHA256_DIGEST_LENGTH];

#ifdef USE_OPENSSL
#if OPENSSL_VERSION_NUMBER < 0x10100000L
            HMAC_CTX ctx;
            HMAC_CTX_init(&ctx);
            HMAC_Init_ex(&ctx, key.data(), static_cast<int>(key.size()), EVP_sha256(), NULL);
            HMAC_Update(&ctx, (const unsigned char*)to_sign.c_str(), to_sign.size());
            HMAC_Final(&ctx, digest, &l);
            HMAC_CTX_cleanup(&ctx);
#else
            HMAC_CTX * ctx = HMAC_CTX_new();
            HMAC_CTX_reset(ctx);
            HMAC_Init_ex(ctx, key.data(), key.size(), EVP_sha256(), NULL);
            HMAC_Update(ctx, (const unsigned char*)to_sign.c_str(), to_sign.size());
            HMAC_Final(ctx, digest, &l);
            HMAC_CTX_free(ctx);
#endif
#else
            gnutls_hmac_fast(GNUTLS_MAC_SHA256, key.data(), key.size(), (const unsigned char *) to_sign.data(),
                             to_sign.size(), digest);
#endif

            return to_base64(std::vector<unsigned char>(digest, digest + l));
        }
    }
}
