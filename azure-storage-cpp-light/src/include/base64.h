#pragma once

#include <string>
#include <vector>

#include "storage_EXPORTS.h"

namespace microsoft_azure {
    namespace storage {

        AZURE_STORAGE_API std::string to_base64(const std::vector<unsigned char> &input);
        AZURE_STORAGE_API std::vector<unsigned char> from_base64(const std::string &input);

    }
}
