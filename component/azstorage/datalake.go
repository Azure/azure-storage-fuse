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
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal"
	"github.com/vibhansa-msft/blobfilter"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azdatalake/directory"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azdatalake/file"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azdatalake/filesystem"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azdatalake/service"
)

//go:generate $ASSERT_REMOVER $GOFILE

type Datalake struct {
	AzStorageConnection
	Auth           azAuth
	Service        *service.Client
	Filesystem     *filesystem.Client
	BlockBlob      BlockBlob
	datalakeCPKOpt *file.CPKInfo
}

// Verify that Datalake implements AzConnection interface
var _ AzConnection = &Datalake{}

// transformAccountEndpoint
// Users must set an endpoint to allow blobfuse to
// 1. support Azure clouds (ex: Public, Zonal DNS, China, Germany, Gov, etc)
// 2. direct REST APIs to a truly custom endpoint (ex: www dot custom-domain dot com)
// We can handle case 1 by simply replacing the .dfs. to .blob. and blobfuse will work fine.
// However, case 2 will not work since the endpoint likely only redirects to the dfs endpoint and not the blob endpoint, so we don't know what endpoint to use when we call blob endpoints.
// This is also a known problem with the SDKs.
func transformAccountEndpoint(potentialDfsEndpoint string) string {
	if strings.Contains(potentialDfsEndpoint, ".dfs.") {
		return strings.Replace(potentialDfsEndpoint, ".dfs.", ".blob.", -1)
	} else {
		// Should we just throw here?
		log.Warn("Datalake::transformAccountEndpoint : Detected use of a custom endpoint. Not all operations are guaranteed to work.")
	}
	return potentialDfsEndpoint
}

// transformConfig transforms the adls config to a blob config
func transformConfig(dlConfig AzStorageConfig) AzStorageConfig {
	bbConfig := dlConfig
	bbConfig.authConfig.AccountType = EAccountType.BLOCK()
	bbConfig.authConfig.Endpoint = transformAccountEndpoint(dlConfig.authConfig.Endpoint)
	return bbConfig
}

func (dl *Datalake) Configure(cfg AzStorageConfig) error {
	dl.Config = cfg

	if dl.Config.cpkEnabled {
		dl.datalakeCPKOpt = &file.CPKInfo{
			EncryptionKey:       &dl.Config.cpkEncryptionKey,
			EncryptionKeySHA256: &dl.Config.cpkEncryptionKeySha256,
			EncryptionAlgorithm: to.Ptr(directory.EncryptionAlgorithmTypeAES256),
		}
	}

	err := dl.BlockBlob.Configure(transformConfig(cfg))

	// List call shall always retrieved permissions for HNS accounts
	dl.BlockBlob.listDetails.Permissions = true

	return err
}

// For dynamic config update the config here
func (dl *Datalake) UpdateConfig(cfg AzStorageConfig) error {
	dl.Config.blockSize = cfg.blockSize
	dl.Config.maxConcurrency = cfg.maxConcurrency
	dl.Config.defaultTier = cfg.defaultTier
	dl.Config.ignoreAccessModifiers = cfg.ignoreAccessModifiers
	return dl.BlockBlob.UpdateConfig(cfg)
}

// UpdateServiceClient : Update the SAS specified by the user and create new service client
func (dl *Datalake) UpdateServiceClient(key, value string) (err error) {
	if key == "saskey" {
		dl.Auth.setOption(key, value)
		// get the service client with updated SAS
		svcClient, err := dl.Auth.getServiceClient(&dl.Config)
		if err != nil {
			log.Err("Datalake::UpdateServiceClient : Failed to get service client [%s]", err.Error())
			return err
		}

		// update the service client
		dl.Service = svcClient.(*service.Client)

		// Update the filesystem client
		dl.Filesystem = dl.Service.NewFileSystemClient(dl.Config.container)
	}
	return dl.BlockBlob.UpdateServiceClient(key, value)
}

