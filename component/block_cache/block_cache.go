/*
    _____           _____   _____   ____          ______  _____  ------
   |     |  |      |     | |     | |     |     | |       |            |
   |     |  |      |     | |     | |     |     | |       |            |
   | --- |  |      |     | |-----| |---- |     | |-----| |-----  ------
   |     |  |      |     | |     | |     |     |       | |       |
   | ____|  |_____ | ____| | ____| |     |_____|  _____| |_____  |_____


   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2020-2023 Microsoft Corporation. All rights reserved.
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

package block_cache

import (
	"bytes"
	"container/list"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/config"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal"
	"github.com/Azure/azure-storage-fuse/v2/internal/handlemap"
	"github.com/vibhansa-msft/tlru"
)

/* NOTES:
   - Component shall have a structure which inherits "internal.BaseComponent" to participate in pipeline
   - Component shall register a name and its constructor to participate in pipeline  (add by default by generator)
   - Order of calls : Constructor -> Configure -> Start ..... -> Stop
   - To read any new setting from config file follow the Configure method default comments
*/

// Common structure for Component
type BlockCache struct {
	internal.BaseComponent

	blockSizeMB uint32

	memSizeMB  uint32
	memTimeout uint32

	tmpPath     string
	diskSizeMB  uint32
	diskTimeout uint32

	workers  uint32
	prefetch uint32

	memPolicy  *tlru.TLRU
	diskPolicy *tlru.TLRU

	blockPool  *BlockPool
	threadPool *ThreadPool
	fileLocks  *common.LockMap

	fileNodeMap sync.Map

	maxPoolUsageHit bool
	maxDiskUsageHit bool
}

// Structure defining your config parameters
type BlockCacheOptions struct {
	BlockSize uint32 `config:"block-size-mb" yaml:"block-size-mb,omitempty"`

	MemSize    uint32 `config:"mem-size-mb" yaml:"mem-size-mb,omitempty"`
	MemTimeout uint32 `config:"mem-timeout-sec" yaml:"timeout-sec,omitempty"`

	TmpPath     string `config:"path" yaml:"path,omitempty"`
	DiskSize    uint32 `config:"disk-size-mb" yaml:"disk-size-mb,omitempty"`
	DiskTimeout uint32 `config:"disk-timeout-sec" yaml:"timeout-sec,omitempty"`

	PrefetchCount uint32 `config:"prefetch" yaml:"prefetch,omitempty"`
	Workers       uint32 `config:"parallelism" yaml:"parallelism,omitempty"`
}

// One workitem to be scheduled
type workItem struct {
	handle     *handlemap.Handle
	block      *Block
	prefetched bool
}

const (
	compName              = "block_cache"
	defaultTimeout        = 120
	MAX_POOL_USAGE uint32 = 80
	MIN_POOL_USAGE uint32 = 50
)

// Verification to check satisfaction criteria with Component Interface
var _ internal.Component = &BlockCache{}

func (bc *BlockCache) Name() string {
	return compName
}

func (bc *BlockCache) SetName(name string) {
	bc.BaseComponent.SetName(name)
}

func (bc *BlockCache) SetNextComponent(nc internal.Component) {
	bc.BaseComponent.SetNextComponent(nc)
}

// Start : Pipeline calls this method to start the component functionality
//
//	this shall not Block the call otherwise pipeline will not start
func (bc *BlockCache) Start(ctx context.Context) error {
	log.Trace("BlockCache::Start : Starting component %s", bc.Name())

	bc.threadPool.Start()

	err := bc.memPolicy.Start()
	if err != nil {
		log.Err("BlockCache::Start : failed to start mempolicy [%s]", err.Error())
		return fmt.Errorf("failed to start  mem-policy for block-cache")
	}

	err = bc.diskPolicy.Start()
	if err != nil {
		log.Err("BlockCache::Start : failed to start diskpolicy [%s]", err.Error())
		return fmt.Errorf("failed to start  disk-policy for block-cache")
	}

	bc.maxPoolUsageHit = false
	bc.maxDiskUsageHit = false

	return nil
}

// Stop : Stop the component functionality and kill all threads started
func (bc *BlockCache) Stop() error {
	log.Trace("BlockCache::Stop : Stopping component %s", bc.Name())
	bc.threadPool.Stop()

	_ = bc.memPolicy.Stop()
	_ = bc.diskPolicy.Stop()
	_ = bc.TempCacheCleanup()

	return nil
}

