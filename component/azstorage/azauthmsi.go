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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/go-autorest/autorest/adal"

	"github.com/Azure/azure-storage-azcopy/v10/azbfs"
	"github.com/Azure/azure-storage-azcopy/v10/common"
	"github.com/Azure/azure-storage-blob-go/azblob"
)

// Verify that the Auth implement the correct AzAuth interfaces
var _ azAuth = &azAuthBlobMSI{}
var _ azAuth = &azAuthBfsMSI{}

type azAuthMSI struct {
	azAuthBase
}

func getNextExpiryTimer(token *adal.Token) time.Duration {
	delay := time.Duration(5+rand.Intn(120)) * time.Second
	return time.Until(token.Expires()) - delay
}

// fetchToken : Generates a token based on the config
func (azmsi *azAuthMSI) fetchToken(endpoint string) (*common.OAuthTokenInfo, error) {
	// Resource string is fixed and has no relation with any of the user inputs
	// This is not the resource URL, rather a way to identify the resource type and tenant
	// There are two options in the structure datalake and storage but datalake is not populated
	// and does not work in all types of clouds (US, German, China etc).
	// resource := azure.PublicCloud.ResourceIdentifiers.Datalake
	// resource := azure.PublicCloud.ResourceIdentifiers.Storage
	oAuthTokenInfo := &common.OAuthTokenInfo{
		Identity: true,
		IdentityInfo: common.IdentityInfo{
			ClientID: azmsi.config.ApplicationID,
			ObjectID: azmsi.config.ObjectID,
			MSIResID: azmsi.config.ResourceID},
		ActiveDirectoryEndpoint: endpoint,
	}

	token, err := oAuthTokenInfo.GetNewTokenFromMSI(context.Background())
	if err != nil {
		return nil, err
	}
	oAuthTokenInfo.Token = *token
	return oAuthTokenInfo, nil
}

// fetchTokenFromCLI : Generates a token using the Az Cli
func (azmsi *azAuthMSI) fetchTokenFromCLI() (*common.OAuthTokenInfo, error) {
	resource := "https://storage.azure.com"
	if azmsi.config.AuthResource != "" {
		resource = azmsi.config.AuthResource
	}

	commandLine := "az account get-access-token -o json --resource " + resource
	if azmsi.config.TenantID != "" {
		commandLine += " --tenant " + azmsi.config.TenantID
	}

	cliCmd := exec.CommandContext(context.Background(), "/bin/sh", "-c", commandLine)
	cliCmd.Dir = "/bin"
	cliCmd.Env = os.Environ()

	var stderr bytes.Buffer
	cliCmd.Stderr = &stderr
	output, err := cliCmd.Output()
	if err != nil {
		msg := stderr.String()
		var exErr *exec.ExitError
		if errors.As(err, &exErr) && exErr.ExitCode() == 127 || strings.HasPrefix(msg, "'az' is not recognized") {
			msg = "Azure CLI not found on path"
		}
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf(msg)
	}

	log.Info("azAuthMSI::fetchTokenFromCLI : Successfully fetched token from Azure CLI")
	log.Debug("azAuthMSI::fetchTokenFromCLI : Token: %s", output)
	t := struct {
		AccessToken      string `json:"accessToken"`
		Authority        string `json:"_authority"`
		ClientID         string `json:"_clientId"`
		ExpiresOn        string `json:"expiresOn"`
		IdentityProvider string `json:"identityProvider"`
		IsMRRT           bool   `json:"isMRRT"`
		RefreshToken     string `json:"refreshToken"`
		Resource         string `json:"resource"`
		TokenType        string `json:"tokenType"`
		UserID           string `json:"userId"`
	}{}

	err = json.Unmarshal(output, &t)
	if err != nil {
		return nil, err
	}
	// the Azure CLI's "expiresOn" is local time
	expiresOn, err := time.ParseInLocation("2006-01-02 15:04:05.999999", t.ExpiresOn, time.Local)
	if err != nil {
		return nil, fmt.Errorf("error parsing token expiration time %q: %v", t.ExpiresOn, err)
	}

	tokenInfo := &common.OAuthTokenInfo{
		Token: adal.Token{
			AccessToken:  t.AccessToken,
			RefreshToken: t.RefreshToken,
			ExpiresOn:    json.Number(strconv.FormatInt(expiresOn.Unix(), 10)),
			Resource:     t.Resource,
			Type:         t.TokenType,
		},
	}

	return tokenInfo, nil
}

