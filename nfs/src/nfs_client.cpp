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
    if (!transport.start()) {
        AZLogError("Failed to start the RPC transport.");
        return false;
    }

    /*
     * Also query the attributes for the root fh.
     * XXX: Though libnfs makes getattr call as part of mount but there is no
     *      way for us to fetch those attributes from libnfs, so we need to
     *      query again.
     */
    struct fattr3 fattr;
    const bool ret =
        getattr_sync(*(nfs_get_rootfh(transport.get_nfs_context())), fattr);

    /*
     * If we fail to successfully issue GETATTR RPC to the root fh,
     * then there's something non-trivially wrong, fail client init.
     */
    if (!ret) {
        AZLogError("First GETATTR to rootfh failed!");
        return false;
    }

    /*
     * Initialiaze the root file handle for this client.
     */
    root_fh = get_nfs_inode(nfs_get_rootfh(transport.get_nfs_context()),
                            &fattr,
                            true /* is_root_inode */);

    // Initialize the RPC task list.
    rpc_task_helper = rpc_task_helper::get_instance(this);

    return true;
}

/**
 * Given a filehandle and fattr (containing fileid defining a file/dir),
 * get the nfs_inode for that file/dir. It searches in the global list of
 * all inodes and returns from there if found, else creates a new nfs_inode.
 * The returned inode has it refcnt incremented by 1.
 */
struct nfs_inode *nfs_client::get_nfs_inode(const nfs_fh3 *fh,
                                            const struct fattr3 *fattr,
                                            bool is_root_inode)
{
#ifndef ENABLE_NON_AZURE_NFS
    // Blob NFS supports only these file types.
    assert((fattr->type == NF3REG) ||
           (fattr->type == NF3DIR) ||
           (fattr->type == NF3LNK));
#endif

    const uint32_t file_type = (fattr->type == NF3DIR) ? S_IFDIR :
                                ((fattr->type == NF3LNK) ? S_IFLNK : S_IFREG);

    /*
     * Search in the global inode list first and only if not found, create a
     * new one. This is very important as returning multiple inodes for the
     * same file is recipe for disaster.
     */
    {
        std::shared_lock<std::shared_mutex> lock(inode_map_lock);

        /*
         * Search by fileid in the multimap. Since fileid is not guaranteed to
         * be unique, we need to check for FH match in the matched inode(s)
         * list.
         */
        const auto range = inode_map.equal_range(fattr->fileid);

        for (auto i = range.first; i != range.second; ++i) {
            assert(i->first == fattr->fileid);
            assert(i->second->magic == NFS_INODE_MAGIC);

            if (FH_EQUAL(&(i->second->get_fh()), fh)) {
                // File type must not change for an inode.
                assert(i->second->file_type == file_type);

                if (i->second->is_forgotten()) {
                    AZLogDebug("[{}] Reusing forgotten inode (dircachecnt={})",
                               i->second->get_fuse_ino(),
                               i->second->dircachecnt.load());
                }

                i->second->incref();
                return i->second;
            }
        }
    }

    struct nfs_inode *inode = new nfs_inode(fh, fattr, this, file_type,
                                            is_root_inode ? FUSE_ROOT_ID : 0);

    {
        std::unique_lock<std::shared_mutex> lock(inode_map_lock);

        AZLogWarn("[{}:{}] Allocated new inode ({})",
                  inode->get_filetype_coding(),
                  inode->get_fuse_ino(), inode_map.size());

        /*
         * With the exclusive lock held, check once more if some other thread
         * added this inode before we could get the lock. If so, then delete
         * the inode created above, grab a refcnt on the inode created by the
         * other thread and return that.
         */
        const auto range = inode_map.equal_range(fattr->fileid);

        for (auto i = range.first; i != range.second; ++i) {
            assert(i->first == fattr->fileid);
            assert(i->second->magic == NFS_INODE_MAGIC);

            if (FH_EQUAL(&(i->second->get_fh()), fh)) {
                AZLogWarn("[{}] Another thread added inode, deleting ours",
                          inode->get_fuse_ino());
                delete inode;

                i->second->incref();
                return i->second;
            }
        }

        min_ino = std::min(min_ino, (fuse_ino_t) inode);
        max_ino = std::max(max_ino, (fuse_ino_t) inode);

        inode->incref();

        // Ok, insert the newly allocated inode in the global map.
        inode_map.insert({fattr->fileid, inode});
    }

    return inode;
}

