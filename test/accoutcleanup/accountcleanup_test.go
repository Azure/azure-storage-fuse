//go:build !unittest
// +build !unittest

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

package account_cleanup

import (
	"context"
	"errors"
	"log"
	"os"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/service"
)

func getGenericCredential() (*service.SharedKeyCredential, error) {
	accountNameEnvVar := "STORAGE_ACCOUNT_NAME"
	accountKeyEnvVar := "STORAGE_ACCOUNT_Key"
	accountName, accountKey := os.Getenv(accountNameEnvVar), os.Getenv(accountKeyEnvVar)

	if accountName == "" || accountKey == "" {
		return nil, errors.New(accountNameEnvVar + " and/or " + accountKeyEnvVar + " environment variables not specified.")
	}
	return service.NewSharedKeyCredential(accountName, accountKey)
}

func getGenericServiceClient() (*service.Client, error) {
	credential, err := getGenericCredential()
	if err != nil {
		return nil, err
	}

	serviceURL := "https://" + credential.AccountName() + ".blob.core.windows.net/"
	return service.NewClientWithSharedKeyCredential(serviceURL, credential, nil)
}

func TestDeleteAllTempContainers(t *testing.T) {
	ctx := context.Background()
	svcClient, err := getGenericServiceClient()
	if err != nil {
		log.Fatal(err)
	}

	pager := svcClient.NewListContainersPager(&service.ListContainersOptions{
		Prefix: to.Ptr("fuseut"),
	})

	for pager.More() {
		resp, err := pager.NextPage(ctx)
		if err != nil {
			log.Fatal(err)
		}

		for _, v := range resp.ContainerItems {
			containerClient := svcClient.NewContainerClient(*v.Name)
			t.Log("Deleting container :", v.Name)
			_, err = containerClient.Delete(ctx, nil)
			if err != nil {
				t.Logf("Unable to delete %v : [%v]", v.Name, err.Error())
			}
		}
	}
}

func TestMain(m *testing.M) {
	m.Run()
}
