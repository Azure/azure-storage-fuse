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
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
)


func (sc *StatsCollector) statsDumper() {
	defer sc.workerDone.Done()

	err := createPipe(common.TransferPipe)
	if err != nil {
		log.Err("stats_manager::statsDumper : [%v]", err)
		disableMonitoring()
		return
	}

	f, err := os.OpenFile(common.TransferPipe, os.O_CREATE|os.O_WRONLY, 0777)
	if err != nil {
		log.Err("stats_manager::statsDumper : unable to open pipe file [%v]", err)
		disableMonitoring()
		return
	}
	defer f.Close()

	log.Info("stats_manager::statsDumper : opened transfer pipe file")

	for st := range sc.channel {
		// log.Debug("stats_manager::statsDumper : stats: %v", st)

		idx := sc.compIdx
		if st.IsEvent {
			event := st.CompMsg.(Events)
			pipeMsg := PipeMsg{
				Timestamp:     event.Timestamp,
				ComponentName: stMgrOpt.statsList[idx].ComponentName,
				Operation:     event.Operation,
				Path:          event.Path,
				Value:         event.Value,
			}

			msg, err := json.Marshal(pipeMsg)
			if err != nil {
				log.Err("stats_manager::statsDumper : Unable to marshal [%v]", err)
				continue
			}

			// log.Debug("stats_manager::statsDumper : stats: %v", string(msg))

			stMgrOpt.transferMtx.Lock()
			_, err = fmt.Fprintf(f, "%v\n", string(msg))
			stMgrOpt.transferMtx.Unlock()
			if err != nil {
				log.Err("stats_manager::statsDumper : Unable to write to pipe [%v]", err)
				disableMonitoring()
				break
			}

		} else {
			// accumulate component level stats
			stat := st.CompMsg.(Stats)

			// TODO: check if this lock can be removed
			stMgrOpt.statsMtx.Lock()

			_, isPresent := stMgrOpt.statsList[idx].Value[stat.Key]
			if !isPresent {
				stMgrOpt.statsList[idx].Value[stat.Key] = (int64)(0)
			}

			switch stat.Operation {
			case Increment:
				stMgrOpt.statsList[idx].Value[stat.Key] = stMgrOpt.statsList[idx].Value[stat.Key].(int64) + stat.Value.(int64)

			case Decrement:
				stMgrOpt.statsList[idx].Value[stat.Key] = stMgrOpt.statsList[idx].Value[stat.Key].(int64) - stat.Value.(int64)
				if stMgrOpt.statsList[idx].Value[stat.Key].(int64) < 0 {
					log.Err("stats_manager::statsDumper : Negative value %v after decrement of %v for component %v",
						stMgrOpt.statsList[idx].Value[stat.Key], stat.Key, stMgrOpt.statsList[idx].ComponentName)
				}

			case Replace:
				stMgrOpt.statsList[idx].Value[stat.Key] = stat.Value

			default:
				log.Debug("stats_manager::statsDumper : Incorrect operation for stats collection")
				stMgrOpt.statsMtx.Unlock()
				continue
			}
			stMgrOpt.statsList[idx].Timestamp = stat.Timestamp

			stMgrOpt.statsMtx.Unlock()
		}
	}
}

func statsPolling() {
	// create polling pipe
	err := createPipe(common.PollingPipe)
	if err != nil {
		log.Err("stats_manager::statsPolling : [%v]", err)
		disableMonitoring()
		return
	}

	// open polling pipe
	pf, err := os.OpenFile(common.PollingPipe, os.O_RDONLY, os.ModeNamedPipe)
	if err != nil {
		fmt.Printf("stats_manager::statsPolling : unable to open pipe file [%v]", err)
		return
	}
	defer pf.Close()

	log.Info("stats_manager::statsPolling : opened polling pipe file")

	reader := bufio.NewReader(pf)

	// create transfer pipe
	err = createPipe(common.TransferPipe)
	if err != nil {
		log.Err("stats_manager::statsPolling : [%v]", err)
		disableMonitoring()
		return
	}

	// open transfer pipe
	tf, err := os.OpenFile(common.TransferPipe, os.O_CREATE|os.O_WRONLY, 0777)
	if err != nil {
		log.Err("stats_manager::statsPolling : unable to open pipe file [%v]", err)
		disableMonitoring()
		return
	}
	defer tf.Close()

	log.Info("stats_manager::statsPolling : opened transfer pipe file")

	for {
		// read the polling message sent by stats monitor
		line, err := reader.ReadBytes('\n')
		if err != nil {
			log.Err("stats_manager::statsPolling : Unable to read from pipe [%v]", err)
			disableMonitoring()
			break
		}

		// validating poll message
		if !strings.Contains(string(line), "Poll at") {
			continue
		}

		// TODO: check if this lock can be removed
		stMgrOpt.statsMtx.Lock()
		for _, cmpSt := range stMgrOpt.statsList {
			if len(cmpSt.Value) == 0 {
				continue
			}

			if cmpSt.Timestamp == stMgrOpt.cmpTimeMap[cmpSt.ComponentName] {
				log.Debug("stats_manager::statsPolling : Skipping as there is no change in stats collected for %v", cmpSt.ComponentName)
				continue
			}

			msg, err := json.Marshal(cmpSt)
			if err != nil {
				log.Err("stats_manager::statsPolling : Unable to marshal [%v]", err)
				continue
			}

			// log.Debug("stats_manager::statsPolling : stats: %v", string(msg))

			// send the stats collected so far to transfer pipe
			stMgrOpt.transferMtx.Lock()
			_, err = fmt.Fprintf(tf, "%v\n", string(msg))
			stMgrOpt.transferMtx.Unlock()
			if err != nil {
				log.Err("stats_manager::statsDumper : Unable to write to pipe [%v]", err)
				disableMonitoring()
				break
			}

			stMgrOpt.cmpTimeMap[cmpSt.ComponentName] = cmpSt.Timestamp
		}
		stMgrOpt.statsMtx.Unlock()
	}
}

func createPipe(pipe string) error {
	stMgrOpt.pollMtx.Lock()
	defer stMgrOpt.pollMtx.Unlock()

	_, err := os.Stat(pipe)
	if os.IsNotExist(err) {
		err = syscall.Mkfifo(pipe, 0666)
		if err != nil {
			log.Err("stats_manager::createPipe : unable to create pipe %v [%v]", pipe, err)
			return err
		}
	} else if err != nil {
		log.Err("stats_manager::createPipe : [%v]", err)
		return err
	}
	return nil
}

