#ifndef __NFS_INTERNAL_H__
#define __NFS_INTERNAL_H__

#include "aznfsc.h"

struct mount_options
{
    //
    // This will be of the form account.blob.core.windows.net
    // or for pre-prod : account.blob.preprod.core.windows.net
    // or 			   : IP
    // This will be constructed from the account_name and blobprefix passed by the caller.
    //
    // Server address: account+"."+cloud_suffix
    const std::string server;

    // Path to be exported. /account/container
    const std::string export_path;

    // Defaults to version 3
    const int nfs_version;

    // Port to mount to 2047 or port 2048)
    // This will default to 2048
    const int mount_port;

    const int nfs_port;

    // nconnect option. Number of connections to be established to the server.
    // This will default to 1.
    const int num_connections;

    // Max read and write sizes.
    const size_t rsize;
    const size_t wsize;

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
        readdir_maxcount(aznfsc_cfg.readdir_maxcount)
    {
    }

    mount_options(const mount_options* opt):
        nfs_version(opt->nfs_version),
        mount_port(opt->mount_port),
        nfs_port(opt->nfs_port),
        num_connections(opt->num_connections),
        rsize(1048576),
        wsize(1048576),
        readdir_maxcount(UINT32_MAX)
    {
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

// This structure contains all the server related info returned by the Fsinfo call.
struct nfs_server_info
{
    // TODO: Add members
};

// This structure contains the data returned by the Fsstat call which includes
// all the statistics of the server.
struct nfs_server_stat
{
    // TODO: Add members.
};
#endif /* __NFS_INTERNAL_H__ */
