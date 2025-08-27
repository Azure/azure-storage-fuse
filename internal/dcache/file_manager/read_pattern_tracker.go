/*
    _____           _____   _____   ____          ______  _____  ------
   |     |  |      |     | |     | |     |     | |       |            |
   |     |  |      |     | |     | |     |     | |       |            |
   | --- |  |      |     | |-----| |---- |     | |-----| |-----  ------
   |     |  |      |     | |     | |     |     |       | |       |
   | ____|  |_____ | ____| | ____| |     |_____|  _____| |_____  |_____


   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2020-2025 Microsoft Corporation. All rights reserved.
   Author : <blobFUSEdev@microsoft.com>

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

package filemanager

import (
	"math"
	"sync/atomic"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
)

// Primary goal of our read pattern tracker is to correctly identify parallel FUSE read requests as a result
// of a large application read as sequential. e.g., our target applications may issue large reads, say 10MiB,
// which FUSE will break down into multiple parallel reads, say 1MiB each. These 10x1MiB reads will be processed
// by multiple libfuse threads in non-deterministic order. The application will issue the next 10MiB read
// immediately adjacent to the previous 10MiB read, which is purely sequential reads. We should not consider
// the reordered 1MiB reads as random reads.
//
// We maintain a windowSize that determines how many such parallel reads can be active at any time. If Nth read
// is withing 2*windowSize of previous read, we consider it sequential. This is because the 1st read of prev batch
// and the last read of current batch can be processed one after another and will be 2*windowSize apart.
type RPTracker struct {
	windowSize     int64
	prevReadOffset atomic.Int64
	//
	// randomStreak is incremented when we see a random read, and decremented when we see a sequential read.
	// High positive value means "definitely random", high negative value means "definitely sequential", while
	// values close to zero means "maybe sequential".
	//
	randomStreak atomic.Int64
	fileName     string // For logging purposes only.
}

func NewRPTracker(file string) *RPTracker {
	//
	// This should not be less than max fuse threads (max_threads) as that decides how many parallel reads can be
	// running, and all reads within this window must be considered sequential. Note that we assume that FUSE
	// kernel module will not send reads more than 1MiB to us. If that ever changes this needs to change accordingly.
	//
	windowSizeInMiB := int64(10)
	rpt := &RPTracker{
		windowSize: windowSizeInMiB * common.MbToBytes,
		fileName:   file,
	}

	//
	// Set it to -3 so that if the first read is done at the start of the file, we immediately confirm
	// sequential access pattern, while if it doesn't start at the beginning, we go to "unsure" state.
	//
	rpt.randomStreak.Store(-3)
	return rpt
}

// It updates the read pattern tracker with a new read at offset of length bytes, and returns the
// current access pattern: 1 for sequential, -1 for random, 0 for not sure.
// If you want to know the current access pattern without updating, use Check() instead.
func (t *RPTracker) Update(offset, length int64) int {
	common.Assert(offset >= 0 && length > 0, offset, length)

	prevReadOffset := t.prevReadOffset.Swap(offset + length)
	accessPattern := 0

	absDiff := int64(math.Abs(float64(offset - prevReadOffset)))
	if absDiff > 2*t.windowSize {
		// Read outside the windows, hints at random access.
		if t.randomStreak.Add(1) < -3 {
			log.Warn("RPTracker::Update: File %s (%d) [SEQUENTIAL -> RANDOM], %d -> %d, Streak: %d",
				t.fileName, length, prevReadOffset, offset, t.randomStreak.Load())
			// Reset the streak, it still has to prove randomness with 3 more reads.
			t.randomStreak.Store(0)
		} else {
			// Confirmed random access.
			accessPattern = -1
		}
	} else {
		// Read within the window, hints at sequential access.
		if t.randomStreak.Add(-1) > 3 {
			log.Warn("RPTracker::Update: File %s (%d) [RANDOM -> SEQUENTIAL], %d -> %d, Streak: %d",
				t.fileName, length, prevReadOffset, offset, t.randomStreak.Load())
			// Reset the streak, it still has to prove sequentialness with 3 more reads.
			t.randomStreak.Store(0)
		} else {
			// Confirmed sequential access.
			accessPattern = 1
		}
	}

	return accessPattern
}

// Return 1 for definitely sequential, -1 for definitely random, 0 for not sure.
func (t *RPTracker) Check() int {
	if t.randomStreak.Load() < -3 {
		// Definitely sequential.
		return 1
	} else if t.randomStreak.Load() > 3 {
		// Definitely random.
		return -1
	} else {
		// Not sure.
		return 0
	}
}
