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

	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal"
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
}

type Output struct {
	Timestamp string                `json:"Timestamp"`
	Bfs       []internal.Stats      `json:"BlobfuseStats"`
	FcEvent   []hmcommon.CacheEvent `json:"FileCache"`
	Cpu       string                `json:"CpuUsage"`
	Mem       string                `json:"MemoryUsage"`
	// commenting for now
	// Net       string                `json:"NetworkUsage"`
}

var expLock sync.Mutex
var se *StatsExporter

// create single instance of StatsExporter
func NewStatsExporter() (*StatsExporter, error) {
	if se == nil {
		expLock.Lock()
		defer expLock.Unlock()
		if se == nil {
			se := &StatsExporter{}
			se.channel = make(chan ExportedStat, 100000)
			se.wg.Add(1)
			go se.StatsExporter()

			currDir, err := os.Getwd()
			if err != nil {
				log.Err("stats_export::NewStatsExporter : [%v]", err)
				return nil, err
			}

			se.opFile, err = os.OpenFile(filepath.Join(currDir, hmcommon.OutputFile), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0755)
			if err != nil {
				log.Err("stats_export::NewStatsExporter : Unable to create output file [%v]", err)
				return nil, err
			}
		}
	}

	return se, nil
}

func (se *StatsExporter) Destroy() {
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
		idx, isPresent := se.checkInList(st.Timestamp)
		if isPresent {
			se.addToList(&st, idx)
		} else {
			if len(se.outputList) >= 10 {
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
		se.outputList[idx].Bfs = append(se.outputList[idx].Bfs, st.Stat.(internal.Stats))
	} else if st.MonitorName == hmcommon.FileCacheMon {
		se.outputList[idx].FcEvent = append(se.outputList[idx].FcEvent, st.Stat.(hmcommon.CacheEvent))
	} else if st.MonitorName == hmcommon.CpuProfiler {
		se.outputList[idx].Cpu = st.Stat.(string)
	} else if st.MonitorName == hmcommon.MemoryProfiler {
		se.outputList[idx].Mem = st.Stat.(string)
	}
	// else if st.MonitorName == hmcommon.NetworkProfiler {
	// 	se.outputList[idx].Net = st.Stat.(string)
	// }
}

func (se *StatsExporter) checkInList(t string) (int, bool) {
	for i, val := range se.outputList {
		if val.Timestamp == t {
			return i, true
		}
	}
	return -1, false
}

func (se *StatsExporter) addToOutputFile(op *Output) error {
	jsonData, err := json.MarshalIndent(op, "", "\t")
	if err != nil {
		log.Err("stats_exporter::addToOutputFile : unable to marshal [%v]", err)
		return err
	}
	fmt.Println(string(jsonData))

	_, err = se.opFile.Write(jsonData)
	if err != nil {
		log.Err("stats_exporter::addToOutputFile : unable to write to file [%v]", err)
		return err
	}

	_, err = se.opFile.WriteString("\n")
	if err != nil {
		log.Err("stats_exporter::addToOutputFile : unable to write to file [%v]", err)
		return err
	}

	return nil
}
