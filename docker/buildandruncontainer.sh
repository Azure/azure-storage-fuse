
# Build blobfuse2 binary
ver=`../blobfuse2 --version | cut -d " " -f 3`
tag="azure-blobfuse2.$ver"

./buildcontainer.sh Dockerfile x86_64

# If build was successful then launch a container instance
status=`docker images | grep $tag`

if [ $? = 0 ]; then
	echo " **** Build successful, running container now ******"
	docker run -it --rm \
		--cap-add=SYS_ADMIN \
		--device=/dev/fuse \
		--security-opt apparmor:unconfined \
		-e AZURE_STORAGE_ACCOUNT \
		-e AZURE_STORAGE_ACCESS_KEY \
		-e AZURE_STORAGE_ACCOUNT_CONTAINER \
		$tag
else
	echo "Failed to build docker image"
fi

# Use commands fuse and unfuse inside container to mount and unmount
