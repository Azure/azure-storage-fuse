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
	blockSize   uint64
	memSizeMB   uint32
	workers     uint32
	prefetch    uint32

	threadPool *ThreadPool

	diskPolicy *tlru.TLRU
	
	fileLocks *common.LockMap
	fileNodeMap     sync.Map
	maxDiskUsageHit bool
	
	tmpPath   string
	diskSize    uint64
	diskTimeout uint32
}

// Structure defining your config parameters
type BlockCacheOptions struct {
	BlockSize     uint32 `config:"block-size-mb" yaml:"block-size-mb,omitempty"`
	
	MemSize       uint32 `config:"mem-size-mb" yaml:"mem-size-mb,omitempty"`
	PrefetchCount uint32 `config:"prefetch" yaml:"prefetch,omitempty"`
	
	Workers       uint32 `config:"parallelism" yaml:"parallelism,omitempty"`
	
	DiskSize      uint64 `config:"disk-size-mb" yaml:"disk-size-mb,omitempty"`
	DiskTimeout   uint32 `config:"disk-timeout-sec" yaml:"timeout-sec,omitempty"`
	TmpPath       string `config:"path" yaml:"path,omitempty"`
}

// One workitem to be scheduled
type workItem struct {
	handle *handlemap.Handle
	block  *Block
}

