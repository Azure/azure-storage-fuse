#include "nfs_internal.h"
#include "rpc_task.h"

#define RSTATUS(r) ((r) ? (r)->status : NFS3ERR_SERVERFAULT)

void rpc_task::init_lookup(fuse_req* request,
                           const char* name,
                           fuse_ino_t parent_ino)
{
    req = request;
    optype = FUSE_LOOKUP;
    rpc_api.lookup_task.set_file_name(name);
    rpc_api.lookup_task.set_parent_ino(parent_ino);
}

void rpc_task::init_getattr(fuse_req* request,
                            fuse_ino_t ino)
{
    req = request;
    optype = FUSE_GETATTR;
    rpc_api.getattr_task.set_ino(ino);
}

void rpc_task::init_create_file(fuse_req* request,
                                fuse_ino_t parent_ino,
                                const char* name,
                                mode_t mode,
                                struct fuse_file_info* file)
{
    req = request;
    optype = FUSE_CREATE;
    rpc_api.create_task.set_parent_ino(parent_ino);
    rpc_api.create_task.set_file_name(name);
    rpc_api.create_task.set_mode(mode);
    rpc_api.create_task.set_fuse_file(file);
}

void rpc_task::init_mkdir(fuse_req* request,
                          fuse_ino_t parent_ino,
                          const char* name,
                          mode_t mode)
{
    req = request;
    optype = FUSE_MKDIR;
    rpc_api.mkdir_task.set_parent_ino(parent_ino);
    rpc_api.mkdir_task.set_dir_name(name);
    rpc_api.mkdir_task.set_mode(mode);

}

void rpc_task::init_setattr(fuse_req* request,
                            fuse_ino_t ino,
                            struct stat* attr,
                            int to_set,
                            struct fuse_file_info* file)
{
    req = request;
    optype = FUSE_SETATTR;
    rpc_api.setattr_task.set_ino(ino);
    rpc_api.setattr_task.set_fuse_file(file);
    rpc_api.setattr_task.set_attribute_and_mask(attr, to_set);
}

void rpc_task::init_readdir(fuse_req *request,
                            fuse_ino_t ino,
                            size_t size,
                            off_t offset,
                            struct fuse_file_info *file)
{
    req = request;
    optype = FUSE_READDIR;
    rpc_api.readdir_task.set_inode(ino);
    rpc_api.readdir_task.set_size(size);
    rpc_api.readdir_task.set_offset(offset);
    rpc_api.readdir_task.set_fuse_file(file);
}

void rpc_task::init_readdirplus(fuse_req *request,
                                fuse_ino_t ino,
                                size_t size,
                                off_t offset,
                                struct fuse_file_info *file)
{
    req = request;
    optype = FUSE_READDIRPLUS;
    rpc_api.readdir_task.set_inode(ino);
    rpc_api.readdir_task.set_size(size);
    rpc_api.readdir_task.set_offset(offset);
    rpc_api.readdir_task.set_fuse_file(file);
}

/*
 * TODO: All the RPC callbacks where we receive post-op attributes or receive
 *       attributes o/w, we must call nfs_inode::update() to update the
 *       currently cached attributes. That will invalidate the cache if newly
 *       received attributes indicate file data has changed.
 */

static void getattr_callback(
    struct rpc_context* /* rpc */,
    int rpc_status,
    void *data,
    void *private_data)
{
    rpc_task *task = (rpc_task*) private_data;
    auto res = (GETATTR3res*)data;
    const fuse_ino_t ino =
        task->rpc_api.getattr_task.get_ino();
    struct nfs_inode *inode =
        task->get_client()->get_nfs_inode_from_ino(ino);

    if (task->succeeded(rpc_status, RSTATUS(res))) {
        // Got fresh attributes, update the attributes cached in the inode.
        inode->update(res->GETATTR3res_u.resok.obj_attributes);

        /*
         * Set fuse kernel attribute cache timeout to the current attribute
         * cache timeout for this inode, as per the recent revalidation
         * experience.
         */
        task->reply_attr(&inode->attr, inode->get_actimeo());
    }
    else
    {
        // Since the api failed and can no longer be retried, return error reply.
        task->reply_error(-nfsstat3_to_errno(RSTATUS(res)));
    }
}

