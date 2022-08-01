package network_monitor

import (
	"fmt"

	hmcommon "github.com/Azure/azure-storage-fuse/v2/bin/health-monitor/common"
	hminternal "github.com/Azure/azure-storage-fuse/v2/bin/health-monitor/internal"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
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

func (nw *NetworkProfiler) ExportStats() {
	fmt.Println("Inside network export stats")
}

func (nw *NetworkProfiler) Validate() error {
	if len(nw.pid) == 0 {
		return fmt.Errorf("pid of blobfuse2 is not given")
	}

	if nw.pollInterval == 0 {
		return fmt.Errorf("stats-poll-interval should be non-zero")
	}

	return nil
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
	hminternal.AddMonitor(hmcommon.Network_profiler, NewNetworkMonitor)
}
