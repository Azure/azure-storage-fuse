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

/**
 * C++ object to hold struct nfs_fh3 from libnfs.
 */
struct nfs_fh3_deep
{
    nfs_fh3_deep(const struct nfs_fh3& _fh)
    {
#ifndef ENABLE_NON_AZURE_NFS
        // Blob NFS FH is at least 50 bytes.
        assert(_fh.data.data_len > 50 && _fh.data.data_len <= 64);
#else
        assert(_fh.data.data_len <= 64);
#endif
        fh.data.data_len = _fh.data.data_len;
        fh.data.data_val = &fh_data[0];
        ::memcpy(fh.data.data_val, _fh.data.data_val, fh.data.data_len);
    }

    /**
     * Return the libnfs nfs_fh3 object ref.
     */
    const struct nfs_fh3& get_fh() const
    {
        assert(FH_VALID(&fh));
        return fh;
    }

private:
    struct nfs_fh3 fh;
    char fh_data[64];
};

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
     * VFS can make multiple calls (not writes) to the same file in parallel.
     */
    mutable std::shared_mutex ilock_1;

    /*
     * S_IFREG, S_IFDIR, etc.
     * 0 is not a valid file type.
     */
    const uint32_t file_type = 0;

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

private:
    /*
     * NFSv3 filehandle returned by the server.
     * We use this to identify this file/directory to the server.
     */
    const nfs_fh3_deep fh;

    /*
     * CRC32 hash of fh.
     * This serves multiple purposes, most importantly it can be used to print
     * filehandle hashes in a way that can be used to match with wireshark.
     * Also used for affining writes to a file to one RPC transport.
     */
    const uint32_t crc = 0;

    /*
     * This is a handle to the chunk cache which caches data for this file.
     * Valid only for regular files.
     * filecache_handle starts null in the nfs_inode constructor and is later
     * initialized only in on_fuse_open() (when we return the inode to fuse in
     * a lookup response or the application calls open()/creat()). The idea is
     * to allocate the cache only when really needed. For inodes returned to
     * fuse in a readdirplus response we don't initialize the filecache_handle.
     * Once initialized we never make it null again, though we can make the
     * cache itself empty by invalidate_cache(). So if has_filecache() returns
     * true we can safely access the filecache_handle shared_ptr returned by
     * get_filecache().
     * alloc_filecache() initializes filecache_handle and sets filecache_alloced
     * to true.
     * Access to this shared_ptr must be protect by ilock_1, whereas access to
     * the bytes_chunk_cache itself must be protected by chunkmap_lock_43.
     */
    std::shared_ptr<bytes_chunk_cache> filecache_handle;
    std::atomic<bool> filecache_alloced = false;

    /*
     * Pointer to the readdirectory cache.
     * Only valid for a directory, this will be nullptr for a non-directory.
     * Access to this shared_ptr must be protect by ilock_1, whereas access to
     * the readdirectory_cache itself must be protected by readdircache_lock_2.
     * Also see comments above filecache_handle.
     */
    std::shared_ptr<readdirectory_cache> dircache_handle;
    std::atomic<bool> dircache_alloced = false;

    /*
     * For maintaining readahead state.
     * Valid only for regular files.
     * Access to this shared_ptr must be protect by ilock_1, whereas access to
     * the ra_state itself must be protected by ra_lock_40.
     * Also see comments above filecache_handle.
     */
    std::shared_ptr<ra_state> readahead_state;
    std::atomic<bool> rastate_alloced = false;

