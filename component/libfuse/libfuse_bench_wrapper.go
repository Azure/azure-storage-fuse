//go:build unittest && !fuse2

// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package libfuse

// Benchmark helpers that call into C. They live in a regular file because cgo cannot be used in _test.go files; the
// unittest build tag keeps them out of the blobfuse2 binary. See libfuse_bench_test.go.

// #cgo CFLAGS: -DFUSE_USE_VERSION=39 -D_FILE_OFFSET_BITS=64
// #cgo LDFLAGS: -lfuse3 -ldl -lpthread
// #include "libfuse_wrapper.h"
// #include <pthread.h>
//
// // bench_io_loop : call the read or write callback n times
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
//
// // bench_worker_t : a thread created in C, like the worker threads of libfuse, that calls the callbacks of one file
// typedef struct {
//     pthread_t          thread;
//     pthread_barrier_t *ready;   // passed when every worker is set up, and the timed interval can start
//     pthread_barrier_t *start;   // passed to start the timed interval
//     pthread_barrier_t *done;    // passed when every worker is done
//     int                write;
//     fuse_file_info_t  *fi;
//     char              *buf;
//     size_t             size;
//     long               n;
//     int                failed;
// } bench_worker_t;
//
// static void *bench_worker_main(void *arg)
// {
//     bench_worker_t *w = (bench_worker_t *)arg;
//     // The first callback on a thread created in C attaches Go to the thread, keep that out of the timed interval
//     w->failed = bench_io_loop(w->write, w->fi, w->buf, w->size, 1);
//     pthread_barrier_wait(w->ready);
//     pthread_barrier_wait(w->start);
//     w->failed |= bench_io_loop(w->write, w->fi, w->buf, w->size, w->n);
//     pthread_barrier_wait(w->done);
//     return NULL;
// }
//
// // bench_workers_t : the workers of one benchmark run, and the barriers that the caller waits on with them
// typedef struct {
//     pthread_barrier_t ready, start, done;
//     bench_worker_t   *workers;
//     int               count;
// } bench_workers_t;
//
// static bench_workers_t *bench_workers_start(int count)
// {
//     bench_workers_t *ws = (bench_workers_t *)calloc(1, sizeof(bench_workers_t));
//     ws->workers = (bench_worker_t *)calloc(count, sizeof(bench_worker_t));
//     ws->count = count;
//     pthread_barrier_init(&ws->ready, NULL, count + 1);
//     pthread_barrier_init(&ws->start, NULL, count + 1);
//     pthread_barrier_init(&ws->done, NULL, count + 1);
//     return ws;
// }
//
// static bench_worker_t *bench_worker(bench_workers_t *ws, int i)
// {
//     return &ws->workers[i];
// }
//
// static int bench_workers_spawn(bench_workers_t *ws)
// {
//     for (int i = 0; i < ws->count; i++) {
//         bench_worker_t *w = &ws->workers[i];
//         w->ready = &ws->ready;
//         w->start = &ws->start;
//         w->done = &ws->done;
//         if (pthread_create(&w->thread, NULL, bench_worker_main, w) != 0)
//             return -1;
//     }
//     return 0;
// }
//
// static void bench_wait(pthread_barrier_t *barrier)
// {
//     pthread_barrier_wait(barrier);
// }
//
// // bench_workers_finish : join the workers and free them, returns whether any callback failed
// static int bench_workers_finish(bench_workers_t *ws)
// {
//     int failed = 0;
//     for (int i = 0; i < ws->count; i++) {
//         pthread_join(ws->workers[i].thread, NULL);
//         failed |= ws->workers[i].failed;
//     }
//     pthread_barrier_destroy(&ws->ready);
//     pthread_barrier_destroy(&ws->start);
//     pthread_barrier_destroy(&ws->done);
//     free(ws->workers);
//     free(ws);
//     return failed;
// }
import "C"

import (
	"os"
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
// times in total from that many threads created in C. ns/op is the wall time divided by b.N, so with more than one
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

	// The callbacks are called from threads created in C, as libfuse does, not from goroutines
	ws := C.bench_workers_start(C.int(threads))
	for i, w := range workers {
		n := b.N / threads
		if i == 0 {
			n += b.N % threads
		}
		cw := C.bench_worker(ws, C.int(i))
		cw.write, cw.fi, cw.buf, cw.size, cw.n = mode, w.fi, w.buf, C.size_t(size), C.long(n)
	}
	if C.bench_workers_spawn(ws) != 0 {
		b.Fatal("failed to create worker threads")
	}

	C.bench_wait(&ws.ready)
	b.ResetTimer()
	C.bench_wait(&ws.start)
	C.bench_wait(&ws.done)
	b.StopTimer()

	if C.bench_workers_finish(ws) != 0 {
		b.Error("IO failed")
	}
}
