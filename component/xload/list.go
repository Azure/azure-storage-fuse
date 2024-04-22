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
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal"
)

// Verification to check satisfaction criteria with listing Interface
var _ enumerator = &local{}
var _ enumerator = &remote{}

type enumerator interface {
	readDir(item *workItem) (int, error)
	mkdir(name string) error
	getInputPool() *ThreadPool
	setOutputPool(pool *ThreadPool)
}

type lister struct {
	inputPool  *ThreadPool
	outputPool *ThreadPool
	path       string // base path of the directory to be listed
	next       internal.Component
}

type local struct {
	lister
}

func newLocalLister(path string, next internal.Component) (*local, error) {
	l := &local{
		lister: lister{
			path: path,
			next: next,
		},
	}

	l.inputPool = newThreadPool(MAX_LISTER, l.readDir)
	if l.inputPool == nil {
		log.Err("Xload::newLocalLister : fail to init thread pool")
		return l, fmt.Errorf("fail to init listing thread pool")
	}
	return l, nil
}

func (l *local) startLister() {
	l.inputPool.Start()
}

func (l *local) stopLister() {
	l.inputPool.Stop()
}

func (l *local) getInputPool() *ThreadPool {
	return l.inputPool
}

func (l *local) setOutputPool(pool *ThreadPool) {
	l.outputPool = pool
}

func (l *local) readDir(item *workItem) (int, error) {
	absPath := filepath.Join(l.path, item.path)
	if len(absPath) == 0 {
		log.Err("list::readDir : local path not given for listing")
		return 0, errors.New("local path not given for listing")
	}

	entries, err := os.ReadDir(absPath)
	if err != nil {
		log.Err("list::readDir : [%s]", err.Error())
		return 0, err
	}

	for _, entry := range entries {
		// relPath := getRelativePath(filepath.Join(absPath, entry.Name()), item.basePath)
		relPath := filepath.Join(item.path, entry.Name())

		if entry.IsDir() {
			// spawn go routine for directory creation and then
			// adding to the input channel of the listing component
			go func(name string) {
				err = l.mkdir(name)
				// TODO:: xload : handle error
				if err != nil {
					log.Err("list::readDir : Failed to create directory [%s]", err.Error())
					return
				}

				l.inputPool.Schedule(&workItem{
					path: name,
				})
			}(relPath)

		} else {
			info, err := os.Stat(filepath.Join(absPath, entry.Name()))
			if err == nil {
				// send file to the output channel for chunking
				l.outputPool.Schedule(&workItem{
					path:    relPath,
					dataLen: uint64(info.Size()),
				})
			}
		}
	}

	return len(entries), nil
}

func (l *local) mkdir(name string) error {
	// create directory in container
	return l.next.CreateDir(internal.CreateDirOptions{
		Name: name,
		Mode: 0777,
	})
}

// --------------------------------------------------------------------------------------------------------

type remote struct {
	lister
}

func (r *remote) readDir(item *workItem) (int, error) {
	return 0, nil
}

func (r *remote) mkdir(name string) error {
	return nil
}

func (r *remote) getInputPool() *ThreadPool {
	return nil
}

func (r *remote) setOutputPool(pool *ThreadPool) {

}
