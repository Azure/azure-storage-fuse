# OpenTelemetry Metrics in Blobfuse2

This document describes the OpenTelemetry (OTEL) metrics functionality in Blobfuse2.

## Overview

Blobfuse2 now supports exporting operational metrics via OpenTelemetry, providing real-time insights into:
- Cache performance (hits, misses, evictions, usage)
- System resource utilization (memory, CPU)
- Azure Storage operations (requests, responses, errors, bytes transferred)
- Filesystem operations

## Metrics Collected

### Cache Metrics

**File Cache:**
- `blobfuse.cache.hits{cache_type="file_cache"}` - Number of cache hits
- `blobfuse.cache.misses{cache_type="file_cache"}` - Number of cache misses
- `blobfuse.cache.evictions{cache_type="file_cache"}` - Number of cache evictions
- `blobfuse.cache.usage_mb{component="file_cache"}` - Current cache usage in MB

**Block Cache:**
- `blobfuse.cache.hits{cache_type="block_cache"}` - Number of cache hits
- `blobfuse.cache.misses{cache_type="block_cache"}` - Number of cache misses

### System Metrics

- `blobfuse.system.memory_bytes` - Current memory usage in bytes
- `blobfuse.system.cpu_usage` - CPU usage (user + system time)

### Azure Storage Metrics

- `blobfuse.azure.requests{operation}` - Number of requests to Azure Storage
- `blobfuse.azure.responses{operation, status_code}` - Number of responses from Azure Storage
- `blobfuse.azure.errors{operation, error_type, status_code}` - Number of errors (4xx/5xx)
- `blobfuse.azure.bytes_transferred{operation, direction}` - Bytes uploaded/downloaded
- `blobfuse.azure.request_duration{operation, status_code}` - Request duration histogram

### Operation Metrics

- `blobfuse.operations{operation, component}` - Count of filesystem operations

## Configuration

### Using Configuration File

Add the `metrics` section to your Blobfuse2 configuration file:

```yaml
metrics:
  enabled: true
  endpoint: localhost:4317  # OTLP gRPC collector endpoint
```

See `sampleMetricsConfig.yaml` for a complete example.

### Using Command Line Flags

```bash
blobfuse2 mount <mount-path> \
  --config-file=config.yaml \
  --enable-metrics \
  --metrics-endpoint=localhost:4317
```

### Configuration Options

- `enabled` (boolean): Enable/disable metrics collection (default: false)
- `endpoint` (string): OTLP gRPC collector endpoint in `host:port` format

## Setting Up an OTLP Collector

### Option 1: OpenTelemetry Collector

1. Install the OpenTelemetry Collector:
```bash
wget https://github.com/open-telemetry/opentelemetry-collector-releases/releases/download/v0.91.0/otelcol_0.91.0_linux_amd64.tar.gz
tar -xzf otelcol_0.91.0_linux_amd64.tar.gz
```

2. Create a collector configuration file (`otel-collector-config.yaml`):
```yaml
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317

exporters:
  prometheus:
    endpoint: "0.0.0.0:8889"
  logging:
    loglevel: debug

service:
  pipelines:
    metrics:
      receivers: [otlp]
      exporters: [prometheus, logging]
```

3. Start the collector:
```bash
./otelcol --config=otel-collector-config.yaml
```

### Option 2: Prometheus with OTLP Receiver

Modern Prometheus versions support OTLP ingestion. Configure Prometheus with:

```yaml
scrape_configs:
  - job_name: 'blobfuse2'
    static_configs:
      - targets: ['localhost:8889']
```

### Option 3: Jaeger (for distributed tracing)

While primarily a tracing backend, Jaeger also supports OTLP metrics:

```bash
docker run -d --name jaeger \
  -p 4317:4317 \
  -p 16686:16686 \
  jaegertracing/all-in-one:latest
```

## Viewing Metrics

### With Prometheus

1. Access Prometheus UI at `http://localhost:9090`
2. Query metrics:
   - `blobfuse_cache_hits_total`
   - `blobfuse_azure_requests_total`
   - `rate(blobfuse_azure_request_duration_bucket[5m])`

