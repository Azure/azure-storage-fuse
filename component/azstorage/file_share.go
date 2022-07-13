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
	"path/filepath"
	"strings"
	"syscall"
	"time"

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
	log.Trace("FileShare::ListContainers : Listing containers")
	cntList := make([]string, 0)

	marker := azfile.Marker{}
	for marker.NotDone() {
		resp, err := fs.Service.ListSharesSegment(context.Background(), marker, azfile.ListSharesOptions{})
		if err != nil {
			log.Err("FileShare::ListContainers : Failed to get container list")
			return cntList, err
		}

		for _, v := range resp.ShareItems {
			cntList = append(cntList, v.Name)
		}

		marker = resp.NextMarker
	}

	return cntList, nil
}

// This is just for test, shall not be used otherwise
func (fs *FileShare) SetPrefixPath(path string) error {
	log.Trace("FileShare::SetPrefixPath : path %s", path)
	fs.Config.prefixPath = path
	return nil
}

func (fs *FileShare) Exists(name string) bool {
	log.Trace("FileShare::Exists : name %s", name)
	if _, err := fs.GetAttr(name); err == syscall.ENOENT {
		return false
	}
	return true
}

// CreateFile : Create a new file in the share/virtual directory
func (fs *FileShare) CreateFile(name string, mode os.FileMode) error {
	log.Trace("FileShare::CreateFile : name %s", name)
	var data []byte
	return fs.WriteFromBuffer(filepath.Base(name), nil, data)
}

func (fs *FileShare) CreateDirectory(name string) error {
	return syscall.ENOTSUP
}

// CreateLink : Create a symlink in the share/virtual directory
func (fs *FileShare) CreateLink(source string, target string) error {
	log.Trace("FileShare::CreateLink : %s -> %s", source, target)
	data := []byte(target)
	metadata := make(azfile.Metadata)
	metadata[symlinkKey] = "true"
	return fs.WriteFromBuffer(source, metadata, data)
}

// DeleteFile : Delete a file in the share/virtual directory
func (fs *FileShare) DeleteFile(name string) (err error) {
	log.Trace("FileShare::DeleteFile : name %s", name)

	fileName, dirPath := getFileAndDirFromPath(filepath.Join(fs.Config.prefixPath, name))

	fileURL := fs.Share.NewDirectoryURL(dirPath).NewFileURL(fileName)
	_, err = fileURL.Delete(context.Background())
	if err != nil {
		serr := storeFileErrToErr(err)
		if serr == ErrFileNotFound {
			log.Err("FileShare::DeleteFile : %s does not exist", name)
			return syscall.ENOENT
		} else {
			log.Err("FileShare::DeleteFile : Failed to delete blob %s (%s)", name, err.Error())
			return err
		}
	}

	return nil
}

func (fs *FileShare) DeleteDirectory(name string) error {
	return syscall.ENOTSUP
}

// RenameFile : Rename the file
func (fs *FileShare) RenameFile(source string, target string) error {
	log.Trace("FileShare::RenameFile : %s -> %s", source, target)

	fileName, dirPath := getFileAndDirFromPath(filepath.Join(fs.Config.prefixPath, source))

	fileURL := fs.Share.NewDirectoryURL(dirPath).NewFileURL(fileName)
	newFile := fs.Share.NewDirectoryURL(dirPath).NewFileURL(target)

	prop, err := fileURL.GetProperties(context.Background())
	if err != nil {
		serr := storeFileErrToErr(err)
		if serr == ErrFileNotFound {
			log.Err("FileShare::RenameFile : %s does not exist", source)
			return syscall.ENOENT
		} else {
			log.Err("FileShare::RenameFile : Failed to get file properties for %s (%s)", source, err.Error())
			return err
		}
	}

	startCopy, err := newFile.StartCopy(context.Background(), fileURL.URL(), prop.NewMetadata())

	if err != nil {
		log.Err("FileShare::RenameFile : Failed to start copy of file %s (%s)", source, err.Error())
		return err
	}

	copyStatus := startCopy.CopyStatus()
	for copyStatus == azfile.CopyStatusPending {
		time.Sleep(time.Second * 1)
		prop, err = newFile.GetProperties(context.Background())
		if err != nil {
			log.Err("FileShare::RenameFile : CopyStats : Failed to get blob properties for %s (%s)", source, err.Error())
		}
		copyStatus = prop.CopyStatus()
	}
	log.Trace("FileShare::RenameFile : %s -> %s done", source, target)

	// Copy of the file is done so now delete the older file
	return fs.DeleteFile(source)
}

