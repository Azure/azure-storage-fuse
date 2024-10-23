#include "nfs_inode.h"
#include "nfs_client.h"
#include "file_cache.h"
#include "rpc_task.h"

/**
 * Constructor.
 * nfs_client must be known when nfs_inode is being created.
 * Fuse inode number is set to the address of the nfs_inode object,
 * unless explicitly passed by the caller, which will only be done
 * for the root inode.
 */
nfs_inode::nfs_inode(const struct nfs_fh3 *filehandle,
                     const struct fattr3 *fattr,
                     struct nfs_client *_client,
                     uint32_t _file_type,
                     fuse_ino_t _ino) :
    file_type(_file_type),
    fh(*filehandle),
    crc(calculate_crc32(fh.get_fh())),
    ino(_ino == 0 ? (fuse_ino_t) this : _ino),
    generation(get_current_usecs()),
    client(_client)
{
    // Sanity asserts.
    assert(magic == NFS_INODE_MAGIC);
    assert(filehandle != nullptr);
    assert(fattr != nullptr);
    assert(client != nullptr);
    assert(client->magic == NFS_CLIENT_MAGIC);
    assert(write_error == 0);

#ifndef ENABLE_NON_AZURE_NFS
    // Blob NFS supports only these file types.
    assert((file_type == S_IFREG) ||
           (file_type == S_IFDIR) ||
           (file_type == S_IFLNK));
#endif

    // ino is either set to FUSE_ROOT_ID or set to address of nfs_inode.
    assert((ino == (fuse_ino_t) this) || (ino == FUSE_ROOT_ID));

    /*
     * We always have fattr when creating nfs_inode.
     * Most common case is we are creating nfs_inode when we got a fh (and
     * attributes) for a file, f.e., LOOKUP, CREATE, READDIRPLUS, etc.
     */
    attr.st_ctim = {0, 0};
    nfs_client::stat_from_fattr3(&attr, fattr);

    // file type as per fattr should match the one passed explicitly..
    assert((attr.st_mode & S_IFMT) == file_type);

    attr_timeout_secs = get_actimeo_min();
    attr_timeout_timestamp = get_current_msecs() + attr_timeout_secs*1000;

    /*
     * These are later allocated in open() when we know for sure that they
     * will be needed. f.e., we don't want to create file/dir cache for every
     * file/dir that's enumerated.
     */
    assert(!filecache_handle);
    assert(!filecache_alloced);
    assert(!dircache_handle);
    assert(!dircache_alloced);
    assert(!readahead_state);
    assert(!rastate_alloced);

    assert(lookupcnt == 0);
    assert(dircachecnt == 0);

    assert(!is_silly_renamed);
    assert(silly_renamed_name.empty());
    assert(parent_ino == 0);
}

nfs_inode::~nfs_inode()
{
    assert(magic == NFS_INODE_MAGIC);
    // We should never delete an inode which fuse still has a reference on.
    assert(is_forgotten());
    assert(lookupcnt == 0);
    assert(forget_expected == 0);

#if 1
    /*
     * XXX Remove me once this bug is fixed.
     *     Last time when I hit this, it was for a directory for which the
     *     lookupcnt unnaturally dropped by a large value (4), and it was
     *     not called from decref.. was weird!!
     */
    if (opencnt != 0) {
        AZLogError("[{}:{}] opencnt = {}!", get_filetype_coding(), ino, opencnt.load());
    }
#endif
    // We should never delete an inode which is still open()ed by user.
    assert(opencnt == 0);

    /*
     * We should never delete an inode while it is still referred by parent
     * dir cache.
     */
    assert(dircachecnt == 0);

    /*
     * Directory inodes must not be freed while they have a non-empty dir
     * cache.
     */
    assert((filecache_handle == nullptr) == (filecache_alloced == false));
    assert((dircache_handle == nullptr) == (dircache_alloced == false));
    assert((readahead_state == nullptr) == (rastate_alloced == false));
    assert(((filecache_handle != nullptr) + (dircache_handle != nullptr)) < 2);
    assert(is_cache_empty());

    assert((ino == (fuse_ino_t) this) || (ino == FUSE_ROOT_ID));
    assert(client != nullptr);
    assert(client->magic == NFS_CLIENT_MAGIC);

#ifdef ENABLE_PARANOID
    if (is_silly_renamed) {
        assert(!silly_renamed_name.empty());
        assert(parent_ino != 0);
    } else {
        assert(silly_renamed_name.empty());
        assert(parent_ino == 0);
    }
#endif
}

/**
 * LOCKS: inode_map_lock_0.
 *        readdircache_lock_2 for directory.
 *        chunkmap_lock_43 for file.
 */
