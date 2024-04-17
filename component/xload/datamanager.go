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

import "github.com/Azure/azure-storage-fuse/v2/internal"

// Interface to read and write data
type dataManager interface {
	ReadData(item *workItem) (int, error)
	WriteData(item *workItem) (int, error)
}

// Interface to commit the data
type dataCommitter interface {
	CommitData(name string, ids []string) error
}

// -----------------------------------------------------------------------------------

// LocalDataManager is a data manager for local data
type LocalDataManager struct {
}

// ReadData reads data from the data manager
func (l *LocalDataManager) ReadData(item *workItem) (int, error) {
	n, err := item.fileHandle.ReadAt(item.block.data, int64(item.offset))
	item.responseChannel <- workItemResp{block: item.block, err: err}
	return n, err
}

// WriteData writes data to the data manager
func (l *LocalDataManager) WriteData(item *workItem) (int, error) {
	n, err := item.fileHandle.WriteAt(item.block.data, int64(item.offset))
	item.responseChannel <- workItemResp{block: item.block, err: err}
	return n, err
}

// -----------------------------------------------------------------------------------

// RemoteDataManager is a data manager for remote data
type RemoteDataManager struct {
	remote internal.Component
}

// ReadData reads data from the data manager
func (r *RemoteDataManager) ReadData(item *workItem) (int, error) {
	n, err := r.remote.ReadInBuffer(internal.ReadInBufferOptions{
		Handle: nil,
		Name:   item.path,
		Offset: int64(item.offset),
		Data:   item.block.data,
	})

	item.responseChannel <- workItemResp{block: item.block, err: err}
	return n, err
}

// WriteData writes data to the data manager
func (r *RemoteDataManager) WriteData(item *workItem) (int, error) {
	err := r.remote.StageData(internal.StageDataOptions{
		Name:   item.path,
		Data:   item.block.data[0:item.block.length],
		Offset: uint64(item.offset),
		Id:     item.id})

	item.responseChannel <- workItemResp{block: item.block, err: err}
	return int(item.length), err
}

// CommitData commits data to the data manager
func (r *RemoteDataManager) CommitData(name string, ids []string) error {
	return r.remote.CommitData(internal.CommitDataOptions{
		Name: name,
		List: ids,
	})
}

// -----------------------------------------------------------------------------------
