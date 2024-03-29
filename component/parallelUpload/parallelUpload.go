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
	"io/ioutil"
	"os"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/config"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal"
	"github.com/Azure/azure-storage-fuse/v2/internal/handlemap"
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
	threadPool      *ThreadPool // Pool of threads
	targetDirectory string      // Disk path to the target directory
	workers         uint32      // Number of threads working to fetch the blocks
}

// Structure defining your config parameters
type ParallelUploadOptions struct {
	TargetDirectory string `config:"path" yaml:"path,omitempty"`
}

const compName = "parallelUpload"

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

type Queue struct {
	items []string
}

// Enqueue adds an item to the end of the queue.
func (q *Queue) Enqueue(item string) {
	q.items = append(q.items, item)
}

// Dequeue removes an item from the front of the queue and returns it.
func (q *Queue) Dequeue() string {
	if len(q.items) == 0 {
		return ""
	}
	item := q.items[0]
	q.items = q.items[1:]
	return item
}

// CreateFile: Create a new file
func (pu *ParallelUpload) Upload(options internal.UploadOptions) (*handlemap.Handle, error) {
	log.Trace("BlockCache::CreateFile : name=%s, mode=%d", options.Name, options.Mode)

	_, err := c.NextComponent().CreateFile(options)
	if err != nil {
		log.Err("BlockCache::CreateFile : Failed to create file %s", options.Name)
		return nil, err
	}

	// Initialize the queue
	queue := Queue{}

	// List files in the current directory
	files, err := ioutil.ReadDir(c.targetDirectory)
	if err != nil {
		log.Fatal(err)
	}

	// Add files to the queue
	for _, file := range files {
		if !file.IsDir() { //review: assumming there are only files in the folder
			queue.Enqueue(file.Name())
		}
	}

	c.threadPool = newThreadPool(16, c.download, c.upload)
	if c.threadPool == nil {
		log.Err("BlockCache::Configure : fail to init thread pool")
		return fmt.Errorf("config error in %s [fail to init thread pool]", c.Name())
	}
	// Dequeue and print files
	for len(queue.items) > 0 {
		fmt.Println(queue.Dequeue())
	}

	handle := handlemap.NewHandle(options.Name)
	handle.Size = 0
	handle.Mtime = time.Now()

	// As file is created on storage as well there is no need to mark this as dirty
	// Any write operation to file will mark it dirty and flush will then reupload
	// handle.Flags.Set(handlemap.HandleFlagDirty)
	c.prepareHandleForBlockCache(handle)
	return handle, nil
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
