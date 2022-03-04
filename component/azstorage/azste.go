/*
    _____           _____   _____   ____          ______  _____  ------
   |     |  |      |     | |     | |     |     | |       |            |
   |     |  |      |     | |     | |     |     | |       |            |
   | --- |  |      |     | |-----| |---- |     | |-----| |-----  ------
   |     |  |      |     | |     | |     |     |       | |       |
   | ____|  |_____ | ____| | ____| |     |_____|  _____| |_____  |_____


   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2020-2021 Microsoft Corporation. All rights reserved.
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
	"blobfuse2/common/log"
	"context"
	"errors"
	"fmt"
	"math"
	"net/url"
	"os"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/Azure/azure-storage-azcopy/v10/cmd"
	stecommon "github.com/Azure/azure-storage-azcopy/v10/common"
	"github.com/Azure/azure-storage-azcopy/v10/ste"
	"github.com/Azure/azure-storage-blob-go/azblob"
)

type AzSTEConfig struct {
	Enable         bool
	MinFileSize    int64
	SlicePool      int64
	CacheLimit     int64
	FileCountLimit int64
	GCPercent      int
	partFilePath   string
}

type AzSTE struct {
	jobMgr      ste.IJobMgr
	partNum     uint32
	partNumLock sync.Mutex
}

// InitSTE : this initialize the STE lib and bring it to ready state for transfers
func (azste *AzSTE) Initialize(config AzSTEConfig) (err error) {
	jobID := stecommon.NewJobID()
	tuner := ste.NullConcurrencyTuner{FixedValue: 128}

	var pacer ste.PacerAdmin = ste.NewNullAutoPacer()
	var logLevel stecommon.LogLevel

	stecommon.AzcopyJobPlanFolder = config.partFilePath
	logLevel = stecommon.ELogLevel.Info()
	logger := stecommon.NewSysLogger(jobID, logLevel, "narasimha")
	logger.OpenLog()

	os.MkdirAll(stecommon.AzcopyJobPlanFolder, 0666)
	azste.jobMgr = ste.NewJobMgr(ste.NewConcurrencySettings(math.MaxInt32, false),
		jobID,
		context.Background(),
		stecommon.NewNullCpuMonitor(),
		stecommon.ELogLevel.Error(),
		"blobfuse2",
		config.partFilePath,
		&tuner,
		pacer,
		stecommon.NewMultiSizeSlicePool(config.SlicePool),
		stecommon.NewCacheLimiter(config.CacheLimit),
		stecommon.NewCacheLimiter(config.FileCountLimit),
		logger)

	stecommon.GetLifecycleMgr().E2EEnableAwaitAllowOpenFiles(false)

	go func() {
		time.Sleep(20 * time.Second)         // wait a little, so that our initial pool of buffers can get allocated without heaps of (unnecessary) GC activity
		debug.SetGCPercent(config.GCPercent) // activate more aggressive/frequent GC than the default
	}()

	go func() {
		for {
			s, _ := azste.jobMgr.GetPerfInfo()
			str := "[States: " + strings.Join(s, ", ") + "], "
			log.Err(str)
			time.Sleep(30 * time.Second)

		}
	}()

	return nil
}

// getBlockSize : Based on the available memory define the block size.
func (azste *AzSTE) getBlockSize(filesize int64, minBlockSize int64) (blockSize int64) {
	blockSizeThreshold := int64(256 * 1024 * 1024) /* 256 MB */
	blockSize = minBlockSize

	/* We should not perform checks on filesize, block size limitation here. Those are performed in SDK
	 * and take care of themselves when limits change
	 */

	for ; uint32(filesize/blockSize) > azblob.BlockBlobMaxBlocks; blockSize = 2 * blockSize {
		if blockSize > blockSizeThreshold {
			/*
			 * For a RAM usage of 0.5G/core, we would have 4G memory on typical 8 core device, meaning at a blockSize of 256M,
			 * we can have 4 blocks in core, waiting for a disk or n/w operation. Any higher block size would *sort of*
			 * serialize n/w and disk operations, and is better avoided.
			 */
			blockSize = filesize / azblob.BlockBlobMaxBlocks
			break
		}
	}

	return blockSize
}

