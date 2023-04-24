## 2.0.3 (WIP)
**Bug Fixes**
- [#1080](https://github.com/Azure/azure-storage-fuse/issues/1080) HNS rename flow does not encode source path correctly.
- [#1081](https://github.com/Azure/azure-storage-fuse/issues/1081) Blobfuse will exit with non-zero status code if allow_other option is used but not enabled in fuse config.
- [#1079](https://github.com/Azure/azure-storage-fuse/issues/1079) Shell returns before child process mounts the container and if user tries to bind the mount it leads to inconsistent state.
- If mount fails in forked child, blobfuse2 will return back with status error code.
- [#1100](https://github.com/Azure/azure-storage-fuse/issues/1100) If content-encoding is set in blob then transport layer compression shall be disabled.
- Subdir mount is not able to list blobs correctly when virtual-directory is turned on.
- Adding support to pass down uid/gid values supplied in mount to libfuse.
- [#1102](https://github.com/Azure/azure-storage-fuse/issues/1102) Remove nanoseconds from file times as storage does not provide that  granularity.

**Features**
- Added new CLI parameter "--sync-to-flush". Once configured sync() call on file will force upload a file to storage container. As this is file handle based api, if file was not in file-cache it will first download and then upload the file. 
- Added new CLI parameter "--disable-compression". Disables content compression at transport layer. Required when content-encoding is set to 'gzip' in blob.
- Add new config parameter 'check-lmt' in file-cache. On timeout local file was deleted and redownloaded. With this config set to true last-modified-time of file in local storage and container will be compared and redownload is done if file in container has changed.

**New Config/CLI**
- "--sync-to-flush" : CLI parameter to flush the file on fsync call instead of deleting it from local storage
- "--disable-compression" : CLI parameter to disable content compression at transport layer.
- "file_cache: check-lmt:false' : Config parameter to reduce redownloads of file if they have not been modified since last download.

## 2.0.2 (2022-02-23)
**Bug Fixes**
- [#999](https://github.com/Azure/azure-storage-fuse/issues/999) Upgrade dependencies to resolve known CVEs.
- [#1002](https://github.com/Azure/azure-storage-fuse/issues/1002) In case version check fails to connect to public container, dump a log to check network and proxy settings.
- [#1006](https://github.com/Azure/azure-storage-fuse/issues/1006) Remove user and group config from logrotate file.
- [csi-driver #809](https://github.com/kubernetes-sigs/blob-csi-driver/issues/809) Fail to mount when uid/gid are provided on command line.
- [#1032](https://github.com/Azure/azure-storage-fuse/issues/1032) `mount all` CLI params parsing fix when no `config-file` and `tmp-path` is provided.
- [#1015](https://github.com/Azure/azure-storage-fuse/issues/1015) Default value of `ignore-open-flags` config parameter changed to `true`.
- [#1038](https://github.com/Azure/azure-storage-fuse/issues/1038) Changing default daemon permissions.
- [#1036](https://github.com/Azure/azure-storage-fuse/issues/1036) Fix to avoid panic when $HOME dir is not set.
- [#1036](https://github.com/Azure/azure-storage-fuse/issues/1036) Respect --default-working-dir cli param and use it as default log file path.
- If version check fails due to network issues, mount/mountall/mountv1 command used to terminate. From this release it will just emit an error log and mount will continue.
- If default work directory does not exists, mount shall create it before daemonizing.
- Default value of 'virtual-directory' is changed to true. If your data is created using Blobfuse or AzCopy pass this flag as false in mount command.

## 2.0.1 (2022-12-02)
- Copy of GA release of Blobfuse2. This release was necessary to ensure the GA version resolves ahead of the preview versions.

## 2.0.0 (2022-11-30) (GA release)
**Bug Fixes**
- [#968](https://github.com/Azure/azure-storage-fuse/issues/968) Duplicate directory listing
- [#964](https://github.com/Azure/azure-storage-fuse/issues/964) Rename for FNS account failing with source does not exists error
- [#972](https://github.com/Azure/azure-storage-fuse/issues/972) Mount all fails 
- Added support for "-o nonempty" to mount on a non-empty mount path
- [#985](https://github.com/Azure/azure-storage-fuse/pull/985) fuse required when installing blobfuse2 on a fuse3 supported system

**Breaking Changes**
- Defaults for retry policy changed. Max retries: 3 to 5, Retry delay: 3600 to 900 seconds, Max retry delay: 4 to 60 seconds

**Features**
- Added new CLI parameter "--subdirectory=" to mount only a subdirectory from given container 


## 2.0.0-preview.4 (2022-11-03)
**Breaking Changes**
- Renamed ignore-open-flag config parameter to ignore-open-flags to match CLI parameter
- Renamed health-monitor section in config file to health_monitor

**Features**
- Added support for health-monitor stop --pid=<pid> and health-monitor stop all commands
- Added support to work with virtual directories without special marker directory using the virtual-directory config option.
- Added support for object ID support for MSI credentials
- Added support for system assigned IDs for MSI credentials

**Bug Fixes**
- Auto detect auth mode based on storage config
- Auto correct endpoint for public cloud
- In case of invalid option or failure CLI to return non-zero return code
- ignore-open-flags CLI parameter is now correctly read
- [#921](https://github.com/Azure/azure-storage-fuse/issues/921): Mount using /etc/fstab fixed
- Fixed a bug where mount all required a config file
- [#942](https://github.com/Azure/azure-storage-fuse/issues/942): List api failing when backend does not return any item but only a valid token
- Fix bug in SDK trace logging which prints (MISSING) in log entries

## 2.0.0-preview.3  (2022-09-02)
**Features**
- Added support for directory level SAS while mounting a subdirectory
- Added support for displaying mount space utilization based on file cache consumption (for example when doing `df`)
- Added support for updating MD5 sum on file upload
- Added support for validating MD5 sum on download
- Added backwards compatibility support for all blobfuse v1 CLI options
- Added support to allow disabling writeback cache if a customer is opening a file with O_APPEND
- Added support to ignore append flag on open when writeback cache is on

**Bug Fixes**
- Fixed a bug in parsing output of disk utilization summary
- Fixed a bug in parsing SAS token not having '?' as first character
- Fixed a bug in append file flow resolving data corruption
- Fixed a bug in MSI auth to send correct resource string
- Fixed a bug in OAuth token parsing when expires_on denotes numbers of seconds
- Fixed a bug in rmdir flow. Dont allow directory deletion if local cache says its empty. On container it might still have files.
- Fixed a bug in background mode where auth validation would be run twice
- Fixed a bug in content type parsing for a 7z compressed file
- Fixed a bug in retry logic to retry in case of server timeout errors

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
