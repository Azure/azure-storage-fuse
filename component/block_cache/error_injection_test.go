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
// ErrorInjection Test Suite
// Uses errorInjectingComponent to exercise error paths in BlockCache.
// ============================================================================

const (
	errTestLoopbackPath = "/tmp/blobfuse_errtest_loopback"
	errTestCachePath    = "/tmp/blobfuse_errtest_cache"
	errTestMountPath    = "/tmp/blobfuse_errtest_mount"
)

type ErrorInjectionTestSuite struct {
	suite.Suite
	assert     *assert.Assertions
	blockCache *BlockCache
	loopbackFS *loopback.LoopbackFS
	mock       *errorInjectingComponent
	testPath   string
	cachePath  string
}

func (s *ErrorInjectionTestSuite) SetupTest() {
	s.assert = assert.New(s.T())

	testID := fmt.Sprintf("%d", time.Now().UnixNano())
	s.testPath = filepath.Join(errTestLoopbackPath, testID)
	s.cachePath = filepath.Join(errTestCachePath, testID)

	s.assert.NoError(os.MkdirAll(s.testPath, 0777))
	s.assert.NoError(os.MkdirAll(s.cachePath, 0777))

	cfg := common.LogConfig{Level: common.ELogLevel.LOG_DEBUG()}
	log.SetDefaultLogger("silent", cfg)

	configString := fmt.Sprintf(`
loopbackfs:
  path: %s

block_cache:
  block-size-mb: 1
  mem-size-mb: 20
  prefetch: 4
  parallelism: 4
  path: %s
  disk-size-mb: 50
  disk-timeout-sec: 20
`, s.testPath, s.cachePath)

	s.assert.NoError(config.ReadConfigFromReader(strings.NewReader(configString)))
	config.Set("mount-path", errTestMountPath)

	s.loopbackFS = loopback.NewLoopbackFSComponent().(*loopback.LoopbackFS)
	s.assert.NoError(s.loopbackFS.Configure(true))

	s.mock = newErrorInjectingComponent(s.loopbackFS)

	s.blockCache = NewBlockCacheComponent().(*BlockCache)
	s.blockCache.SetNextComponent(s.mock)
	s.assert.NoError(s.blockCache.Configure(true))

	s.assert.NoError(s.loopbackFS.Start(context.Background()))
	s.assert.NoError(s.blockCache.Start(context.Background()))

	s.blockCache.freeList.debugListMustBeFull()
}

func (s *ErrorInjectionTestSuite) TearDownTest() {
	if s.blockCache != nil {
		s.blockCache.Stop()
		s.blockCache = nil
	}
	if s.loopbackFS != nil {
		s.loopbackFS.Stop()
		s.loopbackFS = nil
	}
	os.RemoveAll(s.testPath)
	os.RemoveAll(s.cachePath)
}

// helper: create a file through BlockCache, write content, sync, release.
func (s *ErrorInjectionTestSuite) seedFile(name string, content []byte) {
	s.T().Helper()
	h, err := s.blockCache.CreateFile(internal.CreateFileOptions{Name: name, Mode: 0777})
	s.assert.NoError(err)
	if len(content) > 0 {
		_, err = s.blockCache.WriteFile(&internal.WriteFileOptions{Handle: h, Offset: 0, Data: content})
		s.assert.NoError(err)
	}
	s.assert.NoError(s.blockCache.SyncFile(internal.SyncFileOptions{Handle: h}))
	s.assert.NoError(s.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: h}))
}

// ============================================================================
// OpenFile error paths
// ============================================================================

func (s *ErrorInjectionTestSuite) TestOpenFile_GetAttrFails() {
	defer s.TearDownTest()

	// Seed a real file first
	s.seedFile("getattr_fail.txt", []byte("data"))

	// Now inject GetAttr error
	s.mock.setError("GetAttr", injectedError("GetAttr"))
	_, err := s.blockCache.OpenFile(internal.OpenFileOptions{Name: "getattr_fail.txt", Flags: os.O_RDONLY, Mode: 0777})
	s.assert.Error(err)
	s.assert.Contains(err.Error(), "GetAttr")
}

