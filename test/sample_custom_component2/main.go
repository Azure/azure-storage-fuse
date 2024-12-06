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

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common/config"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/exported"
)

// SAMPLE CUSTOM COMPONENT IMPLEMENTATION
// To build this component run the following command:
// "go build -buildmode=plugin -o sample_custom_component2.so"
// This is a sample custom component implementation that can be used as a reference to implement custom components.
// The custom component should implement the exported.Component interface.
const (
	CompName = "sample_custom_component2"
	Mb       = 1024 * 1024
)

var _ exported.Component = &sample_custom_component2{}

func (e *sample_custom_component2) SetName(name string) {
	e.BaseComponent.SetName(name)
}

func (e *sample_custom_component2) SetNextComponent(nc exported.Component) {
	e.BaseComponent.SetNextComponent(nc)
}

func GetExternalComponent() (string, func() exported.Component) {
	return CompName, NewexternalComponent
}

func NewexternalComponent() exported.Component {
	comp := &sample_custom_component2{}
	comp.SetName(CompName)
	return comp
}

type sample_custom_component2 struct {
	blockSize    int64
	externalPath string
	exported.BaseComponent
}

type sample_custom_component2Options struct {
	BlockSize int64 `config:"block-size-mb" yaml:"block-size-mb,omitempty"`
}

func (e *sample_custom_component2) Configure(isParent bool) error {
	log.Info("sample_custom_component2:: Configure")
	externalPath := os.Getenv("EXTERNAL_PATH")
	if externalPath == "" {
		log.Err("EXTERNAL_PATH not set")
		return fmt.Errorf("EXTERNAL_PATH not set")
	}
	e.externalPath = externalPath
	conf := sample_custom_component2Options{}
	err := config.UnmarshalKey(e.Name(), &conf)
	if err != nil {
		log.Err("sample_custom_component2::Configure : config error [invalid config attributes]")
		return fmt.Errorf("error reading config for %s: %w", e.Name(), err)
	}
	if config.IsSet(e.Name()+".block-size-mb") && conf.BlockSize > 0 {
		e.blockSize = conf.BlockSize * int64(Mb)
	}
	return nil
}

func (e *sample_custom_component2) CreateFile(options exported.CreateFileOptions) (*exported.Handle, error) {
	log.Info("sample_custom_component2::CreateFile")
	filePath := filepath.Join(e.externalPath, options.Name)
	fileHandle, err := os.OpenFile(filePath, os.O_CREATE, 0777)
	if err != nil {
		log.Err("Failed to create file %s", filePath)
		return nil, err
	}
	defer fileHandle.Close()
	handle := &exported.Handle{
		Path: options.Name,
	}
	return handle, nil
}

func (e *sample_custom_component2) CreateDir(options exported.CreateDirOptions) error {
	log.Info("sample_custom_component2::CreateDir")
	dirPath := filepath.Join(e.externalPath, options.Name)
	err := os.MkdirAll(dirPath, 0777)
	if err != nil {
		log.Err("Failed to create directory %s", dirPath)
		return err
	}
	return nil
}

func (e *sample_custom_component2) StreamDir(options exported.StreamDirOptions) ([]*exported.ObjAttr, string, error) {
	log.Info("sample_custom_component2::StreamDir")
	var objAttrs []*exported.ObjAttr
	path := formatListDirName(options.Name)
	files, err := os.ReadDir(filepath.Join(e.externalPath, path))
	if err != nil {
		log.Trace("test1::StreamDir : Error reading directory %s : %s", path, err.Error())
		return nil, "", err
	}

	for _, file := range files {
		attr, err := e.GetAttr(exported.GetAttrOptions{Name: path + file.Name()})
		if err != nil {
			if err != syscall.ENOENT {
				log.Trace("test1::StreamDir : Error getting file attributes: %s", err.Error())
				return objAttrs, "", err
			}
			log.Trace("test1::StreamDir : File not found: %s", file.Name())
			continue
		}

		objAttrs = append(objAttrs, attr)
	}

	return objAttrs, "", nil
}
func (e *sample_custom_component2) IsDirEmpty(options exported.IsDirEmptyOptions) bool {
	log.Info("sample_custom_component2::IsDirEmpty")
	files, err := os.ReadDir(filepath.Join(e.externalPath, options.Name))
	if err != nil {
		log.Err("Failed to read directory %s", options.Name)
		return false
	}
	return len(files) == 0
}

func (e *sample_custom_component2) DeleteDir(options exported.DeleteDirOptions) error {
	log.Info("sample_custom_component2::DeleteDir")
	dirPath := filepath.Join(e.externalPath, options.Name)
	err := os.RemoveAll(dirPath)
	if err != nil {
		log.Err("Failed to delete directory %s", dirPath)
		return err
	}
	return nil
}

func (e *sample_custom_component2) StageData(opt exported.StageDataOptions) error {
	log.Info("sample_custom_component2:: StageData")
	filePath := filepath.Join(e.externalPath, opt.Name)
	fileHandle, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY, 0777)
	if err != nil {
		log.Err("Failed to open file %s", filePath)
		return err
	}
	defer fileHandle.Close()

	fileOffset := int64(opt.Offset)

	_, err = fileHandle.WriteAt(opt.Data, fileOffset)

	if err != nil {
		log.Err("Failed to write to file %s at offset %d", filePath, fileOffset)
		return err
	}

	return nil
}

func (e *sample_custom_component2) ReadInBuffer(opt exported.ReadInBufferOptions) (length int, err error) {
	log.Info("sample_custom_component2:: ReadInBuffer")

	filePath := filepath.Join(e.externalPath, opt.Handle.Path)
	fileHandle, err := os.OpenFile(filePath, os.O_RDONLY, 0777)
	if err != nil {
		log.Err("Failed to open file %s", filePath)
		return 0, err
	}
	defer fileHandle.Close()

	n, err := fileHandle.ReadAt(opt.Data, opt.Offset)
	if err != nil {
		log.Err("Failed to read from file %s at offset %d", filePath, opt.Offset)
		return 0, err
	}
	return n, nil
}

func (e *sample_custom_component2) GetAttr(options exported.GetAttrOptions) (attr *exported.ObjAttr, err error) {
	log.Info("sample_custom_component2::GetAttr for %s", options.Name)
	fileAttr, err := os.Stat(filepath.Join(e.externalPath, options.Name))
	if err != nil {
		log.Trace("sample_custom_component2::GetAttr : Error getting file attributes: %s", err.Error())
		return &exported.ObjAttr{}, err
	}

	// Populate the ObjAttr struct with the file info.
	attr = &exported.ObjAttr{
		Mtime:  fileAttr.ModTime(), // Modified time
		Atime:  time.Now(),         // Access time (current time as approximation)
		Ctime:  fileAttr.ModTime(), // Change time (same as modified time in this case)
		Crtime: fileAttr.ModTime(), // Creation time (not available in Go, using modified time)
		Mode:   fileAttr.Mode(),    // Permissions
		Path:   options.Name,       // File path
		Name:   fileAttr.Name(),    // Base name of the path
		Size:   fileAttr.Size(),    // File size
	}
	if fileAttr.IsDir() {
		attr.Flags.Set(exported.PropFlagIsDir)
	}
	return attr, nil
}

func formatListDirName(path string) string {
	// If we check the root directory, make sure we pass "" instead of "/"
	// If we aren't checking the root directory, then we want to extend the directory name so List returns all children and does not include the path itself.
	if path == "/" {
		path = ""
	} else if path != "" {
		path = exported.ExtendDirName(path)
	}
	return path
}
