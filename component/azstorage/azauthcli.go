// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package azstorage

import (
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/service"
	serviceBfs "github.com/Azure/azure-sdk-for-go/sdk/storage/azdatalake/service"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
)

// Verify that the Auth implement the correct AzAuth interfaces
var _ azAuth = &azAuthBlobCLI{}
var _ azAuth = &azAuthDatalakeCLI{}

type azAuthCLI struct {
	azAuthBase
}

func (azcli *azAuthCLI) getTokenCredential() (azcore.TokenCredential, error) {
	cred, err := azidentity.NewAzureCLICredential(nil)
	return cred, err
}

type azAuthBlobCLI struct {
	azAuthCLI
}

// getServiceClient : returns service client for blob using azcli as authentication mode
func (azcli *azAuthBlobCLI) getServiceClient(stConfig *AzStorageConfig) (any, error) {
	cred, err := azcli.getTokenCredential()
	if err != nil {
		log.Err("azAuthBlobCLI::getServiceClient : Failed to get token credential from azcli [%s]", err.Error())
		return nil, err
	}

	opts, err := getAzBlobServiceClientOptions(stConfig)
	if err != nil {
		log.Err("azAuthBlobCLI::getServiceClient : Failed to create client options [%s]", err.Error())
		return nil, err
	}

	svcClient, err := service.NewClient(azcli.config.Endpoint, cred, opts)
	if err != nil {
		log.Err("azAuthBlobCLI::getServiceClient : Failed to create service client [%s]", err.Error())
	}

	return svcClient, err
}

type azAuthDatalakeCLI struct {
	azAuthCLI
}

// getServiceClient : returns service client for datalake using azcli as authentication mode
func (azcli *azAuthDatalakeCLI) getServiceClient(stConfig *AzStorageConfig) (any, error) {
	cred, err := azcli.getTokenCredential()
	if err != nil {
		log.Err("azAuthDatalakeCLI::getServiceClient : Failed to get token credential from azcli [%s]", err.Error())
		return nil, err
	}

	opts, err := getAzDatalakeServiceClientOptions(stConfig)
	if err != nil {
		log.Err("azAuthDatalakeCLI::getServiceClient : Failed to create client options [%s]", err.Error())
		return nil, err
	}

	svcClient, err := serviceBfs.NewClient(azcli.config.Endpoint, cred, opts)
	if err != nil {
		log.Err("azAuthDatalakeCLI::getServiceClient : Failed to create service client [%s]", err.Error())
	}

	return svcClient, err
}
