#!/bin/bash

# Change this path to where you want to clone the repo
REPO_PATH=~/mlperf/storage

# Install necessary packages
sudo DEBIAN_FRONTEND=noninteractive apt install python3-pip python3-venv libopenmpi-dev openmpi-common -y

# Create virtual environment for package installations
python3 -m venv ~/.venvs/myenv
source ~/.venvs/myenv/bin/activate

# Upgrade pip
python3 -m pip install --upgrade pip

# Repo should be cloned in $REPO_PATH
if [ ! -d "$REPO_PATH" ]; then
    echo "Cloning mlperf storage repository"
    mkdir -p "$(dirname "$REPO_PATH")"
    cd "$(dirname "$REPO_PATH")" || exit 1
    git clone -b v2.0 https://github.com/mlcommons/storage.git
fi

cd $REPO_PATH || exit 1

# Install python dependencies
pip3 install -e .

# Check CLI installation
mlpstorage --version