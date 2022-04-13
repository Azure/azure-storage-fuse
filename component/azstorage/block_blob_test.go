// +build !authtest
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

package azstorage

import (
	"blobfuse2/common"
	"blobfuse2/common/config"
	"blobfuse2/common/log"
	"blobfuse2/internal"
	"blobfuse2/internal/handlemap"
	"bytes"
	"container/list"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/Azure/azure-pipeline-go/pipeline"
	"github.com/Azure/azure-storage-blob-go/azblob"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

var ctx = context.Background()

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
func uploadReaderAtToBlockBlob(ctx context.Context, reader io.ReaderAt, readerSize, singleUploadSize int64,
	blockBlobURL azblob.BlockBlobURL, o azblob.UploadToBlockBlobOptions) (azblob.CommonResponse, error) {
	if o.BlockSize == 0 {
		// If bufferSize > (BlockBlobMaxStageBlockBytes * BlockBlobMaxBlocks), then error
		if readerSize > azblob.BlockBlobMaxStageBlockBytes*azblob.BlockBlobMaxBlocks {
			return nil, errors.New("buffer is too large to upload to a block blob")
		}
		// If bufferSize <= singleUploadSize, then Upload should be used with just 1 I/O request
		if readerSize <= singleUploadSize {
			o.BlockSize = singleUploadSize // Default if unspecified
		} else {
			o.BlockSize = readerSize / azblob.BlockBlobMaxBlocks   // buffer / max blocks = block size to use all 50,000 blocks
			if o.BlockSize < azblob.BlobDefaultDownloadBlockSize { // If the block size is smaller than 4MB, round up to 4MB
				o.BlockSize = azblob.BlobDefaultDownloadBlockSize
			}
			// StageBlock will be called with blockSize blocks and a Parallelism of (BufferSize / BlockSize).
		}
	}

	if readerSize <= singleUploadSize {
		// If the size can fit in 1 Upload call, do it this way
		var body io.ReadSeeker = io.NewSectionReader(reader, 0, readerSize)
		if o.Progress != nil {
			body = pipeline.NewRequestBodyProgress(body, o.Progress)
		}
		return blockBlobURL.Upload(ctx, body, o.BlobHTTPHeaders, o.Metadata, o.AccessConditions, o.BlobAccessTier, o.BlobTagsMap, o.ClientProvidedKeyOptions)
	}

	var numBlocks = uint16(((readerSize - 1) / o.BlockSize) + 1)

	blockIDList := make([]string, numBlocks) // Base-64 encoded block IDs
	progress := int64(0)
	progressLock := &sync.Mutex{}

	err := azblob.DoBatchTransfer(ctx, azblob.BatchTransferOptions{
		OperationName: "uploadReaderAtToBlockBlob",
		TransferSize:  readerSize,
		ChunkSize:     o.BlockSize,
		Parallelism:   o.Parallelism,
		Operation: func(offset int64, count int64, ctx context.Context) error {
			// This function is called once per block.
			// It is passed this block's offset within the buffer and its count of bytes
			// Prepare to read the proper block/section of the buffer
			var body io.ReadSeeker = io.NewSectionReader(reader, offset, count)
			blockNum := offset / o.BlockSize
			if o.Progress != nil {
				blockProgress := int64(0)
				body = pipeline.NewRequestBodyProgress(body,
					func(bytesTransferred int64) {
						diff := bytesTransferred - blockProgress
						blockProgress = bytesTransferred
						progressLock.Lock() // 1 goroutine at a time gets a progress report
						progress += diff
						o.Progress(progress)
						progressLock.Unlock()
					})
			}

			// Block IDs are unique values to avoid issue if 2+ clients are uploading blocks
			// at the same time causing PutBlockList to get a mix of blocks from all the clients.
			blockIDList[blockNum] = base64.StdEncoding.EncodeToString(newUUID().bytes())
			_, err := blockBlobURL.StageBlock(ctx, blockIDList[blockNum], body, o.AccessConditions.LeaseAccessConditions, nil, o.ClientProvidedKeyOptions)
			return err
		},
	})
	if err != nil {
		return nil, err
	}
	// All put blocks were successful, call Put Block List to finalize the blob
	return blockBlobURL.CommitBlockList(ctx, blockIDList, o.BlobHTTPHeaders, o.Metadata, o.AccessConditions, o.BlobAccessTier, o.BlobTagsMap, o.ClientProvidedKeyOptions)
}

type blockBlobTestSuite struct {
	suite.Suite
	assert       *assert.Assertions
	az           *AzStorage
	serviceUrl   azblob.ServiceURL
	containerUrl azblob.ContainerURL
	config       string
	container    string
}

func newTestAzStorage(configuration string) *AzStorage {
	config.ReadConfigFromReader(strings.NewReader(configuration))
	az := NewazstorageComponent()
	az.Configure()

	return az.(*AzStorage)
}

func (s *blockBlobTestSuite) SetupTest() {
	// Logging config
	cfg := common.LogConfig{
		FilePath:    "./logfile.txt",
		MaxFileSize: 10,
		FileCount:   10,
		Level:       common.ELogLevel.LOG_DEBUG(),
	}
	log.SetDefaultLogger("base", cfg)

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

	cfgData, _ := ioutil.ReadAll(cfgFile)
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

	s.az = newTestAzStorage(configuration)
	s.az.Start(ctx) // Note: Start->TestValidation will fail but it doesn't matter. We are creating the container a few lines below anyway.
	// We could create the container before but that requires rewriting the code to new up a service client.

	s.serviceUrl = s.az.storage.(*BlockBlob).Service // Grab the service client to do some validation
	s.containerUrl = s.serviceUrl.NewContainerURL(s.container)
	if create {
		s.containerUrl.Create(ctx, azblob.Metadata{}, azblob.PublicAccessNone)
	}
}

func (s *blockBlobTestSuite) tearDownTestHelper(delete bool) {
	s.az.Stop()
	if delete {
		s.containerUrl.Delete(ctx, azblob.ContainerAccessConditions{})
	}
}

func (s *blockBlobTestSuite) cleanupTest() {
	s.tearDownTestHelper(true)
	log.Destroy()
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
	s.assert.EqualValues(AccessTiers["none"], s.az.stConfig.defaultTier)
	s.assert.EqualValues(0, s.az.stConfig.cancelListForSeconds)
	s.assert.EqualValues(3, s.az.stConfig.maxRetries)
	s.assert.EqualValues(3600, s.az.stConfig.maxTimeout)
	s.assert.EqualValues(1, s.az.stConfig.backoffTime)
	s.assert.EqualValues(3, s.az.stConfig.maxRetryDelay)
	s.assert.Empty(s.az.stConfig.proxyAddress)
}

func randomString(length int) string {
	rand.Seed(time.Now().UnixNano())
	b := make([]byte, length)
	rand.Read(b)
	return fmt.Sprintf("%x", b)[:length]
}

func generateContainerName() string {
	return "fuseutc" + randomString(8)
}

func generateDirectoryName() string {
	return "dir" + randomString(8)
}

func generateFileName() string {
	return "file" + randomString(8)
}

func (s *blockBlobTestSuite) TestListContainers() {
	defer s.cleanupTest()
	// Setup
	num := 10
	prefix := generateContainerName()
	for i := 0; i < num; i++ {
		s.serviceUrl.NewContainerURL(prefix+fmt.Sprint(i)).Create(ctx, nil, azblob.PublicAccessNone)
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

	// Cleanup
	r, _ := s.serviceUrl.ListContainersSegment(ctx, azblob.Marker{}, azblob.ListContainersSegmentOptions{Prefix: prefix})
	for _, c := range r.ContainerItems {
		s.serviceUrl.NewContainerURL(c.Name).Delete(ctx, azblob.ContainerAccessConditions{})
	}
}

// TODO : ListContainersHuge: Maybe this is overkill?

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
			dir := s.containerUrl.NewBlobURL(internal.TruncateDirName(path))
			props, err := dir.GetProperties(ctx, azblob.BlobAccessConditions{}, azblob.ClientProvidedKeyOptions{})
			s.assert.Nil(err)
			s.assert.NotNil(props)
			s.assert.NotEmpty(props.NewMetadata())
			s.assert.Contains(props.NewMetadata(), folderKey)
			s.assert.EqualValues("true", props.NewMetadata()[folderKey])
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
			dir := s.containerUrl.NewBlobURL(internal.TruncateDirName(path))
			_, err = dir.GetProperties(ctx, azblob.BlobAccessConditions{}, azblob.ClientProvidedKeyOptions{})
			s.assert.NotNil(err)
		})
	}
}

