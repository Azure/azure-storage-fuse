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

package cmd

import (
	"fmt"
	"os"
	"testing"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/config"
	"github.com/Azure/azure-storage-fuse/v2/common/log"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type genOneConfigTestSuite struct {
	suite.Suite
	assert *assert.Assertions
}

var configGenOne string = `
logging:
    type: syslog
file_cache:
    path: fileCachePath
libfuse:
    attribute-expiration-sec: 120
    entry-expiration-sec: 60
azstorage:
    account-name: myAccountName
    tenantid: myTenantId
    clientid: myClientId
    endpoint: myEndpoint
    container: myContainer
    mode: spn
    max-retries: 2
components:
    - libfuse
    - file_cache
    - azstorage
`

var invalidConfig string = `
azstorage:
    account-name: myAccountName
    mode: key
components:
    - azstorage
`

var invalidAuthMode string = `
azstorage:
    account-name: myAccountName
    tenantid: myTenantId
    clientid: myClientId
    mode: key
components:
    - azstorage
`

func (suite *genOneConfigTestSuite) SetupTest() {
	suite.assert = assert.New(suite.T())
	err := log.SetDefaultLogger("silent", common.LogConfig{Level: common.ELogLevel.LOG_DEBUG()})
	if err != nil {
		panic("Unable to set silent logger as default.")
	}
}

func (suite *genOneConfigTestSuite) cleanupTest() {
	resetCLIFlags(*gen1Cmd)
	generateJsonOnly = false
}

func TestGenOneConfig(t *testing.T) {
	suite.Run(t, new(genOneConfigTestSuite))
}

func (suite *genOneConfigTestSuite) TestConfigCreation() {
	defer suite.cleanupTest()
	confFile, _ := os.CreateTemp("", "conf*.yaml")
	outFile, _ := os.CreateTemp("", "adlsgen1fuse*.json")
	mntDir, err := os.MkdirTemp("", "mntdir")

	suite.assert.Nil(err)

	defer os.Remove(confFile.Name())
	defer os.Remove(outFile.Name())
	defer os.Remove(mntDir)

	_, err = confFile.WriteString(configGenOne)
	suite.assert.Nil(err)

	_, err = executeCommandC(rootCmd, "mountgen1", mntDir, "--generate-json-only=true", "--required-free-space-mb=500", fmt.Sprintf("--config-file=%s", confFile.Name()), fmt.Sprintf("--output-file=%s", outFile.Name()))
	suite.assert.Nil(err)

	viper.SetConfigFile("json")
	config.ReadFromConfigFile(outFile.Name())

	var clientId, tenantId, cacheDir, mountDirTest string
	config.UnmarshalKey("clientid", &clientId)
	config.UnmarshalKey("tenantid", &tenantId)
	config.UnmarshalKey("cachedir", &cacheDir)
	config.UnmarshalKey("mountdir", &mountDirTest)

	suite.assert.EqualValues("myClientId", clientId)
	suite.assert.EqualValues("myTenantId", tenantId)
	suite.assert.Contains(cacheDir, "fileCachePath")
	suite.assert.EqualValues(mntDir, mountDirTest)
}

func (suite *genOneConfigTestSuite) TestInvalidConfig() {
	defer suite.cleanupTest()
	confFile, _ := os.CreateTemp("", "conf*.yaml")
	outFile, _ := os.CreateTemp("", "adlsgen1fuse*.json")
	mntDir, err := os.MkdirTemp("", "mntdir")

	suite.assert.Nil(err)

	defer os.Remove(confFile.Name())
	defer os.Remove(outFile.Name())
	defer os.Remove(mntDir)

	_, err = confFile.WriteString(invalidConfig)
	suite.assert.Nil(err)

	_, err = executeCommandC(rootCmd, "mountgen1", mntDir, "--generate-json-only=true", fmt.Sprintf("--config-file=%s", confFile.Name()), fmt.Sprintf("--output-file=%s", outFile.Name()))
	suite.assert.NotNil(err)
}

func (suite *genOneConfigTestSuite) TestInvalidAuthMode() {
	defer suite.cleanupTest()
	confFile, _ := os.CreateTemp("", "conf*.yaml")
	outFile, _ := os.CreateTemp("", "adlsgen1fuse*.json")
	mntDir, err := os.MkdirTemp("", "mntdir")

	suite.assert.Nil(err)

	defer os.Remove(confFile.Name())
	defer os.Remove(outFile.Name())
	defer os.Remove(mntDir)

	_, err = confFile.WriteString(invalidAuthMode)
	suite.assert.Nil(err)

	_, err = executeCommandC(rootCmd, "mountgen1", mntDir, "--generate-json-only=true", fmt.Sprintf("--config-file=%s", confFile.Name()), fmt.Sprintf("--output-file=%s", outFile.Name()))
	suite.assert.NotNil(err)
}

func (suite *genOneConfigTestSuite) TestGen1FuseMount() {
	defer suite.cleanupTest()
	confFile, _ := os.CreateTemp("", "conf*.yaml")
	outFile, _ := os.CreateTemp("", "adlsgen1fuse*.json")
	mntDir, err := os.MkdirTemp("", "mntdir")

	suite.assert.Nil(err)

	defer os.Remove(confFile.Name())
	defer os.Remove(outFile.Name())
	defer os.Remove(mntDir)

	_, err = confFile.WriteString(configGenOne)
	suite.assert.Nil(err)

	_, err = executeCommandC(rootCmd, "mountgen1", mntDir, "--required-free-space-mb=500", fmt.Sprintf("--config-file=%s", confFile.Name()), fmt.Sprintf("--output-file=%s", outFile.Name()))
	suite.assert.NotNil(err)
}
