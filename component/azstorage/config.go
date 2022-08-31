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
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/Azure/azure-storage-fuse/v2/common/config"
	"github.com/Azure/azure-storage-fuse/v2/common/log"

	"github.com/Azure/azure-storage-blob-go/azblob"
	"github.com/JeffreyRichter/enum/enum"
)

//  AuthType Enum
type AuthType int

var EAuthType = AuthType(0).INVALID_AUTH()

func (AuthType) INVALID_AUTH() AuthType {
	return AuthType(0)
}

func (AuthType) KEY() AuthType {
	return AuthType(1)
}

func (AuthType) SAS() AuthType {
	return AuthType(2)
}

func (AuthType) SPN() AuthType {
	return AuthType(3)
}

func (AuthType) MSI() AuthType {
	return AuthType(4)
}

func (a AuthType) String() string {
	return enum.StringInt(a, reflect.TypeOf(a))
}

func (a *AuthType) Parse(s string) error {
	enumVal, err := enum.ParseInt(reflect.TypeOf(a), s, true, false)
	if enumVal != nil {
		*a = enumVal.(AuthType)
	}
	return err
}

//  AccountType Enum
type AccountType int

var EAccountType = AccountType(0).INVALID_ACC()

func (AccountType) INVALID_ACC() AccountType {
	return AccountType(0)
}

func (AccountType) BLOCK() AccountType {
	return AccountType(1)
}

func (AccountType) ADLS() AccountType {
	return AccountType(2)
}

func (f AccountType) String() string {
	return enum.StringInt(f, reflect.TypeOf(f))
}

func (a *AccountType) Parse(s string) error {
	enumVal, err := enum.ParseInt(reflect.TypeOf(a), s, true, false)
	if enumVal != nil {
		*a = enumVal.(AccountType)
	}
	return err
}

// Environment variable names
// Here we are not reading MSI_ENDPOINT and MSI_SECRET as they are read by go-sdk directly
// https://github.com/Azure/go-autorest/blob/a46566dfcbdc41e736295f94e9f690ceaf50094a/autorest/adal/token.go#L788
// newServicePrincipalTokenFromMSI : reads them directly from env
const (
	EnvAzStorageAccount            = "AZURE_STORAGE_ACCOUNT"
	EnvAzStorageAccountType        = "AZURE_STORAGE_ACCOUNT_TYPE"
	EnvAzStorageAccessKey          = "AZURE_STORAGE_ACCESS_KEY"
	EnvAzStorageSasToken           = "AZURE_STORAGE_SAS_TOKEN"
	EnvAzStorageIdentityClientId   = "AZURE_STORAGE_IDENTITY_CLIENT_ID"
	EnvAzStorageIdentityResourceId = "AZURE_STORAGE_IDENTITY_RESOURCE_ID"
	EnvAzStorageIdentityObjectId   = "AZURE_STORAGE_IDENTITY_OBJECT_ID"
	EnvAzStorageSpnTenantId        = "AZURE_STORAGE_SPN_TENANT_ID"
	EnvAzStorageSpnClientId        = "AZURE_STORAGE_SPN_CLIENT_ID"
	EnvAzStorageSpnClientSecret    = "AZURE_STORAGE_SPN_CLIENT_SECRET"
	EnvAzStorageAadEndpoint        = "AZURE_STORAGE_AAD_ENDPOINT"
	EnvAzStorageAuthType           = "AZURE_STORAGE_AUTH_TYPE"
	EnvAzStorageBlobEndpoint       = "AZURE_STORAGE_BLOB_ENDPOINT"
	EnvHttpProxy                   = "http_proxy"
	EnvHttpsProxy                  = "https_proxy"
	EnvAzStorageAccountContainer   = "AZURE_STORAGE_ACCOUNT_CONTAINER"
)

