package block_cache_new

import (
	"context"
	"errors"
	"time"
)

func scheduleUpload(blk *block, r requestType) {
	blk.uploadDone = make(chan error, 1)
	ctx, cancel := context.WithCancel(context.Background())
	blk.uploadCtx = ctx
	taskDone := make(chan struct{}, 1)
	blk.cancelOngoingAsyncUpload = func() {
		cancel()
		<-taskDone
	}
	wp.createTask(ctx, taskDone, true, r, blk)
}

// Schedules the upload and return true if the block is local/uncommited.
func syncUploader(blk *block) bool {
	blk.Lock()
	defer blk.Unlock()
	if blk.state == localBlock {
		if blk.hole {
			// This is a sparse block.
			err := punchHole(blk.file)
			if err == nil {
				blk.state = uncommitedBlock
			}
		} else {
			if blk.buf == nil {
				panic("Local Block must always have some buffer")
			}
			// Check if async upload is in progress.
			select {
			case err, ok := <-blk.uploadDone:
				if ok && err == nil && !errors.Is(blk.uploadCtx.Err(), context.Canceled) {
					// Upload was already completed by async scheduler and no more write came after it.
					blk.state = uncommitedBlock
					close(blk.uploadDone)
				} else {
					scheduleUpload(blk, syncRequest)
				}
			case <-time.NewTimer(20 * time.Millisecond).C:
				// Taking toomuch time for async upload to complete, cancel the upload and schedule a new one.
				blk.cancelOngoingAsyncUpload()
				scheduleUpload(blk, syncRequest)
			}
			//err = syncBuffer(file.Name, file.size, blk)
		}
		return true
	} else if blk.state == uncommitedBlock {
		return true
	}
	return false
}
