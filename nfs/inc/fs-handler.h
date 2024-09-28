#ifndef __FS_HANDLER_H__
#define __FS_HANDLER_H__

#include "nfs_client.h"

#ifdef ENABLE_NO_FUSE
static inline
int fuse_reply_err(fuse_req_t req, int err)
{
    assert(err >= 0);

    PXT *pxtask = _FR2PXT(req);
    pxtask->res = -err;
    return 0;
}

static inline
int fuse_reply_open(fuse_req_t req, const struct fuse_file_info *f)
{
    PXT *pxtask = _FR2PXT(req);
    pxtask->res = 0;
    return 0;
}
#endif

/*
 * These are FS handlers common between fuse and nofuse mode.
 * Keeping them common ensures that the exact same code works in fuse and
 * nofuse mode. Obviously fuse_req_t does not have the same sigificance
 * in nonfuse mode, instead it's used more as a request context.
 *
 * TODO: Currently it contains many functions which are not needed by nofuse.
 *       Move those to main.cpp.
 */

static void aznfsc_ll_lookup(fuse_req_t req,
                             fuse_ino_t parent_ino,
                             const char *name)
{
    AZLogDebug("aznfsc_ll_lookup(req={}, parent_ino={}, name={})",
               fmt::ptr(req), parent_ino, name);

    struct nfs_client *client = get_nfs_client_from_fuse_req(req);
    client->lookup(req, parent_ino, name);
}

[[maybe_unused]]
static void aznfsc_ll_getattr(fuse_req_t req,
                              fuse_ino_t ino,
                              struct fuse_file_info *fi)
{
    AZLogDebug("aznfsc_ll_getattr(req={}, ino={}, fi={})",
               fmt::ptr(req), ino, fmt::ptr(fi));

    struct nfs_client *client = get_nfs_client_from_fuse_req(req);
    client->getattr(req, ino, fi);
}

[[maybe_unused]]
static void aznfsc_ll_setattr(fuse_req_t req,
                              fuse_ino_t ino,
                              struct stat *attr,
                              int to_set /* bitmask indicating the attributes to set */,
                              struct fuse_file_info *fi)
{
    // TODO: Log all to-be-set attributes.
    AZLogDebug("aznfsc_ll_setattr(req={}, ino={}, to_set=0x{:x}, fi={})",
               fmt::ptr(req), ino, to_set, fmt::ptr(fi));

    struct nfs_client *client = get_nfs_client_from_fuse_req(req);
    client->setattr(req, ino, attr, to_set, fi);
}

[[maybe_unused]]
static void aznfsc_ll_readlink(fuse_req_t req,
                               fuse_ino_t ino)
{
    AZLogDebug("aznfsc_ll_readlink(req={}, ino={})",
               fmt::ptr(req), ino);

    struct nfs_client *client = get_nfs_client_from_fuse_req(req);
    client->readlink(req, ino);
}

[[maybe_unused]]
static void aznfsc_ll_mknod(fuse_req_t req,
                            fuse_ino_t parent_ino,
                            const char *name,
                            mode_t mode,
                            dev_t rdev)
{
    AZLogDebug("aznfsc_ll_mknod(req={}, parent_ino={}, name={}, "
               "mode=0{:03o})",
               fmt::ptr(req), parent_ino, name, mode);

    if (S_ISREG(mode)) {
        struct nfs_client *client = get_nfs_client_from_fuse_req(req);
        client->mknod(req, parent_ino, name, mode);
    } else {
        AZLogError("mknod(req={}, parent_ino={}, name={}, "
                   "mode=0{:03o}) is unsupported for non-regular files.",
                   fmt::ptr(req), parent_ino, name, mode);
        fuse_reply_err(req, ENOSYS);
    }
}

[[maybe_unused]]
static void aznfsc_ll_mkdir(fuse_req_t req,
                            fuse_ino_t parent_ino,
                            const char *name,
                            mode_t mode)
{
    AZLogDebug("aznfsc_ll_mkdir(req={}, parent_ino={}, name={}, mode=0{:03o})",
               fmt::ptr(req), parent_ino, name, mode);

    struct nfs_client *client = get_nfs_client_from_fuse_req(req);
    client->mkdir(req, parent_ino, name, mode);
}

