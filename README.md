# Blobfuse2 - A Microsoft supported Azure Storage FUSE driver
## About
Blobfuse2 is an open source project developed to provide a virtual filesystem backed by the Azure Storage. It uses the libfuse open source library (fuse3) to communicate with the Linux FUSE kernel module, and implements the filesystem operations using the Azure Storage REST APIs.
This is the next generation [blobfuse](https://github.com/Azure/azure-storage-fuse)

Blobfuse2 is stable, and is ***supported by Microsoft*** provided that it is used within its limits documented here. Blobfuse2 supports both reads and writes however, it does not guarantee continuous sync of data written to storage using other APIs or other mounts of Blobfuse2. For data integrity it is recommended that multiple sources do not modify the same blob/file. Please submit an issue [here](https://github.com/azure/azure-storage-fuse/issues) for any issues/feature requests/questions.

## Features
- Mount an Azure storage blob container or datalake file system on Linux.
- Basic file system operations such as mkdir, opendir, readdir, rmdir, open, 
   read, create, write, close, unlink, truncate, stat, rename
- Local caching to improve subsequent access times
- Streaming to support reading large files
- Parallel downloads and uploads to improve access time for large files
- Multiple mounts to the same container for read-only workloads

## Distinctive features compared to blobfuse (v1.x)
- Blobfuse2 is fuse3 compatible (other than Ubuntu-18 and Debian-9, where it still runs with fuse2)
- Support for higher service version offering latest and greatest of azure storage features (supported by azure go-sdk)
- Set blob tier while uploading the data to storage
- Attribute cache invalidation based on timeout
- For flat namesepce accounts, user can configure default permissions for files and folders
- Improved cache eviction algorithm for file cache to control disk footprint of blobfuse2
- Improved cache eviction algorithm for streamed buffers to control memory footprint of blobfuse2
- Utility to convert blobfuse CLI and config parameters to a blobfuse2 compatible config for easy migration
- CLI to mount Blobfuse2 with legacy Blobfuse config and CLI parameters (Refer to Migration guide for this)
- Version check and upgrade prompting 
- Option to mount a sub-directory from a container 
- CLI to mount all containers (with a allowlist and denylist) in a given storage account
- CLI to list all blobfuse2 mount points
- CLI to unmount one, multiple or all blobfuse2 mountpoints
- Option to dump logs to syslog or a file on disk
- Support for config file encryption and mounting with an encrypted config file via a passphrase (CLI or environment variable) to decrypt the config file
- CLI to check or update a parameter in the encrypted config

 ## Blobfuse2 performance compared to blobfuse(v1.x.x)
- 'git clone' operation is 25% faster (tested with vscode repo cloning)
- ResNet50 image classification job is 7-8% faster (tested with 1.3 million images)
- Regular file uploads are 10% faster
- Verified listing of 1-Billion files in a directory (which v1.x does not support)


## Download Blobfuse2
You can install Blobfuse2 by cloning this repository. In the workspace root execute `go build` to build the binary. 

<!-- ## Find Help
For complete guidance, visit any of these articles
* Blobfuse2 Wiki -->

## Supported Operations
The general format of the Blobfuse2 commands is `blobfuse2 [command] [arguments] --[flag-name]=[flag-value]`
* `help` - Help about any command
* `mount` - Mounts an Azure container as a filesystem. The supported containers include
  - Azure Blob Container
  - Azure Datalake Gen2 Container
* `mount all` - Mounts all the containers in an Azure account as a filesystem. The supported storage services include
  - [Blob Storage](https://docs.microsoft.com/en-us/azure/storage/blobs/storage-blobs-introduction)
  - [Datalake Storage Gen2](https://docs.microsoft.com/en-us/azure/storage/blobs/data-lake-storage-introduction)
* `mount list` - Lists all Blobfuse2 filesystems.
* `secure decrypt` - Decrypts a config file.
* `secure encrypt` - Encrypts a config file.
* `secure get` - Gets value of a config parameter from an encrypted config file.
* `secure set` - Updates value of a config parameter.
* `unmount` - Unmounts the Blobfuse2 filesystem.
* `unmount all` - Unmounts all Blobfuse2 filesystems.

## Find help from your command prompt
To see a list of commands, type `blobfuse2 -h` and then press the ENTER key.
To learn about a specific command, just include the name of the command (For example: `blobfuse2 mount -h`).

## Usage
- Mount with blobfuse2
    * blobfuse2 mount <mount path> --config-file=<config file>
- Mount blobfuse2 using legacy blobfuse config and cli parameters
    * blobfuse2 mountv1 <blobfuse mount cli with options>
- Mount all containers in your storage account
    * blobfuse2 mount all <mount path> --config-file=<config file>
- List all mount instances of blobfuse2
    * blobfuse2 mount list
- Unmount blobfuse2
    * sudo fusermount3 -u <mount path>
- Unmount all blobfuse2 instances
    * blobfuse2 unmount all 

<!---TODO Add Usage for mount, unmount, etc--->
## CLI parameters
- General options
    * `--config-file=<PATH>`: The path to the config file.
    * `--log-level=<LOG_*>`: The level of logs to capture.
    * `--log-file-path=<PATH>`: The path for the log file.
    * `--foreground=true`: Mounts the system in foreground mode.
    * `--read-only=true`: Mount container in read-only mode.
    * `--default-working-dir`: The default working directory to store log files and other blobfuse2 related information.
    * `--disable-version-check=true`: Disable the blobfuse2 version check.
- Attribute cache options
    * `--attr-cache-timeout=<TIMEOUT IN SECONDS>`: The timeout for the attribute cache entries.
    * `--no-symlinks=true`: To improve performance disable symlink support.
- Storage options
    * `--container-name=<CONTAINER NAME>`: The container to mount.
- File cache options
    * `--file-cache-timeout=<TIMEOUT IN SECONDS>`: Timeout for which file is cached on local system.
    * `--tmp-path=<PATH>`: The path to the file cache.
    * `--cache-size-mb=<SIZE IN MB>`: Amount of disk cache that can be used by blobfuse.
    * `--high-disk-threshold=<PERCENTAGE>`: If local cache usage exceeds this, start early eviction of files from cache.
    * `--low-disk-threshold=<PERCENTAGE>`: If local cache usage comes below this threshold then stop early eviction.
- Fuse options
    * `--attr-timeout=<TIMEOUT IN SECONDS>`: Time the kernel can cache inode attributes.
    * `--entry-timeout=<TIMEOUT IN SECONDS>`: Time the kernel can cache directory listing.
    * `--negative-timeout=<TIMEOUT IN SECONDS>`: Time the kernel can cache non-existance of file or directory.
    * `--allow-other=true`: Allow other users to have access this mount point.

## Environment variables
- General options
    * `AZURE_STORAGE_ACCOUNT`: Specifies the storage account to be connected.
    * `AZURE_STORAGE_ACCOUNT_TYPE`: Specifies the account type 'block' or 'adls'
    * `AZURE_STORAGE_ACCOUNT_CONTAINER`: Specifies the name of the container to be mounted
    * `AZURE_STORAGE_BLOB_ENDPOINT`: Specifies the blob endpoint to use. Defaults to *.blob.core.windows.net, but is useful for targeting storage emulators.
    * `AZURE_STORAGE_AUTH_TYPE`: Overrides the currently specified auth type. Case insensitive. Options: Key, SAS, MSI, SPN
- Account key auth:
    * `AZURE_STORAGE_ACCESS_KEY`: Specifies the storage account key to use for authentication.
- SAS token auth:
    * `AZURE_STORAGE_SAS_TOKEN`: Specifies the SAS token to use for authentication.
- Managed Identity auth:
    * `AZURE_STORAGE_IDENTITY_CLIENT_ID`: Only one of these three parameters are needed if multiple identities are present on the system.
    * `AZURE_STORAGE_IDENTITY_OBJECT_ID`: Only one of these three parameters are needed if multiple identities are present on the system.
    * `AZURE_STORAGE_IDENTITY_RESOURCE_ID`: Only one of these three parameters are needed if multiple identities are present on the system.
    * `MSI_ENDPOINT`: Specifies a custom managed identity endpoint, as IMDS may not be available under some scenarios. Uses the `MSI_SECRET` parameter as the `Secret` header.
    * `MSI_SECRET`: Specifies a custom secret for an alternate managed identity endpoint.
- Service Principal Name auth:
    * `AZURE_STORAGE_SPN_CLIENT_ID`: Specifies the client ID for your application registration
    * `AZURE_STORAGE_SPN_TENANT_ID`: Specifies the tenant ID for your application registration
    * `AZURE_STORAGE_AAD_ENDPOINT`: Specifies a custom AAD endpoint to authenticate against
    * `AZURE_STORAGE_SPN_CLIENT_SECRET`: Specifies the client secret for your application registration.
- Proxy Server:
    * `http_proxy`: The proxy server address. Example: http://10.1.22.4:8080/".    
    * `https_proxy`: The proxy server address when https is turned off forcing http. Example: http://10.1.22.4:8080/".

## Config file
- See [this](./sampleFileCacheConfig.yaml) sample config file.
- See [this](./setup/baseConfig.yaml) config file for a list and description of all possible configurable options in blobfuse2. 

## Frequently Asked Questions
- How do I generate a SAS with permissions for rename?
az cli has a command to generate a sas token. Open a command prompt and make sure you are logged in to az cli. Run the following command and the sas token will be displayed in the command prompt.
az storage container generate-sas --account-name <account name ex:myadlsaccount> --account-key <accountKey> -n <container name> --permissions dlrwac --start <today's date ex: 2021-03-26> --expiry <date greater than the current time ex:2021-03-28>

## Un-Supported File system operations
- mkfifo : fifo creation is not supported by blobfuse2 and this will result in "function not implemented" error
- chown  : Change of ownership is not supported by Azure Storage hence Blobfuse2 does not support this.
- Creation of device files or pipes is not supported by Blobfuse2.
- Blobfuse2 does not support extended-attributes (x-attrs) operations

## Un-Supported Scenarios
- Blobfuse2 does not support overlapping mount paths. While running multiple instances of Blobfuse2 make sure each instance has a unique and non-overlapping mount point.
- Blobfuse2 does not support co-existance with NFS on same mount path. Behaviour in this case is undefined.

## Limitations
- In case of BlockBlob accounts, ACLs are not supported by Azure Storage so Blobfuse2 will bydefault return success for 'chmod' operation. However it will work fine for Gen2 (DataLake) accounts.


### Syslog security warning
By default, Blobfuse2 will log to syslog. The default settings will, in some cases, log relevant file paths to syslog. 
If this is sensitive information, turn off logging or set log-level to LOG_ERR.  


## License
This project is licensed under MIT.
 
## Contributing
This project welcomes contributions and suggestions.  Most contributions 
require you to agree to a Contributor License Agreement (CLA) declaring 
that you have the right to, and actually do, grant us the rights to use 
your contribution. For details, visit https://cla.microsoft.com.

When you submit a pull request, a CLA-bot will automatically determine 
whether you need to provide a CLA and decorate the PR appropriately 
(e.g., label, comment). Simply follow the instructions provided by the 
bot. You will only need to do this once across all repos using our CLA.

This project has adopted the [Microsoft Open Source Code of Conduct](https://opensource.microsoft.com/codeofconduct/).
For more information see the [Code of Conduct FAQ](https://opensource.microsoft.com/codeofconduct/faq/) or
contact [opencode@microsoft.com](mailto:opencode@microsoft.com) with any additional questions or comments.

