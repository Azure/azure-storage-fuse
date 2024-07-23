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

#define _MiB (1024 * 1024ULL)
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

    // We shouldn't be called for directories.
    assert(!inode->is_dir());

    // Readahead needs filecache.
    assert(inode->filecache_handle != nullptr);

    /*
     * By the time ra_state is initialized mount must have already
     * completed and we must have the rsize/wsize value advertized
     * by the server.
     */
    assert(client->mnt_options.rsize_adj >= AZNFSCFG_RSIZE_MIN &&
           client->mnt_options.rsize_adj <= AZNFSCFG_RSIZE_MAX);

    assert(client->mnt_options.readahead_kb >= AZNFSCFG_READAHEAD_KB_MIN &&
           client->mnt_options.readahead_kb <= AZNFSCFG_READAHEAD_KB_MAX);

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
    const struct bytes_chunk bc;

    /*
     * rpc_task tracking this readahead.
     *
     * Note: We don't strictly need an rpc_task to track readahead reads
     *       since we don't need to send a fuse reply, but we still use
     *       one rpc_task per readahead read so that readahead reads are
     *       also limited by the number of concurrent rpc_tasks allowed.
     */
    struct rpc_task *task;

    ra_context(rpc_task *_task, const struct bytes_chunk& _bc) :
        bc(_bc),
        task(_task)
    {
        assert(task->magic == RPC_TASK_MAGIC);
        assert(bc.length > 0 && bc.length <= AZNFSC_MAX_CHUNK_SIZE);
        assert(bc.offset < AZNFSC_MAX_FILE_SIZE);
    }
};

static void readahead_callback (
    struct rpc_context* /* rpc */,
    int rpc_status,
    void *data,
    void *private_data)
{
    struct ra_context *ctx = (struct ra_context*) private_data;
    struct rpc_task *task = ctx->task;
    const struct bytes_chunk *bc = &ctx->bc;
    auto res = (READ3res*) data;
    const char *errstr = nullptr;

    assert(task->magic == RPC_TASK_MAGIC);
    assert(bc->length > 0);

    // Cannot have read more than requested.
    assert(res->READ3res_u.resok.count <= bc->length);

    const int status = task->status(rpc_status, NFS_STATUS(res), &errstr);
    const fuse_ino_t ino = task->rpc_api.read_task.get_inode();
    struct nfs_inode *inode = task->get_client()->get_nfs_inode_from_ino(ino);
    const auto read_cache = inode->filecache_handle;

    assert(read_cache != nullptr);
    assert(ino == inode->get_fuse_ino());

    // Success or failure, report readahead completion.
    inode->readahead_state->on_readahead_complete(bc->offset, bc->length);

    if (status != 0) {
        /*
         * Readahead read failed? Nothing to do, unlock the membuf, release
         * the byte range and pretend as if we never issued this read.
         */
        assert(res->READ3res_u.resok.count == 0);

        bc->get_membuf()->clear_locked();
        bc->get_membuf()->clear_inuse();

        // Release the buffer since we did not fill it.
        read_cache->release(bc->offset, bc->length);

        AZLogWarn("[{}] readahead_callback [FAILED]: "
                  "Requested (off: {}, len: {}): rpc_status={}, "
                  "nfs_status={}, error={}",
                  ino,
                  bc->offset,
                  bc->length,
                  rpc_status,
                  (int) NFS_STATUS(res),
                  errstr);
        goto delete_ctx;
    }

    if (res->READ3res_u.resok.count != bc->length) {
        /*
         * Most common reason of partial read would be readahead beyond eof,
         * but server may return partial reads even for reads within the file.
         *
         * XXX Making it a warning log for now so that we analyze these reads.
         *     Later make it an info log.
         */
        if (res->READ3res_u.resok.eof) {
            AZLogDebug("[{}] readahead_callback [PARTIAL READ (EOF)]: "
                       "Requested (off: {}, len: {}), Read (len: {} eof: {})",
                       ino,
                       bc->offset,
                       bc->length,
                       res->READ3res_u.resok.count,
                       res->READ3res_u.resok.eof);
        } else {
            AZLogWarn("[{}] readahead_callback [PARTIAL READ (NOT EOF)]: "
                      "Requested (off: {}, len: {}), Read (len: {} eof: {})",
                      ino,
                      bc->offset,
                      bc->length,
                      res->READ3res_u.resok.count,
                      res->READ3res_u.resok.eof);
        }

        bc->get_membuf()->clear_locked();
        bc->get_membuf()->clear_inuse();

        /*
         * In case of short read we cannot safely mark the membuf as uptodate
         * as we risk some other thread reading one or more bytes from the
         * released part of the membuf and incorrectly treating them as
         * uptodate. Note that we have not written those bytes so that other
         * reader will get garbage data.
         */
        read_cache->release(bc->offset + res->READ3res_u.resok.count,
                            bc->length - res->READ3res_u.resok.count);

        goto delete_ctx;
    }

    // Common case.
    AZLogInfo("[{}] readahead_callback: off: {}, len: {}, eof: {}",
               ino,
               bc->offset,
               bc->length,
               res->READ3res_u.resok.eof);

    if (bc->is_empty) {
        /*
         * Only the first read which got hold of the complete membuf will have
         * this byte_chunk set to empty. Only such reads should set the uptodate
         * flag.
         */
        AZLogDebug("Setting uptodate flag. off: {}, len: {}",
                  task->rpc_api.read_task.get_offset(),
                  task->rpc_api.read_task.get_size());

        assert(bc->maps_full_membuf());
        bc->get_membuf()->set_uptodate();
    } else {
        AZLogDebug("Not updating uptodate flag. off: {}, len: {}",
                  task->rpc_api.read_task.get_offset(),
                  task->rpc_api.read_task.get_size());
    }

    /*
     * Release the lock that we held on the membuf since the data is now
     * written and ready to be read.
     */
    bc->get_membuf()->clear_locked();
    bc->get_membuf()->clear_inuse();

delete_ctx:
    // Free the readahead RPC task.
    task->free_rpc_task();

    // Free the context.
    delete ctx;
}

