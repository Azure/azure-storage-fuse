#include "constants.h"

namespace microsoft_azure {
    namespace storage {
        namespace constants {

#define DAT(x, y) const char *x{ y };
#include "constants.dat"
#undef DAT

        }
    }
}
