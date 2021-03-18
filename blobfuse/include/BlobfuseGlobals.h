#pragma once

#include <string>
#include <map>
#include <syslog.h>
#include <algorithm>
#include <BlobfuseConstants.h>

#include <OAuthToken.h>
#include <OAuthTokenCredentialManager.h>
#include <AttrCacheWrapper.h>

#include <blob/blob_client.h>
#include <adls_client.h>
#include <BlobfuseGcCache.h>

using namespace azure::storage_lite;
using namespace azure::storage_adls;

#define METADATA std::vector<std::pair<std::string, std::string>>
#define LISTALL_RES std::vector<std::pair<std::vector<list_segmented_item>, bool>>

bool is_directory_empty(const char *tmpDir);
extern struct configParams config_options;
extern struct globalTimes_st globalTimes;

struct globalTimes_st
{
    time_t lastModifiedTime;
    time_t lastAccessTime;
    time_t lastChangeTime;
};

// Global struct storing the Storage connection information and the tmpPath.
struct configParams
{
    std::string accountName;
    //std::string authType;
    AUTH_TYPE authType;
    std::string blobEndpoint;
    std::string accountKey;
    std::string sasToken;
    std::string identityClientId;
    std::string spnClientId;
    std::string spnClientSecret;
    std::string spnTenantId;
    std::string aadEndpoint;
    std::string objectId;
    std::string resourceId;
    std::string msiEndpoint;
    std::string msiSecret;
    std::string containerName;
    std::string tmpPath;
    std::string logLevel;
    int fileCacheTimeoutInSeconds;
    bool useHttps;
    bool useAttrCache;
    bool useADLS;
    bool noSymlinks;
    bool cacheOnList;
    //this is set by the --allow-other flag,
    // 0770 if not set, 0777 if the flag is set
    int defaultPermission;
    int concurrency;
    unsigned long long cacheSize;
    volatile int  cancel_list_on_mount_secs;
    bool emptyDirCheck;
    bool uploadIfModified;
    std::string mntPath;
    int high_disk_threshold;
    int low_disk_threshold;
    unsigned long long cachePollTimeout;
    unsigned long long maxEviction;
};

// FUSE contains a specific type of command-line option parsing; here we are just following the pattern.
struct cmdlineOptions
{
    const char *tmp_path; // Path to the temp / file cache directory
    const char *config_file; // Connection to Azure Storage information (account name, account key, etc)
    const char *useHttps; // True if https should be used (defaults to false)
    const char *file_cache_timeout_in_seconds; // Timeout for the file cache (defaults to 120 seconds)
    const char *container_name; //container to mount. Used only if config_file is not provided
    const char *log_level; // Sets the level at which the process should log to syslog.
    const char *useAttrCache; // True if the cache for blob attributes should be used.
    const char *use_adls; // True if the dfs/DataLake endpoint should be used when necessary
    const char *no_symlinks; // Whether to enable symlink support on adls account or not
    const char *cache_on_list; // Cache blob property when list operation is done
    const char *version; // print blobfuse version
    const char *help; // print blobfuse usage
    const char *concurrency; // Max Concurrency factor for blob client wrapper (default 40)
    const char *cache_size_mb; // MAX Size of cache in MBs
    const char *cancel_list_on_mount_seconds; // Block the list api call on mount for n seconds
    const char *empty_dir_check;
    const char *upload_if_modified;
    const char *encode_full_file_name; // Encode the '%' symbol in file name
    const char *high_disk_threshold; // High disk threshold percentage
    const char *low_disk_threshold; // Low disk threshold percentage
    const char *cache_poll_timeout_msec; // Timeout for cache eviction thread in case queue is empty
    const char *max_eviction; // Maximum number of files to be deleted from cache to converse cpu
    const char *set_content_type; // Whether to set content type while upload blob
};


// FUSE gives you one 64-bit pointer to use for communication between API's.
// An instance of this struct is pointed to by that pointer.
struct fhwrapper
{
    int fh; // The handle to the file in the file cache to use for read/write operations.
    bool write_mode; // False when the file was opened in read-only mode
    bool upload_on_close; // False if file is not written or created. Upload only if the flag is true
    fhwrapper(int fh, bool mode) : fh(fh), write_mode(mode), upload_on_close(false)
    {

    }
};

std::string to_lower(std::string original);
inline bool is_lowercase_string(const std::string &s);
AUTH_TYPE get_auth_type(std::string authStr = "");
