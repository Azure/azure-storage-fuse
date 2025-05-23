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

package internal

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
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
}

type Output struct {
	Timestamp string                  `json:"Timestamp,omitempty"`
	Bfs       []stats_manager.PipeMsg `json:"BlobfuseStats,omitempty"`
	FcEvent   []*hmcommon.CacheEvent  `json:"FileCache,omitempty"`
	Cpu       string                  `json:"CPUUsage,omitempty"`
	Mem       string                  `json:"MemoryUsage,omitempty"`
	Net       string                  `json:"NetworkUsage,omitempty"`
}

const monitorURL = "https://centralus.monitoring.azure.com/subscriptions/ba45b233-e2ef-4169-8808-49eb0d8eba0d/resourceGroups/sanjsingh-rg/providers/Microsoft.Compute/virtualMachines/sanjana/metrics"

var expLock sync.Mutex
var se *StatsExporter

// atomic variable to prevent writing to channel after it has been closed
var pidStatus int32 = 0

// create single instance of StatsExporter
func NewStatsExporter() (*StatsExporter, error) {
	if se == nil {
		expLock.Lock()
		defer expLock.Unlock()
		if se == nil {
			se = &StatsExporter{}
			se.channel = make(chan ExportedStat, 10000)
			se.wg.Add(1)
			go se.StatsExporter()

			err := se.getNewFile()
			if err != nil {
				log.Err("stats_exporter::NewStatsExporter : [%v]", err)
				return nil, err
			}
		}
	}

	return se, nil
}

