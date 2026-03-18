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

package cmd

import (
	"strings"
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

type osArgs struct {
	input  string
	output string
}

func (suite *rootCmdSuite) SetupTest() {
	suite.assert = assert.New(suite.T())
	err := log.SetDefaultLogger("silent", common.LogConfig{Level: common.ELogLevel.LOG_DEBUG()})
	if err != nil {
		panic("Unable to set silent logger as default.")
	}
}

func (suite *rootCmdSuite) cleanupTest() {
	rootCmd.SetOut(nil)
	rootCmd.SetErr(nil)
	rootCmd.SetArgs(nil)
}

func (suite *rootCmdSuite) TestNoOptions() {
	defer suite.cleanupTest()
	out, err := executeCommandC(rootCmd, "")
	suite.assert.Contains(out, "missing command options")
	suite.assert.Error(err)
}

func (suite *rootCmdSuite) TestNoOptionsNoVersionCheck() {
	defer suite.cleanupTest()
	out, err := executeCommandC(rootCmd, "--disable-version-check")
	suite.assert.Contains(out, "missing command options")
	suite.assert.Error(err)
}

// TestCheckVersionExistsInvalidURL verifies that a completely invalid URL
// returns false rather than panicking.
func (suite *rootCmdSuite) TestCheckVersionExistsInvalidURL() {
	defer suite.cleanupTest()
	found := checkVersionExists("://bad-url")
	suite.assert.False(found)
}

// TestNoSecurityWarnings verifies that the current build version has
// no security-warning file on the benchmarks branch.
func (suite *rootCmdSuite) TestNoSecurityWarnings() {
	defer suite.cleanupTest()
	warningsUrl := common.GitHubReleaseBaseURL + "/securitywarnings/" + common.Blobfuse2Version
	found := checkVersionExists(warningsUrl)
	suite.assert.False(found)
}

// TestSecurityWarnings verifies that version 1.1.1 has a security-warning
// file on the benchmarks branch (live HTTP call).
func (suite *rootCmdSuite) TestSecurityWarnings() {
	defer suite.cleanupTest()
	warningsUrl := common.GitHubReleaseBaseURL + "/securitywarnings/" + "1.1.1"
	found := checkVersionExists(warningsUrl)
	suite.assert.True(found)
}

// TestBlockedVersion verifies that version 1.1.1 has a blocked-version
// file on the benchmarks branch (live HTTP call).
func (suite *rootCmdSuite) TestBlockedVersion() {
	defer suite.cleanupTest()
	blockedUrl := common.GitHubReleaseBaseURL + "/blockedversions/" + "1.1.1"
	isBlocked := checkVersionExists(blockedUrl)
	suite.assert.True(isBlocked)
}

// TestNonBlockedVersion verifies that the current build version is NOT in the
// blocked-versions list on the benchmarks branch.
func (suite *rootCmdSuite) TestNonBlockedVersion() {
	defer suite.cleanupTest()
	blockedUrl := common.GitHubReleaseBaseURL + "/blockedversions/" + common.Blobfuse2Version
	found := checkVersionExists(blockedUrl)
	suite.assert.False(found)
}

// TODO: uncomment this after release
// TestLatestVersionExists verifies that the file release/latest/{Blobfuse2Version} is
// present on the benchmarks branch (it is the current GA latest).
// func (suite *rootCmdSuite) TestLatestVersionExists() {
// 	defer suite.cleanupTest()
// 	latestUrl := common.GitHubReleaseBaseURL + "/latest/" + common.Blobfuse2Version
// 	found := checkVersionExists(latestUrl)
// 	suite.assert.True(found)
// }

// TestLatestVersionNotExists verifies that an unknown/old version does NOT
// have a file under release/latest/.
func (suite *rootCmdSuite) TestLatestVersionNotExists() {
	defer suite.cleanupTest()
	latestUrl := common.GitHubReleaseBaseURL + "/latest/1.0.0"
	found := checkVersionExists(latestUrl)
	suite.assert.False(found)
}

func getDummyVersion() string {
	return "1.0.0"
}

// TestDetectNewVersionCurrentOlder sets the current version to a dummy old
// value (1.0.0) so that release/latest/1.0.0 returns 404, which triggers the
// "new version available" message.
func (suite *rootCmdSuite) TestDetectNewVersionCurrentOlder() {
	defer suite.cleanupTest()
	savedVersion := common.Blobfuse2Version
	common.Blobfuse2Version = getDummyVersion()
	defer func() { common.Blobfuse2Version = savedVersion }()

	msg := <-beginDetectNewVersion()
	suite.assert.NotEmpty(msg)
	suite.assert.Contains(msg, "A new version of Blobfuse2 is available")
}

// TestDetectNewVersionCurrentLatest sets the current version to the actual
// latest on the benchmarks branch so that release/latest/{Blobfuse2Version}
// exists and no upgrade message is produced.
// func (suite *rootCmdSuite) TestDetectNewVersionCurrentLatest() {
// 	defer suite.cleanupTest()
// 	common.Blobfuse2Version = common.Blobfuse2Version_()
// 	msg := <-beginDetectNewVersion()
// 	suite.assert.Nil(msg)
// }

func (suite *rootCmdSuite) TestParseArgs() {
	defer suite.cleanupTest()
	var inputs = []osArgs{
		{input: "mount abc", output: "mount abc"},
		{input: "mount abc --config-file=./config.yaml", output: "mount abc --config-file=./config.yaml"},
		{input: "help", output: "help"},
		{input: "--help", output: "--help"},
		{input: "version", output: "version"},
		{input: "--version", output: "--version"},
		{input: "version --check=true", output: "version --check=true"},
		{input: "mount abc --config-file=./config.yaml -o ro", output: "mount abc --config-file=./config.yaml -o ro"},
		{input: "abc", output: "mount abc"},
		{input: "-o", output: ""},
		{input: "", output: ""},

		{input: "/home/mntdir -o rw,--config-file=config.yaml,dev,suid", output: "mount /home/mntdir -o rw,dev,suid --config-file=config.yaml"},
		{input: "/home/mntdir -o --config-file=config.yaml,rw,dev,suid", output: "mount /home/mntdir -o rw,dev,suid --config-file=config.yaml"},
		{input: "/home/mntdir -o --config-file=config.yaml,rw", output: "mount /home/mntdir -o rw --config-file=config.yaml"},
		{input: "/home/mntdir -o rw,--config-file=config.yaml,dev,suid -o allow_other", output: "mount /home/mntdir -o rw,dev,suid --config-file=config.yaml -o allow_other"},
		{input: "/home/mntdir -o rw,--config-file=config.yaml,dev,suid -o allow_other,--adls=true", output: "mount /home/mntdir -o rw,dev,suid --config-file=config.yaml -o allow_other --adls=true"},
		{input: "/home/mntdir -o --config-file=config.yaml", output: "mount /home/mntdir --config-file=config.yaml"},
		{input: "/home/mntdir -o", output: "mount /home/mntdir"},
		{input: "mount /home/mntdir -o --config-file=config.yaml,rw", output: "mount /home/mntdir -o rw --config-file=config.yaml"},
	}
	for _, i := range inputs {
		o := parseArgs(strings.Split("blobfuse2 "+i.input, " "))
		suite.assert.Equal(i.output, strings.Join(o, " "))
	}
}

func TestRootCmd(t *testing.T) {
	suite.Run(t, new(rootCmdSuite))
}
