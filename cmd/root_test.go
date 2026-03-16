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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

// knownSecurityWarnings lists versions that have security warnings in the release directory.
var knownSecurityWarnings = map[string]bool{
	"1.1.1":           true,
	"2.2.0":           true,
	"2.2.1":           true,
	"2.3.0":           true,
	"2.3.0~preview.1": true,
	"2.3.1~preview.1": true,
}

// knownBlockedVersions lists versions that are blocked in the release directory.
var knownBlockedVersions = map[string]bool{
	"1.1.1": true,
	"2.3.0": true,
}

// newMockGitHubServer creates an httptest server that simulates the GitHub API
// endpoints used by checkVersionExists and getGitHubLatestRemoteVersion.
func newMockGitHubServer(latestTag string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		// Simulate /releases/latest
		if path == "/releases/latest" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{
				"tag_name": latestTag,
			})
			return
		}

		// Simulate /contents/release/securitywarnings/<version>
		if strings.HasPrefix(path, "/contents/release/securitywarnings/") {
			version := strings.TrimPrefix(path, "/contents/release/securitywarnings/")
			if knownSecurityWarnings[version] {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"name":"` + version + `"}`))
				return
			}
			w.WriteHeader(http.StatusNotFound)
			return
		}

		// Simulate /contents/release/blockedversions/<version>
		if strings.HasPrefix(path, "/contents/release/blockedversions/") {
			version := strings.TrimPrefix(path, "/contents/release/blockedversions/")
			if knownBlockedVersions[version] {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"name":"` + version + `"}`))
				return
			}
			w.WriteHeader(http.StatusNotFound)
			return
		}

		// Anything else: 404
		w.WriteHeader(http.StatusNotFound)
	}))
}

type rootCmdSuite struct {
	suite.Suite
	assert *assert.Assertions
	server *httptest.Server

	// originals saved for restore
	origReleaseURL  string
	origContentsURL string
	origVersion     string
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

	// Save original values
	suite.origReleaseURL = common.GitHubLatestReleaseURL
	suite.origContentsURL = common.GitHubReleaseContentsURL
	suite.origVersion = common.Blobfuse2Version

	// Start mock GitHub API server (latest tag = current version so "same version" is default)
	suite.server = newMockGitHubServer("blobfuse2-" + common.Blobfuse2Version_())

	// Override the URL vars to point at the mock server
	common.GitHubLatestReleaseURL = suite.server.URL + "/releases/latest"
	common.GitHubReleaseContentsURL = suite.server.URL + "/contents/release"
}

func (suite *rootCmdSuite) cleanupTest() {
	rootCmd.SetOut(nil)
	rootCmd.SetErr(nil)
	rootCmd.SetArgs(nil)

	// Restore original URL vars and version
	common.GitHubLatestReleaseURL = suite.origReleaseURL
	common.GitHubReleaseContentsURL = suite.origContentsURL
	common.Blobfuse2Version = suite.origVersion
}

func (suite *rootCmdSuite) TearDownTest() {
	if suite.server != nil {
		suite.server.Close()
	}
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

func (suite *rootCmdSuite) TestCheckVersionExistsInvalidURL() {
	defer suite.cleanupTest()
	found := checkVersionExists("abcd")
	suite.assert.False(found)
}

func (suite *rootCmdSuite) TestNoSecurityWarnings() {
	defer suite.cleanupTest()
	warningsUrl := common.GitHubReleaseContentsURL + "/securitywarnings/" + common.Blobfuse2Version
	found := checkVersionExists(warningsUrl)
	suite.assert.False(found)
}

func (suite *rootCmdSuite) TestSecurityWarnings() {
	defer suite.cleanupTest()
	warningsUrl := common.GitHubReleaseContentsURL + "/securitywarnings/" + "1.1.1"
	found := checkVersionExists(warningsUrl)
	suite.assert.True(found)
}

func (suite *rootCmdSuite) TestBlockedVersion() {
	defer suite.cleanupTest()
	warningsUrl := common.GitHubReleaseContentsURL + "/blockedversions/" + "1.1.1"
	isBlocked := checkVersionExists(warningsUrl)
	suite.assert.True(isBlocked)
}

func (suite *rootCmdSuite) TestNonBlockedVersion() {
	defer suite.cleanupTest()
	warningsUrl := common.GitHubReleaseContentsURL + "/blockedversions/" + common.Blobfuse2Version
	found := checkVersionExists(warningsUrl)
	suite.assert.False(found)
}

func (suite *rootCmdSuite) TestGetRemoteVersionInvalidURL() {
	defer suite.cleanupTest()
	out, err := getGitHubLatestRemoteVersion("abcd")
	suite.assert.Nil(out)
	suite.assert.Error(err)
}

func (suite *rootCmdSuite) TestGetRemoteVersionInvalidRepo() {
	defer suite.cleanupTest()
	// Use a path on the mock server that will return 404
	latestVersionUrl := suite.server.URL + "/releases/nonexistent"
	out, err := getGitHubLatestRemoteVersion(latestVersionUrl)
	suite.assert.Nil(out)
	suite.assert.Error(err)
	suite.assert.Contains(err.Error(), "error in GitHub GET latest release")
}

func getDummyVersion() string {
	return "1.0.0"
}

func (suite *rootCmdSuite) TestGetRemoteVersionValidRepo() {
	defer suite.cleanupTest()
	latestVersionUrl := common.GitHubLatestReleaseURL
	out, err := getGitHubLatestRemoteVersion(latestVersionUrl)
	suite.assert.NotEmpty(out)
	suite.assert.NoError(err)
}

func (suite *rootCmdSuite) TestGetRemoteVersionCurrentOlder() {
	defer suite.cleanupTest()
	common.Blobfuse2Version = getDummyVersion()
	msg := <-beginDetectNewVersion()
	suite.assert.NotEmpty(msg)
	suite.assert.Contains(msg, "A new version of Blobfuse2 is available")
}

func (suite *rootCmdSuite) TestGetRemoteVersionCurrentSame() {
	defer suite.cleanupTest()
	common.Blobfuse2Version = common.Blobfuse2Version_()
	msg := <-beginDetectNewVersion()
	suite.assert.Nil(msg)
}

// func (suite *rootCmdSuite) testExecute() {
// 	defer suite.cleanupTest()
// 	buf := new(bytes.Buffer)
// 	rootCmd.SetOut(buf)
// 	rootCmd.SetErr(buf)
// 	rootCmd.SetArgs([]string{"--version"})

// 	err := Execute()
// 	suite.assert.NoError(err)
// 	suite.assert.Contains(buf.String(), "blobfuse2 version")
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
