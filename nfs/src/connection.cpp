#include "connection.h"
#include "nfs_client.h"

bool nfs_connection::open()
{
    // open() must be called only for a closed connection.
    assert(nfs_context == nullptr);

    nfs_context = nfs_init_context();
    if (nfs_context == nullptr)
    {
        AZLogError("Failed to init libnfs nfs_context");
        return false;
    }

    nfs_set_mountport(nfs_context, client->mnt_options.get_mount_port());
    nfs_set_nfsport(nfs_context, client->mnt_options.get_port());
    nfs_set_writemax(nfs_context, client->mnt_options.wsize);
    nfs_set_readmax(nfs_context, client->mnt_options.rsize);

    /*
     * Call libnfs for mounting the share.
     * This will create a connection to the NFS server and perform mount.
     * After this the nfs_context can be used for sending NFS requests.
     */
    if (nfs_mount(nfs_context,
                  client->mnt_options.server.c_str(),
                  client->mnt_options.export_path.c_str()) != 0)
    {
        AZLogError("Failed to mount nfs share ({}:{}): {}",
                   client->mnt_options.server,
                   client->mnt_options.export_path,
                   nfs_get_error(nfs_context));
        return false;
    }

    AZLogInfo("Successfully mounted nfs share ({}:{})!",
              client->mnt_options.server,
              client->mnt_options.export_path);

    /*
     * We use libnfs in multithreading mode as we want 1 thread to do the IOs
     * on the nfs context and another thread to service this nfs context to
     * send/recv data over the socket. Hence we must initialize and start the
     * service thread.
     *
     * TODO: See if we should take care of the locking or should we use this multithreading model.
     */
    if (nfs_mt_service_thread_start(nfs_context))
    {
        AZLogError("Failed to start libnfs service thread.");
        return false;
    }

    return true;
}