type azAuthBlobMSI struct {
	azAuthMSI
}

// GetCredential : Get MSI based credentials for blob
func (azmsi *azAuthBlobMSI) getCredential() interface{} {
	// Generate the token based on configured inputs

	var token *common.OAuthTokenInfo = nil
	var err error = nil
	norefresh := false

	msi_endpoint := os.Getenv("MSI_ENDPOINT")
	if strings.Contains(msi_endpoint, "127.0.0.1:") || strings.Contains(msi_endpoint, "localhost:") ||
		strings.Contains(azmsi.config.ActiveDirectoryEndpoint, "127.0.0.1:") {
		// this might be AML workspace so try to get token using CLI
		log.Info("azAuthBlobMSI::getCredential : Potential AML workspace detected")
		token, err = azmsi.fetchTokenFromCLI()
		if err != nil {
			log.Err("azAuthBlobMSI::getCredential : %s", err.Error())
		} else if token != nil {
			norefresh = true
		}
	}

	if token == nil {
		log.Debug("azAuthBlobMSI::getCredential : Going for conventional fetchToken. MSI Endpoint : %s", msi_endpoint)
		token, err = azmsi.fetchToken(msi_endpoint)

		if token == nil {
			log.Debug("azAuthBlobMSI::getCredential : Going for conventional fetchToken without endpoint")
			token, err = azmsi.fetchToken("")
		}
	}

	if err != nil {
		// fmt.Println(token.AccessToken)
		log.Err("azAuthBlobMSI::getCredential : Failed to get credential [%s]", err.Error())
		return nil
	}

	var tc azblob.TokenCredential
	if norefresh {
		log.Info("azAuthBlobMSI::getCredential : MSI Token over CLI retrieved")
		log.Debug("azAuthBlobMSI::getCredential : Token: %s (%s)", token.AccessToken, token.Expires())
		// We are running in cli mode so token can not be refreshed, on expiry just get the new token
		tc = azblob.NewTokenCredential(token.AccessToken, func(tc azblob.TokenCredential) time.Duration {
			for failCount := 0; failCount < 5; failCount++ {
				newToken, err := azmsi.fetchTokenFromCLI()
				if err != nil {
					log.Err("azAuthBlobMSI::getCredential : Failed to refresh token attempt %d [%s]", failCount, err.Error())
					time.Sleep(time.Duration(rand.Intn(5)) * time.Second)
					continue
				}

				// set the new token value
				tc.SetToken(newToken.AccessToken)
				log.Info("azAuthBlobMSI::getCredential : New MSI Token over CLI retrieved")
				log.Debug("azAuthBlobMSI::getCredential : New Token: %s (%s)", newToken.AccessToken, newToken.Expires())

				// Get the next token slightly before the current one expires
				return getNextExpiryTimer(&newToken.Token)

			}
			log.Err("azAuthBlobMSI::getCredential : Failed to refresh token bailing out.")
			return 0
		})
	} else {
		log.Info("azAuthBlobMSI::getCredential : MSI Token retrieved")
		log.Debug("azAuthBlobMSI::getCredential : Token: %s (%s)", token.AccessToken, token.Expires())
		// Using token create the credential object, here also register a call back which refreshes the token
		tc = azblob.NewTokenCredential(token.AccessToken, func(tc azblob.TokenCredential) time.Duration {
			// token, err := azmsi.fetchToken(msi_endpoint)
			// if err != nil {
			// 	log.Err("azAuthBlobMSI::getCredential : Failed to fetch token [%s]", err.Error())
			// 	return 0
			// }
			for failCount := 0; failCount < 5; failCount++ {
				newToken, err := token.Refresh(context.Background())
				if err != nil {
					log.Err("azAuthBlobMSI::getCredential : Failed to refresh token attempt %d [%s]", failCount, err.Error())
					time.Sleep(time.Duration(rand.Intn(5)) * time.Second)
					continue
				}

				// set the new token value
				tc.SetToken(newToken.AccessToken)
				log.Info("azAuthBlobMSI::getCredential : New MSI Token retrieved")
				log.Debug("azAuthBlobMSI::getCredential : New Token: %s (%s)", newToken.AccessToken, newToken.Expires())

				// Get the next token slightly before the current one expires
				return getNextExpiryTimer(newToken)
			}
			log.Err("azAuthBlobMSI::getCredential : Failed to refresh token bailing out.")
			return 0
		})
	}

	return tc
}

type azAuthBfsMSI struct {
	azAuthMSI
}

