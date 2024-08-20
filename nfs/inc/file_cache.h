#ifndef __AZNFSC_FILE_CACHE_H__
#define __AZNFSC_FILE_CACHE_H__

#include <map>
#include <mutex>
#include <memory>
#include <vector>
#include <list>
#include <atomic>
#include <chrono>

#include <cstring>
#include <cstdint>
#include <cassert>
#include <unistd.h>

#include "aznfsc.h"

/*
 * Reminder to audit use of asserts to ensure we don't depend on assert
 * for error handling.
 *
 * XXX : This is temporarily disabled to support Release builds, but we need
 *       to audit this before releasing.
 */
#if 0
#ifdef NDEBUG
#error "Need to audit use of asserts in file_cache"
#endif
#endif

/*
 * Uncomment this if you want to use the tailroom from last chunk for
 * new get() requests asking for data right after the last chunk.
 * f.e., let's say application made following sequence of get() requests:
 * 1. get(0, 131072) - due to 128K read request.
 * 2. Now it reads from the backend and backend returned eof after say
 *    10 bytes. Caller will call release(10, 131062) to trim the last
 *    chunk for correctly representing the file size. This will not free
 *    the membuf but just reduce the bytes_chunk's length.
 * 3. Now caller makes the call get(10, 131062).
 *
 * With UTILIZE_TAILROOM_FROM_LAST_MEMBUF defined, the bytes_chunk that
 * we return will still point to the existing membuf. Without the define
 * this new bytes_chunk will get its own membuf of 131062 bytes.
 *
 * Though it saves space but it complicates things wrt setting of uptodate
 * MB_Flag. Since the uptodate flag is a property of the membuf the 2nd
 * caller who gets [10, 1310272) should not treat it as uptodat as it's
 * not (only first 10 bytes are uptodate).
 * Since this will not happen much in practice, for now keep it disabled
 * so that we don't have to worry about this complexity.
 *
 * IMPORTANT: If this is enabled we no longer have the simple rule that all
 *            bytes_chunk with is_empty true refer to "full membuf". If we
 *            ever enable this define we need to think about that too.
 *
 */
//#define UTILIZE_TAILROOM_FROM_LAST_MEMBUF


namespace aznfsc {

/*
 * This is the maximum chunk size we allow. This is like our page size, but
 * unlike the usual page cache where every page is fixed size, our chunk cache
 * may have chunks of different sizes, though for the perfect case where
 * applications are doing sequential reads/writes all/most chunks would have
 * the max size. Infact, we want large chunks to reduce maintenance overhead.
 * Currently fuse kernel driver never sends any read/write IO larger than 1MB,
 * so that will end up being the chunk size, but for background IOs that we
 * initiate (readahead IOs) we will use the max chunk size.
 * A chunk need not map 1:1 to an NFS READ/WRITE RPC, though typically we will
 * issue one NFS RPC for one chunk but we can have 1:m or m:1 mappings where
 * multiple chunks are populated by one NFS RPC and vice versa.
 *
 * See comment above membuf::flag.
 */
#define AZNFSC_MAX_CHUNK_SIZE (4ULL * 1024 * 1024)

#define AZNFSC_BAD_OFFSET (~0ull)

#define PAGE_SIZE (4096ULL)

// Forward declaration.
class bytes_chunk_cache;

/**
 * membuf::flag bits.
 */
namespace MB_Flag {
    enum : uint32_t
    {
       Uptodate = (1 << 0), // Fit for reading.
       Locked   = (1 << 1), // Exclusive access for updating membuf data.
       Dirty    = (1 << 2), // Data in membuf is newer than the Blob.
       Flushing = (1 << 3), // Data from dirty membuf is being synced to Blob.
    };
}

/**
 * Memory buffer, used for caching chunks in memory.
 * For file-backed bytes_chunk_cache membufs are realized by mmap()ed memory,
 * while for non file-backed bytes_chunk_cache membufs are realized by memory
 * allocated on the heap.
 */
struct membuf
{
    /**
     * Three things define a membuf:
     * 1. What offset inside the file it's caching.
     * 2. Count of bytes it's caching.
     * 3. Backing file fd (in case of file-backed caches).
     *
     * We also have the bytes_chunk_cache backlink. This is strictly for
     * updating various cache metrics as membuf flags are updated.
     */
    membuf(bytes_chunk_cache *_bcc,
           uint64_t _offset,
           uint64_t _length,
           int _backing_file_fd = -1);

    /**
     * membuf destructor, frees the memory used by membuf.
     * This frees heap allocated memory for non file-backed membufs, while
     * for file-backed membufs the mmap()ed memory is munmap()ed.
     * Since membuf is used as a shared_ptr, this will be called when the
     * last ref to the shared_ptr is dropped, typically when the bytes_chunk
     * referring to the membuf is discarded.
     */
    ~membuf();

    /*
     * bytes_chunk_cache to which this membuf belongs.
     * This is strictly for updating various cache metrics as membuf flags are
     * updated.
     * Those are atomic, hence it's ok to access w/o serializing access to
     * the cache.
     */
    bytes_chunk_cache *bcc = nullptr;

    // This membuf caches file data in the range [offset, offset+length).
    const uint64_t offset;
    const uint64_t length;

