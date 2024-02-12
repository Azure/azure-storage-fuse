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
	"github.com/Azure/azure-storage-fuse/v2/common/log"
)

// azAuth : Interface to define a generic authentication type
type azAuth interface {
	getEndpoint() string
	setOption(key, value string)
	getCredential() interface{}
}

// getAzAuth returns a new AzAuth
// config: Defines the AzAuthConfig
func getAzAuth(config azAuthConfig) azAuth {
	log.Debug("azAuth::getAzAuth : account %s, account-type %s, protocol %s, endpoint %s",
		config.AccountName,
		config.AccountType,
		func(useHttp bool) string {
			if useHttp {
				return "http"
			}
			return "https"
		}(config.UseHTTP),
		config.Endpoint)

	if EAccountType.ADLS() == config.AccountType {
		return getAzAuthBfs(config)
	}
	return nil
}

func getAzAuthBfs(config azAuthConfig) azAuth {
	base := azAuthBase{config: config}
	if config.AuthMode == EAuthType.KEY() {
		return &azAuthBfsKey{
			azAuthKey{
				azAuthBase: base,
			},
		}
	} else if config.AuthMode == EAuthType.SAS() {
		return &azAuthBfsSAS{
			azAuthSAS{
				azAuthBase: base,
			},
		}
	} else if config.AuthMode == EAuthType.MSI() {
		return &azAuthBfsMSI{
			azAuthMSI{
				azAuthBase: base,
			},
		}
	} else if config.AuthMode == EAuthType.SPN() {
		return &azAuthBfsSPN{
			azAuthSPN{
				azAuthBase: base,
			},
		}
	} else {
		log.Crit("azAuth::getAzAuthBfs : Auth type %s not supported. Failed to create Auth object", config.AuthMode)
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
