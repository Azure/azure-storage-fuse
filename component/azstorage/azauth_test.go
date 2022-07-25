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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type storageTestConfiguration struct {
	// Get the mount path from command line argument
	BlockAccount       string `json:"block-acct"`
	AdlsAccount        string `json:"adls-acct"`
	BlockContainer     string `json:"block-cont"`
	AdlsContainer      string `json:"adls-cont"`
	BlockContainerHuge string `json:"block-cont-huge"`
	AdlsContainerHuge  string `json:"adls-cont-huge"`
	BlockKey           string `json:"block-key"`
	AdlsKey            string `json:"adls-key"`
	BlockSas           string `json:"block-sas"`
	AdlsSas            string `json:"adls-sas"`
	MsiAppId           string `json:"msi-appid"`
	MsiResId           string `json:"msi-resid"`
	SpnClientId        string `json:"spn-client"`
	SpnTenantId        string `json:"spn-tenant"`
	SpnClientSecret    string `json:"spn-secret"`
	SkipMsi            bool   `json:"skip-msi"`
	ProxyAddress       string `json:"proxy-address"`
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

	cfgData, _ := ioutil.ReadAll(cfgFile)
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
		assert.Fail(testName + " : Failed to create Storage object")
	}
	if err := stg.SetupPipeline(); err != nil {
		assert.Fail(testName + " : Failed to setup pipeline")
	}
	err := stg.TestPipeline()
	if err != nil {
		assert.Fail(testName + " : Failed to TestPipeline")
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

// Can't use HTTP requests with MSI/SPN credentials
func (suite *authTestSuite) TestAdlskMsiAppId() {
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
		suite.validateStorageTest("TestAdlskMsiAppId", stgConfig)
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
func (suite *authTestSuite) TestBlockSpn() {
	defer suite.cleanupTest()
	stgConfig := AzStorageConfig{
		container: storageTestConfigurationParameters.BlockContainer,
		authConfig: azAuthConfig{
			AuthMode:     EAuthType.SPN(),
			AccountType:  EAccountType.BLOCK(),
			AccountName:  storageTestConfigurationParameters.BlockAccount,
			ClientID:     storageTestConfigurationParameters.SpnClientId,
			TenantID:     storageTestConfigurationParameters.SpnTenantId,
			ClientSecret: storageTestConfigurationParameters.SpnClientSecret,
			Endpoint:     generateEndpoint(false, storageTestConfigurationParameters.BlockAccount, EAccountType.BLOCK()),
		},
	}
	suite.validateStorageTest("TestBlockSpn", stgConfig)
}

func (suite *authTestSuite) TestAdlsSpn() {
	defer suite.cleanupTest()
	stgConfig := AzStorageConfig{
		container: storageTestConfigurationParameters.AdlsContainer,
		authConfig: azAuthConfig{
			AuthMode:     EAuthType.SPN(),
			AccountType:  EAccountType.ADLS(),
			AccountName:  storageTestConfigurationParameters.AdlsAccount,
			ClientID:     storageTestConfigurationParameters.SpnClientId,
			TenantID:     storageTestConfigurationParameters.SpnTenantId,
			ClientSecret: storageTestConfigurationParameters.SpnClientSecret,
			Endpoint:     generateEndpoint(false, storageTestConfigurationParameters.AdlsAccount, EAccountType.ADLS()),
		},
	}
	suite.validateStorageTest("TestAdlsSpn", stgConfig)
}

func (suite *authTestSuite) cleanupTest() {
	_ = log.Destroy()
}

func TestAuthTestSuite(t *testing.T) {
	suite.Run(t, new(authTestSuite))
}
