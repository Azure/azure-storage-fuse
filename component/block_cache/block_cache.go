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

package block_cache

import (
	"context"
	"fmt"
	"math"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/config"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal"
	"github.com/Azure/azure-storage-fuse/v2/internal/handlemap"
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

	blockSize       uint64          // Size of each block to be cached
	memSize         uint64          // Mem size to be used for caching at the startup
	mntPath         string          // Mount path
	tmpPath         string          // Disk path where these blocks will be cached
	diskSize        uint64          // Size of disk space allocated for the caching
	diskTimeout     uint32          // Timeout for which disk blocks will be cached
	workers         uint32          // Number of threads working to fetch the blocks
	prefetch        uint32          // Number of blocks to be prefetched
	fileLocks       *common.LockMap // Locks for each file_blockid to avoid multiple threads to fetch same block
	fileNodeMap     sync.Map        // Map holding files that are there in our cache
	maxDiskUsageHit bool            // Flag to indicate if we have hit max disk usage
	noPrefetch      bool            // Flag to indicate if prefetch is disabled
	prefetchOnOpen  bool            // Start prefetching on file open call instead of waiting for first read
	consistency     bool            // Flag to indicate if strong data consistency is enabled
	//	stream          *Stream
	lazyWrite              bool           // Flag to indicate if lazy write is enabled
	fileCloseOpt           sync.WaitGroup // Wait group to wait for all async close operations to complete
	maxFileSize            uint64         // Max file size supported by block cache
	deferEmptyBlobCreation bool           // Flag to indicate if empty blob creation is deferred until file is closed
}

// Structure defining your config parameters
type BlockCacheOptions struct {
	BlockSize              float64 `config:"block-size-mb" yaml:"block-size-mb,omitempty"`
	MemSize                uint64  `config:"mem-size-mb" yaml:"mem-size-mb,omitempty"`
	TmpPath                string  `config:"path" yaml:"path,omitempty"`
	DiskSize               uint64  `config:"disk-size-mb" yaml:"disk-size-mb,omitempty"`
	DiskTimeout            uint32  `config:"disk-timeout-sec" yaml:"timeout-sec,omitempty"`
	PrefetchCount          uint32  `config:"prefetch" yaml:"prefetch,omitempty"`
	Workers                uint32  `config:"parallelism" yaml:"parallelism,omitempty"`
	PrefetchOnOpen         bool    `config:"prefetch-on-open" yaml:"prefetch-on-open,omitempty"`
	Consistency            bool    `config:"consistency" yaml:"consistency,omitempty"`
	CleanupOnStart         bool    `config:"cleanup-on-start" yaml:"cleanup-on-start,omitempty"`
	DeferEmptyBlobCreation bool    `config:"defer-empty-blob-creation" yaml:"defer-empty-blob-creation,omitempty"`
}

const (
	compName                = "block_cache"
	defaultTimeout          = 120
	defaultBlockSize        = 16
	MAX_POOL_USAGE   uint32 = 80
	MIN_POOL_USAGE   uint32 = 50
	MIN_PREFETCH            = 5
	MIN_WRITE_BLOCK         = 3
	MIN_RANDREAD            = 10
	MAX_FAIL_CNT            = 3
	MAX_BLOCKS              = 50000
)

// Verification to check satisfaction criteria with Component Interface
var _ internal.Component = &BlockCache{}
var bc *BlockCache

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

	if err := createFreeList(bc.blockSize, bc.memSize); err != nil {
		log.Err("BlockCache::Start : fail to initialize buffer pool [%v]", err)
		return fmt.Errorf("failed to start %s [%v]", bc.Name(), err)
	}

	NewWorkerPool(int(bc.workers))
	NewBufferTableMgr()
	return nil
}

// Stop : Stop the component functionality and kill all threads started
func (bc *BlockCache) Stop() error {
	log.Trace("BlockCache::Stop : Stopping component %s", bc.Name())

	destroyFreeList()

	// Clear the disk cache on exit
	// if bc.tmpPath != "" {
	// 	_ = bc.diskPolicy.Stop()
	// 	_ = common.TempCacheCleanup(bc.tmpPath)
	// }

	return nil
}

// GenConfig : Generate the default config for the component
func (bc *BlockCache) GenConfig() string {
	log.Info("BlockCache::Configure : config generation started")
	return ""
}

