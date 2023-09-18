

ver=`../blobfuse2 --version | cut -d " " -f 3`
image="azure-blobfuse2-$3.$ver"

docker login blobfuse2containers.azurecr.io --username $1 --password $2

# Publish Ubn-22 container image
docker tag $image:latest blobfuse2containers.azurecr.io/$image
docker push blobfuse2containers.azurecr.io/$image

docker logout blobfuse2containers.azurecr.io

