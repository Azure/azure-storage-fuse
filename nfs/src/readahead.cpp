#include "aznfsc.h"
#include "readahead.h"
#include "rpc_task.h"
#include "file_cache.h"

/*
 * This enables debug logs and also runs the self tests.
 * Must enable once after adding a new self-test or making any changes to
 * the class.
 */
//#define DEBUG_READAHEAD

#define _MiB (1024 * 1024LL)
#define _GiB (_MiB * 1024)
#define _TiB (_GiB * 1024)

#define NFS_STATUS(r) ((r) ? (r)->status : NFS3ERR_SERVERFAULT)

namespace aznfsc {

ra_state::ra_state(struct nfs_client *_client,
                   struct nfs_inode *_inode) :
        client(_client),
        inode(_inode),
        ra_bytes(client->mnt_options.readahead_kb * 1024),
        def_ra_size(std::min<uint64_t>(client->mnt_options.rsize_adj, ra_bytes))
{
    assert(client->magic == NFS_CLIENT_MAGIC);
    assert(inode->magic == NFS_INODE_MAGIC);

    // We should be called only for regular files.
    assert(inode->is_regfile());

    // Readahead needs filecache.
    assert(inode->filecache_handle != nullptr);

    /*
     * By the time ra_state is initialized mount must have already
     * completed and we must have the rsize/wsize value advertized
     * by the server.
     */
    assert(client->mnt_options.rsize_adj >= AZNFSCFG_RSIZE_MIN &&
           client->mnt_options.rsize_adj <= AZNFSCFG_RSIZE_MAX);

    assert((client->mnt_options.readahead_kb >= AZNFSCFG_READAHEAD_KB_MIN &&
            client->mnt_options.readahead_kb <= AZNFSCFG_READAHEAD_KB_MAX) ||
           (client->mnt_options.readahead_kb == 0));

    AZLogInfo("[{}] Readahead set to {} bytes with default RA size {} bytes",
              inode->get_fuse_ino(), ra_bytes, def_ra_size);
}

/**
 * Readahead context.
 * All ongoing readahead reads are tracked using one ra_context object.
 */
struct ra_context
{
    /*
     * bytes_chunk which this readahead is reading from the file.
     */
    struct bytes_chunk bc;

    /*
     * rpc_task tracking this readahead.
     *
     * Note: We don't strictly need an rpc_task to track readahead reads
     *       since we don't need to send a fuse reply, but we still use
     *       one rpc_task per readahead read so that readahead reads are
     *       also limited by the number of concurrent rpc_tasks allowed.
     */
    struct rpc_task *task;

