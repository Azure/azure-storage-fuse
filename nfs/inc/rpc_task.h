#ifndef __RPC_TASK_H__
#define __RPC_TASK_H__

#include <cstddef>
#include <string>
#include <mutex>
#include <stack>
#include <shared_mutex>
#include <vector>
#include <set>
#include <thread>

#include "nfs_client.h"
#include "file_cache.h"
#include "rpc_stats.h"
#include "log.h"

// Maximum number of simultaneous rpc tasks (sync + async).
#define MAX_OUTSTANDING_RPC_TASKS 65536

// Maximum number of simultaneous async rpc tasks.
#define MAX_ASYNC_RPC_TASKS 1024

/**
 * LOOKUP RPC task definition.
 */
struct lookup_rpc_task
{
    void set_file_name(const char *name)
    {
        file_name = ::strdup(name);
    }

    void set_parent_ino(fuse_ino_t parent)
    {
        parent_ino = parent;
    }

    fuse_ino_t get_parent_ino() const
    {
        return parent_ino;
    }

    const char *get_file_name() const
    {
        return file_name;
    }

    /**
     * Release any resources used up by this task.
     */
    void release()
    {
        ::free(file_name);
    }

private:
    fuse_ino_t parent_ino;
    char *file_name;
};

/**
 * WRITE RPC task definition.
 */
struct write_rpc_task
{
    void set_ino(fuse_ino_t ino)
    {
        file_ino = ino;
    }

    void set_offset(off_t off)
    {
        offset = off;
    }

    void set_size(size_t size)
    {
        /*
         * length is how much this WRITE RPC wants to write and write_count
         * is how much it has written so far. Note that WRITE RPC may write
         * partial data so we need to track write_count.
         */
        length = size;
        write_count = 0;
    }

    void set_count(size_t count)
    {
        // Must not write more than requested.
        assert(count <= length);
        write_count = count;
    }

    void set_buffer_vector(struct fuse_bufvec *bufv)
    {
        write_bufv = bufv;
    }

    fuse_ino_t get_ino() const
    {
        return file_ino;
    }

    off_t get_offset() const
    {
        return offset;
    }

    size_t get_size() const
    {
        return length;
    }

    size_t get_count() const
    {
        return write_count;
    }

    struct fuse_bufvec *get_buffer_vector() const
    {
        return write_bufv;
    }

    /**
     * Release any resources used up by this task.
     */
    void release()
    {
    }

private:
    fuse_ino_t file_ino;
    size_t length;
    size_t write_count;
    off_t  offset;
    struct fuse_bufvec *write_bufv;
};

/**
 * FLUSH RPC task definition.
 */
struct flush_rpc_task
{
    void set_ino(fuse_ino_t ino)
    {
        file_ino = ino;
    }

    fuse_ino_t get_ino() const
    {
        return file_ino;
    }

    /**
     * Release any resources used up by this task.
     */
    void release()
    {
    }

private:
    fuse_ino_t file_ino;
};

/**
 * Write callback context, used by following calls:
 * - Write
 * - Flush
 * - Fsync
 */
struct write_context
{
    void set_count(size_t _count)
    {
        count = _count;
    }

    size_t get_count() const
    {
        return count;
    }

    struct rpc_task *get_task() const
    {
        return task;
    }

    void set_task(struct rpc_task *_task)
    {
        task = _task;
    }

    const struct bytes_chunk& get_bytes_chunk() const
    {
        return bc;
    }

    fuse_ino_t get_ino() const
    {
        return ino;
    }

    /**
     * Release any resources used up by this task.
     */
    void release()
    {
    }

    write_context(const struct bytes_chunk& _bc,
                  rpc_task *_task,
                  fuse_ino_t _ino) :
        bc(_bc),
        task(_task),
        ino(_ino),
        count(0)
      {

      }

private:
    /*
     * Note: We always write the full underlying membuf and not just the
     *       portion represented by bc.
     */
    const struct bytes_chunk bc;
    struct rpc_task *task;
    fuse_ino_t ino;
    size_t count;
};

/**
 * GETATTR RPC task definition.
 */
struct getattr_rpc_task
{
    void set_ino(fuse_ino_t ino)
    {
        this->ino = ino;
    }

    fuse_ino_t get_ino() const
    {
        return ino;
    }

private:
    fuse_ino_t ino;
};


/**
 * SETATTR RPC task definition.
 */
struct setattr_rpc_task
{
    void set_ino(fuse_ino_t ino)
    {
        this->ino = ino;
    }

