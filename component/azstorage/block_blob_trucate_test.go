//go:build !authtest
// +build !authtest

/*
    _____           _____   _____   ____          ______  _____  ------
   |     |  |      |     | |     | |     |     | |       |            |
   |     |  |      |     | |     | |     |     | |       |            |
   | --- |  |      |     | |-----| |---- |     | |-----| |-----  ------
   |     |  |      |     | |     | |     |     |       | |       |
   | ____|  |_____ | ____| | ____| |     |_____|  _____| |_____  |_____


   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2020-2024 Microsoft Corporation. All rights reserved.
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

package azstorage

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"os"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/container"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/service"
	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type blockBlobTruncateSuite struct {
	suite.Suite
	assert          *assert.Assertions
	az              *AzStorage
	serviceClient   *service.Client
	containerClient *container.Client
	config          string
	container       string
}

func (s *blockBlobTruncateSuite) SetupTest() {
	// Logging config
	cfg := common.LogConfig{
		FilePath:    "./logfile.txt",
		MaxFileSize: 10,
		FileCount:   10,
		Level:       common.ELogLevel.LOG_DEBUG(),
	}
	_ = log.SetDefaultLogger("base", cfg)

	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Println("Unable to get home directory")
		os.Exit(1)
	}
	cfgFile, err := os.Open(homeDir + "/azuretest.json")
	if err != nil {
		fmt.Println("Unable to open config file")
		os.Exit(1)
	}

	cfgData, _ := io.ReadAll(cfgFile)
	err = json.Unmarshal(cfgData, &storageTestConfigurationParameters)
	if err != nil {
		fmt.Println("Failed to parse the config file")
		os.Exit(1)
	}

	cfgFile.Close()
	s.setupTestHelper("", "", true)
}

func (s *blockBlobTruncateSuite) setupTestHelper(configuration string, container string, create bool) {
	if container == "" {
		container = generateContainerName()
	}
	s.container = container
	if configuration == "" {
		configuration = fmt.Sprintf("azstorage:\n  account-name: %s\n  endpoint: https://%s.blob.core.windows.net/\n  type: block\n  account-key: %s\n  mode: key\n  container: %s\n  block-size-mb: 16\n  fail-unsupported-op: true",
			storageTestConfigurationParameters.BlockAccount, storageTestConfigurationParameters.BlockAccount, storageTestConfigurationParameters.BlockKey, s.container)
	}
	s.config = configuration

	s.assert = assert.New(s.T())

	s.az, _ = newTestAzStorage(configuration)
	_ = s.az.Start(ctx) // Note: Start->TestValidation will fail but it doesn't matter. We are creating the container a few lines below anyway.
	// We could create the container before but that requires rewriting the code to new up a service client.

	s.serviceClient = s.az.storage.(*BlockBlob).Service // Grab the service client to do some validation
	s.containerClient = s.serviceClient.NewContainerClient(s.container)
	if create {
		_, _ = s.containerClient.Create(ctx, nil)
	}
}

func (s *blockBlobTruncateSuite) tearDownTestHelper(delete bool) {
	_ = s.az.Stop()
	if delete {
		_, _ = s.containerClient.Delete(ctx, nil)
	}
}

func (s *blockBlobTruncateSuite) cleanupTest() {
	s.tearDownTestHelper(true)
	_ = log.Destroy()
}

const maxSize = 500 // Max size in MB

func getMaxSize() int64 {
	return maxSize * MB
}

func generateRandomSize() int64 {
	size, _ := rand.Int(rand.Reader, big.NewInt(maxSize+1))
	return size.Int64() * MB
}

type FileSize struct {
	OriginalSize  int64
	TruncatedSize int64
}

func (suite *blockBlobTruncateSuite) TestFileTruncate() {
	fileSizes := make([]FileSize, 0)

	fileSizes = append(fileSizes, FileSize{288358400, 269484032})

	// Truncate a new (empty) file to 0
	fileSizes = append(fileSizes, FileSize{0, 0})

	// Truncate a new file to same size
	fileSizes = append(fileSizes, FileSize{5, 5})

	// Truncate a new file to same size
	fileSizes = append(fileSizes, FileSize{5, 20})

	// Truncate file to max size to 0 and reverse
	fileSizes = append(fileSizes, FileSize{0, getMaxSize() - 5})
	fileSizes = append(fileSizes, FileSize{getMaxSize() - 5, 0})

	// Truncate file from very low size to big size and reverse
	fileSizes = append(fileSizes, FileSize{(16 * MB * 3) + 50, 10})
	fileSizes = append(fileSizes, FileSize{10, (16 * MB * 3) + 50})

	// Create random numbers for file sizes and truncate sizes
	for i := 0; i < 50; i++ {
		fileSizes = append(fileSizes, FileSize{generateRandomSize(), generateRandomSize()})
	}

	for idx, fs := range fileSizes {
		fmt.Printf("Truncate Test: %v => %v\n", fs.OriginalSize, fs.TruncatedSize)
		filename := fmt.Sprintf("testfile_%d", idx)

		data := bytes.Repeat([]byte{'a'}, int(fs.OriginalSize))

		err := os.WriteFile(filename, data, 0777)
		suite.assert.Nil(err)

		h, err := os.Open(filename)
		suite.assert.Nil(err)
		suite.assert.NotNil(h)

		suite.az.CopyFromFile(internal.CopyFromFileOptions{Name: filename, File: h, Metadata: nil})
		_ = h.Close()
		os.Remove(filename)

		err = suite.az.TruncateFile(internal.TruncateFileOptions{Name: filename, Size: fs.TruncatedSize, BlockSize: 16 * MB})
		suite.assert.Nil(err)

		attr, err := suite.az.GetAttr(internal.GetAttrOptions{Name: filename})
		suite.assert.Nil(err)
		suite.assert.Equal(fs.TruncatedSize, attr.Size)

		_ = suite.az.DeleteFile(internal.DeleteFileOptions{Name: filename})
	}
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestBlockBlobTruncate(t *testing.T) {
	suite.Run(t, new(blockBlobTruncateSuite))
}
