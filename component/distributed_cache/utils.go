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
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func getBlockDeviceUUId(path string) (string, error) {
	device, err := findMountDevice(path)
	if err != nil {
		return "", err
	}
	out, err := exec.Command("blkid", "-o", "value", "-s", "UUID", device).Output()
	if err != nil {
		return "", fmt.Errorf("error running blkid: %v", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func findMountDevice(path string) (string, error) {
	file, err := os.Open("/proc/mounts")
	if err != nil {
		return "", err
	}
	defer file.Close()

	dirToMatch, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}

	// Try exact mount first, then parent directories if needed
	scanner := bufio.NewScanner(file)
	var device, mountPoint string
	var bestMatch string
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) >= 2 {
			device, mountPoint = fields[0], fields[1]
			if mountPoint == dirToMatch {
				return device, nil
			}
			if strings.HasPrefix(dirToMatch, mountPoint) && len(mountPoint) > len(bestMatch) {
				bestMatch = device
			}
		}
	}

	if bestMatch == "" {
		return "", fmt.Errorf("no mount device found for path: %s", path)
	}
	return bestMatch, nil
}
