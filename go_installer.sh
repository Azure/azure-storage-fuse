#!/bin/bash
work_dir=$(echo $1 | sed 's:/*$::')
version="1.24.0"
arch=`hostnamectl | grep "Arch" | rev | cut -d " " -f 1 | rev`

if [ $arch != "arm64" ]
then
  arch="amd64"
fi

echo "Installing on : " $arch " Version : " $version
wget "https://golang.org/dl/go$version.linux-$arch.tar.gz" -P "$work_dir"
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf "$work_dir"/go"$version".linux-$arch.tar.gz
sudo ln -sf /usr/local/go/bin/go /usr/bin/go
sudo ln -sf /usr/local/go/bin/gofmt /usr/bin/gofmt
