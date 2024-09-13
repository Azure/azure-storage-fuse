#ifndef __NO_FUSE_H__
#define __NO_FUSE_H__

#ifndef ENABLE_NO_FUSE
#error "nofuse.h must be included only when ENABLE_NO_FUSE is defined"
#endif

#include <atomic>
#include <shared_mutex>
#include <unordered_map>
#include <filesystem>

#include <stdint.h>
#include <unistd.h>
#include <sys/types.h>
#include <sys/stat.h>
#include <fcntl.h>

struct nfs_client;

namespace aznfsc {

/**
 * Fuse compat definitions copied from libfuse.
 * Most of these are needed to let the aznfsclient code compile w/o needing
 * many ugly ifdefs, while we may use some of these in nofuse code.
 *
 * Note: We use fuse_ino_t to represent the inode number of a file/dir even in
 *       nofuse mode. This is because all the lower level code uses fuse_ino_t
 *       to identify files/dirs and we want to use that code as-is.
 *       Obviously we don't use fuse_req since that's an internal/opaque data
 *       structure that fuse uses. Wherever fuse_req_t is being passed we pass
 *       a pointer to posix_task, as a posix_task tracks an application request
 *       in nofuse mode much like how fuse_req_t tracks in fuse mode.
 *       Use _FR2PXT() and _PXT2FR() helpers for safely converting between the
 *       two.
 */
#define AZNFSC_FUSE_COMPAT
#ifdef AZNFSC_FUSE_COMPAT

/** The node ID of the root inode */
#define FUSE_ROOT_ID 1

/** Inode number type */
typedef uint64_t fuse_ino_t;

/*
 * TODO: See if we want to carry something inside this for nofuse.
 */
struct fuse_req {
};

/** Request pointer type */
typedef struct fuse_req *fuse_req_t;

enum fuse_opcode {
    FUSE_LOOKUP         = 1,
    FUSE_FORGET         = 2,  /* no reply */
    FUSE_GETATTR        = 3,
    FUSE_SETATTR        = 4,
    FUSE_READLINK       = 5,
    FUSE_SYMLINK        = 6,
    FUSE_MKNOD          = 8,
    FUSE_MKDIR          = 9,
    FUSE_UNLINK         = 10,
    FUSE_RMDIR          = 11,
    FUSE_RENAME         = 12,
    FUSE_LINK           = 13,
    FUSE_OPEN           = 14,
    FUSE_READ           = 15,
    FUSE_WRITE          = 16,
    FUSE_STATFS         = 17,
    FUSE_RELEASE        = 18,
    FUSE_FSYNC          = 20,
    FUSE_SETXATTR       = 21,
    FUSE_GETXATTR       = 22,
    FUSE_LISTXATTR      = 23,
    FUSE_REMOVEXATTR    = 24,
    FUSE_FLUSH          = 25,
    FUSE_INIT           = 26,
    FUSE_OPENDIR        = 27,
    FUSE_READDIR        = 28,
    FUSE_RELEASEDIR     = 29,
    FUSE_FSYNCDIR       = 30,
    FUSE_GETLK          = 31,
    FUSE_SETLK          = 32,
    FUSE_SETLKW         = 33,
    FUSE_ACCESS         = 34,
    FUSE_CREATE         = 35,
    FUSE_INTERRUPT      = 36,
    FUSE_BMAP           = 37,
    FUSE_DESTROY        = 38,
    FUSE_IOCTL          = 39,
    FUSE_POLL           = 40,
    FUSE_NOTIFY_REPLY   = 41,
    FUSE_BATCH_FORGET   = 42,
    FUSE_FALLOCATE      = 43,
    FUSE_READDIRPLUS    = 44,
    FUSE_RENAME2        = 45,
    FUSE_LSEEK          = 46,
    FUSE_COPY_FILE_RANGE= 47,
    FUSE_SETUPMAPPING   = 48,
    FUSE_REMOVEMAPPING  = 49,
    FUSE_SYNCFS         = 50,
    FUSE_TMPFILE        = 51,
    FUSE_STATX          = 52,

    /* CUSE specific operations */
    CUSE_INIT           = 4096,

