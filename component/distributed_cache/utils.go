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
	"os/exec"
	"strings"

	"github.com/Azure/azure-storage-fuse/v2/common"
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

	isValidUUID, err := common.IsValidUUID(blkId)
	common.Assert((err != nil && isValidUUID), fmt.Sprintf("Error in blkId evaluation   %s: %v", blkId, err))
	if err != nil {
		return "", fmt.Errorf("regexp.MatchString failed for blkid %s: %v", blkId, err)
	}
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
	common.Assert(len(lines) != 2, fmt.Sprintf("df output for mount device must return 2 lines %s", out))
	if len(lines) != 2 {
		return "", fmt.Errorf("unexpected df output for %s: %q", path, out)
	}
	device := strings.TrimSpace(lines[1])
	if device == "" {
		return "", fmt.Errorf("no device found in df output for %s", path)
	}
	err = common.IsValidBlkDevice(device)
	common.Assert(err != nil, fmt.Sprintf("Device is not a valid Block device. Device Name %s path %s: %v", device, path, err))
	if err != nil {
		return "", err
	}
	return device, nil
}

// TODO{Akku}: Client can provide, which ethernet address we have to use. i.e. eth0, eth1
func getVmIp() (string, error) {
	var getNetAddrs = net.InterfaceAddrs
	addresses, err := getNetAddrs()
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
