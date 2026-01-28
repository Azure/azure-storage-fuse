# OpenTelemetry Integration for Blobfuse2

This guide explains how to configure Blobfuse2 to send logs to Azure Monitor using OpenTelemetry.

## Overview

Blobfuse2 now supports OpenTelemetry (OTel) for logging, enabling you to send structured logs to Azure Monitor, Application Insights, or any OpenTelemetry-compatible backend. This integration uses the OpenTelemetry Protocol (OTLP) to export logs.

## Architecture

```
┌─────────────┐         OTLP/HTTP          ┌──────────────────┐
│             │  ────────────────────────>  │  OpenTelemetry   │
│  Blobfuse2  │  (logs with attributes)     │    Collector     │
│             │                              │                  │
└─────────────┘                              └──────────────────┘
                                                      │
                                                      │ Azure Monitor
                                                      │ Exporter
                                                      ▼
                                             ┌──────────────────┐
                                             │  Azure Monitor/  │
                                             │  Application     │
                                             │    Insights      │
                                             └──────────────────┘
```

**Key Components:**
1. **Blobfuse2**: Emits structured logs via OTLP
2. **OpenTelemetry Collector**: Receives logs and forwards to Azure Monitor
3. **Azure Monitor/Application Insights**: Stores and visualizes logs

## Prerequisites

1. **Azure Application Insights** resource (or any OpenTelemetry-compatible backend)
2. **OpenTelemetry Collector** with Azure Monitor exporter configured
3. **Go 1.24.4+** (for building Blobfuse2 with OpenTelemetry support)

## Configuration

### 1. Blobfuse2 Configuration

Update your Blobfuse2 configuration file to use OpenTelemetry logging:

```yaml
logging:
  # Set type to 'otel' to enable OpenTelemetry logging
  type: otel
  
  # Log level: log_off, log_crit, log_err, log_warning, log_info, log_trace, log_debug
  level: log_info
  
  # OTLP endpoint (optional - defaults to OTEL_EXPORTER_OTLP_ENDPOINT env var)
  # For local collector, use: localhost:4318
  otel-endpoint: localhost:4318
  
  # Optional: Enable goroutine ID tracking for debugging
  goroutine-id: false
  
  # Optional: Enable performance time tracking
  track-time: false
```

**Configuration Options:**
- `type`: Set to `otel` to enable OpenTelemetry logging
- `level`: Log level (log_off, log_crit, log_err, log_warning, log_info, log_trace, log_debug)
- `otel-endpoint`: OTLP HTTP endpoint (e.g., `localhost:4318`)
  - If omitted, uses `OTEL_EXPORTER_OTLP_ENDPOINT` environment variable
- `goroutine-id`: Include goroutine ID in log attributes (useful for debugging)
- `track-time`: Enable time tracking for performance monitoring

### 2. OpenTelemetry Collector Configuration

Create an OpenTelemetry Collector configuration file (`otel-collector-config.yaml`):

```yaml
receivers:
  otlp:
    protocols:
      http:
        endpoint: 0.0.0.0:4318
      grpc:
        endpoint: 0.0.0.0:4317

processors:
  batch:
    timeout: 10s
    send_batch_size: 1024

exporters:
  azuremonitor:
    connection_string: "${APPLICATIONINSIGHTS_CONNECTION_STRING}"
    
  # Optional: Also export to console for debugging
  logging:
    loglevel: debug

service:
  pipelines:
    logs:
      receivers: [otlp]
      processors: [batch]
      exporters: [azuremonitor]
    
    # Uncomment to also see logs in collector output
    # logs:
    #   receivers: [otlp]
    #   processors: [batch]
    #   exporters: [azuremonitor, logging]
```

### 3. Running the OpenTelemetry Collector

#### Using Docker

```bash
# Set your Application Insights connection string
export APPLICATIONINSIGHTS_CONNECTION_STRING="InstrumentationKey=<your-key>;IngestionEndpoint=https://....applicationinsights.azure.com/"

# Run the OpenTelemetry Collector
docker run -d \
  --name otel-collector \
  -p 4317:4317 \
  -p 4318:4318 \
  -e APPLICATIONINSIGHTS_CONNECTION_STRING="${APPLICATIONINSIGHTS_CONNECTION_STRING}" \
  -v $(pwd)/otel-collector-config.yaml:/etc/otel-collector-config.yaml \
  otel/opentelemetry-collector-contrib:latest \
  --config=/etc/otel-collector-config.yaml
```