    /*
     * Actual allocated length. This can be greater than length for
     * file-backed membufs. See comments above allocated_buffer.
     * Once set this will not change, even when the membuf is drop'ed and
     * allocated_buffer becomes nullptr.
     */
    uint64_t allocated_length = 0;

    // Backing file fd (-1 for non file-backed caches).
    const int backing_file_fd = -1;

    /*
     * Data buffer used for cacheing. This will be mmap()ed memory for
     * file-backed caches and heap buffer for non file-backed ones. For non
     * file-backed caches, these will never be nullptr, but for file-backed
     * caches, nullptr means that the cache is dropped and we need to load the
     * data from the backing file.
     *
     * Since mmap() can only be done for page aligned file offsets, we need
     * allocated_buffer to track the page aligned mmap()ed address, while
     * buffer is the actual buffer address to use for storing cached data.
     * For non file-backed membufs, both will be same.
     * This also means that for file-backed caches the actual allocated bytes
     * is "length + (buffer - allocated_buffer)". See allocated_length.
     *
     * Once set allocated_buffer should not change. For file-backed caches
     * drop() will munmap() and set allocated_buffer to nullptr.
     */
    uint8_t *buffer = nullptr;
    uint8_t *allocated_buffer = nullptr;

    /*
     * If is_file_backed() is true then 'allocated_buffer' is the mmap()ed
     * address o/w it's the heap allocation address.
     */
    bool is_file_backed() const
    {
        return (backing_file_fd != -1);
    }

    // Returns buffer address for storing the data.
    uint8_t *get() const
    {
        return buffer;
    }

    /**
     * Drop data cached in memory.
     * This is a no-op for non file-backed membufs since for them memory
     * is the only place where data is stored. For file-backed membufs this
     * drops data from the memory while the data is still present in the file.
     * load() can be used to reload data in memory cache.
     *
     * Returns the number of bytes reclaimed by dropping the cache. A -ve
     * return indicates error in munmap().
     */
    int64_t drop();

    /**
     * Load data from file backend into memory.
     * This is a no-op for non file-backed membufs since for them the data
     * is always in memory.
     * load() assumes that the backing file is present and has a size at least
     * equal to offset+length, so that it can map valid data. The backing file
     * obviously will need to be invalidated if the file's data has changed
     * (conveyed by mtime/size change) and anytime the backing file is
     * invalidated all membufs referring to data inside the file MUST be
     * destroyed first.
     */
    bool load();

    /**
     * Is membuf uptodate?
     * Only uptodate membufs are fit for reading.
     * A newly created membuf is not uptodate and must be set uptodate
     * after reading the required data from the Blob.
     */
    bool is_uptodate() const
    {
        return (flag & MB_Flag::Uptodate);
    }

    void set_uptodate();
    void clear_uptodate();

    bool is_locked() const
    {
        const bool locked = (flag & MB_Flag::Locked);
        // If locked, must be inuse.
        assert(is_inuse() || !locked);
        return locked;
    }

    void set_locked();
    void clear_locked();
    bool try_lock();

    /**
     * A membuf is marked dirty when the membuf data is updated, making it
     * out of sync with the Blob contents for the range. This should be done
     * by writer threads which write application data into membuf. A dirty
     * membuf must be written to the Blob before it can be freed. Once written,
     * it should be marked not-dirty by calling clear_dirty().
     */
    bool is_dirty() const
    {
        const bool dirty = (flag & MB_Flag::Dirty);

        /*
         * Make sure is_dirty returns true only when is_uptodate() is true
         * otherwise we may write garbage data to Blob.
         */
        assert(!dirty || is_uptodate());

        return dirty;
    }

    void set_dirty();
    void clear_dirty();
 
    bool is_flushing() const
    {
        return (flag & MB_Flag::Flushing);
    }

    void set_flushing();
    void clear_flushing();

    bool is_inuse() const
    {
        return (inuse > 0);
    }