void nfs_inode::decref(size_t cnt, bool from_forget)
{
    AZLogDebug("[{}:{}] decref(cnt={}, from_forget={}) called "
               " (lookupcnt={}, dircachecnt={}, forget_expected={})",
               get_filetype_coding(), ino, cnt, from_forget,
               lookupcnt.load(), dircachecnt.load(),
               forget_expected.load());

    /*
     * We only decrement lookupcnt in forget and once lookupcnt drops to
     * 0 we mark the inode as forgotten, so decref() should not be called
     * for forgotten inode.
     */
    assert(!is_forgotten());
    assert(cnt > 0);
    assert(lookupcnt >= cnt);

    if (from_forget) {
#ifdef ENABLE_PARANOID
        /*
         * Fuse should not call more forgets than how many times we returned
         * the inode to fuse.
         */
        if ((int64_t) cnt > forget_expected) {
            AZLogError("[{}:{}] Extra forget from fuse @ {}, got {}, "
                       "expected {}, last forget seen @ {}, lookupcnt={}, "
                       "dircachecnt={}",
                       get_filetype_coding(), ino,
                       get_current_usecs(), cnt, forget_expected.load(),
                       last_forget_seen_usecs, lookupcnt.load(),
                       dircachecnt.load());
            assert(0);
        }
        last_forget_seen_usecs = get_current_usecs();
#endif

        forget_expected -= cnt;
        assert(forget_expected >= 0);
    }

try_again:
    /*
     * Grab an extra ref so that the lookupcnt-=cnt does not cause the refcnt
     * to drop to 0, else some other thread can delete the inode before we get
     * to call put_nfs_inode().
     */
    ++lookupcnt;
    const bool forget_now = ((lookupcnt -= cnt) == 1);

    if (forget_now) {
        /*
         * For directory inodes it's a good time to purge the dircache, since
         * fuse VFS has lost all references on the directory. Note that we
         * can purge the directory cache at a later point also, but doing it
         * here causes the fuse client to behave like the Linux kernel NFS
         * client where we can purge the directory cache by writing to
         * /proc/sys/vm/drop_caches.
         * Also for files since the inode last ref is dropped, further accesses
         * are unlikely, hence we can drop file caches too.
         *
         * Note that invalidate_cache with purge_now=true, will take exclusive
         * lock on chunkmap_lock_43 for files and readdircache_lock_2 for
         * directories.
         */
        invalidate_cache(true /* purge_now */);

        /*
         * Reduce the extra refcnt and revert the cnt.
         * After this the inode will have 'cnt' references that need to be
         * dropped by put_nfs_inode() call below, with inode_map_lock_0 held.
         */
        lookupcnt += (cnt - 1);
        assert(lookupcnt >= cnt);

        /*
         * It's possible that while we were purging the dir cache above,
         * some other thread got a new ref on this inode (maybe it enumerated
         * its parent dir). In that case put_nfs_inode() will not free the
         * inode.
         */
        if (lookupcnt == cnt) {
            AZLogDebug("[{}:{}] lookupcnt dropping by {}, to 0, forgetting inode",
                       get_filetype_coding(), ino, cnt);
        } else {
            AZLogWarn("[{}:{}] lookupcnt dropping by {}, to {} "
                      "(some other thread got a fresh ref)",
                      get_filetype_coding(), ino, cnt, lookupcnt - cnt);
        }

        /*
         * This FORGET would drop the lookupcnt to 0, fuse vfs should not send
         * any more forgets, delete the inode. Note that before we grab the
         * inode_map_lock_0 in put_nfs_inode() some other thread can reuse the
         * forgotten inode, in which case put_nfs_inode() will just skip it.
         *
         * TODO: In order to avoid taking inode_map_lock_0 for every forget,
         *       see if we should batch them in a threadlocal vector and call
         *       put_nfs_inodes() for a batch.
         */
        client->put_nfs_inode(this, cnt);
    } else {
        /*
         * After the --lookupcnt below some other thread calling decref()
         * can delete this inode, so don't access it after that, hence we
         * log before that but with updated lookupcnt.
         */
        AZLogDebug("[{}:{}] lookupcnt decremented by {}, to {}, "
                   "dircachecnt: {}, forget_expected: {}",
                   get_filetype_coding(), ino, cnt,
                   lookupcnt.load() - 1, dircachecnt.load(),
                   forget_expected.load());

        if (--lookupcnt == 0) {
            /*
             * This means that there was some thread holding a lookupcnt
             * ref on the inode but it just now released it (after we checked
             * above and before the --lookupcnt here) and now this forget
             * makes this inode's lookupcnt 0.
             */
            lookupcnt += cnt;
            goto try_again;
        }
    }
}

bool nfs_inode::in_ra_window(uint64_t offset, uint64_t length) const
{
    if (!has_rastate()) {
        return false;
    }

    return get_rastate()->in_ra_window(offset, length);
}

/**
 * Note: nfs_inode::lookup() method currently has limited usage.
 *       It is only meant to be called from silly_rename() where we know
 *       that kernel must be holding a lock on the to-be-deleted file's
 *       inode and hence we can be certain that the corresponding nfs_inode
 *       pointer is accessible. Note that we don't take ref on the nfs_inode
 *       and depend on the kernel holding a use count on the inode.
 *       Even if the parent dir mtime changes and we do a revalidate() and
 *       lookup_sync(), the corresponding nfs_inode will still be present in
 *       our inode_map since kernel wouldn't have called forget on the inode.
 */
struct nfs_inode *nfs_inode::lookup(const char *filename)
{
    // Must be called only for a directory inode.
    assert(is_dir());

    // Revalidate to ensure dnlc cache can be safely used.
    revalidate();

