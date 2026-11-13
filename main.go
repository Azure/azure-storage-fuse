// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package main

import (
	"github.com/Azure/azure-storage-fuse/v2/cmd"
)

//go:generate ./cmd/componentGenerator.sh $NAME
//  To use go:generate run command   "NAME="component" go generate"

func main() {
	_ = cmd.Execute()
}
