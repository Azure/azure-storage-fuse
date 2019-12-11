#pragma once

#include <string>
#include <vector>

#ifdef WIN32
#include <Windows.h>
#include <bcrypt.h>
#else
#include <gnutls/gnutls.h>
#include <gnutls/crypto.h>
#define SHA256_DIGEST_LENGTH    32
#endif

#include "storage_EXPORTS.h"

namespace microsoft_azure {
    namespace storage {
        AZURE_STORAGE_API std::string hash_impl(const std::string &input, const std::vector<unsigned char> &key);
    }
}
