#ifndef __READDIR_RPC_TASK__
#define __READDIR_RPC_TASK__

#include "aznfsc.h"
#include <map>
#include <shared_mutex>
#include <vector>
#include <ctime>

// 1GB
#define MAX_CACHE_SIZE_LIMIT 1073741824

//struct readdirectory_cache;
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
    const struct nfs_inode* const nfs_ino;
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

struct readdirectory_cache
{
private:
    //const fuse_ino_t inode;

    /*
     * This will be set if we have read all the entries of the directory
     * from the backend.
     */
    bool eof;

    size_t cache_size;

//    const std::time_t create_time;

    // The time at which we checked the directory mtime by making getattr call.
    std::time_t last_mtimecheck;
    //std::atomic<std::time_t> last_mtimecheck;

    cookieverf3 cookie_verifier;

    // TODO: See if we can just make it a vector and can be indexed by cookie.
    std::map<cookie3, struct directory_entry*> dir_entries;

    std::shared_mutex lock_dirlistcache;

public:
    readdirectory_cache():
      //  inode(ino),
        eof(false),
        cache_size(0),
  //      create_time(std::time(nullptr)),
        last_mtimecheck(std::time(nullptr))
    {
        //AZLogDebug("In readdirectory_cache() constr");
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
        // last_mtimecheck.store(std::time(nullptr));
    }

    std::time_t get_lastmtime()
    {
        // TODO: Make this atomic or put it under lock.
        return last_mtimecheck;
        //return last_mtimecheck.load();
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

    void clear()
    {
        AZLogInfo("clear called");
        // This cache has now become invalid. clean it up.
        std::unique_lock<std::shared_mutex> lock(lock_dirlistcache);

        eof = false;
        cache_size = 0;
        last_mtimecheck = std::time(nullptr);
        ::memset(&cookie_verifier, 0, sizeof(cookie_verifier));

  //      create_time(std::time(nullptr)),
        for (auto it = dir_entries.begin(); it != dir_entries.end(); ++it)
        {
            free(it->second);
        }

        dir_entries.clear();
    }

    ~readdirectory_cache()
    {
        AZLogInfo(" ~readdirectory_cache() called");
        // This cache has now become invalid. clean it up.
        std::unique_lock<std::shared_mutex> lock(lock_dirlistcache);

        for (auto it = dir_entries.begin(); it != dir_entries.end(); ++it)
        {
            free(it->second);
        }

        dir_entries.clear();
    }

// Helper methods.
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
    static bool make_getattr_call(fuse_ino_t inode, struct stat& attr)
    {
        // Make a sync call to fetch the attributes.

        // TODO: Populate this function.
        return true;
    }
};
#endif /* __READDIR_RPC_TASK___ */

