#include "blobfuse.h"
#include <sys/file.h>

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


int azs_open(const char *path, struct fuse_file_info *fi)
{
    if (AZS_PRINT)
    {
        fprintf(stdout, "azs_open called with path = %s, fi->flags = %X. \n", path, fi->flags);
    }
    std::string pathString(path);
    const char * mntPath;
    std::string mntPathString = prepend_mnt_path_string(pathString);
    mntPath = mntPathString.c_str();
    /*
    if (fi->flags & O_WRONLY == O_WRONLY)
    {
        int res;
        ensure_files_directory_exists(mntPathString);
        res = open(mntPath, fi->flags);
        if (AZS_PRINT)
        {
            printf("res = %d, errno = %d, ENOENT = %d\n", res, errno, ENOENT);
        }

        if (res == -1)
            return -errno;

        struct fhwrapper *fhwrap = new fhwrapper(res, true);
        fi->fh = (long unsigned int)fhwrap;

    }
    else if (fi->flags & O_RDWR == O_RDWR)
    {
        return -1; // Read/Write not currently supported.
    }
    else
    {
    */
    auto fmutex = file_lock_map::get_instance()->get_mutex(path);
    std::lock_guard<std::mutex> lock(*fmutex);
    struct stat buf;
    int statret = stat(mntPath, &buf);
    if ((statret != 0) || ((time(NULL) - buf.st_atime) > 120))
    {
        remove(mntPath);

        ensure_files_directory_exists(mntPathString);
        std::ofstream filestream(mntPathString, std::ofstream::binary | std::ofstream::out);
        int fd = open(mntPath, O_WRONLY);
        if (fd == -1)
        {
            return -errno;
        }
        flock(fd, LOCK_EX);
        errno = 0;
        azure_blob_client_wrapper->download_blob_to_stream(str_options.containerName, pathString.substr(1), 0ULL, 1000000000000ULL, filestream);
        flock(fd, LOCK_UN);
        close(fd);
        if (errno != 0)
        {
            return 0 - map_errno(errno);
        }
    }

    errno = 0;
    int res;

    res = open(mntPath, fi->flags);
    if (AZS_PRINT)
    {
        printf("Accessing %s gives res = %d, errno = %d, ENOENT = %d\n", mntPath, res, errno, ENOENT);
    }

    if (res == -1)
    {
        return -errno;
    }
    if((fi->flags&O_RDONLY) == O_RDONLY)
    {
       flock(res, LOCK_SH);
    }
    else
    {
       flock(res, LOCK_EX);
    }

    fchmod(res, 0777);
    struct fhwrapper *fhwrap = new fhwrapper(res, (((fi->flags & O_WRONLY) == O_WRONLY) || ((fi->flags & O_RDWR) == O_RDWR)));
    fi->fh = (long unsigned int)fhwrap;
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


int azs_create(const char *path, mode_t mode, struct fuse_file_info *fi)
{
    if (AZS_PRINT)
    {
        fprintf(stdout, "azs_create called with path = %s, mode = %d, fi->flags = %x\n", path, mode, fi->flags);
    }
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
            mkdir(mntPathCopy, 0777);
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
    if (AZS_PRINT)
    {
        fprintf(stdout, "azs_flush called with path = %s, fi->flags = %d\n", path, fi->flags);
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
        close(dup(((struct fhwrapper *)fi->fh)->fh));
        if (((struct fhwrapper *)fi->fh)->upload)
        {
            // TODO: This will currently upload the full file on every flush() call.  We may want to keep track of whether
            // or not flush() has been called already, and not re-upload the file each time.
            std::vector<std::pair<std::string, std::string>> metadata;
            errno = 0;
            azure_blob_client_wrapper->upload_file_to_blob(mntPath, str_options.containerName, pathString.substr(1), metadata, 8);
            if (errno != 0)
            {
                return 0 - map_errno(errno);
            }
        }
    }

    return 0;
}


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
    remove(mntPath);

    errno = 0;
    azure_blob_client_wrapper->delete_blob(str_options.containerName, pathString.substr(1));
    if (errno != 0)
    {
        return 0 - map_errno(errno);
    }
    return 0;
}
