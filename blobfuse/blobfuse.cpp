#include <blobfuse.h>
#include <boost/filesystem.hpp>
#include <string>
#include <signal.h>
#include <mntent.h>
#include <sys/types.h>
#include <dirent.h>
#include <pwd.h>

#include <include/StorageBfsClientBase.h>
#include <include/BlockBlobBfsClient.h>
#include <include/DataLakeBfsClient.h>
#include <include/AttrCacheBfsClient.h>

const std::string log_ident = "blobfuse";
struct cmdlineOptions cmd_options;
struct configParams config_options;
struct globalTimes_st globalTimes;
std::shared_ptr<StorageBfsClientBase> storage_client;

extern bool gZonalDNS;
int stdErrFD = -1;
bool is_directory_mounted(const char* mntDir);
time_t gMountTime;
extern bool gSetContentType;

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

#define OPTION(t, p) { t, offsetof(struct cmdlineOptions, p), 1 }
const struct fuse_opt option_spec[] =
{
    OPTION("--tmp-path=%s", tmp_path),
    OPTION("--config-file=%s", config_file),
    OPTION("--use-https=%s", useHttps),
    OPTION("--file-cache-timeout-in-seconds=%s", file_cache_timeout_in_seconds),
    OPTION("--container-name=%s", container_name),
    OPTION("--log-level=%s", log_level),
    OPTION("--use-attr-cache=%s", useAttrCache),
    OPTION("--use-adls=%s", use_adls),
    OPTION("--no-symlinks=%s", no_symlinks), 
    OPTION("--cache-on-list=%s", cache_on_list),
    OPTION("--max-concurrency=%s", concurrency),
    OPTION("--cache-size-mb=%s", cache_size_mb),
    OPTION("--empty-dir-check=%s", empty_dir_check),
    OPTION("--upload-modified-only=%s", upload_if_modified),
    OPTION("--cancel-list-on-mount-seconds=%s", cancel_list_on_mount_seconds),
    OPTION("--high-disk-threshold=%s", high_disk_threshold),
    OPTION("--low-disk-threshold=%s", low_disk_threshold),
    OPTION("--cache-poll-timeout-msec=%s", cache_poll_timeout_msec),
    OPTION("--max-eviction=%s", max_eviction),
    OPTION("--set-content-type=%s", set_content_type),
    OPTION("--ca-cert-file=%s", caCertFile),
    OPTION("--https-proxy=%s", httpsProxy),
    OPTION("--http-proxy=%s", httpProxy),
    OPTION("--max-retry=%s", max_retry),
    OPTION("--max-retry-interval-in-seconds=%s", max_timeout),
    OPTION("--retry-delay-factor=%s", retry_delay),
    OPTION("--basic-remount-check=%s", basic_remount_check),
    OPTION("--pre-mount-validate=%s", pre_mount_validate),
    
    OPTION("--read-stream=%s", stream_read),
    OPTION("--stream-buffer-size-mb=%s", stream_buffer),

    OPTION("--version", version),
    OPTION("-v", version),
    OPTION("--help", help),
    OPTION("-h", help),
    FUSE_OPT_END
};