// Configure : Pipeline will call this method after constructor so that you can read config and initialize yourself
//
//	Return failure if any config is not valid to exit the process
func (bc *BlockCache) Configure(_ bool) error {
	log.Trace("BlockCache::Configure : %s", bc.Name())

	// if common.IsStream {
	// 	err := bc.stream.Configure(true)
	// 	if err != nil {
	// 		log.Err("BlockCache:Stream::Configure : config error [invalid config attributes]")
	// 		return fmt.Errorf("config error in %s [%s]", bc.Name(), err.Error())
	// 	}
	// }

	conf := BlockCacheOptions{}
	err := config.UnmarshalKey(bc.Name(), &conf)
	if err != nil {
		log.Err("BlockCache::Configure : config error [invalid config attributes]")
		return fmt.Errorf("config error in %s [%s]", bc.Name(), err.Error())
	}

	if config.IsSet(compName + ".defer-empty-blob-creation") {
		bc.deferEmptyBlobCreation = conf.DeferEmptyBlobCreation
	} else {
		bc.deferEmptyBlobCreation = true
	}

	bc.blockSize = uint64(defaultBlockSize) * common.MbToBytes
	if config.IsSet(compName + ".block-size-mb") {
		bc.blockSize = uint64(conf.BlockSize * float64(common.MbToBytes))
	}

	bc.maxFileSize = bc.blockSize * MAX_BLOCKS

	if config.IsSet(compName + ".mem-size-mb") {
		bc.memSize = conf.MemSize * common.MbToBytes
	} else {
		//		bc.memSize = bc.getDefaultMemSize()
	}

	bc.diskTimeout = defaultTimeout
	if config.IsSet(compName + ".disk-timeout-sec") {
		bc.diskTimeout = conf.DiskTimeout
	}

	bc.consistency = conf.Consistency

	bc.prefetchOnOpen = conf.PrefetchOnOpen
	bc.prefetch = uint32(math.Max((MIN_PREFETCH*2)+1, (float64)(2*runtime.NumCPU())))
	bc.noPrefetch = false

	if (!config.IsSet(compName + ".mem-size-mb")) && (uint64(bc.prefetch)*uint64(bc.blockSize)) > bc.memSize {
		bc.prefetch = (MIN_PREFETCH * 2) + 1
	}

	err = config.UnmarshalKey("lazy-write", &bc.lazyWrite)
	if err != nil {
		log.Err("BlockCache: config error [unable to obtain lazy-write]")
		return fmt.Errorf("config error in %s [%s]", bc.Name(), err.Error())
	}

	if config.IsSet(compName + ".prefetch") {
		bc.prefetch = conf.PrefetchCount
		if bc.prefetch == 0 {
			bc.noPrefetch = true
		} // else if conf.PrefetchCount <= (MIN_PREFETCH * 2) {
		// 	log.Err("BlockCache::Configure : Prefetch count can not be less then %v", (MIN_PREFETCH*2)+1)
		// 	return fmt.Errorf("config error in %s [invalid prefetch count]", bc.Name())
		// }
	}

	bc.maxDiskUsageHit = false

	bc.workers = uint32(3 * runtime.NumCPU())
	if config.IsSet(compName + ".parallelism") {
		bc.workers = conf.Workers
	}

	bc.tmpPath = common.ExpandPath(conf.TmpPath)

	if bc.tmpPath != "" {
		//check mnt path is not same as temp path
		err = config.UnmarshalKey("mount-path", &bc.mntPath)
		if err != nil {
			log.Err("BlockCache: config error [unable to obtain Mount Path]")
			return fmt.Errorf("config error in %s [%s]", bc.Name(), err.Error())
		}

		if bc.mntPath == bc.tmpPath {
			log.Err("BlockCache: config error [tmp-path is same as mount path]")
			return fmt.Errorf("config error in %s error [tmp-path is same as mount path]", bc.Name())
		}

		// Extract values from 'conf' and store them as you wish here
		_, err = os.Stat(bc.tmpPath)
		if os.IsNotExist(err) {
			log.Info("BlockCache: config error [tmp-path does not exist. attempting to create tmp-path.]")
			err := os.Mkdir(bc.tmpPath, os.FileMode(0755))
			if err != nil {
				log.Err("BlockCache: config error creating directory of temp path after clean [%s]", err.Error())
				return fmt.Errorf("config error in %s [%s]", bc.Name(), err.Error())
			}
		}

		if !common.IsDirectoryEmpty(bc.tmpPath) {
			log.Err("BlockCache: config error %s directory is not empty", bc.tmpPath)
			return fmt.Errorf("config error in %s [%s]", bc.Name(), "temp directory not empty")
		}

		//		bc.diskSize = bc.getDefaultDiskSize(bc.tmpPath)
		if config.IsSet(compName + ".disk-size-mb") {
			bc.diskSize = conf.DiskSize * common.MbToBytes
		}
	}

	if bc.tmpPath != "" {
		// bc.diskPolicy, err = tlru.New(uint32((bc.diskSize)/bc.blockSize), bc.diskTimeout, bc.diskEvict, 60, bc.checkDiskUsage)
		// if err != nil {
		// 	log.Err("BlockCache::Configure : fail to create LRU for memory nodes [%s]", err.Error())
		// 	return fmt.Errorf("config error in %s [%s]", bc.Name(), err.Error())
		// }
	}

	log.Crit("BlockCache::Configure : block size %v, mem size %v, worker %v, prefetch %v, disk path %v, max size %v, disk timeout %v, prefetch-on-open %t, maxDiskUsageHit %v, noPrefetch %v, consistency %v, cleanup-on-start %t, defer-empty-blob-creation %v",
		bc.blockSize, bc.memSize, bc.workers, bc.prefetch, bc.tmpPath, bc.diskSize, bc.diskTimeout, bc.prefetchOnOpen, bc.maxDiskUsageHit, bc.noPrefetch, bc.consistency, conf.CleanupOnStart, bc.deferEmptyBlobCreation)

	return nil
}

