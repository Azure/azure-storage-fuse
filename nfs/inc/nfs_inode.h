#ifndef __NFS_INODE_H__
#define __NFS_INODE_H__

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
