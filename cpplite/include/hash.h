#pragma once

#include <string>
#include <vector>

#define SHA256_DIGEST_LENGTH    32

#include "storage_EXPORTS.h"

namespace azure {  namespace storage_lite {
    AZURE_STORAGE_API std::string hash(const std::string &to_sign, const std::vector<unsigned char> &key);
}}  // azure::storage_lite
