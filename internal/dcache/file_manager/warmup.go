package filemanager

import (
	"sync"

	"github.com/Azure/azure-storage-fuse/v2/common"
	cm "github.com/Azure/azure-storage-fuse/v2/internal/dcache/clustermap"
)

type cacheWarmup struct {
	Size                int64 // file size in bytes to warm up
	MaxChunks           int64
	NxtChunkIdxToRead   int64
	CurReadOffsetHandle chan int64
	Wg                  sync.WaitGroup
	Done                chan struct{}
	Error               error
}

func NewCacheWarmup(size int64) *cacheWarmup {
	numChunks := int64((cm.GetCacheConfig().ChunkSizeMB * common.MbToBytes))
	maxChunks := (size + numChunks - 1) / numChunks
	cw := &cacheWarmup{
		Size:                size,
		MaxChunks:           maxChunks,
		NxtChunkIdxToRead:   0,
		CurReadOffsetHandle: make(chan int64, 1),
		Done:                make(chan struct{}),
	}
	cw.Wg.Add(1)
	return cw
}

func (cw *cacheWarmup) EndCacheWarmup() {
	close(cw.Done)
	cw.Wg.Wait()
}
