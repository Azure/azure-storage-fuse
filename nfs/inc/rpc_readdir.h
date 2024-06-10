#ifndef __READDIR_RPC_TASK__
#define __READDIR_RPC_TASK__

#include "aznfsc.h"
#include <map>
#include <shared_mutex>
#include <vector>
#include <ctime>

// 1GB
#define MAX_CACHE_SIZE_LIMIT 1073741824

//struct directory_list_cache;
//struct directory_entry;

struct directory_entry
{
#if 0
    cookie3 cookie;
    struct stat attributes;
    nfs_inode* nfs_ino;
    char* name;
#endif
    const cookie3 cookie;
    const struct stat attributes;
    const nfs_inode* const nfs_ino;
    const char* const name;

    directory_entry(const char* name_, cookie3 cookie_, struct stat attr, nfs_inode* nfs_ino_):
        cookie(cookie_),
        attributes(attr),
        nfs_ino(nfs_ino_),
        name(name_)
    {}

// Get the size of the directory entry
    size_t get_size(bool skip_attr_size = false)
    {
        if (skip_attr_size)
        {
            return (strlen(name) + offsetof(struct directory_entry, name) - sizeof(struct stat));
        }
        else
        {
            return (strlen(name) + offsetof(struct directory_entry, name));
        }
    }
};

struct directory_list_cache
{
private:
    const fuse_ino_t inode;
    
    /*
     * This will be set if we have read all the entries of the directory
     * from the backend.
     */
    bool eof;

    size_t cache_size;

    const std::time_t create_time;

    // The time at which we checked the directory mtime by making getattr call.
    std::time_t last_mtimecheck;
    
    cookieverf3 cookie_verifier;
    
    // TODO: See if we can just make it a vector and can be indexed by cookie.
    std::map<cookie3, struct directory_entry*> dir_entries;

    std::shared_mutex lock_dirlistcache;

public:
    directory_list_cache(fuse_ino_t ino):
        inode(ino),
        eof(false),
        cache_size(0),
        create_time(std::time(nullptr)),
        last_mtimecheck(std::time(nullptr))
    {
        assert(dir_entries.empty());
        ::memset(&cookie_verifier, 0, sizeof(cookie_verifier));
    }

    std::shared_mutex& get_lock()
    {
        return lock_dirlistcache;
    }

    /*
     * Return true and populates the \p dirent if the entry corresponding to \p cookie exists.
     * Returns false otherwise.
     */
    bool get_entry_at(cookie3 cookie, struct directory_entry** dirent)
    {
        // Take shared lock on the map.
        std::shared_lock<std::shared_mutex> lock(lock_dirlistcache);
        auto it = dir_entries.find(cookie);

        if (it != dir_entries.end())
        {
            *dirent = it->second;
            return true;
        }

        return false;
    }

    /*
     * Updates the last_mtimecheck to now.
     */
    void set_mtime()
    {
        // TODO: Put this behind a lock or make it atomic.
        last_mtimecheck = std::time(nullptr);
    }

    std::time_t get_lastmtime()
    {
        // TODO: Make this atomic or put it under lock.
        return last_mtimecheck;
    }

    bool add(struct directory_entry* entry)
    {
        const size_t entry_size = entry->get_size();

        assert(entry != nullptr);
        // if (get_cache_size() < MAX_ALLOWED_SIZE) // TODO: Add this check later.
        {
            // Get exclusive lock on the map to add the entry to the map.
            std::unique_lock<std::shared_mutex> lock(lock_dirlistcache);
            if (cache_size >= MAX_CACHE_SIZE_LIMIT)
            {
                AZLogWarn("Exceeding cache max size. No more entries will be added to the cache! cuurent size: {}", cache_size); 
                return false;
            }

            const auto& it = dir_entries.insert({entry->cookie, entry});
            cache_size += entry_size;
            return it.second;
        }

        /*
         * TODO: Prune the map for space constraint.
         * For now we will just not add entry into the cache if it is full.
         */
        return false;
    }

    const cookieverf3* get_cookieverf() const
    {
        return &cookie_verifier;
    }

    bool get_eof() const
    {
        return eof;
    }

    void set_cookieverf(const cookieverf3* cokieverf)
    {
        assert(cokieverf != nullptr);

        // TODO: Can this be made atomic? Get exclusive lock to update the cookie verifier.
        std::unique_lock<std::shared_mutex> lock(lock_dirlistcache);

        ::memcpy(&cookie_verifier, cokieverf, sizeof(cookie_verifier));
    }