    /* Reserved opcodes: helpful to detect structure endian-ness */
    CUSE_INIT_BSWAP_RESERVED    = 1048576,  /* CUSE_INIT << 8 */
    FUSE_INIT_BSWAP_RESERVED    = 436207616,    /* FUSE_INIT << 24 */
};

/* 'to_set' flags in setattr */
#define FUSE_SET_ATTR_MODE  (1 << 0)
#define FUSE_SET_ATTR_UID   (1 << 1)
#define FUSE_SET_ATTR_GID   (1 << 2)
#define FUSE_SET_ATTR_SIZE  (1 << 3)
#define FUSE_SET_ATTR_ATIME (1 << 4)
#define FUSE_SET_ATTR_MTIME (1 << 5)
#define FUSE_SET_ATTR_ATIME_NOW (1 << 7)
#define FUSE_SET_ATTR_MTIME_NOW (1 << 8)
#define FUSE_SET_ATTR_FORCE (1 << 9)
#define FUSE_SET_ATTR_CTIME (1 << 10)
#define FUSE_SET_ATTR_KILL_SUID (1 << 11)
#define FUSE_SET_ATTR_KILL_SGID (1 << 12)
#define FUSE_SET_ATTR_FILE  (1 << 13)
#define FUSE_SET_ATTR_KILL_PRIV (1 << 14)
#define FUSE_SET_ATTR_OPEN  (1 << 15)
#define FUSE_SET_ATTR_TIMES_SET (1 << 16)
#define FUSE_SET_ATTR_TOUCH (1 << 17)


/**
 * Information about an open file.
 *
 * File Handles are created by the open, opendir, and create methods and closed
 * by the release and releasedir methods.  Multiple file handles may be
 * concurrently open for the same file.  Generally, a client will create one
 * file handle per file descriptor, though in some cases multiple file
 * descriptors can share a single file handle.
 */
struct fuse_file_info {
    /** Open flags.  Available in open(), release() and create() */
    int flags;

    /** In case of a write operation indicates if this was caused
        by a delayed write from the page cache. If so, then the
        context's pid, uid, and gid fields will not be valid, and
        the *fh* value may not match the *fh* value that would
        have been sent with the corresponding individual write
        requests if write caching had been disabled. */
    unsigned int writepage : 1;

    /** Can be filled in by open/create, to use direct I/O on this file. */
    unsigned int direct_io : 1;

    /** Can be filled in by open and opendir. It signals the kernel that any
        currently cached data (ie., data that the filesystem provided the
        last time the file/directory was open) need not be invalidated when
        the file/directory is closed. */
    unsigned int keep_cache : 1;

    /** Can be filled by open/create, to allow parallel direct writes on this
         *  file */
    unsigned int parallel_direct_writes : 1;

    /** Indicates a flush operation.  Set in flush operation, also
        maybe set in highlevel lock operation and lowlevel release
        operation. */
    unsigned int flush : 1;

    /** Can be filled in by open, to indicate that the file is not
        seekable. */
    unsigned int nonseekable : 1;

    /* Indicates that flock locks for this file should be
       released.  If set, lock_owner shall contain a valid value.
       May only be set in ->release(). */
    unsigned int flock_release : 1;

    /** Can be filled in by opendir. It signals the kernel to
        enable caching of entries returned by readdir().  Has no
        effect when set in other contexts (in particular it does
        nothing when set by open()). */
    unsigned int cache_readdir : 1;

    /** Can be filled in by open, to indicate that flush is not needed
        on close. */
    unsigned int noflush : 1;

    /** Padding.  Reserved for future use*/
    unsigned int padding : 23;
    unsigned int padding2 : 32;

    /** File handle id.  May be filled in by filesystem in create,
     * open, and opendir().  Available in most other file operations on the
     * same file handle. */
    uint64_t fh;

    /** Lock owner id.  Available in locking operations and flush */
    uint64_t lock_owner;

    /** Requested poll events.  Available in ->poll.  Only set on kernels
        which support it.  If unsupported, this field is set to zero. */
    uint32_t poll_events;
};


/**
 * Buffer copy flags
 */
enum fuse_buf_copy_flags {
    /**
     * Don't use splice(2)
     *
     * Always fall back to using read and write instead of
     * splice(2) to copy data from one file descriptor to another.
     *
     * If this flag is not set, then only fall back if splice is
     * unavailable.
     */
    FUSE_BUF_NO_SPLICE  = (1 << 1),

