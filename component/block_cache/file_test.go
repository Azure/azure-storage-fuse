package block_cache

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/config"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/component/loopback"
	"github.com/Azure/azure-storage-fuse/v2/internal"
	"github.com/Azure/azure-storage-fuse/v2/internal/handlemap"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

// ============================================================================
// Test Suite for File read/write/flush/truncate operations
// ============================================================================

const (
	fileTestLoopbackPath = "/tmp/blobfuse_file_test_loopback"
	fileTestCachePath    = "/tmp/blobfuse_file_test_cache"
	fileTestMountPath    = "/tmp/blobfuse_file_test_mount"
)

// FileOperationsTestSuite tests read, write, flush, and truncate on File objects
// directly, exercising corner cases that are hard to reach through the high-level
// BlockCache API.
type FileOperationsTestSuite struct {
	suite.Suite
	assert     *assert.Assertions
	blockCache *BlockCache
	loopbackFS *loopback.LoopbackFS
	testPath   string
	cachePath  string
}

func (suite *FileOperationsTestSuite) SetupSuite() {
	suite.assert = assert.New(suite.T())

	testID := fmt.Sprintf("%d", time.Now().UnixNano())
	suite.testPath = filepath.Join(fileTestLoopbackPath, testID)
	suite.cachePath = filepath.Join(fileTestCachePath, testID)

	suite.assert.NoError(os.MkdirAll(suite.testPath, 0777))
	suite.assert.NoError(os.MkdirAll(suite.cachePath, 0777))

	cfg := common.LogConfig{Level: common.ELogLevel.LOG_DEBUG()}
	log.SetDefaultLogger("silent", cfg)

	configString := fmt.Sprintf(`
loopbackfs:
  path: %s
  block-size-mb: 1

block_cache:
  block-size-mb: 1
  mem-size-mb: 20
  prefetch: 4
  parallelism: 4
  path: %s
  disk-size-mb: 50
  disk-timeout-sec: 20
`, suite.testPath, suite.cachePath)

	suite.assert.NoError(config.ReadConfigFromReader(strings.NewReader(configString)))
	config.Set("mount-path", fileTestMountPath)

	suite.loopbackFS = loopback.NewLoopbackFSComponent().(*loopback.LoopbackFS)
	suite.assert.NoError(suite.loopbackFS.Configure(true))

	suite.blockCache = NewBlockCacheComponent().(*BlockCache)
	suite.blockCache.SetNextComponent(suite.loopbackFS)
	suite.assert.NoError(suite.blockCache.Configure(true))

	suite.assert.NoError(suite.loopbackFS.Start(context.Background()))
	suite.assert.NoError(suite.blockCache.Start(context.Background()))
}

func (suite *FileOperationsTestSuite) TearDownSuite() {
	if suite.blockCache != nil {
		suite.blockCache.Stop()
	}
	if suite.loopbackFS != nil {
		suite.loopbackFS.Stop()
	}
	os.RemoveAll(suite.testPath)
	os.RemoveAll(suite.cachePath)
}

// openReadFile writes content directly into the loopback storage and opens the
// file read-only through BlockCache.  The loopback block list is synthetic for
// read-only access, so no block-list validation is performed.
func (suite *FileOperationsTestSuite) openReadFile(name string, content []byte) (*handlemap.Handle, *File) {
	suite.T().Helper()
	err := os.WriteFile(filepath.Join(suite.testPath, name), content, 0777)
	suite.assert.NoError(err)

	handle, err := suite.blockCache.OpenFile(internal.OpenFileOptions{
		Name:  name,
		Flags: os.O_RDONLY,
		Mode:  0777,
	})

	suite.assert.NoError(err)
	suite.assert.NotNil(handle)
	return handle, handle.IFObj.(*blockCacheHandle).file
}

// openWriteFile creates (or primes) a file through the BlockCache pipeline and
// returns a handle that is valid for write operations.
//
//   - If content is nil/empty the file is created fresh (size 0).
//   - If content is non-empty the file is created, the content is written via
//     BlockCache, flushed, closed, and then re-opened for O_RDWR so that the
//     committed block list is available for write-mode validation.
func (suite *FileOperationsTestSuite) openWriteFile(name string, content []byte) (*handlemap.Handle, *File) {
	suite.T().Helper()
	if len(content) == 0 {
		handle, err := suite.blockCache.CreateFile(internal.CreateFileOptions{Name: name, Mode: 0777})
		suite.assert.NoError(err)
		suite.assert.NotNil(handle)
		return handle, handle.IFObj.(*blockCacheHandle).file
	}

	// Seed content through BlockCache so committed blocks exist in storage.
	seedHandle, err := suite.blockCache.CreateFile(internal.CreateFileOptions{Name: name, Mode: 0777})
	suite.assert.NoError(err)
	_, err = suite.blockCache.WriteFile(&internal.WriteFileOptions{
		Handle: seedHandle,
		Offset: 0,
		Data:   content,
	})
	suite.assert.NoError(err)
	suite.assert.NoError(suite.blockCache.SyncFile(internal.SyncFileOptions{Handle: seedHandle}))
	suite.assert.NoError(suite.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: seedHandle}))

	// Re-open for read-write; block list is now committed and passes validation.
	handle, err := suite.blockCache.OpenFile(internal.OpenFileOptions{
		Name:  name,
		Flags: os.O_RDWR,
		Mode:  0777,
	})
	suite.assert.NoError(err)
	suite.assert.NotNil(handle)
	return handle, handle.IFObj.(*blockCacheHandle).file
}