static void lookup_callback(
    struct rpc_context* /* rpc */,
    int rpc_status,
    void *data,
    void *private_data)
{
    rpc_task *task = (rpc_task*) private_data;
    auto res = (LOOKUP3res*)data;

    if (rpc_status == RPC_STATUS_SUCCESS && RSTATUS(res) == NFS3ERR_NOENT) {
        /*
         * Special case for creating negative dentry.
         */
        task->get_client()->reply_entry(
            task,
            nullptr /* fh */,
            nullptr /* fattr */,
            nullptr);
    } else if(task->succeeded(rpc_status, RSTATUS(res))) {
        assert(res->LOOKUP3res_u.resok.obj_attributes.attributes_follow);

        task->get_client()->reply_entry(
            task,
            &res->LOOKUP3res_u.resok.object,
            &res->LOOKUP3res_u.resok.obj_attributes.post_op_attr_u.attributes,
            nullptr);
    }
    else
    {
        // Since the api failed and can no longer be retried, return error reply.
        task->reply_error(-nfsstat3_to_errno(RSTATUS(res)));
    }
}

static void createfile_callback(
    struct rpc_context* /* rpc */,
    int rpc_status,
    void *data,
    void *private_data)
{
    rpc_task *task = (rpc_task*) private_data;
    auto res = (CREATE3res*)data;

    if (task->succeeded(rpc_status, RSTATUS(res)))
    {
        assert(
            res->CREATE3res_u.resok.obj.handle_follows &&
            res->CREATE3res_u.resok.obj_attributes.attributes_follow);

        task->get_client()->reply_entry(
            task,
            &res->CREATE3res_u.resok.obj.post_op_fh3_u.handle,
            &res->CREATE3res_u.resok.obj_attributes.post_op_attr_u.attributes,
            task->rpc_api.create_task.get_file());
    }
    else
    {
        // Since the api failed and can no longer be retried, return error reply.
        task->reply_error(-nfsstat3_to_errno(RSTATUS(res)));
    }
}

static void setattr_callback(
    struct rpc_context* /* rpc */,
    int rpc_status,
    void *data,
    void *private_data)
{
    rpc_task *task = (rpc_task*) private_data;
    auto res = (SETATTR3res*)data;
    const fuse_ino_t ino =
        task->rpc_api.setattr_task.get_ino();
    const struct nfs_inode *inode =
        task->get_client()->get_nfs_inode_from_ino(ino);

    if (task->succeeded(rpc_status, RSTATUS(res)))
    {
        assert(res->SETATTR3res_u.resok.obj_wcc.after.attributes_follow);

        struct stat st;

        task->get_client()->stat_from_fattr3(
            &st, &res->SETATTR3res_u.resok.obj_wcc.after.post_op_attr_u.attributes);
        /*
         * Set fuse kernel attribute cache timeout to the current attribute
         * cache timeout for this inode, as per the recent revalidation
         * experience.
         */
        task->reply_attr(&st, inode->get_actimeo());
    }
    else
    {
        // Since the api failed and can no longer be retried, return error reply.
        task->reply_error(-nfsstat3_to_errno(RSTATUS(res)));
    }
}

void mkdir_callback(
    struct rpc_context* /* rpc */,
    int rpc_status,
    void *data,
    void *private_data)
{
    rpc_task *task = (rpc_task*) private_data;
    auto res = (MKDIR3res*)data;

    if (task->succeeded(rpc_status, RSTATUS(res)))
    {
        assert(
            res->MKDIR3res_u.resok.obj.handle_follows &&
            res->MKDIR3res_u.resok.obj_attributes.attributes_follow);

        task->get_client()->reply_entry(
            task,
            &res->MKDIR3res_u.resok.obj.post_op_fh3_u.handle,
            &res->MKDIR3res_u.resok.obj_attributes.post_op_attr_u.attributes,
            nullptr);
    }
    else
    {
        // Since the api failed and can no longer be retried, return error reply.
        task->reply_error(-nfsstat3_to_errno(RSTATUS(res)));
    }
}

