#pragma once
#include <string>

struct mount_options
{
    // Server address: account+"."+cloud_suffix
    std::string server;

    // Path to be exported. /account/container
    std::string export_path;

    // Defaults to version 3
    int nfs_version;

    // Port to mount to 2047 or port 2048)
    // This will default to 2048
    int mount_port;

    int nfs_port;

    // nconnect option. Number of connections to be established to the server.
    // This will default to 1.
    int num_connections;

    // Max read and write sizes.
    size_t readmax;
    size_t writemax;

    // Maximum number of readdir entries that can be requested.
    uint32_t readdir_maxcount;

    // Add any other options as needed.

    mount_options():
        server(""),
        export_path(""),
        nfs_version(3),
        mount_port(2048),
        nfs_port(2048),
        num_connections(1)
    {}

    mount_options(const mount_options* opt):
        nfs_version(opt->nfs_version),
        mount_port(opt->mount_port),
        nfs_port(opt->nfs_port),
        num_connections(opt->num_connections),
        readmax(1048576),
        writemax(1048576),
        readdir_maxcount(UINT32_MAX)
    {
    }

    void set_read_max(size_t max)
    {
        readmax = max;
    }

    size_t get_read_max() const
    {
        return readmax;
    }

    void set_write_max(size_t max)
    {
        writemax = max;
    }

    size_t get_write_max() const
    {
        return writemax;
    }

    void set_nfs_port(int port)
    {
        nfs_port = port;
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
struct NfsServerInfo
{
    // TODO: Add members
};

// This structure contains the data returned by the Fsstat call which includes
// all the statistics of the server.
struct NfsServerStat
{
    // TODO: Add members.
};
