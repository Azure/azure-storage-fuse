#include "aznfsc.h"
#include "util.h"

#include <sys/sysmacros.h>

namespace aznfsc {

/**
 * Set readahead_kb for kernel readahead.
 * This sets the kernel readahead value of aznfsc_cfg.readahead_kb iff kernel
 * data cache is enabled and user cache is not enabled. We don't want double
 * readahead.
 */
void set_kernel_readahead()
{
    const char *mountpoint = aznfsc_cfg.mountpoint.c_str();
    const int readahead_kb = aznfsc_cfg.readahead_kb;

    if (readahead_kb < 0)
        return;

    if (!aznfsc_cfg.cache.data.kernel.enable) {
        AZLogDebug("Not setting kernel readahead_kb for {}: "
                   "cache.data.kernel.enable=false", mountpoint);
        return;
    } else if (aznfsc_cfg.cache.data.user.enable) {
        AZLogDebug("Not setting kernel readahead_kb for {}: "
                   "cache.data.user.enable=true", mountpoint);
        return;
    }

    /*
     * Do this asynchronously in a thread as we call it from init() and it
     * will cause a callback into fuse as it performs stat() of the root.
     */
    std::thread thr([=]() {
            struct stat sb;
            char sysfs_file[64];
            char readahead_kb_str[16];
            int ret, fd;

            if (::stat(mountpoint, &sb) != 0) {
                AZLogWarn("Failed to set readahead_kb for {}: stat() failed: {}",
                           mountpoint, strerror(errno));
                return;
            }

            ret = ::snprintf(sysfs_file, sizeof(sysfs_file),
                             "/sys/class/bdi/%d:%d/read_ahead_kb",
                              major(sb.st_dev), minor(sb.st_dev));
            if (ret == -1 || ret >= (int) sizeof(sysfs_file)) {
                AZLogWarn("Failed to set readahead_kb for {}: "
                          "snprintf(sysfs) failed : {}",
                          mountpoint, ret);
                return;
            }

            fd = ::open(sysfs_file, O_RDWR);
            if (fd == -1) {
                AZLogWarn("Failed to set readahead_kb for {}: "
                          "open({}) failed: {}",
                          mountpoint, sysfs_file, ::strerror(errno));
                return;
            }

            ret = ::snprintf(readahead_kb_str, sizeof(readahead_kb_str), "%d",
                             readahead_kb);
            if (ret == -1 || ret >= (int) sizeof(readahead_kb_str)) {
                ::close(fd);
                AZLogWarn("Failed to set readahead_kb for {}: "
                          "snprintf(readahead_kb) failed: {}",
                          mountpoint, ret);
                return;
            }

            if (::write(fd, readahead_kb_str,
                        ::strlen(readahead_kb_str)) == -1) {
                ::close(fd);
                AZLogWarn("Failed to set readahead_kb for {}: "
                          "write({}) failed: {}",
                          mountpoint, sysfs_file, strerror(errno));
                return;
            }

            ::close(fd);

            AZLogInfo("Set readahead_kb {} for {}",
                      readahead_kb_str, sysfs_file);
            return;
    });

    thr.detach();
}

#ifdef ENABLE_PRESSURE_POINTS
bool inject_error(double pct_prob)
{
    if (pct_prob == 0) {
        pct_prob = inject_err_prob_pct_def;
    }
    /*
     * We multiply double pct_prob with 10000, this enables us to consider
     * values as less as 0.0001% i.e., 1 in a million.
     * Anything less will result in a 0% probability.
     */
    assert(pct_prob >= 0 && pct_prob <= 100);
    const uint64_t rnd = random_number(0, 1000'000);
    return rnd < (pct_prob * 10'000);
}
#endif

}
