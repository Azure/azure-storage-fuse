#include "nfs_internal.h"
#include "rpc_task.h"
#include "nfs_client.h"
#include "rpc_stats.h"

/*
 * If this is defined we will call release() for the byte chunk which is read
 * by the application. This helps free the cache as soon as the reader reads
 * it. The idea is to not keep cached data hanging around for any longer than
 * it's needed. Most common access pattern that we need to support is seq read
 * where readahead fills the cache and helps future application read calls.
 * Once application has read the data, we don't want it to linger in the cache
 * for any longer. This means future reads won't get it, but the idea is that
 * if future reads are also sequential, they will get it by readahead.
 *
 * If this is disabled, then pruning is the only way to reclaim cache memory.
 */
#define RELEASE_CHUNK_AFTER_APPLICATION_READ

#define NFS_STATUS(r) ((r) ? (r)->status : NFS3ERR_SERVERFAULT)

#ifdef ENABLE_PRESSURE_POINTS
#define INJECT_JUKEBOX(res, task) \
do { \
    if (res && (NFS_STATUS(res) == NFS3_OK)) { \
        if (inject_error()) { \
            if (task->rpc_api->is_dirop()) { \
                AZLogWarn("[{}/{}] PP: {} jukebox", \
                          task->rpc_api->get_parent_ino(), \
                          task->rpc_api->get_file_name(), \
                          __FUNCTION__); \
            } else { \
                AZLogWarn("[{}] PP: {} jukebox", \
                          task->rpc_api->get_ino(), __FUNCTION__); \
            } \
            (res)->status = NFS3ERR_JUKEBOX; \
        } \
    } \
} while (0)
#else
#define INJECT_JUKEBOX(res, task) /* nothing */
#endif

#ifdef ENABLE_PRESSURE_POINTS
#define INJECT_BAD_COOKIE(res, task) \
do { \
    /* \
     * Must be called only for READDIR and READDIRPLUS. \
     */ \
    assert(task->get_op_type() == FUSE_READDIR || \
           task->get_op_type() == FUSE_READDIRPLUS); \
    if (res && (NFS_STATUS(res) == NFS3_OK)) { \
        /* \
         * Don't simulate badcookie error for cookie==0. \
         */ \
        if (inject_error() && \
            task->rpc_api->readdir_task.get_offset() != 0) { \
            AZLogWarn("[{}] PP: {} bad cookie, offset {}, target_offset {} ", \
                       task->rpc_api->readdir_task.get_ino(), \
                       __FUNCTION__, \
                       task->rpc_api->readdir_task.get_offset(), \
                       task->rpc_api->readdir_task.get_target_offset()); \
            (res)->status = NFS3ERR_BAD_COOKIE; \
        } \
    } \
} while (0)
#else
#define INJECT_BAD_COOKIE(res, task) /* nothing */
#endif

#ifdef ENABLE_PRESSURE_POINTS
#define INJECT_CREATE_FH_POPULATE_FAILURE(res, task) \
do { \
    /* \
     * Must be called only for create and mknod task. \
     */ \
    assert(task->get_op_type() == FUSE_CREATE); \
    if (res && (NFS_STATUS(res) == NFS3_OK)) { \
        if (inject_error()) { \
            AZLogWarn("[{}] PP: {} failed to populate fh, parent ino {}, filename {} ", \
                       __FUNCTION__, \
                       task->rpc_api->create_task.get_parent_ino(), \
                       task->rpc_api->create_task.get_file_name()); \
            (res)->CREATE3res_u.resok.obj.handle_follows = 0; \
        } \
    } \
} while (0)
#else
#define INJECT_CREATE_FH_POPULATE_FAILURE(res, task) /* nothing */
#endif

#ifdef ENABLE_PRESSURE_POINTS
#define INJECT_MKNOD_FH_POPULATE_FAILURE(res, task) \
do { \
    /* \
     * Must be called only for mknod task. \
     */ \
    assert(task->get_op_type() == FUSE_MKNOD); \
    if (res && (NFS_STATUS(res) == NFS3_OK)) { \
        if (inject_error()) { \
            AZLogWarn("[{}] PP: {} failed to populate fh, parent ino {}, filename {} ", \
                       __FUNCTION__, \
                       task->rpc_api->mknod_task.get_parent_ino(), \
                       task->rpc_api->mknod_task.get_file_name()); \
            (res)->CREATE3res_u.resok.obj.handle_follows = 0; \
        } \
    } \
} while (0)
#else
#define INJECT_MKNOD_FH_POPULATE_FAILURE(res, task) /* nothing */
#endif

#ifdef ENABLE_PRESSURE_POINTS
#define INJECT_MKDIR_FH_POPULATE_FAILURE(res, task) \
do { \
    /* \
     * Must be called only for mkdir task. \
     */ \
    assert(task->get_op_type() == FUSE_MKDIR); \
    if (res && (NFS_STATUS(res) == NFS3_OK)) { \
        if (inject_error()) { \
            AZLogWarn("[{}] PP: {} failed to populate fh, parent ino {}, dirname {} ", \
                       __FUNCTION__, \
                       task->rpc_api->mkdir_task.get_parent_ino(), \
                       task->rpc_api->mkdir_task.get_dir_name()); \
            (res)->MKDIR3res_u.resok.obj.handle_follows = 0; \
        } \
    } \
} while (0)
#else
#define INJECT_MKDIR_FH_POPULATE_FAILURE(res, task) /* nothing */
#endif

#ifdef ENABLE_PRESSURE_POINTS
#define INJECT_SETATTR_FH_POPULATE_FAILURE(res, task) \
do { \
    /* \
     * Must be called only for setattr. \
     */ \
    assert(task->get_op_type() == FUSE_SETATTR); \
    if (res && (NFS_STATUS(res) == NFS3_OK)) { \
        if (inject_error()) { \
            AZLogWarn("[{}] PP: {} failed to populate fh, ino {}", \
                       __FUNCTION__, \
                       task->rpc_api->setattr_task.get_ino()); \
            (res)->SETATTR3res_u.resok.obj_wcc.after.attributes_follow = 0; \
        } \
    } \
} while (0)
#else
#define INJECT_SETATTR_FH_POPULATE_FAILURE(res, task) /* nothing */
#endif

/* static */
std::atomic<int> rpc_task::async_slots = MAX_ASYNC_RPC_TASKS;

/* static */
const std::string rpc_task::fuse_opcode_to_string(fuse_opcode opcode)
{
#define _case(op)   \
    case FUSE_##op: \
        return #op;

    switch (opcode) {
        _case(LOOKUP);
        _case(FORGET);
        _case(GETATTR);
        _case(SETATTR);
        _case(READLINK);
        _case(SYMLINK);
        _case(MKNOD);
        _case(MKDIR);
        _case(UNLINK);
        _case(RMDIR);
        _case(RENAME);
        _case(LINK);
        _case(OPEN);
        _case(READ);
        _case(WRITE);
        _case(STATFS);
        _case(RELEASE);
        _case(FSYNC);
        _case(SETXATTR);
        _case(GETXATTR);
        _case(LISTXATTR);
        _case(REMOVEXATTR);
        _case(FLUSH);
        _case(INIT);
        _case(OPENDIR);
        _case(READDIR);
        _case(RELEASEDIR);
        _case(FSYNCDIR);
        _case(GETLK);
        _case(SETLK);
        _case(SETLKW);
        _case(ACCESS);
        _case(CREATE);
        _case(INTERRUPT);
        _case(BMAP);
        _case(DESTROY);
        _case(IOCTL);
        _case(POLL);
        _case(NOTIFY_REPLY);
        _case(BATCH_FORGET);
        _case(FALLOCATE);
        _case(READDIRPLUS);
        _case(RENAME2);
        _case(LSEEK);
        _case(COPY_FILE_RANGE);
        _case(SETUPMAPPING);
        _case(REMOVEMAPPING);
#if 0
        _case(SYNCFS);
        _case(TMPFILE);
        _case(STATX);
#endif
        default:
            AZLogError("fuse_opcode_to_string: Unknown opcode {}", (int) opcode);
            return "Unknown";
    }
#undef _case
}

void rpc_task::init_lookup(fuse_req *request,
                           const char *name,
                           fuse_ino_t parent_ino)
{
    assert(get_op_type() == FUSE_LOOKUP);
    set_fuse_req(request);
    rpc_api->lookup_task.set_file_name(name);
    rpc_api->lookup_task.set_parent_ino(parent_ino);
    rpc_api->lookup_task.set_fuse_file(nullptr);

    fh_hash = get_client()->get_nfs_inode_from_ino(parent_ino)->get_crc();
}

void rpc_task::init_proxy_lookup(fuse_req *request,
                                 const char *name,
                                 fuse_ino_t parent_ino,
                                 enum fuse_opcode proxy_optype,
                                 fuse_file_info *fileinfo)
{
    init_lookup(request, name, parent_ino);

    rpc_api->lookup_task.set_fuse_file(fileinfo);
    set_proxy_op_type(proxy_optype);
}

void rpc_task::run_proxy_lookup()
{
    run_lookup();
}

void rpc_task::init_access(fuse_req *request,
                           fuse_ino_t ino,
                           int mask)
{
    assert(get_op_type() == FUSE_ACCESS);
    set_fuse_req(request);
    rpc_api->access_task.set_ino(ino);
    rpc_api->access_task.set_mask(mask);

    fh_hash = get_client()->get_nfs_inode_from_ino(ino)->get_crc();
}

void rpc_task::init_flush(fuse_req *request,
                          fuse_ino_t ino)
{
    assert(get_op_type() == FUSE_FLUSH);
    set_fuse_req(request);
    rpc_api->flush_task.set_ino(ino);

    fh_hash = get_client()->get_nfs_inode_from_ino(ino)->get_crc();
}

void rpc_task::init_write(fuse_req *request,
                          fuse_ino_t ino,
                          struct fuse_bufvec *bufv,
                          size_t size,
                          off_t offset)
{
    assert(get_op_type() == FUSE_WRITE);
    set_fuse_req(request);
    rpc_api->write_task.set_size(size);
    rpc_api->write_task.set_offset(offset);
    rpc_api->write_task.set_ino(ino);
    rpc_api->write_task.set_buffer_vector(bufv);

    fh_hash = get_client()->get_nfs_inode_from_ino(ino)->get_crc();
}

void rpc_task::init_getattr(fuse_req *request,
                            fuse_ino_t ino)
{
    assert(get_op_type() == FUSE_GETATTR);
    set_fuse_req(request);
    rpc_api->getattr_task.set_ino(ino);

    fh_hash = get_client()->get_nfs_inode_from_ino(ino)->get_crc();
}

void rpc_task::init_proxy_getattr(fuse_req *request,
                                  fuse_ino_t ino,
                                  enum fuse_opcode proxy_optype)
{
    init_getattr(request, ino);
    set_proxy_op_type(proxy_optype);
}

void rpc_task::init_statfs(fuse_req *request,
                           fuse_ino_t ino)
{
    assert(get_op_type() == FUSE_STATFS);
    set_fuse_req(request);
    rpc_api->statfs_task.set_ino(ino);

    fh_hash = get_client()->get_nfs_inode_from_ino(ino)->get_crc();
}

void rpc_task::init_create_file(fuse_req *request,
                                fuse_ino_t parent_ino,
                                const char *name,
                                mode_t mode,
                                struct fuse_file_info *file)
{
    assert(get_op_type() == FUSE_CREATE);
    set_fuse_req(request);

    const fuse_ctx *ctx = fuse_req_ctx(request);
    assert(ctx != nullptr);

    rpc_api->create_task.set_parent_ino(parent_ino);
    rpc_api->create_task.set_file_name(name);
    rpc_api->create_task.set_uid(ctx->uid);
    rpc_api->create_task.set_gid(ctx->gid);

    const mode_t effective_mode = (mode & (~ctx->umask));
    rpc_api->create_task.set_mode(effective_mode);

    rpc_api->create_task.set_fuse_file(file);

    fh_hash = get_client()->get_nfs_inode_from_ino(parent_ino)->get_crc();
}

void rpc_task::init_mknod(fuse_req *request,
                          fuse_ino_t parent_ino,
                          const char *name,
                          mode_t mode)
{
    assert(get_op_type() == FUSE_MKNOD);
    set_fuse_req(request);

    const fuse_ctx *ctx = fuse_req_ctx(request);
    assert(ctx != nullptr);

    rpc_api->mknod_task.set_parent_ino(parent_ino);
    rpc_api->mknod_task.set_file_name(name);
    rpc_api->mknod_task.set_uid(ctx->uid);
    rpc_api->mknod_task.set_gid(ctx->gid);

    const mode_t effective_mode = (mode & (~ctx->umask));
    rpc_api->mknod_task.set_mode(effective_mode);

    fh_hash = get_client()->get_nfs_inode_from_ino(parent_ino)->get_crc();
}

void rpc_task::init_mkdir(fuse_req *request,
                          fuse_ino_t parent_ino,
                          const char *name,
                          mode_t mode)
{
    assert(get_op_type() == FUSE_MKDIR);
    set_fuse_req(request);

    const fuse_ctx *ctx = fuse_req_ctx(request);
    assert(ctx != nullptr);

    rpc_api->mkdir_task.set_parent_ino(parent_ino);
    rpc_api->mkdir_task.set_dir_name(name);
    rpc_api->mkdir_task.set_uid(ctx->uid);
    rpc_api->mkdir_task.set_gid(ctx->gid);

    const mode_t effective_mode = (mode & (~ctx->umask));
    rpc_api->mkdir_task.set_mode(effective_mode);

    fh_hash = get_client()->get_nfs_inode_from_ino(parent_ino)->get_crc();
}

void rpc_task::init_unlink(fuse_req *request,
                           fuse_ino_t parent_ino,
                           const char *name,
                           bool for_silly_rename)
{
    assert(get_op_type() == FUSE_UNLINK);
    set_fuse_req(request);
    rpc_api->unlink_task.set_parent_ino(parent_ino);
    rpc_api->unlink_task.set_file_name(name);
    rpc_api->unlink_task.set_for_silly_rename(for_silly_rename);

    fh_hash = get_client()->get_nfs_inode_from_ino(parent_ino)->get_crc();
}

void rpc_task::init_rmdir(fuse_req *request,
                          fuse_ino_t parent_ino,
                          const char *name)
{
    assert(get_op_type() == FUSE_RMDIR);
    set_fuse_req(request);
    rpc_api->rmdir_task.set_parent_ino(parent_ino);
    rpc_api->rmdir_task.set_dir_name(name);

    fh_hash = get_client()->get_nfs_inode_from_ino(parent_ino)->get_crc();
}

void rpc_task::init_symlink(fuse_req *request,
                            const char *link,
                            fuse_ino_t parent_ino,
                            const char *name)
{
    assert(get_op_type() == FUSE_SYMLINK);
    set_fuse_req(request);

    const fuse_ctx *ctx = fuse_req_ctx(request);
    assert(ctx != nullptr);

    rpc_api->symlink_task.set_link(link);
    rpc_api->symlink_task.set_parent_ino(parent_ino);
    rpc_api->symlink_task.set_name(name);
    rpc_api->symlink_task.set_uid(ctx->uid);
    rpc_api->symlink_task.set_gid(ctx->gid);

    fh_hash = get_client()->get_nfs_inode_from_ino(parent_ino)->get_crc();
}

/*
 * silly_rename: Is this a silly rename (or user initiated rename)?
 * silly_rename_ino: Fuse inode number of the file being silly renamed.
 *                   Must be 0 if silly_rename is false.
 */
void rpc_task::init_rename(fuse_req *request,
                           fuse_ino_t parent_ino,
                           const char *name,
                           fuse_ino_t newparent_ino,
                           const char *newname,
                           bool silly_rename,
                           fuse_ino_t silly_rename_ino,
                           unsigned int flags)
{
    assert(get_op_type() == FUSE_RENAME);
    assert(silly_rename == (silly_rename_ino != 0));
    set_fuse_req(request);
    rpc_api->rename_task.set_parent_ino(parent_ino);
    rpc_api->rename_task.set_name(name);
    rpc_api->rename_task.set_newparent_ino(newparent_ino);
    rpc_api->rename_task.set_newname(newname);
    rpc_api->rename_task.set_silly_rename(silly_rename);
    rpc_api->rename_task.set_silly_rename_ino(silly_rename_ino);
    rpc_api->rename_task.set_flags(flags);

    /*
     * In case of cross-dir rename, we have to choose between
     * old and new dir to have the updated cache. We prefer
     * new_dir as that's where the user expects the file to
     * show up.
     */
    fh_hash = get_client()->get_nfs_inode_from_ino(newparent_ino)->get_crc();
}

void rpc_task::init_readlink(fuse_req *request,
                            fuse_ino_t ino)
{
    assert(get_op_type() == FUSE_READLINK);
    set_fuse_req(request);
    rpc_api->readlink_task.set_ino(ino);

    fh_hash = get_client()->get_nfs_inode_from_ino(ino)->get_crc();
}

void rpc_task::init_setattr(fuse_req *request,
                            fuse_ino_t ino,
                            const struct stat *attr,
                            int to_set,
                            struct fuse_file_info *file)
{
    assert(get_op_type() == FUSE_SETATTR);
    set_fuse_req(request);
    rpc_api->setattr_task.set_ino(ino);
    rpc_api->setattr_task.set_fuse_file(file);
    rpc_api->setattr_task.set_attribute_and_mask(attr, to_set);

    fh_hash = get_client()->get_nfs_inode_from_ino(ino)->get_crc();
}

void rpc_task::init_readdir(fuse_req *request,
                            fuse_ino_t ino,
                            size_t size,
                            off_t offset,
                            off_t target_offset,
                            struct fuse_file_info *file)
{
    assert(get_op_type() == FUSE_READDIR);
    set_fuse_req(request);
    rpc_api->readdir_task.set_ino(ino);
    rpc_api->readdir_task.set_size(size);
    rpc_api->readdir_task.set_offset(offset);
    rpc_api->readdir_task.set_target_offset(target_offset);
    rpc_api->readdir_task.set_fuse_file(file);

    fh_hash = get_client()->get_nfs_inode_from_ino(ino)->get_crc();
}

void rpc_task::init_readdirplus(fuse_req *request,
                                fuse_ino_t ino,
                                size_t size,
                                off_t offset,
                                off_t target_offset,
                                struct fuse_file_info *file)
{
    assert(get_op_type() == FUSE_READDIRPLUS);
    set_fuse_req(request);
    rpc_api->readdir_task.set_ino(ino);
    rpc_api->readdir_task.set_size(size);
    rpc_api->readdir_task.set_offset(offset);
    rpc_api->readdir_task.set_target_offset(target_offset);
    rpc_api->readdir_task.set_fuse_file(file);

    fh_hash = get_client()->get_nfs_inode_from_ino(ino)->get_crc();
}

void rpc_task::init_read(fuse_req *request,
                         fuse_ino_t ino,
                         size_t size,
                         off_t offset,
                         struct fuse_file_info *file)
{
    assert(get_op_type() == FUSE_READ);
    set_fuse_req(request);
    rpc_api->read_task.set_ino(ino);
    rpc_api->read_task.set_size(size);
    rpc_api->read_task.set_offset(offset);
    rpc_api->read_task.set_fuse_file(file);

    fh_hash = get_client()->get_nfs_inode_from_ino(ino)->get_crc();

    /*
     * We can perform round-robin for reads even for port 2048.
     *
     * TODO: Control this with a config.
     */
    set_csched(CONN_SCHED_RR);
}

