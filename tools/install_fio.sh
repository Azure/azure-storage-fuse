#!/bin/bash

# Exit immediately if a command exits with a non-zero status.
set -e

# Update package lists and install dependencies
echo "Updating package lists and installing dependencies..."
sudo apt-get update
sudo apt-get install -y build-essential git libaio-dev

# Clone the fio repository
echo "Cloning the fio repository..."
git clone https://github.com/axboe/fio.git
cd fio
git checkout fio-3.36

# Configure, compile, and install fio
echo "Configuring, compiling, and installing fio..."
./configure
make &> /dev/null
sudo make install &> /dev/null

# Clean up the build directory
echo "Cleaning up..."
cd ..
rm -rf fio

# Print the fio version to confirm installation
echo "Installation complete. Verifying fio version..."
fio --version