// Directory structure
// a/
//  a/c1/
//   a/c1/gc1
//	a/c2
// ab/
//  ab/c1
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
		_, err := s.containerUrl.NewBlobURL(p.Value.(string)).GetProperties(ctx, azblob.BlobAccessConditions{}, azblob.ClientProvidedKeyOptions{})
		s.assert.Nil(err)
	}
	for p := ab.Front(); p != nil; p = p.Next() {
		_, err := s.containerUrl.NewBlobURL(p.Value.(string)).GetProperties(ctx, azblob.BlobAccessConditions{}, azblob.ClientProvidedKeyOptions{})
		s.assert.Nil(err)
	}
	for p := ac.Front(); p != nil; p = p.Next() {
		_, err := s.containerUrl.NewBlobURL(p.Value.(string)).GetProperties(ctx, azblob.BlobAccessConditions{}, azblob.ClientProvidedKeyOptions{})
		s.assert.Nil(err)
	}
	return a, ab, ac
}

func (s *blockBlobTestSuite) TestDeleteDirHierarchy() {
	defer s.cleanupTest()
	// Setup
	base := generateDirectoryName()
	a, ab, ac := s.setupHierarchy(base)

	err := s.az.DeleteDir(internal.DeleteDirOptions{Name: base})

	s.assert.Nil(err)

	/// a paths should be deleted
	for p := a.Front(); p != nil; p = p.Next() {
		_, err = s.containerUrl.NewBlobURL(p.Value.(string)).GetProperties(ctx, azblob.BlobAccessConditions{}, azblob.ClientProvidedKeyOptions{})
		s.assert.NotNil(err)
	}
	ab.PushBackList(ac) // ab and ac paths should exist
	for p := ab.Front(); p != nil; p = p.Next() {
		_, err = s.containerUrl.NewBlobURL(p.Value.(string)).GetProperties(ctx, azblob.BlobAccessConditions{}, azblob.ClientProvidedKeyOptions{})
		s.assert.Nil(err)
	}
}