type AzStorageOptions struct {
	AccountType             string `config:"type" yaml:"type,omitempty"`
	UseHTTP                 bool   `config:"use-http" yaml:"use-http,omitempty"`
	AccountName             string `config:"account-name" yaml:"account-name,omitempty"`
	AccountKey              string `config:"account-key" yaml:"account-key,omitempty"`
	SaSKey                  string `config:"sas" yaml:"sas,omitempty"`
	ApplicationID           string `config:"appid" yaml:"appid,omitempty"`
	ResourceID              string `config:"resid" yaml:"resid,omitempty"`
	ObjectID                string `config:"objid" yaml:"objid,omitempty"`
	TenantID                string `config:"tenantid" yaml:"tenantid,omitempty"`
	ClientID                string `config:"clientid" yaml:"clientid,omitempty"`
	ClientSecret            string `config:"clientsecret" yaml:"clientsecret,omitempty"`
	ActiveDirectoryEndpoint string `config:"aadendpoint" yaml:"aadendpoint,omitempty"`
	Endpoint                string `config:"endpoint" yaml:"endpoint,omitempty"`
	AuthMode                string `config:"mode" yaml:"mode,omitempty"`
	Container               string `config:"container" yaml:"container,omitempty"`
	PrefixPath              string `config:"subdirectory" yaml:"subdirectory,omitempty"`
	BlockSize               int64  `config:"block-size-mb" yaml:"block-size-mb,omitempty"`
	MaxConcurrency          uint16 `config:"max-concurrency" yaml:"max-concurrency,omitempty"`
	DefaultTier             string `config:"tier" yaml:"tier,omitempty"`
	CancelListForSeconds    uint16 `config:"block-list-on-mount-sec" yaml:"block-list-on-mount-sec,omitempty"`
	MaxRetries              int32  `config:"max-retries" yaml:"max-retries,omitempty"`
	MaxTimeout              int32  `config:"max-retry-timeout-sec" yaml:"max-retry-timeout-sec,omitempty"`
	BackoffTime             int32  `config:"retry-backoff-sec" yaml:"retry-backoff-sec,omitempty"`
	MaxRetryDelay           int32  `config:"max-retry-delay-sec" yaml:"max-retry-delay-sec,omitempty"`
	HttpProxyAddress        string `config:"http-proxy" yaml:"http-proxy,omitempty"`
	HttpsProxyAddress       string `config:"https-proxy" yaml:"https-proxy,omitempty"`
	SdkTrace                bool   `config:"sdk-trace" yaml:"sdk-trace,omitempty"`
	FailUnsupportedOp       bool   `config:"fail-unsupported-op" yaml:"fail-unsupported-op,omitempty"`
	AuthResourceString      string `config:"auth-resource" yaml:"auth-resource,omitempty"`
	UpdateMD5               bool   `config:"update-md5" yaml:"update-md5"`
	ValidateMD5             bool   `config:"validate-md5" yaml:"validate-md5"`

	// v1 support
	UseAdls        bool   `config:"use-adls"`
	UseHTTPS       bool   `config:"use-https"`
	SetContentType bool   `config:"set-content-type"`
	CaCertFile     string `config:"ca-cert-file"`
}

//  RegisterEnvVariables : Register environment varilables
func RegisterEnvVariables() {
	config.BindEnv("azstorage.account-name", EnvAzStorageAccount)
	config.BindEnv("azstorage.type", EnvAzStorageAccountType)

	config.BindEnv("azstorage.account-key", EnvAzStorageAccessKey)

	config.BindEnv("azstorage.sas", EnvAzStorageSasToken)

	config.BindEnv("azstorage.appid", EnvAzStorageIdentityClientId)
	config.BindEnv("azstorage.resid", EnvAzStorageIdentityResourceId)

	config.BindEnv("azstorage.tenantid", EnvAzStorageSpnTenantId)
	config.BindEnv("azstorage.clientid", EnvAzStorageSpnClientId)
	config.BindEnv("azstorage.clientsecret", EnvAzStorageSpnClientSecret)
	config.BindEnv("azstorage.objid", EnvAzStorageIdentityObjectId)

	config.BindEnv("azstorage.aadendpoint", EnvAzStorageAadEndpoint)

	config.BindEnv("azstorage.endpoint", EnvAzStorageBlobEndpoint)

	config.BindEnv("azstorage.mode", EnvAzStorageAuthType)

	config.BindEnv("azstorage.http-proxy", EnvHttpProxy)
	config.BindEnv("azstorage.https-proxy", EnvHttpsProxy)

	config.BindEnv("azstorage.container", EnvAzStorageAccountContainer)
}

