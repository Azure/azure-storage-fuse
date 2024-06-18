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

    // Just init enough to treat cached attributes as invalid.
    attr.st_ctim = {0, 0};
    attr.st_mtim = {0, 0};

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
    const bool ret = make_getattr_call(fattr);

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

    if (update_nolock(fattr)) {
        // File changed.
        attr_timeout_secs = get_actimeo_min();
    } else {
        // File not changed.
        attr_timeout_secs =
            std::min((int) attr_timeout_secs*2, get_actimeo_max());
    }

    attr_timeout_timestamp = now_msecs + attr_timeout_secs*1000;
}

/**
 * Caller must hold exclusive inode lock.
 */
bool nfs_inode::update_nolock(const struct fattr3& fattr)
{
    const bool fattr_is_newer =
        (compare_timespec_and_nfstime(attr.st_ctim, fattr.ctime) == -1);

    // ctime has not increased, i.e., cached attributes are newer, skip update.
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

    // Update cached attributes.
    get_client()->stat_from_fattr3(&attr, &fattr);

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

void nfs_inode::lookup_readdircache(
    cookie3 cookie_,
    size_t max_size,
    std::vector<directory_entry* >& results,
    bool& eof,
    bool skip_attr_size)
{
    int num_of_entries_found_in_cache = 0;

    size_t rem_size = max_size;
    while (rem_size > 0)
    {
        struct directory_entry* entry;
        bool found = dircache_handle->lookup(cookie_, &entry);
        if (found)
        {
            const size_t curr_entry_size = entry->get_size(skip_attr_size);
            if (rem_size >= curr_entry_size)
            {
                num_of_entries_found_in_cache++;

                results.push_back(entry);
            }
            else
            {
                // We have populated the maximum entries requested, hence break.
                break;
            }
            rem_size -= curr_entry_size;
            cookie_++;
        }
        else
        {
            AZLogDebug("Traversed map: Num of entries returned from cache {}", num_of_entries_found_in_cache);

            /*
             * If we don't find the current cookie, then we will not find the next
             * ones as well since they are stored sequentially.
             */
            return;
        }
    }
    eof = dircache_handle->get_eof();
    AZLogDebug("Buffer exhaust: Num of entries returned from cache {}", num_of_entries_found_in_cache);
}

// TODO: Add comments.
struct getattr_context
{
    nfs_inode* ino_ptr;
    struct fattr3* attr;
    bool callback_called;
    bool is_callback_success;
    std::mutex ctx_mutex;
    std::condition_variable cv;

    getattr_context(nfs_inode *ino_ptr_, struct fattr3 *attr_):
        ino_ptr(ino_ptr_),
        attr(attr_),
        callback_called(false),
        is_callback_success(false)
    {}
};

static void getattr_callback(
    struct rpc_context* /* rpc */,
    int rpc_status,
    void* data,
    void* private_data)
{
    auto ctx = (struct getattr_context*)private_data;
    auto res = (GETATTR3res*)data;

    if (res && (rpc_status == RPC_STATUS_SUCCESS) && (res->status == NFS3_OK))
    {
        *ctx->attr = res->GETATTR3res_u.resok.obj_attributes;
        ctx->is_callback_success = true;
    }
    ctx->callback_called = true;
    ctx->cv.notify_one();
}

bool nfs_inode::make_getattr_call(struct fattr3& attr)
{
    // TODO:Make sync getattr call once libnfs adds support.

    bool rpc_retry = false;

    struct getattr_context* ctx = new getattr_context(this, &attr);


    do {
        struct GETATTR3args args;
        ::memset(&args, 0, sizeof(args));
        args.object = get_client()->get_nfs_inode_from_ino(ino)->get_fh();

        if (rpc_nfs3_getattr_task(nfs_get_rpc_context(client->get_nfs_context()), getattr_callback, &args, ctx) == NULL)
        {
            /*
             * This call fails due to internal issues like OOM etc
             * and not due to an actual error, hence retry.
             */
            rpc_retry = true;
        }
    } while (rpc_retry);

    std::unique_lock<std::mutex> lock(ctx->ctx_mutex);
    ctx->cv.wait(lock, [&ctx] { return ctx->callback_called; } );

    const bool success = ctx->is_callback_success;
    delete ctx;

    return success;
}
