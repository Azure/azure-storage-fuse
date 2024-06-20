#include "rpc_readdir.h"
#include "nfs_inode.h"

directory_entry::directory_entry(const char* name_,
                                 cookie3 cookie_,
                                 const struct stat& attr,
                                 struct nfs_inode* nfs_inode_) :
    cookie(cookie_),
    attributes(attr),
    has_attributes(true),
    nfs_inode(nfs_inode_),
    name(name_)
{
    // Sanity check for attr. Blob NFS only supports these files.
    assert(((attr.st_mode & S_IFMT) == S_IFREG) ||
           ((attr.st_mode & S_IFMT) == S_IFDIR) ||
           ((attr.st_mode & S_IFMT) == S_IFLNK));

    /*
     * While this directory_entry is allocated (and present in
     * readdirectory_cache) , we will return this inode to fuse,
     * so keep it allocated.
     * Note that fuse can call forget() for the inode, even though
     * we might have it in our cache.
     */
    nfs_inode->incref();
}

directory_entry::directory_entry(const char* name_,
                                 cookie3 cookie_,
                                 uint64_t fileid_) :
    cookie(cookie_),
    has_attributes(false),
    nfs_inode(nullptr),
    name(name_)
{
    // NFS recommends against this.
    assert(fileid_ != 0);

    // fuse_add_direntry() needs these two fields, so set them.
    ::memset(&attributes, 0, sizeof(attributes));
    attributes.st_ino = fileid_;
    attributes.st_mode = 0;
}

directory_entry::~directory_entry()
{
    // Drop the ref we held in the constructor.
    if (nfs_inode) {
        nfs_inode->decref();
    }
}
