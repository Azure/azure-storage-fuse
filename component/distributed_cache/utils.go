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
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/internal"
)

func getBlockDeviceUUId(path string) (string, error) {
	// TODO{Akku}: support non‐disk filesystems (e.g. NFS).
	// For example, create/lookup a “.rvid” file inside the RV folder and use that UUID.
	device, err := findMountDevice(path)
	if err != nil {
		return "", err
	}

	out, err := exec.Command("blkid", "-o", "value", "-s", "UUID", device).Output()
	if err != nil {
		return "", fmt.Errorf("error running blkid: %v", err)
	}
	blkId := strings.TrimSpace(string(out))

	isValidUUID := common.IsValidUUID(blkId)
	common.Assert((isValidUUID), fmt.Sprintf("Error in blkId evaluation   %s: %v", blkId, err))
	if !isValidUUID {
		return "", fmt.Errorf("not a valid blkid %s", blkId)
	}
	return blkId, nil
}

func findMountDevice(path string) (string, error) {
	// Call: df --output=source <path>
	out, err := exec.Command("df", "--output=source", path).Output()
	if err != nil {
		return "", fmt.Errorf("failed to run df on %s: %v", path, err)
	}
	// df prints a header line, then the device
	dfOutString := string(out)
	lines := strings.Split(strings.TrimSpace(dfOutString), "\n")
	common.Assert(len(lines) == 2, fmt.Sprintf("df output for mount device must return 2 lines %s", out))
	if len(lines) != 2 {
		return "", fmt.Errorf("unexpected df output for %s: %q", path, out)
	}
	device := strings.TrimSpace(lines[1])
	if device == "" {
		return "", fmt.Errorf("no device found in df output for %s", path)
	}
	err = common.IsValidBlkDevice(device)
	common.Assert(err == nil, fmt.Sprintf("Device is not a valid Block device. Device Name %s path %s: %v", device, path, err))
	if err != nil {
		return "", err
	}
	return device, nil
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
// rawPath is the resultant path after removing virtual dirs like "fs=azure/dcache"
// returns path if it dont find any virtual dirs.
func getFS(path string) (isAzurePath bool, isDcachePath bool, rawPath string) {
	rawPath = path
	isAzurePath, tempPath := isPathContainsSubDir(path, "fs=azure")
	if isAzurePath {
		rawPath = tempPath
	} else {
		isDcachePath, tempPath = isPathContainsSubDir(path, "fs=dcache")
		if isDcachePath {
			rawPath = tempPath
		}
	}
	return isAzurePath, isDcachePath, rawPath
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
