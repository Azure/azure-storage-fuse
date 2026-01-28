# Testing OpenTelemetry Integration - Summary

This document provides a comprehensive overview of how to test the OpenTelemetry integration for Blobfuse2.

## What Was Implemented

1. **OpenTelemetry Logger** (`common/log/otel_logger.go`):
   - Implements the existing `Logger` interface
   - Sends logs via OTLP HTTP protocol
   - Supports all existing log levels (debug, trace, info, warning, error, critical)
   - Includes rich structured attributes (file, line, PID, mount path, goroutine ID)
   - Uses batch processing for performance

2. **Configuration Support**:
   - Added `otel-endpoint` configuration option
   - Environment variable support via `OTEL_EXPORTER_OTLP_ENDPOINT`
   - Backward compatible with existing logging types (syslog, base, silent)

3. **Documentation**:
   - `OTEL_SETUP.md`: Complete setup guide with Azure Monitor integration
   - `OTEL_TESTING.md`: Quick testing script
   - `sampleOtelConfig.yaml`: Sample configuration file
   - Updated README with OpenTelemetry references

4. **Testing**:
   - Unit tests for all logger functionality
   - Tests handle cases with and without collector running
   - All existing logger tests still pass

## Testing Approaches

### 1. Unit Testing (Automated)

Unit tests are included and can be run without external dependencies:

```bash
# Run all logger tests
go test -v -timeout=2m ./common/log/ --tags=unittest

# Run only OpenTelemetry logger tests
go test -v -timeout=2m ./common/log/ -run TestOtelLogger --tags=unittest
```

**What is tested:**
- Logger creation and initialization
- Log level handling
- All logging methods (Debug, Trace, Info, Warn, Err, Crit)
- Interface compliance
- Factory pattern (via `SetDefaultLogger`)
- Configuration parsing

**Expected Results:**
All tests should pass. Some tests may log warnings about collector connection failures, which is expected when no collector is running.

### 2. Integration Testing with Mock Collector (Manual)

For testing without Azure Monitor, you can verify logs reach the collector:

```bash
# Start a collector that only logs to console
docker run -d \
  --name otel-collector-test \
  -p 4318:4318 \
  -v $(pwd)/otel-collector-test.yaml:/etc/config.yaml \
  otel/opentelemetry-collector-contrib:latest \
  --config=/etc/config.yaml

# Simple collector config (otel-collector-test.yaml):
# receivers:
#   otlp:
#     protocols:
#       http:
#         endpoint: 0.0.0.0:4318
# exporters:
#   logging:
#     loglevel: debug
# service:
#   pipelines:
#     logs:
#       receivers: [otlp]
#       exporters: [logging]
```

Then mount Blobfuse2 with the test config and monitor collector logs:
```bash
docker logs -f otel-collector-test
```

### 3. Full Integration Testing with Azure Monitor (Manual)

For complete end-to-end testing with Azure Monitor:

**Prerequisites:**
- Azure subscription
- Application Insights resource
- Valid connection string

**Steps:**
1. Follow the guide in `OTEL_SETUP.md`
2. Use the quick test script in `OTEL_TESTING.md`
3. Verify logs appear in Azure Monitor (may take 1-2 minutes)

**Verification:**
```kql
traces
| where timestamp > ago(10m)
| where customDimensions.tag == "blobfuse2"
| order by timestamp desc
```

### 4. Configuration Testing

Test that configuration is parsed correctly:

```bash
# Test with config file
./blobfuse2 mount /tmp/test \
  --config-file=sampleOtelConfig.yaml \
  --dry-run  # If available

# Test with CLI override
./blobfuse2 mount /tmp/test \
  --config-file=config.yaml \
  --log-type=otel \
  --log-level=LOG_DEBUG

# Test with environment variable
export OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318
./blobfuse2 mount /tmp/test --config-file=config.yaml
```

### 5. Performance Testing

Verify that OpenTelemetry logging doesn't significantly impact performance:

```bash
# Baseline with base logger
time ./run_benchmark.sh --log-type=base

# With OpenTelemetry logger
time ./run_benchmark.sh --log-type=otel

# Compare results
```

Expected: Performance should be similar due to asynchronous batching.

## Test Coverage

### Functional Coverage
- [x] Logger initialization
- [x] All log levels (debug, trace, info, warning, error, critical)
- [x] Log formatting with attributes
- [x] Configuration parsing
- [x] Factory pattern integration
- [x] Graceful shutdown
- [x] Environment variable support

### Integration Coverage
- [x] OTLP HTTP protocol
- [ ] End-to-end with Azure Monitor (manual verification needed)
- [ ] Performance under load (manual verification needed)
- [ ] Long-running stability (manual verification needed)

### Error Handling Coverage
- [x] Missing collector (logs warning, continues)
- [x] Invalid configuration (returns error)
- [x] Network failures during shutdown (handled gracefully)

## Known Limitations

1. **No Collector Required for Development**: The logger initializes successfully even without a collector, but logs won't be exported until a collector is available.

2. **Flush on Shutdown**: Logs are batched, so some logs may not be exported if the application crashes before shutdown.

3. **No gRPC Support**: Currently only HTTP protocol is implemented. gRPC support could be added if needed.

4. **No Local Fallback**: If the collector is unavailable, logs are only written to stdout. There's no automatic fallback to file-based logging.

## Troubleshooting Tests

### Unit Tests Failing

```bash
# Clean and retry
go clean -testcache
go test -v ./common/log/ --tags=unittest
```

### Collector Connection Issues

```bash
# Verify collector is running
docker ps | grep otel-collector

# Check collector is listening
netstat -tulpn | grep 4318

# Test endpoint manually
curl -v http://localhost:4318/v1/logs
```

### Azure Monitor Not Receiving Logs

1. Check Application Insights connection string
2. Verify collector configuration
3. Wait 2-3 minutes for ingestion
4. Check Azure Monitor for service issues

## Future Testing Recommendations

1. **Load Testing**: Test with high log volume (1000+ logs/second)
2. **Stress Testing**: Test with limited network bandwidth
3. **Failover Testing**: Test collector restart scenarios
4. **Multi-mount Testing**: Test multiple Blobfuse2 instances logging to same collector
5. **Long-running Testing**: Test for memory leaks over 24+ hours

## Summary

The OpenTelemetry integration is fully implemented and tested at the unit level. Manual integration testing with Azure Monitor is straightforward using the provided documentation and test scripts. The implementation follows Go best practices and integrates seamlessly with the existing logging infrastructure.

For production use:
1. Start with the quick test script (`OTEL_TESTING.md`)
2. Follow the full setup guide (`OTEL_SETUP.md`)
3. Monitor initial deployment closely
4. Gradually increase log level as needed
5. Set up alerts for collector health
