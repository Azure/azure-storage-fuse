//go:build !authtest
// +build !authtest

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

package azstorage

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type storageTestConfiguration struct {
	// Get the mount path from command line argument
	BlockAccount   string `json:"block-acct"`
	AdlsAccount    string `json:"adls-acct"`
	BlockContainer string `json:"block-cont"`
	AdlsContainer  string `json:"adls-cont"`
	// AdlsDirectory      string `json:"adls-dir"`
	BlockContainerHuge string `json:"block-cont-huge"`
	AdlsContainerHuge  string `json:"adls-cont-huge"`
	BlockKey           string `json:"block-key"`
	AdlsKey            string `json:"adls-key"`
	BlockSas           string `json:"block-sas"`
	BlockContSasUbn18  string `json:"block-cont-sas-ubn-18"`
	BlockContSasUbn20  string `json:"block-cont-sas-ubn-20"`
	AdlsSas            string `json:"adls-sas"`
	// AdlsDirSasUbn18    string `json:"adls-dir-sas-ubn-18"`
	// AdlsDirSasUbn20    string `json:"adls-dir-sas-ubn-20"`
	MsiAppId        string `json:"msi-appid"`
	MsiResId        string `json:"msi-resid"`
	MsiObjId        string `json:"msi-objid"`
	SpnClientId     string `json:"spn-client"`
	SpnTenantId     string `json:"spn-tenant"`
	SpnClientSecret string `json:"spn-secret"`
	SkipMsi         bool   `json:"skip-msi"`
	SkipAzCLI       bool   `json:"skip-azcli"`
	ProxyAddress    string `json:"proxy-address"`
}

var storageTestConfigurationParameters storageTestConfiguration

type authTestSuite struct {
	suite.Suite
}

func (suite *authTestSuite) SetupTest() {
	cfg := common.LogConfig{
		FilePath:    "./logfile.txt",
		MaxFileSize: 10,
		FileCount:   10,
		Level:       common.ELogLevel.LOG_DEBUG(),
	}
	err := log.SetDefaultLogger("base", cfg)
	if err != nil {
		fmt.Println("Unable to set default logger")
		os.Exit(1)
	}

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

	cfgData, _ := io.ReadAll(cfgFile)
	err = json.Unmarshal(cfgData, &storageTestConfigurationParameters)
	if err != nil {
		fmt.Println("Failed to parse the config file")
		os.Exit(1)
	}

	cfgFile.Close()
}

func (suite *authTestSuite) validateStorageTest(testName string, stgConfig AzStorageConfig) {
	assert := assert.New(suite.T())
	stg := NewAzStorageConnection(stgConfig)
	if stg == nil {
		assert.Fail(testName + " : Failed to create Storage object.")
	}
	if err := stg.SetupPipeline(); err != nil {
		assert.Fail(testName + " : Failed to setup pipeline. " + err.Error())
	}
	err := stg.TestPipeline()
	if err != nil {
		assert.Fail(testName + " : Failed to TestPipeline. " + err.Error())
	}
}

func generateEndpoint(useHttp bool, accountName string, accountType AccountType) string {
	endpoint := ""
	if useHttp {
		endpoint += "http://"
	} else {
		endpoint += "https://"
	}
	endpoint += accountName
	if accountType == EAccountType.ADLS() {
		endpoint += ".dfs."
	} else if accountType == EAccountType.BLOCK() {
		endpoint += ".blob."
	}
	endpoint += "core.windows.net/"
	return endpoint
}

func (suite *authTestSuite) TestBlockInvalidAuth() {
	defer suite.cleanupTest()
	stgConfig := AzStorageConfig{
		container: storageTestConfigurationParameters.BlockContainer,
		authConfig: azAuthConfig{
			AuthMode:    EAuthType.INVALID_AUTH(),
			AccountType: EAccountType.BLOCK(),
			AccountName: storageTestConfigurationParameters.BlockAccount,
			AccountKey:  storageTestConfigurationParameters.BlockKey,
			Endpoint:    generateEndpoint(false, storageTestConfigurationParameters.BlockAccount, EAccountType.BLOCK()),
		},
	}
	assert := assert.New(suite.T())
	stg := NewAzStorageConnection(stgConfig)
	if stg == nil {
		assert.Fail("TestInvalidAuth : Failed to create Storage object")
	}
	if err := stg.SetupPipeline(); err == nil {
		assert.Fail("TestInvalidAuth : Setup pipeline even though auth is invalid")
	}
}

