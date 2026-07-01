// Copyright (c) 2026 Microsoft Corporation.
// Licensed under the MIT License.

//go:build unittest

package dist_cache

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"math/rand"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/config"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/component/block_cache"
	"github.com/Azure/azure-storage-fuse/v2/component/file_cache"
	"github.com/Azure/azure-storage-fuse/v2/component/loopback"
	"github.com/Azure/azure-storage-fuse/v2/internal"

	pb "github.com/nearora-msft/dist-cache-client-go/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"google.golang.org/protobuf/proto"
)

// ============================================================================
// Embedded mock TCP cache server (speaks the distributed cache wire protocol)
// ============================================================================

type integMockServer struct {
	listener net.Listener
	addr     string
	mu       sync.Mutex
	store    map[string][]byte // cacheKey -> data
	attrs    map[string]*pb.FileAttribute
	groups   map[string]map[string]bool // groupID -> set of cacheKeys
	locks    map[string]bool            // cacheKey -> locked
	closed   bool
}

func newIntegMockServer(t *testing.T) *integMockServer {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	s := &integMockServer{
		listener: l,
		addr:     l.Addr().String(),
		store:    make(map[string][]byte),
		attrs:    make(map[string]*pb.FileAttribute),
		groups:   make(map[string]map[string]bool),
		locks:    make(map[string]bool),
	}

	go s.serve()
	return s
}

func (s *integMockServer) serve() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}
		go s.handleConn(conn)
	}
}

func (s *integMockServer) handleConn(nc net.Conn) {
	defer nc.Close()
	for {
		var hdr [4]byte
		if _, err := io.ReadFull(nc, hdr[:]); err != nil {
			return
		}
		length := binary.BigEndian.Uint32(hdr[:])
		if length > 64*1024*1024 {
			return
		}

		buf := make([]byte, length)
		if _, err := io.ReadFull(nc, buf); err != nil {
			return
		}

		var req pb.Request
		if err := proto.Unmarshal(buf, &req); err != nil {
			return
		}

		// Read upload data if present
		var uploadData []byte
		if upload := req.GetUploadrequest(); upload != nil && upload.Filesize > 0 {
			uploadData = make([]byte, upload.Filesize)
			if _, err := io.ReadFull(nc, uploadData); err != nil {
				return
			}
		}

		respMsg, respData := s.handleRequest(&req, uploadData)

		respBytes, err := proto.Marshal(respMsg)
		if err != nil {
			return
		}
		binary.BigEndian.PutUint32(hdr[:], uint32(len(respBytes)))
		nc.Write(hdr[:])
		nc.Write(respBytes)
		if len(respData) > 0 {
			nc.Write(respData)
		}
	}
}

func (s *integMockServer) handleRequest(req *pb.Request, uploadData []byte) (proto.Message, []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()

	switch p := req.Payload.(type) {
	case *pb.Request_Uploadrequest:
		key := p.Uploadrequest.Filename
		s.store[key] = append([]byte(nil), uploadData...)
		// Track group membership
		if gid := p.Uploadrequest.GetGroupid(); len(gid) > 0 {
			gidStr := string(gid)
			if s.groups[gidStr] == nil {
				s.groups[gidStr] = make(map[string]bool)
			}
			s.groups[gidStr][key] = true
		}
		return &pb.UploadResponse{Result: pb.UploadResponse_SUCCESS}, nil

	case *pb.Request_Downloadrequest:
		key := p.Downloadrequest.Filename
		data, ok := s.store[key]
		if !ok {
			if p.Downloadrequest.Enablelock {
				if s.locks[key] {
					return &pb.DownloadResponse{
						Result: pb.DownloadResponse_NOT_FOUND_ALREADY_LOCKED,
					}, nil
				}
				s.locks[key] = true
				return &pb.DownloadResponse{
					Result: pb.DownloadResponse_NOT_FOUND_GOT_LOCK,
				}, nil
			}
			return &pb.DownloadResponse{
				Result: pb.DownloadResponse_NOT_FOUND,
			}, nil
		}
		// Handle partial downloads (offset + length)
		offset := p.Downloadrequest.Offset
		length := p.Downloadrequest.Length
		if offset >= uint64(len(data)) {
			return &pb.DownloadResponse{Result: pb.DownloadResponse_NOT_FOUND}, nil
		}
		end := offset + length
		if end > uint64(len(data)) || length == 0 {
			end = uint64(len(data))
		}
		slice := data[offset:end]
		// Clear lock on successful download
		delete(s.locks, key)
		resp := &pb.DownloadResponse{
			Result:   pb.DownloadResponse_SUCCESS,
			Filesize: uint64(len(slice)),
		}
		// Include gid metadata if tracked
		for gid, keys := range s.groups {
			if keys[key] {
				resp.Metadata = map[string][]byte{"gid": []byte(gid)}
				break
			}
		}
		return resp, slice

	case *pb.Request_Deleterequest:
		if fn := p.Deleterequest.GetFilename(); fn != "" {
			delete(s.store, fn)
			delete(s.locks, fn)
		}
		if gid := p.Deleterequest.GetGroupid(); len(gid) > 0 {
			gidStr := string(gid)
			if keys, ok := s.groups[gidStr]; ok {
				for k := range keys {
					delete(s.store, k)
					delete(s.locks, k)
				}
				delete(s.groups, gidStr)
			}
		}
		return &pb.DeleteResponse{Result: pb.DeleteResponse_SUCCESS}, nil

	case *pb.Request_Getattributerequest:
		attr, ok := s.attrs[p.Getattributerequest.Filename]
		if !ok {
			return &pb.GetAttributeResponse{
				Result: pb.GetAttributeResponse_NOT_FOUND,
			}, nil
		}
		return &pb.GetAttributeResponse{
			Result:        pb.GetAttributeResponse_SUCCESS,
			Fileattribute: attr,
		}, nil

	case *pb.Request_Putattributerequest:
		for _, fa := range p.Putattributerequest.Fileattributes {
			s.attrs[fa.Filename] = fa.Fileattribute
		}
		return &pb.PutAttributeResponse{Result: pb.PutAttributeResponse_SUCCESS}, nil

	case *pb.Request_Getcacheserversrequest:
		return &pb.GetCacheServersResponse{
			Result:          pb.GetCacheServersResponse_SUCCESS,
			Serveraddresses: []string{s.addr},
		}, nil

	default:
		return &pb.UploadResponse{Result: pb.UploadResponse_INTERNAL_ERROR}, nil
	}
}

func (s *integMockServer) chunkCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.store)
}

func (s *integMockServer) close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.closed {
		s.closed = true
		s.listener.Close()
	}
}

// ============================================================================
// Test suite: dist_cache → loopbackfs
// ============================================================================

type distCacheIntegSuite struct {
	suite.Suite
	assert *assert.Assertions

	srv          *integMockServer
	storagePath  string // loopbackfs path (simulates Azure blob)
	distCache    *DistCache
	loopbackComp internal.Component
	configString string
}

func randomStr(length int) string {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	b := make([]byte, length)
	r.Read(b)
	return fmt.Sprintf("%x", b)[:length]
}

func (suite *distCacheIntegSuite) SetupTest() {
	suite.assert = assert.New(suite.T())

	err := log.SetDefaultLogger("silent", common.LogConfig{Level: common.ELogLevel.LOG_DEBUG()})
	suite.assert.NoError(err)

	// Reset global viper config to avoid pollution from other tests
	config.ResetConfig()

	// Create temp directory for loopbackfs (simulates Azure Blob storage)
	suite.storagePath = filepath.Join(os.TempDir(), "dcache_integ_storage_"+randomStr(8))
	err = os.MkdirAll(suite.storagePath, 0777)
	suite.assert.NoError(err)

	// Start embedded mock cache server
	suite.srv = newIntegMockServer(suite.T())

	// Build config: dist_cache → loopbackfs (no file_cache, test dist_cache directly)
	suite.configString = fmt.Sprintf(
		"loopbackfs:\n  path: %s\n\ndist_cache:\n  server-list: %s\n  bypass-on-error: true\n  chunk-size-mb: 1\n  cache-prefix: test/container\n",
		suite.storagePath, suite.srv.addr)

	err = config.ReadConfigFromReader(strings.NewReader(suite.configString))
	suite.assert.NoError(err)

	// Setup loopbackfs
	suite.loopbackComp = loopback.NewLoopbackFSComponent()
	err = suite.loopbackComp.Configure(true)
	suite.assert.NoError(err)
	err = suite.loopbackComp.Start(context.Background())
	suite.assert.NoError(err)

	// Setup dist_cache
	comp := NewDistCacheComponent()
	suite.distCache = comp.(*DistCache)
	suite.distCache.SetNextComponent(suite.loopbackComp)
	err = suite.distCache.Configure(true)
	suite.assert.NoError(err)
	err = suite.distCache.Start(context.Background())
	suite.assert.NoError(err)
}