/*
 * TODO: All the RPC callbacks where we receive post-op attributes or receive
 *       attributes o/w, we must call nfs_inode::update() to update the
 *       currently cached attributes. That will invalidate the cache if newly
 *       received attributes indicate file data has changed.
 * Update: This is done for success returns, but we need it for failed return
 *       too.
 */

static void getattr_callback(
    struct rpc_context *rpc,
    int rpc_status,
    void *data,
    void *private_data)
{
    rpc_task *task = (rpc_task*) private_data;
    assert(task->magic == RPC_TASK_MAGIC);

    auto res = (GETATTR3res*)data;

    INJECT_JUKEBOX(res, task);

    const fuse_ino_t ino =
        task->rpc_api->getattr_task.get_ino();
    struct nfs_inode *inode =
        task->get_client()->get_nfs_inode_from_ino(ino);
    const int status = task->status(rpc_status, NFS_STATUS(res));

    /*
     * Now that the request has completed, we can query libnfs for the
     * dispatch time.
     */
    task->get_stats().on_rpc_complete(rpc_get_pdu(rpc), NFS_STATUS(res));

    if (status == 0) {
        // Got fresh attributes, update the attributes cached in the inode.
        inode->update(&(res->GETATTR3res_u.resok.obj_attributes));

        /*
         * Set fuse kernel attribute cache timeout to the current attribute
         * cache timeout for this inode, as per the recent revalidation
         * experience.
         */
        task->reply_attr(inode->get_attr(), inode->get_actimeo());
    } else if (NFS_STATUS(res) == NFS3ERR_JUKEBOX) {
        task->get_client()->jukebox_retry(task);
    } else {
        task->reply_error(status);
    }
}

static void lookup_callback(
    struct rpc_context *rpc,
    int rpc_status,
    void *data,
    void *private_data)
{
    rpc_task *task = (rpc_task*) private_data;
    assert(task->magic == RPC_TASK_MAGIC);

    assert(task->rpc_api->optype == FUSE_LOOKUP);
    auto res = (LOOKUP3res*)data;

    INJECT_JUKEBOX(res, task);

    // Parent directory inode.
    const fuse_ino_t ino =
        task->rpc_api->lookup_task.get_parent_ino();
    struct nfs_inode *inode =
        task->get_client()->get_nfs_inode_from_ino(ino);
    const int status = task->status(rpc_status, NFS_STATUS(res));

    /*
     * Kernel must cache -ve entries.
     */
    const bool cache_negative =
        ((aznfsc_cfg.lookupcache_int == AZNFSCFG_LOOKUPCACHE_ALL) &&
        (task->get_proxy_op_type() == FUSE_LOOKUP));

    /*
     * Now that the request has completed, we can query libnfs for the
     * dispatch time.
     */
    task->get_stats().on_rpc_complete(rpc_get_pdu(rpc), NFS_STATUS(res));

    if (((rpc_status == RPC_STATUS_SUCCESS) &&
         (NFS_STATUS(res) == NFS3ERR_NOENT)) && cache_negative) {
        /*
         * Special case for creating negative dentry.
         */
        task->get_client()->reply_entry(
            task,
            nullptr /* fh */,
            nullptr /* fattr */,
            nullptr);
    } else if (status == 0) {
        /*
         * Update attributes of parent directory returned in postop
         * attributes.
         */
        UPDATE_INODE_ATTR(inode, res->LOOKUP3res_u.resok.dir_attributes);

        assert(res->LOOKUP3res_u.resok.obj_attributes.attributes_follow);
        task->get_client()->reply_entry(
            task,
            &res->LOOKUP3res_u.resok.object,
            &res->LOOKUP3res_u.resok.obj_attributes.post_op_attr_u.attributes,
            task->rpc_api->lookup_task.get_fuse_file());
    } else if (NFS_STATUS(res) == NFS3ERR_JUKEBOX) {
        task->get_client()->jukebox_retry(task);
    } else {
        task->reply_error(status);
    }
}

void access_callback(
    struct rpc_context *rpc,
    int rpc_status,
    void *data,
    void *private_data)
{
    rpc_task *task = (rpc_task*) private_data;
    assert(task->magic == RPC_TASK_MAGIC);

    auto res = (ACCESS3res*) data;

    INJECT_JUKEBOX(res, task);

    const fuse_ino_t ino =
        task->rpc_api->access_task.get_ino();
    struct nfs_inode *inode =
        task->get_client()->get_nfs_inode_from_ino(ino);
    const int status = task->status(rpc_status, NFS_STATUS(res));

    /*
     * Now that the request has completed, we can query libnfs for the
     * dispatch time.
     */
    task->get_stats().on_rpc_complete(rpc_get_pdu(rpc), NFS_STATUS(res));

    if (NFS_STATUS(res) == NFS3ERR_JUKEBOX) {
        task->get_client()->jukebox_retry(task);
    } else {
        if (status == 0) {
            UPDATE_INODE_ATTR(inode, res->ACCESS3res_u.resok.obj_attributes);
        }
        task->reply_error(status);
    }
}

/*
 * Called when libnfs completes a WRITE_IOV RPC.
 */
static void write_iov_callback(
    struct rpc_context *rpc,
    int rpc_status,
    void *data,
    void *private_data)
{
    AZLogDebug("write_iov_callback");
    assert(rpc != nullptr);

    struct rpc_task *task = (struct rpc_task *) private_data;
    assert(task->magic == RPC_TASK_MAGIC);
    // Only flush tasks use this callback.
    assert(task->get_op_type() == FUSE_FLUSH);

    // Those flush tasks must have pvt set to a bc_iovec ptr.
    struct bc_iovec *bciov = (struct bc_iovec *) task->rpc_api->pvt;
    assert(bciov);
    assert(bciov->magic == BC_IOVEC_MAGIC);

    struct nfs_client *client = task->get_client();
    assert(client->magic == NFS_CLIENT_MAGIC);

    auto res = (WRITE3res *)data;

    INJECT_JUKEBOX(res, task);

    const char* errstr;
    const int status = task->status(rpc_status, NFS_STATUS(res), &errstr);
    const fuse_ino_t ino = task->rpc_api->flush_task.get_ino();
    struct nfs_inode *inode = client->get_nfs_inode_from_ino(ino);

    /*
     * Now that the request has completed, we can query libnfs for the
     * dispatch time.
     */
    task->get_stats().on_rpc_complete(
        rpc_get_pdu(rpc),
        NFS_STATUS(res));

    // Success case.
    if (status == 0) {
        /*
         * WCC implementation.
         * If pre-op attributes indicate that the file changed since we cached,
         * it implies some other client updated the file. In this case the best
         * course of action is to drop our cached data. Note that we drop only
         * non-dirty data, anyways multiple client writing to the same file
         * w/o locking would result in undefined data state.
         */
        UPDATE_INODE_WCC(inode, res->WRITE3res_u.resok.file_wcc);

#ifdef ENABLE_PRESSURE_POINTS
        /*
         * Short write pressure point.
         */
        if (inject_error()) {
            // Set write size to a random percent of the actual size.
            const uint32_t pct = random_number(1, 100);
            const uint32_t adj_size =
                std::max((res->WRITE3res_u.resok.count * pct) / 100, 1U);
            assert(adj_size <= res->WRITE3res_u.resok.count);
            AZLogWarn("[{}] PP: short write {} -> {}",
                      ino, res->WRITE3res_u.resok.count, adj_size);
            res->WRITE3res_u.resok.count = adj_size;
        }
#endif
        // Successful Blob write must not return 0.
        assert(res->WRITE3res_u.resok.count > 0);
        assert(res->WRITE3res_u.resok.count <= bciov->length);
        assert(bciov->length <= bciov->orig_length);
        assert(bciov->offset >= bciov->orig_offset);

        /*
         * Did the write for the entire bciov complete?
         * Note that bciov is a vector of multiple bytes_chunk and for each
         * of them we write the entire membuf.
         */
        const bool is_partial_write =
            (res->WRITE3res_u.resok.count < bciov->length);

        if (is_partial_write) {
            AZLogDebug("[{}] Partial write: [{}, {}) of [{}, {})",
                       ino,
                       bciov->offset,
                       bciov->offset + res->WRITE3res_u.resok.count,
                       bciov->orig_offset,
                       bciov->orig_offset + bciov->orig_length);

            // Update bciov after the current write.
            bciov->on_io_complete(res->WRITE3res_u.resok.count);

            // Create a new flush_task for the remaining bc_iovec.
            struct rpc_task *flush_task =
                    client->get_rpc_task_helper()->alloc_rpc_task(FUSE_FLUSH);
            flush_task->init_flush(nullptr /* fuse_req */, ino);
            // Any new task should start fresh as a parent task.
            assert(flush_task->rpc_api->parent_task == nullptr);

            // Hand over the remaining bciov to the new flush_task.
            assert(flush_task->rpc_api->pvt == nullptr);
            flush_task->rpc_api->pvt = task->rpc_api->pvt;
            task->rpc_api->pvt = nullptr;

            // Issue write for the remaining data.
            flush_task->issue_write_rpc();

            /*
             * Release this task since it has done it's job.
             * Now next the callback will be called when the above partial
             * write completes.
             */
            task->free_rpc_task();

            return;
        } else {
            // Complete bc_iovec IO completed.
            bciov->on_io_complete(res->WRITE3res_u.resok.count);

            // Complete data writen to blob.
            AZLogDebug("[{}] Completed write, off: {}, len: {}",
                       ino, bciov->offset, bciov->length);
        }
    } else if (NFS_STATUS(res) == NFS3ERR_JUKEBOX) {
        AZLogDebug("[{}] JUKEBOX error write, off: {}, len: {}",
                   ino,
                   bciov->offset,
                   bciov->length);
        task->get_client()->jukebox_retry(task);
        return;
    } else {
        /*
         * Since the api failed and can no longer be retried, set write_error
         * and do not clear dirty flag.
         */
        AZLogError("[{}] Write [{}, {}) failed with status {}: {}",
                   ino,
                   bciov->offset,
                   bciov->length,
                   status, errstr);

        inode->set_write_error(status);

        /*
         * on_io_fail() will clear flushing from all remaining membufs.
         */
        bciov->on_io_fail();
    }

    delete bciov;
    task->rpc_api->pvt = nullptr;

    // Release the task.
    task->free_rpc_task();
}

bool rpc_task::add_bc(const bytes_chunk& bc)
{
    assert(get_op_type() == FUSE_FLUSH);

    struct bc_iovec *bciov = (struct bc_iovec *) rpc_api->pvt;
    assert(bciov->magic == BC_IOVEC_MAGIC);

    return bciov->add_bc(bc);
}

void rpc_task::issue_write_rpc()
{
    // Must only be called for a flush task.
    assert(get_op_type() == FUSE_FLUSH);

    const fuse_ino_t ino = rpc_api->flush_task.get_ino();
    struct nfs_inode *inode = get_client()->get_nfs_inode_from_ino(ino);
    struct bc_iovec *bciov = (struct bc_iovec *) rpc_api->pvt;
    assert(bciov->magic == BC_IOVEC_MAGIC);

    WRITE3args args;
    ::memset(&args, 0, sizeof(args));
    bool rpc_retry = false;
    const uint64_t offset = bciov->offset;
    const uint64_t length = bciov->length;

    assert(bciov->iovcnt > 0 && bciov->iovcnt <= BC_IOVEC_MAX_VECTORS);
    assert(offset < AZNFSC_MAX_FILE_SIZE);
    assert((offset + length) < AZNFSC_MAX_FILE_SIZE);
    assert(length > 0);

    AZLogDebug("issue_write_iovec offset:{}, length:{}", offset, length);
    args.file = inode->get_fh();
    args.offset = offset;
    args.count = length;
    args.stable = FILE_SYNC;

    do {
        rpc_retry = false;
        stats.on_rpc_issue();

        if (rpc_nfs3_writev_task(get_rpc_ctx(),
                                        write_iov_callback, &args,
                                        bciov->iov,
                                        bciov->iovcnt,
                                        this) == NULL) {
            stats.on_rpc_cancel();
            /*
             * Most common reason for this is memory allocation failure,
             * hence wait for some time before retrying. Also block the
             * current thread as we really want to slow down things.
             *
             * TODO: For soft mount should we fail this?
             */
            rpc_retry = true;

            AZLogWarn("rpc_nfs3_write_task failed to issue, retrying "
                        "after 5 secs!");
            ::sleep(5);
        }
    } while (rpc_retry);
}

static void statfs_callback(
    struct rpc_context *rpc,
    int rpc_status,
    void *data,
    void *private_data)
{
    rpc_task *task = (rpc_task*) private_data;
    assert(task->magic == RPC_TASK_MAGIC);

    auto res = (FSSTAT3res*)data;

    INJECT_JUKEBOX(res, task);

    const fuse_ino_t ino =
        task->rpc_api->statfs_task.get_ino();
    struct nfs_inode *inode =
        task->get_client()->get_nfs_inode_from_ino(ino);
    const int status = task->status(rpc_status, NFS_STATUS(res));

    /*
     * Now that the request has completed, we can query libnfs for the
     * dispatch time.
     */
    task->get_stats().on_rpc_complete(rpc_get_pdu(rpc), NFS_STATUS(res));

    if (status == 0) {
        UPDATE_INODE_ATTR(inode, res->FSSTAT3res_u.resok.obj_attributes);

        struct statvfs st;
        ::memset(&st, 0, sizeof(st));
        st.f_bsize = task->get_client()->mnt_options.wsize_adj;
        if (st.f_bsize < 4096) {
            st.f_bsize = 4096;
        }
        st.f_frsize = st.f_bsize;
        st.f_blocks = res->FSSTAT3res_u.resok.tbytes / st.f_bsize;
        st.f_bfree = res->FSSTAT3res_u.resok.fbytes / st.f_bsize;
        st.f_bavail = res->FSSTAT3res_u.resok.abytes / st.f_bsize;
        st.f_files = res->FSSTAT3res_u.resok.tfiles;
        st.f_ffree = res->FSSTAT3res_u.resok.ffiles;
        st.f_favail = res->FSSTAT3res_u.resok.afiles;
        st.f_namemax = NAME_MAX;

        task->reply_statfs(&st);
    } else if (NFS_STATUS(res) == NFS3ERR_JUKEBOX) {
        task->get_client()->jukebox_retry(task);
    } else {
        task->reply_error(status);
    }
}

static void createfile_callback(
    struct rpc_context *rpc,
    int rpc_status,
    void *data,
    void *private_data)
{
    rpc_task *task = (rpc_task*) private_data;
    assert(task->magic == RPC_TASK_MAGIC);

    auto res = (CREATE3res*)data;

#if 0
    /*
     * Don't inject jukebox for non-idempotent requests.
     */
    INJECT_JUKEBOX(res, task);
#endif

    const fuse_ino_t ino =
        task->rpc_api->create_task.get_parent_ino();
    struct nfs_inode *inode =
        task->get_client()->get_nfs_inode_from_ino(ino);
    const int status = task->status(rpc_status, NFS_STATUS(res));

    INJECT_CREATE_FH_POPULATE_FAILURE(res, task);

    /*
     * Now that the request has completed, we can query libnfs for the
     * dispatch time.
     */
    task->get_stats().on_rpc_complete(rpc_get_pdu(rpc), NFS_STATUS(res));

    if (status == 0) {
        if (!res->CREATE3res_u.resok.obj.handle_follows) {
            /*
             * If the server doesn't send the filehandle (which is perfectly
             * valid), make a LOOKUP RPC request to query the filehandle and
             * pass that to fuse.
             */
            AZLogWarn("CreateFile failed to return filehandle, req={}, "
                "parent_ino={}, name={}. Issuing lookup.",
                fmt::ptr(task->get_fuse_req()),
                task->rpc_api->create_task.get_parent_ino(),
                task->rpc_api->create_task.get_file_name());

            struct rpc_task *proxy_lookup_tsk =
                task->get_client()->get_rpc_task_helper()->alloc_rpc_task(
                    FUSE_LOOKUP);
            proxy_lookup_tsk->init_proxy_lookup(
                task->get_fuse_req(),
                task->rpc_api->create_task.get_file_name(),
                task->rpc_api->create_task.get_parent_ino(),
                FUSE_CREATE,
                task->rpc_api->create_task.get_fuse_file());
            proxy_lookup_tsk->run_proxy_lookup();

            /*
             * Free the current task as the response will be sent by the
             * lookup task made below.
             * Note: task should not be accessed after this.
             */
            task->free_rpc_task();

            return;
        }

        /*
         * See comment above readdirectory_cache::lookuponly, why we don't need
         * to call UPDATE_INODE_ATTR() to invalidate the readdirectory_cache,
         * even though we cannot correctly update our readdir cache with the
         * newly created file (as the readdir cache also needs the cookie to
         * be filled which only server can return in a READDIR{PLUS} response.
         *
         * If userspace attribute cache is disabled we use UPDATE_INODE_ATTR()
         * which will force the parent directory readdir cache to be
         * invalidated as directory mtime after this create operation would
         * have changed.
         */
        if (aznfsc_cfg.cache.attr.user.enable) {
            UPDATE_INODE_WCC(inode, res->CREATE3res_u.resok.dir_wcc);
        } else {
            UPDATE_INODE_ATTR(inode, res->CREATE3res_u.resok.dir_wcc.after);
        }

        task->get_client()->reply_entry(
            task,
            &res->CREATE3res_u.resok.obj.post_op_fh3_u.handle,
            &res->CREATE3res_u.resok.obj_attributes.post_op_attr_u.attributes,
            task->rpc_api->create_task.get_fuse_file());
    } else if (NFS_STATUS(res) == NFS3ERR_JUKEBOX) {
        task->get_client()->jukebox_retry(task);
    } else {
        task->reply_error(status);
    }
}

static void setattr_callback(
    struct rpc_context *rpc,
    int rpc_status,
    void *data,
    void *private_data)
{
    rpc_task *task = (rpc_task*) private_data;
    assert(task->magic == RPC_TASK_MAGIC);

    auto res = (SETATTR3res*)data;

    INJECT_JUKEBOX(res, task);

    const fuse_ino_t ino =
        task->rpc_api->setattr_task.get_ino();
    struct nfs_inode *inode =
        task->get_client()->get_nfs_inode_from_ino(ino);
    const int status = task->status(rpc_status, NFS_STATUS(res));

    /*
     * Now that the request has completed, we can query libnfs for the
     * dispatch time.
     */
    task->get_stats().on_rpc_complete(rpc_get_pdu(rpc), NFS_STATUS(res));

    INJECT_SETATTR_FH_POPULATE_FAILURE(res, task);

    if (status == 0) {
        if (!res->SETATTR3res_u.resok.obj_wcc.after.attributes_follow) {
            /* 
             * For NFS, the postop attributes are optional, but fuse expects
             * us to pass attributes in the callback. If NFS server fails to
             * return the postop attributes, make a GETATTR RPC request to
             * query the attributes and pass that to fuse.
             */
            AZLogWarn("Setattr failed to return postop req={}, ino={}."
                " Issuing getattr RPC to fetch post-op attributes.",
                fmt::ptr(task->get_fuse_req()),
                task->rpc_api->setattr_task.get_ino());

            struct rpc_task *proxy_getattr_tsk =
                task->get_client()->get_rpc_task_helper()->alloc_rpc_task(
                FUSE_GETATTR);
            proxy_getattr_tsk->init_proxy_getattr(
                task->get_fuse_req(),
                task->rpc_api->setattr_task.get_ino(),
                FUSE_SETATTR);
            proxy_getattr_tsk->run_proxy_getattr();

            /*
             * Free the current task as the response will be sent by the
             * getattr task made below.
             * Note: task should not be accessed after this.
             */
            task->free_rpc_task();

            return;
        }

        UPDATE_INODE_WCC(inode, res->SETATTR3res_u.resok.obj_wcc);

        struct stat st = {};

        /*
         * TODO For NFS the postop attributes are optional, but fuse expects
         *      us to pass attributes in the callback. If NFS server fails to
         *      return the postop attributes we must query the attributes using
         *      a GETATTR RPC.
         */
        assert(res->SETATTR3res_u.resok.obj_wcc.after.attributes_follow);
        nfs_client::stat_from_fattr3(
            st, res->SETATTR3res_u.resok.obj_wcc.after.post_op_attr_u.attributes);

        /*
         * Set fuse kernel attribute cache timeout to the current attribute
         * cache timeout for this inode, as per the recent revalidation
         * experience.
         */
        task->reply_attr(st, inode->get_actimeo());
    } else if (NFS_STATUS(res) == NFS3ERR_JUKEBOX) {
        task->get_client()->jukebox_retry(task);
    } else {
        task->reply_error(status);
    }
}

