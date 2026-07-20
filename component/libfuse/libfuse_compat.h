#ifndef BLOBFUSE_LIBFUSE_COMPAT_H
#define BLOBFUSE_LIBFUSE_COMPAT_H

#include <stdio.h>

static int libfuse_version_supports_dir_cache(const char *version)
{
    unsigned int major = 0;
    unsigned int minor = 0;
    unsigned int patch = 0;
    char major_separator = '\0';
    char minor_separator = '\0';

    if (version == NULL ||
        sscanf(version, "%u%c%u%c%u", &major, &major_separator, &minor, &minor_separator, &patch) != 5 ||
        major_separator != '.' || minor_separator != '.')
        return 0;

    return major > 3 || (major == 3 && (minor > 16 || (minor == 16 && patch >= 1)));
}

#endif // BLOBFUSE_LIBFUSE_COMPAT_H