// closeFile flushes and releases a handle opened via openWriteFile.
func (suite *FileOperationsTestSuite) closeFile(handle *handlemap.Handle) {
	suite.T().Helper()
	suite.assert.NoError(suite.blockCache.FlushFile(internal.FlushFileOptions{Handle: handle}))
	suite.assert.NoError(suite.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: handle}))
}

// ============================================================================
// READ tests
// ============================================================================

// Reading at exactly fileSize should return 0 bytes and io.EOF.
func (suite *FileOperationsTestSuite) TestRead_EOFAtFileSize() {
	name := "test_read_eof_at_size.txt"
	content := []byte("hello")
	handle, f := suite.openReadFile(name, content)
	defer suite.assert.NoError(suite.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: handle}))

	buf := make([]byte, 10)
	n, err := f.read(suite.blockCache, &internal.ReadInBufferOptions{
		Handle: handle,
		Offset: int64(len(content)),
		Data:   buf,
	})
	suite.assert.Equal(0, n)
	suite.assert.Equal(io.EOF, err)
}

// Reading past fileSize should also return io.EOF immediately.
func (suite *FileOperationsTestSuite) TestRead_EOFPastFileSize() {
	name := "test_read_eof_past.txt"
	content := []byte("world")
	handle, f := suite.openReadFile(name, content)
	defer suite.assert.NoError(suite.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: handle}))

	buf := make([]byte, 10)
	n, err := f.read(suite.blockCache, &internal.ReadInBufferOptions{
		Handle: handle,
		Offset: int64(len(content)) + 100,
		Data:   buf,
	})
	suite.assert.Equal(0, n)
	suite.assert.Equal(io.EOF, err)
}

// Basic read at offset 0 should return all bytes.
func (suite *FileOperationsTestSuite) TestRead_BasicReadAtZero() {
	name := "test_read_basic.txt"
	content := []byte("Hello, World!")
	handle, f := suite.openReadFile(name, content)
	defer func() {
		suite.assert.NoError(suite.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: handle}))
	}()

	buf := make([]byte, len(content))
	n, err := f.read(suite.blockCache, &internal.ReadInBufferOptions{
		Handle: handle,
		Offset: 0,
		Data:   buf,
	})
	suite.assert.NoError(err)
	suite.assert.Equal(len(content), n)
	suite.assert.Equal(content, buf)
}

// Read with a buffer smaller than file content — only buf-capacity bytes returned.
func (suite *FileOperationsTestSuite) TestRead_BufferSmallerThanContent() {
	name := "test_read_small_buf.txt"
	content := []byte("ABCDEFGHIJ")
	handle, f := suite.openReadFile(name, content)
	defer func() {
		suite.assert.NoError(suite.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: handle}))
	}()

	buf := make([]byte, 5)
	n, err := f.read(suite.blockCache, &internal.ReadInBufferOptions{
		Handle: handle,
		Offset: 0,
		Data:   buf,
	})
	suite.assert.NoError(err)
	suite.assert.Equal(5, n)
	suite.assert.Equal(content[:5], buf)
}

// Read starting at a non-zero offset should return the correct sub-slice.
func (suite *FileOperationsTestSuite) TestRead_NonZeroOffset() {
	name := "test_read_offset.txt"
	content := []byte("0123456789")
	handle, f := suite.openReadFile(name, content)
	defer func() {
		suite.assert.NoError(suite.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: handle}))
	}()

	buf := make([]byte, 4)
	n, err := f.read(suite.blockCache, &internal.ReadInBufferOptions{
		Handle: handle,
		Offset: 3,
		Data:   buf,
	})
	suite.assert.NoError(err)
	suite.assert.Equal(4, n)
	suite.assert.Equal(content[3:7], buf)
}

// Read spanning multiple 1 MiB blocks. Write 2 MiB + a few bytes, read across boundary.
func (suite *FileOperationsTestSuite) TestRead_SpanningMultipleBlocks() {
	name := "test_read_multiblock.txt"
	blockSz := int(suite.blockCache.blockSize)

	// Build a payload: block0 fully "A", block1 first 16 bytes "B"
	data := make([]byte, blockSz+16)
	for i := range blockSz {
		data[i] = 'A'
	}
	for i := blockSz; i < len(data); i++ {
		data[i] = 'B'
	}

	err := os.WriteFile(filepath.Join(suite.testPath, name), data, 0777)
	suite.assert.NoError(err)

	handle, err := suite.blockCache.OpenFile(internal.OpenFileOptions{
		Name:  name,
		Flags: os.O_RDONLY,
		Mode:  0777,
	})
	suite.assert.NoError(err)
	defer func() {
		suite.assert.NoError(suite.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: handle}))
	}()

	f := handle.IFObj.(*blockCacheHandle).file

	// Read 32 bytes that straddle the block boundary (last 16 of block0, first 16 of block1)
	readOffset := int64(blockSz - 16)
	buf := make([]byte, 32)
	n, err := f.read(suite.blockCache, &internal.ReadInBufferOptions{
		Handle: handle,
		Offset: readOffset,
		Data:   buf,
	})
	suite.assert.NoError(err)
	suite.assert.Equal(32, n)
	suite.assert.Equal(bytes.Repeat([]byte("A"), 16), buf[:16])
	suite.assert.Equal(bytes.Repeat([]byte("B"), 16), buf[16:])
}

// numPendingReads should be updated correctly around reads.
func (suite *FileOperationsTestSuite) TestRead_PendingReadsTracking() {
	name := "test_read_pending.txt"
	content := []byte("track pending reads")
	handle, f := suite.openReadFile(name, content)
	defer func() {
		suite.assert.NoError(suite.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: handle}))
	}()

	beforeReads := f.numPendingReads.Load()

	buf := make([]byte, len(content))
	_, _ = f.read(suite.blockCache, &internal.ReadInBufferOptions{
		Handle: handle,
		Offset: 0,
		Data:   buf,
	})

	afterReads := f.numPendingReads.Load()
	suite.assert.Equal(beforeReads, afterReads, "numPendingReads must return to baseline after read")
}

