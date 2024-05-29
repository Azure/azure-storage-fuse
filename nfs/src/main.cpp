#include "aznfsc.h"
#include "nfs_client.h"
#include "nfs_internal.h"
#include "yaml-cpp/yaml.h"
using namespace std;

// This holds the global options for the fuse like max_write, max_readahead etc.
struct fuse_conn_info_opts* fuse_conn_info_opts_ptr;

/**
 * This structure holds the entire aznfsclient configuration that can control
 * the behaviour of aznfsclient fuse program. These config variables can be
 * configured in many ways, allowing user to conveniently express their default
 * configuration and allowing easy overrides for some as needed.
 *
 * Here are the various ways these config values are populated:
 * 1. Most configs have default values.
 *    Note: Some of the config variables pertain to user details and cannot
 *          have default values.
 * 2. Convenient place for defining config variables which don't need to be
 *    changed often is the config.yaml file that user can provide with the
 *    --config-file=./config.yaml cmdline option to aznfsclient.
 *    These override the defaults.
 * 3. Some but not all config variables can be set using environment variables.
 *    These override the variables set by config.yaml and the default.
 * 4. Most config variables can be set using specific command line options to
 *    aznfsclient.
 *    These have the highest preference and will override the variables set
 *    by environment variables, config.yaml and the default.
 *
 * Note: This MUST not contains C++ object types as members as fuse parser
 *       writes into those members. For char* members fuse also allocates memory.
 */
struct aznfsc_cfg
{
    // config.yaml file path specified using --config-file= cmdline option.
    const char* config_yaml;

    /*
     * Storage account and container to mount and the optional cloud suffix.
     * The share path mounted is:
     * <account>.<cloud_suffix>:/<account>/<container>
     */
    const char* account;
    const char* container;
    char* cloud_suffix;

    /*
     * NFS and Mount port to use.
     * If this is non-zero, portmapper won't be contacted.
     */
    uint16_t port = 0;

    // Nfsv3 version
    int version;

    // Number of connections to be established to the server.
    int nconnect;

    // Max number of times the API will be retried before erroring out.
    int max_num_of_retries;

    // Maximum time the API call waits before returning a timeout to the caller.
    uint64_t timeout_in_sec;

    // Maximum size of read request.
    size_t readmax;

    // Maximum size of write request.
    size_t writemax;

    // Number of times the request will be retransmitted to the server when no response is received.
    int retrans;

    // Maximum number of readdir entries that can be requested.
    uint32_t readdir_maxcount;

    /*
     * TODO:
     * - Add auth related config.
     * - Add perf related config,
     *   e.g., amount of RAM used for staging writes, etc.
     */

    // Populate default values in the constructor which can be overwritten later.
    aznfsc_cfg():
        config_yaml(nullptr),
        account(nullptr),
        container(nullptr),
        cloud_suffix(nullptr),
        port(0),
        version(3),
        nconnect(1),
        max_num_of_retries(3),
        timeout_in_sec(600),
        readmax(1048576 /* Setting it to 1MB now, should be modified later */),
        writemax(1048576 /* Setting it to 1MB now, should be modified later */),
        retrans(3),
        readdir_maxcount(UINT32_MAX)
    {
        cloud_suffix = (char*)malloc(strlen("blob.core.windows.net") + 1);
        cloud_suffix = strdup("blob.core.windows.net");
    }
} aznfsc_cfg;

#define AZNFSC_OPT(templ, key) { templ, offsetof(struct aznfsc_cfg, key), 0}

/*
 * These are aznfsclient specific options.
 * These can be passed to aznfsclient fuse program, in addition to the standard
 * fuse options.
 */
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

/*
 * This function parses the contents of the yaml config file denoted by path config_file
 * into the aznfsc_cfg structure.
 * TODO: Validate the values and make sure they are within the range expected.
 */
