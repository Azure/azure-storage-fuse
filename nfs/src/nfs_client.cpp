#include "nfs_client.h"
#include "nfs_internal.h"
#include "nfs_api_context.h"

std::atomic<bool> NfsClient::initialized(false);
std::string NfsClient::server("");
std::string NfsClient::exportPath("");
RPCTransport* NfsClient::transport;
NFSFileHandle* NfsClient::rootFh;

#define RSTATUS(r) ((r) ? (r)->status : NFS3ERR_SERVERFAULT)

// The user should first init the client class before using it.
bool NfsClient::Init(
    std::string& acctName,
    std::string& contName,
    std::string& blobSuffix,
    struct mountOptions* opt)
{
    // Check if init() has been called before
    if (!initialized.exchange(true)) {

        GetInstanceImpl(&acctName, &contName, &blobSuffix, opt);

        // Get the RPC transport to be used for this client.
        transport = RPCTransport::GetInstance(opt);

        // This will init the transport layer and start the connections to the server.
        // It returns FALSE if it fails to create the connections.
        if (!transport->start())
        {
            AZLogError("Failed to start the RPC transport.");
            return false;
        }

        // Initialiaze the root file handle.
	// TODO: Take care of freeing this. Should this be freed in the ~NfsClient()?
        rootFh = new NFSFileHandle(nfs_get_rootfh(transport->GetNfsContext()) /*, 1  ino will be 1 for root */);
        rootFh->SetInode(1);
        //AZLogInfo("Obtained root fh is {}", rootFh->GetFh());

        return true;
    }

    // Return false if the method is called again.
    return false;
}

/*
 *  The methods below are used to implement the specific nfsv3 APIs.
 */
static void getattrCallback(
    struct rpc_context* /* rpc */,
    int rpc_status,
    void* data,
    void* privateData)
{
    auto ctx = (NfsApiContextInode*)privateData;
    auto res = (GETATTR3res*)data;
    bool retry;

    if (ctx->succeeded(rpc_status, RSTATUS(res), retry))
    {
        struct stat st;
        ctx->getClient()->stat_from_fattr3(
            &st, &res->GETATTR3res_u.resok.obj_attributes);

        // TODO: Set the Attr timeout to a better value.
        ctx->replyAttr(&st, 60/*getAttrTimeout()*/);
    }
    else if (retry)
    {
        ctx->getClient()->getattrWithContext(ctx);
    }
    else
    {
        // Since the api failed and can no longer be retried, return error reply.
        ctx->replyError(-nfsstat3_to_errno(RSTATUS(res)));
    }
}

void NfsClient::getattrWithContext(NfsApiContextInode* ctx) {
    bool rpcRetry = false;
    auto inode = ctx->getInode();
   
    do {
        struct GETATTR3args args;
        ::memset(&args, 0, sizeof(args));
        args.object = GetFhFromInode(inode)->GetFh();

        if (rpc_nfs3_getattr_task(ctx->GetRpcCtx(), getattrCallback, &args, ctx) == NULL)
        {
            // This call fails due to internal issues like OOM etc
            // and not due to an actual error, hence retry.
            rpcRetry = true;
        }
    } while (rpcRetry);
}

void NfsClient::getattr(
    fuse_req_t req,
    fuse_ino_t inode,
    struct fuse_file_info* file)
{
    //
    // The context created here will be freed at the time response is sent to the client.
    // In this case it will be freed in replyError() or replyAttr().
    //
    auto ctx = new NfsApiContextInode(this, req, FOPTYPE_GETATTR, inode);
    getattrWithContext(ctx);
}

static void lookupCallback(
    struct rpc_context* /* rpc */,
    int rpc_status,
    void* data,
    void* private_data) {
    auto ctx = (NfsApiContextParentName*)private_data;
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

        ctx->getClient()->replyEntry(
            ctx,
            nullptr /* fh */,
            &dummyAttr,
	    nullptr);
    }
    else if(ctx->succeeded(rpc_status, RSTATUS(res), retry))
    {
        assert(res->LOOKUP3res_u.resok.obj_attributes.attributes_follow);

        ctx->getClient()->replyEntry(
	    ctx,
            &res->LOOKUP3res_u.resok.object,
            &res->LOOKUP3res_u.resok.obj_attributes.post_op_attr_u.attributes,
            nullptr);
    }
    else if(retry)
    {
        ctx->getClient()->lookupWithContext(ctx);
    }
    else
    {
        // Since the api failed and can no longer be retried, return error reply.
        ctx->replyError(-nfsstat3_to_errno(RSTATUS(res)));
    }
}

void NfsClient::lookupWithContext(NfsApiContextParentName* ctx)
{
    bool rpcRetry = false;
    auto parent = ctx->getParent();

    do {
        LOOKUP3args args;
        ::memset(&args, 0, sizeof(args));
        args.what.dir = GetFhFromInode(parent)->GetFh();
        args.what.name = (char*)ctx->getName();

        if (rpc_nfs3_lookup_task(ctx->GetRpcCtx(), lookupCallback, &args, ctx) == NULL)
        {
            // This call fails due to internal issues like OOM etc
            // and not due to an actual error, hence retry.
            rpcRetry = true;
        }
    } while (rpcRetry);
}

void NfsClient::lookup(fuse_req_t req, fuse_ino_t parent, const char* name)
{
    //
    // The context created here will be freed at the time response is sent to the client.
    // In this case it will be freed in replyError() or replyEntry().
    //
    auto ctx = new NfsApiContextParentName(this, req, FOPTYPE_LOOKUP, parent, name);
    lookupWithContext(ctx);
}