func (s *blockBlobTestSuite) TestDeleteSubDirPrefixPath() {
	defer s.cleanupTest()
	// Setup
	base := generateDirectoryName()
	a, ab, ac := s.setupHierarchy(base)

	s.az.storage.SetPrefixPath(base)

	err := s.az.DeleteDir(internal.DeleteDirOptions{Name: "c1"})
	s.assert.Nil(err)

	// a paths under c1 should be deleted
	for p := a.Front(); p != nil; p = p.Next() {
		path := p.Value.(string)
		_, err = s.containerUrl.NewBlobURL(path).GetProperties(ctx, azblob.BlobAccessConditions{}, azblob.ClientProvidedKeyOptions{})
		if strings.HasPrefix(path, base+"/c1") {
			s.assert.NotNil(err)
		} else {
			s.assert.Nil(err)
		}
	}
	ab.PushBackList(ac) // ab and ac paths should exist
	for p := ab.Front(); p != nil; p = p.Next() {
		_, err = s.containerUrl.NewBlobURL(p.Value.(string)).GetProperties(ctx, azblob.BlobAccessConditions{}, azblob.ClientProvidedKeyOptions{})
		s.assert.Nil(err)
	}
}

func (s *blockBlobTestSuite) TestDeleteDirError() {
	defer s.cleanupTest()
	// Setup
	name := generateDirectoryName()

	err := s.az.DeleteDir(internal.DeleteDirOptions{Name: name})

	s.assert.NotNil(err)
	s.assert.EqualValues(syscall.ENOENT, err)
	// Directory should not be in the account
	dir := s.containerUrl.NewBlobURL(name)
	_, err = dir.GetProperties(ctx, azblob.BlobAccessConditions{}, azblob.ClientProvidedKeyOptions{})
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
	dir := s.containerUrl.NewBlobURL(name)
	_, err := dir.GetProperties(ctx, azblob.BlobAccessConditions{}, azblob.ClientProvidedKeyOptions{})
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
			entries, err := s.az.ReadDir(internal.ReadDirOptions{Name: name})
			s.assert.Nil(err)
			s.assert.EqualValues(1, len(entries))
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
	s.assert.True(entries[0].IsMetadataRetrieved())
	s.assert.True(entries[0].IsModeDefault())
	// Check the file
	s.assert.EqualValues(base+"/c2", entries[1].Path)
	s.assert.EqualValues("c2", entries[1].Name)
	s.assert.False(entries[1].IsDir())
	s.assert.True(entries[1].IsMetadataRetrieved())
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
			entries, err := s.az.ReadDir(internal.ReadDirOptions{Name: ""})
			s.assert.Nil(err)
			s.assert.EqualValues(3, len(entries))
			// Check the base dir
			s.assert.EqualValues(base, entries[0].Path)
			s.assert.EqualValues(base, entries[0].Name)
			s.assert.True(entries[0].IsDir())
			s.assert.True(entries[0].IsMetadataRetrieved())
			s.assert.True(entries[0].IsModeDefault())
			// Check the baseb dir
			s.assert.EqualValues(base+"b", entries[1].Path)
			s.assert.EqualValues(base+"b", entries[1].Name)
			s.assert.True(entries[1].IsDir())
			s.assert.True(entries[1].IsMetadataRetrieved())
			s.assert.True(entries[1].IsModeDefault())
			// Check the basec file
			s.assert.EqualValues(base+"c", entries[2].Path)
			s.assert.EqualValues(base+"c", entries[2].Name)
			s.assert.False(entries[2].IsDir())
			s.assert.True(entries[2].IsMetadataRetrieved())
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
	s.assert.True(entries[0].IsMetadataRetrieved())
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
	s.assert.True(entries[0].IsMetadataRetrieved())
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
	dir := s.containerUrl.NewBlobURL(name)
	_, err = dir.GetProperties(ctx, azblob.BlobAccessConditions{}, azblob.ClientProvidedKeyOptions{})
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
			dir := s.containerUrl.NewBlobURL(internal.TruncateDirName(input.src))
			_, err = dir.GetProperties(ctx, azblob.BlobAccessConditions{}, azblob.ClientProvidedKeyOptions{})
			s.assert.NotNil(err)

			// Dst should be in the account
			dir = s.containerUrl.NewBlobURL(internal.TruncateDirName(input.dst))
			_, err = dir.GetProperties(ctx, azblob.BlobAccessConditions{}, azblob.ClientProvidedKeyOptions{})
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
		_, err = s.containerUrl.NewBlobURL(p.Value.(string)).GetProperties(ctx, azblob.BlobAccessConditions{}, azblob.ClientProvidedKeyOptions{})
		s.assert.NotNil(err)
	}
	abSrc.PushBackList(acSrc) // abSrc and acSrc paths should exist
	for p := abSrc.Front(); p != nil; p = p.Next() {
		_, err = s.containerUrl.NewBlobURL(p.Value.(string)).GetProperties(ctx, azblob.BlobAccessConditions{}, azblob.ClientProvidedKeyOptions{})
		s.assert.Nil(err)
	}
	// Destination
	// aDst paths should exist
	for p := aDst.Front(); p != nil; p = p.Next() {
		_, err = s.containerUrl.NewBlobURL(p.Value.(string)).GetProperties(ctx, azblob.BlobAccessConditions{}, azblob.ClientProvidedKeyOptions{})
		s.assert.Nil(err)
	}
	abDst.PushBackList(acDst) // abDst and acDst paths should not exist
	for p := abDst.Front(); p != nil; p = p.Next() {
		_, err = s.containerUrl.NewBlobURL(p.Value.(string)).GetProperties(ctx, azblob.BlobAccessConditions{}, azblob.ClientProvidedKeyOptions{})
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
		_, err = s.containerUrl.NewBlobURL(path).GetProperties(ctx, azblob.BlobAccessConditions{}, azblob.ClientProvidedKeyOptions{})
		if strings.HasPrefix(path, baseSrc+"/c1") {
			s.assert.NotNil(err)
		} else {
			s.assert.Nil(err)
		}
	}
	abSrc.PushBackList(acSrc) // abSrc and acSrc paths should exist
	for p := abSrc.Front(); p != nil; p = p.Next() {
		_, err = s.containerUrl.NewBlobURL(p.Value.(string)).GetProperties(ctx, azblob.BlobAccessConditions{}, azblob.ClientProvidedKeyOptions{})
		s.assert.Nil(err)
	}
	// Destination
	// aDst paths should exist -> aDst and aDst/gc1
	_, err = s.containerUrl.NewBlobURL(baseSrc+"/"+baseDst).GetProperties(ctx, azblob.BlobAccessConditions{}, azblob.ClientProvidedKeyOptions{})
	s.assert.Nil(err)
	_, err = s.containerUrl.NewBlobURL(baseSrc+"/"+baseDst+"/gc1").GetProperties(ctx, azblob.BlobAccessConditions{}, azblob.ClientProvidedKeyOptions{})
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
	dir := s.containerUrl.NewBlobURL(src)
	_, err = dir.GetProperties(ctx, azblob.BlobAccessConditions{}, azblob.ClientProvidedKeyOptions{})
	s.assert.NotNil(err)
	dir = s.containerUrl.NewBlobURL(dst)
	_, err = dir.GetProperties(ctx, azblob.BlobAccessConditions{}, azblob.ClientProvidedKeyOptions{})
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
	file := s.containerUrl.NewBlobURL(name)
	props, err := file.GetProperties(ctx, azblob.BlobAccessConditions{}, azblob.ClientProvidedKeyOptions{})
	s.assert.Nil(err)
	s.assert.NotNil(props)
	s.assert.Empty(props.NewMetadata())
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
	file := s.containerUrl.NewBlobURL(name)
	_, err = file.GetProperties(ctx, azblob.BlobAccessConditions{}, azblob.ClientProvidedKeyOptions{})
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
	file := s.containerUrl.NewBlobURL(name)
	_, err = file.GetProperties(ctx, azblob.BlobAccessConditions{}, azblob.ClientProvidedKeyOptions{})
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
	source := s.containerUrl.NewBlobURL(src)
	_, err = source.GetProperties(ctx, azblob.BlobAccessConditions{}, azblob.ClientProvidedKeyOptions{})
	s.assert.NotNil(err)
	// Dst should be in the account
	destination := s.containerUrl.NewBlobURL(dst)
	_, err = destination.GetProperties(ctx, azblob.BlobAccessConditions{}, azblob.ClientProvidedKeyOptions{})
	s.assert.Nil(err)
}