func (suite *distCacheIntegSuite) TearDownTest() {
	if suite.distCache != nil {
		_ = suite.distCache.Stop()
	}
	if suite.loopbackComp != nil {
		_ = suite.loopbackComp.Stop()
	}
	if suite.srv != nil {
		suite.srv.close()
	}
	os.RemoveAll(suite.storagePath)
}

func TestDistCacheIntegration(t *testing.T) {
	suite.Run(t, new(distCacheIntegSuite))
}

// --- Test: CopyToFile cold read (L2 miss) populates L2, second read is L2 hit ---

func (suite *distCacheIntegSuite) TestCopyToFile_ColdRead_PopulatesL2() {
	// Create a file in the loopback storage (simulates Azure blob)
	testData := []byte("hello from azure storage - integration test data!")
	fileName := "test_cold_read.txt"
	err := os.WriteFile(filepath.Join(suite.storagePath, fileName), testData, 0644)
	suite.assert.NoError(err)

	// First read: L2 miss → fetches from loopback → populates L2
	f1, err := os.CreateTemp("", "dcache-integ-cold-*")
	require.NoError(suite.T(), err)
	defer os.Remove(f1.Name())
	defer f1.Close()

	err = suite.distCache.CopyToFile(internal.CopyToFileOptions{
		Name:  fileName,
		Count: int64(len(testData)),
		File:  f1,
	})
	suite.assert.NoError(err)

	// Verify data is correct
	f1.Seek(0, 0)
	got := make([]byte, len(testData))
	n, _ := f1.Read(got)
	suite.assert.Equal(len(testData), n)
	suite.assert.Equal(testData, got)

	// Wait for async L2 population
	time.Sleep(200 * time.Millisecond)

	// Verify L2 was populated (server has stored chunks)
	suite.assert.Greater(suite.srv.chunkCount(), 0, "L2 should have chunks after cold read")

	// Second read: should hit L2 (data already in cache server)
	// Delete the source file to prove we're reading from L2
	os.Remove(filepath.Join(suite.storagePath, fileName))

	// Clear dirty flag so the second read uses L2
	suite.distCache.clearDirty(fileName)

	f2, err := os.CreateTemp("", "dcache-integ-warm-*")
	require.NoError(suite.T(), err)
	defer os.Remove(f2.Name())
	defer f2.Close()

	err = suite.distCache.CopyToFile(internal.CopyToFileOptions{
		Name:  fileName,
		Count: int64(len(testData)),
		File:  f2,
	})
	suite.assert.NoError(err)

	f2.Seek(0, 0)
	got2 := make([]byte, len(testData))
	n2, _ := f2.Read(got2)
	suite.assert.Equal(len(testData), n2)
	suite.assert.Equal(testData, got2, "second read should serve from L2 cache")
}

// --- Test: CopyFromFile write-through populates L2 ---

func (suite *distCacheIntegSuite) TestCopyFromFile_WriteThrough_PopulatesL2() {
	fileName := "test_write.txt"
	testData := []byte("written data for integration test - populates L2!")

	// Create the file in storage first (simulates existing blob)
	err := os.WriteFile(filepath.Join(suite.storagePath, fileName), []byte("old"), 0644)
	suite.assert.NoError(err)

	// Write through dist_cache
	f, err := os.CreateTemp("", "dcache-integ-write-*")
	require.NoError(suite.T(), err)
	defer os.Remove(f.Name())
	f.Write(testData)
	f.Seek(0, 0)

	err = suite.distCache.CopyFromFile(internal.CopyFromFileOptions{
		Name: fileName,
		File: f,
	})
	suite.assert.NoError(err)

	// Wait for async L2 population
	time.Sleep(300 * time.Millisecond)

	// Verify the file was written through to loopback
	stored, err := os.ReadFile(filepath.Join(suite.storagePath, fileName))
	suite.assert.NoError(err)
	suite.assert.Equal(testData, stored, "write-through should update loopback storage")

	// Verify L2 was populated
	suite.assert.Greater(suite.srv.chunkCount(), 0, "L2 should be populated after write")
}

// --- Test: DeleteFile marks dirty so subsequent reads bypass L2 ---

func (suite *distCacheIntegSuite) TestDeleteFile_BypassesL2() {
	fileName := "test_delete.txt"
	testData := []byte("data to be deleted")
	newData := []byte("new data after recreation")

	// Create file and populate L2 via a cold read
	err := os.WriteFile(filepath.Join(suite.storagePath, fileName), testData, 0644)
	suite.assert.NoError(err)

	f, err := os.CreateTemp("", "dcache-integ-del-*")
	require.NoError(suite.T(), err)
	defer os.Remove(f.Name())
	defer f.Close()

	err = suite.distCache.CopyToFile(internal.CopyToFileOptions{
		Name:  fileName,
		Count: int64(len(testData)),
		File:  f,
	})
	suite.assert.NoError(err)
	time.Sleep(200 * time.Millisecond)

	suite.assert.Greater(suite.srv.chunkCount(), 0, "L2 should have chunks")

	// Delete the file — marks it as dirty
	err = suite.distCache.DeleteFile(internal.DeleteFileOptions{Name: fileName})
	suite.assert.NoError(err)

	// Recreate the file with new data in loopback
	err = os.WriteFile(filepath.Join(suite.storagePath, fileName), newData, 0644)
	suite.assert.NoError(err)

	// Read should bypass L2 (dirty flag) and serve fresh data from loopback
	f2, err := os.CreateTemp("", "dcache-integ-del-read-*")
	require.NoError(suite.T(), err)
	defer os.Remove(f2.Name())
	defer f2.Close()

	err = suite.distCache.CopyToFile(internal.CopyToFileOptions{
		Name:  fileName,
		Count: int64(len(newData)),
		File:  f2,
	})
	suite.assert.NoError(err)

	f2.Seek(0, 0)
	got := make([]byte, len(newData))
	n, _ := f2.Read(got)
	suite.assert.Equal(len(newData), n)
	suite.assert.Equal(newData, got, "read after delete should serve fresh data, not stale L2")
}

// --- Test: ReadInBuffer (block_cache path) L2 miss then hit ---

func (suite *distCacheIntegSuite) TestReadInBuffer_L2MissThenHit() {
	fileName := "test_block_read.bin"
	// Create data that's exactly one chunk (1 MB = chunk-size-mb in config)
	chunkSize := 1 * 1024 * 1024
	testData := make([]byte, chunkSize)
	rand.Read(testData)

	err := os.WriteFile(filepath.Join(suite.storagePath, fileName), testData, 0644)
	suite.assert.NoError(err)

	// First read: L2 miss → reads from loopback → populates L2
	buf := make([]byte, chunkSize)
	n, err := suite.distCache.ReadInBuffer(&internal.ReadInBufferOptions{
		Path:   fileName,
		Offset: 0,
		Data:   buf,
		Size:   int64(chunkSize),
	})
	suite.assert.NoError(err)
	suite.assert.Equal(chunkSize, n)
	suite.assert.Equal(testData, buf[:n])

	// Wait for async L2 population
	time.Sleep(200 * time.Millisecond)

	// Delete source to prove second read is from L2
	os.Remove(filepath.Join(suite.storagePath, fileName))
	suite.distCache.clearDirty(fileName)

	// Second read: should hit L2
	buf2 := make([]byte, chunkSize)
	n2, err := suite.distCache.ReadInBuffer(&internal.ReadInBufferOptions{
		Path:   fileName,
		Offset: 0,
		Data:   buf2,
		Size:   int64(chunkSize),
	})
	suite.assert.NoError(err)
	suite.assert.Equal(chunkSize, n2)
	suite.assert.Equal(testData, buf2[:n2], "second read should serve from L2 cache")
}

