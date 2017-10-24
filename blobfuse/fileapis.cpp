#include "blobfuse.h"
#include <sys/file.h>

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
    static file_lock_map* get_instance()
    {
        if(nullptr == _instance.get())
        {
            std::lock_guard<std::mutex> lock(s_mutex);
            if(nullptr == _instance.get())
            {
                _instance.reset(new file_lock_map());
            }
        }
        return _instance.get();
    }

    std::shared_ptr<std::mutex> get_mutex(std::string path)
    {
        std::lock_guard<std::mutex> lock(m_mutex);
        auto iter = m_lock_map.find(path);
        if(iter == m_lock_map.end())
        {
            auto file_mutex = std::make_shared<std::mutex>();
            m_lock_map[path] = file_mutex;
            return file_mutex;
        }
        else
        {
            return iter->second;
        }
    }

    std::shared_ptr<std::mutex> get_mutex(const char* path)
    {
        std::string spath(path);
        return get_mutex(spath);
    }

    protected:
    file_lock_map()
    {
    }

    private:
    static std::shared_ptr<file_lock_map> _instance;
    static std::mutex s_mutex;
    std::mutex m_mutex;
    std::map<std::string, std::shared_ptr<std::mutex>> m_lock_map;
};

std::shared_ptr<file_lock_map> file_lock_map::_instance;
std::mutex file_lock_map::s_mutex;

// Opens a file for reading or writing
// Behavior is defined by a normal, open() system call.
// In all methods in this file, the variables "path" and "pathString" refer to the input path - the path as seen by the application using FUSE as a file system.
// The variables "mntPath" and "mntPathString" refer to on-disk cached location of the corresponding file/blob.
int azs_open(const char *path, struct fuse_file_info *fi)
{
    if (AZS_PRINT)
    {
        fprintf(stdout, "azs_open called with path = %s, fi->flags = %X, O_TRUNC = %d. \n", path, fi->flags, ((fi->flags & O_TRUNC) == O_TRUNC));
    }
    std::string pathString(path);
    const char * mntPath;
    std::string mntPathString = prepend_mnt_path_string(pathString);
    mntPath = mntPathString.c_str();

    // Here, we lock the file path using the mutex.  This ensures that multiple threads aren't trying to create and download the same blob/file simultaneously.
    // We cannot use "flock" to prevent against this, because a) the file might not yet exist, and b) flock locks do not persist across file delete / recreate operations, and file renames.
    auto fmutex = file_lock_map::get_instance()->get_mutex(path);
    std::lock_guard<std::mutex> lock(*fmutex);

    // If the file/blob being opened does not exist in the cache, or the version in the cache is too old, we need to download / refresh the data from the service.
    struct stat buf;
    int statret = stat(mntPath, &buf);
    if ((statret != 0) || ((time(NULL) - buf.st_atime) > file_cache_timeout_in_seconds))  // TODO: Consider using "modified time" here, rather than "access time".
    {
        remove(mntPath);

        if(0 != ensure_files_directory_exists(mntPathString))
        {
            fprintf(stderr, "Failed to create file or direcotry on cache directory: %s, errno = %d.\n", mntPathString.c_str(),  errno);
            return -1;
        }
        std::ofstream filestream(mntPathString, std::ofstream::binary | std::ofstream::out);
        errno = 0;
        int fd = open(mntPath, O_WRONLY);
        if (fd == -1)
        {
            return -errno;
        }
        errno = 0;

        // Here, we take an exclusive flock lock on the file in the cache.  This prevents any other process/thread from accessing or modifying the file while it's being downloaded from the service.
        // Without this, we have the potential for data loss, if one process is writing data to a file while another is downloading the data in Storage to the file.
        // This operation cannot deadlock with the mutex acquired above, because we acquire the lock in non-blocking mode.
        int flockres = flock(fd, LOCK_EX|LOCK_NB);
        if (flockres != 0)
        {
            if (errno == EWOULDBLOCK)
            {
                // Someone else holds the lock.  In this case, we will postpone updating the cache until the next time open() is called.
                // TODO: examine the possibility that we can never acquire the lock and refresh the cache.
            }
            else
            {
                // Failed to acquire the lock for some other reason.  We close the open fd, and fail.
                int flockerrno = errno;
                close(fd);
                return -flockerrno;
            }
        }
        else
        {
            // We have the exclusive lock on the file, we are safe to download it from the service.
            errno = 0;
            azure_blob_client_wrapper->download_blob_to_stream(str_options.containerName, pathString.substr(1), 0ULL, 1000000000000ULL, filestream);
            int storage_errno = errno;
            flock(fd, LOCK_UN);
            close(fd);
            if (storage_errno != 0)
            {
                return 0 - map_errno(storage_errno);
            }
            if (errno != 0)
            {
                return -errno;
            }
        }
    }

    errno = 0;
    int res;

    // Open a file handle to the file in the cache.
    // This will be stored in 'fi', and used for later read/write operations.
    res = open(mntPath, fi->flags);
    if (AZS_PRINT)
    {
        printf("Accessing %s gives res = %d, errno = %d, ENOENT = %d, processID = %d\n", mntPath, res, errno, ENOENT, getpid());
    }

    if (res == -1)
    {
        return -errno;
    }

    // At this point, the file exists in the cache and we have an open file handle to it.  We now attempt to acquire the flock lock in shared mode, to be held while reading and writing to the file. 
    if((fi->flags&O_NONBLOCK) == O_NONBLOCK)
    {
        if(0 != flock(res, LOCK_SH|LOCK_NB))
        {
            if (AZS_PRINT)
            {
                printf("flock error in NB.  errno = %d, res = %d\n", errno, res);
            }
            int flockerrno = errno;
            close(res);
            return 0 - flockerrno;
        }
    }
    else
    {
        if (0 != flock(res, LOCK_SH))
        {
            if (AZS_PRINT)
            {
                printf("flock error.  errno = %d, res = %d\n", errno, res);
            }
            int flockerrno = errno;
            close(res);
            return 0 - flockerrno;            
        }
    }

    // TODO: Actual access control
    fchmod(res, 0700);

    // Stora the open file handle, and whether or not the file should be uploaded on close().
    // TODO: Optimize the scenario where the file is open for read/write, but no actual writing occurs, to not upload the blob.
    struct fhwrapper *fhwrap = new fhwrapper(res, (((fi->flags & O_WRONLY) == O_WRONLY) || ((fi->flags & O_RDWR) == O_RDWR)));
    fi->fh = (long unsigned int)fhwrap; // Store the file handle for later use.
//    }
    return 0;
}

