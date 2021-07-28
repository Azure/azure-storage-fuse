#pragma once

#include <string>
#include <mutex>
#include <deque>

// deque to age cached files based on timeout
struct file_to_delete
{
    std::string path;
    time_t closed_time;
    bool force;
};

class gc_cache
{
public:
    gc_cache(std::string cache_folder, int timeout) :
        disk_threshold_reached(false),
        cache_folder_path(cache_folder),
        file_cache_timeout_in_seconds(timeout),
        m_current_usage(0){}
    void run();
    void uncache_file(std::string path, bool force = false);
    void addCacheBytes(std::string path, long int size);

private:
    bool disk_threshold_reached;
    std::string prepend_mnt_path_string(const std::string& path);
    std::deque<file_to_delete> m_cleanup;
    std::mutex m_deque_lock;
    void run_gc_cache();
    bool check_disk_space();
    std::string cache_folder_path;
    int file_cache_timeout_in_seconds;
    unsigned long long m_current_usage;
};
