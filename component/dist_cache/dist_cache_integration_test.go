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

// --- Test: Start() bypass semantics on unreachable servers ---
//
// bypass-on-error is checked in Start() (dcache.New() connect failure) and in
// ReadInBuffer() (nil-client runtime path). These two standalone tests cover
// both bypass=true (Start returns nil, subsequent reads pass through to the
// next component) and bypass=false (Start returns a wrapped error).
//
// The failure is driven by pointing dist_cache at an unroutable discovery-url
// (loopback port 1 -> immediate ECONNREFUSED). With no server-list / env
// fallback, resolveServers() returns ErrNoServers and dcache.New() fails.
//
// They are standalone (not suite-based) to avoid corrupting the shared
// distCacheIntegSuite config state, matching the pattern used by
// TestConfigure_NoServers_ReturnsError above.

func TestStart_BypassOnError_True_UnreachableServers_Succeeds(t *testing.T) {
	err := log.SetDefaultLogger("silent", common.LogConfig{Level: common.ELogLevel.LOG_DEBUG()})
	require.NoError(t, err)

	config.ResetConfig()

	oldEnv := os.Getenv("DIST_CACHE_SERVER_LIST")
	os.Unsetenv("DIST_CACHE_SERVER_LIST")
	defer os.Setenv("DIST_CACHE_SERVER_LIST", oldEnv)

	storagePath := filepath.Join(os.TempDir(), "dcache_integ_bypass_true_"+randomStr(8))
	require.NoError(t, os.MkdirAll(storagePath, 0777))
	defer os.RemoveAll(storagePath)

	cfg := fmt.Sprintf(
		"loopbackfs:\n  path: %s\n\ndist_cache:\n  discovery-url: 127.0.0.1:1\n  cache-prefix: test/container\n  bypass-on-error: true\n",
		storagePath)
	require.NoError(t, config.ReadConfigFromReader(strings.NewReader(cfg)))

	lb := loopback.NewLoopbackFSComponent()
	require.NoError(t, lb.Configure(true))
	require.NoError(t, lb.Start(context.Background()))
	defer func() { _ = lb.Stop() }()

	dc := NewDistCacheComponent().(*DistCache)
	dc.SetNextComponent(lb)
	require.NoError(t, dc.Configure(true))

	// Start must swallow the connect failure and return nil.
	err = dc.Start(context.Background())
	require.NoError(t, err, "Start() must succeed when bypass-on-error=true and servers are unreachable")
	defer func() { _ = dc.Stop() }()

	assert.Nil(t, dc.client, "client should remain nil after failed connect")

	// Subsequent ReadInBuffer must transparently forward to loopbackfs.
	fileName := "bypass_start_read.bin"
	testData := []byte("hello from loopback via bypass path")
	require.NoError(t, os.WriteFile(filepath.Join(storagePath, fileName), testData, 0644))

	buf := make([]byte, len(testData))
	n, err := dc.ReadInBuffer(&internal.ReadInBufferOptions{
		Path:   fileName,
		Offset: 0,
		Data:   buf,
		Size:   int64(len(testData)),
	})
	require.NoError(t, err, "ReadInBuffer should bypass to loopback when client is nil and bypass=true")
	assert.Equal(t, len(testData), n)
	assert.Equal(t, testData, buf[:n])
}

func TestStart_BypassOnError_False_UnreachableServers_ReturnsError(t *testing.T) {
	err := log.SetDefaultLogger("silent", common.LogConfig{Level: common.ELogLevel.LOG_DEBUG()})
	require.NoError(t, err)

	config.ResetConfig()

	oldEnv := os.Getenv("DIST_CACHE_SERVER_LIST")
	os.Unsetenv("DIST_CACHE_SERVER_LIST")
	defer os.Setenv("DIST_CACHE_SERVER_LIST", oldEnv)

	storagePath := filepath.Join(os.TempDir(), "dcache_integ_bypass_false_"+randomStr(8))
	require.NoError(t, os.MkdirAll(storagePath, 0777))
	defer os.RemoveAll(storagePath)

	cfg := fmt.Sprintf(
		"loopbackfs:\n  path: %s\n\ndist_cache:\n  discovery-url: 127.0.0.1:1\n  cache-prefix: test/container\n  bypass-on-error: false\n",
		storagePath)
	require.NoError(t, config.ReadConfigFromReader(strings.NewReader(cfg)))

	lb := loopback.NewLoopbackFSComponent()
	require.NoError(t, lb.Configure(true))
	require.NoError(t, lb.Start(context.Background()))
	defer func() { _ = lb.Stop() }()

	dc := NewDistCacheComponent().(*DistCache)
	dc.SetNextComponent(lb)
	require.NoError(t, dc.Configure(true))

	// Start must surface the connect failure as a wrapped error.
	err = dc.Start(context.Background())
	require.Error(t, err, "Start() must fail when bypass-on-error=false and servers are unreachable")
	assert.Contains(t, err.Error(), "dist_cache: failed to start")
	assert.Nil(t, dc.client, "client should remain nil after failed Start()")
}

// --- Test: Chunk size resolution from config ---

func (suite *distCacheIntegSuite) TestChunkSize_FromConfig() {
	// The suite uses chunk-size-mb: 1
	suite.assert.Equal(int64(1*1024*1024), suite.distCache.chunkSize)
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