/**
 * Read data from the file (the blob) into the input buffer
 * @param  path   Path of the file (blob) to read from
 * @param  buf    Buffer in which to copy the data
 * @param  size   Amount of data to copy
 * @param  offset Offset in the file (the blob) from which to begin reading.
 * @param  fi     File info for this file.
 * @return        TODO: Error codes
 */
int azs_read(const char *path, char *buf, size_t size, off_t offset, struct fuse_file_info *fi)
{
    std::string pathString(path);
    const char * mntPath;
    std::string mntPathString = prepend_mnt_path_string(pathString);
    mntPath = mntPathString.c_str();
    int fd;
    int res;

    (void) fi;
    if (fi == NULL)
        fd = open(mntPath, O_RDONLY);
    else
        fd = ((struct fhwrapper *)fi->fh)->fh;

    if (fd == -1)
        return -errno;

    res = pread(fd, buf, size, offset);
    if (res == -1)
        res = -errno;

    if (fi == NULL)
        close(fd);
    return res;
}

// Note that in FUSE, create is not the same as open with specific flags (the way it is in Linux)
// See the FUSE docs on these methods for more details.
int azs_create(const char *path, mode_t mode, struct fuse_file_info *fi)
{
    if (AZS_PRINT)
    {
        fprintf(stdout, "azs_create called with path = %s, mode = %d, fi->flags = %x\n", path, mode, fi->flags);
    }

    auto fmutex = file_lock_map::get_instance()->get_mutex(path);
    std::lock_guard<std::mutex> lock(*fmutex);

    std::string pathString(path);
    const char * mntPath;
    std::string mntPathString = prepend_mnt_path_string(pathString);
    mntPath = mntPathString.c_str();
    int res;

    int mntPathLength = strlen(mntPath);
    char *mntPathCopy = (char *)malloc(mntPathLength + 1);
    memcpy(mntPathCopy, mntPath, mntPathLength);
    mntPathCopy[mntPathLength] = 0;

    // Have to create any directories that don't already exist in the path to the file.
    // // TODO: Change this to use the 'ensure_directory'exists' method.
    char *cur = mntPathCopy + 1;
    cur = strchr(cur, '/');
    while (cur)
    {
        *cur = 0;
        if (AZS_PRINT)
        {
            fprintf(stdout, "Now validating and possibly creating %s\n", mntPathCopy);
        }
        if (access(mntPathCopy, F_OK) != 0)
        {
            mkdir(mntPathCopy, 0700);
        }
        *cur = '/';
        cur = cur + 1;
        cur = strchr(cur, '/');
    }
    res = open(mntPath, fi->flags, mode);
    if (AZS_PRINT)
    {
        fprintf(stdout, "mntPath = %s, result = %d\n", mntPath, res);
    }
    if (res == -1)
    {
        if (AZS_PRINT)
        {
            fprintf(stdout, "Error in open, errno = %d\n", errno);
        }
        return -errno;
    }

    struct fhwrapper *fhwrap = new fhwrapper(res, true);
    fi->fh = (long unsigned int)fhwrap;
    return 0;
}

