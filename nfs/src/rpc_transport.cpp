#include "rpc_transport.h"
#include "nfs_client.h"

#include <thread>
#include <vector>

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
     * Note: We perform all the mounts in parallel else it takes a long time
     *       to startup.
     */
    std::vector<std::thread> vt;
    std::atomic<int> successful_connections = 0;

    for (int i = 0; i < client->mnt_options.num_connections; i++) {
        vt.emplace_back(std::thread([&, i]() {
            AZLogDebug("Starting thread for creating connection #{}", i);

            struct nfs_connection *connection = new nfs_connection(client, i);

            if (!connection->open()) {
                AZLogError("Failed to setup connection #{}", i);

                /*
                 * Failed to establish this connection.
                 * Convey failure to the main thread by storing a nullptr for
                 * this connection. It'll then perform the cleanup.
                 */
                delete connection;

                assert(nfs_connections[i] == nullptr);
                nfs_connections[i] = nullptr;

                return;
            }

            /*
             * TODO: assert that rootfh received over each connection is same.
             */

            /*
             * Ok, connection setup properly, add it to the list of connections
             * for this transport and update number of successful connections,
             * so that caller knows whether all connections were successfully
             * created.
             */
            assert(nfs_connections[i] == nullptr);
            nfs_connections[i] = connection;
            successful_connections++;

            return;
        }));
    }

    /*
     * Now wait for all connections to mount and setup correctly..
     */
    AZLogDebug("Waiting for {} nconnect connection(s) to setup",
               client->mnt_options.num_connections);

    for (auto& t : vt) {
        t.join();
    }

    /*
     * TODO: Continue if we have minimum number of connections?
     */
    if (successful_connections != client->mnt_options.num_connections) {
        AZLogError("One or more connection setup failed, cleaning up!");
        close();
        return false;
    }

    assert((int) nfs_connections.size() == client->mnt_options.num_connections);

    AZLogDebug("Successfully created all {} nconnect connection(s)",
               client->mnt_options.num_connections);
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
struct nfs_context *rpc_transport::get_nfs_context(conn_sched_t csched,
                                                   uint32_t fh_hash) const
{
    uint32_t idx = 0;

    switch (csched) {
        case CONN_SCHED_FIRST:
            idx = 0;
            break;
        case CONN_SCHED_RR:
            idx = (last_context++ % client->mnt_options.num_connections);
            break;
        case CONN_SCHED_FH_HASH:
            assert(fh_hash != 0);
            idx = fh_hash % client->mnt_options.num_connections;
            break;
        default:
            assert(0);
    }

    return nfs_connections[idx]->get_nfs_context();
}
