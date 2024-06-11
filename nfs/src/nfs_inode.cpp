#ifndef __NFS_INODE__
#define __NFS_INODE__

#include "nfs_inode.h"

bool nfs_inode::purge_readdircache_if_required()
{
    struct directory_entry* dirent = nullptr;
    bool success = false;
    struct stat attr;

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
        success = readdirectory_cache::make_getattr_call(ino, attr);
        // Issue a getattr call to the server to fetch the directory mtime.
    }

    if (success)
    {
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

#endif /* __NFS_INODE__ */
