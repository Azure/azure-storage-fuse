#define FUSE_USE_VERSION 39
#define _FILE_OFFSET_BITS 64

#include <fuse.h>
#include "../component/libfuse/libfuse_compat.h"

#ifndef FUSE_CAP_NO_OPENDIR_SUPPORT
#error "FUSE_CAP_NO_OPENDIR_SUPPORT is required"
#endif

int main(void)
{
    const char *runtime_version = fuse_pkgversion();
    struct fuse_file_info file_info = {0};

    if (libfuse_version_supports_dir_cache("3.5.0") ||
        libfuse_version_supports_dir_cache("3.16.0") ||
        !libfuse_version_supports_dir_cache("3.16.1") ||
        !libfuse_version_supports_dir_cache("3.18.0-rc0") ||
        libfuse_version_supports_dir_cache("invalid"))
        return 1;

    file_info.cache_readdir = 1;
    if (file_info.cache_readdir != 1)
        return 1;

    if (libfuse_version_supports_dir_cache(runtime_version)) {
        printf("libfuse runtime %s forwards cache_readdir through the high-level API\n",
               runtime_version);
    } else {
        printf("libfuse runtime %s does not forward cache_readdir; "
               "blobfuse2 will disable kernel list caching\n",
               runtime_version != NULL ? runtime_version : "unknown");
    }

    return 0;
}