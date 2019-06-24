#include "blobfuse.h"
#include <string>

namespace {
    std::string trim(const std::string& str) {
        const size_t start = str.find_first_not_of(' ');
        if (std::string::npos == start) {
            return std::string();
        }
        const size_t end = str.find_last_not_of(' ');
        return str.substr(start, end - start + 1);
    }
}

// FUSE contains a specific type of command-line option parsing; here we are just following the pattern.
struct options
{
    const char *tmp_path; // Path to the temp / file cache directory
    const char *config_file; // Connection to Azure Storage information (account name, account key, etc)
    const char *use_https; // True if https should be used (defaults to false)
    const char *file_cache_timeout_in_seconds; // Timeout for the file cache (defaults to 120 seconds)
    const char *container_name; //container to mount. Used only if config_file is not provided
    const char *log_level; // Sets the level at which the process should log to syslog.
    const char *use_attr_cache; // True if the cache for blob attributes should be used.
    const char *version; // print blobfuse version
    const char *help; // print blobfuse usage
};

struct options options;
struct str_options str_options;
int file_cache_timeout_in_seconds;
int default_permission;

#define OPTION(t, p) { t, offsetof(struct options, p), 1 }
const struct fuse_opt option_spec[] =
{
    OPTION("--tmp-path=%s", tmp_path),
    OPTION("--config-file=%s", config_file),
    OPTION("--use-https=%s", use_https),
    OPTION("--file-cache-timeout-in-seconds=%s", file_cache_timeout_in_seconds),
    OPTION("--container-name=%s", container_name),
    OPTION("--log-level=%s", log_level),
    OPTION("--use-attr-cache=%s", use_attr_cache),
    OPTION("--version", version),
    OPTION("-v", version),
    OPTION("--help", help),
    OPTION("-h", help),
    FUSE_OPT_END
};

std::shared_ptr<sync_blob_client> azure_blob_client_wrapper;
class gc_cache gc_cache;

// Currently, the cpp lite lib puts the HTTP status code in errno.
// This mapping tries to convert the HTTP status code to a standard Linux errno.
// TODO: Ensure that we map any potential HTTP status codes we might receive.
std::map<int, int> error_mapping = {{404, ENOENT}, {403, EACCES}, {1600, ENOENT}};

const std::string former_directory_signifier = ".directory";

static struct fuse_operations azs_blob_operations;

const std::string log_ident = "blobfuse";

inline bool is_lowercase_string(const std::string &s)
{
    return (s.size() == static_cast<size_t>(std::count_if(s.begin(), s.end(),[](unsigned char c)
    {
        return std::islower(c);
    })));
}

// Read Storage connection information from the environment variables
int read_config_env()
{
    char* env_account = getenv("AZURE_STORAGE_ACCOUNT");
    char* env_account_key = getenv("AZURE_STORAGE_ACCESS_KEY");
    char* env_sas_token = getenv("AZURE_STORAGE_SAS_TOKEN");

    if(env_account)
    {
        str_options.accountName = env_account;
    }
    else
    {
        syslog(LOG_CRIT, "Unable to start blobfuse.  No config file was specified and AZURE_STORAGE_ACCESS_KEY environment variable is empty.");
        fprintf(stderr, "No config file was specified and AZURE_STORAGE_ACCOUNT environment variable is empty.\n");
        return -1;
    }

    if(env_account_key)
    {
        str_options.accountKey = env_account_key;
    }
    
    if(env_sas_token)
    {
        str_options.sasToken = env_sas_token;
    }

    if((!env_account_key && !env_sas_token) ||
       (env_account_key && env_sas_token)) 
    {
        syslog(LOG_CRIT, "Unable to start blobfuse.  If no config file is specified, exactly one of the environment variables AZURE_STORAGE_ACCESS_KEY or AZURE_STORAGE_SAS_TOKEN must be set.");
        fprintf(stderr, "Unable to start blobfuse.  If no config file is specified, exactly one of the environment variables AZURE_STORAGE_ACCESS_KEY or AZURE_STORAGE_SAS_TOKEN must be set.\n");
    }

    return 0;
}

