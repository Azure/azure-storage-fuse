#include "aznfsc.h"
#include "nfs_client.h"
#include "nfs_internal.h"
#include "rpc_task.h"
#include "rpc_readdir.h"

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

    /*
     * Initialiaze the root file handle for this client.
     * Note that we don't know the root directory attributes yet.
     * First GETATTR call would fetch those and then we will save it
     * in nfs_inode::attr.
     */
    root_fh = new nfs_inode(
                nfs_get_rootfh(transport.get_nfs_context()),
                nullptr, /* fattr3 */
                this,
                S_IFDIR,
                FUSE_ROOT_ID);
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
    fuse_ino_t ino,
    struct fuse_file_info* file)
{
    struct rpc_task *tsk = rpc_task_helper->alloc_rpc_task();

    tsk->init_getattr(req, ino);
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
    fuse_ino_t ino,
    struct stat* attr,
    int to_set,
    struct fuse_file_info* file)
{
    struct rpc_task *tsk = rpc_task_helper->alloc_rpc_task();

    tsk->init_setattr(req, ino, attr, to_set, file);
    tsk->run_setattr();
}

void nfs_client::readdir(
    fuse_req_t req,
    fuse_ino_t ino,
    size_t size,
    off_t offset,
    struct fuse_file_info* file)
{
    struct rpc_task *tsk = rpc_task_helper->alloc_rpc_task();
    struct nfs_inode *inode = get_nfs_inode_from_ino(ino);

    // Force revalidate for offset==0 to ensure cto consistency.
    inode->revalidate(offset == 0);

    tsk->init_readdir(req, ino, size, offset, file);
    tsk->run_readdir();
}

void nfs_client::readdirplus(
    fuse_req_t req,
    fuse_ino_t ino,
    size_t size,
    off_t offset,
    struct fuse_file_info* file)
{
    struct rpc_task *tsk = rpc_task_helper->alloc_rpc_task();
    struct nfs_inode *inode = get_nfs_inode_from_ino(ino);

    // Force revalidate for offset==0 to ensure cto consistency.
    inode->revalidate(offset == 0);

    tsk->init_readdirplus(req, ino, size, offset, file);
    tsk->run_readdirplus();
}

//
// Creates a new inode for the given fh and passes it to fuse layer.
// This will be called by the APIs which must return a filehandle back to the client
// like lookup, create etc.
//
void nfs_client::reply_entry(
    struct rpc_task *ctx,
    const nfs_fh3 *fh,
    const struct fattr3 *fattr,
    const struct fuse_file_info *file)
{
    static struct fattr3 zero_fattr;
    struct nfs_inode *nfs_ino = nullptr;
    fuse_entry_param entry;

    memset(&entry, 0, sizeof(entry));

    if (fh) {
        // Blob NFS supports only these file types.
        assert((fattr->type == NF3REG) ||
               (fattr->type == NF3DIR) ||
               (fattr->type == NF3LNK));

        const uint32_t file_type =
            (fattr->type == NF3DIR) ? S_IFDIR
                                   : ((fattr->type == NF3LNK) ? S_IFLNK
                                                             : S_IFREG);

        // This will be freed from fuse forget callback.
        nfs_ino = new nfs_inode(fh, fattr, this, file_type);

        entry.ino = nfs_ino->get_fuse_ino();
        entry.attr = nfs_ino->attr;
        entry.attr_timeout = nfs_ino->get_actimeo();
        entry.entry_timeout = nfs_ino->get_actimeo();
    }
    else
    {
        /*
         * The only valid case where reply_entry() is called with null fh
         * is the case where lookup yielded "not found". We are using the
         * fuse support for negative dentry where we should respond with
         * success but ino set to 0 to convey to fuse that it must cache
         * the negative dentry for entry_timeout period.
         * This caching helps to improve performance by avoiding repeated
         * lookup requests for entries that are known not to exist.
         *
         * TODO: See if negative dentry timeout of 30 secs is good.
         */
        assert(!fattr);
        stat_from_fattr3(&entry.attr, &zero_fattr);
        entry.attr_timeout = 30;
        entry.entry_timeout = 30;
    }

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
/* static */
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

/*
 * TODO: Once we add sync getattr API in libnfs, we can get rid of this
 *       code. Till then use getattr_sync() to get attributes from the server.
 */
#if 1
struct getattr_context
{
    struct fattr3 *fattr;
    bool callback_called;
    bool is_callback_success;
    std::mutex ctx_mutex;
    std::condition_variable cv;

    getattr_context(struct fattr3 *fattr_):
        fattr(fattr_),
        callback_called(false),
        is_callback_success(false)
    {}
};

static void getattr_callback(
    struct rpc_context* /* rpc */,
    int rpc_status,
    void *data,
    void *private_data)
{
    auto ctx = (struct getattr_context*) private_data;
    auto res = (GETATTR3res*) data;

    if (res && (rpc_status == RPC_STATUS_SUCCESS) && (res->status == NFS3_OK)) {
        *(ctx->fattr) = res->GETATTR3res_u.resok.obj_attributes;
        ctx->is_callback_success = true;
    }
    ctx->callback_called = true;
    ctx->cv.notify_one();
}

/**
 * Issue a sync GETATTR RPC call to filehandle 'fh' and save the received
 * attributes in 'fattr'.
 */
bool nfs_client::getattr_sync(const struct nfs_fh3& fh, struct fattr3& fattr)
{
    // TODO:Make sync getattr call once libnfs adds support.

    bool rpc_retry = false;
    struct getattr_context *ctx = new getattr_context(&fattr);

    do {
        struct GETATTR3args args;
        args.object = fh;

        if (rpc_nfs3_getattr_task(nfs_get_rpc_context(get_nfs_context()),
                                  getattr_callback, &args, ctx) == NULL) {
            /*
             * This call fails due to internal issues like OOM etc
             * and not due to an actual error, hence retry.
             */
            rpc_retry = true;
        }
    } while (rpc_retry);

    std::unique_lock<std::mutex> lock(ctx->ctx_mutex);
    ctx->cv.wait(lock, [&ctx] { return ctx->callback_called; } );

    const bool success = ctx->is_callback_success;
    delete ctx;

    return success;
}
#endif
