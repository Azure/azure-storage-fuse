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
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blockblob"
)

// azSDKTimeout bounds a single SDK operation (upload / delete). Keep it
// generous enough to accommodate a 100+ MiB payload upload from a slow
// pipeline agent, but tight enough that a genuine hang is caught before the
// per-test `go test` timeout fires.
const azSDKTimeout = 5 * time.Minute

// generateRandomBytes returns a slice of n cryptographically random bytes.
// Using crypto/rand rather than math/rand ensures that dist_cache chunk
// payloads are not accidentally identical across tests, which would defeat
// per-chunk cache-key isolation.
func generateRandomBytes(t *testing.T, n int) []byte {
	t.Helper()
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		t.Fatalf("rand.Read: %v", err)
	}
	return buf
}

// md5Sum returns the hex MD5 of `data`. Only used for equality assertions
// between what we uploaded and what we read; not a security boundary.
func md5Sum(data []byte) string {
	sum := md5.Sum(data)
	return hex.EncodeToString(sum[:])
}

// randomTestDirName returns a short random hex string suitable for isolating
// concurrent test runs within the same container.
func randomTestDirName(t *testing.T, n int) string {
	t.Helper()
	b := make([]byte, (n+1)/2)
	if _, err := rand.Read(b); err != nil {
		t.Fatalf("rand.Read: %v", err)
	}
	return fmt.Sprintf("%x", b)[:n]
}

// newBlockBlobClient constructs an azblob block-blob client for the given
// blob path (relative to the container root). Every test that seeds data
// goes through this: the read-only dist_cache design forbids writing
// through the FUSE mount, so all writes must reach azstorage out-of-band.
func newBlockBlobClient(blobPath string) (*blockblob.Client, error) {
	if testCfg.storageAccount == "" || testCfg.storageKey == "" {
		return nil, fmt.Errorf(
			"azstorage credentials not set (need STO_ACC_NAME + STO_ACC_KEY, " +
				"or -storage-account / -storage-key)")
	}
	if testCfg.storageContainer == "" {
		return nil, fmt.Errorf("azstorage container not set (need containerName / DCACHE_E2E_CONTAINER)")
	}
	cred, err := azblob.NewSharedKeyCredential(testCfg.storageAccount, testCfg.storageKey)
	if err != nil {
		return nil, fmt.Errorf("shared key credential: %w", err)
	}
	url := fmt.Sprintf("%s/%s/%s",
		trimTrailingSlash(testCfg.storageEndpoint),
		testCfg.storageContainer,
		blobPath)
	client, err := blockblob.NewClientWithSharedKeyCredential(url, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("block blob client: %w", err)
	}
	return client, nil
}

func trimTrailingSlash(s string) string {
	for len(s) > 0 && s[len(s)-1] == '/' {
		s = s[:len(s)-1]
	}
	return s
}

// uploadBlob writes `data` to the given container-relative blob path via
// the Azure SDK. This is the *only* legitimate write path for these tests:
// the FUSE mount is opened read-only, so we cannot (and must not) write
// through it. UploadBuffer chunks large payloads into multiple staged
// blocks internally; the resulting blob is a single object.
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

// deleteBlob removes a blob from azstorage. Used from t.Cleanup hooks so
// tests do not pollute the container across runs. Failures are logged, not
// fatal: masking a real assertion failure with a cleanup error is worse
// than leaking a test blob whose name is already random-suffixed.
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
