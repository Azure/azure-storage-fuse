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

package parallelUpload

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/config"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal"
)

/* NOTES:
   - Component shall have a structure which inherits "internal.BaseComponent" to participate in pipeline
   - Component shall register a name and its constructor to participate in pipeline  (add by default by generator)
   - Order of calls : Constructor -> Configure -> Start ..... -> Stop
   - To read any new setting from config file follow the Configure method default comments
*/

// Common structure for Component
type ParallelUpload struct {
	internal.BaseComponent
	targetDirectory string // Disk path to the target directory
}

// Structure defining your config parameters
type ParallelUploadOptions struct {
	TargetDirectory string `config:"path" yaml:"path,omitempty"`
}

const compName = "parallel_upload"

// Verification to check satisfaction criteria with Component Interface
var _ internal.Component = &ParallelUpload{}

func (c *ParallelUpload) Name() string {
	return compName
}

func (c *ParallelUpload) SetName(name string) {
	c.BaseComponent.SetName(name)
}

func (c *ParallelUpload) SetNextComponent(nc internal.Component) {
	c.BaseComponent.SetNextComponent(nc)
}

// Start : Pipeline calls this method to start the component functionality
//
//	this shall not block the call otherwise pipeline will not start
func (c *ParallelUpload) Start(ctx context.Context) error {
	log.Trace("ParallelUpload::Start : Starting component %s", c.Name())

	// ParallelUpload : start code goes here
	err := c.Upload2()
	if err != nil {
		log.Err("ParallelUpload::Upload functin call failed")
		return fmt.Errorf("ParallelUpload: UPLOAD FUNCTION CALL FAILED")
	}

	return nil
}

// Stop : Stop the component functionality and kill all threads started
func (c *ParallelUpload) Stop() error {
	log.Trace("ParallelUpload::Stop : Stopping component %s", c.Name())

	return nil
}

// Configure : Pipeline will call this method after constructor so that you can read config and initialize yourself
//
//	Return failure if any config is not valid to exit the process
func (c *ParallelUpload) Configure(_ bool) error {
	log.Trace("ParallelUpload::Configure : %s", c.Name())

	// >> If you do not need any config parameters remove below code and return nil
	conf := ParallelUploadOptions{}
	err := config.UnmarshalKey(c.Name(), &conf)
	if err != nil {
		log.Err("ParallelUpload::Configure : config error [invalid config attributes]")
		return fmt.Errorf("ParallelUpload: config error [invalid config attributes]")
	}

	c.targetDirectory = "./" //review
	if config.IsSet(compName + ".path") {
		c.targetDirectory = common.ExpandPath(conf.TargetDirectory)
		// Extract values from 'conf' and store them as you wish here
		_, err = os.Stat(c.targetDirectory)
		if os.IsNotExist(err) {
			log.Info("ParallelUpload: config error [target-directory does not exist.]")
			return fmt.Errorf("config error in %s [%s]", c.Name(), err.Error())
		}
	}
	return nil
}

func (c *ParallelUpload) parallelUploadFileToBlob(file string, filePath string, wg *sync.WaitGroup, ch chan<- error) error {
	defer wg.Done()

	uploadHandle, err := os.Open(filePath)
	if err != nil {
		ch <- fmt.Errorf("ParallelUpload::ParallelUploadFileToBlob : error [unable to open upload handle] %s [%s]", filePath, err.Error())
		return err
	}
	defer uploadHandle.Close()

	err = c.NextComponent().CopyFromFile(
		internal.CopyFromFileOptions{
			Name: file,
			File: uploadHandle,
		})

	uploadHandle.Close()
	if err != nil {
		log.Err("ParallelUpload::ParallelUploadFileToBlob : %s upload failed [%s]", file, err.Error())
		return err
	}
	fmt.Printf("Uploaded file %s to Azure Blob Storage\n", filePath)
	return nil
}

func (c *ParallelUpload) Upload() error {
	// log.Trace("ParallelUpload::CreateFile : name=%s, mode=%d", options.Name, options.Mode)
	log.Trace("ParallelUpload::Upload : Starting component")

	var wg sync.WaitGroup
	ch := make(chan error)

	// Create a worker pool with 16 goroutines
	poolSize := 16
	sem := make(chan struct{}, poolSize)

	// Walk through the directory tree
	err := filepath.Walk(c.targetDirectory, func(filePath string, fileInfo os.FileInfo, err error) error {
		if err != nil {
			fmt.Printf("Error accessing path %s: %v\n", filePath, err)
			return nil
		}
		if !fileInfo.IsDir() {
			sem <- struct{}{}
			wg.Add(1)
			go func(filePath string) {
				defer func() { <-sem }()
				// c.parallelUploadFileToBlob(file.Name(), filePath, &wg, ch)
			}(filePath)
		}
		return nil
	})
	if err != nil {
		fmt.Printf("Error walking through directory: %v\n", err)
		os.Exit(1)
	}

	// Wait for all goroutines to finish
	go func() {
		wg.Wait()
		close(ch)
	}()

	// Listen for errors from goroutines
	for err := range ch {
		if err != nil {
			fmt.Println(err)
		}
	}
	return nil
}

func (c *ParallelUpload) Upload2() error {
	// log.Trace("ParallelUpload::CreateFile : name=%s, mode=%d", options.Name, options.Mode)
	log.Trace("ParallelUpload::Upload : Starting component")
	// fmt.Print("SEE")
	files, err := os.ReadDir(c.targetDirectory)
	if err != nil {
		fmt.Printf("Failed to read directory: %v\n", err)
		os.Exit(1)
	}

	var wg sync.WaitGroup
	ch := make(chan error)

	// Create a worker pool with 16 goroutines
	poolSize := 16
	sem := make(chan struct{}, poolSize)

	for _, file := range files {
		if !file.IsDir() {
			filePath := filepath.Join(c.targetDirectory, file.Name())
			fileName := file.Name()
			sem <- struct{}{}
			wg.Add(1)
			go func(filePath string) {
				defer func() { <-sem }()
				c.parallelUploadFileToBlob(fileName, filePath, &wg, ch)
			}(filePath)
		}
		fmt.Printf("Number of goroutines running: %d\n", runtime.NumGoroutine())
	}

	// Wait for all goroutines to finish
	go func() {
		wg.Wait()
		close(ch)
		// Check the number of goroutines running
	}()

	// Listen for errors from goroutines
	for err := range ch {
		if err != nil {
			fmt.Println(err)
		}
	}
	return nil
}

// OnConfigChange : If component has registered, on config file change this method is called
func (c *ParallelUpload) OnConfigChange() {
}

// ------------------------- Factory -------------------------------------------

// Pipeline will call this method to create your object, initialize your variables here
// << DO NOT DELETE ANY AUTO GENERATED CODE HERE >>
func NewParallelUploadComponent() internal.Component {
	comp := &ParallelUpload{}
	comp.SetName(compName)
	return comp
}

// On init register this component to pipeline and supply your constructor
func init() {
	internal.AddComponent(compName, NewParallelUploadComponent)

	targetPathFlag := config.AddStringFlag("target-path", "", "configures the path for the target directory. Configure the fastest disk (SSD or ramdisk) for best performance.")
	config.BindPFlag(compName+".path", targetPathFlag)
}