void mknod_callback(
    struct rpc_context *rpc,
    int rpc_status,
    void *data,
    void *private_data)
{
    rpc_task *task = (rpc_task*) private_data;
    assert(task->magic == RPC_TASK_MAGIC);

    auto res = (CREATE3res*)data;

#if 0
    /*
     * Don't inject jukebox for non-idempotent requests.
     */
    INJECT_JUKEBOX(res, task);
#endif

    const fuse_ino_t ino =
        task->rpc_api->mknod_task.get_parent_ino();
    struct nfs_inode *inode =
        task->get_client()->get_nfs_inode_from_ino(ino);
    const int status = task->status(rpc_status, NFS_STATUS(res));

    INJECT_MKNOD_FH_POPULATE_FAILURE(res, task);

    /*
     * Now that the request has completed, we can query libnfs for the
     * dispatch time.
     */
    task->get_stats().on_rpc_complete(rpc_get_pdu(rpc), NFS_STATUS(res));

    if (status == 0) {
        if (!res->CREATE3res_u.resok.obj.handle_follows) {
            /*
             * If the server doesn't send the filehandle (which is perfectly
             * valid), make a LOOKUP RPC request to query the filehandle and
             * pass that to fuse.
             */
            AZLogWarn("mknod failed to return filehandle, req={}, "
                "parent_ino={}, name={}. Issuing lookup.",
                fmt::ptr(task->get_fuse_req()),
                task->rpc_api->mknod_task.get_parent_ino(),
                task->rpc_api->mknod_task.get_file_name());

            struct rpc_task *proxy_lookup_tsk =
                    task->get_client()->get_rpc_task_helper()->alloc_rpc_task(
                        FUSE_LOOKUP);
            proxy_lookup_tsk->init_proxy_lookup(
                task->get_fuse_req(),
                task->rpc_api->mknod_task.get_file_name(),
                task->rpc_api->mknod_task.get_parent_ino(),
                FUSE_MKNOD);
            proxy_lookup_tsk->run_proxy_lookup();

            /*
             * Free the current task as the response will be sent by the
             * lookup task made below.
             * Note: task should not be accessed after this.
             */
            task->free_rpc_task();

            return;
        }

        /*
         * See comment above readdirectory_cache::lookuponly, why we don't need
         * to call UPDATE_INODE_ATTR() to invalidate the readdirectory_cache,
         * even though we cannot correctly update our readdir cache with the
         * newly created file (as the readdir cache also needs the cookie to
         * be filled which only server can return in a READDIR{PLUS} response.
         */
        if (aznfsc_cfg.cache.attr.user.enable) {
            UPDATE_INODE_WCC(inode, res->CREATE3res_u.resok.dir_wcc);
        } else {
            UPDATE_INODE_ATTR(inode, res->CREATE3res_u.resok.dir_wcc.after);
        }

        task->get_client()->reply_entry(
            task,
            &res->CREATE3res_u.resok.obj.post_op_fh3_u.handle,
            &res->CREATE3res_u.resok.obj_attributes.post_op_attr_u.attributes,
            nullptr);
    } else if (NFS_STATUS(res) == NFS3ERR_JUKEBOX) {
        task->get_client()->jukebox_retry(task);
    } else {
        task->reply_error(status);
    }
}

void mkdir_callback(
    struct rpc_context *rpc,
    int rpc_status,
    void *data,
    void *private_data)
{
    rpc_task *task = (rpc_task*) private_data;
    assert(task->magic == RPC_TASK_MAGIC);

    auto res = (MKDIR3res*)data;

#if 0
    /*
     * Don't inject jukebox for non-idempotent requests.
     */
    INJECT_JUKEBOX(res, task);
#endif

    const fuse_ino_t ino =
        task->rpc_api->mkdir_task.get_parent_ino();
    struct nfs_inode *inode =
        task->get_client()->get_nfs_inode_from_ino(ino);
    const int status = task->status(rpc_status, NFS_STATUS(res));

    INJECT_MKDIR_FH_POPULATE_FAILURE(res, task);

    /*
     * Now that the request has completed, we can query libnfs for the
     * dispatch time.
     */
    task->get_stats().on_rpc_complete(rpc_get_pdu(rpc), NFS_STATUS(res));

    if (status == 0) {
        if (!res->MKDIR3res_u.resok.obj.handle_follows) {
            /*
             * If the server doesn't send the filehandle (which is perfectly
             * valid), make a LOOKUP RPC request to query the filehandle and
             * pass that to fuse.
             */
            AZLogWarn("mkdir failed to return filehandle, req={}, "
                "parent_ino={}, name={}. Issuing lookup.",
                fmt::ptr(task->get_fuse_req()),
                task->rpc_api->mkdir_task.get_parent_ino(),
                task->rpc_api->mkdir_task.get_dir_name());

            struct rpc_task *proxy_lookup_tsk =
                    task->get_client()->get_rpc_task_helper()->alloc_rpc_task(
                        FUSE_LOOKUP);
            proxy_lookup_tsk->init_proxy_lookup(
                task->get_fuse_req(),
                task->rpc_api->mkdir_task.get_dir_name(),
                task->rpc_api->mkdir_task.get_parent_ino(),
                FUSE_MKDIR);
            proxy_lookup_tsk->run_proxy_lookup();

            /*
             * Free the current task as the response will be sent by the
             * lookup task made below.
             * Note: task should not be accessed after this.
             */
            task->free_rpc_task();

            return;
        }
        /*
         * See comment above readdirectory_cache::lookuponly, why we don't need
         * to call UPDATE_INODE_ATTR() to invalidate the readdirectory_cache,
         * even though we cannot correctly update our readdir cache with the
         * newly created file (as the readdir cache also needs the cookie to
         * be filled which only server can return in a READDIR{PLUS} response.
         */
        if (aznfsc_cfg.cache.attr.user.enable) {
            UPDATE_INODE_WCC(inode, res->MKDIR3res_u.resok.dir_wcc);
        } else {
            UPDATE_INODE_ATTR(inode, res->MKDIR3res_u.resok.dir_wcc.after);
        }

        task->get_client()->reply_entry(
            task,
            &res->MKDIR3res_u.resok.obj.post_op_fh3_u.handle,
            &res->MKDIR3res_u.resok.obj_attributes.post_op_attr_u.attributes,
            nullptr);
    } else if (NFS_STATUS(res) == NFS3ERR_JUKEBOX) {
        task->get_client()->jukebox_retry(task);
    } else {
        task->reply_error(status);
    }
}

void unlink_callback(
    struct rpc_context *rpc,
    int rpc_status,
    void *data,
    void *private_data)
{
    rpc_task *task = (rpc_task*) private_data;
    assert(task->magic == RPC_TASK_MAGIC);

    auto res = (REMOVE3res*)data;

#if 0
    /*
     * Don't inject jukebox for non-idempotent requests.
     */
    INJECT_JUKEBOX(res, task);
#endif

    const fuse_ino_t parent_ino =
        task->rpc_api->unlink_task.get_parent_ino();
    struct nfs_inode *parent_inode =
        task->get_client()->get_nfs_inode_from_ino(parent_ino);
    // Are we unlinking a silly-renamed file?
    const bool for_silly_rename =
        task->rpc_api->unlink_task.get_for_silly_rename();
    const int status = task->status(rpc_status, NFS_STATUS(res));

    /*
     * Now that the request has completed, we can query libnfs for the
     * dispatch time.
     */
    task->get_stats().on_rpc_complete(rpc_get_pdu(rpc), NFS_STATUS(res));

    if (NFS_STATUS(res) == NFS3ERR_JUKEBOX) {
        task->get_client()->jukebox_retry(task);
    } else {
        if (status == 0) {
            /*
             * See comment above readdirectory_cache::lookuponly, why we don't
             * need to call UPDATE_INODE_ATTR() to invalidate the
             * readdirectory_cache.
             */
            if (aznfsc_cfg.cache.attr.user.enable) {
                UPDATE_INODE_WCC(parent_inode, res->REMOVE3res_u.resok.dir_wcc);
            } else {
                UPDATE_INODE_ATTR(parent_inode, res->REMOVE3res_u.resok.dir_wcc.after);
            }
        }

        task->reply_error(status);

        /*
         * Drop parent directory refcnt taken in rename_callback().
         * Note that we drop the refcnt irrespective of the unlink status.
         * This is done as fuse ignores any error returns from release()
         * which means the inode will be forgotten and hence we must drop
         * the parent directory inode ref which was taken to have a valid
         * parent directory inode till the child inode is present.
         * For jukebox we will retry the rename and drop the parent dir
         * ref when the unlink completes.
         */
        if (for_silly_rename) {
            parent_inode->decref();
        }
    }
}

void rmdir_callback(
    struct rpc_context *rpc,
    int rpc_status,
    void *data,
    void *private_data)
{
    rpc_task *task = (rpc_task*) private_data;
    assert(task->magic == RPC_TASK_MAGIC);

    auto res = (RMDIR3res*) data;

#if 0
    /*
     * Don't inject jukebox for non-idempotent requests.
     */
    INJECT_JUKEBOX(res, task);
#endif

    const fuse_ino_t ino =
        task->rpc_api->rmdir_task.get_parent_ino();
    struct nfs_inode *inode =
        task->get_client()->get_nfs_inode_from_ino(ino);
    const int status = task->status(rpc_status, NFS_STATUS(res));

    /*
     * Now that the request has completed, we can query libnfs for the
     * dispatch time.
     */
    task->get_stats().on_rpc_complete(rpc_get_pdu(rpc), NFS_STATUS(res));

    if (NFS_STATUS(res) == NFS3ERR_JUKEBOX) {
        task->get_client()->jukebox_retry(task);
    } else {
        if (status == 0) {
            /*
             * See comment above readdirectory_cache::lookuponly, why we don't
             * need to call UPDATE_INODE_ATTR() to invalidate the
             * readdirectory_cache.
             */
            if (aznfsc_cfg.cache.attr.user.enable) {
                UPDATE_INODE_WCC(inode, res->RMDIR3res_u.resok.dir_wcc);
            } else {
                UPDATE_INODE_ATTR(inode, res->RMDIR3res_u.resok.dir_wcc.after);
            }
        }
        task->reply_error(status);
    }
}

void symlink_callback(
    struct rpc_context *rpc,
    int rpc_status,
    void *data,
    void *private_data)
{
    rpc_task *task = (rpc_task*) private_data;
    assert(task->magic == RPC_TASK_MAGIC);

    auto res = (SYMLINK3res*) data;

#if 0
    /*
     * Don't inject jukebox for non-idempotent requests.
     */
    INJECT_JUKEBOX(res, task);
#endif

    const fuse_ino_t ino =
        task->rpc_api->symlink_task.get_parent_ino();
    struct nfs_inode *inode =
        task->get_client()->get_nfs_inode_from_ino(ino);
    const int status = task->status(rpc_status, NFS_STATUS(res));

    /*
     * Now that the request has completed, we can query libnfs for the
     * dispatch time.
     */
    task->get_stats().on_rpc_complete(rpc_get_pdu(rpc), NFS_STATUS(res));

    if (status == 0) {
        /*
         * See comment above readdirectory_cache::lookuponly, why we don't need
         * to call UPDATE_INODE_ATTR() to invalidate the readdirectory_cache,
         * even though we cannot correctly update our readdir cache with the
         * newly created file (as the readdir cache also needs the cookie to
         * be filled which only server can return in a READDIR{PLUS} response.
         */
        if (aznfsc_cfg.cache.attr.user.enable) {
            UPDATE_INODE_WCC(inode, res->SYMLINK3res_u.resok.dir_wcc);
        } else {
            UPDATE_INODE_ATTR(inode, res->SYMLINK3res_u.resok.dir_wcc.after);
        }

        assert(
            res->SYMLINK3res_u.resok.obj.handle_follows &&
            res->SYMLINK3res_u.resok.obj_attributes.attributes_follow);

        task->get_client()->reply_entry(
            task,
            &res->SYMLINK3res_u.resok.obj.post_op_fh3_u.handle,
            &res->SYMLINK3res_u.resok.obj_attributes.post_op_attr_u.attributes,
            nullptr);
    } else if (NFS_STATUS(res) == NFS3ERR_JUKEBOX) {
        task->get_client()->jukebox_retry(task);
    } else {
        task->reply_error(status);
    }
}

void rename_callback(
    struct rpc_context *rpc,
    int rpc_status,
    void *data,
    void *private_data)
{
    rpc_task *task = (rpc_task*) private_data;
    assert(task->magic == RPC_TASK_MAGIC);

    assert(task->rpc_api->optype == FUSE_RENAME);
    struct nfs_client *client = task->get_client();
    const fuse_ino_t parent_ino = task->rpc_api->rename_task.get_parent_ino();
    struct nfs_inode *parent_inode = client->get_nfs_inode_from_ino(parent_ino);
    const fuse_ino_t newparent_ino = task->rpc_api->rename_task.get_newparent_ino();
    struct nfs_inode *newparent_inode = client->get_nfs_inode_from_ino(newparent_ino);

    const bool silly_rename = task->rpc_api->rename_task.get_silly_rename();
    auto res = (RENAME3res*) data;

#if 0
    /*
     * Don't inject jukebox for non-idempotent requests.
     */
    INJECT_JUKEBOX(res, task);
#endif

    const int status = task->status(rpc_status, NFS_STATUS(res));

    /*
     * Now that the request has completed, we can query libnfs for the
     * dispatch time.
     */
    task->get_stats().on_rpc_complete(rpc_get_pdu(rpc), NFS_STATUS(res));

    /*
     * If this rename is a silly rename for an unlink operation, we need to
     * store the directory inode and the renamed filename so that we can
     * delete the silly renamed file when the last open count on this inode
     * is dropped.
     *
     * Note: Silly rename is done in response to a user unlink call and VFS
     *       holds the inode lock for the duration of the unlink, which means
     *       we will not get any other call for this inode, so we can safely
     *       access the inode w/o lock.
     */
    if (status == 0 && silly_rename) {
        const fuse_ino_t silly_rename_ino =
            task->rpc_api->rename_task.get_silly_rename_ino();
        assert(client->magic == NFS_CLIENT_MAGIC);
        struct nfs_inode *silly_rename_inode =
            client->get_nfs_inode_from_ino(silly_rename_ino);
        assert(silly_rename_inode->magic == NFS_INODE_MAGIC);

        // Silly rename has the same source and target dir.
        assert(parent_ino == newparent_ino);

        silly_rename_inode->silly_renamed_name =
            task->rpc_api->rename_task.get_newname();
        silly_rename_inode->parent_ino =
            task->rpc_api->rename_task.get_newparent_ino();
        silly_rename_inode->is_silly_renamed = true;

        /*
         * Successfully (silly)renamed, hold a ref on the parent directory
         * inode so that it doesn't go away until we have deleted the
         * silly-renamed file. This ref is dropped in unlink_callback().
         */
        parent_inode->incref();

        AZLogInfo("[{}] Silly rename successfully completed! "
                  "to-delete: {}/{}",
                  silly_rename_ino,
                  silly_rename_inode->parent_ino,
                  silly_rename_inode->silly_renamed_name);

#ifdef ENABLE_PARANOID
        assert(silly_rename_inode->silly_renamed_name.find(".nfs") == 0);
#endif
    }

    if (NFS_STATUS(res) == NFS3ERR_JUKEBOX) {
        task->get_client()->jukebox_retry(task);
    } else {
        if (status == 0) {
            /*
             * We cannot use UPDATE_INODE_WCC() here as we cannot update our
             * readdir cache with the newly created file/dir, as the readdir
             * cache also needs the cookie to be filled which only server can
             * return.
             * So we cause the cache to be invalidated.
             */
            UPDATE_INODE_ATTR(parent_inode,
                              res->RENAME3res_u.resok.fromdir_wcc.after);

            if (newparent_ino != parent_ino) {
                UPDATE_INODE_ATTR(newparent_inode,
                                  res->RENAME3res_u.resok.todir_wcc.after);
            }
        }
        task->reply_error(status);
    }
}

void readlink_callback(
    struct rpc_context *rpc,
    int rpc_status,
    void *data,
    void *private_data)
{
    rpc_task *task = (rpc_task*) private_data;
    assert(task->magic == RPC_TASK_MAGIC);

    auto res = (READLINK3res*) data;

    const fuse_ino_t ino =
        task->rpc_api->readlink_task.get_ino();
    struct nfs_inode *inode =
        task->get_client()->get_nfs_inode_from_ino(ino);
    INJECT_JUKEBOX(res, task);

    const int status = task->status(rpc_status, NFS_STATUS(res));

    /*
     * Now that the request has completed, we can query libnfs for the
     * dispatch time.
     */
    task->get_stats().on_rpc_complete(rpc_get_pdu(rpc), NFS_STATUS(res));

    if (status == 0) {
        UPDATE_INODE_ATTR(inode, res->READLINK3res_u.resok.symlink_attributes);

        task->reply_readlink(res->READLINK3res_u.resok.data);
    } else if (NFS_STATUS(res) == NFS3ERR_JUKEBOX) {
        task->get_client()->jukebox_retry(task);
    } else {
        task->reply_error(status);
    }
}

void rpc_task::run_lookup()
{
    fuse_ino_t parent_ino = rpc_api->lookup_task.get_parent_ino();
    struct nfs_inode *inode = get_client()->get_nfs_inode_from_ino(parent_ino);
    bool rpc_retry;
    const char *const filename = (char*) rpc_api->lookup_task.get_file_name();

    INC_GBL_STATS(tot_lookup_reqs, 1);

    /*
     * Lookup dnlc to see if we have valid cached lookup data.
     */
    if (aznfsc_cfg.cache.attr.user.enable) {
        bool negative_confirmed = false;
        struct nfs_inode *child_inode =
            inode->dnlc_lookup(filename, &negative_confirmed);
        if (child_inode) {
            AZLogDebug("[{}/{}] Returning cached lookup, child_ino={}",
                    parent_ino, filename, child_inode->get_fuse_ino());

            INC_GBL_STATS(lookup_served_from_cache, 1);

            struct fattr3 fattr;
            child_inode->fattr3_from_stat(fattr);
            get_client()->reply_entry(
                this,
                &child_inode->get_fh(),
                &fattr,
                rpc_api->lookup_task.get_fuse_file());

            // Drop the ref held by dnlc_lookup().
            child_inode->decref();
            return;
        } else if (negative_confirmed &&
            (get_proxy_op_type() == FUSE_LOOKUP)) {
            AZLogDebug("[{}/{}] Returning cached lookup (negative)",
                    parent_ino, filename);

            INC_GBL_STATS(lookup_served_from_cache, 1);
            get_client()->reply_entry(this,
                    nullptr /* fh */,
                    nullptr /* fattr */,
                    nullptr /* file */);
            return;
        }
    }

    do {
        LOOKUP3args args;
        args.what.dir = inode->get_fh();
        args.what.name = (char *) filename;

        rpc_retry = false;
        /*
         * Note: Once we call the libnfs async method, the callback can get
         *       called anytime after that, even before it returns to the
         *       caller. Since callback can free the task, it's not safe to
         *       access the task object after making the libnfs call.
         */
        stats.on_rpc_issue();
        if (rpc_nfs3_lookup_task(get_rpc_ctx(), lookup_callback, &args,
                                 this) == NULL) {
            stats.on_rpc_cancel();
            /*
             * Most common reason for this is memory allocation failure,
             * hence wait for some time before retrying. Also block the
             * current thread as we really want to slow down things.
             *
             * TODO: For soft mount should we fail this?
             */
            rpc_retry = true;

            AZLogWarn("rpc_nfs3_lookup_task failed to issue, retrying "
                      "after 5 secs!");
            ::sleep(5);
        }
    } while (rpc_retry);
}