/**
 * Write data to the file.
 *
 * Here, we are still just writing data to the local buffer, not forwarding to Storage.
 * TODO: possible in-memory caching?
 * TODO: for very large files, start uploading to Storage before all the data has been written here.
 * @param  path   Path to the file to write.
 * @param  buf    Buffer containing the data to write.
 * @param  size   Amount of data to write
 * @param  offset Offset in the file to write the data to
 * @param  fi     Fuse file info, containing the fh pointer
 * @return        TODO: Error codes.
 */
int azs_write(const char *path, const char *buf, size_t size, off_t offset, struct fuse_file_info *fi)
{
    std::string pathString(path);
    const char * mntPath;
    std::string mntPathString = prepend_mnt_path_string(pathString);
    mntPath = mntPathString.c_str();
    int fd;
    int res;

    (void) fi;
    if (fi == NULL)
        fd = open(mntPath, O_WRONLY);
    else
        fd = ((struct fhwrapper *)fi->fh)->fh;

    if (fd == -1)
        return -errno;

    res = pwrite(fd, buf, size, offset);
    if (res == -1)
        res = -errno;

    if (fi == NULL)
        close(fd);
    return res;
}

int azs_flush(const char *path, struct fuse_file_info *fi)
{
    // At this point, the shared flock will be held.
    if (AZS_PRINT)
    {
        fprintf(stdout, "azs_flush called with path = %s, fi->flags = %d, (((struct fhwrapper *)fi->fh)->fh) = %d, pid = %d\n", path, fi->flags, (((struct fhwrapper *)fi->fh)->fh), getpid());
    }

    // In some cases, due (I believe) to us using the hard_unlink option, path will be null.  Thus, we need to get the file name from the file descriptor:

    char path_link_buffer[50];
    snprintf(path_link_buffer, 50, "/proc/self/fd/%d", (((struct fhwrapper *)fi->fh)->fh));

    // canonicalize_file_name will follow symlinks to give the actual path name.
    char *path_buffer = canonicalize_file_name(path_link_buffer);
    if (path_buffer == NULL)
    {
        if (AZS_PRINT)
        {
            fprintf(stdout, "Skipped blob upload because file no longer exists.\n");
        }
        return 0;
    }

    if (AZS_PRINT)
    {
        fprintf(stdout, "path_link_buffer = %s, path_buffer = %s\n", path_link_buffer, path_buffer);
    }

    // Note that we don't have to prepend the tmpPath, because we already have it, because we're not using the input path but instead are querying for it.
    std::string mntPathString(path_buffer);
    const char * mntPath = path_buffer;
    if (AZS_PRINT)
    {
        fprintf(stdout, "Now accessing %s.\n", mntPath);
    }
    if (access(mntPath, F_OK) != -1 )
    {
        // We cannot close the actual file handle to the temp file, because of the possibility of flush being called multiple times for a given call to open().
        // For some file systems, however, close() flushes data, so we do want to do that before uploading data to a blob.
        // The solution (taken from the FUSE documentation) is to close a duplicate of the file descriptor.
        close(dup(((struct fhwrapper *)fi->fh)->fh));
        if (((struct fhwrapper *)fi->fh)->upload)
        {
            // Here, we acquire the mutex on the file path.  This is necessary to guard against several race conditions.
            // For example, say that a cache refresh is triggered.  There is a small window of time where the file has been removed and not yet re-downloaded.
            // If the blob upload occurred during that window, this could result in the blob being over-written with a zero-length blob, causing data loss.
            // An flock exclusive lock is not good enough here, because it does not hold across unlink and re-creates, and because the flosk is not acquired in open() before remove() is called during cache refresh.
            // We are not concerned with the possibility of writes from another process occurring during blob upload, because when that other process flushes the file, it will re-upload the blob, correcting any potential errors.
            auto fmutex = file_lock_map::get_instance()->get_mutex(path_buffer);
            std::lock_guard<std::mutex> lock(*fmutex);

            // Check to ensure that the file still exists; that unlink() hasn't been called previously.
            struct stat buf;
            int statret = stat(mntPath, &buf);
            if (statret != 0)
            {
                if (errno == ENOENT)
                {
                    // If the file in the cache no longer exists, that means unlink() was called on some other thread/process, since we opened the file.
                    // In this case, we do not want to upload a zero-length blob to the service or error out, we want to silently discard any data that has been written and
                    // and with no blob on the service or in the cache.
                    // This mimics the behavior of a real file system.
                    
                    //delete(path_buffer);
                    free(path_buffer);
                    if (AZS_PRINT)
                    {
                        fprintf(stdout, "Skipped blob upload because file no longer exists, special race-condition logic.\n");
                    }
                    return 0;
                }
                else
                {
                    //delete(path_buffer);
                    free(path_buffer);
                    return -errno;
                }
            }

            // TODO: This will currently upload the full file on every flush() call.  We may want to keep track of whether
            // or not flush() has been called already, and not re-upload the file each time.
            std::vector<std::pair<std::string, std::string>> metadata;
            errno = 0;
            azure_blob_client_wrapper->upload_file_to_blob(mntPath, str_options.containerName, mntPathString.substr(str_options.tmpPath.size() + 6 /* there are six characters in "/root/" */), metadata, 8);
            if (errno != 0)
            {
                    //delete(path_buffer);
                    free(path_buffer);
                return 0 - map_errno(errno);
            }
        }
    }
    else
    {
        if (AZS_PRINT)
        {
            fprintf(stdout, "Skipped blob upload because file no longer exists.\n");
        }

    }
    //delete(path_buffer);
    free(path_buffer);
    return 0;
}

