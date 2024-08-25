#include "aznfsc.h"
#include "nfs_client.h"
#include "nfs_internal.h"
#include "rpc_stats.h"
#include "file_cache.h"
#include "yaml-cpp/yaml.h"

#include <sys/sysmacros.h>
#include <signal.h>
#include <thread>

using namespace std;

/*
 * Global aznfsc config instance holding all the aznfs client configuration.
 */
struct aznfsc_cfg aznfsc_cfg;

/**
 * This holds the global options for the fuse like max_write, max_readahead etc,
 * passed from command line.
 */
struct fuse_conn_info_opts* fuse_conn_info_opts_ptr;

/*
 * These are aznfsclient specific options.
 * These can be passed to aznfsclient fuse program, in addition to the standard
 * fuse options.
 */
#define AZNFSC_OPT(templ, key) { templ, offsetof(struct aznfsc_cfg, key), 0}

static const struct fuse_opt aznfsc_opts[] =
{
    AZNFSC_OPT("--config-file=%s", config_yaml),
    AZNFSC_OPT("--account=%s", account),
    AZNFSC_OPT("--container=%s", container),
    AZNFSC_OPT("--cloud-suffix=%s", cloud_suffix),
    AZNFSC_OPT("--port=%u", port),
    AZNFSC_OPT("--nconnect=%u", nconnect),
    FUSE_OPT_END
};

void aznfsc_help(const char *argv0)
{
    printf("usage: %s [options] <mountpoint>\n\n", argv0);
    printf("    --config-file=<config.yaml file path>\n");
    printf("    --account=<storage account>\n");
    printf("    --container=<container>\n");
    printf("    --cloud-suffix=<cloud suffix>\n");
    printf("    --port=<Blob NFS port, can be 2048 or 2047>\n");
    printf("    --nconnect=<number of simultaneous connections>\n");
}

/*
 * This function parses the contents of the yaml config file denoted by path
 * config_file into the aznfsc_cfg structure.
 */
