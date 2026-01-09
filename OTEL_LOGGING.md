# OpenTelemetry Logging Integration for Azure Storage Fuse

This document describes the OpenTelemetry (OTel) logging integration that enables Azure Storage Fuse to export logs directly to an OpenTelemetry collector via gRPC, with periodic batch export to Azure Monitor.

## Overview

The OTel logging integration provides:

- **Direct gRPC Export**: Logs are sent directly from the application to an OpenTelemetry collector via gRPC protocol
- **Batch Processing**: Logs are batched and exported periodically (1-minute intervals by default)
- **Azure Monitor Integration**: Batched logs are periodically exported to Azure Monitor (Application Insights or Log Analytics)
- **Configurable Endpoints**: Support for custom collector endpoints and Azure Monitor endpoints
- **Resource Attributes**: Automatic inclusion of service metadata with logs

## Architecture

```
Azure Storage Fuse
        |
        | (gRPC)
        v
OpenTelemetry Collector (localhost:4317)
        |
        | (Batch Export - 1 min interval)
        v
Azure Monitor / Application Insights
```

## Components

### 1. OTelLogExporter (`common/log/otel_exporter.go`)

Manages the OpenTelemetry log exporter with gRPC connection.

**Key Functions:**
- `NewOTelLogExporter()` - Creates exporter with gRPC connection to collector
- `GetLoggerProvider()` - Returns the OTel logger provider
- `Shutdown()` - Gracefully closes connections and flushes buffered logs
- `IsClosed()` - Checks exporter state
- `GetEndpoint()` - Returns configured collector endpoint

**Features:**
- Thread-safe operations with RWMutex
- 1-minute batch timeout
- 10,000 log queue capacity
- 512 logs per batch

### 2. Azure Monitor Exporter (`common/log/azure_monitor_exporter.go`)

Bridges between OTel collector and Azure Monitor for periodic log export.

**Key Components:**
- `AzureMonitorExporterConfig` - Configuration struct
- `ConfigureAzureMonitorExport()` - Setup function
- `AzureMonitorLogsBridge` - Destination for batched logs

### 3. Configuration (`setup/otel_config.yaml`)

YAML configuration file for OTel settings.

## Setup Instructions

### Prerequisites

1. OpenTelemetry Collector running (default: localhost:4317)
2. Azure Account with Application Insights or Log Analytics workspace
3. Instrumentation Key from Application Insights

### 1. Run OpenTelemetry Collector Locally

Create `docker-compose.yml`:

```yaml
version: '3'
services:
  otel-collector:
    image: otel/opentelemetry-collector-contrib:latest
    ports:
      - "4317:4317"  # gRPC receiver
      - "4318:4318"  # HTTP receiver
    volumes:
      - ./otel-collector-config.yaml:/etc/otel/config.yaml
    command: ["--config=/etc/otel/config.yaml"]
```

Create `otel-collector-config.yaml`:

```yaml
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317
      http:
        endpoint: 0.0.0.0:4318

exporters:
  logging:
    loglevel: debug
  # Add Azure Monitor exporter here

service:
  pipelines:
    logs:
      receivers: [otlp]
      exporters: [logging]
```

Start the collector:
```bash
docker-compose up -d
```

### 2. Configure Azure Storage Fuse

Edit `setup/otel_config.yaml`:

```yaml
otel:
  enabled: true
  collector_endpoint: "localhost:4317"  # or your collector address
  batch:
    timeout_seconds: 60  # 1-minute batch interval
    max_queue_size: 10000
    max_batch_size: 512
  azure_monitor:
    instrumentation_key: "your-instrumentation-key"
    service_name: "azure-storage-fuse"
    service_version: "2.5.1"
```

### 3. Set Environment Variables

```bash
export AZURE_INSTRUMENTATION_KEY="your-instrumentation-key"
export OTEL_COLLECTOR_ENDPOINT="localhost:4317"
export LOG_LEVEL="info"
```

### 4. Initialize in Application Code

In `main.go`:

```go
import "github.com/Azure/azure-storage-fuse/v2/common/log"

func main() {
    // Initialize OTel exporter
    otelExp, err := log.NewOTelLogExporter(
        context.Background(),
        "localhost:4317",
        "azure-storage-fuse",
    )
    if err != nil {
        // handle error
    }
    defer otelExp.Shutdown(context.Background())
    
    // Initialize Azure Monitor export
    cleanup, err := log.ConfigureAzureMonitorExport(
        context.Background(),
        log.AzureMonitorExporterConfig{
            InstrumentationKey: os.Getenv("AZURE_INSTRUMENTATION_KEY"),
            ServiceName: "azure-storage-fuse",
            ServiceVersion: "2.5.1",
        },
    )
    if err != nil {
        // handle error
    }
    defer cleanup(context.Background())
    
    // Rest of application
}
```

## Batch Export Details

- **Timeout**: 60 seconds (1 minute)
- **Max Queue Size**: 10,000 log records
- **Max Batch Size**: 512 logs per export
- **Behavior**: Logs are exported when timeout expires OR queue reaches max size (whichever comes first)

## Testing

### 1. Verify Collector is Receiving Logs

```bash
docker-compose logs otel-collector
```

### 2. Check Azure Monitor

In Application Insights:
```kusto
traces
| where cloud_RoleName == "azure-storage-fuse"
| order by timestamp desc
| limit 100
```

## Troubleshooting

### Logs not appearing in Azure Monitor

1. Verify OTel collector is running: `curl http://localhost:13133/`
2. Check instrumentation key is set correctly
3. Verify network connectivity to Azure Monitor endpoint
4. Check logs in OTel collector: `docker-compose logs otel-collector`

### Connection refused errors

- Ensure OTel collector is running on expected endpoint
- Check firewall/network settings
- Verify endpoint configuration in `otel_config.yaml`

## Performance Considerations

- Batch processing reduces network overhead
- 1-minute interval balances latency vs. efficiency
- Adjust `max_queue_size` and `timeout_seconds` based on log volume
- Consider using sampling for high-volume scenarios

## References

- [OpenTelemetry Go Logs](https://opentelemetry.io/docs/instrumentation/go/manual/logs/)
- [OpenTelemetry Collector](https://opentelemetry.io/docs/collector/)
- [Azure Monitor Integration](https://learn.microsoft.com/en-us/azure/azure-monitor/)

## Future Enhancements

- [ ] Support for trace context propagation
- [ ] Log sampling and filtering
- [ ] Custom resource attributes
- [ ] Multi-endpoint export
- [ ] Metrics collection integration
