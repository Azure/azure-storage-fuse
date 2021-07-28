
#include <BlobStreamer.h>
#include <base64.h>

const unsigned long long DOWNLOAD_CHUNK_SIZE = 16 * 1024 * 1024;
const unsigned long long DOWNLOAD_SINGLE_CUNK_FILE_SIZE = 64 * 1024 * 1024;

std::string GenerateBlockId(unsigned int idx) {
    std::string raw_block_id = std::to_string((int)idx);
    //pad the string to length of 6.
    raw_block_id.insert(raw_block_id.begin(), 12 - raw_block_id.length(), '0');
    const std::string block_id_un_base64 = raw_block_id + get_uuid();
    const std::string block_id(azure::storage_lite::to_base64(reinterpret_cast<const unsigned char*>(block_id_un_base64.c_str()), block_id_un_base64.size()));
    put_block_list_request_base::block_item block;
    return raw_block_id;
}

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
    CacheSizeCalculator::GetObj()->AddSize(block->length);
    m_block_cache_list.push_front(block);

    return 0;
}

// Remove a unused block from the list
int StreamObject::RemoveBlock()
{
    BlobBlock* last_block = m_block_cache_list.back();

    // Lock it so that we wait untill any reader is using this block
    // then we remove it from the list and unlock, as its not in the list now,
    // no one can search or use it anymore
    last_block->lck.lock();
    m_block_cache_list.pop_back();
    last_block->lck.unlock();

    if (last_block) {
        // Release memory used by this block
        CacheSizeCalculator::GetObj()->RemoveSize(last_block->length);
        last_block->buff.clear();
        delete last_block;
    }

    return 0;
}

