#include <stdlib.h>
#include <stdio.h>
#include <stdarg.h>
#include <limits.h>
#include <sys/types.h>
#include <sys/stat.h>
#include <fcntl.h>
#include <dlfcn.h>

#include <filesystem>
#include <atomic>

#include "aznfsc.h"
#include "rpc_task.h"
#include "nfs_inode.h"
#include "fs-handler.h"

namespace aznfsc {

decltype(PXT::__readlink_orig) posix_task::__readlink_orig = nullptr;
decltype(PXT::____xstat_orig) posix_task::____xstat_orig = nullptr;
decltype(PXT::____fxstat_orig) posix_task::____fxstat_orig = nullptr;
decltype(PXT::__open_orig) posix_task::__open_orig = nullptr;
decltype(PXT::__fopen_orig) posix_task::__fopen_orig = nullptr;
decltype(PXT::__close_orig) posix_task::__close_orig = nullptr;
decltype(PXT::__dup_orig) posix_task::__dup_orig = nullptr;
decltype(PXT::__dup2_orig) posix_task::__dup2_orig = nullptr;
decltype(PXT::__read_orig) posix_task::__read_orig = nullptr;
decltype(PXT::client) posix_task::client = nullptr;
std::unordered_map<int, struct fdinfo> posix_task::fdmap;
std::mutex posix_task::fdmap_mutex;

/*
 * We don't want to intercept any calls made before we are correctly init'ed
 * or during cleanup.
 */
static std::atomic<bool> init_done = false;
static std::atomic<bool> in_cleanup = false;

static void __nofuse_cleanup(void)
{
    /*
     * If we call exit() from our cleanup code, don't re-start cleanup.
     */
    if (in_cleanup.exchange(true) == true) {
        AZLogWarn("[NOFUSE] Recursive call to __nofuse_cleanup, ignoring!");
        return;
    }

    AZLogInfo("Shutting down!");

    nfs_client::get_instance().shutdown();
}

__attribute__((constructor))
static void __nofuse_init()
{
    /*
     * Must be called only once.
     */
    assert(!aznfsc_cfg.config_yaml);
    assert(aznfsc_cfg.mountpoint.empty());

    init_log();

    const char *nofuse_debug = ::getenv("AZNFSC_NOFUSE_DEBUG");
    if (nofuse_debug && (::atoi(nofuse_debug) == 1)) {
        enable_debug_logs = true;
        spdlog::set_level(spdlog::level::debug);
    }

    aznfsc_cfg.config_yaml = ::getenv("AZNFSC_NOFUSE_CONFIG_YAML");
    if (!aznfsc_cfg.config_yaml) {
        AZLogError("[NOFUSE] Environment variable AZNFSC_NOFUSE_CONFIG_YAML not set!");
        ::exit(-1);
    }

    /*
     * Parse config yaml.
     */
    if (!aznfsc_cfg.parse_config_yaml()) {
        assert(aznfsc_cfg.mountpoint.empty());
        AZLogError("[NOFUSE] Failed to parse aznfsc config {}!", aznfsc_cfg.config_yaml);
        ::exit(-2);
    }

    /*
     * account and container are mandatory parameters which do not have a
     * default value, so ensure they are set before proceeding further.
     */
    if (aznfsc_cfg.account == nullptr) {
        assert(aznfsc_cfg.mountpoint.empty());
        AZLogError("[NOFUSE] Account name not found in {}!", aznfsc_cfg.config_yaml);
        ::exit(-3);
    }

    if (aznfsc_cfg.container == nullptr) {
        assert(aznfsc_cfg.mountpoint.empty());
        AZLogError("[NOFUSE] Container name not found in {}!", aznfsc_cfg.config_yaml);
        ::exit(-4);
    }

    const char *nofuse_root = ::getenv("AZNFSC_NOFUSE_ROOT");
    if (!nofuse_root) {
        AZLogError("[NOFUSE] Environment variable AZNFSC_NOFUSE_ROOT not set!");
        ::exit(-5);
    }

    char *nofuse_root_abs = ::realpath(nofuse_root, NULL);
    if (!nofuse_root_abs) {
        AZLogError("[NOFUSE] __nofuse_init: realpath({}) failed: {}",
                   nofuse_root, ::strerror(errno));
        ::exit(-6);
    }

    if (::strcmp(nofuse_root, nofuse_root_abs) != 0) {
        AZLogError("[NOFUSE] __nofuse_init: AZNFSC_NOFUSE_ROOT *must* be a "
                   "canonical path (absolute path with no '.', '..', "
                   "extra slashes or symlinks)!");
        ::exit(-7);
    }

    if (nofuse_root_abs[1] == '\0') {
        AZLogError("[NOFUSE] __nofuse_init: AZNFSC_NOFUSE_ROOT cannot be /");
        ::exit(-8);
    }


    // Set default values for config variables not set using the above.
    aznfsc_cfg.set_defaults_and_sanitize();

    aznfsc_cfg.mountpoint = nofuse_root_abs;
    ::free(nofuse_root_abs);

    /*
     * Initialize nfs_client singleton.
     * This creates the libnfs polling thread(s).
     */
    if (!nfs_client::get_instance().init()) {
        AZLogError("[NOFUSE] Failed to init the NFS client");
        ::exit(-9);
    }

    // Store nfs_client ref for easy access later.
    PXT::client = &(nfs_client::get_instance());

    const int ret = ::atexit(__nofuse_cleanup);
    if (ret != 0) {
        AZLogError("[NOFUSE] Failed to set exit handler: {}", ret);
        ::exit(-10);
    }

    AZLogInfo("[NOFUSE] Aznfsclient nofuse driver ready!");

    init_done = true;
}

/**
 * Does pathname exactly match AZNFSC_NOFUSE_ROOT?
 */
static inline bool is_root_dir(const char *pathname)
{
    return (aznfsc_cfg.mountpoint == pathname);
}

/**
 * Does pathname start with '/'?
 */
static inline bool is_absolute_path(const char *pathname)
{
    return pathname[0] == '/';
}

/**
 * Does pathname have '.', '..', duplicate slashes?
 */
static inline bool is_lexically_normal(const char *pathname)
{
    return (::strcmp(std::filesystem::path(pathname).lexically_normal().c_str(),
                     pathname) == 0);
}

/**
 * Returns true if the given pathname is inside the directory
 * AZNFSC_NOFUSE_ROOT.
 *
 * Note: This MUST NOT make any filesystem call as that may result in an
 *       infnite recursion.
 */
bool posix_task::path_in_mountpoint(const char *pathname) const
{
    /*
     * When running in gdb this may be called before __nofuse_init() is called,
     * return false.
     */
    if (aznfsc_cfg.mountpoint.empty()) {
        AZLogDebug("[NOFUSE] path_in_mountpoint({}) called before init",
                   pathname);
        return false;
    }

    // mountpoint MUST be set to an absolute path.
    assert(is_absolute_path(aznfsc_cfg.mountpoint.c_str()));

    /*
     * TODO: For now we support only absolute and lexically normal paths.
     */
    assert(pathname);
    if (!is_absolute_path(pathname)) {
        AZLogDebug("[NOFUSE] path_in_mountpoint({}) called for relative path",
                   pathname);
        return false;
    }

    assert(is_lexically_normal(pathname));

    static int mplen = ::strlen(aznfsc_cfg.mountpoint.c_str());
    assert(mplen > 1);

    const bool in_mp = (::strncmp(pathname, aznfsc_cfg.mountpoint.c_str(),
                                  mplen) == 0);

    AZLogDebug("[NOFUSE] Path {} is {}under mountpoint {}",
               pathname, in_mp ? "" : "NOT ", aznfsc_cfg.mountpoint);
    return in_mp;
}

bool posix_task::fd_in_mountpoint(int fd) const
{
    std::unique_lock<std::mutex> lock(fdmap_mutex);
    const bool in_mp = fdmap.find(fd) != fdmap.end();

    AZLogDebug("[NOFUSE] fd {} is {}under mountpoint {}",
               fd, in_mp ? "" : "NOT ", aznfsc_cfg.mountpoint);
    return in_mp;
}

void posix_task::fd_add_to_map(int fd, fuse_ino_t ino)
{
    std::unique_lock<std::mutex> lock(fdmap_mutex);
    [[maybe_unused]] auto p = fdmap.try_emplace(fd, fd, ino);
    assert(p.second == true);
    AZLogDebug("fd [{}] -> ino [{}]", fd, ino);
}

void posix_task::fd_remove_from_map(int fd)
{
    std::unique_lock<std::mutex> lock(fdmap_mutex);
    [[maybe_unused]] const int num_erased = fdmap.erase(fd);
    assert(num_erased == 1);
    AZLogDebug("Removed fd [{}]", fd);
}

void posix_task::fd_add_pos(int fd, off_t pos)
{
    std::unique_lock<std::mutex> lock(fdmap_mutex);
    auto it = fdmap.find(fd);
    assert(it != fdmap.end());
    AZLogDebug("fd [{}] pos {} -> {}",
               fd, it->second.pos.load(), it->second.pos.load() + pos);
    it->second.pos += pos;
}

int posix_task::path_to_ino(const char *pathname,
                            bool follow_symlink,
                            fuse_ino_t& ino)
{
    int ret;

    // TODO: Add support for symlink resolution.
    assert(!follow_symlink);

    AZLogDebug("[NOFUSE] path_to_ino(pathname={}, follow_symlink={})",
                pathname, follow_symlink);

    assert(is_absolute_path(pathname));
    assert(is_lexically_normal(pathname));

#if 0
    // path_to_ino() MUST be called only for paths inside the mountpoint.
    assert(path_in_mountpoint(pathname));
#endif

    if (is_root_dir(pathname)) {
        ino = FUSE_ROOT_ID;
        AZLogDebug("[NOFUSE] pathname={} is root", pathname);
        return 0;
    }

    std::filesystem::path dirname =
        std::filesystem::path(pathname).parent_path();
    std::filesystem::path filename =
        std::filesystem::path(pathname).filename();

    AZLogDebug("[NOFUSE] path_to_ino: pathname={}, dirname={}, filename={}",
                pathname, dirname.c_str(), filename.c_str());

    /*
     * If parent_ino not already set for this posix_task, set it now.
     */
    fuse_ino_t parent_ino;

    ret = path_to_ino(dirname.c_str(), false, parent_ino);
    if (ret != 0) {
        AZLogError("[NOFUSE] path_to_ino: pathname={}, failed",
                dirname.c_str());
        return ret;
    }

    /*
     * Issue a LOOKUP call to find the inode of filename inside its parent.
     */
    ret = lookup_ll(parent_ino, filename.c_str(), follow_symlink, ino);
    if (ret != 0) {
        assert(errno > 0);
        return -errno;
    }

    AZLogDebug("[NOFUSE] path_to_ino: pathname={}, returning ino={}",
              pathname, ino);

    return 0;
}

int posix_task::fd_to_ino(int fd, fuse_ino_t& _ino, off_t *offset)
{
    /*
     * If already converted and cached, use that.
     */
    if (ino != 0) {
        _ino = ino;
        goto done;
    }

    {
        std::unique_lock<std::mutex> lock(fdmap_mutex);

        auto it = fdmap.find(fd);
        if (it == fdmap.end()) {
            return -ENOENT;
        }

        _ino = ino = it->second.ino;
        if (offset) {
            *offset = it->second.pos;
        }
    }

done:
#ifdef ENABLE_PARANOID
    // This will check the magic.
    assert(client->get_nfs_inode_from_ino(ino) != nullptr);
#endif
    return 0;
}

int posix_task::lookup_ll(fuse_ino_t parent_ino, const char *filename,
                      bool follow_symlink, fuse_ino_t& ino)
{
    AZLogDebug("[NOFUSE] lookup_ll: parent_ino={}, filename={}",
                parent_ino, filename);
    callback_called = false;
    aznfsc_ll_lookup(_PXT2FR(this), parent_ino, filename);

    wait();

    ino = lookup.ino;

    AZLogDebug("[NOFUSE] lookup_ll: parent_ino={}, filename={}, returning res={}, ino={}",
                parent_ino, filename, res, ino);

    if (res < 0) {
        errno = -res;
        return -1;
    }

    assert(res == 0);
    return res;
}

ssize_t posix_task::readlink_ll(fuse_ino_t ino)
{
    callback_called = false;
    aznfsc_ll_readlink(_PXT2FR(this), ino);

    wait();

    AZLogDebug("[NOFUSE] readlink_ll: ino={}, buf={}, bufsiz={} res={}",
                ino, readlink.buf, readlink.bufsiz, res);

    if (res < 0) {
        errno = -res;
        return -1;
    }

    assert(res > 0);
    return res;
}

ssize_t posix_task::getattr_ll(fuse_ino_t ino)
{
    callback_called = false;
    aznfsc_ll_getattr(_PXT2FR(this), ino, nullptr);

    wait();

    AZLogDebug("[NOFUSE] getattr_ll: ino={}, statbuf={}, res={}",
                ino, fmt::ptr(getattr.statbuf), res);

    if (res < 0) {
        errno = -res;
        return -1;
    }

    assert(res == 0);
    return res;
}

ssize_t posix_task::read_ll(fuse_ino_t ino, off_t offset, size_t count)
{
    AZLogDebug("[NOFUSE] read_ll: ino={}, offset={}, count={}",
                ino, offset, count);

    callback_called = false;
    aznfsc_ll_read(_PXT2FR(this), ino, count, offset, nullptr);

    wait();

    if (res < 0) {
        errno = -res;
        return -1;
    }

    assert(res >= 0);
    return res;
}

} /* namespace aznfsc */