func (bc *BlockCache) TempCacheCleanup() error {
	log.Err("BlockCache::TempCacheCleanup : Cleaning up temp directory %s", bc.tmpPath)

	dirents, err := os.ReadDir(bc.tmpPath)
	if err != nil {
		log.Err("BlockCache::TempCacheCleanup : Failed to list directory %s [%v]", bc.tmpPath, err.Error())
		return nil
	}

	for _, entry := range dirents {
		os.RemoveAll(filepath.Join(bc.tmpPath, entry.Name()))
	}

	return nil
}

// Configure : Pipeline will call this method after constructor so that you can read config and initialize yourself
//
//	Return failure if any config is not valid to exit the process
func (bc *BlockCache) Configure(_ bool) error {
	log.Trace("BlockCache::Configure : %s", bc.Name())

	readonly := false
	err := config.UnmarshalKey("read-only", &readonly)
	if err != nil {
		log.Err("BlockCache::Configure : config error [unable to obtain read-only]")
		return fmt.Errorf("BlockCache: unable to obtain read-only")
	}

	if !readonly {
		log.Err("BlockCache::Configure : config error [filesystem is not mounted in read-only mode]")
		return fmt.Errorf("BlockCache: filesystem is not mounted in read-only mode")
	}

	// >> If you do not need any config parameters remove below code and return nil
	conf := BlockCacheOptions{}
	err = config.UnmarshalKey(bc.Name(), &conf)
	if err != nil {
		log.Err("BlockCache::Configure : config error [invalid config attributes]")
		return fmt.Errorf("BlockCache: config error [invalid config attributes]")
	}

	bc.blockSizeMB = 8
	if config.IsSet(compName + ".block-size-mb") {
		bc.blockSizeMB = conf.BlockSize

	}

	bc.memSizeMB = 4192
	if config.IsSet(compName + ".mem-size-mb") {
		bc.memSizeMB = conf.MemSize
	}
	bc.memTimeout = defaultTimeout
	if config.IsSet(compName + ".mem-timeout-sec") {
		bc.memTimeout = conf.MemTimeout
	}

	bc.diskSizeMB = 4192
	if config.IsSet(compName + ".disk-size-mb") {
		bc.diskSizeMB = conf.DiskSize
	}
	bc.diskTimeout = defaultTimeout
	if config.IsSet(compName + ".disk-timeout-sec") {
		bc.diskTimeout = conf.DiskTimeout
	}

	bc.prefetch = 8
	if config.IsSet(compName + ".prefetch") {
		bc.prefetch = conf.PrefetchCount
	}

	bc.workers = 128
	if config.IsSet(compName + ".parallelism") {
		bc.workers = conf.Workers
	}

	bc.tmpPath = common.ExpandPath(conf.TmpPath)
	if bc.tmpPath == "" {
		log.Err("BlockCache: config error [tmp-path not set]")
		return fmt.Errorf("config error in %s error [tmp-path not set]", bc.Name())
	}

	// Extract values from 'conf' and store them as you wish here
	_, err = os.Stat(bc.tmpPath)
	if os.IsNotExist(err) {
		log.Err("BlockCache: config error [tmp-path does not exist. attempting to create tmp-path.]")
		err := os.Mkdir(bc.tmpPath, os.FileMode(0755))
		if err != nil {
			log.Err("BlockCache: config error creating directory after clean [%s]", err.Error())
			return fmt.Errorf("config error in %s [%s]", bc.Name(), err.Error())
		}
	}

	log.Info("BlockCache::Configure : block size %v, mem size %v, worker %v, prefeth %v, disk path %v, max size %v, block timeout %v, disk timeout %v",
		bc.blockSizeMB, bc.memSizeMB, bc.workers, bc.prefetch, bc.tmpPath, bc.diskSizeMB, bc.memTimeout, bc.diskTimeout)

	bc.blockPool = NewBlockPool((uint64)(bc.blockSizeMB)*_1MB, (uint64)(bc.memSizeMB)*_1MB)
	if bc.blockPool == nil {
		log.Err("BlockCache::Configure : fail to init Block pool")
		return fmt.Errorf("BlockCache: failed to init Block pool")
	}

	bc.threadPool = newThreadPool(bc.workers, bc.download)
	if bc.threadPool == nil {
		log.Err("BlockCache::Configure : fail to init thread pool")
		return fmt.Errorf("BlockCache: failed to init thread pool")
	}

	bc.memPolicy, err = tlru.New(bc.memSizeMB/bc.blockSizeMB, bc.memTimeout, bc.memEvict, 60, bc.checkBlockPool)
	if err != nil {
		log.Err("BlockCache::Configure : fail to create LRU for memory nodes [%s]", err.Error())
		return fmt.Errorf("BlockCache: fail to create LRU for memory nodes")
	}

	bc.diskPolicy, err = tlru.New(bc.diskSizeMB/bc.blockSizeMB, bc.diskTimeout, bc.diskEvict, 60, bc.checkDiskUsage)
	if err != nil {
		log.Err("BlockCache::Configure : fail to create LRU for memory nodes [%s]", err.Error())
		return fmt.Errorf("BlockCache: fail to create LRU for memory nodes")
	}

	return nil
}

