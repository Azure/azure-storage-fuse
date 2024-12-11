## 2.4.1 (Unreleased)
**Bug Fixes**
- Create block pool only in the child process.
- Prevent the block cache to truncate the file size to zero when the file is opened in O_WRONLY mode when writebackcache is disabled.

## 2.4.0 (Unreleased)
**Features**
- Added 'gen-config' command to auto generate the recommended blobfuse2 config file based on computing resources and memory available on the node. Command details can be found with `blobfuse2 gen-config --help`.
- Added option to set Entry cache to hold directory listing results in cache for a given timeout. This will reduce REST calls going to storage and enables faster access across multiple applications that use Blobfuse on the same node.

**Bug Fixes**
- [#1426](https://github.com/Azure/azure-storage-fuse/issues/1426) Read panic in block-cache due to boundary conditions.
- Do not allow mount path and temp-cache path to be same when using block-cache.
- Do not allow to mount with non-empty directory provided for disk persistence in block-cache.
- Rename file was calling an additional getProperties call.
- Delete empty directories from local cache on rmdir operation.
- [#1547](https://github.com/Azure/azure-storage-fuse/issues/1547) Truncate logic of file cache is modified to prevent downloading and uploading the entire file.
- Updating a file via Blobfuse2 was resetting the ACLs and Permissions applied to file in Datalake.

**Other Changes**
- `Stream` option automatically replaced with "Stream with Block-cache" internally for optimized performance.
- Login via Managed Identify is supported with Object-ID for all versions of blobfuse except 2.3.0 and 2.3.2.To use Object-ID for these two versions, use AzCLI or utilize Application/Client-ID or Resource ID base authentication..
- Version check is now moved to a static website hosted on a public container.

## 2.3.2 (2024-09-03)
**Bug Fixes**
- Fixed the case where file creation using SAS on HNS accounts was returning back wrong error code.
- [#1402](https://github.com/Azure/azure-storage-fuse/issues/1402) Fixed proxy URL parsing.
- In flush operation, the blocks will be committed only if the handle is dirty.
- Fixed an issue in File-Cache that caused upload to fail due to insufficient permissions.

**Data Integrity Fixes**
- Fixed block-cache read of small files in direct-io mode, where file size is not multiple of kernel buffer size.
- Fixed race condition in block-cache random write flow where a block is being uploaded and written to in parallel.
- Fixed issue in block-cache random read/write flow where a uncommitted block, which is deleted from local cache, is reused.
- Sparse file data integrity issues fixed.

**Other Changes**
- LFU policy in file cache has been removed.
- Default values, if not assigned in config, for the following parameters in block-cache are calculated as follows:
    - Memory preallocated for Block-Cache is 80% of free memory
    - Disk Cache Size is 80% of free disk space
    - Prefetch is 2 times number of CPU cores
    - Parallelism is 3 times the number of CPU cores
- Default value of Disk Cache Size in File Cache is 80% of free disk space

## 2.3.0 (2024-05-16)
**Bug Fixes**
- For fuse minor version check rely on the fusermount3 command output rather then one exposed from fuse_common.
- Fixed large number of threads from TLRU causing crash during disk eviction in block-cache.
- Fixed issue where get attributes was failing for directories in blob accounts when CPK flag was enabled.

**Features**
- Added support for authentication using Azure CLI.

**Other Changes**
- Added support in
    - Ubuntu 24.04 (x86_64 and ARM64)
    - Rocky Linux 8 and 9
    - Alma Linux 8 and 9
- Added support for FIPS based Linux systems.
- Updated dependencies to address security vulnerabilities.

## 2.3.0~preview.1 (2024-04-04)
**Bug Fixes**
- [#1057](https://github.com/Azure/azure-storage-fuse/issues/1057) Fixed the issue where user-assigned identity is not used to authenticate when system-assigned identity is enabled.
- Listing blobs is now supported for blob names that contain characters that aren't valid in XML (U+FFFE or U+FFFF).
- [#1359](https://github.com/Azure/azure-storage-fuse/issues/1359), [#1368](https://github.com/Azure/azure-storage-fuse/issues/1368) Fixed RHEL 8.6 mount failure

**Features**
- Migrated to the latest [azblob SDK](https://pkg.go.dev/github.com/Azure/azure-sdk-for-go/sdk/storage/azblob).
- Migrated to the latest [azdatalake SDK](https://pkg.go.dev/github.com/Azure/azure-sdk-for-go/sdk/storage/azdatalake).
- Migrated from deprecated ADAL to MSAL through the latest [azidentity SDK](https://pkg.go.dev/github.com/Azure/azure-sdk-for-go/sdk/azidentity).
- Added support for uploading blobs in cold and premium tier.
- Support CPK for adls storage accounts.
- Lazy-write support for async flush and close file call. Actual upload will be scheduled in background when this feature is enabled.

## 2.2.1 (2024-02-28)
**Bug Fixes**
- Fixed panic while truncating a file to a very large size.
- Fixed block-cache panic on flush of a file which has no active changeset
- Fixed block-cache panic on renaming a file and then flushing older handle
- Fixed block-cache flush resulting in invalid-block-list error

## 2.2.0 (2024-01-24)
**Bug Fixes**
- Invalidate attribute cache entry on `PathAlreadyExists` error in create directory operation.
- When `$HOME` environment variable is not present, use the current directory.
- Fixed mount failure on nonempty mount path for fuse3.

**Features**
- Support CPK for block storage accounts.
- Added support to write files using block-cache
    - Optimized for sequential writing
    - Editing/Appending existing files works only if files were originally created using block-cache with the same block size

## 2.1.2 (2023-11-17)
**Bug Fixes**
- [#1243](https://github.com/Azure/azure-storage-fuse/issues/1243) Fixed issue where symlink was not working for ADLS accounts.
- [#1259](https://github.com/Azure/azure-storage-fuse/issues/1259) sync-to-flush will force upload the file contents to container.
- [#1285](https://github.com/Azure/azure-storage-fuse/issues/1285) Rename directory fails for blob accounts when marker blob does not exist for source directory.
- [#1284](https://github.com/Azure/azure-storage-fuse/issues/1284) Fixed truncate behaviour for streaming write.
- [#1142](https://github.com/Azure/azure-storage-fuse/issues/1142) Fixed truncate behaviour for streaming write.
- Randomize token refresh interval for MSI and SPN to support multi-instance deployment.

## 2.1.1 (2023-10-31)
**Bug Fixes**
- [#1237](https://github.com/Azure/azure-storage-fuse/issues/1237) Fixed the case sensitivity of content type for file extensions.
- [#1230](https://github.com/Azure/azure-storage-fuse/issues/1230) Disable deletion of files from local-cache on sync. Use `--ignore-sync` cli option to enable this.
- Rename API for HNS account now works with user delegation SAS
- SAS token is redacted in logs for rename api over dfs endpoint
- Allow user to configure custom AAD endpoint using MSI_ENPOINT environment variable for MSI based authentication
- Fail mount if block-cache prefetch count exceeds the defined memory limits.
- uid/gid supplied as CLI parameters will be shown as actual user/group while listing files.
- Corrected handling of `umask` libfuse option.

**Optimizations**
- Optimized file-cache to skip download when O_TRUNC flag is provided in open call.
- Refresh token 5 minutes before the expiry instead of last 10 seconds.

**Features**
- Sync in stream mode will force upload the file to storage container.
- Fail `Open` and `Write` operations with file-cache if the file size exceeds the high threshold set with local cache limits.

## 2.1.0 (2023-08-31)
**Features**
- Added support for ARM64 architecture.
- Block cache component added to support faster serial reads of large files with prefetching of blocks
    - As of now only one file single threaded read is faster
    - Only read-only mounts will support block-cache
- Adaptive prefetching to support random reads without incurring extra network cost
- Block cache with disk backup to reduce network cost if same blocks are read again
- On AML compute cluster MSI authentication is now supported (this will use the identity assigned to compute cluster) 

**Bug Fixes**
- Fix to evict the destination file from local cache post rename file operation.
- If `$PATH` is not populated correctly, find out correct path for `du` command.
- Disable `kernel_cache` and `writeback_cache` when `direct_io` is set.
- Fix FUSE CLI parameter parsing, where CLI overrides parameters provided in config file.
- [#1226](https://github.com/Azure/azure-storage-fuse/issues/1226) If max disk-cache size is not configured, check the available disk space to kick-in early eviction.
- [#1230](https://github.com/Azure/azure-storage-fuse/issues/1230) Truncate file locally and then upload instead of downloading it again.

## 2.0.5 (2023-08-02)
**Features**
- In case of MSI based authentication, user shall provide object-id of the identity and honour-acl flag for file-system to work with ACLs assigned to the given identity instead of permissions.
- Added support to read OAuth token from a user given file.

**Bug Fixes**
- Fixed priority level check of components to reject invalid pipeline entries.
- [#1196](https://github.com/Azure/azure-storage-fuse/issues/1196) 100% CPU usage in 2.0.4 fixed.
- [#1207](https://github.com/Azure/azure-storage-fuse/issues/1207) Fix log-rotate script.
- Unmount command was looking for `fusermount` while on fuse3 systems it should be looking for `fusermount3`.
- If `du` command is not found skip checking for disk usage in LRU cache-eviction policy.
- V1 flag of `file-cache-timeout-in-seconds` not interpreted correctly by V2 and causing eviction policy to assume its 0. 
- If `du` is not found on standard path try paths where it can potentially be found.
- Fix uid/gid marshalling for `mountv1` command, which was resulting in panic.

## 2.0.4 (2023-07-03)
**Features**
- Added new config parameter "max-fuse-threads" under "libfuse" config to control max threads allowed at libfuse layer.
- Added new config parameter 'refresh-sec' in 'file-cache'. When file-cache-timeout is set to a large value, this field can control when to refresh the file if file in container has changed.
- Added FUSE option `direct_io` to bypass the kernel cache and perform direct I/O operations.


**Bug Fixes**
- [#1116](https://github.com/Azure/azure-storage-fuse/issues/1116) Relative path for tmp-cache is resulting into file read-write failure.
- [#1151](https://github.com/Azure/azure-storage-fuse/issues/1151) Reason for unmount failure is not displayed in the console output.
- Remove leading slashes from subdirectory name.
- [#1156](https://github.com/Azure/azure-storage-fuse/issues/1156) Reuse 'auth-resource' config to alter scope of SPN token.
- [#1175](https://github.com/Azure/azure-storage-fuse/issues/1175) Divide by 0 exception in Stream in case of direct streaming option.
- [#1161](https://github.com/Azure/azure-storage-fuse/issues/1161) Add more verbose logs for pipeline init failures
- Return permission denied error for `AuthorizationPermissionMismatch` error from service.
- [#1187](https://github.com/Azure/azure-storage-fuse/issues/1187) File-cache path will be created recursively, if it does not exist.
- Resolved bug related to constant 5% CPU usage even where there is no activity on the blobfuse2 mounted path.

## 2.0.3 (2023-04-26)
**Bug Fixes**
- [#1080](https://github.com/Azure/azure-storage-fuse/issues/1080) HNS rename flow does not encode source path correctly.
- [#1081](https://github.com/Azure/azure-storage-fuse/issues/1081) Blobfuse will exit with non-zero status code if allow_other option is used but not enabled in fuse config.
- [#1079](https://github.com/Azure/azure-storage-fuse/issues/1079) Shell returns before child process mounts the container and if user tries to bind the mount it leads to inconsistent state.
- If mount fails in forked child, blobfuse2 will return back with status error code.
- [#1100](https://github.com/Azure/azure-storage-fuse/issues/1100) If content-encoding is set in blob then transport layer compression shall be disabled.
- Subdir mount is not able to list blobs correctly when virtual-directory is turned on.
- Adding support to pass down uid/gid values supplied in mount to libfuse.
- [#1102](https://github.com/Azure/azure-storage-fuse/issues/1102) Remove nanoseconds from file times as storage does not provide that  granularity.
- [#1113](https://github.com/Azure/azure-storage-fuse/issues/1113) Allow-root option is not sent down to libfuse.

**Features**
- Added new CLI parameter "--sync-to-flush". Once configured sync() call on file will force upload a file to storage container. As this is file handle based api, if file was not in file-cache it will first download and then upload the file. 
- Added new CLI parameter "--disable-compression". Disables content compression at transport layer. Required when content-encoding is set to 'gzip' in blob.
- Added new config "max-results-for-list" that allow users to change maximum results returned as part of list calls during getAttr.
- Added new config "max-files" that allows users to change maximum files attributes that can be cached in attribute cache.
- Ensures all panic errors are logged before blobfuse crashes. 

## 2.0.2 (2023-02-23)
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
