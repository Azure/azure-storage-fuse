package stats_manager

const (
	// Stats collection operation types
	Increment = "increment"
	Decrement = "decrement"
	Replace   = "replace"

	// AzStorage stats types
	BytesDownloaded = "Bytes Downloaded"
	BytesUploaded   = "Bytes Uploaded"

	// File Cache stats types
	CacheUsage   = "Cache Usage"
	UsagePercent = "Usage Percent"
)