    void set_fuse_file(fuse_file_info *fileinfo)
    {
        /*
         * fuse can pass this as nullptr.
         * The fuse_file_info pointer passed to the fuse lowlevel API is only
         * valid in the issue path. Since we want to use it after that, we have
         * to make a deep copy of that.
         */
        if (fileinfo != nullptr) {
            file = *fileinfo;
            file_ptr = &file;
        } else {
            file_ptr = nullptr;
        }
    }

    void set_attribute_and_mask(const struct stat *attr, int mask)
    {
        attribute = attr;
        /*
         * We don't make use of FUSE_SET_ATTR_CTIME, ignore it.
         */
        to_set = mask & ~FUSE_SET_ATTR_CTIME;
    }

    const struct stat *get_attr() const
    {
        return attribute;
    }

    int get_attr_flags_to_set() const
    {
        return to_set;
    }

    struct fuse_file_info *get_fuse_file() const
    {
        return file_ptr;
    }

    fuse_ino_t get_ino() const
    {
        return ino;
    }

private:
    // Inode of the file for which attributes have to be set.
    fuse_ino_t ino;

    // File info passed by the fuse layer.
    fuse_file_info file;
    fuse_file_info *file_ptr;

    /*
     * Attributes value to be set to.
     * Note: This is only valid till the issue path call returns.
     *       DO NOT ACCESS THIS IN THE COMPLETION PATH.
     */
    const struct stat *attribute;

    // Valid attribute mask to be set.
    int to_set;
};

struct create_file_rpc_task
{
    fuse_ino_t get_parent_ino() const
    {
        return parent_ino;
    }

    const char *get_file_name() const
    {
        return file_name;
    }

    mode_t get_mode() const
    {
        return mode;
    }

    struct fuse_file_info *get_fuse_file() const
    {
        return file_ptr;
    }

    void set_parent_ino(fuse_ino_t parent)
    {
        parent_ino = parent;
    }

    void set_file_name(const char *name)
    {
        file_name = ::strdup(name);
    }

    void set_mode(mode_t mode)
    {
        this->mode = mode;
    }

    void set_fuse_file(fuse_file_info *fileinfo)
    {
        assert(fileinfo != nullptr);
        file = *fileinfo;
        file_ptr = &file;
    }

    void release()
    {
        ::free(file_name);
    }

private:
    fuse_ino_t parent_ino;
    char *file_name;
    mode_t mode;
    struct fuse_file_info file;
    struct fuse_file_info *file_ptr;
    bool is_used;
};

struct mkdir_rpc_task
{
    fuse_ino_t get_parent_ino() const
    {
        return parent_ino;
    }

    const char *get_dir_name() const
    {
        return dir_name;
    }

    mode_t get_mode() const
    {
        return mode;
    }

    void set_parent_ino(fuse_ino_t parent)
    {
        parent_ino = parent;
    }

    void set_dir_name(const char *name)
    {
        dir_name = ::strdup(name);
    }

    void set_mode(mode_t mod)
    {
        mode = mod;
    }

    void release()
    {
        ::free(dir_name);
    }

private:
    fuse_ino_t parent_ino;
    char *dir_name;
    mode_t mode;
    bool is_used;
};

struct unlink_rpc_task
{
    fuse_ino_t get_parent_ino() const
    {
        return parent_ino;
    }

    const char *get_file_name() const
    {
        return file_name;
    }

    void set_parent_ino(fuse_ino_t parent)
    {
        parent_ino = parent;
    }

    void set_file_name(const char *name)
    {
        file_name = ::strdup(name);
    }

    void release()
    {
        ::free(file_name);
    }

private:
    fuse_ino_t parent_ino;
    char *file_name;
};

struct rmdir_rpc_task
{
    fuse_ino_t get_parent_ino() const
    {
        return parent_ino;
    }

    const char *get_dir_name() const
    {
        return dir_name;
    }

    void set_parent_ino(fuse_ino_t parent)
    {
        parent_ino = parent;
    }

    void set_dir_name(const char *name)
    {
        dir_name = ::strdup(name);
    }

    void release()
    {
        ::free(dir_name);
    }

private:
    fuse_ino_t parent_ino;
    char *dir_name;
};

struct symlink_rpc_task
{
    fuse_ino_t get_parent_ino() const
    {
        return parent_ino;
    }

    const char *get_name() const
    {
        return name;
    }

    const char *get_link() const
    {
        return link;
    }

    void set_parent_ino(fuse_ino_t parent)
    {
        parent_ino = parent;
    }

    void set_name(const char *_name)
    {
        name = ::strdup(_name);
    }

    void set_link(const char *_link)
    {
        link = ::strdup(_link);
    }