// Read Storage connection information from the environment variables
int read_config_env()
{
    char* env_account = getenv("AZURE_STORAGE_ACCOUNT");
    char* env_account_key = getenv("AZURE_STORAGE_ACCESS_KEY");
    char* env_sas_token = getenv("AZURE_STORAGE_SAS_TOKEN");
    char* env_blob_endpoint = getenv("AZURE_STORAGE_BLOB_ENDPOINT");
    char* env_identity_client_id = getenv("AZURE_STORAGE_IDENTITY_CLIENT_ID");
    char* env_identity_object_id = getenv("AZURE_STORAGE_IDENTITY_OBJECT_ID");
    char* env_identity_resource_id = getenv("AZURE_STORAGE_IDENTITY_RESOURCE_ID");
    char* env_managed_identity_endpoint = getenv("MSI_ENDPOINT");
    char* env_managed_identity_secret = getenv("MSI_SECRET");
    char* env_spn_client_id = getenv("AZURE_STORAGE_SPN_CLIENT_ID");
    char* env_spn_tenant_id = getenv("AZURE_STORAGE_SPN_TENANT_ID");
    char* env_spn_client_secret = getenv("AZURE_STORAGE_SPN_CLIENT_SECRET");
    char* env_auth_type = getenv("AZURE_STORAGE_AUTH_TYPE");
    char* env_aad_endpoint = getenv("AZURE_STORAGE_AAD_ENDPOINT");
    char* env_https_proxy = getenv("https_proxy");
    char* env_http_proxy = getenv("http_proxy");
    
    if(env_account)
    {
        config_options.accountName = env_account;

        if(env_account_key)
        {
            config_options.accountKey = env_account_key;
        }

        if(env_sas_token)
        {
            config_options.sasToken = env_sas_token;
        }

        if(env_identity_client_id)
        {
            config_options.identityClientId = env_identity_client_id;
        }

        if (env_spn_client_secret)
        {
            config_options.spnClientSecret = env_spn_client_secret;
        }

        if (env_spn_tenant_id)
        {
            config_options.spnTenantId = env_spn_tenant_id;
        }

        if (env_spn_client_id)
        {
            config_options.spnClientId = env_spn_client_id;
        }

        if(env_identity_object_id)
        {
            config_options.objectId = env_identity_object_id;
        }

        if(env_identity_resource_id)
        {
            config_options.resourceId = env_identity_resource_id;
        }

        if(env_managed_identity_endpoint)
        {
            config_options.msiEndpoint = env_managed_identity_endpoint;
        }

        if(env_managed_identity_secret)
        {
            config_options.msiSecret = env_managed_identity_secret;
        }

        if(env_auth_type)
        {
            config_options.authType = get_auth_type(env_auth_type);
        } else {
            config_options.authType = get_auth_type();
        }

        if(env_aad_endpoint)
        {
            config_options.aadEndpoint = env_auth_type;
        }

        if(env_blob_endpoint) {
            // Optional to specify blob endpoint
            config_options.blobEndpoint = env_blob_endpoint;
        }

        if(env_https_proxy) {
            config_options.httpsProxy = env_https_proxy;
        }

        if(env_http_proxy) {
            config_options.httpProxy = env_http_proxy;
        }
    }
    else
    {
        syslog(LOG_CRIT, "Unable to start blobfuse.  No config file was specified and the AZURE_STORAGE_ACCOUNT"
                         "environment variable was empty");
        fprintf(stderr, "Unable to start blobfuse.  No config file was specified and the AZURE_STORAGE_ACCOUNT"
                        "environment variable was empty\n");
        return -1;
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
    bool set_auth_type = false;

    char* env_spn_client_secret = getenv("AZURE_STORAGE_SPN_CLIENT_SECRET");
    char* env_msi_secret = getenv("MSI_SECRET");

    if (env_spn_client_secret) {
        config_options.spnClientSecret = env_spn_client_secret;
    }

    if (env_msi_secret) {
        config_options.msiSecret = env_msi_secret;
    }

    while(std::getline(file, line))
    {
        // skip over comments
        if(line[0] == '#') {
            continue;
        }

        replace(line.begin(), line.end(), '\t', ' ');
        data.str(line.substr(line.find(" ")+1));
        const std::string value(trim(data.str()));
    
        if(line.find("accountName") != std::string::npos)
        {
            syslog(LOG_DEBUG, "Account name found");
            std::string accountNameStr(value);
            config_options.accountName = accountNameStr;
        }
        else if(line.find("accountKey") != std::string::npos)
        {
            syslog(LOG_DEBUG, "Account Key found");
            std::string accountKeyStr(value);
            config_options.accountKey = accountKeyStr;
        }
        else if(line.find("sasToken") != std::string::npos)
        {
            syslog(LOG_DEBUG, "Sas Token found");
            std::string sasTokenStr(value);
            config_options.sasToken = sasTokenStr;
        }
        else if(line.find("containerName") != std::string::npos)
        {
            syslog(LOG_DEBUG, "Container name found");
            std::string containerNameStr(value);
            config_options.containerName = containerNameStr;
        }
        else if(line.find("blobEndpoint") != std::string::npos)
        {
            std::string blobEndpointStr(value);
            config_options.blobEndpoint = blobEndpointStr;
        }
        else if(line.find("identityClientId") != std::string::npos)
        {
            std::string clientIdStr(value);
            config_options.identityClientId = clientIdStr;
        }
        else if(line.find("identityObjectId") != std::string::npos)
        {
            std::string objectIdStr(value);
            config_options.objectId = objectIdStr;
        }
        else if(line.find("identityResourceId") != std::string::npos)
        {
            std::string resourceIdStr(value);
            config_options.resourceId = resourceIdStr;
        }
        else if(line.find("caCertFile") != std::string::npos)
        {
            std::string caCertFileStr(value);
            config_options.caCertFile = caCertFileStr;            
            syslog(LOG_DEBUG, "caCertFile has the value %s", config_options.caCertFile.c_str());
        }
        else if(line.find("httpsProxy") != std::string::npos)
        {
            std::string httpsProxyStr(value);
            config_options.httpsProxy = httpsProxyStr;
            syslog(LOG_DEBUG, "Https Proxy server %s will be used to connect to storage account", config_options.httpsProxy.c_str());
        }
        else if(line.find("httpProxy") != std::string::npos)
        {
            std::string httpProxyStr(value);
            config_options.httpProxy = httpProxyStr;
            syslog(LOG_DEBUG, "Http Proxy server %s will be used to connect to storage account", config_options.httpsProxy.c_str());
        }
        else if(line.find("authType") != std::string::npos)
        {
            config_options.authType = get_auth_type(value);
            set_auth_type = true;
        }
        else if(line.find("msiEndpoint") != std::string::npos)
        {
            std::string msiEndpointStr(value);
            config_options.msiEndpoint = msiEndpointStr;
        }
        else if(line.find("servicePrincipalClientId") != std::string::npos)
        {
            std::string spClientIdStr(value);
            config_options.spnClientId = spClientIdStr;
        }
        else if(line.find("servicePrincipalTenantId") != std::string::npos)
        {
            std::string spTenantIdStr(value);
            config_options.spnTenantId = spTenantIdStr;
        }
        else if(line.find("servicePrincipalClientSecret") != std::string::npos)
        {
            std::string spClientSecretStr(value);
            config_options.spnClientSecret = spClientSecretStr;
        }
        else if(line.find("aadEndpoint") != std::string::npos)
        {
            std::cout << line.find("aadEndpoint");
            std::string altAADEndpointStr(value);
            config_options.aadEndpoint = altAADEndpointStr;
        }
        else if(line.find("logLevel") != std::string::npos)
        {
            std::string logLevel(value);
            config_options.logLevel = logLevel;
        } 
        else if(line.find("accountType") != std::string::npos)
        {
            std::string acctType(value);
            if (acctType == "adls")
                config_options.useADLS = true;
        } 
        else if(line.find("dnsType") != std::string::npos)
        {
            std::string val(value);
            if (val == "zonal")
                gZonalDNS = true;
        }  

        data.clear();
    }

    if(!set_auth_type)
    {
        config_options.authType = get_auth_type();
    }
    
    if(config_options.accountName.empty())
    {
        syslog (LOG_CRIT, "Unable to start blobfuse. Account name is missing in the config file.");
        fprintf(stderr, "Unable to start blobfuse. Account name is missing in the config file.\n");
        return -1;
    }
    else if(config_options.containerName.empty())
    {
        syslog (LOG_CRIT, "Unable to start blobfuse. Container name is missing in the config file.");
        fprintf(stderr, "Unable to start blobfuse. Container name is missing in the config file.\n");
        return -1;
    }
    else
    {
        return 0;
    }
}

void destroyBlobfuseOnAuthError()
{
    char errStr[] = "Unmounting blobfuse.\n";
    
    syslog(LOG_ERR, errStr, sizeof(errStr));
    ssize_t n = write(stdErrFD, errStr, sizeof(errStr));
    if (n == -1) {
        syslog(LOG_ERR, "Failed to report back failure.");
    }
    
    fuse_unmount(config_options.mntPath.c_str(), NULL);

    if (is_directory_mounted(config_options.mntPath.c_str())) 
    {
        char errStr[] = "Failed to unmount blobfuse. Manually unmount using fusermount command.\n";
        syslog(LOG_ERR, errStr, sizeof(errStr));
        ssize_t n = write(stdErrFD, errStr, sizeof(errStr));
        if (n == -1) {
            syslog(LOG_ERR, "%s", errStr);
        }
    } else {
        char errStr[] = "Unmounted blobfuse successfully.\n";
        syslog(LOG_ERR, errStr, sizeof(errStr));
        ssize_t n = write(stdErrFD, errStr, sizeof(errStr));
        if (n == -1) {
            syslog(LOG_ERR, "%s", errStr);
        }
    }
    close(stdErrFD);
    exit(1);
}

int configure_tls();
void *azs_init(struct fuse_conn_info * conn)
{
    syslog(LOG_DEBUG, "azs_init ran");

    gMountTime = time(NULL);
    /*
    cfg->attr_timeout = 360;
    cfg->kernel_cache = 1;
    cfg->entry_timeout = 120;
    cfg->negative_timeout = 120;
    */
   // even 4.18 does not like this so 5.4 is not enough so 
    if (kernel_version < blobfuse_constants::minKernelVersion) {
        conn->max_write = 4194304;
	// let fuselib pick 128KB
	//conn->max_read = 4194304;
    } else {
        conn->want |= FUSE_CAP_BIG_WRITES;
    }
    conn->max_readahead = 4194304;
    conn->max_background = 128;
    //  conn->want |= FUSE_CAP_WRITEBACK_CACHE | FUSE_CAP_EXPORT_SUPPORT; // TODO: Investigate putting this back in when we downgrade to fuse 2.9

    g_gc_cache = std::make_shared<gc_cache>(config_options.tmpPath, config_options.fileCacheTimeoutInSeconds);
    g_gc_cache->run();
    
    if (!config_options.preMountValidate && libcurl_version < blobfuse_constants::minCurlVersion) {
        // Curl was not intialized for lower version as that results into
        // failure post fork. So tls and cpplite were not initialized pre-fork for lower
        // kernel version. Do the init now before starting.
        syslog(LOG_CRIT, "** Post fork authentication for older libcurl version");

        configure_tls();
        if(storage_client->AuthenticateStorage())
        {
            syslog(LOG_DEBUG, "Successfully Authenticated!");   
        }
        else
        {
            char errStr[] = "Unable to start blobfuse due to a lack of credentials. Please check the readme for valid auth setups.\n";
            if (stdErrFD != -1) {
                ssize_t n = write(stdErrFD, errStr, sizeof(errStr));
                if (n == -1) {
                    syslog(LOG_ERR, "Failed to report back the status of auth.");
                }
            }
            syslog(LOG_ERR, "%s", errStr);
            std::thread t1(std::bind(&destroyBlobfuseOnAuthError));
            t1.detach();
            return NULL;
        }
    }
    if (stdErrFD != -1) {
        // Test code to print on console 
        /*char errStr[] = "Blobfuse Auth successful\n";
        ssize_t n = write(stdErrFD, errStr, sizeof(errStr));
        if (n == -1) {
            syslog(LOG_ERR, "Failed to report back the status of auth.");
        }*/
        close(stdErrFD);
    }

    if (config_options.authType == MSI_AUTH ||
        config_options.authType == SPN_AUTH)
    {
        std::shared_ptr<OAuthTokenCredentialManager> tokenManager;
        if (config_options.caCertFile.empty())
        {
            tokenManager = GetTokenManagerInstance(EmptyCallback);
        }
        else
        {
            tokenManager = GetTokenManagerInstance(EmptyCallback, config_options.caCertFile, config_options.httpsProxy);        }

        tokenManager->StartTokenMonitor();
    }

    return NULL;
}

// TODO: print FUSE usage as well
void print_usage()
{
    fprintf(stdout, "Usage: blobfuse <mount-folder> --tmp-path=</path/to/fusecache> [--config-file=</path/to/config.cfg> | --container-name=<containername>]");
    fprintf(stdout, "    [--use-https=true] [--file-cache-timeout-in-seconds=120] [--log-level=LOG_OFF|LOG_CRIT|LOG_ERR|LOG_WARNING|LOG_INFO|LOG_DEBUG] [--use-attr-cache=true]\n\n");
    fprintf(stdout, "In addition to setting --tmp-path parameter, you must also do one of the following:\n");
    fprintf(stdout, "1. Specify a config file (using --config-file]=) with account name (accountName), container name (containerName), and\n");
    fprintf(stdout,  "\ta. account key (accountKey),\n");
    fprintf(stdout,  "\tb. account SAS token (sasToken),\n");
    fprintf(stdout,  "\tc. valid service principal credentials (servicePrincipalClientId, servicePrincipalTenantId, and environment variable AZURE_STORAGE_SPN_CLIENT_SECRET) with access to the storage account or,\n");
    fprintf(stdout,  "\td. valid MSI credentials (one or none of identityClientId, identityObjectId, identityResourceId) with access to the storage account. Custom endpoints w/ secrets can be used via msiEndpoint and MSI_SECRET\n");
    fprintf(stdout, "2. Set the environment variables AZURE_STORAGE_ACCOUNT and (AZURE_STORAGE_ACCESS_KEY or AZURE_STORAGE_SAS_TOKEN), and specify the container name with --container-name=\n\n");
    fprintf(stdout, "See https://github.com/Azure/azure-storage-fuse for detailed installation and advanced configuration instructions, including SPN and MSI via environment variables.\n");
}

void print_version()
{
    fprintf(stdout, "blobfuse %s\n", BFUSE_VER);
}

int set_log_mask(const char * min_log_level_char, bool blobfuseInit)
{
    if (!min_log_level_char)
    {
        syslog(LOG_CRIT, "Setting logging level to : LOG_WARNING");
        setlogmask(LOG_UPTO(LOG_WARNING));
        return 0;
    }
    gEnableLogsHttp = false;

    std::string min_log_level(min_log_level_char);
    if (min_log_level.empty())
    {
        syslog(LOG_CRIT, "Setting logging level to : LOG_WARNING");
        setlogmask(LOG_UPTO(LOG_WARNING));
        return 0;
    }
    
    syslog(LOG_CRIT, "Setting logging level to : %s", min_log_level.c_str());

    // cmd_options for logging: LOG_OFF, LOG_CRIT, LOG_ERR, LOG_WARNING, LOG_INFO, LOG_DEBUG
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
        gEnableLogsHttp = true;
        setlogmask(LOG_UPTO(LOG_INFO));
        return 0;
    }
    if (min_log_level == "LOG_DEBUG")
    {
        gEnableLogsHttp = true;
        setlogmask(LOG_UPTO(LOG_DEBUG));
        return 0;
    }

    if (blobfuseInit) {
        syslog(LOG_CRIT, "Unable to start blobfuse. Error: Invalid log level \"%s\"", min_log_level.c_str());
        fprintf(stderr, "Unable to start blobfuse. Error: Invalid log level Specified\"%s\".  Permitted values are LOG_OFF, LOG_CRIT, LOG_ERR, LOG_WARNING, LOG_INFO, LOG_DEBUG.\n", min_log_level.c_str());
        fprintf(stdout, "If --log-level is not specified logging level will default to  LOG_WARNING.\n\n");
    } else {
        set_log_mask(cmd_options.log_level, false);
    }
    return 1;
}