// Caller must hold the inode_map_lock.
void nfs_client::put_nfs_inode_nolock(struct nfs_inode *inode,
                                      size_t dropcnt)
{
    assert(inode->magic == NFS_INODE_MAGIC);
    assert(inode->lookupcnt >= dropcnt);

    /*
     * We have to reduce the lookupcnt by dropcnt regardless of whether we
     * free the inode or not. After dropping the lookupcnt if it becomes 0
     * then we proceed to perform the other checks for deciding whether the
     * inode can be safely removed from inode_map and freed.
     */
    inode->lookupcnt -= dropcnt;

    /*
     * Caller should call us only for forgotten inodes but it's possible that
     * after we held the inode_map_lock some other thread got a reference on
     * this inode.
     */
    if (inode->lookupcnt > 0) {
        AZLogWarn("[{}] Inode no longer forgotten", inode->get_fuse_ino());
        return;
    }

    /*
     * Directory inodes cannot be deleted while the directory cache is not
     * purged. Note that we purge directory cache from decref() when the
     * refcnt reaches 0, i.e., fuse is no longer referencing the directory.
     * So, a non-zero directory cache count means that some other thread
     * started enumerating the directory before we could delete the directory
     * inode. Fuse will call FORGET on the directory and then we can free this
     * inode.
     */
    if (inode->is_dir() && (inode->dircache_handle->get_num_entries() != 0)) {
        AZLogWarn("[{}] Inode still has {} entries in dircache, skipping",
                  inode->get_fuse_ino(),
                  inode->dircache_handle->get_num_entries());
        return;
    }


    /*
     * If this inode is referenced by some directory_entry then we cannot free
     * it. We will attempt to free it later when the parent directory is purged
     * and the inode loses its last dircachecnt reference.
     */
    if (inode->dircachecnt) {
        AZLogVerbose("[{}] Inode is cached by readdir ({})",
                     inode->get_fuse_ino(), inode->dircachecnt.load());
        return;
    }

    /*
     * Ok, inode is not referenced by fuse VFS and it's not referenced by
     * any readdir cache, let's remove it from the inode_map. Once removed
     * from inode_map, any subsequent get_nfs_inode() calls for this file
     * (fh and fileid) will allocate a new nfs_inode, which will most likely
     * result in a new fuse inode number.
     */
    auto range = inode_map.equal_range(inode->get_fileid());

    for (auto i = range.first; i != range.second; ++i) {
        assert(i->first == inode->get_fileid());
        assert(i->second->magic == NFS_INODE_MAGIC);

        if (i->second == inode) {
            AZLogWarn("[{}:{}] Deleting inode (inode_map size: {})",
                      inode->get_filetype_coding(),
                      inode->get_fuse_ino(),
                      inode_map.size()-1);
            inode_map.erase(i);
            delete inode;
            return;
        }
    }

    // We must find the inode in inode_map.
    assert(0);
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

void nfs_client::write(fuse_req_t req, fuse_ino_t ino, const char* buf, size_t size, off_t off)
{
    struct rpc_task *tsk = rpc_task_helper->alloc_rpc_task();

    tsk->init_write(req, ino, buf, size, off);
    tsk->run_write();
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

void nfs_client::rmdir(
    fuse_req_t req,
    fuse_ino_t parent_ino,
    const char* name)
{
    struct rpc_task *tsk = rpc_task_helper->alloc_rpc_task();

    tsk->init_rmdir(req, parent_ino, name);
    tsk->run_rmdir();
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

/*
 * Creates a new inode for the given fh and passes it to fuse layer.
 * This will be called by the APIs which must return a filehandle back to the
 * client like lookup, create etc.
 */
void nfs_client::reply_entry(
    struct rpc_task *ctx,
    const nfs_fh3 *fh,
    const struct fattr3 *fattr,
    const struct fuse_file_info *file)
{
    static struct fattr3 zero_fattr;
    struct nfs_inode *inode = nullptr;
    fuse_entry_param entry;

    memset(&entry, 0, sizeof(entry));

    if (fh) {
        // This will be freed from fuse forget callback.
        inode = get_nfs_inode(fh, fattr);

        entry.ino = inode->get_fuse_ino();
        entry.attr = inode->attr;
        entry.attr_timeout = inode->get_actimeo();
        entry.entry_timeout = inode->get_actimeo();
    } else {
        /*
         * The only valid case where reply_entry() is called with null fh
         * is the case where lookup yielded "not found". We are using the
         * fuse support for negative dentry where we should respond with
         * success but ino set to 0 to convey to fuse that it must cache
         * the negative dentry for entry_timeout period.
         * This caching helps to improve performance by avoiding repeated
         * lookup requests for entries that are known not to exist.
         *
         * TODO: See if negative dentry timeout of 5 secs is ok.
         */
        assert(!fattr);
        stat_from_fattr3(&entry.attr, &zero_fattr);
        entry.attr_timeout = 5;
        entry.entry_timeout = 5;
    }

    if (file) {
        ctx->reply_create(&entry, file);
    } else {
        ctx->reply_entry(&entry);
    }
}

// Translate a NFS fattr3 into struct stat.
/* static */
void nfs_client::stat_from_fattr3(struct stat *st, const struct fattr3 *attr)
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
    {
        std::unique_lock<std::mutex> lock(ctx->ctx_mutex);

        if (res && (rpc_status == RPC_STATUS_SUCCESS) && (res->status == NFS3_OK)) {
            *(ctx->fattr) = res->GETATTR3res_u.resok.obj_attributes;
            ctx->is_callback_success = true;
        }
        ctx->callback_called = true;
    }

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

try_again:
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
    if (!ctx->cv.wait_for(lock, std::chrono::seconds(120),
                          [&ctx] { return (ctx->callback_called == true); })) {
        AZLogWarn("Timed out waiting for getattr response, re-issuing getattr!");
        goto try_again;
    }

    const bool success = ctx->is_callback_success;
    delete ctx;

    return success;
}
#endif