    void release()
    {
        ::free(name);
        ::free(link);
    }

private:
    fuse_ino_t parent_ino;
    char *name;
    char *link;
};

struct rename_rpc_task
{
    fuse_ino_t get_parent_ino() const
    {
        return parent_ino;
    }

    fuse_ino_t get_newparent_ino() const
    {
        return newparent_ino;
    }

    const char *get_name() const
    {
        return name;
    }

    const char *get_newname() const
    {
        return newname;
    }

    unsigned int get_flags() const
    {
        return flags;
    }

    void set_parent_ino(fuse_ino_t parent)
    {
        parent_ino = parent;
    }

    void set_newparent_ino(fuse_ino_t parent)
    {
        newparent_ino = parent;
    }

    void set_name(const char *_name)
    {
        name = ::strdup(_name);
    }

    void set_newname(const char *name)
    {
        newname = ::strdup(name);
    }

    void set_flags(unsigned int _flags)
    {
        flags = _flags;
    }

    void release()
    {
        ::free(name);
        ::free(newname);
    }

private:
    fuse_ino_t parent_ino;
    fuse_ino_t newparent_ino;
    char *name;
    char *newname;
    unsigned int flags;
};

struct readlink_rpc_task
{
    void set_ino(fuse_ino_t _ino)
    {
        ino = _ino;
    }

    fuse_ino_t get_ino() const
    {
        return ino;
    }

private:
    fuse_ino_t ino;
};

struct readdir_rpc_task
{
public:
    void set_size(size_t sz)
    {
        size = sz;
    }

    void set_offset(off_t off)
    {
        offset = off;
    }

    void set_ino(fuse_ino_t ino)
    {
        inode = ino;
    }

    void set_fuse_file(fuse_file_info* fileinfo)
    {
        // The fuse can pass this as nullptr.
        if (fileinfo != nullptr) {
            file = *fileinfo;
            file_ptr = &file;
        } else {
            file_ptr = nullptr;
        }
    }

    fuse_ino_t get_ino() const
    {
        return inode;
    }

    off_t get_offset() const
    {
        return offset;
    }

    size_t get_size() const
    {
        return size;
    }

    struct fuse_file_info *get_fuse_file() const
    {
        return file_ptr;
    }

private:
    // Inode of the directory.
    fuse_ino_t inode;

    // Maximum size of entries requested by the caller.
    size_t size;

    off_t offset;

    // File info passed by the fuse layer.
    fuse_file_info file;
    fuse_file_info* file_ptr;
};

struct read_rpc_task
{
public:
    void set_size(size_t sz)
    {
        size = sz;
    }

    void set_offset(off_t off)
    {
        offset = off;
    }

    void set_ino(fuse_ino_t ino)
    {
        inode = ino;
    }

    void set_fuse_file(fuse_file_info* fileinfo)
    {
        // The fuse can pass this as nullptr.
        if (fileinfo != nullptr) {
            file = *fileinfo;
            file_ptr = &file;
        } else {
            file_ptr = nullptr;
        }
    }

    fuse_ino_t get_ino() const
    {
        return inode;
    }

    off_t get_offset() const
    {
        return offset;
    }

    size_t get_size() const
    {
        return size;
    }

    struct fuse_file_info *get_fuse_file() const
    {
        return file_ptr;
    }

private:
    // Inode of the file.
    fuse_ino_t inode;

    // Size of data to be read.
    size_t size;

    // Offset from which the file data should be read.
    off_t offset;

    // File info passed by the fuse layer.
    fuse_file_info file;
    fuse_file_info *file_ptr;
};

/**
 * RPC API specific task info.
 * This must be sufficient information needed to retry the task in case of
 * JUKEBOX failures.
 */
struct api_task_info
{
    ~api_task_info()
    {
        release();
    }

    /*
     * Fuse request structure.
     * This is the request structure passed from the fuse layer, on behalf of
     * which this RPC task is run.
     */
    fuse_req *req = nullptr;