// --- Test: Multi-chunk file read (CopyToFile with file larger than chunk size) ---

func (suite *distCacheIntegSuite) TestCopyToFile_MultiChunk() {
	fileName := "test_multi_chunk.bin"
	// 3.5 chunks (chunk size = 1 MB)
	chunkSize := 1 * 1024 * 1024
	fileSize := 3*chunkSize + chunkSize/2
	testData := make([]byte, fileSize)
	rand.Read(testData)

	err := os.WriteFile(filepath.Join(suite.storagePath, fileName), testData, 0644)
	suite.assert.NoError(err)

	// Read the multi-chunk file
	f, err := os.CreateTemp("", "dcache-integ-multi-*")
	require.NoError(suite.T(), err)
	defer os.Remove(f.Name())
	defer f.Close()

	err = suite.distCache.CopyToFile(internal.CopyToFileOptions{
		Name:  fileName,
		Count: int64(fileSize),
		File:  f,
	})
	suite.assert.NoError(err)

	// Verify data integrity
	f.Seek(0, 0)
	got := make([]byte, fileSize)
	n, _ := io.ReadFull(f, got)
	suite.assert.Equal(fileSize, n)
	suite.assert.Equal(testData, got, "multi-chunk read should produce correct data")

	// Wait for L2 population
	time.Sleep(300 * time.Millisecond)

	// Verify multiple chunks were stored
	suite.assert.GreaterOrEqual(suite.srv.chunkCount(), 4, "should have at least 4 chunks for 3.5 MB file with 1 MB chunks")
}

// --- Test: StageData + CommitData (block_cache write path) populates L2 ---

func (suite *distCacheIntegSuite) TestStageCommit_PopulatesL2() {
	fileName := "test_stage_commit.bin"
	chunkSize := 1 * 1024 * 1024

	// Create an existing file (so GetAttr works for ETag resolution)
	err := os.WriteFile(filepath.Join(suite.storagePath, fileName), []byte("old"), 0644)
	suite.assert.NoError(err)

	// Stage two 1MB blocks
	block0 := make([]byte, chunkSize)
	rand.Read(block0)
	block1 := make([]byte, chunkSize)
	rand.Read(block1)

	err = suite.distCache.StageData(internal.StageDataOptions{
		Name:   fileName,
		Offset: 0,
		Data:   block0,
		Id:     "block-0",
	})
	suite.assert.NoError(err)

	err = suite.distCache.StageData(internal.StageDataOptions{
		Name:   fileName,
		Offset: uint64(chunkSize),
		Data:   block1,
		Id:     "block-1",
	})
	suite.assert.NoError(err)

	// Commit
	err = suite.distCache.CommitData(internal.CommitDataOptions{
		Name:      fileName,
		List:      []string{"block-0", "block-1"},
		BlockSize: uint64(chunkSize),
	})
	suite.assert.NoError(err)

	// Wait for async flush to L2
	time.Sleep(300 * time.Millisecond)

	// Verify data was committed to loopback
	stored, err := os.ReadFile(filepath.Join(suite.storagePath, fileName))
	suite.assert.NoError(err)
	expected := append(block0, block1...)
	suite.assert.Equal(expected, stored, "committed data should match staged blocks")

	// Verify L2 was populated (blocks flushed to cache)
	suite.assert.Greater(suite.srv.chunkCount(), 0, "L2 should have chunks after commit+flush")
}

// --- Test: RenameFile marks old name dirty so reads bypass L2 ---

func (suite *distCacheIntegSuite) TestRenameFile_BypassesL2() {
	oldName := "test_rename_old.txt"
	newName := "test_rename_new.txt"
	testData := []byte("data to rename")
	newData := []byte("different content after recreate")

	// Create file and populate L2
	err := os.WriteFile(filepath.Join(suite.storagePath, oldName), testData, 0644)
	suite.assert.NoError(err)

	f, err := os.CreateTemp("", "dcache-integ-rename-*")
	require.NoError(suite.T(), err)
	defer os.Remove(f.Name())
	defer f.Close()

	err = suite.distCache.CopyToFile(internal.CopyToFileOptions{
		Name:  oldName,
		Count: int64(len(testData)),
		File:  f,
	})
	suite.assert.NoError(err)
	time.Sleep(200 * time.Millisecond)

	suite.assert.Greater(suite.srv.chunkCount(), 0, "L2 should have chunks")

	// Rename through dist_cache (which forwards to loopback)
	err = suite.distCache.RenameFile(internal.RenameFileOptions{Src: oldName, Dst: newName})
	suite.assert.NoError(err)

	// Recreate old name with different data
	err = os.WriteFile(filepath.Join(suite.storagePath, oldName), newData, 0644)
	suite.assert.NoError(err)

	// Read of old name should bypass L2 (dirty) and serve fresh data
	f2, err := os.CreateTemp("", "dcache-integ-rename-read-*")
	require.NoError(suite.T(), err)
	defer os.Remove(f2.Name())
	defer f2.Close()

	err = suite.distCache.CopyToFile(internal.CopyToFileOptions{
		Name:  oldName,
		Count: int64(len(newData)),
		File:  f2,
	})
	suite.assert.NoError(err)

	f2.Seek(0, 0)
	got := make([]byte, len(newData))
	n, _ := f2.Read(got)
	suite.assert.Equal(len(newData), n)
	suite.assert.Equal(newData, got, "read of old name should bypass stale L2 after rename")
}

// --- Test: TruncateFile marks dirty so reads bypass L2 ---

func (suite *distCacheIntegSuite) TestTruncateFile_BypassesL2() {
	fileName := "test_truncate.txt"
	testData := []byte("data that will be truncated")
	newData := []byte("short")

	// Create file and populate L2
	err := os.WriteFile(filepath.Join(suite.storagePath, fileName), testData, 0644)
	suite.assert.NoError(err)

	f, err := os.CreateTemp("", "dcache-integ-trunc-*")
	require.NoError(suite.T(), err)
	defer os.Remove(f.Name())
	defer f.Close()

	err = suite.distCache.CopyToFile(internal.CopyToFileOptions{
		Name:  fileName,
		Count: int64(len(testData)),
		File:  f,
	})
	suite.assert.NoError(err)
	time.Sleep(200 * time.Millisecond)

	suite.assert.Greater(suite.srv.chunkCount(), 0, "L2 should have chunks")

	// Truncate and rewrite
	err = suite.distCache.TruncateFile(internal.TruncateFileOptions{
		Name:    fileName,
		OldSize: int64(len(testData)),
		NewSize: 0,
	})
	suite.assert.NoError(err)

	// Put new (shorter) data in loopback
	err = os.WriteFile(filepath.Join(suite.storagePath, fileName), newData, 0644)
	suite.assert.NoError(err)

	// Read should bypass L2 (dirty) and serve the new shorter content
	f2, err := os.CreateTemp("", "dcache-integ-trunc-read-*")
	require.NoError(suite.T(), err)
	defer os.Remove(f2.Name())
	defer f2.Close()

	err = suite.distCache.CopyToFile(internal.CopyToFileOptions{
		Name:  fileName,
		Count: int64(len(newData)),
		File:  f2,
	})
	suite.assert.NoError(err)

	f2.Seek(0, 0)
	got := make([]byte, len(newData))
	n, _ := f2.Read(got)
	suite.assert.Equal(len(newData), n)
	suite.assert.Equal(newData, got, "read after truncate should serve fresh data, not stale L2")
}

// --- Test: Graceful degradation when cache server is down ---

func (suite *distCacheIntegSuite) TestGracefulDegradation_ServerDown() {
	fileName := "test_degradation.txt"
	testData := []byte("data when server is down")

	err := os.WriteFile(filepath.Join(suite.storagePath, fileName), testData, 0644)
	suite.assert.NoError(err)

	// Stop the cache server to simulate outage
	suite.srv.close()

	// Read should still succeed (bypasses to loopback)
	f, err := os.CreateTemp("", "dcache-integ-degrade-*")
	require.NoError(suite.T(), err)
	defer os.Remove(f.Name())
	defer f.Close()

	err = suite.distCache.CopyToFile(internal.CopyToFileOptions{
		Name:  fileName,
		Count: int64(len(testData)),
		File:  f,
	})
	suite.assert.NoError(err)

	f.Seek(0, 0)
	got := make([]byte, len(testData))
	n, _ := f.Read(got)
	suite.assert.Equal(len(testData), n)
	suite.assert.Equal(testData, got, "read should succeed from loopback when L2 is down")
}

