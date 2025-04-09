#!/bin/bash

echo "Code generate using - $(thrift --version)"

# Generate Go code from Thrift IDL
thrift -r --gen go dcache.thrift

# fix import path
sed -i 's#"dcache"#"github.com/Azure/azure-storage-fuse/v2/internal/dcache_lib/rpc/gen-go/dcache"#g' gen-go/dcache/chunk_service-remote/chunk_service-remote.go

# add copyright to generated files
cd ../../..
./copyright_fix.sh
