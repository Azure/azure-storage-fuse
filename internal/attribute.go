/*
    _____           _____   _____   ____          ______  _____  ------
   |     |  |      |     | |     | |     |     | |       |            |
   |     |  |      |     | |     | |     |     | |       |            |
   | --- |  |      |     | |-----| |---- |     | |-----| |-----  ------
   |     |  |      |     | |     | |     |     |       | |       |
   | ____|  |_____ | ____| | ____| |     |_____|  _____| |_____  |_____


   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2020-2022 Microsoft Corporation. All rights reserved.
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

package internal

import (
	"os"
	"time"
)

// BitMap : Generic BitMap to maintain flags
type BitMap uint16

// IsSet : Check whether the given bit is set or not
func (bm BitMap) IsSet(bit uint16) bool { return (bm & (1 << bit)) != 0 }

// Set : Set the given bit in bitmap
func (bm *BitMap) Set(bit uint16) { *bm |= (1 << bit) }

// Clear : Clear the given bit from bitmap
func (bm BitMap) Clear(bit uint16) { bm &= ^(1 << bit) }

func NewDirBitMap() BitMap {
	bm := BitMap(0)
	bm.Set(PropFlagIsDir)
	return bm
}

func NewSymlinkBitMap() BitMap {
	bm := BitMap(0)
	bm.Set(PropFlagSymlink)
	return bm
}

func NewFileBitMap() BitMap {
	bm := BitMap(0)
	return bm
}

// Flags represented in BitMap for various properties of the object
const (
	PropFlagUnknown uint16 = iota
	PropFlagNotExists
	PropFlagIsDir
	PropFlagEmptyDir
	PropFlagSymlink
	PropFlagMetadataRetrieved
	PropFlagModeDefault // TODO: Does this sound better as ModeDefault or DefaultMode? The getter would be IsModeDefault or IsDefaultMode
)

// ObjAttr : Attributes of any file/directory
type ObjAttr struct {
	Path     string            // full path
	Name     string            // base name of the path
	Size     int64             // size of the file/directory
	Mode     os.FileMode       // permissions in 0xxx format
	Mtime    time.Time         // modified time
	Atime    time.Time         // access time
	Ctime    time.Time         // change time
	Crtime   time.Time         // creation time
	Flags    BitMap            // flags
	Metadata map[string]string // extra information to preseve
}

// IsDir : Test blob is a directory or not
func (attr *ObjAttr) IsDir() bool {
	return attr.Flags.IsSet(PropFlagIsDir)
}

// IsSymlink : Test blob is a symlink or not
func (attr *ObjAttr) IsSymlink() bool {
	return attr.Flags.IsSet(PropFlagSymlink)
}

// IsMetadataRetrieved : Whether or not metadata has been retrieved for this path.
// Datalake list paths does not support returning x-ms-properties (metadata), so we cannot be sure if the path is a symlink or not.
func (attr *ObjAttr) IsMetadataRetrieved() bool {
	return attr.Flags.IsSet(PropFlagMetadataRetrieved)
}

// IsModeDefault : Whether or not to use the default mode.
// This is set in any storage service that does not support chmod/chown.
func (attr *ObjAttr) IsModeDefault() bool {
	return attr.Flags.IsSet(PropFlagModeDefault)
}
