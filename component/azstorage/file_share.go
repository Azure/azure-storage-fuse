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
	"context"
	"errors"
	"net/url"
	"os"

	"github.com/Azure/azure-storage-file-go/azfile"
)

type FileShare struct {
	AzStorageConnection
	Auth    azAuth
	Service azfile.ServiceURL
	Share   azfile.ShareURL
}

func (fs *FileShare) Configure(cfg AzStorageConfig) error {
	fs.Config = cfg

	return nil
}

// For dynamic config update the config here
func (fs *FileShare) UpdateConfig(cfg AzStorageConfig) error {
	fs.Config.blockSize = cfg.blockSize
	fs.Config.maxConcurrency = cfg.maxConcurrency
	fs.Config.defaultTier = cfg.defaultTier
	fs.Config.ignoreAccessModifiers = cfg.ignoreAccessModifiers
	return nil
}

// getCredential : Create the credential object
func (fs *FileShare) getCredential() azfile.Credential {
	log.Trace("FileShare::getCredential : Getting credential")

	fs.Auth = getAzAuth(fs.Config.authConfig)
	if fs.Auth == nil {
		log.Err("FileShare::getCredential : Failed to retrieve auth object")
		return nil
	}

	cred := fs.Auth.getCredential()
	if cred == nil {
		log.Err("FileShare::getCredential : Failed to get credential")
		return nil
	}

	return cred.(azfile.Credential)
}

// SetupPipeline : Based on the config setup the ***URLs
func (fs *FileShare) SetupPipeline() error {
	log.Trace("Fileshare::SetupPipeline : Setting up")
	var err error

	// Get the credential
	cred := fs.getCredential()
	if cred == nil {
		log.Err("FileShare::SetupPipeline : Failed to get credential")
		return errors.New("failed to get credential")
	}

	// Create a new pipeline
	fs.Pipeline = azfile.NewPipeline(cred, getAzFilePipelineOptions(fs.Config)) // need to modify utils.go?
	if fs.Pipeline == nil {
		log.Err("FileShare::SetupPipeline : Failed to create pipeline object")
		return errors.New("failed to create pipeline object")
	}

	// Get the endpoint url from the credential
	fs.Endpoint, err = url.Parse(fs.Auth.getEndpoint())
	if err != nil {
		log.Err("BlockBlob::SetupPipeline : Failed to form base end point url (%s)", err.Error())
		return errors.New("failed to form base end point url")
	}

	// Create the service url
	fs.Service = azfile.NewServiceURL(*fs.Endpoint, fs.Pipeline)

	// Create the container url
	fs.Share = fs.Service.NewShareURL(fs.Config.container)

	return nil
}

// TestPipeline : Validate the credentials specified in the auth config
func (fs *FileShare) TestPipeline() error {
	log.Trace("FileShare::TestPipeline : Validating")

	if fs.Config.mountAllContainers {
		return nil
	}

	if fs.Share.String() == "" {
		log.Err("FileShare::TestPipeline : Container URL is not built, check your credentials")
		return nil
	}

	marker := (azfile.Marker{})
	listBlob, err := fs.Share.NewRootDirectoryURL().ListFilesAndDirectoriesSegment(context.Background(), marker,
		azfile.ListFilesAndDirectoriesOptions{MaxResults: 2})

	if err != nil {
		log.Err("FileShare::TestPipeline : Failed to validate account with given auth %s", err.Error)
		return err
	}

	if listBlob == nil {
		log.Info("FileShare::TestPipeline : Container is empty")
	}
	return nil
}

func (fs *FileShare) ListContainers() ([]string, error) {
	return nil, nil
}

// This is just for test, shall not be used otherwise
func (fs *FileShare) SetPrefixPath(string) error {
	return nil
}

func (fs *FileShare) Exists(name string) bool {
	return false
}
func (fs *FileShare) CreateFile(name string, mode os.FileMode) error {
	return nil
}
func (fs *FileShare) CreateDirectory(name string) error {
	return nil
}
func (fs *FileShare) CreateLink(source string, target string) error {
	return nil
}

func (fs *FileShare) DeleteFile(name string) error {
	return nil
}
func (fs *FileShare) DeleteDirectory(name string) error {
	return nil
}

func (fs *FileShare) RenameFile(string, string) error {
	return nil
}
func (fs *FileShare) RenameDirectory(string, string) error {
	return nil
}

func (fs *FileShare) GetAttr(name string) (attr *internal.ObjAttr, err error) {
	return nil, nil
}

// Standard operations to be supported by any account type
func (fs *FileShare) List(prefix string, marker *string, count int32) ([]*internal.ObjAttr, *string, error) {
	return nil, nil, nil
}

func (fs *FileShare) ReadToFile(name string, offset int64, count int64, fi *os.File) error {
	return nil
}
func (fs *FileShare) ReadBuffer(name string, offset int64, len int64) ([]byte, error) {
	return nil, nil
}
func (fs *FileShare) ReadInBuffer(name string, offset int64, len int64, data []byte) error {
	return nil
}

func (fs *FileShare) WriteFromFile(name string, metadata map[string]string, fi *os.File) error {
	return nil
}
func (fs *FileShare) WriteFromBuffer(name string, metadata map[string]string, data []byte) error {
	return nil
}
func (fs *FileShare) Write(options internal.WriteFileOptions) error {
	return nil
}
func (fs *FileShare) GetFileBlockOffsets(name string) (*common.BlockOffsetList, error) {
	return nil, nil
}

func (fs *FileShare) ChangeMod(string, os.FileMode) error {
	return nil
}
func (fs *FileShare) ChangeOwner(string, int, int) error {
	return nil
}
func (fs *FileShare) TruncateFile(string, int64) error {
	return nil
}
func (fs *FileShare) StageAndCommit(name string, bol *common.BlockOffsetList) error {
	return nil
}

func (fs *FileShare) NewCredentialKey(_, _ string) error {
	return nil
}
