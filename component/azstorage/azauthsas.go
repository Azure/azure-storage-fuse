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
	"fmt"

	"github.com/Azure/azure-storage-fuse/v2/common/log"

	"github.com/Azure/azure-storage-azcopy/v10/azbfs"
)

// Verify that the Auth implement the correct AzAuth interfaces
var _ azAuth = &azAuthBfsSAS{}

type azAuthSAS struct {
	azAuthBase
}

// GetEndpoint : Gets the SAS endpoint
func (azsas *azAuthSAS) getEndpoint() string {
	return fmt.Sprintf("%s%s",
		azsas.config.Endpoint,
		azsas.config.SASKey)
}

// SetOption : Sets the sas key information for the SAS auth.
func (azsas *azAuthSAS) setOption(key, value string) {
	if key == "saskey" {
		azsas.config.SASKey = value
	}
}

type azAuthBfsSAS struct {
	azAuthSAS
}

// GetCredential : Gets SAS based credentials for datralake
func (azsas *azAuthBfsSAS) getCredential() interface{} {
	if azsas.config.SASKey == "" {
		log.Err("azAuthBfsSAS::getCredential : SAS key for account is empty, cannot authenticate user")
		return nil
	}

	return azbfs.NewAnonymousCredential()
}
