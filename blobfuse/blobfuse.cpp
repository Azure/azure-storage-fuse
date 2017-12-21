#include "blobfuse.h"

// FUSE contains a specific type of command-line option parsing; here we are just following the pattern.
// The only two custom options we take in are the tmpPath (path to temp / file cache directory) and the configFile (connection to Azure Storage info.)
struct options
{
    const char *tmp_path; // Path to the temp / file cache directory
    const char *config_file; // Connection to Azure Storage information (account name, account key, etc)
    const char *use_https; // True if https should be used (defaults to false)
    const char *file_cache_timeout_in_seconds; // Timeout for the file cache (defaults to 120 seconds)
    const char *list_attribute_cache; // caches file and directory attributes in temp location without the contents
};

struct options options;
struct str_options str_options;
int file_cache_timeout_in_seconds;
bool list_attribute_cache;

#define OPTION(t, p) { t, offsetof(struct options, p), 1 }
const struct fuse_opt option_spec[] =
{
    OPTION("--tmp-path=%s", tmp_path),
    OPTION("--config-file=%s", config_file),
    OPTION("--use-https=%s", use_https),
    OPTION("--file-cache-timeout-in-seconds=%s", file_cache_timeout_in_seconds),
    OPTION("--list-attribute-cache=%s", list_attribute_cache),
    FUSE_OPT_END
};

std::shared_ptr<blob_client_wrapper> azure_blob_client_wrapper;

// Currently, the cpp lite lib puts the HTTP status code in errno.
// This mapping tries to convert the HTTP status code to a standard Linux errno.
// TODO: Ensure that we map any potential HTTP status codes we might receive.
std::map<int, int> error_mapping = {{404, ENOENT}, {403, EACCES}, {1600, ENOENT}};

const std::string directorySignifier = ".directory";

static struct fuse_operations azs_blob_operations;

inline bool is_lowercase_string(const std::string &s)
{
    return (s.size() == static_cast<size_t>(std::count_if(s.begin(), s.end(),[](unsigned char c)
    {
        return std::islower(c);
    })));
}

// Read Storage connection information from the config file
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

    while(std::getline(file, line))
    {

        data.str(line.substr(line.find(" ")+1));

        if(line.find("accountName") != std::string::npos)
        {
            std::string accountNameStr(data.str());
            /*            if(!is_lowercase_string(accountNameStr))
                        {
                            fprintf(stderr, "Account name must be lower cases.");
                            return -1;
                        }
                        else
                        {*/
            str_options.accountName = accountNameStr;
//            }
        }
        else if(line.find("accountKey") != std::string::npos)
        {
            std::string accountKeyStr(data.str());
            str_options.accountKey = accountKeyStr;
        }
        else if(line.find("containerName") != std::string::npos)
        {
            std::string containerNameStr(data.str());
            /*            if(!is_lowercase_string(containerNameStr))
                        {
                            fprintf(stderr, "Container name must be lower cases.");
                            return -1;
                        }
                        else
                        {*/
            str_options.containerName = containerNameStr;
//            }
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

int test_sparse_files()
{

   // path to testfile in temp location
   std::string pathStr("testfile");
   std::string mntPathString = prepend_mnt_path_string(pathStr);

   // create the test file
   int fd = open((mntPathString).c_str(), O_RDWR | O_CREAT, S_IRWXU | S_IRWXG);
   if(fd == -1)
   {
       return 1;
   }
   else
   {
       int res = ftruncate(fd, 1000000);
       if(res == -1)
       {
           return 1;
       }

       // ensure data to be synced to disk
       syncfs(fd);

       // now test whether the file allocates any blocks on disk
       struct stat buf;
       int statret = stat(mntPathString.c_str(), &buf);
       if(statret == 0 && buf.st_blocks == 0)
       {
           unlink(mntPathString.c_str());
           return 0;
       }
       else
       {
           unlink(mntPathString.c_str());
           return 1;
       }
   }

   return 1;
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

// TODO: print FUSE usage as well
void print_usage()
{
    fprintf(stdout, "Usage: blobfuse <mount-folder> --config-file=<config-file> --tmp-path=<temp-path> [--use-https=false] [--file-cache-timeout-in-seconds=120] [--list-attribute-cache=false]\n");
    fprintf(stdout, "Please see https://github.com/Azure/azure-storage-fuse for installation and configuration instructions.\n");
}

int main(int argc, char *argv[])
{
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

    // FUSE has a standard method of argument parsing, here we just follow the pattern.
    struct fuse_args args = FUSE_ARGS_INIT(argc, argv);
    int ret = 0;
    try
    {

        if (fuse_opt_parse(&args, &options, option_spec, NULL) == -1)
        {
            return 1;
        }

        ret = read_config(options.config_file);
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

    std::string tmpPathStr(options.tmp_path);
    str_options.tmpPath = tmpPathStr;
    const int defaultMaxConcurrency = 20;
    bool use_https = false;
    if (options.use_https != NULL)
    {
        std::string https(options.use_https);
        if (https == "true")
        {
            use_https = true;
        }
    }

    list_attribute_cache = false;
    if (options.list_attribute_cache != NULL)
    {
        std::string list_attribute_cache_string(options.list_attribute_cache);
        if(list_attribute_cache_string == "true" && test_sparse_files() == 0)
        {
            list_attribute_cache = true;
        }
    }

    azure_blob_client_wrapper = std::make_shared<blob_client_wrapper>(blob_client_wrapper::blob_client_wrapper_init(str_options.accountName, str_options.accountKey, defaultMaxConcurrency, use_https));
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

    fuse_opt_add_arg(&args, "-omax_read=131072");
    fuse_opt_add_arg(&args, "-omax_write=131072");
    if(0 != ensure_files_directory_exists_in_cache(prepend_mnt_path_string("/placeholder")))
    {
        fprintf(stderr, "Failed to create direcotry on cache directory: %s, errno = %d.\n", prepend_mnt_path_string("/placeholder").c_str(),  errno);
        return 1;
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

    // FUSE contains a feature where it automatically implements 'soft' delete if one process has a file open when another calls unlink().
    // This feature causes us a bunch of problems, so we use "-ohard_remove" to disable it, and track the needed 'soft delete' functionality on our own.
    fuse_opt_add_arg(&args, "-ohard_remove");
    fuse_opt_add_arg(&args, "-obig_writes");
    fuse_opt_add_arg(&args, "-ofsname=blobfuse");
    fuse_opt_add_arg(&args, "-okernel_cache");
    umask(0);

    ret =  fuse_main(args.argc, args.argv, &azs_blob_operations, NULL);

    return ret;
}
