/*
    _____           _____   _____   ____          ______  _____  ------
   |     |  |      |     | |     | |     |     | |       |            |
   |     |  |      |     | |     | |     |     | |       |            |
   | --- |  |      |     | |-----| |---- |     | |-----| |-----  ------
   |     |  |      |     | |     | |     |     |       | |       |
   | ____|  |_____ | ____| | ____| |     |_____|  _____| |_____  |_____


   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2020-2022 Microsoft Corporation. All rights reserved.
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

package internal

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal/stats_manager"
	hmcommon "github.com/Azure/azure-storage-fuse/v2/tools/health-monitor/common"
)

type ExportedStat struct {
	Timestamp   string
	MonitorName string
	Stat        interface{}
}

type StatsExporter struct {
	channel    chan ExportedStat
	wg         sync.WaitGroup
	opFile     *os.File
	outputList []*Output
	filesList  []string
	fileIdx    int
}

type Output struct {
	Timestamp string                  `json:"Timestamp,omitempty"`
	Bfs       []stats_manager.PipeMsg `json:"BlobfuseStats,omitempty"`
	FcEvent   []*hmcommon.CacheEvent  `json:"FileCache,omitempty"`
	Cpu       string                  `json:"CpuUsage,omitempty"`
	Mem       string                  `json:"MemoryUsage,omitempty"`
	Net       string                  `json:"NetworkUsage,omitempty"`
}

var expLock sync.Mutex
var se *StatsExporter

// create single instance of StatsExporter
func NewStatsExporter() (*StatsExporter, error) {
	if se == nil {
		expLock.Lock()
		defer expLock.Unlock()
		if se == nil {
			se = &StatsExporter{}
			se.fileIdx = 0
			se.channel = make(chan ExportedStat, 10000)
			se.wg.Add(1)
			go se.StatsExporter()

			err := se.getNewFile()
			if err != nil {
				log.Err("stats_export::NewStatsExporter : [%v]", err)
				return nil, err
			}
		}
	}

	return se, nil
}

func (se *StatsExporter) Destroy() {
	_, err := se.opFile.WriteString("\n]")
	if err != nil {
		log.Err("stats_exporter::NewStatsExporter : unable to write to file [%v]", err)
	}

	se.opFile.Close()
	close(se.channel)
	se.wg.Wait()
}

func (se *StatsExporter) AddMonitorStats(monName string, timestamp string, st interface{}) {
	// check if the channel is full
	if len(se.channel) == cap(se.channel) {
		// remove the first element from the channel
		<-se.channel
	}

	se.channel <- ExportedStat{
		Timestamp:   timestamp,
		MonitorName: monName,
		Stat:        st,
	}
}

func (se *StatsExporter) StatsExporter() {
	defer se.wg.Done()

	for st := range se.channel {
		idx := se.checkInList(st.Timestamp)
		if idx != -1 {
			se.addToList(&st, idx)
		} else {
			// keep max 4 timestamps in memory
			if len(se.outputList) >= 4 {
				err := se.addToOutputFile(se.outputList[0])
				if err != nil {
					log.Err("stats_export::StatsExporter : [%v]", err)
				}

				se.outputList = se.outputList[1:]
			}
			se.outputList = append(se.outputList, &Output{
				Timestamp: st.Timestamp,
			})

			se.addToList(&st, len(se.outputList)-1)
		}
	}
}

func (se *StatsExporter) addToList(st *ExportedStat, idx int) {
	if st.MonitorName == hmcommon.BlobfuseStats {
		se.outputList[idx].Bfs = append(se.outputList[idx].Bfs, st.Stat.(stats_manager.PipeMsg))
	} else if st.MonitorName == hmcommon.FileCacheMon {
		se.outputList[idx].FcEvent = append(se.outputList[idx].FcEvent, st.Stat.(*hmcommon.CacheEvent))
	} else if st.MonitorName == hmcommon.CpuProfiler {
		se.outputList[idx].Cpu = st.Stat.(string)
	} else if st.MonitorName == hmcommon.MemoryProfiler {
		se.outputList[idx].Mem = st.Stat.(string)
	} else if st.MonitorName == hmcommon.NetworkProfiler {
		se.outputList[idx].Net = st.Stat.(string)
	}
}

// check if the given timestamp is present in the output list
// return index if present else return -1
func (se *StatsExporter) checkInList(t string) int {
	for i, val := range se.outputList {
		if val.Timestamp == t {
			return i
		}
	}
	return -1
}

func (se *StatsExporter) addToOutputFile(op *Output) error {
	jsonData, err := json.MarshalIndent(op, "", "\t")
	if err != nil {
		log.Err("stats_exporter::addToOutputFile : unable to marshal [%v]", err)
		return err
	}

	_, err = se.opFile.Write(jsonData)
	if err != nil {
		log.Err("stats_exporter::addToOutputFile : unable to write to file [%v]", err)
		return err
	}

	err = se.checkOutputFile()
	if err != nil {
		log.Err("stats_exporter::addToOutputFile : [%v]", err)
		return err
	}

	return nil
}

func (se *StatsExporter) checkOutputFile() error {
	f, err := se.opFile.Stat()
	if err != nil {
		log.Err("stats_export::checkOutputFile : Unable to get file info [%v]", err)
		return err
	}

	sz := f.Size()

	// close current file and create a new file if the size of current file is greater than 10MB
	if sz >= hmcommon.OutputFileSizeinMB*common.MbToBytes {
		_, err = se.opFile.WriteString("\n]")
		if err != nil {
			log.Err("stats_exporter::checkOutputFile : unable to write to file [%v]", err)
			return err
		}

		log.Debug("stats_export::checkOutputFile : closing file %v", f.Name())
		se.opFile.Close()

		err = se.getNewFile()
		if err != nil {
			log.Err("stats_export::checkOutputFile : [%v]")
			return err
		}
		return nil
	} else {
		_, err = se.opFile.WriteString(",\n")
		if err != nil {
			log.Err("stats_exporter::checkOutputFile : unable to write to file [%v]", err)
			return err
		}
	}

	return nil
}

func (se *StatsExporter) getNewFile() error {
	currDir, err := os.Getwd()
	if err != nil {
		log.Err("stats_export::NewStatsExporter : [%v]", err)
		return err
	}

	se.fileIdx += 1
	fileName := fmt.Sprintf("%v_%v_%v.%v", hmcommon.OutputFileName, hmcommon.Pid, se.fileIdx, hmcommon.OutputFileExtension)
	se.opFile, err = os.OpenFile(filepath.Join(currDir, fileName), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0755)
	if err != nil {
		log.Err("stats_export::NewStatsExporter : Unable to create output file [%v]", err)
		return err
	}

	se.filesList = append(se.filesList, filepath.Join(currDir, fileName))

	// keep latest 10 output files
	if len(se.filesList) > hmcommon.OutputFileCount {
		se.deleteOldFile()
	}

	_, err = se.opFile.WriteString("[")
	if err != nil {
		log.Err("stats_exporter::NewStatsExporter : unable to write to file [%v]", err)
		return err
	}

	return nil
}

func (se *StatsExporter) deleteOldFile() {
	os.RemoveAll(se.filesList[0])
	log.Debug("stats_export::deleteOldFile : deleted output file %v", se.filesList[0])
	se.filesList = se.filesList[1:]
}

func CloseExporter() error {
	se, err := NewStatsExporter()
	if err != nil || se == nil {
		log.Err("stats_export::CloseExporter : Error in creating stats exporter instance [%v]", err)
		return err
	}

	se.Destroy()
	return nil
}
