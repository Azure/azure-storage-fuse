#ifndef __RPC_TRANSPORT_H__
#define __RPC_TRANSPORT_H__

#include "aznfsc.h"
#include "connection.h"

/*
 * This represents an RPC transport.
 * An RPC transport is comprised of one or more nfs_connection and uses those
 * to carry RPC requests to the server.
 * This has a vector of nfs_connection objects on which the request will be sent out.
 * If the mount is done with nconnect=x, then this layer will take care of round
 * robining the requests over x connections.
 */
class rpc_transport
{
    /*
     * nfs_client that this transport belongs to.
     */
    struct nfs_client *client = nullptr;

    /*
     * All connections that make the transport.
     * It'll have as many connections as the nconnect config/mount option.
     *
     * TODO: Check if we need a lock over this vector.
     */
    std::vector<struct nfs_connection*> nfs_connections;

    /*
     * Last context on which the request was sent.
     * Note: Each context is identified from 0 to X (where X is the value of nconnect)
     */
    mutable int last_context = 0;

public:
    rpc_transport(struct nfs_client* _client):
        client(_client),
        nfs_connections(aznfsc_cfg.nconnect, nullptr)
    {
        assert(client != nullptr);
    }

    /*
     * This should open the connection(s) to the remote server and initialize
     * all the data members accordingly.
     * This will create 'x' number of nfs_connection objects where 'x' is the
     * nconnect option and call open for each of them.
     * It opens 'x' connections to the server and adds the nfs_connection to
     * the nfs_connections vector.
     * If we are unable to create any connection, then the method will return
     * false.
     *
     * TODO: See if we want to open connections only when needed.
     */
    bool start();

    /*
     * Close all the connections to the server and clean up the structure.
     * TODO: See from where this should be called.
     */
    void close();

    /*
     *
     * This is used to get the connection on which the nfs_client associated
     * with this transport can send the NFS request.
     * For now, this just returns the connection on which the client can send
     * the request.
     * This can be further enhanced to support a good scheduling of the requests
     * over multiple connection.
     */
    struct nfs_context *get_nfs_context() const;
};

#endif /* __RPC_TRANSPORT_H__ */
