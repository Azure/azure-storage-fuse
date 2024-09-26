#ifndef __NFS_INODE_H__
#define __NFS_INODE_H__

#include <atomic>
#include "aznfsc.h"
#include "rpc_readdir.h"
#include "file_cache.h"
#include "readahead.h"

#define NFS_INODE_MAGIC *((const uint32_t *)"NFSI")

// Compare two nfs_fh3 filehandles.
#define FH_EQUAL(fh1, fh2) \
    (((fh1)->data.data_len == (fh2)->data.data_len) && \
     (!::memcmp((fh1)->data.data_val, \
                (fh2)->data.data_val, \
                (fh1)->data.data_len)))

#define FH_VALID(fh) \
    (((fh)->data.data_len > 0) && ((fh)->data.data_val != nullptr))

// *fh1 = *fh2
#define FH_COPY(fh1, fh2) \
do { \
    /* Don't overwrite a valid fh, leaking memory */ \
    assert(!FH_VALID(fh1)); \
    assert(FH_VALID(fh2)); \
    (fh1)->data.data_len = (fh2)->data.data_len; \
    (fh1)->data.data_val = (char *) ::malloc((fh1)->data.data_len); \
    ::memcpy((fh1)->data.data_val, (fh2)->data.data_val, (fh1)->data.data_len); \
} while (0)

#define FH_FREE(fh) \
do { \
    ::free((fh)->data.data_val); \
    (fh)->data.data_val = nullptr; \
    (fh)->data.data_len = 0; \
} while (0)

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
     *
     * TODO: See if we need this lock or fuse vfs will cover for this.
     */
    mutable std::shared_mutex ilock;

    /*
     * Ref count of this inode.
     * Fuse expects that whenever we make one of the following calls, we
     * must increment the lookupcnt of the inode:
     * - fuse_reply_entry()
     * - fuse_reply_create()
     * - Lookup count of every entry returned by readdirplus(), except "."
     *   and "..", is incremented by one. Note that readdir() does not
     *   affect the lookup count of any of the entries returned.
     *
     * Since an nfs_inode is created only in response to one of the above,
     * we set the lookupcnt to 1 when the nfs_inode is created. Later if
     * we are not able to successfully convey creation of the inode to fuse
     * we drop the ref. This is important as unless fuse knows about an
     * inode it'll never call forget() for it and we will leak the inode.
     * forget() causes lookupcnt for an inode to be reduced by the "nlookup"
     * parameter count. forget_multi() does the same for multiple inodes in
     * a single call.
     * On umount the lookupcnt for all inodes implicitly drops to zero, and
     * fuse may not call forget() for the affected inodes.
     *
     * Till the lookupcnt of an inode drops to zero, we MUST not free the
     * nfs_inode structure, as kernel may send requests for files with
     * non-zero lookupcnt, even after calls to unlink(), rmdir() or rename().
     *
     * dircachecnt is another refcnt which is the number of readdirplus
     * directory_entry,s that refer to this nfs_inode. An inode can only be
     * deleted when both lookupcnt and dircachecnt become 0, i.e., fuse
     * vfs does not have a reference to the inode and it's not cached in
     * any of our readdirectory_cache,s.
     *
     * See comment above inode_map.
     */
    mutable std::atomic<uint64_t> lookupcnt = 0;
    mutable std::atomic<uint64_t> dircachecnt = 0;

    /*
     * How many open fds for this file are currently present in fuse.
     * Incremented when fuse calls open()/creat().
     */
    std::atomic<uint64_t> opencnt = 0;

    /*
     * Silly rename related info.
     * If this inode has been successfully silly renamed, is_silly_renamed will
     * be set and silly_renamed_name will contain the silly renamed name and
     * parent_ino is the parent directory ino. These will be needed for
     * deleting ths silly renamed file once the last handle on the file is
     * closed by user.
     * silly_rename_level helps to get unique names in case the silly renamed
     * file itself is deleted.
     */
    bool is_silly_renamed = false;
    std::string silly_renamed_name;
    fuse_ino_t parent_ino = 0;
    int silly_rename_level = 0;

    /*
     * NFSv3 filehandle returned by the server.
     * We use this to identify this file/directory to the server.
     */
    nfs_fh3 fh = {0, nullptr};

    /*
     * CRC32 hash of fh.
     */
    uint32_t crc = 0;

    /*
     * Fuse inode number.
     * This is how fuse identifies this file/directory to us.
     * Fuse expects us to ensure that if we reuse ino we must ensure that the
     * ino/generation pair is unique for the life of the fuse filesystem (and
     * not just unique for one mount). This is specially useful if this fuse
     * filesystem is exported over NFS. Since NFS would issue filehandles
     * based on the ino number and generation pair, if ino number and generation
     * pair is not unique NFS server might issue the same FH to two different
     * files if "fuse driver + NFS server" is restarted. To avoid that make
     * sure generation id is unique. We use the current epoch in usecs to
     * ensure uniqueness. Note that even if the time goes back, it's highly
     * unlikely that we use the same ino number and usec combination, but
     * it's technically possible.
     *
     * IMPORTANT: Need to ensure time is sync'ed and it doesn't go back.
     */
    const fuse_ino_t ino;
    const uint64_t generation;

    /*
     * S_IFREG, S_IFDIR, etc.
     * 0 is not a valid file type.
     */
    const uint32_t file_type = 0;

    /*
     * Cached attributes for this inode and the current value of attribute
     * cache timeout. attr_timeout_secs will have a value between
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
     *
     * See update_nolock() how these attributes are compared with freshly
     * fetched attributes to see if inode has changed.
     *
     * Note: Update and access it under ilock.
     */
    struct stat attr;
    int64_t attr_timeout_secs = -1;
    int64_t attr_timeout_timestamp = -1;

    /*
     * Time in usecs we received the last cached write for this inode.
     * See discussion in stamp_cached_write() for details.
     */
    std::atomic<int64_t> last_cached_write = 0;

    // nfs_client owning this inode.
    struct nfs_client *const client;

    /*
     * Pointer to the readdirectory cache.
     * Only valid for a directory, this will be nullptr for a non-directory.
     */
    std::shared_ptr<readdirectory_cache> dircache_handle;

    /*
     * This is a handle to the chunk cache which caches data for this file.
     * Valid only for regular files.
     */
    std::shared_ptr<bytes_chunk_cache> filecache_handle;

    /*
     * For maintaining readahead state.
     * Valid only for regular files.
     */
    std::shared_ptr<ra_state> readahead_state;

    /*
     * How many forget count we expect from fuse.
     * It'll be incremented whenever we are able to successfully call one of
     * the following:
     * - fuse_reply_create()
     * - fuse_reply_entry()
     * - fuse_reply_buf() (for readdirplus and not for readdir)
     *
     * Fuse must call exactly these many forgets on this inode and the inode
     * can only be freed when forget_expected becomes 0. Fuse must not call
     * more forgets than forget_expected.
     *
     * Note: forget_expected may become 0 indicating that fuse doesn't know
     *       about this inode but inode may still be in use (lookupcnt or
     *       dircachecnt can be non-zero), then we don't free the inode.
     *
     * We use this for forgetting all inodes on unmount, and also for
     * debugging to see if fuse forgets to call forget :-)
     */
    std::atomic<int64_t> forget_expected = 0;

