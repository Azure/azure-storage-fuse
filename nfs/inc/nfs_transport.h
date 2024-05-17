#pragma once

#include "connection.h"

//
// This is the actual transport class.
// This class is responsible for sending the requests out on the connections.
// This has a vector of NfsConnection objects on which the request will be sent out.
// If the mount is done with nconnect=x, then this layer will take care of round robining the
// requests over x connections.
//
class RPCTransport
{
    // This contains all the information needed to create connection to the server.
    struct mountOptions* mntOptions;

    //
    // Vector of NfsConnection. This will be of length nconnect option passed by the user.
    // TODO: Check if we need a lock over this vector.
    //
    std::vector<struct NfsConnection*> nfsConnections;

    //
    // Last context on which the request was sent.
    // Note: Each context is identified from 0 to X (where X is the value of nconnect)
    //
    static int last_context;

    // Make the constructor private as it is singleton.
    RPCTransport(struct mountOptions* mountOptions):
        mntOptions(mountOptions)
    {
        assert(mntOptions != nullptr);
    }

public:

    // This is the method which should be called to get an instance of this class by the user.
    static RPCTransport* GetInstance(struct mountOptions* mntOptions)
    {
        static RPCTransport instance(mntOptions);
        return &instance;
    }

    //
    // This should open the connection(s) to the remote server and initialize all the data members accordingly.
    // This will create 'x' number of NfsConnection objects where 'x' is the nconnect option and call open for each of them.
    // It opens 'x' connections to the server and adds the NfsConnection to the nfsConnections vector.
    // If we are unable to create any connection, then the method should return FALSE to the caller.
    // TODO: See if we want to open only some connections and open other connections when needed.
    //
    bool start()
    {
        for (int i = 0; i <  mntOptions->numConnections; i++)
        {
            struct NfsConnection* connection = new NfsConnection(mntOptions);

            if (!connection->open())
            {
                // Failed to establish the connection.
                // TODO: Destroy open connections if any.
                return false;
            }

            // Add this connection to the vector.
            nfsConnections.push_back(connection);
        }

        return true;
    }

    //
    // Close all the connections to the server and clean up the structure.
    // TODO: See from where this should be called.
    //
    void close()
    {
        for (int i = 0; i <  mntOptions->numConnections; i++)
        {
            struct NfsConnection* connection = nfsConnections[i];
            connection->close();
            delete connection;
        }

        nfsConnections.clear();
    }

    //
    // This is used to Get the connection on which the NFSClient associated with this transport can send the NFS request.
    // For now, this just returns the connection on which the client can send the request.
    // This can be further enhanced to support a good scheduling of the requests over multiple connection.
    //
    struct nfs_context* GetNfsContext()
    {
        return nfsConnections[(last_context++)%(mntOptions->numConnections)]->GetNfsContext();
    }
};

