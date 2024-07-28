#include "nfs_inode.h"
#include "nfs_client.h"

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
    file_type(_file_type),
    client(_client)
{
    // Sanity asserts.
    assert(magic == NFS_INODE_MAGIC);
    assert(filehandle != nullptr);
    assert(fattr != nullptr);
    assert(client != nullptr);
    assert(client->magic == NFS_CLIENT_MAGIC);

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
     * We always have fattr when creating nfs_inode.
     * Most common case is we are creating nfs_inode when we got a fh (and
     * attributes) for a file, f.e., LOOKUP, CREATE, READDIRPLUS, etc.
     */
    client->stat_from_fattr3(&attr, fattr);

    // file type as per fattr should match the one passed explicitly..
    assert((attr.st_mode & S_IFMT) == file_type);

    attr_timeout_secs = get_actimeo_min();
    attr_timeout_timestamp = get_current_msecs() + attr_timeout_secs*1000;

    if (is_regfile()) {
        if (aznfsc_cfg.cachedir != nullptr) {
            const std::string backing_file_name =
                std::string(aznfsc_cfg.cachedir) + "/" + std::to_string(ino);
            filecache_handle =
                std::make_shared<bytes_chunk_cache>(backing_file_name.c_str());
        } else {
            filecache_handle = std::make_shared<bytes_chunk_cache>();
        }
        readahead_state = std::make_shared<ra_state>(client, this);
    } else if (is_dir()) {
        dircache_handle = std::make_shared<readdirectory_cache>(client, this);
    }
}

nfs_inode::~nfs_inode()
{
    assert(magic == NFS_INODE_MAGIC);
    // We should never delete an inode which fuse still has a reference on.
    assert(is_forgotten());
    assert(lookupcnt == 0);
    /*
     * We should never delete an inode while it is still referred by parent
     * dir cache.
     */
    assert(dircachecnt == 0);
    /*
     * Directory inodes must not be freed while they have a non-empty dir
     * cache.
     */
    if (is_dir()) {
        assert(dircache_handle);
        assert(dircache_handle->get_num_entries() == 0);
    } else if (is_regfile()) {
        assert(filecache_handle);
        assert(filecache_handle->is_empty());
    }

#ifndef ENABLE_NON_AZURE_NFS
    assert(fh.data.data_len > 50 && fh.data.data_len <= 64);
#else
    assert(fh.data.data_len <= 64);
#endif
    assert((ino == (fuse_ino_t) this) || (ino == FUSE_ROOT_ID));
    assert(client != nullptr);
    assert(client->magic == NFS_CLIENT_MAGIC);

    assert(fh.data.data_val != nullptr);
    delete fh.data.data_val;
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
        if (from_forget) {
            if (is_dir()) {
                purge_dircache();
            } else if (is_regfile()) {
                purge_filecache();
            }
        }

        /*
         * Reduce the extra refcnt and revert the cnt.
         * After this the inode will have 'cnt' references that need to be
         * dropped by put_nfs_inode() call below, with inode_map_lock held.
         */
        lookupcnt += (cnt - 1);

        AZLogDebug("[{}] lookupcnt dropping({}) to 0, forgetting inode",
                   ino, cnt);

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

void nfs_inode::revalidate(bool force)
{
    const int64_t now_msecs = get_current_msecs();
    /*
     * If attributes are currently not cached in nfs_inode::attr then
     * attr_timeout_timestamp will be -1 which will force revalidation.
     */
    const bool revalidate_now = force || (attr_timeout_timestamp < now_msecs);

    // Nothing to do, return.
    if (!revalidate_now) {
        AZLogDebug("revalidate_now is false");
        return;
    }

    /*
     * Query the attributes of the file from the server to find out if
     * the file has changed and we need to invalidate the cached data.
     */
    struct fattr3 fattr;
    const bool ret = client->getattr_sync(get_fh(), fattr);

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
         */
        attr_timeout_secs =
            std::min((int) attr_timeout_secs*2, get_actimeo_max());
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
     * TODO: Right now we just purge the readdir cache.
     *       Once we have the file cache too, we need to purge that for files.
     */
    if (is_dir()) {
        purge_dircache_nolock();
    } else {
        purge_filecache();
    }
}

/*
 * Purge the readdir cache.
 * TODO: For now we purge the entire cache. This can be later changed to purge
 *       parts of cache.
 */
void nfs_inode::purge_dircache_nolock()
{
    AZLogWarn("[{}] Purging dircache", get_fuse_ino());

    dircache_handle->clear();
}

/*
 * Purge the file cache.
 * TODO: For now we purge the entire cache. This can be later changed to purge
 *       parts of cache.
 */
void nfs_inode::purge_filecache_nolock()
{
    AZLogWarn("[{}] Purging filecache", get_fuse_ino());

    filecache_handle->clear();
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
                 */
                if (readdirplus) {
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
