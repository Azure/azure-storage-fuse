#include "aznfsc.h"
#include "nfs_client.h"
#include "nfs_internal.h"
#include "rpc_task.h"
#include "rpc_readdir.h"

#define NFS_STATUS(r) ((r) ? (r)->status : NFS3ERR_SERVERFAULT)

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
        getattr_sync(*(nfs_get_rootfh(transport.get_nfs_context())),
                     FUSE_ROOT_ID, fattr);

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

void nfs_client::jukebox_runner()
{
    AZLogDebug("Started jukebox_runner");

    do {
        if (jukebox_seeds.empty()) {
            ::sleep(5);
        } else {
            ::sleep(1);
        }

        {
            std::unique_lock<std::mutex> lock(jukebox_seeds_lock);
            if (jukebox_seeds.empty()) {
                continue;
            }
        }

        AZLogDebug("jukebox_runner woken up ({} requests in queue)",
                   jukebox_seeds.size());

        /*
         * Go over all queued requests and issue those which are ready to be
         * issued, i.e., they have been queued for more than JUKEBOX_DELAY_SECS
         * seconds. We issue the requests after releasing jukebox_seeds_lock.
         */
        std::vector<jukebox_seedinfo *> jsv;
        {
            std::unique_lock<std::mutex> lock(jukebox_seeds_lock);
            while (!jukebox_seeds.empty()) {
                struct jukebox_seedinfo *js = jukebox_seeds.front();

                if (js->run_at_msecs > get_current_msecs()) {
                    break;
                }

                jukebox_seeds.pop();

                jsv.push_back(js);
            }
        }

        for (struct jukebox_seedinfo *js : jsv) {
            switch (js->rpc_api->optype) {
                case FUSE_LOOKUP:
                    AZLogWarn("[JUKEBOX REISSUE] lookup(req={}, "
                              "parent_ino={}, name={})",
                              fmt::ptr(js->rpc_api->req),
                              js->rpc_api->lookup_task.get_parent_ino(),
                              js->rpc_api->lookup_task.get_file_name());
                    lookup(js->rpc_api->req,
                           js->rpc_api->lookup_task.get_parent_ino(),
                           js->rpc_api->lookup_task.get_file_name());
                    break;
                case FUSE_ACCESS:
                    AZLogWarn("[JUKEBOX REISSUE] access(req={}, "
                              "ino={}, mask=0{:03o})",
                              fmt::ptr(js->rpc_api->req),
                              js->rpc_api->access_task.get_ino(),
                              js->rpc_api->access_task.get_mask());
                    access(js->rpc_api->req,
                           js->rpc_api->access_task.get_ino(),
                           js->rpc_api->access_task.get_mask());
                    break;
                case FUSE_GETATTR:
                    AZLogWarn("[JUKEBOX REISSUE] getattr(req={}, ino={}, "
                              "fi=null)",
                              fmt::ptr(js->rpc_api->req),
                              js->rpc_api->getattr_task.get_ino());
                    getattr(js->rpc_api->req,
                            js->rpc_api->getattr_task.get_ino(),
                            nullptr);
                    break;
                case FUSE_SETATTR:
                    AZLogWarn("[JUKEBOX REISSUE] setattr(req={}, ino={}, "
                              "to_set=0x{:x}, fi={})",
                              fmt::ptr(js->rpc_api->req),
                              js->rpc_api->setattr_task.get_ino(),
                              js->rpc_api->setattr_task.get_attr_flags_to_set(),
                              fmt::ptr(js->rpc_api->setattr_task.get_fuse_file()));
                    setattr(js->rpc_api->req,
                            js->rpc_api->setattr_task.get_ino(),
                            js->rpc_api->setattr_task.get_attr(),
                            js->rpc_api->setattr_task.get_attr_flags_to_set(),
                            js->rpc_api->setattr_task.get_fuse_file());
                    break;
                case FUSE_CREATE:
                    AZLogWarn("[JUKEBOX REISSUE] create(req={}, parent_ino={},"
                              " name={}, mode=0{:03o}, fi={})",
                              fmt::ptr(js->rpc_api->req),
                              js->rpc_api->create_task.get_parent_ino(),
                              js->rpc_api->create_task.get_file_name(),
                              js->rpc_api->create_task.get_mode(),
                              fmt::ptr(js->rpc_api->create_task.get_fuse_file()));
                    create(js->rpc_api->req,
                           js->rpc_api->create_task.get_parent_ino(),
                           js->rpc_api->create_task.get_file_name(),
                           js->rpc_api->create_task.get_mode(),
                           js->rpc_api->create_task.get_fuse_file());
                    break;
                case FUSE_MKNOD:
                    AZLogWarn("[JUKEBOX REISSUE] mknod(req={}, parent_ino={},"
                              " name={}, mode=0{:03o})",
                              fmt::ptr(js->rpc_api->req),
                              js->rpc_api->mknod_task.get_parent_ino(),
                              js->rpc_api->mknod_task.get_file_name(),
                              js->rpc_api->mknod_task.get_mode());
                    mknod(js->rpc_api->req,
                           js->rpc_api->mknod_task.get_parent_ino(),
                           js->rpc_api->mknod_task.get_file_name(),
                           js->rpc_api->mknod_task.get_mode());
                    break;
                case FUSE_MKDIR:
                    AZLogWarn("[JUKEBOX REISSUE] mkdir(req={}, parent_ino={}, "
                              "name={}, mode=0{:03o})",
                              fmt::ptr(js->rpc_api->req),
                              js->rpc_api->mkdir_task.get_parent_ino(),
                              js->rpc_api->mkdir_task.get_dir_name(),
                              js->rpc_api->mkdir_task.get_mode());
                    mkdir(js->rpc_api->req,
                          js->rpc_api->mkdir_task.get_parent_ino(),
                          js->rpc_api->mkdir_task.get_dir_name(),
                          js->rpc_api->mkdir_task.get_mode());
                    break;
                case FUSE_RMDIR:
                    AZLogWarn("[JUKEBOX REISSUE] rmdir(req={}, parent_ino={}, "
                              "name={})",
                              fmt::ptr(js->rpc_api->req),
                              js->rpc_api->rmdir_task.get_parent_ino(),
                              js->rpc_api->rmdir_task.get_dir_name());
                    rmdir(js->rpc_api->req,
                          js->rpc_api->rmdir_task.get_parent_ino(),
                          js->rpc_api->rmdir_task.get_dir_name());
                    break;
                case FUSE_UNLINK:
                    AZLogWarn("[JUKEBOX REISSUE] unlink(req={}, parent_ino={}, "
                              "name={})",
                              fmt::ptr(js->rpc_api->req),
                              js->rpc_api->unlink_task.get_parent_ino(),
                              js->rpc_api->unlink_task.get_file_name());
                    unlink(js->rpc_api->req,
                           js->rpc_api->unlink_task.get_parent_ino(),
                           js->rpc_api->unlink_task.get_file_name());
                    break;
                case FUSE_SYMLINK:
                    AZLogWarn("[JUKEBOX REISSUE] symlink(req={}, link={}, "
                              "parent_ino={}, name={})",
                              fmt::ptr(js->rpc_api->req),
                              js->rpc_api->symlink_task.get_link(),
                              js->rpc_api->symlink_task.get_parent_ino(),
                              js->rpc_api->symlink_task.get_name());
                    symlink(js->rpc_api->req,
                            js->rpc_api->symlink_task.get_link(),
                            js->rpc_api->symlink_task.get_parent_ino(),
                            js->rpc_api->symlink_task.get_name());
                    break;
                case FUSE_READLINK:
                    AZLogWarn("[JUKEBOX REISSUE] readlink(req={}, ino={})",
                              fmt::ptr(js->rpc_api->req),
                              js->rpc_api->readlink_task.get_ino());
                    readlink(js->rpc_api->req,
                             js->rpc_api->readlink_task.get_ino());
                    break;
                case FUSE_RENAME:
                    AZLogWarn("[JUKEBOX REISSUE] rename(req={}, parent_ino={}, "
                              "name={}, newparent_ino={}, newname={}, flags={})",
                              fmt::ptr(js->rpc_api->req),
                              js->rpc_api->rename_task.get_parent_ino(),
                              js->rpc_api->rename_task.get_name(),
                              js->rpc_api->rename_task.get_newparent_ino(),
                              js->rpc_api->rename_task.get_newname(),
                              js->rpc_api->rename_task.get_flags());
                    rename(js->rpc_api->req,
                           js->rpc_api->rename_task.get_parent_ino(),
                           js->rpc_api->rename_task.get_name(),
                           js->rpc_api->rename_task.get_newparent_ino(),
                           js->rpc_api->rename_task.get_newname(),
                           js->rpc_api->rename_task.get_silly_rename(),
                           js->rpc_api->rename_task.get_silly_rename_ino(),
                           js->rpc_api->rename_task.get_flags());
                    break;
                case FUSE_READ:
                    AZLogWarn("[JUKEBOX REISSUE] read(req={}, ino={}, "
                              "size={}, offset={} fi={})",
                              fmt::ptr(js->rpc_api->req),
                              js->rpc_api->read_task.get_ino(),
                              js->rpc_api->read_task.get_size(),
                              js->rpc_api->read_task.get_offset(),
                              fmt::ptr(js->rpc_api->read_task.get_fuse_file()));
                    jukebox_read(js->rpc_api);
                    break;
                case FUSE_READDIR:
                    AZLogWarn("[JUKEBOX REISSUE] readdir(req={}, ino={}, "
                              "size={}, off={}, fi={})",
                              fmt::ptr(js->rpc_api->req),
                              js->rpc_api->readdir_task.get_ino(),
                              js->rpc_api->readdir_task.get_size(),
                              js->rpc_api->readdir_task.get_offset(),
                              fmt::ptr(js->rpc_api->readdir_task.get_fuse_file()));
                    readdir(js->rpc_api->req,
                            js->rpc_api->readdir_task.get_ino(),
                            js->rpc_api->readdir_task.get_size(),
                            js->rpc_api->readdir_task.get_offset(),
                            js->rpc_api->readdir_task.get_fuse_file());
                    break;
                case FUSE_READDIRPLUS:
                    AZLogWarn("[JUKEBOX REISSUE] readdirplus(req={}, ino={}, "
                              "size={}, off={}, fi={})",
                              fmt::ptr(js->rpc_api->req),
                              js->rpc_api->readdir_task.get_ino(),
                              js->rpc_api->readdir_task.get_size(),
                              js->rpc_api->readdir_task.get_offset(),
                              fmt::ptr(js->rpc_api->readdir_task.get_fuse_file()));
                    readdirplus(js->rpc_api->req,
                                js->rpc_api->readdir_task.get_ino(),
                                js->rpc_api->readdir_task.get_size(),
                                js->rpc_api->readdir_task.get_offset(),
                                js->rpc_api->readdir_task.get_fuse_file());
                    break;
                case FUSE_FLUSH:
                    AZLogWarn("[JUKEBOX REISSUE] flush(req={}, ino={})",
                              fmt::ptr(js->rpc_api->req),
                              js->rpc_api->flush_task.get_ino());
                    jukebox_flush(js->rpc_api);
                    break;
                /* TODO: Add other request types */
                default:
                    AZLogError("Unknown jukebox seed type: {}", (int) js->rpc_api->optype);
                    assert(0);
                    break;
            }

            delete js;
        }
    } while (!shutting_down);
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

        AZLogDebug("[{}:{} / 0x{:08x}] Allocated new inode ({})",
                   inode->get_filetype_coding(),
                   inode->get_fuse_ino(), inode->get_crc(), inode_map.size());

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
    AZLogDebug("[{}] put_nfs_inode_nolock(dropcnt={}) called, lookupcnt={}",
               inode->get_fuse_ino(), dropcnt, inode->lookupcnt.load());

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
        AZLogWarn("[{}] Inode no longer forgotten: lookupcnt={}",
                  inode->get_fuse_ino(), inode->lookupcnt.load());
        return;
    }

    /*
     * This inode is going to be freed, either we never conveyed the inode
     * to fuse (we couldn't fit the directory entry in readdirplus buffer
     * or we failed to call fuse_reply_entry(), fuse_reply_create() or
     * fuse_reply_buf()), or fuse called forget for the inode.
     */
    assert(!inode->returned_to_fuse || inode->forget_seen);

    /*
     * Directory inodes cannot be deleted while the directory cache is not
     * purged. Note that we purge directory cache from decref() when the
     * refcnt reaches 0, i.e., fuse is no longer referencing the directory.
     * So, a non-zero directory cache count means that some other thread
     * started enumerating the directory before we could delete the directory
     * inode. Fuse will call FORGET on the directory and then we can free this
     * inode.
     */
    if (inode->is_dir() && !inode->is_cache_empty()) {
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

struct nfs_context* nfs_client::get_nfs_context(conn_sched_t csched,
                                                uint32_t fh_hash) const
{
    return transport.get_nfs_context(csched, fh_hash);
}

void nfs_client::lookup(fuse_req_t req, fuse_ino_t parent_ino, const char* name)
{
    struct rpc_task *tsk = rpc_task_helper->alloc_rpc_task(FUSE_LOOKUP);

    tsk->init_lookup(req, name, parent_ino);
    tsk->run_lookup();

    /*
     * Note: Don't access tsk after this as it may get freed anytime after
     *       the run_lookup() call. This applies to all APIs.
     */
}

static void lookup_sync_callback(
    struct rpc_context *rpc,
    int rpc_status,
    void *data,
    void *private_data)
{
    struct sync_rpc_context *ctx = (struct sync_rpc_context *) private_data;
    assert(ctx->magic == SYNC_RPC_CTX_MAGIC);

    rpc_task *task = ctx->task;
    assert(task->magic == RPC_TASK_MAGIC);
    assert(task->rpc_api->optype == FUSE_LOOKUP);

    fuse_ino_t *child_ino_p = (fuse_ino_t *) task->rpc_api->pvt;
    assert(child_ino_p != nullptr);
    *child_ino_p = 0;

    auto res = (LOOKUP3res *) data;
    const int status = task->status(rpc_status, NFS_STATUS(res));

    /*
     * Convey status to the issuer.
     */
    ctx->rpc_status = rpc_status;
    ctx->nfs_status = NFS_STATUS(res);

    /*
     * Now that the request has completed, we can query libnfs for the
     * dispatch time.
     */
    task->get_stats().on_rpc_complete(rpc_get_pdu(rpc), NFS_STATUS(res));

    {
        std::unique_lock<std::mutex> lock(ctx->mutex);

        // Must be called only once.
        assert(!ctx->callback_called);
        ctx->callback_called = true;

        if (status == 0) {
            assert(res->LOOKUP3res_u.resok.obj_attributes.attributes_follow);

            const nfs_fh3 *fh = (const nfs_fh3 *) &res->LOOKUP3res_u.resok.object;
            const struct fattr3 *fattr =
                (const struct fattr3 *) &res->LOOKUP3res_u.resok.obj_attributes.post_op_attr_u.attributes;
            const struct nfs_inode *inode = task->get_client()->get_nfs_inode(fh, fattr);
            (*child_ino_p) = inode->get_fuse_ino();
            if (ctx->fattr) {
                *(ctx->fattr) = *fattr;
            }
        }
    }

    ctx->cv.notify_one();
}

bool nfs_client::lookup_sync(fuse_ino_t parent_ino,
                             const char* name,
                             fuse_ino_t& child_ino)
{
    assert(name != nullptr);

    struct nfs_inode *parent_inode = get_nfs_inode_from_ino(parent_ino);
    const uint32_t fh_hash = parent_inode->get_crc();
    struct nfs_context *nfs_context =
        get_nfs_context(CONN_SCHED_FH_HASH, fh_hash);
    struct rpc_task *task = nullptr;
    struct sync_rpc_context *ctx = nullptr;
    struct rpc_pdu *pdu = nullptr;
    struct rpc_context *rpc = nullptr;
    bool rpc_retry = false;
    bool success = false;

    AZLogDebug("lookup_sync({}/{})", parent_ino, name);

try_again:
    do {
        LOOKUP3args args;
        args.what.dir = parent_inode->get_fh();
        args.what.name = (char *) name;

        if (task) {
            task->free_rpc_task();
        }

        task = get_rpc_task_helper()->alloc_rpc_task(FUSE_LOOKUP);
        task->init_lookup(nullptr /* fuse_req */, name, parent_ino);
        task->rpc_api->pvt = &child_ino;

        if (ctx) {
            delete ctx;
        }

        ctx = new sync_rpc_context(task, nullptr);
        rpc = nfs_get_rpc_context(nfs_context);

        rpc_retry = false;
        task->get_stats().on_rpc_issue();
        if ((pdu = rpc_nfs3_lookup_task(rpc, lookup_sync_callback,
                                        &args, ctx)) == NULL) {
            task->get_stats().on_rpc_cancel();
            /*
             * This call fails due to internal issues like OOM etc
             * and not due to an actual error, hence retry.
             */
            rpc_retry = true;
        }
    } while (rpc_retry);

    /*
     * If the LOOKUP response doesn't come for 60 secs we give up and send
     * a new one. We must cancel the old one.
     */
    {
        std::unique_lock<std::mutex> lock(ctx->mutex);
wait_more:
        if (!ctx->cv.wait_for(lock, std::chrono::seconds(60),
                              [&ctx] { return (ctx->callback_called == true); })) {
            if (rpc_cancel_pdu(rpc, pdu) == 0) {
                task->get_stats().on_rpc_cancel();
                AZLogWarn("Timed out waiting for lookup response, re-issuing "
                          "lookup!");
                // This goto will cause the above lock to unlock.
                goto try_again;
            } else {
                /*
                 * If rpc_cancel_pdu() fails it most likely means we got the RPC
                 * response right after we timed out waiting. It's best to wait
                 * for the callback to be called.
                 */
                AZLogWarn("Timed out waiting for lookup response, couldn't "
                          "cancel existing pdu, waiting some more!");
                // This goto will *not* cause the above lock to unlock.
                goto wait_more;
            }
        } else {
            assert(ctx->callback_called);
            assert(ctx->rpc_status != -1);
            assert(ctx->nfs_status != -1);

            const int status = task->status(ctx->rpc_status, ctx->nfs_status);
            if (status == 0) {
                success = true;
            } else if (ctx->rpc_status == RPC_STATUS_SUCCESS &&
                       ctx->nfs_status == NFS3ERR_JUKEBOX) {
                AZLogInfo("Got NFS3ERR_JUKEBOX for LOOKUP, re-issuing "
                          "after 1 sec!");
                ::usleep(1000 * 1000);
                // This goto will cause the above lock to unlock.
                goto try_again;
            }
        }
    }

    if (task) {
        task->free_rpc_task();
    }

    delete ctx;

    return success;
}

void nfs_client::access(fuse_req_t req, fuse_ino_t ino, int mask)
{
    struct rpc_task *tsk = rpc_task_helper->alloc_rpc_task(FUSE_ACCESS);

    tsk->init_access(req, ino, mask);
    tsk->run_access();
}

void nfs_client::flush(fuse_req_t req, fuse_ino_t ino)
{
    struct rpc_task *tsk = rpc_task_helper->alloc_rpc_task(FUSE_FLUSH);

    tsk->init_flush(req, ino);
    tsk->run_flush();
}

void nfs_client::write(fuse_req_t req, fuse_ino_t ino, struct fuse_bufvec *bufv, size_t size, off_t off)
{
    struct rpc_task *tsk = rpc_task_helper->alloc_rpc_task(FUSE_WRITE);

    tsk->init_write(req, ino, bufv, size, off);
    tsk->run_write();
}

void nfs_client::getattr(
    fuse_req_t req,
    fuse_ino_t ino,
    struct fuse_file_info* file)
{
    struct nfs_inode *inode = get_nfs_inode_from_ino(ino);

    /*
     * This is to satisfy a POSIX requirement which expects utime/stat to
     * return updated attributes after sync'ing any pending writes.
     * If there is lot of dirty data cached this might take very long, as
     * it'll wait for the entire data to be written and acknowledged by the
     * NFS server.
     *
     * TODO: If it turns out to cause bad user experience, we can explore
     *       updating nfs_inode::attr during writes and then returning
     *       attributes from that instead of making a getattr call here.
     *       We need to think carefully though.
     */
    if (inode->is_regfile()) {
        AZLogDebug("[{}] Flushing file data ahead of getattr",
                   inode->get_fuse_ino());
        inode->flush_cache_and_wait();
    }

    struct rpc_task *tsk = rpc_task_helper->alloc_rpc_task(FUSE_GETATTR);

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
    struct rpc_task *tsk = rpc_task_helper->alloc_rpc_task(FUSE_CREATE);

    tsk->init_create_file(req, parent_ino, name, mode, file);
    tsk->run_create_file();
}

void nfs_client::mknod(
    fuse_req_t req,
    fuse_ino_t parent_ino,
    const char* name,
    mode_t mode)
{
    struct rpc_task *tsk = rpc_task_helper->alloc_rpc_task(FUSE_MKNOD);

    tsk->init_mknod(req, parent_ino, name, mode);
    tsk->run_mknod();
}

void nfs_client::mkdir(
    fuse_req_t req,
    fuse_ino_t parent_ino,
    const char* name,
    mode_t mode)
{
    struct rpc_task *tsk = rpc_task_helper->alloc_rpc_task(FUSE_MKDIR);

    tsk->init_mkdir(req, parent_ino, name, mode);
    tsk->run_mkdir();
}

/*
 * Returns:
 *  true  - silly rename was needed and done.
 *  false - silly rename not needed.
 */
bool nfs_client::silly_rename(
    fuse_req_t req,
    fuse_ino_t parent_ino,
    const char* name)
{
    struct nfs_inode *parent_inode = get_nfs_inode_from_ino(parent_ino);
    // Inode of the file being silly renamed.
    struct nfs_inode *inode = parent_inode->lookup(name);

    /*
     * This is called from aznfsc_ll_unlink() for all unlinked files, so
     * this is a good place to remove the entry from DNLC.
     */
    parent_inode->dnlc_remove(name);

    /*
     * Note: VFS will hold the inode lock for the target file, so it won't
     *       go away till the rename_callback() is called (and we respond to
     *       fuse).
     */
    if (inode && inode->is_open()) {
        char newname[64];
        ::snprintf(newname, sizeof(newname), ".nfs_%lu_%lu_%d",
                   inode->get_fuse_ino(), inode->get_generation(),
                   inode->get_silly_rename_level());

        AZLogInfo("silly_rename: Renaming {}/{} -> {}, ino={}",
                  parent_ino, name, newname, inode->get_fuse_ino());

        rename(req, parent_ino, name, parent_ino, newname, true,
               inode->get_fuse_ino(), 0);
        return true;
    } else if (!inode) {
        AZLogError("silly_rename: Failed to get inode for file {}/{}. File "
                   "will be deleted, any process having file open will get "
                   "errors when accessing it!",
                   parent_ino, name);
    }

    return false;
}

void nfs_client::unlink(
    fuse_req_t req,
    fuse_ino_t parent_ino,
    const char* name)
{
    struct rpc_task *tsk = rpc_task_helper->alloc_rpc_task(FUSE_UNLINK);

    tsk->init_unlink(req, parent_ino, name);
    tsk->run_unlink();
}

void nfs_client::rmdir(
    fuse_req_t req,
    fuse_ino_t parent_ino,
    const char* name)
{
    struct rpc_task *tsk = rpc_task_helper->alloc_rpc_task(FUSE_RMDIR);
    struct nfs_inode *parent_inode = get_nfs_inode_from_ino(parent_ino);

    parent_inode->dnlc_remove(name);
    tsk->init_rmdir(req, parent_ino, name);
    tsk->run_rmdir();
}

void nfs_client::symlink(
    fuse_req_t req,
    const char* link,
    fuse_ino_t parent_ino,
    const char* name)
{
    struct rpc_task *tsk = rpc_task_helper->alloc_rpc_task(FUSE_SYMLINK);

    tsk->init_symlink(req, link, parent_ino, name);
    tsk->run_symlink();
}

void nfs_client::rename(
    fuse_req_t req,
    fuse_ino_t parent_ino,
    const char *name,
    fuse_ino_t newparent_ino,
    const char *new_name,
    bool silly_rename,
    fuse_ino_t silly_rename_ino,
    unsigned int flags)
{
    struct rpc_task *tsk = rpc_task_helper->alloc_rpc_task(FUSE_RENAME);
    struct nfs_inode *parent_inode = get_nfs_inode_from_ino(parent_ino);

    parent_inode->dnlc_remove(name);

    tsk->init_rename(req, parent_ino, name, newparent_ino, new_name,
                     silly_rename, silly_rename_ino, flags);
    tsk->run_rename();
}

void nfs_client::readlink(
    fuse_req_t req,
    fuse_ino_t ino)
{
    struct rpc_task *tsk = rpc_task_helper->alloc_rpc_task(FUSE_READLINK);

    tsk->init_readlink(req, ino);
    tsk->run_readlink();
}

void nfs_client::setattr(
    fuse_req_t req,
    fuse_ino_t ino,
    const struct stat* attr,
    int to_set,
    struct fuse_file_info* file)
{
    struct rpc_task *tsk = rpc_task_helper->alloc_rpc_task(FUSE_SETATTR);

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
    struct rpc_task *tsk = rpc_task_helper->alloc_rpc_task(FUSE_READDIR);
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
    struct rpc_task *tsk = rpc_task_helper->alloc_rpc_task(FUSE_READDIRPLUS);
    struct nfs_inode *inode = get_nfs_inode_from_ino(ino);

    // Force revalidate for offset==0 to ensure cto consistency.
    inode->revalidate(offset == 0);

    tsk->init_readdirplus(req, ino, size, offset, file);
    tsk->run_readdirplus();
}

void nfs_client::read(
    fuse_req_t req,
    fuse_ino_t ino,
    size_t size,
    off_t off,
    struct fuse_file_info *fi)
{
    struct rpc_task *tsk = rpc_task_helper->alloc_rpc_task(FUSE_READ);
    struct nfs_inode *inode = get_nfs_inode_from_ino(ino);

    // Revalidate if attribute cache timeout expired.
    inode->revalidate();

    tsk->init_read(req, ino, size, off, fi);

    /*
     * Allocate readahead_state if not already done.
     */
    inode->get_or_alloc_rastate();

    /*
     * Issue readaheads (if any) before application read.
     * Note that application read can block on membuf lock while readahead
     * read skips locked membufs. This way we can have readahead reads sent
     * to the server even while application read causes us to block.
     */
    [[maybe_unused]] const int num_ra =
        inode->readahead_state->issue_readaheads();

    AZLogDebug("[{}] {} readaheads issued for client read offset: {} size: {}",
               ino, num_ra, off, size);

    inode->readahead_state->on_application_read(off, size);
    tsk->run_read();
}

/*
 * This function will be called only to retry the write requests that failed
 * with JUKEBOX error.
 * rpc_api defines the RPC request that need to be retried.
 */
void nfs_client::jukebox_flush(struct api_task_info *rpc_api)
{
    /*
     * For write task pvt has write_iov_context, which has copy of byte_chunk vector.
     * To proceed it should be valid.
     */
    assert(rpc_api->pvt != nullptr);
    assert(rpc_api->optype == FUSE_FLUSH);

    struct rpc_task *flush_task =
        get_rpc_task_helper()->alloc_rpc_task(FUSE_FLUSH);
    flush_task->init_flush(nullptr /* fuse_req */,
                           rpc_api->flush_task.get_ino());
    // Any new task should start fresh as a parent task.
    assert(flush_task->rpc_api->parent_task == nullptr);

    [[maybe_unused]] struct write_iov_context *ctx =
        (struct write_iov_context *) rpc_api->pvt;
    assert(ctx->magic == WRITE_CONTEXT_MAGIC);

    /*
     * We currently only support buffered writes where the original fuse write
     * task completes after copying data to the bytes_chunk_cache and later
     * we sync the dirty membuf using one or more flush rpc_tasks whose sole
     * job is to ensure they sync the part of the blob they are assigned.
     * They don't need a parent_task which is usually the fuse task that needs
     * to be completed once the underlying tasks complete.
     */
    assert(rpc_api->parent_task == nullptr);

    flush_task->resissue_write_iovec(ctx->get_bc_vec(), ctx->get_ino());
    delete ctx;
}

/*
 * This function will be called only to retry the read requests that failed
 * with JUKEBOX error.
 * rpc_api defines the RPC request that need to be retried.
 */
void nfs_client::jukebox_read(struct api_task_info *rpc_api)
{
    assert(rpc_api->optype == FUSE_READ);

    struct rpc_task *child_tsk =
        get_rpc_task_helper()->alloc_rpc_task(FUSE_READ);

    child_tsk->init_read(
        rpc_api->req,
        rpc_api->read_task.get_ino(),
        rpc_api->read_task.get_size(),
        rpc_api->read_task.get_offset(),
        rpc_api->read_task.get_fuse_file());

    /*
     * Read API calls will be issued only for child tasks, hence
     * copy the parent info from the original task to this retry task.
     */
    assert(rpc_api->parent_task != nullptr);
    assert(rpc_api->parent_task->magic == RPC_TASK_MAGIC);
    child_tsk->rpc_api->parent_task = rpc_api->parent_task;

    [[maybe_unused]]  const struct rpc_task *const parent_task =
        child_tsk->rpc_api->parent_task;

    /*
     * Since we are retrying this child task, the parent read task should have
     * atleast 1 ongoing read.
     */
    assert(parent_task->num_ongoing_backend_reads > 0);

    /*
     * Child task should always read a subset of the parent task.
     */
    assert(child_tsk->rpc_api->read_task.get_offset() >=
            parent_task->rpc_api->read_task.get_offset());
    assert(child_tsk->rpc_api->read_task.get_size() <=
            parent_task->rpc_api->read_task.get_size());

    assert(rpc_api->bc != nullptr);

    // Jukebox retry is for an existing request issued to the backend.
    assert(rpc_api->bc->num_backend_calls_issued > 0);

#ifdef ENABLE_PARANOID
    {
        unsigned int i;
        for (i = 0; i < parent_task->bc_vec.size(); i++) {
            if (rpc_api->bc == &parent_task->bc_vec[i])
                break;
        }

        /*
         * rpc_api->bc MUST refer to one of the elements in
         * parent_task->bc_vec.
         */
        assert(i != parent_task->bc_vec.size());
    }
#endif

    /*
     * The jukebox retry task also should read into the same bc.
     */
    child_tsk->rpc_api->bc = rpc_api->bc;

    /*
     * The bytes_chunk held by this task must have its inuse count
     * bumped as the get() call made to obtain this chunk initially would
     * have set it.
     */
    assert(rpc_api->bc->get_membuf()->is_inuse());

    // Issue the read to the server
    child_tsk->read_from_server(*(rpc_api->bc));
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
    /*
     * Kernel must cache lookup result.
     */
    const bool cache_positive =
        (aznfsc_cfg.lookupcache_int == AZNFSCFG_LOOKUPCACHE_ALL ||
         aznfsc_cfg.lookupcache_int == AZNFSCFG_LOOKUPCACHE_POS);

    memset(&entry, 0, sizeof(entry));

    if (fh) {
        const fuse_ino_t parent_ino = ctx->rpc_api->get_parent_ino();
        struct nfs_inode *parent_inode =
            ctx->get_client()->get_nfs_inode_from_ino(parent_ino);

        /*
         * This will grab a lookupcnt ref on the inode, which will be freed
         * from fuse forget callback.
         */
        inode = get_nfs_inode(fh, fattr);

        entry.ino = inode->get_fuse_ino();
        entry.generation = inode->get_generation();
        entry.attr = inode->attr;
        if (cache_positive) {
            entry.attr_timeout = inode->get_actimeo();
            entry.entry_timeout = inode->get_actimeo();
        } else {
            entry.attr_timeout = 0;
            entry.entry_timeout = 0;
        }

        AZLogDebug("[{}] <{}> Returning ino {} to fuse (filename {})",
                   parent_ino,
                   rpc_task::fuse_opcode_to_string(ctx->rpc_api->optype),
                   inode->get_fuse_ino(),
                   ctx->rpc_api->get_file_name());

        parent_inode->dnlc_add(ctx->rpc_api->get_file_name(),
                               inode->get_fuse_ino());

        /*
         * This is the common place where we return inode to fuse.
         * After this fuse can call any of the functions that might need file
         * or dir cache, so allocate them now.
         */
        if (inode->is_regfile()) {
            inode->get_or_alloc_filecache();
        } else if (inode->is_dir()) {
            inode->get_or_alloc_dircache();
        }
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
         * TODO: See if negative entries must be cached for lesser time.
         */
        assert(aznfsc_cfg.lookupcache_int == AZNFSCFG_LOOKUPCACHE_ALL);
        assert(!fattr);
        stat_from_fattr3(&entry.attr, &zero_fattr);

        entry.attr_timeout = aznfsc_cfg.actimeo;
        entry.entry_timeout = aznfsc_cfg.actimeo;
    }

    if (file) {
        ctx->reply_create(&entry, file);
    } else {
        ctx->reply_entry(&entry);
    }
}

void nfs_client::jukebox_retry(struct rpc_task *task)
{
    {
        AZLogDebug("Queueing rpc_task {} for jukebox retry", fmt::ptr(task));

        /*
         * Transfer ownership of rpc_api from rpc_task to jukebox_seedinfo.
         */
        std::unique_lock<std::mutex> lock(jukebox_seeds_lock);
        jukebox_seeds.emplace(new jukebox_seedinfo(task->rpc_api));

        task->rpc_api = nullptr;
    }

    /*
     * Free the current task that failed with JUKEBOX error.
     * The retried task will use a new rpc_task structure (and new XID).
     * Note that we don't callback into fuse as yet.
     */
    task->free_rpc_task();
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
static void getattr_sync_callback(
    struct rpc_context *rpc,
    int rpc_status,
    void *data,
    void *private_data)
{
    auto ctx = (struct sync_rpc_context*) private_data;
    assert(ctx->magic == SYNC_RPC_CTX_MAGIC);
    auto res = (GETATTR3res*) data;

    rpc_task *task = ctx->task;

    ctx->rpc_status = rpc_status;
    ctx->nfs_status = NFS_STATUS(res);

    if (task) {
        assert(task->magic == RPC_TASK_MAGIC);
        assert(task->rpc_api->optype == FUSE_GETATTR);
        /*
         * Now that the request has completed, we can query libnfs for the
         * dispatch time.
         */
        task->get_stats().on_rpc_complete(rpc_get_pdu(rpc), NFS_STATUS(res));
    }

    {
        std::unique_lock<std::mutex> lock(ctx->mutex);

        // Must be called only once.
        assert(!ctx->callback_called);
        ctx->callback_called = true;

        if ((ctx->rpc_status == RPC_STATUS_SUCCESS) &&
                (ctx->nfs_status == NFS3_OK)) {
            assert(ctx->fattr);
            *(ctx->fattr) = res->GETATTR3res_u.resok.obj_attributes;
        }
    }

    ctx->cv.notify_one();
}

/**
 * Issue a sync GETATTR RPC call to filehandle 'fh' and save the received
 * attributes in 'fattr'.
 */
bool nfs_client::getattr_sync(const struct nfs_fh3& fh,
                              fuse_ino_t ino,
                              struct fattr3& fattr)
{
    const uint32_t fh_hash = calculate_crc32(
            (const unsigned char *) fh.data.data_val, fh.data.data_len);
    struct nfs_context *nfs_context = get_nfs_context(CONN_SCHED_FH_HASH, fh_hash);
    struct rpc_task *task = nullptr;
    struct sync_rpc_context *ctx = nullptr;
    struct rpc_pdu *pdu = nullptr;
    struct rpc_context *rpc;
    bool rpc_retry = false;
    bool success = false;

try_again:
    do {
        struct GETATTR3args args;
        args.object = fh;

        /*
         * Very first call to getattr_sync(), called from nfs_client::init(), for
         * getting the root filehandle attributes won't have the rpc_task_helper
         * set, so that single GETATTR RPC won't be accounted in rpc stats.
         */
        if (get_rpc_task_helper() != nullptr) {
            if (task) {
                task->free_rpc_task();
            }
            task = get_rpc_task_helper()->alloc_rpc_task(FUSE_GETATTR);
            task->init_getattr(nullptr /* fuse_req */, ino);
        } else {
            assert(ino == FUSE_ROOT_ID);
        }

        if (ctx) {
            delete ctx;
        }

        ctx = new sync_rpc_context(task, &fattr);
        rpc = nfs_get_rpc_context(nfs_context);

        rpc_retry = false;
        if (task) {
            task->get_stats().on_rpc_issue();
        }
        if ((pdu = rpc_nfs3_getattr_task(rpc, getattr_sync_callback,
                                         &args, ctx)) == NULL) {
            if (task) {
                task->get_stats().on_rpc_cancel();
            }
            /*
             * This call fails due to internal issues like OOM etc
             * and not due to an actual error, hence retry.
             */
            rpc_retry = true;
        }
    } while (rpc_retry);

    /*
     * If the GETATTR response doesn't come for 60 secs we give up and send
     * a new one. We must cancel the old one.
     */
    {
        std::unique_lock<std::mutex> lock(ctx->mutex);
wait_more:
        if (!ctx->cv.wait_for(lock, std::chrono::seconds(60),
                              [&ctx] { return (ctx->callback_called == true); })) {
            if (rpc_cancel_pdu(rpc, pdu) == 0) {
                if (task) {
                    task->get_stats().on_rpc_cancel();
                }
                AZLogWarn("Timed out waiting for getattr response, re-issuing "
                          "getattr!");
                // This goto will cause the above lock to unlock.
                goto try_again;
            } else {
                /*
                 * If rpc_cancel_pdu() fails it most likely means we got the RPC
                 * response right after we timed out waiting. It's best to wait
                 * for the callback to be called.
                 */
                AZLogWarn("Timed out waiting for getattr response, couldn't "
                          "cancel existing pdu, waiting some more!");
                // This goto will *not* cause the above lock to unlock.
                goto wait_more;
            }
        } else {
            assert(ctx->callback_called);
            assert(ctx->rpc_status != -1);
            assert(ctx->nfs_status != -1);

            if ((ctx->rpc_status == RPC_STATUS_SUCCESS) &&
                    (ctx->nfs_status == NFS3_OK)) {
                success = true;
            } else if (ctx->rpc_status == RPC_STATUS_SUCCESS &&
                       ctx->nfs_status == NFS3ERR_JUKEBOX) {
                AZLogInfo("Got NFS3ERR_JUKEBOX for GETATTR, re-issuing "
                          "after 1 sec!");
                ::usleep(1000 * 1000);
                // This goto will cause the above lock to unlock.
                goto try_again;
            }
        }
    }

    if (task) {
        task->free_rpc_task();
    }

    delete ctx;

    return success;
}
#endif
