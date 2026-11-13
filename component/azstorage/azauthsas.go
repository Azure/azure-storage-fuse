// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package azstorage

import (
	"errors"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/service"
	serviceBfs "github.com/Azure/azure-sdk-for-go/sdk/storage/azdatalake/service"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
)

// Verify that the Auth implement the correct AzAuth interfaces
var _ azAuth = &azAuthBlobSAS{}
var _ azAuth = &azAuthDatalakeSAS{}

type azAuthSAS struct {
	azAuthBase
}

// SetOption : Sets the sas key information for the SAS auth.
func (azsas *azAuthSAS) setOption(key, value string) {
	if key == "saskey" {
		azsas.config.SASKey = value
	}
}

// GetEndpoint : Gets the SAS endpoint
func (azsas *azAuthSAS) getEndpoint() string {
	return azsas.config.Endpoint + "?" + strings.TrimLeft(azsas.config.SASKey, "?")
}

type azAuthBlobSAS struct {
	azAuthSAS
}

// getServiceClient : returns SAS based service client for blob
func (azsas *azAuthBlobSAS) getServiceClient(stConfig *AzStorageConfig) (any, error) {
	if azsas.config.SASKey == "" {
		log.Err("azAuthBlobSAS::getServiceClient : SAS key for account is empty, cannot authenticate user")
		return nil, errors.New("sas key for account is empty, cannot authenticate user")
	}

	opts, err := getAzBlobServiceClientOptions(stConfig)
	if err != nil {
		log.Err("azAuthBlobSAS::getServiceClient : Failed to create client options [%s]", err.Error())
		return nil, err
	}

	svcClient, err := service.NewClientWithNoCredential(azsas.getEndpoint(), opts)
	if err != nil {
		log.Err("azAuthBlobSAS::getServiceClient : Failed to create service client [%s]", err.Error())
	}

	return svcClient, err
}

type azAuthDatalakeSAS struct {
	azAuthSAS
}

// getServiceClient : returns SAS based service client for datalake
func (azsas *azAuthDatalakeSAS) getServiceClient(stConfig *AzStorageConfig) (any, error) {
	if azsas.config.SASKey == "" {
		log.Err("azAuthDatalakeSAS::getServiceClient : SAS key for account is empty, cannot authenticate user")
		return nil, errors.New("sas key for account is empty, cannot authenticate user")
	}

	opts, err := getAzDatalakeServiceClientOptions(stConfig)
	if err != nil {
		log.Err("azAuthDatalakeSAS::getServiceClient : Failed to create client options [%s]", err.Error())
		return nil, err
	}

	svcClient, err := serviceBfs.NewClientWithNoCredential(azsas.getEndpoint(), opts)
	if err != nil {
		log.Err("azAuthDatalakeSAS::getServiceClient : Failed to create service client [%s]", err.Error())
	}

	return svcClient, err
}
