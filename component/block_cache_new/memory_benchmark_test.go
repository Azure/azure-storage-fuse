package block_cache_new

import (
	"fmt"
	"sync"
	"syscall"
	"testing"
)

// Buffer size for testing
const bufferSize = 8 * 1024 * 1024
const entries = 100

var i int = 0

type buf []byte

var mmapbufs []buf = make([]buf, entries)
var zerobuf buf = make(buf, bufferSize)

// mmapAllocation function using syscall to allocate memory

func mmapAllocation() (b []byte, err error) {
	// Use an anonymous memory map (no file backing)
	b = mmapbufs[i%entries]
	if b == nil {
		b, err = syscall.Mmap(-1, 0, bufferSize, syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_ANON|syscall.MAP_PRIVATE)
		if err != nil {
			return nil, err
		}
		mmapbufs[i%entries] = b
	}
	// Copy nil data to the buffer
	copy(b, zerobuf)
	i++
	return b, nil
}

// syncPoolAllocation function using sync.Pool for memory allocation
func syncPoolAllocation(pool *sync.Pool) []byte {
	b := pool.Get().([]byte)
	copy(b, zerobuf)
	return b
}

// Benchmark function for mmap allocation
func BenchmarkMmapAllocation(b *testing.B) {
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Test mmap allocation
		_, err := mmapAllocation()
		if err != nil {
			b.Fatalf("Failed to mmap memory: %v", err)
		}
	}
}

// Benchmark function for sync.Pool allocation
func BenchmarkSyncPoolAllocation(b *testing.B) {
	// Create sync.Pool for fixed size buffers
	pool := &sync.Pool{
		New: func() interface{} {
			return make([]byte, bufferSize)
		},
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Test sync.Pool allocation
		b := syncPoolAllocation(pool)
		pool.Put(b)
	}
}

// // Test function to compare the allocation time using mmap and sync.Pool
// func TestAllocationPerformance(t *testing.T) {
// 	tests := []struct {
// 		name      string
// 		benchmark func(b *testing.B)
// 	}{
// 		{"Mmap Allocation", BenchmarkMmapAllocation},
// 		{"Sync Pool Allocation", BenchmarkSyncPoolAllocation},
// 	}

// 	for _, test := range tests {
// 		t.Run(test.name, func(t *testing.T) {
// 			// Run benchmark
// 			res := testing.Benchmark(test.benchmark)
// 			fmt.Println(test.name, res)
// 		})
// 	}
// }

func TestMain(m *testing.M) {
	// Run the benchmark
	result := testing.Benchmark(BenchmarkMmapAllocation)
	fmt.Println("Mmap Allocation Benchmark Result:", result)

	result2 := testing.Benchmark(BenchmarkSyncPoolAllocation)
	fmt.Println("Sync Pool Allocation Benchmark Result:", result2)
}