func (suite *authTestSuite) TestAdlsInvalidAuth() {
	defer suite.cleanupTest()
	stgConfig := AzStorageConfig{
		container: storageTestConfigurationParameters.AdlsContainer,
		authConfig: azAuthConfig{
			AuthMode:    EAuthType.INVALID_AUTH(),
			AccountType: EAccountType.ADLS(),
			AccountName: storageTestConfigurationParameters.AdlsAccount,
			AccountKey:  storageTestConfigurationParameters.AdlsKey,
			Endpoint:    generateEndpoint(false, storageTestConfigurationParameters.AdlsAccount, EAccountType.ADLS()),
		},
	}
	assert := assert.New(suite.T())
	stg := NewAzStorageConnection(stgConfig)
	if stg == nil {
		assert.Fail("TestInvalidAuth : Failed to create Storage object")
	}
	if err := stg.SetupPipeline(); err == nil {
		assert.Fail("TestInvalidAuth : Setup pipeline even though auth is invalid")
	}
}

func (suite *authTestSuite) TestInvalidAccountType() {
	defer suite.cleanupTest()
	stgConfig := AzStorageConfig{
		container: storageTestConfigurationParameters.BlockContainer,
		authConfig: azAuthConfig{
			AuthMode:    EAuthType.KEY(),
			AccountType: EAccountType.INVALID_ACC(),
			AccountName: storageTestConfigurationParameters.BlockAccount,
			AccountKey:  storageTestConfigurationParameters.BlockKey,
			Endpoint:    generateEndpoint(false, storageTestConfigurationParameters.BlockAccount, EAccountType.BLOCK()),
		},
	}
	assert := assert.New(suite.T())
	stg := NewAzStorageConnection(stgConfig)
	if stg != nil {
		assert.Fail("TestInvalidAuth : Created Storage object even though account type is invalid")
	}
}

func (suite *authTestSuite) TestBlockInvalidSharedKey() {
	defer suite.cleanupTest()
	stgConfig := AzStorageConfig{
		container: storageTestConfigurationParameters.BlockContainer,
		authConfig: azAuthConfig{
			AuthMode:    EAuthType.KEY(),
			AccountType: EAccountType.BLOCK(),
			AccountName: storageTestConfigurationParameters.BlockAccount,
			AccountKey:  "",
			Endpoint:    generateEndpoint(false, storageTestConfigurationParameters.BlockAccount, EAccountType.BLOCK()),
		},
	}
	assert := assert.New(suite.T())
	stg := NewAzStorageConnection(stgConfig)
	if stg == nil {
		assert.Fail("TestBlockInvalidSharedKey : Failed to create Storage object")
	}
	if err := stg.SetupPipeline(); err == nil {
		assert.Fail("TestBlockInvalidSharedKey : Setup pipeline even though shared key is invalid")
	}
}

func (suite *authTestSuite) TestBlockInvalidSharedKey2() {
	defer suite.cleanupTest()
	stgConfig := AzStorageConfig{
		container: storageTestConfigurationParameters.BlockContainer,
		authConfig: azAuthConfig{
			AuthMode:    EAuthType.KEY(),
			AccountType: EAccountType.BLOCK(),
			AccountName: storageTestConfigurationParameters.BlockAccount,
			AccountKey:  "abcd>=", // string that will fail to base64 decode
			Endpoint:    generateEndpoint(false, storageTestConfigurationParameters.BlockAccount, EAccountType.BLOCK()),
		},
	}
	assert := assert.New(suite.T())
	stg := NewAzStorageConnection(stgConfig)
	if stg == nil {
		assert.Fail("TestBlockInvalidSharedKey : Failed to create Storage object")
	}
	if err := stg.SetupPipeline(); err == nil {
		assert.Fail("TestBlockInvalidSharedKey : Setup pipeline even though shared key is invalid")
	}
}