    /*
     * First search in dnlc, if not found perform LOOKUP RPC.
     */
    struct nfs_client *client = get_client();
    struct nfs_inode *child_inode = dnlc_lookup(filename);
    fuse_ino_t child_ino = 0;

    if (child_inode) {
        child_ino = child_inode->get_fuse_ino();
        assert(child_ino != 0);

        AZLogDebug("{}/{} -> {}, found in DNLC! (lookupcnt: {}, "
                   "dircachecnt: {}, forget_expected: {})",
                   get_fuse_ino(), filename, child_ino,
                   child_inode->lookupcnt.load(),
                   child_inode->dircachecnt.load(),
                   child_inode->forget_expected.load());
       /*
        * Caller doesn't expect a ref on the inode, drop the ref held by
        * dnlc_lookup().
        */
        child_inode->decref();
    }

    if (child_ino == 0) {
       if (!client->lookup_sync(get_fuse_ino(), filename, child_ino)) {
           AZLogDebug("{}/{}, sync LOOKUP failed!",
                      get_fuse_ino(), filename);
           return nullptr;
       }
       assert(child_ino != 0);
       child_inode = client->get_nfs_inode_from_ino(child_ino);

       AZLogDebug("{}/{} -> {}, found via sync LOOKUP! (lookupcnt: {}, "
                  "dircachecnt: {}, forget_expected: {})",
                  get_fuse_ino(), filename, child_ino,
                  child_inode->lookupcnt.load(),
                  child_inode->dircachecnt.load(),
                  child_inode->forget_expected.load());
       /*
        * Caller doesn't expect a ref on the child_inode, drop the ref held by
        * lookup_sync().
        */
       child_inode->decref();
    }

    return child_inode;
}

int nfs_inode::get_actimeo_min() const
{
    switch (file_type) {
        case S_IFDIR:
            return client->mnt_options.acdirmin;
        default:
            return client->mnt_options.acregmin;
    }
}

int nfs_inode::get_actimeo_max() const
{
    switch (file_type) {
        case S_IFDIR:
            return client->mnt_options.acdirmax;
        default:
            return client->mnt_options.acregmax;
    }
}

void nfs_inode::sync_membufs(std::vector<bytes_chunk> &bc_vec, bool is_flush)
{
    if (bc_vec.empty()) {
        return;
    }

    /*
     * Create the flush task to carry out the write.
     */
    struct rpc_task *flush_task = nullptr;

    // Flush dirty membufs to backend.
    for (bytes_chunk &bc : bc_vec) {
        /*
         * Get the underlying membuf for bc.
         * Note that we write the entire membuf, even though bc may be referring
         * to a smaller window.
         *
         * Correction: We may not write the entire membuf in case the bytes_chunk
         *             was trimmed. Since get_dirty_bc_range() returns full
         *             bytes_chunks from the chunkmap, we should get full
         *             (but potentially trimmed) bytes_chunks here.
         */
        struct membuf *mb = bc.get_membuf();

        /*
         * Verify the mb.
         * Caller must hold an inuse count on the membufs.
         * sync_membufs() takes ownership of that inuse count and will drop it.
         * We have two cases:
         * 1. We decide to issue the write IO.
         *    In this case the inuse count will be dropped by
         *    write_iov_callback().
         *    This will be the only inuse count and the buffer will be
         *    release()d after write_iov_callback() (in bc_iovec destructor).
         * 2. We found the membuf as flushing.
         *    In this case we don't issue the write and return, but only after
         *    dropping the inuse count.
         */
        assert(mb != nullptr);
        assert(mb->is_inuse());

        if (is_flush) {
            /*
             * get_dirty_bc_range() must have held an inuse count.
             * We hold an extra inuse count so that we can safely wait for the
             * flush in the "waiting loop" in nfs_inode::flush_cache_and_wait().
             * This is needed as we drop inuse count if membuf is already being
             * flushed by another thread or it may drop when the write_iov_callback()
             * completes which can happen before we reach the waiting loop.
             */
            mb->set_inuse();
        }

        /*
         * Lock the membuf. If multiple writer threads want to flush the same
         * membuf the first one will find it dirty and not flushing, that thread
         * should initiate the Blob write. Others that come in while the 1st thread
         * started flushing but the write has not completed, will find it "dirty
         * and flushing" and they can avoid the write and optionally choose to wait
         * for it to complete by waiting for the lock. Others who find it after the
         * write is done and lock is released will find it not "dirty and not
         * flushing". They can just skip.
         *
         * Note that we allocate the rpc_task for flush before the lock as it may
         * block.
         * TODO: We don't do it currently, fix this!
         */
        if (mb->is_flushing() || !mb->is_dirty()) {
            mb->clear_inuse();

            continue;
        }

        mb->set_locked();
        if (mb->is_flushing() || !mb->is_dirty()) {
            mb->clear_locked();
            mb->clear_inuse();

            continue;
        }

        if (flush_task == nullptr) {
            flush_task =
                get_client()->get_rpc_task_helper()->alloc_rpc_task(FUSE_FLUSH);
            flush_task->init_flush(nullptr /* fuse_req */, ino);
            assert(flush_task->rpc_api->pvt == nullptr);
            flush_task->rpc_api->pvt = new bc_iovec(this);
        }

        /*
         * Add as many bytes_chunk to the flush_task as it allows.
         * Once packed completely, then dispatch the write.
         */
        if (flush_task->add_bc(bc)) {
            continue;
        } else {
            /*
             * This flush_task will orchestrate this write.
             */
            flush_task->issue_write_rpc();

            /*
             * Create the new flush task to carry out the write for next bc,
             * which we failed to add to the existing flush_task.
             */
            flush_task =
                get_client()->get_rpc_task_helper()->alloc_rpc_task(FUSE_FLUSH);
            flush_task->init_flush(nullptr /* fuse_req */, ino);
            assert(flush_task->rpc_api->pvt == nullptr);
            flush_task->rpc_api->pvt = new bc_iovec(this);

            // Single bc addition should not fail.
            [[maybe_unused]] bool res = flush_task->add_bc(bc);
            assert(res == true);
        }
    }

    // Dispatch the leftover bytes (or full write).
    if (flush_task) {
        flush_task->issue_write_rpc();
    }
}