    void set_inuse();
    void clear_inuse();

private:
    /*
     * Lock to correctly read and update the membuf state.
     *
     * Note on safely accessing membuf
     * ===============================
     * Since multiple threads may be trying to read and write to the same file
     * or part of the file, we need to define some rules for ensuring consistent
     * access. Here are the rules:
     *
     * 1. Any reader or writer gets access to membuf by a call to
     *    bytes_chunk_cache::get(). membufs are managed by shared_ptr, hence
     *    the reader/writer is guaranteed that as long as it does not destroy
     *    the returned bytes_chunk, the underlying membuf will not be freed.
     *    Note that consistent read/write access though needs synchronization
     *    among the various reader/writer threads, read on.
     * 2. A thread trying to write to the membuf must get exclusive access to
     *    the membuf. It can get that by calling set_locked(). set_locked()
     *    will block if the lock is already held by some other thread and will
     *    return after acquiring the lock. Blocking threads will wait on the
     *    condition_variable 'cv' and will be woken up when the current locking
     *    thread unlocks. Note that membuf will be written under the following
     *    cases:
     *     i) A writer writes user passed data to the membuf.
     *    ii) A reader reads data from the Blob into the membuf.
     * 3. Membufs also have an inuse count which indicates if there could be
     *    an ongoing IO (whether there is actually an ongoing IO can be
     *    found by using the locked bit). The purpose of inuse count is to
     *    just mark the membuf such that clear() doesn't clear membufs which
     *    might soon afterwards have IOs issued.
     *    bytes_chunk_cache::get() will bump the inuse count of all membufs
     *    it returns since the caller most likely might perform IO on the
     *    membuf. It's caller's responsibility to clear the inuse by calling
     *    clear_inuse() once they are done performing the IO. This should be
     *    done after performing the IO, and releasing the lock taken for the
     *    IO.
     * 4. A newly created membuf does not have valid data and hence a reader
     *    should not read from it. Such an membuf is "not uptodate" and a
     *    reader must first read the corresponding file data into the membuf,
     *    and mark the membuf "uptodate" after successfully reading the data
     *    into it. It can do that after getting exclusive access to membuf
     *    by calling set_locked(). Any other reader which accesses the membuf
     *    in the meantime will find it "not update" and it'll try to update
     *    the membuf itself but it'll find the membuf locked, so set_locked()
     *    will cause the thread to wait on 'cv'. Once the current reader
     *    updates the membuf, it marks it "uptodate" by calling set_uptodate()
     *    and then unlock it by calling clear_locked(). Other readers waiting
     *    for the lock will get woken up and they will discover that the
     *    membuf is uptodate by checking is_uptodate() and they can then read
     *    that data into their application data buffers.
     *
     *    IMPORTANT RULES FOR UPDATING THE "UPTODATE" BIT
     *    ===============================================
     *    - Any reader that gets a bytes_chunk whose membuf is not uptodate,
     *      must try to read data from the Blob, but only mark it uptodate if
     *      is_empty was also true for the bytes_chunk. This is because
     *      is_empty will be true for bytes_chunk representing "full membuf"
     *      (see UTILIZE_TAILROOM_FROM_LAST_MEMBUF) and hence they only can
     *      correctly mark the membuf as uptodate. Other readers, if they get
     *      the lock first, they can issue the Blob read but they cannot mark
     *      the membuf as uptodate. Note that the first caller to read a byte
     *      range will always get an is_empty bytes_chunk and it should read
     *      into the membuf but some other caller might get the lock first and
     *      hence they may read partial data into the membuf, which will be
     *      overwritten by the caller with is_empty true.
     *      So, ONLY IF maps_full_membuf() returns true for a bytes_chunk, the
     *      reader MUST mark the membuf uptodate.
     *
     *    - Writers must set the uptodate bit only if they write the entire
     *      membuf (maps_full_membuf() returns true), else they should not
     *      change the uptodate bit.
     *
     * 5. If a reader finds that a membuf is uptodate (as per is_uptodate()), it
     *    can return the membuf data to the application. Note that some writer
     *    may be writing to the data simultaneously and reader may get a mix
     *    of old and new data. This is fine as per POSIX. Users who care about
     *    this must synchronize access to the file.
     * 6. Once an membuf is marked uptodate it remains uptodate for the life
     *    of the membuf, unless one of the following happens:
     *     i) We detect via file mtime change that our cached copy is no longer
     *        valid. In this case the entire cache for that file is clear()ed
     *        which causes all bytes_chunk and hence all membufs to be freed.
     *    ii) An NFS read from the given portion of Blob fails. We will need to
     *        understand the effects of this better, since we normally never
     *        fail an NFS IO (think hard mount).
     * 7. A writer MUST observe the following rules:
     *     i) If writer is writing to a part of the membuf, it MUST ensure
     *        that membuf is uptodate before it can modify part of the membuf.
     *        It must do that by reading the *entire* membuf from the Blob,
     *        and marking the membuf as uptodate. Then it must update the part
     *        it wants to. After writing it must mark the membuf as dirty.
     *        All of this must be done while holding the lock on the membuf as
     *        we want the membuf update to be atomic.
     *    ii) If writer is writing to the entire membuf (maps_full_membuf()
     *        returns true), it can directly write to the membuf even if it's
     *        not already uptodate, and after writing it must mark the membuf
     *        as uptodate and dirty.
     * 7. A writer must mark the membuf dirty by calling set_dirty(), after it
     *    updates the membuf data. Dirty membufs must be synced with the Blob
     *    at some later time and once those writes to Blob succeed, the membuf
     *    dirty flag must be cleared by calling clear_dirty(). Note that a
     *    dirty membuf is still uptodate since it has the latest content for
     *    the reader.
     */
    std::mutex lock;

    /*
     * Flag bitmap for correctly defining the state of this membuf.
     * This is a bitwise or of zero or more MB_Flag values.
     *
     * Note: membuf::flag represents the state of the entire membuf,
     *       irrespective of the offset within the membuf a particular
     *       bytes_chunk represents. This means even if one thread has to
     *       read say 1 byte but the actual bytes_chunk created by another
     *       thread is of size 1GB, the former thread has to wait till the
     *       entire 1GB data is read by the other thread and the membuf is
     *       marked MB_Flag::Uptodate.
     *       This means very large membufs will cause unnecessary waits.
     *       Test out and find a good value for AZNFSC_MAX_CHUNK_SIZE.
     */
    std::atomic<uint32_t> flag = 0;

    // For managing threads waiting on MB_Flag::Locked.
    std::condition_variable cv;

