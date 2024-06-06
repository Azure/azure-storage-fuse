#ifndef __NFS_INODE_H__
#define __NFS_INODE_H__

#include "aznfsc.h"

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

    // Fuse inode number/
    fuse_ino_t ino;

    // TODO: Add blob info structure.

    nfs_inode(const struct nfs_fh3 *filehandle):
        ino(0)
    {
        // Sanity assert.
        assert(filehandle->data.data_len > 50 &&
               filehandle->data.data_len <= 64);

        fh.data.data_len = filehandle->data.data_len;
        fh.data.data_val = new char[fh.data.data_len];
        ::memcpy(fh.data.data_val, filehandle->data.data_val, fh.data.data_len);
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
};
#endif /* __NFS_INODE_H__ */