#ifdef ENABLE_PARANOID
    uint64_t last_forget_seen_usecs = 0;
#endif

    /*
     * Stores the write error observed when performing backend writes to this
     * Blob. This helps us duly fail close(), if one or more IOs have failed
     * for the Blob. Note that the application read may complete immediately
     * after copying the data to the cache but later when sync'ing dirty
     * membufs with the Blob we might encounter write failures. These failures
     * MUST be conveyed to the application via close(), else it'll never know.
     *
     * This is either 0 (no error) or a +ve errno value.
     */
    int write_error = 0;

    /**
     * TODO: Initialize attr with postop attributes received in the RPC
     *       response.
     */
    nfs_inode(const struct nfs_fh3 *filehandle,
              const struct fattr3 *fattr,
              struct nfs_client *_client,
              uint32_t _file_type,
              fuse_ino_t _ino = 0);

    ~nfs_inode();

    /**
     * Allocate file cache if not already allocated.
     * This must be called from code that returns an inode after a regular
     * file is opened or created.
     */
    std::shared_ptr<bytes_chunk_cache>& get_or_alloc_filecache()
    {
        assert(is_regfile());
        {
            std::shared_lock<std::shared_mutex> lock(ilock);
            if (filecache_handle)
                return filecache_handle;
        }

        std::unique_lock<std::shared_mutex> lock(ilock);
        if (!filecache_handle) {
            if (aznfsc_cfg.filecache.enable && aznfsc_cfg.filecache.cachedir) {
                const std::string backing_file_name =
                    std::string(aznfsc_cfg.filecache.cachedir) + "/" + std::to_string(get_fuse_ino());
                filecache_handle =
                    std::make_shared<bytes_chunk_cache>(this, backing_file_name.c_str());
            } else {
                filecache_handle = std::make_shared<bytes_chunk_cache>(this);
            }
        }

        return filecache_handle;
    }

    /**
     * Allocate directory cache if not already allocated.
     * This must be called from code that returns an inode after a directory
     * is opened or created.
     */
    std::shared_ptr<readdirectory_cache>& get_or_alloc_dircache()
    {
        assert(is_dir());
        {
            std::shared_lock<std::shared_mutex> lock(ilock);
            if (dircache_handle)
                return dircache_handle;
        }

        std::unique_lock<std::shared_mutex> lock(ilock);
        if (!dircache_handle) {
            dircache_handle = std::make_shared<readdirectory_cache>(client, this);
        }

        return dircache_handle;
    }

    /**
     * Allocate readahead_state if not already allocated.
     */
    std::shared_ptr<ra_state>& get_or_alloc_rastate()
    {
        assert(is_regfile());
        {
            std::shared_lock<std::shared_mutex> lock(ilock);
            if (readahead_state)
                return readahead_state;
        }

        std::unique_lock<std::shared_mutex> lock(ilock);
        if (!readahead_state) {
            readahead_state = std::make_shared<ra_state>(client, this);
        }

        return readahead_state;
    }

    /**
     * Return the fuse inode number for this inode.
     */
    fuse_ino_t get_fuse_ino() const
    {
        assert(ino != 0);
        return ino;
    }

    /**
     * Return the generation number for this inode.
     */
    uint64_t get_generation() const
    {
        assert(generation != 0);
        return generation;
    }

    int get_silly_rename_level()
    {
        return silly_rename_level++;
    }

    /**
     * Return the NFS fileid. This is also the inode number returned by
     * stat(2).
     */
    uint64_t get_fileid() const
    {
        assert(attr.st_ino != 0);
        return attr.st_ino;
    }

    /**
     * Checks whether inode->attr is expired as per the current actimeo.
     */
    bool attr_cache_expired() const
    {
        /*
         * This is set in the constructor as a newly created nfs_inode always
         * has attributes cached in nfs_inode::attr.
         */
        assert(attr_timeout_timestamp != -1);

        const int64_t now_msecs = get_current_msecs();
        const bool attr_expired = (attr_timeout_timestamp < now_msecs);

        return attr_expired;
    }

    /**
     * Get the estimated file size based on the cached attributes. Note that
     * this is based on cached attributes which might be old and hence the
     * size may not match the recent size, caller should use this just as an
     * estimate and should not use it for any hard failures that may be in
     * violation of the protocol.
     * If cached attributes have expired (as per the configured actimeo) then
     * it returns -1 and caller must handle it.
     */
    int64_t get_file_size() const
    {
        assert((size_t) attr.st_size <= AZNFSC_MAX_FILE_SIZE);
        return attr_cache_expired() ? -1 : attr.st_size;
    }

    /**
     * Check if [offset, offset+length) lies within the current RA window.
     * bytes_chunk_cache would call this to find out if a particular membuf
     * can be purged. Membufs in RA window would mostly be used soon and
     * should not be purged.
     * Note that it checks if there is any overlap and not whether it fits
     * entirely within the RA window.
     */
    bool in_ra_window(uint64_t offset, uint64_t length) const;

    /**
     * Is this file currently open()ed by any application.
     */
    bool is_open() const
    {
        return opencnt > 0;
    }

    /**
     * Return the nfs_inode corresponding to filename in the directory
     * represented by this inode.
     * It'll hold a lookupcnt ref on the returned inode and caller must drop
     * that ref by calling decref().
     */
    struct nfs_inode *dnlc_lookup(const char *filename) const
    {
        assert(is_dir());

        if (dircache_handle) {
            struct nfs_inode *inode = dircache_handle->dnlc_lookup(filename);
            // dnlc_lookup() must have held a lookupcnt ref.
            assert(!inode || inode->lookupcnt > 0);

            return inode;
        }

        return nullptr;
    }

    /*
     * Find nfs_inode for 'filename' in this directory.
     * It first searches in dnlc and if not found there makes a sync LOOKUP
     * call.
     * This calls revalidate().
     */
    struct nfs_inode *lookup(const char *filename);

    /**
     * Note usecs when the last cached write was received for this inode.
     * A cached write is not a direct application write but writes cached
     * by fuse kernel driver and then dispatched later as possibly bigger
     * writes. These have fi->writepage set.
     * We use this to decide if we need to no-op a setattr(mtime) call.
     * Note that fuse does not provide filesystems a way to convey "nocmtime",
     * i.e. fuse should not call setattr(mtime) to set file mtime during
     * cached write calls. Fuse will not call setattr(mtime) if we are not
     * using kernel cache as it expects the filesystem to manage mtime itself,
     * but if kernel cache is used fuse calls setattr(mtime) very often which
     * slows down the writes. Since our backing filesystem is NFS it'll take
     * care of updating mtime and hence we can ignore such setattr(mtime)
     * calls. To distinguish setattr(mtime) done as a result of writes from
     * ones that are done as a result of explicit utime() call by application,
     * we check if we have seen cached write recently.
     */
     void stamp_cached_write()
     {
         if (aznfsc_cfg.cache.data.kernel.enable) {
             last_cached_write = get_current_usecs();
         }
     }

     /**
      * Should we skip setattr(mtime) call for this inode?
      * See discussion above stamp_cached_write().
      */
     bool skip_mtime_update() const
     {
        static const int64_t one_sec = 1000 * 1000ULL;
        const int64_t now_usecs = get_current_usecs();
        const int64_t now_msecs = now_usecs / 1000ULL;
        const bool attrs_valid = (attr_timeout_timestamp >= now_msecs);
        /*
         * Kernel can be sending multiple writes/setattr in parallel over
         * multiple fuse threads, hence last_cached_write may be greater
         * than now_usecs.
         */
        const bool write_seen_recently =
            ((last_cached_write > now_usecs) ||
             ((now_usecs - last_cached_write) < one_sec));

        /*
         * We skip setattr(mtime) if we have seen a cached write in the last
         * one sec and if we have valid cached attributes for this inode.
         * Note that we need to return updated attributes in setattr response.
         */
        return (write_seen_recently && attrs_valid);
     }

    /**
     * Increment lookupcnt of the inode.
     */
    void incref() const
    {
        lookupcnt++;

        AZLogDebug("[{}] lookupcnt incremented to {} (dircachecnt={}, "
                   "forget_expected={})",
                   ino, lookupcnt.load(), dircachecnt.load(),
                   forget_expected.load());
    }

    /**
     * Decrement lookupcnt of the inode and delete it if lookupcnt
     * reaches 0.
     * 'cnt' is the amount by which the lookupcnt must be decremented.
     * This is usually the nlookup parameter passed by fuse FORGET, when
     * decref() is called from fuse FORGET, else it's 1.
     * 'from_forget' should be set to true when calling decref() for
     * handling fuse FORGET. Note that fuse FORGET is special as it
     * conveys important information about the inode. Since FORGET may
     * mean that fuse VFS does not have any reference to the inode, we can
     * use that to perform some imp tasks like, purging the readdir cache
     * for directory inodes. This is imp as it makes the client behave
     * like the kernel NFS client where flushing the cache causes the
     * directory cache to be flushed, and this can be a useful technique
     * in cases where NFS client is not being consistent with the server.
     */
    void decref(size_t cnt = 1, bool from_forget = false);

    /**
     * Returns true if inode is FORGOTten by fuse.
     * Forgotten inodes will not be referred by fuse in any api call.
     * Note that forgotten inodes may still hang around if they are
     * referenced by at least one directory_entry cache.
     */
    bool is_forgotten() const
    {
        return (lookupcnt == 0);
    }

    /**
     * Is this inode cached by any readdirectory_cache?
     */
    bool is_dircached() const
    {
        return (dircachecnt > 0);
    }

    nfs_client *get_client() const
    {
        assert(client != nullptr);
        return client;
    }

    const struct nfs_fh3& get_fh() const
    {
        assert(FH_VALID(&fh));
        return fh;
    }

    uint32_t get_crc() const
    {
        return crc;
    }

    bool is_dir() const
    {
        return (file_type == S_IFDIR);
    }

    // Is regular file?
    bool is_regfile() const
    {
        return (file_type == S_IFREG);
    }

    /**
     * Short character code for file_type, useful for logs.
     */
    char get_filetype_coding() const
    {
#ifndef ENABLE_NON_AZURE_NFS
        assert(file_type == S_IFDIR ||
               file_type == S_IFREG ||
               file_type == S_IFLNK);
#endif
        return (file_type == S_IFDIR) ? 'D' :
               ((file_type == S_IFLNK) ? 'S' :
                ((file_type == S_IFREG) ? 'R' : 'U'));
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
     * Get current attribute cache timeout value (in secs) for this inode.
     * Note that the attribute cache timeout moves between the min and max
     * values returned by the above methods, depending on whether the last
     * revalidation attempt was a success or not.
     */
    int get_actimeo() const
    {
        // If not set, return the min configured value.
        return (attr_timeout_secs != -1) ? attr_timeout_secs
                                         : get_actimeo_min();
    }


    /**
     * Is the inode cache (filecache_handle or dircache_handle) empty?
     */
    bool is_cache_empty() const
    {
        if (is_regfile()) {
            return !filecache_handle || filecache_handle->is_empty();
        } else if (is_dir()) {
            return !dircache_handle || dircache_handle->is_empty();
        } else {
            return true;
        }
    }

    /**
     * Copy application data into the inode's file cache.
     *
     * bufv: fuse_bufvec containing application data, passed by fuse.
     * offset: starting offset in file where the data should be written.
     * extent_left: after this copy what's the left edge of the longest dirty
     *              extent containing this latest write.
     * extent_right: after this copy what's the right edge of the longest dirty
     *               extent containing this latest write.
     * Caller can use the extent length information to decide if it wants to
     * dispatch an NFS write right now or wait and batch more, usually by
     * comparing it with the wsize value.
     *
     * Returns 0 if copy was successful, else a +ve errno value indicating the
     * error. This can be passed as-is to the rpc_task reply_error() method to
     * convey the error to fuse.
     *
     * Note: The membufs to which the data is copied will be marked dirty and
     *       uptodate once copy_to_cache() returns.
     */
    int copy_to_cache(const struct fuse_bufvec* bufv,
                      off_t offset,
                      uint64_t *extent_left,
                      uint64_t *extent_right);

    /**
     * Flush the dirty file cache represented by filecache_handle and wait
     * till all dirty data is sync'ed with the NFS server.
     * Note that filecache_handle is the only writeback cache that we have
     * and hence this only flushes that.
     * For a non-reg file inode this will be a no-op.
     * Returns 0 on success and a positive errno value on error.
     *
     * Note: This doesn't take the inode lock but instead it would grab the
     *       filecache_handle lock and get the list of dirty membufs at this
     *       instant and flush those. Any new dirty membufs added after it
     *       queries the dirty membufs list, are not flushed.
     */
    int flush_cache_and_wait();

    /**
     * Sync the dirty membufs in the file cache to the NFS server.
     * All contiguous dirty membufs are clubbed together and sent to the
     * NFS server in a single write call.
     */
    void sync_membufs(std::vector<bytes_chunk> &bcs, bool is_flush);

    /**
     * Called when last open fd is closed for a file.
     * release() will return true if the inode was silly renamed and it
     * initiated an unlink of the inode.
     */
    bool release(fuse_req_t req);

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
     * Force update inode->attr with fattr.
     * Unlike update_nolock() it doesn't invalidate the cache.
     * Use it when you know that cache need not be invalidated, as it's
     * already done.
     */
    void force_update_attr_nolock(const struct fattr3& fattr);

    void force_update_attr(const struct fattr3& fattr)
    {
        std::unique_lock<std::shared_mutex> lock(ilock);
        force_update_attr_nolock(fattr);
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
     * Caller must hold inode->ilock.
     */
    void purge_dircache_nolock();

    void purge_dircache()
    {
        std::unique_lock<std::shared_mutex> lock(ilock);
        purge_dircache_nolock();
    }

    /**
     * Store the first error encountered while writing dirty
     * membuf to Blob.
     */
    void set_write_error(int error)
    {
        assert(error > 0);

        if (this->write_error == 0) {
            this->write_error = error;
        }
    }

    /**
     * Returns the error, saved by prior call to set_write_error().
     */
    int get_write_error() const
    {
        return write_error;
    }

    /**
     * Caller must hold inode->ilock.
     */
    void purge_filecache_nolock();

    /**
     * Caller must hold the inode lock.
     * TODO: Implement this when we add read/write support.
     */
    void purge_filecache()
    {
        std::unique_lock<std::shared_mutex> lock(ilock);
        purge_filecache_nolock();
    }

    /**
     * Directory cache lookup method.
     *
     * cookie: offset in the directory from which the entries should be listed.
     * max_size: do not return entries more than these many bytes.
     * results: returned entries are populated in this vector.
     * eof: will be set if there are no more entries in the directory, after
     *      the last entry returned.
     * readdirplus: consumer of the returned directory entries is readdirplus.
     *              This will affect how the size of entries is added while
     *              comparing with max_size. If readdirplus is true, then we
     *              account for attribute size too, since readdirplus would
     *              be sending attributes too.
     */
    void lookup_dircache(
        cookie3 cookie,
        size_t max_size,
        std::vector<const directory_entry*>& results,
        bool& eof,
        bool readdirplus);
};
#endif /* __NFS_INODE_H__ */
