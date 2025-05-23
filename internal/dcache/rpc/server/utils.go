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
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache"
	cm "github.com/Azure/azure-storage-fuse/v2/internal/dcache/clustermap"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache/rpc"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache/rpc/gen-go/dcache/models"
)

// returns the chunk path and hash path for the given fileID and offsetInMB from the regular MV directory
// If not present, return the path of the sync MV directory
func getChunkAndHashPath(cacheDir string, mvName string, fileID string, offsetInMB int64) (string, string) {
	chunkPath, hashPath := getRegularMVPath(cacheDir, mvName, fileID, offsetInMB)
	_, err := os.Stat(chunkPath)
	if err != nil {
		log.Debug("utils::getChunkAndHashPath: chunk file %s does not exist, returning .sync directory path", chunkPath)
		return getSyncMVPath(cacheDir, mvName, fileID, offsetInMB)
	}

	return chunkPath, hashPath
}

// returns the chunk path and hash path for the given fileID and offsetInMB from regular MV directory
func getRegularMVPath(cacheDir string, mvName string, fileID string, offsetInMB int64) (string, string) {
	chunkPath := filepath.Join(cacheDir, mvName, fmt.Sprintf("%v.%v.data", fileID, offsetInMB))
	hashPath := filepath.Join(cacheDir, mvName, fmt.Sprintf("%v.%v.hash", fileID, offsetInMB))
	return chunkPath, hashPath
}

// returns the chunk path and hash path for the given fileID and offsetInMB from MV.sync directory
func getSyncMVPath(cacheDir string, mvName string, fileID string, offsetInMB int64) (string, string) {
	chunkPath := filepath.Join(cacheDir, mvName+".sync", fmt.Sprintf("%v.%v.data", fileID, offsetInMB))
	hashPath := filepath.Join(cacheDir, mvName+".sync", fmt.Sprintf("%v.%v.hash", fileID, offsetInMB))
	return chunkPath, hashPath
}

// return the chunk address in the format <fileID>-<rvID>-<mvName>-<offsetInMB>
func getChunkAddress(fileID string, rvID string, mvName string, offsetInMB int64) string {
	return fmt.Sprintf("%v-%v-%v-%v", fileID, rvID, mvName, offsetInMB)
}

// sort the component RVs in the MV
// The RVs are sorted in increasing order of their names
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

// end sync operation will call this method to move all the chunks from the sync MV path to the regular MV path
func moveChunksToRegularMVPath(syncMvPath string, regMvPath string) error {
	entries, err := os.ReadDir(syncMvPath)
	if err != nil {
		log.Err("utils::moveChunksToRegularMVPath: Failed to read directory %s [%v]", syncMvPath, err)
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			log.Warn("utils::moveChunksToRegularMVPath: Skipping directory %s", entry.Name())
			// We only save chunks in the .sync folder.
			common.Assert(false)
			continue
		}

		// TODO: Check and assert that all entries are valid chunk names.

		src := filepath.Join(syncMvPath, entry.Name())
		dest := filepath.Join(regMvPath, entry.Name())

		err = os.Rename(src, dest)
		if err != nil {
			log.Err("utils::moveChunksToRegularMVPath: Failed to move chunk %s -> %s [%v]",
				src, dest, err.Error())
			return err
		}

		log.Debug("utils::moveChunksToRegularMVPath: Moved chunk %s -> %s", src, dest)
	}

	return nil
}

// TODO: apart from just populating the rv related info, it should also update the mvinfo.
// For that it'll need to find the mv folders inside the rv, enumerate and stat all chunks
// to find the totalChunkBytes, etc. This is needed when a node with data, restarts

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

// return mvs-per-rv from dcache config
func getMVsPerRV() int64 {
	return int64(cm.GetCacheConfig().MvsPerRv)
}

// When an MV is in degraded state because one or more of its RV went offline,
// the caller (lowest index online RV) can call this method to get the
// size of the MV. The caller will then send JoinMV RPC call to the
// new RVs, passing the size of the MV to them. On basis of this,
// the new RVs will decide if they can join the MV or not.
func GetMyMVSize(mvName string, rvName string) (int64, error) {
	// TODO: should we block the IO operations on the MV while this is happening?
	common.Assert(handler != nil)

	// TODO: replace with IsValidMV and IsValidRV
	common.Assert(cm.IsValidRVName(rvName), rvName)
	common.Assert(cm.IsValidMVName(mvName), mvName)

	resp, err := handler.GetMVSize(context.Background(), &models.GetMVSizeRequest{
		SenderNodeID: rpc.GetMyNodeUUID(),
		MV:           mvName,
		RVName:       rvName,
	})

	if err != nil {
		log.Err("utils::GetMyMVSize: Failed to get MV size for %s/%s [%v]", rvName, mvName, err)
		return 0, err
	}

	return resp.MvSize, nil
}