    /*
     * This will refer to the parent task for a child task.
     * This will be nullptr for parent task.
     *
     * When do we need parent tasks?
     * Note that one fuse request is tracked using one rpc_task, so if we
     * have to issue multiple backend RPCs to serve a single fuse read, then
     * each of those multiple backend RPCs become a new (child) rpc_task and
     * the rpc_task tracking the fuse request becomes the parent task.
     * We do this for a couple of reasons, most importantly it helps RPC
     * accounting/stats (which needs to track every RPC issued to the backend)
     * and secondly it helps to use the RPC pool size for limiting the max
     * outstanding RPCs to the backend.
     *
     * When do we need multiple backend RPCs to serve a single fuse RPC?
     * The only known case is of fuse read when bytes_chunk_cache::get()
     * returns more than one bytes_chunk for a given fuse read. Note that each
     * of these bytes_chunk is issued as an individual RPC READ to the NFS
     * server. We allocate child rpc_task structures for each of these READs
     * and the fuse read is tracked by the parent rpc_task.
     * Note that we can make use of rpc_nfs3_readv_task() API to issue a
     * single RPC READ, but if the to-be-read data is not contiguous inside
     * the file (this can happen if some data in the middle is already in the
     * cache) we may still need multiple RPC READs.
     * This should not be very common though.
     */
    rpc_task *parent_task = nullptr;

    /*
     * The byte chunk for the read task.
     * For a child read task, this will be the byte chunk to which it will
     * read the data.
     */
    struct bytes_chunk *bc = nullptr;

    /*
     * Operation type.
     * Used to access the following union.
     * Note that 0 is an invalid fuse opcode.
     */
    enum fuse_opcode optype = (fuse_opcode) 0;

    /*
     * Unnamed union for easy access.
     */
    union
    {
        struct lookup_rpc_task lookup_task;
        struct write_rpc_task write_task;
        struct flush_rpc_task flush_task;
        struct getattr_rpc_task getattr_task;
        struct setattr_rpc_task setattr_task;
        struct create_file_rpc_task create_task;
        struct mkdir_rpc_task mkdir_task;
        struct unlink_rpc_task unlink_task;
        struct rmdir_rpc_task rmdir_task;
        struct symlink_rpc_task symlink_task;
        struct rename_rpc_task rename_task;
        struct readlink_rpc_task readlink_task;
        struct readdir_rpc_task readdir_task;
        struct read_rpc_task read_task;
    };

    /**
     * We cannot specify destructors for the <api>_rpc_task structures, since
     * they are part of a C union. Use release() method for performing any
     * cleanup.
     */
    void release()
    {
        assert(optype > 0 && optype <= FUSE_OPCODE_MAX);

        switch(optype) {
            case FUSE_LOOKUP:
                lookup_task.release();
                break;
            case FUSE_CREATE:
                create_task.release();
                break;
            case FUSE_MKDIR:
                mkdir_task.release();
                break;
            default :
                break;
        }
    }
};

#define RPC_TASK_MAGIC *((const uint32_t *)"RTSK")

/**
 * This describes an RPC task which is created to handle a fuse request.
 * The RPC task tracks the progress of the RPC request sent to the server and
 * remains valid till the RPC request completes.
 */
struct rpc_task
{
    friend class rpc_task_helper;

    const uint32_t magic = RPC_TASK_MAGIC;

    /*
     * The client for which the context is created.
     * This is initialized when the rpc_task is added to the free tasks list
     * and never changed afterwards, since we have just one nfs_client used by
     * all rpc tasks.
     */
    struct nfs_client *const client;

    // This is the index of the object in the rpc_task_list vector.
    const int index;
private:
    /*
     * Flag to identify async tasks.
     * All rpc_tasks start sync, but the caller can make an rpc_task
     * async by calling set_async_function(). The std::function object
     * passed to set_async_function() will be run asynchronously by the
     * rpc_task. That should call the run*() method at the least.
     */
    std::atomic<bool> is_async_task = false;

    // Put a cap on how many async tasks we can start.
    static std::atomic<int> async_slots;

    /*
     * Connection scheduling type to be used for this RPC.
     * Issuer of the RPC will know what connection scheduling type is most
     * optimal for the RPC, it must set it and RPC layer will then honor that.
     */
    conn_sched_t csched = CONN_SCHED_INVALID;

    /*
     * FH hash to be used for connection scheduling if/for CONN_SCHED_FH_HASH.
     */
    uint32_t fh_hash = 0;

public:
    /*
     * Valid only for read RPC tasks.
     * To serve single client read call we may issue multiple NFS reads
     * depending on the chunks returned by bytes_chunk_cache::get().
     *
     * num_ongoing_backend_reads tracks how many of the backend reads are
     * currently pending. We cannot complete the application read until all
     * reads complete (either success or failure).
     *
     * read_status is the final read status we need to send to fuse.
     * This is set when we get an error, so that even if later reads complete
     * successfully we fail the fuse read.
     */
    std::atomic<int> num_ongoing_backend_reads = 0;
    std::atomic<int> read_status = 0;

