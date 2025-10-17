/*
    _____           _____   _____   ____          ______  _____  ------
   |     |  |      |     | |     | |     |     | |       |            |
   |     |  |      |     | |     | |     |     | |       |            |
   | --- |  |      |     | |-----| |---- |     | |-----| |-----  ------
   |     |  |      |     | |     | |     |     |       | |       |
   | ____|  |_____ | ____| | ____| |     |_____|  _____| |_____  |_____


   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2020-2025 Microsoft Corporation. All rights reserved.
   Author : <blobfusedev@microsoft.com>

   Permission is hereby granted, free of charge, to any person obtaining a copy
   of this software and associated documentation files (the "Software"), to deal
   in the Software without restriction, including without limitation the rights
   to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
   copies of the Software, and to permit persons to whom the Software is
   furnished to do so, subject to the following conditions:

   The above copyright notice and this permission notice shall be included in all
   copies or substantial portions of the Software.

   THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
   IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
   FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
   AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
   LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
   OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
   SOFTWARE
*/

package replication_manager

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	cm "github.com/Azure/azure-storage-fuse/v2/internal/dcache/clustermap"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache/rpc/gen-go/dcache/models"
)

//go:generate $ASSERT_REMOVER $GOFILE

const (
	// Max number of RVs (and MVs) supported, in the cluster.
	// Note: This is a static limit to avoid using a global lock.
	staticMaxRVs = 10000

	// We should not keep more than these many chunks in flight to an MV.
	maxCwnd = 16

	// Set to true to disable congestion control and send as many requests as needed to MVs.
	// Sending uncontrolled requests to an MV can hurt cluster performance, to be used only for testing.
	disableCongestionControl = false

	// Set to true to enable debug logging for congestion control.
	// Helps in understanding how congestion control is working, to see how cwnd is changing, etc.
	// Will generate a lot of logs, so should not be enabled in production.
	// TODO: Remove this once we have tested enough with various cluster sizes and loads.
	debugCongestionControl = false
)

var (
	// Max qsize we want to see for an MV.
	// Various throttling decisions are based on this value.
	// Actual qsize can go little higher than this (but should not go much higher) as each sending node
	// applies throttling independently and may take some time to react to a high qsize value.
	maxQsizeGoal = 0

	// Average time (in msec) to write a chunk at the RV.
	chunkMSecAvg = 0
)

// Congestion related information for an MV.
// Server sends its current qsize in the PutChunkDC response (for each RV) to convey the RV's congestion
// status. We use it to control our congestion window, i.e., the number of requests we will keep outstanding
// to the MV before we wait for some of them to complete, thus making sure we don't overload an already congested
// MV replica or the network path to it, while making better use of our egress n/w bandwidth to send writes
// to MV replicas which are not/less congested.
type mvCongInfo struct {
	mu        sync.Mutex
	mvName    string       // Name of this MV, e.g., "mv0", "mv1", etc. Used for logging only.
	inflight  atomic.Int64 // PutChunkDC requests in flight to this MV.
	admitting atomic.Int64 // Requests currently in admit().
	cwnd      atomic.Int64 // Congestion window size, i.e., total number of inflight requests allowed.
	estQSize  atomic.Int64 // Maximum of the qsize of all component RVs. Debug only.
	minRTT    atomic.Int64 // Minimum RTT observed (in nanoseconds). Debug only.
	maxRTT    atomic.Int64 // Maximum RTT observed (in nanoseconds). Debug only.
	lastRTT   atomic.Int64 // RTT as per the last completed request (in nanoseconds). Debug only.
	lastRTTAt atomic.Int64 // Time when lastRTT was recorded (in nanoseconds since epoch). Debug only.
}

// mvCnginfo stores congestion info for each MV that we have written to.
// Indexed by mv idx.
// Note: We use a static array instead of a map to avoid global lock.
var mvCnginfo []mvCongInfo

