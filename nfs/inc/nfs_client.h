#ifndef __NFS_CLIENT_H__
#define __NFS_CLIENT_H__

#include <queue>

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

/**
 * RPC requests that fail with JUKEBOX error are retried after these many secs.
 * We try after 5 seconds similar to Linux NFS client.
 */
#define JUKEBOX_DELAY_SECS 5

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
     * Map of all inodes returned to fuse and which are not FORGET'ed
     * by fuse. The idea behind this map is to make sure we never return
     * two different fuse_ino_t inode number for the same file, lest it'll
     * confuse the VFS layer. This is achieved by adding any inode we
     * return to fuse, to this map.
     * An inode will be removed from the map only when all the following
     * conditions are met:
     * 1. inode->lookupcnt becomes 0.
     *    This confirms that fuse vfs does not have this inode and hence
     *    it cannnot make any call on this inode.
     * 2. inode->dircachecnt becomes 0.
     *    Whenever we cache directory_entry for readdirplus, the
     *    directory_entry also refers to the inode and hence we need to
     *    make sure that the inode is not freed till any directory_entry
     *    is referring to it.
     */
    std::multimap<uint64_t /* fileid */, struct nfs_inode*> inode_map;
    std::shared_mutex inode_map_lock;

    /*
     * Every RPC request is represented by an rpc_task which is created when
     * the fuse request is received and remains till the NFS server sends a
     * response. rpc_task_helper class allows efficient allocation of RPC
     * tasks.
     */
    struct rpc_task_helper *rpc_task_helper = nullptr;

    /*
     * JUKEBOX errors are handled by re-running the nfs_client handler for the
     * given request, f.e., for a READDIRPLUS request failing with JUKEBOX error
     * we will call nfs_client::readdirplus() again after JUKEBOX_DELAY_SECS
     * seconds. For this we need to save enough information needed to run the
     * nfs_client handler. jukebox_seedinfo stores that information and we
     * queue that in jukebox_seeds.
     */
    std::thread jukebox_thread;
    void jukebox_runner();
    std::queue<struct jukebox_seedinfo*> jukebox_seeds;
    mutable std::mutex jukebox_seeds_lock;

    /*
     * Holds info about the server, queried by FSINFO.
     */
    struct nfs_server_info server_info;

    /*
     * Holds info about the server, queried by FSSTAT.
     */
    struct nfs_server_stat server_stat;

    /*
     * Since we use the address of nfs_inode as the inode number we
     * return to fuse, this is a small sanity check we do to check if
     * fuse is passing us valid inode numbers.
     */
    uint64_t min_ino = UINT64_MAX;
    uint64_t max_ino = 0;

    /*
     * Set in shutdown() to let others know that nfs_client is shutting
     * down. They can use this to quit what they are doing and plan for
     * graceful shutdown.
     */
    std::atomic<bool> shutting_down = false;

    nfs_client() :
        transport(this),
        jukebox_thread(std::thread(&nfs_client::jukebox_runner, this))
    {
    }

    ~nfs_client()
    {
        /*
         * Drop the initial ref held in nfs_client::init().
         */
        if (root_fh) {
            root_fh->decref();
            root_fh = nullptr;
        }
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

    /**
     * Returns true if nfs_client is shutting down.
     */
    bool is_shutting_down() const
    {
        return shutting_down;
    }

    /**
     * Must be called on fuse unmount.
     * TODO: Audit this to make sure we perform cleanup for all components.
     */
    void shutdown()
    {
        assert(!shutting_down);
        shutting_down = true;

#ifdef ENABLE_PARANOID
        for (auto it : inode_map) {
            const struct nfs_inode *inode = it.second;
            AZLogDebug("[{}:{}] Inode still present at shutdown: "
                       "lookupcnt={}, dircachecnt={}, forget_seen={}, "
                       "is_cache_empty={}",
                       inode->get_filetype_coding(),
                       inode->get_fuse_ino(),
                       inode->lookupcnt.load(),
                       inode->dircachecnt.load(),
                       inode->forget_seen,
                       inode->is_cache_empty());
        }
#endif

        transport.close();
        jukebox_thread.join();
    }

    const struct rpc_transport& get_transport() const
    {
        return transport;
    }

    struct rpc_task_helper *get_rpc_task_helper()
    {
        return rpc_task_helper;
    }

    std::shared_mutex& get_inode_map_lock()
    {
        return inode_map_lock;
    }

    /*
     * The user should first init the client class before using it.
     */
    bool init();

    /*
     * Get the libnfs context on which the libnfs API calls can be made.
     *
     * csched:  The connection scheduling type to be used when selecting the
     *          NFS context/connection.
     * fh_hash: Filehandle hash, used only when CONN_SCHED_FH_HASH scheduling
     *          mode is used. This provides a unique hash for the file/dir
     *          that is the target for this request. All requests to the same
     *          file/dir are sent over the same connection.
     */
    struct nfs_context* get_nfs_context(conn_sched_t csched,
                                        uint32_t fh_hash) const;

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
        // 0 is not a valid inode number.
        assert(ino != 0);

        if (ino == FUSE_ROOT_ID)
        {
            // root_fh must have been created by now.
            assert(root_fh != nullptr);
            assert(root_fh->magic == NFS_INODE_MAGIC);
            return root_fh;
        }

        assert(ino >= min_ino);
        assert(ino <= max_ino);

        struct nfs_inode *const nfsi =
            reinterpret_cast<struct nfs_inode *>(ino);

        // Dangerous cast, deserves validation.
        assert(nfsi->magic == NFS_INODE_MAGIC);

        return nfsi;
    }

    /**
     * Given a filehandle and fattr (oontaining fileid defining a file/dir),
     * get the nfs_inode for that file/dir. It searches in the global list of
     * all inodes and returns from there if found, else creates a new nfs_inode.
     * Note that we don't want to return multiple fuse inodes for the same
     * file (represented by the filehandle). If fuse guarantees that it'll
     * never make a lookup or any other call that gets a new inode, until
     * it calls forget for that inode, then we can probably use different
     * inodes for the same file but not at the same time. Since fuse doesn't
     * guarantee we play safe and make sure for a given file we use the
     * same nfs_inode as long one is cached with us. New incarnation of
     * fuse driver will give a different fuse ino for the same file, but
     * that should be ok.
     * It'll grab a refcnt on the inode before returning. Caller must ensure
     * that the ref is duly dropped at an appropriate time. Most commonly
     * this refcnt held by get_nfs_inode() is trasferred to fuse and is
     * dropped when fuse FORGETs the inode.
     * 'is_root_inode' must be set when the inode being requested is the
     * root inode. Root inode is special in that it has the special fuse inode
     * number of 1, rest other inodes have inode number as the address of
     * the nfs_inode structure, which allows fast ino->inode mapping.
     */
    struct nfs_inode *get_nfs_inode(const nfs_fh3 *fh,
                                    const struct fattr3 *fattr,
                                    bool is_root_inode = false);

    /**
     * Release the given inode, called when fuse FORGET call causes the
     * inode lookupcnt to drop to 0, i.e., the inode is no longer in use
     * by fuse VFS. Note that it takes a dropcnt parameter which is the
     * nlookup parameter passed by fuse FORGET. Instead of the caller
     * reducing lookupcnt and then calling put_nfs_inode(), the caller
     * passes the amount by which the lookupcnt must be dropped. This is
     * important as we need to drop the lookupcnt inside the inode_map_lock,
     * else if we drop before the lock and lookupcnt becomes 0, some other
     * thread can delete the inode while we still don't have the lock, and
     * then when we proceed to delete the inode, we would be accessing the
     * already deleted inode.
     *
     * If the inode lookupcnt (after reducing by dropcnt), becomes 0 and it's
     * not referenced by any readdirectory_cache (inode->dircachecnt is 0)
     * then the inode is removed from the inode_map and freed.
     *
     * This nolock version does not hold the inode_map_lock so the caller
     * must hold the lock before calling this. Usually you will call one of
     * the other variants which hold the lock.
     */
    void put_nfs_inode_nolock(struct nfs_inode *inode, size_t dropcnt);

    void put_nfs_inode(struct nfs_inode *inode, size_t dropcnt)
    {
        AZLogDebug("[{}] put_nfs_inode(dropcnt={}) called",
                   inode->get_fuse_ino(), dropcnt);
        /*
         * We need to hold the inode_map_lock while we check the inode for
         * eligibility to remove (and finally remove) from the inode_map.
         */
        std::unique_lock<std::shared_mutex> lock(inode_map_lock);
        put_nfs_inode_nolock(inode, dropcnt);
    }

    /*
     *
     * Define Nfsv3 API specific functions and helpers after this point.
     *
     * TODO: Add more NFS APIs as we implement them.
     */

    void getattr(
        fuse_req_t req,
        fuse_ino_t ino,
        struct fuse_file_info* file);

    /**
     * Issue a sync GETATTR RPC call to filehandle 'fh' and save the received
     * attributes in 'fattr'.
     * This is to be used internally and not for serving fuse requests.
     */
    bool getattr_sync(const struct nfs_fh3& fh,
                      fuse_ino_t ino,
                      struct fattr3& attr);

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

    /*
     * Try to perform silly rename of the given file and return true if silly
     * rename was required (and done), else return false.
     * Note that silly rename is required for a file that's open when unlink
     * request is received for it.
     */
    bool silly_rename(
        fuse_req_t req,
        fuse_ino_t parent_ino,
        const char *name);

    void unlink(
        fuse_req_t req,
        fuse_ino_t parent_ino,
        const char* name);

    void rmdir(
        fuse_req_t req,
        fuse_ino_t parent_ino,
        const char* name);

    void symlink(
        fuse_req_t req,
        const char *link,
        fuse_ino_t parent_ino,
        const char *name);

    /**
     * silly_rename must be passed as true if this is a silly rename and not
     * rename triggered by user. We silly rename a file that's being unlinked
     * while it has a non-zero opencnt.
     * In that case, silly_rename_ino is the ino of the file that's being
     * unlinked.
     */
    void rename(
        fuse_req_t req,
        fuse_ino_t parent_ino,
        const char *name,
        fuse_ino_t newparent_ino,
        const char *new_name,
        bool silly_rename,
        fuse_ino_t silly_rename_ino,
        unsigned int flags);

    void readlink(
        fuse_req_t req,
        fuse_ino_t ino);

    void setattr(
        fuse_req_t req,
        fuse_ino_t ino,
        const struct stat* attr,
        int to_set,
        struct fuse_file_info* file);

    void lookup(
        fuse_req_t req,
        fuse_ino_t parent_ino,
        const char* name);

    /**
     * Sync version of lookup().
     * This is to be used internally and not for serving fuse requests.
     * It returns true if we are able to get a success response for the
     * LOOKUP RPC that we sent, in that case child_ino will contain the
     * child's fuse inode number.
     */
    bool lookup_sync(
        fuse_ino_t parent_ino,
        const char *name,
        fuse_ino_t& child_ino);

    void access(
        fuse_req_t req,
        fuse_ino_t ino,
        int mask);

    void write(
        fuse_req_t req,
        fuse_ino_t ino,
        struct fuse_bufvec *bufv,
        size_t size,
        off_t off);

    void flush(
        fuse_req_t req,
        fuse_ino_t ino);

    void readdir(
        fuse_req_t req,
        fuse_ino_t ino,
        size_t size,
        off_t off,
        struct fuse_file_info* file);

    void readdirplus(
        fuse_req_t req,
        fuse_ino_t ino,
        size_t size,
        off_t off,
        struct fuse_file_info* file);

    void read(
        fuse_req_t req,
        fuse_ino_t ino,
        size_t size,
        off_t off,
        struct fuse_file_info *fi);

    void jukebox_read(struct api_task_info *rpc_api);

    void jukebox_flush(struct api_task_info *rpc_api);

    static void stat_from_fattr3(struct stat* st, const struct fattr3* attr);

    void reply_entry(
        struct rpc_task* ctx,
        const nfs_fh3* fh,
        const struct fattr3* attr,
        const struct fuse_file_info* file);

    /**
     * Call this to handle NFS3ERR_JUKEBOX error received for rpc_task.
     * This will save information needed to re-issue the call and queue
     * it in jukebox_seeds from where jukebox_runner will issue the call
     * after JUKEBOX_DELAY_SECS seconds.
     */
    void jukebox_retry(struct rpc_task *task);
};

/**
 * Sync RPC calls can use this context structure to communicate between
 * issuer and the callback.
 */
#define SYNC_RPC_CTX_MAGIC *((const uint32_t *)"SRCX")

struct sync_rpc_context
{
    const uint32_t magic = SYNC_RPC_CTX_MAGIC;
    /*
     * Set by the callback to convey that callback is indeed called.
     * Issuer can find this to see if it timed out waiting for the callback.
     */
    bool callback_called = false;

    /*
     * RPC and NFS status, only valid if callback_called is true.
     * Also, nfs_status is only valid if rpc_status is RPC_STATUS_SUCCESS.
     */
    int rpc_status = -1;
    int nfs_status = -1;

    /*
     * Condition variable on which the issuer will wait for the callback to
     * be called.
     */
    std::condition_variable cv;
    std::mutex mutex;

    /*
     * The rpc_task tracking the actual RPC call.
     */
    struct rpc_task *const task;

    /*
     * Most NFS RPCs carry postop attributes. If this is not null, callback
     * will fill this with the postop attributes received.
     */
    struct fattr3 *const fattr = nullptr;

    sync_rpc_context(struct rpc_task *_task, struct fattr3 *_fattr):
        task(_task),
        fattr(_fattr)
    {
    }
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
