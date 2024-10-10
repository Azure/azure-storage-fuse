package xload

import (
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal"
)

// verify that the below types implement the xcomponent interfaces
var _ xcomponent = &dataManager{}
var _ xcomponent = &localDataManager{}
var _ xcomponent = &remoteDataManager{}

type dataManager struct {
	xbase
}

// --------------------------------------------------------------------------------------------------------

type localDataManager struct {
	dataManager
}

func newLocalDataManager() (*localDataManager, error) {
	ldm := &localDataManager{}

	ldm.init()
	return ldm, nil
}

func (ldm *localDataManager) init() {
	ldm.pool = newThreadPool(MAX_WORKER_COUNT, ldm.process)
	if ldm.pool == nil {
		log.Err("localDataManager::init : fail to init thread pool")
	}
}

func (ldm *localDataManager) start() {
	ldm.getThreadPool().Start()
}

func (ldm *localDataManager) stop() {
	if ldm.getThreadPool() != nil {
		ldm.getThreadPool().Stop()
	}
	ldm.getNext().stop()
}

// ReadData reads data from the data manager
func (ldm *localDataManager) ReadData(item *workItem) (int, error) {
	return item.fileHandle.ReadAt(item.block.data, item.block.offset)
}

// WriteData writes data to the data manager
func (ldm *localDataManager) WriteData(item *workItem) (int, error) {
	return item.fileHandle.WriteAt(item.block.data, item.block.offset)
}

// --------------------------------------------------------------------------------------------------------

type remoteDataManager struct {
	dataManager
}

func newRemoteDataManager(remote internal.Component) (*remoteDataManager, error) {
	rdm := &remoteDataManager{
		dataManager: dataManager{
			xbase: xbase{
				remote: remote,
			},
		},
	}

	rdm.init()
	return rdm, nil
}

func (rdm *remoteDataManager) init() {
	rdm.pool = newThreadPool(MAX_WORKER_COUNT, rdm.process)
	if rdm.pool == nil {
		log.Err("remoteDataManager::init : fail to init thread pool")
	}
}

func (rdm *remoteDataManager) start() {
	rdm.getThreadPool().Start()
}

func (rdm *remoteDataManager) stop() {
	if rdm.getThreadPool() != nil {
		rdm.getThreadPool().Stop()
	}
	rdm.getNext().stop()
}

// ReadData reads data from the data manager
func (rdm *remoteDataManager) ReadData(item *workItem) (int, error) {
	return rdm.getRemote().ReadInBuffer(internal.ReadInBufferOptions{
		Handle: nil,
		Name:   item.path,
		Offset: item.block.offset,
		Data:   item.block.data,
	})
}

// WriteData writes data to the data manager
func (rdm *remoteDataManager) WriteData(item *workItem) (int, error) {
	return int(item.block.length), rdm.getRemote().StageData(internal.StageDataOptions{
		Name: item.path,
		Data: item.block.data[0:item.block.length],
		// Offset: uint64(item.block.offset),
		Id: item.block.id})
}

// CommitData commits data to the data manager
func (rdm *remoteDataManager) commitData(name string, ids []string) error {
	return rdm.remote.CommitData(internal.CommitDataOptions{
		Name: name,
		List: ids,
	})
}
