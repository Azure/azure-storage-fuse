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
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blockblob"
	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/config"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type configTestSuite struct {
	suite.Suite
}

func (suite *configTestSuite) SetupTest() {
	err := log.SetDefaultLogger("silent", common.LogConfig{Level: common.ELogLevel.LOG_DEBUG()})
	if err != nil {
		panic("Unable to set silent logger as default.")
	}
}

func (s *configTestSuite) TestEmptyAccountName() {
	defer config.ResetConfig()
	assert := assert.New(s.T())
	az := &AzStorage{}
	opt := AzStorageOptions{}

	err := ParseAndValidateConfig(az, opt)
	assert.NotNil(err)
	assert.Contains(err.Error(), "account name not provided")

}

func (s *configTestSuite) TestEmptyAccountType() {
	defer config.ResetConfig()
	assert := assert.New(s.T())
	az := &AzStorage{}
	opt := AzStorageOptions{}
	opt.AccountName = "abcd"

	err := ParseAndValidateConfig(az, opt)
	assert.NotNil(err)
}

func (s *configTestSuite) TestInvalidAccountType() {
	defer config.ResetConfig()
	assert := assert.New(s.T())
	az := &AzStorage{}
	opt := AzStorageOptions{}
	opt.AccountName = "abcd"
	opt.AccountType = "abcd"

	err := ParseAndValidateConfig(az, opt)
	assert.NotNil(err)
	assert.Contains(err.Error(), "invalid account type")

	opt.AccountType = "INVALID_ACC"
	err = ParseAndValidateConfig(az, opt)
	assert.NotNil(err)
	assert.Contains(err.Error(), "invalid account type")
}

func (s *configTestSuite) TestUseADLSFlag() {
	defer config.ResetConfig()
	assert := assert.New(s.T())
	az := &AzStorage{}
	opt := AzStorageOptions{}
	opt.AccountName = "abcd"
	opt.AccountType = "abcd"

	config.SetBool(compName+".use-adls", true)
	opt.UseAdls = true
	err := ParseAndValidateConfig(az, opt)
	assert.NotNil(err)
	assert.Equal(az.stConfig.authConfig.AccountType, az.stConfig.authConfig.AccountType.ADLS())

	config.SetBool(compName+".use-adls", true)
	opt.UseAdls = false
	err = ParseAndValidateConfig(az, opt)
	assert.NotNil(err)
	assert.Equal(az.stConfig.authConfig.AccountType, az.stConfig.authConfig.AccountType.BLOCK())
}

func (s *configTestSuite) TestBlockSize() {
	defer config.ResetConfig()
	assert := assert.New(s.T())
	az := &AzStorage{}
	opt := AzStorageOptions{}
	opt.AccountName = "abcd"
	opt.BlockSize = 10

	err := ParseAndValidateConfig(az, opt)
	assert.NotNil(err)
	assert.Equal(az.stConfig.blockSize, opt.BlockSize*1024*1024)

	opt.BlockSize = blockblob.MaxStageBlockBytes + 1
	err = ParseAndValidateConfig(az, opt)
	assert.NotNil(err)
	assert.Contains(err.Error(), "block size is too large")
}

func (s *configTestSuite) TestProtoType() {
	defer config.ResetConfig()
	assert := assert.New(s.T())
	az := &AzStorage{}
	opt := AzStorageOptions{}
	opt.AccountName = "abcd"
	opt.Container = "abcd"

	config.SetBool(compName+".use-https", true)
	opt.UseHTTPS = true
	err := ParseAndValidateConfig(az, opt)
	assert.Nil(err)
	assert.Equal(az.stConfig.authConfig.UseHTTP, false)

	config.SetBool(compName+".use-https", false)
	opt.UseHTTPS = false
	opt.AccountType = "adls"
	err = ParseAndValidateConfig(az, opt)
	assert.Nil(err)
	assert.Equal(az.stConfig.authConfig.UseHTTP, true)
}

