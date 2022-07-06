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
	"blobfuse2/common/log"
	"time"

	"github.com/Azure/azure-storage-azcopy/v10/azbfs"
	"github.com/Azure/azure-storage-blob-go/azblob"
	"github.com/Azure/go-autorest/autorest/adal"
	"github.com/Azure/go-autorest/autorest/azure"
)

// Verify that the Auth implement the correct AzAuth interfaces
var _ azAuth = &azAuthBlobMSI{}
var _ azAuth = &azAuthBfsMSI{}

type azAuthMSI struct {
	azAuthBase
}

// fetchToken : Generates a token based on the config
func (azmsi *azAuthMSI) fetchToken() (*adal.ServicePrincipalToken, error) {
	// Resource string is fixed and has no relation with any of the user inputs
	// This is not the resource URL, rather a way to identify the resource type and tenant
	// There are two options in the structure datalake and storage but datalake is not populated
	// and does not work in all types of clouds (US, German, China etc).
	// resource := azure.PublicCloud.ResourceIdentifiers.Datalake
	resource := azure.PublicCloud.ResourceIdentifiers.Storage

	log.Info("AzAuthMSI::fetchToken : Resource : %s", resource)

	spt, err := adal.NewServicePrincipalTokenFromManagedIdentity(resource, &adal.ManagedIdentityOptions{
		ClientID:           azmsi.config.ApplicationID,
		IdentityResourceID: azmsi.config.ResourceID,
	}, func(token adal.Token) error { return nil })

	if err != nil {
		log.Err("AzAuthMSI::fetchToken : Failed to generate MSI token (%s)", err.Error())
		return nil, err
	}

	return spt, nil
}

type azAuthBlobMSI struct {
	azAuthMSI
}

// GetCredential : Get MSI based credentials for blob
func (azmsi *azAuthBlobMSI) getCredential() interface{} {
	// Generate the token based on configured inputs
	spt, err := azmsi.fetchToken()
	if err != nil {
		return nil
	}

	// Using token create the credential object, here also register a call back which refreshes the token
	tc := azblob.NewTokenCredential(spt.Token().AccessToken, func(tc azblob.TokenCredential) time.Duration {
		err := spt.Refresh()
		if err != nil {
			log.Err("azAuthBlobMSI::getCredential : Failed to refresh token (%s)", err.Error())
			return 0
		}

		// set the new token value
		tc.SetToken(spt.Token().AccessToken)
		log.Debug("azAuthBlobMSI::getCredential : MSI Token retrieved %s (%d)", spt.Token().AccessToken, spt.Token().Expires())

		// Get the next token slightly before the current one expires
		return time.Until(spt.Token().Expires()) - 10*time.Second
	})

	return tc
}

type azAuthBfsMSI struct {
	azAuthMSI
}

// GetCredential : Get MSI based credentials for datalake
func (azmsi *azAuthBfsMSI) getCredential() interface{} {
	// Generate the token based on configured inputs
	spt, err := azmsi.fetchToken()
	if err != nil {
		return nil
	}

	// Using token create the credential object, here also register a call back which refreshes the token
	tc := azbfs.NewTokenCredential(spt.Token().AccessToken, func(tc azbfs.TokenCredential) time.Duration {
		err := spt.Refresh()
		if err != nil {
			log.Err("azAuthBfsMSI::getCredential : Failed to refresh token (%s)", err.Error())
			return 0
		}

		// set the new token value
		tc.SetToken(spt.Token().AccessToken)
		log.Debug("azAuthBfsMSI::getCredential : MSI Token retrieved %s (%d)", spt.Token().AccessToken, spt.Token().Expires())

		// Get the next token slightly before the current one expires
		return time.Until(spt.Token().Expires()) - 10*time.Second
	})

	return tc
}