    /*
     * This is currently valid only for reads.
     * This contains vector of byte chunks which is returned by making a call
     * to bytes_chunk_cache::get().
     * This is populated by only one thread that calls run_read() for this
     * task, once populated multiple parallel threads may read it, so we
     * don't need to synchronize access to this with a lock.
     */
    std::vector<bytes_chunk> bc_vec;

    enum fuse_opcode optype = (fuse_opcode) 0;

protected:
    /*
     * RPC stats. This has both stats specific to this RPC as well as
     * aggregated RPC stats for all RPCs of this type and also global stats
     * for all RPCs.
     */
    rpc_stats_az stats;

public:
    rpc_task(struct nfs_client *_client, int _index) :
        client(_client),
        index(_index)
    {
    }

    /*
     * RPC API specific task info.
     * This is a pointer so that it can be quickly xferred to jukebox_seedinfo.
     */
    api_task_info *rpc_api = nullptr;

    // TODO: Add valid flag here for APIs?

    /**
     * Set a function to be run by this rpc_task asynchronously.
     * Calling set_async_function() makes an rpc_task async.
     * The callback function takes two parameters:
     * 1. rpc_task pointer.
     * 2. Optional arbitrary data that caller may want to pass.
     */
    bool set_async_function(
            std::function<void(struct rpc_task*)> func)
    {
        // Must not already be async.
        assert(!is_async());
        assert(func);

        if (--async_slots < 0) {
            ++async_slots;
            AZLogError("Too many async rpc tasks: {}", async_slots.load());
            assert(0);
            /*
             * TODO: Add a condition_variable where caller can wait and
             *       will be woken up once it can create an async task.
             *       Till then caller can wait for try again.
             */
            return false;
        }

        /*
         * Mark this task async.
         * This has to be done before calling func() as it may check it
         * for the rpc_task.
         */
        is_async_task = true;

        std::thread thread([this, func]()
        {
            func(this);
            /*
             * We increase the async_slots here and not in free_rpc_task()
             * as the task is technically done, it just needs to be reaped.
             */
            async_slots++;
        });

        /*
         * We detach from this thread to avoid having to join it, as
         * that causes problems with some caller calling free_rpc_task()
         * from inside func(). Moreover this is more like the sync tasks
         * where the completion context doesn't worry whether the issuing
         * thread has completed.
         * Since we don't have a graceful exit scenario, waiting for the
         * async threads is not really necessary.
         */
        thread.detach();

        return true;
    }

    /**
     * Return a string representation of opcode for logging.
     */
    static const std::string fuse_opcode_to_string(fuse_opcode opcode);

    /**
     * Check if this rpc_task is an async task.
     */
    bool is_async() const
    {
        return is_async_task;
    }

    /*
     * init/run methods for the LOOKUP RPC.
     */
    void init_lookup(fuse_req *request,
                     const char *name,
                     fuse_ino_t parent_ino);
    void run_lookup();

    /*
     * init/run methods for the WRITE RPC.
     */
    void init_write(fuse_req *request,
                    fuse_ino_t ino,
                    struct fuse_bufvec *buf,
                    size_t size,
                    off_t offset);
    void run_write();

    /*
     * init/run methods for the FLUSH/RELEASE RPC.
     */
    void init_flush(fuse_req *request,
                    fuse_ino_t ino);
    void run_flush();

    /*
     * init/run methods for the GETATTR RPC.
     */
    void init_getattr(fuse_req *request,
                      fuse_ino_t ino);
    void run_getattr();

    /*
     * init/run methods for the SETATTR RPC.
     */
    void init_setattr(fuse_req *request,
                      fuse_ino_t ino,
                      const struct stat *attr,
                      int to_set,
                      struct fuse_file_info *file);
    void run_setattr();

    /*
     * init/run methods for the CREATE RPC.
     */
    void init_create_file(fuse_req *request,
                          fuse_ino_t parent_ino,
                          const char *name,
                          mode_t mode,
                          struct fuse_file_info *file);
    void run_create_file();

    /*
     * init/run methods for the MKDIR RPC.
     */
    void init_mkdir(fuse_req *request,
                    fuse_ino_t parent_ino,
                    const char *name,
                    mode_t mode);
    void run_mkdir();

    /*
     * init/run methods for the REMOVE RPC.
     */
    void init_unlink(fuse_req *request,
                     fuse_ino_t parent_ino,
                     const char *name);

    void run_unlink();

    /*
     * init/run methods for the RMDIR RPC.
     */
    void init_rmdir(fuse_req *request,
                    fuse_ino_t parent_ino,
                    const char *name);

    void run_rmdir();

