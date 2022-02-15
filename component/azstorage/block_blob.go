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
	"blobfuse2/common/exectime"
	"blobfuse2/common/log"
	"blobfuse2/internal"
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/Azure/azure-storage-blob-go/azblob"
)

const (
	folderKey  = "hdi_isfolder"
	symlinkKey = "is_symlink"
)

type BlockBlob struct {
	AzStorageConnection
	Auth            azAuth
	Service         azblob.ServiceURL
	Container       azblob.ContainerURL
	blobAccCond     azblob.BlobAccessConditions
	blobCPKOpt      azblob.ClientProvidedKeyOptions
	downloadOptions azblob.DownloadFromBlobOptions
	listDetails     azblob.BlobListingDetails
}

// Verify that BlockBlob implements AzConnection interface
var _ AzConnection = &BlockBlob{}

func (bb *BlockBlob) Configure(cfg AzStorageConfig) error {
	bb.Config = cfg

	bb.blobAccCond = azblob.BlobAccessConditions{}
	bb.blobCPKOpt = azblob.ClientProvidedKeyOptions{}

	bb.downloadOptions = azblob.DownloadFromBlobOptions{
		BlockSize:   bb.Config.blockSize,
		Parallelism: bb.Config.maxConcurrency,
	}

	bb.listDetails = azblob.BlobListingDetails{
		Metadata:  true,
		Deleted:   false,
		Snapshots: false,
	}

	return nil
}

// For dynamic config update the config here
func (bb *BlockBlob) UpdateConfig(cfg AzStorageConfig) error {
	bb.Config.blockSize = cfg.blockSize
	bb.Config.maxConcurrency = cfg.maxConcurrency
	bb.Config.defaultTier = cfg.defaultTier
	bb.Config.ignoreAccessModifiers = cfg.ignoreAccessModifiers
	return nil
}

// NewCredentialKey : Update the credential key specified by the user
func (bb *BlockBlob) NewCredentialKey(key, value string) (err error) {
	if key == "saskey" {
		bb.Auth.setOption(key, value)
		// Update the endpoint url from the credential
		bb.Endpoint, err = url.Parse(bb.Auth.getEndpoint())
		if err != nil {
			log.Err("BlockBlob::NewCredentialKey : Failed to form base endpoint url (%s)", err.Error())
			return errors.New("failed to form base endpoint url")
		}

		// Update the service url
		bb.Service = azblob.NewServiceURL(*bb.Endpoint, bb.Pipeline)

		// Update the container url
		bb.Container = bb.Service.NewContainerURL(bb.Config.container)
	}
	return nil
}

// getCredential : Create the credential object
func (bb *BlockBlob) getCredential() azblob.Credential {
	log.Trace("BlockBlob::getCredential : Getting credential")

	bb.Auth = getAzAuth(bb.Config.authConfig)
	if bb.Auth == nil {
		log.Err("BlockBlob::getCredential : Failed to retreive auth object")
		return nil
	}

	cred := bb.Auth.getCredential()

	return cred.(azblob.Credential)
}

// SetupPipeline : Based on the config setup the ***URLs
func (bb *BlockBlob) SetupPipeline() error {
	log.Trace("BlockBlob::SetupPipeline : Setting up")
	var err error

	// Get the credential
	cred := bb.getCredential()
	if cred == nil {
		log.Err("BlockBlob::SetupPipeline : Failed to get credential")
		return errors.New("failed to get credential")
	}

	// Create a new pipeline
	bb.Pipeline = azblob.NewPipeline(cred, getAzBlobPipelineOptions(bb.Config))
	if bb.Pipeline == nil {
		log.Err("BlockBlob::SetupPipeline : Failed to create pipeline object")
		return errors.New("failed to create pipeline object")
	}

	// Get the endpoint url from the credential
	bb.Endpoint, err = url.Parse(bb.Auth.getEndpoint())
	if err != nil {
		log.Err("BlockBlob::SetupPipeline : Failed to form base end point url (%s)", err.Error())
		return errors.New("failed to form base end point url")
	}

	// Create the service url
	bb.Service = azblob.NewServiceURL(*bb.Endpoint, bb.Pipeline)

	// Create the container url
	bb.Container = bb.Service.NewContainerURL(bb.Config.container)

	return nil
}

