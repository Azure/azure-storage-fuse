// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

//go:build unittest

package dcache

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFoldSHA256Deterministic verifies that the same input always produces the same hash.
func TestFoldSHA256Deterministic(t *testing.T) {
	h1 := foldSHA256("test-key-123")
	h2 := foldSHA256("test-key-123")
	assert.Equal(t, h1, h2)
}

// TestFoldSHA256DifferentInputs verifies different inputs produce different hashes.
func TestFoldSHA256DifferentInputs(t *testing.T) {
	h1 := foldSHA256("key-a")
	h2 := foldSHA256("key-b")
	assert.NotEqual(t, h1, h2)
}

// TestConsistentHashRingBasic verifies basic ring operations.
func TestConsistentHashRingBasic(t *testing.T) {
	servers := []string{"server-0:9000", "server-1:9000", "server-2:9000"}
	ring := NewConsistentHashRing(servers, 750)

	// Should return a valid server for any key
	for _, key := range []string{"test/file.bin:0:16777216", "another/key", "third-key"} {
		server, err := ring.GetServer(key)
		require.NoError(t, err)
		assert.Contains(t, servers, server)
	}
}

// TestConsistentHashRingEmpty verifies error on empty ring.
func TestConsistentHashRingEmpty(t *testing.T) {
	ring := NewConsistentHashRing(nil, 750)
	_, err := ring.GetServer("any-key")
	assert.ErrorIs(t, err, ErrNoServers)
}

// TestConsistentHashRingDeterministic verifies the same key always maps to the same server.
func TestConsistentHashRingDeterministic(t *testing.T) {
	servers := []string{"server-0:9000", "server-1:9000", "server-2:9000"}
	ring := NewConsistentHashRing(servers, 750)

	server1, _ := ring.GetServer("test-key")
	server2, _ := ring.GetServer("test-key")
	assert.Equal(t, server1, server2)
}

// TestConsistentHashRingDistribution verifies keys are distributed across servers.
func TestConsistentHashRingDistribution(t *testing.T) {
	servers := []string{"server-0:9000", "server-1:9000", "server-2:9000"}
	ring := NewConsistentHashRing(servers, 750)

	counts := make(map[string]int)
	for i := 0; i < 10000; i++ {
		key := GenerateCacheKey("prefix", "file.bin", "", int64(i)*16*1024*1024, 16*1024*1024)
		server, err := ring.GetServer(key)
		require.NoError(t, err)
		counts[server]++
	}

	// Each server should get at least 20% of keys (expect ~33%)
	for _, count := range counts {
		assert.Greater(t, count, 2000, "server should get >20%% of keys")
	}
}

// TestConsistentHashRingUpdateServers verifies adding/removing servers.
func TestConsistentHashRingUpdateServers(t *testing.T) {
	ring := NewConsistentHashRing([]string{"server-0:9000", "server-1:9000"}, 750)

	// Record server for a key
	serverBefore, _ := ring.GetServer("stable-key")
	assert.NotEmpty(t, serverBefore)

	// Add a third server
	ring.UpdateServers([]string{"server-0:9000", "server-1:9000", "server-2:9000"})

	// The key should still resolve (may or may not move)
	serverAfter, err := ring.GetServer("stable-key")
	require.NoError(t, err)
	assert.NotEmpty(t, serverAfter)

	// Verify new server is in the ring
	srvs := ring.Servers()
	assert.Len(t, srvs, 3)
}

// TestConsistentHashRingMinimalRemapping verifies that adding a server
// only remaps a fraction of keys.
func TestConsistentHashRingMinimalRemapping(t *testing.T) {
	servers := []string{"s0", "s1", "s2"}
	ring := NewConsistentHashRing(servers, 750)

	// Record initial mapping
	const numKeys = 10000
	initial := make(map[string]string, numKeys)
	for i := 0; i < numKeys; i++ {
		key := GenerateCacheKey("p", "f", "", int64(i)*16*1024*1024, 16*1024*1024)
		s, _ := ring.GetServer(key)
		initial[key] = s
	}

	// Add a server
	ring.UpdateServers([]string{"s0", "s1", "s2", "s3"})

	// Count remapped keys
	remapped := 0
	for key, oldServer := range initial {
		newServer, _ := ring.GetServer(key)
		if newServer != oldServer {
			remapped++
		}
	}

	// With 4 servers, ~25% of keys should remap (expect <35%)
	remapPct := float64(remapped) / float64(numKeys) * 100
	assert.Less(t, remapPct, 35.0, "too many keys remapped: %.1f%%", remapPct)
}

// TestGenerateCacheKeyDeterministic verifies cache keys are deterministic.
func TestGenerateCacheKeyDeterministic(t *testing.T) {
	k1 := GenerateCacheKey("acct/container", "path/to/file.bin", "", 0, 16*1024*1024)
	k2 := GenerateCacheKey("acct/container", "path/to/file.bin", "", 0, 16*1024*1024)
	assert.Equal(t, k1, k2)
	assert.Len(t, k1, 64) // SHA256 hex = 64 chars
}

// TestGenerateCacheKeyDifferentOffsets verifies different offsets produce different keys.
func TestGenerateCacheKeyDifferentOffsets(t *testing.T) {
	k0 := GenerateCacheKey("acct/ctr", "file.bin", "", 0, 16*1024*1024)
	k1 := GenerateCacheKey("acct/ctr", "file.bin", "", 16*1024*1024, 16*1024*1024)
	assert.NotEqual(t, k0, k1)
}

// TestGenerateCacheKeyDefaultChunkNoSuffix verifies the 4 MiB default
// does not include chunk size in the key (matching C++ behavior).
func TestGenerateCacheKeyDefaultChunkNoSuffix(t *testing.T) {
	const defaultChunk = 4 * 1024 * 1024
	k := GenerateCacheKey("acct/ctr", "file.bin", "", 0, defaultChunk)
	assert.Len(t, k, 64)

	// With non-default chunk size, the key should be different
	kNonDefault := GenerateCacheKey("acct/ctr", "file.bin", "", 0, 16*1024*1024)
	assert.NotEqual(t, k, kNonDefault)
}

// TestGenerateAttrCacheKey verifies attribute cache keys are SHA256 of the path.
func TestGenerateAttrCacheKey(t *testing.T) {
	k1 := GenerateAttrCacheKey("container/path/file.bin")
	k2 := GenerateAttrCacheKey("container/path/file.bin")
	assert.Equal(t, k1, k2)
	assert.Len(t, k1, 64)

	k3 := GenerateAttrCacheKey("different/path")
	assert.NotEqual(t, k1, k3)
}

// TestParseServerList verifies server list string parsing.
func TestParseServerList(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"host1:9000,host2:9000", []string{"host1:9000", "host2:9000"}},
		{"host1:9000, host2:9000 , host3:9000", []string{"host1:9000", "host2:9000", "host3:9000"}},
		{"single:9000", []string{"single:9000"}},
		{"", nil},
		{" , , ", nil},
	}

	for _, tt := range tests {
		result := parseServerList(tt.input)
		if tt.expected == nil {
			assert.Empty(t, result)
		} else {
			assert.Equal(t, tt.expected, result)
		}
	}
}
