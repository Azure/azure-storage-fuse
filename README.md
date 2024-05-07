# BlobFuse - A Microsoft supported Azure Storage FUSE driver
## About

BlobFuse is an open source project developed to provide a virtual filesystem backed by the Azure Blob storage. It uses the [libfuse](https://github.com/libfuse/libfuse) open source library to communicate with the Linux FUSE kernel module, and implements the filesystem operations using the Azure Storage Blob REST APIs.

Blobfuse is stable, and is ***supported by Microsoft*** provided that it is used within its limits documented here. Blobfuse supports both reads and writes however, it does guarantee continuous sync on data written to storage using other APIs or other mounts of blobfuse. For data integrity it is recommended that multiple sources do not modify the same blob. Please submit an issue [here](https://github.com/azure/azure-storage-fuse/issues) for any issues/requests/questions.

## Deprecation Notice

Blobfuse v1 is no longer actively maintained and has been deprecated. It has been superseded by blobfuse v2.<br/>
If you would like to access next generation blobfuse, please refer to [blobfuse2](https://github.com/Azure/azure-storage-fuse). The binaries are available [here](https://github.com/Azure/azure-storage-fuse/releases) and the migration guide can be found [here](https://github.com/Azure/azure-storage-fuse/blob/main/MIGRATION.md).

---

## Features
- Mount a Blob storage container on Linux
- Basic file system operations such as mkdir, opendir, readdir, rmdir, open, read, create, write, close, unlink, truncate, stat, rename
- Local cache to improve subsequent access times
- Parallel download and upload features for fast access to large blobs
- Allows multiple nodes to mount the same container for read-only scenarios.
- Authenicates using storage key credentials, SaS Key, Managed Identity and SPN
- Allows ADLS Gen2 features

## Installation
You can install blobfuse from the Linux Software Repository for Microsoft products. The process is explained in the [blobfuse installation](https://github.com/Azure/azure-storage-fuse/wiki/1.-Installation) page. Alternatively, you can clone this repository, install the dependencies (fuse, libcurl, gcrypt and GnuTLS) and build from source code. See details in the [wiki](https://github.com/Azure/azure-storage-fuse/wiki/1.-Installation#build-from-source).

## Usage

### Mounting
Once you have installed blobfuse, configure your account credentials either in the template provided in blobfuse folder (connection.cfg), or in the environment variables. For brevity, let's use the following environment variables for authentication using account name and key: 

```
export AZURE_STORAGE_ACCOUNT=myaccountname
export AZURE_STORAGE_ACCESS_KEY=myaccountkey
```

Use of a high performance disk, or ramdisk for the local cache is recommended. In Azure VMs, this is the ephemeral disk which is mounted on /mnt in Ubuntu, and /mnt/resource in RHEL. Please make sure that your user has write access to this location. If not, create and `chown` to your user.

```
mkdir -p /mnt/blobfusetmp
chown <myuser> /mnt/blobfusetmp
```

Create your mountpoint (```mkdir /path/to/mount```) and mount a Blob container (must already exist) with blobfuse:
```
blobfuse /path/to/mount --container-name=mycontainer --tmp-path=/mnt/blobfusetmp
```

**NOTE**
Use absolute paths for directory paths in the command. Relative, and shortcut paths (~/) do not work.

For more information, see the [wiki](https://github.com/Azure/azure-storage-fuse/wiki/2.-Configuring-and-Running)

### Mount Options
- All options for the FUSE module is described in the [FUSE man page](http://manpages.ubuntu.com/manpages/xenial/man8/mount.fuse.8.html)
- See [mount.sh](https://github.com/Azure/azure-storage-fuse/blob/master/mount.sh) provided in this repository for a sample of most used options
- In addition to the FUSE module options; blobfuse offers following options:
	* **--tmp-path=/path/to/cache** : Configures the tmp location for the cache. Always configure the fastest disk (SSD or ramdisk) for best performance. 
	* [OPTIONAL] **--empty-dir-check=true** : Disallows remounting using a non-empty tmp-path, default is false. This option is only available after version 1.3.1.
	* [OPTIONAL] **--config-file=/path/to/connection.cfg** : Configures the path for the file where the account credentials are provided
	* [OPTIONAL] **--container-name=container** : Required if no configuration file is specified. Also set account name and key/SAS via the environment variables AZURE_STORAGE_ACCOUNT and AZURE_STORAGE_ACCESS_KEY/AZURE_STORAGE_SAS_TOKEN
	* [OPTIONAL] **--use-https=true|false** : Enables HTTPS communication with Blob storage. True by default. HTTPS must be if you are communicating to the Storage Container through OAuth.
	* [OPTIONAL] **--file-cache-timeout-in-seconds=120** : Blobs will be cached in the temp folder for this many seconds. 120 seconds by default. During this time, blobfuse will not check whether the file is up to date or not.
	* [OPTIONAL] **--log-level=LOG_WARNING** : Enables logs written to syslog. Set to LOG_WARNING by default. Allowed values are LOG_OFF|LOG_CRIT|LOG_ERR|LOG_WARNING|LOG_INFO|LOG_DEBUG
	* [OPTIONAL] **--use-attr-cache=true|false** : Enables attributes of a blob being cached. False by default. (Only available in blobfuse 1.1.0 or above)
    * [OPTIONAL] **--use-adls=true|false** : Enables blobfuse to access Azure DataLake storage account.This option is only available after version 1.3.1
    * [OPTIONAL] **--no-symlinks=true** : Turns off symlinks. Turning off symlinks will improve performance. Symlinks are on by default. This option is only available after version 1.3.1.
    * [OPTIONAL] **--max-concurrency=20** : option to override default number of concurrent storage connections, default=12, please note that blobfuse caps this at 40. So if the user specifies a value between 0 to 40, the users value will override the default. However, if the user specifies a value > 40 blobfuse will still only allow 40 concurrent connections.
    * [OPTIONAL] **--cache-size-mb=1000** : option to setup the cache-size in MB. Default will be 80% of the available memory, eviction will happen beyond that. Use this option to lower the cache size or increase it. This option is only available after version 1.3.1.
     * [OPTIONAL] **--attr_timeout=20** : The attribute timeout in seconds. Performance improvement option. It is a default fuse option to cache the attributes of a file. For further details look at the FUSE man page. The attributes of recently accessed files will be saved for the specified seconds.
     * [OPTIONAL] **--entry_timeout=20** : The entry timeout in seconds. Performance improvement option. It is a default fuse option to cache the list of files in a readdir call. For further details look at the FUSE man page. The attributes of recently accessed files will be saved for the specified seconds.
     * [OPTIONAL] **--cancel-list-on-mount-seconds=0** : A list call to the container is by default issued on mount. Setting this value to a number greater than 10 seconds will cancel this default list call. For containers with a very large number of files setting this to 10 seconds can save $$. Please be cautious to not set it larger than 60 seconds because no list call will work this time elapses.
     * [OPTIONAL] **--high-disk-threshold=90** : High disk threshold percentage. When the disk usage reaches this threshold cache eviction resumes. This parameter overrides 'file-cache-timeout-in-seconds' parameter and cached files will be removed even if it is not expired. Files which are currently in use (open) will not be evicted from cache.
     * [OPTIONAL] **--low-disk-threshold=80** : Low disk threshold percentage. Stop evicting cache, which was triggered by 'high-disk-threshold' when disk usage returns back to level specified by low-disk-threshold
     * [OPTIONAL] **--upload-modified-only=false** : Flag to turn off unnecessary uploads to storage. The default is false so any open file in write mode will get uploaded to storage. If you intend to upload only files that have their content modified set --upload-modified-only=true. Please note that if you set this to true changes to metadata only will be ignored and not pushed to storage. Setting this to true will prevent excessive billing from PUTs and improve perf. This option is only available from version 1.3.7
     * [OPTIONAL] **--cache-poll-timeout-msec=1** : Time in milliseconds in order to poll for possible expired files awaiting cache eviction. Default is 1 milisecond. Longer values may help performance.
     * [OPTIONAL] **--max-eviction=0** : Number of files to be evicted from cache at once. Default is 0 meaning no limit. All expired files will be evicted. This value may be set to a number greater than zero to batch up cache eviction in different cycles so that 100% CPU does not get consumed by cache eviction. 
     * [OPTIONAL] **--set-content-type=false** : Flag to turn on automatic 'content-type' property based on the file extension. Default is false so content-type will only be set if the uploaded file specifies it.
     * [OPTIONAL] **--ca-cert-file=/etc/ssl/certs/proxy.pem** : If external network is only available through a proxy server, this parameter should specify the proxy pem certificate otherwise blobfuse cannot connect to the storage account. This option is only available from version 1.3.7
     * [OPTIONAL] **--https-proxy=http://10.1.22.4:8080/** : If external network is only available through a proxy server, this parameter should specify the proxy server along with the port which is 8080 unless there are some deviations from normal port allocation numbers. This option is only available from version 1.3.7. Use this cli option only when proxy needs a certificate for authentication to go through and certificate is not available at standard linux path. For regular proxy connection defining standard environment variables for proxy (http_proxy/https_proxy) are enough.
     * [OPTIONAL] **--http-proxy=http://10.1.22.4:8080/** : Only used when https is turned off using --use-https=false, and if external network is only available through a proxy server, this parameter should specify the proxy server along with the port which is 8080 unless there are some deviations from normal port allocation numbers. This option is only available from version 1.3.7. Use this cli option only when proxy needs a certificate for authentication to go through and certificate is not available at standard linux path. For regular proxy connection defining standard envrionment variables for proxy (http_proxy/https_proxy) are enough.
     * [OPTIONAL] **--max-retry=26** : Maximum retry count if the failure codes are retryable. Default count is 26.  This option is only available from version 1.3.8
     * [OPTIONAL] **--max-retry-interval-in-seconds=60** : Maximum number of seconds between 2 retries, retry interval is exponentially increased but it can never exceed this value. Default naximum interval is 60 seconds.  This option is only available from version 1.3.8
     * [OPTIONAL] **--basic-remount-check=false** : Set this to true if you want to check for an already mounted status using /etc/mtab instead of calling the syscall 'setmntent'. Default is false. It is known that for AKS 1.19 and a few other linux distros, blobfuse will throw a segmentation fault error on mount, so set this to true.  This option is only available from version 1.3.8
     * [OPTIONAL] **--pre-mount-validate=false** : Set this to true to skip the cURL version check and just straight validate storage connection before mount. Default is false, so use this only if you know that you have the recent Curl version, otherwise blobfuse will hang. This option is only available from version 1.3.8
    * [OPTIONAL] **--background-download=false** : Set this true if you want file download to run in the background on open. Setting this to true will put a wait on 'read'/'write' calls till download completes. If the file is already in the local cache this switch is not evaluated. Default value is false. This option is only available from version 1.4.0
    * [OPTIONAL] **--invalidate-on-sync=false** : Set this to true if you want the particular file or directory content and attribute cache to be invalidated when the linux "sync" command is issued on a file or on a directory. 'sync' on file will remove the file from cache and invalidate its attribute cache, while 'sync' on directory will invalidate attribute cache for all files and directories under it recursively. Default is false. This option is only available from version 1.4.0
    * [OPTIONAL] **--streaming=false** : Enable read streaming of files instead of disk-caching. This option works only with read-only mount. This option is only available from version 1.4.0
    * [OPTIONAL] **--stream-cache-mb=500** : Limit total amount of data being cached in memory to conserve memory footprint of blobfuse.
    * [OPTIONAL] **--max-blocks-per-file=3** : Maximum number of blocks to be cached in memory for a read streaming.
    * [OPTIONAL] **--block-size-mb=16** : Size (in MB) of a block to be downloaded during streaming. If configured block-size is <= 64MB then block will be downloaded in a single thread. For higher block size parts of it will be downloaded in parallel. "--max-concurrency" parameter can be used to control parallelism factor. When higher block size is configured, memory usage of blobfuse will be high as these blocks are cached in memory only.
    * [OPTIONAL] **--ignore-open-flags=false** : There are certain flags in Open file-system call, which are not supported by blobfuse. If file handle is open with such flags, read/write operations fail at later stage. This CLI option allows user to supress (ignore) those flags while opening the file handle. Ignored flags are O_SYNC and O_DIRECT.
    * [OPTIONAL] **--debug-libcurl=false** : Sometimes the HTTP client fails in an unexpected way. This CLI option allows users to debug libcurl calls.

    
### Valid authentication setups:

- Account Name & Key (`authType Key`)
    - Requires the accountName, accountKey and containerName specified in the config file or command line.
    - Alternatively accountName and accountKey can be specified by the following environment values instead: AZURE_STORAGE_ACCOUNT, AZURE_STORAGE_ACCESS_KEY. 
- Account Name & SAS (`authType SAS`)
    - Requires the accountName, containerName and sasToken specified in the config file or command line.
    - Alternatively accountName and sasToken can be specified by the following environment values instead: AZURE_STORAGE_ACCOUNT, AZURE_STORAGE_SAS_TOKEN.
- Managed Identity (`authType MSI`)
    - Single assigned identity:
        - No extra parameters needed.
    - Multiple assigned identities:
        - At least one of the following for the intended identity:
            - Client ID (Use this if you are using a custom Managed Identity endpoint)
            - Object ID
            - Resource ID
        - Client ID can be specified in the config file by the key 'identityClientId' or as the environment value AZURE_STORAGE_IDENTITY_CLIENT_ID
        - Object ID can be specified in the config file by the key 'identityObjectId' or as the environment value AZURE_STORAGE_IDENTITY_OBJECT_ID 
        - Resource ID can be specified in the config file by the key 'identityResourceId' or as the environment value AZURE_STORAGE_IDENTITY_RESOURCE_ID 
    - Add Storage Blob Data Contributor roles to this identity in the Storage account.
    - MSI_ENDPOINT environment value can be used to specify a custom AAD endpoint to authenticate against
    - MSI_SECRET environment value can be used to specify a custom secret for an alternate managed identity endpoint. 
- Service Principal Name (`authType SPN`)
    - Requires servicePrincipalClientId, servicePrincipalTenantId, servicePrincipalClientSecret specified in the config file.    
    - Alternatively servicePrincipalClientId, servicePrincipalTenantId and servicePrincipalClientSecret can be specified by the following environment values instead: AZURE_STORAGE_SPN_CLIENT_ID, AZURE_STORAGE_SPN_TENANT_ID, AZURE_STORAGE_SPN_CLIENT_SECRET 
    - AZURE_STORAGE_AAD_ENDPOINT environment value can be used to specify a custom AAD endpoint to authenticate against
    - Add Storage Blob Data Contributor roles to this identity in the Storage account.

### Environment variables

- General options
    * `AZURE_STORAGE_ACCOUNT`: Specifies the storage account blobfuse targets.
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

### Config file options

- General options
    * `accountName`: Specifies the storage account blobfuse targets.
    * `blobEndpoint`: Optional parameter, defaults to `blob.core.windows.net`". This parameter should be specified for zonal dns accounts, custom domain name accounts or storage emulators.(e.g. storageaccountname.blob.region.azurestack.company.com)
    * `authType`: Overrides the currently specified auth type. Options: Key, SAS, MSI, SPN (Using this option is only available for 1.2.0 or above)
    * `logLevel`: Specifies the logging level. Use to change the logging level dynamically. Read `Logging` section for details. For allowed values refer to `--log-level` command line option.
    * `accountType`: Specifies the type of account. Either `block` or `adls` can be specified, `block` is the default value. Same can also be controlled through the command line option `--use-adls=true`. If wrong account type is supplied, certain features may not work as expected. To learn more about ADLS follow the link `(https://docs.microsoft.com/en-us/azure/storage/blobs/data-lake-storage-introduction)`

- Account key auth:
    * `accountKey`: Specifies the storage account key to use for authentication.

- SAS token auth:
    * `sasToken`: Specifies the SAS token to use for authentication.

- Managed Identity auth: (Only available for 1.2.0 or above)
    * `identityClientId`: If a MI endpoint is specified, this is the only parameter used, in the form of the `Secret` header. Only one of these three parameters are needed if multiple identities are present on the system.
    * `identityObjectId`: Only one of these three parameters are needed if multiple identities are present on the system.
    * `identityResourceId`: Only one of these three parameters are needed if multiple identities are present on the system.
    * `msiEndpoint`: Specifies a custom managed identity endpoint, as IMDS may not be available under some scenarios. Uses the `identityClientId` parameter as the `Secret` header.
    * (environment variable) `MSI_SECRET`: Specifies a custom secret for an alternate managed identity endpoint.

- Service Principal Name auth:
    * `servicePrincipalClientId`: Specifies the client ID for your application registration
    * `servicePrincipalTenantId`: Specifies the tenant ID for your application registration
    * `aadEndpoint`: Specifies a custom AAD endpoint to authenticate against
    * (environment variable) `AZURE_STORAGE_SPN_CLIENT_SECRET`: Specifies the client secret for your application registration. Please store this in the environment variable, not a config option.
- Proxy Server:
    * `caCertFile`: The absolute full name with path of the ca certificate for the proxy server. Example: /etc/ssl/certs/mitmproxy-ca-cert.pem
    * `httpsProxy`: The proxy server address. Example: http://10.1.22.4:8080/". Environment variable can be created instead of this config as export https_proxy=http://10.1.22.4:8080/.   
    * `httpProxy`:  The proxy server address when https is turned off forcing http. Example: http://10.1.22.4:8080/". Environment variable can be created instead of this config as export https_proxy=http://10.1.22.4:8080/.

## Considerations

### Design
- When blobfuse receives an 'open' request for a file, it will block and download the entire content of the blob down to the cache location specified in ```--tmp-path```
- All read and writes will go to the cache location when the file is open
- When blobfuse receives a 'close' request for the file, it will block and upload the entire content to Blob storage, and return success/failure to the 'close' call.
- If blobfuse receives another open request within ```--file-cache-timeout-in-seconds```, it will simply use the existing file in the local cache rather than downloading the file again from Blob storage.
- Files in the cache (```--tmp-path```) will be deleted after ```--file-cache-timeout-in-seconds```. Make sure to configure your tmp path  with enough space to accomodate this behavior, or set ```--file-cache-timeout-in-seconds``` to 0 to accelerate deletion of cached files.

### Performance and caching
Please take careful note of the following points, before using blobfuse:
- In order to achieve reasonable performance, blobfuse requires a temporary directory to use as a local cache. This directory will contain the full contents of any file (blob) read to or written from through blobfuse. Cached files will be purged as they age (--file-cache-timeout-in-seconds) if there are no longer open file handles to them.
  - Putting the cache directory on a ramdisk, or on an SSD (ephemeral disk on Azure) will greatly enhance performance.
  - Blobfuse currently does not manage available disk space in the tmp path. Make sure to have enough space, or reduce ```--file-cache-timeout-in-seconds``` value to accelerating purging cached files.
  - In order to delete the cache, un-mount and re-mount blobfuse.
  - Do not use the same cache directory for multiple instances of blobfuse, or for any other purpose while blobfuse is running.
- When file-cache-timeout is set to a higher value and user wants to force evict a file from cache before configured time then use 'sync/fsync' command on the file to evict it forcefully. To enable this feature use '--invalidate-on-sync=true' cli option while mounting. If 'sync/fsync' is done on dir it will invalidate attribute cache for all child path for that directory.

### If your workload is read-only:
- Because blobs get cached locally and reused for a number of seconds (--file-cache-timeout-in-seconds), if the blob on the service is modified, these changes will only be retrieved after the local cache times out, and the file is closed and re-opened.
- By setting ```--file-cache-timeout-in-seconds``` to 0, you may achieve close-to-open cache consistency like in NFS v3. This means once a file is closed, subsequent opens will see the latest changes from the Blob storage service ignoring the local cache.

### If your workload is NOT read-only:
- Do not edit, modify, or delete the contents of the temp directory while blobfuse is mounted. Doing so could cause data loss or data corruption.
- While a container is mounted, the data in the container should not be modified by any process other than blobfuse.  This includes other instances of blobfuse, running on this or other machines.  Doing so could cause data loss or data corruption.  Mounting other containers is fine.
- Modifications to files are not persisted to Azure Blob storage until the file is closed. If multiple handles are open to a file simultaneously, and data in the file has been modified, the close of each handle will flush the file to blob storage.

### Logging
- By default logging level is set to `LOG_WARNING`
- User can provide `--log-level` command line option to set logging to a desired level when blobfuse starts
- Later if user wishes to change the logging level without remounting the container then follow below steps
	- edit your config file provide `logLevel` config
	- accepted values are same as `--log-level` command line options
		e.g. logLevel LOG_DEBUG 
	- save the config file 
	- send a `SIGUSR1` to running blobfuse instance. 
		- ```$> kill -SIGUSR1 <pidof blobfuse>```
	- to go back to your default logging level (provided in command line options) 
		- remove the `logLevel` entry from config file 
		- after saving config file send `SIGUSR1` to running instance of blobfuse.
- By default logs are directed to system-configured log file e.g. /var/log/syslog or var/log/message (Depending Upon the Linux OS Family)
- If user wishes to redirect blobfuse logs to a different file, follow the below procedure
	- copy 10-blobfuse.conf to `/etc/rsyslog.d/`
	- copy blobfuse-logrotate to `/etc/logrotate.d/`
	- restart rsyslog service
	- ```$> service rsyslog restart```
	- Required files are provided along blobfuse package
	- NOTE: some of these steps may need `sudo` rights 

### Syslog security warning
By default, blobfuse will log to syslog.  The default settings will, in some cases, log relevant file paths to syslog.  If this is sensitive information, turn off logging completely.  See the [wiki](https://github.com/Azure/azure-storage-fuse/wiki/5.-Logging) for more details.

### Current Limitations
- Some file system APIs have not been implemented: readlink, link, lock and extended attribute calls.
- Not optimized for updating an existing file. blobfuse downloads the entire file to local cache to be able to modify and update the file
- When using enabling the "--use-attr-cache" feature, there may be an issue with overflow and will not clear the attribute cache until blobfuse is unmounted
- See the list of differences between POSIX and blobfuse [here](https://github.com/Azure/azure-storage-fuse/wiki/4.-Limitations-%7C-Differences-from-POSIX)
- For Gen-2 account blobs, a chmod operation will remove mask and additional principals in the ACL even if the prior ACLs included additional principals and mask.

## License
This project is licensed under MIT.
 
## Contributing

This project welcomes contributions and suggestions.  Most contributions require you to agree to a
Contributor License Agreement (CLA) declaring that you have the right to, and actually do, grant us
the rights to use your contribution. For details, visit https://cla.microsoft.com.

When you submit a pull request, a CLA-bot will automatically determine whether you need to provide
a CLA and decorate the PR appropriately (e.g., label, comment). Simply follow the instructions
provided by the bot. You will only need to do this once across all repos using our CLA.

This project has adopted the [Microsoft Open Source Code of Conduct](https://opensource.microsoft.com/codeofconduct/).
For more information see the [Code of Conduct FAQ](https://opensource.microsoft.com/codeofconduct/faq/) or
contact [opencode@microsoft.com](mailto:opencode@microsoft.com) with any additional questions or comments.