/*
 * Following are the various fuse_reply*() functions which are re-purposed
 * for nofuse. This helps us use the exact same low level code for both fuse
 * and nofuse.
 */

int rpc_task::fuse_reply_err(fuse_req_t req, int err)
{
    AZLogDebug("[NOFUSE] fuse_reply_err(req={}, err={})", fmt::ptr(req), err);

    PXT *pxtask = _FR2PXT(req);
    pxtask->res = -err;

    pxtask->wakeup();
    return 0;
}

int rpc_task::fuse_reply_entry(fuse_req_t req, const struct fuse_entry_param *e)
{
    AZLogDebug("[NOFUSE] fuse_reply_entry(req={}, e={})",
               fmt::ptr(req), fmt::ptr(e));

    PXT *pxtask = _FR2PXT(req);

    /*
     * ino=0 is a special case where the entry was not found but we are
     * returning success for -ve cacheing purpose.
     */
    if (e->ino == 0) {
        pxtask->res = -ENOENT;
    } else {
        pxtask->res = 0;
        pxtask->lookup.ino = e->ino;
        pxtask->lookup.generation = e->generation;
        if (pxtask->lookup.attr) {
            *(pxtask->lookup.attr) = e->attr;
        }
    }

    pxtask->wakeup();
    return 0;
}

