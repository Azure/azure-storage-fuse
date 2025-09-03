# Blobfuse2 - A Microsoft supported Azure Storage FUSE driver
## About
Blobfuse2 is an open source project developed to provide a virtual filesystem backed by the Azure Storage. It uses the libfuse open source library (fuse3) to communicate with the Linux FUSE kernel module, and implements the filesystem operations using the Azure Storage REST APIs.
This is the next generation [blobfuse](https://github.com/Azure/azure-storage-fuse).

## About Data Consistency and Concurrency
Blobfuse2 is stable and ***supported by Microsoft*** when used within its [documented limits](#un-supported-file-system-operations). Blobfuse2 supports high-performance reads and writes with strong consistency; however, it is recommended that multiple clients do not modify the same blob/file simultaneously to ensure data integrity. Blobfuse2 does not guarantee continuous synchronization of data written to the same blob/file using multiple clients or across multiple mounts of Blobfuse2 concurrently. If you modify an existing blob/file with another client while also reading that object, Blobfuse2 will not return the most up-to-date data. To ensure your reads see the newest blob/file data, disable all forms of caching at kernel (using `direct-io`) as well as at Blobfuse2 level, and then re-open the blob/file.

Please submit an issue [here](https://github.com/azure/azure-storage-fuse/issues) for any issues/feature requests/questions.

## [List of Supported Platforms](https://github.com/Azure/azure-storage-fuse/wiki/Blobfuse2-Supported-Platforms)

## [Steps to Install Blobfuse2](https://github.com/Azure/azure-storage-fuse/wiki/Blobfuse2-Installation)

## [Choose config](https://github.com/Azure/azure-storage-fuse/wiki/Blobfuse2%E2%80%90Config-Guide)

## [Config File Best Practices](https://github.com/Azure/azure-storage-fuse/wiki/Blobfuse2-File-Best-Practices)

## [Blockcache Limitations And Recommendations](https://github.com/Azure/azure-storage-fuse/wiki/Blobfuse2-Blockcache-Limitations-And-Recommendations)

## [Supported Operations And Usage](https://github.com/Azure/azure-storage-fuse/wiki/Blobfuse2-Supported-Operation-and-Usage)

## [Benchmarks](https://azure.github.io/azure-storage-fuse/)

## [Features](https://github.com/Azure/azure-storage-fuse/wiki/Blobfuse2-Features)

## [_New Health Monitor_](https://github.com/Azure/azure-storage-fuse/blob/main/tools/health-monitor/README.md)

## [CLI parameters](https://github.com/Azure/azure-storage-fuse/wiki/Blobfuse2%E2%80%90Cli%E2%80%90Parameters)

## [Environment variables](https://github.com/Azure/azure-storage-fuse/wiki/Blobfuse2%E2%80%90Environment-Variables)

## [Blob Filter](https://github.com/Azure/azure-storage-fuse/wiki/Blobfuse2%E2%80%90Blob-Filter)

## [Preload Data](https://github.com/Azure/azure-storage-fuse/wiki/Blobfuse2%E2%80%90Preload)

## [Using Private Endpoints with HNS-Enabled Storage Accounts](https://github.com/Azure/azure-storage-fuse/wiki/Blobfuse2%E2%80%90Private-Endpoint-With-HNS)

## [Enhancement Over BlobFusev1](https://github.com/Azure/azure-storage-fuse/wiki/Blobfuse2-Enhancement-Over-V1)

## [Frequently Asked Questions](https://github.com/Azure/azure-storage-fuse/wiki/Blobfuse2%E2%80%90FAQ)

## [Limitations And Unsupported Operations](https://github.com/Azure/azure-storage-fuse/wiki/Blobfuse2-Limitations)


##  NOTICE
- Due to known data consistency issues when using Blobfuse2 in `block-cache` mode,  it is strongly recommended that all Blobfuse2 installations be upgraded to version 2.3.2. For more information, see [this](https://github.com/Azure/azure-storage-fuse/wiki/Blobfuse2-Known-issues).
- Login via Managed Identify is supported with Object-ID for all versions of Blobfuse except 2.3.0 and 2.3.2.To use Object-ID for these two versions, use Azure CLI or utilize Application/Client-ID or Resource ID based authentication.
- `streaming` mode is being deprecated. This is the older option and is replaced by streaming with `block-cache` mode which is the more performant streaming option.
- Block cache will no longer dynamically consume more memory if required by application but will strictly adhere to the memory limit which is 80% of free memory by default or whatever is configured by the user.
  

## Find help from your command prompt
To see a list of commands, type `blobfuse2 -h` and then press the ENTER key.
To learn about a specific command, just include the name of the command (For example: `blobfuse2 mount -h`).

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
