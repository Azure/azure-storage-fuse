#ifndef __NO_FUSE_H__
#define __NO_FUSE_H__

#ifndef ENABLE_NO_FUSE
#error "nofuse.h must be included only when ENABLE_NO_FUSE is defined"
#endif

#include <stdint.h>

namespace aznfsc {

/*
 * Fuse compat definitions copied from libfuse.
 * Most of these are needed to let existing code compile w/o many ifdefs,
 * while we may use some of these in nofuse code.
 */
#define AZNFSC_FUSE_COMPAT
#ifdef AZNFSC_FUSE_COMPAT

/** The node ID of the root inode */
#define FUSE_ROOT_ID 1

/** Inode number type */
typedef uint64_t fuse_ino_t;

/*
 * TODO: Let's see what we want to carry inside this for nofuse.
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

}

#endif /* __NO_FUSE_H__ */