int rpc_task::fuse_reply_readlink(fuse_req_t req, const char *linkname)
{
    AZLogDebug("[NOFUSE] fuse_reply_readlink(req={}, linkname={})",
               fmt::ptr(req), linkname);

    PXT *pxtask = _FR2PXT(req);

    assert(pxtask->readlink.buf != nullptr);
    assert(pxtask->readlink.bufsiz != 0);

    const size_t copylen = ::strnlen(linkname, pxtask->readlink.bufsiz);

    /*
     * readlink(3) manpage says this about the return:
     * On success, these calls return the number of bytes placed in buf.
     * (If  the  returned value equals bufsiz, then truncation may have
     * occurred.)  On error, -1 is returned and errno is set to indicate
     * the error.
     * Also,
     * readlink() does not append a null byte to buf.
     */
    ::strncpy(pxtask->readlink.buf, linkname, copylen);
    pxtask->readlink.bufsiz = copylen;

    pxtask->res = pxtask->readlink.bufsiz;

    pxtask->wakeup();
    return 0;
}

int rpc_task::fuse_reply_attr(fuse_req_t req,
                              const struct stat *attr,
                              double attr_timeout)
{
    AZLogDebug("[NOFUSE] fuse_reply_attr(req={}, attr_timeout={})",
               fmt::ptr(req), attr_timeout);

    PXT *pxtask = _FR2PXT(req);

    if (pxtask->getattr.statbuf) {
        *(pxtask->getattr.statbuf) = *attr;
    }

    pxtask->res = 0;
    pxtask->wakeup();
    return 0;
}

