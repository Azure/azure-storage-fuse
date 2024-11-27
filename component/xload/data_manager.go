package xload

import (
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal"
	"github.com/Azure/azure-storage-fuse/v2/internal/handlemap"
)

// verify that the below types implement the xcomponent interfaces
var _ xcomponent = &dataManager{}
var _ xcomponent = &localDataManager{}
var _ xcomponent = &remoteDataManager{}

const DATA_MANAGER string = "DATA_MANAGER"

type dataManager struct {
	xbase
}

// --------------------------------------------------------------------------------------------------------

type localDataManager struct {
	dataManager
}

func newLocalDataManager() (*localDataManager, error) {
	log.Debug("data_manager::newLocalDataManager : create new local data manager")

	ldm := &localDataManager{}
	ldm.setName(DATA_MANAGER)
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
	log.Debug("localDataManager::start : start local data manager")
	ldm.getThreadPool().Start()
}

func (ldm *localDataManager) stop() {
	log.Debug("localDataManager::stop : stop local data manager")
	if ldm.getThreadPool() != nil {
		ldm.getThreadPool().Stop()
	}
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
	log.Debug("data_manager::newRemoteDataManager : create new remote data manager")

	rdm := &remoteDataManager{
		dataManager: dataManager{
			xbase: xbase{
				remote: remote,
			},
		},
	}

	rdm.setName(DATA_MANAGER)
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
	log.Debug("remoteDataManager::start : start remote data manager")
	rdm.getThreadPool().Start()
}

func (rdm *remoteDataManager) stop() {
	log.Debug("remoteDataManager::stop : stop remote data manager")
	if rdm.getThreadPool() != nil {
		rdm.getThreadPool().Stop()
	}
}

// upload or download block
func (rdm *remoteDataManager) process(item *workItem) (int, error) {
	if item.download {
		return rdm.ReadData(item)
	} else {
		return rdm.WriteData(item)
	}
}

// ReadData reads data from the data manager
func (rdm *remoteDataManager) ReadData(item *workItem) (int, error) {
	log.Debug("remoteDataManager::ReadData : Scheduling download for %s offset %v", item.path, item.block.offset)

	h := handlemap.NewHandle(item.path)
	h.Size = int64(item.dataLen)
	return rdm.getRemote().ReadInBuffer(internal.ReadInBufferOptions{
		Handle: h,
		Offset: item.block.offset,
		Data:   item.block.data,
	})
}

// WriteData writes data to the data manager
func (rdm *remoteDataManager) WriteData(item *workItem) (int, error) {
	log.Debug("remoteDataManager::WriteData : Scheduling upload for %s offset %v", item.path, item.block.offset)

	return int(item.block.length), rdm.getRemote().StageData(internal.StageDataOptions{
		Name: item.path,
		Data: item.block.data[0:item.block.length],
		// Offset: uint64(item.block.offset),
		Id: item.block.id})
}
