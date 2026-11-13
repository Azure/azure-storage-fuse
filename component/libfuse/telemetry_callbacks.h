/* Copyright (c) Microsoft Corporation. Licensed under the MIT License. */
#ifndef BLOBFUSE_TELEMETRY_CALLBACKS_H
#define BLOBFUSE_TELEMETRY_CALLBACKS_H

#include <dlfcn.h>
#include <stddef.h>

static const char *telemetry_libfuse_version(void)
{
    typedef const char *(*version_func)(void);
    void *handle = dlopen(NULL, RTLD_LAZY);
    if (!handle)
        return "unknown";
    version_func version = (version_func)dlsym(handle, "fuse_pkgversion");
    const char *result = version ? version() : "unknown";
    dlclose(handle);
    return result;
}

#endif