// Concurrent reads from the same File should all succeed and return consistent data.
func (suite *FileOperationsTestSuite) TestRead_ConcurrentReads() {
	name := "test_read_concurrent.txt"
	content := []byte("concurrent read test data payload")
	handle, f := suite.openReadFile(name, content)
	defer func() {
		suite.assert.NoError(suite.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: handle}))
	}()

	const goroutines = 8
	var wg sync.WaitGroup
	errs := make([]error, goroutines)
	results := make([][]byte, goroutines)

	beforeReads := f.numPendingReads.Load()

	for i := range goroutines {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			buf := make([]byte, len(content))
			n, err := f.read(suite.blockCache, &internal.ReadInBufferOptions{
				Handle: handle,
				Offset: 0,
				Data:   buf,
			})
			errs[idx] = err
			if err == nil {
				results[idx] = buf[:n]
			}
		}(i)
	}
	wg.Wait()

	afterReads := f.numPendingReads.Load()
	suite.assert.Equal(beforeReads, afterReads, "numPendingReads must return to baseline after read")

	for i := range goroutines {
		suite.assert.NoError(errs[i], "goroutine %d should not error", i)
		suite.assert.Equal(content, results[i], "goroutine %d should read correct data", i)
	}
}

// ============================================================================
// WRITE tests
// ============================================================================

// Writing data that exceeds the internal max file size limit must return an error.
func (suite *FileOperationsTestSuite) TestWrite_ExceedsMaxFileSize() {
	name := "test_write_maxsize.txt"
	handle, f := suite.openWriteFile(name, nil)
	defer suite.closeFile(handle)

	largeData := make([]byte, 10)
	err := f.write(suite.blockCache, &internal.WriteFileOptions{
		Handle: handle,
		Offset: int64(suite.blockCache.maxFileSize) - 5,
		Data:   largeData,
	})
	suite.assert.Error(err, "write past maxFileSize must return an error")
	suite.assert.Contains(err.Error(), "maximum file size")
}

// If f.err is set (sticky error), all subsequent writes must fail immediately.
func (suite *FileOperationsTestSuite) TestWrite_StickyErrorPreventsWrite() {
	name := "test_write_sticky.txt"
	handle, f := suite.openWriteFile(name, nil)

	f.err.Store("injected error")

	err := f.write(suite.blockCache, &internal.WriteFileOptions{
		Handle: handle,
		Offset: 0,
		Data:   []byte("data"),
	})
	suite.assert.Error(err)
	suite.assert.Contains(err.Error(), "previous write error")

	// flush should also fail with the same error
	err = suite.blockCache.FlushFile(internal.FlushFileOptions{Handle: handle})
	suite.assert.Error(err)
	suite.assert.Contains(err.Error(), "previous write error")

	// release file also fail since it calls flush
	err = suite.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: handle})
	suite.assert.Error(err)
	suite.assert.Contains(err.Error(), "previous write error")
}

// Try to write to a file which has no blocks but have some content. generally this can
// when the blob is created outside of block cache with put-blob style upload. generally
// for these blobs open call itself would fail if the flags contain O_RDWR/O_WRONLY.
func (suite *FileOperationsTestSuite) TestWrite_PutBlobStyleFile() {
	name := "test_write_putblob.txt"
	content := []byte("content without blocks")

	// For Creating put-blob style file, just write to loopback directly without going through
	// block cache, so that block list is not valid.
	err := os.WriteFile(filepath.Join(suite.testPath, name), content, 0777)
	suite.assert.NoError(err)

	handle, err := suite.blockCache.OpenFile(internal.OpenFileOptions{
		Name:  name,
		Flags: os.O_WRONLY,
		Mode:  0777,
	})

	suite.assert.Nil(handle)
	suite.assert.Error(err)
	suite.assert.Contains(err.Error(), "Invalid Block List, not compatible with Block Cache for write operations")

	suite.assert.NoError(os.Remove(filepath.Join(suite.testPath, name)))
}

// Basic write at offset 0: file size must be updated, data readable.
func (suite *FileOperationsTestSuite) TestWrite_BasicWrite() {
	name := "test_write_basic.txt"
	handle, f := suite.openWriteFile(name, nil)

	payload := []byte("basic write payload")
	err := f.write(suite.blockCache, &internal.WriteFileOptions{
		Handle: handle,
		Offset: 0,
		Data:   payload,
	})
	suite.assert.NoError(err)
	suite.assert.Equal(int64(len(payload)), atomic.LoadInt64(&f.size))

	suite.closeFile(handle)
}

// Write at a non-zero offset should extend file size to offset+len(data).
func (suite *FileOperationsTestSuite) TestWrite_AtNonZeroOffset() {
	name := "test_write_offset.txt"
	// Seed with some content first (written through BlockCache so block list is valid)
	handle, f := suite.openWriteFile(name, []byte("AAAAAAAAAA"))

	payload := []byte("BBBB")
	offset := int64(6)
	err := f.write(suite.blockCache, &internal.WriteFileOptions{
		Handle: handle,
		Offset: offset,
		Data:   payload,
	})
	suite.assert.NoError(err)
	suite.assert.Equal(offset+int64(len(payload)), atomic.LoadInt64(&f.size))

	suite.closeFile(handle)
}

