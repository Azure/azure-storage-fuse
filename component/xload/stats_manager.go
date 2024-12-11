package xload

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
)

type statsManager struct {
	totalFiles      uint64          // total number of files that have been scanned so far
	success         uint64          // number of files that have been successfully processed
	failed          uint64          // number of files that failed
	dirs            uint64          // number of directories processed
	bytesDownloaded uint64          // total number of bytes downloaded
	bytesUploaded   uint64          // total number of bytes uploaded
	startTime       time.Time       // variable indicating the time at which the stats manager started
	fileHandle      *os.File        // file where stats will be dumped
	wg              sync.WaitGroup  // wait group to wait for stats manager thread to finish
	items           chan *statsItem // channel to hold the stats items
	done            chan bool       //channel to indicate if the stats manager has completed or not
	// TODO:: xload :
	// bandwidth utilization
	// bytes downloaded
	// dump to json file
}

type statsItem struct {
	component        string // component name which has exported the stat
	listerCount      uint64 // number of files scanned by the lister in an iteration
	name             string // name of the file processed
	dir              bool   // flag to indicate if the item is a directory
	success          bool   // flag to indicate if the file has been processed successfully or not
	download         bool   // flag to denote upload or download
	bytesTransferred uint64 // bytes uploaded or downloaded for this file
}

const (
	STATS_MANAGER  = "STATS_MANAGER"
	DURATION       = 4                                     // time interval in seconds at which the stats will be dumped
	JSON_FILE_PATH = "~/.blobfuse2/xload_stats_{PID}.json" // json file path where the stats manager will dump the stats
)

func newStatsmanager(count uint32) (*statsManager, error) {
	pid := fmt.Sprintf("%v", os.Getpid())
	path := common.ExpandPath(strings.ReplaceAll(JSON_FILE_PATH, "{PID}", pid))
	fh, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		log.Err("statsManager::newStatsmanager : failed to create json file %v [%v]", path, err.Error())
		return nil, err
	}

	return &statsManager{
		fileHandle: fh,
		items:      make(chan *statsItem, count*2),
		done:       make(chan bool),
	}, nil
}

func (sm *statsManager) start() {
	sm.wg.Add(1)
	sm.startTime = time.Now().UTC()
	log.Debug("statsManager::start : start stats manager at time %v", sm.startTime.Format(time.RFC1123))
	go sm.statsProcessor()
	go sm.statsExporter()
}

func (sm *statsManager) stop() {
	log.Debug("statsManager::stop : stop stats manager")
	sm.fileHandle.Close()
	close(sm.done)
	close(sm.items)
	sm.wg.Wait()
}

func (sm *statsManager) addStats(item *statsItem) {
	sm.items <- item
}

func (sm *statsManager) statsProcessor() {
	defer sm.wg.Done()

	for item := range sm.items {
		switch item.component {
		case LISTER:
			sm.totalFiles += item.listerCount
			// log.Debug("statsManager::statsProcessor : Directory listed %v, total number of files listed so far = %v", item.name, sm.totalFiles)
			if item.dir {
				sm.dirs += 1
				if item.success {
					sm.success += 1
				} else {
					sm.failed += 1
				}
			}

		case SPLITTER:
			// log.Debug("statsManager::statsProcessor : splitter: Name %v, success %v, download %v", item.name, item.success, item.download)
			if item.success {
				sm.success += 1
			} else {
				sm.failed += 1
			}

		case DATA_MANAGER:
			// log.Debug("statsManager::statsProcessor : data manager: Name %v, success %v, download %v, bytes transferred %v", item.name, item.success, item.download, item.bytesTransferred)
			if item.download {
				sm.bytesDownloaded += item.bytesTransferred
			} else {
				sm.bytesUploaded += item.bytesTransferred
			}

		case STATS_MANAGER:
			sm.calculateBandwidth()

		default:
			log.Err("statsManager::statsProcessor : wrong component name used for sending stats")
		}
	}

	log.Debug("statsManager::statsProcessor : stats processor completed")
}

func (sm *statsManager) statsExporter() {
	ticker := time.NewTicker(DURATION * time.Second)

	for {
		select {
		case <-sm.done:
			ticker.Stop()
			return
		case <-ticker.C:
			sm.addStats(&statsItem{
				component: STATS_MANAGER,
			})
		}
	}
}

func (sm *statsManager) calculateBandwidth() {
	if sm.totalFiles == 0 {
		log.Debug("statsManager::calculateBandwidth : skipping as total files listed so far is %v", sm.totalFiles)
		return
	}

	currTime := time.Now().UTC()
	timeLapsed := currTime.Sub(sm.startTime).Seconds()
	bytesTransferred := sm.bytesDownloaded + sm.bytesUploaded
	filesProcessed := sm.success + sm.failed
	filesPending := sm.totalFiles - filesProcessed
	percentCompleted := (float64(filesProcessed) / float64(sm.totalFiles)) * 100
	bandwidthMbps := float64(bytesTransferred*8) / (timeLapsed * float64(_1MB))

	log.Debug("statsManager::calculateBandwidth : timestamp %v, %.2f%%, %v Done, %v Failed, "+
		"%v Pending, %v Total, Bytes transferred %v, Throughput (Mbps): %.2f",
		currTime.Format(time.RFC1123), percentCompleted, sm.success, sm.failed,
		filesPending, sm.totalFiles, bytesTransferred, bandwidthMbps)

	if sm.totalFiles == filesProcessed && sm.totalFiles != sm.dirs {
		sm.done <- true
	}

	// TODO:: xload : dump to json file
}
