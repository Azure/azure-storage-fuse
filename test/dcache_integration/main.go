package main

import (
	"bufio"
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

// metricsSnapshot holds aggregated counters from all cache servers.
type metricsSnapshot struct {
	downloadSuccess int64
	downloadMiss    int64 // InvalidTransition = cache miss / not-found
	uploadSuccess   int64
	cacheSizeBytes  int64
	perServer       map[string]serverMetrics
}

type serverMetrics struct {
	downloadSuccess int64
	downloadMiss    int64
	uploadSuccess   int64
	cacheSizeBytes  int64
}

// fetchMetrics queries the prometheus endpoint on all cache servers and aggregates counters.
func fetchMetrics(metricsURLs []string) metricsSnapshot {
	snap := metricsSnapshot{perServer: make(map[string]serverMetrics)}
	for _, u := range metricsURLs {
		body, err := httpGet(u)
		if err != nil {
			continue
		}
		sm := parseServerMetrics(body)
		// extract instance name from URL for labeling
		instance := u
		if idx := strings.Index(u, "//"); idx >= 0 {
			instance = u[idx+2:]
		}
		if idx := strings.Index(instance, ":"); idx >= 0 {
			instance = instance[:idx]
		}
		snap.perServer[instance] = sm
		snap.downloadSuccess += sm.downloadSuccess
		snap.downloadMiss += sm.downloadMiss
		snap.uploadSuccess += sm.uploadSuccess
		snap.cacheSizeBytes += sm.cacheSizeBytes
	}
	return snap
}

func httpGet(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	return string(b), err
}

func parseServerMetrics(body string) serverMetrics {
	var sm serverMetrics
	scanner := bufio.NewScanner(strings.NewReader(body))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "#") {
			continue
		}
		// cache_server_request_counter{instance="X",request_type="Download",status="Success"} 852
		if strings.Contains(line, "cache_server_request_counter") {
			if strings.Contains(line, `request_type="Download"`) && strings.Contains(line, `status="Success"`) {
				sm.downloadSuccess = parseMetricValue(line)
			} else if strings.Contains(line, `request_type="Download"`) && strings.Contains(line, `status="InvalidTransition"`) {
				sm.downloadMiss = parseMetricValue(line)
			} else if strings.Contains(line, `request_type="Upload"`) && strings.Contains(line, `status="Success"`) {
				sm.uploadSuccess = parseMetricValue(line)
			}
		} else if strings.Contains(line, "cache_server_cache_size_bytes{") {
			sm.cacheSizeBytes = parseMetricValue(line)
		}
	}
	return sm
}

func parseMetricValue(line string) int64 {
	parts := strings.Fields(line)
	if len(parts) < 2 {
		return 0
	}
	v, _ := strconv.ParseFloat(parts[len(parts)-1], 64)
	return int64(v)
}

// metricsDelta computes the difference between two snapshots.
func metricsDelta(before, after metricsSnapshot) metricsSnapshot {
	return metricsSnapshot{
		downloadSuccess: after.downloadSuccess - before.downloadSuccess,
		downloadMiss:    after.downloadMiss - before.downloadMiss,
		uploadSuccess:   after.uploadSuccess - before.uploadSuccess,
		cacheSizeBytes:  after.cacheSizeBytes,
	}
}

func hitRate(hits, misses int64) string {
	total := hits + misses
	if total == 0 {
		return "n/a"
	}
	return fmt.Sprintf("%.1f%% (%d/%d)", float64(hits)/float64(total)*100, hits, total)
}

// metricsURLsFromServers converts data-port server addresses to metrics URLs.
func metricsURLsFromServers(dataServers []string) []string {
	var urls []string
	for _, s := range dataServers {
		// Replace data port (9065) with metrics port (9096)
		host := s
		if idx := strings.LastIndex(s, ":"); idx >= 0 {
			host = s[:idx]
		}
		urls = append(urls, fmt.Sprintf("http://%s:9096/metrics", host))
	}
	return urls
}