// Read Storage connection information from the config file
int read_config(const std::string configFile)
{
    std::ifstream file(configFile);
    if(!file)
    {
        syslog(LOG_CRIT, "Unable to start blobfuse.  No config file found at %s.", configFile.c_str());
        fprintf(stderr, "No config file found at %s.\n", configFile.c_str());
        return -1;
    }

    std::string line;
    std::istringstream data;

    while(std::getline(file, line))
    {

        data.str(line.substr(line.find(" ")+1));
        const std::string value(trim(data.str()));

        if(line.find("accountName") != std::string::npos)
        {
            std::string accountNameStr(value);
            str_options.accountName = accountNameStr;
        }
        else if(line.find("accountKey") != std::string::npos)
        {
            std::string accountKeyStr(value);
            str_options.accountKey = accountKeyStr;
        }
        else if(line.find("sasToken") != std::string::npos)
        {
	    std::string sasTokenStr(value);
	    str_options.sasToken = sasTokenStr;
        }
        else if(line.find("containerName") != std::string::npos)
        {
            std::string containerNameStr(value);
            str_options.containerName = containerNameStr;
        }
        else if(line.find("blobEndpoint") != std::string::npos)
        {
            std::string blobEndpointStr(value);
            str_options.blobEndpoint = blobEndpointStr;
        }

        data.clear();
    }

    if(str_options.accountName.empty())
    {
        syslog (LOG_CRIT, "Unable to start blobfuse. Account name is missing in the config file.");
        fprintf(stderr, "Account name is missing in the config file.\n");
        return -1;
    }
    else if((str_options.accountKey.empty() && str_options.sasToken.empty()) || 
	    (!str_options.accountKey.empty() && !str_options.sasToken.empty()))
    {
        syslog (LOG_CRIT, "Unable to start blobfuse. Exactly one of Account Key and SAS token must be specified in the config file, and the other line should be deleted.");
        fprintf(stderr, "Unable to start blobfuse. Exactly one of Account Key and SAS token must be specified in the config file, and the other line should be deleted.\n");
        return -1;
    }
    else if(str_options.containerName.empty())
    {
        syslog (LOG_CRIT, "Unable to start blobfuse. Container name is missing in the config file.");
        fprintf(stderr, "Container name is missing in the config file.\n");
        return -1;
    }
    else
    {
        return 0;
    }
}


void *azs_init(struct fuse_conn_info * conn)
{
    if (str_options.use_attr_cache)
    {
        azure_blob_client_wrapper = std::make_shared<blob_client_attr_cache_wrapper>(blob_client_attr_cache_wrapper::blob_client_attr_cache_wrapper_init(str_options.accountName, str_options.accountKey, str_options.sasToken, 20/*concurrency*/, str_options.use_https,
                                                                                                                    str_options.blobEndpoint));
    }
    else
    {
        azure_blob_client_wrapper = std::make_shared<blob_client_wrapper>(blob_client_wrapper::blob_client_wrapper_init(str_options.accountName, str_options.accountKey, str_options.sasToken, 20/*concurrency*/, str_options.use_https,
                                                                                                                    str_options.blobEndpoint));
    }

    if(errno != 0)
    {
        syslog(LOG_CRIT, "azs_init - Unable to start blobfuse.  Creating blob client failed: errno = %d.\n", errno);

        // TODO: Improve this error case
        return NULL;
    }
    /*
    cfg->attr_timeout = 360;
    cfg->kernel_cache = 1;
    cfg->entry_timeout = 120;
    cfg->negative_timeout = 120;
    */
    conn->max_write = 4194304;
    //conn->max_read = 4194304;
    conn->max_readahead = 4194304;
    conn->max_background = 128;
    //  conn->want |= FUSE_CAP_WRITEBACK_CACHE | FUSE_CAP_EXPORT_SUPPORT; // TODO: Investigate putting this back in when we downgrade to fuse 2.9

    gc_cache.run();

    return NULL;
}

