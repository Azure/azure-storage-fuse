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

package common

import (
	"crypto/rand"
	"os"
	"path/filepath"
	"reflect"

	"github.com/JeffreyRichter/enum/enum"
)

// Standard config default values
const (
	DefaultMaxLogFileSize = 512
	DefaultLogFileCount   = 10
	Blobfuse2Version      = "2.0.0-preview.1"
	FileSystemName        = "blobfuse2"

	DefaultConfigFilePath = "$HOME/.blobfuse2/config.yaml"

	MaxConcurrency     = 40
	DefaultConcurrency = 20

	MaxDirListCount                             = 5000
	DefaultFilePermissionBits       os.FileMode = 0755
	DefaultDirectoryPermissionBits  os.FileMode = 0775
	DefaultAllowOtherPermissionBits os.FileMode = 0777
)

var DefaultWorkDir = "$HOME/.blobfuse2"
var DefaultLogFilePath = filepath.Join(DefaultWorkDir, "blobfuse2.log")

//LogLevel enum
type LogLevel int

var ELogLevel = LogLevel(0).INVALID()

func (LogLevel) INVALID() LogLevel {
	return LogLevel(0)
}

func (LogLevel) LOG_OFF() LogLevel {
	return LogLevel(1)
}

func (LogLevel) LOG_CRIT() LogLevel {
	return LogLevel(2)
}

func (LogLevel) LOG_ERR() LogLevel {
	return LogLevel(3)
}

func (LogLevel) LOG_WARNING() LogLevel {
	return LogLevel(4)
}

func (LogLevel) LOG_INFO() LogLevel {
	return LogLevel(5)
}

func (LogLevel) LOG_TRACE() LogLevel {
	return LogLevel(6)
}

func (LogLevel) LOG_DEBUG() LogLevel {
	return LogLevel(7)
}

func (l LogLevel) String() string {
	return enum.StringInt(l, reflect.TypeOf(l))
}

func (l *LogLevel) Parse(s string) error {
	enumVal, err := enum.ParseInt(reflect.TypeOf(l), s, true, false)
	if enumVal != nil {
		*l = enumVal.(LogLevel)
	}
	return err
}

type FileType int

var EFileType = FileType(0).File()

func (FileType) File() FileType {
	return FileType(0)
}

func (FileType) Dir() FileType {
	return FileType(1)
}

func (FileType) Symlink() FileType {
	return FileType(2)
}

func (f FileType) String() string {
	return enum.StringInt(f, reflect.TypeOf(f))
}

func (f *FileType) Parse(s string) error {
	enumVal, err := enum.ParseInt(reflect.TypeOf(f), s, true, false)
	if enumVal != nil {
		*f = enumVal.(FileType)
	}
	return err
}

type EvictionPolicy int

var EPolicy = EvictionPolicy(0).LRU()

func (EvictionPolicy) LRU() EvictionPolicy {
	return EvictionPolicy(0)
}
func (EvictionPolicy) LFU() EvictionPolicy {
	return EvictionPolicy(1)
}
func (EvictionPolicy) ARC() EvictionPolicy {
	return EvictionPolicy(2)
}

func (ep *EvictionPolicy) Parse(s string) error {
	enumVal, err := enum.ParseInt(reflect.TypeOf(ep), s, true, false)
	if enumVal != nil {
		*ep = enumVal.(EvictionPolicy)
	}
	return err
}

type LogConfig struct {
	Level       LogLevel
	FilePath    string
	MaxFileSize uint64
	FileCount   uint64
	TimeTracker bool
}

type Block struct {
	Id         string
	StartIndex int64
	EndIndex   int64
	Size       int64
}

// list that holds blocks containing ids and corresponding offsets
type BlockOffsetList []*Block

func (bol BlockOffsetList) FindBlocksToModify(offset, length int64) (BlockOffsetList, int64, bool) {
	size := int64(0)
	currentBlockOffset := offset
	var modBlockList BlockOffsetList
	// TODO: chSange this to binary search (logn) for better perf
	for _, blk := range bol {
		if currentBlockOffset >= blk.StartIndex && currentBlockOffset <= blk.EndIndex && currentBlockOffset <= offset+length {
			modBlockList = append(modBlockList, blk)
			size += blk.Size
		}
		currentBlockOffset = blk.EndIndex
	}
	return modBlockList, size, offset+length >= bol[len(bol)-1].EndIndex
}

// A UUID representation compliant with specification in RFC 4122 document.
type uuid [16]byte

// NewUUID returns a new uuid using RFC 4122 algorithm.
func NewUUID() (u uuid) {
	u = uuid{}
	// Set all bits to randomly (or pseudo-randomly) chosen values.
	rand.Read(u[:])
	u[8] = (u[8] | 0x40) & 0x7F // u.setVariant(ReservedRFC4122)

	var version byte = 4
	u[6] = (u[6] & 0xF) | (version << 4) // u.setVersion(4)
	return
}

func (u uuid) Bytes() []byte {
	return u[:]
}
