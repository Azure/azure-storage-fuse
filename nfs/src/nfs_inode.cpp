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
    ino(_ino == 0 ? (fuse_ino_t) this : _ino),
    generation(get_current_usecs()),
    file_type(_file_type),
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
    // Blob NFS FH is at least 50 bytes.
    assert(filehandle->data.data_len > 50 &&
           filehandle->data.data_len <= 64);
    // Blob NFS supports only these file types.
    assert((file_type == S_IFREG) ||
           (file_type == S_IFDIR) ||
           (file_type == S_IFLNK));

#else
    assert(filehandle->data.data_len <= 64);
#endif

    // ino is either set to FUSE_ROOT_ID or set to address of nfs_inode.
    assert((ino == (fuse_ino_t) this) || (ino == FUSE_ROOT_ID));

    fh.data.data_len = filehandle->data.data_len;
    fh.data.data_val = new char[fh.data.data_len];
    ::memcpy(fh.data.data_val, filehandle->data.data_val, fh.data.data_len);

    /*
     * Calculate and store the CRC32 hash of the filehandle.
     * This serves multiple purposes, most importantly it can be used to print
     * filehandle hashes in a way that can be used to match with wireshark.
     */
    crc = calculate_crc32((const unsigned char*) fh.data.data_val,
                          fh.data.data_len);

    /*
     * We always have fattr when creating nfs_inode.
     * Most common case is we are creating nfs_inode when we got a fh (and
     * attributes) for a file, f.e., LOOKUP, CREATE, READDIRPLUS, etc.
     */
    client->stat_from_fattr3(&attr, fattr);

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
    assert(!dircache_handle);
    assert(!readahead_state);

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
    assert(is_cache_empty());

#ifndef ENABLE_NON_AZURE_NFS
    assert(fh.data.data_len > 50 && fh.data.data_len <= 64);
#else
    assert(fh.data.data_len <= 64);
#endif
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

    assert(fh.data.data_val != nullptr);
    delete[] fh.data.data_val;
    fh.data.data_val = nullptr;
    fh.data.data_len = 0;
}

void nfs_inode::decref(size_t cnt, bool from_forget)
{
    AZLogDebug("[{}] decref(cnt={}, from_forget={}) called "
               " (lookupcnt={}, dircachecnt={})",
               ino, cnt, from_forget,
               lookupcnt.load(), dircachecnt.load());

    /*
     * We only decrement lookupcnt in forget and once lookupcnt drops to
     * 0 we mark the inode as forgotten, so decref() should not be called
     * for forgotten inode.
     */
    assert(!is_forgotten());
    assert(cnt > 0);
    assert(lookupcnt >= cnt);

    if (from_forget) {
        /*
         * Fuse should not call forget more than once for an inode.
         */
        assert(!forget_seen);
        forget_seen = true;
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
         */
        if (is_dir()) {
            purge_dircache();
        } else if (is_regfile()) {
            purge_filecache();
        }

        /*
         * Reduce the extra refcnt and revert the cnt.
         * After this the inode will have 'cnt' references that need to be
         * dropped by put_nfs_inode() call below, with inode_map_lock held.
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
            AZLogDebug("[{}] lookupcnt dropping({}) to 0, forgetting inode",
                       ino, cnt);
        } else {
            AZLogWarn("[{}] lookupcnt dropping({}) to {} "
                      "(some other thread got a fresh ref)",
                      ino, cnt, lookupcnt - cnt);
        }

        /*
         * This FORGET would drop the lookupcnt to 0, fuse vfs should not send
         * any more forgets, delete the inode. Note that before we grab the
         * inode_map_lock in put_nfs_inode() some other thread can reuse the
         * forgotten inode, in which case put_nfs_inode() will just skip it.
         *
         * TODO: In order to avoid taking the inode_map_lock for every forget,
         *       see if we should batch them in a threadlocal vector and call
         *       put_nfs_inodes() for a batch.
         */
        client->put_nfs_inode(this, cnt);
    } else {
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

        AZLogDebug("[{}] lookupcnt decremented({}) to {}",
                   ino, cnt, lookupcnt.load());
    }
}

