#ifndef __AZNFSC_H__
#define __AZNFSC_H__

#include <stddef.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <errno.h>
#include <fcntl.h>
#include <unistd.h>
#include <limits.h>
#include <assert.h>

#define FUSE_USE_VERSION 312
#include <fuse3/fuse_lowlevel.h>
#include <fuse3/fuse.h>
#include <linux/fuse.h>

#include "libnfs.h"
#include "libnfs-raw.h"
#include "libnfs-raw-mount.h"
#include "libnfs-raw-nfs.h"

#include "aznfsc_config.h"
#include "log.h"
#include "util.h"

using namespace aznfsc;

// Min/Max values for various aznfsc_cfg options.
#define AZNFSCFG_NCONNECT_MIN   1
#define AZNFSCFG_NCONNECT_MAX   256
#define AZNFSCFG_TIMEO_MIN      100
#define AZNFSCFG_TIMEO_MAX      6000
#define AZNFSCFG_RSIZE_MIN      8192
#define AZNFSCFG_RSIZE_MAX      104857600
#define AZNFSCFG_WSIZE_MIN      8192
#define AZNFSCFG_WSIZE_MAX      104857600
#define AZNFSCFG_READDIR_MIN    8192
#define AZNFSCFG_READDIR_MAX    4194304
#define AZNFSCFG_READAHEAD_KB_MIN 128
#define AZNFSCFG_READAHEAD_KB_MAX 1048576
#define AZNFSCFG_RETRANS_MIN    1
#define AZNFSCFG_RETRANS_MAX    100
#define AZNFSCFG_ACTIMEO_MIN    1
#define AZNFSCFG_ACTIMEO_MAX    3600

// W/o jumbo blocks, 5TiB is the max file size we can support.
#define AZNFSC_MAX_FILE_SIZE    (100 * 1024 * 1024 * 50'000ULL)

/*
 * Max fuse_opcode enum value.
 * This keeps increasing with newer fuse versions, but we don't want it
 * to be the exact maximum, we just want it to be more than all the opcodes
 * that we support.
 */
#define FUSE_OPCODE_MAX         FUSE_LSEEK

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
    const char *config_yaml = nullptr;

    /*************************************************
     **                Mount path                   **
     ** Identify the server and the export to mount **
     *************************************************/

    /*
     * Storage account and container to mount and the optional cloud suffix.
     * The share path mounted is:
     * <account>.<cloud_suffix>:/<account>/<container>
     */
    const char *account = nullptr;
    const char *container = nullptr;
    const char *cloud_suffix = nullptr;

    /*************************************************
     **                   Misc                      **
     *************************************************/

    // Directory where file caches will be persisted.
    const char *cachedir = nullptr;

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
    int port = -1;

    // Number of connections to be established to the server.
    int nconnect = -1;

    // Maximum size of read request.
    int rsize = -1;

    // Maximum size of write request.
    int wsize = -1;

    /*
     * Number of times the request will be retransmitted to the server when no
     * response is received, before the "server not responding" message is
     * logged and further recovery is attempted.
     */
    int retrans = -1;

    /*
     * Time in deci-seconds we will wait for a response before retrying the
     * request.
     */
    int timeo = -1;

    /*
     * Regular file and directory attribute cache timeout min and max values.
     * min value specifies the minimum time in seconds that we cache the
     * corresponding file type's attributes before we request fresh attributes
     * from the server. A successful attribute revalidation (i.e., mtime
     * remains unchanged) doubles the attribute timeout (up to
     * acregmax/acdirmax for file/directory), while a failed revalidation
     * resets it to acregmin/acdirmin.
     * If actimeo is specified it overrides all ac{reg|dir}min/ac{reg|dir}max
     * and the single actimeo value is used as the min and max attribute cache
     * timeout values for both file and directory types.
     */
    int acregmin = -1;
    int acregmax = -1;
    int acdirmin = -1;
    int acdirmax = -1;
    int actimeo = -1;

    // Maximum number of readdir entries that can be requested in a single call.
    int readdir_maxcount = -1;

    // Readahead size in KB.
    int readahead_kb = -1;

    /*
     * TODO:
     * - Add auth related config.
     * - Add perf related config,
     * - Add hard/soft mount option,
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
     * Local mountpoint.
     * This is not present in the config file, but is taken from the
     * cmdline.
     */
    std::string mountpoint;

    /**
     * Parse config_yaml if set by cmdline --config-file=
     */
    bool parse_config_yaml();

    /**
     * Set default values for options not yet assigned.
     * This must be called after fuse_opt_parse() and parse_config_yaml()
     * assign config values from command line and the config yaml file.
     * Also sanitizes various values.
     */
    void set_defaults_and_sanitize();
} aznfsc_cfg_t;

extern struct aznfsc_cfg aznfsc_cfg;

#endif /* __AZNFSC_H__ */
