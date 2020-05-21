#include "blobfuse.h"
#include <boost/filesystem.hpp>
#include <string>
#include <signal.h>
#include <mntent.h>
#include <sys/types.h>
#include <dirent.h>

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

    if(env_account)
    {
        str_options.accountName = env_account;

        if(env_account_key)
        {
            str_options.accountKey = env_account_key;
        }

        if(env_sas_token)
        {
            str_options.sasToken = env_sas_token;
        }

        if(env_identity_client_id)
        {
            str_options.identityClientId = env_identity_client_id;
        }

        if (env_spn_client_secret)
        {
            str_options.spnClientSecret = env_spn_client_secret;
        }

        if (env_spn_tenant_id)
        {
            str_options.spnTenantId = env_spn_tenant_id;
        }

        if (env_spn_client_id)
        {
            str_options.spnClientId = env_spn_client_id;
        }

        if(env_identity_object_id)
        {
            str_options.objectId = env_identity_object_id;
        }

        if(env_identity_resource_id)
        {
            str_options.resourceId = env_identity_resource_id;
        }

        if(env_managed_identity_endpoint)
        {
            str_options.msiEndpoint = env_managed_identity_endpoint;
        }

        if(env_managed_identity_secret)
        {
            str_options.msiSecret = env_managed_identity_secret;
        }

        if(env_auth_type)
        {
            str_options.authType = env_auth_type;
        }

        if(env_aad_endpoint)
        {
            str_options.aadEndpoint = env_auth_type;
        }

        if(env_blob_endpoint) {
            // Optional to specify blob endpoint
            str_options.blobEndpoint = env_blob_endpoint;
        }
    }
    else
    {
        syslog(LOG_CRIT, "Unable to start blobfuse.  No config file was specified and the AZURE_STORAGE_ACCCOUNT"
                         "environment variable was empty");
        fprintf(stderr, "Unable to start blobfuse.  No config file was specified and the AZURE_STORAGE_ACCCOUNT"
                        "environment variable was empty\n");
        return -1;
    }

    return 0;
}

