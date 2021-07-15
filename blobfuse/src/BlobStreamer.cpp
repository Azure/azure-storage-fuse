
#include <BlobStreamer.h>

const unsigned long long DOWNLOAD_CHUNK_SIZE = 16 * 1024 * 1024;

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
    if (m_block_list.size() >= max_blocks_per_file || 
        CacheSizeCalculator::GetObj()->MaxLimitReached()) {
        // We have exceeeded max number of blocks allowed so delete the end of the queue
        RemoveBlock();
    }

    // Add the new block to front of the list (LRU)
    CacheSizeCalculator::GetObj()->AddSize(block->buff.str().size());
    m_block_list.push_front(block);

    return 0;
}

// Remove a unused block from the list
int StreamObject::RemoveBlock()
{
    BlobBlock* last_block = m_block_list.back();
    m_block_list.pop_back();

    if (last_block) {
        // Release memory used by this block
        CacheSizeCalculator::GetObj()->RemoveSize(last_block->buff.str().size());
        last_block->buff.clear();
        delete last_block;
    }

    return 0;
}

// Search for a block based on offset
BlobBlock* StreamObject::GetBlock(uint64_t offset)
{
    // Start offsets are rounded off to 16MB multiples so just match the start offset
    for(auto it = m_block_list.begin(); it != m_block_list.end(); ++it) {
        if ((*it)->valid && (*it)->start == offset) {
            return (*it);
        }
    }

    return NULL;
}

// Cleanup the file info for this object as its no more nee
void StreamObject::Cleanup()
{
    while(m_block_list.size() > 0)
        RemoveBlock();
}



//  Get block of a given file based on give offset
BlobBlock* BlobStreamer::GetBlock(const char* file_name, uint64_t offset, StreamObject* obj)
{
    uint64_t start_offset = offset - (offset % DOWNLOAD_CHUNK_SIZE);

    obj->Lock();
    BlobBlock *block = obj->GetBlock(start_offset);

    if (block == NULL || block->valid == false) {
        // Either block was not found or its not valid so download a new block
        BlobBlock *block = new BlobBlock;
        block->start = start_offset;
        block->valid = true;
        block->last = false;
        
        azclient->DownloadToStream(file_name, block->buff, start_offset, DOWNLOAD_CHUNK_SIZE);
        uint32_t read_len = block->buff.str().size();
        block->end = block->start + read_len;

        if (read_len < DOWNLOAD_CHUNK_SIZE) {
            // We asked to read 16MB but got less data then assume its the end of file
            block->last = true;
        }

        obj->AddBlock(block, max_blocks_per_file);
    }
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
        GetBlock(file_name, 0, obj);
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
            m_mutex.lock();
            file_map.erase(file_name);
            delete obj;
            m_mutex.unlock();
        }
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
        len = os.str().size();
        os.read(out, len);
        out[len]= '\0';

    } else {
        // Caching of block is allowed so we need to check block exists or not
        m_mutex.lock();
        auto iter = file_map.find(file_name);
        if(iter == file_map.end()) {
            m_mutex.unlock();
            return 0;
        }
        StreamObject* obj = iter->second;
        m_mutex.unlock();

        BlobBlock* block = GetBlock(file_name, offset, obj);
        if (block == NULL){
            // For some reason we failed to get the block object
            return 0;
        }
        
        // Based on offset and block being used calculate the start offset inside the block
        int start_offset = offset - block->start;

        // As cached buffer is a stream seek to intended offset and read data from there
        block->buff.seekg(start_offset, std::ios::beg);
        block->buff.read(out, length);

        // Based on offset and length and available data calculate length of data being read
        len = block->buff.tellg();
        if (len < 0) {
            // we have hit end of the buffer
            len = block->buff.str().size() - start_offset;
        } else {
            len = (len - start_offset);
        }

        if (len < 0) {
            // Should not happen but just in case shit happens
            len = 0;
        }

        out[len] = '\0';

    } 

    return len;
}



