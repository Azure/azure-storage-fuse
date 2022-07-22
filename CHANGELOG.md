## 2.0.0-preview.3 (WIP)
**Features**
- Added support for directory level SAS while mounting a subdirectory
- Added support for displaying mount space utilization based on file cache consumption (for example when doing `df`)

**Bug Fixes**
- Fixed a bug in parsing output of disk utilization summary
- Fixed a bug in parsing SAS token not having '?' as first character
- Fixed a bug in append file flow resolving data corruption
- Fixed a bug in MSI auth to send correct resource string
- Fixed a bug in OAuth token parsing when expires_on denotes numbers of seconds
- Fixed a bug in rmdir flow. Dont allow directory deletion if local cache says its empty. On container it might still have files.
## 2.0.0-preview.2 (2022-05-31)
**Performance Improvements**
- fio: Outperforms blobfuse by 10% in sequential reads

**Features**
- Added support for Debian 11 and Mariner OS
- Added support to load an external extension library
- Added support to mount without a config file
- Added support to preserve file metadata 
- Added support to preserve additional principals added to the ACL
- Added support to stream writes (without caching)
- Added support to warn customers if using a vulnerable version of Blobfuse2
- Added support to warn customers if using an older version of Blobfuse2
- Added support for . and .. in listing

**Breaking Changes**
- Changed default logging to syslog if the syslog service is running, otherwise file based 

**Bug Fixes**
- Fixed a bug that caused reads to be shorter than expected
- Fixed bug where remount on empty containers was not throwing error when the mount path has trailing '/'
- Fixed a bug where the DIRECT flag was not masked out
- Fixed a bug where files > 80GB would fail to upload when block size is not explicitly set
- Fixed a bug where the exit status was 0 despite invalid flags being passed
- Fixed bug where endpoint would be populated incorrectly when passed as environment variable
- Fixed a bug where mounting to an already mounted path would not fail 
- Fixed a bug where newly created datalake files > 256MB would fail to upload
- Fixed a bug that caused parameters explicitly set to 0 to be the default value
- Fixed a bug where `df` command showed root stats rather than Blobfuse mount file cache stats

## 2.0.0-preview.1 (2022-02-14)
- First Release

**Top Features**
- Compatibility with libfuse3, including libfuse2 compatibility for OSes that do not support libfuse3
- Service version upgrade from "2018-11-09" to "2020-04-08" (STG77) through the azure-go-sdk
- Maximum blob size in a single write 64MB -> 5000MB
- Maximum block size 100MB -> 4000MB
- Maximum file size supported 4.77TB -> 196TB
- File creation time and last access time now pulled from service 
- Passthrough directly from filesystem calls to Azure Storage APIs
- Read streaming (helpful for large files that cannot fit on disk)
- Logging to syslog or a file
- Mount a subdirectory in a container
- Mount all containers in an account
- Automatic blobfuse2 version checking and prompt for users to upgrade
- Encrypted config support 
- Present custom default permissions for files in block blob accounts
- Attribute cache invalidation based on timeout
- Set custom default blob tier while uploading files to container
- Pluggable cache eviction policies for file cache and streaming 

**User Experience**

Blobfuse2 supports a special command that will accept the v1 CLI parameters and v1 config file format, convert it to a v2 config format and mount with v2. 

Blobfuse2 exposes all supported options in a clean yaml formatted configuration file in addition to a few common options exposed as CLI parameters. In contrast, Blobfuse exposes most of its options as CLI parameters and a few others in a key-value configuration file. 

**Validation**
Extensive suite of validation run with focus on scale and compatibility.

Various improvements/parity to existing blobfuse validated like

- Listing of >1M files: Parity under the same mount conditions.
- Git clone: Outperforms blobfuse by 20% in git clone (large repo, with over 1 million objects).
- resnet-50: Parity at 32 threads.