auth_type get_auth_type() 
{   
    std::string lcAuthType = to_lower(str_options.authType);
    lcAuthType = trim(lcAuthType);
    int lcAuthTypeSize = (int)lcAuthType.size();
    // sometimes an extra space or tab sticks to authtype thats why this size comparison, it is not always 3 lettered
    if(lcAuthTypeSize > 0 && lcAuthTypeSize < 5) 
    {
        // an extra space or tab sticks to msi thats find and not ==, this happens when we also have an MSIEndpoint and MSI_SECRET in the config
        if (lcAuthType.find("msi") != std::string::npos) {
            // MSI does not require any parameters to work, as a lone system assigned identity will work with no parameters.
            return MSI_AUTH;
        } else if (lcAuthType == "key") {
            if(!str_options.accountKey.empty()) // An account name is already expected to be specified.
                return KEY_AUTH;
            else
                return INVALID_AUTH;
        } else if (lcAuthType == "sas") {
            if (!str_options.sasToken.empty()) // An account name is already expected to be specified.
                return SAS_AUTH;
            else
                return INVALID_AUTH;
        } else if (lcAuthType == "spn") {
            return SPN_AUTH;
        }
    } 
    else 
    {
        if (!str_options.objectId.empty() || !str_options.identityClientId.empty() || !str_options.resourceId.empty() || !str_options.msiSecret.empty() || !str_options.msiEndpoint.empty()) {
            return MSI_AUTH;
        } else if (!str_options.accountKey.empty()) {
            return KEY_AUTH;
        } else if (!str_options.sasToken.empty()) {
            return SAS_AUTH;
        } else if (!str_options.spnClientSecret.empty() && !str_options.spnClientId.empty() && !str_options.spnTenantId.empty()) {
            return SPN_AUTH;
        }
    }
    return INVALID_AUTH;
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

    char* env_spn_client_secret = getenv("AZURE_STORAGE_SPN_CLIENT_SECRET");
    char* env_msi_secret = getenv("MSI_SECRET");

    if (env_spn_client_secret) {
        str_options.spnClientSecret = env_spn_client_secret;
    }

    if (env_msi_secret) {
        str_options.msiSecret = env_msi_secret;
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
        else if(line.find("identityClientId") != std::string::npos)
        {
            std::string clientIdStr(value);
            str_options.identityClientId = clientIdStr;
        }
        else if(line.find("identityObjectId") != std::string::npos)
        {
            std::string objectIdStr(value);
            str_options.objectId = objectIdStr;
        }
        else if(line.find("identityResourceId") != std::string::npos)
        {
            std::string resourceIdStr(value);
            str_options.resourceId = resourceIdStr;
        }
        else if(line.find("authType") != std::string::npos)
        {
            std::string authTypeStr(value);
            str_options.authType = authTypeStr;
        }
        else if(line.find("msiEndpoint") != std::string::npos)
        {
            std::string msiEndpointStr(value);
            str_options.msiEndpoint = msiEndpointStr;
        }
        else if(line.find("servicePrincipalClientId") != std::string::npos)
        {
            std::string spClientIdStr(value);
            str_options.spnClientId = spClientIdStr;
        }
        else if(line.find("servicePrincipalTenantId") != std::string::npos)
        {
            std::string spTenantIdStr(value);
            str_options.spnTenantId = spTenantIdStr;
        }
        else if(line.find("aadEndpoint") != std::string::npos)
        {
            std::cout << line.find("aadEndpoint");
            std::string altAADEndpointStr(value);
            str_options.aadEndpoint = altAADEndpointStr;
        }
        else if(line.find("logLevel") != std::string::npos)
        {
            std::string logLevel(value);
            str_options.logLevel = logLevel;
        }   

        data.clear();
    }

    if(str_options.accountName.empty())
    {
        syslog (LOG_CRIT, "Unable to start blobfuse. Account name is missing in the config file.");
        fprintf(stderr, "Account name is missing in the config file.\n");
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
    // TODO: Make all of this go down roughly the same pipeline, rather than having spaghettified code
    auth_type AuthType = get_auth_type();

    if (str_options.use_attr_cache)
    {
        if(AuthType == MSI_AUTH || AuthType == SPN_AUTH)
        {
            //1. Get OAuth Token
            std::function<OAuthToken(std::shared_ptr<CurlEasyClient>)> OTMCallback = EmptyCallback;

            if (AuthType == MSI_AUTH) {
                OTMCallback = SetUpMSICallback(
                        str_options.identityClientId,
                        str_options.objectId,
                        str_options.resourceId,
                        str_options.msiEndpoint,
                        str_options.msiSecret);
            } else {
                OTMCallback = SetUpSPNCallback(
                        str_options.spnTenantId,
                        str_options.spnClientId,
                        str_options.spnClientSecret,
                        str_options.aadEndpoint);
            }

            GetTokenManagerInstance(OTMCallback); // We supply a default callback because we asssume that the oauth token manager has not initialized yet.
            //2. try to make blob client wrapper using oauth token
            //str_options.accountName
            azure_blob_client_wrapper = std::make_shared<blob_client_attr_cache_wrapper>(
                    blob_client_attr_cache_wrapper::blob_client_attr_cache_wrapper_oauth(
                     str_options.accountName,
                     constants::max_concurrency_blob_wrapper,
                     str_options.blobEndpoint));
        }
        else if(AuthType == KEY_AUTH) {
            azure_blob_client_wrapper = std::make_shared<blob_client_attr_cache_wrapper>(
                blob_client_attr_cache_wrapper::blob_client_attr_cache_wrapper_init_accountkey(
                    str_options.accountName,
                    str_options.accountKey,
                    constants::max_concurrency_blob_wrapper,
                    str_options.use_https,
                    str_options.blobEndpoint));
        }
        else if(AuthType == SAS_AUTH) {
            azure_blob_client_wrapper = std::make_shared<blob_client_attr_cache_wrapper>(
                blob_client_attr_cache_wrapper::blob_client_attr_cache_wrapper_init_sastoken(
                    str_options.accountName,
                    str_options.sasToken,
                    constants::max_concurrency_blob_wrapper,
                     str_options.use_https,
                    str_options.blobEndpoint));
        }
        else
        {
            syslog(LOG_ERR, "Unable to start blobfuse due to a lack of credentials. Please check the readme for valid auth setups.");
        }
    }
    else
    {
        //TODO: Make a for authtype, and then if that's not specified, then a check against what credentials were specified
        if(AuthType == MSI_AUTH || AuthType == SPN_AUTH)
        { // If MSI is explicit, or if MSI options are set and auth type is implicit
            //1. get oauth token
            std::function<OAuthToken(std::shared_ptr<CurlEasyClient>)> OTMCallback = EmptyCallback;

            if (AuthType == MSI_AUTH) {
                OTMCallback = SetUpMSICallback(
                        str_options.identityClientId,
                        str_options.objectId,
                        str_options.resourceId,
                        str_options.msiEndpoint,
                        str_options.msiSecret);
            } else {
                OTMCallback = SetUpSPNCallback(
                        str_options.spnTenantId,
                        str_options.spnClientId,
                        str_options.spnClientSecret,
                        str_options.aadEndpoint);
            }
            
            GetTokenManagerInstance(OTMCallback);
            //2. try to make blob client wrapper using oauth token
            azure_blob_client_wrapper = blob_client_wrapper_init_oauth(
                    str_options.accountName,
                    constants::max_concurrency_blob_wrapper,
                    str_options.blobEndpoint);
        }
        else if(AuthType == KEY_AUTH) {
            azure_blob_client_wrapper = blob_client_wrapper_init_accountkey(
            str_options.accountName,
            str_options.accountKey,
            constants::max_concurrency_blob_wrapper,
            str_options.use_https,
            str_options.blobEndpoint);
        }
        else if(AuthType == SAS_AUTH) {
            azure_blob_client_wrapper = blob_client_wrapper_init_sastoken(
            str_options.accountName,
            str_options.sasToken,
            constants::max_concurrency_blob_wrapper,
            str_options.use_https,
            str_options.blobEndpoint);
        }
        else
        {
            syslog(LOG_ERR, "Unable to start blobfuse due to a lack of credentials. Please check the readme for valid auth setups.");
        }
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

    g_gc_cache.run();

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
    fprintf(stdout, "blobfuse 1.2.3\n");
}

int set_log_mask(const char * min_log_level_char, bool blobfuseInit)
{
    if (!min_log_level_char)
    {
        syslog(LOG_CRIT, "Setting logging level to : LOG_WARNING");
        setlogmask(LOG_UPTO(LOG_WARNING));
        return 0;
    }
    std::string min_log_level(min_log_level_char);
    if (min_log_level.empty())
    {
        syslog(LOG_CRIT, "Setting logging level to : LOG_WARNING");
        setlogmask(LOG_UPTO(LOG_WARNING));
        return 0;
    }
    
    syslog(LOG_CRIT, "Setting logging level to : %s", min_log_level.c_str());

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

    if (blobfuseInit) {
        syslog(LOG_CRIT, "Unable to start blobfuse. Error: Invalid log level \"%s\"", min_log_level.c_str());
        fprintf(stderr, "Error: Invalid log level \"%s\".  Permitted values are LOG_OFF, LOG_CRIT, LOG_ERR, LOG_WARNING, LOG_INFO, LOG_DEBUG.\n", min_log_level.c_str());
        fprintf(stdout, "If not specified, logging will default to LOG_WARNING.\n\n");
    } else {
        set_log_mask(options.log_level, false);
    }
    return 1;
}

/*
 *  This function is called only during SIGUSR1 handling.
 *  Objective here is to read only the 'logLevel' from the config file
 *  If logLevel is removed from config file then reset the logging level
 *  back to what was provided int the command line options, otherwise use
 *  this config as the new logging level..
 */
int refresh_from_config_file(const std::string configFile)
{
    std::ifstream file(configFile);
    if(!file)
    {
        syslog(LOG_CRIT, "Unable to read config file : %s", configFile.c_str());
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

       std::size_t pos = line.find("logLevel");
        if(pos != std::string::npos)
        {
           std::string logLevel = line.substr(line.find(" ")+1);
           str_options.logLevel = trim(logLevel);
            logLevelFound = true;
        }
    }

    if(!logLevelFound) {
        str_options.logLevel = (options.log_level) ? : "";
    }

    return 0;
}

void sig_usr_handler(int signum)
{
    if (signum == SIGUSR1) {
        syslog(LOG_INFO, "Received signal SIGUSR1");
        if (0 == refresh_from_config_file(options.config_file)) {
            set_log_mask(str_options.logLevel.c_str(), false);
        }
    }
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

    signal(SIGUSR1, sig_usr_handler);
}

/*
 *  Check if the given directory is already mounted or not
 *  If mounted then re-mounting again shall fail.
 */ 
bool is_directory_mounted(const char* mntDir) {
     struct mntent *mnt_ent;
     bool found = false;
     FILE *mnt_list;
 
     mnt_list = setmntent(_PATH_MOUNTED, "r");
     while ((mnt_ent = getmntent(mnt_list))) 
     {
         if (!strcmp(mnt_ent->mnt_dir, mntDir)) 
         {
             found = true;
             break;
         }
     }
     endmntent(mnt_list);
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

    return (cnt <= 2);
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

        if (args && args->argv && argc > 1 && 
            is_directory_mounted(argv[1])) {
                syslog(LOG_CRIT, "Unable to start blobfuse. '%s'is already mounted.", argv[1]);
                fprintf(stderr, "Error: '%s' is already mounted. Recheck your config\n", argv[1]);
                return 1;
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

    int res = set_log_mask(options.log_level, true);
    if (res != 0)
    {
        print_usage();
        return 1;
    }

    // remove last trailing slash in tmp_path
    if(!options.tmp_path)
    {
        fprintf(stderr, "Error: --tmp-path is not set.\n");
        print_usage();
        return 1;
    }

    std::string tmpPathStr(options.tmp_path);
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
        }

        // Error out if we emptied this string
        if (tmpPathStr.empty())
        {
            fprintf(stderr, "Error: --tmp-path resolved to empty path.\n");
            print_usage();
            return 1;
        }
    }

    if ((!tmpPathStr.empty()) &&
        (!is_directory_empty(tmpPathStr.c_str()))) 
    {
        syslog(LOG_CRIT, "Unable to start blobfuse. temp directory '%s'is not empty.", tmpPathStr.c_str());
        fprintf(stderr, "Error: temp directory '%s' is not empty. blobfuse needs an empty temp directory\n", tmpPathStr.c_str());
        return 1;
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
        std::shared_ptr<blob_client_wrapper> temp_azure_blob_client_wrapper;
        auth_type AuthType = get_auth_type();
        //TODO: Make a for authtype, and then if that's not specified, then a check against what credentials were specified
        if(AuthType == MSI_AUTH || AuthType == SPN_AUTH)
        {
            //1. get oauth token
            std::function<OAuthToken(std::shared_ptr<CurlEasyClient>)> OTMCallback = EmptyCallback;

            if (AuthType == MSI_AUTH) {
                OTMCallback = SetUpMSICallback(
                        str_options.identityClientId,
                        str_options.objectId,
                        str_options.resourceId,
                        str_options.msiEndpoint,
                        str_options.msiSecret);
            } else {
                OTMCallback = SetUpSPNCallback(
                        str_options.spnTenantId,
                        str_options.spnClientId,
                        str_options.spnClientSecret,
                        str_options.aadEndpoint);
            }

            std::shared_ptr<OAuthTokenCredentialManager> tokenManager = GetTokenManagerInstance(OTMCallback);

            if (!tokenManager->is_valid_connection()) {
                // todo: isolate definitions of errno's for this function so we can output something meaningful.
                errno = 1;
            }

            //2. try to make blob client wrapper using oauth token
            temp_azure_blob_client_wrapper = blob_client_wrapper_init_oauth(
                    str_options.accountName,
                    constants::max_concurrency_blob_wrapper,
                    str_options.blobEndpoint);
        }
        else if(AuthType == KEY_AUTH) {
            temp_azure_blob_client_wrapper = blob_client_wrapper_init_accountkey(
            str_options.accountName,
            str_options.accountKey,
            constants::max_concurrency_blob_wrapper,
            str_options.use_https,
            str_options.blobEndpoint);
        }
        else if(AuthType == SAS_AUTH) {
            temp_azure_blob_client_wrapper = blob_client_wrapper_init_sastoken(
            str_options.accountName,
            str_options.sasToken,
            constants::max_concurrency_blob_wrapper,
            str_options.use_https,
            str_options.blobEndpoint);
        }
        else
        {
            syslog(LOG_ERR, "Unable to start blobfuse due to a lack of credentials. Please check the readme for valid auth setups.");
            errno = 1;
        }
        if(errno != 0)
        {
            syslog(LOG_CRIT, "Unable to start blobfuse.  Creating local blob client failed: errno = %d.\n", errno);
            fprintf(stderr, "Unable to start blobfuse due to a lack of credentials. Please check the readme for valid auth setups."
                            " Creating blob client failed: errno = %d.\n", errno);
            return 1;
        }

        // Check if the account name/key and container is correct by attempting to list a blob.
        // This will succeed even if there are zero blobs.
        list_blobs_hierarchical_response response = temp_azure_blob_client_wrapper->list_blobs_hierarchical(
            str_options.containerName,
            "/",
            std::string(),
            std::string(),
            1);
        if(errno != 0)
        {
            syslog(LOG_CRIT, "Unable to start blobfuse.  Failed to connect to the storage container. There might be something wrong about the storage config, please double check the storage account name, account key/sas token/OAuth access token and container name. errno = %d\n", errno);
            fprintf(stderr, "Failed to connect to the storage container. There might be something wrong about the storage config, please double check the storage account name, account key/sas token/OAuth access token and container name. errno = %d\n", errno);
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
