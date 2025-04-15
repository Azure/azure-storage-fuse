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

package xload

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common/config"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal"
)

// verify that the below types implement the xcomponent interfaces
var _ XComponent = &lister{}
var _ XComponent = &remoteLister{}

// verify that the below types implement the xenumerator interfaces
var _ enumerator = &remoteLister{}

type lister struct {
	XBase
	path              string      // base path of the directory to be listed
	defaultPermission os.FileMode // default permission of files and directories in the local path
}

type enumerator interface {
	mkdir(name string) error
}

// --------------------------------------------------------------------------------------------------------

type remoteLister struct {
	lister
	listBlocked bool
}

type remoteListerOptions struct {
	path              string
	workerCount       uint32
	defaultPermission os.FileMode
	remote            internal.Component
	statsMgr          *StatsManager
}

func newRemoteLister(opts *remoteListerOptions) (*remoteLister, error) {
	if opts == nil || opts.path == "" || opts.remote == nil || opts.statsMgr == nil || opts.workerCount == 0 {
		log.Err("lister::NewRemoteLister : invalid parameters sent to create remote lister")
		return nil, fmt.Errorf("invalid parameters sent to create remote lister")
	}

	log.Debug("lister::NewRemoteLister : create new remote lister for %s, default permission %v, workers %v", opts.path, opts.defaultPermission, opts.workerCount)

	rl := &remoteLister{
		lister: lister{
			path:              opts.path,
			defaultPermission: opts.defaultPermission,
		},
		listBlocked: false,
	}

	rl.SetName(LISTER)
	rl.SetWorkerCount(opts.workerCount)
	rl.SetRemote(opts.remote)
	rl.SetStatsManager(opts.statsMgr)
	rl.Init()
	return rl, nil
}

func (rl *remoteLister) Init() {
	rl.SetThreadPool(NewThreadPool(rl.GetWorkerCount(), rl.Process))
	if rl.GetThreadPool() == nil {
		log.Err("remoteLister::Init : fail to init thread pool")
	}
}

func (rl *remoteLister) Start() {
	log.Debug("remoteLister::Start : start remote lister for %s", rl.path)
	rl.GetThreadPool().Start()
	rl.Schedule(&WorkItem{CompName: rl.GetName()})
}

func (rl *remoteLister) Stop() {
	log.Debug("remoteLister::Stop : stop remote lister for %s", rl.path)
	if rl.GetThreadPool() != nil {
		rl.GetThreadPool().Stop()
	}
	rl.GetNext().Stop()
}

// wait for the configured block-list-on-mount-sec to make the list call
func waitForListTimeout() error {
	var blockListSeconds uint16 = 0
	err := config.UnmarshalKey("azstorage.block-list-on-mount-sec", &blockListSeconds)
	if err != nil {
		return err
	}
	time.Sleep(time.Duration(blockListSeconds) * time.Second)
	return nil
}

func (rl *remoteLister) Process(item *WorkItem) (int, error) {
	relPath := item.Path // TODO:: xload : check this for subdirectory mounting

	log.Debug("remoteLister::Process : Reading remote dir %s", relPath)

	// this block will be executed only in the first list call for the remote directory
	// so haven't made the listBlocked variable atomic
	if !rl.listBlocked {
		log.Debug("remoteLister::Process : Waiting for block-list-on-mount-sec before making the list call")
		err := waitForListTimeout()
		if err != nil {
			log.Err("remoteLister::Process : unable to unmarshal block-list-on-mount-sec [%s]", err.Error())
			return 0, err
		}
		rl.listBlocked = true
	}

	marker := ""
	var cnt, iteration int
	for {
		entries, new_marker, err := rl.GetRemote().StreamDir(internal.StreamDirOptions{
			Name:  relPath,
			Token: marker,
		})
		if err != nil {
			log.Err("remoteLister::Process : Remote listing failed for %s [%s]", relPath, err.Error())
			break
		}

		marker = new_marker
		cnt += len(entries)
		iteration++
		log.Debug("remoteLister::Process : count: %d , iterations: %d", cnt, iteration)

		// send number of items listed in current iteration to stats manager
		rl.GetStatsManager().AddStats(&StatsItem{
			Component:   LISTER,
			Name:        relPath,
			ListerCount: uint64(len(entries)),
		})

		for _, entry := range entries {
			log.Debug("remoteLister::Process : Iterating: %s, Is directory: %v", entry.Path, entry.IsDir())

			if entry.IsDir() {
				// create directory in local
				// spawn go routine for directory creation and then
				// adding to the input channel of the listing component
				// TODO:: xload : check how many threads can we spawn
				go func(name string) {
					localPath := filepath.Join(rl.path, name)
					err = rl.mkdir(localPath)
					// TODO:: xload : handle error
					if err != nil {
						log.Err("remoteLister::Process : Failed to create directory [%s]", err.Error())
						return
					}

					// push the directory to input pool for its listing
					rl.Schedule(&WorkItem{
						CompName: rl.GetName(),
						Path:     name,
					})
				}(entry.Path)
			} else {
				fileMode := rl.defaultPermission
				if !entry.IsModeDefault() {
					fileMode = entry.Mode
				}

				// send file to the splitter's channel for chunking
				rl.GetNext().Schedule(&WorkItem{
					CompName: rl.GetNext().GetName(),
					Path:     entry.Path,
					DataLen:  uint64(entry.Size),
					Mode:     fileMode,
					Atime:    entry.Atime,
					Mtime:    entry.Mtime,
					MD5:      entry.MD5,
				})
			}
		}

		if len(new_marker) == 0 {
			log.Debug("remoteLister::Process : remote listing done for %s", relPath)
			break
		}
	}

	return cnt, nil
}

func (rl *remoteLister) mkdir(name string) error {
	log.Debug("remoteLister::mkdir : Creating local path: %s, mode %v", name, rl.defaultPermission)
	err := os.MkdirAll(name, rl.defaultPermission)

	// send stats for dir creation
	rl.GetStatsManager().AddStats(&StatsItem{
		Component: LISTER,
		Name:      name,
		Dir:       true,
		Success:   err == nil,
		Download:  true,
	})
	return err
}