// createServiceClient : Create the service client
func (dl *Datalake) createServiceClient() (*service.Client, error) {
	log.Trace("Datalake::createServiceClient : Getting service client")

	dl.Auth = getAzAuth(dl.Config.authConfig)
	if dl.Auth == nil {
		log.Err("Datalake::createServiceClient : Failed to retrieve auth object")
		return nil, fmt.Errorf("failed to retrieve auth object")
	}

	svcClient, err := dl.Auth.getServiceClient(&dl.Config)
	if err != nil {
		log.Err("Datalake::createServiceClient : Failed to get service client [%s]", err.Error())
		return nil, err
	}

	return svcClient.(*service.Client), nil
}

// SetupPipeline : Based on the config setup the ***URLs
func (dl *Datalake) SetupPipeline() error {
	log.Trace("Datalake::SetupPipeline : Setting up")
	var err error

	// create the service client
	dl.Service, err = dl.createServiceClient()
	if err != nil {
		log.Err("Datalake::SetupPipeline : Failed to get service client [%s]", err.Error())
		return err
	}

	// create the filesystem client
	dl.Filesystem = dl.Service.NewFileSystemClient(dl.Config.container)

	return dl.BlockBlob.SetupPipeline()
}

// TestPipeline : Validate the credentials specified in the auth config
func (dl *Datalake) TestPipeline() error {
	log.Trace("Datalake::TestPipeline : Validating")

	if dl.Config.mountAllContainers {
		return nil
	}

	if dl.Filesystem == nil || dl.Filesystem.DFSURL() == "" || dl.Filesystem.BlobURL() == "" {
		log.Err("Datalake::TestPipeline : Filesystem Client is not built, check your credentials")
		return nil
	}

	maxResults := int32(2)
	listPathPager := dl.Filesystem.NewListPathsPager(false, &filesystem.ListPathsOptions{
		MaxResults: &maxResults,
		Prefix:     &dl.Config.prefixPath,
	})

	// we are just validating the auth mode used. So, no need to iterate over the pages
	resp, err := listPathPager.NextPage(context.Background())
	if err != nil {
		log.Err("Datalake::TestPipeline : Failed to validate account with given auth %s", err.Error())
		var respErr *azcore.ResponseError
		errors.As(err, &respErr)
		if respErr != nil {
			return fmt.Errorf("Datalake::TestPipeline : [%s]", respErr.ErrorCode)
		}
		return err
	} else {
		// If the account is not HNS, then the permissions will be nil
		// For empty containers there will be another check done by block_blob TestPipeline
		// so no need to error out if there are no paths
		if len(resp.Paths) > 0 {
			if resp.Paths[0].Permissions == nil {
				// This is to block FNS account being mounted as HNS account
				return fmt.Errorf("Datalake::TestPipeline : Account is not HNS, kindly set correct account type")
			}
		}
	}

	return dl.BlockBlob.TestPipeline()
}

// IsAccountADLS : Check account is ADLS or not
func (dl *Datalake) IsAccountADLS() bool {
	return dl.BlockBlob.IsAccountADLS()
}

func (dl *Datalake) ListContainers() ([]string, error) {
	log.Trace("Datalake::ListContainers : Listing containers")
	return dl.BlockBlob.ListContainers()
}

func (dl *Datalake) SetPrefixPath(path string) error {
	log.Trace("Datalake::SetPrefixPath : path %s", path)
	dl.Config.prefixPath = path
	return dl.BlockBlob.SetPrefixPath(path)
}

// CreateFile : Create a new file in the filesystem/directory
func (dl *Datalake) CreateFile(name string, mode os.FileMode) error {
	log.Trace("Datalake::CreateFile : name %s", name)
	err := dl.BlockBlob.CreateFile(name, mode)
	if err != nil {
		log.Err("Datalake::CreateFile : Failed to create file %s [%s]", name, err.Error())
		return err
	}
	err = dl.ChangeMod(name, mode)
	if err != nil {
		log.Err("Datalake::CreateFile : Failed to set permissions on file %s [%s]", name, err.Error())
		return err
	}

	return nil
}

