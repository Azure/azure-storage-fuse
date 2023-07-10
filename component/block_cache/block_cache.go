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

	blockSize uint64
	memSize   uint64

	tmpPath     string
	diskSize    uint64
	diskTimeout uint32

	workers  uint32
	prefetch uint32

	diskPolicy *tlru.TLRU

	blockPool  *BlockPool
	threadPool *ThreadPool
	fileLocks  *common.LockMap

	fileNodeMap     sync.Map
	maxDiskUsageHit bool
	noPrefetch      bool
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
	MIN_PREFETCH          = 5
	MIN_RANDREAD          = 10
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

	if bc.tmpPath != "" {
		err := bc.diskPolicy.Start()
		if err != nil {
			log.Err("BlockCache::Start : failed to start diskpolicy [%s]", err.Error())
			return fmt.Errorf("failed to start  disk-policy for block-cache")
		}
	}

	return nil
}

// Stop : Stop the component functionality and kill all threads started
func (bc *BlockCache) Stop() error {
	log.Trace("BlockCache::Stop : Stopping component %s", bc.Name())
	bc.threadPool.Stop()

	if bc.tmpPath != "" {
		_ = bc.diskPolicy.Stop()
		_ = bc.TempCacheCleanup()
	}

	return nil
}

func (bc *BlockCache) TempCacheCleanup() error {
	if bc.tmpPath == "" {
		return nil
	}

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

	bc.blockSize = uint64(16) * _1MB
	if config.IsSet(compName + ".block-size-mb") {
		bc.blockSize = conf.BlockSize * _1MB

	}

	bc.memSize = uint64(4192) * _1MB
	if config.IsSet(compName + ".mem-size-mb") {
		bc.memSize = conf.MemSize * _1MB
	}

	bc.diskSize = uint64(4192) * _1MB
	if config.IsSet(compName + ".disk-size-mb") {
		bc.diskSize = conf.DiskSize * _1MB
	}
	bc.diskTimeout = defaultTimeout
	if config.IsSet(compName + ".disk-timeout-sec") {
		bc.diskTimeout = conf.DiskTimeout
	}

	bc.prefetch = MIN_PREFETCH
	bc.noPrefetch = false
	if config.IsSet(compName + ".prefetch") {
		bc.prefetch = conf.PrefetchCount
		if bc.prefetch == 0 {
			bc.noPrefetch = true
		}
	}

	bc.maxDiskUsageHit = false

	bc.workers = 128
	if config.IsSet(compName + ".parallelism") {
		bc.workers = conf.Workers
	}

	bc.tmpPath = ""
	if conf.TmpPath != "" {
		bc.tmpPath = common.ExpandPath(conf.TmpPath)

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
	}

	log.Info("BlockCache::Configure : block size %v, mem size %v, worker %v, prefeth %v, disk path %v, max size %v, disk timeout %v",
		bc.blockSize, bc.memSize, bc.workers, bc.prefetch, bc.tmpPath, bc.diskSize, bc.diskTimeout)

	bc.blockPool = NewBlockPool(bc.blockSize, bc.memSize)
	if bc.blockPool == nil {
		log.Err("BlockCache::Configure : fail to init Block pool")
		return fmt.Errorf("BlockCache: failed to init Block pool")
	}

	bc.threadPool = newThreadPool(bc.workers, bc.download)
	if bc.threadPool == nil {
		log.Err("BlockCache::Configure : fail to init thread pool")
		return fmt.Errorf("BlockCache: failed to init thread pool")
	}

	if bc.tmpPath != "" {
		bc.diskPolicy, err = tlru.New(uint32(bc.diskSize/bc.blockSize), bc.diskTimeout, bc.diskEvict, 60, bc.checkDiskUsage)
		if err != nil {
			log.Err("BlockCache::Configure : fail to create LRU for memory nodes [%s]", err.Error())
			return fmt.Errorf("BlockCache: fail to create LRU for memory nodes")
		}
	}

	return nil
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
	for i := 0; i < MIN_PREFETCH; i++ {
		blockList.PushFront(bc.blockPool.MustGet())
	}

	return handle, nil
}

