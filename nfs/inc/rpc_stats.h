#ifndef __RPCSTATS_H__
#define __RPCSTATS_H__

#include <atomic>
#include <mutex>

#include "aznfsc.h"
#include "libnfs-raw.h"

namespace aznfsc {

/**
 * Stats for a specific RPC type.
 */
struct rpc_opstat
{
    /*
     * How many RPCs of this type completed?
     */
    std::atomic<uint64_t> count = 0;

    /*
     * How many RPCs of this type are issued to libnfs and awaiting completion
     * callback.
     */
    std::atomic<uint64_t> pending = 0;

    /*
     * Cumulative request bytes.
     * This includes the RPC header and payload bytes.
     */
    std::atomic<uint64_t> bytes_sent = 0;

    /*
     * Cumulative response bytes.
     * This includes the RPC header and payload bytes.
     */
    std::atomic<uint64_t> bytes_rcvd = 0;

    /*
     * Cumulative time taken by the server.
     */
    std::atomic<uint64_t> rtt_usec = 0;

    /*
     * Cumulative time spent waiting to be dispatched over the socket,
     * in libnfs.
     */
    std::atomic<uint64_t> dispatch_usec = 0;

    /*
     * Cumulative time taken for request processing.
     * This includes times taken by server and any other delay on the client.
     * Most prominent client delays include:
     * - Time waiting for a free RPC task to be available.
     * - Scheduling delays causing delay in sending the request and processing
     *   the response.
     */
    std::atomic<uint64_t> total_usec = 0;

    /*
     * Error map to store all the errors encountered by the given api.
     * This is guarded by rpc_stats_az::lock.
     */
    std::map<int /*error status*/, uint64_t /*error count*/> error_map;
};

/**
 * Class for maintaining RPC stats.
 * An object of this must be included in rpc_task and user must call designated
 * event handler methods at appropriate times in the life of the RPC task
 * processing.
 */
class rpc_stats_az
{
public:
    rpc_stats_az() = default;

    /**
     * Event handler method to be called right after the RPC is created.
     * start_usec is the time when the fuse request handler was called.
     * It can be different from create time if new RPC creation had to wait
     * as we may have run out of RPC slots.
     */
    void on_rpc_create(enum fuse_opcode _optype, uint64_t start_usec)
    {
        // 0 is not a valid fuse_opcode;
        assert(_optype > 0 && _optype <= FUSE_OPCODE_MAX);

        /*
         * FUSE_RELEASE is sent as FUSE_FLUSH.
         * Also, FUSE_FLUSH is accounted as FUSE_WRITE as both result in WRITE
         * RPCs.
         */
        assert(_optype != FUSE_RELEASE);
        if (_optype == FUSE_FLUSH) {
            _optype = FUSE_WRITE;
        }

        optype = _optype;
        stamp.start = start_usec;
        stamp.create = get_current_usecs();
        stamp.issue = 0;
        stamp.dispatch = 0;
        stamp.complete = 0;
        stamp.free = 0;
        req_size = 0;
        resp_size = 0;

        assert(stamp.create >= stamp.start);
    }

    /**
     * Event handler method to be called when the RPC request is issued, i.e.,
     * when the libnfs async method is called.
     * Call it just before the libnfs async method. If the async method fails
     * to issue, call on_rpc_cancel() to undo what this does.
     *
     * Note: Callback can get called before the async method returns to the
     *       caller.
     */
    void on_rpc_issue()
    {
        assert(stamp.issue == 0);
        stamp.issue = get_current_usecs();
        assert(stamp.issue >= stamp.create);

        assert(optype > 0 && optype <= FUSE_OPCODE_MAX);
        opstats[optype].pending++;
    }

    /**
     * Call this to undo the effects of on_rpc_issue() if the async request
     * fails to issue for some reason.
     */
    void on_rpc_cancel()
    {
        assert(stamp.issue != 0);
        assert((int64_t) stamp.issue <= get_current_usecs());
        assert(stamp.dispatch == 0);
        assert(stamp.complete == 0);
        stamp.issue = 0;

        assert(optype > 0 && optype <= FUSE_OPCODE_MAX);
        assert(opstats[optype].pending > 0);
        opstats[optype].pending--;
    }

    /**
     * Event handler method to be called when the RPC completes, i.e., when
     * the libnfs callback is called.
     * This MUST be called *only* from the libnfs callback, since only there
     * the pdu is valid and we can call the rpc_pdu_get_*() methods.
     */
    void on_rpc_complete(struct rpc_pdu *pdu, nfsstat3 status)
    {
        assert(nfsstat3_to_errno(status) != -ERANGE);

        req_size = rpc_pdu_get_req_size(pdu);
        // 40 is the size of NFS NULL RPC request.
        assert(req_size >= 40);

        resp_size = rpc_pdu_get_resp_size(pdu);
        // 24 is the size of NFS NULL RPC response.
        assert(resp_size >= 24);

        assert(stamp.dispatch == 0);
        assert(stamp.issue != 0);
        stamp.dispatch = rpc_pdu_get_dispatch_usecs(pdu);
        assert(stamp.dispatch >= stamp.issue);

        stamp.complete = get_current_usecs();
        assert(stamp.complete > stamp.dispatch);

        assert(optype > 0 && optype <= FUSE_OPCODE_MAX);
        assert(opstats[optype].pending > 0);
        opstats[optype].pending--;

        if (status != NFS3_OK) {
            /*
             * This thread will block till it obtains the lock.
             * This can result in delayed response to the fuse as
             * on_rpc_complete will be called before sending response to fuse.
             * This should be okay as this will happen only in error state.
             */
            std::unique_lock<std::mutex> _lock(lock);
            auto result = opstats[optype].error_map.emplace(status, 1);
            if (!result.second) {
                // If the key already exists, increment the error count.
                ++(result.first->second);
            }
        }
    }

