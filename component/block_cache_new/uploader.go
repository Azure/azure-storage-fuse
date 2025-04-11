package block_cache_new

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal"
)

func scheduleUpload(blk *block, r requestType) {
	blk.uploadDone = make(chan error, 1)
	blk.forceCancelUpload = make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	blk.uploadCtx = ctx
	taskDone := make(chan struct{}, 1)
	// blk.refCnt++
	blk.cancelOngoingAsyncUpload = func() {
		log.Info("BlockCache::scheduleUpload : Async Upload Cancel for blk idx : %d, file : %s", blk.idx, blk.file.Name)
		// Before Cancelling the upload, wait to see if there is a flush operation is going on for the file.
		// If there is a flush operation, then wait until the flush completes on the file.
		select {
		case <-blk.file.flushOngoing: // This will always be success if there is no flush operation for the file.
		case <-blk.forceCancelUpload: // This is always blocked, until unless closed.
			log.Info("BlockCache::scheduleUpload : async Upload Canceled by flush call blk idx : %d, file : %s", blk.idx, blk.file.Name)
		}
		cancel()
		<-taskDone
	}
	wp.createTask(ctx, taskDone, true, r, blk)
}

// Schedules the upload and return true if the block is local/uncommited.
func uploader(blk *block, r requestType) (state blockState, err error) {
	blk.Lock()
	defer blk.Unlock()
	var ok bool
	if blk.state == localBlock {
		if blk.hole {
			// This is a sparse block.
			log.Info("BlockCache::Uploader : Punching a hole inside the file blk idx : %d, file name : %s", blk.idx, blk.file.Name)
			err = punchHole(blk.file)
			if err == nil {
				blk.state = uncommitedBlock
			}
		} else if blk.buf != nil {
			// Check if async upload is in progress.
			now := time.Now()
		outer:
			for {
				select {
				case err, ok = <-blk.uploadDone:
					if ok {
						close(blk.uploadDone)
					}
					if ok && err == nil && blk.uploadCtx.Err() == nil {
						// Upload was already completed by async scheduler and no more write came after it.
						blk.state = uncommitedBlock
					} else {
						close(blk.forceCancelUpload)
						blk.cancelOngoingAsyncUpload()
						scheduleUpload(blk, r)
					}
					break outer
				default:
					// Taking toomuch time for request to complete,
					// cancel the ongoing upload and schedule a new one.
					if time.Since(now) > 10*time.Second && r.isRequestSync() {
						log.Info("BlockCache::Uploader : Cancelling ongoing async upload and scheduling the new one")
						// Here we should not wait for async upload to hang on to the flush to complete, as this came from the flush op, hence closing the channel would do it.
						if blk.state == localBlock {
							close(blk.forceCancelUpload)
							blk.cancelOngoingAsyncUpload()
							scheduleUpload(blk, r)
						}
						break outer
					} else if r.isRequestScheduled() {
						// The block has already scheduled, just let go
						panic("catasrophy for select statement")
					} else {
						time.Sleep(1 * time.Millisecond)
					}
				}
			}
		} else {
			panic(fmt.Sprintf("BlockCache::uploader : buffer is misssing blk idx : %d, file name :%s", blk.idx, blk.file.Name))
		}
	}

	if r.isRequestSync() {
		err, ok = <-blk.uploadDone
		if ok {
			close(blk.uploadDone)
		}
		if ok && err == nil && blk.uploadCtx.Err() == nil {
			blk.state = uncommitedBlock
		} else {
			if err != nil {
				panic(fmt.Sprintf("BlockCache::uploader : Sync upload failed with err %s, blk idx : %d, file name :%s", err.Error(), blk.idx, blk.file.Name))
			} else if blk.uploadCtx.Err() != nil {
				panic(fmt.Sprintf("BlockCache::uploader : Sync upload failed with err %s, blk idx : %d, file name :%s", blk.uploadCtx.Err().Error(), blk.idx, blk.file.Name))
			}
		}
	}
	state = blk.state
	return
}

// stages empty block for the hole
func punchHole(f *File) error {
	if f.holePunched {
		return nil
	}
	err := syncZeroBuffer(f.Name)
	if err == nil {
		f.holePunched = true
	}

	return err
}

func syncZeroBuffer(name string) error {
	return bc.NextComponent().StageData(
		internal.StageDataOptions{
			Ctx:  context.Background(),
			Name: name,
			Id:   zeroBlockId,
			Data: zeroBuffer.data,
		},
	)

}