// CloseFile: Flush the file and invalidate it from the cache.
func (bc *BlockCache) CloseFile(options internal.CloseFileOptions) error {
	log.Trace("BlockCache::CloseFile : name=%s, handle=%d", options.Handle.Path, options.Handle.ID)

	// Release the blocks that are in use and wipe out handle map
	options.Handle.Cleanup()

	// Relese the blocks that are not in use
	blockList := options.Handle.TempObj.(*list.List)
	node := blockList.Front()
	for ; node != nil; node = blockList.Front() {
		block := blockList.Remove(node).(*Block)
		block.ReUse()
		bc.blockPool.Release(block)
	}
	options.Handle.TempObj = nil

	return nil
}

// ReadInBuffer: Read the local file into a buffer
func (bc *BlockCache) ReadInBuffer(options internal.ReadInBufferOptions) (int, error) {
	options.Handle.Lock()
	defer options.Handle.Unlock()

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

	return dataRead, nil
}

// getBlock: From offset generate the Block index and get the Block
func (bc *BlockCache) getBlock(handle *handlemap.Handle, readoffset uint64) (*Block, error) {
	if readoffset >= uint64(handle.Size) {
		return nil, io.EOF
	}

	index := bc.getBlockIndex(readoffset)
	node, found := handle.GetValue(fmt.Sprintf("%v", index))
	if !found {
		// If this is the first read request then prefetch all required nodes
		val, _ := handle.GetValue("#")
		if !bc.noPrefetch && val.(uint64) == 0 {
			log.Info("BlockCache::getBlock : Starting the prefetch %v : %s (%v : %v)", handle.ID, handle.Path, readoffset, index)

			// This is the first read for this file handle so start prefetching all the nodes
			err := bc.startPrefetch(handle, index, MIN_PREFETCH, false)
			if err != nil {
				log.Err("BlockCache::getBlock : Unable to start prefetch %s (%v : %v) [%s]", handle.Path, readoffset, index, err.Error())
				return nil, fmt.Errorf("unable to start prefetch for this handle")
			}
		} else {
			handle.OptCnt++
			log.Info("BlockCache::getBlock : Unable to get block %v : %s (%v) Random opt %v", handle.ID, handle.Path, index, handle.OptCnt)

			// This block is not present even after prefetch so lets download it now
			err := bc.refreshBlock(handle, index, true, false)
			if err != nil {
				log.Err("BlockCache::getBlock : Unable to start prefetch %v : %s (%v : %v) [%s]", handle.ID, handle.Path, readoffset, index, err.Error())
				return nil, fmt.Errorf("unable to start prefetch for this handle")
			}
		}

		node, found = handle.GetValue(fmt.Sprintf("%v", index))
		if !found {
			log.Err("BlockCache::getBlock : Something went wrong not able to find the Block %v : %s (%v)", handle.ID, handle.Path, index)
			return nil, fmt.Errorf("not able to find block immediately after scheudling")
		}
	}

	block := node.(*Block)

	// Wait for this block to complete the download
	prefetchCnt := 0
	t := int(0)
	t = <-block.state

	if t == 1 {
		//log.Info("BlockCache::getBlock : First reader for the block hit %v : %s (%v)", handle.ID, handle.Path, index)
		if !bc.noPrefetch {
			// block is ready and we are the first reader so its time to remove the second last block from here
			headBlock := handle.TempObj.(*list.List).Front().Value.(*Block)
			diff := (block.id - headBlock.id)
			if diff >= 2 {
				if handle.OptCnt < MIN_RANDREAD {
					prefetchCnt = MIN_PREFETCH
					headBlock.stage = BlockReady
				}
			}
		}
		block.Unblock()
	}

	nodeList := handle.TempObj.(*list.List)
	cnt := uint32(0)
	if prefetchCnt > 0 {
		if nodeList.Len() < int(bc.prefetch) {
			for i := 0; i < prefetchCnt && nodeList.Len() < int(bc.prefetch); i++ {
				block := bc.blockPool.TryGet()
				if block != nil {
					nodeList.PushFront(block)
					cnt++
				}
			}
		} else {
			cnt = 1
		}

		//log.Info("BlockCache::getBlock : Go for prefetch of %v blocks %v : %s (%v)", cnt, handle.ID, handle.Path, index)
		val, _ := handle.GetValue("#")
		_ = bc.startPrefetch(handle, val.(uint64), cnt, true)
	} else if handle.OptCnt > MIN_RANDREAD && nodeList.Len() > MIN_PREFETCH {
		// There might be excess blocks in the list.
		// As this file is in random read mode now, release the excess buffers
		log.Info("BlockCache::getBlock : Cleanup excessive blocks  %v : %s (%v)", handle.ID, handle.Path, index)
		node := nodeList.Front()

		for ; node != nil && nodeList.Len() > MIN_PREFETCH; node = nodeList.Front() {
			block := nodeList.Remove(node).(*Block)
			if block.id != int64(index) {
				handle.RemoveValue(fmt.Sprintf("%v", block.id))
				block.ReUse()
				bc.blockPool.Release(block)
				cnt++
			}
		}
		log.Info("BlockCache::getBlock : Cleanup excessive blocks  %v : %s (%v blocks removed)", handle.ID, handle.Path, cnt)
	}

	return block, nil
}