// nextPartNum : Method to create next part number for transfer. On hitting the int limit reset the jobMgr object
func (azste *AzSTE) nextPartNum() uint32 {
	azste.partNumLock.Lock()
	defer azste.partNumLock.Unlock()

	if azste.partNum == math.MaxUint32 {
		azste.jobMgr.Reset(context.Background(), "blobfuse2")
		azste.partNum = 0
	}

	ret := azste.partNum
	azste.partNum = azste.partNum + 1
	return ret
}

type UploadParam struct {
	filePath  string
	blobPath  string
	blockSize int64
	meta      azblob.Metadata
	tier      stecommon.BlockBlobTier
}

// Upload : Function to upload a file to container using STE lib
func (azste *AzSTE) Upload(param UploadParam) error {
	srcResource, _ := cmd.SplitResourceString(param.filePath, stecommon.ELocation.Local())
	dstResource, _ := cmd.SplitResourceString(param.blobPath, stecommon.ELocation.Blob())
	p := stecommon.PartNumber(azste.nextPartNum())

	fi, _ := os.Stat(param.filePath)

	t := stecommon.CopyTransfer{
		Source:           "",
		Destination:      "",
		EntityType:       stecommon.EEntityType.File(),
		LastModifiedTime: fi.ModTime(),
		SourceSize:       fi.Size(),
		Metadata:         stecommon.FromAzBlobMetadataToCommonMetadata(param.meta),
	}

	var metadata = ""
	for k, v := range param.meta {
		metadata = metadata + fmt.Sprintf("%s=%s;", k, v)
	}
	if len(metadata) > 0 { //Remove trailing ';'
		metadata = metadata[:len(metadata)-1]
	}

	order := stecommon.CopyJobPartOrderRequest{
		JobID:           azste.jobMgr.JobID(),
		PartNum:         p,
		FromTo:          stecommon.EFromTo.LocalBlob(),
		ForceWrite:      stecommon.EOverwriteOption.True(),
		ForceIfReadOnly: false,
		AutoDecompress:  false,
		Priority:        stecommon.EJobPriority.Normal(),
		LogLevel:        stecommon.ELogLevel.Debug(),
		BlobAttributes: stecommon.BlobTransferAttributes{
			BlobType:         stecommon.EBlobType.BlockBlob(),
			BlockSizeInBytes: azste.getBlockSize(fi.Size(), param.blockSize),
			Metadata:         metadata,
			ContentType:      getContentType(param.filePath),
			BlockBlobTier:    param.tier,
		},
		CommandString:   "NONE",
		DestinationRoot: dstResource,
		SourceRoot:      srcResource,
		Fpo:             stecommon.EFolderPropertiesOption.NoFolders(),
	}
	order.Transfers.List = append(order.Transfers.List, t)

	jppfn := ste.JobPartPlanFileName(fmt.Sprintf(ste.JobPartPlanFileNameFormat, azste.jobMgr.JobID().String(), p, ste.DataSchemaVersion))
	jppfn.Create(order)

	azste.jobMgr.AddJobPart(order.PartNum, jppfn, nil, order.SourceRoot.SAS, order.DestinationRoot.SAS, true)

	// Update jobPart Status with the status Manager
	azste.jobMgr.SendJobPartCreatedMsg(ste.JobPartCreatedMsg{TotalTransfers: uint32(len(order.Transfers.List)),
		IsFinalPart:          true,
		TotalBytesEnumerated: order.Transfers.TotalSizeInBytes,
		FileTransfers:        order.Transfers.FileTransferCount,
		FolderTransfer:       order.Transfers.FolderTransferCount})

	jobDone := false
	var status stecommon.JobStatus

	for !jobDone {
		part, _ := azste.jobMgr.JobPartMgr(p)
		status = part.Plan().JobPartStatus()
		jobDone = status.IsJobDone()
		time.Sleep(time.Second * 1)
	}

	if err := os.Remove(jppfn.GetJobPartPlanPath()); err != nil {
		log.Info("AzSTE::Upload : Failed to remove job plan (%s)", err.Error())
	}

	if status != stecommon.EJobStatus.Completed() {
		log.Err("AzSTE::Upload : Failed to upload file %s", param.filePath)
		return errors.New("STE failed to upload file")
	}

	return nil
}

type DownloadParam struct {
	blobPath  string
	filePath  string
	blockSize int64
}