func (suite *authTestSuite) TestBlockSharedKey() {
	defer suite.cleanupTest()
	stgConfig := AzStorageConfig{
		container: storageTestConfigurationParameters.BlockContainer,
		authConfig: azAuthConfig{
			AuthMode:    EAuthType.KEY(),
			AccountType: EAccountType.BLOCK(),
			AccountName: storageTestConfigurationParameters.BlockAccount,
			AccountKey:  storageTestConfigurationParameters.BlockKey,
			Endpoint:    generateEndpoint(false, storageTestConfigurationParameters.BlockAccount, EAccountType.BLOCK()),
		},
	}
	suite.validateStorageTest("TestBlockSharedKey", stgConfig)
}
func (suite *authTestSuite) TestHttpBlockSharedKey() {
	defer suite.cleanupTest()
	stgConfig := AzStorageConfig{
		container: storageTestConfigurationParameters.BlockContainer,
		authConfig: azAuthConfig{
			AuthMode:    EAuthType.KEY(),
			AccountType: EAccountType.BLOCK(),
			AccountName: storageTestConfigurationParameters.BlockAccount,
			AccountKey:  storageTestConfigurationParameters.BlockKey,
			UseHTTP:     true,
			Endpoint:    generateEndpoint(true, storageTestConfigurationParameters.BlockAccount, EAccountType.BLOCK()),
		},
	}
	suite.validateStorageTest("TestHttpBlockSharedKey", stgConfig)
}

func (suite *authTestSuite) TestAdlsInvalidSharedKey() {
	defer suite.cleanupTest()
	stgConfig := AzStorageConfig{
		container: storageTestConfigurationParameters.AdlsContainer,
		authConfig: azAuthConfig{
			AuthMode:    EAuthType.KEY(),
			AccountType: EAccountType.ADLS(),
			AccountName: storageTestConfigurationParameters.AdlsAccount,
			AccountKey:  "",
			Endpoint:    generateEndpoint(false, storageTestConfigurationParameters.AdlsAccount, EAccountType.ADLS()),
		},
	}
	assert := assert.New(suite.T())
	stg := NewAzStorageConnection(stgConfig)
	if stg == nil {
		assert.Fail("TestAdlsInvalidSharedKey : Failed to create Storage object")
	}
	if err := stg.SetupPipeline(); err == nil {
		assert.Fail("TestAdlsInvalidSharedKey : Setup pipeline even though shared key is invalid")
	}
}

func (suite *authTestSuite) TestAdlsSharedKey() {
	defer suite.cleanupTest()
	stgConfig := AzStorageConfig{
		container: storageTestConfigurationParameters.AdlsContainer,
		authConfig: azAuthConfig{
			AuthMode:    EAuthType.KEY(),
			AccountType: EAccountType.ADLS(),
			AccountName: storageTestConfigurationParameters.AdlsAccount,
			AccountKey:  storageTestConfigurationParameters.AdlsKey,
			Endpoint:    generateEndpoint(false, storageTestConfigurationParameters.AdlsAccount, EAccountType.ADLS()),
		},
	}
	suite.validateStorageTest("TestAdlsSharedKey", stgConfig)
}

func (suite *authTestSuite) TestHttpAdlsSharedKey() {
	defer suite.cleanupTest()
	stgConfig := AzStorageConfig{
		container: storageTestConfigurationParameters.AdlsContainer,
		authConfig: azAuthConfig{
			AuthMode:    EAuthType.KEY(),
			AccountType: EAccountType.ADLS(),
			AccountName: storageTestConfigurationParameters.AdlsAccount,
			AccountKey:  storageTestConfigurationParameters.AdlsKey,
			UseHTTP:     true,
			Endpoint:    generateEndpoint(true, storageTestConfigurationParameters.AdlsAccount, EAccountType.ADLS()),
		},
	}
	suite.validateStorageTest("TestHttpAdlsSharedKey", stgConfig)
}

func (suite *authTestSuite) TestBlockInvalidSasKey() {
	defer suite.cleanupTest()
	stgConfig := AzStorageConfig{
		container: storageTestConfigurationParameters.BlockContainer,
		authConfig: azAuthConfig{
			AuthMode:    EAuthType.SAS(),
			AccountType: EAccountType.BLOCK(),
			AccountName: storageTestConfigurationParameters.BlockAccount,
			SASKey:      "",
			Endpoint:    generateEndpoint(false, storageTestConfigurationParameters.BlockAccount, EAccountType.BLOCK()),
		},
	}
	assert := assert.New(suite.T())
	stg := NewAzStorageConnection(stgConfig)
	if stg == nil {
		assert.Fail("TestBlockInvalidSasKey : Failed to create Storage object")
	}
	if err := stg.SetupPipeline(); err == nil {
		assert.Fail("TestBlockInvalidSasKey : Setup pipeline even though sas key is invalid")
	}
}

