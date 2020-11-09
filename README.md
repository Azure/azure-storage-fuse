# blobfuse
## About

blobfuse is an open source project developed to provide a virtual filesystem backed by the Azure Blob storage. It uses the [libfuse](https://github.com/libfuse/libfuse) open source library to communicate with the Linux FUSE kernel module, and implements the filesystem operations using the Azure Storage Blob REST APIs.

Blobfuse is stable, and is supported by Azure Storage given that it is used within its limits documented here. Please submit an issue [here](https://github.com/azure/azure-storage-fuse/issues) for any issues/requests/questions.

## Features
- Mount a Blob storage container on Linux
- Basic file system operations such as mkdir, opendir, readdir, rmdir, open, read, create, write, close, unlink, truncate, stat, rename
- Local cache to improve subsequent access times
- Parallel download and upload features for fast access to large blobs
- Allows multiple nodes to mount the same container for read-only scenarios.

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
    * [OPTIONAL] **--max-concurrency=12** : option to override fuse max_concurrency, default=40
    * [OPTIONAL] **--cache-size-mb=1000** : option to setup the cache-size in MB. Default will be 80% of the available memory, eviction will happen beyond that. Use this option to lower the cache size or increase it. This option is only available after version 1.3.1.
     * [OPTIONAL] **--attr_timeout=20** : The attribute timeout in seconds. Performance improvement option. It is a default fuse option. For further details look at the FUSE man page. The attributes of recently accessed files will be saved for the specified seconds.
     * [OPTIONAL] **--entry_timeout=20** : The entry timeout in seconds. Performance improvement option. It is a default fuse option. For further details look at the FUSE man page. The attributes of recently accessed files will be saved for the specified seconds.
    
    
### Valid authentication setups:

- Account Name & Key (`authType Key`)
    - Requires the accountName, accountKey and containerName specified in the config file or command line.
    - Alternatively accountName and accountKey can be specified by the following environment values instead: AZURE_STORAGE_ACCOUNT, AZURE_STORAGE_ACCESS_KEY. 
- Account Name & SAS (`authType SAS`)
    - Requires the accountName, containerName and sasToken specified in the config file or command line.
    - Alternatively accountName can be specified by the environment values AZURE_STORAGE_ACCOUNT
- Managed Service Identity (`authType MSI`)
    - Single assigned identity:
        - No extra parameters needed.
    - Multiple assigned identities:
        - At least one of the following for the intended identity:
            - Client ID (Use this if you are using a custom MSI endpoint)
            - Object ID
            - Resource ID
- Service Principal Name (`authType SPN`)
    - Requires servicePrincipalClientId, servicePrincipalTenantId, servicePrincipalClientSecret specified in the config file.    
    - Alternatively servicePrincipalClientSecret can be specified by the environment value AZURE_STORAGE_SPN_CLIENT_SECRET 
    - AZURE_STORAGE_AAD_ENDPOINT`environment value can be used to specify a custom AAD endpoint to authenticate against

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
    * `blobEndpoint`: Specifies the blob endpoint to use. Defaults to *.blob.core.windows.net, but is useful for targeting storage emulators.
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
- By default logs are directed to system-configured syslog file e.g. /var/log/syslog
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
- Some file system APIs have not been implemented: readlink, symlink, link, chmod, chown, fsync, lock and extended attribute calls.
- Not optimized for updating an existing file. blobfuse downloads the entire file to local cache to be able to modify and update the file
- When using enabling the "--use-attr-cache" feature, there may be an issue with overflow and will not clear the attribute cache until blobfuse is unmounted
- See the list of differences between POSIX and blobfuse [here](https://github.com/Azure/azure-storage-fuse/wiki/4.-Limitations-%7C-Differences-from-POSIX)

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

