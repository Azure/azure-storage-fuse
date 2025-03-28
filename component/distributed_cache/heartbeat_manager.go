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
package distributed_cache

import (
	"encoding/json"
	"syscall"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal"
)

type HeartbeatManager struct {
	comp         internal.Component
	cachePath    string
	hbDuration   uint16
	hbPath       string
	maxCacheSize uint64
	maxMissedHbs uint8
	nodeId       string
	ticker       *time.Ticker
}

type HeartbeatData struct {
	IPAddr         string `json:"ipaddr"`
	NodeID         string `json:"nodeid"`
	Hostname       string `json:"hostname"`
	LastHeartbeat  uint64 `json:"last_heartbeat"`
	TotalSpaceByte uint64 `json:"total_space_byte"`
	UsedSpaceByte  uint64 `json:"used_space_byte"`
}

func (hm *HeartbeatManager) Start() {
	hm.ticker = time.NewTicker(time.Duration(hm.hbDuration) * time.Second)
	go func() {
		for range hm.ticker.C {
			log.Trace("Scheduled task triggered")
			hm.Starthb()
			hm.StartDiscovery()
		}
	}()
}

func (hm *HeartbeatManager) stopScehduler() {
	if hm.ticker != nil {
		hm.ticker.Stop()
		hm.ticker = nil
	}
}

func (hm *HeartbeatManager) Starthb() error {
	uuidVal, err := common.GetUUID()
	if err != nil {
		log.Err("AddHeartBeat: Failed to retrieve UUID, error: %v", err)
		return err
	}
	hm.nodeId = uuidVal

	hbPath := hm.hbPath + "/Nodes/" + hm.nodeId + ".hb"
	ipaddr, err := getVmIp()
	if err != nil {
		log.Err("AddHeartBeat: Failed to get VM IP")
		return err
	}
	totalSpace, used_space, err := evaluateVMStorage(hm.cachePath)
	if err != nil {
		log.Err("AddHeartBeat: Failed to evaluate VM storage: ", err)
		return err
	}
	hostname, _ := common.GetHostName()
	totalSpace = func() uint64 {
		if hm.maxCacheSize != 0 {
			return hm.maxCacheSize
		}
		return totalSpace
	}()
	hbData := HeartbeatData{
		IPAddr:         ipaddr,
		NodeID:         hm.nodeId,
		Hostname:       hostname,
		LastHeartbeat:  uint64(time.Now().Unix()),
		TotalSpaceByte: totalSpace,
		UsedSpaceByte:  used_space,
	}

	// Marshal the data into JSON
	data, err := json.MarshalIndent(hbData, "", "  ")
	if err != nil {
		log.Err("AddHeartBeat: Failed to marshal heartbeat data")
		return err
	}

	// Create a heartbeat file in storage with <nodeId>.hb
	if err := hm.comp.WriteFromBuffer(internal.WriteFromBufferOptions{Name: hbPath, Data: data}); err != nil {
		log.Err("AddHeartBeat: Failed to write heartbeat file: ", err)
		return err
	}
	log.Trace("AddHeartBeat: Heartbeat file updated successfully")
	return nil
}

func (hm *HeartbeatManager) Stop() error {
	hm.stopScehduler()
	hbPath := hm.hbPath + "/Nodes/" + hm.nodeId + ".hb"
	err := hm.comp.DeleteFile(internal.DeleteFileOptions{Name: hbPath})
	if err != nil {
		log.Err("HeartbeatManager::Stop Failed to delete heartbeat file: ", err)
		return err
	}
	return nil
}

var PeersByNodeId map[string]*Peer = make(map[string]*Peer)
var PeersByName map[string]*Peer = make(map[string]*Peer)

func (hm *HeartbeatManager) StartDiscovery() {
	attrs, err := hm.comp.ReadDir(internal.ReadDirOptions{Name: hm.hbPath + "/Nodes/"})
	if err != nil {
		log.Err("HeartbeatManager::StartDiscovery: Failed to read Cache Node directory: %v", err)
		return
	}
	for _, attr := range attrs {
		log.Info(attr.Name)
		data, err := hm.comp.ReadFileWithName(internal.ReadFileWithNameOptions{
			Path: attr.Path,
		})
		if err != nil {
			maxRetry := 3
			counter := 4
			if err == syscall.ENOENT {
				peer := PeersByNodeId[attr.Path]
				delete(PeersByNodeId, attr.Path)
				delete(PeersByName, peer.NodeID)
				peer = nil
				continue
			} else if maxRetry > counter {
				for retryForMaxNumber() != nil && maxRetry > counter {

					counter++
				}
			}
			log.Err("HeartbeatManager::StartDiscovery: Failed to read Cache Node directory: %v", err)
			continue
		}

		peer := Peer{}
		if err := json.Unmarshal(data, &peer); err != nil {
			continue
		}

		allowableMissHB := time.Now().Unix() - int64(int(hm.hbDuration)*int(hm.maxMissedHbs))
		// If the heartbeat is older than some threshold, remove
		if int64(peer.LastHeartbeat) < allowableMissHB {
			err = hm.comp.DeleteFile(internal.DeleteFileOptions{Name: attr.Path})
			log.Err("HeartbeatManager::StartDiscovery: Failed to delete Node heartbeat: %v", err)
		} else {
			PeersByNodeId[peer.NodeID] = &peer
			PeersByName[attr.Path] = &peer
		}

	}
}

func retryForMaxNumber() error {
	return nil
}