// TODO: print FUSE usage as well
void print_usage()
{
    fprintf(stdout, "Usage: blobfuse <mount-folder> --tmp-path=</path/to/fusecache> [--config-file=</path/to/config.cfg> | --container-name=<containername>]");
    fprintf(stdout, "    [--use-https=true] [--file-cache-timeout-in-seconds=120] [--log-level=LOG_OFF|LOG_CRIT|LOG_ERR|LOG_WARNING|LOG_INFO|LOG_DEBUG] [--use-attr-cache=true]\n\n");
    fprintf(stdout, "In addition to setting --tmp-path parameter, you must also do one of the following:\n");
    fprintf(stdout, "1. Specify a config file (using --config-file]=) with account name, account key, and container name, OR\n");
    fprintf(stdout, "2. Set the environment variables AZURE_STORAGE_ACCOUNT and AZURE_STORAGE_ACCESS_KEY, and specify the container name with --container-name=\n\n");
    fprintf(stdout, "See https://github.com/Azure/azure-storage-fuse for detailed installation and configuration instructions.\n");
}

void print_version()
{
    fprintf(stdout, "blobfuse 1.0.2\n");
}

int set_log_mask(const char * min_log_level_char)
{
    if (!min_log_level_char)
    {
        setlogmask(LOG_UPTO(LOG_WARNING));
        return 0;
    }
    std::string min_log_level(min_log_level_char);
    if (min_log_level.empty())
    {
        setlogmask(LOG_UPTO(LOG_WARNING));
        return 0;
    }
    // Options for logging: LOG_OFF, LOG_CRIT, LOG_ERR, LOG_WARNING, LOG_INFO, LOG_DEBUG
    if (min_log_level == "LOG_OFF")
    {
        setlogmask(LOG_UPTO(LOG_EMERG)); // We don't use 'LOG_EMERG', so this won't log anything.
        return 0;
    }
    if (min_log_level == "LOG_CRIT")
    {
        setlogmask(LOG_UPTO(LOG_CRIT));
        return 0;
    }
    if (min_log_level == "LOG_ERR")
    {
        setlogmask(LOG_UPTO(LOG_ERR));
        return 0;
    }
    if (min_log_level == "LOG_WARNING")
    {
        setlogmask(LOG_UPTO(LOG_WARNING));
        return 0;
    }
    if (min_log_level == "LOG_INFO")
    {
        setlogmask(LOG_UPTO(LOG_INFO));
        return 0;
    }
    if (min_log_level == "LOG_DEBUG")
    {
        setlogmask(LOG_UPTO(LOG_DEBUG));
        return 0;
    }

    syslog(LOG_CRIT, "Unable to start blobfuse. Error: Invalid log level \"%s\"", min_log_level.c_str());
    fprintf(stdout, "Error: Invalid log level \"%s\".  Permitted values are LOG_OFF, LOG_CRIT, LOG_ERR, LOG_WARNING, LOG_INFO, LOG_DEBUG.\n", min_log_level.c_str());
    fprintf(stdout, "If not specified, logging will default to LOG_WARNING.\n\n");
    return 1;
}

