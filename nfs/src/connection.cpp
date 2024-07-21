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

    struct mount_options& mo = client->mnt_options;
    const std::string url_str = mo.get_url_str();

    AZLogDebug("Parsing NFS URL string: {}", url_str);

    struct nfs_url *url = nfs_parse_url_full(nfs_context, url_str.c_str());
    if (url == NULL) {
        AZLogError("Failed to parse nfs url {}", url_str);
        goto destroy_context;
    }

    assert(mo.server == url->server);
    assert(mo.export_path == url->path);

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

    /*
     * A successful mount must have negotiated valid values for these.
     */
    assert(nfs_get_readmax(nfs_context) >= AZNFSCFG_RSIZE_MIN);
    assert(nfs_get_readmax(nfs_context) <= AZNFSCFG_RSIZE_MAX);

    assert(nfs_get_writemax(nfs_context) >= AZNFSCFG_WSIZE_MIN);
    assert(nfs_get_writemax(nfs_context) <= AZNFSCFG_WSIZE_MAX);

    assert(nfs_get_readdir_maxcount(nfs_context) >= AZNFSCFG_READDIR_MIN);
    assert(nfs_get_readdir_maxcount(nfs_context) <= AZNFSCFG_READDIR_MAX);

    /*
     * Save the final negotiated value in mount_options for future ref.
     */
    if (mo.rsize_adj == 0) {
        mo.rsize_adj = nfs_get_readmax(nfs_context);
    } else {
        // All connections must have the same negotiated value.
        assert(mo.rsize_adj == (int) nfs_get_readmax(nfs_context));
    }

    if (mo.wsize_adj == 0) {
        mo.wsize_adj = nfs_get_writemax(nfs_context);
    } else {
        // All connections must have the same negotiated value.
        assert(mo.wsize_adj == (int) nfs_get_readmax(nfs_context));
    }

    if (mo.readdir_maxcount_adj == 0) {
        mo.readdir_maxcount_adj = nfs_get_readdir_maxcount(nfs_context);
    } else {
        // All connections must have the same negotiated value.
        assert(mo.readdir_maxcount_adj ==
               (int) nfs_get_readdir_maxcount(nfs_context));
    }

    AZLogInfo("[{}] Successfully mounted nfs share ({}:{}). "
              "Negotiated values: readmax={}, writemax={}, readdirmax={}",
              (void *) nfs_context,
              mo.server,
              mo.export_path,
              nfs_get_readmax(nfs_context),
              nfs_get_writemax(nfs_context),
              nfs_get_readdir_maxcount(nfs_context));

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
