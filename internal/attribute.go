// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package internal

import (
	"os"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common"
)

func NewDirBitMap() common.BitMap64 {
	bm := common.BitMap64(0)
	bm.Set(PropFlagIsDir)
	return bm
}

func NewSymlinkBitMap() common.BitMap64 {
	bm := common.BitMap64(0)
	bm.Set(PropFlagSymlink)
	return bm
}

func NewFileBitMap() common.BitMap64 {
	bm := common.BitMap64(0)
	return bm
}

// Flags represented in common.BitMap16 for various properties of the object
const (
	PropFlagUnknown uint64 = iota
	PropFlagNotExists
	PropFlagIsDir
	PropFlagEmptyDir
	PropFlagSymlink
	PropFlagModeDefault // TODO: Does this sound better as ModeDefault or DefaultMode? The getter would be IsModeDefault or IsDefaultMode
)

// ObjAttr : Attributes of any file/directory
type ObjAttr struct {
	Mtime    time.Time          // modified time
	Atime    time.Time          // access time
	Ctime    time.Time          // change time
	Crtime   time.Time          // creation time
	Size     int64              // size of the file/directory
	Mode     os.FileMode        // permissions in 0xxx format
	Flags    common.BitMap64    // flags
	Path     string             // full path
	Name     string             // base name of the path
	MD5      []byte             // MD5 of the blob as per last GetAttr
	ETag     string             // ETag of the blob as per last GetAttr
	Metadata map[string]*string // extra information to preserve
}

// IsDir : Test blob is a directory or not
func (attr *ObjAttr) IsDir() bool {
	return attr.Flags.IsSet(PropFlagIsDir)
}

// IsSymlink : Test blob is a symlink or not
func (attr *ObjAttr) IsSymlink() bool {
	return attr.Flags.IsSet(PropFlagSymlink)
}

// IsModeDefault : Whether or not to use the default mode.
// This is set in any storage service that does not support chmod/chown.
func (attr *ObjAttr) IsModeDefault() bool {
	return attr.Flags.IsSet(PropFlagModeDefault)
}
