#ifndef BLOBSTREAMER_H_
#define BLOBSTREAMER_H_

#include <blobfuse.h>
#include <BlobfuseGlobals.h>
#include<StorageBfsClientBase.h>

#include <FileLockMap.h>
#include <list>

using namespace std;
using namespace azure::storage_lite;
using namespace azure::storage_adls;

class CacheSizeCalculator
{
    public:
        static CacheSizeCalculator* GetObj();

        void SetMaxSize(uint64_t max) {
            max_usage = max;
        }   

        void AddSize(uint64_t bytes) {
            std::lock_guard<std::mutex> lock(m_mutex);
            current_usage += bytes; 
        }

        void RemoveSize(uint64_t bytes) {
            std::lock_guard<std::mutex> lock(m_mutex);
            if (current_usage < bytes)
                current_usage = 0;
            else 
                current_usage -= bytes; 
        }

        uint64_t GetUsage() {
            return current_usage;
        }

        bool MaxLimitReached() {
            bool flag = current_usage >= max_usage;
            if (flag) {
                syslog(LOG_INFO, "Current block cache usage has reached the configured limit %lu > %lu", current_usage, max_usage);
            }

            return flag;
        }

    private:

        CacheSizeCalculator() {
            current_usage = max_usage = 0;
        }

        static CacheSizeCalculator* mInstance;  // Static object for singleton class

        uint64_t        current_usage;          //  Current memory usage based on how many blocks are cached
        uint64_t        max_usage;              //  Max memory usage allowed by configuration for caching
        std::mutex      m_mutex;                //  Mutex to sync the map

};

// Structure representing one block segment
struct BlobBlock {
        bool                valid;          // Block is valid or not
        bool                last;           // This block is the last block of file
        uint64_t            start_offset;    // Start offset of block
        uint64_t            data_length;    // End offset of block
        std::stringstream   buff;           // Buffer holding the data
        std::mutex          lck;            // No one shall delete the block when someone is reading it

        BlobBlock() {
            valid = false;
            last = false;
            start_offset = 0;
            data_length = 0;
        }
};

// StreamObject : Holds all available blocks for a given file
class StreamObject {
    public:
        StreamObject() {
            ref_count = 0;
        }

        int IncRefCount() {
            std::lock_guard<std::mutex> lock(m_mutex);
            ref_count++;
            return ref_count;
        }

        int DecRefCount() {
            std::lock_guard<std::mutex> lock(m_mutex);
            ref_count--;

            if (ref_count == 0) {
                Cleanup();
            }

            return ref_count;
        }

        int GetRefCount() {
            std::lock_guard<std::mutex> lock(m_mutex);
            return ref_count;
        }

        void Lock() {
            m_mutex.lock();
        }

        void UnLock() {
            m_mutex.unlock();
        }

        uint64_t GetCacheListSize() {
            return uint64_t(m_block_cache_list.size());
        }


        // Add a new block for this file
        int AddBlock(BlobBlock* block, uint64_t max_blocks);

        // Remove existing block for this file
        int RemoveBlock();

        // Get the block based on offset and length, if not add it
        BlobBlock* GetBlock(uint64_t offset);

        // Remove all blocks and wipe out this file info
        void Cleanup();

    private:
        int                 ref_count;      // How many open handles are there for this file
        std::mutex          m_mutex;        // Mutex for safety

        list<BlobBlock*>    m_block_cache_list;   // List of blocks cached for this file
};


// BlobStreamer : Holds a map holding StreamObject for each file being open
class  BlobStreamer {
    public:
        BlobStreamer(std::shared_ptr<StorageBfsClientBase> client, uint64_t buffer_size, int max_blocks, uint64_t blk_size):
            azclient(client)
        {
            if (buffer_size == 0) {
                buffer_size = INT64_MAX;
            }

            CacheSizeCalculator::GetObj()->SetMaxSize(buffer_size);
            nextHandle = 0;
            max_blocks_per_file = max_blocks;
            block_size = blk_size;
        }

        // As all data is cached in memory, there is no physical handle we can open so just return back a seq number for handle
        int GetDummyHandle() {
            std::lock_guard<std::mutex> lock(m_mutex);
            return ++nextHandle;
        }

        // Function to get a block for the file from cache or download a new one
        BlobBlock* GetBlock(const char* file_name, uint64_t offset, StreamObject* obj);

        // Open file causes a new entry in map and caching its first buffer
        int OpenFile(const char* file_name);

        // Read file searches the map gets the object and gets the required block based on offset
        int ReadFile(const char* file_name, uint64_t offset, uint64_t length, char* out);

        // Close file decrements ref count and cleansup file info if all handles are closed
        int CloseFile(const char* file_name);

        // Delete file removes all the buffers in the memory
        int DeleteFile(const char* file_name);

    private:
        uint64_t        max_blocks_per_file;    //  Max number of blocks we can cache per file
        uint64_t        block_size;             //  Size of each block to be downloaded
        int             nextHandle;             //  Next available handle id
        std::mutex      m_mutex;                //  Mutex to sync the map

        std::map<std::string, StreamObject*>    file_map;           // Map holding stream object per file
        std::shared_ptr<StorageBfsClientBase>   azclient;        // Storage client object to download new blocks     

};

#endif