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

    fh_hash = get_client()->get_nfs_inode_from_ino(parent_ino)->get_crc();
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
                           const char *name)
{
    assert(get_op_type() == FUSE_UNLINK);
    set_fuse_req(request);
    rpc_api->unlink_task.set_parent_ino(parent_ino);
    rpc_api->unlink_task.set_file_name(name);

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
                            struct fuse_file_info *file)
{
    assert(get_op_type() == FUSE_READDIR);
    set_fuse_req(request);
    rpc_api->readdir_task.set_ino(ino);
    rpc_api->readdir_task.set_size(size);
    rpc_api->readdir_task.set_offset(offset);
    rpc_api->readdir_task.set_fuse_file(file);

    fh_hash = get_client()->get_nfs_inode_from_ino(ino)->get_crc();
}

void rpc_task::init_readdirplus(fuse_req *request,
                                fuse_ino_t ino,
                                size_t size,
                                off_t offset,
                                struct fuse_file_info *file)
{
    assert(get_op_type() == FUSE_READDIRPLUS);
    set_fuse_req(request);
    rpc_api->readdir_task.set_ino(ino);
    rpc_api->readdir_task.set_size(size);
    rpc_api->readdir_task.set_offset(offset);
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
        inode->update(res->GETATTR3res_u.resok.obj_attributes);

        /*
         * Set fuse kernel attribute cache timeout to the current attribute
         * cache timeout for this inode, as per the recent revalidation
         * experience.
         */
        task->reply_attr(&inode->attr, inode->get_actimeo());
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
    const int status = task->status(rpc_status, NFS_STATUS(res));

    /*
     * Kernel must cache -ve entries.
     */
    const bool cache_negative =
        (aznfsc_cfg.lookupcache_int == AZNFSCFG_LOOKUPCACHE_ALL);

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
        assert(res->LOOKUP3res_u.resok.obj_attributes.attributes_follow);
        task->get_client()->reply_entry(
            task,
            &res->LOOKUP3res_u.resok.object,
            &res->LOOKUP3res_u.resok.obj_attributes.post_op_attr_u.attributes,
            nullptr);
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
    const int status = task->status(rpc_status, NFS_STATUS(res));

    /*
     * Now that the request has completed, we can query libnfs for the
     * dispatch time.
     */
    task->get_stats().on_rpc_complete(rpc_get_pdu(rpc), NFS_STATUS(res));

