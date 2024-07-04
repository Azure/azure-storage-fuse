#ifndef __AZNFSC_FILE_CACHE_H__
#define __AZNFSC_FILE_CACHE_H__

#include <map>
#include <mutex>
#include <memory>

#include <cstdint>
#include <cassert>
#include <vector>

namespace aznfsc {

// W/o jumbo blocks, 5TB is the max file size we can support.
#define AZNFSC_MAX_FILE_SIZE    (100 * 1024 * 1024 * 50'000ULL)

/*
 * We should set fuse max_write and max_read option to 16MB.
 * With that application read/writes will be limited to 16MB and hence the
 * chunk size. Note that single readahead size is also limited by this, but
 * user can always issue multiple readahead reads.
 */
#define AZNFSC_MAX_CHUNK_SIZE (16 * 1024 * 1024)

#define AZNFSC_BAD_OFFSET (~0ull)

// Forward declaration.
class bytes_chunk_cache;

/**
 * This represents one contiguous chunk of bytes in bytes_chunk_cache.
 * bytes_chunk_cache consists of zero or more bytes_chunk ordered by offset.
 * Note that a byte range can be cached using one or more bytes_chunk and the
 * size of the individual component bytes_chunk depends on the order in which
 * the application writes data to the file.
 * A contiguous file range cached by a series of bytes_chunk is called an
 * "extent". Extents are important as they decide if/when we can issue full
 * block-sized write.
 */
struct bytes_chunk
{
    // bytes_chunk_cache needs to access the private member alloc_buffer.
    friend bytes_chunk_cache;

private:
    /*
     * This is the actual allocated buffer. The 'buffer' member will point
     * inside this allocated buffer. It typically points to the beginning of
     * the buffer (aka alloc_buffer.get()) but with cache trimming, buffer can
     * be updated and point anywhere inside the allocated buffer. Any chunk
     * that refers to the same allocated buffer will hold a ref to alloc_buffer,
     * so alloc_buffer will be freed when the last ref is dropped. This should
     * typically happen when the chunk is freed.
     */
    std::shared_ptr<uint8_t> alloc_buffer;

public:
    // Offset from the start of file this chunk represents.
    uint64_t offset;

    /*
     * Length of this chunk.
     * User can safely access [buffer, buffer+length).
     */
    uint64_t length;

    /*
     * Start of valid cached data corresponding to this chunk.
     * This will typically have the value alloc_buffer.get(), i.e., it points
     * to the start of the data buffer represented by the shared pointer
     * alloc_buffer, but if some cached data is deleted from the beginning of a
     * chunk, causing the buffer to be "trimmed" from the beginning, this can
     * point anywhere inside the buffer.
     */
    uint8_t *buffer;

    /*
     * is_empty indicates whether buffer contains valid data.
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
    bool is_empty;

    /**
     * Constructor to create a brand new chunk with newly allocated buffer.
     * This chunk is the sole owner of alloc_buffer and 'buffer' points to the
     * start of allocated buffer, alloc_buffer.get(). Later as this chunk is
     * split or returned to the caller through get(), alloc_buffer may have
     * more owners. When the last owner releases claim alloc_buffer will be
     * freed. This should happen when the chunk is freed.
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
    bytes_chunk(uint64_t _offset,
                uint64_t _length) :
        bytes_chunk(_offset,
                    _length,
                    true /* is_empty */,
                    std::shared_ptr<uint8_t>(new uint8_t[_length]))
    {
    }

    /**
     * Constructor to create a chunk that refers to alloc_buffer from another
     * existing chunk. The additional _buffer parameter allows flexibility of
     * making buffer point anywhere inside alloc_buffer.
     * This is useful for chunks created due to splitting or when returning
     * bytes_chunk from bytes_chunk_cache::get().
     */
    bytes_chunk(uint64_t _offset,
                uint64_t _length,
                uint8_t *_buffer,
                const std::shared_ptr<uint8_t>& _alloc_buffer) :
        bytes_chunk(_offset,
                    _length,
                    false /* is_empty */,
                    _alloc_buffer)
    {
        // Update buffer as requested.
        buffer = _buffer;

        // Make sure buffer points inside the allocate buffer.
        assert(buffer != nullptr);
        assert(alloc_buffer != nullptr);
        assert((buffer - alloc_buffer.get()) >= 0);
        assert((buffer - alloc_buffer.get()) <= AZNFSC_MAX_CHUNK_SIZE);
    }

    /**
     * The actual constructor called by all the overloads.
     */
    bytes_chunk(uint64_t _offset,
                uint64_t _length,
                bool _is_empty,
                const std::shared_ptr<uint8_t>& _alloc_buffer) :
        alloc_buffer(_alloc_buffer),
        offset(_offset),
        length(_length),
        buffer(alloc_buffer.get()),
        is_empty(_is_empty)
    {
        assert(offset < AZNFSC_MAX_FILE_SIZE);
        // 0-sized chunks don't exist.
        assert(length > 0);
        assert(length <= AZNFSC_MAX_CHUNK_SIZE);
        assert((offset + length) <= AZNFSC_MAX_FILE_SIZE);
        assert(buffer != nullptr);
        assert(alloc_buffer != nullptr);
        assert(alloc_buffer.use_count() > 1);
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
 */
class bytes_chunk_cache
{
public:
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
     */
    std::vector<bytes_chunk> get(uint64_t offset,
                                 uint64_t length,
                                 uint64_t *extent_left = nullptr,
                                 uint64_t *extent_right = nullptr)
    {
        return scan(offset, length, scan_action::SCAN_ACTION_GET,
                    extent_left, extent_right);
    }

    /**
     * Free chunks in the range [offset, offset+length).
     * Only chunks which are completely contained inside the range are freed,
     * while chunks which lie partially in the range are trimmed (by updating
     * the buffer, length and offset members). These will be freed later when
     * a release() call causes them to contain no valid data
     *
     * Note: All bytes in range [offset, offset+length) MUST be present in the
     *       cache, else it'll cause assertion failure. There is no usecase
     *       for releasing arbitrary portions of the cache and hence we don't
     *       implement that. The only known valid usecases for release() are:
     *       1. After a write completes release the chunks written. This can be
     *          done to make sure we only have dirty data (not yet written to
     *          the backing blob) in the cache.
     *       2. We do a get() to allocate 4K bytes in the cache but the file
     *          read() returned eof after 100 bytes, then [100, 4096) must be
     *          release()d, and this is fine since we know that the cache has
     *          that range.
     *       If you want to release an arbitrary range, first do a get() of that
     *       range and then release() each chunk separately.
     *
     * For releasing all chunks use releaseall().
     */
    void release(uint64_t offset, uint64_t length)
    {
        scan(offset, length, scan_action::SCAN_ACTION_RELEASE);
    }

    /**
     * Release all chunks from the cache.
     */
    void releaseall()
    {
        chunkmap.clear();
    }

    /**
     * This will run self tests to test the correctness of this class.
     */
    static int unit_test();

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

    /*
     * std::map of bytes_chunk, indexed by the starting offset of the chunk.
     */
    std::map<uint64_t, struct bytes_chunk> chunkmap;

    // Lock to protect chunkmap.
    std::mutex lock;
};

}

#endif /* __AZNFSC_FILE_CACHE_H__ */
