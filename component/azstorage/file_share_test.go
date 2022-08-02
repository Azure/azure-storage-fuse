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
	"blobfuse2/common/log"
	"blobfuse2/internal"
	"container/list"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"github.com/Azure/azure-storage-file-go/azfile"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type fileTestSuite struct {
	suite.Suite
	assert     *assert.Assertions
	az         *AzStorage
	serviceUrl azfile.ServiceURL
	shareUrl   azfile.ShareURL
	config     string
	container  string
}

func (s *fileTestSuite) SetupTest() {
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

func (s *fileTestSuite) setupTestHelper(configuration string, container string, create bool) {
	if container == "" {
		container = generateContainerName()
	}
	s.container = container
	if configuration == "" {
		configuration = fmt.Sprintf("azstorage:\n  account-name: %s\n  endpoint: https://%s.file.core.windows.net/\n  type: file\n  account-key: %s\n  mode: key\n  container: %s\n  fail-unsupported-op: true",
			storageTestConfigurationParameters.FileAccount, storageTestConfigurationParameters.FileAccount, storageTestConfigurationParameters.FileKey, s.container)
	}
	s.config = configuration

	s.assert = assert.New(s.T())

	s.az, _ = newTestAzStorage(configuration)
	s.az.Start(ctx) // Note: Start->TestValidation will fail but it doesn't matter. We are creating the container a few lines below anyway.
	// We could create the container before but that requires rewriting the code to new up a service client.

	s.serviceUrl = s.az.storage.(*FileShare).Service // Grab the service client to do some validation
	s.shareUrl = s.serviceUrl.NewShareURL(s.container)
	if create {
		s.shareUrl.Create(ctx, azfile.Metadata{}, 0)
	}
}

func (s *fileTestSuite) tearDownTestHelper(delete bool) {
	s.az.Stop()
	if delete {
		s.shareUrl.Delete(ctx, azfile.DeleteSnapshotsOptionNone)
	}
}

func (s *fileTestSuite) cleanupTest() {
	s.tearDownTestHelper(true)
	log.Destroy()
}

// others that block_blob_test.go has but this doesn't
// these don't directly test a method in file_share.go

func (s *fileTestSuite) TestDefault() {
	defer s.cleanupTest()
	s.assert.Equal(storageTestConfigurationParameters.FileAccount, s.az.stConfig.authConfig.AccountName)
	s.assert.Equal(EAccountType.FILE(), s.az.stConfig.authConfig.AccountType)
	s.assert.False(s.az.stConfig.authConfig.UseHTTP)
	s.assert.Equal(storageTestConfigurationParameters.FileKey, s.az.stConfig.authConfig.AccountKey)
	s.assert.Empty(s.az.stConfig.authConfig.SASKey)
	s.assert.Empty(s.az.stConfig.authConfig.ApplicationID)
	s.assert.Empty(s.az.stConfig.authConfig.ResourceID)
	s.assert.Empty(s.az.stConfig.authConfig.ActiveDirectoryEndpoint)
	s.assert.Empty(s.az.stConfig.authConfig.ClientSecret)
	s.assert.Empty(s.az.stConfig.authConfig.TenantID)
	s.assert.Empty(s.az.stConfig.authConfig.ClientID)
	s.assert.EqualValues("https://"+s.az.stConfig.authConfig.AccountName+".file.core.windows.net/", s.az.stConfig.authConfig.Endpoint)
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

func (s *fileTestSuite) TestInvalidRangeSize() {
	defer s.cleanupTest()
	configuration := fmt.Sprintf("azstorage:\n  account-name: %s\n  endpoint: https://%s.file.core.windows.net/\n  type: block\n  block-size-mb: 5\n account-key: %s\n  mode: key\n  container: %s\n  fail-unsupported-op: true",
		storageTestConfigurationParameters.FileAccount, storageTestConfigurationParameters.FileAccount, storageTestConfigurationParameters.FileKey, s.container)
	_, err := newTestAzStorage(configuration)
	s.assert.NotNil(err)
}

func (s *fileTestSuite) TestListShares() {
	defer s.cleanupTest()
	// Setup
	num := 10
	prefix := generateContainerName()
	for i := 0; i < num; i++ {
		c := s.serviceUrl.NewShareURL(prefix + fmt.Sprint(i))
		c.Create(ctx, nil, 0)
		defer c.Delete(ctx, azfile.DeleteSnapshotsOptionNone)
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

func (s *fileTestSuite) TestCreateDir() {
	defer s.cleanupTest()
	// Testing dir and dir/
	var paths = []string{generateDirectoryName(), generateDirectoryName() + "/"}
	for _, path := range paths {
		log.Debug(path)
		s.Run(path, func() {
			err := s.az.CreateDir(internal.CreateDirOptions{Name: path})

			s.assert.Nil(err)
			// Directory should be in the account
			dir := s.shareUrl.NewDirectoryURL(internal.TruncateDirName(path))

			props, err := dir.GetProperties(ctx)
			s.assert.Nil(err)
			s.assert.NotNil(props)
			s.assert.NotEmpty(props.NewMetadata())
			s.assert.Contains(props.NewMetadata(), folderKey)
			s.assert.EqualValues("true", props.NewMetadata()[folderKey])
		})
	}
}

func (s *fileTestSuite) TestDeleteDir() {
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
			dir := s.shareUrl.NewDirectoryURL(internal.TruncateDirName(path))
			_, err = dir.GetProperties(ctx)
			s.assert.NotNil(err)
		})
	}
}

func (s *fileTestSuite) setupHierarchy(base string) (*list.List, *list.List, *list.List) {
	// Hierarchy looks as follows
	// a/
	//  a/c1/
	//   a/c1/gc1
	//	a/c2
	// ab/
	//  ab/c1
	// ac
	err := s.az.CreateDir(internal.CreateDirOptions{Name: base})
	s.assert.Nil(err)
	c1 := base + "/c1"
	err = s.az.CreateDir(internal.CreateDirOptions{Name: c1})
	s.assert.Nil(err)
	gc1 := c1 + "/gc1"
	_, err = s.az.CreateFile(internal.CreateFileOptions{Name: gc1})
	s.assert.Nil(err)
	c2 := base + "/c2"
	_, err = s.az.CreateFile(internal.CreateFileOptions{Name: c2})
	s.assert.Nil(err)
	abPath := base + "b"
	err = s.az.CreateDir(internal.CreateDirOptions{Name: abPath})
	s.assert.Nil(err)
	abc1 := abPath + "/c1"
	_, err = s.az.CreateFile(internal.CreateFileOptions{Name: abc1})
	s.assert.Nil(err)
	acPath := base + "c"
	_, err = s.az.CreateFile(internal.CreateFileOptions{Name: acPath})
	s.assert.Nil(err)

	a, ab, ac := generateNestedDirectory(base)

	// Validate the paths were setup correctly and all paths exist
	for p := a.Front(); p != nil; p = p.Next() {
		_, err := s.shareUrl.NewDirectoryURL(p.Value.(string)).GetProperties(ctx)
		tmp := p.Value.(string)
		print(tmp)
		if err != nil {

			fileName, dirPath := getFileAndDirFromPath(p.Value.(string))
			_, err := s.shareUrl.NewDirectoryURL(dirPath).NewFileURL(fileName).GetProperties(ctx)
			s.assert.Nil(err) // RESOURCE NOT FOUND FOR FILES?
		} else {
			s.assert.Nil(err)
		}
	}
	for p := ab.Front(); p != nil; p = p.Next() {
		_, err := s.shareUrl.NewDirectoryURL(p.Value.(string)).GetProperties(ctx)
		if err != nil {
			fileName, dirPath := getFileAndDirFromPath(p.Value.(string))
			_, err := s.shareUrl.NewDirectoryURL(dirPath).NewFileURL(fileName).GetProperties(ctx)
			s.assert.Nil(err)
		} else {
			s.assert.Nil(err)
		}
	}
	for p := ac.Front(); p != nil; p = p.Next() {
		_, err := s.shareUrl.NewDirectoryURL(p.Value.(string)).GetProperties(ctx)
		if err != nil {
			fileName, dirPath := getFileAndDirFromPath(p.Value.(string))
			_, err := s.shareUrl.NewDirectoryURL(dirPath).NewFileURL(fileName).GetProperties(ctx)
			s.assert.Nil(err)
		} else {
			s.assert.Nil(err)
		}
	}
	return a, ab, ac
}

func (s *fileTestSuite) TestDeleteDirHierarchy() {
	defer s.cleanupTest()
	// Setup
	base := generateDirectoryName()
	a, ab, ac := s.setupHierarchy(base)

	err := s.az.DeleteDir(internal.DeleteDirOptions{Name: base})

	s.assert.Nil(err)

	// a paths should be deleted
	for p := a.Front(); p != nil; p = p.Next() {
		_, err = s.shareUrl.NewDirectoryURL(p.Value.(string)).GetProperties(ctx)

		if err != nil {
			fileName, dirPath := getFileAndDirFromPath(p.Value.(string))
			_, err := s.shareUrl.NewDirectoryURL(dirPath).NewFileURL(fileName).GetProperties(ctx)
			s.assert.Nil(err)
		} else {
			s.assert.Nil(err)
		}
	}

	ab.PushBackList(ac) // ab and ac paths should exist
	for p := ab.Front(); p != nil; p = p.Next() {
		_, err = s.shareUrl.NewDirectoryURL(p.Value.(string)).GetProperties(ctx)

		if err != nil {
			fileName, dirPath := getFileAndDirFromPath(p.Value.(string))
			_, err := s.shareUrl.NewDirectoryURL(dirPath).NewFileURL(fileName).GetProperties(ctx)
			s.assert.Nil(err)
		} else {
			s.assert.Nil(err)
		}
	}
}

func (s *fileTestSuite) TestCreateFile() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()

	h, err := s.az.CreateFile(internal.CreateFileOptions{Name: name})

	s.assert.Nil(err)
	s.assert.NotNil(h)
	s.assert.EqualValues(name, h.Path)
	s.assert.EqualValues(0, h.Size)
	// File should be in the account
	file := s.shareUrl.NewRootDirectoryURL().NewFileURL(name)
	props, err := file.GetProperties(ctx)
	s.assert.Nil(err)
	s.assert.NotNil(props)
	s.assert.Empty(props.NewMetadata())
}

func TestFileShare(t *testing.T) {
	suite.Run(t, new(fileTestSuite))
}