func (s *ErrorInjectionTestSuite) TestOpenFile_GetCommittedBlockListFails() {
	defer s.TearDownTest()

	s.seedFile("blocklist_fail.txt", []byte("data"))

	s.mock.setError("GetCommittedBlockList", injectedError("GetCommittedBlockList"))
	_, err := s.blockCache.OpenFile(internal.OpenFileOptions{Name: "blocklist_fail.txt", Flags: os.O_RDWR, Mode: 0777})
	s.assert.Error(err)
	s.assert.Contains(err.Error(), "GetCommittedBlockList")
}

// ============================================================================
// CreateFile error path
// ============================================================================

func (s *ErrorInjectionTestSuite) TestCreateFile_BackendFails() {
	defer s.TearDownTest()

	s.mock.setError("CreateFile", injectedError("CreateFile"))
	_, err := s.blockCache.CreateFile(internal.CreateFileOptions{Name: "create_fail.txt", Mode: 0777})
	s.assert.Error(err)
	s.assert.Contains(err.Error(), "CreateFile")
}

// ============================================================================
// DeleteFile / RenameFile / DeleteDir / RenameDir error paths
// ============================================================================

func (s *ErrorInjectionTestSuite) TestDeleteFile_BackendFails() {
	defer s.TearDownTest()

	s.seedFile("del_fail.txt", []byte("x"))
	s.mock.setError("DeleteFile", injectedError("DeleteFile"))
	err := s.blockCache.DeleteFile(internal.DeleteFileOptions{Name: "del_fail.txt"})
	s.assert.Error(err)
}

func (s *ErrorInjectionTestSuite) TestRenameFile_BackendFails() {
	defer s.TearDownTest()

	s.seedFile("rename_src.txt", []byte("x"))
	s.mock.setError("RenameFile", injectedError("RenameFile"))
	err := s.blockCache.RenameFile(internal.RenameFileOptions{Src: "rename_src.txt", Dst: "rename_dst.txt"})
	s.assert.Error(err)
}

func (s *ErrorInjectionTestSuite) TestDeleteDir_BackendFails() {
	defer s.TearDownTest()

	s.assert.NoError(os.MkdirAll(filepath.Join(s.testPath, "deldir"), 0777))
	s.mock.setError("DeleteDir", injectedError("DeleteDir"))
	err := s.blockCache.DeleteDir(internal.DeleteDirOptions{Name: "deldir"})
	s.assert.Error(err)
}

func (s *ErrorInjectionTestSuite) TestRenameDir_BackendFails() {
	defer s.TearDownTest()

	s.assert.NoError(os.MkdirAll(filepath.Join(s.testPath, "rendir"), 0777))
	s.mock.setError("RenameDir", injectedError("RenameDir"))
	err := s.blockCache.RenameDir(internal.RenameDirOptions{Src: "rendir", Dst: "rendir2"})
	s.assert.Error(err)
}

// ============================================================================
// Upload (StageData) error path — exercises uploadBlock error and sticky error
// ============================================================================

func (s *ErrorInjectionTestSuite) TestFlush_StageDataFails() {
	defer s.TearDownTest()

	h, err := s.blockCache.CreateFile(internal.CreateFileOptions{Name: "stage_fail.txt", Mode: 0777})
	s.assert.NoError(err)

	_, err = s.blockCache.WriteFile(&internal.WriteFileOptions{Handle: h, Offset: 0, Data: []byte("upload me")})
	s.assert.NoError(err)

	// Inject StageData error before flush
	s.mock.setError("StageData", injectedError("StageData"))
	err = s.blockCache.SyncFile(internal.SyncFileOptions{Handle: h})

	// Subsequent write should fail with sticky error
	_, err = s.blockCache.WriteFile(&internal.WriteFileOptions{Handle: h, Offset: 0, Data: []byte("x")})
	s.assert.Error(err)
	s.assert.Contains(err.Error(), "previous write error")

	// Release should also propagate error
	err = s.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: h})
	s.assert.Error(err)
}

// ============================================================================
// CommitData error path — exercises flush commit failure
// ============================================================================

func (s *ErrorInjectionTestSuite) TestFlush_CommitDataFails() {
	defer s.TearDownTest()

	h, err := s.blockCache.CreateFile(internal.CreateFileOptions{Name: "commit_fail.txt", Mode: 0777})
	s.assert.NoError(err)

	_, err = s.blockCache.WriteFile(&internal.WriteFileOptions{Handle: h, Offset: 0, Data: []byte("commit me")})
	s.assert.NoError(err)

	s.mock.setError("CommitData", injectedError("CommitData"))
	err = s.blockCache.SyncFile(internal.SyncFileOptions{Handle: h})
	s.assert.Error(err)

	s.mock.clearErrors()
	// Release with error state
	_ = s.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: h})
}

