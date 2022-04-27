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
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"math"
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
		log.Err("BlockBlob::getCredential : Failed to retrieve auth object")
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

// GetAttr : Retrieve attributes of the blob
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
	// Note: Since we make a list call with a prefix, we will not fail here for a non-existent directory.
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
	//defer exectime.StatTimeCurrentBlock("BlockBlob::ReadToFile")()

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
	//defer exectime.StatTimeCurrentBlock("WriteFromFile::WriteFromFile")()

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

// GetFileBlockOffsets: store blocks ids and corresponding offsets
func (bb *BlockBlob) GetFileBlockOffsets(name string) (*common.BlockOffsetList, error) {
	var blockOffset int64 = 0
	blockList := common.BlockOffsetList{}
	blobURL := bb.Container.NewBlockBlobURL(filepath.Join(bb.Config.prefixPath, name))
	storageBlockList, err := blobURL.GetBlockList(
		context.Background(), azblob.BlockListCommitted, bb.blobAccCond.LeaseAccessConditions)
	if err != nil {
		log.Err("BlockBlob::GetFileBlockOffsets : Failed to get block list %s ", name, err.Error())
		return &common.BlockOffsetList{}, err
	}
	for _, block := range storageBlockList.CommittedBlocks {
		blk := &common.Block{
			Id:         block.Name,
			StartIndex: int64(blockOffset),
			EndIndex:   int64(blockOffset) + block.Size,
		}
		blockOffset += block.Size
		blockList.BlockList = append(blockList.BlockList, blk)
	}
	return &blockList, nil
}

// create our definition of block
func (bb *BlockBlob) createBlock(blockIdLength, startIndex, size int64) *common.Block {
	newBlockId := base64.StdEncoding.EncodeToString(common.NewUUID(blockIdLength))
	newBlock := &common.Block{
		Id:         newBlockId,
		StartIndex: startIndex,
		EndIndex:   startIndex + size,
		Dirty:      true,
	}
	return newBlock
}

func (bb *BlockBlob) createNewBlocks(blockList *common.BlockOffsetList, offset, length, blockIdLength int64) int64 {
	prevIndex := blockList.BlockList[len(blockList.BlockList)-1].EndIndex
	// BufferSize is the size of the buffer that will go beyond our current blob (appended)
	var bufferSize int64
	for i := prevIndex; i < offset+length; i += bb.Config.blockSize {
		// create a new block if we hit our block size
		blkSize := int64(math.Min(float64(bb.Config.blockSize), float64((offset+length)-i)))
		newBlock := bb.createBlock(blockIdLength, i, blkSize)
		blockList.BlockList = append(blockList.BlockList, newBlock)
		// reset the counter since it will help us to determine if there is leftovers at the end
		bufferSize += blkSize
	}
	return bufferSize
}

