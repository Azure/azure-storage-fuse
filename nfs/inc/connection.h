#pragma once

#include "nfs_internal.h"
#include "log.h"
#include "aznfsc.h"

class nfs_connection
{
private:
    // The mount options struct which has info of the server and export to connect to.
    struct mount_options* mnt_options;

    /*
     * nfs_context structure on which the actual API operation happens.
     * This is initialized when the cinnection is started.
     */
    struct nfs_context* nfs_context;

public:
    nfs_connection(struct mount_options* mnt_opt):
        mnt_options(mnt_opt)
    {
    }

    ~nfs_connection()
    {
        //We will not close the connection, we expect the caller to close this.
    }

    struct nfs_context* get_nfs_context()
    {
        return nfs_context;
    }

    /*
     * This should open the connection to the server.
     * It should init the nfs_context, make a mount call and start a poll loop on those by calling nfs_mt_service_thread_start(ctx)
     * This will return false if we fail to open the connection.
     */
    bool open();

    // Close the connections to the server and clean up the structure.
    void close()
    {
        nfs_mt_service_thread_stop(nfs_context);
        nfs_destroy_context(nfs_context);
    }
};
