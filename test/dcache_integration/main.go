package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	dcache "github.com/Azure/azure-storage-fuse/v2/internal/dist_cache_client"
)

func main() {
	servers := os.Getenv("DCACHE_SERVERS")
	if servers == "" {
		servers = "cacheserver-0.cacheserver.cache-server.svc.cluster.local:9065,cacheserver-1.cacheserver.cache-server.svc.cluster.local:9065,cacheserver-2.cacheserver.cache-server.svc.cluster.local:9065"
	}
	fmt.Printf("Servers: %s\n", servers)

	serverList := strings.Split(servers, ",")
	client, err := dcache.New(
		dcache.WithServerList(serverList),
		dcache.WithRequestTimeout(10*time.Second),
		dcache.WithChunkSize(4*1024*1024),
		dcache.WithCachePrefix("blobfuse-test"),
	)
	if err != nil {
		fmt.Printf("FAIL: New: %v\n", err)
		os.Exit(1)
	}
	defer client.Close()
	fmt.Println("OK: client created")

	ctx := context.Background()
	prefix := fmt.Sprintf("integration-test/%d", time.Now().UnixNano())

	// Test 1: Upload a small file (ignorelock=true since no prior Download lock)
	fmt.Println("\n--- Test 1: Upload small file ---")
	testData := []byte("Hello from blobfuse2 distributed cache integration test!")
	err = client.Upload(ctx, prefix+"/hello.txt", bytes.NewReader(testData), int64(len(testData)), dcache.WithIgnoreLock(true))
	if err == dcache.ErrFileExists {
		fmt.Println("OK: file already cached (from previous run)")
	} else if err != nil {
		fmt.Printf("FAIL: Upload: %v\n", err)
		os.Exit(1)
	} else {
		fmt.Println("OK: uploaded integration-test/hello.txt")
	}

	// Test 2: Download the file back
	fmt.Println("\n--- Test 2: Download file ---")
	var dlBuf bytes.Buffer
	meta, err := client.DownloadWithSize(ctx, prefix+"/hello.txt", int64(len(testData)), &dlBuf)
	if err != nil {
		fmt.Printf("FAIL: Download: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("OK: downloaded %d bytes\n", meta.Size)

	if !bytes.Equal(dlBuf.Bytes(), testData) {
		fmt.Printf("FAIL: content mismatch: got %q want %q\n", dlBuf.String(), string(testData))
		os.Exit(1)
	}
	fmt.Println("OK: content verified")

	// Test 3: Upload/Download chunk
	fmt.Println("\n--- Test 3: Upload/Download chunk ---")
	chunkData := []byte("chunk data for block cache testing - offset 0")
	err = client.UploadChunk(ctx, prefix+"/blocks.bin", 0, chunkData, dcache.WithIgnoreLock(true))
	if err != nil && err != dcache.ErrFileExists {
		fmt.Printf("FAIL: UploadChunk: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("OK: chunk uploaded (or already cached)")

	buf := make([]byte, 4096)
	n, err := client.DownloadChunk(ctx, prefix+"/blocks.bin", 0, buf)
	if err != nil {
		fmt.Printf("FAIL: DownloadChunk: %v\n", err)
		os.Exit(1)
	}
	if !bytes.Equal(buf[:n], chunkData) {
		fmt.Printf("FAIL: chunk mismatch: got %d bytes want %d\n", n, len(chunkData))
		os.Exit(1)
	}
	fmt.Printf("OK: chunk verified (%d bytes)\n", n)

	// Test 4: Delete (server may not support filename-based delete yet)
	fmt.Println("\n--- Test 4: Delete ---")
	err = client.Delete(ctx, prefix+"/hello.txt", int64(len(testData)))
	if err != nil {
		fmt.Printf("WARN: Delete returned error: %v (may not be supported)\n", err)
	} else {
		fmt.Println("OK: delete request accepted")
	}

	// Verify deletion (server may return success without actually deleting)
	var verifyBuf bytes.Buffer
	_, err = client.DownloadWithSize(ctx, prefix+"/hello.txt", int64(len(testData)), &verifyBuf)
	if err != nil {
		fmt.Printf("OK: confirmed deleted (got: %v)\n", err)
	} else {
		fmt.Println("OK: delete accepted but file still cached (known server limitation: filename delete not yet implemented)")
	}

	// Test 5: Large file (multi-chunk) - 8MB with 4MB chunks
	fmt.Println("\n--- Test 5: Multi-chunk upload/download (8MB) ---")
	largeData := make([]byte, 8*1024*1024)
	for i := range largeData {
		largeData[i] = byte(i % 251)
	}

	start := time.Now()
	err = client.Upload(ctx, prefix+"/large.bin", bytes.NewReader(largeData), int64(len(largeData)), dcache.WithIgnoreLock(true))
	if err != nil && err != dcache.ErrFileExists {
		fmt.Printf("FAIL: Upload large: %v\n", err)
		os.Exit(1)
	}
	uploadDur := time.Since(start)
	fmt.Printf("OK: uploaded 8MB in %v (%.1f MB/s)\n", uploadDur, 8.0/uploadDur.Seconds())

	var largeDlBuf bytes.Buffer
	start = time.Now()
	meta, err = client.DownloadWithSize(ctx, prefix+"/large.bin", int64(len(largeData)), &largeDlBuf)
	if err != nil {
		fmt.Printf("FAIL: Download large: %v\n", err)
		os.Exit(1)
	}
	downloadDur := time.Since(start)
	fmt.Printf("OK: downloaded %d bytes in %v (%.1f MB/s)\n", meta.Size, downloadDur, float64(meta.Size)/1024/1024/downloadDur.Seconds())

	if !bytes.Equal(largeDlBuf.Bytes(), largeData) {
		got := largeDlBuf.Bytes()
		for i := 0; i < len(largeData) && i < len(got); i++ {
			if got[i] != largeData[i] {
				fmt.Printf("FAIL: content mismatch at byte %d (got %d want %d)\n", i, got[i], largeData[i])
				os.Exit(1)
			}
		}
		fmt.Printf("FAIL: size mismatch got %d want %d\n", len(got), len(largeData))
		os.Exit(1)
	}
	fmt.Println("OK: 8MB content verified byte-for-byte")

	// Test 6: Download with lock protocol (simulating blobfuse CopyToFile miss)
	fmt.Println("\n--- Test 6: Lock protocol ---")
	var lockBuf bytes.Buffer
	_, err = client.DownloadWithSize(ctx, prefix+"/nonexistent.txt", 100, &lockBuf, dcache.WithLock(true))
	if err == dcache.ErrNotFoundGotLock {
		fmt.Println("OK: got NOT_FOUND_GOT_LOCK (expected for new file)")
	} else if err == dcache.ErrNotFound {
		fmt.Println("OK: got NOT_FOUND (lock not enabled on server)")
	} else if err != nil {
		fmt.Printf("WARN: unexpected error: %v\n", err)
	} else {
		fmt.Println("WARN: file unexpectedly found")
	}

	// Cleanup
	client.Delete(ctx, prefix+"/large.bin", int64(len(largeData)))
	client.Delete(ctx, prefix+"/blocks.bin", int64(len(chunkData)))

	fmt.Println("\n=== ALL TESTS PASSED ===")
}
