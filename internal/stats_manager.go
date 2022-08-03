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
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"syscall"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
)

type ChannelReader func()

const (
	// Stats collection operation types
	Increment = "increment"
	Decrement = "decrement"
	Replace   = "replace"

	// AzStorage stats types
	BytesDownloaded = "Bytes Downloaded"
	BytesUploaded   = "Bytes Uploaded"

	// File Cache stats types
	CacheUsage   = "Cache Usage"
	UsagePercent = "Usage Percent"
)

type StatsCollector struct {
	channel    chan ChannelMsg
	workerDone sync.WaitGroup
	reader     ChannelReader
	compIdx    int
}

type Stats struct {
	Timestamp     string                 `json:"timestamp"`
	ComponentName string                 `json:"componentName"`
	Operation     string                 `json:"operation"`
	Path          string                 `json:"path"`
	Value         map[string]interface{} `json:"value"`
}

type ChannelMsg struct {
	IsEvent   bool
	CompStats Stats
}

var transferPipe = "/home/sourav/monitorPipe"
var pollingPipe = "/home/sourav/pollPipe"
var statsList []*Stats
var cmpTimeMap map[string]string = make(map[string]string)
var pollStarted bool = false

var transferMtx sync.Mutex
var pollMtx sync.Mutex
var statsMtx sync.Mutex

func NewStatsCollector(componentName string, reader ChannelReader) *StatsCollector {
	sc := &StatsCollector{}

	if common.EnableMonitoring {
		sc.channel = make(chan ChannelMsg, 100000)
		sc.reader = reader

		statsMtx.Lock()

		sc.compIdx = len(statsList)
		cmpSt := Stats{
			Timestamp:     time.Now().Format(time.RFC3339),
			ComponentName: componentName,
			Operation:     "Stats Collected",
			Value:         make(map[string]interface{})}
		statsList = append(statsList, &cmpSt)

		cmpTimeMap[componentName] = cmpSt.Timestamp

		statsMtx.Unlock()

		sc.Init()
	}

	return sc
}

func (sc *StatsCollector) Init() {
	sc.workerDone.Add(1)
	go sc.statsDumper()

	pollMtx.Lock()
	defer pollMtx.Unlock()
	if !pollStarted {
		pollStarted = true
		go statsPolling()
	}
}

func (sc *StatsCollector) Destroy() error {
	close(sc.channel)
	sc.workerDone.Wait()
	return nil
}

func (sc *StatsCollector) AddStats(cmpName string, op string, path string, isEvent bool, mp map[string]interface{}) {
	if common.EnableMonitoring {
		st := Stats{
			ComponentName: cmpName,
			Operation:     op,
			Path:          path,
			Timestamp:     time.Now().Format(time.RFC3339)}

		if mp != nil {
			st.Value = mp
		}

		sc.channel <- ChannelMsg{IsEvent: isEvent, CompStats: st}
	}
}

func (sc *StatsCollector) statsDumper() {
	defer sc.workerDone.Done()

	err := createPipe(transferPipe)
	if err != nil {
		log.Err("StatsManager::StatsDumper : [%v]", err)
		return
	}

	f, err := os.OpenFile(transferPipe, os.O_CREATE|os.O_WRONLY, 0777)
	if err != nil {
		log.Err("StatsManager::StatsDumper : unable to open pipe file [%v]", err)
		return
	}
	defer f.Close()

	log.Info("StatsManager::StatsDumper : opened transfer pipe file")

	for st := range sc.channel {
		log.Debug("StatsManager::StatsDumper : stats: %v", st)
		if st.IsEvent {
			msg, err := json.Marshal(st.CompStats)
			if err != nil {
				log.Err("StatsManager::StatsDumper : Unable to marshal [%v]", err)
				continue
			}

			log.Debug("StatsManager::StatsDumper : stats: %v", string(msg))

			transferMtx.Lock()
			_, err = f.WriteString(fmt.Sprintf("%v\n", string(msg)))
			transferMtx.Unlock()
			if err != nil {
				log.Err("StatsManager::StatsDumper : Unable to write to pipe [%v]", err)
				break
			}

		} else {
			// accumulate component level stats
			for key, val := range st.CompStats.Value {
				idx := sc.compIdx

				statsMtx.Lock()

				_, isPresent := statsList[idx].Value[key]
				if !isPresent {
					statsList[idx].Value[key] = (int64)(0)
				}

				switch st.CompStats.Operation {
				case Increment:
					statsList[idx].Value[key] = statsList[idx].Value[key].(int64) + val.(int64)

				case Decrement:
					statsList[idx].Value[key] = statsList[idx].Value[key].(int64) - val.(int64)
					if statsList[idx].Value[key].(int64) < 0 {
						statsList[idx].Value[key] = (int64)(0)
					}

				case Replace:
					statsList[idx].Value[key] = val

				default:
					log.Debug("StatsManager::StatsDumper : Incorrect operation for stats collection")
					statsMtx.Unlock()
					continue
				}
				statsList[idx].Timestamp = time.Now().Format(time.RFC3339)

				statsMtx.Unlock()
			}
		}
	}
}