bool nfs_inode::in_ra_window(uint64_t offset, uint64_t length) const
{
    if (!readahead_state) {
        return false;
    }

    return readahead_state->in_ra_window(offset, length);
}

struct nfs_inode *nfs_inode::lookup(const char *filename)
{
    // Must be called only for a directory inode.
    assert(is_dir());

    // Revalidate to ensure dnlc cache can be safely used.
    revalidate();

    struct nfs_client *client = get_client();
    fuse_ino_t child_ino = 0;

    /*
     * First search in dnlc, if not found perform LOOKUP RPC.
     */
    {
        std::unique_lock<std::shared_mutex> lock(ilock);
        auto it = dnlc.find(filename);
        if (it != dnlc.end()) {
            child_ino = it->second;
            assert(child_ino != 0);
            AZLogDebug("{}/{} -> {}, found in DNLC!",
                       get_fuse_ino(), filename, child_ino);
        }
    }

    if (child_ino == 0) {
       if (!client->lookup_sync(get_fuse_ino(), filename, child_ino)) {
           AZLogDebug("{}/{}, sync LOOKUP failed!",
                      get_fuse_ino(), filename);
           return nullptr;
       }
       AZLogDebug("{}/{} -> {}, found via sync LOOKUP!",
                  get_fuse_ino(), filename, child_ino);
       assert(child_ino != 0);
    }

