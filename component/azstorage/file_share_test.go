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
	"blobfuse2/internal/handlemap"
	"container/list"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"syscall"
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

// TODO: testinvalidrangesize()?

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
	// Hierarchy looks as follows, a = base
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
		if err != nil {

			fileName, dirPath := getFileAndDirFromPath(p.Value.(string))
			_, err := s.shareUrl.NewDirectoryURL(dirPath).NewFileURL(fileName).GetProperties(ctx)
			s.assert.Nil(err)
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
		_, direrr := s.shareUrl.NewDirectoryURL(p.Value.(string)).GetProperties(ctx)

		fileName, dirPath := getFileAndDirFromPath(p.Value.(string))
		_, fileerr := s.shareUrl.NewDirectoryURL(dirPath).NewFileURL(fileName).GetProperties(ctx)

		s.assert.NotNil(direrr)
		s.assert.NotNil(fileerr)
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

func (s *fileTestSuite) TestDeleteSubDirPrefixPath() {
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

		// 4 cases: nonexistent file & directory, existing file & directory
		_, direrr := s.shareUrl.NewDirectoryURL(path).GetProperties(ctx)

		if direrr != nil {
			fileName, dirPath := getFileAndDirFromPath(path)
			_, fileerr := s.shareUrl.NewDirectoryURL(dirPath).NewFileURL(fileName).GetProperties(ctx)
			if fileerr == nil { // existing file
				if strings.HasPrefix(path, base+"/c1") {
					s.assert.NotNil(fileerr)
				} else {
					s.assert.Nil(fileerr)
				}
				break
			}

			if strings.HasPrefix(path, base+"/c1") { // nonexistent file and dir
				s.assert.NotNil(direrr)
				s.assert.NotNil(fileerr)
			} else {
				s.assert.Nil(direrr)
				s.assert.Nil(fileerr)
			}
		} else { // existing dir
			if strings.HasPrefix(path, base+"/c1") {
				s.assert.NotNil(direrr)
			} else {
				s.assert.Nil(direrr)
			}
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

func (s *fileTestSuite) TestDeleteDirError() {
	defer s.cleanupTest()
	// Setup
	name := generateDirectoryName()

	err := s.az.DeleteDir(internal.DeleteDirOptions{Name: name})
	s.assert.NotNil(err)
	s.assert.EqualValues(syscall.ENOENT, storeFileErrToErr(err))

	// Directory should not be in the account
	dir := s.shareUrl.NewDirectoryURL(name)
	_, err = dir.GetProperties(ctx)
	s.assert.NotNil(err)
}

func (s *fileTestSuite) TestIsDirEmpty() {
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

func (s *fileTestSuite) TestIsDirEmptyFalse() {
	defer s.cleanupTest()
	// Setup
	name := generateDirectoryName()
	s.az.CreateDir(internal.CreateDirOptions{Name: name})

	file := name + "/" + generateFileName()
	s.az.CreateFile(internal.CreateFileOptions{Name: file})

	empty := s.az.IsDirEmpty(internal.IsDirEmptyOptions{Name: name})

	s.assert.False(empty)
}

func (s *fileTestSuite) TestIsDirEmptyError() {
	defer s.cleanupTest()
	// Setup
	name := generateDirectoryName()

	empty := s.az.IsDirEmpty(internal.IsDirEmptyOptions{Name: name})
	s.assert.False(empty) // Note: FileShare fails for nonexistent directory.
	// FileShare behaves differently from BlockBlob (See comment in BlockBlob.List).

	// Directory should not be in the account
	dir := s.shareUrl.NewDirectoryURL(name)
	_, err := dir.GetProperties(ctx)
	s.assert.NotNil(err)
}

func (s *fileTestSuite) TestReadDir() {
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

func (s *fileTestSuite) TestReadDirHierarchy() {
	defer s.cleanupTest()
	// Setup
	base := generateDirectoryName()
	s.setupHierarchy(base)

	// TODO: test metadata retrieval once SDK is updated (in this method and others below)

	// ReadDir only reads the first level of the hierarchy
	entries, err := s.az.ReadDir(internal.ReadDirOptions{Name: base})
	s.assert.Nil(err)
	s.assert.EqualValues(2, len(entries))
	// Check the file
	s.assert.EqualValues(base+"/c2", entries[0].Path)
	s.assert.EqualValues("c2", entries[0].Name)
	s.assert.False(entries[0].IsDir())
	//s.assert.True(entries[0].IsMetadataRetrieved())
	s.assert.True(entries[0].IsModeDefault())
	// Check the dir
	s.assert.EqualValues(base+"/c1", entries[1].Path)
	s.assert.EqualValues("c1", entries[1].Name)
	s.assert.True(entries[1].IsDir())
	// s.assert.True(entries[1].IsMetadataRetrieved())
	s.assert.True(entries[1].IsModeDefault())
}

func (s *fileTestSuite) TestReadDirRoot() {
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
			s.assert.EqualValues(base, entries[1].Path)
			s.assert.EqualValues(base, entries[1].Name)
			s.assert.True(entries[1].IsDir())
			// s.assert.True(entries[1].IsMetadataRetrieved())
			s.assert.True(entries[1].IsModeDefault())
			// Check the baseb dir
			s.assert.EqualValues(base+"b", entries[2].Path)
			s.assert.EqualValues(base+"b", entries[2].Name)
			s.assert.True(entries[2].IsDir())
			// s.assert.True(entries[2].IsMetadataRetrieved())
			s.assert.True(entries[2].IsModeDefault())
			// Check the basec file
			s.assert.EqualValues(base+"c", entries[0].Path)
			s.assert.EqualValues(base+"c", entries[0].Name)
			s.assert.False(entries[0].IsDir())
			// s.assert.True(entries[0].IsMetadataRetrieved())
			s.assert.True(entries[0].IsModeDefault())
		})
	}
}

func (s *fileTestSuite) TestReadDirSubDir() {
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
	// s.assert.True(entries[0].IsMetadataRetrieved())
	s.assert.True(entries[0].IsModeDefault())
}

func (s *fileTestSuite) TestReadDirSubDirPrefixPath() {
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
	// s.assert.True(entries[0].IsMetadataRetrieved())
	s.assert.True(entries[0].IsModeDefault())
}

func (s *fileTestSuite) TestReadDirError() {
	defer s.cleanupTest()
	// Setup
	name := generateDirectoryName()

	entries, err := s.az.ReadDir(internal.ReadDirOptions{Name: name})

	s.assert.NotNil(err) // Note: FileShare fails for nonexistent directory.
	// FileShare behaves differently from BlockBlob (See comment in BlockBlob.List).
	s.assert.Empty(entries)
	// Directory should not be in the account
	dir := s.shareUrl.NewDirectoryURL(name)
	_, err = dir.GetProperties(ctx)
	s.assert.NotNil(err)
}

func (s *fileTestSuite) TestReadDirListBlocked() {
	defer s.cleanupTest()
	// Setup
	s.tearDownTestHelper(false) // Don't delete the generated container.

	listBlockedTime := 10
	config := fmt.Sprintf("azstorage:\n  account-name: %s\n  endpoint: https://%s.file.core.windows.net/\n  type: file\n  account-key: %s\n  mode: key\n  container: %s\n  block-list-on-mount-sec: %d\n  fail-unsupported-op: true\n",
		storageTestConfigurationParameters.FileAccount, storageTestConfigurationParameters.FileAccount, storageTestConfigurationParameters.FileKey, s.container, listBlockedTime)
	s.setupTestHelper(config, s.container, true)

	name := generateDirectoryName()
	s.az.CreateDir(internal.CreateDirOptions{Name: name})
	childName := name + "/" + generateFileName()
	s.az.CreateFile(internal.CreateFileOptions{Name: childName})

	entries, err := s.az.ReadDir(internal.ReadDirOptions{Name: name})
	s.assert.Nil(err)
	s.assert.EqualValues(0, len(entries)) // Since we block the list, it will return an empty list.
}

func (s *fileTestSuite) TestRenameDir() {
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
			dir := s.shareUrl.NewDirectoryURL(internal.TruncateDirName(input.src))
			_, err = dir.GetProperties(ctx)
			s.assert.NotNil(err)

			// Dst should be in the account
			dir = s.shareUrl.NewDirectoryURL(internal.TruncateDirName(input.dst))
			_, err = dir.GetProperties(ctx)
			s.assert.Nil(err)
		})
	}

}