void rpc_task::run_lookup()
{
    fuse_ino_t parent_ino = rpc_api.lookup_task.get_parent_ino();
    bool rpc_retry = false;

    do {
        LOOKUP3args args;

        args.what.dir = get_client()->get_nfs_inode_from_ino(parent_ino)->get_fh();
        args.what.name = (char*) rpc_api.lookup_task.get_file_name();

        if (rpc_nfs3_lookup_task(get_rpc_ctx(), lookup_callback, &args, this) == NULL) {
            /*
             * This call fails due to internal issues like OOM etc
             * and not due to an actual error, hence retry.
             */
            rpc_retry = true;
        }
    } while (rpc_retry);
}

void rpc_task::run_getattr()
{
    bool rpc_retry = false;
    auto ino = rpc_api.getattr_task.get_ino();

    do {
        struct GETATTR3args args;
        ::memset(&args, 0, sizeof(args));
        args.object = get_client()->get_nfs_inode_from_ino(ino)->get_fh();

        if (rpc_nfs3_getattr_task(get_rpc_ctx(), getattr_callback, &args, this) == NULL)
        {
            /*
             * This call fails due to internal issues like OOM etc
             * and not due to an actual error, hence retry.
             */
            rpc_retry = true;
        }
    } while (rpc_retry);
}

void rpc_task::run_create_file()
{
    bool rpc_retry = false;
    auto parent_ino = rpc_api.create_task.get_parent_ino();

    do {
        CREATE3args args;
        ::memset(&args, 0, sizeof(args));
        args.where.dir = get_client()->get_nfs_inode_from_ino(parent_ino)->get_fh();
        args.where.name = (char*)rpc_api.create_task.get_file_name();
        args.how.mode = (rpc_api.create_task.get_file()->flags & O_EXCL) ? GUARDED : UNCHECKED;
        args.how.createhow3_u.obj_attributes.mode.set_it = 1;
        args.how.createhow3_u.obj_attributes.mode.set_mode3_u.mode = rpc_api.create_task.get_mode();

        if (rpc_nfs3_create_task(get_rpc_ctx(), createfile_callback, &args, this) == NULL)
        {
            /*
             * This call fails due to internal issues like OOM etc
             * and not due to an actual error, hence retry.
             */
            rpc_retry = true;
        }
    }  while (rpc_retry);
}

void rpc_task::run_mkdir()
{
    bool rpc_retry = false;
    auto parent_ino = rpc_api.mkdir_task.get_parent_ino();

    do {
        MKDIR3args args;
        ::memset(&args, 0, sizeof(args));
        args.where.dir = get_client()->get_nfs_inode_from_ino(parent_ino)->get_fh();
        args.where.name = (char*)rpc_api.mkdir_task.get_file_name();
        args.attributes.mode.set_it = 1;
        args.attributes.mode.set_mode3_u.mode = rpc_api.mkdir_task.get_mode();

        if (rpc_nfs3_mkdir_task(get_rpc_ctx(), mkdir_callback, &args, this) == NULL)
        {
            /*
             * This call fails due to internal issues like OOM etc
             * and not due to an actual error, hence retry.
             */
            rpc_retry = true;
        }
    } while (rpc_retry);
}

