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

package azstorage

import (
	"bytes"
	"container/list"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/streaming"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blockblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/container"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/service"
	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/config"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal"
	"github.com/Azure/azure-storage-fuse/v2/internal/handlemap"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

var ctx = context.Background()

const MB = 1024 * 1024

// A UUID representation compliant with specification in RFC 4122 document.
type uuid [16]byte

const reservedRFC4122 byte = 0x40

func (u uuid) bytes() []byte {
	return u[:]
}

// NewUUID returns a new uuid using RFC 4122 algorithm.
func newUUID() (u uuid) {
	u = uuid{}
	// Set all bits to randomly (or pseudo-randomly) chosen values.
	rand.Read(u[:])
	u[8] = (u[8] | reservedRFC4122) & 0x7F // u.setVariant(ReservedRFC4122)

	var version byte = 4
	u[6] = (u[6] & 0xF) | (version << 4) // u.setVersion(4)
	return
}

// uploadReaderAtToBlockBlob uploads a buffer in blocks to a block blob.
func uploadReaderAtToBlockBlob(ctx context.Context, reader io.ReaderAt, readerSize, singleUploadSize int64, blockBlobClient *blockblob.Client, o *blockblob.UploadBufferOptions) error {
	if o == nil {
		o = &blockblob.UploadBufferOptions{}
	}

	if readerSize <= singleUploadSize {
		// If the size can fit in 1 Upload call, do it this way
		var body io.ReadSeeker = io.NewSectionReader(reader, 0, readerSize)
		_, err := blockBlobClient.Upload(ctx, streaming.NopCloser(body), &blockblob.UploadOptions{
			CPKInfo: o.CPKInfo,
		})
		return err
	}

	// calculate block size if not given
	if o.BlockSize == 0 {
		// If bufferSize > (BlockBlobMaxStageBlockBytes * BlockBlobMaxBlocks), then error
		if readerSize > blockblob.MaxStageBlockBytes*blockblob.MaxBlocks {
			return errors.New("buffer is too large to upload to a block blob")
		}
		// If bufferSize <= singleUploadSize, then Upload should be used with just 1 I/O request
		if readerSize <= singleUploadSize {
			o.BlockSize = singleUploadSize // Default if unspecified
		} else {
			o.BlockSize = int64(math.Ceil(float64(readerSize) / blockblob.MaxBlocks)) // buffer / max blocks = block size to use all 50,000 blocks
			if o.BlockSize < blob.DefaultDownloadBlockSize {                          // If the block size is smaller than 4MB, round up to 4MB
				o.BlockSize = blob.DefaultDownloadBlockSize
			}
		}
	}

	var numBlocks = uint16(((readerSize - 1) / o.BlockSize) + 1)
	if numBlocks > blockblob.MaxBlocks {
		return errors.New("block limit exceeded")
	}

	blockIDList := make([]string, numBlocks) // Base-64 encoded block IDs

	for i := uint16(0); i < numBlocks; i++ {
		offset := int64(i) * o.BlockSize
		chunkSize := o.BlockSize

		// for last block, chunk size might be less than block size
		if i == numBlocks-1 {
			chunkSize = readerSize - offset
		}

		var body io.ReadSeeker = io.NewSectionReader(reader, offset, chunkSize)
		blockIDList[i] = base64.StdEncoding.EncodeToString(newUUID().bytes())
		_, err := blockBlobClient.StageBlock(ctx, blockIDList[i], streaming.NopCloser(body), &blockblob.StageBlockOptions{
			CPKInfo: o.CPKInfo,
		})
		if err != nil {
			return err
		}
	}

	_, err := blockBlobClient.CommitBlockList(ctx, blockIDList, &blockblob.CommitBlockListOptions{
		CPKInfo: o.CPKInfo,
	})
	return err
}

type blockBlobTestSuite struct {
	suite.Suite
	assert          *assert.Assertions
	az              *AzStorage
	serviceClient   *service.Client
	containerClient *container.Client
	config          string
	container       string
}

func newTestAzStorage(configuration string) (*AzStorage, error) {
	err := config.ReadConfigFromReader(strings.NewReader(configuration))
	if err != nil {
		return nil, err
	}
	az := NewazstorageComponent()
	err = az.Configure(true)
	return az.(*AzStorage), err
}

func (s *blockBlobTestSuite) SetupTest() {
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

func (s *blockBlobTestSuite) setupTestHelper(configuration string, container string, create bool) {
	if container == "" {
		container = generateContainerName()
	}
	s.container = container
	if configuration == "" {
		configuration = fmt.Sprintf("azstorage:\n  account-name: %s\n  endpoint: https://%s.blob.core.windows.net/\n  type: block\n  account-key: %s\n  mode: key\n  container: %s\n  fail-unsupported-op: true",
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

func (s *blockBlobTestSuite) tearDownTestHelper(delete bool) {
	_ = s.az.Stop()
	if delete {
		_, _ = s.containerClient.Delete(ctx, nil)
	}
}

func (s *blockBlobTestSuite) cleanupTest() {
	s.tearDownTestHelper(true)
	_ = log.Destroy()
}

func (s *blockBlobTestSuite) TestInvalidBlockSize() {
	defer s.cleanupTest()
	configuration := fmt.Sprintf("azstorage:\n  account-name: %s\n  endpoint: https://%s.blob.core.windows.net/\n  type: block\n  block-size-mb: 5000\n account-key: %s\n  mode: key\n  container: %s\n  fail-unsupported-op: true",
		storageTestConfigurationParameters.BlockAccount, storageTestConfigurationParameters.BlockAccount, storageTestConfigurationParameters.BlockKey, s.container)
	_, err := newTestAzStorage(configuration)
	s.assert.NotNil(err)
}

func (s *blockBlobTestSuite) TestDefault() {
	defer s.cleanupTest()
	s.assert.Equal(storageTestConfigurationParameters.BlockAccount, s.az.stConfig.authConfig.AccountName)
	s.assert.Equal(EAccountType.BLOCK(), s.az.stConfig.authConfig.AccountType)
	s.assert.False(s.az.stConfig.authConfig.UseHTTP)
	s.assert.Equal(storageTestConfigurationParameters.BlockKey, s.az.stConfig.authConfig.AccountKey)
	s.assert.Empty(s.az.stConfig.authConfig.SASKey)
	s.assert.Empty(s.az.stConfig.authConfig.ApplicationID)
	s.assert.Empty(s.az.stConfig.authConfig.ResourceID)
	s.assert.Empty(s.az.stConfig.authConfig.ActiveDirectoryEndpoint)
	s.assert.Empty(s.az.stConfig.authConfig.ClientSecret)
	s.assert.Empty(s.az.stConfig.authConfig.TenantID)
	s.assert.Empty(s.az.stConfig.authConfig.ClientID)
	s.assert.EqualValues("https://"+s.az.stConfig.authConfig.AccountName+".blob.core.windows.net/", s.az.stConfig.authConfig.Endpoint)
	s.assert.Equal(EAuthType.KEY(), s.az.stConfig.authConfig.AuthMode)
	s.assert.Equal(s.container, s.az.stConfig.container)
	s.assert.Empty(s.az.stConfig.prefixPath)
	s.assert.EqualValues(0, s.az.stConfig.blockSize)
	s.assert.EqualValues(32, s.az.stConfig.maxConcurrency)
	s.assert.Equal((*blob.AccessTier)(nil), s.az.stConfig.defaultTier)
	s.assert.EqualValues(0, s.az.stConfig.cancelListForSeconds)

	s.assert.EqualValues(5, s.az.stConfig.maxRetries)
	s.assert.EqualValues(900, s.az.stConfig.maxTimeout)
	s.assert.EqualValues(4, s.az.stConfig.backoffTime)
	s.assert.EqualValues(60, s.az.stConfig.maxRetryDelay)

	s.assert.Empty(s.az.stConfig.proxyAddress)
}

func randomString(length int) string {
	b := make([]byte, length)
	rand.Read(b)
	return fmt.Sprintf("%x", b)[:length]
}

func generateContainerName() string {
	return "fuseutc" + randomString(8)
}

func generateCPKInfo() (CPKEncryptionKey string, CPKEncryptionKeySHA256 string) {
	key := make([]byte, 32)
	rand.Read(key)
	CPKEncryptionKey = base64.StdEncoding.EncodeToString(key)
	hash := sha256.New()
	hash.Write(key)
	CPKEncryptionKeySHA256 = base64.StdEncoding.EncodeToString(hash.Sum(nil))
	return CPKEncryptionKey, CPKEncryptionKeySHA256
}

func generateDirectoryName() string {
	return "dir" + randomString(8)
}

func generateFileName() string {
	return "file" + randomString(8)
}

func (s *blockBlobTestSuite) TestModifyEndpoint() {
	defer s.cleanupTest()
	// Setup
	s.tearDownTestHelper(false) // Don't delete the generated container.
	config := fmt.Sprintf("azstorage:\n  account-name: %s\n  endpoint: https://%s.dfs.core.windows.net/\n  type: block\n  account-key: %s\n  mode: key\n  container: %s\n  fail-unsupported-op: true",
		storageTestConfigurationParameters.BlockAccount, storageTestConfigurationParameters.BlockAccount, storageTestConfigurationParameters.BlockKey, s.container)
	s.setupTestHelper(config, s.container, true)

	err := s.az.storage.TestPipeline()
	s.assert.Nil(err)
}

func (s *blockBlobTestSuite) TestNoEndpoint() {
	defer s.cleanupTest()
	// Setup
	s.tearDownTestHelper(false) // Don't delete the generated container.
	config := fmt.Sprintf("azstorage:\n  account-name: %s\n  type: block\n  account-key: %s\n  mode: key\n  container: %s\n  fail-unsupported-op: true",
		storageTestConfigurationParameters.BlockAccount, storageTestConfigurationParameters.BlockKey, s.container)
	s.setupTestHelper(config, s.container, true)

	err := s.az.storage.TestPipeline()
	s.assert.Nil(err)
}

func (s *blockBlobTestSuite) TestAccountType() {
	defer s.cleanupTest()
	// Setup
	s.tearDownTestHelper(false) // Don't delete the generated container.
	config := fmt.Sprintf("azstorage:\n  account-name: %s\n  type: block\n  account-key: %s\n  mode: key\n  container: %s\n  fail-unsupported-op: true",
		storageTestConfigurationParameters.BlockAccount, storageTestConfigurationParameters.BlockKey, s.container)
	s.setupTestHelper(config, s.container, true)

	val := s.az.storage.IsAccountADLS()
	s.assert.False(val)
}

func (s *blockBlobTestSuite) TestContainerNotFound() {
	defer s.cleanupTest()
	// Setup
	s.tearDownTestHelper(false) // Don't delete the generated container.
	config := fmt.Sprintf("azstorage:\n  account-name: %s\n  type: block\n  account-key: %s\n  mode: key\n  container: %s\n  fail-unsupported-op: true",
		storageTestConfigurationParameters.BlockAccount, storageTestConfigurationParameters.BlockKey, "foo")
	s.setupTestHelper(config, "foo", false)

	err := s.az.storage.TestPipeline()
	s.assert.NotNil(err)
	s.assert.Contains(err.Error(), "ContainerNotFound")
}

func (s *blockBlobTestSuite) TestListContainers() {
	defer s.cleanupTest()
	// Setup
	num := 10
	prefix := generateContainerName()
	for i := 0; i < num; i++ {
		c := s.serviceClient.NewContainerClient(prefix + fmt.Sprint(i))
		c.Create(ctx, nil)
		defer c.Delete(ctx, nil)
	}

	containers, err := s.az.ListContainers()

	s.assert.Nil(err)
	s.assert.NotNil(containers)
	s.assert.True(len(containers) >= num)
	count := 0
	for _, c := range containers {
		if strings.HasPrefix(c, prefix) {
			count++
		}
	}
	s.assert.EqualValues(num, count)
}

// TODO : ListContainersHuge: Maybe this is overkill?

func checkMetadata(metadata map[string]*string, key string, val string) bool {
	for k, v := range metadata {
		if v != nil && strings.ToLower(k) == key && val == *v {
			return true
		}
	}
	return false
}

func (s *blockBlobTestSuite) TestCreateDir() {
	defer s.cleanupTest()
	// Testing dir and dir/
	var paths = []string{generateDirectoryName(), generateDirectoryName() + "/"}
	for _, path := range paths {
		log.Debug(path)
		s.Run(path, func() {
			err := s.az.CreateDir(internal.CreateDirOptions{Name: path})

			s.assert.Nil(err)
			// Directory should be in the account
			dir := s.containerClient.NewBlobClient(internal.TruncateDirName(path))
			props, err := dir.GetProperties(ctx, nil)
			s.assert.Nil(err)
			s.assert.NotNil(props)
			s.assert.NotEmpty(props.Metadata)
			s.assert.True(checkMetadata(props.Metadata, folderKey, "true"))
		})
	}
}

func (s *blockBlobTestSuite) TestDeleteDir() {
	defer s.cleanupTest()
	// Testing dir and dir/
	var paths = []string{generateDirectoryName(), generateDirectoryName() + "/"}
	for _, path := range paths {
		log.Debug(path)
		s.Run(path, func() {
			s.az.CreateDir(internal.CreateDirOptions{Name: path})

			err := s.az.DeleteDir(internal.DeleteDirOptions{Name: path})

			s.assert.Nil(err)
			// Directory should not be in the account
			dir := s.containerClient.NewBlobClient(internal.TruncateDirName(path))
			_, err = dir.GetProperties(ctx, nil)
			s.assert.NotNil(err)
		})
	}
}

// Directory structure
// a/
//
//	 a/c1/
//	  a/c1/gc1
//		a/c2
//
// ab/
//
//	ab/c1
//
// ac
func generateNestedDirectory(path string) (*list.List, *list.List, *list.List) {
	aPaths := list.New()
	aPaths.PushBack(internal.TruncateDirName(path))

	aPaths.PushBack(filepath.Join(path, "c1"))
	aPaths.PushBack(filepath.Join(path, "c2"))
	aPaths.PushBack(filepath.Join(filepath.Join(path, "c1"), "gc1"))

	abPaths := list.New()
	path = internal.TruncateDirName(path)
	abPaths.PushBack(path + "b")
	abPaths.PushBack(filepath.Join(path+"b", "c1"))

	acPaths := list.New()
	acPaths.PushBack(path + "c")

	return aPaths, abPaths, acPaths
}

func (s *blockBlobTestSuite) setupHierarchy(base string) (*list.List, *list.List, *list.List) {
	// Hierarchy looks as follows
	// a/
	//  a/c1/
	//   a/c1/gc1
	//	a/c2
	// ab/
	//  ab/c1
	// ac
	s.az.CreateDir(internal.CreateDirOptions{Name: base})
	c1 := base + "/c1"
	s.az.CreateDir(internal.CreateDirOptions{Name: c1})
	gc1 := c1 + "/gc1"
	s.az.CreateFile(internal.CreateFileOptions{Name: gc1})
	c2 := base + "/c2"
	s.az.CreateFile(internal.CreateFileOptions{Name: c2})
	abPath := base + "b"
	s.az.CreateDir(internal.CreateDirOptions{Name: abPath})
	abc1 := abPath + "/c1"
	s.az.CreateFile(internal.CreateFileOptions{Name: abc1})
	acPath := base + "c"
	s.az.CreateFile(internal.CreateFileOptions{Name: acPath})

	a, ab, ac := generateNestedDirectory(base)

	// Validate the paths were setup correctly and all paths exist
	for p := a.Front(); p != nil; p = p.Next() {
		_, err := s.containerClient.NewBlobClient(p.Value.(string)).GetProperties(ctx, nil)
		s.assert.Nil(err)
	}
	for p := ab.Front(); p != nil; p = p.Next() {
		_, err := s.containerClient.NewBlobClient(p.Value.(string)).GetProperties(ctx, nil)
		s.assert.Nil(err)
	}
	for p := ac.Front(); p != nil; p = p.Next() {
		_, err := s.containerClient.NewBlobClient(p.Value.(string)).GetProperties(ctx, nil)
		s.assert.Nil(err)
	}
	return a, ab, ac
}

func (s *blockBlobTestSuite) TestDeleteDirError() {
	defer s.cleanupTest()
	// Setup
	name := generateDirectoryName()

	err := s.az.DeleteDir(internal.DeleteDirOptions{Name: name})

	// ENOENT error is ignored in blob accounts because when a directory which is not present is deleted,
	// libfuse first makes a GetAttr call and fails the operation stating that the directory is not present.
	// This change is added for cases when a directory is deleted for which we don't have marker blob.
	s.assert.Nil(err)
	// Directory should not be in the account
	dir := s.containerClient.NewBlobClient(name)
	_, err = dir.GetProperties(ctx, nil)
	s.assert.NotNil(err)
}

func (s *blockBlobTestSuite) TestIsDirEmpty() {
	defer s.cleanupTest()
	// Setup
	name := generateDirectoryName()
	s.az.CreateDir(internal.CreateDirOptions{Name: name})

	// Testing dir and dir/
	var paths = []string{name, name + "/"}
	for _, path := range paths {
		log.Debug(path)
		s.Run(path, func() {
			empty := s.az.IsDirEmpty(internal.IsDirEmptyOptions{Name: name})

			s.assert.True(empty)
		})
	}
}

func (s *blockBlobTestSuite) TestIsDirEmptyFalse() {
	defer s.cleanupTest()
	// Setup
	name := generateDirectoryName()
	s.az.CreateDir(internal.CreateDirOptions{Name: name})
	file := name + "/" + generateFileName()
	s.az.CreateFile(internal.CreateFileOptions{Name: file})

	empty := s.az.IsDirEmpty(internal.IsDirEmptyOptions{Name: name})

	s.assert.False(empty)
}

func (s *blockBlobTestSuite) TestIsDirEmptyError() {
	defer s.cleanupTest()
	// Setup
	name := generateDirectoryName()

	empty := s.az.IsDirEmpty(internal.IsDirEmptyOptions{Name: name})

	s.assert.True(empty) // Note: See comment in BlockBlob.List. BlockBlob behaves differently from Datalake

	// Directory should not be in the account
	dir := s.containerClient.NewBlobClient(name)
	_, err := dir.GetProperties(ctx, nil)
	s.assert.NotNil(err)
}

func (s *blockBlobTestSuite) TestReadDir() {
	defer s.cleanupTest()
	// This tests the default listBlocked = 0. It should return the expected paths.
	// Setup
	name := generateDirectoryName()
	s.az.CreateDir(internal.CreateDirOptions{Name: name})
	childName := name + "/" + generateFileName()
	s.az.CreateFile(internal.CreateFileOptions{Name: childName})

	// Testing dir and dir/
	var paths = []string{name, name + "/"}
	for _, path := range paths {
		log.Debug(path)
		s.Run(path, func() {
			entries, err := s.az.ReadDir(internal.ReadDirOptions{Name: path})
			s.assert.Nil(err)
			s.assert.EqualValues(1, len(entries))
		})
	}
}

func (s *blockBlobTestSuite) TestReadDirNoVirtualDirectory() {
	defer s.cleanupTest()
	// This tests the default listBlocked = 0. It should return the expected paths.
	// Setup
	name := generateDirectoryName()
	childName := name + "/" + generateFileName()
	s.az.CreateFile(internal.CreateFileOptions{Name: childName})

	// Testing dir and dir/
	var paths = []string{"", "/"}
	for _, path := range paths {
		log.Debug(path)
		s.Run(path, func() {
			entries, err := s.az.ReadDir(internal.ReadDirOptions{Name: path})
			s.assert.Nil(err)
			s.assert.EqualValues(1, len(entries))
			s.assert.EqualValues(name, entries[0].Path)
			s.assert.EqualValues(name, entries[0].Name)
			s.assert.True(entries[0].IsDir())
			s.assert.True(entries[0].IsModeDefault())
		})
	}
}

func (s *blockBlobTestSuite) TestReadDirHierarchy() {
	defer s.cleanupTest()
	// Setup
	base := generateDirectoryName()
	s.setupHierarchy(base)

	// ReadDir only reads the first level of the hierarchy
	entries, err := s.az.ReadDir(internal.ReadDirOptions{Name: base})
	s.assert.Nil(err)
	s.assert.EqualValues(2, len(entries))
	// Check the dir
	s.assert.EqualValues(base+"/c1", entries[0].Path)
	s.assert.EqualValues("c1", entries[0].Name)
	s.assert.True(entries[0].IsDir())
	s.assert.True(entries[0].IsModeDefault())
	// Check the file
	s.assert.EqualValues(base+"/c2", entries[1].Path)
	s.assert.EqualValues("c2", entries[1].Name)
	s.assert.False(entries[1].IsDir())
	s.assert.True(entries[1].IsModeDefault())
}

func (s *blockBlobTestSuite) TestReadDirRoot() {
	defer s.cleanupTest()
	// Setup
	base := generateDirectoryName()
	s.setupHierarchy(base)

	// Testing dir and dir/
	var paths = []string{"", "/"}
	for _, path := range paths {
		log.Debug(path)
		s.Run(path, func() {
			// ReadDir only reads the first level of the hierarchy
			entries, err := s.az.ReadDir(internal.ReadDirOptions{Name: path})
			s.assert.Nil(err)
			s.assert.EqualValues(3, len(entries))
			// Check the base dir
			s.assert.EqualValues(base, entries[0].Path)
			s.assert.EqualValues(base, entries[0].Name)
			s.assert.True(entries[0].IsDir())
			s.assert.True(entries[0].IsModeDefault())
			// Check the baseb dir
			s.assert.EqualValues(base+"b", entries[1].Path)
			s.assert.EqualValues(base+"b", entries[1].Name)
			s.assert.True(entries[1].IsDir())
			s.assert.True(entries[1].IsModeDefault())
			// Check the basec file
			s.assert.EqualValues(base+"c", entries[2].Path)
			s.assert.EqualValues(base+"c", entries[2].Name)
			s.assert.False(entries[2].IsDir())
			s.assert.True(entries[2].IsModeDefault())
		})
	}
}

func (s *blockBlobTestSuite) TestReadDirSubDir() {
	defer s.cleanupTest()
	// Setup
	base := generateDirectoryName()
	s.setupHierarchy(base)

	// ReadDir only reads the first level of the hierarchy
	entries, err := s.az.ReadDir(internal.ReadDirOptions{Name: base + "/c1"})
	s.assert.Nil(err)
	s.assert.EqualValues(1, len(entries))
	// Check the dir
	s.assert.EqualValues(base+"/c1"+"/gc1", entries[0].Path)
	s.assert.EqualValues("gc1", entries[0].Name)
	s.assert.False(entries[0].IsDir())
	s.assert.True(entries[0].IsModeDefault())
}

func (s *blockBlobTestSuite) TestReadDirSubDirPrefixPath() {
	defer s.cleanupTest()
	// Setup
	base := generateDirectoryName()
	s.setupHierarchy(base)

	s.az.storage.SetPrefixPath(base)

	// ReadDir only reads the first level of the hierarchy
	entries, err := s.az.ReadDir(internal.ReadDirOptions{Name: "/c1"})
	s.assert.Nil(err)
	s.assert.EqualValues(1, len(entries))
	// Check the dir
	s.assert.EqualValues("c1"+"/gc1", entries[0].Path)
	s.assert.EqualValues("gc1", entries[0].Name)
	s.assert.False(entries[0].IsDir())
	s.assert.True(entries[0].IsModeDefault())
}

func (s *blockBlobTestSuite) TestReadDirError() {
	defer s.cleanupTest()
	// Setup
	name := generateDirectoryName()

	entries, err := s.az.ReadDir(internal.ReadDirOptions{Name: name})

	s.assert.Nil(err) // Note: See comment in BlockBlob.List. BlockBlob behaves differently from Datalake
	s.assert.Empty(entries)
	// Directory should not be in the account
	dir := s.containerClient.NewBlobClient(name)
	_, err = dir.GetProperties(ctx, nil)
	s.assert.NotNil(err)
}

func (s *blockBlobTestSuite) TestReadDirListBlocked() {
	defer s.cleanupTest()
	// Setup
	s.tearDownTestHelper(false) // Don't delete the generated container.

	listBlockedTime := 10
	config := fmt.Sprintf("azstorage:\n  account-name: %s\n  endpoint: https://%s.blob.core.windows.net/\n  type: block\n  account-key: %s\n  mode: key\n  container: %s\n  block-list-on-mount-sec: %d\n  fail-unsupported-op: true\n",
		storageTestConfigurationParameters.BlockAccount, storageTestConfigurationParameters.BlockAccount, storageTestConfigurationParameters.BlockKey, s.container, listBlockedTime)
	s.setupTestHelper(config, s.container, true)

	name := generateDirectoryName()
	s.az.CreateDir(internal.CreateDirOptions{Name: name})
	childName := name + "/" + generateFileName()
	s.az.CreateFile(internal.CreateFileOptions{Name: childName})

	entries, err := s.az.ReadDir(internal.ReadDirOptions{Name: name})
	s.assert.Nil(err)
	s.assert.EqualValues(0, len(entries)) // Since we block the list, it will return an empty list.
}

func (s *blockBlobTestSuite) TestStreamDirSmallCountNoDuplicates() {
	defer s.cleanupTest()
	// Setup
	s.az.CreateFile(internal.CreateFileOptions{Name: "blob1.txt"})
	s.az.CreateFile(internal.CreateFileOptions{Name: "blob2.txt"})
	s.az.CreateFile(internal.CreateFileOptions{Name: "newblob1.txt"})
	s.az.CreateFile(internal.CreateFileOptions{Name: "newblob2.txt"})
	s.az.CreateDir(internal.CreateDirOptions{Name: "myfolder"})
	s.az.CreateFile(internal.CreateFileOptions{Name: "myfolder/newblobA.txt"})
	s.az.CreateFile(internal.CreateFileOptions{Name: "myfolder/newblobB.txt"})

	var iteration int = 0
	var marker string = ""
	blobList := make([]*internal.ObjAttr, 0)

	for {
		new_list, new_marker, err := s.az.StreamDir(internal.StreamDirOptions{Name: "/", Token: marker, Count: 1})
		s.assert.Nil(err)
		blobList = append(blobList, new_list...)
		marker = new_marker
		iteration++

		log.Debug("AzStorage::ReadDir : So far retrieved %d objects in %d iterations", len(blobList), iteration)
		if new_marker == "" {
			break
		}
	}

	s.assert.EqualValues(5, len(blobList))
}

func (s *blockBlobTestSuite) TestRenameDir() {
	defer s.cleanupTest()
	// Test handling "dir" and "dir/"
	var inputs = []struct {
		src string
		dst string
	}{
		{src: generateDirectoryName(), dst: generateDirectoryName()},
		{src: generateDirectoryName() + "/", dst: generateDirectoryName()},
		{src: generateDirectoryName(), dst: generateDirectoryName() + "/"},
		{src: generateDirectoryName() + "/", dst: generateDirectoryName() + "/"},
	}

	for _, input := range inputs {
		s.Run(input.src+"->"+input.dst, func() {
			// Setup
			s.az.CreateDir(internal.CreateDirOptions{Name: input.src})

			err := s.az.RenameDir(internal.RenameDirOptions{Src: input.src, Dst: input.dst})
			s.assert.Nil(err)
			// Src should not be in the account
			dir := s.containerClient.NewBlobClient(internal.TruncateDirName(input.src))
			_, err = dir.GetProperties(ctx, nil)
			s.assert.NotNil(err)

			// Dst should be in the account
			dir = s.containerClient.NewBlobClient(internal.TruncateDirName(input.dst))
			_, err = dir.GetProperties(ctx, nil)
			s.assert.Nil(err)
		})
	}

}

func (s *blockBlobTestSuite) TestRenameDirHierarchy() {
	defer s.cleanupTest()
	// Setup
	baseSrc := generateDirectoryName()
	aSrc, abSrc, acSrc := s.setupHierarchy(baseSrc)
	baseDst := generateDirectoryName()
	aDst, abDst, acDst := generateNestedDirectory(baseDst)

	err := s.az.RenameDir(internal.RenameDirOptions{Src: baseSrc, Dst: baseDst})
	s.assert.Nil(err)

	// Source
	// aSrc paths should be deleted
	for p := aSrc.Front(); p != nil; p = p.Next() {
		_, err = s.containerClient.NewBlobClient(p.Value.(string)).GetProperties(ctx, nil)
		s.assert.NotNil(err)
	}
	abSrc.PushBackList(acSrc) // abSrc and acSrc paths should exist
	for p := abSrc.Front(); p != nil; p = p.Next() {
		_, err = s.containerClient.NewBlobClient(p.Value.(string)).GetProperties(ctx, nil)
		s.assert.Nil(err)
	}
	// Destination
	// aDst paths should exist
	for p := aDst.Front(); p != nil; p = p.Next() {
		_, err = s.containerClient.NewBlobClient(p.Value.(string)).GetProperties(ctx, nil)
		s.assert.Nil(err)
	}
	abDst.PushBackList(acDst) // abDst and acDst paths should not exist
	for p := abDst.Front(); p != nil; p = p.Next() {
		_, err = s.containerClient.NewBlobClient(p.Value.(string)).GetProperties(ctx, nil)
		s.assert.NotNil(err)
	}
}

func (s *blockBlobTestSuite) TestRenameDirSubDirPrefixPath() {
	defer s.cleanupTest()
	// Setup
	baseSrc := generateDirectoryName()
	aSrc, abSrc, acSrc := s.setupHierarchy(baseSrc)
	baseDst := generateDirectoryName()

	s.az.storage.SetPrefixPath(baseSrc)

	err := s.az.RenameDir(internal.RenameDirOptions{Src: "c1", Dst: baseDst})
	s.assert.Nil(err)

	// Source
	// aSrc paths under c1 should be deleted
	for p := aSrc.Front(); p != nil; p = p.Next() {
		path := p.Value.(string)
		_, err = s.containerClient.NewBlobClient(path).GetProperties(ctx, nil)
		if strings.HasPrefix(path, baseSrc+"/c1") {
			s.assert.NotNil(err)
		} else {
			s.assert.Nil(err)
		}
	}
	abSrc.PushBackList(acSrc) // abSrc and acSrc paths should exist
	for p := abSrc.Front(); p != nil; p = p.Next() {
		_, err = s.containerClient.NewBlobClient(p.Value.(string)).GetProperties(ctx, nil)
		s.assert.Nil(err)
	}
	// Destination
	// aDst paths should exist -> aDst and aDst/gc1
	_, err = s.containerClient.NewBlobClient(baseSrc+"/"+baseDst).GetProperties(ctx, nil)
	s.assert.Nil(err)
	_, err = s.containerClient.NewBlobClient(baseSrc+"/"+baseDst+"/gc1").GetProperties(ctx, nil)
	s.assert.Nil(err)
}

func (s *blockBlobTestSuite) TestRenameDirError() {
	defer s.cleanupTest()
	// Setup
	src := generateDirectoryName()
	dst := generateDirectoryName()

	err := s.az.RenameDir(internal.RenameDirOptions{Src: src, Dst: dst})

	s.assert.NotNil(err)
	s.assert.EqualValues(syscall.ENOENT, err)
	// Neither directory should be in the account
	dir := s.containerClient.NewBlobClient(src)
	_, err = dir.GetProperties(ctx, nil)
	s.assert.NotNil(err)
	dir = s.containerClient.NewBlobClient(dst)
	_, err = dir.GetProperties(ctx, nil)
	s.assert.NotNil(err)
}

func (s *blockBlobTestSuite) TestRenameDirWithoutMarker() {
	defer s.cleanupTest()
	src := generateDirectoryName()
	dst := generateDirectoryName()

	for i := 0; i < 5; i++ {
		blockBlobClient := s.containerClient.NewBlockBlobClient(fmt.Sprintf("%s/blob%v", src, i))
		testData := "test data"
		data := []byte(testData)
		// upload blob
		err := uploadReaderAtToBlockBlob(ctx, bytes.NewReader(data), int64(len(data)), int64(len(data)), blockBlobClient, nil)
		s.assert.Nil(err)

		_, err = blockBlobClient.GetProperties(ctx, nil)
		s.assert.Nil(err)
	}

	err := s.az.RenameDir(internal.RenameDirOptions{Src: src, Dst: dst})
	s.assert.Nil(err)

	for i := 0; i < 5; i++ {
		srcBlobClient := s.containerClient.NewBlockBlobClient(fmt.Sprintf("%s/blob%v", src, i))
		dstBlobClient := s.containerClient.NewBlockBlobClient(fmt.Sprintf("%s/blob%v", dst, i))

		_, err = srcBlobClient.GetProperties(ctx, nil)
		s.assert.NotNil(err)

		_, err = dstBlobClient.GetProperties(ctx, nil)
		s.assert.Nil(err)
	}

	// verify that the marker blob does not exist for both source and destination directory
	srcDirClient := s.containerClient.NewBlockBlobClient(src)
	dstDirClient := s.containerClient.NewBlockBlobClient(dst)

	_, err = srcDirClient.GetProperties(ctx, nil)
	s.assert.NotNil(err)

	_, err = dstDirClient.GetProperties(ctx, nil)
	s.assert.NotNil(err)
}

func (s *blockBlobTestSuite) TestCreateFile() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()

	h, err := s.az.CreateFile(internal.CreateFileOptions{Name: name})

	s.assert.Nil(err)
	s.assert.NotNil(h)
	s.assert.EqualValues(name, h.Path)
	s.assert.EqualValues(0, h.Size)

	// File should be in the account
	file := s.containerClient.NewBlobClient(name)
	props, err := file.GetProperties(ctx, nil)
	s.assert.Nil(err)
	s.assert.NotNil(props)
	s.assert.Empty(props.Metadata)
}

func (s *blockBlobTestSuite) TestOpenFile() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	s.az.CreateFile(internal.CreateFileOptions{Name: name})

	h, err := s.az.OpenFile(internal.OpenFileOptions{Name: name})
	s.assert.Nil(err)
	s.assert.NotNil(h)
	s.assert.EqualValues(name, h.Path)
	s.assert.EqualValues(0, h.Size)
}

func (s *blockBlobTestSuite) TestOpenFileError() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()

	h, err := s.az.OpenFile(internal.OpenFileOptions{Name: name})
	s.assert.NotNil(err)
	s.assert.EqualValues(syscall.ENOENT, err)
	s.assert.Nil(h)
}

func (s *blockBlobTestSuite) TestOpenFileSize() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	size := 10
	s.az.CreateFile(internal.CreateFileOptions{Name: name})
	s.az.TruncateFile(internal.TruncateFileOptions{Name: name, Size: int64(size)})

	h, err := s.az.OpenFile(internal.OpenFileOptions{Name: name})
	s.assert.Nil(err)
	s.assert.NotNil(h)
	s.assert.EqualValues(name, h.Path)
	s.assert.EqualValues(size, h.Size)
}

func (s *blockBlobTestSuite) TestCloseFile() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h, _ := s.az.CreateFile(internal.CreateFileOptions{Name: name})

	// This method does nothing.
	err := s.az.CloseFile(internal.CloseFileOptions{Handle: h})
	s.assert.Nil(err)
}