// OnConfigChange : When config file is changed, this will be called by pipeline. Refresh required config here
func (bc *BlockCache) OnConfigChange() {
	log.Trace("AzStorage::OnConfigChange : %s", bc.Name())

	// >> If you do not need any config parameters remove below code and return nil
	conf := BlockCacheOptions{}
	err := config.UnmarshalKey(bc.Name(), &conf)
	if err != nil {
		log.Err("BlockCache::OnConfigChange : config error [invalid config attributes]")
		return
	}

	if conf.BlockSize == 0 || conf.MemSize == 0 || conf.Workers == 0 {
		log.Err("BlockCache::OnConfigChange : Invalid config attributes. block size %v, mem size %v, worker %v, prefeth %v",
			conf.BlockSize, conf.MemSize, conf.Workers, conf.PrefetchCount)
		return
	}

	bc.blockPool.ReSize((uint64)(conf.BlockSize)*_1MB, (uint64)(conf.MemSize)*_1MB)
	bc.memSizeMB = conf.MemSize
}

// OpenFile: Makes the file available in the local cache for further file operations.
func (bc *BlockCache) OpenFile(options internal.OpenFileOptions) (*handlemap.Handle, error) {
	log.Trace("BlockCache::OpenFile : name=%s, flags=%d, mode=%s", options.Name, options.Flags, options.Mode)

	attr, err := bc.NextComponent().GetAttr(internal.GetAttrOptions{Name: options.Name})
	if err != nil {
		log.Err("BlockCache::OpenFile : Failed to get attr of %s [%s]", options.Name, err.Error())
		return nil, err
	}

	handle := handlemap.NewHandle(options.Name)
	handle.Size = attr.Size

	// Schedule the download of first N blocks for this file here
	nextoffset := uint64(0)
	prefetch := bc.blockPool.Available(bc.prefetch)
	for i := 0; uint32(i) < prefetch && int64(nextoffset) < handle.Size; i++ {
		success := bc.lineupDownload(handle, nextoffset, (i == 0))
		if !success {
			break
		}
		nextoffset += bc.blockPool.blockSize
	}

	return handle, nil
}

// ReadInBuffer: Read the local file into a buffer
func (bc *BlockCache) ReadInBuffer(options internal.ReadInBufferOptions) (int, error) {
	// The file should already be in the cache since CreateFile/OpenFile was called before and a shared lock was acquired.

	dataRead := int(0)
	for dataRead < len(options.Data) {

		node, err := bc.getBlock(options.Handle, uint64(options.Offset))
		if err != nil {
			if err != io.EOF {
				return 0, fmt.Errorf("BlockCache::ReadInBuffer : Failed to get the Block %s # %v [%v]", options.Handle.Path, options.Offset, err.Error())
			} else {
				return dataRead, err
			}
		}

		if node == nil {
			return dataRead, fmt.Errorf("BlockCache::ReadInBuffer : Failed to retrieve block %s # %v", options.Handle.Path, options.Offset)
		}

		block := node.Value.(*workItem).block
		readOffset := uint64(options.Offset) - (block.id * bc.blockPool.blockSize)
		bytesRead := copy(options.Data[dataRead:], block.data[readOffset:])

		options.Offset += int64(bytesRead)
		dataRead += bytesRead
	}

	return dataRead, nil
}

// CloseFile: Flush the file and invalidate it from the cache.
func (bc *BlockCache) CloseFile(options internal.CloseFileOptions) error {
	log.Trace("BlockCache::CloseFile : name=%s, handle=%d", options.Handle.Path, options.Handle.ID)

	options.Handle.CleanupWithCallback(func(key string, item interface{}) {
		if key != "#" {
			bc.memPolicy.Remove(item.(*list.Element))
		}
	})

	return nil
}

