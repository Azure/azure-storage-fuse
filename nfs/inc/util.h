#ifndef __AZNFSC_UTIL_H__
#define __AZNFSC_UTIL_H__

#include <sys/types.h>
#include <sys/stat.h>
#include <unistd.h>

#include <string>
#include <regex>
#include <chrono>

using namespace std::chrono;

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

/**
 * Return milliseconds since epoch.
 * Use this for timestamping.
 */
static inline
int64_t get_current_msecs()
{
    return duration_cast<milliseconds>(
            system_clock::now().time_since_epoch()).count();
}

/**
 * Compares a timespec time ts with nfstime3 time nt and returns
 * 0 if both represent the same time
 * -1 if ts < nt
 * 1 if ts > nt
 */
static inline
int compare_timespec_and_nfstime(const struct timespec& ts,
                                 const struct nfstime3& nt)
{
    const uint64_t ns1 = ts.tv_sec*1000'000'000ULL + ts.tv_nsec;
    const uint64_t ns2 = nt.seconds*1000'000'000ULL + nt.nseconds;

    if (ns1 == ns2)
        return 0;
    else if (ns1 < ns2)
        return -1;
    else
        return 1;
}

}

#endif /* __AZNFSC_UTIL_H__ */
