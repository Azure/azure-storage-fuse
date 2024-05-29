#pragma once

#include "connection.h"

//
// This is the actual transport class.
// This class is responsible for sending the requests out on the connections.
// This has a vector of nfs_connection objects on which the request will be sent out.
// If the mount is done with nconnect=x, then this layer will take care of round robining the
// requests over x connections.
//
class rpc_transport
{
    // This contains all the information needed to create connection to the server.
    struct mount_options* mnt_options;

    //
    // Vector of nfs_connection. This will be of length nconnect option passed by the user.
    // TODO: Check if we need a lock over this vector.
    //
    std::vector<struct nfs_connection*> nfs_connections;

    //
    // Last context on which the request was sent.
    // Note: Each context is identified from 0 to X (where X is the value of nconnect)
    //
    static int last_context;

    // Make the constructor private as it is singleton.
    rpc_transport(struct mount_options* mount_options):
        mnt_options(mount_options)
    {
        assert(mnt_options != nullptr);
    }

public:
    // This is the method which should be called to get an instance of this class by the user.
    static rpc_transport* get_instance(struct mount_options* mnt_options)
    {
        static rpc_transport instance(mnt_options);
        return &instance;
    }

    //
    // This should open the connection(s) to the remote server and initialize all the data members accordingly.
    // This will create 'x' number of nfs_connection objects where 'x' is the nconnect option and call open for each of them.
    // It opens 'x' connections to the server and adds the nfs_connection to the nfs_connections vector.
    // If we are unable to create any connection, then the method should return FALSE to the caller.
    // TODO: See if we want to open only some connections and open other connections when needed.
    //
    bool start();

    //
    // Close all the connections to the server and clean up the structure.
    // TODO: See from where this should be called.
    //
    void close();

    //
    // This is used to Get the connection on which the nfs_client associated with this transport can send the NFS request.
    // For now, this just returns the connection on which the client can send the request.
    // This can be further enhanced to support a good scheduling of the requests over multiple connection.
    //
    struct nfs_context* get_nfs_context();
};