    /*
     * Incremented by bytes_chunk_cache::get() before returning a membuf to
     * the caller. Caller must decrement it once they are done reading or
     * writing the membuf.
     */
    std::atomic<uint32_t> inuse = 0;
};

/**
 * This represents one contiguous chunk of bytes in bytes_chunk_cache.
 * bytes_chunk_cache consists of zero or more bytes_chunk ordered by offset.
 * Note that a byte range can be cached using one or more bytes_chunk and the
 * size of the individual component bytes_chunk depends on the order in which
 * the application writes data to the file.
 * A contiguous file range cached by a series of bytes_chunk is called an
 * "extent". Extents are important as they decide if/when we can issue full
 * block-sized write to the Blob.
 */
struct bytes_chunk
{
    // bytes_chunk_cache needs to access the private member alloc_buffer.
    friend bytes_chunk_cache;

private:
    // bytes_chunk_cache to which this chunk belongs.
    bytes_chunk_cache *bcc = nullptr;

    /*
     * This is the allocated buffer. The actual buffer where data is stored
     * can be found by adding buffer_offset to this, and can be retrieved using
     * the convenience function get_buffer(). buffer_offset is typically 0 but
     * it can be non-zero when multiple chunks are referring to the same buffer
     * but at different offsets (e.g., cache trimming).
     * Any chunk that refers to the same allocated buffer will hold a ref to
     * alloc_buffer, so alloc_buffer will be freed when the last ref is dropped.
     * This should typically happen when the chunk is freed.
     *
     * To find the length of the allocated buffer, use alloc_buffer->length.
     */
    std::shared_ptr<membuf> alloc_buffer;

public:
    // Offset from the start of file this chunk represents.
    uint64_t offset = 0;

    /*
     * Length of this chunk.
     * User can safely access [buffer, buffer+length).
     */
    uint64_t length = 0;

    // Offset of buffer from alloc_buffer->get().
    uint64_t buffer_offset = 0;

    /*
     * Private data. User can use this to store anything they want, f.e.,
     * read callback may set this to actual bytes read in case it's less than
     * length.
     * This is opaque to the cache.
     */
    uint64_t pvt = 0;

    /*
     * Number of backend calls issued to sync this byte chunk with the backing
     * blob. It could be read call(s) to read data from the blob or it could be
     * write call(s) to sync dirty byte chunk.
     *
     * Note: Values greater than 1 signify partial read/write calls.
     */
    int num_backend_calls_issued = 0;

    /*
     * is_empty indicates whether buffer contains valid data. It's meaningful
     * when bytes_chunk are returned by a call to bytes_chunk_cache::get(),
     * and not for bytes_chunk stored in bytes_chunk_cache::chunkmap.
     * It is used by the caller differently, depending on whether it wants to
     * write/read to/from the file. For a writer it's not very significant,
     * it simply means that this data is not currently cached. For a reader,
     * otoh, it's significant information as it means that the data in this
     * chunk is not valid file data and caller must ensure that the actual
     * file data is read into it before using it. Chunks with is_empty=false
     * need not read file data as they already have data read from the file.
     * Obviously, the caller has to make sure that cache data is valid, i.e.,
     * file mtime has not changed since the last time it was cached.
     *
     * Note: Once get() returns a chunk, subsequent calls will return the
     *       chunk with is_empty=false. This may confuse new callers to think
     *       the chunk has valid data. The onus is on the caller to synchronize
     *       the callers such that new callers don't get the chunk till the
     *       current one filling the chunk is not done. Typically the file
     *       inode lock will be used for this.
     */
    bool is_empty = true;

    /**
     * Return membuf corresponding to this bytes_chunk.
     * This will be used by caller to synchronize operations on the membuf.
     * See membuf::flag and various operations that can be done on them.
     */
    struct membuf *get_membuf() const
    {
        struct membuf *mb = alloc_buffer.get();

        // membuf must have valid alloc_buffer at all times.
        assert(mb != nullptr);

        return mb;
    }

    /**
     * Returns usecount for the underlying membuf.
     * A bytes_chunk added only to bytes_chunk_cache::chunkmap has a usecount
     * of 1 and every user that calls get() will get one usecount on the
     * respective membuf.
     */
    int get_membuf_usecount() const
    {
        return alloc_buffer.use_count();
    }

    /**
     * Start of valid cached data corresponding to this chunk.
     * This will typically have the value alloc_buffer->get(), i.e., it points
     * to the start of the data buffer represented by the shared pointer
     * alloc_buffer, but if some cached data is deleted from the beginning of a
     * chunk, causing the buffer to be "trimmed" from the beginning, this can
     * point anywhere inside the buffer.
     */
    uint8_t *get_buffer() const
    {
        // Should not call on a dropped cache.
        assert(alloc_buffer->get() != nullptr);
        assert(buffer_offset < alloc_buffer->length);

        return alloc_buffer->get() + buffer_offset;
    }

    /**
     * Does this bytes_chunk cover the "full membuf"?,
     * i.e., following is true:
     * (buffer_offset == 0 && length == alloc_buffer->length)
     */
    bool maps_full_membuf() const
    {
        return ((buffer_offset == 0) && (length == alloc_buffer->length));
    }

    /**
     * Is it safe to release (remove from chunkmap) this bytes_chunk?
     * bytes_chunk whose underlying membuf is either inuse or dirty are not
     * safe to release.
     */
    bool safe_to_release() const
    {
        const struct membuf *mb = get_membuf();
        return !mb->is_inuse() && !mb->is_dirty();
    }