const (
	compName              = "block_cache"
	defaultTimeout        = 120
	defaultDiskSize       = 4192
	MAX_POOL_USAGE uint32 = 95
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
	
	// Block cache uses disk to store temporary blocks in order to ensure we are not 
	// downloading the same block multiple times if it is needed by multiple handles 
	// of the same file.
	// This policy will maintain disk space using lru based eviction strategy.
	err := bc.diskPolicy.Start()
	if err != nil {
		log.Err("BlockCache::Start : failed to start diskpolicy [%s]", err.Error())
		return fmt.Errorf("failed to start  disk-policy for block-cache")
	}
	
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

	if !config.IsSet(compName + ".block-size-mb") {
		conf.BlockSize = 8
	}

	if !config.IsSet(compName + ".mem-size-mb") {
		conf.MemSize = 1024
	}

	if !config.IsSet(compName + ".prefetch") {
		conf.PrefetchCount = 8
	}

	if !config.IsSet(compName + ".parallelism") {
		conf.Workers = 32
	}

	bc.blockSizeMB = conf.BlockSize
	bc.blockSize = uint64(bc.blockSizeMB*_1MB)
	bc.memSizeMB = conf.MemSize
	bc.workers = conf.Workers
	bc.prefetch = conf.PrefetchCount
	
	bc.diskSize = uint64(defaultDiskSize) * uint64(_1MB)
	if config.IsSet(compName + ".disk-size-mb") {
		bc.diskSize = conf.DiskSize * uint64(_1MB)
	}
	
	bc.diskTimeout = defaultTimeout
	if config.IsSet(compName + ".disk-timeout-sec") {
		bc.diskTimeout = conf.DiskTimeout
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

	log.Info("BlockCache::Configure : block size %v, mem size %v, worker %v, prefetch %v",
		bc.blockSizeMB, bc.memSizeMB, bc.workers, bc.prefetch)

	bc.threadPool = newThreadPool(bc.workers, bc.download)
	if bc.threadPool == nil {
		log.Err("BlockCache::Configure : fail to init thread pool")
		return fmt.Errorf("BlockCache: failed to init thread pool")
	}

	bc.diskPolicy, err = tlru.New(uint32(bc.diskSize/bc.blockSize), bc.diskTimeout, bc.diskEvict, 60, bc.checkDiskUsage)
	if err != nil {
		log.Err("BlockCache::Configure : fail to create LRU for memory nodes [%s]", err.Error())
		return fmt.Errorf("BlockCache: fail to create LRU for memory nodes")
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
	handle.Prefetched = make(chan int, 1)
	
	poolSize := bc.blockSizeMB*_1MB*(bc.prefetch + 2)
	handle.BlockPool = NewBlockPool((uint64)(bc.blockSizeMB*_1MB), (uint64)(poolSize))
	if handle.BlockPool == nil {
		log.Err("BlockCache::Configure : fail to init Block pool")
		return nil, fmt.Errorf("BlockCache: failed to init Block pool")
	}
	
	// Schedule the prefetch for this handle
	handle.Prefetched <- 1

	return handle, nil
}

// CloseFile: Flush the file and invalidate it from the cache.
func (bc *BlockCache) CloseFile(options internal.CloseFileOptions) error {
	log.Trace("BlockCache::CloseFile : name=%s, handle=%d", options.Handle.Path, options.Handle.ID)

	options.Handle.CleanupWithCallback(func(key string, item interface{}) {
		if key != "#" {
			options.Handle.BlockPool.(*BlockPool).Release(item.(workItem).block)
		}
	})
	
	// TODO: Figure out release of unread blocks
	return nil
}

// ReadInBuffer: Read the local file into a buffer
func (bc *BlockCache) ReadInBuffer(options internal.ReadInBufferOptions) (int, error) {
	// Only one thread should begin prefetch per handle, all other read calls will wait here until prefetch is completed
	// and channel is closed.
	t := 0
	t = <- options.Handle.Prefetched
	
	// Ensure we are the first thread to reach here.
	if t == 1 {
		nextoffset := uint64(options.Offset)
		for i := 0; int32(i) < int32(bc.prefetch - 2) && int64(nextoffset) < options.Handle.Size; i++ {
			success := bc.lineupDownload(options.Handle, nextoffset)
			if !success {
				break
			}
			nextoffset += bc.blockSize
			options.Handle.SetValue("#", nextoffset)
		}
		close(options.Handle.Prefetched)
	}

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

		readOffset := uint64(options.Offset) - (block.id * bc.blockSize)
		bytesRead := copy(options.Data[dataRead:], block.data[readOffset:])

		options.Offset += int64(bytesRead)
		dataRead += bytesRead
	}
	return dataRead, nil
}

// download : Method to download the given amount of data
func (bc *BlockCache) lineupDownload(handle *handlemap.Handle, offset uint64) bool {
	index := bc.getBlockIndex(offset)
	_, found := handle.GetValue(fmt.Sprintf("%v", index))
	if !found {
		item := workItem{
			handle: handle,
			block:  handle.BlockPool.(*BlockPool).Get(),
		}

		if item.block == nil {
			log.Err("BlockCache::lineupDownload : Failed to schedule prefetch of %s # %v, block: %v", handle.Path, offset, item.block.id)
			return false
		}

		item.block.id = offset / bc.blockSize

		handle.SetValue(fmt.Sprintf("%v", item.block.id), item)
		handle.PushFrontBlock(index)
		bc.threadPool.Schedule(item)
	} else {
		log.Debug("Cache missed earlier not liningup download: %v::%v::%v", handle.ID, index, offset)
	}
	
	return true
}

// download : Method to download the given amount of data
func (bc *BlockCache) download(i interface{}) {
	item := i.(workItem)
	offset := int64(item.block.id * bc.blockSize)
	fileName := fmt.Sprintf("%s::%v", item.handle.Path, offset)
	diskUsed := false

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
	}
	
	if err != nil {
		log.Info("BlockCache::download : Reading data from network %s", fileName)
		n, err := bc.NextComponent().ReadInBuffer(internal.ReadInBufferOptions{
			Handle: item.handle,
			Offset: int64(item.block.id * bc.blockSize),
			Data:   item.block.data,
		})

		if err != nil {
			// Fail to read the data so just reschedule this request
			log.Err("BlockCache::download : Failed to read %s from offset %v [%s]", item.handle.Path, item.block.id, err.Error())
			bc.threadPool.Schedule(item)
			return
		} else if n == 0 {
			// No data read so just reschedule this request
			log.Err("BlockCache::download : Failed to read %s from offset %v [0 bytes read]", item.handle.Path, item.block.id)
			bc.threadPool.Schedule(item)
			return
		}
	}

	if !diskUsed {
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
	
	// Unblock readers of this Block
	_ = item.block.ReadyForReading()
}

// getBlockIndex: From offset get the block index
func (bc *BlockCache) getBlockIndex(offset uint64) uint64 {
	return offset / bc.blockSize
}

// getBlock: From offset generate the Block index and get the Block
func (bc *BlockCache) getBlock(handle *handlemap.Handle, readoffset uint64) (*Block, error) {
	if readoffset >= uint64(handle.Size) {
		return nil, io.EOF
	}

	index := bc.getBlockIndex(readoffset)
	item, found := handle.GetValue(fmt.Sprintf("%v", index))

	if !found {
		// This offset is not cached yet, so lineup the download
		success := bc.lineupDownload(handle, readoffset)
		if !success {
			return nil, fmt.Errorf("failed to schedule download")
		}

		item, found = handle.GetValue(fmt.Sprintf("%v", index))
		if !found {
			log.Err("BlockCache::ReadInBuffer : Something went wrong not able to find the Block %s # %v", handle.Path, index)
			return nil, fmt.Errorf("not able to find block immediately after scheudling")
		}
	}

	block := item.(workItem).block

	// Wait for this block to complete the download
	t := int(0)
	t = <-block.state

	if t == 2 {
		// Block is ready and we are the second reader so its time to schedule the next block.
		lastoffset, found := handle.GetValue("#")
		if found && lastoffset.(uint64) < uint64(handle.Size) {
			success := bc.lineupDownload(handle, lastoffset.(uint64))
			if success {
				handle.SetValue("#", lastoffset.(uint64)+bc.blockSize)
			}
		}
		
		_ = block.Unblock()
	} else if t == 1 {
		// block is ready and we are the first reader so its time to release the oldest block.
		if block.id >= 2 {
			ReleaseBlock(handle)
		}
	}
	return block, nil
}

func ReleaseBlock(handle *handlemap.Handle) {
	delId := handle.PopBackBlock()
	delItem, delFound := handle.GetValue(fmt.Sprintf("%v", delId))
	if delFound {
		handle.RemoveValue(fmt.Sprintf("%v", delId))
		<-delItem.(workItem).block.state
		handle.BlockPool.(*BlockPool).Release(delItem.(workItem).block)
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
// we start to evict from disk as soon as usage hits max limit and continue to evict until
// usage drops below the min limit.
func (bc *BlockCache) checkDiskUsage() bool {
	data := getUsage(bc.tmpPath)
	usage := uint32((data * 100) / float64(bc.diskSize))

	if bc.maxDiskUsageHit {
		if usage > MIN_POOL_USAGE {
			return true
		}
		bc.maxDiskUsageHit = false
	} else {
		if usage > MAX_POOL_USAGE {
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
