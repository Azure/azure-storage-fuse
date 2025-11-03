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

package rpc_server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"syscall"
	"time"
	"unsafe"

	"github.com/shirou/gopsutil/mem"

	"maps"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache"
	cm "github.com/Azure/azure-storage-fuse/v2/internal/dcache/clustermap"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache/rpc"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache/rpc/gen-go/dcache/models"
)

//go:generate $ASSERT_REMOVER $GOFILE

// returns the chunk and hash path for the given fileID and offsetInMB from RV/MV directory as,
// <cache dir>/<mvName>/<fileID>.<offsetInMB>.data and
// <cache dir>/<mvName>/<fileID>.<offsetInMB>.hash
func getChunkAndHashPath(cacheDir string, mvName string, fileID string, offsetInMB int64) (string, string) {
	chunkPath := filepath.Join(cacheDir, mvName, fmt.Sprintf("%v.%v.data", fileID, offsetInMB))
	hashPath := filepath.Join(cacheDir, mvName, fmt.Sprintf("%v.%v.hash", fileID, offsetInMB))
	return chunkPath, hashPath
}

// Sort the component RVs in the MV.
// The RVs are sorted in increasing order of their names.
func sortComponentRVs(rvs []*models.RVNameAndState) {
	sort.Slice(rvs, func(i, j int) bool {
		return rvs[i].Name < rvs[j].Name
	})
}

// Check if the component RVs are the same. The list is sorted before comparison.
// An example of RV array can be like,
//
// [
//
//	{"name":"rv0", "state": "online"},
//	{"name":"rv5", "state": "syncing"},
//	{"name":"rv9", "state": "outofsync"}
//
// ]
//
// checkState boolean flag indicates if the state of the component RVs in the request should be
// matched against the state of the component RVs in the mvInfo data.
func isComponentRVsValid(rvInMV []*models.RVNameAndState, rvInReq []*models.RVNameAndState, checkState bool) error {
	if len(rvInMV) != len(rvInReq) {
		return fmt.Errorf("request component RVs %s is not same as MV component RVs %s",
			rpc.ComponentRVsToString(rvInReq), rpc.ComponentRVsToString(rvInMV))
	}

	sortComponentRVs(rvInReq)

	isValid := true
	for i := 0; i < len(rvInMV); i++ {
		common.Assert(rvInMV[i] != nil)
		common.Assert(rvInReq[i] != nil)

		if rvInMV[i].Name != rvInReq[i].Name || (checkState && rvInMV[i].State != rvInReq[i].State) {
			isValid = false
			break
		}
	}

	if !isValid {
		rvInMvStr := rpc.ComponentRVsToString(rvInMV)
		rvInReqStr := rpc.ComponentRVsToString(rvInReq)
		err := fmt.Errorf("request component RVs %s is not same as MV component RVs %s",
			rvInReqStr, rvInMvStr)
		log.Err("utils::isComponentRVsValid: %v", err)
		return err
	}

	return nil
}

// Check if the RV is present in the component RVs of the MV.
func isRVPresentInMV(rvs []*models.RVNameAndState, rvName string) bool {
	for _, rv := range rvs {
		common.Assert(rv != nil)
		if rv.Name == rvName {
			return true
		}
	}

	return false
}

// create the rvID map from RVs present in the current node
func getRvIDMap(rvs map[string]dcache.RawVolume) map[string]*rvInfo {
	rvIDMap := make(map[string]*rvInfo)

	for rvName, val := range rvs {
		rvInfo := &rvInfo{
			rvID:     val.RvId,
			rvName:   rvName,
			cacheDir: val.LocalCachePath,
		}

		rvIDMap[val.RvId] = rvInfo
	}

	return rvIDMap
}

