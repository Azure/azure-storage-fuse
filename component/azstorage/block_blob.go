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
	"math"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"time"

	"github.com/Azure/azure-pipeline-go/pipeline"
	"github.com/Azure/azure-storage-azcopy/v10/ste"
	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal"
	"github.com/Azure/azure-storage-fuse/v2/internal/stats_manager"

	"github.com/Azure/azure-storage-blob-go/azblob"
)

const (
	folderKey           = "hdi_isfolder"
	symlinkKey          = "is_symlink"
	max_context_timeout = 5
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
	blockLocks      common.KeyedMutex
}

// Verify that BlockBlob implements AzConnection interface
var _ AzConnection = &BlockBlob{}

const (
	MaxBlocksSize = azblob.BlockBlobMaxStageBlockBytes * azblob.BlockBlobMaxBlocks
)

func (bb *BlockBlob) Configure(cfg AzStorageConfig) error {
	bb.Config = cfg

	bb.blobAccCond = azblob.BlobAccessConditions{}
	if bb.Config.cpkEnabled {
		bb.blobCPKOpt = azblob.ClientProvidedKeyOptions{
			EncryptionKey:       &bb.Config.cpkEncryptionKey,
			EncryptionKeySha256: &bb.Config.cpkEncryptionKeySha256,
			EncryptionAlgorithm: "AES256",
		}
	} else {
		bb.blobCPKOpt = azblob.ClientProvidedKeyOptions{}
	}

	bb.downloadOptions = azblob.DownloadFromBlobOptions{
		BlockSize:                bb.Config.blockSize,
		Parallelism:              bb.Config.maxConcurrency,
		ClientProvidedKeyOptions: bb.blobCPKOpt,
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
			log.Err("BlockBlob::NewCredentialKey : Failed to form base endpoint url [%s]", err.Error())
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
	if cred == nil {
		log.Err("BlockBlob::getCredential : Failed to get credential")
		return nil
	}

	return cred.(azblob.Credential)
}

// NewPipeline creates a Pipeline using the specified credentials and options.
func NewBlobPipeline(c azblob.Credential, o azblob.PipelineOptions, ro ste.XferRetryOptions) pipeline.Pipeline {
	// Closest to API goes first; closest to the wire goes last
	f := []pipeline.Factory{
		azblob.NewTelemetryPolicyFactory(o.Telemetry),
		azblob.NewUniqueRequestIDPolicyFactory(),
		ste.NewBlobXferRetryPolicyFactory(ro),
	}
	f = append(f, c)
	f = append(f,
		pipeline.MethodFactoryMarker(), // indicates at what stage in the pipeline the method factory is invoked
		ste.NewRequestLogPolicyFactory(ste.RequestLogOptions{
			LogWarningIfTryOverThreshold: o.RequestLog.LogWarningIfTryOverThreshold,
			SyslogDisabled:               o.RequestLog.SyslogDisabled,
		}))

	return pipeline.NewPipeline(f, pipeline.Options{HTTPSender: o.HTTPSender, Log: o.Log})
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
	options, retryOptions := getAzBlobPipelineOptions(bb.Config)
	bb.Pipeline = NewBlobPipeline(cred, options, retryOptions)
	if bb.Pipeline == nil {
		log.Err("BlockBlob::SetupPipeline : Failed to create pipeline object")
		return errors.New("failed to create pipeline object")
	}

	// Get the endpoint url from the credential
	bb.Endpoint, err = url.Parse(bb.Auth.getEndpoint())
	if err != nil {
		log.Err("BlockBlob::SetupPipeline : Failed to form base end point url [%s]", err.Error())
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
		azblob.ListBlobsSegmentOptions{MaxResults: 2,
			Prefix: bb.Config.prefixPath,
		})

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
			err = bb.DeleteFile(split(bb.Config.prefixPath, blobInfo.Name))
			if err != nil {
				log.Err("BlockBlob::DeleteDirectory : Failed to delete file %s [%s]", blobInfo.Name, err.Error)
			}
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
			log.Err("BlockBlob::RenameFile : Failed to get blob properties for %s [%s]", source, err.Error())
			return err
		}
	}

	startCopy, err := newBlob.StartCopyFromURL(context.Background(), blobURL.URL(),
		prop.NewMetadata(), azblob.ModifiedAccessConditions{}, azblob.BlobAccessConditions{}, bb.Config.defaultTier, nil)

	if err != nil {
		log.Err("BlockBlob::RenameFile : Failed to start copy of file %s [%s]", source, err.Error())
		return err
	}

	copyStatus := startCopy.CopyStatus()
	for copyStatus == azblob.CopyStatusPending {
		time.Sleep(time.Second * 1)
		prop, err = newBlob.GetProperties(context.Background(), bb.blobAccCond, bb.blobCPKOpt)
		if err != nil {
			log.Err("BlockBlob::RenameFile : CopyStats : Failed to get blob properties for %s [%s]", source, err.Error())
		}
		copyStatus = prop.CopyStatus()
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
			srcDirPresent = true
			srcPath := split(bb.Config.prefixPath, blobInfo.Name)
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

	blobURL := bb.Container.NewBlockBlobURL(filepath.Join(bb.Config.prefixPath, name))
	prop, err := blobURL.GetProperties(context.Background(), bb.blobAccCond, bb.blobCPKOpt)

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
		Size:   prop.ContentLength(),
		Mode:   0,
		Mtime:  prop.LastModified(),
		Atime:  prop.LastModified(),
		Ctime:  prop.LastModified(),
		Crtime: prop.CreationTime(),
		Flags:  internal.NewFileBitMap(),
		MD5:    prop.ContentMD5(),
	}

	parseMetadata(attr, prop.NewMetadata())

	attr.Flags.Set(internal.PropFlagMetadataRetrieved)
	attr.Flags.Set(internal.PropFlagModeDefault)

	return attr, nil
}

