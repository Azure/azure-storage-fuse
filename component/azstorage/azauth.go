/*
    _____           _____   _____   ____          ______  _____  ------
   |     |  |      |     | |     | |     |     | |       |            |
   |     |  |      |     | |     | |     |     | |       |            |
   | --- |  |      |     | |-----| |---- |     | |-----| |-----  ------
   |     |  |      |     | |     | |     |     |       | |       |
   | ____|  |_____ | ____| | ____| |     |_____|  _____| |_____  |_____


   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2020-2024 Microsoft Corporation. All rights reserved.
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
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/cloud"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
)

// AzAuthConfig : Config to authenticate to storage
type azAuthConfig struct {
	// Account
	AccountName string
	UseHTTP     bool
	AccountType AccountType
	AuthMode    AuthType

	// Key config
	AccountKey string

	// SAS config
	SASKey string

	// MSI config
	ApplicationID string
	ResourceID    string
	ObjectID      string

	// SPN config
	TenantID                string
	ClientID                string
	ClientSecret            string
	OAuthTokenFilePath      string
	WorkloadIdentityToken   string
	ActiveDirectoryEndpoint string

	Endpoint     string
	AuthResource string
}

// azAuth : Interface to define a generic authentication type
type azAuth interface {
	getEndpoint() string
	setOption(key, value string)
	getServiceClient(stConfig *AzStorageConfig) (interface{}, error)
}

// getAzAuth returns a new AzAuth
// config: Defines the AzAuthConfig
func getAzAuth(config azAuthConfig) azAuth {
	log.Debug("azAuth::getAzAuth : Account: %s, AccountType: %s, Protocol: %s, Endpoint: %s",
		config.AccountName,
		config.AccountType,
		func(useHttp bool) string {
			if useHttp {
				return "http"
			}
			return "https"
		}(config.UseHTTP),
		config.Endpoint)

	if EAccountType.BLOCK() == config.AccountType {
		return getAzBlobAuth(config)
	} else if EAccountType.ADLS() == config.AccountType {
		return getAzDatalakeAuth(config)
	}
	return nil
}

func getAzBlobAuth(config azAuthConfig) azAuth {
	base := azAuthBase{config: config}
	if config.AuthMode == EAuthType.KEY() {
		return &azAuthBlobKey{
			azAuthKey{
				azAuthBase: base,
			},
		}
	} else if config.AuthMode == EAuthType.SAS() {
		return &azAuthBlobSAS{
			azAuthSAS{
				azAuthBase: base,
			},
		}
	} else if config.AuthMode == EAuthType.MSI() {
		return &azAuthBlobMSI{
			azAuthMSI{
				azAuthBase: base,
			},
		}
	} else if config.AuthMode == EAuthType.SPN() {
		return &azAuthBlobSPN{
			azAuthSPN{
				azAuthBase: base,
			},
		}
	} else if config.AuthMode == EAuthType.AZCLI() {
		return &azAuthBlobCLI{
			azAuthCLI{
				azAuthBase: base,
			},
		}
	} else {
		log.Crit("azAuth::getAzBlobAuth : Auth type %s not supported. Failed to create Auth object", config.AuthMode)
	}
	return nil
}

func getAzDatalakeAuth(config azAuthConfig) azAuth {
	base := azAuthBase{config: config}
	if config.AuthMode == EAuthType.KEY() {
		return &azAuthDatalakeKey{
			azAuthKey{
				azAuthBase: base,
			},
		}
	} else if config.AuthMode == EAuthType.SAS() {
		return &azAuthDatalakeSAS{
			azAuthSAS{
				azAuthBase: base,
			},
		}
	} else if config.AuthMode == EAuthType.MSI() {
		return &azAuthDatalakeMSI{
			azAuthMSI{
				azAuthBase: base,
			},
		}
	} else if config.AuthMode == EAuthType.SPN() {
		return &azAuthDatalakeSPN{
			azAuthSPN{
				azAuthBase: base,
			},
		}
	} else if config.AuthMode == EAuthType.AZCLI() {
		return &azAuthDatalakeCLI{
			azAuthCLI{
				azAuthBase: base,
			},
		}
	} else {
		log.Crit("azAuth::getAzDatalakeAuth : Auth type %s not supported. Failed to create Auth object", config.AuthMode)
	}
	return nil
}

type azAuthBase struct {
	config azAuthConfig
}

// SetOption : Sets any optional information for the auth.
func (base *azAuthBase) setOption(_, _ string) {}

// GetEndpoint : Gets the endpoint
func (base *azAuthBase) getEndpoint() string {
	return base.config.Endpoint
}

// this type is included in OAuth modes - spn and msi
type azOAuthBase struct{}

// TODO:: track2 : check ActiveDirectoryEndpoint and AuthResource part
func (base *azOAuthBase) getAzIdentityClientOptions(config *azAuthConfig) azcore.ClientOptions {
	if config == nil {
		log.Err("azAuth::getAzIdentityClientOptions : azAuthConfig is nil")
		return azcore.ClientOptions{}
	}
	opts := azcore.ClientOptions{
		Cloud:   getCloudConfiguration(config.Endpoint),
		Logging: getSDKLogOptions(),
	}

	if config.ActiveDirectoryEndpoint != "" {
		log.Debug("azAuthBase::getAzIdentityClientOptions : ActiveDirectoryAuthorityHost = %s", config.ActiveDirectoryEndpoint)
		opts.Cloud.ActiveDirectoryAuthorityHost = config.ActiveDirectoryEndpoint
	}
	if config.AuthResource != "" {
		if val, ok := opts.Cloud.Services[cloud.ResourceManager]; ok {
			log.Debug("azAuthBase::getAzIdentityClientOptions : AuthResource = %s", config.AuthResource)
			val.Endpoint = config.AuthResource
			opts.Cloud.Services[cloud.ResourceManager] = val
		}
	}

	return opts
}