// CreateDirectory : Create a new directory in the filesystem/directory
func (dl *Datalake) CreateDirectory(name string, _ bool) error {
	log.Trace("Datalake::CreateDirectory : name %s", name)

	directoryURL := dl.Filesystem.NewDirectoryClient(filepath.Join(dl.Config.prefixPath, name))
	_, err := directoryURL.Create(context.Background(), &directory.CreateOptions{
		CPKInfo: dl.datalakeCPKOpt,
		AccessConditions: &directory.AccessConditions{
			ModifiedAccessConditions: &directory.ModifiedAccessConditions{
				IfNoneMatch: to.Ptr(azcore.ETagAny),
			},
		},
	})

	if err != nil {
		serr := storeDatalakeErrToErr(err)
		if serr == InvalidPermission {
			log.Err("Datalake::CreateDirectory : Insufficient permissions for %s [%s]", name, err.Error())
			return syscall.EACCES
		} else if serr == ErrFileAlreadyExists {
			log.Err("Datalake::CreateDirectory : Path already exists for %s [%s]", name, err.Error())
			return syscall.EEXIST
		} else {
			log.Err("Datalake::CreateDirectory : Failed to create directory %s [%s]", name, err.Error())
			return err
		}
	}

	return nil
}

// CreateLink : Create a symlink in the filesystem/directory
func (dl *Datalake) CreateLink(source string, target string) error {
	log.Trace("Datalake::CreateLink : %s -> %s", source, target)
	return dl.BlockBlob.CreateLink(source, target)
}

// DeleteFile : Delete a file in the filesystem/directory
func (dl *Datalake) DeleteFile(name string) (err error) {
	log.Trace("Datalake::DeleteFile : name %s", name)
	fileClient := dl.Filesystem.NewFileClient(filepath.Join(dl.Config.prefixPath, name))
	_, err = fileClient.Delete(context.Background(), nil)
	if err != nil {
		serr := storeDatalakeErrToErr(err)
		if serr == ErrFileNotFound {
			log.Err("Datalake::DeleteFile : %s does not exist", name)
			return syscall.ENOENT
		} else if serr == BlobIsUnderLease {
			log.Err("Datalake::DeleteFile : %s is under lease [%s]", name, err.Error())
			return syscall.EIO
		} else if serr == InvalidPermission {
			log.Err("Datalake::DeleteFile : Insufficient permissions for %s [%s]", name, err.Error())
			return syscall.EACCES
		} else {
			log.Err("Datalake::DeleteFile : Failed to delete file %s [%s]", name, err.Error())
			return err
		}
	}

	return nil
}

// DeleteDirectory : Delete a directory in the filesystem/directory
func (dl *Datalake) DeleteDirectory(name string) (err error) {
	log.Trace("Datalake::DeleteDirectory : name %s", name)

	directoryClient := dl.Filesystem.NewDirectoryClient(filepath.Join(dl.Config.prefixPath, name))
	_, err = directoryClient.Delete(context.Background(), nil)
	// TODO : There is an ability to pass a continuation token here for recursive delete, should we implement this logic to follow continuation token? The SDK does not currently do this.
	if err != nil {
		serr := storeDatalakeErrToErr(err)
		if serr == ErrFileNotFound {
			log.Err("Datalake::DeleteDirectory : %s does not exist", name)
			return syscall.ENOENT
		} else {
			log.Err("Datalake::DeleteDirectory : Failed to delete directory %s [%s]", name, err.Error())
			return err
		}
	}

	return nil
}