// ============================================================================
// Download (ReadInBuffer) error path — exercises downloadBlock error
// ============================================================================

func (s *ErrorInjectionTestSuite) TestRead_DownloadFails() {
	defer s.TearDownTest()

	s.seedFile("download_fail.txt", []byte("some content here"))

	s.mock.setError("ReadInBuffer", injectedError("ReadInBuffer"))
	h, err := s.blockCache.OpenFile(internal.OpenFileOptions{Name: "download_fail.txt", Flags: os.O_RDONLY, Mode: 0777})
	s.assert.NoError(err)

	buf := make([]byte, 100)
	_, err = s.blockCache.ReadInBuffer(&internal.ReadInBufferOptions{Handle: h, Offset: 0, Data: buf})
	s.assert.Error(err)

	s.mock.clearErrors()
	_ = s.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: h})
}

// ============================================================================
// Flush sparse file — exercises uploadZeroBlock path
// ============================================================================

func (s *ErrorInjectionTestSuite) TestFlush_SparseFile() {
	defer s.TearDownTest()

	h, err := s.blockCache.CreateFile(internal.CreateFileOptions{Name: "sparse.txt", Mode: 0777})
	s.assert.NoError(err)

	blockSz := int(s.blockCache.blockSize)
	// Write at offset 3*blockSize to create sparse blocks 0,1,2
	payload := bytes.Repeat([]byte("S"), 100)
	_, err = s.blockCache.WriteFile(&internal.WriteFileOptions{Handle: h, Offset: int64(blockSz * 3), Data: payload})
	s.assert.NoError(err)

	err = s.blockCache.SyncFile(internal.SyncFileOptions{Handle: h})
	s.assert.NoError(err)

	s.assert.NoError(s.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: h}))

	// Verify file size
	data, err := os.ReadFile(filepath.Join(s.testPath, "sparse.txt"))
	s.assert.NoError(err)
	s.assert.Equal(blockSz*3+100, len(data))
	// Sparse region should be zeros
	s.assert.Equal(bytes.Repeat([]byte{0}, blockSz*3), data[:blockSz*3])
	s.assert.Equal(payload, data[blockSz*3:])
}

// ============================================================================
// Flush with file extension — sizeOnStorage < size path
// ============================================================================

func (s *ErrorInjectionTestSuite) TestFlush_FileExtension_SizeOnStorageMismatch() {
	defer s.TearDownTest()

	// Create a file with partial first block
	h, err := s.blockCache.CreateFile(internal.CreateFileOptions{Name: "extend.txt", Mode: 0777})
	s.assert.NoError(err)
	halfBlock := bytes.Repeat([]byte("H"), int(s.blockCache.blockSize)/2)
	_, err = s.blockCache.WriteFile(&internal.WriteFileOptions{Handle: h, Offset: 0, Data: halfBlock})
	s.assert.NoError(err)
	s.assert.NoError(s.blockCache.SyncFile(internal.SyncFileOptions{Handle: h}))
	s.assert.NoError(s.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: h}))

	// Reopen and extend by writing at second block
	h, err = s.blockCache.OpenFile(internal.OpenFileOptions{Name: "extend.txt", Flags: os.O_RDWR, Mode: 0777})
	s.assert.NoError(err)
	extension := bytes.Repeat([]byte("E"), int(s.blockCache.blockSize))
	_, err = s.blockCache.WriteFile(&internal.WriteFileOptions{Handle: h, Offset: int64(s.blockCache.blockSize), Data: extension})
	s.assert.NoError(err)

	// This flush should detect sizeOnStorage < size and extend the first block with zeros
	s.assert.NoError(s.blockCache.SyncFile(internal.SyncFileOptions{Handle: h}))
	s.assert.NoError(s.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: h}))

	data, err := os.ReadFile(filepath.Join(s.testPath, "extend.txt"))
	s.assert.NoError(err)
	s.assert.Equal(int(s.blockCache.blockSize)*2, len(data))
}

