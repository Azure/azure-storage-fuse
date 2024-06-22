#include "connection.h"
#include "nfs_client.h"

bool nfs_connection::open()
{
    // open() must be called only for a closed connection.
    assert(nfs_context == nullptr);

    nfs_context = nfs_init_context();
    if (nfs_context == nullptr) {
        AZLogError("Failed to init libnfs nfs_context");
        return false;
    }

    const struct mount_options& mo = client->mnt_options;
    const std::string url_str = mo.get_url_str();

    AZLogDebug("Parsing NFS URL string: {}", url_str);

    struct nfs_url *url = nfs_parse_url_full(nfs_context, url_str.c_str());
    if (url == NULL) {
        AZLogError("Failed to parse nfs url {}", url_str);
        goto destroy_context;
    }

    assert(mo.server == url->server);
    assert(mo.export_path == url->path);

    nfs_set_writemax(nfs_context, mo.wsize);
    nfs_set_readmax(nfs_context, mo.rsize);

    /*
     * Call libnfs for mounting the share.
     * This will create a connection to the NFS server and perform mount.
     * After this the nfs_context can be used for sending NFS requests.
     */
    if (nfs_mount(nfs_context,
                  mo.server.c_str(),
                  mo.export_path.c_str()) != 0) {
        AZLogError("[{}] Failed to mount nfs share ({}:{}): {}",
                   (void *) nfs_context,
                   mo.server,
                   mo.export_path,
                   nfs_get_error(nfs_context));
        goto destroy_context;
    }

    AZLogInfo("[{}] Successfully mounted nfs share ({}:{})!",
              (void *) nfs_context,
              mo.server,
              mo.export_path);

    /*
     * We use libnfs in multithreading mode as we want 1 thread to do the IOs
     * on the nfs context and another thread to service this nfs context to
     * send/recv data over the socket. Hence we must initialize and start the
     * service thread.
     *
     * TODO: See if we should take care of the locking or should we use this multithreading model.
     */
    if (nfs_mt_service_thread_start(nfs_context)) {
        AZLogError("[{}] Failed to start libnfs service thread.",
                   (void *) nfs_context);
        goto unmount_and_destroy_context;
    }

    return true;

unmount_and_destroy_context:
    nfs_umount(nfs_context);
destroy_context:
    nfs_destroy_context(nfs_context);
    nfs_context = nullptr;
    return false;
}