func (s *blockBlobTestSuite) TestRenameFileMetadataConservation() {
	defer s.cleanupTest()
	// Setup
	src := generateFileName()
	source := s.containerUrl.NewBlobURL(src)
	s.az.CreateFile(internal.CreateFileOptions{Name: src})
	// Add srcMeta to source
	srcMeta := make(azblob.Metadata)
	srcMeta["foo"] = "bar"
	source.SetMetadata(ctx, srcMeta, azblob.BlobAccessConditions{}, azblob.ClientProvidedKeyOptions{})
	dst := generateFileName()

	err := s.az.RenameFile(internal.RenameFileOptions{Src: src, Dst: dst})
	s.assert.Nil(err)

	// Src should not be in the account
	_, err = source.GetProperties(ctx, azblob.BlobAccessConditions{}, azblob.ClientProvidedKeyOptions{})
	s.assert.NotNil(err)
	// Dst should be in the account
	destination := s.containerUrl.NewBlobURL(dst)
	props, err := destination.GetProperties(ctx, azblob.BlobAccessConditions{}, azblob.ClientProvidedKeyOptions{})
	s.assert.Nil(err)
	// Dst should have metadata
	destMeta := props.NewMetadata()
	s.assert.Contains(destMeta, "foo")
	s.assert.EqualValues("bar", destMeta["foo"])
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
	source := s.containerUrl.NewBlobURL(src)
	_, err = source.GetProperties(ctx, azblob.BlobAccessConditions{}, azblob.ClientProvidedKeyOptions{})
	s.assert.NotNil(err)
	destination := s.containerUrl.NewBlobURL(dst)
	_, err = destination.GetProperties(ctx, azblob.BlobAccessConditions{}, azblob.ClientProvidedKeyOptions{})
	s.assert.NotNil(err)
}