//    ----------- Config Parsing and Validation  ---------------

// formatEndPoint : add the protocol and missing "/" at the end to the endpoint
func formatEndPoint(endpoint string, http bool) string {
	correctedEP := endpoint

	// If the pvtEndpoint does not have protocol mentioned in front, pvtEndpoint parsing will fail while
	// creating URI also the string shall end with "/"
	if correctedEP != "" {
		if !strings.Contains(correctedEP, "://") {
			if http {
				correctedEP = "http://" + correctedEP
			} else {
				correctedEP = "https://" + correctedEP
			}
		}

		if correctedEP[len(correctedEP)-1] != '/' {
			correctedEP = correctedEP + "/"
		}
	}

	return correctedEP
}

// ParseAndValidateConfig : Parse and validate config
func ParseAndValidateConfig(az *AzStorage, opt AzStorageOptions) error {
	log.Trace("ParseAndValidateConfig : Parsing config")

	// Validate account name is present or not
	if opt.AccountName == "" {
		return errors.New("account name not provided")
	}
	az.stConfig.authConfig.AccountName = opt.AccountName

	// Validate account type property
	if opt.AccountType == "" {
		opt.AccountType = "block"
	}

	if opt.BlockSize != 0 {
		if opt.BlockSize > azblob.BlockBlobMaxStageBlockBytes {
			log.Err("block size is too large. Block size has to be smaller than %s Bytes", azblob.BlockBlobMaxStageBlockBytes)
			return errors.New("block size is too large")
		}
		az.stConfig.blockSize = opt.BlockSize * 1024 * 1024
	}

	if config.IsSet(compName + ".use-adls") {
		if opt.UseAdls {
			az.stConfig.authConfig.AccountType = az.stConfig.authConfig.AccountType.ADLS()
		} else {
			az.stConfig.authConfig.AccountType = az.stConfig.authConfig.AccountType.BLOCK()
		}
	} else {
		var accountType AccountType
		err := accountType.Parse(opt.AccountType)
		if err != nil {
			log.Err("ParseAndValidateConfig : Failed to parse account type %s", opt.AccountType)
			return errors.New("invalid account type")
		}

		if accountType == EAccountType.INVALID_ACC() {
			log.Err("ParseAndValidateConfig : Invalid account type %s", opt.AccountType)
			return errors.New("invalid account type")
		}

		az.stConfig.authConfig.AccountType = accountType
	}

	// Validate container name is present or not
	err := config.UnmarshalKey("mount-all-containers", &az.stConfig.mountAllContainers)
	if err != nil {
		log.Err("ParseAndValidateConfig : Failed to detect mount-all-container")
	}

	if !az.stConfig.mountAllContainers && opt.Container == "" {
		return errors.New("container name not provided")
	}

	az.stConfig.container = opt.Container

	if config.IsSet(compName + ".use-https") {
		opt.UseHTTP = !opt.UseHTTPS
	}

	// Validate endpoint
	if opt.Endpoint == "" {
		log.Warn("ParseAndValidateConfig : account endpoint not provided, assuming the default .core.windows.net style endpoint")
		if az.stConfig.authConfig.AccountType == EAccountType.BLOCK() {
			opt.Endpoint = fmt.Sprintf("%s.blob.core.windows.net", opt.AccountName)
		} else if az.stConfig.authConfig.AccountType == EAccountType.ADLS() {
			opt.Endpoint = fmt.Sprintf("%s.dfs.core.windows.net", opt.AccountName)
		}
	}
	az.stConfig.authConfig.Endpoint = opt.Endpoint
	az.stConfig.authConfig.Endpoint = formatEndPoint(az.stConfig.authConfig.Endpoint, opt.UseHTTP)

	az.stConfig.authConfig.ActiveDirectoryEndpoint = opt.ActiveDirectoryEndpoint
	az.stConfig.authConfig.ActiveDirectoryEndpoint = formatEndPoint(az.stConfig.authConfig.ActiveDirectoryEndpoint, false)

	// If subdirectory is mounted, take the prefix path
	az.stConfig.prefixPath = opt.PrefixPath

	// Block list call on mount for given amount of time
	az.stConfig.cancelListForSeconds = opt.CancelListForSeconds

	httpProxyProvided := opt.HttpProxyAddress != ""
	httpsProxyProvided := opt.HttpsProxyAddress != ""

	// Set whether to use http or https and proxy
	if opt.UseHTTP {
		az.stConfig.authConfig.UseHTTP = true
		if httpProxyProvided {
			az.stConfig.proxyAddress = opt.HttpProxyAddress
		} else if httpsProxyProvided {
			az.stConfig.proxyAddress = opt.HttpsProxyAddress
		}
	} else {
		if httpsProxyProvided {
			az.stConfig.proxyAddress = opt.HttpsProxyAddress
		} else {
			if httpProxyProvided {
				log.Err("ParseAndValidateConfig : `http-proxy` Invalid : must set `use-http: true` in your config file")
				return errors.New("`http-proxy` Invalid : must set `use-http: true` in your config file")
			}
		}
	}
	log.Info("ParseAndValidateConfig : using the following proxy address from the config file: %s", az.stConfig.proxyAddress)

	az.stConfig.sdkTrace = opt.SdkTrace

	log.Info("ParseAndValidateConfig : sdk logging from the config file: %t", az.stConfig.sdkTrace)

	err = ParseAndReadDynamicConfig(az, opt, false)
	if err != nil {
		return err
	}

	var authType AuthType
	if opt.AuthMode == "" {
		opt.AuthMode = "key"
	}

	err = authType.Parse(opt.AuthMode)
	if err != nil {
		log.Err("ParseAndValidateConfig : Invalid auth type %s", opt.AccountType)
		return errors.New("invalid auth type")
	}

	switch authType {
	case EAuthType.KEY():
		az.stConfig.authConfig.AuthMode = EAuthType.KEY()
		if opt.AccountKey == "" {
			return errors.New("storage key not provided")
		}
		az.stConfig.authConfig.AccountKey = opt.AccountKey
	case EAuthType.SAS():
		az.stConfig.authConfig.AuthMode = EAuthType.SAS()
		if opt.SaSKey == "" {
			return errors.New("SAS key not provided")
		}
		az.stConfig.authConfig.SASKey = sanitizeSASKey(opt.SaSKey)
	case EAuthType.MSI():
		az.stConfig.authConfig.AuthMode = EAuthType.MSI()
		if opt.ApplicationID == "" && opt.ResourceID == "" {
			//lint:ignore ST1005 ignore
			return errors.New("Application ID and Resource ID not provided")
		}
		az.stConfig.authConfig.ApplicationID = opt.ApplicationID
		az.stConfig.authConfig.ResourceID = opt.ResourceID

	case EAuthType.SPN():
		az.stConfig.authConfig.AuthMode = EAuthType.SPN()
		if opt.ClientID == "" || opt.ClientSecret == "" || opt.TenantID == "" {
			//lint:ignore ST1005 ignore
			return errors.New("Client ID, Tenant ID or Client Secret not provided")
		}
		az.stConfig.authConfig.ClientID = opt.ClientID
		az.stConfig.authConfig.ClientSecret = opt.ClientSecret
		az.stConfig.authConfig.TenantID = opt.TenantID
	default:
		log.Err("ParseAndValidateConfig : Invalid auth mode %s", opt.AuthMode)
		return errors.New("invalid auth mode")
	}
	az.stConfig.authConfig.AuthResource = opt.AuthResourceString

	// Retry policy configuration
	// A user provided value of 0 doesn't make sense for MaxRetries, MaxTimeout, BackoffTime, or MaxRetryDelay.
	az.stConfig.maxRetries = 3
	az.stConfig.maxTimeout = 3600
	az.stConfig.backoffTime = 1
	az.stConfig.maxRetryDelay = 3
	if opt.MaxRetries != 0 {
		az.stConfig.maxRetries = opt.MaxRetries
	}
	if opt.MaxTimeout != 0 {
		az.stConfig.maxTimeout = opt.MaxTimeout
	}
	if opt.BackoffTime != 0 {
		az.stConfig.backoffTime = opt.BackoffTime
	}
	if opt.MaxRetryDelay != 0 {
		az.stConfig.maxRetryDelay = opt.MaxRetryDelay
	}

	if config.IsSet(compName + ".set-content-type") {
		log.Warn("unsupported v1 CLI parameter: set-content-type is always true in blobfuse2.")
	}
	if config.IsSet(compName + ".ca-cert-file") {
		log.Warn("unsupported v1 CLI parameter: ca-cert-file is not supported in blobfuse2. Use the default ca cert path for your environment.")
	}
	if config.IsSet(compName + ".debug-libcurl") {
		log.Warn("unsupported v1 CLI parameter: debug-libcurl is not applicable in blobfuse2.")
	}

	log.Info("ParseAndValidateConfig : Account: %s, Container: %s, AccountType: %s, Auth: %s, Prefix: %s, Endpoint: %s, ListBlock: %d, MD5 : %v %v",
		az.stConfig.authConfig.AccountName, az.stConfig.container, az.stConfig.authConfig.AccountType, az.stConfig.authConfig.AuthMode,
		az.stConfig.prefixPath, az.stConfig.authConfig.Endpoint, az.stConfig.cancelListForSeconds, az.stConfig.validateMD5, az.stConfig.updateMD5)

	log.Info("ParseAndValidateConfig : Retry Config: Retry count %d, Max Timeout %d, BackOff Time %d, Max Delay %d",
		az.stConfig.maxRetries, az.stConfig.maxTimeout, az.stConfig.backoffTime, az.stConfig.maxRetryDelay)

	return nil
}

