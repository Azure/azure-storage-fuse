package network_monitor

import (
	"fmt"

	hmcommon "github.com/Azure/azure-storage-fuse/v2/bin/health-monitor/common"
	hminternal "github.com/Azure/azure-storage-fuse/v2/bin/health-monitor/internal"
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
	fmt.Println("Inside network monitor")
	return nil
}

func (nw *NetworkProfiler) ExportStats() {
	fmt.Println("Inside network export stats")
}

func NewNetworkMonitor() hminternal.Monitor {
	nw := &NetworkProfiler{
		pid:          hmcommon.Pid,
		pollInterval: hmcommon.StatsPollinterval,
	}

	nw.SetName(hmcommon.Network_profiler)

	return nw
}

func init() {
	fmt.Println("Inside network profiler")
	hminternal.AddMonitor(hmcommon.Network_profiler, NewNetworkMonitor)
}
