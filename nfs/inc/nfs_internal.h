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

    // Local mount directory.
    const std::string mountpoint;

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

    // ro or rw mount?
    const bool readonly;

    // Max read and write sizes.
    const int rsize;
    const int wsize;

    // rsize and wsize adjusted as per server advertised values.
    int rsize_adj = 0;
    int wsize_adj = 0;

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
    const int readdir_maxcount;

    // Readahead size in KB.
    const int readahead_kb;

    // readdir_maxcount adjusted as per server advertised value.
    int readdir_maxcount_adj = 0;

    // Add any other options as needed.

    /*
     * TODO: Add support for readonly mount.
     */
    mount_options():
        server(aznfsc_cfg.server),
        export_path(aznfsc_cfg.export_path),
        mountpoint(aznfsc_cfg.mountpoint),
        nfs_version(3),
        mount_port(aznfsc_cfg.port),
        nfs_port(aznfsc_cfg.port),
        num_connections(aznfsc_cfg.nconnect),
        readonly(false),
        rsize(aznfsc_cfg.rsize),
        wsize(aznfsc_cfg.wsize),
        retrans(aznfsc_cfg.retrans),
        timeo(aznfsc_cfg.timeo),
        acregmin(aznfsc_cfg.acregmin),
        acregmax(aznfsc_cfg.acregmax),
        acdirmin(aznfsc_cfg.acdirmin),
        acdirmax(aznfsc_cfg.acdirmax),
        actimeo(aznfsc_cfg.actimeo),
        readdir_maxcount(aznfsc_cfg.readdir_maxcount),
        readahead_kb(aznfsc_cfg.readahead_kb)
    {
        assert(!server.empty());
        assert(!export_path.empty());
        assert(!mountpoint.empty());
    }

    /**
     * From the mount options create a url string required by libnfs.
     * This is how mount options are passed to libnfs.
     */
    const std::string get_url_str() const
    {
        std::string url(1024, '\0');
        // TODO: Take it from aznfsc_cfg.
        const int debug = 1;

        /*
         * For Blob NFS force nfsport and mountport to avoid portmapper
         * calls. We assume Blob NFS if port is set to 2048 or 2047.
         * For using non Blob NFS servers set port to 2049 in config.yaml.
         */
        int size;

        if (nfs_port == 2048 || nfs_port == 2047) {
            size = std::snprintf(
                                const_cast<char*>(url.data()),
                                url.size(),
                                "nfs://%s%s/?version=3&debug=%d&xprtsec=none&nfsport=%d&mountport=%d&timeo=%d&retrans=%d&rsize=%d&wsize=%d&readdir-buffer=%d",
                                server.c_str(),
                                export_path.c_str(),
                                debug,
                                nfs_port,
                                mount_port,
                                timeo,
                                retrans,
                                rsize,
                                wsize,
                                readdir_maxcount);
        } else {
            size = std::snprintf(
                                const_cast<char*>(url.data()),
                                url.size(),
                                "nfs://%s%s/?version=3&debug=%d&xprtsec=none&timeo=%d&retrans=%d&rsize=%d&wsize=%d&readdir-buffer=%d",
                                server.c_str(),
                                export_path.c_str(),
                                debug,
                                timeo,
                                retrans,
                                rsize,
                                wsize,
                                readdir_maxcount);
        }

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
