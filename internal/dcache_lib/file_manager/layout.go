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

type fileLayout struct {
	fileName        string
	fileId          string // UUID to represent the file inside Dcache
	size            int64
	state           fileState
	openCount       int      // Number of Read Fd's present for this file accross the dCache
	clustermapEpoch int      // Clustermap Epoch value when the file was created.
	hash            string   // Hash of the entire file data.
	mvlist          []string // todo: this should be replaced with type of mv.
}

var _ fileLayoutMgr = &fileLayout{}

// Create Placeholder file to prevent the creation from the other nodes
func (*fileLayout) CreateNewFileLayout(fileName string) *fileLayout {
	return nil
}

// Update the fileSize and its state and update the .md file.
// The Call comes when the close call comes to write FD.
func (*fileLayout) ConfirmFileLayout(file *fileLayout, size int64) error {
	return nil
}

// Delete Filelayout incase of write/close failure.(remove .md file)
func (*fileLayout) DeleteFileLayout(file *fileLayout) error {
	return nil
}

// Check File is present in Dcache, return filelayout if it's present
func (*fileLayout) CheckFileInDCache(fileName string) (isPresent bool, f *fileLayout) {
	return false, nil
}

// Check File is present in Azure.
func (*fileLayout) CheckFileInAzure(fileName string) (isPresent bool) {
	return false
}

// Increments File read FD count in .md file.
func (*fileLayout) IncrementFDCount(file *fileLayout) error {
	return errors.New("todo: Implement with unlink/remove")
}

// Decrements File read FD count in .md file.
func (*fileLayout) DecrementFDCount(file *fileLayout) error {
	return errors.New("todo: Implement with unlink/remove")
}
