#include "connection.h"
#include "nfs_client.h"

#include <sys/socket.h>
#include <netinet/in.h>
#include <netinet/tcp.h>

bool nfs_connection::open()
{
    const int nodelay = 1;

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

    nfs_destroy_url(url);

    /*
     * Call libnfs for mounting the share.
     * This will create a connection to the NFS server and perform mount.
     * After this the nfs_context can be used for sending NFS requests.
     */
    int status;
    do {
        status = nfs_mount(nfs_context, mo.server.c_str(),
                           mo.export_path.c_str());
        if (status == -EAGAIN) {
            AZLogWarn("[{}] JUKEBOX error mounting nfs share ({}:{}): {}, "
                      "retrying in 5 secs!",
                      (void *) nfs_context,
                      mo.server,
                      mo.export_path,
                      nfs_get_error(nfs_context));
            ::sleep(5);
            continue;
        } else if (status != 0) {
            AZLogError("[{}] Failed to mount nfs share ({}:{}): {} ({})",
                       (void *) nfs_context,
                       mo.server,
                       mo.export_path,
                       nfs_get_error(nfs_context),
                       status);
            goto destroy_context;
        }
    } while (status == -EAGAIN);

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
        assert(mo.wsize_adj == (int) nfs_get_writemax(nfs_context));
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
     * We must send requests promptly w/o waiting for nagle delay.
     *
     * TODO: Once this is moved to libnfs, it can be removed from here.
     */
    if (::setsockopt(nfs_get_fd(nfs_context), IPPROTO_TCP, TCP_NODELAY,
                     &nodelay, sizeof(nodelay)) != 0) {
        AZLogError("Cannot enable TCP_NODELAY for fd {}: {}",
                   nfs_get_fd(nfs_context), strerror(errno));
        // Let's assert in debug builds and continue o/w.
        assert(0);
    }

    /*
     * libnfs service loop wakes up every poll_timeout msecs to see if there
     * is any request pdu to send. Though lone request pdus are sent in the
     * requester's context, but in some cases it helps for libnfs to check
     * more promptly.
     */
    nfs_set_poll_timeout(nfs_context, 1);

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