// --- Test: ReadInBuffer graceful degradation ---

func (suite *distCacheIntegSuite) TestReadInBuffer_GracefulDegradation() {
	fileName := "test_block_degrade.bin"
	testData := make([]byte, 4096)
	rand.Read(testData)

	err := os.WriteFile(filepath.Join(suite.storagePath, fileName), testData, 0644)
	suite.assert.NoError(err)

	// Stop the server
	suite.srv.close()

	// Read should succeed from loopback
	buf := make([]byte, 4096)
	n, err := suite.distCache.ReadInBuffer(&internal.ReadInBufferOptions{
		Path:   fileName,
		Offset: 0,
		Data:   buf,
		Size:   int64(len(testData)),
	})
	suite.assert.NoError(err)
	suite.assert.Equal(len(testData), n)
	suite.assert.Equal(testData, buf[:n])
}

// --- Test: Dirty flag prevents stale L2 reads after write ---

func (suite *distCacheIntegSuite) TestDirtyFlag_PreventsStaleRead() {
	fileName := "test_dirty.txt"
	oldData := []byte("old content in blob")
	newData := []byte("new content after write")

	// Create initial file
	err := os.WriteFile(filepath.Join(suite.storagePath, fileName), oldData, 0644)
	suite.assert.NoError(err)

	// Read to populate L2
	f1, err := os.CreateTemp("", "dcache-integ-dirty-read1-*")
	require.NoError(suite.T(), err)
	defer os.Remove(f1.Name())
	defer f1.Close()

	err = suite.distCache.CopyToFile(internal.CopyToFileOptions{
		Name:  fileName,
		Count: int64(len(oldData)),
		File:  f1,
	})
	suite.assert.NoError(err)
	time.Sleep(200 * time.Millisecond)

	// Write new data (CopyFromFile marks dirty)
	wf, err := os.CreateTemp("", "dcache-integ-dirty-write-*")
	require.NoError(suite.T(), err)
	defer os.Remove(wf.Name())
	wf.Write(newData)
	wf.Seek(0, 0)

	err = suite.distCache.CopyFromFile(internal.CopyFromFileOptions{
		Name: fileName,
		File: wf,
	})
	suite.assert.NoError(err)

	// File is now dirty — subsequent read should bypass L2 and hit loopback
	// (which has the new data since write-through succeeded)
	f2, err := os.CreateTemp("", "dcache-integ-dirty-read2-*")
	require.NoError(suite.T(), err)
	defer os.Remove(f2.Name())
	defer f2.Close()

	err = suite.distCache.CopyToFile(internal.CopyToFileOptions{
		Name:  fileName,
		Count: int64(len(newData)),
		File:  f2,
	})
	suite.assert.NoError(err)

	f2.Seek(0, 0)
	got := make([]byte, len(newData))
	n, _ := f2.Read(got)
	suite.assert.Equal(len(newData), n)
	suite.assert.Equal(newData, got, "read after write should serve fresh data (bypassing stale L2)")
}

// --- Test: Priority returns LevelMid ---

func (suite *distCacheIntegSuite) TestPriority() {
	suite.assert.Equal(internal.EComponentPriority.LevelMid(), suite.distCache.Priority())
}

// --- Test: Config validation (no servers configured) ---
// This test is standalone (not suite-based) to avoid corrupting global config state.

func TestConfigure_NoServers_ReturnsError(t *testing.T) {
	err := log.SetDefaultLogger("silent", common.LogConfig{Level: common.ELogLevel.LOG_DEBUG()})
	require.NoError(t, err)

	// Reset global viper config to avoid pollution from other tests
	config.ResetConfig()

	// Save and restore env var
	oldEnv := os.Getenv("DIST_CACHE_SERVER_LIST")
	os.Unsetenv("DIST_CACHE_SERVER_LIST")
	defer os.Setenv("DIST_CACHE_SERVER_LIST", oldEnv)

	storagePath := filepath.Join(os.TempDir(), "dcache_integ_noservers_"+randomStr(8))
	err = os.MkdirAll(storagePath, 0777)
	require.NoError(t, err)
	defer os.RemoveAll(storagePath)

	badConfig := fmt.Sprintf("loopbackfs:\n  path: %s\n\ndist_cache:\n  bypass-on-error: true\n", storagePath)
	err = config.ReadConfigFromReader(strings.NewReader(badConfig))
	require.NoError(t, err)

	lb := loopback.NewLoopbackFSComponent()
	_ = lb.Configure(true)

	comp := NewDistCacheComponent()
	dc := comp.(*DistCache)
	dc.SetNextComponent(lb)

	err = dc.Configure(true)
	assert.Error(t, err, "Configure should fail when no server discovery is configured")
	assert.Contains(t, err.Error(), "no server discovery configured")
}

// --- Test: Chunk size resolution from config ---

func (suite *distCacheIntegSuite) TestChunkSize_FromConfig() {
	// The suite uses chunk-size-mb: 1
	suite.assert.Equal(int64(1*1024*1024), suite.distCache.chunkSize)
}

// ============================================================================
// Test suite: file_cache → dist_cache → loopbackfs (full pipeline)
// ============================================================================

type fileCacheDistCacheSuite struct {
	suite.Suite
	assert *assert.Assertions

	srv          *integMockServer
	storagePath  string             // loopbackfs path (simulates Azure blob)
	cachePath    string             // file_cache local cache directory
	fileCache    internal.Component // file_cache (top of pipeline)
	distCache    *DistCache         // dist_cache (middle)
	loopbackComp internal.Component // loopbackfs (bottom, simulates azstorage)
}

func (suite *fileCacheDistCacheSuite) SetupTest() {
	suite.assert = assert.New(suite.T())

	err := log.SetDefaultLogger("silent", common.LogConfig{Level: common.ELogLevel.LOG_DEBUG()})
	suite.assert.NoError(err)

	// Reset global viper config to avoid pollution from other tests
	config.ResetConfig()

	rand := randomStr(8)
	suite.storagePath = filepath.Join(os.TempDir(), "fdc_storage_"+rand)
	suite.cachePath = filepath.Join(os.TempDir(), "fdc_cache_"+rand)
	err = os.MkdirAll(suite.storagePath, 0777)
	suite.assert.NoError(err)
	os.RemoveAll(suite.cachePath) // file_cache creates this itself

	// Start embedded mock cache server
	suite.srv = newIntegMockServer(suite.T())

	// Build config: file_cache → dist_cache → loopbackfs
	// timeout-sec must be > 0 so the local cache file survives long enough for
	// dist_cache's async L2 populate goroutine to re-open it after Release.
	// Keep it small so tests that need to observe eviction can sleep through it.
	cfg := fmt.Sprintf(
		"file_cache:\n  path: %s\n  offload-io: true\n  timeout-sec: 2\n\n"+
			"dist_cache:\n  server-list: %s\n  bypass-on-error: true\n  chunk-size-mb: 1\n  cache-prefix: test/container\n\n"+
			"loopbackfs:\n  path: %s\n",
		suite.cachePath, suite.srv.addr, suite.storagePath)

	err = config.ReadConfigFromReader(strings.NewReader(cfg))
	suite.assert.NoError(err)

	// Build pipeline bottom-up: loopback → dist_cache → file_cache
	suite.loopbackComp = loopback.NewLoopbackFSComponent()
	err = suite.loopbackComp.Configure(true)
	suite.assert.NoError(err)

	comp := NewDistCacheComponent()
	suite.distCache = comp.(*DistCache)
	suite.distCache.SetNextComponent(suite.loopbackComp)
	err = suite.distCache.Configure(true)
	suite.assert.NoError(err)

	suite.fileCache = file_cache.NewFileCacheComponent()
	suite.fileCache.SetNextComponent(suite.distCache)
	err = suite.fileCache.Configure(true)
	suite.assert.NoError(err)

	// Start bottom-up
	err = suite.loopbackComp.Start(context.Background())
	suite.assert.NoError(err)
	err = suite.distCache.Start(context.Background())
	suite.assert.NoError(err)
	err = suite.fileCache.Start(context.Background())
	suite.assert.NoError(err)
}

