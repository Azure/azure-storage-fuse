//go:build fuse2

// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package libfuse

// kernelListCacheTracker is not supported on fuse2. This stub satisfies the
// type reference in libfuse.go (which has no build tag) so that fuse2 builds
// compile. newKernelListCacheTracker is never called under fuse2 because
// libfuse2_handler.go zeroes kernelListCacheTtlInSec at init time.
type kernelListCacheTracker struct{}

func newKernelListCacheTracker(_ uint32) *kernelListCacheTracker { return &kernelListCacheTracker{} }
func (t *kernelListCacheTracker) start()                         {}
func (t *kernelListCacheTracker) stop()                          {}
