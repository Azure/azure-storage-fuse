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
	"regexp"
	"strconv"
	"strings"
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
	//lastBytesUploaded   float64
	//lastBytesDownloaded float64
	//hasPrev             bool
}

type Output struct {
	Timestamp string                  `json:"Timestamp,omitempty"`
	Bfs       []stats_manager.PipeMsg `json:"BlobfuseStats,omitempty"`
	FcEvent   []*hmcommon.CacheEvent  `json:"FileCache,omitempty"`
	Cpu       string                  `json:"CPUUsage,omitempty"`
	Mem       string                  `json:"MemoryUsage,omitempty"`
	Net       string                  `json:"NetworkUsage,omitempty"`
	Policy    string                  `json:"Policy,omitempty"`
}

const monitorURL = "https://westus2.monitoring.azure.com/subscriptions/ba45b233-e2ef-4169-8808-49eb0d8eba0d/resourceGroups/sanjsingh-rg/providers/Microsoft.Compute/virtualMachines/sanjanavm2/metrics"

var expLock sync.Mutex
var se *StatsExporter
var hostname string

func init() {
	var err error
	hostname, err = os.Hostname()
	if err != nil {
		log.Warn("Unable to get hostname: %v", err)
		hostname = "unknown"
	}
	log.Debug("Hostname initialized: %s", hostname)
}

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
			// Keep max 3 timestamps in memory
			if len(se.outputList) >= 3 {

				metrics := se.parseAndValidateMetrics(se.outputList[0])
				if len(metrics) > 0 {
					token, err := getAzureMonitorToken()
					if err != nil {
						log.Err("Token fetch failed [%v]", err)
					} else {
						for name, value := range metrics {
							payload := buildAzureMonitorPayload(name, value, se.outputList[0].Timestamp)
							if payload == nil {
								log.Err("Failed to build payload for metric %s", name)
								continue
							}
							err := sendToAzureMonitorAPI(payload, token)
							if err != nil {
								log.Err("Failed to send metric %s to Azure Monitor [%v]", name, err)
							}
						}
					}
				} else {
					log.Info("No valid metrics to send for timestamp %s", se.outputList[0].Timestamp)
				}

				err := se.addToOutputFile(se.outputList[0])
				if err != nil {
					log.Err("addToOutputFile error: [%v]", err)
				}

				se.outputList = se.outputList[1:]
			}

			se.outputList = append(se.outputList, &Output{
				Timestamp: st.Timestamp,
			})
			se.addToList(&st, len(se.outputList)-1)
			log.Info("✅ New version of bfusemon has started")

			metrics := se.parseAndValidateMetrics(se.outputList[len(se.outputList)-1])
			if len(metrics) > 0 {
				token, err := getAzureMonitorToken()
				if err != nil {
					log.Err("Token fetch failed [%v]", err)
					continue
				}

				for name, value := range metrics {
					payload := buildAzureMonitorPayload(name, value, se.outputList[len(se.outputList)-1].Timestamp)
					if payload == nil {
						log.Err("Failed to build payload for metric %s", name)
						continue
					}
					err := sendToAzureMonitorAPI(payload, token)
					if err != nil {
						log.Err("Failed to send metric %s to Azure Monitor [%v]", name, err)
					}
				}
			} else {
				log.Info("No valid metrics to send for timestamp %s", se.outputList[len(se.outputList)-1].Timestamp)
			}
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

// parse and validate metrics from Output struct
func (se *StatsExporter) parseAndValidateMetrics(out *Output) map[string]float64 {
	numericSuffixRegex := regexp.MustCompile(`[^0-9.\-]+$`)
	metrics := map[string]string{
		"CPUUsage":     out.Cpu,
		"MemoryUsage":  out.Mem,
		"NetworkUsage": out.Net,
	}
	validMetrics := make(map[string]float64)

	for metricName, valueStr := range metrics {
		cleanStr := numericSuffixRegex.ReplaceAllString(valueStr, "")
		cleanStr = strings.TrimSpace(cleanStr)

		if cleanStr == "" {
			log.Warn("Empty value after cleaning for metric [%s]", metricName)
			continue
		}

		value, err := strconv.ParseFloat(cleanStr, 64)
		if err != nil {
			log.Err("Unable to parse value [%v] for metric [%s]", err, metricName)
			continue
		}
		validMetrics[metricName] = value
	}

	blobfuseMetrics := computeBlobfuseByteDeltas(out.Bfs)
	for k, v := range blobfuseMetrics {
		validMetrics[k] = v
	}

	return validMetrics
}

