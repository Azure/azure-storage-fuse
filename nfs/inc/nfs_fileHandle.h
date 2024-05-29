#pragma once
#include "aznfsc.h"

struct nfs_file_handle
{
    // Nfs filehandle returned by the server.
    nfs_fh3 fh;

    // TODO: Add blob info structure.

    //inode_number;
    fuse_ino_t inode;

    nfs_file_handle(const struct nfs_fh3* filehandle):
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
