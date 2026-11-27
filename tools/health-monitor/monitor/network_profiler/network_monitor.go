// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package network_monitor

import (
	"fmt"

	"github.com/Azure/azure-storage-fuse/v2/common/log"
	hmcommon "github.com/Azure/azure-storage-fuse/v2/tools/health-monitor/common"
	hminternal "github.com/Azure/azure-storage-fuse/v2/tools/health-monitor/internal"
)

type NetworkProfiler struct {
	name         string
	pid          string
	pollInterval int
}

func (nw *NetworkProfiler) GetName() string {
	return nw.name
}

func (nw *NetworkProfiler) SetName(name string) {
	nw.name = name
}

func (nw *NetworkProfiler) Monitor() error {
	err := nw.Validate()
	if err != nil {
		log.Err("network_monitor::Monitor : [%v]", err)
		return err
	}

	return nil
}

func (nw *NetworkProfiler) ExportStats(timestamp string, st any) {
	se, err := hminternal.NewStatsExporter()
	if err != nil || se == nil {
		log.Err("network_monitor::ExportStats : Error in creating stats exporter instance [%v]", err)
		return
	}

	se.AddMonitorStats(nw.GetName(), timestamp, st)
}

func (nw *NetworkProfiler) Validate() error {
	if len(nw.pid) == 0 {
		return fmt.Errorf("pid of blobfuse2 is not given")
	}

	if nw.pollInterval == 0 {
		return fmt.Errorf("process-monitor-interval-sec should be non-zero")
	}

	return nil
}

func NewNetworkMonitor() hminternal.Monitor {
	nw := &NetworkProfiler{
		pid:          hmcommon.Pid,
		pollInterval: hmcommon.ProcMonInterval,
	}

	nw.SetName(hmcommon.NetworkProfiler)

	return nw
}

func init() {
	// commenting this for now
	// hminternal.AddMonitor(hmcommon.NetworkProfiler, NewNetworkMonitor)
}