void rpc_task::run_setattr()
{
    auto ino = rpc_api.setattr_task.get_ino();
    auto attr = rpc_api.setattr_task.get_attr();
    const int valid = rpc_api.setattr_task.get_attr_flags_to_set();
    bool rpc_retry = false;

    do {
        SETATTR3args args;
        ::memset(&args, 0, sizeof(args));
        args.object = get_client()->get_nfs_inode_from_ino(ino)->get_fh();

        if (valid & FUSE_SET_ATTR_SIZE) {
            AZLogInfo("Setting size to {}", attr->st_size);

            args.new_attributes.size.set_it = 1;
            args.new_attributes.size.set_size3_u.size = attr->st_size;
        }

        if (valid & FUSE_SET_ATTR_MODE) {
            AZLogInfo("Setting mode to {}", attr->st_mode);

            args.new_attributes.mode.set_it = 1;
            args.new_attributes.mode.set_mode3_u.mode = attr->st_mode;
        }

        if (valid & FUSE_SET_ATTR_UID) {
            AZLogInfo("Setting uid to {}", attr->st_uid);
            args.new_attributes.uid.set_it = 1;
            args.new_attributes.uid.set_uid3_u.uid = attr->st_uid;
        }

        if (valid & FUSE_SET_ATTR_GID) {
            AZLogInfo("Setting gid to {}", attr->st_gid);

            args.new_attributes.gid.set_it = 1;
            args.new_attributes.gid.set_gid3_u.gid = attr->st_gid;
        }

        if (valid & FUSE_SET_ATTR_ATIME) {
            // TODO: These log are causing crash, look at it later.
            // AZLogInfo("Setting atime to {}", attr->st_atim.tv_sec);

            args.new_attributes.atime.set_it = SET_TO_CLIENT_TIME;
            args.new_attributes.atime.set_atime_u.atime.seconds =
                attr->st_atim.tv_sec;
            args.new_attributes.atime.set_atime_u.atime.nseconds =
                attr->st_atim.tv_nsec;
        }

        if (valid & FUSE_SET_ATTR_MTIME) {
            // TODO: These log are causing crash, look at it later.
            // AZLogInfo("Setting mtime to {}", attr->st_mtim.tv_sec);

            args.new_attributes.mtime.set_it = SET_TO_CLIENT_TIME;
            args.new_attributes.mtime.set_mtime_u.mtime.seconds =
                attr->st_mtim.tv_sec;
            args.new_attributes.mtime.set_mtime_u.mtime.nseconds =
                attr->st_mtim.tv_nsec;
        }

        if (valid & FUSE_SET_ATTR_ATIME_NOW) {
            args.new_attributes.atime.set_it = SET_TO_SERVER_TIME;
        }

        if (valid & FUSE_SET_ATTR_MTIME_NOW) {
            args.new_attributes.mtime.set_it = SET_TO_SERVER_TIME;
        }

        if (rpc_nfs3_setattr_task(get_rpc_ctx(), setattr_callback, &args, this) == NULL)
        {
            /*
             * This call fails due to internal issues like OOM etc
             * and not due to an actual error, hence retry.
             */
            rpc_retry = true;
        }
    } while (rpc_retry);
}

void rpc_task::free_rpc_task()
{
    switch(get_op_type()) {
    case FUSE_LOOKUP:
        rpc_api.lookup_task.release();
        break;
    case FUSE_CREATE:
        rpc_api.create_task.release();
        break;
    case FUSE_MKDIR:
        rpc_api.mkdir_task.release();
        break;
    default :
        break;
    }
    client->get_rpc_task_helper()->free_rpc_task(this);
}

struct nfs_context* rpc_task::get_nfs_context() const
{
    return client->get_nfs_context();
}

void rpc_task::run_readdir()
{
    get_readdir_entries_from_cache();
}

void rpc_task::run_readdirplus()
{
    get_readdir_entries_from_cache();
}

/*
 * Callback for the READDIR RPC. Once this callback is called, it will first
 * populate the readdir cache with the newly fetched entries (minus the
 * attributes). Additionally it will populate the readdirentries vector and
 * call send_readdir_response() to respond to the fuse readdir call.
 */
