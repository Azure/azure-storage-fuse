#pragma once
#include "nfs_client.h"
#include "fuse_optype.h"

class NfsApiContext
{
private:
    // The client for which the context is created.
    NfsClient* client;

    // Fuse request structure.
    // This will be the request structure passed from the fuse layer.
    fuse_req* req;

    // Max number of times the NFS APIs can be retried.
    static int maxErrnoRetries;

    int numOfTimesRetried;

protected:
    // Operation type. This is used only for logging.
    enum fuse_optype optype;

public:
    NfsApiContext(NfsClient* clnt, struct fuse_req* freq, enum fuse_optype opType):
        client(clnt),
        req(freq),
        numOfTimesRetried(0),
        optype(opType)
    {}

    virtual ~NfsApiContext() {};

    static void setMaxErrnoRetries(int maxRetries)
    {
        maxErrnoRetries = maxRetries;
    }

    static int getMaxErrnoRetries()
    {
        return maxErrnoRetries;
    }

    struct nfs_context* GetNfsContext() const
    {
        return client->GetNfsContext();
    }

    struct rpc_context* GetRpcCtx() const
    {
        return nfs_get_rpc_context(GetNfsContext());
    }

    NfsClient* getClient() const
    {
        return client;
    }

    // This method will reply with error and delete the context object.
    void replyError(int rc)
    {
        fuse_reply_err(req, rc);
        delete this;
    }

    void replyAttr(const struct stat* attr, double attr_timeout)
    {
        fuse_reply_attr(req, attr, attr_timeout);
        delete this;
    }

    void replyWrite(size_t count)
    {
        fuse_reply_write(req, count);
        delete this;
    }

    void replyEntry(const struct fuse_entry_param* e)
    {
        fuse_reply_entry(req, e);
        delete this;
    }

    void replyCreate(
        const struct fuse_entry_param* entry,
        const struct fuse_file_info* file)
    {
        fuse_reply_create(req, entry, file);
        delete this;
    }

    //
    // Check RPC completion for success.
    //
    // On success, true is returned.
    // On failure, false is returned and \p retry is set to true if the error is retryable else set to false.
    //
    bool succeeded(
        int rpc_status,
        int nfs_status,
        bool& retry,
        bool idempotent = true)
    {
        retry = false;

        if (rpc_status != RPC_STATUS_SUCCESS && (numOfTimesRetried < getMaxErrnoRetries()))
        {
            retry = true;
            return false;
        }

        if (nfs_status != NFS3_OK)
        {
            if (idempotent && (numOfTimesRetried < getMaxErrnoRetries()) && isRetryableError(nfs_status))
            {
                numOfTimesRetried++;
                retry = true;
                return false;
            }

            return false;
        }

        return true; // success.
    }

    bool isRetry() const
    {
        return numOfTimesRetried > 0;
    }

    bool isRetryableError(int nfs_status)
    {
        switch (nfs_status)
        {
        case NFS3ERR_IO:
        case NFS3ERR_SERVERFAULT:
        case NFS3ERR_ROFS:
        case NFS3ERR_PERM:
            return true;
        default:
            return false;
        }
    }

    struct fuse_req* getReq() const
    {
        return req;
    }
};

//
// This can be used for Nfsv3 APIs that take inode as a parameter.
//
class NfsApiContextInode:  public NfsApiContext
{
public:
    NfsApiContextInode(
        NfsClient* client,
        struct fuse_req* req,
        enum fuse_optype optype,
        fuse_ino_t ino)
        : NfsApiContext(client, req, optype) {
        inode = ino;
    }

    fuse_ino_t getInode() const
    {
        return inode;
    }

private:
    fuse_ino_t inode;
};

// This can be used for Nfsv3 APIs that takes file name and parent inode as a parameter.
class NfsApiContextParentName : public NfsApiContext {
public:
    NfsApiContextParentName(
        NfsClient* client,
        struct fuse_req* req,
        enum fuse_optype optype,
        fuse_ino_t parent,
        const char* name)
        : NfsApiContext(client, req, optype),
          parentIno(parent)
    {
        fileName = ::strdup(name);
    }

    ~NfsApiContextParentName()
    {
        ::free((void*)fileName);
    }

    fuse_ino_t getParent() const
    {
        return parentIno;
    }

    const char* getName() const
    {
        return fileName;
    }

private:
    fuse_ino_t parentIno;
    const char* fileName;
};

// This is the context that will be used by the Nfsv3 Create API.
class NfsCreateApiContext : public NfsApiContextParentName
{
public:
    NfsCreateApiContext(
        NfsClient* client,
        struct fuse_req* req,
        enum fuse_optype optype,
        fuse_ino_t parent,
        const char* name,
        mode_t createmode,
        struct fuse_file_info* fileinfo)
        : NfsApiContextParentName(client, req, optype, parent, name)
    {
        mode = createmode;

        if (fileinfo) {
            ::memcpy(&file, fileinfo, sizeof(file));
            filePtr = &file;
        } else {
            filePtr = nullptr;
        }
    }

    mode_t getMode() const
    {
        return mode;
    }

    struct fuse_file_info* getFile() const
    {
        return filePtr;
    }

private:
    mode_t mode;

    struct fuse_file_info file;
    struct fuse_file_info* filePtr;
};

// This can be used for Nfsv3 APIs that takes file and inode as parameters.
class NfsApiContextInodeFile : public NfsApiContextInode
{
public:
    NfsApiContextInodeFile(
        NfsClient* client,
        struct fuse_req* req,
        enum fuse_optype optype,
        fuse_ino_t inode,
        fuse_file_info* fileinfo)
        : NfsApiContextInode(client, req, optype, inode) {
        if (fileinfo) {
            ::memcpy(&file, fileinfo, sizeof(file));
            filePtr = &file;
        } else {
            filePtr = nullptr;
        }
    }

    fuse_file_info* getFile() const
    {
        return filePtr;
    }

private:
    fuse_file_info file;
    fuse_file_info* filePtr;
};

// This is the context that will be used by the Nfsv3 Setattr API
class NfsSetattrApiContext: public NfsApiContextInodeFile {
public:
    NfsSetattrApiContext(
        NfsClient* client,
        struct fuse_req* req,
        enum fuse_optype optype,
        fuse_ino_t inode,
        struct stat* attribute,
        int toSet,
        struct fuse_file_info* file)
        : NfsApiContextInodeFile(client, req, optype, inode, file) {
        attr = attribute;
        to_set = toSet;
    }

    struct stat* getAttr() const
    {
        return attr;
    }

    int getAttrFlagsToSet() const
    {
        return to_set;
    }

private:
    struct stat* attr;
    // Valid attribute mask to be set.
    int to_set;
};

int NfsApiContext::maxErrnoRetries(3);
