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
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"hash/crc64"
	"io"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"gopkg.in/ini.v1"
)

var RootMount bool
var ForegroundMount bool
var IsStream bool

// IsDirectoryMounted is a utility function that returns true if the directory is already mounted using fuse
func IsDirectoryMounted(path string) bool {
	mntList, err := os.ReadFile("/etc/mtab")
	if err != nil {
		//fmt.Println("failed to read mount points : ", err.Error())
		return false
	}

	// removing trailing / from the path
	path = strings.TrimRight(path, "/")

	for _, line := range strings.Split(string(mntList), "\n") {
		if strings.TrimSpace(line) != "" {
			mntPoint := strings.Split(line, " ")[1]
			if path == mntPoint {
				// with earlier fuse driver ' fuse.' was searched in /etc/mtab
				// however with libfuse entry does not have that signature
				// if this path is already mounted using fuse then fail
				if strings.Contains(line, "fuse") {
					//fmt.Println(path, " is already mounted.")
					return true
				}
			}
		}
	}

	return false
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
	pidString := strings.Replace(out.String(), "\n", " ", -1)
	pids := strings.Split(pidString, " ")
	for _, pid := range pids {
		// Get the mount path for this pid
		// For this we need to check the command line arguments given to this command
		// If the path is same then we need to return true
		if pid == "" {
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

// IsDirectoryEmpty is a utility function that returns true if the directory at that path is empty or not
func IsDirectoryEmpty(path string) bool {
	if !DirectoryExists(path) {
		// Directory does not exists so safe to assume its empty
		return true
	}

	f, _ := os.Open(path)
	defer f.Close()

	_, err := f.Readdirnames(1)
	// If there is nothing in the directory then it is empty
	return err == io.EOF
}

func TempCacheCleanup(path string) error {
	if !IsDirectoryEmpty(path) {
		// List the first level children of the directory
		dirents, err := os.ReadDir(path)
		if err != nil {
			// Failed to list, return back error
			return fmt.Errorf("failed to list directory contents : %s", err.Error())
		}

		// Delete all first level children with their hierarchy
		for _, entry := range dirents {
			os.RemoveAll(filepath.Join(path, entry.Name()))
		}
	}

	return nil
}

// DirectoryExists is a utility function that returns true if the directory at that path exists and returns false if it does not exist.
func DirectoryExists(path string) bool {
	_, err := os.Stat(path)

	if os.IsNotExist(err) {
		return false
	} else if err != nil {
		return false
	}
	return true
}

// GetCurrentUser is a utility function that returns the UID and GID of the user that invokes the blobfuse2 command.
func GetCurrentUser() (uint32, uint32, error) {
	var (
		currentUser      *user.User
		userUID, userGID uint64
	)

	currentUser, err := user.Current()
	if err != nil {
		return 0, 0, err
	}

	userUID, err = strconv.ParseUint(currentUser.Uid, 10, 32)
	if err != nil {
		return 0, 0, err
	}

	userGID, err = strconv.ParseUint(currentUser.Gid, 10, 32)
	if err != nil {
		return 0, 0, err
	}

	if currentUser.Name == "root" || userUID == 0 {
		RootMount = true
	} else {
		RootMount = false
	}

	return uint32(userUID), uint32(userGID), nil
}

// normalizeObjectName : If file contains \\ in name replace it with ..
func NormalizeObjectName(name string) string {
	return strings.ReplaceAll(name, "\\", "/")
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

// Encrypt given data using the key provided
func EncryptData(plainData []byte, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	ciphertext := gcm.Seal(nonce, nonce, plainData, nil)
	return ciphertext, nil
}

// Decrypt given data using the key provided
func DecryptData(cipherData []byte, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := cipherData[:gcm.NonceSize()]
	ciphertext := cipherData[gcm.NonceSize():]

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}

	return plaintext, nil
}

func GetCurrentDistro() string {
	cfg, err := ini.Load("/etc/os-release")
	if err != nil {
		return ""
	}

	distro := cfg.Section("").Key("PRETTY_NAME").String()
	return distro
}

type BitMap16 uint16

// IsSet : Check whether the given bit is set or not
func (bm BitMap16) IsSet(bit uint16) bool { return (bm & (1 << bit)) != 0 }

// Set : Set the given bit in bitmap
func (bm *BitMap16) Set(bit uint16) { *bm |= (1 << bit) }

// Clear : Clear the given bit from bitmap
func (bm *BitMap16) Clear(bit uint16) { *bm &= ^(1 << bit) }

// Reset : Reset the whole bitmap by setting it to 0
func (bm *BitMap16) Reset() { *bm = 0 }

type KeyedMutex struct {
	mutexes sync.Map // Zero value is empty and ready for use
}

func (m *KeyedMutex) GetLock(key string) *sync.Mutex {
	value, _ := m.mutexes.LoadOrStore(key, &sync.Mutex{})
	mtx := value.(*sync.Mutex)
	return mtx
}

// check if health monitor is enabled and blofuse stats monitor is not disabled
func MonitorBfs() bool {
	return EnableMonitoring && !BfsDisabled
}

// convert ~ to $HOME in path
func ExpandPath(path string) string {
	if path == "" {
		return path
	}

	if strings.HasPrefix(path, "~/") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		path = filepath.Join(homeDir, path[2:])
	}

	path = os.Expand(path, func(key string) string {
		if azureSpecialContainers[key] {
			return "$" + key // Keep it as is
		}
		return os.Getenv(key) // Expand normally
	})

	path, _ = filepath.Abs(path)
	return path
}

// NotifyMountToParent : Send a signal to parent process about successful mount
func NotifyMountToParent() error {
	if !ForegroundMount {
		ppid := syscall.Getppid()
		if ppid > 1 {
			if err := syscall.Kill(ppid, syscall.SIGUSR2); err != nil {
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

var currentUID int = -1

// GetDiskUsageFromStatfs: Current disk usage of temp path
func GetDiskUsageFromStatfs(path string) (float64, float64, error) {
	// We need to compute the disk usage percentage for the temp path
	var stat syscall.Statfs_t
	err := syscall.Statfs(path, &stat)
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

type WriteToFileOptions struct {
	Flags      int
	Permission os.FileMode
}

func WriteToFile(filename string, data string, options WriteToFileOptions) error {
	// Open the file with the provided flags, create it if it doesn't exist
	//check if options.Permission is 0 if so then assign 0777
	if options.Permission == 0 {
		options.Permission = 0777
	}
	file, err := os.OpenFile(filename, options.Flags|os.O_CREATE|os.O_WRONLY, options.Permission)
	if err != nil {
		return fmt.Errorf("error opening file: [%s]", err.Error())
	}
	defer file.Close() // Ensure the file is closed when we're done

	// Write the data content to the file
	if _, err := file.WriteString(data); err != nil {
		return fmt.Errorf("error writing to file [%s]", err.Error())
	}

	return nil
}

func GetCRC64(data []byte, len int) []byte {
	// Create a CRC64 hash using the ECMA polynomial
	crc64Table := crc64.MakeTable(crc64.ECMA)
	checksum := crc64.Checksum(data[:len], crc64Table)

	checksumBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(checksumBytes, checksum)

	return checksumBytes
}

// parseUint32 converts a *string to uint32
func ParseUint32(s string) uint32 {
	if s == "" {
		return 0
	}
	val, err := strconv.ParseUint(s, 10, 32)
	if err != nil {
		return 0
	}
	return uint32(val)
}

func ReadMetadata(metadata map[string]*string, key string) *string {
	lowerKey := strings.ToLower(key)
	for mapKey, mapValue := range metadata {
		if strings.ToLower(mapKey) == lowerKey {
			return mapValue
		}
	}
	return nil
}
func GetMD5(fi *os.File) ([]byte, error) {
	hasher := md5.New()
	_, err := io.Copy(hasher, fi)

	if err != nil {
		return nil, fmt.Errorf("failed to generate md5 [%s]", err.Error())
	}

	return hasher.Sum(nil), nil
}

func ComponentInPipeline(pipeline []string, component string) bool {
	for _, comp := range pipeline {
		if comp == component {
			return true
		}
	}

	return false
}

func ValidatePipeline(pipeline []string) error {
	// file-cache, block-cache and xload are mutually exclusive
	if ComponentInPipeline(pipeline, "file_cache") &&
		ComponentInPipeline(pipeline, "block_cache") {
		return fmt.Errorf("mount: file-cache and block-cache cannot be used together")
	}

	if ComponentInPipeline(pipeline, "file_cache") &&
		ComponentInPipeline(pipeline, "xload") {
		return fmt.Errorf("mount: file-cache and xload cannot be used together")
	}

	if ComponentInPipeline(pipeline, "block_cache") &&
		ComponentInPipeline(pipeline, "xload") {
		return fmt.Errorf("mount: block-cache and xload cannot be used together")
	}

	return nil
}

func UpdatePipeline(pipeline []string, component string) []string {
	if ComponentInPipeline(pipeline, component) {
		return pipeline
	}

	if component == "xload" {
		for i, comp := range pipeline {
			if comp == "file_cache" || comp == "block_cache" {
				pipeline[i] = component
				return pipeline
			}
		}
	}

	if component == "block_cache" {
		for i, comp := range pipeline {
			if comp == "file_cache" || comp == "xload" {
				pipeline[i] = component
				return pipeline
			}
		}
	}

	return pipeline
}