static void readdir_callback(
    struct rpc_context* /* rpc */,
    int rpc_status,
    void *data,
    void *private_data)
{
    rpc_task *task = (rpc_task*) private_data;
    assert(task->get_op_type() == FUSE_READDIR);
    READDIR3res *res = (READDIR3res*) data;
    const fuse_ino_t dir_ino = task->rpc_api.readdir_task.get_inode();
    struct nfs_inode *const dir_inode = task->get_client()->get_nfs_inode_from_ino(dir_ino);
    // How many max bytes worth of entries data does the caller want?
    ssize_t rem_size = task->rpc_api.readdir_task.get_size();
    std::vector<const directory_entry*> readdirentries;
    int num_dirents = 0;

    if (task->succeeded(rpc_status, RSTATUS(res))) {
        const struct entry3 *entry = res->READDIR3res_u.resok.reply.entries;
        const bool eof = res->READDIR3res_u.resok.reply.eof;
        int64_t eof_cookie = -1;

        // Get handle to the readdirectory cache.
        std::shared_ptr<readdirectory_cache> readdircache_handle = dir_inode->dircache_handle;
        assert(readdircache_handle != nullptr);

        // Process all dirents received.
        while (entry) {
            /*
             * Keep updating eof_cookie, when we exit the loop we will have
             * eof_cookie set correctly.
             */
            if (eof) {
                eof_cookie = entry->cookie;
            }

            /*
             * Create the directory entries here.
             * Note: This will be freed in the destructor
             *       ~readdirectory_cache(), or when we purge the readdir
             *       cache as a result of inode invalidate (when file
             *       changes).
             * TODO: Try to steal entry->name to avoid the strdup().
             */
            struct directory_entry* dir_entry =
                new directory_entry(strdup(entry->name),
                                    entry->cookie,
                                    entry->fileid);

            // Add to readdirectory_cache for future use.
            readdircache_handle->add(dir_entry);

            /*
             * Add it to the directory_entry vector but ONLY upto the byte
             * limit requested by fuse readdir call.
             */
            if (rem_size >= 0) {
                rem_size -= dir_entry->get_fuse_buf_size(false /* readdirplus */);
                if (rem_size >= 0) {
                    readdirentries.push_back(dir_entry);
                }
            }

            entry = entry->nextentry;
            ++num_dirents;
        }

        AZLogDebug("readdir_callback: Num of entries returned by server is {}, "
                   "returned to fuse: {}", num_dirents, readdirentries.size());

        readdircache_handle->set_cookieverf(&res->READDIR3res_u.resok.cookieverf);

        if (eof) {
            /*
             * If we pass the last cookie or beyond it, then server won't
             * return any directory entries, but it'll set eof to true.
             * In such case, we must already have set eof and eof_cookie.
             */
            if (eof_cookie != -1) {
                readdircache_handle->set_eof(eof_cookie);
            } else {
                assert(readdircache_handle->get_eof() == true);
                assert((int64_t) readdircache_handle->get_eof_cookie() != -1);
            }
        }

        task->send_readdir_response(readdirentries);
    } else {
        // Since the api failed and can no longer be retried, return error reply.
        task->reply_error(-nfsstat3_to_errno(RSTATUS(res)));
    }
}

/*
 * Callback for the READDIR RPC. Once this callback is called, it will first
 * populate the readdir cache with the newly fetched entries (with the
 * attributes). Additionally it will populate the readdirentries vector and
 * call send_readdir_response() to respond to the fuse readdir call.
 */