[[maybe_unused]]
static void aznfsc_ll_unlink(fuse_req_t req,
                             fuse_ino_t parent_ino,
                             const char *name)
{
    AZLogDebug("aznfsc_ll_unlink(req={}, parent_ino={}, name={})",
               fmt::ptr(req), parent_ino, name);

    struct nfs_client *client = get_nfs_client_from_fuse_req(req);

    /*
     * Call silly rename to see if it wants to silly rename instead of unlink.
     * We will perform silly rename if the opencnt of the file is not 0, i.e.,
     * some process has the file open. This is for POSIX compliance, where
     * open files should be accessible till the last open handle is closed.
     * Depending on the silly_rename status this will reply to the fuse unlink
     * request.
     */
    if (client->silly_rename(req, parent_ino, name)) {
        return;
    }

    client->unlink(req, parent_ino, name);
}

[[maybe_unused]]
static void aznfsc_ll_rmdir(fuse_req_t req,
                            fuse_ino_t parent_ino,
                            const char *name)
{
    AZLogDebug("aznfsc_ll_rmdir(req={}, parent_ino={}, name={})",
               fmt::ptr(req), parent_ino, name);

    struct nfs_client *client = get_nfs_client_from_fuse_req(req);
    client->rmdir(req, parent_ino, name);
}

[[maybe_unused]]
static void aznfsc_ll_symlink(fuse_req_t req,
                              const char *link,
                              fuse_ino_t parent_ino,
                              const char *name)
{
    AZLogDebug("aznfsc_ll_symlink(req={}, link={}, parent_ino={}, name={})",
               fmt::ptr(req), link, parent_ino, name);

    struct nfs_client *client = get_nfs_client_from_fuse_req(req);
    client->symlink(req, link, parent_ino, name);
}

[[maybe_unused]]
static void aznfsc_ll_rename(fuse_req_t req,
                             fuse_ino_t parent_ino,
                             const char *name,
                             fuse_ino_t newparent_ino,
                             const char *newname,
                             unsigned int flags)
{
    /*
     * We don't support renameat2() i.e., no support for `RENAME_EXCHANGE` or
     * `RENAME_NOREPLACE` flags. Force flags to 0. Default NFS rename behaviour
     * should be fine.
     */
    if (flags != 0) {
        AZLogWarn("aznfsc_ll_rename(req={}, parent_ino={}, name={}, "
                  "newparent_ino={}, newname={}, flags={})",
                  fmt::ptr(req), parent_ino, name,
                  newparent_ino, newname, flags);
        flags = 0;
    } else {
        AZLogDebug("aznfsc_ll_rename(req={}, parent_ino={}, name={}, "
                   "newparent_ino={}, newname={}, flags={})",
                   fmt::ptr(req), parent_ino, name,
                   newparent_ino, newname, flags);
    }

    struct nfs_client *client = get_nfs_client_from_fuse_req(req);
    client->rename(req, parent_ino, name, newparent_ino, newname,
                   false, 0, flags);
}

[[maybe_unused]]
static void aznfsc_ll_link(fuse_req_t req,
                           fuse_ino_t ino,
                           fuse_ino_t newparent_ino,
                           const char *newname)
{
    /*
     * TODO: Fill me.
     */
    fuse_reply_err(req, ENOSYS);
}

[[maybe_unused]]
static void aznfsc_ll_open(fuse_req_t req,
                           fuse_ino_t ino,
                           struct fuse_file_info *fi)
{
    AZLogInfo("aznfsc_ll_open(req={}, ino={}, fi={})",
              fmt::ptr(req), ino, fmt::ptr(fi));