func (suite *authTestSuite) TestBlockSasKey() {
	defer suite.cleanupTest()
	stgConfig := AzStorageConfig{
		container: storageTestConfigurationParameters.BlockContainer,
		authConfig: azAuthConfig{
			AuthMode:    EAuthType.SAS(),
			AccountType: EAccountType.BLOCK(),
			AccountName: storageTestConfigurationParameters.BlockAccount,
			SASKey:      storageTestConfigurationParameters.BlockSas,
			Endpoint:    generateEndpoint(false, storageTestConfigurationParameters.BlockAccount, EAccountType.BLOCK()),
		},
	}
	suite.validateStorageTest("TestBlockSasKey", stgConfig)
}

func (suite *authTestSuite) TestHttpBlockSasKey() {
	defer suite.cleanupTest()
	stgConfig := AzStorageConfig{
		container: storageTestConfigurationParameters.BlockContainer,
		authConfig: azAuthConfig{
			AuthMode:    EAuthType.SAS(),
			AccountType: EAccountType.BLOCK(),
			AccountName: storageTestConfigurationParameters.BlockAccount,
			SASKey:      storageTestConfigurationParameters.BlockSas,
			UseHTTP:     true,
			Endpoint:    generateEndpoint(true, storageTestConfigurationParameters.BlockAccount, EAccountType.BLOCK()),
		},
	}
	suite.validateStorageTest("TestHttpBlockSasKey", stgConfig)
}

func (suite *authTestSuite) TestBlockContSasKey() {
	defer suite.cleanupTest()
	sas := ""
	if storageTestConfigurationParameters.BlockContainer == "test-cnt-ubn-18" {
		sas = storageTestConfigurationParameters.BlockContSasUbn18
	} else if storageTestConfigurationParameters.BlockContainer == "test-cnt-ubn-20" {
		sas = storageTestConfigurationParameters.BlockContSasUbn20
	} else {
		return
	}

	stgConfig := AzStorageConfig{
		container: storageTestConfigurationParameters.BlockContainer,
		authConfig: azAuthConfig{
			AuthMode:    EAuthType.SAS(),
			AccountType: EAccountType.BLOCK(),
			AccountName: storageTestConfigurationParameters.BlockAccount,
			SASKey:      sas,
			Endpoint:    generateEndpoint(false, storageTestConfigurationParameters.BlockAccount, EAccountType.BLOCK()),
		},
	}
	suite.validateStorageTest("TestBlockContSasKey", stgConfig)
}

func (suite *authTestSuite) TestHttpBlockContSasKey() {
	defer suite.cleanupTest()
	sas := ""
	if storageTestConfigurationParameters.BlockContainer == "test-cnt-ubn-18" {
		sas = storageTestConfigurationParameters.BlockContSasUbn18
	} else if storageTestConfigurationParameters.BlockContainer == "test-cnt-ubn-20" {
		sas = storageTestConfigurationParameters.BlockContSasUbn20
	} else {
		return
	}
	stgConfig := AzStorageConfig{
		container: storageTestConfigurationParameters.BlockContainer,
		authConfig: azAuthConfig{
			AuthMode:    EAuthType.SAS(),
			AccountType: EAccountType.BLOCK(),
			AccountName: storageTestConfigurationParameters.BlockAccount,
			SASKey:      sas,
			UseHTTP:     true,
			Endpoint:    generateEndpoint(true, storageTestConfigurationParameters.BlockAccount, EAccountType.BLOCK()),
		},
	}
	suite.validateStorageTest("TestHttpBlockContSasKey", stgConfig)
}

func (suite *authTestSuite) TestBlockSasKeySetOption() {
	defer suite.cleanupTest()
	stgConfig := AzStorageConfig{
		container: storageTestConfigurationParameters.BlockContainer,
		authConfig: azAuthConfig{
			AuthMode:    EAuthType.SAS(),
			AccountType: EAccountType.BLOCK(),
			AccountName: storageTestConfigurationParameters.BlockAccount,
			SASKey:      storageTestConfigurationParameters.BlockSas,
			Endpoint:    generateEndpoint(false, storageTestConfigurationParameters.BlockAccount, EAccountType.BLOCK()),
		},
	}
	assert := assert.New(suite.T())
	stg := NewAzStorageConnection(stgConfig)
	if stg == nil {
		assert.Fail("TestBlockSasKeySetOption : Failed to create Storage object")
	}
	stg.SetupPipeline()
	stg.UpdateServiceClient("saskey", storageTestConfigurationParameters.BlockSas)
	if err := stg.SetupPipeline(); err != nil {
		assert.Fail("TestBlockSasKeySetOption : Failed to setup pipeline")
	}
	err := stg.TestPipeline()
	if err != nil {
		assert.Fail("TestBlockSasKeySetOption : Failed to TestPipeline")
	}
}