void set_up_callbacks()
{
    openlog(log_ident.c_str(), LOG_NDELAY | LOG_PID, 0);

    // Here, we set up all the callbacks that FUSE requires.
    azs_blob_operations.init = azs_init;
    azs_blob_operations.getattr = azs_getattr;
    azs_blob_operations.statfs = azs_statfs;
    azs_blob_operations.access = azs_access;
    azs_blob_operations.readlink = azs_readlink;
    azs_blob_operations.readdir = azs_readdir;
    azs_blob_operations.open = azs_open;
    azs_blob_operations.read = azs_read;
    azs_blob_operations.release = azs_release;
    azs_blob_operations.fsync = azs_fsync;
    azs_blob_operations.create = azs_create;
    azs_blob_operations.write = azs_write;
    azs_blob_operations.mkdir = azs_mkdir;
    azs_blob_operations.unlink = azs_unlink;
    azs_blob_operations.rmdir = azs_rmdir;
    azs_blob_operations.chown = azs_chown;
    azs_blob_operations.chmod = azs_chmod;
    //#ifdef HAVE_UTIMENSAT
    azs_blob_operations.utimens = azs_utimens;
    //#endif
    azs_blob_operations.destroy = azs_destroy;
    azs_blob_operations.truncate = azs_truncate;
    azs_blob_operations.rename = azs_rename;
    azs_blob_operations.setxattr = azs_setxattr;
    azs_blob_operations.getxattr = azs_getxattr;
    azs_blob_operations.listxattr = azs_listxattr;
    azs_blob_operations.removexattr = azs_removexattr;
    azs_blob_operations.flush = azs_flush;
}

int read_and_set_arguments(int argc, char *argv[], struct fuse_args *args)
{
    // FUSE has a standard method of argument parsing, here we just follow the pattern.
    *args = FUSE_ARGS_INIT(argc, argv);

    // Check for existence of allow_other flag and change the default permissions based on that
    default_permission = 0770;
    std::vector<std::string> string_args(argv, argv+argc);
    for (size_t i = 1; i < string_args.size(); ++i) {
      if (string_args[i].find("allow_other") != std::string::npos) {
          default_permission = 0777; 
      }
    }

    int ret = 0;
    try
    {

        if (fuse_opt_parse(args, &options, option_spec, NULL) == -1)
        {
            return 1;
        }

        if(options.version)
        {
            print_version();
            exit(0);
        }

        if(options.help)
        {
            print_usage();
            exit(0);
        }

        if(!options.config_file)
        {
            if(!options.container_name)
            {
                syslog(LOG_CRIT, "Unable to start blobfuse, no config file provided and --container-name is not set.");
                fprintf(stderr, "Error: No config file provided and --container-name is not set.\n");
                print_usage();
                return 1;
            }

            std::string container(options.container_name);
            str_options.containerName = container;
            ret = read_config_env();
        }
        else
        {
            ret = read_config(options.config_file);
        }

        if (ret != 0)
        {
            return ret;
        }
    }
    catch(std::exception &)
    {
        print_usage();
        return 1;
    }

    int res = set_log_mask(options.log_level);
    if (res != 0)
    {
        print_usage();
        return 1;
    }

    // remove last trailing slash in tmo_path
    if(!options.tmp_path)
    {
        fprintf(stderr, "Error: --tmp-path is not set.\n");
        print_usage();
        return 1;
    }
    
    std::string tmpPathStr(options.tmp_path);
    if (!tmpPathStr.empty() && tmpPathStr[tmpPathStr.size() - 1] == '/')
    {
        tmpPathStr.erase(tmpPathStr.size() - 1);
    }
    
    str_options.tmpPath = tmpPathStr;
    str_options.use_https = true;
    if (options.use_https != NULL)
    {
        std::string https(options.use_https);
        if (https == "false")
        {
            str_options.use_https = false;
        }
    }

    str_options.use_attr_cache = false;
    if (options.use_attr_cache != NULL)
    {
        std::string attr_cache(options.use_attr_cache);
        if (attr_cache == "true")
        {
            str_options.use_attr_cache = true;
        }
    }

    if (options.file_cache_timeout_in_seconds != NULL)
    {
        std::string timeout(options.file_cache_timeout_in_seconds);
        file_cache_timeout_in_seconds = stoi(timeout);
    }
    else
    {
        file_cache_timeout_in_seconds = 120;
    }
    return 0;
}

