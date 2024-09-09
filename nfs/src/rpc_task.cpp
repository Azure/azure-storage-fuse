#include "nfs_internal.h"
#include "rpc_task.h"
#include "nfs_client.h"

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

void rpc_task::init_create_file(fuse_req *request,
                                fuse_ino_t parent_ino,
                                const char *name,
                                mode_t mode,
                                struct fuse_file_info *file)
{
    assert(get_op_type() == FUSE_CREATE);
    set_fuse_req(request);
    rpc_api->create_task.set_parent_ino(parent_ino);
    rpc_api->create_task.set_file_name(name);
    rpc_api->create_task.set_mode(mode);
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
    rpc_api->mknod_task.set_parent_ino(parent_ino);
    rpc_api->mknod_task.set_file_name(name);
    rpc_api->mknod_task.set_mode(mode);

    fh_hash = get_client()->get_nfs_inode_from_ino(parent_ino)->get_crc();
}

void rpc_task::init_mkdir(fuse_req *request,
                          fuse_ino_t parent_ino,
                          const char *name,
                          mode_t mode)
{
    assert(get_op_type() == FUSE_MKDIR);
    set_fuse_req(request);
    rpc_api->mkdir_task.set_parent_ino(parent_ino);
    rpc_api->mkdir_task.set_dir_name(name);
    rpc_api->mkdir_task.set_mode(mode);

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
    rpc_api->symlink_task.set_link(link);
    rpc_api->symlink_task.set_parent_ino(parent_ino);
    rpc_api->symlink_task.set_name(name);

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
 * Called when libnfs completes a WRITE RPC.
 */
static void write_callback(
    struct rpc_context *rpc,
    int rpc_status,
    void *data,
    void *private_data)
{
    struct write_context *ctx = (write_context*) private_data;
    struct rpc_task *task = ctx->get_task();
    assert(task->magic == RPC_TASK_MAGIC);

    struct bytes_chunk *bc = &ctx->get_bytes_chunk();
    // rpc_api->bc points to the bc inside write_context.
    assert(task->rpc_api->bc == bc);
    struct membuf *mb = bc->get_membuf();
    struct nfs_client *client = task->get_client();
    assert(client->magic == NFS_CLIENT_MAGIC);

    struct nfs_inode *inode =
            client->get_nfs_inode_from_ino(ctx->get_ino());

    /*
     * Sanity check.
     */
    assert(rpc != nullptr);
    assert(task->get_op_type() == FUSE_WRITE ||
           task->get_op_type() == FUSE_FLUSH ||
           task->get_op_type() == FUSE_RELEASE);
    assert(mb != nullptr);
    assert(mb->is_inuse() && mb->is_locked());
    assert(mb->is_flushing() && mb->is_dirty() && mb->is_uptodate());

    // If we have already written everything, why are we here.
    assert(bc->pvt < bc->length);

    /*
     * write_callback() must only be called after a write done from fuse,
     * which means file cache must have been allocated.
     */
    assert(inode->filecache_handle);

#if 0
    /*
     * We should only ever write full membufs.
     * Note: bc may be referring to a smaller window but we write the entire
     *       membuf. Leaving this assert commented to highlight that.
     */
    assert(bc->maps_full_membuf());
#endif

    auto res = (WRITE3res *)data;
    const char* errstr;
    const int status = task->status(rpc_status, NFS_STATUS(res), &errstr);

    /*
     * Now that the request has completed, we can query libnfs for the
     * dispatch time.
     */
    task->get_stats().on_rpc_complete(rpc_get_pdu(rpc), NFS_STATUS(res));

    // Success case.
    if (status == 0) {
        // Successful Blob write must not return 0.
        assert(res->WRITE3res_u.resok.count > 0);

        /*
         * How much has been written till now (including this WRITE).
         */
        bc->pvt += res->WRITE3res_u.resok.count;
        assert(bc->pvt <= mb->length);

        /*
         * Have we not yet written what we set out to write?
         * Note that we sync the entire membuf and not just bc->length.
         */
        const bool is_partial_write = (bc->pvt < mb->length);

        if (is_partial_write) {
            /*
             * Written less than intended, unlikely but possible.
             * Arrange to write rest of the data.
             */
            WRITE3args args;
            struct rpc_pdu *pdu;
            fuse_ino_t ino = ctx->get_ino();

            args.file = client->get_nfs_inode_from_ino(ino)->get_fh();
            args.offset = mb->offset + bc->pvt;
            args.count  = mb->length - bc->pvt;
            args.stable = FILE_SYNC;
            args.data.data_len = mb->length - bc->pvt;
            args.data.data_val = (char *) mb->buffer + bc->pvt;

            /*
             * Create new flush task to write remaining buffer.
             * New task help us to account stats accurately.
             * This flush task will be continuing the job of writing to the
             * same bc, so set bc and ctx.
             */
            struct rpc_task *flush_task =
                client->get_rpc_task_helper()->alloc_rpc_task(FUSE_FLUSH);
            flush_task->init_flush(nullptr /* fuse_req */, ino);
            flush_task->rpc_api->bc = bc;
            flush_task->rpc_api->pvt = (void *) ctx;

            // Update the ctx with new task.
            ctx->set_task(flush_task);

            // TODO: Make this AZLogDebug after some time.
            AZLogInfo("[{}] Partial write: [{}, {}) of [{}, {})", ctx->get_ino(),
                       mb->offset + bc->pvt, mb->offset + mb->length,
                       mb->offset, mb->offset + mb->length);

            bool rpc_retry;
            do {
                rpc_retry = false;
                flush_task->get_stats().on_rpc_issue();
                if ((pdu = rpc_nfs3_write_task(flush_task->get_rpc_ctx(), write_callback,
                                                &args, ctx)) == NULL) {
                    flush_task->get_stats().on_rpc_cancel();
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

            /*
             * Release this task since it has done it's job.
             * Now next the callback will be called when the above partial
             * write completes.
             */
            task->free_rpc_task();

            return;
        } else {
            // Complete data writen to blob.
            AZLogDebug("[{}] Completed write, off: {}, len: {}",
                       ctx->get_ino(), mb->offset, mb->length);

            mb->clear_dirty();
            mb->clear_flushing();
        }
    } else if (NFS_STATUS(res) == NFS3ERR_JUKEBOX) {
        AZLogDebug("[{}] JUKEBOX error write, off: {}, len: {}",
                   ctx->get_ino(),
                   mb->offset + bc->pvt,
                   mb->length - bc->pvt);
        task->get_client()->jukebox_retry(task);
        return;
    } else {
        /*
         * Since the api failed and can no longer be retried, set write_error
         * and do not clear dirty flag.
         */
        AZLogError("[{}] Write [{}, {}) failed with status {}: {}",
                   ctx->get_ino(),
                   mb->offset + bc->pvt, mb->offset + mb->length,
                   status, errstr);

        assert(mb->is_dirty());
        mb->clear_flushing();
        inode->set_write_error(status);
    }

    mb->clear_locked();
    mb->clear_inuse();

    // We don't want to cache the data after the write completes.
    inode->filecache_handle->release(bc->offset, bc->length);

    // Release the task.
    task->free_rpc_task();

    delete ctx;
}

static void createfile_callback(
    struct rpc_context *rpc,
    int rpc_status,
    void *data,
    void *private_data)
{
    rpc_task *task = (rpc_task*) private_data;
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

/*
 * Copy application data to chunk cache and return the updated buffers'
 * vector. Caller will then have to write these chunks to the Blob.
 */
static std::vector<bytes_chunk>
copy_to_cache(struct rpc_task *task,
              fuse_ino_t ino,
              struct fuse_bufvec* bufv,
              off_t offset,
              size_t length,
              int& error)
{
    // Check the valid ino.
    assert(ino != 0);

    /*
     * XXX We are only handling count=1, assert to know if kernel sends more,
     *     we would want to handle that.
     */
    assert(bufv->count == 1);

    const char *buf = (char *) bufv->buf[bufv->idx].mem + bufv->off;
    struct nfs_inode *inode = task->get_client()->get_nfs_inode_from_ino(ino);
    assert(inode->filecache_handle);
    std::vector<bytes_chunk> bc_vec =
        inode->filecache_handle->get(offset, length);

    // Result of pread/read.
    [[maybe_unused]] size_t remaining = length;

    for (auto &bc : bc_vec) {
        struct membuf *mb = bc.get_membuf();

        // Lock the membuf to do the operation.
        mb->set_locked();

        assert(remaining >= bc.length);

        /*
         * If we own the full membuf we can safely copy to it, also if the
         * membuf is uptodate we can safely copy to it. In both cases the
         * membuf remains uptodate after the copy.
         */
        if (bc.is_empty || mb->is_uptodate()) {
            ::memcpy(bc.get_buffer(), buf, bc.length);
            mb->set_dirty();
            mb->set_uptodate();
        } else {
            /*
             * bc refers to part of the membuf and membuf is not uptodate.
             * In this case we need to read back the entire membuf and then
             * update the part that user wants to write to.
             *
             * TODO: Need to issue read.
             */
            assert(0);
        }

        mb->clear_locked();
        buf += bc.length;
        remaining -= bc.length;

        assert((int) remaining >= 0);
    }

    assert(remaining == 0);

    return bc_vec;
}



/**
 * Synchronize (aka flush) a membuf to backing Blob.
 * It does the following:
 * 1. Creates new rpc_task for the flush operation.
 *    This helps account for these flush operations and prevents them from
 *    growing uncontrolled.
 * 2. Flush the membuf to backend asynchronously, while membuf is locked.
 * 3. On completion write_callback() is called.
 * 4. write_callback() clears the dirty bit, releases membuf lock and releases
 *    this chunk from chunkmap.
 * 5. Frees the flush rpc_task.
 *
 * Note:- Write call doesn't need to wait for completion of this async task.
 *        After starting async task, we can complete the write request, as
 *        write buffer is already copied to membuf while the sync happens in
 *        the background. Note that this is for buffered IO where we copy the
 *        the application data into the chunk cache.
 *
 * Note:- Flush can wait for this calls to finish by waiting on membuf lock.
 *
 * TODO: Need to throttle writes beyond the max RPC tasks limit.
 *       Ideally we should put a cap on how many we keep outstanding with a
 *       single file.
 */
void rpc_task::sync_membuf(struct bytes_chunk& bc,
                           fuse_ino_t ino)
{
    /*
     * Get the underlying membuf for bc.
     * Note that we write the entire membuf, even though bc may be referring
     * to a smaller window.
     */
    struct membuf *mb = bc.get_membuf();

    /*
     * For jukebox retries rpc_api->pvt must be pointing at the write_context.
     * For others we allocate and set write_context here.
     */
    const bool is_jukebox_retry = (rpc_api->pvt != nullptr);

    // Verify the mb.
    assert(mb != nullptr);
    assert(mb->is_inuse());

    /*
     * Lock the membuf. If multiple writer threads want to flush the same
     * membuf the first one will find it dirty and not flushing, that thread
     * should initiate the Blob write. Others that come in while the 1st thread
     * started flushing but the write has not completed, will find it "dirty
     * and flushing" and they can avoid the write and optionally choose to wait
     * for it to complete by waiting for the lock. Others who find it after the
     * write is done and lock is released will find it not "dirty and not
     * flushing". They can just skip.
     * Note that we allocate the rpc_task for flush before the lock as it may
     * block.
     *
     * Note: Jukebox retries will have mb->is_flushing set (when the original
     *       task was issued), but we still want to issue the retry task.
     */
    if (!mb->is_dirty() || (mb->is_flushing() && !is_jukebox_retry)) {
        free_rpc_task();
        return;
    }

    /*
     * For jukebox retries lock would already be held, else hold it now.
     */
    if (!is_jukebox_retry) {
        mb->set_locked();
    } else {
        assert(mb->is_locked());
    }

    if ((mb->is_dirty() && !mb->is_flushing()) || is_jukebox_retry) {
        WRITE3args args;
        struct rpc_pdu *pdu;
        struct write_context *ctx;

        // Never ever write non-uptodate membufs.
        assert(mb->is_uptodate());
        assert(mb->is_dirty());

        // Fill the WRITE RPC arguments.
        args.file = client->get_nfs_inode_from_ino(ino)->get_fh();
        args.offset = mb->offset + bc.pvt;
        args.count  = mb->length - bc.pvt;
        args.stable = FILE_SYNC;
        args.data.data_len = mb->length - bc.pvt;
        args.data.data_val = (char *) mb->buffer + bc.pvt;

        if (rpc_api->pvt == nullptr) {
            assert(bc.pvt == 0);
            ctx = new write_context(bc, this, ino);
            rpc_api->pvt = (void *) ctx;
        } else {
            /*
             * For jukebox retries rpc_api->pvt must already be pointing to
             * the write_context.
             */
            ctx = (write_context *) rpc_api->pvt;
            assert(&bc == &ctx->get_bytes_chunk());
            ctx->set_task(this);
        }

        rpc_api->bc = &(ctx->get_bytes_chunk());

        // Set flag to flushing.
        mb->set_flushing();

        bool rpc_retry;
        do {
            rpc_retry = false;
            stats.on_rpc_issue();
            if ((pdu = rpc_nfs3_write_task(get_rpc_ctx(),
                                    write_callback, &args,
                                    ctx)) == NULL) {
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
    } else {
        mb->clear_locked();
        free_rpc_task();
    }
}

void rpc_task::run_write()
{
    fuse_ino_t ino = rpc_api->write_task.get_ino();
    struct nfs_inode *inode = get_client()->get_nfs_inode_from_ino(ino);
    size_t length = rpc_api->write_task.get_size();
    struct fuse_bufvec *bufv = rpc_api->write_task.get_buffer_vector();
    off_t offset = rpc_api->write_task.get_offset();

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
    std::vector<bytes_chunk> bc_vec =
            copy_to_cache(this, ino, bufv, offset, length, error_code);
    if (error_code != 0) {
        AZLogWarn("[{}] copy_to_cache failed with error={}, "
                  "failing write!", ino, error_code);
        reply_error(error_code);
        return;
    }

    assert(bc_vec.size() > 0);

    for (bytes_chunk& bc : bc_vec) {
        /*
         * Create the flush task to carry out the write.
         */
        struct rpc_task *flush_task =
            get_client()->get_rpc_task_helper()->alloc_rpc_task(FUSE_FLUSH);
        flush_task->init_flush(nullptr /* fuse_req */, rpc_api->flush_task.get_ino());

        // sync_membuf() uses it to identify jukebox retries, so assert.
        assert(flush_task->rpc_api->pvt == nullptr);
        flush_task->sync_membuf(bc, ino);
    }

    // Send reply to original request without waiting for the backend write to complete.
    reply_write(length);
}

void rpc_task::run_flush()
{
    fuse_ino_t ino = rpc_api->write_task.get_ino();
    struct nfs_inode *inode = get_client()->get_nfs_inode_from_ino(ino);

    /*
     * Check if any write error set, if set don't attempt the flush and fail
     * the flush operation.
     */
    int error_code = inode->get_write_error();
    if (error_code != 0) {
        AZLogWarn("[{}] Previous write to this Blob failed with error={}, "
                  "skipping new flush!", ino, error_code);

        reply_error(error_code);
        return;
    }

    /*
     * Get the filecache_handle from the inode.
     * If flush() is called w/o open(), there won't be any cache, skip.
     */
    auto filecache_handle = inode->filecache_handle;
    if (!filecache_handle) {
        reply_error(0);
        return;
    }

    // Get the dirty bytes_chunk from the filecache handle.
    std::vector<bytes_chunk> bc_vec = filecache_handle->get_dirty_bc();

    // Flush dirty membufs to backend.
    for (bytes_chunk& bc : bc_vec) {
        /*
         * Create the flush task to carry out the write.
         */
        struct rpc_task *flush_task =
            get_client()->get_rpc_task_helper()->alloc_rpc_task(FUSE_FLUSH);
        flush_task->init_flush(nullptr /* fuse_req */, rpc_api->flush_task.get_ino());

        // sync_membuf() uses it to identify jukebox retries, so assert.
        assert(flush_task->rpc_api->pvt == nullptr);

        // Flush the membuf to backend.
        flush_task->sync_membuf(bc, ino);
    }

    /*
     * Our caller expects us to return only after the flush completes.
     * Wait for all the membufs to flush and get result back.
     */
    for (bytes_chunk &bc : bc_vec) {
        struct membuf *mb = bc.get_membuf();
        assert(mb != nullptr);

        mb->set_locked();
        assert(mb->is_inuse());

        /*
         * If still dirty after we get the lock, it may mean two things:
         * - Write failed.
         * - Some other thread got the lock before us and it made the
         *   membuf dirty again.
         */
        if (mb->is_dirty() && inode->get_write_error()) {
            AZLogError("[{}] Flush [{}, {}) failed with error: {}",
                       ino,
                       bc.offset, bc.offset + bc.length,
                       inode->get_write_error());
        }

        mb->clear_locked();
        mb->clear_inuse();
    }

    reply_error(inode->get_write_error());
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
        args.how.createhow3_u.obj_attributes.mode.set_mode3_u.mode = rpc_api->create_task.get_mode();

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

        ::memset(&args, 0, sizeof(args));
        args.where.dir = get_client()->get_nfs_inode_from_ino(parent_ino)->get_fh();
        args.where.name = (char*)rpc_api->mkdir_task.get_dir_name();
        args.attributes.mode.set_it = 1;
        args.attributes.mode.set_mode3_u.mode = rpc_api->mkdir_task.get_mode();

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

                AZLogDebug("Data read from cache. size: {}, offset: {}",
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
            
            /*
             * This will drop the original dircachecnt on the inode and
             * also delete the inode if the lookupcnt ref is also 0.
             */
            dircache_handle->remove(entry->cookie);
            struct directory_entry* dir_entry = new struct directory_entry(strdup(entry->name),
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
         */
        if(it->nfs_inode) {
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
