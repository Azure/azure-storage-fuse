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

package xload

import (
	"context"

	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal"
)

type XComponent interface {
	Init()
	Start(context.Context)
	Stop()
	Schedule(*WorkItem) error
	Process(*WorkItem) (int, error)
	GetNext() XComponent
	SetNext(XComponent)
	GetThreadPool() *ThreadPool
	SetThreadPool(*ThreadPool)
	GetRemote() internal.Component
	SetRemote(internal.Component)
	GetName() string
	SetName(string)
	GetStatsManager() *StatsManager
	SetStatsManager(*StatsManager)
}

type XBase struct {
	name        string
	pool        *ThreadPool
	remote      internal.Component
	next        XComponent
	statsMgr    *StatsManager
	workerCount uint32
}

var _ XComponent = &XBase{}

func (xb *XBase) Init() {
}

func (xb *XBase) Start(_ context.Context) {
}

func (xb *XBase) Stop() {
}

func (xb *XBase) Schedule(item *WorkItem) error {
	if xb.GetThreadPool() != nil {
		return xb.GetThreadPool().Schedule(item)
	} else {
		// TODO:: xload : check if this call goes to the process method of the calling component
		_, err := xb.Process(item)
		if err != nil {
			log.Err("xcomponent::Schedule : Failed to process for %v [%v]", item.CompName, err.Error())
		}
	}
	return nil
}

func (xb *XBase) Process(item *WorkItem) (int, error) {
	return 0, nil
}

func (xb *XBase) GetNext() XComponent {
	return xb.next
}

func (xb *XBase) SetNext(next XComponent) {
	xb.next = next
}

func (xb *XBase) GetThreadPool() *ThreadPool {
	return xb.pool
}

func (xb *XBase) SetThreadPool(pool *ThreadPool) {
	xb.pool = pool
}

func (xb *XBase) GetRemote() internal.Component {
	return xb.remote
}

func (xb *XBase) SetRemote(comp internal.Component) {
	xb.remote = comp
}

func (xb *XBase) GetName() string {
	return xb.name
}

func (xb *XBase) SetName(name string) {
	xb.name = name
}

func (xb *XBase) GetStatsManager() *StatsManager {
	return xb.statsMgr
}

func (xb *XBase) SetStatsManager(sm *StatsManager) {
	xb.statsMgr = sm
}

func (xb *XBase) GetWorkerCount() uint32 {
	return xb.workerCount
}

func (xb *XBase) SetWorkerCount(workerCount uint32) {
	xb.workerCount = workerCount
}
