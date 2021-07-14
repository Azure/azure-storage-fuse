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

// Maximum number of blocks to be stored per file in case caching is enabled
#define MAX_BLOCKS_PER_FILE 3

// Structure representing one block segment
struct BlobBlock {
        bool                valid;  // Block is valid or not
        bool                last;   // This block is the last block of file
        uint64_t            start;  // Start offset of block
        uint64_t            end;    // End offset of block
        std::stringstream   buff;   // Buffer holding the data
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
        
        list<BlobBlock*>    m_block_list;   // List of blocks cached for this file
};


// BlobStreamer : Holds a map holding StreamObject for each file being open
class  BlobStreamer {
    public:
        BlobStreamer(std::shared_ptr<StorageBfsClientBase> client, uint64_t buffer_size, int max_blocks):
            blob_client(client)
        {
            nextHandle = 0;
            current_usage = 0;
            max_usage = buffer_size;
            max_blocks_per_file = max_blocks;
        }

        // As all data is cached in memory, there is no physical handle we can open so just return back a seq number for handle
        int GetDummyHandle() {
            std::lock_guard<std::mutex> lock(m_mutex);
            return ++nextHandle;
        }

        // Update the current usage
        void UpdateUSage(int size){
            std::lock_guard<std::mutex> lock(m_mutex);
            current_usage += size;
        }

        // Open file causes a new entry in map and caching its first buffer
        int OpenFile(const char* file_name);

        // Read file searches the map gets the object and gets the required block based on offset
        int ReadFile(const char* file_name, uint64_t offset, uint64_t length, char* out);

        // Close file decrements ref count and cleansup file info if all handles are closed
        int CloseFile(const char* file_name);

        // Search and add a new block if it does not exists for the given file
        BlobBlock* GetBlock(const char* file_name, uint64_t offset, StreamObject* obj);

    private:
        uint64_t        current_usage;          //  Current memory usage based on how many blocks are cached
        uint64_t        max_usage;              //  Max memory usage allowed by configuration for caching
        uint64_t        max_blocks_per_file;    //  Max number of blocks we can cache per file
        int             nextHandle;             //  Next available handle id
        std::mutex      m_mutex;                //  Mutex to sync the map
        

        std::map<std::string, StreamObject*>    m_file_map;  // Map holding stream object per file
        std::shared_ptr<StorageBfsClientBase>   blob_client; // Storage client object to download new blocks     

};

#endif