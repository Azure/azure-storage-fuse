#ifndef __NFS_INODE_H__
#define __NFS_INODE_H__

#include "aznfsc.h"

#define NFS_INODE_MAGIC *((const uint32_t *)"NFSI")

struct nfs_inode
{
    const uint32_t magic = NFS_INODE_MAGIC;

    // Nfs filehandle returned by the server.
    nfs_fh3 fh;

    // TODO: Add blob info structure.

    //inode_number;
    fuse_ino_t inode;

    nfs_inode(const struct nfs_fh3* filehandle):
        inode(0)
    {
        fh.data.data_val = new char[filehandle->data.data_len];
        fh.data.data_len = filehandle->data.data_len;
        ::memcpy(fh.data.data_val, filehandle->data.data_val, fh.data.data_len);
    }

    void set_inode(fuse_ino_t ino)
    {
        inode = ino;
    }

    const struct nfs_fh3& get_fh() const
    {
        return fh;
    }
};
#endif /* __NFS_INODE_H__ */