    /**
     * Force splice
     *
     * Always use splice(2) to copy data from one file descriptor
     * to another.  If splice is not available, return -EINVAL.
     */
    FUSE_BUF_FORCE_SPLICE   = (1 << 2),

    /**
     * Try to move data with splice.
     *
     * If splice is used, try to move pages from the source to the
     * destination instead of copying.  See documentation of
     * SPLICE_F_MOVE in splice(2) man page.
     */
    FUSE_BUF_SPLICE_MOVE    = (1 << 3),

    /**
     * Don't block on the pipe when copying data with splice
     *
     * Makes the operations on the pipe non-blocking (if the pipe
     * is full or empty).  See SPLICE_F_NONBLOCK in the splice(2)
     * man page.
     */
    FUSE_BUF_SPLICE_NONBLOCK= (1 << 4)
};

/** Directory entry parameters supplied to fuse_reply_entry() */
struct fuse_entry_param {
    /** Unique inode number
     *
     * In lookup, zero means negative entry (from version 2.5)
     * Returning ENOENT also means negative entry, but by setting zero
     * ino the kernel may cache negative entries for entry_timeout
     * seconds.
     */
    fuse_ino_t ino;

    /** Generation number for this entry.
     *
     * If the file system will be exported over NFS, the
     * ino/generation pairs need to be unique over the file
     * system's lifetime (rather than just the mount time). So if
     * the file system reuses an inode after it has been deleted,
     * it must assign a new, previously unused generation number
     * to the inode at the same time.
     *
     */
    uint64_t generation;

    /** Inode attributes.
     *
     * Even if attr_timeout == 0, attr must be correct. For example,
     * for open(), FUSE uses attr.st_size from lookup() to determine
     * how many bytes to request. If this value is not correct,
     * incorrect data will be returned.
     */
    struct stat attr;

    /** Validity timeout (in seconds) for inode attributes. If
        attributes only change as a result of requests that come
        through the kernel, this should be set to a very large
        value. */
    double attr_timeout;

    /** Validity timeout (in seconds) for the name. If directory
        entries are changed/deleted only as a result of requests
        that come through the kernel, this should be set to a very
        large value. */
    double entry_timeout;
};

/**
 * Buffer flags
 */
enum fuse_buf_flags {
    /**
     * Buffer contains a file descriptor
     *
     * If this flag is set, the .fd field is valid, otherwise the
     * .mem fields is valid.
     */
    FUSE_BUF_IS_FD      = (1 << 1),

    /**
     * Seek on the file descriptor
     *
     * If this flag is set then the .pos field is valid and is
     * used to seek to the given offset before performing
     * operation on file descriptor.
     */
    FUSE_BUF_FD_SEEK    = (1 << 2),

    /**
     * Retry operation on file descriptor
     *
     * If this flag is set then retry operation on file descriptor
     * until .size bytes have been copied or an error or EOF is
     * detected.
     */
    FUSE_BUF_FD_RETRY   = (1 << 3)
};

/**
 * Single data buffer
 *
 * Generic data buffer for I/O, extended attributes, etc...  Data may
 * be supplied as a memory pointer or as a file descriptor
 */
struct fuse_buf {
    /**
     * Size of data in bytes
     */
    size_t size;

    /**
     * Buffer flags
     */
    enum fuse_buf_flags flags;

    /**
     * Memory pointer
     *
     * Used unless FUSE_BUF_IS_FD flag is set.
     */
    void *mem;

    /**
     * File descriptor
     *
     * Used if FUSE_BUF_IS_FD flag is set.
     */
    int fd;

    /**
     * File position
     *
     * Used if FUSE_BUF_FD_SEEK flag is set.
     */
    off_t pos;
};

/**
 * Data buffer vector
 *
 * An array of data buffers, each containing a memory pointer or a
 * file descriptor.
 *
 * Allocate dynamically to add more than one buffer.
 */
struct fuse_bufvec {
    /**
     * Number of buffers in the array
     */
    size_t count;

    /**
     * Index of current buffer within the array
     */
    size_t idx;

    /**
     * Current offset within the current buffer
     */
    size_t off;

