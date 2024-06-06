#ifndef __AZNFSC_UTIL_H__
#define __AZNFSC_UTIL_H__

#include <string>
#include <regex>

namespace aznfsc {

static inline
bool is_valid_storageaccount(const std::string& account)
{
    const std::regex rexpr("^[a-z0-9]{3,24}$");
    return std::regex_match(account, rexpr);
}

static inline
bool is_valid_container(const std::string& container)
{
    const std::regex rexpr("^[a-z0-9](?!.*--)[a-z0-9-]{1,61}[a-z0-9]$");
    return std::regex_match(container, rexpr);
}

static inline
bool is_valid_cloud_suffix(const std::string& cloud_suffix)
{
    const std::regex rexpr("^(z[0-9]+.)?(privatelink.)?blob(.preprod)?.core.(windows.net|usgovcloudapi.net|chinacloudapi.cn)$");
    return std::regex_match(cloud_suffix, rexpr);
}

}

#endif /* __AZNFSC_UTIL_H__ */