func (suite *fileCacheDistCacheSuite) TearDownTest() {
	if suite.fileCache != nil {
		_ = suite.fileCache.Stop()
	}
	if suite.distCache != nil {
		_ = suite.distCache.Stop()
	}
	if suite.loopbackComp != nil {
		_ = suite.loopbackComp.Stop()
	}
	if suite.srv != nil {
		suite.srv.close()
	}
	os.RemoveAll(suite.storagePath)
	os.RemoveAll(suite.cachePath)
}

func TestFileCacheDistCachePipeline(t *testing.T) {
	suite.Run(t, new(fileCacheDistCacheSuite))
}

// --- Test: Read through full pipeline (cold: L1 miss → L2 miss → loopback) ---

func (suite *fileCacheDistCacheSuite) TestRead_ColdPath() {
	fileName := "read_cold.txt"
	testData := []byte("hello from azure via the full pipeline!")

	// Place file in fake storage (simulates blob in Azure)
	err := os.WriteFile(filepath.Join(suite.storagePath, fileName), testData, 0644)
	suite.assert.NoError(err)

	// Open through file_cache → triggers CopyToFile → dist_cache → loopback
	handle, err := suite.fileCache.OpenFile(internal.OpenFileOptions{Name: fileName, Mode: 0777})
	require.NoError(suite.T(), err)

	// Read through file_cache
	buf := make([]byte, len(testData))
	n, err := suite.fileCache.ReadInBuffer(&internal.ReadInBufferOptions{Handle: handle, Offset: 0, Data: buf})
	suite.assert.NoError(err)
	suite.assert.Equal(len(testData), n)
	suite.assert.Equal(testData, buf[:n])

	err = suite.fileCache.ReleaseFile(internal.ReleaseFileOptions{Handle: handle})
	suite.assert.NoError(err)

	// Wait for async L2 population
	time.Sleep(200 * time.Millisecond)

	// L2 should be populated
	suite.assert.Greater(suite.srv.chunkCount(), 0, "L2 should have chunks after read through full pipeline")
}

// --- Test: Write through full pipeline (file_cache → flush → dist_cache → loopback) ---

func (suite *fileCacheDistCacheSuite) TestWrite_FlushToStorage() {
	fileName := "write_flush.txt"
	testData := []byte("written through full pipeline and flushed!")

	// Create file through file_cache
	handle, err := suite.fileCache.CreateFile(internal.CreateFileOptions{Name: fileName, Mode: 0777})
	require.NoError(suite.T(), err)

	// Write data
	_, err = suite.fileCache.WriteFile(&internal.WriteFileOptions{Handle: handle, Offset: 0, Data: testData})
	suite.assert.NoError(err)

	// Flush → file_cache calls CopyFromFile → dist_cache → loopback
	err = suite.fileCache.FlushFile(internal.FlushFileOptions{Handle: handle})
	suite.assert.NoError(err)

	err = suite.fileCache.ReleaseFile(internal.ReleaseFileOptions{Handle: handle})
	suite.assert.NoError(err)

	// Verify data arrived in loopback (fake storage) — this is the critical
	// assertion: write-through from file_cache → dist_cache → loopback works.
	stored, err := os.ReadFile(filepath.Join(suite.storagePath, fileName))
	suite.assert.NoError(err)
	suite.assert.Equal(testData, stored, "write should propagate through dist_cache to storage")

	// Wait for async L2 population
	time.Sleep(300 * time.Millisecond)

	// Verify L2 was populated
	suite.assert.Greater(suite.srv.chunkCount(), 0, "L2 should have chunks after write through full pipeline")
}

// --- Test: Multi-chunk write through full pipeline ---

func (suite *fileCacheDistCacheSuite) TestWrite_MultiChunkFlush() {
	fileName := "write_multi_chunk.bin"
	// 2.5 chunks (chunk-size-mb: 1)
	chunkSize := 1 * 1024 * 1024
	fileSize := 2*chunkSize + chunkSize/2
	testData := make([]byte, fileSize)
	rand.Read(testData)

	// Create file through file_cache
	handle, err := suite.fileCache.CreateFile(internal.CreateFileOptions{Name: fileName, Mode: 0777})
	require.NoError(suite.T(), err)

	// Write multi-chunk data
	_, err = suite.fileCache.WriteFile(&internal.WriteFileOptions{Handle: handle, Offset: 0, Data: testData})
	suite.assert.NoError(err)

	// Flush → file_cache calls CopyFromFile → dist_cache → loopback
	err = suite.fileCache.FlushFile(internal.FlushFileOptions{Handle: handle})
	suite.assert.NoError(err)

	err = suite.fileCache.ReleaseFile(internal.ReleaseFileOptions{Handle: handle})
	suite.assert.NoError(err)

	// Verify all data arrived in loopback storage
	stored, err := os.ReadFile(filepath.Join(suite.storagePath, fileName))
	suite.assert.NoError(err)
	suite.assert.Equal(testData, stored, "multi-chunk write should propagate through dist_cache to storage")

	// Wait for async L2 population
	time.Sleep(300 * time.Millisecond)

	// Verify multiple chunks were stored in L2
	suite.assert.GreaterOrEqual(suite.srv.chunkCount(), 3, "should have at least 3 chunks for 2.5 MB file with 1 MB chunks")
}

// --- Test: Delete through pipeline → dirty flag prevents stale L2 read ---

func (suite *fileCacheDistCacheSuite) TestDelete_PreventsStaleL2Read() {
	fileName := "delete_stale.txt"
	testData := []byte("data that will be deleted then recreated")
	newData := []byte("recreated data after delete")

	// Create file through file_cache pipeline to populate L2
	handle, err := suite.fileCache.CreateFile(internal.CreateFileOptions{Name: fileName, Mode: 0777})
	require.NoError(suite.T(), err)
	_, err = suite.fileCache.WriteFile(&internal.WriteFileOptions{Handle: handle, Offset: 0, Data: testData})
	suite.assert.NoError(err)
	err = suite.fileCache.FlushFile(internal.FlushFileOptions{Handle: handle})
	suite.assert.NoError(err)
	err = suite.fileCache.ReleaseFile(internal.ReleaseFileOptions{Handle: handle})
	suite.assert.NoError(err)
	time.Sleep(200 * time.Millisecond)

	// Delete through the pipeline
	err = suite.fileCache.DeleteFile(internal.DeleteFileOptions{Name: fileName})
	suite.assert.NoError(err)

	// Recreate with new data
	err = os.WriteFile(filepath.Join(suite.storagePath, fileName), newData, 0644)
	suite.assert.NoError(err)

	// Read again — should bypass stale L2 and serve fresh data from loopback
	h2, err := suite.fileCache.OpenFile(internal.OpenFileOptions{Name: fileName, Mode: 0777})
	require.NoError(suite.T(), err)

	buf2 := make([]byte, len(newData))
	n, err := suite.fileCache.ReadInBuffer(&internal.ReadInBufferOptions{Handle: h2, Offset: 0, Data: buf2})
	suite.assert.NoError(err)
	suite.assert.Equal(len(newData), n)
	suite.assert.Equal(newData, buf2[:n], "read after delete should serve fresh data, not stale L2")

	err = suite.fileCache.ReleaseFile(internal.ReleaseFileOptions{Handle: h2})
	suite.assert.NoError(err)
}

// --- Test: Rename through pipeline ---

func (suite *fileCacheDistCacheSuite) TestRename_ThroughPipeline() {
	oldName := "rename_old.txt"
	newName := "rename_new.txt"
	testData := []byte("data that will be renamed")

	// Create file through file_cache pipeline
	handle, err := suite.fileCache.CreateFile(internal.CreateFileOptions{Name: oldName, Mode: 0777})
	require.NoError(suite.T(), err)
	_, err = suite.fileCache.WriteFile(&internal.WriteFileOptions{Handle: handle, Offset: 0, Data: testData})
	suite.assert.NoError(err)
	err = suite.fileCache.FlushFile(internal.FlushFileOptions{Handle: handle})
	suite.assert.NoError(err)
	err = suite.fileCache.ReleaseFile(internal.ReleaseFileOptions{Handle: handle})
	suite.assert.NoError(err)
	time.Sleep(200 * time.Millisecond)

	// Rename through pipeline
	err = suite.fileCache.RenameFile(internal.RenameFileOptions{Src: oldName, Dst: newName})
	suite.assert.NoError(err)

	// Verify renamed file is accessible under new name
	h2, err := suite.fileCache.OpenFile(internal.OpenFileOptions{Name: newName, Mode: 0777})
	require.NoError(suite.T(), err)

	buf2 := make([]byte, len(testData))
	n, err := suite.fileCache.ReadInBuffer(&internal.ReadInBufferOptions{Handle: h2, Offset: 0, Data: buf2})
	suite.assert.NoError(err)
	suite.assert.Equal(len(testData), n)
	suite.assert.Equal(testData, buf2[:n], "renamed file should be readable under new name")

	err = suite.fileCache.ReleaseFile(internal.ReleaseFileOptions{Handle: h2})
	suite.assert.NoError(err)
}

