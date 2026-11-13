// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

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