    /*
     * We plan to manage our own file cache for better control over writes.
     *
     * Note: We don't need to set these explicitly as they default to
     *       these values, we do it to highlight our intent.
     *
     * TODO: Explore kernel caching, its benefits and side-effects.
     */
    fi->direct_io = !aznfsc_cfg.cache.data.kernel.enable;
    fi->keep_cache = aznfsc_cfg.cache.data.kernel.enable;
    fi->nonseekable = 0;
    fi->parallel_direct_writes = 1;
    fi->noflush = 0;

    struct nfs_client *client = get_nfs_client_from_fuse_req(req);
    struct nfs_inode *inode = client->get_nfs_inode_from_ino(ino);

    // Make sure it's not called for directories.
    assert(!inode->is_dir());

    /*
     * TODO: See comments in readahead.h, ideally readahead state should be
     *       per file pointer (per open handle) but since fuse doesn't let
     *       us know the file pointer we maintain readahead state per inode.
     *       We must reset the readahead state so that this file handle can
     *       correctly perform readaheads and not confuse this as an access
     *       using the prev handle. This means if we open more than one handle
     *       simultaneously it will cause the readahead state to be reset.
     *
     *       This is a hack and needs to be properly addressed!
     */
    if (inode->is_regfile() && !inode->is_cache_empty()) {
        AZLogInfo("[{}] Clearing cache", ino);
        inode->filecache_handle->clear();
    }

    if (inode->is_regfile() && inode->readahead_state) {
        AZLogInfo("[{}] Resetting readahead state", ino);
        inode->readahead_state->reset();
    }

    /*
     * If file cache is not allocated, allocate now.
     * Mostly it'll be allocated in nfs_client::reply_entry().
     */
    if (inode->is_regfile()) {
        inode->get_or_alloc_filecache();
    } else if (inode->is_dir()) {
        inode->get_or_alloc_dircache();
    }

    inode->opencnt++;

    fuse_reply_open(req, fi);
}

[[maybe_unused]]
static void aznfsc_ll_read(fuse_req_t req,
                           fuse_ino_t ino,
                           size_t size,
                           off_t off,
                           struct fuse_file_info *fi)
{
    AZLogDebug("aznfsc_ll_read(req={}, ino={}, size={}, offset={} fi={}",
                fmt::ptr(req), ino, size, off, fmt::ptr(fi));

    /*
     * Sanity assert. 1MiB is the max read size fuse will ever issue.
     * If fuse sends more we'd like to know.
     *
     * TODO: Remove this before going to production.
     */
    assert(size <= 1048576);

    struct nfs_client *client = get_nfs_client_from_fuse_req(req);
    client->read(req, ino, size, off, fi);
}

[[maybe_unused]]
static void aznfsc_ll_write(fuse_req_t req,
                            fuse_ino_t ino,
                            const char *buf,
                            size_t size,
                            off_t off,
                            struct fuse_file_info *fi)
{
    /*
     * XXX: write will be never called as we implement write_buf.
     */
    AZLogError("aznfsc_ll_write(req={}, ino={}, buf={}, size={}, off={}, fi={})",
               fmt::ptr(req), ino, fmt::ptr(buf), size, off, fmt::ptr(fi));

    fuse_reply_err(req, ENOSYS);
}

[[maybe_unused]]
static void aznfsc_ll_flush(fuse_req_t req,
                            fuse_ino_t ino,
                            struct fuse_file_info *fi)
{
    AZLogDebug("aznfsc_ll_flush(req={}, ino={}, fi={})",
               fmt::ptr(req), ino, fmt::ptr(fi));

    struct nfs_client *client = get_nfs_client_from_fuse_req(req);
    client->flush(req, ino);
}

[[maybe_unused]]
static void aznfsc_ll_release(fuse_req_t req,
                              fuse_ino_t ino,
                              struct fuse_file_info *fi)
{
    /*
     * Fuse calls flush() for every fd closed and release() once per file,
     * when the last fd to that file is closed.
     * Though we shouldn't need the flush here but for safety we put it
     * here as fuse doc says flush()) may not be called.
     */
    AZLogDebug("aznfsc_ll_release(req={}, ino={}, fi={})",
               fmt::ptr(req), ino, fmt::ptr(fi));

