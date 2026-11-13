// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

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

	suite.assert.NoError(err)

	defer os.Remove(confFile.Name())
	defer os.Remove(outFile.Name())
	defer os.Remove(mntDir)

	_, err = confFile.WriteString(configGenOne)
	suite.assert.NoError(err)

	_, err = executeCommandC(rootCmd, "mountgen1", mntDir, "--generate-json-only=true", "--required-free-space-mb=500", fmt.Sprintf("--config-file=%s", confFile.Name()), fmt.Sprintf("--output-file=%s", outFile.Name()))
	suite.assert.NoError(err)

	viper.SetConfigFile("json")
	err = config.ReadFromConfigFile(outFile.Name())
	suite.assert.NoError(err)

	var clientId, tenantId, cacheDir, mountDirTest string
	err = config.UnmarshalKey("clientid", &clientId)
	suite.assert.NoError(err)
	err = config.UnmarshalKey("tenantid", &tenantId)
	suite.assert.NoError(err)
	err = config.UnmarshalKey("cachedir", &cacheDir)
	suite.assert.NoError(err)
	err = config.UnmarshalKey("mountdir", &mountDirTest)
	suite.assert.NoError(err)

	suite.assert.Equal("myClientId", clientId)
	suite.assert.Equal("myTenantId", tenantId)
	suite.assert.Contains(cacheDir, "fileCachePath")
	suite.assert.Equal(mntDir, mountDirTest)
}

func (suite *genOneConfigTestSuite) TestInvalidConfig() {
	defer suite.cleanupTest()
	confFile, _ := os.CreateTemp("", "conf*.yaml")
	outFile, _ := os.CreateTemp("", "adlsgen1fuse*.json")
	mntDir, err := os.MkdirTemp("", "mntdir")

	suite.assert.NoError(err)

	defer os.Remove(confFile.Name())
	defer os.Remove(outFile.Name())
	defer os.Remove(mntDir)

	_, err = confFile.WriteString(invalidConfig)
	suite.assert.NoError(err)

	_, err = executeCommandC(rootCmd, "mountgen1", mntDir, "--generate-json-only=true", fmt.Sprintf("--config-file=%s", confFile.Name()), fmt.Sprintf("--output-file=%s", outFile.Name()))
	suite.assert.Error(err)
}

func (suite *genOneConfigTestSuite) TestInvalidAuthMode() {
	defer suite.cleanupTest()
	confFile, _ := os.CreateTemp("", "conf*.yaml")
	outFile, _ := os.CreateTemp("", "adlsgen1fuse*.json")
	mntDir, err := os.MkdirTemp("", "mntdir")

	suite.assert.NoError(err)

	defer os.Remove(confFile.Name())
	defer os.Remove(outFile.Name())
	defer os.Remove(mntDir)

	_, err = confFile.WriteString(invalidAuthMode)
	suite.assert.NoError(err)

	_, err = executeCommandC(rootCmd, "mountgen1", mntDir, "--generate-json-only=true", fmt.Sprintf("--config-file=%s", confFile.Name()), fmt.Sprintf("--output-file=%s", outFile.Name()))
	suite.assert.Error(err)
}

func (suite *genOneConfigTestSuite) TestGen1FuseMount() {
	defer suite.cleanupTest()
	confFile, _ := os.CreateTemp("", "conf*.yaml")
	outFile, _ := os.CreateTemp("", "adlsgen1fuse*.json")
	mntDir, err := os.MkdirTemp("", "mntdir")

	suite.assert.NoError(err)

	defer os.Remove(confFile.Name())
	defer os.Remove(outFile.Name())
	defer os.Remove(mntDir)

	_, err = confFile.WriteString(configGenOne)
	suite.assert.NoError(err)

	_, err = executeCommandC(rootCmd, "mountgen1", mntDir, "--required-free-space-mb=500", fmt.Sprintf("--config-file=%s", confFile.Name()), fmt.Sprintf("--output-file=%s", outFile.Name()))
	suite.assert.Error(err)
}
