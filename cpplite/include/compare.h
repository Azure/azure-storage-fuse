#pragma once

#ifdef __linux__
#include <string.h>
#else
#include <cstring>
#include <string>
#endif

#ifdef _MSC_VER 
#define strncasecmp _strnicmp
#define strcasecmp _stricmp
#endif

namespace azure { namespace storage_lite {
    // Case insensitive comparator for std::map comparator
    struct case_insensitive_compare 
    {
        bool operator()(const std::string &lhs, const std::string &rhs) const 
        {
            return strcasecmp(lhs.c_str(), rhs.c_str()) < 0;
        }
    };
}} // azure::storage_lite
