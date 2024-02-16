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
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common/log"

	"github.com/Azure/azure-storage-azcopy/v10/azbfs"
	"github.com/Azure/go-autorest/autorest/adal"
	"github.com/Azure/go-autorest/autorest/azure"
)

// Verify that the Auth implement the correct AzAuth interfaces
var _ azAuth = &azAuthBfsSPN{}

type azAuthSPN struct {
	azAuthBase
}

func getNextExpiryTimerSPN(spt *adal.ServicePrincipalToken) time.Duration {
	delay := time.Duration(5+rand.Intn(120)) * time.Second
	return time.Until(spt.Token().Expires()) - delay
}

func (azspn *azAuthSPN) getAADEndpoint() string {
	if azspn.config.ActiveDirectoryEndpoint != "" {
		return azspn.config.ActiveDirectoryEndpoint
	}
	return azure.PublicCloud.ActiveDirectoryEndpoint
}

// fetchToken : Generates a token based on the config
func (azspn *azAuthSPN) fetchToken() (*adal.ServicePrincipalToken, error) {
	//  Use the configured AAD endpoint for token generation
	config, err := adal.NewOAuthConfig(azspn.getAADEndpoint(), azspn.config.TenantID)
	if err != nil {
		log.Err("AzAuthSPN::fetchToken : Failed to generate OAuth Config for SPN [%s]", err.Error())
		return nil, err
	}

	//  Create the resource URL
	resourceURL := azspn.config.AuthResource
	if resourceURL == "" {
		resourceURL = azspn.getEndpoint()
	}

	//  Generate the SPN token
	var spt *adal.ServicePrincipalToken
	if azspn.config.OAuthTokenFilePath != "" {
		log.Trace("AzAuthSPN::fetchToken : Going for fedrated token flow.")

		tokenReader := func() (string, error) {
			token, err := os.ReadFile(azspn.config.OAuthTokenFilePath)
			if err != nil {
				return "", fmt.Errorf("failed to read OAuth token file %s [%v]", azspn.config.OAuthTokenFilePath, err.Error())
			}
			return string(token), nil
		}

		spt, err = adal.NewServicePrincipalTokenFromFederatedTokenCallback(*config, azspn.config.ClientID, tokenReader, resourceURL)
		if err != nil {
			log.Err("AzAuthSPN::fetchToken : Failed to generate token for SPN [%s]", err.Error())
			return nil, err
		}
	} else {
		spt, err = adal.NewServicePrincipalToken(*config, azspn.config.ClientID, azspn.config.ClientSecret, resourceURL)
		if err != nil {
			log.Err("AzAuthSPN::fetchToken : Failed to generate token for SPN [%s]", err.Error())
			return nil, err
		}
	}

	return spt, nil
}

type azAuthBfsSPN struct {
	azAuthSPN
}

// GetCredential : Get SPN based credentials for datalake
func (azspn *azAuthBfsSPN) getCredential() interface{} {

	spt, err := azspn.fetchToken()
	if err != nil {
		log.Err("azAuthBfsSPN::getCredential : Failed to fetch token for SPN [%s]", err.Error())
		return nil
	}

	// Using token create the credential object, here also register a call back which refreshes the token
	tc := azbfs.NewTokenCredential(spt.Token().AccessToken, func(tc azbfs.TokenCredential) time.Duration {
		// spt, err = azspn.fetchToken()
		// if err != nil {
		// 	log.Err("azAuthBfsSPN::getCredential : Failed to fetch SPN token [%s]", err.Error())
		// 	return 0
		// }
		for failCount := 0; failCount < 5; failCount++ {
			err = spt.Refresh()
			if err != nil {
				log.Err("azAuthBfsSPN::getCredential : Failed to refresh token attempt %d [%s]", failCount, err.Error())
				time.Sleep(time.Duration(rand.Intn(5)) * time.Second)
				continue
			}

			// set the new token value
			tc.SetToken(spt.Token().AccessToken)
			log.Info("azAuthBfsSPN::getCredential : SPN Token retrieved")
			log.Debug("azAuthBfsSPN::getCredential : Token: %s (%s)", spt.Token().AccessToken, spt.Token().Expires())

			// Get the next token slightly before the current one expires
			return getNextExpiryTimerSPN(spt)
			// Test code to expire token every 30 seconds
			// return time.Until(time.Now()) + 30*time.Second
		}
		log.Err("azAuthBfsSPN::getCredential : Failed to refresh token bailing out.")
		return 0
	})

	return tc
}
