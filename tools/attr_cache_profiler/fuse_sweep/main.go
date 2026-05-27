/*
   FUSE cache warmer for attr_cache live profiling.

   positive mode: warms attr_cache with synthetic positive entries.
     -sweep stat     (default) stats N paths with the dummy_positive_entry prefix;
                     loopback_fs.GetAttr intercepts these and returns a synthetic
                     ObjAttr without touching disk.
     -sweep readdir  triggers ReadDir on the mount root → StreamDir/cacheAttributes
                     inserts every real entry in one pass.

   negative mode: stats N paths with the dummy_negative_entry prefix;
                  loopback_fs.GetAttr intercepts these and returns ENOENT,
                  causing attr_cache to cache a negative entry per path.

   All generated filenames are 60 bytes (20-char prefix + 40-digit index).

   Usage:
     fuse_sweep -mount ~/mnt -mode positive
     fuse_sweep -mount ~/mnt -mode positive -sweep readdir
     fuse_sweep -mount ~/mnt -mode negative -n 5000000 -workers 16
*/

package main

import (
	"flag"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

func main() {
	mount := flag.String("mount", os.Getenv("HOME")+"/mnt", "FUSE mount point")
	mode := flag.String("mode", "positive", "positive | negative")
	sweep := flag.String("sweep", "stat", "positive mode sweep method: stat | readdir")
	n := flag.Int("n", 5_000_000, "number of entries")
	workers := flag.Int("workers", 16, "parallel goroutines; FUSE max_background is 12 by default so >16 rarely helps")
	flag.Parse()

	switch *mode {
	case "positive":
		sweepPositive(*mount, *sweep, *n, *workers)
	case "negative":
		sweepNegative(*mount, *n, *workers)
	default:
		fmt.Fprintln(os.Stderr, "mode must be positive or negative")
		os.Exit(1)
	}
}

func sweepPositive(mount, sweep string, n, workers int) {
	switch sweep {
	case "readdir":
		sweepPositiveReaddir(mount)
	case "stat":
		sweepPositiveStat(mount, n, workers)
	default:
		fmt.Fprintf(os.Stderr, "sweep must be stat or readdir\n")
		os.Exit(1)
	}
}

func sweepPositiveReaddir(mount string) {
	fmt.Printf("ReadDir on %s to populate attr_cache with positive entries...\n", mount)
	t0 := time.Now()

	entries, err := os.ReadDir(mount)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ReadDir failed: %v\n", err)
		os.Exit(1)
	}

	elapsed := time.Since(t0)
	fmt.Printf("Done: %d entries cached in %v (%.0f entries/sec)\n",
		len(entries), elapsed, float64(len(entries))/elapsed.Seconds())
}

// sweepPositiveStat stats N synthetic paths (60-byte filenames: dummy_positive_entry + 40-digit index).
// loopback_fs.GetAttr intercepts any name containing "dummy_positive_entry" and returns a
// synthetic ObjAttr, so no real files need to exist on disk.
func sweepPositiveStat(mount string, n, workers int) {
	fmt.Printf("Statting %d synthetic positive paths under %s (%d workers)...\n", n, mount, workers)
	t0 := time.Now()

	var cached atomic.Int64
	var errors atomic.Int64

	jobs := make(chan int, workers*2)
	var wg sync.WaitGroup

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var st syscall.Stat_t
			for idx := range jobs {
				path := fmt.Sprintf("%s/dummy_positive_entry%040d", mount, idx)
				if err := syscall.Stat(path, &st); err == nil {
					cached.Add(1)
				} else {
					errors.Add(1)
				}
				if c := cached.Load(); c > 0 && c%500_000 == 0 {
					rate := float64(c) / time.Since(t0).Seconds()
					fmt.Printf("  %d / %d cached  (%.0f/sec, ETA %.0fs)\n",
						c, n, rate, float64(n-int(c))/rate)
				}
			}
		}()
	}

	for i := 0; i < n; i++ {
		jobs <- i
	}
	close(jobs)
	wg.Wait()

	elapsed := time.Since(t0)
	fmt.Printf("Done: %d entries cached, %d errors, in %v (%.0f entries/sec)\n",
		cached.Load(), errors.Load(), elapsed, float64(cached.Load())/elapsed.Seconds())
}

// sweepNegative stats N synthetic paths (60-byte filenames: dummy_negative_entry + 40-digit index).
// loopback_fs.GetAttr intercepts any name containing "dummy_negative_entry" and returns ENOENT,
// causing attr_cache to cache a negative entry per path without any backend I/O.
func sweepNegative(mount string, n, workers int) {
	fmt.Printf("Statting %d synthetic negative paths under %s (%d workers)...\n", n, mount, workers)
	t0 := time.Now()

	var cached atomic.Int64
	var errors atomic.Int64

	jobs := make(chan int, workers*2)
	var wg sync.WaitGroup

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var st syscall.Stat_t
			for idx := range jobs {
				path := fmt.Sprintf("%s/dummy_negative_entry%040d", mount, idx)
				err := syscall.Stat(path, &st)
				if err == syscall.ENOENT {
					cached.Add(1)
				} else {
					errors.Add(1)
				}
				if c := cached.Load(); c > 0 && c%500_000 == 0 {
					rate := float64(c) / time.Since(t0).Seconds()
					fmt.Printf("  %d / %d cached  (%.0f/sec, ETA %.0fs)\n",
						c, n, rate, float64(n-int(c))/rate)
				}
			}
		}()
	}

	for i := 0; i < n; i++ {
		jobs <- i
	}
	close(jobs)
	wg.Wait()

	elapsed := time.Since(t0)
	fmt.Printf("Done: %d ENOENT entries cached, %d errors, in %v (%.0f entries/sec)\n",
		cached.Load(), errors.Load(), elapsed, float64(cached.Load())/elapsed.Seconds())
}
