#include "constants.h"

namespace azure {  namespace storage_lite {  namespace constants {

#define DAT(x, y) const char *x{ y };
#include "constants.dat"
#undef DAT

}}}  // azure::storage_lite