// TestPipeline : Validate the credentials specified in the auth config
func (bb *BlockBlob) TestPipeline() error {
	log.Trace("BlockBlob::TestPipeline : Validating")

	if bb.Config.mountAllContainers {
		return nil
	}

	if bb.Container.String() == "" {
		log.Err("BlockBlob::TestPipeline : Container URL is not built, check your credentials")
		return nil
	}

	marker := (azblob.Marker{})
	listBlob, err := bb.Container.ListBlobsHierarchySegment(context.Background(), marker, "/",
		azblob.ListBlobsSegmentOptions{MaxResults: 2})

	if err != nil {
		log.Err("BlockBlob::TestPipeline : Failed to validate account with given auth %s", err.Error)
		return err
	}

	if listBlob == nil {
		log.Info("BlockBlob::TestPipeline : Container is empty")
	}
	return nil
}

func (bb *BlockBlob) ListContainers() ([]string, error) {
	log.Trace("BlockBlob::ListContainers : Listing containers")
	cntList := make([]string, 0)

	marker := azblob.Marker{}
	for marker.NotDone() {
		resp, err := bb.Service.ListContainersSegment(context.Background(), marker, azblob.ListContainersSegmentOptions{})
		if err != nil {
			log.Err("BlockBlob::ListContainers : Failed to get container list")
			return cntList, err
		}

		for _, v := range resp.ContainerItems {
			cntList = append(cntList, v.Name)
		}

		marker = resp.NextMarker
	}

	return cntList, nil
}

func (bb *BlockBlob) SetPrefixPath(path string) error {
	log.Trace("BlockBlob::SetPrefixPath : path %s", path)
	bb.Config.prefixPath = path
	return nil
}

// Exists : Check whether or not a given blob exists
func (bb *BlockBlob) Exists(name string) bool {
	log.Trace("BlockBlob::Exists : name %s", name)
	if _, err := bb.GetAttr(name); err == syscall.ENOENT {
		return false
	}
	return true
}

// CreateFile : Create a new file in the container/virtual directory
func (bb *BlockBlob) CreateFile(name string, mode os.FileMode) error {
	log.Trace("BlockBlob::CreateFile : name %s", name)
	var data []byte
	return bb.WriteFromBuffer(name, nil, data)
}

// CreateDirectory : Create a new directory in the container/virtual directory
func (bb *BlockBlob) CreateDirectory(name string) error {
	log.Trace("BlockBlob::CreateDirectory : name %s", name)

	var data []byte
	metadata := make(azblob.Metadata)
	metadata[folderKey] = "true"

	return bb.WriteFromBuffer(name, metadata, data)
}

// CreateLink : Create a symlink in the container/virtual directory
func (bb *BlockBlob) CreateLink(source string, target string) error {
	log.Trace("BlockBlob::CreateLink : %s -> %s", source, target)
	data := []byte(target)
	metadata := make(azblob.Metadata)
	metadata[symlinkKey] = "true"
	return bb.WriteFromBuffer(source, metadata, data)
}

// DeleteFile : Delete a blob in the container/virtual directory
func (bb *BlockBlob) DeleteFile(name string) (err error) {
	log.Trace("BlockBlob::DeleteFile : name %s", name)

	blobURL := bb.Container.NewBlobURL(filepath.Join(bb.Config.prefixPath, name))
	_, err = blobURL.Delete(context.Background(), azblob.DeleteSnapshotsOptionInclude, bb.blobAccCond)
	if err != nil {
		serr := storeBlobErrToErr(err)
		if serr == ErrFileNotFound {
			log.Err("BlockBlob::DeleteFile : %s does not exist", name)
			return syscall.ENOENT
		} else if serr == BlobIsUnderLease {
			log.Err("BlockBlob::DeleteFile : %s is under lease (%s)", name, err.Error())
			return syscall.EIO
		} else {
			log.Err("BlockBlob::DeleteFile : Failed to delete blob %s (%s)", name, err.Error())
			return err
		}
	}

	return nil
}

