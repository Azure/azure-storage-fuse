#ifndef __RPC_TRANSPORT_H__
#define __RPC_TRANSPORT_H__

#include "aznfsc.h"
#include "connection.h"

/**
 * Connection scheduling types supported when sending RPCs over multiple
 * nconnect connections. This decides which connection is picked for a given
 * RPC request.
 */
typedef enum
{
    CONN_SCHED_INVALID  = 0,

    /*
     * Always send over the first connection.
     */
    CONN_SCHED_FIRST    = 1,

    /*
     * Round robin requests over all connections.
     */
    CONN_SCHED_RR       = 2,

    /*
     * Every file is affined to one connection based on the FH hash, so all
     * requests to one file go over the same connection while different files
     * will use different connections.
     */
    CONN_SCHED_FH_HASH  = 3,
} conn_sched_t;

/*
 * This represents an RPC transport.
 * An RPC transport is comprised of one or more nfs_connection and uses those
 * to carry RPC requests to the server.
 * This has a vector of nfs_connection objects on which the request will be
 * sent out.
 * If the mount is done with nconnect=x, then this layer will take care of round
 * robining the requests over x connections.
 */

#define RPC_TRANSPORT_MAGIC *((const uint32_t *)"RPCT")

struct rpc_transport
{
    const uint32_t magic = RPC_TRANSPORT_MAGIC;

private:
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
     * Note: Each context is identified from 0 to X-1 (where X is the value of nconnect).
     * Note: We initialize it to UINT32_MAX-1 to force wraparound and catch
     *       if it's not properly handled.
     *
     * Note: Note that this is updated by multiple threads w/o any lock, and
     *       hence TSAN complains, but we are ok with occassional inaccuracy,
     *       in favour of performance.
     *       Following TSAN suppression suppresses it:
     *       - race:rpc_transport::last_context
     */
    mutable uint32_t last_context = UINT32_MAX - 2;

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
     * This is used to get the connection on which the nfs_client associated
     * with this transport can send the NFS request.
     * For now, this just returns the next connection on which the client can
     * send the request.
     * This can be further enhanced to support a good scheduling of the requests
     * over multiple connection.
     *
     * csched:  The connection scheduling type to be used when selecting the
     *          NFS context/connection.
     * fh_hash: Filehandle hash, used only when CONN_SCHED_FH_HASH scheduling
     *          mode is used. This provides a unique hash for the file/dir
     *          that is the target for this request. All requests to the same
     *          file/dir are sent over the same connection.
     */
    struct nfs_context *get_nfs_context(conn_sched_t csched = CONN_SCHED_FIRST,
                                        uint32_t fh_hash = 0) const;

    const std::vector<struct nfs_connection*>& get_all_connections() const
    {
        return nfs_connections;
    }
};

#endif /* __RPC_TRANSPORT_H__ */
