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
    // Sanity assert.
    assert(client != nullptr);
    assert(client->magic == NFS_CLIENT_MAGIC);
    assert(filehandle->data.data_len > 50 &&
            filehandle->data.data_len <= 64);
    // ino is either set to FUSE_ROOT_ID or set to address of nfs_inode.
    assert((ino == (fuse_ino_t) this) || (ino == FUSE_ROOT_ID));

    // Blob NFS supports only these file types.
    assert((file_type == S_IFREG) ||
           (file_type == S_IFDIR) ||
           (file_type == S_IFLNK));

    fh.data.data_len = filehandle->data.data_len;
    fh.data.data_val = new char[fh.data.data_len];
    ::memcpy(fh.data.data_val, filehandle->data.data_val, fh.data.data_len);

    /*
     * Most common case is we are creating nfs_inode when we got a fh (and
     * attributes) for a file, f.e., LOOKUP, CREATE, READDIRPLUS, etc.
     */
    if (fattr) {
        client->stat_from_fattr3(&attr, fattr);

        // file type as per fattr should match the one passed explicitly..
        assert((attr.st_mode & S_IFMT) == file_type);

        attr_timeout_secs = get_actimeo_min();
        attr_timeout_timestamp = get_current_msecs() + attr_timeout_secs*1000;
    } else {
        /*
         * Just init enough to treat cached attributes as invalid.
         * This is still not usable, as the least we need from attributes
         * is st_ino (for inode) and st_mode (for filetype).
         */
        attr.st_ctim = {0, 0};
        attr.st_mtim = {0, 0};
    }

    dircache_handle = std::make_shared<readdirectory_cache>();
}

nfs_inode::~nfs_inode()
{
    assert(lookupcnt == 0);
    assert(fh.data.data_len > 50 && fh.data.data_len <= 64);
    assert((ino == (fuse_ino_t) this) || (ino == FUSE_ROOT_ID));
    assert(client != nullptr);
    assert(client->magic == NFS_CLIENT_MAGIC);

    delete fh.data.data_val;
    fh.data.data_val = nullptr;
    fh.data.data_len = 0;
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
        purge_dircache();
    } else {
        purge_filecache();
    }
}

/*
 * Purge the readdir cache.
 *  TODO: For now we purge the entire cache. This can be later changed to purge
 *        parts of cache.
 */
void nfs_inode::purge_dircache()
{
    dircache_handle->clear();
}

void nfs_inode::lookup_dircache(
    cookie3 cookie,
    size_t max_size,
    std::vector<const directory_entry*>& results,
    bool& eof,
    bool readdirplus)
{
    int num_cache_entries = 0;
    ssize_t rem_size = max_size;
    // Have we seen eof from the server?
    const bool dir_eof_seen = dircache_handle->get_eof();

    // We should have non-zero space to fill in entries.
    assert(rem_size > 0);

    while (rem_size > 0) {
        const struct directory_entry* entry = dircache_handle->lookup(cookie);

        if (entry) {
            rem_size -= entry->get_fuse_buf_size(readdirplus);

            if (rem_size >= 0) {
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
                // No space left to add more entries.
                AZLogDebug("lookup_dircache: Returning {} entries, as {} bytes "
                           "of output buffer exhausted (eof={})",
                           num_cache_entries, max_size, eof);
                break;
            }

            cookie++;
        } else {
            /*
             * Call after we return the last cookie, comes here.
             */
            if (dir_eof_seen && (cookie >= dircache_handle->get_eof_cookie())) {
                eof = true;
            }

            AZLogDebug("lookup_dircache: Returning {} entries, as next cookie {} "
                       "not found in cache (eof={})",
                       num_cache_entries, cookie, eof);

            /*
             * If we don't find the current cookie, then we will not find the
             * next ones as well since they are stored sequentially.
             */
            break;
        }
    }
}
