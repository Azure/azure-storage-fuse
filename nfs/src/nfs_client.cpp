#include "nfs_client.h"
#include "nfs_internal.h"

std::atomic<bool> nfs_client::initialized(false);
std::string nfs_client::server("");
std::string nfs_client::export_path("");
rpc_transport* nfs_client::transport;
rpc_task_helper* nfs_client::rpc_task_helper_instance;
nfs_file_handle* nfs_client::root_fh;

#define RSTATUS(r) ((r) ? (r)->status : NFS3ERR_SERVERFAULT)

// The user should first init the client class before using it.
bool nfs_client::init(
    std::string& acc_name,
    std::string& cont_name,
    std::string& blob_suffix,
    struct mount_options* opt)
{
    // Check if init() has been called before
    if (!initialized.exchange(true)) {

        get_instance_impl(&acc_name, &cont_name, &blob_suffix, opt);

        // Get the RPC transport to be used for this client.
        transport = rpc_transport::get_instance(opt);

        // This will init the transport layer and start the connections to the server.
        // It returns FALSE if it fails to create the connections.
        if (!transport->start())
        {
            AZLogError("Failed to start the RPC transport.");
            return false;
        }

        // initialiaze the root file handle.
        // TODO: Take care of freeing this. Should this be freed in the ~nfs_client()?
        root_fh = new nfs_file_handle(nfs_get_rootfh(transport->get_nfs_context()) /*, 1  ino will be 1 for root */);
        root_fh->set_inode(1);
        //AZLogInfo("Obtained root fh is {}", root_fh->get_fh());

        // Initialize the RPC task list.
        rpc_task_helper_instance = rpc_task_helper::get_instance();

        return true;
    }

    // Return false if the method is called again.
    return false;
}

struct nfs_context* nfs_client::get_nfs_context() const
{
    return transport->get_nfs_context();
}

void nfs_client::lookup(fuse_req_t req, fuse_ino_t parent, const char* name)
{
    struct rpc_task* tsk = nullptr;
    bool success = rpc_task_helper_instance->get_rpc_task_instance(&tsk);

    if (success)
    {
        assert (tsk != nullptr);
        tsk->set_client (this);
        tsk->set_fuse_req(req);
        tsk->set_op_type(FOPTYPE_LOOKUP);
        tsk->rpc_api.lookup_task.set_file_name(name);
        tsk->rpc_api.lookup_task.set_parent_inode(parent);

        tsk->run_lookup_rpc_task();
    }
    // TODO: See what should be done in failure case.
}

void nfs_client::getattr(
    fuse_req_t req,
    fuse_ino_t inode,
    struct fuse_file_info* file)
{
    struct rpc_task* tsk = nullptr;
    bool success = rpc_task_helper_instance->get_rpc_task_instance(&tsk);

    if (success)
    {
        assert (tsk != nullptr);
        tsk->set_client (this);
        tsk->set_fuse_req(req);
        tsk->set_op_type(FOPTYPE_GETATTR);
        tsk->rpc_api.getattr_task.set_inode(inode);

        tsk->run_getattr_rpc_task();
    }
    // TODO: See what should be done in failure case.
}

void nfs_client::create(
    fuse_req_t req,
    fuse_ino_t parent,
    const char* name,
    mode_t mode,
    struct fuse_file_info* file)
{
    struct rpc_task* tsk = nullptr;
    bool success = rpc_task_helper_instance->get_rpc_task_instance(&tsk);

    if (success)
    {
        assert (tsk != nullptr);
        tsk->set_client (this);
        tsk->set_fuse_req(req);
        tsk->set_op_type(FOPTYPE_CREATE);
        tsk->rpc_api.create_task.set_parent_inode(parent);
        tsk->rpc_api.create_task.set_file_name(name);
        tsk->rpc_api.create_task.set_mode(mode);
        tsk->rpc_api.create_task.set_fuse_file(file);

        tsk->run_create_file_rpc_task();
    }
    // TODO: See what should be done in failure case.
}

void nfs_client::mkdir(
    fuse_req_t req,
    fuse_ino_t parent,
    const char* name,
    mode_t mode)
{
    struct rpc_task* tsk = nullptr;
    bool success = rpc_task_helper_instance->get_rpc_task_instance(&tsk);

    if (success)
    {
        assert (tsk != nullptr);
        tsk->set_client (this);
        tsk->set_fuse_req(req);
        tsk->set_op_type(FOPTYPE_MKDIR);
        tsk->rpc_api.mkdir_task.set_parent_inode(parent);
        tsk->rpc_api.mkdir_task.set_dir_name(name);
        tsk->rpc_api.mkdir_task.set_mode(mode);

        tsk->run_mkdir_rpc_task();
    }
    // TODO: See what should be done in failure case.
}

void nfs_client::setattr(
    fuse_req_t req,
    fuse_ino_t inode,
    struct stat* attr,
    int toSet,
    struct fuse_file_info* file)
{
    struct rpc_task* tsk = nullptr;
    bool success = rpc_task_helper_instance->get_rpc_task_instance(&tsk);

    if (success)
    {
        assert (tsk != nullptr);
        tsk->set_client (this);
        tsk->set_fuse_req(req);
        tsk->set_op_type(FOPTYPE_SETATTR);
        tsk->rpc_api.setattr_task.set_inode(inode);
        tsk->rpc_api.setattr_task.set_fuse_file(file);
        tsk->rpc_api.setattr_task.set_attribute_and_mask(attr, toSet);
        tsk->run_setattr_rpc_task();
    }
    // TODO: See what should be done in failure case.
}

//
// Creates a new inode for the given fh and passes it to fuse_reply_entry().
// This will be called by the APIs which much return a filehandle back to the client
// like lookup, create etc.
//
void nfs_client::reply_entry(
    struct rpc_task* ctx,
    const nfs_fh3* fh,
    const struct fattr3* attr,
    const struct fuse_file_info* file)
{
    nfs_file_handle* filehandle;

    if (fh)
    {
        // TODO: When should this be freed? This should be freed when the ino is freed,
        // 	 but decide when should that be done?
        filehandle = new nfs_file_handle(fh);
        filehandle->set_inode((fuse_ino_t)filehandle);
    }
    else
    {
        filehandle = nullptr;
    }

    fuse_entry_param entry;
    memset(&entry, 0, sizeof(entry));

    stat_from_fattr3(&entry.attr, attr);
    entry.ino = (fuse_ino_t)(uintptr_t)filehandle;

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
