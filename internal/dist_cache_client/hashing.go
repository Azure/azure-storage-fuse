// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package dcache

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"sync"
)

// ConsistentHashRing implements SHA256-based consistent hashing with virtual nodes,
// matching the C++ ConsistentHasher used by the distributed cache server.
type ConsistentHashRing struct {
	mu         sync.RWMutex
	ring       []ringEntry // sorted by hash
	vnodeCount int
	serverSet  map[string]struct{}
}

type ringEntry struct {
	hash   uint64
	server string
}

// NewConsistentHashRing creates a new consistent hash ring.
// vnodeCount should be 750 to match the C++ server default.
func NewConsistentHashRing(servers []string, vnodeCount int) *ConsistentHashRing {
	r := &ConsistentHashRing{
		vnodeCount: vnodeCount,
		serverSet:  make(map[string]struct{}),
	}
	r.addServersLocked(servers)
	return r
}

// GetServer returns the server responsible for the given key.
// The key is hashed with SHA256, folded to 64 bits, and the nearest
// clockwise ring entry is returned (matching C++ ConsistentHasher::GetServer).
func (r *ConsistentHashRing) GetServer(key string) (string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(r.ring) == 0 {
		return "", ErrNoServers
	}

	h := foldSHA256(key)

	// Binary search for the first entry with hash >= h (clockwise)
	idx := sort.Search(len(r.ring), func(i int) bool {
		return r.ring[i].hash >= h
	})

	// Wrap around to the beginning of the ring
	if idx == len(r.ring) {
		idx = 0
	}

	return r.ring[idx].server, nil
}

// UpdateServers replaces the server list, only modifying entries for
// servers that were added or removed.
func (r *ConsistentHashRing) UpdateServers(servers []string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	newSet := make(map[string]struct{}, len(servers))
	for _, s := range servers {
		newSet[s] = struct{}{}
	}

	// Find servers to remove
	var toRemove []string
	for s := range r.serverSet {
		if _, ok := newSet[s]; !ok {
			toRemove = append(toRemove, s)
		}
	}

	// Find servers to add
	var toAdd []string
	for s := range newSet {
		if _, ok := r.serverSet[s]; !ok {
			toAdd = append(toAdd, s)
		}
	}

	if len(toRemove) == 0 && len(toAdd) == 0 {
		return
	}

	r.removeServersLocked(toRemove)
	r.addServersLocked(toAdd)
}

// Servers returns the current list of servers.
func (r *ConsistentHashRing) Servers() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	servers := make([]string, 0, len(r.serverSet))
	for s := range r.serverSet {
		servers = append(servers, s)
	}
	sort.Strings(servers)
	return servers
}

func (r *ConsistentHashRing) addServersLocked(servers []string) {
	for _, server := range servers {
		if _, exists := r.serverSet[server]; exists {
			continue
		}
		r.serverSet[server] = struct{}{}
		for v := 0; v < r.vnodeCount; v++ {
			vnodeKey := fmt.Sprintf("%s_%d", server, v)
			h := foldSHA256(vnodeKey)
			r.ring = append(r.ring, ringEntry{hash: h, server: server})
		}
	}
	sort.Slice(r.ring, func(i, j int) bool {
		return r.ring[i].hash < r.ring[j].hash
	})
}

func (r *ConsistentHashRing) removeServersLocked(servers []string) {
	removeSet := make(map[string]struct{}, len(servers))
	for _, s := range servers {
		removeSet[s] = struct{}{}
		delete(r.serverSet, s)
	}

	filtered := r.ring[:0]
	for _, e := range r.ring {
		if _, remove := removeSet[e.server]; !remove {
			filtered = append(filtered, e)
		}
	}
	r.ring = filtered
}

// foldSHA256 computes SHA256 of the input and folds the 256-bit hash
// into a 64-bit value by XORing four 64-bit chunks. This matches the
// C++ ConsistentHasher which XORs four 16-hex-digit substrings.
func foldSHA256(input string) uint64 {
	h := sha256.Sum256([]byte(input))
	hexStr := hex.EncodeToString(h[:])

	// C++ implementation: XOR four 16-hex-digit chunks of the hex string
	var result uint64
	for i := 0; i < 4; i++ {
		chunk := hexStr[i*16 : (i+1)*16]
		var val uint64
		for _, c := range chunk {
			val <<= 4
			switch {
			case c >= '0' && c <= '9':
				val |= uint64(c - '0')
			case c >= 'a' && c <= 'f':
				val |= uint64(c - 'a' + 10)
			}
		}
		result ^= val
	}
	return result
}

// GenerateCacheKey creates a distributed cache chunk key.
// The etag is included in the key so each blob revision occupies a distinct cache slot.
//
//	SHA256(cachePrefix/filePath:etag:offset[:chunkSize])
//
// The chunkSize suffix is included only when it differs from the default (4 MiB).
// Since blobfuse uses 16 MiB (non-default), the suffix is always included.
func GenerateCacheKey(cachePrefix, filePath, etag string, offset, chunkSize int64) string {
	const defaultServerChunkSize = 4 * 1024 * 1024 // 4 MiB server default

	var keyInput string
	if chunkSize == defaultServerChunkSize {
		keyInput = fmt.Sprintf("%s/%s:%s:%d", cachePrefix, filePath, etag, offset)
	} else {
		keyInput = fmt.Sprintf("%s/%s:%s:%d:%d", cachePrefix, filePath, etag, offset, chunkSize)
	}

	h := sha256.Sum256([]byte(keyInput))
	return hex.EncodeToString(h[:])
}

// GenerateAttrCacheKey creates a cache key for file attributes.
// This is simply SHA256(filePath), matching the C++ attribute cache key format.
func GenerateAttrCacheKey(filePath string) string {
	h := sha256.Sum256([]byte(filePath))
	return hex.EncodeToString(h[:])
}