func (s *fileTestSuite) TestRenameDirHierarchy() {
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
		_, direrr := s.shareUrl.NewDirectoryURL(p.Value.(string)).GetProperties(ctx)

		fileName, dirPath := getFileAndDirFromPath(p.Value.(string))
		_, fileerr := s.shareUrl.NewDirectoryURL(dirPath).NewFileURL(fileName).GetProperties(ctx)

		s.assert.NotNil(direrr)
		s.assert.NotNil(fileerr)
	}
	abSrc.PushBackList(acSrc) // abSrc and acSrc paths should exist
	for p := abSrc.Front(); p != nil; p = p.Next() {
		_, direrr := s.shareUrl.NewDirectoryURL(p.Value.(string)).GetProperties(ctx)
		if direrr != nil {
			fileName, dirPath := getFileAndDirFromPath(p.Value.(string))
			_, fileerr := s.shareUrl.NewDirectoryURL(dirPath).NewFileURL(fileName).GetProperties(ctx)

			if fileerr != nil { // nonexistent file and dir
				s.assert.NotNil(fileerr)
				s.assert.NotNil(direrr)
			} else { // existing file
				s.assert.Nil(fileerr)
				s.assert.NotNil(direrr)
			}
		} else { // existing dir
			s.assert.Nil(direrr)
		}
	}
	// Destination
	// aDst paths should exist
	for p := aDst.Front(); p != nil; p = p.Next() {
		_, direrr := s.shareUrl.NewDirectoryURL(p.Value.(string)).GetProperties(ctx)
		if direrr != nil {
			fileName, dirPath := getFileAndDirFromPath(p.Value.(string))
			_, fileerr := s.shareUrl.NewDirectoryURL(dirPath).NewFileURL(fileName).GetProperties(ctx)

			if fileerr != nil { // nonexistent file and dir
				s.assert.NotNil(fileerr)
				s.assert.NotNil(direrr)
			} else { // existing file
				s.assert.Nil(fileerr)
				s.assert.NotNil(direrr)
			}
		} else { // existing dir
			s.assert.Nil(direrr)
		}
	}
	abDst.PushBackList(acDst) // abDst and acDst paths should not exist
	for p := abDst.Front(); p != nil; p = p.Next() {
		_, direrr := s.shareUrl.NewDirectoryURL(p.Value.(string)).GetProperties(ctx)

		fileName, dirPath := getFileAndDirFromPath(p.Value.(string))
		_, fileerr := s.shareUrl.NewDirectoryURL(dirPath).NewFileURL(fileName).GetProperties(ctx)

		s.assert.NotNil(direrr)
		s.assert.NotNil(fileerr)
	}
}

