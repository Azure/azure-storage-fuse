# Blobfuse Migration Guide from v1.X.X to v2.X.X

In this guide, we list the main changes you need to be aware of when migrating your workloads from blobfuse 1.X.X to blobfuse2 2.X.X.

<!-- Do we need an upgrade story? -->

## Prerequisites
Follow the steps in Installation to get started. <!-- TODO: Add link to installation and copy paste the below to the Installation page -->

blobfuse2 is currently available in the Microsoft product repositories for Ubuntu, SLES, Debian, CentOS/RedHat distros. Packages are only available for x86 platforms. For Kubernetes support, go [here](https://github.com/kubernetes-sigs/blob-csi-driver).

1. Configure the apt repository for Microsoft products following [this](https://docs.microsoft.com/en-us/windows-server/administration/Linux-Package-Repository-for-Microsoft-Software) guideline.

On Ubuntu
```
wget https://packages.microsoft.com/config/ubuntu/<UBUNTU_VERSION>/packages-microsoft-prod.deb
sudo dpkg -i packages-microsoft-prod.deb
sudo apt-get update
```

On RHEL
```
 sudo rpm -Uvh https://packages.microsoft.com/rhel/<RHEL_VERSION>/prod/blobfuse2-<BLOBFUSE2_VERSION>-RHEL-<RHEL_VERSION>-x86_64.rpm
```

On Kubernetes, go [here](https://github.com/kubernetes-sigs/blob-csi-driver)

1. Install blobfuse2

On Ubuntu
```
sudo apt-get install blobfuse2 fuse
```

On RedHat/CentOS
```
sudo yum install blobfuse2 fuse
```

Now you're good to go.

## Updated Package Name
The package name has been updated to `blobfuse2` to 

- Prevent breaking existing blobfuse workloads 
- Allow the two blobfuse versions to co-exist during migration

## Blobfuse2 Config Converter Tool
To make migration easier, we created a tool to mount with v1 configurations to migrate from blobfuse to blobfuse2 seamlessly.

Run the following command with the same CLI parameters as you would pass to blobfuse

```
blobfuse2 mountv1 <MOUNT_PATH> --config-file=<BLOBFUSE_V1_CONFIG_PATH> <BLOBFUSE_V1_CLI_PARAMETERS> --output-file=<OPTIONAL_DESIRED_BLOBFUSE2_CONFIG_PATH>
```

You can also choose to only convert the v1 configuration to v2 without mounting by passing `--convert-config-only=true`

## Mounting
Blobfuse2 can be mounted with the following command
```
blobfuse2 mount <MOUNT_PATH> --config-file=<CONFIG_PATH> <ADDITIONAL_CLI_PARAMETERS>
```

A detailed description of all the mount options offered by blobfuse2 can be see in the README. If you have an existing blobfuse command and config, you may also choose to migrate with the following instructions.

## Mount options
blobfuse2 will read options for a given parameter with the following order of precedence
1. CLI flag
2. environment variable
3. config file

Note: For an exhaustive list of the blobfuse2 config file options and the format of the file, see [here](/setup/baseConfig.yaml).
<!-- TODO: Link that correctly once on github -->

### Blobfuse CLI Flag Options
<!-- Note: When editing this table, please ensure it is formatted neatly -->
| Blobfuse CLI Flag Parameter             | Blobfuse2 Replacement CLI Parameter | Blobfuse2 Replacement Config File | Notes                                                                     |
|-----------------------------------------|-------------------------------------|-----------------------------------|---------------------------------------------------------------------------|
| -o allow_other                          | --allow_other                       | allow-other                       |                                                                           |
| -o ro                                   | --read-only                         | read-only                         |                                                                           |
| --tmp-path=PATH                         | --tmp-path=PATH                     | file_cache.path                   |                                                                           |
| --empty-dir-check=false                 |                                     | file_cache.allow-non-empty-temp   |                                                                           |
| --config-file=PATH                      | --config-file=PATH                  |                                   |                                                                           |
| --container-name=NAME                   | --container-name=NAME               | azstorage.container               |                                                                           |
| --use-https=true                        |                                     | azstorage.use-http                | This parameter has the opposite boolean semantics                         |
| --file-cache-timeout-in-seconds=120     |                                     | file_cache.timeout-sec            | Default changed to 0                                                      |
| --log-level=LOG_WARNING                 | --log-level=LOG_WARNING             | logging.level                     |                                                                           |
| --use-attr-cache=true                   |                                     | attr_cache                        | Add attr_cache to the components list                                     |
| --use-adls=false                        |                                     | azstorage.type                    | Specify either 'block' or 'adls'                                          |
| --no-symlinks=false                     | --no-symlinks=false                 | attr_cache.no-symlinks            |                                                                           |
| --cache-on-list=true                    |                                     | attr_cache.no-cache-on-list       | This parameter has the opposite boolean semantics                         |
| --upload-modified-only=false            |                                     |                                   | Default behavior in blobfuse2                                             |
| --max-concurrency=12                    |                                     | azstorage.max-concurrency         |                                                                           |
| --cache-size-mb=0                       |                                     | file_cache.max-size-mb            |                                                                           |
| --cancel-list-on-mount-seconds=0        |                                     | azstorage.block-list-on-mount-sec |                                                                           |
| --high-disk-threshold=90                |                                     | file_cache.high-threshold         |                                                                           |
| --low-disk-threshold=80                 |                                     | file_cache.low-threshold          |                                                                           |
| --cache-poll-timeout-msec=1000          |                                     |                                   | Not an option in blobfuse2                                                |
| ---max-eviction=0                       |                                     | file_cache.max-eviction           |                                                                           |
| --set-content-type=false                |                                     |                                   | Not an option in blobfuse2, always true                                   |
| --ca-cert-file=/etc/ssl/certs/proxy.pem |                                     |                                   | Store the ca cert file in the default/standard Linux path                 |
| --https-proxy=http://10.1.22.4:8080/    |                                     | azstorage.https-proxy             |                                                                           |
| --http-proxy=http://10.1.22.4:8080/     |                                     | azstorage.http-proxy              |                                                                           |
| --max-retry=26                          |                                     | azstorage.max-retries             |                                                                           |
| --max-retry-interval-in-seconds=60      |                                     | azstorage.max-retry-timeout-sec   |                                                                           |
| --retry-delay-factor=1.2                |                                     | azstorage.retry-backoff-sec       |                                                                           |
| --basic-remount-check=false             |                                     |                                   | Not applicable in blobfuse2                                               |
| --pre-mount-validate=true               |                                     |                                   | Always on in Blobfuse2 blobfuse2                                          |
| --background-download=false             |                                     |                                   | Not an option in blobfuse2                                                |
| --invalidate-on-sync=false              |                                     |                                   | Always on in blobfuse2                                                    |
| --streaming=true                        |                                     | stream                            | Add stream to the components list                                         |
| --stream-cache-mb=500                   |                                     | stream.cache-size-mb              |                                                                           |
| ---max-blocks-per-file=3                |                                     | stream.blocks-per-file            |                                                                           |
| --block-size-mb=16                      |                                     | stream.block-size-mb              |                                                                           |
| -o attr_timeout=20                      | --attr_timeout=20                   | libfuse.attribute-expiration-sec  | Default changed to 0                                                      |
| -o entry_timeout=20                     | --entry_timeout=20                  | libfuse.entry-expiration-sec      | Default changed to 0                                                      |
| -o negative_timeout=20                  | --negative_timeout=20               |                                   | Default changed to 0                                                      |
| -d                                      |                                     | libfuse.fuse-trace                |                                                                           |
| -o umask                                |                                     | libfuse.default-permission        |                                                                           |

### Blobfuse Environment Variable Options
Blobfuse2 reads all the environment variables that Blobfuse does, so no changes for environment variables are required during migration.

### Blobfuse Config File Options
<!-- Note: When editing this table, please ensure it is formatted neatly -->
| Blobfuse Config File Parameter | Blobfuse2 Replacement Environment Variable | Blobfuse2 Replacement Config File |
|--------------------------------|--------------------------------------------|-----------------------------------|
| accountName                    |                                            | azstorage.account-name            |
| containerName                  |                                            | azstorage.container               |
| blobEndpoint                   |                                            | azstorage.endpoint                |
| authType                       |                                            | azstorage.mode                    |
| accountType                    |                                            | azstorage.type                    |
| accountKey                     |                                            | azstorage.account-key             |
| sasToken                       |                                            | azstorage.sas                     |
| identityClientId               |                                            | azstorage.appid                   |
| identityObjectId               |                                            | azstorage.objid                   |
| identityResourceId             |                                            | azstorage.resid                   |
| msiEndpoint                    | MSI_ENDPOINT                               |                                   |
| servicePrincipalClientId       |                                            | azstorage.clientid                |
| servicePrincipalTenantId       |                                            | azstorage.tenantid                |
| aadEndpoint                    |                                            | azstorage.aadendpoint             |
| logLevel                       |                                            | logging.level                     |
| httpProxy                      |                                            | azstorage.http-proxy              |
| httpsProxy                     |                                            | azstorage.https-proxy             |
| caCertFile                     | N/A                                        | N/A                               |
| dnsType                        | N/A                                        | N/A                               |
