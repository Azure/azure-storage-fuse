# Quick Test Script for OpenTelemetry Integration

This script provides a quick way to test the OpenTelemetry integration with Blobfuse2.

## Prerequisites

1. Docker installed
2. Azure Application Insights connection string
3. Blobfuse2 built with OpenTelemetry support

## Step 1: Create OpenTelemetry Collector Configuration

Save this as `otel-collector-test.yaml`:

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
  # Export to Azure Monitor
  azuremonitor:
    connection_string: "${APPLICATIONINSIGHTS_CONNECTION_STRING}"
  
  # Also log to console for immediate verification
  logging:
    loglevel: debug

service:
  pipelines:
    logs:
      receivers: [otlp]
      processors: [batch]
      exporters: [azuremonitor, logging]
```

## Step 2: Start OpenTelemetry Collector

```bash
# Set your Application Insights connection string
export APPLICATIONINSIGHTS_CONNECTION_STRING="InstrumentationKey=xxx;IngestionEndpoint=https://xxx.applicationinsights.azure.com/"

# Start collector
docker run -d \
  --name otel-collector-test \
  -p 4317:4317 \
  -p 4318:4318 \
  -e APPLICATIONINSIGHTS_CONNECTION_STRING="${APPLICATIONINSIGHTS_CONNECTION_STRING}" \
  -v $(pwd)/otel-collector-test.yaml:/etc/otel-collector-config.yaml \
  otel/opentelemetry-collector-contrib:latest \
  --config=/etc/otel-collector-config.yaml

# Verify it's running
docker logs otel-collector-test
```

## Step 3: Create Test Configuration for Blobfuse2

Save this as `test-otel-config.yaml`:

```yaml
logging:
  type: otel
  level: log_debug
  otel-endpoint: localhost:4318
  goroutine-id: true

# Minimal mount config for testing
components:
  - libfuse
  - file_cache
  - attr_cache
  - azstorage

libfuse:
  attribute-expiration-sec: 120
  entry-expiration-sec: 120

file_cache:
  path: /tmp/blobfuse-cache
  timeout-sec: 120
  max-size-mb: 1024

attr_cache:
  timeout-sec: 300

azstorage:
  type: block
  account-name: <YOUR_ACCOUNT_NAME>
  account-key: <YOUR_ACCOUNT_KEY>
  container: <YOUR_CONTAINER>
  mode: key
```

## Step 4: Test Blobfuse2 with OpenTelemetry

```bash
# Create mount point
mkdir -p /tmp/blobfuse-mount
mkdir -p /tmp/blobfuse-cache

# Mount with OpenTelemetry logging (foreground for testing)
./blobfuse2 mount /tmp/blobfuse-mount \
  --config-file=test-otel-config.yaml \
  --foreground

# In another terminal, generate test activity
ls /tmp/blobfuse-mount/
echo "test data" > /tmp/blobfuse-mount/test-file.txt
cat /tmp/blobfuse-mount/test-file.txt
rm /tmp/blobfuse-mount/test-file.txt
```

## Step 5: Verify Logs

### Check Collector Logs
```bash
docker logs -f otel-collector-test
```

You should see log entries being received and exported.

### Check Azure Monitor

1. Go to Azure Portal
2. Navigate to your Application Insights resource
3. Go to "Logs"
4. Run this query:

```kql
traces
| where timestamp > ago(10m)
| where customDimensions.tag == "blobfuse2"
| project timestamp, message, severityLevel, customDimensions
| order by timestamp desc
```

## Step 6: Cleanup

```bash
# Unmount
fusermount -u /tmp/blobfuse-mount

# Stop collector
docker stop otel-collector-test
docker rm otel-collector-test

# Clean up test directories
rm -rf /tmp/blobfuse-mount /tmp/blobfuse-cache
```

## Expected Results

You should see:
1. Blobfuse2 logs appearing in stdout (console)
2. Logs appearing in the collector output (docker logs)
3. Logs appearing in Azure Monitor within 1-2 minutes

## Troubleshooting

### Logs not appearing in collector

```bash
# Check if collector is reachable
curl -v http://localhost:4318/v1/logs

# Check collector logs for errors
docker logs otel-collector-test | grep -i error
```

### Logs not appearing in Azure Monitor

1. Wait 2-3 minutes for ingestion
2. Verify connection string is correct
3. Check collector logs for Azure Monitor export errors
4. Verify Application Insights resource has data ingestion enabled

### Blobfuse2 fails to start

```bash
# Try with base logger first to verify mount works
./blobfuse2 mount /tmp/blobfuse-mount \
  --config-file=test-otel-config.yaml \
  --log-type=base \
  --log-file-path=/tmp/blobfuse.log \
  --foreground
```

## Performance Testing

To test performance impact:

```bash
# Mount with base logger
time dd if=/dev/zero of=/tmp/blobfuse-mount/testfile bs=1M count=100

# Mount with otel logger
time dd if=/dev/zero of=/tmp/blobfuse-mount/testfile bs=1M count=100

# Compare times
```

Performance impact should be minimal due to batching.