func (s *blockBlobTestSuite) TestReadFile() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h, _ := s.az.CreateFile(internal.CreateFileOptions{Name: name})
	testData := "test data"
	data := []byte(testData)
	s.az.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data, FileOffsets: &common.BlockOffsetList{}})
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
	s.az.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data, FileOffsets: &common.BlockOffsetList{}})
	h, _ = s.az.OpenFile(internal.OpenFileOptions{Name: name})

	output := make([]byte, 5)
	len, err := s.az.ReadInBuffer(internal.ReadInBufferOptions{Handle: h, Offset: 0, Data: output})
	s.assert.Nil(err)
	s.assert.EqualValues(5, len)
	s.assert.EqualValues(testData[:5], output)
}

func (s *blockBlobTestSuite) TestReadInBufferLargeBuffer() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h, _ := s.az.CreateFile(internal.CreateFileOptions{Name: name})
	testData := "test data"
	data := []byte(testData)
	s.az.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data, FileOffsets: &common.BlockOffsetList{}})
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
	count, err := s.az.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data, FileOffsets: &common.BlockOffsetList{}})
	s.assert.Nil(err)
	s.assert.EqualValues(len(data), count)

	// Blob should have updated data
	file := s.containerUrl.NewBlobURL(name)
	resp, err := file.Download(ctx, 0, int64(len(data)), azblob.BlobAccessConditions{}, false, azblob.ClientProvidedKeyOptions{})
	s.assert.Nil(err)
	output, _ := ioutil.ReadAll(resp.Body(azblob.RetryReaderOptions{}))
	s.assert.EqualValues(testData, output)
}

