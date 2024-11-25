package xload

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal"
)

// verify that the below types implement the xcomponent interfaces
var _ xcomponent = &splitter{}
var _ xcomponent = &uploadSplitter{}
var _ xcomponent = &downloadSplitter{}

const SPLITTER string = "splitter"

type splitter struct {
	xbase
	blockSize uint64
	blockPool *BlockPool
	path      string
}

// --------------------------------------------------------------------------------------------------------

type uploadSplitter struct {
	splitter
}

func newUploadSpiltter(blockSize uint64, blockPool *BlockPool, path string, remote internal.Component) (*uploadSplitter, error) {
	u := &uploadSplitter{
		splitter: splitter{
			blockSize: blockSize,
			blockPool: blockPool,
			path:      path,
			xbase: xbase{
				remote: remote,
			},
		},
	}

	u.setName(SPLITTER)
	u.init()
	return u, nil
}

func (u *uploadSplitter) init() {
	u.pool = newThreadPool(MAX_DATA_SPLITTER, u.process)
	if u.pool == nil {
		log.Err("uploadSplitter::init : fail to init thread pool")
	}
}

func (u *uploadSplitter) start() {
	u.getThreadPool().Start()
}

func (u *uploadSplitter) stop() {
	if u.getThreadPool() != nil {
		u.getThreadPool().Stop()
	}
	u.getNext().stop()
}

// split data in chunks which is sent for staging to the data manager
// and then commit the staged data
func (u *uploadSplitter) process(item *workItem) (int, error) {
	var err error
	var ids []string

	log.Trace("uploadSplitter::process : Splitting data for %s", item.path)
	if item.path != "" {
		return 0, nil
	}

	if item.dataLen == 0 {
		// TODO:: xload : If file is of size 0 then we just need to create the file
		return 0, nil
	}

	numBlocks := ((item.dataLen - 1) / u.blockSize) + 1
	offset := int64(0)

	item.fileHandle, err = os.OpenFile(filepath.Join(u.path, item.path), os.O_RDONLY, 0644)
	if err != nil {
		return -1, fmt.Errorf("failed to open file %s [%v]", item.path, err)
	}

	wg := sync.WaitGroup{}
	wg.Add(1)

	responseChannel := make(chan *workItem, numBlocks)

	operationSuccess := true
	go func() {
		defer wg.Done()

		for i := 0; i < int(numBlocks); i++ {
			respSplitItem := <-responseChannel
			if respSplitItem.err != nil {
				log.Err("uploadSplitter::process : Failed to read data from file %s", item.path)
				operationSuccess = false
			}
			if respSplitItem.block != nil {
				log.Trace("uploadSplitter::process : [%d] Upload successful for %s block[%d] %s offset %v", i, item.path, respSplitItem.block.index, respSplitItem.block.id, respSplitItem.block.offset)
				u.blockPool.Release(respSplitItem.block)
			}
		}
	}()

	for i := 0; i < int(numBlocks); i++ {
		block := u.blockPool.Get()
		if block == nil {
			responseChannel <- &workItem{err: fmt.Errorf("failed to get block from pool for file %s %v", item.path, offset)}
		} else {
			dataLen, err := item.fileHandle.ReadAt(block.data, offset)
			if err != nil && err != io.EOF {
				responseChannel <- &workItem{err: fmt.Errorf("failed to read block from file %s %v", item.path, offset)}
			} else {
				block.index = i
				block.offset = offset
				block.length = int64(dataLen)
				block.id = common.GetBlockID(16)

				splitItem := &workItem{
					compName:        u.getNext().getName(),
					path:            item.path,
					dataLen:         item.dataLen,
					fileHandle:      nil,
					block:           block,
					responseChannel: responseChannel,
					download:        false,
				}
				ids = append(ids, splitItem.block.id)
				log.Trace("uploadSplitter::process : Scheduling %s block [%d] %s offset %v length %v", item.path, splitItem.block.index, splitItem.block.id, offset, splitItem.block.length)
				u.getNext().getThreadPool().Schedule(splitItem)
			}
		}

		offset += int64(u.blockSize)
	}

	wg.Wait()
	item.fileHandle.Close()

	if !operationSuccess {
		log.Err("uploadSplitter::process : Failed to upload data from file %s", item.path)
		return -1, fmt.Errorf("failed to upload data from file %s", item.path)
	}

	err = u.getRemote().CommitData(internal.CommitDataOptions{
		Name: item.path,
		List: ids,
	})
	if err != nil {
		log.Err("uploadSplitter::process : failed to commit data [%s]", err.Error())
	}

	return 0, err
}