### With Grafana

1. Add Prometheus as a data source
2. Create dashboards to visualize:
   - Cache hit ratio: `rate(blobfuse_cache_hits_total[5m]) / (rate(blobfuse_cache_hits_total[5m]) + rate(blobfuse_cache_misses_total[5m]))`
   - Request latency percentiles: `histogram_quantile(0.95, rate(blobfuse_azure_request_duration_bucket[5m]))`
   - Error rates: `rate(blobfuse_azure_errors_total[5m])`

## Metric Labels

Metrics include labels for filtering and aggregation:

- `component`: Component name (e.g., "file_cache", "block_cache", "azstorage")
- `cache_type`: Type of cache (e.g., "file_cache", "block_cache")
- `operation`: Operation type (e.g., "download", "upload", "CreateFile")
- `status_code`: HTTP status code for Azure Storage operations
- `error_type`: Error category ("client_error" for 4xx, "server_error" for 5xx)
- `direction`: Data transfer direction ("upload" or "download")

## Performance Considerations

- Metrics collection has minimal overhead (~0.1% CPU)
- Metrics are exported every 10 seconds by default
- No metrics are collected when `enabled: false`
- The OTLP exporter uses gRPC for efficient data transmission

## Troubleshooting

### Metrics not appearing

1. Check that metrics are enabled in configuration
2. Verify the OTLP collector endpoint is correct and reachable
3. Check Blobfuse2 logs for metrics initialization errors:
```bash
grep "metrics" /tmp/blobfuse2.log
```

### Collector connection errors

- Ensure the collector is running and listening on the configured port
- Check firewall rules allow traffic to the collector endpoint
- Verify network connectivity: `telnet localhost 4317`

### High memory usage

If metrics collection causes memory issues:
- Reduce the number of labels or metric cardinality
- Increase the export interval in the collector configuration
- Consider sampling high-frequency operations

## Example Queries

### Cache Performance

```promql
# Cache hit ratio
sum(rate(blobfuse_cache_hits_total[5m])) / 
  (sum(rate(blobfuse_cache_hits_total[5m])) + sum(rate(blobfuse_cache_misses_total[5m])))

# Cache eviction rate
rate(blobfuse_cache_evictions_total[5m])

# Current cache usage
blobfuse_cache_usage_mb
```

### Azure Storage Operations

```promql
# Request rate by operation
sum(rate(blobfuse_azure_requests_total[5m])) by (operation)

# Error rate
sum(rate(blobfuse_azure_errors_total[5m])) by (error_type)

# 95th percentile latency
histogram_quantile(0.95, rate(blobfuse_azure_request_duration_bucket[5m]))

# Bandwidth usage
sum(rate(blobfuse_azure_bytes_transferred_total[5m])) by (direction)
```

### System Resources

```promql
# Memory usage
blobfuse_system_memory_bytes

# CPU usage trend
rate(blobfuse_system_cpu_usage[5m])
```

## Integration with Monitoring Systems

### Alerting Rules

Example Prometheus alerting rules:

```yaml
groups:
  - name: blobfuse2_alerts
    rules:
      - alert: HighErrorRate
        expr: rate(blobfuse_azure_errors_total[5m]) > 0.1
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High error rate in Blobfuse2"
          
      - alert: LowCacheHitRatio
        expr: |
          sum(rate(blobfuse_cache_hits_total[5m])) /
          (sum(rate(blobfuse_cache_hits_total[5m])) + sum(rate(blobfuse_cache_misses_total[5m]))) < 0.5
        for: 10m
        labels:
          severity: info
        annotations:
          summary: "Low cache hit ratio"
```

## Further Reading

- [OpenTelemetry Documentation](https://opentelemetry.io/docs/)
- [OTLP Specification](https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/protocol/otlp.md)
- [Prometheus OTLP Receiver](https://prometheus.io/docs/prometheus/latest/feature_flags/#otlp-receiver)