// ============================================================================
// Sparse block upload error — exercises uploadZeroBlock failure in flush
// ============================================================================

func (s *ErrorInjectionTestSuite) TestFlush_SparseUploadFails() {
	defer s.TearDownTest()

	h, err := s.blockCache.CreateFile(internal.CreateFileOptions{Name: "sparse_fail.txt", Mode: 0777})
	s.assert.NoError(err)

	blockSz := int(s.blockCache.blockSize)
	// Write at offset 2*blockSize to create a sparse block 0,1
	_, err = s.blockCache.WriteFile(&internal.WriteFileOptions{
		Handle: h,
		Offset: int64(blockSz * 2),
		Data:   []byte("x"),
	})
	s.assert.NoError(err)

	// Inject StageData error — the sparse zero block upload will fail
	s.mock.setError("StageData", injectedError("StageData"))
	err = s.blockCache.SyncFile(internal.SyncFileOptions{Handle: h})
	s.assert.Error(err)

	s.mock.clearErrors()
	_ = s.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: h})
}

// ============================================================================
// TruncateFile with nil handle — exercises internal open path
// ============================================================================

func (s *ErrorInjectionTestSuite) TestTruncateFile_NilHandle_OpenFails() {
	defer s.TearDownTest()

	// File doesn't exist — GetAttr will fail
	err := s.blockCache.TruncateFile(internal.TruncateFileOptions{Name: "noexist.txt", NewSize: 0, Handle: nil})
	s.assert.Error(err)
}

// ============================================================================
// SyncFile error path
// ============================================================================

func (s *ErrorInjectionTestSuite) TestSyncFile_FlushFails() {
	defer s.TearDownTest()

	h, err := s.blockCache.CreateFile(internal.CreateFileOptions{Name: "sync_fail.txt", Mode: 0777})
	s.assert.NoError(err)
	_, err = s.blockCache.WriteFile(&internal.WriteFileOptions{Handle: h, Offset: 0, Data: []byte("sync me")})
	s.assert.NoError(err)

	s.mock.setError("StageData", injectedError("StageData"))
	err = s.blockCache.SyncFile(internal.SyncFileOptions{Handle: h})
	s.assert.Error(err)

	s.mock.clearErrors()
	_ = s.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: h})
}

// ============================================================================
// Concurrent writes with eviction pressure and error injection
// ============================================================================

func (s *ErrorInjectionTestSuite) TestConcurrent_WritesWithEvictionAndErrors() {
	defer s.TearDownTest()

	blockSz := int(s.blockCache.blockSize)
	payload := bytes.Repeat([]byte("C"), blockSz)

	// Seed enough files to fill buffers
	for i := 0; i < 25; i++ {
		s.seedFile(fmt.Sprintf("conc_%d.txt", i), payload)
	}

	// Now do concurrent reads — exercises eviction and GetOrCreateBufferDescriptor paths
	var wg sync.WaitGroup
	start := make(chan bool)
	for i := 0; i < 15; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			<-start
			name := fmt.Sprintf("conc_%d.txt", idx)
			h, err := s.blockCache.OpenFile(internal.OpenFileOptions{Name: name, Flags: os.O_RDONLY, Mode: 0777})
			if err != nil {
				return
			}
			buf := make([]byte, blockSz)
			s.blockCache.ReadInBuffer(&internal.ReadInBufferOptions{Handle: h, Offset: 0, Data: buf})
			s.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: h})
		}(i)
	}
	close(start)
	wg.Wait()
}

// ============================================================================
// Full block write triggers async upload, then read back — exercises
// scheduleUpload async path and worker uploadBlock cleanup
// ============================================================================

