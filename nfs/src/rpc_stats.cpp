#include <sys/types.h>
#include <sys/socket.h>
#include <netinet/in.h>
#include <arpa/inet.h>

#include "rpc_stats.h"
#include "rpc_task.h"
#include "nfs_client.h"

namespace aznfsc {

/* static */ struct rpc_opstat rpc_stats_az::opstats[FUSE_OPCODE_MAX + 1];
/* static */ std::mutex rpc_stats_az::lock;

/* static */
void rpc_stats_az::dump_stats()
{
    const struct nfs_client& client = nfs_client::get_instance();
    const struct rpc_transport& transport = client.get_transport();
    const std::vector<struct nfs_connection*> connections =
        transport.get_all_connections();
    const struct mount_options& mo = client.mnt_options;
    const struct sockaddr_storage *saddr = nullptr;
    struct rpc_stats cum_stats = {0};
    std::string str;

    /*
     * Take exclusive lock to avoid mixing dump from simultaneous dump
     * requests.
     */
    std::unique_lock<std::mutex> _lock(lock);

    /*
     * Go over all connections, query libnfs for stats for each and accumulate
     * them.
     */
    for (struct nfs_connection *conn : connections) {
        struct rpc_stats stats;
        struct rpc_context *rpc = nfs_get_rpc_context(conn->get_nfs_context());

        /*
         * All nconnect connections will terminate at the same IPv4 address,
         * so use the one corresponding to the first connection.
         */
        if (!saddr) {
            saddr = nfs_get_server_address(conn->get_nfs_context());
            // Currently Blob NFS only supports IPv4 address.
            assert(((struct sockaddr_in *)saddr)->sin_family == AF_INET);
        }

        rpc_get_stats(rpc, &stats);

#define _CUM(s) cum_stats.s += stats.s
        _CUM(num_req_sent);
        _CUM(num_resp_rcvd);
        _CUM(num_timedout);
        _CUM(num_timedout_in_outqueue);
        _CUM(num_major_timedout);
        _CUM(num_retransmitted);
        _CUM(num_reconnects);
        _CUM(outqueue_len);
        _CUM(waitpdu_len);
#undef _CUM
    }

    str += "---[RPC stats]----------\n";
    str += "Stats for " + mo.server + ":" + mo.export_path +
           "mounted on " + mo.mountpoint + ":\n";
    str += "  NFS mount options:" +
           std::string(mo.readonly ? "ro" : "rw") +
           std::string(",vers=3") +
           ",rsize=" + std::to_string(mo.rsize_adj) +
           ",wsize=" + std::to_string(mo.wsize_adj) +
           ",acregmin=" + std::to_string(mo.acregmin) +
           ",acregmax=" + std::to_string(mo.acregmax) +
           ",acdirmin=" + std::to_string(mo.acdirmin) +
           ",acdirmax=" + std::to_string(mo.acdirmax) +
           std::string(",hard,proto=tcp") +
           ",nconnect=" + std::to_string(mo.num_connections) +
           ",port=" + std::to_string(mo.nfs_port) +
           ",timeo=" + std::to_string(mo.timeo) +
           ",retrans=" + std::to_string(mo.retrans) +
           std::string(",sec=sys") +
           std::string(",xprtsec=none") +
           std::string(",mountaddr=") +
           ::inet_ntoa(((struct sockaddr_in *)saddr)->sin_addr) +
           ",mountport=" + std::to_string(mo.mount_port) +
           std::string(",mountproto=tcp\n");

    str += "RPC statistics:\n";
    str += "  " + std::to_string(cum_stats.num_req_sent) +
                  " RPC requests sent\n";
    str += "  " + std::to_string(cum_stats.num_resp_rcvd) +
                  " RPC replies received\n";
    str += "  " + std::to_string(cum_stats.outqueue_len) +
                  " RPC requests in libnfs outqueue\n";
    str += "  " + std::to_string(cum_stats.waitpdu_len) +
                  " RPC requests in libnfs waitpdu queue\n";
    str += "  " + std::to_string(cum_stats.num_timedout_in_outqueue) +
                  " RPC requests timed out in outqueue\n";
    str += "  " + std::to_string(cum_stats.num_timedout) +
                  " RPC requests timed out waiting for response\n";
    str += "  " + std::to_string(cum_stats.num_major_timedout) +
                  " RPC requests major timed out\n";
    str += "  " + std::to_string(cum_stats.num_retransmitted) +
                  " RPC requests retransmitted\n";
    str += "  " + std::to_string(cum_stats.num_reconnects) +
                  " Reconnect attempts\n";

#define DUMP_OP(opcode) \
do { \
    const auto& ops = opstats[opcode]; \
    if (ops.count != 0) { \
        const std::string opstr = rpc_task::fuse_opcode_to_string(opcode); \
        const int pcent_ops = (ops.count * 100) / cum_stats.num_req_sent; \
        str += opstr + ":\n"; \
        str += "        " + std::to_string(ops.count) + \
                        " ops (" + std::to_string(pcent_ops) + "%)\n"; \
        if (ops.pending > 0) { \
            str += "        " + std::to_string(ops.pending) + \
                            " pending\n"; \
        } \
        str += "        Avg bytes sent per op: " + \
                        std::to_string(ops.bytes_sent / ops.count) + "\n"; \
        str += "        Avg bytes received per op: " + \
                        std::to_string(ops.bytes_rcvd / ops.count) + "\n"; \
        str += "        Avg RTT: " + \
                        std::to_string(ops.rtt_usec / (ops.count * 1000.0)) + \
                        " msec\n"; \
        str += "        Avg dispatch wait: " + \
                        std::to_string(ops.dispatch_usec / (ops.count * 1000.0)) + \
                        " msec\n"; \
        str += "        Avg Total execute time: " + \
                        std::to_string(ops.total_usec / (ops.count * 1000.0)) + \
                        " msec\n"; \
        if (!ops.error_map.empty()) { \
            str += "        Errors encountered: \n"; \
            for (const auto& entry : ops.error_map) { \
                str += "            " + \
                        std::string(nfsstat3_to_str(entry.first)) +  ": " + \
                        std::to_string(entry.second) + "\n"; \
            } \
        } \
    } \
} while (0)

    DUMP_OP(FUSE_LOOKUP);
    DUMP_OP(FUSE_CREATE);
    DUMP_OP(FUSE_MKDIR);
    DUMP_OP(FUSE_GETATTR);
    DUMP_OP(FUSE_SETATTR);
    DUMP_OP(FUSE_RMDIR);
    DUMP_OP(FUSE_UNLINK);
    DUMP_OP(FUSE_READ);
    DUMP_OP(FUSE_WRITE);
    DUMP_OP(FUSE_READDIR);
    DUMP_OP(FUSE_READDIRPLUS);

    /*
     * TODO: Add more ops.
     */

    AZLogWarn("\n{}\n", str.c_str());
}

}