func (s *blockBlobTestSuite) TestTruncateFileSmaller() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h, _ := s.az.CreateFile(internal.CreateFileOptions{Name: name})
	testData := "test data"
	data := []byte(testData)
	truncatedLength := 5
	s.az.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data, FileOffsets: &common.BlockOffsetList{}})

	err := s.az.TruncateFile(internal.TruncateFileOptions{Name: name, Size: int64(truncatedLength)})
	s.assert.Nil(err)

	// Blob should have updated data
	file := s.containerUrl.NewBlobURL(name)
	resp, err := file.Download(ctx, 0, int64(truncatedLength), azblob.BlobAccessConditions{}, false, azblob.ClientProvidedKeyOptions{})
	s.assert.Nil(err)
	s.assert.EqualValues(truncatedLength, resp.ContentLength())
	output, _ := ioutil.ReadAll(resp.Body(azblob.RetryReaderOptions{}))
	s.assert.EqualValues(testData[:truncatedLength], output)
}

func (s *blockBlobTestSuite) TestTruncateFileEqual() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h, _ := s.az.CreateFile(internal.CreateFileOptions{Name: name})
	testData := "test data"
	data := []byte(testData)
	truncatedLength := 9
	s.az.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data, FileOffsets: &common.BlockOffsetList{}})

	err := s.az.TruncateFile(internal.TruncateFileOptions{Name: name, Size: int64(truncatedLength)})
	s.assert.Nil(err)

	// Blob should have updated data
	file := s.containerUrl.NewBlobURL(name)
	resp, err := file.Download(ctx, 0, int64(truncatedLength), azblob.BlobAccessConditions{}, false, azblob.ClientProvidedKeyOptions{})
	s.assert.Nil(err)
	s.assert.EqualValues(truncatedLength, resp.ContentLength())
	output, _ := ioutil.ReadAll(resp.Body(azblob.RetryReaderOptions{}))
	s.assert.EqualValues(testData, output)
}

