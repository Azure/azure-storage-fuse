// Copyright © 2020 Microsoft <wastore@microsoft.com>
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package main

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/Azure/azure-pipeline-go/pipeline"
	"github.com/Azure/azure-storage-azcopy/v10/cmd"
	"github.com/Azure/azure-storage-azcopy/v10/common"
	"github.com/Azure/azure-storage-azcopy/v10/ste"
	"github.com/Azure/azure-storage-blob-go/azblob"
)

//HTTPClientFactory returns http sender with given client
func HTTPClientFactory(client *http.Client) pipeline.FactoryFunc {
	return pipeline.FactoryFunc(func(next pipeline.Policy, po *pipeline.PolicyOptions) pipeline.PolicyFunc {
		return func(ctx context.Context, request pipeline.Request) (pipeline.Response, error) {
			r, err := client.Do(request.WithContext(ctx))
			if err != nil {
				err = pipeline.NewError(err, "HTTP request failed")
			}
			return pipeline.NewHTTPResponse(r), err
		}
	})
}

func GetBlockSize(filesize int64, minBlockSize int64) (blockSize int64) {
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

var jobMgr ste.IJobMgr
var globalPartNum uint32
var partNumLock sync.Mutex

func JobMgr() ste.IJobMgr {
	return jobMgr
}

func SetJobMgr(jm ste.IJobMgr) {
	jobMgr = jm
}

func NextPartNum() uint32 {
	partNumLock.Lock()
	defer partNumLock.Unlock()
	if globalPartNum == math.MaxUint32 {
		jobMgr.Reset(context.Background(), "Lustre")
		globalPartNum = 0
	}
	ret := globalPartNum
	globalPartNum = globalPartNum + 1
	return ret
}

func RestPartNum() {
	globalPartNum = 0
}

func Upload(filePath string, blobPath string, blockSize int64, meta azblob.Metadata) error {
	srcResource, _ := cmd.SplitResourceString(filePath, common.ELocation.Local())
	dstResource, _ := cmd.SplitResourceString(blobPath, common.ELocation.Blob())
	p := common.PartNumber(NextPartNum())

	fi, _ := os.Stat(filePath)

	t := common.CopyTransfer{
		Source:           "",
		Destination:      "",
		EntityType:       common.EEntityType.File(),
		LastModifiedTime: fi.ModTime(),
		SourceSize:       fi.Size(),
		Metadata:         common.FromAzBlobMetadataToCommonMetadata(meta),
	}

	var metadata = ""
	for k, v := range meta {
		metadata = metadata + fmt.Sprintf("%s=%s;", k, v)
	}
	if len(metadata) > 0 { //Remove trailing ';'
		metadata = metadata[:len(metadata)-1]
	}

	order := common.CopyJobPartOrderRequest{
		JobID:           JobMgr().JobID(),
		PartNum:         p,
		FromTo:          common.EFromTo.LocalBlob(),
		ForceWrite:      common.EOverwriteOption.True(),
		ForceIfReadOnly: false,
		AutoDecompress:  false,
		Priority:        common.EJobPriority.Normal(),
		LogLevel:        common.ELogLevel.Debug(),
		BlobAttributes: common.BlobTransferAttributes{
			BlobType:         common.EBlobType.BlockBlob(),
			BlockSizeInBytes: GetBlockSize(fi.Size(), blockSize),
			Metadata:         metadata,
		},
		CommandString:   "NONE",
		DestinationRoot: dstResource,
		SourceRoot:      srcResource,
		Fpo:             common.EFolderPropertiesOption.NoFolders(),
	}
	order.Transfers.List = append(order.Transfers.List, t)

	jppfn := ste.JobPartPlanFileName(fmt.Sprintf(ste.JobPartPlanFileNameFormat, jobMgr.JobID().String(), p, ste.DataSchemaVersion))
	jppfn.Create(order)

	jobMgr.AddJobPart(order.PartNum, jppfn, nil, order.SourceRoot.SAS, order.DestinationRoot.SAS, true)

	// Update jobPart Status with the status Manager
	jobMgr.SendJobPartCreatedMsg(ste.JobPartCreatedMsg{TotalTransfers: uint32(len(order.Transfers.List)),
		IsFinalPart:          true,
		TotalBytesEnumerated: order.Transfers.TotalSizeInBytes,
		FileTransfers:        order.Transfers.FileTransferCount,
		FolderTransfer:       order.Transfers.FolderTransferCount})

	jobDone := false
	var status common.JobStatus
	for !jobDone {
		part, _ := jobMgr.JobPartMgr(p)
		status = part.Plan().JobPartStatus()
		jobDone = status.IsJobDone()
		time.Sleep(time.Second * 1)
	}

	if err := os.Remove(jppfn.GetJobPartPlanPath()); err != nil {
		fmt.Println(err.Error())
	}
	if status != common.EJobStatus.Completed() {
		return errors.New("STE Failed")
	}

	return nil
}

func Download(blobPath string, filePath string, blockSize int64) error {
	dstResource, _ := cmd.SplitResourceString(filePath, common.ELocation.Local())
	srcResource, _ := cmd.SplitResourceString(blobPath, common.ELocation.Blob())
	p := common.PartNumber(NextPartNum())

	getBlobProperties := func(blobPath string) (*azblob.BlobGetPropertiesResponse, error) {
		rawURL, _ := url.Parse(blobPath)
		blobUrlParts := azblob.NewBlobURLParts(*rawURL)
		blobUrlParts.BlobName = strings.TrimSuffix(blobUrlParts.BlobName, "/")

		// perform the check
		blobURL := azblob.NewBlobURL(blobUrlParts.URL(), azblob.NewPipeline(azblob.NewAnonymousCredential(), azblob.PipelineOptions{}))
		return blobURL.GetProperties(context.TODO(), azblob.BlobAccessConditions{}, azblob.ClientProvidedKeyOptions{})
	}

	props, err := getBlobProperties(blobPath)
	if err != nil {
		return err
	}
	t := common.CopyTransfer{
		Source:             "",
		Destination:        "",
		EntityType:         common.EEntityType.File(),
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

	order := common.CopyJobPartOrderRequest{
		JobID:           JobMgr().JobID(),
		PartNum:         p,
		FromTo:          common.EFromTo.BlobLocal(),
		ForceWrite:      common.EOverwriteOption.True(),
		ForceIfReadOnly: false,
		AutoDecompress:  false,
		Priority:        common.EJobPriority.Normal(),
		LogLevel:        common.ELogLevel.Debug(),
		BlobAttributes: common.BlobTransferAttributes{
			BlobType:         common.EBlobType.BlockBlob(),
			BlockSizeInBytes: GetBlockSize(props.ContentLength(), blockSize),
		},
		CommandString:   "NONE",
		DestinationRoot: dstResource,
		SourceRoot:      srcResource,
		Fpo:             common.EFolderPropertiesOption.NoFolders(),
	}
	order.Transfers.List = append(order.Transfers.List, t)

	jppfn := ste.JobPartPlanFileName(fmt.Sprintf(ste.JobPartPlanFileNameFormat, jobMgr.JobID().String(), p, ste.DataSchemaVersion))
	jppfn.Create(order)

	jobMgr.AddJobPart(order.PartNum, jppfn, nil, order.SourceRoot.SAS, order.DestinationRoot.SAS, true)

	// Update jobPart Status with the status Manager
	jobMgr.SendJobPartCreatedMsg(ste.JobPartCreatedMsg{TotalTransfers: uint32(len(order.Transfers.List)),
		IsFinalPart:          true,
		TotalBytesEnumerated: order.Transfers.TotalSizeInBytes,
		FileTransfers:        order.Transfers.FileTransferCount,
		FolderTransfer:       order.Transfers.FolderTransferCount})

	jobDone := false
	var status common.JobStatus
	for !jobDone {
		part, _ := jobMgr.JobPartMgr(p)
		status = part.Plan().JobPartStatus()
		jobDone = status.IsJobDone()
		time.Sleep(time.Second * 1)
	}

	if err := os.Remove(jppfn.GetJobPartPlanPath()); err != nil {
		fmt.Println(err.Error())
	}

	if status != common.EJobStatus.Completed() {
		return errors.New("STE Failed")
	}

	return nil
}
