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
                     fuse_ino_t _ino) :
    ino(_ino == 0 ? (fuse_ino_t) this : _ino),
    client(_client)
{
    // Sanity assert.
    assert(client != nullptr);
    assert(client->magic == NFS_CLIENT_MAGIC);
    assert(filehandle->data.data_len > 50 &&
            filehandle->data.data_len <= 64);
    // ino is either set to FUSE_ROOT_ID or set to address of nfs_inode.
    assert((ino == (fuse_ino_t) this) || (ino == FUSE_ROOT_ID));

    fh.data.data_len = filehandle->data.data_len;
    fh.data.data_val = new char[fh.data.data_len];
    ::memcpy(fh.data.data_val, filehandle->data.data_val, fh.data.data_len);

    dircache_handle = std::make_shared<readdirectory_cache>();
}

nfs_inode::~nfs_inode()
{
    assert(fh.data.data_len > 50 && fh.data.data_len <= 64);
    assert((ino == (fuse_ino_t) this) || (ino == FUSE_ROOT_ID));
    assert(client != nullptr);
    assert(client->magic == NFS_CLIENT_MAGIC);

    delete fh.data.data_val;
    fh.data.data_val = nullptr;
    fh.data.data_len = 0;
}
bool nfs_inode::purge_readdircache_if_required()
{
    struct directory_entry* dirent = nullptr;
    bool success = false;
    struct stat attr;
    struct fattr3 fattribute;

    /*
     * For now the only prune condition check is the directory mtime.
     * This function can later be extended to add more conditions.
     */
    bool should_update_mtime = false;
    {
        bool found = dircache_handle->get_entry_at(1 /* cookie*/, &dirent); // cookie 1 represent '.', hence this gets us cached dir mtime.
        if (!found)
        {
            /*
             * cookie = 1 refers to the current directory '.'.
             * This entry should never be purged from the cache, else we will not know the cached
             * directory mtime.
             * If we encounter such a state, just purge the complete cache and proceed.
             */
            goto purge_cache;
        }

        const std::time_t now = std::time(nullptr);
        const std::time_t last_mtimecheck = dircache_handle->get_lastmtime();
        assert (now >= last_mtimecheck);
        if (std::difftime(now, last_mtimecheck) > 30) /*sec. TODO: Add #define for this */
        {
            should_update_mtime = true;
        }
        else
        {
            // No need to do any mtime check, just return.
            return false;
        }
    }

    if (should_update_mtime)
    {
        success = make_getattr_call(fattribute);
        // Issue a getattr call to the server to fetch the directory mtime.
    }

    if (success)
    {
        get_client()->stat_from_fattr3(&attr, &fattribute);
        if (!readdirectory_cache::are_mtimes_equal(&dirent->attributes, &attr))
        {
            /*
             * Purge the cache since the directory mtime has changed/
             * This indicates that the directory has changed.
             */
            goto purge_cache;
        }

        // Update the mtime.
        dircache_handle->set_mtime();
        return false;
    }
    else
    {
        // TODO: What should we do if getattr call fails?
        return false;
    }

purge_cache:
    purge();
    return true;
}

/*
 * Purge the readdir cache.
 *  TODO: For now we purge the entire cache. This can be later changed to purge
 *        parts of cache.
 */
void nfs_inode::purge()
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
    // Check if the cache should be purged.
    purge_readdircache_if_required();

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