/**
 * Note: This takes shared lock on ilock_1.
 */
int nfs_inode::copy_to_cache(const struct fuse_bufvec* bufv,
                             off_t offset,
                             uint64_t *extent_left,
                             uint64_t *extent_right)
{
    /*
     * XXX We currently only handle bufv with count=1.
     *     Ref aznfsc_ll_write_buf().
     */
    assert(bufv->count == 1);

    /*
     * copy_to_cache() must be called only for a regular file and it must have
     * filecache initialized.
     */
    assert(is_regfile());
    assert(has_filecache());
    assert(offset < (off_t) AZNFSC_MAX_FILE_SIZE);

    assert(bufv->idx < bufv->count);
    const size_t length = bufv->buf[bufv->idx].size - bufv->off;
    assert((int) length >= 0);
    assert((offset + length) <= AZNFSC_MAX_FILE_SIZE);
    /*
     * TODO: Investigate using splice for zero copy.
     */
    const char *buf = (char *) bufv->buf[bufv->idx].mem + bufv->off;
    int err = 0;
    bool inject_eagain = false;

    /*
     * Get bytes_chunk(s) covering the range [offset, offset+length).
     * We need to copy application data to those.
     */
    std::vector<bytes_chunk> bc_vec =
        filecache_handle->getx(offset, length, extent_left, extent_right);

    size_t remaining = length;

    for (auto& bc : bc_vec) {
        struct membuf *mb = bc.get_membuf();
#ifdef ENABLE_PARANOID
        bool found_not_uptodate = false;

        if (!err && inject_error()) {
            err = EAGAIN;
            AZLogWarn("[{}] PP: copy_to_cache(): injecting EAGAIN for membuf "
                      "[{}, {}) (bc [{}, {})), length={}, remaining={}",
                      ino, mb->offset, mb->offset+mb->length,
                      bc.offset, bc.offset+bc.length,
                      length, remaining);
        }
#endif

        /*
         * If we have already failed with EAGAIN, just drain the bc_vec
         * clearing the inuse count for all the bytes_chunk.
         *
         * TODO: If we have copied at least one byte, do not fail but instead
         *       let the caller know that we copied ledd.
         */
        if (err == EAGAIN) {
            mb->clear_inuse();
            assert(remaining >= bc.length);
            remaining -= bc.length;
            continue;
        }

        /*
         * Lock the membuf while we copy application data into it.
         */
        mb->set_locked();

        /*
         * If we own the full membuf we can safely copy to it, also if the
         * membuf is uptodate we can safely copy to it. In both cases the
         * membuf remains uptodate after the copy.
         */
try_copy:
        if (bc.maps_full_membuf() || mb->is_uptodate()) {
            assert(bc.length <= remaining);
            ::memcpy(bc.get_buffer(), buf, bc.length);
            mb->set_uptodate();
            mb->set_dirty();
        } else {
#ifdef ENABLE_PARANOID
            /*
             * Once we find the membuf uptodate, after waiting, and run
             * try_copy again, we must not find the membuf not-uptodate
             * again.
             */
            assert(!found_not_uptodate);
            found_not_uptodate = true;
#endif

            /*
             * bc refers to part of the membuf and membuf is not uptodate.
             * This can happen if our bytes_chunk_cache::get() call raced with
             * some other thread and they requested a bigger bytes_chunk than
             * us. The original bytes_chunk was allocated per their request
             * and our request was smaller one that fitted completely within
             * their request and and hence we were given the same membuf,
             * albeit a smaller bytes_chunk. Now both the threads would next
             * try to lock the membuf to perform their corresponding IO, this
             * time we won the race and hence when we look at the membuf it's
             * a partial one and not uptodate. Since membuf is not uptodate
             * we will need to do a read-modify-write operation to correctly
             * update part of the membuf. Since we know that some other thread
             * is waiting to perform IO on the entire membuf, we simply let
             * that thread proceed with its IO. Once it's done the membuf will
             * be uptodate and then we can perform the simple copy.
             * We wait for 50 msecs after releasing the lock to let the other
             * thread get the lock. Once it gets the lock it'll only release
             * it after it performs the IO. So, after we reacquire the lock
             * if the membuf is not uptodate it implies that the other thread
             * wasn't able to mark the membuf uptodate. In this case we need
             * to get fresh bytes_chunk vector and re-do the copy.
             */
            AZLogWarn("[{}] Waiting for membuf [{}, {}) (bc [{}, {})) to "
                      "become uptodate", ino,
                      mb->offset, mb->offset+mb->length,
                      bc.offset, bc.offset+bc.length);

            mb->clear_locked();
            ::usleep(50 * 1000);
            mb->set_locked();

#ifdef ENABLE_PARANOID
            inject_eagain = inject_error();
#endif

            if (mb->is_uptodate() && !inject_eagain) {
                AZLogWarn("[{}] Membuf [{}, {}) (bc [{}, {})) is now uptodate, "
                          "retrying copy", ino,
                          mb->offset, mb->offset+mb->length,
                          bc.offset, bc.offset+bc.length);
                goto try_copy;
            } else {
                AZLogWarn("[{}] {}Membuf [{}, {}) (bc [{}, {})) not marked "
                          "uptodate by other thread, returning EAGAIN",
                          ino, inject_eagain ? "PP: " : "",
                          mb->offset, mb->offset+mb->length,
                          bc.offset, bc.offset+bc.length);
                assert(err == 0);
                err = EAGAIN;

                /*
                 * Release the membuf before returning, so that when the caller
                 * calls us again we get a new "full" membuf not this partial
                 * membuf again, else we will be stuck in a loop.
                 * We need to drop the inuse count for release() to work, then
                 * re-acquire it for subsequent code to work.
                 */
                mb->clear_inuse();
                filecache_handle->release(mb->offset, mb->length);
                mb->set_inuse();
            }
        }

        /*
         * Done with the copy, release the membuf lock and clear inuse.
         * The membuf is marked dirty so it's safe against cache prune/release.
         * When we decide to flush this dirty membuf that time it'll be duly
         * locked.
         */
        mb->clear_locked();
        mb->clear_inuse();

        buf += bc.length;
        assert(remaining >= bc.length);
        remaining -= bc.length;
    }

    assert(remaining == 0);
    return err;
}

