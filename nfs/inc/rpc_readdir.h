#ifndef __READDIR_RPC_TASK__
#define __READDIR_RPC_TASK__

#include "aznfsc.h"
#include <map>
#include <shared_mutex>
#include <vector>
#include <ctime>
#include <dirent.h>

// 1GB
#define MAX_CACHE_SIZE_LIMIT 1073741824

//struct readdirectory_cache;
//struct directory_entry;

/*
 * This is an entry in the unified readdir/DNLC cache.
 * Note that an entry can be added to the unified cache, via one of the
 * following:
 * 1. READDIRPLUS response.
 *    This creates the most complete entry with a valid cookie and a valid
 *    nfs_inode pointer (with attributes and filehandle).
 *    This can serve: READDIRPLUS, READDIR and LOOKUP requests.
 * 2. READDIR response.
 *    This creates an entry with a valid cookie but no nfs_inode pointer.
 *    Only attributes.st_ino (the inode number) is valid.
 *    This can serve: READDIR requests.
 * 3. LOOKUP response.
 *    This creates an entry with a special cookie (which is not possible in
 *    READDIR/READDIRPLUS responses) but a valid nfs_inode pointer.
 *    This can serve: LOOKUP requests. Though it has the nfs_inode pointer
 *                    it doesn't have the cookie, hence cannot serve directory
 *                    enumeration requests which need a valid cookie.
 *
 * Note: Blob NFS uses cookies starting at 1 and increasing by 1 for every
 *       file, so we use UINT64_MAX/2 as the starting value for the special
 *       cookie. This should never be returned in READDIR/READDIRPLUS response
 *       hence we won't mistake a type (3) entry as type (1).

 * Note on updating directory entries added in readdirectory_cache.
 *
 * READDIR and READDIRPLUS responses will always update old entries, deleting
 * existing ones and adding new ones. This means if we have a type (1) entry
 * and we get a READDIR response, it'll be deleted and a new type (2) entry
 * will be created.
 *
 * LOOKUP response will update the entry with the following rules:
 * Note that we don't want to blindly replace type (1) or (2) entries with
 * type (3) entries as those are not usable by READDIR/READDIRPLUS then.
 *
 * - If we have a type (1) entry and the new nfs_inode in the lookup response
 *   matches the saved one, don't do anything. This is the common case.
 * - If we have a type (1) entry and the new nfs_inode does not match the saved
 *   one, it means the file was either renamed or deleted and re-created. Next
 *   time when aznfsc_ll_readdir{plus}() is called it'll purge the entire
 *   readdir cache as the parent directory mtime would be different, thus
 *   ensuring correctness, but if lookup is called before readdir/readdirplus
 *   it'll delete the old entry and create a new type (3) entry.
 * - If we have a type (2) entry and the new nfs_inode matches the saved one,
 *   add nfs_inode and update directory_entry.attributes, thus promoting it to
 *   type (1). Only in this case we update nfs_inode in a directory_entry.
 * - If we have a type (2) entry and the new nfs_inode does not match the
 *   saved one, delete the old entry and create a new type (3) entry.
 * - If we have a type (3) entry and the new nfs_inode matches the saved one,
 *   don't do anything.
 * - If we have a type (3) entry and the new nfs_inode does not matche the
 *   saved one, delete the old entry and create a new type (3) entry.
 */
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
    struct nfs_inode *const nfs_inode;
    char *const name;

    // Constructor for adding a readdirplus returned entry.
    directory_entry(char* name_,
                    cookie3 cookie_,
                    const struct stat& attr,
                    struct nfs_inode* nfs_inode_);

    // Constructor for adding a readdir returned entry.
    directory_entry(char* name_,
                    cookie3 cookie_,
                    uint64_t fileid_);

    ~directory_entry();

    /**
     * nfs_inode is a const member to highlight the fact that it's not updated
     * once initialized by the constuctor. Only in the case where we need to
     * promote a type (2) entry to type (1) on receiving a LOOKUP response,
     * we allow the update.
     *
     * Every nfs_inode referenced by directory_entry has a dircachecnt ref
     * which is dropped by ~directory_entry(), we grab that here as we add
     * the nfs_inode to directory_entry.
     */
    void update_inode(struct nfs_inode *inode);

    /**
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
    size_t get_fuse_buf_size([[maybe_unused]] bool readdirplus) const
    {
#ifndef ENABLE_NO_FUSE
        if (readdirplus) {
            return fuse_add_direntry_plus(
                    nullptr, nullptr, 0, name, nullptr, 0);
        } else {
            return fuse_add_direntry(
                    nullptr, nullptr, 0, name, nullptr, 0);
        }
#else
        /*
         * In nofuse mode we just add dirent objects to user buffer.
         */
        return sizeof(struct dirent) + ::strlen(name);
#endif
    }

    static bool is_dot_or_dotdot(const char *name)
    {
        return (name != nullptr) &&
               ((name[0] == '.') &&
                ((name[1] == '\0') ||
                 ((name[1] == '.') && (name[2] == '\0'))));
    }

    bool is_dot_or_dotdot() const
    {
        return is_dot_or_dotdot(name);
    }
};

/**
 * This is our unified readdir and DNLC cache.
 */