static void readdirplus_callback(
    struct rpc_context* /* rpc */,
    int rpc_status,
    void *data,
    void *private_data)
{
    rpc_task *task = (rpc_task*) private_data;
    assert(task->get_op_type() == FUSE_READDIRPLUS);
    READDIRPLUS3res *res = (READDIRPLUS3res*) data;
    const fuse_ino_t dir_ino = task->rpc_api.readdir_task.get_inode();
    struct nfs_inode *dir_inode = task->get_client()->get_nfs_inode_from_ino(dir_ino);
    // How many max bytes worth of entries data does the caller want?
    ssize_t rem_size = task->rpc_api.readdir_task.get_size();
    std::vector<const directory_entry*> readdirentries;
    int num_dirents = 0;

    if (task->succeeded(rpc_status, RSTATUS(res))) {
        const struct entryplus3 *entry = res->READDIRPLUS3res_u.resok.reply.entries;
        const bool eof = res->READDIRPLUS3res_u.resok.reply.eof;
        int64_t eof_cookie = -1;

        // Get handle to the readdirectory cache.
        std::shared_ptr<readdirectory_cache> readdircache_handle = dir_inode->dircache_handle;
        assert(readdircache_handle != nullptr);

        while (entry) {
            const struct fattr3 *fattr = nullptr;
            const bool is_dot_or_dotdot = directory_entry::is_dot_or_dotdot(entry->name);

            /*
             * Keep updating eof_cookie, when we exit the loop we will have
             * eof_cookie set correctly.
             */
            if (eof) {
                eof_cookie = entry->cookie;
            }

            if (entry->name_attributes.attributes_follow) {
                fattr = &(entry->name_attributes.post_op_attr_u.attributes);

                // Blob NFS will never send these two different.
                assert(fattr->fileid == entry->fileid);
            }

            /*
             * Create a new nfs inode for the entry.
             *
             * TODO: We should not blindly create a new nfs_inode, instead we
             *       MUST check if this inode exists and if yes, use that.
             */
            struct nfs_inode *const nfs_inode =
                task->get_client()->get_nfs_inode(
                    &entry->name_handle.post_op_fh3_u.handle, fattr);

            if (!fattr) {
                /*
                 * If readdirplus entry doesn't carry attributes, then we
                 * just save the inode number and filetype as DT_UNKNOWN.
                 *
                 * Blob NFS though must always send attributes in a readdirplus
                 * response.
                 */
                assert(0);
                nfs_inode->attr.st_ino = entry->fileid;
                nfs_inode->attr.st_mode = 0;
            }

            /*
             * Create the directory entries here.
             * Note: This will be freed in the destructor
             *       ~readdirectory_cache(), or when we purge the readdir
             *       cache as a result of inode invalidate (when file
             *       changes).
             * TODO: Try to steal entry->name to avoid the strdup().
             */
            struct directory_entry *dir_entry =
                new struct directory_entry(strdup(entry->name),
                                    entry->cookie,
                                    nfs_inode->attr,
                                    nfs_inode);

            /*
             * dir_entry must have one ref on the inode.
             * This ref will protect the inode while this directory_entry is
             * present in the readdirectory_cache (added below).
             */
            assert(nfs_inode->dircachecnt >= 1);

            // Add to readdirectory_cache for future use.
            readdircache_handle->add(dir_entry);

            /*
             * Add it to the directory_entry vector but ONLY upto the byte
             * limit requested by fuse readdirplus call.
             */
            if (rem_size >= 0) {
                rem_size -= dir_entry->get_fuse_buf_size(true /* readdirplus */);
                if (rem_size >= 0) {
                    /*
                     * send_readdir_response() will drop this ref after adding
                     * to fuse buf, after which the lookupcnt ref will protect
                     * it.
                     */
                    nfs_inode->dircachecnt++;
                    readdirentries.push_back(dir_entry);

                    /*
                     * Fuse promises to call FORGET for each readdirplus
                     * returned entry that is not "." or "..". Note that
                     * get_nfs_inode() above would have grabbed a lookupcnt
                     * ref for all entries, here we drop ref on "." and "..",
                     * so we are only left with ref on non "." and "..".
                     */
                    if (is_dot_or_dotdot) {
                        nfs_inode->decref();
                    }
                } else {
                    /*
                     * We are unable to add this entry to the fuse response
                     * buffer, so we won't notify fuse of this entry.
                     * Drop the ref held by get_nfs_inode().
                     */
                    AZLogDebug("[{}] {}: Dropping ref since couldn't fit in "
                               "fuse response buffer",
                               nfs_inode->get_fuse_ino(),
                               entry->name);
                    nfs_inode->decref();
                }
            } else {
                AZLogDebug("[{}] {}: Dropping ref since couldn't fit in "
                           "fuse response buffer",
                           nfs_inode->get_fuse_ino(),
                           entry->name);
                nfs_inode->decref();
            }

            entry = entry->nextentry;
            ++num_dirents;
        }

        AZLogDebug("readdir_callback: Num of entries returned by server is {}, "
                   "returned to fuse: {}", num_dirents, readdirentries.size());

        readdircache_handle->set_cookieverf(&res->READDIRPLUS3res_u.resok.cookieverf);

        if (eof) {
            /*
             * If we pass the last cookie or beyond it, then server won't
             * return any directory entries, but it'll set eof to true.
             * In such case, we must already have set eof and eof_cookie.
             */
            if (eof_cookie != -1) {
                readdircache_handle->set_eof(eof_cookie);
            } else {
                assert(readdircache_handle->get_eof() == true);
                assert((int64_t) readdircache_handle->get_eof_cookie() != -1);
            }
        }

        task->send_readdir_response(readdirentries);
    } else {
        // Since the api failed and can no longer be retried, return error reply.
        task->reply_error(-nfsstat3_to_errno(RSTATUS(res)));
    }
}