func (s *configTestSuite) TestProxyConfig() {
	defer config.ResetConfig()
	assert := assert.New(s.T())

	az := &AzStorage{}
	opt := AzStorageOptions{}
	opt.AccountName = "abcd"
	opt.Container = "abcd"

	config.SetBool(compName+".use-https", false)
	opt.UseHTTPS = false

	opt.HttpsProxyAddress = "127.0.0.1"
	err := ParseAndValidateConfig(az, opt)
	assert.Nil(err)
	assert.Equal(az.stConfig.proxyAddress, formatEndpointProtocol(opt.HttpsProxyAddress, !opt.UseHTTPS))

	opt.HttpsProxyAddress = "https://128.0.0.1:8080/"
	err = ParseAndValidateConfig(az, opt)
	assert.Nil(err)
	assert.Equal(az.stConfig.proxyAddress, formatEndpointProtocol(opt.HttpsProxyAddress, !opt.UseHTTPS))

	opt.HttpsProxyAddress = "http://129.0.0.1:8080/"
	err = ParseAndValidateConfig(az, opt)
	assert.Nil(err)
	assert.Equal(az.stConfig.proxyAddress, formatEndpointProtocol(opt.HttpsProxyAddress, !opt.UseHTTPS))

	opt.HttpProxyAddress = "130.0.0.1"
	err = ParseAndValidateConfig(az, opt)
	assert.Nil(err)
	assert.Equal(az.stConfig.proxyAddress, formatEndpointProtocol(opt.HttpProxyAddress, !opt.UseHTTPS))

	opt.HttpProxyAddress = "http://131.0.0.1:8080/"
	err = ParseAndValidateConfig(az, opt)
	assert.Nil(err)
	assert.Equal(az.stConfig.proxyAddress, formatEndpointProtocol(opt.HttpProxyAddress, !opt.UseHTTPS))

	config.SetBool(compName+".use-https", true)
	opt.UseHTTPS = true
	opt.HttpsProxyAddress = ""

	opt.HttpProxyAddress = "132.0.0.1"
	err = ParseAndValidateConfig(az, opt)
	assert.NotNil(err)
	assert.Contains(err.Error(), "`http-proxy` Invalid : must set `use-http: true`")

	opt.HttpsProxyAddress = "133.0.0.1"
	err = ParseAndValidateConfig(az, opt)
	assert.Nil(err)
	assert.Equal(az.stConfig.proxyAddress, formatEndpointProtocol(opt.HttpsProxyAddress, !opt.UseHTTPS))

	opt.HttpsProxyAddress = "http://134.0.0.1:8080/"
	err = ParseAndValidateConfig(az, opt)
	assert.Nil(err)
	assert.Equal(az.stConfig.proxyAddress, formatEndpointProtocol(opt.HttpsProxyAddress, !opt.UseHTTPS))

	opt.HttpsProxyAddress = "https://135.0.0.1:8080/"
	err = ParseAndValidateConfig(az, opt)
	assert.Nil(err)
	assert.Equal(az.stConfig.proxyAddress, formatEndpointProtocol(opt.HttpsProxyAddress, !opt.UseHTTPS))
}

func (s *configTestSuite) TestMaxResultsForList() {
	defer config.ResetConfig()
	assert := assert.New(s.T())
	az := &AzStorage{}
	opt := AzStorageOptions{}
	opt.AccountName = "abcd"
	opt.Container = "abcd"

	err := ParseAndValidateConfig(az, opt)
	assert.Nil(err)
	assert.Equal(az.stConfig.maxResultsForList, DefaultMaxResultsForList)

	config.Set(compName+".max-results-for-list", "10")
	opt.MaxResultsForList = 10
	err = ParseAndValidateConfig(az, opt)
	assert.Nil(err)
	assert.Equal(az.stConfig.maxResultsForList, opt.MaxResultsForList)
}

func (s *configTestSuite) TestAuthModeNotSet() {
	defer config.ResetConfig()
	assert := assert.New(s.T())
	az := &AzStorage{}
	opt := AzStorageOptions{}
	opt.AccountName = "abcd"
	opt.Container = "abcd"

	err := ParseAndValidateConfig(az, opt)
	assert.Nil(err)
	assert.Equal(az.stConfig.authConfig.AuthMode, EAuthType.MSI())
}