// This returns the maximum MVsPerRV value that we allow.
// We allow more MVs to be placed per RV in fix-mv than new-mv.
func getMVsPerRV() int64 {
	mvsPerRV := int64(cm.MVsPerRVForFixMV.Load())
	common.Assert(mvsPerRV > 0, mvsPerRV)
	common.Assert(mvsPerRV > int64(cm.MVsPerRVForNewMV), mvsPerRV, cm.MVsPerRVForNewMV)
	return mvsPerRV
}

// Check if any of the RV present in the component RVs has inband-offline state.
func containsInbandOfflineState(componentRVs *[]*models.RVNameAndState) bool {
	for _, rv := range *componentRVs {
		common.Assert(rv != nil)
		if rv.State == string(dcache.StateInbandOffline) {
			return true
		}
	}

	return false
}

// Update the inband-offline state to offline for all the component RVs in the request.
func updateInbandOfflineToOffline(componentRVs *[]*models.RVNameAndState) {
	for _, rv := range *componentRVs {
		common.Assert(rv != nil)
		if rv.State == string(dcache.StateInbandOffline) {
			rv.State = string(dcache.StateOffline)
		}
	}
}

// This method is wrapper for the GetChunk() RPC call. It is used when the both the client and server
// belong to the same node, i.e. the RPC is called locally.
func GetChunkLocal(ctx context.Context, req *models.GetChunkRequest) (*models.GetChunkResponse, error) {
	common.Assert(req != nil)
	common.Assert(req.Address != nil)

	// Caller must not set SenderNodeID, catch misbehaving callers.
	common.Assert(len(req.SenderNodeID) == 0, req.SenderNodeID)
	req.SenderNodeID = rpc.GetMyNodeUUID()

	//
	// This chunk is being read locally without any RPC, so we can set IsLocalRV to true. This is used for
	// taking the decision in the handler to allocate the chunk from the buffer pool instead of initializing
	// a new buffer.
	//
	req.IsLocalRV = true

	common.Assert(handler != nil)

	return handler.GetChunk(ctx, req)
}

// This method is wrapper for the PutChunk() RPC call. It is used when the both the client and server
// belong to the same node, i.e. the RPC is called locally.
func PutChunkLocal(ctx context.Context, req *models.PutChunkRequest) (*models.PutChunkResponse, error) {
	common.Assert(req != nil)
	common.Assert(req.Chunk != nil)
	common.Assert(req.Chunk.Address != nil)

	// Caller must not set SenderNodeID, catch misbehaving callers.
	common.Assert(len(req.SenderNodeID) == 0, req.SenderNodeID)
	req.SenderNodeID = rpc.GetMyNodeUUID()

	common.Assert(handler != nil)

	return handler.PutChunk(ctx, req)
}

// This method is wrapper for the PutChunkDC() RPC call. It is used when the both the client and server
// belong to the same node, i.e. the RPC is called locally.
func PutChunkDCLocal(ctx context.Context, req *models.PutChunkDCRequest) (*models.PutChunkDCResponse, error) {
	common.Assert(req != nil)
	common.Assert(req.Request != nil)
	common.Assert(req.Request.Chunk != nil)
	common.Assert(req.Request.Chunk.Address != nil)
	common.Assert(len(req.NextRVs) > 0)

	// Caller must not set SenderNodeID, catch misbehaving callers.
	common.Assert(len(req.Request.SenderNodeID) == 0, req.Request.SenderNodeID)
	req.Request.SenderNodeID = rpc.GetMyNodeUUID()

	common.Assert(handler != nil)

	resp, err := handler.PutChunkDC(ctx, req)

	return resp, err
}

// This method is wrapper for the GetMVSize() RPC call. It is used when the both the client and server
// belong to the same node, i.e. the RPC is called locally.
func GetMVSizeLocal(ctx context.Context, req *models.GetMVSizeRequest) (*models.GetMVSizeResponse, error) {
	common.Assert(req != nil)

	// Caller must not set SenderNodeID, catch misbehaving callers.
	common.Assert(len(req.SenderNodeID) == 0, req.SenderNodeID)
	req.SenderNodeID = rpc.GetMyNodeUUID()

	common.Assert(handler != nil)

	return handler.GetMVSize(ctx, req)
}