void set_new_sas_token()
{
    syslog(LOG_DEBUG, "Sas Token Refresh");
    storage_client->RefreshSASToken(config_options.sasToken);
}

/*
 *  This function is called only during SIGUSR1 handling.
 *  Objective here is to read only the 'logLevel' from the config file
 *  If logLevel is removed from config file then reset the logging level
 *  back to what was provided int the command line cmd_options, otherwise use
 *  this config as the new logging level..
 */

#define LOG_LVL_REFRESH             (1 << 0)
#define SAS_TOKEN_REFRESH           (1 << 1)
unsigned int CONF_FILE_REFRESH = 0;

int refresh_from_config_file(const std::string configFile)
{
    CONF_FILE_REFRESH = 0;

    std::ifstream file(configFile);
    if(!file)
    {
        syslog(LOG_CRIT, "Unable to read config file : %s", configFile.c_str());
        fprintf(stderr, "Unable to read config file : %s", configFile.c_str());
        return -1;
    }

    std::string line;
    bool logLevelFound = false;
    while(std::getline(file, line))
    {
        // skip over comments
        if(line[0] == '#') {
            continue;
        }

        // Refresh logging level
        std::size_t pos = line.find("logLevel");
        if(pos != std::string::npos)
        {
           std::string logLevel = trim(line.substr(line.find(" ") + 1));
           if (logLevel.compare(config_options.logLevel)) {
                config_options.logLevel = logLevel;
                logLevelFound = true;
                CONF_FILE_REFRESH |= LOG_LVL_REFRESH;
           }
        }

        // Refresh the SAS token so that token change does not require remount
        if(line.find("sasToken") != std::string::npos)
        {
            std::string sasTokenStr = trim(line.substr(line.find(" ") + 1));
            if (sasTokenStr.compare(config_options.sasToken)) {
                config_options.sasToken = sasTokenStr;
                CONF_FILE_REFRESH |= SAS_TOKEN_REFRESH;
            }
        }
    }

    if(!logLevelFound) {
        config_options.logLevel = (cmd_options.log_level) ? : "";
    }

    return 0;
}

