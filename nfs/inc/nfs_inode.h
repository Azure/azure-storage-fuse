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
     * This is how fuse identifiees this file/directory to us.
     */
    fuse_ino_t ino;

    // nfs_client owning this inode.
    struct nfs_client *const client;

    /*
     * Pointer to the readdirectory cache.
     * Only valid for a directory, this will be nullptr for a non-directory.
     */
    std::shared_ptr<readdirectory_cache> dircache_handle;
    
    nfs_inode(const struct nfs_fh3 *filehandle,
              struct nfs_client *_client,
              fuse_ino_t _ino = 0);

    ~nfs_inode();

    /**
     * Increment lookupcnt of the inode.
     */
    void incref() const
    {
        lookupcnt++;

        AZLogDebug("ino {} lookupcnt incremented to {}",
                   ino, lookupcnt.load());
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

    bool purge_readdircache_if_required();

    void purge();

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
