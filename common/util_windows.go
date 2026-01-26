//go:build windows

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

package common

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/shirou/gopsutil/v4/mem"
	"golang.org/x/sys/windows"
)

const SectorSize = 4096 // Sector size of disk

// IsDriveLetter returns true if the path is a drive letter on Windows, such
// as 'D:' or 'f:'. Returns false otherwise.
func IsDriveLetter(path string) bool {
	pattern := `^[A-Za-z]:$`
	match, _ := regexp.MatchString(pattern, path)
	return match
}

// NotifyMountToParent : Does nothing on Windows
func NotifyMountToParent() error {
	return nil
}

// totalSectors walks through all files in the path and gives an estimate of the total number of sectors
// that are being used. Based on https://stackoverflow.com/questions/32482673/how-to-get-directory-total-size
func totalSectors(path string) int64 {
	//bytes per sector is hard coded to 4096 bytes since syscall to windows and BytesPerSector for the drive in question is an estimate.
	// https://devblogs.microsoft.com/oldnewthing/20160427-00/?p=93365

	var totalSectors int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			totalSectors += (info.Size() / SectorSize)
			if info.Size()%SectorSize != 0 {
				totalSectors++
			}
		}
		return err
	})

	// TODO: Handle this error properly
	if err != nil {
		return totalSectors
	}

	return totalSectors

}

// GetUsage: The current disk usage in MB
func GetUsage(path string) (float64, error) {
	totalSectors := totalSectors(path)

	totalBytes := float64(totalSectors * SectorSize)
	totalBytes = totalBytes / MbToBytes

	return totalBytes, nil
}

// GetDiskUsageFromStatfs: Current disk usage of temp path
func GetDiskUsageFromStatfs(path string) (float64, float64, error) {
	// We need to compute the disk usage percentage for the temp path
	var free, total, avail uint64

	// Get path to the cache
	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		panic(err)
	}
	err = windows.GetDiskFreeSpaceEx(pathPtr, &free, &total, &avail)
	if err != nil {
		return 0, 0, err
	}

	usedSpace := float64(total - avail)
	return usedSpace, float64(usedSpace) / float64(total) * 100, nil
}

// GetAvailFree: Available bytes
func GetAvailFree(path string) (uint64, uint64, error) {
	var free, total, avail uint64

	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, 0, err
	}
	err = windows.GetDiskFreeSpaceEx(pathPtr, &free, &total, &avail)
	if err != nil {
		return 0, 0, err
	}

	return avail, free, nil
}

// GetFreeRam: Available ram
func GetFreeRam() (uint64, error) {
	v, err := mem.VirtualMemory()
	if err != nil {
		return 0, err
	}
	return v.Available, nil
}

// List all mount points which were mounted using blobfuse2
func ListMountPoints() ([]string, error) {
	out, err := exec.Command(`C:\Program Files (x86)\WinFsp\bin\launchctl-x64.exe`, "list").Output()
	if err != nil {
		fmt.Printf("Is WinFSP installed? 'launchctl-x64.exe list' failed with error: %v\n", err)
		return nil, err
	}
	var mntList []string
	outList := strings.Split(string(out), "\n")
	for _, item := range outList {
		if strings.HasPrefix(item, "blobfuse2") {
			// Extract the mount path from this line
			mntPath := strings.Split(item, " ")[1]
			mntList = append(mntList, mntPath)
		}
	}
	return mntList, nil
}