// Get the time when the RV joined this MV and the last write to this RV/MV replica by a PutChunk(sync) request.
// This will be used to determine if there are any stuck sync jobs caused due to source RV going offline.
// For more details see the comments in mvInfo.joinTime and mvInfo.lastSyncWriteTime.
func GetMVJoinAndLastSyncWriteTime(rvName string, mvName string) (int64, int64) {
	common.Assert(cm.IsValidRVName(rvName), rvName)
	common.Assert(cm.IsValidMVName(mvName), mvName)
	common.Assert(handler != nil)

	rvInfo := handler.getRVInfoFromRVName(rvName)
	common.Assert(rvInfo != nil, rvName)

	//
	// It's possible that caller's clustermap is stale and RV is not part of the MV anymore.
	//
	mvInfo := rvInfo.getMVInfo(mvName)
	if mvInfo == nil {
		// Special values to convey rvName/mvName is non-existent.
		return -1, -1
	}

	//
	// Since the RV has joined the MV, the joinTime must be set.
	// Note: time.Now().Unix() is not guaranteed to be monotonic, so following asserts may fail, but
	//       it's rare, so still useful.
	//
	common.Assert(mvInfo.joinTime.Load() > 0 && mvInfo.joinTime.Load() <= time.Now().Unix(),
		rvName, mvName, mvInfo.joinTime.Load(), time.Now().Unix())
	// lastSyncWriteTime can be 0 if there has not been any sync write to this RV/MV replica.
	common.Assert(mvInfo.lastSyncWriteTime.Load() >= 0 && mvInfo.lastSyncWriteTime.Load() <= time.Now().Unix(),
		rvName, mvName, mvInfo.lastSyncWriteTime.Load(), time.Now().Unix())
	// If set, lastSyncWriteTime must be >= joinTime.
	common.Assert(mvInfo.lastSyncWriteTime.Load() == 0 || mvInfo.lastSyncWriteTime.Load() >= mvInfo.joinTime.Load(),
		rvName, mvName, mvInfo.lastSyncWriteTime.Load(), mvInfo.joinTime.Load())

	return mvInfo.joinTime.Load(), mvInfo.lastSyncWriteTime.Load()
}

// Maps are passed as reference in Go. So, if we get the local clustermap reference and update it,
// it can lead to inconsistency. So, as temporary workaround, we are deep copying the map here.
//
// TODO: Check at all places where we pass the clustermap as reference and are updating it.
//       Check the best way to avoid deep copying the map.

func deepCopyRVMap(rvs map[string]dcache.StateEnum) map[string]dcache.StateEnum {
	common.Assert(rvs != nil)

	newRVs := make(map[string]dcache.StateEnum)
	maps.Copy(newRVs, rvs)

	return newRVs
}

