/*
    _____           _____   _____   ____          ______  _____  ------
   |     |  |      |     | |     | |     |     | |       |            |
   |     |  |      |     | |     | |     |     | |       |            |
   | --- |  |      |     | |-----| |---- |     | |-----| |-----  ------
   |     |  |      |     | |     | |     |     |       | |       |
   | ____|  |_____ | ____| | ____| |     |_____|  _____| |_____  |_____


   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2020-2025 Microsoft Corporation. All rights reserved.
   Author : <blobfusedev@microsoft.com>

   Permission is hereby granted, free of charge, to any person obtaining a copy
   of this software and associated documentation files (the "Software"), to deal
   in the Software without restriction, including without limitation the rights
   to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
   copies of the Software, and to permit persons to whom the Software is
   furnished to do so, subject to the following conditions:

   The above copyright notice and this permission notice shall be included in all
   copies or substantial portions of the Software.

   THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
   IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
   FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
   AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
   LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
   OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
   SOFTWARE
*/

package xload

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
)

type StatsManager struct {
	totalFiles      uint64          // total number of files that have been scanned so far
	success         uint64          // number of files that have been successfully processed
	failed          uint64          // number of files that failed
	dirs            uint64          // number of directories processed
	bytesDownloaded uint64          // total number of bytes downloaded
	bytesUploaded   uint64          // total number of bytes uploaded
	startTime       time.Time       // variable indicating the time at which the stats manager started
	fileHandle      *os.File        // file where stats will be dumped
	waitGroup       sync.WaitGroup  // wait group to wait for stats manager thread to finish
	items           chan *StatsItem // channel to hold the stats items
	done            chan bool       // channel to indicate if the stats manager has completed or not
	pool            *BlockPool      // Object of block pool
}

type StatsItem struct {
	Component        string // component name which has exported the stat
	ListerCount      uint64 // number of files scanned by the lister in an iteration
	Name             string // name of the file processed
	Dir              bool   // flag to indicate if the item is a directory
	Success          bool   // flag to indicate if the file has been processed successfully or not
	Download         bool   // flag to denote upload or download
	BytesTransferred uint64 // bytes uploaded or downloaded for this file
}

type statsJSONData struct {
	Timestamp        string  `json:"Timestamp"`
	PercentCompleted float64 `json:"PercentCompleted"`
	Total            uint64  `json:"Total"`
	Done             uint64  `json:"Done"`
	Failed           uint64  `json:"Failed"`
	Pending          uint64  `json:"Pending"`
	BytesTransferred uint64  `json:"BytesTransferred"`
	BandwidthMbps    float64 `json:"Bandwidth(Mbps)"`
}

const (
	STATS_MANAGER  = "STATS_MANAGER"
	DURATION       = 4                        // time interval in seconds at which the stats will be dumped
	JSON_FILE_NAME = "xload_stats_{PID}.json" // json file name where the stats manager will dump the stats
)

func NewStatsManager(count uint32, isExportEnabled bool, pool *BlockPool) (*StatsManager, error) {
	var fh *os.File
	var err error
	if isExportEnabled {
		pid := fmt.Sprintf("%v", os.Getpid())
		path := common.ExpandPath(filepath.Join(common.DefaultWorkDir, strings.ReplaceAll(JSON_FILE_NAME, "{PID}", pid)))
		log.Crit("statsManager::NewStatsManager : creating json file %v", path)
		fh, err = os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0755)
		if err != nil {
			log.Err("statsManager::NewStatsManager : failed to create json file %v [%v]", path, err.Error())
			return nil, err
		}
	}

	return &StatsManager{
		fileHandle: fh,
		items:      make(chan *StatsItem, count*2),
		done:       make(chan bool, 1),
		pool:       pool,
	}, nil
}

func (sm *StatsManager) Start() {
	sm.waitGroup.Add(1)
	sm.startTime = time.Now().UTC()
	log.Debug("statsManager::start : start stats manager at time %v", sm.startTime.Format(time.RFC1123))
	_ = sm.writeToJSON([]byte("[\n"), false)
	_ = sm.marshalStatsData(&statsJSONData{Timestamp: sm.startTime.Format(time.RFC1123)}, false)
	_ = sm.writeToJSON([]byte("\n]"), false)
	go sm.statsProcessor()
	go sm.statsExporter()
}

// TODO:: xload : the stop method runs on unmount. See if the channels can be closed if the job is 100% complete
func (sm *StatsManager) Stop() {
	log.Debug("statsManager::stop : stop stats manager")
	sm.done <- true // close the stats exporter thread
	close(sm.done)  // TODO::xload : check if closing the done channel here will lead to closing the stats exporter thread
	close(sm.items)
	sm.waitGroup.Wait()

	if sm.fileHandle != nil {
		sm.fileHandle.Close()
	}
}

func (sm *StatsManager) AddStats(item *StatsItem) {
	sm.items <- item
}

