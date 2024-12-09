package xload

import (
	"sync"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common/log"
)

type statsManager struct {
	totalFiles        uint64          // total number of files that have been scanned so far
	success           uint64          // number of files that have been successfully processed
	failed            uint64          // number of files that failed
	bytesDownloaded   uint64          // total number of bytes downloaded
	bytesUploaded     uint64          // total number of bytes uploaded
	lastProcessedTime time.Time       // variable indicating the time at which the stats was last dumped
	wg                sync.WaitGroup  // wait group to wait for stats manager thread to finish
	items             chan *statsItem // channel to hold the stats items
	// TODO:: xload :
	// bandwidth utilization
	// bytes downloaded
	// dump to json file
}

type statsItem struct {
	listerCount      uint64 // number of files scanned by the lister in an iteration
	name             string // name of the file processed
	success          bool   // flag to indicate if the file has been processed successfully or not
	download         bool   // flag to denote upload or download
	bytesTransferred uint64 // bytes uploaded or downloaded for this file
}

const DELTA = 2 // time interval in seconds at which the stats will be dumped

func newStatsmanager(count uint32) *statsManager {
	return &statsManager{
		items: make(chan *statsItem, count*2),
	}
}

func (sm *statsManager) start() {
	log.Debug("statsManager::start : start stats manager")
	sm.wg.Add(1)
	sm.lastProcessedTime = time.Now().UTC()
	go sm.statsProcessor()
}

func (sm *statsManager) stop() {
	log.Debug("statsManager::stop : stop stats manager")
	close(sm.items)
	sm.wg.Wait()
}

func (sm *statsManager) addStats(item *statsItem) {
	sm.items <- item
}

func (sm *statsManager) statsProcessor() {
	defer sm.wg.Done()

	for item := range sm.items {
		if item.listerCount > 0 {
			// stats sent by the lister component
			log.Debug("statsManager::statsProcessor : Directory listed %v, count %v", item.name, item.listerCount)
			sm.totalFiles += item.listerCount
			log.Debug("statsManager::statsProcessor : Total number of files listed so far = %v", sm.totalFiles)
		} else {
			// stats sent by the splitter component
			log.Debug("statsManager::statsProcessor : Name %v, success %v, download %v, bytes transferred %v", item.name, item.success, item.download, item.bytesTransferred)

			if item.success {
				sm.success += 1
			} else {
				sm.failed += 1
			}

			if item.download {
				sm.bytesDownloaded += item.bytesTransferred
			} else {
				sm.bytesUploaded += item.bytesTransferred
			}

			sm.calculateBandwidth()
		}
	}

	log.Debug("statsManager::statsProcessor : stats processor completed")
}

func (st *statsManager) calculateBandwidth() {
	currTime := time.Now().UTC()
	timeLapsed := currTime.Sub(st.lastProcessedTime).Seconds()
	if timeLapsed < float64(DELTA) {
		return
	}

	bytesTransferred := st.bytesDownloaded + st.bytesUploaded
	filesProcessed := st.success + st.failed
	filesPending := st.totalFiles - filesProcessed
	percentCompleted := (float64(filesProcessed) / float64(st.totalFiles)) * 100
	bandwidthMbps := float64(bytesTransferred*8) / (timeLapsed * float64(_1MB))

	log.Debug("statsManager::calculateBandwidth : timestamp %v, %v %, %v Done, %v Failed, "+
		"%v Pending, %v Total, Bytes transferred %v, Throughput (Mbps): %v",
		currTime.String(), percentCompleted, st.success, st.failed,
		filesPending, st.totalFiles, bytesTransferred, bandwidthMbps)

	st.lastProcessedTime = currTime
	// TODO:: xload : dump to json file
}