// Note that there is not much point in doing error-checking in this method, as release() does not offer a way to communicate any errors with the caller (it's called async with the thread that called close())
int azs_release(const char *path, struct fuse_file_info * fi)
{
    if (AZS_PRINT)
    {
        fprintf(stdout, "azs_release called with path = %s, fi->flags = %d\n", path, fi->flags);
    }
    std::string pathString(path);
    const char * mntPath;
    std::string mntPathString = prepend_mnt_path_string(pathString);
    mntPath = mntPathString.c_str();
    if (AZS_PRINT)
    {
        fprintf(stdout, "Now accessing %s.\n", mntPath);
    }
    if (access(mntPath, F_OK) != -1 )
    {
        // Unlock the file and close the file handle.
        // Note that this will release the shared lock acquired in the corresponding open() call (the one that gave us this file descriptor, in the fuse_file_info).
        // It will not release any locks acquired from other calls to open(), in this process or in others.
        flock(((struct fhwrapper *)fi->fh)->fh, LOCK_UN);
        close(((struct fhwrapper *)fi->fh)->fh);
    }
    else
    {
        if (AZS_PRINT)
        {
            fprintf(stdout, "Access failed.\n");
        }
    }
    return 0;
}

int azs_unlink(const char *path)
{
    if (AZS_PRINT)
    {
        fprintf(stdout, "azs_unlink called with path = %s\n", path);
    }
    std::string pathString(path);
    const char * mntPath;
    std::string mntPathString = prepend_mnt_path_string(pathString);
    mntPath = mntPathString.c_str();
    if (AZS_PRINT)
    {
        fprintf(stdout, "deleting file %s\n", mntPath);
    }

    // We must hold the mutex here, otherwise there is a potential race condition in the following scenario:
    // 1. Process A opens a file for writing and writes to it.
    // 2. Process B calls "unlink"
    // 3. Process A flushes and closes the file.
    // In this case, the file (blob) should not exist.  When process A closes the file, it's closing a file handle that's been unlinked from the directory tree, so any data in the file should be discarded when all handles/links to the file are closed.
    // Most of the time, this will work here as well, because when we upload the blob in flush(), the Azure Storage C++ Lite library acquires a new handle to the file to upload it.  If the file has been unlinked during this time, no data will be uploaded.
    // However, there is a potential race condition.  If unlink() is called in between the Azure Storage C++ Lite library opening the file (for upload), and actually uploading the data, data may be successfully uploaded.
    // Acquiring the mutex here guards against that condition.
    auto fmutex = file_lock_map::get_instance()->get_mutex(path);
    std::lock_guard<std::mutex> lock(*fmutex);
    int remove_success = remove(mntPath);
    // We don't fail if the remove() failed, because that's just removing the file in the local file cache, which may or may not be there.

    if (AZS_PRINT)
    {
        fprintf(stdout, "remove_success = %d, errno = %d\n", remove_success, errno);
    }

    errno = 0;
    azure_blob_client_wrapper->delete_blob(str_options.containerName, pathString.substr(1));
    if (errno != 0)
    {
        if (AZS_PRINT)
        {
            fprintf(stdout, "Storage error occurred, errno =  = %d\n", errno);
        }        

        // If we successfully removed the file locally and the blob does not exist, we should still return success - this accounts for the case where the file hasn't yet been uploaded.
        if (!((remove_success == 0) && (errno = 404)))
        {
            return 0 - map_errno(errno);
        }
    }
    return 0;
}

