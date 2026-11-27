//go:build unittest && !fuse2

// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package libfuse

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// BenchmarkCallback measures the read and write callbacks of libfuse, called from C the way libfuse worker threads
// call them, without a FUSE mount. "pipeline" files are not served natively, so every call goes into Go and to the
// next component, which does no work. "native" files are served in C from a local file, as with file_cache.
//
// CI compares these benchmarks between a pull request and its base, see test/scripts/libfuse_bench_compare.sh.
func BenchmarkCallback(b *testing.B) {
	local := filepath.Join(b.TempDir(), "bench")
	if err := os.WriteFile(local, make([]byte, 4096), 0600); err != nil {
		b.Fatal(err)
	}

	for _, op := range []string{"read", "write"} {
		for _, threads := range []int{1, 4} {
			b.Run(fmt.Sprintf("%s/pipeline/threads=%d", op, threads), func(b *testing.B) {
				benchFileIO(b, &benchComponent{}, op == "write", 4096, threads)
			})
		}
		b.Run(op+"/native/threads=1", func(b *testing.B) {
			benchFileIO(b, &benchComponent{localPath: local}, op == "write", 4096, 1)
		})
	}
}
