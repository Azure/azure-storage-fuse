package block_cache_new

import (
	"context"
	"fmt"
	"time"
)

func scheduleUpload(blk *block, r requestType) {
	blk.uploadDone = make(chan error, 1)
	ctx, cancel := context.WithCancel(context.Background())
	taskDone := make(chan struct{}, 1)
	blk.cancelOngoingAsyncUpload = func() {
		cancel()
		<-taskDone
	}
	wp.createTask(ctx, taskDone, true, r, blk)
}

// Schedules the upload and return true if the block is local/uncommited.
func uploader(blk *block, r requestType) (state blockState, err error) {
	blk.Lock()
	defer blk.Unlock()
	if blk.state == localBlock {
		if blk.hole {
			// This is a sparse block.
			err = punchHole(blk.file)
			if err == nil {
				blk.state = uncommitedBlock
			}
		} else if blk.buf != nil {
			// Check if async upload is in progress.
			select {
			case err, ok := <-blk.uploadDone:
				if ok && err == nil {
					// Upload was already completed by async scheduler and no more write came after it.
					blk.state = uncommitedBlock
					close(blk.uploadDone)
				} else {
					logy.Write([]byte(fmt.Sprintf("BlockCache::uploader :[sync: %d], path=%s, blk Idx = %d\n", r, blk.file.Name, blk.idx)))
					scheduleUpload(blk, r)
				}
			case <-time.NewTimer(1000 * time.Millisecond).C:
				// Taking toomuch time for request to complete,
				// cancel the ongoing upload and schedule a new one.
				if r == syncRequest {
					logy.Write([]byte(fmt.Sprintf("BlockCache::uploader Debug :[sync: %d], path=%s, blk Idx = %d\n", r, blk.file.Name, blk.idx)))
					blk.cancelOngoingAsyncUpload()
					scheduleUpload(blk, r)
				}
			}
		}
	}
	if r == syncRequest {
		err, ok := <-blk.uploadDone
		if ok && err == nil {
			blk.state = uncommitedBlock
			close(blk.uploadDone)
		}
	}
	state = blk.state
	return
}
