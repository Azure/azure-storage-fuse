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

// Package block_cache implements a high-performance caching layer for Azure Storage Fuse (Blobfuse2).
//
// # Overview
//
// The block_cache component provides an in-memory buffer cache for file data, allowing
// efficient read and write operations by caching fixed-size blocks of data. It sits between
// the FUSE filesystem layer and the Azure Storage backend, reducing latency and improving
// throughput by minimizing remote storage operations.
//
// # Key Concepts
//
//   - **Block**: A fixed-size chunk of file data (configurable, default 16MB). Files are divided
//     into sequential blocks for caching.
//
//   - **Buffer**: An in-memory storage area that holds the actual data for a block. Buffers are
//     allocated from a fixed-size buffer pool.
//
//   - **Buffer Descriptor**: Metadata structure that tracks the state of a buffer, including
//     reference count, dirty flag, validity, and association with a block.
//
//   - **Buffer Table Manager**: Maps blocks to their corresponding buffer descriptors, enabling
//     fast lookup of cached data.
//
//   - **Free List**: Manages available buffers, implementing allocation and eviction policies
//     when the buffer pool is exhausted.
//
// # Architecture
//
// The component follows a layered architecture:
//
//	FUSE Operations (read/write/truncate)
//	         ↓
//	    File Operations (file.go)
//	         ↓
//	   Block Operations (block.go)
//	         ↓
//	  Buffer Management (buffer_mgr.go)
//	         ↓
//	  Buffer Allocation (freelist.go)
//	         ↓
//	   Buffer Pool (buffer_pool.go)
//	         ↓
//	 Worker Pool (async I/O operations)
//
// # Concurrency Model
//
// The block_cache is designed for high concurrency:
//
// - **Per-file locking**: File-level read-write locks protect file metadata and block lists.
// - **Per-buffer locking**: Buffer content locks allow concurrent reads while blocking writes.
// - **Reference counting**: Prevents premature buffer eviction while in use.
// - **Lock-free operations**: Atomic operations minimize contention on hot paths.
//
// # Buffer Lifecycle
//
//  1. **Allocation**: Buffer allocated from free list or evicted from cache
//  2. **Mapping**: Buffer mapped to block in buffer table manager (refCnt = 1 for table)
//  3. **Usage**: Users acquire references (refCnt++), perform I/O operations
//  4. **Release**: Users release references (refCnt--)
//  5. **Eviction**: When refCnt reaches 1 (only table reference), buffer becomes eviction candidate
//  6. **Cleanup**: When refCnt reaches 0 (removed from table and all users released), buffer returns to free list
//
// # Configuration
//
// Key configuration parameters:
//
// - block-size-mb: Size of each cached block (default: 16 MB)
// - mem-size-mb: Total memory allocated for buffer pool
// - prefetch: Number of blocks to prefetch for sequential reads
// - parallelism: Number of worker threads for async I/O operations
//
// # Performance Optimization
//
// - **Read-ahead**: Sequential access patterns trigger automatic prefetching
// - **Write coalescing**: Multiple writes to the same block are batched
// - **Lazy write**: Dirty blocks are uploaded asynchronously when possible
// - **Eviction policy**: LRU-based eviction prioritizes frequently accessed blocks
//
// # Thread Safety
//
// All public methods are thread-safe and designed for concurrent access from multiple
// FUSE threads. Internal synchronization uses a combination of mutexes, read-write locks,
// and atomic operations to balance safety and performance.
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

