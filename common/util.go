/*
    _____           _____   _____   ____          ______  _____  ------
   |     |  |      |     | |     | |     |     | |       |            |
   |     |  |      |     | |     | |     |     | |       |            |
   | --- |  |      |     | |-----| |---- |     | |-----| |-----  ------
   |     |  |      |     | |     | |     |     |       | |       |
   | ____|  |_____ | ____| | ____| |     |_____|  _____| |_____  |_____


   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2020-2022 Microsoft Corporation. All rights reserved.
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
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/user"
	"strconv"
	"strings"

	"gopkg.in/ini.v1"
)

var RootMount bool

//IsDirectoryMounted is a utility function that returns true if the directory is already mounted using fuse
func IsDirectoryMounted(path string) bool {
	mntList, err := ioutil.ReadFile("/etc/mtab")
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

//IsDirectoryEmpty is a utility function that returns true if the directory at that path is empty or not
func IsDirectoryEmpty(path string) bool {
	f, _ := os.Open(path)
	defer f.Close()

	_, err := f.Readdirnames(1)
	if err == io.EOF {
		return true
	}

	if err != nil && err.Error() == "invalid argument" {
		fmt.Println("Broken Mount : First Unmount ", path)
	}

	return false
}

//DirectoryExists is a utility function that returns true if the directory at that path exists and returns false if it does not exist.
func DirectoryExists(path string) bool {
	_, err := os.Stat(path)

	if os.IsNotExist(err) {
		return false
	} else if err != nil {
		return false
	}
	return true
}

//GetCurrentUser is a utility function that returns the UID and GID of the user that invokes the blobfuse2 command.
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
