#!/bin/bash
work_dir=$(echo $1 | sed 's:/*$::')
if go version | grep -q "$2"; then
  echo "Exists"
else
  wget "https://golang.org/dl/go$2.linux-amd64.tar.gz" -P "$work_dir"
  sudo rm -rf /usr/local/go
  sudo tar -C /usr/local -xzf "$work_dir"/go"$2".linux-amd64.tar.gz
  sudo ln -sf /usr/local/go/bin/go /usr/bin/go
fi
