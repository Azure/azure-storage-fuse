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
	"errors"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/service"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azdatalake"
	serviceBfs "github.com/Azure/azure-sdk-for-go/sdk/storage/azdatalake/service"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
)

// Verify that the Auth implement the correct AzAuth interfaces
var _ azAuth = &azAuthBlobKey{}
var _ azAuth = &azAuthDatalakeKey{}

type azAuthKey struct {
	azAuthBase
}

type azAuthBlobKey struct {
	azAuthKey
}

// getServiceClient : returns shared key based service client for blob
func (azkey *azAuthBlobKey) getServiceClient(stConfig *AzStorageConfig) (any, error) {
	if azkey.config.AccountKey == "" {
		log.Err("azAuthBlobKey::getServiceClient : Shared key for account is empty, cannot authenticate user")
		return nil, errors.New("shared key for account is empty, cannot authenticate user")
	}

	cred, err := azblob.NewSharedKeyCredential(azkey.config.AccountName, azkey.config.AccountKey)
	if err != nil {
		log.Err("azAuthBlobKey::getServiceClient : Failed to create shared key credential [%s]", err.Error())
		return nil, err
	}

	opts, err := getAzBlobServiceClientOptions(stConfig)
	if err != nil {
		log.Err("azAuthBlobKey::getServiceClient : Failed to create client options [%s]", err.Error())
		return nil, err
	}

	svcClient, err := service.NewClientWithSharedKeyCredential(azkey.config.Endpoint, cred, opts)
	if err != nil {
		log.Err("azAuthBlobKey::getServiceClient : Failed to create service client [%s]", err.Error())
	}

	return svcClient, err
}

type azAuthDatalakeKey struct {
	azAuthKey
}

// getServiceClient : returns shared key based service client for datalake
func (azkey *azAuthDatalakeKey) getServiceClient(stConfig *AzStorageConfig) (any, error) {
	if azkey.config.AccountKey == "" {
		log.Err("azAuthDatalakeKey::getServiceClient : Shared key for account is empty, cannot authenticate user")
		return nil, errors.New("shared key for account is empty, cannot authenticate user")
	}

	cred, err := azdatalake.NewSharedKeyCredential(azkey.config.AccountName, azkey.config.AccountKey)
	if err != nil {
		log.Err("azAuthDatalakeKey::getServiceClient : Failed to create shared key credential [%s]", err.Error())
		return nil, err
	}

	opts, err := getAzDatalakeServiceClientOptions(stConfig)
	if err != nil {
		log.Err("azAuthDatalakeKey::getServiceClient : Failed to create client options [%s]", err.Error())
		return nil, err
	}

	svcClient, err := serviceBfs.NewClientWithSharedKeyCredential(azkey.config.Endpoint, cred, opts)
	if err != nil {
		log.Err("azAuthDatalakeKey::getServiceClient : Failed to create service client [%s]", err.Error())
	}

	return svcClient, err
}
