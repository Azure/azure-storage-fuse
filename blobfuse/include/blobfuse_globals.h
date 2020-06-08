#pragma once

#include <string>
#include <map>
#include <syslog.h>


// Global struct storing the Storage connection information and the tmpPath.
struct configParams
{
    std::string accountName;
    std::string authType;
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
    //this is set by the --allow-other flag,
    // 0770 if not set, 0777 if the flag is set
    int defaultPermission;
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
    const char *version; // print blobfuse version
    const char *help; // print blobfuse usage
};


// FUSE gives you one 64-bit pointer to use for communication between API's.
// An instance of this struct is pointed to by that pointer.
struct fhwrapper
{
    int fh; // The handle to the file in the file cache to use for read/write operations.
    bool upload; // True if the blob should be uploaded when the file is closed.  (False when the file was opened in read-only mode.)
    fhwrapper(int fh, bool upload) : fh(fh), upload(upload)
    {

    }
};