#include "nfs_transport.h"

int rpc_transport::last_context(0);

//
// This should open the connection(s) to the remote server and initialize all the data members accordingly.
// This will create 'x' number of nfs_connection objects where 'x' is the nconnect option and call open for each of them.
// It opens 'x' connections to the server and adds the nfs_connection to the nfs_connections vector.
// If we are unable to create any connection, then the method should return FALSE to the caller.
// TODO: See if we want to open only some connections and open other connections when needed.
//
bool rpc_transport::start()
{
    for (int i = 0; i <  mnt_options->num_connections; i++)
    {
        struct nfs_connection* connection = new nfs_connection(mnt_options);

        if (!connection->open())
        {
            // Failed to establish the connection.
            // TODO: Destroy open connections if any.
            return false;
        }

        // Add this connection to the vector.
        nfs_connections.push_back(connection);
    }

    return true;
}

void rpc_transport::close()
{
    for (int i = 0; i <  mnt_options->num_connections; i++)
    {
        struct nfs_connection* connection = nfs_connections[i];
        connection->close();
        delete connection;
    }

    nfs_connections.clear();
}

struct nfs_context* rpc_transport::get_nfs_context()
{
    return nfs_connections[(last_context++)%(mnt_options->num_connections)]->get_nfs_context();
}