    void set_eof()
    {
         AZLogInfo("set eof called");
        eof = true;
    }

    bool lookup(cookie3 cookie_, struct directory_entry** entry)
    {
        *entry = nullptr;

        // Take shared look to see if the entry exist in the cache.
        std::shared_lock<std::shared_mutex> lock(lock_dirlistcache);

        auto it = dir_entries.find(cookie_);

        if (it != dir_entries.end())
        {
            // TODO: See if we need to update the last access time.
            *entry = it->second;
            return true;
        }

        return false;
    }

    ~directory_list_cache()
    {
        AZLogInfo(" ~directory_list_cache() called");
        // This cache has now become invalid. clean it up.
        std::unique_lock<std::shared_mutex> lock(lock_dirlistcache);

        for (auto it = dir_entries.begin(); it != dir_entries.end(); ++it)
        {
            free(it->second);
        }

        dir_entries.clear();
    }
};

static bool are_mtimes_equal(const struct stat* attr1, const struct stat* attr2)
{
// #TODO : Added only for testing. Remove this!!
return true;
        static int i =0;
        if (++i % 5 == 0)
            return true;
         else
            return false;

    if (attr1 == nullptr || attr2 == nullptr) {
        //        std::cerr << "One of the attributes is null." << std::endl;
        return false;
    }
    return (attr1->st_mtim.tv_sec == attr2->st_mtim.tv_sec) &&
           (attr1->st_mtim.tv_nsec == attr2->st_mtim.tv_nsec);
}

/*
 * This is the readdir map which contains a map of the readdir listing
 * corresponding to a inode.
 */
struct readdir_cache
{
private:
    std::map<fuse_ino_t, std::shared_ptr<directory_list_cache>> m_readdirmap;

    std::shared_mutex m_lockreaddirmap;

    // nfs client instance to which this cache belongs.
    const nfs_client* nfsclient;

    readdir_cache(const nfs_client* nfsclient_):
        nfsclient(nfsclient_)
    {}

public:
    static struct readdir_cache* get_instance(const nfs_client* nfsclient)
    {
        static readdir_cache rcache(nfsclient);
        return &rcache;
    }

    bool make_getattr_call(fuse_ino_t inode, struct stat& attr)
    {
        // Make a sync call to fetch the attributes.

        // TODO: Populate this function.
        return true;
    }

    // Purge the entry corresponding to ino
    void purge(fuse_ino_t inode)
    {
        // Take exclusive lock to purge the cache.
        std::unique_lock<std::shared_mutex> lock(m_lockreaddirmap);
        const auto it = m_readdirmap.find(inode);
        if (it !=  m_readdirmap.end())
        {
            /*
             * Note: If someone is already holding a ref to this shared pointer, they will
             * continue to use it till it goes out of scope.
             */
            m_readdirmap.erase(it);
        }
    }