func (suite *authTestSuite) TestAdlsInvalidSasKey() {
	defer suite.cleanupTest()
	stgConfig := AzStorageConfig{
		container: storageTestConfigurationParameters.AdlsContainer,
		authConfig: azAuthConfig{
			AuthMode:    EAuthType.SAS(),
			AccountType: EAccountType.ADLS(),
			AccountName: storageTestConfigurationParameters.AdlsAccount,
			SASKey:      "",
			Endpoint:    generateEndpoint(false, storageTestConfigurationParameters.AdlsAccount, EAccountType.ADLS()),
		},
	}
	assert := assert.New(suite.T())
	stg := NewAzStorageConnection(stgConfig)
	if stg == nil {
		assert.Fail("TestAdlsInvalidSasKey : Failed to create Storage object")
	}
	if err := stg.SetupPipeline(); err == nil {
		assert.Fail("TestAdlsInvalidSasKey : Setup pipeline even though sas key is invalid")
	}
}

// ADLS tests container SAS by default since ADLS account SAS does not support permissions.
func (suite *authTestSuite) TestAdlsSasKey() {
	defer suite.cleanupTest()
	stgConfig := AzStorageConfig{
		container: storageTestConfigurationParameters.AdlsContainer,
		authConfig: azAuthConfig{
			AuthMode:    EAuthType.SAS(),
			AccountType: EAccountType.ADLS(),
			AccountName: storageTestConfigurationParameters.AdlsAccount,
			SASKey:      storageTestConfigurationParameters.AdlsSas,
			Endpoint:    generateEndpoint(false, storageTestConfigurationParameters.AdlsAccount, EAccountType.ADLS()),
		},
	}
	suite.validateStorageTest("TestAdlsSasKey", stgConfig)
}

func (suite *authTestSuite) TestHttpAdlsSasKey() {
	defer suite.cleanupTest()
	stgConfig := AzStorageConfig{
		container: storageTestConfigurationParameters.AdlsContainer,
		authConfig: azAuthConfig{
			AuthMode:    EAuthType.SAS(),
			AccountType: EAccountType.ADLS(),
			AccountName: storageTestConfigurationParameters.AdlsAccount,
			SASKey:      storageTestConfigurationParameters.AdlsSas,
			UseHTTP:     true,
			Endpoint:    generateEndpoint(true, storageTestConfigurationParameters.AdlsAccount, EAccountType.ADLS()),
		},
	}
	suite.validateStorageTest("TestHttpAdlsSasKey", stgConfig)
}

// func (suite *authTestSuite) TestAdlsDirSasKey() {
// 	defer suite.cleanupTest()
// 	assert := assert.New(suite.T())
// 	sas := ""
// 	if storageTestConfigurationParameters.AdlsDirectory == "test-dir-ubn-18" {
// 		sas = storageTestConfigurationParameters.AdlsDirSasUbn18
// 	} else if storageTestConfigurationParameters.AdlsDirectory == "test-dir-ubn-20" {
// 		sas = storageTestConfigurationParameters.AdlsDirSasUbn20
// 	} else {
// 		assert.Fail("TestAdlsDirSasKey : Unknown Directory for Sas Test")
// 	}
// 	stgConfig := AzStorageConfig{
// 		container:  storageTestConfigurationParameters.AdlsContainer,
// 		prefixPath: storageTestConfigurationParameters.AdlsDirectory,
// 		authConfig: azAuthConfig{
// 			AuthMode:    EAuthType.SAS(),
// 			AccountType: EAccountType.ADLS(),
// 			AccountName: storageTestConfigurationParameters.AdlsAccount,
// 			SASKey:      sas,
// 			Endpoint:    generateEndpoint(false, storageTestConfigurationParameters.AdlsAccount, EAccountType.ADLS()),
// 		},
// 	}
// 	suite.validateStorageTest("TestAdlsDirSasKey", stgConfig)
// }