    /**
     * Constructor to create a brand new chunk with newly allocated buffer.
     * This chunk is the sole owner of alloc_buffer and 'buffer_offset' is 0.
     * Later as this chunk is split or returned to the caller through get(),
     * alloc_buffer may have more owners. When the last owner releases claim
     * alloc_buffer will be freed. This should happen when the chunk is freed.
     *
     * XXX: If we need to gracefully handle allocation failure, the buffer
     *      allocation must be done by the caller.
     *
     * XXX Default std new[] implementation is very slow, use tcmalloc for
     *     much faster perf. The main problem with std new is that it doesn't
     *     use memory pools and for large allocations it gets/releases memory
     *     to the system, which causes zero'ing overhead as kernel has to
     *     zero pages.
     */
    bytes_chunk(bytes_chunk_cache *_bcc,
                uint64_t _offset,
                uint64_t _length);

    /**
     * Constructor to create a chunk that refers to alloc_buffer from another
     * existing chunk. The additional _buffer_offset allows flexibility to
     * each chunk to point anywhere inside alloc_buffer.
     * This is useful for chunks created due to splitting or when returning
     * bytes_chunk from bytes_chunk_cache::get().
     */
    bytes_chunk(bytes_chunk_cache *_bcc,
                uint64_t _offset,
                uint64_t _length,
                uint64_t _buffer_offset,
                const std::shared_ptr<membuf>& _alloc_buffer,
                bool _is_empty = false);

    /**
     * Copy constructor, only for use by test code.
     */
    bytes_chunk(const bytes_chunk& rhs) :
        bytes_chunk(rhs.bcc,
                    rhs.offset,
                    rhs.length,
                    rhs.buffer_offset,
                    rhs.alloc_buffer,
                    rhs.is_empty)

    {
        pvt = rhs.pvt;
    }

    /**
     * Default constructor, only for use by test code.
     */
    bytes_chunk() = default;


#ifdef UTILIZE_TAILROOM_FROM_LAST_MEMBUF
    /**
     * Return available space at the end of buffer.
     * This is usually helpful when a prev read() was short and could not fill
     * the entire buffer and then a subsequent read() is issued to fill
     * subsequent data.
     */
    uint64_t tailroom() const
    {
        const int64_t tailroom =
            (alloc_buffer->length - (buffer_offset + length));
        assert(tailroom >= 0);
        assert(tailroom <= (int64_t) AZNFSC_MAX_CHUNK_SIZE);

        return tailroom;
    }
#endif

    /**
     * Drop data cached in memory, for this bytes_chunk.
     *
     * Returns the number of bytes reclaimed by dropping the cache. A -ve
     * return indicates error in munmap().
     */
    int64_t drop()
    {
        assert(get_membuf_usecount() > 0);

        /*
         * If the membuf is being used by someone else, we cannot drop/munmap
         * it, o/w users accessing the data will start getting errors.
         */
        if (get_membuf_usecount() == 1) {
            return alloc_buffer->drop();
        }

        return 0;
    }

    /**
     * Load data from file backend into memory, for this bytes_chunk.
     */
    void load()
    {
        [[maybe_unused]] const bool ret = alloc_buffer->load();
        assert(ret);
    }
};

/**
 * bytes_chunk_cache::scan() can behave differently depending on the scan_action
 * passed.
 */
enum class scan_action
{
    SCAN_ACTION_INVALID = 0,
    SCAN_ACTION_GET,
    SCAN_ACTION_RELEASE,
};

/**
 * This is the per-file cache that caches variable sized extents and is
 * indexed using byte offset and length.
 *
 * Note on read/write performance using bytes_chunk_cache
 * ======================================================
 * If you use file-backed bytes_chunk_cache then the performance of that
 * will be limited by the backing file read/write performance as the data
 * read from the NFS server is placed into the read buffers which are actually
 * the mmap()ed buffers, hence the steady state write performance will be
 * limited by the file write throughput. Having said that, if you have large
 * amount of RAM and the file being read can fit completely in RAM, then the
 * read will happen very fast and then the data can be flushed to the backing
 * file later.
 * OTOH, if you use non file-backed cache, and make sure you release the
 * chunks as they are read from the server, then the read performance is only
 * limited by the memory write speed.
 * Similar log applies to write.
 */
class bytes_chunk_cache
{
    friend membuf;
    friend bytes_chunk;

public:
    bytes_chunk_cache(const char *_backing_file_name = nullptr) :
        backing_file_name(_backing_file_name ? _backing_file_name : "")
    {
        // File will be opened on first access.
        assert(backing_file_fd == -1);
        assert((int) backing_file_len == 0);

        if (!backing_file_name.empty()) {
            AZLogDebug("File-backed bytes_chunk_cache created with backing "
                       "file {}", backing_file_name);
        } else {
            AZLogDebug("Memory-backed bytes_chunk_cache created");
        }

        bcc_all_it = bcc_all.insert(bcc_all.begin(), this);

        AZLogDebug("Added new file cache {}, total file caches now: {}",
                   fmt::ptr(this),
                   bcc_all.size());
    }

    ~bytes_chunk_cache()
    {
        clear();
        bcc_all.erase(bcc_all_it);
        AZLogDebug("Deleted file cache {}, total file caches now: {}",
                   fmt::ptr(this),
                   bcc_all.size());
    }

    /**
     * Call this to check if the cache is empty.
     */
    bool is_empty() const
    {
        return chunkmap.empty();
    }