func (s *blockBlobTestSuite) TestCloseFileFakeHandle() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h := handlemap.NewHandle(name)

	// This method does nothing.
	err := s.az.CloseFile(internal.CloseFileOptions{Handle: h})
	s.assert.Nil(err)
}

func (s *blockBlobTestSuite) TestDeleteFile() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	s.az.CreateFile(internal.CreateFileOptions{Name: name})

	err := s.az.DeleteFile(internal.DeleteFileOptions{Name: name})
	s.assert.Nil(err)

	// File should not be in the account
	file := s.containerClient.NewBlobClient(name)
	_, err = file.GetProperties(ctx, nil)
	s.assert.NotNil(err)
}

func (s *blockBlobTestSuite) TestDeleteFileError() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()

	err := s.az.DeleteFile(internal.DeleteFileOptions{Name: name})
	s.assert.NotNil(err)
	s.assert.EqualValues(syscall.ENOENT, err)

	// File should not be in the account
	file := s.containerClient.NewBlobClient(name)
	_, err = file.GetProperties(ctx, nil)
	s.assert.NotNil(err)
}

func (s *blockBlobTestSuite) TestRenameFile() {
	defer s.cleanupTest()
	// Setup
	src := generateFileName()
	s.az.CreateFile(internal.CreateFileOptions{Name: src})
	dst := generateFileName()

	err := s.az.RenameFile(internal.RenameFileOptions{Src: src, Dst: dst})
	s.assert.Nil(err)

	// Src should not be in the account
	source := s.containerClient.NewBlobClient(src)
	_, err = source.GetProperties(ctx, nil)
	s.assert.NotNil(err)
	// Dst should be in the account
	destination := s.containerClient.NewBlobClient(dst)
	_, err = destination.GetProperties(ctx, nil)
	s.assert.Nil(err)
}

