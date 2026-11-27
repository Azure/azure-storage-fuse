// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package common

import (
	"sync"
	"testing"
)

func TestLockMapConcurrentHandleCount(t *testing.T) {
	var item LockMapItem
	const workers = 16
	const iterations = 1000
	var group sync.WaitGroup
	for range workers {
		group.Go(func() {
			for range iterations {
				item.Inc()
				_ = item.Count()
			}
		})
	}
	group.Wait()
	if got := item.Count(); got != workers*iterations {
		t.Fatalf("count after increments = %d, want %d", got, workers*iterations)
	}
	for range workers {
		group.Go(func() {
			for range iterations {
				item.Dec()
				_ = item.Count()
			}
		})
	}
	group.Wait()
	if got := item.Count(); got != 0 {
		t.Fatalf("count after decrements = %d, want 0", got)
	}
}