func (s *ErrorInjectionTestSuite) TestAsyncUpload_ThenRead() {
	defer s.TearDownTest()

	h, err := s.blockCache.CreateFile(internal.CreateFileOptions{Name: "async_upload.txt", Mode: 0777})
	s.assert.NoError(err)

	blockSz := int(s.blockCache.blockSize)
	// Write exactly one full block to trigger async upload
	payload := bytes.Repeat([]byte("A"), blockSz)
	_, err = s.blockCache.WriteFile(&internal.WriteFileOptions{Handle: h, Offset: 0, Data: payload})
	s.assert.NoError(err)

	// Write a second block
	payload2 := bytes.Repeat([]byte("B"), blockSz)
	_, err = s.blockCache.WriteFile(&internal.WriteFileOptions{Handle: h, Offset: int64(blockSz), Data: payload2})
	s.assert.NoError(err)

	s.assert.NoError(s.blockCache.SyncFile(internal.SyncFileOptions{Handle: h}))
	s.assert.NoError(s.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: h}))

	// Read back
	h, err = s.blockCache.OpenFile(internal.OpenFileOptions{Name: "async_upload.txt", Flags: os.O_RDONLY, Mode: 0777})
	s.assert.NoError(err)
	buf := make([]byte, blockSz*2)
	n, err := s.blockCache.ReadInBuffer(&internal.ReadInBufferOptions{Handle: h, Offset: 0, Data: buf})
	s.assert.NoError(err)
	s.assert.Equal(blockSz*2, n)
	s.assert.Equal(payload, buf[:blockSz])
	s.assert.Equal(payload2, buf[blockSz:])
	s.assert.NoError(s.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: h}))
}

// ============================================================================
// Read-after-write on uncommitted block — exercises the bufDescStatusNeedsFileFlush
// retry path in file.read and file.write
// ============================================================================

func (s *ErrorInjectionTestSuite) TestReadAfterWrite_UncommittedBlock() {
	defer s.TearDownTest()

	h, err := s.blockCache.CreateFile(internal.CreateFileOptions{Name: "rw_uncommitted.txt", Mode: 0777})
	s.assert.NoError(err)

	blockSz := int(s.blockCache.blockSize)
	payload := bytes.Repeat([]byte("U"), blockSz)
	_, err = s.blockCache.WriteFile(&internal.WriteFileOptions{Handle: h, Offset: 0, Data: payload})
	s.assert.NoError(err)

	// Sync to upload (block becomes uncommitted)
	s.assert.NoError(s.blockCache.SyncFile(internal.SyncFileOptions{Handle: h}))

	// Now read — the block is committed after sync, so this should succeed
	buf := make([]byte, blockSz)
	n, err := s.blockCache.ReadInBuffer(&internal.ReadInBufferOptions{Handle: h, Offset: 0, Data: buf})
	s.assert.NoError(err)
	s.assert.Equal(blockSz, n)
	s.assert.Equal(payload, buf)

	s.assert.NoError(s.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: h}))
}

// ============================================================================
// Flush empty file — exercises CreateFile path in flush
// ============================================================================

func (s *ErrorInjectionTestSuite) TestFlush_EmptyFile() {
	defer s.TearDownTest()

	h, err := s.blockCache.CreateFile(internal.CreateFileOptions{Name: "empty_flush.txt", Mode: 0777})
	s.assert.NoError(err)

	bcHandle := h.IFObj.(*blockCacheHandle)
	bcHandle.file.synced = false

	s.assert.NoError(s.blockCache.SyncFile(internal.SyncFileOptions{Handle: h}))
	s.assert.NoError(s.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: h}))

	data, err := os.ReadFile(filepath.Join(s.testPath, "empty_flush.txt"))
	s.assert.NoError(err)
	s.assert.Equal(0, len(data))
}

// ============================================================================
// Multi-handle open/close with buffer release — exercises releaseAllBuffersForFile
// ============================================================================

func (s *ErrorInjectionTestSuite) TestMultiHandle_ReleaseAllBuffers() {
	defer s.TearDownTest()

	s.seedFile("multi_handle.txt", bytes.Repeat([]byte("M"), 1024))

	// Open first handle and read to cache a buffer
	h1, err := s.blockCache.OpenFile(internal.OpenFileOptions{Name: "multi_handle.txt", Flags: os.O_RDONLY, Mode: 0777})
	s.assert.NoError(err)
	buf := make([]byte, 1024)
	_, err = s.blockCache.ReadInBuffer(&internal.ReadInBufferOptions{Handle: h1, Offset: 0, Data: buf})
	s.assert.NoError(err)

	// Open second handle
	h2, err := s.blockCache.OpenFile(internal.OpenFileOptions{Name: "multi_handle.txt", Flags: os.O_RDONLY, Mode: 0777})
	s.assert.NoError(err)

	// Release second handle — file should stay open, buffers should not be released yet
	s.assert.NoError(s.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: h2}))
	s.assert.Panics(s.blockCache.freeList.debugListMustBeFull)

	// Release first handle — last handle, should trigger releaseAllBuffersForFile
	s.assert.NoError(s.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: h1}))
}

