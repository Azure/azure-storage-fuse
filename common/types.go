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

package common

import (
	"crypto/rand"
	"encoding/base64"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"time"

	"github.com/JeffreyRichter/enum/enum"
)

// Standard config default values
const (
	blobfuse2Version_ = "2.5.0~preview.1"

	DefaultMaxLogFileSize = 512
	DefaultLogFileCount   = 10
	FileSystemName        = "blobfuse2"

	DefaultConfigFilePath = "config.yaml"

	MaxConcurrency     = 40
	DefaultConcurrency = 20

	MaxDirListCount                             = 5000
	DefaultFilePermissionBits       os.FileMode = 0755
	DefaultDirectoryPermissionBits  os.FileMode = 0775
	DefaultAllowOtherPermissionBits os.FileMode = 0777

	MbToBytes     = 1024 * 1024
	GbToBytes     = 1024 * MbToBytes
	BfuseStats    = "blobfuse_stats"
	BlockIDLength = 16

	// File system block size
	FS_BLOCK_SIZE = 4096

	FuseAllowedFlags = "invalid FUSE options. Allowed FUSE configurations are: `-o attr_timeout=TIMEOUT`, `-o negative_timeout=TIMEOUT`, `-o entry_timeout=TIMEOUT` `-o allow_other`, `-o allow_root`, `-o umask=PERMISSIONS -o default_permissions`, `-o ro`"

	UserAgentHeader = "User-Agent"

	BlockCacheRWErrMsg = "Notice: The random write flow using block cache is temporarily blocked due to potential data integrity issues. This is a precautionary measure. \nIf you see this message, contact blobfusedev@microsoft.com or create a GitHub issue. We're working on a fix. More details: https://aka.ms/blobfuse2warnings."
)

func FuseIgnoredFlags() []string {
	return []string{"default_permissions", "rw", "dev", "nodev", "suid", "nosuid", "delay_connect", "auto", "noauto", "user", "nouser", "exec", "noexec"}
}

var Blobfuse2Version = Blobfuse2Version_()

func Blobfuse2Version_() string {
	return blobfuse2Version_
}

var DefaultWorkDir = "$HOME/.blobfuse2"
var DefaultLogFilePath = filepath.Join(DefaultWorkDir, "blobfuse2.log")
var StatsConfigFilePath = filepath.Join(DefaultWorkDir, "stats_monitor.cfg")

var EnableMonitoring = false
var BfsDisabled = false
var TransferPipe = "/tmp/transferPipe"
var PollingPipe = "/tmp/pollPipe"

var MountPath string

// LogLevel enum
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

type LogConfig struct {
	Level       LogLevel
	MaxFileSize uint64
	FileCount   uint64
	FilePath    string
	TimeTracker bool
	Tag         string // logging tag which can be either blobfuse2 or bfusemon
}

// Flags for blocks
const (
	BlockFlagUnknown uint16 = iota
	DirtyBlock
	TruncatedBlock
	RemovedBlocks
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

func (block *Block) Removed() bool {
	return block.Flags.IsSet(RemovedBlocks)
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
	Size          int64
	Mtime         time.Time
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

// returns block id of given length
func GetBlockID(len int64) string {
	return base64.StdEncoding.EncodeToString(NewUUIDWithLength(len))
}

func GetIdLength(id string) int64 {
	existingBlockId, _ := base64.StdEncoding.DecodeString(id)
	return int64(len(existingBlockId))
}

// Align up the size of the file to the file system block size
// This is required when doing direct IO operations.
func AlignToBlockSize(size int64) int64 {
	Assert(size >= 0, size)

	if size%FS_BLOCK_SIZE == 0 {
		return size
	}

	return ((size / FS_BLOCK_SIZE) + 1) * FS_BLOCK_SIZE
}

func init() {
	val, present := os.LookupEnv("HOME")
	if !present {
		val = "./"
	}
	DefaultWorkDir = filepath.Join(val, ".blobfuse2")
	DefaultLogFilePath = filepath.Join(DefaultWorkDir, "blobfuse2.log")
	StatsConfigFilePath = filepath.Join(DefaultWorkDir, "stats_monitor.cfg")
}

var azureSpecialContainers = map[string]bool{
	"web":        true,
	"logs":       true,
	"changefeed": true,
}
