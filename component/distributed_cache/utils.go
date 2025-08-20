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

package distributed_cache

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache"
	gouuid "github.com/google/uuid"
)

//go:generate $ASSERT_REMOVER $GOFILE

func getRVUuid(nodeUUID string, path string) (string, error) {
	// Create or read a deterministic UUID stamped in ".rvId" file inside the top level RV dir.
	// Deterministic UUID is generated from the canonical absolute directory path, not randomly.

	// Canonicalize the path to avoid duplicates due to different path representations.
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("filepath.Abs(%s) failed: %v", path, err)
	}

	path = abs
	uuidFilePath := filepath.Join(path, ".rvid")

	// Try reading existing UUID from the file.
	if data, err := os.ReadFile(uuidFilePath); err == nil {
		rvId := strings.TrimSpace(string(data))
		if common.IsValidUUID(rvId) {
			return rvId, nil
		}
		return "", fmt.Errorf("RVId %s in RV UUID file %s is not valid", rvId, uuidFilePath)
	} else if !os.IsNotExist(err) {
		// Any read error other than 'file not found' is propagated.
		return "", fmt.Errorf("failed to read RV UUID from file at %s: %v", uuidFilePath, err)
	}

	// File doesn't exist, generate a deterministic UUID using SHA1 of (nodeUUID + '|' + canonical path).
	deterministicKey := nodeUUID + "|" + path
	rvUUID := gouuid.NewSHA1(gouuid.NameSpaceDNS, []byte(deterministicKey)).String()
	common.Assert(common.IsValidUUID(rvUUID), fmt.Sprintf("Generated deterministic UUID %s is not valid", rvUUID))

	if err := os.WriteFile(uuidFilePath, []byte(rvUUID), 0400); err != nil {
		return "", fmt.Errorf("failed to write RV UUID file at %s: %v", uuidFilePath, err)
	}

	log.Info("DistributedCache::getRVUuid: Saved RV UUID %s in %s", rvUUID, uuidFilePath)

	return rvUUID, nil
}

// TODO{Akku}: Client can provide, which ethernet address we have to use. i.e. eth0, eth1
func getVmIp() (string, error) {
	addresses, err := net.InterfaceAddrs()
	if err != nil {
		return "", err
	}

	var vmIP string
	for _, addr := range addresses {
		ipNet, ok := addr.(*net.IPNet)
		if !ok || ipNet.IP.IsLoopback() {
			continue
		}
		if ipNet.IP.To4() != nil {
			vmIP = ipNet.IP.String()
			// parts := strings.Split(vmIP, ".")
			// vmIP = fmt.Sprintf("%s.%s.%d.%d", parts[0], parts[1], rand.Intn(256), rand.Intn(256))
			break
		}
	}

	if !common.IsValidIP(vmIP) {
		return "", fmt.Errorf("unable to find a valid non-loopback IPv4 address")
	}
	return vmIP, nil
}

// Get the placeHolder dir/virtual sub component for root of mountpoint.
// This virtual directory should only valid if it's present at the root of the mountpoint.
func getPlaceholderDirForRoot(path string) *internal.ObjAttr {
	attr := &internal.ObjAttr{
		Path:  path,
		Size:  4096,
		Mode:  os.ModeDir,
		Mtime: time.Now(),
		Flags: internal.NewDirBitMap(),
	}
	attr.Atime = attr.Mtime
	attr.Crtime = attr.Mtime
	attr.Ctime = attr.Mtime
	attr.Flags.Set(internal.PropFlagModeDefault)
	return attr
}

// returns true for isAzurePath, if path has "fs=azure" as its first subdir.
// return true for isDcachPath, if path has "fs=dcache" as its first subdir.
// return true for isDebugPath, if path has "fs=debug" as its first subdir
// rawPath is the resultant path after removing virtual dirs like "fs=azure/dcache"
// returns path if it dont find any virtual dirs.
func getFS(path string) (isAzurePath bool, isDcachePath bool, isDebugPath bool, rawPath string) {
	rawPath = path
	isDcachePath, tempPath := isPathContainsSubDir(path, "fs=dcache")
	if isDcachePath {
		rawPath = tempPath
		return
	}

	isAzurePath, tempPath = isPathContainsSubDir(path, "fs=azure")
	if isAzurePath {
		rawPath = tempPath
		return
	}

	isDebugPath, tempPath = isPathContainsSubDir(path, "fs=debug")
	if isDebugPath {
		rawPath = tempPath
		return
	}
	return
}

