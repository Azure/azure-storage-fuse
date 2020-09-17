#include <thread>
#include <functional>
#include <unistd.h>
#include <sys/statvfs.h>
#include <sys/stat.h>
#include <fstream>
#include <sys/file.h>

#include <BlobfuseGcCache.h>
#include <BlobfuseGlobals.h>
#include <BlobfuseConstants.h>
#include "FileLockMap.h"

extern struct configParams config_options;

bool gc_cache::check_disk_space()
{
    double total;
    double available;
    unsigned long long used;

    if (config_options.cacheSize == 0) {
        struct statvfs buf;
        if(statvfs(cache_folder_path.c_str(), &buf) != 0)
        {
            return false;
        }

        //calculating the percentage of the amount of used space on the cached disk
        //<used space in bytes> = <total size of disk in bytes> - <size of available disk space in bytes>
        //<used percent of cached disk >= <used space> / <total size>
        //f_frsize - the fundamental file system block size (in bytes) (used to convert file system blocks to bytes)
        //f_blocks - total number of blocks on the filesystem/disk in the units of f_frsize
        //f_bfree - total number of free blocks in units of f_frsize
        total = buf.f_blocks * buf.f_frsize;
        available = buf.f_bfree * buf.f_frsize;
        used = total - available;
    } else {
        if (is_directory_empty(cache_folder_path.c_str()))
            m_current_usage = 0;

        total = config_options.cacheSize;
        used = m_current_usage;
    }
   
    double used_percent = (double)(used / total) * (double)100;

    if(used_percent >= HIGH_THRESHOLD_VALUE && !disk_threshold_reached)
    {
        return true;
    }
    else if(used_percent >= LOW_THRESHOLD_VALUE && disk_threshold_reached)
    {
        return true;
    }
    return false;
}

void gc_cache::uncache_file(std::string path)
{
    file_to_delete file;
    file.path = path;
    file.closed_time = time(NULL);

    // lock before updating deque
    std::lock_guard<std::mutex> lock(m_deque_lock);
    m_cleanup.push_back(file);
}

void gc_cache::addCacheBytes(std::string /*path*/, long int size)
{
    if (config_options.cacheSize > 0) {
        m_current_usage += size;
        if (m_current_usage > (config_options.cacheSize * 0.80)){
            AZS_DEBUGLOGV("Cache reaching its max value MAX : %llu, Current %llu",
                    config_options.cacheSize, m_current_usage);
        }
    }
}

void gc_cache::run()
{
    std::thread t1(std::bind(&gc_cache::run_gc_cache,this));
    t1.detach();
}

// cleanup function to clean cached files that are too old
void gc_cache::run_gc_cache()
{

    while(true){

        // lock the deque
        file_to_delete file;
        bool is_empty;
        {
            std::lock_guard<std::mutex> lock(m_deque_lock);
            is_empty = m_cleanup.empty();
            if(!is_empty)
            {
                file = m_cleanup.front();
            }
        }

        //if deque is empty, skip
        if(is_empty)
        {
            //run it every 1 second
            usleep(1000);
            continue;
        }

        time_t now = time(NULL);
        //check if the closed time is old enough to delete
        if(((now - file.closed_time) > file_cache_timeout_in_seconds) || disk_threshold_reached)
        {
            AZS_DEBUGLOGV("File %s being considered for deletion by file cache GC.\n", file.path.c_str());

            // path in the temp location
            const char * mntPath;
            std::string mntPathString = prepend_mnt_path_string(file.path);
            mntPath = mntPathString.c_str();

            //check if the file on disk is still too old
            //mutex lock
            auto fmutex = file_lock_map::get_instance()->get_mutex(file.path.c_str());
            std::lock_guard<std::mutex> lock(*fmutex);

            struct stat buf;
            stat(mntPath, &buf);
            if (((now - buf.st_mtime) > file_cache_timeout_in_seconds) ||
                disk_threshold_reached)
            {
                //clean up the file from cache
                int fd = open(mntPath, O_WRONLY);
                if (fd > 0)
                {
                    int flockres = flock(fd, LOCK_EX|LOCK_NB);
                    if (flockres != 0)
                    {
                        if (errno == EWOULDBLOCK)
                        {
                            // Someone else holds the lock.  In this case, we will postpone updating the cache until the next time open() is called.
                            // TODO: examine the possibility that we can never acquire the lock and refresh the cache.
                            AZS_DEBUGLOGV("Did not clean up file %s from file cache because there's still an open file handle to it.", mntPath);
                        }
                        else
                        {
                            // Failed to acquire the lock for some other reason.  We close the open fd, and continue.
                            syslog(LOG_ERR, "Did not clean up file %s from file cache because we failed to acquire the flock for an unknown reason, errno = %d.\n", mntPath, errno);
                        }
                    }
                    else
                    {
                        unlink(mntPath);
                        flock(fd, LOCK_UN);
                        
                        if (m_current_usage > (unsigned long long)buf.st_size)
                            m_current_usage -= buf.st_size;
                        else
                            m_current_usage = 0;
                        
                        //update disk space
                        disk_threshold_reached = check_disk_space();
                    }

                    close(fd);
                }
                else
                {
                    //TODO:if we can't open the file consistently, should we just try to move onto the next file?
                    //or somehow timeout on a file we can't open?
                    AZS_DEBUGLOGV("Failed to open file %s from file cache in GC, skipping cleanup. errno from open = %d.", mntPath, errno);
                }
            }

            // lock to remove from front
            {
                std::lock_guard<std::mutex> lock(m_deque_lock);
                m_cleanup.pop_front();
            }

        }
        else
        {
            // no file was timed out - let's wait a second
            usleep(1000);
            //check disk space
            disk_threshold_reached = check_disk_space();
        }
    }

}

std::string gc_cache::prepend_mnt_path_string(const std::string& path)
{
    std::string result;
    result.reserve(cache_folder_path.length() + 5 + path.length());
    return result.append(cache_folder_path).append("/root").append(path);
}
