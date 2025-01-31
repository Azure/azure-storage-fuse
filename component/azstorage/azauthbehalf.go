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
	"context"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/service"
	serviceBfs "github.com/Azure/azure-sdk-for-go/sdk/storage/azdatalake/service"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
)

// Verify that the Auth implement the correct AzAuth interfaces
var _ azAuth = &azAuthBlobOnBehalf{}
var _ azAuth = &azAuthDatalakeOnBehalf{}

type azAuthOnBehalf struct {
	azAuthBase
	azOAuthBase
}

func (azonbehalf *azAuthOnBehalf) getTokenCredential() (azcore.TokenCredential, error) {
	opts := azonbehalf.getAzIdentityClientOptions(&azonbehalf.config)
	behalfOpts := &azidentity.OnBehalfOfCredentialOptions{
		ClientOptions: opts,
	}

	// Create MSI cred to fetch token
	msiOpts := &azidentity.ManagedIdentityCredentialOptions{
		ClientOptions: opts,
	}
	msiOpts.ID = azidentity.ClientID(azonbehalf.config.ApplicationID)
	cred, err := azidentity.NewManagedIdentityCredential(msiOpts)
	if err != nil {
		log.Err("azAuthOnBehalf::getTokenCredential : Failed to create managed identity credential [%s]", err.Error())
		return nil, err
	}

	scope := "https://vault.azure.net/.default"
	if azonbehalf.config.AADScope != "" {
		scope = azonbehalf.config.AADScope
	}

	getClientAssertions := func(context.Context) (string, error) {
		token, err := cred.GetToken(context.Background(), policy.TokenRequestOptions{
			Scopes: []string{scope},
		})

		if err != nil {
			log.Err("azAuthOnBehalf::getTokenCredential : Failed to get token from managed identity credential [%s]", err.Error())
			return "", err
		}

		return token.Token, nil
	}

	return azidentity.NewOnBehalfOfCredentialWithClientAssertions(
		azonbehalf.config.TenantID,
		azonbehalf.config.ClientID,
		azonbehalf.config.UserAssertion,
		getClientAssertions,
		behalfOpts)
}

type azAuthBlobOnBehalf struct {
	azAuthOnBehalf
}

// getServiceClient : returns SPN based service client for blob
func (azonbehalf *azAuthBlobOnBehalf) getServiceClient(stConfig *AzStorageConfig) (interface{}, error) {
	cred, err := azonbehalf.getTokenCredential()
	if err != nil {
		log.Err("azAuthBlobOnBehalf::getServiceClient : Failed to get token credential from client assertion [%s]", err.Error())
		return nil, err
	}

	opts, err := getAzBlobServiceClientOptions(stConfig)
	if err != nil {
		log.Err("azAuthBlobOnBehalf::getServiceClient : Failed to create client options [%s]", err.Error())
		return nil, err
	}

	svcClient, err := service.NewClient(azonbehalf.config.Endpoint, cred, opts)
	if err != nil {
		log.Err("azAuthBlobOnBehalf::getServiceClient : Failed to create service client [%s]", err.Error())
	}

	return svcClient, err
}

type azAuthDatalakeOnBehalf struct {
	azAuthOnBehalf
}

// getServiceClient : returns SPN based service client for blob
func (azonbehalf *azAuthDatalakeOnBehalf) getServiceClient(stConfig *AzStorageConfig) (interface{}, error) {
	cred, err := azonbehalf.getTokenCredential()
	if err != nil {
		log.Err("azAuthDatalakeOnBehalf::getServiceClient : Failed to get token credential from client assertion [%s]", err.Error())
		return nil, err
	}

	opts, err := getAzDatalakeServiceClientOptions(stConfig)
	if err != nil {
		log.Err("azAuthDatalakeOnBehalf::getServiceClient : Failed to create client options [%s]", err.Error())
		return nil, err
	}

	svcClient, err := serviceBfs.NewClient(azonbehalf.config.Endpoint, cred, opts)
	if err != nil {
		log.Err("azAuthDatalakeOnBehalf::getServiceClient : Failed to create service client [%s]", err.Error())
	}

	return svcClient, err
}