// Perform direct IO read if possible, else fallback to buffered read.
// Handle partial reads and any transient errors.
func SafeRead(filePath *string, readOffset int64, data *[]byte, forceBufferedRead bool) (int /* read bytes */, error) {
	var fh *os.File
	var n, fd int
	var err error

	common.Assert(filePath != nil && len(*filePath) > 0)
	common.Assert(data != nil && len(*data) > 0)
	common.Assert(readOffset >= 0)

	readLength := len(*data)

	//
	// Caller must pass data buffer aligned on FS_BLOCK_SIZE, else we have to unnecessarily perform buffered read.
	// Smaller buffers (less than 1MiB) have been seen to be not aligned to FS_BLOCK_SIZE, we exclude
	// those from the assert since those are rare and do not affect performance.
	//
	dataAddr := unsafe.Pointer(&(*data)[0])
	isDataBufferAligned := ((uintptr(dataAddr) % common.FS_BLOCK_SIZE) == 0)
	common.Assert((readLength < 1024*1024) || isDataBufferAligned,
		uintptr(dataAddr), readLength, common.FS_BLOCK_SIZE)

	//
	// Read using buffered IO mode if,
	//   - Caller wants us to force buffered read,
	//   - The requested offset and length is not aligned to file system block size.
	//   - The buffer is not aligned to file system block size.
	//
	if forceBufferedRead ||
		readLength%common.FS_BLOCK_SIZE != 0 ||
		readOffset%common.FS_BLOCK_SIZE != 0 ||
		!isDataBufferAligned {
		// Log if we have to perform buffered read for large reads due to unaligned buffer.
		if !forceBufferedRead && (readLength >= (1024*1024) && !isDataBufferAligned) {
			log.Warn("SafeRead: Performing buffered read for %s, offset: %d, length: %d",
				*filePath, readOffset, readLength)
		}
		goto bufferedRead
	}

	//
	// Direct IO read.
	//
	fd, err = syscall.Open(*filePath, syscall.O_RDONLY|syscall.O_DIRECT, 0)
	if err != nil {
		return -1, fmt.Errorf("failed to open file %s [%w]", *filePath, err)
	}
	defer syscall.Close(fd)

	if readOffset != 0 {
		_, err = syscall.Seek(fd, readOffset, 0)
		if err != nil {
			return -1, fmt.Errorf("failed to seek in file %s at offset %d [%v]",
				*filePath, readOffset, err)
		}
	}

	n, err = syscall.Read(fd, *data)
	if err == nil {
		//
		// Partial reads should be rare, if it happens fallback to the buffered ReadAt() call which will
		// try to read all the requested bytes.
		//
		// TODO: Make sure this is not common path.
		//
		if n != readLength {
			common.Assert(n < readLength, n, readLength, *filePath)
			log.Warn("SafeRead: Partial read (%d of %d), file: %s, offset: %d, falling back to buffered read",
				n, readLength, *filePath, readOffset)
			common.Assert(false, n, readLength, *filePath)
			goto bufferedRead
		}
		return n, nil
	}

	// For EINVAL, fall through to buffered read.
	if !errors.Is(err, syscall.EINVAL) {
		return -1, fmt.Errorf("failed to read file: %s offset: %d [%v]", *filePath, readOffset, err)
	}

	// TODO: Remove this once this is tested sufficiently.
	log.Warn("Direct read failed with EINVAL, performing buffered read, file: %s, offset: %d, err: %v",
		*filePath, readOffset, err)

bufferedRead:
	fh, err = os.Open(*filePath)
	if err != nil {
		return -1, fmt.Errorf("failed to open file %s [%w]", *filePath, err)
	}
	defer fh.Close()

	//
	// When reading metadata chunk, we may read less than requested length and hence EOF will be returned
	// but that's not an error.
	//
	n, err = fh.ReadAt(*data, readOffset)
	if err != nil && err != io.EOF {
		return -1, fmt.Errorf("failed to read file %s at offset %d, readLength: %d [%v]",
			*filePath, readOffset, readLength, err)
	}

	// See comment in readChunkAndHash() why metadata chunk read may return less data than requested.
	common.Assert((n == readLength) ||
		(n > 0 && n < readLength && readLength == dcache.MDChunkSize && err == io.EOF),
		n, readLength, *filePath, err)

	return n, nil
}

func getMemoryInfo() (uint64, uint64, string, error) {
	memStat, err := mem.VirtualMemory()
	if err != nil {
		return 0, 0, "", err
	}

	percentMemUsed := fmt.Sprintf("%.2f%%", memStat.UsedPercent)
	return memStat.Total, memStat.Used, percentMemUsed, nil
}

// Silence unused import errors for release builds.
func init() {
	common.IsValidUUID("00000000-0000-0000-0000-000000000000")
	time.Since(time.Now())
}
