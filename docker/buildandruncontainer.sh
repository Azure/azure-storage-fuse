
# Build blobfuse2 binary
cd ..
if [ "$1" == "fuse2" ]
then
	echo "Building blobfuse2 with libfuse"
	./build.sh fuse2
else
	echo "Building blobfuse2 with libfuse3"
	./build.sh
fi

# As docker build can not go out of scope of this directory copy the binary here
cd -
cp ../blobfuse2 ./
cp ../setup/11-blobfuse2.conf ./
cp ../setup/blobfuse2-logrotate ./

# Cleanup older container image from docker
docker image rm azure-blobfuse2 -f

# Build new container image using current code
if [ "$1" == "fuse2" ]
then
	echo "Build container for libfuse"
	docker build -t azure-blobfuse2 -f Dockerfile . --build-arg FUSE2=TRUE
else
	echo "Build container for libfuse3"
	docker build -t azure-blobfuse2 -f Dockerfile .
fi
 
# Image build is executed so we can clean up temp executable from here
rm -rf ./blobfuse2
rm -rf 11-blobfuse2.conf blobfuse2-logrotate

# If build was successful then launch a container instance
status=`docker images | grep azure-blobfuse2`
if [ $? = 0 ]; then
	echo " **** Build successful, running container now ******"
	docker run -it --rm \
		--cap-add=SYS_ADMIN \
		--device=/dev/fuse \
		--security-opt apparmor:unconfined \
		-e AZURE_STORAGE_ACCOUNT \
		-e AZURE_STORAGE_ACCESS_KEY \
		-e AZURE_STORAGE_ACCOUNT_CONTAINER \
		azure-blobfuse2
else
	echo "Failed to build docker image"
fi

# Use commands fuse and unfuse inside container to mount and unmount