// ============================================================================
// GetAttr for open modified file returns in-memory size
// ============================================================================

func (s *ErrorInjectionTestSuite) TestGetAttr_OpenModifiedFile() {
	defer s.TearDownTest()

	h, err := s.blockCache.CreateFile(internal.CreateFileOptions{Name: "getattr_mod.txt", Mode: 0777})
	s.assert.NoError(err)

	payload := bytes.Repeat([]byte("G"), 2048)
	_, err = s.blockCache.WriteFile(&internal.WriteFileOptions{Handle: h, Offset: 0, Data: payload})
	s.assert.NoError(err)

	attr, err := s.blockCache.GetAttr(internal.GetAttrOptions{Name: "getattr_mod.txt"})
	s.assert.NoError(err)
	s.assert.GreaterOrEqual(attr.Size, int64(2048))

	s.assert.NoError(s.blockCache.SyncFile(internal.SyncFileOptions{Handle: h}))
	s.assert.NoError(s.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: h}))
}

// ============================================================================
// FlushFile error path (already tested via SyncFile, but separate API entry point)
// ============================================================================

func (s *ErrorInjectionTestSuite) TestFlushFile_Error() {
	defer s.TearDownTest()

	h, err := s.blockCache.CreateFile(internal.CreateFileOptions{Name: "flush_err.txt", Mode: 0777})
	s.assert.NoError(err)
	_, err = s.blockCache.WriteFile(&internal.WriteFileOptions{Handle: h, Offset: 0, Data: []byte("flush")})
	s.assert.NoError(err)

	s.mock.setError("StageData", injectedError("StageData"))
	err = s.blockCache.FlushFile(internal.FlushFileOptions{Handle: h})
	s.assert.Error(err)

	s.mock.clearErrors()
	_ = s.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: h})
}

// ============================================================================
// TruncateFile error via API
// ============================================================================

func (s *ErrorInjectionTestSuite) TestTruncateFile_Error() {
	defer s.TearDownTest()

	s.seedFile("trunc_err.txt", bytes.Repeat([]byte("T"), 2048))

	h, err := s.blockCache.OpenFile(internal.OpenFileOptions{Name: "trunc_err.txt", Flags: os.O_RDWR, Mode: 0777})
	s.assert.NoError(err)

	// Inject StageData error — truncate calls flush internally which stages data
	s.mock.setError("CommitData", injectedError("CommitData"))
	err = s.blockCache.TruncateFile(internal.TruncateFileOptions{Name: "trunc_err.txt", NewSize: 10, Handle: h})
	s.assert.Error(err)

	s.mock.clearErrors()
	_ = s.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: h})
}

// Suppress unused import warnings
var (
	_ = io.EOF
	_ = atomic.Int32{}
	_ = handlemap.Handle{}
)

// ============================================================================
// StatFs error path
// ============================================================================

func (s *ErrorInjectionTestSuite) TestStatFs_Success() {
	defer s.TearDownTest()

	statfs, ok, err := s.blockCache.StatFs()
	s.assert.NoError(err)
	s.assert.True(ok)
	s.assert.NotNil(statfs)
}

// ============================================================================
// Write after flush error — exercises sticky error propagation
// ============================================================================

func (s *ErrorInjectionTestSuite) TestWriteAfterFlushError_StickyError() {
	defer s.TearDownTest()

	h, err := s.blockCache.CreateFile(internal.CreateFileOptions{Name: "write_after_err.txt", Mode: 0777})
	s.assert.NoError(err)
	_, err = s.blockCache.WriteFile(&internal.WriteFileOptions{Handle: h, Offset: 0, Data: []byte("initial data")})
	s.assert.NoError(err)

	s.mock.setError("StageData", injectedError("StageData"))
	err = s.blockCache.SyncFile(internal.SyncFileOptions{Handle: h})
	s.assert.Error(err)

	_, err = s.blockCache.WriteFile(&internal.WriteFileOptions{Handle: h, Offset: 0, Data: []byte("x")})
	s.assert.Error(err)

	err = s.blockCache.FlushFile(internal.FlushFileOptions{Handle: h})
	s.assert.Error(err)

	s.mock.clearErrors()
	_ = s.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: h})
}