func (s *blockBlobTestSuite) TestRenameFileMetadataConservation() {
	defer s.cleanupTest()
	// Setup
	src := generateFileName()
	source := s.containerClient.NewBlobClient(src)
	s.az.CreateFile(internal.CreateFileOptions{Name: src})
	// Add srcMeta to source
	srcMeta := make(map[string]*string)
	srcMeta["foo"] = to.Ptr("bar")
	source.SetMetadata(ctx, srcMeta, nil)
	dst := generateFileName()

	err := s.az.RenameFile(internal.RenameFileOptions{Src: src, Dst: dst})
	s.assert.Nil(err)

	// Src should not be in the account
	_, err = source.GetProperties(ctx, nil)
	s.assert.NotNil(err)
	// Dst should be in the account
	destination := s.containerClient.NewBlobClient(dst)
	props, err := destination.GetProperties(ctx, nil)
	s.assert.Nil(err)
	// Dst should have metadata
	s.assert.True(checkMetadata(props.Metadata, "foo", "bar"))
}

func (s *blockBlobTestSuite) TestRenameFileError() {
	defer s.cleanupTest()
	// Setup
	src := generateFileName()
	dst := generateFileName()

	err := s.az.RenameFile(internal.RenameFileOptions{Src: src, Dst: dst})
	s.assert.NotNil(err)
	s.assert.EqualValues(syscall.ENOENT, err)

	// Src and destination should not be in the account
	source := s.containerClient.NewBlobClient(src)
	_, err = source.GetProperties(ctx, nil)
	s.assert.NotNil(err)
	destination := s.containerClient.NewBlobClient(dst)
	_, err = destination.GetProperties(ctx, nil)
	s.assert.NotNil(err)
}

func (s *blockBlobTestSuite) TestRenameFileNoReplace() {
	defer s.cleanupTest()
	// Setup
	src := generateFileName()
	s.az.CreateFile(internal.CreateFileOptions{Name: src})
	dst := generateFileName()
	s.az.CreateFile(internal.CreateFileOptions{Name: dst})

	// Attempt to rename src to dst, which already exists
	err := s.az.RenameFile(internal.RenameFileOptions{Src: src, Dst: dst, NoReplace: true})
	s.assert.NotNil(err)
	s.assert.EqualValues(syscall.EEXIST, err)

	// Src should be in the account
	source := s.containerClient.NewBlobClient(src)
	_, err = source.GetProperties(ctx, nil)
	s.assert.Nil(err)
	// Dst should  be in the account
	destination := s.containerClient.NewBlobClient(dst)
	_, err = destination.GetProperties(ctx, nil)
	s.assert.Nil(err)
}

func (s *blockBlobTestSuite) TestReadFile() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h, err := s.az.CreateFile(internal.CreateFileOptions{Name: name})
	s.assert.Nil(err)
	testData := "test data"
	data := []byte(testData)
	s.az.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data})
	h, _ = s.az.OpenFile(internal.OpenFileOptions{Name: name})

	output, err := s.az.ReadFile(internal.ReadFileOptions{Handle: h})
	s.assert.Nil(err)
	s.assert.EqualValues(testData, output)
}

func (s *blockBlobTestSuite) TestReadFileError() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h := handlemap.NewHandle(name)

	_, err := s.az.ReadFile(internal.ReadFileOptions{Handle: h})
	s.assert.NotNil(err)
	s.assert.EqualValues(syscall.ENOENT, err)
}

func (s *blockBlobTestSuite) TestReadInBuffer() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h, _ := s.az.CreateFile(internal.CreateFileOptions{Name: name})
	testData := "test data"
	data := []byte(testData)
	s.az.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data})
	h, _ = s.az.OpenFile(internal.OpenFileOptions{Name: name})

	output := make([]byte, 5)
	len, err := s.az.ReadInBuffer(internal.ReadInBufferOptions{Handle: h, Offset: 0, Data: output})
	s.assert.Nil(err)
	s.assert.EqualValues(5, len)
	s.assert.EqualValues(testData[:5], output)
}

func (s *blockBlobTestSuite) TestReadInBufferWithoutHandle() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h, err := s.az.CreateFile(internal.CreateFileOptions{Name: name})
	s.assert.Nil(err)
	s.assert.NotNil(h)

	testData := "test data"
	data := []byte(testData)
	n, err := s.az.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data})
	s.assert.Nil(err)
	s.assert.Equal(n, len(data))

	output := make([]byte, 5)
	len, err := s.az.ReadInBuffer(internal.ReadInBufferOptions{Offset: 0, Data: output, Path: name, Size: (int64)(len(data))})
	s.assert.Nil(err)
	s.assert.EqualValues(5, len)
	s.assert.EqualValues(testData[:5], output)
}

func (s *blockBlobTestSuite) TestReadInBufferEmptyPath() {
	defer s.cleanupTest()

	output := make([]byte, 5)
	len, err := s.az.ReadInBuffer(internal.ReadInBufferOptions{Offset: 0, Data: output, Size: 5})
	s.assert.NotNil(err)
	s.assert.EqualValues(0, len)
	s.assert.Equal(err.Error(), "path not given for download")
}

func (bbTestSuite *blockBlobTestSuite) TestReadInBufferWithETAG() {
	defer bbTestSuite.cleanupTest()
	// Setup
	name := generateFileName()
	handle, _ := bbTestSuite.az.CreateFile(internal.CreateFileOptions{Name: name})
	testData := "test data"
	data := []byte(testData)
	bbTestSuite.az.WriteFile(internal.WriteFileOptions{Handle: handle, Offset: 0, Data: data})
	handle, _ = bbTestSuite.az.OpenFile(internal.OpenFileOptions{Name: name})

	output := make([]byte, 5)
	var etag string
	len, err := bbTestSuite.az.ReadInBuffer(internal.ReadInBufferOptions{Handle: handle, Offset: 0, Data: output, Etag: &etag})
	bbTestSuite.assert.Nil(err)
	bbTestSuite.assert.NotEqual(etag, "")
	bbTestSuite.assert.EqualValues(5, len)
	bbTestSuite.assert.EqualValues(testData[:5], output)
	_ = bbTestSuite.az.CloseFile(internal.CloseFileOptions{Handle: handle})
}

func (bbTestSuite *blockBlobTestSuite) TestReadInBufferWithETAGMismatch() {
	defer bbTestSuite.cleanupTest()
	// Setup
	name := generateFileName()
	handle, _ := bbTestSuite.az.CreateFile(internal.CreateFileOptions{Name: name})
	testData := "test data 12345678910"
	data := []byte(testData)
	bbTestSuite.az.WriteFile(internal.WriteFileOptions{Handle: handle, Offset: 0, Data: data})
	_ = bbTestSuite.az.CloseFile(internal.CloseFileOptions{Handle: handle})

	attr, err := bbTestSuite.az.GetAttr(internal.GetAttrOptions{Name: name})
	bbTestSuite.assert.Nil(err)
	bbTestSuite.assert.NotNil(attr)
	bbTestSuite.assert.NotEqual("", attr.ETag)
	bbTestSuite.assert.Equal(int64(len(data)), attr.Size)

	output := make([]byte, 5)
	var etag string

	handle, _ = bbTestSuite.az.OpenFile(internal.OpenFileOptions{Name: name})
	_, err = bbTestSuite.az.ReadInBuffer(internal.ReadInBufferOptions{Handle: handle, Offset: 0, Data: output, Etag: &etag})
	bbTestSuite.assert.Nil(err)
	bbTestSuite.assert.NotEqual(etag, "")
	etag = strings.Trim(etag, `"`)
	bbTestSuite.assert.Equal(etag, attr.ETag)

	// Update the file in parallel using another handle
	handle1, err := bbTestSuite.az.OpenFile(internal.OpenFileOptions{Name: name})
	bbTestSuite.assert.Nil(err)
	testData = "test data 12345678910 123123123123123123123"
	data = []byte(testData)
	bbTestSuite.az.WriteFile(internal.WriteFileOptions{Handle: handle1, Offset: 0, Data: data})
	_ = bbTestSuite.az.CloseFile(internal.CloseFileOptions{Handle: handle1})

	// Read data back using older handle
	_, err = bbTestSuite.az.ReadInBuffer(internal.ReadInBufferOptions{Handle: handle, Offset: 5, Data: output, Etag: &etag})
	bbTestSuite.assert.Nil(err)
	bbTestSuite.assert.NotEqual(etag, "")
	etag = strings.Trim(etag, `"`)
	bbTestSuite.assert.NotEqual(etag, attr.ETag)

	_ = bbTestSuite.az.CloseFile(internal.CloseFileOptions{Handle: handle})
}

func (s *blockBlobTestSuite) TestReadInBufferLargeBuffer() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h, _ := s.az.CreateFile(internal.CreateFileOptions{Name: name})
	testData := "test data"
	data := []byte(testData)
	s.az.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data})
	h, _ = s.az.OpenFile(internal.OpenFileOptions{Name: name})

	output := make([]byte, 1000) // Testing that passing in a super large buffer will still work
	len, err := s.az.ReadInBuffer(internal.ReadInBufferOptions{Handle: h, Offset: 0, Data: output})
	s.assert.Nil(err)
	s.assert.EqualValues(h.Size, len)
	s.assert.EqualValues(testData, output[:h.Size])
}

func (s *blockBlobTestSuite) TestReadInBufferEmpty() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h, _ := s.az.CreateFile(internal.CreateFileOptions{Name: name})

	output := make([]byte, 10)
	len, err := s.az.ReadInBuffer(internal.ReadInBufferOptions{Handle: h, Offset: 0, Data: output})
	s.assert.Nil(err)
	s.assert.EqualValues(0, len)
}

func (s *blockBlobTestSuite) TestReadInBufferBadRange() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h := handlemap.NewHandle(name)
	h.Size = 10

	_, err := s.az.ReadInBuffer(internal.ReadInBufferOptions{Handle: h, Offset: 20, Data: make([]byte, 2)})
	s.assert.NotNil(err)
	s.assert.EqualValues(syscall.ERANGE, err)
}

func (s *blockBlobTestSuite) TestReadInBufferError() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h := handlemap.NewHandle(name)
	h.Size = 10

	_, err := s.az.ReadInBuffer(internal.ReadInBufferOptions{Handle: h, Offset: 0, Data: make([]byte, 2)})
	s.assert.NotNil(err)
	s.assert.EqualValues(syscall.ENOENT, err)
}

func (s *blockBlobTestSuite) TestWriteFile() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h, _ := s.az.CreateFile(internal.CreateFileOptions{Name: name})

	testData := "test data"
	data := []byte(testData)
	count, err := s.az.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data})
	s.assert.Nil(err)
	s.assert.EqualValues(len(data), count)

	// Blob should have updated data
	file := s.containerClient.NewBlobClient(name)
	resp, err := file.DownloadStream(ctx, &blob.DownloadStreamOptions{
		Range: blob.HTTPRange{Offset: 0, Count: int64(len(data))},
	})
	s.assert.Nil(err)
	output, _ := io.ReadAll(resp.Body)
	s.assert.EqualValues(testData, output)
}

func (s *blockBlobTestSuite) TestTruncateSmallFileSmaller() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h, _ := s.az.CreateFile(internal.CreateFileOptions{Name: name})
	testData := "test data"
	data := []byte(testData)
	truncatedLength := 5
	s.az.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data})

	err := s.az.TruncateFile(internal.TruncateFileOptions{Name: name, Size: int64(truncatedLength)})
	s.assert.Nil(err)

	// Blob should have updated data
	file := s.containerClient.NewBlobClient(name)
	resp, err := file.DownloadStream(ctx, &blob.DownloadStreamOptions{
		Range: blob.HTTPRange{Offset: 0, Count: int64(truncatedLength)},
	})
	s.assert.Nil(err)
	s.assert.NotNil(resp.ContentLength)
	s.assert.EqualValues(truncatedLength, *resp.ContentLength)
	output, _ := io.ReadAll(resp.Body)
	s.assert.EqualValues(testData[:truncatedLength], output[:])
}

func (s *blockBlobTestSuite) TestTruncateEmptyFileToLargeSize() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h, _ := s.az.CreateFile(internal.CreateFileOptions{Name: name})
	s.assert.NotNil(h)

	blobSize := int64((1 * common.GbToBytes) + 13)
	err := s.az.TruncateFile(internal.TruncateFileOptions{Name: name, Size: blobSize})
	s.assert.Nil(err)

	props, err := s.az.GetAttr(internal.GetAttrOptions{Name: name})
	s.assert.Nil(err)
	s.assert.NotNil(props)
	s.assert.EqualValues(blobSize, props.Size)

	err = s.az.DeleteFile(internal.DeleteFileOptions{Name: name})
	s.assert.Nil(err)
}

func (s *blockBlobTestSuite) TestTruncateChunkedFileSmaller() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	s.az.CreateFile(internal.CreateFileOptions{Name: name})
	testData := "test data"
	data := []byte(testData)
	truncatedLength := 5
	// use our method to make the max upload size (size before a blob is broken down to blocks) to 4 Bytes
	err := uploadReaderAtToBlockBlob(ctx, bytes.NewReader(data), int64(len(data)), 4, s.containerClient.NewBlockBlobClient(name), &blockblob.UploadBufferOptions{
		BlockSize: 4,
	})
	s.assert.Nil(err)

	err = s.az.TruncateFile(internal.TruncateFileOptions{Name: name, Size: int64(truncatedLength)})
	s.assert.Nil(err)

	// Blob should have updated data
	file := s.containerClient.NewBlobClient(name)
	resp, err := file.DownloadStream(ctx, &blob.DownloadStreamOptions{
		Range: blob.HTTPRange{Offset: 0, Count: int64(truncatedLength)},
	})
	s.assert.Nil(err)
	s.assert.NotNil(resp.ContentLength)
	s.assert.EqualValues(truncatedLength, *resp.ContentLength)
	output, _ := io.ReadAll(resp.Body)
	s.assert.EqualValues(testData[:truncatedLength], output)
}

func (s *blockBlobTestSuite) TestTruncateSmallFileEqual() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h, _ := s.az.CreateFile(internal.CreateFileOptions{Name: name})
	testData := "test data"
	data := []byte(testData)
	truncatedLength := 9
	s.az.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data})

	err := s.az.TruncateFile(internal.TruncateFileOptions{Name: name, Size: int64(truncatedLength)})
	s.assert.Nil(err)

	// Blob should have updated data
	file := s.containerClient.NewBlobClient(name)
	resp, err := file.DownloadStream(ctx, &blob.DownloadStreamOptions{
		Range: blob.HTTPRange{Offset: 0, Count: int64(truncatedLength)},
	})
	s.assert.Nil(err)
	s.assert.NotNil(resp.ContentLength)
	s.assert.EqualValues(truncatedLength, *resp.ContentLength)
	output, _ := io.ReadAll(resp.Body)
	s.assert.EqualValues(testData, output)
}

func (s *blockBlobTestSuite) TestTruncateChunkedFileEqual() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	s.az.CreateFile(internal.CreateFileOptions{Name: name})
	testData := "test data"
	data := []byte(testData)
	truncatedLength := 9
	// use our method to make the max upload size (size before a blob is broken down to blocks) to 4 Bytes
	err := uploadReaderAtToBlockBlob(ctx, bytes.NewReader(data), int64(len(data)), 4, s.containerClient.NewBlockBlobClient(name), &blockblob.UploadBufferOptions{
		BlockSize: 4,
	})
	s.assert.Nil(err)

	err = s.az.TruncateFile(internal.TruncateFileOptions{Name: name, Size: int64(truncatedLength)})
	s.assert.Nil(err)

	// Blob should have updated data
	file := s.containerClient.NewBlobClient(name)
	resp, err := file.DownloadStream(ctx, &blob.DownloadStreamOptions{
		Range: blob.HTTPRange{Offset: 0, Count: int64(truncatedLength)},
	})
	s.assert.Nil(err)
	s.assert.NotNil(resp.ContentLength)
	s.assert.EqualValues(truncatedLength, *resp.ContentLength)
	output, _ := io.ReadAll(resp.Body)
	s.assert.EqualValues(testData, output)
}

func (s *blockBlobTestSuite) TestTruncateSmallFileBigger() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h, _ := s.az.CreateFile(internal.CreateFileOptions{Name: name})
	testData := "test data"
	data := []byte(testData)
	truncatedLength := 15
	s.az.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data})

	err := s.az.TruncateFile(internal.TruncateFileOptions{Name: name, Size: int64(truncatedLength)})
	s.assert.Nil(err)

	// Blob should have updated data
	file := s.containerClient.NewBlobClient(name)
	resp, err := file.DownloadStream(ctx, &blob.DownloadStreamOptions{
		Range: blob.HTTPRange{Offset: 0, Count: int64(truncatedLength)},
	})
	s.assert.Nil(err)
	s.assert.NotNil(resp.ContentLength)
	s.assert.EqualValues(truncatedLength, *resp.ContentLength)
	output, _ := io.ReadAll(resp.Body)
	s.assert.EqualValues(testData, output[:len(data)])
}

func (s *blockBlobTestSuite) TestTruncateChunkedFileBigger() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	s.az.CreateFile(internal.CreateFileOptions{Name: name})
	testData := "test data"
	data := []byte(testData)
	truncatedLength := 15
	// use our method to make the max upload size (size before a blob is broken down to blocks) to 4 Bytes
	err := uploadReaderAtToBlockBlob(ctx, bytes.NewReader(data), int64(len(data)), 4, s.containerClient.NewBlockBlobClient(name), &blockblob.UploadBufferOptions{
		BlockSize: 4,
	})
	s.assert.Nil(err)

	err = s.az.TruncateFile(internal.TruncateFileOptions{Name: name, Size: int64(truncatedLength)})
	s.assert.Nil(err)

	// Blob should have updated data
	file := s.containerClient.NewBlobClient(name)
	resp, err := file.DownloadStream(ctx, &blob.DownloadStreamOptions{
		Range: blob.HTTPRange{Offset: 0, Count: int64(truncatedLength)},
	})
	s.assert.Nil(err)
	s.assert.NotNil(resp.ContentLength)
	s.assert.EqualValues(truncatedLength, *resp.ContentLength)
	output, _ := io.ReadAll(resp.Body)
	s.assert.EqualValues(testData, output[:len(data)])
}

func (s *blockBlobTestSuite) TestTruncateFileError() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()

	err := s.az.TruncateFile(internal.TruncateFileOptions{Name: name})
	s.assert.NotNil(err)
	s.assert.EqualValues(syscall.ENOENT, err)
}

func (s *blockBlobTestSuite) TestWriteSmallFile() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h, _ := s.az.CreateFile(internal.CreateFileOptions{Name: name})
	testData := "test data"
	data := []byte(testData)
	dataLen := len(data)
	_, err := s.az.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data})
	s.assert.Nil(err)
	f, _ := os.CreateTemp("", name+".tmp")
	defer os.Remove(f.Name())

	err = s.az.CopyToFile(internal.CopyToFileOptions{Name: name, File: f})
	s.assert.Nil(err)

	output := make([]byte, len(data))
	f, _ = os.Open(f.Name())
	len, err := f.Read(output)
	s.assert.Nil(err)
	s.assert.EqualValues(dataLen, len)
	s.assert.EqualValues(testData, output)
	f.Close()
}

