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
	"strings"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/service"
)

//
// This test will delete the containers that were created in the pipeline runs and deleted but backend has not deleted
// them by some reason. We use this test in nightly pipeline in the start which will prevent storage account to not
// get exploded by these temporary containers that were used in the tests.
//

func getGenericCredential() (*service.SharedKeyCredential, error) {
	accountNameEnvVar := "STORAGE_ACCOUNT_NAME"
	accountKeyEnvVar := "STORAGE_ACCOUNT_KEY"
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

	pager := svcClient.NewListContainersPager(&service.ListContainersOptions{})

	for pager.More() {
		resp, err := pager.NextPage(ctx)
		if err != nil {
			log.Fatal(err)
		}

		for _, cont := range resp.ContainerItems {
			//
			// containers created by block_blob_test.go start with prefix and containers created dynamically in
			// pipelines runs have length 40. Delete all such containers if backend GC has skipped their deletion
			// for some reason.
			//
			if strings.HasPrefix(*cont.Name, "fuseutc") || len(*cont.Name) == 40 {
				containerClient := svcClient.NewContainerClient(*cont.Name)
				t.Log("Deleting container :", cont.Name)
				_, err = containerClient.Delete(ctx, nil)
				if err != nil {
					t.Logf("Unable to delete %v : [%v]", cont.Name, err.Error())
				}
			}
		}
	}
}

func TestMain(m *testing.M) {
	m.Run()
}