int ra_state::issue_readaheads()
{
    uint64_t ra_offset;
    auto read_cache = inode->filecache_handle;
    int ra_issued = 0;

    /*
     * No cache, can't readahead.
     */
    if (!read_cache) {
        return 0;
    }

    /*
     * Issue all readaheads allowed by this ra_state.
     */
    while ((ra_offset = get_next_ra()) != 0) {
        AZLogDebug("[{}] Issuing readahead at off: {} len: {}",
                   inode->get_fuse_ino(), ra_offset, def_ra_size);

        /*
         * Get bytes_chunk representing the byte range we want to readahead
         * and issue READ RPCs for all.
         */
        std::vector<bytes_chunk> bcv = read_cache->get(ra_offset, def_ra_size);

        for (const bytes_chunk& bc : bcv) {

            // Every bytes_chunk must lie within the readahead.
            assert(bc.offset >= ra_offset);
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

                bc.get_membuf()->clear_locked();
                bc.get_membuf()->clear_inuse();
                continue;
            }

            /*
             * Ok, now issue READ RPCs to read this byte range.
             */
            struct rpc_task *tsk =
                client->get_rpc_task_helper()->alloc_rpc_task();

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

            /*
             * bc holds a ref on the membuf so we can safely access membuf
             * only till we have bc in the scope. In readahead_callback() we
             * need to access bc, hence we transfer ownership to the ra_context
             * object allocated below.
             */
            struct ra_context *ctx = new ra_context(tsk, bc);

            READ3args args;
            ::memset(&args, 0, sizeof(args));
            args.file = inode->get_fh();
            args.offset = bc.offset;
            args.count = bc.length;

            AZLogInfo("[{}] Issuing readahead read to backend at "
                       "off: {} len: {}",
                       inode->get_fuse_ino(),
                       args.offset,
                       args.count);

            /*
             * tsk->get_rpc_ctx() call below will round robin readahead
             * requests across all available connections.
             *
             * TODO: See if issuing a batch of reads over one connection
             *       before moving to the other connection helps.
             */
            if (rpc_nfs3_read_task(
                        tsk->get_rpc_ctx(),
                        readahead_callback,
                        bc.get_buffer(),
                        bc.length,
                        &args,
                        ctx) == NULL) {
                /*
                 * This call failed due to internal issues like OOM etc
                 * and not due to an actual RPC/NFS error, anyways pretend
                 * as if we never issued this.
                 */
                AZLogWarn("[{}] Skipping readahead at off: {} len: {}. "
                          "rpc_nfs3_read_task() failed!",
                          inode->get_fuse_ino(), args.offset, args.count);

                bc.get_membuf()->clear_locked();
                bc.get_membuf()->clear_inuse();

                // Release the buffer since we did not fill it.
                read_cache->release(bc.offset, bc.length);
                tsk->free_rpc_task();
                delete ctx;
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

    return ra_issued;
}

/* static */
int ra_state::unit_test()
{
    ra_state ras{128 * 1024, 4 * 1024};
    uint64_t next_ra;
    uint64_t next_read;
    uint64_t complete_ra;

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