func (bc *BlockCache) GetAttr(options internal.GetAttrOptions) (*internal.ObjAttr, error) {
	log.Trace("BlockCache::GetAttr : file: %s", options.Name)

	attr, err := bc.NextComponent().GetAttr(options)
	if err != nil {
		return attr, err
	}

	// file stucture has more updated info than attribute cache/Azure storage, if the file is open
	file, ok := checkFileExistsInOpen(options.Name)
	if ok {
		fileSize := atomic.LoadInt64(&file.size)
		if (fileSize != -1) && fileSize != attr.Size {
			// There has been a modification done on the file. Return new attribute with new file size.
			// We dont want to update the value inside the attribute itself, as it changes the state of the attribute
			// inside the attribute cache
			newattr := *attr
			newattr.Size = fileSize
			return &newattr, nil
		}
	}

	return attr, nil
}

// CreateFile: Create a new file
func (bc *BlockCache) CreateFile(options internal.CreateFileOptions) (*handlemap.Handle, error) {
	log.Trace("BlockCache::CreateFile : name=%s, mode=%s", options.Name, options.Mode)
	_, err := bc.NextComponent().CreateFile(options)
	if err != nil {
		log.Err("BlockCache::CreateFile : Failed to create file %s", options.Name)
		return nil, err
	}

	return bc.OpenFile(internal.OpenFileOptions{
		Name:  options.Name,
		Flags: os.O_RDWR | os.O_CREATE,
		Mode:  options.Mode,
	})
}