// RenameFile : Rename the file
// While renaming the file, Creation time is preserved but LMT is changed for the destination blob.
// and also Etag of the destination blob changes
func (dl *Datalake) RenameFile(options internal.RenameFileOptions) error {
	log.Trace("Datalake::RenameFile : %s -> %s, NoReplace: %v", options.Src, options.Dst, options.NoReplace)

	fileClient := dl.Filesystem.NewFileClient(url.PathEscape(filepath.Join(dl.Config.prefixPath, options.Src)))

	renameOptions := &file.RenameOptions{
		CPKInfo: dl.datalakeCPKOpt,
	}

	if options.NoReplace {
		renameOptions.AccessConditions = &file.AccessConditions{
			ModifiedAccessConditions: &file.ModifiedAccessConditions{
				IfNoneMatch: to.Ptr(azcore.ETagAny),
			},
		}
	}

	renameResponse, err := fileClient.Rename(context.Background(), filepath.Join(dl.Config.prefixPath, options.Dst), renameOptions)
	if err != nil {
		serr := storeDatalakeErrToErr(err)
		if serr == ErrFileNotFound {
			log.Err("Datalake::RenameFile : %s does not exist", options.Src)
			return syscall.ENOENT
		} else if serr == ErrFileAlreadyExists {
			common.Assert(options.NoReplace, options)
			log.Err("BlockBlob::RenameFile : Dst Blob Exists %s [%s]", options.Dst, err.Error())
			return syscall.EEXIST
		} else {
			log.Err("Datalake::RenameFile : Failed to rename file %s to %s [%s]", options.Src, options.Dst, err.Error())
			return err
		}
	}
	modifyLMTandEtag(options.SrcAttr, renameResponse.LastModified, sanitizeEtag(renameResponse.ETag))
	return nil
}

// RenameDirectory : Rename the directory
func (dl *Datalake) RenameDirectory(source string, target string) error {
	log.Trace("Datalake::RenameDirectory : %s -> %s", source, target)

	directoryClient := dl.Filesystem.NewDirectoryClient(url.PathEscape(filepath.Join(dl.Config.prefixPath, source)))
	_, err := directoryClient.Rename(context.Background(), filepath.Join(dl.Config.prefixPath, target), &directory.RenameOptions{
		CPKInfo: dl.datalakeCPKOpt,
	})
	if err != nil {
		serr := storeDatalakeErrToErr(err)
		if serr == ErrFileNotFound {
			log.Err("Datalake::RenameDirectory : %s does not exist", source)
			return syscall.ENOENT
		} else {
			log.Err("Datalake::RenameDirectory : Failed to rename directory %s to %s [%s]", source, target, err.Error())
			return err
		}
	}

	return nil
}

// GetAttr : Retrieve attributes of the path
func (dl *Datalake) GetAttr(name string) (blobAttr *internal.ObjAttr, err error) {
	log.Trace("Datalake::GetAttr : name %s", name)

	fileClient := dl.Filesystem.NewFileClient(filepath.Join(dl.Config.prefixPath, name))
	prop, err := fileClient.GetProperties(context.Background(), &file.GetPropertiesOptions{
		CPKInfo: dl.datalakeCPKOpt,
	})
	if err != nil {
		e := storeDatalakeErrToErr(err)
		if e == ErrFileNotFound {
			return blobAttr, syscall.ENOENT
		} else if e == InvalidPermission {
			log.Err("Datalake::GetAttr : Insufficient permissions for %s [%s]", name, err.Error())
			return blobAttr, syscall.EACCES
		} else {
			log.Err("Datalake::GetAttr : Failed to get path properties for %s [%s]", name, err.Error())
			return blobAttr, err
		}
	}

	mode, err := getFileMode(*prop.Permissions)
	if err != nil {
		log.Err("Datalake::GetAttr : Failed to get file mode for %s [%s]", name, err.Error())
		return blobAttr, err
	}

	blobAttr = &internal.ObjAttr{
		Path:   name,
		Name:   filepath.Base(name),
		Size:   *prop.ContentLength,
		Mode:   mode,
		Mtime:  *prop.LastModified,
		Atime:  *prop.LastModified,
		Ctime:  *prop.LastModified,
		Crtime: *prop.LastModified,
		Flags:  internal.NewFileBitMap(),
		ETag:   sanitizeEtag(prop.ETag),
	}
	parseMetadata(blobAttr, prop.Metadata)

	if *prop.ResourceType == "directory" {
		blobAttr.Flags = internal.NewDirBitMap()
		blobAttr.Mode = blobAttr.Mode | os.ModeDir
	}

	if dl.Config.honourACL && dl.Config.authConfig.ObjectID != "" {
		acl, err := fileClient.GetAccessControl(context.Background(), nil)
		if err != nil {
			// Just ignore the error here as rest of the attributes have been retrieved
			log.Err("Datalake::GetAttr : Failed to get ACL for %s [%s]", name, err.Error())
		} else {
			mode, err := getFileModeFromACL(dl.Config.authConfig.ObjectID, *acl.ACL, *acl.Owner)
			if err != nil {
				log.Err("Datalake::GetAttr : Failed to get file mode from ACL for %s [%s]", name, err.Error())
			} else {
				blobAttr.Mode = mode
			}
		}
	}

	if dl.Config.filter != nil {
		if !dl.Config.filter.IsAcceptable(&blobfilter.BlobAttr{
			Name:  blobAttr.Name,
			Mtime: blobAttr.Mtime,
			Size:  blobAttr.Size,
		}) {
			log.Debug("Datalake::GetAttr : Filtered out %s", name)
			return nil, syscall.ENOENT
		}
	}

	return blobAttr, nil
}