// DeleteDirectory : Delete a virtual directory in the container/virtual directory
func (bb *BlockBlob) DeleteDirectory(name string) (err error) {
	log.Trace("BlockBlob::DeleteDirectory : name %s", name)

	for marker := (azblob.Marker{}); marker.NotDone(); {
		listBlob, err := bb.Container.ListBlobsFlatSegment(context.Background(), marker,
			azblob.ListBlobsSegmentOptions{MaxResults: common.MaxDirListCount,
				Prefix: filepath.Join(bb.Config.prefixPath, name) + "/",
			})

		if err != nil {
			log.Err("BlockBlob::DeleteDirectory : Failed to get list of blobs %s", err.Error)
			return err
		}
		marker = listBlob.NextMarker

		// Process the blobs returned in this result segment (if the segment is empty, the loop body won't execute)
		for _, blobInfo := range listBlob.Segment.BlobItems {
			bb.DeleteFile(split(bb.Config.prefixPath, blobInfo.Name))
		}
	}
	return bb.DeleteFile(name)
}

// RenameFile : Rename the file
func (bb *BlockBlob) RenameFile(source string, target string) error {
	log.Trace("BlockBlob::RenameFile : %s -> %s", source, target)

	blobURL := bb.Container.NewBlockBlobURL(filepath.Join(bb.Config.prefixPath, source))
	newBlob := bb.Container.NewBlockBlobURL(filepath.Join(bb.Config.prefixPath, target))

	prop, err := blobURL.GetProperties(context.Background(), bb.blobAccCond, bb.blobCPKOpt)
	if err != nil {
		serr := storeBlobErrToErr(err)
		if serr == ErrFileNotFound {
			log.Err("BlockBlob::RenameFile : %s does not exist", source)
			return syscall.ENOENT
		} else {
			log.Err("BlockBlob::RenameFile : Failed to get blob properties for %s (%s)", source, err.Error())
			return err
		}
	}

	startCopy, err := newBlob.StartCopyFromURL(context.Background(), blobURL.URL(),
		prop.NewMetadata(), azblob.ModifiedAccessConditions{}, azblob.BlobAccessConditions{}, bb.Config.defaultTier, nil)

	if err != nil {
		log.Err("BlockBlob::RenameFile : Failed to start copy of file %s (%s)", source, err.Error())
		return err
	}

	copyStatus := startCopy.CopyStatus()
	for copyStatus == azblob.CopyStatusPending {
		time.Sleep(time.Second * 1)
		prop, err = newBlob.GetProperties(context.Background(), bb.blobAccCond, bb.blobCPKOpt)
		if err != nil {
			log.Err("BlockBlob::RenameFile : CopyStats : Failed to get blob properties for %s (%s)", source, err.Error())
		}
		copyStatus = prop.CopyStatus()
	}
	log.Trace("BlockBlob::RenameFile : %s -> %s done", source, target)

	// Copy of the file is done so now delete the older file
	return bb.DeleteFile(source)
}

// RenameDirectory : Rename the directory
func (bb *BlockBlob) RenameDirectory(source string, target string) error {
	log.Trace("BlockBlob::RenameDirectory : %s -> %s", source, target)

	for marker := (azblob.Marker{}); marker.NotDone(); {
		listBlob, err := bb.Container.ListBlobsFlatSegment(context.Background(), marker,
			azblob.ListBlobsSegmentOptions{MaxResults: common.MaxDirListCount,
				Prefix: filepath.Join(bb.Config.prefixPath, source) + "/",
			})

		if err != nil {
			log.Err("BlockBlob::RenameDirectory : Failed to get list of blobs %s", err.Error)
			return err
		}
		marker = listBlob.NextMarker

		// Process the blobs returned in this result segment (if the segment is empty, the loop body won't execute)
		for _, blobInfo := range listBlob.Segment.BlobItems {
			srcPath := split(bb.Config.prefixPath, blobInfo.Name)
			bb.RenameFile(srcPath, strings.Replace(srcPath, source, target, 1))
		}
	}

	return bb.RenameFile(source, target)
}

