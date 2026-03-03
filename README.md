# Blobfuse - A Microsoft supported Azure Storage FUSE driver
## About

Blobfuse is an open source project developed to provide a virtual filesystem backed by the Azure Storage. It uses the libfuse open source library (fuse3) to communicate with the Linux FUSE kernel module, and implements the filesystem operations using the Azure Storage REST APIs. BlobFuse lets Linux workloads use Azure Blob Storage like a local file system for several scenarios, including AI/ML training and checkpointing, HPC simulations, Kubernetes stateful workloads, big data analytics/preprocessing, and large-scale backup and archiving. BlobFuse supports standard file operations with caching, offers health monitoring and blob filtering, and can preload containers or folders into the local cache for faster access.

BlobFuse supports two operating modes: a **caching** mode that works by cahing data locally on VM nodes, ideal for repeatedly accessed data that fits on VM local storage, and a **streaming** mode that reads large files in chunks directly from storage for big AI/ML, genomics, or HPC workloads. This [article](https://learn.microsoft.com/azure/storage/blobs/blobfuse2-streaming-versus-caching) helps you decide which mode is best suited for your workloads.
> [!NOTE]
> BlobFuse v2 is the latest version of BlobFuse and has many significant improvements over BlobFuse v1. BlobFuse v1 support ends in September 2026. Migrate to BlobFuse v2 by using the provided [instructions](https://github.com/Azure/azure-storage-fuse/blob/main/MIGRATION.md).

Detailed documentation of BlobFuse is [here](https://learn.microsoft.com/azure/storage/blobs/blobfuse2-what-is).  
Please submit any issues/feature requests/questions [here](https://github.com/azure/azure-storage-fuse/issues).

## Install BlobFuse
You can install BlobFuse from Microsoft repositories for Linux by using simple commands to install the BlobFuse package. If no package is available for your distribution and version, you can build the binary from source code. Refer to the [instructions](https://learn.microsoft.com/azure/storage/blobs/blobfuse2-mount-container) for details.

## Mount BlobFuse
You can mount a container by using the `mount` command. You can either include your desired configuration settings as command line parameters or provide a configuration file that contains your settings. Refer the [page](https://learn.microsoft.com/azure/storage/blobs/blobfuse2-install) for details.

## BlobFuse Logging
By default, BlobFuse logs warnings to the system log. However, you can route logs to a local directory, change which types of information appear in logs, or disable logs entirely by changing the default configuration. Refer the [page](https://learn.microsoft.com/azure/storage/blobs/blobfuse2-enable-logs) for details.

## Monitor BlobFuse
Health monitor is a tool that you use to monitor mount activities and resource usage. This [article](https://learn.microsoft.com/azure/storage/blobs/blobfuse2-health-monitor) describes what data you can obtain, and how to enable health monitor and view output reports.

## Find help from your command prompt
To see a list of commands, type `blobfuse2 -h` and then press the ENTER key.
To learn about a specific command, just include the name of the command (For example: `blobfuse2 mount -h`).

## Limitations and known issues with BlobFuse
Refer the [page](https://learn.microsoft.com/en-us/azure/storage/blobs/blobfuse2-known-issues) for limitations and known issues.  
For troubleshooting common issues, refer this [page](https://learn.microsoft.com/azure/storage/blobs/blobfuse2-troubleshooting).

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