// Download : Function to download blob to a file using STE lib
func (azste *AzSTE) Download(options DownloadParam) error {
	dstResource, _ := cmd.SplitResourceString(options.filePath, stecommon.ELocation.Local())
	srcResource, _ := cmd.SplitResourceString(options.blobPath, stecommon.ELocation.Blob())
	p := stecommon.PartNumber(azste.nextPartNum())

	getBlobProperties := func(blobPath string) (*azblob.BlobGetPropertiesResponse, error) {
		rawURL, _ := url.Parse(blobPath)
		blobUrlParts := azblob.NewBlobURLParts(*rawURL)
		blobUrlParts.BlobName = strings.TrimSuffix(blobUrlParts.BlobName, "/")

		// perform the check
		blobURL := azblob.NewBlobURL(blobUrlParts.URL(), azblob.NewPipeline(azblob.NewAnonymousCredential(), azblob.PipelineOptions{}))
		return blobURL.GetProperties(context.TODO(), azblob.BlobAccessConditions{}, azblob.ClientProvidedKeyOptions{})
	}

	props, err := getBlobProperties(options.blobPath)
	if err != nil {
		return err
	}
	t := stecommon.CopyTransfer{
		Source:             "",
		Destination:        "",
		EntityType:         stecommon.EEntityType.File(),
		LastModifiedTime:   props.LastModified(),
		SourceSize:         props.ContentLength(),
		ContentType:        props.ContentType(),
		ContentEncoding:    props.ContentEncoding(),
		ContentDisposition: props.ContentDisposition(),
		ContentLanguage:    props.ContentLanguage(),
		CacheControl:       props.CacheControl(),
		ContentMD5:         props.ContentMD5(),
		Metadata:           nil,
		BlobType:           props.BlobType(),
		BlobTags:           nil,
	}

	order := stecommon.CopyJobPartOrderRequest{
		JobID:           azste.jobMgr.JobID(),
		PartNum:         p,
		FromTo:          stecommon.EFromTo.BlobLocal(),
		ForceWrite:      stecommon.EOverwriteOption.True(),
		ForceIfReadOnly: false,
		AutoDecompress:  false,
		Priority:        stecommon.EJobPriority.Normal(),
		LogLevel:        stecommon.ELogLevel.Debug(),
		BlobAttributes: stecommon.BlobTransferAttributes{
			BlobType:         stecommon.EBlobType.BlockBlob(),
			BlockSizeInBytes: azste.getBlockSize(props.ContentLength(), options.blockSize),
		},
		CommandString:   "NONE",
		DestinationRoot: dstResource,
		SourceRoot:      srcResource,
		Fpo:             stecommon.EFolderPropertiesOption.NoFolders(),
	}

	order.Transfers.List = append(order.Transfers.List, t)

	jppfn := ste.JobPartPlanFileName(fmt.Sprintf(ste.JobPartPlanFileNameFormat, azste.jobMgr.JobID().String(), p, ste.DataSchemaVersion))
	jppfn.Create(order)

	azste.jobMgr.AddJobPart(order.PartNum, jppfn, nil, order.SourceRoot.SAS, order.DestinationRoot.SAS, true)

	// Update jobPart Status with the status Manager
	azste.jobMgr.SendJobPartCreatedMsg(ste.JobPartCreatedMsg{TotalTransfers: uint32(len(order.Transfers.List)),
		IsFinalPart:          true,
		TotalBytesEnumerated: order.Transfers.TotalSizeInBytes,
		FileTransfers:        order.Transfers.FileTransferCount,
		FolderTransfer:       order.Transfers.FolderTransferCount})

	jobDone := false
	var status stecommon.JobStatus
	for !jobDone {
		part, _ := azste.jobMgr.JobPartMgr(p)
		status = part.Plan().JobPartStatus()
		jobDone = status.IsJobDone()
		time.Sleep(time.Second * 1)
	}

	if err := os.Remove(jppfn.GetJobPartPlanPath()); err != nil {
		log.Info("AzSTE::Download : Failed to remove job plan (%s)", err.Error())
	}

	if status != stecommon.EJobStatus.Completed() {
		log.Err("AzSTE::Download : Failed to upload file %s", options.filePath)
		return errors.New("STE failed to download file")
	}

	return nil
}