// ParseAndReadDynamicConfig : On config change read only the required config
func ParseAndReadDynamicConfig(az *AzStorage, opt AzStorageOptions, reload bool) error {
	log.Trace("ParseAndReadDynamicConfig : Reparsing config")

	// If block size and max concurrency is configured use those
	// A user provided value of 0 doesn't make sense for BlockSize, or MaxConcurrency.
	if opt.BlockSize != 0 {
		az.stConfig.blockSize = opt.BlockSize * 1024 * 1024
	}

	if opt.MaxConcurrency != 0 {
		az.stConfig.maxConcurrency = opt.MaxConcurrency
	}

	// Populate default tier
	if opt.DefaultTier != "" {
		az.stConfig.defaultTier = getAccessTierType(opt.DefaultTier)
	}

	az.stConfig.ignoreAccessModifiers = !opt.FailUnsupportedOp
	az.stConfig.validateMD5 = opt.ValidateMD5
	az.stConfig.updateMD5 = opt.UpdateMD5

	// Auth related reconfig
	switch opt.AuthMode {
	case "sas":
		az.stConfig.authConfig.AuthMode = EAuthType.SAS()
		if opt.SaSKey == "" {
			return errors.New("SAS key not provided")
		}

		oldSas := az.stConfig.authConfig.SASKey
		az.stConfig.authConfig.SASKey = sanitizeSASKey(opt.SaSKey)

		if reload {
			log.Info("ParseAndReadDynamicConfig : SAS Key updated")

			if err := az.storage.NewCredentialKey("saskey", az.stConfig.authConfig.SASKey); err != nil {
				az.stConfig.authConfig.SASKey = oldSas
				_ = az.storage.NewCredentialKey("saskey", az.stConfig.authConfig.SASKey)
				return errors.New("SAS key update failure")
			}
		}
	}

	return nil
}