// OpenFile: Create a handle for the file user has requested to open
func (bc *BlockCache) OpenFile(options internal.OpenFileOptions) (*handlemap.Handle, error) {
	log.Trace("BlockCache::OpenFile : name=%s, flags=%s, mode=%s", options.Name, common.PrettyOpenFlags(options.Flags), options.Mode)

	attr, err := bc.GetAttr(internal.GetAttrOptions{Name: options.Name})
	if err != nil {
		log.Err("BlockCache::OpenFile : Failed to get attr of %s [%s]", options.Name, err.Error())
		return nil, err
	}

	handle := createFreshHandleForFile(options.Name, attr.Size, attr.Mtime, options.Flags)

	// Get file object from the map or create a new one for this path.
	f, firstOpen := getFileFromPath(handle)
	f.mu.Lock()
	defer f.mu.Unlock()

	handle.IFObj = &blockCacheHandle{
		file:            f,
		patternDetector: newPatternDetector(),
	}

	if f.size == -1 {
		f.size = attr.Size
		f.sizeOnStorage = attr.Size
	}

	if options.Flags&os.O_TRUNC != 0 {
		log.Debug("BlockCache::OpenFile : Truncating file %s on open", options.Name)

		if !firstOpen {
			// There are some open handles for this file, flush all the data before truncating.
			err = f.flush(false /* takefilelock */)
			if err != nil {
				log.Err("BlockCache::OpenFile : Failed to flush file %s before truncating on open [%v]", options.Name, err)
				deleteOpenHandleForFile(handle, false /* takeFileLock */)
				return nil, err
			}
			releaseAllBuffersForFile(f)
			f.blockList = newBlockList()
		}

		f.size = 0
		f.synced = false
	}

	if f.size == 0 {
		// This check would be helpful for newly created files
		f.blockList.state = blockListValid
	}

	if f.size > 0 {
		if (options.Flags&os.O_WRONLY != 0) || (options.Flags&os.O_RDWR != 0) {
			if f.blockList.state == blockListNotRetrieved {
				blkList, err := bc.NextComponent().GetCommittedBlockList(options.Name)
				if err != nil {
					log.Err("BlockCache::OpenFile : Failed to get block list of %s, first_open: %v, curOpenHandles: %d, [%v]",
						options.Name, firstOpen, len(f.handles), err)
					deleteOpenHandleForFile(handle, false /* takeFileLock */)
					return nil, err
				}

				err = validateBlockList(blkList, f)
				if err != nil {
					log.Err("BlockCache::OpenFile : Invalid block list for file: %s, first_open: %v, curOpenHandles: %d,  [%v]",
						options.Name, firstOpen, len(f.handles), err)
					f.blockList.state = blockListInvalid
					deleteOpenHandleForFile(handle, false /* takeFileLock */)
					return nil, err
				} else {
					f.blockList.state = blockListValid
				}
			} else if f.blockList.state == blockListInvalid {
				log.Err("BlockCache::OpenFile : Invalid block list for file: %s, first_open: %v, curOpenHandles: %d",
					options.Name, firstOpen, len(f.handles))
				deleteOpenHandleForFile(handle, false /* takeFileLock */)
				return nil, fmt.Errorf("invalid block list for file: %s", options.Name)
			}
		} else {
			updateBlockListForReadOnlyFile(f)
		}
	}

	// libfuse handler, is not sending the flush call if this is not set. So setting it by default.
	// TODO: no need for this
	handle.Flags.Set(handlemap.HandleFlagDirty)

	return handle, nil
}

// ReadInBuffer: Read the file into a buffer
func (bc *BlockCache) ReadInBuffer(options *internal.ReadInBufferOptions) (int, error) {
	bcHandle := options.Handle.IFObj.(*blockCacheHandle)

	log.Debug("BlockCache::ReadInBuffer : name: %s, buf size: %d, offset: %d, handle: %d",
		options.Handle.Path, len(options.Data), options.Offset, options.Handle.ID)

	// we schedule read-ahead per handle, rather than per file, to support multiple handles reading concurrently
	// on the same file with different access patterns.
	bcHandle.file.scheduleReadAhead(bcHandle.patternDetector, options.Offset)

	n, err := bcHandle.file.read(options)
	if err != nil {
		log.Err("BlockCache::ReadInBuffer : Failed to read file %s at offset %d, size %d [%v]",
			options.Handle.Path, options.Offset, len(options.Data), err)
	}

	return n, err
}

// WriteFile: Write to the local file
func (bc *BlockCache) WriteFile(options *internal.WriteFileOptions) (int, error) {
	bcHandle := options.Handle.IFObj.(*blockCacheHandle)

	log.Debug("BlockCache::WriteFile : name: %s, buf size: %d, offset: %d, handle: %d",
		options.Handle.Path, len(options.Data), options.Offset, options.Handle.ID)

	err := bcHandle.file.write(options)
	if err != nil {
		log.Err("BlockCache::WriteFile : Failed to write file %s at offset %d, size %d [%v]",
			options.Handle.Path, options.Offset, len(options.Data), err)
	}

	return len(options.Data), err
}

