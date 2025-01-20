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
	"os"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blob"
	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal"
	"github.com/vibhansa-msft/blobfilter"
)

// Example for azblob usage : https://pkg.go.dev/github.com/Azure/azure-sdk-for-go/sdk/storage/azblob#pkg-examples
// For methods help refer : https://pkg.go.dev/github.com/Azure/azure-sdk-for-go/sdk/storage/azblob#Client
type AzStorageConfig struct {
	authConfig azAuthConfig

	container      string
	prefixPath     string
	blockSize      int64
	maxConcurrency uint16

	// tier to be set on every upload
	defaultTier *blob.AccessTier

	// Return back readDir on mount for given amount of time
	cancelListForSeconds uint16

	// Retry policy config
	maxRetries            int32
	maxTimeout            int32
	backoffTime           int32
	maxRetryDelay         int32
	proxyAddress          string
	ignoreAccessModifiers bool
	mountAllContainers    bool

	updateMD5          bool
	validateMD5        bool
	virtualDirectory   bool
	maxResultsForList  int32
	disableCompression bool

	telemetry   string
	honourACL   bool
	preserveACL bool

	// CPK related config
	cpkEnabled             bool
	cpkEncryptionKey       string
	cpkEncryptionKeySha256 string

	// Blob filters
	filter *blobfilter.BlobFilter
}

type AzStorageConnection struct {
	Config AzStorageConfig
}

type AzConnection interface {
	Configure(cfg AzStorageConfig) error
	UpdateConfig(cfg AzStorageConfig) error

	SetupPipeline() error
	TestPipeline() error

	ListContainers() ([]string, error)

	// This is just for test, shall not be used otherwise
	SetPrefixPath(string) error

	CreateFile(name string, mode os.FileMode) error
	CreateDirectory(name string) error
	CreateLink(source string, target string) error

	DeleteFile(name string) error
	DeleteDirectory(name string) error

	RenameFile(string, string, *internal.ObjAttr) error
	RenameDirectory(string, string) error

	GetAttr(name string) (attr *internal.ObjAttr, err error)

	// Standard operations to be supported by any account type
	List(prefix string, marker *string, count int32) ([]*internal.ObjAttr, *string, error)

	ReadToFile(name string, offset int64, count int64, fi *os.File) error
	ReadBuffer(name string, offset int64, len int64) ([]byte, error)
	ReadInBuffer(name string, offset int64, len int64, data []byte) error

	WriteFromFile(name string, metadata map[string]*string, fi *os.File) error
	WriteFromBuffer(name string, metadata map[string]*string, data []byte) error
	Write(options internal.WriteFileOptions) error
	GetFileBlockOffsets(name string) (*common.BlockOffsetList, error)

	ChangeMod(string, os.FileMode) error
	ChangeOwner(string, int, int) error
	TruncateFile(string, int64) error
	StageAndCommit(name string, bol *common.BlockOffsetList) error

	GetCommittedBlockList(string) (*internal.CommittedBlockList, error)
	StageBlock(string, []byte, string) error
	CommitBlocks(string, []string) error

	UpdateServiceClient(_, _ string) error

	SetFilter(string) error
}

// NewAzStorageConnection : Based on account type create respective AzConnection Object
func NewAzStorageConnection(cfg AzStorageConfig) AzConnection {
	if cfg.authConfig.AccountType == EAccountType.INVALID_ACC() {
		log.Err("NewAzStorageConnection : Invalid account type")
	} else if cfg.authConfig.AccountType == EAccountType.BLOCK() {
		stg := &BlockBlob{}
		_ = stg.Configure(cfg)
		return stg
	} else if cfg.authConfig.AccountType == EAccountType.ADLS() {
		stg := &Datalake{}
		_ = stg.Configure(cfg)
		return stg
	}

	return nil
}