func (s *blockBlobTestSuite) TestOverwriteSmallFile() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h, _ := s.az.CreateFile(internal.CreateFileOptions{Name: name})
	testData := "test-replace-data"
	data := []byte(testData)
	dataLen := len(data)
	_, err := s.az.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data})
	s.assert.Nil(err)
	f, _ := os.CreateTemp("", name+".tmp")
	defer os.Remove(f.Name())
	newTestData := []byte("newdata")
	_, err = s.az.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 5, Data: newTestData})
	s.assert.Nil(err)

	currentData := []byte("test-newdata-data")
	output := make([]byte, len(currentData))

	err = s.az.CopyToFile(internal.CopyToFileOptions{Name: name, File: f})
	s.assert.Nil(err)

	f, _ = os.Open(f.Name())
	len, err := f.Read(output)
	s.assert.Nil(err)
	s.assert.EqualValues(dataLen, len)
	s.assert.EqualValues(currentData, output)
	f.Close()
}

func (s *blockBlobTestSuite) TestOverwriteAndAppendToSmallFile() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h, _ := s.az.CreateFile(internal.CreateFileOptions{Name: name})
	testData := "test-data"
	data := []byte(testData)

	_, err := s.az.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data})
	s.assert.Nil(err)
	f, _ := os.CreateTemp("", name+".tmp")
	defer os.Remove(f.Name())
	newTestData := []byte("newdata")
	_, err = s.az.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 5, Data: newTestData})
	s.assert.Nil(err)

	currentData := []byte("test-newdata")
	dataLen := len(currentData)
	output := make([]byte, dataLen)

	err = s.az.CopyToFile(internal.CopyToFileOptions{Name: name, File: f})
	s.assert.Nil(err)

	f, _ = os.Open(f.Name())
	len, err := f.Read(output)
	s.assert.Nil(err)
	s.assert.EqualValues(dataLen, len)
	s.assert.EqualValues(currentData, output)
	f.Close()
}

func (s *blockBlobTestSuite) TestAppendToSmallFile() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h, _ := s.az.CreateFile(internal.CreateFileOptions{Name: name})
	testData := "test-data"
	data := []byte(testData)

	_, err := s.az.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data})
	s.assert.Nil(err)
	f, _ := os.CreateTemp("", name+".tmp")
	defer os.Remove(f.Name())
	newTestData := []byte("-newdata")
	_, err = s.az.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 9, Data: newTestData})
	s.assert.Nil(err)

	currentData := []byte("test-data-newdata")
	dataLen := len(currentData)
	output := make([]byte, dataLen)

	err = s.az.CopyToFile(internal.CopyToFileOptions{Name: name, File: f})
	s.assert.Nil(err)

	f, _ = os.Open(f.Name())
	len, err := f.Read(output)
	s.assert.Nil(err)
	s.assert.EqualValues(dataLen, len)
	s.assert.EqualValues(currentData, output)
	f.Close()
}

func (s *blockBlobTestSuite) TestAppendOffsetLargerThanSmallFile() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h, _ := s.az.CreateFile(internal.CreateFileOptions{Name: name})
	testData := "test-data"
	data := []byte(testData)

	_, err := s.az.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data})
	s.assert.Nil(err)
	f, _ := os.CreateTemp("", name+".tmp")
	defer os.Remove(f.Name())
	newTestData := []byte("newdata")
	_, err = s.az.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 12, Data: newTestData})
	s.assert.Nil(err)

	currentData := []byte("test-data\x00\x00\x00newdata")
	dataLen := len(currentData)
	output := make([]byte, dataLen)

	err = s.az.CopyToFile(internal.CopyToFileOptions{Name: name, File: f})
	s.assert.Nil(err)

	f, _ = os.Open(f.Name())
	len, err := f.Read(output)
	s.assert.Nil(err)
	s.assert.EqualValues(dataLen, len)
	s.assert.EqualValues(currentData, output)
	f.Close()
}

// This test is a regular blob (without blocks) and we're adding data that will cause it to create blocks
func (s *blockBlobTestSuite) TestAppendBlocksToSmallFile() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h, _ := s.az.CreateFile(internal.CreateFileOptions{Name: name})
	testData := "test-data"
	data := []byte(testData)

	// use our method to make the max upload size (size before a blob is broken down to blocks) to 9 Bytes
	err := uploadReaderAtToBlockBlob(ctx, bytes.NewReader(data), int64(len(data)), 9, s.containerClient.NewBlockBlobClient(name), &blockblob.UploadBufferOptions{
		BlockSize: 8,
	})
	s.assert.Nil(err)
	f, _ := os.CreateTemp("", name+".tmp")
	defer os.Remove(f.Name())
	newTestData := []byte("-newdata-newdata-newdata")
	_, err = s.az.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 9, Data: newTestData})
	s.assert.Nil(err)

	currentData := []byte("test-data-newdata-newdata-newdata")
	dataLen := len(currentData)
	output := make([]byte, dataLen)

	err = s.az.CopyToFile(internal.CopyToFileOptions{Name: name, File: f})
	s.assert.Nil(err)

	f, _ = os.Open(f.Name())
	len, err := f.Read(output)
	s.assert.Nil(err)
	s.assert.EqualValues(dataLen, len)
	s.assert.EqualValues(currentData, output)
	f.Close()
}

func (s *blockBlobTestSuite) TestOverwriteBlocks() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h, _ := s.az.CreateFile(internal.CreateFileOptions{Name: name})
	testData := "testdatates1dat1tes2dat2tes3dat3tes4dat4"
	data := []byte(testData)

	// use our method to make the max upload size (size before a blob is broken down to blocks) to 4 Bytes
	err := uploadReaderAtToBlockBlob(ctx, bytes.NewReader(data), int64(len(data)), 4, s.containerClient.NewBlockBlobClient(name), &blockblob.UploadBufferOptions{
		BlockSize: 4,
	})
	s.assert.Nil(err)
	f, _ := os.CreateTemp("", name+".tmp")
	defer os.Remove(f.Name())
	newTestData := []byte("cake")
	_, err = s.az.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 16, Data: newTestData})
	s.assert.Nil(err)

	currentData := []byte("testdatates1dat1cakedat2tes3dat3tes4dat4")
	dataLen := len(currentData)
	output := make([]byte, dataLen)

	err = s.az.CopyToFile(internal.CopyToFileOptions{Name: name, File: f})
	s.assert.Nil(err)

	f, _ = os.Open(f.Name())
	len, err := f.Read(output)
	s.assert.Nil(err)
	s.assert.EqualValues(dataLen, len)
	s.assert.EqualValues(currentData, output)
	f.Close()
}

func (s *blockBlobTestSuite) TestOverwriteAndAppendBlocks() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h, _ := s.az.CreateFile(internal.CreateFileOptions{Name: name})
	testData := "testdatates1dat1tes2dat2tes3dat3tes4dat4"
	data := []byte(testData)

	// use our method to make the max upload size (size before a blob is broken down to blocks) to 4 Bytes
	err := uploadReaderAtToBlockBlob(ctx, bytes.NewReader(data), int64(len(data)), 4, s.containerClient.NewBlockBlobClient(name), &blockblob.UploadBufferOptions{
		BlockSize: 4,
	})
	s.assert.Nil(err)
	f, _ := os.CreateTemp("", name+".tmp")
	defer os.Remove(f.Name())
	newTestData := []byte("43211234cake")
	_, err = s.az.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 32, Data: newTestData})
	s.assert.Nil(err)

	currentData := []byte("testdatates1dat1tes2dat2tes3dat343211234cake")
	dataLen := len(currentData)
	output := make([]byte, dataLen)

	err = s.az.CopyToFile(internal.CopyToFileOptions{Name: name, File: f})
	s.assert.Nil(err)

	f, _ = os.Open(f.Name())
	len, _ := f.Read(output)
	s.assert.EqualValues(dataLen, len)
	s.assert.EqualValues(currentData, output)
	f.Close()
}

func (s *blockBlobTestSuite) TestAppendBlocks() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h, _ := s.az.CreateFile(internal.CreateFileOptions{Name: name})
	testData := "testdatates1dat1tes2dat2tes3dat3tes4dat4"
	data := []byte(testData)

	// use our method to make the max upload size (size before a blob is broken down to blocks) to 4 Bytes
	err := uploadReaderAtToBlockBlob(ctx, bytes.NewReader(data), int64(len(data)), 4, s.containerClient.NewBlockBlobClient(name), &blockblob.UploadBufferOptions{
		BlockSize: 4,
	})
	s.assert.Nil(err)
	f, _ := os.CreateTemp("", name+".tmp")
	defer os.Remove(f.Name())
	newTestData := []byte("43211234cake")
	_, err = s.az.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: newTestData})
	s.assert.Nil(err)

	currentData := []byte("43211234cakedat1tes2dat2tes3dat3tes4dat4")
	dataLen := len(currentData)
	output := make([]byte, dataLen)

	err = s.az.CopyToFile(internal.CopyToFileOptions{Name: name, File: f})
	s.assert.Nil(err)

	f, _ = os.Open(f.Name())
	len, _ := f.Read(output)
	s.assert.EqualValues(dataLen, len)
	s.assert.EqualValues(currentData, output)
	f.Close()
}

func (s *blockBlobTestSuite) TestAppendOffsetLargerThanSize() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h, _ := s.az.CreateFile(internal.CreateFileOptions{Name: name})
	testData := "testdatates1dat1tes2dat2tes3dat3tes4dat4"
	data := []byte(testData)

	// use our method to make the max upload size (size before a blob is broken down to blocks) to 4 Bytes
	err := uploadReaderAtToBlockBlob(ctx, bytes.NewReader(data), int64(len(data)), 4, s.containerClient.NewBlockBlobClient(name), &blockblob.UploadBufferOptions{
		BlockSize: 4,
	})
	s.assert.Nil(err)
	f, _ := os.CreateTemp("", name+".tmp")
	defer os.Remove(f.Name())
	newTestData := []byte("43211234cake")
	_, err = s.az.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 45, Data: newTestData})
	s.assert.Nil(err)

	currentData := []byte("testdatates1dat1tes2dat2tes3dat3tes4dat4\x00\x00\x00\x00\x0043211234cake")
	dataLen := len(currentData)
	output := make([]byte, dataLen)

	err = s.az.CopyToFile(internal.CopyToFileOptions{Name: name, File: f})
	s.assert.Nil(err)

	f, _ = os.Open(f.Name())
	len, _ := f.Read(output)
	s.assert.EqualValues(dataLen, len)
	s.assert.EqualValues(currentData, output)
	f.Close()
}

func (s *blockBlobTestSuite) TestCopyToFileError() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	f, _ := os.CreateTemp("", name+".tmp")
	defer os.Remove(f.Name())

	err := s.az.CopyToFile(internal.CopyToFileOptions{Name: name, File: f})
	s.assert.NotNil(err)
}

func (s *blockBlobTestSuite) TestCopyFromFile() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	s.az.CreateFile(internal.CreateFileOptions{Name: name})
	testData := "test data"
	data := []byte(testData)
	homeDir, _ := os.UserHomeDir()
	f, _ := os.CreateTemp(homeDir, name+".tmp")
	defer os.Remove(f.Name())
	f.Write(data)

	err := s.az.CopyFromFile(internal.CopyFromFileOptions{Name: name, File: f})

	s.assert.Nil(err)

	// Blob should have updated data
	file := s.containerClient.NewBlobClient(name)
	resp, err := file.DownloadStream(ctx, &blob.DownloadStreamOptions{
		Range: blob.HTTPRange{Offset: 0, Count: int64(len(data))},
	})
	s.assert.Nil(err)
	output, _ := io.ReadAll(resp.Body)
	s.assert.EqualValues(testData, output)
}

func (s *blockBlobTestSuite) TestCreateLink() {
	defer s.cleanupTest()
	// Setup
	target := generateFileName()
	s.az.CreateFile(internal.CreateFileOptions{Name: target})
	name := generateFileName()

	err := s.az.CreateLink(internal.CreateLinkOptions{Name: name, Target: target})
	s.assert.Nil(err)

	// Link should be in the account
	link := s.containerClient.NewBlobClient(name)
	props, err := link.GetProperties(ctx, nil)
	s.assert.Nil(err)
	s.assert.NotNil(props)
	s.assert.NotEmpty(props.Metadata)
	s.assert.True(checkMetadata(props.Metadata, symlinkKey, "true"))
	s.assert.NotNil(props.ContentLength)
	resp, err := link.DownloadStream(ctx, &blob.DownloadStreamOptions{
		Range: blob.HTTPRange{Offset: 0, Count: *props.ContentLength},
	})
	s.assert.Nil(err)
	data, _ := io.ReadAll(resp.Body)
	s.assert.EqualValues(target, data)
}

func (s *blockBlobTestSuite) TestReadLink() {
	defer s.cleanupTest()
	// Setup
	target := generateFileName()
	s.az.CreateFile(internal.CreateFileOptions{Name: target})
	name := generateFileName()
	s.az.CreateLink(internal.CreateLinkOptions{Name: name, Target: target})

	read, err := s.az.ReadLink(internal.ReadLinkOptions{Name: name})
	s.assert.Nil(err)
	s.assert.EqualValues(target, read)
}

func (s *blockBlobTestSuite) TestReadLinkError() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()

	_, err := s.az.ReadLink(internal.ReadLinkOptions{Name: name})
	s.assert.NotNil(err)
	s.assert.EqualValues(syscall.ENOENT, err)
}

func (s *blockBlobTestSuite) TestGetAttrDir() {
	defer s.cleanupTest()
	vdConfig := fmt.Sprintf("azstorage:\n  account-name: %s\n  endpoint: https://%s.blob.core.windows.net/\n  type: block\n  account-key: %s\n  mode: key\n  container: %s\n  fail-unsupported-op: true\n  virtual-directory: true",
		storageTestConfigurationParameters.BlockAccount, storageTestConfigurationParameters.BlockAccount, storageTestConfigurationParameters.BlockKey, s.container)
	configs := []string{"", vdConfig}
	for _, c := range configs {
		// This is a little janky but required since testify suite does not support running setup or clean up for subtests.
		s.tearDownTestHelper(false)
		s.setupTestHelper(c, s.container, true)
		testName := ""
		if c != "" {
			testName = "virtual-directory"
		}
		s.Run(testName, func() {
			// Setup
			name := generateDirectoryName()
			s.az.CreateDir(internal.CreateDirOptions{Name: name})

			props, err := s.az.GetAttr(internal.GetAttrOptions{Name: name})
			s.assert.Nil(err)
			s.assert.NotNil(props)
			s.assert.True(props.IsDir())
			s.assert.NotEmpty(props.Metadata)
			s.assert.True(checkMetadata(props.Metadata, folderKey, "true"))
		})
	}
}

func (s *blockBlobTestSuite) TestGetAttrVirtualDir() {
	defer s.cleanupTest()
	vdConfig := fmt.Sprintf("azstorage:\n  account-name: %s\n  endpoint: https://%s.blob.core.windows.net/\n  type: block\n  account-key: %s\n  mode: key\n  container: %s\n  fail-unsupported-op: true\n  virtual-directory: true",
		storageTestConfigurationParameters.BlockAccount, storageTestConfigurationParameters.BlockAccount, storageTestConfigurationParameters.BlockKey, s.container)
	// This is a little janky but required since testify suite does not support running setup or clean up for subtests.
	s.tearDownTestHelper(false)
	s.setupTestHelper(vdConfig, s.container, true)
	// Setup
	dirName := generateFileName()
	name := dirName + "/" + generateFileName()
	s.az.CreateFile(internal.CreateFileOptions{Name: name})

	props, err := s.az.GetAttr(internal.GetAttrOptions{Name: dirName})
	s.assert.Nil(err)
	s.assert.NotNil(props)
	s.assert.True(props.IsDir())
	s.assert.False(props.IsSymlink())

	// Check file in dir too
	props, err = s.az.GetAttr(internal.GetAttrOptions{Name: name})
	s.assert.Nil(err)
	s.assert.NotNil(props)
	s.assert.False(props.IsDir())
	s.assert.False(props.IsSymlink())
}

func (s *blockBlobTestSuite) TestGetAttrVirtualDirSubDir() {
	defer s.cleanupTest()
	vdConfig := fmt.Sprintf("azstorage:\n  account-name: %s\n  endpoint: https://%s.blob.core.windows.net/\n  type: block\n  account-key: %s\n  mode: key\n  container: %s\n  fail-unsupported-op: true\n  virtual-directory: true",
		storageTestConfigurationParameters.BlockAccount, storageTestConfigurationParameters.BlockAccount, storageTestConfigurationParameters.BlockKey, s.container)
	// This is a little janky but required since testify suite does not support running setup or clean up for subtests.
	s.tearDownTestHelper(false)
	s.setupTestHelper(vdConfig, s.container, true)
	// Setup
	dirName := generateFileName()
	subDirName := dirName + "/" + generateFileName()
	name := subDirName + "/" + generateFileName()
	s.az.CreateFile(internal.CreateFileOptions{Name: name})

	props, err := s.az.GetAttr(internal.GetAttrOptions{Name: dirName})
	s.assert.Nil(err)
	s.assert.NotNil(props)
	s.assert.True(props.IsDir())
	s.assert.False(props.IsSymlink())

	// Check subdir in dir too
	props, err = s.az.GetAttr(internal.GetAttrOptions{Name: subDirName})
	s.assert.Nil(err)
	s.assert.NotNil(props)
	s.assert.True(props.IsDir())
	s.assert.False(props.IsSymlink())

	// Check file in subdir too
	props, err = s.az.GetAttr(internal.GetAttrOptions{Name: name})
	s.assert.Nil(err)
	s.assert.NotNil(props)
	s.assert.False(props.IsDir())
	s.assert.False(props.IsSymlink())
}

