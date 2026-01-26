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

package stats_manager

import (
	"maps"
	"sync"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
)

type StatsCollector struct {
	channel    chan ChannelMsg
	workerDone sync.WaitGroup
	compIdx    int
}

type PipeMsg struct {
	Timestamp     string         `json:"timestamp"`
	ComponentName string         `json:"componentName,omitempty"`
	Operation     string         `json:"operation,omitempty"`
	Path          string         `json:"path,omitempty"`
	Value         map[string]any `json:"value,omitempty"`
}

type Events struct {
	Timestamp string
	Operation string
	Path      string
	Value     map[string]any
}

type Stats struct {
	Timestamp string
	Operation string
	Key       string
	Value     any
}

type ChannelMsg struct {
	IsEvent bool
	CompMsg any
}

type statsManagerOpt struct {
	statsList []*PipeMsg
	// map to store the last updated timestamp of component's stats
	// This way a component's stat which was not updated is not pushed to the transfer pipe
	cmpTimeMap  map[string]string
	pollStarted bool
	transferMtx sync.Mutex
	pollMtx     sync.Mutex
	statsMtx    sync.Mutex
}

var stMgrOpt statsManagerOpt

func NewStatsCollector(componentName string) *StatsCollector {
	sc := &StatsCollector{}

	if common.MonitorBfs() {
		sc.channel = make(chan ChannelMsg, 10000)

		stMgrOpt.statsMtx.Lock()

		sc.compIdx = len(stMgrOpt.statsList)
		cmpSt := PipeMsg{
			Timestamp:     time.Now().Format(time.RFC3339),
			ComponentName: componentName,
			Operation:     "",
			Value:         make(map[string]any),
		}
		stMgrOpt.statsList = append(stMgrOpt.statsList, &cmpSt)

		stMgrOpt.cmpTimeMap[componentName] = cmpSt.Timestamp

		stMgrOpt.statsMtx.Unlock()

		sc.Init()
		log.Debug("stats_manager::NewStatsCollector : %v", componentName)
	}

	return sc
}

func (sc *StatsCollector) Init() {
	sc.workerDone.Add(1)
	go sc.statsDumper()

	stMgrOpt.pollMtx.Lock()
	defer stMgrOpt.pollMtx.Unlock()
	if !stMgrOpt.pollStarted {
		stMgrOpt.pollStarted = true
		go statsPolling()
	}
}

func (sc *StatsCollector) Destroy() {
	if common.MonitorBfs() {
		close(sc.channel)
		sc.workerDone.Wait()
	}
}

func (sc *StatsCollector) PushEvents(op string, path string, mp map[string]any) {
	if common.MonitorBfs() {
		event := Events{
			Timestamp: time.Now().Format(time.RFC3339),
			Operation: op,
			Path:      path,
		}

		if mp != nil {
			event.Value = make(map[string]any)
			maps.Copy(event.Value, mp)
		}

		// check if the channel is full
		if len(sc.channel) == cap(sc.channel) {
			// remove the first element from the channel
			<-sc.channel
		}

		sc.channel <- ChannelMsg{
			IsEvent: true,
			CompMsg: event,
		}
	}
}

func (sc *StatsCollector) UpdateStats(op string, key string, val any) {
	if common.MonitorBfs() {
		st := Stats{
			Timestamp: time.Now().Format(time.RFC3339),
			Operation: op,
			Key:       key,
			Value:     val,
		}

		// check if the channel is full
		if len(sc.channel) == cap(sc.channel) {
			// remove the first element from the channel
			<-sc.channel
		}

		sc.channel <- ChannelMsg{
			IsEvent: false,
			CompMsg: st,
		}
	}
}

func disableMonitoring() {
	common.EnableMonitoring = false
	log.Debug("stats_manager::disableMonitoring : disabling monitoring flag")
}

func init() {
	stMgrOpt = statsManagerOpt{}
	stMgrOpt.pollStarted = false
	stMgrOpt.cmpTimeMap = make(map[string]string)
}
