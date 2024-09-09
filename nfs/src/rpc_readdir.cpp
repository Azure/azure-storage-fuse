#include "rpc_readdir.h"
#include "nfs_inode.h"
#include "nfs_client.h"

directory_entry::directory_entry(const char *name_,
                                 cookie3 cookie_,
                                 const struct stat& attr,
                                 struct nfs_inode* nfs_inode_) :
    cookie(cookie_),
    attributes(attr),
    has_attributes(true),
    nfs_inode(nfs_inode_),
    name(name_)
{
    // Sanity check for attr. Blob NFS only supports these files.
    assert(((attr.st_mode & S_IFMT) == S_IFREG) ||
           ((attr.st_mode & S_IFMT) == S_IFDIR) ||
           ((attr.st_mode & S_IFMT) == S_IFLNK));

    /*
     * inode must have a refcnt held before adding to directory_entry.
     * Every inode referenced from a directory_entry has a dircachecnt
     * reference held. We need the lookupcnt ref to ensure the inode is
     * not freed before we grab the dircachecnt ref.
     * Once dircachecnt ref is held, the caller may choose to drop the
     * lookupcnt ref and dircachecnt ref will correctly prevent the inode
     * from being freed while it's referenced by the directory_entry.
     */
    assert(!nfs_inode->is_forgotten());
    nfs_inode->dircachecnt++;
}

directory_entry::directory_entry(const char *name_,
                                 cookie3 cookie_,
                                 uint64_t fileid_) :
    cookie(cookie_),
    has_attributes(false),
    nfs_inode(nullptr),
    name(name_)
{
    // NFS recommends against this.
    assert(fileid_ != 0);

    // fuse_add_direntry() needs these two fields, so set them.
    ::memset(&attributes, 0, sizeof(attributes));
    attributes.st_ino = fileid_;
    attributes.st_mode = 0;
}

directory_entry::~directory_entry()
{
    AZLogVerbose("~directory_entry called");
    if (nfs_inode) {
        assert(nfs_inode->dircachecnt > 0);
        nfs_inode->dircachecnt--;
    }
}