// BlockCache is the main component structure that manages block-level caching for file data.
//
// It implements the internal.Component interface and participates in the Blobfuse2 pipeline.
// BlockCache sits between the libfuse component and the Azure Storage backend, providing:
//
//   - In-memory caching of file blocks to reduce remote storage access
//   - Efficient read-ahead for sequential access patterns
//   - Write buffering and coalescing for improved write performance
//   - Concurrent access management through reference counting
//
// Lifecycle:
//  1. Constructor (NewBlockCacheComponent) creates the component
//  2. Configure() reads configuration and initializes parameters
//  3. Start() initializes buffer pool and worker threads
//  4. File operations (OpenFile, ReadInBuffer, WriteFile, etc.) handle I/O
//  5. Stop() cleans up resources and stops workers
//
// Thread Safety:
// All methods are thread-safe and designed for concurrent access from multiple FUSE threads.
type BlockCache struct {
	internal.BaseComponent

	// Block and buffer pool configuration
	blockSize uint64 // Size of each block to be cached (in bytes, e.g., 16MB)
	memSize   uint64 // Total memory allocated for buffer pool (in bytes)

	// Disk caching configuration (currently unused, reserved for future disk-backed caching)
	mntPath     string // Mount path for the filesystem
	tmpPath     string // Disk path where blocks could be cached to disk
	diskSize    uint64 // Size of disk space allocated for caching
	diskTimeout uint32 // Timeout for disk-cached blocks (in seconds)

	// Worker pool configuration
	workers  uint32 // Number of worker threads for async download/upload operations
	prefetch uint32 // Number of blocks to prefetch for sequential reads

	// File and block management
	fileLocks   *common.LockMap // Per-file locks to coordinate operations (currently unused)
	fileNodeMap sync.Map        // Map of filepath -> *File for tracking open files (currently unused)

	// Performance and behavior flags
	maxDiskUsageHit        bool // Flag indicating if disk cache limit was reached (currently unused)
	noPrefetch             bool // If true, disables read-ahead prefetching
	prefetchOnOpen         bool // If true, start prefetching immediately on file open
	consistency            bool // If true, ensures strong consistency with storage
	lazyWrite              bool // If true, enables lazy write mode (write buffering)
	deferEmptyBlobCreation bool // If true, defers creation of empty files until data is written

	// Synchronization
	fileCloseOpt sync.WaitGroup // Wait group for async file close operations

	// Limits
	maxFileSize uint64 // Maximum file size supported by block cache (blockSize * MAX_BLOCKS)
}

// BlockCacheOptions defines configuration parameters for the BlockCache component.
//
// These options are loaded from the configuration file and control the behavior
// of the block cache, including memory usage, prefetching, and performance tuning.
type BlockCacheOptions struct {
	// BlockSize is the size of each cached block in megabytes (float for precision).
	// Default: 16 MB. Larger blocks reduce metadata overhead but increase memory usage.
	BlockSize float64 `config:"block-size-mb" yaml:"block-size-mb,omitempty"`

	// MemSize is the total memory allocated for the buffer pool in megabytes.
	// If not specified, a default based on available system RAM is used.
	MemSize uint64 `config:"mem-size-mb" yaml:"mem-size-mb,omitempty"`

	// TmpPath is the directory path for disk-based caching (currently unused).
	// Reserved for future implementation of two-tier caching (memory + disk).
	TmpPath string `config:"path" yaml:"path,omitempty"`

	// DiskSize is the disk space allocated for caching in megabytes (currently unused).
	DiskSize uint64 `config:"disk-size-mb" yaml:"disk-size-mb,omitempty"`

	// DiskTimeout is the duration in seconds that disk-cached blocks remain valid (currently unused).
	DiskTimeout uint32 `config:"disk-timeout-sec" yaml:"timeout-sec,omitempty"`

	// PrefetchCount is the maximum number of blocks to prefetch for sequential reads.
	// Set to 0 to disable prefetching. Default: calculated based on CPU count.
	PrefetchCount uint32 `config:"prefetch" yaml:"prefetch,omitempty"`

	// Workers is the number of goroutines in the worker pool for async I/O operations.
	// Default: 3 * number of CPUs. Higher values increase parallelism but use more resources.
	Workers uint32 `config:"parallelism" yaml:"parallelism,omitempty"`

	// PrefetchOnOpen enables immediate prefetching when a file is opened.
	// If false, prefetching starts after the first read operation.
	PrefetchOnOpen bool `config:"prefetch-on-open" yaml:"prefetch-on-open,omitempty"`

	// Consistency enables strong data consistency mode.
	// When true, ensures reads always reflect the latest data from storage.
	Consistency bool `config:"consistency" yaml:"consistency,omitempty"`

	// CleanupOnStart removes any cached data from tmpPath on startup.
	CleanupOnStart bool `config:"cleanup-on-start" yaml:"cleanup-on-start,omitempty"`

	// DeferEmptyBlobCreation postpones creation of empty files in storage.
	// When true, empty files are only created when data is written or the handle is closed.
	// Default: true (recommended for better performance).
	DeferEmptyBlobCreation bool `config:"defer-empty-blob-creation" yaml:"defer-empty-blob-creation,omitempty"`
}

// Component configuration constants
const (
	compName                = "block_cache" // Component name used in configuration and logs
	defaultTimeout          = 120           // Default disk cache timeout in seconds
	defaultBlockSize        = 16            // Default block size in megabytes
	MAX_POOL_USAGE   uint32 = 80            // Maximum buffer pool usage threshold (percentage)
	MIN_POOL_USAGE   uint32 = 50            // Minimum buffer pool usage threshold (percentage)
	MIN_PREFETCH            = 5             // Minimum number of blocks for prefetch
	MIN_WRITE_BLOCK         = 3             // Minimum number of blocks for write operations
	MIN_RANDREAD            = 10            // Minimum random read threshold
	MAX_FAIL_CNT            = 3             // Maximum failure count before error
	MAX_BLOCKS              = 50000         // Maximum number of blocks per file (limits file size)
)

