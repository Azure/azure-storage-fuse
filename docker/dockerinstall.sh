# Cleanup old installation
sudo apt remove docker-desktop -y 2>/dev/null || true
rm -rf $HOME/.docker/desktop 2>/dev/null || true
sudo rm -f /usr/local/bin/com.docker.cli 2>/dev/null || true
sudo apt purge docker-desktop -y 2>/dev/null || true
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
sudo groupadd docker 2>/dev/null || true  # Don't fail if group already exists
sudo usermod -aG docker $USER

# Create .docker directory if it doesn't exist and set permissions
mkdir -p $HOME/.docker
sudo chown "$USER":"$USER" $HOME/.docker -R
sudo chmod g+rwx "$HOME/.docker" -R

# Set proper permissions on docker socket
sudo chown root:docker /var/run/docker.sock
sudo chmod 666 /var/run/docker.sock

# Start docker service
sudo systemctl enable docker
sudo systemctl start docker

# Wait a moment for docker to start
sleep 3

# Delete old blobfuse2 image (only after docker is running)
if docker images | grep -q blobfuse 2>/dev/null; then
    docker rmi $(docker images | grep blobfuse | awk '{print $3}') 2>/dev/null || true
else
    echo "No blobfuse images found to remove"
fi

# Remove existing images
docker system prune -f 2>/dev/null || true

# List docker container images
docker images

# List docker instances running
docker container ls

echo ""
echo "Docker installation completed."
echo ""

