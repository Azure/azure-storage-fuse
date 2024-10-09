package xload

import (
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal"
)

// verify that the below types implement the xcomponent interfaces
var _ xcomponent = &xmanager{}
var _ xcomponent = &xLocalDataManager{}
var _ xcomponent = &xRemoteDataManager{}

type xmanager struct {
	xbase
}

// --------------------------------------------------------------------------------------------------------

type xLocalDataManager struct {
	xmanager
}

func newLocalDataManager() (*xLocalDataManager, error) {
	ldm := &xLocalDataManager{}

	ldm.init()
	return ldm, nil
}

func (ldm *xLocalDataManager) init() {
	ldm.pool = newThreadPool(MAX_WORKER_COUNT, ldm.process)
	if ldm.pool == nil {
		log.Err("xLocalDataManager::init : fail to init thread pool")
	}
}

func (ldm *xLocalDataManager) start() {
	ldm.getThreadPool().Start()
}

func (ldm *xLocalDataManager) stop() {
	if ldm.getThreadPool() != nil {
		ldm.getThreadPool().Stop()
	}
	ldm.getNext().stop()
}

// ReadData reads data from the data manager
func (ldm *xLocalDataManager) ReadData(item *workItem) (int, error) {
	return item.fileHandle.ReadAt(item.block.data, item.block.offset)
}

// WriteData writes data to the data manager
func (ldm *xLocalDataManager) WriteData(item *workItem) (int, error) {
	return item.fileHandle.WriteAt(item.block.data, item.block.offset)
}

// --------------------------------------------------------------------------------------------------------

type xRemoteDataManager struct {
	xmanager
}

func newRemoteDataManager(remote internal.Component) (*xRemoteDataManager, error) {
	rdm := &xRemoteDataManager{
		xmanager: xmanager{
			xbase: xbase{
				remote: remote,
			},
		},
	}

	rdm.init()
	return rdm, nil
}

func (rdm *xRemoteDataManager) init() {
	rdm.pool = newThreadPool(MAX_WORKER_COUNT, rdm.process)
	if rdm.pool == nil {
		log.Err("xRemoteDataManager::init : fail to init thread pool")
	}
}

func (rdm *xRemoteDataManager) start() {
	rdm.getThreadPool().Start()
}

func (rdm *xRemoteDataManager) stop() {
	if rdm.getThreadPool() != nil {
		rdm.getThreadPool().Stop()
	}
	rdm.getNext().stop()
}

// ReadData reads data from the data manager
func (rdm *xRemoteDataManager) ReadData(item *workItem) (int, error) {
	return rdm.getRemote().ReadInBuffer(internal.ReadInBufferOptions{
		Handle: nil,
		Name:   item.path,
		Offset: item.block.offset,
		Data:   item.block.data,
	})
}

// WriteData writes data to the data manager
func (rdm *xRemoteDataManager) WriteData(item *workItem) (int, error) {
	return int(item.block.length), rdm.getRemote().StageData(internal.StageDataOptions{
		Name: item.path,
		Data: item.block.data[0:item.block.length],
		// Offset: uint64(item.block.offset),
		Id: item.block.id})
}

// CommitData commits data to the data manager
func (rdm *xRemoteDataManager) commitData(name string, ids []string) error {
	return rdm.remote.CommitData(internal.CommitDataOptions{
		Name: name,
		List: ids,
	})
}