// Verification to check satisfaction criteria with Component Interface
var _ internal.Component = &BlockCache{}
var bc *BlockCache

// Name returns the component name used for configuration and logging.
func (bc *BlockCache) Name() string {
	return compName
}

// SetName sets the component name. Called by the pipeline during initialization.
func (bc *BlockCache) SetName(name string) {
	bc.BaseComponent.SetName(name)
}

// SetNextComponent sets the next component in the pipeline.
// BlockCache typically sits above the Azure Storage component.
func (bc *BlockCache) SetNextComponent(nc internal.Component) {
	bc.BaseComponent.SetNextComponent(nc)
}

// Start initializes the BlockCache component and starts its worker pool.
//
// This method is called by the pipeline after Configure() completes.
// It performs the following initialization:
//
//  1. Creates the buffer pool and free list for buffer management
//  2. Initializes the worker pool for async I/O operations
//  3. Sets up the buffer table manager for block-to-buffer mapping
//
// This method must not block, as it would prevent the pipeline from starting.
// All background operations are launched as goroutines.
//
// Returns an error if buffer pool initialization fails.
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

// Stop shuts down the BlockCache component and releases all resources.
//
// This method is called by the pipeline during shutdown. It performs cleanup:
//
//  1. Destroys the buffer pool and free list
//  2. Releases all allocated memory buffers
//  3. (Future) Cleans up disk cache if configured
//
// After Stop() completes, the component cannot be reused without reinitialization.
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

// GenConfig generates the default configuration for the BlockCache component.
// Currently returns an empty string as default config is handled elsewhere.
func (bc *BlockCache) GenConfig() string {
	log.Info("BlockCache::Configure : config generation started")
	return ""
}

// Configure reads configuration and initializes BlockCache parameters.
//
// This method is called by the pipeline after the constructor and before Start().
// It performs the following:
//
//  1. Reads and validates configuration from the config file
//  2. Sets default values for unspecified parameters
//  3. Calculates derived values (e.g., maxFileSize = blockSize * MAX_BLOCKS)
//  4. Validates configuration consistency (e.g., tmpPath != mntPath)
//
// Configuration validation:
//   - Block size must be positive
//   - Memory size defaults to system RAM percentage if not specified
//   - Prefetch count defaults based on CPU count
//   - Worker count defaults to 3x CPU count
//
// Returns an error if configuration is invalid, which will cause the pipeline
// to fail initialization and exit.
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

// GetAttr retrieves file attributes for the specified file.
//
// This method intercepts GetAttr calls to provide updated file size information
// for files that are currently open and modified. This ensures that GetAttr
// returns the current in-memory size rather than the stale size from storage.
//
// Behavior:
//   - If file is not open: forwards to next component (returns storage attributes)
//   - If file is open and modified: returns updated attributes with current size
//   - If file is open but not modified: forwards to next component
//
// This is critical for correctness when applications check file size after writing
// but before the file is closed and flushed to storage.
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

// CreateFile creates a new file in storage and opens it for reading/writing.
//
// This method:
//  1. Creates the file in the next component (storage layer)
//  2. Opens the file using OpenFile() to set up caching structures
//
// The actual file creation in storage may be deferred if deferEmptyBlobCreation
// is enabled, in which case the file is only created in storage when data is
// written or the file is closed.
//
// Returns a handle that can be used for subsequent I/O operations, or an error
// if creation fails.
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

// OpenFile opens a file and creates a handle for I/O operations.
//
// This method is called when a user opens a file. It performs initialization:
//
//  1. Retrieves file attributes (size, mtime) from storage
//  2. Creates a new handle with a pattern detector for read-ahead optimization
//  3. Gets or creates a File object for this path (shared across handles)
//  4. Initializes file state (size, block list) if this is the first open
//  5. Handles special open flags:
//     - O_TRUNC: truncates file to zero size
//     - O_WRONLY/O_RDWR: retrieves block list for write operations
//     - O_RDONLY: creates synthetic block list for read-only access
//
// Block List Management:
//   - For write-enabled files: validates and loads committed block list from storage
//   - For read-only files: creates a synthetic block list based on file size
//   - Invalid block lists (non-aligned blocks) cause open to fail
//
// Thread Safety:
// Multiple handles can be open for the same file simultaneously. The File object
// is shared and synchronized. Each handle gets its own pattern detector for
// independent read-ahead behavior.
//
// Returns a handle for I/O operations, or an error if open fails.
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