func (s *blockBlobTestSuite) TestGetAttrDirWithCPKEnabled() {
	defer s.cleanupTest()
	CPKEncryptionKey, CPKEncryptionKeySHA256 := generateCPKInfo()
	config := fmt.Sprintf("azstorage:\n  account-name: %s\n  endpoint: https://%s.blob.core.windows.net/\n  type: block\n  cpk-enabled: true\n  cpk-encryption-key: %s\n  cpk-encryption-key-sha256: %s\n  account-key: %s\n  mode: key\n  container: %s\n",
		storageTestConfigurationParameters.BlockAccount, storageTestConfigurationParameters.BlockAccount, CPKEncryptionKey, CPKEncryptionKeySHA256, storageTestConfigurationParameters.BlockKey, s.container)

	s.tearDownTestHelper(false)
	s.setupTestHelper(config, s.container, false)

	name := generateDirectoryName()
	s.az.CreateDir(internal.CreateDirOptions{Name: name})

	props, err := s.az.GetAttr(internal.GetAttrOptions{Name: name})
	s.assert.Nil(err)
	s.assert.NotNil(props)
	s.assert.True(props.IsDir())
	s.assert.NotEmpty(props.Metadata)
	s.assert.True(checkMetadata(props.Metadata, folderKey, "true"))
}

func (s *blockBlobTestSuite) TestGetAttrFile() {
	defer s.cleanupTest()
	vdConfig := fmt.Sprintf("azstorage:\n  account-name: %s\n  endpoint: https://%s.blob.core.windows.net/\n  type: block\n  account-key: %s\n  mode: key\n  container: %s\n  fail-unsupported-op: true\n  virtual-directory: true",
		storageTestConfigurationParameters.BlockAccount, storageTestConfigurationParameters.BlockAccount, storageTestConfigurationParameters.BlockKey, s.container)
	configs := []string{"", vdConfig}
	for _, c := range configs {
		// This is a little janky but required since testify suite does not support running setup or clean up for subtests.
		s.tearDownTestHelper(false)
		s.setupTestHelper(c, s.container, true)
		testName := ""
		if c != "" {
			testName = "virtual-directory"
		}
		s.Run(testName, func() {
			// Setup
			name := generateFileName()
			s.az.CreateFile(internal.CreateFileOptions{Name: name})

			props, err := s.az.GetAttr(internal.GetAttrOptions{Name: name})
			s.assert.Nil(err)
			s.assert.NotNil(props)
			s.assert.False(props.IsDir())
			s.assert.False(props.IsSymlink())
		})
	}
}

func (s *blockBlobTestSuite) TestGetAttrLink() {
	defer s.cleanupTest()
	vdConfig := fmt.Sprintf("azstorage:\n  account-name: %s\n  endpoint: https://%s.blob.core.windows.net/\n  type: block\n  account-key: %s\n  mode: key\n  container: %s\n  fail-unsupported-op: true\n  virtual-directory: true",
		storageTestConfigurationParameters.BlockAccount, storageTestConfigurationParameters.BlockAccount, storageTestConfigurationParameters.BlockKey, s.container)
	configs := []string{"", vdConfig}
	for _, c := range configs {
		// This is a little janky but required since testify suite does not support running setup or clean up for subtests.
		s.tearDownTestHelper(false)
		s.setupTestHelper(c, s.container, true)
		testName := ""
		if c != "" {
			testName = "virtual-directory"
		}
		s.Run(testName, func() {
			// Setup
			target := generateFileName()
			s.az.CreateFile(internal.CreateFileOptions{Name: target})
			name := generateFileName()
			s.az.CreateLink(internal.CreateLinkOptions{Name: name, Target: target})

			props, err := s.az.GetAttr(internal.GetAttrOptions{Name: name})
			s.assert.Nil(err)
			s.assert.NotNil(props)
			s.assert.True(props.IsSymlink())
			s.assert.NotEmpty(props.Metadata)
			s.assert.True(checkMetadata(props.Metadata, symlinkKey, "true"))
		})
	}
}

func (s *blockBlobTestSuite) TestGetAttrFileSize() {
	defer s.cleanupTest()
	vdConfig := fmt.Sprintf("azstorage:\n  account-name: %s\n  endpoint: https://%s.blob.core.windows.net/\n  type: block\n  account-key: %s\n  mode: key\n  container: %s\n  fail-unsupported-op: true\n  virtual-directory: true",
		storageTestConfigurationParameters.BlockAccount, storageTestConfigurationParameters.BlockAccount, storageTestConfigurationParameters.BlockKey, s.container)
	configs := []string{"", vdConfig}
	for _, c := range configs {
		// This is a little janky but required since testify suite does not support running setup or clean up for subtests.
		s.tearDownTestHelper(false)
		s.setupTestHelper(c, s.container, true)
		testName := ""
		if c != "" {
			testName = "virtual-directory"
		}
		s.Run(testName, func() {
			// Setup
			name := generateFileName()
			h, _ := s.az.CreateFile(internal.CreateFileOptions{Name: name})
			testData := "test data"
			data := []byte(testData)
			s.az.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data})

			props, err := s.az.GetAttr(internal.GetAttrOptions{Name: name})
			s.assert.Nil(err)
			s.assert.NotNil(props)
			s.assert.False(props.IsDir())
			s.assert.False(props.IsSymlink())
			s.assert.EqualValues(len(testData), props.Size)
		})
	}
}

func (s *blockBlobTestSuite) TestGetAttrFileTime() {
	defer s.cleanupTest()
	vdConfig := fmt.Sprintf("azstorage:\n  account-name: %s\n  endpoint: https://%s.blob.core.windows.net/\n  type: block\n  account-key: %s\n  mode: key\n  container: %s\n  fail-unsupported-op: true\n  virtual-directory: true",
		storageTestConfigurationParameters.BlockAccount, storageTestConfigurationParameters.BlockAccount, storageTestConfigurationParameters.BlockKey, s.container)
	configs := []string{"", vdConfig}
	for _, c := range configs {
		// This is a little janky but required since testify suite does not support running setup or clean up for subtests.
		s.tearDownTestHelper(false)
		s.setupTestHelper(c, s.container, true)
		testName := ""
		if c != "" {
			testName = "virtual-directory"
		}
		s.Run(testName, func() {
			// Setup
			name := generateFileName()
			h, _ := s.az.CreateFile(internal.CreateFileOptions{Name: name})
			testData := "test data"
			data := []byte(testData)
			s.az.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data})

			before, err := s.az.GetAttr(internal.GetAttrOptions{Name: name})
			s.assert.Nil(err)
			s.assert.NotNil(before.Mtime)

			time.Sleep(time.Second * 3) // Wait 3 seconds and then modify the file again

			s.az.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data})

			after, err := s.az.GetAttr(internal.GetAttrOptions{Name: name})
			s.assert.Nil(err)
			s.assert.NotNil(after.Mtime)

			s.assert.True(after.Mtime.After(before.Mtime))
		})
	}
}

func (s *blockBlobTestSuite) TestGetAttrError() {
	defer s.cleanupTest()
	vdConfig := fmt.Sprintf("azstorage:\n  account-name: %s\n  endpoint: https://%s.blob.core.windows.net/\n  type: block\n  account-key: %s\n  mode: key\n  container: %s\n  fail-unsupported-op: true\n  virtual-directory: true",
		storageTestConfigurationParameters.BlockAccount, storageTestConfigurationParameters.BlockAccount, storageTestConfigurationParameters.BlockKey, s.container)
	configs := []string{"", vdConfig}
	for _, c := range configs {
		// This is a little janky but required since testify suite does not support running setup or clean up for subtests.
		s.tearDownTestHelper(false)
		s.setupTestHelper(c, s.container, true)
		testName := ""
		if c != "" {
			testName = "virtual-directory"
		}
		s.Run(testName, func() {
			// Setup
			name := generateFileName()

			_, err := s.az.GetAttr(internal.GetAttrOptions{Name: name})
			s.assert.NotNil(err)
			s.assert.EqualValues(syscall.ENOENT, err)
		})
	}
}

// If support for chown or chmod are ever added to blob, add tests for error cases and modify the following tests.
func (s *blockBlobTestSuite) TestChmod() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	s.az.CreateFile(internal.CreateFileOptions{Name: name})

	err := s.az.Chmod(internal.ChmodOptions{Name: name, Mode: 0666})
	s.assert.NotNil(err)
	s.assert.EqualValues(syscall.ENOTSUP, err)
}

func (s *blockBlobTestSuite) TestChmodIgnore() {
	defer s.cleanupTest()
	// Setup
	s.tearDownTestHelper(false) // Don't delete the generated container.

	config := fmt.Sprintf("azstorage:\n  account-name: %s\n  endpoint: https://%s.blob.core.windows.net/\n  type: block\n  account-key: %s\n  mode: key\n  container: %s\n  fail-unsupported-op: false\n",
		storageTestConfigurationParameters.BlockAccount, storageTestConfigurationParameters.BlockAccount, storageTestConfigurationParameters.BlockKey, s.container)
	s.setupTestHelper(config, s.container, true)
	name := generateFileName()
	s.az.CreateFile(internal.CreateFileOptions{Name: name})

	err := s.az.Chmod(internal.ChmodOptions{Name: name, Mode: 0666})
	s.assert.Nil(err)
}

func (s *blockBlobTestSuite) TestChown() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	s.az.CreateFile(internal.CreateFileOptions{Name: name})

	err := s.az.Chown(internal.ChownOptions{Name: name, Owner: 6, Group: 5})
	s.assert.NotNil(err)
	s.assert.EqualValues(syscall.ENOTSUP, err)
}

func (s *blockBlobTestSuite) TestChownIgnore() {
	defer s.cleanupTest()
	// Setup
	s.tearDownTestHelper(false) // Don't delete the generated container.

	config := fmt.Sprintf("azstorage:\n  account-name: %s\n  endpoint: https://%s.blob.core.windows.net/\n  type: block\n  account-key: %s\n  mode: key\n  container: %s\n  fail-unsupported-op: false\n",
		storageTestConfigurationParameters.BlockAccount, storageTestConfigurationParameters.BlockAccount, storageTestConfigurationParameters.BlockKey, s.container)
	s.setupTestHelper(config, s.container, true)
	name := generateFileName()
	s.az.CreateFile(internal.CreateFileOptions{Name: name})

	err := s.az.Chown(internal.ChownOptions{Name: name, Owner: 6, Group: 5})
	s.assert.Nil(err)
}

func (s *blockBlobTestSuite) TestBlockSize() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()

	bb := BlockBlob{}

	// For filesize 0 expected blocksize is 256MB
	block, err := bb.calculateBlockSize(name, 0)
	s.assert.Nil(err)
	s.assert.EqualValues(block, blockblob.MaxUploadBlobBytes)

	// For filesize 100MB expected blocksize is 256MB
	block, err = bb.calculateBlockSize(name, (100 * 1024 * 1024))
	s.assert.Nil(err)
	s.assert.EqualValues(block, blockblob.MaxUploadBlobBytes)

	// For filesize 500MB expected blocksize is 4MB
	block, err = bb.calculateBlockSize(name, (500 * 1024 * 1024))
	s.assert.Nil(err)
	s.assert.EqualValues(block, blob.DefaultDownloadBlockSize)

	// For filesize 1GB expected blocksize is 4MB
	block, err = bb.calculateBlockSize(name, (1 * 1024 * 1024 * 1024))
	s.assert.Nil(err)
	s.assert.EqualValues(block, blob.DefaultDownloadBlockSize)

	// For filesize 500GB expected blocksize is 10737424
	block, err = bb.calculateBlockSize(name, (500 * 1024 * 1024 * 1024))
	s.assert.Nil(err)
	s.assert.EqualValues(block, int64(10737424))

	// For filesize 1TB expected blocksize is 21990240  (1TB/50000 ~= rounded off to next multiple of 8)
	block, err = bb.calculateBlockSize(name, (1 * 1024 * 1024 * 1024 * 1024))
	s.assert.Nil(err)
	s.assert.EqualValues(block, int64(21990240))

	// For filesize 100TB expected blocksize is 2199023256  (100TB/50000 ~= rounded off to next multiple of 8)
	block, err = bb.calculateBlockSize(name, (100 * 1024 * 1024 * 1024 * 1024))
	s.assert.Nil(err)
	s.assert.EqualValues(block, int64(2199023256))

	// For filesize 190TB expected blocksize is 4178144192  (190TB/50000 ~= rounded off to next multiple of 8)
	block, err = bb.calculateBlockSize(name, (190 * 1024 * 1024 * 1024 * 1024))
	s.assert.Nil(err)
	s.assert.EqualValues(block, int64(4178144192))

	// Boundary condition which is exactly max size supported by sdk
	block, err = bb.calculateBlockSize(name, (blockblob.MaxStageBlockBytes * blockblob.MaxBlocks))
	s.assert.Nil(err)
	s.assert.EqualValues(block, int64(blockblob.MaxStageBlockBytes)) // 4194304000

	// For Filesize created using dd for 1TB size
	block, err = bb.calculateBlockSize(name, int64(1099511627776))
	s.assert.Nil(err)
	s.assert.EqualValues(block, int64(21990240))

	// Boundary condition 5 bytes less then max expected file size
	block, err = bb.calculateBlockSize(name, (blockblob.MaxStageBlockBytes*blockblob.MaxBlocks)-5)
	s.assert.Nil(err)
	s.assert.EqualValues(block, int64(blockblob.MaxStageBlockBytes))

	// Boundary condition 1 bytes more then max expected file size
	block, err = bb.calculateBlockSize(name, (blockblob.MaxStageBlockBytes*blockblob.MaxBlocks)+1)
	s.assert.NotNil(err)
	s.assert.EqualValues(block, 0)

	// Boundary condition 5 bytes more then max expected file size
	block, err = bb.calculateBlockSize(name, (blockblob.MaxStageBlockBytes*blockblob.MaxBlocks)+5)
	s.assert.NotNil(err)
	s.assert.EqualValues(block, 0)

	// Boundary condition file size one block short of file blocks
	block, err = bb.calculateBlockSize(name, (blockblob.MaxStageBlockBytes*blockblob.MaxBlocks)-blockblob.MaxStageBlockBytes)
	s.assert.Nil(err)
	s.assert.EqualValues(block, 4194220120)

	// Boundary condition one byte more then max block size
	block, err = bb.calculateBlockSize(name, (4194304001 * blockblob.MaxBlocks))
	s.assert.NotNil(err)
	s.assert.EqualValues(block, 0)

	// For filesize 200TB, error is expected as max 190TB only supported
	block, err = bb.calculateBlockSize(name, (200 * 1024 * 1024 * 1024 * 1024))
	s.assert.NotNil(err)
	s.assert.EqualValues(block, 0)
}

func (s *blockBlobTestSuite) TestGetFileBlockOffsetsSmallFile() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h, _ := s.az.CreateFile(internal.CreateFileOptions{Name: name})
	testData := "testdatates1dat1tes2dat2tes3dat3tes4dat4"
	data := []byte(testData)

	s.az.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data})

	// GetFileBlockOffsets
	offsetList, err := s.az.GetFileBlockOffsets(internal.GetFileBlockOffsetsOptions{Name: name})
	s.assert.Nil(err)
	s.assert.Len(offsetList.BlockList, 0)
	s.assert.True(offsetList.SmallFile())
	s.assert.EqualValues(0, offsetList.BlockIdLength)
}

func (s *blockBlobTestSuite) TestGetFileBlockOffsetsChunkedFile() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	s.az.CreateFile(internal.CreateFileOptions{Name: name})
	testData := "testdatates1dat1tes2dat2tes3dat3tes4dat4"
	data := []byte(testData)

	// use our method to make the max upload size (size before a blob is broken down to blocks) to 4 Bytes
	err := uploadReaderAtToBlockBlob(ctx, bytes.NewReader(data), int64(len(data)), 4, s.containerClient.NewBlockBlobClient(name), &blockblob.UploadBufferOptions{
		BlockSize: 4,
	})
	s.assert.Nil(err)

	// GetFileBlockOffsets
	offsetList, err := s.az.GetFileBlockOffsets(internal.GetFileBlockOffsetsOptions{Name: name})
	s.assert.Nil(err)
	s.assert.Len(offsetList.BlockList, 10)
	s.assert.Zero(offsetList.Flags)
	s.assert.EqualValues(16, offsetList.BlockIdLength)
}

func (s *blockBlobTestSuite) TestGetFileBlockOffsetsError() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()

	// GetFileBlockOffsets
	_, err := s.az.GetFileBlockOffsets(internal.GetFileBlockOffsetsOptions{Name: name})
	s.assert.NotNil(err)
}

func (s *blockBlobTestSuite) TestFlushFileEmptyFile() {
	defer s.cleanupTest()

	// Setup
	name := generateFileName()
	h, _ := s.az.CreateFile(internal.CreateFileOptions{Name: name})

	bol, _ := s.az.GetFileBlockOffsets(internal.GetFileBlockOffsetsOptions{Name: name})
	handlemap.CreateCacheObject(int64(16*MB), h)
	h.CacheObj.BlockOffsetList = bol

	err := s.az.FlushFile(internal.FlushFileOptions{Handle: h})
	s.assert.Nil(err)

	output, err := s.az.ReadFile(internal.ReadFileOptions{Handle: h})
	s.assert.Nil(err)
	s.assert.EqualValues("", output)
}

func (s *blockBlobTestSuite) TestFlushFileChunkedFile() {
	defer s.cleanupTest()

	// Setup
	name := generateFileName()
	h, _ := s.az.CreateFile(internal.CreateFileOptions{Name: name})
	data := make([]byte, 16*MB)
	rand.Read(data)

	// use our method to make the max upload size (size before a blob is broken down to blocks) to 4 Bytes
	err := uploadReaderAtToBlockBlob(ctx, bytes.NewReader(data), int64(len(data)), 4, s.containerClient.NewBlockBlobClient(name), &blockblob.UploadBufferOptions{
		BlockSize: 4 * MB,
	})
	s.assert.Nil(err)
	bol, _ := s.az.GetFileBlockOffsets(internal.GetFileBlockOffsetsOptions{Name: name})
	handlemap.CreateCacheObject(int64(16*MB), h)
	h.CacheObj.BlockOffsetList = bol

	err = s.az.FlushFile(internal.FlushFileOptions{Handle: h})
	s.assert.Nil(err)

	output, err := s.az.ReadFile(internal.ReadFileOptions{Handle: h})
	s.assert.Nil(err)
	s.assert.EqualValues(data, output)
}

