#pragma once

#include <cstdint>

#include "storage_EXPORTS.h"

namespace azure {  namespace storage_lite {  namespace constants {

#define DAT(x, y) extern AZURE_STORAGE_API const char *x; const int x ## _size{ sizeof(y) / sizeof(char) - 1 };
#include "constants.dat"
#undef DAT

    const uint64_t default_block_size = 8 * 1024 * 1024;
    const uint64_t max_block_size = 100 * 1024 * 1024;
    const uint64_t max_num_blocks = 50000;

}}}  // azure::storage_lite::constants
