# Docker Testing Guide for Azure Storage Fuse OTel Logging

This guide provides step-by-step instructions for testing the OpenTelemetry logging integration using Docker containers.

## Prerequisites

- Docker installed (version 20.10+)
- Docker Compose installed (version 1.29+)
- Git and the Azure Storage Fuse repository cloned
- Basic knowledge of Docker and command line

## Quick Start (3 Steps)

### Step 1: Start OTel Services

```bash
# Navigate to the repository root
cd /path/to/azure-storage-fuse

# Make the startup script executable
chmod +x start-otel-collector.sh

# Run the script
./start-otel-collector.sh
```

This will start:
- OpenTelemetry Collector (gRPC: 4317, HTTP: 4318)
- Jaeger UI (http://localhost:16686)
- Prometheus (http://localhost:9090)

### Step 2: Configure Azure Storage Fuse

Edit your Azure Storage Fuse configuration or set environment variables:

```bash
# Set OTel collector endpoint
export OTEL_COLLECTOR_ENDPOINT="localhost:4317"

# Or update setup/otel_config.yaml:
# otel:
#   enabled: true
#   collector_endpoint: "localhost:4317"
```

### Step 3: Run Azure Storage Fuse

```bash
# Build the application
go build -o blobfuse2 ./main.go

# Run with OTel enabled
./blobfuse2 [your-options]
```

## Viewing Logs and Metrics

### View in Jaeger UI

1. Open http://localhost:16686 in your browser
2. Select service "azure-storage-fuse" from the dropdown
3. View distributed traces and logs

### View OTel Collector Logs

```bash
# From repository root
docker-compose -f docker-compose-otel.yml logs -f otel-collector
```

### View Prometheus Metrics

1. Open http://localhost:9090 in your browser
2. View OTel Collector metrics at `localhost:8888/metrics`

### Check OTel Collector Health

```bash
# Check health status
curl http://localhost:13133

# Should return JSON with status information
```

## Common Commands

### Start Services
```bash
./start-otel-collector.sh
```

### Stop Services
```bash
docker-compose -f docker-compose-otel.yml down
```

### View All Logs
```bash
docker-compose -f docker-compose-otel.yml logs -f
```

### View Specific Service Logs
```bash
# OTel Collector logs
docker-compose -f docker-compose-otel.yml logs -f otel-collector

# Jaeger logs
docker-compose -f docker-compose-otel.yml logs -f jaeger

# Prometheus logs
docker-compose -f docker-compose-otel.yml logs -f prometheus
```

### Clean Up (remove volumes)
```bash
docker-compose -f docker-compose-otel.yml down -v
```

## Troubleshooting

### Containers won't start

```bash
# Check Docker daemon
docker ps

# Check logs for errors
docker-compose -f docker-compose-otel.yml logs
```

### Port conflicts

If ports are already in use, edit `docker-compose-otel.yml` to use different ports:

```yaml
otel-collector:
  ports:
    - "4317:4317"  # Change first number to a free port
```

### Connection refused errors

```bash
# Verify services are running
docker-compose -f docker-compose-otel.yml ps

# Restart services
docker-compose -f docker-compose-otel.yml restart
```

### OTel Collector not receiving logs

1. Verify endpoint is correct: `localhost:4317`
2. Check Azure Storage Fuse is running and logging
3. View collector logs for errors:
   ```bash
   docker-compose -f docker-compose-otel.yml logs otel-collector
   ```
4. Ensure firewall isn't blocking port 4317

## Performance Testing

To generate high-volume logs for testing:

```bash
# Run Azure Storage Fuse with debug logging
export LOG_LEVEL=debug
./blobfuse2 [options]

# Monitor batch export
docker-compose -f docker-compose-otel.yml logs -f otel-collector | grep -i batch
```

## Accessing Jaeger UI

**URL**: http://localhost:16686

**Features**:
- Search for traces by service, operation, tags
- View detailed trace timelines
- See distributed tracing information
- Filter by duration, errors, etc.

## Accessing Prometheus

**URL**: http://localhost:9090

**Useful Queries**:
```promql
# OTel Collector uptime
up{job="otel-collector"}

# Logs exported
otelcol_exporter_sent_spans

# Batch processor metrics
otelcol_processor_batch_send_failed_metric_points
```

## Next Steps

1. Test with real Azure Storage Fuse workloads
2. Configure Azure Monitor export for production
3. Set up persistent volumes for long-term metric storage
4. Implement alerting rules in Prometheus

## Support

For issues or questions:
- Check OTEL_LOGGING.md for detailed configuration
- Review docker-compose-otel.yml for service definitions
- Check OTel Collector logs for error messages
- Consult OpenTelemetry documentation