// List : Get a list of path matching the given prefix
// This fetches the list using a marker so the caller code should handle marker logic
// If count=0 - fetch max entries
func (dl *Datalake) List(prefix string, marker *string, count int32) ([]*internal.ObjAttr, *string, error) {
	return dl.BlockBlob.List(prefix, marker, count)
}

// ReadToFile : Download a file to a local file
func (dl *Datalake) ReadToFile(name string, offset int64, count int64, fi *os.File) (err error) {
	return dl.BlockBlob.ReadToFile(name, offset, count, fi)
}

// ReadBuffer : Download a specific range from a file to a buffer
func (dl *Datalake) ReadBuffer(name string, offset int64, len int64) ([]byte, error) {
	return dl.BlockBlob.ReadBuffer(name, offset, len)
}

// ReadInBuffer : Download specific range from a file to a user provided buffer
func (dl *Datalake) ReadInBuffer(name string, offset int64, len int64, data []byte, etag *string) error {
	return dl.BlockBlob.ReadInBuffer(name, offset, len, data, etag)
}

// WriteFromFile : Upload local file to file
func (dl *Datalake) WriteFromFile(name string, metadata map[string]*string, fi *os.File) (err error) {
	// File in DataLake may have permissions and ACL set. Just uploading the file will override them.
	// So, we need to get the existing permissions and ACL and set them back after uploading the file.

	var acl string = ""
	var fileClient *file.Client = nil

	if dl.Config.preserveACL {
		fileClient = dl.Filesystem.NewFileClient(filepath.Join(dl.Config.prefixPath, name))
		resp, err := fileClient.GetAccessControl(context.Background(), nil)
		if err != nil {
			log.Err("Datalake::getACL : Failed to get ACLs for file %s [%s]", name, err.Error())
		} else if resp.ACL != nil {
			acl = *resp.ACL
		}
	}

	// Upload the file, which will override the permissions and ACL
	retCode := dl.BlockBlob.WriteFromFile(name, metadata, fi)

	if acl != "" {
		// Cannot set both permissions and ACL in one call. ACL includes permission as well so just setting those back
		// Just setting up the permissions will delete existing ACLs applied on the blob so do not convert this code to
		// just set the permissions.
		_, err := fileClient.SetAccessControl(context.Background(), &file.SetAccessControlOptions{
			ACL: &acl,
		})

		if err != nil {
			// Earlier code was ignoring this so it might break customer cases where they do not have auth to update ACL
			log.Err("Datalake::WriteFromFile : Failed to set ACL for %s [%s]", name, err.Error())
		}
	}

	return retCode
}

// WriteFromBuffer : Upload from a buffer to a file
func (dl *Datalake) WriteFromBuffer(options internal.WriteFromBufferOptions) (string, error) {
	return dl.BlockBlob.WriteFromBuffer(options)
}

// Write : Write to a file at given offset
func (dl *Datalake) Write(options *internal.WriteFileOptions) error {
	return dl.BlockBlob.Write(options)
}

func (dl *Datalake) StageAndCommit(name string, bol *common.BlockOffsetList) error {
	return dl.BlockBlob.StageAndCommit(name, bol)
}

