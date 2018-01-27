# blobfuse
## About

blobfuse is an open source project developed to provide a virtual filesystem backed by the Azure Blob storage. It uses the [libfuse](https://github.com/libfuse/libfuse) open source library to communicate with the Linux FUSE kernel module, and implements the filesystem operations using the Azure Storage Blob REST APIs.

Please note that this tool is currently in Preview, and need your feedback for improvements. Please submit an issue [here](https://github.com/azure/azure-storage-fuse/issues) for any issues/requests/questions.

## Features
- Mount a Blob storage container on Linux
- Basic file system operations such as mkdir, opendir, readdir, rmdir, open, read, create, write, close, unlink, truncate, stat, rename
- Local cache to improve subsequent access times
- Parallel download and upload features for fast access to large blobs
- Allows multiple nodes to mount the same container for read-only scenarios.

## Considerations
Please take careful note of the following points, before using blobfuse:
- In order to achieve reasonable performance, blobfuse requires a temp directory to use as a local cache. This directory will contain the full contents of any file (blob) read to or written from through blobfuse, and will be purged as they age (--file-cache-timeout-in-seconds).
  - Putting this directory on an SSD (ephemeral disk on Azure) will greatly enhance performance.
  - In order to delete the cache, un-mount and re-mount blobfuse.
  - Do not use the same temp directory for multiple instances of blobfuse, or for any other purpose while blobfuse is running.
  
### If your workload is read-only:
- Because blobs get cached locally and reused for a number of seconds (--file-cache-timeout-in-seconds), if the blob on the service is modified, these changes will only be retrieved after the local cache times out, and the file is closed and re-opened. 
- By setting --file-cache-timeout-in-seconds to 0, you may achieve close-to-open cache consistency like in NFS v3. This means once a file is closed, subsequent opens will see the latest changes from the Blob storage service ignoring the local cache.

### If your workload is not read-only:
- Do not edit, modify, or delete the contents of the temp directory while blobfuse is mounted. Doing so could cause data loss or data corruption.
- While a container is mounted, the data in the container should not be modified by any process other than blobfuse.  This includes other instances of blobfuse, running on this or other machines.  Doing so could cause data loss or data corruption.  Mounting other containers is fine.
- Modifications to files are not persisted to Azure Blob storage until the file is closed. If multiple handles are open to a file simultaneously, and data in the file has been modified, the close of each handle will flush the file to blob storage. 

## Current Limitations
- Some file system APIs have not been implemented: readlink, symlink, link, chmod, chown, fsync, lock and extended attribute calls.
- Not optimized for updating an existing file. blobfuse downloads the entire file to local cache to be able to modify and update the file
- Does not support SAS yet
- High latency compared to local filesystems. Further you are from the Azure region, higher the latency is.
- Currently does not implement data integrity checks. Always have your data backed up before using blobfuse. Use Https as a data integrity mechanism over the wire.
- Because Azure Block Blobs do not have file and directory semantics, certain directory operations may not behave entirely as expected. For example, deleting the last file in a directory may also delete the directory.

## Installation

You can install blobfuse from the Linux Software Repository for Microsoft products. The process is explained in the [blobfuse installation](https://github.com/Azure/azure-storage-fuse/wiki/Installation) page. Alternatively, you can clone this repository, install the dependencies (libfuse, libcurl and GnuTLS) and build from source code. See details in the [wiki](https://github.com/Azure/azure-storage-fuse/wiki/Installation#build-from-source).

## Usage

### Mounting
Once you have built blobfuse, configure your account credentials in the template provided in blobfuse folder (connection.cfg).

Here is the format for connection.cfg:
```
accountName <account-name-here> 
accountKey <account-key-here> 
containerName <container-name-here>
```

Alternatively provide the account name and key in the environment variables AZURE_STORAGE_ACCOUNT and AZURE_STORAGE_ACCESS_KEY, and set --container-name while mounting.

By default, blobfuse will use the ephemeral disks in Azure VMs as the local cache (/mnt/blobfusetmp). Please make sure that your user has write access to this location. If not, create and `chown` to your user.

```
mkdir -p /mnt/blobfusetmp
chown <myuser> /mnt/blobfusetmp
```

Now you can mount using the provided mount script (mount.sh):
```
./mount.sh </path/to/mount>
```

### Mount Options
- You can modify the default FUSE options in mount.sh file. All options for FUSE is described in the [FUSE man page](http://manpages.ubuntu.com/manpages/xenial/man8/mount.fuse.8.html)
- In addition to the FUSE kernel module options; blobfuse offers following options:
	* --tmp-path=/path/to/cache : Configures the tmp location for the cache. Always configure the fastest disk (SSD or ramdisk) for best performance. 
	* --config-path=/path/to/connection.cfg : Configures the path for the file where the account credentials are provided
	* [OPTIONAL] --container-name=container : Required if no configuration file is specified. Also set account name and key via the environment variables AZURE_STORAGE_ACCOUNT and AZURE_STORAGE_ACCESS_KEY
	* [OPTIONAL] --use-https=true/false : Enables HTTPS communication with Blob storage. True by default. 
	* [OPTIONAL] --file-cache-timeout-in-seconds=120 : Blobs will be cached in the temp folder for this many seconds. 120 seconds by default. During this time, blobfuse will not check whether the file is up to date or not.
	

### Notes
- blobfuse is currently in Preview. Expect significant performance (and stability) improvements in the upcoming months.
- Performance is varied with your setup. Specifically latency plays an important role in filesystems. Expect better performance within an Azure region (Azure VM and Azure Storage account in the same region)
- Rename operation on directories is not an atomic operation. blobfuse iterates over the entire directory and issue [Copy Blob API](https://docs.microsoft.com/en-us/rest/api/storageservices/copy-blob) one by one, hence expect poor performance for rename operations on directories.
- Files you open in blobfuse mount get stored in the local cache, and only persisted to Blob storage after a 'close' is called. 

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