struct readdirectory_cache
{
private:
    /*
     * The singleton nfs_client, for convenience.
     */
    struct nfs_client *const client;

    /*
     * Directory inode, whose contents are cached by this readdirectory_cache.
     */
    struct nfs_inode *const inode;

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

    /*
     * Last cookie of the sequence that started at the start of the directory.
     * If the sequence goes all the way upto eof w/o any gaps in between then
     * we can mark the directory as "confirmed", in set_eof().
     * It means that we have the entire directory in our cache and hence DNLC
     * cache can reply to negative lookup with certainty.
     * This is reset when we purge the cache.
     */
    uint64_t seq_last_cookie = 0;

    // Size of the cache.
    size_t cache_size;

    cookieverf3 cookie_verifier;

    /*
     * Absolute time in msecs since epoch when this directory cache was last
     * confirmed. A directory is said to be "confirmed" when we know that we
     * have the full directory cached and hence we can respond to -ve lookup
     * requests with confidence. By definition every newly created directory
     * starts as confirmed.
     */
    std::atomic<uint64_t> confirmed_msecs = 0;

    /*
     * dir_entries is the readdir cache, indexed by cookie value.
     * We double readdir cache as DNLC cache too. dnlc_map is used to convert
     * filename (which is the index into the DNLC cache) to cookie (which is
     * the index into the readdir cache).
     */
    std::map<cookie3, struct directory_entry*> dir_entries;
    std::unordered_map<std::string, cookie3> dnlc_map;

    /*
     * This lock protects all the members of this readdirectory_cache.
     * XXX We should use inode->ilock to protect this too?
     */
    mutable std::shared_mutex readdircache_lock;

public:
    readdirectory_cache(struct nfs_client *_client,
                        struct nfs_inode *_inode):
        client(_client),
        inode(_inode),
        eof(false),
        cache_size(0)
    {
        assert(client);
        assert(inode);
        assert(dir_entries.empty());
        // Initial cookie_verifier must be 0.
        ::memset(&cookie_verifier, 0, sizeof(cookie_verifier));
    }

    ~readdirectory_cache();

    std::shared_mutex& get_lock()
    {
        return readdircache_lock;
    }

    /**
     * Call this to check if the cache is empty.
     */
    bool is_empty() const
    {
        return dir_entries.empty();
    }

    // This is helpul for asserting.
    size_t get_num_entries() const
    {
        return dir_entries.size();
    }

    /*
     * Return true and populates the \p dirent if the entry corresponding
     * to \p cookie exists.
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

    /**
     * Set this directory cache as "confirmed".
     */
    void set_confirmed();
    void clear_confirmed();

    /**
     * Is this directory cache confirmed?
     * If readdirectory_cache lookup returns no entry and is_confirmed()
     * returns true, then we can return a negative lookup response to fuse.
     * Depending on the config we may only consider if the directory was
     * confirmed no longer than a certain period.
     */
    bool is_confirmed() const;

    bool add(struct directory_entry* entry, bool acquire_lock = true);
    void dnlc_add(const char *filename, struct nfs_inode *inode);

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

    void set_eof(uint64_t eof_cookie);

    /**
     * Given a filename, returns the cookie corresponding to that.
     * The cookie returned is the one returned for this filename, by the latest
     * READDIR/READDIRPLUS response.
     * A return value of 0 means the file was not found in the cache.
     *
     * Note: Caller MUST hold the readdircache_lock.
     */
    cookie3 filename_to_cookie(const char *filename) const
    {
        const auto it = dnlc_map.find(filename);
        const cookie3 cookie = (it == dnlc_map.end()) ? 0 : it->second;

#ifndef ENABLE_NON_AZURE_NFS
        /*
         * Blob NFS uses 1:1 mappign betweek cookie and files, so the
         * following sanity assert should be good to catch any bugs.
         */
        assert(cookie < UINT32_MAX);
#endif

        return cookie;
    }

    /**
     * Lookup and return the directory_entry corresponding to the
     * given cookie.
     * lookup() is the readdir cache lookup method, while dnlc_lookup() is
     * the DNLC cache lookup method.
     *
     * Note: lookup() returns after holding a dircachecnt ref on the inode,
     *       while dnlc_lookup() holds a lookupcnt ref on the inode.
     *       Caller must drop this extra ref held.
     */
    struct directory_entry *lookup(
            cookie3 cookie,
            const char *filename_hint = nullptr,
            bool acquire_lock = true) const;

    struct nfs_inode *dnlc_lookup(const char *filename,
                                  bool *negative_confirmed = nullptr) const;

    /**
     * Remove the given cookie from readdirectory_cache.
     * Returns false if the cookie was not found, else it delete the cookie
     * and returns true. It also deletes the inode if this was the last ref
     * on the inode.
     * remove() is the readdir cache delete method, while dnlc_remove() is
     * the DNLC cache delete method.
     */
    bool remove(cookie3 cookie,
                const char *filename_hint = nullptr,
                bool acquire_lock = true);

    bool dnlc_remove(const char *filename)
    {
        assert(filename != nullptr);
        return remove(0, filename);
    }

    /**
     * Remove all entries from the cache.
     * Also delete the inodes for those entries for which this was the last
     * ref.
     */
    void clear();
};
#endif /* __READDIR_RPC_TASK___ */
