// Latency profiler for distributed cache chunk operations.
// Measures per-operation timing for single-chunk downloads at various sizes.
package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
	"time"

	dcache "github.com/nearora-msft/dist-cache-client-go"
)

func main() {
	servers := os.Getenv("DCACHE_SERVERS")
	if servers == "" {
		fmt.Fprintln(os.Stderr, "DCACHE_SERVERS required")
		os.Exit(1)
	}

	addrs := strings.Split(servers, ",")
	iters := 100

	fmt.Printf("=== Latency Profiler ===\nServers: %d | Iterations: %d\n\n", len(addrs), iters)

	for _, chunkMB := range []int{4, 16, 32, 64, 128} {
		profileChunkSize(addrs, int64(chunkMB)*1024*1024, chunkMB, iters)
	}

	// Multi-chunk throughput profile
	fmt.Println("=== Multi-chunk Download Profile ===")
	for _, parallel := range []int{4, 8, 16, 32} {
		profileMultiChunk(addrs, 1024*1024*1024, 32*1024*1024, parallel, 10)
	}
}

func profileChunkSize(addrs []string, chunkSize int64, chunkMB int, iters int) {
	ctx := context.Background()
	client, err := dcache.New(
		dcache.WithServerList(addrs),
		dcache.WithChunkSize(chunkSize),
		dcache.WithMaxParallelOps(1),
		dcache.WithRequestTimeout(30*time.Second),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "client error: %v\n", err)
		return
	}
	defer client.Close()

	key := fmt.Sprintf("latency-probe/%dMB", chunkMB)
	data := make([]byte, chunkSize)
	rand.Read(data)
	if err := client.Upload(ctx, key, "", bytes.NewReader(data), chunkSize, dcache.WithIgnoreLock(true)); err != nil {
		fmt.Fprintf(os.Stderr, "upload %dMB failed: %v\n", chunkMB, err)
		return
	}

	// Warmup
	for i := 0; i < 5; i++ {
		var buf bytes.Buffer
		client.DownloadWithSize(ctx, key, chunkSize, &buf)
	}

	var latencies []time.Duration
	for i := 0; i < iters; i++ {
		var buf bytes.Buffer
		buf.Grow(int(chunkSize))
		start := time.Now()
		_, err := client.DownloadWithSize(ctx, key, chunkSize, &buf)
		if err != nil {
			continue
		}
		latencies = append(latencies, time.Since(start))
	}

	if len(latencies) == 0 {
		fmt.Printf("--- %dMB: no successful reads ---\n", chunkMB)
		return
	}

	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
	p50 := latencies[pctl(len(latencies), 50)]
	p50MB := float64(chunkMB) / p50.Seconds()
	netTimeMS := float64(chunkMB) * 8 / 64 * 1000 // at 64 Gbps
	overhead := ms(p50) - netTimeMS

	fmt.Printf("--- %3dMB chunk: p50=%.1fms  min=%.1fms  max=%.1fms  | p50 tput=%.0f MB/s  | net=%.1fms overhead=%.1fms (%.0f%%)\n",
		chunkMB, ms(p50), ms(latencies[0]), ms(latencies[len(latencies)-1]),
		p50MB, netTimeMS, overhead, overhead/ms(p50)*100)
}

func profileMultiChunk(addrs []string, fileSize int64, chunkSize int64, parallel int, iters int) {
	ctx := context.Background()
	client, err := dcache.New(
		dcache.WithServerList(addrs),
		dcache.WithChunkSize(chunkSize),
		dcache.WithMaxParallelOps(parallel),
		dcache.WithRequestTimeout(60*time.Second),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "client error: %v\n", err)
		return
	}
	defer client.Close()

	key := fmt.Sprintf("latency-probe/1GB-c%d", chunkSize/1024/1024)

	// Ensure data exists
	data := make([]byte, fileSize)
	rand.Read(data)
	if err := client.Upload(ctx, key, "", bytes.NewReader(data), fileSize, dcache.WithIgnoreLock(true)); err != nil {
		// May already exist
		fmt.Fprintf(os.Stderr, "upload 1GB: %v (continuing)\n", err)
	}
	data = nil

	// Warmup
	var buf bytes.Buffer
	buf.Grow(int(fileSize))
	client.DownloadWithSize(ctx, key, fileSize, &buf)
	buf.Reset()

	var latencies []time.Duration
	for i := 0; i < iters; i++ {
		buf.Reset()
		start := time.Now()
		_, err := client.DownloadWithSize(ctx, key, fileSize, &buf)
		if err != nil {
			fmt.Fprintf(os.Stderr, "P%d iter %d: %v\n", parallel, i, err)
			continue
		}
		latencies = append(latencies, time.Since(start))
	}

	if len(latencies) == 0 {
		fmt.Printf("  P%d: no successful reads\n", parallel)
		return
	}

	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
	p50 := latencies[pctl(len(latencies), 50)]
	p50MB := float64(fileSize) / p50.Seconds() / (1024 * 1024)

	fmt.Printf("  P%-3d: p50=%.0fms  min=%.0fms  max=%.0fms  → %.0f MB/s\n",
		parallel, ms(p50), ms(latencies[0]), ms(latencies[len(latencies)-1]), p50MB)
}

func ms(d time.Duration) float64 { return float64(d.Microseconds()) / 1000.0 }

func pctl(n int, p int) int {
	idx := int(math.Round(float64(n-1) * float64(p) / 100))
	if idx < 0 {
		return 0
	}
	if idx >= n {
		return n - 1
	}
	return idx
}