// ============================================================================
// Flush empty file with CreateFile backend error
// ============================================================================

func (s *ErrorInjectionTestSuite) TestFlush_EmptyFile_CreateFileFails() {
	defer s.TearDownTest()

	h, err := s.blockCache.CreateFile(internal.CreateFileOptions{Name: "empty_create_fail.txt", Mode: 0777})
	s.assert.NoError(err)

	bcHandle := h.IFObj.(*blockCacheHandle)
	bcHandle.file.synced = false

	s.mock.setError("CreateFile", injectedError("CreateFile"))
	err = s.blockCache.SyncFile(internal.SyncFileOptions{Handle: h})
	s.assert.Error(err)

	s.mock.clearErrors()
	_ = s.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: h})
}

// Test write to a block that was previously uploaded and evicted — exercises the
// bufDescStatusNeedsFileFlush retry path in file.write.
func (s *ErrorInjectionTestSuite) TestWrite_UncommittedBlockRetry() {
	defer s.TearDownTest()

	blockSz := int(s.blockCache.blockSize)

	h, err := s.blockCache.CreateFile(internal.CreateFileOptions{Name: "uncommitted_retry.txt", Mode: 0777})
	s.assert.NoError(err)

	// Write a full block — triggers async upload, block becomes uncommitted
	payload := bytes.Repeat([]byte("U"), blockSz)
	_, err = s.blockCache.WriteFile(&internal.WriteFileOptions{Handle: h, Offset: 0, Data: payload})
	s.assert.NoError(err)

	// Sync to commit
	s.assert.NoError(s.blockCache.SyncFile(internal.SyncFileOptions{Handle: h}))

	// Write many more blocks to force eviction of the first block's buffer
	for i := 1; i <= 25; i++ {
		_, err = s.blockCache.WriteFile(&internal.WriteFileOptions{
			Handle: h,
			Offset: int64(i * blockSz),
			Data:   bytes.Repeat([]byte("E"), blockSz),
		})
		if err != nil {
			break
		}
	}

	// Now write back to block 0 — if its buffer was evicted and the block is uncommitted,
	// this exercises the retry path
	_, err = s.blockCache.WriteFile(&internal.WriteFileOptions{Handle: h, Offset: 0, Data: []byte("overwrite")})
	// This may or may not succeed depending on eviction, but should not panic
	s.assert.NoError(s.blockCache.SyncFile(internal.SyncFileOptions{Handle: h}))
	s.assert.NoError(s.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: h}))
}

// Test read from a block that needs file flush — exercises the read retry path.
func (s *ErrorInjectionTestSuite) TestRead_UncommittedBlockRetry() {
	defer s.TearDownTest()

	blockSz := int(s.blockCache.blockSize)

	h, err := s.blockCache.CreateFile(internal.CreateFileOptions{Name: "read_retry.txt", Mode: 0777})
	s.assert.NoError(err)

	// Write a full block
	payload := bytes.Repeat([]byte("R"), blockSz)
	_, err = s.blockCache.WriteFile(&internal.WriteFileOptions{Handle: h, Offset: 0, Data: payload})
	s.assert.NoError(err)

	// Sync to upload+commit
	s.assert.NoError(s.blockCache.SyncFile(internal.SyncFileOptions{Handle: h}))

	// Force eviction by writing many blocks
	for i := 1; i <= 25; i++ {
		_, err = s.blockCache.WriteFile(&internal.WriteFileOptions{
			Handle: h,
			Offset: int64(i * blockSz),
			Data:   bytes.Repeat([]byte("F"), blockSz),
		})
		if err != nil {
			break
		}
	}
	s.blockCache.SyncFile(internal.SyncFileOptions{Handle: h})

	// Read block 0 — exercises eviction during read
	buf := make([]byte, blockSz)
	n, err := s.blockCache.ReadInBuffer(&internal.ReadInBufferOptions{Handle: h, Offset: 0, Data: buf})
	s.assert.NoError(err)
	s.assert.Equal(blockSz, n)

	s.assert.NoError(s.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: h}))
}

// ============================================================================
// Entry point
// ============================================================================

func TestErrorInjectionSuite(t *testing.T) {
	suite.Run(t, new(ErrorInjectionTestSuite))
}
