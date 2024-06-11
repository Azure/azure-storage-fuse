#ifndef __NFS_INODE_H__
#define __NFS_INODE_H__

#include "aznfsc.h"
#include "rpc_readdir.h"

#define NFS_INODE_MAGIC *((const uint32_t *)"NFSI")

/**
 * This is the NFS inode structure. There is one of these per file
 * and contains any global information about the file., f.e.,
 * - NFS filehandle for accessing the file.
 * - FUSE inode number of the file.
 * - File/Readahead cache (if any).
 * - Anything else that we want to maintain per file.
 */
struct nfs_inode
{
    const uint32_t magic = NFS_INODE_MAGIC;

    // NFSv3 filehandle returned by the server.
    nfs_fh3 fh;

    // Fuse inode number.
    fuse_ino_t ino;

    struct nfs_client *const client;

    // Pointer to the readdirectory cache.
    std::shared_ptr<readdirectory_cache> dircache_handle;
    
    nfs_inode(const struct nfs_fh3 *filehandle, struct nfs_client *_client):
        ino(0),
         client(_client)
    {
        // Sanity assert.
        assert(filehandle->data.data_len > 50 &&
               filehandle->data.data_len <= 64);

        fh.data.data_len = filehandle->data.data_len;
        fh.data.data_val = new char[fh.data.data_len];
        ::memcpy(fh.data.data_val, filehandle->data.data_val, fh.data.data_len);
        dircache_handle = std::make_shared<readdirectory_cache>();
    }

    nfs_client *get_client() const
    {
        assert (client != nullptr);
        return client;
    }

    void set_inode(fuse_ino_t _ino)
    {
        // ino is either set to FUSE_ROOT_ID or set to address of nfs_inode.
        assert((_ino == (fuse_ino_t) this) || (_ino == FUSE_ROOT_ID));

        ino = _ino;
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
