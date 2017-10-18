#include "blobfuse.h"

// FUSE contains a specific type of command-line option parsing; here we are just following the pattern.
// The only two custom options we take in are the tmpPath (path to temp / file cache directory) and the configFile (connection to Azure Storage info.)
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


// Read Storage connection information from the config file
int read_config(std::string configFile)
{
    std::ifstream file(configFile);
    if(!file)
    {
        std::cout<<"No config file found at " << configFile <<std::endl;
        return -1;
    }

    std::string line;
    std::istringstream data;

    while(std::getline(file, line)) {

        data.str(line.substr(line.find(" ")+1));

        if(line.find("accountName") != std::string::npos){
            std::string accountNameStr(data.str());
            str_options.accountName = accountNameStr;
        }
        else if(line.find("accountKey") != std::string::npos){
            std::string accountKeyStr(data.str());
            str_options.accountKey = accountKeyStr;
        }
        else if(line.find("containerName") != std::string::npos){
            std::string containerNameStr(data.str());
            str_options.containerName = containerNameStr;
        }

        data.clear();
    }

    return 0;

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
    fprintf(stdout, "Usage: blobfuse <mount-folder> --configfile=<config-file> --tmpPath=<temp-path>\n");
}

int main(int argc, char *argv[])
{
	// Here, we set up all the callbacks that FUSE requires.
    azs_blob_readonly_operations.init = azs_init;
    azs_blob_readonly_operations.getattr = azs_getattr;
    azs_blob_readonly_operations.statfs = azs_statfs;
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

    // FUSE has a standard method of argument parsing, here we just follow the pattern.
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

    azure_blob_client_wrapper = std::make_shared<blob_client_wrapper>(blob_client_wrapper::blob_client_wrapper_init(str_options.accountName, str_options.accountKey, 20));
    if(errno != 0)
    {
        fprintf(stderr, "Creating blob client failed.");
        return 1;
    }
    fuse_opt_add_arg(&args, "-omax_read=4194304");  // Increase the read buffer size as high as possible


	// FUSE contains a feature whereit automatically implements 'soft' delete if one process has a file open when another calls unlink().
	// This feature causes us a bunch of problems, so we use "-ohard_remove" to disable it, and track the needed 'soft delete' functionality on our own.
	fuse_opt_add_arg(&args, "-ohard_remove"); 
    ensure_files_directory_exists(prepend_mnt_path_string("/placeholder"));

    umask(0);

    ret =  fuse_main(args.argc, args.argv, &azs_blob_readonly_operations, NULL);

    return ret;
}