    if (NFS_STATUS(res) == NFS3ERR_JUKEBOX) {
        task->get_client()->jukebox_retry(task);
    } else {
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
    struct rpc_pdu *pdu;
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

        if ((pdu = rpc_nfs3_writev_task(get_rpc_ctx(),
                                        write_iov_callback, &args,
                                        bciov->iov,
                                        bciov->iovcnt,
                                        this)) == NULL) {
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
    const int status = task->status(rpc_status, NFS_STATUS(res));

    /*
     * Now that the request has completed, we can query libnfs for the
     * dispatch time.
     */
    task->get_stats().on_rpc_complete(rpc_get_pdu(rpc), NFS_STATUS(res));

    if (status == 0) {
        struct statvfs st;
        ::memset(&st, 0, sizeof(st));
        st.f_bsize = NFS_BLKSIZE;
        st.f_blocks = res->FSSTAT3res_u.resok.tbytes / NFS_BLKSIZE;
        st.f_bfree = res->FSSTAT3res_u.resok.fbytes / NFS_BLKSIZE;
        st.f_bavail = res->FSSTAT3res_u.resok.abytes / NFS_BLKSIZE;
        st.f_files = res->FSSTAT3res_u.resok.tfiles;
        st.f_ffree = res->FSSTAT3res_u.resok.ffiles;
        st.f_favail = res->FSSTAT3res_u.resok.afiles;

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
    const int status = task->status(rpc_status, NFS_STATUS(res));

    /*
     * Now that the request has completed, we can query libnfs for the
     * dispatch time.
     */
    task->get_stats().on_rpc_complete(rpc_get_pdu(rpc), NFS_STATUS(res));

    if (status == 0) {
        assert(
            res->CREATE3res_u.resok.obj.handle_follows &&
            res->CREATE3res_u.resok.obj_attributes.attributes_follow);

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

    if (status == 0) {
        assert(res->SETATTR3res_u.resok.obj_wcc.after.attributes_follow);

        /*
         * Update the cached inode attributes from the postop attributes
         * received in this response.
         */
        inode->update(res->SETATTR3res_u.resok.obj_wcc.after.post_op_attr_u.attributes);

        struct stat st;

        task->get_client()->stat_from_fattr3(
            &st, &res->SETATTR3res_u.resok.obj_wcc.after.post_op_attr_u.attributes);

        /*
         * Set fuse kernel attribute cache timeout to the current attribute
         * cache timeout for this inode, as per the recent revalidation
         * experience.
         */
        task->reply_attr(&st, inode->get_actimeo());
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
    const int status = task->status(rpc_status, NFS_STATUS(res));

    /*
     * Now that the request has completed, we can query libnfs for the
     * dispatch time.
     */
    task->get_stats().on_rpc_complete(rpc_get_pdu(rpc), NFS_STATUS(res));

    if (status == 0) {
        assert(
            res->CREATE3res_u.resok.obj.handle_follows &&
            res->CREATE3res_u.resok.obj_attributes.attributes_follow);

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
    const int status = task->status(rpc_status, NFS_STATUS(res));

    /*
     * Now that the request has completed, we can query libnfs for the
     * dispatch time.
     */
    task->get_stats().on_rpc_complete(rpc_get_pdu(rpc), NFS_STATUS(res));

    if (status == 0) {
        assert(
            res->MKDIR3res_u.resok.obj.handle_follows &&
            res->MKDIR3res_u.resok.obj_attributes.attributes_follow);

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
    const int status = task->status(rpc_status, NFS_STATUS(res));

    /*
     * Now that the request has completed, we can query libnfs for the
     * dispatch time.
     */
    task->get_stats().on_rpc_complete(rpc_get_pdu(rpc), NFS_STATUS(res));

    if (NFS_STATUS(res) == NFS3ERR_JUKEBOX) {
        task->get_client()->jukebox_retry(task);
    } else {
        task->reply_error(status);
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
    const int status = task->status(rpc_status, NFS_STATUS(res));

    /*
     * Now that the request has completed, we can query libnfs for the
     * dispatch time.
     */
    task->get_stats().on_rpc_complete(rpc_get_pdu(rpc), NFS_STATUS(res));

    if (NFS_STATUS(res) == NFS3ERR_JUKEBOX) {
        task->get_client()->jukebox_retry(task);
    } else {
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
    const int status = task->status(rpc_status, NFS_STATUS(res));

    /*
     * Now that the request has completed, we can query libnfs for the
     * dispatch time.
     */
    task->get_stats().on_rpc_complete(rpc_get_pdu(rpc), NFS_STATUS(res));

    if (status == 0) {
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
    const bool silly_rename = task->rpc_api->rename_task.get_silly_rename();
    auto res = (RENAME3res*) data;
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
        struct nfs_client *client = task->get_client();
        assert(client->magic == NFS_CLIENT_MAGIC);
        struct nfs_inode *silly_rename_inode =
            client->get_nfs_inode_from_ino(silly_rename_ino);
        assert(silly_rename_inode->magic == NFS_INODE_MAGIC);

        // Silly rename has the same source and target dir.
        assert(task->rpc_api->rename_task.get_parent_ino() ==
               task->rpc_api->rename_task.get_newparent_ino());

        silly_rename_inode->silly_renamed_name =
            task->rpc_api->rename_task.get_newname();
        silly_rename_inode->parent_ino =
            task->rpc_api->rename_task.get_newparent_ino();
        silly_rename_inode->is_silly_renamed = true;

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
    const int status = task->status(rpc_status, NFS_STATUS(res));

    /*
     * Now that the request has completed, we can query libnfs for the
     * dispatch time.
     */
    task->get_stats().on_rpc_complete(rpc_get_pdu(rpc), NFS_STATUS(res));

    if (status == 0) {
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
    bool rpc_retry;
    rpc_pdu *pdu = nullptr;

    do {
        LOOKUP3args args;
        args.what.dir = get_client()->get_nfs_inode_from_ino(parent_ino)->get_fh();
        args.what.name = (char*) rpc_api->lookup_task.get_file_name();

        rpc_retry = false;
        /*
         * Note: Once we call the libnfs async method, the callback can get
         *       called anytime after that, even before it returns to the
         *       caller. Since callback can free the task, it's not safe to
         *       access the task object after making the libnfs call.
         */
        stats.on_rpc_issue();
        if ((pdu = rpc_nfs3_lookup_task(get_rpc_ctx(), lookup_callback, &args,
                                 this)) == NULL) {
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
    rpc_pdu *pdu = nullptr;

    do {
        ACCESS3args args;
        args.object = get_client()->get_nfs_inode_from_ino(ino)->get_fh();
        args.access = rpc_api->access_task.get_mask();

        rpc_retry = false;
        stats.on_rpc_issue();
        if ((pdu = rpc_nfs3_access_task(get_rpc_ctx(), access_callback, &args,
                                        this)) == NULL) {
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
     * Copy application data into chunk cache and initiate writes for all
     * membufs. We don't wait for the writes to actually finish, which means
     * we support buffered writes.
     */
    error_code = inode->copy_to_cache(bufv, offset,
                                      &extent_left, &extent_right);
    if (error_code != 0) {
        AZLogWarn("[{}] copy_to_cache failed with error={}, "
                  "failing write!", ino, error_code);
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
        inode->filecache_handle->max_dirty_extent_bytes();
    assert(max_dirty_extent > 0);

    /*
     * How many bytes in the cache need to be flushed.
     */
    const uint64_t bytes_to_flush =
        inode->filecache_handle->get_bytes_to_flush();

    AZLogDebug("extent_left: {}, extent_right: {}, size: {}, "
               "bytes_to_flush: {} (max_dirty_extent: {})",
               extent_left, extent_right,
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

    std::vector<bytes_chunk> bc_vec =
        inode->filecache_handle->get_dirty_bc_range(extent_left, extent_right);

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
    rpc_pdu *pdu = nullptr;

    do {
        GETATTR3args args;

        args.object = get_client()->get_nfs_inode_from_ino(ino)->get_fh();

        rpc_retry = false;
        stats.on_rpc_issue();
        if ((pdu = rpc_nfs3_getattr_task(get_rpc_ctx(), getattr_callback, &args,
                                  this)) == NULL) {
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

void rpc_task::run_statfs()
{
    bool rpc_retry;
    auto ino = rpc_api->statfs_task.get_ino();
    rpc_pdu *pdu = nullptr;

    do {
        FSSTAT3args args;
        args.fsroot = get_client()->get_nfs_inode_from_ino(ino)->get_fh();

        rpc_retry = false;
        stats.on_rpc_issue();
        if ((pdu = rpc_nfs3_fsstat_task(get_rpc_ctx(), statfs_callback, &args,
                                 this)) == NULL) {
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
    rpc_pdu *pdu = nullptr;

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
        if ((pdu = rpc_nfs3_create_task(get_rpc_ctx(), createfile_callback, &args,
                                 this)) == NULL) {
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
    rpc_pdu *pdu = nullptr;

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
        if ((pdu = rpc_nfs3_create_task(get_rpc_ctx(), mknod_callback, &args,
                                 this)) == NULL) {
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
    rpc_pdu *pdu = nullptr;

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
        if ((pdu = rpc_nfs3_mkdir_task(get_rpc_ctx(), mkdir_callback, &args,
                                this)) == NULL) {
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
    rpc_pdu *pdu = nullptr;

    do {
        REMOVE3args args;
        args.object.dir = get_client()->get_nfs_inode_from_ino(parent_ino)->get_fh();
        args.object.name = (char*) rpc_api->unlink_task.get_file_name();

        rpc_retry = false;
        stats.on_rpc_issue();
        if ((pdu = rpc_nfs3_remove_task(get_rpc_ctx(),
                                 unlink_callback, &args, this)) == NULL) {
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
    rpc_pdu *pdu = nullptr;

    do {
        RMDIR3args args;

        args.object.dir = get_client()->get_nfs_inode_from_ino(parent_ino)->get_fh();
        args.object.name = (char*) rpc_api->rmdir_task.get_dir_name();

        rpc_retry = false;
        stats.on_rpc_issue();
        if ((pdu = rpc_nfs3_rmdir_task(get_rpc_ctx(),
                                rmdir_callback, &args, this)) == NULL) {
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
    rpc_pdu *pdu = nullptr;

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
        if ((pdu = rpc_nfs3_symlink_task(get_rpc_ctx(),
                                         symlink_callback,
                                         &args,
                                         this)) == NULL) {
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

    rpc_pdu *pdu = nullptr;

    do {
        RENAME3args args;
        args.from.dir = get_client()->get_nfs_inode_from_ino(parent_ino)->get_fh();
        args.from.name = (char*) rpc_api->rename_task.get_name();
        args.to.dir = get_client()->get_nfs_inode_from_ino(newparent_ino)->get_fh();
        args.to.name = (char*) rpc_api->rename_task.get_newname();

        rpc_retry = false;
        stats.on_rpc_issue();
        if ((pdu = rpc_nfs3_rename_task(get_rpc_ctx(),
                                        rename_callback,
                                        &args,
                                        this)) == NULL) {
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
    rpc_pdu *pdu = nullptr;

    do {
        READLINK3args args;
        args.symlink = get_client()->get_nfs_inode_from_ino(ino)->get_fh();

        rpc_retry = false;
        stats.on_rpc_issue();
        if ((pdu = rpc_nfs3_readlink_task(get_rpc_ctx(),
                                          readlink_callback,
                                          &args,
                                          this)) == NULL) {
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
    rpc_pdu *pdu = nullptr;

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
        reply_attr(&inode->attr, inode->get_actimeo());
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
        if ((pdu = rpc_nfs3_setattr_task(get_rpc_ctx(), setattr_callback, &args,
                                  this)) == NULL) {
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
    std::shared_ptr<bytes_chunk_cache>& filecache_handle =
        inode->filecache_handle;

    assert(inode->is_regfile());
    // Must have been allocated in open().
    assert(filecache_handle);

    /*
     * run_read() is called once for a fuse read request and must not be
     * called for a child task.
     */
    assert(rpc_api->parent_task == nullptr);

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
     * here.
     */
    assert(bc->pvt < bc->length);

    const char* errstr;
    auto res = (READ3res*)data;
    const int status = (task->status(rpc_status, NFS_STATUS(res), &errstr));
    fuse_ino_t ino = task->rpc_api->read_task.get_ino();
    struct nfs_inode *inode = task->get_client()->get_nfs_inode_from_ino(ino);
    auto filecache_handle = inode->filecache_handle;
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

        AZLogDebug("[{}] read_callback: {}Read completed for offset: {} "
                   " size: {} Bytes read: {} eof: {}, total bytes read till "
                   "now: {} of {} for [{}, {}) num_backend_calls_issued: {}",
                   ino,
                   is_partial_read ? "Partial " : "",
                   issued_offset,
                   issued_length,
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
                rpc_pdu *pdu = nullptr;
                rpc_retry = false;
                child_tsk->get_stats().on_rpc_issue();
                if ((pdu = rpc_nfs3_read_task(
                        child_tsk->get_rpc_ctx(),
                        read_callback,
                        bc->get_buffer() + bc->pvt,
                        new_size,
                        &new_args,
                        (void *) new_ctx)) == NULL) {
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

        if (bc->is_empty && (bc->length == bc->pvt)) {
            /*
             * Only the first read which got hold of the complete membuf
             * will have this byte_chunk set to empty.
             * Only such reads should set the uptodate flag.
             * Also the uptodate flag should be set only if we have read
             * the entire membuf.
             */
            AZLogDebug("[{}] Setting uptodate flag. offset: {}, length: {}",
                       ino,
                       task->rpc_api->read_task.get_offset(),
                       task->rpc_api->read_task.get_size());

            assert(bc->maps_full_membuf());
            bc->get_membuf()->set_uptodate();
        } else {
            /*
             * If we got eof in a partial read, release the non-existent
             * portion of the chunk.
             */
            if (bc->is_empty && (bc->length > bc->pvt) &&
                res->READ3res_u.resok.eof) {
                filecache_handle->release(bc->offset + bc->pvt,
                                          bc->length - bc->pvt);
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

        rpc_pdu *pdu = nullptr;
        rpc_retry = false;
        stats.on_rpc_issue();
        if ((pdu = rpc_nfs3_read_task(
                get_rpc_ctx(), /* This round robins request across connections */
                read_callback,
                bc.get_buffer() + bc.pvt,
                args.count,
                &args,
                (void *) ctx)) == NULL) {
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
 * call send_readdir_response() to respond to the fuse readdir call.
 *
 * TODO: Restart directory enumeration on getting NFS3ERR_BAD_COOKIE.
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
    const fuse_ino_t dir_ino = task->rpc_api->readdir_task.get_ino();
    struct nfs_inode *const dir_inode =
        task->get_client()->get_nfs_inode_from_ino(dir_ino);
    // How many max bytes worth of entries data does the caller want?
    ssize_t rem_size = task->rpc_api->readdir_task.get_size();
    std::vector<const directory_entry*> readdirentries;
    int num_dirents = 0;
    const int status = task->status(rpc_status, NFS_STATUS(res));

    /*
     * Now that the request has completed, we can query libnfs for the
     * dispatch time.
     */
    task->get_stats().on_rpc_complete(rpc_get_pdu(rpc), NFS_STATUS(res));

    if (status == 0) {
        const struct entry3 *entry = res->READDIR3res_u.resok.reply.entries;
        const bool eof = res->READDIR3res_u.resok.reply.eof;
        int64_t eof_cookie = -1;

        // Get handle to the readdirectory cache.
        std::shared_ptr<readdirectory_cache>& dircache_handle =
            dir_inode->dircache_handle;
        assert(dircache_handle);

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
            struct directory_entry *dir_entry =
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

                // This will drop the original dircachecnt on the inode.
                dircache_handle->remove(entry->cookie);
            }

            dir_entry = new directory_entry(strdup(entry->name),
                                            entry->cookie,
                                            entry->fileid);

            // Add to readdirectory_cache for future use.
            dircache_handle->add(dir_entry);

            /*
             * Add it to the directory_entry vector but ONLY upto the byte
             * limit requested by fuse readdir call.
             */
            if (rem_size >= 0) {
                rem_size -= dir_entry->get_fuse_buf_size(false /* readdirplus */);
                if (rem_size >= 0) {
                    readdirentries.push_back(dir_entry);
                }
            }

            entry = entry->nextentry;
            ++num_dirents;
        }

        AZLogDebug("readdir_callback: Num of entries returned by server is {}, "
                   "returned to fuse: {}, eof: {}, eof_cookie: {}",
                   num_dirents, readdirentries.size(),
                   eof, eof_cookie);

        dircache_handle->set_cookieverf(&res->READDIR3res_u.resok.cookieverf);

        if (eof) {
            /*
             * If we pass the last cookie or beyond it, then server won't
             * return any directory entries, but it'll set eof to true.
             * In such case, we must already have set eof and eof_cookie.
             */
            if (eof_cookie != -1) {
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
                    AZLogWarn("[{}] readdir_callback: Directory shrank in the "
                              "server! cookie asked = {}. Purging cache!",
                              dir_ino, task->rpc_api->readdir_task.get_offset());
                    dir_inode->invalidate_cache();
                } else {
                    assert((int64_t) dircache_handle->get_eof_cookie() != -1);
                }
            }
        }

        task->send_readdir_response(readdirentries);
    } else if (NFS_STATUS(res) == NFS3ERR_JUKEBOX) {
        task->get_client()->jukebox_retry(task);
    } else {
        task->reply_error(status);
    }
}

/*
 * Callback for the READDIR RPC. Once this callback is called, it will first
 * populate the readdir cache with the newly fetched entries (with the
 * attributes). Additionally it will populate the readdirentries vector and
 * call send_readdir_response() to respond to the fuse readdir call.
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
    const fuse_ino_t dir_ino = task->rpc_api->readdir_task.get_ino();
    struct nfs_inode *const dir_inode =
        task->get_client()->get_nfs_inode_from_ino(dir_ino);
    // How many max bytes worth of entries data does the caller want?
    ssize_t rem_size = task->rpc_api->readdir_task.get_size();
    std::vector<const directory_entry*> readdirentries;
    int num_dirents = 0;
    const int status = task->status(rpc_status, NFS_STATUS(res));

    /*
     * Now that the request has completed, we can query libnfs for the
     * dispatch time.
     */
    task->get_stats().on_rpc_complete(rpc_get_pdu(rpc), NFS_STATUS(res));

    if (status == 0) {
        const struct entryplus3 *entry =
            res->READDIRPLUS3res_u.resok.reply.entries;
        const bool eof = res->READDIRPLUS3res_u.resok.reply.eof;
        int64_t eof_cookie = -1;

        // Get handle to the readdirectory cache.
        std::shared_ptr<readdirectory_cache>& dircache_handle =
            dir_inode->dircache_handle;
        assert(dircache_handle != nullptr);

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
            const struct fattr3 *fattr = nullptr;
            const bool is_dot_or_dotdot =
                directory_entry::is_dot_or_dotdot(entry->name);

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
             * this directory_entry to fuse. Since fuse doesn't want us to
             * grab a ref for "." and "..", we drop ref for those later below.
             */
            struct nfs_inode *const nfs_inode =
                task->get_client()->get_nfs_inode(
                    &entry->name_handle.post_op_fh3_u.handle, fattr);

            if (!fattr) {
                /*
                 * If readdirplus entry doesn't carry attributes, then we
                 * just save the inode number and filetype as DT_UNKNOWN.
                 *
                 * Blob NFS though must always send attributes in a readdirplus
                 * response.
                 */
                assert(0);
                nfs_inode->attr.st_ino = entry->fileid;
                nfs_inode->attr.st_mode = 0;
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
            struct directory_entry *dir_entry =
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
                 * This will drop the original dircachecnt on the inode and
                 * also delete the inode if the lookupcnt ref is also 0.
                 */
                dircache_handle->remove(entry->cookie);
            }

            dir_entry = new struct directory_entry(strdup(entry->name),
                                                   entry->cookie,
                                                   nfs_inode->attr,
                                                   nfs_inode);

            /*
             * dir_entry must have one ref on the inode.
             * This ref will protect the inode while this directory_entry is
             * present in the readdirectory_cache (added below).
             */
            assert(nfs_inode->dircachecnt >= 1);

            // Add to readdirectory_cache for future use.
            dircache_handle->add(dir_entry);

            /*
             * Add it to the directory_entry vector but ONLY upto the byte
             * limit requested by fuse readdirplus call.
             */
            if (rem_size >= 0) {
                rem_size -= dir_entry->get_fuse_buf_size(true /* readdirplus */);
                if (rem_size >= 0) {
                    /*
                     * send_readdir_response() will drop this ref after adding
                     * to fuse buf, after which the lookupcnt ref will protect
                     * it.
                     */
                    nfs_inode->dircachecnt++;
                    readdirentries.push_back(dir_entry);

                    /*
                     * Fuse promises to call FORGET for each readdirplus
                     * returned entry that is not "." or "..". Note that
                     * get_nfs_inode() above would have grabbed a lookupcnt
                     * ref for all entries, here we drop ref on "." and "..",
                     * so we are only left with ref on non "." and "..".
                     */
                    if (is_dot_or_dotdot) {
                        nfs_inode->decref();
                    }
                } else {
                    /*
                     * We are unable to add this entry to the fuse response
                     * buffer, so we won't notify fuse of this entry.
                     * Drop the ref held by get_nfs_inode().
                     */
                    AZLogDebug("[{}] {}: Dropping ref since couldn't fit in "
                               "fuse response buffer",
                               nfs_inode->get_fuse_ino(),
                               entry->name);
                    nfs_inode->decref();
                }
            } else {
                AZLogDebug("[{}] {}: Dropping ref since couldn't fit in "
                           "fuse response buffer",
                           nfs_inode->get_fuse_ino(),
                           entry->name);
                nfs_inode->decref();
            }

            entry = entry->nextentry;
            ++num_dirents;
        }

        AZLogDebug("readdirplus_callback: Num of entries returned by server "
                   "is {}, returned to fuse: {}, eof: {}, eof_cookie: {}",
                   num_dirents, readdirentries.size(),
                   eof, eof_cookie);

        dircache_handle->set_cookieverf(&res->READDIRPLUS3res_u.resok.cookieverf);

        if (eof) {
            /*
             * If we pass the last cookie or beyond it, then server won't
             * return any directory entries, but it'll set eof to true.
             * In such case, we must already have set eof and eof_cookie.
             */
            if (eof_cookie != -1) {
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
                    AZLogWarn("[{}] readdirplus_callback: Directory shrank in "
                              "the server! cookie asked = {}. Purging cache!",
                              dir_ino, task->rpc_api->readdir_task.get_offset());
                    dir_inode->invalidate_cache();
                } else {
                    assert((int64_t) dircache_handle->get_eof_cookie() != -1);
                }
            }
        }

        task->send_readdir_response(readdirentries);
    } else if (NFS_STATUS(res) == NFS3ERR_JUKEBOX) {
        task->get_client()->jukebox_retry(task);
    } else {
        task->reply_error(status);
    }
}

void rpc_task::get_readdir_entries_from_cache()
{
    const bool readdirplus = (get_op_type() == FUSE_READDIRPLUS);
    struct nfs_inode *nfs_inode =
        get_client()->get_nfs_inode_from_ino(rpc_api->readdir_task.get_ino());
    // Must have been allocated by opendir().
    assert(nfs_inode->dircache_handle);
    bool is_eof = false;
    std::vector<const directory_entry*> readdirentries;

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
        // We are done fetching the entries, send the response now.
        send_readdir_response(readdirentries);
    }
}

void rpc_task::fetch_readdir_entries_from_server()
{
    bool rpc_retry;
    const fuse_ino_t dir_ino = rpc_api->readdir_task.get_ino();
    struct nfs_inode *dir_inode = get_client()->get_nfs_inode_from_ino(dir_ino);
    assert(dir_inode->dircache_handle);
    const cookie3 cookie = rpc_api->readdir_task.get_offset();
    rpc_pdu *pdu = nullptr;

    do {
        READDIR3args args;

        args.dir = dir_inode->get_fh();
        args.cookie = cookie;
        ::memcpy(&args.cookieverf,
                 dir_inode->dircache_handle->get_cookieverf(),
                 sizeof(args.cookieverf));

        args.count = nfs_get_readdir_maxcount(get_nfs_context());

        rpc_retry = false;
        stats.on_rpc_issue();
        if ((pdu = rpc_nfs3_readdir_task(get_rpc_ctx(),
                                  readdir_callback,
                                  &args,
                                  this)) == NULL) {
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
    assert(dir_inode->dircache_handle);
    const cookie3 cookie = rpc_api->readdir_task.get_offset();
    rpc_pdu *pdu = nullptr;

    do {
        READDIRPLUS3args args;

        args.dir = dir_inode->get_fh();
        args.cookie = cookie;
        ::memcpy(&args.cookieverf,
                 dir_inode->dircache_handle->get_cookieverf(),
                 sizeof(args.cookieverf));

        /*
         * Use dircount/maxcount according to the user configured and
         * the server advertised value.
         */
        args.maxcount = nfs_get_readdir_maxcount(get_nfs_context());
        args.dircount = args.maxcount;

        rpc_retry = false;
        stats.on_rpc_issue();
        if ((pdu = rpc_nfs3_readdirplus_task(get_rpc_ctx(),
                                      readdirplus_callback,
                                      &args,
                                      this)) == NULL) {
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

void rpc_task::send_readdir_response(
    const std::vector<const directory_entry*>& readdirentries)
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
    struct nfs_inode *parent_inode =
        get_client()->get_nfs_inode_from_ino(parent_ino);

    // Fuse always requests 4096 bytes.
    assert(size >= 4096);

    // Allocate fuse response buffer.
    char *buf1 = (char *) malloc(size);
    if (!buf1) {
        reply_error(ENOMEM);
        return;
    }

    char *current_buf = buf1;
    size_t rem = size;
    int num_entries_added = 0;

    AZLogDebug("send_readdir_response: Number of directory entries to send {}",
               readdirentries.size());

    for (const auto& it : readdirentries) {
        /*
         * Caller should make sure that it adds only directory entries after
         * what was requested in the READDIR{PLUS} call to readdirentries.
         */
        assert((uint64_t) it->cookie > (uint64_t) rpc_api->readdir_task.get_offset());
        size_t entsize;

        /*
         * Drop the ref held inside lookup() or readdirplus_callback().
         *
         * Note: entry->nfs_inode may be null for entries populated using
         *       only readdir however, it is guaranteed to be present for
         *       readdirplus.
         */
        if (it->nfs_inode) {
            assert(it->nfs_inode->dircachecnt > 0);
            it->nfs_inode->dircachecnt--;
        }

        if (readdirplus) {
            struct fuse_entry_param fuseentry;

            /*
             * We are going to return this inode to fuse.
             * Set forget_seen (in case this is not a fresh inode but being
             * recycled from inode_map) and clear returned_to_fuse.
             */
            it->nfs_inode->forget_seen = false;
            it->nfs_inode->returned_to_fuse = true;

            // We don't need the memset as we are setting all members.
            //memset(&fuseentry, 0, sizeof(fuseentry));
            fuseentry.attr = it->attributes;
            fuseentry.ino = it->nfs_inode->get_fuse_ino();
            fuseentry.generation = it->nfs_inode->get_generation();
            fuseentry.attr_timeout = it->nfs_inode->get_actimeo();
            fuseentry.entry_timeout = it->nfs_inode->get_actimeo();

            AZLogDebug("[{}] <{}> Returning ino {} to fuse (filename {})",
                       parent_ino,
                       rpc_task::fuse_opcode_to_string(rpc_api->optype),
                       fuseentry.ino,
                       it->name);

            /*
             * Readdirplus returns inode for every file, so it's the
             * equivalent of lookup (and fuse may skip lookup if this file
             * is opened), so save in dnlc.
             */
            parent_inode->dnlc_add(it->name, fuseentry.ino);

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

        // Increment the buffer pointer to point to the next free space.
        current_buf += entsize;
        rem -= entsize;
        num_entries_added++;

        if (readdirplus) {
            /*
             * Fuse expects lookupcnt of every entry returned by readdirplus(),
             * except "." and "..", to be incremented. Make sure get_nfs_inode()
             * has duly taken the refs.
             *
             * If fuse_reply_buf() below fails we drop these refcnts below.
             */
            if (!it->is_dot_or_dotdot()) {
                assert(it->nfs_inode->lookupcnt > 0);
            }
        }
    }

    AZLogDebug("Num of entries sent in readdir response is {}", num_entries_added);

    if (fuse_reply_buf(get_fuse_req(), buf1, size - rem) != 0) {
        AZLogError("fuse_reply_buf failed!");

        if (readdirplus) {
            for (const auto& it : readdirentries) {
                AZLogDebug("[{}] Dropping lookupcnt, now {}",
                           it->nfs_inode->get_fuse_ino(),
                           it->nfs_inode->lookupcnt.load());
#ifdef ENABLE_PARANOID
                it->nfs_inode->returned_to_fuse = false;
#endif
                it->nfs_inode->decref();
            }
        }
    }

    free(buf1);
    free_rpc_task();
}
