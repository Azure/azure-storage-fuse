#pragma once

namespace microsoft_azure {
    namespace storage {
        namespace constants {

#define DAT(x, y) extern const char *x; const int x ## _size{ sizeof(y) / sizeof(char) - 1 };
#include "constants.dat"
#undef DAT

        }
    }
}