int rpc_task::fuse_reply_iov(fuse_req_t req, const struct iovec *iov, int count)
{
    AZLogDebug("[NOFUSE] fuse_reply_iov(req={}, count={})",
               fmt::ptr(req), count);

    assert((iov == nullptr) == (count == 0));

    PXT *pxtask = _FR2PXT(req);
    off_t copied = 0;

    /*
     * fuse_reply_iov() is called with iov=nullptr on eof.
     */
    if (iov == nullptr) {
        goto done;
    }

    /*
     * Copy data from iov into the caller supplied buffer.
     */
    for (int i = 0; i < count; i++) {
        ::memcpy((uint8_t *) pxtask->read.buf + copied,
                 iov[i].iov_base,
                 iov[i].iov_len);
        copied += iov[i].iov_len;
    }

    /*
     * Update the file position.
     */
    pxtask->fd_add_pos(pxtask->read.fd, copied);

done:
    pxtask->res = copied;
    assert(pxtask->res >= 0);
    pxtask->wakeup();
    return 0;
}

/*
 * TODO: Make this return proper values.
 */
const struct fuse_ctx *rpc_task::fuse_req_ctx(fuse_req_t req)
{
    [[maybe_unused]] PXT *pxtask = _FR2PXT(req);
    static struct fuse_ctx ctx = {0, 0, 100, 0};
    return &ctx;
}