void rpc_task::get_readdir_entries_from_cache()
{
    const bool readdirplus = (get_op_type() == FUSE_READDIRPLUS);
    struct nfs_inode *nfs_inode =
        get_client()->get_nfs_inode_from_ino(rpc_api.readdir_task.get_inode());
    bool is_eof = false;

    std::vector<const directory_entry*> readdirentries;

    /*
     * Query requested directory entries from the readdir cache.
     * Requested directory entries are the ones with cookie after the one
     * requested by the client.
     * Note that Blob NFS uses cookie values that increase by 1 for every file.
     */
    nfs_inode->lookup_dircache(rpc_api.readdir_task.get_offset() + 1,
                               rpc_api.readdir_task.get_size(),
                               readdirentries,
                               is_eof,
                               readdirplus);

    /*
     * If eof is already received don't ask any more entries from the server.
     */
    if (readdirentries.empty() && !is_eof) {
        /*
         * Read from the backend only if there is no entry present in the
         * cache.
         * Note: It is okay to send less number of entries than requested since
         *       the Fuse layer will request for more num of entries later.
         */
        if (readdirplus) {
            fetch_readdirplus_entries_from_server();
        } else {
            fetch_readdir_entries_from_server();
        }
    } else {
        // We are done fetching the entries, send the response now.
        send_readdir_response(readdirentries);
    }
}

void rpc_task::fetch_readdir_entries_from_server()
{
    bool rpc_retry = false;
    const fuse_ino_t dir_ino = rpc_api.readdir_task.get_inode();
    struct nfs_inode *dir_inode = get_client()->get_nfs_inode_from_ino(dir_ino);
    const cookie3 cookie = rpc_api.readdir_task.get_offset();

    do {
        struct READDIR3args args;

        args.dir = dir_inode->get_fh();
        args.cookie = cookie;
        ::memcpy(&args.cookieverf,
                 dir_inode->dircache_handle->get_cookieverf(),
                 sizeof(args.cookieverf));
        args.count = 1048576; // TODO: Set this to user passed value.

        if (rpc_nfs3_readdir_task(get_rpc_ctx(),
                                  readdir_callback,
                                  &args,
                                  this) == NULL) {
            /*
             * This call fails due to internal issues like OOM etc
             * and not due to an actual error, hence retry.
             */
            rpc_retry = true;
        }
    } while (rpc_retry);
}

void rpc_task::fetch_readdirplus_entries_from_server()
{
    bool rpc_retry = false;
    const fuse_ino_t dir_ino = rpc_api.readdir_task.get_inode();
    struct nfs_inode *dir_inode = get_client()->get_nfs_inode_from_ino(dir_ino);
    const cookie3 cookie = rpc_api.readdir_task.get_offset();

    do {
        struct READDIRPLUS3args args;

        args.dir = dir_inode->get_fh();
        args.cookie = cookie;
        ::memcpy(&args.cookieverf,
                 dir_inode->dircache_handle->get_cookieverf(),
                 sizeof(args.cookieverf));

        args.dircount = 1048576; // TODO: Set this to user passed value.
        args.maxcount = 1048576;

        if (rpc_nfs3_readdirplus_task(get_rpc_ctx(),
                                      readdirplus_callback,
                                      &args,
                                      this) == NULL) {
            /*
             * This call fails due to internal issues like OOM etc
             * and not due to an actual error, hence retry.
             */
            rpc_retry = true;
        }
    } while (rpc_retry);
}