// GetCredential : Get MSI based credentials for datalake
func (azmsi *azAuthBfsMSI) getCredential() interface{} {
	// Generate the token based on configured inputs
	var token *common.OAuthTokenInfo = nil
	var err error = nil
	norefresh := false

	msi_endpoint := os.Getenv("MSI_ENDPOINT")
	log.Info("azAuthBfsMSI::getCredential : MSI_ENDPOINT = %v", msi_endpoint)

	if strings.Contains(msi_endpoint, "127.0.0.1:") || strings.Contains(msi_endpoint, "localhost:") ||
		strings.Contains(azmsi.config.ActiveDirectoryEndpoint, "127.0.0.1:") {
		// this might be AML workspace so try to get token using CLI
		log.Info("azAuthBfsMSI::getCredential : Potential AML workspace detected")
		token, err = azmsi.fetchTokenFromCLI()
		if err != nil {
			log.Err("azAuthBfsMSI::getCredential : %s", err.Error())
		} else if token != nil {
			norefresh = true
		}
	}

	if token == nil {
		log.Debug("azAuthBfsMSI::getCredential : Going for conventional fetchToken. MSI Endpoint : %s", msi_endpoint)
		token, err = azmsi.fetchToken(msi_endpoint)

		if token == nil {
			log.Debug("azAuthBfsMSI::getCredential : Going for conventional fetchToken without endpoint")
			token, err = azmsi.fetchToken("")
		}
	}

	if err != nil {
		// fmt.Println(token.AccessToken)
		log.Err("azAuthBfsMSI::getCredential : Failed to get credential [%s]", err.Error())
		return nil
	}

	var tc azbfs.TokenCredential
	if norefresh {
		log.Info("azAuthBfsMSI::getCredential : MSI Token over CLI retrieved")
		log.Debug("azAuthBfsMSI::getCredential : Token: %s (%s)", token.AccessToken, token.Expires())
		// We are running in cli mode so token can not be refreshed, on expiry just get the new token
		tc = azbfs.NewTokenCredential(token.AccessToken, func(tc azbfs.TokenCredential) time.Duration {
			for failCount := 0; failCount < 5; failCount++ {
				newToken, err := azmsi.fetchTokenFromCLI()
				if err != nil {
					log.Err("azAuthBfsMSI::getCredential : Failed to refresh token attempt %d [%s]", failCount, err.Error())
					time.Sleep(time.Duration(rand.Intn(5)) * time.Second)
					continue
				}

				// set the new token value
				tc.SetToken(newToken.AccessToken)
				log.Info("azAuthBfsMSI::getCredential : New MSI Token over CLI retrieved")
				log.Debug("azAuthBfsMSI::getCredential : New Token: %s (%s)", newToken.AccessToken, newToken.Expires())

				// Get the next token slightly before the current one expires
				return getNextExpiryTimer(&newToken.Token)
			}
			log.Err("azAuthBfsMSI::getCredential : Failed to refresh token bailing out.")
			return 0
		})
	} else {
		log.Info("azAuthBfsMSI::getCredential : MSI Token retrieved")
		log.Debug("azAuthBfsMSI::getCredential : Token: %s (%s)", token.AccessToken, token.Expires())
		// Using token create the credential object, here also register a call back which refreshes the token
		tc = azbfs.NewTokenCredential(token.AccessToken, func(tc azbfs.TokenCredential) time.Duration {
			// token, err := azmsi.fetchToken(msi_endpoint)
			// if err != nil {
			// 	log.Err("azAuthBfsMSI::getCredential : Failed to fetch token [%s]", err.Error())
			// 	return 0
			// }
			for failCount := 0; failCount < 5; failCount++ {
				newToken, err := token.Refresh(context.Background())
				if err != nil {
					log.Err("azAuthBfsMSI::getCredential : Failed to refresh token attempt %d [%s]", failCount, err.Error())
					time.Sleep(time.Duration(rand.Intn(5)) * time.Second)
					continue
				}

				// set the new token value
				tc.SetToken(newToken.AccessToken)
				log.Info("azAuthBfsMSI::getCredential : New MSI Token retrieved")
				log.Debug("azAuthBfsMSI::getCredential : New Token: %s (%s)", newToken.AccessToken, newToken.Expires())

				// Get the next token slightly before the current one expires
				return getNextExpiryTimer(newToken)
			}
			log.Err("azAuthBfsMSI::getCredential : Failed to refresh token bailing out.")
			return 0
		})
	}

	return tc
}
