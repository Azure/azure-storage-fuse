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

	"github.com/Azure/azure-storage-azcopy/v10/azbfs"
	"github.com/Azure/azure-storage-blob-go/azblob"
)

// Verify that the Auth implement the correct AzAuth interfaces
var _ azAuth = &azAuthBlobKey{}
var _ azAuth = &azAuthBfsKey{}

type azAuthKey struct {
	azAuthBase
}

type azAuthBlobKey struct {
	azAuthKey
}

// GetCredential : Gets shared key based storage credentials for blob
func (azkey *azAuthBlobKey) getCredential() interface{} {
	if azkey.config.AccountKey == "" {
		log.Err("azAuthBlobKey::getCredential : Shared key for account is empty, cannot autheticate user")
		return nil
	}

	credential, err := azblob.NewSharedKeyCredential(
		azkey.config.AccountName,
		azkey.config.AccountKey)
	if err != nil {
		log.Err("azAuthBlobKey::getCredential : Failed to create shared key credentials")
		return nil
	}

	return credential
}

type azAuthBfsKey struct {
	azAuthKey
}

// GetCredential : Gets shared key based storage credentials for datalake
func (azkey *azAuthBfsKey) getCredential() interface{} {
	if azkey.config.AccountKey == "" {
		log.Err("azAuthBfsKey::getCredential : Shared key for account is empty, cannot autheticate user")
		return nil
	}

	credential := azbfs.NewSharedKeyCredential(
		azkey.config.AccountName,
		azkey.config.AccountKey)

	return credential
}
