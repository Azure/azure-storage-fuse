#ifndef __NFS_INODE_H__
#define __NFS_INODE_H__

#include <atomic>
#include "aznfsc.h"
#include "rpc_readdir.h"

#define NFS_INODE_MAGIC *((const uint32_t *)"NFSI")

/**
 * This is the NFS inode structure. There is one of these per file/directory
 * and contains any global information about the file/directory., f.e.,
 * - NFS filehandle for accessing the file/directory.
 * - FUSE inode number of the file/directory.
 * - File/Readahead cache (if any).
 * - Anything else that we want to maintain per file.
 */
struct nfs_inode
{
    /*
     * As we typecast back-n-forth between the fuse inode number and our
     * nfs_inode structure, we use the magic number to confirm that we
     * have the correct pointer.
     */
    const uint32_t magic = NFS_INODE_MAGIC;

    /*
     * Inode lock.
     * Inode must be updated only with this lock held.
     */
    std::shared_mutex ilock;

    /*
     * Ref count of this inode.
     * Whenever we make one of the following calls, we must increment the
     * lookupcnt of the inode:
     * - fuse_reply_entry()
     * - fuse_reply_create()
     * - Lookup count of every entry returned by readdirplus(), except "."
     *   and "..", is incremented by one. Note that readdir() does not
     *   affect the lookup count of any of the entries returned.
     *
     * Note that the lookupcnt is set to 0 when the nfs_inode is created
     * and only when we are able to successfully convey creation of the inode
     * to fuse, we increment it to 1. This is important as unless fuse
     * knows about an inode it'll never call forget() for it and we will
     * leak the inode.
     *
     * forget() causes lookupcnt for an inode to be reduced by the "nlookup"
     * parameter count. forget_multi() does the same for multiple inodes in
     * a single call.
     * On umount the lookupcnt for all inodes implicitly drops to zero, and
     * fuse may not call forget() for the affected inodes.
     *
     * Till the lookupcnt of an inode drops to zero, we MUST not free the
     * nfs_inode structure, as kernel may send requests for files with
     * non-zero lookupcnt, even after calls to unlink(), rmdir() or rename().
     */
    mutable std::atomic<uint64_t> lookupcnt = 0;

    /*
     * NFSv3 filehandle returned by the server.
     * We use this to identify this file/directory to the server.
     */
    nfs_fh3 fh;

    /*
     * Fuse inode number.
     * This is how fuse identifies this file/directory to us.
     */
    const fuse_ino_t ino;

    /*
     * S_IFREG, S_IFDIR, etc.
     * 0 is not a valid file type.
     */
    const uint32_t file_type = 0;

    /*
     * Cached attributes for this inode and the current value of attribute
     * cache timeout. attr_timeout_secs will be have a value between
     * [acregmin, acregmax] or [acdirmin, acdirmax], depending on the
     * filetype, and holds the current attribute cache timeout value for
     * this inode, adjusted by exponential backoff and capped by the max
     * limit.
     * These cached attributes are valid till the absolute milliseconds value
     * attr_timeout_timestamp. On expiry of this we will revalidate the inode
     * by querying the attributes from the server. If the revalidation is
     * successful (i.e., inode has not changed since we cached), then we
     * increase attr_timeout_secs in an exponential fashion (upto the max
     * actimeout value) and set attr_timeout_timestamp accordingly.
     *
     * If attr_timeout_secs is -1 that implies that cached attributes are
     * not valid and we need to fetch the attributes from the server.
     */
    struct stat attr;
    int64_t attr_timeout_secs = -1;
    int64_t attr_timeout_timestamp = -1;

    // nfs_client owning this inode.
    struct nfs_client *const client;

    /*
     * Pointer to the readdirectory cache.
     * Only valid for a directory, this will be nullptr for a non-directory.
     */
    std::shared_ptr<readdirectory_cache> dircache_handle;
    
    /**
     * TODO: Initialize attr with postop attributes received in the RPC
     *       response.
     */
    nfs_inode(const struct nfs_fh3 *filehandle,
              struct nfs_client *_client,
              uint32_t _file_type,
              fuse_ino_t _ino = 0);

    ~nfs_inode();

