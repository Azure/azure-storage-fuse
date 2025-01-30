package block_cache_new

import (
	"context"
	"errors"
	"fmt"
	"time"
)

func scheduleDownload(blk *block, r requestType) {
	if !blk.buf.valid {
		blk.downloadDone = make(chan error, 1)
		ctx, cancel := context.WithCancel(context.Background())
		blk.downloadCtx = ctx
		taskDone := make(chan struct{}, 1)
		blk.cancelOngolingAsyncDownload = func() {
			cancel()
			<-taskDone
		}
		wp.createTask(ctx, taskDone, false, r, blk)
	}
}

func asyncDownloadScheduler(blk *block) {
	blk.Lock()
	defer blk.Unlock()
	if blk.buf == nil {
		bPool.getBufferForBlock(blk)
	}
	if blk.state == committedBlock {
		select {
		case err, ok := <-blk.downloadDone: // Check if sync upload is in progress
			if !ok {
				logy.Write([]byte(fmt.Sprintf("Async Downloader: Scheduling blk idx: %d, filePath: %s\n", blk.idx, blk.file.Name)))
				scheduleDownload(blk, asyncRequest)
			} else {
				if err == nil {
					blk.buf.valid = true
					close(blk.downloadDone)
				} else {
					// todo: Download has failed, error handling
				}
			}
		case <-time.NewTimer(5 * time.Millisecond).C:
		}
	}
}

func syncDownloader(idx int, blk *block) (state blockState, err error) {
	blk.Lock()
	defer blk.Unlock()
	if blk.buf == nil {
		bPool.getBufferForBlock(blk)
	}

	if blk.state == committedBlock {
		// Check if async Download is in progress.
		select {
		case err, ok := <-blk.downloadDone:
			if ok && err == nil && !errors.Is(blk.downloadCtx.Err(), context.Canceled) {
				// Download was already completed by async scheduler.
				blk.buf.valid = true
				close(blk.downloadDone)
			} else if !blk.buf.valid {
				scheduleDownload(blk, syncRequest)
				err = <-blk.downloadDone
				if err == nil {
					blk.buf.valid = true
					close(blk.downloadDone)
				}
			}
		case <-time.NewTimer(20 * time.Millisecond).C:
			// Taking toomuch time for async upload to complete, cancel the upload and schedule a new one.
			blk.cancelOngolingAsyncDownload()
			scheduleDownload(blk, syncRequest)
			err = <-blk.downloadDone
			if err == nil {
				blk.buf.valid = true
				close(blk.downloadDone)
			}
		}
	}
	state = blk.state
	return
}
