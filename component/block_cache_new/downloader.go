package block_cache_new

import (
	"context"
	"time"
)

func scheduleDownload(blk *block, r requestType) {
	if !blk.buf.valid {
		blk.downloadDone = make(chan error, 1)
		ctx, cancel := context.WithCancel(context.Background())
		taskDone := make(chan struct{}, 1)
		blk.cancelOngolingAsyncDownload = func() {
			cancel()
			<-taskDone
		}
		wp.createTask(ctx, taskDone, false, r, blk)
	}
}

func downloader(blk *block, r requestType) (state blockState, err error) {
	blk.Lock()
	defer blk.Unlock()
	if blk.buf == nil {
		bPool.getBufferForBlock(blk)
	}

	if blk.state == committedBlock {
		// Check if async Download is in progress.
		select {
		case err, ok := <-blk.downloadDone:
			if ok && err == nil {
				// Download was already completed.
				blk.buf.valid = true
				close(blk.downloadDone)
			} else if !blk.buf.valid {
				scheduleDownload(blk, r)
			}
		case <-time.NewTimer(1000 * time.Millisecond).C:
			// Taking toomuch time for completing the request, cancel and reschedule.
			blk.cancelOngolingAsyncDownload()
			scheduleDownload(blk, r)
		}
	}
	if r == syncRequest {
		err, ok := <-blk.downloadDone
		if ok && err == nil {
			blk.buf.valid = true
			close(blk.downloadDone)
		}
	}
	state = blk.state
	return
}