func (s *fileTestSuite) TestRenameDirSubDirPrefixPath() {
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
		_, direrr := s.shareUrl.NewDirectoryURL(path).GetProperties(ctx)

		if direrr != nil {
			fileName, dirPath := getFileAndDirFromPath(path)
			_, fileerr := s.shareUrl.NewDirectoryURL(dirPath).NewFileURL(fileName).GetProperties(ctx)
			if fileerr == nil { // existing file
				if strings.HasPrefix(path, baseDst+"/c1") {
					s.assert.NotNil(fileerr)
				} else {
					s.assert.Nil(fileerr)
				}
				break
			}
			// nonexistent file and dir
			if strings.HasPrefix(path, baseSrc+"/c1") {
				s.assert.NotNil(direrr)
				s.assert.NotNil(fileerr)
			} else {
				s.assert.Nil(direrr)
				s.assert.Nil(fileerr)
			}
		} else { // existing dir
			if strings.HasPrefix(path, baseSrc+"/c1") {
				s.assert.NotNil(direrr)
			} else {
				s.assert.Nil(direrr)
			}
		}

		if strings.HasPrefix(path, baseSrc+"/c1") { // nonexistent dir
			s.assert.NotNil(direrr)
		} else { // existing dir
			s.assert.Nil(direrr)
		}
	}

	abSrc.PushBackList(acSrc) // abSrc and acSrc paths should exist
	for p := abSrc.Front(); p != nil; p = p.Next() {
		_, err = s.shareUrl.NewDirectoryURL(p.Value.(string)).GetProperties(ctx)

		if err != nil {
			fileName, dirPath := getFileAndDirFromPath(p.Value.(string))
			_, err := s.shareUrl.NewDirectoryURL(dirPath).NewFileURL(fileName).GetProperties(ctx)
			s.assert.Nil(err)
		} else {
			s.assert.Nil(err)
		}
	}
	// Destination
	// aDst paths should exist -> aDst and aDst/gc1
	_, err = s.shareUrl.NewDirectoryURL(baseSrc + "/" + baseDst).GetProperties(ctx)
	s.assert.Nil(err)
	fileName, dirPath := getFileAndDirFromPath(baseSrc + "/" + baseDst + "/gc1")
	_, err = s.shareUrl.NewDirectoryURL(dirPath).NewFileURL(fileName).GetProperties(ctx)
	s.assert.Nil(err)
}