// Write that spans two blocks must write all bytes and update size correctly.
func (suite *FileOperationsTestSuite) TestWrite_SpanningBlocks() {
	name := "test_write_spanblocks.txt"
	blockSz := int(suite.blockCache.blockSize)

	handle, f := suite.openWriteFile(name, nil)

	// Write data that crosses block boundary: start 16 bytes before end of block0
	payload := bytes.Repeat([]byte("X"), 32)
	startOffset := int64(blockSz - 16)
	err := f.write(suite.blockCache, &internal.WriteFileOptions{
		Handle: handle,
		Offset: startOffset,
		Data:   payload,
	})
	suite.assert.NoError(err)
	suite.assert.Equal(startOffset+int64(len(payload)), atomic.LoadInt64(&f.size))
	suite.assert.Equal(len(f.blockList.list), 2, "should have at least 2 blocks after spanning write")

	suite.closeFile(handle)
}

// Write must mark synced=false so a subsequent flush actually uploads data.
func (suite *FileOperationsTestSuite) TestWrite_MarksSyncedFalse() {
	name := "test_write_synced.txt"
	handle, f := suite.openWriteFile(name, nil)
	defer suite.closeFile(handle)

	suite.assert.True(f.synced, "file starts synced")

	err := f.write(suite.blockCache, &internal.WriteFileOptions{
		Handle: handle,
		Offset: 0,
		Data:   []byte("dirty"),
	})
	suite.assert.NoError(err)
	suite.assert.False(f.synced, "write must mark file as not synced")
}

// Concurrent writes to the same file should all succeed without data races.
func (suite *FileOperationsTestSuite) TestWrite_ConcurrentWrites() {
	name := "test_write_concurrent.txt"
	handle, f := suite.openWriteFile(name, nil)
	defer suite.closeFile(handle)

	payload := bytes.Repeat([]byte("C"), 512)

	const goroutines = 4
	var wg sync.WaitGroup
	errs := make([]error, goroutines)

	for i := range goroutines {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			errs[idx] = f.write(suite.blockCache, &internal.WriteFileOptions{
				Handle: handle,
				Offset: int64(idx) * 512,
				Data:   payload,
			})
		}(i)
	}
	wg.Wait()

	for i := range goroutines {
		suite.assert.NoError(errs[i], "goroutine %d write should succeed", i)
	}
}

// ============================================================================
// FLUSH tests
// ============================================================================

// flush on a file whose blockList state is not blockListValid must return nil (no-op).
func (suite *FileOperationsTestSuite) TestFlush_BlockListNotValid_NoOp() {
	f := createFile("flush_invalid_state.txt")
	// state is blockListNotRetrieved by default
	f.synced = false

	err := f.flush(suite.blockCache, true)
	suite.assert.NoError(err)
}

// flush on a file that is already synced must return nil without uploading anything.
func (suite *FileOperationsTestSuite) TestFlush_AlreadySynced_NoOp() {
	name := "test_flush_synced.txt"
	handle, f := suite.openWriteFile(name, nil)
	defer suite.closeFile(handle)

	suite.assert.True(f.synced)

	err := f.flush(suite.blockCache, true)
	suite.assert.NoError(err)
	suite.assert.True(f.synced, "synced must remain true")
}

// flush should safely wait for in-flight writes and not deadlock.
func (suite *FileOperationsTestSuite) TestFlush_ConcurrentWithWrites() {
	name := "test_flush_concurrent_writes.txt"
	handle, f := suite.openWriteFile(name, nil)
	defer suite.closeFile(handle)

	const writers = 5
	start := make(chan struct{})
	var wg sync.WaitGroup
	errs := make(chan error, writers+1)
	active := atomic.Int32{}

	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			<-start
			active.Add(1)
			defer active.Add(-1)

			errs <- f.write(suite.blockCache, &internal.WriteFileOptions{
				Handle: handle,
				Offset: int64(idx) * 256,
				Data:   bytes.Repeat([]byte("W"), 256),
			})
		}(i)
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		<-start
		deadline := time.Now().Add(500 * time.Millisecond)
		for active.Load() == 0 && time.Now().Before(deadline) {
			time.Sleep(5 * time.Millisecond)
		}
		errs <- f.flush(suite.blockCache, true)
	}()

	close(start)
	wg.Wait()
	close(errs)

	for err := range errs {
		suite.assert.NoError(err)
	}
}

// flush when f.err is set must return an error immediately.
func (suite *FileOperationsTestSuite) TestFlush_StickyErrorFails() {
	name := "test_flush_sticky.txt"
	handle, f := suite.openWriteFile(name, nil)
	defer func() {
		err := suite.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: handle})
		suite.assert.Error(err)
		suite.assert.Contains(err.Error(), "previous write error")
	}()

	// Force unsynced state and inject error
	f.blockList.state = blockListValid
	f.synced = false
	f.err.Store("simulated upload error")

	err := f.flush(suite.blockCache, true)
	suite.assert.Error(err)
	suite.assert.Contains(err.Error(), "previous write error")
}

// flush after a write must commit data to storage and mark synced=true.
func (suite *FileOperationsTestSuite) TestFlush_AfterWrite_CommitsData() {
	name := "test_flush_after_write.txt"
	handle, f := suite.openWriteFile(name, nil)
	defer suite.closeFile(handle)

	payload := []byte("flush me to storage")
	err := f.write(suite.blockCache, &internal.WriteFileOptions{
		Handle: handle,
		Offset: 0,
		Data:   payload,
	})
	suite.assert.NoError(err)
	suite.assert.False(f.synced)

	err = f.flush(suite.blockCache, true)
	suite.assert.NoError(err)
	suite.assert.True(f.synced)

	// Verify the data actually landed in the loopback storage
	diskData, err := os.ReadFile(filepath.Join(suite.testPath, name))
	suite.assert.NoError(err)
	suite.assert.Equal(payload, diskData)
}