func (s *configTestSuite) TestAuthModeKey() {
	defer config.ResetConfig()
	assert := assert.New(s.T())
	az := &AzStorage{}
	opt := AzStorageOptions{}
	opt.AccountName = "abcd"
	opt.Container = "abcd"
	opt.AuthMode = "key"

	err := ParseAndValidateConfig(az, opt)
	assert.NotNil(err)
	assert.Equal(az.stConfig.authConfig.AuthMode, EAuthType.KEY())
	assert.Contains(err.Error(), "storage key not provided")

	opt.AccountKey = "abc"
	err = ParseAndValidateConfig(az, opt)
	assert.Nil(err)
	assert.Equal(az.stConfig.authConfig.AccountKey, opt.AccountKey)
}

func (s *configTestSuite) TestAuthModeSAS() {
	defer config.ResetConfig()
	assert := assert.New(s.T())
	az := &AzStorage{}
	opt := AzStorageOptions{}
	opt.AccountName = "abcd"
	opt.Container = "abcd"
	opt.AuthMode = "sas"

	err := ParseAndValidateConfig(az, opt)
	assert.NotNil(err)
	assert.Equal(az.stConfig.authConfig.AuthMode, EAuthType.SAS())
	assert.Contains(err.Error(), "SAS key not provided")

	opt.SaSKey = "abc"
	err = ParseAndValidateConfig(az, opt)
	assert.Nil(err)
}

func (s *configTestSuite) TestAuthModeMSI() {
	defer config.ResetConfig()
	assert := assert.New(s.T())
	az := &AzStorage{}
	opt := AzStorageOptions{}
	opt.AccountName = "abcd"
	opt.Container = "abcd"
	opt.AuthMode = "msi"

	err := ParseAndValidateConfig(az, opt)
	assert.Nil(err)
	assert.Equal(az.stConfig.authConfig.AuthMode, EAuthType.MSI())

	opt.ApplicationID = "abc"
	err = ParseAndValidateConfig(az, opt)
	assert.Nil(err)
	assert.Equal(az.stConfig.authConfig.AuthMode, EAuthType.MSI())
	assert.Equal(az.stConfig.authConfig.ApplicationID, opt.ApplicationID)
	assert.Equal(az.stConfig.authConfig.ResourceID, "")

	// test more than one credential passed for msi
	opt.ResourceID = "123"
	err = validateMsiConfig(opt)
	assert.NotNil(err)
	opt.ApplicationID = ""

	err = ParseAndValidateConfig(az, opt)
	assert.Nil(err)
	assert.Equal(az.stConfig.authConfig.ResourceID, opt.ResourceID)

	opt.ResourceID = ""
	opt.ObjectID = "1234obj"

	err = ParseAndValidateConfig(az, opt)
	assert.Nil(err)
	assert.Equal(az.stConfig.authConfig.ObjectID, opt.ObjectID)
}

func (s *configTestSuite) TestAuthModeSPN() {
	defer config.ResetConfig()
	assert := assert.New(s.T())
	az := &AzStorage{}
	opt := AzStorageOptions{}
	opt.AccountName = "abcd"
	opt.Container = "abcd"
	opt.AuthMode = "spn"

	err := ParseAndValidateConfig(az, opt)
	assert.NotNil(err)
	assert.Equal(az.stConfig.authConfig.AuthMode, EAuthType.SPN())
	assert.Contains(err.Error(), "Client ID, Tenant ID or Client Secret, OAuthTokenFilePath, WorkloadIdentityToken not provided")

	opt.ClientID = "abc"
	err = ParseAndValidateConfig(az, opt)
	assert.NotNil(err)
	assert.Equal(az.stConfig.authConfig.AuthMode, EAuthType.SPN())
	assert.Contains(err.Error(), "Client ID, Tenant ID or Client Secret, OAuthTokenFilePath, WorkloadIdentityToken not provided")

	opt.ClientSecret = "123"
	opt.TenantID = "xyz"
	err = ParseAndValidateConfig(az, opt)
	assert.Nil(err)
	assert.Equal(az.stConfig.authConfig.ClientID, opt.ClientID)
	assert.Equal(az.stConfig.authConfig.ClientSecret, opt.ClientSecret)
	assert.Equal(az.stConfig.authConfig.TenantID, opt.TenantID)
}