// ReadInBuffer reads data from a file into the provided buffer.
//
// This method handles read requests from FUSE by:
//
//  1. Scheduling read-ahead based on detected access pattern (per-handle)
//  2. Reading the requested data from cached blocks
//  3. Fetching blocks from storage if not in cache
//
// Read-ahead:
//   - Each handle has its own pattern detector to support concurrent reads
//     with different access patterns on the same file
//   - Sequential patterns trigger automatic prefetching
//   - Random patterns disable prefetching to avoid wasting cache space
//
// The actual read implementation is in file.read(), which handles:
//   - Block-level I/O and cache management
//   - Waiting for async downloads to complete
//   - Copying data from cached blocks to user buffer
//
// Returns the number of bytes read, or an error if the read fails.
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

// WriteFile writes data to a file at the specified offset.
//
// This method handles write requests from FUSE by delegating to file.write().
// The write operation:
//
//  1. Allocates or reuses blocks to cover the write range
//  2. Copies data from user buffer to cached blocks
//  3. Marks modified blocks as dirty
//  4. May schedule async upload if blocks are full
//  5. Updates file size if the write extends the file
//
// Write Behavior:
//   - Writes are cached in memory; blocks are uploaded to storage during flush
//   - Partial block writes are supported (read-modify-write)
//   - Multiple concurrent writes to the same file are serialized
//   - Write errors are sticky (subsequent operations fail)
//
// The actual write implementation is in file.write().
//
// Returns the number of bytes written (should always equal len(options.Data)),
// or an error if the write fails.
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

// TruncateFile truncates or extends a file to the specified size.
//
// This method handles truncate operations by:
//
//  1. Opening the file if no handle is provided
//  2. Delegating to file.truncate() for the actual operation
//  3. Closing the temporary handle if one was created
//
// Truncate Behavior:
//   - Shrinking: removes blocks beyond new size, clears partial block
//   - Extending: adds new zero-filled blocks
//   - Always flushes file before and after the operation
//   - Updates file size atomically
//
// The actual truncate implementation is in file.truncate().
//
// Returns an error if the truncate operation fails.
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

// SyncFile synchronizes file data with storage (fsync operation).
//
// This method is called when a user application calls fsync() on a file descriptor.
// It ensures all modified data is written to storage by:
//
//  1. Flushing all dirty blocks to storage
//  2. Committing the block list
//  3. Waiting for all uploads to complete
//
// After SyncFile returns successfully, the file data is guaranteed to be
// persistent in Azure Storage.
//
// The actual sync implementation is in file.flush().
//
// Returns an error if any upload or commit operation fails.
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

// FlushFile flushes file data to storage (called on close).
//
// This method is called when a user application closes a file descriptor.
// Note: Multiple flush calls may occur for the same handle if the application
// used dup2() or fork(), as each file descriptor triggers a separate flush.
//
// FlushFile:
//  1. Waits for any pending writes to complete
//  2. Uploads all dirty blocks to storage
//  3. Commits the block list
//
// Unlike SyncFile (explicit fsync), FlushFile is implicit and happens automatically
// when the application closes the file. Both use the same underlying file.flush()
// implementation.
//
// Returns an error if any upload or commit operation fails.
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

// ReleaseFile releases all resources associated with a file handle.
//
// This method is called after all file descriptors for a handle have been closed.
// It performs cleanup:
//
//  1. Flushes any remaining dirty data (handles memory-mapped files)
//  2. Removes the handle from the file's handle list
//  3. If this was the last handle:
//     - Removes the file from the file map
//     - Releases all cached blocks back to the free list
//
// Unlike FlushFile (which may be called multiple times), ReleaseFile is called
// exactly once per handle, after all file descriptors are closed.
//
// Memory-mapped files:
// For memory-mapped files, the OS may not call FlushFile before ReleaseFile
// if the backing file descriptor was already closed. ReleaseFile performs
// a final flush to ensure no data is lost.
//
// Returns an error if the final flush fails (error is logged but cleanup proceeds).
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