    /**
     * Return a vector of bytes_chunk cacheing the byte range [offset, offset+length).
     * Parts of the range that correspond to chunks already present in the
     * cache will refer to those existing chunks, for such chunks is_empty will
     * be set to false, while those parts of the range for which there wasn't
     * an already cached chunk found, new chunks will be allocated and inserted
     * into the chunkmap. These new chunks will have is_empty set to true.
     * This means after this function successfully returns there will be
     * chunks present in the cache for the entire range [offset, offset+length).
     *
     * If extent_left and extent_right are non-null, on completion, they will
     * hold the left and right edges of the extent containing the range
     * [offset, offset+left). Note that an extent is a collection of one or
     * more chunks which cache contiguous bytes.
     *
     * This will be called by both,
     * - writers, who want to write to the specified range in the file.
     *   The returned chunks is a scatter list where the caller should write.
     *   bytes_chunk::buffer is the buffer corresponding to each chunk where
     *   caller should write bytes_chunk::length amount of data.
     * - readers, who want to read the specified range from the file.
     *   The returned chunks is a scatter list containing the data from the
     *   file. Chunks with is_empty set to true indicate that we don't have
     *   cached data for that chunk and the caller must arrange to read that
     *   data from the file.
     *
     * TODO: Reuse buffer from prev/adjacent chunk if it has space. Currently
     *       we will allocate a new buffer, this works but is wasteful.
     *       e.g.,
     *       get(0, 4096)
     *       release(10, 4086) <-- this will just update length but the buffer
     *                             will remain.
     *       get(10, 4086)  <-- this get() should reuse the existing buffer.
     *
     *       Update: This is now done, but we still haven't generalized the
     *               solution to reuse buffer for all cases, but the most
     *               common case is now addressed! Leaving the TODO for
     *               tracking the generalized case.
     *       Update2: This introduces challenges, so it's turned off for now.
     *                see UTILIZE_TAILROOM_FROM_LAST_MEMBUF.
     *
     * Note: Caller must do the following for correctly using the returned
     *       bytes_chunks:
     *
     *       1. Since get() increments the inuse count for each membuf it
     *          returns, caller must call clear_inuse() once it's done
     *          performing IO on the membuf. For writers it'll be after they
     *          are done copying application data to the membuf and marking
     *          it dirty, and for readers it'll be after they are done reading
     *          data from the Blob into the membuf. Since membufs for which
     *          is_empty is not true, are not read, for them the inuse count
     *          can be dropped right after return. Once the caller drops inuse
     *          count bytes_chunk_cache::clear() can potentially remove the
     *          membuf from the cache, so the caller must make sure that it
     *          drops inuse count only after correctly setting the state,
     *          i.e., call set_dirty() after writing to the membuf.
     *       2. IOs can be performed to the membuf only after locking it using
     *          set_locked(). Once the IO completes release the lock using
     *          clear_locked(). This must be done before calling clear_inuse().
     *          So the logical seq of operations are:
     *          >> get()
     *          >> for each bytes_chunk returned
     *          >>      set_locked()
     *          >>      perform IO
     *          >>      clear_locked()
     *          >>      clear_inuse()
     */
    std::vector<bytes_chunk> get(uint64_t offset,
                                 uint64_t length,
                                 uint64_t *extent_left = nullptr,
                                 uint64_t *extent_right = nullptr)
    {
        num_get++;
        num_get_g++;
        bytes_get += length;
        bytes_get_g += length;

        /*
         * Perform inline pruning if needed.
         * We do inline pruning when we are "extremely" high on memory usage
         * and hence cannot proceed w/o making space for the new request.
         */
         inline_prune();

        return scan(offset, length, scan_action::SCAN_ACTION_GET,
                    extent_left, extent_right);
    }

    /**
     * Try and release chunks in the range [offset, offset+length) from
     * chunkmap. Only chunks which are fully contained inside the range would
     * be released, while chunks which lie partially in the range are trimmed
     * (by updating the buffer, length and offset members). These will be
     * released later when a future release() call causes them to contain no
     * valid data. release() will skip/ignore any byte range that's not
     * currently cached.
     *
     * Note that a release() call is an advice and not an order. Only those
     * chunks will be released which are not actively being used. Following
     * chunks won't be released:
     * - Which are inuse.
     *   These may have ongoing IOs, so not safe to release.
     * - Which are dirty.
     *   These need to be flushed to the Blob, else we lose data.
     *
     * Additionally, release() will *not* trim chunks unless the release()d
     * range aligns with either the left or the right edge, i.e., for ranges
     * falling in the middle of a chunk will be skipped.
     *
     * If release() successfully releases one or more chunks, a subsequent
     * call to get() won't find them in the chunkmap and hence will allocate
     * fresh chunk (with is_empty true).
     *
     * Note that release() removes the chunks from chunkmap and drops the
     * original ref on the membufs. The membuf itself won't be freed till the
     * last ref on it is dropped, i.e., users can safely access membuf(s)
     * returned by get() even if some other thread calls release().
     *
     * Note: For releasing all chunks and effectively nuking the cache, use
     *       clear(), but note that clear() also won't release above chunks,
     *       for which safe_to_release() returns false.
     */
    void release(uint64_t offset, uint64_t length)
    {
        num_release++;
        num_release_g++;
        bytes_release += length;
        bytes_release_g += length;

        scan(offset, length, scan_action::SCAN_ACTION_RELEASE);
    }