// getBlockIndex: From offset get the block index
func (bc *BlockCache) getBlockIndex(offset uint64) uint64 {
	return offset / bc.blockSize
}

// refreshBlock: Get a block from teh list and prepare it for prefetch
func (bc *BlockCache) refreshBlock(handle *handlemap.Handle, index uint64, force bool, prefetch bool) error {
	log.Info("BlockCache::refreshBlock : Request to download %v : %s (%v : %v)", handle.ID, handle.Path, index, prefetch)

	offset := index * bc.blockSize
	nodeList := handle.TempObj.(*list.List)

	node := nodeList.Front()
	block := node.Value.(*Block)

	//log.Info("BlockCache::refreshBlock : Time to enqueue %v : %s (%v)", handle.ID, handle.Path, index)

	if force || block.stage == BlockReady || block.id == -1 {
		if block.id != -1 {
			//log.Info("BlockCache::refreshBlock : Removing %v block for %v : %s", block.id, handle.ID, handle.Path)
			handle.RemoveValue(fmt.Sprintf("%v", block.id))
		}

		//log.Info("BlockCache::refreshBlock : Enqueue %v block for %v : %s", index, handle.ID, handle.Path)
		block.ReUse()
		block.id = int64(index)
		block.offset = offset

		nodeList.MoveToBack(node)
		handle.SetValue(fmt.Sprintf("%v", index), block)
		handle.SetValue("#", (index + 1))

		bc.lineupDownload(handle, block, prefetch)
	} else {
		log.Err("BlockCache::refreshBlock : Failed to get the block %v : %s (%v : %v)", handle.ID, handle.Path, offset, index)
		return fmt.Errorf("failed to get block")
	}

	return nil
}

// startPrefetch: Start prefetchign the blocks from this offset
func (bc *BlockCache) startPrefetch(handle *handlemap.Handle, index uint64, count uint32, prefetch bool) error {
	for i := uint32(0); i < count; i++ {
		_, found := handle.GetValue(fmt.Sprintf("%v", index))
		if !found {
			err := bc.refreshBlock(handle, index, false, prefetch)
			if err != nil {
				return err
			}
			index++
		}
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
	fileName := fmt.Sprintf("%s::%v", item.handle.Path, item.block.id)

	flock := bc.fileLocks.Get(fileName)
	flock.Lock()
	defer flock.Unlock()

	var diskNode any
	found := false
	localPath := ""

	if bc.tmpPath != "" {
		// Update diskpolicy to reflect the new file
		diskNode, found = bc.fileNodeMap.Load(fileName)
		if !found {
			diskNode = bc.diskPolicy.Add(fileName)
			bc.fileNodeMap.Store(fileName, diskNode)
		} else {
			bc.diskPolicy.Refresh(diskNode.(*list.Element))
		}

		// Check local file exists for this offset and file combination or not
		localPath = filepath.Join(bc.tmpPath, fileName)
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

	if bc.tmpPath != "" {
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
}

// diskEvict: Callback when a node from disk expires
func (bc *BlockCache) diskEvict(node *list.Element) {
	fileName := node.Value.(string)
	flock := bc.fileLocks.Get(fileName)
	flock.Lock()
	defer flock.Unlock()

	bc.fileNodeMap.Delete(fileName)

	localPath := filepath.Join(bc.tmpPath, fileName)
	err := os.Remove(localPath)
	if err != nil {
		log.Err("blockCache::diskEvict : Failed to remove %s [%s]", localPath, err.Error())
	}
}

// checkDiskUsage: Callback to check usage of disk and decide whether eviction is needed
func (bc *BlockCache) checkDiskUsage() bool {
	data := getUsage(bc.tmpPath)
	usage := uint32((data * 100) / float64(bc.diskSize))

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
	return comp
}

// On init register this component to pipeline and supply your constructor
func init() {
	internal.AddComponent(compName, NewBlockCacheComponent)
}
