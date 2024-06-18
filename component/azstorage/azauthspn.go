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
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/service"
	serviceBfs "github.com/Azure/azure-sdk-for-go/sdk/storage/azdatalake/service"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
)

// Verify that the Auth implement the correct AzAuth interfaces
var _ azAuth = &azAuthBlobSPN{}
var _ azAuth = &azAuthDatalakeSPN{}

type azAuthSPN struct {
	azAuthBase
	azOAuthBase
}

func (azspn *azAuthSPN) getTokenCredential() (azcore.TokenCredential, error) {
	var cred azcore.TokenCredential
	var err error

	clOpts := azspn.getAzIdentityClientOptions(&azspn.config)
	if azspn.config.OAuthTokenFilePath != "" {
		log.Trace("AzAuthSPN::getTokenCredential : Going for fedrated token flow")

		// TODO:: track2 : test this in Azure Kubernetes setup
		cred, err = azidentity.NewWorkloadIdentityCredential(&azidentity.WorkloadIdentityCredentialOptions{
			ClientOptions: clOpts,
			ClientID:      azspn.config.ClientID,
			TenantID:      azspn.config.TenantID,
			TokenFilePath: azspn.config.OAuthTokenFilePath,
		})
		if err != nil {
			log.Err("AzAuthSPN::getTokenCredential : Failed to generate token for SPN [%s]", err.Error())
			return nil, err
		}
	} else {
		log.Trace("AzAuthSPN::getTokenCredential : Using client secret for fetching token")

		cred, err = azidentity.NewClientSecretCredential(azspn.config.TenantID, azspn.config.ClientID, azspn.config.ClientSecret, &azidentity.ClientSecretCredentialOptions{
			ClientOptions: clOpts,
		})
		if err != nil {
			log.Err("AzAuthSPN::getTokenCredential : Failed to generate token for SPN [%s]", err.Error())
			return nil, err
		}
	}

	return cred, err
}

type azAuthBlobSPN struct {
	azAuthSPN
}

// getServiceClient : returns SPN based service client for blob
func (azspn *azAuthBlobSPN) getServiceClient(stConfig *AzStorageConfig) (interface{}, error) {
	cred, err := azspn.getTokenCredential()
	if err != nil {
		log.Err("azAuthBlobSPN::getServiceClient : Failed to get token credential from SPN [%s]", err.Error())
		return nil, err
	}

	opts, err := getAzBlobServiceClientOptions(stConfig)
	if err != nil {
		log.Err("azAuthBlobSPN::getServiceClient : Failed to create client options [%s]", err.Error())
		return nil, err
	}

	svcClient, err := service.NewClient(azspn.config.Endpoint, cred, opts)
	if err != nil {
		log.Err("azAuthBlobSPN::getServiceClient : Failed to create service client [%s]", err.Error())
	}

	return svcClient, err
}

type azAuthDatalakeSPN struct {
	azAuthSPN
}

// getServiceClient : returns SPN based service client for datalake
func (azspn *azAuthDatalakeSPN) getServiceClient(stConfig *AzStorageConfig) (interface{}, error) {
	cred, err := azspn.getTokenCredential()
	if err != nil {
		log.Err("azAuthDatalakeSPN::getServiceClient : Failed to get token credential from SPN [%s]", err.Error())
		return nil, err
	}

	opts, err := getAzDatalakeServiceClientOptions(stConfig)
	if err != nil {
		log.Err("azAuthDatalakeSPN::getServiceClient : Failed to create client options [%s]", err.Error())
		return nil, err
	}

	svcClient, err := serviceBfs.NewClient(azspn.config.Endpoint, cred, opts)
	if err != nil {
		log.Err("azAuthDatalakeSPN::getServiceClient : Failed to create service client [%s]", err.Error())
	}

	return svcClient, err
}
