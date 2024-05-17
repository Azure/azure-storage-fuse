#pragma once
#include <cstddef>
#include <string>

struct mountOptions
{
    // Server address: account+"."+cloud_suffix
    std::string server;

    // Path to be exported. /account/container
    std::string exportPath;

    // Defaults to version 3
    int nfsVersion;

    // Port to mount to 2047 or port 2048)
    // This will default to 2048
    int mountPort;

    int nfsPort;

    // nconnect option. Number of connections to be established to the server.
    // This will default to 1.
    int numConnections;

    // Max read and write sizes.
    size_t readmax;
    size_t writemax;

    // Maximum number of readdir entries that can be requested.
    uint32_t readdir_maxcount;

    // Add any other options as needed.

    mountOptions():
        server(""),
        exportPath(""),
        nfsVersion(3),
        mountPort(2048),
        nfsPort(2048),
        numConnections(1)
    {}

    mountOptions(const mountOptions* opt):
        nfsVersion(opt->nfsVersion),
        mountPort(opt->mountPort),
        nfsPort(opt->nfsPort),
        numConnections(opt->numConnections),
        readmax(1048576),
        writemax(1048576),
        readdir_maxcount(UINT32_MAX)
    {
    }

    void SetReadMax(size_t max)
    {
        readmax = max;
    }

    size_t GetReadMax() const
    {
        return readmax;
    }

    void SetWriteMax(size_t max)
    {
        writemax = max;
    }

    size_t GetWriteMax() const
    {
        return writemax;
    }

    void SetNfsPort(int port)
    {
        nfsPort = port;
    }

    int GetMountPort() const
    {
        return mountPort;
    }

    int GetPort() const
    {
        return nfsPort;
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
