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

package rpc

import "errors"

const (
	InvalidRequest      = iota + 1 // invalid rpc request
	InvalidRVID                    // RV does not have the rvID
	InvalidRV                      // RV is invalid for the given node
	InternalServerError            // Miscellaneous errors
	ChunkNotFound                  // Chunk not found
	ChunkAlreadyExists             // Chunk already exists
	MaxMVsExceeded                 // Max number of MVs exceeded for the RV
	// Component RVs are invalid for the MV.
	// This indicates the client that its copy of clustermap is stale.
	// So, it should fetch the latest clustermap copy and retry.
	NeedToRefreshClusterMap
)

type ResponseError struct {
	errCode int
	errMsg  string
}

func NewResponseError(errorCode int, errorMessage string) *ResponseError {
	return &ResponseError{
		errCode: errorCode,
		errMsg:  errorMessage,
	}
}

func (e *ResponseError) Code() int {
	return e.errCode
}

func (e *ResponseError) Error() string {
	return e.errMsg
}

// check if the error is of type ResponseError
func GetRPCResponseError(err error) *ResponseError {
	var respErr *ResponseError
	ok := errors.As(err, &respErr)
	if !ok {
		return nil
	}

	return respErr
}
