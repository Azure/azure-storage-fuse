#include "nfs_internal.h"
#include "rpc_task.h"

#define RSTATUS(r) ((r) ? (r)->status : NFS3ERR_SERVERFAULT)

int rpc_task::max_errno_retries(3);

void rpc_task::set_lookup(struct nfs_client* clt,
                          fuse_req* request,
                          const char* name,
                          fuse_ino_t parent_ino)
{
    client = clt;
    req = request;
    optype = FOPTYPE_LOOKUP;
    rpc_api.lookup_task.set_file_name(name);
    rpc_api.lookup_task.set_parent_inode(parent_ino);
}

void rpc_task::set_getattr(struct nfs_client* clt,
                           fuse_req* request,
                           fuse_ino_t ino)
{
    client = clt;
    req = request;
    optype = FOPTYPE_GETATTR;
    rpc_api.getattr_task.set_inode(ino);
}

void rpc_task::set_create_file(struct nfs_client* clt,
                               fuse_req* request,
                               fuse_ino_t parent_ino,
                               const char* name,
                               mode_t mode,
                               struct fuse_file_info* file)
{
    client = clt;
    req = request;
    optype = FOPTYPE_CREATE;
    rpc_api.create_task.set_parent_inode(parent_ino);
    rpc_api.create_task.set_file_name(name);
    rpc_api.create_task.set_mode(mode);
    rpc_api.create_task.set_fuse_file(file);
}

void rpc_task::set_mkdir(struct nfs_client* clt,
                         fuse_req* request,
                         fuse_ino_t parent_ino,
                         const char* name,
                         mode_t mode)
{
    client = clt;
    req = request;
    optype = FOPTYPE_MKDIR;
    rpc_api.mkdir_task.set_parent_inode(parent_ino);
    rpc_api.mkdir_task.set_dir_name(name);
    rpc_api.mkdir_task.set_mode(mode);

}

void rpc_task::set_setattr(struct nfs_client* clt,
                           fuse_req* request,
                           fuse_ino_t ino,
                           struct stat* attr,
                           int toSet,
                           struct fuse_file_info* file)
{
    client = clt;
    req = request;
    optype = FOPTYPE_SETATTR;
    rpc_api.setattr_task.set_inode(ino);
    rpc_api.setattr_task.set_fuse_file(file);
    rpc_api.setattr_task.set_attribute_and_mask(attr, toSet);
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

// This is the task responsible for making the lookup task.
// lookup_task structure should be populated before calling this function.
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

void rpc_task::free_rpc_task()
{
    // Clean the mebers since we could not have the destructor.
    switch(optype)
    {
    case fuse_optype::FOPTYPE_LOOKUP:
        rpc_api.lookup_task.free_name();
        break;
    case fuse_optype::FOPTYPE_CREATE:
        rpc_api.create_task.free_name();
        break;
    case fuse_optype::FOPTYPE_MKDIR:
        rpc_api.mkdir_task.free_name();
        break;
    default :
        break;
    }
    client->get_rpc_task_helper_instance()->free_rpc_task_instance(this);
}

struct nfs_context* rpc_task::get_nfs_context() const
{
    return client->get_nfs_context();
}
