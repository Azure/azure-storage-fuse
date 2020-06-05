#ifndef FILE_LOCK_MAP_H
#define FILE_LOCK_MAP_H

#include <mutex>
#include <map>
#include <memory>

// We use two different locking schemes to protect files / blobs against data corruption and data loss scenarios.
// The first is an in-memory std::mutex, the second is flock (Linux).  Each file path gets its own mutex and flock lock.
// The in-memory mutex should only be held while control is in a method that is directly communicating with Azure Storage.
// The flock lock should be held continuously, from the time that the file is opened until the time that the file is closed.  It should also be held during blob download and upload.
// Blob download should hold the flock lock in exclusive mode.  Read/write operations should hold it in shared mode.
// Explanations for why we lock in various places are in-line.

// This class contains mutexes that we use to lock file paths during blob upload / download / delete.
// Each blob / file path gets its own mutex.
// This mutex should never be held when control is not in an open(), flush(), or unlink() method.
class file_lock_map
{
public:
    static file_lock_map* get_instance();
    std::shared_ptr<std::mutex> get_mutex(const std::string& path);

private:
    file_lock_map()
    {
    }

    static std::shared_ptr<file_lock_map> s_instance;
    static std::mutex s_mutex;
    std::mutex m_mutex;
    std::map<std::string, std::shared_ptr<std::mutex>> m_lock_map;
};
#endif //FILE_LOCK_MAP_H