void sig_usr_handler(int signum)
{
    if (signum == SIGUSR1) {
        syslog(LOG_INFO, "Received signal SIGUSR1");
        if (0 == refresh_from_config_file(cmd_options.config_file)) {
            if (CONF_FILE_REFRESH & LOG_LVL_REFRESH)
                set_log_mask(config_options.logLevel.c_str(), false);
            
            if (CONF_FILE_REFRESH & SAS_TOKEN_REFRESH)
                set_new_sas_token();
        }
    }
}

void set_up_callbacks(struct fuse_operations &azs_blob_operations)
{
    openlog(log_ident.c_str(), LOG_NDELAY | LOG_PID, 0);

    // Here, we set up all the callbacks that FUSE requires.
    azs_blob_operations.init = azs_init;
    azs_blob_operations.getattr = azs_getattr;
    azs_blob_operations.statfs = azs_statfs;
    azs_blob_operations.access = azs_access;
    azs_blob_operations.readlink = azs_readlink;
    azs_blob_operations.symlink = azs_symlink;
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

    signal(SIGUSR1, sig_usr_handler);
}

/*
 *  Check if the given directory is already mounted for the mounttype fuse or not
 *  If mounted then re-mounting again shall fail.
 */ 
bool is_directory_mounted(const char* mntDir) {
    bool found = false;

    if (!config_options.basicRemountCheck) {
        syslog(LOG_INFO, "Using syscall to detect directory is already mounted or not");
        struct mntent *mnt_ent;

        FILE *mnt_list;

        mnt_list = setmntent(_PATH_MOUNTED, "r");
        while ((mnt_ent = getmntent(mnt_list))) 
        {
            if (!strcmp(mnt_ent->mnt_dir, mntDir) && !strcmp(mnt_ent->mnt_type, "fuse")) 
            {
                found = true;
                break;
            }
        }
        endmntent(mnt_list);
    } else {
        syslog(LOG_INFO, "Reading /etc/mtab to detect directory is already mounted or not");
        ssize_t read = 0;
        size_t len = 0;
        char *line = NULL;

        FILE *fp = fopen("/etc/mtab", "r");
        if (fp != NULL) {
            while ((read = getline(&line, &len, fp)) != -1) {
                if (strstr(line, mntDir) != NULL && strstr(line, "fuse") != NULL) 
                {
                    found = true;
                    break;
                }
            }
        fclose(fp);
        }
    }
    
    return found;
}