bool aznfsc_cfg::parse_config_yaml()
{

#define __CHECK_INT(var, min, max, zeroisvalid) \
do { \
    if (((var) == -1) && config[#var]) { \
        (var) = config[#var].as<int>(); \
        if ((var) < min || (var) > max) { \
            if ((zeroisvalid) && ((var) == 0)) { \
                break; \
            } \
            throw YAML::Exception( \
                    config[#var].Mark(), \
                    std::string("Invalid value for config "#var": ") + \
                    std::to_string(var) + \
                    std::string(" (valid range [") + \
                    std::to_string(min) + ", " + std::to_string(max) + "])"); \
        }\
    } \
} while (0);

/*
 * Macro to check validity of config var of integer type.
 */
#define _CHECK_INT(var, min, max) __CHECK_INT(var, min, max, false)

/*
 * Macro to check validity of config var of integer type with 0 being a
 * valid value.
 */
#define _CHECK_INTZ(var, min, max) __CHECK_INT(var, min, max, true)

/*
 * Macro to check validity of config var of boolean type.
 */
#define _CHECK_BOOL(var) \
do { \
    if (config[#var]) { \
        (var) = config[#var].as<bool>(); \
    } \
} while (0);


/*
 * Macro to check validity of config var of string type.
 */
#define __CHECK_STR(var, is_valid, ignore_empty) \
do { \
    if (((var) == nullptr) && config[#var]) { \
        /* Empty key is returned as "null" by the yaml parser */ \
        if ((ignore_empty) && config[#var].as<std::string>() == "null") { \
            break; \
        } \
        (var) = ::strdup(config[#var].as<std::string>().c_str()); \
        if (!is_valid(var)) { \
            throw YAML::Exception( \
                    config[#var].Mark(), \
                    std::string("Invalid value for config "#var": ") + \
                    std::string(var)); \
        } \
    } \
} while (0);

#define _CHECK_STR(var) __CHECK_STR(var, is_valid_##var, false)
#define _CHECK_STR2(var, is_valid) __CHECK_STR(var, is_valid, false)

    if (config_yaml == nullptr) {
        return true;
    }

    AZLogDebug("Parsing config yaml {}", config_yaml);

    /*
     * We parse the config yaml and set *only* those options which are not yet
     * set by cmdline. Thus cmdline options are given higher priority than the
     * corresponding option in the config yaml.
     */
    try {
        YAML::Node config = YAML::LoadFile(config_yaml);

        _CHECK_STR(account);
        _CHECK_STR(container);
        _CHECK_STR(cloud_suffix);

        if ((port == -1) && config["port"]) {
            port = config["port"].as<int>();
#ifndef ENABLE_NON_AZURE_NFS
            if (port != 2048 && port != 2047) {
                throw YAML::Exception(
                    config["port"].Mark(),
                    std::string("Invalid port number: ") +
                    std::to_string(port) +
                    std::string(" (can be 2048 or 2047)"));
            }
#endif
        }

        _CHECK_INT(nconnect, AZNFSCFG_NCONNECT_MIN, AZNFSCFG_NCONNECT_MAX);
        _CHECK_INT(timeo, AZNFSCFG_TIMEO_MIN, AZNFSCFG_TIMEO_MAX);
        _CHECK_INT(acregmin, AZNFSCFG_ACTIMEO_MIN, AZNFSCFG_ACTIMEO_MAX);
        _CHECK_INT(acregmax, AZNFSCFG_ACTIMEO_MIN, AZNFSCFG_ACTIMEO_MAX);
        _CHECK_INT(acdirmin, AZNFSCFG_ACTIMEO_MIN, AZNFSCFG_ACTIMEO_MAX);
        _CHECK_INT(acdirmax, AZNFSCFG_ACTIMEO_MIN, AZNFSCFG_ACTIMEO_MAX);
        _CHECK_INT(actimeo, AZNFSCFG_ACTIMEO_MIN, AZNFSCFG_ACTIMEO_MAX);
        _CHECK_STR(lookupcache);
        _CHECK_INT(rsize, AZNFSCFG_RSIZE_MIN, AZNFSCFG_RSIZE_MAX);
        _CHECK_INT(wsize, AZNFSCFG_WSIZE_MIN, AZNFSCFG_WSIZE_MAX);
        _CHECK_INT(retrans, AZNFSCFG_RETRANS_MIN, AZNFSCFG_RETRANS_MAX);
        _CHECK_INT(readdir_maxcount, AZNFSCFG_READDIR_MIN, AZNFSCFG_READDIR_MAX);
        /*
         * Allow special value of 0 to disable readahead.
         * Mostly useful for testing.
         */
        _CHECK_INTZ(readahead_kb, AZNFSCFG_READAHEAD_KB_MIN, AZNFSCFG_READAHEAD_KB_MAX);
        _CHECK_INT(fuse_max_background, AZNFSCFG_FUSE_MAX_BG_MIN, AZNFSCFG_FUSE_MAX_BG_MAX);

        _CHECK_BOOL(cache.readdir.kernel.enable);
        _CHECK_BOOL(cache.readdir.user.enable);
        _CHECK_BOOL(cache.data.kernel.enable);

        _CHECK_BOOL(cache.data.user.enable);
        if (cache.data.user.enable) {
            _CHECK_INT(cache.data.user.max_size_mb,
                       AZNFSCFG_CACHE_MAX_MB_MIN, AZNFSCFG_CACHE_MAX_MB_MAX);
        }

        _CHECK_BOOL(filecache.enable);
        if (filecache.enable) {
            _CHECK_STR2(filecache.cachedir, is_valid_cachedir);
            _CHECK_INT(filecache.max_size_gb, AZNFSCFG_FILECACHE_MAX_GB_MIN, AZNFSCFG_FILECACHE_MAX_GB_MAX);
        }

    } catch (const YAML::BadFile& e) {
        AZLogError("Error loading config file {}: {}", config_yaml, e.what());
        return false;
    } catch (const YAML::Exception& e) {
        AZLogError("Error parsing config file {}: {}", config_yaml, e.what());
        return false;
    } catch (...) {
        AZLogError("Unknown error parsing config file {}", config_yaml);
        return false;
    }

    return true;
}

/**
 * Set default values for options not yet assigned.
 * This must be called after fuse_opt_parse() and parse_config_yaml()
 * assign config values from command line and the config yaml file.
 * Also sanitizes various values.
 */
void aznfsc_cfg::set_defaults_and_sanitize()
{
    if (port == -1)
        port = 2048;
    if (nconnect == -1)
        nconnect = 1;
    if (rsize == -1)
        rsize = 1048576;
    if (wsize == -1)
        wsize = 1048576;
    if (retrans == -1)
        retrans = 3;
    if (timeo == -1)
        timeo = 600;
    if (acregmin == -1)
        acregmin = 3;
    if (acregmax == -1)
        acregmax = 60;
    if (acdirmin == -1)
        acdirmin = 30;
    if (acdirmax == -1)
        acdirmax = 60;
    if (actimeo != -1) {
        acregmin = acregmax = acdirmin = acdirmax = actimeo;
    } else {
        /*
         * This is used only by nfs_client::reply_entry() for setting the
         * timeout of negative lookup result.
         * Rest everywhere we will use ac{reg|dir}{min|max}.
         */
        actimeo = AZNFSCFG_ACTIMEO_MIN;
    }
    if (acregmin > acregmax)
        acregmin = acregmax;
    if (acdirmin > acdirmax)
        acdirmin = acdirmax;

    if (std::string(lookupcache) == "all") {
        lookupcache_int = AZNFSCFG_LOOKUPCACHE_ALL;
    } else if (std::string(lookupcache) == "none") {
        lookupcache_int = AZNFSCFG_LOOKUPCACHE_NONE;
    } else if (std::string(lookupcache) == "pos" ||
            std::string(lookupcache) == "positive") {
        lookupcache_int = AZNFSCFG_LOOKUPCACHE_POS;
    } else {
        lookupcache_int = AZNFSCFG_LOOKUPCACHE_DEF;
    }

    if (readdir_maxcount == -1)
        readdir_maxcount = 1048576;
    if (readahead_kb == -1)
        readahead_kb = 16384;
    if (cache.data.user.enable) {
        if (cache.data.user.max_size_mb == -1)
            cache.data.user.max_size_mb = AZNFSCFG_CACHE_MAX_MB_DEF;
    }
    if (filecache.enable) {
        if (filecache.max_size_gb == -1)
            filecache.max_size_gb = AZNFSCFG_FILECACHE_MAX_GB_DEF;
    }
    if (cloud_suffix == nullptr)
        cloud_suffix = ::strdup("blob.core.windows.net");

    assert(account != nullptr);
    assert(container != nullptr);

    // Set aggregates.
    server = std::string(account) + "." + std::string(cloud_suffix);
    export_path = "/" + std::string(account) + "/" + std::string(container);

    // Dump the final config values for debugging.
    AZLogDebug("===== config start =====");
    AZLogDebug("port = {}", port);
    AZLogDebug("nconnect = {}", nconnect);
    AZLogDebug("rsize = {}", rsize);
    AZLogDebug("wsize = {}", wsize);
    AZLogDebug("retrans = {}", retrans);
    AZLogDebug("timeo = {}", timeo);
    AZLogDebug("acregmin = {}", acregmin);
    AZLogDebug("acregmax = {}", acregmax);
    AZLogDebug("acdirmin = {}", acdirmin);
    AZLogDebug("acdirmax = {}", acdirmax);
    AZLogDebug("actimeo = {}", actimeo);
    AZLogDebug("lookupcache = {}", lookupcache);
    AZLogDebug("readdir_maxcount = {}", readdir_maxcount);
    AZLogDebug("readahead_kb = {}", readahead_kb);
    AZLogDebug("fuse_max_background = {}", fuse_max_background);
    AZLogDebug("cache.readdir.kernel.enable = {}", cache.readdir.kernel.enable);
    AZLogDebug("cache.readdir.user.enable = {}", cache.readdir.user.enable);
    AZLogDebug("cache.data.kernel.enable = {}", cache.data.kernel.enable);
    AZLogDebug("cache.data.user.enable = {}", cache.data.user.enable);
    AZLogDebug("cache.data.user.max_size_mb = {}", cache.data.user.max_size_mb);
    AZLogDebug("filecache.enable = {}", filecache.enable);
    AZLogDebug("filecache.cachedir = {}", filecache.cachedir ? filecache.cachedir : "");
    AZLogDebug("filecache.max_size_gb = {}", filecache.max_size_gb);
    AZLogDebug("account = {}", account);
    AZLogDebug("container = {}", container);
    AZLogDebug("cloud_suffix = {}", cloud_suffix);
    AZLogDebug("===== config end =====");
}

/**
 * Set readahead_kb for kernel readahead.
 * This sets the kernel readahead value of aznfsc_cfg.readahead_kb iff kernel
 * data cache is enabled and user cache is not enabled. We don't want double
 * readahead.
 */
static void set_kernel_readahead()
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

static void aznfsc_ll_init(void *userdata,
                           struct fuse_conn_info *conn)
{
    /*
     * TODO: Kernel conveys us the various filesystem limits by passing the
     *       fuse_conn_info pointer. If we need to reduce any of the limits
     *       we can do so. Usually we may not be interested in reducing any
     *       of those limits.
     *       We can at least log from here so that we know the limits.
     */

    /*
     * Apply the user passed options (-o). This must be done before
     * the overrides we have below. This is because those overrides are
     * our limitation and we cannot let user bypass them.
     *
     * Note: fuse_session_new() no longer accepts arguments
     *       command line options can only be set using
     *       fuse_apply_conn_info_opts().
     */
    fuse_apply_conn_info_opts(fuse_conn_info_opts_ptr, conn);

    /*
     * XXX Disable readdir temporarily while I work on fixing readdirplus.
     *     Once readdirplus is audited/fixed, enable readdir and audit/fix
     *     that.
     * TODO: Readdir works fine but just that for readdir fuse kernel
     *       will not send FORGET and thus we currently don't delete those
     *       entries and the inodes. Need to add memory pressure based
     *       deletion for those.
     */
    conn->want |= FUSE_CAP_READDIRPLUS;
    conn->want |= FUSE_CAP_READDIRPLUS_AUTO;

    /*
     * Fuse kernel driver must issue parallel readahead requests.
     */
    conn->want |= FUSE_CAP_ASYNC_READ;

    // Blob NFS doesn't support locking.
    conn->want &= ~FUSE_CAP_POSIX_LOCKS;
    conn->want &= ~FUSE_CAP_FLOCK_LOCKS;

    // TODO: See if we can support O_TRUNC.
    conn->want &= ~FUSE_CAP_ATOMIC_O_TRUNC;

    /*
     * For availing perf advantage of splice() we must add splice()/sendfile()
     * support to libnfs. Till then just disable splicing so fuse never sends
     * us fd+offset but just a plain buffer.
     * Test splice read/write performance before enabling.
     */
    conn->want &= ~FUSE_CAP_SPLICE_WRITE;
    conn->want &= ~FUSE_CAP_SPLICE_MOVE;
    conn->want &= ~FUSE_CAP_SPLICE_READ;

    conn->want |= FUSE_CAP_AUTO_INVAL_DATA;
    conn->want |= FUSE_CAP_ASYNC_DIO;

    if (aznfsc_cfg.cache.data.kernel.enable) {
        conn->want |= FUSE_CAP_WRITEBACK_CACHE;
    } else {
        conn->want &= ~FUSE_CAP_WRITEBACK_CACHE;
    }

    conn->want |= FUSE_CAP_PARALLEL_DIROPS;
    conn->want &= ~FUSE_CAP_POSIX_ACL;

    // TODO: See if we should enable this.
    conn->want &= ~FUSE_CAP_CACHE_SYMLINKS;
    conn->want &= ~FUSE_CAP_SETXATTR_EXT;

#if 0
    /*
     * Fuse wants max_read set here to match the mount option passed
     * -o max_read=<n>
     */
    if (conn->max_read) {
        conn->max_read =
            std::min<unsigned int>(conn->max_read, AZNFSC_MAX_CHUNK_SIZE);
    } else {
        conn->max_read = AZNFSC_MAX_CHUNK_SIZE;
    }

    if (conn->max_readahead) {
        conn->max_readahead =
            std::min<unsigned int>(conn->max_readahead, AZNFSC_MAX_CHUNK_SIZE);
    } else {
        conn->max_readahead = AZNFSC_MAX_CHUNK_SIZE;
    }
    if (conn->max_write) {
        conn->max_write =
            std::min<unsigned int>(conn->max_write, AZNFSC_MAX_CHUNK_SIZE);
    } else {
        conn->max_write = AZNFSC_MAX_CHUNK_SIZE;
    }
#endif

    /*
     * If user has explicitly specified "-o max_background=", honour that,
     * else if he has specified fuse_max_background config, use that, else
     * pick a good default.
     */
    if (conn->max_background == 0) {
        if (aznfsc_cfg.fuse_max_background != -1) {
            conn->max_background = aznfsc_cfg.fuse_max_background;
        } else {
            conn->max_background = AZNFSCFG_FUSE_MAX_BG_DEF;
        }
    }

    /*
     * Set kernel readahead_kb if kernel data cache is enabled.
     */
    set_kernel_readahead();

    AZLogDebug("===== fuse_conn_info fields start =====");
    AZLogDebug("proto_major = {}", conn->proto_major);
    AZLogDebug("proto_minor = {}", conn->proto_minor);
    AZLogDebug("max_write = {}", conn->max_write);
    AZLogDebug("max_read = {}", conn->max_read);
    AZLogDebug("max_readahead = {}", conn->max_readahead);
    AZLogDebug("capable = 0x{:x}", conn->capable);
    AZLogDebug("want = 0x{:x}", conn->want);
    AZLogDebug("max_background = {}", conn->max_background);
    AZLogDebug("congestion_threshold = {}", conn->congestion_threshold);
    AZLogDebug("time_gran = {}", conn->time_gran);
    AZLogDebug("===== fuse_conn_info fields end =====");
}

static void aznfsc_ll_destroy(void *userdata)
{
    /*
     * TODO: Again, we can just log from here or any cleanup we want to do
     *       when a fuse nfs filesystem is unmounted. Note that connection to
     *       the kernel may be gone by the time this is called so we cannot
     *       make any call that calls into kernel.
     */
}

static void aznfsc_ll_lookup(fuse_req_t req,
                             fuse_ino_t parent_ino,
                             const char *name)
{
    AZLogDebug("aznfsc_ll_lookup(req={}, parent_ino={}, name={})",
               fmt::ptr(req), parent_ino, name);

    struct nfs_client *client = get_nfs_client_from_fuse_req(req);
    client->lookup(req, parent_ino, name);
}

static std::atomic<uint64_t> total_forgotten = 0;

static void aznfsc_ll_forget(fuse_req_t req,
                             fuse_ino_t ino,
                             uint64_t nlookup)
{
    total_forgotten++;

    AZLogDebug("aznfsc_ll_forget(req={}, ino={}, nlookup={}) "
               "total_forgotten={}",
               fmt::ptr(req), ino, nlookup, total_forgotten.load());

    struct nfs_client *client = get_nfs_client_from_fuse_req(req);
    struct nfs_inode *inode = client->get_nfs_inode_from_ino(ino);

    /*
     * Decrement refcnt of the inode and free the inode if refcnt becomes 0.
     */
    inode->decref(nlookup, true /* from_forget */);
    fuse_reply_none(req);
}

static void aznfsc_ll_getattr(fuse_req_t req,
                              fuse_ino_t ino,
                              struct fuse_file_info *fi)
{
    AZLogDebug("aznfsc_ll_getattr(req={}, ino={}, fi={})",
               fmt::ptr(req), ino, fmt::ptr(fi));

    struct nfs_client *client = get_nfs_client_from_fuse_req(req);
    client->getattr(req, ino, fi);
}

static void aznfsc_ll_setattr(fuse_req_t req,
                              fuse_ino_t ino,
                              struct stat *attr,
                              int to_set /* bitmask indicating the attributes to set */,
                              struct fuse_file_info *fi)
{
    // TODO: Log all to-be-set attributes.
    AZLogDebug("aznfsc_ll_setattr(req={}, ino={}, to_set=0x{:x}, fi={})",
               fmt::ptr(req), ino, to_set, fmt::ptr(fi));

    struct nfs_client *client = get_nfs_client_from_fuse_req(req);
    client->setattr(req, ino, attr, to_set, fi);
}

static void aznfsc_ll_readlink(fuse_req_t req,
                               fuse_ino_t ino)
{
    AZLogDebug("aznfsc_ll_readlink(req={}, ino={})",
               fmt::ptr(req), ino);

    struct nfs_client *client = get_nfs_client_from_fuse_req(req);
    client->readlink(req, ino);
}

static void aznfsc_ll_mknod(fuse_req_t req,
                            fuse_ino_t parent_ino,
                            const char *name,
                            mode_t mode,
                            dev_t rdev)
{
    /*
     * TODO: Fill me.
     */
    fuse_reply_err(req, ENOSYS);
}

static void aznfsc_ll_mkdir(fuse_req_t req,
                            fuse_ino_t parent_ino,
                            const char *name,
                            mode_t mode)
{
    AZLogDebug("aznfsc_ll_mkdir(req={}, parent_ino={}, name={}, mode=0{:03o})",
               fmt::ptr(req), parent_ino, name, mode);

    struct nfs_client *client = get_nfs_client_from_fuse_req(req);
    client->mkdir(req, parent_ino, name, mode);
}

static void aznfsc_ll_unlink(fuse_req_t req,
                             fuse_ino_t parent_ino,
                             const char *name)
{
    AZLogDebug("aznfsc_ll_unlink(req={}, parent_ino={}, name={})",
               fmt::ptr(req), parent_ino, name);

    struct nfs_client *client = get_nfs_client_from_fuse_req(req);
    client->unlink(req, parent_ino, name);
}

static void aznfsc_ll_rmdir(fuse_req_t req,
                            fuse_ino_t parent_ino,
                            const char *name)
{
    AZLogDebug("aznfsc_ll_rmdir(req={}, parent_ino={}, name={})",
               fmt::ptr(req), parent_ino, name);

    struct nfs_client *client = get_nfs_client_from_fuse_req(req);
    client->rmdir(req, parent_ino, name);
}

static void aznfsc_ll_symlink(fuse_req_t req,
                              const char *link,
                              fuse_ino_t parent_ino,
                              const char *name)
{
    AZLogDebug("aznfsc_ll_symlink(req={}, link={}, parent_ino={}, name={})",
               fmt::ptr(req), link, parent_ino, name);

    struct nfs_client *client = get_nfs_client_from_fuse_req(req);
    client->symlink(req, link, parent_ino, name);
}

static void aznfsc_ll_rename(fuse_req_t req,
                             fuse_ino_t parent_ino,
                             const char *name,
                             fuse_ino_t newparent_ino,
                             const char *newname,
                             unsigned int flags)
{
    AZLogDebug("aznfsc_ll_rename(req={}, parent_ino={}, name={},"
               " newparent_ino={}, newname={}, flags={})",
               fmt::ptr(req), parent_ino, name,
               newparent_ino, newname, flags);

    // We don't support `RENAME_EXCHANGE` or `RENAME_NOREPLACE` flags for now.
    assert(flags == 0);

    struct nfs_client *client = get_nfs_client_from_fuse_req(req);
    client->rename(req, parent_ino, name, newparent_ino, newname, flags);
}

static void aznfsc_ll_link(fuse_req_t req,
                           fuse_ino_t ino,
                           fuse_ino_t newparent_ino,
                           const char *newname)
{
    /*
     * TODO: Fill me.
     */
    fuse_reply_err(req, ENOSYS);
}

static void aznfsc_ll_open(fuse_req_t req,
                           fuse_ino_t ino,
                           struct fuse_file_info *fi)
{
    AZLogInfo("aznfsc_ll_open(req={}, ino={}, fi={})",
              fmt::ptr(req), ino, fmt::ptr(fi));

    /*
     * We plan to manage our own file cache for better control over writes.
     *
     * Note: We don't need to set these explicitly as they default to
     *       these values, we do it to highlight our intent.
     *
     * TODO: Explore kernel caching, its benefits and side-effects.
     */
    fi->direct_io = !aznfsc_cfg.cache.data.kernel.enable;
    fi->keep_cache = aznfsc_cfg.cache.data.kernel.enable;
    fi->nonseekable = 0;
    fi->parallel_direct_writes = 1;
    fi->noflush = 0;

    struct nfs_client *client = get_nfs_client_from_fuse_req(req);
    struct nfs_inode *inode = client->get_nfs_inode_from_ino(ino);

    /*
     * TODO: See comments in readahead.h, ideally readahead state should be
     *       per file pointer (per open handle) but since fuse doesn't let
     *       us know the file pointer we maintain readahead state per inode.
     *       We must reset the readahead state so that this file handle can
     *       correctly perform readaheads and not confuse this as an access
     *       using the prev handle. This means if we open more than one handle
     *       simultaneously it will cause the readahead state to be reset.
     *
     *       This is a hack and needs to be properly addressed!
     */
    if (inode->is_regfile() && !inode->is_cache_empty()) {
        AZLogInfo("[{}] Clearing cache", ino);
        inode->filecache_handle->clear();
    }

    if (inode->is_regfile() && inode->readahead_state) {
        AZLogInfo("[{}] Resetting readahead state", ino);
        inode->readahead_state->reset();
    }

    /*
     * If file cache is not allocated, allocate now.
     * Mostly it'll be allocated in nfs_client::reply_entry().
     */
    if (inode->is_regfile()) {
        inode->get_or_alloc_filecache();
    } else if (inode->is_dir()) {
        inode->get_or_alloc_dircache();
    }

    fuse_reply_open(req, fi);
}

static void aznfsc_ll_read(fuse_req_t req,
                           fuse_ino_t ino,
                           size_t size,
                           off_t off,
                           struct fuse_file_info *fi)
{
    AZLogDebug("aznfsc_ll_read(req={}, ino={}, size={}, offset={} fi={}",
                fmt::ptr(req), ino, size, off, fmt::ptr(fi));

    /*
     * Sanity assert. 1MiB is the max read size fuse will ever issue.
     * If fuse sends more we'd like to know.
     *
     * TODO: Remove this before going to production.
     */
    assert(size <= 1048576);

    struct nfs_client *client = get_nfs_client_from_fuse_req(req);
    client->read(req, ino, size, off, fi);
}

static void aznfsc_ll_write(fuse_req_t req,
                            fuse_ino_t ino,
                            const char *buf,
                            size_t size,
                            off_t off,
                            struct fuse_file_info *fi)
{
    /*
     * XXX: write will be never called as we implement write_buf.
     */
    AZLogError("aznfsc_ll_write(req={}, ino={}, buf={}, size={}, off={}, fi={})",
               fmt::ptr(req), ino, fmt::ptr(buf), size, off, fmt::ptr(fi));

    fuse_reply_err(req, ENOSYS);
}

static void aznfsc_ll_flush(fuse_req_t req,
                            fuse_ino_t ino,
                            struct fuse_file_info *fi)
{
    AZLogDebug("aznfsc_ll_flush(req={}, ino={}, fi={})",
               fmt::ptr(req), ino, fmt::ptr(fi));

    struct nfs_client *client = get_nfs_client_from_fuse_req(req);
    client->flush(req, ino);
}

static void aznfsc_ll_release(fuse_req_t req,
                              fuse_ino_t ino,
                              struct fuse_file_info *fi)
{
    /*
     * Fuse calls flush() for every fd closed and release() once per file,
     * when the last fd to that file is closed.
     * Though we shouldn't need the flush here but for safety we put it
     * here as fuse doc says flush()) may not be called.
     */
    AZLogDebug("aznfsc_ll_release(req={}, ino={}, fi={})",
               fmt::ptr(req), ino, fmt::ptr(fi));

    struct nfs_client *client = get_nfs_client_from_fuse_req(req);
    client->flush(req, ino);
}

static void aznfsc_ll_fsync(fuse_req_t req,
                            fuse_ino_t ino,
                            int datasync,
                            struct fuse_file_info *fi)
{
    /*
     * TODO: Fill me.
     */
    fuse_reply_err(req, ENOSYS);
}

static void aznfsc_ll_opendir(fuse_req_t req,
                              fuse_ino_t ino,
                              struct fuse_file_info *fi)
{
    AZLogDebug("aznfsc_ll_opendir(req={}, ino={}, fi={})",
               fmt::ptr(req), ino, fmt::ptr(fi));

    /*
     * We manage our own readdir cache and we don't want kernel to
     * cache directory contents.
     *
     * Note: We don't need to set these explicitly as they default to
     *       these values, we do it to highlight our intent.
     * TODO: Later explore if kernel cacheing directory content is beneficial
     *       and what are the side effects, if any.
     */
    fi->direct_io = !aznfsc_cfg.cache.readdir.kernel.enable;
    fi->keep_cache = 1;
    fi->nonseekable = 0;
    fi->cache_readdir = aznfsc_cfg.cache.readdir.kernel.enable;
    fi->noflush = 0;

    struct nfs_client *client = get_nfs_client_from_fuse_req(req);
    struct nfs_inode *inode = client->get_nfs_inode_from_ino(ino);

    assert(inode->is_dir());

    inode->get_or_alloc_dircache();
    fuse_reply_open(req, fi);
}

static void aznfsc_ll_readdir(fuse_req_t req,
                              fuse_ino_t ino,
                              size_t size,
                              off_t off,
                              struct fuse_file_info *fi)
{
    AZLogDebug("aznfsc_ll_readdir(req={}, ino={}, size={}, off={}, fi={})",
               fmt::ptr(req), ino, size, off, fmt::ptr(fi));

    struct nfs_client *client = get_nfs_client_from_fuse_req(req);
    client->readdir(req, ino, size, off, fi);
}

static void aznfsc_ll_releasedir(fuse_req_t req,
                                 fuse_ino_t ino,
                                 struct fuse_file_info *fi)
{
    AZLogDebug("aznfsc_ll_releasedir(req={}, ino={}, fi={})",
               fmt::ptr(req), ino, fmt::ptr(fi));

    /*
     * We don't do anything in opendir() so nothing to be done in
     * releasedir().
     *
     * TODO: See if we want to flush the directory buffer to create
     *       space. This may be helpful for find(1)workloads which
     *       traverse a directory just once.
     */

     fuse_reply_err(req, 0);
}

static void aznfsc_ll_fsyncdir(fuse_req_t req,
                               fuse_ino_t ino,
                               int datasync,
                               struct fuse_file_info *fi)
{
    /*
     * TODO: Fill me.
     */
    fuse_reply_err(req, ENOSYS);
}

static void aznfsc_ll_statfs(fuse_req_t req,
                             fuse_ino_t ino)
{

    /*
     * TODO: Fill me.
     */
    fuse_reply_err(req, ENOSYS);
}

static void aznfsc_ll_setxattr(fuse_req_t req,
                               fuse_ino_t ino,
                               const char *name,
                               const char *value,
                               size_t size,
                               int flags)
{
    /*
     * TODO: Fill me.
     */
    fuse_reply_err(req, ENOSYS);
}

static void aznfsc_ll_getxattr(fuse_req_t req,
                               fuse_ino_t ino,
                               const char *name,
                               size_t size)
{
    /*
     * TODO: Fill me.
     */
    fuse_reply_err(req, ENOSYS);
}

static void aznfsc_ll_listxattr(fuse_req_t req,
                                fuse_ino_t ino,
                                size_t size)
{
    /*
     * TODO: Fill me.
     */
    fuse_reply_err(req, ENOSYS);
}

static void aznfsc_ll_removexattr(fuse_req_t req,
                                  fuse_ino_t ino,
                                  const char *name)
{
    /*
     * TODO: Fill me.
     */
    fuse_reply_err(req, ENOSYS);
}

static void aznfsc_ll_access(fuse_req_t req,
                             fuse_ino_t ino,
                             int mask)
{
    /*
     * TODO: Fill me.
     */
    AZLogInfo("aznfsc_ll_access(req={}, ino={}, mask={:x}",
              fmt::ptr(req), ino, mask);
    fuse_reply_err(req, ENOSYS);
}

static void aznfsc_ll_create(fuse_req_t req,
                             fuse_ino_t parent_ino,
                             const char *name,
                             mode_t mode,
                             struct fuse_file_info *fi)
{
    AZLogDebug("aznfsc_ll_create(req={}, parent_ino={}, name={}, "
               "mode=0{:03o}, fi={})",
               fmt::ptr(req), parent_ino, name, mode, fmt::ptr(fi));

    struct nfs_client *client = get_nfs_client_from_fuse_req(req);
    client->create(req, parent_ino, name, mode, fi);
}

static void aznfsc_ll_getlk(fuse_req_t req,
                            fuse_ino_t ino,
                            struct fuse_file_info *fi,
                            struct flock *lock)
{
    /*
     * TODO: Fill me.
     */
    fuse_reply_err(req, ENOSYS);
}

static void aznfsc_ll_setlk(fuse_req_t req,
                            fuse_ino_t ino,
                            struct fuse_file_info *fi,
                            struct flock *lock,
                            int sleep)
{
    /*
     * TODO: Fill me.
     */
    fuse_reply_err(req, ENOSYS);
}

static void aznfsc_ll_bmap(fuse_req_t req,
                           fuse_ino_t ino,
                           size_t blocksize,
                           uint64_t idx)
{
    /*
     * TODO: Fill me.
     */
    fuse_reply_err(req, ENOSYS);
}

#if FUSE_USE_VERSION < 35
static void aznfsc_ll_ioctl(fuse_req_t req,
                            fuse_ino_t ino,
                            int cmd,
                            void *arg,
                            struct fuse_file_info *fi,
                            unsigned flags,
                            const void *in_buf,
                            size_t in_bufsz,
                            size_t out_bufsz)
{
    /*
     * TODO: Fill me.
     */
    fuse_reply_err(req, ENOSYS);
}
#else
static void aznfsc_ll_ioctl(fuse_req_t req,
                            fuse_ino_t ino,
                            unsigned int cmd,
                            void *arg,
                            struct fuse_file_info *fi,
                            unsigned flags,
                            const void *in_buf,
                            size_t in_bufsz,
                            size_t out_bufsz)
{
    /*
     * TODO: Fill me.
     */
    fuse_reply_err(req, ENOSYS);
}
#endif

static void aznfsc_ll_poll(fuse_req_t req,
                           fuse_ino_t ino,
                           struct fuse_file_info *fi,
                           struct fuse_pollhandle *ph)
{
    /*
     * TODO: Fill me.
     */
    fuse_reply_err(req, ENOSYS);
}

static void aznfsc_ll_write_buf(fuse_req_t req,
                                fuse_ino_t ino,
                                struct fuse_bufvec *bufv,
                                off_t off,
                                struct fuse_file_info *fi)
{
    assert(bufv->idx < bufv->count);
    const size_t length = bufv->buf[bufv->idx].size - bufv->off;
    assert((int) length >= 0);

    /*
     * XXX We are only handling count=1, assert to know if kernel sends more,
     *     we would want to handle that.
     */
    assert(bufv->count == 1);

    AZLogDebug("aznfsc_ll_write_buf(req={}, ino={}, bufv={}, off={}, len={}, "
               "fi={} [writepage: {}, flush: {}]",
               fmt::ptr(req), ino, fmt::ptr(bufv), off, length, fmt::ptr(fi),
               fi->writepage ? 1 : 0, fi->flush ? 1 : 0);

    struct nfs_client *client = get_nfs_client_from_fuse_req(req);

    /*
     * Sanity assert. 1MiB is the max write size fuse will ever issue.
     * If fuse sends more we'd like to know.
     *
     * TODO: Remove this before going to production.
     */
    assert(length <= 1048576);

    client->write(req, ino, bufv, length, off);
}

static void aznfsc_ll_retrieve_reply(fuse_req_t req,
                                     void *cookie,
                                     fuse_ino_t ino,
                                     off_t offset,
                                     struct fuse_bufvec *bufv)
{
    /*
     * TODO: Fill me.
     */
    fuse_reply_err(req, ENOSYS);
}

void aznfsc_ll_forget_multi(fuse_req_t req,
                            size_t count,
                            struct fuse_forget_data *forgets)
{
    total_forgotten += count;

    AZLogDebug("aznfsc_ll_forget_multi(req={}, count={}) total_forgotten={}",
               fmt::ptr(req), count, total_forgotten.load());

    struct nfs_client *client = get_nfs_client_from_fuse_req(req);

    for (size_t i = 0; i < count; i++) {
        const uint64_t nlookup = forgets[i].nlookup;
        const fuse_ino_t ino = forgets[i].ino;
        struct nfs_inode *inode = client->get_nfs_inode_from_ino(ino);

        AZLogDebug("forget(ino={}, nlookup={})", ino, nlookup);
        /*
         * Decrement refcnt of the inode and free the inode if refcnt
         * becomes 0.
         */
        inode->decref(nlookup, true /* from_forget */);
    }

    fuse_reply_none(req);
}

static void aznfsc_ll_flock(fuse_req_t req,
                            fuse_ino_t ino,
                            struct fuse_file_info *fi,
                            int op)
{
    /*
     * TODO: Fill me.
     */
    fuse_reply_err(req, ENOSYS);
}

static void aznfsc_ll_fallocate(fuse_req_t req,
                                fuse_ino_t ino,
                                int mode,
                                off_t offset,
                                off_t length,
                                struct fuse_file_info *fi)
{
    /*
     * TODO: Fill me.
     */
    fuse_reply_err(req, ENOSYS);
}

static void aznfsc_ll_readdirplus(fuse_req_t req,
                                  fuse_ino_t ino,
                                  size_t size,
                                  off_t off,
                                  struct fuse_file_info *fi)
{
    AZLogDebug("aznfsc_ll_readdirplus(req={}, ino={}, size={}, off={}, fi={})",
               fmt::ptr(req), ino, size, off, fmt::ptr(fi));

    struct nfs_client *client = get_nfs_client_from_fuse_req(req);
    client->readdirplus(req, ino, size, off, fi);
}

void aznfsc_ll_copy_file_range(fuse_req_t req,
                               fuse_ino_t ino_in,
                               off_t off_in,
                               struct fuse_file_info *fi_in,
                               fuse_ino_t ino_out,
                               off_t off_out,
                               struct fuse_file_info *fi_out,
                               size_t len,
                               int flags)
{
    /*
     * TODO: Fill me.
     */
    fuse_reply_err(req, ENOSYS);
}

static void aznfsc_ll_lseek(fuse_req_t req,
                            fuse_ino_t ino,
                            off_t off,
                            int whence,
                            struct fuse_file_info *fi)
{
    /*
     * TODO: Fill me.
     */
    fuse_reply_err(req, ENOSYS);
}

static struct fuse_lowlevel_ops aznfsc_ll_ops = {
    .init               = aznfsc_ll_init,
    .destroy            = aznfsc_ll_destroy,
    .lookup             = aznfsc_ll_lookup,
    .forget             = aznfsc_ll_forget,
    .getattr            = aznfsc_ll_getattr,
    .setattr            = aznfsc_ll_setattr,
    .readlink           = aznfsc_ll_readlink,
    .mknod              = aznfsc_ll_mknod,
    .mkdir              = aznfsc_ll_mkdir,
    .unlink             = aznfsc_ll_unlink,
    .rmdir              = aznfsc_ll_rmdir,
    .symlink            = aznfsc_ll_symlink,
    .rename             = aznfsc_ll_rename,
    .link               = aznfsc_ll_link,
    .open               = aznfsc_ll_open,
    .read               = aznfsc_ll_read,
    .write              = aznfsc_ll_write,
    .flush              = aznfsc_ll_flush,
    .release            = aznfsc_ll_release,
    .fsync              = aznfsc_ll_fsync,
    .opendir            = aznfsc_ll_opendir,
    .readdir            = aznfsc_ll_readdir,
    .releasedir         = aznfsc_ll_releasedir,
    .fsyncdir           = aznfsc_ll_fsyncdir,
    .statfs             = aznfsc_ll_statfs,
    .setxattr           = aznfsc_ll_setxattr,
    .getxattr           = aznfsc_ll_getxattr,
    .listxattr          = aznfsc_ll_listxattr,
    .removexattr        = aznfsc_ll_removexattr,
    .access             = aznfsc_ll_access,
    .create             = aznfsc_ll_create,
    .getlk              = aznfsc_ll_getlk,
    .setlk              = aznfsc_ll_setlk,
    .bmap               = aznfsc_ll_bmap,
    .ioctl              = aznfsc_ll_ioctl,
    .poll               = aznfsc_ll_poll,
    .write_buf          = aznfsc_ll_write_buf,
    .retrieve_reply     = aznfsc_ll_retrieve_reply,
    .forget_multi       = aznfsc_ll_forget_multi,
    .flock              = aznfsc_ll_flock,
    .fallocate          = aznfsc_ll_fallocate,
    .readdirplus        = aznfsc_ll_readdirplus,
    .copy_file_range    = aznfsc_ll_copy_file_range,
    .lseek              = aznfsc_ll_lseek,
};

/*
 * Setup signal handler for the given signal.
 */
static int set_signal_handler(int signum, void (*handler)(int))
{
    struct sigaction sa;

    memset(&sa, 0, sizeof(struct sigaction));
    sa.sa_handler = handler;
    sigemptyset(&(sa.sa_mask));
    sa.sa_flags = 0;

    return sigaction(signum, &sa, NULL);
}

static void handle_usr1([[maybe_unused]] int signum)
{
    assert(signum == SIGUSR1);
    rpc_stats_az::dump_stats();
}

int main(int argc, char *argv[])
{
    // Initialize logger first thing.
    init_log();

    AZLogInfo("aznfsclient version {}.{}.{}",
              AZNFSCLIENT_VERSION_MAJOR,
              AZNFSCLIENT_VERSION_MINOR,
              AZNFSCLIENT_VERSION_PATCH);

    struct fuse_args args = FUSE_ARGS_INIT(argc, argv);
    struct fuse_session *se = NULL;
    struct fuse_cmdline_opts opts;
    struct fuse_loop_config *loop_config = fuse_loop_cfg_create();
    int ret = -1;

    /* Don't mask creation mode, kernel already did that */
    umask(0);

    /*
     * Parse general cmdline options first for properly honoring help
     * and debug level arguments.
     */
    if (fuse_parse_cmdline(&args, &opts) != 0) {
        return 1;
    }

    if (opts.show_help) {
        aznfsc_help(argv[0]);
        fuse_cmdline_help();
        fuse_lowlevel_help();
        ret = 0;
        goto err_out1;
    } else if (opts.show_version) {
        printf("FUSE library version %s\n", fuse_pkgversion());
        fuse_lowlevel_version();
        ret = 0;
        goto err_out1;
    }

    /*
     * If -d or "-o debug" cmdline option was passed, reset log level to
     * debug.
     */
    if (opts.debug) {
        enable_debug_logs = true;
        spdlog::set_level(spdlog::level::debug);
    }

    // Parse fuse_conn_info_opts options like -o writeback_cache.
    fuse_conn_info_opts_ptr = fuse_parse_conn_info_opts(&args);

    // Parse aznfsclient specific options.
    if (fuse_opt_parse(&args, &aznfsc_cfg, aznfsc_opts, NULL) == -1) {
        return 1;
    }

    /*
     * TODO: Add validity checks for aznfsc_cfg cmdline options, similar to
     *       parse_config_yaml().
     */

    // Parse config yaml if --config-yaml option provided.
    if (!aznfsc_cfg.parse_config_yaml()) {
        return 1;
    }

    /*
     * account and container are mandatory parameters which do not have a
     * default value, so ensure they are set before proceeding further.
     */
    if (aznfsc_cfg.account == nullptr) {
        AZLogError("Account name must be set either from cmdline or config yaml!");
        return 1;
    }

    if (aznfsc_cfg.container == nullptr) {
        AZLogError("Container name must be set either from cmdline or config yaml!");
        return 1;
    }

    aznfsc_cfg.mountpoint = opts.mountpoint;

    // Set default values for config variables not set using the above.
    aznfsc_cfg.set_defaults_and_sanitize();

    se = fuse_session_new(&args, &aznfsc_ll_ops, sizeof(aznfsc_ll_ops),
                          &nfs_client::get_instance());
    if (se == NULL) {
        AZLogError("fuse_session_new failed");
        goto err_out1;
    }

    if (fuse_set_signal_handlers(se) != 0) {
        AZLogError("fuse_set_signal_handlers failed");
        goto err_out2;
    }

    /*
     * Setup SIGUSR1 handler for dumping RPC stats.
     */
    if (set_signal_handler(SIGUSR1, handle_usr1) != 0) {
        AZLogError("set_signal_handler(SIGUSR1) failed: {}", ::strerror(errno));
        goto err_out3;
    }

    if (fuse_session_mount(se, opts.mountpoint) != 0) {
        AZLogError("fuse_session_mount failed");
        goto err_out3;
    }

    if (fuse_daemonize(opts.foreground) != 0) {
        AZLogError("fuse_daemonize failed");
        goto err_out4;
    }

    /*
     * Initialize nfs_client singleton.
     * This creates the libnfs polling thread(s) and hence it MUST be called
     * after fuse_daemonize(), else those threads will get killed.
     */
    if (!nfs_client::get_instance().init()) {
        AZLogError("Failed to init the NFS client");
        goto err_out4;
    }

    AZLogInfo("==> Aznfsclient fuse driver ready to serve requests!");

    if (opts.singlethread) {
        ret = fuse_session_loop(se);
    } else {
        fuse_loop_cfg_set_clone_fd(loop_config, opts.clone_fd);
        fuse_loop_cfg_set_max_threads(loop_config, opts.max_threads);
        fuse_loop_cfg_set_idle_threads(loop_config, opts.max_idle_threads);

        ret = fuse_session_loop_mt(se, loop_config);
    }

    /*
     * We come here when user unmounts the fuse filesystem.
     */
    AZLogInfo("Shutting down!");

    nfs_client::get_instance().shutdown();

err_out4:
    fuse_loop_cfg_destroy(loop_config);
    fuse_session_unmount(se);
err_out3:
    fuse_remove_signal_handlers(se);
err_out2:
    fuse_session_destroy(se);
err_out1:
    free(opts.mountpoint);
    fuse_opt_free_args(&args);

    return ret ? 1 : 0;
}
