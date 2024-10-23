#include <sys/types.h>
#include <sys/stat.h>
#include <fcntl.h>
#include <sys/mman.h>

#include "aznfsc.h"
#include "file_cache.h"
#include "nfs_inode.h"

/*
 * This enables debug logs and also runs the self tests.
 * Must enable once after adding a new self-test.
 */
//#define DEBUG_FILE_CACHE

#ifndef DEBUG_FILE_CACHE
#undef AZLogVerbose
#define AZLogVerbose(fmt, ...)    /* nothing */
#else
/*
 * Debug is not enabled early on when self-tests run, so use Info.
 * Uncomment these if you want to see debug logs from cache self-test.
 */
#undef AZLogVerbose
#define AZLogVerbose AZLogInfo
#endif

namespace aznfsc {

/* static */ std::atomic<uint64_t> bytes_chunk_cache::num_caches = 0;

/* static */ std::atomic<uint64_t> bytes_chunk_cache::num_chunks_g = 0;
/* static */ std::atomic<uint64_t> bytes_chunk_cache::num_get_g = 0;
/* static */ std::atomic<uint64_t> bytes_chunk_cache::bytes_get_g = 0;
/* static */ std::atomic<uint64_t> bytes_chunk_cache::num_release_g = 0;
/* static */ std::atomic<uint64_t> bytes_chunk_cache::bytes_release_g = 0;
/* static */ std::atomic<uint64_t> bytes_chunk_cache::bytes_allocated_g = 0;
/* static */ std::atomic<uint64_t> bytes_chunk_cache::bytes_cached_g = 0;
/* static */ std::atomic<uint64_t> bytes_chunk_cache::bytes_dirty_g = 0;
/* static */ std::atomic<uint64_t> bytes_chunk_cache::bytes_flushing_g = 0;
/* static */ std::atomic<uint64_t> bytes_chunk_cache::bytes_uptodate_g = 0;
/* static */ std::atomic<uint64_t> bytes_chunk_cache::bytes_inuse_g = 0;
/* static */ std::atomic<uint64_t> bytes_chunk_cache::bytes_locked_g = 0;

membuf::membuf(bytes_chunk_cache *_bcc,
               uint64_t _offset,
               uint64_t _length,
               int _backing_file_fd) :
               bcc(_bcc),
               offset(_offset),
               length(_length),
               backing_file_fd(_backing_file_fd)
{
    if (is_file_backed()) {
        assert(allocated_length == 0);

        [[maybe_unused]] const bool ret = load();
        assert(ret);

        // load() must have updated these.
        assert(allocated_length >= length);
        assert(bcc->bytes_allocated >= allocated_length);
        assert(bcc->bytes_allocated_g >= allocated_length);
    } else {
        // TODO: Handle memory alloc failures gracefully.
        allocated_buffer = buffer = new uint8_t[length];
        allocated_length = length;

        bcc->bytes_allocated_g += allocated_length;
        bcc->bytes_allocated += allocated_length;
    }
}

membuf::~membuf()
{
    // bytes_chunk_cache backlink must be set.
    assert(bcc);

    // inuse membuf must never be destroyed.
    assert(!is_inuse());

    // dirty membuf must never be destroyed.
    assert(!is_dirty());

    // locked membuf must never be destroyed.
    assert(!is_locked());

    if (is_file_backed()) {
        if (allocated_buffer) {
            /*
             * allocated_buffer must be page aligned, and buffer must point
             * inside the page starting at allocated_buffer.
             */
            assert(((uint64_t) allocated_buffer & (PAGE_SIZE - 1)) == 0);
            assert(buffer < (allocated_buffer + PAGE_SIZE));
            assert(allocated_length >= length);

            drop();

            // drop() would update metrics.
        }
    } else {
        // Non file-backed membufs must always have a valid buffer.
        assert(allocated_buffer != nullptr);
        assert(buffer == allocated_buffer);
        assert(length == allocated_length);

        assert(bcc->bytes_allocated >= allocated_length);
        assert(bcc->bytes_allocated_g >= allocated_length);
        bcc->bytes_allocated -= allocated_length;
        bcc->bytes_allocated_g -= allocated_length;

        delete [] allocated_buffer;
        allocated_buffer = buffer = nullptr;
    }

    assert(!allocated_buffer);
}

int64_t membuf::drop()
{
    /*
     * Dropping cache for non file-backed chunks doesn't make sense, since for
     * non file-backed caches memory holds the only copy and hence we cannot
     * drop that. For file-backed caches we can drop the memory but the backing
     * file will still contain the cached data and can be loaded when needed.
     */
    if (!is_file_backed()) {
        return 0;
    }

    // If data is not loaded, it's a no-op.
    if (!allocated_buffer) {
        return 0;
    }

    assert(length > 0);
    assert(allocated_length >= length);

    AZLogDebug("munmap(buffer={}, length={})",
               fmt::ptr(allocated_buffer), allocated_length);

    const int ret = ::munmap(allocated_buffer, allocated_length);
    if (ret != 0) {
        AZLogError("munmap(buffer={}, length={}) failed: {}",
                   fmt::ptr(allocated_buffer), allocated_length,
                   strerror(errno));
        assert(0);
        return -1;
    }

    allocated_buffer = buffer = nullptr;

    assert(bcc->bytes_allocated >= allocated_length);
    assert(bcc->bytes_allocated_g >= allocated_length);
    bcc->bytes_allocated -= allocated_length;
    bcc->bytes_allocated_g -= allocated_length;

    return allocated_length;
}

/**
 * TODO: This is currently not used and not tested for various scenarios.
 *       Whet it extensively once we enable file-backed cache.
 *       Few things to consider:
 *       - What will be the membuf flag after load?
 *         If we set it as uptodate then we risk data consistency issues
 *         unless we are certain through other means that the file-backed
 *         data is indeed uptodate with the actual blob data.
 *         OTOH if we set it to non-uptodate then it's not very useful to
 *         load from the backing-file.
 *         This means we need to compare the backing-file mtime with the
 *         blob mtime and decide based on that.
 *      - How do we trim?
 *        We cannot change just in-core data members as they will be lost
 *        once a file-backed cache is dropped and later loaded.
 *        Either we ignore release for file-backed caches or handle it
 *        properly. Note that if we were to munmap() on release then we need
 *        to worry about page boundaries etc.
 */
bool membuf::load()
{
    // Loading memcache for non file-backed chunks doesn't make sense.
    if (!is_file_backed()) {
        // Non file-backed chunks must have a valid buffer at all times.
        assert(allocated_buffer);
        return true;
    }

    // XXX Fail here till we sort out the above TODOs.
    assert(0);

    // If data is already loaded, it's a no-op.
    if (allocated_buffer) {
        return true;
    }

    // allocated_buffer and buffer must agree.
    assert(buffer == nullptr);

#if 0
    // Caller must have ensured this.
    assert(bcc->backing_file_len >= (offset + length));
#endif

    // mmap() allows only 4k aligned offsets.
    const uint64_t adjusted_offset = offset & ~(PAGE_SIZE - 1);

    /*
     * First time around allocated_length would be 0, after that it must be
     * set to correct value.
     */
    assert((allocated_length == 0) ||
           (allocated_length == (length + (offset - adjusted_offset))));

    allocated_length = length + (offset - adjusted_offset);

    AZLogDebug("mmap(fd={}, length={}, offset={})",
               backing_file_fd, allocated_length, adjusted_offset);

    /*
     * Default value of /proc/sys/vm/max_map_count may not be sufficient
     * for large files. Need to increase it.
     */
    assert(adjusted_offset <= offset);
    allocated_buffer =
        (uint8_t *) ::mmap(nullptr,
                           allocated_length,
                           PROT_READ | PROT_WRITE,
                           MAP_SHARED,
                           backing_file_fd,
                           adjusted_offset);

    if (allocated_buffer == MAP_FAILED) {
        AZLogError("mmap(fd={}, length={}, offset={}) failed: {}",
                   backing_file_fd, length, adjusted_offset,
                   strerror(errno));
        assert(0);
        return false;
    }

    buffer = allocated_buffer + (offset - adjusted_offset);

    bcc->bytes_allocated_g += allocated_length;
    bcc->bytes_allocated += allocated_length;

    return true;
}

/**
 * Must be called to set membuf update only after successfully reading
 * all the data that this membuf refers to.
 */
void membuf::set_uptodate()
{
    /*
     * Must be locked and inuse.
     * Note that following is the correct sequence of operations.
     *
     * get()
     * set_locked()
     * << read data from blob into the above membuf(s) >>
     * set_uptodate()
     * clear_locked()
     * clear_inuse()
     */
    assert(is_locked());
    assert(is_inuse());

    /*
     * We set MB_Flag::Uptodate conditionally to keep the bytes_uptodate
     * metrics sane. Also the debug log is helpful to understand when a
     * membuf actually became uptodate for the first time.
     */
    if (!(flag & MB_Flag::Uptodate)) {
        flag |= MB_Flag::Uptodate;

        bcc->bytes_uptodate_g += length;
        bcc->bytes_uptodate += length;

        AZLogDebug("Set uptodate membuf [{}, {}), fd={}",
                   offset, offset+length, backing_file_fd);
    }
}

/**
 * Must be called when a read from Blob fails.
 *
 * Note: This is highly unlikely with the default hard mount semantics.
 */
void membuf::clear_uptodate()
{
    // See comment in set_uptodate() above.
    assert(is_locked());
    assert(is_inuse());

    flag &= ~MB_Flag::Uptodate;

    assert(bcc->bytes_uptodate >= length);
    assert(bcc->bytes_uptodate_g >= length);
    bcc->bytes_uptodate -= length;
    bcc->bytes_uptodate_g -= length;

    AZLogWarn("Clear uptodate membuf [{}, {}), fd={}",
              offset, offset+length, backing_file_fd);

    /*
     * Let's assert in debug builds so that we know if it happens.
     */
    assert(0);
}

/**
 * Must be called to mark membuf as "currently flushing dirty data to Blob".
 * so that any thread wanting to flush a membuf can note this and doesn't wait
 * for membuf lock (for issuing the flush).
 */
void membuf::set_flushing()
{
    /*
     * Must be locked and inuse.
     * Note that following is the correct sequence of operations.
     *
     * get()
     * if (is_dirty() && !is_flushing())
     * {
     *  set_locked()
     *  set_flushing()
     *  << write membuf data to the blob >>
     *  clear_dirty()
     *  clear_flushing()
     *  clear_locked()
     *  clear_inuse()
     * }
     */
    assert(is_locked());
    assert(is_inuse());
    // Dirty MUST be uptodate.
    assert(is_dirty() && is_uptodate());

    flag |= MB_Flag::Flushing;

    bcc->bytes_flushing_g += length;
    bcc->bytes_flushing += length;

    AZLogDebug("Set flushing membuf [{}, {}), fd={}",
               offset, offset+length, backing_file_fd);
}

/**
 * Must be called after flushing dirty membuf to Blob.
 */
void membuf::clear_flushing()
{
    // See comment in set_flushing() above.
    assert(is_locked());
    assert(is_inuse());

    // No spurious calls to clear_flushing().
    assert(is_flushing());

    /*
     * clear_flushing() must be called after clear_dirty().
     * In case WRITE RPC fails, we don't clear dirty flag, in that case this
     * assert will fail. We still leave it as it helps catch workflow bugs and
     * we mostly do hard mount where write never fails.
     *
     * TODO: Remove me once we have enough testing.
     *       Don't release it to production.
     */
    assert(!is_dirty());

    flag &= ~MB_Flag::Flushing;

    assert(bcc->bytes_flushing >= length);
    assert(bcc->bytes_flushing_g >= length);
    bcc->bytes_flushing -= length;
    bcc->bytes_flushing_g -= length;

    AZLogDebug("Clear flushing membuf [{}, {}), fd={}",
               offset, offset+length, backing_file_fd);
}

/**
 * Try to lock the membuf and return whether we were able to lock it.
 * If membuf was already locked, this will return false and caller doesn't
 * have the lock, else caller will have the lock and it'll return true.
 */
bool membuf::try_lock()
{
    assert(is_inuse());
    const bool locked = !(flag.fetch_or(MB_Flag::Locked) & MB_Flag::Locked);

    if (locked) {
        bcc->bytes_locked_g += length;
        bcc->bytes_locked += length;
    }

    return locked;
}

/**
 * A membuf must be locked for getting exclusive access whenever any
 * thread wants to update the membuf data. This can be done by reader
 * threads when they read data from the Blob into a newly created membuf,
 * or by writer threads when they are copying application data into the
 * membuf.
 */
void membuf::set_locked()
{
#ifdef ENABLE_PRESSURE_POINTS
    /*
     * Simulate delay in acquiring lock.
     */
    if (inject_error()) {
        const uint64_t sleep_usecs = random_number(10'000, 1000'000);
        AZLogWarn("set_locked() membuf [{}, {}), delaying {} usecs",
                  offset, offset+length, sleep_usecs);
        ::usleep(sleep_usecs);
    }
#endif

    AZLogDebug("Locking membuf [{}, {}), fd={}",
               offset, offset+length, backing_file_fd);

    /*
     * get() returns with inuse set on the returned membufs.
     * Caller should drop the inuse count only after the IO is fully done.
     * i.e. following is the valid sequence of calls.
     *
     * get()
     * set_locked()
     * << perform IO >>
     * clear_locked()
     * clear_inuse()
     */
    assert(is_inuse());

    // Common case, not locked, lock w/o waiting.
    while (!try_lock()) {
        std::unique_lock<std::mutex> _lock(mb_lock_44);

        /*
         * When reading data from the Blob, NFS read may take some time,
         * we wait for 120 secs and log an error message, to catch any
         * deadlocks.
         */
        if (!cv.wait_for(_lock, std::chrono::seconds(120),
                         [this]{ return !this->is_locked(); })) {
            AZLogError("Timed out waiting for membuf lock, re-trying!");
        }
    }

    AZLogDebug("Successfully locked membuf [{}, {}), fd={}",
               offset, offset+length, backing_file_fd);

    // Must never return w/o locking the membuf.
    assert(is_locked());
    assert(is_inuse());

    return;
}

/**
 * Unlock after a prior successful call to set_locked().
 */
void membuf::clear_locked()
{
    // Must be locked, catch bad callers.
    assert(is_locked());

    // inuse must be set. See comment in set_locked().
    assert(is_inuse());

    // Flushing musy be done with lock held for the entire duration.
    assert(!is_flushing());

    {
        std::unique_lock<std::mutex> _lock(mb_lock_44);
        flag &= ~MB_Flag::Locked;

        AZLogDebug("Unlocked membuf [{}, {}), fd={}",
                   offset, offset+length, backing_file_fd);
    }

    assert(bcc->bytes_locked >= length);
    assert(bcc->bytes_locked_g >= length);
    bcc->bytes_locked -= length;
    bcc->bytes_locked_g -= length;

    // Wakeup one waiter.
    cv.notify_one();
}

void membuf::set_dirty()
{
    /*
     * Must be locked and inuse.
     * Note that following is the correct sequence of operations.
     *
     * get()
     * set_locked()
     * << write application data into the above membuf(s) >>
     * set_dirty()
     * clear_locked()
     * clear_inuse()
     */
    assert(is_locked());
    assert(is_inuse());

    flag |= MB_Flag::Dirty;

    bcc->bytes_dirty_g += length;
    bcc->bytes_dirty += length;

    AZLogDebug("Set dirty membuf [{}, {}), fd={}",
               offset, offset+length, backing_file_fd);
}

void membuf::clear_dirty()
{
    // See comment in set_dirty().
    assert(is_locked());
    assert(is_inuse());

    // We completed writing dirty data, flushing must have been set.
    assert(is_flushing());

    flag &= ~MB_Flag::Dirty;

    assert(bcc->bytes_dirty >= length);
    assert(bcc->bytes_dirty_g >= length);
    bcc->bytes_dirty -= length;
    bcc->bytes_dirty_g -= length;

    AZLogDebug("Clear dirty membuf [{}, {}), fd={}",
               offset, offset+length, backing_file_fd);
}

void membuf::set_inuse()
{
    bcc->bytes_inuse_g += length;
    bcc->bytes_inuse += length;

    inuse++;
}

void membuf::clear_inuse()
{
    /*
     * We must not clear inuse with lock held but we cannot assert for
     * !is_locked() here as some other thread can get the lock after we release.
     */

    assert(bcc->bytes_inuse >= length);
    assert(bcc->bytes_inuse_g >= length);
    bcc->bytes_inuse -= length;
    bcc->bytes_inuse_g -= length;

    assert(inuse > 0);
    inuse--;
}

bytes_chunk::bytes_chunk(bytes_chunk_cache *_bcc,
                         uint64_t _offset,
                         uint64_t _length) :
             bytes_chunk(_bcc,
                         _offset,
                         _length,
                         0 /* buffer_offset */,
                         std::make_shared<membuf>(_bcc,
                                                  _offset,
                                                  _length,
                                                  _bcc->backing_file_fd),
                         true /* is_whole */,
                         true /* is_new */)
{
}

bytes_chunk::bytes_chunk(bytes_chunk_cache *_bcc,
                         uint64_t _offset,
                         uint64_t _length,
                         uint64_t _buffer_offset,
                         const std::shared_ptr<membuf>& _alloc_buffer,
                         bool _is_whole,
                         bool _is_new) :
             bcc(_bcc),
             alloc_buffer(_alloc_buffer),
             offset(_offset),
             length(_length),
             buffer_offset(_buffer_offset),
             is_whole(_is_whole),
             is_new(_is_new)
{
    // new bytes_chunk MUST cover whole membuf.
    assert(!is_new || is_whole);

    assert(bcc != nullptr);
    // alloc_buffer must always be valid.
    assert(alloc_buffer != nullptr);
    assert(alloc_buffer.use_count() > 1);
    /*
     * By the time bytes_chunk constructor is called
     * bytes_chunk_cache::scan() MUST have opened the backing file.
     */
    assert(bcc->backing_file_name.empty() == (bcc->backing_file_fd == -1));
    assert(offset < AZNFSC_MAX_FILE_SIZE);
    // 0-sized chunks don't exist.
    assert(length > 0);
    assert(length <= alloc_buffer->length);
    assert((buffer_offset + length) <= alloc_buffer->length);
    assert(alloc_buffer->length <= AZNFSC_MAX_CHUNK_SIZE);
    assert((offset + length) <= AZNFSC_MAX_FILE_SIZE);

    /*
     * Since we are allocating this chunk most likely user is going to
     * use it, load data from backing file, if not already loaded.
     *
     * XXX This load() failure is fatal.
     */
    load();
    assert(get_buffer() != nullptr);
}

std::vector<bytes_chunk> bytes_chunk_cache::scan(uint64_t offset,
                                                 uint64_t length,
                                                 scan_action action,
                                                 uint64_t *bytes_released,
                                                 uint64_t *extent_left,
                                                 uint64_t *extent_right)
{
#ifdef ENABLE_PRESSURE_POINTS
    /*
     * Simulate delay in getting bytes_chunk vector.
     */
    if (inject_error()) {
        const uint64_t sleep_usecs = random_number(10'000, 1000'000);
        if (inode) {
            AZLogWarn("[{}] scan(offset={}, length={}), delaying {} usecs",
                      inode->get_fuse_ino(), offset, length, sleep_usecs);
        }
        ::usleep(sleep_usecs);
    }
#endif

    assert(offset < AZNFSC_MAX_FILE_SIZE);
    assert(length > 0);
    // Cannot write more than AZNFSC_MAX_CHUNK_SIZE in a single call.
    assert(length <= AZNFSC_MAX_CHUNK_SIZE);
    assert((offset + length) <= AZNFSC_MAX_FILE_SIZE);
    assert((action == scan_action::SCAN_ACTION_GET) ||
           (action == scan_action::SCAN_ACTION_RELEASE));

    // Range check makes sense only for get().
    assert((action == scan_action::SCAN_ACTION_GET) ||
           (extent_left == nullptr && extent_right == nullptr));

    // Doesn't make sense to query just one.
    assert((extent_left == nullptr) == (extent_right == nullptr));

    // bytes_released MUST be passed for (and only for) SCAN_ACTION_RELEASE.
    assert((action == scan_action::SCAN_ACTION_RELEASE) ==
           (bytes_released != nullptr));

    // inode must be valid when get()/release() is called.
    assert(!inode || (inode->magic == NFS_INODE_MAGIC));

    // bytes_chunk vector that will be returned to the caller.
    std::vector<bytes_chunk> chunkvec;

    // offset and length cursor, updated as we add chunks to chunkvec.
    uint64_t next_offset = offset;
    uint64_t remaining_length = length;

    if (bytes_released)
        *bytes_released = 0;

    /*
     * Do we need to find containing extent's left and right edges?
     * We should need it only when the caller intends to write to the returned
     * membufs.
     */
    const bool find_extent = (extent_left != nullptr);

    // Convenience variable to access the current chunk in the map.
    bytes_chunk *bc;

#ifdef UTILIZE_TAILROOM_FROM_LAST_MEMBUF
    // Last chunk (when we are getting byte range right after the last chunk).
    bytes_chunk *last_bc = nullptr;
#endif

    // Temp variables to hold chunk details for newly added chunk.
    uint64_t chunk_offset, chunk_length;

    /*
     * TODO: See if we can hold shared lock for cases where we don't have to
     *       update chunkmap.
     */
    const std::unique_lock<std::mutex> _lock(chunkmap_lock_43);

	/*
	 * Before we proceed with the cache lookup check if invalidate is pending.
	 */
	if (invalidate_pending.exchange(false)) {
		AZLogDebug("[{}] (Deferred) Purging file_cache", inode->get_fuse_ino());
		clear_nolock();
	}

    /*
     * Temp variables to hold details for releasing a range.
     * All chunks in the range [begin_delete, end_delete) will be freed as
     * they fall completely inside the released range.
     * Used only for SCAN_ACTION_RELEASE.
     */
    std::map <uint64_t,
              struct bytes_chunk>::iterator begin_delete = chunkmap.end();
    std::map <uint64_t,
              struct bytes_chunk>::iterator end_delete = chunkmap.end();

    /*
     * Variables to track the extent this write is part of.
     * We will udpate these as the left and right edges of the extent are
     * confirmed. Used only for SCAN_ACTION_GET when find_extent is true,
     * which will be true for writers.
     * lookback_it is the iterator to the chunk starting which we should
     * "look back" for the left edge of the extent containing the just written
     * chunk. We basically scan to the left till we find a gap or we find a
     * membuf that has needs_flush() false or we hit the end.
     * Note that these will only ever point to a membuf edge.
     */
    uint64_t _extent_left = AZNFSC_BAD_OFFSET;
    uint64_t _extent_right = AZNFSC_BAD_OFFSET;
    std::map <uint64_t,
              struct bytes_chunk>::iterator lookback_it = chunkmap.end();

#define SET_LOOKBACK_IT_TO_PREV() \
do { \
    if (it != chunkmap.begin()) { \
        lookback_it = std::prev(it); \
        bc = &(lookback_it->second); \
        AZLogVerbose("lookback_it: [{},{})", \
                     bc->offset, bc->offset + bc->length); \
    } else { \
        assert(lookback_it == chunkmap.end()); \
    } \
} while (0)

    /*
     * First things first, if file-backed cache and backing file not yet open,
     * open it.
     */
    if (action == scan_action::SCAN_ACTION_GET) {
        if ((backing_file_fd == -1) && !backing_file_name.empty()) {
            backing_file_fd = ::open(backing_file_name.c_str(),
                                     O_CREAT|O_TRUNC|O_RDWR, 0755);
            if (backing_file_fd == -1) {
                AZLogError("Failed to open backing_file {}: {}",
                           backing_file_name, strerror(errno));
                assert(0);
                return chunkvec;
            } else {
                AZLogInfo("Opened backing_file {}: fd={}",
                           backing_file_name, backing_file_fd);
            }
        }

        /*
         * Extend backing_file as the very first thing.
         * It is important that when membuf::load() is called, the backing file
         * has size >= (offset + length).
         */
        if (!extend_backing_file(offset + length)) {
            AZLogError("Failed to extend backing_file to {} bytes: {}",
                       offset+length, strerror(errno));
            assert(0);
            return chunkvec;
        }
    }

    /*
     * Find chunk with offset >= next_offset.
     * We start from the first chunk covering the start of the requested range
     * and then iterate over the subsequent chunks (allocating missing chunks
     * along the way) till we cover the entire requested range. Newly allocated
     * chunks can be identified in the returned chunkvec as they have is_new
     * set.
     */
    auto it = chunkmap.lower_bound(next_offset);

    if (it == chunkmap.end()) {
        /*
         * next_offset is greater than the greatest offset in the chunkmap.
         * We still have to check the last chunk to see if it has some or all
         * of the requested range.
         */
        if (chunkmap.empty()) {
            if (action == scan_action::SCAN_ACTION_RELEASE) {
                /*
                 * Empty cache, nothing to release.
                 */
                AZLogVerbose("<Release [{}, {})> Empty cache, nothing to release",
                             offset, offset + length);
                goto end;
            }

            /*
             * Only chunk being added, so left and right edge of that are also
             * the extent's left and right edge.
             */
            _extent_left = next_offset;
            _extent_right = next_offset + remaining_length;

            AZLogVerbose("(first/only chunk) _extent_left: {} _extent_right: {}",
                         _extent_left, _extent_right);

            assert(lookback_it == chunkmap.end());
            goto allocate_only_chunk;
        } else {
            // Iterator to the last chunk.
            it = std::prev(it);
            bc = &(it->second);

            if ((bc->offset + bc->length) <= next_offset) {
                /*
                 * Requested range lies after the end of last chunk. This means
                 * for SCAN_ACTION_RELEASE we have nothing to do.
                 * For SCAN_ACTION_GET we will need to allocate a new chunk and
                 * this will be the only chunk needed to cover the requested range.
                 */
                if (action == scan_action::SCAN_ACTION_RELEASE) {
                    AZLogVerbose("<Release [{}, {})> First byte to release "
                                 "lies after the last chunk [{}, {})",
                                 offset, offset + length,
                                 bc->offset, bc->offset + bc->length);
                    goto end;
                }

                if ((bc->offset + bc->length) < next_offset) {
                    /*
                     * New chunk starts at a gap after the last chunk.
                     * next_offset is the definitive _extent_left and we don't
                     * need to look back.
                     */
                    _extent_left = next_offset;
                    AZLogVerbose("_extent_left: {}", _extent_left);
                    assert(lookback_it == chunkmap.end());
                } else {
                    /*
                     * New chunk starts right after the last chunk.
                     * Set tentative left edge and set lookback_it to the last
                     * chunk so that we can later "look back" and find the
                     * actual left edge.
                     */
                    _extent_left = next_offset;
                    AZLogVerbose("(tentative) _extent_left: {}", _extent_left);

                    AZLogVerbose("lookback_it: [{},{})",
                                 bc->offset, bc->offset + bc->length);
                    lookback_it = it;
#ifdef UTILIZE_TAILROOM_FROM_LAST_MEMBUF
                    last_bc = bc;
#endif
                }

                _extent_right = next_offset + remaining_length;
                AZLogVerbose("_extent_right: {}", _extent_right);

                assert(remaining_length > 0);
                goto allocate_only_chunk;
            } else {
                /*
                 * Part or whole of requested range lies in the last chunk.
                 * Set _extent_left tentatively, _extent_right will be set by
                 * the for loop below. Also for finding the real left edge we
                 * need to search backwards from the prev chunk, hence set
                 * lookback_it to that.
                 */
                _extent_left = bc->offset;
                AZLogVerbose("(tentative) _extent_left: {}", _extent_left);

                SET_LOOKBACK_IT_TO_PREV();
            }
        }
    } else {
        /*
         * There's at least one chunk having offset greater than the requested
         * chunk's offset (next_offset).
         *
         * it->first >= next_offset, we have two cases:
         * 1. (it.first == next_offset) => desired data starts from this chunk.
         * 2. (it.first > next_offset)  => desired data starts before this chunk.
         *                                 It may start within the prev chunk,
         *                                 or this chunk may start in the gap
         *                                 between the prev chunk and this chunk,
         *                                 in that case we need to create a new
         *                                 chunk before this chunk.
         */
        assert(it->first == it->second.offset);
        assert(it->first >= next_offset);

        if (it->first == next_offset) {
            bc = &(it->second);
            /*
             * Requested range starts from this chunk. Set _extent_left
             * tentatively to this chunk's left edge and set lookback_it
             * to the prev chunk for finding the true left edge later.
             * _extent_right will be set by the for loop and later updated
             * correctly.
             */
            _extent_left = it->first;
            AZLogVerbose("(tentative) _extent_left: {}", _extent_left);

            SET_LOOKBACK_IT_TO_PREV();
        } else {
            /*
             * Requested range starts before this chunk.
             */
            assert(it->first > next_offset);

            if (it == chunkmap.begin()) {
                /*
                 * If this is the first chunk then part or whole of the
                 * requested range lies before this chunk and we need to
                 * create a new chunk for that. For SCAN_ACTION_RELEASE
                 * we just ignore the part before this chunk.
                 */
                bc = &(it->second);
                assert(bc->offset > next_offset);

                /*
                 * Newly created chunk's offset and length.
                 * For the release case chunk_offset and chunk_length are not
                 * used but we must update remaining_length and next_offset to
                 * correctly track the "to-be-released" range.
                 */
                chunk_offset = next_offset;
                chunk_length = std::min(bc->offset - next_offset,
                                        remaining_length);

                remaining_length -= chunk_length;
                next_offset += chunk_length;

                if (action == scan_action::SCAN_ACTION_GET) {
                    /*
                     * This newly added chunk is the first chunk, so its offset
                     * is the left edge. We mark the right edge tentatively,
                     * it'll be confirmed after we look forward.
                     */
                    _extent_left = chunk_offset;
                    _extent_right = chunk_offset + chunk_length;
                    assert(lookback_it == chunkmap.end());

                    AZLogVerbose("_extent_left: {}", _extent_left);
                    AZLogVerbose("(tentative) _extent_right: {}", _extent_right);

                    chunkvec.emplace_back(this, chunk_offset, chunk_length);
                    AZLogVerbose("(new chunk) [{},{})",
                                 chunk_offset, chunk_offset + chunk_length);
                } else {
                    AZLogVerbose("<Release [{}, {})> (non-existent chunk) "
                                 "[{},{})",
                                 offset, offset + length,
                                 chunk_offset, chunk_offset + chunk_length);
                }
            } else {
                /*
                 * Requested range starts before this chunk and we have a
                 * chunk before this chunk.
                 */

                // This chunk (we need it later).
                auto itn = it;
                bytes_chunk *bcn = &(itn->second);
                assert(bcn->offset > next_offset);

                // Prev chunk.
                it = std::prev(it);
                bc = &(it->second);

                if ((bc->offset + bc->length) <= next_offset) {
                    /*
                     * Prev chunk ends before the 1st byte from the requested
                     * range. This means we need to allocate a chunk after the
                     * prev chunk. The new chunk size will be from next_offset
                     * till the start offset of the next chunk (bcn) or
                     * remaining_length whichever is smaller.
                     *
                     * For the release case chunk_offset and chunk_length are not
                     * used but we must update remaining_length and next_offset to
                     * correctly track the "to-be-released" range.
                     */
                    chunk_offset = next_offset;
                    chunk_length = std::min(bcn->offset - next_offset,
                                            remaining_length);

                    remaining_length -= chunk_length;
                    next_offset += chunk_length;

                    if (action == scan_action::SCAN_ACTION_GET) {
                        /*
                         * If this new chunk starts right after the prev chunk, then
                         * we don't know the actual value of _extent_left unless we
                         * scan left and check. In that case we set lookback_it to
                         * the prev chunk, so that we can later "look back" and find
                         * the left edge.
                         * If it doesn't start right after, then chunk_offset becomes
                         * _extent_left.
                         */
                        if ((bc->offset + bc->length) < next_offset) {
                            /*
                             * New chunk does not touch the prev chunk, so the new
                             * chunk offset is the _extent_left.
                             */
                            _extent_left = chunk_offset;
                            AZLogVerbose("_extent_left: {}", _extent_left);
                            assert(lookback_it == chunkmap.end());
                        } else {
                            _extent_left = chunk_offset;
                            AZLogVerbose("(tentative) _extent_left: {}", _extent_left);
                            /*
                             * Else, new chunk touches the prev chunk, so we need
                             * to "look back" for finding the left edge.
                             */
                            AZLogVerbose("lookback_it: [{},{})",
                                         bc->offset, bc->offset + bc->length);
                            lookback_it = it;
                        }

                        _extent_right = chunk_offset + chunk_length;
                        AZLogVerbose("(tentative) _extent_right: {}", _extent_right);

                        // Search for more chunks should start from the next chunk.
                        it = itn;

                        chunkvec.emplace_back(this, chunk_offset, chunk_length);
                        AZLogVerbose("(new chunk) [{},{})",
                                     chunk_offset, chunk_offset + chunk_length);
                    } else {
                        // Search for more chunks should start from the next chunk.
                        it = itn;

                        AZLogVerbose("<Release [{}, {})> (non-existent chunk) "
                                     "[{},{})",
                                     offset, offset + length,
                                     chunk_offset, chunk_offset + chunk_length);
                    }
                } else {
                    /*
                     * Prev chunk contains some bytes from initial part of the
                     * requested range. Set _extent_left tentative, the for loop
                     * below will set _extent_right correctly.
                     * Need to "look back" to find the true left edge and look
                     * forward to find the true right edge.
                     */
                    _extent_left = bc->offset;
                    AZLogVerbose("(tentative) _extent_left: {}", _extent_left);

                    SET_LOOKBACK_IT_TO_PREV();
                }
            }
        }
    }

    /*
     * _extent_left MUST be set for all cases that require us to traverse the
     * chunkmap. lookback_it may or may not be set depending on whether
     * _extent_left is tentative and we need to search backwards for the true
     * left edge.
     */
    if (action == scan_action::SCAN_ACTION_GET) {
        assert(_extent_left != AZNFSC_BAD_OFFSET);
    }

    /*
     * Now sequentially go over the remaining chunks till we cover the entire
     * requested range. For SCAN_ACTION_GET if some chunk doesn't exist, it'll
     * be allocated, while for SCAN_ACTION_GET non-existent chunks are ignored.
     */
    for (; remaining_length != 0 && it != chunkmap.end(); ) {
        bc = &(it->second);

        /*
         * For the GET and file-backed cache, make sure the requested chunk is
         * duly mmapped so that any IO that caller performs on the returned
         * bytes_chunk is served from the backing file.
         */
        if (action == scan_action::SCAN_ACTION_GET) {
            bc->load();
        }

        /*
         * next_offset must lie before the end of current chunk, else we should
         * not be inside the for loop.
         */
        assert(next_offset < (bc->offset + bc->length));

        chunk_offset = next_offset;

        if (next_offset == bc->offset) {
            /*
             * Our next offset of interest (next_offset) lies exactly at the
             * start of this chunk.
             */
            chunk_length = std::min(bc->length, remaining_length);
            assert(chunk_length > 0);

            if (action == scan_action::SCAN_ACTION_GET) {
                /*
                 * Starting offset of this request matches the bytes_chunk in
                 * the chunkmap, if length also matches then is_whole MUST
                 * be set.
                 */
                assert(chunk_offset == bc->offset);
                const bool is_whole = (chunk_length == bc->length);
                chunkvec.emplace_back(this, chunk_offset, chunk_length,
                                      bc->buffer_offset, bc->alloc_buffer,
                                      is_whole);
                AZLogVerbose("(existing chunk) [{},{}) b:{} a:{}",
                             chunk_offset, chunk_offset + chunk_length,
                             fmt::ptr(chunkvec.back().get_buffer()),
                             fmt::ptr(bc->alloc_buffer->get()));
            } else if (bc->safe_to_release()) {
                assert (action == scan_action::SCAN_ACTION_RELEASE);

                /*
                 * chunk_length bytes will be released.
                 */
                *bytes_released += chunk_length;

                if (chunk_length == bc->length) {
                    /*
                     * File-backed cache may not have the membuf allocated in
                     * case the cache is dropped. bc->get_buffer() will assert
                     * so avoid calling it.
                     */
                    AZLogVerbose("<Release [{}, {})> (releasing chunk) [{},{}) "
                                 "b:{} a:{}",
                                 offset, offset + length,
                               chunk_offset, chunk_offset + chunk_length,
                               bc->alloc_buffer->get() ?
                                    fmt::ptr(bc->get_buffer()) : nullptr,
                               fmt::ptr(bc->alloc_buffer->get()));
                    /*
                     * Queue the chunk for deletion, since the entire chunk is
                     * released.
                     */
                    if (begin_delete == chunkmap.end()) {
                        begin_delete = it;
                    }
                    /*
                     * Keep updating end_delete with every full chunk
                     * processed, that way in the end once we are done we will
                     * have end_delete correctly point to one past the last
                     * to-be-deleted chunk.
                     */
                    end_delete = std::next(it);
                } else {
                    assert(chunk_length == remaining_length);
                    /*
                     * Else trim the chunk (from the left).
                     */
                    AZLogVerbose("<Release [{}, {})> (trimming chunk from left) "
                                 "[{},{}) -> [{},{})",
                                 offset, offset + length,
                                 bc->offset, bc->offset + bc->length,
                                 bc->offset + chunk_length,
                                 bc->offset + bc->length);

                    bc->offset += chunk_length;
                    bc->buffer_offset += chunk_length;
                    bc->length -= chunk_length;

                    /*
                     * Don't update num_chunks/num_chunks_g as we remove one
                     * and add one chunk.
                     */
                    assert(bytes_cached >= chunk_length);
                    assert(bytes_cached_g >= chunk_length);
                    bytes_cached -= chunk_length;
                    bytes_cached_g -= chunk_length;

                    /*
                     * Since the key (offset) for this chunk changed, we need
                     * to remove and re-insert into the map (with the updated
                     * key/offset). For the buffer, it shall refer to the same
                     * buffer (albeit different offset) that the original chunk
                     * was using.
                     * Add the new chunk first before deleting the old chunk,
                     * else bc->alloc_buffer may get freed.
                     *
                     * This can only happen for the last chunk in the range and
                     * hence it's ok to update the chunkmap. We should exit the
                     * for loop here.
                     */
                    auto p = chunkmap.try_emplace(bc->offset, this, bc->offset,
                                                  bc->length, bc->buffer_offset,
                                                  bc->alloc_buffer);
                    assert(p.second);
                    /*
                     * Now that the older chunk is going and is being replaced
                     * by this chunk, if end_delete was pointing at the old
                     * chunk, change it to point to this new chunk. Note that
                     * the new chunk will be the next in line and hence we
                     * can safely replace end_delete with this.
                     */
                    if (it == end_delete) {
                        end_delete = p.first;
                    }

                    chunkmap.erase(it);
                    goto done;
                }
            } else {
                AZLogVerbose("<Release [{}, {})> skipping [{}, {}) as not safe "
                             "to release: inuse={}, dirty={}",
                             offset, offset + length,
                             chunk_offset, chunk_offset + chunk_length,
                             bc->get_membuf()->get_inuse(),
                             bc->get_membuf()->is_dirty());
            }

            // This chunk is fully consumed, move to the next chunk.
            ++it;
        } else if (next_offset < bc->offset) {
            /*
             * Our next offset of interest (next_offset) lies before the
             * next chunk. For SCAN_ACTION_GET we need to allocate a new
             * chunk, for SCAN_ACTION_RELEASE ignore this non-existent byte
             * range. We set chunk_length so that remaining_length and
             * next_offset are correctly updated at the end of the loop.
             */
            chunk_length = std::min(bc->offset - next_offset,
                                    remaining_length);

            if (action == scan_action::SCAN_ACTION_GET) {
                chunkvec.emplace_back(this, chunk_offset, chunk_length);
                AZLogVerbose("(new chunk) [{},{})",
                             chunk_offset, chunk_offset+chunk_length);
            } else {
                AZLogVerbose("<Release [{}, {})> (non-existent chunk) [{},{})",
                             offset, offset + length,
                             chunk_offset, chunk_offset + chunk_length);
            }

            /*
             * In the next iteration we need to look at the current chunk, so
             * don't increment the iterator.
             * We continue from here as we want to set _extent_right
             * differently than what we do at end-of-loop.
             */
            remaining_length -= chunk_length;
            assert((int64_t) remaining_length >= 0);
            next_offset += chunk_length;

            if (action == scan_action::SCAN_ACTION_GET) {
                _extent_right = next_offset;
                AZLogVerbose("(tentative) _extent_right: {}", _extent_right);
            }
            continue;
        } else /* (next_offset > bc->offset) */ {
            /*
             * Our next offset of interest (next_offset) lies within this
             * chunk.
             */
            chunk_length = std::min(bc->offset + bc->length - next_offset,
                                    remaining_length);
            assert(chunk_length > 0);

            if (action == scan_action::SCAN_ACTION_GET) {
                /*
                 * Returned bytes_chunk doesn't have the same starting offset
                 * as the bytes_chunk in the chunkmap, so is_whole MUST be
                 * set to false.
                 */
                chunkvec.emplace_back(this, chunk_offset, chunk_length,
                                      bc->buffer_offset + (next_offset - bc->offset),
                                      bc->alloc_buffer,
                                      false /* is_whole */);
                AZLogVerbose("(existing chunk) [{},{}) b:{} a:{}",
                             chunk_offset, chunk_offset + chunk_length,
                             fmt::ptr(chunkvec.back().get_buffer()),
                             fmt::ptr(bc->alloc_buffer->get()));
            } else if (bc->safe_to_release()) {
                assert(action == scan_action::SCAN_ACTION_RELEASE);
                assert(chunk_length <= remaining_length);

                /*
                 * We have two cases:
                 * 1. The released part lies at the end of the chunk, so we
                 *    can safely release by trimming this chunk from the right.
                 * 2. The released part lies in the middle with un-released
                 *    ranges before and after the released chunk. To duly
                 *    release it we need to trim the original chunk to contain
                 *    data before the released data and create a new chunk to
                 *    hold the data after the released data, and copy data from
                 *    the existing membuf into this new membuf. This ends up
                 *    being expensive and not practically useful. Note that the
                 *    reason for caller doing release() is that it wants the
                 *    membuf memory to be released, but in this case we are not
                 *    releasing data but instead allocating more data and
                 *    copying it. This becomes worse when caller makes small
                 *    small release() calls from middle of the membuf.
                 *    We choose to ignore such release() calls and not release
                 *    any range in this case.
                 */

                const uint64_t chunk_after_offset =
                    next_offset + chunk_length;
                const uint64_t chunk_after_length =
                    bc->offset + bc->length - chunk_after_offset;

                if (chunk_after_length == 0) {
                    assert(chunk_length ==
                           (bc->offset + bc->length - next_offset));

                    const uint64_t trim_bytes = chunk_length;

                    /*
                     * All chunk data after next_offset is released, trim the
                     * chunk.
                     */
                    AZLogVerbose("<Release [{}, {})> (trimming chunk from right) "
                                 "[{},{}) -> [{},{})",
                                 offset, offset + length,
                                 bc->offset, bc->offset + bc->length,
                                 bc->offset, next_offset);

                    bc->length = next_offset - bc->offset;
                    assert((int64_t) bc->length > 0);

                    assert(bytes_cached >= trim_bytes);
                    assert(bytes_cached_g >= trim_bytes);
                    bytes_cached -= trim_bytes;
                    bytes_cached_g -= trim_bytes;

                    /*
                     * chunk_length bytes are now released.
                     */
                    *bytes_released += trim_bytes;
                } else {
                    /*
                     * The to-be-released range must lie entirely within this
                     * chunk.
                     */
                    assert(offset == next_offset);
                    assert(length == remaining_length);

                    AZLogVerbose("<Release [{}, {})> skipping as it lies in the "
                                 "middle of the chunk [{},{})",
                                 offset, offset + length,
                                 bc->offset, bc->offset + bc->length);
                }
            } else {
                AZLogVerbose("<Release [{}, {})> skipping [{}, {}) as not safe "
                             "to release: inuse={}, dirty={}",
                             offset, offset + length,
                             chunk_offset, chunk_offset + chunk_length,
                           bc->get_membuf()->get_inuse(),
                           bc->get_membuf()->is_dirty());
            }

            // This chunk is fully consumed, move to the next chunk.
            ++it;
        }

done:
        remaining_length -= chunk_length;
        assert((int64_t) remaining_length >= 0);
        next_offset += chunk_length;

        /*
         * Once this for loop exits, the search for _extent_right continues
         * with 'it', so we must make sure that 'it' points to the next chunk
         * that we want to check. Note that we search for _extent_right only
         * for SCAN_ACTION_GET.
         */
        if (action == scan_action::SCAN_ACTION_GET) {
            _extent_right = bc->offset + bc->length;
            AZLogVerbose("(tentative) _extent_right: {}", _extent_right);
        }
    }

    /*
     * Allocate the only or the last chunk beyond the highest chunk we have
     * in our cache. For the SCAN_ACTION_RELEASE case we simply ignore whatever
     * to-be-released byte range remains after the last chunk.
     */
allocate_only_chunk:
    if (remaining_length != 0) {
        if (action == scan_action::SCAN_ACTION_GET) {
            AZLogVerbose("(only/last chunk) [{},{})",
                         next_offset, next_offset + remaining_length);

    #ifdef UTILIZE_TAILROOM_FROM_LAST_MEMBUF
            if (last_bc && (last_bc->tailroom() > 0)) {
                chunk_length = std::min(last_bc->tailroom(), remaining_length);

                AZLogVerbose("(sharing last chunk's alloc_buffer) [{},{})",
                             next_offset, next_offset + chunk_length);

                /*
                 * Since this new chunk is sharing alloc_buffer with the last
                 * chunk, is_new must be false.
                 * Also it's not referring to the entire membuf, so is_whole
                 * must be false.
                 */
                chunkvec.emplace_back(this, next_offset,
                                      chunk_length,
                                      last_bc->buffer_offset + last_bc->length,
                                      last_bc->alloc_buffer,
                                      false /* is_whole */,
                                      false /* is_new */);

                /*
                 * last chunk and this new chunk are sharing the same
                 * alloc_buffer.
                 */
                assert(last_bc->alloc_buffer.use_count() >= 2);

                remaining_length -= chunk_length;
                next_offset += chunk_length;
            }
    #endif

            if (remaining_length) {
                AZLogVerbose("(new last chunk) [{},{})",
                             next_offset, next_offset + remaining_length);
                chunkvec.emplace_back(this, next_offset, remaining_length);
            }

            remaining_length = 0;
        } else {
            AZLogVerbose("<Release [{}, {})> (non-existent chunk after end) "
                         "[{},{})",
                         offset, offset + length,
                         next_offset, next_offset + remaining_length);
            remaining_length = 0;
        }
    }

    /*
     * Insert the new chunks in the end.
     * We cannot do this inside the for loop above as it'll change the chunkmap
     * while we are traversing it.
     */
    for (const auto& chunk : chunkvec) {

        /*
         * All the membufs that we return to the caller, we increment the
         * inuse count for each of them. Once the caller is done using those
         * (writing application data by writers and reading blob data into it
         * by readers) they must decrease the inuse count by clear_inuse().
         * This is done to make sure a membuf is skipped by clear() if it has
         * ongoing IOs.
         */
        if (action == scan_action::SCAN_ACTION_GET) {
            chunk.alloc_buffer->set_inuse();
        }

        if (chunk.is_new) {
            // New chunk is always a whole chunk.
            assert(chunk.is_whole);
            assert(chunk.alloc_buffer->allocated_buffer != nullptr);
            assert(chunk.alloc_buffer->buffer >=
                   chunk.alloc_buffer->allocated_buffer);
            assert(chunk.alloc_buffer->length > 0);
            assert(chunk.alloc_buffer->allocated_length >=
                   chunk.alloc_buffer->length);

#ifndef UTILIZE_TAILROOM_FROM_LAST_MEMBUF
            /*
             * Empty bytes_chunk should only correspond to full membufs, but
             * not if we use tailroom from previous chunks to provide space
             * for new chunks added at the end.
             */
            assert(chunk.maps_full_membuf());
            assert(chunk.buffer_offset == 0);
            assert(chunk.length == chunk.alloc_buffer->length);
#endif

            /*
             * Other than when we are adding cache chunks, we should never come
             * here for allocating new chunk buffer.
             */
            assert(action == scan_action::SCAN_ACTION_GET);

            AZLogVerbose("(adding to chunkmap) [{},{})",
                         chunk.offset, chunk.offset + chunk.length);
            /*
             * This will grab a ref on the alloc_buffer allocated when we
             * added the chunk to chunkvec. On returning from this function
             * chunkvec will be destroyed and it'll release its reference,
             * so the chunkmap reference will be the only reference left.
             */
#ifndef NDEBUG
            auto p = chunkmap.try_emplace(chunk.offset, chunk.bcc, chunk.offset,
                                          chunk.length, chunk.buffer_offset,
                                          chunk.alloc_buffer);
            assert(p.second == true);
#else
            chunkmap.try_emplace(chunk.offset, chunk.bcc, chunk.offset,
                                 chunk.length, chunk.buffer_offset,
                                 chunk.alloc_buffer);
#endif
            // One more chunk added to chunkmap.
            num_chunks++;
            num_chunks_g++;
            bytes_cached_g += chunk.length;
            bytes_cached += chunk.length;

            /*
             * New chunks are always included in the extent range.
             */
            if ((chunk.offset + chunk.length) > _extent_right) {
                _extent_right = (chunk.offset + chunk.length);
                AZLogVerbose("(tentative) _extent_right: {}", _extent_right);
            }
        }
    }

    /*
     * Delete chunks in the range [begin_delete, end_delete).
     */
    if (action == scan_action::SCAN_ACTION_RELEASE) {
        uint64_t bytes_released_tmp = 0;

        if (begin_delete != chunkmap.end()) {
            for (auto _it = begin_delete, next_it = _it;
                 _it != end_delete; _it = next_it) {
                ++next_it;
                bc = &(_it->second);
                /*
                 * Not all chunks from begin_delete to end_delete are
                 * guaranteed safe-to-delete, so check before deleting.
                 */
                if (bc->safe_to_release()) {
                    AZLogVerbose("<Release [{}, {})> (freeing chunk) [{},{}) "
                                 "b:{} a:{}",
                                 offset, offset + length,
                                 bc->offset, bc->offset + bc->length,
                                 bc->alloc_buffer->get() ?
                                      fmt::ptr(bc->get_buffer()) : nullptr,
                                 fmt::ptr(bc->alloc_buffer->get()));

                    assert(num_chunks > 0);
                    num_chunks--;
                    assert(num_chunks_g > 0);
                    num_chunks_g--;

                    assert(bytes_cached >= bc->length);
                    assert(bytes_cached_g >= bc->length);
                    bytes_cached -= bc->length;
                    bytes_cached_g -= bc->length;

                    bytes_released_tmp += bc->length;

                    chunkmap.erase(_it);
                }
            }
        }

        assert(bytes_released_tmp <= *bytes_released);
    } else {
        assert((begin_delete == chunkmap.end()) &&
               (end_delete == chunkmap.end()));
    }

    if (find_extent) {
        /*
         * Set/update extent left edge.
         */
        if (lookback_it != chunkmap.end()) {
            do {
                bc = &(lookback_it->second);

                if ((_extent_left != AZNFSC_BAD_OFFSET) &&
                    ((bc->offset + bc->length) != _extent_left)) {
                    AZLogVerbose("(hit gap) _extent_left: {}, [{}, {})",
                                 _extent_left,
                                 bc->offset, (bc->offset + bc->length));
                    break;
                }

                if (!bc->needs_flush()) {
                    AZLogVerbose("(hit noflush) _extent_left: {}, [{}, {})",
                                 _extent_left,
                                 bc->offset, (bc->offset + bc->length));
                    break;
                }

                _extent_left = bc->offset;
                AZLogVerbose("_extent_left: {}", _extent_left);
            } while (lookback_it-- != chunkmap.begin());
        }

        /*
         * Set/update extent right edge.
         */
        for (; it != chunkmap.end(); ++it) {
            bc = &(it->second);

            if ((_extent_right != AZNFSC_BAD_OFFSET) &&
                (bc->offset != _extent_right)) {
                AZLogVerbose("(hit gap) _extent_right: {}, [{}, {})",
                             _extent_right,
                             bc->offset, (bc->offset + bc->length));
                break;
            }

            if (!bc->needs_flush()) {
                AZLogVerbose("(hit noflush) _extent_right: {}, [{}, {})",
                             _extent_right,
                             bc->offset, (bc->offset + bc->length));
                break;
            }

            _extent_right = bc->offset + bc->length;
            AZLogVerbose("_extent_right: {}", _extent_right);
        }

        *extent_left = _extent_left;
        *extent_right = _extent_right;
    }

end:
    return (action == scan_action::SCAN_ACTION_GET)
                ? chunkvec : std::vector<bytes_chunk>();
}

/*
 * TODO: Add pruning stats.
 */
void bytes_chunk_cache::inline_prune()
{
    uint64_t inline_bytes = 0;
    uint64_t pruned_bytes = 0;

    get_prune_goals(&inline_bytes, nullptr);

    // Inline pruning not needed.
    if (inline_bytes == 0) {
        return;
    }

    const std::unique_lock<std::mutex> _lock(chunkmap_lock_43);

    /*
     * Multiple fuse threads may get the prune goals and then all of them
     * will prune that much resulting in too much pruning, so fetch the prune
     * goals once after acquiring the lock.
     */
    get_prune_goals(&inline_bytes, nullptr);

    if (inline_bytes == 0) {
        return;
    }

    AZLogDebug("[{}] inline_prune(): Inline prune goal of {:0.2f} MB",
               fmt::ptr(this), inline_bytes / (1024 * 1024.0));

    uint32_t inuse = 0, dirty = 0, inra = 0;
    uint64_t inuse_bytes = 0, dirty_bytes = 0, inra_bytes = 0;

    for (auto it = chunkmap.cbegin(), next_it = it;
         (it != chunkmap.cend()) && (pruned_bytes < inline_bytes);
         it = next_it) {
        ++next_it;
        const struct bytes_chunk *bc = &(it->second);
        const struct membuf *mb = bc->get_membuf();

        /*
         * inode will be null only for testing.
         */
        assert(!inode || (inode->magic == NFS_INODE_MAGIC));
        if (inode && inode->in_ra_window(mb->offset, mb->length)) {
            AZLogDebug("[{}] inline_prune(): skipping as membuf(offset={}, "
                       "length={}) lies in RA window",
                       fmt::ptr(this), mb->offset, mb->length);
            inra++;
            inra_bytes += mb->allocated_length;
            continue;
        }

        /*
         * Possibly under IO.
         */
        if (mb->is_inuse()) {
            AZLogDebug("[{}] inline_prune(): skipping as membuf(offset={}, "
                       "length={}) is inuse (locked={}, dirty={}, flushing={}, "
                       "uptodate={})",
                       fmt::ptr(this), mb->offset, mb->length,
                       mb->is_locked() ? "yes" : "no",
                       mb->is_dirty() ? "yes" : "no",
                       mb->is_flushing() ? "yes" : "no",
                       mb->is_uptodate() ? "yes" : "no");
            inuse++;
            inuse_bytes += mb->allocated_length;
            continue;
        }

        /*
         * Not under use, cannot be locked.
         * Note that users are supposed to drop the inuse count only after
         * releasing the membuf lock.
         */
        assert(!mb->is_locked());

        /*
         * Has data to be written to Blob.
         * Cannot safely drop this from the cache.
         */
        if (mb->is_dirty()) {
            AZLogDebug("[{}] inline_prune(): skipping as membuf(offset={}, "
                       "length={}) is dirty (flushing={}, uptodate={})",
                       fmt::ptr(this), mb->offset, mb->length,
                       mb->is_flushing() ? "yes" : "no",
                       mb->is_uptodate() ? "yes" : "no");
            dirty++;
            dirty_bytes += mb->allocated_length;
            continue;
        }

        AZLogDebug("[{}] inline_prune(): deleting membuf(offset={}, length={})",
                   fmt::ptr(this), mb->offset, mb->length);

        /*
         * Release the chunk.
         * This will release the membuf (munmap() it in case of file-backed
         * cache and delete it for heap backed cache). At this point the membuf
         * is guaranteed to be not in use since we checked the inuse count
         * above.
         */
        assert(num_chunks > 0);
        num_chunks--;
        assert(num_chunks_g > 0);
        num_chunks_g--;

        assert(bytes_cached >= bc->length);
        assert(bytes_cached_g >= bc->length);
        bytes_cached -= bc->length;
        bytes_cached_g -= bc->length;

        pruned_bytes += mb->allocated_length;

        chunkmap.erase(it);
    }

    if (pruned_bytes < inline_bytes) {
        AZLogDebug("Could not meet inline prune goal, pruned {} of {} bytes "
                   "[inuse={}/{}, dirty={}/{}, inra={}/{}]",
                   pruned_bytes, inline_bytes,
                   inuse, inuse_bytes,
                   dirty, dirty_bytes,
                   inra, inra_bytes);
    } else {
        AZLogDebug("Successfully pruned {} bytes [inuse={}/{}, dirty={}/{}, "
                   "inra={}/{}]",
                   pruned_bytes,
                   inuse, inuse_bytes,
                   dirty, dirty_bytes,
                   inra, inra_bytes);
    }
}

int64_t bytes_chunk_cache::drop(uint64_t offset, uint64_t length)
{
    if (backing_file_name.empty()) {
        // No-op for non file-backed caches.
        return 0;
    }

    const std::unique_lock<std::mutex> _lock(chunkmap_lock_43);

    /*
     * Find chunk with offset >= next_offset. Note that we only drop caches
     * for chunks which completely lie in the range, i.e., partial chunks are
     * skipped. This is ok as dropping caches is only for saving memory and
     * not doing it doesn't cause correctness isssues.
     */
    auto it = chunkmap.lower_bound(offset);

    // No full chunk lies in the given range.
    if (it == chunkmap.end()) {
        return 0;
    }

    uint64_t remaining_length = length;
    int64_t total_dropped_bytes = 0;

    /*
     * Iterate over all chunks and drop chunks that completely lie in the
     * requested range.
     */
    for (; remaining_length != 0 && it != chunkmap.end(); ++it) {
        bytes_chunk *bc = &(it->second);

        if (remaining_length < bc->length) {
            break;
        }

        /*
         * This will not drop the cache if the membuf is being referenced
         * by some other user (other than the original chunkmap reference).
         */
        const int64_t dropped_bytes = bc->drop();
        if (dropped_bytes > 0) {
            total_dropped_bytes += dropped_bytes;
        }

        remaining_length -= bc->length;
    }

    return total_dropped_bytes;
}

/**
 * Caller MUST hold exclusive lock on chunkmap_lock_43.
 */
void bytes_chunk_cache::clear_nolock()
{
    AZLogDebug("[{}] Cache purge: chunkmap.size()={}, backing_file_name={}",
               fmt::ptr(this), chunkmap.size(), backing_file_name);

    assert(bytes_allocated <= bytes_allocated_g);
    assert(bytes_cached <= bytes_cached_g);

    /*
     * We go over all the bytes_chunk to see if they can be freed. Following
     * bytes_chunk cannot be freed:
     * 1. If it's marked dirty, i.e., it has data which needs to be sync'ed to
     *    the Blob. This is application data which need to be written to the
     *    Blob and freeing the bytes_chunk w/o that will cause data consistency
     *    issues as we have already completed these writes to the application.
     * 2. If it's locked, i.e., it currently has some IO ongoing. If the
     *    ongoing IO is reading data from Blob into the cache, we actually
     *    do not care, but if the lock is held for writing application data
     *    into the membuf then we cannot free it.
     *
     * Since bytes_chunk_cache::get() increases the inuse count of all membufs
     * returned, and it does that while holding the bytes_chunk_cache::lock, we
     * can safely remove from chunkmap iff inuse/dirty/locked are not set.
     */
    const uint64_t start_size = chunkmap.size();

    for (auto it = chunkmap.cbegin(), next_it = it;
         it != chunkmap.cend();
         it = next_it) {
        ++next_it;
        const struct bytes_chunk *bc = &(it->second);
        const struct membuf *mb = bc->get_membuf();

        /*
         * Possibly under IO.
         * It could be writer writing application data into the membuf, or
         * reader reading Blob data into the membuf. For the read case we don't
         * really care but we cannot distinguish between the two.
         *
         * TODO: Currently this means we also don't invalidate membufs which
         *       may be fetched for read. Technically these shouldn't be
         *       skipped.
         */
        if (mb->is_inuse()) {
            AZLogDebug("[{}] Cache purge: skipping inuse membuf(offset={}, "
                       "length={}) (inuse count={}, dirty={})",
                       fmt::ptr(this), mb->offset, mb->length,
                       mb->get_inuse(), mb->is_dirty());
            continue;
        }

        /*
         * Usually inuse count is dropped after the lock so if inuse count
         * is zero membuf must not be locked, but users who may want to
         * release() some chunk while holding the lock may drop their inuse
         * count to allow release() to release the bytes_chunk.
         */
        if (mb->is_locked()) {
            AZLogDebug("[{}] Cache purge: skipping locked membuf(offset={}, "
                       "length={}) (inuse count={}, dirty={})",
                       fmt::ptr(this), mb->offset, mb->length,
                       mb->get_inuse(), mb->is_dirty());
            continue;
        }

        /*
         * Has data to be written to Blob.
         * Cannot safely drop this from the cache.
         */
        if (mb->is_dirty()) {
            AZLogDebug("[{}] Cache purge: skipping dirty membuf(offset={}, "
                       "length={})",
                       fmt::ptr(this), mb->offset, mb->length);
            continue;
        }

        AZLogDebug("[{}] Cache purge: deleting membuf(offset={}, length={}), "
                   "use_count={}, deleted {} of {}",
                   fmt::ptr(this), mb->offset, mb->length,
                   bc->get_membuf_usecount(),
                   start_size - chunkmap.size(), start_size);

        // Make sure the compound check also passes.
        assert(bc->safe_to_release());

        /*
         * Release the chunk.
         * This will release the membuf (munmap() it in case of file-backed
         * cache and delete it for heap backed cache). At this point the membuf
         * is guaranteed to be not in use since we checked the inuse count
         * above.
         */
        assert(num_chunks > 0);
        num_chunks--;
        assert(num_chunks_g > 0);
        num_chunks_g--;

        assert(bytes_cached >= bc->length);
        assert(bytes_cached_g >= bc->length);
        bytes_cached -= bc->length;
        bytes_cached_g -= bc->length;

        chunkmap.erase(it);
    }

    if (!chunkmap.empty()) {
        AZLogDebug("[{}] Cache purge: Skipping delete for backing_file_name={}, "
                   "as chunkmap not empty (still present {} of {})",
                   fmt::ptr(this), backing_file_name,
                   chunkmap.size(), start_size);
        assert(bytes_allocated > 0);
        return;
    }

    /*
     * Entire cache is purged, bytes_cached and bytes_allocated must drop to 0.
     *
     * Note: If some caller is still holding a bytes_chunk reference, the
     *       membuf will not be freed and hence bytes_allocated won't drop to 0.
     *       But, since we allow clear() only when inuse is 0, technically we
     *       shouldn't have any such user.
     *
     *       XXX Even though we allow clear() only when inuse is 0, it's
     *           possible that the caller has dropped the inuse ref but is
     *           still holding on to the bytes_chunk/membuf, which will cause
     *           bytes_chunk to be removed from the chunkmap but the membuf
     *           will still not be freed, causing bytes_allocated to not drop
     *           to 0. f.e., rpc_task::bc_vec holds bytes_chunk references but
     *           we may drop inuse when read completes.
     */
    assert(bytes_cached == 0);

    if (bytes_allocated != 0) {
        AZLogWarn("[{}] Cache purge: bytes_allocated is still {}, some user "
                  "is still holding on to the bytes_chunk/membuf even after "
                  "dropping the inuse count: backing_file_name={}",
                  fmt::ptr(this), bytes_allocated.load(), backing_file_name);
#if 0
        assert(0);
#endif
    }

    /*
     * If all chunks are released, delete the backing file in case of
     * file-backed caches.
     */
    if (backing_file_fd != -1) {
        const int ret = ::close(backing_file_fd);
        if (ret != 0) {
            AZLogError("Cache purge: close(fd={}) failed: {}",
                    backing_file_fd, strerror(errno));
            assert(0);
        } else {
            AZLogDebug("Cache purge: Backing file {} closed, fd={}",
                       backing_file_name, backing_file_fd);
        }
        backing_file_fd = -1;
        backing_file_len = 0;
    }

    assert(backing_file_len == 0);

    if (!backing_file_name.empty()) {
        const int ret = ::unlink(backing_file_name.c_str());
        if ((ret != 0) && (errno != ENOENT)) {
            AZLogError("Cache purge: unlink({}) failed: {}",
                       backing_file_name, strerror(errno));
            assert(0);
        } else {
            AZLogDebug("Backing file {} deleted", backing_file_name);
        }
    }
}

std::vector<bytes_chunk> bytes_chunk_cache::get_dirty_bc_range(uint64_t start_off, uint64_t end_off) const
{
    std::vector<bytes_chunk> bc_vec;

    // TODO: Make it shared lock.
    const std::unique_lock<std::mutex> _lock(chunkmap_lock_43);
    auto it = chunkmap.lower_bound(start_off);

    while (it != chunkmap.cend() && it->first <= end_off) {
        const struct bytes_chunk& bc = it->second;
        struct membuf *mb = bc.get_membuf();

        if (mb->is_dirty()) {
            mb->set_inuse();
            bc_vec.emplace_back(bc);
        }

        ++it;
    }

    return bc_vec;
}

#ifdef DEBUG_FILE_CACHE
static bool is_read()
{
    // 60:40 R:W.
    return random_number(0, 100) <= 60;
}

static void cache_read(bytes_chunk_cache& cache,
                       uint64_t offset,
                       uint64_t length)
{
    std::vector<bytes_chunk> v;

    AZLogVerbose("=====> cache_read({}, {})", offset, offset+length);
    v = cache.get(offset, length);
    // At least one chunk.
    assert(v.size() >= 1);
    assert(v[0].offset == offset);

    // Sanitize the returned chunkvec.
    uint64_t prev_chunk_right_edge = AZNFSC_BAD_OFFSET;
    uint64_t total_length = 0;

    for ([[maybe_unused]] const auto& e : v) {
        assert(e.length > 0);
        assert(e.length <= AZNFSC_MAX_CHUNK_SIZE);

        total_length += e.length;

        // Chunks must be contiguous.
        if (prev_chunk_right_edge != AZNFSC_BAD_OFFSET) {
            assert(e.offset == prev_chunk_right_edge);
        }
        prev_chunk_right_edge = e.offset + e.length;

        /*
         * All membufs MUST be returned with inuse incremented.
         */
        assert(e.get_membuf()->is_inuse());
        e.get_membuf()->clear_inuse();
    }

    assert(total_length == length);

    AZLogVerbose("=====> cache_read({}, {}): vec={}",
               offset, offset+length, v.size());
}

static void cache_write(bytes_chunk_cache& cache,
                        uint64_t offset,
                        uint64_t length)
{
    std::vector<bytes_chunk> v;
    uint64_t l, r;

    AZLogVerbose("=====> cache_write({}, {})", offset, offset+length);
    v = cache.getx(offset, length, &l, &r);
    // At least one chunk.
    assert(v.size() >= 1);
    assert(v[0].offset == offset);
    assert(l <= v[0].offset);

    // Sanitize the returned chunkvec.
    uint64_t prev_chunk_right_edge = AZNFSC_BAD_OFFSET;
    uint64_t total_length = 0;

    for ([[maybe_unused]] const auto& e : v) {
        assert(e.length > 0);
        assert(e.length <= AZNFSC_MAX_CHUNK_SIZE);

        total_length += e.length;

        // Chunks must be contiguous.
        if (prev_chunk_right_edge != AZNFSC_BAD_OFFSET) {
            assert(e.offset == prev_chunk_right_edge);
        }
        prev_chunk_right_edge = e.offset + e.length;
        assert(r >= prev_chunk_right_edge);

        /*
         * All membufs MUST be returned with inuse incremented.
         */
        assert(e.get_membuf()->is_inuse());
        e.get_membuf()->clear_inuse();
    }

    assert(total_length == length);

    AZLogVerbose("=====> cache_write({}, {}): l={} r={} vec={}",
                 offset, offset+length, l, r, v.size());
    AZLogVerbose("=====> cache_release({}, {})", offset, offset+length);
    assert(cache.release(offset, length) <= length);
}

/* static */
int bytes_chunk_cache::unit_test()
{
    assert(::sysconf(_SC_PAGESIZE) == PAGE_SIZE);

    /*
     * Choose file-backed or non file-backed cache for testing.
     * For file-backed cache, make sure /tmp as sufficient space.
     */
#if 1
    bytes_chunk_cache cache(nullptr);
#else
    bytes_chunk_cache cache(nullptr, "/tmp/bytes_chunk_cache");
#endif

    std::vector<bytes_chunk> v;
    uint64_t l, r;
    /*
     * Sometimes we want to validate that a bytes_chunk returned at a later
     * point refers to a chunk allocated earlier. We use these temp bytes_chunk
     * for that. Note that bytes_chunk can be deleted by calls to release(),
     * and calls to dropall() may drop the buffer mappings, so might need to
     * load() before we can use the buffers.
     */
    bytes_chunk bc, bc1, bc2, bc3;
    [[maybe_unused]] uint8_t *buffer;

#define ASSERT_NEW(chunk, start, end) \
do { \
    assert(chunk.offset == start); \
    assert(chunk.length == end-start); \
    assert(chunk.is_new); \
    assert(chunk.is_whole); \
    if (cache.is_file_backed()) { \
        assert(chunk.get_membuf()->buffer >= \
               chunk.get_membuf()->allocated_buffer); \
        assert(chunk.get_membuf()->allocated_length >= \
               chunk.get_membuf()->length); \
    } else { \
        assert(chunk.get_membuf()->allocated_buffer == \
               chunk.get_membuf()->buffer); \
        assert(chunk.get_membuf()->allocated_length == \
               chunk.get_membuf()->length); \
    } \
    assert((uint64_t) (chunk.get_membuf()->buffer - \
                chunk.get_membuf()->allocated_buffer) == \
           (chunk.get_membuf()->allocated_length - \
                chunk.get_membuf()->length)); \
    assert(chunk.bcc->bytes_cached >= chunk.length); \
    assert(chunk.bcc->bytes_cached_g >= chunk.bcc->bytes_cached); \
    /* All membufs MUST be returned with inuse incremented */ \
    assert(chunk.get_membuf()->is_inuse()); \
    chunk.get_membuf()->clear_inuse(); \
} while (0)

#define ASSERT_EXISTING(chunk, start, end) \
do { \
    assert(chunk.offset == start); \
    assert(chunk.length == end-start); \
    assert(!(chunk.is_new)); \
    if (cache.is_file_backed()) { \
        assert(chunk.get_membuf()->buffer >= \
               chunk.get_membuf()->allocated_buffer); \
        assert(chunk.get_membuf()->allocated_length >= \
               chunk.get_membuf()->length); \
    } else { \
        assert(chunk.get_membuf()->allocated_buffer == \
               chunk.get_membuf()->buffer); \
        assert(chunk.get_membuf()->allocated_length == \
               chunk.get_membuf()->length); \
    } \
    assert((uint64_t) (chunk.get_membuf()->buffer - \
                chunk.get_membuf()->allocated_buffer) == \
           (chunk.get_membuf()->allocated_length - \
                chunk.get_membuf()->length)); \
    assert(chunk.bcc->bytes_cached >= chunk.length); \
    assert(chunk.bcc->bytes_cached_g >= chunk.bcc->bytes_cached); \
    /* All membufs MUST be returned with inuse incremented */ \
    assert(chunk.get_membuf()->is_inuse()); \
    chunk.get_membuf()->clear_inuse(); \
} while (0)

#define ASSERT_EXTENT(left, right) \
do { \
    assert(l == left); \
    assert(r == right); \
} while (0)

#define ASSERT_DROPALL() \
do { \
    /* get all chunks and calculate total allocated bytes */ \
    uint64_t total_allocated_bytes = 0; \
    uint64_t total_bytes = 0; \
    for ([[maybe_unused]] const auto& e : cache.chunkmap) { \
        total_allocated_bytes += e.second.get_membuf()->allocated_length; \
        total_bytes += e.second.get_membuf()->length; \
    } \
    [[maybe_unused]] const uint64_t total_dropped_bytes = cache.dropall(); \
    if (cache.is_file_backed()) { \
        /* For file-backed caches all allocated bytes must be dropped */ \
        assert(total_dropped_bytes == total_allocated_bytes); \
    } else { \
        /* For memory-backed caches drop should be a no-op */ \
        assert(total_dropped_bytes == 0); \
    } \
    /* \
     * drop() should not change length and allocated_length, but it should
     * set allocated_buffer and buffer to nullptr.
     */ \
    uint64_t total_allocated_bytes1 = 0; \
    uint64_t total_bytes1 = 0; \
    for ([[maybe_unused]] const auto& e : cache.chunkmap) { \
        if (cache.is_file_backed()) { \
            assert(e.second.get_membuf()->allocated_buffer == nullptr); \
            assert(e.second.get_membuf()->buffer == nullptr); \
        } else { \
            assert(e.second.get_membuf()->allocated_buffer != nullptr); \
            assert(e.second.get_membuf()->buffer != nullptr); \
        } \
        total_allocated_bytes1 += e.second.get_membuf()->allocated_length; \
        total_bytes1 += e.second.get_membuf()->length; \
    } \
    assert(total_bytes1 == total_bytes); \
    assert(total_allocated_bytes1 == total_allocated_bytes); \
} while (0);

#define PRINT_CHUNK(chunk) \
do { \
    assert(chunk.length > 0); \
    AZLogInfo("[{},{}){}{} <{}> use_count={}, flag=0x{:x}", chunk.offset,\
              chunk.offset + chunk.length,\
              chunk.is_new ? " [New]" : "", \
              chunk.is_whole ? " [Whole]" : "", \
              fmt::ptr(chunk.get_buffer()), \
              chunk.get_membuf_usecount(), \
              chunk.get_membuf()->get_flag()); \
} while (0)

#define PRINT_CHUNKMAP() \
    AZLogInfo("==== [{}] chunkmap start [a:{} c:{}] ====", \
              __LINE__, cache.bytes_allocated.load(), cache.bytes_cached.load()); \
    for (auto& e : cache.chunkmap) { \
        /* mmap() just in case drop was called prior to this */ \
        e.second.load(); \
        PRINT_CHUNK(e.second); \
    } \
    AZLogInfo("==== chunkmap end ====");

    /*
     * Get cache chunks covering range [0, 300).
     * Since the cache is empty, it'll add a new empty chunk and return that.
     * The newly added chunk is also the largest contiguous block containing
     * the chunk.
     */
    AZLogInfo("========== [Get] --> (0, 300) ==========");
    v = cache.getx(0, 300, &l, &r);
    assert(v.size() == 1);

    ASSERT_EXTENT(0, 300);
    ASSERT_NEW(v[0], 0, 300);
    /*
     * This bytes_chunk later gets deleted by the call to release(200,100),
     * so we store the buffer.
     */
    buffer = v[0].get_buffer();

    for ([[maybe_unused]] const auto& e : v) {
        PRINT_CHUNK(e);
    }
    PRINT_CHUNKMAP();

    /*
     * Release data range [0, 100).
     * After this the cache should have chunk [100, 300).
     */
    AZLogInfo("========== [Release] --> (0, 100) ==========");
    assert(cache.release(0, 100) == 100);

    /*
     * Release data range [200, 300).
     * After this the cache should have chunk [100, 200).
     */
    AZLogInfo("========== [Release] --> (200, 100) ==========");
    assert(cache.release(200, 100) == 100);

    /*
     * Get cache chunks covering range [100, 200).
     * This will return the (only) existing chunk.
     * The newly added chunk is also the largest contiguous block containing
     * the chunk.
     */
    AZLogInfo("========== [Get] --> (100, 100) ==========");
    v = cache.getx(100, 100, &l, &r);
    assert(v.size() == 1);

    ASSERT_EXTENT(100, 200);
    ASSERT_EXISTING(v[0], 100, 200);
    assert(v[0].get_buffer() == (buffer + 100));
    assert(v[0].is_whole);

    for ([[maybe_unused]] const auto& e : v) {
        PRINT_CHUNK(e);
    }
    PRINT_CHUNKMAP();

    /*
     * Get cache chunks covering range [50, 150).
     * This should return 2 chunks:
     * 1. Newly allocated chunk [50, 100).
     * 2. Existing chunk data from [100, 150).
     *
     * The largest contiguous block containing the requested chunk is [50, 200).
     */
    AZLogInfo("========== [Get] --> (50, 100) ==========");
    v = cache.getx(50, 100, &l, &r);
    assert(v.size() == 2);

    ASSERT_EXTENT(50, 200);
    ASSERT_NEW(v[0], 50, 100);
    ASSERT_EXISTING(v[1], 100, 150);
    assert(v[1].get_buffer() == (buffer + 100));
    assert(!v[1].is_whole);

    for ([[maybe_unused]] const auto& e : v) {
        PRINT_CHUNK(e);
    }
    PRINT_CHUNKMAP();

    /*
     * Need to clear the vector before dropall, else drop won't drop as
     * bytes_chunk will have more than 1 use_count.
     */
    AZLogInfo("========== [Dropall] ==========");
    v.clear();
    ASSERT_DROPALL();

    /*
     * Get cache chunks covering range [250, 300).
     * This should return 1 chunk:
     * 1. Newly allocated chunk [250, 300).
     *
     * The largest contiguous block containing the requested chunk is [250, 300).
     */
    AZLogInfo("========== [Get] --> (250, 50) ==========");
    v = cache.getx(250, 50, &l, &r);
    assert(v.size() == 1);

    ASSERT_EXTENT(250, 300);
    ASSERT_NEW(v[0], 250, 300);
    bc = v[0];

    for ([[maybe_unused]] const auto& e : v) {
        PRINT_CHUNK(e);
    }
    PRINT_CHUNKMAP();

    /*
     * Get cache chunks covering range [50, 200).
     * This should return 1 chunk:
     * 1. Existing chunk [50, 100).
     * 2. Existing chunk [100, 200).
     *
     * The largest contiguous block containing the requested chunk is [50, 200).
     */
    AZLogInfo("========== [Get] --> (50, 150) ==========");
    v = cache.getx(50, 150, &l, &r);
    assert(v.size() == 2);

    ASSERT_EXTENT(50, 200);
    ASSERT_EXISTING(v[0], 50, 100);
    ASSERT_EXISTING(v[1], 100, 200);
    v[0].get_membuf()->set_inuse();
    v[0].get_membuf()->set_locked();
    v[0].get_membuf()->set_uptodate();
    v[0].get_membuf()->set_dirty();
    v[0].get_membuf()->clear_locked();
    v[0].get_membuf()->clear_inuse();
    assert(v[0].needs_flush());

    PRINT_CHUNKMAP();

    /*
     * Get cache chunks covering range [0, 50).
     * This should return 1 chunk:
     * 1. Newly allocated chunk [0, 50).
     *
     * The largest contiguous block containing the requested chunk is [0, 100).
     * [0, 50) is included in the extent range since that contains the data
     * just written by user.
     * [50, 100) is included in the extent range as the membuf is dirty (marked
     * above).
     * [100, 200) though contiguous, it's not included in the extent range as
     * needs_flush() is not true for it.
     */
    AZLogInfo("========== [Get] --> (0, 50) ==========");
    v = cache.getx(0, 50, &l, &r);
    assert(v.size() == 1);

    ASSERT_EXTENT(0, 100);
    ASSERT_NEW(v[0], 0, 50);

    for ([[maybe_unused]] const auto& e : v) {
        PRINT_CHUNK(e);
    }
    PRINT_CHUNKMAP();

    /*
     * Get cache chunks covering range [150, 275).
     * This should return following chunks:
     * 1. Existing chunk [150, 200).
     * 2. Newly allocated chunk [200, 250).
     * 3. Existing chunk [250, 275).
     *
     * The largest contiguous block containing the requested chunk is [50, 300).
     * [50, 100) is included in the extent range as the membuf is dirty (marked
     * above).
     * [100, 200) is included in the extent range since that partly contains the
     * data just written by user.
     * [200, 250) is included in the extent range since that fully contains the
     * data just written by user.
     * [250, 300) is included in the extent range since that partly contains the
     * data just written by user.
     */
    AZLogInfo("========== [Get] --> (150, 125) ==========");
    v = cache.getx(150, 125, &l, &r);
    assert(v.size() == 3);

    ASSERT_EXTENT(50, 300);
    ASSERT_EXISTING(v[0], 150, 200);
    ASSERT_NEW(v[1], 200, 250);
    ASSERT_EXISTING(v[2], 250, 275);
    assert(v[2].get_buffer() == bc.get_buffer());
    bc1 = v[0];
    bc2 = v[1];

    for ([[maybe_unused]] const auto& e : v) {
        PRINT_CHUNK(e);
    }
    PRINT_CHUNKMAP();

    /*
     * Cannot call ASSERT_DROPALL() here as that asserts that we drop all
     * chunks, but since we hold extra refs to chunks we won't drop all.
     */
    AZLogInfo("========== [Dropall] ==========");
    v.clear();
    cache.dropall();

    // Reload all bytes_chunk, after dropall().
    bc.load();
    bc1.load();
    bc2.load();

    /*
     * Get cache chunks covering range [0, 300).
     * This is all the chunks and should return following chunks:
     * 1. Existing chunk [0, 50).
     * 2. Existing chunk [50, 100).
     * 3. Existing chunk [100, 200).
     * 4. Existing chunk [200, 250).
     * 5. Existing chunk [250, 300).
     *
     * Clear dirty flag from [50, 100) to allow the release() below to release
     * it.
     */
    AZLogInfo("========== [Get] --> (0, 300) ==========");
    v = cache.getx(0, 300, &l, &r);
    assert(v.size() == 5);

    ASSERT_EXTENT(0, 300);
    ASSERT_EXISTING(v[0], 0, 50);
    ASSERT_EXISTING(v[1], 50, 100);
    ASSERT_EXISTING(v[2], 100, 200);
    ASSERT_EXISTING(v[3], 200, 250);
    ASSERT_EXISTING(v[4], 250, 300);
    PRINT_CHUNKMAP();
    // Clear dirty.
    v[1].get_membuf()->set_inuse();
    v[1].get_membuf()->set_locked();
    v[1].get_membuf()->set_flushing();
    v[1].get_membuf()->clear_dirty();
    v[1].get_membuf()->clear_flushing();
    v[1].get_membuf()->clear_locked();
    v[1].get_membuf()->clear_inuse();
    assert(!v[1].needs_flush());

    /*
     * Release data range [0, 175).
     * After this the cache should have the following chunk:
     * 1. [175, 200).
     * 2. [200, 250).
     * 3. [250, 300).
     */
    AZLogInfo("========== [Release] --> (0, 175) ==========");
    assert(cache.release(0, 175) == 175);

    /*
     * Get cache chunks covering range [100, 280).
     * This should return following chunks:
     * 1. Newly allocated chunk [100, 175).
     * 2. Existing chunk [175, 200).
     * 3. Existing chunk [200, 250).
     * 4. Existing chunk [250, 280).
     *
     * The largest contiguous block containing the requested chunk is [100, 300).
     */
    AZLogInfo("========== [Get] --> (100, 180) ==========");
    v = cache.getx(100, 180, &l, &r);
    assert(v.size() == 4);

    ASSERT_EXTENT(100, 300);
    ASSERT_NEW(v[0], 100, 175);
    ASSERT_EXISTING(v[1], 175, 200);
    assert(v[1].get_buffer() == (bc1.get_buffer() + 25));
    ASSERT_EXISTING(v[2], 200, 250);
    assert(v[2].get_buffer() == bc2.get_buffer());
    ASSERT_EXISTING(v[3], 250, 280);
    assert(v[3].get_buffer() == bc.get_buffer());
    bc3 = v[0];

    for ([[maybe_unused]] const auto& e : v) {
        PRINT_CHUNK(e);
    }
    PRINT_CHUNKMAP();

    /*
     * Get cache chunks covering range [0, 350).
     * This should return following chunks:
     * 1. Newly allocated chunk [0, 100).
     * 2. Existing chunk [100, 175).
     * 3. Existing chunk [175, 200).
     * 4. Existing chunk [200, 250).
     * 5. Existing chunk [250, 300).
     * 6. Newly allocated chunk [300, 350).
     *
     * The largest contiguous block containing the requested chunk is [0, 350).
     */
    AZLogInfo("========== [Get] --> (0, 350) ==========");
    v = cache.getx(0, 350, &l, &r);
    assert(v.size() == 6);

    ASSERT_EXTENT(0, 350);
    ASSERT_NEW(v[0], 0, 100);
    ASSERT_EXISTING(v[1], 100, 175);
    assert(v[1].get_buffer() == bc3.get_buffer());
    ASSERT_EXISTING(v[2], 175, 200);
    assert(v[2].get_buffer() == (bc1.get_buffer() + 25));
    ASSERT_EXISTING(v[3], 200, 250);
    assert(v[3].get_buffer() == bc2.get_buffer());
    ASSERT_EXISTING(v[4], 250, 300);
    assert(v[4].get_buffer() == bc.get_buffer());
    ASSERT_NEW(v[5], 300, 350);
    bc1 = v[0];
    bc3 = v[5];

    for ([[maybe_unused]] const auto& e : v) {
        PRINT_CHUNK(e);
    }
    PRINT_CHUNKMAP();

    /*
     * Release data range [50, 225).
     * After this the cache should have the following chunks:
     * 1. [0, 50).
     * 2. [225, 250).
     * 3. [250, 300).
     * 4. [300, 350).
     */
    AZLogInfo("========== [Release] --> (50, 175) ==========");
    assert(cache.release(50, 175) == 175);

    /*
     * Get cache chunks covering range [0, 325).
     * This should return following chunks:
     * 1. Existing chunk [0, 50).
     * 2. Newly allocated chunk [50, 225).
     * 3. Existing chunk [225, 250).
     * 4. Existing chunk [250, 300).
     * 5. Existing chunk [300, 325).
     *
     * The largest contiguous block containing the requested chunk is [0, 350).
     */
    AZLogInfo("========== [Get] --> (0, 325) ==========");
    v = cache.getx(0, 325, &l, &r);
    assert(v.size() == 5);

    ASSERT_EXTENT(0, 350);
    ASSERT_EXISTING(v[0], 0, 50);
    assert(v[0].get_buffer() == bc1.get_buffer());
    ASSERT_NEW(v[1], 50, 225);
    ASSERT_EXISTING(v[2], 225, 250);
    assert(v[2].get_buffer() == (bc2.get_buffer() + 25));
    ASSERT_EXISTING(v[3], 250, 300);
    assert(v[3].get_buffer() == bc.get_buffer());
    ASSERT_EXISTING(v[4], 300, 325);
    assert(v[4].get_buffer() == bc3.get_buffer());

    for ([[maybe_unused]] const auto& e : v) {
        PRINT_CHUNK(e);
    }
    PRINT_CHUNKMAP();

    /*
     * Release data range [0, 349).
     * After this the cache should have the following chunks:
     * 1. [349, 350).
     */
    AZLogInfo("========== [Release] --> (0, 349) ==========");
    assert(cache.release(0, 349) == 349);

    /*
     * Get cache chunks covering range [349, 350).
     * This should return following chunks:
     * 1. Existing chunk [349, 350).
     *
     * The largest contiguous block containing the requested chunk is [349, 350).
     */
    AZLogInfo("========== [Get] --> (349, 1) ==========");
    v = cache.getx(349, 1, &l, &r);
    assert(v.size() == 1);

    ASSERT_EXTENT(349, 350);
    ASSERT_EXISTING(v[0], 349, 350);
    assert(v[0].get_buffer() == (bc3.get_buffer() + 49));

    for ([[maybe_unused]] const auto& e : v) {
        PRINT_CHUNK(e);
    }
    PRINT_CHUNKMAP();

    /*
     * Release data range [349, 350).
     * This should release the last chunk remaining and cache should be empty
     * after this.
     */
    AZLogInfo("========== [Release] --> (349, 1) ==========");
    assert(cache.release(349, 1) == 1);

    AZLogInfo("========== [Dropall] ==========");
    cache.dropall();

    // Reload all bytes_chunk, after dropall().
    bc.load();
    bc1.load();
    bc2.load();
    bc3.load();

    /*
     * Get cache chunks covering range [0, 131072).
     * This should return following chunks:
     * 1. Newly allocated chunk [0, 131072).
     *
     * The largest contiguous block containing the requested chunk is
     * [0, 131072).
     */
    AZLogInfo("========== [Get] --> (0, 131072) ==========");
    v = cache.getx(0, 131072, &l, &r);
    assert(v.size() == 1);

    ASSERT_EXTENT(0, 131072);
    ASSERT_NEW(v[0], 0, 131072);
    bc = v[0];

    for ([[maybe_unused]] const auto& e : v) {
        PRINT_CHUNK(e);
    }
    PRINT_CHUNKMAP();

    /*
     * Release data range [6, 131072), emulating eof after short read.
     * This should not release any buffer but should just reduce the length
     * of the chunk.
     */
    AZLogInfo("========== [Release] --> (6, 131066) ==========");
    assert(cache.release(6, 131066) == 131066);
    PRINT_CHUNKMAP();

    /*
     * Get cache chunks covering range [6, 20).
     * This should return following chunks:
     * 1. Newly allocated chunk [6, 20).
     *
     * The largest contiguous block containing the requested chunk is
     * [0, 20).
     */
    AZLogInfo("========== [Get] --> (6, 14) ==========");
    v = cache.getx(6, 14, &l, &r);
    assert(v.size() == 1);

    ASSERT_EXTENT(6, 20);
    ASSERT_NEW(v[0], 6, 20);
#ifdef UTILIZE_TAILROOM_FROM_LAST_MEMBUF
    // Must use the alloc_buffer from last chunk.
    assert(v[0].get_buffer() == (bc.get_buffer() + 6));
#else
    assert(v[0].buffer_offset == 0);
#endif

    for ([[maybe_unused]] const auto& e : v) {
        PRINT_CHUNK(e);
    }
    PRINT_CHUNKMAP();

    /*
     * Get cache chunks covering range [5, 30).
     * This should return following chunks:
     * 1. Existing chunk [5, 6).
     * 2. Existing chunk [6, 20).
     * 3. Newly allocated chunk [20, 30).
     *
     * The largest contiguous block containing the requested chunk is
     * [0, 30).
     */
    AZLogInfo("========== [Get] --> (5, 25) ==========");
    v = cache.getx(5, 25, &l, &r);
    assert(v.size() == 3);

    ASSERT_EXTENT(0, 30);
    ASSERT_EXISTING(v[0], 5, 6);
    assert(v[0].get_buffer() == (bc.get_buffer() + 5));
    ASSERT_EXISTING(v[1], 6, 20);
#ifdef UTILIZE_TAILROOM_FROM_LAST_MEMBUF
    assert(v[1].get_buffer() == (bc.get_buffer() + 6));
#else
    assert(v[1].buffer_offset == 0);
#endif
    ASSERT_NEW(v[2], 20, 30);

    for ([[maybe_unused]] const auto& e : v) {
        PRINT_CHUNK(e);
    }
    PRINT_CHUNKMAP();

    /*
     * Clear entire cache.
     * cache.clear() asserts that bytes_allocated must drop to 0 if all chunks
     * are deleted. That will fail if we have some references to membuf(s),
     * hence we need to destruct all bytes_chunk references that we have
     * accumulated till now.
     */
    AZLogInfo("========== [Clear] ==========");
    v.clear();
    bc.~bytes_chunk();
    bc1.~bytes_chunk();
    bc2.~bytes_chunk();
    bc3.~bytes_chunk();

    cache.clear();
    PRINT_CHUNKMAP();

    /*
     * Get cache chunks covering range [5, 30).
     * This should return following chunks:
     * 1. Newly allocated chunk [5, 30).
     *
     * The largest contiguous block containing the requested chunk is
     * [5, 30).
     */
    AZLogInfo("========== [Get] --> (5, 25) ==========");
    v = cache.getx(5, 25, &l, &r);
    assert(v.size() == 1);

    ASSERT_EXTENT(5, 30);
    ASSERT_NEW(v[0], 5, 30);

    for ([[maybe_unused]] const auto& e : v) {
        PRINT_CHUNK(e);
    }
    PRINT_CHUNKMAP();

    /*
     * Get cache chunks covering range [5, 50).
     * This should return following chunks:
     * 1. Existing chunk [5, 30).
     * 2. Newly allocated chunk [30, 50).
     *
     * The largest contiguous block containing the requested chunk is
     * [5, 50).
     */
    AZLogInfo("========== [Get] --> (5, 45) ==========");
    v = cache.getx(5, 45, &l, &r);
    assert(v.size() == 2);

    ASSERT_EXTENT(5, 50);
    ASSERT_EXISTING(v[0], 5, 30);
    ASSERT_NEW(v[1], 30, 50);

    for ([[maybe_unused]] const auto& e : v) {
        PRINT_CHUNK(e);
    }
    PRINT_CHUNKMAP();

    /*
     * Get cache chunks covering range [5, 100).
     * This should return following chunks:
     * 1. Existing chunk [5, 30).
     * 2. Existing chunk [30, 50).
     * 3. Newly allocated chunk [50, 50).
     *
     * The largest contiguous block containing the requested chunk is
     * [5, 100).
     */
    AZLogInfo("========== [Get] --> (5, 95) ==========");
    v = cache.getx(5, 95, &l, &r);
    assert(v.size() == 3);

    ASSERT_EXTENT(5, 100);
    ASSERT_EXISTING(v[0], 5, 30);
    ASSERT_EXISTING(v[1], 30, 50);
    ASSERT_NEW(v[2], 50, 100);

    for ([[maybe_unused]] const auto& e : v) {
        PRINT_CHUNK(e);
    }
    PRINT_CHUNKMAP();

    /*
     * Release byte range [0, 200), but after setting the following:
     * - [5, 30) as dirty, and
     * - [50, 100) as inuse
     * If we call release for the range [0, 200), it covers the entire
     * cache, so it'll try to release all the chunks but it cannot release
     * chunks v[0] and v[2] as they are dirty and inuse respectively, both
     * of which are not safe_to_release().
     */
    AZLogInfo("========== [Release] --> (0, 200) ==========");
    v[0].get_membuf()->set_inuse();
    v[0].get_membuf()->set_locked();
    v[0].get_membuf()->set_dirty();
    v[0].get_membuf()->set_uptodate();
    v[0].get_membuf()->clear_locked();
    assert(!v[0].safe_to_release());
    assert(v[1].safe_to_release());
    v[2].get_membuf()->set_inuse();
    // hold the lock at the time of release() to ensure this works.
    v[2].get_membuf()->set_locked();
    assert(!v[2].safe_to_release());

    // It should just release [30,50).
    assert(cache.release(0, 200) == 20);

    v[0].get_membuf()->set_locked();
    v[0].get_membuf()->set_flushing();
    v[0].get_membuf()->clear_dirty();
    v[0].get_membuf()->clear_flushing();
    v[0].get_membuf()->clear_locked();
    v[0].get_membuf()->clear_inuse();

    v[2].get_membuf()->clear_locked();
    v[2].get_membuf()->clear_inuse();

    /*
     * Get cache chunks covering range [5, 200).
     * This should return following chunks:
     * 1. Existing chunk [5, 30).
     * 2. New chunk [30, 50).
     * 3. Existing chunk [50, 100).
     * 4. Newly allocated chunk [100, 200).
     *
     * The largest contiguous block containing the requested chunk is
     * [5, 200).
     */
    AZLogInfo("========== [Get] --> (5, 195) ==========");
    v = cache.getx(5, 195, &l, &r);
    assert(v.size() == 4);

    ASSERT_EXTENT(5, 200);
    ASSERT_EXISTING(v[0], 5, 30);
    ASSERT_NEW(v[1], 30, 50);
    ASSERT_EXISTING(v[2], 50, 100);
    ASSERT_NEW(v[3], 100, 200);

    for ([[maybe_unused]] const auto& e : v) {
        PRINT_CHUNK(e);
    }
    PRINT_CHUNKMAP();

    /*
     * Release [0, 500) should cover the entire cache and release all 195
     * bytes:
     * [5, 30)
     * [30, 50)
     * [50, 100)
     * [100, 200)
     */
    AZLogInfo("========== [Release] --> (0, 500) ==========");
    assert(cache.release(0, 500) == 195);
    assert(cache.chunkmap.empty());

    assert(cache.release(0, 1) == 0);
    assert(cache.release(10, 20) == 0);
    assert(cache.release(2, 2000) == 0);

    /*
     * Now run some random cache get/release to stress test the cache.
     */
    AZLogInfo("========== Starting cache stress  ==========");

    for (int i = 0; i < 10'000'000; i++) {
        AZLogVerbose("\n\n ----[ {} ]----------\n", i);

        const uint64_t offset = random_number(0, 100'000'000);
        const uint64_t length = random_number(1, AZNFSC_MAX_CHUNK_SIZE);
        const bool should_drop_all = random_number(0, 100) <= 1;

        // Randomly drop caches for testing.
        if (should_drop_all) {
            cache.dropall();
        }

        if (is_read()) {
            cache_read(cache, offset, length);
        } else {
            cache_write(cache, offset, length);
        }
    }

    AZLogInfo("========== Cache stress successful!  ==========");

    return 0;
}

static int _i = bytes_chunk_cache::unit_test();
#endif

}