// Call once at startup.
// Do not call from init() as cm.ChunkSizeMB may not be initialized then.
func initCongInfo() {
	// Initialize only once.
	common.Assert(mvCnginfo == nil)

	mvCnginfo = make([]mvCongInfo, staticMaxRVs)

	for i := range mvCnginfo {
		//
		// Initial cwnd to a new MV, to which we have not sent any request.
		// This cannot be 0, since we need at least one PutChunkDC request to query/gauge the MV's
		// congestion status.
		//
		mvCnginfo[i].cwnd.Store(2)
		mvCnginfo[i].mvName = fmt.Sprintf("mv%d", i)
		mvCnginfo[i].minRTT.Store(int64(time.Hour)) // Large initial value.
	}

	//
	// We set maxQsizeGoal based on the chunk size, with the following reasoning.
	// Do not keep more than 1s worth of chunks queued up at an MV, assuming disk speed of 4GBps.
	// Note that since senders apply throttling independently, actual qsize can go slightly higher
	// than this which is ok, and that's the reason we aim for only 1s worth of chunks and not more.
	//
	chunkMSecAvg = int(float64(cm.ChunkSizeMB) * float64(0.25))
	chunkMSecAvg = max(chunkMSecAvg, 1) // At least 1 msec.
	maxQsizeGoal = int(float64(1*1000) / float64(chunkMSecAvg))
	maxQsizeGoal = max(maxQsizeGoal, 100) // At least 100.

	log.Info("throttling::initCongInfo: cong_enable: %v, cong_debug: %v, maxQsizeGoal: %d, chunkMSecAvg: %d, chunkSizeMB: %d, maxCwnd: %d, staticMaxRVs: %d",
		!disableCongestionControl, debugCongestionControl, maxQsizeGoal, chunkMSecAvg,
		cm.ChunkSizeMB, maxCwnd, staticMaxRVs)
}

// Return mvCongInfo for the given MV name.
func getMVCongInfo(mvName string) *mvCongInfo {
	var mvIdx int

	_, err := fmt.Sscanf(mvName, "mv%d", &mvIdx)
	_ = err
	common.Assert(err == nil, mvName, err)
	common.Assert(mvIdx >= 0 && mvIdx < len(mvCnginfo), mvName, mvIdx, len(mvCnginfo))
	common.Assert(mvCnginfo[mvIdx].mvName == mvName, mvName, mvCnginfo[mvIdx].mvName, mvIdx)

	return &(mvCnginfo[mvIdx])
}

// Increments the inflight count and returns the new value.
func (mvci *mvCongInfo) incInflight() int64 {
	return mvci.inflight.Add(1)
}

// Decrements the inflight count and returns the new value.
func (mvci *mvCongInfo) decInflight() int64 {
	common.Assert(mvci.inflight.Load() > 0, mvci.mvName, mvci.inflight.Load())
	return mvci.inflight.Add(-1)
}