    /*
     * init/run methods for the SYMLINK RPC.
     */
    void init_symlink(fuse_req *request,
                      const char *link,
                      fuse_ino_t parent_ino,
                      const char *name);

    void run_symlink();

    /*
     * init/run methods for the RENAME RPC.
     */
    void init_rename(fuse_req *request,
                     fuse_ino_t parent_ino,
                     const char *name,
                     fuse_ino_t newparent_ino,
                     const char *newname,
                     unsigned int flags);

    void run_rename();

    /*
     * init/run methods for the READLINK RPC.
     */
    void init_readlink(fuse_req *request,
                       fuse_ino_t ino);

    void run_readlink();

    // This function is responsible for setting up the members of readdir_task.
    void init_readdir(fuse_req *request,
                     fuse_ino_t inode,
                     size_t size,
                     off_t offset,
                     struct fuse_file_info *file);

    void run_readdir();

    // This function is responsible for setting up the members of readdirplus_task.
    void init_readdirplus(fuse_req *request,
                         fuse_ino_t inode,
                         size_t size,
                         off_t offset,
                         struct fuse_file_info *file);

    void run_readdirplus();

    // This function is responsible for setting up the members of read task.
    void init_read(fuse_req *request,
                   fuse_ino_t inode,
                   size_t size,
                   off_t offset,
                   struct fuse_file_info *file);

    void run_read();

    void set_csched(conn_sched_t _csched)
    {
        assert(_csched > CONN_SCHED_INVALID &&
               _csched <= CONN_SCHED_FH_HASH);
        csched = _csched;
    }

    conn_sched_t get_csched() const
    {
        assert(csched > CONN_SCHED_INVALID &&
               csched <= CONN_SCHED_FH_HASH);
        return csched;
    }

    uint32_t get_fh_hash() const
    {
        // When get_fh_hash() is called, fh_hash must be set.
        assert(fh_hash != 0);
        return fh_hash;
    }

    void set_fuse_req(fuse_req *request)
    {
        rpc_api->req = request;
    }

    struct fuse_req *get_fuse_req() const
    {
        return rpc_api->req;
    }

    void set_op_type(enum fuse_opcode _optype)
    {
        optype = rpc_api->optype = _optype;
    }

    enum fuse_opcode get_op_type() const
    {
        return optype;
    }

    rpc_stats_az& get_stats()
    {
        return stats;
    }

    struct nfs_context *get_nfs_context() const;

    struct rpc_context *get_rpc_ctx() const
    {
        return nfs_get_rpc_context(get_nfs_context());
    }

    nfs_client *get_client() const
    {
        assert (client != nullptr);
        return client;
    }

    int get_index() const
    {
        return index;
    }

    // The task should not be accessed after this function is called.
    void free_rpc_task();

    // This method will reply with error and free the rpc task.
    void reply_error(int rc)
    {
        fuse_reply_err(get_fuse_req(), rc);
        free_rpc_task();
    }

    void reply_readlink(const char* linkname)
    {
        fuse_reply_readlink(get_fuse_req(), linkname);
        free_rpc_task();
    }

    void reply_attr(const struct stat *attr, double attr_timeout)
    {
        fuse_reply_attr(get_fuse_req(), attr, attr_timeout);
        free_rpc_task();
    }

    void reply_write(size_t count)
    {
        /*
         * Currently fuse sends max 1MiB write requests, so we should never
         * be responding more than that.
         * This is a sanity assert for catching unintended bugs, update if
         * fuse max write size changes.
         */
        assert(count <= 1048576);

        fuse_reply_write(get_fuse_req(), count);
        free_rpc_task();
    }

    void reply_iov(struct iovec *iov, size_t count)
    {
        // If count is non-zero iov must be valid and v.v.
        assert((iov == nullptr) == (count == 0));

        /*
         * Currently fuse sends max 1MiB read requests, so we should never
         * be responding more than that.
         * This is a sanity assert for catching unintended bugs, update if
         * fuse max read size changes.
         */
        assert(count <= 1048576);

        fuse_reply_iov(get_fuse_req(), iov, count);
        free_rpc_task();
    }

    void reply_entry(const struct fuse_entry_param *e)
    {
        struct nfs_inode *inode = nullptr;

        /*
         * As per fuse on a successful call to fuse_reply_create() the
         * inode's lookup count must be incremented. We increment the
         * inode's lookupcnt in get_nfs_inode(), this lookup count will
         * be transferred to fuse on successful fuse_reply_create() call,
         * but if that fails then we need to drop the ref.
         * "ino == 0" implies a failed lookup call, so we don't have a valid
         * inode number to return.
         */
        if (e->ino != 0) {
            inode = client->get_nfs_inode_from_ino(e->ino);
            assert(inode->lookupcnt >= 1);
        }

        if (fuse_reply_entry(get_fuse_req(), e) < 0) {
            if (inode) {
                /*
                 * Not able to convey to fuse should invoke FORGET
                 * workflow.
                 */
                inode->decref(1, true /* from_forget */);
            }
        }

        free_rpc_task();
    }