void rpc_task::run_access()
{
    const fuse_ino_t ino = rpc_api->access_task.get_ino();
    bool rpc_retry;

    do {
        ACCESS3args args;
        args.object = get_client()->get_nfs_inode_from_ino(ino)->get_fh();
        args.access = rpc_api->access_task.get_mask();

        rpc_retry = false;
        stats.on_rpc_issue();
        if (rpc_nfs3_access_task(get_rpc_ctx(), access_callback, &args,
                                        this) == NULL) {
            stats.on_rpc_cancel();
            /*
             * Most common reason for this is memory allocation failure,
             * hence wait for some time before retrying. Also block the
             * current thread as we really want to slow down things.
             *
             * TODO: For soft mount should we fail this?
             */
            rpc_retry = true;

            AZLogWarn("rpc_nfs3_access_task failed to issue, retrying "
                      "after 5 secs!");
            ::sleep(5);
        }
    } while (rpc_retry);
}



void rpc_task::run_write()
{
    const fuse_ino_t ino = rpc_api->write_task.get_ino();
    struct nfs_inode *const inode = get_client()->get_nfs_inode_from_ino(ino);
    const size_t length = rpc_api->write_task.get_size();
    struct fuse_bufvec *const bufv = rpc_api->write_task.get_buffer_vector();
    const off_t offset = rpc_api->write_task.get_offset();
    uint64_t extent_left = 0;
    uint64_t extent_right = 0;

    /*
     * aznfsc_ll_write_buf() can only be called after aznfsc_ll_open() so
     * filecache must have been allocated when we reach here.
     */
    assert(inode->has_filecache());

    // Update cached write timestamp, if needed.
    inode->stamp_cached_write();

    /*
     * Don't issue new writes if some previous write had failed with error.
     * Since we store the write_error insid the nfs_inode it'll remain set
     * till this inode is forgotten and released and then application does
     * a fresh open to get a new nfs_inode.
     *
     * TODO: Shall we clear the write_error after conveying to the application
     *       once?
     */
    int error_code = inode->get_write_error();
    if (error_code != 0) {
        AZLogWarn("[{}] Previous write to this Blob failed with error={}, "
                  "skipping new write!", ino, error_code);

        reply_error(error_code);
        return;
    }

    /*
     * Fuse doesn't let us decide the max file size supported, so kernel can
     * technically send us a request for an offset larger than we support.
     * Failing here is the best response we can give.
     */
    if ((offset + length) > AZNFSC_MAX_FILE_SIZE) {
        AZLogWarn("[{}] Write beyond maximum file size suported ({}), "
                  "offset={}, length={}",
                  ino, AZNFSC_MAX_FILE_SIZE, offset, length);

        reply_error(EFBIG);
        return;
    }

    /*
     * Copy application data into chunk cache and initiate writes for all
     * membufs. We don't wait for the writes to actually finish, which means
     * we support buffered writes.
     * Note that copy_to_cache() may return EAGAIN in the rare case where this
     * writer thread races with another thread trying to read the same membuf.
     * The membuf is bigger than what the writer wants to write and is not
     * uptodate, so the writer needs to wait for the the reader to read into
     * the membuf and mark it uptodate before it can update the part it wants
     * to write. The more common case is that the reader reads into the membuf,
     * marks it uptodate and then writer gets the lock and proceeds, but it's
     * possible that reader cannot complete the read (most likely reason being
     * the file ends before the membuf). In this case copy_to_cache() fails with
     * EAGAIN so that we can repeat the whole process right from getting the
     * membufs. We do it for 10 times before failing the write, as it's highly
     * unlikely that we need to repeat more than that.
     */
    for (int i = 0; i < 10; i++) {
        error_code = inode->copy_to_cache(bufv, offset,
                                          &extent_left, &extent_right);
        if (error_code != EAGAIN) {
            break;
        }

        AZLogWarn("[{}] copy_to_cache(offset={}) failed with EAGAIN, retrying",
                  ino, offset);
    }

    if (error_code != 0) {
        AZLogWarn("[{}] copy_to_cache failed with error={}, "
                  "failing write!", ino, error_code);

        if (error_code == EAGAIN) {
            error_code = EIO;
        }
        reply_error(error_code);
        return;
    }

    assert(extent_right >= (extent_left + length));

    /*
     * If the extent size exceeds the max allowed dirty size as returned by
     * max_dirty_extent_bytes(), then it's time to flush the extent.
     * Note that this will cause sequential writes to be flushed at just the
     * right intervals to optimize fewer write calls and also allowing the
     * server scheduler to merge better.
     * See bytes_to_flush for how random writes are flushed.
     *
     * Note: max_dirty_extent is static as it doesn't change after it's
     *       queried for the first time.
     */
    static const uint64_t max_dirty_extent =
        inode->get_filecache()->max_dirty_extent_bytes();
    assert(max_dirty_extent > 0);

    /*
     * How many bytes in the cache need to be flushed.
     */
    const uint64_t bytes_to_flush =
        inode->get_filecache()->get_bytes_to_flush();

    AZLogDebug("[{}] extent_left: {}, extent_right: {}, size: {}, "
               "bytes_to_flush: {} (max_dirty_extent: {})",
               ino, extent_left, extent_right,
               (extent_right - extent_left),
               bytes_to_flush,
               max_dirty_extent);

    if ((extent_right - extent_left) < max_dirty_extent) {
        /*
         * Current extent is not big enough to be flushed, see if we have
         * enough dirty data that needs to be flushed. This is to cause
         * random writes to be periodically flushed.
         */
        if (bytes_to_flush < max_dirty_extent) {
            AZLogDebug("Reply write without syncing to Blob");
            reply_write(length);
            return;
        }

        /*
         * This is the case of non-sequential writes causing enough dirty
         * data to be accumulated, need to flush all of that.
         */
        extent_left = 0;
        extent_right = UINT64_MAX;
    }

    /*
     * Ok, we need to flush the extent now, check if we must do it inline.
     */
    if (inode->get_filecache()->do_inline_write()) {
        INC_GBL_STATS(inline_writes, 1);

        AZLogDebug("[{}] Inline write, {} bytes extent @ [{}, {})",
                   ino, (extent_right - extent_left),
                   extent_left, extent_right);

        const int err = inode->flush_cache_and_wait(extent_left, extent_right);
        if (err == 0) {
            reply_write(length);
            return;
        } else {
            AZLogError("[{}] Inline write, {} bytes extent @ [{}, {}), failed "
                       "with err {}",
                       ino, (extent_right - extent_left),
                       extent_left, extent_right, err);
            reply_error(err);
            return;
        }
    }

    std::vector<bytes_chunk> bc_vec =
        inode->get_filecache()->get_dirty_bc_range(extent_left, extent_right);

    if (bc_vec.size() == 0) {
        reply_write(length);
        return;
    }

    /*
     * Pass is_flush as false, since we don't want the writes to complete
     * before returning.
     */
    inode->sync_membufs(bc_vec, false /* is_flush */);

    // Send reply to original request without waiting for the backend write to complete.
    reply_write(length);
}

void rpc_task::run_flush()
{
    const fuse_ino_t ino = rpc_api->flush_task.get_ino();
    struct nfs_inode *const inode = get_client()->get_nfs_inode_from_ino(ino);

    reply_error(inode->flush_cache_and_wait());
}

void rpc_task::run_getattr()
{
    bool rpc_retry;
    auto ino = rpc_api->getattr_task.get_ino();
    struct nfs_inode *inode = get_client()->get_nfs_inode_from_ino(ino);

    INC_GBL_STATS(tot_getattr_reqs, 1);

    /*
     * If inode's cached attribute is valid, use that.
     */
    if (aznfsc_cfg.cache.attr.user.enable) {
        if (!inode->attr_cache_expired()) {
            INC_GBL_STATS(getattr_served_from_cache, 1);
            AZLogDebug("[{}] Returning cached attributes", ino);
            reply_attr(inode->get_attr(), inode->get_actimeo());
            return;
        }
    }

    do {
        GETATTR3args args;

        args.object = inode->get_fh();

        rpc_retry = false;
        stats.on_rpc_issue();
        if (rpc_nfs3_getattr_task(get_rpc_ctx(), getattr_callback, &args,
                                  this) == NULL) {
            stats.on_rpc_cancel();
            /*
             * Most common reason for this is memory allocation failure,
             * hence wait for some time before retrying. Also block the
             * current thread as we really want to slow down things.
             *
             * TODO: For soft mount should we fail this?
             */
            rpc_retry = true;

            AZLogWarn("rpc_nfs3_getattr_task failed to issue, retrying "
                      "after 5 secs!");
            ::sleep(5);
        }
    } while (rpc_retry);
}

void rpc_task::run_proxy_getattr()
{
    run_getattr();
}

void rpc_task::run_statfs()
{
    bool rpc_retry;
    auto ino = rpc_api->statfs_task.get_ino();

    do {
        FSSTAT3args args;
        args.fsroot = get_client()->get_nfs_inode_from_ino(ino)->get_fh();

        rpc_retry = false;
        stats.on_rpc_issue();
        if (rpc_nfs3_fsstat_task(get_rpc_ctx(), statfs_callback, &args,
                                 this) == NULL) {
            stats.on_rpc_cancel();
            /*
             * Most common reason for this is memory allocation failure,
             * hence wait for some time before retrying. Also block the
             * current thread as we really want to slow down things.
             *
             * TODO: For soft mount should we fail this?
             */
            rpc_retry = true;

            AZLogWarn("rpc_nfs3_fsstat_task failed to issue, retrying "
                      "after 5 secs!");
            ::sleep(5);
        }
    }  while (rpc_retry);
}

void rpc_task::run_create_file()
{
    bool rpc_retry;
    auto parent_ino = rpc_api->create_task.get_parent_ino();

    do {
        CREATE3args args;
        ::memset(&args, 0, sizeof(args));

        args.where.dir = get_client()->get_nfs_inode_from_ino(parent_ino)->get_fh();
        args.where.name = (char*)rpc_api->create_task.get_file_name();
        args.how.mode = (rpc_api->create_task.get_fuse_file()->flags & O_EXCL) ? GUARDED : UNCHECKED;
        args.how.createhow3_u.obj_attributes.mode.set_it = 1;
        args.how.createhow3_u.obj_attributes.mode.set_mode3_u.mode =
            rpc_api->create_task.get_mode();
        args.how.createhow3_u.obj_attributes.uid.set_it = 1;
        args.how.createhow3_u.obj_attributes.uid.set_uid3_u.uid =
            rpc_api->create_task.get_uid();
        args.how.createhow3_u.obj_attributes.gid.set_it = 1;
        args.how.createhow3_u.obj_attributes.gid.set_gid3_u.gid =
            rpc_api->create_task.get_gid();

        rpc_retry = false;
        stats.on_rpc_issue();
        if (rpc_nfs3_create_task(get_rpc_ctx(), createfile_callback, &args,
                                 this) == NULL) {
            stats.on_rpc_cancel();
            /*
             * Most common reason for this is memory allocation failure,
             * hence wait for some time before retrying. Also block the
             * current thread as we really want to slow down things.
             *
             * TODO: For soft mount should we fail this?
             */
            rpc_retry = true;

            AZLogWarn("rpc_nfs3_create_task failed to issue, retrying "
                      "after 5 secs!");
            ::sleep(5);
        }
    }  while (rpc_retry);
}

void rpc_task::run_mknod()
{
    bool rpc_retry;
    auto parent_ino = rpc_api->mknod_task.get_parent_ino();

    // mknod is supported only for regular file.
    assert(S_ISREG(rpc_api->mknod_task.get_mode()));

    do {
        CREATE3args args;
        ::memset(&args, 0, sizeof(args));

        args.where.dir = get_client()->get_nfs_inode_from_ino(parent_ino)->get_fh();
        args.where.name = (char*)rpc_api->mknod_task.get_file_name();
        args.how.createhow3_u.obj_attributes.mode.set_it = 1;
        args.how.createhow3_u.obj_attributes.mode.set_mode3_u.mode =
            rpc_api->mknod_task.get_mode();
        args.how.createhow3_u.obj_attributes.uid.set_it = 1;
        args.how.createhow3_u.obj_attributes.uid.set_uid3_u.uid =
            rpc_api->mknod_task.get_uid();
        args.how.createhow3_u.obj_attributes.gid.set_it = 1;
        args.how.createhow3_u.obj_attributes.gid.set_gid3_u.gid =
            rpc_api->mknod_task.get_gid();

        rpc_retry = false;
        stats.on_rpc_issue();
        if (rpc_nfs3_create_task(get_rpc_ctx(), mknod_callback, &args,
                                 this) == NULL) {
            stats.on_rpc_cancel();
            /*
             * Most common reason for this is memory allocation failure,
             * hence wait for some time before retrying. Also block the
             * current thread as we really want to slow down things.
             *
             * TODO: For soft mount should we fail this?
             */
            rpc_retry = true;

            AZLogWarn("rpc_nfs3_create_task failed to issue, retrying "
                      "after 5 secs!");
            ::sleep(5);
        }
    }  while (rpc_retry);
}

void rpc_task::run_mkdir()
{
    bool rpc_retry;
    auto parent_ino = rpc_api->mkdir_task.get_parent_ino();

    do {
        MKDIR3args args;
        ::memset(&args, 0, sizeof(args));

        args.where.dir = get_client()->get_nfs_inode_from_ino(parent_ino)->get_fh();
        args.where.name = (char*)rpc_api->mkdir_task.get_dir_name();
        args.attributes.mode.set_it = 1;
        args.attributes.mode.set_mode3_u.mode = rpc_api->mkdir_task.get_mode();
        args.attributes.uid.set_it = 1;
        args.attributes.uid.set_uid3_u.uid = rpc_api->mkdir_task.get_uid();
        args.attributes.gid.set_it = 1;
        args.attributes.gid.set_gid3_u.gid = rpc_api->mkdir_task.get_gid();

        rpc_retry = false;
        stats.on_rpc_issue();
        if (rpc_nfs3_mkdir_task(get_rpc_ctx(), mkdir_callback, &args,
                                this) == NULL) {
            stats.on_rpc_cancel();
            /*
             * Most common reason for this is memory allocation failure,
             * hence wait for some time before retrying. Also block the
             * current thread as we really want to slow down things.
             *
             * TODO: For soft mount should we fail this?
             */
            rpc_retry = true;

            AZLogWarn("rpc_nfs3_mkdir_task failed to issue, retrying "
                      "after 5 secs!");
            ::sleep(5);
        }
    } while (rpc_retry);
}

void rpc_task::run_unlink()
{
    bool rpc_retry;
    auto parent_ino = rpc_api->unlink_task.get_parent_ino();

    do {
        REMOVE3args args;
        args.object.dir = get_client()->get_nfs_inode_from_ino(parent_ino)->get_fh();
        args.object.name = (char*) rpc_api->unlink_task.get_file_name();

        rpc_retry = false;
        stats.on_rpc_issue();
        if (rpc_nfs3_remove_task(get_rpc_ctx(),
                                 unlink_callback, &args, this) == NULL) {
            stats.on_rpc_cancel();
            /*
             * Most common reason for this is memory allocation failure,
             * hence wait for some time before retrying. Also block the
             * current thread as we really want to slow down things.
             *
             * TODO: For soft mount should we fail this?
             */
            rpc_retry = true;

            AZLogWarn("rpc_nfs3_remove_task failed to issue, retrying "
                      "after 5 secs!");
            ::sleep(5);
        }
    } while (rpc_retry);
}

void rpc_task::run_rmdir()
{
    bool rpc_retry;
    auto parent_ino = rpc_api->rmdir_task.get_parent_ino();

    do {
        RMDIR3args args;

        args.object.dir = get_client()->get_nfs_inode_from_ino(parent_ino)->get_fh();
        args.object.name = (char*) rpc_api->rmdir_task.get_dir_name();

        rpc_retry = false;
        stats.on_rpc_issue();
        if (rpc_nfs3_rmdir_task(get_rpc_ctx(),
                                rmdir_callback, &args, this) == NULL) {
            stats.on_rpc_cancel();
            /*
             * Most common reason for this is memory allocation failure,
             * hence wait for some time before retrying. Also block the
             * current thread as we really want to slow down things.
             *
             * TODO: For soft mount should we fail this?
             */
            rpc_retry = true;

            AZLogWarn("rpc_nfs3_rmdir_task failed to issue, retrying "
                      "after 5 secs!");
            ::sleep(5);
        }
    } while (rpc_retry);
}

void rpc_task::run_symlink()
{
    bool rpc_retry;
    const fuse_ino_t parent_ino = rpc_api->symlink_task.get_parent_ino();

    do {
        SYMLINK3args args;
        ::memset(&args, 0, sizeof(args));

        args.where.dir = get_client()->get_nfs_inode_from_ino(parent_ino)->get_fh();
        args.where.name = (char*) rpc_api->symlink_task.get_name();
        args.symlink.symlink_data = (char*) rpc_api->symlink_task.get_link();
        args.symlink.symlink_attributes.uid.set_it = 1;
        args.symlink.symlink_attributes.uid.set_uid3_u.uid =
            rpc_api->symlink_task.get_uid();
        args.symlink.symlink_attributes.gid.set_it = 1;
        args.symlink.symlink_attributes.gid.set_gid3_u.gid =
            rpc_api->symlink_task.get_gid();

        rpc_retry = false;
        stats.on_rpc_issue();
        if (rpc_nfs3_symlink_task(get_rpc_ctx(),
                                         symlink_callback,
                                         &args,
                                         this) == NULL) {
            stats.on_rpc_cancel();
            /*
             * Most common reason for this is memory allocation failure,
             * hence wait for some time before retrying. Also block the
             * current thread as we really want to slow down things.
             *
             * TODO: For soft mount should we fail this?
             */
            rpc_retry = true;

            AZLogWarn("rpc_nfs3_symlink_task failed to issue, retrying "
                      "after 5 secs!");
            ::sleep(5);
        }
    } while (rpc_retry);
}

void rpc_task::run_rename()
{
    bool rpc_retry;
    const fuse_ino_t parent_ino = rpc_api->rename_task.get_parent_ino();
    const fuse_ino_t newparent_ino = rpc_api->rename_task.get_newparent_ino();

    do {
        RENAME3args args;
        args.from.dir = get_client()->get_nfs_inode_from_ino(parent_ino)->get_fh();
        args.from.name = (char*) rpc_api->rename_task.get_name();
        args.to.dir = get_client()->get_nfs_inode_from_ino(newparent_ino)->get_fh();
        args.to.name = (char*) rpc_api->rename_task.get_newname();

        rpc_retry = false;
        stats.on_rpc_issue();
        if (rpc_nfs3_rename_task(get_rpc_ctx(),
                                        rename_callback,
                                        &args,
                                        this) == NULL) {
            stats.on_rpc_cancel();
            /*
             * Most common reason for this is memory allocation failure,
             * hence wait for some time before retrying. Also block the
             * current thread as we really want to slow down things.
             *
             * TODO: For soft mount should we fail this?
             */
            rpc_retry = true;

            AZLogWarn("rpc_nfs3_rename_task failed to issue, retrying "
                      "after 5 secs!");
            ::sleep(5);
        }
    } while (rpc_retry);
}