/**
 * Note: This takes shared lock on ilock_1.
 */
int nfs_inode::flush_cache_and_wait(uint64_t start_off, uint64_t end_off)
{
    /*
     * MUST be called only for regular files.
     * Leave the assert to catch if fuse ever calls flush() on non-reg files.
     */
    if (!is_regfile()) {
        assert(0);
        return 0;
    }

    /*
     * Check if any write error set, if set don't attempt the flush and fail
     * the flush operation.
     */
    const int error_code = get_write_error();
    if (error_code != 0) {
        AZLogWarn("[{}] Previous write to this Blob failed with error={}, "
                  "skipping new flush!", ino, error_code);

        return error_code;
    }

    /*
     * If flush() is called w/o open(), there won't be any cache, skip.
     */
    if (!has_filecache()) {
        return 0;
    }

    /*
     * Get the dirty bytes_chunk from the filecache handle.
     * This will grab an exclusive lock on the file cache and return the list
     * of dirty bytes_chunks at that point. Note that we can have new dirty
     * bytes_chunks created but we don't want to wait for those.
     */
    std::vector<bytes_chunk> bc_vec =
        filecache_handle->get_dirty_bc_range(start_off, end_off);

    /*
     * sync_membufs() iterate over the bc_vec and start flushing the dirty membufs.
     * It batches the contigious dirty membufs and issues a single write RPC for them.
     */
    sync_membufs(bc_vec, true);

    /*
     * Our caller expects us to return only after the flush completes.
     * Wait for all the membufs to flush and get result back.
     */
    for (bytes_chunk &bc : bc_vec) {
        struct membuf *mb = bc.get_membuf();

        assert(mb != nullptr);
        assert(mb->is_inuse());
        mb->set_locked();

        /*
         * If still dirty after we get the lock, it may mean two things:
         * - Write failed.
         * - Some other thread got the lock before us and it made the
         *   membuf dirty again.
         */
        if (mb->is_dirty() && get_write_error()) {
            AZLogError("[{}] Flush [{}, {}) failed with error: {}",
                       ino,
                       bc.offset, bc.offset + bc.length,
                       get_write_error());
        }

        mb->clear_locked();
        mb->clear_inuse();

        /*
         * Release the bytes_chunk back to the filecache.
         * These bytes_chunks are not needed anymore as the flush is done.
         *
         * Note: We come here for bytes_chunks which were found dirty by the
         *       above loop. These writes may or may not have been issued by
         *       us (if not issued by us it was because some other thread,
         *       mostly the writer issued the write so we found it flushing
         *       and hence didn't issue). In any case since we have an inuse
         *       count, release() called from write_callback() would not have
         *       released it, so we need to release it now.
         */
        filecache_handle->release(bc.offset, bc.length);
    }

    return get_write_error();
}

