// +build !unittest

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

package account_cleanup

import (
	"context"
	"errors"
	"log"
	"net/url"
	"os"
	"regexp"
	"testing"

	"github.com/Azure/azure-storage-file-go/azfile"
)

func getGenericFileCredential() (*azfile.SharedKeyCredential, error) {
	accountNameEnvVar := "STORAGE_ACCOUNT_NAME"
	accountKeyEnvVar := "STORAGE_ACCOUNT_KEY"
	accountName, accountKey := os.Getenv(accountNameEnvVar), os.Getenv(accountKeyEnvVar)

	if accountName == "" || accountKey == "" {
		return nil, errors.New(accountNameEnvVar + " and/or " + accountKeyEnvVar + " environment variables not specified.")
	}
	return azfile.NewSharedKeyCredential(accountName, accountKey)
}

func getGenericFSU() (azfile.ServiceURL, error) {
	credential, err := getGenericFileCredential()
	if err != nil {
		return azfile.ServiceURL{}, err
	}

	pipeline := azfile.NewPipeline(credential, azfile.PipelineOptions{})
	filePrimaryURL, _ := url.Parse("https://" + credential.AccountName() + ".file.core.windows.net/")
	return azfile.NewServiceURL(*filePrimaryURL, pipeline), nil
}

func TestDeleteAllTempShares(t *testing.T) {
	ctx := context.Background()
	fsu, err := getGenericFSU()
	if err != nil {
		log.Fatal(err)
	}

	marker := azfile.Marker{}
	pattern := "fuseut*"

	for marker.NotDone() {
		resp, err := fsu.ListSharesSegment(ctx, marker, azfile.ListSharesOptions{})

		if err != nil {
			log.Fatal(err)
		}

		for _, v := range resp.ShareItems {
			matched, err := regexp.MatchString(pattern, v.Name)
			if matched && err == nil {
				shareURL := fsu.NewShareURL(v.Name)
				shareURL.Delete(ctx, azfile.DeleteSnapshotsOptionNone)
				t.Log("Deleting share :", v.Name)
			} else {
				t.Log("Skipping share :", v.Name)
			}
		}
		marker = resp.NextMarker
	}
}

func TestMain(m *testing.M) {
	m.Run()
}
