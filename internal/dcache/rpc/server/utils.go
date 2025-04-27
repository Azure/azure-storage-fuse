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
	"fmt"
	"os"
	"path/filepath"

	"github.com/Azure/azure-storage-fuse/v2/common/log"
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

// check if the component RVs are the same
// the list is sorted before comparison
func isComponentRVsValid(rv1 []string, rv2 []string) bool {
	if len(rv1) != len(rv2) {
		return false
	}

	for i := 0; i < len(rv1); i++ {
		// TODO: discuss if the state of RV also needs to be compared
		// The RV array can be like ["rv0", "rv5=syncing", "rv9=outofsync"]
		// RV array can be like ["rv0", "rv5=syncing", "rv9=outofsync"]
		// s1 := (strings.Split(rv1[i], "="))[0]
		// s2 := (strings.Split(rv2[i], "="))[0]

		s1 := rv1[i]
		s2 := rv2[i]
		if s1 != s2 {
			return false
		}
	}

	return true
}

// end sync operation will call this method to move all the chunks from the sync MV path to the regular MV path
func moveChunksToRegularMVPath(syncMvPath string, regMvPath string) error {
	entries, err := os.ReadDir(syncMvPath)
	if err != nil {
		log.Err("utils::moveChunksToRegularMVPath: Failed to read directory %s [%v]", syncMvPath, err.Error())
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			log.Warn("utils::moveChunksToRegularMVPath: Skipping directory %s", entry.Name())
			continue
		}

		src := filepath.Join(syncMvPath, entry.Name())
		dest := filepath.Join(regMvPath, entry.Name())

		err = os.Rename(src, dest)
		if err != nil {
			log.Err("utils::moveChunksToRegularMVPath: Failed to move chunk %s to %s [%v]", src, dest, err.Error())
			return err
		}
	}

	return nil
}