    /**
     * Array of buffers
     */
    struct fuse_buf buf[1];
};
#endif /* AZNFSC_FUSE_COMPAT */

/**
 * Information tracked for each fd.
 * We need to track the corresponding fuse_ino_t that we can use for making
 * aznfsc_ll_*() calls. Another imp thing that we need to track is the current
 * position in the file.
 *
 * TODO: Need to support dup/dup2 to have multiple fds refer to the same file,
 *       and thus share the offset. This we should be able to do by
 *       intercepting the dup/dup2 functions.
 */
struct fdinfo
{
    fdinfo(int _fd, fuse_ino_t _ino) :
        fd(_fd),
        ino(_ino)
    {
        assert(fd > 2);
        assert(ino != 0);
    }

    // POSIX fd.
    int fd = -1;

    // Corresponding inode, for passing to aznfsc_ll_* APIs.
    fuse_ino_t ino = 0;

    // Current position within the file where next read/write will be done.
    std::atomic<off_t> pos = 0;

    /*
     * Get next free fd for returning from open().
     *
     * TODO: Currently we always return a constantly increasing fd and we
     *       don't reuse closed fds.
     */
    static int get_next_fd()
    {
        static std::atomic<int> gfd = 3;
        return gfd++;
    }
};

/**
 * Every POSIX API is run in the context of a posix_task.
 * It tracks the progress of the POSIX API, including communication and
 * synchronization between issuer and callback thread.
 */
#define POSIX_TASK_MAGIC *((const uint32_t *)"PSXT")
typedef struct posix_task
{
    const uint32_t magic = POSIX_TASK_MAGIC;

    /**
     * Called by issuer thread to wait on cv after making the aznfsc_ll_* call.
     * See wakeup().
     */
    void wait() const
    {
        std::unique_lock<std::mutex> lock(mutex);
#if 0
        /*
         * We cannot assert this as the aznfsc_ll_*() callback may get called
         * before we get to call wait(). In that case wait() can quickly return.
         */
        assert(!callback_called);
#endif
        cv.wait(lock, [this] { return (this->callback_called == true); });
    }

    /**
     * Called by completion/callback thread to wake up the issuer waiting on
     * cv, after the request is completed by the NFS server.
     * See wait().
     */
    void wakeup()
    {
        {
            std::unique_lock<std::mutex> lock(mutex);
            assert(!callback_called);
            callback_called = true;
        }
        cv.notify_one();
    }

    /**
     * Does the given path/fd lie inside AZNFSC_NOFUSE_ROOT?
     * For paths/fds that lie inside AZNFSC_NOFUSE_ROOT we intercept and forward
     * the requests to the NFS server, while for paths/fds not lying inside the
     * mountpoint the call is forwarded to the original libc function.
     */
    bool path_in_mountpoint(const char *pathname) const;
    bool fd_in_mountpoint(int fd) const;

    /**
     * Given a pathname return the inode number corresponding to that.
     * pathname can be absolute or relative path,if follow_symlink is true
     * and last component of pathname refers to a symbolic link, it follows
     * the symbolic link and returns the inode for the target.
     * On success it returns 0 and returns the inode number in 'ino'.
     * On error it returns -errno.
     *
     * TODO: For now only absolute pathnames with no symlink are supported.
     */
    int path_to_ino(const char *pathname,
                    bool follow_symlink,
                    fuse_ino_t& ino);

    /**
     * Given an fd, return the corresponding fuse_ino_t.
     * 'fd' can have the special value AT_FDCWD to mean cwd (not supported
     * now).
     * On success it returns 0 and returns the inode number in 'ino'.
     * On error it returns -errno.
     */
    int fd_to_ino(int fd, fuse_ino_t& _ino, off_t *offset = nullptr);

    const std::filesystem::path& get_dirname() const
    {
        return dirname;
    }

    const std::filesystem::path& get_filename() const
    {
        return filename;
    }

    /*
     * Add new fd->ino mapping to fdmap.
     */
    void fd_add_to_map(int fd, fuse_ino_t ino);

    /*
     * Add 'pos' to the current fd offset.
     */
    void fd_add_pos(int fd, off_t pos);

    /*
     * For every aznfsc_ll_* call that we make we have a struct to hold the
     * response data and a common 'res' to hold the status. Callback fills the
     * status and response here and notifies the issuer thread by calling
     * wakeup(), which would be waiting on wait().
     *
     * The *_ll() functions have the same return as the corresponding POSIX
     * API, i.e., they return 0/+ve on success and -1 on failure and set errno.
     */

    /*
     * Common result status for all functions.
     * 0/+ve indicates success, else it holds -ve errno.
     */
    ssize_t res = -1;

    int lookup_ll(fuse_ino_t parent_ino, const char *filename,
                  bool follow_symlink, fuse_ino_t& ino);
    struct {
        fuse_ino_t ino;
        uint64_t generation;
        struct stat *attr = nullptr;
    } lookup;

    ssize_t readlink_ll(fuse_ino_t ino);
    struct {
        char *buf = nullptr;
        size_t bufsiz = 0;
    } readlink;

    ssize_t getattr_ll(fuse_ino_t ino);
    struct {
        struct stat *statbuf = nullptr;
    } getattr;

    ssize_t read_ll(fuse_ino_t ino, off_t offset, size_t count);
    struct {
        int fd = -1;
        void *buf = nullptr;
        size_t count = 0;
    } read;

    /*
     * For every glibc call that we hijack we store the pointer to the original
     * function here.
     */

    static ssize_t (*__readlink_orig)(const char *pathname,
                                      char *buf,
                                      size_t bufsiz);
    static int (*____xstat_orig)(int ver,
                                 const char *pathname,
                                 struct stat *statbuf);
    static int (*____fxstat_orig)(int ver,
                                 int fd,
                                 struct stat *statbuf);
    static ssize_t (*__open_orig)(const char *pathname,
                                  int flags,
                                  ...);
    static ssize_t (*__read_orig)(int fd,
                                  void *buf,
                                  size_t count);

    /*
     * nfs_client reference for easy access.
     * Set in the __nofuse_init() constructor.
     */
    static struct nfs_client *client;

private:
    /*
     * This is information identifying the file/dir being referred to by this
     * posix_task. Note that posix_task tracks operation on a filesystem object
     * and these identify the target filesystem object.
     * Any POSIX call identifies the file/dir either by 'fd' or by 'pathname'
     * (for metadata operations). Rest of the members cache some of the
     * intermediate results.
     *
     * fd:          fd passed to the POSIX call. Will not be set for calls
     *              that identify the target file/dir by a pathname.
     * ino:         Will be set if we have resolved the target file/dir inode.
     * parent_ino:  Target file/dir's parent's inode, if resolved.
     * dirname:     Parent directory path, if resolved.
     * filename:    Last component of file/dir, if resolved.
     *
     * Note: Not all of these may be initialized, so check before using.
     * Note: rename(2) is the only call that identifies 2 files/dirs.
     *       For rename(2) these identify the oldpath. newpath details are
     *       stored in the rename data struct.
     */
    int fd = -1;
    std::atomic<fuse_ino_t> ino = 0;
    fuse_ino_t parent_ino = 0;
    std::filesystem::path dirname;
    std::filesystem::path filename;

    /*
     * Set by completion thread to indicate completion (successful or failed)
     * to calling thread.
     */
    std::atomic<bool> callback_called = false;

    /**
     * Conditional variable through which completion thread signals the issuer
     * thread.
     */
    mutable std::condition_variable cv;
    mutable std::mutex mutex;

    /*
     * This is the fd table where we maintain info for each open fd.
     * This is protected by fdmap_mutex.
     */
    static std::unordered_map<int, struct fdinfo> fdmap;
    static std::mutex fdmap_mutex;
} PXT;

static inline
fuse_req_t _PXT2FR(PXT *pxtask)
{
    assert(pxtask != nullptr);
    assert(pxtask->magic == POSIX_TASK_MAGIC);
    return reinterpret_cast<fuse_req_t>(pxtask);
}

static inline
PXT *_FR2PXT(fuse_req_t req)
{
    assert(req != nullptr);
    PXT *pxtask = reinterpret_cast<PXT*>(req);
    assert(pxtask->magic == POSIX_TASK_MAGIC);

    return pxtask;
}

}

#endif /* __NO_FUSE_H__ */
