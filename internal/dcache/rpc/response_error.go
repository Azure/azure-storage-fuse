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
	"strings"
	"syscall"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"

	//"github.com/apache/thrift/lib/go/thrift"

	"github.com/Azure/azure-storage-fuse/v2/internal/dcache/rpc/gen-go/dcache/models"
)

func NewResponseError(errorCode models.ErrorCode, errorMessage string) *models.ResponseError {
	return &models.ResponseError{
		Code:    models.ErrorCode(errorCode),
		Message: errorMessage,
	}
}

// check if the error is of type *models.ResponseError
func GetRPCResponseError(err error) *models.ResponseError {
	common.Assert(err != nil)

	var respErr *models.ResponseError
	ok := errors.As(err, &respErr)
	if !ok {
		return nil
	}

	return respErr
}

// Check if the error returned by thrift indicates connection closed by server.
func IsConnectionClosed(err error) bool {
	common.Assert(err != nil)

	// RPC error, cannot be a connection reset error.
	if GetRPCResponseError(err) != nil {
		log.Debug("IsConnectionClosed: is RPC error: %v", err)
		return false
	}

	// Note: This doesn't work.
	//te := thrift.NewTTransportExceptionFromError(err)
	//return te.TypeId() == thrift.NOT_OPEN

	return errors.Is(err, syscall.EPIPE)
}

// Check if the error returned by thrift indicates connection refused by server.
func IsConnectionRefused(err error) bool {
	common.Assert(err != nil)

	// RPC error, cannot be a connection refused error.
	if GetRPCResponseError(err) != nil {
		log.Debug("IsConnectionRefused: is RPC error: %v", err)
		return false
	}

	log.Debug("IsConnectionRefused: err: %v, err: %T", err, err)

	//
	// TODO: This does not seem to match when we get the following error from thrift.
	// [dial tcp 10.0.0.5:9090: connect: connection refused]
	//
	// Doing string match for now.
	//
	//return errors.Is(err, syscall.ECONNREFUSED)

	connectionRefused := "connection refused"
	return strings.Contains(err.Error(), connectionRefused)
}

// Check if the error returned by thrift indicates timeout.
func IsTimedOut(err error) bool {
	common.Assert(err != nil)

	// RPC error, cannot be a connection reset error.
	if GetRPCResponseError(err) != nil {
		log.Debug("IsTimedOut: is RPC error: %v", err)
		return false
	}

	// TODO: This is untested, see whether this works or the ETIMEDOUT check works!
	//te := thrift.NewTTransportExceptionFromError(err)
	//return te.TypeId() == thrift.TIMED_OUT

	return errors.Is(err, syscall.ETIMEDOUT)
}