func (s *blockBlobTestSuite) TestFlushFileUpdateChunkedFile() {
	defer s.cleanupTest()

	// Setup
	name := generateFileName()
	blockSize := 4 * MB
	h, _ := s.az.CreateFile(internal.CreateFileOptions{Name: name})
	data := make([]byte, 16*MB)
	rand.Read(data)

	// use our method to make the max upload size (size before a blob is broken down to blocks) to 4 Bytes
	err := uploadReaderAtToBlockBlob(ctx, bytes.NewReader(data), int64(len(data)), 4, s.containerClient.NewBlockBlobClient(name), &blockblob.UploadBufferOptions{
		BlockSize: int64(blockSize),
	})
	s.assert.Nil(err)
	bol, _ := s.az.GetFileBlockOffsets(internal.GetFileBlockOffsetsOptions{Name: name})
	handlemap.CreateCacheObject(int64(16*MB), h)
	h.CacheObj.BlockOffsetList = bol

	updatedBlock := make([]byte, 2*MB)
	rand.Read(updatedBlock)
	h.CacheObj.BlockOffsetList.BlockList[1].Data = make([]byte, blockSize)
	s.az.storage.ReadInBuffer(name, int64(blockSize), int64(blockSize), h.CacheObj.BlockOffsetList.BlockList[1].Data, nil)
	copy(h.CacheObj.BlockOffsetList.BlockList[1].Data[MB:2*MB+MB], updatedBlock)
	h.CacheObj.BlockOffsetList.BlockList[1].Flags.Set(common.DirtyBlock)

	err = s.az.FlushFile(internal.FlushFileOptions{Handle: h})
	s.assert.Nil(err)

	output, err := s.az.ReadFile(internal.ReadFileOptions{Handle: h})
	s.assert.Nil(err)
	s.assert.NotEqualValues(data, output)
	s.assert.EqualValues(data[:5*MB], output[:5*MB])
	s.assert.EqualValues(updatedBlock, output[5*MB:5*MB+2*MB])
	s.assert.EqualValues(data[7*MB:], output[7*MB:])
}

func (s *blockBlobTestSuite) TestFlushFileTruncateUpdateChunkedFile() {
	defer s.cleanupTest()

	// Setup
	name := generateFileName()
	blockSize := 4 * MB
	h, _ := s.az.CreateFile(internal.CreateFileOptions{Name: name})
	data := make([]byte, 16*MB)
	rand.Read(data)

	// use our method to make the max upload size (size before a blob is broken down to blocks) to 4 Bytes
	err := uploadReaderAtToBlockBlob(ctx, bytes.NewReader(data), int64(len(data)), 4, s.containerClient.NewBlockBlobClient(name), &blockblob.UploadBufferOptions{
		BlockSize: int64(blockSize),
	})
	s.assert.Nil(err)
	bol, _ := s.az.GetFileBlockOffsets(internal.GetFileBlockOffsetsOptions{Name: name})
	handlemap.CreateCacheObject(int64(16*MB), h)
	h.CacheObj.BlockOffsetList = bol

	// truncate block
	h.CacheObj.BlockOffsetList.BlockList[1].Data = make([]byte, blockSize/2)
	h.CacheObj.BlockOffsetList.BlockList[1].EndIndex = int64(blockSize + blockSize/2)
	s.az.storage.ReadInBuffer(name, int64(blockSize), int64(blockSize)/2, h.CacheObj.BlockOffsetList.BlockList[1].Data, nil)
	h.CacheObj.BlockOffsetList.BlockList[1].Flags.Set(common.DirtyBlock)

	// remove 2 blocks
	h.CacheObj.BlockOffsetList.BlockList = h.CacheObj.BlockOffsetList.BlockList[:2]

	err = s.az.FlushFile(internal.FlushFileOptions{Handle: h})
	s.assert.Nil(err)

	output, err := s.az.ReadFile(internal.ReadFileOptions{Handle: h})
	s.assert.Nil(err)
	s.assert.NotEqualValues(data, output)
	s.assert.EqualValues(data[:6*MB], output[:6*MB])
}

func (s *blockBlobTestSuite) TestFlushFileAppendBlocksEmptyFile() {
	defer s.cleanupTest()

	// Setup
	name := generateFileName()
	blockSize := 2 * MB
	h, _ := s.az.CreateFile(internal.CreateFileOptions{Name: name})

	bol, _ := s.az.GetFileBlockOffsets(internal.GetFileBlockOffsetsOptions{Name: name})
	handlemap.CreateCacheObject(int64(12*MB), h)
	h.CacheObj.BlockOffsetList = bol
	h.CacheObj.BlockIdLength = 16

	data1 := make([]byte, blockSize)
	rand.Read(data1)
	blk1 := &common.Block{
		StartIndex: 0,
		EndIndex:   int64(blockSize),
		Id:         base64.StdEncoding.EncodeToString(common.NewUUIDWithLength(h.CacheObj.BlockIdLength)),
		Data:       data1,
	}
	blk1.Flags.Set(common.DirtyBlock)

	data2 := make([]byte, blockSize)
	rand.Read(data2)
	blk2 := &common.Block{
		StartIndex: int64(blockSize),
		EndIndex:   2 * int64(blockSize),
		Id:         base64.StdEncoding.EncodeToString(common.NewUUIDWithLength(h.CacheObj.BlockIdLength)),
		Data:       data2,
	}
	blk2.Flags.Set(common.DirtyBlock)

	data3 := make([]byte, blockSize)
	rand.Read(data3)
	blk3 := &common.Block{
		StartIndex: 2 * int64(blockSize),
		EndIndex:   3 * int64(blockSize),
		Id:         base64.StdEncoding.EncodeToString(common.NewUUIDWithLength(h.CacheObj.BlockIdLength)),
		Data:       data3,
	}
	blk3.Flags.Set(common.DirtyBlock)
	h.CacheObj.BlockOffsetList.BlockList = append(h.CacheObj.BlockOffsetList.BlockList, blk1, blk2, blk3)
	bol.Flags.Clear(common.SmallFile)

	err := s.az.FlushFile(internal.FlushFileOptions{Handle: h})
	s.assert.Nil(err)

	output, err := s.az.ReadFile(internal.ReadFileOptions{Handle: h})
	s.assert.Nil(err)
	s.assert.EqualValues(blk1.Data, output[0:blockSize])
	s.assert.EqualValues(blk2.Data, output[blockSize:2*blockSize])
	s.assert.EqualValues(blk3.Data, output[2*blockSize:3*blockSize])
}

func (s *blockBlobTestSuite) TestFlushFileAppendBlocksChunkedFile() {
	defer s.cleanupTest()

	// Setup
	name := generateFileName()
	blockSize := 2 * MB
	fileSize := 16 * MB
	h, _ := s.az.CreateFile(internal.CreateFileOptions{Name: name})
	data := make([]byte, fileSize)
	rand.Read(data)

	// use our method to make the max upload size (size before a blob is broken down to blocks) to 4 Bytes
	err := uploadReaderAtToBlockBlob(ctx, bytes.NewReader(data), int64(len(data)), 4, s.containerClient.NewBlockBlobClient(name), &blockblob.UploadBufferOptions{
		BlockSize: int64(blockSize),
	})
	s.assert.Nil(err)
	bol, _ := s.az.GetFileBlockOffsets(internal.GetFileBlockOffsetsOptions{Name: name})
	handlemap.CreateCacheObject(int64(16*MB), h)
	h.CacheObj.BlockOffsetList = bol
	h.CacheObj.BlockIdLength = 16

	data1 := make([]byte, blockSize)
	rand.Read(data1)
	blk1 := &common.Block{
		StartIndex: int64(fileSize),
		EndIndex:   int64(fileSize + blockSize),
		Id:         base64.StdEncoding.EncodeToString(common.NewUUIDWithLength(h.CacheObj.BlockIdLength)),
		Data:       data1,
	}
	blk1.Flags.Set(common.DirtyBlock)

	data2 := make([]byte, blockSize)
	rand.Read(data2)
	blk2 := &common.Block{
		StartIndex: int64(fileSize + blockSize),
		EndIndex:   int64(fileSize + 2*blockSize),
		Id:         base64.StdEncoding.EncodeToString(common.NewUUIDWithLength(h.CacheObj.BlockIdLength)),
		Data:       data2,
	}
	blk2.Flags.Set(common.DirtyBlock)

	data3 := make([]byte, blockSize)
	rand.Read(data3)
	blk3 := &common.Block{
		StartIndex: int64(fileSize + 2*blockSize),
		EndIndex:   int64(fileSize + 3*blockSize),
		Id:         base64.StdEncoding.EncodeToString(common.NewUUIDWithLength(h.CacheObj.BlockIdLength)),
		Data:       data3,
	}
	blk3.Flags.Set(common.DirtyBlock)
	h.CacheObj.BlockOffsetList.BlockList = append(h.CacheObj.BlockOffsetList.BlockList, blk1, blk2, blk3)
	bol.Flags.Clear(common.SmallFile)

	err = s.az.FlushFile(internal.FlushFileOptions{Handle: h})
	s.assert.Nil(err)

	output, err := s.az.ReadFile(internal.ReadFileOptions{Handle: h})
	s.assert.Nil(err)
	s.assert.EqualValues(data, output[0:fileSize])
	s.assert.EqualValues(blk1.Data, output[fileSize:fileSize+blockSize])
	s.assert.EqualValues(blk2.Data, output[fileSize+blockSize:fileSize+2*blockSize])
	s.assert.EqualValues(blk3.Data, output[fileSize+2*blockSize:fileSize+3*blockSize])
}

func (s *blockBlobTestSuite) TestFlushFileTruncateBlocksEmptyFile() {
	defer s.cleanupTest()

	// Setup
	name := generateFileName()
	blockSize := 4 * MB
	h, _ := s.az.CreateFile(internal.CreateFileOptions{Name: name})

	bol, _ := s.az.GetFileBlockOffsets(internal.GetFileBlockOffsetsOptions{Name: name})
	handlemap.CreateCacheObject(int64(12*MB), h)
	h.CacheObj.BlockOffsetList = bol
	h.CacheObj.BlockIdLength = 16

	blk1 := &common.Block{
		StartIndex: 0,
		EndIndex:   int64(blockSize),
		Id:         base64.StdEncoding.EncodeToString(common.NewUUIDWithLength(h.CacheObj.BlockIdLength)),
	}
	blk1.Flags.Set(common.TruncatedBlock)
	blk1.Flags.Set(common.DirtyBlock)

	blk2 := &common.Block{
		StartIndex: int64(blockSize),
		EndIndex:   2 * int64(blockSize),
		Id:         base64.StdEncoding.EncodeToString(common.NewUUIDWithLength(h.CacheObj.BlockIdLength)),
	}
	blk2.Flags.Set(common.TruncatedBlock)
	blk2.Flags.Set(common.DirtyBlock)

	blk3 := &common.Block{
		StartIndex: 2 * int64(blockSize),
		EndIndex:   3 * int64(blockSize),
		Id:         base64.StdEncoding.EncodeToString(common.NewUUIDWithLength(h.CacheObj.BlockIdLength)),
	}
	blk3.Flags.Set(common.TruncatedBlock)
	blk3.Flags.Set(common.DirtyBlock)
	h.CacheObj.BlockOffsetList.BlockList = append(h.CacheObj.BlockOffsetList.BlockList, blk1, blk2, blk3)
	bol.Flags.Clear(common.SmallFile)

	err := s.az.FlushFile(internal.FlushFileOptions{Handle: h})
	s.assert.Nil(err)

	output, err := s.az.ReadFile(internal.ReadFileOptions{Handle: h})
	s.assert.Nil(err)
	data := make([]byte, 3*blockSize)
	s.assert.EqualValues(data, output)
}

func (s *blockBlobTestSuite) TestFlushFileTruncateBlocksChunkedFile() {
	defer s.cleanupTest()

	// Setup
	name := generateFileName()
	blockSize := 4 * MB
	fileSize := 16 * MB
	h, _ := s.az.CreateFile(internal.CreateFileOptions{Name: name})
	data := make([]byte, fileSize)
	rand.Read(data)

	// use our method to make the max upload size (size before a blob is broken down to blocks) to 4 Bytes
	err := uploadReaderAtToBlockBlob(ctx, bytes.NewReader(data), int64(len(data)), 4, s.containerClient.NewBlockBlobClient(name), &blockblob.UploadBufferOptions{
		BlockSize: int64(blockSize),
	})
	s.assert.Nil(err)
	bol, _ := s.az.GetFileBlockOffsets(internal.GetFileBlockOffsetsOptions{Name: name})
	handlemap.CreateCacheObject(int64(16*MB), h)
	h.CacheObj.BlockOffsetList = bol
	h.CacheObj.BlockIdLength = 16

	blk1 := &common.Block{
		StartIndex: int64(fileSize),
		EndIndex:   int64(fileSize + blockSize),
		Id:         base64.StdEncoding.EncodeToString(common.NewUUIDWithLength(h.CacheObj.BlockIdLength)),
	}
	blk1.Flags.Set(common.TruncatedBlock)
	blk1.Flags.Set(common.DirtyBlock)

	blk2 := &common.Block{
		StartIndex: int64(fileSize + blockSize),
		EndIndex:   int64(fileSize + 2*blockSize),
		Id:         base64.StdEncoding.EncodeToString(common.NewUUIDWithLength(h.CacheObj.BlockIdLength)),
	}
	blk2.Flags.Set(common.TruncatedBlock)
	blk2.Flags.Set(common.DirtyBlock)

	blk3 := &common.Block{
		StartIndex: int64(fileSize + 2*blockSize),
		EndIndex:   int64(fileSize + 3*blockSize),
		Id:         base64.StdEncoding.EncodeToString(common.NewUUIDWithLength(h.CacheObj.BlockIdLength)),
	}
	blk3.Flags.Set(common.TruncatedBlock)
	blk3.Flags.Set(common.DirtyBlock)
	h.CacheObj.BlockOffsetList.BlockList = append(h.CacheObj.BlockOffsetList.BlockList, blk1, blk2, blk3)
	bol.Flags.Clear(common.SmallFile)

	err = s.az.FlushFile(internal.FlushFileOptions{Handle: h})
	s.assert.Nil(err)

	output, err := s.az.ReadFile(internal.ReadFileOptions{Handle: h})
	s.assert.Nil(err)
	s.assert.EqualValues(data, output[:fileSize])
	emptyData := make([]byte, 3*blockSize)
	s.assert.EqualValues(emptyData, output[fileSize:])
}

func (s *blockBlobTestSuite) TestFlushFileAppendAndTruncateBlocksEmptyFile() {
	defer s.cleanupTest()

	// Setup
	name := generateFileName()
	blockSize := 7 * MB
	h, _ := s.az.CreateFile(internal.CreateFileOptions{Name: name})

	bol, _ := s.az.GetFileBlockOffsets(internal.GetFileBlockOffsetsOptions{Name: name})
	handlemap.CreateCacheObject(int64(12*MB), h)
	h.CacheObj.BlockOffsetList = bol
	h.CacheObj.BlockIdLength = 16

	data1 := make([]byte, blockSize)
	rand.Read(data1)
	blk1 := &common.Block{
		StartIndex: 0,
		EndIndex:   int64(blockSize),
		Id:         base64.StdEncoding.EncodeToString(common.NewUUIDWithLength(h.CacheObj.BlockIdLength)),
		Data:       data1,
	}
	blk1.Flags.Set(common.DirtyBlock)

	blk2 := &common.Block{
		StartIndex: int64(blockSize),
		EndIndex:   2 * int64(blockSize),
		Id:         base64.StdEncoding.EncodeToString(common.NewUUIDWithLength(h.CacheObj.BlockIdLength)),
	}
	blk2.Flags.Set(common.DirtyBlock)
	blk2.Flags.Set(common.TruncatedBlock)

	blk3 := &common.Block{
		StartIndex: 2 * int64(blockSize),
		EndIndex:   3 * int64(blockSize),
		Id:         base64.StdEncoding.EncodeToString(common.NewUUIDWithLength(h.CacheObj.BlockIdLength)),
	}
	blk3.Flags.Set(common.DirtyBlock)
	blk3.Flags.Set(common.TruncatedBlock)
	h.CacheObj.BlockOffsetList.BlockList = append(h.CacheObj.BlockOffsetList.BlockList, blk1, blk2, blk3)
	bol.Flags.Clear(common.SmallFile)

	err := s.az.FlushFile(internal.FlushFileOptions{Handle: h})
	s.assert.Nil(err)

	output, err := s.az.ReadFile(internal.ReadFileOptions{Handle: h})
	s.assert.Nil(err)
	data := make([]byte, blockSize)
	s.assert.EqualValues(blk1.Data, output[0:blockSize])
	s.assert.EqualValues(data, output[blockSize:2*blockSize])
	s.assert.EqualValues(data, output[2*blockSize:3*blockSize])
}

func (s *blockBlobTestSuite) TestFlushFileAppendAndTruncateBlocksChunkedFile() {
	defer s.cleanupTest()

	// Setup
	name := generateFileName()
	blockSize := 7 * MB
	fileSize := 16 * MB
	h, _ := s.az.CreateFile(internal.CreateFileOptions{Name: name})
	data := make([]byte, fileSize)
	rand.Read(data)

	// use our method to make the max upload size (size before a blob is broken down to blocks) to 4 Bytes
	err := uploadReaderAtToBlockBlob(ctx, bytes.NewReader(data), int64(len(data)), 4, s.containerClient.NewBlockBlobClient(name), &blockblob.UploadBufferOptions{
		BlockSize: int64(blockSize),
	})
	s.assert.Nil(err)
	bol, _ := s.az.GetFileBlockOffsets(internal.GetFileBlockOffsetsOptions{Name: name})
	handlemap.CreateCacheObject(int64(16*MB), h)
	h.CacheObj.BlockOffsetList = bol
	h.CacheObj.BlockIdLength = 16

	data1 := make([]byte, blockSize)
	rand.Read(data1)
	blk1 := &common.Block{
		StartIndex: int64(fileSize),
		EndIndex:   int64(fileSize + blockSize),
		Id:         base64.StdEncoding.EncodeToString(common.NewUUIDWithLength(h.CacheObj.BlockIdLength)),
		Data:       data1,
	}
	blk1.Flags.Set(common.DirtyBlock)

	blk2 := &common.Block{
		StartIndex: int64(fileSize + blockSize),
		EndIndex:   int64(fileSize + 2*blockSize),
		Id:         base64.StdEncoding.EncodeToString(common.NewUUIDWithLength(h.CacheObj.BlockIdLength)),
	}
	blk2.Flags.Set(common.DirtyBlock)
	blk2.Flags.Set(common.TruncatedBlock)

	blk3 := &common.Block{
		StartIndex: int64(fileSize + 2*blockSize),
		EndIndex:   int64(fileSize + 3*blockSize),
		Id:         base64.StdEncoding.EncodeToString(common.NewUUIDWithLength(h.CacheObj.BlockIdLength)),
	}
	blk3.Flags.Set(common.DirtyBlock)
	blk3.Flags.Set(common.TruncatedBlock)
	h.CacheObj.BlockOffsetList.BlockList = append(h.CacheObj.BlockOffsetList.BlockList, blk1, blk2, blk3)
	bol.Flags.Clear(common.SmallFile)

	err = s.az.FlushFile(internal.FlushFileOptions{Handle: h})
	s.assert.Nil(err)

	// file should be empty
	output, err := s.az.ReadFile(internal.ReadFileOptions{Handle: h})
	s.assert.Nil(err)
	s.assert.EqualValues(data, output[:fileSize])
	emptyData := make([]byte, blockSize)
	s.assert.EqualValues(blk1.Data, output[fileSize:fileSize+blockSize])
	s.assert.EqualValues(emptyData, output[fileSize+blockSize:fileSize+2*blockSize])
	s.assert.EqualValues(emptyData, output[fileSize+2*blockSize:fileSize+3*blockSize])
}