    struct nfs_client *client = get_nfs_client_from_fuse_req(req);
    struct nfs_inode *inode = client->get_nfs_inode_from_ino(ino);

    // Must be called for an open file.
    assert(inode->is_open());

    /*
     * inode release() will return true if the inode was silly renamed and it
     * initiated an unlink of the inode. In that case fuse will be sent a
     * response based on the unlink outcome. Also, we don't flush in that case
     * as the file is going to be unlinked anyways.
     */
    if (!inode->release(req)) {
        client->flush(req, ino);
    }
}

[[maybe_unused]]
static void aznfsc_ll_fsync(fuse_req_t req,
                            fuse_ino_t ino,
                            int datasync,
                            struct fuse_file_info *fi)
{
    /*
     * TODO: Fill me.
     */
    fuse_reply_err(req, ENOSYS);
}

[[maybe_unused]]
static void aznfsc_ll_opendir(fuse_req_t req,
                              fuse_ino_t ino,
                              struct fuse_file_info *fi)
{
    AZLogDebug("aznfsc_ll_opendir(req={}, ino={}, fi={})",
               fmt::ptr(req), ino, fmt::ptr(fi));

    /*
     * We manage our own readdir cache and we don't want kernel to
     * cache directory contents.
     *
     * Note: We don't need to set these explicitly as they default to
     *       these values, we do it to highlight our intent.
     * TODO: Later explore if kernel cacheing directory content is beneficial
     *       and what are the side effects, if any.
     */
    fi->direct_io = !aznfsc_cfg.cache.readdir.kernel.enable;
    fi->keep_cache = 1;
    fi->nonseekable = 0;
    fi->cache_readdir = aznfsc_cfg.cache.readdir.kernel.enable;
    fi->noflush = 0;

    struct nfs_client *client = get_nfs_client_from_fuse_req(req);
    struct nfs_inode *inode = client->get_nfs_inode_from_ino(ino);

    assert(inode->is_dir());

    inode->get_or_alloc_dircache();
    fuse_reply_open(req, fi);
}

[[maybe_unused]]
static void aznfsc_ll_readdir(fuse_req_t req,
                              fuse_ino_t ino,
                              size_t size,
                              off_t off,
                              struct fuse_file_info *fi)
{
    AZLogDebug("aznfsc_ll_readdir(req={}, ino={}, size={}, off={}, fi={})",
               fmt::ptr(req), ino, size, off, fmt::ptr(fi));

    struct nfs_client *client = get_nfs_client_from_fuse_req(req);
    client->readdir(req, ino, size, off, fi);
}

[[maybe_unused]]
static void aznfsc_ll_releasedir(fuse_req_t req,
                                 fuse_ino_t ino,
                                 struct fuse_file_info *fi)
{
    AZLogDebug("aznfsc_ll_releasedir(req={}, ino={}, fi={})",
               fmt::ptr(req), ino, fmt::ptr(fi));

    /*
     * We don't do anything in opendir() so nothing to be done in
     * releasedir().
     *
     * TODO: See if we want to flush the directory buffer to create
     *       space. This may be helpful for find(1)workloads which
     *       traverse a directory just once.
     */

     fuse_reply_err(req, 0);
}

[[maybe_unused]]
static void aznfsc_ll_fsyncdir(fuse_req_t req,
                               fuse_ino_t ino,
                               int datasync,
                               struct fuse_file_info *fi)
{
    /*
     * TODO: Fill me.
     */
    fuse_reply_err(req, ENOSYS);
}

[[maybe_unused]]
static void aznfsc_ll_statfs(fuse_req_t req,
                             fuse_ino_t ino)
{
    AZLogDebug("aznfsc_ll_statfs(req={}, ino={})", fmt::ptr(req), ino);

    struct nfs_client *client = get_nfs_client_from_fuse_req(req);
    client->statfs(req, ino);
}