void rpc_task::run_readlink()
{
    bool rpc_retry;
    const fuse_ino_t ino = rpc_api->readlink_task.get_ino();

    do {
        READLINK3args args;
        args.symlink = get_client()->get_nfs_inode_from_ino(ino)->get_fh();

        rpc_retry = false;
        stats.on_rpc_issue();
        if (rpc_nfs3_readlink_task(get_rpc_ctx(),
                                          readlink_callback,
                                          &args,
                                          this) == NULL) {
            stats.on_rpc_cancel();
            /*
             * Most common reason for this is memory allocation failure,
             * hence wait for some time before retrying. Also block the
             * current thread as we really want to slow down things.
             *
             * TODO: For soft mount should we fail this?
             */
            rpc_retry = true;

            AZLogWarn("rpc_nfs3_readlink_task failed to issue, retrying "
                      "after 5 secs!");
            ::sleep(5);
        }
    } while (rpc_retry);
}

void rpc_task::run_setattr()
{
    auto ino = rpc_api->setattr_task.get_ino();
    struct nfs_inode *inode = get_client()->get_nfs_inode_from_ino(ino);
    auto attr = rpc_api->setattr_task.get_attr();
    const int valid = rpc_api->setattr_task.get_attr_flags_to_set();
    bool rpc_retry;

    /*
     * If this is a setattr(mtime) call called for updating mtime of a file
     * under write in writeback mode, skip the call and return cached
     * attributes. Note that write requests sent to NSF server will correctly
     * update the mtime so we don't need to do that.
     * Since fuse doesn't provide us a way to turn off these setattr(mtime)
     * calls, we have this hack.
     */
    if ((valid && !(valid & ~FUSE_SET_ATTR_MTIME)) && inode->skip_mtime_update()) {
        /*
         * Set fuse kernel attribute cache timeout to the current attribute
         * cache timeout for this inode, as per the recent revalidation
         * experience.
         */
        AZLogDebug("[{}] Skipping mtime update", ino);
        reply_attr(inode->get_attr(), inode->get_actimeo());
        return;
    }

    do {
        SETATTR3args args;
        ::memset(&args, 0, sizeof(args));

        args.object = inode->get_fh();

        if (valid & FUSE_SET_ATTR_MODE) {
            AZLogDebug("Setting mode to 0{:o}", attr->st_mode);
            args.new_attributes.mode.set_it = 1;
            args.new_attributes.mode.set_mode3_u.mode = attr->st_mode;
        }

        if (valid & FUSE_SET_ATTR_UID) {
            AZLogDebug("Setting uid to {}", attr->st_uid);
            args.new_attributes.uid.set_it = 1;
            args.new_attributes.uid.set_uid3_u.uid = attr->st_uid;
        }

        if (valid & FUSE_SET_ATTR_GID) {
            AZLogDebug("Setting gid to {}", attr->st_gid);
            args.new_attributes.gid.set_it = 1;
            args.new_attributes.gid.set_gid3_u.gid = attr->st_gid;
        }

        if (valid & FUSE_SET_ATTR_SIZE) {
            AZLogDebug("Setting size to {}", attr->st_size);
            args.new_attributes.size.set_it = 1;
            args.new_attributes.size.set_size3_u.size = attr->st_size;
        }

        if (valid & FUSE_SET_ATTR_ATIME) {
            // TODO: These log are causing crash, look at it later.
            // AZLogDebug("Setting atime to {}", attr->st_atim.tv_sec);

            args.new_attributes.atime.set_it = SET_TO_CLIENT_TIME;
            args.new_attributes.atime.set_atime_u.atime.seconds =
                attr->st_atim.tv_sec;
            args.new_attributes.atime.set_atime_u.atime.nseconds =
                attr->st_atim.tv_nsec;
        }

        if (valid & FUSE_SET_ATTR_MTIME) {
            // TODO: These log are causing crash, look at it later.
            // AZLogDebug("Setting mtime to {}", attr->st_mtim.tv_sec);

            args.new_attributes.mtime.set_it = SET_TO_CLIENT_TIME;
            args.new_attributes.mtime.set_mtime_u.mtime.seconds =
                attr->st_mtim.tv_sec;
            args.new_attributes.mtime.set_mtime_u.mtime.nseconds =
                attr->st_mtim.tv_nsec;
        }

        if (valid & FUSE_SET_ATTR_ATIME_NOW) {
            args.new_attributes.atime.set_it = SET_TO_SERVER_TIME;
        }

        if (valid & FUSE_SET_ATTR_MTIME_NOW) {
            args.new_attributes.mtime.set_it = SET_TO_SERVER_TIME;
        }

        rpc_retry = false;
        stats.on_rpc_issue();
        if (rpc_nfs3_setattr_task(get_rpc_ctx(), setattr_callback, &args,
                                  this) == NULL) {
            stats.on_rpc_cancel();
            /*
             * Most common reason for this is memory allocation failure,
             * hence wait for some time before retrying. Also block the
             * current thread as we really want to slow down things.
             *
             * TODO: For soft mount should we fail this?
             */
            rpc_retry = true;

            AZLogWarn("rpc_nfs3_setattr_task failed to issue, retrying "
                      "after 5 secs!");
            ::sleep(5);
        }
    } while (rpc_retry);
}

void rpc_task::run_read()
{
    const fuse_ino_t ino = rpc_api->read_task.get_ino();
    struct nfs_inode *inode = get_client()->get_nfs_inode_from_ino(ino);

    assert(inode->is_regfile());
    /*
     * aznfsc_ll_read() can only be called after aznfsc_ll_open() so filecache
     * and readahead state must have been allocated when we reach here.
     */
    assert(inode->has_filecache());
    assert(inode->has_rastate());

    std::shared_ptr<bytes_chunk_cache>& filecache_handle =
        inode->get_filecache();

    /*
     * run_read() is called once for a fuse read request and must not be
     * called for a child task.
     */
    assert(rpc_api->parent_task == nullptr);

    /*
     * In solowriter mode we know the file size definitively.
     * This is an optimization that saves an extra READ call to the server.
     * The server will correctly return 0+eof for this READ call so we are
     * functionally correct in other consistency modes too.
     */
    if (aznfsc_cfg.consistency_solowriter) {
        const int64_t file_size = inode->get_file_size();
        if ((file_size != -1) &&
            (rpc_api->read_task.get_offset() >= file_size)) {
            AZLogDebug("[{}] Read returning 0 bytes (eof) as requested "
                       "offset ({}) >= file size ({})",
                       ino, rpc_api->read_task.get_offset(), file_size);
            reply_iov(nullptr, 0);
            return;
        }
    }

    /*
     * Get bytes_chunks covering the region caller wants to read.
     * The bytes_chunks returned could be any mix of old (already cached) or
     * new (cache allocated but yet to be read from blob). Note that reads
     * don't need any special protection. The caller just wants to read the
     * current contents of the blob, these can change immediately after or
     * even while we are reading, resulting in any mix of old or new data.
     */
    assert(bc_vec.empty());
    bc_vec = filecache_handle->get(
                       rpc_api->read_task.get_offset(),
                       rpc_api->read_task.get_size());

    const size_t size = bc_vec.size();
    assert(size > 0);

    // There should not be any reads running for this RPC task initially.
    assert(num_ongoing_backend_reads == 0);

    AZLogDebug("[{}] run_read: offset {}, size: {}, chunks: {}",
               ino,
               rpc_api->read_task.get_offset(),
               rpc_api->read_task.get_size(),
               size);

    /*
     * Now go through the byte chunk vector to see if the chunks are
     * uptodate. For chunks which are not uptodate we will issue read
     * calls to fetch the data from the NFS server. These will be issued in
     * parallel. Once all chunks are uptodate we can complete the read to the
     * caller.
     *
     * Note that we bump num_ongoing_backend_reads by 1 before issuing
     * the first backend read. This is done to make sure if read_callback()
     * is called before we could issues all reads, we don't mistake it for
     * "all issued reads have completed". It is ok to update this without a lock
     * since this is the only thread at this point which will access this.
     *
     * Note: Membufs which are found uptodate here shouldn't suddenly become
     *       non-uptodate when the other reads complete, o/w we have a problem.
     *       An uptodate membuf doesn't become non-uptodate but it can be
     *       written by some writer thread, while we are waiting for other
     *       chunk(s) to be read from backend or even while we are reading
     *       them while sending the READ response.
     */

    [[maybe_unused]] size_t total_length = 0;
    bool found_in_cache = true;

    num_ongoing_backend_reads = 1;

    for (size_t i = 0; i < size; i++) {
        /*
         * Every bytes_chunk returned by get() must have its inuse count
         * bumped. Also they must have pvt set to the initial value of 0
         * and num_backend_calls_issued set to initial value of 0.
         */
        assert(bc_vec[i].get_membuf()->is_inuse());
        assert(bc_vec[i].pvt == 0);
        assert(bc_vec[i].num_backend_calls_issued == 0);

        total_length += bc_vec[i].length;

        if (!bc_vec[i].get_membuf()->is_uptodate()) {
            /*
             * Now we are going to call read_from_server() which will issue
             * an NFS read that will read the data from the NFS server and
             * update the buffer. Grab the membuf lock, this will be unlocked
             * in read_callback() once the data has been read into the buffer
             * and it's marked uptodate.
             *
             * Note: This will block till the lock is obtained.
             */
            bc_vec[i].get_membuf()->set_locked();

            // Check if the buffer got updated by the time we got the lock.
            if (bc_vec[i].get_membuf()->is_uptodate()) {
                /*
                * Release the lock since we no longer intend on writing
                * to this buffer.
                */
                bc_vec[i].get_membuf()->clear_locked();
                bc_vec[i].get_membuf()->clear_inuse();

                /*
                 * Set "bytes read" to "bytes requested" since the data is read
                 * from the cache.
                 */
                bc_vec[i].pvt = bc_vec[i].length;

                INC_GBL_STATS(bytes_read_from_cache, bc_vec[i].length);

                AZLogDebug("Data read from cache. offset: {}, length: {}",
                        bc_vec[i].offset,
                        bc_vec[i].length);

#ifdef RELEASE_CHUNK_AFTER_APPLICATION_READ
                /*
                 * Since the data is read from the cache, the chances of reading
                 * it again from cache is negligible since this is a sequential
                 * read pattern.
                 * Free such chunks to reduce the memory utilization.
                 */
                filecache_handle->release(bc_vec[i].offset, bc_vec[i].length);
#endif
                continue;
            }

            found_in_cache = false;

            /*
             * TODO: If we have just 1 bytes_chunk to fill, which is the most
             *       common case, avoid creating child task and process
             *       everything in this same task.
             *       Also for contiguous reads use the libnfs vectored read API.
             */

            /*
             * Create a child rpc task to issue the read RPC to the backend.
             */
            struct rpc_task *child_tsk =
                get_client()->get_rpc_task_helper()->alloc_rpc_task(FUSE_READ);

            child_tsk->init_read(
                rpc_api->req,
                rpc_api->read_task.get_ino(),
                bc_vec[i].length,
                bc_vec[i].offset,
                rpc_api->read_task.get_fuse_file());

            // Set the parent task of the child to the current RPC task.
            child_tsk->rpc_api->parent_task = this;

            /*
             * Set "bytes read" to 0 and this will be updated as data is read,
             * likely in partial read calls. So at any time bc.pvt will be the
             * total data read.
             */
            bc_vec[i].pvt = 0;

            // Set the byte chunk that this child task is incharge of updating.
            child_tsk->rpc_api->bc = &bc_vec[i];

            /*
             * Child task should always read a subset of the parent task.
             */
            assert(child_tsk->rpc_api->read_task.get_offset() >=
                   rpc_api->read_task.get_offset());
            assert(child_tsk->rpc_api->read_task.get_size() <=
                   rpc_api->read_task.get_size());

            child_tsk->read_from_server(bc_vec[i]);
        } else {
            bc_vec[i].get_membuf()->clear_inuse();

            /*
             * Set "bytes read" to "bytes requested" since the data is read
             * from the cache.
             */
            bc_vec[i].pvt = bc_vec[i].length;

            INC_GBL_STATS(bytes_read_from_cache, bc_vec[i].length);

#ifdef RELEASE_CHUNK_AFTER_APPLICATION_READ
            /*
             * Data read from cache. For the most common sequential read
             * pattern this cached data won't be needed again, release
             * it promptly to ease memory pressure.
             * Note that this is just a suggestion to release the buffer.
             * The buffer may not be released if it's in use by any other
             * user.
             */
            filecache_handle->release(bc_vec[i].offset, bc_vec[i].length);
#endif
        }
    }

    // get() must return bytes_chunks exactly covering the requested range.
    assert(total_length == rpc_api->read_task.get_size());

    /*
     * Decrement the read ref incremented above.
     * Each completing child task will also update the parent task's
     * num_ongoing_backend_reads, so we check for that.
     */
    assert(num_ongoing_backend_reads >= 1);
    if (--num_ongoing_backend_reads != 0) {
        assert(!found_in_cache);
        /*
         * Not all backend reads have completed yet. When the last backend
         * read completes read_callback() will arrange to send the read
         * response to fuse.
         * This is the more common case as backend READs will take time to
         * complete.
         */
        return;
    }

    /*
     * Either no chunk needed backend read (likely) or all backend reads issued
     * above completed (unlikely).
     */

    if (found_in_cache) {
        AZLogDebug("[{}] Data read from cache, offset: {}, size: {}",
                   ino,
                   rpc_api->read_task.get_offset(),
                   rpc_api->read_task.get_size());
    }

    // Send the response.
    send_read_response();
}

void rpc_task::send_read_response()
{
    [[maybe_unused]] const fuse_ino_t ino = rpc_api->read_task.get_ino();

    // This should always be called on the parent task.
    assert(rpc_api->parent_task == nullptr);

    /*
     * We must send response only after all component reads complete, they may
     * succeed or fail.
     */
    assert(num_ongoing_backend_reads == 0);

    if (read_status != 0) {
        // Non-zero status indicates failure, reply with error in such cases.
        AZLogDebug("[{}] Sending failed read response {}", ino, read_status.load());

        reply_error(read_status);
        return;
    }

    /*
     * No go over all the chunks and send a vectored read response to fuse.
     * Note that only the last chunk can be partial.
     * XXX No, in case of multiple chunks and short read, multiple can be
     *     partial.
     */
    size_t count = bc_vec.size();

    // Create an array of iovec struct
    struct iovec iov[count];
    uint64_t bytes_read = 0;
    [[maybe_unused]] bool partial_read = false;

    for (size_t i = 0; i < count; i++) {
        assert(bc_vec[i].pvt <= bc_vec[i].length);

        iov[i].iov_base = (void *) bc_vec[i].get_buffer();
        iov[i].iov_len = bc_vec[i].pvt;

        bytes_read += bc_vec[i].pvt;

        if (bc_vec[i].pvt < bc_vec[i].length) {
#if 0
            assert((i == count-1) || (bc_vec[i+1].length == 0));
#endif
            partial_read = true;
            count = i + 1;
            break;
        }
    }

    assert((bytes_read == rpc_api->read_task.get_size()) || partial_read);

    // Send response to caller.
    if (bytes_read == 0) {
        AZLogDebug("[{}] Sending empty read response", ino);
        reply_iov(nullptr, 0);
    } else {
        AZLogDebug("[{}] Sending success read response, iovec={}, "
                   "bytes_read={}",
                   ino, count, bytes_read);
        INC_GBL_STATS(tot_bytes_read, bytes_read);
        reply_iov(iov, count);
    }
}

struct read_context
{
    rpc_task *task;
    struct bytes_chunk *bc;

    read_context(
        rpc_task *_task,
        struct bytes_chunk *_bc):
        task(_task),
        bc(_bc)
    {
        assert(task->magic == RPC_TASK_MAGIC);
        assert(bc->length > 0 && bc->length <= AZNFSC_MAX_CHUNK_SIZE);
        assert(bc->offset < AZNFSC_MAX_FILE_SIZE);
    }
};


