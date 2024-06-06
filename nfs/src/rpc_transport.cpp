#include "rpc_transport.h"
#include "nfs_client.h"

bool rpc_transport::start()
{
    // constructor must have resized the connection vector correctly.
    assert((int) nfs_connections.size() == client->mnt_options.num_connections);

    /*
     * An RPC transport is composed of one or more connections.
     * Starting the transport involves setting up these connections.
     *
     * Note: Currently we create all the connections upfront for simplicity.
     *       We might consider setting up connections when needed and then
     *       destroying them if idle for some time.
     */
    for (int i = 0; i < client->mnt_options.num_connections; i++) {
        struct nfs_connection *connection = new nfs_connection(client, i);

        if (!connection->open()) {
            /*
             * Failed to establish this connection.
             * Destroy all connections created till now, and this connection.
             */
            close();
            delete connection;
            return false;
        }

        /*
         * TODO: assert that rootfh received over each connection is same.
         */

        /*
         * Ok, connection setup properly, add it to the list of connections
         * for this transport.
         */
        assert(nfs_connections[i] == nullptr);
        nfs_connections[i] = connection;
    }

    assert((int) nfs_connections.size() == client->mnt_options.num_connections);

    return true;
}

void rpc_transport::close()
{
    assert((int) nfs_connections.size() == client->mnt_options.num_connections);

    for (int i = 0; i < (int) nfs_connections.size(); i++) {
        struct nfs_connection *connection = nfs_connections[i];
        if (connection != nullptr) {
            connection->close();
            delete connection;
        }
    }

    nfs_connections.clear();
}

/*
 * This function decides which connection should be chosen for sending
 * the current request.
 * TODO: This is round-robined for now, should be modified later.
 */
struct nfs_context *rpc_transport::get_nfs_context() const
{
    return nfs_connections[(last_context++)%(client->mnt_options.num_connections)]->get_nfs_context();
}
