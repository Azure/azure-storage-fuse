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
	"encoding/base64"
	"os"
	"path/filepath"
	"reflect"
	"sync"

	"github.com/JeffreyRichter/enum/enum"
)

// Standard config default values
const (
	DefaultMaxLogFileSize = 512
	DefaultLogFileCount   = 10
	Blobfuse2Version      = "2.0.0-preview.2"
	FileSystemName        = "blobfuse2"

	DefaultConfigFilePath = "config.yaml"

	MaxConcurrency     = 40
	DefaultConcurrency = 20

	MaxDirListCount                             = 5000
	DefaultFilePermissionBits       os.FileMode = 0755
	DefaultDirectoryPermissionBits  os.FileMode = 0775
	DefaultAllowOtherPermissionBits os.FileMode = 0777
)

var DefaultWorkDir = "$HOME/.blobfuse2"
var DefaultLogFilePath = filepath.Join(DefaultWorkDir, "blobfuse2.log")
var StatsConfigFilePath = filepath.Join(DefaultWorkDir, "stats_monitor.cfg")
var DefaultPipeline = []string{"libfuse", "file_cache", "attr_cache", "azstorage"}
var DefaultStreamPipeline = []string{"libfuse", "stream", "attr_cache", "azstorage"}

var EnableMonitoring = false

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
	MaxFileSize uint64
	FileCount   uint64
	FilePath    string
	TimeTracker bool
	Tag         string // logging tag which can be either blobfuse2 or healthmon
}

// Flags for blocks
const (
	BlockFlagUnknown uint16 = iota
	DirtyBlock
	TruncatedBlock
)

type Block struct {
	sync.RWMutex
	StartIndex int64
	EndIndex   int64
	Flags      BitMap16
	Id         string
	Data       []byte
}

// Dirty : Handle is dirty or not
func (block *Block) Dirty() bool {
	return block.Flags.IsSet(DirtyBlock)
}

// Truncated : block created on a truncate operation
func (block *Block) Truncated() bool {
	return block.Flags.IsSet(TruncatedBlock)
}

// Flags for block offset list
const (
	BolFlagUnknown uint16 = iota
	SmallFile
)

// list that holds blocks containing ids and corresponding offsets
type BlockOffsetList struct {
	BlockList     []*Block //blockId to offset mapping
	Flags         BitMap16
	BlockIdLength int64
}

// Dirty : Handle is dirty or not
func (bol *BlockOffsetList) SmallFile() bool {
	return bol.Flags.IsSet(SmallFile)
}

// return true if item found and index of the item
func (bol BlockOffsetList) BinarySearch(offset int64) (bool, int) {
	lowerBound := 0
	size := len(bol.BlockList)
	higherBound := size - 1
	for lowerBound <= higherBound {
		middleIndex := (lowerBound + higherBound) / 2
		// we found the starting block that changes are being applied to
		if bol.BlockList[middleIndex].EndIndex > offset && bol.BlockList[middleIndex].StartIndex <= offset {
			return true, middleIndex
			// if the end index is smaller or equal then we need to increase our lower bound
		} else if bol.BlockList[middleIndex].EndIndex <= offset {
			lowerBound = middleIndex + 1
			// if the start index is larger than the offset we need to decrease our upper bound
		} else if bol.BlockList[middleIndex].StartIndex > offset {
			higherBound = middleIndex - 1
		}
	}
	// return size as this would be where the new blocks start
	return false, size
}

// returns index of first mod block, size of mod data, does the new data exceed current size?, is it append only?
func (bol BlockOffsetList) FindBlocks(offset, length int64) ([]*Block, bool) {
	// size of mod block list
	currentBlockOffset := offset
	var blocks []*Block
	found, index := bol.BinarySearch(offset)
	if !found {
		return blocks, false
	}
	for _, blk := range bol.BlockList[index:] {
		if blk.StartIndex > offset+length {
			break
		}
		if currentBlockOffset >= blk.StartIndex && currentBlockOffset < blk.EndIndex && currentBlockOffset <= offset+length {
			blocks = append(blocks, blk)
			currentBlockOffset = blk.EndIndex
		}
	}
	return blocks, true
}

// returns index of first mod block, size of mod data, does the new data exceed current size?, is it append only?
func (bol BlockOffsetList) FindBlocksToModify(offset, length int64) (int, int64, bool, bool) {
	// size of mod block list
	size := int64(0)
	appendOnly := true
	currentBlockOffset := offset
	found, index := bol.BinarySearch(offset)
	if !found {
		return index, 0, true, appendOnly
	}
	// after the binary search just iterate to find the remaining blocks
	for _, blk := range bol.BlockList[index:] {
		if blk.StartIndex > offset+length {
			break
		}
		if currentBlockOffset >= blk.StartIndex && currentBlockOffset < blk.EndIndex && currentBlockOffset <= offset+length {
			appendOnly = false
			blk.Flags.Set(DirtyBlock)
			currentBlockOffset = blk.EndIndex
			size += (blk.EndIndex - blk.StartIndex)
		}
	}

	return index, size, offset+length >= bol.BlockList[len(bol.BlockList)-1].EndIndex, appendOnly
}

// A UUID representation compliant with specification in RFC 4122 document.
type uuid [16]byte

const reservedRFC4122 byte = 0x40

func (u uuid) Bytes() []byte {
	return u[:]
}

// NewUUIDWithLength returns a new uuid using RFC 4122 algorithm with the given length.
func NewUUIDWithLength(length int64) []byte {
	u := make([]byte, length)
	// Set all bits to randomly (or pseudo-randomly) chosen values.
	_, err := rand.Read(u[:])
	if err == nil {
		u[8] = (u[8] | 0x40) & 0x7F // u.setVariant(ReservedRFC4122)
		var version byte = 4
		u[6] = (u[6] & 0xF) | (version << 4) // u.setVersion(4)
	}
	return u[:]
}

// NewUUID returns a new uuid using RFC 4122 algorithm.
func NewUUID() (u uuid) {
	u = uuid{}
	// Set all bits to randomly (or pseudo-randomly) chosen values.
	_, err := rand.Read(u[:])
	if err == nil {
		u[8] = (u[8] | reservedRFC4122) & 0x7F // u.setVariant(ReservedRFC4122)
		var version byte = 4
		u[6] = (u[6] & 0xF) | (version << 4) // u.setVersion(4)
	}
	return
}

func GetIdLength(id string) int64 {
	existingBlockId, _ := base64.StdEncoding.DecodeString(id)
	return int64(len(existingBlockId))
}
