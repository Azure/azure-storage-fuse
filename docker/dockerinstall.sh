
# Cleanup old installation
sudo apt remove docker-desktop
rm-r $HOME/.docker/desktop
sudo rm/usr/local/bin/com.docker.cli
sudo apt purge docker-desktop
sudo apt-get update

# Install certificates and pre-requisites
sudo apt-get install ca-certificates curl gnupg lsb-release -y
sudo mkdir -p /etc/apt/keyrings

# Create keyring for docker
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo gpg --dearmor -o /etc/apt/keyrings/docker.gpg

# Create file for installation
echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable"| sudo tee /etc/apt/sources.list.d/docker.list > /dev/null

# Install docker 
sudo apt-get update
sudo apt-get install docker-ce docker-ce-cli containerd.io docker-compose-plugin -y
sudo apt-get update

# Resolve permission issues to connect to docker socket
sudo groupadd docker
sudo usermod -aG docker $USER
sudo chown root:docker /var/run/docker.sock
sudo chown "$USER":"$USER" /home/"$USER"/.docker -R
sudo chmod g+rwx "$HOME/.docker" -R

# Delete old blobfuse2 image
docker rmi `docker images | grep blobfuse | cut -d " " -f1`

# Remove existing images
docker system prune -f

# Start docker service
sudo service docker start

# List docker container images
docker images ls

# List docker instances running
docker container ls