func (fs *FileShare) RenameDirectory(string, string) error {
	return syscall.ENOTSUP
}

// GetAttr : Retrieve attributes of a file or directory
func (fs *FileShare) GetAttr(name string) (attr *internal.ObjAttr, err error) {
	log.Trace("FileShare::GetAttr : name %s", name)

	fileName, dirPath := getFileAndDirFromPath(filepath.Join(fs.Config.prefixPath, name))

	fileURL := fs.Share.NewDirectoryURL(dirPath).NewFileURL(fileName)
	prop, fileerr := fileURL.GetProperties(context.Background())

	if fileerr == nil { // file
		ctime, err := time.Parse(time.RFC1123, prop.FileChangeTime())
		if err != nil {
			ctime = prop.LastModified()
		}
		crtime, err := time.Parse(time.RFC1123, prop.FileCreationTime())
		if err != nil {
			crtime = prop.LastModified()
		}
		attr = &internal.ObjAttr{
			Path:   name, // We don't need to strip the prefixPath here since we pass the input name
			Name:   filepath.Base(name),
			Size:   prop.ContentLength(),
			Mode:   0,
			Mtime:  prop.LastModified(),
			Atime:  prop.LastModified(),
			Ctime:  ctime,
			Crtime: crtime,
			Flags:  internal.NewFileBitMap(),
		}
		parseMetadata(attr, prop.NewMetadata())
		attr.Flags.Set(internal.PropFlagMetadataRetrieved)
		attr.Flags.Set(internal.PropFlagModeDefault)

		return attr, nil
	} else if storeFileErrToErr(fileerr) == ErrFileNotFound { // directory
		dirURL := fs.Share.NewDirectoryURL(filepath.Join(fs.Config.prefixPath, name))
		prop, direrr := dirURL.GetProperties(context.Background())

		if direrr == nil {
			ctime, err := time.Parse(time.RFC1123, prop.FileChangeTime())
			if err != nil {
				ctime = prop.LastModified()
			}
			crtime, err := time.Parse(time.RFC1123, prop.FileCreationTime())
			if err != nil {
				crtime = prop.LastModified()
			}
			attr = &internal.ObjAttr{
				Path:   name,
				Name:   filepath.Base(name),
				Size:   4096,
				Mode:   0,
				Mtime:  prop.LastModified(),
				Atime:  prop.LastModified(),
				Ctime:  ctime,
				Crtime: crtime,
				Flags:  internal.NewDirBitMap(),
			}
			parseMetadata(attr, prop.NewMetadata())
			attr.Flags.Set(internal.PropFlagMetadataRetrieved)
			attr.Flags.Set(internal.PropFlagModeDefault)

			return attr, nil
		}
		return attr, syscall.ENOENT
	}
	// error
	log.Err("FileShare::GetAttr : Failed to get file/directory properties for %s (%s)", name, err.Error())
	return attr, fileerr
}

