package block_cache_new

import (
	"context"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common/log"
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
		// Increment the ref cnt for blk as download is in progress.
		// Its gets automatically once the download completes/fails
		blk.refCnt.Add(1)
		wp.createTask(ctx, taskDone, false, r, blk)
	}
}

// reponsible for shceduling the download and wait for the download to complete if request is of type sync.
// If block is already present then returns it.
func downloader(blk *block, r requestType) (state blockState, err error) {
	blk.Lock()
	defer blk.Unlock()
	if blk.buf == nil {
		bPool.getBufferForBlock(blk)
	}

	if blk.state == committedBlock {
		// Check if async Download is in progress.
		now := time.Now()
	outer:
		for {
			select {
			case err, ok := <-blk.downloadDone:
				if ok && err == nil {
					// Download was already completed.
					blk.buf.valid = true
					close(blk.downloadDone)
				} else if !blk.buf.valid {
					scheduleDownload(blk, r)
				}
				break outer
			default:
				// Taking toomuch time for completing the request, cancel and reschedule.
				if time.Since(now) > 1000*time.Millisecond {
					log.Info("BlockCache::downloader : Cancelling ongoing async upload and scheduling the new one")
					blk.cancelOngolingAsyncDownload()
					scheduleDownload(blk, r)
					break outer
				}
			}
		}
	}

	if r == syncRequest {
		bPool.updateRecentnessOfBlk <- blk
		err, ok := <-blk.downloadDone
		if ok && err == nil {
			blk.buf.valid = true
			close(blk.downloadDone)
		}
	}

	return
}