    ra_context(rpc_task *_task, struct bytes_chunk& _bc) :
        bc(_bc),
        task(_task)
    {
        assert(task->magic == RPC_TASK_MAGIC);
        assert(bc.length > 0 && bc.length <= AZNFSC_MAX_CHUNK_SIZE);
        assert(bc.offset < AZNFSC_MAX_FILE_SIZE);
    }
};

static void readahead_callback (
    struct rpc_context *rpc,
    int rpc_status,
    void *data,
    void *private_data)
{
    struct ra_context *ctx = (struct ra_context*) private_data;
    struct rpc_task *task = ctx->task;
    struct bytes_chunk *bc = &ctx->bc;
    auto res = (READ3res*) data;
    const char *errstr = nullptr;

    assert(task->magic == RPC_TASK_MAGIC);
    assert(bc->length > 0);
    assert(task->rpc_api->read_task.get_offset() >= (off_t) bc->offset);
    assert(task->rpc_api->read_task.get_size() <= bc->length);

    // Cannot have read more than requested.
    assert(res->READ3res_u.resok.count <= bc->length);

    /*
     * This callback would be called for some backend call that we must have
     * issued.
     */
    assert(bc->num_backend_calls_issued >= 1);

    /*
     * If we have already finished reading the entire bytes_chunk, why are we
     * here.
     */
    assert(bc->pvt < bc->length);

    const int status = task->status(rpc_status, NFS_STATUS(res), &errstr);
    const fuse_ino_t ino = task->rpc_api->read_task.get_ino();
    struct nfs_inode *inode = task->get_client()->get_nfs_inode_from_ino(ino);
    const auto read_cache = inode->filecache_handle;

    assert(read_cache != nullptr);
    assert(ino == inode->get_fuse_ino());

    /*
     * Now that the request has completed, we can query libnfs for the
     * dispatch time.
     */
    task->get_stats().on_rpc_complete(rpc_get_pdu(rpc), NFS_STATUS(res));

    /*
     * Offset and length for the actual read request for which this callback
     * is called. Note that the entire read may not be satisfied, it may be
     * a partial read response.
     */
    const uint64_t issued_offset = bc->offset + bc->pvt;
    const uint64_t issued_length = bc->length - bc->pvt;

    if (status != 0) {
        /*
         * Readahead read failed? Nothing to do, unlock the membuf, release
         * the byte range and pretend as if we never issued this read.
         * We may have successfully read some part of it, as some prior read
         * calls may have completed partially, but we cannot mark the membuf
         * uptodate unless we read it fully, so we have to just drop it.
         * Note that those prio successful reads would have caused the RPC
         * stats to be updated, but that's fine.
         */

        bc->get_membuf()->clear_locked();
        bc->get_membuf()->clear_inuse();

        // Release the buffer since we did not fill it.
        read_cache->release(bc->offset, bc->length);

        AZLogWarn("[{}] readahead_callback [FAILED] for offset: {} size: {} "
                  "total bytes read till now: {} of {} for [{}, {}) "
                  "num_backend_calls_issued: {}, rpc_status: {}, nfs_status: {}, "
                  "error: {}",
                  ino,
                  issued_offset,
                  issued_length,
                  bc->pvt,
                  bc->length,
                  bc->offset,
                  bc->offset + bc->length,
                  bc->num_backend_calls_issued,
                  rpc_status,
                  (int) NFS_STATUS(res),
                  errstr);

        goto delete_ctx;
    } else {
        UPDATE_INODE_ATTR(inode, res->READ3res_u.resok.file_attributes);

        /*
         * Only first read call would have bc->pvt == 0, for subsequent calls
         * we will have num_backend_calls_issued > 1.
         */
        assert((bc->pvt == 0) || (bc->num_backend_calls_issued > 1));

        // We should never get more data than what we requested.
        assert(res->READ3res_u.resok.count <= issued_length);

        const bool is_partial_read = !res->READ3res_u.resok.eof &&
            (res->READ3res_u.resok.count < issued_length);

        // Update bc->pvt with fresh bytes read in this call.
        bc->pvt += res->READ3res_u.resok.count;
        assert(bc->pvt <= bc->length);

        INC_GBL_STATS(bytes_read_ahead, res->READ3res_u.resok.count);

        AZLogDebug("[{}] readahead_callback: {}Read completed for offset: {} "
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
            assert(bc->pvt < bc->length);

            const off_t new_offset = bc->offset + bc->pvt;
            const size_t new_size = bc->length - bc->pvt;
            bool rpc_retry = false;

            READ3args new_args;
            new_args.file = inode->get_fh();
            new_args.offset = new_offset;
            new_args.count = new_size;

            // Create a new child task to carry out this request.
            struct rpc_task *partial_read_tsk =
                task->get_client()->get_rpc_task_helper()->alloc_rpc_task(FUSE_READ);

            partial_read_tsk->init_read(
                task->rpc_api->req,
                task->rpc_api->read_task.get_ino(),
                new_size,
                new_offset,
                task->rpc_api->read_task.get_fuse_file());

            ctx->task = partial_read_tsk;

            bc->num_backend_calls_issued++;
            assert(bc->num_backend_calls_issued > 1);

            AZLogDebug("[{}] Issuing partial read at offset: {} size: {}"
                       " for [{}, {})",
                       ino,
                       new_offset,
                       new_size,
                       bc->offset,
                       bc->offset + bc->length);

            rpc_pdu *pdu = nullptr;

            do {
                rpc_retry = false;
                /*
                 * We have identified partial read case where the
                 * server has returned fewer bytes than requested.
                 * Hence we will issue read for the remaining.
                 *
                 * Note: It is okay to issue a read call directly here
                 *       as we are holding all the needed locks and refs.
                 */
                partial_read_tsk->get_stats().on_rpc_issue();
                if ((pdu = rpc_nfs3_read_task(
                        partial_read_tsk->get_rpc_ctx(),
                        readahead_callback,
                        bc->get_buffer() + bc->pvt,
                        new_size,
                        &new_args,
                        (void *) ctx)) == NULL) {
                    partial_read_tsk->get_stats().on_rpc_cancel();
                    /*
                     * This call fails due to internal issues like OOM
                     * etc and not due to an actual error, hence retry.
                     */
                    AZLogWarn("rpc_nfs3_read_task failed to issue, retrying "
                              "after 5 secs!");
                    ::sleep(5);

                    rpc_retry = true;
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
    }

    /*
     * We come here only after the complete readahead read has successfully
     * completed.
     * We should never return lesser bytes than requested, unless eof is
     * encountered.
     */
    assert(status == 0);
    assert((bc->length == bc->pvt) || res->READ3res_u.resok.eof);

    if (bc->is_empty && (bc->length == bc->pvt)) {
        /*
         * Only the first read which got hold of the complete membuf will have
         * this byte_chunk set to empty. Only such reads should set the uptodate
         * flag.
         */
        AZLogDebug("[{}] Setting uptodate flag for membuf [{}, {})",
                   ino, bc->offset, bc->offset + bc->length);

        assert(bc->maps_full_membuf());
        bc->get_membuf()->set_uptodate();
    } else {
        bool set_uptodate = false;
        /*
         * If we got eof in a partial read, release the non-existent
         * portion of the chunk.
         */
        if (bc->is_empty && (bc->length > bc->pvt) &&
            res->READ3res_u.resok.eof) {
            assert(res->READ3res_u.resok.count < issued_length);

            const uint64_t released_bytes =
                read_cache->release(bc->offset + bc->pvt,
                                    bc->length - bc->pvt);
                /*
                 * If we are able to successfully release all the extra bytes
                 * from the bytes_chunk, that means there's no other thread
                 * actively performing IOs to the underlying membuf, so we can
                 * mark it uptodate.
                 */
                if (released_bytes == (bc->length - bc->pvt)) {
                    AZLogWarn("[{}] Setting uptodate flag for membuf [{}, {}), "
                              "after readahead hit eof, requested [{}, {}), "
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
            AZLogDebug("[{}] Not updating uptodate flag for membuf [{}, {})",
                       ino, bc->offset, bc->offset + bc->length);
        }
    }

    /*
     * Release the lock that we held on the membuf since the data is now
     * written and ready to be read.
     */
    bc->get_membuf()->clear_locked();
    bc->get_membuf()->clear_inuse();

delete_ctx:
    // Success or failure, report readahead completion.
    inode->readahead_state->on_readahead_complete(bc->offset, bc->length);

    // Free the readahead RPC task.
    task->free_rpc_task();

    // Free the context.
    delete ctx;

    // Decrement the extra ref taken on inode at the time read was issued.
    inode->decref();
}

int64_t ra_state::get_next_ra(uint64_t length)
{
    if (length == 0) {
        length = def_ra_size;
    }

    /*
     * RA is disabled?
     */
    if (length == 0) {
        return -1;
    }

    /*
     * Don't perform readahead beyond eof.
     * If we don't have a file size estimate (probably the attr cache is too
     * old) then also we play safe and do not perform readahead.
     */
    const int64_t filesize =
        inode ? inode->get_file_size(): AZNFSC_MAX_FILE_SIZE;
    assert(filesize >= 0 || filesize == -1);
    if ((filesize == -1) ||
        ((int64_t) (last_byte_readahead + 1 + length) > filesize)) {
        return -2;
    }

    /*
     * Application read pattern is known to be non-sequential?
     */
    if (!is_sequential()) {
        return -3;
    }

    /*
     * If we already have ra_bytes readahead bytes read, don't readahead
     * more.
     */
    if ((last_byte_readahead + length) > (max_byte_read + ra_bytes)) {
        return -4;
    }

    /*
     * Keep readahead bytes issued always less than ra_bytes.
     */
    if ((ra_ongoing += length) > ra_bytes) {
        assert(ra_ongoing >= length);
        ra_ongoing -= length;
        return -5;
    }

    std::unique_lock<std::shared_mutex> _lock(lock);

    /*
     * Atomically update last_byte_readahead, as we don't want to return
     * duplicate readahead offset to multiple calls.
     */
    const uint64_t next_ra =
        std::atomic_exchange(&last_byte_readahead, last_byte_readahead + length) + 1;

    assert((int64_t) next_ra > 0);
    return next_ra;
}
/*
 * TODO: Add readahead stats.
 */
int ra_state::issue_readaheads()
{
    int64_t ra_offset;
    auto read_cache = inode->filecache_handle;
    int ra_issued = 0;
    static uint64_t num_no_readahead;

    /*
     * No cache, can't readahead.
     */
    if (!read_cache) {
        return 0;
    }

    /*
     * If userspace data cache is disabled, don't do readaheads.
     */
    if (!aznfsc_cfg.cache.data.user.enable) {
        return 0;
    }

    /*
     * Issue all readaheads allowed by this ra_state.
     */
    while ((ra_offset = get_next_ra()) > 0) {
        AZLogDebug("[{}] Issuing readahead at off: {} len: {}: ongoing: {} ({})",
                   inode->get_fuse_ino(), ra_offset, def_ra_size,
                   ra_ongoing.load(), ra_bytes);

        /*
         * Get bytes_chunk representing the byte range we want to readahead
         * and issue READ RPCs for all.
         */
        std::vector<bytes_chunk> bcv = read_cache->get(ra_offset, def_ra_size);

        for (bytes_chunk& bc : bcv) {

            // Every bytes_chunk must lie within the readahead.
            assert(bc.offset >= (uint64_t) ra_offset);
            assert((bc.offset + bc.length) <= (ra_offset + def_ra_size));

            // get() must grab the inuse count.
            assert(bc.get_membuf()->is_inuse());

            /*
             * Before we issue READ to populate the bytes_chunk, take the
             * membuf lock. We use try_lock() and skip readahead if we don't
             * get the lock. It's ok to skip readahead rather than holding the
             * caller. Mostly if there is a single reader we will get the lock.
             * This lock will be released in the readahead_callback() after the
             * buffer is populated.
             * Note that if the membuf is already locked it means some other
             * context is already performing IO to it. We should not release
             * the buffer.
             */
            if (!bc.get_membuf()->try_lock()) {
                AZLogWarn("[{}] Skipping readahead at off: {} len: {}. "
                          "Could not get membuf lock!",
                          inode->get_fuse_ino(), bc.offset, bc.length);

                on_readahead_complete(bc.offset, bc.length);
                bc.get_membuf()->clear_inuse();
                continue;
            }

            /*
             * If the buffer is already uptodate, skip readahead.
             */
            if (bc.get_membuf()->is_uptodate()) {
                AZLogWarn("[{}] Skipping readahead at off: {} len: {}. "
                          "Membuf already uptodate!",
                          inode->get_fuse_ino(), bc.offset, bc.length);

                on_readahead_complete(bc.offset, bc.length);
                bc.get_membuf()->clear_locked();
                bc.get_membuf()->clear_inuse();
                continue;
            }

            /*
             * Ok, now issue READ RPCs to read this byte range.
             */
            struct rpc_task *tsk =
                client->get_rpc_task_helper()->alloc_rpc_task(FUSE_READ);

            /*
             * fuse_req is needed to send the fuse response, since we don't
             * need to send response for readahead reads, it can be null.
             * fuse_file_info is not used too.
             */
            tsk->init_read(nullptr,                /* fuse_req */
                           inode->get_fuse_ino(),  /* ino */
                           bc.length,              /* size */
                           bc.offset,              /* offset */
                           nullptr);               /* fuse_file_info */

            // No reads should be issued to backend at this point.
            assert(bc.num_backend_calls_issued == 0);
            bc.num_backend_calls_issued++;

            assert(bc.pvt == 0);

            /*
             * bc holds a ref on the membuf so we can safely access membuf
             * only till we have bc in the scope. In readahead_callback() we
             * need to access bc, hence we transfer ownership to the ra_context
             * object allocated below.
             */
            struct ra_context *ctx = new ra_context(tsk, bc);
            assert(ctx->bc.num_backend_calls_issued == 1);

            READ3args args;
            ::memset(&args, 0, sizeof(args));
            args.file = inode->get_fh();
            args.offset = bc.offset;
            args.count = bc.length;

            /*
             * Grab a ref on this inode so that it is not freed when the
             * readahead reads are going on. Since the fuse layer does not
             * know of this readahead operation, it is possible that the fuse
             * may release this inode soon after the application read returns.
             * We do not want to be in that state and hence grab an extra ref
             * on this inode.
             * This should be decremented in readahead_callback()
             */
            inode->incref();

            AZLogDebug("[{}] Issuing readahead read to backend at "
                       "off: {} len: {}",
                       inode->get_fuse_ino(),
                       args.offset,
                       args.count);

            rpc_pdu *pdu = nullptr;

            /*
             * tsk->get_rpc_ctx() call below will round robin readahead
             * requests across all available connections.
             *
             * TODO: See if issuing a batch of reads over one connection
             *       before moving to the other connection helps.
             */
            tsk->get_stats().on_rpc_issue();
            if ((pdu = rpc_nfs3_read_task(
                        tsk->get_rpc_ctx(),
                        readahead_callback,
                        bc.get_buffer(),
                        bc.length,
                        &args,
                        ctx)) == NULL) {
                tsk->get_stats().on_rpc_cancel();
                /*
                 * This call failed due to internal issues like OOM etc
                 * and not due to an actual RPC/NFS error, anyways pretend
                 * as if we never issued this.
                 */
                AZLogWarn("[{}] Skipping readahead at off: {} len: {}. "
                          "rpc_nfs3_read_task() failed!",
                          inode->get_fuse_ino(), args.offset, args.count);

                on_readahead_complete(bc.offset, bc.length);
                bc.get_membuf()->clear_locked();
                bc.get_membuf()->clear_inuse();

                // Release the buffer since we did not fill it.
                read_cache->release(bc.offset, bc.length);
                tsk->free_rpc_task();
                delete ctx;

                // Decrement the extra ref that was taken.
                inode->decref();

                continue;
            }

            ra_issued++;

            AZLogDebug("[{}] rpc_nfs3_read_task() successfully dispatched "
                       "#{} readahead at off: {} len: {}. ",
                       inode->get_fuse_ino(),
                       ra_issued,
                       args.offset,
                       args.count);
        }
    }

    if (ra_issued == 0) {
        // Log once every 1000 failed calls.
        if ((++num_no_readahead % 1000) == 0) {
            AZLogDebug("[{}] num_no_readahead={}, reason={}",
                       inode->get_fuse_ino(), num_no_readahead, ra_offset);
        }
    }

    return ra_issued;
}

/* static */
int ra_state::unit_test()
{
    ra_state ras{128 * 1024, 4 * 1024};
    int64_t next_ra;
    int64_t next_read;
    int64_t complete_ra;

    AZLogInfo("Unit testing ra_state, start");

    // 1st read.
    next_read = 0*_MiB;
    ras.on_application_read(next_read, 1*_MiB);

    // Only 1 read complete, cannot confirm sequential pattern till 3 reads.
    assert(ras.get_next_ra(4*_MiB) == 0);

    next_read += 1*_MiB;
    ras.on_application_read(next_read, 1*_MiB);

    // Only 2 reads complete, cannot confirm sequential pattern till 3 reads.
    assert(ras.get_next_ra(4*_MiB) == 0);

    next_read += 1*_MiB;
    ras.on_application_read(next_read, 1*_MiB);

    /*
     * Ok 3 reads complete, all were sequential, so now we should get a
     * readahead recommendation.
     */
    next_ra = 3*_MiB;
    assert(ras.get_next_ra(4*_MiB) == next_ra);

    /*
     * Since we have 128MB ra window, next 31 (+1 above) get_next_ra() calls
     * will recommend readahead.
     */
    for (int i = 0; i < 31; i++) {
        next_ra += 4*_MiB;
        /*
         * We don't pass the length parameter to get_next_ra(), it should
         * use the default ra size set in the constructor. We set that to
         * 4MiB.
         */
        assert(ras.get_next_ra() == next_ra);
    }

    // No more readahead reads after full ra window is issued.
    assert(ras.get_next_ra(4*_MiB) == 0);

    /*
     * Complete one readahead.
     * We don't pass the length parameter to on_readahead_complete(), it should
     * use the default ra size set in the constructor. We set that to 4MiB.
     */
    complete_ra = 3*_MiB;
    ras.on_readahead_complete(complete_ra);

    // One more readahead should be allowed.
    next_ra += 4*_MiB;
    assert(ras.get_next_ra(4*_MiB) == next_ra);

    // Not any more.
    assert(ras.get_next_ra(4*_MiB) == 0);

    // Complete all readahead reads.
    for (int i = 0; i < 32; i++) {
        complete_ra += 4*_MiB;
        ras.on_readahead_complete(complete_ra, 4*_MiB);
    }

    // Now it should recommend next readahead.
    next_ra += 4*_MiB;
    assert(ras.get_next_ra(4*_MiB) == next_ra);

    // Complete that one too.
    complete_ra = next_ra;
    ras.on_readahead_complete(complete_ra, 4*_MiB);

    /*
     * Now issue next read at 100MB offset.
     * This will cause access density to drop since now we have a gap of
     * 97MiB and we have just read 4MiB till now.
     */
    ras.on_application_read(100*_MiB, 1*_MiB);
    assert(ras.get_next_ra(4*_MiB) == 0);

    /*
     * Read the entire gap.
     * This will fill the gap and get the access density back to 100%, so
     * now it should recommend readahead.
     */
    for (int i = 0; i < 97; i++) {
        next_read += 1*_MiB;
        ras.on_application_read(next_read, 1*_MiB);
    }

    /*
     * Readahead recommended should be after the last byte read or the last
     * readahead byte, whichever is larger. In this case next readahead is
     * larger.
     */
    next_ra += 4*_MiB;
    assert(next_ra > 101*_MiB);
    assert(ras.get_next_ra(4*_MiB) == next_ra);

    /*
     * Read from a new section.
     * This should reset the pattern detector and it should not recommend a
     * readahead, till it again confirms a sequential pattern.
     */
    next_read = 2*_GiB;
    ras.on_application_read(next_read, 1*_MiB);
    assert(ras.get_next_ra(4*_MiB) == 0);

    // 2nd read in the new section.
    next_read += 1*_MiB;
    ras.on_application_read(next_read, 1*_MiB);
    assert(ras.get_next_ra(4*_MiB) == 0);

    // 3rd read in the new section.
    next_read += 1*_MiB;
    ras.on_application_read(next_read, 1*_MiB);

    next_ra = next_read + 1*_MiB;
    assert(ras.get_next_ra(4*_MiB) == next_ra);

    /*
     * Read from the next section. We will only do random reads so pattern
     * detector should not see a seq pattern and must not recommend readahead.
     */
    next_read = 4*_GiB;
    ras.on_application_read(next_read, 1*_MiB);
    assert(ras.get_next_ra(4*_MiB) == 0);

    for (int i = 0; i < 1000; i++) {
        next_read = random_number(0, 1*_TiB);
        ras.on_application_read(next_read, 1*_MiB);
        assert(ras.get_next_ra(4*_MiB) == 0);

        next_read = random_number(1*_TiB, 2*_TiB);
        ras.on_application_read(next_read, 1*_MiB);
        assert(ras.get_next_ra(4*_MiB) == 0);
    }

    /*
     * Jump to a new section.
     * Here we will only do sequential reads. After 3 sequential reads, we
     * should detect the pattern and after that we should recommend readahead.
     */
    next_read = 10*_GiB;
    ras.on_application_read(next_read, 1*_MiB);
    assert(ras.get_next_ra(4*_MiB) == 0);

    next_read += 1*_MiB;
    ras.on_application_read(next_read, 1*_MiB);
    assert(ras.get_next_ra(4*_MiB) == 0);

    next_read += 1*_MiB;
    ras.on_application_read(next_read, 1*_MiB);

    next_ra = next_read+1*_MiB;
    assert(ras.get_next_ra(4*_MiB) == next_ra);

    for (int i = 0; i < 2000; i++) {
        next_read += 1*_MiB;
        ras.on_application_read(next_read, 1*_MiB);

        next_ra += 4*_MiB;
        assert(ras.get_next_ra(4*_MiB) == next_ra);

        complete_ra = next_ra;
        ras.on_readahead_complete(complete_ra, 4*_MiB);
    }

    // Stress run.
    for (int i = 0; i < 10'000'000; i++) {
        next_read = random_number(0, 1*_TiB);
        ras.on_application_read(next_read, 1*_MiB);
        assert(!ras.is_sequential());
        assert(ras.get_next_ra(4*_MiB) == 0);

        next_read = random_number(1*_TiB, 2*_TiB);
        ras.on_application_read(next_read, 1*_MiB);
        assert(!ras.is_sequential());
        assert(ras.get_next_ra(4*_MiB) == 0);

        next_read = random_number(2*_TiB, 3*_TiB);
        ras.on_application_read(next_read, 1*_MiB);
        assert(!ras.is_sequential());
        assert(ras.get_next_ra(4*_MiB) == 0);

        next_read = random_number(3*_TiB, 4*_TiB);
        ras.on_application_read(next_read, 1*_MiB);
        assert(!ras.is_sequential());
        assert(ras.get_next_ra(4*_MiB) == 0);
    }

    AZLogInfo("Unit testing ra_state, done!");

    return 0;
}

#ifdef DEBUG_READAHEAD
static int _i = ra_state::unit_test();
#endif

}
