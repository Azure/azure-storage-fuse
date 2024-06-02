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

static void getattr_callback(
    struct rpc_context* /* rpc */,
    int rpc_status,
    void* data,
    void* private_data)
{
    auto task = (rpc_task*)private_data;
    auto res = (GETATTR3res*)data;

    if (task->succeeded(rpc_status, RSTATUS(res)))
    {
        struct stat st;
        task->get_client()->stat_from_fattr3(
            &st, &res->GETATTR3res_u.resok.obj_attributes);

        // TODO: Set the Attr timeout to a better value.
        task->reply_attr(&st, 60/*getAttrTimeout()*/);
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
    void* data,
    void* private_data)
{
    auto task = (rpc_task*)private_data;
    auto res = (LOOKUP3res*)data;

    if (rpc_status == RPC_STATUS_SUCCESS && RSTATUS(res) == NFS3ERR_NOENT)
    {
        //
        // Special case for fuse: A "negative entry" refers to an entry that doesn't exist
        // in the file system. If we want negative cache, we must not return ENOENT,
        // instead we should return success with zero inode.
        // When the FUSE kernel module receives a negative entry response, it may cache this
        // information for a certain duration specified by the entry_timeout parameter.
        // This caching helps to improve performance by avoiding repeated lookup requests
        // for entries that are known not to exist.
        //
        struct fattr3 dummyAttr;
        ::memset(&dummyAttr, 0, sizeof(dummyAttr));

        task->get_client()->reply_entry(
            task,
            nullptr /* fh */,
            &dummyAttr,
            nullptr);
    }
    else if(task->succeeded(rpc_status, RSTATUS(res)))
    {
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
    void* data,
    void* private_data)
{
    auto task = (rpc_task*)private_data;
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
    void* data,
    void* private_data)
{
    auto task = (rpc_task*)private_data;
    auto res = (SETATTR3res*)data;

    if (task->succeeded(rpc_status, RSTATUS(res)))
    {
        assert(res->SETATTR3res_u.resok.obj_wcc.after.attributes_follow);

        struct stat st;
        task->get_client()->stat_from_fattr3(
            &st, &res->SETATTR3res_u.resok.obj_wcc.after.post_op_attr_u.attributes);
        task->reply_attr(&st, 60 /* TODO: Set reasonable value nfs_client::getAttrTimeout() */);
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
    void* data,
    void* private_data)
{
    auto task = (rpc_task*)private_data;
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

// This is the task responsible for making the lookup task.
// lookup_task structure should be populated before calling this function.
void rpc_task::run_lookup()
{
    bool rpc_retry = false;
    auto parent_ino = rpc_api.lookup_task.get_parent_ino();

    do {
        LOOKUP3args args;
        ::memset(&args, 0, sizeof(args));
        args.what.dir = get_client()->get_nfs_inode_from_ino(parent_ino)->get_fh();
        args.what.name = (char*)rpc_api.lookup_task.get_file_name();

        if (rpc_nfs3_lookup_task(get_rpc_ctx(), lookup_callback, &args, this) == NULL)
        {
            // This call fails due to internal issues like OOM etc
            // and not due to an actual error, hence retry.
            rpc_retry = true;
        }
    } while (rpc_retry);
}

void rpc_task::run_getattr()
{
    bool rpc_retry = false;
    auto inode = rpc_api.getattr_task.get_ino();

    do {
        struct GETATTR3args args;
        ::memset(&args, 0, sizeof(args));
        args.object = get_client()->get_nfs_inode_from_ino(inode)->get_fh();

        if (rpc_nfs3_getattr_task(get_rpc_ctx(), getattr_callback, &args, this) == NULL)
        {
            // This call fails due to internal issues like OOM etc
            // and not due to an actual error, hence retry.
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
            // This call fails due to internal issues like OOM etc
            // and not due to an actual error, hence retry.
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
            // This call fails due to internal issues like OOM etc
            // and not due to an actual error, hence retry.
            rpc_retry = true;
        }
    } while (rpc_retry);
}

void rpc_task::run_setattr()
{
    auto inode = rpc_api.setattr_task.get_ino();
    auto attr = rpc_api.setattr_task.get_attr();
    const int valid = rpc_api.setattr_task.get_attr_flags_to_set();
    bool rpc_retry = false;

    do {
        SETATTR3args args;
        ::memset(&args, 0, sizeof(args));
        args.object = get_client()->get_nfs_inode_from_ino(inode)->get_fh();

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
            AZLogInfo("Setting atime to now");
            args.new_attributes.atime.set_it = SET_TO_SERVER_TIME;
        }

        if (valid & FUSE_SET_ATTR_MTIME_NOW) {
            AZLogInfo("Setting mtime to now");
            args.new_attributes.mtime.set_it = SET_TO_SERVER_TIME;
        }

        if (rpc_nfs3_setattr_task(get_rpc_ctx(), setattr_callback, &args, this) == NULL)
        {
            // This call fails due to internal issues like OOM etc
            // and not due to an actual error, hence retry.
            rpc_retry = true;
        }
    } while (rpc_retry);
}

#if 1
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
#endif

struct nfs_context* rpc_task::get_nfs_context() const
{
    return client->get_nfs_context();
}

void rpc_task::set_readdir(struct nfs_client* clt,
                           fuse_req* request,
                           fuse_ino_t ino,
                           size_t size,
                           off_t offset,
                           struct fuse_file_info* file)
{
    client = clt;
    req = request;
    optype = FUSE_READDIR;

    rpc_api.readdir_task.set_inode(ino);
    rpc_api.readdir_task.set_size(size);
    rpc_api.readdir_task.set_offset(offset);
    rpc_api.readdir_task.set_fuse_file(file);
    rpc_api.readdir_task.set_cookie(offset);
    rpc_api.readdir_task.set_cookieverf(0);
}

void rpc_task::set_readdirplus(struct nfs_client* clt,
                               fuse_req* request,
                               fuse_ino_t ino,
                               size_t size,
                               off_t offset,
                               struct fuse_file_info* file)
{
    client = clt;
    req = request;
    optype = FUSE_READDIRPLUS;
    rpc_api.readdirplus_task.set_inode(ino);
    rpc_api.readdirplus_task.set_size(size);
    rpc_api.readdirplus_task.set_offset(offset);
    rpc_api.readdirplus_task.set_fuse_file(file);
    // rpc_api.readdirplus_task.set_cookieverf(0);
    rpc_api.readdirplus_task.set_cookie(offset);
}

static void getattr_callback(
    struct rpc_context* /* rpc */,
    int rpc_status,
    void* data,
    void* private_data)
{
    auto task = (rpc_task*)private_data;
    auto res = (GETATTR3res*)data;
    bool retry;

    if (task->succeeded(rpc_status, RSTATUS(res), retry))
    {
        struct stat st;
        task->get_client()->stat_from_fattr3(
            &st, &res->GETATTR3res_u.resok.obj_attributes);

        // TODO: Set the Attr timeout to a better value.
        task->reply_attr(&st, 60/*getAttrTimeout()*/);
    }
    else if (retry)
    {
        task->run_getattr();
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
    void* data,
    void* private_data) {
    auto task = (rpc_task*)private_data;
    auto res = (LOOKUP3res*)data;
    bool retry;

    if (rpc_status == RPC_STATUS_SUCCESS && RSTATUS(res) == NFS3ERR_NOENT)
    {
        //
        // Special case for fuse: A "negative entry" refers to an entry that doesn't exist
        // in the file system. If we want negative cache, we must not return ENOENT,
        // instead we should return success with zero inode.
        // When the FUSE kernel module receives a negative entry response, it may cache this
        // information for a certain duration specified by the entry_timeout parameter.
        // This caching helps to improve performance by avoiding repeated lookup requests
        // for entries that are known not to exist.
        //
        struct fattr3 dummyAttr;
        ::memset(&dummyAttr, 0, sizeof(dummyAttr));

        task->get_client()->reply_entry(
            task,
            nullptr /* fh */,
            &dummyAttr,
            nullptr);
    }
    else if(task->succeeded(rpc_status, RSTATUS(res), retry))
    {
        assert(res->LOOKUP3res_u.resok.obj_attributes.attributes_follow);

        task->get_client()->reply_entry(
            task,
            &res->LOOKUP3res_u.resok.object,
            &res->LOOKUP3res_u.resok.obj_attributes.post_op_attr_u.attributes,
            nullptr);
    }
    else if(retry)
    {
        task->run_lookup();
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
    void* data,
    void* private_data)
{
    auto task = (rpc_task*)private_data;
    auto res = (CREATE3res*)data;
    bool retry;

    if (task->succeeded(rpc_status, RSTATUS(res), retry, false))
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
    else if (retry)
    {
        task->run_create_file();
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
    void* data,
    void* private_data)
{
    auto task = (rpc_task*)private_data;
    auto res = (SETATTR3res*)data;
    bool retry;

    if (task->succeeded(rpc_status, RSTATUS(res), retry))
    {
        assert(res->SETATTR3res_u.resok.obj_wcc.after.attributes_follow);

        struct stat st;
        task->get_client()->stat_from_fattr3(
            &st, &res->SETATTR3res_u.resok.obj_wcc.after.post_op_attr_u.attributes);
        task->reply_attr(&st, 60 /* TODO: Set reasonable value nfs_client::getAttrTimeout() */);
    }
    else if (retry)
    {
        task->run_setattr();
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
    void* data,
    void* private_data) {
    auto task = (rpc_task*)private_data;
    auto res = (MKDIR3res*)data;
    bool retry;

    if (task->succeeded(rpc_status, RSTATUS(res), retry, false))
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
    else if (retry)
    {
        task->run_mkdir();
    }
    else
    {
        // Since the api failed and can no longer be retried, return error reply.
        task->reply_error(-nfsstat3_to_errno(RSTATUS(res)));
    }
}

static void readdirplus_callback(
    struct rpc_context* /* rpc */,
    int rpc_status,
    void* data,
    void* private_data)
{
    auto task = (rpc_task*)private_data;
    auto res = (READDIRPLUS3res*)data;
    bool retry;

    if (task->succeeded(rpc_status, RSTATUS(res), retry))
    {
        // Allocate buffer
        int num_entries_returned_in_this_iter = 0;
        size_t sz = task->rpc_api.readdirplus_task.get_size();

        char *buf1 = (char *)malloc(sz);
        if (!buf1)
        {
            fuse_reply_err(task->get_req(), ENOMEM);
            return;
        }

        char *current_buff = buf1;
        size_t rem = sz;

        struct entryplus3* entry = res->READDIRPLUS3res_u.resok.reply.entries;

        while (entry)
        {
            if ((int)entry->cookie <= (int)task->rpc_api.readdirplus_task.get_offset())
            {
                /*
                 * Skip entries until the offset
                 * TODO: See if we need this. Can't we control this thorugh the cookie?
                 */
                entry = entry->nextentry;
                continue;
            }

            // Structure to hold the attributes.
            struct stat st;
            if (entry->name_attributes.attributes_follow)
            {
                task->get_client()->stat_from_fattr3(
                    &st, &entry->name_attributes.post_op_attr_u.attributes);
            }
            else
            {
                ::memset(&st, 0, sizeof(st));
            }

            // Create a new inode for the entry
            nfs_inode* nfs_ino;
            nfs_ino = new nfs_inode(&entry->name_handle.post_op_fh3_u.handle);
            nfs_ino->set_inode((fuse_ino_t)nfs_ino);

            struct fuse_entry_param fuseentry;
            memset(&fuseentry, 0, sizeof(fuseentry));
            fuseentry.attr = st;
            fuseentry.ino = (fuse_ino_t)(uintptr_t)nfs_ino;

            /*
             * TODO: Set the timeout to better value.
             */
            fuseentry.attr_timeout = 60;
            fuseentry.entry_timeout = 60;

            /*
             * Insert the entry into the buffer.
             * If the buffer space is less, fuse_add_direntry_plus will not add entry to
             * the buffer but will still return the space needed to add this entry.
             */
            size_t entsize = fuse_add_direntry_plus(task->get_req(),
                                                    current_buff,
                                                    rem, /* size left in the buffer */
                                                    entry->name,
                                                    &fuseentry,
                                                    entry->cookie);

            /*
             * Our buffer size was small and hence we can't add any more entries, so just break the loop.
             * This also means that we have not inserted the current entry to the direent buffer.
             */
            if (entsize > rem)
            {
                break;
            }

            num_entries_returned_in_this_iter++;

            // Increment the buffer pointer to point to the next free space.
            current_buff += entsize;
            rem -= entsize;

            // Fetch the next entry.
            entry = entry->nextentry;
        }

        fuse_reply_buf(task->get_req(),
                       buf1,
                       sz - rem);

        // Free the buffer.
        free(buf1);
    }
    else if (retry)
    {
        task->run_readdirplus();
    }
    else
    {
        // Since the api failed and can no longer be retried, return error reply.
        task->reply_error(-nfsstat3_to_errno(RSTATUS(res)));
    }
}

static void readdir_callback(
    struct rpc_context* /* rpc */,
    int rpc_status,
    void* data,
    void* private_data)
{
    auto task = (rpc_task*)private_data;
    auto res = (READDIR3res*)data;
    bool retry;

    if (task->succeeded(rpc_status, RSTATUS(res), retry))
    {
        int num_entries_returned_in_this_iter = 0;
        size_t sz = task->rpc_api.readdir_task.get_size();

        // Allocate buffer
        char *buf1 = (char *)malloc(sz);
        if (!buf1)
        {
            fuse_reply_err(task->get_req(), ENOMEM);
            return;
        }

        char *current_buf = buf1;
        size_t rem = sz;

        struct entry3* entry = res->READDIR3res_u.resok.reply.entries;

        while (entry)
        {
            if ((int)entry->cookie <= (int)task->rpc_api.readdir_task.get_offset())
            {
                /*
                 * Skip entries until the offset
                 * TODO: See if we need this. Can't we control this thorugh the cookie?
                 */
                entry = entry->nextentry;
                continue;
            }

            /*
             * Insert the entry into the buffer.
             * If the buffer space is less, fuse_add_direntry will not add entry to
             * the buffer but will still return the space needed to add this entry.
             */
            size_t entsize = fuse_add_direntry(task->get_req(),
                                               current_buf,
                                               rem, /* size left in the buffer */
                                               entry->name,
                                               nullptr,
                                               entry->cookie);

            /*
             * Our buffer size was small and hence we can't add any more entries, so just break the loop.
             * This also means that we have not inserted the current entry to the direent buffer.
             */
            if (entsize > rem)
            {
                break;
            }

            num_entries_returned_in_this_iter++;

            // Increment the buffer pointer to point to the next free space.
            current_buf += entsize;
            rem -= entsize;
            entry = entry->nextentry;
        }

        fuse_reply_buf(task->get_req(),
                       buf1,
                       sz - rem);

        free(buf1);
    }
    else if (retry)
    {
        task->run_readdir();
    }
    else
    {
        // Since the api failed and can no longer be retried, return error reply.
        task->reply_error(-nfsstat3_to_errno(RSTATUS(res)));
    }
}

void rpc_task::run_lookup()
{
    bool rpc_retry = false;
    auto parent_ino = rpc_api.lookup_task.get_parent_inode();

    do {
        LOOKUP3args args;
        ::memset(&args, 0, sizeof(args));
        args.what.dir = get_client()->get_nfs_inode_from_ino(parent_ino)->get_fh();
        args.what.name = (char*)rpc_api.lookup_task.get_name();

        if (rpc_nfs3_lookup_task(get_rpc_ctx(), lookup_callback, &args, this) == NULL)
        {
            // This call fails due to internal issues like OOM etc
            // and not due to an actual error, hence retry.
            rpc_retry = true;
        }
    } while (rpc_retry);
}

void rpc_task::run_getattr()
{
    bool rpc_retry = false;
    auto inode = rpc_api.getattr_task.get_inode();

    do {
        struct GETATTR3args args;
        ::memset(&args, 0, sizeof(args));
        args.object = get_client()->get_nfs_inode_from_ino(inode)->get_fh();

        if (rpc_nfs3_getattr_task(get_rpc_ctx(), getattr_callback, &args, this) == NULL)
        {
            // This call fails due to internal issues like OOM etc
            // and not due to an actual error, hence retry.
            rpc_retry = true;
        }
    } while (rpc_retry);
}

void rpc_task::run_create_file()
{
    bool rpc_retry = false;
    auto parent_ino = rpc_api.create_task.get_parent_inode();

    do {
        CREATE3args args;
        ::memset(&args, 0, sizeof(args));
        args.where.dir = get_client()->get_nfs_inode_from_ino(parent_ino)->get_fh();
        args.where.name = (char*)rpc_api.create_task.get_name();
        args.how.mode = (rpc_api.create_task.get_file()->flags & O_EXCL) ? GUARDED : UNCHECKED;
        args.how.createhow3_u.obj_attributes.mode.set_it = 1;
        args.how.createhow3_u.obj_attributes.mode.set_mode3_u.mode = rpc_api.create_task.get_mode();

        if (rpc_nfs3_create_task(get_rpc_ctx(), createfile_callback, &args, this) == NULL)
        {
            // This call fails due to internal issues like OOM etc
            // and not due to an actual error, hence retry.
            rpc_retry = true;
        }
    }  while (rpc_retry);
}

void rpc_task::run_mkdir()
{
    bool rpc_retry = false;
    auto parent_ino = rpc_api.mkdir_task.get_parent_inode();

    do {
        MKDIR3args args;
        ::memset(&args, 0, sizeof(args));
        args.where.dir = get_client()->get_nfs_inode_from_ino(parent_ino)->get_fh();
        args.where.name = (char*)rpc_api.mkdir_task.get_name();
        args.attributes.mode.set_it = 1;
        args.attributes.mode.set_mode3_u.mode = rpc_api.mkdir_task.get_mode();


        if (rpc_nfs3_mkdir_task(get_rpc_ctx(), mkdir_callback, &args, this) == NULL)
        {
            // This call fails due to internal issues like OOM etc
            // and not due to an actual error, hence retry.
            rpc_retry = true;
        }
    } while (rpc_retry);
}

void rpc_task::run_setattr()
{
    auto inode = rpc_api.setattr_task.get_inode();
    auto attr = rpc_api.setattr_task.get_attr();
    const int valid = rpc_api.setattr_task.get_attr_flags_to_set();
    bool rpc_retry = false;

    do {
        SETATTR3args args;
        ::memset(&args, 0, sizeof(args));
        args.object = get_client()->get_nfs_inode_from_ino(inode)->get_fh();

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
            AZLogInfo("Setting atime to now");
            args.new_attributes.atime.set_it = SET_TO_SERVER_TIME;
        }

        if (valid & FUSE_SET_ATTR_MTIME_NOW) {
            AZLogInfo("Setting mtime to now");
            args.new_attributes.mtime.set_it = SET_TO_SERVER_TIME;
        }

        if (rpc_nfs3_setattr_task(get_rpc_ctx(), setattr_callback, &args, this) == NULL)
        {
            // This call fails due to internal issues like OOM etc
            // and not due to an actual error, hence retry.
            rpc_retry = true;
        }
    } while (rpc_retry);
}

void rpc_task::run_readdir()
{

    bool rpc_retry = false;
    auto inode = rpc_api.readdir_task.get_inode();

    do {
        struct READDIR3args args;
        ::memset(&args, 0, sizeof(args));
        args.dir = get_client()->get_nfs_inode_from_ino(inode)->get_fh();
        args.cookie = rpc_api.readdir_task.get_cookie();
        ::memcpy(&args.cookieverf, rpc_api.readdir_task.get_cookieverf(), sizeof(args.cookieverf));
        args.count = rpc_api.readdir_task.get_size();

        if (rpc_nfs3_readdir_task(get_rpc_ctx(), readdir_callback, &args, this) == NULL)
        {
            // This call fails due to internal issues like OOM etc
            // and not due to an actual error, hence retry.
            rpc_retry = true;
        }
    } while (rpc_retry);
}

void rpc_task::run_readdirplus()
{
    bool rpc_retry = false;
    auto inode = rpc_api.readdirplus_task.get_inode();

    do {
        struct READDIRPLUS3args args;
        ::memset(&args, 0, sizeof(args));
        args.dir = get_client()->get_nfs_inode_from_ino(inode)->get_fh();
        args.cookie = rpc_api.readdirplus_task.get_cookie();
        ::memcpy(&args.cookieverf, rpc_api.readdirplus_task.get_cookieverf(), sizeof(args.cookieverf));
        args.dircount = 65536; // TODO: See what this value should be set to.
        args.maxcount = 65536;

        if (rpc_nfs3_readdirplus_task(get_rpc_ctx(), readdirplus_callback, &args, this) == NULL)
        {
            // This call fails due to internal issues like OOM etc
            // and not due to an actual error, hence retry.
            rpc_retry = true;
        }
    } while (rpc_retry);
}
