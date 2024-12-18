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
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/streaming"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blockblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/container"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/service"
	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal"
	"github.com/Azure/azure-storage-fuse/v2/internal/stats_manager"
)

const (
	folderKey           = "hdi_isfolder"
	symlinkKey          = "is_symlink"
	max_context_timeout = 5
)

type BlockBlob struct {
	AzStorageConnection
	Auth            azAuth
	Service         *service.Client
	Container       *container.Client
	blobCPKOpt      *blob.CPKInfo
	downloadOptions *blob.DownloadFileOptions
	listDetails     container.ListBlobsInclude
	blockLocks      common.KeyedMutex
}

// Verify that BlockBlob implements AzConnection interface
var _ AzConnection = &BlockBlob{}

const (
	MaxBlobSize = blockblob.MaxStageBlockBytes * blockblob.MaxBlocks
)

func (bb *BlockBlob) Configure(cfg AzStorageConfig) error {
	bb.Config = cfg

	if bb.Config.cpkEnabled {
		bb.blobCPKOpt = &blob.CPKInfo{
			EncryptionKey:       &bb.Config.cpkEncryptionKey,
			EncryptionKeySHA256: &bb.Config.cpkEncryptionKeySha256,
			EncryptionAlgorithm: to.Ptr(blob.EncryptionAlgorithmTypeAES256),
		}
	}

	bb.downloadOptions = &blob.DownloadFileOptions{
		BlockSize:   bb.Config.blockSize,
		Concurrency: bb.Config.maxConcurrency,
		CPKInfo:     bb.blobCPKOpt,
	}

	bb.listDetails = container.ListBlobsInclude{
		Metadata:    true,
		Deleted:     false,
		Snapshots:   false,
		Permissions: false,
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

// UpdateServiceClient : Update the SAS specified by the user and create new service client
func (bb *BlockBlob) UpdateServiceClient(key, value string) (err error) {
	if key == "saskey" {
		bb.Auth.setOption(key, value)

		// get the service client with updated SAS
		svcClient, err := bb.Auth.getServiceClient(&bb.Config)
		if err != nil {
			log.Err("BlockBlob::UpdateServiceClient : Failed to get service client [%s]", err.Error())
			return err
		}

		// update the service client
		bb.Service = svcClient.(*service.Client)

		// Update the container client
		bb.Container = bb.Service.NewContainerClient(bb.Config.container)
	}
	return nil
}

// createServiceClient : Create the service client
func (bb *BlockBlob) createServiceClient() (*service.Client, error) {
	log.Trace("BlockBlob::createServiceClient : Getting service client")

	bb.Auth = getAzAuth(bb.Config.authConfig)
	if bb.Auth == nil {
		log.Err("BlockBlob::createServiceClient : Failed to retrieve auth object")
		return nil, fmt.Errorf("failed to retrieve auth object")
	}

	svcClient, err := bb.Auth.getServiceClient(&bb.Config)
	if err != nil {
		log.Err("BlockBlob::createServiceClient : Failed to get service client [%s]", err.Error())
		return nil, err
	}

	return svcClient.(*service.Client), nil
}

// SetupPipeline : Based on the config setup the ***URLs
func (bb *BlockBlob) SetupPipeline() error {
	log.Trace("BlockBlob::SetupPipeline : Setting up")
	var err error

	// create the service client
	bb.Service, err = bb.createServiceClient()
	if err != nil {
		log.Err("BlockBlob::SetupPipeline : Failed to get service client [%s]", err.Error())
		return err
	}

	// create the container client
	bb.Container = bb.Service.NewContainerClient(bb.Config.container)
	return nil
}

// TestPipeline : Validate the credentials specified in the auth config
func (bb *BlockBlob) TestPipeline() error {
	log.Trace("BlockBlob::TestPipeline : Validating")

	if bb.Config.mountAllContainers {
		return nil
	}

	if bb.Container == nil || bb.Container.URL() == "" {
		log.Err("BlockBlob::TestPipeline : Container Client is not built, check your credentials")
		return nil
	}

	listBlobPager := bb.Container.NewListBlobsHierarchyPager("/", &container.ListBlobsHierarchyOptions{
		MaxResults: to.Ptr((int32)(2)),
		Prefix:     &bb.Config.prefixPath,
	})

	// we are just validating the auth mode used. So, no need to iterate over the pages
	_, err := listBlobPager.NextPage(context.Background())
	if err != nil {
		log.Err("BlockBlob::TestPipeline : Failed to validate account with given auth %s", err.Error)
		return err
	}

	return nil
}

func (bb *BlockBlob) ListContainers() ([]string, error) {
	log.Trace("BlockBlob::ListContainers : Listing containers")
	cntList := make([]string, 0)

	pager := bb.Service.NewListContainersPager(nil)
	for pager.More() {
		resp, err := pager.NextPage(context.Background())
		if err != nil {
			log.Err("BlockBlob::ListContainers : Failed to get container list [%s]", err.Error())
			return cntList, err
		}
		for _, v := range resp.ContainerItems {
			cntList = append(cntList, *v.Name)
		}
	}

	return cntList, nil
}

func (bb *BlockBlob) SetPrefixPath(path string) error {
	log.Trace("BlockBlob::SetPrefixPath : path %s", path)
	bb.Config.prefixPath = path
	return nil
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
	metadata := make(map[string]*string)
	metadata[folderKey] = to.Ptr("true")

	return bb.WriteFromBuffer(name, metadata, data)
}

// CreateLink : Create a symlink in the container/virtual directory
func (bb *BlockBlob) CreateLink(source string, target string) error {
	log.Trace("BlockBlob::CreateLink : %s -> %s", source, target)
	data := []byte(target)
	metadata := make(map[string]*string)
	metadata[symlinkKey] = to.Ptr("true")
	return bb.WriteFromBuffer(source, metadata, data)
}

// DeleteFile : Delete a blob in the container/virtual directory
func (bb *BlockBlob) DeleteFile(name string) (err error) {
	log.Trace("BlockBlob::DeleteFile : name %s", name)

	blobClient := bb.Container.NewBlobClient(filepath.Join(bb.Config.prefixPath, name))
	_, err = blobClient.Delete(context.Background(), &blob.DeleteOptions{
		DeleteSnapshots: to.Ptr(blob.DeleteSnapshotsOptionTypeInclude),
	})
	if err != nil {
		serr := storeBlobErrToErr(err)
		if serr == ErrFileNotFound {
			log.Err("BlockBlob::DeleteFile : %s does not exist", name)
			return syscall.ENOENT
		} else if serr == BlobIsUnderLease {
			log.Err("BlockBlob::DeleteFile : %s is under lease [%s]", name, err.Error())
			return syscall.EIO
		} else {
			log.Err("BlockBlob::DeleteFile : Failed to delete blob %s [%s]", name, err.Error())
			return err
		}
	}

	return nil
}

// DeleteDirectory : Delete a virtual directory in the container/virtual directory
func (bb *BlockBlob) DeleteDirectory(name string) (err error) {
	log.Trace("BlockBlob::DeleteDirectory : name %s", name)

	pager := bb.Container.NewListBlobsFlatPager(&container.ListBlobsFlatOptions{
		Prefix: to.Ptr(filepath.Join(bb.Config.prefixPath, name) + "/"),
	})
	for pager.More() {
		listBlobResp, err := pager.NextPage(context.Background())
		if err != nil {
			log.Err("BlockBlob::DeleteDirectory : Failed to get list of blobs %s", err.Error())
			return err
		}

		// Process the blobs returned in this result segment (if the segment is empty, the loop body won't execute)
		for _, blobInfo := range listBlobResp.Segment.BlobItems {
			err = bb.DeleteFile(split(bb.Config.prefixPath, *blobInfo.Name))
			if err != nil {
				log.Err("BlockBlob::DeleteDirectory : Failed to delete file %s [%s]", *blobInfo.Name, err.Error())
			}
		}
	}

	err = bb.DeleteFile(name)
	// libfuse deletes the files in the directory before this method is called.
	// If the marker blob for directory is not present, ignore the ENOENT error.
	if err == syscall.ENOENT {
		err = nil
	}
	return err
}

// RenameFile : Rename the file
func (bb *BlockBlob) RenameFile(source string, target string) error {
	log.Trace("BlockBlob::RenameFile : %s -> %s", source, target)

	blobClient := bb.Container.NewBlockBlobClient(filepath.Join(bb.Config.prefixPath, source))
	newBlobClient := bb.Container.NewBlockBlobClient(filepath.Join(bb.Config.prefixPath, target))

	_, err := blobClient.GetProperties(context.Background(), &blob.GetPropertiesOptions{
		CPKInfo: bb.blobCPKOpt,
	})
	if err != nil {
		serr := storeBlobErrToErr(err)
		if serr == ErrFileNotFound {
			log.Err("BlockBlob::RenameFile : %s does not exist", source)
			return syscall.ENOENT
		} else {
			log.Err("BlockBlob::RenameFile : Failed to get blob properties for %s [%s]", source, err.Error())
			return err
		}
	}

	// not specifying source blob metadata, since passing empty metadata headers copies
	// the source blob metadata to destination blob
	startCopy, err := newBlobClient.StartCopyFromURL(context.Background(), blobClient.URL(), &blob.StartCopyFromURLOptions{
		Tier: bb.Config.defaultTier,
	})

	if err != nil {
		log.Err("BlockBlob::RenameFile : Failed to start copy of file %s [%s]", source, err.Error())
		return err
	}

	copyStatus := startCopy.CopyStatus
	for copyStatus != nil && *copyStatus == blob.CopyStatusTypePending {
		time.Sleep(time.Second * 1)
		prop, err := newBlobClient.GetProperties(context.Background(), &blob.GetPropertiesOptions{
			CPKInfo: bb.blobCPKOpt,
		})
		if err != nil {
			log.Err("BlockBlob::RenameFile : CopyStats : Failed to get blob properties for %s [%s]", source, err.Error())
		}
		copyStatus = prop.CopyStatus
	}

	log.Trace("BlockBlob::RenameFile : %s -> %s done", source, target)

	// Copy of the file is done so now delete the older file
	err = bb.DeleteFile(source)
	for retry := 0; retry < 3 && err == syscall.ENOENT; retry++ {
		// Sometimes backend is able to copy source file to destination but when we try to delete the
		// source files it returns back with ENOENT. If file was just created on backend it might happen
		// that it has not been synced yet at all layers and hence delete is not able to find the source file
		log.Trace("BlockBlob::RenameFile : %s -> %s, unable to find source. Retrying %d", source, target, retry)
		time.Sleep(1 * time.Second)
		err = bb.DeleteFile(source)
	}

	if err == syscall.ENOENT {
		// Even after 3 retries, 1 second apart if server returns 404 then source file no longer
		// exists on the backend and its safe to assume rename was successful
		err = nil
	}

	return err
}

// RenameDirectory : Rename the directory
func (bb *BlockBlob) RenameDirectory(source string, target string) error {
	log.Trace("BlockBlob::RenameDirectory : %s -> %s", source, target)

	srcDirPresent := false
	pager := bb.Container.NewListBlobsFlatPager(&container.ListBlobsFlatOptions{
		Prefix: to.Ptr(filepath.Join(bb.Config.prefixPath, source) + "/"),
	})
	for pager.More() {
		listBlobResp, err := pager.NextPage(context.Background())
		if err != nil {
			log.Err("BlockBlob::RenameDirectory : Failed to get list of blobs %s", err.Error())
			return err
		}

		// Process the blobs returned in this result segment (if the segment is empty, the loop body won't execute)
		for _, blobInfo := range listBlobResp.Segment.BlobItems {
			srcDirPresent = true
			srcPath := split(bb.Config.prefixPath, *blobInfo.Name)
			err = bb.RenameFile(srcPath, strings.Replace(srcPath, source, target, 1))
			if err != nil {
				log.Err("BlockBlob::RenameDirectory : Failed to rename file %s [%s]", srcPath, err.Error)
			}
		}
	}

	err := bb.RenameFile(source, target)
	// check if the marker blob for source directory does not exist but
	// blobs were present in it, which were renamed earlier
	if err == syscall.ENOENT && srcDirPresent {
		err = nil
	}
	return err
}

func (bb *BlockBlob) getAttrUsingRest(name string) (attr *internal.ObjAttr, err error) {
	log.Trace("BlockBlob::getAttrUsingRest : name %s", name)

	blobClient := bb.Container.NewBlockBlobClient(filepath.Join(bb.Config.prefixPath, name))
	prop, err := blobClient.GetProperties(context.Background(), &blob.GetPropertiesOptions{
		CPKInfo: bb.blobCPKOpt,
	})

	if err != nil {
		e := storeBlobErrToErr(err)
		if e == ErrFileNotFound {
			return attr, syscall.ENOENT
		} else if e == InvalidPermission {
			log.Err("BlockBlob::getAttrUsingRest : Insufficient permissions for %s [%s]", name, err.Error())
			return attr, syscall.EACCES
		} else {
			log.Err("BlockBlob::getAttrUsingRest : Failed to get blob properties for %s [%s]", name, err.Error())
			return attr, err
		}
	}

	// Since block blob does not support acls, we set mode to 0 and FlagModeDefault to true so the fuse layer can return the default permission.
	attr = &internal.ObjAttr{
		Path:   name, // We don't need to strip the prefixPath here since we pass the input name
		Name:   filepath.Base(name),
		Size:   *prop.ContentLength,
		Mode:   0,
		Mtime:  *prop.LastModified,
		Atime:  *prop.LastModified,
		Ctime:  *prop.LastModified,
		Crtime: *prop.CreationTime,
		Flags:  internal.NewFileBitMap(),
		MD5:    prop.ContentMD5,
	}

	parseMetadata(attr, prop.Metadata)

	attr.Flags.Set(internal.PropFlagMetadataRetrieved)
	attr.Flags.Set(internal.PropFlagModeDefault)

	return attr, nil
}

func (bb *BlockBlob) getAttrUsingList(name string) (attr *internal.ObjAttr, err error) {
	log.Trace("BlockBlob::getAttrUsingList : name %s", name)

	iteration := 0
	var marker, new_marker *string
	var blobs []*internal.ObjAttr
	blobsRead := 0

	for marker != nil || iteration == 0 {
		blobs, new_marker, err = bb.List(name, marker, bb.Config.maxResultsForList)
		if err != nil {
			e := storeBlobErrToErr(err)
			if e == ErrFileNotFound {
				return attr, syscall.ENOENT
			} else if e == InvalidPermission {
				log.Err("BlockBlob::getAttrUsingList : Insufficient permissions for %s [%s]", name, err.Error())
				return attr, syscall.EACCES
			} else {
				log.Warn("BlockBlob::getAttrUsingList : Failed to list blob properties for %s [%s]", name, err.Error())
			}
		}

		for i, blob := range blobs {
			log.Trace("BlockBlob::getAttrUsingList : Item %d Blob %s", i+blobsRead, blob.Name)
			if blob.Path == name {
				return blob, nil
			}
		}

		marker = new_marker
		iteration++
		blobsRead += len(blobs)

		log.Trace("BlockBlob::getAttrUsingList : So far retrieved %d objects in %d iterations", blobsRead, iteration)
		if new_marker == nil || *new_marker == "" {
			break
		}
	}

	if err == nil {
		log.Warn("BlockBlob::getAttrUsingList : blob %s does not exist", name)
		return nil, syscall.ENOENT
	}

	log.Err("BlockBlob::getAttrUsingList : Failed to list blob properties for %s [%s]", name, err.Error())
	return nil, err
}

// GetAttr : Retrieve attributes of the blob
func (bb *BlockBlob) GetAttr(name string) (attr *internal.ObjAttr, err error) {
	log.Trace("BlockBlob::GetAttr : name %s", name)

	// To support virtual directories with no marker blob, we call list instead of get properties since list will not return a 404
	if bb.Config.virtualDirectory {
		return bb.getAttrUsingList(name)
	}

	return bb.getAttrUsingRest(name)
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

	if count == 0 {
		count = common.MaxDirListCount
	}

	listPath := bb.getListPath(prefix)
	pager := bb.Container.NewListBlobsHierarchyPager("/", &container.ListBlobsHierarchyOptions{
		Marker:     marker,
		MaxResults: &count,
		Prefix:     &listPath,
		Include:    bb.listDetails,
	})

	listBlob, err := pager.NextPage(context.Background())
	if err != nil {
		log.Err("BlockBlob::List : Failed to list the container with the prefix %s", err.Error)
		return nil, nil, err
	}

	blobList, dirList, err := bb.processBlobItems(listBlob.Segment.BlobItems)
	if err != nil {
		return nil, nil, err
	}

	err = bb.processBlobPrefixes(listBlob.Segment.BlobPrefixes, dirList, &blobList)
	if err != nil {
		return nil, nil, err
	}

	return blobList, listBlob.NextMarker, nil
}

func (bb *BlockBlob) getListPath(prefix string) string {
	listPath := filepath.Join(bb.Config.prefixPath, prefix)
	if (prefix != "" && prefix[len(prefix)-1] == '/') || (prefix == "" && bb.Config.prefixPath != "") {
		listPath += "/"
	}
	return listPath
}

func (bb *BlockBlob) processBlobItems(blobItems []*container.BlobItem) ([]*internal.ObjAttr, map[string]bool, error) {
	blobList := make([]*internal.ObjAttr, 0)
	dirList := make(map[string]bool)

	for _, blobInfo := range blobItems {
		attr, err := bb.getBlobAttr(blobInfo)
		if err != nil {
			return nil, nil, err
		}
		blobList = append(blobList, attr)

		if attr.IsDir() {
			dirList[*blobInfo.Name+"/"] = true
			attr.Size = 4096
		}
	}

	return blobList, dirList, nil
}

func (bb *BlockBlob) getBlobAttr(blobInfo *container.BlobItem) (*internal.ObjAttr, error) {
	if blobInfo.Properties.CustomerProvidedKeySHA256 != nil && *blobInfo.Properties.CustomerProvidedKeySHA256 != "" {
		log.Trace("BlockBlob::List : blob is encrypted with customer provided key so fetching metadata explicitly using REST")
		return bb.getAttrUsingRest(*blobInfo.Name)
	}
	mode, err := bb.getFileMode(blobInfo.Properties.Permissions)
	if err != nil {
		return nil, err
	}

	attr := &internal.ObjAttr{
		Path:   split(bb.Config.prefixPath, *blobInfo.Name),
		Name:   filepath.Base(*blobInfo.Name),
		Size:   *blobInfo.Properties.ContentLength,
		Mode:   mode,
		Mtime:  *blobInfo.Properties.LastModified,
		Atime:  bb.dereferenceTime(blobInfo.Properties.LastAccessedOn, *blobInfo.Properties.LastModified),
		Ctime:  *blobInfo.Properties.LastModified,
		Crtime: bb.dereferenceTime(blobInfo.Properties.CreationTime, *blobInfo.Properties.LastModified),
		Flags:  internal.NewFileBitMap(),
		MD5:    blobInfo.Properties.ContentMD5,
	}
	parseMetadata(attr, blobInfo.Metadata)
	attr.Flags.Set(internal.PropFlagMetadataRetrieved)
	attr.Flags.Set(internal.PropFlagModeDefault)

	return attr, nil
}

func (bb *BlockBlob) getFileMode(permissions *string) (os.FileMode, error) {
	if permissions == nil {
		return 0, nil
	}
	return getFileMode(*permissions)
}

func (bb *BlockBlob) dereferenceTime(input *time.Time, defaultTime time.Time) time.Time {
	if input == nil {
		return defaultTime
	}
	return *input
}

func (bb *BlockBlob) processBlobPrefixes(blobPrefixes []*container.BlobPrefix, dirList map[string]bool, blobList *[]*internal.ObjAttr) error {
	for _, blobInfo := range blobPrefixes {
		if _, ok := dirList[*blobInfo.Name]; ok {
			continue
		}

		_, err := bb.getAttrUsingRest(*blobInfo.Name)
		if err == syscall.ENOENT {
			attr := bb.createDirAttr(*blobInfo.Name)
			*blobList = append(*blobList, attr)
		} else if bb.listDetails.Permissions {
			attr, err := bb.createDirAttrWithPermissions(blobInfo)
			if err != nil {
				return err
			}
			*blobList = append(*blobList, attr)
		}
	}

	return nil
}

func (bb *BlockBlob) createDirAttr(name string) *internal.ObjAttr {
	name = strings.TrimSuffix(name, "/")
	attr := &internal.ObjAttr{
		Path:  split(bb.Config.prefixPath, name),
		Name:  filepath.Base(name),
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
	return attr
}

func (bb *BlockBlob) createDirAttrWithPermissions(blobInfo *container.BlobPrefix) (*internal.ObjAttr, error) {
	if blobInfo.Properties == nil {
		return nil, fmt.Errorf("failed to get properties of blobprefix %s", *blobInfo.Name)
	}

	mode, err := bb.getFileMode(blobInfo.Properties.Permissions)
	if err != nil {
		mode = 0
		log.Warn("BlockBlob::createDirAttrWithPermissions : Failed to get file mode for %s [%s]", *blobInfo.Name, err.Error())
	}

	name := strings.TrimSuffix(*blobInfo.Name, "/")
	attr := &internal.ObjAttr{
		Path:   split(bb.Config.prefixPath, name),
		Name:   filepath.Base(name),
		Size:   *blobInfo.Properties.ContentLength,
		Mode:   mode,
		Mtime:  *blobInfo.Properties.LastModified,
		Atime:  bb.dereferenceTime(blobInfo.Properties.LastAccessedOn, *blobInfo.Properties.LastModified),
		Ctime:  *blobInfo.Properties.LastModified,
		Crtime: bb.dereferenceTime(blobInfo.Properties.CreationTime, *blobInfo.Properties.LastModified),
		Flags:  internal.NewDirBitMap(),
	}
	attr.Flags.Set(internal.PropFlagMetadataRetrieved)
	attr.Flags.Set(internal.PropFlagModeDefault)

	return attr, nil
}

// track the progress of download of blobs where every 100MB of data downloaded is being tracked. It also tracks the completion of download
func trackDownload(name string, bytesTransferred int64, count int64, downloadPtr *int64) {
	if bytesTransferred >= (*downloadPtr)*100*common.MbToBytes || bytesTransferred == count {
		(*downloadPtr)++
		log.Debug("BlockBlob::trackDownload : Download: Blob = %v, Bytes transferred = %v, Size = %v", name, bytesTransferred, count)

		// send the download progress as an event
		azStatsCollector.PushEvents(downloadProgress, name, map[string]interface{}{bytesTfrd: bytesTransferred, size: count})
	}
}

// ReadToFile : Download a blob to a local file
func (bb *BlockBlob) ReadToFile(name string, offset int64, count int64, fi *os.File) (err error) {
	log.Trace("BlockBlob::ReadToFile : name %s, offset : %d, count %d", name, offset, count)
	//defer exectime.StatTimeCurrentBlock("BlockBlob::ReadToFile")()

	blobClient := bb.Container.NewBlobClient(filepath.Join(bb.Config.prefixPath, name))

	downloadPtr := to.Ptr(int64(1))

	if common.MonitorBfs() {
		bb.downloadOptions.Progress = func(bytesTransferred int64) {
			trackDownload(name, bytesTransferred, count, downloadPtr)
		}
	}

	defer log.TimeTrack(time.Now(), "BlockBlob::ReadToFile", name)

	dlOpts := *bb.downloadOptions
	dlOpts.Range = blob.HTTPRange{
		Offset: offset,
		Count:  count,
	}

	_, err = blobClient.DownloadFile(context.Background(), fi, &dlOpts)

	if err != nil {
		e := storeBlobErrToErr(err)
		if e == ErrFileNotFound {
			return syscall.ENOENT
		} else {
			log.Err("BlockBlob::ReadToFile : Failed to download blob %s [%s]", name, err.Error())
			return err
		}
	} else {
		log.Debug("BlockBlob::ReadToFile : Download complete of blob %v", name)

		// store total bytes downloaded so far
		azStatsCollector.UpdateStats(stats_manager.Increment, bytesDownloaded, count)
	}

	if bb.Config.validateMD5 {
		// Compute md5 of local file
		fileMD5, err := getMD5(fi)
		if err != nil {
			log.Warn("BlockBlob::ReadToFile : Failed to generate MD5 Sum for %s", name)
		} else {
			// Get latest properties from container to get the md5 of blob
			prop, err := blobClient.GetProperties(context.Background(), &blob.GetPropertiesOptions{
				CPKInfo: bb.blobCPKOpt,
			})
			if err != nil {
				log.Warn("BlockBlob::ReadToFile : Failed to get properties of blob %s [%s]", name, err.Error())
			} else {
				blobMD5 := prop.ContentMD5
				if blobMD5 == nil {
					log.Warn("BlockBlob::ReadToFile : Failed to get MD5 Sum for blob %s", name)
				} else {
					// compare md5 and fail is not match
					if !reflect.DeepEqual(fileMD5, blobMD5) {
						log.Err("BlockBlob::ReadToFile : MD5 Sum mismatch %s", name)
						return errors.New("md5 sum mismatch on download")
					}
				}
			}
		}
	}

	return nil
}

// ReadBuffer : Download a specific range from a blob to a buffer
func (bb *BlockBlob) ReadBuffer(name string, offset int64, len int64) ([]byte, error) {
	log.Trace("BlockBlob::ReadBuffer : name %s, offset %v, len %v", name, offset, len)
	var buff []byte
	if len == 0 {
		attr, err := bb.GetAttr(name)
		if err != nil {
			return buff, err
		}
		len = attr.Size - offset
	}

	buff = make([]byte, len)
	blobClient := bb.Container.NewBlobClient(filepath.Join(bb.Config.prefixPath, name))

	dlOpts := (blob.DownloadBufferOptions)(*bb.downloadOptions)
	dlOpts.Range = blob.HTTPRange{
		Offset: offset,
		Count:  len,
	}

	_, err := blobClient.DownloadBuffer(context.Background(), buff, &dlOpts)

	if err != nil {
		e := storeBlobErrToErr(err)
		if e == ErrFileNotFound {
			return buff, syscall.ENOENT
		} else if e == InvalidRange {
			return buff, syscall.ERANGE
		}

		log.Err("BlockBlob::ReadBuffer : Failed to download blob %s [%s]", name, err.Error())
		return buff, err
	}

	return buff, nil
}

// ReadInBuffer : Download specific range from a file to a user provided buffer
func (bb *BlockBlob) ReadInBuffer(name string, offset int64, len int64, data []byte) error {
	// log.Trace("BlockBlob::ReadInBuffer : name %s", name)
	blobClient := bb.Container.NewBlobClient(filepath.Join(bb.Config.prefixPath, name))
	opt := (blob.DownloadBufferOptions)(*bb.downloadOptions)
	opt.BlockSize = len
	opt.Range = blob.HTTPRange{
		Offset: offset,
		Count:  len,
	}

	ctx, cancel := context.WithTimeout(context.Background(), max_context_timeout*time.Minute)
	defer cancel()

	_, err := blobClient.DownloadBuffer(ctx, data, &opt)

	if err != nil {
		e := storeBlobErrToErr(err)
		if e == ErrFileNotFound {
			return syscall.ENOENT
		} else if e == InvalidRange {
			return syscall.ERANGE
		}

		log.Err("BlockBlob::ReadInBuffer : Failed to download blob %s [%s]", name, err.Error())
		return err
	}

	return nil
}

func (bb *BlockBlob) calculateBlockSize(name string, fileSize int64) (blockSize int64, err error) {
	// If bufferSize > (BlockBlobMaxStageBlockBytes * BlockBlobMaxBlocks), then error
	if fileSize > MaxBlobSize {
		log.Err("BlockBlob::calculateBlockSize : buffer is too large to upload to a block blob %s", name)
		err = errors.New("buffer is too large to upload to a block blob")
		return 0, err
	}

	// If bufferSize <= BlockBlobMaxUploadBlobBytes, then Upload should be used with just 1 I/O request
	if fileSize <= blockblob.MaxUploadBlobBytes {
		// Files up to 256MB can be uploaded as a single block
		blockSize = blockblob.MaxUploadBlobBytes
	} else {
		// buffer / max blocks = block size to use all 50,000 blocks
		blockSize = int64(math.Ceil(float64(fileSize) / blockblob.MaxBlocks))

		if blockSize < blob.DefaultDownloadBlockSize {
			// Block size is smaller then 4MB then consider 4MB as default
			blockSize = blob.DefaultDownloadBlockSize
		} else {
			if (blockSize & (-8)) != 0 {
				// EXTRA : round off the block size to next higher multiple of 8.
				// No reason to do so just the odd numbers in block size will not be good on server end is assumption
				blockSize = (blockSize + 7) & (-8)
			}

			if blockSize > blockblob.MaxStageBlockBytes {
				// After rounding off the blockSize has become bigger then max allowed blocks.
				log.Err("BlockBlob::calculateBlockSize : blockSize exceeds max allowed block size for %s", name)
				err = errors.New("block-size is too large to upload to a block blob")
				return 0, err
			}
		}
	}

	log.Info("BlockBlob::calculateBlockSize : %s size %v, blockSize %v", name, fileSize, blockSize)
	return blockSize, nil
}

// track the progress of upload of blobs where every 100MB of data uploaded is being tracked. It also tracks the completion of upload
func trackUpload(name string, bytesTransferred int64, count int64, uploadPtr *int64) {
	if bytesTransferred >= (*uploadPtr)*100*common.MbToBytes || bytesTransferred == count {
		(*uploadPtr)++
		log.Debug("BlockBlob::trackUpload : Upload: Blob = %v, Bytes transferred = %v, Size = %v", name, bytesTransferred, count)

		// send upload progress as event
		azStatsCollector.PushEvents(uploadProgress, name, map[string]interface{}{bytesTfrd: bytesTransferred, size: count})
	}
}

// WriteFromFile : Upload local file to blob
func (bb *BlockBlob) WriteFromFile(name string, metadata map[string]*string, fi *os.File) (err error) {
	log.Trace("BlockBlob::WriteFromFile : name %s", name)
	//defer exectime.StatTimeCurrentBlock("WriteFromFile::WriteFromFile")()

	blobClient := bb.Container.NewBlockBlobClient(filepath.Join(bb.Config.prefixPath, name))
	defer log.TimeTrack(time.Now(), "BlockBlob::WriteFromFile", name)

	uploadPtr := to.Ptr(int64(1))

	blockSize := bb.Config.blockSize
	// get the size of the file
	stat, err := fi.Stat()
	if err != nil {
		log.Err("BlockBlob::WriteFromFile : Failed to get file size %s [%s]", name, err.Error())
		return err
	}

	// if the block size is not set then we configure it based on file size
	if blockSize == 0 {
		// based on file-size calculate block size
		blockSize, err = bb.calculateBlockSize(name, stat.Size())
		if err != nil {
			return err
		}
	}

	// Compute md5 of this file is requested by user
	// If file is uploaded in one shot (no blocks created) then server is populating md5 on upload automatically.
	// hence we take cost of calculating md5 only for files which are bigger in size and which will be converted to blocks.
	md5sum := []byte{}
	if bb.Config.updateMD5 && stat.Size() >= blockblob.MaxUploadBlobBytes {
		md5sum, err = getMD5(fi)
		if err != nil {
			// Md5 sum generation failed so set nil while uploading
			log.Warn("BlockBlob::WriteFromFile : Failed to generate md5 of %s", name)
			md5sum = []byte{0}
		}
	}

	uploadOptions := &blockblob.UploadFileOptions{
		BlockSize:   blockSize,
		Concurrency: bb.Config.maxConcurrency,
		Metadata:    metadata,
		AccessTier:  bb.Config.defaultTier,
		HTTPHeaders: &blob.HTTPHeaders{
			BlobContentType: to.Ptr(getContentType(name)),
			BlobContentMD5:  md5sum,
		},
		CPKInfo: bb.blobCPKOpt,
	}
	if common.MonitorBfs() && stat.Size() > 0 {
		uploadOptions.Progress = func(bytesTransferred int64) {
			trackUpload(name, bytesTransferred, stat.Size(), uploadPtr)
		}
	}

	_, err = blobClient.UploadFile(context.Background(), fi, uploadOptions)

	if err != nil {
		serr := storeBlobErrToErr(err)
		if serr == BlobIsUnderLease {
			log.Err("BlockBlob::WriteFromFile : %s is under a lease, can not update file [%s]", name, err.Error())
			return syscall.EIO
		} else if serr == InvalidPermission {
			log.Err("BlockBlob::WriteFromFile : Insufficient permissions for %s [%s]", name, err.Error())
			return syscall.EACCES
		} else {
			log.Err("BlockBlob::WriteFromFile : Failed to upload blob %s [%s]", name, err.Error())
		}
		return err
	} else {
		log.Debug("BlockBlob::WriteFromFile : Upload complete of blob %v", name)

		// store total bytes uploaded so far
		if stat.Size() > 0 {
			azStatsCollector.UpdateStats(stats_manager.Increment, bytesUploaded, stat.Size())
		}
	}

	return nil
}

// WriteFromBuffer : Upload from a buffer to a blob
func (bb *BlockBlob) WriteFromBuffer(name string, metadata map[string]*string, data []byte) error {
	log.Trace("BlockBlob::WriteFromBuffer : name %s", name)
	blobClient := bb.Container.NewBlockBlobClient(filepath.Join(bb.Config.prefixPath, name))

	defer log.TimeTrack(time.Now(), "BlockBlob::WriteFromBuffer", name)

	_, err := blobClient.UploadBuffer(context.Background(), data, &blockblob.UploadBufferOptions{
		BlockSize:   bb.Config.blockSize,
		Concurrency: bb.Config.maxConcurrency,
		Metadata:    metadata,
		AccessTier:  bb.Config.defaultTier,
		HTTPHeaders: &blob.HTTPHeaders{
			BlobContentType: to.Ptr(getContentType(name)),
		},
		CPKInfo: bb.blobCPKOpt,
	})

	if err != nil {
		log.Err("BlockBlob::WriteFromBuffer : Failed to upload blob %s [%s]", name, err.Error())
		return err
	}

	return nil
}

// GetFileBlockOffsets: store blocks ids and corresponding offsets
func (bb *BlockBlob) GetFileBlockOffsets(name string) (*common.BlockOffsetList, error) {
	var blockOffset int64 = 0
	blockList := common.BlockOffsetList{}
	blobClient := bb.Container.NewBlockBlobClient(filepath.Join(bb.Config.prefixPath, name))

	storageBlockList, err := blobClient.GetBlockList(context.Background(), blockblob.BlockListTypeCommitted, nil)

	if err != nil {
		log.Err("BlockBlob::GetFileBlockOffsets : Failed to get block list %s ", name, err.Error())
		return &common.BlockOffsetList{}, err
	}

	// if block list empty its a small file
	if len(storageBlockList.CommittedBlocks) == 0 {
		blockList.Flags.Set(common.SmallFile)
		return &blockList, nil
	}

	for _, block := range storageBlockList.CommittedBlocks {
		blk := &common.Block{
			Id:         *block.Name,
			StartIndex: int64(blockOffset),
			EndIndex:   int64(blockOffset) + *block.Size,
		}
		blockOffset += *block.Size
		blockList.BlockList = append(blockList.BlockList, blk)
	}
	// blockList.Etag = storageBlockList.ETag()
	blockList.BlockIdLength = common.GetIdLength(blockList.BlockList[0].Id)
	return &blockList, nil
}

func (bb *BlockBlob) createBlock(blockIdLength, startIndex, size int64) *common.Block {
	newBlockId := base64.StdEncoding.EncodeToString(common.NewUUIDWithLength(blockIdLength))
	newBlock := &common.Block{
		Id:         newBlockId,
		StartIndex: startIndex,
		EndIndex:   startIndex + size,
	}
	// mark truncated since it is a new empty block
	newBlock.Flags.Set(common.TruncatedBlock)
	newBlock.Flags.Set(common.DirtyBlock)
	return newBlock
}

// create new blocks based on the offset and total length we're adding to the file
func (bb *BlockBlob) createNewBlocks(blockList *common.BlockOffsetList, offset, length int64) (int64, error) {
	blockSize := bb.Config.blockSize
	prevIndex := blockList.BlockList[len(blockList.BlockList)-1].EndIndex
	numOfBlocks := int64(len(blockList.BlockList))
	if blockSize == 0 {
		blockSize = (16 * 1024 * 1024)
		if math.Ceil((float64)(numOfBlocks)+(float64)(length)/(float64)(blockSize)) > blockblob.MaxBlocks {
			blockSize = int64(math.Ceil((float64)(length) / (float64)(blockblob.MaxBlocks-numOfBlocks)))
			if blockSize > blockblob.MaxStageBlockBytes {
				return 0, errors.New("cannot accommodate data within the block limit")
			}
		}
	} else if math.Ceil((float64)(numOfBlocks)+(float64)(length)/(float64)(blockSize)) > blockblob.MaxBlocks {
		return 0, errors.New("cannot accommodate data within the block limit with configured block-size")
	}

	// BufferSize is the size of the buffer that will go beyond our current blob (appended)
	var bufferSize int64
	for i := prevIndex; i < offset+length; i += blockSize {
		blkSize := int64(math.Min(float64(blockSize), float64((offset+length)-i)))
		newBlock := bb.createBlock(blockList.BlockIdLength, i, blkSize)
		blockList.BlockList = append(blockList.BlockList, newBlock)
		// reset the counter to determine if there are leftovers at the end
		bufferSize += blkSize
	}
	return bufferSize, nil
}

func (bb *BlockBlob) removeBlocks(blockList *common.BlockOffsetList, size int64, name string) *common.BlockOffsetList {
	_, index := blockList.BinarySearch(size)
	// if the start index is equal to new size - block should be removed - move one index back
	if blockList.BlockList[index].StartIndex == size {
		index = index - 1
	}
	// if the file we're shrinking is in the middle of a block then shrink that block
	if blockList.BlockList[index].EndIndex > size {
		blk := blockList.BlockList[index]
		blk.EndIndex = size
		blk.Data = make([]byte, blk.EndIndex-blk.StartIndex)
		blk.Flags.Set(common.DirtyBlock)

		err := bb.ReadInBuffer(name, blk.StartIndex, blk.EndIndex-blk.StartIndex, blk.Data)
		if err != nil {
			log.Err("BlockBlob::removeBlocks : Failed to remove blocks %s [%s]", name, err.Error())
		}

	}
	blk := blockList.BlockList[index]
	blk.Flags.Set(common.RemovedBlocks)
	blockList.BlockList = blockList.BlockList[:index+1]

	return blockList
}

func (bb *BlockBlob) TruncateFile(name string, size int64) error {
	// log.Trace("BlockBlob::TruncateFile : name=%s, size=%d", name, size)
	attr, err := bb.GetAttr(name)
	if err != nil {
		log.Err("BlockBlob::TruncateFile : Failed to get attributes of file %s [%s]", name, err.Error())
		if err == syscall.ENOENT {
			return err
		}
	}
	if size == 0 || attr.Size == 0 {
		// If we are resizing to a value > 1GB then we need to upload multiple blocks to resize
		if size > 1*common.GbToBytes {
			blkSize := int64(16 * common.MbToBytes)
			blobName := filepath.Join(bb.Config.prefixPath, name)
			blobClient := bb.Container.NewBlockBlobClient(blobName)

			blkList := make([]string, 0)
			id := base64.StdEncoding.EncodeToString(common.NewUUIDWithLength(16))

			for i := 0; size > 0; i++ {
				if i == 0 || size < blkSize {
					// Only first and last block we upload and rest all we replicate with the first block itself
					if size < blkSize {
						blkSize = size
						id = base64.StdEncoding.EncodeToString(common.NewUUIDWithLength(16))
					}
					data := make([]byte, blkSize)

					_, err = blobClient.StageBlock(context.Background(),
						id,
						streaming.NopCloser(bytes.NewReader(data)),
						&blockblob.StageBlockOptions{
							CPKInfo: bb.blobCPKOpt,
						})
					if err != nil {
						log.Err("BlockBlob::TruncateFile : Failed to stage block for %s [%s]", name, err.Error())
						return err
					}
				}
				blkList = append(blkList, id)
				size -= blkSize
			}

			err = bb.CommitBlocks(blobName, blkList)
			if err != nil {
				log.Err("BlockBlob::TruncateFile : Failed to commit blocks for %s [%s]", name, err.Error())
				return err
			}
		} else {
			err := bb.WriteFromBuffer(name, nil, make([]byte, size))
			if err != nil {
				log.Err("BlockBlob::TruncateFile : Failed to set the %s to 0 bytes [%s]", name, err.Error())
			}
		}
		return err
	}

	//If new size is less than 256MB
	if size < blockblob.MaxUploadBlobBytes {
		data, err := bb.HandleSmallFile(name, size, attr.Size)
		if err != nil {
			log.Err("BlockBlob::TruncateFile : Failed to read small file %s", name, err.Error())
			return err
		}
		err = bb.WriteFromBuffer(name, nil, data)
		if err != nil {
			log.Err("BlockBlob::TruncateFile : Failed to write from buffer file %s", name, err.Error())
			return err
		}
	} else {
		bol, err := bb.GetFileBlockOffsets(name)
		if err != nil {
			log.Err("BlockBlob::TruncateFile : Failed to get block list of file %s [%s]", name, err.Error())
			return err
		}
		if bol.SmallFile() {
			data, err := bb.HandleSmallFile(name, size, attr.Size)
			if err != nil {
				log.Err("BlockBlob::TruncateFile : Failed to read small file %s", name, err.Error())
				return err
			}
			err = bb.WriteFromBuffer(name, nil, data)
			if err != nil {
				log.Err("BlockBlob::TruncateFile : Failed to write from buffer file %s", name, err.Error())
				return err
			}
		} else {
			if size < attr.Size {
				bol = bb.removeBlocks(bol, size, name)
			} else if size > attr.Size {
				_, err = bb.createNewBlocks(bol, bol.BlockList[len(bol.BlockList)-1].EndIndex, size-attr.Size)
				if err != nil {
					log.Err("BlockBlob::TruncateFile : Failed to create new blocks for file %s", name, err.Error())
					return err
				}
			}
			err = bb.StageAndCommit(name, bol)
			if err != nil {
				log.Err("BlockBlob::TruncateFile : Failed to stage and commit file %s", name, err.Error())
				return err
			}
		}
	}

	return nil
}

func (bb *BlockBlob) HandleSmallFile(name string, size int64, originalSize int64) ([]byte, error) {
	var data = make([]byte, size)
	var err error
	if size > originalSize {
		err = bb.ReadInBuffer(name, 0, 0, data)
		if err != nil {
			log.Err("BlockBlob::TruncateFile : Failed to read small file %s", name, err.Error())
		}
	} else {
		err = bb.ReadInBuffer(name, 0, size, data)
		if err != nil {
			log.Err("BlockBlob::TruncateFile : Failed to read small file %s", name, err.Error())
		}
	}
	return data, err
}

// Write : write data at given offset to a blob
func (bb *BlockBlob) Write(options internal.WriteFileOptions) error {
	name := options.Handle.Path
	offset := options.Offset
	defer log.TimeTrack(time.Now(), "BlockBlob::Write", options.Handle.Path)
	log.Trace("BlockBlob::Write : name %s offset %v", name, offset)
	// tracks the case where our offset is great than our current file size (appending only - not modifying pre-existing data)
	var dataBuffer *[]byte
	// when the file offset mapping is cached we don't need to make a get block list call
	fileOffsets, err := bb.GetFileBlockOffsets(name)
	if err != nil {
		return err
	}
	length := int64(len(options.Data))
	data := options.Data
	// case 1: file consists of no blocks (small file)
	if fileOffsets.SmallFile() {
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
			newBufferSize, err = bb.createNewBlocks(fileOffsets, offset, length)
			if err != nil {
				log.Err("BlockBlob::Write : Failed to create new blocks for file %s", name, err.Error())
				return err
			}
		}
		// buffer that holds that pre-existing data in those blocks we're interested in
		oldDataBuffer := make([]byte, oldDataSize+newBufferSize)
		if !appendOnly {
			// fetch the blocks that will be impacted by the new changes so we can overwrite them
			err = bb.ReadInBuffer(name, fileOffsets.BlockList[index].StartIndex, oldDataSize, oldDataBuffer)
			if err != nil {
				log.Err("BlockBlob::Write : Failed to read data in buffer %s [%s]", name, err.Error())
			}
		}
		// this gives us where the offset with respect to the buffer that holds our old data - so we can start writing the new data
		blockOffset := offset - fileOffsets.BlockList[index].StartIndex
		copy(oldDataBuffer[blockOffset:], data)
		err := bb.stageAndCommitModifiedBlocks(name, oldDataBuffer, fileOffsets)
		return err
	}
	return nil
}

// TODO: make a similar method facing stream that would enable us to write to cached blocks then stage and commit
func (bb *BlockBlob) stageAndCommitModifiedBlocks(name string, data []byte, offsetList *common.BlockOffsetList) error {
	blobClient := bb.Container.NewBlockBlobClient(filepath.Join(bb.Config.prefixPath, name))
	blockOffset := int64(0)
	var blockIDList []string
	for _, blk := range offsetList.BlockList {
		blockIDList = append(blockIDList, blk.Id)
		if blk.Dirty() {
			_, err := blobClient.StageBlock(context.Background(),
				blk.Id,
				streaming.NopCloser(bytes.NewReader(data[blockOffset:(blk.EndIndex-blk.StartIndex)+blockOffset])),
				&blockblob.StageBlockOptions{
					CPKInfo: bb.blobCPKOpt,
				})

			if err != nil {
				log.Err("BlockBlob::stageAndCommitModifiedBlocks : Failed to stage to blob %s at block %v [%s]", name, blockOffset, err.Error())
				return err
			}
			blockOffset = (blk.EndIndex - blk.StartIndex) + blockOffset
		}
	}
	_, err := blobClient.CommitBlockList(context.Background(),
		blockIDList,
		&blockblob.CommitBlockListOptions{
			HTTPHeaders: &blob.HTTPHeaders{
				BlobContentType: to.Ptr(getContentType(name)),
			},
			Tier:    bb.Config.defaultTier,
			CPKInfo: bb.blobCPKOpt,
		})

	if err != nil {
		log.Err("BlockBlob::stageAndCommitModifiedBlocks : Failed to commit block list to blob %s [%s]", name, err.Error())
		return err
	}
	return nil
}

func (bb *BlockBlob) StageAndCommit(name string, bol *common.BlockOffsetList) error {
	// lock on the blob name so that no stage and commit race condition occur causing failure
	blobMtx := bb.blockLocks.GetLock(name)
	blobMtx.Lock()
	defer blobMtx.Unlock()
	blobClient := bb.Container.NewBlockBlobClient(filepath.Join(bb.Config.prefixPath, name))
	var blockIDList []string
	var data []byte
	staged := false
	for _, blk := range bol.BlockList {
		blockIDList = append(blockIDList, blk.Id)
		if blk.Truncated() {
			data = make([]byte, blk.EndIndex-blk.StartIndex)
			blk.Flags.Clear(common.TruncatedBlock)
		} else {
			data = blk.Data
		}
		if blk.Dirty() {
			_, err := blobClient.StageBlock(context.Background(),
				blk.Id,
				streaming.NopCloser(bytes.NewReader(data)),
				&blockblob.StageBlockOptions{
					CPKInfo: bb.blobCPKOpt,
				})
			if err != nil {
				log.Err("BlockBlob::StageAndCommit : Failed to stage to blob %s with ID %s at block %v [%s]", name, blk.Id, blk.StartIndex, err.Error())
				return err
			}
			staged = true
			blk.Flags.Clear(common.DirtyBlock)
		} else if blk.Removed() {
			staged = true
		}
	}
	if staged {
		_, err := blobClient.CommitBlockList(context.Background(),
			blockIDList,
			&blockblob.CommitBlockListOptions{
				HTTPHeaders: &blob.HTTPHeaders{
					BlobContentType: to.Ptr(getContentType(name)),
				},
				Tier:    bb.Config.defaultTier,
				CPKInfo: bb.blobCPKOpt,
				// AccessConditions: &blob.AccessConditions{ModifiedAccessConditions: &blob.ModifiedAccessConditions{IfMatch: bol.Etag}},
			})
		if err != nil {
			log.Err("BlockBlob::StageAndCommit : Failed to commit block list to blob %s [%s]", name, err.Error())
			return err
		}
		// update the etag
		// bol.Etag = resp.ETag()
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

// GetCommittedBlockList : Get the list of committed blocks
func (bb *BlockBlob) GetCommittedBlockList(name string) (*internal.CommittedBlockList, error) {
	blobClient := bb.Container.NewBlockBlobClient(filepath.Join(bb.Config.prefixPath, name))

	storageBlockList, err := blobClient.GetBlockList(context.Background(), blockblob.BlockListTypeCommitted, nil)

	if err != nil {
		log.Err("BlockBlob::GetFileBlockOffsets : Failed to get block list %s ", name, err.Error())
		return nil, err
	}

	// if block list empty its a small file
	if len(storageBlockList.CommittedBlocks) == 0 {
		return nil, nil
	}

	blockList := make(internal.CommittedBlockList, 0)
	startOffset := int64(0)
	for _, block := range storageBlockList.CommittedBlocks {
		blk := internal.CommittedBlock{
			Id:     *block.Name,
			Offset: startOffset,
			Size:   uint64(*block.Size),
		}
		startOffset += *block.Size
		blockList = append(blockList, blk)
	}

	return &blockList, nil
}

// StageBlock : stages a block and returns its blockid
func (bb *BlockBlob) StageBlock(name string, data []byte, id string) error {
	log.Trace("BlockBlob::StageBlock : name %s, ID %v, length %v", name, id, len(data))

	ctx, cancel := context.WithTimeout(context.Background(), max_context_timeout*time.Minute)
	defer cancel()

	blobClient := bb.Container.NewBlockBlobClient(filepath.Join(bb.Config.prefixPath, name))
	_, err := blobClient.StageBlock(ctx,
		id,
		streaming.NopCloser(bytes.NewReader(data)),
		&blockblob.StageBlockOptions{
			CPKInfo: bb.blobCPKOpt,
		})

	if err != nil {
		log.Err("BlockBlob::StageBlock : Failed to stage to blob %s with ID %s [%s]", name, id, err.Error())
		return err
	}

	return nil
}

// CommitBlocks : persists the block list
func (bb *BlockBlob) CommitBlocks(name string, blockList []string) error {
	log.Trace("BlockBlob::CommitBlocks : name %s", name)

	ctx, cancel := context.WithTimeout(context.Background(), max_context_timeout*time.Minute)
	defer cancel()

	blobClient := bb.Container.NewBlockBlobClient(filepath.Join(bb.Config.prefixPath, name))
	_, err := blobClient.CommitBlockList(ctx,
		blockList,
		&blockblob.CommitBlockListOptions{
			HTTPHeaders: &blob.HTTPHeaders{
				BlobContentType: to.Ptr(getContentType(name)),
			},
			Tier:    bb.Config.defaultTier,
			CPKInfo: bb.blobCPKOpt,
		})

	if err != nil {
		log.Err("BlockBlob::CommitBlocks : Failed to commit block list to blob %s [%s]", name, err.Error())
		return err
	}

	return nil
}