// --- Test: Graceful degradation (cache server down, full pipeline still works) ---

func (suite *fileCacheDistCacheSuite) TestGracefulDegradation_ServerDown() {
	fileName := "degrade_pipeline.txt"
	testData := []byte("data when L2 is completely unavailable")

	err := os.WriteFile(filepath.Join(suite.storagePath, fileName), testData, 0644)
	suite.assert.NoError(err)

	// Kill the cache server
	suite.srv.close()

	// Read through full pipeline should still work (L2 bypassed → loopback)
	h, err := suite.fileCache.OpenFile(internal.OpenFileOptions{Name: fileName, Mode: 0777})
	require.NoError(suite.T(), err)

	buf := make([]byte, len(testData))
	n, err := suite.fileCache.ReadInBuffer(&internal.ReadInBufferOptions{Handle: h, Offset: 0, Data: buf})
	suite.assert.NoError(err)
	suite.assert.Equal(len(testData), n)
	suite.assert.Equal(testData, buf[:n], "pipeline should serve data from loopback when L2 is down")

	err = suite.fileCache.ReleaseFile(internal.ReleaseFileOptions{Handle: h})
	suite.assert.NoError(err)
}

// --- Test: Multi-chunk file through full pipeline ---

func (suite *fileCacheDistCacheSuite) TestRead_MultiChunk_FullPipeline() {
	fileName := "multi_chunk_pipeline.bin"
	// 2.5 chunks (chunk-size-mb: 1)
	chunkSize := 1 * 1024 * 1024
	fileSize := 2*chunkSize + chunkSize/2
	testData := make([]byte, fileSize)
	rand.Read(testData)

	err := os.WriteFile(filepath.Join(suite.storagePath, fileName), testData, 0644)
	suite.assert.NoError(err)

	// Read through the full pipeline
	h, err := suite.fileCache.OpenFile(internal.OpenFileOptions{Name: fileName, Mode: 0777})
	require.NoError(suite.T(), err)

	buf := make([]byte, fileSize)
	n, err := suite.fileCache.ReadInBuffer(&internal.ReadInBufferOptions{Handle: h, Offset: 0, Data: buf})
	suite.assert.NoError(err)
	suite.assert.Equal(fileSize, n)
	suite.assert.Equal(testData, buf[:n], "multi-chunk file should be read correctly through full pipeline")

	err = suite.fileCache.ReleaseFile(internal.ReleaseFileOptions{Handle: h})
	suite.assert.NoError(err)

	// Wait for L2 population
	time.Sleep(300 * time.Millisecond)
	suite.assert.GreaterOrEqual(suite.srv.chunkCount(), 3, "should have at least 3 chunks for 2.5 MB file")
}

// --- Test: Write then read-back through pipeline ---

func (suite *fileCacheDistCacheSuite) TestWriteThenRead_FullPipeline() {
	fileName := "write_read.txt"
	testData := []byte("round-trip through file_cache → dist_cache → loopback and back!")

	// Write via file_cache
	handle, err := suite.fileCache.CreateFile(internal.CreateFileOptions{Name: fileName, Mode: 0777})
	require.NoError(suite.T(), err)
	_, err = suite.fileCache.WriteFile(&internal.WriteFileOptions{Handle: handle, Offset: 0, Data: testData})
	suite.assert.NoError(err)
	err = suite.fileCache.FlushFile(internal.FlushFileOptions{Handle: handle})
	suite.assert.NoError(err)
	err = suite.fileCache.ReleaseFile(internal.ReleaseFileOptions{Handle: handle})
	suite.assert.NoError(err)

	// Wait for L1 eviction and L2 population. file_cache's eviction worker ticks
	// every timeout-sec (2s) and removes files idle for >= timeout-sec, so worst
	// case is ~2x timeout-sec. Poll up to ~5s and assert eviction actually happened.
	localPath := filepath.Join(suite.cachePath, fileName)
	evicted := false
	for i := 0; i < 25; i++ {
		if _, err := os.Stat(localPath); os.IsNotExist(err) {
			evicted = true
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	suite.assert.True(evicted, "local cache file should be evicted within ~5s")

	// Clear dirty so L2 is used
	suite.distCache.clearDirty(fileName)

	// Read back — should come from L2 or loopback
	h, err := suite.fileCache.OpenFile(internal.OpenFileOptions{Name: fileName, Mode: 0777})
	require.NoError(suite.T(), err)

	buf := make([]byte, len(testData))
	n, err := suite.fileCache.ReadInBuffer(&internal.ReadInBufferOptions{Handle: h, Offset: 0, Data: buf})
	suite.assert.NoError(err)
	suite.assert.Equal(len(testData), n)
	suite.assert.Equal(testData, buf[:n], "data should survive full write → evict → read round-trip")

	err = suite.fileCache.ReleaseFile(internal.ReleaseFileOptions{Handle: h})
	suite.assert.NoError(err)
}

// ============================================================================
// Test suite: block_cache → dist_cache → loopbackfs (full pipeline)
// ============================================================================

var homedir, _ = os.UserHomeDir()
var mntpoint = homedir + "/mountpoint"

type blockCacheDistCacheSuite struct {
	suite.Suite
	assert *assert.Assertions

	srv          *integMockServer
	storagePath  string             // loopbackfs path (simulates Azure blob)
	diskPath     string             // block_cache disk cache directory
	blockCache   internal.Component // block_cache (top of pipeline)
	distCache    *DistCache         // dist_cache (middle)
	loopbackComp internal.Component // loopbackfs (bottom, simulates azstorage)
}

func (suite *blockCacheDistCacheSuite) SetupTest() {
	suite.assert = assert.New(suite.T())

	err := log.SetDefaultLogger("silent", common.LogConfig{Level: common.ELogLevel.LOG_DEBUG()})
	suite.assert.NoError(err)

	// Reset global viper config to avoid pollution from other tests
	config.ResetConfig()

	rand := randomStr(8)
	suite.storagePath = filepath.Join(os.TempDir(), "bdc_storage_"+rand)
	suite.diskPath = filepath.Join(os.TempDir(), "bdc_disk_"+rand)
	err = os.MkdirAll(suite.storagePath, 0777)
	suite.assert.NoError(err)
	os.RemoveAll(suite.diskPath) // block_cache creates this itself

	// Start embedded mock cache server
	suite.srv = newIntegMockServer(suite.T())

	// Build config: block_cache → dist_cache → loopbackfs
	// block-size-mb: 1 to match loopback's GetCommittedBlockList (hardcoded 1MB)
	cfg := fmt.Sprintf(
		"read-only: true\n\n"+
			"block_cache:\n  block-size-mb: 1\n  mem-size-mb: 20\n  prefetch: 12\n  parallelism: 10\n  path: %s\n  disk-size-mb: 50\n  disk-timeout-sec: 20\n\n"+
			"dist_cache:\n  server-list: %s\n  bypass-on-error: true\n  chunk-size-mb: 1\n  cache-prefix: test/container\n\n"+
			"loopbackfs:\n  path: %s\n",
		suite.diskPath, suite.srv.addr, suite.storagePath)

	err = config.ReadConfigFromReader(strings.NewReader(cfg))
	suite.assert.NoError(err)
	config.Set("mount-path", mntpoint)

	// Build pipeline bottom-up: loopback → dist_cache → block_cache
	suite.loopbackComp = loopback.NewLoopbackFSComponent()
	err = suite.loopbackComp.Configure(true)
	suite.assert.NoError(err)

	comp := NewDistCacheComponent()
	suite.distCache = comp.(*DistCache)
	suite.distCache.SetNextComponent(suite.loopbackComp)
	err = suite.distCache.Configure(true)
	suite.assert.NoError(err)

	suite.blockCache = block_cache.NewBlockCacheComponent()
	suite.blockCache.SetNextComponent(suite.distCache)
	err = suite.blockCache.Configure(true)
	if err != nil {
		suite.T().Skipf("block_cache configure failed (likely low memory): %v", err)
		return
	}

	// Start bottom-up
	err = suite.loopbackComp.Start(context.Background())
	suite.assert.NoError(err)
	err = suite.distCache.Start(context.Background())
	suite.assert.NoError(err)
	err = suite.blockCache.Start(context.Background())
	suite.assert.NoError(err)
}

func (suite *blockCacheDistCacheSuite) TearDownTest() {
	if suite.blockCache != nil {
		_ = suite.blockCache.Stop()
	}
	if suite.distCache != nil {
		_ = suite.distCache.Stop()
	}
	if suite.loopbackComp != nil {
		_ = suite.loopbackComp.Stop()
	}
	if suite.srv != nil {
		suite.srv.close()
	}
	os.RemoveAll(suite.storagePath)
	os.RemoveAll(suite.diskPath)
}

func TestBlockCacheDistCachePipeline(t *testing.T) {
	suite.Run(t, new(blockCacheDistCacheSuite))
}

// --- Test: Read through full pipeline (block_cache → dist_cache L2 miss → loopback) ---

func (suite *blockCacheDistCacheSuite) TestRead_ColdPath() {
	fileName := "bc_read_cold.bin"
	testData := make([]byte, 2*1024*1024) // 2 MB = 2 blocks
	rand.Read(testData)

	err := os.WriteFile(filepath.Join(suite.storagePath, fileName), testData, 0644)
	suite.assert.NoError(err)

	// Open through block_cache → GetAttr → dist_cache → loopback
	h, err := suite.blockCache.OpenFile(internal.OpenFileOptions{Name: fileName})
	require.NoError(suite.T(), err)
	suite.assert.Equal(int64(len(testData)), h.Size)

	// Read all data through block_cache → ReadInBuffer → dist_cache → loopback
	buf := make([]byte, len(testData))
	offset := 0
	for offset < len(testData) {
		n, err := suite.blockCache.ReadInBuffer(&internal.ReadInBufferOptions{
			Handle: h, Offset: int64(offset), Data: buf[offset:],
		})
		if n > 0 {
			offset += n
		}
		if err != nil {
			break
		}
	}
	suite.assert.Equal(len(testData), offset)
	suite.assert.Equal(testData, buf, "block_cache should serve correct data through dist_cache pipeline")

	err = suite.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: h})
	suite.assert.NoError(err)

	// Wait for async L2 population
	time.Sleep(300 * time.Millisecond)

	// L2 should be populated (ReadInBuffer path → dist_cache → L2 upload)
	suite.assert.Greater(suite.srv.chunkCount(), 0, "L2 should have chunks after block_cache read")
}

