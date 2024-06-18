#include "aznfsc.h"
#include "nfs_client.h"
#include "nfs_internal.h"
#include "yaml-cpp/yaml.h"

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

        if ((account == nullptr) && config["account"]) {
            account = ::strdup(config["account"].as<std::string>().c_str());
            if (!is_valid_storageaccount(account)) {
                throw YAML::Exception(
                    config["account"].Mark(),
                    std::string("Invalid storage account name: ") +
                    std::string(account));
            }
        }

        if ((container == nullptr) && config["container"]) {
            container = ::strdup(config["container"].as<std::string>().c_str());
            if (!is_valid_container(container)) {
                throw YAML::Exception(
                    config["container"].Mark(),
                    std::string("Invalid container name: ") +
                    std::string(container));
            }
        }

        if ((cloud_suffix == nullptr) && config["cloud_suffix"]) {
            cloud_suffix =
                ::strdup(config["cloud_suffix"].as<std::string>().c_str());
            if (!is_valid_cloud_suffix(cloud_suffix)) {
                throw YAML::Exception(
                    config["cloud_suffix"].Mark(),
                    std::string("Invalid cloud_suffix: ") +
                    std::string(cloud_suffix));
            }
        }

        if ((port == -1) && config["port"]) {
            port = config["port"].as<int>();
            if (port != 2048 && port != 2047) {
                throw YAML::Exception(
                    config["port"].Mark(),
                    std::string("Invalid port number: ") +
                    std::to_string(port) +
                    std::string(" (can be 2048 or 2047)"));
            }
        }

        if ((nconnect == -1) && config["nconnect"]) {
            nconnect = config["nconnect"].as<int>();
            if (nconnect < AZNFSCFG_NCONNECT_MIN ||
                nconnect > AZNFSCFG_NCONNECT_MAX) {
                throw YAML::Exception(
                    config["nconnect"].Mark(),
                    std::string("Invalid nconnect value: ") +
                    std::to_string(nconnect) +
                    std::string(" (valid range [") +
                    std::to_string(AZNFSCFG_NCONNECT_MIN) +
                    ", " + std::to_string(AZNFSCFG_NCONNECT_MAX) + "])");
            }
        }

        if ((timeo == -1) && config["timeo"]) {
            timeo = config["timeo"].as<int>();
            if (timeo < AZNFSCFG_TIMEO_MIN || timeo > AZNFSCFG_TIMEO_MAX) {
                throw YAML::Exception(
                    config["timeo"].Mark(),
                    std::string("Invalid timeo value: ") +
                    std::to_string(timeo) +
                    std::string(" (valid range [") +
                    std::to_string(AZNFSCFG_TIMEO_MIN) +
                    ", " + std::to_string(AZNFSCFG_TIMEO_MAX) + "])");
            }
        }

        if ((acregmin == -1) && config["acregmin"]) {
            acregmin = config["acregmin"].as<int>();
            if (acregmin < AZNFSCFG_ACTIMEO_MIN ||
                acregmin > AZNFSCFG_ACTIMEO_MAX) {
                throw YAML::Exception(
                    config["acregmin"].Mark(),
                    std::string("Invalid acregmin value: ") +
                    std::to_string(acregmin) +
                    std::string(" (valid range [") +
                    std::to_string(AZNFSCFG_ACTIMEO_MIN) +
                    ", " + std::to_string(AZNFSCFG_ACTIMEO_MAX) + "])");
            }
        }

        if ((acregmax == -1) && config["acregmax"]) {
            acregmax = config["acregmax"].as<int>();
            if (acregmax < AZNFSCFG_ACTIMEO_MIN ||
                acregmax > AZNFSCFG_ACTIMEO_MAX) {
                throw YAML::Exception(
                    config["acregmax"].Mark(),
                    std::string("Invalid acregmax value: ") +
                    std::to_string(acregmax) +
                    std::string(" (valid range [") +
                    std::to_string(AZNFSCFG_ACTIMEO_MIN) +
                    ", " + std::to_string(AZNFSCFG_ACTIMEO_MAX) + "])");
            }
        }

        if ((acdirmin == -1) && config["acdirmin"]) {
            acdirmin = config["acdirmin"].as<int>();
            if (acdirmin < AZNFSCFG_ACTIMEO_MIN ||
                acdirmin > AZNFSCFG_ACTIMEO_MAX) {
                throw YAML::Exception(
                    config["acdirmin"].Mark(),
                    std::string("Invalid acdirmin value: ") +
                    std::to_string(acdirmin) +
                    std::string(" (valid range [") +
                    std::to_string(AZNFSCFG_ACTIMEO_MIN) +
                    ", " + std::to_string(AZNFSCFG_ACTIMEO_MAX) + "])");
            }
        }

        if ((acdirmax == -1) && config["acdirmax"]) {
            acdirmax = config["acdirmax"].as<int>();
            if (acdirmax < AZNFSCFG_ACTIMEO_MIN ||
                acdirmax > AZNFSCFG_ACTIMEO_MAX) {
                throw YAML::Exception(
                    config["acdirmax"].Mark(),
                    std::string("Invalid acdirmax value: ") +
                    std::to_string(acdirmax) +
                    std::string(" (valid range [") +
                    std::to_string(AZNFSCFG_ACTIMEO_MIN) +
                    ", " + std::to_string(AZNFSCFG_ACTIMEO_MAX) + "])");
            }
        }

        if ((actimeo == -1) && config["actimeo"]) {
            actimeo = config["actimeo"].as<int>();
            if (actimeo < AZNFSCFG_ACTIMEO_MIN ||
                actimeo > AZNFSCFG_ACTIMEO_MAX) {
                throw YAML::Exception(
                    config["actimeo"].Mark(),
                    std::string("Invalid actimeo value: ") +
                    std::to_string(actimeo) +
                    std::string(" (valid range [") +
                    std::to_string(AZNFSCFG_ACTIMEO_MIN) +
                    ", " + std::to_string(AZNFSCFG_ACTIMEO_MAX) + "])");
            }
        }

        if ((rsize == -1) && config["rsize"]) {
            rsize = config["rsize"].as<int>();
            if (rsize < AZNFSCFG_RSIZE_MIN || rsize > AZNFSCFG_RSIZE_MAX) {
                throw YAML::Exception(
                    config["rsize"].Mark(),
                    std::string("Invalid rsize value: ") +
                    std::to_string(rsize) +
                    std::string(" (valid range [") +
                    std::to_string(AZNFSCFG_RSIZE_MIN) +
                    ", " + std::to_string(AZNFSCFG_RSIZE_MAX) + "])");
            }
        }

        if ((wsize == -1) && config["wsize"]) {
            wsize = config["wsize"].as<int>();
            if (wsize < AZNFSCFG_WSIZE_MIN || wsize > AZNFSCFG_WSIZE_MAX) {
                throw YAML::Exception(
                    config["wsize"].Mark(),
                    std::string("Invalid wsize value: ") +
                    std::to_string(wsize) +
                    std::string(" (valid range [") +
                    std::to_string(AZNFSCFG_WSIZE_MIN) +
                    ", " + std::to_string(AZNFSCFG_WSIZE_MAX) + "])");
            }
        }

        if ((retrans == -1) && config["retrans"]) {
            retrans = config["retrans"].as<int>();
            if (retrans < AZNFSCFG_RETRANS_MIN ||
                retrans > AZNFSCFG_RETRANS_MAX) {
                throw YAML::Exception(
                    config["retrans"].Mark(),
                    std::string("Invalid retrans value: ") +
                    std::to_string(retrans) +
                    std::string(" (valid range [") +
                    std::to_string(AZNFSCFG_RETRANS_MIN) +
                    ", " + std::to_string(AZNFSCFG_RETRANS_MAX) + "])");
            }
        }

        if ((readdir_maxcount == -1) && config["readdir_maxcount"]) {
            readdir_maxcount = config["readdir_maxcount"].as<int>();
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
    if (actimeo != -1)
        acregmin = acregmax = acdirmin = acdirmax = actimeo;
    if (acregmin > acregmax)
        acregmin = acregmax;
    if (acdirmin > acdirmax)
        acdirmin = acdirmax;
    if (readdir_maxcount == -1)
        readdir_maxcount = INT_MAX;
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
    AZLogDebug("readdir_maxcount = {}", readdir_maxcount);
    AZLogDebug("account = {}", account);
    AZLogDebug("container = {}", container);
    AZLogDebug("cloud_suffix = {}", cloud_suffix);
    AZLogDebug("===== config end =====");
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
     * XXX Disable readdir temporarily while I work on fixing readdirplus.
     *     Once readdirplus is audited/fixed, enable readdir and audit/fix
     *     that.
     */
    conn->want |= FUSE_CAP_READDIRPLUS;
    conn->want &= ~FUSE_CAP_READDIRPLUS_AUTO;

    conn->want |= FUSE_CAP_ASYNC_READ;

    // Blob NFS doesn't support locking.
    conn->want &= ~FUSE_CAP_POSIX_LOCKS;
    conn->want &= ~FUSE_CAP_FLOCK_LOCKS;

    // See if we can support O_TRUNC.
    conn->want &= ~FUSE_CAP_ATOMIC_O_TRUNC;

    // Test splice read/write performance before enabling.
    //conn->want |= FUSE_CAP_SPLICE_WRITE;
    //conn->want |= FUSE_CAP_SPLICE_MOVE;
    //conn->want |= FUSE_CAP_SPLICE_READ;

    conn->want |= FUSE_CAP_AUTO_INVAL_DATA;
    conn->want |= FUSE_CAP_ASYNC_DIO;

    conn->want &= ~FUSE_CAP_WRITEBACK_CACHE;
    conn->want |= FUSE_CAP_PARALLEL_DIROPS;
    conn->want &= ~FUSE_CAP_POSIX_ACL;
    conn->want &= ~FUSE_CAP_CACHE_SYMLINKS;
    conn->want &= ~FUSE_CAP_SETXATTR_EXT;

    /*
     * Now apply the user passed options (-o). This must be done after
     * whatever default flags we set above, so that user can override
     * them. If there's some flag that we want to force irrespective of
     * user argument, that should be set after this.
     *
     * Note: fuse_session_new() no longer accepts arguments
     *       command line options can only be set using
     *       fuse_apply_conn_info_opts().
     */
    fuse_apply_conn_info_opts(fuse_conn_info_opts_ptr, conn);

    AZLogDebug("===== fuse_conn_info fields start =====");
    AZLogDebug("proto_major = {}", conn->proto_major);
    AZLogDebug("proto_minor = {}", conn->proto_minor);
    AZLogDebug("max_write = {}", conn->max_write);
    AZLogDebug("max_read = {}", conn->max_write);
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

static void aznfsc_ll_forget(fuse_req_t req,
                             fuse_ino_t ino,
                             uint64_t nlookup)
{
    AZLogDebug("aznfsc_ll_forget(req={}, ino={}, nlookup={})",
               fmt::ptr(req), ino, nlookup);

    struct nfs_client *client = get_nfs_client_from_fuse_req(req);

    /*
     * Decrement refcnt of the inode and free the inode if refcnt becomes 0.
     */
    assert(nlookup > 0);
    while (nlookup--) {
        client->get_nfs_inode_from_ino(ino)->decref();
    }

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
    /*
     * TODO: Fill me.
     */
    fuse_reply_err(req, ENOSYS);
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
    AZLogDebug("aznfsc_ll_mkdir(req={}, parent_ino={}, name={}, mode=0{:03o}",
               fmt::ptr(req), parent_ino, name, mode);

    struct nfs_client *client = get_nfs_client_from_fuse_req(req);
    client->mkdir(req, parent_ino, name, mode);
}

static void aznfsc_ll_unlink(fuse_req_t req,
                             fuse_ino_t parent_ino,
                             const char *name)
{
    /*
     * TODO: Fill me.
     */
    fuse_reply_err(req, ENOSYS);
}

static void aznfsc_ll_rmdir(fuse_req_t req,
                            fuse_ino_t parent_ino,
                            const char *name)
{
    /*
     * TODO: Fill me.
     */
    fuse_reply_err(req, ENOSYS);
}

static void aznfsc_ll_symlink(fuse_req_t req,
                              const char *link,
                              fuse_ino_t parent_ino,
                              const char *name)
{
    /*
     * TODO: Fill me.
     */
    fuse_reply_err(req, ENOSYS);
}

static void aznfsc_ll_rename(fuse_req_t req,
                             fuse_ino_t parent_ino,
                             const char *name,
                             fuse_ino_t newparent_ino,
                             const char *newname,
                             unsigned int flags)
{
    /*
     * TODO: Fill me.
     */
    fuse_reply_err(req, ENOSYS);
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
    fi->direct_io = 1;
    fi->keep_cache = 0;
    fi->nonseekable = 0;
    fi->parallel_direct_writes = 1;
    fi->noflush = 0;

    fuse_reply_open(req, fi);
}

static void aznfsc_ll_read(fuse_req_t req,
                           fuse_ino_t ino,
                           size_t size,
                           off_t off,
                           struct fuse_file_info *fi)
{
    /*
     * TODO: Fill me.
     */
    fuse_reply_err(req, ENOSYS);
}

static void aznfsc_ll_write(fuse_req_t req,
                            fuse_ino_t ino,
                            const char *buf,
                            size_t size,
                            off_t off,
                            struct fuse_file_info *fi)
{
    /*
     * TODO: Fill me.
     */
    fuse_reply_err(req, ENOSYS);
}

static void aznfsc_ll_flush(fuse_req_t req,
                            fuse_ino_t ino,
                            struct fuse_file_info *fi)
{
    AZLogInfo("aznfsc_ll_flush(req={}, ino={}, fi={})",
               fmt::ptr(req), ino, fmt::ptr(fi));
    /*
     * TODO: Fill me.
     */
    fuse_reply_err(req, ENOSYS);
}

static void aznfsc_ll_release(fuse_req_t req,
                              fuse_ino_t ino,
                              struct fuse_file_info *fi)
{
    AZLogInfo("aznfsc_ll_release(req={}, ino={}, fi={})",
               fmt::ptr(req), ino, fmt::ptr(fi));
    /*
     * TODO: Fill me.
     */
    fuse_reply_err(req, ENOSYS);
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
    AZLogInfo("aznfsc_ll_opendir(req={}, ino={}, fi={})",
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
    fi->direct_io = 0;
    fi->keep_cache = 0;
    fi->nonseekable = 0;
    fi->cache_readdir = 0;
    fi->noflush = 0;

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

    // Till we have readdir disabled in aznfsc_ll_init().
    assert(0);

    struct nfs_client *client = get_nfs_client_from_fuse_req(req);
    client->readdir(req, ino, size, off, fi);
}

static void aznfsc_ll_releasedir(fuse_req_t req,
                                 fuse_ino_t ino,
                                 struct fuse_file_info *fi)
{
    AZLogInfo("aznfsc_ll_releasedir(req={}, ino={}, fi={})",
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
    /*
     * TODO: Fill me.
     */
    fuse_reply_err(req, ENOSYS);
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
    AZLogDebug("aznfsc_ll_forget_multi(req={}, count={})",
               fmt::ptr(req), count);

    struct nfs_client *client = get_nfs_client_from_fuse_req(req);

    for (size_t i = 0; i < count; i++) {
        uint64_t nlookup = forgets[i].nlookup;
        const fuse_ino_t ino = forgets[i].ino;

        AZLogDebug("forget(ino={}, nlookup={})", ino, nlookup);
        /*
         * Decrement refcnt of the inode and free the inode if refcnt
         * becomes 0.
         */
        while (nlookup--) {
            client->get_nfs_inode_from_ino(ino)->decref();
        }
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

int main(int argc, char *argv[])
{
    // Initialize logger first thing.
    init_log();

    AZLogInfo("aznfsclient version {}.{}.{}",
              AZNFSCLIENT_VERSION_MAJOR,
              AZNFSCLIENT_VERSION_MINOR,
              AZNFSCLIENT_VERSION_PATCH);

    struct fuse_args args = FUSE_ARGS_INIT(argc, argv);
    struct fuse_session *se;
    struct fuse_cmdline_opts opts;
    struct fuse_loop_config loop_config;
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

    // Set default values for config variables not set using the above.
    aznfsc_cfg.set_defaults_and_sanitize();

    // Initialize nfs_client singleton.
    if (!nfs_client::get_instance().init()) {
        AZLogError("Failed to init the NFS client");
        goto err_out4;
    }

    se = fuse_session_new(&args, &aznfsc_ll_ops, sizeof(aznfsc_ll_ops), &nfs_client::get_instance());
    if (se == NULL) {
        goto err_out1;
    }

    if (fuse_set_signal_handlers(se) != 0) {
        goto err_out2;
    }

    if (fuse_session_mount(se, opts.mountpoint) != 0) {
        goto err_out3;
    }

    opts.foreground = 1;
    fuse_daemonize(opts.foreground);

    /* Block until ctrl+c or fusermount -u */
    printf("singlethread: %d\n", opts.singlethread);

    if (opts.singlethread) {
        ret = fuse_session_loop(se);
    } else {
        loop_config.clone_fd = opts.clone_fd;
        loop_config.max_idle_threads = opts.max_idle_threads;
        ret = fuse_session_loop_mt(se, &loop_config);
    }

err_out4:
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