int azs_truncate(const char * path, off_t off)
{
    if (AZS_PRINT)
    {
        fprintf(stdout, "azs_truncate called.  Path = %s, offset = %ld\n", path, off);
    }

    if (off != 0)
    {
        errno = 1; // TODO: set errno and return as appropriate.
        return -errno;
    }

    auto fmutex = file_lock_map::get_instance()->get_mutex(path);
    std::lock_guard<std::mutex> lock(*fmutex);

    std::string pathString(path);
    const char * mntPath;
    std::string mntPathString = prepend_mnt_path_string(pathString);
    mntPath = mntPathString.c_str();

    struct stat buf;
    int statret = stat(mntPath, &buf);
    if (statret == 0)
    {
        // The file exists in the local cache.  So, we call truncate() on the file in the cache, then upload a zero-length blob to the service, overriding any data.
        int truncret = truncate(mntPath, 0);
        if (truncret == 0)
        {
            // We want to upload a zero-length blob.
            std::istringstream emptyDataStream("");

            std::vector<std::pair<std::string, std::string>> metadata;
            errno = 0;
            azure_blob_client_wrapper->upload_block_blob_from_stream(str_options.containerName, pathString.substr(1), emptyDataStream, metadata);
            if (errno != 0)
            {
                return 0 - map_errno(errno); // TODO: Investigate what might happen in this case - the blob has been truncated locally, but not on the service.
            }
            else
            {
                return 0;
            }

        }
        else
        {
            return -errno;
        }
    }
    else
    {
        // The blob/file does not exist locally.  We need to see if it exists on the service (if it doesn't we return ENOENT.)
        if (azure_blob_client_wrapper->blob_exists(str_options.containerName, pathString.substr(1))) // TODO: Once we have support for access conditions, we could remove this call, and replace with a put_block_list with if-match-*
        {
            int fd = open(mntPath, O_CREAT|O_WRONLY|O_TRUNC);
            if (fd != 0)
            {
                return -errno;
            }
            close(fd);

            // We want to upload a zero-length blob.
            std::istringstream emptyDataStream("");

            std::vector<std::pair<std::string, std::string>> metadata;
            errno = 0;
            azure_blob_client_wrapper->upload_block_blob_from_stream(str_options.containerName, pathString.substr(1), emptyDataStream, metadata);
            if (errno != 0)
            {
                return 0 - map_errno(errno); // TODO: Investigate what might happen in this case - the blob has been truncated locally, but not on the service.
            }
            else
            {
                return 0;
            }
        }
        else
        {
            return -ENOENT;
        }
    }
    return 0;
}
