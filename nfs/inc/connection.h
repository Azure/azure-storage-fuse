#pragma once

#include"nfs_internal.h"
#include "log.h"
#include "aznfsc.h"

class NfsConnection
{
private:
    // The mount options struct which has info of the server and export to connect to.
    struct mountOptions* mntOptions;

    /*
     * nfs_context structure on which the actual API operation happens.
     * This is initialized when the cinnection is started.
     */
    struct nfs_context* nfsContext;

public:
    NfsConnection(struct mountOptions* mntOpt):
        mntOptions(mntOpt)
    {
    }

    ~NfsConnection()
    {
        //We will not close the connection, we expect the caller to close this.
    }

    struct nfs_context* GetNfsContext()
    {
        return nfsContext;
    }

    /*
     * This should open the connection to the server.
     * It should init the nfsContext, make a mount call and start a poll loop on those by calling nfs_mt_service_thread_start(ctx)
     * This will return false if we fail to open the connection.
     */
    bool open()
    {
        nfsContext = nfs_init_context();

        if (nfsContext == nullptr)
        {
            AZLogError("Failed to init the Nfs context");
            return false;
        }

        nfs_set_mountport(nfsContext, mntOptions->GetMountPort());
        nfs_set_nfsport(nfsContext, mntOptions->GetPort());

        nfs_set_writemax(nfsContext, mntOptions->GetWriteMax());
        nfs_set_readmax(nfsContext, mntOptions->GetReadMax());

        // Mount the share so that the connection is established.
        if (nfs_mount(nfsContext,  mntOptions->server.c_str(),  mntOptions->exportPath.c_str()) != 0)
        {
            AZLogError("Failed to mount the nfs share, Error: {}", nfs_get_error(nfsContext));
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
        if (nfs_mt_service_thread_start(nfsContext))
        {
            AZLogError("Failed to start nfs service thread.");
            return false;
        }

        return true;
    }

    // Close the connections to the server and clean up the structure.
    void close()
    {
        nfs_mt_service_thread_stop(nfsContext);
        nfs_destroy_context(nfsContext);
    }
};
