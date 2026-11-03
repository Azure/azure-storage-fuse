//go:build unittest && !fuse2

/*
    _____           _____   _____   ____          ______  _____  ------
   |     |  |      |     | |     | |     |     | |       |            |
   |     |  |      |     | |     | |     |     | |       |            |
   | --- |  |      |     | |-----| |---- |     | |-----| |-----  ------
   |     |  |      |     | |     | |     |     |       | |       |
   | ____|  |_____ | ____| | ____| |     |_____|  _____| |_____  |_____


   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2020-2026 Microsoft Corporation. All rights reserved.
   Author : <blobfusedev@microsoft.com>

   Permission is hereby granted, free of charge, to any person obtaining a copy
   of this software and associated documentation files (the "Software"), to deal
   in the Software without restriction, including without limitation the rights
   to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
   copies of the Software, and to permit persons to whom the Software is
   furnished to do so, subject to the following conditions:

   The above copyright notice and this permission notice shall be included in all
   copies or substantial portions of the Software.

   THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
   IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
   FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
   AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
   LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
   OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
   SOFTWARE
*/

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