    /*
     * Returns all dirty chunks in chunkmap.
     * Before returning it increases the inuse count of underlying membuf(s).
     * Caller will typically sync dirty membuf to Blob and once done must call
     * clear_inuse().
     */
    std::vector<bytes_chunk> get_dirty_bc() const;

    /**
     * Drop cached data in the given range.
     * This must be called only for file-backed caches. For non file-backed
     * caches this is a no-op.
     *
     * Returns the number of bytes reclaimed by dropping the cache. A -ve
     * return indicates error in munmap().
     *
     * Note: It might make sense to not call drop() at all and leave all chunks
     *       mmap()ed at all times, and depend on kernel to manage the buffer
     *       cache. If kernel drops some part of the buffer cache, subsequent
     *       users of that byte range would cause a page fault and kernel will
     *       silently load data from the backing file.
     *       Another approach would be to use mlock() to lock buffer cache data
     *       that we want and let drop() munlock() it so that kernel can choose
     *       to free it. This needs to be tested.
     *       The advantage of having drop support is that we can choose to
     *       drop specific file caches, which are less/not used, and leave the
     *       more actively used caches mapped. Kernel won't have this knowledge
     *       and it can flush any of the file caches under memory pressure.
     */
    int64_t drop(uint64_t offset, uint64_t length);

    /**
     * Clear the cache by releasing all chunks from the cache.
     * For file-backed cache, this also releases all the file blocks.
     * This will be called for invalidating the cache for a file, typically
     * when we detect that file has changed (through getattr or preop attrs
     * telling that mtime is different than what we have cached).
     *
     * Following chunks won't be released.
     * - Which are inuse.
     *   These may have ongoing IOs, so not safe to release.
     * - Which are dirty.
     *   These need to be flushed to the Blob, else we lose data.
     */
    void clear();

    /**
     * Drop memory cache for all chunks in this bytes_chunk_cache.
     * Chunks will be loaded as user calls get().
     *
     * Returns the number of bytes reclaimed by dropping the cache. A -ve
     * return indicates error in munmap().
     *
     * See discussion in drop().
     */
    int64_t dropall()
    {
        return drop(0, UINT64_MAX);
    }

    bool is_file_backed() const
    {
        return !backing_file_name.empty();
    }

    /**
     * get_prune_goals() looks at the following information and returns prune
     * goals for this cache:
     * - Total memory consumed by all caches.
     * - aznfsc_cfg.cache_max_mb (maximum total cache size allowed).
     * - Memory consumed by this particular cache.
     *
     * It returns two types of prune goals:
     * - Inline.
     *   This tells how much memory to free inline.
     *   This will be non-zero only under extreme memory pressure where we
     *   cannot let writers continue w/o making space.
     * - Periodic.
     *   This tells how much memory to free by the periodic sync thread.
     *   In most common cases this is how memory will be reclaimed.
     */
    void get_prune_goals(uint64_t *inline_bytes, uint64_t *periodic_bytes) const
    {
        // Maximum cache size allowed in bytes.
        static const uint64_t max_total = (aznfsc_cfg.cache.data.user.max_size_mb * 1024 * 1024ULL);
        assert(max_total != 0);

        /*
         * If cache usage grows to 80% of max, we enforce inline pruning for
         * writers. When cache usage grows more than 60% we recommend periodic
         * pruning. If the cache size is sufficient, hopefully we will not need
         * inline pruning too often, as it hurts application write performance.
         * Once curr_bytes_total exceeds inline_threshold we need to perform
         * inline pruning. We prune all the way upto inline_target to avoid
         * hysteresis. Similarly for periodic pruning we prune all the way
         * upto periodic_target.
         *
         * Following also means that at any time, half of the cache_max_mb
         * can be safely present in the cache.
         */
        static const uint64_t inline_threshold = (max_total * 0.8);
        static const uint64_t inline_target = (max_total * 0.7);
        static const uint64_t periodic_threshold = (max_total * 0.6);
        static const uint64_t periodic_target = (max_total * 0.5);

        /*
         * Current total cache size in bytes. Save it once to avoid issues
         * with bytes_allocated* changing midway in these calculations.
         */
        const uint64_t curr_bytes_total = bytes_allocated_g;
        const uint64_t curr_bytes = bytes_allocated;

        if (inline_bytes) {
            *inline_bytes = 0;
        }

        if (periodic_bytes) {
            *periodic_bytes = 0;
        }

        /*
         * If current cache usage is more than the inline_threshold limit, we
         * need to recommend inline pruning. We calculate how much %age of
         * total caches we need to prune and then divide it proportionately
         * among various caches (bigger caches need to prune more). We prune
         * upto inline_target;
         */
        if (inline_bytes && (curr_bytes_total > inline_threshold)) {
            assert(inline_threshold > inline_target);

            // How much to prune?
            const uint64_t total_inline_goal =
                (curr_bytes_total - inline_target);
            const double percent_inline_goal =
                (total_inline_goal * 100.0 ) / curr_bytes_total;

            *inline_bytes = (curr_bytes * percent_inline_goal) / 100;

            // Prune at least 1MB.
            if (*inline_bytes < 1048576) {
                *inline_bytes = 1048576;
            }
        }

        if (periodic_bytes && (curr_bytes_total > periodic_threshold)) {
            assert(periodic_threshold > periodic_target);

            const uint64_t total_periodic_goal =
                (curr_bytes_total - periodic_target);
            const double percent_periodic_goal =
                (total_periodic_goal * 100.0 ) / curr_bytes_total;

            *periodic_bytes = (curr_bytes * percent_periodic_goal) / 100;

            if (*periodic_bytes < 1048576) {
                *periodic_bytes = 1048576;
            }
        }
    }

