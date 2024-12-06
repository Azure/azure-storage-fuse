package xload

import (
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common/log"
)

type statsManager struct {
	totalFiles      uint64    // total number of files that have been scanned so far
	success         uint64    // number of files that have been successfully processed
	failed          uint64    // number of files that failed
	bytesDownloaded uint64    // total number of bytes downloaded
	bytesUploaded   uint64    // total number of bytes uploaded
	startTime       time.Time // variable indicating the time at which the stats manager started
	items           chan *statsItem
	// TODO:: xload :
	// bandwidth utilization
	// bytes downloaded
	// dump to json file
}

type statsItem struct {
	listerCount      uint64    // number of files scanned by the lister in an iteration
	name             string    // name of the file processed
	success          bool      // flag to indicate if the file has been processed successfully or not
	download         bool      // flag to denote upload or download
	bytesTransferred uint64    // bytes uploaded or downloaded for this file
	timestamp        time.Time // time at which the stat was pushed to the channel
}

func newStatsmanager(count uint32) *statsManager {
	return &statsManager{
		startTime: time.Now().UTC(),
		items:     make(chan *statsItem, count*2),
	}
}

func (st *statsManager) statsProcessor(item *statsItem) {
	for item := range st.items {
		if item.listerCount > 0 {
			// stats sent by the lister component
			log.Debug("statsManager::statsProcessor : Directory listed %v, count %v", item.name, item.listerCount)
			st.totalFiles += item.listerCount
			log.Debug("statsManager::statsProcessor : Total number of files listed so far = %v", st.totalFiles)
		} else {
			// stats sent by the splitter component
			log.Debug("statsManager::statsProcessor : Name %v, success %v, download %v, bytes transferred %v", item.name, item.success, item.download, item.bytesTransferred)

			if item.success {
				st.success += 1
			} else {
				st.failed += 1
			}

			if item.download {
				st.bytesDownloaded += item.bytesTransferred
			} else {
				st.bytesUploaded += item.bytesTransferred
			}

			st.calculateBandwidth(item.timestamp)
		}
	}
}

func (st *statsManager) calculateBandwidth(timestamp time.Time) {
	bytesTransferred := st.bytesDownloaded + st.bytesUploaded
	filesProcessed := st.success + st.failed
	filesPending := st.totalFiles - filesProcessed
	percentCompleted := (float64(filesProcessed) / float64(st.totalFiles)) * 100
	bandwidthMbps := float64(bytesTransferred*8) / (timestamp.Sub(st.startTime).Seconds() * float64(_1MB))

	log.Debug("statsManager::calculateBandwidth : %v %, %v Done, %v Failed, %v Pending, %v Total, Throughput (Mbps): %v", percentCompleted, st.success, st.failed, filesPending, st.totalFiles, bandwidthMbps)

	// TODO:: xload : dump to json file
}
