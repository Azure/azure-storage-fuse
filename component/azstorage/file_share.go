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
	"errors"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/Azure/azure-storage-file-go/azfile"
)

const (
	// FileMaxSizeInBytes indicates the maximum size of a file
	FileMaxSizeInBytes         = 4398046511104 // 4TiB
	tmpFileCreationSizeInBytes = 1000000000    // 1GB
)

type FileShare struct {
	AzStorageConnection
	Auth            azAuth
	Service         azfile.ServiceURL
	Share           azfile.ShareURL
	downloadOptions azfile.DownloadFromAzureFileOptions
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

// NewCredentialKey : Update the credential key specified by the user
func (fs *FileShare) NewCredentialKey(key, value string) (err error) {
	if key == "saskey" {
		fs.Auth.setOption(key, value)
		// Update the endpoint url from the credential
		fs.Endpoint, err = url.Parse(fs.Auth.getEndpoint())
		if err != nil {
			log.Err("FileShare::NewCredentialKey : Failed to form base endpoint url (%s)", err.Error())
			return errors.New("failed to form base endpoint url")
		}

		// Update the service url
		fs.Service = azfile.NewServiceURL(*fs.Endpoint, fs.Pipeline)

		// Update the container url
		fs.Share = fs.Service.NewShareURL(fs.Config.container)
	}
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
	listFile, err := fs.Share.NewRootDirectoryURL().ListFilesAndDirectoriesSegment(context.Background(), marker,
		azfile.ListFilesAndDirectoriesOptions{MaxResults: 2})

	if err != nil {
		log.Err("FileShare::TestPipeline : Failed to validate account with given auth %s", err.Error())
		return err
	}

	if listFile == nil {
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
			log.Err("FileShare::ListContainers : Failed to get container list %s", err.Error())
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

// Exists : Check whether or not a given file exists
func (fs *FileShare) Exists(name string) bool {
	log.Trace("FileShare::Exists : name %s", name)
	if _, err := fs.GetAttr(name); err == syscall.ENOENT {
		return false
	}
	return true
}

// CreateFile : Create a new file in the share/directory
func (fs *FileShare) CreateFile(name string, mode os.FileMode) error {
	log.Trace("FileShare::CreateFile : name %s", name)

	fileName, dirPath := getFileAndDirFromPath(filepath.Join(fs.Config.prefixPath, name))
	fileURL := fs.Share.NewDirectoryURL(dirPath).NewFileURL(fileName)

	_, err := fileURL.Create(context.Background(), tmpFileCreationSizeInBytes, azfile.FileHTTPHeaders{
		ContentType: getContentType(name),
	},
		nil)

	if err != nil {
		log.Err("FileShare::CreateFile : Failed to create file %s %s", name, err.Error())
		return err
	}

	return nil
}

func (fs *FileShare) CreateDirectory(name string) error {
	log.Trace("FileShare::CreateDirectory : name %s", name)

	metadata := make(azfile.Metadata)
	metadata[folderKey] = "true"

	dirURL := fs.Share.NewDirectoryURL(filepath.Join(fs.Config.prefixPath, name))

	_, err := dirURL.Create(context.Background(), metadata, azfile.SMBProperties{})

	if err != nil {
		log.Err("FileShare::CreateDirectory : Failed to create directory %s %s", name, err.Error())
		return err
	}
	return nil
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
			log.Err("FileShare::DeleteFile : %s does not exist %s", name, err.Error())
			return syscall.ENOENT
		} else {
			log.Err("FileShare::DeleteFile : Failed to delete file %s (%s)", name, err.Error())
			return err
		}
	}

	return nil
}

// DeleteDirectory : Delete a virtual directory in the container/virtual directory
func (fs *FileShare) DeleteDirectory(name string) (err error) {
	log.Trace("FileShare::DeleteDirectory : name %s", name)

	dirURL := fs.Share.NewDirectoryURL(filepath.Join(fs.Config.prefixPath, name))

	for marker := (azfile.Marker{}); marker.NotDone(); {
		listFile, err := dirURL.ListFilesAndDirectoriesSegment(context.Background(), marker,
			azfile.ListFilesAndDirectoriesOptions{
				MaxResults: common.MaxDirListCount,
			})
		if err != nil {
			log.Err("FileShare::DeleteDirectory : Failed to get list of files and directories %s", err.Error())
			return err
		}
		marker = listFile.NextMarker

		// Process the files returned in this result segment (if the segment is empty, the loop body won't execute)
		for _, fileInfo := range listFile.FileItems {
			err = fs.DeleteFile(filepath.Join(name, fileInfo.Name))
			if err != nil {
				log.Err("FileShare::DeleteDirectory : Failed to delete files  %s", err.Error())
				return err
			}
		}

		for _, dirInfo := range listFile.DirectoryItems {
			err = fs.DeleteDirectory(filepath.Join(filepath.Join(fs.Config.prefixPath, name), dirInfo.Name))
			if err != nil {
				log.Err("FileShare::DeleteDirectory : Failed delete subdirectories  %s", err.Error())
				return err
			}
		}
	}

	_, err = dirURL.Delete(context.Background())
	if err != nil {
		serr := storeFileErrToErr(err)
		if serr == ErrFileNotFound {
			log.Err("FileShare::DeleteDirectory : %s does not exist", name)
			return syscall.ENOENT
		} else {
			log.Err("FileShare::DeleteDirectory : Failed to delete directory %s (%s)", name, err.Error())
			return err
		}
	}

	return nil
}

// RenameFile : Rename a file
func (fs *FileShare) RenameFile(source string, target string) error {
	log.Trace("FileShare::RenameFile : %s -> %s", source, target)

	srcFileName, srcDirPath := getFileAndDirFromPath(filepath.Join(fs.Config.prefixPath, source))
	tgtFileName, tgtDirPath := getFileAndDirFromPath(filepath.Join(fs.Config.prefixPath, target)) // need if renaming file indirectly through RenameDirectory(), where dir rather than filename needs to be changed

	srcFileURL := fs.Share.NewDirectoryURL(srcDirPath).NewFileURL(srcFileName)
	tgtFileURL := fs.Share.NewDirectoryURL(tgtDirPath).NewFileURL(tgtFileName)

	prop, err := srcFileURL.GetProperties(context.Background())
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

	startCopy, err := tgtFileURL.StartCopy(context.Background(), srcFileURL.URL(), prop.NewMetadata())

	if err != nil {
		log.Err("FileShare::RenameFile : Failed to start copy of file %s (%s)", source, err.Error())
		return err
	}

	copyStatus := startCopy.CopyStatus()
	for copyStatus == azfile.CopyStatusPending {
		time.Sleep(time.Second * 1)
		prop, err = tgtFileURL.GetProperties(context.Background())
		if err != nil {
			log.Err("FileShare::RenameFile : CopyStats : Failed to get file properties for %s (%s)", source, err.Error())
		}
		copyStatus = prop.CopyStatus()
	}
	log.Trace("FileShare::RenameFile : %s -> %s done", source, target)

	// Copy of the file is done so now delete the older file
	return fs.DeleteFile(source)
}

// RenameDirectory : Rename a directory
func (fs *FileShare) RenameDirectory(source string, target string) error {
	log.Trace("FileShare::RenameDirectory : %s -> %s", source, target)

	fs.CreateDirectory(target)

	for marker := (azfile.Marker{}); marker.NotDone(); {
		listFile, err := fs.Share.NewDirectoryURL(filepath.Join(fs.Config.prefixPath, source)).ListFilesAndDirectoriesSegment(context.Background(), marker,
			azfile.ListFilesAndDirectoriesOptions{
				MaxResults: common.MaxDirListCount,
			})
		if err != nil {
			log.Err("FileShare::RenameDirectory : Failed to get list of files and directories %s", err.Error())
			fs.DeleteDirectory(target)
			return err
		}
		marker = listFile.NextMarker

		// Process the files returned in this result segment (if the segment is empty, the loop body won't execute)
		for _, fileInfo := range listFile.FileItems {
			err = fs.RenameFile(filepath.Join(source, fileInfo.Name), filepath.Join(target, fileInfo.Name))
			if err != nil {
				log.Err("FileShare::RenameDirectory : Failed to move files to new directory %s", err.Error())
				fs.DeleteDirectory(target)
				return err
			}
		}

		for _, dirInfo := range listFile.DirectoryItems {
			err = fs.RenameDirectory(filepath.Join(source, dirInfo.Name), filepath.Join(target, dirInfo.Name))
			if err != nil {
				log.Err("FileShare::RenameDirectory : Failed to move subdirectories to new directory  %s", err.Error())
				fs.DeleteDirectory(target)
				return err
			}
		}
	}

	return fs.DeleteDirectory(source)
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
	log.Err("FileShare::GetAttr : Failed to get file/directory properties for %s (%s)", name, fileerr.Error())
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
		log.Err("File::List : Failed to list the container with the prefix %s", err.Error())
		return fileList, nil, err
	}

	// Process the files returned in this result segment (if the segment is empty, the loop body won't execute)
	for _, fileInfo := range listFile.FileItems {
		attr := &internal.ObjAttr{
			Path: split(fs.Config.prefixPath, filepath.Join(listPath, fileInfo.Name)),
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
			Path: split(fs.Config.prefixPath, filepath.Join(listPath, dirInfo.Name)),
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

// ReadToFile : Download an Azure file to a local file
func (fs *FileShare) ReadToFile(name string, offset int64, count int64, fi *os.File) error {
	log.Trace("FileShare::ReadToFile : name %s, offset : %d, count %d", name, offset, count)
	//defer exectime.StatTimeCurrentBlock("FileShare::ReadToFile")()

	if offset != 0 {
		log.Err("FileShare::ReadToFile : offset is not 0")
		return errors.New("offset is not 0")
	}

	fileName, dirPath := getFileAndDirFromPath(filepath.Join(fs.Config.prefixPath, name))
	fileURL := fs.Share.NewDirectoryURL(dirPath).NewFileURL(fileName)

	defer log.TimeTrack(time.Now(), "FileShare::ReadToFile", name)
	_, err := azfile.DownloadAzureFileToFile(context.Background(), fileURL, fi, fs.downloadOptions)

	if err != nil {
		e := storeFileErrToErr(err)
		if e == ErrFileNotFound {
			return syscall.ENOENT
		} else {
			log.Err("FileShare::ReadToFile : Failed to download file %s (%s)", name, err.Error())
			return err
		}
	}

	return nil
}

// ReadBuffer : Downloads a file to a buffer
func (fs *FileShare) ReadBuffer(name string, offset int64, len int64) ([]byte, error) {
	log.Trace("FileShare::ReadBuffer : name %s", name)
	var buff []byte

	if offset != 0 {
		log.Err("FileShare::ReadToFile : offset is not 0")
		return buff, errors.New("offset is not 0")
	}

	if len == 0 {
		len = azfile.CountToEnd
		attr, err := fs.GetAttr(name)
		if err != nil {
			return buff, err
		}
		buff = make([]byte, attr.Size)
	} else {
		buff = make([]byte, len)
	}

	fileName, dirPath := getFileAndDirFromPath(filepath.Join(fs.Config.prefixPath, name))
	fileURL := fs.Share.NewDirectoryURL(dirPath).NewFileURL(fileName)

	_, err := azfile.DownloadAzureFileToBuffer(context.Background(), fileURL, buff, fs.downloadOptions)

	if err != nil {
		e := storeFileErrToErr(err)
		if e == ErrFileNotFound {
			return buff, syscall.ENOENT
		} else if e == InvalidRange {
			return buff, syscall.ERANGE
		}

		log.Err("FileShare::ReadBuffer : Failed to download file %s (%s)", name, err.Error())
		return buff, err
	}

	return buff, nil
}

// ReadInBuffer : Download specific range from a file to a user provided buffer
func (fs *FileShare) ReadInBuffer(name string, offset int64, len int64, data []byte) error {
	log.Trace("FileShare::ReadInBuffer : name %s", name)

	if offset != 0 {
		log.Err("FileShare::ReadToFile : offset is not 0")
		return errors.New("offset is not 0")
	}

	fileName, dirPath := getFileAndDirFromPath(filepath.Join(fs.Config.prefixPath, name))
	fileURL := fs.Share.NewDirectoryURL(dirPath).NewFileURL(fileName)

	_, err := azfile.DownloadAzureFileToBuffer(context.Background(), fileURL, data, fs.downloadOptions)

	if err != nil {
		e := storeFileErrToErr(err)
		if e == ErrFileNotFound {
			return syscall.ENOENT
		} else if e == InvalidRange {
			return syscall.ERANGE
		}

		log.Err("FileShare::ReadInBuffer : Failed to download file %s (%s)", name, err.Error())
		return err
	}

	return nil
}

// WriteFromFile : Upload local file to Azure file
func (fs *FileShare) WriteFromFile(name string, metadata map[string]string, fi *os.File) error {
	log.Trace("FileShare::WriteFromFile : name %s", name)
	//defer exectime.StatTimeCurrentBlock("WriteFromFile::WriteFromFile")()

	fileName, dirPath := getFileAndDirFromPath(filepath.Join(fs.Config.prefixPath, name))
	fileURL := fs.Share.NewDirectoryURL(dirPath).NewFileURL(fileName)

	defer log.TimeTrack(time.Now(), "FileShare::WriteFromFile", name)
	var rangeSize int64

	fileSize := fs.Config.blockSize
	// if the range size is not set then we configure it based on file size
	if fileSize == 0 {
		// get the size of the file
		stat, err := fi.Stat()
		if err != nil {
			log.Err("FileShare::WriteFromFile : Failed to get file size %s (%s)", name, err.Error())
			return err
		}

		// based on file-size calculate range size
		rangeSize, err = fs.calculateRangeSize(name, stat.Size())
		if err != nil {
			log.Err("FileShare::calculateFileSize : Failed to get file size %s (%s)", name, err.Error())
			return err
		}
	}

	err := azfile.UploadFileToAzureFile(context.Background(), fi, fileURL, azfile.UploadToAzureFileOptions{
		RangeSize:   rangeSize,
		Parallelism: fs.Config.maxConcurrency,
		Metadata:    metadata,
		FileHTTPHeaders: azfile.FileHTTPHeaders{
			ContentType: getContentType(name),
		},
	})

	if err != nil {
		serr := storeFileErrToErr(err)
		if serr == ErrFileAlreadyExists {
			log.Err("BlockBlob::WriteFromFile : %s already exists (%s)", name, err.Error())
			return syscall.EIO
		} else {
			log.Err("BlockBlob::WriteFromFile : Failed to upload blob %s (%s)", name, err.Error())
		}
		return err
	}

	return nil
}

// WriteFromBuffer : Upload from a buffer to a file
func (fs *FileShare) WriteFromBuffer(name string, metadata map[string]string, data []byte) error {
	log.Trace("FileShare::WriteFromBuffer : name %s", name)

	fileName, dirPath := getFileAndDirFromPath(filepath.Join(fs.Config.prefixPath, name))
	fileURL := fs.Share.NewDirectoryURL(dirPath).NewFileURL(fileName)

	defer log.TimeTrack(time.Now(), "FileShare::WriteFromBuffer", name)
	err := azfile.UploadBufferToAzureFile(context.Background(), data, fileURL, azfile.UploadToAzureFileOptions{
		RangeSize:   fs.Config.blockSize,
		Parallelism: fs.Config.maxConcurrency,
		Metadata:    metadata,
		FileHTTPHeaders: azfile.FileHTTPHeaders{
			ContentType: getContentType(name),
		},
	})

	if err != nil {
		log.Err("FileShare::WriteFromBuffer : Failed to upload file %s (%s)", name, err.Error())
		return err
	}

	return nil
}

// ChangeMod : Change mode of a file
func (fs *FileShare) ChangeMod(name string, _ os.FileMode) error {
	log.Trace("FileShare::ChangeMod : name %s", name)

	if fs.Config.ignoreAccessModifiers {
		// for operations like git clone where transaction fails if chmod is not successful
		// return success instead of ENOSYS
		return nil
	}

	// This is not currently supported for a fileshare account
	return syscall.ENOTSUP
}

// ChangeOwner : Change owner of a file
func (fs *FileShare) ChangeOwner(name string, _ int, _ int) error {
	log.Trace("FileShare::ChangeOwner : name %s", name)

	if fs.Config.ignoreAccessModifiers {
		// for operations like git clone where transaction fails if chown is not successful
		// return success instead of ENOSYS
		return nil
	}

	// This is not currently supported for a fileshare account
	return syscall.ENOTSUP
}

// StageAndCommit : write data to an Azure file given a list of ranges
func (fs *FileShare) StageAndCommit(name string, bol *common.BlockOffsetList) error {
	log.Trace("FileShare::StageAndCommit : name %s", name)

	fileName, dirPath := getFileAndDirFromPath(filepath.Join(fs.Config.prefixPath, name))
	fileURL := fs.Share.NewDirectoryURL(dirPath).NewFileURL(fileName)

	var data []byte

	for _, rng := range bol.BlockList {
		if rng.Truncated() {
			data = make([]byte, rng.EndIndex-rng.StartIndex)
			rng.Flags.Clear(common.TruncatedBlock)
		} else {
			data = rng.Data
		}
		if rng.Dirty() {
			_, err := fileURL.UploadRange(context.Background(),
				rng.StartIndex,
				bytes.NewReader(data),
				nil,
			)
			if err != nil {
				log.Err("FileShare::StageAndCommit : Failed to upload range to file %s at index %v (%s)", name, rng.StartIndex, err.Error())
				return err
			}
			rng.Flags.Clear(common.DirtyBlock)
		}
	}
	return nil
}

// Write : write data at given offset to an Azure file
func (fs *FileShare) Write(options internal.WriteFileOptions) (err error) {
	name := options.Handle.Path
	offset := options.Offset
	data := options.Data
	// length := int64(len(options.Data))

	defer log.TimeTrack(time.Now(), "FileShare::Write", options.Handle.Path)
	log.Trace("FileShare::Write : name %s offset %v", name, offset)

	if len(data) == 0 {
		return nil
	}

	fileName, dirPath := getFileAndDirFromPath(filepath.Join(fs.Config.prefixPath, name))
	fileURL := fs.Share.NewDirectoryURL(dirPath).NewFileURL(fileName)

	// fileOffsets, err := fs.GetFileBlockOffsets(name)
	// if err != nil {
	// 	log.Err("FileShare::Write : Failed to get file range offsets %s", err.Error())
	// 	return err
	// }

	// _, _, exceedsFileBlocks, _ := fileOffsets.FindBlocksToModify(offset, length) // **********but this method only looks for true file size rather than file capacity
	// if exceedsFileBlocks {
	// 	err = fs.TruncateFile(name, offset+length)
	// 	if err != nil {
	// 		log.Err("FileShare::Write : Failed to truncate Azure file %s", err.Error())
	// 		return err
	// 	}
	// }

	_, err = fileURL.UploadRange(context.Background(), options.Offset, bytes.NewReader(data), nil)
	if err != nil {
		log.Err("FileShare::Write : Failed to write data to Azure file %s", err.Error())
		return err
	}

	return nil
}

// GetFileBlockOffsets : store file range list and corresponding offsets
func (fs *FileShare) GetFileBlockOffsets(name string) (shareFileRangeList *common.BlockOffsetList, err error) {
	log.Trace("FileShare::GetFileBlockOffsets : name %s", name)
	rangeList := common.BlockOffsetList{}

	fileName, dirPath := getFileAndDirFromPath(filepath.Join(fs.Config.prefixPath, name))
	fileURL := fs.Share.NewDirectoryURL(dirPath).NewFileURL(fileName)

	storageRangeList, err := fileURL.GetRangeList(
		context.Background(), 0, 0)
	if err != nil {
		log.Err("FileShare::GetFileBlockOffsets : Failed to get range list %s ", name, err.Error())
		return &common.BlockOffsetList{}, err
	}

	if len(rangeList.BlockList) == 0 {
		rangeList.Flags.Set(common.SmallFile)
		return &rangeList, nil
	}
	for _, rng := range storageRangeList.Ranges {
		fileRng := &common.Block{
			StartIndex: rng.Start,
			EndIndex:   rng.End,
		}
		rangeList.BlockList = append(rangeList.BlockList, fileRng)
	}

	return &rangeList, nil
}

// TruncateFile : resize the file to a smaller, equal, or bigger size
func (fs *FileShare) TruncateFile(name string, size int64) (err error) {
	log.Trace("FileShare::TruncateFile : name=%s, size=%d", name, size)

	fileName, dirPath := getFileAndDirFromPath(filepath.Join(fs.Config.prefixPath, name))
	fileURL := fs.Share.NewDirectoryURL(dirPath).NewFileURL(fileName)

	_, err = fileURL.Resize(context.Background(), size)
	if err != nil {
		log.Err("FileShare::TruncateFile : failed to resize file %s", name)
		return err
	}
	return nil
}

// getFileAndDirFromPath : Helper that separates directory/directories and file name of a given file/directory path
// Covers case where name param includes subdirectories and not just the file name
// Only call when path includes file
// Assumes files don't have "/" at the end whereas directories do
func getFileAndDirFromPath(completePath string) (fileName string, dirPath string) {
	if completePath == "" {
		return "", ""
	}

	splitPath := strings.Split(completePath, "/")

	dirArray := splitPath[:len(splitPath)-1]
	dirPath = strings.Join(dirArray, "/") // doesn't end with "/"

	fileName = filepath.Base(completePath)

	return fileName, dirPath
}

// calculateRangeSize : calulates range size of the file based on file size
func (fs *FileShare) calculateRangeSize(name string, fileSize int64) (rangeSize int64, err error) {
	if fileSize > FileMaxSizeInBytes {
		log.Err("FileShare::calculateRangeSize : buffer is too large to upload to an Azure file %s", name)
		err = errors.New("buffer is too large to upload to an Azure file")
		return 0, err
	}

	if fileSize <= azfile.FileMaxUploadRangeBytes {
		// Files up to 4MB can be uploaded as a single range
		rangeSize = azfile.FileMaxUploadRangeBytes
	} else {
		// max number of ranges = max file size / max size for one range
		fileShareMaxRanges := FileMaxSizeInBytes / azfile.FileMaxUploadRangeBytes

		// buffer / max number of file ranges = range size to use for all ranges
		rangeSize = int64(math.Ceil(float64(fileSize) / float64(fileShareMaxRanges)))

		if rangeSize < azfile.FileMaxUploadRangeBytes {
			// Range size is smaller than 4MB then consider 4MB as default
			rangeSize = azfile.FileMaxUploadRangeBytes
		} else {
			if (rangeSize & (-8)) != 0 {
				// EXTRA : round off the range size to next higher multiple of 8.
				// No reason to do so; assuming odd numbers in range size will not be good on server end
				rangeSize = (rangeSize + 7) & (-8)
			}

			if rangeSize > azfile.FileMaxUploadRangeBytes {
				// After rounding off the rangeSize has become bigger then max allowed range size.
				log.Err("FileShare::calculateRangeSize : rangeSize exceeds max allowed range size for %s", name)
				err = errors.New("ragnge size is too large to upload to a file")
				return 0, err
			}
		}
	}

	log.Info("FileShare::calculateRangeSize : %s size %lu, blockSize %lu", name, fileSize, rangeSize)
	return rangeSize, nil
}