func statsPolling() {
	// create polling pipe
	err := createPipe(pollingPipe)
	if err != nil {
		log.Err("StatsManager::StatsPolling : [%v]", err)
		return
	}

	// open polling pipe
	pf, err := os.OpenFile(pollingPipe, os.O_RDONLY, os.ModeNamedPipe)
	if err != nil {
		fmt.Printf("StatsManager::StatsPolling : unable to open pipe file [%v]", err)
		return
	}
	defer pf.Close()

	log.Info("StatsManager::StatsPolling : opened polling pipe file")

	reader := bufio.NewReader(pf)

	// create transfer pipe
	// TODO: case where multiple threads try to create the pipe simultaneously
	err = createPipe(transferPipe)
	if err != nil {
		log.Err("StatsManager::StatsPolling : [%v]", err)
		return
	}

	// open transfer pipe
	tf, err := os.OpenFile(transferPipe, os.O_CREATE|os.O_WRONLY, 0777)
	if err != nil {
		log.Err("StatsManager::StatsPolling : unable to open pipe file [%v]", err)
		return
	}
	defer tf.Close()

	log.Info("StatsManager::StatsPolling : opened transfer pipe file")

	for {
		// read the polling message sent by stats monitor
		line, err := reader.ReadBytes('\n')
		if err != nil {
			log.Err("StatsReader::Reader : [%v]", err)
			break
		}
		log.Debug("StatsManager::StatsPolling : Polling message: %v\n", string(line))

		statsMtx.Lock()
		for _, cmpSt := range statsList {
			if len(cmpSt.Value) == 0 {
				continue
			}

			if cmpSt.Timestamp == cmpTimeMap[cmpSt.ComponentName] {
				log.Debug("StatsManager::StatsPolling : Skipping as there is no change in stats collected for %v", cmpSt.ComponentName)
				continue
			}

			msg, err := json.Marshal(cmpSt)
			if err != nil {
				log.Err("StatsManager::StatsPolling : Unable to marshal [%v]", err)
				continue
			}

			log.Debug("StatsManager::StatsPolling : stats: %v", string(msg))

			// send the stats collected so far to transfer pipe
			transferMtx.Lock()
			_, err = tf.WriteString(fmt.Sprintf("%v\n", string(msg)))
			transferMtx.Unlock()
			if err != nil {
				log.Err("StatsManager::StatsDumper : Unable to write to pipe [%v]", err)
				break
			}

			cmpTimeMap[cmpSt.ComponentName] = cmpSt.Timestamp
		}
		statsMtx.Unlock()
	}
}

func createPipe(pipe string) error {
	pollMtx.Lock()
	defer pollMtx.Unlock()

	_, err := os.Stat(pipe)
	if os.IsNotExist(err) {
		err = syscall.Mkfifo(pipe, 0666)
		if err != nil {
			log.Err("StatsManager::createPipe : unable to create pipe %v [%v]", pipe, err)
			return err
		}
	} else if err != nil {
		log.Err("StatsManager::createPipe : [%v]", err)
		return err
	}
	return nil
}
