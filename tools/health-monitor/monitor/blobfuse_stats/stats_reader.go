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

package blobfuse_stats

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"syscall"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal/stats_manager"
	hmcommon "github.com/Azure/azure-storage-fuse/v2/tools/health-monitor/common"
	hminternal "github.com/Azure/azure-storage-fuse/v2/tools/health-monitor/internal"
)

type BlobfuseStats struct {
	name         string
	pollInterval int
	transferPipe string
	pollingPipe  string
}

func (bfs *BlobfuseStats) GetName() string {
	return bfs.name
}

func (bfs *BlobfuseStats) SetName(name string) {
	bfs.name = name
}

func (bfs *BlobfuseStats) Monitor() error {
	err := bfs.Validate()
	if err != nil {
		log.Err("StatsReader::Monitor : [%v]", err)
		return err
	}
	log.Debug("StatsReader::Monitor : started")

	go bfs.statsPoll()

	return bfs.statsReader()
}

func (bfs *BlobfuseStats) ExportStats(timestamp string, st interface{}) {
	se, err := hminternal.NewStatsExporter()
	if err != nil || se == nil {
		log.Err("StatsReader::ExportStats : Error in creating stats exporter instance [%v]", err)
		return
	}

	se.AddMonitorStats(bfs.GetName(), timestamp, st)
}

func (bfs *BlobfuseStats) Validate() error {
	if bfs.pollInterval == 0 {
		return fmt.Errorf("blobfuse-poll-interval should be non-zero")
	}

	err := hmcommon.CheckProcessStatus(hmcommon.Pid)
	if err != nil {
		return err
	}

	return nil
}

func (bfs *BlobfuseStats) statsReader() error {
	err := createPipe(bfs.transferPipe)
	if err != nil {
		log.Err("StatsReader::statsReader : [%v]", err)
		return err
	}

	f, err := os.OpenFile(bfs.transferPipe, os.O_RDONLY, os.ModeNamedPipe)
	if err != nil {
		log.Err("StatsReader::statsReader : unable to open pipe file [%v]", err)
		return err
	}
	defer f.Close()

	reader := bufio.NewReader(f)
	var e error = nil

	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			log.Err("StatsReader::statsReader : [%v]", err)
			e = err
			break
		}

		// log.Debug("StatsReader::statsReader : Line: %v", string(line))

		st := stats_manager.PipeMsg{}
		err = json.Unmarshal(line, &st)
		if err != nil {
			log.Err("StatsReader::statsReader : Unable to unmarshal json [%v]", err)
			continue
		}
		bfs.ExportStats(st.Timestamp, st)
	}

	return e
}

func (bfs *BlobfuseStats) statsPoll() {
	err := createPipe(bfs.pollingPipe)
	if err != nil {
		log.Err("StatsReader::statsPoll : [%v]", err)
		return
	}

	pf, err := os.OpenFile(bfs.pollingPipe, os.O_CREATE|os.O_WRONLY, 0777)
	if err != nil {
		log.Err("StatsReader::statsPoll : unable to open pipe file [%v]", err)
		return
	}
	defer pf.Close()

	ticker := time.NewTicker(time.Duration(bfs.pollInterval) * time.Second)
	defer ticker.Stop()

	for t := range ticker.C {
		_, err = pf.WriteString(fmt.Sprintf("Poll at %v\n", t.Format(time.RFC3339)))
		if err != nil {
			log.Err("StatsReader::statsPoll : [%v]", err)
			break
		}
	}
}

func createPipe(pipe string) error {
	_, err := os.Stat(pipe)
	if os.IsNotExist(err) {
		err = syscall.Mkfifo(pipe, 0666)
		if err != nil {
			log.Err("StatsReader::createPipe : unable to create pipe [%v]", err)
			return err
		}
	} else if err != nil {
		log.Err("StatsReader::createPipe : [%v]", err)
		return err
	}
	return nil
}

func NewBlobfuseStatsMonitor() hminternal.Monitor {
	bfs := &BlobfuseStats{
		pollInterval: hmcommon.BfsPollInterval,
		transferPipe: common.TransferPipe,
		pollingPipe:  common.PollingPipe,
	}

	bfs.SetName(hmcommon.BlobfuseStats)

	return bfs
}

func init() {
	hminternal.AddMonitor(hmcommon.BlobfuseStats, NewBlobfuseStatsMonitor)
}