// func (suite *authTestSuite) TestHttpAdlsDirSasKey() {
// 	defer suite.cleanupTest()
// 	assert := assert.New(suite.T())
// 	sas := ""
// 	if storageTestConfigurationParameters.AdlsDirectory == "test-dir-ubn-18" {
// 		sas = storageTestConfigurationParameters.AdlsDirSasUbn18
// 	} else if storageTestConfigurationParameters.AdlsDirectory == "test-dir-ubn-20" {
// 		sas = storageTestConfigurationParameters.AdlsDirSasUbn20
// 	} else {
// 		assert.Fail("TestHttpAdlsDirSasKey : Unknown Directory for Sas Test")
// 	}
// 	stgConfig := AzStorageConfig{
// 		container:  storageTestConfigurationParameters.AdlsContainer,
// 		prefixPath: storageTestConfigurationParameters.AdlsDirectory,
// 		authConfig: azAuthConfig{
// 			AuthMode:    EAuthType.SAS(),
// 			AccountType: EAccountType.ADLS(),
// 			AccountName: storageTestConfigurationParameters.AdlsAccount,
// 			SASKey:      sas,
// 			UseHTTP:     true,
// 			Endpoint:    generateEndpoint(true, storageTestConfigurationParameters.AdlsAccount, EAccountType.ADLS()),
// 		},
// 	}
// 	suite.validateStorageTest("TestHttpAdlsDirSasKey", stgConfig)
// }

func (suite *authTestSuite) TestAdlsSasKeySetOption() {
	defer suite.cleanupTest()
	stgConfig := AzStorageConfig{
		container: storageTestConfigurationParameters.AdlsContainer,
		authConfig: azAuthConfig{
			AuthMode:    EAuthType.SAS(),
			AccountType: EAccountType.ADLS(),
			AccountName: storageTestConfigurationParameters.AdlsAccount,
			SASKey:      storageTestConfigurationParameters.AdlsSas,
			Endpoint:    generateEndpoint(false, storageTestConfigurationParameters.AdlsAccount, EAccountType.ADLS()),
		},
	}
	assert := assert.New(suite.T())
	stg := NewAzStorageConnection(stgConfig)
	if stg == nil {
		assert.Fail("TestBlockSasKeySetOption : Failed to create Storage object")
	}
	stg.SetupPipeline()
	stg.UpdateServiceClient("saskey", storageTestConfigurationParameters.AdlsSas)
	if err := stg.SetupPipeline(); err != nil {
		assert.Fail("TestBlockSasKeySetOption : Failed to setup pipeline")
	}
	err := stg.TestPipeline()
	if err != nil {
		assert.Fail("TestBlockSasKeySetOption : Failed to TestPipeline")
	}
}

func (suite *authTestSuite) TestBlockMsiAppId() {
	defer suite.cleanupTest()
	if !storageTestConfigurationParameters.SkipMsi {
		stgConfig := AzStorageConfig{
			container: storageTestConfigurationParameters.BlockContainer,
			authConfig: azAuthConfig{
				AuthMode:      EAuthType.MSI(),
				AccountType:   EAccountType.BLOCK(),
				AccountName:   storageTestConfigurationParameters.BlockAccount,
				ApplicationID: storageTestConfigurationParameters.MsiAppId,
				Endpoint:      generateEndpoint(false, storageTestConfigurationParameters.BlockAccount, EAccountType.BLOCK()),
			},
		}
		suite.validateStorageTest("TestBlockMsiAppId", stgConfig)
	}
}
func (suite *authTestSuite) TestBlockMsiResId() {
	defer suite.cleanupTest()
	if !storageTestConfigurationParameters.SkipMsi {
		stgConfig := AzStorageConfig{
			container: storageTestConfigurationParameters.BlockContainer,
			authConfig: azAuthConfig{
				AuthMode:    EAuthType.MSI(),
				AccountType: EAccountType.BLOCK(),
				AccountName: storageTestConfigurationParameters.BlockAccount,
				ResourceID:  storageTestConfigurationParameters.MsiResId,
				Endpoint:    generateEndpoint(false, storageTestConfigurationParameters.BlockAccount, EAccountType.BLOCK()),
			},
		}
		suite.validateStorageTest("TestBlockMsiResId", stgConfig)
	}
}

