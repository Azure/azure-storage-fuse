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
// The user should first init the class by calling nfs_client::init() by specifying all the parameters needed to mount the filesystem.
// Once this is done, all the callers can get an instance of this class by calling the get_instance() method.
// This instance can then be used to call the APIs like getattr, write etc.
//
class nfs_client
{
private:

    //
    // This will be of the form account.blob.core.windows.net
    // or for pre-prod : account.blob.preprod.core.windows.net
    // or 			   : IP
    // This will be constructed from the account_name and blobprefix passed by the caller.
    //
    static std::string server;

    // TODO: See if we really need to store the account and container name since we already have the server and export_path.
    std::string account_name;
    std::string container_name;

    // Export path which is of the form /account_name/ContainerName
    static std::string export_path;

    //
    // Options to be passed to the mount command. Like port, proto etc.
    //
    struct mount_options* mnt_options;

    //
    // File handle obtained after mounting the filesystem.
    // This will be set after calling nfs_mount which is done in the init() method.
    //
    static nfs_file_handle* root_fh;

    //
    // The transport object responsible for actually sending out the requests to the server.
    //
    static struct rpc_transport* transport;

    static struct rpc_task_helper* rpc_task_helper_instance;

    // Holds info about the server.
    struct NfsServerInfo* server_info;

    // Contains info of the server stat.
    struct NfsServerStat* server_stat;

    //
    // This will be set to true if the nfs_client is init'd.
    // This should be set to TRUE before calling the nfs_client::get_instance()
    //
    static std::atomic<bool> initialized;

    nfs_client(std::string* acc_name, std::string* cont_name, std::string* blob_suffix, struct mount_options* opt)
    {
        assert(acc_name != nullptr);
        assert(cont_name != nullptr);
        assert(blob_suffix != nullptr);

        account_name = *acc_name;
        container_name = *cont_name;

        server = account_name + "." + *blob_suffix;
        export_path = "/" + account_name + "/" + container_name;

        mnt_options = opt;
    }

public:
    static nfs_client& get_instance_impl(std::string* acc_name = nullptr,
                                         std::string* cont_name = nullptr,
                                         std::string* blob_suffix = nullptr,
                                         struct mount_options* opt = nullptr)
    {
        static nfs_client instance{acc_name, cont_name, blob_suffix, opt};

        // nfs_client::init() should be called before calling this.
        assert(is_nfs_client_initd());
        return instance;
    }

    static struct rpc_task_helper* get_rpc_task_helper_instance()
    {
        return rpc_task_helper_instance;
    }

    // This is the method which should be called to get an instance of this class by the user.
    static nfs_client& get_instance()
    {
        return get_instance_impl();
    }

    // The user should first init the client class before using it.
    static bool init(
        std::string& acc_name,
        std::string& cont_name,
        std::string& blob_suffix,
        mount_options* opt);

    static bool is_nfs_client_initd()
    {
        return initialized;
    }

    //
    // Get the nfs context on which the libnfs API calls can be made.
    //
    struct nfs_context* get_nfs_context() const;

    //
    // The inode structure will be the address of the location where the actual NFS filehandle is stored.
    // Hence by just dereferencing this structure we will be able to get the filehandle.
    // This filehandle will remain valid till the ino is freeed by calling the free API.
    // TODO: See when the free API should be called.
    //
    nfs_file_handle* get_fh_from_inode(fuse_ino_t ino)
    {
        if (ino == 1 /*FUSE_ROOT_ID*/)
        {
            return root_fh;
        }
        return (nfs_file_handle*)(uintptr_t)ino;

    }

    //
    // Define Nfsv3 APi specific functions and helpers after this point.
    // TODO: For now I have just added the methods needed for few calls, add more going forward.
    //

    void getattr(
        fuse_req_t req,
        fuse_ino_t inode,
        struct fuse_file_info* file);

    void create(
        fuse_req_t req,
        fuse_ino_t parent,
        const char* name,
        mode_t mode,
        struct fuse_file_info* file);

    void mkdir(
        fuse_req_t req,
        fuse_ino_t parent,
        const char* name,
        mode_t mode);

    void setattr(
        fuse_req_t req,
        fuse_ino_t inode,
        struct stat* attr,
        int toSet,
        struct fuse_file_info* file);

    void lookup(
        fuse_req_t req,
        fuse_ino_t parent,
        const char* name);

    void readdir(
        fuse_req_t req,
        fuse_ino_t /* inode */,
        size_t size,
        off_t off,
        struct fuse_file_info* file);

    static void stat_from_fattr3(struct stat* st, const struct fattr3* attr);

    void reply_entry(
        struct rpc_task* ctx,
        const nfs_fh3* fh,
        const struct fattr3* attr,
        const struct fuse_file_info* file);
};
