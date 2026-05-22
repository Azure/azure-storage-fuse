#!/bin/bash
# Fail fast on any error, unset variable, or pipe failure so a broken
# download/extract cannot leave the agent in a half-installed state.
set -euo pipefail

work_dir=$(echo "$1" | sed 's:/*$::')

# Microsoft build of Go (FIPS-capable; ships systemcrypto GOEXPERIMENT).
# Pinned to a specific patch version for reproducible builds. Override by
# exporting GO_VERSION before run (either a specific version like "1.26.3"
# or a major.minor stream like "1.26" which the aka.ms redirector resolves
# to the latest patch in that line).
version="${GO_VERSION:-1.26.3}"
arch=$(hostnamectl | grep "Arch" | rev | cut -d " " -f 1 | rev)

if [ "$arch" != "arm64" ]
then
  arch="amd64"
fi

echo "Installing Microsoft Go (FIPS) on : $arch  Stream : $version"
tarball="go${version}.linux-${arch}.tar.gz"

# TLS-verified download from the Microsoft-controlled aka.ms redirector.
# --tries / --timeout avoid hanging the pipeline on transient network issues.
wget --tries=3 --timeout=60 -O "$work_dir/$tarball" \
  "https://aka.ms/golang/release/latest/${tarball}"

sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf "$work_dir/$tarball"
sudo ln -sf /usr/local/go/bin/go /usr/bin/go
sudo ln -sf /usr/local/go/bin/gofmt /usr/bin/gofmt

# Smoke test: surface the toolchain version so build logs make the FIPS
# lineage obvious, and fail the script here if the install is broken.
/usr/local/go/bin/go version

# Remove the tarball
rm "$work_dir/$tarball"
