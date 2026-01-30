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
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"golang.org/x/sys/windows"
)

func (sc *StatsCollector) statsDumper() {
	defer sc.workerDone.Done()

	var tPipe windows.Handle
	var err error
	for {
		// To see documentation for the arguments for this function see
		// https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-createfilea
		tPipe, err = windows.CreateFile(
			windows.StringToUTF16Ptr(common.TransferPipe),
			windows.GENERIC_WRITE,
			windows.FILE_SHARE_WRITE,
			nil,
			windows.OPEN_EXISTING,
			windows.FILE_ATTRIBUTE_NORMAL,
			windows.InvalidHandle,
		)

		if err == nil {
			break
		}
		windows.Close(tPipe)

		if err == windows.ERROR_FILE_NOT_FOUND {
			log.Info("stats_manager::statsDumper : Named pipe %s not found, retrying...", common.TransferPipe)
			time.Sleep(1 * time.Second)
		} else if err == windows.ERROR_PIPE_BUSY {
			log.Err("stats_manager::statsDumper: Pipe instances are busy, retrying...")
			time.Sleep(1 * time.Second)
		} else {
			log.Err("stats_manager::statsDumper: Unable to open pipe %s with error [%v]", common.TransferPipe, err)
			return
		}
	}

	log.Info("stats_manager::statsDumper : opened transfer pipe file")
	defer windows.Close(tPipe)

	// The channel is used for two types of messages. First, if the message is an event
	// then we send the message to the transfer pipe. If it is not an event, then it is
	// a message about the changing of given values such as incrementing or decrementing
	// the number of open file handles.
	for st := range sc.channel {
		log.Debug("stats_manager::statsDumper : stats from channel: %v", st)

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
			err = windows.WriteFile(tPipe, msg, nil, nil)
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
	handle, err := windows.CreateNamedPipe(
		windows.StringToUTF16Ptr(common.PollingPipe),
		windows.PIPE_ACCESS_DUPLEX,
		windows.PIPE_TYPE_MESSAGE|windows.PIPE_READMODE_MESSAGE|windows.PIPE_WAIT,
		windows.PIPE_UNLIMITED_INSTANCES,
		4096,
		4096,
		0,
		nil,
	)
	if err != nil {
		log.Err("stats_manager::statsPolling : unable to create pipe [%v]", err)
		return
	}
	defer windows.Close(handle)

	log.Info("stats_manager::statsPolling : Creating named pipe %s", common.PollingPipe)

	// This is a blocking call that waits for a client instance to call the CreateFile function and once that
	// happens then we can safely start writing to the named pipe.
	// See https://learn.microsoft.com/en-us/windows/win32/api/namedpipeapi/nf-namedpipeapi-connectnamedpipe
	err = windows.ConnectNamedPipe(handle, nil)
	if err != nil {
		log.Err("stats_manager::statsPolling : unable to connect to named pipe %s: [%v]", common.PollingPipe, err)
		return
	}
	log.Info("StatsReader::statsReader : Connected polling pipe")

	// Setup transfer pipe by looping to try to create a file (open the pipe).
	// If the server has not been setup yet, then this will fail so we wait
	// and then try again.
	var tPipe windows.Handle
	for {
		// To see documentation for the arguments for this function see
		// https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-createfilea
		tPipe, err = windows.CreateFile(
			windows.StringToUTF16Ptr(common.TransferPipe),
			windows.GENERIC_WRITE,
			windows.FILE_SHARE_WRITE,
			nil,
			windows.OPEN_EXISTING,
			windows.FILE_ATTRIBUTE_NORMAL,
			windows.InvalidHandle,
		)

		// The pipe was created
		if err == nil {
			break
		}

		windows.Close(tPipe)
		if err == windows.ERROR_FILE_NOT_FOUND {
			log.Info("stats_manager::statsPolling : Named pipe %s not found, retrying...", common.TransferPipe)
			time.Sleep(1 * time.Second)
		} else if err == windows.ERROR_PIPE_BUSY {
			log.Err("stats_manager::statsPolling: Pipe instances are busy, retrying...")
			time.Sleep(1 * time.Second)
		} else {
			log.Err("stats_manager::statsPolling: Unable to open pipe %s with error [%v]", common.TransferPipe, err)
			return
		}
	}
	defer windows.Close(tPipe)

	var buf [4096]byte
	var bytesRead uint32
	var messageBuf bytes.Buffer

	for {
		// Empty the buffer before reading
		messageBuf.Reset()
		// read the polling message sent by stats monitor
		for {
			err := windows.ReadFile(handle, buf[:], &bytesRead, nil)

			if err != nil && err != windows.ERROR_MORE_DATA {
				log.Err("stats_manager::statsPolling : Unable to read from pipe [%v]", err)
				disableMonitoring()
				return
			}

			messageBuf.Write(buf[:bytesRead])

			if err != windows.ERROR_MORE_DATA {
				break
			}
		}

		message := messageBuf.String()
		log.Debug("stats_manager::statsPolling : Received message to polling pipe %v", message)

		// validating poll message
		if !strings.Contains(string(message), "Poll at") {
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

			log.Debug("stats_manager::statsPolling : stats: %v", string(msg))

			// send the stats collected so far to transfer pipe
			stMgrOpt.transferMtx.Lock()
			err = windows.WriteFile(tPipe, []byte(fmt.Sprintf("%v\n", string(msg))), nil, nil)
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