// download : Method to download the given amount of data
func (bc *BlockCache) lineupDownload(handle *handlemap.Handle, offset uint64, wait bool) bool {
	item := &workItem{
		handle:     handle,
		block:      bc.blockPool.Get(wait),
		prefetched: !wait,
	}

	if item.block == nil {
		log.Err("BlockCache::lineupDownload : Failed to schedule prefetch of %s # %v, block: %v", handle.Path, offset, wait)
		return false
	}

	item.block.id = offset / bc.blockPool.blockSize

	node := bc.memPolicy.Add(item)
	if node == nil {
		log.Err("BlockCache::lineupDownload : Failed to push item to policy %s # %v, block: %v", handle.Path, offset, wait)
		return false
	}

	bc.threadPool.Schedule(offset == 0, item)

	return true
}

// download : Method to download the given amount of data
func (bc *BlockCache) download(i interface{}) {
	item := i.(*workItem)
	offset := int64(item.block.id * uint64(bc.blockSizeMB))
	fileName := fmt.Sprintf("%s::%v", item.handle.Path, offset)
	diskUsed := false

	flock := bc.fileLocks.Get(fileName)
	flock.Lock()
	defer flock.Unlock()

	diskNode, found := bc.fileNodeMap.Load(fileName)
	if !found {
		diskNode = bc.diskPolicy.Add(fileName)
		bc.fileNodeMap.Store(fileName, diskNode)
	}

	localPath := filepath.Join(bc.tmpPath, fileName)
	_, err := os.Stat(localPath)
	if err == nil {
		// File exists locally so read from there into buffer
		f, err := os.Open(localPath)
		if err == nil {
			log.Info("BlockCache::download : Reading data from disk cache %s", fileName)
			_, err = f.Read(item.block.data)
			if err != nil {
				log.Err("BlockCache::download : Failed to read data from disk cache %s [%s]", fileName, err.Error())
			} else {
				diskUsed = true
			}
		}
		f.Close()
		bc.diskPolicy.Refresh(diskNode.(*list.Element))
	}

	if err != nil {
		log.Info("BlockCache::download : Reading data from network %s", fileName)
		n, err := bc.NextComponent().ReadInBuffer(internal.ReadInBufferOptions{
			Handle: item.handle,
			Offset: int64(item.block.id * bc.blockPool.blockSize),
			Data:   item.block.data,
		})

		if err != nil {
			// Fail to read the data so just reschedule this request
			log.Err("BlockCache::download : Failed to read %s from offset %v [%s]", item.handle.Path, item.block.id, err.Error())
			bc.threadPool.Schedule(false, item)
			return
		} else if n == 0 {
			// No data read so just reschedule this request
			log.Err("BlockCache::download : Failed to read %s from offset %v [0 bytes read]", item.handle.Path, item.block.id)
			bc.threadPool.Schedule(false, item)
			return
		}
	}

	// Unblock readers of this Block
	_ = item.block.ReadyForReading()

	if !diskUsed {
		// Dump this buffer to local file
		f, err := os.Create(localPath)
		if err == nil {
			_, err := f.Write(item.block.data)
			if err != nil {
				log.Err("BlockCache::download : Failed to write %s to disk [%v]]", localPath, err.Error())
				f.Close()
				_ = os.Remove(localPath)
			} else {
				f.Close()
				// Validate cache
				bc.diskPolicy.Refresh(diskNode.(*list.Element))
			}
		}
	}
}

// getBlockIndex: From offset get the block index
func (bc *BlockCache) getBlockIndex(offset uint64) uint64 {
	return offset / bc.blockPool.blockSize
}

// getBlock: From offset generate the Block index and get the Block
func (bc *BlockCache) getBlock(handle *handlemap.Handle, readoffset uint64) (*list.Element, error) {
	if readoffset >= uint64(handle.Size) {
		return nil, io.EOF
	}

	index := bc.getBlockIndex(readoffset)
	node, found := handle.GetValue(fmt.Sprintf("%v", index))

	if !found {
		// This offset is not cached yet, so lineup the download
		success := bc.lineupDownload(handle, readoffset, true)
		if !success {
			return nil, fmt.Errorf("failed to schedule download")
		}

		node, found = handle.GetValue(fmt.Sprintf("%v", index))
		if !found {
			log.Err("BlockCache::ReadInBuffer : Something went wrong not able to find the Block %s # %v", handle.Path, index)
			return nil, fmt.Errorf("not able to find block immediately after scheudling")
		}
	}

	item := node.(*list.Element)
	block := item.Value.(*workItem).block

	// Validate cache
	bc.memPolicy.Refresh(item)

	// Wait for this block to complete the download
	t := int(0)
	t = <-block.state

	if t == 2 {
		// block is ready and we are the second reader so its time to schedule the next block
		lastoffset, found := handle.GetValue("#")
		if found && lastoffset.(uint64) < uint64(handle.Size) {
			if bc.blockPool.Available(1) > 0 {
				_ = bc.lineupDownload(handle, lastoffset.(uint64), false)
			}
		}
	} else if t == 1 {
		// block is ready and we are the first reader so its time to remove the second last block from here
		if block.id >= 2 {
			delId := block.id - 2
			delItem, delFound := handle.GetValue(fmt.Sprintf("%v", delId))
			if delFound {
				handle.RemoveValue(fmt.Sprintf("%v", delId))
				bc.memPolicy.Remove(delItem.(*list.Element))
			}
		}
	}

	return item, nil
}

