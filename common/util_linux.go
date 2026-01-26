//go:build linux

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
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"
)

// NotifyMountToParent : Send a signal to parent process about successful mount
func NotifyMountToParent() error {
	if !ForegroundMount {
		ppid := unix.Getppid()
		if ppid > 1 {
			if err := unix.Kill(ppid, unix.SIGUSR2); err != nil {
				return err
			}
		} else {
			return fmt.Errorf("failed to get parent pid, received : %v", ppid)
		}
	}

	return nil
}

var duPath []string = []string{"/usr/bin/du", "/usr/local/bin/du", "/usr/sbin/du", "/usr/local/sbin/du", "/sbin/du", "/bin/du"}
var selectedDuPath string = ""

// GetUsage: The current disk usage in MB
func GetUsage(path string) (float64, error) {
	var currSize float64
	var out bytes.Buffer

	if selectedDuPath == "" {
		selectedDuPath = "-"
		for _, dup := range duPath {
			_, err := os.Stat(dup)
			if err == nil {
				selectedDuPath = dup
				break
			}
		}
	}

	if selectedDuPath == "-" {
		return 0, fmt.Errorf("failed to find du")
	}

	// du - estimates file space usage
	// https://man7.org/linux/man-pages/man1/du.1.html
	// Note: We cannot just pass -BM as a parameter here since it will result in less accurate estimates of the size of the path
	// (i.e. du will round up to 1M if the path is smaller than 1M).
	cmd := exec.Command(selectedDuPath, "-sh", path)
	cmd.Stdout = &out

	err := cmd.Run()
	if err != nil {
		return 0, err
	}

	size := strings.Split(out.String(), "\t")[0]
	if size == "0" {
		return 0, nil
	}

	// some OS's use "," instead of "." that will not work for float parsing - replace it
	size = strings.Replace(size, ",", ".", 1)
	parsed, err := strconv.ParseFloat(size[:len(size)-1], 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse du output")
	}

	switch size[len(size)-1] {
	case 'K':
		currSize = parsed / float64(1024)
	case 'M':
		currSize = parsed
	case 'G':
		currSize = parsed * 1024
	case 'T':
		currSize = parsed * 1024 * 1024
	}

	return currSize, nil
}

// GetAvailFree: Available bytes
func GetAvailFree(path string) (uint64, uint64, error) {
	var stat unix.Statfs_t
	err := unix.Statfs(path, &stat)
	if err != nil {
		return 0, 0, err
	}
	return stat.Bavail * uint64(stat.Bsize), stat.Bfree * uint64(stat.Bsize), err
}

// GetFreeRam: Available ram
func GetFreeRam() (uint64, error) {
	var sysinfo unix.Sysinfo_t
	err := unix.Sysinfo(&sysinfo)
	if err != nil {
		return 0, err
	}
	return sysinfo.Freeram * uint64(sysinfo.Unit), nil
}

var currentUID int = -1

// GetDiskUsageFromStatfs: Current disk usage of temp path
func GetDiskUsageFromStatfs(path string) (float64, float64, error) {
	// We need to compute the disk usage percentage for the temp path
	var stat unix.Statfs_t
	err := unix.Statfs(path, &stat)
	if err != nil {
		return 0, 0, err
	}

	if currentUID == -1 {
		currentUID = os.Getuid()
	}

	var availableSpace uint64
	if currentUID == 0 {
		// Sudo  has mounted
		availableSpace = stat.Bfree * uint64(stat.Frsize)
	} else {
		// non Sudo has mounted
		availableSpace = stat.Bavail * uint64(stat.Frsize)
	}

	totalSpace := stat.Blocks * uint64(stat.Frsize)
	usedSpace := float64(totalSpace - availableSpace)
	return usedSpace, float64(usedSpace) / float64(totalSpace) * 100, nil
}

// List all mount points which were mounted using blobfuse2
func ListMountPoints() ([]string, error) {
	file, err := os.Open("/etc/mtab")
	if err != nil {
		return nil, err
	}

	defer file.Close()

	// Read /etc/mtab file line by line
	var mntList []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()

		// If there is any directory mounted using blobfuse2 its of our interest
		if strings.HasPrefix(line, "blobfuse2") {
			// Extract the mount path from this line
			mntPath := strings.Split(line, " ")[1]
			mntList = append(mntList, mntPath)
		}
	}

	return mntList, nil
}


func IsMountActive(path string) (bool, error) {
	// Get the process details for this path using ps -aux
	var out bytes.Buffer
	cmd := exec.Command("pidof", "blobfuse2")
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		if err.Error() == "exit status 1" {
			return false, nil
		} else {
			return true, fmt.Errorf("failed to get pid of blobfuse2 [%v]", err.Error())
		}
	}

	// out contains the list of pids of the processes that are running
	pidString := strings.ReplaceAll(out.String(), "\n", " ")
	pids := strings.Split(pidString, " ")
	myPid := strconv.Itoa(os.Getpid())
	for _, pid := range pids {
		// Get the mount path for this pid
		// For this we need to check the command line arguments given to this command
		// If the path is same then we need to return true
		if pid == "" || pid == myPid {
			continue
		}

		cmd = exec.Command("ps", "-o", "args=", "-p", pid)
		out.Reset()
		cmd.Stdout = &out

		err := cmd.Run()
		if err != nil {
			return true, fmt.Errorf("failed to get command line arguments for pid %s [%v]", pid, err.Error())
		}

		if strings.Contains(out.String(), path) {
			return true, nil
		}
	}

	return false, nil
}

func GetFuseMinorVersion() int {
	var out bytes.Buffer
	cmd := exec.Command("fusermount3", "--version")
	cmd.Stdout = &out

	err := cmd.Run()
	if err != nil {
		return 0
	}

	output := strings.Split(out.String(), ":")
	if len(output) < 2 {
		return 0
	}

	version := strings.Trim(output[1], " ")
	if version == "" {
		return 0
	}

	output = strings.Split(version, ".")
	if len(output) < 2 {
		return 0
	}

	val, err := strconv.Atoi(output[1])
	if err != nil {
		return 0
	}

	return val
}
