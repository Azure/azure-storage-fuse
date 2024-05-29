#include "connection.h"

bool nfs_connection::open()
{
    nfs_context = nfs_init_context();

    if (nfs_context == nullptr)
    {
        AZLogError("Failed to init the Nfs context");
        return false;
    }

    nfs_set_mountport(nfs_context, mnt_options->get_mount_port());
    nfs_set_nfsport(nfs_context, mnt_options->get_port());

    nfs_set_writemax(nfs_context, mnt_options->get_write_max());
    nfs_set_readmax(nfs_context, mnt_options->get_read_max());

    // Mount the share so that the connection is established.
    if (nfs_mount(nfs_context,  mnt_options->server.c_str(),  mnt_options->export_path.c_str()) != 0)
    {
        AZLogError("Failed to mount the nfs share, Error: {}", nfs_get_error(nfs_context));
        return false;
    }
    else
    {
        AZLogInfo("Mounted successfully!");
    }

    /*
     * We exploit the libnfs multithreading support as we want 1 thread to do the IOs on the
     * nfs context and other thread ot service this nfs context to send the data over socket.
     * Hence we must initialize and start the service thread.
     * TODO: See if we should take care of the locking or should we use this multithreading model.
     */
    if (nfs_mt_service_thread_start(nfs_context))
    {
        AZLogError("Failed to start nfs service thread.");
        return false;
    }

    return true;
}
