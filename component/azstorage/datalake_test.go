// +build !authtest
/*
    _____           _____   _____   ____          ______  _____  ------
   |     |  |      |     | |     | |     |     | |       |            |
   |     |  |      |     | |     | |     |     | |       |            |
   | --- |  |      |     | |-----| |---- |     | |-----| |-----  ------
   |     |  |      |     | |     | |     |     |       | |       |
   | ____|  |_____ | ____| | ____| |     |_____|  _____| |_____  |_____


   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2020-2021 Microsoft Corporation. All rights reserved.
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
	"time"

	"github.com/Azure/azure-storage-azcopy/v10/azbfs"
	"github.com/Azure/azure-storage-blob-go/azblob"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type datalakeTestSuite struct {
	suite.Suite
	assert       *assert.Assertions
	az           *AzStorage
	serviceUrl   azbfs.ServiceURL
	containerUrl azbfs.FileSystemURL
	config       string
	container    string
}

func (s *datalakeTestSuite) SetupTest() {
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

func (s *datalakeTestSuite) setupTestHelper(configuration string, container string, create bool) {
	if container == "" {
		container = generateContainerName()
	}
	s.container = container
	if configuration == "" {
		configuration = fmt.Sprintf("azstorage:\n  account-name: %s\n  endpoint: https://%s.dfs.core.windows.net/\n  type: adls\n  account-key: %s\n  mode: key\n  container: %s\n  fail-unsupported-op: true",
			storageTestConfigurationParameters.AdlsAccount, storageTestConfigurationParameters.AdlsAccount, storageTestConfigurationParameters.AdlsKey, s.container)
	}
	s.config = configuration

	s.assert = assert.New(s.T())

	s.az = newTestAzStorage(configuration)
	s.az.Start(ctx) // Note: Start->TestValidation will fail but it doesnt matter. We are creating the container a few lines below anyway.
	// We could create the container before but that requires rewriting the code to new up a service client.

	s.serviceUrl = s.az.storage.(*Datalake).Service // Grab the service client to do some validation
	s.containerUrl = s.serviceUrl.NewFileSystemURL(s.container)
	if create {
		s.containerUrl.Create(ctx)
	}
}

func (s *datalakeTestSuite) tearDownTestHelper(delete bool) {
	s.az.Stop()
	if delete {
		s.containerUrl.Delete(ctx)
	}
}

func (s *datalakeTestSuite) cleanupTest() {
	s.tearDownTestHelper(true)
	log.Destroy()
}

func (s *datalakeTestSuite) TestDefault() {
	defer s.cleanupTest()
	s.assert.Equal(storageTestConfigurationParameters.AdlsAccount, s.az.stConfig.authConfig.AccountName)
	s.assert.Equal(EAccountType.ADLS(), s.az.stConfig.authConfig.AccountType)
	s.assert.False(s.az.stConfig.authConfig.UseHTTP)
	s.assert.Equal(storageTestConfigurationParameters.AdlsKey, s.az.stConfig.authConfig.AccountKey)
	s.assert.Empty(s.az.stConfig.authConfig.SASKey)
	s.assert.Empty(s.az.stConfig.authConfig.ApplicationID)
	s.assert.Empty(s.az.stConfig.authConfig.ResourceID)
	s.assert.Empty(s.az.stConfig.authConfig.ActiveDirectoryEndpoint)
	s.assert.Empty(s.az.stConfig.authConfig.ClientSecret)
	s.assert.Empty(s.az.stConfig.authConfig.TenantID)
	s.assert.Empty(s.az.stConfig.authConfig.ClientID)
	s.assert.EqualValues("https://"+s.az.stConfig.authConfig.AccountName+".dfs.core.windows.net/", s.az.stConfig.authConfig.Endpoint)
	s.assert.Equal(EAuthType.KEY(), s.az.stConfig.authConfig.AuthMode)
	s.assert.Equal(s.container, s.az.stConfig.container)
	s.assert.Empty(s.az.stConfig.prefixPath)
	s.assert.EqualValues(16*1024*1024, s.az.stConfig.blockSize)
	s.assert.EqualValues(32, s.az.stConfig.maxConcurrency)
	s.assert.EqualValues(AccessTiers["none"], s.az.stConfig.defaultTier)
	s.assert.EqualValues(0, s.az.stConfig.cancelListForSeconds)
	s.assert.EqualValues(3, s.az.stConfig.maxRetries)
	s.assert.EqualValues(3600, s.az.stConfig.maxTimeout)
	s.assert.EqualValues(1, s.az.stConfig.backoffTime)
	s.assert.EqualValues(3, s.az.stConfig.maxRetryDelay)
	s.assert.Empty(s.az.stConfig.proxyAddress)
}

func (s *datalakeTestSuite) TestListContainers() {
	defer s.cleanupTest()
	// Setup
	num := 10
	prefix := generateContainerName()
	for i := 0; i < num; i++ {
		s.serviceUrl.NewFileSystemURL(prefix + fmt.Sprint(i)).Create(ctx)
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
	r, _ := s.serviceUrl.ListFilesystemsSegment(ctx, nil)
	for _, c := range r.Filesystems {
		if strings.HasPrefix(*c.Name, prefix) {
			s.serviceUrl.NewFileSystemURL(*c.Name).Delete(ctx)
		}
	}
}

// TODO : ListContainersHuge: Maybe this is overkill?

func (s *datalakeTestSuite) TestCreateDir() {
	defer s.cleanupTest()
	// Testing dir and dir/
	var paths = []string{generateDirectoryName(), generateDirectoryName() + "/"}
	for _, path := range paths {
		log.Debug(path)
		s.Run(path, func() {
			err := s.az.CreateDir(internal.CreateDirOptions{Name: path})

			s.assert.Nil(err)
			// Directory should be in the account
			dir := s.containerUrl.NewDirectoryURL(internal.TruncateDirName(path))
			_, err = dir.GetProperties(ctx)
			s.assert.Nil(err)
		})
	}
}

func (s *datalakeTestSuite) TestDeleteDir() {
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
			dir := s.containerUrl.NewDirectoryURL(internal.TruncateDirName(path))
			_, err = dir.GetProperties(ctx)
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

func (s *datalakeTestSuite) setupHierarchy(base string) (*list.List, *list.List, *list.List) {
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
		_, err := s.containerUrl.NewDirectoryURL(p.Value.(string)).GetProperties(ctx)
		s.assert.Nil(err)
	}
	for p := ab.Front(); p != nil; p = p.Next() {
		_, err := s.containerUrl.NewDirectoryURL(p.Value.(string)).GetProperties(ctx)
		s.assert.Nil(err)
	}
	for p := ac.Front(); p != nil; p = p.Next() {
		_, err := s.containerUrl.NewDirectoryURL(p.Value.(string)).GetProperties(ctx)
		s.assert.Nil(err)
	}
	return a, ab, ac
}

func (s *datalakeTestSuite) TestDeleteDirHierarchy() {
	defer s.cleanupTest()
	// Setup
	base := generateDirectoryName()
	a, ab, ac := s.setupHierarchy(base)

	err := s.az.DeleteDir(internal.DeleteDirOptions{Name: base})

	s.assert.Nil(err)

	// a paths should be deleted
	for p := a.Front(); p != nil; p = p.Next() {
		_, err = s.containerUrl.NewDirectoryURL(p.Value.(string)).GetProperties(ctx)
		s.assert.NotNil(err)
	}
	ab.PushBackList(ac) // ab and ac paths should exist
	for p := ab.Front(); p != nil; p = p.Next() {
		_, err = s.containerUrl.NewDirectoryURL(p.Value.(string)).GetProperties(ctx)
		s.assert.Nil(err)
	}
}

func (s *datalakeTestSuite) TestDeleteSubDirPrefixPath() {
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
		_, err = s.containerUrl.NewDirectoryURL(path).GetProperties(ctx)
		if strings.HasPrefix(path, base+"/c1") {
			s.assert.NotNil(err)
		} else {
			s.assert.Nil(err)
		}
	}
	ab.PushBackList(ac) // ab and ac paths should exist
	for p := ab.Front(); p != nil; p = p.Next() {
		_, err = s.containerUrl.NewDirectoryURL(p.Value.(string)).GetProperties(ctx)
		s.assert.Nil(err)
	}
}

func (s *datalakeTestSuite) TestDeleteDirError() {
	defer s.cleanupTest()
	// Setup
	name := generateDirectoryName()

	err := s.az.DeleteDir(internal.DeleteDirOptions{Name: name})

	s.assert.NotNil(err)
	s.assert.EqualValues(syscall.ENOENT, err)
	// Directory should not be in the account
	dir := s.containerUrl.NewDirectoryURL(name)
	_, err = dir.GetProperties(ctx)
	s.assert.NotNil(err)
}

func (s *datalakeTestSuite) TestIsDirEmpty() {
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

func (s *datalakeTestSuite) TestIsDirEmptyFalse() {
	defer s.cleanupTest()
	// Setup
	name := generateDirectoryName()
	s.az.CreateDir(internal.CreateDirOptions{Name: name})
	file := name + "/" + generateFileName()
	s.az.CreateFile(internal.CreateFileOptions{Name: file})

	empty := s.az.IsDirEmpty(internal.IsDirEmptyOptions{Name: name})

	s.assert.False(empty)
}

func (s *datalakeTestSuite) TestIsDirEmptyError() {
	defer s.cleanupTest()
	// Setup
	name := generateDirectoryName()

	empty := s.az.IsDirEmpty(internal.IsDirEmptyOptions{Name: name})

	s.assert.False(empty) // Note: See comment in BlockBlob.List. BlockBlob behaves differently from Datalake

	// Directory should not be in the account
	dir := s.containerUrl.NewDirectoryURL(name)
	_, err := dir.GetProperties(ctx)
	s.assert.NotNil(err)
}

func (s *datalakeTestSuite) TestReadDir() {
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

func (s *datalakeTestSuite) TestReadDirHierarchy() {
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
	s.assert.False(entries[0].IsMetadataRetrieved())
	s.assert.False(entries[0].IsModeDefault())
	// Check the file
	s.assert.EqualValues(base+"/c2", entries[1].Path)
	s.assert.EqualValues("c2", entries[1].Name)
	s.assert.False(entries[1].IsDir())
	s.assert.False(entries[1].IsMetadataRetrieved())
	s.assert.False(entries[1].IsModeDefault())
}

func (s *datalakeTestSuite) TestReadDirRoot() {
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
			s.assert.False(entries[0].IsMetadataRetrieved())
			s.assert.False(entries[0].IsModeDefault())
			// Check the baseb dir
			s.assert.EqualValues(base+"b", entries[1].Path)
			s.assert.EqualValues(base+"b", entries[1].Name)
			s.assert.True(entries[1].IsDir())
			s.assert.False(entries[1].IsMetadataRetrieved())
			s.assert.False(entries[1].IsModeDefault())
			// Check the basec file
			s.assert.EqualValues(base+"c", entries[2].Path)
			s.assert.EqualValues(base+"c", entries[2].Name)
			s.assert.False(entries[2].IsDir())
			s.assert.False(entries[2].IsMetadataRetrieved())
			s.assert.False(entries[2].IsModeDefault())
		})
	}
}

func (s *datalakeTestSuite) TestReadDirSubDir() {
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
	s.assert.False(entries[0].IsMetadataRetrieved())
	s.assert.False(entries[0].IsModeDefault())
}

func (s *datalakeTestSuite) TestReadDirSubDirPrefixPath() {
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
	s.assert.EqualValues(base+"/c1"+"/gc1", entries[0].Path)
	s.assert.EqualValues("gc1", entries[0].Name)
	s.assert.False(entries[0].IsDir())
	s.assert.False(entries[0].IsMetadataRetrieved())
	s.assert.False(entries[0].IsModeDefault())
}

func (s *datalakeTestSuite) TestReadDirError() {
	defer s.cleanupTest()
	// Setup
	name := generateDirectoryName()

	entries, err := s.az.ReadDir(internal.ReadDirOptions{Name: name})

	s.assert.NotNil(err) // Note: See comment in BlockBlob.List. BlockBlob behaves differently from Datalake
	s.assert.Empty(entries)
	// Directory should not be in the account
	dir := s.containerUrl.NewDirectoryURL(name)
	_, err = dir.GetProperties(ctx)
	s.assert.NotNil(err)
}

func (s *datalakeTestSuite) TestReadDirListBlocked() {
	defer s.cleanupTest()
	// Setup
	s.tearDownTestHelper(false) // Don't delete the generated container.

	listBlockedTime := 10
	config := fmt.Sprintf("azstorage:\n  account-name: %s\n  endpoint: https://%s.dfs.core.windows.net/\n  type: adls\n  account-key: %s\n  mode: key\n  container: %s\n  block-list-on-mount-sec: %d\n  fail-unsupported-op: true\n",
		storageTestConfigurationParameters.AdlsAccount, storageTestConfigurationParameters.AdlsAccount, storageTestConfigurationParameters.AdlsKey, s.container, listBlockedTime)
	s.setupTestHelper(config, s.container, true)

	name := generateDirectoryName()
	s.az.CreateDir(internal.CreateDirOptions{Name: name})
	childName := name + "/" + generateFileName()
	s.az.CreateFile(internal.CreateFileOptions{Name: childName})

	entries, err := s.az.ReadDir(internal.ReadDirOptions{Name: name})
	s.assert.Nil(err)
	s.assert.EqualValues(0, len(entries)) // Since we block the list, it will return an empty list.
}

func (s *datalakeTestSuite) TestRenameDir() {
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
			dir := s.containerUrl.NewDirectoryURL(internal.TruncateDirName(input.src))
			_, err = dir.GetProperties(ctx)
			s.assert.NotNil(err)

			// Dst should be in the account
			dir = s.containerUrl.NewDirectoryURL(internal.TruncateDirName(input.dst))
			_, err = dir.GetProperties(ctx)
			s.assert.Nil(err)
		})
	}

}

func (s *datalakeTestSuite) TestRenameDirHierarchy() {
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
		_, err = s.containerUrl.NewDirectoryURL(p.Value.(string)).GetProperties(ctx)
		s.assert.NotNil(err)
	}
	abSrc.PushBackList(acSrc) // abSrc and acSrc paths should exist
	for p := abSrc.Front(); p != nil; p = p.Next() {
		_, err = s.containerUrl.NewDirectoryURL(p.Value.(string)).GetProperties(ctx)
		s.assert.Nil(err)
	}
	// Destination
	// aDst paths should exist
	for p := aDst.Front(); p != nil; p = p.Next() {
		_, err = s.containerUrl.NewDirectoryURL(p.Value.(string)).GetProperties(ctx)
		s.assert.Nil(err)
	}
	abDst.PushBackList(acDst) // abDst and acDst paths should not exist
	for p := abDst.Front(); p != nil; p = p.Next() {
		_, err = s.containerUrl.NewDirectoryURL(p.Value.(string)).GetProperties(ctx)
		s.assert.NotNil(err)
	}
}

func (s *datalakeTestSuite) TestRenameDirSubDirPrefixPath() {
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
		_, err = s.containerUrl.NewDirectoryURL(path).GetProperties(ctx)
		if strings.HasPrefix(path, baseSrc+"/c1") {
			s.assert.NotNil(err)
		} else {
			s.assert.Nil(err)
		}
	}
	abSrc.PushBackList(acSrc) // abSrc and acSrc paths should exist
	for p := abSrc.Front(); p != nil; p = p.Next() {
		_, err = s.containerUrl.NewDirectoryURL(p.Value.(string)).GetProperties(ctx)
		s.assert.Nil(err)
	}
	// Destination
	// aDst paths should exist -> aDst and aDst/gc1
	_, err = s.containerUrl.NewDirectoryURL(baseSrc + "/" + baseDst).GetProperties(ctx)
	s.assert.Nil(err)
	_, err = s.containerUrl.NewDirectoryURL(baseSrc + "/" + baseDst + "/gc1").GetProperties(ctx)
	s.assert.Nil(err)
}

func (s *datalakeTestSuite) TestRenameDirError() {
	defer s.cleanupTest()
	// Setup
	src := generateDirectoryName()
	dst := generateDirectoryName()

	err := s.az.RenameDir(internal.RenameDirOptions{Src: src, Dst: dst})

	s.assert.NotNil(err)
	s.assert.EqualValues(syscall.ENOENT, err)
	// Neither directory should be in the account
	dir := s.containerUrl.NewDirectoryURL(src)
	_, err = dir.GetProperties(ctx)
	s.assert.NotNil(err)
	dir = s.containerUrl.NewDirectoryURL(dst)
	_, err = dir.GetProperties(ctx)
	s.assert.NotNil(err)
}

func (s *datalakeTestSuite) TestCreateFile() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()

	h, err := s.az.CreateFile(internal.CreateFileOptions{Name: name})

	s.assert.Nil(err)
	s.assert.NotNil(h)
	s.assert.EqualValues(name, h.Path)
	s.assert.EqualValues(0, h.Size)
	// File should be in the account
	file := s.containerUrl.NewDirectoryURL(name)
	props, err := file.GetProperties(ctx)
	s.assert.Nil(err)
	s.assert.NotNil(props)
	s.assert.Empty(props.XMsProperties())
}

func (s *datalakeTestSuite) TestOpenFile() {
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

func (s *datalakeTestSuite) TestOpenFileError() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()

	h, err := s.az.OpenFile(internal.OpenFileOptions{Name: name})
	s.assert.NotNil(err)
	s.assert.EqualValues(syscall.ENOENT, err)
	s.assert.Nil(h)
}

func (s *datalakeTestSuite) TestOpenFileSize() {
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

func (s *datalakeTestSuite) TestCloseFile() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h, _ := s.az.CreateFile(internal.CreateFileOptions{Name: name})

	// This method does nothing.
	err := s.az.CloseFile(internal.CloseFileOptions{Handle: h})
	s.assert.Nil(err)
}

func (s *datalakeTestSuite) TestCloseFileFakeHandle() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h := handlemap.NewHandle(name)

	// This method does nothing.
	err := s.az.CloseFile(internal.CloseFileOptions{Handle: h})
	s.assert.Nil(err)
}

func (s *datalakeTestSuite) TestDeleteFile() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	s.az.CreateFile(internal.CreateFileOptions{Name: name})

	err := s.az.DeleteFile(internal.DeleteFileOptions{Name: name})
	s.assert.Nil(err)

	// File should not be in the account
	file := s.containerUrl.NewDirectoryURL(name)
	_, err = file.GetProperties(ctx)
	s.assert.NotNil(err)
}

func (s *datalakeTestSuite) TestDeleteFileError() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()

	err := s.az.DeleteFile(internal.DeleteFileOptions{Name: name})
	s.assert.NotNil(err)
	s.assert.EqualValues(syscall.ENOENT, err)

	// File should not be in the account
	file := s.containerUrl.NewDirectoryURL(name)
	_, err = file.GetProperties(ctx)
	s.assert.NotNil(err)
}

func (s *datalakeTestSuite) TestRenameFile() {
	defer s.cleanupTest()
	// Setup
	src := generateFileName()
	s.az.CreateFile(internal.CreateFileOptions{Name: src})
	dst := generateFileName()

	err := s.az.RenameFile(internal.RenameFileOptions{Src: src, Dst: dst})
	s.assert.Nil(err)

	// Src should not be in the account
	source := s.containerUrl.NewDirectoryURL(src)
	_, err = source.GetProperties(ctx)
	s.assert.NotNil(err)
	// Dst should be in the account
	destination := s.containerUrl.NewDirectoryURL(dst)
	_, err = destination.GetProperties(ctx)
	s.assert.Nil(err)
}

func (s *datalakeTestSuite) TestRenameFileMetadataConservation() {
	defer s.cleanupTest()
	// Setup
	src := generateFileName()
	source := s.containerUrl.NewRootDirectoryURL().NewFileURL(src)
	s.az.CreateFile(internal.CreateFileOptions{Name: src})
	// Add srcMeta to source
	srcMeta := make(azblob.Metadata)
	srcMeta["foo"] = "bar"
	source.CreateWithOptions(ctx, azbfs.CreateFileOptions{Metadata: srcMeta}, azbfs.BlobFSAccessControl{})
	dst := generateFileName()

	err := s.az.RenameFile(internal.RenameFileOptions{Src: src, Dst: dst})
	s.assert.Nil(err)

	// Src should not be in the account
	_, err = source.GetProperties(ctx)
	s.assert.NotNil(err)
	// Dst should be in the account
	destination := s.containerUrl.NewRootDirectoryURL().NewFileURL(dst)
	props, err := destination.GetProperties(ctx)
	s.assert.Nil(err)
	// Dst should have metadata
	destMeta := newMetadata(props.XMsProperties())
	s.assert.Contains(destMeta, "foo")
	s.assert.EqualValues("bar", destMeta["foo"])
}

func (s *datalakeTestSuite) TestRenameFileError() {
	defer s.cleanupTest()
	// Setup
	src := generateFileName()
	dst := generateFileName()

	err := s.az.RenameFile(internal.RenameFileOptions{Src: src, Dst: dst})
	s.assert.NotNil(err)
	s.assert.EqualValues(syscall.ENOENT, err)

	// Src and destination should not be in the account
	source := s.containerUrl.NewDirectoryURL(src)
	_, err = source.GetProperties(ctx)
	s.assert.NotNil(err)
	destination := s.containerUrl.NewDirectoryURL(dst)
	_, err = destination.GetProperties(ctx)
	s.assert.NotNil(err)
}

func (s *datalakeTestSuite) TestReadFile() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h, _ := s.az.CreateFile(internal.CreateFileOptions{Name: name})
	testData := "test data"
	data := []byte(testData)
	s.az.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data})
	h, _ = s.az.OpenFile(internal.OpenFileOptions{Name: name})

	output, err := s.az.ReadFile(internal.ReadFileOptions{Handle: h})
	s.assert.Nil(err)
	s.assert.EqualValues(testData, output)
}

func (s *datalakeTestSuite) TestReadFileError() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h := handlemap.NewHandle(name)

	_, err := s.az.ReadFile(internal.ReadFileOptions{Handle: h})
	s.assert.NotNil(err)
	s.assert.EqualValues(syscall.ENOENT, err)
}

func (s *datalakeTestSuite) TestReadInBuffer() {
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

func (s *datalakeTestSuite) TestReadInBufferLargeBuffer() {
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

func (s *datalakeTestSuite) TestReadInBufferEmpty() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h, _ := s.az.CreateFile(internal.CreateFileOptions{Name: name})

	output := make([]byte, 10)
	len, err := s.az.ReadInBuffer(internal.ReadInBufferOptions{Handle: h, Offset: 0, Data: output})
	s.assert.Nil(err)
	s.assert.EqualValues(0, len)
}

func (s *datalakeTestSuite) TestReadInBufferBadRange() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h := handlemap.NewHandle(name)
	h.Size = 10

	_, err := s.az.ReadInBuffer(internal.ReadInBufferOptions{Handle: h, Offset: 20, Data: make([]byte, 2)})
	s.assert.NotNil(err)
	s.assert.EqualValues(syscall.ERANGE, err)
}

func (s *datalakeTestSuite) TestReadInBufferError() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h := handlemap.NewHandle(name)
	h.Size = 10

	_, err := s.az.ReadInBuffer(internal.ReadInBufferOptions{Handle: h, Offset: 0, Data: make([]byte, 2)})
	s.assert.NotNil(err)
	s.assert.EqualValues(syscall.ENOENT, err)
}

func (s *datalakeTestSuite) TestWriteFile() {
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
	file := s.containerUrl.NewRootDirectoryURL().NewFileURL(name)
	resp, err := file.Download(ctx, 0, int64(len(data)))
	s.assert.Nil(err)
	output, _ := ioutil.ReadAll(resp.Body(azbfs.RetryReaderOptions{}))
	s.assert.EqualValues(testData, output)
}

func (s *datalakeTestSuite) TestTruncateFileSmaller() {
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
	file := s.containerUrl.NewRootDirectoryURL().NewFileURL(name)
	resp, err := file.Download(ctx, 0, int64(truncatedLength))
	s.assert.Nil(err)
	s.assert.EqualValues(truncatedLength, resp.ContentLength())
	output, _ := ioutil.ReadAll(resp.Body(azbfs.RetryReaderOptions{}))
	s.assert.EqualValues(testData[:truncatedLength], output)
}

func (s *datalakeTestSuite) TestTruncateFileEqual() {
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
	file := s.containerUrl.NewRootDirectoryURL().NewFileURL(name)
	resp, err := file.Download(ctx, 0, int64(truncatedLength))
	s.assert.Nil(err)
	s.assert.EqualValues(truncatedLength, resp.ContentLength())
	output, _ := ioutil.ReadAll(resp.Body(azbfs.RetryReaderOptions{}))
	s.assert.EqualValues(testData, output)
}

func (s *datalakeTestSuite) TestTruncateFileBigger() {
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
	file := s.containerUrl.NewRootDirectoryURL().NewFileURL(name)
	resp, err := file.Download(ctx, 0, int64(truncatedLength))
	s.assert.Nil(err)
	s.assert.EqualValues(truncatedLength, resp.ContentLength())
	output, _ := ioutil.ReadAll(resp.Body(azbfs.RetryReaderOptions{}))
	s.assert.EqualValues(testData, output[:len(data)])
}

func (s *datalakeTestSuite) TestTruncateFileError() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()

	err := s.az.TruncateFile(internal.TruncateFileOptions{Name: name})
	s.assert.NotNil(err)
	s.assert.EqualValues(syscall.ENOENT, err)
}

func (s *datalakeTestSuite) TestCopyToFile() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	h, _ := s.az.CreateFile(internal.CreateFileOptions{Name: name})
	testData := "test data"
	data := []byte(testData)
	dataLen := len(data)
	s.az.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data})
	f, _ := ioutil.TempFile("", name+".tmp")
	defer os.Remove(f.Name())

	err := s.az.CopyToFile(internal.CopyToFileOptions{Name: name, File: f})
	s.assert.Nil(err)

	output := make([]byte, len(data))
	f, _ = os.Open(f.Name())
	len, err := f.Read(output)
	s.assert.Nil(err)
	s.assert.EqualValues(dataLen, len)
	s.assert.EqualValues(testData, output)
	f.Close()
}

func (s *datalakeTestSuite) TestCopyToFileError() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	f, _ := ioutil.TempFile("", name+".tmp")
	defer os.Remove(f.Name())

	err := s.az.CopyToFile(internal.CopyToFileOptions{Name: name, File: f})
	s.assert.NotNil(err)
}

func (s *datalakeTestSuite) TestCopyFromFile() {
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
	file := s.containerUrl.NewRootDirectoryURL().NewFileURL(name)
	resp, err := file.Download(ctx, 0, int64(len(data)))
	s.assert.Nil(err)
	output, _ := ioutil.ReadAll(resp.Body(azbfs.RetryReaderOptions{}))
	s.assert.EqualValues(testData, output)
}

func (s *datalakeTestSuite) TestCreateLink() {
	defer s.cleanupTest()
	// Setup
	target := generateFileName()
	s.az.CreateFile(internal.CreateFileOptions{Name: target})
	name := generateFileName()

	err := s.az.CreateLink(internal.CreateLinkOptions{Name: name, Target: target})
	s.assert.Nil(err)

	// Link should be in the account
	link := s.containerUrl.NewRootDirectoryURL().NewFileURL(name)
	props, err := link.GetProperties(ctx)
	s.assert.Nil(err)
	s.assert.NotNil(props)
	metadata := newMetadata(props.XMsProperties())
	s.assert.NotEmpty(metadata)
	s.assert.Contains(metadata, "Is_symlink")
	s.assert.EqualValues("true", metadata["Is_symlink"])
	resp, err := link.Download(ctx, 0, props.ContentLength())
	s.assert.Nil(err)
	data, _ := ioutil.ReadAll(resp.Body(azbfs.RetryReaderOptions{}))
	s.assert.EqualValues(target, data)
}

func (s *datalakeTestSuite) TestReadLink() {
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

func (s *datalakeTestSuite) TestReadLinkError() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()

	_, err := s.az.ReadLink(internal.ReadLinkOptions{Name: name})
	s.assert.NotNil(err)
	s.assert.EqualValues(syscall.ENOENT, err)
}

func (s *datalakeTestSuite) TestGetAttrDir() {
	defer s.cleanupTest()
	// Setup
	name := generateDirectoryName()
	s.az.CreateDir(internal.CreateDirOptions{Name: name})

	props, err := s.az.GetAttr(internal.GetAttrOptions{Name: name})
	s.assert.Nil(err)
	s.assert.NotNil(props)
	s.assert.True(props.IsDir())
}

func (s *datalakeTestSuite) TestGetAttrFile() {
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

func (s *datalakeTestSuite) TestGetAttrLink() {
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
	s.assert.Contains(props.Metadata, "Is_symlink")
	s.assert.EqualValues("true", props.Metadata["Is_symlink"])
}

func (s *datalakeTestSuite) TestGetAttrFileSize() {
	defer s.cleanupTest()
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
}

func (s *datalakeTestSuite) TestGetAttrFileTime() {
	defer s.cleanupTest()
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
	time.Sleep(time.Second * 1)

	after, err := s.az.GetAttr(internal.GetAttrOptions{Name: name})
	s.assert.Nil(err)
	s.assert.NotNil(after.Mtime)

	s.assert.True(after.Mtime.After(before.Mtime))
}

func (s *datalakeTestSuite) TestGetAttrError() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()

	_, err := s.az.GetAttr(internal.GetAttrOptions{Name: name})
	s.assert.NotNil(err)
	s.assert.EqualValues(syscall.ENOENT, err)
}

func (s *datalakeTestSuite) TestChmod() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	s.az.CreateFile(internal.CreateFileOptions{Name: name})

	err := s.az.Chmod(internal.ChmodOptions{Name: name, Mode: 0666})
	s.assert.Nil(err)

	// File's ACL info should have changed
	file := s.containerUrl.NewRootDirectoryURL().NewFileURL(name)
	acl, err := file.GetAccessControl(ctx)
	s.assert.Nil(err)
	s.assert.EqualValues("user::rw-,group::rw-,other::rw-", acl.ACL)
}

func (s *datalakeTestSuite) TestChmodError() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()

	err := s.az.Chmod(internal.ChmodOptions{Name: name, Mode: 0666})
	s.assert.NotNil(err)
	s.assert.EqualValues(syscall.ENOENT, err)
}

// If support for chown or chmod are ever added to blob, add tests for error cases and modify the following tests.
func (s *datalakeTestSuite) TestChown() {
	defer s.cleanupTest()
	// Setup
	name := generateFileName()
	s.az.CreateFile(internal.CreateFileOptions{Name: name})

	err := s.az.Chown(internal.ChownOptions{Name: name, Owner: 6, Group: 5})
	s.assert.NotNil(err)
	s.assert.EqualValues(syscall.ENOTSUP, err)
}

// func (s *datalakeTestSuite) TestRAGRS() {
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

// 	config := fmt.Sprintf("azstorage:\n  account-name: %s\n  type: adls\n  account-key: %s\n  mode: key\n  container: %s\n  endpoint: https://%s-secondary.dfs.core.windows.net\n",
// 		storageTestConfigurationParameters.AdlsAccount, storageTestConfigurationParameters.AdlsKey, s.container, storageTestConfigurationParameters.AdlsAccount)
// 	s.setupTestHelper(config, s.container, false) // Don't create a new container

// 	h, _ = s.az.OpenFile(internal.OpenFileOptions{Name: name})
// 	output, err := s.az.ReadFile(internal.ReadFileOptions{Handle: h})
// 	s.assert.Nil(err)
// 	s.assert.EqualValues(testData, output)
// 	s.az.CloseFile(internal.CloseFileOptions{Handle: h})
// }

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestDatalake(t *testing.T) {
	suite.Run(t, new(datalakeTestSuite))
}
