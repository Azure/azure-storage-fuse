#include "rpc_transport.h"
#include "nfs_client.h"

bool rpc_transport::start()
{
    for (int i = 0; i < client->mnt_options.num_connections; i++)
    {
        struct nfs_connection *connection = new nfs_connection(client);

        if (!connection->open())
        {
            // Failed to establish the connection.
            // TODO: Destroy open connections if any.
            return false;
        }

        // Add this connection to the vector.
        nfs_connections[i] = connection;
    }

    return true;
}

void rpc_transport::close()
{
    for (int i = 0; i <  client->mnt_options.num_connections; i++)
    {
        struct nfs_connection *connection = nfs_connections[i];
        connection->close();
        delete connection;
    }

    nfs_connections.clear();
}

// This function decides which connection should be chosen for sending
// the current request.
// TODO: This is round-robined for now, should be modified later.
struct nfs_context *rpc_transport::get_nfs_context() const
{
    return nfs_connections[(last_context++)%(client->mnt_options.num_connections)]->get_nfs_context();
}