// --- Test: Read from L2 (block_cache → dist_cache L2 hit) ---

func (suite *blockCacheDistCacheSuite) TestRead_L2Hit() {
	fileName := "bc_read_l2hit.bin"
	chunkSize := 1 * 1024 * 1024
	testData := make([]byte, chunkSize) // exactly 1 block
	rand.Read(testData)

	err := os.WriteFile(filepath.Join(suite.storagePath, fileName), testData, 0644)
	suite.assert.NoError(err)

	// First read: populates L2
	h1, err := suite.blockCache.OpenFile(internal.OpenFileOptions{Name: fileName})
	require.NoError(suite.T(), err)

	buf := make([]byte, chunkSize)
	n, err := suite.blockCache.ReadInBuffer(&internal.ReadInBufferOptions{
		Handle: h1, Offset: 0, Data: buf,
	})
	suite.assert.True(err == nil || err == io.EOF, "expected nil or EOF, got: %v", err)
	suite.assert.Equal(chunkSize, n)
	suite.assert.Equal(testData, buf[:n])

	err = suite.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: h1})
	suite.assert.NoError(err)

	// Wait for L2 population
	time.Sleep(300 * time.Millisecond)
	suite.assert.Greater(suite.srv.chunkCount(), 0)

	// Delete the source to prove second read comes from L2
	os.Remove(filepath.Join(suite.storagePath, fileName))
	suite.distCache.clearDirty(fileName)

	// Second read: L2 hit — but block_cache needs GetAttr, which requires loopback.
	// Recreate a dummy file with the same size so GetAttr succeeds.
	dummy := make([]byte, chunkSize)
	err = os.WriteFile(filepath.Join(suite.storagePath, fileName), dummy, 0644)
	suite.assert.NoError(err)

	h2, err := suite.blockCache.OpenFile(internal.OpenFileOptions{Name: fileName})
	require.NoError(suite.T(), err)

	buf2 := make([]byte, chunkSize)
	n2, err := suite.blockCache.ReadInBuffer(&internal.ReadInBufferOptions{
		Handle: h2, Offset: 0, Data: buf2,
	})
	suite.assert.True(err == nil || err == io.EOF, "expected nil or EOF, got: %v", err)
	suite.assert.Equal(chunkSize, n2)
	suite.assert.Equal(testData, buf2[:n2], "second read should serve original data from L2 cache")

	err = suite.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: h2})
	suite.assert.NoError(err)
}

// --- Test: Small file read (< 1 block) ---

func (suite *blockCacheDistCacheSuite) TestRead_SmallFile() {
	fileName := "bc_small.txt"
	testData := []byte("small file through block_cache + dist_cache pipeline")

	err := os.WriteFile(filepath.Join(suite.storagePath, fileName), testData, 0644)
	suite.assert.NoError(err)

	h, err := suite.blockCache.OpenFile(internal.OpenFileOptions{Name: fileName})
	require.NoError(suite.T(), err)
	suite.assert.Equal(int64(len(testData)), h.Size)

	buf := make([]byte, len(testData))
	n, err := suite.blockCache.ReadInBuffer(&internal.ReadInBufferOptions{
		Handle: h, Offset: 0, Data: buf,
	})
	suite.assert.True(err == nil || err == io.EOF, "expected nil or EOF, got: %v", err)
	suite.assert.Equal(len(testData), n)
	suite.assert.Equal(testData, buf[:n])

	err = suite.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: h})
	suite.assert.NoError(err)
}

// --- Test: Multi-block sequential read ---

func (suite *blockCacheDistCacheSuite) TestRead_MultiBlock_Sequential() {
	fileName := "bc_multi_seq.bin"
	// 3.5 blocks (block-size-mb: 1)
	chunkSize := 1 * 1024 * 1024
	fileSize := 3*chunkSize + chunkSize/2
	testData := make([]byte, fileSize)
	rand.Read(testData)

	err := os.WriteFile(filepath.Join(suite.storagePath, fileName), testData, 0644)
	suite.assert.NoError(err)

	h, err := suite.blockCache.OpenFile(internal.OpenFileOptions{Name: fileName})
	require.NoError(suite.T(), err)

	// Read in small chunks to test sequential prefetch
	readBuf := make([]byte, 4096)
	var result []byte
	offset := 0
	for {
		n, err := suite.blockCache.ReadInBuffer(&internal.ReadInBufferOptions{
			Handle: h, Offset: int64(offset), Data: readBuf,
		})
		if n > 0 {
			result = append(result, readBuf[:n]...)
			offset += n
		}
		if err != nil {
			break
		}
	}
	suite.assert.Equal(fileSize, len(result))
	suite.assert.Equal(testData, result, "sequential multi-block read should produce correct data")

	err = suite.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: h})
	suite.assert.NoError(err)

	// Wait for L2 population
	time.Sleep(300 * time.Millisecond)
	suite.assert.GreaterOrEqual(suite.srv.chunkCount(), 4, "should have at least 4 chunks for 3.5 MB file")
}

// --- Test: Delete through pipeline ---

