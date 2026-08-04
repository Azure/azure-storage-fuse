// Copyright (c) 2026 Microsoft Corporation.
// Licensed under the MIT License.

//go:build unittest

package dist_cache

import (
	"context"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"io"
	"math/rand"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/config"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/component/block_cache"
	"github.com/Azure/azure-storage-fuse/v2/component/loopback"
	"github.com/Azure/azure-storage-fuse/v2/internal"

	dcache "github.com/nearora-msft/dist-cache-client-go"
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
	store    map[string][]byte            // cacheKey -> data
	meta     map[string]map[string][]byte // cacheKey -> metadata (persisted on upload)
	uploads  int                          // count of Uploadrequest messages received
	attrs    map[string]*pb.FileAttribute
	groups   map[string]map[string]bool // groupID -> set of cacheKeys
	locks    map[string]bool            // cacheKey -> locked
	// zeroByteHits[key]=true → Download returns SUCCESS Filesize=0 (simulates
	// a corrupt/empty entry). Checked before store/lock.
	zeroByteHits map[string]bool
	closed       bool
}

func newIntegMockServer(t *testing.T) *integMockServer {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	s := &integMockServer{
		listener:     l,
		addr:         l.Addr().String(),
		store:        make(map[string][]byte),
		meta:         make(map[string]map[string][]byte),
		attrs:        make(map[string]*pb.FileAttribute),
		groups:       make(map[string]map[string]bool),
		locks:        make(map[string]bool),
		zeroByteHits: make(map[string]bool),
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
		s.uploads++
		key := p.Uploadrequest.Filename
		s.store[key] = append([]byte(nil), uploadData...)
		if md := p.Uploadrequest.GetMetadata(); len(md) > 0 {
			copyMd := make(map[string][]byte, len(md))
			for k, v := range md {
				copyMd[k] = append([]byte(nil), v...)
			}
			s.meta[key] = copyMd
		} else {
			delete(s.meta, key)
		}
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
		// zeroByteHits override wins over store/lock.
		if s.zeroByteHits[key] {
			return &pb.DownloadResponse{
				Result:   pb.DownloadResponse_SUCCESS,
				Filesize: 0,
			}, nil
		}
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
		// Echo persisted metadata so the client's checksum verification passes.
		if md, ok := s.meta[key]; ok && len(md) > 0 {
			out := make(map[string][]byte, len(md))
			for k, v := range md {
				out[k] = append([]byte(nil), v...)
			}
			resp.Metadata = out
		}
		// Include gid metadata if tracked (adds to whatever the client sent).
		for gid, keys := range s.groups {
			if keys[key] {
				if resp.Metadata == nil {
					resp.Metadata = map[string][]byte{}
				}
				resp.Metadata["gid"] = []byte(gid)
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

// uploadCount returns the number of Upload requests the server has received
// (independent of priming writes that touch s.store directly).
func (s *integMockServer) uploadCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.uploads
}

// primeLock marks a key as locked by a phantom peer.
func (s *integMockServer) primeLock(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.locks[key] = true
}

// primeChunk seeds the store as if a peer finished populating. It also
// records the CRC32 checksum metadata the real client expects so that
// downloads pass checksum verification.
func (s *integMockServer) primeChunk(key string, data []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.store[key] = append([]byte(nil), data...)
	if s.meta[key] == nil {
		s.meta[key] = make(map[string][]byte)
	}
	s.meta[key]["CHUNK_CHECKSUM"] = []byte(strconv.FormatUint(uint64(crc32.ChecksumIEEE(data)), 10))
}

// primeCorruptChunk seeds the store with a checksum that does NOT match the
// data, simulating in-flight corruption of an L2 chunk.
func (s *integMockServer) primeCorruptChunk(key string, data []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.store[key] = append([]byte(nil), data...)
	if s.meta[key] == nil {
		s.meta[key] = make(map[string][]byte)
	}
	s.meta[key]["CHUNK_CHECKSUM"] = []byte(strconv.FormatUint(uint64(crc32.ChecksumIEEE(data))^0xDEADBEEF, 10))
}

// expireLock clears the lock, simulating the peer's server-side TTL firing.
func (s *integMockServer) expireLock(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.locks, key)
}

// primeZeroByteHit makes Downloads for key return SUCCESS Filesize=0.
func (s *integMockServer) primeZeroByteHit(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.zeroByteHits[key] = true
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

	// Build config: dist_cache → loopbackfs (no file_cache, test dist_cache directly).
	// block_cache.block-size-mb is the single source of chunk size, even when
	// block_cache isn't in the pipeline for this suite.
	suite.configString = fmt.Sprintf(
		"loopbackfs:\n  path: %s\n\nazstorage:\n  account-name: test\n  container: container\n\nblock_cache:\n  block-size-mb: 1\n\ndist_cache:\n  server-list: %s\n",
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
	// Create data that's exactly one chunk (1 MB = block_cache.block-size-mb in config)
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

// --- Test: poll observes L2 hit after peer populates ---

// AlreadyLocked → poll → L2 hit: prime lock, start reader, prime chunk
// mid-poll. Reader's next iteration observes SUCCESS.
func (suite *distCacheIntegSuite) TestReadInBuffer_Poll_HitsL2AfterPeerPopulates() {
	fileName := "test_poll_hit.bin"
	chunkSize := 1 * 1024 * 1024 // matches block_cache.block-size-mb=1
	peerData := make([]byte, chunkSize)
	rand.Read(peerData)

	// Different payload on loopback proves the read came from L2.
	loopbackData := make([]byte, chunkSize)
	rand.Read(loopbackData)
	err := os.WriteFile(filepath.Join(suite.storagePath, fileName), loopbackData, 0644)
	suite.assert.NoError(err)

	cacheKey := dcache.GenerateCacheKey("test/container", fileName, "", 0, int64(chunkSize))
	suite.srv.primeLock(cacheKey)

	buf := make([]byte, chunkSize)
	type result struct {
		n   int
		err error
	}
	done := make(chan result, 1)
	go func() {
		n, err := suite.distCache.ReadInBuffer(&internal.ReadInBufferOptions{
			Path:   fileName,
			Offset: 0,
			Data:   buf,
			Size:   int64(chunkSize),
		})
		done <- result{n, err}
	}()

	// Sleep past the first poll iteration (backoff=200ms), then prime chunk.
	time.Sleep(300 * time.Millisecond)
	suite.srv.primeChunk(cacheKey, peerData)

	select {
	case r := <-done:
		suite.assert.NoError(r.err)
		suite.assert.Equal(chunkSize, r.n)
		suite.assert.Equal(peerData, buf[:r.n], "read must be served from L2, not from loopback storage")
	case <-time.After(5 * time.Second):
		suite.T().Fatal("reader did not complete within 5s (poll should have observed the primed chunk)")
	}
}

// --- Test: poll times out and falls through to storage without populating ---

// AlreadyLocked → poll → timeout: permanent lock so poll never sees the
// chunk. Reader falls through to loopback and must NOT populate.
func (suite *distCacheIntegSuite) TestReadInBuffer_Poll_TimeoutFallsThroughNoPopulate() {
	fileName := "test_poll_timeout.bin"
	chunkSize := 1 * 1024 * 1024
	testData := make([]byte, chunkSize)
	rand.Read(testData)
	err := os.WriteFile(filepath.Join(suite.storagePath, fileName), testData, 0644)
	suite.assert.NoError(err)

	cacheKey := dcache.GenerateCacheKey("test/container", fileName, "", 0, int64(chunkSize))
	suite.srv.primeLock(cacheKey)

	chunksBefore := suite.srv.chunkCount()

	buf := make([]byte, chunkSize)
	start := time.Now()
	n, err := suite.distCache.ReadInBuffer(&internal.ReadInBufferOptions{
		Path:   fileName,
		Offset: 0,
		Data:   buf,
		Size:   int64(chunkSize),
	})
	elapsed := time.Since(start)

	suite.assert.NoError(err)
	suite.assert.Equal(chunkSize, n)
	suite.assert.Equal(testData, buf[:n], "read must fall through to loopback on poll timeout")

	// maxPollDuration is 3s; the call cannot return earlier.
	suite.assert.GreaterOrEqual(elapsed, 3*time.Second, "poll should have run to its full deadline")

	// Drain any in-flight populate before checking no upload happened.
	suite.distCache.inflight.Wait()
	suite.assert.Equal(chunksBefore, suite.srv.chunkCount(),
		"must NOT populate on poll timeout: peer still owns the lock")
}

// --- Test: poll inherits the miss-lock and populates L2 ---

// AlreadyLocked → poll → GotLock: prime lock, start reader, expire lock
// mid-poll. Next iteration re-acquires the lock; caller fetches loopback
// and populates.
func (suite *distCacheIntegSuite) TestReadInBuffer_Poll_InheritsLockPopulatesL2() {
	fileName := "test_poll_inherit.bin"
	chunkSize := 1 * 1024 * 1024
	testData := make([]byte, chunkSize)
	rand.Read(testData)
	err := os.WriteFile(filepath.Join(suite.storagePath, fileName), testData, 0644)
	suite.assert.NoError(err)

	cacheKey := dcache.GenerateCacheKey("test/container", fileName, "", 0, int64(chunkSize))
	suite.srv.primeLock(cacheKey)
	chunksBefore := suite.srv.chunkCount()

	buf := make([]byte, chunkSize)
	type result struct {
		n   int
		err error
	}
	done := make(chan result, 1)
	go func() {
		n, err := suite.distCache.ReadInBuffer(&internal.ReadInBufferOptions{
			Path:   fileName,
			Offset: 0,
			Data:   buf,
			Size:   int64(chunkSize),
		})
		done <- result{n, err}
	}()

	// Sleep past the first poll iteration, then expire the peer's lock.
	time.Sleep(300 * time.Millisecond)
	suite.srv.expireLock(cacheKey)

	select {
	case r := <-done:
		suite.assert.NoError(r.err)
		suite.assert.Equal(chunkSize, r.n)
		suite.assert.Equal(testData, buf[:r.n], "read must fetch from loopback after inheriting the miss-lock")
	case <-time.After(5 * time.Second):
		suite.T().Fatal("reader did not complete within 5s (poll should have inherited the lock)")
	}

	// Inheriting the lock means we must populate.
	suite.distCache.inflight.Wait()
	suite.assert.Greater(suite.srv.chunkCount(), chunksBefore,
		"L2 must be populated after inheriting the miss-lock via poll")
}

// --- Test: poll observes a zero-byte cache entry, falls through, no populate ---

// AlreadyLocked → poll → zero-byte hit: prime lock, start reader, prime
// zero-byte SUCCESS mid-poll. Poll returns errPollCorruptHit; caller falls
// through to loopback and must NOT populate (no lock ownership).
func (suite *distCacheIntegSuite) TestReadInBuffer_Poll_ZeroByteHitFallsThroughNoPopulate() {
	fileName := "test_poll_zerobyte.bin"
	chunkSize := 1 * 1024 * 1024
	testData := make([]byte, chunkSize)
	rand.Read(testData)
	err := os.WriteFile(filepath.Join(suite.storagePath, fileName), testData, 0644)
	suite.assert.NoError(err)

	cacheKey := dcache.GenerateCacheKey("test/container", fileName, "", 0, int64(chunkSize))
	suite.srv.primeLock(cacheKey)
	chunksBefore := suite.srv.chunkCount()

	buf := make([]byte, chunkSize)
	type result struct {
		n   int
		err error
	}
	done := make(chan result, 1)
	go func() {
		n, err := suite.distCache.ReadInBuffer(&internal.ReadInBufferOptions{
			Path:   fileName,
			Offset: 0,
			Data:   buf,
			Size:   int64(chunkSize),
		})
		done <- result{n, err}
	}()

	// Sleep past the first poll iteration, then flip to zero-byte SUCCESS.
	time.Sleep(300 * time.Millisecond)
	suite.srv.primeZeroByteHit(cacheKey)

	select {
	case r := <-done:
		suite.assert.NoError(r.err)
		suite.assert.Equal(chunkSize, r.n)
		suite.assert.Equal(testData, buf[:r.n], "read must fall through to loopback on zero-byte poll hit")
	case <-time.After(5 * time.Second):
		suite.T().Fatal("reader did not complete within 5s")
	}

	suite.distCache.inflight.Wait()
	suite.assert.Equal(chunksBefore, suite.srv.chunkCount(),
		"must NOT populate on zero-byte poll hit: no lock ownership")
}

// --- Tests: checksum-mismatch handling (initial download + poll) ---

// Corrupt L2 chunk + bypass-on-error=true (suite default): must fall through
// to loopback and NOT populate (peer still owns the entry).
func (suite *distCacheIntegSuite) TestReadInBuffer_ChecksumMismatch_BypassFallsThroughToStorage() {
	fileName := "test_checksum_bypass.bin"
	chunkSize := 1 * 1024 * 1024
	loopbackData := make([]byte, chunkSize)
	rand.Read(loopbackData)
	err := os.WriteFile(filepath.Join(suite.storagePath, fileName), loopbackData, 0644)
	suite.assert.NoError(err)

	corruptData := make([]byte, chunkSize)
	rand.Read(corruptData)
	cacheKey := dcache.GenerateCacheKey("test/container", fileName, "", 0, int64(chunkSize))
	suite.srv.primeCorruptChunk(cacheKey, corruptData)
	uploadsBefore := suite.srv.uploadCount()

	buf := make([]byte, chunkSize)
	n, err := suite.distCache.ReadInBuffer(&internal.ReadInBufferOptions{
		Path:   fileName,
		Offset: 0,
		Data:   buf,
		Size:   int64(chunkSize),
	})

	suite.assert.NoError(err)
	suite.assert.Equal(chunkSize, n)
	suite.assert.Equal(loopbackData, buf[:n], "read must fall through to loopback on L2 checksum mismatch")

	suite.distCache.inflight.Wait()
	suite.assert.Equal(uploadsBefore, suite.srv.uploadCount(),
		"must NOT populate on L2 checksum mismatch: peer still owns the entry")
}

// Corrupt L2 chunk + bypass-on-error=false: must return EIO without falling
// through to loopback.
func (suite *distCacheIntegSuite) TestReadInBuffer_ChecksumMismatch_StrictReturnsEIO() {
	fileName := "test_checksum_strict.bin"
	chunkSize := 1 * 1024 * 1024
	loopbackData := make([]byte, chunkSize)
	rand.Read(loopbackData)
	err := os.WriteFile(filepath.Join(suite.storagePath, fileName), loopbackData, 0644)
	suite.assert.NoError(err)

	corruptData := make([]byte, chunkSize)
	rand.Read(corruptData)
	cacheKey := dcache.GenerateCacheKey("test/container", fileName, "", 0, int64(chunkSize))
	suite.srv.primeCorruptChunk(cacheKey, corruptData)

	suite.distCache.bypassOnError = false
	defer func() { suite.distCache.bypassOnError = true }()

	buf := make([]byte, chunkSize)
	n, err := suite.distCache.ReadInBuffer(&internal.ReadInBufferOptions{
		Path:   fileName,
		Offset: 0,
		Data:   buf,
		Size:   int64(chunkSize),
	})

	suite.assert.ErrorIs(err, syscall.EIO)
	suite.assert.Equal(0, n)
}

// Poll observes a corrupt chunk + bypass-on-error=true: fall through to
// loopback, no populate.
func (suite *distCacheIntegSuite) TestReadInBuffer_Poll_ChecksumMismatch_BypassFallsThrough() {
	fileName := "test_poll_checksum_bypass.bin"
	chunkSize := 1 * 1024 * 1024
	loopbackData := make([]byte, chunkSize)
	rand.Read(loopbackData)
	err := os.WriteFile(filepath.Join(suite.storagePath, fileName), loopbackData, 0644)
	suite.assert.NoError(err)

	cacheKey := dcache.GenerateCacheKey("test/container", fileName, "", 0, int64(chunkSize))
	suite.srv.primeLock(cacheKey)
	uploadsBefore := suite.srv.uploadCount()

	buf := make([]byte, chunkSize)
	type result struct {
		n   int
		err error
	}
	done := make(chan result, 1)
	go func() {
		n, err := suite.distCache.ReadInBuffer(&internal.ReadInBufferOptions{
			Path:   fileName,
			Offset: 0,
			Data:   buf,
			Size:   int64(chunkSize),
		})
		done <- result{n, err}
	}()

	corruptData := make([]byte, chunkSize)
	rand.Read(corruptData)
	time.Sleep(300 * time.Millisecond)
	suite.srv.primeCorruptChunk(cacheKey, corruptData)

	select {
	case r := <-done:
		suite.assert.NoError(r.err)
		suite.assert.Equal(chunkSize, r.n)
		suite.assert.Equal(loopbackData, buf[:r.n], "poll checksum mismatch must fall through to loopback")
	case <-time.After(5 * time.Second):
		suite.T().Fatal("reader did not complete within 5s")
	}

	suite.distCache.inflight.Wait()
	suite.assert.Equal(uploadsBefore, suite.srv.uploadCount(),
		"must NOT populate on poll checksum mismatch: no lock ownership")
}

// Poll observes a corrupt chunk + bypass-on-error=false: return EIO.
func (suite *distCacheIntegSuite) TestReadInBuffer_Poll_ChecksumMismatch_StrictReturnsEIO() {
	fileName := "test_poll_checksum_strict.bin"
	chunkSize := 1 * 1024 * 1024
	loopbackData := make([]byte, chunkSize)
	rand.Read(loopbackData)
	err := os.WriteFile(filepath.Join(suite.storagePath, fileName), loopbackData, 0644)
	suite.assert.NoError(err)

	cacheKey := dcache.GenerateCacheKey("test/container", fileName, "", 0, int64(chunkSize))
	suite.srv.primeLock(cacheKey)

	suite.distCache.bypassOnError = false
	defer func() { suite.distCache.bypassOnError = true }()

	buf := make([]byte, chunkSize)
	type result struct {
		n   int
		err error
	}
	done := make(chan result, 1)
	go func() {
		n, err := suite.distCache.ReadInBuffer(&internal.ReadInBufferOptions{
			Path:   fileName,
			Offset: 0,
			Data:   buf,
			Size:   int64(chunkSize),
		})
		done <- result{n, err}
	}()

	corruptData := make([]byte, chunkSize)
	rand.Read(corruptData)
	time.Sleep(300 * time.Millisecond)
	suite.srv.primeCorruptChunk(cacheKey, corruptData)

	select {
	case r := <-done:
		suite.assert.ErrorIs(r.err, syscall.EIO)
		suite.assert.Equal(0, r.n)
	case <-time.After(5 * time.Second):
		suite.T().Fatal("reader did not complete within 5s")
	}
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

	badConfig := fmt.Sprintf("loopbackfs:\n  path: %s\n\ndist_cache:\n", storagePath)
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

// --- Test: Start() fails fast on unreachable servers ---
//
// Failure is driven by an unroutable discovery-url (loopback port 1 ->
// ECONNREFUSED); with no fallback, dcache.New() fails.
//
// Standalone (not suite-based) to avoid corrupting shared config state.

func TestStart_UnreachableServers_FailsFast(t *testing.T) {
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
		"loopbackfs:\n  path: %s\n\nazstorage:\n  account-name: test\n  container: container\n\ndist_cache:\n  discovery-url: 127.0.0.1:1\n",
		storagePath)
	require.NoError(t, config.ReadConfigFromReader(strings.NewReader(cfg)))

	lb := loopback.NewLoopbackFSComponent()
	require.NoError(t, lb.Configure(true))
	require.NoError(t, lb.Start(context.Background()))
	defer func() { _ = lb.Stop() }()

	dc := NewDistCacheComponent().(*DistCache)
	dc.SetNextComponent(lb)
	require.NoError(t, dc.Configure(true))

	err = dc.Start(context.Background())
	require.Error(t, err, "Start() must fail fast when servers are unreachable")
	assert.Contains(t, err.Error(), "dist_cache: failed to start")
	assert.Nil(t, dc.client, "client should remain nil after failed Start()")
}

// --- Test: Chunk size resolution from config ---

func (suite *distCacheIntegSuite) TestChunkSize_FromConfig() {
	// The suite uses block_cache.block-size-mb: 1
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
	// block-size-mb: 1 to match loopback's GetCommittedBlockList (hardcoded 1MB).
	// mem-size-mb sized so dist_cache's fair-share (10% of bcRef) covers
	// distCacheMinBuffers × chunkSize and async populate is enabled — this
	// suite exercises the L2 population path, so we must stay above the
	// disable threshold in resolveMemBudget.
	cfg := fmt.Sprintf(
		"read-only: true\n\n"+
			"azstorage:\n  account-name: test\n  container: container\n\n"+
			"block_cache:\n  block-size-mb: 1\n  mem-size-mb: 100\n  prefetch: 12\n  parallelism: 10\n  path: %s\n  disk-size-mb: 50\n  disk-timeout-sec: 20\n\n"+
			"dist_cache:\n  server-list: %s\n\n"+
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