bool nfs_inode::release(fuse_req_t req)
{
    assert(opencnt > 0);
    if (--opencnt != 0 || !is_silly_renamed) {
        return false;
    }

    /*
     * Delete the silly rename file.
     * Note that we will now respond to fuse when the unlink completes.
     * The caller MUST arrange to *not* respond to fuse.
     * Silly rename is done only for regular files.
     */
    assert(!silly_renamed_name.empty());
    assert(parent_ino != 0);
    assert(is_regfile());

    AZLogInfo("Deleting silly renamed file, {}/{}",
              parent_ino, silly_renamed_name);

    client->unlink(req, parent_ino,
                   silly_renamed_name.c_str(), true /* for_silly_rename */);
    return true;
}

void nfs_inode::revalidate(bool force)
{
    /*
     * This is set in the constructor as a newly created nfs_inode always has
     * attributes cached in nfs_inode::attr.
     */
    assert(attr_timeout_timestamp != -1);

    const bool revalidate_now = force || attr_cache_expired();

    // Nothing to do, return.
    if (!revalidate_now) {
        AZLogDebug("revalidate_now is false");
        return;
    }

    /*
     * If the cache is empty we can save the GETATTR call below, as we have
     * nothing to invalidate even if GETATTR response suggests us to. This is
     * useful for fresh directory enumerations (common when running "find"
     * command) where these GETATTR RPCs add unwanted delay.
     */
    if (is_cache_empty()) {
        AZLogDebug("revalidate: Skipping as cache is empty!");
        return;
    }

    /*
     * Query the attributes of the file from the server to find out if
     * the file has changed and we need to invalidate the cached data.
     */
    struct fattr3 fattr;
    const bool ret = client->getattr_sync(get_fh(), get_fuse_ino(), fattr);

    /*
     * If we fail to query fresh attributes then we can't do much.
     * We don't update attr_timeout_timestamp so that next time we
     * retry querying the attributes again.
     */
    if (!ret) {
        AZLogWarn("Failed to query attributes for ino {}", ino);
        return;
    }

    /*
     * Let update() decide if the freshly received attributes indicate file
     * has changed that what we have cached, and if so update the cached
     * attributes and invalidate the cache as appropriate.
     */
    std::unique_lock<std::shared_mutex> lock(ilock_1);

    if (!update_nolock(&fattr)) {
        /*
         * File not changed, exponentially increase attr_timeout_secs.
         * File changed case is handled inside update_nolock() as that's
         * needed by other callsites of update_nolock().
         * We don't increase the attribute cache timeout for the forced
         * case as that can result in quick getattr calls and doesn't
         * necessarily mean that the attributes have not changed for the
         * entire attribute cache timeout period.
         */
        if (!force) {
            attr_timeout_secs =
                std::min((int) attr_timeout_secs*2, get_actimeo_max());
        }
        attr_timeout_timestamp = get_current_msecs() + attr_timeout_secs*1000;
    }
}

/**
 * Caller must hold exclusive inode lock.
 */
