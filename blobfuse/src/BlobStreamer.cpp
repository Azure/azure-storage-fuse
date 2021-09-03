#include <BlobStreamer.h>
#include <base64.h>

#define MAX_BLOCK_SIZE_FOR_SINGLE_READ (64 * 1204 * 1024)

extern struct configParams config_options;
CacheSizeCalculator* CacheSizeCalculator::mInstance = NULL;
CacheSizeCalculator* CacheSizeCalculator::GetObj()
{
    if (NULL == mInstance) {
        mInstance = new CacheSizeCalculator();
    }
    return mInstance;
}


// Add a new block to this stream object
int StreamObject::AddBlock(BlobBlock* block, uint64_t max_blocks_per_file)
{
    if (GetCacheListSize() >= max_blocks_per_file || 
        CacheSizeCalculator::GetObj()->MaxLimitReached()) {
        // We have exceeeded max number of blocks allowed so delete the end of the queue
        RemoveBlock();
    }

    // Add the new block to front of the list (LRU)
    CacheSizeCalculator::GetObj()->AddSize(block->data_length);
    m_block_cache_list.push_front(block);

    return 0;
}

// Remove a unused block from the list
int StreamObject::RemoveBlock()
{
    BlobBlock* last_block = m_block_cache_list.back();

    if (last_block) {
        // Lock it so that we wait untill any reader is using this block
        // then we remove it from the list and unlock, as its not in the list now,
        // no one can search or use it anymore
        last_block->lck.lock();
        m_block_cache_list.pop_back();
        
        // Release memory used by this block
        CacheSizeCalculator::GetObj()->RemoveSize(last_block->data_length);
        last_block->buff.clear();
        last_block->lck.unlock();
        
        delete last_block;
    }

    return 0;
}

// Search for a block based on offset
BlobBlock* StreamObject::GetBlock(uint64_t offset)
{
    // Start offsets are rounded off to block-size multiples so just match the start offset
    for(auto it = m_block_cache_list.begin(); it != m_block_cache_list.end(); ++it) {
        if ((*it)->valid && (*it)->start_offset == offset) {
            return (*it);
        }
    }

    return NULL;
}


// Cleanup the file info for this object as its no more nee
void StreamObject::Cleanup()
{
    while(GetCacheListSize() > 0)
        RemoveBlock();
}

// -----------------------------------------------------------------------------------------------------------

//  Get block of a given file based on give offset
BlobBlock* BlobStreamer::GetBlock(const char* file_name, uint64_t offset, StreamObject* obj)
{
    // As we always download in term of block_size to search a block that can serve this request
    // we need to find the start offset of block to which this offset belongs so convert offset
    // to its closest start-offset of a given block.
    uint64_t start_offset = (offset - (offset % block_size));

    obj->Lock();

    BlobBlock *block = obj->GetBlock(start_offset);

    if (block == NULL || block->valid == false) {
        // Either block was not found or its not valid so download a new block
        syslog(LOG_DEBUG, "File %s Block offset %lu : not found. Download and cache", file_name, offset);

        block = new BlobBlock;
        block->start_offset = start_offset;
        block->valid = true;
        block->last = false;

        if (block_size > MAX_BLOCK_SIZE_FOR_SINGLE_READ) {
            char *buff = (char*)malloc(block_size);
            azclient->DownloadToBuffer(file_name, buff, start_offset, block_size, config_options.concurrency);
            block->buff.write(buff, block_size);
            free(buff);
        } else {
            azclient->DownloadToStream(file_name, block->buff, start_offset, block_size);
        }

        if (errno != 0)
        {
            obj->UnLock();

            int storage_errno = errno;
            if (errno == 416) {
                syslog(LOG_DEBUG, "Failed to download block of %s with offset %lu.  Errno = %d (Out of range).\n", file_name, start_offset, storage_errno);
                errno = storage_errno;
                return NULL;
            }

            syslog(LOG_ERR, "Failed to download block of %s with offset %lu.  Errno = %d.\n", file_name, start_offset, storage_errno);
            return NULL;
        }

        block->data_length = block->buff.str().size();
        if (block->data_length < block_size) {
            // We asked to read 16MB but got less data then assume its the end of file
            //syslog(LOG_DEBUG, "File %s Block offset %lu : is last block", file_name, offset);
            block->last = true;
        }

        obj->AddBlock(block, max_blocks_per_file);

    }
    block->lck.lock();
    obj->UnLock();

    return block;
}


// When file open is hit, just download the first block of file and cache it if caching is allowed
int BlobStreamer::OpenFile(const char* file_name)
{
    // If caching of block is not allowed then there is nothing to be done here.
    if (max_blocks_per_file > 0) {
        m_mutex.lock();

        StreamObject* obj = NULL;
        auto iter = file_map.find(file_name);

        if(iter == file_map.end()) {
            // File is not found in the map so create a new entry and cache the first block
            obj =  new StreamObject;
            file_map[file_name] = obj;
        } else {
            // File object exists in our map
            obj = iter->second;
        }

        m_mutex.unlock();

        // Mark one more open handle exists for this file
        obj->IncRefCount();

        // Download and save the first block of this file for future read.
        BlobBlock* block = GetBlock(file_name, 0, obj);
        block->lck.unlock();
    }

    return 0;
}

