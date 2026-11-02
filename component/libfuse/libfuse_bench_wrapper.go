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

// Benchmark helpers that call into C. They live in a regular file because cgo cannot be used in _test.go files; the
// unittest build tag keeps them out of the blobfuse2 binary. See libfuse_bench_test.go.

// #cgo CFLAGS: -DFUSE_USE_VERSION=39 -D_FILE_OFFSET_BITS=64
// #cgo LDFLAGS: -lfuse3 -ldl
// #include "libfuse_wrapper.h"
//
// // bench_io_loop : call the read or write callback n times, the way a libfuse worker thread does
// static int bench_io_loop(int write, fuse_file_info_t *fi, char *buf, size_t size, long n)
// {
//     int failed = 0;
//     for (long i = 0; i < n; i++) {
//         int res = write ? native_write_file("/bench", buf, size, 0, fi)
//                         : native_read_file("/bench", buf, size, 0, fi);
//         failed |= res < 0;
//     }
//     return failed;
// }
import "C"

import (
	"os"
	"sync"
	"testing"
	"unsafe"

	"github.com/Azure/azure-storage-fuse/v2/internal"
	"github.com/Azure/azure-storage-fuse/v2/internal/handlemap"
)

// benchComponent is the next component of libfuse in the callback benchmarks. Without localPath, files are served
// through the pipeline and reads and writes do no work, so that only the cost of libfuse is measured. With localPath,
// files are served natively from that local file, like file_cache does.
type benchComponent struct {
	internal.BaseComponent
	localPath string
}

func (c *benchComponent) OpenFile(options internal.OpenFileOptions) (*handlemap.Handle, error) {
	handle := handlemap.NewHandle(options.Name)
	if c.localPath != "" {
		f, err := os.OpenFile(c.localPath, os.O_RDWR, 0)
		if err != nil {
			return nil, err
		}
		handle.SetFileObject(f)
		handle.UnixFD = uint64(f.Fd())
		handle.Flags.Set(handlemap.HandleFlagCached)
	}
	return handle, nil
}

func (c *benchComponent) ReadInBuffer(options *internal.ReadInBufferOptions) (int, error) {
	return len(options.Data), nil
}

func (c *benchComponent) WriteFile(options *internal.WriteFileOptions) (int, error) {
	return len(options.Data), nil
}

func (c *benchComponent) FlushFile(internal.FlushFileOptions) error {
	return nil
}

func (c *benchComponent) ReleaseFile(options internal.ReleaseFileOptions) error {
	if f := options.Handle.GetFileObject(); f != nil {
		return f.Close()
	}
	return nil
}

// benchFileIO opens a file per thread through libfuse_open, then calls the read or write callback of libfuse b.N
// times in total from C loops on that many threads. ns/op is the wall time divided by b.N, so with more than one
// thread it is the inverse of the combined throughput.
func benchFileIO(b *testing.B, next *benchComponent, write bool, size int, threads int) {
	lf := &Libfuse{}
	lf.SetNextComponent(next)
	saved := fuseFS
	fuseFS = lf
	defer func() { fuseFS = saved }()

	path := C.CString("/bench")
	defer C.free(unsafe.Pointer(path))

	type worker struct {
		fi  *C.fuse_file_info_t
		buf *C.char
	}
	workers := make([]worker, threads)
	for i := range workers {
		fi := (*C.fuse_file_info_t)(C.calloc(1, C.sizeof_fuse_file_info_t))
		fi.flags = C.O_RDWR
		if ret := libfuse_open(path, fi); ret != 0 {
			b.Fatalf("open failed: %d", ret)
		}
		workers[i] = worker{fi, (*C.char)(C.calloc(1, C.size_t(size)))}
	}
	defer func() {
		for _, w := range workers {
			libfuse_release(path, w.fi)
			C.free(unsafe.Pointer(w.fi))
			C.free(unsafe.Pointer(w.buf))
		}
	}()

	var mode C.int
	if write {
		mode = 1
	}

	b.ResetTimer()
	var wg sync.WaitGroup
	for i, w := range workers {
		n := b.N / threads
		if i == 0 {
			n += b.N % threads
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			if C.bench_io_loop(mode, w.fi, w.buf, C.size_t(size), C.long(n)) != 0 {
				b.Error("IO failed")
			}
		}()
	}
	wg.Wait()
}