// buildAzureMonitorPayload constructs the payload for Azure Monitor API
func buildAzureMonitorPayload(metricName string, value float64, timestampStr string) []byte {
	timestamp, err := time.Parse(time.RFC3339, timestampStr)
	if err != nil {
		log.Err("buildAzureMonitorPayload: Invalid timestamp format [%v]", err)
		return nil
	}
	//mountPath := common.MountPath
	log.Debug("buildAzureMonitorPayload: Using mount path [%s]", hmcommon.MountPath)
	if hmcommon.MountPath == "" {
		log.Warn("buildAzureMonitorPayload: Mount path is empty, using default value")
		hmcommon.MountPath = "/mnt/blobfuse2"
	}

	payload := map[string]interface{}{
		"time": timestamp,
		"data": map[string]interface{}{
			"baseData": map[string]interface{}{
				"metric":    metricName,
				"namespace": "CustomMetrics",
				"dimNames":  []string{"MountPath", "HostName"},
				"series": []map[string]interface{}{
					{
						"dimValues": []string{hmcommon.MountPath, hostname},
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
		log.Err("buildAzureMonitorPayload: Unable to marshal payload [%v]", err)
		return nil
	}
	return jsonPayload
}

//get token for Azure Monitor using managed identity

func getAzureMonitorToken() (string, error) {
	log.Info("Creating managed identity credentials")
	cred, err := azidentity.NewManagedIdentityCredential(nil)
	if err != nil {
		log.Err("Unable to create managed identity credential [%v]", err)
		return "", err
	}

	log.Debug("Requesting token for Azure Monitor")
	token, err := cred.GetToken(context.Background(), policy.TokenRequestOptions{
		Scopes: []string{"https://monitor.azure.com/.default"},
	})
	if err != nil {
		log.Err("Unable to get token [%v]", err)
		return "", err
	}

	log.Debug("Token successfully retrieved")
	return token.Token, nil
}

// sendToAzureMonitorAPI sends the payload to Azure Monitor API

func sendToAzureMonitorAPI(payload []byte, token string) error {

	req, err := http.NewRequest("POST", monitorURL, bytes.NewBuffer(payload))
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-ms-monitor-metrics-format", "body")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("HTTP status %d", resp.StatusCode)
	}

	log.Info("sendToAzureMonitorAPI: Successfully posted metric")
	return nil
}

//var lastBytesTransferred float64 = -1
//var lastBytesDownloaded float64 = -1

func computeBlobfuseByteDeltas(bfsList []stats_manager.PipeMsg) map[string]float64 {
	metrics := make(map[string]float64)
	/*var transferred, downloaded float64
	var foundTransferred, foundDownloaded bool*/
	if len(bfsList) == 0 {
		return metrics
	}

	latest := bfsList[len(bfsList)-1]

	for k, v := range latest.Value {
		switch k {
		case
			"InformationalCount",
			"SuccessCount",
			"RedirectCount",
			"ClientErrorCount",
			"ServerErrorCount",
			"FailureCount",
			"totalRequests",
			"GetRequestCount",
			"PostRequestCount",
			"PutRequestCount",
			"DeleteRequestCount",
			"HeadRequestCount",
			"Bytes Downloaded",
			"Bytes Uploaded",
			"OtherRequestCount":

			if val, ok := v.(int); ok {
				metrics[k] = float64(val)
			} else if val64, ok := v.(int64); ok {
				metrics[k] = float64(val64)
			} else if valFloat, ok := v.(float64); ok {
				metrics[k] = valFloat
			}
		}
	}

	/*for _, bfs := range bfsList {
		// Handle "Bytes Transferred"
		if btRaw, ok := bfs.Value["Bytes Transferred"]; ok {
			if bt, ok := btRaw.(float64); ok {
				transferred += bt
				foundTransferred = true
			}
		}
		// Handle "BytesDownloaded"
		if bdRaw, ok := bfs.Value["BytesDownloaded"]; ok {
			if bd, ok := bdRaw.(float64); ok {
				downloaded += bd
				foundDownloaded = true
			}
		}
	}

	// Compute deltas
	if foundTransferred && lastBytesTransferred >= 0 {
		metrics["BytesTransferredDelta"] = transferred - lastBytesTransferred
	}
	if foundDownloaded && lastBytesDownloaded >= 0 {
		metrics["BytesDownloadedDelta"] = downloaded - lastBytesDownloaded
	}

	// Update last values
	if foundTransferred {
		lastBytesTransferred = transferred
	}
	if foundDownloaded {
		lastBytesDownloaded = downloaded
	}*/

	return metrics
}
