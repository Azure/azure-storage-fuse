
# Build blobfuse2 binary
cd ..
echo "Building blobfuse2 with libfuse3"
./build.sh
ls -l blobfuse2

# As docker build can not go out of scope of this directory copy the binary here
cd -
cp ../blobfuse2 ./
cp ../setup/11-blobfuse2.conf ./
cp ../setup/blobfuse2-logrotate ./

ver=`./blobfuse2 --version | cut -d " " -f 3`
tag="azure-blobfuse2-$2.$ver"

# Cleanup older container image from docker
docker image rm $tag -f

# Build new container image using current code
echo "Build container for libfuse3"
docker build -t $tag -f $1 .

# List all images to verify if new image is created
docker images

# Image build is executed so we can clean up temp executable from here
rm -rf ./blobfuse2
rm -rf 11-blobfuse2.conf blobfuse2-logrotate

# If build was successful then launch a container instance
status=`docker images | grep $tag`
echo $status
