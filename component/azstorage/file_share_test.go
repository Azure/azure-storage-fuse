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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
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

func (s *fileTestSuite) tearDownTestHelper(delete bool) { // pass in 2nd param for Delete()?
	s.az.Stop()
	if delete {
		// s.shareUrl.Delete(ctx)
	}
}

func (s *fileTestSuite) cleanupTest() {
	s.tearDownTestHelper(true)
	log.Destroy()
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