func (s *fileTestSuite) TestRenameDirError() {
	defer s.cleanupTest()
	// Setup
	src := generateDirectoryName()
	dst := generateDirectoryName()

	err := s.az.RenameDir(internal.RenameDirOptions{Src: src, Dst: dst})
	s.assert.NotNil(err)
	s.assert.EqualValues(syscall.ENOENT, storeFileErrToErr(err))

	// Neither directory should be in the account
	dir := s.shareUrl.NewDirectoryURL(dst)
	_, err = dir.GetProperties(ctx)
	s.assert.NotNil(err)

	dir = s.shareUrl.NewDirectoryURL(src)
	_, err = dir.GetProperties(ctx)
	s.assert.NotNil(err)

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

func (s *fileTestSuite) TestOpenFile() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	s.az.CreateFile(internal.CreateFileOptions{Name: name})

	h, err := s.az.OpenFile(internal.OpenFileOptions{Name: name})
	s.assert.Nil(err)
	s.assert.NotNil(h)
	s.assert.EqualValues(name, h.Path)
	// s.assert.EqualValues(0, h.Size)
}

func (s *fileTestSuite) TestOpenFileError() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()

	h, err := s.az.OpenFile(internal.OpenFileOptions{Name: name})
	s.assert.NotNil(err)
	s.assert.EqualValues(syscall.ENOENT, err)
	s.assert.Nil(h)
}

func (s *fileTestSuite) TestOpenFileSize() {
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

func (s *fileTestSuite) TestCloseFile() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h, _ := s.az.CreateFile(internal.CreateFileOptions{Name: name})

	// This method does nothing.
	err := s.az.CloseFile(internal.CloseFileOptions{Handle: h})
	s.assert.Nil(err)
}

func (s *fileTestSuite) TestCloseFileFakeHandle() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h := handlemap.NewHandle(name)

	// This method does nothing.
	err := s.az.CloseFile(internal.CloseFileOptions{Handle: h})
	s.assert.Nil(err)
}

func (s *fileTestSuite) TestDeleteFile() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	s.az.CreateFile(internal.CreateFileOptions{Name: name})

	err := s.az.DeleteFile(internal.DeleteFileOptions{Name: name})
	s.assert.Nil(err)

	// File should not be in the account
	fileName, dirPath := getFileAndDirFromPath(name)
	file := s.shareUrl.NewDirectoryURL(dirPath).NewFileURL(fileName)
	_, err = file.GetProperties(ctx)
	s.assert.NotNil(err)
}

func (s *fileTestSuite) TestDeleteFileError() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()

	err := s.az.DeleteFile(internal.DeleteFileOptions{Name: name})
	s.assert.NotNil(err)
	s.assert.EqualValues(syscall.ENOENT, err)

	// File should not be in the account
	fileName, dirPath := getFileAndDirFromPath(name)
	file := s.shareUrl.NewDirectoryURL(dirPath).NewFileURL(fileName)
	_, err = file.GetProperties(ctx)
	s.assert.NotNil(err)
}

func (s *fileTestSuite) TestRenameFile() {
	defer s.cleanupTest()
	// Setup
	src := generateFileName()
	s.az.CreateFile(internal.CreateFileOptions{Name: src})
	dst := generateFileName()

	err := s.az.RenameFile(internal.RenameFileOptions{Src: src, Dst: dst})
	s.assert.Nil(err)

	// Src should not be in the account
	fileName, dirPath := getFileAndDirFromPath(src)
	source := s.shareUrl.NewDirectoryURL(dirPath).NewFileURL(fileName)
	_, err = source.GetProperties(ctx)
	s.assert.NotNil(err)
	// Dst should be in the account
	fileName, dirPath = getFileAndDirFromPath(dst)
	destination := s.shareUrl.NewDirectoryURL(dirPath).NewFileURL(fileName)
	_, err = destination.GetProperties(ctx)
	s.assert.Nil(err)
}

