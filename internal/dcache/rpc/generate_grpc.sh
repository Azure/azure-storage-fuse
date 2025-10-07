#!/bin/bash
set -euo pipefail

echo "Code generate using - $(protoc --version)"

# Ensure GOPATH/bin on PATH so protoc finds plugins
export GOPATH="${GOPATH:-$HOME/go}"
export PATH="$PATH:$GOPATH/bin"

# Check if protoc is available
if ! command -v protoc &> /dev/null; then
    echo "Error: protoc is not installed. Run: sudo apt install -y protobuf-compiler"
    exit 1
fi

# Check if protoc-gen-go is available
if ! command -v protoc-gen-go &> /dev/null; then
    echo "Error: protoc-gen-go is not installed. Run: go install google.golang.org/protobuf/cmd/protoc-gen-go@latest"
    exit 1
fi

# Check if protoc-gen-go-grpc is available
if ! command -v protoc-gen-go-grpc &> /dev/null; then
    echo "Error: protoc-gen-go-grpc is not installed. Run: go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest"
    exit 1
fi

# Determine repo root
SCRIPT_DIR=$(cd "$(dirname "$0")" && pwd)
REPO_ROOT=$(cd "$SCRIPT_DIR/../../.." && pwd)

cd "$REPO_ROOT" || exit 1

# Clean previous generated code to avoid stale artifacts
rm -rf internal/dcache/rpc/gen-go-grpc

# Generate Go code from Protocol Buffer definitions.
echo "Running protoc..."

protoc \
    -I internal/dcache/rpc \
    --go_out=. \
    --go-grpc_out=. \
    internal/dcache/rpc/models.proto \
    internal/dcache/rpc/service.proto
    
echo "protoc completed"

# Fix formatting if generation succeeded
if [ -d internal/dcache/rpc/gen-go-grpc ]; then
    gofmt -w internal/dcache/rpc/gen-go-grpc/
fi

echo "gRPC code generation completed successfully!"

# Note: We'll add copyright fixing later, similar to the Thrift generation
# cd ../../..
# ./copyright_fix.sh