int configure_tls()
{
    // For proper locking, instructing gcrypt to use pthreads 
    gcry_control(GCRYCTL_SET_THREAD_CBS, &gcry_threads_pthread);
    if(GNUTLS_E_SUCCESS != gnutls_global_init())
    {
        syslog (LOG_CRIT, "Unable to start blobfuse. GnuTLS initialization failed: errno = %d.\n", errno);
        fprintf(stderr, "GnuTLS initialization failed: errno = %d.\n", errno);
        return 1; 
    }
    return 0;
}

int validate_storage_connection()
{
    // The current implementation of blob_client_wrapper calls curl_global_init() in the constructor, and curl_global_cleanup in the destructor.
    // Unfortunately, curl_global_init() has to be called in the same process as any HTTPS calls that are made, otherwise NSS is not configured properly.
    // When running in daemon mode, the current process forks() and exits, while the child process lives on as a daemon.
    // So, here we create and destroy a temp blob client in order to test the connection info, and we create the real one in azs_init, which is called after the fork().
    {
        const int defaultMaxConcurrency = 20;
        blob_client_wrapper temp_azure_blob_client_wrapper = blob_client_wrapper::blob_client_wrapper_init(str_options.accountName, str_options.accountKey, str_options.sasToken, defaultMaxConcurrency, str_options.use_https, 
													   str_options.blobEndpoint);
        if(errno != 0)
        {
            syslog(LOG_CRIT, "Unable to start blobfuse.  Creating local blob client failed: errno = %d.\n", errno);
            fprintf(stderr, "Creating blob client failed: errno = %d.\n", errno);
            return 1;
        }

        // Check if the account name/key and container is correct by attempting to list a blob.
        // This will succeed even if there are zero blobs.
        list_blobs_hierarchical_response response = temp_azure_blob_client_wrapper.list_blobs_hierarchical(str_options.containerName, "/", std::string(), std::string(), 1);
        if(errno != 0)
        {
            syslog(LOG_CRIT, "Unable to start blobfuse.  Failed to connect to the storage container. There might be something wrong about the storage config, please double check the storage account name, account key and container name. errno = %d\n", errno);
            fprintf(stderr, "Failed to connect to the storage container. There might be something wrong about the storage config, please double check the storage account name, account key and container name. errno = %d\n", errno);
            return 1;
        }
    }
    return 0;
}

void configure_fuse(struct fuse_args *args)
{
    fuse_opt_add_arg(args, "-omax_read=131072");
    fuse_opt_add_arg(args, "-omax_write=131072");

    if (options.file_cache_timeout_in_seconds != NULL)
    {
        std::string timeout(options.file_cache_timeout_in_seconds);
        file_cache_timeout_in_seconds = stoi(timeout);
    }
    else
    {
        file_cache_timeout_in_seconds = 120;
    }

    // FUSE contains a feature where it automatically implements 'soft' delete if one process has a file open when another calls unlink().
    // This feature causes us a bunch of problems, so we use "-ohard_remove" to disable it, and track the needed 'soft delete' functionality on our own.
    fuse_opt_add_arg(args, "-ohard_remove");
    fuse_opt_add_arg(args, "-obig_writes");
    fuse_opt_add_arg(args, "-ofsname=blobfuse");
    fuse_opt_add_arg(args, "-okernel_cache");
    umask(0);
}

int initialize_blobfuse()
{
    if(0 != ensure_files_directory_exists_in_cache(prepend_mnt_path_string("/placeholder")))
    {
        syslog(LOG_CRIT, "Unable to start blobfuse.  Failed to create directory on cache directory: %s, errno = %d.\n", prepend_mnt_path_string("/placeholder").c_str(),  errno);
        fprintf(stderr, "Failed to create directory on cache directory: %s, errno = %d.\n", prepend_mnt_path_string("/placeholder").c_str(),  errno);
        return 1;
    }
    return 0;
}
