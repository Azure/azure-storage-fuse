#include "rpc_readdir.h"
#include "nfs_inode.h"

directory_entry::directory_entry(const char* name_,
                                 cookie3 cookie_,
                                 const struct stat& attr,
                                 nfs_inode* nfs_ino_):
    cookie(cookie_),
    attributes(attr),
    nfs_ino(nfs_ino_),
    name(name_)
{
    /*
     * While this directory_entry is allocated (and present in
     * readdirectory_cache) , we will return this inode to fuse,
     * so keep it allocated.
     * Note that fuse can call forget() for the inode, even though
     * we might have it in our cache.
     */
    nfs_ino->incref();
}

directory_entry::~directory_entry()
{
    // Drop the ref we held in the constructor.
    nfs_ino->decref();
}
