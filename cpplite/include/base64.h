#pragma once

#include <string>
#include <vector>

#include "storage_EXPORTS.h"

namespace azure {  namespace storage_lite {

        AZURE_STORAGE_API std::string to_base64(const std::vector<unsigned char> &input);
        AZURE_STORAGE_API std::string to_base64(const unsigned char* input, size_t size);
        AZURE_STORAGE_API std::vector<unsigned char> from_base64(const std::string &input);

}}
