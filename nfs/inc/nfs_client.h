#pragma once

#include "nfs_fileHandle.h"
#include "nfs_transport.h"
#include "nfs_internal.h"

extern "C" {
    // libnfs does not offer a prototype for this in any public header,
    // but exports it anyway.
    const struct nfs_fh3* nfs_get_rootfh(struct nfs_context* nfs);
}

//
// This is the class which is responsible for making all the Nfsv3 API calls.
// This is a singleton class.
// The user should first init the class by calling NfsClient::Init() by specifying all the parameters needed to mount the filesystem.
// Once this is done, all the callers can get an instance of this class by calling the GetInstance() method.
// This instance can then be used to call the APIs like getattr, write etc.
//
class NfsClient
{
private:

    //
    // This will be of the form account.blob.core.windows.net
    // or for pre-prod : account.blob.preprod.core.windows.net
    // or 			   : IP
    // This will be constructed from the accountName and blobprefix passed by the caller.
    //
    static std::string server;

    // TODO: See if we really need to store the account and container name since we already have the server and exportPath.
    std::string accountName;
    std::string containerName;

    // Export path which is of the form /accountName/ContainerName
    static std::string exportPath;

    //
    // Options to be passed to the mount command. Like port, proto etc.
    //
    struct mountOptions* mntOptions;

    //
    // File handle obtained after mounting the filesystem.
    // This will be set after calling nfs_mount which is done in the Init() method.
    //
    static NFSFileHandle* rootFh;

    //
    // The transport object responsible for actually sending out the requests to the server.
    //
    static RPCTransport* transport;

    // Holds info about the server.
    struct NfsServerInfo* serverInfo;

    // Contains info of the server stat.
    struct NfsServerStat* serverStat;

    //
    // This will be set to true if the NfsClient is init'd.
    // This should be set to TRUE before calling the NfsClient::GetInstance()
    //
    static std::atomic<bool> initialized;

    NfsClient(std::string* acctName, std::string* contName, std::string* blobSuffix, struct mountOptions* opt)
    {
        assert(acctName != nullptr);
        assert(contName != nullptr);
        assert(blobSuffix != nullptr);

        accountName = *acctName;
        containerName = *contName;

        server = accountName + "." + *blobSuffix;
        exportPath = "/" + accountName + "/" + containerName;

        mntOptions = opt;
    }

public:
    static NfsClient& GetInstanceImpl(std::string* acctName = nullptr,
                                      std::string* contName = nullptr,
                                      std::string* blobSuffix = nullptr,
                                      struct mountOptions* opt = nullptr)
    {
        static NfsClient instance{acctName, contName, blobSuffix, opt};

        // NfsClient::Init() should be called before calling this.
        assert(IsNfsClientInitd());
        return instance;
    }

    // This is the method which should be called to get an instance of this class by the user.
    static NfsClient& GetInstance()
    {
        return GetInstanceImpl();
    }

    // The user should first init the client class before using it.
    static bool Init(
        std::string& acctName,
        std::string& contName,
        std::string& blobSuffix,
        mountOptions* opt);

    static bool IsNfsClientInitd()
    {
        return initialized;
    }

    //
    // Get the nfs context on which the libnfs API calls can be made.
    //
    struct nfs_context* GetNfsContext() const
    {
        return transport->GetNfsContext();
    }

    //
    // The inode structure will be the address of the location where the actual NFS filehandle is stored.
    // Hence by just dereferencing this structure we will be able to get the filehandle.
    // This filehandle will remain valid till the ino is freeed by calling the free API.
    // TODO: See when the free API should be called.
    //
    NFSFileHandle* GetFhFromInode(fuse_ino_t ino)
    {
        if (ino == 1 /*FUSE_ROOT_ID*/)
        {
            return rootFh;
        }
        return (NFSFileHandle*)(uintptr_t)ino;

    }

    //
    // Define Nfsv3 APi specific functions and helpers after this point.
    // TODO: For now I have just added the methods needed for few calls, add more going forward.
    //

    void getattrWithContext(struct NfsApiContextInode* ctx);

    void getattr(
        fuse_req_t req,
        fuse_ino_t inode,
        struct fuse_file_info* file);

    void createFileWithContext(struct NfsCreateApiContext* ctx);

    void create(
        fuse_req_t req,
        fuse_ino_t parent,
        const char* name,
        mode_t mode,
        struct fuse_file_info* file);

    void mkdirWithContext(struct NfsMkdirApiContext* ctx);

    void mkdir(
        fuse_req_t req,
        fuse_ino_t parent,
        const char* name,
        mode_t mode);

    void setattrWithContext(struct NfsSetattrApiContext* ctx);

    void setattr(
        fuse_req_t req,
        fuse_ino_t inode,
        struct stat* attr,
        int toSet,
        struct fuse_file_info* file);

    void lookupWithContext(struct NfsApiContextParentName* ctx);

    void lookup(
        fuse_req_t req,
        fuse_ino_t parent,
        const char* name);

    static void stat_from_fattr3(struct stat* st, const struct fattr3* attr);

    void replyEntry(
        struct NfsApiContext* ctx,
        const nfs_fh3* fh,
        const struct fattr3* attr,
        const struct fuse_file_info* file);
};