// flush of a file with no blocks (empty file) must create an empty object in storage.
func (suite *FileOperationsTestSuite) TestFlush_EmptyFile_CreatesBlob() {
	name := "test_flush_empty.txt"
	handle, f := suite.openWriteFile(name, nil)
	defer suite.closeFile(handle)

	// Mark dirty so flush doesn't short-circuit
	f.synced = false

	err := f.flush(suite.blockCache, true)
	suite.assert.NoError(err)
	suite.assert.True(f.synced)

	info, err := os.Stat(filepath.Join(suite.testPath, name))
	suite.assert.NoError(err)
	suite.assert.Equal(int64(0), info.Size())
}

// A second flush immediately after the first must be a no-op (synced=true).
// This can happen, when fd is getting duplicated using dup, dup(2), fork, etc.
// and both fds are flushed/closed independently.
func (suite *FileOperationsTestSuite) TestFlush_DoubleFlushed_SecondIsNoOp() {
	name := "test_flush_double.txt"
	handle, f := suite.openWriteFile(name, nil)
	defer suite.closeFile(handle)

	payload := []byte("double flush")
	suite.assert.NoError(f.write(suite.blockCache, &internal.WriteFileOptions{
		Handle: handle,
		Offset: 0,
		Data:   payload,
	}))

	// First flush
	suite.assert.NoError(f.flush(suite.blockCache, true))
	suite.assert.True(f.synced)

	// Second flush must pass through quickly
	suite.assert.NoError(f.flush(suite.blockCache, true))
	suite.assert.True(f.synced)
}

// Flush a multi-block file and verify all data is consistent.
func (suite *FileOperationsTestSuite) TestFlush_MultiBlock_AllDataCommitted() {
	name := "test_flush_multiblock.txt"
	handle, f := suite.openWriteFile(name, nil)
	defer suite.closeFile(handle)

	blockSz := int(suite.blockCache.blockSize)
	// Write 1.5 blocks worth of data
	half := blockSz / 2
	payload := make([]byte, blockSz+half)
	for i := range payload {
		payload[i] = byte(i % 256)
	}

	suite.assert.NoError(f.write(suite.blockCache, &internal.WriteFileOptions{
		Handle: handle,
		Offset: 0,
		Data:   payload,
	}))

	suite.assert.NoError(f.flush(suite.blockCache, true))
	suite.assert.True(f.synced)

	diskData, err := os.ReadFile(filepath.Join(suite.testPath, name))
	suite.assert.NoError(err)
	suite.assert.Equal(payload, diskData)
}

// Flush multi-block sparse file
func (suite *FileOperationsTestSuite) TestFlush_MultiBlockSparseFile() {
	name := "test_flush_multiblock.txt"
	handle, f := suite.openWriteFile(name, nil)
	defer suite.closeFile(handle)

	blockSz := int(suite.blockCache.blockSize)
	// Write 1.5 blocks worth of data
	half := blockSz / 2
	payload := make([]byte, blockSz+half)
	for i := range payload {
		payload[i] = byte(i % 256)
	}

	suite.assert.NoError(f.write(suite.blockCache, &internal.WriteFileOptions{
		Handle: handle,
		Offset: 0,
		Data:   payload,
	}))

	suite.assert.NoError(f.write(suite.blockCache, &internal.WriteFileOptions{
		Handle: handle,
		Offset: int64(blockSz*4 + 100), // write beyond current size to create a sparse region
		Data:   payload,
	}))

	suite.assert.NoError(f.flush(suite.blockCache, true))
	suite.assert.True(f.synced)

	filePayload := make([]byte, int64(blockSz*4+100)+int64(len(payload)))
	copy(filePayload[0:], payload)
	copy(filePayload[int64(blockSz*4+100):], payload)

	diskData, err := os.ReadFile(filepath.Join(suite.testPath, name))
	suite.assert.NoError(err)
	suite.assert.Equal(len(diskData), len(filePayload))
	suite.assert.Equal(filePayload, diskData)
}

// ============================================================================
// TRUNCATE tests
// ============================================================================

// Truncate to the same size should be a no-op (size unchanged, no flush needed).
func (suite *FileOperationsTestSuite) TestTruncate_SameSize_NoOp() {
	name := "test_truncate_same.txt"
	content := []byte("same size content")
	handle, f := suite.openWriteFile(name, content)
	defer suite.closeFile(handle)

	sizeBefore := atomic.LoadInt64(&f.size)
	err := f.truncate(suite.blockCache, &internal.TruncateFileOptions{
		Name:    name,
		NewSize: sizeBefore,
	})
	suite.assert.NoError(err)
	suite.assert.Equal(sizeBefore, atomic.LoadInt64(&f.size), "size must not change")
}

// Truncate with a sticky error must fail immediately.
func (suite *FileOperationsTestSuite) TestTruncate_StickyError_Fails() {
	name := "test_truncate_sticky.txt"
	content := []byte("some content")
	handle, f := suite.openWriteFile(name, content)
	defer func() {
		suite.assert.Error(suite.blockCache.FlushFile(internal.FlushFileOptions{Handle: handle}))
		suite.assert.Error(suite.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: handle}))
	}()

	f.err.Store("injected truncate error")
	err := f.truncate(suite.blockCache, &internal.TruncateFileOptions{
		Name:    name,
		NewSize: 5,
	})
	suite.assert.Error(err)
	suite.assert.Contains(err.Error(), "previous write error")
}

