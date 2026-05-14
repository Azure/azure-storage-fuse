#!/bin/bash
work_dir=$(echo $1 | sed 's:/*$::')
# Microsoft build of Go (FIPS-capable; ships systemcrypto GOEXPERIMENT).
# Major.minor stream is pinned via aka.ms redirector so we always pull the
# latest patch in that line. Override by exporting GO_MAJOR_MINOR before run.
version="${GO_MAJOR_MINOR:-1.26.2}"
arch=`hostnamectl | grep "Arch" | rev | cut -d " " -f 1 | rev`

if [ $arch != "arm64" ]
then
  arch="amd64"
fi

echo "Installing Microsoft Go (FIPS) on : " $arch " Stream : " $version
tarball="go${version}.linux-${arch}.tar.gz"
wget -O "$work_dir/$tarball" "https://aka.ms/golang/release/latest/${tarball}"
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf "$work_dir/$tarball"
sudo ln -sf /usr/local/go/bin/go /usr/bin/go
sudo ln -sf /usr/local/go/bin/gofmt /usr/bin/gofmt

# Surface the toolchain version so build logs make the FIPS lineage obvious.
/usr/local/go/bin/go version
