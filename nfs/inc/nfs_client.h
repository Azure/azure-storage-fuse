#ifndef __NFS_CLIENT_H__
#define __NFS_CLIENT_H__

#include "nfs_inode.h"
#include "rpc_transport.h"
#include "nfs_internal.h"

extern "C" {
    /*
     * libnfs does not offer a prototype for this in any public header,
     * but exports it anyway.
     *
     * TODO: Update libnfs to export this and remove from here.
     */
    const struct nfs_fh3* nfs_get_rootfh(struct nfs_context* nfs);
}

/**
 * This represents the NFS client. Since we have only one NFS client at a time,
 * this is a singleton class.
 * Caller can make NFSv3 API calls by calling corresponding methods from this
 * class. Those methods will then call into libnfs to make the actual NFS RPC
 * User should first init the class by calling init() by specifying all the
 * parameters needed to mount the filesystem.
 * Once initialized, callers can get the singleton instance of this class by
 * calling the get_instance() static method.
 * The returned instance can then be used to call the APIs like getattr, write etc.
 */
#define NFS_CLIENT_MAGIC *((const uint32_t *)"NFSC")

struct nfs_client
{
    const uint32_t magic = NFS_CLIENT_MAGIC;
private:
    /*
     * This is the RPC transport connected to the NFS server.
     * RPC transport is made up of one or more nfs_connection which are used
     * to carry the RPC requests/responses.
     */
    struct rpc_transport transport;

    /*
     * Root File Handle obtained after mounting the filesystem.
     * This will be set after calling nfs_mount which is done in the init()
     * method.
     */
    struct nfs_inode *root_fh = nullptr;

    /*
     * Every RPC request is represented by an rpc_task which is created when
     * the fuse request is received and remains till the NFS server sends a
     * response. rpc_task_helper class allows efficient allocation of RPC
     * tasks.
     */
    struct rpc_task_helper *rpc_task_helper = nullptr;

    /*
     * Holds info about the server, queried by FSINFO.
     */
    struct nfs_server_info server_info;

    /*
     * Holds info about the server, queried by FSSTAT.
     */
    struct nfs_server_stat server_stat;

    nfs_client() :
    	transport(this)
    {
    }

public:
    /*
     * Mount options (to be) used for mounting. These contain details of the
     * server and share that's mounted and also the mount options used.
     */
    struct mount_options mnt_options;

    /*
     * Return the instance of the singleton class.
     */
    static nfs_client& get_instance()
    {
    	static nfs_client client;
        return client;
    }

    struct rpc_task_helper *get_rpc_task_helper()
    {
        return rpc_task_helper;
    }

    /*
     * The user should first init the client class before using it.
     */
    bool init();

    /*
     * Get the libnfs context on which the libnfs API calls can be made.
     */
    struct nfs_context* get_nfs_context() const;

    /*
     * Given an inode number, return the nfs_inode structure.
     * For efficient access we use the address of the nfs_inode structure as
     * the inode number. Fuse should always pass inode numbers we return in
     * one of the create APIs, so it should be ok to trust fuse.
     * Once Fuse calls the forget() API for an inode, it won't pass that
     * inode number in any future request, so we can safely destroy the
     * nfs_inode on forget.
     */
    struct nfs_inode *get_nfs_inode_from_ino(fuse_ino_t ino)
    {
        if (ino == 1 /* FUSE_ROOT_ID */)
        {
            // root_fh must have been created by now.
            assert(root_fh != nullptr);
            assert(root_fh->magic == NFS_INODE_MAGIC);
            return root_fh;
        }

        struct nfs_inode *const nfsi = reinterpret_cast<struct nfs_inode *>(ino);

        // Dangerous cast, deserves validation.
        assert(nfsi->magic == NFS_INODE_MAGIC);

        return nfsi;
    }

    /*
     *
     * Define Nfsv3 API specific functions and helpers after this point.
     *
     * TODO: Add more NFS APIs as we implement them.
     */

    void getattr(
        fuse_req_t req,
        fuse_ino_t inode,
        struct fuse_file_info* file);

    void create(
        fuse_req_t req,
        fuse_ino_t parent_ino,
        const char* name,
        mode_t mode,
        struct fuse_file_info* file);

    void mkdir(
        fuse_req_t req,
        fuse_ino_t parent_ino,
        const char* name,
        mode_t mode);

    void setattr(
        fuse_req_t req,
        fuse_ino_t inode,
        struct stat* attr,
        int to_set,
        struct fuse_file_info* file);

    void lookup(
        fuse_req_t req,
        fuse_ino_t parent_ino,
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

/**
 * We store the nfs_client pointer inside the fuse req private pointer.
 * This allows us to retrieve it fast.
 */
static inline
struct nfs_client *get_nfs_client_from_fuse_req(const fuse_req_t req)
{
    struct nfs_client *const client =
        reinterpret_cast<struct nfs_client*>(fuse_req_userdata(req));

    // Dangerous cast, make sure we got a correct pointer.
    assert(client->magic == NFS_CLIENT_MAGIC);

    return client;
}

#endif /* __NFS_CLIENT_H__ */