[[maybe_unused]]
static void aznfsc_ll_setxattr(fuse_req_t req,
                               fuse_ino_t ino,
                               const char *name,
                               const char *value,
                               size_t size,
                               int flags)
{
    /*
     * TODO: Fill me.
     */
    fuse_reply_err(req, ENOSYS);
}

[[maybe_unused]]
static void aznfsc_ll_getxattr(fuse_req_t req,
                               fuse_ino_t ino,
                               const char *name,
                               size_t size)
{
    /*
     * TODO: Fill me.
     */
    fuse_reply_err(req, ENOSYS);
}

[[maybe_unused]]
static void aznfsc_ll_listxattr(fuse_req_t req,
                                fuse_ino_t ino,
                                size_t size)
{
    /*
     * TODO: Fill me.
     */
    fuse_reply_err(req, ENOSYS);
}

[[maybe_unused]]
static void aznfsc_ll_removexattr(fuse_req_t req,
                                  fuse_ino_t ino,
                                  const char *name)
{
    /*
     * TODO: Fill me.
     */
    fuse_reply_err(req, ENOSYS);
}

[[maybe_unused]]
static void aznfsc_ll_access(fuse_req_t req,
                             fuse_ino_t ino,
                             int mask)
{
    AZLogDebug("aznfsc_ll_access(req={}, ino={}, mask=0{:03o})",
               fmt::ptr(req), ino, mask);

    struct nfs_client *client = get_nfs_client_from_fuse_req(req);
    client->access(req, ino, mask);
}

[[maybe_unused]]
static void aznfsc_ll_create(fuse_req_t req,
                             fuse_ino_t parent_ino,
                             const char *name,
                             mode_t mode,
                             struct fuse_file_info *fi)
{
    AZLogDebug("aznfsc_ll_create(req={}, parent_ino={}, name={}, "
               "mode=0{:03o}, fi={})",
               fmt::ptr(req), parent_ino, name, mode, fmt::ptr(fi));

    /*
     * See aznfsc_ll_open().
     */
    fi->direct_io = !aznfsc_cfg.cache.data.kernel.enable;
    fi->keep_cache = aznfsc_cfg.cache.data.kernel.enable;
    fi->nonseekable = 0;
    fi->parallel_direct_writes = 1;
    fi->noflush = 0;

    struct nfs_client *client = get_nfs_client_from_fuse_req(req);
    client->create(req, parent_ino, name, mode, fi);
}

[[maybe_unused]]
static void aznfsc_ll_getlk(fuse_req_t req,
                            fuse_ino_t ino,
                            struct fuse_file_info *fi,
                            struct flock *lock)
{
    /*
     * TODO: Fill me.
     */
    fuse_reply_err(req, ENOSYS);
}

[[maybe_unused]]
static void aznfsc_ll_setlk(fuse_req_t req,
                            fuse_ino_t ino,
                            struct fuse_file_info *fi,
                            struct flock *lock,
                            int sleep)
{
    /*
     * TODO: Fill me.
     */
    fuse_reply_err(req, ENOSYS);
}

[[maybe_unused]]
static void aznfsc_ll_bmap(fuse_req_t req,
                           fuse_ino_t ino,
                           size_t blocksize,
                           uint64_t idx)
{
    /*
     * TODO: Fill me.
     */
    fuse_reply_err(req, ENOSYS);
}

#if FUSE_USE_VERSION < 35
[[maybe_unused]]
static void aznfsc_ll_ioctl(fuse_req_t req,
                            fuse_ino_t ino,
                            int cmd,
                            void *arg,
                            struct fuse_file_info *fi,
                            unsigned flags,
                            const void *in_buf,
                            size_t in_bufsz,
                            size_t out_bufsz)
{
    /*
     * TODO: Fill me.
     */
    fuse_reply_err(req, ENOSYS);
}
#else
[[maybe_unused]]
static void aznfsc_ll_ioctl(fuse_req_t req,
                            fuse_ino_t ino,
                            unsigned int cmd,
                            void *arg,
                            struct fuse_file_info *fi,
                            unsigned flags,
                            const void *in_buf,
                            size_t in_bufsz,
                            size_t out_bufsz)
{
    /*
     * TODO: Fill me.
     */
    fuse_reply_err(req, ENOSYS);
}
#endif