// function to know path consists of given subdir at it's root
// returns path without the subdir
func isPathContainsSubDir(path string, subdir string) (found bool, resPath string) {
	if len(path) == 0 {
		return false, path
	}

	after, found := strings.CutPrefix(path, subdir)
	if !found {
		return false, path
	}

	resPath = after
	if len(resPath) > 0 && resPath[0] != '/' {
		return false, path
	}
	resPath = strings.TrimPrefix(resPath, "/")
	return
}

// hides the cache folder that starts with prefix __CACHE__.
func hideCacheMetadata(dirList []*internal.ObjAttr) []*internal.ObjAttr {
	newDirList := make([]*internal.ObjAttr, len(dirList))
	i := 0
	for _, attr := range dirList {
		// todo: think of a better approach for doing the following.
		if !strings.HasPrefix(attr.Path, "__CACHE__") {
			newDirList[i] = attr
			i++
		}
	}
	return newDirList[:i]
}

func isMountPointRoot(path string) bool {
	if len(path) == 0 || (len(path) == 1 && path[0] == '/') {
		return true
	}
	return false
}

// Get Dcache File size from the blob metadata property.
func parseDcacheMetadata(attr *internal.ObjAttr) error {
	// No need to parse the metadata for directories.
	if attr.IsDir() {
		return nil
	}
	log.Debug("utils::parseDcacheMetadata: file: %s", attr.Name)

	var fileSize int64
	var err error

	if val, ok := attr.Metadata["cache_object_length"]; ok {
		fileSize, err = strconv.ParseInt(*val, 10, 64)
		if err == nil {
			if fileSize >= 0 {
				attr.Size = fileSize
				common.Assert(attr.Size != math.MaxInt64)
			} else if fileSize == -1 {
				//
				// FileSize can be negative in two cases:
				// Case1: File is currently being created by this or some other node in dcache.
				// Case2: Blobfuse crashed between createFileInit() and createFileFinalize().
				//        In that case we'll be having a stale entry which takes up the path and
				//        disallows further file creations on that path.
				//
				// These files are distinguished from the rest of the files by their size, while
				// getting the attr/listing the dir.
				// Though we do not hide these files from listing or lookup, we do not allow
				// these files to be read or deleted.
				//
				attr.Size = math.MaxInt64
			}
		} else {
			err = fmt.Errorf("strconv failed for cache_object_length: %s, file: %s, error: %v",
				*val, attr.Name, err)
			log.Err("utils::parseDcacheMetadata: %v", err)
			common.Assert(false, err)
			return err
		}
	} else {
		err = fmt.Errorf("Blob metadata for %s doesn't have cache_object_length property", attr.Name)
		log.Err("utils::parseDcacheMetadata: %v", err)
		common.Assert(false, err)
		return err
	}

	// parse file state.
	if state, ok := attr.Metadata["state"]; ok {
		if !(*state == string(dcache.Writing) || *state == string(dcache.Ready)) {
			err = fmt.Errorf("File: %s, has invalid state: [%s]", attr.Name, *state)
			log.Err("utils::parseDcacheMetadata: %v", err)
			common.Assert(false, err)
			return err
		}
	} else {
		err = fmt.Errorf("Blob metadata for %s doesn't have state property", attr.Name)
		log.Err("utils::parseDcacheMetadata: %v", err)
		common.Assert(false, err)
		return err
	}

	// parse open count and validate that it's not -ve.
	if val, ok := attr.Metadata["opencount"]; ok {
		openCount, err := strconv.ParseInt(*val, 10, 64)
		if err == nil {
			if openCount < 0 {
				err = fmt.Errorf("File: %s, has invalid openCount: [%s]", attr.Name, *val)
				log.Err("utils::parseDcacheMetadata: %v", err)
				common.Assert(false, err)
				return err
			}
		} else {
			err = fmt.Errorf("strconv failed for opencount: %s, file: %s, error: %v",
				*val, attr.Name, err)
			log.Err("utils::parseDcacheMetadata: %v", err)
			common.Assert(false, err)
			return err
		}
	} else {
		err = fmt.Errorf("Blob metadata for %s doesn't have opencount property", attr.Name)
		log.Err("utils::parseDcacheMetadata: %v", err)
		common.Assert(false, err)
		return err
	}

	return nil
}

