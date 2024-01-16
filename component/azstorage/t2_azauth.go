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
	ActiveDirectoryEndpoint string

	Endpoint     string
	AuthResource string
}

// TODO: remove T2 suffix

// azAuth : Interface to define a generic authentication type
type azAuthT2 interface {
	getServiceClient(stConfig *AzStorageConfig) (interface{}, error)
}

// getAzAuth returns a new AzAuth
// config: Defines the AzAuthConfig
func getAzAuthT2(config azAuthConfig) azAuthT2 {
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

func getAzBlobAuth(config azAuthConfig) azAuthT2 {
	base := azAuthBaseT2{config: config}
	if config.AuthMode == EAuthType.KEY() {
		return &azAuthBlobKeyT2{
			azAuthKeyT2{
				azAuthBaseT2: base,
			},
		}
	} else if config.AuthMode == EAuthType.SAS() {
		return &azAuthBlobSAST2{
			azAuthSAST2{
				azAuthBaseT2: base,
			},
		}
	} else if config.AuthMode == EAuthType.MSI() {
		return &azAuthBlobMSIT2{
			azAuthMSIT2{
				azAuthBaseT2: base,
			},
		}
	} else if config.AuthMode == EAuthType.SPN() {
		return &azAuthBlobSPNT2{
			azAuthSPNT2{
				azAuthBaseT2: base,
			},
		}
	} else {
		log.Crit("azAuth::getAzBlobAuth : Auth type %s not supported. Failed to create Auth object", config.AuthMode)
	}
	return nil
}

func getAzDatalakeAuth(config azAuthConfig) azAuthT2 {
	base := azAuthBaseT2{config: config}
	if config.AuthMode == EAuthType.KEY() {
		return &azAuthDatalakeKey{
			azAuthKeyT2{
				azAuthBaseT2: base,
			},
		}
	} else if config.AuthMode == EAuthType.SAS() {
		return &azAuthDatalakeSAS{
			azAuthSAST2{
				azAuthBaseT2: base,
			},
		}
	} else if config.AuthMode == EAuthType.MSI() {
		return &azAuthDatalakeMSI{
			azAuthMSIT2{
				azAuthBaseT2: base,
			},
		}
	} else if config.AuthMode == EAuthType.SPN() {
		return &azAuthDatalakeSPN{
			azAuthSPNT2{
				azAuthBaseT2: base,
			},
		}
	} else {
		log.Crit("azAuth::getAzDatalakeAuth : Auth type %s not supported. Failed to create Auth object", config.AuthMode)
	}
	return nil
}

type azAuthBaseT2 struct {
	config azAuthConfig
}

// TODO: check ActiveDirectoryEndpoint and AuthResource part
func (base *azAuthBaseT2) getAzIdentityClientOptions() azcore.ClientOptions {
	opts := azcore.ClientOptions{}
	if base.config.ActiveDirectoryEndpoint != "" || base.config.AuthResource != "" {
		opts.Cloud = cloud.AzurePublic
		if base.config.ActiveDirectoryEndpoint != "" {
			log.Debug("azAuthBase::getAzIdentityClientOptions : ActiveDirectoryAuthorityHost = %s", base.config.ActiveDirectoryEndpoint)
			opts.Cloud.ActiveDirectoryAuthorityHost = base.config.ActiveDirectoryEndpoint
		}
		if base.config.AuthResource != "" {
			if val, ok := opts.Cloud.Services[cloud.ResourceManager]; ok {
				log.Debug("azAuthBase::getAzIdentityClientOptions : AuthResource = %s", base.config.AuthResource)
				val.Endpoint = base.config.AuthResource
				opts.Cloud.Services[cloud.ResourceManager] = val
			}
		}
	}

	return opts
}