// Shrink truncate: size must decrease and the data beyond newSize must be gone.
func (suite *FileOperationsTestSuite) TestTruncate_Shrink() {
	name := "test_truncate_shrink.txt"
	content := []byte("Hello, World! Extra content here.")
	handle, f := suite.openWriteFile(name, content)
	defer suite.closeFile(handle)

	newSize := int64(5)
	err := f.truncate(suite.blockCache, &internal.TruncateFileOptions{
		Name:    name,
		NewSize: newSize,
	})
	suite.assert.NoError(err)
	suite.assert.Equal(newSize, atomic.LoadInt64(&f.size))

	// Verify on-disk data was truncated
	diskData, err := os.ReadFile(filepath.Join(suite.testPath, name))
	suite.assert.NoError(err)
	suite.assert.Equal(int64(len(diskData)), newSize)
	suite.assert.Equal(content[:newSize], diskData)
}

// Extend truncate: size must grow; the new region should read as zeros.
func (suite *FileOperationsTestSuite) TestTruncate_Extend() {
	name := "test_truncate_extend.txt"
	content := []byte("short")
	handle, f := suite.openWriteFile(name, content)
	defer suite.closeFile(handle)

	newSize := int64(20)
	err := f.truncate(suite.blockCache, &internal.TruncateFileOptions{
		Name:    name,
		NewSize: newSize,
	})
	suite.assert.NoError(err)
	suite.assert.Equal(newSize, atomic.LoadInt64(&f.size))

	diskData, err := os.ReadFile(filepath.Join(suite.testPath, name))
	suite.assert.NoError(err)
	suite.assert.Equal(newSize, int64(len(diskData)))
	// Original bytes preserved
	suite.assert.Equal(content, diskData[:len(content)])
	// Extension bytes are zero
	suite.assert.Equal(bytes.Repeat([]byte{0}, int(newSize)-len(content)), diskData[len(content):])
}

// Truncate to zero: block list should be empty and size should be 0.
func (suite *FileOperationsTestSuite) TestTruncate_ToZero() {
	name := "test_truncate_zero.txt"
	content := []byte("wipe me out")
	handle, f := suite.openWriteFile(name, content)
	defer suite.closeFile(handle)

	err := f.truncate(suite.blockCache, &internal.TruncateFileOptions{
		Name:    name,
		NewSize: 0,
	})
	suite.assert.NoError(err)
	suite.assert.Equal(int64(0), atomic.LoadInt64(&f.size))
	suite.assert.Equal(0, len(f.blockList.list), "block list must be empty after truncate-to-zero")

	diskData, err := os.ReadFile(filepath.Join(suite.testPath, name))
	suite.assert.NoError(err)
	suite.assert.Empty(diskData)
}

// Shrink truncate across a block boundary should remove excess blocks and
// zero out the tail of the last retained block.
func (suite *FileOperationsTestSuite) TestTruncate_ShrinkAcrossBlockBoundary() {
	name := "test_truncate_cross_block.txt"
	blockSz := int(suite.blockCache.blockSize)

	// Create a file slightly larger than one block
	data := make([]byte, blockSz+512)
	for i := range data {
		data[i] = 0xFF
	}

	handle, f := suite.openWriteFile(name, data)
	defer suite.closeFile(handle)

	// Truncate to 100 bytes — should land inside block 0
	newSize := int64(100)
	err := f.truncate(suite.blockCache, &internal.TruncateFileOptions{
		Name:    name,
		NewSize: newSize,
	})
	suite.assert.NoError(err)
	suite.assert.Equal(newSize, atomic.LoadInt64(&f.size))
	suite.assert.Equal(1, len(f.blockList.list), "only one block should remain after shrink")

	diskData, err := os.ReadFile(filepath.Join(suite.testPath, name))
	suite.assert.NoError(err)
	suite.assert.Equal(newSize, int64(len(diskData)))
	suite.assert.Equal(bytes.Repeat([]byte{0xFF}, int(newSize)), diskData)
}

// Extend by more than one block should add multiple blocks filled with zeros.
func (suite *FileOperationsTestSuite) TestTruncate_ExtendByMultipleBlocks() {
	name := "test_truncate_multiblock_extend.txt"
	content := []byte("start")
	handle, f := suite.openWriteFile(name, content)
	defer suite.closeFile(handle)

	blockSz := int(suite.blockCache.blockSize)
	newSize := int64(blockSz*2 + 100) // spans 3 blocks
	err := f.truncate(suite.blockCache, &internal.TruncateFileOptions{
		Name:    name,
		NewSize: newSize,
	})
	suite.assert.NoError(err)
	suite.assert.Equal(newSize, atomic.LoadInt64(&f.size))
	suite.assert.Equal(3, len(f.blockList.list), "should have 3 blocks after multi-block extend")

	filePayload := make([]byte, newSize)
	copy(filePayload, content)

	diskData, err := os.ReadFile(filepath.Join(suite.testPath, name))
	suite.assert.NoError(err)
	suite.assert.Equal(newSize, int64(len(diskData)))
	suite.assert.Equal(filePayload, diskData)
}

// Write to a file with sticky error must fail immediately.
func (suite *FileOperationsTestSuite) TestWrite_StickyErrorFails() {
	name := "test_write_sticky.txt"
	handle, f := suite.openWriteFile(name, nil)
	defer func() {
		err := suite.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: handle})
		suite.assert.Error(err)
	}()

	f.err.Store("injected write error")
	err := f.write(suite.blockCache, &internal.WriteFileOptions{
		Handle: handle,
		Offset: 0,
		Data:   []byte("should fail"),
	})
	suite.assert.Error(err)
	suite.assert.Contains(err.Error(), "previous write error")
}

