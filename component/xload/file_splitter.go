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
	"fmt"

	"github.com/Azure/azure-storage-fuse/v2/common/log"
)

type fileSpiltter struct {
	inputPool  *ThreadPool
	outputPool *ThreadPool
}

func newFileSpiltter() (*fileSpiltter, error) {
	fs := &fileSpiltter{}

	fs.inputPool = newThreadPool(128, fs.chunk)
	if fs.inputPool == nil {
		log.Err("Xload::newFileSpiltter : fail to init file splitter pool")
		return fs, fmt.Errorf("fail to init file splitter thread pool")
	}
	return fs, nil
}

func (fs *fileSpiltter) chunk(item *workItem) (int, error) {
	log.Debug("FileSplitter::chunk : %s , %s", item.basePath, item.name)
	fmt.Printf("%s , %s\n", item.basePath, item.name)
	return 0, nil
}
