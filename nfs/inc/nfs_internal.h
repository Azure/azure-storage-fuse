#ifndef __NFS_INTERNAL_H__
#define __NFS_INTERNAL_H__

#include "aznfsc.h"

struct mount_options
{
    /*
     * This will be of the form account.blob.core.windows.net
     * or for pre-prod : account.blob.preprod.core.windows.net
     * or 			   : IP
     * This will be constructed from the account name and cloud_suffix passed
     * by the caller.
     *
     * Server address: account + "." + cloud_suffix
     */
    const std::string server;

    // Path to be exported. /account/container
    const std::string export_path;

    // Defaults to version 3
    const int nfs_version;

    // Port to mount to (2047 or 2048).
    const int mount_port;

    // Same as mount_port.
    const int nfs_port;

    /*
     * nconnect option.
     * Number of connections to be established to the server.
     */
    const int num_connections;

    // Max read and write sizes.
    const size_t rsize;
    const size_t wsize;

    // How many RPC retransmits before major recovery.
    const int retrans;
    // Deci-seconds to timeout.
    const int timeo;

    // Attribute cache timeout related options.
    const int acregmin;
    const int acregmax;
    const int acdirmin;
    const int acdirmax;
    const int actimeo;

    // Maximum number of readdir entries that can be requested.
    const uint32_t readdir_maxcount;

    // Add any other options as needed.

    mount_options():
        server(aznfsc_cfg.server),
        export_path(aznfsc_cfg.export_path),
        nfs_version(3),
        mount_port(aznfsc_cfg.port),
        nfs_port(aznfsc_cfg.port),
        num_connections(aznfsc_cfg.nconnect),
        rsize(aznfsc_cfg.rsize),
        wsize(aznfsc_cfg.wsize),
        retrans(aznfsc_cfg.retrans),
        timeo(aznfsc_cfg.timeo),
        acregmin(aznfsc_cfg.acregmin),
        acregmax(aznfsc_cfg.acregmax),
        acdirmin(aznfsc_cfg.acdirmin),
        acdirmax(aznfsc_cfg.acdirmax),
        actimeo(aznfsc_cfg.actimeo),
        readdir_maxcount(aznfsc_cfg.readdir_maxcount)
    {
    }

    mount_options(const mount_options* opt):
        nfs_version(opt->nfs_version),
        mount_port(opt->mount_port),
        nfs_port(opt->nfs_port),
        num_connections(opt->num_connections),
        rsize(opt->rsize),
        wsize(opt->wsize),
        retrans(opt->retrans),
        timeo(opt->timeo),
        acregmin(opt->acregmin),
        acregmax(opt->acregmax),
        acdirmin(opt->acdirmin),
        acdirmax(opt->acdirmax),
        actimeo(opt->actimeo),
        readdir_maxcount(opt->readdir_maxcount)
    {
    }

    /**
     * From the mount options create a url string required by libnfs.
     * This is how mount options are passed to libnfs.
     */
    const std::string get_url_str() const
    {
        std::string url(1024, '\0');
        // TODO: Take it from aznfsc_cfg.
        const int debug = 0;

        /*
         * For Blob NFS force nfsport and mountport to avoid portmapper
         * calls.
         */
#ifndef ENABLE_NON_AZURE_NFS
        const int size = std::snprintf(
                            const_cast<char*>(url.data()),
                            url.size(),
                            "nfs://%s%s/?version=3&debug=%d&xprtsec=none&nfsport=%d&mountport=%d&timeo=%d&retrans=%d",
                            server.c_str(),
                            export_path.c_str(),
                            debug,
                            nfs_port,
                            mount_port,
                            timeo,
                            retrans);
#else
        const int size = std::snprintf(
                            const_cast<char*>(url.data()),
                            url.size(),
                            "nfs://%s%s/?version=3&debug=%d&xprtsec=none&timeo=%d&retrans=%d",
                            server.c_str(),
                            export_path.c_str(),
                            debug,
                            timeo,
                            retrans);
#endif

        assert(size < (int) url.size());
        url.resize(size);
        return url;
    }

    size_t get_read_max() const
    {
        return rsize;
    }

    size_t get_write_max() const
    {
        return wsize;
    }

    int get_mount_port() const
    {
        return mount_port;
    }

    int get_port() const
    {
        return nfs_port;
    }
};

/**
 * This structure contains all the dynamic filesystem info returned by the
 * FSINFO RPC.
 */
struct nfs_server_info
{
    // TODO: Add members
};

/**
 * This structure contains all the static filesystem info returned by the
 * FSSTAT RPC.
 */
struct nfs_server_stat
{
    // TODO: Add members.
};
#endif /* __NFS_INTERNAL_H__ */
