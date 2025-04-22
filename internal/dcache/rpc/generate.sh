#!/bin/bash

echo "Code generate using - $(thrift --version)"

# Generate Go code from Thrift IDL
thrift -r --gen go service.thrift

# fix import path
find ./gen-go/dcache/ -type f -name "*.go" -exec sed -i 's#"dcache/models"#"github.com/Azure/azure-storage-fuse/v2/internal/dcache/rpc/gen-go/dcache/models"#g' {} +
find ./gen-go/dcache/ -type f -name "*.go" -exec sed -i 's#"dcache/service"#"github.com/Azure/azure-storage-fuse/v2/internal/dcache/rpc/gen-go/dcache/service"#g' {} +

# add copyright to generated files
cd ../../..
./copyright_fix.sh