// GetAttr : Retreive attributes of the blob
func (bb *BlockBlob) GetAttr(name string) (attr *internal.ObjAttr, err error) {
	log.Trace("BlockBlob::GetAttr : name %s", name)

	blobURL := bb.Container.NewBlockBlobURL(filepath.Join(bb.Config.prefixPath, name))
	prop, err := blobURL.GetProperties(context.Background(), bb.blobAccCond, bb.blobCPKOpt)

	if err != nil {
		e := storeBlobErrToErr(err)
		if e == ErrFileNotFound {
			return attr, syscall.ENOENT
		} else {
			log.Err("BlockBlob::GetAttr : Failed to get blob properties for %s (%s)", name, err.Error())
			return attr, err
		}
	}

	// Since block blob does not support acls, we set mode to 0 and FlagModeDefault to true so the fuse layer can return the default permission.
	attr = &internal.ObjAttr{
		Path:   name, // We don't need to strip the prefixPath here since we pass the input name
		Name:   filepath.Base(name),
		Size:   prop.ContentLength(),
		Mode:   0,
		Mtime:  prop.LastModified(),
		Atime:  prop.LastModified(),
		Ctime:  prop.LastModified(),
		Crtime: prop.CreationTime(),
		Flags:  internal.NewFileBitMap(),
	}
	parseMetadata(attr, prop.NewMetadata())
	attr.Flags.Set(internal.PropFlagMetadataRetrieved)
	attr.Flags.Set(internal.PropFlagModeDefault)

	return attr, nil
}

// List : Get a list of blobs matching the given prefix
// This fetches the list using a marker so the caller code should handle marker logic
// If count=0 - fetch max entries
func (bb *BlockBlob) List(prefix string, marker *string, count int32) ([]*internal.ObjAttr, *string, error) {
	log.Trace("BlockBlob::List : prefix %s, marker %s", prefix, func(marker *string) string {
		if marker != nil {
			return *marker
		} else {
			return ""
		}
	}(marker))

	blobList := make([]*internal.ObjAttr, 0)

	if count == 0 {
		count = common.MaxDirListCount
	}

	listPath := filepath.Join(bb.Config.prefixPath, prefix)
	if (prefix != "" && prefix[len(prefix)-1] == '/') || (prefix == "" && bb.Config.prefixPath != "") {
		listPath += "/"
	}

	// Get a result segment starting with the blob indicated by the current Marker.
	listBlob, err := bb.Container.ListBlobsHierarchySegment(context.Background(), azblob.Marker{Val: marker}, "/",
		azblob.ListBlobsSegmentOptions{MaxResults: count,
			Prefix:  listPath,
			Details: bb.listDetails,
		})
	// Note: Since we make a list call with a prefix, we will not fail here for a non-existant directory.
	// The blob service will not validate for us whether or not the path exists.
	// This is different from ADLS Gen2 behavior.
	// APIs that may be affected include IsDirEmpty, ReadDir and StreamDir

	if err != nil {
		log.Err("BlockBlob::List : Failed to list the container with the prefix %s", err.Error)
		return blobList, nil, err
	}

	dereferenceTime := func(input *time.Time, defaultTime time.Time) time.Time {
		if input == nil {
			return defaultTime
		} else {
			return *input
		}
	}

	// Process the blobs returned in this result segment (if the segment is empty, the loop body won't execute)
	// Since block blob does not support acls, we set mode to 0 and FlagModeDefault to true so the fuse layer can return the default permission.

	// For some directories 0 byte meta file may not exists so just create a map to figure out such directories
	var dirList = make(map[string]bool)

	for _, blobInfo := range listBlob.Segment.BlobItems {
		attr := &internal.ObjAttr{
			Path:   split(bb.Config.prefixPath, blobInfo.Name),
			Name:   filepath.Base(blobInfo.Name),
			Size:   *blobInfo.Properties.ContentLength,
			Mode:   0,
			Mtime:  blobInfo.Properties.LastModified,
			Atime:  dereferenceTime(blobInfo.Properties.LastAccessedOn, blobInfo.Properties.LastModified),
			Ctime:  blobInfo.Properties.LastModified,
			Crtime: dereferenceTime(blobInfo.Properties.CreationTime, blobInfo.Properties.LastModified),
			Flags:  internal.NewFileBitMap(),
		}

		parseMetadata(attr, blobInfo.Metadata)
		attr.Flags.Set(internal.PropFlagMetadataRetrieved)
		attr.Flags.Set(internal.PropFlagModeDefault)
		blobList = append(blobList, attr)

		if attr.IsDir() {
			// 0 byte meta found so mark this directory in map
			dirList[blobInfo.Name+"/"] = true
			attr.Size = 4096
		}
	}

	// If in case virtual directory exists but its corrosponding 0 byte file is not there holding hdi_isfolder then just iterating
	// BlobItems will fail to identify that directory. In such cases BlobPrefixes help to list all directories
	// dirList contains all dirs for which we got 0 byte meta file, so except those add rest to the list
	for _, blobInfo := range listBlob.Segment.BlobPrefixes {
		if !dirList[blobInfo.Name] {
			//log.Info("BlockBlob::List : meta file does not exists for dir %s", blobInfo.Name)
			// For these dirs we get only the name and no other properties so hardcoding time to current time
			attr := &internal.ObjAttr{
				Path:  split(bb.Config.prefixPath, blobInfo.Name),
				Name:  filepath.Base(blobInfo.Name),
				Size:  4096,
				Mode:  os.ModeDir,
				Mtime: time.Now(),
				Flags: internal.NewDirBitMap(),
			}
			attr.Atime = attr.Mtime
			attr.Crtime = attr.Mtime
			attr.Ctime = attr.Mtime
			attr.Flags.Set(internal.PropFlagMetadataRetrieved)
			attr.Flags.Set(internal.PropFlagModeDefault)
			blobList = append(blobList, attr)
		}
	}

	// Clean up the temp map as its no more needed
	for k := range dirList {
		delete(dirList, k)
	}

	return blobList, listBlob.NextMarker.Val, nil
}