func (suite *authTestSuite) TestBlockMsiObjId() {
	defer suite.cleanupTest()
	if !storageTestConfigurationParameters.SkipMsi {
		stgConfig := AzStorageConfig{
			container: storageTestConfigurationParameters.BlockContainer,
			authConfig: azAuthConfig{
				AuthMode:    EAuthType.MSI(),
				AccountType: EAccountType.BLOCK(),
				AccountName: storageTestConfigurationParameters.BlockAccount,
				ObjectID:    storageTestConfigurationParameters.MsiObjId,
				Endpoint:    generateEndpoint(false, storageTestConfigurationParameters.BlockAccount, EAccountType.BLOCK()),
			},
		}
		suite.validateStorageTest("TestBlockMsiObjId", stgConfig)
	}
}

// Can't use HTTP requests with MSI/SPN credentials
func (suite *authTestSuite) TestAdlsMsiAppId() {
	defer suite.cleanupTest()
	if !storageTestConfigurationParameters.SkipMsi {
		stgConfig := AzStorageConfig{
			container: storageTestConfigurationParameters.AdlsContainer,
			authConfig: azAuthConfig{
				AuthMode:      EAuthType.MSI(),
				AccountType:   EAccountType.ADLS(),
				AccountName:   storageTestConfigurationParameters.AdlsAccount,
				ApplicationID: storageTestConfigurationParameters.MsiAppId,
				Endpoint:      generateEndpoint(false, storageTestConfigurationParameters.AdlsAccount, EAccountType.ADLS()),
			},
		}
		suite.validateStorageTest("TestAdlsMsiAppId", stgConfig)
	}
}

func (suite *authTestSuite) TestAdlskMsiResId() {
	defer suite.cleanupTest()
	if !storageTestConfigurationParameters.SkipMsi {
		stgConfig := AzStorageConfig{
			container: storageTestConfigurationParameters.AdlsContainer,
			authConfig: azAuthConfig{
				AuthMode:    EAuthType.MSI(),
				AccountType: EAccountType.ADLS(),
				AccountName: storageTestConfigurationParameters.AdlsAccount,
				ResourceID:  storageTestConfigurationParameters.MsiResId,
				Endpoint:    generateEndpoint(false, storageTestConfigurationParameters.AdlsAccount, EAccountType.ADLS()),
			},
		}
		suite.validateStorageTest("TestAdlskMsiResId", stgConfig)
	}
}

func (suite *authTestSuite) TestBlockAzCLI() {
	defer suite.cleanupTest()
	stgConfig := AzStorageConfig{
		container: storageTestConfigurationParameters.BlockContainer,
		authConfig: azAuthConfig{
			AuthMode:    EAuthType.AZCLI(),
			AccountType: EAccountType.BLOCK(),
			AccountName: storageTestConfigurationParameters.BlockAccount,
			Endpoint:    generateEndpoint(false, storageTestConfigurationParameters.BlockAccount, EAccountType.BLOCK()),
		},
	}

	assert := assert.New(suite.T())
	stg := NewAzStorageConnection(stgConfig)
	assert.NotNil(stg)

	err := stg.SetupPipeline()
	assert.Nil(err)

	err = stg.TestPipeline()
	if storageTestConfigurationParameters.SkipAzCLI {
		// error is returned when azcli is not installed or logged out
		assert.NotNil(err)
	} else {
		assert.Nil(err)
	}
}

func (suite *authTestSuite) TestAdlsAzCLI() {
	defer suite.cleanupTest()
	stgConfig := AzStorageConfig{
		container: storageTestConfigurationParameters.AdlsContainer,
		authConfig: azAuthConfig{
			AuthMode:    EAuthType.AZCLI(),
			AccountType: EAccountType.ADLS(),
			AccountName: storageTestConfigurationParameters.AdlsAccount,
			Endpoint:    generateEndpoint(false, storageTestConfigurationParameters.AdlsAccount, EAccountType.ADLS()),
		},
	}

	assert := assert.New(suite.T())
	stg := NewAzStorageConnection(stgConfig)
	assert.NotNil(stg)

	err := stg.SetupPipeline()
	assert.Nil(err)

	err = stg.TestPipeline()
	if storageTestConfigurationParameters.SkipAzCLI {
		// error is returned when azcli is not installed or logged out
		assert.NotNil(err)
	} else {
		assert.Nil(err)
	}
}

func (suite *authTestSuite) cleanupTest() {
	_ = log.Destroy()
}

func TestAuthTestSuite(t *testing.T) {
	suite.Run(t, new(authTestSuite))
}
