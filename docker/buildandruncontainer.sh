
# Build blobfuse2 binary
ver=`../blobfuse2 --version | cut -d " " -f 3`
tag="azure-blobfuse2.$ver"

./buildcontainer.sh Dockerfile x86_64

# Function to run a single container instance
run_container_instance() {
    local instance_name=$1
    local mount_point=$2
    local container_name=$3
    local log_path="/home/dcacheuser/blobfuse2_logs"
    
    echo "Starting container instance: $instance_name"
    # Check if container exists
    if docker ps -a --format '{{.Names}}' | grep -q "^$container_name$"; then
        docker rm -f $container_name
    fi
    docker run -d \
        --name "$container_name" \
        --cap-add=SYS_ADMIN \
        --device=/dev/fuse \
        --security-opt apparmor:unconfined \
        -v "$mount_point:/mnt/blobfuse" \
        -v "/mnt/cachedir:/mnt/cachedir" \
        -v "/mnt:/host_mnt" \
        -v "$log_path/${instance_name}:/root/.blobfuse2" \
        -e INSTANCE_NAME="$instance_name" \
        $tag
}

# If build was successful then launch container instances
status=`docker images | grep $tag`

if [ $? = 0 ]; then
    echo " **** Build successful, running multiple containers now ******"
    
    # Create mount points for multiple instances, cache directory, and blobfuse2 log directories
    sudo mkdir -p ~/mnt/blobfuse1 ~/mnt/blobfuse2 ~/mnt/blobfuse3
    sudo mkdir -p /home/dcacheuser/blobfuse2_logs/instance1 /home/dcacheuser/blobfuse2_logs/instance2 /home/dcacheuser/blobfuse2_logs/instance3
    sudo chmod 777 ~/mnt/blobfuse1 ~/mnt/blobfuse2 ~/mnt/blobfuse3
    sudo chmod 777 /home/dcacheuser/blobfuse2_logs/instance1 /home/dcacheuser/blobfuse2_logs/instance2 /home/dcacheuser/blobfuse2_logs/instance3
    
    # Run multiple container instances
    run_container_instance "instance1" "/mnt/blobfuse1" "blobfuse-container-1"
    # run_container_instance "instance2" "/mnt/blobfuse2" "blobfuse-container-2" 
    # run_container_instance "instance3" "/mnt/blobfuse3" "blobfuse-container-3"
    
    echo "Container instances started:"
    docker ps --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"
    
    echo ""
    echo "To interact with containers:"
    echo "  docker exec -it blobfuse-container-1 /bin/bash"
    echo "  docker exec -it blobfuse-container-2 /bin/bash"
    echo "  docker exec -it blobfuse-container-3 /bin/bash"
    echo ""
    echo "To stop all containers:"
    echo "  docker stop blobfuse-container-1 blobfuse-container-2 blobfuse-container-3"
    echo ""
    echo "To remove all containers:"
    echo "  docker rm blobfuse-container-1 blobfuse-container-2 blobfuse-container-3"
    echo ""
    echo "To check blobfuse2 logs and data:"
    echo "  ls -la /home/dcacheuser/blobfuse2_logs/instance1/"
    echo "  ls -la /home/dcacheuser/blobfuse2_logs/instance2/"
    echo "  ls -la /home/dcacheuser/blobfuse2_logs/instance3/"
    echo ""
    echo "To view logs:"
    echo "  tail -f /home/dcacheuser/blobfuse2_logs/instance1/*.log"
    echo ""
    
else
    echo "Failed to build docker image"
fi

# Use commands fuse and unfuse inside container to mount and unmount

