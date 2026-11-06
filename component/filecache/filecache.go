/*
    _____           _____   _____   ____          ______  _____  ------
   |     |  |      |     | |     | |     |     | |       |            |
   |     |  |      |     | |     | |     |     | |       |            |
   | --- |  |      |     | |-----| |---- |     | |-----| |-----  ------
   |     |  |      |     | |     | |     |     |       | |       |
   | ____|  |_____ | ____| | ____| |     |_____|  _____| |_____  |_____


   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2020-2026 Microsoft Corporation. All rights reserved.
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

package filecache

import (
	"errors"

	"github.com/Azure/azure-storage-fuse/v2/internal"
)

// compName is the name of the component. Both engines register under it, and a mount uses one of them.
const compName = "file_cache"

// errIncomplete fails the configuration while the engine is being written, so that it cannot be used by mistake.
var errIncomplete = errors.New("the engine is not complete yet")

// FileCache is engine v2 of the file_cache component.
type FileCache struct {
	internal.BaseComponent
}

var _ internal.Component = &FileCache{}

// New returns an engine that still needs to be configured with Configure.
func New() *FileCache {
	fc := &FileCache{}
	fc.SetName(compName)
	return fc
}

// Priority places the engine where engine v1 is in the pipeline (COMP-01).
func (fc *FileCache) Priority() internal.ComponentPriority {
	return internal.EComponentPriority.LevelMid()
}

// Configure fails if the engine does not support the config (CheckSupported).
func (fc *FileCache) Configure(_ bool) error {
	return CheckSupported()
}

// CheckSupported returns an error naming a setting of the config that the engine does not support, or nil if it
// supports them all. Settings with invalid values are not reported here, but by Configure.
func CheckSupported() error {
	return errIncomplete
}
