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
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/service"
	serviceBfs "github.com/Azure/azure-sdk-for-go/sdk/storage/azdatalake/service"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
)

// Verify that the Auth implement the correct AzAuth interfaces
var _ azAuth = &azAuthBlobMSI{}
var _ azAuth = &azAuthDatalakeMSI{}

type azAuthMSI struct {
	azAuthBase
	azOAuthBase
}

func (azmsi *azAuthMSI) getTokenCredential() (azcore.TokenCredential, error) {
	opts := azmsi.getAzIdentityClientOptions(&azmsi.config)

	msiOpts := &azidentity.ManagedIdentityCredentialOptions{
		ClientOptions: opts,
	}

	if azmsi.config.ApplicationID != "" {
		msiOpts.ID = azidentity.ClientID(azmsi.config.ApplicationID)
	} else if azmsi.config.ResourceID != "" {
		msiOpts.ID = azidentity.ResourceID(azmsi.config.ResourceID)
	} else if azmsi.config.ObjectID != "" {
		// Object id is supported by azidentity hence commenting the earlier code
		msiOpts.ID = azidentity.ObjectID(azmsi.config.ObjectID)

		// login using azcli
		// return azmsi.getTokenCredentialUsingCLI()
	}

	cred, err := azidentity.NewManagedIdentityCredential(msiOpts)
	return cred, err
}

/*
func (azmsi *azAuthMSI) getTokenCredentialUsingCLI() (azcore.TokenCredential, error) {
	command := "az login --identity --username " + azmsi.config.ObjectID

	cliCmd := exec.CommandContext(context.Background(), "/bin/sh", "-c", command)
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
		return nil, fmt.Errorf("%s", msg)
	}

	log.Info("azAuthMSI::getTokenCredentialUsingCLI : Successfully logged in using Azure CLI")
	log.Debug("azAuthMSI::getTokenCredentialUsingCLI : Output: %s", output)

	cred, err := azidentity.NewAzureCLICredential(nil)
	return cred, err
}
*/

type azAuthBlobMSI struct {
	azAuthMSI
}

// getServiceClient : returns MSI based service client for blob
func (azmsi *azAuthBlobMSI) getServiceClient(stConfig *AzStorageConfig) (interface{}, error) {
	cred, err := azmsi.getTokenCredential()
	if err != nil {
		log.Err("azAuthBlobMSI::getServiceClient : Failed to get token credential from MSI [%s]", err.Error())
		return nil, err
	}

	opts, err := getAzBlobServiceClientOptions(stConfig)
	if err != nil {
		log.Err("azAuthBlobMSI::getServiceClient : Failed to create client options [%s]", err.Error())
		return nil, err
	}

	svcClient, err := service.NewClient(azmsi.config.Endpoint, cred, opts)
	if err != nil {
		log.Err("azAuthBlobMSI::getServiceClient : Failed to create service client [%s]", err.Error())
	}

	return svcClient, err
}

type azAuthDatalakeMSI struct {
	azAuthMSI
}

// getServiceClient : returns MSI based service client for datalake
func (azmsi *azAuthDatalakeMSI) getServiceClient(stConfig *AzStorageConfig) (interface{}, error) {
	cred, err := azmsi.getTokenCredential()
	if err != nil {
		log.Err("azAuthDatalakeMSI::getServiceClient : Failed to get token credential from MSI [%s]", err.Error())
		return nil, err
	}

	opts, err := getAzDatalakeServiceClientOptions(stConfig)
	if err != nil {
		log.Err("azAuthDatalakeMSI::getServiceClient : Failed to create client options [%s]", err.Error())
		return nil, err
	}

	svcClient, err := serviceBfs.NewClient(azmsi.config.Endpoint, cred, opts)
	if err != nil {
		log.Err("azAuthDatalakeMSI::getServiceClient : Failed to create service client [%s]", err.Error())
	}

	return svcClient, err
}