    void reply_create(
        const struct fuse_entry_param *entry,
        const struct fuse_file_info *file)
    {
        // inode number cannot be 0 in a create response().
        assert(entry->ino != 0);

        /*
         * As per fuse on a successful call to fuse_reply_create() the
         * inode's lookup count must be incremented. We increment the
         * inode's lookupcnt in get_nfs_inode(), this lookup count will
         * be transferred to fuse on successful fuse_reply_create() call,
         * but if that fails then we need to drop the ref.
         */
        struct nfs_inode *inode = client->get_nfs_inode_from_ino(entry->ino);
        assert(inode->lookupcnt >= 1);

        if (fuse_reply_create(get_fuse_req(), entry, file) < 0) {
            /*
             * Not able to convey to fuse should invoke FORGET
             * workflow.
             */
            inode->decref(1, true /* from_forget */);
        }

        free_rpc_task();
    }

    /**
     * Check RPC and NFS status to find completion status of the RPC task.
     * Returns 0 if rpc_task succeeded execution at the server, else returns
     * a +ve errno value.
     * If user has passed the last argument errstr as non-null, then it'll
     * additionally store an error strig there.
     */
    static int status(int rpc_status,
                      int nfs_status,
                      const char **errstr = nullptr)
    {
        if (rpc_status != RPC_STATUS_SUCCESS) {
            if (errstr) {
                *errstr = "RPC error";
            }

            /*
             * TODO: libnfs only returns the following RPC errors
             *       RPC status can indicate access denied error too,
             *       need to support that.
             */
            assert(rpc_status == RPC_STATUS_ERROR ||
                   rpc_status == RPC_STATUS_TIMEOUT ||
                   rpc_status == RPC_STATUS_CANCEL);

            // For now just EIO.
            return EIO;
        }

        if (nfs_status == NFS3_OK) {
            if (errstr) {
                *errstr = "Success";
            }
            return 0;
        }

        if (errstr) {
            *errstr = nfsstat3_to_str(nfs_status);
        }

        return -nfsstat3_to_errno(nfs_status);
    }

    void send_readdir_response(
            const std::vector<const directory_entry*>& readdirentries);

    void get_readdir_entries_from_cache();

    void fetch_readdir_entries_from_server();
    void fetch_readdirplus_entries_from_server();

    void send_read_response();
    void read_from_server(struct bytes_chunk &bc);
};

class rpc_task_helper
{
private:
    // Mutex for synchronizing access to free_task_index stack.
    std::shared_mutex task_index_lock;

    // Stack containing index into the rpc_task_list vector.
    std::stack<int> free_task_index;

#ifdef ENABLE_PARANOID
    // Set for catching double free.
    std::set<int> free_task_index_set;
#endif

    /*
     * List of RPC tasks which is used to run the task.
     * Making this a vector of rpc_task* instead of rpc_task saves any
     * restrictions on the members of rpc_task. With rpc_task being the
     * element type, it needs to be move constructible, so we cannot have
     * atomic members f.e.
     * Anyway these rpc_task once allocated live for the life of the program.
     */
    std::vector<struct rpc_task*> rpc_task_list;

    // Condition variable to wait for free task index availability.
    std::condition_variable_any cv;

    // This is a singleton class, hence make the constructor private.
    rpc_task_helper(struct nfs_client *client)
    {
        assert(client != nullptr);

        // There should be no elements in the stack.
        assert(free_task_index.empty());

        // Initialize the index stack.
        for (int i = 0; i < MAX_OUTSTANDING_RPC_TASKS; i++) {
            free_task_index.push(i);

#ifdef ENABLE_PARANOID
            const auto p = free_task_index_set.insert(i);
            assert(p.second);
            assert(free_task_index_set.size() == free_task_index.size());
#endif
            rpc_task_list.emplace_back(new rpc_task(client, i));
        }

        // There should be MAX_OUTSTANDING_RPC_TASKS index available.
        assert(free_task_index.size() == MAX_OUTSTANDING_RPC_TASKS);
    }

public:
    ~rpc_task_helper()
    {
        /*
         * We should be called when there are no outstanding tasks.
         */
        assert(free_task_index.size() == MAX_OUTSTANDING_RPC_TASKS);

        while (!free_task_index.empty()) {
            free_task_index.pop();
        }

        for (int i = 0; i < MAX_OUTSTANDING_RPC_TASKS; i++) {
            assert(rpc_task_list[i]);
            delete rpc_task_list[i];
        }
        rpc_task_list.clear();
    }

