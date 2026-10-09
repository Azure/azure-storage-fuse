//go:build !unittest
// +build !unittest

/*
    _____           _____   _____   ____          ______  _____  ------
   |     |  |      |     | |     | |     |     | |       |            |
   |     |  |      |     | |     | |     |     | |       |            |
   | --- |  |      |     | |-----| |---- |     | |-----| |-----  ------
   |     |  |      |     | |     | |     |     |       | |       |
   | ____|  |_____ | ____| | ____| |     |_____|  _____| |_____  |_____


   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2020-2026 Microsoft Corporation. All rights reserved.
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

package dcache_e2e

import (
	"context"
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blockblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/container"
)

// azSDKTimeout bounds one Azure SDK operation.
const azSDKTimeout = 5 * time.Minute

// Shared per-container client; the SDK pipeline is safe for concurrent use.
var (
	containerClientOnce sync.Once
	containerClient     *container.Client
	containerClientErr  error
)

// getContainerClient lazily builds one container client shared across helpers.
func getContainerClient() (*container.Client, error) {
	containerClientOnce.Do(func() {
		if testCfg.storageAccount == "" || testCfg.storageKey == "" {
			containerClientErr = fmt.Errorf(
				"azstorage credentials not set (need STO_ACC_NAME + STO_ACC_KEY, " +
					"or -storage-account / -storage-key)")
			return
		}
		if testCfg.storageContainer == "" {
			containerClientErr = fmt.Errorf(
				"azstorage container not set (need containerName / DCACHE_E2E_CONTAINER)")
			return
		}
		cred, err := azblob.NewSharedKeyCredential(testCfg.storageAccount, testCfg.storageKey)
		if err != nil {
			containerClientErr = fmt.Errorf("shared key credential: %w", err)
			return
		}
		url := fmt.Sprintf("%s/%s",
			trimTrailingSlash(testCfg.storageEndpoint),
			testCfg.storageContainer)
		containerClient, containerClientErr = container.NewClientWithSharedKeyCredential(url, cred, nil)
		if containerClientErr != nil {
			containerClientErr = fmt.Errorf("container client: %w", containerClientErr)
		}
	})
	return containerClient, containerClientErr
}

// generateRandomBytes returns unique test payload data.
func generateRandomBytes(t *testing.T, n int) []byte {
	t.Helper()
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		t.Fatalf("rand.Read: %v", err)
	}
	return buf
}

// md5Sum returns a checksum for data-integrity assertions.
func md5Sum(data []byte) string {
	sum := md5.Sum(data)
	return hex.EncodeToString(sum[:])
}

// randomTestDirName returns a short random hex suffix.
func randomTestDirName(t *testing.T, n int) string {
	t.Helper()
	b := make([]byte, (n+1)/2)
	if _, err := rand.Read(b); err != nil {
		t.Fatalf("rand.Read: %v", err)
	}
	return fmt.Sprintf("%x", b)[:n]
}

// newBlockBlobClient returns a per-blob client that shares the container-level pipeline.
func newBlockBlobClient(blobPath string) (*blockblob.Client, error) {
	c, err := getContainerClient()
	if err != nil {
		return nil, err
	}
	return c.NewBlockBlobClient(blobPath), nil
}

func trimTrailingSlash(s string) string {
	for len(s) > 0 && s[len(s)-1] == '/' {
		s = s[:len(s)-1]
	}
	return s
}

// uploadBlob seeds data outside the read-only FUSE mount.
func uploadBlob(t *testing.T, blobPath string, data []byte) {
	t.Helper()
	client, err := newBlockBlobClient(blobPath)
	if err != nil {
		t.Fatalf("upload %s: %v", blobPath, err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), azSDKTimeout)
	defer cancel()
	if _, err := client.UploadBuffer(ctx, data, nil); err != nil {
		t.Fatalf("upload %s: %v", blobPath, err)
	}
}

// deleteBlob logs cleanup failures to preserve the original test result.
func deleteBlob(t *testing.T, blobPath string) {
	t.Helper()
	client, err := newBlockBlobClient(blobPath)
	if err != nil {
		t.Logf("cleanup: delete %s: %v", blobPath, err)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), azSDKTimeout)
	defer cancel()
	if _, err := client.Delete(ctx, nil); err != nil {
		t.Logf("cleanup: delete %s: %v", blobPath, err)
	}
}
