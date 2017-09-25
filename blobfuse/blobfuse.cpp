#include "blobfuse.h"

struct options {
    const char *tmpPath;
    const char *configFile;
};

struct options options;
struct str_options str_options;

#define OPTION(t, p) { t, offsetof(struct options, p), 1 }
const struct fuse_opt option_spec[] = {
    OPTION("--tmpPath=%s", tmpPath),
    OPTION("--configFile=%s", configFile),
    FUSE_OPT_END
};


std::shared_ptr<blob_client_wrapper> azure_blob_client_wrapper;
std::map<int, int> error_mapping = {{404, ENOENT}, {403, EACCES}, {1600, ENOENT}};
const std::string directorySignifier = ".directory";

static struct fuse_operations azs_blob_readonly_operations;


inline bool is_lowercase_string(const std::string &s)
{
    return (s.size() == static_cast<size_t>(std::count_if(s.begin(), s.end(),[](unsigned char c){ return std::islower(c); })));
}

int read_config(std::string configFile)
{
    std::ifstream file(configFile);
    if(!file)
    {
        fprintf(stderr, "No config file found at %s.\n", configFile.c_str());
        return -1;
    }

    std::string line;
    std::istringstream data;

    while(std::getline(file, line)) {

        data.str(line.substr(line.find(" ")+1));

        if(line.find("accountName") != std::string::npos){
            std::string accountNameStr(data.str());
            if(is_lowercase_string(accountNameStr))
            {
                fprintf(stderr, "Account name must be lower cases.");
                return -1;
            }
            else
            {
                str_options.accountName = accountNameStr;
            }
        }
        else if(line.find("accountKey") != std::string::npos){
            std::string accountKeyStr(data.str());
            str_options.accountKey = accountKeyStr;
        }
        else if(line.find("containerName") != std::string::npos){
            std::string containerNameStr(data.str());
            if(is_lowercase_string(containerNameStr))
            {
                fprintf(stderr, "Container name must be lower cases.");
                return -1;
            }
            else
            {
                str_options.containerName = containerNameStr;
            }
        }

        data.clear();
    }

    if(str_options.accountName.size() == 0)
    {
        fprintf(stderr, "Account name is missing in the configure file.");
        return -1;
    }
    else if(str_options.accountKey.size() == 0)
    {
        fprintf(stderr, "Account key is missing in the configure file.");
        return -1;
    }
    else if(str_options.containerName.size() == 0)
    {
        fprintf(stderr, "Container name is missing in the configure file.");
        return -1;
    }
    else
    {
        return 0;
    }
}


void *azs_init(struct fuse_conn_info * conn)
{
    /*cfg->kernel_cache = 1;
    cfg->attr_timeout = 360;
    cfg->entry_timeout = 120;
    cfg->negative_timeout = 120;
    */
    conn->max_write = 4194304;
    //conn->max_read = 4194304;
    conn->max_readahead = 4194304;
    conn->max_background = 128;
    //  conn->want |= FUSE_CAP_WRITEBACK_CACHE | FUSE_CAP_EXPORT_SUPPORT; // TODO: Investigate putting this back in when we downgrade to fuse 2.9
    return NULL;
}

void print_usage()
{
    fprintf(stdout, "Usage: blobfuse <mount-folder> --configfile=<config-file> --tmpPath=<temp-path>\n");
}

int main(int argc, char *argv[])
{
    azs_blob_readonly_operations.init = azs_init;
    azs_blob_readonly_operations.getattr = azs_getattr;
    azs_blob_readonly_operations.access = azs_access;
    azs_blob_readonly_operations.readlink = azs_readlink;
    azs_blob_readonly_operations.readdir = azs_readdir;
    azs_blob_readonly_operations.open = azs_open;
    azs_blob_readonly_operations.read = azs_read;
    azs_blob_readonly_operations.release = azs_release;
    azs_blob_readonly_operations.fsync = azs_fsync;
    azs_blob_readonly_operations.create = azs_create;
    azs_blob_readonly_operations.write = azs_write;
    azs_blob_readonly_operations.mkdir = azs_mkdir;
    azs_blob_readonly_operations.unlink = azs_unlink;
    azs_blob_readonly_operations.rmdir = azs_rmdir;
    azs_blob_readonly_operations.chown = azs_chown;
    azs_blob_readonly_operations.chmod = azs_chmod;
    //#ifdef HAVE_UTIMENSAT
    azs_blob_readonly_operations.utimens = azs_utimens;
    //#endif
    azs_blob_readonly_operations.destroy = azs_destroy;
    azs_blob_readonly_operations.truncate = azs_truncate;
    azs_blob_readonly_operations.rename = azs_rename;
    azs_blob_readonly_operations.setxattr = azs_setxattr;
    azs_blob_readonly_operations.getxattr = azs_getxattr;
    azs_blob_readonly_operations.listxattr = azs_listxattr;
    azs_blob_readonly_operations.removexattr = azs_removexattr;
    azs_blob_readonly_operations.flush = azs_flush;

    struct fuse_args args = FUSE_ARGS_INIT(argc, argv);
    int ret = 0;
    try
    {

        if (fuse_opt_parse(&args, &options, option_spec, NULL) == -1)
        {
            return 1;
        }

        ret = read_config(options.configFile);
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

    std::string tmpPathStr(options.tmpPath);
    str_options.tmpPath = tmpPathStr;
    const int defaultMaxConcurrency = 20;

    azure_blob_client_wrapper = std::make_shared<blob_client_wrapper>(blob_client_wrapper::blob_client_wrapper_init(str_options.accountName, str_options.accountKey, defaultMaxConcurrency));
    if(errno != 0)
    {
        fprintf(stderr, "Creating blob client failed: errno = %d.\n", errno);
        return 1;
    }

    // Check if the account name/key and container is correct.
    if(azure_blob_client_wrapper->container_exists(str_options.containerName) == false
      || errno != 0)
    {
        fprintf(stderr, "Failed to connect to the storage container. There might be something wrong about the storage config, please double check the storage account name, account key and container name. errno = %d\n", errno);
        return 1;
    }

    fuse_opt_add_arg(&args, "-omax_read=4194304");
    ensure_files_directory_exists(prepend_mnt_path_string("/placeholder"));

    umask(0);

    ret =  fuse_main(args.argc, args.argv, &azs_blob_readonly_operations, NULL);

    return ret;
}