static void read_callback(
    struct rpc_context *rpc,
    int rpc_status,
    void *data,
    void *private_data)
{
    struct read_context *ctx = (read_context*) private_data;
    rpc_task *task = ctx->task;
    assert(task->magic == RPC_TASK_MAGIC);

    /*
     * Parent task corresponds to the fuse request that intiated the read.
     * This will be used to complete the request to fuse.
     */
    rpc_task *parent_task = task->rpc_api->parent_task;

    /*
     * Only child tasks can issue the read RPC, hence the callback should
     * be called only for them.
     */
    assert(parent_task != nullptr);
    assert(parent_task->magic == RPC_TASK_MAGIC);

    /*
     * num_ongoing_backend_reads is updated only for the parent task and it
     * counts how many child rpc tasks are ongoing for this parent task.
     * num_ongoing_backend_reads will always be 0 for child tasks.
     */
    assert(parent_task->num_ongoing_backend_reads > 0);
    assert(task->num_ongoing_backend_reads == 0);

    struct bytes_chunk *bc = ctx->bc;
    assert(bc->length > 0);

    // We are in the callback, so at least one backend call was issued.
    assert(bc->num_backend_calls_issued > 0);

    /*
     * If we have already finished reading the entire bytes_chunk, why are we
     * here. We must have locked the membuf and marked inuse before we issued
     * the read.
     */
    assert(bc->pvt < bc->length);
    assert(bc->get_membuf()->is_inuse());
    assert(bc->get_membuf()->is_locked());

    const char* errstr;
    auto res = (READ3res*)data;

    INJECT_JUKEBOX(res, task);

    const int status = (task->status(rpc_status, NFS_STATUS(res), &errstr));
    fuse_ino_t ino = task->rpc_api->read_task.get_ino();
    struct nfs_inode *inode = task->get_client()->get_nfs_inode_from_ino(ino);

    /*
     * Applications can only issue reads on an open fd and we ensure filecache
     * is created on file open.
     */
    assert(inode->has_filecache());
    auto filecache_handle = inode->get_filecache();
    /*
     * read_callback() must only be called for read done from fuse for which
     * we must have allocated the cache.
     */
    assert(filecache_handle);
    const uint64_t issued_offset = bc->offset + bc->pvt;
    const uint64_t issued_length = bc->length - bc->pvt;

    /*
     * It is okay to free the context here as we do not access it after this
     * point.
     */
    delete ctx;

    /*
     * Now that the request has completed, we can query libnfs for the
     * dispatch time.
     */
    task->get_stats().on_rpc_complete(rpc_get_pdu(rpc), NFS_STATUS(res));

    if (status == 0) {
        UPDATE_INODE_ATTR(inode, res->READ3res_u.resok.file_attributes);

#ifdef ENABLE_PRESSURE_POINTS
        /*
         * Short read pressure point, skip when eof received.
         */
        if (inject_error() && !res->READ3res_u.resok.eof) {
            // Set read size to a random percent of the actual size.
            const uint32_t pct = random_number(1, 100);
            const uint32_t adj_size =
                std::max((res->READ3res_u.resok.count * pct) / 100, 1U);
            assert(adj_size <= res->READ3res_u.resok.count);
            AZLogWarn("[{}] PP: short read {} -> {}",
                      ino, res->READ3res_u.resok.count, adj_size);
            res->READ3res_u.resok.count = adj_size;
        }
#endif

        assert((bc->pvt == 0) || (bc->num_backend_calls_issued > 1));

        // We should never get more data than what we requested.
        assert(res->READ3res_u.resok.count <= issued_length);

        const bool is_partial_read = !res->READ3res_u.resok.eof &&
            (res->READ3res_u.resok.count < issued_length);

        // Update bc->pvt with fresh bytes read in this call.
        bc->pvt += res->READ3res_u.resok.count;
        assert(bc->pvt <= bc->length);

        AZLogDebug("[{}] read_callback: {}Read completed for [{}, {}), "
                   "Bytes read: {} eof: {}, total bytes read till "
                   "now: {} of {} for [{}, {}) num_backend_calls_issued: {}",
                   ino,
                   is_partial_read ? "Partial " : "",
                   issued_offset,
                   issued_offset + issued_length,
                   res->READ3res_u.resok.count,
                   res->READ3res_u.resok.eof,
                   bc->pvt,
                   bc->length,
                   bc->offset,
                   bc->offset + bc->length,
                   bc->num_backend_calls_issued);

        /*
         * In case of partial read, issue read for the remaining.
         */
        if (is_partial_read) {
            const off_t new_offset = bc->offset + bc->pvt;
            const size_t new_size = bc->length - bc->pvt;

            // Create a new child task to carry out this request.
            struct rpc_task *child_tsk =
                task->get_client()->get_rpc_task_helper()->alloc_rpc_task(FUSE_READ);

            child_tsk->init_read(
                task->rpc_api->req,
                task->rpc_api->read_task.get_ino(),
                new_size,
                new_offset,
                task->rpc_api->read_task.get_fuse_file());

            /*
             * Set the parent task of the child to the parent of the
             * current RPC task. This is required if the current task itself
             * is one of the child tasks running part of the fuse read request.
             */
            child_tsk->rpc_api->parent_task = parent_task;

            /*
             * Child task must continue to fill the same bc.
             */
            child_tsk->rpc_api->bc = bc;

            /*
             * TODO: To avoid allocating a new read_context we can reuse the
             *       existing contest but we have to update the task member.
             */
            struct read_context *new_ctx = new read_context(child_tsk, bc);
            bool rpc_retry;
            READ3args new_args;

            new_args.file = inode->get_fh();
            new_args.offset = new_offset;
            new_args.count = new_size;

            // One more backend call issued to fill this bc.
            bc->num_backend_calls_issued++;

            AZLogDebug("[{}] Issuing partial read at offset: {} size: {}"
                       " for [{}, {})",
                       ino,
                       new_offset,
                       new_size,
                       bc->offset,
                       bc->offset + bc->length);

            do {
                /*
                 * We have identified partial read case where the
                 * server has returned fewer bytes than requested.
                 * Fuse cannot accept fewer bytes than requested,
                 * unless it's an eof or error.
                 * Hence we will issue read for the remaining.
                 *
                 * Note: It is okay to issue a read call directly here
                 *       as we are holding all the needed locks and refs.
                 */
                rpc_retry = false;
                child_tsk->get_stats().on_rpc_issue();
                if (rpc_nfs3_read_task(
                        child_tsk->get_rpc_ctx(),
                        read_callback,
                        bc->get_buffer() + bc->pvt,
                        new_size,
                        &new_args,
                        (void *) new_ctx) == NULL) {
                    child_tsk->get_stats().on_rpc_cancel();
                    /*
                     * Most common reason for this is memory allocation failure,
                     * hence wait for some time before retrying. Also block the
                     * current thread as we really want to slow down things.
                     *
                     * TODO: For soft mount should we fail this?
                     */
                    rpc_retry = true;

                    AZLogWarn("rpc_nfs3_read_task failed to issue, retrying "
                              "after 5 secs!");
                    ::sleep(5);
                }
            } while (rpc_retry);

            // Free the current RPC task as it has done its bit.
            task->free_rpc_task();

            /*
             * Return from the callback here. The rest of the callback
             * will be processed once this partial read completes.
             */
            return;
        }

        /*
         * We should never return lesser bytes to the fuse than requested,
         * unless error or eof is encountered after this point.
         */
        assert((bc->length == bc->pvt) || res->READ3res_u.resok.eof);

        if (bc->maps_full_membuf() && (bc->length == bc->pvt)) {
            /*
             * Only the first read which got hold of the complete membuf
             * will have this byte_chunk set to empty.
             * Only such reads should set the uptodate flag.
             * Also the uptodate flag should be set only if we have read
             * the entire membuf.
             */
#ifdef ENABLE_PRESSURE_POINTS
            if (inject_error()) {
                AZLogDebug("[{}] PP: Not setting uptodate flag for membuf "
                           "[{}, {})",
                           ino, bc->offset, bc->offset + bc->length);
            } else
#endif
            {
                AZLogDebug("[{}] Setting uptodate flag for membuf [{}, {})",
                           ino, bc->offset, bc->offset + bc->length);

                bc->get_membuf()->set_uptodate();
            }
        } else {
            bool set_uptodate = false;

            /*
             * If we got eof in a partial read, release the non-existent
             * portion of the chunk.
             */
            if (bc->maps_full_membuf() && (bc->length > bc->pvt) &&
                res->READ3res_u.resok.eof) {
                assert(res->READ3res_u.resok.count < issued_length);

                /*
                 * We need to clear the inuse count held by this thread, else
                 * release() will not be able to release. We drop and then
                 * promptly grab the inuse count after the release(), so that
                 * set_uptodate() can be called.
                 */
                bc->get_membuf()->clear_inuse();
                const uint64_t released_bytes =
                    filecache_handle->release(bc->offset + bc->pvt,
                                              bc->length - bc->pvt);
                bc->get_membuf()->set_inuse();

                /*
                 * If we are able to successfully release all the extra bytes
                 * from the bytes_chunk, that means there's no other thread
                 * actively performing IOs to the underlying membuf, so we can
                 * mark it uptodate.
                 */
                assert(released_bytes <= (bc->length - bc->pvt));
                if (released_bytes == (bc->length - bc->pvt)) {
                    AZLogDebug("[{}] Setting uptodate flag for membuf [{}, {}) "
                               "after read hit eof, requested [{}, {}), "
                               "got [{}, {})",
                               ino,
                               bc->offset, bc->offset + bc->length,
                               issued_offset,
                               issued_offset + issued_length,
                               issued_offset,
                               issued_offset + res->READ3res_u.resok.count);
                    bc->get_membuf()->set_uptodate();
                    set_uptodate = true;
                }
            }

            if (!set_uptodate) {
                AZLogDebug("[{}] Not setting uptodate flag for membuf "
                           "[{}, {}), maps_full_membuf={}, is_new={}, "
                           "bc->length={}, bc->pvt={}",
                           ino, bc->offset, bc->offset + bc->length,
                           bc->maps_full_membuf(), bc->is_new, bc->length,
                           bc->pvt);
            }
        }
    } else if (NFS_STATUS(res) == NFS3ERR_JUKEBOX) {
        task->get_client()->jukebox_retry(task);

        /*
         * Note: The lock on the membuf will be held till the task is retried.
         *       This lock will be released only if the retry passes or fails
         *       with error other than NFS3ERR_JUKEBOX.
         */
        return;
    } else {
        AZLogError("[{}] Read failed for offset: {} size: {} "
                   "total bytes read till now: {} of {} for [{}, {}) "
                   "num_backend_calls_issued: {} error: {}",
                   ino,
                   issued_offset,
                   issued_length,
                   bc->pvt,
                   bc->length,
                   bc->offset,
                   bc->offset + bc->length,
                   bc->num_backend_calls_issued,
                   errstr);
    }

    /*
    * Release the lock that we held on the membuf since the data is now
    * written to it.
    * The lock is needed only to write the data and not to just read it.
    * Hence it is safe to read this membuf even beyond this point.
    */
    bc->get_membuf()->clear_locked();
    bc->get_membuf()->clear_inuse();

#ifdef RELEASE_CHUNK_AFTER_APPLICATION_READ
    /*
    * Since we come here only for client reads, we will not cache the data,
    * hence release the chunk.
    * This can safely be done for both success and failure case.
    */
    filecache_handle->release(bc->offset, bc->length);
#endif

    // For failed status we must never mark the buffer uptodate.
    assert(!status || !bc->get_membuf()->is_uptodate());

    // Once failed, read_status remains at failed.
    int expected = 0;
    parent_task->read_status.compare_exchange_weak(expected, status);

    /*
     * Decrement the number of reads issued atomically and if it becomes zero
     * it means this is the last read completing. We send the response if all
     * the reads have completed or the read failed.
     */
    if (--parent_task->num_ongoing_backend_reads == 0) {
        /*
         * Parent task must send the read response to fuse.
         * This will also free parent_task.
         */
        parent_task->send_read_response();

        // Free the child task after sending the response.
        task->free_rpc_task();
    } else {
        AZLogDebug("No response sent, waiting for more reads to complete."
                   " num_ongoing_backend_reads: {}",
                   parent_task->num_ongoing_backend_reads.load());

        /*
         * This task has completed its part of the read, free it here.
         * When all reads complete, the parent task will be completed.
         */
        task->free_rpc_task();
        return;
    }
}

/*
 * This function issues READ rpc to the backend to read data into bc.
 * Offset and length to read is specified by the rpc_task object on which it's
 * called. bc conveys where the data has to be read into. Note that we may
 * have to call read_from_server() multiple times to fill the same bc in
 * parts in case of partial writes or even repeat the same call (in the case
 * of jukebox).  Every time some part of bc is filled, bc.pvt is updated to
 * reflect that and hence following are also the offset and length of data to
 * be read.
 * bc.offset + bc.pvt
 * bc.length - bc.pvt
 * and similarly "bc.get_buffer + bc.pvt" is the address where the data has
 * to be read into.
 *
 * Note: Caller MUST hold a lock on the underlying membuf of bc by calling
 *       bc.get_membuf()->set_locked().
 */
void rpc_task::read_from_server(struct bytes_chunk &bc)
{
    bool rpc_retry;
    const auto ino = rpc_api->read_task.get_ino();
    struct nfs_inode *inode = get_client()->get_nfs_inode_from_ino(ino);

    /*
     * Fresh reads will have num_backend_calls_issued == 0 and it'll be updated
     * as we issue backend calls (with the value becoming > 1 in case of partial
     * reads). When any of such reads is retried due to jukebox it'll have
     * num_backend_calls_issued > 0.
     */
    const bool is_jukebox_read = (bc.num_backend_calls_issued > 0);

    assert(rpc_api->bc == &bc);
    assert(bc.get_membuf()->is_locked());

    /*
     * This should always be called from the child task as we will issue read
     * RPC to the backend.
     */
    assert(rpc_api->parent_task != nullptr);

    /*
     * bc.pvt is the cursor holding the number of bytes that we have already
     * read and tell where to read the next data into. For partial reads it
     * tracks the progress and helps find out the next bytes read. It should
     * be correctly updated and this child task should read the required bytes.
     */
    assert(bc.pvt < bc.length);
    assert(rpc_api->read_task.get_offset() == ((off_t) bc.offset + (off_t) bc.pvt));
    assert(rpc_api->read_task.get_size() == (bc.length - bc.pvt));

    /*
     * This will be freed in read_callback().
     * Note that the read_context doesn't grab an extra ref on the membuf.
     * Parent rpc_task has bc_vec[] which holds a ref till the entire read
     * (possibly issued as multiple child reads) completes.
     */
    struct read_context *ctx = new read_context(this, &bc);

    do {
        READ3args args;

        args.file = inode->get_fh();
        args.offset = bc.offset + bc.pvt;
        args.count = bc.length - bc.pvt;

        /*
         * Increment the number of reads issued for the parent task.
         * This should not be incremented for a jukebox retried read since the
         * original read has already incremented the num_ongoing_backend_reads.
         */
        if (!is_jukebox_read) {
            rpc_api->parent_task->num_ongoing_backend_reads++;
        } else {
            assert(rpc_api->parent_task->num_ongoing_backend_reads > 0);
        }

        bc.num_backend_calls_issued++;

        AZLogDebug("Issuing read to backend at offset: {} length: {}",
                   args.offset, args.count);

        rpc_retry = false;
        stats.on_rpc_issue();
        if (rpc_nfs3_read_task(
                get_rpc_ctx(), /* This round robins request across connections */
                read_callback,
                bc.get_buffer() + bc.pvt,
                args.count,
                &args,
                (void *) ctx) == NULL) {
            stats.on_rpc_cancel();
            /*
             * Most common reason for this is memory allocation failure,
             * hence wait for some time before retrying. Also block the
             * current thread as we really want to slow down things.
             *
             * TODO: For soft mount should we fail this?
             */
            rpc_retry = true;

            AZLogWarn("rpc_nfs3_read_task failed to issue, retrying "
                      "after 5 secs!");
            ::sleep(5);
        }
    } while (rpc_retry);
}

void rpc_task::free_rpc_task()
{
    assert(get_op_type() <= FUSE_OPCODE_MAX);

    /*
     * Destruct anything allocated by the RPC API specific structs.
     * The rpc_api itself continues to exist. We avoid alloc/dealloc for every
     * task.
     */
    if (rpc_api) {
        rpc_api->release();
    }

    /*
     * Some RPCs store some data in the rpc_task directly.
     * That can be cleaned up here.
     */
    switch(get_op_type()) {
    case FUSE_READ:
        /*
         * rpc_api->parent_task will be nullptr for a parent task.
         * Only parent tasks run the fuse request so only they can have
         * bc_vec[] non-empty. Also only they can have read_status as
         * non-zero as the overall status of the fuse read is tracked by the
         * parent task.
         */
        assert(bc_vec.empty() || (rpc_api->parent_task == nullptr));
        assert((read_status == 0) || (rpc_api->parent_task == nullptr));
        assert(num_ongoing_backend_reads == 0);

        read_status = 0;
        bc_vec.clear();
        break;
    default :
        break;
    }

    stats.on_rpc_free();
    client->get_rpc_task_helper()->free_rpc_task(this);
}

struct nfs_context* rpc_task::get_nfs_context() const
{
    return client->get_nfs_context(csched, fh_hash);
}

void rpc_task::run_readdir()
{
    get_readdir_entries_from_cache();
}

void rpc_task::run_readdirplus()
{
    get_readdir_entries_from_cache();
}

/*
 * Callback for the READDIR RPC. Once this callback is called, it will first
 * populate the readdir cache with the newly fetched entries (minus the
 * attributes). Additionally it will populate the readdirentries vector and
 * call send_readdir_or_readdirplus_response() to respond to the fuse readdir
 * call.
 */
