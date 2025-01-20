# This setup script can be used to install all the dependencies required to clone and run the project on Ubuntu machines

!/bin/bash

Run the go_installer script with the parent directory as an argument
./go_installer.sh ../
echo "Installed go" 
go version
sudo apt update -y
sudo apt install openssh-server -y
sudo apt install net-tools -y
sudo apt install git -y
sudo apt install gcc -y
sudo apt install libfuse-dev -y
sudo apt install fuse -y
sudo apt install fuse3 -y
sudo apt install libfuse3-dev -y
echo "Installed all dependencies" -y

# Open the file /etc/fuse.conf and uncomment the line user_allow_other
sudo sed -i 's/#user_allow_other/user_allow_other/' /etc/fuse.conf
echo "Uncommented user_allow_other in /etc/fuse.conf"

# Add Microsoft Linux repository for Ubuntu
wget -qO- https://packages.microsoft.com/keys/microsoft.asc | sudo apt-key add -
sudo add-apt-repository "$(wget -qO- https://packages.microsoft.com/config/ubuntu/$(lsb_release -rs)/prod.list)"
sudo apt update

# Install Blobfuse2
sudo apt install blobfuse2 -y
echo "Installed Blobfuse2"

#Blobfuse2 version
blobfuse2 --version

#Build blobfuse2 from repo
#Navigate to the parent directory of the project and run
#./build.sh

# For not entering password every time on running sudo command, add this line at the end of the 
# /etc/sudoers file,
# <user_name> ALL=(ALL:ALL) NOPASSWD:ALL

# Calling the setup script for AzSecPack setup
echo "Calling the setup script for AzSecPack setup"
setup/vmSetupAzSecPack.sh


