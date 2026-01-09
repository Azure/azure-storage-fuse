#!/bin/bash

# Start OTel Collector and supporting services for Azure Storage Fuse testing
# This script starts the OpenTelemetry Collector, Jaeger, and Prometheus for log visualization

set -e

echo "Starting OpenTelemetry Collector and supporting services..."
echo "========================================================"

# Check if docker and docker-compose are installed
if ! command -v docker &> /dev/null; then
    echo "Error: Docker is not installed. Please install Docker first."
    exit 1
fi

if ! command -v docker-compose &> /dev/null; then
    echo "Error: Docker Compose is not installed. Please install Docker Compose first."
    exit 1
fi

# Get the directory where this script is located
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

echo "Script directory: $SCRIPT_DIR"
echo ""

# Check if required files exist
if [ ! -f "$SCRIPT_DIR/docker-compose-otel.yml" ]; then
    echo "Error: docker-compose-otel.yml not found in $SCRIPT_DIR"
    exit 1
fi

if [ ! -f "$SCRIPT_DIR/otel-collector-config.yaml" ]; then
    echo "Error: otel-collector-config.yaml not found in $SCRIPT_DIR"
    exit 1
fi

echo "✓ docker-compose-otel.yml found"
echo "✓ otel-collector-config.yaml found"
echo ""

# Stop existing containers if running
echo "Stopping any existing containers..."
docker-compose -f "$SCRIPT_DIR/docker-compose-otel.yml" down 2>/dev/null || true

echo ""
echo "Starting services..."
echo ""

# Start the services
docker-compose -f "$SCRIPT_DIR/docker-compose-otel.yml" up -d

# Wait for services to start
echo "Waiting for services to start (10 seconds)..."
sleep 10

echo ""
echo "========================================================"
echo "Services started successfully!"
echo "========================================================"
echo ""
echo "Service Information:"
echo "  - OTel Collector (gRPC): localhost:4317"
echo "  - OTel Collector (HTTP): localhost:4318"
echo "  - Jaeger UI: http://localhost:16686"
echo "  - Prometheus: http://localhost:9090"
echo "  - OTel Collector Health: http://localhost:13133"
echo ""
echo "To stop services, run:"
echo "  docker-compose -f $SCRIPT_DIR/docker-compose-otel.yml down"
echo ""
echo "To view logs, run:"
echo "  docker-compose -f $SCRIPT_DIR/docker-compose-otel.yml logs -f"
echo ""
echo "Next steps:"
echo "  1. Configure Azure Storage Fuse to use endpoint: localhost:4317"
echo "  2. Run Azure Storage Fuse"
echo "  3. View logs in Jaeger UI or check OTel Collector logs"
echo ""
