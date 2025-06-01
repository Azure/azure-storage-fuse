package metrics

import (
	"sync/atomic"
)

// ExportedStat represents the exported stat structure
type ExportedStat struct {
	Timestamp   string
	MonitorName string
	Stat        interface{}
}

var (
	exportCh  chan ExportedStat
	pidStatus int32 = 0 // optionally make public or add getter/setter
)

// InitChannel allows StatsExporter to inject the shared channel
func InitChannel(ch chan ExportedStat) {
	exportCh = ch
}

// SetPIDStatus allows managing process status externally if needed
func SetPIDStatus(status int32) {
	atomic.StoreInt32(&pidStatus, status)
}

// AddMonitorStats adds a metric to the channel for exporter
func AddMonitorStats(monName, timestamp string, st interface{}) {
	if exportCh == nil {
		return
	}

	if len(exportCh) == cap(exportCh) {
		<-exportCh // remove oldest
	}

	if atomic.LoadInt32(&pidStatus) == 0 {
		exportCh <- ExportedStat{
			Timestamp:   timestamp,
			MonitorName: monName,
			Stat:        st,
		}
	}
}
