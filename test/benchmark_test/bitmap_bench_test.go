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

// run this benchmark with: go test bitmap_bench_test.go -bench=. -benchmem

package benchmark_test

import (
	"sync/atomic"
	"testing"
)

// --- Your original implementations (paste or import from your package) ---

type BitMap64 uint64

// IsSet : Check whether the given bit is set or not
func (bm *BitMap64) IsSet(bit uint64) bool {
	return (atomic.LoadUint64((*uint64)(bm)) & (1 << bit)) != 0
}

// Set : Set the given bit in bitmap
// Return true if the bit was not set and was set by this call, false if the bit was already set.
func (bm *BitMap64) Set(bit uint64) bool {
	for {
		loaded := atomic.LoadUint64((*uint64)(bm))
		if (loaded & (1 << bit)) != 0 {
			// Bit already set.
			return false
		}
		newValue := loaded | (1 << bit)
		if atomic.CompareAndSwapUint64((*uint64)(bm), loaded, newValue) {
			// Bit was set successfully.
			return true
		}
	}
}

// Clear : Clear the given bit from bitmap
// Return true if the bit is set and cleared by this call, false if the bit was already cleared.
func (bm *BitMap64) Clear(bit uint64) bool {
	for {
		loaded := atomic.LoadUint64((*uint64)(bm))
		if (loaded & (1 << bit)) == 0 {
			// Bit already cleared.
			return false
		}
		newValue := loaded &^ (1 << bit)
		if atomic.CompareAndSwapUint64((*uint64)(bm), loaded, newValue) {
			// Bit was cleared successfully.
			return true
		}
	}
}

// Reset : Reset the whole bitmap by setting it to 0
// Return true if the bitmap is cleared by this call, false if it was already cleared.
func (bm *BitMap64) Reset() bool {
	for {
		loaded := atomic.LoadUint64((*uint64)(bm))
		if loaded == 0 {
			// Bitmap already cleared.
			return false
		}
		if atomic.CompareAndSwapUint64((*uint64)(bm), loaded, 0) {
			// Bitmap was cleared successfully.
			return true
		}
	}
}

type BitMap16 uint16

// IsSet : Check whether the given bit is set or not
func (bm BitMap16) IsSet(bit uint16) bool { return (bm & (1 << bit)) != 0 }

// Set : Set the given bit in bitmap
func (bm *BitMap16) Set(bit uint16) { *bm |= (1 << bit) }

// Clear : Clear the given bit from bitmap
func (bm *BitMap16) Clear(bit uint16) { *bm &= ^(1 << bit) }

// Reset : Reset the whole bitmap by setting it to 0
func (bm *BitMap16) Reset() { *bm = 0 }

// --- Benchmarks ---
//
// Run with: go test -bench=. -benchmem

// Single-threaded benchmarks for Set

func BenchmarkBitMap64_Set(b *testing.B) {
	var bm BitMap64

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Restrict bit index to 0..63
		bit := uint64(i & 63)
		bm.Set(bit)
	}
}

func BenchmarkBitMap16_Set(b *testing.B) {
	var bm BitMap16

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Restrict bit index to 0..15
		bit := uint16(i & 15)
		bm.Set(bit)
	}
}

// Single-threaded benchmarks for IsSet

func BenchmarkBitMap64_IsSet(b *testing.B) {
	var bm BitMap64
	// Pre-set some bits
	for i := 0; i < 64; i++ {
		bm.Set(uint64(i))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bit := uint64(i & 63)
		_ = bm.IsSet(bit)
	}
}

func BenchmarkBitMap16_IsSet(b *testing.B) {
	var bm BitMap16
	// Pre-set some bits
	for i := 0; i < 16; i++ {
		bm.Set(uint16(i))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bit := uint16(i & 15)
		_ = bm.IsSet(bit)
	}
}

// Single-threaded benchmarks for Clear

func BenchmarkBitMap64_Clear(b *testing.B) {
	var bm BitMap64
	// Pre-set all bits
	for i := 0; i < 64; i++ {
		bm.Set(uint64(i))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bit := uint64(i & 63)
		bm.Clear(bit)
	}
}

func BenchmarkBitMap16_Clear(b *testing.B) {
	var bm BitMap16
	// Pre-set all bits
	for i := 0; i < 16; i++ {
		bm.Set(uint16(i))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bit := uint16(i & 15)
		bm.Clear(bit)
	}
}

// Single-threaded benchmarks for Reset

func BenchmarkBitMap64_Reset(b *testing.B) {
	var bm BitMap64
	// Pre-set all bits once
	for i := 0; i < 64; i++ {
		bm.Set(uint64(i))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bm.Reset()
	}
}

func BenchmarkBitMap16_Reset(b *testing.B) {
	var bm BitMap16
	// Pre-set all bits once
	for i := 0; i < 16; i++ {
		bm.Set(uint16(i))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bm.Reset()
	}
}

// Parallel benchmarks to highlight atomic contention vs non-atomic

func BenchmarkBitMap64_Set_Parallel(b *testing.B) {
	var bm BitMap64
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			bit := uint64(i & 63)
			bm.Set(bit)
			i++
		}
	})
}

func BenchmarkBitMap16_Set_Parallel(b *testing.B) {
	var bm BitMap16
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			bit := uint16(i & 15)
			bm.Set(bit)
			i++
		}
	})
}
