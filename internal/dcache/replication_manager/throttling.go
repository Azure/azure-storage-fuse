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
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache/rpc/gen-go/dcache/models"
)

//go:generate $ASSERT_REMOVER $GOFILE

const (
	// Max number of RVs (and MVs) supported, in the cluster.
	// Note: This is a static limit to avoid using a global lock.
	staticMaxRVs = 10000
)

// Congestion related information for an MV.
// Server sends its current qsize in the PutChunkDC response (for each RV) to convey the RV's congestion
// status. We use it to control our congestion window, i.e., the number of requests we will keep outstanding
// to the MV before we wait for some of them to complete, thus making sure we don't overload an already congested
// MV replica or the network path to it, while making better use of our egress n/w bandwidth to send writes
// to MV replicas which are not/less congested.
type mvCongInfo struct {
	mu        sync.Mutex
	mvName    string       // Name of this MV, e.g., "mv0", "mv1", etc.
	inflight  atomic.Int64 // PutChunkDC requests in flight to this MV.
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
// Safe to call from init().
func initCongInfo() {
	// Initialize only once.
	common.Assert(mvCnginfo == nil)

	mvCnginfo = make([]mvCongInfo, staticMaxRVs)

	for i := range mvCnginfo {
		// We need at least one PutChunkDC request to query/gauge the MV's congestion status.
		mvCnginfo[i].cwnd.Store(1)
		mvCnginfo[i].mvName = fmt.Sprintf("mv%d", i)
		mvCnginfo[i].minRTT.Store(int64(time.Hour)) // Large initial value.
	}
}

// Return mvCongInfo for the given MV name.
func getMVCongInfo(mvName string) *mvCongInfo {
	var mvIdx int
	_, err := fmt.Sscanf(mvName, "mv%d", &mvIdx)
	_ = err
	common.Assert(err == nil, mvName, err)
	common.Assert(mvIdx >= 0 && mvIdx < len(mvCnginfo), mvName, mvIdx, len(mvCnginfo))

	return &(mvCnginfo[mvIdx])
}

// Call this before sending a PutChunkDC request to the given MV.
func (mvci *mvCongInfo) incInflight() {
	mvci.inflight.Add(1)
}

// Call this when a PutChunkDC request to the given MV completes (success or failure).
func (mvci *mvCongInfo) decInflight() {
	common.Assert(mvci.inflight.Load() > 0, mvci.mvName, mvci.inflight.Load())
	mvci.inflight.Add(-1)
}

// Perform admission control before sending a PutChunkDC request to this MV.
// It waits if the number of inflight requests is more than the congestion window size.
func (mvci *mvCongInfo) admit() {
	if mvci.inflight.Add(1) <= mvci.cwnd.Load() {
		return
	}

	mvci.inflight.Add(-1)

	//
	// If inflight requests are more than cwnd, wait for enough inflight requests to complete.
	// We grow the cwnd as more requests complete successfully, proving that the MV can handle
	// more requests. If the MV is slow, the cwnd will cause us to exercise sender flow control.
	// Note that this is the same idea as TCP cwnd used for sender side flow control, but unlike
	// TCP where cwnd starts at a small +ve number, we start at 0, allowing just one request to
	// be sent for an MV and only when that completes successfully (proving MV is not congested),
	// we increase the cwnd and allow more requests to be sent to that MV.
	//
	// TODO: This log will be emitted for first few requests to an MV after a new file is opened,
	//       as cwnd starts at 0. We leave it around as it prints useful info about congested MVs.
	//       Remove it once we have tested it enough.
	//
	log.Warn("ReplicationManager::WriteMV: %s inflight(%d) > cwnd(%d), estQSize: %d, lastRTT: %s, minRTT: %s, maxRTT: %s, timeSinceLastRTT: %s",
		mvci.mvName, mvci.inflight.Load(), mvci.cwnd.Load(),
		mvci.estQSize.Load(), time.Duration(mvci.lastRTT.Load()),
		time.Duration(mvci.minRTT.Load()), time.Duration(mvci.maxRTT.Load()),
		time.Since(time.Unix(0, mvci.lastRTTAt.Load())))

	waitLoop := int64(0)
	for {
		if mvci.inflight.Add(1) <= mvci.cwnd.Load() {
			return
		}
		mvci.inflight.Add(-1)
		time.Sleep(1 * time.Millisecond)
		waitLoop++
		common.Assert(waitLoop < 30000, mvci.mvName, mvci.inflight.Load(), mvci.cwnd.Load())
	}
}

func (mvci *mvCongInfo) done() {
	common.Assert(mvci.inflight.Load() > 0, mvci.mvName, mvci.inflight.Load())
	mvci.inflight.Add(-1)
}

func (mvci *mvCongInfo) onPutChunkDCSuccess(rtt time.Duration,
	req *WriteMvRequest, putChunkDCResp *models.PutChunkDCResponse) {
	//
	// Qsize of an MV is the largest qsize among all its component RVs.
	//
	estQSize := -1
	for rvName, putChunkResp := range putChunkDCResp.Responses {
		_ = rvName
		if putChunkResp.Error == nil {
			log.Debug("ReplicationManager::writeMVInternal: PutChunkDC response from %s/%s, chunkIdx: %d, qsize: %d",
				rvName, req.MvName, req.ChunkIndex, putChunkResp.Response.Qsize)

			common.Assert(putChunkResp.Response.Qsize >= 0, putChunkResp.Response.Qsize, rvName, req.MvName)
			estQSize = max(estQSize, int(putChunkResp.Response.Qsize))
		}
	}

	if estQSize >= 0 {
		// 10K is an arbitrary large value, we shouldn't be seeing such high values due to client throttling.
		common.Assert(estQSize < 10000, estQSize, putChunkDCResp)

		mvci.mu.Lock()

		mvci.lastRTT.Store(int64(rtt))
		mvci.lastRTTAt.Store(time.Now().UnixNano())

		if int64(rtt) > mvci.maxRTT.Load() {
			mvci.maxRTT.Store(int64(rtt))
		}

		if int64(rtt) < mvci.minRTT.Load() || mvci.minRTT.Load() == int64(0) {
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
		doSleep := false
		waitMsec := int64(0)

		if estQSize < 30 {
			//
			// Feed more if RV is "free".
			// Keeping more han 8 requests in flight to an MV doeesn't help, we'd rather send requests
			// to other less loaded MVs and make better use of cluster n/w bandwidth.
			//
			if mvci.cwnd.Add(1) > 8 {
				mvci.cwnd.Store(8)
			}
		} else if estQSize < 100 {
			//
			// Let requests trickle through if RV is "moderately loaded".
			// We need to be conservative here as multiple nodes may be writing to the same MV/RV
			// and that can cause the queue to build up quickly.
			//
			newCwnd := max(mvci.cwnd.Load()/2, 1)
			mvci.cwnd.Store(newCwnd)
		} else {
			//
			// If heavily loaded, then slow down, don't allow any more requests to this
			// MV till all inflight requests are done.
			//
			mvci.cwnd.Store(0)
			doSleep = true

			if estQSize > 200 {
				waitMsec = int64((estQSize * 4) / 2)
			}
		}

		mvci.mu.Unlock()

		if doSleep {
			// Wait for all inflight requests to complete.
			waitLoop := int64(0)
			common.Assert(mvci.inflight.Load() > 0, mvci.mvName, mvci.inflight.Load(), mvci.cwnd.Load())
			for mvci.inflight.Load()-1 > mvci.cwnd.Load() {
				time.Sleep(1 * time.Millisecond)
				waitMsec--
				waitLoop++
				common.Assert(waitLoop < 30000, req.MvName, mvci.inflight.Load(), mvci.cwnd.Load())
			}

			if waitMsec > 0 {
				time.Sleep(time.Duration(waitMsec) * time.Millisecond)
			}

			// Now open up slowly.
			mvci.cwnd.Store(1)
		}
	}
}

func init() {
	initCongInfo()
}

// Silence unused import errors for release builds.
func init() {
	common.IsValidUUID("00000000-0000-0000-0000-000000000000")
	log.Info("")
	fmt.Printf("")
}