// ReadToFile : Download a blob to a local file
func (bb *BlockBlob) ReadToFile(name string, offset int64, count int64, fi *os.File) (err error) {
	log.Trace("BlockBlob::ReadToFile : name %s, offset : %d, count %d", name, offset, count)
	defer exectime.StatTimeCurrentBlock("BlockBlob::ReadToFile")()

	blobURL := bb.Container.NewBlobURL(filepath.Join(bb.Config.prefixPath, name))

	defer log.TimeTrack(time.Now(), "BlockBlob::ReadToFile", name)
	err = azblob.DownloadBlobToFile(context.Background(), blobURL, offset, count, fi, bb.downloadOptions)

	if err != nil {
		e := storeBlobErrToErr(err)
		if e == ErrFileNotFound {
			return syscall.ENOENT
		} else {
			log.Err("BlockBlob::ReadToFile : Failed to download blob %s (%s)", name, err.Error())
			return err
		}
	}

	return nil
}

// ReadBuffer : Download a specific range from a blob to a buffer
func (bb *BlockBlob) ReadBuffer(name string, offset int64, len int64) ([]byte, error) {
	log.Trace("BlockBlob::ReadBuffer : name %s", name)
	var buff []byte
	if len == 0 {
		len = azblob.CountToEnd
		attr, err := bb.GetAttr(name)
		if err != nil {
			return buff, err
		}
		buff = make([]byte, attr.Size)
	} else {
		buff = make([]byte, len)
	}

	blobURL := bb.Container.NewBlobURL(filepath.Join(bb.Config.prefixPath, name))
	err := azblob.DownloadBlobToBuffer(context.Background(), blobURL, offset, len, buff, bb.downloadOptions)

	if err != nil {
		e := storeBlobErrToErr(err)
		if e == ErrFileNotFound {
			return buff, syscall.ENOENT
		} else if e == InvalidRange {
			return buff, syscall.ERANGE
		}

		log.Err("BlockBlob::ReadBuffer : Failed to download blob %s (%s)", name, err.Error())
		return buff, err
	}

	return buff, nil
}

