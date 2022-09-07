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

package cmd

import (
	"testing"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type rootCmdSuite struct {
	suite.Suite
	assert *assert.Assertions
}

func (suite *rootCmdSuite) SetupTest() {
	suite.assert = assert.New(suite.T())
	err := log.SetDefaultLogger("silent", common.LogConfig{Level: common.ELogLevel.LOG_DEBUG()})
	if err != nil {
		panic("Unable to set silent logger as default.")
	}
}

func (suite *rootCmdSuite) cleanupTest() {
	resetCLIFlags(*generateConfigCmd)
}

func (suite *rootCmdSuite) TestNoOptions() {
	defer suite.cleanupTest()
	out, err := executeCommandC(rootCmd, "")
	suite.assert.Contains(out, "missing command options")
	suite.assert.NotNil(err)
}

func (suite *rootCmdSuite) TestNoOptionsNoVersionCheck() {
	defer suite.cleanupTest()
	out, err := executeCommandC(rootCmd, "--disable-version-check")
	suite.assert.Contains(out, "missing command options")
	suite.assert.NotNil(err)
}

func (suite *rootCmdSuite) TestNoMountPath() {
	defer suite.cleanupTest()
	out, err := executeCommandC(rootCmd, "mount")
	suite.assert.Contains(out, "accepts 1 arg(s), received 0")
	suite.assert.NotNil(err)
}

func (suite *rootCmdSuite) TestCheckVersionExistsInvalidURL() {
	found := checkVersionExists("abcd")
	suite.assert.False(found)
}

func (suite *rootCmdSuite) TestNoSecurityWarnings() {
	warningsUrl := common.Blobfuse2ListContainerURL + "/securitywarnings/" + common.Blobfuse2Version
	found := checkVersionExists(warningsUrl)
	suite.assert.False(found)
}

func (suite *rootCmdSuite) TestGetRemoteVersionInvalidURL() {
	out, err := getRemoteVersion("abcd")
	suite.assert.Empty(out)
	suite.assert.NotNil(err)
}

func (suite *rootCmdSuite) TestGetRemoteVersionInvalidContainer() {
	latestVersionUrl := common.Blobfuse2ListContainerURL + "?restype=container&comp=list&prefix=latest1/"
	out, err := getRemoteVersion(latestVersionUrl)
	suite.assert.Empty(out)
	suite.assert.NotNil(err)
	suite.assert.Contains(err.Error(), "unable to get latest version")
}

func getDummyVersion() string {
	return "1.0.0"
}

func (suite *rootCmdSuite) TestGetRemoteVersionValidContainer() {
	latestVersionUrl := common.Blobfuse2ListContainerURL + "?restype=container&comp=list&prefix=latest/"
	out, err := getRemoteVersion(latestVersionUrl)
	suite.assert.NotEmpty(out)
	suite.assert.Nil(err)
}

func (suite *rootCmdSuite) TestGetRemoteVersionCurrentOlder() {
	common.Blobfuse2Version = getDummyVersion()
	msg := <-beginDetectNewVersion()
	suite.assert.NotEmpty(msg)
	suite.assert.Contains(msg, "A new version of Blobfuse2 is available")
}

func (suite *rootCmdSuite) TestGetRemoteVersionCurrentSame() {
	common.Blobfuse2Version = common.Blobfuse2Version_()
	msg := <-beginDetectNewVersion()
	suite.assert.Nil(msg)
}

func TestRootCmd(t *testing.T) {
	suite.Run(t, new(rootCmdSuite))
}
