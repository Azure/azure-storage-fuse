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

package file_manager

import "errors"

type fileState int

const (
	Ready fileState = iota
	Writing
	Deleting
)

type FileLayout struct {
	FileName        string    `json:"fileName,omitempty"`
	FileId          string    `json:"fileId,omitempty"`
	Size            int64     `json:"size,omitempty"`
	State           fileState `json:"state,omitempty"`
	OpenCount       int       `json:"open-count,omitempty"`
	ClustermapEpoch int       `json:"clustermap-epoch,omitempty"`
	Hash            string    `json:"sha1hash,omitempty"`
	ChunkSize       int       `json:"chunk-size,omitempty"`
	StripeSize      int       `json:"stripe-size,omitempty"`
	Mvlist          []string  `json:"mv-list,omitempty"`
}

var _ IFileLayoutMgr = &FileLayoutMgr{}

type FileLayoutMgr struct{}

// Check file in Dcache, fail the call if it's already present.
// Also check the file in Azure, If necessary
// Choose appropriate mv's
// Create the place holder .md file in Azure to prevent other nodes to write to same file.
func (FileLayoutMgr) CreateNewFileLayout(fileName string) (*FileLayout, error) {
	return nil, nil
}

// Update the fileSize and its state and update the .md file.
// The Call comes when the close call comes to write FD.
func (FileLayoutMgr) ConfirmFileLayout(file *FileLayout, size int64) error {
	return nil
}

// Delete Filelayout incase of write/close failure for writeFD.(remove .md file)
func (FileLayoutMgr) DeleteFileLayout(file *FileLayout) error {
	return nil
}

// Check File is present in Dcache, return filelayout if it's already present
func (FileLayoutMgr) CheckFileInDCache(fileName string) (isPresent bool, f *FileLayout) {
	return false, nil
}

// Check If File is present in Azure.
func (FileLayoutMgr) CheckFileInAzure(fileName string) (isPresent bool) {
	return false
}

// Increments File read FD count in .md file.
func (FileLayoutMgr) IncrementFDCount(file *FileLayout) error {
	return errors.New("todo: Implement with unlink/remove")
}

// Decrements File read FD count in .md file.
func (FileLayoutMgr) DecrementFDCount(file *FileLayout) error {
	return errors.New("todo: Implement with unlink/remove")
}
