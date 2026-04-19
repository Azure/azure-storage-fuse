package block_cache

import (
	"testing"

	"github.com/Azure/azure-storage-fuse/v2/internal"
	"github.com/Azure/azure-storage-fuse/v2/internal/handlemap"
	"github.com/stretchr/testify/assert"
)

// Test that ReleaseFile with an invalid handle type returns an error.
func TestReleaseFile_InvalidHandleType(t *testing.T) {
	bc := NewBlockCacheComponent().(*BlockCache)
	handle := handlemap.NewHandle("/tmp/file")
	handle.IFObj = "not a blockCacheHandle"

	err := bc.ReleaseFile(internal.ReleaseFileOptions{Handle: handle})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid handle type")
}

// Test that FlushFile with an invalid handle type returns an error.
func TestFlushFile_InvalidHandleType(t *testing.T) {
	bc := NewBlockCacheComponent().(*BlockCache)
	handle := handlemap.NewHandle("/tmp/file")
	handle.IFObj = "not a blockCacheHandle"

	err := bc.FlushFile(internal.FlushFileOptions{Handle: handle})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid handle type")
}

// Test that SyncFile with an invalid handle type returns an error.
func TestSyncFile_InvalidHandleType(t *testing.T) {
	bc := NewBlockCacheComponent().(*BlockCache)
	handle := handlemap.NewHandle("/tmp/file")
	handle.IFObj = "not a blockCacheHandle"

	err := bc.SyncFile(internal.SyncFileOptions{Handle: handle})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid handle type")
}

// Test that WriteFile with an invalid handle type returns an error.
func TestWriteFile_InvalidHandleType(t *testing.T) {
	bc := NewBlockCacheComponent().(*BlockCache)
	handle := handlemap.NewHandle("/tmp/file")
	handle.IFObj = "not a blockCacheHandle"

	n, err := bc.WriteFile(&internal.WriteFileOptions{Handle: handle, Offset: 0, Data: []byte("x")})
	assert.Error(t, err)
	assert.Equal(t, 0, n)
	assert.Contains(t, err.Error(), "invalid handle type")
}

// Test that ReadInBuffer with an invalid handle type returns an error.
func TestReadInBuffer_InvalidHandleType(t *testing.T) {
	bc := NewBlockCacheComponent().(*BlockCache)
	handle := handlemap.NewHandle("/tmp/file")
	handle.IFObj = "not a blockCacheHandle"

	n, err := bc.ReadInBuffer(&internal.ReadInBufferOptions{Handle: handle, Offset: 0, Data: make([]byte, 10)})
	assert.Error(t, err)
	assert.Equal(t, 0, n)
	assert.Contains(t, err.Error(), "invalid handle type")
}

// Test that TruncateFile with an invalid handle type returns an error.
func TestTruncateFile_InvalidHandleType(t *testing.T) {
	bc := NewBlockCacheComponent().(*BlockCache)
	handle := handlemap.NewHandle("/tmp/file")
	handle.IFObj = "not a blockCacheHandle"

	err := bc.TruncateFile(internal.TruncateFileOptions{Name: "/tmp/file", NewSize: 0, Handle: handle})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid handle type")
}