static void readdir_callback(
    struct rpc_context *rpc,
    int rpc_status,
    void *data,
    void *private_data)
{
    rpc_task *const task = (rpc_task*) private_data;
    assert(task->magic == RPC_TASK_MAGIC);

    assert(task->get_op_type() == FUSE_READDIR);
    READDIR3res *const res = (READDIR3res*) data;

    INJECT_JUKEBOX(res, task);
    INJECT_BAD_COOKIE(res, task);

    const fuse_ino_t dir_ino = task->rpc_api->readdir_task.get_ino();
    struct nfs_inode *const dir_inode =
        task->get_client()->get_nfs_inode_from_ino(dir_ino);
    // How many max bytes worth of entries data does the caller want?
    ssize_t rem_size = task->rpc_api->readdir_task.get_size();
    std::vector<std::shared_ptr<const directory_entry>> readdirentries;
    const int status = task->status(rpc_status, NFS_STATUS(res));
    bool eof = false;

    /*
     * readdir can be called on a directory after open()ing it, so we must have
     * created dircache.
     */
    assert(dir_inode->has_dircache());

    // For readdir we don't use parent_task to track the fuse request.
    assert(task->rpc_api->parent_task == nullptr);

    /*
     * Last valid offset seen for this directory enumeration. We keep on
     * updating this as we read entries from the returned list, so at any
     * point it contains the last cookie seen from the server and in case of
     * re-enumeration the next READDIR RPC should ask entries starting from
     * last_valid_offset+1.
     * Only used when re-enumerating.
     */
    off_t last_valid_offset = task->rpc_api->readdir_task.get_offset();

    /*
     * This tracks if we have got a "new entry" that we would like to send to
     * fuse. For the regular case (no re-enumeration) this is not very
     * interesting as all entries received are entries, but for re-enumeration
     * case this will be set only when we get an entry with cookie greater than
     * the target_offset (set when we received the NFS3ERR_BAD_COOKIE error).
     * Note that we will need to send response to fuse when either got_new_entry
     * is set or we got eof.
     */
    bool got_new_entry = false;

    const bool is_reenumerating =
        (task->rpc_api->readdir_task.get_target_offset() != 0);

    /*
     * Now that the request has completed, we can query libnfs for the
     * dispatch time.
     */
    task->get_stats().on_rpc_complete(rpc_get_pdu(rpc), NFS_STATUS(res));

    if (status == 0) {
        /*
         * Update attributes of parent directory returned in postop
         * attributes. If directory mtime has changed since the last time it'll
         * invalidate the cache.
         */
        UPDATE_INODE_ATTR(dir_inode, res->READDIR3res_u.resok.dir_attributes);

        const struct entry3 *entry = res->READDIR3res_u.resok.reply.entries;
        eof = res->READDIR3res_u.resok.reply.eof;
        int64_t eof_cookie = -1;
        int num_dirents = 0;

        // Get handle to the readdirectory cache.
        std::shared_ptr<readdirectory_cache>& dircache_handle =
            dir_inode->get_dircache();

        // Process all dirents received.
        while (entry) {
#ifdef ENABLE_PRESSURE_POINTS
            /*
             * Short readdir pressure point, skip when eof received, return
             * at least one entry.
             */
            if (inject_error() && !eof && num_dirents) {
                AZLogWarn("[{}] PP: short readdir after {} entries",
                          dir_ino, num_dirents);
                break;
            }
#endif

            /*
             * Blob NFS server should not send a cookie less than what we asked
             * for.
             */
            assert(entry->cookie > (uint64_t) last_valid_offset);
            last_valid_offset = entry->cookie;

            /*
             * Keep updating eof_cookie, when we exit the loop we will have
             * eof_cookie set correctly.
             */
            if (eof) {
                eof_cookie = entry->cookie;
            }

            /*
             * See if we have the directory_entry corresponding to this
             * cookie already present in the readdirectory_cache.
             * If so, we need to first remove the existing entry and add
             * this new entry.
             *
             * Then create the new directory entry which will be added to
             * readdirectory_cache, and also conveyed to fuse as part of
             * readdir response. The directory_entry added to the
             * readdirectory_cache will be freed when the directory cache
             * is purged (when fuse forgets the directory or under memory
             * pressure).
             *
             * TODO: Try to steal entry->name to avoid the strdup().
             */
            std::shared_ptr<struct directory_entry> dir_entry =
                dircache_handle->lookup(entry->cookie);

            if (dir_entry ) {
                assert(dir_entry->cookie == entry->cookie);
                if (dir_entry->nfs_inode) {
                    /*
                     * Drop the extra ref held by lookup().
                     * Original ref held by readdirectory_cache::add()
                     * must also be present, remove() will drop that.
                     */
                    assert(dir_entry->nfs_inode->dircachecnt >= 2);
                    dir_entry->nfs_inode->dircachecnt--;
                }

                /*
                 * Reset the dir_entry shared_ptr so that the subsequent
                 * remove() call can release the original shared_ptr ref
                 * on the directory_entry and free it.
                 */
                dir_entry.reset();
                dircache_handle->remove(entry->cookie);
            }

            dir_entry = std::make_shared<struct directory_entry>(
                                            strdup(entry->name),
                                            entry->cookie,
                                            entry->fileid);

            // Add to readdirectory_cache for future use.
            [[maybe_unused]] const bool added = dircache_handle->add(dir_entry);

#ifdef ENABLE_PARANOID
            if (added) {
                /*
                 * Entries added by readdir do not contribute to dnlc cache.
                 *
                 * Note: Technically there could be some other thread processing
                 *       readdirplus response for the same directory and it may
                 *       race with this thread, remove the above entry added by
                 *       readdir and add an entry by readdirplus. This is so
                 *       rare that we assert anyway.
                 */
                assert(dircache_handle->dnlc_lookup(dir_entry->name) == nullptr);
                assert(dir_inode->dnlc_lookup(dir_entry->name) == nullptr);
            }
#endif

            /*
             * If this is a re-enumeration callback, the target_offset would
             * be set to one more than the last cookie received before we got
             * the badcookie error, otherwise target_offset will be 0.
             * If we see something new here, this can mean one of the two:
             * - This is a regular (non re-enumeration) call.
             * - This is a re-enumeration call and we have seen a cookie >=
             *   target_offset, the last cookie seen before the badcookie error.
             * In either case, we need to return this this new entry (and
             * subsequent ones) to fuse.
             */
            got_new_entry = (((off_t) entry->cookie >=
                        task->rpc_api->readdir_task.get_target_offset()));

            // Only for re-enumeration case we can have got_new_entry as false.
            assert(got_new_entry || is_reenumerating);

            /*
             * If we found an entry that has not been sent before, we need to
             * add it to the directory_entry vector but ONLY upto the byte
             * limit requested by fuse readdir call.
             */
            if (got_new_entry && rem_size >= 0) {
                rem_size -= dir_entry->get_fuse_buf_size(false /* readdirplus */);
                if (rem_size >= 0) {
                    /*
                     * readdir_callback() MUST NOT return directory_entry with
                     * nfs_inode set.
                     */
                    assert(dir_entry->nfs_inode == nullptr);
                    readdirentries.push_back(dir_entry);
                }  else {
                    /*
                     * We are unable to add this entry to the fuse response
                     * buffer, so we won't notify fuse of this entry.
                     */
                    AZLogDebug("{}/{}: Couldn't fit in fuse response buffer",
                               dir_ino, dir_entry->name);
                }
            } else {
                AZLogDebug("{}/{}: Couldn't fit in fuse response buffer "
                           "or re-enumerating after NFS3ERR_BAD_COOKIE and did "
                           "not hit the target, cookie: {}, target_offset: {}, "
                           "rem_size: {}",
                           dir_ino, dir_entry->name,
                           dir_entry->cookie,
                           task->rpc_api->readdir_task.get_target_offset(),
                           rem_size);
            }

            entry = entry->nextentry;
            ++num_dirents;
        }

        AZLogDebug("readdir_callback {}: Num of entries returned by server is {}, "
                   "returned to fuse: {}, eof: {}, eof_cookie: {}",
                   is_reenumerating ? "(R)" : "",
                   num_dirents, readdirentries.size(), eof, eof_cookie);

        dircache_handle->set_cookieverf(&res->READDIR3res_u.resok.cookieverf);

        if (eof) {
            /*
             * If we pass the last cookie or beyond it, then server won't
             * return any directory entries, but it'll set eof to true.
             * In such case, we must already have set eof and eof_cookie.
             */
            if (eof_cookie != -1) {
                assert(num_dirents > 0);
                dircache_handle->set_eof(eof_cookie);
            } else {
                assert(num_dirents == 0);
                assert(readdirentries.size() == 0);
                if (dircache_handle->get_eof() != true) {
                    /*
                     * Server returned 0 entries and set eof to true, but the
                     * previous READDIR call that we made, for that server
                     * didn't return eof, this means the directory shrank in the
                     * server. To be safe, invalidate the cache.
                     */
                    AZLogWarn("[{}] readdir_callback {}: Directory shrank in "
                            "the server! cookie asked: {} target_offset: {}. "
                            "Purging cache!",
                            dir_ino, is_reenumerating ? "(R)" : "",
                            task->rpc_api->readdir_task.get_offset(),
                            task->rpc_api->readdir_task.get_target_offset());
                    dir_inode->invalidate_cache();
                } else {
                    assert((int64_t) dircache_handle->get_eof_cookie() != -1);
                }
            }
        }

        // Only send to fuse if we have seen new entries or EOF.
        if (got_new_entry || eof) {
            task->send_readdir_or_readdirplus_response(readdirentries);
            return;
        }
    } else if (NFS_STATUS(res) == NFS3ERR_JUKEBOX) {
        task->get_client()->jukebox_retry(task);
        return;
    } else if (NFS_STATUS(res) == NFS3ERR_BAD_COOKIE) {
        AZLogWarn("[{}] readdir_callback {}: got NFS3ERR_BAD_COOKIE for "
                  "offset: {}, clearing dircache and starting re-enumeration",
                  dir_ino,
                  is_reenumerating ? "(R)" : "",
                  task->rpc_api->readdir_task.get_offset());

        dir_inode->invalidate_cache();

        /*
         * We have received a bad cookie error, we have to restart enumeration
         * until either the server returns a valid response or we reach eof.
         * If we keep getting bad cookie we will keep on reenumerating forever.
         */
        last_valid_offset = 0;

        /*
         * We need to maintain the monotonocity of the target_offset
         * because it represents the offsets already sent to fuse as part of
         * this enumeration. This protects us from sending duplicate entries
         * to fuse if we receive bad_cookie before we reach the target during
         * reenumeration.
         * If this is the first bad_cookie error for this enumeration, then
         * target_offset must be set to "get_offset() + 1", else if it's a
         * re-enumeration and we again got a badcookie then the target_offset
         * must not be set less than the original target_offset.
         */
        task->rpc_api->readdir_task.set_target_offset(
                std::max(task->rpc_api->readdir_task.get_offset() + 1,
                         task->rpc_api->readdir_task.get_target_offset()));
    } else {
        task->reply_error(status);
        return;
    }

    /*
     * We have not seen a new entry and the call has not failed, hence this is a
     * reenumeration call and we have not reached the target_offset yet. We have to
     * start another readdir call for the next batch.
     * The assert has last_valid_offset==0 clause for cases where the callback
     * was called for a regular readdir (not re-enumerating) but it failed with
     * badcookie and hence we are here enumerating.
     */
    assert(!got_new_entry);
    assert(is_reenumerating || last_valid_offset == 0);
    assert(last_valid_offset <
            task->rpc_api->readdir_task.get_target_offset());
    assert(!eof);

    /*
     * Create a new child task to carry out this request.
     * Query cookies starting from last_valid_offset+1.
     * If re-enumeration, set the target_offset appropriately.
     */
    struct rpc_task *child_tsk =
        task->get_client()->get_rpc_task_helper()->alloc_rpc_task(FUSE_READDIR);

    child_tsk->init_readdir(
        task->rpc_api->req,
        task->rpc_api->readdir_task.get_ino(),
        task->rpc_api->readdir_task.get_size(),
        last_valid_offset,
        task->rpc_api->readdir_task.get_target_offset(),
        task->rpc_api->readdir_task.get_fuse_file());

    assert(child_tsk->rpc_api->parent_task == nullptr);

    AZLogDebug("[{}] readdir_callback{}: Re-enumerating from {} with "
               "target_offset {}",
               dir_ino, last_valid_offset,
               task->rpc_api->readdir_task.get_target_offset());

    /*
     * This will orchestrate a new readdir call and we will handle the response
     * in the callback. We already ensure we do not send duplicate entries to fuse.
     */
    child_tsk->fetch_readdir_entries_from_server();

    // Free the current task here, the child task will ensure a response is sent.
    task->free_rpc_task();
}

/*
 * Callback for the READDIR RPC. Once this callback is called, it will first
 * populate the readdir cache with the newly fetched entries (with the
 * attributes). Additionally it will populate the readdirentries vector and
 * call send_readdir_or_readdirplus_response() to respond to the fuse readdir call.
 *
 * TODO: Restart directory enumeration on getting NFS3ERR_BAD_COOKIE.
 */
static void readdirplus_callback(
    struct rpc_context *rpc,
    int rpc_status,
    void *data,
    void *private_data)
{
    rpc_task *const task = (rpc_task*) private_data;
    assert(task->magic == RPC_TASK_MAGIC);

    assert(task->get_op_type() == FUSE_READDIRPLUS);
    READDIRPLUS3res *const res = (READDIRPLUS3res*) data;

    INJECT_JUKEBOX(res, task);
    INJECT_BAD_COOKIE(res, task);

    const fuse_ino_t dir_ino = task->rpc_api->readdir_task.get_ino();
    struct nfs_inode *const dir_inode =
        task->get_client()->get_nfs_inode_from_ino(dir_ino);
    // How many max bytes worth of entries data does the caller want?
    ssize_t rem_size = task->rpc_api->readdir_task.get_size();
    std::vector<std::shared_ptr<const directory_entry>> readdirentries;
    const int status = task->status(rpc_status, NFS_STATUS(res));
    bool eof = false;

    /*
     * readdir can be called on a directory after open()ing it, so we must have
     * created dircache.
     */
    assert(dir_inode->has_dircache());

    // For readdir we don't use parent_task to track the fuse request.
    assert(task->rpc_api->parent_task == nullptr);

    /*
     * Last valid offset seen for this directory enumeration. We keep on
     * updating this as we read entries from the returned list, so at any
     * point it contains the last cookie seen from the server and in case of
     * re-enumeration the next READDIR RPC should ask entries starting from
     * last_valid_offset+1.
     * Only used when re-enumerating.
     */
    off_t last_valid_offset = task->rpc_api->readdir_task.get_offset();

    /*
     * This tracks if we have got a "new entry" that we would like to send to
     * fuse. For the regular case (no re-enumeration) this is not very
     * interesting as all entries received are entries, but for re-enumeration
     * case this will be set only when we get an entry with cookie greater than
     * the target_offset (set when we received the NFS3ERR_BAD_COOKIE error).
     * Note that we will need to send response to fuse when either got_new_entry
     * is set or we got eof.
     */
    bool got_new_entry = false;

    const bool is_reenumerating =
        (task->rpc_api->readdir_task.get_target_offset() != 0);

    /*
     * Now that the request has completed, we can query libnfs for the
     * dispatch time.
     */
    task->get_stats().on_rpc_complete(rpc_get_pdu(rpc), NFS_STATUS(res));

    if (status == 0) {
        /*
         * Update attributes of parent directory returned in postop
         * attributes. If directory mtime has changed since the last time it'll
         * invalidate the cache.
         */
        UPDATE_INODE_ATTR(dir_inode, res->READDIRPLUS3res_u.resok.dir_attributes);

        const struct entryplus3 *entry =
            res->READDIRPLUS3res_u.resok.reply.entries;
        eof = res->READDIRPLUS3res_u.resok.reply.eof;
        int64_t eof_cookie = -1;
        int num_dirents = 0;

        // Get handle to the readdirectory cache.
        std::shared_ptr<readdirectory_cache>& dircache_handle =
            dir_inode->get_dircache();

        // Process all dirents received.
        while (entry) {
#ifdef ENABLE_PRESSURE_POINTS
            /*
             * Short readdir pressure point, skip when eof received, return
             * at least one entry.
             */
            if (inject_error() && !eof && num_dirents) {
                AZLogWarn("[{}] PP: short readdirplus after {} entries",
                          dir_ino, num_dirents);
                break;
            }
#endif
            /*
             * Blob NFS server should not send a cookie less than what we asked
             * for.
             */
            assert(entry->cookie > (uint64_t) last_valid_offset);
            last_valid_offset = entry->cookie;

            const struct fattr3 *fattr = nullptr;
            /*
             * Keep updating eof_cookie, when we exit the loop we will have
             * eof_cookie set correctly.
             */
            if (eof) {
                eof_cookie = entry->cookie;
            }

            if (entry->name_attributes.attributes_follow) {
                fattr = &(entry->name_attributes.post_op_attr_u.attributes);

                // Blob NFS will never send these two different.
                assert(fattr->fileid == entry->fileid);
            }

            /*
             * Get the nfs inode for the entry.
             * Note that we first check if this inode exists (i.e., we have
             * conveyed it to fuse in the past and fuse has not FORGOTten it)
             * and if so use that, else create a new nfs_inode.
             * This will grab a lookupcnt ref on this inode. We will transfer
             * this same ref to fuse if we are able to successfully convey
             * this directory_entry to fuse.
             * We also increment forget_expected as fuse will call forget()
             * for these inodes.
             *
             * Note:  Caller must call decref() and decrement forget_expected
             *        for inodes corresponding to directory_entrys that are not
             *        returned to fuse.
             */
            struct nfs_inode *const nfs_inode =
                task->get_client()->get_nfs_inode(
                    &entry->name_handle.post_op_fh3_u.handle, fattr);
            nfs_inode->forget_expected++;

            if (!fattr) {
                /*
                 * If readdirplus entry doesn't carry attributes, then we
                 * just save the inode number and filetype as DT_UNKNOWN.
                 *
                 * Blob NFS though must always send attributes in a readdirplus
                 * response.
                 */
                assert(0);
                nfs_inode->get_attr_nolock().st_ino = entry->fileid;
                nfs_inode->get_attr_nolock().st_mode = 0;
            }

            /*
             * See if we have the directory_entry corresponding to this
             * cookie already present in the readdirectory_cache.
             * If so, we need to first remove the existing entry and add
             * this new entry. The existing entry may be created by readdir
             * in which case it won't have attributes stored or it could be
             * a readdirplus created entry in which case it will have inode
             * and attributes stored. If this is the last dircachecnt ref
             * on this inode remove() will also try to delete the inode.
             *
             * Then create the new directory entry which will be added to
             * readdirectory_cache, and also conveyed to fuse as part of
             * readdirplus response. The directory_entry added to the
             * readdirectory_cache will be freed when the directory cache
             * is purged (when fuse FORGETs the directory or under memory
             * pressure).
             *
             * TODO: Try to steal entry->name to avoid the strdup().
             */
            std::shared_ptr<struct directory_entry> dir_entry =
                dircache_handle->lookup(entry->cookie);

            if (dir_entry) {
                assert(dir_entry->cookie == entry->cookie);
                if (dir_entry->nfs_inode) {
                    /*
                     * Drop the extra ref held by lookup().
                     * Original ref held by readdirectory_cache::add()
                     * must also be present, remove() will drop that.
                     */
                    assert(dir_entry->nfs_inode->dircachecnt >= 2);
                    dir_entry->nfs_inode->dircachecnt--;
                }

                /*
                 * Reset the dir_entry shared_ptr so that the subsequent
                 * remove() call can release the original shared_ptr ref
                 * on the directory_entry, and also delete the inode if the
                 * lookupcnt ref is also 0.
                 */
                dir_entry.reset();
                dircache_handle->remove(entry->cookie);
            }

            /*
             * This dir_entry shared_ptr will hold one dircachecnt ref on
             * the inode. This will be transferred to the directory_entry
             * installed by the following add() call.
             */
            dir_entry = std::make_shared<struct directory_entry>(
                                                   strdup(entry->name),
                                                   entry->cookie,
                                                   nfs_inode->get_attr(),
                                                   nfs_inode);

            /*
             * dir_entry must have one ref on the inode.
             * This ref will protect the inode while this directory_entry is
             * present in the readdirectory_cache (added below).
             */
            assert(nfs_inode->dircachecnt >= 1);

            // Add to readdirectory_cache for future use.
            [[maybe_unused]] const bool added = dircache_handle->add(dir_entry);

#ifdef ENABLE_PARANOID
            if (added) {
                /*
                 * Now we should be able to perform dnlc lookup for
                 * dir_entry->name and it must yield nfs_inode. Try both from
                 * the dircache_handle and the inode.
                 *
                 * Note: This assert can fail under very rare circumstances.
                 *       See note in readdir_callback().
                 */
                struct nfs_inode *tmpi =
                    dircache_handle->dnlc_lookup(dir_entry->name);
                assert(tmpi == nfs_inode);
                tmpi->decref();

                tmpi = dir_inode->dnlc_lookup(dir_entry->name);
                assert(tmpi == nfs_inode);
                tmpi->decref();
            }
#endif

            /*
             * If this is a re-enumeration callback, the target_offset would
             * be set to one more than the last cookie received before we got
             * the badcookie error, otherwise target_offset will be 0.
             * If we see something new here, this can mean one of the two:
             * - This is a regular (non re-enumeration) call.
             * - This is a re-enumeration call and we have seen a cookie >=
             *   target_offset, the last cookie seen before the badcookie error.
             * In either case, we need to return this this new entry (and
             * subsequent ones) to fuse.
             */
            got_new_entry = (((off_t) entry->cookie >=
                        task->rpc_api->readdir_task.get_target_offset()));

            // Only for re-enumeration case we can have got_new_entry as false.
            assert(got_new_entry || is_reenumerating);

            /*
             * If we found an entry that has not been sent before, we need to
             * add it to the directory_entry vector but ONLY upto the byte
             * limit requested by fuse readdirplus call.
             */
            if (got_new_entry && rem_size >= 0) {
                rem_size -= dir_entry->get_fuse_buf_size(true /* readdirplus */);
                if (rem_size >= 0) {
                    /*
                     * Any directory_entry added must have the inode's lookupcnt
                     * ref and forget_expected bumped.
                     */
                    assert(dir_entry->nfs_inode);
                    assert(dir_entry->nfs_inode->forget_expected > 0);
                    assert(dir_entry->nfs_inode->lookupcnt > 0);

                    readdirentries.push_back(dir_entry);
                } else {
                    /*
                     * We are unable to add this entry to the fuse response
                     * buffer, so we won't notify fuse of this entry.
                     * Drop the ref held by get_nfs_inode().
                     */
                    AZLogDebug("[{}] {}/{}: Dropping ref since couldn't fit in "
                               "fuse response buffer",
                               nfs_inode->get_fuse_ino(),
                               dir_ino, dir_entry->name);
                    assert(nfs_inode->forget_expected > 0);
                    nfs_inode->forget_expected--;
                    dir_entry.reset();
                    nfs_inode->decref();
                }
            } else {
                AZLogDebug("[{}] {}/{}: Dropping ref since couldn't fit in "
                           "fuse response buffer or re-enumerating after "
                           "NFS3ERR_BAD_COOKIE and did not hit the target, "
                           "cookie: {}, target_offset: {}, rem_size: {}",
                           nfs_inode->get_fuse_ino(),
                           dir_ino, dir_entry->name,
                           dir_entry->cookie,
                           task->rpc_api->readdir_task.get_target_offset(),
                           rem_size);
                assert(nfs_inode->forget_expected > 0);
                nfs_inode->forget_expected--;
                dir_entry.reset();
                nfs_inode->decref();
            }

            entry = entry->nextentry;
            ++num_dirents;
        }

        AZLogDebug("readdirplus_callback {}: Num of entries returned by server "
                   "is {}, returned to fuse: {}, eof: {}, eof_cookie: {}",
                   is_reenumerating ? "(R)" : "",
                   num_dirents, readdirentries.size(), eof, eof_cookie);

        dircache_handle->set_cookieverf(&res->READDIRPLUS3res_u.resok.cookieverf);

        if (eof) {
            /*
             * If we pass the last cookie or beyond it, then server won't
             * return any directory entries, but it'll set eof to true.
             * In such case, we must already have set eof and eof_cookie.
             */
            if (eof_cookie != -1) {
                assert(num_dirents > 0);
                dircache_handle->set_eof(eof_cookie);
            } else {
                assert(readdirentries.size() == 0);
                if (dircache_handle->get_eof() != true) {
                    /*
                     * Server returned 0 entries and set eof to true, but the
                     * previous READDIR call that we made, for that server
                     * didn't return eof, this means the directory shrank in the
                     * server. To be safe, invalidate the cache.
                     */
                    AZLogWarn("[{}] readdirplus_callback {}: Directory shrank in "
                            "the server! cookie asked: {} target_offset: {}. "
                            "Purging cache!",
                            dir_ino, is_reenumerating ? "(R)" : "",
                            task->rpc_api->readdir_task.get_offset(),
                            task->rpc_api->readdir_task.get_target_offset());
                    dir_inode->invalidate_cache();
                } else {
                    assert((int64_t) dircache_handle->get_eof_cookie() != -1);
                }
            }
        }

        // Only send to fuse if we have seen new entries.
        if (got_new_entry || eof) {
            task->send_readdir_or_readdirplus_response(readdirentries);
            return;
        }
    } else if (NFS_STATUS(res) == NFS3ERR_JUKEBOX) {
        task->get_client()->jukebox_retry(task);
        return;
    } else if (NFS_STATUS(res) == NFS3ERR_BAD_COOKIE) {
        AZLogWarn("[{}] readdirplus_callback {}: got NFS3ERR_BAD_COOKIE for "
                  "offset: {}, clearing dircache and starting re-enumeration",
                  dir_ino,
                  is_reenumerating ? "(R)" : "",
                  task->rpc_api->readdir_task.get_offset());

        dir_inode->invalidate_cache();

        /*
         * We have received a bad cookie error, we have to restart enumeration
         * until either the server returns a valid response or we reach eof.
         * If we keep getting bad cookie we will keep on reenumerating forever.
         */
        last_valid_offset = 0;

        /*
         * We need to maintain the monotonocity of the target_offset
         * because it represents the offsets already sent to fuse as part of
         * this enumeration. This protects us from sending duplicate entries
         * to fuse if we receive bad_cookie before we reach the target during
         * reenumeration.
         * If this is the first bad_cookie error for this enumeration, then
         * target_offset must be set to "get_offset() + 1", else if it's a
         * re-enumeration and we again got a badcookie then the target_offset
         * must not be set less than the original target_offset.
         */
        task->rpc_api->readdir_task.set_target_offset(
                std::max(task->rpc_api->readdir_task.get_offset() + 1,
                         task->rpc_api->readdir_task.get_target_offset()));
    } else {
        task->reply_error(status);
        return;
    }

    /*
     * We have not seen a new entry and the call has not failed, hence this is a
     * reenumeration call and we have not reached the target_offset yet. We have to
     * start another readdirplus call for the next batch.
     * The assert has last_valid_offset==0 clause for cases where the callback
     * was called for a regular readdir (not re-enumerating) but it failed with
     * badcookie and hence we are here enumerating.
     */
    assert(!got_new_entry);
    assert(is_reenumerating || last_valid_offset == 0);
    assert(last_valid_offset <
            task->rpc_api->readdir_task.get_target_offset());
    assert(!eof);

    /*
     * Create a new child task to carry out this request.
     * Query cookies starting from last_valid_offset+1.
     * If re-enumeration, set the target_offset appropriately.
     */
    struct rpc_task *child_tsk =
        task->get_client()->get_rpc_task_helper()->alloc_rpc_task(FUSE_READDIRPLUS);
    child_tsk->init_readdirplus(
        task->rpc_api->req,
        task->rpc_api->readdir_task.get_ino(),
        task->rpc_api->readdir_task.get_size(),
        last_valid_offset,
        task->rpc_api->readdir_task.get_target_offset(),
        task->rpc_api->readdir_task.get_fuse_file());

    assert(child_tsk->rpc_api->parent_task == nullptr);

    AZLogDebug("[{}] readdirplus_callback{}: Re-enumerating from {} with "
               "target_offset {}",
               dir_ino, last_valid_offset,
               task->rpc_api->readdir_task.get_target_offset());

    /*
     * This will orchestrate a new readdir call and we will handle the response
     * in the callback. We already ensure we do not send duplicate entries to fuse.
     */
    child_tsk->fetch_readdirplus_entries_from_server();

    // Free the current task here, the child task will ensure a response is sent.
    task->free_rpc_task();
}