// ReadInBuffer : Download specific range from a file to a user provided buffer
func (bb *BlockBlob) ReadInBuffer(name string, offset int64, len int64, data []byte) error {
	log.Trace("BlockBlob::ReadInBuffer : name %s", name)
	blobURL := bb.Container.NewBlobURL(filepath.Join(bb.Config.prefixPath, name))
	err := azblob.DownloadBlobToBuffer(context.Background(), blobURL, offset, len, data, bb.downloadOptions)

	if err != nil {
		e := storeBlobErrToErr(err)
		if e == ErrFileNotFound {
			return syscall.ENOENT
		} else if e == InvalidRange {
			return syscall.ERANGE
		}

		log.Err("BlockBlob::ReadInBuffer : Failed to download blob %s (%s)", name, err.Error())
		return err
	}

	return nil
}

// WriteFromFile : Upload local file to blob
func (bb *BlockBlob) WriteFromFile(name string, metadata map[string]string, fi *os.File) (err error) {
	log.Trace("BlockBlob::WriteFromFile : name %s", name)
	defer exectime.StatTimeCurrentBlock("WriteFromFile::WriteFromFile")()

	blobURL := bb.Container.NewBlockBlobURL(filepath.Join(bb.Config.prefixPath, name))

	defer log.TimeTrack(time.Now(), "BlockBlob::WriteFromFile", name)
	_, err = azblob.UploadFileToBlockBlob(context.Background(), fi, blobURL, azblob.UploadToBlockBlobOptions{
		BlockSize:      bb.Config.blockSize,
		Parallelism:    bb.Config.maxConcurrency,
		Metadata:       metadata,
		BlobAccessTier: bb.Config.defaultTier,
		BlobHTTPHeaders: azblob.BlobHTTPHeaders{
			ContentType: getContentType(name),
		},
	})

	if err != nil {
		serr := storeBlobErrToErr(err)
		if serr == BlobIsUnderLease {
			log.Err("BlockBlob::WriteFromFile : %s is under a lease, can not update file (%s)", name, err.Error())
			return syscall.EIO
		} else {
			log.Err("BlockBlob::WriteFromFile : Failed to upload blob %s (%s)", name, err.Error())
		}
		return err
	}

	return nil
}

// WriteFromBuffer : Upload from a buffer to a blob
func (bb *BlockBlob) WriteFromBuffer(name string, metadata map[string]string, data []byte) error {
	log.Trace("BlockBlob::WriteFromBuffer : name %s", name)
	blobURL := bb.Container.NewBlockBlobURL(filepath.Join(bb.Config.prefixPath, name))

	defer log.TimeTrack(time.Now(), "BlockBlob::WriteFromBuffer", name)
	_, err := azblob.UploadBufferToBlockBlob(context.Background(), data, blobURL, azblob.UploadToBlockBlobOptions{
		BlockSize:      bb.Config.blockSize,
		Parallelism:    bb.Config.maxConcurrency,
		Metadata:       metadata,
		BlobAccessTier: bb.Config.defaultTier,
		BlobHTTPHeaders: azblob.BlobHTTPHeaders{
			ContentType: getContentType(name),
		},
	})

	if err != nil {
		log.Err("BlockBlob::WriteFromBuffer : Failed to upload blob %s (%s)", name, err.Error())
		return err
	}

	return nil
}

func (bb *BlockBlob) GetFileBlockOffsets(name string) (common.BlockOffsetList, bool, error) {
	var blockOffset int64 = 0
	var blockList common.BlockOffsetList
	blobURL := bb.Container.NewBlockBlobURL(filepath.Join(bb.Config.prefixPath, name))
	storageBlockList, _ := blobURL.GetBlockList(
		context.Background(), azblob.BlockListCommitted, bb.blobAccCond.LeaseAccessConditions)
	for _, block := range *&storageBlockList.CommittedBlocks {
		blk := &common.Block{
			Id:         block.Name,
			StartIndex: int64(blockOffset),
			EndIndex:   int64(blockOffset) + block.Size,
			Size:       block.Size,
		}
		blockOffset += block.Size
		blockList = append(blockList, blk)
	}
	return blockList, len(blockList) > 0, nil
}