func (s *fileTestSuite) TestRenameFileMetadataConservation() {
	defer s.cleanupTest()
	// Setup
	src := generateFileName()
	fileName, dirPath := getFileAndDirFromPath(src)
	source := s.shareUrl.NewDirectoryURL(dirPath).NewFileURL(fileName)
	s.az.CreateFile(internal.CreateFileOptions{Name: src})

	// Add srcMeta to source
	srcMeta := make(azfile.Metadata)
	srcMeta["foo"] = "bar"
	source.SetMetadata(ctx, srcMeta)

	dst := generateFileName()
	err := s.az.RenameFile(internal.RenameFileOptions{Src: src, Dst: dst})
	s.assert.Nil(err)

	// Src should not be in the account
	_, err = source.GetProperties(ctx)
	s.assert.NotNil(err)
	// Dst should be in the account
	fileName, dirPath = getFileAndDirFromPath(dst)
	destination := s.shareUrl.NewDirectoryURL(dirPath).NewFileURL(fileName)
	props, err := destination.GetProperties(ctx)
	s.assert.Nil(err)
	// Dst should have metadata
	destMeta := props.NewMetadata()
	s.assert.Contains(destMeta, "foo")
	s.assert.EqualValues("bar", destMeta["foo"])
}

func (s *fileTestSuite) TestRenameFileError() {
	defer s.cleanupTest()
	// Setup
	src := generateFileName()
	dst := generateFileName()

	err := s.az.RenameFile(internal.RenameFileOptions{Src: src, Dst: dst})
	s.assert.NotNil(err)
	s.assert.EqualValues(syscall.ENOENT, err)

	// Src and destination should not be in the account
	fileName, dirPath := getFileAndDirFromPath(src)
	source := s.shareUrl.NewDirectoryURL(dirPath).NewFileURL(fileName)
	_, err = source.GetProperties(ctx)
	s.assert.NotNil(err)
	fileName, dirPath = getFileAndDirFromPath(dst)
	destination := s.shareUrl.NewDirectoryURL(dirPath).NewFileURL(fileName)
	_, err = destination.GetProperties(ctx)
	s.assert.NotNil(err)
}

// TODO: fix
func (s *fileTestSuite) TestReadFile() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h, _ := s.az.CreateFile(internal.CreateFileOptions{Name: name})
	testData := "test data"
	data := []byte(testData)
	print(data)
	s.az.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data})
	h, _ = s.az.OpenFile(internal.OpenFileOptions{Name: name})

	output, err := s.az.ReadFile(internal.ReadFileOptions{Handle: h})
	s.assert.Nil(err)
	s.assert.EqualValues(testData, output)
}

func (s *fileTestSuite) TestReadFileError() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h := handlemap.NewHandle(name)

	_, err := s.az.ReadFile(internal.ReadFileOptions{Handle: h})
	s.assert.NotNil(err)
	s.assert.EqualValues(syscall.ENOENT, err)
}

func (s *fileTestSuite) TestReadInBuffer() {
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

func (s *fileTestSuite) TestReadInBufferLargeBuffer() {
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

func (s *fileTestSuite) TestReadInBufferEmpty() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h, _ := s.az.CreateFile(internal.CreateFileOptions{Name: name})

	output := make([]byte, 10)
	len, err := s.az.ReadInBuffer(internal.ReadInBufferOptions{Handle: h, Offset: 0, Data: output})
	s.assert.Nil(err)
	s.assert.EqualValues(0, len)
}

func (s *fileTestSuite) TestReadInBufferBadRange() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h := handlemap.NewHandle(name)
	h.Size = 10

	_, err := s.az.ReadInBuffer(internal.ReadInBufferOptions{Handle: h, Offset: 20, Data: make([]byte, 2)})
	s.assert.NotNil(err)
	s.assert.EqualValues(syscall.ERANGE, err)
}

func TestFileShare(t *testing.T) {
	suite.Run(t, new(fileTestSuite))
}