[[maybe_unused]]
static void aznfsc_ll_poll(fuse_req_t req,
                           fuse_ino_t ino,
                           struct fuse_file_info *fi,
                           struct fuse_pollhandle *ph)
{
    /*
     * TODO: Fill me.
     */
    fuse_reply_err(req, ENOSYS);
}

[[maybe_unused]]
static void aznfsc_ll_write_buf(fuse_req_t req,
                                fuse_ino_t ino,
                                struct fuse_bufvec *bufv,
                                off_t off,
                                struct fuse_file_info *fi)
{
    assert(bufv->idx < bufv->count);
    const size_t length = bufv->buf[bufv->idx].size - bufv->off;
    assert((int) length >= 0);

    /*
     * XXX We are only handling count=1, assert to know if kernel sends more,
     *     we would want to handle that.
     */
    assert(bufv->count == 1);

    AZLogDebug("aznfsc_ll_write_buf(req={}, ino={}, bufv={}, off={}, len={}, "
               "fi={} [writepage: {}, flush: {}]",
               fmt::ptr(req), ino, fmt::ptr(bufv), off, length, fmt::ptr(fi),
               fi->writepage ? 1 : 0, fi->flush ? 1 : 0);

    struct nfs_client *client = get_nfs_client_from_fuse_req(req);

    /*
     * Sanity assert. 1MiB is the max write size fuse will ever issue.
     * If fuse sends more we'd like to know.
     *
     * TODO: Remove this before going to production.
     */
    assert(length <= 1048576);

    client->write(req, ino, bufv, length, off);
}

[[maybe_unused]]
static void aznfsc_ll_retrieve_reply(fuse_req_t req,
                                     void *cookie,
                                     fuse_ino_t ino,
                                     off_t offset,
                                     struct fuse_bufvec *bufv)
{
    /*
     * TODO: Fill me.
     */
    fuse_reply_err(req, ENOSYS);
}

[[maybe_unused]]
static void aznfsc_ll_flock(fuse_req_t req,
                            fuse_ino_t ino,
                            struct fuse_file_info *fi,
                            int op)
{
    /*
     * TODO: Fill me.
     */
    fuse_reply_err(req, ENOSYS);
}

[[maybe_unused]]
static void aznfsc_ll_fallocate(fuse_req_t req,
                                fuse_ino_t ino,
                                int mode,
                                off_t offset,
                                off_t length,
                                struct fuse_file_info *fi)
{
    /*
     * TODO: Fill me.
     */
    fuse_reply_err(req, ENOSYS);
}

[[maybe_unused]]
static void aznfsc_ll_readdirplus(fuse_req_t req,
                                  fuse_ino_t ino,
                                  size_t size,
                                  off_t off,
                                  struct fuse_file_info *fi)
{
    AZLogDebug("aznfsc_ll_readdirplus(req={}, ino={}, size={}, off={}, fi={})",
               fmt::ptr(req), ino, size, off, fmt::ptr(fi));

    struct nfs_client *client = get_nfs_client_from_fuse_req(req);
    client->readdirplus(req, ino, size, off, fi);
}

void aznfsc_ll_copy_file_range(fuse_req_t req,
                               fuse_ino_t ino_in,
                               off_t off_in,
                               struct fuse_file_info *fi_in,
                               fuse_ino_t ino_out,
                               off_t off_out,
                               struct fuse_file_info *fi_out,
                               size_t len,
                               int flags)
{
    /*
     * TODO: Fill me.
     */
    fuse_reply_err(req, ENOSYS);
}

[[maybe_unused]]
static void aznfsc_ll_lseek(fuse_req_t req,
                            fuse_ino_t ino,
                            off_t off,
                            int whence,
                            struct fuse_file_info *fi)
{
    /*
     * TODO: Fill me.
     */
    fuse_reply_err(req, ENOSYS);
}

#endif /* __FS_HANDLER_H__ */
