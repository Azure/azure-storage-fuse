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
	randomStreak   atomic.Int64
	fileName       string // For logging purposes only.
}

func NewRPTracker(file string) *RPTracker {
	//
	// This should not be less than max fuse threads (max_threads) as that decides how many parallel reads can be
	// running, and all reads within this window must be considered sequential. Note that we assume that FUSE
	// kernel module will not send reads more than 1MiB to us. If that ever changes this needs to change accordingly.
	//
	windowSizeInMiB := int64(10)
	return &RPTracker{
		windowSize: windowSizeInMiB * common.MbToBytes,
		fileName:   file,
	}
}

func (t *RPTracker) Update(offset, length int64) {
	common.Assert(offset >= 0 && length > 0, offset, length)

	absDiff := int64(math.Abs(float64(offset - t.prevReadOffset.Load())))
	if absDiff > 2*t.windowSize {
		t.randomStreak.Add(1)
		if t.randomStreak.Load() == 3 {
			log.Debug("RPTracker::Update: File %s [SEQUENTIAL -> RANDOM], offset: %d, length: %d, prevOffset: %d, absDiff: %d",
				t.fileName, offset, length, t.prevReadOffset.Load(), absDiff)
		}
	} else {
		//
		// TODO: This means that if two random reads happen to be within 2*windowSize, we will consider
		//       it sequential. See if we can improve this.
		//
		if t.randomStreak.Load() >= 3 {
			log.Debug("RPTracker::Update: File %s [RANDOM -> SEQUENTIAL], offset: %d, length: %d, prevOffset: %d, absDiff: %d",
				t.fileName, offset, length, t.prevReadOffset.Load(), absDiff)
		}
		t.randomStreak.Store(0)
	}

	t.prevReadOffset.Store(offset + length)
}

func (t *RPTracker) IsSequential() bool {
	//
	// If we have seen 3 or more random reads in a row, we consider the pattern random.
	// So, we start considering read access as sequential and only mark it random when proven otherwise.
	//
	return t.randomStreak.Load() < 3
}
