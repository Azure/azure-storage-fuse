#ifndef __AZNFSC_H__
#define __AZNFSC_H__

#include <stddef.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <errno.h>
#include <fcntl.h>
#include <unistd.h>
#include <assert.h>

#define FUSE_USE_VERSION 35
#include <fuse3/fuse_lowlevel.h>
#include <fuse3/fuse.h>
#include <linux/fuse.h>

#include "libnfs.h"
#include "libnfs-raw.h"
#include "libnfs-raw-mount.h"
#include "libnfs-raw-nfs.h"

#include "aznfsc_config.h"
#include "log.h"

/**
 * This structure holds the entire aznfsclient configuration that controls the
 * behaviour of the aznfsclient fuse program. These config variables can be
 * configured in many ways, allowing user to conveniently express their default
 * configuration and allowing easy overrides for some as needed.
 *
 * Here are the various ways these config values are populated:
 * 1. Most configs have default values.
 *    Note: Some of the config variables pertain to user details and cannot
 *          have default values.
 * 2. Convenient place for defining config variables which don't need to be
 *    changed often is the config.yaml file that user can provide with the
 *    --config-file=./config.yaml cmdline option to aznfsclient.
 *    These override the defaults.
 * 3. Some but not all config variables can be set using environment variables.
 *    These override the variables set by config.yaml and the default.
 * 4. Most config variables can be set using specific command line options to
 *    aznfsclient.
 *    These have the highest preference and will override the variables set
 *    by environment variables, config.yaml and the default.
 *
 * Note: This MUST not contains C++ object types as members as fuse parser
 *       writes into those members. For char* members fuse also allocates
 *       memory.
 *       An exception to this are the fields in the "Aggregates" section.
 *       These are not set by fuse parser but are stored for convenience.
 */
typedef struct aznfsc_cfg
{
    // config.yaml file path specified using --config-file= cmdline option.
    const char* config_yaml = nullptr;

    /*************************************************
     **                Mount path                   **
     ** Identify the server and the export to mount **
     *************************************************/

    /*
     * Storage account and container to mount and the optional cloud suffix.
     * The share path mounted is:
     * <account>.<cloud_suffix>:/<account>/<container>
     */
    const char* account = nullptr;
    const char* container = nullptr;
    const char* cloud_suffix = nullptr;

    /**********************************************************************
     **                          Mount options                           **
     ** These are deliberately named after the popular NFS mount options **
     **********************************************************************/

    /*
     * NFS and Mount port to use.
     * If this is non-zero, portmapper won't be contacted.
     * Note that Blob NFS uses the same port for Mount and NFS, hence we have
     * just one config.
     */
    uint16_t port = 0;

    // Number of connections to be established to the server.
    uint16_t nconnect = 1;

    // Maximum size of read request.
    uint32_t rsize = 1048576;

    // Maximum size of write request.
    uint32_t wsize = 1048576;

    /*
     * Number of times the request will be retransmitted to the server when no
     * response is received, before the "server not responding" message is
     * logged and further recovery is attempted.
     */
    uint16_t retrans = 3;

    /*
     * Time in deci-seconds we will wait for a response before retrying the
     * request.
     */
    uint16_t timeo = 600;

    // Maximum number of readdir entries that can be requested in a single call.
    uint32_t readdir_maxcount = UINT32_MAX;

    /*
     * TODO:
     * - Add auth related config.
     * - Add perf related config,
     *   e.g., amount of RAM used for staging writes, etc.
     */

    /**************************************************************************
     **                            Aggregates                                **
     ** These store composite config variables formed from other config      **
     ** variables which were set as options using aznfsc_opts.               **
     ** These aggregate membets MUST NOT be set as options using aznfsc_opts,**
     ** as these can be C++ objects.                                         **
     **************************************************************************/
    std::string server;
    std::string export_path;

    /**
     * Default constructor.
     * It's primarily there to assign default value for cloud_suffix.
     * We need to allocate memory that can be free()d since fuse option
     * parser will call free() before assigning fresh value.
     */
    aznfsc_cfg() :
	    cloud_suffix(strdup("blob.core.windows.net"))
    {
    }
} aznfsc_cfg_t;

extern struct aznfsc_cfg aznfsc_cfg;

#endif /* __AZNFSC_H__ */