// memEvict: Callback when a node from memory expires
func (bc *BlockCache) memEvict(node *list.Element) {
	item := node.Value.(*workItem)
	item.handle.RemoveValue(fmt.Sprintf("%v", item.block.id))
	bc.blockPool.Release(item.block)
}

// diskEvict: Callback when a node from disk expires
func (bc *BlockCache) diskEvict(node *list.Element) {
	fileName := node.Value.(string)
	flock := bc.fileLocks.Get(fileName)
	flock.Lock()
	defer flock.Unlock()

	localPath := filepath.Join(bc.tmpPath, fileName)
	err := os.Remove(localPath)
	if err != nil {
		log.Err("blockCache::diskEvict : Failed to remove %s [%s]", localPath, err.Error())
	}
}

// checkBlockPool: Callback to check usage of block pool and decide whether eviction is needed
func (bc *BlockCache) checkBlockPool() bool {
	usage := bc.blockPool.Usage()

	if bc.maxPoolUsageHit {
		if usage > MIN_POOL_USAGE {
			return true
		}
		bc.maxPoolUsageHit = false
	} else {
		if bc.blockPool.Usage() > MAX_POOL_USAGE {
			bc.maxPoolUsageHit = true
			return true
		}
	}

	return false
}

// checkDiskUsage: Callback to check usage of disk and decide whether eviction is needed
func (bc *BlockCache) checkDiskUsage() bool {
	data := getUsage(bc.tmpPath)
	usage := uint32((data * 100) / float64(bc.diskSizeMB))

	if bc.maxDiskUsageHit {
		if usage > MIN_POOL_USAGE {
			return true
		}
		bc.maxDiskUsageHit = false
	} else {
		if bc.blockPool.Usage() > MAX_POOL_USAGE {
			bc.maxDiskUsageHit = true
			return true
		}
	}

	return false
}

func getUsage(path string) float64 {
	log.Trace("cachePolicy::getCacheUsage : %s", path)

	var currSize float64
	var out bytes.Buffer

	// du - estimates file space usage
	// https://man7.org/linux/man-pages/man1/du.1.html
	// Note: We cannot just pass -BM as a parameter here since it will result in less accurate estimates of the size of the path
	// (i.e. du will round up to 1M if the path is smaller than 1M).
	cmd := exec.Command("du", "-sh", path)
	cmd.Stdout = &out

	err := cmd.Run()
	if err != nil {
		log.Err("BlockCache::getCacheUsage : error running du [%s]", err.Error())
		return 0
	}

	size := strings.Split(out.String(), "\t")[0]
	if size == "0" {
		return 0
	}
	// some OS's use "," instead of "." that will not work for float parsing - replace it
	size = strings.Replace(size, ",", ".", 1)
	parsed, err := strconv.ParseFloat(size[:len(size)-1], 64)
	if err != nil {
		log.Err("BlockCache::getCacheUsage : error parsing folder size [%s]", err.Error())
		return 0
	}

	switch size[len(size)-1] {
	case 'K':
		currSize = parsed / float64(1024)
	case 'M':
		currSize = parsed
	case 'G':
		currSize = parsed * 1024
	case 'T':
		currSize = parsed * 1024 * 1024
	}

	log.Debug("BlockCache::getCacheUsage : current cache usage : %fMB", currSize)
	return currSize
}

// ------------------------- Factory -------------------------------------------

// Pipeline will call this method to create your object, initialize your variables here
// << DO NOT DELETE ANY AUTO GENERATED CODE HERE >>
func NewBlockCacheComponent() internal.Component {
	comp := &BlockCache{
		fileLocks: common.NewLockMap(),
	}
	comp.SetName(compName)
	config.AddConfigChangeEventListener(comp)
	return comp
}

// On init register this component to pipeline and supply your constructor
func init() {
	internal.AddComponent(compName, NewBlockCacheComponent)
}
