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

package xload

import (
	"os"
	"path/filepath"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common/config"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal"
)

// verify that the below types implement the xcomponent interfaces
var _ xcomponent = &lister{}
var _ xcomponent = &remoteLister{}

// verify that the below types implement the xenumerator interfaces
var _ enumerator = &remoteLister{}

const LISTER string = "lister"

type lister struct {
	xbase
	path string // base path of the directory to be listed
}

type enumerator interface {
	mkdir(name string) error
}

// --------------------------------------------------------------------------------------------------------

type remoteLister struct {
	lister
	listBlocked bool
}

func newRemoteLister(path string, remote internal.Component, statsMgr *statsManager) (*remoteLister, error) {
	log.Debug("lister::newRemoteLister : create new remote lister for %s", path)

	rl := &remoteLister{
		lister: lister{
			path: path,
			xbase: xbase{
				remote:   remote,
				statsMgr: statsMgr,
			},
		},
		listBlocked: false,
	}

	rl.setName(LISTER)
	rl.init()
	return rl, nil
}

func (rl *remoteLister) init() {
	rl.pool = newThreadPool(MAX_LISTER, rl.process)
	if rl.pool == nil {
		log.Err("remoteLister::init : fail to init thread pool")
	}
}

func (rl *remoteLister) start() {
	log.Debug("remoteLister::start : start remote lister for %s", rl.path)
	rl.getThreadPool().Start()
	rl.getThreadPool().Schedule(&workItem{compName: rl.getName()})
}

func (rl *remoteLister) stop() {
	log.Debug("remoteLister::stop : stop remote lister for %s", rl.path)
	if rl.getThreadPool() != nil {
		rl.getThreadPool().Stop()
	}
	rl.getNext().stop()
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

func (rl *remoteLister) process(item *workItem) (int, error) {
	absPath := item.path // TODO:: xload : check this for subdirectory mounting

	log.Debug("remoteLister::process : Reading remote dir %s", absPath)

	// this block will be executed only in the first list call for the remote directory
	// so haven't made the listBlocked variable atomic
	if !rl.listBlocked {
		log.Debug("remoteLister::process : Waiting for block-list-on-mount-sec before making the list call")
		err := waitForListTimeout()
		if err != nil {
			log.Err("remoteLister::process : unable to unmarshal block-list-on-mount-sec [%s]", err.Error())
			return 0, err
		}
		rl.listBlocked = true
	}

	marker := ""
	var cnt, iteration int
	for {
		entries, new_marker, err := rl.getRemote().StreamDir(internal.StreamDirOptions{
			Name:  absPath,
			Token: marker,
		})
		if err != nil {
			log.Err("remoteLister::process : Remote listing failed for %s [%s]", absPath, err.Error())
		}

		marker = new_marker
		cnt += len(entries)
		iteration++
		log.Debug("remoteLister::process : count: %d , iterations: %d", cnt, iteration)

		// send number of items listed in current iteration to stats manager
		rl.getStatsManager().addStats(&statsItem{
			component:   LISTER,
			name:        absPath,
			listerCount: uint64(len(entries)),
		})

		for _, entry := range entries {
			log.Debug("remoteLister::process : Iterating: %s, Is directory: %v", entry.Path, entry.IsDir())

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
						log.Err("remoteLister::process : Failed to create directory [%s]", err.Error())
						return
					}

					// push the directory to input pool for its listing
					rl.getThreadPool().Schedule(&workItem{
						compName: rl.getName(),
						path:     name,
					})
				}(entry.Path)
			} else {
				// send file to the output channel for chunking
				rl.getNext().getThreadPool().Schedule(&workItem{
					compName: rl.getNext().getName(),
					path:     entry.Path,
					dataLen:  uint64(entry.Size),
				})
			}
		}

		if len(new_marker) == 0 {
			log.Debug("remoteLister::process : remote listing done for %s", absPath)
			break
		}
	}

	return cnt, nil
}

func (rl *remoteLister) mkdir(name string) error {
	log.Debug("remoteLister::mkdir : Creating local path: %s", name)
	err := os.MkdirAll(name, 0777)

	// send stats for dir creation
	rl.getStatsManager().addStats(&statsItem{
		component: SPLITTER, // using splitter here to track the success/failure of mkdir operation
		name:      name,
		success:   err == nil,
		download:  true,
	})
	return err
}