    return client->get_nfs_inode_from_ino(child_ino);
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

int nfs_inode::flush_cache_and_wait()
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
    if (!filecache_handle) {
        return 0;
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
        flush_task->init_flush(nullptr /* fuse_req */, ino);

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
        if (mb->is_dirty() && get_write_error()) {
            AZLogError("[{}] Flush [{}, {}) failed with error: {}",
                       ino,
                       bc.offset, bc.offset + bc.length,
                       get_write_error());
        }

        mb->clear_locked();
        mb->clear_inuse();
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
     */
    assert(!silly_renamed_name.empty());
    assert(parent_ino != 0);

    AZLogInfo("Deleting silly renamed file, {}/{}",
              parent_ino, silly_renamed_name);

    client->unlink(req, parent_ino, silly_renamed_name.c_str());
    return true;
}

void nfs_inode::revalidate(bool force)
{
    /*
     * This is set in the constructor as a newly created nfs_inode always has
     * attributes cached in nfs_inode::attr.
     */
    assert(attr_timeout_timestamp != -1);

    const int64_t now_msecs = get_current_msecs();
    const bool revalidate_now = force || (attr_timeout_timestamp < now_msecs);

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
    if (is_cache_empty() && is_dnlc_empty()) {
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
    std::unique_lock<std::shared_mutex> lock(ilock);

    if (!update_nolock(fattr)) {
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
        attr_timeout_timestamp = now_msecs + attr_timeout_secs*1000;
    }
}

/**
 * Caller must hold exclusive inode lock.
 */
bool nfs_inode::update_nolock(const struct fattr3& fattr)
{
    const bool fattr_is_newer =
        (compare_timespec_and_nfstime(attr.st_ctim, fattr.ctime) == -1);

    // ctime has not increased, i.e., cached attributes are valid, skip update.
    if (!fattr_is_newer) {
        return false;
    }

    /*
     * We consider file data as changed when either the mtime or the size
     * changes.
     */
    const bool file_data_changed =
        ((compare_timespec_and_nfstime(attr.st_mtim, fattr.mtime) != 0) ||
         (attr.st_size != (off_t) fattr.size));

    AZLogDebug("Got attributes newer than cached attributes, "
               "ctime: {}.{} -> {}.{}, mtime: {}.{} -> {}.{}, size: {} -> {}",
               attr.st_ctim.tv_sec, attr.st_ctim.tv_nsec,
               fattr.ctime.seconds, fattr.ctime.nseconds,
               attr.st_mtim.tv_sec, attr.st_mtim.tv_nsec,
               fattr.mtime.seconds, fattr.mtime.nseconds,
               attr.st_size, fattr.size);

    /*
     * Update cached attributes and also reset the attr_timeout_secs and
     * attr_timeout_timestamp since the attributes have changed.
     */
    get_client()->stat_from_fattr3(&attr, &fattr);
    attr_timeout_secs = get_actimeo_min();
    attr_timeout_timestamp = get_current_msecs() + attr_timeout_secs*1000;

    // file type should not change.
    assert((attr.st_mode & S_IFMT) == file_type);

    // Invalidate cache iff file data has changed.
    if (file_data_changed) {
        invalidate_cache_nolock();
    }

    return true;
}

/**
 * Caller must hold exclusive inode lock.
 */
void nfs_inode::invalidate_cache_nolock()
{
    /*
     * When directory mtime changes then we purge the readdir cache for that
     * directory and also the DNLC cache for that directory. DNLC cache needs
     * to be purged as directory contents changing could mean any existing
     * file may have been deleted.
     */
    if (is_dir()) {
        purge_dircache_nolock();
        purge_dnlc_nolock();
    } else if (is_regfile()) {
        assert(is_dnlc_empty());
        purge_filecache_nolock();
    }
}

/*
 * Purge the readdir cache.
 * TODO: For now we purge the entire cache. This can be later changed to purge
 *       parts of cache.
 */
void nfs_inode::purge_dircache_nolock()
{
    if (dircache_handle) {
        AZLogWarn("[{}] Purging dircache", get_fuse_ino());
        dircache_handle->clear();
    }
}

void nfs_inode::purge_dnlc_nolock()
{
    if (!dnlc.empty()) {
        AZLogWarn("[{}] Purging dnlc", get_fuse_ino());
        dnlc.clear();
    }
}

/*
 * Purge the file cache.
 * TODO: For now we purge the entire cache. This can be later changed to purge
 *       parts of cache.
 */
void nfs_inode::purge_filecache_nolock()
{
    if (filecache_handle) {
        AZLogWarn("[{}] Purging filecache", get_fuse_ino());
        filecache_handle->clear();
    }
}

void nfs_inode::lookup_dircache(
    cookie3 cookie,
    size_t max_size,
    std::vector<const directory_entry*>& results,
    bool& eof,
    bool readdirplus)
{
    // Sanity check.
    assert(max_size > 0 && max_size <= (64*1024*1024));
    assert(results.empty());
    // Must have been allocated in open()/opendir().
    assert(dircache_handle);

#ifndef ENABLE_NON_AZURE_NFS
    // Blob NFS uses cookie as a counter, so 4B is a practical check.
    assert(cookie < UINT32_MAX);
#endif

    int num_cache_entries = 0;
    ssize_t rem_size = max_size;
    // Have we seen eof from the server?
    const bool dir_eof_seen = dircache_handle->get_eof();

    eof = false;

    while (rem_size > 0) {
        const struct directory_entry *entry = dircache_handle->lookup(cookie);

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
                 * This entry can fit in the fuse buffer.
                 * We have to increment the lookupcnt for non "." and ".."
                 * entries. Note that we took a dircachecnt reference inside
                 * readdirectory_cache::lookup() call above, to make sure that
                 * till we increase this refcnt, the inode is not freed.
                 */
                if (readdirplus && !entry->is_dot_or_dotdot()) {
                    entry->nfs_inode->incref();
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
                 * after lookup() releases the readdircache_lock since this dir
                 * is being enumerate by the current thread and hence it must
                 * have the directory open which should prevent fuse vfs from
                 * calling forget on the directory inode.
                 *
                 * Note: entry->nfs_inode may be null for entries populated using
                 *       only readdir however, it is guaranteed to be present for
                 *       readdirplus.
                 */
                if (entry->nfs_inode) {
                    assert(entry->nfs_inode->dircachecnt >= 2);
                    entry->nfs_inode->dircachecnt--;
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