bool parse_config_file(const char* config_file)
{
    if (config_file == nullptr)
    {
        return false;
    }

    try {
        YAML::Node config = YAML::LoadFile(config_file);

        if (aznfsc_cfg.account == nullptr && config["account"])
        {
            aznfsc_cfg.account = strdup(config["account"].as<std::string>().c_str());
        }
        if (aznfsc_cfg.container == nullptr && config["container"])
        {
            aznfsc_cfg.container = strdup(config["container"].as<std::string>().c_str());
        }
        if (config["cloud_suffix"])
        {
            aznfsc_cfg.cloud_suffix = strdup(config["cloud_suffix"].as<std::string>().c_str());
        }
        if (config["port"])
        {
            aznfsc_cfg.port = config["port"].as<uint16_t>();
        }
        if (config["version"])
        {
            aznfsc_cfg.version = config["version"].as<int>();
        }
        if (config["nconnect"])
        {
            aznfsc_cfg.nconnect = config["nconnect"].as<int>();
        }
        if (config["max_num_of_retries"])
        {
            aznfsc_cfg.max_num_of_retries = config["max_num_of_retries"].as<int>();
        }
        if (config["timeout_in_sec"])
        {
            aznfsc_cfg.timeout_in_sec = config["timeout_in_sec"].as<uint64_t>();
        }
        if (config["readmax"])
        {
            aznfsc_cfg.readmax = config["readmax"].as<size_t>();
        }
        if (config["writemax"])
        {
            aznfsc_cfg.writemax = config["writemax"].as<size_t>();
        }
        if (config["retrans"])
        {
            aznfsc_cfg.retrans = config["retrans"].as<int>();
        }
        if (config["readdir_maxcount"])
        {
            aznfsc_cfg.readdir_maxcount = config["readdir_maxcount"].as<uint32_t>();
        }
    } catch (const YAML::BadFile& e) {
        AZLogError("Error loading file: {}, error: {}", config_file, e.what());
        return false;
    } catch (const YAML::Exception& e) {
        AZLogError("Error parsing the config file: {}, error: {}", config_file, e.what());
        return false;
    }

    return true;
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
     * fuse_session_new() no longer accepts arguments
     * command line options can only be set using fuse_apply_conn_info_opts().
     */
    fuse_apply_conn_info_opts(fuse_conn_info_opts_ptr, conn);
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
                             fuse_ino_t parent,
                             const char *name)
{
    AZLogInfo("aznfsc_ll_lookup file: {}", name);

    auto client = reinterpret_cast<struct nfs_client*>(fuse_req_userdata(req));
    client->lookup(req, parent, name);
}

static void aznfsc_ll_forget(fuse_req_t req,
                             fuse_ino_t ino,
                             uint64_t nlookup)
{
    /*
     * TODO: Fill me.
     *       This is where we free an inode if we are caching it.
     */
    fuse_reply_none(req);
}

static void aznfsc_ll_getattr(fuse_req_t req,
                              fuse_ino_t ino,
                              struct fuse_file_info *fi)
{
    AZLogInfo("Getattr called");

    auto client = reinterpret_cast<struct nfs_client*>(fuse_req_userdata(req));
    client->getattr(req, ino, fi);
}

static void aznfsc_ll_setattr(fuse_req_t req,
                              fuse_ino_t ino,
                              struct stat *attr,
                              int to_set /* bitmask indicating the attributes to set */,
                              struct fuse_file_info *fi)
{
    AZLogInfo("Setattr called");

    auto client = reinterpret_cast<struct nfs_client*>(fuse_req_userdata(req));
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
                            fuse_ino_t parent,
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
                            fuse_ino_t parent,
                            const char *name,
                            mode_t mode)
{
    AZLogInfo("Mkdir called, name: {}", name);

    auto client = reinterpret_cast<struct nfs_client*>(fuse_req_userdata(req));
    client->mkdir(req, parent, name, mode);
}

static void aznfsc_ll_unlink(fuse_req_t req,
                             fuse_ino_t parent,
                             const char *name)
{
    /*
     * TODO: Fill me.
     */
    fuse_reply_err(req, ENOSYS);
}

static void aznfsc_ll_rmdir(fuse_req_t req,
                            fuse_ino_t parent,
                            const char *name)
{
    /*
     * TODO: Fill me.
     */
    fuse_reply_err(req, ENOSYS);
}

static void aznfsc_ll_symlink(fuse_req_t req,
                              const char *link,
                              fuse_ino_t parent,
                              const char *name)
{
    /*
     * TODO: Fill me.
     */
    fuse_reply_err(req, ENOSYS);
}