func (s *blockBlobTestSuite) TestUpdateConfig() {
	defer s.cleanupTest()

	s.az.storage.UpdateConfig(AzStorageConfig{
		blockSize:             7 * MB,
		maxConcurrency:        4,
		defaultTier:           to.Ptr(blob.AccessTierArchive),
		ignoreAccessModifiers: true,
	})

	s.assert.EqualValues(7*MB, s.az.storage.(*BlockBlob).Config.blockSize)
	s.assert.EqualValues(4, s.az.storage.(*BlockBlob).Config.maxConcurrency)
	s.assert.EqualValues(blob.AccessTierArchive, *s.az.storage.(*BlockBlob).Config.defaultTier)
	s.assert.True(s.az.storage.(*BlockBlob).Config.ignoreAccessModifiers)
}

func (s *blockBlobTestSuite) TestMD5SetOnUpload() {
	defer s.cleanupTest()
	vdConfig := fmt.Sprintf("azstorage:\n  account-name: %s\n  endpoint: https://%s.blob.core.windows.net/\n  type: block\n  account-key: %s\n  mode: key\n  container: %s\n  fail-unsupported-op: true\n  virtual-directory: true",
		storageTestConfigurationParameters.BlockAccount, storageTestConfigurationParameters.BlockAccount, storageTestConfigurationParameters.BlockKey, s.container)
	configs := []string{"", vdConfig}
	for _, c := range configs {
		// This is a little janky but required since testify suite does not support running setup or clean up for subtests.
		s.tearDownTestHelper(false)
		s.setupTestHelper(c, s.container, true)
		testName := ""
		if c != "" {
			testName = "virtual-directory"
		}
		s.Run(testName, func() {
			// Setup
			s.tearDownTestHelper(false) // Don't delete the generated container.

			config := fmt.Sprintf("azstorage:\n  account-name: %s\n  endpoint: https://%s.blob.core.windows.net/\n  type: block\n  account-key: %s\n  mode: key\n  container: %s\n  update-md5: true\n",
				storageTestConfigurationParameters.BlockAccount, storageTestConfigurationParameters.BlockAccount, storageTestConfigurationParameters.BlockKey, s.container)
			s.setupTestHelper(config, s.container, true)

			name := generateFileName()
			f, err := os.Create(name)
			s.assert.Nil(err)
			s.assert.NotNil(f)

			data := make([]byte, blockblob.MaxUploadBlobBytes+1)
			_, _ = rand.Read(data)

			n, err := f.Write(data)
			s.assert.Nil(err)
			s.assert.EqualValues(n, blockblob.MaxUploadBlobBytes+1)
			_, _ = f.Seek(0, 0)

			err = s.az.storage.WriteFromFile(name, nil, f)
			s.assert.Nil(err)

			prop, err := s.az.storage.GetAttr(name)
			s.assert.Nil(err)
			s.assert.NotEmpty(prop.MD5)

			_, _ = f.Seek(0, 0)
			localMD5, err := common.GetMD5(f)
			s.assert.Nil(err)
			s.assert.EqualValues(localMD5, prop.MD5)

			_ = s.az.storage.DeleteFile(name)
			_ = f.Close()
			_ = os.Remove(name)
		})
	}
}

func (s *blockBlobTestSuite) TestMD5NotSetOnUpload() {
	defer s.cleanupTest()
	vdConfig := fmt.Sprintf("azstorage:\n  account-name: %s\n  endpoint: https://%s.blob.core.windows.net/\n  type: block\n  account-key: %s\n  mode: key\n  container: %s\n  fail-unsupported-op: true\n  virtual-directory: true",
		storageTestConfigurationParameters.BlockAccount, storageTestConfigurationParameters.BlockAccount, storageTestConfigurationParameters.BlockKey, s.container)
	configs := []string{"", vdConfig}
	for _, c := range configs {
		// This is a little janky but required since testify suite does not support running setup or clean up for subtests.
		s.tearDownTestHelper(false)
		s.setupTestHelper(c, s.container, true)
		testName := ""
		if c != "" {
			testName = "virtual-directory"
		}
		s.Run(testName, func() {
			// Setup
			s.tearDownTestHelper(false) // Don't delete the generated container.

			config := fmt.Sprintf("azstorage:\n  account-name: %s\n  endpoint: https://%s.blob.core.windows.net/\n  type: block\n  account-key: %s\n  mode: key\n  container: %s\n  update-md5: false\n",
				storageTestConfigurationParameters.BlockAccount, storageTestConfigurationParameters.BlockAccount, storageTestConfigurationParameters.BlockKey, s.container)
			s.setupTestHelper(config, s.container, true)

			name := generateFileName()
			f, err := os.Create(name)
			s.assert.Nil(err)
			s.assert.NotNil(f)

			data := make([]byte, blockblob.MaxUploadBlobBytes+1)
			_, _ = rand.Read(data)

			n, err := f.Write(data)
			s.assert.Nil(err)
			s.assert.EqualValues(n, blockblob.MaxUploadBlobBytes+1)
			_, _ = f.Seek(0, 0)

			err = s.az.storage.WriteFromFile(name, nil, f)
			s.assert.Nil(err)

			prop, err := s.az.storage.GetAttr(name)
			s.assert.Nil(err)
			s.assert.Empty(prop.MD5)

			_ = s.az.storage.DeleteFile(name)
			_ = f.Close()
			_ = os.Remove(name)
		})
	}
}

func (s *blockBlobTestSuite) TestMD5AutoSetOnUpload() {
	defer s.cleanupTest()
	vdConfig := fmt.Sprintf("azstorage:\n  account-name: %s\n  endpoint: https://%s.blob.core.windows.net/\n  type: block\n  account-key: %s\n  mode: key\n  container: %s\n  fail-unsupported-op: true\n  virtual-directory: true",
		storageTestConfigurationParameters.BlockAccount, storageTestConfigurationParameters.BlockAccount, storageTestConfigurationParameters.BlockKey, s.container)
	configs := []string{"", vdConfig}
	for _, c := range configs {
		// This is a little janky but required since testify suite does not support running setup or clean up for subtests.
		s.tearDownTestHelper(false)
		s.setupTestHelper(c, s.container, true)
		testName := ""
		if c != "" {
			testName = "virtual-directory"
		}
		s.Run(testName, func() {
			// Setup
			s.tearDownTestHelper(false) // Don't delete the generated container.

			config := fmt.Sprintf("azstorage:\n  account-name: %s\n  endpoint: https://%s.blob.core.windows.net/\n  type: block\n  account-key: %s\n  mode: key\n  container: %s\n  update-md5: false\n",
				storageTestConfigurationParameters.BlockAccount, storageTestConfigurationParameters.BlockAccount, storageTestConfigurationParameters.BlockKey, s.container)
			s.setupTestHelper(config, s.container, true)

			name := generateFileName()
			f, err := os.Create(name)
			s.assert.Nil(err)
			s.assert.NotNil(f)

			data := make([]byte, 100)
			_, _ = rand.Read(data)

			n, err := f.Write(data)
			s.assert.Nil(err)
			s.assert.EqualValues(n, 100)
			_, _ = f.Seek(0, 0)

			err = s.az.storage.WriteFromFile(name, nil, f)
			s.assert.Nil(err)

			prop, err := s.az.storage.GetAttr(name)
			s.assert.Nil(err)
			s.assert.NotEmpty(prop.MD5)

			_, _ = f.Seek(0, 0)
			localMD5, err := common.GetMD5(f)
			s.assert.Nil(err)
			s.assert.EqualValues(localMD5, prop.MD5)

			_ = s.az.storage.DeleteFile(name)
			_ = f.Close()
			_ = os.Remove(name)
		})
	}
}

func (s *blockBlobTestSuite) TestInvalidateMD5PostUpload() {
	defer s.cleanupTest()
	vdConfig := fmt.Sprintf("azstorage:\n  account-name: %s\n  endpoint: https://%s.blob.core.windows.net/\n  type: block\n  account-key: %s\n  mode: key\n  container: %s\n  fail-unsupported-op: true\n  virtual-directory: true",
		storageTestConfigurationParameters.BlockAccount, storageTestConfigurationParameters.BlockAccount, storageTestConfigurationParameters.BlockKey, s.container)
	configs := []string{"", vdConfig}
	for _, c := range configs {
		// This is a little janky but required since testify suite does not support running setup or clean up for subtests.
		s.tearDownTestHelper(false)
		s.setupTestHelper(c, s.container, true)
		testName := ""
		if c != "" {
			testName = "virtual-directory"
		}
		s.Run(testName, func() {
			// Setup
			s.tearDownTestHelper(false) // Don't delete the generated container.

			config := fmt.Sprintf("azstorage:\n  account-name: %s\n  endpoint: https://%s.blob.core.windows.net/\n  type: block\n  account-key: %s\n  mode: key\n  container: %s\n  update-md5: true\n  validate-md5: true\n",
				storageTestConfigurationParameters.BlockAccount, storageTestConfigurationParameters.BlockAccount, storageTestConfigurationParameters.BlockKey, s.container)
			s.setupTestHelper(config, s.container, true)

			name := generateFileName()
			f, err := os.Create(name)
			s.assert.Nil(err)
			s.assert.NotNil(f)

			data := make([]byte, 100)
			_, _ = rand.Read(data)

			n, err := f.Write(data)
			s.assert.Nil(err)
			s.assert.EqualValues(n, 100)
			_, _ = f.Seek(0, 0)

			err = s.az.storage.WriteFromFile(name, nil, f)
			s.assert.Nil(err)

			blobClient := s.containerClient.NewBlobClient(name)
			_, _ = blobClient.SetHTTPHeaders(context.Background(), blob.HTTPHeaders{BlobContentMD5: []byte("blobfuse")}, nil)

			prop, err := s.az.storage.GetAttr(name)
			s.assert.Nil(err)
			s.assert.NotEmpty(prop.MD5)

			_, _ = f.Seek(0, 0)
			localMD5, err := common.GetMD5(f)
			s.assert.Nil(err)
			s.assert.NotEqualValues(localMD5, prop.MD5)

			_ = s.az.storage.DeleteFile(name)
			_ = f.Close()
			_ = os.Remove(name)
		})
	}
}

func (s *blockBlobTestSuite) TestValidateAutoMD5OnRead() {
	defer s.cleanupTest()
	vdConfig := fmt.Sprintf("azstorage:\n  account-name: %s\n  endpoint: https://%s.blob.core.windows.net/\n  type: block\n  account-key: %s\n  mode: key\n  container: %s\n  fail-unsupported-op: true\n  virtual-directory: true",
		storageTestConfigurationParameters.BlockAccount, storageTestConfigurationParameters.BlockAccount, storageTestConfigurationParameters.BlockKey, s.container)
	configs := []string{"", vdConfig}
	for _, c := range configs {
		// This is a little janky but required since testify suite does not support running setup or clean up for subtests.
		s.tearDownTestHelper(false)
		s.setupTestHelper(c, s.container, true)
		testName := ""
		if c != "" {
			testName = "virtual-directory"
		}
		s.Run(testName, func() {
			// Setup
			s.tearDownTestHelper(false) // Don't delete the generated container.

			config := fmt.Sprintf("azstorage:\n  account-name: %s\n  endpoint: https://%s.blob.core.windows.net/\n  type: block\n  account-key: %s\n  mode: key\n  container: %s\n  update-md5: false\n  validate-md5: true\n",
				storageTestConfigurationParameters.BlockAccount, storageTestConfigurationParameters.BlockAccount, storageTestConfigurationParameters.BlockKey, s.container)
			s.setupTestHelper(config, s.container, true)

			name := generateFileName()
			f, err := os.Create(name)
			s.assert.Nil(err)
			s.assert.NotNil(f)

			data := make([]byte, 100)
			_, _ = rand.Read(data)

			n, err := f.Write(data)
			s.assert.Nil(err)
			s.assert.EqualValues(n, 100)
			_, _ = f.Seek(0, 0)

			err = s.az.storage.WriteFromFile(name, nil, f)
			s.assert.Nil(err)
			_ = f.Close()
			_ = os.Remove(name)

			prop, err := s.az.storage.GetAttr(name)
			s.assert.Nil(err)
			s.assert.NotEmpty(prop.MD5)

			f, err = os.Create(name)
			s.assert.Nil(err)
			s.assert.NotNil(f)

			err = s.az.storage.ReadToFile(name, 0, 100, f)
			s.assert.Nil(err)

			_ = s.az.storage.DeleteFile(name)
			_ = os.Remove(name)
		})
	}
}

func (s *blockBlobTestSuite) TestValidateManualMD5OnRead() {
	defer s.cleanupTest()
	vdConfig := fmt.Sprintf("azstorage:\n  account-name: %s\n  endpoint: https://%s.blob.core.windows.net/\n  type: block\n  account-key: %s\n  mode: key\n  container: %s\n  fail-unsupported-op: true\n  virtual-directory: true",
		storageTestConfigurationParameters.BlockAccount, storageTestConfigurationParameters.BlockAccount, storageTestConfigurationParameters.BlockKey, s.container)
	configs := []string{"", vdConfig}
	for _, c := range configs {
		// This is a little janky but required since testify suite does not support running setup or clean up for subtests.
		s.tearDownTestHelper(false)
		s.setupTestHelper(c, s.container, true)
		testName := ""
		if c != "" {
			testName = "virtual-directory"
		}
		s.Run(testName, func() {
			// Setup
			s.tearDownTestHelper(false) // Don't delete the generated container.

			config := fmt.Sprintf("azstorage:\n  account-name: %s\n  endpoint: https://%s.blob.core.windows.net/\n  type: block\n  account-key: %s\n  mode: key\n  container: %s\n  update-md5: true\n  validate-md5: true\n",
				storageTestConfigurationParameters.BlockAccount, storageTestConfigurationParameters.BlockAccount, storageTestConfigurationParameters.BlockKey, s.container)
			s.setupTestHelper(config, s.container, true)

			name := generateFileName()
			f, err := os.Create(name)
			s.assert.Nil(err)
			s.assert.NotNil(f)

			data := make([]byte, blockblob.MaxUploadBlobBytes+1)
			_, _ = rand.Read(data)

			n, err := f.Write(data)
			s.assert.Nil(err)
			s.assert.EqualValues(n, blockblob.MaxUploadBlobBytes+1)
			_, _ = f.Seek(0, 0)

			err = s.az.storage.WriteFromFile(name, nil, f)
			s.assert.Nil(err)
			_ = f.Close()
			_ = os.Remove(name)

			prop, err := s.az.storage.GetAttr(name)
			s.assert.Nil(err)
			s.assert.NotEmpty(prop.MD5)

			f, err = os.Create(name)
			s.assert.Nil(err)
			s.assert.NotNil(f)

			err = s.az.storage.ReadToFile(name, 0, blockblob.MaxUploadBlobBytes+1, f)
			s.assert.Nil(err)

			_ = s.az.storage.DeleteFile(name)
			_ = os.Remove(name)
		})
	}
}

func (s *blockBlobTestSuite) TestInvalidMD5OnRead() {
	defer s.cleanupTest()
	vdConfig := fmt.Sprintf("azstorage:\n  account-name: %s\n  endpoint: https://%s.blob.core.windows.net/\n  type: block\n  account-key: %s\n  mode: key\n  container: %s\n  fail-unsupported-op: true\n  virtual-directory: true",
		storageTestConfigurationParameters.BlockAccount, storageTestConfigurationParameters.BlockAccount, storageTestConfigurationParameters.BlockKey, s.container)
	configs := []string{"", vdConfig}
	for _, c := range configs {
		// This is a little janky but required since testify suite does not support running setup or clean up for subtests.
		s.tearDownTestHelper(false)
		s.setupTestHelper(c, s.container, true)
		testName := ""
		if c != "" {
			testName = "virtual-directory"
		}
		s.Run(testName, func() {
			// Setup
			s.tearDownTestHelper(false) // Don't delete the generated container.

			config := fmt.Sprintf("azstorage:\n  account-name: %s\n  endpoint: https://%s.blob.core.windows.net/\n  type: block\n  account-key: %s\n  mode: key\n  container: %s\n  update-md5: true\n  validate-md5: true\n",
				storageTestConfigurationParameters.BlockAccount, storageTestConfigurationParameters.BlockAccount, storageTestConfigurationParameters.BlockKey, s.container)
			s.setupTestHelper(config, s.container, true)

			name := generateFileName()
			f, err := os.Create(name)
			s.assert.Nil(err)
			s.assert.NotNil(f)

			data := make([]byte, 100)
			_, _ = rand.Read(data)

			n, err := f.Write(data)
			s.assert.Nil(err)
			s.assert.EqualValues(n, 100)
			_, _ = f.Seek(0, 0)

			err = s.az.storage.WriteFromFile(name, nil, f)
			s.assert.Nil(err)
			_ = f.Close()
			_ = os.Remove(name)

			blobClient := s.containerClient.NewBlobClient(name)
			_, _ = blobClient.SetHTTPHeaders(context.Background(), blob.HTTPHeaders{BlobContentMD5: []byte("blobfuse")}, nil)

			prop, err := s.az.storage.GetAttr(name)
			s.assert.Nil(err)
			s.assert.NotEmpty(prop.MD5)

			f, err = os.Create(name)
			s.assert.Nil(err)
			s.assert.NotNil(f)

			err = s.az.storage.ReadToFile(name, 0, 100, f)
			s.assert.NotNil(err)
			s.assert.Contains(err.Error(), "md5 sum mismatch on download")

			_ = s.az.storage.DeleteFile(name)
			_ = os.Remove(name)
		})
	}
}