    static rpc_task_helper *get_instance(struct nfs_client *client = nullptr)
    {
        static rpc_task_helper helper(client);
        return &helper;
    }

    /**
     * This returns a free rpc task instance from the pool of rpc tasks.
     * This call will block till a free rpc task is available.
     */
    struct rpc_task *alloc_rpc_task(fuse_opcode optype)
    {
        // get_free_idx() can block, collect start time before that.
        const uint64_t start_usec = get_current_usecs();
        const int free_index = get_free_idx();
        struct rpc_task *task = rpc_task_list[free_index];

        assert(task->magic == RPC_TASK_MAGIC);
        assert(task->client != nullptr);
        assert(task->index == free_index);
        // Every rpc_task starts as sync.
        assert(!task->is_async());

        /*
         * Only first time around rpc_api will be null for a rpc_task, after
         * that it can be null only if the task failed with a JUKEBOX error in
         * which case the rpc_api would have been xferred to jukebox_seedinfo
         * by nfs_client::jukebox_retry().
         */
        if (!task->rpc_api) {
            task->rpc_api = new api_task_info();
        }

        task->rpc_api->req = nullptr;
        task->set_op_type(optype);
        task->stats.on_rpc_create(optype, start_usec);

        // No task starts as a child task.
        assert(task->rpc_api->parent_task == nullptr);

#ifndef ENABLE_NON_AZURE_NFS
        assert(task->client->mnt_options.nfs_port == 2047 ||
               task->client->mnt_options.nfs_port == 2048);
#endif
        /*
         * Set the default connection scheduling type based on the NFS port
         * used. Later init_*() method can set it to a more appropriate value.
         */
        task->csched = (task->client->mnt_options.nfs_port == 2047) ?
                        CONN_SCHED_RR : CONN_SCHED_FH_HASH;

        return task;
    }

    int get_free_idx()
    {
        std::unique_lock<std::shared_mutex> lock(task_index_lock);

        // Wait until a free rpc task is available.
        while (free_task_index.empty()) {
#ifdef ENABLE_PARANOID
            assert(free_task_index_set.empty());
#endif
            if (!cv.wait_for(lock, std::chrono::seconds(30),
                             [this] { return !free_task_index.empty(); })) {
                AZLogError("Timed out waiting for free rpc_task, re-trying!");
            }
        }

        const int free_index = free_task_index.top();
        free_task_index.pop();

#ifdef ENABLE_PARANOID
        // Must also be free as per free_task_index_set.
        const size_t cnt = free_task_index_set.erase(free_index);
        assert(cnt == 1);
        assert(free_task_index_set.size() == free_task_index.size());
#endif

        // Must be a valid index.
        assert(free_index >= 0 && free_index < MAX_OUTSTANDING_RPC_TASKS);

        return free_index;
    }

    void release_free_index(int index)
    {
        // Must be a valid index.
        assert(index >= 0 && index < MAX_OUTSTANDING_RPC_TASKS);

        {
            std::unique_lock<std::shared_mutex> lock(task_index_lock);

#ifdef ENABLE_PARANOID
            assert(free_task_index_set.size() == free_task_index.size());
            // Must not already be free.
            const auto p = free_task_index_set.insert(index);
            assert(p.second);
#endif

            free_task_index.push(index);
        }

        // Notify any waiters blocked in alloc_rpc_task().
        cv.notify_one();
    }

    void free_rpc_task(struct rpc_task *task)
    {
        assert(task->magic == RPC_TASK_MAGIC);
        task->is_async_task = false;

        release_free_index(task->get_index());
    }
};

/**
 * Seed info needed to re-run a task that had failed with JUKEBOX error.
 */
struct jukebox_seedinfo
{
    ~jukebox_seedinfo()
    {
        /*
         * This will also call api_task_info::release().
         */
        delete rpc_api;
    }

    /*
     * Information needed to restart the task.
     */
    api_task_info *rpc_api;

    /*
     * When to rerun the task.
     */
    int64_t run_at_msecs;

    jukebox_seedinfo(api_task_info *_rpc_api) :
        rpc_api(_rpc_api),
        run_at_msecs(get_current_msecs() + JUKEBOX_DELAY_SECS*1000)
    {
    }
};

#endif /*__RPC_TASK_H__*/