static void createFileCallback(
    struct rpc_context* /* rpc */,
    int rpc_status,
    void* data,
    void* private_data)
{
    auto ctx = (NfsCreateApiContext*)private_data;
    auto res = (CREATE3res*)data;
    bool retry;

    if (ctx->succeeded(rpc_status, RSTATUS(res), retry, false))
    {
        assert(
            res->CREATE3res_u.resok.obj.handle_follows &&
            res->CREATE3res_u.resok.obj_attributes.attributes_follow);

	ctx->getClient()->replyEntry(
	    ctx,
            &res->CREATE3res_u.resok.obj.post_op_fh3_u.handle,
            &res->CREATE3res_u.resok.obj_attributes.post_op_attr_u.attributes,
            ctx->getFile());
    }
    else if (retry)
    {
        ctx->getClient()->createFileWithContext(ctx);
    }
    else
    {
	// Since the api failed and can no longer be retried, return error reply.
	ctx->replyError(-nfsstat3_to_errno(RSTATUS(res)));
    }

}

void NfsClient::createFileWithContext(NfsCreateApiContext* ctx)
{
    bool rpcRetry = false;
    auto parent = ctx->getParent();

    do {
        CREATE3args args;
        ::memset(&args, 0, sizeof(args));
        args.where.dir = GetFhFromInode(parent)->GetFh();
        args.where.name = (char*)ctx->getName();
        args.how.mode = (ctx->getFile()->flags & O_EXCL) ? GUARDED : UNCHECKED;
        args.how.createhow3_u.obj_attributes.mode.set_it = 1;
        args.how.createhow3_u.obj_attributes.mode.set_mode3_u.mode = ctx->getMode();

        if (rpc_nfs3_create_task(ctx->GetRpcCtx(), createFileCallback, &args, ctx) == NULL)
        {
            // This call fails due to internal issues like OOM etc
            // and not due to an actual error, hence retry.
            rpcRetry = true;
        }
    }  while (rpcRetry);
}

void NfsClient::create(
    fuse_req_t req,
    fuse_ino_t parent,
    const char* name,
    mode_t mode,
    struct fuse_file_info* file)
{
    //
    // The context created here will be freed at the time response is sent to the client.
    // In this case it will be freed in replyError() or replyEntry().
    //
    auto ctx = new NfsCreateApiContext(this, req, FOPTYPE_CREATE, parent, name, mode, file);
    createFileWithContext(ctx);
}

static void setattrCallback(
    struct rpc_context* /* rpc */,
    int rpc_status,
    void* data,
    void* private_data)
{
    auto ctx = (NfsSetattrApiContext*)private_data;
    auto res = (SETATTR3res*)data;
    bool retry;

    if (ctx->succeeded(rpc_status, RSTATUS(res), retry))
    {
        assert(res->SETATTR3res_u.resok.obj_wcc.after.attributes_follow);

	struct stat st;
        ctx->getClient()->stat_from_fattr3(
            &st, &res->SETATTR3res_u.resok.obj_wcc.after.post_op_attr_u.attributes);
        ctx->replyAttr(&st, 60 /* TODO: Set reasonable value NfsClient::getAttrTimeout() */);
    }
    else if (retry)
    {
        ctx->getClient()->setattrWithContext(ctx);
    }
    else
    {
        // Since the api failed and can no longer be retried, return error reply.
        ctx->replyError(-nfsstat3_to_errno(RSTATUS(res)));
    }
}

void NfsClient::setattrWithContext(NfsSetattrApiContext* ctx)
{
    auto inode = ctx->getInode();
    auto attr = ctx->getAttr();
    const int valid = ctx->getAttrFlagsToSet();
    bool rpcRetry = false;

    do {
        SETATTR3args args;
        ::memset(&args, 0, sizeof(args));
        args.object = GetFhFromInode(inode)->GetFh();

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

        if (rpc_nfs3_setattr_task(ctx->GetRpcCtx(), setattrCallback, &args, ctx) == NULL)
        {
            // This call fails due to internal issues like OOM etc
            // and not due to an actual error, hence retry.
            rpcRetry = true;
        }
    } while (rpcRetry);
}

void NfsClient::setattr(
    fuse_req_t req,
    fuse_ino_t inode,
    struct stat* attr,
    int toSet,
    struct fuse_file_info* file)
{
    //
    // The context created here will be freed at the time response is sent to the client.
    // In this case it will be freed in replyError() or replyAttr().
    //
    auto ctx = new NfsSetattrApiContext(
        this, req, FOPTYPE_SETATTR, inode, attr, toSet, file);
    setattrWithContext(ctx);
}

//
// Creates a new inode for the given fh and passes it to fuse_reply_entry().
// This will be called by the APIs which much return a filehandle back to the client
// like lookup, create etc.
//
void NfsClient::replyEntry(
    NfsApiContext* ctx,
    const nfs_fh3* fh,
    const struct fattr3* attr,
    const struct fuse_file_info* file)
{
    NFSFileHandle* filehandle;
    
    if (fh)
    {
	// TODO: When should this be freed? This should be freed when the ino is freed,
	// 	 but decide when should that be done?
        filehandle = new NFSFileHandle(fh);
        filehandle->SetInode((fuse_ino_t)filehandle);
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
        ctx->replyCreate(&entry, file);
    }
    else
    {
        ctx->replyEntry(&entry);
    }
}

// Translate a NFS fattr3 into struct stat.
void NfsClient::stat_from_fattr3(struct stat* st, const struct fattr3* attr)
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
