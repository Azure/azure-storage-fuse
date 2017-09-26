# Blobfuse
## About

Blobfuse is an open source project developed to provide a virtual filesystem backed by the Azure Blob storage. It uses the [libfuse](https://github.com/libfuse/libfuse) open source library to communicate with the Linux FUSE kernel module, and implements the filesystem operations using the Azure Storage Blob REST APIs.

Please note that this tool is currently in Preview, and need your feedback for improvements. Please submit an issue [here](https://github.com/azure/azure-storage-fuse/issues) for any issues/requests/questions.

## Features
- Mount a Blob storage container on Linux
- Basic file system operations such as mkdir, opendir, readdir, rmdir, open, read, create, write, close, unlink, truncate, stat, rename
- Local cache to improve subsequent access times
- Parallel download and upload features to fast access large blobs
- Allows multiple nodes to mount the same container (Note however, there is no sync between nodes on writes to blob storage. See limitations.)

## Current Limitations
- Some file system APIs have not been implemented: readlink, symlink, link, chmod, chown, fsync, lock and extended attribute calls
- Not optimized for updating an existing file. Blobfuse downloads the entire file to local cache to be able to modify and update the file
- Does not support SAS yet
- High latency compared to local filesystems. Further you are from the Azure region, higher the latency is.
- Currently does not implement data integrity checks. Always have your data backed up before using Blobfuse. Use Https as a data integrity mechanism over the wire.

## Installation

In order to invoke Blobfuse, you will need to install the dependencies (libfuse, libcurl, GnuTLS and cmake), clone this repository and build. The process is explained in the [Blobfuse installation](https://github.com/Azure/azure-storage-fuse/wiki/Installation) page.

## Usage

### Mounting
Once you have built Blobfuse, configure your account credentials in the template provided in blobfuse folder (connection.cfg).

Here is the format for connection.cfg:
```
accountName <account-name-here> 
accountKey <account-key-here> 
containerName <container-name-here>
```

By default, Blobfuse will use the ephemeral disks in Azure VMs as the local cache (/mnt/blobfusetmp). Please make sure that your user has write access to this location. If not, create and `chown` to your user.

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
- In addition to the FUSE kernel module options; Blobfuse offers following options:
	* --configPath=/path/to/connection.cfg : Configures the path for the file where the account credentials are provided
	* --tmpPath=/path/to/cache : Configures the tmp location for the cache. Always configure the fastest disk (SSD) for best performance. Note that the files in this directory are not purged automatically.

### Notes
- Blobfuse is currently in Preview. Expect significant performance (and stability) improvements in the upcoming months.
- Performance is varied with your setup. Specifically latency plays an important role in filesystems. Expect better performance within an Azure region (Azure VM and Azure Storage account in the same region)
- Rename operation on directories is not an atomic operation. Blobfuse iterates over the entire directory and issue [Copy Blob API](https://docs.microsoft.com/en-us/rest/api/storageservices/copy-blob) one by one, hence expect poor performance for rename operations on directories.
- Files you open in Blobfuse mount get stored in the local cache, and only persisted to Blob storage after a 'close' is called. 

## License
 
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