// TruncateFile: Truncate the file to specified size
func (bc *BlockCache) TruncateFile(options internal.TruncateFileOptions) error {
	log.Trace("BlockCache::TruncateFile : name: %s, size: %d", options.Name, options.NewSize)

	if options.Handle == nil {
		log.Info("BlockCache::TruncateFile : Handle is nil for file %s, Opening the file", options.Name)

		truncHandle, err := bc.OpenFile(internal.OpenFileOptions{
			Name:  options.Name,
			Flags: os.O_RDWR,
		})
		if err != nil {
			log.Err("BlockCache::TruncateFile : Failed to open file %s for truncate [%v]", options.Name, err)
			return err
		}

		defer func() {
			err := bc.ReleaseFile(internal.ReleaseFileOptions{
				Handle: truncHandle,
			})
			if err != nil {
				log.Err("BlockCache::TruncateFile : Failed to release handle for file %s after truncate [%v]", options.Name, err)
			}
		}()

		options.Handle = truncHandle
	}

	bcHandle := options.Handle.IFObj.(*blockCacheHandle)

	log.Debug("BlockCache::TruncateFile : name: %s, size: %d, handle: %d",
		options.Handle.Path, options.NewSize, options.Handle.ID)

	err := bcHandle.file.truncate(&options)
	if err != nil {
		log.Err("BlockCache::TruncateFile : Failed to truncate file %s to size %d [%v]",
			options.Handle.Path, options.NewSize, err)
		return err
	}

	return nil
}

// SyncFile: Sync the file to remote storage
// Call comes when user-application calls fsync on the file descriptor
func (bc *BlockCache) SyncFile(options internal.SyncFileOptions) error {
	bcHandle := options.Handle.IFObj.(*blockCacheHandle)

	log.Debug("BlockCache::SyncFile : handle: %d, path: %s", options.Handle.ID, options.Handle.Path)

	err := bcHandle.file.flush(true /* takefilelock */)
	if err != nil {
		log.Err("BlockCache::SyncFile : Failed to sync file %s [%v]", options.Handle.Path, err)
		return err
	}

	return nil
}

// FlushFile: Call comes when user-application is closing the file descriptor.
// There is a possibility that we get multiple flush calls for the same handle if user used dup2 or fork.
func (bc *BlockCache) FlushFile(options internal.FlushFileOptions) error {
	bcHandle := options.Handle.IFObj.(*blockCacheHandle)

	log.Debug("BlockCache::FlushFile : handle: %d, path: %s", options.Handle.ID, options.Handle.Path)

	err := bcHandle.file.flush(true /* takefilelock */)
	if err != nil {
		log.Err("BlockCache::FlushFile : Failed to flush file %s [%v]", options.Handle.Path, err)
		return err
	}

	return nil
}

// ReleaseFile: all the FD's corresponding to this handle were closed by application so release all the blocks
// and submit back to the pool.
// Flush the file before releasing, if some memory mapped file is still dirty..btw, we don't get the flush call from
// FUSE if the backing fd of the map is already closed by the application.
func (bc *BlockCache) ReleaseFile(options internal.ReleaseFileOptions) error {
	log.Trace("BlockCache::ReleaseFile : handle: %d, path: %s", options.Handle.ID, options.Handle.Path)

	err := bc.FlushFile(internal.FlushFileOptions{
		Handle: options.Handle,
	})
	if err != nil {
		log.Err("BlockCache::ReleaseFile : Failed to flush file %s before release [%v]", options.Handle.Path, err)
	}

	deleteOpenHandleForFile(options.Handle, true /* takeFileLock */)
	// freeList.debugListMustBeFull()
	log.Debug("BlockCache::ReleaseFile : Released handle: %d, path: %s", options.Handle.ID, options.Handle.Path)
	return nil
}

// DeleteFile: Invalidate the file in local cache.
func (bc *BlockCache) DeleteFile(options internal.DeleteFileOptions) error {
	log.Trace("BlockCache::DeleteFile : name: %s", options.Name)

	err := bc.NextComponent().DeleteFile(options)
	if err != nil {
		log.Err("BlockCache::DeleteFile : error  %s [%s]", options.Name, err.Error())
		return err
	}

	return nil
}