const (
	blobIters  = 10 // iterations for blob reads
	cacheIters = 10 // iterations for cache reads (+ 1 untimed warmup)
	chunkSize  = 32 * 1024 * 1024
)

func mbs(size int64, dur time.Duration) float64 {
	if dur == 0 {
		return 0
	}
	return (float64(size) / 1024 / 1024) / dur.Seconds()
}

func fmtSpeed(size int64, dur time.Duration) string {
	if dur == 0 {
		return "n/a"
	}
	return fmt.Sprintf("%.0f MB/s", mbs(size, dur))
}

func median(durs []time.Duration) time.Duration {
	if len(durs) == 0 {
		return 0
	}
	sorted := make([]time.Duration, len(durs))
	copy(sorted, durs)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	mid := len(sorted) / 2
	if len(sorted)%2 == 0 {
		return (sorted[mid-1] + sorted[mid]) / 2
	}
	return sorted[mid]
}

func minDur(durs []time.Duration) time.Duration {
	if len(durs) == 0 {
		return 0
	}
	m := durs[0]
	for _, d := range durs[1:] {
		if d < m {
			m = d
		}
	}
	return m
}

func maxDur(durs []time.Duration) time.Duration {
	if len(durs) == 0 {
		return 0
	}
	m := durs[0]
	for _, d := range durs[1:] {
		if d > m {
			m = d
		}
	}
	return m
}

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
		servers = "cacheserver-0.cacheserver.cache-server.svc.cluster.local:9065," +
			"cacheserver-1.cacheserver.cache-server.svc.cluster.local:9065," +
			"cacheserver-2.cacheserver.cache-server.svc.cluster.local:9065"
	}

	mode := "full"
	if len(os.Args) > 1 {
		mode = os.Args[1]
	}

	sizes := []int64{
		32 * 1024 * 1024,
		64 * 1024 * 1024,
		128 * 1024 * 1024,
		256 * 1024 * 1024,
		512 * 1024 * 1024,
		1024 * 1024 * 1024,
	}

	prefix := os.Getenv("BENCH_PREFIX")
	if prefix == "" {
		prefix = fmt.Sprintf("bench-xnode/%d", time.Now().UnixNano())
	}
	hostname, _ := os.Hostname()

	fmt.Println("=== Azure Blob vs Distributed Cache Benchmark ===")
	fmt.Printf("Mode: %s | Node: %s\n", mode, hostname)
	fmt.Printf("Storage: %s/%s | Chunk: %d MB\n", acctName, container, chunkSize/1024/1024)
	fmt.Printf("Blob iters: %d | Cache iters: %d (1 cold + %d warm)\n", blobIters, cacheIters, cacheIters-1)
	fmt.Printf("Prefix: %s\n\n", prefix)

	type testFile struct {
		label     string
		blobPath  string
		cachePath string
		size      int64
	}
	files := make([]testFile, len(sizes))
	for i, sz := range sizes {
		label := fmt.Sprintf("%dMB", sz/1024/1024)
		files[i] = testFile{
			label:     label,
			blobPath:  fmt.Sprintf("%s/%s.bin", prefix, label),
			cachePath: fmt.Sprintf("%s/%s/%s", container, prefix, label),
			size:      sz,
		}
	}

	blobBase := fmt.Sprintf("https://%s.blob.core.windows.net", acctName)

	type result struct {
		blobMedian  time.Duration
		blobAll     []time.Duration
		cacheCold   time.Duration
		cacheMedian time.Duration // median of warm iterations
		cacheAll    []time.Duration
		populate    time.Duration
		cacheHits   int64
		cacheMisses int64
		uploads     int64
	}
	results := make([]result, len(files))

	// Build metrics URLs from cache server addresses
	serverList := strings.Split(servers, ",")
	metricsURLs := metricsURLsFromServers(serverList)

	// === UPLOAD TEST DATA TO BLOB ===
	if mode == "full" {
		fmt.Println("--- Uploading random test data to Azure Blob ---")
		for _, f := range files {
			fmt.Printf("  %-6s uploading %d MB...", f.label, f.size/1024/1024)
			start := time.Now()
			err := uploadBlob(blobBase, sasToken, container, f.blobPath, f.size)
			dur := time.Since(start)
			if err != nil {
				fmt.Printf(" FAIL: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf(" done in %s (%.0f MB/s)\n", dur.Round(time.Millisecond), mbs(f.size, dur))
		}
	}

	// === BLOB READS (N iterations, report median) ===
	fmt.Printf("\n--- BASELINE: Azure Blob Storage reads (%d iterations) ---\n", blobIters)
	for i, f := range files {
		var durs []time.Duration
		for iter := 0; iter < blobIters; iter++ {
			// Force GC between iterations to reduce noise
			runtime.GC()
			start := time.Now()
			_, err := downloadBlob(blobBase, sasToken, container, f.blobPath)
			dur := time.Since(start)
			if err != nil {
				fmt.Printf("  %-6s blob iter %d FAIL: %v\n", f.label, iter+1, err)
				continue
			}
			durs = append(durs, dur)
		}
		results[i].blobAll = durs
		results[i].blobMedian = median(durs)

		var parts []string
		for j, d := range durs {
			parts = append(parts, fmt.Sprintf("#%d=%s", j+1, fmtSpeed(f.size, d)))
		}
		fmt.Printf("  %-6s median=%s  [%s]\n",
			f.label, fmtSpeed(f.size, results[i].blobMedian), strings.Join(parts, " "))
	}

	// === DISTRIBUTED CACHE ===
	client, err := dcache.New(
		dcache.WithServerList(serverList),
		dcache.WithRequestTimeout(300*time.Second),
		dcache.WithChunkSize(chunkSize),
		dcache.WithCachePrefix(container),
		dcache.WithMaxParallelOps(16),
	)
	if err != nil {
		fmt.Printf("FAIL: dcache.New: %v\n", err)
		os.Exit(1)
	}
	defer client.Close()
	ctx := context.Background()

	// cacheRead runs a single download into a pre-allocated buffer and returns duration.
	cacheRead := func(cl *dcache.Client, path string, size int64) (time.Duration, error) {
		buf := make([]byte, size)
		w := bytes.NewBuffer(buf[:0])
		start := time.Now()
		_, err := cl.DownloadWithSize(ctx, path, size, w)
		return time.Since(start), err
	}

	if mode == "full" {
		fmt.Printf("\n--- DISTRIBUTED CACHE: Node A (populate + %d reads) ---\n", cacheIters)
		for i, f := range files {
			// Snapshot metrics before this file
			metricsBefore := fetchMetrics(metricsURLs)

			// Download from blob into memory, then upload to cache
			start := time.Now()
			data, err := downloadBlob(blobBase, sasToken, container, f.blobPath)
			if err != nil {
				fmt.Printf("  %-6s blob fetch FAIL: %v\n", f.label, err)
				continue
			}
			err = client.Upload(ctx, f.cachePath, "", bytes.NewReader(data), f.size, dcache.WithIgnoreLock(true))
			populateDur := time.Since(start)
			if err != nil {
				fmt.Printf("  %-6s cache upload FAIL: %v\n", f.label, err)
				continue
			}
			results[i].populate = populateDur
			data = nil
			runtime.GC()

			// Untimed warmup read to prime connection pool (isolate upload heat)
			_, _ = cacheRead(client, f.cachePath, f.size)
			runtime.GC()

			// Snapshot metrics before timed reads
			metricsPreRead := fetchMetrics(metricsURLs)

			var allDurs []time.Duration
			for iter := 0; iter < cacheIters; iter++ {
				runtime.GC()
				dur, err := cacheRead(client, f.cachePath, f.size)
				if err != nil {
					fmt.Printf("  %-6s cache iter %d FAIL: %v\n", f.label, iter+1, err)
					continue
				}
				allDurs = append(allDurs, dur)
			}

			// Snapshot metrics after timed reads
			metricsPostRead := fetchMetrics(metricsURLs)

			// Compute deltas — uploads = populate phase, reads = timed read phase
			populateDelta := metricsDelta(metricsBefore, metricsPreRead)
			readDelta := metricsDelta(metricsPreRead, metricsPostRead)

			results[i].cacheAll = allDurs
			results[i].cacheHits = readDelta.downloadSuccess
			results[i].cacheMisses = readDelta.downloadMiss
			results[i].uploads = populateDelta.uploadSuccess
			if len(allDurs) > 0 {
				results[i].cacheCold = allDurs[0]
			}
			if len(allDurs) > 1 {
				results[i].cacheMedian = median(allDurs[1:])
			}

			expectedChunks := (f.size + chunkSize - 1) / chunkSize
			expectedReads := expectedChunks * int64(cacheIters+1) // +1 for warmup

			var parts []string
			for j, d := range allDurs {
				parts = append(parts, fmt.Sprintf("#%d=%s", j+1, fmtSpeed(f.size, d)))
			}
			fmt.Printf("  %-6s pop=%s  median=%s  [%s]\n",
				f.label,
				populateDur.Round(time.Millisecond),
				fmtSpeed(f.size, median(allDurs)),
				strings.Join(parts, " "))
			fmt.Printf("         metrics: uploads=%d (expected %d chunks) | read hits=%d misses=%d hit_rate=%s (expected %d chunk reads)\n",
				results[i].uploads, expectedChunks,
				results[i].cacheHits, results[i].cacheMisses,
				hitRate(results[i].cacheHits, results[i].cacheMisses),
				expectedReads)
		}
		fmt.Printf("\nFor Node B: BENCH_PREFIX=%s\n", prefix)

	} else {
		// Node B mode
		fmt.Printf("\n--- DISTRIBUTED CACHE: Node B reads (%d iterations) ---\n", cacheIters)
		for i, f := range files {
			// Untimed warmup read to prime connection pool
			_, _ = cacheRead(client, f.cachePath, f.size)
			runtime.GC()

			// Snapshot metrics before timed reads
			metricsPreRead := fetchMetrics(metricsURLs)

			var allDurs []time.Duration
			for iter := 0; iter < cacheIters; iter++ {
				runtime.GC()
				dur, err := cacheRead(client, f.cachePath, f.size)
				if err != nil {
					fmt.Printf("  %-6s cache iter %d FAIL: %v\n", f.label, iter+1, err)
					continue
				}
				allDurs = append(allDurs, dur)
			}

			// Snapshot metrics after timed reads
			metricsPostRead := fetchMetrics(metricsURLs)
			readDelta := metricsDelta(metricsPreRead, metricsPostRead)

			results[i].cacheAll = allDurs
			results[i].cacheHits = readDelta.downloadSuccess
			results[i].cacheMisses = readDelta.downloadMiss
			if len(allDurs) > 0 {
				results[i].cacheCold = allDurs[0]
			}
			if len(allDurs) > 1 {
				results[i].cacheMedian = median(allDurs[1:])
			}

			expectedChunks := (f.size + chunkSize - 1) / chunkSize
			expectedReads := expectedChunks * int64(cacheIters)

			var parts []string
			for j, d := range allDurs {
				parts = append(parts, fmt.Sprintf("#%d=%s", j+1, fmtSpeed(f.size, d)))
			}
			fmt.Printf("  %-6s median=%s  [%s]\n",
				f.label,
				fmtSpeed(f.size, median(allDurs)),
				strings.Join(parts, " "))
			fmt.Printf("         metrics: read hits=%d misses=%d hit_rate=%s (expected %d chunk reads)\n",
				results[i].cacheHits, results[i].cacheMisses,
				hitRate(results[i].cacheHits, results[i].cacheMisses),
				expectedReads)
		}
	}

	// === SUMMARY ===
	fmt.Println("\n========== SUMMARY (MB/s) — min / median / max ==========")
	fmt.Printf("%-6s │ %30s │ %30s │ %8s\n",
		"Size", "Blob (min/med/max)", "Cache (min/med/max)", "Speedup")
	fmt.Printf("%-6s │ %30s │ %30s │ %8s\n",
		"------", "------------------------------", "------------------------------", "--------")
	for i, f := range files {
		r := results[i]
		blobStr := "n/a"
		if len(r.blobAll) > 0 {
			blobStr = fmt.Sprintf("%4.0f / %4.0f / %4.0f",
				mbs(f.size, maxDur(r.blobAll)), // min speed = max duration
				mbs(f.size, median(r.blobAll)),
				mbs(f.size, minDur(r.blobAll))) // max speed = min duration
		}
		cacheStr := "n/a"
		if len(r.cacheAll) > 0 {
			cacheStr = fmt.Sprintf("%4.0f / %4.0f / %4.0f",
				mbs(f.size, maxDur(r.cacheAll)),
				mbs(f.size, median(r.cacheAll)),
				mbs(f.size, minDur(r.cacheAll)))
		}
		speedup := ""
		allCacheMedian := median(r.cacheAll)
		if r.blobMedian > 0 && allCacheMedian > 0 {
			speedup = fmt.Sprintf("%.1fx", float64(r.blobMedian)/float64(allCacheMedian))
		}
		fmt.Printf("%-6s │ %30s │ %30s │ %8s\n",
			f.label, blobStr, cacheStr, speedup)
	}

	// === METRICS SUMMARY ===
	fmt.Println("\n========== CACHE METRICS (timed reads only) ==========")
	fmt.Printf("%-6s │ %10s │ %10s │ %10s │ %20s\n",
		"Size", "Hits", "Misses", "Hit Rate", "Expected Chunks")
	fmt.Printf("%-6s │ %10s │ %10s │ %10s │ %20s\n",
		"------", "----------", "----------", "----------", "--------------------")
	var totalHits, totalMisses int64
	for i, f := range files {
		r := results[i]
		totalHits += r.cacheHits
		totalMisses += r.cacheMisses
		expectedChunks := (f.size + chunkSize - 1) / chunkSize
		iters := int64(cacheIters)
		if mode == "full" {
			iters += 1 // warmup counted in metrics
		}
		_ = iters
		fmt.Printf("%-6s │ %10d │ %10d │ %10s │ %20d\n",
			f.label, r.cacheHits, r.cacheMisses,
			hitRate(r.cacheHits, r.cacheMisses),
			expectedChunks*int64(cacheIters))
	}
	fmt.Printf("%-6s │ %10d │ %10d │ %10s │\n",
		"TOTAL", totalHits, totalMisses, hitRate(totalHits, totalMisses))

	// Final cache state
	finalMetrics := fetchMetrics(metricsURLs)
	fmt.Printf("\nCache state: %.1f GB across %d servers\n",
		float64(finalMetrics.cacheSizeBytes)/1024/1024/1024, len(metricsURLs))
	for inst, sm := range finalMetrics.perServer {
		fmt.Printf("  %s: %.1f GB, downloads=%d (hits=%d misses=%d) uploads=%d\n",
			inst, float64(sm.cacheSizeBytes)/1024/1024/1024,
			sm.downloadSuccess+sm.downloadMiss, sm.downloadSuccess, sm.downloadMiss,
			sm.uploadSuccess)
	}

	fmt.Println("\n=== COMPLETE ===")
}

func uploadBlob(blobBase, sas, container, path string, size int64) error {
	data := make([]byte, size)
	rand.Read(data)

	u := fmt.Sprintf("%s/%s/%s?%s", blobBase, container, path, sas)
	req, err := http.NewRequest("PUT", u, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("x-ms-blob-type", "BlockBlob")
	req.Header.Set("x-ms-version", "2023-11-03")
	req.ContentLength = size

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 201 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body[:min(len(body), 300)]))
	}
	return nil
}

func downloadBlob(blobBase, sas, container, path string) ([]byte, error) {
	u := fmt.Sprintf("%s/%s/%s?%s", blobBase, container, path, sas)
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-ms-version", "2023-11-03")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body[:min(len(body), 300)]))
	}
	return io.ReadAll(resp.Body)
}