func (s *blockBlobTestSuite) TestInvalidMD5OnReadNoVaildate() {
	defer s.cleanupTest()
	vdConfig := fmt.Sprintf("azstorage:\n  account-name: %s\n  endpoint: https://%s.blob.core.windows.net/\n  type: block\n  account-key: %s\n  mode: key\n  container: %s\n  fail-unsupported-op: true\n  virtual-directory: true",
		storageTestConfigurationParameters.BlockAccount, storageTestConfigurationParameters.BlockAccount, storageTestConfigurationParameters.BlockKey, s.container)
	configs := []string{"", vdConfig}
	for _, c := range configs {
		// This is a little janky but required since testify suite does not support running setup or clean up for subtests.
		s.tearDownTestHelper(false)
		s.setupTestHelper(c, s.container, true)
		testName := ""
		if c != "" {
			testName = "virtual-directory"
		}
		s.Run(testName, func() {
			// Setup
			s.tearDownTestHelper(false) // Don't delete the generated container.

			config := fmt.Sprintf("azstorage:\n  account-name: %s\n  endpoint: https://%s.blob.core.windows.net/\n  type: block\n  account-key: %s\n  mode: key\n  container: %s\n  update-md5: true\n  validate-md5: false\n",
				storageTestConfigurationParameters.BlockAccount, storageTestConfigurationParameters.BlockAccount, storageTestConfigurationParameters.BlockKey, s.container)
			s.setupTestHelper(config, s.container, true)

			name := generateFileName()
			f, err := os.Create(name)
			s.assert.Nil(err)
			s.assert.NotNil(f)

			data := make([]byte, 100)
			_, _ = rand.Read(data)

			n, err := f.Write(data)
			s.assert.Nil(err)
			s.assert.EqualValues(n, 100)
			_, _ = f.Seek(0, 0)

			err = s.az.storage.WriteFromFile(name, nil, f)
			s.assert.Nil(err)
			_ = f.Close()
			_ = os.Remove(name)

			blobClient := s.containerClient.NewBlobClient(name)
			_, _ = blobClient.SetHTTPHeaders(context.Background(), blob.HTTPHeaders{BlobContentMD5: []byte("blobfuse")}, nil)

			prop, err := s.az.storage.GetAttr(name)
			s.assert.Nil(err)
			s.assert.NotEmpty(prop.MD5)

			f, err = os.Create(name)
			s.assert.Nil(err)
			s.assert.NotNil(f)

			err = s.az.storage.ReadToFile(name, 0, 100, f)
			s.assert.Nil(err)

			_ = s.az.storage.DeleteFile(name)
			_ = os.Remove(name)
		})
	}
}

func (s *blockBlobTestSuite) TestDownloadBlobWithCPKEnabled() {
	defer s.cleanupTest()
	s.tearDownTestHelper(false)
	CPKEncryptionKey, CPKEncryptionKeySHA256 := generateCPKInfo()

	config := fmt.Sprintf("azstorage:\n  account-name: %s\n  endpoint: https://%s.blob.core.windows.net/\n  type: block\n  account-key: %s\n  mode: key\n  container: %s\n  cpk-enabled: true\n  cpk-encryption-key: %s\n  cpk-encryption-key-sha256: %s\n",
		storageTestConfigurationParameters.BlockAccount, storageTestConfigurationParameters.BlockAccount, storageTestConfigurationParameters.BlockKey, s.container, CPKEncryptionKey, CPKEncryptionKeySHA256)
	s.setupTestHelper(config, s.container, false)

	blobCPKOpt := &blob.CPKInfo{
		EncryptionKey:       &CPKEncryptionKey,
		EncryptionKeySHA256: &CPKEncryptionKeySHA256,
		EncryptionAlgorithm: to.Ptr(blob.EncryptionAlgorithmTypeAES256),
	}
	name := generateFileName()
	s.az.CreateFile(internal.CreateFileOptions{Name: name})
	testData := "test data"
	data := []byte(testData)

	err := uploadReaderAtToBlockBlob(ctx, bytes.NewReader(data), int64(len(data)), 100, s.containerClient.NewBlockBlobClient(name), &blockblob.UploadBufferOptions{
		CPKInfo: blobCPKOpt,
	})
	s.assert.Nil(err)

	f, err := os.Create(name)
	s.assert.Nil(err)
	s.assert.NotNil(f)

	err = s.az.storage.ReadToFile(name, 0, int64(len(data)), f)
	s.assert.Nil(err)
	fileData, err := os.ReadFile(name)
	s.assert.Nil(err)
	s.assert.EqualValues(data, fileData)

	buf := make([]byte, len(data))
	err = s.az.storage.ReadInBuffer(name, 0, int64(len(data)), buf, nil)
	s.assert.Nil(err)
	s.assert.EqualValues(data, buf)

	rbuf, err := s.az.storage.ReadBuffer(name, 0, int64(len(data)))
	s.assert.Nil(err)
	s.assert.EqualValues(data, rbuf)
	_ = s.az.storage.DeleteFile(name)
	_ = os.Remove(name)
}

func (s *blockBlobTestSuite) TestUploadBlobWithCPKEnabled() {
	defer s.cleanupTest()
	s.tearDownTestHelper(false)

	CPKEncryptionKey, CPKEncryptionKeySHA256 := generateCPKInfo()

	config := fmt.Sprintf("azstorage:\n  account-name: %s\n  endpoint: https://%s.blob.core.windows.net/\n  type: block\n  cpk-enabled: true\n  cpk-encryption-key: %s\n  cpk-encryption-key-sha256: %s\n  account-key: %s\n  mode: key\n  container: %s\n",
		storageTestConfigurationParameters.BlockAccount, storageTestConfigurationParameters.BlockAccount, CPKEncryptionKey, CPKEncryptionKeySHA256, storageTestConfigurationParameters.BlockKey, s.container)
	s.setupTestHelper(config, s.container, false)

	blobCPKOpt := &blob.CPKInfo{
		EncryptionKey:       &CPKEncryptionKey,
		EncryptionKeySHA256: &CPKEncryptionKeySHA256,
		EncryptionAlgorithm: to.Ptr(blob.EncryptionAlgorithmTypeAES256),
	}
	name1 := generateFileName()
	f, err := os.Create(name1)
	s.assert.Nil(err)
	s.assert.NotNil(f)

	testData := "test data"
	data := []byte(testData)
	_, err = f.Write(data)
	s.assert.Nil(err)
	_, _ = f.Seek(0, 0)

	err = s.az.storage.WriteFromFile(name1, nil, f)
	s.assert.Nil(err)

	file := s.containerClient.NewBlobClient(name1)

	attr, err := s.az.storage.(*BlockBlob).GetAttr(name1)
	s.assert.Nil(err)
	s.assert.NotNil(attr)
	resp, err := file.DownloadStream(ctx, &blob.DownloadStreamOptions{
		Range: blob.HTTPRange{Offset: 0, Count: int64(len(data))},
	})
	s.assert.NotNil(err)
	s.assert.Nil(resp.RequestID)

	resp, err = file.DownloadStream(ctx, &blob.DownloadStreamOptions{
		Range:   blob.HTTPRange{Offset: 0, Count: int64(len(data))},
		CPKInfo: blobCPKOpt,
	})
	s.assert.Nil(err)
	s.assert.NotNil(resp.RequestID)

	name2 := generateFileName()
	eTag, err := s.az.storage.WriteFromBuffer(internal.WriteFromBufferOptions{Name: name2,
		Data: data})
	s.assert.Nil(err)
	s.assert.NotEqual(eTag, "")

	file = s.containerClient.NewBlobClient(name2)
	resp, err = file.DownloadStream(ctx, &blob.DownloadStreamOptions{
		Range: blob.HTTPRange{Offset: 0, Count: int64(len(data))},
	})
	s.assert.NotNil(err)
	s.assert.Nil(resp.RequestID)

	resp, err = file.DownloadStream(ctx, &blob.DownloadStreamOptions{
		Range:   blob.HTTPRange{Offset: 0, Count: int64(len(data))},
		CPKInfo: blobCPKOpt,
	})
	s.assert.Nil(err)
	s.assert.NotNil(resp.RequestID)

	_ = s.az.storage.DeleteFile(name1)
	_ = s.az.storage.DeleteFile(name2)
	_ = os.Remove(name1)
}

// func (s *blockBlobTestSuite) TestRAGRS() {
// 	defer s.cleanupTest()
// 	// Setup
// 	name := generateFileName()
// 	h, _ := s.az.CreateFile(internal.CreateFileOptions{Name: name})
// 	testData := "test data"
// 	data := []byte(testData)
// 	s.az.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data})
// 	h, _ = s.az.OpenFile(internal.OpenFileOptions{Name: name})
// 	s.az.CloseFile(internal.CloseFileOptions{Handle: h})

// 	// This can be flaky since it may take time to replicate the data. We could hardcode a container and file for this test
// 	time.Sleep(time.Second * time.Duration(10))

// 	s.tearDownTestHelper(false) // Don't delete the generated container.

// 	config := fmt.Sprintf("azstorage:\n  account-name: %s\n  type: block\n  account-key: %s\n  mode: key\n  container: %s\n  endpoint: https://%s-secondary.blob.core.windows.net\n",
// 		storageTestConfigurationParameters.BlockAccount, storageTestConfigurationParameters.BlockKey, s.container, storageTestConfigurationParameters.BlockAccount)
// 	s.setupTestHelper(config, s.container, false) // Don't create a new container

// 	h, _ = s.az.OpenFile(internal.OpenFileOptions{Name: name})
// 	output, err := s.az.ReadFile(internal.ReadFileOptions{Handle: h})
// 	s.assert.Nil(err)
// 	s.assert.EqualValues(testData, output)
// 	s.az.CloseFile(internal.CloseFileOptions{Handle: h})
// }

func (suite *blockBlobTestSuite) TestTruncateSmallFileToSmaller() {
	suite.UtilityFunctionTestTruncateFileToSmaller(20*MB, 10*MB)
}

func (suite *blockBlobTestSuite) TestTruncateSmallFileToLarger() {
	suite.UtilityFunctionTruncateFileToLarger(10*MB, 20*MB)
}

func (suite *blockBlobTestSuite) TestTruncateBlockFileToSmaller() {
	suite.UtilityFunctionTestTruncateFileToSmaller(300*MB, 290*MB)
}

func (suite *blockBlobTestSuite) TestTruncateBlockFileToLarger() {
	suite.UtilityFunctionTruncateFileToLarger(290*MB, 300*MB)
}

func (suite *blockBlobTestSuite) TestTruncateNoBlockFileToLarger() {
	suite.UtilityFunctionTruncateFileToLarger(200*MB, 300*MB)
}

func (s *blockBlobTestSuite) TestBlobFilters() {
	defer s.cleanupTest()
	// Setup
	var err error
	name := generateDirectoryName()
	err = s.az.CreateDir(internal.CreateDirOptions{Name: name})
	s.assert.Nil(err)
	_, err = s.az.CreateFile(internal.CreateFileOptions{Name: name + "/abcd1.txt"})
	s.assert.Nil(err)
	_, err = s.az.CreateFile(internal.CreateFileOptions{Name: name + "/abcd2.txt"})
	s.assert.Nil(err)
	_, err = s.az.CreateFile(internal.CreateFileOptions{Name: name + "/abcd3.txt"})
	s.assert.Nil(err)
	_, err = s.az.CreateFile(internal.CreateFileOptions{Name: name + "/abcd4.txt"})
	s.assert.Nil(err)
	_, err = s.az.CreateFile(internal.CreateFileOptions{Name: name + "/bcd1.txt"})
	s.assert.Nil(err)
	_, err = s.az.CreateFile(internal.CreateFileOptions{Name: name + "/cd1.txt"})
	s.assert.Nil(err)
	_, err = s.az.CreateFile(internal.CreateFileOptions{Name: name + "/d1.txt"})
	s.assert.Nil(err)
	err = s.az.CreateDir(internal.CreateDirOptions{Name: name + "/subdir"})
	s.assert.Nil(err)

	var iteration int = 0
	var marker string = ""
	blobList := make([]*internal.ObjAttr, 0)

	for {
		new_list, new_marker, err := s.az.StreamDir(internal.StreamDirOptions{Name: name + "/", Token: marker, Count: 50})
		s.assert.Nil(err)
		blobList = append(blobList, new_list...)
		marker = new_marker
		iteration++

		log.Debug("AzStorage::ReadDir : So far retrieved %d objects in %d iterations", len(blobList), iteration)
		if new_marker == "" {
			break
		}
	}
	s.assert.EqualValues(8, len(blobList))
	err = s.az.storage.(*BlockBlob).SetFilter("name=^abcd.*")
	s.assert.Nil(err)

	blobList = make([]*internal.ObjAttr, 0)
	for {
		new_list, new_marker, err := s.az.StreamDir(internal.StreamDirOptions{Name: name + "/", Token: marker, Count: 50})
		s.assert.Nil(err)
		blobList = append(blobList, new_list...)
		marker = new_marker
		iteration++

		log.Debug("AzStorage::ReadDir : So far retrieved %d objects in %d iterations", len(blobList), iteration)
		if new_marker == "" {
			break
		}
	}
	// Only 4 files matches the pattern but there is a directory as well and directories are not filtered by blobfilter
	s.assert.EqualValues(5, len(blobList))
	err = s.az.storage.(*BlockBlob).SetFilter("name=^bla.*")
	s.assert.Nil(err)

	blobList = make([]*internal.ObjAttr, 0)
	for {
		new_list, new_marker, err := s.az.StreamDir(internal.StreamDirOptions{Name: name + "/", Token: marker, Count: 50})
		s.assert.Nil(err)
		blobList = append(blobList, new_list...)
		marker = new_marker
		iteration++

		log.Debug("AzStorage::ReadDir : So far retrieved %d objects in %d iterations", len(blobList), iteration)
		if new_marker == "" {
			break
		}
	}

	s.assert.EqualValues(1, len(blobList))
	err = s.az.storage.(*BlockBlob).SetFilter("")
	s.assert.Nil(err)
}

func (suite *blockBlobTestSuite) UtilityFunctionTestTruncateFileToSmaller(size int, truncatedLength int) {
	defer suite.cleanupTest()
	// Setup
	vdConfig := fmt.Sprintf("azstorage:\n  account-name: %s\n  endpoint: https://%s.blob.core.windows.net/\n  type: block\n  account-key: %s\n  mode: key\n  container: %s\n  fail-unsupported-op: true\n  virtual-directory: true",
		storageTestConfigurationParameters.BlockAccount, storageTestConfigurationParameters.BlockAccount, storageTestConfigurationParameters.BlockKey, suite.container)
	// // This is a little janky but required since testify suite does not support running setup or clean up for subtests.

	suite.tearDownTestHelper(false)
	suite.setupTestHelper(vdConfig, suite.container, true)

	name := generateFileName()
	h, err := suite.az.CreateFile(internal.CreateFileOptions{Name: name})
	suite.assert.Nil(err)

	data := make([]byte, size)
	suite.az.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data})

	err = suite.az.TruncateFile(internal.TruncateFileOptions{Name: name, Size: int64(truncatedLength)})
	suite.assert.Nil(err)

	// Blob should have updated data
	file := suite.containerClient.NewBlobClient(name)
	resp, err := file.DownloadStream(ctx, &blob.DownloadStreamOptions{
		Range: blob.HTTPRange{Offset: 0, Count: int64(truncatedLength)},
	})
	suite.assert.Nil(err)
	suite.assert.NotNil(resp.ContentLength)
	suite.assert.EqualValues(truncatedLength, *resp.ContentLength)
	output, _ := io.ReadAll(resp.Body)
	suite.assert.EqualValues(data[:truncatedLength], output[:])
}

func (suite *blockBlobTestSuite) UtilityFunctionTruncateFileToLarger(size int, truncatedLength int) {
	defer suite.cleanupTest()
	// Setup
	vdConfig := fmt.Sprintf("azstorage:\n  account-name: %s\n  endpoint: https://%s.blob.core.windows.net/\n  type: block\n  account-key: %s\n  mode: key\n  container: %s\n  fail-unsupported-op: true\n  virtual-directory: true",
		storageTestConfigurationParameters.BlockAccount, storageTestConfigurationParameters.BlockAccount, storageTestConfigurationParameters.BlockKey, suite.container)
	// // This is a little janky but required since testify suite does not support running setup or clean up for subtests.

	suite.tearDownTestHelper(false)
	suite.setupTestHelper(vdConfig, suite.container, true)

	name := generateFileName()
	h, err := suite.az.CreateFile(internal.CreateFileOptions{Name: name})
	suite.assert.Nil(err)

	data := make([]byte, size)
	suite.az.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data})

	err = suite.az.TruncateFile(internal.TruncateFileOptions{Name: name, Size: int64(truncatedLength)})
	suite.assert.Nil(err)

	// Blob should have updated data
	file := suite.containerClient.NewBlobClient(name)
	resp, err := file.DownloadStream(ctx, &blob.DownloadStreamOptions{
		Range: blob.HTTPRange{Offset: 0, Count: int64(truncatedLength)},
	})
	suite.assert.Nil(err)
	suite.assert.NotNil(resp.ContentLength)
	suite.assert.EqualValues(truncatedLength, *resp.ContentLength)
	output, _ := io.ReadAll(resp.Body)
	suite.assert.EqualValues(data, output[:size])

}

func (s *blockBlobTestSuite) TestList() {
	defer s.cleanupTest()
	// Setup
	s.tearDownTestHelper(false) // Don't delete the generated container.
	config := fmt.Sprintf("azstorage:\n  account-name: %s\n  endpoint: https://%s.dfs.core.windows.net/\n  type: block\n  account-key: %s\n  mode: key\n  container: %s\n  fail-unsupported-op: true",
		storageTestConfigurationParameters.BlockAccount, storageTestConfigurationParameters.BlockAccount, storageTestConfigurationParameters.BlockKey, s.container)
	s.setupTestHelper(config, s.container, true)

	base := generateDirectoryName()
	s.setupHierarchy(base)

	blobList, marker, err := s.az.storage.List("", nil, 0)
	s.assert.Nil(err)
	emptyString := ""
	s.assert.Equal(&emptyString, marker)
	s.assert.NotNil(blobList)
	s.assert.EqualValues(3, len(blobList))

	// Test listing with prefix
	blobList, marker, err = s.az.storage.List(base+"b/", nil, 0)
	s.assert.Nil(err)
	s.assert.Equal(&emptyString, marker)
	s.assert.NotNil(blobList)
	s.assert.EqualValues(1, len(blobList))
	s.assert.EqualValues("c1", blobList[0].Name)

	// Test listing with marker
	blobList, marker, err = s.az.storage.List(base, to.Ptr("invalid-marker"), 0)
	s.assert.NotNil(err)
	s.assert.Equal(0, len(blobList))
	s.assert.Nil(marker)

	// Test listing with count
	blobList, marker, err = s.az.storage.List("", nil, 1)
	s.assert.Nil(err)
	s.assert.NotNil(blobList)
	s.assert.NotEmpty(marker)
	s.assert.EqualValues(1, len(blobList))
	s.assert.EqualValues(base, blobList[0].Path)
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestBlockBlob(t *testing.T) {
	suite.Run(t, new(blockBlobTestSuite))
}
