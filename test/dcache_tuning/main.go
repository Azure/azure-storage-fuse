package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	dcache "github.com/nearora-msft/dist-cache-client-go"
)

const defaultIters = 5

func main() {
	acctName := os.Getenv("AZURE_STORAGE_ACCOUNT")
	sasToken := os.Getenv("AZURE_SAS_TOKEN")
	container := os.Getenv("AZURE_CONTAINER")
	if container == "" {
		container = "test-data"
	}
	if acctName == "" || sasToken == "" {
		fmt.Println("ERROR: Set AZURE_STORAGE_ACCOUNT and AZURE_SAS_TOKEN")
		os.Exit(1)
	}

	servers := os.Getenv("DCACHE_SERVERS")
	if servers == "" {
		fmt.Println("ERROR: Set DCACHE_SERVERS")
		os.Exit(1)
	}
	serverList := strings.Split(servers, ",")

	prefix := os.Getenv("BENCH_PREFIX")
	if prefix == "" {
		prefix = fmt.Sprintf("bench-tune/%d", time.Now().UnixNano())
	}

	iters := defaultIters
	if v := os.Getenv("BENCH_ITERS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			iters = n
		}
	}

	// File sizes to test
	sizes := []int64{
		128 * 1024 * 1024,
		256 * 1024 * 1024,
		512 * 1024 * 1024,
		1024 * 1024 * 1024,
	}

	// Configurations to sweep
	type config struct {
		label       string
		chunkSizeMB int
		parallelOps int
		sockBufMB   int
	}

	configs := []config{
		// Baseline (old defaults for comparison)
		{"P8/C32/S0", 32, 8, 0},
		// Top configs from initial sweep
		{"P32/C32/S4", 32, 32, 4},
		{"P32/C32/S16", 32, 32, 16},
		{"P32/C64/S4", 64, 32, 4},
		{"P32/C64/S16", 64, 32, 16},
		// Extremes
		{"P64/C32/S4", 32, 64, 4},
		{"P16/C64/S4", 64, 16, 4},
		{"P32/C128/S4", 128, 32, 4},
	}

	blobBase := fmt.Sprintf("https://%s.blob.core.windows.net", acctName)
	hostname, _ := os.Hostname()

	fmt.Println("=== Distributed Cache Tuning Benchmark ===")
	fmt.Printf("Node: %s | Servers: %d | Iters: %d\n", hostname, len(serverList), iters)
	fmt.Printf("Prefix: %s\n\n", prefix)

	mode := "full"
	if len(os.Args) > 1 {
		mode = os.Args[1]
	}

	// Collect unique chunk sizes
	chunkSizes := map[int]bool{}
	for _, cfg := range configs {
		chunkSizes[cfg.chunkSizeMB] = true
	}

	// Phase 1: Upload blob data once
	if mode == "full" {
		fmt.Println("--- Uploading test data to Azure Blob ---")
		for _, sz := range sizes {
			label := fmt.Sprintf("%dMB", sz/1024/1024)
			blobPath := fmt.Sprintf("%s/%s.bin", prefix, label)
			fmt.Printf("  %s uploading...", label)
			start := time.Now()
			err := uploadBlob(blobBase, sasToken, container, blobPath, sz)
			dur := time.Since(start)
			if err != nil {
				fmt.Printf(" FAIL: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf(" done in %s (%.0f MB/s)\n", dur.Round(time.Millisecond), mbs(sz, dur))
		}
	}

	// Phase 2: For each chunk size, populate cache (separate namespace per chunk size)
	if mode == "full" {
		fmt.Println("\n--- Populating cache for each chunk size ---")
		for csz := range chunkSizes {
			chunkBytes := int64(csz) * 1024 * 1024
			cl, err := dcache.New(
				dcache.WithServerList(serverList),
				dcache.WithRequestTimeout(300*time.Second),
				dcache.WithChunkSize(chunkBytes),
				dcache.WithCachePrefix(container),
				dcache.WithMaxParallelOps(16),
			)
			if err != nil {
				fmt.Printf("  FAIL: dcache.New for chunk=%dMB: %v\n", csz, err)
				continue
			}
			ctx := context.Background()
			for _, sz := range sizes {
				label := fmt.Sprintf("%dMB", sz/1024/1024)
				blobPath := fmt.Sprintf("%s/%s.bin", prefix, label)
				cachePath := fmt.Sprintf("%s/%s/c%d/%s", container, prefix, csz, label)

				data, err := downloadBlob(blobBase, sasToken, container, blobPath)
				if err != nil {
					fmt.Printf("  chunk=%dMB %s blob fetch FAIL: %v\n", csz, label, err)
					continue
				}
				err = cl.Upload(ctx, cachePath, "", bytes.NewReader(data), sz, dcache.WithIgnoreLock(true))
				if err != nil {
					fmt.Printf("  chunk=%dMB %s cache upload FAIL: %v\n", csz, label, err)
					continue
				}
				fmt.Printf("  chunk=%dMB %s populated\n", csz, label)
			}
			cl.Close()
			runtime.GC()
		}
	}

	// Phase 3: Run each configuration
	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("=== CONFIGURATION SWEEP ===")
	fmt.Println(strings.Repeat("=", 80))

	type resultEntry struct {
		config  string
		size    int64
		median  float64
		min     float64
		max     float64
		allMBps []float64
	}
	var allResults []resultEntry

	for _, cfg := range configs {
		chunkBytes := int64(cfg.chunkSizeMB) * 1024 * 1024
		sockBufBytes := cfg.sockBufMB * 1024 * 1024

		opts := []dcache.Option{
			dcache.WithServerList(serverList),
			dcache.WithRequestTimeout(300 * time.Second),
			dcache.WithChunkSize(chunkBytes),
			dcache.WithCachePrefix(container),
			dcache.WithMaxParallelOps(cfg.parallelOps),
			dcache.WithSocketBufferSize(sockBufBytes),
		}

		cl, err := dcache.New(opts...)
		if err != nil {
			fmt.Printf("\nConfig %s: FAIL: %v\n", cfg.label, err)
			continue
		}

		fmt.Printf("\n--- Config: %s (chunk=%dMB, parallel=%d, sockbuf=%dMB) ---\n",
			cfg.label, cfg.chunkSizeMB, cfg.parallelOps, cfg.sockBufMB)

		ctx := context.Background()

		for _, sz := range sizes {
			label := fmt.Sprintf("%dMB", sz/1024/1024)
			cachePath := fmt.Sprintf("%s/%s/c%d/%s", container, prefix, cfg.chunkSizeMB, label)

			// Warmup read (untimed) - primes connections
			buf := make([]byte, sz)
			w := bytes.NewBuffer(buf[:0])
			_, _ = cl.DownloadWithSize(ctx, cachePath, sz, w)
			runtime.GC()

			// Timed iterations
			var mbpsList []float64
			for iter := 0; iter < iters; iter++ {
				runtime.GC()
				w := bytes.NewBuffer(buf[:0])
				start := time.Now()
				_, err := cl.DownloadWithSize(ctx, cachePath, sz, w)
				dur := time.Since(start)
				if err != nil {
					fmt.Printf("  %s iter %d FAIL: %v\n", label, iter+1, err)
					continue
				}
				mbpsList = append(mbpsList, mbs(sz, dur))
			}

			if len(mbpsList) == 0 {
				continue
			}

			sort.Float64s(mbpsList)
			med := mbpsList[len(mbpsList)/2]
			minV := mbpsList[0]
			maxV := mbpsList[len(mbpsList)-1]

			allResults = append(allResults, resultEntry{
				config:  cfg.label,
				size:    sz,
				median:  med,
				min:     minV,
				max:     maxV,
				allMBps: mbpsList,
			})

			var parts []string
			for _, v := range mbpsList {
				parts = append(parts, fmt.Sprintf("%.0f", v))
			}
			fmt.Printf("  %-6s  med=%-6.0f  min=%-6.0f  max=%-6.0f  [%s]\n",
				label, med, minV, maxV, strings.Join(parts, " "))
		}

		cl.Close()
		runtime.GC()
	}

	// Summary table
	fmt.Println("\n" + strings.Repeat("=", 110))
	fmt.Println("=== SUMMARY: Median Throughput (MB/s) by Config x File Size ===")
	fmt.Println(strings.Repeat("=", 110))

	fmt.Printf("%-20s", "Config")
	for _, sz := range sizes {
		fmt.Printf(" | %8dMB", sz/1024/1024)
	}
	fmt.Println()
	fmt.Print(strings.Repeat("-", 20))
	for range sizes {
		fmt.Print(" | " + strings.Repeat("-", 10))
	}
	fmt.Println()

	for _, cfg := range configs {
		fmt.Printf("%-20s", cfg.label)
		for _, sz := range sizes {
			found := false
			for _, r := range allResults {
				if r.config == cfg.label && r.size == sz {
					fmt.Printf(" | %10.0f", r.median)
					found = true
					break
				}
			}
			if !found {
				fmt.Printf(" | %10s", "N/A")
			}
		}
		fmt.Println()
	}

	fmt.Println("\n=== TUNING COMPLETE ===")
}

func mbs(size int64, d time.Duration) float64 {
	if d <= 0 {
		return 0
	}
	return float64(size) / (1024 * 1024) / d.Seconds()
}

func uploadBlob(base, sas, container, path string, size int64) error {
	url := fmt.Sprintf("%s/%s/%s?%s", base, container, path, sas)
	data := make([]byte, size)
	rand.Read(data)
	req, err := http.NewRequest("PUT", url, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("x-ms-blob-type", "BlockBlob")
	req.ContentLength = size

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	if resp.StatusCode != 201 {
		return fmt.Errorf("upload status %d", resp.StatusCode)
	}
	return nil
}

func downloadBlob(base, sas, container, path string) ([]byte, error) {
	url := fmt.Sprintf("%s/%s/%s?%s", base, container, path, sas)
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("download status %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}
