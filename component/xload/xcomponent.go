// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

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