/*
 *  Check if the given temp directory is empty or not
 *  If non-empty then return false, as this can create issues in 
 *  out cache.
 */ 
bool is_directory_empty(const char *tmpDir) {
    int cnt = 0;
    struct dirent *d_ent;

    DIR *dir = opendir(tmpDir);
    if (dir == NULL) 
        return 0;

    // Count number of entries in directory
    // if its more then 2 (. and ..) then its not empty
    while (((d_ent = readdir(dir)) != NULL) 
            && (cnt++ <= 2));
    closedir(dir);
    
    //fprintf(stdout, "count of dir entries %u", cnt);
    return (cnt <= 2);
}


int 

read_and_set_arguments(int argc, char *argv[], struct fuse_args *args)
{
    // FUSE has a standard method of argument parsing, here we just follow the pattern.
    *args = FUSE_ARGS_INIT(argc, argv);

    // Check for existence of allow_other flag and change the default permissions based on that
    config_options.defaultPermission = 0770;
    config_options.readOnlyMount = false;

    std::vector<std::string> string_args(argv, argv+argc);
    for (size_t i = 1; i < string_args.size(); ++i) {
      if (string_args[i].find("allow_other") != std::string::npos) {
          config_options.defaultPermission = 0777; 
      }
      if (string_args[i].find("ro") != std::string::npos) {
          config_options.readOnlyMount = true;
      }
    }

    int ret = 0;
    config_options.useADLS = false;
    config_options.noSymlinks = false;
    config_options.cacheOnList = true;
    config_options.high_disk_threshold = HIGH_THRESHOLD_VALUE;
    config_options.low_disk_threshold = LOW_THRESHOLD_VALUE;

    try
    {

        if (fuse_opt_parse(args, &cmd_options, option_spec, NULL) == -1)
        {
            return 1;
        }

        if(cmd_options.version)
        {
            print_version();
            exit(0);
        }

        if(cmd_options.help)
        {
            print_usage();
            exit(0);
        }

        config_options.basicRemountCheck = false;
        if (cmd_options.basic_remount_check != NULL)
        {
            std::string remnt_chk(cmd_options.basic_remount_check);
            if (remnt_chk == "true")
            {
                syslog(LOG_INFO, "Basic remount check enabled");
                config_options.basicRemountCheck = true;
            }
        }

        if (args && args->argv && argc > 1 && 
            is_directory_mounted(argv[1])) 
        {
            syslog(LOG_CRIT, "Unable to start blobfuse. '%s'is already mounted.", argv[1]);
            fprintf(stderr, "Error: '%s' is already mounted. Recheck your config\n", argv[1]);
            return 1;
        }
        config_options.mntPath = std::string(argv[1]);

        if(!cmd_options.config_file) {
            fprintf(stdout, "no config file");
            if(!cmd_options.container_name)
            {
                syslog(LOG_CRIT, "Unable to start blobfuse, no config file provided and --container-name is not set.");
                fprintf(stderr, "Error: No config file provided and --container-name is not set.\n");
                print_usage();
                return 1;
            }

            std::string container(cmd_options.container_name);
            config_options.containerName = container;
            if(cmd_options.httpsProxy)
            {
                std::string httpsProxy(cmd_options.httpsProxy);
                config_options.httpsProxy = httpsProxy;
            }
            if(cmd_options.caCertFile)
            {
                std::string caCertFile(cmd_options.caCertFile);
                config_options.caCertFile = caCertFile;
            }
            if(cmd_options.httpProxy)
            {
                std::string httpProxy(cmd_options.httpProxy);
                config_options.httpProxy = httpProxy;
            }

            ret = read_config_env();
            fprintf(stdout, "done reading env vars");
        }
        else
        {
            ret = read_config(cmd_options.config_file);
        }

        if (ret != 0)
        {
            return ret;
        }
    }
    catch(std::exception &ex)
    {
        fprintf(stderr, "Exception thrown while parsing arguments %s", ex.what());
        print_usage();
        return 1;
    }

    int res = set_log_mask(cmd_options.log_level, true);
    if (res != 0)
    {
        print_usage();
        return 1;
    }

    // remove last trailing slash in tmp_path
    if(!cmd_options.tmp_path)
    {
        fprintf(stderr, "Error: --tmp-path is not set.\n");
        print_usage();
        return 1;
    }

    std::string tmpPathStr(cmd_options.tmp_path);
    if (!tmpPathStr.empty())
    {
        // First let's normalize the path
        // Don't use canonical because that will check for path existence and permissions
#if BOOST_VERSION > 106000 // lexically_normal was added in boost 1.60.0; ubuntu 16 is only up to 1.58.0
        tmpPathStr = boost::filesystem::path(tmpPathStr).lexically_normal().string();
#else
        tmpPathStr = boost::filesystem::path(tmpPathStr).normalize().string();
#endif

        // Double check that we have not just emptied this string
        if (!tmpPathStr.empty())
        {
            // Trim any trailing '/' or '/.'
            // This will also create a blank string for just '/' which will fail out at the next block
            // .lexically_normal() returns '/.' for directories
            if (tmpPathStr[tmpPathStr.size() - 1] == '/')
            {
                tmpPathStr.erase(tmpPathStr.size() - 1);
            }
            else if (tmpPathStr.size() > 1 && tmpPathStr.compare((tmpPathStr.size() - 2), 2, "/.") == 0)
            {
                tmpPathStr.erase(tmpPathStr.size() - 2);
            }

            if (tmpPathStr[0] == '~') {
                const char *homedir = NULL;
                if ((homedir = getenv("HOME")) == NULL) {
                    homedir = getpwuid(getuid())->pw_dir;
                }
                if (homedir) {
                    syslog(LOG_ERR,"Expanding '~' in tmppath to %s", homedir);
                    tmpPathStr = std::string(homedir) + tmpPathStr.substr(1);
                }
            }
        }

        // Error out if we emptied this string
        if (tmpPathStr.empty())
        {
            fprintf(stderr, "Error: --tmp-path resolved to empty path.\n");
            print_usage();
            return 1;
        }
    }

    config_options.emptyDirCheck = false;
    if (cmd_options.empty_dir_check != NULL) {
        std::string val(cmd_options.empty_dir_check);
        if(val == "true")
        {
            config_options.emptyDirCheck = true;
        }
    }

    config_options.uploadIfModified = false;
    if (cmd_options.upload_if_modified != NULL) {
        syslog(LOG_DEBUG, "upload_if_modified is not null");
        std::string val(cmd_options.upload_if_modified);
        if(val == "true")
        {
            config_options.uploadIfModified = true;
            syslog(LOG_DEBUG, "upload_if_modified is true");
        }
    }
    
    if (!tmpPathStr.empty())
    {    
        bool fail_mount = false;
        struct stat sb;

        // if the directory does not exist no need to valdiate if it is empty
        // so check if the dir exists first and then validate
        if (stat(tmpPathStr.c_str(), &sb) == 0 && S_ISDIR(sb.st_mode)) 
        {            
            if  (!is_directory_empty(tmpPathStr.c_str()) &&
                 config_options.emptyDirCheck)
            {
                // Tmp path exists. if 'root' directory is empty then also its fine
                std::string tmprootPath = tmpPathStr + "/root";
                if (stat(tmprootPath.c_str(), &sb) == 0 && S_ISDIR(sb.st_mode)) {
                    if  (!is_directory_empty(tmprootPath.c_str())) {
                        fail_mount = true;
                    }
                } else {
                    fail_mount = true;
                }

                if (fail_mount) {
                    syslog(LOG_CRIT, "Unable to start blobfuse. temp directory '%s'is not empty.", tmpPathStr.c_str());
                    fprintf(stderr, "Error: temp directory '%s' is not empty. blobfuse needs an empty temp directory\n", tmpPathStr.c_str());
                    return 1;
                }
            }
        }
    }

    config_options.tmpPath = tmpPathStr;
    config_options.useHttps = true;
    if (cmd_options.useHttps != NULL)
    {
        std::string https(cmd_options.useHttps);
        if (https == "false")
        {
            config_options.useHttps = false;
        }
    }

    config_options.useAttrCache = false;
    if (cmd_options.useAttrCache != NULL)
    {
        std::string attr_cache(cmd_options.useAttrCache);
        if (attr_cache == "true")
        {
            config_options.useAttrCache = true;
        }
    }

    if (cmd_options.file_cache_timeout_in_seconds != NULL)
    {
        std::string timeout(cmd_options.file_cache_timeout_in_seconds);
        config_options.fileCacheTimeoutInSeconds = stoi(timeout);
    }
    else
    {
        config_options.fileCacheTimeoutInSeconds = 120;
    }

    if(cmd_options.use_adls != NULL)
    {
        std::string use_adls_value(cmd_options.use_adls);
        if(use_adls_value == "true")
        {
            config_options.useADLS = true;
        } else if(use_adls_value == "false") {
            config_options.useADLS = false;
        }
    }

    if(cmd_options.no_symlinks != NULL)
    {
        std::string no_symlinks(cmd_options.no_symlinks);
        if(no_symlinks == "true")
        {
            config_options.noSymlinks = true;
        } 
    }

    if(cmd_options.cache_on_list != NULL)
    {
        std::string cache_prop(cmd_options.cache_on_list);
        if(cache_prop == "false")
        {
            config_options.cacheOnList = false;
        } 
    }

    config_options.concurrency = (int)(blobfuse_constants::def_concurrency_blob_wrapper);
    if(cmd_options.concurrency != NULL)
    {
        std::string concur(cmd_options.concurrency);
        //config_options.concurrency = stoi(concur);
        config_options.concurrency = (stoi(concur) < blobfuse_constants::max_concurrency_blob_wrapper) ? 
                stoi(concur) : blobfuse_constants::max_concurrency_blob_wrapper;
    }

    config_options.cacheSize = 0;
    if (cmd_options.cache_size_mb != NULL) 
    {
        std::string cache_size(cmd_options.cache_size_mb);
        config_options.cacheSize = stoi(cache_size) * (unsigned long long)(1024l * 1024l);
    }

    config_options.cancel_list_on_mount_secs = 0;
    if (cmd_options.cancel_list_on_mount_seconds != NULL) 
    {
        std::string cancel_list_on_mount_secs(cmd_options.cancel_list_on_mount_seconds);
        config_options.cancel_list_on_mount_secs = stoi(cancel_list_on_mount_secs);
    }

    // Make high and low disk threshold percentage a configurable option
    if (cmd_options.high_disk_threshold != NULL) 
    {
        std::string high_disk_thr(cmd_options.high_disk_threshold);
        config_options.high_disk_threshold = stoi(high_disk_thr);
        if (config_options.high_disk_threshold < 0 || config_options.high_disk_threshold > 100) {
            syslog(LOG_ERR, "Invalid high_disk_threshold, reverting back to default value");
            config_options.high_disk_threshold = HIGH_THRESHOLD_VALUE;
        }
    }

    if (cmd_options.low_disk_threshold != NULL) 
    {
        std::string low_disk_thr(cmd_options.low_disk_threshold);
        config_options.low_disk_threshold = stoi(low_disk_thr);
        if (config_options.low_disk_threshold < 0 || config_options.low_disk_threshold > 100) {
            syslog(LOG_ERR, "Invalid low_disk_threshold, reverting back to default value");
            config_options.low_disk_threshold = LOW_THRESHOLD_VALUE;
        }
    }

    if (config_options.low_disk_threshold >= config_options.high_disk_threshold) {
        syslog(LOG_ERR, "Invalid disk_thresholds, reverting back to default values");
        config_options.high_disk_threshold = HIGH_THRESHOLD_VALUE;
        config_options.low_disk_threshold = LOW_THRESHOLD_VALUE;
    }
    
    config_options.cachePollTimeout = 1000;
    if (cmd_options.cache_poll_timeout_msec != NULL) 
    {
        std::string timeout(cmd_options.cache_poll_timeout_msec);
        config_options.cachePollTimeout = stoi(timeout) * 1000;
    }

    config_options.maxEviction = 0;
    if (cmd_options.max_eviction != NULL) 
    {
        std::string max_evic(cmd_options.max_eviction);
        config_options.maxEviction = stoi(max_evic);
    }

    if (cmd_options.set_content_type != NULL) 
    {
        std::string val(cmd_options.set_content_type);
        if (val == "true")
            gSetContentType = true;
    }

    config_options.preMountValidate = false;
    if (cmd_options.pre_mount_validate != NULL) 
    {
        std::string val(cmd_options.pre_mount_validate);
        if (val == "true") {
            syslog(LOG_INFO, "Pre mount validation enabled");
            config_options.preMountValidate = true;
        }
    }

    // Azure retry policy config
    std::size_t offset = 0;
    config_options.maxTryCount = 0;
    if (cmd_options.max_retry != NULL) 
    {
        std::string max_retry(cmd_options.max_retry);
        config_options.maxTryCount = stoi(max_retry);
    }
    
    config_options.maxTimeoutSeconds = 60.0;
    if (cmd_options.max_timeout != NULL) 
    {
        std::string max_timeout(cmd_options.max_timeout);
        config_options.maxTimeoutSeconds = stod(max_timeout, &offset);
    }
    
    config_options.retryDelay = 1.2;
    if (cmd_options.retry_delay != NULL) 
    {
        std::string retry_delay(cmd_options.retry_delay);
        config_options.retryDelay = stod(retry_delay, &offset);
    }

    config_options.streamRead = false;
    if(cmd_options.stream_read != NULL)
    {
        std::string stream_read(cmd_options.stream_read);
        if(stream_read == "true")
        {
            config_options.streamRead = true;
        } 
    }
    
    config_options.readStreamBufferSize = (500 * 1024 * 1024);
    if (cmd_options.stream_buffer != NULL) 
    {
        std::string stream_buffer(cmd_options.stream_buffer);
        config_options.readStreamBufferSize = uint64_t(stod(stream_buffer, &offset))  * uint64_t(1024 * 1024);
    }

    if (config_options.streamRead && !config_options.readOnlyMount) {
        syslog(LOG_ERR, "Streaming Read is supported only on Readonly Mounts. Use '-o ro' option in mount command");    
        fprintf(stderr, "Streaming Read is supported only on Readonly Mounts. Use '-o ro' option in mount command");  
        return 1;
    }

    syslog(LOG_INFO, "Disk Thresholds : %d - %d, Cache Eviction : %llu-%llu, List Cancel time : %d Retry Policy (%d, %f, %f)", 
        config_options.high_disk_threshold, config_options.low_disk_threshold,
        config_options.cachePollTimeout, config_options.maxEviction,
        config_options.cancel_list_on_mount_secs,
        config_options.maxTryCount, config_options.maxTimeoutSeconds, config_options.retryDelay);

    if (config_options.streamRead) {
        syslog(LOG_INFO, "Streaming Read : %d, Stream buffer size : %lu", 
            config_options.streamRead, config_options.readStreamBufferSize);
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

void configure_fuse(struct fuse_args *args)
{
    populate_kernel_version();
    populate_libcurl_version();

    if (kernel_version < blobfuse_constants::minKernelVersion) {
        fuse_opt_add_arg(args, "-omax_read=131072");
        fuse_opt_add_arg(args, "-omax_write=131072");
    }

    if (cmd_options.file_cache_timeout_in_seconds != NULL)
    {
        std::string timeout(cmd_options.file_cache_timeout_in_seconds);
        config_options.fileCacheTimeoutInSeconds = stoi(timeout);
    }
    else
    {
        config_options.fileCacheTimeoutInSeconds = 120;
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

    // if https is turned off and an http proxy server is specified just set https proxy server to the http proxy server
    // this is a hack but we only use the https_proxy 'variable' while calling the cpplite library and setting this will avoid iof condition checks
    if(!config_options.useHttps && !config_options.httpProxy.empty())
    {
        config_options.httpsProxy = config_options.httpProxy;
    }
    
    //initialize storage client and authenticate, if we fail here, don't call fuse
    if (config_options.useAttrCache) 
    {
        syslog(LOG_INFO, "Initializing blobfuse with attr caching enabled");
        storage_client = std::make_shared<AttrCacheBfsClient>(config_options);
    }
    else if (config_options.useADLS)
    {
        syslog(LOG_INFO, "Initializing blobfuse using DataLake");
        storage_client = std::make_shared<DataLakeBfsClient>(config_options);
    }
    else
    {
        syslog(LOG_DEBUG, "Initializing blobfuse using BlockBlob");
        storage_client = std::make_shared<BlockBlobBfsClient>(config_options);
        syslog(LOG_DEBUG, "Setup storage client");
    }

    syslog(LOG_DEBUG, "Kernel version is %f", kernel_version);
    syslog(LOG_DEBUG, "libcurl version is %f", libcurl_version);

    if (!config_options.preMountValidate && libcurl_version < blobfuse_constants::minCurlVersion) 
    {
        syslog(LOG_WARNING, "** Delaying authentication to post fork for older curl versions");
    } else {
        if(storage_client->AuthenticateStorage())
        {
            syslog(LOG_DEBUG, "Successfully Authenticated!");   
        }
        else
        {
            fprintf(stderr, "Unable to start blobfuse due to authentication or connectivity issues. Please check the readme for valid auth setups.\n");
            syslog(LOG_ERR, "Unable to start blobfuse due to authentication or connectivity issues. Please check the readme for valid auth setups.");
            return -1;
        }
    }

    globalTimes.lastModifiedTime = globalTimes.lastAccessTime = globalTimes.lastChangeTime = time(NULL);
    return 0;
}