// --------------------------------------------------------------------------------------------------------

type downloadSplitter struct {
	splitter
}

func newDownloadSplitter(blockSize uint64, blockPool *BlockPool, path string, remote internal.Component) (*downloadSplitter, error) {
	d := &downloadSplitter{
		splitter: splitter{
			blockSize: blockSize,
			blockPool: blockPool,
			path:      path,
			xbase: xbase{
				remote: remote,
			},
		},
	}

	d.setName(SPLITTER)
	d.init()
	return d, nil
}

func (d *downloadSplitter) init() {
	d.pool = newThreadPool(MAX_DATA_SPLITTER, d.process)
	if d.pool == nil {
		log.Err("downloadSplitter::init : fail to init thread pool")
	}
}

func (d *downloadSplitter) start() {
	d.getThreadPool().Start()
}

func (d *downloadSplitter) stop() {
	if d.getThreadPool() != nil {
		d.getThreadPool().Stop()
	}
	d.getNext().stop()
}

// download data in chunks and then write to the local file
func (d *downloadSplitter) process(item *workItem) (int, error) {
	var err error

	log.Debug("downloadSplitter::process : Splitting data for %s", item.path)
	if len(item.path) == 0 {
		return 0, nil
	}

	numBlocks := ((item.dataLen - 1) / d.blockSize) + 1
	offset := int64(0)

	// TODO:: xload : should we delete the file if it already exists
	item.fileHandle, err = os.OpenFile(filepath.Join(d.path, item.path), os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		// create file
		return -1, fmt.Errorf("failed to open file %s [%v]", item.path, err)
	}

	defer item.fileHandle.Close()

	wg := sync.WaitGroup{}
	wg.Add(1)

	responseChannel := make(chan *workItem, numBlocks)

	operationSuccess := true
	go func() {
		defer wg.Done()

		for i := 0; i < int(numBlocks); i++ {
			respSplitItem := <-responseChannel
			if respSplitItem.err != nil {
				log.Err("downloadSplitter::process : Failed to read data from file %s", item.path)
				operationSuccess = false
			}

			_, err := item.fileHandle.WriteAt(respSplitItem.block.data, respSplitItem.block.offset)
			if err != nil {
				log.Err("downloadSplitter::process : Failed to write data to file %s", item.path)
				operationSuccess = false
			}

			if respSplitItem.block != nil {
				log.Trace("downloadSplitter::process : Download successful %s index %d offset %v", item.path, respSplitItem.block.index, respSplitItem.block.offset)
				d.blockPool.Release(respSplitItem.block)
			}
		}
	}()

	for i := 0; i < int(numBlocks); i++ {
		block := d.blockPool.Get()
		if block == nil {
			responseChannel <- &workItem{err: fmt.Errorf("failed to get block from pool for file %s %v", item.path, offset)}
		} else {
			block.index = i
			block.offset = offset
			block.length = int64(d.blockSize)

			splitItem := &workItem{
				compName:        d.getNext().getName(),
				path:            item.path,
				dataLen:         item.dataLen,
				fileHandle:      item.fileHandle,
				block:           block,
				responseChannel: responseChannel,
				download:        true,
			}
			log.Trace("downloadSplitter::process : Scheduling %s offset %v", item.path, offset)
			d.getNext().getThreadPool().Schedule(splitItem)
		}

		offset += int64(d.blockSize)
	}

	wg.Wait()
	err = item.fileHandle.Truncate(int64(item.dataLen))
	if err != nil {
		log.Err("downloadSplitter::process : Failed to truncate file %s [%s]", item.path, err.Error())
		return -1, err
	}

	if !operationSuccess {
		log.Err("downloadSplitter::process : Failed to download data for file %s", item.path)
		return -1, fmt.Errorf("failed to download data for file %s", item.path)
	}

	return 0, nil
}
