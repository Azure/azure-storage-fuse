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
	"blobfuse2/common"
	"blobfuse2/common/log"
	"blobfuse2/internal"
	"net/url"
	"os"

	"github.com/Azure/azure-pipeline-go/pipeline"
	"github.com/Azure/azure-storage-blob-go/azblob"

	stecommon "github.com/Azure/azure-storage-azcopy/v10/common"
)

// Example for azblob usage : https://godoc.org/github.com/Azure/azure-storage-blob-go/azblob#pkg-examples
// For methods help refer : https://godoc.org/github.com/Azure/azure-storage-blob-go/azblob#ContainerURL
type AzStorageConfig struct {
	authConfig azAuthConfig

	container      string
	prefixPath     string
	blockSize      int64
	maxConcurrency uint16

	// tier to be set on every upload
	defaultTier azblob.AccessTierType

	// Return back readDir on mount for given amount of time
	cancelListForSeconds uint16

	// Retry policy config
	maxRetries            int32
	maxTimeout            int32
	backoffTime           int32
	maxRetryDelay         int32
	proxyAddress          string
	sdkTrace              bool
	ignoreAccessModifiers bool
	mountAllContainers    bool

	// STE config
	steEnable         bool
	steMinFileSize    int64
	steSlicePool      int64
	steCacheLimit     int64
	steFileCountLimit int64
	steGCPercent      int
}

type AzStorageConnection struct {
	Config AzStorageConfig

	Pipeline pipeline.Pipeline

	Endpoint *url.URL

	// STE related stuff
	steConfig AzSTEConfig
	STE       AzSTE
}

type WriteFileOptions struct {
	localPath string
}

type ReadFileOptions struct {
	localPath string
}
type AzConnection interface {
	Configure(cfg AzStorageConfig) error
	UpdateConfig(cfg AzStorageConfig) error

	SetupPipeline() error
	TestPipeline() error

	InitializeSTE() error

	ListContainers() ([]string, error)

	// This is just for test, shall not be used otherwise
	SetPrefixPath(string) error

	Exists(name string) bool
	CreateFile(name string, mode os.FileMode) error
	CreateDirectory(name string) error
	CreateLink(source string, target string) error

	DeleteFile(name string) error
	DeleteDirectory(name string) error

	RenameFile(string, string) error
	RenameDirectory(string, string) error

	GetAttr(name string) (attr *internal.ObjAttr, err error)

	// Standard operations to be supported by any account type
	List(prefix string, marker *string, count int32) ([]*internal.ObjAttr, *string, error)

	ReadToFile(name string, offset int64, count int64, fi *os.File, options ReadFileOptions) error
	ReadBuffer(name string, offset int64, len int64) ([]byte, error)
	ReadInBuffer(name string, offset int64, len int64, data []byte) error

	WriteFromFile(name string, metadata map[string]string, fi *os.File, options WriteFileOptions) error
	WriteFromBuffer(name string, metadata map[string]string, data []byte) error

	ChangeMod(string, os.FileMode) error
	ChangeOwner(string, int, int) error

	NewCredentialKey(_, _ string) error
}

// NewAzStorageConnection : Based on account type create respective AzConnection Object
func NewAzStorageConnection(cfg AzStorageConfig) AzConnection {
	if cfg.authConfig.AccountType == EAccountType.INVALID_ACC() {
		log.Err("NewAzStorageConnection : Invalid account type")
	} else if cfg.authConfig.AccountType == EAccountType.BLOCK() {
		stg := &BlockBlob{}
		if err := stg.Configure(cfg); err != nil {
			log.Err("NewAzStorageConnection : Failed to configure BlockBlob object (%s)", err.Error())
			return nil
		}
		return stg
	} else if cfg.authConfig.AccountType == EAccountType.ADLS() {
		stg := &Datalake{}
		if err := stg.Configure(cfg); err != nil {
			log.Err("NewAzStorageConnection : Failed to configure Datalake object (%s)", err.Error())
			return nil
		}
		return stg
	} else {
		log.Err("NewAzStorageConnection : Invalid account type %s", cfg.authConfig.AccountType)
		return nil
	}

	return nil
}

// ------ STE Specific implementations ----------------
// PopulateSTEConfig : Convert config to STEConfig structure
func (conn *AzStorageConnection) PopulateSTEConfig(cfg AzStorageConfig) error {
	conn.steConfig = AzSTEConfig{
		Enable:         cfg.steEnable,
		MinFileSize:    cfg.steMinFileSize,
		SlicePool:      cfg.steSlicePool,
		CacheLimit:     cfg.steCacheLimit,
		FileCountLimit: cfg.steFileCountLimit,
		GCPercent:      cfg.steGCPercent,
		partFilePath:   os.ExpandEnv(common.DefaultWorkDir),
	}
	return nil
}

func (conn *AzStorageConnection) getSTETier() stecommon.BlockBlobTier {
	switch conn.Config.defaultTier {
	case azblob.AccessTierHot:
		return stecommon.EBlockBlobTier.Hot()
	case azblob.AccessTierCool:
		return stecommon.EBlockBlobTier.Cool()
	case azblob.AccessTierArchive:
		return stecommon.EBlockBlobTier.Archive()
	default:
		return stecommon.EBlockBlobTier.None()
	}
}
