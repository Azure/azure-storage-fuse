# Blobfuse2 - A Microsoft supported Azure Storage FUSE driver
## About
Blobfuse2 is an open source project developed to provide a virtual filesystem backed by the Azure Storage. It uses the libfuse open source library (fuse3) to communicate with the Linux FUSE kernel module, and implements the filesystem operations using the Azure Storage REST APIs.
This is the next generation [blobfuse](https://github.com/Azure/azure-storage-fuse).

## About Data Consistency and Concurrency
Blobfuse2 is stable and ***supported by Microsoft*** when used within its [documented limits](#un-supported-file-system-operations). Blobfuse2 supports high-performance reads and writes with strong consistency; however, it is recommended that multiple clients do not modify the same blob/file simultaneously to ensure data integrity. Blobfuse2 does not guarantee continuous synchronization of data written to the same blob/file using multiple clients or across multiple mounts of Blobfuse2 concurrently. If you modify an existing blob/file with another client while also reading that object, Blobfuse2 will not return the most up-to-date data. To ensure your reads see the newest blob/file data, disable all forms of caching at kernel (using `direct-io`) as well as at Blobfuse2 level, and then re-open the blob/file.

Please submit an issue [here](https://github.com/azure/azure-storage-fuse/issues) for any issues/feature requests/questions.

[This](#config-guide) section will help you choose the correct config for Blobfuse2.

##  NOTICE
- Due to known data consistency issues when using Blobfuse2 in `block-cache` mode,  it is strongly recommended that all Blobfuse2 installations be upgraded to version 2.3.2. For more information, see [this](https://github.com/Azure/azure-storage-fuse/wiki/Blobfuse2-Known-issues).
- Login via Managed Identify is supported with Object-ID for all versions of Blobfuse except 2.3.0 and 2.3.2.To use Object-ID for these two versions, use Azure CLI or utilize Application/Client-ID or Resource ID based authentication.
- `streaming` mode is being deprecated. This is the older option and is replaced by streaming with `block-cache` mode which is the more performant streaming option.

## Limitations in Block Cache
- Concurrent write operations on the same file using multiple handles is not checked for data consistency and may lead to incorrect data being written.
- A read operation on a file that is being written to simultaneously by another process or handle will not return the most up-to-date data.
- When copying files with trailing null bytes using `cp` utility to a Blobfuse2 mounted path, use `--sparse=never` parameter to avoid data being trimmed. For example, `cp --sparse=never src dest`.
- In write operations, data written is persisted (or committed) to the Azure Storage container only when close, sync or flush operations are called by user application.
- Files cannot be modified if they were originally created with block-size different than the one configured.

## Recommendations in Block Cache
- User applications must check the returned code (success/failure) for filesystem calls like read, write, close, flush, etc. If error is returned, the application must abort their respective operation.
- User applications must ensure that there is only one writer at a time for a given file.
- When dealing with very large files (in TiB), the block-size must be configured accordingly. Azure Storage supports only [50,000 blocks](https://learn.microsoft.com/en-us/rest/api/storageservices/put-block-list?tabs=microsoft-entra-id#remarks) per blob.
  
## Blobfuse2 Benchmarks
[This](https://azure.github.io/azure-storage-fuse/) page lists various benchmarking results for HNS and FNS Storage account.

## Supported Platforms
Visit [this](https://github.com/Azure/azure-storage-fuse/wiki/Blobfuse2-Supported-Platforms) page to see list of supported linux distros.

## Features
- Mount an Azure storage blob container or datalake file system on Linux.
- Basic file system operations such as mkdir, opendir, readdir, rmdir, open, 
   read, create, write, close, unlink, truncate, stat, rename
- Local caching to improve subsequent access times
- Block-Cache to support reading AND writing large files 
- Parallel downloads and uploads to improve access time for large files
- Multiple mounts to the same container for read-only workloads

## _New BlobFuse2 Health Monitor_
One of the biggest BlobFuse2 features is our brand new health monitor. It allows customers gain more insight into how their BlobFuse2 instance is behaving with the rest of their machine. Visit [here](https://github.com/Azure/azure-storage-fuse/blob/main/tools/health-monitor/README.md) to set it up.

## Distinctive features compared to blobfuse (v1.x)
- Blobfuse2 is fuse3 compatible (other than Ubuntu-18 and Debian-9, where it still runs with fuse2)
- Support for higher service version offering latest and greatest of azure storage features (supported by azure go-sdk)
- Set blob tier while uploading the data to storage
- Attribute cache invalidation based on timeout
- For flat namespace accounts, user can configure default permissions for files and folders
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
- Set MD5 sum of a blob while uploading
- Validate MD5 sum on download and fail file open on mismatch
- Large file writing through write Block-Cache
- Blob filter to view only files matching given criteria for read-only mount

 ## Blobfuse2 performance compared to blobfuse(v1.x.x)
- 'git clone' operation is 25% faster (tested with vscode repo cloning)
- ResNet50 image classification job is 7-8% faster (tested with 1.3 million images)
- Regular file uploads are 10% faster
- Verified listing of 1-Billion files in a directory (which v1.x does not support)


## Download Blobfuse2
You can install Blobfuse2 by cloning this repository. In the workspace root execute below commands to build the binary.

- sudo apt install fuse3 libfuse3-dev gcc
- go build -o blobfuse2


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
* `gen-config` -  Auto generate recommended blobfuse2 config file.

## Find help from your command prompt
To see a list of commands, type `blobfuse2 -h` and then press the ENTER key.
To learn about a specific command, just include the name of the command (For example: `blobfuse2 mount -h`).

## Usage
- Mount with blobfuse2
    * blobfuse2 mount \<mount path\> --config-file=\<config file\>
- Mount blobfuse2 using legacy blobfuse config and cli parameters
    * blobfuse2 mountv1 \<blobfuse mount cli with options\>
- Mount all containers in your storage account
    * blobfuse2 mount all \<mount path\> --config-file=\<config file\>
- List all mount instances of blobfuse2
    * blobfuse2 mount list
- Unmount blobfuse2
    * sudo fusermount3 -u \<mount path\>
- Unmount all blobfuse2 instances
    * blobfuse2 unmount all 
- Auto generate config file
    * blobfuse2 gen-config --tmp-path=\<local cache path\> --o \<path to save generated config\>

<!---TODO Add Usage for mount, unmount, etc--->
## CLI parameters
- Note: Blobfuse2 accepts all CLI parameters that Blobfuse does, but may ignore parameters that are no longer applicable. 
- General options
    * `--config-file=<PATH>`: The path to the config file.
    * `--log-level=<LOG_*>`: The level of logs to capture.
    * `--log-file-path=<PATH>`: The path for the log file.
    * `--foreground=true`: Mounts the system in foreground mode.
    * `--read-only=true`: Mount container in read-only mode.
    * `--default-working-dir`: The default working directory to store log files and other blobfuse2 related information.
    * `--disable-version-check=true`: Disable the blobfuse2 version check.
    * `--secure-config=true` : Config file is encrypted suing 'blobfuse2 secure` command.
    * `--passphrase=<STRING>` : Passphrase used to encrypt/decrypt config file.
    * `--wait-for-mount=<TIMEOUT IN SECONDS>` : Let parent process wait for given timeout before exit to ensure child has started. 
    * `--block-cache` : To enable block-cache instead of file-cache. This works only when mounted without any config file.
    * `--lazy-write` : To enable async close file handle call and schedule the upload in background.
    * `--filter=<STRING>`: Enable blob filters for read-only mount to restrict the view on what all blobs user can see or read.
    * `--preload`: Enable preload for read-only mount to start downloading all blobs from container when mount succeeds.
- Attribute cache options
    * `--attr-cache-timeout=<TIMEOUT IN SECONDS>`: The timeout for the attribute cache entries.
    * `--no-symlinks=false`: By default symlinks will be supported and the performance overhead, that earlier existed, has been resolved.
- Storage options
    * `--container-name=<CONTAINER NAME>`: The container to mount.
    * `--cancel-list-on-mount-seconds=<TIMEOUT IN SECONDS>`: Time for which list calls will be blocked after mount. ( prevent billing charges on mounting)
    * `--virtual-directory=true` : Support virtual directories without existence of a special marker blob for block blob account.
    * `--subdirectory=<path>` : Subdirectory to mount instead of entire container.
    * `--disable-compression:false` : Disable content encoding negotiation with server. If blobs have 'content-encoding' set to 'gzip' then turn on this flag.
    * `--use-adls=false` : Specify configured storage account is HNS enabled or not. This must be turned on when HNS enabled account is mounted.
    * `--cpk-enabled=true`: Allows mounting containers with cpk. Use config file or env variables to set cpk encryption key and cpk encryption key sha.
- File cache options
    * `--file-cache-timeout=<TIMEOUT IN SECONDS>`: Timeout for which file is cached on local system.
    * `--tmp-path=<PATH>`: The path to the file cache.
    * `--cache-size-mb=<SIZE IN MB>`: Amount of disk cache that can be used by blobfuse. Default - 80% of free disk space.
    * `--high-disk-threshold=<PERCENTAGE>`: If local cache usage exceeds this, start early eviction of files from cache.
    * `--low-disk-threshold=<PERCENTAGE>`: If local cache usage comes below this threshold then stop early eviction.
    * `--sync-to-flush=false` : Sync call will force upload a file to storage container if this is set to true, otherwise it just evicts file from local cache.
- Block-Cache options
    * `--block-cache-block-size=<SIZE IN MB>`: Size of a block to be downloaded as a unit.
    * `--block-cache-pool-size=<SIZE IN MB>`: Size of pool to be used for caching. This limits total memory used by block-cache. Default - 80% of free memory available.
    * `--block-cache-path=<PATH>`: Path where downloaded blocks will be persisted. Not providing this parameter will disable the disk caching.
    * `--block-cache-disk-size=<SIZE IN MB>`: Disk space to be used for caching. Default - 80% of free disk space.
    * `--block-cache-disk-timeout=<seconds>`: Timeout for which disk cache is valid.
    * `--block-cache-prefetch=<Number of blocks>`: Number of blocks to prefetch at max when sequential reads are in progress. Default - 2 times number of CPU cores.
    * `--block-cache-parallelism=<count>`: Number of parallel threads doing upload/download operation. Default - 3 times number of CPU cores.
    * `--block-cache-prefetch-on-open=true`: Start prefetching on open system call instead of waiting for first read. Enhances perf if file is read sequentially from offset 0.
    * `--block-cache-strong-consistency=true`: Enable strong data consistency checks in block-cache. This will increase load on your CPU and may introduce some latency. 
    This will need support of `xattr` on your system. Kindly install the feature manually before using this cli parameter.
- Fuse options
    * `--attr-timeout=<TIMEOUT IN SECONDS>`: Time the kernel can cache inode attributes.
    * `--entry-timeout=<TIMEOUT IN SECONDS>`: Time the kernel can cache directory listing.
    * `--negative-timeout=<TIMEOUT IN SECONDS>`: Time the kernel can cache non-existance of file or directory.
    * `--allow-other`: Allow other users to have access this mount point.
    * `--disable-writeback-cache=true`: Disallow libfuse to buffer write requests if you must strictly open files in O_WRONLY or O_APPEND mode.
    * `--ignore-open-flags=true`: Ignore the append and write only flag since O_APPEND and O_WRONLY is not supported with writeback caching.


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
    * `AZURE_STORAGE_AUTH_RESOURCE` : Scope to be used while requesting for token.
- Workload Identity auth:
    * `AZURE_STORAGE_SPN_CLIENT_ID`: Specifies the clientid of the MI assigned to the storage account | clientid of the MI assigned as subject field on a Federated Identity Credential (FIC) on the App Registration
    * `AZURE_STORAGE_SPN_TENANT_ID`: Specifies the tenant ID for your storage account
    * `AZURE_STORAGE_IDENTITY_CLIENT_ID`: Specifies the application (client) ID of the App Registration or SPN
    * `AZURE_STORAGE_AUTH_RESOURCE` : Scope to be used while requesting for token / MI Audience.
            
            Public Cloud: api://AzureADTokenExchange  (Default)

            US Gov Cloud: api://AzureADTokenExchangeUSGov

            China Cloud operated by 21Vianet: api://AzureADTokenExchangeChina

- Proxy Server:
    * `http_proxy`: The proxy server address. Example: `10.1.22.4:8080`.    
    * `https_proxy`: The proxy server address when https is turned off forcing http. Example: `10.1.22.4:8080`.
- CPK options: 
    * `AZURE_STORAGE_CPK_ENCRYPTION_KEY`: Customer provided base64-encoded AES-256 encryption key value.
    * `AZURE_STORAGE_CPK_ENCRYPTION_KEY_SHA256`: Base64-encoded SHA256 of the cpk encryption key.
- Custom component options:
    * `BLOBFUSE_PLUGIN_PATH`: Specifies plugin file path as a colon-separated list of `.so` files. Example BLOBFUSE_PLUGIN_PATH="/path/to/plugin1.so:/path/to/plugin2.so".


## Config Guide
Below diagrams guide you to choose right configuration for your workloads.

- Choose right Auth mode
<br/><br/>
![alt text](./guide/AuthModeHelper.png?raw=true "Auth Mode Selection Guide")
<br/><br/>
- Choose right caching for Read-Only workloads
<br/><br/>
![alt text](./guide/CacheModeForReadOnlyWorkloads.png?raw=true "Cache Mode Selection Guide For Read-Only Workloads")
<br/><br/>
- Choose right caching for Read-Write workloads
<br/><br/>
![alt text](./guide/CacheModeForReadWriteWorkloads.png?raw=true "Cache Mode Selection Guide For Read-Only Workloads")
<br/><br/>
- Choose right block-cache configuration
<br/><br/>
![alt text](./guide/BlockCacheConfig.png?raw=true "Block-Cache Configuration")
<br/><br/>
- Choose right file-cache configuration
<br/><br/>
![alt text](./guide/FileCacheConfig.png?raw=true "Block-Cache Configuration")
<br/><br/>
- [Sample File Cache Config](./sampleFileCacheConfig.yaml)
- [Sample Block-Cache Config](./sampleBlockCacheConfig.yaml)
- [All Config options](./setup/baseConfig.yaml) 

## Preload

By default, Blobfuse responds to kernel file-system calls. When user application opens a file, it will be downloaded to local cache and will remain there until user closes the file. In case of model training (AI/ML) workflows, if user application is going to process all the data that exists on the storage container, then one by one application will execute open file-system calls and it has to wait till the file download completes. To accelerate such workflows Blobfuse now has a capability to start downloading entire container/sub-directory, that user has mounted to local-cache as soon as mount succeeds, instead of waiting for the user application to execute file open call. Combining this feature with below mentioned blob-filters, user can control what files shall be part of the download set. This helps when you have structured data, and your application does not need access of the entire data but only a subset of it.


Below are some points that user shall be aware while using preload:
- Mount will be done in 'read-only' mode even if user has not provided it explicitly.
- Local cache path provided should have sufficient space to download the contents from storage. If local storage is full, preload will stop downloading the data.
- If user application executes a file system open call on a certain file, call will wait until the file download completes. If the file was not under download, it will be downloaded on priority.
- Files once downloaded will not be evicted from local cache.
- Blobfuse will not detect any changes on the container (files modified in the storage container), once preload completes. All reads will be served from local cache only. For any new files created on the container, open file-system call for the file will start download of that file.
 

## Blob Filter
- In case of read-only mount, user can configure a filter to restrict what all blobs a mount can see or operate on.
- Blobfuse supports filters based on
    - Name
    - Size
    - Last modified time
    - File extension
- Blob Name based filter
    - Supported operations are "=" and "!="
    - Name shall be a valid regex expression
    - e.g. ```filter=name=^mine[0-1]\\d{3}.*```
- Size based filter
    - Supported operations are "<=", ">=", "!=", "<", ">" and "="
    - Size shall be provided in bytes
    - e.g. ```filter=size > 1000```
- Last Modified Date based filter
    - Supported operations are "<=", ">=", "<", ">" and "="
    - Date shall be provided in RFC1123 Format e.g. "Mon, 24 Jan 1982 13:00:00 UTC"
    - e.g. ```filter=modtime>Mon, 24 Jan 1982 13:00:00 UTC```
- File Extension based filter
    - Supported operations are "=" and "!="
    - Extension can be supplied as string. Do not include "." in the filter
    - e.g. ```--filter=format=pdf```
- Multiple filters can be combined using '&&' and '||' operator as well, however precedence using '()' is not supported yet.
    - e.g. ```--filter=name=^testfil.* && size>130000000```

## Using Private Endpoints with HNS-Enabled Storage Accounts

When accessing an HNS (Hierarchical Namespace) enabled Azure Storage account behind private endpoints, it is crucial to create **two separate private endpoints** to ensure proper connectivity:

1. **Private Endpoint for DFS**  
   - Target: `privatelink.dfs.core.windows.net`  
   - This endpoint is necessary for accessing the Data Lake Storage Gen2 (HNS) functionality.

2. **Private Endpoint for Blob**  
   - Target: `privatelink.blob.core.windows.net`  
   - This endpoint is necessary for accessing Blob Storage operations.

### Why Both Endpoints Are Required

HNS-enabled storage accounts utilize separate endpoints for Blob and DFS operations:
- The DFS endpoint (`dfs.core.windows.net`) is used for namespace-related operations like directory and file management.
- The Blob endpoint (`blob.core.windows.net`) is used for operations like streaming data to and from blobs.

## Frequently Asked Questions
- How do I generate a SAS with permissions for rename?
az cli has a command to generate a sas token. Open a command prompt and make sure you are logged in to az cli. Run the following command and the sas token will be displayed in the command prompt.
az storage container generate-sas --account-name <account name ex:myadlsaccount> --account-key <accountKey> -n <container name> --permissions dlrwac --start <today's date ex: 2021-03-26> --expiry <date greater than the current time ex:2021-03-28>
- Why do I get EINVAL on opening a file with WRONLY or APPEND flags?
To improve performance, Blobfuse2 by default enables writeback caching, which can produce unexpected behavior for files opened with WRONLY or APPEND flags, so Blobfuse2 returns EINVAL on open of a file with those flags. Either use disable-writeback-caching to turn off writeback caching (can potentially result in degraded performance) or ignore-open-flags (replace WRONLY with RDWR and ignore APPEND) based on your workload. 
- How to mount blobfuse2 inside a container?
Refer to 'docker' folder in this repo. It contains a sample 'Dockerfile'. If you wish to create your own container image, try 'buildandruncontainer.sh' script, it will create a container image and launch the container using current environment variables holding your storage account credentials.
- Why am I not able to see the updated contents of file(s), which were updated through means other than Blobfuse2 mount?
If your use-case involves updating/uploading file(s) through other means and you wish to see the updated contents on Blobfuse2 mount then you need to disable kernel page-cache. `-o direct_io` CLI parameter is the option you need to use while mounting. Along with this, set `file-cache-timeout=0` and all other libfuse caching parameters should also be set to 0. User shall be aware that disabling kernel cache can result into more calls to Azure Storage which will have cost and performance implications. 

## Un-Supported File system operations
- mkfifo : fifo creation is not supported by blobfuse2 and this will result in "function not implemented" error
- chown  : Change of ownership is not supported by Azure Storage hence Blobfuse2 does not support this.
- Creation of device files or pipes is not supported by Blobfuse2.
- Blobfuse2 does not support extended-attributes (x-attrs) operations
- Blobfuse2 does not support lseek() operation on directory handles. No error is thrown but it will not work as expected.

## Un-Supported Scenarios
- Blobfuse2 does not support overlapping mount paths. While running multiple instances of Blobfuse2 make sure each instance has a unique and non-overlapping mount point.
- Blobfuse2 does not support co-existance with NFS on same mount path. Behaviour in this case is undefined.
- For block blob accounts, where data is uploaded through other means, Blobfuse2 expects special directory marker files to exist in container. In absence of this
  few file operations might not work. For e.g. if you have a blob 'A/B/c.txt' then special marker files shall exists for 'A' and 'A/B', otherwise opening of 'A/B/c.txt' will fail.
  Once a 'ls' operation is done on these directories 'A' and 'A/B' you will be able to open 'A/B/c.txt' as well. Possible workaround to resolve this from your container is to either

  create the directory marker files manually through portal or run 'mkdir' command for 'A' and 'A/B' from blobfuse. Refer [me](https://github.com/Azure/azure-storage-fuse/issues/866) 
  for details on this.

## Limitations
- In case of BlockBlob accounts, ACLs are not supported by Azure Storage so Blobfuse2 will by default return success for 'chmod' operation. However it will work fine for Gen2 (DataLake) accounts.
- When Blobfuse2 is mounted on a container, SYS_ADMIN privileges are required for it to interact with the fuse driver. If container is created without the privilege, mount will fail. Sample command to spawn a docker container is 

    `docker run -it --rm --cap-add=SYS_ADMIN --device=/dev/fuse --security-opt apparmor:unconfined <environment variables> <docker image>`
- In case of `mount all` system may limit on number of containers you can mount in parallel (when you go above 100 containers). To increase this system limit use below command
    `echo 256 | sudo tee /proc/sys/fs/inotify/max_user_instances`
- Refer [this](#limitations-in-block-cache) for block-cache limitations.

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