    /**
     * Increment lookupcnt of the inode.
     */
    void incref() const
    {
        lookupcnt++;

        AZLogDebug("ino {} lookupcnt incremented to {}",
                   ino, (int) lookupcnt.load());
    }

    /**
     * Decrement lookupcnt of the inode and delete it if lookupcnt
     * reaches 0.
     */
    void decref()
    {
        assert(lookupcnt > 0);

        if (--lookupcnt == 0) {
            AZLogDebug("ino {} lookupcnt decremented to 0, freeing inode",
                       ino, lookupcnt.load());
            delete this;
        } else {
            AZLogDebug("ino {} lookupcnt decremented to {}",
                       ino, lookupcnt.load());
        }
    }

    nfs_client *get_client() const
    {
        assert(client != nullptr);
        return client;
    }

    const struct nfs_fh3& get_fh() const
    {
        return fh;
    }

    bool is_dir() const
    {
        return (file_type == S_IFDIR);
    }

    /**
     * Get the minimum attribute cache timeout value in seconds, to be used
     * for this file.
     */
    int get_actimeo_min() const;

    /**
     * Get the maximum attribute cache timeout value in seconds, to be used
     * for this file.
     */
    int get_actimeo_max() const;

    /**
     * Revalidate the inode.
     * Revalidation is done by querying the inode attributes from the server
     * and comparing them against the saved attributes. If the freshly fetched
     * attributes indicate "change in file/dir content" by indicators such as
     * mtime and/or size, then we invalidate the cached data of the inode.
     * If 'force' is false then inode attributes are fetched only if the last
     * fetched attributes are older than attr_timeout_secs, while if 'force'
     * is true we fetch the attributes regardless. This could f.e., be needed
     * when a file/dir is opened (for close-to-open consistency reasons).
     * Other reasons for force invalidating the caches could be if file/dir
     * was updated by calls to write()/create()/rename().
     *
     * This holds the inode lock.
     */
    void revalidate(bool force = false);

    /**
     * Update the inode given that we have received fresh attributes from
     * the server. These fresh attributes could have been received as
     * postop attributes to any of the requests or it could be a result of
     * explicit GETATTR call that we make from revalidate() when the attribute
     * cache times out.
     * We process the freshly received attributes as follows:
     * - If the ctime has not changed, then the file has not changed, and
     *   we don't do anything, else
     * - If mtime has changed then the file data and metadata has changed
     *   and we need to drop the caches and update nfs_inode::attr, else
     * - If just ctime has changed then only the file metadata has changed
     *   and we update nfs_inode::attr from the received attributes.
     *
     * Returns true if 'fattr' is newer than the cached attributes.
     *
     * Caller must hold the inode lock.
     */
    bool update_nolock(const struct fattr3& fattr);

    /**
     * Convenience function that calls update_nolock() after holding the
     * inode lock.
     *
     * XXX This MUST be called whenever we get fresh attributes for a file,
     *     most commonly as post-op attributes along with some RPC response.
     */
    bool update(const struct fattr3& fattr)
    {
        std::unique_lock<std::shared_mutex> lock(ilock);
        return update_nolock(fattr);
    }

    /**
     * Invalidate/zap the cached data.
     * Depending on whether this inode corresponds to a regular file or a
     * directory, this will invalidate the appropriate cache.
     *
     * Caller must hold the inode lock.
     */
    void invalidate_cache_nolock();

    /**
     * Convenience function that calls invalidate_cache_nolock() after
     * holding the inode lock.
     */
    void invalidate_cache()
    {
        std::unique_lock<std::shared_mutex> lock(ilock);
        invalidate_cache_nolock();
    }

    /**
     * Caller must hold the inode lock.
     */
    void purge_dircache();

    /*
     * This function populates the \p results vector by fetching the entries from
     * readdirectory cache starting at offset \p cookie upto size \p max_size.
     */
    void lookup_readdircache(
        cookie3 cookie /* offset in the directory from which the directory should be listed*/,
        size_t max_size /* maximum size of entries to be returned*/,
        std::vector<directory_entry* >& results /* dir entries listed*/,
        bool& eof,
        bool skip_attr_size = false);

    bool make_getattr_call(struct fattr3& attr);
};
#endif /* __NFS_INODE_H__ */