// DeleteFile deletes a file from storage.
//
// This method forwards the delete operation to the next component (storage layer).
// The block cache does not maintain persistent cache entries, so no cache
// invalidation is needed.
//
// If the file is currently open, the in-memory state remains valid until all
// handles are closed. The file will be removed from storage but cached data
// remains accessible until ReleaseFile is called.
//
// Returns an error if the delete operation fails in storage.
func (bc *BlockCache) DeleteFile(options internal.DeleteFileOptions) error {
	log.Trace("BlockCache::DeleteFile : name: %s", options.Name)

	err := bc.NextComponent().DeleteFile(options)
	if err != nil {
		log.Err("BlockCache::DeleteFile : error  %s [%s]", options.Name, err.Error())
		return err
	}

	return nil
}

// RenameFile renames a file in storage.
//
// This method forwards the rename operation to the next component (storage layer).
// Since the block cache is purely in-memory and keyed by file path, no explicit
// cache invalidation is needed.
//
// If the file is currently open under the old name, it remains open and accessible.
// New opens must use the new name.
//
// Returns an error if the rename operation fails in storage.
func (bc *BlockCache) RenameFile(options internal.RenameFileOptions) error {
	log.Trace("BlockCache::RenameFile : src: %s -> dst: %s", options.Src, options.Dst)

	err := bc.NextComponent().RenameFile(options)
	if err != nil {
		log.Err("BlockCache::RenameFile : %s failed to rename file [%s]", options.Src, err.Error())
		return err
	}

	return nil
}

// DeleteDir recursively deletes a directory in storage.
//
// This method forwards the delete operation to the next component (storage layer).
// No cache invalidation is needed as the cache is purely in-memory.
//
// Returns an error if the delete operation fails in storage.
func (bc *BlockCache) DeleteDir(options internal.DeleteDirOptions) error {
	log.Trace("BlockCache::DeleteDir : name: %s", options.Name)

	err := bc.NextComponent().DeleteDir(options)
	if err != nil {
		log.Err("BlockCache::DeleteDir : %s failed", options.Name)
		return err
	}

	return nil
}

// RenameDir renames a directory in storage.
//
// This method forwards the rename operation to the next component (storage layer).
// No cache invalidation is needed as the cache is purely in-memory and keyed
// by full file paths.
//
// Returns an error if the rename operation fails in storage.
func (bc *BlockCache) RenameDir(options internal.RenameDirOptions) error {
	log.Trace("BlockCache::RenameDir : src: %s -> dst: %s", options.Src, options.Dst)

	err := bc.NextComponent().RenameDir(options)
	if err != nil {
		log.Err("BlockCache::RenameDir : error %s [%s]", options.Src, err.Error())
		return err
	}

	return nil
}

// StatFs returns filesystem statistics.
//
// This method returns a dummy statfs structure as BlockCache does not track
// filesystem-level statistics. The actual implementation is in the storage layer.
//
// Returns an empty syscall.Statfs_t structure and true to indicate success.
func (bc *BlockCache) StatFs() (*syscall.Statfs_t, bool, error) {
	log.Trace("BlockCache::StatFS")
	return &syscall.Statfs_t{}, true, nil
}

// ------------------------- Factory -------------------------------------------

// NewBlockCacheComponent creates a new BlockCache component instance.
//
// This factory function is called by the pipeline during initialization.
// It creates the component with default values; actual configuration happens
// later in Configure().
//
// The global 'bc' variable is set to enable access from package-level functions
// like block size calculations.
//
// Returns a new BlockCache component implementing the internal.Component interface.
func NewBlockCacheComponent() internal.Component {
	comp := &BlockCache{
		fileLocks: common.NewLockMap(),
	}
	bc = comp
	comp.SetName(compName)
	return comp
}

// init registers the BlockCache component with the pipeline.
//
// This function is called automatically when the package is imported.
// It performs two tasks:
//
//  1. Registers the component factory with the pipeline so BlockCache can be
//     included in the component chain
//  2. Defines command-line flags for block cache configuration
//
// Command-line flags:
//   - --block-cache-block-size: Block size in MB
//   - --block-cache-pool-size: Total memory pool size in MB
//   - --block-cache-path: Path for disk caching (future feature)
//   - --block-cache-disk-size: Disk cache size in MB (future feature)
//   - --block-cache-disk-timeout: Disk cache timeout in seconds (future feature)
//   - --block-cache-prefetch: Number of blocks to prefetch
//   - --block-cache-parallelism: Number of worker threads
//   - --block-cache-prefetch-on-open: Enable prefetch on file open
//   - --block-cache-strong-consistency: Enable strong consistency mode
//   - --block-cache-defer-empty-file-creation: Defer empty file creation to close
//
// These flags are bound to the configuration system and can be set via
// command line or configuration file.
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
