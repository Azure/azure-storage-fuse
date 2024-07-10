#include <random>
#include <sys/types.h>
#include <sys/stat.h>
#include <fcntl.h>
#include <sys/mman.h>

#include "file_cache.h"

/*
 * This enables debug logs and also runs the self tests.
 * Must enable once after adding a new self-test.
 */
//#define DEBUG_FILE_CACHE

#ifndef DEBUG_FILE_CACHE
#undef AZLogInfo
#undef AZLogDebug
#define AZLogInfo(fmt, ...)     /* nothing */
#define AZLogDebug(fmt, ...)    /* nothing */
#else
/*
 * Debug is not enabled early on when self-tests run, so use Info.
 * Uncomment these if you want to see debug logs from cache self-test.
 */
//#undef AZLogDebug
//#define AZLogDebug AZLogInfo
#endif

namespace aznfsc {

membuf::membuf(uint64_t _offset,
               uint64_t _length,
               int _backing_file_fd) :
               offset(_offset),
               length(_length),
               backing_file_fd(_backing_file_fd)
{
    if (is_file_backed()) {
        const bool ret = load();
        assert(ret);
    } else {
        // TODO: Handle memory alloc failures gracefully.
        allocated_buffer = buffer = new uint8_t[_length];
    }
}

bool membuf::drop()
{
    /*
     * Dropping memcache for non file-backed chunks doesn't make sense, and
     * is a no-op.
     */
    if (!is_file_backed()) {
        return true;
    }

    // If data is not loaded, it's a no-op.
    if (!allocated_buffer) {
        return true;
    }

    assert(length > 0);

    AZLogDebug("munmap(buffer={}, length={})",
               fmt::ptr(allocated_buffer), length);

    const int ret = ::munmap(allocated_buffer, length);
    if (ret != 0) {
        AZLogError("munmap(buffer={}, length={}) failed: {}",
                   fmt::ptr(allocated_buffer), length, strerror(errno));
        assert(0);
        return false;
    }

    allocated_buffer = buffer = nullptr;

    return true;
}

bool membuf::load()
{
    // Loading memcache for non file-backed chunks doesn't make sense.
    if (!is_file_backed()) {
        // Non file-backed chunks must have a valid buffer at all times.
        assert(allocated_buffer);
        return true;
    }

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

    AZLogDebug("mmap(fd={}, length={}, offset={})",
               backing_file_fd, length, adjusted_offset);

    /*
     * Default value of /proc/sys/vm/max_map_count may not be sufficient
     * for large files. Need to increase it.
     */
    assert(adjusted_offset <= offset);
    allocated_buffer =
        (uint8_t *) ::mmap(nullptr,
                           length + (offset - adjusted_offset),
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

    return true;
}

bytes_chunk::bytes_chunk(bytes_chunk_cache *_bcc,
                         uint64_t _offset,
                         uint64_t _length) :
             bytes_chunk(_bcc,
                         _offset,
                         _length,
                         0 /* buffer_offset */,
                         std::make_shared<membuf>(_offset,
                                                  _length,
                                                  _bcc->backing_file_fd),
                         true /* is_empty */)
{
}

bytes_chunk::bytes_chunk(bytes_chunk_cache *_bcc,
                         uint64_t _offset,
                         uint64_t _length,
                         uint64_t _buffer_offset,
                         const std::shared_ptr<membuf>& _alloc_buffer,
                         bool _is_empty) :
             bcc(_bcc),
             alloc_buffer(_alloc_buffer),
             offset(_offset),
             length(_length),
             buffer_offset(_buffer_offset),
             is_empty(_is_empty)
{
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
                                                 uint64_t *extent_left,
                                                 uint64_t *extent_right)
{
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

    // bytes_chunk vector that will be returned to the caller.
    std::vector<bytes_chunk> chunkvec;

    // offset and length cursor, updated as we add chunks to chunkvec.
    uint64_t next_offset = offset;
    uint64_t remaining_length = length;

    // Convenience variable to access the current chunk in the map.
    bytes_chunk *bc;

#ifdef UTILIZE_TAILROOM_FROM_LAST_MEMBUF
    // Last chunk (when we are getting byte range right after the last chunk).
    bytes_chunk *last_bc = nullptr;
#endif

    // Temp variables to hold chunk details for newly added chunk.
    uint64_t chunk_offset, chunk_length;

    /*
     * Temp variables to hold details for releasing a range.
     * All chunks in the range [begin_delete, end_delete) will be freed as
     * they fall completely inside the released range.
     * When we release a range that completely lies within a chunk, then we need
     * to allocate a new chunk to hold the data after the released range, while
     * the existing chunk is trimmed to hold the data before the range.
     * chunk_after is the new chunk thus created.
     * Used only for SCAN_ACTION_RELEASE.
     */
    std::map <uint64_t,
              struct bytes_chunk>::iterator begin_delete = chunkmap.end();
    std::map <uint64_t,
              struct bytes_chunk>::iterator end_delete = chunkmap.end();
    struct bytes_chunk *chunk_after = nullptr;

    /*
     * Variables to track the extent this write is part of.
     * We will udpate these as the left and right edges of the extent are
     * confirmed. Used only for SCAN_ACTION_GET.
     * lookback_it is the iterator to the chunk starting which we should
     * "look back" for the left edge of the extent containing the just written
     * chunk. We basically scan to the left till we find a gap or we hit the
     * end.
     */
    uint64_t _extent_left = AZNFSC_BAD_OFFSET;
    uint64_t _extent_right = AZNFSC_BAD_OFFSET;
    std::map <uint64_t,
              struct bytes_chunk>::iterator lookback_it = chunkmap.end();

    /*
     * TODO: See if we can hold shared lock for cases where we don't have to
     *       update chunkmap.
     */
    const std::unique_lock<std::mutex> _lock(lock);

    /*
     * First things first, if file-backed cache and backing file not yet open,
     * open it.
     */
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
    if (action == scan_action::SCAN_ACTION_GET) {
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
     * chunks can be identified in the returned chunkvec as they have is_empty
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
            /*
             * Cache-release is called only after cache-get which would have
             * allocated the requested range, so we should not find non-existent
             * chunks in the requested range.
             */
            assert(action != scan_action::SCAN_ACTION_RELEASE);

            /*
             * Only chunk being added, so left and right edge of that are also
             * the extent's left and right edge.
             */
            _extent_left = next_offset;
            _extent_right = next_offset + remaining_length;

            AZLogDebug("(first chunk) _extent_left: {} _extent_right: {}",
                       _extent_left, _extent_right);

            goto allocate_only_chunk;
        } else {
            // Iterator to the last chunk.
            it = std::prev(it);
            bc = &(it->second);

            if ((bc->offset + bc->length) <= next_offset) {
                /*
                 * Requested range lies after the end of last chunk, we will need
                 * to allocate a new chunk and this will be the only chunk needed
                 * to cover the requested range.
                 */
                assert(action != scan_action::SCAN_ACTION_RELEASE);

                /*
                 * If this new chunk starts right after the last chunk, then
                 * we don't know the actual value of _extent_left unless we
                 * scan left and check. In that case we set lookback_it to 'it'
                 * so that we can later "look back" and find the left edge.
                 * If it doesn't start right after, then next_offset becomes
                 * _extent_left.
                 */
                if ((bc->offset + bc->length) < next_offset) {
                    _extent_left = next_offset;
                    AZLogDebug("_extent_left: {}", _extent_left);
                } else {
                    AZLogDebug("lookback_it: [{},{})",
                               bc->offset, bc->offset + bc->length);
                    lookback_it = it;
#ifdef UTILIZE_TAILROOM_FROM_LAST_MEMBUF
                    last_bc = bc;
#endif
                }

                _extent_right = next_offset + remaining_length;
                AZLogDebug("_extent_right: {}", _extent_right);

                goto allocate_only_chunk;
            } else {
                /*
                 * Part or whole of requested range lies in the last chunk.
                 * The following for loop will correctly handle this.
                 * We don't know the left most edge until we "look back" from
                 * this chunk. Right edge is the right edge of the last chunk
                 * unless the current chunk goes past that, in which case that
                 * becomes the right edge. We don't set it here.
                 */
                AZLogDebug("lookback_it: [{},{})",
                           bc->offset, bc->offset + bc->length);
                lookback_it = it;
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
             * Requested range starts from this chunk.
             * The following for loop will correctly handle this.
             * Need to "look back" to find the left edge and look forward to
             * find the right edge.
             */
            AZLogDebug("lookback_it: [{},{})",
                       bc->offset, bc->offset + bc->length);
            lookback_it = it;
        } else {
            /*
             * Requested range starts before this chunk.
             */
            assert(it->first > next_offset);

            if (it == chunkmap.begin()) {
                /*
                 * If this is the first chunk then part or whole of the
                 * requested range lies before this chunk and we need to
                 * create a new chunk for that.
                 */
                assert(action != scan_action::SCAN_ACTION_RELEASE);

                bc = &(it->second);
                assert(bc->offset > next_offset);

                // Newly created chunk's offset and length.
                chunk_offset = next_offset;
                chunk_length = std::min(bc->offset - next_offset,
                                        remaining_length);

                remaining_length -= chunk_length;
                next_offset += chunk_length;

                /*
                 * This is the first chunk, so its offset is the left edge.
                 * We mark the right edge tentatively, it'll be confirmed after
                 * we look forward.
                 */
                _extent_left = chunk_offset;
                _extent_right = chunk_offset + chunk_length;

                AZLogDebug("_extent_left: {}", _extent_left);
                AZLogDebug("(tentative) _extent_right: {}", _extent_right);

                chunkvec.emplace_back(this, chunk_offset, chunk_length);
                AZLogDebug("(new chunk) [{},{})",
                           chunk_offset, chunk_offset + chunk_length);
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
                     */
                    assert(action != scan_action::SCAN_ACTION_RELEASE);

                    chunk_offset = next_offset;
                    chunk_length = std::min(bcn->offset - next_offset,
                                            remaining_length);

                    remaining_length -= chunk_length;
                    next_offset += chunk_length;

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
                        AZLogDebug("_extent_left: {}", _extent_left);
                    } else {
                        /*
                         * Else, new chunk touches the prev chunk, so we need
                         * to "look back" for finding the left edge.
                         */
                        AZLogDebug("lookback_it: [{},{})",
                                   bc->offset, bc->offset + bc->length);
                        lookback_it = it;
                    }
                    _extent_right = chunk_offset + chunk_length;
                    AZLogDebug("(tentative) _extent_right: {}", _extent_right);

                    // Search for more chunks should start from the next chunk.
                    it = itn;

                    chunkvec.emplace_back(this, chunk_offset, chunk_length);
                    AZLogDebug("(new chunk) [{},{})",
                               chunk_offset, chunk_offset + chunk_length);
                } else {
                    /*
                     * Prev chunk contains some bytes from initial part of the
                     * requested range. The following for loop will correctly
                     * handle this.
                     */
                    AZLogDebug("lookback_it: [{},{})",
                               bc->offset, bc->offset + bc->length);
                    lookback_it = it;
                }
            }
        }
    }