#### Using Binary

Download the OpenTelemetry Collector from [GitHub releases](https://github.com/open-telemetry/opentelemetry-collector-contrib/releases):

```bash
# Download and extract
wget https://github.com/open-telemetry/opentelemetry-collector-releases/releases/download/v0.96.0/otelcol-contrib_0.96.0_linux_amd64.tar.gz
tar -xzf otelcol-contrib_0.96.0_linux_amd64.tar.gz

# Set connection string
export APPLICATIONINSIGHTS_CONNECTION_STRING="InstrumentationKey=<your-key>;..."

# Run collector
./otelcol-contrib --config=otel-collector-config.yaml
```

## Getting Your Application Insights Connection String

1. Go to [Azure Portal](https://portal.azure.com)
2. Navigate to your Application Insights resource
3. Click on "Overview" or "Properties"
4. Copy the **Connection String** (not the Instrumentation Key)

The connection string format:
```
InstrumentationKey=<guid>;IngestionEndpoint=https://<region>.applicationinsights.azure.com/;LiveEndpoint=https://<region>.livediagnostics.monitor.azure.com/
```

## Running Blobfuse2 with OpenTelemetry

### Using Configuration File

```bash
# Mount with OpenTelemetry logging
blobfuse2 mount <mount-path> --config-file=config.yaml
```

### Using Environment Variables

You can also configure the OTLP endpoint via environment variable:

```bash
# Set the OTLP endpoint
export OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318

# Mount (otel-endpoint will use the environment variable if not set in config)
blobfuse2 mount <mount-path> --config-file=config.yaml
```

### Command Line Override

```bash
# Override logging type via CLI
blobfuse2 mount <mount-path> \
  --config-file=config.yaml \
  --log-type=otel \
  --log-level=LOG_DEBUG
```

## Log Attributes

Blobfuse2 enriches logs with the following attributes when using OpenTelemetry:

| Attribute | Description | Example |
|-----------|-------------|---------|
| `level` | Log level | "LOG_INFO", "LOG_ERR" |
| `source.file` | Source file name | "file_cache.go" |
| `source.line` | Line number | 123 |
| `pid` | Process ID | 12345 |
| `tag` | Application tag | "blobfuse2" |
| `mount_path` | Mount path | "/mnt/blobfuse" |
| `goroutine_id` | Goroutine ID (if enabled) | 42 |

## Querying Logs in Azure Monitor

Once logs are flowing to Azure Monitor, you can query them using Kusto Query Language (KQL):

### Basic Log Query

```kql
traces
| where timestamp > ago(1h)
| where customDimensions.tag == "blobfuse2"
| project timestamp, message, severityLevel, customDimensions
| order by timestamp desc
```

### Filter by Log Level

```kql
traces
| where timestamp > ago(1h)
| where customDimensions.tag == "blobfuse2"
| where customDimensions.level == "LOG_ERR"
| project timestamp, message, customDimensions.["source.file"], customDimensions.["source.line"]
| order by timestamp desc
```

### Analyze by Mount Path

```kql
traces
| where timestamp > ago(24h)
| where customDimensions.tag == "blobfuse2"
| summarize count() by tostring(customDimensions.mount_path), tostring(customDimensions.level)
| order by count_ desc
```

### Monitor Errors Over Time

```kql
traces
| where timestamp > ago(24h)
| where customDimensions.tag == "blobfuse2"
| where customDimensions.level in ("LOG_ERR", "LOG_CRIT")
| summarize count() by bin(timestamp, 1h)
| render timechart
```

## Testing the Integration

### 1. Start the OpenTelemetry Collector

```bash
# Terminal 1: Start collector
docker run -d \
  --name otel-collector \
  -p 4317:4317 \
  -p 4318:4318 \
  -e APPLICATIONINSIGHTS_CONNECTION_STRING="${APPLICATIONINSIGHTS_CONNECTION_STRING}" \
  -v $(pwd)/otel-collector-config.yaml:/etc/otel-collector-config.yaml \
  otel/opentelemetry-collector-contrib:latest \
  --config=/etc/otel-collector-config.yaml

# Verify collector is running
docker logs otel-collector
```

### 2. Mount Blobfuse2 with OpenTelemetry Logging

```bash
# Terminal 2: Mount with OpenTelemetry
blobfuse2 mount /mnt/blobfuse \
  --config-file=sampleOtelConfig.yaml \
  --foreground
```

### 3. Generate Test Logs

```bash
# Terminal 3: Generate activity
ls /mnt/blobfuse
echo "test" > /mnt/blobfuse/test.txt
cat /mnt/blobfuse/test.txt
rm /mnt/blobfuse/test.txt
```

### 4. Verify Logs in Azure Monitor

1. Go to your Application Insights resource in Azure Portal
2. Navigate to "Logs"
3. Run the basic query:
   ```kql
   traces
   | where timestamp > ago(5m)
   | where customDimensions.tag == "blobfuse2"
   | order by timestamp desc
   ```

### 5. Verify Logs Locally (Optional)

To see logs locally without Azure Monitor, modify the collector config to include console logging:

```yaml
exporters:
  logging:
    loglevel: debug

service:
  pipelines:
    logs:
      receivers: [otlp]
      processors: [batch]
      exporters: [logging]
```

Then check the collector logs:
```bash
docker logs -f otel-collector
```

## Troubleshooting

### Logs Not Appearing in Azure Monitor

1. **Check Collector Status**
   ```bash
   docker logs otel-collector
   ```
   Look for errors related to Azure Monitor connection.

2. **Verify Connection String**
   Ensure `APPLICATIONINSIGHTS_CONNECTION_STRING` is set correctly with the full connection string (not just the instrumentation key).

3. **Check Network Connectivity**
   ```bash
   # Test if collector is reachable from Blobfuse2
   curl -v http://localhost:4318/v1/logs
   ```

4. **Enable Debug Logging in Collector**
   Set `loglevel: debug` in the collector's logging exporter to see detailed information.

### Blobfuse2 Connection Errors

If Blobfuse2 fails to connect to the collector:

1. **Verify Endpoint Configuration**
   ```yaml
   logging:
     otel-endpoint: localhost:4318  # Correct format
   ```

2. **Check Collector is Running**
   ```bash
   netstat -tulpn | grep 4318
   ```

3. **Try with Environment Variable**
   ```bash
   export OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318
   ```

### Performance Impact

OpenTelemetry logging uses batching to minimize performance impact:
- Logs are sent asynchronously in batches
- Default batch size: 1024 logs
- Default timeout: 10 seconds

To adjust batching parameters, modify the collector config:
```yaml
processors:
  batch:
    timeout: 5s
    send_batch_size: 512
```

## Alternative Backends

While this guide focuses on Azure Monitor, you can use OpenTelemetry with other backends:

### Jaeger (for traces and logs)
```yaml
exporters:
  jaeger:
    endpoint: localhost:14250
```

### Prometheus (for metrics)
```yaml
exporters:
  prometheus:
    endpoint: "0.0.0.0:8889"
```

### OTLP (generic endpoint)
```yaml
exporters:
  otlp:
    endpoint: https://your-backend:4317
```

## Best Practices

1. **Use Appropriate Log Levels**
   - Production: `log_warning` or `log_err`
   - Development: `log_info` or `log_debug`

2. **Monitor Collector Health**
   - Set up alerts for collector downtime
   - Monitor collector resource usage

3. **Batch Configuration**
   - Adjust batch size based on log volume
   - Higher volumes may benefit from larger batches

4. **Security**
   - Use HTTPS for production deployments
   - Secure connection strings using Azure Key Vault
   - Restrict network access to collector

5. **Cost Management**
   - Monitor Application Insights data ingestion
   - Use appropriate log levels to control volume
   - Consider sampling for high-volume scenarios

## Reference

- [OpenTelemetry Go SDK](https://opentelemetry.io/docs/languages/go/)
- [Azure Monitor OpenTelemetry](https://learn.microsoft.com/en-us/azure/azure-monitor/app/opentelemetry)
- [OpenTelemetry Collector](https://opentelemetry.io/docs/collector/)
- [Application Insights Connection Strings](https://learn.microsoft.com/en-us/azure/azure-monitor/app/sdk-connection-string)

## Support

For issues or questions:
- GitHub Issues: https://github.com/Azure/azure-storage-fuse/issues
- Email: blobfusedev@microsoft.com
