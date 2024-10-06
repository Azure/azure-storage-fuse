#include "aznfsc.h"
#include "rpc_stats.h"

#include <signal.h>

/*
 * Note: This file should only contain code needed for fuse interfacing.
 */

using namespace std;

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
 * FS handler definitions common between fuse and nofuse.
 */
#include "fs-handler.h"

/*
 * Handlers specific to fuse.
 */
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

    /*
     * Hide fuse'ism and behave like a normal POSIX fs.
     * TODO: Make this configurable?
     */
    if (fuse_opt_add_arg(&args, "-oallow_other,default_permissions") == -1) {
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
