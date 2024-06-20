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
    const cookie3 cookie;
    struct stat attributes;
    /*
     * whether 'attributes' holds valid attributes?
     * directory_entry which are made as a result of READDIR call, would
     * not have the attributes. Those can only be used by subsequent
     * readdir calls made by fuse. If fuse makes readdirplus call and
     * we don't have the attributes, we treat it as "entry not found"
     * and reach out to server with a READDIRPLUS call and on receipt
     * of response update the directory_entry cache, this time with
     * attributes.
     */
    bool has_attributes;

    /*
     * Again, for READDIR fetched entries, we won't know the filehandle
     * (and the fileid), hence we won't have the inode set.
     */
    struct nfs_inode* const nfs_inode;
    const char* const name;

    // Constructor for adding a readdirplus returned entry.
    directory_entry(const char* name_,
                    cookie3 cookie_,
                    const struct stat& attr,
                    struct nfs_inode* nfs_inode_);

    // Constructor for adding a readdir returned entry.
    directory_entry(const char* name_,
                    cookie3 cookie_,
                    uint64_t fileid_);

    ~directory_entry();

    /*
     * Returns size of the directory_entry.
     * This is used to find the cache space taken by this directory_entry.
     */
    size_t get_cache_size() const
    {
        /*
         * Since we store this directory_entry in a map, it will have two
         * pointers and a key and value, all 8 bytes each, so we add those
         * to get a closer estimate.
         *
         * Note: It may take slightly more than this.
         */
        return sizeof(*this) + strlen(name) + 4*sizeof(uint64_t);
    }

    /**
     * Return size of fuse buffer required to hold this directory_entry.
     * If readdirplus is true, the size returned is for containing the
     * entry along with the attributes, else it's w/o the attributes.
     */
    size_t get_fuse_buf_size(bool readdirplus) const
    {
        if (readdirplus) {
            return fuse_add_direntry_plus(
                    nullptr, nullptr, 0, name, nullptr, 0);
        } else {
            return fuse_add_direntry(
                    nullptr, nullptr, 0, name, nullptr, 0);
        }
    }

    bool is_dot_or_dotdot() const
    {
        return (name != nullptr) &&
               ((name[0] == '.') &&
                ((name[1] == '\0') ||
                 ((name[1] == '.') && (name[2] == '\0'))));
    }
};

#define DIR_MTIME_REFRESH_INTERVAL_SEC 30

struct readdirectory_cache
{
private:
    /*
     * This will be set if we have read all the entries of the directory
     * from the backend.
     */
    bool eof;

    /*
     * last cookie.
     * Only valid if eof is true.
     */
    uint64_t eof_cookie = (uint64_t) -1;

    // Size of the cache.
    size_t cache_size;

    cookieverf3 cookie_verifier;

    std::map<cookie3, struct directory_entry*> dir_entries;

    // This lock protects all the memberf of this readdirectory_cache.
    std::shared_mutex readdircache_lock;

public:
    readdirectory_cache():
        eof(false),
        cache_size(0)
    {
        assert(dir_entries.empty());
        // Initial cookie_verifier must be 0.
        ::memset(&cookie_verifier, 0, sizeof(cookie_verifier));
    }

    std::shared_mutex& get_lock()
    {
        return readdircache_lock;
    }

    /*
     * Return true and populates the \p dirent if the entry corresponding to \p cookie exists.
     * Returns false otherwise.
     */
    bool get_entry_at(cookie3 cookie, struct directory_entry** dirent)
    {
        // Take shared lock on the map.
        std::shared_lock<std::shared_mutex> lock(readdircache_lock);
        auto it = dir_entries.find(cookie);

        if (it != dir_entries.end())
        {
            *dirent = it->second;
            return true;
        }

        return false;
    }

    bool add(struct directory_entry* entry)
    {
        assert(entry != nullptr);
        
        {
            // Get exclusive lock on the map to add the entry to the map.
            std::unique_lock<std::shared_mutex> lock(readdircache_lock);
        
            if (cache_size >= MAX_CACHE_SIZE_LIMIT)
            {
                AZLogWarn("Exceeding cache max size. No more entries will be added to the cache! cuurent size: {}", cache_size);
                return false;
            }

            const auto& it = dir_entries.insert({entry->cookie, entry});
            cache_size += entry->get_cache_size();
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

    uint64_t get_eof_cookie() const
    {
        return eof_cookie;
    }

    void set_cookieverf(const cookieverf3* cokieverf)
    {
        assert(cokieverf != nullptr);

        // TODO: Can this be made atomic? Get exclusive lock to update the cookie verifier.
        std::unique_lock<std::shared_mutex> lock(readdircache_lock);
        ::memcpy(&cookie_verifier, cokieverf, sizeof(cookie_verifier));
    }

    void set_eof(uint64_t eof_cookie)
    {
        // Every directory will at least have "." and "..".
        assert(eof_cookie >= 2);

        eof = true;
        this->eof_cookie = eof_cookie;
    }

    struct directory_entry *lookup(cookie3 cookie)
    {
        // Take shared look to see if the entry exist in the cache.
        std::shared_lock<std::shared_mutex> lock(readdircache_lock);

        const auto it = dir_entries.find(cookie);

        return (it != dir_entries.end()) ? it->second : nullptr;
    }

    void clear()
    {
        std::unique_lock<std::shared_mutex> lock(readdircache_lock);

        eof = false;
        cache_size = 0;
        ::memset(&cookie_verifier, 0, sizeof(cookie_verifier));

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
        std::unique_lock<std::shared_mutex> lock(readdircache_lock);

        for (auto it = dir_entries.begin(); it != dir_entries.end(); ++it)
        {
            free(it->second);
        }

        dir_entries.clear();
    }

    // Various helper methods added below.
    static bool are_mtimes_equal(const struct stat* attr1, const struct stat* attr2)
    {
        if (attr1 == nullptr || attr2 == nullptr) {
            //        std::cerr << "One of the attributes is null." << std::endl;
            return false;
        }
        return (attr1->st_mtim.tv_sec == attr2->st_mtim.tv_sec) &&
               (attr1->st_mtim.tv_nsec == attr2->st_mtim.tv_nsec);
    }
};
#endif /* __READDIR_RPC_TASK___ */