// RenameFile: Invalidate the file in local cache.
func (bc *BlockCache) RenameFile(options internal.RenameFileOptions) error {
	log.Trace("BlockCache::RenameFile : src: %s -> dst: %s", options.Src, options.Dst)

	err := bc.NextComponent().RenameFile(options)
	if err != nil {
		log.Err("BlockCache::RenameFile : %s failed to rename file [%s]", options.Src, err.Error())
		return err
	}

	return nil
}

// DeleteDir: Recursively invalidate the directory and its children
func (bc *BlockCache) DeleteDir(options internal.DeleteDirOptions) error {
	log.Trace("BlockCache::DeleteDir : name: %s", options.Name)

	err := bc.NextComponent().DeleteDir(options)
	if err != nil {
		log.Err("BlockCache::DeleteDir : %s failed", options.Name)
		return err
	}

	return nil
}

// RenameDir: Recursively invalidate the source directory and its children
func (bc *BlockCache) RenameDir(options internal.RenameDirOptions) error {
	log.Trace("BlockCache::RenameDir : src: %s -> dst: %s", options.Src, options.Dst)

	err := bc.NextComponent().RenameDir(options)
	if err != nil {
		log.Err("BlockCache::RenameDir : error %s [%s]", options.Src, err.Error())
		return err
	}

	return nil
}

func (bc *BlockCache) StatFs() (*syscall.Statfs_t, bool, error) {
	log.Trace("BlockCache::StatFS")
	return &syscall.Statfs_t{}, true, nil
}

// ------------------------- Factory -------------------------------------------
// Pipeline will call this method to create your object, initialize your variables here
// << DO NOT DELETE ANY AUTO GENERATED CODE HERE >>
func NewBlockCacheComponent() internal.Component {
	comp := &BlockCache{
		fileLocks: common.NewLockMap(),
	}
	bc = comp
	comp.SetName(compName)
	return comp
}

// On init register this component to pipeline and supply your constructor
func init() {
	internal.AddComponent(compName, NewBlockCacheComponent)
	blockSizeMb := config.AddFloat64Flag("block-cache-block-size", 0.0, "Size (in MB) of a block to be downloaded for block-cache.")
	config.BindPFlag(compName+".block-size-mb", blockSizeMb)

	blockPoolMb := config.AddUint64Flag("block-cache-pool-size", 0, "Size (in MB) of total memory preallocated for block-cache.")
	config.BindPFlag(compName+".mem-size-mb", blockPoolMb)

	blockCachePath := config.AddStringFlag("block-cache-path", "", "Path to store downloaded blocks.")
	config.BindPFlag(compName+".path", blockCachePath)

	blockDiskMb := config.AddUint64Flag("block-cache-disk-size", 0, "Size (in MB) of total disk capacity that block-cache can use.")
	config.BindPFlag(compName+".disk-size-mb", blockDiskMb)

	blockDiskTimeout := config.AddUint32Flag("block-cache-disk-timeout", 0, "Timeout (in seconds) for which persisted data remains in disk cache.")
	config.BindPFlag(compName+".disk-timeout-sec", blockDiskTimeout)

	blockCachePrefetch := config.AddUint32Flag("block-cache-prefetch", 0, "Max number of blocks to prefetch.")
	config.BindPFlag(compName+".prefetch", blockCachePrefetch)

	blockParallelism := config.AddUint32Flag("block-cache-parallelism", 128, "Number of worker thread responsible for upload/download jobs.")
	config.BindPFlag(compName+".parallelism", blockParallelism)

	blockCachePrefetchOnOpen := config.AddBoolFlag("block-cache-prefetch-on-open", false, "Start prefetching on open or wait for first read.")
	config.BindPFlag(compName+".prefetch-on-open", blockCachePrefetchOnOpen)

	strongConsistency := config.AddBoolFlag("block-cache-strong-consistency", false, "Enable strong data consistency for block cache.")
	config.BindPFlag(compName+".consistency", strongConsistency)

	// New flags go here
	deferEmptyBlobCreation := config.AddBoolFlag("block-cache-defer-empty-file-creation", true, "When a new file is created, defer its creation on the remote storage until data is actually written to it. file is created on remote storage when the handle is closed/fsynced.")
	config.BindPFlag(compName+".defer-empty-blob-creation", deferEmptyBlobCreation)
}