func (dl *Datalake) GetFileBlockOffsets(name string) (*common.BlockOffsetList, error) {
	return dl.BlockBlob.GetFileBlockOffsets(name)
}

func (dl *Datalake) TruncateFile(name string, size int64) error {
	return dl.BlockBlob.TruncateFile(name, size)
}

// ChangeMod : Change mode of a path
func (dl *Datalake) ChangeMod(name string, mode os.FileMode) error {
	log.Trace("Datalake::ChangeMod : Change mode of file %s to %s", name, mode)
	fileClient := dl.Filesystem.NewFileClient(filepath.Join(dl.Config.prefixPath, name))

	/*
		// If we need to call the ACL set api then we need to get older acl string here
		// and create new string with the username included in the string
		// Keeping this code here so in future if its required we can get the string and manipulate

		currPerm, err := fileURL.getACL(context.Background())
		e := storeDatalakeErrToErr(err)
		if e == ErrFileNotFound {
			return syscall.ENOENT
		} else if err != nil {
			log.Err("Datalake::ChangeMod : Failed to get mode of file %s [%s]", name, err.Error())
			return err
		}
	*/

	newPerm := getACLPermissions(mode)
	_, err := fileClient.SetAccessControl(context.Background(), &file.SetAccessControlOptions{
		Permissions: &newPerm,
	})
	if err != nil {
		log.Err("Datalake::ChangeMod : Failed to change mode of file %s to %s [%s]", name, mode, err.Error())
		e := storeDatalakeErrToErr(err)
		if e == ErrFileNotFound {
			return syscall.ENOENT
		} else if e == InvalidPermission {
			return syscall.EACCES
		} else {
			return err
		}
	}

	return nil
}

// ChangeOwner : Change owner of a path
func (dl *Datalake) ChangeOwner(name string, _ int, _ int) error {
	log.Trace("Datalake::ChangeOwner : name %s", name)

	if dl.Config.ignoreAccessModifiers {
		// for operations like git clone where transaction fails if chown is not successful
		// return success instead of ENOSYS
		return nil
	}

	// TODO: This is not supported for now.
	// fileURL := dl.Filesystem.NewRootDirectoryURL().NewFileURL(filepath.Join(dl.Config.prefixPath, name))
	// group := strconv.Itoa(gid)
	// owner := strconv.Itoa(uid)
	// _, err := fileURL.SetAccessControl(context.Background(), azbfs.BlobFSAccessControl{Group: group, Owner: owner})
	// e := storeDatalakeErrToErr(err)
	// if e == ErrFileNotFound {
	// 	return syscall.ENOENT
	// } else if err != nil {
	// 	log.Err("Datalake::ChangeOwner : Failed to change ownership of file %s to %s [%s]", name, mode, err.Error())
	// 	return err
	// }
	return syscall.ENOTSUP
}

// GetCommittedBlockList : Get the list of committed blocks
func (dl *Datalake) GetCommittedBlockList(name string) (*internal.CommittedBlockList, error) {
	return dl.BlockBlob.GetCommittedBlockList(name)
}

// StageBlock : stages a block and returns its blockid
func (dl *Datalake) StageBlock(name string, data []byte, id string) error {
	return dl.BlockBlob.StageBlock(name, data, id)
}

// CommitBlocks : persists the block list
func (dl *Datalake) CommitBlocks(name string, blockList []string, newEtag *string) error {
	return dl.BlockBlob.CommitBlocks(name, blockList, newEtag)
}

func (dl *Datalake) SetFilter(filter string) error {
	if filter == "" {
		dl.Config.filter = nil
	} else {
		dl.Config.filter = &blobfilter.BlobFilter{}
		err := dl.Config.filter.Configure(filter)
		if err != nil {
			return err
		}
	}

	return dl.BlockBlob.SetFilter(filter)
}

// SetMetadata : Set metadata property of path
func (dl *Datalake) SetMetadata(filePath string, newMetadata map[string]*string, etag *azcore.ETag, overwrite bool) (err error) {
	return dl.BlockBlob.SetMetadata(filePath, newMetadata, etag, overwrite)
}