public:
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
     * fetched preop or postop attributes to see if file/dir has changed
     * (and thus the cache must be invalidated).
     *
     * Note: Update and access it under ilock_1.
     * TODO: Audit all places where we access attr and make sure it's done
     *       under ilock_1.
     */
    struct stat attr;

    /*
     * attr_timeout_secs is protected by ilock_1.
     * attr_timeout_timestamp is updated inder ilock_1, but can be accessed
     * w/o ilock_1, f.e., run_getattr()->attr_cache_expired().
     */
    int64_t attr_timeout_secs = -1;
    std::atomic<int64_t> attr_timeout_timestamp = -1;

    /*
     * Time in usecs we received the last cached write for this inode.
     * See discussion in stamp_cached_write() for details.
     */
    std::atomic<int64_t> last_cached_write = 0;

    // nfs_client owning this inode.
    struct nfs_client *const client;

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
     * Does this nfs_inode have cache allocated?
     * It correctly checks cache for both directory and file inodes and it
     * only checks if the cache is allocated and not whether cache has some
     * data.
     *
     * Note: Only files/directories which are open()ed will have cache
     *       allocated, also since directory cache doubles as DNLC, for
     *       directories if at least one file/subdir inside this directory is
     *       looked up by fuse, the cache will be allocated.
     *
     * LOCKS: None.
     */
    bool has_cache() const
    {
        if (is_dir()) {
            return has_dircache();
        } else if (is_regfile()) {
            return has_filecache();
        }

        return false;
    }

    /**
     * Is the inode cache (filecache_handle or dircache_handle) empty?
     *
     * Note: This returns the current inode cache status at the time of this
     *       call, it my change right after this function returns. Keep this
     *       in mind when using the result.
     *
     * LOCKS: None.
     */
    bool is_cache_empty() const
    {
        if (is_regfile()) {
            return !has_filecache() || filecache_handle->is_empty();
        } else if (is_dir()) {
            return !has_dircache() || dircache_handle->is_empty();
        } else {
            return true;
        }
    }

    /**
     * Allocate file cache if not already allocated.
     * This must be called from code that returns an inode after a regular
     * file is opened or created.
     * It's a no-op if the filecache is already allocated.
     *
     * LOCKS: If not already allocated it'll take exclusive ilock_1.
     */
    void alloc_filecache()
    {
        assert(is_regfile());

        if (filecache_alloced) {
            // Once allocated it cannot become null again.
            assert(filecache_handle);
            return;
        }

        std::unique_lock<std::shared_mutex> lock(ilock_1);
        if (!filecache_handle) {
            assert(!filecache_alloced);

            if (aznfsc_cfg.filecache.enable && aznfsc_cfg.filecache.cachedir) {
                const std::string backing_file_name =
                    std::string(aznfsc_cfg.filecache.cachedir) + "/" + std::to_string(get_fuse_ino());
                filecache_handle =
                    std::make_shared<bytes_chunk_cache>(this, backing_file_name.c_str());
            } else {
                filecache_handle = std::make_shared<bytes_chunk_cache>(this);
            }
            filecache_alloced = true;
        }
    }

    /**
     * This MUST be called only after has_filecache() returns true, else
     * there's a possibility of data race, as the returned filecache_handle
     * ref may be updated by alloc_filecache() right after get_filecache()
     * returns and while the caller is accessing the shared_ptr.
     * So f.e., calling "if (get_filecache())" to check presence of cache is
     * not safe as get_filecache() is being used as a boolean here so it calls
     * "shared_ptr::operator bool()" which returns true even while the
     * shared_ptr is being initialized by alloc_filecache(), thus it causes
     * a data race.
     * Once filecache_handle is allocated by alloc_filecache() it remains set
     * for the life of the inode, so we can safely use the shared_ptr w/o the
     * inode lock.
     *
     * Note: This MUST be called only when has_filecache() returns true.
     *
     * LOCKS: None.
     */
    std::shared_ptr<bytes_chunk_cache>& get_filecache()
    {
        assert(is_regfile());
        assert(filecache_alloced);
        assert(filecache_handle);

        return filecache_handle;
    }

    /**
     * External users of this nfs_inode can check for presence of filecache by
     * calling has_filecache().
     *
     * LOCKS: None.
     */
    bool has_filecache() const
    {
        assert(is_regfile());
        assert(!filecache_alloced || filecache_handle);

        return filecache_alloced;
    }

    /**
     * Allocate directory cache if not already allocated.
     * This must be called from code that returns an inode after a directory
     * is opened or created.
     * It's a no-op if the dircache is already allocated.
     *
     * LOCKS: If not already allocated it'll take exclusive ilock_1.
     */
    void alloc_dircache(bool newly_created_directory = false)
    {
        assert(is_dir());

        if (dircache_alloced) {
            // Once allocated it cannot become null again.
            assert(dircache_handle);
            return;
        }

        std::unique_lock<std::shared_mutex> lock(ilock_1);
        if (!dircache_handle) {
            assert(!dircache_alloced);

            dircache_handle = std::make_shared<readdirectory_cache>(client, this);
            /*
             * If this directory is just created, mark it as "confirmed".
             */
            if (newly_created_directory) {
                dircache_handle->set_confirmed();
            }

            dircache_alloced = true;
        }
    }

    /**
     * This MUST be called only after has_dircache() returns true.
     * See comment above get_filecache().
     *
     * Note: This MUST be called only when has_dircache() returns true.
     *
     * LOCKS: None.
     */
    std::shared_ptr<readdirectory_cache>& get_dircache()
    {
        assert(is_dir());
        assert(dircache_alloced);
        assert(dircache_handle);

        return dircache_handle;
    }

    /**
     * External users of this nfs_inode can check for presence of dircache by
     * calling has_dircache().
     *
     * LOCKS: None.
     */
    bool has_dircache() const
    {
        assert(is_dir());
        assert(!dircache_alloced || dircache_handle);

        return dircache_alloced;
    }

    /**
     * Allocate readahead_state if not already allocated.
     * This must be called from code that returns an inode after a directory
     * is opened or created.
     * It's a no-op if the rastate is already allocated.
     *
     * LOCKS: If not already allocated it'll take exclusive ilock_1.
     */
    void alloc_rastate()
    {
        assert(is_regfile());

        if (rastate_alloced) {
            // Once allocated it cannot become null again.
            assert(readahead_state);
            return;
        }

        std::unique_lock<std::shared_mutex> lock(ilock_1);
        /*
         * readahead_state MUST only be created if filecache_handle is set.
         */
        assert(filecache_handle);
        if (!readahead_state) {
            assert(!rastate_alloced);
            readahead_state = std::make_shared<ra_state>(client, this);
            rastate_alloced = true;
        }
    }

    /**
     * This MUST be called only after has_rastate() returns true.
     * See comment above get_filecache().
     *
     * Note: This MUST be called only when has_rastate() returns true.
     *
     * LOCKS: None.
     */
    const std::shared_ptr<ra_state>& get_rastate() const
    {
        assert(is_regfile());
        assert(rastate_alloced);
        assert(readahead_state);

        return readahead_state;
    }

    std::shared_ptr<ra_state>& get_rastate()
    {
        assert(is_regfile());
        assert(rastate_alloced);
        assert(readahead_state);

        return readahead_state;
    }

    /**
     * External users of this nfs_inode can check for presence of dircache by
     * calling has_dircache().
     *
     * LOCKS: None.
     */
    bool has_rastate() const
    {
        assert(is_regfile());
        assert(!rastate_alloced || readahead_state);

        return rastate_alloced;
    }

    /**
     * This must be called from all paths where we respond to a fuse request
     * that amounts to open()ing a file/directory. Once a file/directory is
     * open()ed, application can call all the POSIX APIs that take an fd, so if
     * we defer anything in the nfs_inode constructor (as we are not sure if
     * application will call any POSIX API on the file) perform the allocation
     * here.
     *
     * LOCKS: Exclusive ilock_1.
     */
    void on_fuse_open(enum fuse_opcode optype)
    {
        /*
         * Only these fuse ops correspond to open()/creat() which return an
         * fd.
         */
        assert((optype == FUSE_CREATE) ||
               (optype == FUSE_OPEN) ||
               (optype == FUSE_OPENDIR));

        opencnt++;

        if (is_regfile()) {
            /*
             * Allocate filecache_handle after readahead_state as we assert
             * for filecache_handle in alloc_rastate().
             */
            alloc_filecache();
            alloc_rastate();
        } else if (is_dir()) {
            alloc_dircache();
        }
    }

    /**
     * This must be called from all paths where we respond to a fuse request
     * that makes fuse aware of this inode. It could be lookup or readdirplus.
     * Once fuse receives an inode it can call operations like lookup/getattr.
     * See on_fuse_open() which is called by paths which not only return the inode
     * but also an fd to the application, f.e. creat().
     *
     * LOCKS: Exclusive ilock_1.
     */
    void on_fuse_lookup(enum fuse_opcode optype)
    {
        /*
         * Only these fuse ops correspond to operations that return an inode
         * to fuse, but don't cause a fd to be returned to the application.
         * FUSE_READDIR and FUSE_READDIRPLUS are the only other ops that return
         * inode to fuse but we don't call on_fuse_lookup() for those as they
         * could be a lot and most commonly applications will not perform IO
         * on all files returned by readdir/readdirplus.
         */
        assert((optype == FUSE_LOOKUP) ||
               (optype == FUSE_MKNOD) ||
               (optype == FUSE_MKDIR) ||
               (optype == FUSE_SYMLINK));

        if (is_regfile()) {
            assert(optype == FUSE_LOOKUP ||
                   optype == FUSE_MKNOD);
        } else if (is_dir()) {
            assert(optype == FUSE_LOOKUP ||
                   optype == FUSE_MKDIR);
            /*
             * We have a unified cache for readdir/readdirplus and lookup, so
             * we need to create the readdir cache on lookup.
             */
            alloc_dircache(optype == FUSE_MKDIR);
        }
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
     *
     * LOCKS: None.
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
     *
     * Note: Shared readdircache_lock_2.
     */
    struct nfs_inode *dnlc_lookup(const char *filename,
                                  bool *negative_confirmed = nullptr) const
    {
        assert(is_dir());

        if (has_dircache()) {
            struct nfs_inode *inode =
                dircache_handle->dnlc_lookup(filename, negative_confirmed);
            // dnlc_lookup() must have held a lookupcnt ref.
            assert(!inode || inode->lookupcnt > 0);

            return inode;
        }

        return nullptr;
    }

    /**
     * Add DNLC entry "filename -> inode".
     */
    void dnlc_add(const char *filename, struct nfs_inode *inode)
    {
        assert(filename);
        assert(inode);
        assert(inode->magic == NFS_INODE_MAGIC);
        assert(is_dir());

        /*
         * Directory inodes returned by READDIRPLUS won't have dircache
         * allocated, and fuse may call lookup on them, allocate dircache now
         * before calling dnlc_add().
         */
        alloc_dircache();

        dircache_handle->dnlc_add(filename, inode);
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

        AZLogDebug("[{}] lookupcnt incremented to {} (dircachecnt: {}, "
                   "forget_expected: {})",
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
        return fh.get_fh();
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
     * EAGAIN is the special error code that would mean that caller must retry
     * the current copy_to_cache() call.
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
     * till all dirty data is sync'ed with the NFS server. Only dirty data
     * in the given range is flushed if provided, else all dirty data is
     * flushed.
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
    int flush_cache_and_wait(uint64_t start_off = 0,
                             uint64_t end_off = UINT64_MAX);

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
     * LOCKS: If revalidating it'll take exclusive ilock_1.
     */
    void revalidate(bool force = false);

    /**
     * Update the inode given that we have received fresh attributes from
     * the server. These fresh attributes could have been received as
     * postop (and preop) attributes to any of the requests or it could be a
     * result of explicit GETATTR call that we make from revalidate() when the
     * attribute cache times out.
     * We process the freshly received attributes as follows:
     * - If the ctime has not changed, then the file has not changed, and
     *   we don't do anything, else
     * - If mtime has changed then the file data and metadata has changed
     *   and we need to drop the caches and update nfs_inode::attr, else
     * - If just ctime has changed then only the file metadata has changed
     *   and we update nfs_inode::attr from the received attributes.
     *
     * Returns true if preattr/postattr indicate that file has changed (either
     * metadata, or both) since we cached it, false indicates that file has not
     * changed.
     *
     * LOCKS: Caller must take exclusive ilock_1.
     */
    bool update_nolock(const struct fattr3 *postattr,
                       const struct wcc_attr *preattr = nullptr);

    /**
     * Convenience function that calls update_nolock() after holding the
     * inode lock.
     *
     * LOCKS: Exclusive ilock_1.
     *
     * XXX This MUST be called whenever we get fresh attributes for a file,
     *     most commonly as post-op attributes along with some RPC response.
     */
    bool update(const struct fattr3 *postattr,
                const struct wcc_attr *preattr = nullptr)
    {
        std::unique_lock<std::shared_mutex> lock(ilock_1);
        return update_nolock(postattr, preattr);
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
        std::unique_lock<std::shared_mutex> lock(ilock_1);
        force_update_attr_nolock(fattr);
    }

    /**
     * Invalidate/zap the cached data. This will correctly invalidate cached
     * data for both file and directory caches.
     * By default it will just mark the cache as invalid and the actual purging
     * will be deferred till the next access to the cache, and will be done in
     * the context that accesses the cache, but the caller can request the cache
     * to be purged inline by passing purge_now as true.
     *
     * LOCKS: None when purge_now is false.
     *        When purge_now is true, exclusive chunkmap_lock_43 for files and
     *        exclusive readdircache_lock_2 for directories.
     */
    void invalidate_cache(bool purge_now = false)
    {
        if (is_dir()) {
            if (has_dircache()) {
                assert(dircache_handle);
                AZLogDebug("[{}] Invalidating dircache", get_fuse_ino());
                dircache_handle->invalidate();

                if (purge_now) {
                    AZLogDebug("[{}] (Purgenow) Purging dircache", get_fuse_ino());
                    dircache_handle->clear();
                    AZLogDebug("[{}] (Purgenow) Purged dircache", get_fuse_ino());
                }
            }
        } else if (is_regfile()) {
            if (has_filecache()) {
                assert(filecache_handle);
                AZLogDebug("[{}] Invalidating filecache", get_fuse_ino());
                filecache_handle->invalidate();

                if (purge_now) {
                    AZLogDebug("[{}] (Purgenow) Purging filecache", get_fuse_ino());
                    filecache_handle->clear();
                    AZLogDebug("[{}] (Purgenow) Purged filecache", get_fuse_ino());
                }
            }
        }
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
     * Directory cache lookup method.
     *
     * cookie: offset in the directory from which the entries should be listed.
     * max_size: do not return entries more than these many bytes.
     * results: returned entries are populated in this vector. Each of these
     *          entry has a shared_ptr ref held so they can be safely used even
     *          if the actual directory_entry in readdirectory_cache is deleted.
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
        std::vector<std::shared_ptr<const directory_entry>>& results,
        bool& eof,
        bool readdirplus);
};
#endif /* __NFS_INODE_H__ */
