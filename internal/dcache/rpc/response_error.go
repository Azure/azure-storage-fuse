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

import (
	"errors"
	"syscall"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/apache/thrift/lib/go/thrift"
)

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
	common.Assert(err != nil)

	var respErr *ResponseError
	ok := errors.As(err, &respErr)
	if !ok {
		return nil
	}

	return respErr
}

// Check if the error returned by thrift indicates connection closed by server.
func IsConnectionClosed(err error) bool {
	common.Assert(err != nil)

	log.Debug("IsConnectionClosed: err: %v, %T", err, err)
	log.Debug("errors.Is(err, syscall.EPIPE) = %v", errors.Is(err, syscall.EPIPE))

	// RPC error, cannot be a connection reset error.
	if GetRPCResponseError(err) != nil {
		log.Debug("IsConnectionClosed: is RPC error: %v", err)
		return false
	}

	te := thrift.NewTTransportExceptionFromError(err)
	log.Debug("IsConnectionClosed: te: %v, %+v, %T, te.TypeId=%v", te, te, te, te.TypeId())
	return te.TypeId() == thrift.NOT_OPEN
}

// Check if the error returned by thrift indicates timeout.
func IsTimedOut(err error) bool {
	common.Assert(err != nil)

	// RPC error, cannot be a connection reset error.
	if GetRPCResponseError(err) != nil {
		return false
	}

	te := thrift.NewTTransportExceptionFromError(err)
	return te.TypeId() == thrift.TIMED_OUT
}
