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

	blockSizeMB uint64
	memSizeMB   uint64

	tmpPath     string
	diskSizeMB  uint64
	diskTimeout uint32

	workers  uint32
	prefetch uint32

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
	BlockSize uint64 `config:"block-size-mb" yaml:"block-size-mb,omitempty"`

	MemSize    uint64 `config:"mem-size-mb" yaml:"mem-size-mb,omitempty"`
	MemTimeout uint32 `config:"mem-timeout-sec" yaml:"timeout-sec,omitempty"`

	TmpPath     string `config:"path" yaml:"path,omitempty"`
	DiskSize    uint64 `config:"disk-size-mb" yaml:"disk-size-mb,omitempty"`
	DiskTimeout uint32 `config:"disk-timeout-sec" yaml:"timeout-sec,omitempty"`

	PrefetchCount uint32 `config:"prefetch" yaml:"prefetch,omitempty"`
	Workers       uint32 `config:"parallelism" yaml:"parallelism,omitempty"`
}

// One workitem to be scheduled
type workItem struct {
	handle   *handlemap.Handle
	block    *Block
	prefetch bool
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

	err := bc.diskPolicy.Start()
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

	bc.blockSizeMB = uint64(8)
	if config.IsSet(compName + ".block-size-mb") {
		bc.blockSizeMB = conf.BlockSize

	}

	bc.memSizeMB = uint64(4192)
	if config.IsSet(compName + ".mem-size-mb") {
		bc.memSizeMB = conf.MemSize
	}

	bc.diskSizeMB = uint64(4192)
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

	log.Info("BlockCache::Configure : block size %v, mem size %v, worker %v, prefeth %v, disk path %v, max size %v, disk timeout %v",
		bc.blockSizeMB, bc.memSizeMB, bc.workers, bc.prefetch, bc.tmpPath, bc.diskSizeMB, bc.diskTimeout)

	bc.blockPool = NewBlockPool(bc.blockSizeMB*_1MB, bc.memSizeMB*_1MB)
	if bc.blockPool == nil {
		log.Err("BlockCache::Configure : fail to init Block pool")
		return fmt.Errorf("BlockCache: failed to init Block pool")
	}

	bc.threadPool = newThreadPool(bc.workers, bc.download)
	if bc.threadPool == nil {
		log.Err("BlockCache::Configure : fail to init thread pool")
		return fmt.Errorf("BlockCache: failed to init thread pool")
	}

	bc.diskPolicy, err = tlru.New(uint32(bc.diskSizeMB/bc.blockSizeMB), bc.diskTimeout, bc.diskEvict, 60, bc.checkDiskUsage)
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
	handle.Mtime = attr.Mtime

	// Set next offset to download to 0
	handle.SetValue("#", (uint64)(0))

	// Allocate a block pool object for this handle
	// Acutal linked list to hold the nodes
	blockList := list.New()
	handle.TempObj = blockList

	// Fill this local block pool with prefetch number of blocks
	for i := uint32(0); i < bc.prefetch; i++ {
		blockList.PushFront(bc.blockPool.MustGet())
	}

	return handle, nil
}

// CloseFile: Flush the file and invalidate it from the cache.
func (bc *BlockCache) CloseFile(options internal.CloseFileOptions) error {
	log.Trace("BlockCache::CloseFile : name=%s, handle=%d", options.Handle.Path, options.Handle.ID)

	i := uint32(0)

	// Release the blocks that are in use and wipe out handle map
	options.Handle.Cleanup()

	// Relese the blocks that are not in use
	blockList := options.Handle.TempObj.(*list.List)
	for ; i < bc.prefetch; i++ {
		node := blockList.Back()
		block := blockList.Remove(node).(*Block)

		block.ReUse()
		bc.blockPool.Release(block)
	}

	return nil
}

// ReadInBuffer: Read the local file into a buffer
func (bc *BlockCache) ReadInBuffer(options internal.ReadInBufferOptions) (int, error) {
	// The file should already be in the cache since CreateFile/OpenFile was called before and a shared lock was acquired.

	dataRead := int(0)
	for dataRead < len(options.Data) {
		block, err := bc.getBlock(options.Handle, uint64(options.Offset))
		if err != nil {
			if err != io.EOF {
				return 0, fmt.Errorf("BlockCache::ReadInBuffer : Failed to get the Block %s # %v [%v]", options.Handle.Path, options.Offset, err.Error())
			} else {
				return dataRead, err
			}
		}

		if block == nil {
			return dataRead, fmt.Errorf("BlockCache::ReadInBuffer : Failed to retrieve block %s # %v", options.Handle.Path, options.Offset)
		}

		readOffset := uint64(options.Offset) - block.offset
		bytesRead := copy(options.Data[dataRead:], block.data[readOffset:])

		options.Offset += int64(bytesRead)
		dataRead += bytesRead
	}

	options.Handle.OptCnt++
	return dataRead, nil
}