    /*
     * Either we should know the left edge or we should have set the lookback_it
     * to the chunk from where we start "looking back".
     */
    assert((_extent_left == AZNFSC_BAD_OFFSET) ==
           (lookback_it != chunkmap.end()));

    /*
     * Now sequentially go over the remaining chunks till we cover the entire
     * requested range. If some chunk doesn't exist, it'll be allocated.
     */
    for (; remaining_length != 0 && it != chunkmap.end(); ) {
        bc = &(it->second);

        bc->load();

        /*
         * next_offset must lie before the end of current chunk, else we should
         * not be inside the for loop.
         */
        assert(next_offset < (bc->offset + bc->length));

        chunk_offset = next_offset;

        if (next_offset == bc->offset) {
            chunk_length = std::min(bc->length, remaining_length);

            if (action == scan_action::SCAN_ACTION_GET) {
                chunkvec.emplace_back(this, chunk_offset, chunk_length,
                                      bc->buffer_offset, bc->alloc_buffer);
                AZLogDebug("(existing chunk) [{},{}) b:{} a:{}",
                           chunk_offset, chunk_offset + chunk_length,
                           fmt::ptr(chunkvec.back().get_buffer()),
                           fmt::ptr(bc->alloc_buffer->get()));
            } else {
                assert (action == scan_action::SCAN_ACTION_RELEASE);
                if (chunk_length == bc->length) {
                    AZLogDebug("(releasing chunk) [{},{}) b:{} a:{}",
                               chunk_offset, chunk_offset + chunk_length,
                               fmt::ptr(bc->get_buffer()),
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
                    AZLogDebug("(trimming chunk from left) [{},{}) -> [{},{})",
                               bc->offset, bc->offset + bc->length,
                               bc->offset + chunk_length,
                               bc->offset + bc->length);

                    bc->offset += chunk_length;
                    bc->buffer_offset += chunk_length;
                    bc->length -= chunk_length;

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
            }

            // This chunk is fully consumed, move to the next chunk.
            ++it;
        } else if (next_offset < bc->offset) {
            // Starts before the next chunk.
            assert(action != scan_action::SCAN_ACTION_RELEASE);
            chunk_length = std::min(bc->offset - next_offset,
                                    remaining_length);

            chunkvec.emplace_back(this, chunk_offset, chunk_length);
            AZLogDebug("(new chunk) [{},{})",
                       chunk_offset, chunk_offset+chunk_length);

            /*
             * In the next iteration we need to look at the current chunk, so
             * don't increment the iterator.
             */
        } else /* (next_offset > bc->offset) */ {
            chunk_length = std::min(bc->offset + bc->length - next_offset,
                                    remaining_length);

            if (action == scan_action::SCAN_ACTION_GET) {
                chunkvec.emplace_back(this, chunk_offset, chunk_length,
                                      bc->buffer_offset + (next_offset - bc->offset),
                                      bc->alloc_buffer);
                AZLogDebug("(existing chunk) [{},{}) b:{} a:{}",
                           chunk_offset, chunk_offset + chunk_length,
                           fmt::ptr(chunkvec.back().get_buffer()),
                           fmt::ptr(bc->alloc_buffer->get()));
            } else {
                assert(action == scan_action::SCAN_ACTION_RELEASE);
                if (chunk_length == remaining_length) {
                    /*
                     * Part of the chunk is released in the middle.
                     * We need to trim the original chunk to contain data before
                     * the released data and create a new chunk to hold the data
                     * after the released data.
                     */
                    const uint64_t chunk_after_offset =
                        next_offset + chunk_length;
                    const uint64_t chunk_after_length =
                        bc->offset + bc->length - next_offset - chunk_length;

                    if (chunk_after_length > 0) {
                        assert(chunk_after == nullptr);
                        chunk_after = new bytes_chunk(this,
                                                      chunk_after_offset,
                                                      chunk_after_length);

                        AZLogDebug("(chunk after) [{},{})",
                                   chunk_after_offset,
                                   chunk_after_offset + chunk_after_length);
                    }

                    AZLogDebug("(trimming chunk from right) [{},{}) -> [{},{})",
                            bc->offset, bc->offset + bc->length,
                            bc->offset, next_offset);
                    bc->length = next_offset - bc->offset;
                    assert((int64_t) bc->length > 0);
                } else {
                    assert(chunk_length ==
                           (bc->offset + bc->length - next_offset));
                    assert(chunk_length < remaining_length);

                    /*
                     * Entire chunk after next_offset is released, trim the
                     * chunk.
                     */
                    AZLogDebug("(trimming chunk from right) [{},{}) -> [{},{})",
                               bc->offset, bc->offset + bc->length,
                               bc->offset, next_offset);

                    bc->length = next_offset - bc->offset;
                    assert((int64_t) bc->length > 0);
                }
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
        _extent_right = bc->offset + bc->length;
        AZLogDebug("_extent_right: {}", _extent_right);
    }

    /*
     * Allocate the only or the last chunk beyond the highest chunk we have
     * in our cache.
     */
allocate_only_chunk:
    if (remaining_length != 0) {
        /*
         * Other than when we are adding cache chunks, we should never come
         * here for allocating new chunk buffer.
         */
        assert(action == scan_action::SCAN_ACTION_GET);

        AZLogDebug("(only/last chunk) [{},{})",
                   next_offset, next_offset + remaining_length);

#ifdef UTILIZE_TAILROOM_FROM_LAST_MEMBUF
        if (last_bc && (last_bc->tailroom() > 0)) {
            chunk_length = std::min(last_bc->tailroom(), remaining_length);

            AZLogDebug("(sharing last chunk's alloc_buffer) [{},{})",
                       next_offset, next_offset + chunk_length);

            /*
             * Though this new chunk is sharing alloc_buffer with the last
             * chunk, it's nevertheless a new chunk and hence is_empty must
             * be true.
             */
            chunkvec.emplace_back(this, next_offset,
                                  chunk_length,
                                  last_bc->buffer_offset + last_bc->length,
                                  last_bc->alloc_buffer,
                                  true /* is_empty */);

            // last chunk and this new chunk are sharing the same alloc_buffer.
            assert(last_bc->alloc_buffer.use_count() >= 2);

            remaining_length -= chunk_length;
            next_offset += chunk_length;
        }
#endif

        if (remaining_length) {
            AZLogDebug("(new last chunk) [{},{})",
                       next_offset, next_offset + remaining_length);
            chunkvec.emplace_back(this, next_offset, remaining_length);
        }

        remaining_length = 0;
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

        if (chunk.is_empty) {
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

            AZLogDebug("(adding to chunkmap) [{},{})",
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

            if ((chunk.offset + chunk.length) > _extent_right) {
                _extent_right = (chunk.offset + chunk.length);
                AZLogDebug("_extent_right: {}", _extent_right);
            }
        }
    }

    /*
     * Delete chunks in the range [begin_delete, end_delete).
     */
    if (action == scan_action::SCAN_ACTION_RELEASE) {
        if (begin_delete != chunkmap.end()) {
            for (auto _it = begin_delete; _it != end_delete; ++_it) {
                bc = &(_it->second);
                AZLogDebug("(freeing chunk) [{},{}) b:{} a:{}",
                           bc->offset, bc->offset + bc->length,
                           fmt::ptr(bc->get_buffer()),
                           fmt::ptr(bc->alloc_buffer->get()));
            }

            // Delete the entire range.
            chunkmap.erase(begin_delete, end_delete);
        }
    } else {
        assert((begin_delete == chunkmap.end()) &&
               (end_delete == chunkmap.end()));
    }

    /*
     * If we have a chunk_after create it now?
     * chunk_after is the chunk created when some part from within a chunk
     * is deleted (not touching either edge).
     */
    if (chunk_after) {
            // Only possible when we release a byte range within a chunk.
            assert(action == scan_action::SCAN_ACTION_RELEASE);

            AZLogDebug("(chunk after insert) [{},{})",
                       chunk_after->offset,
                       chunk_after->offset + chunk_after->length);
#ifndef NDEBUG
            const auto p = chunkmap.try_emplace(chunk_after->offset,
                                                this,
                                                chunk_after->offset,
                                                chunk_after->length,
                                                chunk_after->buffer_offset,
                                                chunk_after->alloc_buffer);
            assert(p.second == true);
#else
            chunkmap.try_emplace(chunk_after->offset,
                                 this,
                                 chunk_after->offset,
                                 chunk_after->length,
                                 chunk_after->buffer_offset,
                                 chunk_after->alloc_buffer);
#endif

            delete chunk_after;
    }

    if (action == scan_action::SCAN_ACTION_GET) {
        /*
         * If left edge is not set, set it now.
         */
        if (_extent_left == AZNFSC_BAD_OFFSET) {
            assert(lookback_it != chunkmap.end());
            do {
                bc = &(lookback_it->second);

                if ((_extent_left != AZNFSC_BAD_OFFSET) &&
                    ((bc->offset + bc->length) != _extent_left)) {
                    AZLogDebug("(hit gap) _extent_left: {} rightedge: {}",
                            _extent_left, (bc->offset + bc->length));
                    break;
                }

                _extent_left = bc->offset;
                AZLogDebug("_extent_left: {}", _extent_left);
            } while (lookback_it-- != chunkmap.begin());
        }

        /*
         * Set/update extent right edge.
         */
        for (; it != chunkmap.end(); ++it) {
            bc = &(it->second);

            if ((_extent_right != AZNFSC_BAD_OFFSET) &&
                (bc->offset != _extent_right)) {
                AZLogDebug("(hit gap) _extent_right: {} leftedge: {}",
                        _extent_right, bc->offset);
                break;
            }

            _extent_right = bc->offset + bc->length;
            AZLogDebug("_extent_right: {}", _extent_right);
        }

        if (extent_left) {
            *extent_left = _extent_left;
        }

        if (extent_right) {
            *extent_right = _extent_right;
        }

    }

    return (action == scan_action::SCAN_ACTION_GET)
                ? chunkvec : std::vector<bytes_chunk>();
}

void bytes_chunk_cache::drop(uint64_t offset, uint64_t length)
{
    if (backing_file_name.empty()) {
        // No-op for non file-backed caches.
        return;
    }

    const std::unique_lock<std::mutex> _lock(lock);

    /*
     * Find chunk with offset >= next_offset. Note that we only drop caches
     * for chunks which completely lie in the range, i.e., partial chunks are
     * skipped. This is ok as dropping caches is only for saving memory and
     * not doing it doesn't cause correctness isssues.
     */
    auto it = chunkmap.lower_bound(offset);

    // No full chunk lies in the given range.
    if (it == chunkmap.end()) {
        return;
    }

    uint64_t remaining_length = length;

    /*
     * Iterate over all chunks and drop chunks that completely lie in the
     * requested range.
     */
    for (; remaining_length != 0 && it != chunkmap.end(); ++it) {
        bytes_chunk *bc = &(it->second);

        if (remaining_length < bc->length) {
            break;
        }

        bc->drop();

        remaining_length -= bc->length;
    }
}

void bytes_chunk_cache::clear()
{
    AZLogInfo("[{}] clear() called, backing_file_name={}",
              fmt::ptr(this), backing_file_name);

    /*
     * We hold the bytes_chunk_cache lock and go over all the bytes_chunk to
     * see if they can be freed. Following bytes_chunk cannot be freed:
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
     * can safely remove from chunkmap if inuse/dirty/locked are not set.
     */
    const std::unique_lock<std::mutex> _lock(lock);

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
            AZLogInfo("[{}] skipping as membuf(offset={}, length={}) is inuse",
                      fmt::ptr(this), mb->offset, mb->length);
            continue;
        }

        /*
         * Not under use, cannot be locked.
         * Note that users are supposed to drop the inuse count only after
         * grabbing the membuf lock.
         */
        assert(!mb->is_locked());

        /*
         * Has data to be written to Blob.
         * Cannot safely drop this from the cache.
         */
        if (mb->is_dirty()) {
            AZLogInfo("[{}] skipping as membuf(offset={}, length={}) is dirty",
                      fmt::ptr(this), mb->offset, mb->length);
            continue;
        }

        AZLogDebug("[{}] deleting membuf(offset={}, length={})",
                   fmt::ptr(this), mb->offset, mb->length);

        /*
         * Release the chunk.
         * This will release the membuf (munmap() it in case of file-backed
         * cache and delete it for heap backed cache). At this point the membuf
         * is guaranteed to be not in use since we checked the inuse count
         * above.
         */
        chunkmap.erase(it);
    }

    if (!chunkmap.empty()) {
        AZLogInfo("[{}] Skipping delete for backing_file_name={}, as chunkmap "
                  "not empty", fmt::ptr(this), backing_file_name);
        return;
    }

    /*
     * If all chunks are released, delete the backing file in case of
     * file-backed caches.
     */
    if (backing_file_fd != -1) {
        const int ret = ::close(backing_file_fd);
        if (ret != 0) {
            AZLogError("close(fd={}) failed: {}",
                    backing_file_fd, strerror(errno));
            assert(0);
        } else {
            AZLogInfo("Backing file {} closed, fd={}",
                    backing_file_name, backing_file_fd);
        }
        backing_file_fd = -1;
        backing_file_len = 0;
    }

    assert(backing_file_len == 0);

    if (!backing_file_name.empty()) {
        const int ret = ::unlink(backing_file_name.c_str());
        if ((ret != 0) && (errno != ENOENT)) {
            AZLogError("unlink({}) failed: {}",
                    backing_file_name, strerror(errno));
            assert(0);
        } else {
            AZLogInfo("Backing file {} deleted", backing_file_name);
        }
    }
}

/**
 * Generate a random number in the range [min, max].
 */
static uint64_t random_number(uint64_t min, uint64_t max)
{
    static std::mt19937 gen((uint64_t) std::chrono::system_clock::now().time_since_epoch().count());
    return min + (gen() % (max - min + 1));
}

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
    uint64_t l, r;

    AZLogDebug("=====> cache_read({}, {})", offset, offset+length);
    v = cache.get(offset, length, &l, &r);
    // At least one chunk.
    assert(v.size() >= 1);
    assert(v[0].offset == offset);
    assert(l <= v[0].offset);

    // Sanitize the returned chunkvec.
    uint64_t prev_chunk_right_edge = AZNFSC_BAD_OFFSET;
    uint64_t total_length = 0;

    for (auto e : v) {
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

    AZLogDebug("=====> cache_read({}, {}): l={} r={} vec={}",
               offset, offset+length, l, r, v.size());
}

static void cache_write(bytes_chunk_cache& cache,
                        uint64_t offset,
                        uint64_t length)
{
    std::vector<bytes_chunk> v;
    uint64_t l, r;

    AZLogDebug("=====> cache_write({}, {})", offset, offset+length);
    v = cache.get(offset, length, &l, &r);
    // At least one chunk.
    assert(v.size() >= 1);
    assert(v[0].offset == offset);
    assert(l <= v[0].offset);

    // Sanitize the returned chunkvec.
    uint64_t prev_chunk_right_edge = AZNFSC_BAD_OFFSET;
    uint64_t total_length = 0;

    for (auto e : v) {
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

    AZLogDebug("=====> cache_write({}, {}): l={} r={} vec={}",
               offset, offset+length, l, r, v.size());
    AZLogDebug("=====> cache_release({}, {})", offset, offset+length);
    cache.release(offset, length);
}

/* static */
int bytes_chunk_cache::unit_test()
{
    assert(::sysconf(_SC_PAGESIZE) == PAGE_SIZE);

    /*
     * Choose file-backed or non file-backed cache for testing.
     * For file-backed cache, make sure /tmp as sufficient space.
     */
#if 0
    bytes_chunk_cache cache;
#else
    bytes_chunk_cache cache("/tmp/bytes_chunk_cache");
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
    uint8_t *buffer;

#define ASSERT_NEW(chunk, start, end) \
do { \
    assert(chunk.offset == start); \
    assert(chunk.length == end-start); \
    assert(chunk.is_empty); \
    /* All membufs MUST be returned with inuse incremented */ \
    assert(chunk.get_membuf()->is_inuse()); \
    chunk.get_membuf()->clear_inuse(); \
} while (0)

#define ASSERT_EXISTING(chunk, start, end) \
do { \
    assert(chunk.offset == start); \
    assert(chunk.length == end-start); \
    assert(!(chunk.is_empty)); \
    /* All membufs MUST be returned with inuse incremented */ \
    assert(chunk.get_membuf()->is_inuse()); \
    chunk.get_membuf()->clear_inuse(); \
} while (0)

#define ASSERT_EXTENT(left, right) \
do { \
    assert(l == left); \
    assert(r == right); \
} while (0)

#define PRINT_CHUNK(chunk) \
        AZLogInfo("[{},{}){} <{}>", chunk.offset,\
                  chunk.offset + chunk.length,\
                  chunk.is_empty ? " [Empty]" : "", \
                  fmt::ptr(chunk.get_buffer()))

    /*
     * Get cache chunks covering range [0, 300).
     * Since the cache is empty, it'll add a new empty chunk and return that.
     * The newly added chunk is also the largest contiguous block containing
     * the chunk.
     */
    AZLogInfo("========== [Get] --> (0, 300) ==========");
    v = cache.get(0, 300, &l, &r);
    assert(v.size() == 1);

    ASSERT_EXTENT(0, 300);
    ASSERT_NEW(v[0], 0, 300);
    /*
     * This bytes_chunk later gets deleted by the call to release(200,100),
     * so we store the buffer.
     */
    buffer = v[0].get_buffer();

    for (auto e : v) {
        PRINT_CHUNK(e);
    }

    /*
     * Release data range [0, 100).
     * After this the cache should have chunk [100, 300).
     */
    AZLogInfo("========== [Release] --> (0, 100) ==========");
    cache.release(0, 100);

    /*
     * Release data range [200, 300).
     * After this the cache should have chunk [100, 200).
     */
    AZLogInfo("========== [Release] --> (200, 100) ==========");
    cache.release(200, 100);

    /*
     * Get cache chunks covering range [100, 200).
     * This will return the (only) existing chunk.
     * The newly added chunk is also the largest contiguous block containing
     * the chunk.
     */
    AZLogInfo("========== [Get] --> (100, 100) ==========");
    v = cache.get(100, 100, &l, &r);
    assert(v.size() == 1);

    ASSERT_EXTENT(100, 200);
    ASSERT_EXISTING(v[0], 100, 200);
    assert(v[0].get_buffer() == (buffer + 100));

    for (auto e : v) {
        PRINT_CHUNK(e);
    }

    /*
     * Get cache chunks covering range [50, 150).
     * This should return 2 chunks:
     * 1. Newly allocated chunk [50, 100).
     * 2. Existing chunk data from [100, 150).
     *
     * The largest contiguous block containing the requested chunk is [50, 200).
     */
    AZLogInfo("========== [Get] --> (50, 100) ==========");
    v = cache.get(50, 100, &l, &r);
    assert(v.size() == 2);

    ASSERT_EXTENT(50, 200);
    ASSERT_NEW(v[0], 50, 100);
    ASSERT_EXISTING(v[1], 100, 150);
    assert(v[1].get_buffer() == (buffer + 100));

    for (auto e : v) {
        PRINT_CHUNK(e);
    }

    AZLogInfo("========== [Dropall] ==========");
    cache.dropall();

    /*
     * Get cache chunks covering range [250, 300).
     * This should return 1 chunk:
     * 1. Newly allocated chunk [250, 300).
     *
     * The largest contiguous block containing the requested chunk is [250, 300).
     */
    AZLogInfo("========== [Get] --> (250, 50) ==========");
    v = cache.get(250, 50, &l, &r);
    assert(v.size() == 1);

    ASSERT_EXTENT(250, 300);
    ASSERT_NEW(v[0], 250, 300);
    new (&bc) bytes_chunk(v[0]);

    for (auto e : v) {
        PRINT_CHUNK(e);
    }

    /*
     * Get cache chunks covering range [0, 50).
     * This should return 1 chunk:
     * 1. Newly allocated chunk [0, 50).
     *
     * The largest contiguous block containing the requested chunk is [0, 200).
     */
    AZLogInfo("========== [Get] --> (0, 50) ==========");
    v = cache.get(0, 50, &l, &r);
    assert(v.size() == 1);

    ASSERT_EXTENT(0, 200);
    ASSERT_NEW(v[0], 0, 50);

    for (auto e : v) {
        PRINT_CHUNK(e);
    }

    /*
     * Get cache chunks covering range [150, 275).
     * This should return following chunks:
     * 1. Existing chunk [150, 200).
     * 2. Newly allocated chunk [200, 250).
     * 3. Existing chunk [250, 275).
     *
     * The largest contiguous block containing the requested chunk is [0, 300).
     */
    AZLogInfo("========== [Get] --> (150, 125) ==========");
    v = cache.get(150, 125, &l, &r);
    assert(v.size() == 3);

    ASSERT_EXTENT(0, 300);
    ASSERT_EXISTING(v[0], 150, 200);
    ASSERT_NEW(v[1], 200, 250);
    ASSERT_EXISTING(v[2], 250, 275);
    assert(v[2].get_buffer() == bc.get_buffer());
    new (&bc1) bytes_chunk(v[0]);
    new (&bc2) bytes_chunk(v[1]);

    for (auto e : v) {
        PRINT_CHUNK(e);
    }

    AZLogInfo("========== [Dropall] ==========");
    cache.dropall();

    // Reload all bytes_chunk, after dropall().
    bc.load();
    bc1.load();
    bc2.load();

    /*
     * Release data range [0, 175).
     * After this the cache should have the following chunk:
     * 1. [175, 200).
     * 2. [200, 250).
     * 3. [250, 300).
     */
    AZLogInfo("========== [Release] --> (0, 175) ==========");
    cache.release(0, 175);

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
    v = cache.get(100, 180, &l, &r);
    assert(v.size() == 4);

    ASSERT_EXTENT(100, 300);
    ASSERT_NEW(v[0], 100, 175);
    ASSERT_EXISTING(v[1], 175, 200);
    assert(v[1].get_buffer() == (bc1.get_buffer() + 25));
    ASSERT_EXISTING(v[2], 200, 250);
    assert(v[2].get_buffer() == bc2.get_buffer());
    ASSERT_EXISTING(v[3], 250, 280);
    assert(v[3].get_buffer() == bc.get_buffer());
    new (&bc3) bytes_chunk(v[0]);

    for (auto e : v) {
        PRINT_CHUNK(e);
    }

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
    v = cache.get(0, 350, &l, &r);
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
    new (&bc1) bytes_chunk(v[0]);
    new (&bc3) bytes_chunk(v[5]);

    for (auto e : v) {
        PRINT_CHUNK(e);
    }

    /*
     * Release data range [50, 225).
     * After this the cache should have the following chunks:
     * 1. [0, 50).
     * 2. [225, 250).
     * 3. [250, 300).
     * 4. [300, 350).
     */
    AZLogInfo("========== [Release] --> (50, 175) ==========");
    cache.release(50, 175);

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
    v = cache.get(0, 325, &l, &r);
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

    for (auto e : v) {
        PRINT_CHUNK(e);
    }

    /*
     * Release data range [0, 349).
     * After this the cache should have the following chunks:
     * 1. [349, 350).
     */
    AZLogInfo("========== [Release] --> (0, 349) ==========");
    cache.release(0, 349);

    /*
     * Get cache chunks covering range [349, 350).
     * This should return following chunks:
     * 1. Existing chunk [349, 350).
     *
     * The largest contiguous block containing the requested chunk is [349, 350).
     */
    AZLogInfo("========== [Get] --> (349, 1) ==========");
    v = cache.get(349, 1, &l, &r);
    assert(v.size() == 1);

    ASSERT_EXTENT(349, 350);
    ASSERT_EXISTING(v[0], 349, 350);
    assert(v[0].get_buffer() == (bc3.get_buffer() + 49));

    for (auto e : v) {
        PRINT_CHUNK(e);
    }

    /*
     * Release data range [349, 350).
     * This should release the last chunk remaining and cache should be empty
     * after this.
     */
    AZLogInfo("========== [Release] --> (349, 1) ==========");
    cache.release(349, 1);

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
    v = cache.get(0, 131072, &l, &r);
    assert(v.size() == 1);

    ASSERT_EXTENT(0, 131072);
    ASSERT_NEW(v[0], 0, 131072);
    new (&bc) bytes_chunk(v[0]);

    for (auto e : v) {
        PRINT_CHUNK(e);
    }

    /*
     * Release data range [6, 131072), emulating eof after short read.
     * This should not release any buffer but should just reduce the length
     * of the chunk.
     */
    AZLogInfo("========== [Release] --> (6, 131066) ==========");
    cache.release(6, 131066);

    /*
     * Get cache chunks covering range [6, 20).
     * This should return following chunks:
     * 1. Newly allocated chunk [6, 20).
     *
     * The largest contiguous block containing the requested chunk is
     * [0, 20).
     */
    AZLogInfo("========== [Get] --> (6, 14) ==========");
    v = cache.get(6, 14, &l, &r);
    assert(v.size() == 1);

    ASSERT_EXTENT(0, 20);
    ASSERT_NEW(v[0], 6, 20);
#ifdef UTILIZE_TAILROOM_FROM_LAST_MEMBUF
    // Must use the alloc_buffer from last chunk.
    assert(v[0].get_buffer() == (bc.get_buffer() + 6));
#else
    assert(v[0].buffer_offset == 0);
#endif

    for (auto e : v) {
        PRINT_CHUNK(e);
    }

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
    v = cache.get(5, 25, &l, &r);
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

    for (auto e : v) {
        PRINT_CHUNK(e);
    }

    /*
     * Clear entire cache.
     */
    AZLogInfo("========== [Clear] ==========");
    cache.clear();

    /*
     * Get cache chunks covering range [5, 30).
     * This should return following chunks:
     * 1. Newly allocated chunk [5, 30).
     *
     * The largest contiguous block containing the requested chunk is
     * [5, 30).
     */
    AZLogInfo("========== [Get] --> (5, 25) ==========");
    v = cache.get(5, 25, &l, &r);
    assert(v.size() == 1);

    ASSERT_EXTENT(5, 30);
    ASSERT_NEW(v[0], 5, 30);

    for (auto e : v) {
        PRINT_CHUNK(e);
    }

    /*
     * Now run some random cache get/release to stress test the cache.
     */
    AZLogInfo("========== Starting cache stress  ==========");

    for (int i = 0; i < 10'000'000; i++) {
        AZLogDebug("\n\n ----[ {} ]----------\n", i);

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

#ifdef DEBUG_FILE_CACHE
static int _i = bytes_chunk_cache::unit_test();
#endif

}
