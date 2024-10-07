package xload

import (
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal"
)

// verify that the below types implement the xcomponent interfaces
var _ xcomponent = &xsplitter{}
var _ xcomponent = &xUploadSplitter{}
var _ xcomponent = &xDownloadSplitter{}

type xsplitter struct {
	xbase
	blockSize uint64
	blockPool *BlockPool
	path      string
}

// --------------------------------------------------------------------------------------------------------

type xUploadSplitter struct {
	xsplitter
	// commiter dataCommitter
}

func newUploadSpiltter(blockSize uint64, blockPool *BlockPool, path string, remote internal.Component) (*xUploadSplitter, error) {
	u := &xUploadSplitter{
		xsplitter: xsplitter{
			blockSize: blockSize,
			blockPool: blockPool,
			path:      path,
			xbase: xbase{
				remote: remote,
			},
		},
	}

	u.init()
	return u, nil
}

func (u *xUploadSplitter) init() {
	u.pool = newThreadPool(MAX_DATA_SPLITTER, u.process)
	if u.pool == nil {
		log.Err("xsplitter::init : fail to init thread pool")
	}
}

func (u *xUploadSplitter) start() {
	u.getThreadPool().Start()
	u.getThreadPool().Schedule(&workItem{})
}

func (u *xUploadSplitter) stop() {
	if u.getThreadPool() != nil {
		u.getThreadPool().Stop()
	}
	u.getNext().stop()
}

// --------------------------------------------------------------------------------------------------------

type xDownloadSplitter struct {
	xsplitter
}

func newDownloadSplitter(path string, remote internal.Component) (*xDownloadSplitter, error) {
	return nil, nil
}