// Write exceeding max file size must error.
func (suite *FileOperationsTestSuite) TestWrite_ExceedsMaxFileSizeReturnsError() {
	name := "test_write_overflow.txt"
	handle, f := suite.openWriteFile(name, nil)
	defer suite.closeFile(handle)

	// Write at an offset that would exceed maxFileSize
	err := f.write(suite.blockCache, &internal.WriteFileOptions{
		Handle: handle,
		Offset: int64(suite.blockCache.maxFileSize),
		Data:   []byte("x"),
	})
	suite.assert.Error(err)
	suite.assert.Contains(err.Error(), "maximum file size")
}

// Concurrent read and write on same file with data integrity.
func (suite *FileOperationsTestSuite) TestConcurrent_ReadWriteFlush() {
	name := "test_concurrent_rwf.txt"
	payload := bytes.Repeat([]byte("R"), int(suite.blockCache.blockSize))
	handle, f := suite.openWriteFile(name, payload)
	defer suite.closeFile(handle)

	const goroutines = 6
	start := make(chan struct{})
	var wg sync.WaitGroup
	errs := make([]error, goroutines)

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			<-start
			switch idx % 3 {
			case 0: // reader
				buf := make([]byte, 128)
				_, errs[idx] = f.read(suite.blockCache, &internal.ReadInBufferOptions{
					Handle: handle,
					Offset: 0,
					Data:   buf,
				})
			case 1: // writer
				errs[idx] = f.write(suite.blockCache, &internal.WriteFileOptions{
					Handle: handle,
					Offset: int64(idx) * 64,
					Data:   bytes.Repeat([]byte("W"), 64),
				})
			case 2: // flusher
				errs[idx] = f.flush(suite.blockCache, true)
			}
		}(i)
	}

	close(start)
	wg.Wait()

	for i, err := range errs {
		suite.assert.NoError(err, "goroutine %d failed", i)
	}
}

// Truncate then write then flush must produce correct data.
func (suite *FileOperationsTestSuite) TestTruncate_ThenWrite_ThenFlush() {
	name := "test_trunc_write_flush.txt"
	content := bytes.Repeat([]byte("A"), 512)
	handle, f := suite.openWriteFile(name, content)
	defer suite.closeFile(handle)

	// Truncate to 10
	suite.assert.NoError(f.truncate(suite.blockCache, &internal.TruncateFileOptions{
		Name:    name,
		NewSize: 10,
	}))

	// Write at offset 5
	suite.assert.NoError(f.write(suite.blockCache, &internal.WriteFileOptions{
		Handle: handle,
		Offset: 5,
		Data:   []byte("HELLO"),
	}))

	suite.assert.NoError(f.flush(suite.blockCache, true))

	diskData, err := os.ReadFile(filepath.Join(suite.testPath, name))
	suite.assert.NoError(err)
	suite.assert.Equal(10, len(diskData))
	suite.assert.Equal([]byte("HELLO"), diskData[5:10])
}

// Write a full block to trigger async upload, then read back to verify.
func (suite *FileOperationsTestSuite) TestWrite_FullBlockTriggersUpload() {
	name := "test_write_full_block.txt"
	handle, f := suite.openWriteFile(name, nil)
	defer suite.closeFile(handle)

	blockSz := int(suite.blockCache.blockSize)
	payload := bytes.Repeat([]byte("F"), blockSz)

	err := f.write(suite.blockCache, &internal.WriteFileOptions{
		Handle: handle,
		Offset: 0,
		Data:   payload,
	})
	suite.assert.NoError(err)
	suite.assert.Equal(int64(blockSz), atomic.LoadInt64(&f.size))

	// Flush and verify on disk
	suite.assert.NoError(f.flush(suite.blockCache, true))
	diskData, err := os.ReadFile(filepath.Join(suite.testPath, name))
	suite.assert.NoError(err)
	suite.assert.Equal(payload, diskData)
}

// Write spanning 3+ blocks then flush to exercise multi-block upload and GetOrCreateBufferDescriptor.
func (suite *FileOperationsTestSuite) TestWrite_MultiBlock_ThenFlush() {
	name := "test_write_3blocks.txt"
	handle, f := suite.openWriteFile(name, nil)
	defer suite.closeFile(handle)

	blockSz := int(suite.blockCache.blockSize)
	payload := bytes.Repeat([]byte("M"), blockSz*3+100)

	err := f.write(suite.blockCache, &internal.WriteFileOptions{
		Handle: handle,
		Offset: 0,
		Data:   payload,
	})
	suite.assert.NoError(err)
	suite.assert.Equal(int64(len(payload)), atomic.LoadInt64(&f.size))
	suite.assert.GreaterOrEqual(len(f.blockList.list), 4)

	suite.assert.NoError(f.flush(suite.blockCache, true))
	diskData, err := os.ReadFile(filepath.Join(suite.testPath, name))
	suite.assert.NoError(err)
	suite.assert.Equal(payload, diskData)
}

// Read after write on an uncommitted block should trigger flush and succeed.
func (suite *FileOperationsTestSuite) TestRead_AfterWriteUncommittedBlock() {
	name := "test_read_uncommitted.txt"
	handle, f := suite.openWriteFile(name, nil)
	defer suite.closeFile(handle)

	blockSz := int(suite.blockCache.blockSize)

	payload := bytes.Repeat([]byte("M"), blockSz)
	suite.assert.NoError(f.write(suite.blockCache, &internal.WriteFileOptions{
		Handle: handle,
		Offset: 0,
		Data:   payload,
	}))
	suite.assert.NoError(f.write(suite.blockCache, &internal.WriteFileOptions{
		Handle: handle,
		Offset: int64(blockSz),
		Data:   payload,
	}))
	suite.assert.NoError(f.write(suite.blockCache, &internal.WriteFileOptions{
		Handle: handle,
		Offset: 2 * int64(blockSz),
		Data:   payload,
	}))

	// Read back the first block - should trigger flush of all blocks and return correct data
	buf := make([]byte, len(payload))
	n, err := f.read(suite.blockCache, &internal.ReadInBufferOptions{
		Handle: handle,
		Offset: 0,
		Data:   buf,
	})

	suite.assert.NoError(err)
	suite.assert.Equal(len(payload), n)
	suite.assert.Equal(payload, buf)
}