// Write : write data at given offset to a blob
func (bb *BlockBlob) Write(options internal.WriteFileOptions) error {
	name := options.Handle.Path
	offset := options.Offset
	defer log.TimeTrack(time.Now(), "BlockBlob::Write", options.Handle.Path)
	log.Trace("BlockBlob::Write : name %s offset %v", name, offset)
	// tracks the case where our offset is great than our current file size (appending only - not modifying pre-existing data)
	var dataBuffer *[]byte

	fileOffsets := options.FileOffsets
	// when the file offset mapping is cached we don't need to make a get block list call
	if fileOffsets != nil && !fileOffsets.Cached {
		var err error
		fileOffsets, err = bb.GetFileBlockOffsets(name)
		if err != nil {
			return err
		}
	}

	length := int64(len(options.Data))
	data := options.Data
	// case 1: file consists of no blocks (small file)
	if fileOffsets != nil && len(fileOffsets.BlockList) == 0 {
		// get all the data
		oldData, _ := bb.ReadBuffer(name, 0, 0)
		// update the data with the new data
		// if we're only overwriting existing data
		if int64(len(oldData)) >= offset+length {
			copy(oldData[offset:], data)
			dataBuffer = &oldData
			// else appending and/or overwriting
		} else {
			// if the file is not empty then we need to combine the data
			if len(oldData) > 0 {
				// new data buffer with the size of old and new data
				newDataBuffer := make([]byte, offset+length)
				// copy the old data into it
				// TODO: better way to do this?
				if offset != 0 {
					copy(newDataBuffer, oldData)
					oldData = nil
				}
				// overwrite with the new data we want to add
				copy(newDataBuffer[offset:], data)
				dataBuffer = &newDataBuffer
			} else {
				dataBuffer = &data
			}
		}
		// WriteFromBuffer should be able to handle the case where now the block is too big and gets split into multiple blocks
		err := bb.WriteFromBuffer(name, options.Metadata, *dataBuffer)
		if err != nil {
			log.Err("BlockBlob::Write : Failed to upload to blob %s ", name, err.Error())
			return err
		}
		// case 2: given offset is within the size of the blob - and the blob consists of multiple blocks
		// case 3: new blocks need to be added
	} else {
		index, oldDataSize, exceedsFileBlocks, appendOnly := fileOffsets.FindBlocksToModify(offset, length)
		// keeps track of how much new data will be appended to the end of the file (applicable only to case 3)
		newBufferSize := int64(0)
		// case 3?
		if exceedsFileBlocks {
			// get length of blockID in order to generate a consistent size block ID so storage does not throw
			existingBlockId, _ := base64.StdEncoding.DecodeString(fileOffsets.BlockList[0].Id)
			blockIdLength := len(existingBlockId)
			newBufferSize = bb.createNewBlocks(fileOffsets, offset, length, int64(blockIdLength))
		}
		// buffer that holds that pre-existing data in those blocks we're interested in
		oldDataBuffer := make([]byte, oldDataSize+newBufferSize)
		if !appendOnly {
			// fetch the blocks that will be impacted by the new changes so we can overwrite them
			bb.ReadInBuffer(name, fileOffsets.BlockList[index].StartIndex, oldDataSize, oldDataBuffer)
		}
		// this gives us where the offset with respect to the buffer that holds our old data - so we can start writing the new data
		blockOffset := offset - fileOffsets.BlockList[index].StartIndex
		copy(oldDataBuffer[blockOffset:], data)
		err := bb.stageAndCommitModifiedBlocks(name, oldDataBuffer, index, fileOffsets)
		return err
	}
	return nil
}

// TODO: make a similar method facing stream that would enable us to write to cached blocks then stage and commit
func (bb *BlockBlob) stageAndCommitModifiedBlocks(name string, data []byte, index int, offsetList *common.BlockOffsetList) error {
	blobURL := bb.Container.NewBlockBlobURL(filepath.Join(bb.Config.prefixPath, name))
	blockOffset := int64(0)
	var blockIDList []string
	for _, blk := range offsetList.BlockList {
		blockIDList = append(blockIDList, blk.Id)
		if blk.Dirty {
			_, err := blobURL.StageBlock(context.Background(),
				blk.Id,
				bytes.NewReader(data[blockOffset:(blk.EndIndex-blk.StartIndex)+blockOffset]),
				bb.blobAccCond.LeaseAccessConditions,
				nil,
				bb.downloadOptions.ClientProvidedKeyOptions)
			if err != nil {
				log.Err("BlockBlob::stageAndCommitModifiedBlocks : Failed to stage to blob %s at block %v (%s)", name, blockOffset, err.Error())
				return err
			}
			blockOffset = (blk.EndIndex - blk.StartIndex) + blockOffset
		}
	}
	_, err := blobURL.CommitBlockList(context.Background(),
		blockIDList,
		azblob.BlobHTTPHeaders{ContentType: getContentType(name)},
		nil,
		bb.blobAccCond,
		bb.Config.defaultTier,
		nil, // datalake doesn't support tags here
		bb.downloadOptions.ClientProvidedKeyOptions)
	if err != nil {
		log.Err("BlockBlob::stageAndCommitModifiedBlocks : Failed to commit block list to blob %s (%s)", name, err.Error())
		return err
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
