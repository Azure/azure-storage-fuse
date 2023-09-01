

ver=`../blobfuse2 --version | cut -d " " -f 3`
image="azure-blobfuse2.$ver"
alpine_image="azure-alpine-blobfuse2.$ver"

docker login blobfuse2containers.azurecr.io --username $1 --password $2

# Publish Ubn-22 container image
docker tag $image:latest blobfuse2containers.azurecr.io/$image
docker push blobfuse2containers.azurecr.io/$image

# Publish Alpine container image
docker tag $alpine_image:latest blobfuse2containers.azurecr.io/$alpine_image
docker push blobfuse2containers.azurecr.io/$alpine_image

docker logout blobfuse2containers.azurecr.io

