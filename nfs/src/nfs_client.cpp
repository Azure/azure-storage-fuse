#include "aznfsc.h"
#include "nfs_client.h"
#include "nfs_internal.h"
#include "rpc_task.h"

#define RSTATUS(r) ((r) ? (r)->status : NFS3ERR_SERVERFAULT)

// The user should first init the client class before using it.
bool nfs_client::init()
{
    // init() must be called only once.
    assert(root_fh == nullptr);

    const std::string& acc_name = aznfsc_cfg.account;
    const std::string& cont_name = aznfsc_cfg.container;
    const std::string& blob_suffix = aznfsc_cfg.cloud_suffix;

    /*
     * Setup RPC transport.
     * This will create all required connections and perform NFS mount on
     * those, setting up libnfs nfs_context for each connection.
     * Once this is done the connections are ready to carry RPC req/resp.
     */
    if (!transport.start())
    {
        AZLogError("Failed to start the RPC transport.");
        return false;
    }

    // initialiaze the root file handle.
    // TODO: Take care of freeing this. Should this be freed in the ~nfs_client()?
    root_fh = new nfs_inode(nfs_get_rootfh(transport.get_nfs_context()) /*, 1  ino will be 1 for root */);
    root_fh->set_inode(FUSE_ROOT_ID);
    //AZLogInfo("Obtained root fh is {}", root_fh->get_fh());

    // Initialize the RPC task list.
    rpc_task_helper = rpc_task_helper::get_instance(this);

    return true;
}

struct nfs_context* nfs_client::get_nfs_context() const
{
    return transport.get_nfs_context();
}

void nfs_client::lookup(fuse_req_t req, fuse_ino_t parent_ino, const char* name)
{
    struct rpc_task *tsk = rpc_task_helper->alloc_rpc_task();

    tsk->init_lookup(req, name, parent_ino);
    tsk->run_lookup();
}

void nfs_client::getattr(
    fuse_req_t req,
    fuse_ino_t inode,
    struct fuse_file_info* file)
{
    struct rpc_task *tsk = rpc_task_helper->alloc_rpc_task();

    tsk->init_getattr(req, inode);
    tsk->run_getattr();
}

void nfs_client::create(
    fuse_req_t req,
    fuse_ino_t parent_ino,
    const char* name,
    mode_t mode,
    struct fuse_file_info* file)
{
    struct rpc_task *tsk = rpc_task_helper->alloc_rpc_task();

    tsk->init_create_file(req, parent_ino, name, mode, file);
    tsk->run_create_file();
}

void nfs_client::mkdir(
    fuse_req_t req,
    fuse_ino_t parent_ino,
    const char* name,
    mode_t mode)
{
    struct rpc_task *tsk = rpc_task_helper->alloc_rpc_task();

    tsk->init_mkdir(req, parent_ino, name, mode);
    tsk->run_mkdir();
}

void nfs_client::setattr(
    fuse_req_t req,
    fuse_ino_t inode,
    struct stat* attr,
    int to_set,
    struct fuse_file_info* file)
{
    struct rpc_task *tsk = rpc_task_helper->alloc_rpc_task();

    tsk->init_setattr(req, inode, attr, to_set, file);
    tsk->run_setattr();
}

//
// Creates a new inode for the given fh and passes it to fuse layer.
// This will be called by the APIs which much return a filehandle back to the client
// like lookup, create etc.
//
void nfs_client::reply_entry(
    struct rpc_task* ctx,
    const nfs_fh3* fh,
    const struct fattr3* attr,
    const struct fuse_file_info* file)
{
    nfs_inode* nfs_ino;

    if (fh)
    {
        // TODO: When should this be freed? This should be freed when the ino is freed,
        // 	 but decide when should that be done?
        nfs_ino = new nfs_inode(fh);
        nfs_ino->set_inode((fuse_ino_t)nfs_ino);
    }
    else
    {
        nfs_ino = nullptr;
    }

    fuse_entry_param entry;
    memset(&entry, 0, sizeof(entry));

    stat_from_fattr3(&entry.attr, attr);
    entry.ino = (fuse_ino_t)(uintptr_t)nfs_ino;

    /*
     * TODO: Set the timeout to better value.
     */
    entry.attr_timeout = 60; //attrTimeout;
    entry.entry_timeout = 60; //attrTimeout;

    if (file)
    {
        ctx->reply_create(&entry, file);
    }
    else
    {
        ctx->reply_entry(&entry);
    }
}

// Translate a NFS fattr3 into struct stat.
void nfs_client::stat_from_fattr3(struct stat* st, const struct fattr3* attr)
{
    ::memset(st, 0, sizeof(*st));
    st->st_dev = attr->fsid;
    st->st_ino = attr->fileid;
    st->st_mode = attr->mode;
    st->st_nlink = attr->nlink;
    st->st_uid = attr->uid;
    st->st_gid = attr->gid;
    // TODO: Uncomment the below line.
    // st->st_rdev = makedev(attr->rdev.specdata1, attr->rdev.specdata2);
    st->st_size = attr->size;
    st->st_blksize = NFS_BLKSIZE;
    st->st_blocks = (attr->used + 511) >> 9;
    st->st_atim.tv_sec = attr->atime.seconds;
    st->st_atim.tv_nsec = attr->atime.nseconds;
    st->st_mtim.tv_sec = attr->mtime.seconds;
    st->st_mtim.tv_nsec = attr->mtime.nseconds;
    st->st_ctim.tv_sec = attr->ctime.seconds;
    st->st_ctim.tv_nsec = attr->ctime.nseconds;
    switch (attr->type) {
    case NF3REG:
        st->st_mode |= S_IFREG;
        break;
    case NF3DIR:
        st->st_mode |= S_IFDIR;
        break;
    case NF3BLK:
        st->st_mode |= S_IFBLK;
        break;
    case NF3CHR:
        st->st_mode |= S_IFCHR;
        break;
    case NF3LNK:
        st->st_mode |= S_IFLNK;
        break;
    case NF3SOCK:
        st->st_mode |= S_IFSOCK;
        break;
    case NF3FIFO:
        st->st_mode |= S_IFIFO;
        break;
    }
}