bool nfs_inode::update_nolock(const struct fattr3 *postattr,
                              const struct wcc_attr *preattr)
{
    /*
     * We must be called with at least one of preop or postop attributes.
     * Operations that do not change file/dir, they will only get postop
     * attributes from the server.
     * Update operations that change file/dir, they will get both postop and
     * preop attributes for success case and for failure cases they may not
     * get the postop attributes.
     */
    assert(preattr || postattr);

#ifdef ENABLE_PARANOID
    /*
     * XXX This assert has been seen to fail (for unlink).
     */
#if 0
    if (preattr && postattr) {
        /*
         * ctime cannot go back.
         */
        assert(compare_nfstime(postattr->ctime, preattr->ctime) >= 0);
    }
#endif
#endif

    /*
     * If postattr are present and they do not have a newer ctime than the
     * cached attributes, then our cache (both attributes and data if any) is
     * uptodate.
     */
    if (postattr) {
        const bool postattr_is_newer =
            (compare_timespec_and_nfstime(attr.st_ctim, postattr->ctime) == -1);

        if (!postattr_is_newer) {
            /*
             * Attributes haven't changed from the cached ones, refresh the
             * attribute cache timeout.
             */
            assert(attr_timeout_timestamp != -1);
            assert(attr_timeout_secs != -1);
            attr_timeout_timestamp =
                std::max(get_current_msecs() + attr_timeout_secs*1000,
                         attr_timeout_timestamp.load());
            return false;
        }
    }

    /*
     * Either postattr is not provided (rare) or postattr has a newer ctime
     * than the cached attributes. Latter could mean either file/dir data has
     * changed (in which case we need to invalidate our cached data) or just
     * the file/dir metadata has changed (in which case we don't invalidate the
     * cached data and just update the inode attributes).
     * For the "has file/dir data changed" check we use the preop attributes if
     * provided, else we use the postop attributes. Note that requests which
     * change file/dir will receive both preop and postop attributes from the
     * server and for such requests we need to check cached attributes against
     * the preop attributes to ignore changes done by the request itself. Other
     * requests which do not change file/dir only have the postop attributes for
     * this check.
     * Note that we consider file/dir data as changed when either the mtime or
     * the size changes.
     */
    const nfstime3 *pmtime = preattr ? &preattr->mtime : &postattr->mtime;
    const nfstime3 *pctime = preattr ? &preattr->ctime : &postattr->ctime;
    const size3    *psize  = preattr ? &preattr->size  : &postattr->size;
    const bool file_data_changed =
        ((compare_timespec_and_nfstime(attr.st_mtim, *pmtime) != 0) ||
         (attr.st_size != (off_t) *psize));

    /*
     * Update cached attributes and also reset the attr_timeout_secs and
     * attr_timeout_timestamp since the attributes have changed.
     */
    if (postattr) {
        AZLogDebug("[{}:{}] Got attributes newer than cached attributes, "
                   "ctime: {}.{} -> {}.{}, mtime: {}.{} -> {}.{}, "
                   "size: {} -> {}",
                   get_filetype_coding(), get_fuse_ino(),
                   attr.st_ctim.tv_sec, attr.st_ctim.tv_nsec,
                   postattr->ctime.seconds, postattr->ctime.nseconds,
                   attr.st_mtim.tv_sec, attr.st_mtim.tv_nsec,
                   postattr->mtime.seconds, postattr->mtime.nseconds,
                   attr.st_size, postattr->size);

        nfs_client::stat_from_fattr3(&attr, postattr);
        attr_timeout_secs = get_actimeo_min();
        attr_timeout_timestamp = get_current_msecs() + attr_timeout_secs*1000;

        // file type should not change.
        assert((attr.st_mode & S_IFMT) == file_type);
    }

    /*
     * Invalidate cache iff file data has changed.
     *
     * Note: This does not flush the dirty membufs, those will be flushed
     *       later when we decide to flush the cache. This means if some
     *       other client has written to the same parts of the file as
     *       this node, those will be overwritten when we flush our cache.
     *       This is not something unexpected as multiple writers updating
     *       a file w/o coordinating using file locks is expected to result
     *       in undefined results.
     *       This also means that if another client has truncated the file
     *       we will reduce the file size in our saved nfs_inode::attr.
     *       Later when we flush the dirty membufs the size will be updated
     *       if some of those membufs write past the file.
     *
     * Note: For the rare case where server doesn't provide postop attributes
     *       but only preop attributes, we might invalidate the cached data
     *       and not update the cached attributes. This would cause the next
     *       wcc data to also cause cache invalidation, untill we update the
     *       cached attributes. This should not be common case and in case
     *       it happens we will effectively run w/o attribute and data cache,
     *       which is safe.
     *       XXX We don't update ctime/mtime/size from preop attr even if they
     *           are more recent.
     */
    if (file_data_changed) {
        AZLogDebug("[{}:{}] {} changed at server, "
                   "ctime: {}.{} -> {}.{}, mtime: {}.{} -> {}.{}, "
                   "size: {} -> {}",
                   get_filetype_coding(), get_fuse_ino(),
                   is_dir() ? "Directory" : "File",
                   pctime->seconds, pctime->nseconds,
                   attr.st_ctim.tv_sec, attr.st_ctim.tv_nsec,
                   pmtime->seconds, pmtime->nseconds,
                   attr.st_mtim.tv_sec, attr.st_mtim.tv_nsec,
                   *psize, attr.st_size);

        invalidate_cache();
    }

    return true;
}

/*
 * Caller must hold exclusive lock on nfs_inode->ilock_1.
 */
void nfs_inode::force_update_attr_nolock(const struct fattr3& fattr)
{
    const bool fattr_is_newer =
        (compare_timespec_and_nfstime(attr.st_ctim, fattr.ctime) == -1);

    /*
     * Only update inode attributes if fattr is newer.
     * If not newer, don't update attr_timeout_timestamp as we would like
     * to query the server and find out what's going on.
     */
    if (!fattr_is_newer) {
        return;
    }

    /*
     * Update cached attributes and also reset the attr_timeout_secs and
     * attr_timeout_timestamp since the attributes have changed.
     */
    nfs_client::stat_from_fattr3(&attr, &fattr);
    attr_timeout_secs = get_actimeo_min();
    attr_timeout_timestamp = get_current_msecs() + attr_timeout_secs*1000;

    // file type should not change.
    assert((attr.st_mode & S_IFMT) == file_type);
}

/*
 * This will query the dir_entries map looking for upto 'max_size' entries
 * starting at 'cookie'.
 * The returned directory entries will be filled in 'results' vector.
 * If 'readdirplus' is true it means caller wants these entries for responding
 * to a READDIRPLUS request, which means all directory_entry returned will
 * have a valid nfs_inode pointer.
 * If 'readdirplus' is false it means caller wants these entries for responding
 * to a READDIR request, in that case directory_entry returned may or may not
 * have a valid nfs_inode pointer.
 * Every directory_entry returned that has a valid nfs_inode, a lookupcnt ref
 * will be held and also forget_expected will be increased for the inode. For
 * entries passed to fuse these will be dropped when fuse calls forget for those
 * inodes. For the rest, caller must arrange to drop both the lookupcnt and
 * forget_expected.
 */