void rpc_task::get_readdir_entries_from_cache()
{
    const bool readdirplus = (get_op_type() == FUSE_READDIRPLUS);
    struct nfs_inode *nfs_inode =
        get_client()->get_nfs_inode_from_ino(rpc_api->readdir_task.get_ino());
    // Must have been allocated by opendir().
    assert(nfs_inode->has_dircache());
    bool is_eof = false;
    std::vector<std::shared_ptr<const directory_entry>> readdirentries;

    assert(nfs_inode->is_dir());

    /*
     * Query requested directory entries from the readdir cache.
     * Requested directory entries are the ones with cookie after the one
     * requested by the client.
     * Note that Blob NFS uses cookie values that increase by 1 for every file.
     */
    nfs_inode->lookup_dircache(rpc_api->readdir_task.get_offset() + 1,
                               rpc_api->readdir_task.get_size(),
                               readdirentries,
                               is_eof,
                               readdirplus);

    /*
     * If eof is already received don't ask any more entries from the server.
     */
    if (readdirentries.empty() && !is_eof) {
        /*
         * Read from the backend only if there is no entry present in the
         * cache.
         * Note: It is okay to send less number of entries than requested since
         *       the Fuse layer will request for more num of entries later.
         */
        if (readdirplus) {
            fetch_readdirplus_entries_from_server();
        } else {
            fetch_readdir_entries_from_server();
        }
    } else {
        /*
         * We are done fetching the entries, send the response now.
         * Note that after lookup_dircache() populated directory entries in
         * readdirentries, one or more of these directory_entry can get freed,
         * f.e., we may receive an unlink() call for those. silly_rename() will
         * remove the corresponding directory_entry from the readdirectory_cache
         * which will cause the directory_entry to be freed. Obviously we hold
         * a ref on the directory_entry->inode so the inode will be accessible.
         * readdirentries holds shared_ptr of directory_entry objects, so even
         * if unlink deletes the directory_entry from readdirectory_cache, our
         * ref is valid and we can safely return it to fuse. Fuse will later
         * call forget() for this inode and then we will free the inode.
         * Note that since the file doesn't really exist now, any lookup() or
         * unlink() call will fail with ENOENT.
         */
        send_readdir_or_readdirplus_response(readdirentries);
    }
}

void rpc_task::fetch_readdir_entries_from_server()
{
    bool rpc_retry;
    const fuse_ino_t dir_ino = rpc_api->readdir_task.get_ino();
    struct nfs_inode *dir_inode = get_client()->get_nfs_inode_from_ino(dir_ino);
    assert(dir_inode->has_dircache());
    const cookie3 cookie = rpc_api->readdir_task.get_offset();

    do {
        READDIR3args args;

        args.dir = dir_inode->get_fh();
        args.cookie = cookie;
        ::memcpy(&args.cookieverf,
                 dir_inode->get_dircache()->get_cookieverf(),
                 sizeof(args.cookieverf));

        args.count = nfs_get_readdir_maxcount(get_nfs_context());

        rpc_retry = false;
        stats.on_rpc_issue();
        if (rpc_nfs3_readdir_task(get_rpc_ctx(),
                                  readdir_callback,
                                  &args,
                                  this) == NULL) {
            stats.on_rpc_cancel();
            /*
             * Most common reason for this is memory allocation failure,
             * hence wait for some time before retrying. Also block the
             * current thread as we really want to slow down things.
             *
             * TODO: For soft mount should we fail this?
             */
            rpc_retry = true;

            AZLogWarn("rpc_nfs3_readdir_task failed to issue, retrying "
                      "after 5 secs!");
            ::sleep(5);
        }
    } while (rpc_retry);
}

void rpc_task::fetch_readdirplus_entries_from_server()
{
    bool rpc_retry;
    const fuse_ino_t dir_ino = rpc_api->readdir_task.get_ino();
    struct nfs_inode *dir_inode = get_client()->get_nfs_inode_from_ino(dir_ino);
    assert(dir_inode->has_dircache());
    const cookie3 cookie = rpc_api->readdir_task.get_offset();

    do {
        READDIRPLUS3args args;

        args.dir = dir_inode->get_fh();
        args.cookie = cookie;
        ::memcpy(&args.cookieverf,
                 dir_inode->get_dircache()->get_cookieverf(),
                 sizeof(args.cookieverf));

        /*
         * Use dircount/maxcount according to the user configured and
         * the server advertised value.
         */
        args.maxcount = nfs_get_readdir_maxcount(get_nfs_context());
        args.dircount = args.maxcount;

        rpc_retry = false;
        stats.on_rpc_issue();
        if (rpc_nfs3_readdirplus_task(get_rpc_ctx(),
                                      readdirplus_callback,
                                      &args,
                                      this) == NULL) {
            stats.on_rpc_cancel();
            /*
             * Most common reason for this is memory allocation failure,
             * hence wait for some time before retrying. Also block the
             * current thread as we really want to slow down things.
             *
             * TODO: For soft mount should we fail this?
             */
            rpc_retry = true;

            AZLogWarn("rpc_nfs3_readdirplus_task failed to issue, retrying "
                      "after 5 secs!");
            ::sleep(5);
        }
    } while (rpc_retry);
}

/*
 * readdirentries vector passed to send_readdir_or_readdirplus_response() has
 * one or more directory_entry with following properties:
 * - For READDIRPLUS response every directory_entry MUST have a valid nfs_inode
 *   pointer.
 * - For READDIR response directory_entry may or may not have a valid nfs_inode
 *   pointer.
 * - If directory_entry has a valid nfs_inode pointer then the caller MUST have
 *   held a lookupcnt ref on the inode. It must have additionally incremented
 *   forget_expected.
 *
 * When responding to READDIRPLUS, fuse wants us to hold a ref on each inode
 * corresponding to a file/dir which is not "." or "..", as it'll call forget
 * for each of these.
 * When responding to READDIR, fuse doesn't want us to hold a ref on any inode.
 * This means:
 * - For READDIR send_readdir_or_readdirplus_response() MUST call decref() for
 *   each inode, and drop forget_expected.
 * - For READDIRPLUS and successful callback to fuse, it MUST call decref() for
 *   "." and ".." (for the rest fuse will call forget), and drop forget_expected.
 * - For READDIRPLUS and failed callback to fuse, it MUST call decref() for each
 *   inode and drop forget_expected.
 * - Any directory_entry that could not be packed in the fuse response, if the
 *   directory_entry has a valid nfs_inode, then it MUST call decref() for the
 *   inode and decrement forget_expected.
 *
 * TODO: While this is processing readdirentries, can those directory_entry
 *       objects be modified? Note that they are present in inode->dir_entries
 *       and we have just a shared_ptr to them.
 */
void rpc_task::send_readdir_or_readdirplus_response(
    const std::vector<std::shared_ptr<const directory_entry>>& readdirentries)
{
    const bool readdirplus = (get_op_type() == FUSE_READDIRPLUS);
    /*
     * Max size the fuse buf is allowed to take.
     * We will allocate this much and then fill as many directory entries can
     * fit in this. Since the caller would have also considered this same size
     * while filling entries in readdirentries, we will usually be able to
     * consume all the entries in readdirentries.
     */
    const size_t size = rpc_api->readdir_task.get_size();
    const fuse_ino_t parent_ino = rpc_api->readdir_task.get_ino();

    // Fuse always requests 4096 bytes.
    assert(size >= 4096);

    // Allocate fuse response buffer.
    char *buf1 = (char *) ::malloc(size);
    if (!buf1) {
        reply_error(ENOMEM);
        return;
    }

    char *current_buf = buf1;
    size_t rem = size;
    size_t num_entries_added = 0;

    AZLogDebug("send_readdir_or_readdirplus_response: Number of directory"
               " entries to send {}, size: {}",
               readdirentries.size(), size);

    for (auto& it : readdirentries) {
        /*
         * Caller should make sure that it adds only directory entries after
         * what was requested in the READDIR{PLUS} call to readdirentries.
         */
        assert((uint64_t) it->cookie >
                (uint64_t) rpc_api->readdir_task.get_offset());
        size_t entsize;

        if (readdirplus) {
            /*
             * For readdirplus, caller MUST have set the inode and bumped
             * lookupcnt ref and forget_expected.
             */
            assert(it->nfs_inode);
            assert(it->nfs_inode->lookupcnt > 0);
            assert(it->nfs_inode->forget_expected > 0);

            struct fuse_entry_param fuseentry;

#ifdef ENABLE_PARANOID
            /*
             * it->attributes are copied from nfs_inode->attr at the time when
             * the directory_entry was created, after that inode's ctime can
             * only go forward.
             *
             * TODO: Remove directory_entry->attributes if we don't need them.
             */
			{
				std::shared_lock<std::shared_mutex> lock(it->nfs_inode->ilock_1);
				assert((::memcmp(&it->attributes,
								 &it->nfs_inode->get_attr_nolock(),
                                 sizeof(struct stat)) == 0) ||
					   (compare_timespec(it->attributes.st_ctim,
										 it->nfs_inode->get_attr_nolock().st_ctim) < 0));
				assert(it->attributes.st_ino == it->nfs_inode->get_attr_nolock().st_ino);
			}
#endif

            // We don't need the memset as we are setting all members.
            fuseentry.attr = it->nfs_inode->get_attr();
            fuseentry.ino = it->nfs_inode->get_fuse_ino();
            fuseentry.generation = it->nfs_inode->get_generation();
            fuseentry.attr_timeout = it->nfs_inode->get_actimeo();
            fuseentry.entry_timeout = it->nfs_inode->get_actimeo();

            AZLogDebug("[{}] <{}> Returning ino {} to fuse (filename: {}, "
                       "lookupcnt: {}, dircachecnt: {}, forget_expected: {})",
                       parent_ino,
                       rpc_task::fuse_opcode_to_string(rpc_api->optype),
                       fuseentry.ino,
                       it->name,
                       it->nfs_inode->lookupcnt.load(),
                       it->nfs_inode->dircachecnt.load(),
                       it->nfs_inode->forget_expected.load());

            /*
             * Insert the entry into the buffer.
             * If the buffer space is less, fuse_add_direntry_plus will not
             * add entry to the buffer but will still return the space needed
             * to add this entry.
             */
            entsize = fuse_add_direntry_plus(get_fuse_req(),
                                             current_buf,
                                             rem, /* size left in the buffer */
                                             it->name,
                                             &fuseentry,
                                             it->cookie);
        } else {
            /*
             * Insert the entry into the buffer.
             * If the buffer space is less, fuse_add_direntry will not add
             * entry to the buffer but will still return the space needed to
             * add this entry.
             */
            entsize = fuse_add_direntry(get_fuse_req(),
                                        current_buf,
                                        rem, /* size left in the buffer */
                                        it->name,
                                        &it->attributes,
                                        it->cookie);
        }

        /*
         * Our buffer size was small and hence we can't add any more entries,
         * so just break the loop. This also means that we have not inserted
         * the current entry to the dirent buffer.
         *
         * Note: This should not happen since the caller would have filled
         *       just enough entries in readdirentries.
         */
        if (entsize > rem) {
            break;
        }

#ifdef ENABLE_PRESSURE_POINTS
        if (num_entries_added > 0) {
            if (inject_error()) {
                AZLogWarn("[{}] PP: sending less directory entries to fuse, "
                          "size: {}, rem: {}, num_entries_added: {}",
                          parent_ino, size, rem, num_entries_added);
                break;
            }
        }
#endif

        // Increment the buffer pointer to point to the next free space.
        current_buf += entsize;
        rem -= entsize;
        num_entries_added++;

        if (readdirplus) {
            assert(it->nfs_inode);
            assert(it->nfs_inode->lookupcnt > 0);
            assert(it->nfs_inode->forget_expected > 0);

            /*
             * Caller would have bumped lookupcnt ref and forget_expected fpr
             * *all* entries, fuse expects lookupcnt of every entry returned
             * by readdirplus(), except "." and "..", to be incremented, so
             * drop ref and forget_expected for "." and "..".
             *
             * Note: We clear it->nfs_inode so that if fuse_reply_buf() fails
             *       and we need to drop lookupcnt ref and forget_expected for
             *       all the entries, we don't drop them again for these inodes.
             */
            if (it->is_dot_or_dotdot()) {
                it->nfs_inode->forget_expected--;
                it->nfs_inode->decref();
            }
        } else if (it->nfs_inode) {
            /*
             * For READDIR response, we need to drop lookupcnt ref and
             * forget_expected for all entries with a valid inode.
             *
             * Note: entry->nfs_inode may be null for entries populated using
             *       only readdir however, it is guaranteed to be present for
             *       readdirplus.
             */
            assert(it->nfs_inode->forget_expected > 0);
            it->nfs_inode->forget_expected--;
            it->nfs_inode->decref();
        }
    }

    /*
     * startidx is the starting index into readdirentries vector from where
     * we start cleaning up. In case of error this will be reset to 0, else
     * it's set to num_entries_added.
     */
    size_t startidx = num_entries_added;
    bool inject_fuse_reply_buf_failure = false;

    /*
     * XXX Applications don't seem to handle EINVAL return from getdents()
     *     as expected, i.e., they don't retry the call with a bigger buffer.
     *     Instead they treat it as an error. Keep it disabled.
     */
#if 0
#ifdef ENABLE_PRESSURE_POINTS
    inject_fuse_reply_buf_failure = inject_error();
#endif
#endif

    if (!inject_fuse_reply_buf_failure) {
        AZLogDebug("Num of entries sent in readdir response is {}", num_entries_added);

        if (fuse_reply_buf(get_fuse_req(), buf1, size - rem) != 0) {
            AZLogError("fuse_reply_buf failed!");
            startidx = 0;
        }
    } else {
        AZLogWarn("[{}] PP: injecting fuse_reply_buf() failure, "
                  "size: {}, rem: {}, num_entries_added: {}",
                  parent_ino, size, rem, num_entries_added);
        startidx = 0;
    }

    for (size_t i = startidx; i < readdirentries.size(); i++) {
        const std::shared_ptr<const directory_entry>& it = readdirentries[i];
        /*
         * If directory_entry doesn't have a valid inode, no cleanup to do.
         */
        if (!it->nfs_inode) {
            assert(!readdirplus);
            continue;
        }

        /*
         * Till num_entries_added we have dropped the lookupcnt ref and
         * forget_expected for:
         * - "." amd ".." for readdirplus.
         * - all for readdir.
         * Skip those now.
         * Beyond num_entries_added, we have to drop for all.
         */
        if (i < num_entries_added) {
            if (!readdirplus || it->is_dot_or_dotdot()) {
                continue;
            }
        }

        AZLogDebug("[{}] Dropping lookupcnt, now {}, "
                   "forget_expected: {}",
                   it->nfs_inode->get_fuse_ino(),
                   it->nfs_inode->lookupcnt.load(),
                   it->nfs_inode->forget_expected.load());
        assert(it->nfs_inode->forget_expected > 0);
        it->nfs_inode->forget_expected--;
        it->nfs_inode->decref();
    }

    free(buf1);

    if (!inject_fuse_reply_buf_failure) {
        free_rpc_task();
    } else {
        /*
         * EINVAL return from getdents() imply insufficient buffer, so caller
         * should retry.
         */
        reply_error(EINVAL);
    }
}