static void aznfsc_ll_rename(fuse_req_t req,
                             fuse_ino_t parent,
                             const char *name,
                             fuse_ino_t newparent,
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
                           fuse_ino_t newparent,
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
    /*
     * TODO: Fill me.
     */
    fuse_reply_err(req, ENOSYS);
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
    /*
     * TODO: Fill me.
     */
    fuse_reply_err(req, ENOSYS);
}

static void aznfsc_ll_release(fuse_req_t req,
                              fuse_ino_t ino,
                              struct fuse_file_info *fi)
{
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
    /*
     * TODO: Fill me.
     */
    fuse_reply_err(req, ENOSYS);
}

static void aznfsc_ll_readdir(fuse_req_t req,
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

static void aznfsc_ll_releasedir(fuse_req_t req,
                                 fuse_ino_t ino,
                                 struct fuse_file_info *fi)
{
    /*
     * TODO: Fill me.
     */
    fuse_reply_err(req, ENOSYS);
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
                             fuse_ino_t parent,
                             const char *name,
                             mode_t mode,
                             struct fuse_file_info *fi)
{
    AZLogInfo("Creating file: {}", name);

    auto client = reinterpret_cast<struct nfs_client*>(fuse_req_userdata(req));
    client->create(req, parent, name, mode, fi);
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
    /*
     * TODO: Fill me.
     */
    fuse_reply_err(req, ENOSYS);
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
    /*
     * TODO: Fill me.
     */
    fuse_reply_err(req, ENOSYS);
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

    /*
     * First parse the options from the config file if that is present.
     * We should do this before we parse the command line options since the latter should take higher priority.
     */
    char* config_file = nullptr;
    for (int idx = 0; idx < argc; idx++)
    {
        std::string arg = argv[idx];
        if (arg.find("--config-file=") == 0)
        {
            config_file = strdup(arg.substr(14).c_str());
        }
    }

    if (config_file != nullptr)
    {
        AZLogInfo ("Parsing the config file {}", config_file);
        parse_config_file(config_file);
        free(config_file);
    }

    struct fuse_session *se;
    struct fuse_cmdline_opts opts;
    struct fuse_loop_config loop_config;
    int ret = -1;

    /* Don't mask creation mode, kernel already did that */
    umask(0);

    /* accept options like -o writeback_cache */
    fuse_conn_info_opts_ptr = fuse_parse_conn_info_opts(&args);

    if (fuse_opt_parse(&args, &aznfsc_cfg, aznfsc_opts, NULL) == -1) {
        return 1;
    }

    if (fuse_parse_cmdline(&args, &opts) != 0) {
        return 1;
    }

    if (opts.show_help) {
        printf("usage: %s [options] <mountpoint>\n\n", argv[0]);
        printf("[options]: --config-file=<config.yaml file path>\n");
        printf("           --account=<storage account>\n");
        printf("           --container=<container>\n");
        printf("           --cloud-suffix=<cloud suffix>\n");
        printf("           --por=<port for connecting to Blob NFS>\n");
        printf("Example :   ./aznfsclient --config-file=./config.yaml /mnt/tmp\n\n");
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

    {
        std::string account = aznfsc_cfg.account;
        std::string container = aznfsc_cfg.container;
        std::string suffix = aznfsc_cfg.cloud_suffix;

        // TODO: See if we need a seperate mount_options structure.
        // 	 Can we just pass the aznfsc_cfg structure if we move its defination to a .h file?
        //
        struct mount_options mntOpt;
        mntOpt.server = account + "." + suffix;
        mntOpt.export_path = "/" + account + "/" + container;
        mntOpt.set_nfs_port(aznfsc_cfg.port);
        mntOpt.num_connections = aznfsc_cfg.nconnect;
        mntOpt.nfs_version = aznfsc_cfg.version;
        mntOpt.set_read_max(aznfsc_cfg.readmax);
        mntOpt.set_write_max(aznfsc_cfg.writemax);
        // TODO: We are still not using the aznfsc_cfg.max_num_of_retries and timeout, handle them in appropriate place.

        // init the nfs client to use it.
        if (!nfs_client::init(account, container, suffix, &mntOpt))
        {
            AZLogError("Failed to init the NFS client.");
            goto err_out4;
        }
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