func (suite *blockCacheDistCacheSuite) TestDeleteFile_ThroughPipeline() {
	fileName := "bc_delete.bin"
	testData := make([]byte, 1*1024*1024)
	rand.Read(testData)

	err := os.WriteFile(filepath.Join(suite.storagePath, fileName), testData, 0644)
	suite.assert.NoError(err)

	// Read to populate L2
	h, err := suite.blockCache.OpenFile(internal.OpenFileOptions{Name: fileName})
	require.NoError(suite.T(), err)
	buf := make([]byte, len(testData))
	_, err = suite.blockCache.ReadInBuffer(&internal.ReadInBufferOptions{Handle: h, Offset: 0, Data: buf})
	suite.assert.True(err == nil || err == io.EOF, "expected nil or EOF, got: %v", err)
	err = suite.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: h})
	suite.assert.NoError(err)
	time.Sleep(200 * time.Millisecond)

	suite.assert.Greater(suite.srv.chunkCount(), 0)

	// Delete through pipeline
	err = suite.blockCache.DeleteFile(internal.DeleteFileOptions{Name: fileName})
	suite.assert.NoError(err)

	// Verify file is deleted from loopback
	_, err = os.Stat(filepath.Join(suite.storagePath, fileName))
	suite.assert.True(os.IsNotExist(err), "file should be deleted from storage")
}

// --- Test: Rename through pipeline ---

func (suite *blockCacheDistCacheSuite) TestRenameFile_ThroughPipeline() {
	oldName := "bc_rename_old.bin"
	newName := "bc_rename_new.bin"
	testData := make([]byte, 1*1024*1024)
	rand.Read(testData)

	err := os.WriteFile(filepath.Join(suite.storagePath, oldName), testData, 0644)
	suite.assert.NoError(err)

	// Rename through block_cache → dist_cache → loopback
	err = suite.blockCache.RenameFile(internal.RenameFileOptions{Src: oldName, Dst: newName})
	suite.assert.NoError(err)

	// Verify old file is gone, new file exists with same data
	_, err = os.Stat(filepath.Join(suite.storagePath, oldName))
	suite.assert.True(os.IsNotExist(err))

	stored, err := os.ReadFile(filepath.Join(suite.storagePath, newName))
	suite.assert.NoError(err)
	suite.assert.Equal(testData, stored, "renamed file should have same data")
}

// --- Test: Graceful degradation (cache server down, reads still work) ---

func (suite *blockCacheDistCacheSuite) TestGracefulDegradation_ServerDown() {
	fileName := "bc_degrade.bin"
	testData := make([]byte, 1*1024*1024)
	rand.Read(testData)

	err := os.WriteFile(filepath.Join(suite.storagePath, fileName), testData, 0644)
	suite.assert.NoError(err)

	// Kill the cache server
	suite.srv.close()

	// Read should still work: block_cache → dist_cache (bypass) → loopback
	h, err := suite.blockCache.OpenFile(internal.OpenFileOptions{Name: fileName})
	require.NoError(suite.T(), err)

	buf := make([]byte, len(testData))
	n, err := suite.blockCache.ReadInBuffer(&internal.ReadInBufferOptions{
		Handle: h, Offset: 0, Data: buf,
	})
	suite.assert.True(err == nil || err == io.EOF, "expected nil or EOF, got: %v", err)
	suite.assert.Equal(len(testData), n)
	suite.assert.Equal(testData, buf[:n], "read should succeed from loopback when L2 is down")

	err = suite.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: h})
	suite.assert.NoError(err)
}

// --- Test: Write (CreateFile + WriteFile + FlushFile) stages and commits through dist_cache ---

func (suite *blockCacheDistCacheSuite) TestWrite_CreateFlush_PopulatesL2() {
	fileName := "bc_write_flush.bin"
	chunkSize := 1 * 1024 * 1024
	testData := make([]byte, chunkSize)
	rand.Read(testData)

	// Create file through block_cache → dist_cache → loopback
	handle, err := suite.blockCache.CreateFile(internal.CreateFileOptions{Name: fileName, Mode: 0644})
	require.NoError(suite.T(), err)

	// Write data through block_cache
	n, err := suite.blockCache.WriteFile(&internal.WriteFileOptions{
		Handle: handle, Offset: 0, Data: testData,
	})
	suite.assert.NoError(err)
	suite.assert.Equal(chunkSize, n)

	// Flush → block_cache calls StageData + CommitData on dist_cache → loopback
	err = suite.blockCache.FlushFile(internal.FlushFileOptions{Handle: handle})
	suite.assert.NoError(err)

	err = suite.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: handle})
	suite.assert.NoError(err)

	// Wait for async L2 population
	time.Sleep(300 * time.Millisecond)

	// Verify data was committed to loopback storage
	stored, err := os.ReadFile(filepath.Join(suite.storagePath, fileName))
	suite.assert.NoError(err)
	suite.assert.Equal(testData, stored, "write should propagate through block_cache → dist_cache → loopback")

	// Verify L2 was populated (StageData/CommitData triggers L2 flush in dist_cache)
	suite.assert.Greater(suite.srv.chunkCount(), 0, "L2 should have chunks after block_cache write+flush")
}

// --- Test: Multi-block write through block_cache pipeline populates multiple L2 chunks ---

func (suite *blockCacheDistCacheSuite) TestWrite_MultiBlock_PopulatesL2() {
	fileName := "bc_write_multi.bin"
	chunkSize := 1 * 1024 * 1024
	// 3 full blocks
	fileSize := 3 * chunkSize
	testData := make([]byte, fileSize)
	rand.Read(testData)

	// Create and write through block_cache
	handle, err := suite.blockCache.CreateFile(internal.CreateFileOptions{Name: fileName, Mode: 0644})
	require.NoError(suite.T(), err)

	n, err := suite.blockCache.WriteFile(&internal.WriteFileOptions{
		Handle: handle, Offset: 0, Data: testData,
	})
	suite.assert.NoError(err)
	suite.assert.Equal(fileSize, n)

	// Flush stages all blocks then commits
	err = suite.blockCache.FlushFile(internal.FlushFileOptions{Handle: handle})
	suite.assert.NoError(err)

	err = suite.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: handle})
	suite.assert.NoError(err)

	// Wait for async L2 population
	time.Sleep(300 * time.Millisecond)

	// Verify full data reached loopback
	stored, err := os.ReadFile(filepath.Join(suite.storagePath, fileName))
	suite.assert.NoError(err)
	suite.assert.Equal(testData, stored, "multi-block write should propagate to loopback")

	// Verify multiple chunks in L2 (one per block)
	suite.assert.GreaterOrEqual(suite.srv.chunkCount(), 3, "should have at least 3 chunks for 3 MB file with 1 MB blocks")
}

// --- Test: Write then read-back through block_cache pipeline ---

func (suite *blockCacheDistCacheSuite) TestWrite_ThenRead_RoundTrip() {
	fileName := "bc_write_read.bin"
	chunkSize := 1 * 1024 * 1024
	testData := make([]byte, chunkSize)
	rand.Read(testData)

	// Write through block_cache
	handle, err := suite.blockCache.CreateFile(internal.CreateFileOptions{Name: fileName, Mode: 0644})
	require.NoError(suite.T(), err)

	_, err = suite.blockCache.WriteFile(&internal.WriteFileOptions{
		Handle: handle, Offset: 0, Data: testData,
	})
	suite.assert.NoError(err)

	err = suite.blockCache.FlushFile(internal.FlushFileOptions{Handle: handle})
	suite.assert.NoError(err)

	err = suite.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: handle})
	suite.assert.NoError(err)

	// Wait for L2 population
	time.Sleep(300 * time.Millisecond)
	suite.assert.Greater(suite.srv.chunkCount(), 0)

	// Clear dirty so L2 is used on read
	suite.distCache.clearDirty(fileName)

	// Read back through block_cache → dist_cache (L2 hit) → loopback
	h, err := suite.blockCache.OpenFile(internal.OpenFileOptions{Name: fileName})
	require.NoError(suite.T(), err)
	suite.assert.Equal(int64(chunkSize), h.Size)

	buf := make([]byte, chunkSize)
	nRead, err := suite.blockCache.ReadInBuffer(&internal.ReadInBufferOptions{
		Handle: h, Offset: 0, Data: buf,
	})
	suite.assert.True(err == nil || err == io.EOF, "expected nil or EOF, got: %v", err)
	suite.assert.Equal(chunkSize, nRead)
	suite.assert.Equal(testData, buf[:nRead], "data should survive write → flush → read round-trip through block_cache pipeline")

	err = suite.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: h})
	suite.assert.NoError(err)
}