// WriteFromBuffer : write data at given offset to a blob
func (bb *BlockBlob) Write(name string, offset int64, length int64, data []byte, FileOffsets common.BlockOffsetList) error {
	defer log.TimeTrack(time.Now(), "BlockBlob::Write", name)
	blobURL := bb.Container.NewBlockBlobURL(filepath.Join(bb.Config.prefixPath, name))
	multipleBlocks := true
	var blockList common.BlockOffsetList
	// if this is not 0 then we passed a cached block ID list
	if len(FileOffsets) == 0 {
		blockList, multipleBlocks, _ = bb.GetFileBlockOffsets(name)
	}
	// case 1: file consists of no blocks (small file)
	if !multipleBlocks {
		// get all the data
		oldData, _ := bb.ReadBuffer(name, 0, 0)
		// update the data with the new data
		if int64(len(oldData)) >= offset+length {
			copy(oldData[offset:], data)
			// uplaod the data
			bb.WriteFromBuffer(name, nil, oldData)
		} else {
			d := make([]byte, offset+length)
			copy(d, oldData)
			oldData = nil
			copy(d[offset:], data)
			// WriteFromBuffer should be able to handle the case where now the block is too big and gets split into multiple blocks
			bb.WriteFromBuffer(name, nil, d)
		}
		// case 2: Given offset is within the size of the blob - and the blob consists of multiple blocks
		// TODO: case 3: offset is ahead of blocks (appending)
	} else {
		modifiedBlockList, oldDataSize := blockList.FindBlocksToModify(offset, length)
		// buffer that holds that pre-existing data in those blocks we're interested in
		oldDataBuffer := make([]byte, oldDataSize)
		// fetch the blocks that will be impacted by the new changes so we can overwrite them
		bb.ReadInBuffer(name, modifiedBlockList[0].StartIndex, oldDataSize, oldDataBuffer)
		blockOffset := offset - modifiedBlockList[0].StartIndex
		copy(oldDataBuffer[blockOffset:], data)

		for _, blk := range modifiedBlockList {
			blk.Data = oldDataBuffer[blk.StartIndex:blk.EndIndex]
			_, err := blobURL.StageBlock(context.Background(),
				blk.Id, bytes.NewReader(blk.Data),
				bb.blobAccCond.LeaseAccessConditions,
				nil, bb.downloadOptions.ClientProvidedKeyOptions)
			if err != nil {
				fmt.Println("error: ", err)
			}
		}
		var blockIDList []string
		for _, blk := range blockList {
			blockIDList = append(blockIDList, blk.Id)
		}
		_, err := blobURL.CommitBlockList(context.Background(),
			blockIDList,
			azblob.BlobHTTPHeaders{ContentType: getContentType(name)},
			nil, azblob.BlobAccessConditions{}, bb.Config.defaultTier, azblob.BlobTagsMap{}, bb.downloadOptions.ClientProvidedKeyOptions)
		if err != nil {
			fmt.Println("error: ", err)
		}
	}
	return nil
}

// ChangeMod : Change mode of a blob
func (bb *BlockBlob) ChangeMod(name string, _ os.FileMode) error {
	log.Trace("BlockBlob::ChangeMod : name %s", name)

	if bb.Config.ignoreAccessModifiers {
		// for operations like git clone where transaction fails if chmod is not successful
		// return success instead of ENOSYS
		return nil
	}

	// This is not currently supported for a flat namespace account
	return syscall.ENOTSUP
}

// ChangeOwner : Change owner of a blob
func (bb *BlockBlob) ChangeOwner(name string, _ int, _ int) error {
	log.Trace("BlockBlob::ChangeOwner : name %s", name)

	if bb.Config.ignoreAccessModifiers {
		// for operations like git clone where transaction fails if chown is not successful
		// return success instead of ENOSYS
		return nil
	}

	// This is not currently supported for a flat namespace account
	return syscall.ENOTSUP
}
