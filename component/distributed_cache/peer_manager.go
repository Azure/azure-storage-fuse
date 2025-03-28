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

	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal"
	"github.com/Azure/azure-storage-fuse/v2/internal/handlemap"
)

type PeerManager struct {
	comp      internal.Component
	cachePath string
	hbPath    string
	nodeId    string
}

var Peers map[string]*Peer = make(map[string]*Peer)

type Peer struct {
	IPAddr        string
	NodeID        string
	Hostname      string
	TotalSpace    uint64
	UsedSpace     uint64
	LastHeartbeat int64
}

func (pm *PeerManager) StartDiscovery() {
	attrs, err := pm.comp.ReadDir(internal.ReadDirOptions{Name: pm.hbPath + "/Nodes/"})
	if err != nil {
		return
	}
	for _, attr := range attrs {
		log.Info(attr.Name)
		data, err := pm.comp.ReadFile(internal.ReadFileOptions{
			Handle: handlemap.NewHandle(attr.Path),
		})
		if err != nil {
			continue
		}

		peer := Peer{}
		if err := json.Unmarshal(data, &peer); err != nil {
			continue
		}

		Peers[peer.NodeID] = &peer
	}
}
