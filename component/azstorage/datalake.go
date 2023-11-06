/*
    _____           _____   _____   ____          ______  _____  ------
   |     |  |      |     | |     | |     |     | |       |            |
   |     |  |      |     | |     | |     |     | |       |            |
   | --- |  |      |     | |-----| |---- |     | |-----| |-----  ------
   |     |  |      |     | |     | |     |     |       | |       |
   | ____|  |_____ | ____| | ____| |     |_____|  _____| |_____  |_____


   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2020-2023 Microsoft Corporation. All rights reserved.
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
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/Azure/azure-pipeline-go/pipeline"
	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal"

	"github.com/Azure/azure-storage-azcopy/v10/azbfs"
	"github.com/Azure/azure-storage-azcopy/v10/ste"
)

type Datalake struct {
	AzStorageConnection
	Auth       azAuth
	Service    azbfs.ServiceURL
	Filesystem azbfs.FileSystemURL
	BlockBlob  BlockBlob
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
	return dl.BlockBlob.Configure(transformConfig(cfg))
}

// For dynamic config update the config here
func (dl *Datalake) UpdateConfig(cfg AzStorageConfig) error {
	dl.Config.blockSize = cfg.blockSize
	dl.Config.maxConcurrency = cfg.maxConcurrency
	dl.Config.defaultTier = cfg.defaultTier
	dl.Config.ignoreAccessModifiers = cfg.ignoreAccessModifiers
	return dl.BlockBlob.UpdateConfig(cfg)
}

// NewSASKey : New SAS key provided by user
func (dl *Datalake) NewCredentialKey(key, value string) (err error) {
	if key == "saskey" {
		dl.Auth.setOption(key, value)
		// Update the endpoint url from the credential
		dl.Endpoint, err = url.Parse(dl.Auth.getEndpoint())
		if err != nil {
			log.Err("Datalake::NewCredentialKey : Failed to form base endpoint url [%s]", err.Error())
			return errors.New("failed to form base endpoint url")
		}

		// Update the service url
		dl.Service = azbfs.NewServiceURL(*dl.Endpoint, dl.Pipeline)

		// Update the filesystem url
		dl.Filesystem = dl.Service.NewFileSystemURL(dl.Config.container)
	}
	return dl.BlockBlob.NewCredentialKey(key, value)
}

// getCredential : Create the credential object
func (dl *Datalake) getCredential() azbfs.Credential {
	log.Trace("Datalake::getCredential : Getting credential")

	dl.Auth = getAzAuth(dl.Config.authConfig)
	if dl.Auth == nil {
		log.Err("Datalake::getCredential : Failed to retrieve auth object")
		return nil
	}

	cred := dl.Auth.getCredential()
	if cred == nil {
		log.Err("Datalake::getCredential : Failed to get credential")
		return nil
	}

	return cred.(azbfs.Credential)
}

// NewPipeline creates a Pipeline using the specified credentials and options.
func NewBfsPipeline(c azbfs.Credential, o azbfs.PipelineOptions, ro ste.XferRetryOptions) pipeline.Pipeline {
	// Closest to API goes first; closest to the wire goes last
	f := []pipeline.Factory{
		azbfs.NewTelemetryPolicyFactory(o.Telemetry),
		azbfs.NewUniqueRequestIDPolicyFactory(),
		// ste.NewBlobXferRetryPolicyFactory(ro),
		ste.NewBFSXferRetryPolicyFactory(ro),
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
func (dl *Datalake) SetupPipeline() error {
	log.Trace("Datalake::SetupPipeline : Setting up")
	var err error

	// Get the credential
	cred := dl.getCredential()
	if cred == nil {
		log.Err("Datalake::SetupPipeline : Failed to get credential")
		return errors.New("failed to get credential")
	}

	// Create a new pipeline
	options, retryOptions := getAzBfsPipelineOptions(dl.Config)
	dl.Pipeline = NewBfsPipeline(cred, options, retryOptions)
	if dl.Pipeline == nil {
		log.Err("Datalake::SetupPipeline : Failed to create pipeline object")
		return errors.New("failed to create pipeline object")
	}

	// Get the endpoint url from the credential
	dl.Endpoint, err = url.Parse(dl.Auth.getEndpoint())
	if err != nil {
		log.Err("Datalake::SetupPipeline : Failed to form base end point url [%s]", err.Error())
		return errors.New("failed to form base end point url")
	}

	// Create the service url
	dl.Service = azbfs.NewServiceURL(*dl.Endpoint, dl.Pipeline)

	// Create the filesystem url
	dl.Filesystem = dl.Service.NewFileSystemURL(dl.Config.container)

	return dl.BlockBlob.SetupPipeline()
}

// TestPipeline : Validate the credentials specified in the auth config
func (dl *Datalake) TestPipeline() error {
	log.Trace("Datalake::TestPipeline : Validating")

	if dl.Config.mountAllContainers {
		return nil
	}

	if dl.Filesystem.String() == "" {
		log.Err("Datalake::TestPipeline : Filesystem URL is not built, check your credentials")
		return nil
	}

	maxResults := int32(2)
	listPath, err := dl.Filesystem.ListPaths(context.Background(),
		azbfs.ListPathsFilesystemOptions{
			Path:       &dl.Config.prefixPath,
			Recursive:  false,
			MaxResults: &maxResults,
		})

	if err != nil {
		log.Err("Datalake::TestPipeline : Failed to validate account with given auth %s", err.Error)
		return err
	}

	if listPath == nil {
		log.Info("Datalake::TestPipeline : Filesystem is empty")
	}
	return dl.BlockBlob.TestPipeline()
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
func (dl *Datalake) CreateDirectory(name string) error {
	log.Trace("Datalake::CreateDirectory : name %s", name)

	directoryURL := dl.Filesystem.NewDirectoryURL(filepath.Join(dl.Config.prefixPath, name))
	_, err := directoryURL.Create(context.Background(), false)

	if err != nil {
		serr := storeDatalakeErrToErr(err)
		if serr == InvalidPermission {
			log.Err("Datalake::CreateDirectory : Insufficient permissions for %s [%s]", name, err.Error())
			return syscall.EACCES
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

	fileURL := dl.Filesystem.NewRootDirectoryURL().NewFileURL(filepath.Join(dl.Config.prefixPath, name))
	_, err = fileURL.Delete(context.Background())
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

	directoryURL := dl.Filesystem.NewDirectoryURL(filepath.Join(dl.Config.prefixPath, name))
	_, err = directoryURL.Delete(context.Background(), nil, true)
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
func (dl *Datalake) RenameFile(source string, target string) error {
	log.Trace("Datalake::RenameFile : %s -> %s", source, target)

	fileURL := dl.Filesystem.NewRootDirectoryURL().NewFileURL(url.PathEscape(filepath.Join(dl.Config.prefixPath, source)))

	_, err := fileURL.Rename(context.Background(),
		azbfs.RenameFileOptions{
			DestinationPath: filepath.Join(dl.Config.prefixPath, target),
		})
	if err != nil {
		serr := storeDatalakeErrToErr(err)
		if serr == ErrFileNotFound {
			log.Err("Datalake::RenameFile : %s does not exist", source)
			return syscall.ENOENT
		} else {
			log.Err("Datalake::RenameFile : Failed to rename file %s to %s [%s]", source, target, err.Error())
			return err
		}
	}

	return nil
}

// RenameDirectory : Rename the directory
func (dl *Datalake) RenameDirectory(source string, target string) error {
	log.Trace("Datalake::RenameDirectory : %s -> %s", source, target)

	directoryURL := dl.Filesystem.NewDirectoryURL(url.PathEscape(filepath.Join(dl.Config.prefixPath, source)))

	_, err := directoryURL.Rename(context.Background(),
		azbfs.RenameDirectoryOptions{
			DestinationPath: filepath.Join(dl.Config.prefixPath, target),
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
func (dl *Datalake) GetAttr(name string) (attr *internal.ObjAttr, err error) {
	log.Trace("Datalake::GetAttr : name %s", name)

	pathURL := dl.Filesystem.NewRootDirectoryURL().NewFileURL(filepath.Join(dl.Config.prefixPath, name))
	prop, err := pathURL.GetProperties(context.Background())
	if err != nil {
		e := storeDatalakeErrToErr(err)
		if e == ErrFileNotFound {
			return attr, syscall.ENOENT
		} else if e == InvalidPermission {
			log.Err("Datalake::GetAttr : Insufficient permissions for %s [%s]", name, err.Error())
			return attr, syscall.EACCES
		} else {
			log.Err("Datalake::GetAttr : Failed to get path properties for %s [%s]", name, err.Error())
			return attr, err
		}
	}

	lastModified, err := time.Parse(time.RFC1123, prop.LastModified())

	if err != nil {
		log.Err("Datalake::GetAttr : Failed to convert last modified time for %s [%s]", name, err.Error())
		return attr, err
	}

	mode, err := getFileMode(prop.XMsPermissions())
	if err != nil {
		log.Err("Datalake::GetAttr : Failed to get file mode for %s [%s]", name, err.Error())
		return attr, err
	}

	attr = &internal.ObjAttr{
		Path:   name,
		Name:   filepath.Base(name),
		Size:   prop.ContentLength(),
		Mode:   mode,
		Mtime:  lastModified,
		Atime:  lastModified,
		Ctime:  lastModified,
		Crtime: lastModified,
		Flags:  internal.NewFileBitMap(),
	}
	parseProperties(attr, prop.XMsProperties())
	if azbfs.PathResourceDirectory == azbfs.PathResourceType(prop.XMsResourceType()) {
		attr.Flags = internal.NewDirBitMap()
		attr.Mode = attr.Mode | os.ModeDir
	}
	attr.Flags.Set(internal.PropFlagMetadataRetrieved)

	if dl.Config.honourACL && dl.Config.authConfig.ObjectID != "" {
		acl, err := pathURL.GetAccessControl(context.Background())
		if err != nil {
			// Just ignore the error here as rest of the attributes have been retrieved
			log.Err("Datalake::GetAttr : Failed to get ACL for %s [%s]", name, err.Error())
		} else {
			mode, err := getFileModeFromACL(dl.Config.authConfig.ObjectID, acl.ACL, acl.Owner)
			if err != nil {
				log.Err("Datalake::GetAttr : Failed to get file mode from ACL for %s [%s]", name, err.Error())
			} else {
				attr.Mode = mode
			}
		}
	}

	return attr, nil
}

// List : Get a list of path matching the given prefix
// This fetches the list using a marker so the caller code should handle marker logic
// If count=0 - fetch max entries
func (dl *Datalake) List(prefix string, marker *string, count int32) ([]*internal.ObjAttr, *string, error) {
	log.Trace("Datalake::List : prefix %s, marker %s", prefix, func(marker *string) string {
		if marker != nil {
			return *marker
		} else {
			return ""
		}
	}(marker))

	pathList := make([]*internal.ObjAttr, 0)

	if count == 0 {
		count = common.MaxDirListCount
	}

	prefixPath := filepath.Join(dl.Config.prefixPath, prefix)
	if prefix != "" && prefix[len(prefix)-1] == '/' {
		prefixPath += "/"
	}

	// Get a result segment starting with the path indicated by the current Marker.
	listPath, err := dl.Filesystem.ListPaths(context.Background(),
		azbfs.ListPathsFilesystemOptions{
			Path:              &prefixPath,
			Recursive:         false,
			MaxResults:        &count,
			ContinuationToken: marker,
		})

	if err != nil {
		log.Err("Datalake::List : Failed to validate account with given auth %s", err.Error())
		m := ""
		e := storeDatalakeErrToErr(err)
		if e == ErrFileNotFound { // TODO: should this be checked for list calls
			return pathList, &m, syscall.ENOENT
		} else if e == InvalidPermission {
			return pathList, &m, syscall.EACCES
		} else {
			return pathList, &m, err
		}
	}

	// Process the paths returned in this result segment (if the segment is empty, the loop body won't execute)
	for _, pathInfo := range listPath.Paths {
		var attr *internal.ObjAttr

		if dl.Config.disableSymlink {
			var mode fs.FileMode
			if pathInfo.Permissions != nil {
				mode, err = getFileMode(*pathInfo.Permissions)
				if err != nil {
					log.Err("Datalake::List : Failed to get file mode for %s [%s]", *pathInfo.Name, err.Error())
					m := ""
					return pathList, &m, err
				}
			} else {
				// This happens when a blob account is mounted with type:adls
				log.Err("Datalake::List : Failed to get file permissions for %s", *pathInfo.Name)
			}

			var contentLength int64 = 0
			if pathInfo.ContentLength != nil {
				contentLength = *pathInfo.ContentLength
			} else {
				// This happens when a blob account is mounted with type:adls
				log.Err("Datalake::List : Failed to get file length for %s", *pathInfo.Name)
			}

			attr = &internal.ObjAttr{
				Path:   *pathInfo.Name,
				Name:   filepath.Base(*pathInfo.Name),
				Size:   contentLength,
				Mode:   mode,
				Mtime:  pathInfo.LastModifiedTime(),
				Atime:  pathInfo.LastModifiedTime(),
				Ctime:  pathInfo.LastModifiedTime(),
				Crtime: pathInfo.LastModifiedTime(),
				Flags:  internal.NewFileBitMap(),
			}
			if pathInfo.IsDirectory != nil && *pathInfo.IsDirectory {
				attr.Flags = internal.NewDirBitMap()
				attr.Mode = attr.Mode | os.ModeDir
			}
		} else {
			attr, err = dl.GetAttr(*pathInfo.Name)
			if err != nil {
				log.Err("Datalake::List : Failed to get properties for %s [%s]", *pathInfo.Name, err.Error())
				m := ""
				return pathList, &m, err
			}
		}

		// Note: Datalake list paths does not return metadata/properties.
		// To account for this and accurately return attributes when needed,
		// we have a flag for whether or not metadata has been retrieved.
		// If this flag is not set the attribute cache will call get attributes
		// to fetch metadata properties.
		// Any method that populates the metadata should set the attribute flag.
		// Alternatively, if you want Datalake list paths to return metadata/properties as well.
		// pass CLI parameter --no-symlinks=false in the mount command.
		pathList = append(pathList, attr)
	}

	m := listPath.XMsContinuation()
	return pathList, &m, nil
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
func (dl *Datalake) ReadInBuffer(name string, offset int64, len int64, data []byte) error {
	return dl.BlockBlob.ReadInBuffer(name, offset, len, data)
}

// WriteFromFile : Upload local file to file
func (dl *Datalake) WriteFromFile(name string, metadata map[string]string, fi *os.File) (err error) {
	return dl.BlockBlob.WriteFromFile(name, metadata, fi)
}

// WriteFromBuffer : Upload from a buffer to a file
func (dl *Datalake) WriteFromBuffer(name string, metadata map[string]string, data []byte) error {
	return dl.BlockBlob.WriteFromBuffer(name, metadata, data)
}

// Write : Write to a file at given offset
func (dl *Datalake) Write(options internal.WriteFileOptions) error {
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
	fileURL := dl.Filesystem.NewRootDirectoryURL().NewFileURL(filepath.Join(dl.Config.prefixPath, name))

	/*
		// If we need to call the ACL set api then we need to get older acl string here
		// and create new string with the username included in the string
		// Keeping this code here so in future if its required we can get the string and manipulate

		currPerm, err := fileURL.GetAccessControl(context.Background())
		e := storeDatalakeErrToErr(err)
		if e == ErrFileNotFound {
			return syscall.ENOENT
		} else if err != nil {
			log.Err("Datalake::ChangeMod : Failed to get mode of file %s [%s]", name, err.Error())
			return err
		}
	*/

	newPerm := getACLPermissions(mode)
	_, err := fileURL.SetAccessControl(context.Background(), azbfs.BlobFSAccessControl{Permissions: newPerm})
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