// ============================================================================
// Entry point
// ============================================================================

func TestFileOperationsSuite(t *testing.T) {
	suite.Run(t, new(FileOperationsTestSuite))
}

func TestCreateFile(t *testing.T) {
	f := createFile("test.txt")

	assert.NotNil(t, f)
	assert.Equal(t, "test.txt", f.Name)
	assert.Equal(t, int64(-1), f.size)
	assert.Equal(t, int64(-1), f.sizeOnStorage)
	assert.True(t, f.synced)
	assert.NotNil(t, f.handles)
	assert.Equal(t, 0, len(f.handles))
	assert.NotNil(t, f.blockList)
	assert.Equal(t, int32(0), f.numPendingReads.Load())
}

func TestFileUpdateFileSize(t *testing.T) {
	f := createFile("test.txt")
	f.size = 100

	// Update to larger size
	f.updateFileSize(200)
	assert.Equal(t, int64(200), f.size)

	// Try to update to smaller size - should not change
	f.updateFileSize(150)
	assert.Equal(t, int64(200), f.size, "Size should not decrease")

	// Update to same size
	f.updateFileSize(200)
	assert.Equal(t, int64(200), f.size)

	// Update to even larger size
	f.updateFileSize(300)
	assert.Equal(t, int64(300), f.size)
}

func TestFileUpdateFileSize_Concurrent(t *testing.T) {
	f := createFile("test.txt")
	f.size = 0

	// Simulate concurrent updates
	done := make(chan bool)
	for i := 1; i <= 10; i++ {
		go func(size int64) {
			f.updateFileSize(size)
			done <- true
		}(int64(i * 100))
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Final size should be the maximum
	assert.Equal(t, int64(1000), f.size)
}

func TestFileInitialSizeIsNegative(t *testing.T) {
	f := createFile("test.txt")
	assert.Equal(t, int64(-1), f.size, "Initial size is -1, operations must handle this")
	assert.Equal(t, int64(-1), f.sizeOnStorage, "Initial sizeOnStorage is -1")
}

// Initially synced is true, as the file has no local changes compared to storage.
func TestFileInitialSyncedState(t *testing.T) {
	f := createFile("test.txt")
	assert.True(t, f.synced, "New file starts as synced=true, verify this is intended")
}

func TestFile_ErrorState(t *testing.T) {
	f := createFile("test.txt")

	// Initially no error
	assert.Nil(t, f.err.Load())

	// Store an error
	f.err.Store("test error")
	assert.NotNil(t, f.err.Load())
	assert.Equal(t, "test error", f.err.Load())
}

func TestFile_PendingReads(t *testing.T) {
	f := createFile("test.txt")

	assert.Equal(t, int32(0), f.numPendingReads.Load())

	// Simulate pending reads
	f.numPendingReads.Add(1)
	assert.Equal(t, int32(1), f.numPendingReads.Load())

	f.numPendingReads.Add(5)
	assert.Equal(t, int32(6), f.numPendingReads.Load())

	f.numPendingReads.Add(-6)
	assert.Equal(t, int32(0), f.numPendingReads.Load())
}

func TestFile_SingleBlockFilePersisted(t *testing.T) {
	f := createFile("test.txt")

	// Test initial state
	assert.False(t, f.singleBlockFilePersisted)

	// Simulate persisting as single block
	f.singleBlockFilePersisted = true
	assert.True(t, f.singleBlockFilePersisted)
}

func TestFile_BlockListInitialization(t *testing.T) {
	f := createFile("test.txt")

	assert.NotNil(t, f.blockList)
	assert.Equal(t, blockListNotRetrieved, f.blockList.state)
	assert.Equal(t, 0, len(f.blockList.list))
}

func TestFile_PendingWritersInitialization(t *testing.T) {
	f := createFile("test.txt")

	// Add and wait should work without explicit initialization
	f.pendingWriters.Add(1)
	go func() {
		f.pendingWriters.Done()
	}()
	f.pendingWriters.Wait()
	// If we reach here, it means WaitGroup works correctly
}

func TestFile_EtagField(t *testing.T) {
	f := createFile("test.txt")

	// Initially empty
	assert.Equal(t, "", f.Etag)

	// Can be set
	f.Etag = "some-etag-value"
	assert.Equal(t, "some-etag-value", f.Etag)
}

func TestFile_NameField(t *testing.T) {
	f := createFile("test-file-name.txt")
	assert.Equal(t, "test-file-name.txt", f.Name)

	// Test with path
	f2 := createFile("path/to/file.txt")
	assert.Equal(t, "path/to/file.txt", f2.Name)
}

func TestFile_SizeFields(t *testing.T) {
	f := createFile("test.txt")

	// Test independent size fields
	f.size = 1000
	f.sizeOnStorage = 500

	assert.Equal(t, int64(1000), f.size)
	assert.Equal(t, int64(500), f.sizeOnStorage)

	// These should be independent
	f.size = 2000
	assert.Equal(t, int64(2000), f.size)
	assert.Equal(t, int64(500), f.sizeOnStorage, "sizeOnStorage should not change")
}
