package cmd

import (
	_ "github.com/Azure/azure-storage-fuse/v2/component/attr_cache"
	_ "github.com/Azure/azure-storage-fuse/v2/component/azstorage"
	_ "github.com/Azure/azure-storage-fuse/v2/component/block_cache"
	_ "github.com/Azure/azure-storage-fuse/v2/component/file_cache"
	_ "github.com/Azure/azure-storage-fuse/v2/component/libfuse"
	_ "github.com/Azure/azure-storage-fuse/v2/component/loopback"
	_ "github.com/Azure/azure-storage-fuse/v2/component/parallelUpload"
	_ "github.com/Azure/azure-storage-fuse/v2/component/stream"
)