// getBlock: From offset generate the Block index and get the Block
func (bc *BlockCache) getBlock(handle *handlemap.Handle, readoffset uint64) (*Block, error) {
	if readoffset >= uint64(handle.Size) {
		return nil, io.EOF
	}

	index := bc.getBlockIndex(readoffset)

	handle.Lock()
	node, found := handle.GetValue(fmt.Sprintf("%v", index))
	if !found {
		// If this is the first read request then prefetch all required nodes
		val, _ := handle.GetValue("#")
		if val.(uint64) == 0 {
			// This is the first read for this file handle so start prefetching all the nodes
			err := bc.startPrefetch(handle, index)
			if err != nil {
				log.Err(err.Error())
				log.Err("BlockCache::getBlock : Unable to start prefetch %s # %v", handle.Path, readoffset)
				return nil, fmt.Errorf("unable to start prefetch for this handle")
			}
		} else {
			// This block is not present even after prefetch so lets download it now
			bc.prepareBlock(handle, index, false)
		}

		node, found = handle.GetValue(fmt.Sprintf("%v", index))
		if !found {
			log.Err("BlockCache::ReadInBuffer : Something went wrong not able to find the Block %s # %v", handle.Path, index)
			return nil, fmt.Errorf("not able to find block immediately after scheudling")
		}
	}

	handle.Unlock()

	block := node.(*Block)

	// Wait for this block to complete the download
	t := int(0)
	t = <-block.state

	if t == 1 {
		// block is ready and we are the first reader so its time to remove the second last block from here
		firstBlock := handle.TempObj.(*list.List).Front().Value.(*Block)
		diff := (block.id - firstBlock.id)
		if diff >= 2 {
			handle.TempObj.(*list.List).Front().Value.(*Block).stage = BlockReady
			val, _ := handle.GetValue("#")
			bc.prepareBlock(handle, val.(uint64)+1, true)
		}

		block.Unblock()
	}

	return block, nil
}

// getBlockIndex: From offset get the block index
func (bc *BlockCache) getBlockIndex(offset uint64) uint64 {
	return offset / uint64(bc.blockSizeMB)
}

// prepareBlock: Get a block from teh list and prepare it for prefetch
func (bc *BlockCache) prepareBlock(handle *handlemap.Handle, index uint64, prefetch bool) error {
	offset := index * bc.blockSizeMB * _1MB
	nodeList := handle.TempObj.(*list.List)

	node := nodeList.Front()
	block := node.Value.(*Block)

	if block.stage == BlockReady {
		block.ReUse()

		block.id = index
		block.offset = offset

		bc.lineupDownload(handle, block, prefetch)
		nodeList.PushBack(node)

		handle.SetValue("#", (index+1)*bc.blockSizeMB*_1MB)
		handle.SetValue(fmt.Sprintf("%v", index), block)

	} else {
		return fmt.Errorf("BlockCache::startPrefetch : Failed to get the block %s # %v", handle.Path, offset)
	}

	return nil
}

// startPrefetch: Start prefetchign the blocks from this offset
func (bc *BlockCache) startPrefetch(handle *handlemap.Handle, index uint64) error {
	for i := uint32(0); i < bc.prefetch; i++ {
		err := bc.prepareBlock(handle, index, i >= 3)
		if err != nil {
			return err
		}
		index++
	}

	return nil
}

// download : Method to download the given amount of data
func (bc *BlockCache) lineupDownload(handle *handlemap.Handle, block *Block, prefetch bool) {
	item := &workItem{
		handle:   handle,
		block:    block,
		prefetch: prefetch,
	}

	block.stage = BlockQueued
	bc.threadPool.Schedule(!prefetch, item)
}

// download : Method to download the given amount of data
func (bc *BlockCache) download(i interface{}) {
	item := i.(*workItem)
	fileName := fmt.Sprintf("%s::%v", item.handle.Path, item.block.offset)

	flock := bc.fileLocks.Get(fileName)
	flock.Lock()
	defer flock.Unlock()

	// Update diskpolicy to reflect the new file
	diskNode, found := bc.fileNodeMap.Load(fileName)
	if !found {
		diskNode = bc.diskPolicy.Add(fileName)
		bc.fileNodeMap.Store(fileName, diskNode)
	} else {
		bc.diskPolicy.Refresh(diskNode.(*list.Element))
	}

	// Check local file exists for this offset and file combination or not
	localPath := filepath.Join(bc.tmpPath, fileName)
	_, err := os.Stat(localPath)

	if err == nil {
		// If file exists then read the block from the local file
		f, err := os.Open(localPath)
		if err != nil {
			log.Err("BlockCache::download : Failed to open file %s [%s]", fileName, err.Error())
			_ = os.Remove(localPath)
		} else {
			log.Info("BlockCache::download : Reading data from disk cache %s", fileName)

			_, err = f.Read(item.block.data)
			if err != nil {
				log.Err("BlockCache::download : Failed to read data from disk cache %s [%s]", fileName, err.Error())
				_ = os.Remove(localPath)
			}

			f.Close()
			_ = item.block.ReadyForReading()

			return
		}
	}

	// If file does not exists then download the block from the remote storage
	log.Info("BlockCache::download : Downloading data from remote storage %s", fileName)
	n, err := bc.NextComponent().ReadInBuffer(internal.ReadInBufferOptions{
		Handle: item.handle,
		Offset: int64(item.block.offset),
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

	_ = item.block.ReadyForReading()

	// Dump this buffer to local file
	f, err := os.Create(localPath)
	if err == nil {
		_, err := f.Write(item.block.data)
		if err != nil {
			log.Err("BlockCache::download : Failed to write %s to disk [%v]]", localPath, err.Error())
			_ = os.Remove(localPath)
		}

		f.Close()
		bc.diskPolicy.Refresh(diskNode.(*list.Element))
	}
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