// List : Get a list of files/directories matching the given prefix
// This fetches the list using a marker so the caller code should handle marker logic
// If count=0 - fetch max entries
func (fs *FileShare) List(prefix string, marker *string, count int32) ([]*internal.ObjAttr, *string, error) {
	log.Trace("FileShare::List : prefix %s, marker %s", prefix, func(marker *string) string {
		if marker != nil {
			return *marker
		} else {
			return ""
		}
	}(marker))

	fileList := make([]*internal.ObjAttr, 0)

	if count == 0 {
		count = common.MaxDirListCount
	}

	listPath := filepath.Join(fs.Config.prefixPath, prefix)

	listFile, err := fs.Share.NewDirectoryURL(listPath).ListFilesAndDirectoriesSegment(context.Background(), azfile.Marker{Val: marker},
		azfile.ListFilesAndDirectoriesOptions{MaxResults: count})

	if err != nil {
		log.Err("File::List : Failed to list the container with the prefix %s", err.Error)
		return fileList, nil, err
	}

	// Process the files returned in this result segment (if the segment is empty, the loop body won't execute)
	for _, fileInfo := range listFile.FileItems {
		attr := &internal.ObjAttr{
			Path: split(fs.Config.prefixPath, fileInfo.Name),
			Name: filepath.Base(fileInfo.Name),
			Size: fileInfo.Properties.ContentLength,
			Mode: 0,
			// Azure file SDK supports 2019.02.02 but time and metadata are only supported by 2020.x.x onwards
			// TODO: support times when Azure SDK is updated
			Mtime:  time.Now(),
			Atime:  time.Now(),
			Ctime:  time.Now(),
			Crtime: time.Now(),
			Flags:  internal.NewFileBitMap(),
		}

		attr.Flags.Set(internal.PropFlagModeDefault)
		fileList = append(fileList, attr)

		if attr.IsDir() {
			attr.Size = 4096
		}
	}

	for _, dirInfo := range listFile.DirectoryItems {
		attr := &internal.ObjAttr{
			Path: split(fs.Config.prefixPath, dirInfo.Name),
			Name: filepath.Base(dirInfo.Name),
			Size: 4096,
			Mode: os.ModeDir,
			// Azure file SDK supports 2019.02.02 but time, metadata, and dir size are only supported by 2020.x.x onwards
			// TODO: support times when Azure SDK is updated
			Mtime:  time.Now(),
			Atime:  time.Now(),
			Ctime:  time.Now(),
			Crtime: time.Now(),
			Flags:  internal.NewDirBitMap(),
		}

		attr.Flags.Set(internal.PropFlagModeDefault)
		fileList = append(fileList, attr)
	}

	return fileList, listFile.NextMarker.Val, nil
}

func (fs *FileShare) ReadToFile(name string, offset int64, count int64, fi *os.File) error {
	return syscall.ENOTSUP
}
func (fs *FileShare) ReadBuffer(name string, offset int64, len int64) ([]byte, error) {
	return nil, syscall.ENOTSUP
}
func (fs *FileShare) ReadInBuffer(name string, offset int64, len int64, data []byte) error {
	return syscall.ENOTSUP
}

func (fs *FileShare) WriteFromFile(name string, metadata map[string]string, fi *os.File) error {
	return syscall.ENOTSUP
}

// WriteFromBuffer : Upload from a buffer to a file
func (fs *FileShare) WriteFromBuffer(name string, metadata map[string]string, data []byte) error {
	return syscall.ENOTSUP
}

func (fs *FileShare) Write(options internal.WriteFileOptions) error {
	return syscall.ENOTSUP
}

func (fs *FileShare) GetFileBlockOffsets(name string) (*common.BlockOffsetList, error) {
	return nil, syscall.ENOTSUP
}

func (fs *FileShare) ChangeMod(string, os.FileMode) error {
	return syscall.ENOTSUP
}
func (fs *FileShare) ChangeOwner(string, int, int) error {
	return syscall.ENOTSUP
}
func (fs *FileShare) TruncateFile(string, int64) error {
	return syscall.ENOTSUP
}
func (fs *FileShare) StageAndCommit(name string, bol *common.BlockOffsetList) error {
	return syscall.ENOTSUP
}

func (fs *FileShare) NewCredentialKey(_, _ string) error {
	return syscall.ENOTSUP
}

// separates directory/directories and file name of a given file/directory path
// covers case where name param includes subdirectories and not just the file name
func getFileAndDirFromPath(completePath string) (fileName string, dirPath string) {
	if completePath == "" {
		return "", ""
	}

	splitPath := strings.Split(completePath, "/")

	dirArray := splitPath[:len(splitPath)-1]
	dirPath = strings.Join(dirArray, "/") // doesn't end with "/"

	fileName = splitPath[len(splitPath)-1]

	return fileName, dirPath
}