void rpc_task::send_readdir_response(
        const std::vector<const directory_entry*>& readdirentries)
{
    const bool readdirplus = (get_op_type() == FUSE_READDIRPLUS);
    /*
     * Max size the fuse buf is allowed to take.
     * We will allocate this much and then fill as many directory entries can
     * fit in this. Since the caller would have also considered this same size
     * while filling entries in readdirentries, we will usually be able to
     * consume all the entries in readdirentries.
     */
    const size_t size = rpc_api.readdir_task.get_size();

    // Fuse always requests 4096 bytes.
    assert(size >= 4096);

    // Allocate fuse response buffer.
    char *buf1 = (char *) malloc(size);
    if (!buf1) {
        reply_error(ENOMEM);
        return;
    }

    char *current_buf = buf1;
    size_t rem = size;
    int num_entries_added = 0;

    AZLogDebug("send_readdir_response: Number of directory entries to send {}",
               readdirentries.size());

    for (const auto& it : readdirentries) {
        /*
         * Caller should make sure that it adds only directory entries after
         * what was requested in the READDIR{PLUS} call to readdirentries.
         */
        assert((uint64_t) it->cookie > (uint64_t) rpc_api.readdir_task.get_offset());
        size_t entsize;

        if (readdirplus) {
            struct fuse_entry_param fuseentry;

            /*
             * Drop the ref held inside lookup() or readdirplus_callback().
             */
            it->nfs_inode->dircachecnt--;

            // We don't need the memset as we are setting all members.
            //memset(&fuseentry, 0, sizeof(fuseentry));
            fuseentry.attr = it->attributes;
            fuseentry.ino = it->nfs_inode->ino;

            // XXX: Do we need to worry about generation?
            fuseentry.generation = 0;

            fuseentry.attr_timeout = it->nfs_inode->get_actimeo();
            fuseentry.entry_timeout = it->nfs_inode->get_actimeo();

            /*
             * Insert the entry into the buffer.
             * If the buffer space is less, fuse_add_direntry_plus will not
             * add entry to the buffer but will still return the space needed
             * to add this entry.
             */
            entsize = fuse_add_direntry_plus(get_req(),
                    current_buf,
                    rem, /* size left in the buffer */
                    it->name,
                    &fuseentry,
                    it->cookie);
        } else {
            /*
             * Insert the entry into the buffer.
             * If the buffer space is less, fuse_add_direntry will not add
             * entry to the buffer but will still return the space needed to
             * add this entry.
             */
            entsize = fuse_add_direntry(get_req(),
                    current_buf,
                    rem, /* size left in the buffer */
                    it->name,
                    &it->attributes,
                    it->cookie);
        }

        /*
         * Our buffer size was small and hence we can't add any more entries,
         * so just break the loop. This also means that we have not inserted
         * the current entry to the dirent buffer.
         *
         * Note: This should not happen since the caller would have filled
         *       just enough entries in readdirentries.
         */
        if (entsize > rem) {
            break;
        }

        // Increment the buffer pointer to point to the next free space.
        current_buf += entsize;
        rem -= entsize;
        num_entries_added++;

        if (readdirplus) {
            /*
             * Fuse expects lookupcnt of every entry returned by readdirplus(),
             * except "." and "..", to be incremented. Make sure get_nfs_inode()
             * has duly taken the refs.
             *
             * TODO: If fuse_reply_buf() below fails we must drop these refcnts.
             */
            if (!it->is_dot_or_dotdot()) {
                assert(it->nfs_inode->lookupcnt > 0);
            }
        }
    }

    AZLogDebug("Num of entries sent in readdir response is {}", num_entries_added);

    if (fuse_reply_buf(get_req(), buf1, size - rem) != 0) {
        AZLogError("fuse_reply_buf failed!");
    }

    free(buf1);
    free_rpc_task();
}