// Perform admission control before sending a PutChunkDC request to this MV.
// It waits if the number of inflight requests is more than the congestion window size.
// Every request admitted here must grab an inflight count and must eventually call mvci.done()
// when the request is done.
func (mvci *mvCongInfo) admit() {
	if disableCongestionControl {
		return
	}

	//
	// What's the inflight count including this request, as seen by this request.
	// Every request grabs an inflight count before comparing it with cwnd, and those who see
	// and inflight count within cwnd are allowed to proceed, others wait.
	// This way we guarantee that strictly no more than cwnd requests are allowed to proceed.
	//
	inflightCount := mvci.incInflight()

	// Inflight request count including this one is within allowed cwnd, so allow.
	if inflightCount <= mvci.cwnd.Load() {
		if debugCongestionControl {
			log.Info("CWND[0]: %s, estQSize: %d, inflight: %d, cwnd: %d, admitting: %d, no wait!",
				mvci.mvName, mvci.estQSize.Load(), inflightCount, mvci.cwnd.Load(),
				mvci.admitting.Load())
		}
		return
	}

	mvci.admitting.Add(1)

	// onPutChunkDCSuccess() may set cwnd to 0 to indicate that we should drain all inflight requests.
	drainInflight := (mvci.cwnd.Load() == 0)

	if debugCongestionControl {
		log.Info("CWND[1]: %s, estQSize: %d, inflight: %d, cwnd: %d, admitting: %d, need to wait!",
			mvci.mvName, mvci.estQSize.Load(), mvci.inflight.Load(), mvci.cwnd.Load(), mvci.admitting.Load())
	}

	//
	// If inflight requests are more than cwnd, wait for enough inflight requests to complete.
	// We grow the cwnd as more requests complete successfully, proving that the MV can handle
	// more requests. If the MV is slow, the cwnd will cause us to exercise sender flow control.
	// Note that this is the similar idea as TCP cwnd used for sender side flow control to avoid
	// excessive requests at the MV and only sending more requests when the MV is able to handle
	// them.
	//
	// TODO: Make this debug log once we have tested enough with various cluster sizes and loads.
	//
	log.Warn("[SLOW] throttling::admit: %s, inflight(%d) > cwnd(%d), admitting: %d, estQSize: %d, lastRTT: %s, minRTT: %s, maxRTT: %s, timeSinceLastRTT: %s",
		mvci.mvName, mvci.inflight.Load(), mvci.cwnd.Load(), mvci.admitting.Load(),
		mvci.estQSize.Load(), time.Duration(mvci.lastRTT.Load()),
		time.Duration(mvci.minRTT.Load()), time.Duration(mvci.maxRTT.Load()),
		time.Since(time.Unix(0, mvci.lastRTTAt.Load())))

	waitLoop := int64(0)
	for {
		//
		// If cwnd was 0 and we are draining inflight requests, we need to set cwnd to 1
		// once all "true" inflight requests are done to allow new requests to proceed.
		// Since a request grabs an inflight count even before it's admitted, "true" inflight
		// requests are (inflight - admitting).
		//
		if drainInflight && (mvci.inflight.Load()-mvci.admitting.Load() == 0) {
			mvci.cwnd.Store(1)
		}

		// Grab our place in the cwnd.
		if inflightCount <= mvci.cwnd.Load() {
			log.Info("CWND[2]: %s, estQSize: %d, inflight: %d, cwnd: %d, admitting: %d, waited for %d ms!",
				mvci.mvName, mvci.estQSize.Load(), inflightCount, mvci.cwnd.Load(),
				mvci.admitting.Load(), waitLoop)
			mvci.admitting.Add(-1)
			return
		}

		// No request should be waiting for more than 5 secs.
		if waitLoop > 5000 {
			log.Warn("[SLOW] throttling::admit: %s waited too long (%d ms), inflight(%d) > cwnd(%d), admitting: %d, estQSize: %d, lastRTT: %s, minRTT: %s, maxRTT: %s, timeSinceLastRTT: %s",
				mvci.mvName, waitLoop, mvci.inflight.Load(), mvci.cwnd.Load(), mvci.admitting.Load(),
				mvci.estQSize.Load(), time.Duration(mvci.lastRTT.Load()),
				time.Duration(mvci.minRTT.Load()), time.Duration(mvci.maxRTT.Load()),
				time.Since(time.Unix(0, mvci.lastRTTAt.Load())))

			if debugCongestionControl {
				log.Info("CWND[3]: %s, estQSize: %d, inflight: %d, cwnd: %d, admitting: %d, waited for waitLoop: %d!",
					mvci.mvName, mvci.estQSize.Load(), mvci.inflight.Load(), mvci.cwnd.Load(),
					mvci.admitting.Load(), waitLoop)
			}

			mvci.admitting.Add(-1)
			return
		}

		//
		// No luck yet, drop the inflight count, wait for a msec and try again.
		// Decrement admitting before inflight to correctly drain inflight requests in the above logic.
		//
		mvci.admitting.Add(-1)
		mvci.decInflight()
		time.Sleep(1 * time.Millisecond)
		inflightCount = mvci.incInflight()
		mvci.admitting.Add(1)
		waitLoop++
	}
}

// Call it for all requests admitted via mvci.admit() above.
func (mvci *mvCongInfo) done() {
	if disableCongestionControl {
		return
	}

	// Must have the inflight count grabbed in admit().
	common.Assert(mvci.inflight.Load() > 0, mvci.mvName, mvci.inflight.Load())
	mvci.decInflight()
}

