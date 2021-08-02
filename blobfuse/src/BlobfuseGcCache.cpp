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
#include "Permissions.h"

extern struct configParams config_options;

bool gc_cache::check_disk_space()
{
    double total;
    double available;
    unsigned long long used;

    static bool user_detected = false;
    static bool is_sudo_user = false;

    if (!user_detected) {
        // If effective uid = 0 then we are running as sudo
        if (geteuid() == 0)
            is_sudo_user = true;
        user_detected = true;

        AZS_DEBUGLOGV("GC_Cache User identified. Privileged : %d", (int)(is_sudo_user));
    }

    if (config_options.cacheSize == 0) {
        struct statvfs buf;
        if(statvfs(cache_folder_path.c_str(), &buf) != 0)
        {
            AZS_DEBUGLOGV("GC_Cache statvfs failed with err : %d", errno);
            return false;
        }

        //calculating the percentage of the amount of used space on the cached disk
        //<used space in bytes> = <total size of disk in bytes> - <size of available disk space in bytes>
        //<used percent of cached disk >= <used space> / <total size>
        //f_frsize - the fundamental file system block size (in bytes) (used to convert file system blocks to bytes)
        //f_blocks - total number of blocks on the filesystem/disk in the units of f_frsize
        //f_bfree - total number of free blocks in units of f_frsize

        // f_bfree : Number of blocks free (root user)
        // f_bavail : Nukber of blocks available for unprivillaged user (non root)
        total = buf.f_blocks * buf.f_frsize;

        if (is_sudo_user)
            available = buf.f_bfree * buf.f_frsize;
        else    
            available = buf.f_bavail * buf.f_frsize;

        used = total - available;
    } else {
        if (is_directory_empty(cache_folder_path.c_str()))
            m_current_usage = 0;

        total = config_options.cacheSize;
        used = m_current_usage;
    }
   
    double used_percent = (double)(used / total) * (double)100;

    if(used_percent >= config_options.high_disk_threshold && !disk_threshold_reached)
    {
        return true;
    }
    else if(used_percent >= config_options.low_disk_threshold && disk_threshold_reached)
    {
        return true;
    }
    return false;
}

void gc_cache::uncache_file(std::string path, bool force)
{
    file_to_delete file;
    file.path = path;
    file.force = force;
    file.closed_time = time(NULL);

    // lock before updating deque
    std::lock_guard<std::mutex> lock(m_deque_lock);
    if (force) {
        // If a force delete is done due to fsync then put this file in front of the queue
        // so that this can be deleted early then other files waiting for expiry
        m_cleanup.push_front(file);
    } else {
        m_cleanup.push_back(file);
    }

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
    unsigned long long evicted = 0;

    while(true){

        // lock the deque
        file_to_delete file;
        bool is_empty;
        bool permissionsoverwritten = false;
        int originalpermissions = config_options.defaultPermission;
        {
            std::lock_guard<std::mutex> lock(m_deque_lock);
            is_empty = m_cleanup.empty();
            if(!is_empty)
            {
                file = m_cleanup.front();
            }

            if ((!is_empty) && 
                config_options.maxEviction > 0 && 
                evicted >= config_options.maxEviction) {
                is_empty = true;
            }
        }

        //if deque is empty, skip
        if(is_empty)
        {
            //run it every 1 second
            evicted = 0;
            usleep(config_options.cachePollTimeout);
            continue;
        }

        time_t now = time(NULL);
        //check if the closed time is old enough to delete
        if(file_cache_timeout_in_seconds == 0 ||
           disk_threshold_reached ||
           ((now - file.closed_time) > file_cache_timeout_in_seconds))
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
                if (errno == 13) {
                    AZS_DEBUGLOGV("GCClean ran into permission issues for opening WRONLY for file %s", mntPath);
                    int fdr = open(mntPath, O_RDONLY);
                    if (fdr > 0) {
                        errno = 0;
                        originalpermissions = buf.st_mode;
                        if (fchmod(fdr, config_options.defaultPermission) == 0) {
                            AZS_DEBUGLOGV("GCClean could not open for delete, so, Write protection overwritten for %s, resubmitting for cleanup", mntPath);
                            permissionsoverwritten = true;
                        }
                        else
                        {
                            AZS_DEBUGLOGV("GC cleanup fails with no write file permission and Unable to change permissions to file %s", mntPath);
                        }
                        //TODO: If there is lock issue below convert permissions back to what it was.
                        close(fdr);
                    }
                    else{
                         AZS_DEBUGLOGV("Failed to open file %s in read only mode errno = %d.", mntPath, errno);
                    }
                    fd = open(mntPath, O_WRONLY);
                }
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
                        if (permissionsoverwritten == true)
                        {
                            fchmod(fd, originalpermissions);
                        }
                    }
                    else
                    {
                        evicted++;
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
                else {
                    if (errno == 2)
                    {
                        AZS_DEBUGLOGV("File %s does not exist, gc cleanup not done",mntPath);
                    }
                    else{
                        //TODO:we are not putting the file back in the queue should we?
                        AZS_DEBUGLOGV("Failed to open file %s from file cache in GC, skipping cleanup. errno from open = %d.", mntPath, errno);
                        if (permissionsoverwritten == true)
                        {
                            fchmod(fd, originalpermissions);
                            AZS_DEBUGLOGV("Permissions overwriiten for file %s original %d, overwritten permission %d", mntPath, originalpermissions, config_options.defaultPermission);
                        }
                    }
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
            evicted = 0;
            usleep(config_options.cachePollTimeout);
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