    /**
     * Check and perform inline pruning if needed.
     * We do inline pruning when we are "extremely" high on memory usage and
     * hence cannot proceed w/o making space for this new request. This must be
     * called from get() which may need more memory.
     *
     * TODO: Also add periodic pruning support.
     */
    void inline_prune();

    /**
     * This will run self tests to test the correctness of this class.
     */
    static int unit_test();

    /*
     * Stats for this cache.
     * These have been made public for easy access, w/o needing whole bunch
     * of accessor methods. Don't update them from outside!
     *
     * bytes_allocated is the total number of memmory bytes allocated for all
     * the bytes_chunk in this cache. Note that all of that memory may not be
     * used for cacheing and bytes_cached is the total bytes actually used for
     * cacheing. Following are the cases where allocated would be larger than
     * used:
     * - release() may release parts of the cache, though membuf cannot be
     *   freed till the entire membuf is unused.
     * - For file-backed cache we have to mmap() on a 4k granularity but the
     *   actual bytes_chunk may not be 4k granular.
     *
     * bytes_cached tracks the total number of bytes cached, not necessarily
     * in memory. For file-backed cache, bytes_cached may refer to memory bytes
     * or file bytes. Note that bytes_cached is not reduced when membuf is
     * drop()ped. This is because the data is still cached, albeit in the
     * backing file.
     */
    std::atomic<uint64_t> num_chunks = 0;
    std::atomic<uint64_t> num_get = 0;
    std::atomic<uint64_t> bytes_get = 0;
    std::atomic<uint64_t> num_release = 0;
    std::atomic<uint64_t> bytes_release = 0;
    std::atomic<uint64_t> bytes_allocated = 0;
    std::atomic<uint64_t> bytes_cached = 0;
    std::atomic<uint64_t> bytes_dirty = 0;
    std::atomic<uint64_t> bytes_uptodate = 0;
    std::atomic<uint64_t> bytes_inuse = 0;
    std::atomic<uint64_t> bytes_locked = 0;

    /*
     * Global stats for all caches.
     */
    static std::atomic<uint64_t> num_chunks_g;
    static std::atomic<uint64_t> num_get_g;
    static std::atomic<uint64_t> bytes_get_g;
    static std::atomic<uint64_t> num_release_g;
    static std::atomic<uint64_t> bytes_release_g;
    static std::atomic<uint64_t> bytes_allocated_g;
    static std::atomic<uint64_t> bytes_cached_g;
    static std::atomic<uint64_t> bytes_dirty_g;
    static std::atomic<uint64_t> bytes_uptodate_g;
    static std::atomic<uint64_t> bytes_inuse_g;
    static std::atomic<uint64_t> bytes_locked_g;

private:
    /**
     * Scan all chunks lying in the range [offset, offset+length) and perform
     * requested action, as described below:
     *
     * SCAN_ACTION_GET     -> Return list of chunks covering the requested
     *                        range, allocating non-existent chunks and adding
     *                        to chunkmap. If extent_left/extent_right are non
     *                        null, they contain the left and right edge of the
     *                        contiguous extent that contains [offset, offset+length).
     * SCAN_ACTION_RELEASE -> Free chunks contained in the requested range.
     */
    std::vector<bytes_chunk> scan(uint64_t offset,
                                  uint64_t length,
                                  scan_action action,
                                  uint64_t *extent_left = nullptr,
                                  uint64_t *extent_right = nullptr);

    /**
     * This must be called with bytes_chunk_cache lock held.
     */
    bool extend_backing_file(uint64_t newlen)
    {
        // No-op for non file-backed caches.
        if (backing_file_fd == -1) {
            assert(backing_file_len == 0);
            return true;
        }

        assert(newlen > 0);
        assert(newlen <= AZNFSC_MAX_FILE_SIZE);
        assert(backing_file_fd > 0);

        if (backing_file_len < newlen) {
            const int ret = ::ftruncate(backing_file_fd, newlen);
            if (ret != 0) {
                AZLogError("ftruncate(fd={}, length={}) failed: {}",
                           backing_file_fd, newlen, strerror(errno));
                assert(0);
                return false;
            }

            backing_file_len = newlen;
        }

        return true;
    }

    /*
     * std::map of bytes_chunk, indexed by the starting offset of the chunk.
     */
    std::map<uint64_t, struct bytes_chunk> chunkmap;

    // Lock to protect chunkmap.
    mutable std::mutex lock;

    std::string backing_file_name;
    int backing_file_fd = -1;
    std::atomic<uint64_t> backing_file_len = 0;

    /*
     * Static list of all bytes_chunk_cache created.
     * bytes_chunk_cache constructor adds to this list while the destructor
     * removes from this list.
     */
    static std::list<bytes_chunk_cache*> bcc_all;
    std::list<bytes_chunk_cache*>::iterator bcc_all_it;
};

}

#endif /* __AZNFSC_FILE_CACHE_H__ */