func (s *blockBlobTestSuite) TestTruncateFileBigger() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h, _ := s.az.CreateFile(internal.CreateFileOptions{Name: name})
	testData := "test data"
	data := []byte(testData)
	truncatedLength := 15
	s.az.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data, FileOffsets: &common.BlockOffsetList{}})

	err := s.az.TruncateFile(internal.TruncateFileOptions{Name: name, Size: int64(truncatedLength)})
	s.assert.Nil(err)

	// Blob should have updated data
	file := s.containerUrl.NewBlobURL(name)
	resp, err := file.Download(ctx, 0, int64(truncatedLength), azblob.BlobAccessConditions{}, false, azblob.ClientProvidedKeyOptions{})
	s.assert.Nil(err)
	s.assert.EqualValues(truncatedLength, resp.ContentLength())
	output, _ := ioutil.ReadAll(resp.Body(azblob.RetryReaderOptions{}))
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
	_, err := s.az.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data, FileOffsets: &common.BlockOffsetList{}})
	s.assert.Nil(err)
	f, _ := ioutil.TempFile("", name+".tmp")
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
	_, err := s.az.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data, FileOffsets: &common.BlockOffsetList{}})
	s.assert.Nil(err)
	f, _ := ioutil.TempFile("", name+".tmp")
	defer os.Remove(f.Name())
	newTestData := []byte("newdata")
	_, err = s.az.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 5, Data: newTestData, FileOffsets: &common.BlockOffsetList{}})
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

	_, err := s.az.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data, FileOffsets: &common.BlockOffsetList{}})
	s.assert.Nil(err)
	f, _ := ioutil.TempFile("", name+".tmp")
	defer os.Remove(f.Name())
	newTestData := []byte("newdata")
	_, err = s.az.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 5, Data: newTestData, FileOffsets: &common.BlockOffsetList{}})
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

	_, err := s.az.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data, FileOffsets: &common.BlockOffsetList{}})
	s.assert.Nil(err)
	f, _ := ioutil.TempFile("", name+".tmp")
	defer os.Remove(f.Name())
	newTestData := []byte("-newdata")
	_, err = s.az.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 9, Data: newTestData, FileOffsets: &common.BlockOffsetList{}})
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

	_, err := s.az.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data, FileOffsets: &common.BlockOffsetList{}})
	s.assert.Nil(err)
	f, _ := ioutil.TempFile("", name+".tmp")
	defer os.Remove(f.Name())
	newTestData := []byte("newdata")
	_, err = s.az.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 12, Data: newTestData, FileOffsets: &common.BlockOffsetList{}})
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
	_, err := uploadReaderAtToBlockBlob(ctx, bytes.NewReader(data), int64(len(data)), 9, s.containerUrl.NewBlockBlobURL(name), azblob.UploadToBlockBlobOptions{
		BlockSize: 8,
	})
	s.assert.Nil(err)
	f, _ := ioutil.TempFile("", name+".tmp")
	defer os.Remove(f.Name())
	newTestData := []byte("-newdata-newdata-newdata")
	_, err = s.az.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 9, Data: newTestData, FileOffsets: &common.BlockOffsetList{}})
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
	_, err := uploadReaderAtToBlockBlob(ctx, bytes.NewReader(data), int64(len(data)), 4, s.containerUrl.NewBlockBlobURL(name), azblob.UploadToBlockBlobOptions{
		BlockSize: 4,
	})
	s.assert.Nil(err)
	f, _ := ioutil.TempFile("", name+".tmp")
	defer os.Remove(f.Name())
	newTestData := []byte("cake")
	_, err = s.az.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 16, Data: newTestData, FileOffsets: &common.BlockOffsetList{}})
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
	_, err := uploadReaderAtToBlockBlob(ctx, bytes.NewReader(data), int64(len(data)), 4, s.containerUrl.NewBlockBlobURL(name), azblob.UploadToBlockBlobOptions{
		BlockSize: 4,
	})
	s.assert.Nil(err)
	f, _ := ioutil.TempFile("", name+".tmp")
	defer os.Remove(f.Name())
	newTestData := []byte("43211234cake")
	_, err = s.az.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 32, Data: newTestData, FileOffsets: &common.BlockOffsetList{}})
	s.assert.Nil(err)

	currentData := []byte("testdatates1dat1tes2dat2tes3dat343211234cake")
	dataLen := len(currentData)
	output := make([]byte, dataLen)

	err = s.az.CopyToFile(internal.CopyToFileOptions{Name: name, File: f})
	s.assert.Nil(err)

	f, _ = os.Open(f.Name())
	len, err := f.Read(output)
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
	_, err := uploadReaderAtToBlockBlob(ctx, bytes.NewReader(data), int64(len(data)), 4, s.containerUrl.NewBlockBlobURL(name), azblob.UploadToBlockBlobOptions{
		BlockSize: 4,
	})
	s.assert.Nil(err)
	f, _ := ioutil.TempFile("", name+".tmp")
	defer os.Remove(f.Name())
	newTestData := []byte("43211234cake")
	_, err = s.az.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: newTestData, FileOffsets: &common.BlockOffsetList{}})
	s.assert.Nil(err)

	currentData := []byte("43211234cakedat1tes2dat2tes3dat3tes4dat4")
	dataLen := len(currentData)
	output := make([]byte, dataLen)

	err = s.az.CopyToFile(internal.CopyToFileOptions{Name: name, File: f})
	s.assert.Nil(err)

	f, _ = os.Open(f.Name())
	len, err := f.Read(output)
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
	_, err := uploadReaderAtToBlockBlob(ctx, bytes.NewReader(data), int64(len(data)), 4, s.containerUrl.NewBlockBlobURL(name), azblob.UploadToBlockBlobOptions{
		BlockSize: 4,
	})
	s.assert.Nil(err)
	f, _ := ioutil.TempFile("", name+".tmp")
	defer os.Remove(f.Name())
	newTestData := []byte("43211234cake")
	_, err = s.az.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 45, Data: newTestData, FileOffsets: &common.BlockOffsetList{}})
	s.assert.Nil(err)

	currentData := []byte("testdatates1dat1tes2dat2tes3dat3tes4dat4\x00\x00\x00\x00\x0043211234cake")
	dataLen := len(currentData)
	output := make([]byte, dataLen)

	err = s.az.CopyToFile(internal.CopyToFileOptions{Name: name, File: f})
	s.assert.Nil(err)

	f, _ = os.Open(f.Name())
	len, err := f.Read(output)
	s.assert.EqualValues(dataLen, len)
	s.assert.EqualValues(currentData, output)
	f.Close()
}