// Search for a block based on offset
BlobBlock* StreamObject::GetBlock(uint64_t offset)
{
    // Start offsets are rounded off to 16MB multiples so just match the start offset
    for(auto it = m_block_cache_list.begin(); it != m_block_cache_list.end(); ++it) {
        if ((*it)->valid && (*it)->start == offset) {
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
    unsigned long long download_chunk_size = DOWNLOAD_CHUNK_SIZE;
    uint64_t start_offset = offset - (offset % DOWNLOAD_CHUNK_SIZE);

    obj->Lock();

    if (obj->GetCacheListSize() == 0 && offset == 0) {
        // There is nothing cached and this if the first request to download the block.
        // So get the block list here and keep it handy for future
        obj->SetBlockIdList(azclient->GetBlockList(file_name));
    }

    if (obj->IsSingleBlockFile()) {
        // This file was uploaded as a single block of max 64MB so download entire data in one shot
        // as the block list will be empty and later we can not download based on block id
        download_chunk_size = DOWNLOAD_SINGLE_CUNK_FILE_SIZE;
        start_offset = offset - (offset % DOWNLOAD_SINGLE_CUNK_FILE_SIZE);
    }
    
    BlobBlock *block = obj->GetBlock(start_offset);

    if (block == NULL || block->valid == false) {
        // Either block was not found or its not valid so download a new block
        syslog(LOG_DEBUG, "File %s Block offset %lu : not found. Download and cache", file_name, offset);

        block = new BlobBlock;
        block->start = start_offset;
        block->valid = true;
        block->last = false;

        azclient->DownloadToStream(file_name, block->buff, start_offset, download_chunk_size);
        if (errno != 0)
        {
            obj->UnLock();

            int storage_errno = errno;
            if (errno == 416) {
                syslog(LOG_ERR, "Failed to download block of %s with offset %lu.  Errno = %d (Out of range).\n", file_name, start_offset, storage_errno);
                errno = storage_errno;
                return NULL;
            }
            
            syslog(LOG_ERR, "Failed to download block of %s with offset %lu.  Errno = %d.\n", file_name, start_offset, storage_errno);
            return NULL;
        }
        
        block->length = block->buff.str().size();
        if (block->length < download_chunk_size) {
            // We asked to read 16MB but got less data then assume its the end of file
            syslog(LOG_ERR, "File %s Block offset %lu : is last block", file_name, offset);
            block->last = true;
        }

        obj->AddBlock(block, max_blocks_per_file);

    }
    block->lck.lock();
    obj->UnLock();

    return block;
}

// Upload the blck based on its offset
int BlobStreamer::UploadBlock(const char* file_name, StreamObject* obj, BlobBlock* block)
{
    uint64_t block_len = block->buff.str().size();

    obj->Lock();
    if (obj->IsSingleBlockFile()) {
        // If the file is small then it may be having only one block and not the list
        if (block_len <= DOWNLOAD_SINGLE_CUNK_FILE_SIZE) {
            // It can still fit in 64MB block so not need to break it down
            errno = 0;
            azclient->UploadFromStream(block->buff, file_name);
            if (errno != 0) {
                syslog(LOG_ERR,"Failed to upload block from stream for file %s, block offset : %ld, length : %ld", file_name, block->start, block->length);
                return -errno;
            }
        } else {
            // We need to break this down to 16BM chunks here
            unsigned int idx = 0;
            while (block_len > 0) {
                BlockIdItem item;
                item.name = GenerateBlockId(idx++);
                item.begin = (idx * DOWNLOAD_CHUNK_SIZE);
                item.size = (block_len > DOWNLOAD_CHUNK_SIZE) ? DOWNLOAD_CHUNK_SIZE : block_len;
                item.type =  azure::storage_lite::put_block_list_request_base::block_type::uncommitted;

                azclient->UploadBlockWithID(file_name, item.name, (block->buff.str().c_str() + item.begin), item.size);
                if (errno != 0) {
                    syslog(LOG_ERR,"Failed to upload block for file %s, block offset : %lld, length : %lld", file_name, item.begin, item.size);
                    return -errno;
                }
                obj->m_block_id_list.push_back(item);
                block_len -= item.size;
            }
        }
    } else {
        for (unsigned int i = 0; i < obj->m_block_id_list.size(); i++) {
            if (obj->m_block_id_list[i].begin == block->start) {
                if (block_len <= DOWNLOAD_CHUNK_SIZE) {
                    // Post writing also block size is not going beyond 16MB
                    errno = 0;
                    azclient->UploadBlockWithID(file_name, obj->m_block_id_list[i].name, block->buff.str().c_str(), block_len);
                    if (errno != 0) {
                        syslog(LOG_ERR,"Failed to upload block for file %s, block offset : %ld, length : %ld", file_name, block->start, block->length);
                        return -errno;
                    }
                    obj->m_block_id_list[i].type =  azure::storage_lite::put_block_list_request_base::block_type::uncommitted;
                    break;
                } else {
                    // Post write block size has gone beyond 16MB so we need to break this down to multiple blocks now
                    obj->m_block_id_list.pop_back();
                    while (block_len > 0) {
                        BlockIdItem item;
                        item.name = GenerateBlockId(i++);
                        item.begin = (i * DOWNLOAD_CHUNK_SIZE);
                        item.size = (block_len > DOWNLOAD_CHUNK_SIZE) ? DOWNLOAD_CHUNK_SIZE : block_len;
                        item.type =  azure::storage_lite::put_block_list_request_base::block_type::uncommitted;

                        azclient->UploadBlockWithID(file_name, item.name, (block->buff.str().c_str() + item.begin), item.size);
                        if (errno != 0) {
                            syslog(LOG_ERR,"Failed to upload block for file %s, block offset : %lld, length : %lld", file_name, item.begin, item.size);
                            return -errno;
                        }
                        obj->m_block_id_list.push_back(item);
                        block_len -= item.size;
                    }
                }
            }
        }
    }
    obj->UnLock();

    return 0;
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
    obj->Lock();
    file_map.erase(file_name);
    m_mutex.unlock();

    obj->Cleanup();
    delete obj;
    return 0;
}

// FlushFile will update the block list to the container
int BlobStreamer::FlushFile(const char* file_name)
{
    // Caching of block is allowed so we need to check block exists or not
    m_mutex.lock();
    auto iter = file_map.find(file_name);
    if(iter == file_map.end()) {
        m_mutex.unlock();
        return 0;
    }
    StreamObject* obj = iter->second;
    obj->Lock();
    m_mutex.unlock();
    
    if (obj->IsSingleBlockFile()) {
        obj->UnLock();
        return 0;
    }

    errno = 0;
    std::vector<azure::storage_lite::put_block_list_request_base::block_item> block_list;

    for (unsigned int i = 0; i < obj->m_block_id_list.size(); i++) {
        azure::storage_lite::put_block_list_request_base::block_item item;
        item.id = obj->m_block_id_list[i].name;
        item.type = obj->m_block_id_list[i].type;
        block_list.push_back(item);
    }

    azclient->PutBlockList(file_name, block_list, std::vector<std::pair<std::string, std::string>>());
    obj->UnLock();

    if (errno != 0) {
        int storage_errno = errno;
        syslog(LOG_ERR, "Failed to upload block listof %s Errno = %d.\n", file_name, storage_errno);
        return -storage_errno;
    }

    for (unsigned int i = 0; i < obj->m_block_id_list.size(); i++) {
        obj->m_block_id_list[i].type = azure::storage_lite::put_block_list_request_base::block_type::committed;
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
        
        azclient->DownloadToStream(file_name, os, offset, length);
        if (errno != 0)
        {
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
        uint64_t start_offset = offset - block->start;
        uint64_t pending_data = block->length - start_offset;

        syslog(LOG_ERR, "%s : Read Block (%lu, %lu, %d)  Request (%lu, %lu)  Read (%lu, %lu)",
                file_name, block->start, ((block->length + block->start) - 1), block->last, 
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
                syslog(LOG_ERR, "%s read at offset %lu marks end of file with %d bytes", file_name, offset, len);
                length = 0;
            } else {
                // Data overlaps two blocks so we need to to partial read here
                length -= pending_data;
                offset += pending_data;
                syslog(LOG_ERR, "%s read at offset %lu overlaps two blocks for %lu bytes", file_name, offset, length);
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



// WriteFile will get the requested block and update the data in it and upload the block list again to container 
int BlobStreamer::WriteFile(const char* file_name, uint64_t offset, uint64_t length, const char* data)
{
    int len = 0;
    
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
        uint64_t start_offset = offset - block->start;
        uint64_t pending_data = block->length - start_offset;

        syslog(LOG_ERR, "%s : Write Block (%lu, %lu, %d)  Request (%lu, %lu)  Read (%lu, %lu)",
                file_name, block->start, ((block->length + block->start) - 1), block->last, 
                offset, length, 
                start_offset, pending_data);

        if (obj->IsSingleBlockFile() || block->last) {
            // This file is single block file (< 64MB) so far so there is no block id list created for this.
            // Just write the data to the uni-block and be done with it
            block->buff.seekp(start_offset, std::ios::beg);
            block->buff.write((data + len), length);
            len += length;
            length = 0;
        } else if (pending_data < length) {
            // Either request overlaps two blocks or request exceeds the file size
            // So read the remaining data from this block and decide
            block->buff.seekp(start_offset, std::ios::beg);
            block->buff.write((data + len), pending_data);
            
            len += pending_data;

            if (block->last) {
                // This block is the last so terminate the loop
                syslog(LOG_ERR, "%s read at offset %lu marks end of file with %d bytes", file_name, offset, len);
                length = 0;
            } else {
                // Data overlaps two blocks so we need to to partial read here
                length -= pending_data;
                offset += pending_data;
                syslog(LOG_ERR, "%s read at offset %lu overlaps two blocks for %lu bytes", file_name, offset, length);
            }
        } else {
            // Data is fully available in this block so finish the read from this block and return
            block->buff.seekp(start_offset, std::ios::beg);
            block->buff.write((data + len), length);
            len += length;
            length = 0;
        }

        UploadBlock(file_name, obj, block);
        block->lck.unlock();
    }

    return len;
}