    /**
     * Event handler method to be called right before the RPC is freed.
     */
    void on_rpc_free()
    {
        /*
         * stamp.issue won't be set for requests which were not sent to
         * the server. Most likely reason is that the request was served from
         * the cache.
         * stamp.complete won't be set (while stamp.issue is set) for
         * requests which don't get a response. Even those we don't count for
         * stats.
         */
        if (stamp.issue != 0 && stamp.complete != 0) {
            assert(stamp.complete > stamp.dispatch);
            assert(stamp.dispatch >= stamp.issue);

            stamp.free = get_current_usecs();

            assert(optype > 0 && optype <= FUSE_OPCODE_MAX);
            opstats[optype].count++;
            opstats[optype].bytes_sent += req_size;
            opstats[optype].bytes_rcvd += resp_size;

            opstats[optype].rtt_usec += (stamp.complete - stamp.dispatch);
            opstats[optype].dispatch_usec += (stamp.dispatch - stamp.issue);
            opstats[optype].total_usec += (stamp.complete - stamp.start);
        } else if (stamp.issue == 0) {
            /*
             * Requests not issued.
             * See skip_mtime_update() for how we can come here for
             * FUSE_SETATTR.
             */
            assert(stamp.dispatch == 0);
            assert(stamp.complete == 0);
            assert(optype == FUSE_READDIR ||
                   optype == FUSE_READDIRPLUS ||
                   optype == FUSE_READ ||
                   optype == FUSE_WRITE ||
                   optype == FUSE_GETATTR ||
                   optype == FUSE_LOOKUP ||
                   optype == FUSE_SETATTR);
        } else {
            /*
             * Requests issued but not completed.
             */
            assert(stamp.issue != 0);
            assert(stamp.dispatch == 0);
            assert(stamp.complete == 0);

            AZLogWarn("Didn't get response for RPC request type {}", (int) optype);
        }
    }

    /**
     * TODO: See if we need to track retries.
     */
    void on_rpc_retry(int num_retries);

    /**
     * Dump the cumulative stats collected till now.
     * Note that it tries to mimic the o/p of the Linux mountstats(8) command
     * for better readability.
     */
    static void dump_stats();

private:
    enum fuse_opcode optype = (fuse_opcode) 0;
    size_t req_size = 0;
    size_t resp_size = 0;

    /*
     * Timestamp in microseconds for various stages of the RPC.
     *
     * start:     When the alloc_rpc_task() was called.
     * create:    When the rpc_task was actually created.
     *            Note that if we run out of rpc_tasks then alloc_rpc_task()
     *            will have to wait for some ongoing RPC to complete.
     * issue:     When the libnfs async method is called.
     * dispatch:  When libnfs successfully writes the complete request along
     *            with data (if any) over the socket.
     *            Note that due to TCP connection b/w and sndbuf size, this
     *            may be much later than the issue time.
     *            (dispatch - issue) is the time the request was queued in
     *            libnfs waiting to be dispatched over the socket.
     * complete:  When the libnfs async method completes and the callback is
     *            called. (complete - dispatch) is the time taken by the
     *            server to process the RPC.
     * free:      When free_rpc_task() was called.
     */
    struct {
        uint64_t start = 0;
        uint64_t create = 0;
        uint64_t issue = 0;
        uint64_t dispatch = 0;
        uint64_t complete = 0;
        uint64_t free = 0;
    } stamp;

    /*
     * Aggregated per-RPC-type stats, for all RPCs issued of a given type.
     */
    static struct rpc_opstat opstats[FUSE_OPCODE_MAX + 1];

    /*
     * Lock for synchronizing dumping stats and for inserting into error_map.
     */
    static std::mutex lock;

public:
    /*
     * Misc global stats.
     * tot_bytes_read: Total bytes read by the application.
     * bytes_read_from_cache: How many bytes were read from the cache.
     *                        This will indicate our readahead effectiveness.
     * bytes_read_ahead: How many bytes were read ahead.
     * tot_getattr_reqs: How many getattr requests were received from fuse.
     * getattr_served_from_cache: How many were served from inode->attr cache.
     */
    static std::atomic<uint64_t> tot_bytes_read;
    static std::atomic<uint64_t> bytes_read_from_cache;
    static std::atomic<uint64_t> bytes_read_ahead;
    static std::atomic<uint64_t> tot_getattr_reqs;
    static std::atomic<uint64_t> getattr_served_from_cache;
    static std::atomic<uint64_t> tot_lookup_reqs;
    static std::atomic<uint64_t> lookup_served_from_cache;
    static std::atomic<uint64_t> inline_writes;
};

#define INC_GBL_STATS(var, inc)  rpc_stats_az::var += (inc)
#define GET_GBL_STATS(var)       rpc_stats_az::var

}

#endif /* __RPCSTATS_H__ */