func (s *blockBlobTestSuite) TestCopyToFileError() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	f, _ := ioutil.TempFile("", name+".tmp")
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
	f, _ := ioutil.TempFile(homeDir, name+".tmp")
	defer os.Remove(f.Name())
	f.Write(data)

	err := s.az.CopyFromFile(internal.CopyFromFileOptions{Name: name, File: f})

	s.assert.Nil(err)

	// Blob should have updated data
	file := s.containerUrl.NewBlobURL(name)
	resp, err := file.Download(ctx, 0, int64(len(data)), azblob.BlobAccessConditions{}, false, azblob.ClientProvidedKeyOptions{})
	s.assert.Nil(err)
	output, _ := ioutil.ReadAll(resp.Body(azblob.RetryReaderOptions{}))
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
	link := s.containerUrl.NewBlobURL(name)
	props, err := link.GetProperties(ctx, azblob.BlobAccessConditions{}, azblob.ClientProvidedKeyOptions{})
	s.assert.Nil(err)
	s.assert.NotNil(props)
	s.assert.NotEmpty(props.NewMetadata())
	s.assert.Contains(props.NewMetadata(), symlinkKey)
	s.assert.EqualValues("true", props.NewMetadata()[symlinkKey])
	resp, err := link.Download(ctx, 0, props.ContentLength(), azblob.BlobAccessConditions{}, false, azblob.ClientProvidedKeyOptions{})
	s.assert.Nil(err)
	data, _ := ioutil.ReadAll(resp.Body(azblob.RetryReaderOptions{}))
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
	// Setup
	name := generateDirectoryName()
	s.az.CreateDir(internal.CreateDirOptions{Name: name})

	props, err := s.az.GetAttr(internal.GetAttrOptions{Name: name})
	s.assert.Nil(err)
	s.assert.NotNil(props)
	s.assert.True(props.IsDir())
	s.assert.NotEmpty(props.Metadata)
	s.assert.Contains(props.Metadata, folderKey)
	s.assert.EqualValues("true", props.Metadata[folderKey])
}

func (s *blockBlobTestSuite) TestGetAttrFile() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	s.az.CreateFile(internal.CreateFileOptions{Name: name})

	props, err := s.az.GetAttr(internal.GetAttrOptions{Name: name})
	s.assert.Nil(err)
	s.assert.NotNil(props)
	s.assert.False(props.IsDir())
	s.assert.False(props.IsSymlink())
}

func (s *blockBlobTestSuite) TestGetAttrLink() {
	defer s.cleanupTest()
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
	s.assert.Contains(props.Metadata, symlinkKey)
	s.assert.EqualValues("true", props.Metadata[symlinkKey])
}

func (s *blockBlobTestSuite) TestGetAttrFileSize() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h, _ := s.az.CreateFile(internal.CreateFileOptions{Name: name})
	testData := "test data"
	data := []byte(testData)
	s.az.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data, FileOffsets: &common.BlockOffsetList{}})

	props, err := s.az.GetAttr(internal.GetAttrOptions{Name: name})
	s.assert.Nil(err)
	s.assert.NotNil(props)
	s.assert.False(props.IsDir())
	s.assert.False(props.IsSymlink())
	s.assert.EqualValues(len(testData), props.Size)
}

func (s *blockBlobTestSuite) TestGetAttrFileTime() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h, _ := s.az.CreateFile(internal.CreateFileOptions{Name: name})
	testData := "test data"
	data := []byte(testData)
	s.az.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data, FileOffsets: &common.BlockOffsetList{}})

	before, err := s.az.GetAttr(internal.GetAttrOptions{Name: name})
	s.assert.Nil(err)
	s.assert.NotNil(before.Mtime)

	time.Sleep(time.Second * 3) // Wait 3 seconds and then modify the file again

	s.az.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data, FileOffsets: &common.BlockOffsetList{}})

	after, err := s.az.GetAttr(internal.GetAttrOptions{Name: name})
	s.assert.Nil(err)
	s.assert.NotNil(after.Mtime)

	s.assert.True(after.Mtime.After(before.Mtime))
}

func (s *blockBlobTestSuite) TestGetAttrError() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()

	_, err := s.az.GetAttr(internal.GetAttrOptions{Name: name})
	s.assert.NotNil(err)
	s.assert.EqualValues(syscall.ENOENT, err)
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

func (s *blockBlobTestSuite) TestChown() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	s.az.CreateFile(internal.CreateFileOptions{Name: name})

	err := s.az.Chown(internal.ChownOptions{Name: name, Owner: 6, Group: 5})
	s.assert.NotNil(err)
	s.assert.EqualValues(syscall.ENOTSUP, err)
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

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestBlockBlob(t *testing.T) {
	suite.Run(t, new(blockBlobTestSuite))
}
