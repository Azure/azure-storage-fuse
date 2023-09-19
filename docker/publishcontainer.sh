

ver=`../blobfuse2 --version | cut -d " " -f 3`
image="azure-blobfuse2-$3.$ver"

sudo docker login blobfuse2containers.azurecr.io --username $1 --password $2

# Publish Ubn-22 container image
sudo docker tag $image:latest blobfuse2containers.azurecr.io/$image
sudo docker push blobfuse2containers.azurecr.io/$image

sudo docker logout blobfuse2containers.azurecr.io