// Hide the files which are set to deleting. Such files are named with suffix ".dcache.deleting"
func parseDcacheMetadataForDirEntries(dirList []*internal.ObjAttr) []*internal.ObjAttr {
	newDirList := make([]*internal.ObjAttr, len(dirList))
	i := 0

	for _, attr := range dirList {
		// Hide deleted files from fuse.
		if isDeletedDcacheFile(attr.Name) {
			log.Info("DistributedCache::parseDcacheMetadataForDirEntries: skipping deleted file: %s",
				attr.Name)
			continue
		}

		err := parseDcacheMetadata(attr)
		if err == nil {
			newDirList[i] = attr
			i++
		} else {
			log.Err("DistributedCache::parseDcacheMetadataForDirEntries: skipping dir entry, failed to parse metadata file: %s: %v",
				attr.Name, err)
		}
	}

	return newDirList[:i]
}

// Check if the file name refers to a deleted dcache file (waiting to be GC'ed).
func isDeletedDcacheFile(rawPath string) bool {
	return strings.HasSuffix(rawPath, dcache.DcacheDeletingFileNameSuffix)
}

// Queries the Azure Instance Metadata Service to get the Fault Domain and Update Domain for this VM.
// Returns -1 for Fault Domain or Update Domain if not available.
func queryVMFaultAndUpdateDomain() (int /* faultDomain */, int /* updateDomain */, error) {
	const imdsURL = "http://169.254.169.254/metadata/instance/compute?api-version=2021-02-01"

	//
	// TODO: There's a "platformSubFaultDomain" also but from the documentation it seems to be not used.
	//
	type ComputeMetadata struct {
		FaultDomain  string `json:"platformFaultDomain"`
		UpdateDomain string `json:"platformUpdateDomain"`
	}

	client := &http.Client{}
	req, err := http.NewRequest("GET", imdsURL, nil)
	if err != nil {
		err = fmt.Errorf("error creating request to Azure Instance Metadata Service %s: %v",
			imdsURL, err)
		return -1, -1, err
	}

	// Required header for Azure Instance Metadata Service.
	req.Header.Add("Metadata", "true")

	resp, err := client.Do(req)
	if err != nil {
		err = fmt.Errorf("error making request to Azure Instance Metadata Service %s: %v",
			imdsURL, err)
		return -1, -1, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		err = fmt.Errorf("error reading response from Azure Instance Metadata Service %s: %v",
			imdsURL, err)
		return -1, -1, err
	}

	var metadata ComputeMetadata
	if err := json.Unmarshal(body, &metadata); err != nil {
		err = fmt.Errorf("error unmarshalling JSON response from Azure Instance Metadata Service %s: %v [%v]",
			imdsURL, err, body)
		return -1, -1, err
	}

	//
	// Not sure if FaultDomain and UpdateDomain are always returned by IMDS.
	// Don't fail if they are empty, just return them as empty strings and let caller handle as per config setting.
	//
	fdId, udId := -1, -1
	if metadata.FaultDomain != "" {
		fdId, err = strconv.Atoi(metadata.FaultDomain)
		if err != nil {
			err = fmt.Errorf("error converting Fault Domain (%s) to integer: %v", metadata.FaultDomain, err)
			return -1, -1, err
		}
	}

	if metadata.UpdateDomain != "" {
		udId, err = strconv.Atoi(metadata.UpdateDomain)
		if err != nil {
			err = fmt.Errorf("error converting Update Domain (%s) to integer: %v", metadata.UpdateDomain, err)
			return -1, -1, err
		}
	}

	return fdId, udId, nil
}
