package azstorage

// PolicyMetric defines the standard structure for metrics sent by the HTTP policy
type PolicyMetric struct {
	RequestCount int64  `json:"request_count"`
	FailureCount int64  `json:"failure_count"`
	DurationMs   int64  `json:"duration_ms"`
	Timestamp    string `json:"timestamp"`
}

type ExportedStat struct {
	Timestamp   string
	MonitorName string
	Stat        interface{}
}

// StatsExporter defines the interface any exporter must implement
type StatsExporter interface {
	// AddMonitorStats sends a metric to the exporter (e.g., via a channel)
	AddMonitorStats(policyName string, timestamp string, stat interface{})
}

var registeredExporter StatsExporter

// RegisterExporter allows external packages (like `internal`) to register their exporter
func RegisterExporter(e StatsExporter) {
	registeredExporter = e
}

// GetRegisteredExporter returns the globally registered exporter instance
func GetRegisteredExporter() StatsExporter {
	return registeredExporter
}
