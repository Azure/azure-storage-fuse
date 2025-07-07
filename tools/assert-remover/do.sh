#!/bin/bash

export ASSERT_REMOVER=$PWD/tools/assert-remover/remove_asserts
go generate -skip="./cmd/componentGenerator.sh $NAME" ./...