// Call this when a PutChunkDC request to this MV (admitted by admit()) completes successfully.
func (mvci *mvCongInfo) onPutChunkDCSuccess(rtt time.Duration,
	req *WriteMvRequest, putChunkDCResp *models.PutChunkDCResponse) {

	if disableCongestionControl {
		return
	}

	//
	// Qsize of an MV is the largest qsize of all its component RVs.
	//
	estQSize := -1
	for rvName, putChunkResp := range putChunkDCResp.Responses {
		_ = rvName
		//
		// In case of an error, reset cwnd to 1 and start probing afresh.
		//
		if putChunkResp.Error != nil {
			log.Warn("throttling::onPutChunkDCSuccess: PutChunkDC failed for %s/%s, chunkIdx: %d, forcing cwnd=1: %v",
				rvName, req.MvName, req.ChunkIndex, putChunkResp.Error)

			mvci.cwnd.Store(1)

			if debugCongestionControl {
				log.Info("CWND[4]: %s, estQSize: %d, inflight: %d, cwnd: %d",
					mvci.mvName, mvci.estQSize.Load(), mvci.inflight.Load(), mvci.cwnd.Load())
			}

			return
		}

		log.Debug("throttling::onPutChunkDCSuccess: PutChunkDC response for %s/%s, chunkIdx: %d, qsize: %d",
			rvName, req.MvName, req.ChunkIndex, putChunkResp.Response.Qsize)

		common.Assert(putChunkResp.Response.Qsize >= 0, putChunkResp.Response.Qsize, rvName, req.MvName)
		estQSize = max(estQSize, int(putChunkResp.Response.Qsize))
	}

	common.Assert(estQSize >= 0, estQSize, putChunkDCResp)

	// 10K is an arbitrary large value, we shouldn't be seeing such high values due to client throttling.
	common.Assert(estQSize < 10000, estQSize, putChunkDCResp)

	mvci.mu.Lock()

	mvci.lastRTT.Store(int64(rtt))
	mvci.lastRTTAt.Store(time.Now().UnixNano())

	if int64(rtt) > mvci.maxRTT.Load() {
		mvci.maxRTT.Store(int64(rtt))
	}

	if int64(rtt) < mvci.minRTT.Load() {
		mvci.minRTT.Store(int64(rtt))
	}

	//
	// minRTT is an estimate of the fastest path to the MV (network + storage).
	// If current RTT is significantly higher than minRTT, it indicates n/w congestion
	// which may not reflect in the qsize value returned by the server.
	// Set estQSize to 10 so that the following code limits the cwnd to 1 and then as
	// things improve we increase the cwnd.
	//
	/*
		if int64(rtt) >= mvci.minRTT.Load()*5 {
			estQSize = max(estQSize, 10)
		}
	*/

	mvci.estQSize.Store(int64(estQSize))

	// < 20% of maxQsizeGoal, so RV is "free".
	if estQSize < maxQsizeGoal/5 {
		//
		// Feed more if RV is "free", growing cwnd exponentially but capped to maxCwnd.
		// Instead of wasting n/w resources for sending writes to an MV that's loaded, we'd rather
		// send requests to other less loaded MVs and make better use of cluster n/w bandwidth.
		//
		if mvci.cwnd.Load()*2 > maxCwnd {
			mvci.cwnd.Store(maxCwnd)
		} else {
			mvci.cwnd.Store(mvci.cwnd.Load() * 2)
		}

		if debugCongestionControl {
			log.Info("CWND[5]: %s, chunkIdx: %d, estQSize: %d, inflight: %d, cwnd: %d",
				req.MvName, req.ChunkIndex, estQSize, mvci.inflight.Load(), mvci.cwnd.Load())
		}
	} else if estQSize < ((maxQsizeGoal * 2) / 5) {
		//
		// 20% to 40% of maxQsizeGoal, so RV is "moderately loaded". It's not swamped with requests,
		// so all requests complete quickly enough and it's not idle either.
		// This means our current cwnd is likely ok, so don't change it.
		// We would like to keep MV in this state as long as possible.
		//
		if debugCongestionControl {
			log.Info("CWND[6]: %s, chunkIdx: %d, estQSize: %d, inflight: %d, cwnd: %d",
				req.MvName, req.ChunkIndex, estQSize, mvci.inflight.Load(), mvci.cwnd.Load())
		}
	} else if estQSize < ((maxQsizeGoal * 3) / 5) {
		//
		// 40% to 60% of maxQsizeGoal, RV is getting "warm", start slow down but not very aggressively.
		//
		newCwnd := max(mvci.cwnd.Load()-1, 1)
		mvci.cwnd.Store(newCwnd)

		if debugCongestionControl {
			log.Info("CWND[7]: %s, chunkIdx: %d, estQSize: %d, inflight: %d, cwnd: %d",
				req.MvName, req.ChunkIndex, estQSize, mvci.inflight.Load(), mvci.cwnd.Load())
		}
	} else if estQSize < ((maxQsizeGoal * 4) / 5) {
		//
		// 60% to 80% of maxQsizeGoal, RV is getting "hot", start slowing down more aggressively.
		// We need to be conservative here as multiple nodes may be writing to the same MV/RV
		// and that can cause the queue to build up rapidly.
		//
		mvci.cwnd.Store(mvci.cwnd.Load() / 2)

		if debugCongestionControl {
			log.Info("CWND[8]: %s, chunkIdx: %d, estQSize: %d, inflight: %d, cwnd: %d",
				req.MvName, req.ChunkIndex, estQSize, mvci.inflight.Load(), mvci.cwnd.Load())
		}
	} else {
		//
		// > 80% of maxQsizeGoal, RV is "heavily loaded", wait for all inflight requests to complete
		// before sending more.
		//
		mvci.cwnd.Store(0)

		if debugCongestionControl {
			log.Info("CWND[9]: %s, chunkIdx: %d, estQSize: %d, inflight: %d, cwnd: %d",
				req.MvName, req.ChunkIndex, estQSize, mvci.inflight.Load(), mvci.cwnd.Load())
		}
	}

	mvci.mu.Unlock()
}

// Silence unused import errors for release builds.
func init() {
	common.IsValidUUID("00000000-0000-0000-0000-000000000000")
	log.Info("")
	fmt.Printf("")
}