func (bb *BlockBlob) getAttrUsingList(name string) (attr *internal.ObjAttr, err error) {
	log.Trace("BlockBlob::getAttrUsingList : name %s", name)

	const maxFailCount = 20
	failCount := 0
	iteration := 0

	var marker *string = nil
	blobsRead := 0

	for failCount < maxFailCount {
		blobs, new_marker, err := bb.List(name, marker, bb.Config.maxResultsForList)
		if err != nil {
			e := storeBlobErrToErr(err)
			if e == ErrFileNotFound {
				return attr, syscall.ENOENT
			} else if e == InvalidPermission {
				log.Err("BlockBlob::getAttrUsingList : Insufficient permissions for %s [%s]", name, err.Error())
				return attr, syscall.EACCES
			} else {
				log.Warn("BlockBlob::getAttrUsingList : Failed to list blob properties for %s [%s]", name, err.Error())
				failCount++
				continue
			}
		}
		failCount = 0

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
		if new_marker == nil || *new_marker == "" || failCount >= maxFailCount {
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
			MD5:    blobInfo.Properties.ContentMD5,
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

	// In case virtual directory exists but its corresponding 0 byte marker file is not there holding hdi_isfolder then just iterating
	// over BlobItems will fail to identify that directory. In such cases BlobPrefixes help to list all directories
	// dirList contains all dirs for which we got 0 byte meta file in this iteration, so exclude those and add rest to the list
	// Note: Since listing is paginated, sometimes the marker file may come in a different iteration from the BlobPrefix. For such
	// cases we manually call GetAttr to check the existence of the marker file.
	for _, blobInfo := range listBlob.Segment.BlobPrefixes {
		if _, ok := dirList[blobInfo.Name]; ok {
			// marker file found in current iteration, skip adding the directory
			continue
		} else {
			// marker file not found in current iteration, so we need to manually check attributes via REST
			_, err := bb.getAttrUsingRest(blobInfo.Name)
			// marker file also not found via manual check, safe to add to list
			if err == syscall.ENOENT {
				// For these dirs we get only the name and no other properties so hardcoding time to current time
				name := strings.TrimSuffix(blobInfo.Name, "/")
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
				blobList = append(blobList, attr)
			}
		}
	}

	// Clean up the temp map as its no more needed
	for k := range dirList {
		delete(dirList, k)
	}

	return blobList, listBlob.NextMarker.Val, nil
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

	blobURL := bb.Container.NewBlobURL(filepath.Join(bb.Config.prefixPath, name))

	var downloadPtr *int64 = new(int64)
	*downloadPtr = 1

	if common.MonitorBfs() {
		bb.downloadOptions.Progress = func(bytesTransferred int64) {
			trackDownload(name, bytesTransferred, count, downloadPtr)
		}
	}

	defer log.TimeTrack(time.Now(), "BlockBlob::ReadToFile", name)
	err = azblob.DownloadBlobToFile(context.Background(), blobURL, offset, count, fi, bb.downloadOptions)

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
			prop, err := blobURL.GetProperties(context.Background(), bb.blobAccCond, bb.blobCPKOpt)
			if err != nil {
				log.Warn("BlockBlob::ReadToFile : Failed to get properties of blob %s [%s]", name, err.Error())
			} else {
				blobMD5 := prop.ContentMD5()
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
	blobURL := bb.Container.NewBlobURL(filepath.Join(bb.Config.prefixPath, name))
	err := azblob.DownloadBlobToBuffer(context.Background(), blobURL, offset, len, buff, bb.downloadOptions)

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
	blobURL := bb.Container.NewBlobURL(filepath.Join(bb.Config.prefixPath, name))
	opt := bb.downloadOptions
	opt.BlockSize = len

	ctx, cancel := context.WithTimeout(context.Background(), max_context_timeout*time.Minute)
	defer cancel()

	err := azblob.DownloadBlobToBuffer(ctx, blobURL, offset, len, data, opt)

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
	if fileSize > MaxBlocksSize {
		log.Err("BlockBlob::calculateBlockSize : buffer is too large to upload to a block blob %s", name)
		err = errors.New("buffer is too large to upload to a block blob")
		return 0, err
	}

	// If bufferSize <= BlockBlobMaxUploadBlobBytes, then Upload should be used with just 1 I/O request
	if fileSize <= azblob.BlockBlobMaxUploadBlobBytes {
		// Files up to 256MB can be uploaded as a single block
		blockSize = azblob.BlockBlobMaxUploadBlobBytes
	} else {
		// buffer / max blocks = block size to use all 50,000 blocks
		blockSize = int64(math.Ceil(float64(fileSize) / azblob.BlockBlobMaxBlocks))

		if blockSize < azblob.BlobDefaultDownloadBlockSize {
			// Block size is smaller then 16MB then consider 16MB as default
			blockSize = azblob.BlobDefaultDownloadBlockSize
		} else {
			if (blockSize & (-8)) != 0 {
				// EXTRA : round off the block size to next higher multiple of 8.
				// No reason to do so just the odd numbers in block size will not be good on server end is assumption
				blockSize = (blockSize + 7) & (-8)
			}

			if blockSize > azblob.BlockBlobMaxStageBlockBytes {
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
func (bb *BlockBlob) WriteFromFile(name string, metadata map[string]string, fi *os.File) (err error) {
	log.Trace("BlockBlob::WriteFromFile : name %s", name)
	//defer exectime.StatTimeCurrentBlock("WriteFromFile::WriteFromFile")()

	blobURL := bb.Container.NewBlockBlobURL(filepath.Join(bb.Config.prefixPath, name))
	defer log.TimeTrack(time.Now(), "BlockBlob::WriteFromFile", name)

	var uploadPtr *int64 = new(int64)
	*uploadPtr = 1

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
	if bb.Config.updateMD5 && stat.Size() >= azblob.BlockBlobMaxUploadBlobBytes {
		md5sum, err = getMD5(fi)
		if err != nil {
			// Md5 sum generation failed so set nil while uploading
			log.Warn("BlockBlob::WriteFromFile : Failed to generate md5 of %s", name)
			md5sum = []byte{0}
		}
	}

	uploadOptions := azblob.UploadToBlockBlobOptions{
		BlockSize:      blockSize,
		Parallelism:    bb.Config.maxConcurrency,
		Metadata:       metadata,
		BlobAccessTier: bb.Config.defaultTier,
		BlobHTTPHeaders: azblob.BlobHTTPHeaders{
			ContentType: getContentType(name),
			ContentMD5:  md5sum,
		},
		ClientProvidedKeyOptions: bb.blobCPKOpt,
	}
	if common.MonitorBfs() && stat.Size() > 0 {
		uploadOptions.Progress = func(bytesTransferred int64) {
			trackUpload(name, bytesTransferred, stat.Size(), uploadPtr)
		}
	}

	_, err = azblob.UploadFileToBlockBlob(context.Background(), fi, blobURL, uploadOptions)

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
		ClientProvidedKeyOptions: bb.blobCPKOpt,
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
	blobURL := bb.Container.NewBlockBlobURL(filepath.Join(bb.Config.prefixPath, name))

	storageBlockList, err := blobURL.GetBlockList(
		context.Background(), azblob.BlockListCommitted, bb.blobAccCond.LeaseAccessConditions)

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
			Id:         block.Name,
			StartIndex: int64(blockOffset),
			EndIndex:   int64(blockOffset) + block.Size,
		}
		blockOffset += block.Size
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
		if math.Ceil((float64)(numOfBlocks)+(float64)(length)/(float64)(blockSize)) > azblob.BlockBlobMaxBlocks {
			blockSize = int64(math.Ceil((float64)(length) / (float64)(azblob.BlockBlobMaxBlocks-numOfBlocks)))
			if blockSize > azblob.BlockBlobMaxStageBlockBytes {
				return 0, errors.New("cannot accommodate data within the block limit")
			}
		}
	} else if math.Ceil((float64)(numOfBlocks)+(float64)(length)/(float64)(blockSize)) > azblob.BlockBlobMaxBlocks {
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
			blobURL := bb.Container.NewBlockBlobURL(blobName)

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

					_, err = blobURL.StageBlock(context.Background(),
						id,
						bytes.NewReader(data),
						bb.blobAccCond.LeaseAccessConditions,
						nil,
						bb.downloadOptions.ClientProvidedKeyOptions)
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
	if size < azblob.BlockBlobMaxUploadBlobBytes {
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
	blobURL := bb.Container.NewBlockBlobURL(filepath.Join(bb.Config.prefixPath, name))
	blockOffset := int64(0)
	var blockIDList []string
	for _, blk := range offsetList.BlockList {
		blockIDList = append(blockIDList, blk.Id)
		if blk.Dirty() {
			_, err := blobURL.StageBlock(context.Background(),
				blk.Id,
				bytes.NewReader(data[blockOffset:(blk.EndIndex-blk.StartIndex)+blockOffset]),
				bb.blobAccCond.LeaseAccessConditions,
				nil,
				bb.downloadOptions.ClientProvidedKeyOptions)
			if err != nil {
				log.Err("BlockBlob::stageAndCommitModifiedBlocks : Failed to stage to blob %s at block %v [%s]", name, blockOffset, err.Error())
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
		bb.downloadOptions.ClientProvidedKeyOptions,
		azblob.ImmutabilityPolicyOptions{})
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
	blobURL := bb.Container.NewBlockBlobURL(filepath.Join(bb.Config.prefixPath, name))
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
			_, err := blobURL.StageBlock(context.Background(),
				blk.Id,
				bytes.NewReader(data),
				bb.blobAccCond.LeaseAccessConditions,
				nil,
				bb.downloadOptions.ClientProvidedKeyOptions)
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
		_, err := blobURL.CommitBlockList(context.Background(),
			blockIDList,
			azblob.BlobHTTPHeaders{ContentType: getContentType(name)},
			nil,
			bb.blobAccCond,
			// azblob.BlobAccessConditions{ModifiedAccessConditions: azblob.ModifiedAccessConditions{IfMatch: bol.Etag}},
			bb.Config.defaultTier,
			nil, // datalake doesn't support tags here
			bb.downloadOptions.ClientProvidedKeyOptions,
			azblob.ImmutabilityPolicyOptions{})
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
	blobURL := bb.Container.NewBlockBlobURL(filepath.Join(bb.Config.prefixPath, name))

	storageBlockList, err := blobURL.GetBlockList(
		context.Background(), azblob.BlockListCommitted, bb.blobAccCond.LeaseAccessConditions)

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
			Id:     block.Name,
			Offset: startOffset,
			Size:   uint64(block.Size),
		}
		startOffset += block.Size
		blockList = append(blockList, blk)
	}

	return &blockList, nil
}

// StageBlock : stages a block and returns its blockid
func (bb *BlockBlob) StageBlock(name string, data []byte, id string) error {
	log.Trace("BlockBlob::StageBlock : name %s", name)

	ctx, cancel := context.WithTimeout(context.Background(), max_context_timeout*time.Minute)
	defer cancel()

	blobURL := bb.Container.NewBlockBlobURL(filepath.Join(bb.Config.prefixPath, name))
	_, err := blobURL.StageBlock(ctx,
		id,
		bytes.NewReader(data),
		bb.blobAccCond.LeaseAccessConditions,
		nil,
		bb.downloadOptions.ClientProvidedKeyOptions)

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

	blobURL := bb.Container.NewBlockBlobURL(filepath.Join(bb.Config.prefixPath, name))
	_, err := blobURL.CommitBlockList(ctx,
		blockList,
		azblob.BlobHTTPHeaders{ContentType: getContentType(name)},
		nil,
		bb.blobAccCond,
		// azblob.BlobAccessConditions{ModifiedAccessConditions: azblob.ModifiedAccessConditions{IfMatch: bol.Etag}},
		bb.Config.defaultTier,
		nil, // datalake doesn't support tags here
		bb.downloadOptions.ClientProvidedKeyOptions,
		azblob.ImmutabilityPolicyOptions{})

	if err != nil {
		log.Err("BlockBlob::CommitBlocks : Failed to commit block list to blob %s [%s]", name, err.Error())
		return err
	}

	return nil
}