func (sm *StatsManager) updateSuccessFailedCtr(isSuccess bool) {
	if isSuccess {
		sm.success += 1
	} else {
		sm.failed += 1
	}
}

func (sm *StatsManager) statsProcessor() {
	defer sm.waitGroup.Done()

	for item := range sm.items {
		switch item.Component {
		case LISTER:
			sm.totalFiles += item.ListerCount
			// log.Debug("statsManager::statsProcessor : Directory listed %v, total number of files listed so far = %v", item.name, sm.totalFiles)
			if item.Dir {
				sm.dirs += 1
				sm.updateSuccessFailedCtr(item.Success)
			}

		case SPLITTER:
			// log.Debug("statsManager::statsProcessor : splitter: Name %v, success %v, download %v", item.name, item.success, item.download)
			sm.updateSuccessFailedCtr(item.Success)

		case DATA_MANAGER:
			// log.Debug("statsManager::statsProcessor : data manager: Name %v, success %v, download %v, bytes transferred %v", item.name, item.success, item.download, item.bytesTransferred)
			if item.Download {
				sm.bytesDownloaded += item.BytesTransferred
			} else {
				sm.bytesUploaded += item.BytesTransferred
			}

		case STATS_MANAGER:
			sm.calculateBandwidth()

		default:
			log.Err("statsManager::statsProcessor : wrong component name used for sending stats")
		}
	}

	log.Debug("statsManager::statsProcessor : stats processor completed")
}

func (sm *StatsManager) statsExporter() {
	ticker := time.NewTicker(DURATION * time.Second)

	for {
		select {
		case <-sm.done:
			ticker.Stop()
			return
		case <-ticker.C:
			sm.AddStats(&StatsItem{
				Component: STATS_MANAGER,
			})
		}
	}
}

func (sm *StatsManager) calculateBandwidth() {
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
	bandwidthMbps := float64(bytesTransferred*8) / (timeLapsed * float64(MB))

	max, pr, reg := sm.pool.GetUsageDetails()
	log.Crit("statsManager::calculateBandwidth : timestamp %v, %.2f%%, %v Done, %v Failed, "+
		"%v Pending, %v Total, Bytes transferred %v, Throughput (Mbps): %.2f, Cache usage: %v%%, (%v / %v / %v)",
		currTime.Format(time.RFC1123), percentCompleted, sm.success, sm.failed,
		filesPending, sm.totalFiles, bytesTransferred, bandwidthMbps, sm.pool.Usage(),
		max, pr, reg)

	if sm.fileHandle != nil {
		err := sm.marshalStatsData(&statsJSONData{
			Timestamp:        currTime.Format(time.RFC1123),
			PercentCompleted: RoundFloat(percentCompleted, 2),
			Total:            sm.totalFiles,
			Done:             sm.success,
			Failed:           sm.failed,
			Pending:          filesPending,
			BytesTransferred: bytesTransferred,
			BandwidthMbps:    RoundFloat(bandwidthMbps, 2),
		}, true)
		if err != nil {
			log.Err("statsManager::calculateBandwidth : failed to write to json file [%v]", err.Error())
		}
	}

	// TODO:: xload : determine more effective way to decide if the listing has completed and the stats exporter can be terminated
	if sm.totalFiles == filesProcessed && sm.totalFiles != sm.dirs {
		sm.done <- true
		return
	}
}

func (sm *StatsManager) marshalStatsData(data *statsJSONData, seek bool) error {
	if sm.fileHandle == nil {
		return nil
	}

	jsonData, err := json.MarshalIndent(data, "", "\t")
	if err != nil {
		log.Err("statsManager::convertToBytes : unable to marshal [%v]", err.Error())
		return err
	}

	err = sm.writeToJSON(jsonData, seek)
	if err != nil {
		log.Err("statsManager::convertToBytes : failed to write to json file [%v]", err.Error())
		return err
	}

	return nil
}

func (sm *StatsManager) writeToJSON(data []byte, seek bool) error {
	if sm.fileHandle == nil {
		return nil
	}

	var err error
	if seek {
		_, err = sm.fileHandle.Seek(-2, io.SeekEnd)
		if err != nil {
			log.Err("statsManager::writeToJSON : failed to seek [%v]", err.Error())
			return err
		}

		_, err = sm.fileHandle.Write([]byte(",\n"))
		if err != nil {
			log.Err("statsManager::writeToJSON : failed to write to json file [%v]", err.Error())
			return err
		}
	}

	_, err = sm.fileHandle.Write(data)
	if err != nil {
		log.Err("statsManager::writeToJSON : failed to write to json file [%v]", err.Error())
		return err
	}

	if seek {
		_, err = sm.fileHandle.Write([]byte("\n]"))
		if err != nil {
			log.Err("statsManager::writeToJSON : failed to write to json file [%v]", err.Error())
			return err
		}
	}

	return nil
}