bool readdirectory_cache::add(struct directory_entry* entry)
{
    assert(entry != nullptr);

    {
        // Get exclusive lock on the map to add the entry to the map.
        std::unique_lock<std::shared_mutex> lock(readdircache_lock);

        // TODO: Fix this.
        if (cache_size >= MAX_CACHE_SIZE_LIMIT) {
            AZLogWarn("[{}] Exceeding cache max size. No more entries will "
                      "be added to the cache! curent size: {}",
                      this->inode->get_fuse_ino(), cache_size);
            return false;
        }

        if (entry->nfs_inode) {
            /*
             * directory_entry constructor must have grabbed the
             * dircachecnt ref.
             */
            assert(entry->nfs_inode->dircachecnt > 0);

            AZLogDebug("[{}] Adding {} fuse ino {}, cookie {}, to readdir "
                       "cache (dircachecnt {})",
                       this->inode->get_fuse_ino(),
                       entry->name,
                       entry->nfs_inode->get_fuse_ino(),
                       entry->cookie,
                       entry->nfs_inode->dircachecnt.load());
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

struct directory_entry *readdirectory_cache::lookup(cookie3 cookie) const
{
    // Take shared look to see if the entry exist in the cache.
    std::shared_lock<std::shared_mutex> lock(readdircache_lock);

    const auto it = dir_entries.find(cookie);

    struct directory_entry *dirent =
        (it != dir_entries.end()) ? it->second : nullptr;

    if (dirent && dirent->nfs_inode) {
        /*
         * When a directory_entry is added to to readdirectory_cache we
         * hold a ref on the inode, so while it's in the cache dircachecnt
         * must be non-zero.
         */
        assert(dirent->nfs_inode->dircachecnt > 0);

        /*
         * Grab a ref on behalf of the caller so that the inode doesn't
         * get freed while the directory_entry is referring to it.
         * Once they are done using this directory_entry, they must drop
         * this ref, mostly done in send_readdir_response().
         */
        dirent->nfs_inode->dircachecnt++;
    }

    return dirent;
}

bool readdirectory_cache::remove(cookie3 cookie)
{
    struct nfs_inode *inode = nullptr;

    {
        std::unique_lock<std::shared_mutex> lock(readdircache_lock);

        const auto it = dir_entries.find(cookie);
        struct directory_entry *dirent =
            (it != dir_entries.end()) ? it->second : nullptr;

        /*
         * Given cookie not found in the cache.
         * It should not happpen though since the caller would call remove()
         * only after checking.
         */
        if (!dirent) {
            return false;
        }

        assert(dirent->cookie == cookie);

        /*
         * This just removes it from the cache, no destructor is called at
         * this point.
         */
        dir_entries.erase(it);

        inode = dirent->nfs_inode;

        // READDIR created cache entry, nothing more to do.
        if (!inode) {
            delete dirent;
            return true;
        }

        assert(inode->magic == NFS_INODE_MAGIC);

        /*
         * Any inode referenced by a directory_entry added to a
         * readdirectory_cache must have one reference held, by
         * readdirectory_cache::add().
         */
        assert(inode->dircachecnt > 0);

        AZLogDebug("[{}] Removing {} fuse ino {}, cookie {}, from "
                   "readdir cache (lookupcnt={}, dircachecnt={})",
                   this->inode->get_fuse_ino(),
                   dirent->name,
                   inode->get_fuse_ino(),
                   dirent->cookie,
                   inode->lookupcnt.load(),
                   inode->dircachecnt.load());

        /*
         * If this is the last dircachecnt on this inode, it means
         * there are no more readdirectory_cache,s referencing this
         * inode. If there are no lookupcnt refs then we can free it.
         * Call put_nfs_inode() to safely free the inode.
         */
        if (inode->dircachecnt == 1) {
            /*
             * Grab inode ref so that the inode is not removed from
             * inode_map and deleted after we drop the dircachecnt
             * and before we grab the inode_map_lock below. This ref
             * will be dropped by put_nfs_inode_nolock() safely with
             * the lock held.
             */
            inode->incref();

            /*
             * This will call ~directory_entry() which will drop the
             * inode's original dircachecnt.
             */
            delete dirent;
        } else {
            delete dirent;
            return true;
        }
    }

    AZLogDebug("[{}] inode {} to be freed, after readdir cache remove",
               this->inode->get_fuse_ino(),
               inode->get_fuse_ino());

    /*
     * Drop the extra ref we held above.
     * Note that we must call put_nfs_inode_nolock() only when we are sure
     * this is the last ref.
     */
    {
        std::unique_lock<std::shared_mutex> lock1(client->get_inode_map_lock());
        assert(inode->lookupcnt > 0);
        if (--inode->lookupcnt == 0) {
            client->put_nfs_inode_nolock(inode, 0 /* dropcnt */);
        }
    }

    return true;
}

/*
 * inode_map_lock must be held by the caller.
 */
void readdirectory_cache::clear()
{
    /*
     * TODO: Later when we implement readdirectory_cache purging due to
     *       memory pressure, we need to ensure that any directory which
     *       is currently being enumerated by nfs_inode::lookup_dircache(),
     *       should not be purged, as that may cause those inodes to be
     *       orphanned (they will have lookupcnt and dircachecnt of 0 and
     *       still lying aroung in the inode_map.
     */
    std::vector<struct nfs_inode*> tofree_vec;

    {
        std::unique_lock<std::shared_mutex> lock(readdircache_lock);

        eof = false;
        cache_size = 0;
        ::memset(&cookie_verifier, 0, sizeof(cookie_verifier));

        for (auto it = dir_entries.begin(); it != dir_entries.end(); ++it) {
            struct nfs_inode *inode = it->second->nfs_inode;
            if (inode) {
                assert(inode->magic == NFS_INODE_MAGIC);
                /*
                 * Any inode referenced by a directory_entry added to
                 * a readdirectory_cache must have one reference held,
                 * by readdirectory_cache::add().
                 */
                assert(inode->dircachecnt > 0);

                AZLogDebug("[{}] Removing {} fuse ino {}, cookie {}, from "
                           "readdir cache (dircachecnt {} lookupcnt {})",
                           this->inode->get_fuse_ino(),
                           it->second->name,
                           inode->get_fuse_ino(),
                           it->second->cookie,
                           inode->dircachecnt.load(), inode->lookupcnt.load());
            }

            /*
             * If this is the last dircachecnt on this inode, it means
             * there are no more readdirectory_cache,s referencing this
             * inode. If there are no lookupcnt refs then we can free it.
             * Call put_nfs_inode() to safely free the inode.
             */
            if (inode && (inode->dircachecnt == 1)) {
                tofree_vec.emplace_back(inode);
                assert(inode->magic == NFS_INODE_MAGIC);

                /*
                 * Grab inode ref so that the inode is not removed from
                 * inode_map and deleted after we drop the dircachecnt
                 * and before we grab the inode_map_lock below. This ref
                 * will be dropped by put_nfs_inode_nolock() safely with
                 * the lock held.
                 */
                inode->incref();
            }

            /*
             * This will call ~directory_entry(), which will drop the
             * dircachecnt. Note that we grabbed a lookupcnt ref on the
             * inode before this to prevent another threads from freeing
             * the inode before we grab the inode_map_lock below.
             */
            delete it->second;
        }

        dir_entries.clear();
    }

    if (!tofree_vec.empty()) {
        std::unique_lock<std::shared_mutex> lock1(client->get_inode_map_lock());

        AZLogDebug("[{}] {} inodes to be freed, after readdir cache purge",
                   this->inode->get_fuse_ino(),
                   tofree_vec.size());

        /*
         * Drop the extra ref we held above, for all inodes in tofree_vec.
         * Note that we must call put_nfs_inode_nolock() only when we are sure
         * it is the last ref on the inode.
         */
        for (struct nfs_inode *inode : tofree_vec) {
            assert(inode->magic == NFS_INODE_MAGIC);
            assert(inode->lookupcnt > 0);
            if (--inode->lookupcnt == 0) {
                client->put_nfs_inode_nolock(inode, 0 /* dropcnt */);
            }
        }
    }
}