void nfs_inode::lookup_dircache(
    cookie3 cookie,
    size_t max_size,
    std::vector<std::shared_ptr<const directory_entry>>& results,
    bool& eof,
    bool readdirplus)
{
    // Sanity check.
    assert(max_size > 0 && max_size <= (64*1024*1024));
    assert(results.empty());
    // Must be called only for a directory inode.
    assert(is_dir());
    // Must have been allocated in open()/opendir().
    assert(has_dircache());

#ifndef ENABLE_NON_AZURE_NFS
    // Blob NFS uses cookie as a counter, so 4B is a practical check.
    assert(cookie < UINT32_MAX);
#endif

    /*
     * Before looking up the cache check if we need to purge it.
     * We need to purge the cache in two cases:
     * 1. readdirectory_cache is marked lookuponly.
     * 2. readdirectory_cache has invalidate_pending set.
     *
     * Note that lookuponly readdir caches cannot be used to serve directory
     * enumeration requests as they are not in sync with the actual directory
     * content (one or more file/dir has been created/deleted since we last
     * enumerated and cachd the enumeration results).
     */
    dircache_handle->clear_if_needed();

    int num_cache_entries = 0;
    ssize_t rem_size = max_size;
    // Have we seen eof from the server?
    const bool dir_eof_seen = dircache_handle->get_eof();

    eof = false;

    while (rem_size > 0) {
        /*
         * lookup() will hold a dircachecnt ref on the inode if entry has a
         * valid nfs_inode. Also, there will one dircachecnt because of the
         * directory_entry being present in dir_entries map.
         */
        std::shared_ptr<struct directory_entry> entry =
            dircache_handle->lookup(cookie);

        /*
         * Cached entries stored by a prior READDIR call are not usable
         * for READDIRPLUS as they won't have the attributes saved, treat
         * them as not present.
         */
        if (entry && readdirplus && !entry->nfs_inode) {
            entry = nullptr;
        }

        if (entry) {
            /*
             * Get the size this entry will take when copied to fuse buffer.
             * The size is more for readdirplus, which copies the attributes
             * too. This way we make sure we don't return more than what fuse
             * readdir/readdirplus call requested.
             */
            rem_size -= entry->get_fuse_buf_size(readdirplus);

            if (rem_size >= 0) {
                /*
                 * This entry can fit in the fuse buffer. If entry->nfs_inode
                 * is valid then increase the inode lookupcnt ref and also the
                 * forget_expected. Note that we do it regardless of whether
                 * the caller wants it for READDIR or READDIRPLUS. Caller must
                 * drop the lookupcnt ref and forget_expected correctly.
                 */
                if (entry->nfs_inode) {
                    /*
                     * lookup() would have held a dircachecnt ref and one
                     * original dircachecnt ref held for each directory_entry
                     * added to dir_entries.
                     */
                    entry->nfs_inode->forget_expected++;
                    entry->nfs_inode->incref();
                    assert(entry->nfs_inode->dircachecnt >= 2);
                    entry->nfs_inode->dircachecnt--;
                }

                num_cache_entries++;
                results.push_back(entry);

                /*
                 * We must convey eof to caller only after we successfully copy
                 * the directory entry with eof_cookie.
                 */
                if (dir_eof_seen &&
                    (entry->cookie == dircache_handle->get_eof_cookie())) {
                    eof = true;
                }
            } else {
                /*
                 * Drop the ref taken inside readdirectory_cache::lookup().
                 * Note that we should have 2 or more dircachecnt references,
                 * one taken by lookup() for the directory_entry copy returned
                 * to us and one already taken as the directory_entry is added
                 * to readdirectory_cache::dir_entries.
                 * Also note that this readdirectory_cache won't be purged,
                 * after lookup() releases readdircache_lock_2 since this dir
                 * is being enumerated by the current thread and hence it must
                 * have the directory open which should prevent fuse vfs from
                 * calling forget on the directory inode.
                 *
                 * Note: entry->nfs_inode may be null for entries populated using
                 *       only readdir however, it is guaranteed to be present for
                 *       readdirplus.
                 */
                if (entry->nfs_inode) {
                    struct nfs_inode *inode = entry->nfs_inode;
                    inode->incref();
                    assert(inode->dircachecnt >= 2);
                    inode->dircachecnt--;
                    entry.reset();
                    inode->decref();
                }

                // No space left to add more entries.
                AZLogDebug("lookup_dircache: Returning {} entries, as {} bytes "
                           "of output buffer exhausted (eof={})",
                           num_cache_entries, max_size, eof);
                break;
            }

            /*
             * TODO: ENABLE_NON_AZURE_NFS alert!!
             *       Note that we assume sequentially increasing cookies.
             *       This is only true for Azure NFS. Linux NFS server
             *       also has sequentially increasing cookies but it
             *       sometimes have gaps in between which causes us to
             *       believe that we don't have the cookie and re-fetch
             *       it from the server.
             */
            cookie++;
        } else {
            /*
             * Call after we return the last cookie, comes here.
             */
            if (dir_eof_seen && (cookie >= dircache_handle->get_eof_cookie())) {
                eof = true;
            }

            AZLogDebug("lookup_dircache: Returning {} entries, as next "
                       "cookie {} not found in cache (eof={})",
                       num_cache_entries, cookie, eof);

            /*
             * If we don't find the current cookie, then we will not find the
             * next ones as well since they are stored sequentially.
             */
            break;
        }
    }
}
