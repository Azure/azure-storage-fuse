#ifndef __AZNFSC_UTIL_H__
#define __AZNFSC_UTIL_H__

#include <sys/types.h>
#include <sys/stat.h>
#include <fcntl.h>
#include <unistd.h>
#include <errno.h>
#include <string.h>
#include <zlib.h>

#include <string>
#include <regex>
#include <chrono>
#include <random>

#include "log.h"

using namespace std::chrono;

namespace aznfsc {

/**
 * Set readahead_kb for kernel readahead.
 * This sets the kernel readahead value of aznfsc_cfg.readahead_kb iff kernel
 * data cache is enabled and user cache is not enabled. We don't want double
 * readahead.
 */
void set_kernel_readahead();

/**
 * Generate a random number in the range [min, max].
 */
static inline
uint64_t random_number(uint64_t min, uint64_t max)
{
    static std::mt19937 gen((uint64_t) std::chrono::system_clock::now().time_since_epoch().count());
    return min + (gen() % (max - min + 1));
}

static inline
bool is_valid_account(const std::string& account)
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

static inline
bool is_valid_cachedir(const std::string& cachedir)
{
    struct stat statbuf;

    if (cachedir.empty()) {
        AZLogDebug("cachedir is empty");
        return false;
    }

    if (::stat(cachedir.c_str(), &statbuf) != 0) {
        AZLogWarn("stat() failed for cachedir {}: {}",
                  cachedir, strerror(errno));
        return false;
    }

    if (!S_ISDIR(statbuf.st_mode)) {
        AZLogWarn("cachedir {} is not a directory", cachedir);
        return false;
    }

    /*
     * Creating a probe file with the same mode as the actual backing
     * files is the best way to test.
     */
    const std::string probe_file = cachedir + "/.probe";
    const int fd = ::open(probe_file.c_str(), O_CREAT|O_TRUNC|O_RDWR, 0600);

    if (fd < 0) {
        AZLogWarn("Failed to create probe file {}, cannot use cachedir {}: {}",
                  probe_file, cachedir, strerror(errno));
        return false;
    }

    return true;
}

static inline
bool is_valid_lookupcache(const std::string& lookupcache)
{
    return (lookupcache == "all" || lookupcache == "none" ||
            lookupcache == "pos" || lookupcache == "positive");
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
 * Return microseconds since epoch.
 * Use this for accurate stats.
 */
static inline
int64_t get_current_usecs()
{
    return duration_cast<microseconds>(
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

static inline
int compare_timespec(const struct timespec& ts1,
                     const struct timespec& ts2)
{
    const uint64_t ns1 = ts1.tv_sec*1000'000'000ULL + ts1.tv_nsec;
    const uint64_t ns2 = ts2.tv_sec*1000'000'000ULL + ts2.tv_nsec;

    if (ns1 == ns2)
        return 0;
    else if (ns1 < ns2)
        return -1;
    else
        return 1;
}

static inline
int compare_nfstime(const struct nfstime3& nt1,
                    const struct nfstime3& nt2)
{
    const uint64_t ns1 = nt1.seconds*1000'000'000ULL + nt1.nseconds;
    const uint64_t ns2 = nt2.seconds*1000'000'000ULL + nt2.nseconds;

    if (ns1 == ns2)
        return 0;
    else if (ns1 < ns2)
        return -1;
    else
        return 1;
}

static inline
uint32_t calculate_crc32(const unsigned char *buf, int len)
{
    return ::crc32(::crc32(0L, Z_NULL, 0), buf, len);
}

/**
 * Inject error with given probability percentage.
 * f.e., pct_prob=0.1 would cause inject_error() to return true for 0.1% of
 * the calls, i.e., 1 in 1000.
 * Environment variable AZNFSC_INJECT_ERROR_PERCENT can be used to set the
 * default value of pct_prob, if caller doesn't pass an explicit value.
 *
 * Note: Inject errors with caution. Only inject errors which can be fixed by
 *       retries and do not result in application failures.
 */
bool inject_error(double pct_prob = 0);

}

#endif /* __AZNFSC_UTIL_H__ */