func (s *configTestSuite) TestOtherFlags() {
	defer config.ResetConfig()
	assert := assert.New(s.T())
	az := &AzStorage{}
	opt := AzStorageOptions{}
	opt.AccountName = "abcd"
	opt.Container = "abcd"
	opt.AuthMode = "sas"

	// opt.SaSKey = "xyz"
	opt.MaxRetries = 10
	opt.MaxTimeout = 10
	opt.BackoffTime = 10
	opt.MaxRetryDelay = 10
	opt.BlockSize = 5
	opt.MaxConcurrency = 20
	opt.DefaultTier = "hot"

	config.SetBool(compName+".set-content-type", true)
	config.SetBool(compName+".ca-cert-file", true)
	config.SetBool(compName+".debug-libcurl", true)

	err := ParseAndValidateConfig(az, opt)
	assert.NotNil(err)
	assert.Equal(err.Error(), "SAS key not provided")
}

func (s *configTestSuite) TestCompressionType() {
	defer config.ResetConfig()
	assert := assert.New(s.T())
	az := &AzStorage{}
	opt := AzStorageOptions{}
	opt.AccountName = "abcd"
	opt.Container = "abcd"

	err := ParseAndValidateConfig(az, opt)
	assert.Nil(err)
	assert.Equal(az.stConfig.disableCompression, false)

	opt.DisableCompression = true
	config.SetBool(compName+".disable-compression", true)
	err = ParseAndValidateConfig(az, opt)
	assert.Nil(err)
	assert.Equal(az.stConfig.disableCompression, true)

	opt.DisableCompression = false
	config.SetBool(compName+".disable-compression", false)
	err = ParseAndValidateConfig(az, opt)
	assert.Nil(err)
	assert.Equal(az.stConfig.disableCompression, false)

}

func (s *configTestSuite) TestSASRefresh() {
	defer config.ResetConfig()
	assert := assert.New(s.T())
	az := &AzStorage{}
	opt := AzStorageOptions{}
	opt.AccountName = "abcd"
	opt.Container = "abcd"
	opt.AuthMode = "sas"

	opt.SaSKey = "xyz"
	opt.MaxRetries = 10
	opt.MaxTimeout = 10
	opt.BackoffTime = 10
	opt.MaxRetryDelay = 10
	opt.BlockSize = 5
	opt.MaxConcurrency = 20
	opt.DefaultTier = "hot"

	config.SetBool(compName+".set-content-type", true)
	config.SetBool(compName+".ca-cert-file", true)
	config.SetBool(compName+".debug-libcurl", true)

	az.storage = &BlockBlob{Auth: &azAuthBlobSAS{azAuthSAS: azAuthSAS{azAuthBase: azAuthBase{config: azAuthConfig{Endpoint: "abcd:://qreq!@#$%^&*()_)(*&^%$#"}}}}}
	err := ParseAndReadDynamicConfig(az, opt, true)
	assert.Nil(err)
}

func (s *configTestSuite) TestDisableSetAccessControlConfig() {
	defer config.ResetConfig()
	assert := assert.New(s.T())
	az := &AzStorage{}
	opt := AzStorageOptions{
		AccountName:             "testaccount",
		AccountType:             "adls",
		Container:               "test",
		AuthMode:                "key",
		AccountKey:              "testkey",
		DisableSetAccessControl: true,
	}

	err := ParseAndValidateConfig(az, opt)
	assert.Nil(err)
	assert.True(az.stConfig.disableSetAccessControl)

	// Test with false value
	opt.DisableSetAccessControl = false
	err = ParseAndValidateConfig(az, opt)
	assert.Nil(err)
	assert.False(az.stConfig.disableSetAccessControl)
}

func TestConfigTestSuite(t *testing.T) {
	suite.Run(t, new(configTestSuite))
}