func (se *StatsExporter) Destroy() {
	// add 1 to the atomic variable. This will prevent writing to it in AddMonitorStats() method
	atomic.AddInt32(&pidStatus, 1)

	// write remaining data to the output file
	for i, op := range se.outputList {
		jsonData, err := json.MarshalIndent(op, "", "\t")
		if err != nil {
			log.Err("stats_exporter::Destroy : unable to marshal [%v]", err)
		}

		_, err = se.opFile.Write(jsonData)
		if err != nil {
			log.Err("stats_exporter::Destroy : unable to write to file [%v]", err)
		}

		if i != len(se.outputList)-1 {
			_, err := se.opFile.WriteString(",\n")
			if err != nil {
				log.Err("stats_exporter::Destroy : unable to write to file [%v]", err)
			}
		} else {
			_, err := se.opFile.WriteString("\n]")
			if err != nil {
				log.Err("stats_exporter::Destroy : unable to write to file [%v]", err)
			}
		}
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

	if atomic.LoadInt32(&pidStatus) == 0 {
		se.channel <- ExportedStat{
			Timestamp:   timestamp,
			MonitorName: monName,
			Stat:        st,
		}
	}
}

func (se *StatsExporter) StatsExporter() {
	defer se.wg.Done()

	for st := range se.channel {
		idx := se.checkInList(st.Timestamp)
		if idx != -1 {
			se.addToList(&st, idx)
		} else {
			// keep max 3 timestamps in memory
			if len(se.outputList) >= 3 {
				_ = se.sendToAzureMonitor(se.outputList[0])
				err := se.addToOutputFile(se.outputList[0])
				if err != nil {
					log.Err("stats_exporter::StatsExporter : [%v]", err)
				}

				se.outputList = se.outputList[1:]
			}
			se.outputList = append(se.outputList, &Output{
				Timestamp: st.Timestamp,
			})

			se.addToList(&st, len(se.outputList)-1)
			_ = se.sendToAzureMonitor(se.outputList[len(se.outputList)-1])
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
		log.Err("stats_exporter::checkOutputFile : Unable to get file info [%v]", err)
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

		log.Debug("stats_exporter::checkOutputFile : closing file %v", f.Name())
		se.opFile.Close()

		err = se.getNewFile()
		if err != nil {
			log.Err("stats_exporter::checkOutputFile : [%v]")
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
	var fname string
	var fnameNew string
	var err error

	baseName := filepath.Join(hmcommon.OutputPath, hmcommon.OutputFileName)

	// Remove the oldest file
	fname = fmt.Sprintf("%v_%v_%v.%v", baseName, hmcommon.Pid, (hmcommon.OutputFileCount - 1), hmcommon.OutputFileExtension)
	_ = os.Remove(fname)

	for i := hmcommon.OutputFileCount - 2; i > 0; i-- {
		fname = fmt.Sprintf("%v_%v_%v.%v", baseName, hmcommon.Pid, i, hmcommon.OutputFileExtension)
		fnameNew = fmt.Sprintf("%v_%v_%v.%v", baseName, hmcommon.Pid, (i + 1), hmcommon.OutputFileExtension)

		// Move each file to next number 8 -> 9, 7 -> 8, 6 -> 7 ...
		_ = os.Rename(fname, fnameNew)
	}

	// Rename the latest file to _1
	fname = fmt.Sprintf("%v_%v.%v", baseName, hmcommon.Pid, hmcommon.OutputFileExtension)
	fnameNew = fmt.Sprintf("%v_%v_1.%v", baseName, hmcommon.Pid, hmcommon.OutputFileExtension)
	_ = os.Rename(fname, fnameNew)

	fname = fmt.Sprintf("%v_%v.%v", baseName, hmcommon.Pid, hmcommon.OutputFileExtension)
	se.opFile, err = os.OpenFile(fname, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0755)
	if err != nil {
		log.Err("stats_exporter::getNewFile : Unable to create output file [%v]", err)
		return err
	}

	_, err = se.opFile.WriteString("[")
	if err != nil {
		log.Err("stats_exporter::getNewFile : unable to write to file [%v]", err)
		return err
	}

	return nil
}

func CloseExporter() error {
	se, err := NewStatsExporter()
	if err != nil || se == nil {
		log.Err("stats_exporter::CloseExporter : Error in creating stats exporter instance [%v]", err)
		return err
	}

	se.Destroy()
	return nil
}

func (se *StatsExporter) sendToAzureMonitor(out *Output) error {
	timestamp, err := time.Parse(time.RFC3339, out.Timestamp)
	if err != nil {
		log.Err("stats_exporter::sendToAzureMonitor : Unable to parse timestamp [%v]", err)
		return err
	}

	metrics := map[string]string{
		"CPUUsage":     out.Cpu,
		"MemoryUsage":  out.Mem,
		"NetworkUsage": out.Net,
	}
	log.Info("stats_exporter::sendToAzureMonitor : Creating managed identity credentials")
	cred, err := azidentity.NewManagedIdentityCredential(nil)
	if err != nil {
		log.Err("stats_exporter::sendToAzureMonitor : Unable to create managed identity credential [%v]", err)
		return err
	}

	log.Debug("stats_exporter::sendToAzureMonitor : Requesting token for Azure Monitor")
	token, err := cred.GetToken(context.Background(), policy.TokenRequestOptions{
		Scopes: []string{"https://monitor.azure.com/.default"},
	})
	if err != nil {
		log.Err("stats_exporter::sendToAzureMonitor : Unable to get token [%v]", err)
		return err
	}
	log.Debug("stats_exporter::sendToAzureMonitor : Token successfully retrieved")

	for metricName, valueStr := range metrics {
		value, err := strconv.ParseFloat(valueStr, 64)
		if err != nil {
			log.Err("stats_exporter::sendToAzureMonitor : Unable to parse value [%v] for metric [%s]", err, metricName)
			continue
		}
		log.Debug("stats_exporter::sendToAzureMonitor : Preparing to send metric [%s] with value [%f]", metricName, value)

		payload := map[string]interface{}{
			"time": timestamp,
			"data": map[string]interface{}{
				"baseData": map[string]interface{}{
					"metric":    metricName,
					"namespace": "CustomMetrics",
					"dimNames":  []string{"host"},
					"series": []map[string]interface{}{
						{
							"dimValues": []string{"host1"}, // Replace with dynamic host if needed
							"min":       value,
							"max":       value,
							"sum":       value,
							"count":     1,
						},
					},
				},
			},
		}

		jsonPayload, err := json.Marshal(payload)
		if err != nil {
			log.Err("stats_exporter::sendToAzureMonitor : Unable to marshal payload [%v]", err)
			continue
		}

		log.Debug("stats_exporter::sendToAzureMonitor : Payload: %s", string(jsonPayload))

		req, err := http.NewRequest("POST", monitorURL, bytes.NewBuffer(jsonPayload))
		if err != nil {
			log.Err("stats_exporter::sendToAzureMonitor : Unable to create request [%v]", err)
			continue
		}
		req.Header.Set("Authorization", "Bearer "+token.Token)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-ms-monitor-metrics-format", "body")

		log.Debug("stats_exporter::sendToAzureMonitor : Sending request for metric [%s]", metricName)

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			log.Err("stats_exporter::sendToAzureMonitor : Request failed [%v]", err)
			continue
		}
		defer resp.Body.Close()
		log.Debug("stats_exporter::sendToAzureMonitor : Received response status [%s]", resp.Status)

		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
			log.Err("stats_exporter::sendToAzureMonitor : Failed to post metric [%s], status code [%d]", metricName, resp.StatusCode)
		} else {
			log.Info("stats_exporter::sendToAzureMonitor : Successfully posted metric [%s]", metricName)
		}
	}

	return nil
}