// Close file checks all handles are closed or not, if so wipe out the file info
int BlobStreamer::CloseFile(const char* file_name)
{
    // If caching is not allowed as per config then nothing to be cleaned up here
    if (max_blocks_per_file > 0) {

        m_mutex.lock();
        auto iter = file_map.find(file_name);
        if(iter == file_map.end()) {
            m_mutex.unlock();
            return -1;
        }

        StreamObject* obj = iter->second;
        m_mutex.unlock();

        if (0 == obj->DecRefCount()) {
            // All open handles are closed so file info has been cleanedup
            // We can remove the entry from the map now. Next open will cause a new entry.
            syslog(LOG_DEBUG, "All handles for %s released, cleanup cached blocks", file_name);
            
            m_mutex.lock();
            file_map.erase(file_name);
            
            obj->Lock();
            obj->Cleanup();
            obj->UnLock();

            delete obj;
            m_mutex.unlock();
        }
    }

    return 0;
}


// Read file retreives the data from cache and sends it back to the caller
int BlobStreamer::DeleteFile(const char* file_name)
{
    if (max_blocks_per_file <= 0) {
        return 0;
    }

    // Caching of block is allowed so we need to check block exists or not
    m_mutex.lock();
    auto iter = file_map.find(file_name);
    if(iter == file_map.end()) {
        m_mutex.unlock();
        return 0;
    }

    StreamObject* obj = iter->second;
    file_map.erase(file_name);
    m_mutex.unlock();

    if (obj) {
        obj->Lock();
        obj->Cleanup();
        obj->UnLock();

        delete obj;
    }

    return 0;
}


// Read file retreives the data from cache and sends it back to the caller
int BlobStreamer::ReadFile(const char* file_name, uint64_t offset, uint64_t length, char* out)
{
    int len = 0;

    // If block caching is not allowed as per config then stream data directly from container.
    if (max_blocks_per_file <= 0) {
        // Get data in form of a stream and fill the output buffer with data retreived
        std::stringstream os;
        //syslog(LOG_DEBUG, "%s : Read (%lu, %lu)", file_name, offset, length);
        azclient->DownloadToStream(file_name, os, offset, length);
        if (errno != 0)
        {
            if (errno == 416) {
                // Range given the request is invalid so we mark it as end of file
                errno = 0;
                return 0;
            }

            int storage_errno = errno;
            syslog(LOG_ERR, "Failed to download block of %s with offset %lu.  Errno = %d.\n", file_name, offset, storage_errno);
            return -storage_errno;
        }

        len = os.str().size();
        os.read(out, len);
        out[len] = '\0';
        return len;
    } 

    // Caching of block is allowed so we need to check block exists or not
    m_mutex.lock();
    auto iter = file_map.find(file_name);
    if(iter == file_map.end()) {
        m_mutex.unlock();
        return 0;
    }
    StreamObject* obj = iter->second;
    m_mutex.unlock();

    while (length > 0) {
        //  At max the data requested may overlap two blocks
        //  as soon as we get the full data we return back
        BlobBlock* block = GetBlock(file_name, offset, obj);
        if (block == NULL) {
            if (errno == 416) {
                // Range given the request is invalid so we mark it as end of file
                errno = 0;
                return 0;
            }
            // For some reason we failed to get the block object
            syslog(LOG_ERR, "Failed to get block for %s with offset %lu", file_name, offset);
            return -errno;
        }

        // Based on offset and block being used calculate the start offset inside the block
        uint64_t start_offset = offset - block->start_offset;
        uint64_t pending_data = block->data_length - start_offset;

        syslog(LOG_DEBUG, "%s : Read Block (%lu, %lu, %d)  Request (%lu, %lu)  Read (%lu, %lu)",
                file_name, block->start_offset, ((block->data_length + block->start_offset) - 1), block->last, 
                offset, length, 
                start_offset, pending_data);

        if (pending_data < length) {
            // Either request overlaps two blocks or request exceeds the file size
            // So read the remaining data from this block and decide
            block->buff.seekg(start_offset, std::ios::beg);
            block->buff.read((out + len), pending_data);
            block->lck.unlock();

            len += pending_data;

            if (block->last) {
                // This block is the last so terminate the loop
                //syslog(LOG_DEBUG, "%s read at offset %lu marks end of file with %d bytes", file_name, offset, len);
                length = 0;
            } else {
                // Data overlaps two blocks so we need to to partial read here
                length -= pending_data;
                offset += pending_data;
                //syslog(LOG_DEBUG, "%s read at offset %lu overlaps two blocks for %lu bytes", file_name, offset, length);
            }
        } else {
            // Data is fully available in this block so finish the read from this block and return
            block->buff.seekg(start_offset, std::ios::beg);
            block->buff.read((out + len), length);
            block->lck.unlock();

            len += length;
            length = 0;
        }
    }

    out[len] = '\0';
    return len;
}