extern "C" {

#define CHECK_AND_CALL_ORIG_FUNC_FOR_PATHNAME(pathname, func, force, retonfail, ...) \
do { \
    if (!init_done || in_cleanup || force || !pxtask.path_in_mountpoint(pathname)) { \
        /* \
         * If pathname is not in mountpoint then call the original function. \
         */ \
        if (!PXT::__##func##_orig) { \
            PXT::__##func##_orig = \
                (decltype(PXT::__##func##_orig)) ::dlsym(RTLD_NEXT, #func); \
            if (!PXT::__##func##_orig) { \
                AZLogError("[NOFUSE] dlsym({}) failed: {}", \
                           #func, ::dlerror()); \
                errno = ENOENT; \
                return retonfail; \
            } \
        } \
        AZLogDebug("[NOFUSE] {}={}", #func, fmt::ptr(PXT::__##func##_orig)); \
        return PXT::__##func##_orig(__VA_ARGS__); \
    } \
} while (0)

#define CHECK_AND_CALL_ORIG_FUNC_FOR_FD(fd, func, force, ...) \
do { \
    if (!init_done || in_cleanup || force || !pxtask.fd_in_mountpoint(fd)) { \
        /* \
         * If fd is not in mountpoint then call the original function. \
         */ \
        if (!PXT::__##func##_orig) { \
            PXT::__##func##_orig = \
                (decltype(PXT::__##func##_orig)) ::dlsym(RTLD_NEXT, #func); \
            if (!PXT::__##func##_orig) { \
                AZLogError("[NOFUSE] dlsym({}) failed: {}", \
                           #func, ::dlerror()); \
                errno = ENOENT; \
                return -1; \
            } \
        } \
        AZLogDebug("[NOFUSE] {}={}", #func, fmt::ptr(PXT::__##func##_orig)); \
        return PXT::__##func##_orig(__VA_ARGS__); \
    } \
} while (0)

#define PATH_TO_INO(pathname) \
({ \
    fuse_ino_t ino; \
    const int ret = pxtask.path_to_ino(pathname, \
                                       false /* follow_symlink */, \
                                       ino); \
    if (ret != 0) { \
        assert(ret > 0); \
        AZLogError("[NOFUSE] {}: path_to_ino({}) failed, setting errno={}", \
                   __FUNCTION__, pathname, -ret); \
        errno = -ret; \
        return -1; \
    } \
    ino; \
})

#define FD_TO_INO(fd, off) \
({ \
    fuse_ino_t ino; \
    const int ret = pxtask.fd_to_ino(fd, ino, off); \
    if (ret != 0) { \
        assert(ret > 0); \
        AZLogError("[NOFUSE] {}: fd_to_ino({}) failed, setting errno={}", \
                   __FUNCTION__, fd, -ret); \
        errno = -ret; \
        return -1; \
    } \
    ino; \
})

int __xstat(int ver, const char *pathname, struct stat *statbuf)
{
    AZLogDebug("[NOFUSE] INTERCEPT: __xstat(ver={}, pathname={})",
               ver, pathname);

    const bool force = (!pathname);

    /*
     * POSIX task for tracking this request.
     */
    PXT pxtask;

    CHECK_AND_CALL_ORIG_FUNC_FOR_PATHNAME(pathname,
                                          __xstat,
                                          force,
                                          -1,
                                          ver, pathname, statbuf);

    /*
     * Path is inside mountpoint, need to make the aznfsc_ll_getattr call.
     * That needs the inode number to identify the target.
     */
    const fuse_ino_t ino = PATH_TO_INO(pathname);

    pxtask.getattr.statbuf = statbuf;

    return pxtask.getattr_ll(ino);
}

int __fxstat(int ver, int fd, struct stat *statbuf)
{
    AZLogDebug("[NOFUSE] INTERCEPT: __xfstat(ver={}, fd={})",
               ver, fd);

    const bool force = false;

    /*
     * POSIX task for tracking this request.
     */
    PXT pxtask;

    CHECK_AND_CALL_ORIG_FUNC_FOR_FD(fd,
                                    __fxstat,
                                    force,
                                    ver, fd, statbuf);

    /*
     * Path is inside mountpoint, need to make the aznfsc_ll_getattr call.
     * That needs the inode number to identify the target.
     */
    const fuse_ino_t ino = FD_TO_INO(fd, nullptr);

    pxtask.getattr.statbuf = statbuf;

    return pxtask.getattr_ll(ino);
}

ssize_t readlink(const char *pathname, char *buf, size_t bufsiz)
{
    AZLogDebug("[NOFUSE] INTERCEPT: readlink(pathname={}, buf={}, bufsiz={})",
               pathname, fmt::ptr(buf), bufsiz);

    const bool force = (!pathname || ((int) bufsiz <= 0));

    /*
     * POSIX task for tracking this request.
     */
    PXT pxtask;

    CHECK_AND_CALL_ORIG_FUNC_FOR_PATHNAME(pathname,
                                          readlink,
                                          force,
                                          -1,
                                          pathname, buf, bufsiz);

    /*
     * Path is inside mountpoint, need to make the aznfsc_ll_readlink call.
     * That needs the inode number to identify the symlink.
     */
    const fuse_ino_t ino = PATH_TO_INO(pathname);

    pxtask.readlink.buf = buf;
    pxtask.readlink.bufsiz = bufsiz;

    return pxtask.readlink_ll(ino);
}

int open(const char *pathname, int flags, ...)
{
    mode_t mode = 0;

    if ((flags & O_CREAT) != 0 || (flags & O_TMPFILE) != 0) {
        va_list args;
        va_start(args, flags);
        mode = va_arg(args, mode_t);
        va_end(args);
    }

    AZLogDebug("[NOFUSE] INTERCEPT: open(pathname={}, flags=0x{:x}, mode=0{:03o})",
               pathname, flags, mode);

    const bool force = (!pathname);

    /*
     * POSIX task for tracking this request.
     */
    PXT pxtask;

    CHECK_AND_CALL_ORIG_FUNC_FOR_PATHNAME(pathname,
                                          open,
                                          force,
                                          -1,
                                          pathname, flags, mode);

    const fuse_ino_t ino = PATH_TO_INO(pathname);
    const int fd = fdinfo::get_next_fd();

    pxtask.fd_add_to_map(fd, ino);

    assert(fd > 0);
    return fd;
}

int openat(int dirfd, const char *pathname, int flags, ...)
{
    mode_t mode = 0;

    if ((flags & O_CREAT) != 0 || (flags & O_TMPFILE) != 0) {
        va_list args;
        va_start(args, flags);
        mode = va_arg(args, mode_t);
        va_end(args);
    }

    AZLogDebug("[NOFUSE] INTERCEPT: openat(dirfd={}, pathname={}, flags={}, mode={})",
               dirfd, pathname, flags, mode);

    // XXX Not implemented.

    errno = ENOENT;
    return -1;
}

FILE *fopen(const char *pathname, const char *mode)
{
    AZLogWarn("[NOFUSE] INTERCEPT: fopen(pathname={}, mode={})",
              pathname, mode);
    AZLogWarn("[NOFUSE] fopen not supported, bypassing! If this is your "
              "target file it won't work!");

    /*
     * TODO: We don't support fopen/fclose/fread/fwrite.
     *       Bypass every call.
     */
    /*
     * POSIX task for tracking this request.
     */
    PXT pxtask;

    CHECK_AND_CALL_ORIG_FUNC_FOR_PATHNAME(pathname,
                                          fopen,
                                          true,
                                          NULL,
                                          pathname, mode);
    assert(0);
    return NULL;
}

int close(int fd)
{
    AZLogDebug("[NOFUSE] INTERCEPT: close(fd={})", fd);

    PXT pxtask;

    CHECK_AND_CALL_ORIG_FUNC_FOR_FD(fd, close, false, fd);

    pxtask.fd_remove_from_map(fd);

    return 0;
}

int dup(int oldfd)
{
    AZLogDebug("[NOFUSE] INTERCEPT: dup(oldfd={})", oldfd);

    PXT pxtask;

    CHECK_AND_CALL_ORIG_FUNC_FOR_FD(oldfd, dup, false, oldfd);

    /*
     * TODO: Right now we do a basic dup where we make the newfd also point
     *       to the same ino, but both fds don't share the file pos.
     *       This will work only for cases where application closes the oldfd
     *       after the dup, which is the more common case.
     */
    const fuse_ino_t ino = FD_TO_INO(oldfd, nullptr);
    const int newfd = fdinfo::get_next_fd();

    pxtask.fd_add_to_map(newfd, ino);
    return newfd;
}

int dup2(int oldfd, int newfd)
{
    AZLogDebug("[NOFUSE] INTERCEPT: dup2(oldfd={}, newfd={})",
               oldfd, newfd);
    PXT pxtask;

    CHECK_AND_CALL_ORIG_FUNC_FOR_FD(oldfd, dup2, false, oldfd, newfd);

    const fuse_ino_t ino = FD_TO_INO(oldfd, nullptr);

    pxtask.fd_add_to_map(newfd, ino);
    return newfd;
}

ssize_t read(int fd, void *buf, size_t count)
{
    AZLogDebug("[NOFUSE] INTERCEPT: read(fd={}, buf={}, count={})",
               fd, fmt::ptr(buf), count);

    PXT pxtask;
    const bool force = aznfsc_cfg.mountpoint.empty();

    CHECK_AND_CALL_ORIG_FUNC_FOR_FD(fd, read, force, fd, buf, count);

    off_t offset;
    const fuse_ino_t ino = FD_TO_INO(fd, &offset);

    pxtask.read.fd = fd;
    pxtask.read.buf = buf;
    pxtask.read.count = count;

    return pxtask.read_ll(ino, offset, count);
}

}