    // Checks if the cache entry should be purged. Returns true if purging is done.
    bool purge_cache_if_required(fuse_ino_t inode)
    {
        // TODO: Just pass inode and get the shared ptr from find.
        std::shared_ptr<directory_list_cache> dircache_handle;
        {
            // Taske sahred lock to see if the entry already exists.
            std::shared_lock<std::shared_mutex> lock(m_lockreaddirmap);
            const auto it = m_readdirmap.find(inode);
            if (it !=  m_readdirmap.end())
            {
                // Return the entry if it exists in the map.
                dircache_handle = it->second;
            }
            else
            {
                // No entry in the cache, no point in purgin.
                return false;
            }
        }

        struct directory_entry* dirent = nullptr;
        bool success = false;
        struct stat attr;

        /*
         * For now the only prune condition check is the directory mtime.
         * This function can later be extended to add more conditions.
         */
        bool should_update_mtime = false;
        {
            // Take a shared lock to see if the last_mtime_check is greater than 30 secs.
            //std::shared_lock<std::shared_mutex> lock(dircache_handle->get_lock());

            bool found = dircache_handle->get_entry_at(1 /* cookie*/, &dirent); // cookie 1 represent '.', hence this gets us cached dir mtime.
            if (!found)
            {
                /*
                 * cookie = 0 refers to the current directory '.'.
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
            success = make_getattr_call(inode, attr);
            // Issue a getattr call to the server to fetch the directory mtime.
        }

        if (success)
        {
            // std::shared_lock<std::shared_mutex> lock(dircache_handle->get_lock());
            if (!are_mtimes_equal(&dirent->attributes, &attr))
            {
                /*
                * Purge the cache since the directory mtime has changed/
                * THis indicates that the directory has changed.
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
        purge(inode);
        return true;
    }


    /*
     * This method gets a directory handle to the directory_list_cache into which the
     * directory entries are inserted.
     * If the handle exists, it returns the existing handle, else it creates a new handle
     * and returns this newly created handle.
     * THe caller owning this handle should take appropriate lock before accessing the
     * shared pointer members.
     */
    std::shared_ptr<directory_list_cache> get(fuse_ino_t inode)
    {
        // Check if the cache should be purged.
        purge_cache_if_required(inode);

        {
            // Taske sahred lock to see if the entry already exists.
            std::shared_lock<std::shared_mutex> lock(m_lockreaddirmap);
            const auto it = m_readdirmap.find(inode);
            if (it !=  m_readdirmap.end())
            {
                // Return the entry if it exists in the map.
                return it->second;
            }
        }

        /*
         * If we reach here, it means the entry corresponding to the inode does not exist.
         * We will create a new entry in the map and then return the directory_list_cache poitner.
         */

        std::shared_ptr<directory_list_cache> dirlistcache_handle =
            std::make_shared<directory_list_cache>(inode);

        // Take an exclusive lock to insert the entry to the map.
        std::unique_lock<std::shared_mutex> lock(m_lockreaddirmap);
        auto it = m_readdirmap.insert({inode, dirlistcache_handle});

        /*
         * There is the chance that the above insert can fail if another thread inserts
         * the entry by the time we upgrade from shared_lock to exclusive_lock on the map.
         */
        if (!it.second)
        {
            const auto iter = m_readdirmap.find(inode);
            assert(iter != m_readdirmap.end());
            return iter->second;
        }

        return dirlistcache_handle;
    }

#if 0
    bool add(fuse_ino_t inode, std::shared_ptr<directory_list_cache> dir_cache)
    {
        // Take exclusive lock to add the entry to the cache.
        std::unique_lock<std::shared_mutex> lock(m_lockreaddirmap);
//         std::shared_ptr<directory_list_cache> dir_cache_sp(dir_cache);
        const auto& it = m_readdirmap.insert({inode, dir_cache});
        return it.second;
    }
#endif
    /*
     * This function lists directory entries corresponding to the inode.
     * It looks into the cache to list entries starting from cookie \p cookie_ and
     * fills the entry into \p results upto max size \pmax_size.
     * Note: This will only fetch the entries from the cache and does not do a backend call.
     */
    void lookup(
        fuse_ino_t inode /* dir inode for which readdir call is requested */,
        cookie3 cookie_ /* offset in the directory from which the directory should be listed*/,
        size_t max_size /* maximum size of entries to be returned*/,
        std::vector<directory_entry* >& results /* dir entries listed*/,
        //size_t& result_size /* size of the directory entries being returned*/,
        bool& eof,
        bool skip_attr_size = false)
    {
        // Check if the cache should be purged.
        purge_cache_if_required(inode);

        std::shared_ptr<directory_list_cache> dir_cache_handle;
        {
            // Take a shared lock on the map.
            std::shared_lock<std::shared_mutex> lock(m_lockreaddirmap);

            auto it = m_readdirmap.find(inode);
            if (it != m_readdirmap.end())
            {
                dir_cache_handle = it->second;
            }
            else
            {
                // There is no entry corresponding to this inode in the cache.
                return;
                //return false;
            }

            /*
             * Note: The shared-lock on the m_readdirmap is released here, as we already have obtained the
             *       shared_ptr on the directory_list_cache entry. So we basically have a shared pointer to access
             *       the directory entries and we do not have to block access to other directory listing.
             */
        }

        int num_of_entries_found_in_cache = 0;

        size_t rem_size = max_size;
        //bool dir_present_in_cache = false;
        while (rem_size > 0)
        {
            struct directory_entry* entry;
            bool found = dir_cache_handle->lookup(cookie_, &entry);
            if (found)
            {
         //       dir_present_in_cache = true;
                const size_t curr_entry_size = entry->get_size(skip_attr_size);
                if (rem_size >= curr_entry_size)
                {
                    num_of_entries_found_in_cache++;
                    //result_size += curr_entry_size;
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
                //return false;
            }
        }

        eof = dir_cache_handle->get_eof();
        AZLogDebug("Buffer exhaust: Num of entries returned from cache {}", num_of_entries_found_in_cache);
        /*
         * NOTE: We will not populate the cookie verifier here nor access it since
         * that will be done only at the time the readdir call is made to the backend.
         */
        //return dir_present_in_cache;
    }
};
#endif /* __READDIR_RPC_TASK__ */
